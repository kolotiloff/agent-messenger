'use strict';
(() => {
  const root = document.getElementById('am-concept');
  // The Mac app and the Windows window draw their own frame and title (D-054).
  if(window.webkit?.messageHandlers?.updates||window.amDesktop)document.body.classList.add('am-native');
  const $ = id => root.querySelector('#' + id);
  const selectViews = new Map();
  function syncSelects(){selectViews.forEach(view=>view.sync());}
  // Keep the existing select as the source of truth for delivery guards. The
  // visible list is a top-layer popover, so neither pane clips it or grows.
  function enhanceSelect(select) {
    const trigger=document.createElement('button');trigger.type='button';trigger.className='am-select-trigger';
    const label=select.labels[0]?.textContent||select.getAttribute('aria-label')||'Выбор';
    trigger.setAttribute('aria-label',label);trigger.setAttribute('aria-haspopup','listbox');trigger.setAttribute('aria-expanded','false');
    const value=document.createElement('span');value.className='am-select-value';
    const chevron=document.createElement('span');chevron.className='am-select-chevron';chevron.setAttribute('aria-hidden','true');trigger.append(value,chevron);
    const menu=document.createElement('div');menu.id=select.id+'-menu';menu.className='am-select-menu';menu.setAttribute('popover','auto');menu.setAttribute('role','listbox');menu.setAttribute('aria-label',label);menu.tabIndex=-1;
    trigger.setAttribute('aria-controls',menu.id);select.after(trigger);root.append(menu);select.hidden=true;
    select.labels[0]?.addEventListener('click',event=>{event.preventDefault();trigger.focus();});
    let options=[];
    const isOpen=()=>menu.matches(':popover-open');
    function close(restore=false){if(isOpen())menu.hidePopover();if(restore)trigger.focus();}
    function sync(){value.textContent=select.selectedOptions[0]?.textContent||'Нет данных';trigger.disabled=select.disabled;if(select.disabled)close();}
    function choose(index){if(select.disabled||select.options[index]?.disabled)return;select.selectedIndex=index;sync();close(true);select.dispatchEvent(new Event('change',{bubbles:true}));}
    function open(keyboard){
      if(select.disabled)return;menu.replaceChildren();options=[];
      Array.from(select.options).forEach((option,index)=>{
        const row=document.createElement('div');row.className='am-select-option';row.setAttribute('role','option');row.setAttribute('aria-selected',String(option.selected));row.tabIndex=-1;
        const text=document.createElement('span');text.textContent=option.textContent;
        if(option.dataset.provider)row.append(providerIcon(option.dataset.provider));
        const check=document.createElement('span');check.className='am-select-check';check.setAttribute('aria-hidden','true');if(option.selected)check.innerHTML='<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3 8.5l3.2 3.2L13 4.5"/></svg>';row.append(text,check);
        row.addEventListener('pointermove',()=>{if(document.activeElement!==row)row.focus({preventScroll:true});});
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
      const row=options[select.selectedIndex]||options[0];row?.scrollIntoView({block:'nearest'});
      if(keyboard)row?.focus({preventScroll:true});else menu.focus({preventScroll:true});
    }
    trigger.addEventListener('click',()=>isOpen()?close():open());
    trigger.addEventListener('keydown',event=>{if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();open(true);if(event.key==='Home')options[0]?.focus();if(event.key==='End')options.at(-1)?.focus();}});
    menu.addEventListener('keydown',event=>{
      const index=options.indexOf(document.activeElement);let next=index;
      if(index<0&&(event.key==='ArrowDown'||event.key==='ArrowUp'))next=Math.max(0,select.selectedIndex);
      else if(event.key==='ArrowDown')next=Math.min(options.length-1,index+1);
      else if(event.key==='ArrowUp')next=Math.max(0,index-1);
      else if(event.key==='Home')next=0;
      else if(event.key==='End')next=options.length-1;
      else if(event.key==='Enter'||event.key===' '){event.preventDefault();if(index>=0)choose(index);return;}
      else if(event.key==='Escape'){event.preventDefault();close(true);return;}
      else if(event.key==='Tab'){close(true);return;}else return;
      event.preventDefault();options[next]?.focus();
    });
    menu.addEventListener('toggle',()=>trigger.setAttribute('aria-expanded',String(isOpen())));
    new MutationObserver(()=>{close();sync();}).observe(select,{childList:true,subtree:true,attributes:true,attributeFilter:['disabled']});
    window.addEventListener('resize',()=>close());select.addEventListener('change',sync);
    selectViews.set(select.id,{sync});sync();
  }
  const providerPaths = {"claude":["m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z"],"cursor":["M11.503.131 1.891 5.678a.84.84 0 0 0-.42.726v11.188c0 .3.162.575.42.724l9.609 5.55a1 1 0 0 0 .998 0l9.61-5.55a.84.84 0 0 0 .42-.724V6.404a.84.84 0 0 0-.42-.726L12.497.131a1.01 1.01 0 0 0-.996 0M2.657 6.338h18.55c.263 0 .43.287.297.515L12.23 22.918c-.062.107-.229.064-.229-.06V12.335a.59.59 0 0 0-.295-.51l-9.11-5.257c-.109-.063-.064-.23.061-.23"],"codex":["M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"]};
  // Fonts place their capitals differently inside the line box, so a fixed
  // top padding centres control text in one font and not in another. Measure
  // the font in use and set the padding that puts the capitals in the middle.
  function centreControlText(){
    try{
      const host=root,style=getComputedStyle(host),size=parseFloat(style.fontSize);
      const probe=document.createElement('div');
      probe.style.cssText='position:absolute;visibility:hidden;line-height:1.35;white-space:nowrap';
      probe.append('Н',Object.assign(document.createElement('span'),{style:'display:inline-block;width:0;height:0'}));
      host.append(probe);
      const baseline=probe.lastChild.getBoundingClientRect().top-probe.getBoundingClientRect().top;
      probe.remove();
      const ctx=document.createElement('canvas').getContext('2d');
      ctx.font=style.fontWeight+' '+style.fontSize+' '+style.fontFamily;
      const cap=ctx.measureText('Н').actualBoundingBoxAscent;
      if(!(cap>0&&baseline>0&&size>0))return;
      const nudge=2*(size*1.35/2-(baseline-cap/2));
      host.style.setProperty('--am-text-nudge',Math.round(Math.min(6,Math.max(0,nudge))*2)/2+'px');
    }catch{}
  }
  function applyTheme(theme, persist=false) {
    if(!['system','light','dark'].includes(theme))theme='system';
    document.documentElement.dataset.theme=theme;$('theme-select').value=theme;syncSelects();
    // The Windows window colours its title bar like the page.
    if(window.amTitleBar){const dark=theme==='dark'||(theme==='system'&&matchMedia('(prefers-color-scheme: dark)').matches);try{window.amTitleBar(dark);}catch(_){}}
    if(persist) {
      if(window.webkit?.messageHandlers?.appearance)window.webkit.messageHandlers.appearance.postMessage(theme);
      else try {localStorage.setItem('messenger-theme',theme);} catch(_) {}
    }
  }
  // Task window (D-024, D-026, T-029). The window observes tasks; the owner
  // talks to the coordinator in the Claude app. Its only actions are creating
  // a project, changing the project folder, connecting an agent program and
  // answering a worker's question to the owner.
  const names={claude:'Claude Code',codex:'Codex',cursor:'Cursor'};
  const shortNames={claude:'Claude',codex:'Codex',cursor:'Cursor'};
  function el(tag,text,cls){const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(cls)n.className=cls;return n;}
  function providerIcon(provider){
    const wrap=el('span',undefined,'am-provider-icon');wrap.setAttribute('aria-hidden','true');
    if(!providerPaths[provider]){wrap.textContent=provider==='owner'?'Я':'→';return wrap;}
    wrap.dataset.provider=provider;
    const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('focusable','false');
    for(const d of providerPaths[provider]){const path=document.createElementNS('http://www.w3.org/2000/svg','path');path.setAttribute('d',d);svg.append(path);}wrap.append(svg);return wrap;
  }
  function notice(node){const n=$('notice');n.replaceChildren();if(node)n.append(node);n.hidden=!node;}
  let token=location.hash.slice(1);
  try{if(token)sessionStorage.setItem('messenger-ui-token',token);else token=sessionStorage.getItem('messenger-ui-token')||'';}catch(_){}
  if(location.hash)history.replaceState(null,'',location.pathname);
  function remember(key,value){try{localStorage.setItem(key,value);}catch(_){}}
  function recall(key){try{return localStorage.getItem(key)||'';}catch(_){return '';}}
  let view=null,project=recall('messenger-project'),selectedTask='',feed=null,feedSignature='',viewSignature='';
  let online=false,offlineReason='',discovery=[],tab='tasks',refreshing=false,answering=false;
  const agentChecks=new Map(),checking=new Set(),rowNote=new Map();let autoChecked=false,discoveryLoaded=false;
  let clientInfo=null,clientInfoAt=0,restart={busy:false,ask:null,error:''};
  // The window can be newer than the client still running in the background
  // (after an update). Say how to restart it; nothing is restarted by itself.
  function shutdownCommand(){return view?.messenger&&view?.data_root?'"'+view.messenger+'" --root "'+view.data_root+'" shutdown':'';}
  function outdatedMessage(){
    if(Date.now()-clientInfoAt>3000)loadClientInfo();
    if(clientInfo?.restart)return Error('В фоне старая версия Messenger.');
    const e=Error('В фоне старая версия клиента, перезапустите его');
    e.hint='Закройте Messenger, выполните скопированную команду в Терминале и откройте Messenger снова.';
    e.command=shutdownCommand();
    return e;
  }
  // An error of the owner's action stays at the top until it is closed or
  // the next action succeeds; the 2 s refresh does not hide it.
  let actionError=null;
  function showError(error){actionError=error?{text:error.message,command:error.command||'',hint:error.hint||''}:null;renderToasts();}
  // A small chip above the input field, like a chat's status chips.
  function renderToasts(){
    const box=$('toasts');box.replaceChildren();box.hidden=!actionError;if(!actionError)return;
    const chip=el('div',undefined,'am-toast');chip.append(el('span',actionError.text,'am-toast-text'));if(actionError.hint)chip.title=actionError.hint;
    if(actionError.command){
      const command=actionError.command,copy=el('button','Скопировать команду');copy.type='button';copy.title=command;
      copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent='Скопировано';}catch(_){copy.textContent='Не удалось скопировать';}setTimeout(()=>copy.textContent='Скопировать команду',2000);});
      chip.append(copy);
    }
    const close=el('button','×','am-toast-close');close.type='button';close.setAttribute('aria-label','Закрыть');close.addEventListener('click',()=>showError(null));chip.append(close);
    box.append(chip);
  }
  async function api(path,timeout=4500){
    if(!token)throw Error('Откройте приватную ссылку, которую выдала команда ui.');
    const response=await fetch(path,{headers:{Authorization:'Bearer '+token},cache:'no-store',signal:AbortSignal.timeout(timeout)});
    if(!response.ok){
      if(response.status===401)throw Error('Ссылка доступа устарела. Откройте окно заново.');
      const body=await response.text().catch(()=>'');
      if(body.startsWith('client_outdated'))throw outdatedMessage();
      if(body.startsWith('client_rejected'))throw Error('Клиент Messenger отклонил запрос окна. Подробности: messenger doctor.');
      throw Error('Клиент Messenger не запущен или не отвечает. Данные не обновляются.');
    }
    return response.json();
  }
  async function postAPI(path,body,timeout=6000){
    let response;
    // No answer means the result is unknown, not that nothing happened.
    try{response=await fetch(path,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(timeout)});}
    catch(_){throw Error('Ответ не получен, результат неизвестен. Проверьте состояние, прежде чем повторять.');}
    let result={};try{result=await response.json();}catch(_){}
    if(response.ok&&result.ok===true)return result;
    if(result.state==='client_outdated')throw outdatedMessage();
    if(response.status===409&&result.detail)throw Error(result.detail);
    if(response.status===400)throw Error('Проверьте введённые данные.');
    throw Error('Действие не подтверждено. Проверьте состояние и повторите.');
  }
  function minutes(from,to){
    const a=Date.parse(from),b=to?Date.parse(to):Date.now();
    if(isNaN(a)||isNaN(b))return '';
    const m=Math.max(0,Math.round((b-a)/60000));
    if(m<1)return '<1 мин';
    if(m<60)return m+' мин';
    const h=Math.floor(m/60);return h+' ч'+(m%60?' '+m%60+' мин':'');
  }
  // T-058: what a task spent, as its agent reported it. Claude reports a
  // price; Codex and Cursor only tokens.
  function tokensText(n){
    if(n>=1e6)return (n/1e6).toLocaleString('ru-RU',{maximumFractionDigits:1})+' млн';
    if(n>=1e3)return Math.round(n/1e3).toLocaleString('ru-RU')+' тыс.';
    return String(n);
  }
  function costText(c){return c>0&&c<0.01?'<$0.01':'$'+c.toFixed(2);}
  function usageShort(u){
    if(!u)return '';
    return u.cost_usd!=null?costText(u.cost_usd):tokensText(u.input_tokens+u.output_tokens)+' токенов';
  }
  function usageLong(u){
    if(!u)return '';
    let s='Потрачено '+(u.cost_usd!=null?costText(u.cost_usd)+' (по расчёту агента), ':'')+'токенов: '+u.input_tokens.toLocaleString('ru-RU')+' на входе';
    if(u.cached_tokens)s+=' (из них '+u.cached_tokens.toLocaleString('ru-RU')+' из кэша)';
    return s+', '+u.output_tokens.toLocaleString('ru-RU')+' на выходе';
  }
  function clock(at){const d=new Date(at);return isNaN(d.getTime())?'':d.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});}
  const activeStates=['running','waiting_owner','waiting_coordinator'];
  // T-034: why a run failed, as the provider reported it. Nothing is retried.
  function resetText(at){
    const d=new Date(at);if(isNaN(d.getTime()))return '';
    const day=x=>x.getFullYear()+'-'+x.getMonth()+'-'+x.getDate(),today=new Date(),tomorrow=new Date();tomorrow.setDate(today.getDate()+1);
    if(day(d)===day(today))return 'в '+clock(at);
    if(day(d)===day(tomorrow))return 'завтра в '+clock(at);
    return d.toLocaleDateString([], {day:'numeric',month:'long'})+' в '+clock(at);
  }
  function failureStatus(t){
    switch(t.failure_reason){
      case 'usage_limit':{const r=t.failure_reset?resetText(t.failure_reset):'';return [r?'Лимит, сброс '+r:'Лимит исчерпан','am-status-bad'];}
      case 'rate_limit':return ['Слишком много запросов','am-status-bad'];
      case 'overload':return ['Сервис перегружен','am-status-bad'];
      case 'sign_in':return ['Нужен вход в аккаунт','am-status-bad'];
    }
    return null;
  }
  function taskStatus(t){
    const end=t.finished_at||t.updated_at,from=t.started_at||t.created_at;
    switch(t.state){
      case 'running':return t.pending_launch?['Ждёт запуска','am-status-wait']:['Работает '+minutes(from),'am-status-run'];
      case 'waiting_owner':return ['Ждёт вашего ответа','am-status-wait'];
      case 'waiting_coordinator':return ['Ждёт ответа координатора','am-status-run'];
      case 'done':return ['Готово за '+minutes(from,end),'am-muted'];
      case 'failed':return failureStatus(t)||['Ошибка через '+minutes(from,end),'am-status-bad'];
      case 'interrupted':return ['Прервана через '+minutes(from,end),'am-status-bad'];
      case 'blocked':return ['Заблокирована через '+minutes(from,end),'am-status-bad'];
      case 'cancelled':return ['Отменена','am-muted'];
    }
    return [t.state,'am-muted'];
  }
  function coordinatorName(client=view?.coordinator?.client){
    const c=(client||'').toLowerCase();
    if(c.includes('codex'))return ['codex','Codex'];
    if(c.includes('claude')||!c||c==='coordinator')return ['claude','Claude'];
    return ['',client];
  }
  function agentState(a){
    if(!online)return ['offline','Нет связи'];
    if(a.waiting_owner>0)return ['wait','Ждёт ответа'];
    if(a.running>0)return ['run',a.running>1?'Работает ('+a.running+')':'Работает'];
    return ['idle','Свободен'];
  }
  function renderProjects(){
    const select=$('project-select');select.replaceChildren();
    for(const p of view?.projects||[]){const o=el('option',p.title||p.name);o.value=p.name;select.append(o);}
    const add=el('option','Новый проект');add.value='__new';select.append(add);
    select.value=view?.project||'__new';
    if(!view?.project&&select.options.length>1)select.selectedIndex=0;
    select.disabled=!view;syncSelects();
  }
  // Right-click menu on a joined device. Revoking cannot be undone, so the
  // menu item only opens a confirmation inside the card.
  function deviceMenu(d,x,y){
    let menu=$('device-menu');
    if(!menu){
      // A manual popover: an automatic one closes on the same right click
      // that opened it.
      menu=el('div',undefined,'am-select-menu am-context-menu');menu.id='device-menu';menu.setAttribute('popover','manual');menu.setAttribute('role','menu');root.append(menu);
      const close=()=>{if(menu.matches(':popover-open'))menu.hidePopover();};
      document.addEventListener('pointerdown',e=>{if(!menu.contains(e.target))close();});
      menu.addEventListener('keydown',e=>{if(e.key==='Escape'){e.preventDefault();close();}});
      menu.addEventListener('focusout',e=>{if(!menu.contains(e.relatedTarget))close();});
      window.addEventListener('resize',close);
    }
    const item=el('div','Отозвать устройство','am-select-option');item.setAttribute('role','menuitem');item.tabIndex=0;
    const pick=()=>{menu.hidePopover();revokeAsk=d.id;clearTimeout(revokeTimer);revokeTimer=setTimeout(()=>{revokeAsk='';renderMachines();},8000);renderMachines();$('machines').querySelector('[data-key="revoke-yes-'+d.id+'"]')?.focus();};
    item.addEventListener('click',pick);item.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();pick();}});
    menu.replaceChildren(item);menu.style.left=x+'px';menu.style.top=y+'px';menu.showPopover();item.focus();
  }
  // One line per agent program, with its state (D-064: no coordinator mark).
  // live is false when the computer cannot be reached.
  function agentLines(agents,coordinatorClient,live){
    const lines=el('div',undefined,'am-machine-agents');
    const [coordProvider,coordLabel]=coordinatorClient!==null?coordinatorName(coordinatorClient):['',''];
    const providers=agents.map(a=>a.agent);
    if(coordProvider&&!providers.includes(coordProvider))providers.unshift(coordProvider);
    for(const provider of providers){
      const a=agents.find(x=>x.agent===provider);
      // Blue means a task is running (T-060). A chat that once used
      // Messenger's tools is listed but not lit: nothing tells whether it is
      // still open.
      const [state,text]=!live?['offline','Нет связи']:a?agentState(a):['idle','Чат с инструментами Messenger'];
      const line=el('div',undefined,'am-agent-line');line.dataset.provider=provider;
      line.title=(names[provider]||provider)+(a?': '+text:'');
      line.append(el('span',undefined,'am-dot am-dot-'+state),el('span',shortNames[provider]||provider));
      lines.append(line);
    }
    if(coordinatorClient!==null&&!coordProvider){
      const line=el('div',undefined,'am-agent-line');line.append(el('span',undefined,'am-dot am-dot-'+(live?'idle':'offline')),el('span',coordLabel));lines.append(line);
    }
    return lines;
  }
  // A computer of the network counts as reachable while its reports arrive:
  // it reports every 15 s, so three missed reports mean it is not.
  const peerFresh=peer=>peer.age>=0&&peer.age<=45;
  function pickable(card,id){
    card.setAttribute('role','button');card.tabIndex=0;card.setAttribute('aria-pressed',String(computerFilter===id));
    card.title=computerFilter===id?'Показать задачи всех компьютеров':'Показать задачи этого компьютера';
    card.addEventListener('click',event=>{if(!event.target.closest('button'))filterComputer(id);});
    card.addEventListener('keydown',event=>{if(event.target===card&&(event.key==='Enter'||event.key===' ')){event.preventDefault();filterComputer(id);}});
  }
  function renderMachines(){
    const box=$('machines');box.replaceChildren();
    const machine=el('div',undefined,'am-machine-card');machine.setAttribute('aria-current','true');pickable(machine,'local');
    const head=el('div',undefined,'am-machine-name');head.append(el('span',view?.computer_name||'Этот компьютер'));
    const dot=el('span',undefined,'am-dot'+(online?' online':''));dot.title=online?'На связи':'Нет связи';head.append(dot);machine.append(head);
    const lines=agentLines(view?.agents||[],view?.coordinator?.connected?(view.coordinator.client||''):null,online);
    if(!lines.children.length)lines.append(el('p',online?'Агенты не подключены':'Нет связи','am-small am-muted'));
    machine.append(lines);box.append(machine);
    // The other computers of this computer's network, with their agents.
    // Only the computer that runs the server can revoke one, from the
    // right-click menu.
    const hosted=new Set((server?.host?.devices||[]).filter(d=>!d.revoked_at).map(d=>d.id));
    for(const peer of server?.peers||[]){
      const fresh=peerFresh(peer);
      const card=el('div',undefined,'am-machine-card am-peer-card');card.dataset.peer=peer.id;pickable(card,peer.id);
      const name=el('div',undefined,'am-machine-name');
      const pdot=el('span',undefined,'am-dot'+(fresh?' online':''));pdot.title=fresh?'На связи':'Нет связи';
      name.append(el('span',peer.name),pdot);card.append(name);
      if(revokeAsk===peer.id&&hosted.has(peer.id)){
        card.append(el('p','Отозвать устройство? Вернуть его можно только новым приглашением.','am-small am-muted'));
        const actions=el('div',undefined,'am-actions am-device-confirm');
        const yes=el('button','Отозвать','am-danger');yes.type='button';yes.dataset.key='revoke-yes-'+peer.id;
        yes.addEventListener('click',()=>{revokeAsk='';serverAction({action:'revoke',device:peer.id},'host','');});
        const no=el('button','Отмена');no.type='button';
        no.addEventListener('click',()=>{revokeAsk='';renderMachines();$('machines').querySelector('[data-key="revoke-'+peer.id+'"]')?.focus();});
        actions.append(no,yes);card.append(actions);box.append(card);continue;
      }
      if(fresh){
        const plines=agentLines(peer.agents||[],peer.coordinator?peer.coordinator:null,true);
        if(!plines.children.length)plines.append(el('p','Агенты не подключены','am-small am-muted'));
        card.append(plines);
      }else{
        card.append(el('p',peer.last_seen?'Был на связи '+stamp(peer.last_seen):'Ещё не выходил на связь','am-small am-muted'));
      }
      if(hosted.has(peer.id)){
        card.classList.add('am-device-card');card.tabIndex=0;card.dataset.key='revoke-'+peer.id;card.title='Правая кнопка мыши — действия';
        card.addEventListener('contextmenu',event=>{event.preventDefault();deviceMenu(peer,event.clientX,event.clientY);});
        card.addEventListener('keydown',event=>{if(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')){event.preventDefault();const r=card.getBoundingClientRect();deviceMenu(peer,r.left+16,r.top+24);}});
      }
      box.append(card);
    }
  }
  // The computer that runs a task: this one, or the other one it was sent to.
  function taskComputer(t){return t.remote&&!t.remote.from?t.remote.id:'local';}
  // A click on a computer in the sidebar shows only its tasks; a second click,
  // the chip's cross or a click on empty sidebar space shows all again (T-056).
  let computerFilter='';
  function filterComputer(id){
    computerFilter=computerFilter===id?'':id;
    if(computerFilter)show('tasks');
    renderMachines();renderTaskList();
  }
  function visibleTasks(){
    const filter=$('agent-filter').value;
    const list=(view?.tasks||[]).filter(t=>(!filter||t.agent===filter)&&(!computerFilter||taskComputer(t)===computerFilter));
    const active=list.filter(t=>activeStates.includes(t.state)),done=list.filter(t=>!activeStates.includes(t.state));
    return [active,done];
  }
  function renderTaskList(){
    const box=$('task-list');box.replaceChildren();
    const [active,done]=visibleTasks();
    const chipName=computerFilter==='local'?(view?.computer_name||'Этот компьютер'):(server?.peers||[]).find(p=>p.id===computerFilter)?.name||(view?.tasks||[]).find(t=>taskComputer(t)===computerFilter)?.remote?.name||'';
    $('computer-chip').hidden=!computerFilter;$('computer-chip-text').textContent=computerFilter?'Компьютер: '+chipName:'';
    $('task-count').textContent=view?.tasks?.length?String(view.tasks.length):'';
    const section=(label,items)=>{
      if(!items.length)return;
      box.append(el('div',label,'am-label am-task-section'));
      for(const t of items){
        const row=el('button',undefined,'am-task');row.type='button';row.setAttribute('aria-pressed',String(t.id===selectedTask));
        const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),el('span',(names[t.agent]||t.agent)+(t.remote&&!t.remote.from?' на '+t.remote.name:'')));
        if(t.remote)agent.title=t.remote.from?'Задачу дал компьютер «'+t.remote.name+'»':'Выполняется на компьютере «'+t.remote.name+'»';
        const [text,cls]=taskStatus(t);
        const status=el('span',undefined,'am-task-status '+cls);
        if(t.state==='waiting_owner')status.append(el('span',undefined,'am-dot am-dot-wait'));
        const words=el('span',text);status.append(words);
        if(t.usage){const used=el('span',', '+usageShort(t.usage),'am-task-usage');used.title=usageLong(t.usage);words.append(used);}
        row.append(agent,el('span',t.title,'am-task-title'),status);
        row.addEventListener('click',()=>{selectedTask=t.id;feedSignature='';renderTaskList();loadFeed();});
        box.append(row);
      }
    };
    section('Активные',active);section('Завершённые',done);
    if(!box.children.length)box.append(el('p',view?.project?(computerFilter||$('agent-filter').value?'Здесь задач нет':'Задач пока нет'):'','am-small am-muted'));
  }
  // Coordinator connection (T-039): the client runs Claude's own
  // `claude mcp add` in the project folder; the owner never opens a terminal.
  let linkBusy='',linkFailure=null;
  async function connectCoordinator(name){
    if(linkBusy)return;
    linkBusy=name;linkFailure=null;if(view?.project===name)renderThread();
    try{
      const r=await postAPI('/api/coordinator-connect',{project:name},40000);
      if(r.result?.state==='failed')linkFailure={project:name,detail:r.result.detail||''};
    }catch(error){linkFailure={project:name,detail:error.message};}
    finally{linkBusy='';viewSignature='';await refresh();}
  }
  // Shown as the first message of the project's conversation, from
  // Messenger, until the coordinator chat connects.
  function connectHelp(){
    const g=el('article',undefined,'am-message-group am-connect'),row=el('div',undefined,'am-message');
    const avatar=el('div',undefined,'am-avatar');const mark=el('span','M','am-provider-icon');mark.setAttribute('aria-hidden','true');avatar.append(mark);
    const body=el('div',undefined,'am-message-body'),meta=el('div',undefined,'am-meta');meta.append(el('strong','Messenger'));
    const box=el('div',undefined,'am-message-content');meta.prepend(avatar);body.append(meta,box);row.append(body);g.append(row);
    const link=view.coordinator_link||{state:'unknown'},failure=linkFailure?.project===view.project?linkFailure:null;
    const connect=label=>{const b=el('button',label);b.type='button';b.addEventListener('click',()=>connectCoordinator(view.project));return b;};
    if(linkBusy===view.project){
      box.append(el('p','Подключаем Claude к проекту…','am-connect-title'),el('p','Messenger добавляет этот проект в настройки Claude. Это займёт несколько секунд.'));
    }else if(link.state==='connected'){
      box.append(el('p','Claude подключён к проекту','am-connect-title'));
      box.append(el('p','Откройте папку проекта в Claude (вкладка Code, этот компьютер) и начните новый чат. Напишите в нём, что нужно сделать. Чтобы проверить подключение, спросите: «Каких агентов ты видишь?»'));
      box.append(el('p','Чат, открытый до подключения, проект не увидит: нужен новый.','am-small am-muted'));
    }else{
      box.append(el('p','Подключите Claude к проекту','am-connect-title'));
      if(failure){box.append(el('p','Не получилось подключить: '+failure.detail),connect('Попробовать ещё раз'));}
      else if(link.state==='no_claude'){box.append(el('p','Сначала подключите Claude Code во вкладке «Агенты», затем нажмите кнопку ниже.'),connect('Подключить к Claude'));}
      else if(link.state==='other'){box.append(el('p','В настройках Claude для этой папки уже есть другое подключение с именем «messenger». Messenger его не меняет. Если оно больше не нужно, удалите его командой «claude mcp remove messenger» в этой папке и нажмите кнопку ниже.'),connect('Подключить к Claude'));}
      else if(link.state==='unknown'){box.append(el('p','Не удалось прочитать настройки Claude'+(link.detail?': '+link.detail:'.')),connect('Подключить к Claude'));}
      else if(link.state==='no_folder'){box.append(el('p','У проекта нет папки на этом компьютере. Укажите её в настройках проекта.'));}
      else{box.append(el('p','Задачи проекта даёт ваш чат в Claude. Messenger сам добавит этот проект в настройки Claude.'),connect('Подключить к Claude'));}
    }
    const manual=el('details',undefined,'am-tool');manual.append(el('summary','Подключить вручную через Терминал'));
    const command='cd "'+view.folder+'" && claude mcp add --scope local messenger -- "'+view.messenger+'" --root "'+view.data_root+'" coordinator --project "'+view.project+'"';
    const copy=el('button','Скопировать команду');copy.type='button';
    copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent='Скопировано';}catch(_){copy.textContent='Скопируйте вручную';}setTimeout(()=>copy.textContent='Скопировать команду',2000);});
    manual.append(el('pre',command,'am-command'),copy);
    box.append(manual,el('p','Чат-координатор видит только задачи этого проекта.','am-small am-muted'));
    return g;
  }
  function renderThread(){
    const thread=$('thread'),head=$('task-head');
    $('compose').hidden=true;feedSignature='';
    if(!view){head.replaceChildren();return;}
    if(!view.project){
      head.replaceChildren();
      const box=el('div',undefined,'am-settings am-connect');box.append(el('h3','Создайте проект'),el('p','Проект связывает чат-координатора, папку проекта на этом компьютере и задачи.'));
      const b=el('button','Новый проект','am-primary');b.type='button';b.addEventListener('click',()=>openProjectDialog(false));box.append(b);
      thread.replaceChildren(box);return;
    }
    if(!view.coordinator.connected&&!view.tasks.length){head.replaceChildren();thread.replaceChildren(connectHelp());return;}
    if(!selectedTask||!feed||feed.task.id!==selectedTask){
      head.replaceChildren();
      if(!selectedTask)thread.replaceChildren(el('p',view.tasks.length?'Выберите задачу в списке справа.':'Задач пока нет. Их создаёт чат-координатор.','am-muted am-empty'));
      return;
    }
    renderFeed();
  }
  // Called when only the feed may have changed.
  function updateFeed(){
    if(!view?.project||!feed||feed.task.id!==selectedTask){renderThread();return;}
    renderFeed();
  }
  function roleTag(text){return el('span',text,'am-role');}
  // Agent text: fenced ``` blocks become code blocks and `inline` spans become
  // code; everything else stays plain text. Built from DOM nodes, never HTML.
  function inlineCode(p,text){
    const parts=text.split(/(`[^`\n]+`)/);
    for(const part of parts){
      if(part.length>2&&part.startsWith('`')&&part.endsWith('`'))p.append(el('code',part.slice(1,-1),'am-code'));
      else if(part)p.append(document.createTextNode(part));
    }
    return p;
  }
  function richText(text,cls){
    const box=el('div',undefined,'am-rich'+(cls?' '+cls:''));
    const pieces=String(text||'').split(/^```[^\n]*\n?/m);
    pieces.forEach((piece,i)=>{
      if(i%2===1){box.append(el('pre',piece.replace(/\n$/,''),'am-code-block'));return;}
      for(const para of piece.split(/\n{2,}/)){if(para.trim())box.append(inlineCode(el('p'),para.replace(/^\n+|\n+$/g,'')));}
    });
    return box;
  }
  // A tool action as one line: its kind and, shortened, what it acted on.
  const toolNames={Bash:'Команда',Read:'Чтение',Edit:'Правка',MultiEdit:'Правка',Write:'Запись',Grep:'Поиск',Glob:'Поиск файлов',WebFetch:'Загрузка страницы',WebSearch:'Поиск в интернете',Task:'Подзадача',TodoWrite:'План'};
  // The command itself, without the shell that runs it.
  function commandText(line){
    const m=line.match(/^"?[^"]*(?:powershell|pwsh)(?:\.exe)?"?\s+-Command\s+([\s\S]*)$/i)||line.match(/^"?\S*\/(?:ba|z)?sh"?\s+-l?c\s+([\s\S]*)$/);
    if(!m)return line;
    const c=m[1].trim(),q=c[0];
    return (q==="'"||q==='"')&&c.endsWith(q)&&c.length>1?c.slice(1,-1):c;
  }
  function toolName(it){return toolNames[it.title]||it.title||'Действие';}
  function toolLine(it){
    const name=toolName(it);
    const first=commandText((it.text||'').split('\n')[0]);
    const short=first.length>90?first.slice(0,89)+'…':first;
    const d=el('details',undefined,'am-tool');const s=el('summary');s.append(el('span',name,'am-tool-name'));
    if(short)s.append(el('span',' '+short,'am-tool-arg'));
    d.append(s);if(it.text&&(it.text!==first||first.length>90))d.append(el('pre',it.text));else if(it.text)d.classList.add('am-tool-flat');
    return d;
  }
  // Stop, like in a chat: a square button in the input field, or Esc.
  let stopping=false;
  async function stopTask(){
    const t=feed?.task;if(!t||stopping||!activeStates.includes(t.state))return;
    stopping=true;$('task-stop').disabled=true;$('task-stop').classList.add('am-tip-off');
    try{await postAPI('/api/task-cancel',{task:t.id});showError(null);feedSignature='';await refresh();}
    catch(e){showError(e);}
    finally{stopping=false;$('task-stop').disabled=false;}
  }
  function renderFeed(){
    const t=feed.task,thread=$('thread'),head=$('task-head');
    const signature=JSON.stringify(feed);if(signature===feedSignature)return;
    const atBottom=thread.scrollHeight-thread.clientHeight-thread.scrollTop<40||!feedSignature;feedSignature=signature;
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    const where=t.remote&&!t.remote.from?' на компьютере «'+t.remote.name+'»':' на '+(view.computer_name||'этом компьютере')+(t.remote?.from?', по заданию компьютера «'+t.remote.name+'»':'');
    title.append(el('p',(names[t.agent]||t.agent)+(t.model?' ('+t.model+')':'')+where+', папка '+(t.folder||'ещё не известна'),'am-small am-muted'));
    const [statusText,statusCls]=taskStatus(t);
    const when=t.finished_at?', '+clock(t.created_at)+'–'+clock(t.finished_at):', начата в '+clock(t.created_at);
    const line=el('p',undefined,'am-small');line.append(el('span',statusText,statusCls),el('span',when,'am-muted'));
    if(t.usage){const used=el('span',', '+usageShort(t.usage),'am-muted');used.title=usageLong(t.usage);line.append(used);}
    title.append(line);
    const headRow=el('div',undefined,'am-task-head-row');headRow.append(title);
    head.replaceChildren(headRow);
    const [coordProvider,coordLabel]=coordinatorName();
    const opened=new Set([...thread.querySelectorAll('details[open][data-key]')].map(d=>d.dataset.key));
    const out=[];let group=null,tools=null;
    // Actions in a row fold into one line (D-055 follow-up).
    const addTool=(c,it,key)=>{
      const line=toolLine(it);line.dataset.key='t'+key;
      if(!tools||tools.host!==c||c.lastChild!==tools.node){c.append(line);tools={host:c,node:line,key,count:1,names:new Set([toolName(it)])};return;}
      if(!tools.box){const box=el('details',undefined,'am-tool-group');box.dataset.key='g'+tools.key;tools.sum=el('summary');box.append(tools.sum);tools.node.replaceWith(box);box.append(tools.node);tools.box=box;tools.node=box;}
      tools.box.append(line);tools.count++;tools.names.add(toolName(it));
      tools.sum.textContent='Действий: '+tools.count+' — '+[...tools.names].join(', ');
    };
    const outgoing=(author,provider,role,text,at)=>{
      group=null;const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message am-outgoing');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      const who=el('strong',author);meta.append(who);if(role)meta.append(roleTag(role));meta.append(el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');content.append(richText(text));body.append(meta,content);row.append(body);g.append(row);out.push(g);
    };
    const workerGroup=at=>{
      if(group)return group;
      const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message');
      const avatar=el('div',undefined,'am-avatar');avatar.append(providerIcon(t.agent));
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      meta.append(el('strong',names[t.agent]||t.agent),roleTag('исполнитель'),el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');meta.prepend(avatar);body.append(meta,content);row.append(body);g.append(row);out.push(g);group=content;return content;
    };
    const delivery=(text,cls)=>el('div',text,'am-delivery '+cls);
    for(const [index,it] of feed.items.entries()){
      if(it.kind!=='tool')tools=null;
      if(it.kind==='task'){outgoing(coordLabel,coordProvider,'координатор',it.text,it.at);continue;}
      if(it.kind==='answer'){if(it.author==='owner')outgoing('Вы','owner','',it.text,it.at);else outgoing(coordLabel,coordProvider,'координатор',it.text,it.at);continue;}
      if(it.kind==='accepted'){const c=out[out.length-1]?.querySelector('.am-message-content');if(c)c.append(delivery('Принято на компьютере «'+it.text+'»','am-status-good'));group=null;continue;}
      if(it.kind==='text'){workerGroup(it.at).append(richText(it.text));continue;}
      if(it.kind==='tool'){addTool(workerGroup(it.at),it,index);continue;}
      if(it.kind==='report'||it.kind==='question'){
        const c=workerGroup(it.at);c.append(richText(it.text,'am-disposition'));
        if(!it.verified)c.append(delivery('Не опубликовано: запуск не прошёл проверку','am-status-bad'));
        else if(it.kind==='report')c.append(delivery('Отчёт отправлен координатору, сессия закрыта','am-status-good'));
        else if(it.audience==='owner')c.append(delivery('Вопрос вам, координатор его не видит','am-status-wait'));
        else c.append(delivery('Вопрос координатору','am-status-wait'));
        group=null;continue;
      }
      if(it.kind==='outcome'){
        group=null;const text={failed:t.remote&&!t.remote.from?'Задача не выполнена':'Запуск завершился с ошибкой',blocked:t.remote&&!t.remote.from?'Задача заблокирована: результат на другом компьютере неизвестен':'Задача заблокирована: процесс не завершился полностью',interrupted:'Клиент перезапустился во время работы; результат неизвестен',cancelled:'Задача отменена'}[it.state]||it.state;
        const note=el('div',undefined,'am-outcome '+(it.state==='cancelled'?'am-muted':'am-status-bad'));note.append(el('p',text));
        const why=t.remote&&!t.remote.from?refusalText(it.text,t.remote.name):'';if(why)note.append(el('p',why));
        if(it.text)note.append(el('p',it.text,'am-small am-muted'));
        const g=el('article',undefined,'am-message-group');g.append(note);out.push(g);
      }
    }
    thread.replaceChildren(...out);
    for(const d of thread.querySelectorAll('details[data-key]'))if(opened.has(d.dataset.key))d.open=true;
    // A task another computer started is answered there.
    const q=t.remote?.from?null:t.owner_question;
    const active=activeStates.includes(t.state),box=$('answer-text');
    $('compose').hidden=!q&&!active;
    box.disabled=!q;box.placeholder=q?'Ответить исполнителю…':t.remote?.from&&t.owner_question?'Отвечают на компьютере «'+t.remote.name+'»':t.pending_launch?'Задача ждёт запуска…':'Исполнитель работает…';
    $('answer-send').hidden=!q;$('answer-hint').hidden=!q;$('task-stop').hidden=!active;
    if(q&&$('compose').dataset.question!==q.id){$('compose').dataset.question=q.id;$('answer-text').value='';$('answer-status').textContent='';}
    answerControls();
    if(atBottom)thread.scrollTop=thread.scrollHeight;
  }
  // Known refusals of another computer, explained in the window's language.
  function refusalText(text,name){
    if(/does not run tasks of other computers/.test(text||''))return 'Компьютер «'+name+'» не принимает задачи других компьютеров. Включите на нём «Задачи других компьютеров» на вкладке «Сервер» и дайте задачу снова.';
    if(/has no project/.test(text||''))return 'На компьютере «'+name+'» нет проекта с таким названием. Создайте его там и выберите папку.';
    if(/has no folder for the project|project folder is missing/.test(text||''))return 'На компьютере «'+name+'» у проекта не выбрана папка или её нет. Выберите папку проекта там и дайте задачу снова.';
    if(/is not set up on this computer/.test(text||''))return 'На компьютере «'+name+'» этот агент не настроен. Настройте его там на вкладке агентов.';
    return '';
  }
  function answerControls(){$('answer-send').disabled=answering||!$('answer-text').value.trim()||!feed?.task?.owner_question||!!feed?.task?.remote?.from;}
  async function sendAnswer(){
    const q=feed?.task?.owner_question;if(!q||answering||feed.task.remote?.from)return;
    const text=$('answer-text').value.trim();if(!text)return;
    let op=recall('messenger-answer-op:'+q.id);
    if(!op){op='window-'+q.id+'-'+crypto.randomUUID();remember('messenger-answer-op:'+q.id,op);}
    answering=true;answerControls();$('answer-status').textContent='Отправляем ответ…';
    try{await postAPI('/api/task-answer',{task:feed.task.id,question:q.id,op_id:op,text});showError(null);$('answer-status').textContent='Ответ записан. Исполнитель продолжит в той же сессии.';$('answer-text').value='';feedSignature='';await refresh();}
    catch(error){$('answer-status').textContent='';showError({message:error.message+' Повтор с тем же текстом безопасен.',command:error.command,hint:error.hint});}
    finally{answering=false;answerControls();}
  }
  function renderAgents(){
    const cards=$('agents');cards.replaceChildren();
    for(const provider of ['claude','codex','cursor']){
      const a=(view?.agents||[]).find(x=>x.agent===provider);
      const found=discovery.find(d=>d.provider===provider);
      const coordinator=view?.coordinator?.connected&&coordinatorName()[0]===provider;
      const card=el('article',undefined,'am-card');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(provider),el('span',names[provider]));card.append(heading);
      // One line per state in one font: the label on the left, the coloured
      // value on the right (owner, preview.37).
      const dl=el('div',undefined,'am-status-lines');
      const pair=(label,value,cls,extra)=>{const p=el('p');const v=el('span',undefined,'am-status-value '+cls);v.append(el('span',value));if(extra)v.append(...extra);p.append(el('span',label,'am-status-label'),v);dl.append(p);};
      // An action the row needs sits in the row itself (owner, preview.37):
      // «Подключить», «Установить», «Войти». While it runs the row says so.
      const inline=(label,fn)=>{const b=el('button',label,'am-inline-action');b.type='button';b.addEventListener('click',fn);return b;};
      const cli=a?.cli||found?.cli||'',profile=a?.environment||found?.environment||'';
      const programNote=rowNote.get(provider+':program');
      if(a)pair('Программа','Подключена','am-status-good');
      else if(programNote)pair('Программа',programNote,'am-status-wait');
      else if(found?.state==='candidate_found')pair('Программа','','',[inline('Подключить',()=>connectAgent(provider))]);
      else if(found?.state==='not_found')pair('Программа','','',[inline('Установить',()=>installAgent(provider))]);
      else pair('Программа','Не проверена','am-status-wait');
      const check=agentChecks.get(provider);
      const auth=check?.authentication?.state;
      const authNote=rowNote.get(provider+':auth');
      // T-056: no confirmed login means «Войти», never «Не проверен»; a
      // confirmed login offers «Выйти».
      if(checking.has(provider))pair('Вход в аккаунт','Проверяем…','am-status-wait');
      else if(authNote)pair('Вход в аккаунт',authNote,'am-status-wait');
      else if(a&&auth==='authenticated')pair('Вход в аккаунт','Выполнен','am-status-good',[inline('Выйти',()=>logoutFromCard(provider,cli,profile))]);
      else if(a&&cli)pair('Вход в аккаунт','','',[inline('Войти',()=>loginFromCard(provider,cli,profile))]);
      else pair('Вход в аккаунт','—','am-muted');
      if(a){
        const trust=a.trust?.state;
        if(trust==='trusted')pair('Рабочая папка','Доверяет','am-status-good');
        else{
          const help=el('button','?','am-q');help.type='button';help.setAttribute('aria-label','Как исправить');
          const hint=el('div',undefined,'am-hint');hint.hidden=true;hint.append(el('p',a.trust?.hint||'Откройте папку проекта в этой программе и подтвердите доверие.'));
          const again=el('button','Проверить снова');again.type='button';again.addEventListener('click',()=>refresh());hint.append(again);
          help.addEventListener('click',()=>{hint.hidden=!hint.hidden;});
          pair('Рабочая папка',trust==='not_trusted'?'Не доверяет':'Не определено',trust==='not_trusted'?'am-status-bad':'am-status-wait',[help]);
          dl.append(hint);
        }
        // T-044: the limit the agent reported during its last run.
        const l=a.limit;
        if(l){
          const r=l.resets_at?resetText(l.resets_at):'';
          const used=typeof l.used_percent==='number'?Math.round(l.used_percent)+'%':'';
          const text=l.state==='exhausted'?(r?'Исчерпан, сброс '+r:'Исчерпан'):l.state==='warning'?(used?used+', близко к пределу':'Близко к пределу')+(r?', сброс '+r:''):(used?'Использовано '+used:'В норме')+(r?', сброс '+r:'');
          pair('Лимит',text,l.state==='exhausted'?'am-status-bad':l.state==='warning'?'am-status-wait':'am-status-good');
          if(l.observed_at)dl.lastChild.title='По данным последней задачи, '+clock(l.observed_at);
        }
      }
      const running=a?.project_running||0,today=a?.project_today||0;
      if(a)pair('Задачи',running||today?running+' идёт, '+today+' сегодня':'Нет',running?'am-status-wait':'am-muted');
      card.append(dl);
      const actions=el('div',undefined,'am-card-actions');
      if(running||today){const tasks=el('button','Задачи');tasks.type='button';tasks.addEventListener('click',()=>{$('agent-filter').value=provider;syncSelects();show('tasks');renderTaskList();});actions.append(tasks);}
      const setup=el('button','Настройки');setup.type='button';setup.addEventListener('click',()=>openAgentSetup(provider));
      actions.append(setup);card.append(actions);cards.append(card);
    }
    renderAccessBar();
  }
  // T-056: one «Любые команды» switch for every agent of this computer.
  function renderAccessBar(){
    const agents=view?.agents||[],bar=$('access-bar');bar.hidden=!agents.length;
    const on=agents.filter(a=>a.full_access).length,all=!!agents.length&&on===agents.length;
    $('access-toggle').setAttribute('aria-checked',all?'true':on?'mixed':'false');
    $('access-title').textContent=on&&!all?'Любые команды: разрешены не всем':'Любые команды';
  }
  async function toggleAllAccess(){
    const on=$('access-toggle').getAttribute('aria-checked')!=='true';
    $('access-toggle').disabled=true;$('access-toggle').setAttribute('aria-checked',String(on));
    try{await postAPI('/api/agent-access',{provider:'all',on});viewSignature='';await refresh();}
    catch(error){renderAccessBar();showError(error);}
    finally{$('access-toggle').disabled=false;}
  }
  // A popup asks before something that is hard to undo: «Отмена» or the
  // named action. The action's error stays in the popup.
  let confirmAction=null;
  function askConfirm(title,text,go,action){
    confirmAction=action;$('confirm-title').textContent=title;$('confirm-text').textContent=text;$('confirm-go').textContent=go;
    $('confirm-error').hidden=true;$('confirm-go').disabled=false;$('confirm-dialog').showModal();
  }
  async function runConfirm(){
    if(!confirmAction)return;$('confirm-go').disabled=true;
    try{await confirmAction();$('confirm-dialog').close();}
    catch(error){$('confirm-error').textContent=error.message;$('confirm-error').hidden=false;}
    finally{$('confirm-go').disabled=false;}
  }
  // The background client can be older than this window after an update.
  // The desktop window offers to restart it; a browser window shows how.
  async function loadClientInfo(){
    clientInfoAt=Date.now();
    try{const info=await api('/api/client-info');clientInfo=info;if(!info.outdated)restart.ask=null;renderRestart();}catch(_){}
  }
  async function restartClient(confirm){
    restart={busy:true,ask:restart.ask,error:''};renderRestart();
    let response,result={};
    try{response=await fetch('/api/restart-client',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({project,confirm}),cache:'no-store',signal:AbortSignal.timeout(50000)});result=await response.json().catch(()=>({}));}
    catch(_){restart={busy:false,ask:null,error:'Ответ не получен, результат неизвестен. Подождите немного: окно проверит клиент само.'};renderRestart();loadClientInfo();return;}
    if(response.ok&&result.ok){restart={busy:false,ask:null,error:''};showError(null);await loadClientInfo();viewSignature='';refresh();return;}
    if(result.state==='confirm'){restart={busy:false,ask:{running:result.running||0,known:result.known!==false},error:''};renderRestart();return;}
    restart={busy:false,ask:null,error:result.detail||'Перезапуск не подтверждён. Проверьте состояние.'};renderRestart();loadClientInfo();
  }
  // A window newer than its client cannot work properly, so this dialog
  // covers the whole window until the client is restarted or the app quits.
  // Quitting closes only the window; the background client keeps running.
  function renderRestart(){
    const dialog=$('restart-dialog');
    const show=!!clientInfo?.outdated||restart.busy||!!restart.error;
    if(!show){if(dialog.open)dialog.close();return;}
    const ask=restart.ask,canRestart=!!clientInfo?.restart,answering=clientInfo?.client==='running';
    const canQuit=!!window.webkit?.messageHandlers?.app;
    let title='Нужно перезапустить Messenger',text;
    if(ask){title=ask.known?'Сейчас выполняются задачи':'Не удалось проверить задачи';text=(ask.known?'Задач в работе: '+ask.running+'. Перезапуск прервёт их.':'Если задачи сейчас идут, перезапуск прервёт их.')+' Сами они не повторятся и будут ждать вашего решения.';}
    else if(restart.busy)text='Останавливаем старую версию и запускаем новую. Это займёт несколько секунд.';
    else if(canRestart)text='Приложение обновилось до версии '+clientInfo.window_version+', а в фоне работает '+(clientInfo.client_version?'версия '+clientInfo.client_version:'более ранняя версия')+'. Пока она не перезапущена, Messenger не может работать. Задачи, история и устройства сохранятся.';
    else text='В фоне работает более ранняя версия клиента. Закройте окно, выполните скопированную команду в Терминале и откройте окно снова.';
    $('restart-title').textContent=title;$('restart-text').textContent=text;
    $('restart-error').textContent=restart.error;$('restart-error').hidden=!restart.error;
    const go=$('restart-go'),quit=$('restart-quit'),copy=$('restart-copy');
    go.hidden=!canRestart||!answering||restart.busy;
    go.textContent=ask?'Перезапустить всё равно':'Перезапустить';
    go.classList.toggle('am-primary',!ask);go.classList.toggle('am-danger',!!ask);
    quit.hidden=!canQuit;quit.disabled=restart.busy;
    copy.hidden=canRestart||!shutdownCommand();
    if(!dialog.open){dialog.showModal();(go.hidden?quit.hidden?copy:quit:go).focus();}
  }
  function renderNotice(){
    if(!online){notice(el('p',offlineReason||'Клиент Messenger не запущен или не отвечает. Данные не обновляются.'));return;}
    if(view?.pending_launch>0&&!view.tasks_enabled){notice(el('p','Задачи записаны, но этот клиент не запускает исполнителей: он запущен без разрешения --allow-tasks.'));return;}
    if(view?.pending_launch>0&&view.paused){
      const box=el('div',undefined,'am-notice-row');box.append(el('p','Обработка на паузе: новые задачи не запускаются.'));
      const go=el('button','Продолжить');go.type='button';go.addEventListener('click',async()=>{go.disabled=true;try{await postAPI('/api/resume',{});showError(null);await refresh();}catch(e){go.disabled=false;showError(e);}});
      box.append(go);notice(box);return;
    }
    notice(null);
  }
  function render(){
    renderProjects();renderMachines();renderTaskList();renderThread();renderAgents();renderProjectPage();renderNotice();autoCheck();
    $('connection').textContent=online?'Клиент доступен':'Клиент недоступен';
  }
  async function loadFeed(){
    if(!selectedTask){feed=null;renderThread();return;}
    const id=selectedTask;
    try{const f=await api('/api/task?id='+encodeURIComponent(id));if(id===selectedTask){feed=f;updateFeed();}}
    catch(_){}
  }
  async function refresh(){
    if(refreshing)return;refreshing=true;
    try{
      let v;
      try{v=await api('/api/window?project='+encodeURIComponent(project));}
      catch(error){if(Date.now()-clientInfoAt>5000)loadClientInfo();online=false;offlineReason=error.name==='TimeoutError'||error.name==='TypeError'?'Окно потеряло связь с программой Messenger. Закройте окно и откройте снова.':error.message;renderMachines();renderNotice();return;}
      online=true;offlineReason='';if(Date.now()-clientInfoAt>10000)loadClientInfo();
      if(!v.projects.some(p=>p.name===project))project=v.project||'';
      const signature=JSON.stringify(v);
      if(signature!==viewSignature){viewSignature=signature;view=v;if(selectedTask&&!v.tasks.some(t=>t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}render();}
      else{renderMachines();renderNotice();}
      if(selectedTask)await loadFeed();
    }catch(error){notice(el('p','Окно не смогло показать данные: '+error.message));}
    finally{refreshing=false;}
  }
  async function loadDiscovery(){try{const d=await api('/api/discovery',8000);discovery=d.agents||[];discoveryLoaded=true;renderAgents();autoCheck();}catch(_){}}
  function show(next){
    tab=next;root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
    root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('list-toggle').hidden=tab!=='tasks';if(tab==='server')loadServer();$('screen-title').textContent={tasks:'Задачи',agents:'Агенты',project:'Проект',server:'Сервер'}[tab];
  }

  // Server tab: the broker hosted by this client and this computer's
  // connection to a broker. Messages stay next to the section they belong to.
  let server=null,serverBusy=false,serverGen=0,serverPolling=false,serverEdited=false,nameFilled=false,invitation=null,inviteTimer=0,revokeAsk='',revokeTimer=0;
  const notes={host:null,member:null,accept:null},noteTimers={};
  function stamp(at){const d=new Date(at);return isNaN(d.getTime())?'':d.toLocaleString('ru-RU',{day:'numeric',month:'short',hour:'2-digit',minute:'2-digit'});}
  function capital(text){return text?text[0].toUpperCase()+text.slice(1):'';}
  // A status reads like the agents' status: a dot and plain text. state is
  // 'good' (green), 'bad' (red) or '' (a hollow grey dot).
  function badge(node,text,state){node.textContent=text;node.classList.toggle('am-good',state==='good');node.classList.toggle('am-bad',state==='bad');}
  // One button starts or stops the server; its settings are locked while it
  // runs. Both buttons share one grid cell, so the row never changes width.
  function swap(on,off,state){for(const [b,shown] of [[on,state],[off,!state]]){b.setAttribute('aria-hidden',String(!shown));b.tabIndex=shown?0:-1;}}
  function renderHostToggle(){
    if(!server)return;const running=server.host.running;
    swap($('host-disable'),$('host-enable'),running);
    $('host-scope').disabled=running;$('host-port').disabled=running;
    const accept=!!server.accept;$('accept-toggle').setAttribute('aria-checked',String(accept));
  }
  async function loadServer(){
    if(serverBusy||serverPolling)return;
    serverPolling=true;const gen=serverGen;
    try{const next=await api('/api/server');if(gen===serverGen&&!serverBusy){server=next;renderServer();}}
    catch(error){if(gen===serverGen&&!serverBusy){$('host-state').textContent=error.message;$('member-state').textContent='';}}
    finally{serverPolling=false;}
  }
  // A note is the owner's last action result in a section. A success fades;
  // an error stays until the section's state changes or the next action.
  function note(section,text,bad,fade){
    clearTimeout(noteTimers[section]);
    const part=server?.[section];
    notes[section]=text?{text:capital(text),bad:!!bad,state:JSON.stringify(part)}:null;
    if(text&&fade)noteTimers[section]=setTimeout(()=>{notes[section]=null;renderServer();},6000);
    renderNotes();
  }
  function renderNotes(){
    if(server)server.accept_part={error:server.network_error||''};
    for(const section of ['host','member','accept']){
      const n=$(section+'-status'),own=notes[section],part=section==='accept'?server?.accept_part:server?.[section];
      if(own&&own.bad&&part&&JSON.stringify(part)!==own.state&&!serverBusy)notes[section]=null;
      const shown=notes[section]||(part?.error?{text:capital(part.error),bad:true}:null);
      n.textContent=shown?.text||'';n.classList.toggle('am-bad',!!shown?.bad);
    }
  }
  function hideInvitation(){invitation=null;clearInterval(inviteTimer);const d=$('invite-dialog');if(d.open)d.close();}
  function renderServer(){
    if(!server)return;const h=server.host,m=server.member;
    const focused=document.activeElement?.dataset?.key||'';
    badge($('host-badge'),h.running?'Работает':h.enabled?'Не запустился':'Остановлен',h.running?'good':h.enabled?'bad':'');
    // Explanations live in the status tooltip so the card does not change
    // height; only a problem the owner must fix stays visible.
    $('host-badge').title=!h.running?'Другие устройства смогут подключаться к этому компьютеру по одноразовому приглашению.':'Принимает устройства: '+(h.scope==='lan'?'в локальной сети':'только с этого компьютера')+', порт '+h.port+'.';
    $('host-state').textContent=h.running&&!h.addresses.length?'У компьютера нет адреса в локальной сети. Подключитесь к сети или выберите «Только этот компьютер».':'';
    if(!serverEdited){$('host-scope').value=h.scope||'local';$('host-port').value=h.port||47100;}
    $('host-running').hidden=!h.running;
    const select=$('host-address'),current=select.value;
    if(Array.from(select.options).map(o=>o.value).join()!==h.addresses.join()){
      select.replaceChildren(...h.addresses.map(a=>{const o=el('option',a);o.value=a;return o;}));
      if(h.addresses.includes(current))select.value=current;
    }
    // A used or expired invitation is spent: the server no longer lists it.
    if(invitation&&(!h.running||!(h.open_invitations||[]).includes(invitation.id)))hideInvitation();
    // Devices that joined this server are listed in the sidebar.
    renderMachines();
    // The certificate fingerprint travels inside the invitation code and the
    // joining computer checks it itself, so the window does not show it.
    badge($('member-badge'),m.connected?(m.reachable?'На связи':m.checked_at?'Нет связи':'Подключено'):'Не подключено',m.connected?(m.reachable?'good':m.checked_at?'bad':''):'');
    $('member-join').hidden=m.connected;$('member-joined').hidden=!m.connected;
    $('member-badge').title=m.connected?'Этот компьютер входит в сервер '+m.address+'.':'Вставьте код приглашения, который выдал сервер на другом компьютере.';
    $('member-state').textContent='';
    const cn=$('computer-name');if(view?.computer_name&&document.activeElement!==cn&&!cn.dataset.edited)cn.value=view.computer_name;
    const dl=$('member-details');dl.replaceChildren();
    const rows=[['Сервер',m.address],['Подключено',stamp(m.connected_at)]];
    if(m.connected)rows.forEach(([k,v])=>{if(v){dl.append(el('dt',k),el('dd',v));}});
    const buttons=root.querySelectorAll('[data-screen=server] button');buttons.forEach(b=>b.disabled=serverBusy);
    if(!serverBusy)$('host-invite').disabled=!h.addresses.length;
    renderHostToggle();
    syncSelects();renderNotes();
    if(focused&&document.activeElement===document.body)root.querySelector('[data-key="'+focused+'"]')?.focus();
  }
  async function serverAction(body,section,done){
    if(serverBusy)return null;
    serverBusy=true;serverGen++;
    let focused=document.activeElement;
    // Starting and stopping show their result in the status dot only, so the
    // card does not grow and shrink around a passing message.
    note(section,body.action==='connect'||body.action==='check'?'Связываемся с сервером…':'');
    renderServer();
    let result=null;
    try{
      const answer=await postAPI('/api/server-action',body,28000);
      result=answer.result;
      if(body.action!=='invite')server=result;
      if(body.action==='check'){const m=server.member;note(section,m.reachable?'Связь есть, устройство на сервере.':(m.error||'Связи нет.'),!m.reachable,m.reachable);}
      else note(section,done,false,true);
    }catch(error){note(section,error.message,true);}
    finally{
      serverBusy=false;serverGen++;renderServer();
      // Start and stop swap places: keep focus on the one now shown.
      if(focused?.getAttribute('aria-hidden')==='true')focused=focused.parentElement.querySelector('[aria-hidden=false]')||focused;
      if(focused?.isConnected&&!focused.hidden&&focused.offsetParent&&focused.getAttribute('aria-hidden')!=='true')focused.focus();
      else $(section+'-title')?.focus();
    }
    return result;
  }
  async function enableServer(){
    const port=Number($('host-port').value);
    if(!Number.isInteger(port)||port<1024||port>65535){note('host','Порт должен быть от 1024 до 65535.',true);return;}
    const scope=$('host-scope').value;
    if(await serverAction({action:'enable',scope,port},'host','')){
      serverEdited=false;hideInvitation();
      if(scope==='lan')note('host',server.host.addresses.length?'Сервер запущен и слушает локальную сеть. Доступ с другого компьютера зависит от брандмауэра.':'Сервер запущен, но адреса в локальной сети у компьютера нет.',false,true);
      renderServer();
    }
  }
  async function invite(){
    const code=await serverAction({action:'invite',address:$('host-address').value},'host','');
    if(!code)return;
    note('host','');invitation=code;
    $('invite-code').textContent=code.code;copyState('');
    clearInterval(inviteTimer);
    const tick=()=>{const left=Math.ceil((new Date(code.expires_at)-Date.now())/60000);if(left<=0){hideInvitation();return;}$('invite-expiry').textContent='Действует ещё '+left+' мин., один раз.';};
    tick();inviteTimer=setInterval(tick,15000);
    $('invite-dialog').showModal();$('invite-copy').focus();
    // The invitation is not in the status yet; show it until the next poll.
    server.host.open_invitations=[...(server.host.open_invitations||[]),code.id];
  }
  async function connectServer(){
    // The computer joins under its one name, «Имя в сети».
    const code=$('member-code').value.trim(),name=$('computer-name').value.trim()||view?.computer_name||'';
    if(!code){note('member','Вставьте код приглашения.',true);return;}
    if(!name){note('member','Укажите имя в сети.',true);return;}
    if(await serverAction({action:'connect',code,name},'member','Подключено.'))$('member-code').value='';
  }
  // «Новый проект» opens a dialog; the current project is set up on its own
  // tab «Проект» (T-064).
  function openProjectDialog(){
    $('project-name').value='';$('project-folder').value='';$('project-rules').value='';$('project-status').textContent='';
    $('project-dialog').showModal();
  }
  async function createProject(){
    const name=$('project-name').value.trim(),folder=$('project-folder').value.trim();
    if(!name||!folder){$('project-status').textContent='Укажите название и папку.';return;}
    $('project-save').disabled=true;$('project-status').textContent='Сохраняем…';
    try{
      await postAPI('/api/project-create',{name,folder});
      const rules=$('project-rules').value.trim();
      if(rules)await postAPI('/api/project-rules',{name,rules});
      project=name;remember('messenger-project',name);
      $('project-dialog').close();viewSignature='';await refresh();
      // Connect the coordinator right away; the result is shown, not retried.
      if(view?.project===name&&!view.coordinator?.connected&&view.coordinator_link?.state!=='connected')connectCoordinator(name);
    }catch(error){$('project-status').textContent=error.message;}
    finally{$('project-save').disabled=false;}
  }
  // The tab saves by itself (owner): the instructions shortly after typing
  // stops, the folder when the field is left, Enter is pressed or a folder is
  // picked, so a half-typed path is not stored. Saves go one at a time; a
  // failed or unanswered save is shown and not repeated; a good one shows
  // nothing (T-064).
  let projectPageFor='',projectSaving=Promise.resolve(),rulesTimer=0;
  function renderProjectPage(){
    const name=view?.project||'';
    $('project-none').hidden=!!name;$('project-page').hidden=!name;
    if(!name){projectPageFor='';return;}
    if(projectPageFor!==name){$('project-page-status').textContent='';clearTimeout(rulesTimer);}
    const fresh=projectPageFor!==name;projectPageFor=name;
    if(fresh||document.activeElement!==$('project-page-name'))$('project-page-name').value=view.project_title||name;
    // Other computers and the coordinator chat know the project by the name it
    // was created with; a rename shows it so the owner can match them (T-065).
    const renamed=!!view.project_title&&view.project_title!==name;$('project-page-key').hidden=!renamed;
    $('project-page-key').textContent=renamed?'Для других компьютеров и координатора проект по-прежнему называется «'+name+'».':'';
    if(fresh||document.activeElement!==$('project-page-folder'))$('project-page-folder').value=view.folder||'';
    if(fresh||(document.activeElement!==$('project-page-rules')&&!rulesTimer))$('project-page-rules').value=view.worker_rules||'';
  }
  function saveProjectField(field){
    const name=view?.project;if(!name)return;
    const status=$('project-page-status');
    projectSaving=projectSaving.then(async()=>{
      if(view?.project!==name)return;
      try{
        if(field==='title'){
          const title=$('project-page-name').value.trim();
          if(!title){status.textContent='Укажите название.';return;}
          if(title===(view.project_title||name))return;
          status.textContent='';await postAPI('/api/project-title',{name,title});
        }else if(field==='folder'){
          const folder=$('project-page-folder').value.trim();
          if(!folder){status.textContent='Укажите папку.';return;}
          if(folder===(view.folder||''))return;
          status.textContent='';await postAPI('/api/project-folder',{name,folder});
        }else{
          const rules=$('project-page-rules').value.trim();
          if(rules===(view.worker_rules||''))return;
          status.textContent='';await postAPI('/api/project-rules',{name,rules});
        }
        viewSignature='';await refresh();
      }catch(error){status.textContent=error.message;}
    });
  }
  // Removing a project asks in a popup first. Its tasks stay in the database;
  // creating the project again brings them back.
  function askRemoveProject(){
    const name=view?.project;if(!name)return;
    askConfirm('Удалить проект «'+(view.project_title||name)+'»?','Проект пропадёт из списка. История задач сохранится и вернётся, если создать проект с тем же именем.','Удалить',async()=>{
      await postAPI('/api/project-remove',{name});
      project='';remember('messenger-project','');selectedTask='';feed=null;feedSignature='';viewSignature='';projectPageFor='';await refresh();
    });
  }
  // «Установить» opens the agent's official page; «Войти» opens a terminal
  // with the agent's own sign-in (D-057).
  // A row note shows for a while, then the row offers its action again.
  function noteRow(key,text,ms){rowNote.set(key,text);renderAgents();if(ms)setTimeout(()=>{if(rowNote.get(key)===text){rowNote.delete(key);renderAgents();}},ms);}
  async function installAgent(provider){
    noteRow(provider+':program','Открываем страницу…');
    try{await postAPI('/api/agent-install',{provider});noteRow(provider+':program','Страница установки открыта',8000);}
    catch(error){rowNote.delete(provider+':program');renderAgents();showError(error);}
  }
  // After «Войти» or «Выйти» the window checks the agent again when the owner
  // comes back to it, so the card shows the new state without a click.
  const authOpened=new Map();
  async function loginFromCard(provider,cli,environment){
    noteRow(provider+':auth','Открываем вход…');
    try{await postAPI('/api/agent-login',{provider,cli,environment});authOpened.set(provider,Date.now());noteRow(provider+':auth','Окно входа открыто',15000);}
    catch(error){rowNote.delete(provider+':auth');renderAgents();showError(error);}
  }
  function logoutFromCard(provider,cli,environment){
    askConfirm('Выйти из '+names[provider]+'?','Агент на этом компьютере перестанет брать задачи, пока вы снова не войдёте.','Выйти',async()=>{
      await postAPI('/api/agent-logout',{provider,cli,environment});authOpened.set(provider,Date.now());noteRow(provider+':auth','Окно выхода открыто',15000);
    });
  }
  window.addEventListener('focus',()=>{
    for(const [provider,at] of authOpened){if(Date.now()-at<10*60*1000&&!checking.has(provider))checkCard(provider,true);else authOpened.delete(provider);}
  });
  async function loginAgent(provider,cli,environment,button,status){
    button.disabled=true;
    try{
      await postAPI('/api/agent-login',{provider,cli,environment});
      const text='Окно входа открыто. Войдите и нажмите «Проверить».';
      if(status)status.textContent=text;else button.textContent='Окно входа открыто';
    }catch(error){if(status)status.textContent=error.message;else showError(error);}
    finally{setTimeout(()=>{button.disabled=false;},3000);}
  }
  let setupProvider='';
  function openAgentSetup(provider){
    setupProvider=provider;const a=(view?.agents||[]).find(x=>x.agent===provider),found=discovery.find(d=>d.provider===provider);
    $('agent-setup-title').textContent='Настройки: '+names[provider];
    $('agent-setup-note').hidden=provider!=='cursor';
    $('agent-cli').value=a?.cli||found?.cli||'';$('agent-profile').value=a?.environment||found?.environment||'';
    $('agent-models').value=(a?.models||[]).join('\n');
    $('agent-models-note').textContent=provider==='claude'?'Claude сам принимает opus, sonnet и haiku. Сюда можно добавить полные названия моделей Claude.':'Пишите названия точно так, как их принимает сам агент. Координатор сможет выбрать только модель из этого списка; если список пуст, задачи идут на модели по умолчанию.';
    setStatus('agent-check-status','');setStatus('agent-save-status','');$('agent-login').hidden=true;
    $('agent-remove').hidden=!a;
    $('agent-setup').showModal();
  }
  // One check at a time: the client refuses a second one while one runs.
  let checkQueue=Promise.resolve();
  function runCheck(selection){
    const run=checkQueue.then(async()=>{
      const response=await fetch('/api/check-cli',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(selection),cache:'no-store',signal:AbortSignal.timeout(22000)});
      if(!response.ok)throw Error(response.status===409?'Проверка уже выполняется.':response.status===422?'Проверьте путь к программе и профилю.':'Проверка не выполнена.');
      const result=await response.json();if(result.ok!==true)throw Error('Ответ проверки не подтверждён.');
      agentChecks.set(selection.provider,result.check);return result.check;
    });
    checkQueue=run.catch(()=>{});return run;
  }
  function agentPaths(provider){
    const a=(view?.agents||[]).find(x=>x.agent===provider),found=discovery.find(d=>d.provider===provider);
    return {provider,cli:a?.cli||found?.cli||'',environment:a?.environment||found?.environment||''};
  }
  // A card check needs only the program: an empty profile is the agent's
  // standard one (T-056). A quiet check shows no error; the card offers «Войти».
  async function checkCard(provider,quiet){
    const selection=agentPaths(provider);if(!selection.cli)return;
    checking.add(provider);renderAgents();
    try{rowNote.delete(provider+':auth');await runCheck(selection);}catch(error){agentChecks.delete(provider);if(!quiet)showError(error);}
    finally{checking.delete(provider);renderAgents();}
  }
  // The window checks the agents by itself once it knows them, so the cards
  // show the version and login without a click.
  function autoCheck(){
    if(autoChecked||!view||!discoveryLoaded)return;autoChecked=true;
    for(const provider of ['claude','codex','cursor']){const s=agentPaths(provider);if(s.cli&&!agentChecks.has(provider))checkCard(provider,true);}
  }
  // «Подключить» on a card: the found program and profile, then a check.
  async function connectAgent(provider){
    const found=discovery.find(d=>d.provider===provider);if(!found)return;
    noteRow(provider+':program','Подключаем…');
    try{await postAPI('/api/agent-set',{provider,cli:found.cli,environment:''});viewSignature='';await refresh();checkCard(provider);}
    catch(error){showError(error);}
    finally{rowNote.delete(provider+':program');renderAgents();}
  }
  // The dialog's statuses sit on the line of their button and replace each
  // other there, so the dialog never grows or jumps (owner, T-056).
  function setStatus(id,text,cls){const n=$(id);n.textContent=text;n.title=text;n.className='am-check-status'+(cls?' '+cls:'');}
  async function checkAgent(){
    const selection={provider:setupProvider,cli:$('agent-cli').value.trim(),environment:$('agent-profile').value.trim()};
    if(!selection.cli){setStatus('agent-check-status','Укажите программу.','am-status-bad');return;}
    $('agent-check').disabled=true;$('agent-login').hidden=true;setStatus('agent-check-status','Проверяем…');
    try{
      const check=await runCheck(selection),auth=check.authentication?.state;
      const version=check.version?.value?'версия '+check.version.value:'версия не определена';
      setStatus('agent-check-status',(auth==='authenticated'?'Вход выполнен':'Вход не выполнен')+', '+version,auth==='authenticated'?'am-status-good':'am-status-bad');
      $('agent-login').hidden=auth==='authenticated';renderAgents();
    }catch(error){setStatus('agent-check-status',error.name==='TimeoutError'?'Проверка не завершилась вовремя.':error.message,'am-status-bad');}
    finally{$('agent-check').disabled=false;}
  }
  // Every change saves by itself (owner, T-056): the program and profile when
  // a field is left or a path is chosen, then a check; the models when their
  // field is left. The result shows on the line at the bottom.
  async function saveAgentPaths(){
    const cli=$('agent-cli').value.trim();if(!cli)return;
    const found=discovery.find(d=>d.provider===setupProvider);
    let environment=$('agent-profile').value.trim();if(found&&environment===found.environment)environment='';
    const a=(view?.agents||[]).find(x=>x.agent===setupProvider);
    if(a&&a.cli===cli&&(a.environment||'')===environment)return;
    setStatus('agent-save-status','Сохраняем…');
    try{await postAPI('/api/agent-set',{provider:setupProvider,cli,environment});
      setStatus('agent-save-status','Сохранено','am-status-good');$('agent-remove').hidden=false;viewSignature='';await refresh();checkAgent();}
    catch(error){setStatus('agent-save-status',error.message,'am-status-bad');}
  }
  async function saveAgentModels(){
    const a=(view?.agents||[]).find(x=>x.agent===setupProvider);if(!a)return;
    const models=$('agent-models').value.split('\n').map(m=>m.trim()).filter(Boolean);
    if(models.join('\n')===(a.models||[]).join('\n'))return;
    setStatus('agent-save-status','Сохраняем…');
    try{await postAPI('/api/agent-models',{provider:setupProvider,models});setStatus('agent-save-status','Сохранено','am-status-good');viewSignature='';await refresh();}
    catch(error){setStatus('agent-save-status',error.message,'am-status-bad');}
  }
  async function removeAgent(){
    $('agent-remove').disabled=true;setStatus('agent-save-status','');
    try{await postAPI('/api/agent-remove',{provider:setupProvider});agentChecks.delete(setupProvider);viewSignature='';await refresh();$('agent-setup').close();}
    catch(error){setStatus('agent-save-status',error.message,'am-status-bad');}
    finally{$('agent-remove').disabled=false;}
  }
  window.messengerRefresh=()=>{viewSignature='';refresh();};
  // «Выбрать»: the local server opens the system's own dialog (D-053); the
  // Mac app keeps its native folder dialog for the project folder.
  async function pick(field,kind,status){
    const input=$(field);$(status).textContent='';
    try{
      const response=await fetch('/api/pick',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({kind,current:input.value.trim()}),cache:'no-store',signal:AbortSignal.timeout(610000)});
      let result={};try{result=await response.json();}catch(_){}
      if(!response.ok||result.ok!==true)throw Error(result.detail||'Окно выбора не открылось. Вставьте путь вручную.');
      if(result.path&&result.path!==input.value){input.value=result.path;input.dispatchEvent(new Event('change'));}
      $(status).textContent='';
    }catch(error){$(status).textContent=error.name==='TimeoutError'?'Окно выбора не ответило. Вставьте путь вручную.':error.message;}
  }
  // The Mac shell's folder picker answers here, into the field that asked.
  let folderTarget='project-folder';
  window.messengerFolderPicked=path=>{if(typeof path==='string'&&path){$(folderTarget).value=path;$(folderTarget).dispatchEvent(new Event('input'));}};
  window.messengerUpdateStatus = status => {
    if(!window.webkit?.messageHandlers?.updates)return;
    const version=typeof status?.version==='string'?status.version:'';
    const installed=typeof status?.installedVersion==='string'?status.installedVersion:'';
    $('app-installed-version').hidden=!installed;
    const preview=installed.match(/^0\.0\.0-preview\.(\d+)$/);
    $('app-installed-version').textContent=preview?'Предварительная версия '+preview[1]:installed?'Версия '+installed:'';
    $('app-installed-version').title=installed?'Точная версия: '+installed:'';
    const detail=typeof status?.detail==='string'?status.detail:'';
    const waiting=detail==='Обновление загружается в фоне';
    // Only an available update is shown, as the one button «Обновить».
    // Background checks and their failures stay quiet; the app checks again
    // by itself.
    const shown=!!version||waiting;
    $('app-update-version').hidden=true;$('app-update-button').hidden=!shown;
    $('app-update-button').title=version?'Версия '+version:'';
    $('app-update-button').textContent=waiting?'Загружается…':'Обновить';
    $('app-update-button').setAttribute('aria-label',version?'Установить обновление '+version:'Обновление загружается');
    $('app-update-button').disabled=waiting;
  };
  // Windows and Linux: the program updates itself from the window (D-048).
  // The update is downloaded and verified in the background; only an update
  // ready to install is shown.
  let selfUpdate=null,selfUpdateBusy=false,selfUpdateTimer=0;
  function renderSelfUpdate(){
    const s=selfUpdate;if(!s)return;
    const installed=typeof s.installed_version==='string'?s.installed_version:'';
    const preview=installed.match(/^0\.0\.0-preview\.(\d+)$/);
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').textContent=preview?'Предварительная версия '+preview[1]:installed?'Версия '+installed:'';
    $('app-installed-version').title=installed?'Точная версия: '+installed:'';
    const version=typeof s.version==='string'?s.version:'';
    const done=s.state==='installed',shown=(s.state==='available'&&!!version)||selfUpdateBusy||done||!!s.message;
    // Only the button «Обновить» while an update waits (owner, T-056); a line
    // appears only for an error or after the update.
    $('app-update-version').hidden=!s.message&&!done;$('app-update-button').hidden=!shown||done;
    $('app-update-version').textContent=s.message||(done?'Обновлено. Окно откроется заново.':'');
    $('app-update-button').title=version?'Версия '+version:'';
    $('app-update-button').textContent=selfUpdateBusy?'Обновляется…':'Обновить';
    $('app-update-button').setAttribute('aria-label',version?'Установить обновление '+version:'Установить обновление');
    $('app-update-button').disabled=selfUpdateBusy;
  }
  async function loadSelfUpdate(){
    if(window.webkit?.messageHandlers?.updates||selfUpdateBusy)return;
    try{const s=await api('/api/self-update');selfUpdate={...s,message:selfUpdate?.message||''};renderSelfUpdate();}
    catch(_){if(!selfUpdate&&selfUpdateTimer){clearInterval(selfUpdateTimer);selfUpdateTimer=0;}}
  }
  async function installSelfUpdate(){
    if(selfUpdateBusy)return;
    selfUpdateBusy=true;if(selfUpdate)selfUpdate.message='';renderSelfUpdate();
    try{const r=await postAPI('/api/self-update',{action:'install'},65000);selfUpdate={...selfUpdate,state:r.state,version:r.version,message:''};}
    catch(error){if(selfUpdate)selfUpdate.message=error.message;}
    finally{selfUpdateBusy=false;renderSelfUpdate();}
  }
  $('app-update-button').addEventListener('click',()=>{
    if(window.webkit?.messageHandlers?.updates)window.webkit.messageHandlers.updates.postMessage('check');
    else installSelfUpdate();
  });
  if(!window.webkit?.messageHandlers?.updates){selfUpdateTimer=setInterval(loadSelfUpdate,60000);loadSelfUpdate();}
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('list-toggle').addEventListener('click',()=>{
    const expanded=$('list-toggle').getAttribute('aria-expanded')!=='true';
    $('list-toggle').setAttribute('aria-expanded',String(expanded));
    const label=expanded?'Скрыть список задач':'Показать список задач';$('list-toggle').setAttribute('aria-label',label);$('list-toggle').title=label;
    $('task-panel').hidden=!expanded;$('task-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  $('project-select').addEventListener('change',()=>{
    const value=$('project-select').value;
    if(value==='__new'){$('project-select').value=view?.project||'__new';syncSelects();openProjectDialog();return;}
    project=value;remember('messenger-project',value);selectedTask='';feed=null;feedSignature='';viewSignature='';refresh();
  });
  $('agent-filter').addEventListener('change',renderTaskList);
  $('computer-chip-clear').addEventListener('click',()=>{computerFilter='';renderMachines();renderTaskList();});
  // The sidebar and the task list are as wide as the owner drags them (T-063).
  // A double click or Home restores the usual width; the width is remembered.
  function splitter(pane,name,variable,usual,min,max,side){
    const handle=el('div',undefined,'am-splitter');
    handle.setAttribute('role','separator');handle.setAttribute('aria-orientation','vertical');handle.tabIndex=0;
    handle.title='Потяните, чтобы изменить ширину. Двойной щелчок: как было';
    handle.setAttribute('aria-label',name);handle.setAttribute('aria-valuemin',String(min));handle.setAttribute('aria-valuemax',String(max));
    const key='messenger-width-'+variable.slice(5);
    const apply=(px,save)=>{px=Math.round(Math.min(max,Math.max(min,px)));root.style.setProperty(variable,px+'px');handle.setAttribute('aria-valuenow',String(px));if(save)remember(key,px===usual?'':String(px));};
    const saved=Number(recall(key));apply(saved>0?saved:usual,false);
    handle.addEventListener('pointerdown',event=>{
      if(event.button!==0)return;event.preventDefault();handle.setPointerCapture(event.pointerId);
      const startX=event.clientX,startW=pane.getBoundingClientRect().width;
      handle.classList.add('am-dragging');document.body.classList.add('am-resizing');
      const move=e=>apply(startW+side*(e.clientX-startX),false);
      const up=e=>{handle.removeEventListener('pointermove',move);handle.removeEventListener('pointerup',up);handle.removeEventListener('pointercancel',up);handle.classList.remove('am-dragging');document.body.classList.remove('am-resizing');apply(startW+side*(e.clientX-startX),true);};
      handle.addEventListener('pointermove',move);handle.addEventListener('pointerup',up);handle.addEventListener('pointercancel',up);
    });
    handle.addEventListener('dblclick',()=>apply(usual,true));
    handle.addEventListener('keydown',event=>{
      const now=pane.getBoundingClientRect().width,step=event.shiftKey?64:16;
      if(event.key==='ArrowLeft')apply(now-side*step,true);else if(event.key==='ArrowRight')apply(now+side*step,true);else if(event.key==='Home')apply(usual,true);else return;
      event.preventDefault();
    });
    pane.append(handle);
  }
  splitter(root.querySelector('.am-sidebar'),'Ширина левой панели','--am-nav',204,160,420,1);
  splitter($('task-panel'),'Ширина списка задач','--am-queue',240,200,520,-1);
  // A click on empty space in the sidebar shows all computers' tasks again.
  root.querySelector('.am-sidebar').addEventListener('click',event=>{if(computerFilter&&!event.target.closest('.am-machine-card,.am-project,.am-side-bottom,.am-splitter,button,select,input,a,[popover]')){computerFilter='';renderMachines();renderTaskList();}});
  $('answer-text').addEventListener('input',()=>{const f=$('answer-text');f.style.height='auto';f.style.height=Math.min(140,Math.max(44,f.scrollHeight))+'px';answerControls();});
  $('answer-send').addEventListener('click',sendAnswer);
  // Enter sends; Shift+Enter adds a line. An IME composition keeps its Enter.
  $('task-stop').addEventListener('click',stopTask);
  $('task-stop').addEventListener('mouseleave',()=>$('task-stop').classList.remove('am-tip-off'));
  document.addEventListener('keydown',e=>{if(e.key==='Escape'&&!e.isComposing&&!$('task-stop').hidden&&!$('compose').hidden&&!document.querySelector('dialog[open]')&&!e.target.closest?.('[role=listbox],[aria-expanded=true]')){e.preventDefault();stopTask();}});
  $('answer-text').addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing&&e.keyCode!==229){e.preventDefault();sendAnswer();}});
  $('project-dialog-close').addEventListener('click',()=>$('project-dialog').close());
  $('invite-close').addEventListener('click',()=>$('invite-dialog').close());
  // A closed invitation is not shown again; a new one can be made at any time.
  $('invite-dialog').addEventListener('close',()=>{invitation=null;clearInterval(inviteTimer);});
  $('project-save').addEventListener('click',createProject);
  $('project-page-rules').addEventListener('input',()=>{clearTimeout(rulesTimer);rulesTimer=setTimeout(()=>{rulesTimer=0;saveProjectField('rules');},800);});
  $('project-page-rules').addEventListener('blur',()=>{if(rulesTimer){clearTimeout(rulesTimer);rulesTimer=0;saveProjectField('rules');}});
  $('project-page-name').addEventListener('change',()=>saveProjectField('title'));
  $('project-page-name').addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();saveProjectField('title');}});
  $('project-page-folder').addEventListener('change',()=>saveProjectField('folder'));
  $('project-page-folder').addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();saveProjectField('folder');}});
  $('project-page-remove').addEventListener('click',askRemoveProject);
  $('confirm-go').addEventListener('click',runConfirm);
  $('confirm-cancel').addEventListener('click',()=>$('confirm-dialog').close());
  $('access-toggle').addEventListener('click',toggleAllAccess);
  $('project-pick').addEventListener('click',()=>{folderTarget='project-folder';if(window.webkit?.messageHandlers?.folder)window.webkit.messageHandlers.folder.postMessage('pick');else pick('project-folder','folder','project-status');});
  $('project-page-pick').addEventListener('click',()=>{folderTarget='project-page-folder';if(window.webkit?.messageHandlers?.folder)window.webkit.messageHandlers.folder.postMessage('pick');else pick('project-page-folder','folder','project-page-status');});
  $('agent-cli-pick').addEventListener('click',()=>pick('agent-cli','file','agent-check-status'));
  $('agent-profile-pick').addEventListener('click',()=>pick('agent-profile','folder','agent-check-status'));
  $('restart-dialog').addEventListener('cancel',event=>event.preventDefault());
  $('restart-go').addEventListener('click',()=>restartClient(!!restart.ask));
  $('restart-quit').addEventListener('click',()=>window.webkit?.messageHandlers?.app?.postMessage('quit'));
  $('restart-copy').addEventListener('click',async()=>{const b=$('restart-copy');try{await navigator.clipboard.writeText(shutdownCommand());b.textContent='Скопировано';}catch(_){b.textContent='Не удалось скопировать';}setTimeout(()=>b.textContent='Скопировать команду',2000);});
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-remove').addEventListener('click',removeAgent);
  $('agent-check').addEventListener('click',checkAgent);
  $('agent-cli').addEventListener('change',saveAgentPaths);
  $('agent-profile').addEventListener('change',saveAgentPaths);
  $('agent-models').addEventListener('change',saveAgentModels);
  $('host-scope').addEventListener('change',()=>{serverEdited=true;});
  $('host-port').addEventListener('input',()=>{serverEdited=true;});
  $('host-enable').addEventListener('click',enableServer);
  $('host-disable').addEventListener('click',()=>serverAction({action:'disable'},'host','').then(r=>{if(r){serverEdited=false;hideInvitation();renderServer();}}));
  $('host-invite').addEventListener('click',invite);
  // The copy icon turns into a check mark once the code is on the clipboard.
  function copyState(state){const b=$('invite-copy');b.toggleAttribute('data-copied',state==='done');
    const label=state==='done'?'Скопировано':state==='failed'?'Не удалось скопировать — выделите код вручную':'Скопировать';
    b.title=label;b.setAttribute('aria-label',state?label:'Скопировать код');}
  $('invite-copy').addEventListener('click',async()=>{try{await navigator.clipboard.writeText($('invite-code').textContent);copyState('done');}catch(_){copyState('failed');}});
  $('member-connect').addEventListener('click',connectServer);
  $('agent-login').addEventListener('click',()=>loginAgent(setupProvider,$('agent-cli').value.trim(),$('agent-profile').value.trim(),$('agent-login'),$('agent-check-status')));
  $('computer-name').addEventListener('input',e=>{e.target.dataset.edited='1';renderServer();});
  // «Имя в сети» saves by itself when the field is left or Enter is pressed;
  // only an error is shown.
  async function renameComputer(){const cn=$('computer-name'),name=cn.value.trim();if(!name||name===view?.computer_name||!cn.dataset.edited)return;if(await serverAction({action:'rename',name},'computer','')){delete cn.dataset.edited;await refresh();renderServer();}}
  $('computer-name').addEventListener('change',renameComputer);
  $('computer-name').addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();renameComputer();}});
  $('accept-toggle').addEventListener('click',()=>{if(!server)return;const on=!server.accept;serverAction({action:'accept',on},'accept',on?'':'Новые задачи других компьютеров будут отклоняться. Уже принятые продолжат работу.');});
  $('member-disconnect').addEventListener('click',()=>serverAction({action:'disconnect'},'member','Отключено. Устройство остаётся в списке сервера, пока его там не отзовут.'));
  let serverPolls=0;
  // The sidebar lists devices from the server state, so it is read on every
  // tab, every 15 s outside «Сервер» (as often as computers report).
  setInterval(()=>{serverPolls++;if(tab==='server'||serverPolls%3===0)loadServer();},5000);
  loadServer();
  let initialTheme=window.messengerInitialTheme;
  if(!initialTheme)try{initialTheme=localStorage.getItem('messenger-theme');}catch(_){}
  enhanceSelect($('theme-select'));enhanceSelect($('project-select'));enhanceSelect($('agent-filter'));enhanceSelect($('host-scope'));enhanceSelect($('host-address'));
  centreControlText();
  applyTheme(initialTheme||'system');
  matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change',()=>{if(document.documentElement.dataset.theme==='system')applyTheme('system');});
  $('theme-select').addEventListener('change',()=>applyTheme($('theme-select').value,true));
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
  const chatBody=root.querySelector('.am-chat-body'),chatFooter=root.querySelector('.am-chat-footer');
  new ResizeObserver(()=>chatBody.style.setProperty('--am-footer-height',chatFooter.getBoundingClientRect().height+'px')).observe(chatFooter);
  show('tasks');refresh();loadDiscovery();
  setInterval(refresh,2000);setInterval(loadDiscovery,60000);
})();
