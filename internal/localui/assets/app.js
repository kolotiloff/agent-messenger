'use strict';
(() => {
  const root = document.getElementById('am-concept');
  if(window.webkit?.messageHandlers?.updates)document.body.classList.add('am-native');
  const $ = id => root.querySelector('#' + id);
  const selectViews = new Map();
  function syncSelects(){selectViews.forEach(view=>view.sync());}
  // Keep the existing select as the source of truth for delivery guards. The
  // visible list is a top-layer popover, so neither pane clips it or grows.
  function enhanceSelect(select) {
    const trigger=document.createElement('button');trigger.type='button';trigger.className='am-select-trigger';
    const label=select.labels[0]?.textContent||'Выбор';
    trigger.setAttribute('aria-label',label);trigger.setAttribute('aria-haspopup','listbox');trigger.setAttribute('aria-expanded','false');
    const value=document.createElement('span');value.className='am-select-value';
    const chevron=document.createElement('span');chevron.className='am-select-chevron';chevron.setAttribute('aria-hidden','true');trigger.append(value,chevron);
    const menu=document.createElement('div');menu.id=select.id+'-menu';menu.className='am-select-menu';menu.setAttribute('popover','auto');menu.setAttribute('role','listbox');menu.setAttribute('aria-label',label);
    trigger.setAttribute('aria-controls',menu.id);select.after(trigger);root.append(menu);select.hidden=true;
    select.labels[0]?.addEventListener('click',event=>{event.preventDefault();trigger.focus();});
    let options=[];
    const isOpen=()=>menu.matches(':popover-open');
    function close(restore=false){if(isOpen())menu.hidePopover();if(restore)trigger.focus();}
    function sync(){value.textContent=select.selectedOptions[0]?.textContent||'Нет данных';trigger.disabled=select.disabled;if(select.disabled)close();}
    function choose(index){if(select.disabled||select.options[index]?.disabled)return;select.selectedIndex=index;sync();close(true);select.dispatchEvent(new Event('change',{bubbles:true}));}
    function open(){
      if(select.disabled)return;menu.replaceChildren();options=[];
      Array.from(select.options).forEach((option,index)=>{
        const row=document.createElement('div');row.className='am-select-option';row.setAttribute('role','option');row.setAttribute('aria-selected',String(option.selected));row.tabIndex=-1;
        const text=document.createElement('span');text.textContent=option.textContent;
        if(option.dataset.provider)row.append(providerIcon(option.dataset.provider));
        const check=document.createElement('span');check.className='am-select-check';check.textContent=option.selected?'✓':'';check.setAttribute('aria-hidden','true');row.append(text,check);
        if(option.disabled)row.setAttribute('aria-disabled','true');
        row.addEventListener('click',()=>choose(index));menu.append(row);options.push(row);
      });
      menu.showPopover();trigger.setAttribute('aria-expanded','true');
      const rect=trigger.getBoundingClientRect(),gap=6,margin=8;
      const below=innerHeight-rect.bottom-gap-margin,above=rect.top-gap-margin;
      const upwards=below<Math.min(300,menu.scrollHeight)&&above>below;
      const height=Math.max(0,Math.min(360,upwards?above:below));
      menu.style.width=Math.min(innerWidth-2*margin,Math.max(rect.width,220))+'px';menu.style.maxHeight=height+'px';
      menu.style.left=Math.max(margin,Math.min(rect.left,innerWidth-menu.getBoundingClientRect().width-margin))+'px';
      menu.style.top=(upwards?rect.top-gap-Math.min(menu.scrollHeight+2,height):rect.bottom+gap)+'px';
      const row=options[select.selectedIndex]||options[0];row?.focus({preventScroll:true});row?.scrollIntoView({block:'nearest'});
    }
    trigger.addEventListener('click',()=>isOpen()?close():open());
    trigger.addEventListener('keydown',event=>{if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();open();if(event.key==='Home')options[0]?.focus();if(event.key==='End')options.at(-1)?.focus();}});
    menu.addEventListener('keydown',event=>{
      const index=options.indexOf(document.activeElement);let next=index;
      if(event.key==='ArrowDown')next=Math.min(options.length-1,index+1);
      else if(event.key==='ArrowUp')next=Math.max(0,index-1);
      else if(event.key==='Home')next=0;
      else if(event.key==='End')next=options.length-1;
      else if(event.key==='Enter'||event.key===' '){event.preventDefault();choose(index);return;}
      else if(event.key==='Escape'){event.preventDefault();close(true);return;}
      else if(event.key==='Tab'){close(true);return;}else return;
      event.preventDefault();options[next]?.focus();
    });
    menu.addEventListener('toggle',()=>trigger.setAttribute('aria-expanded',String(isOpen())));
    new MutationObserver(()=>{close();sync();}).observe(select,{childList:true,subtree:true,attributes:true,attributeFilter:['disabled']});
    window.addEventListener('resize',()=>close());select.addEventListener('change',sync);
    selectViews.set(select.id,{sync});sync();
  }
  function applyTheme(theme, persist=false) {
    if(!['system','light','dark'].includes(theme))theme='system';
    document.documentElement.dataset.theme=theme;$('theme-select').value=theme;syncSelects();
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
  const agentChecks=new Map(),agentSelections=new Map();
  let setupProvider='',checkingAgent=false;
  let creatingSession=false,setupPending=null;
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
    const sendLabel=sending?'Отправляем…':pending?'Повторить доставку':'Отправить';
    $('send').textContent=sending?'…':'↵';$('send').setAttribute('aria-label',sendLabel);$('send').title=sendLabel;
    $('pending-id').hidden=!pending;
    $('pending-id').textContent=pending?'Идентификатор письма: '+pending.message_id:'';
    $('queue-action').disabled=!lastStatus || queueBusy || sending;
    $('queue-action').textContent=queueBusy?'…':lastStatus && lastStatus.paused?'Продолжить':'Пауза';
    $('outbox-panel').hidden=!outboxItems.length;
    const scope=lastStatus?.launch_session,scopeCard=scope&&overview?.sessions.find(s=>s.aliases.includes(scope));
    const outsideScope=scope&&selected!==scope&&!scopeCard?.aliases.includes(selected);
    $('send-hint').textContent=outboxItems.length && !pending?'Сначала разберите сохранённые отправки выше, чтобы случайно не создать повторное задание.':!lastStatus?'Проверяем готовность агента…':lastStatus.cli_launch==='not-authorized'?'Агент пока не запущен. Сообщение сохранится в очереди.':outsideScope?'Для этой сессии запуск выключен. Сообщение останется в очереди.':lastStatus.paused?'Очередь на паузе. Сообщение будет ждать.':'Сообщение будет передано агенту.';
    root.querySelectorAll('[data-outbox-action]').forEach(b=>b.disabled=!lastStatus || sending || queueBusy || !!pending || pendingBroken);
    root.querySelectorAll('.am-agent-action').forEach(b=>b.disabled=!lastStatus || !!pending || sending || b.dataset.empty==='true');
    syncSelects();
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
      $('queue-action-status').textContent='';
    } catch (_) {
      $('queue-action-status').textContent='Изменение не подтверждено. Повторите после проверки состояния.';
    } finally {lastStatus=null;queueBusy=false;actionControls();}
    await refresh();
  }
  const names = {claude:'Claude Code',codex:'Codex',cursor:'Cursor'};
  // Vector marks are bundled locally; rendering never contacts the providers.
  const providerPaths = {"claude":["m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z"],"cursor":["M11.503.131 1.891 5.678a.84.84 0 0 0-.42.726v11.188c0 .3.162.575.42.724l9.609 5.55a1 1 0 0 0 .998 0l9.61-5.55a.84.84 0 0 0 .42-.724V6.404a.84.84 0 0 0-.42-.726L12.497.131a1.01 1.01 0 0 0-.996 0M2.657 6.338h18.55c.263 0 .43.287.297.515L12.23 22.918c-.062.107-.229.064-.229-.06V12.335a.59.59 0 0 0-.295-.51l-9.11-5.257c-.109-.063-.064-.23.061-.23"],"codex":["M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"]};
  function providerIcon(provider) {
    const wrap=el('span',undefined,'am-provider-icon');wrap.setAttribute('aria-hidden','true');
    if(!providerPaths[provider]){wrap.textContent=provider==='local-user'?'Я':'→';return wrap;}
    wrap.dataset.provider=provider;
    const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('focusable','false');
    for(const d of providerPaths[provider]){const path=document.createElementNS('http://www.w3.org/2000/svg','path');path.setAttribute('d',d);svg.append(path);}wrap.append(svg);return wrap;
  }
  function senderProvider(sender){
    if(sender==='local-user')return sender;
    return overview?.sessions?.find(s=>s.aliases.includes(sender))?.provider||'';
  }
  function senderLabel(sender){const provider=senderProvider(sender);return provider==='local-user'?'Вы':provider?(names[provider]||provider)+' · '+sender:sender;}
  const states = {queued:'В очереди',reserved:'Зарезервировано',launching:'Запуск',running:'Обрабатывается',completed:'Обработка завершена',turn_completed:'Ход завершён',uncertain:'Нужен разбор',not_started:'Запуск не состоялся',none:'Попытки нет',idle:'Ожидание письма',active:'Ход активен',starting:'Подготовка'};
  const results = {not_verified:'Успех ещё не подтверждён',verified_turn:'Ответ подтверждён · ход завершён',verified_exit:'Ответ подтверждён · код выхода 0',process_failed:'Процесс завершился с ошибкой'};
  function el(tag, text, cls) { const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls;return n; }
  function notice(text) { $('notice').textContent=text; $('notice').hidden=!text; }
  function show(tab) {
    root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
  root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('queue-toggle').hidden=tab!=='messages';
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
    $('queue-count').textContent=overview?'('+sessions.reduce((n,s)=>n+(s.queued||0),0)+')':'';
    const providers=[...new Set([...Object.keys(names),...sessions.map(s=>s.provider)])];
    for(const provider of providers) {
      const group=sessions.filter(s=>s.provider===provider),card=el('article',undefined,'am-agent');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(provider),el('span',names[provider]||provider));card.append(heading);
      const candidate=discoveryAgents.find(a=>a.provider===provider);
      const installed=candidate?.state==='candidate_found'?'Найден на компьютере':candidate?.state==='not_found'?'Расположение не определено':'Не проверено';
      const attention=group.some(s=>s.decision_pending||s.recorded_attempt_state==='uncertain'||s.recorded_stream_state==='uncertain');
      card.append(el('p',!overview?'Нет данных клиента':!group.length?'Не подключён':attention?'Требует внимания':'Сессии подключены',attention?'am-status-warn':'am-muted'));
      const dl=el('dl');
      const check=agentChecks.get(provider);
      for(const [label,value] of [['Программа',check?.version?.value?installed+' · '+check.version.value:installed],['Вход в аккаунт',check?authenticationText(check.authentication.state):'Не проверен'],['Подключённые сессии',overview?String(group.length):'Неизвестно']]) {
        const pair=el('div');pair.append(el('dt',label),el('dd',value));dl.append(pair);
      }
      card.append(dl);
      const sessionsButton=el('button','Сессии ('+group.length+')','am-agent-action');sessionsButton.type='button';
      sessionsButton.dataset.empty=String(group.length===0);sessionsButton.disabled=!overview||group.length===0;sessionsButton.addEventListener('click',()=>showSessions(provider));card.append(sessionsButton);
      const setup=el('button','Настроить');setup.type='button';setup.disabled=checkingAgent||creatingSession;setup.addEventListener('click',()=>showAgentSetup(provider));card.append(setup);cards.append(card);
    }
    for(const s of sessions) {
      if(!s.queued&&!s.uncertain&&!s.decision_pending)continue;
      const alias=s.aliases[0]||'',row=el('div',undefined,'am-row'),main=el('div',undefined,'am-row-main');
      main.append(el('h3',alias||names[s.provider]||s.provider),el('p','В очереди: '+s.queued+' · Неопределённые в истории: '+s.uncertain,'am-small am-muted'));
      row.append(main,el('span',s.decision_pending?'Есть решение по сессии':s.owner_retained?'Владелец удерживается':'Без владельца','am-small'));queue.append(row);
    }
    if(!queue.children.length)queue.append(el('p',overview?'Нет ожидающих писем':'Данные клиента недоступны.','am-muted'));
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
  function authenticationText(state){return {authenticated:'Вход подтверждён',not_authenticated:'Вход не выполнен',unknown:'Не удалось определить',not_checked:'Не проверен'}[state]||'Не удалось определить';}
  function showAgentSetup(provider){
    if(checkingAgent||creatingSession)return;
    setupProvider=provider;
    const candidate=agentSelections.get(provider)||discoveryAgents.find(a=>a.provider===provider)||{};
    $('agent-setup-title').textContent='Настройка · '+(names[provider]||provider);
    $('agent-cli').value=candidate.cli||'';$('agent-profile').value=candidate.environment||'';
    $('agent-check-status').textContent='';showAgentCheck(agentChecks.get(provider));
    $('agent-check').disabled=false;$('agent-cli').disabled=false;$('agent-profile').disabled=false;
    $('agent-session-setup').hidden=provider!=='claude'||lastStatus?.session_setup!==true;
    $('agent-setup-limit').textContent=provider!=='claude'?'Подключение сессий этого провайдера готовится.':lastStatus?.session_setup!==true?'Для создания сессии требуется обновлённый фоновый клиент. Проверка программы уже доступна.':'';
    $('agent-alias').value=setupPending?.alias||'claude-test-'+crypto.randomUUID().slice(0,8);
    $('agent-create-status').textContent=setupPending?'Предыдущая настройка не подтверждена. Повтор использует те же данные.':'';
    if(setupPending&&provider==='claude'){$('agent-cli').value=setupPending.cli;$('agent-profile').value=setupPending.environment;}
    setupFields();
    $('agent-setup').showModal();
  }
  function setupFields(){
    const frozen=creatingSession||checkingAgent||!!setupPending;
    $('agent-cli').disabled=frozen;$('agent-profile').disabled=frozen;$('agent-alias').disabled=frozen;
    $('agent-check').disabled=frozen;
    $('agent-create').disabled=creatingSession||checkingAgent||lastStatus?.session_setup!==true;
    $('agent-create').textContent=creatingSession?'Создаём сессию…':setupPending?'Повторить подключение':'Создать тестовую сессию';
  }
  async function createAgentSession(){
    if(creatingSession||checkingAgent||setupProvider!=='claude'||lastStatus?.session_setup!==true)return;
    if(!setupPending){
      const draft={id:crypto.randomUUID(),alias:$('agent-alias').value.trim(),cli:$('agent-cli').value.trim(),environment:$('agent-profile').value.trim()};
      if(!draft.alias||!draft.cli||!draft.environment){$('agent-create-status').textContent='Укажите программу, профиль и название сессии.';return;}
      setupPending=draft;
    }
    creatingSession=true;setupFields();renderCards();$('agent-create-status').textContent='Создаём отдельную сессию…';
    try{
      const response=await fetch('/api/setup-claude',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(setupPending),cache:'no-store',signal:AbortSignal.timeout(4500)});
      if(response.status===400||response.status===409){setupPending=null;throw Error('Настройка отклонена. Проверьте пути и выберите свободное название.');}
      if(!response.ok)throw Error('Настройка не подтверждена. Повтор использует те же данные.');
      const result=await response.json();if(result.ok!==true||result.id!==setupPending.id||result.alias!==setupPending.alias)throw Error('Настройка не подтверждена. Повтор использует те же данные.');
      const alias=setupPending.alias;setupPending=null;
      $('agent-create-status').textContent='Сессия «'+alias+'» создана. Модель не запускалась.';
      selected=alias;await refresh();
      $('agent-setup').close();show('messages');$('session').value=alias;actionControls();loadTimeline(false);
    }catch(error){$('agent-create-status').textContent=error.name==='TimeoutError'||error.name==='AbortError'?'Ответ не получен. Повтор использует те же данные; можно проверить список сессий.':error.message;}
    finally{creatingSession=false;setupFields();renderCards();}
  }
  function showAgentCheck(check){
    const result=$('agent-check-result');result.replaceChildren();result.hidden=!check;if(!check)return;
    const reasons={version_not_qualified_for_auth_probe:'Для этой версии проверка входа ещё не подтверждена.',platform_not_qualified:'Проверка на этой ОС ещё не поддерживается.',unrecognized_version_output:'Программа вернула неизвестный формат версии.',timeout_or_cancelled:'Проверка не завершилась вовремя.',execution_failed:'Не удалось выполнить проверку.',output_limit:'Ответ программы превысил допустимый размер.'};
    for(const [label,value] of [['Версия',check.version?.value||'Не удалось определить'],['Вход в аккаунт',authenticationText(check.authentication?.state)],['Проверено',check.checked_at?new Date(check.checked_at).toLocaleString():'Время неизвестно'],['Программа',check.cli],['Профиль',check.environment]]){
      if(!value)continue;const pair=el('div');pair.append(el('dt',label),el('dd',value));result.append(pair);
    }
    const reason=reasons[check.authentication?.reason]||reasons[check.version?.reason];
    if(reason)result.append(el('p',reason,'am-small am-muted'));
  }
  async function checkAgent(){
    if(checkingAgent||!setupProvider)return;
    const provider=setupProvider,selection={provider,cli:$('agent-cli').value.trim(),environment:$('agent-profile').value.trim()};
    if(!selection.cli||!selection.environment){$('agent-check-status').textContent='Укажите программу и каталог профиля.';return;}
    checkingAgent=true;agentChecks.delete(provider);agentSelections.set(provider,selection);showAgentCheck(null);setupFields();renderCards();
    $('agent-cli').disabled=true;$('agent-profile').disabled=true;$('agent-check').disabled=true;$('agent-check-status').textContent='Проверяем версию и вход…';
    try{
      const response=await fetch('/api/check-cli',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(selection),cache:'no-store',signal:AbortSignal.timeout(22000)});
      if(!response.ok)throw Error(response.status===409?'Проверка уже выполняется. Дождитесь её завершения.':response.status===422?'Проверьте путь к программе и существующему профилю.':'Проверка не завершилась. Можно повторить позже.');
      const result=await response.json();if(result.ok!==true||result.check?.provider!==provider)throw Error('Ответ проверки не подтверждён.');
      agentChecks.set(provider,result.check);showAgentCheck(result.check);$('agent-check-status').textContent='Проверка завершена.';
    }catch(error){$('agent-check-status').textContent=error.name==='TimeoutError'||error.name==='AbortError'?'Проверка не завершилась вовремя. Результат неизвестен.':error.message;}
    finally{checkingAgent=false;setupFields();renderCards();}
  }
  function renderMessage(item) {
    const group=el('article',undefined,'am-message-group');
    group.setAttribute('aria-label',item.reply?'Письмо и связанный ответ':'Письмо');
    function message(sender,text,outgoing,date) {
      const row=el('div',undefined,'am-message '+(outgoing?'am-outgoing':'am-incoming'));
      const avatar=el('div',undefined,'am-avatar'),body=el('div',undefined,'am-message-body');
      const meta=el('div',undefined,'am-meta');meta.append(el('strong',senderLabel(sender)));
      avatar.append(providerIcon(senderProvider(sender)));
      if(date){const d=new Date(date);meta.append(el('time',isNaN(d.getTime())?date:d.toLocaleString()));}
      const content=el('div',undefined,'am-message-content');content.append(el('p',text));
      body.append(meta,content);row.append(avatar,body);group.append(row);return body;
    }
    // local-user is the sender assigned by the local UI endpoint. Agent aliases
    // are not evidence of device identity; network ownership will be explicit.
    const body=message(item.sender,item.text,item.sender==='local-user',item.created_at);
    body.append(el('div',stateText(item.state),'am-delivery'));
    if(item.text_truncated)body.append(el('p','Текст сокращён; полная версия остаётся в базе.','am-small am-status-warn'));
    if(item.retry_of)body.append(el('p','Повтор письма: '+item.retry_of,'am-small am-muted'));
    let resultBody=body;
    if(item.reply) {
      resultBody=message(item.reply.sender,item.reply.text,false,null);
      if(item.reply.text_truncated)resultBody.append(el('p','Ответ сокращён.','am-small am-status-warn'));
    }
    const verified=item.result==='verified_turn'||item.result==='verified_exit';
    resultBody.append(el('div',results[item.result]||'Результат неизвестен','am-delivery '+(verified?'am-status-good':item.result==='process_failed'?'am-status-warn':'')));
    if(item.exit_code!==null && item.exit_code!==undefined)resultBody.append(el('div','Код выхода: '+item.exit_code,'am-small am-muted'));
    if(verified && item.state==='uncertain')resultBody.append(el('p','Ответ подтверждён; защитный резерв ещё не снят.','am-small am-status-warn'));
    return group;
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
      $('connection').textContent='Клиент доступен';$('connection-dot').classList.add('online');$('connection-dot').parentElement.title='Этот компьютер подключён';
      $('checked').textContent='Данные обновлены '+new Date().toLocaleTimeString();$('refresh').title='Обновить сообщения и статусы · '+$('checked').textContent;
      $('processing').textContent=status.paused?'На паузе':'Приём включён';
      $('permission').textContent=status.cli_launch==='not-authorized'?'Письма будут ждать запуска агента.':status.launch_session?'Запуск разрешён только для «'+status.launch_session+'».':'';
      const select=$('session');select.replaceChildren();
      for(const s of view.sessions)for(const alias of s.aliases){const option=el('option',(names[s.provider]||s.provider)+' · '+alias);option.value=alias;option.dataset.provider=s.provider;select.append(option);}
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
      $('connection').textContent='Клиент недоступен';$('connection-dot').classList.remove('online');$('connection-dot').parentElement.title='Нет связи с этим компьютером';$('refresh').title='Обновить сообщения и статусы · предыдущая проверка не удалась';
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
    const installed=typeof status?.installedVersion==='string'?status.installedVersion:'';
    $('app-installed-version').hidden=!installed;
    const preview=installed.match(/^0\.0\.0-preview\.(\d+)$/);
    $('app-installed-version').textContent=preview?'Предварительная версия '+preview[1]:installed?'Версия '+installed:'';
    $('app-installed-version').title=installed?'Точная версия: '+installed:'';
    const detail=typeof status?.detail==='string'?status.detail:'';
    const failed=detail.includes('недоступен')||detail.includes('не запустилась')||detail.includes('повторить');
    const checking=status?.checking===true;
    const waiting=detail==='Обновление загружается в фоне';
    $('app-update-button').hidden=!version&&!failed&&!checking;
    const label=version?'Доступно обновление':waiting?'Загружается обновление…':checking?'Проверяем обновления…':'Повторить проверку';
    $('app-update-button').textContent=label;
    $('app-update-button').title=version?'Доступно обновление '+version:detail;
    $('app-update-button').setAttribute('aria-label',version?'Установить обновление '+version:checking?label:'Не удалось проверить обновления. Повторить');
    $('app-update-button').disabled=checking&&!version;
    $('app-update-button').classList.toggle('am-update-available',!!version);
    $('app-update-version').hidden=true;
  };
  $('session-dialog-close').addEventListener('click',()=>$('session-dialog').close());
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-check').addEventListener('click',checkAgent);
  $('agent-create').addEventListener('click',createAgentSession);
  for(const field of [$('agent-cli'),$('agent-profile')])field.addEventListener('input',()=>{agentChecks.delete(setupProvider);showAgentCheck(null);$('agent-check-status').textContent='';renderCards();});
  $('queue-toggle').addEventListener('click',()=>{
    const expanded=$('queue-toggle').getAttribute('aria-expanded')!=='true';
    $('queue-toggle').setAttribute('aria-expanded',String(expanded));
    $('queue-toggle').setAttribute('aria-label',expanded?'Скрыть очередь сообщений':'Показать очередь сообщений');
    $('queue-toggle').title=expanded?'Скрыть очередь сообщений':'Показать очередь сообщений';
    $('queue-panel').hidden=!expanded;
    root.querySelector('.am-chat-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  $('app-update-button').addEventListener('click',()=>window.webkit?.messageHandlers?.updates?.postMessage('check'));
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('session').addEventListener('change',()=>{if(pending||sending)return;selected=$('session').value;actionControls();loadTimeline(false);});
  $('message-text').addEventListener('input',()=>{const field=$('message-text');field.style.height='auto';field.style.height=Math.min(140,Math.max(44,field.scrollHeight))+'px';actionControls();});
  $('send').addEventListener('click',sendMessage);$('queue-action').addEventListener('click',changeQueue);
  $('refresh').addEventListener('click',refresh);$('older').addEventListener('click',()=>loadTimeline(true));
  const chatBody=root.querySelector('.am-chat-body'),chatFooter=root.querySelector('.am-chat-footer');
  new ResizeObserver(()=>{
    const thread=$('thread'),atBottom=thread.scrollHeight-thread.clientHeight-thread.scrollTop<32;
    chatBody.style.setProperty('--am-footer-height',chatFooter.getBoundingClientRect().height+'px');
    if(atBottom)thread.scrollTop=thread.scrollHeight;
  }).observe(chatFooter);
  enhanceSelect($('theme-select'));enhanceSelect($('session'));
  // WebKit may overlay scrollbars while Chromium reserves their width. Use
  // the actual occupied gutter, not the painted thumb width, for equal insets.
  const gutterObserver=new ResizeObserver(entries=>{
    for(const {target} of entries){
      if(!target.getClientRects().length)continue;
      const style=getComputedStyle(target);
      const gutter=Math.max(0,target.offsetWidth-target.clientWidth-parseFloat(style.borderLeftWidth)-parseFloat(style.borderRightWidth));
      const value=gutter+'px';
      if(target.style.getPropertyValue('--am-gutter')!==value)target.style.setProperty('--am-gutter',value);
    }
  });
  root.querySelectorAll('.am-screen,.am-thread,.am-queue-list,.am-dialog-body,.am-machines').forEach(element=>gutterObserver.observe(element));
  refresh();
})();
