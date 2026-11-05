'use strict';
(() => {
  const root = document.getElementById('am-concept');
  const $ = id => root.querySelector('#' + id);
  let token = location.hash.slice(1);
  try {
    if (token) sessionStorage.setItem('messenger-ui-token', token);
    else token = sessionStorage.getItem('messenger-ui-token') || '';
  } catch (_) { /* A restricted browser can still use the initial URL. */ }
  if (location.hash) history.replaceState(null, '', location.pathname);
  let selected = '', cursor = '', overview = null, timelineVersion = 0, refreshVersion = 0;
  let timelineAbort = null;
  const names = {claude:'Claude Code',codex:'Codex CLI',cursor:'Cursor Agent CLI'};
  const states = {queued:'В очереди',reserved:'Зарезервировано',launching:'Запуск',running:'Обрабатывается',completed:'Обработка завершена',turn_completed:'Ход завершён',uncertain:'Нужен разбор',not_started:'Запуск не состоялся',none:'Попытки нет',idle:'Ожидание письма',active:'Ход активен',starting:'Подготовка'};
  const results = {not_verified:'Успех ещё не подтверждён',verified_turn:'Ответ подтверждён · ход завершён',verified_exit:'Ответ подтверждён · код выхода 0',process_failed:'Процесс завершился с ошибкой'};
  function el(tag, text, cls) { const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls;return n; }
  function notice(text) { $('notice').textContent=text; $('notice').hidden=!text; }
  function show(tab) {
    root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
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
    if(!overview || !overview.sessions.length) {
      cards.append(el('p','Управляемые сессии пока не зарегистрированы.','am-muted'));
      queue.append(el('p','Нет зарегистрированных сессий.','am-muted'));return;
    }
    for(const s of overview.sessions) {
      const alias=s.aliases[0] || '',card=el('article',undefined,'am-agent');
      card.append(el('h3',names[s.provider]||s.provider));
      card.append(el('p',s.aliases.join(' · ')||'Нет адреса','am-small am-muted'));
      const dl=el('dl');
      const phase=s.recorded_stream_state==='none'?s.recorded_attempt_state:s.recorded_stream_state;
      for(const [label,value] of [['Настройка запуска',s.launch_configured?'Записана · готовность не проверена':'Не завершена'],['Вход в аккаунт','Не проверен'],['Состояние в базе',stateText(phase)],['Владение сессией',s.owner_retained?'Сохраняется':'Не удерживается'],['Ожидают / неопределённые',s.queued+' / '+s.uncertain]]) {
        const pair=el('div');pair.append(el('dt',label),el('dd',value));dl.append(pair);
      }
      card.append(dl);
      if(s.aliases_truncated)card.append(el('p','Список адресов сокращён.','am-small am-status-warn'));
      const b=el('button','Открыть переписку','am-agent-action');b.type='button';b.disabled=!alias;
      b.addEventListener('click',()=>{$('session').value=alias;selected=alias;show('messages');loadTimeline(false);});card.append(b);cards.append(card);
      const row=el('div',undefined,'am-row'),main=el('div',undefined,'am-row-main');
      main.append(el('h3',alias||names[s.provider]||s.provider),el('p','В очереди: '+s.queued+' · Неопределённые в истории: '+s.uncertain,'am-small am-muted'));
      row.append(main,el('span',s.decision_pending?'Есть решение по сессии':s.owner_retained?'Владелец удерживается':'Без владельца','am-small'));queue.append(row);
    }
    if(overview.truncated) {cards.append(el('p','Показаны первые 100 сессий. Список неполный.','am-status-warn'));queue.append(el('p','Список сессий неполный.','am-status-warn'));}
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
    const version=++refreshVersion;$('refresh').disabled=true;notice('');
    clearTimeline('Обновляем данные…');$('session').disabled=true;
    const controller=new AbortController(),timeout=setTimeout(()=>controller.abort(),4500);
    try {
      const [status,view]=await Promise.all([api('/api/status',controller.signal),api('/api/overview',controller.signal)]);
      if(version!==refreshVersion)return;
      if(view.schema_version!==1||!Array.isArray(view.sessions))throw Error('Версия сводки не поддерживается.');
      overview=view;
      $('connection').textContent='Клиент доступен';$('connection-dot').classList.add('online');
      $('checked').textContent='Обновлено '+new Date().toLocaleTimeString();
      $('processing').textContent=status.paused?'Новые письма на паузе':'Очередь не на паузе';
      $('permission').textContent=status.cli_launch==='not-authorized'?'Запуск моделей не разрешён.':'Разрешение запуска: '+status.cli_launch;
      const select=$('session');select.replaceChildren();
      for(const s of view.sessions)for(const alias of s.aliases){const option=el('option',(names[s.provider]||s.provider)+' · '+alias);option.value=alias;select.append(option);}
      if(!select.options.length){select.append(el('option','Нет зарегистрированных сессий'));select.options[0].value='';selected='';}
      else {if(!Array.from(select.options).some(o=>o.value===selected))selected=select.options[0].value;select.value=selected;select.disabled=false;}
      renderCards();await loadTimeline(false);
    } catch(error) {
      if(version!==refreshVersion)return;
      overview=null;clearTimeline('Данные клиента недоступны. Сохранённые письма не удалены.');
      $('session').replaceChildren(el('option','Клиент недоступен'));$('session').disabled=true;
      $('agents').replaceChildren(el('p','Состояния сессий недоступны.','am-muted'));
      $('queue').replaceChildren(el('p','Очередь не прочитана. Это не означает, что она пуста.','am-muted'));
      $('connection').textContent='Клиент недоступен';$('connection-dot').classList.remove('online');
      $('processing').textContent='Нет актуальных данных';$('permission').textContent='';$('checked').textContent='Обновление не удалось';
      notice(error.name==='AbortError'?'Клиент не ответил вовремя.':error.message);
    } finally {clearTimeout(timeout);if(version===refreshVersion)$('refresh').disabled=false;}
    try {
      const d=await api('/api/discovery',AbortSignal.timeout(4500));if(version!==refreshVersion)return;
      $('platform').textContent={darwin:'macOS',linux:'Linux',windows:'Windows'}[d.os]||d.os;
      $('discovery').replaceChildren();for(const a of d.agents){const row=el('div',undefined,'am-row');row.append(el('span',a.name),el('span',a.state==='candidate_found'?'Найден кандидат CLI':a.state==='not_found'?'Не найден в PATH':'Не удалось определить','am-small am-muted'));$('discovery').append(row);}
    } catch(_) {if(version===refreshVersion)$('discovery').replaceChildren(el('p','Поиск CLI недоступен.','am-muted'));}
  }
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('session').addEventListener('change',()=>{selected=$('session').value;loadTimeline(false);});
  $('refresh').addEventListener('click',refresh);$('older').addEventListener('click',()=>loadTimeline(true));
  refresh();
})();
