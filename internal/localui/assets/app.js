'use strict';
(() => {
  const root = document.getElementById('am-concept');
  if(window.webkit?.messageHandlers?.updates)document.body.classList.add('am-native');
  const $ = id => root.querySelector('#' + id);
  function applyTheme(theme, persist=false) {
    if(!['system','light','dark'].includes(theme))theme='system';
    document.documentElement.dataset.theme=theme;$('theme-select').value=theme;
    if(persist) {
      if(window.webkit?.messageHandlers?.appearance)window.webkit.messageHandlers.appearance.postMessage(theme);
      else try {localStorage.setItem('messenger-theme',theme);} catch(_) {}
    }
  }
  let initialTheme=window.messengerInitialTheme;
  if(!initialTheme)try{initialTheme=localStorage.getItem('messenger-theme');}catch(_){}
  applyTheme(initialTheme||'system');
  $('theme-select').addEventListener('change',()=>applyTheme($('theme-select').value,true));
  let token = location.hash.slice(1);
  try {
    if (token) sessionStorage.setItem('messenger-ui-token', token);
    else token = sessionStorage.getItem('messenger-ui-token') || '';
  } catch (_) { /* A restricted browser can still use the initial URL. */ }
  if (location.hash) history.replaceState(null, '', location.pathname);
  let selected = '', cursor = '', overview = null, timelineVersion = 0, refreshVersion = 0;
  let timelineAbort = null, lastStatus = null, sending = false, queueBusy = false, pending = null, pendingBroken = false;
  let outboxItems = [], outboxReady = false, discoveryAgents = [];
  const pendingKey = 'messenger-ui-pending:' + token;
  try {
    const saved = sessionStorage.getItem(pendingKey);
    if (saved) {
      const value = JSON.parse(saved);
      if (!/^ui-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value.message_id) ||
          typeof value.alias !== 'string' || !value.alias.trim() || typeof value.text !== 'string' || !value.text.trim()) throw Error('invalid pending');
      pending = {message_id:value.message_id,alias:value.alias,text:value.text};
      selected=pending.alias; $('message-text').value=pending.text;
      $('send-status').textContent='Подтверждение не сохранено. Повтор доставит то же письмо; автоматически оно не отправляется. Если клиент успел сохранить письмо, оно также появится в списке восстановления.';
    }
  } catch (_) {
    pendingBroken = true;
    $('send-status').textContent='Хранилище вкладки недоступно или повреждено. Отправка отключена, чтобы не потерять идентификатор письма.';
  }
  function actionControls() {
    $('message-text').readOnly=!!pending || sending || pendingBroken;
    $('session').disabled=!lastStatus || !!pending || sending;
    $('send').disabled=!lastStatus || !outboxReady || !selected || sending || queueBusy || pendingBroken || (!pending && outboxItems.length>0) || (!pending && !$('message-text').value.trim());
    $('send').textContent=sending?'Отправляем…':pending?'Повторить доставку':'Отправить';
    $('pending-id').hidden=!pending;
    $('pending-id').textContent=pending?'Идентификатор письма: '+pending.message_id:'';
    $('queue-action').disabled=!lastStatus || queueBusy || sending;
    $('queue-action').textContent=queueBusy?'Применяем…':lastStatus && lastStatus.paused?'Возобновить очередь':'Приостановить очередь';
    $('outbox-panel').hidden=!outboxItems.length;
    $('send-hint').textContent=outboxItems.length && !pending?'Сначала разберите сохранённые отправки выше, чтобы случайно не создать повторное задание.':'При разрешённом запуске и работающей очереди письмо может вызвать модель.';
    root.querySelectorAll('[data-outbox-action]').forEach(b=>b.disabled=!lastStatus || sending || queueBusy || !!pending || pendingBroken);
    root.querySelectorAll('.am-agent-action').forEach(b=>b.disabled=!lastStatus || !!pending || sending);
  }
  async function postAPI(path, body) {
    const response=await fetch(path,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(4500)});
    if(!response.ok)throw Error('unconfirmed');
    const result=await response.json();if(result.ok!==true)throw Error('unconfirmed');return result;
  }
  function renderOutbox() {
    const list=$('outbox-items');list.replaceChildren();
    for(const item of outboxItems) {
      const row=el('article',undefined,'am-outbox-item');
      row.append(el('strong',item.alias),el('p',item.text),el('p',item.message_id,'am-small am-muted'));
      row.append(el('p',item.state==='queued'?'Письмо уже сохранено в очереди. Повторная отправка не нужна.':'Письмо сохранено в исходящих. Доставка в очередь ещё не подтверждена.','am-small'));
      const button=el('button',item.state==='queued'?'Понятно, убрать из списка':'Восстановить отправку');
      button.type='button';button.dataset.outboxAction=item.message_id;
      button.addEventListener('click',async()=>{
        if(button.disabled || pending || sending || queueBusy)return;
        if(item.state==='queued') {
          queueBusy=true;actionControls();
          try {await acknowledge(item.message_id);$('send-status').textContent='Подтверждение принято. Само письмо и его история остаются в базе.';}
          catch(_){$('send-status').textContent='Подтверждение не сохранено. Письмо остаётся в очереди; отправлять его заново не нужно.';}
          finally {queueBusy=false;actionControls();}
          await refresh();return;
        }
        const envelope={message_id:item.message_id,alias:item.alias,text:item.text};
        try {sessionStorage.setItem(pendingKey,JSON.stringify(envelope));}
        catch(_) {$('send-status').textContent='Хранилище вкладки недоступно. Исходящее письмо остаётся в базе клиента.';return;}
        pending=envelope;selected=item.alias;$('message-text').value=item.text;
        $('send-status').textContent='Восстановлены исходные адрес, текст и идентификатор. Нажмите «Повторить доставку» для явного продолжения.';
        await refresh();
      });row.append(button);list.append(row);
    }
  }
  async function acknowledge(id) {
    const ack=await postAPI('/api/outbox/ack',{message_id:id});
    if(ack.action!=='ui-ack')throw Error('unconfirmed');
  }
  async function sendMessage() {
    if($('send').disabled || sending)return;
    sending=true;actionControls();
    try {
      if(!pending) {
        const envelope={message_id:'ui-'+crypto.randomUUID(),alias:selected,text:$('message-text').value};
        const saved=JSON.stringify(envelope);
        try {
          sessionStorage.setItem(pendingKey,saved);
          if(sessionStorage.getItem(pendingKey)!==saved)throw Error('not persisted');
        } catch (_) {
          // No request has been made; leave a readable draft and refuse sending.
          pendingBroken=true;
          $('send-status').textContent='Не удалось сохранить идентификатор в этой вкладке. Письмо не отправлялось.';
          return;
        }
        pending=envelope;actionControls();
      }
      const result=await postAPI('/api/enqueue',pending);
      if(result.message_id!==pending.message_id || typeof result.created!=='boolean')throw Error('unconfirmed');
      // Clear the saved envelope only after an exact queue ACK. If storage fails,
      // retain the envelope: repeating it still cannot create another request.
      sessionStorage.removeItem(pendingKey);
      if(sessionStorage.getItem(pendingKey)!==null)throw Error('not cleared');
      const acceptedID=pending.message_id;pending=null;$('message-text').value='';
      $('send-status').textContent=result.created?'Письмо сохранено в очереди. Ответ агента появится отдельно.':'Это письмо уже было сохранено. Второе письмо не создано.';
      try {await acknowledge(acceptedID);}catch(_){$('send-status').textContent+=' Подтверждение просмотра не записано; письмо может остаться в списке восстановления.';}
    } catch (_) {
      $('send-status').textContent='Доставка не подтверждена. Письмо могло сохраниться. Повтор использует тот же идентификатор и текст. Если клиент успел сохранить письмо, оно также появится в списке восстановления.';
    } finally {sending=false;actionControls();}
    await refresh();
  }
  async function changeQueue() {
    if($('queue-action').disabled || queueBusy || !lastStatus)return;
    const action=lastStatus.paused?'resume':'pause';queueBusy=true;actionControls();
    try {
      const result=await postAPI('/api/'+action,{});
      if(result.action!==action)throw Error('unconfirmed');
      $('queue-action-status').textContent=action==='pause'?'Обработка новых писем приостановлена. Текущая работа продолжается.':'Очередь возобновлена. Разрешение запуска моделей не изменено.';
    } catch (_) {
      $('queue-action-status').textContent='Команда не подтверждена. Обновляем фактическое состояние; повтор автоматически не выполняется.';
    } finally {lastStatus=null;queueBusy=false;actionControls();}
    await refresh();
  }
  const names = {claude:'Claude Code',codex:'Codex',cursor:'Cursor'};
  const states = {queued:'В очереди',reserved:'Зарезервировано',launching:'Запуск',running:'Обрабатывается',completed:'Обработка завершена',turn_completed:'Ход завершён',uncertain:'Нужен разбор',not_started:'Запуск не состоялся',none:'Попытки нет',idle:'Ожидание письма',active:'Ход активен',starting:'Подготовка'};
  const results = {not_verified:'Успех ещё не подтверждён',verified_turn:'Ответ подтверждён · ход завершён',verified_exit:'Ответ подтверждён · код выхода 0',process_failed:'Процесс завершился с ошибкой'};
  function el(tag, text, cls) { const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls;return n; }
  function notice(text) { $('notice').textContent=text; $('notice').hidden=!text; }
  function show(tab) {
    root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
    window.messengerUpdateStatus = status => {
    if(!window.webkit?.messageHandlers?.updates)return;
    const version=typeof status?.version==='string'?status.version:'';
    $('app-update-button').hidden=false;
    $('app-update-button').textContent=version?'↓ Доступно обновление':'Проверить обновления';
    $('app-update-button').classList.toggle('am-update-available',!!version);
    $('app-update-version').hidden=!version;
    $('app-update-version').textContent=version;
  };
  $('session-dialog-close').addEventListener('click',()=>$('session-dialog').close());
  $('app-update-button').addEventListener('click',()=>window.webkit?.messageHandlers?.updates?.postMessage('check'));
  root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('screen-title').textContent={messages:'Переписка',agents:'Агенты',queue:'Очередь',server:'Сервер и подключение'}[tab];
  }
  async function api(path, signal) {
    if (!token) throw Error('Откройте приватную ссылку, которую выдала команда ui.');
    const response=await fetch(path,{headers:{Authorization:'Bearer '+token},cache:'no-store',signal});
    if(!response.ok) {
      if(response.status===401)throw Error('Ссылка доступа устарела. Откройте ссылку текущего запуска ui.');
      throw Error('Данные недоступны. Проверьте, запущен ли клиент Messenger с этим каталогом данных.');
    }
    return response.json();
  }
  function clearTimeline(text) {
    timelineVersion++; if(timelineAbort)timelineAbort.abort();cursor='';$('older').hidden=true;
    $('thread').replaceChildren(el('p',text,'am-muted'));
  }
  function stateText(value) { return states[value] || 'Неизвестное состояние: '+String(value); }
  function renderCards() {
    const cards=$('agents'),queue=$('queue');cards.replaceChildren();queue.replaceChildren();
    const sessions=overview?.sessions || [];
    const providers=[...new Set([...Object.keys(names),...sessions.map(s=>s.provider)])];
    for(const provider of providers) {
      const group=sessions.filter(s=>s.provider===provider),card=el('article',undefined,'am-agent');
      card.append(el('h3',names[provider]||provider));
      const candidate=discoveryAgents.find(a=>a.provider===provider);
      const installed=candidate?.state==='candidate_found'?'Найден на компьютере':candidate?.state==='not_found'?'Расположение не определено':'Не проверено';
      const attention=group.some(s=>s.decision_pending||s.recorded_attempt_state==='uncertain'||s.recorded_stream_state==='uncertain');
      card.append(el('p',!overview?'Нет данных клиента':!group.length?'Не подключён':attention?'Требует внимания':'Сессии подключены',attention?'am-status-warn':'am-muted'));
      const dl=el('dl');
      for(const [label,value] of [['Программа',installed],['Вход в аккаунт','Не проверен'],['Подключённые сессии',overview?String(group.length):'Неизвестно']]) {
        const pair=el('div');pair.append(el('dt',label),el('dd',value));dl.append(pair);
      }
      card.append(dl);
      const sessionsButton=el('button','Сессии ('+group.length+')','am-agent-action');sessionsButton.type='button';
      sessionsButton.disabled=!overview;sessionsButton.addEventListener('click',()=>showSessions(provider));card.append(sessionsButton);
      card.append(el('p','Подключение и перенастройка из окна пока недоступны.','am-small am-muted'));cards.append(card);
    }
    for(const s of sessions) {
      const alias=s.aliases[0]||'',row=el('div',undefined,'am-row'),main=el('div',undefined,'am-row-main');
      main.append(el('h3',alias||names[s.provider]||s.provider),el('p','В очереди: '+s.queued+' · Неопределённые в истории: '+s.uncertain,'am-small am-muted'));
      row.append(main,el('span',s.decision_pending?'Есть решение по сессии':s.owner_retained?'Владелец удерживается':'Без владельца','am-small'));queue.append(row);
    }
    if(!sessions.length)queue.append(el('p',overview?'Нет зарегистрированных сессий.':'Данные клиента недоступны.','am-muted'));
    if(overview?.truncated) {cards.append(el('p','Список сессий неполный.','am-status-warn'));queue.append(el('p','Список сессий неполный.','am-status-warn'));}
  }
  function showSessions(provider) {
    const dialog=$('session-dialog'),body=$('session-dialog-body');body.replaceChildren();
    $('session-dialog-title').textContent='Сессии · '+(names[provider]||provider);
    const sessions=(overview?.sessions||[]).filter(s=>s.provider===provider);
    if(!sessions.length)body.append(el('p','Подключённых сессий пока нет.','am-muted'));
    for(const s of sessions) {
      const alias=s.aliases[0]||'',row=el('article',undefined,'am-agent-session');
      const phase=s.recorded_stream_state==='none'?s.recorded_attempt_state:s.recorded_stream_state;
      row.append(el('strong',s.aliases.join(' · ')||'Нет адреса'),el('p',stateText(phase),'am-small'),el('p','В очереди: '+s.queued+' · Требуют разбора: '+s.uncertain,'am-small am-muted'));
      row.append(el('p',s.launch_configured?'Настройка запуска записана; готовность не проверена.':'Настройка запуска не завершена.','am-small am-muted'));
      if(s.owner_retained)row.append(el('p','Сессия остаётся закреплена за своим процессом.','am-small am-muted'));
      const button=el('button','Открыть переписку','am-agent-action');button.type='button';button.disabled=!alias||!!pending||sending;
      button.addEventListener('click',()=>{if(pending||sending)return;dialog.close();$('session').value=alias;selected=alias;show('messages');actionControls();loadTimeline(false);});row.append(button);body.append(row);
    }
    if(!dialog.open)dialog.showModal();
  }
  function renderMessage(item) {
    const row=el('article',undefined,'am-message'),avatar=el('div','→','am-avatar'),body=el('div',undefined,'am-message-body');
    const meta=el('div',undefined,'am-meta');meta.append(el('strong',item.sender));
    if(item.created_at) {const date=new Date(item.created_at);meta.append(el('time',isNaN(date.getTime())?item.created_at:date.toLocaleString()));}
    body.append(meta,el('p',item.text),el('div',stateText(item.state),'am-delivery'));
    if(item.text_truncated)body.append(el('p','Текст сокращён; полная версия остаётся в базе.','am-small am-status-warn'));
    if(item.retry_of)body.append(el('p','Повтор письма: '+item.retry_of,'am-small am-muted'));
    if(item.reply) {
      const reply=el('div',undefined,'am-reply');reply.append(el('h3',item.reply.sender+' → '+item.reply.recipient),el('p',item.reply.text));
      if(item.reply.text_truncated)reply.append(el('p','Ответ сокращён.','am-small am-status-warn'));
      body.append(reply);
    }
    const verified=item.result==='verified_turn'||item.result==='verified_exit';
    body.append(el('div',results[item.result]||'Результат неизвестен','am-delivery '+(verified?'am-status-good':item.result==='process_failed'?'am-status-warn':'')));
    if(item.exit_code!==null && item.exit_code!==undefined)body.append(el('div','Код выхода: '+item.exit_code,'am-small am-muted'));
    if(verified && item.state==='uncertain')body.append(el('p','Ответ подтверждён; защитный резерв ещё не снят.','am-small am-status-warn'));
    row.append(avatar,body);return row;
  }
  async function loadTimeline(older) {
    if(!selected){clearTimeline('Выберите зарегистрированную сессию.');return;}
    if(timelineAbort)timelineAbort.abort();timelineAbort=new AbortController();
    const version=++timelineVersion,alias=selected,before=older?cursor:'';
    $('older').disabled=true;
    if(!older)$('thread').replaceChildren(el('p','Загружаем переписку…','am-muted'));
    try {
      const query=new URLSearchParams({alias,limit:'50'});if(before)query.set('before',before);
      const page=await api('/api/timeline?'+query,timelineAbort.signal);
      if(version!==timelineVersion)return;
      if(page.schema_version!==1||!Array.isArray(page.items)||page.session_alias!==alias)throw Error('Версия журнала не поддерживается.');
      const nodes=page.items.map(renderMessage),thread=$('thread');
      const oldHeight=thread.scrollHeight,oldTop=thread.scrollTop;
      if(older)$('thread').prepend(...nodes);else $('thread').replaceChildren(...nodes);
      if(!older&&!nodes.length)$('thread').append(el('p','В этой сессии ещё нет писем.','am-muted'));
      thread.scrollTop=older?oldTop+(thread.scrollHeight-oldHeight):thread.scrollHeight;
      cursor=page.next_before||'';$('older').hidden=!page.has_more||!cursor;notice('');
    } catch(error) {
      if(version!==timelineVersion||error.name==='AbortError')return;
      if(!older)$('thread').replaceChildren(el('p','Переписка не загружена.','am-muted'));
      notice(error.message);
    } finally {if(version===timelineVersion)$('older').disabled=false;}
  }
  async function refresh() {
    const version=++refreshVersion;lastStatus=null;outboxReady=false;actionControls();$('refresh').disabled=true;notice('');
    clearTimeline('Обновляем данные…');$('session').disabled=true;
    const controller=new AbortController(),timeout=setTimeout(()=>controller.abort(),4500);
    try {
      const [status,view,outbox]=await Promise.all([api('/api/status',controller.signal),api('/api/overview',controller.signal),api('/api/outbox',controller.signal)]);
      if(version!==refreshVersion)return;
      if(view.schema_version!==1||!Array.isArray(view.sessions))throw Error('Версия сводки не поддерживается.');
      if(outbox.schema_version!==1 || !Array.isArray(outbox.items) || outbox.items.some(item=>!['prepared','queued'].includes(item.state)))throw Error('Журнал исходящих недоступен или требует обновления клиента.');
      outboxItems=outbox.items;outboxReady=true;overview=view;lastStatus=status;
      renderOutbox();
      // An accepted envelope may survive a lost browser ACK. Keep local pending
      // retry available; it uses the same ID. Other stored sends are reviewed first.
      $('connection').textContent='Клиент доступен';$('connection-dot').classList.add('online');
      $('checked').textContent='Обновлено '+new Date().toLocaleTimeString();
      $('processing').textContent=status.paused?'Новые письма на паузе':'Очередь не на паузе';
      $('permission').textContent=status.cli_launch==='not-authorized'?'Запуск моделей не разрешён.':'Разрешение запуска: '+status.cli_launch;
      const select=$('session');select.replaceChildren();
      for(const s of view.sessions)for(const alias of s.aliases){const option=el('option',(names[s.provider]||s.provider)+' · '+alias);option.value=alias;select.append(option);}
      if(pending && !Array.from(select.options).some(o=>o.value===pending.alias)){const option=el('option',pending.alias);option.value=pending.alias;select.append(option);}
      if(pending)selected=pending.alias;
      if(!select.options.length){select.append(el('option','Нет зарегистрированных сессий'));select.options[0].value='';selected='';}
      else {if(!Array.from(select.options).some(o=>o.value===selected))selected=select.options[0].value;select.value=selected;select.disabled=false;}
      renderCards();actionControls();await loadTimeline(false);
    } catch(error) {
      if(version!==refreshVersion)return;
      overview=null;lastStatus=null;outboxReady=false;actionControls();clearTimeline('Данные клиента недоступны. Сохранённые письма не удалены.');
      $('session').replaceChildren(el('option','Клиент недоступен'));$('session').disabled=true;
      renderCards();
      $('queue').replaceChildren(el('p','Очередь не прочитана. Это не означает, что она пуста.','am-muted'));
      $('connection').textContent='Клиент недоступен';$('connection-dot').classList.remove('online');
      $('processing').textContent='Нет актуальных данных';$('permission').textContent='';$('checked').textContent='Обновление не удалось';
      notice(error.name==='AbortError'?'Клиент не ответил вовремя.':error.message);
    } finally {clearTimeout(timeout);if(version===refreshVersion){$('refresh').disabled=false;actionControls();}}
    try {
      const d=await api('/api/discovery',AbortSignal.timeout(4500));if(version!==refreshVersion)return;
      discoveryAgents=d.agents;renderCards();actionControls();
      $('platform').textContent={darwin:'macOS',linux:'Linux',windows:'Windows'}[d.os]||d.os;
    } catch(_) {if(version===refreshVersion){discoveryAgents=[];renderCards();actionControls();}}

  }
  window.messengerUpdateStatus = status => {
    if(!window.webkit?.messageHandlers?.updates)return;
    const version=typeof status?.version==='string'?status.version:'';
    $('app-update-button').hidden=false;
    $('app-update-button').textContent=version?'↓ Доступно обновление':'Проверить обновления';
    $('app-update-button').classList.toggle('am-update-available',!!version);
    $('app-update-version').hidden=!version;
    $('app-update-version').textContent=version;
  };
  $('session-dialog-close').addEventListener('click',()=>$('session-dialog').close());
  $('app-update-button').addEventListener('click',()=>window.webkit?.messageHandlers?.updates?.postMessage('check'));
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('session').addEventListener('change',()=>{if(pending||sending)return;selected=$('session').value;actionControls();loadTimeline(false);});
  $('message-text').addEventListener('input',()=>{const field=$('message-text');field.style.height='auto';field.style.height=Math.min(140,Math.max(44,field.scrollHeight))+'px';actionControls();});
  $('send').addEventListener('click',sendMessage);$('queue-action').addEventListener('click',changeQueue);
  $('refresh').addEventListener('click',refresh);$('older').addEventListener('click',()=>loadTimeline(true));
  refresh();
})();
