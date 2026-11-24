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
    const label=select.labels[0]?.textContent||select.getAttribute('aria-label')||tr('Выбор');
    trigger.setAttribute('aria-label',label);trigger.setAttribute('aria-haspopup','listbox');trigger.setAttribute('aria-expanded','false');
    const value=document.createElement('span');value.className='am-select-value';
    const chevron=document.createElement('span');chevron.className='am-select-chevron';chevron.setAttribute('aria-hidden','true');trigger.append(value,chevron);
    const menu=document.createElement('div');menu.id=select.id+'-menu';menu.className='am-select-menu';menu.setAttribute('popover','auto');menu.setAttribute('role','listbox');menu.setAttribute('aria-label',label);menu.tabIndex=-1;
    // A modal dialog makes outside nodes inert, even top-layer popovers.
    // Keep its menus inside the dialog so pointer and keyboard input reach them.
    trigger.setAttribute('aria-controls',menu.id);select.after(trigger);(select.closest('dialog')||root).append(menu);select.hidden=true;
    select.labels[0]?.addEventListener('click',event=>{event.preventDefault();trigger.focus();});
    let options=[];
    const isOpen=()=>menu.matches(':popover-open');
    function close(restore=false){if(isOpen())menu.hidePopover();if(restore)trigger.focus();}
    function sync(){value.textContent=select.selectedOptions[0]?.textContent||tr('Нет данных');trigger.disabled=select.disabled;if(select.disabled)close();}
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
    if(!providerPaths[provider]){wrap.textContent=provider==='owner'?tr('Я'):'→';return wrap;}
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
  let view=null,project=recall('messenger-project'),selectedTask='',selectedShared=null,feed=null,feedSignature='',viewSignature='';
  // T-133: the task shown last, and whether the feed should go to its end on
  // the next render (a new task opened, or the owner just wrote).
  let feedShown='',feedStick=false,feedPinned=true,feedCount=0;
  let online=false,offlineReason='',discovery=[],tab='tasks',refreshing=false,answering=false;
  const agentChecks=new Map(),checking=new Set(),rowNote=new Map();let autoChecked=false,discoveryLoaded=false;
  let clientInfo=null,clientInfoAt=0,restart={busy:false,ask:null,error:'',wait:false},autoRestarted=false;
  // The start screen covers the window until the client first answers, for
  // at most bootLimit; then the usual notices take over (T-079).
  let booting=true;const bootLimit=20000;setTimeout(()=>{booting=false;renderBoot();},bootLimit);
  function renderBoot(){
    const updating=restart.busy&&!restart.ask&&!restart.wait;
    $('boot-cover').hidden=!(booting||updating);
    $('boot-text').textContent=updating?tr('Устанавливаем обновление…'):tr('Запуск…');
  }
  // The window can be newer than the client still running in the background
  // (after an update). Say how to restart it; nothing is restarted by itself.
  // Values in a copied command are quoted for the shell it is pasted into:
  // single quotes on macOS and Linux (an inner ' becomes '\''), double
  // quotes on Windows, where a path cannot contain ".
  function shellQuote(value){
    value=String(value);
    if(window.amDesktop==='windows'||/Windows/.test(navigator.userAgent))return '"'+value+'"';
    return "'"+value.replace(/'/g,"'\\''")+"'";
  }
  function shutdownCommand(){return view?.messenger&&view?.data_root?shellQuote(view.messenger)+' --root '+shellQuote(view.data_root)+' shutdown':'';}
  function outdatedMessage(){
    if(Date.now()-clientInfoAt>3000)loadClientInfo();
    if(clientInfo?.restart)return Error(tr('В фоне старая версия Messenger.'));
    const e=Error(tr('В фоне старая версия клиента, перезапустите его'));
    e.hint=tr('Закройте Messenger, выполните скопированную команду в Терминале и откройте Messenger снова.');
    e.command=shutdownCommand();
    return e;
  }
  // An error of the owner's action stays at the top until it is closed or
  // the next action succeeds; the 2 s refresh does not hide it.
  let actionError=null;
  function showError(error){actionError=error?{text:error.message,command:error.command||'',hint:error.hint||''}:null;renderToasts();}
  // A small chip above the input field, like a chat's status chips.
  function renderToasts(){
    const box=$('toasts');
    // An agent action must not report its error inside the hidden Tasks tab.
    if(tab==='tasks')$('notices').after(box);
    else root.querySelector('.am-main').insertBefore(box,root.querySelector('[data-screen]'));
    box.replaceChildren();box.hidden=!actionError;if(!actionError)return;
    const chip=el('div',undefined,'am-toast');chip.append(el('span',actionError.text,'am-toast-text'));if(actionError.hint)chip.title=actionError.hint;
    if(actionError.command){
      const command=actionError.command,copy=el('button',tr('Скопировать команду'));copy.type='button';copy.title=command;
      copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent=tr('Скопировано');}catch(_){copy.textContent=tr('Не удалось скопировать');}setTimeout(()=>copy.textContent=tr('Скопировать команду'),2000);});
      chip.append(copy);
    }
    const close=el('button','×','am-toast-close');close.type='button';close.setAttribute('aria-label',tr('Закрыть'));close.addEventListener('click',()=>showError(null));chip.append(close);
    box.append(chip);
  }
  async function api(path,timeout=4500,signal){
    if(!token)throw Error(tr('Откройте приватную ссылку, которую выдала команда ui.'));
    const response=await fetch(path,{headers:{Authorization:'Bearer '+token},cache:'no-store',signal:signal||AbortSignal.timeout(timeout)});
    if(!response.ok){
      if(response.status===401)throw Error(tr('Ссылка доступа устарела. Откройте окно заново.'));
      const body=await response.text().catch(()=>'');
      if(body.startsWith('client_outdated'))throw outdatedMessage();
      if(body.startsWith('client_rejected'))throw Error(tr('Клиент Messenger отклонил запрос окна. Подробности: messenger doctor.'));
      throw Error(tr('Клиент Messenger не запущен или не отвечает. Данные не обновляются.'));
    }
    return response.json();
  }
  async function postAPI(path,body,timeout=6000,keepalive=false){
    let response;
    // No answer means the result is unknown, not that nothing happened.
    try{response=await fetch(path,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(timeout),keepalive});}
    catch(_){throw Error(tr('Ответ не получен, результат неизвестен. Проверьте состояние, прежде чем повторять.'));}
    let result={};try{result=await response.json();}catch(_){}
    if(response.ok&&result.ok===true)return result;
    if(result.state==='client_outdated')throw outdatedMessage();
    if(result.detail){const error=Error(result.detail);error.command=result.command;error.hint=result.hint;throw error;}
    if(response.status===400)throw Error(tr('Проверьте введённые данные.'));
    throw Error(tr('Действие не подтверждено. Проверьте состояние и повторите.'));
  }
  function minutes(from,to,now=Date.now()){
    const a=Date.parse(from),b=to?Date.parse(to):now;
    if(isNaN(a)||isNaN(b))return '';
    const m=Math.max(0,Math.round((b-a)/60000));
    if(m<1)return tr('<1 мин');
    if(m<60)return m+tr(' мин');
    const h=Math.floor(m/60);return h+tr(' ч')+(m%60?' '+m%60+tr(' мин'):'');
  }
  function elapsedLabel(from,prefix='',cls=''){
    const label=el('span',prefix+minutes(from),cls);
    label.dataset.elapsedFrom=from;
    label.dataset.elapsedPrefix=prefix;
    return label;
  }
  const taskStart=t=>t.started_at||t.created_at;
  // T-058: what a task spent, as its agent reported it. Claude reports a
  // price; Codex and Cursor only tokens.
  function tokensText(n){
    if(n>=1e6)return (n/1e6).toLocaleString(amLocale,{maximumFractionDigits:1})+tr(' млн');
    if(n>=1e3)return Math.round(n/1e3).toLocaleString(amLocale)+tr(' тыс.');
    return String(n);
  }
  // Tokens only: the agents' price is their own estimate and the owner works
  // on subscription limits, so no dollars are shown (D-074).
  function usageShort(u){
    if(!u)return '';
    return tokensText(u.input_tokens+u.output_tokens)+tr(' токенов');
  }
  function usageLong(u){
    if(!u)return '';
    let s=tr('Потрачено токенов: ')+u.input_tokens.toLocaleString(amLocale)+tr(' на входе');
    if(u.cached_tokens)s+=tr(' (из них ')+u.cached_tokens.toLocaleString(amLocale)+tr(' из кэша)');
    return s+', '+u.output_tokens.toLocaleString(amLocale)+tr(' на выходе');
  }
  function clock(at){const d=new Date(at);return isNaN(d.getTime())?'':d.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});}
  const activeStates=['running','waiting_owner','waiting_coordinator'];
  // Relative dates print the time when recent, or whenever an event ended.
  function sameDay(a,b){return a.toDateString()===b.toDateString();}
  function longDay(d){return d.toLocaleDateString(amLocale,d.getFullYear()===new Date().getFullYear()?{day:'numeric',month:'long'}:{day:'numeric',month:'long',year:'numeric'});}
  function relativeWhen(at,includeClock){
    const d=new Date(at);if(!at||isNaN(d.getTime()))return '';
    const today=new Date(),yesterday=new Date();yesterday.setDate(today.getDate()-1);
    if(sameDay(d,today))return tr('сегодня ')+clock(at);
    if(sameDay(d,yesterday))return tr('вчера ')+clock(at);
    const date=d.toLocaleDateString(amLocale,d.getFullYear()===today.getFullYear()?{day:'numeric',month:'short'}:{day:'numeric',month:'short',year:'numeric'});
    return date+(includeClock?' '+clock(at):'');
  }
  function appendTaskWhen(agent,t){
    const ended=finishedStates.includes(t.state)||(!activeStates.includes(t.state)&&!!t.finished_at);
    const at=ended?t.finished_at:taskStart(t),label=relativeWhen(at,ended);
    if(!label)return;
    const time=el('time',label,'am-task-when');
    time.dateTime=at;time.title=taskWhenTitle(t);agent.append(time);
  }
  function appendEventTime(line,at,remote){
    const label=relativeWhen(at,true);
    if(!label)return;
    const time=el('time',label);
    time.dateTime=at;
    if(remote)time.title=tr('Время события на этом компьютере');
    // T-276: the owner prefers a comma to a dot between words.
    line.append(', ',time);
  }
  function taskWhenTitle(t){
    const d=new Date(t.created_at);if(isNaN(d.getTime()))return '';
    const lines=[tr('Создана ')+longDay(d)+tr(' в ')+clock(t.created_at)];
    if(t.started_at&&t.started_at!==t.created_at){
      const start=new Date(t.started_at);
      if(!isNaN(start.getTime()))lines.push(tr('Начата ')+longDay(start)+tr(' в ')+clock(t.started_at));
    }
    const e=new Date(t.finished_at||'');
    if(t.finished_at&&!isNaN(e.getTime())){
      const verb={failed:tr('Завершилась ошибкой'),interrupted:tr('Прервана'),blocked:tr('Заблокирована'),cancelled:tr('Отменена')}[t.state]||tr('Завершена');
      lines.push(verb+' '+(sameDay(d,e)?'':longDay(e)+' ')+tr('в ')+clock(t.finished_at));
      lines.push(tr('Всего ')+minutes(t.created_at,t.finished_at));
    }
    return lines.join('\n');
  }
  // T-034: why a run failed, as the provider reported it. Nothing is retried.
  function resetText(at){
    const d=new Date(at);if(isNaN(d.getTime()))return '';
    const day=x=>x.getFullYear()+'-'+x.getMonth()+'-'+x.getDate(),today=new Date(),tomorrow=new Date();tomorrow.setDate(today.getDate()+1);
    if(day(d)===day(today))return tr('в ')+clock(at);
    if(day(d)===day(tomorrow))return tr('завтра в ')+clock(at);
    return d.toLocaleDateString([], {day:'numeric',month:'long'})+tr(' в ')+clock(at);
  }
  function failureStatus(t){
    switch(t.failure_reason){
      case 'usage_limit':{const r=t.failure_reset?resetText(t.failure_reset):'';return [r?tr('Лимит, сброс ')+r:tr('Лимит исчерпан'),'am-status-bad'];}
      case 'rate_limit':return [tr('Слишком много запросов'),'am-status-bad'];
      case 'overload':return [tr('Сервис перегружен'),'am-status-bad'];
      case 'sign_in':return [tr('Нужен вход в аккаунт'),'am-status-bad'];
    }
    return null;
  }
  // The status with its trailing comma: the comma stays at the end of the line
  // and keeps the muted colour, the token count wraps to the next line.
  function statusLabel(text,cls,from){return from?elapsedLabel(from,tr('Работает '),cls):el('span',text,cls);}
  function statusPiece(text,comma,from){
    const piece=el('span',from?undefined:text);
    if(from)piece.append(statusLabel(text,'',from));
    if(comma)piece.append(el('span',',','am-task-comma'));
    return piece;
  }
  function taskStatus(t){
    const end=t.finished_at||t.updated_at,from=taskStart(t);
    // T-126: an open task on a computer this one lost contact with.
    if(t.remote?.lost&&!t.remote.from&&activeStates.includes(t.state))
      return [tr('Связь потеряна, исход неизвестен'),'am-muted',t.remote.last_seen?'«'+t.remote.name+tr('» не на связи с ')+stamp(t.remote.last_seen):'«'+t.remote.name+tr('» больше нет в сети')];
    // T-137: sent to another computer that has not said it took the task.
    // It may run there with its replies lost, so neither «Работает» nor
    // «not accepted»: say what is known.
    if(t.remote?.unaccepted&&!t.remote.from&&t.state==='running'){
      if(Date.now()-Date.parse(t.created_at)<5*60*1000)return [tr('Ждёт, когда «')+t.remote.name+tr('» примет'),'am-status-wait'];
      return [tr('Нет ответа от «')+t.remote.name+'»','am-status-bad',tr('приём не подтверждён. Задача может идти там, но её ответы сюда не доходят; сама она не повторяется')];
    }
    switch(t.state){
      case 'running':return t.pending_launch?[t.waits_for_lock?tr('Ждёт ресурс «')+t.waits_for_lock+'»':tr('Ждёт запуска'),'am-status-wait']:[tr('Работает ')+minutes(from),'am-status-run','',from];
      case 'waiting_owner':return [tr('Ждёт вашего ответа'),'am-status-wait'];
      case 'waiting_coordinator':return [tr('Ждёт ответа координатора'),'am-status-run'];
      case 'done':return [tr('Работал ')+minutes(from,end),'am-muted'];
      case 'failed':return failureStatus(t)||[tr('Ошибка через ')+minutes(from,end),'am-status-bad'];
      case 'interrupted':return [tr('Прервана через ')+minutes(from,end),'am-status-bad'];
      case 'blocked':return [tr('Заблокирована через ')+minutes(from,end),'am-status-bad'];
      case 'cancelled':return [tr('Отменена'),'am-muted'];
    }
    return [t.state,'am-muted'];
  }
  // T-131: another computer's task is known only from that computer's
  // reports. While it is out of contact, an open task's last state is
  // stale: say that its outcome is unknown instead of «Работает 10 ч».
  function sharedStatus(t,p){
    if(sharedLost(t,p)){
      const since=p.age>=0?stamp(new Date(Date.now()-p.age*1000).toISOString()):'';
      return [tr('Связь потеряна, исход неизвестен'),'am-muted',since?'«'+p.name+tr('» не на связи с ')+since:'«'+p.name+tr('» больше нет в сети')];
    }
    return taskStatus(t);
  }
  // T-131: a task leaves the list only when it no longer runs; a running
  // one is stopped first. Another computer's task can also go when that
  // computer is out of contact, since its last state is stale.
  const finishedStates=['done','failed','cancelled'];
  function sharedLost(t,p){return activeStates.includes(t.state)&&!(p.age>=0&&p.age<=45);}
  function taskMenu(event,t,p){
    event.preventDefault();
    const ok=p?finishedStates.includes(t.state)||sharedLost(t,p):finishedStates.includes(t.state);
    const tip=ok?'':p?tr('Задача ещё идёт на компьютере «')+p.name+tr('». Удалить её можно, когда она закончится'):tr('Сначала остановите задачу');
    popMenu(event.clientX,event.clientY,[{text:tr('Удалить из списка'),off:!ok,tip,pick:()=>askRemoveTask(t,p)}]);
  }
  function askRemoveTask(t,p){
    const text=p?tr('Задача пропадёт из списка на этом компьютере. У других она останется, пока её не удалят на компьютере «')+p.name+'».'
      :tr('Задача пропадёт из списка на всех компьютерах сети. Её история останется в базе этого компьютера.');
    askConfirm(tr('Удалить задачу из списка?'),text,tr('Удалить'),async()=>{
      await postAPI('/api/task-hide',{peer:p?.peer||'',task:t.id});
      if(p){if(selectedShared?.peer===p.peer&&selectedShared?.id===t.id)selectedShared=null;}
      else{
        if(taskHistory)taskHistory.rows.delete(t.id);
        if(selectedTask===t.id){selectedTask='';feed=null;feedSignature='';}
      }
      viewSignature='';await refresh();
    });
  }
  function coordinatorName(client){
    const c=(client||'').toLowerCase();
    if(c.includes('codex'))return ['codex','Codex'];
    if(c.includes('cursor'))return ['cursor','Cursor'];
    if(c.includes('claude')||!c||c==='coordinator')return ['claude','Claude'];
    return ['',client];
  }
  function agentState(a){
    if(!online)return ['offline',tr('Нет связи')];
    if(a.running>0)return ['run',a.running>1?tr('Работает (')+a.running+')':tr('Работает')];
    if(a.waiting_owner>0)return ['wait',tr('Ждёт ответа')];
    return ['idle',tr('Свободен')];
  }
  function renderProjects(){
    const select=$('project-select');
    const options=view?view.projects.map(p=>[p.name,p.title||p.name]):[['',offlineReason?tr('Проекты недоступны'):tr('Загружаем проекты…')]];
    if(view)options.push(['__new',tr('Новый проект')]);
    if(select.options.length!==options.length||options.some(([value,label],i)=>select.options[i].value!==value||select.options[i].textContent!==label)){
      select.replaceChildren(...options.map(([value,label])=>{
        const option=el('option',label);option.value=value;return option;
      }));
    }
    select.value=view?.project||(view?'__new':'');
    if(!view?.project&&select.options.length>1)select.selectedIndex=0;
    if(select.disabled!==!view)select.disabled=!view;
    syncSelects();
  }
  // Right-click menu on another computer of the network. Revoking cannot be
  // undone, so its item only opens a confirmation inside the card. Updating
  // moved to an icon on the card (T-128).
  // A right-click menu; entries are {text,pick,off,tip}. An entry with off
  // is shown dimmed with its reason as a tooltip.
  function popMenu(x,y,entries){
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
    const items=[];
    for(const it of entries){
      const item=el('div',it.text,'am-select-option');item.setAttribute('role','menuitem');item.tabIndex=0;
      if(it.tip)item.title=it.tip;
      if(it.off)item.setAttribute('aria-disabled','true');
      const run=()=>{if(it.off)return;menu.hidePopover();it.pick();};
      item.addEventListener('click',run);item.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();run();}});
      items.push(item);
    }
    if(!items.length)return;
    menu.replaceChildren(...items);menu.style.left=x+'px';menu.style.top=y+'px';menu.showPopover();items[0].focus();
  }
  function deviceMenu(d,x,y,can){
    const items=[];
    const add=(text,pick)=>items.push({text,pick});
    if(can.revoke)add(tr('Отозвать устройство'),()=>{revokeAsk=d.id;clearTimeout(revokeTimer);revokeTimer=setTimeout(()=>{revokeAsk='';renderMachines();},8000);renderMachines();$('machines').querySelector('[data-key="'+CSS.escape('revoke-yes-'+d.id)+'"]')?.focus();});
    popMenu(x,y,items);
  }
  // Asking another computer to update itself (T-078). Its answers arrive
  // with the server status and show as notices above the reply box (T-128);
  // the card only says while it is under way.
  async function requestUpdate(peer){
    try{const answer=await postAPI('/api/server-action',{action:'update',device:peer.id},28000);if(answer?.result)server=answer.result;}
    catch(error){addNotice('refused-'+peer.id+'-'+Date.now(),'bad',(peer.name||'')+tr(' не обновлён'),capital(error.message));}
    renderMachines();
  }
  // The update under way for a card's icon: asked for or updating.
  function updateLine(peer){
    const u=(server?.updates||[]).find(x=>x.computer===peer.id);
    if(u&&!u.stale&&Date.now()-Date.parse(u.updated_at)<24*3600*1000){
      if(u.state==='waiting')return [tr('Обновление запрошено…'),'am-status-wait'];
      if(u.state==='queued')return [tr('ждёт обновления'),'am-status-wait'];
      if(u.state==='updating')return [tr('Обновляется…'),'am-status-run'];
    }
    if(peer.update_pending)return [tr('ждёт обновления'),'am-status-wait'];
    return null;
  }
  // T-128: notices above the reply box. A good one fades after a few seconds
  // on screen; a bad one stays until the owner closes it, also across
  // reloads of the window. Only answers that came while the window was open
  // are shown as good news; problems of the last day are kept.
  const notices=[],noticeSince=Date.now();
  function closedNotices(){try{return JSON.parse(localStorage.getItem('messenger-notices-closed')||'{}')||{};}catch(_){return {};}}
  // T-181: no scroll of their own; at most noticeLimit are shown and a new
  // one pushes out the oldest, which then counts as closed.
  const noticeLimit=3;
  function closeNotice(key){
    const i=notices.findIndex(n=>n.key===key);if(i>=0){clearTimeout(notices[i].timer);notices.splice(i,1);}
    const closed=closedNotices(),cut=Date.now()-2*24*3600*1000;closed[key]=Date.now();
    for(const k of Object.keys(closed))if(closed[k]<cut)delete closed[k];
    remember('messenger-notices-closed',JSON.stringify(closed));renderNotices();
  }
  function addNotice(key,kind,title,text){
    if(notices.some(n=>n.key===key)||closedNotices()[key])return;
    notices.push({key,kind,title,text:text||''});
    while(notices.length>noticeLimit)closeNotice(notices[0].key);
    renderNotices();
  }
  function collectNotices(){
    // T-129: the background program was started inside an agent's sandbox.
    if(clientInfo?.sandbox)addNotice('sandbox-'+(clientInfo.client_version||''),'bad',tr('Messenger запущен из песочницы агента'),tr('Исполнители унаследуют её: без сети и без части папок, даже с «Любыми командами». Закройте Messenger полностью и откройте его снова сами, не из агента.'));
    const names=Object.fromEntries((server?.peers||[]).map(p=>[p.id,p.name]));
    const showUpdate=(u,name)=>{
      const at=Date.parse(u.updated_at);
      if(!(Date.now()-at<24*3600*1000))return;
      const key='update-'+u.request_id+'-'+(u.stale?'stale':u.state);
      if(u.stale)addNotice(key,'bad',name+tr(' не ответил об обновлении'),tr('Проверьте, что программа там запущена, и попросите ещё раз.'));
      else if(u.state==='done'&&at>=noticeSince-60000){
        const same=/последн/i.test(u.text||'');
        addNotice(key,same?'info':'good',same?name+tr(': уже последняя версия'):name+tr(' обновлён до ')+versionName(u.version||''),same?'':u.text||'');
      }else if(u.state==='queued'&&at>=noticeSince-60000)addNotice(key,'info',name+tr(' ждёт обновления'),u.text||tr('Программа обновится, когда задачи закончатся; до тех пор компьютер не берёт новых задач.'));
      else if(u.state==='refused'||u.state==='failed')addNotice(key,'bad',name+tr(' не обновлён'),capital(u.text||''));
    };
    for(const u of server?.updates||[])showUpdate(u,names[u.computer]||tr('Компьютер'));
    if(view?.local_update)showUpdate(view.local_update,view.computer_name||tr('Этот компьютер'));
  }
  const noticeIcons={good:'<path d="M8 12.5l2.7 2.7L16 9.8"/>',info:'<path d="M12 11v5M12 8h.01"/>',bad:'<path d="M12 7.5v5.5M12 16.5h.01"/>'};
  function renderNotices(){
    const box=$('notices');if(!box)return;
    box.replaceChildren();box.hidden=!notices.length;
    for(const n of notices){
      const card=el('div',undefined,'am-notice am-notice-'+n.kind);card.dataset.key=n.key;
      const icon=document.createElementNS('http://www.w3.org/2000/svg','svg');icon.setAttribute('viewBox','0 0 24 24');icon.setAttribute('aria-hidden','true');icon.classList.add('am-notice-icon');
      icon.innerHTML='<circle cx="12" cy="12" r="9"/>'+noticeIcons[n.kind];
      const body=el('div',undefined,'am-notice-body');body.append(el('span',n.title,'am-notice-title'));if(n.text){body.append(el('span',n.text,'am-small am-muted am-notice-text'));card.title=n.text;}
      const close=el('button','×','am-toast-close');close.type='button';close.setAttribute('aria-label',tr('Закрыть'));close.addEventListener('click',()=>closeNotice(n.key));
      card.append(icon,body,close);
      if(n.kind!=='bad'){
        card.append(el('span',undefined,'am-notice-timer'));
        // The countdown starts once the notice can be seen.
        if(!n.timer&&tab==='tasks'&&!document.hidden)n.timer=setTimeout(()=>closeNotice(n.key),6000);
      }
      box.append(card);
    }
  }
  document.addEventListener('visibilitychange',renderNotices);
  // One line per agent program, with its worker state.
  // live is false when the computer cannot be reached.
  function busyTasksByComputer(){
    const byComputer=new Map();
    if(!project||view?.project!==project)return byComputer;
    function add(t,computer){
      const key=computer+'\0'+t.agent;
      if(!byComputer.has(key))byComputer.set(key,[]);
      byComputer.get(key).push(t);
    }
    for(const task of view?.tasks||[])add(task,taskComputer(task));
    for(const peer of view?.shared||[]){
      if(!peerFresh(peer))continue;
      for(const task of peer.tasks||[])add({...task,shared:true},sharedRunsOn(task,peer));
    }
    return byComputer;
  }
  function agentActivity(tasks){
    const active=tasks.filter(t=>activeStates.includes(t.state)&&!t.pending_launch&&!t.remote?.unaccepted&&!t.remote?.lost&&!t.remote?.silent);
    const running=active.filter(t=>t.state==='running'),waiting=active.filter(t=>t.state!=='running');
    return {running,waiting,counts:{running:running.length,waiting_owner:waiting.length}};
  }
  function agentLines(agents,coordinatorClient,live,computer,busyTasks){
    const lines=el('div',undefined,'am-machine-agents');
    const [coordProvider,coordLabel]=coordinatorClient!==null?coordinatorName(coordinatorClient):['',''];
    const providers=agents.map(a=>a.agent);
    // Only agents the owner keeps enabled are listed: a chat of an agent
    // that was paused or removed once used Messenger's tools, but listing
    // it showed the agent as if it were still there (owner, 2026-11-15).
    // A coordinator chat of a program that is not an agent stays listed.
    for(const provider of providers){
      const a=agents.find(x=>x.agent===provider);
      // Blue means a task is running (T-060). A chat that once used
      // Messenger's tools is listed but not lit: nothing tells whether it is
      // still open.
      const {running,waiting,counts}=agentActivity(busyTasks.get(computer+'\0'+provider)||[]);
      const [state,text]=live?agentState(counts):['offline',tr('Нет связи')];
      // A busy agent says so to the right of its name: «работает» in blue,
      // «ждёт ответа» in yellow. A free agent is just its name.
      const line=el('div',undefined,'am-agent-line am-agent-'+state);line.dataset.provider=provider;
      line.title=(names[provider]||provider)+(a?': '+text:'')+(state==='run'&&counts.waiting_owner>0?', '+tr('ждёт ответа')+' ('+counts.waiting_owner+')':'');
      const identity=el('span',shortNames[provider]||provider,state==='run'?'am-agent-running-label':'');
      line.append(identity);
      if(state==='run'||state==='wait'){
        const tasks=state==='run'?running:waiting;
        const dates=tasks.map(taskStart).filter(at=>Number.isFinite(Date.parse(at)));
        const first=dates.reduce((old,at)=>!old||Date.parse(at)<Date.parse(old)?at:old,'');
        const models=[...new Set(tasks.map(t=>t.shared?(t.model?modelName(t.model):''):taskModelLabel(t)).filter(Boolean))];
        if(models.length)identity.append(' ',el('span',models.map(m=>m.replace(/ Medium$/,' Med')).join(', '),'am-agent-model'));
        const total=counts.running+counts.waiting_owner;
        const count=total>1?'('+total+')':'';
        if(state==='run'&&first)line.append(elapsedLabel(first,count?count+' ':'','am-agent-status'));
        else line.append(el('span',(state==='wait'?tr('ждёт ответа'):tr('работает'))+(count?' '+count:''),'am-agent-status'));
      }else if(!live){
        line.append(el('span',tr('нет связи'),'am-agent-status'));
      }
      lines.append(line);
    }
    if(coordinatorClient!==null&&!coordProvider){
      const line=el('div',undefined,'am-agent-line am-agent-'+(live?'idle':'offline'));line.title=tr('Чат с инструментами Messenger');line.append(el('span',coordLabel));lines.append(line);
    }
    return lines;
  }
  // A computer of the network counts as reachable while its reports arrive:
  // it reports every 15 s, so three missed reports mean it is not.
  const peerFresh=peer=>peer.age>=0&&peer.age<=45;
  // machinePeers maps a network peer id to the computer's lasting id
  // (T-273), so a report that briefly lacks the lasting id keeps the card
  // in its place.
  let machineOrder=[],machinePeers={},machineOrderLoaded=false,machineOrderSave=Promise.resolve();
  try{
    const saved=JSON.parse(recall('messenger-machine-order'));
    if(Array.isArray(saved))machineOrder=saved.filter(key=>typeof key==='string');
  }catch(_){}
  function saveMachineOrder(){
    const body={order:machineOrder,peers:machinePeers};
    machineOrderSave=machineOrderSave.then(()=>postAPI('/api/machine-order',body,6000,true))
      .catch(error=>showError(error.message));
  }
  async function loadMachineOrder(){
    try{
      const saved=await api('/api/machine-order');
      if(Array.isArray(saved.order)){
        machineOrder=saved.order;
        if(saved.peers&&typeof saved.peers==='object')machinePeers=saved.peers;
      }else if(saved.order===null&&machineOrder.length){
        // Import a legacy order when the old window origin still exists.
        saveMachineOrder();
      }
    }catch(_){}
    finally{machineOrderLoaded=true;renderMachines();}
  }
  let draggedMachine='',machineDrop=null;
  const machineList=$('machines');
  function clearMachineDrop(){
    if(machineDrop)delete machineDrop.card.dataset.dropEdge;
    machineDrop=null;
  }
  // The order key of a computer of the network: its lasting id, known now
  // or from before, else its peer id. A newly learned id is saved, and an
  // order entry saved under the peer id moves to the lasting id.
  let peerKeySave=false;
  function peerOrderKey(peer){
    const known=machinePeers[peer.id];
    if(peer.computer&&known!==peer.computer&&machineOrderLoaded){
      if(Object.keys(machinePeers).length>=500)machinePeers={};
      machinePeers={...machinePeers,[peer.id]:peer.computer};
      const at=machineOrder.indexOf(peer.id);
      if(at>=0)machineOrder=machineOrder.includes(peer.computer)?machineOrder.filter(key=>key!==peer.id):machineOrder.map(key=>key===peer.id?peer.computer:key);
      if(!peerKeySave){peerKeySave=true;queueMicrotask(()=>{peerKeySave=false;saveMachineOrder();});}
    }
    return peer.computer||machinePeers[peer.id]||peer.id;
  }
  function moveMachine(from,to,after){
    const order=[...machineList.children].map(card=>card.dataset.machineOrder).filter(key=>key!==from);
    const target=order.indexOf(to);
    if(target<0)return;
    order.splice(target+Number(after),0,from);
    machineOrder=[...order,...machineOrder.filter(key=>!order.includes(key))];
    saveMachineOrder();
    renderMachines();
  }
  machineList.addEventListener('dragstart',event=>{
    const card=event.target.closest('[data-machine-order]');
    if(!card)return;
    draggedMachine=card.dataset.machineOrder;
    event.dataTransfer.setData('text/plain',draggedMachine);
    event.dataTransfer.effectAllowed='move';
  });
  machineList.addEventListener('dragover',event=>{
    if(!draggedMachine)return;
    const card=event.target.closest('[data-machine-order]');
    clearMachineDrop();
    if(!card||card.dataset.machineOrder===draggedMachine)return;
    event.preventDefault();
    event.dataTransfer.dropEffect='move';
    const rect=card.getBoundingClientRect(),after=event.clientY>=rect.top+rect.height/2;
    card.dataset.dropEdge=after?'after':'before';
    machineDrop={card,after};
  });
  machineList.addEventListener('dragleave',event=>{
    if(!machineList.contains(event.relatedTarget))clearMachineDrop();
  });
  machineList.addEventListener('drop',event=>{
    if(!draggedMachine||!machineDrop)return;
    event.preventDefault();
    const from=draggedMachine,{card,after}=machineDrop;
    draggedMachine='';clearMachineDrop();
    moveMachine(from,card.dataset.machineOrder,after);
  });
  machineList.addEventListener('dragend',()=>{
    draggedMachine='';clearMachineDrop();renderMachines();
  });
  // T-271: the computers that have a card in the current render.
  let pickedComputers=new Set();
  function pickable(card,id,orderKey=id){
    pickedComputers.add(id);
    card.setAttribute('role','button');card.tabIndex=0;card.setAttribute('aria-pressed',String(computerFilter===id));
    card.dataset.machineOrder=orderKey;card.draggable=machineOrderLoaded;
    card.setAttribute('aria-keyshortcuts','Alt+ArrowUp Alt+ArrowDown');
    card.title=computerFilter===id?tr('Показать все компьютеры'):tr('Показать только этот компьютер');
    card.addEventListener('click',event=>{if(!event.target.closest('button'))filterComputer(id);});
    card.addEventListener('keydown',event=>{
      if(event.target!==card)return;
      if(event.key==='Enter'||event.key===' '){event.preventDefault();filterComputer(id);}
      if(!machineOrderLoaded||!event.altKey||!['ArrowUp','ArrowDown'].includes(event.key))return;
      event.preventDefault();
      const after=event.key==='ArrowDown',next=after?card.nextElementSibling:card.previousElementSibling;
      if(!next)return;
      const key=card.dataset.machineOrder;
      moveMachine(key,next.dataset.machineOrder,after);
      machineList.querySelector('[data-machine-order="'+CSS.escape(key)+'"]')?.focus();
    });
  }
  // The window redraws every two seconds. A part whose content did not change
  // keeps its nodes: rebuilding them anyway reset hover, tooltips and
  // animations, and the window seemed to flicker (owner, 2026-11-12).
  function swapChildren(live,next){
    if(next.innerHTML!==live.innerHTML)live.replaceChildren(...next.childNodes);
  }
  function machineRole(label,tip){
    const tag=el('span',label,'am-small am-muted am-machine-role');tag.title=tip;
    return tag;
  }
  function serverTag(){
    return machineRole(tr('сервер'),tr('На этом компьютере работает сервер сети'));
  }
  function renderMachines(){
    if(draggedMachine)return;
    collectNotices();
    pickedComputers=new Set();
    const live=$('machines'),box=live.cloneNode(false),busyTasks=busyTasksByComputer();
    const machine=el('div',undefined,'am-machine-card');machine.setAttribute('aria-current','true');pickable(machine,'local');
    // Every card is laid out the same (owner, 2026-11-18, T-198): the dot
    // before the name, with the update icon and version at the top right. This computer's version and update button
    // are the window's own elements, moved into its card (placeLocalParts).
    const head=el('div',undefined,'am-machine-name');
    const dot=el('span',undefined,'am-dot'+(online?' online':''));dot.title=online?tr('На связи'):tr('Нет связи');head.append(dot);
    head.append(el('span',view?.computer_name||tr('Этот компьютер')));
    // The computer that runs the network's server is labelled as such.
    if(server?.host?.running&&server.host.scope!=='local')head.append(serverTag());
    const coordinator=view?.project===project?view?.coordinator:null;
    if(coordinator?.active)head.append(machineRole(tr('координатор'),tr('На этом компьютере открыт чат-координатор выбранного проекта')));
    const version=el('div',undefined,'am-machine-version am-local-version');
    machine.append(head);
    // A paused agent is left out, as other computers leave it out of what
    // they report (owner, 2026-11-18: two of three agents were off and the
    // Mac still listed all three). One still running a task stays until it
    // ends.
    const shownAgents=(view?.agents||[]).filter(a=>!a.paused||busyTasks.get('local\0'+a.agent)?.some(t=>activeStates.includes(t.state)));
    const lines=agentLines(shownAgents,coordinator?.connected?(coordinator.client||''):null,online,'local',busyTasks);
    if(!lines.children.length)lines.append(el('p',online?tr('Агенты не подключены'):tr('Нет связи'),'am-small am-muted'));
    machine.append(lines);
    const corner=el('div',undefined,'am-card-update am-local-update');
    if(view?.pending_update==='updating'||view?.pending_update==='queued'){
      const tip=view.pending_update==='updating'?tr('Обновляется…'):tr('ждёт обновления');
      const b=el('span',undefined,'am-update-icon am-peer-update am-update-wait');b.title=tip;b.setAttribute('role','img');b.setAttribute('aria-label',tip);appendUpdateGlyphs(b);
      corner.append(b);
    }
    const localMeta=el('div',undefined,'am-machine-meta');localMeta.append(corner,version);head.append(localMeta);box.append(machine);
    // The other computers of this computer's network, with their agents.
    // Only the computer that runs the server can revoke one, from the
    // right-click menu.
    const hosted=new Set((server?.host?.devices||[]).filter(d=>!d.revoked_at).map(d=>d.id));
    for(const peer of server?.peers||[]){
      const fresh=peerFresh(peer);
      const card=el('div',undefined,'am-machine-card am-peer-card');card.dataset.peer=peer.id;if(peer.host)card.dataset.host='';pickable(card,peer.id,peerOrderKey(peer));
      const name=el('div',undefined,'am-machine-name');
      const pdot=el('span',undefined,'am-dot'+(fresh?' online':''));pdot.title=fresh?tr('На связи'):tr('Нет связи');
      name.append(pdot,el('span',peer.name));
      if(peer.host)name.append(serverTag());
      const meta=el('div',undefined,'am-machine-meta');
      const corner=el('div',undefined,'am-card-update');
      // T-128: an older computer gets the update icon next to its dot; the
      // owner confirms in a popup.
      const busy=updateLine(peer);
      if(busy){
        // While an update is asked for, waits for tasks or is under way, the
        // arrow stays, dimmed and still, and says so on hover: another
        // computer reports no progress, and a spinning mark looked like a
        // download that may wait for hours (owner, 2026-11-14).
        const tip=busy[0];
        const b=el('span',undefined,'am-update-icon am-peer-update am-update-wait');b.title=tip;b.setAttribute('role','img');b.setAttribute('aria-label',tip);appendUpdateGlyphs(b);
        corner.append(b);
      }else if(fresh&&peer.version&&versionOlder(peer.version,newestVersion())){
        const newest=versionName(newestVersion()),label=tr('Обновить до ')+newest;
        const b=el('button',undefined,'am-update-icon am-peer-update');b.type='button';b.title=label;b.setAttribute('aria-label',label);appendUpdateGlyphs(b);
        b.addEventListener('click',event=>{event.stopPropagation();askConfirm(tr('Обновить ')+(peer.name||tr('компьютер'))+tr(' до ')+newest+'?',tr('Программа на том компьютере обновится и перезапустится. Задачи и история сохранятся.')+(peer.host?' '+tr('На нём работает сервер сети: пока он перезапускается, компьютеры не видят друг друга. Если там попросят подтверждение, сервер не вернётся, пока кто-нибудь не ответит у того компьютера.'):''),tr('Обновить'),()=>requestUpdate(peer),true);});
        b.addEventListener('keydown',event=>event.stopPropagation());
        corner.append(b);
      }
      if(corner.children.length)meta.append(corner);
      if(peer.version)meta.append(el('span',versionName(peer.version),'am-version am-machine-version'));
      name.append(meta);card.append(name);
      if(revokeAsk===peer.id&&hosted.has(peer.id)){
        card.append(el('p',tr('Отозвать устройство? Вернуть его можно только новым приглашением.'),'am-small am-muted'));
        const actions=el('div',undefined,'am-actions am-device-confirm');
        const yes=el('button',tr('Отозвать'),'am-danger');yes.type='button';yes.dataset.key='revoke-yes-'+peer.id;
        yes.addEventListener('click',()=>{revokeAsk='';serverAction({action:'revoke',device:peer.id},'host','');});
        const no=el('button',tr('Отмена'));no.type='button';
        no.addEventListener('click',()=>{revokeAsk='';renderMachines();$('machines').querySelector('[data-key="'+CSS.escape('revoke-'+peer.id)+'"]')?.focus();});
        actions.append(no,yes);card.append(actions);box.append(card);continue;
      }
      if(fresh){
        const plines=agentLines(peer.agents||[],peer.coordinator?peer.coordinator:null,true,peer.id,busyTasks);
        if(!plines.children.length)plines.append(el('p',tr('Агенты не подключены'),'am-small am-muted'));
        card.append(plines);
      }else{
        card.append(el('p',peer.last_seen?tr('Был на связи ')+stamp(peer.last_seen):tr('Ещё не выходил на связь'),'am-small am-muted'));
      }
      const can={revoke:hosted.has(peer.id)};
      if(can.revoke){
        card.classList.add('am-device-card');card.tabIndex=0;card.dataset.key='revoke-'+peer.id;card.title=tr('Правая кнопка мыши — действия');
        card.addEventListener('contextmenu',event=>{event.preventDefault();deviceMenu(peer,event.clientX,event.clientY,can);});
        card.addEventListener('keydown',event=>{if(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')){event.preventDefault();const r=card.getBoundingClientRect();deviceMenu(peer,r.left+16,r.top+24,can);}});
      }
      box.append(card);
    }
    const rank=new Map(machineOrder.map((key,index)=>[key,index]));
    const ranks=card=>rank.get(card.dataset.machineOrder)??machineOrder.length;
    box.append(...[...box.children].sort((a,b)=>ranks(a)-ranks(b)));
    // The moved elements are left out of the comparison, so an unchanged
    // list keeps its nodes; before a rebuild they go back to the bottom row,
    // or they would leave the page with the old card.
    const copy=live.cloneNode(true);
    for(const id of localParts)copy.querySelector('#'+id)?.remove();
    if(copy.innerHTML!==box.innerHTML){parkLocalParts();live.replaceChildren(...box.childNodes);}
    placeLocalParts();
    // T-271: only the highlighted card shows the chosen computer, so a
    // computer that left the list no longer filters anything.
    if(computerFilter&&!pickedComputers.has(computerFilter))queueMicrotask(()=>{if(computerFilter&&!pickedComputers.has(computerFilter))clearComputer();});
  }
  // This computer's version and update button live in its card; in a narrow
  // window the computer list is hidden and they stay in the bottom row.
  const localParts=['app-installed-version','app-update-button'];
  const narrowSidebar=matchMedia('(max-width:620px)');
  function parkLocalParts(){
    const row=document.querySelector('.am-version-row');
    row.prepend(...localParts.map(id=>$(id)).filter(node=>node.parentNode!==row));
  }
  function placeLocalParts(){
    const card=$('machines').querySelector('[aria-current="true"]');
    if(narrowSidebar.matches||!card){parkLocalParts();return;}
    const version=card.querySelector('.am-local-version'),corner=card.querySelector('.am-local-update');
    if($('app-installed-version').parentNode!==version)version.append($('app-installed-version'));
    if($('app-update-button').parentNode!==corner)corner.append($('app-update-button'));
  }
  narrowSidebar.addEventListener('change',placeLocalParts);
  // The computer that runs a task: this one, or the other one it was sent to.
  function taskComputer(t){return t.remote&&!t.remote.from?t.remote.id:'local';}
  // Where another computer's task runs (T-186): the computer it was sent
  // to, or the giver itself.
  // «— от Mac»: the computer that gave a task running elsewhere (owner,
  // 2026-11-15).
  function fromTag(name){return el('span',tr(' — от ')+name,'am-task-from');}
  function sharedRunsOn(t,p){return t.runs_on_peer||p.peer;}
  function sharedComputer(t,p){return t.runs_on_peer?((server?.peers||[]).find(x=>x.id===t.runs_on_peer)?.name||t.runs_on||t.runs_on_peer):p.name;}
  // A click on a computer in the sidebar shows only its tasks; a second click
  // or a click on empty sidebar space shows all again (T-056). The highlighted
  // card is the only sign of the choice (T-271).
  let computerFilter='';
  // On the statistics and agents tabs the choice filters that tab (T-076);
  // elsewhere it opens the tasks.
  function filterComputer(id){
    computerFilter=computerFilter===id?'':id;
    // T-096: a new computer starts with all its agents, so an agent chosen
    // earlier cannot hide what runs there.
    if($('agent-filter').value){$('agent-filter').value='';syncSelects();}
    if(computerFilter&&tab!=='stats'&&tab!=='agents')show('tasks');
    computerChanged();
  }
  function clearComputer(){computerFilter='';computerChanged();}
  function computerChanged(){
    renderMachines();
    if(tab==='tasks')openTopTask();else renderTaskList();
    if(tab==='stats')reloadStats();
    if(tab==='agents')renderAgents();
    if(view?.history_paged)refresh();
  }
  function computerName(id){
    return id==='local'?(view?.computer_name||tr('Этот компьютер')):(server?.peers||[]).find(p=>p.id===id)?.name||(view?.tasks||[]).find(t=>taskComputer(t)===id)?.remote?.name||'';
  }
  let taskHistory=null,windowHistoryScope='';
  function taskWindowQuery(){return new URLSearchParams({project,agent:$('agent-filter').value,computer:computerFilter}).toString();}
  function currentTaskHistory(){return taskHistory?.scope===taskWindowQuery()?taskHistory:null;}
  function taskRows(){
    const recent=view?.tasks||[],history=currentTaskHistory();
    if(!history?.rows.size)return recent;
    const visible=new Map(history.rows);
    for(const t of recent)visible.set(t.id,t);
    return [...visible.values()];
  }
  async function loadOlderTasks(){
    const scope=taskWindowQuery();
    if(windowHistoryScope!==scope)return;
    if(!currentTaskHistory())taskHistory={scope,rows:new Map(),next:view?.history_next||'',loading:false};
    const history=taskHistory;
    if(history.loading||!history.next)return;
    history.loading=true;renderTaskList();
    try{
      const q=new URLSearchParams(scope);q.set('before',history.next);
      const page=await api('/api/task-history?'+q);
      if(currentTaskHistory()!==history)return;
      for(const t of page.tasks||[])history.rows.set(t.id,t);
      history.next=page.next||'';
    }catch(error){if(currentTaskHistory()===history)showError(error);}
    finally{history.loading=false;if(currentTaskHistory()===history)renderTaskList();}
  }
  function visibleTasks(){
    const filter=$('agent-filter').value;
    const list=taskRows().filter(t=>(!filter||t.agent===filter)&&(!computerFilter||taskComputer(t)===computerFilter));
    const active=list.filter(t=>activeStates.includes(t.state)),done=list.filter(t=>!activeStates.includes(t.state));
    return [active,done];
  }
  // The task list's groups, newest first: own tasks and other computers'
  // tasks of the project together (T-047, T-110).
  function taskGroups(){
    const [active,done]=visibleTasks(),agentFilter=$('agent-filter').value;
    const items=[...active.map(t=>({t})),...done.map(t=>({t}))];
    for(const p of view?.shared||[]){
      for(const t of p.tasks){
        if(computerFilter&&computerFilter!==sharedRunsOn(t,p))continue;
        if(!agentFilter||t.agent===agentFilter)items.push({t,peer:p});
      }
    }
    items.sort((x,y)=>String(y.t.created_at).localeCompare(String(x.t.created_at)));
    const groups={active:[],attention:[],done:[]};
    for(const it of items){
      if(needsDecision(it))groups.attention.push(it);
      else if(activeStates.includes(it.t.state))groups.active.push(it);
      else groups.done.push(it);
    }
    return groups;
  }
  // An open task whose outcome this computer cannot know: sent to a computer
  // out of contact or silent about it, or another computer's task while that
  // computer is out of contact.
  function needsDecision(it){
    if(it.peer)return sharedLost(it.t,it.peer);
    // Blocked and interrupted tasks hold their computer (it will not update)
    // until someone closes them (D-120).
    if(it.t.state==='blocked'||it.t.state==='interrupted')return true;
    const r=it.t.remote;
    return !!r&&!r.from&&activeStates.includes(it.t.state)&&!!(r.lost||r.silent);
  }
  // Each of them can be closed here: an own task on another computer is
  // cancelled (it closes here at once), a stuck task of this computer closes
  // if its worker is certainly gone (D-120), another computer's task leaves
  // this list.
  function closable(it){return needsDecision(it);}
  function askCloseAll(items){
    const here=items.filter(it=>!it.peer&&!(it.t.remote&&!it.t.remote.from)).length;
    const own=items.filter(it=>!it.peer).length-here,others=items.length-own-here;
    const parts=[];
    if(here)parts.push(tr('Прерванные и заблокированные задачи этого компьютера закроются, если их исполнителя точно нет: ')+here+'.');
    if(own)parts.push(tr('Задачи, отправленные на другие компьютеры, закроются в этом окне: ')+own+tr('. Что с ними там, неизвестно; если связь вернётся, туда придёт отмена.'));
    if(others)parts.push(tr('Задачи других компьютеров пропадут из списка на этом компьютере: ')+others+'.');
    askConfirm(tr('Закрыть все задачи без решения?'),parts.join(' '),tr('Закрыть все'),async()=>{
      const failed=[];
      for(const it of items){
        try{
          if(it.peer)await postAPI('/api/task-hide',{peer:it.peer.peer,task:it.t.id});
          else await postAPI('/api/task-cancel',{task:it.t.id});
        }catch(e){failed.push(e.message);}
      }
      viewSignature='';await refresh();
      if(failed.length)showError(new Error(tr('Не закрылись: ')+failed.length+'. '+failed[0]));
    });
  }
  function askClearDone(items){
    const paged=!!view?.history_paged,scope={project:view?.project,agent:$('agent-filter').value,computer:computerFilter};
    const text=paged?tr('Из списка пропадут все завершённые задачи по текущему фильтру, включая те, которые ещё не загружены. История задач сохранится.')
      :tr('Из списка на этом компьютере пропадут завершённые задачи: ')+items.length+tr('. Задачи этого компьютера пропадут и у других компьютеров; задачи других компьютеров останутся у них.');
    askConfirm(tr('Убрать все завершённые задачи?'),text,tr('Убрать все'),async()=>{
      const failed=[];
      if(paged){
        await postAPI('/api/task-hide-history',scope);
        taskHistory=null;
        if(items.some(it=>!it.peer&&it.t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}
      }
      for(const it of items){
        if(paged&&!it.peer)continue;
        try{await postAPI('/api/task-hide',{peer:it.peer?.peer||'',task:it.t.id});}
        catch(e){failed.push(e.message);}
      }
      viewSignature='';await refresh();
      if(failed.length)showError(new Error(tr('Не убрались: ')+failed.length+'. '+failed[0]));
    });
  }
  // Owner, 2026-11-12: choosing a computer opens its newest active task, or
  // its newest task; with no computer chosen, the top task of the list.
  // On refresh (quiet) the caller loads the conversation itself, and an
  // empty list changes nothing.
  function openTopTask(quiet){
    const g=taskGroups(),it=g.active[0]||[...g.attention,...g.done].sort((x,y)=>String(y.t.created_at).localeCompare(String(x.t.created_at)))[0];
    if(!it){if(quiet)return;selectedTask='';selectedShared=null;feed=null;feedSignature='';renderTaskList();renderThread();return;}
    if(it.peer){selectedShared={peer:it.peer.peer,id:it.t.id};selectedTask='';feed=null;feedSignature='';renderTaskList();renderThread();startWindowWatch();return;}
    if(selectedTask===it.t.id&&!selectedShared)return;
    selectedTask=it.t.id;selectedShared=null;feed=null;feedSignature='';renderTaskList();if(!quiet)loadFeed();
  }
  // A model's display name (T-202): «gpt-6.2-sol[effort=low]»
  // reads «Sol 6.2 Low», «claude-opus-5-5[effort=medium]» «Opus 5.5 Medium».
  // T-274: other ids of plain words read word by word, «opus» «Opus»,
  // «grok-4.7-high-fast» «Grok 4.7 High Fast»; Cursor's own prefix is
  // left out. The window shows every model through this one rule; the
  // exact id stays in the details.
  function modelName(value){
    const m=String(value).match(/^(.*?)(?:\[effort=([a-z]+)\])?$/);
    let base=m[1];const effort=m[2]?' '+capital(m[2]):'';
    const cap=s=>s?s[0].toUpperCase()+s.slice(1):s;
    let g=base.match(/^gpt-(\d+(?:\.\d+)?)-([a-z]+)$/i);
    if(g)return cap(g[2])+' '+g[1]+effort;
    g=base.match(/^claude-([a-z]+)-(\d+)-(\d+)$/i);
    if(g)return cap(g[1])+' '+g[2]+'.'+g[3]+effort;
    if(/^[a-z][a-z0-9.]*(?:-[a-z0-9.]+)*$/i.test(base))return base.replace(/^cursor-(?=.)/i,'').split('-').map(cap).join(' ')+effort;
    return base+effort;
  }
  function taskModelLabel(t){
    if(t.ran_model)return modelName(t.ran_model);
    return t.model?tr('модель не подтверждена'):tr('по умолчанию');
  }
  function taskModelDetail(t){
    if(t.ran_model)return tr('Модель, на которой исполнитель запущен')+': '+t.ran_model;
    return t.model?tr('Модель, которую указал координатор')+': '+t.model:tr('Модель по умолчанию');
  }
  function renderTaskList(){
    const live=$('task-list'),box=live.cloneNode(false);
    // While another project loads, its rows stay in place but cannot be
    // chosen, so the list does not collapse and grow again.
    if(view?.project!==project){if(!live.childElementCount)$('task-count').textContent='';live.inert=true;live.setAttribute('aria-busy','true');return;}
    live.inert=false;live.removeAttribute('aria-busy');
    // T-096: say when the agent filter hides active tasks, with a way back.
    const agentChosen=$('agent-filter').value;
    const hidden=agentChosen?(view?.tasks||[]).filter(t=>t.agent!==agentChosen&&activeStates.includes(t.state)&&(!computerFilter||taskComputer(t)===computerFilter)).length:0;
    if(hidden){
      const all=el('button',undefined,'am-task am-task-hidden');all.type='button';
      all.append(el('span',tr('Фильтр «')+(names[agentChosen]||agentChosen)+tr('» скрывает активные задачи: ')+hidden,'am-task-agent'),el('span',tr('Показать всех агентов'),'am-status-run'));
      all.addEventListener('click',()=>{$('agent-filter').value='';syncSelects();renderTaskList();});
      box.append(all);
    }
    $('task-count').textContent=taskRows().length+(view?.shared||[]).reduce((n,p)=>n+p.tasks.length,0)||'';
    // Another computer's task is read-only and says which computer gave it.
    const ownRow=t=>{
      const row=el('button',undefined,'am-task');row.type='button';row.setAttribute('aria-pressed',String(t.id===selectedTask));
      // T-119: every own task says where it runs; this computer's name
      // stands out. The model it runs on follows the status below (T-202).
      const here=!(t.remote&&!t.remote.from),label=el('span',(names[t.agent]||t.agent)+tr(' на '));
      label.append(el('span',here?(view.computer_name||tr('этом компьютере')):t.remote.name,here?'am-task-here':''));
      if(t.remote?.from)label.append(fromTag(t.remote.name));
      const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),label);
      if(t.remote)agent.title=t.remote.from?tr('Задачу дал компьютер «')+t.remote.name+'»':tr('Выполняется на компьютере «')+t.remote.name+'»';
      appendTaskWhen(agent,t);
      const [text,cls,tip,from]=taskStatus(t);
      const status=el('span',undefined,'am-task-status '+cls);if(tip)status.title=tip;
      const model=taskModelLabel(t);
      const words=el('span');status.append(words);words.append(statusPiece(text,!!t.usage||!!model,from));
      if(t.usage){const used=el('span',usageShort(t.usage),'am-task-usage');used.title=usageLong(t.usage);if(model)used.append(el('span',',','am-task-comma'));words.append(used);}
      if(model){const m=el('span',model,'am-task-usage am-task-model');m.title=taskModelDetail(t);words.append(m);}
      row.append(agent,el('span',t.title,'am-task-title'),status);
      row.addEventListener('click',()=>{selectedTask=t.id;selectedShared=null;feedSignature='';renderTaskList();renderThread();loadFeed();});
      row.addEventListener('contextmenu',e=>taskMenu(e,t,null));
      return row;
    };
    const sharedRow=(t,p)=>{
      const row=el('button',undefined,'am-task am-task-shared');row.type='button';row.title=tr('Задачу дал компьютер «')+p.name+tr('». Здесь её можно только посмотреть');
      row.setAttribute('aria-pressed',String(selectedShared?.peer===p.peer&&selectedShared?.id===t.id));
      // T-111: another computer's task opens as a read-only card.
      row.addEventListener('click',()=>{selectedShared={peer:p.peer,id:t.id};selectedTask='';feed=null;feedSignature='';renderTaskList();renderThread();startWindowWatch();});
      row.addEventListener('contextmenu',e=>taskMenu(e,t,p));
      // T-186: like an own task, it says where it runs; the giver is in the
      // tooltip. An older giver does not say, so it shows the giver's name.
      const label=el('span',(names[t.agent]||t.agent)+tr(' на '));label.append(el('span',sharedComputer(t,p)));
      if(t.runs_on_peer)label.append(fromTag(p.name));
      const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),label);agent.title=tr('Задачу дал компьютер «')+p.name+'»';
      appendTaskWhen(agent,t);
      const [text,cls,tip,from]=sharedStatus(t,p);
      const words=el('span');words.append(statusPiece(text,!!t.tokens||!!t.model,from));
      if(t.tokens){
        const used=el('span',tokensText(t.tokens)+tr(' токенов'),'am-task-usage');
        if(t.model)used.append(el('span',',','am-task-comma'));
        words.append(used);
      }
      if(t.model)words.append(el('span',modelName(t.model),'am-task-usage am-task-model'));
      const status=el('span',undefined,'am-task-status '+cls);status.append(words);if(tip)status.title=tip;
      row.append(agent,el('span',t.title,'am-task-title'),status);
      if(t.report)row.append(el('span',t.report,'am-task-report'));
      return row;
    };
    const section=(label,items,action)=>{
      if(!items.length)return;
      const head=el('div',undefined,'am-label am-task-section');head.append(el('span',label));
      if(action)head.append(action);
      box.append(head);
      for(const it of items)box.append(it.peer?sharedRow(it.t,it.peer):ownRow(it.t));
    };
    const groups=taskGroups();
    section(tr('Активные'),groups.active);
    // Owner, 2026-11-12: tasks whose outcome is unknown kept «Активные» full
    // for days. They wait here until someone closes them; nothing closes
    // them by itself (unknown is not success).
    let closeAll=null;
    if(groups.attention.some(closable)){
      closeAll=el('button',tr('Закрыть все'),'am-task-section-action');closeAll.type='button';
      closeAll.addEventListener('click',()=>askCloseAll(groups.attention.filter(closable)));
    }
    section(tr('Требуют решения'),groups.attention,closeAll);
    // Owner, 2026-11-15: one button removes every finished task from this
    // list, as «Удалить» does one by one. Tasks stay in the database.
    let clearDone=null;
    if(groups.done.length>1){
      clearDone=el('button',tr('Убрать все'),'am-task-section-action');clearDone.type='button';
      clearDone.addEventListener('click',()=>askClearDone(groups.done));
    }
    section(tr('Завершённые'),groups.done,clearDone);
    const history=currentTaskHistory();
    if(view?.history_paged&&windowHistoryScope===taskWindowQuery()&&(history?history.next:view.history_next)){
      const older=el('button',history?.loading?tr('Загружаем…'):tr('Показать ещё 10'),'am-task-section-action');
      older.type='button';older.disabled=!!history?.loading;older.addEventListener('click',loadOlderTasks);
      const more=el('div',undefined,'am-label am-task-section');more.append(older);box.append(more);
    }
    if(!box.children.length)box.append(el('p',view?.project?(computerFilter||$('agent-filter').value?tr('Здесь задач нет'):tr('Задач пока нет. Когда координатор раздаст первые, они появятся здесь.')):'','am-small am-muted'));
    swapChildren(live,box);
  }
  // Coordinator connection (T-039): the client runs Claude's own
  // `claude mcp add` in the project folder; the owner never opens a terminal.
  let linkBusy='',linkFailure=null;
  async function connectCoordinator(agent){
    const name=view?.project;if(!name||linkBusy)return;
    linkBusy=agent;
    if(linkFailure?.project===name&&linkFailure?.agent===agent)linkFailure=null;
    renderThread();renderCoordinatorRow();
    try{
      const r=await postAPI('/api/coordinator-connect',{project:name,agent},40000);
      if(r.result?.state==='failed')linkFailure={project:name,agent,detail:r.result.detail||''};
    }catch(error){linkFailure={project:name,agent,detail:error.message};}
    finally{linkBusy='';viewSignature='';await refresh();}
  }
  async function connectReadyCoordinators(){
    const ready=id=>(view?.agents||[]).some(a=>a.agent===id&&a.cli);
    const link=id=>coordinatorLink(id);
    for(const agent of ['claude','codex','cursor']){
      if(!ready(agent))continue;
      const state=link(agent)?.state;
      if(state==='connected'||state==='no_folder'||state==='other'||state==='off')continue;
      await connectCoordinator(agent);
    }
  }
  // The coordinator of the project on this computer (T-152): Claude's link
  // for the project folder, as its own card of the «Проект» page, and the manual
  // command under it. Agents are set up once; the link is per folder.
  function renderCoordinatorRow(){
    const box=$('coord-box');if(!box||!view?.project)return;
    const heading=el('div',undefined,'am-coord-head');
    heading.append(el('h3',tr('MCP-плагин')));
    heading.append(el('p',tr('Подключение к графическому редактору'),'am-small am-muted'));
    const steps=el('div',undefined,'am-steps');
    steps.append(coordinatorAgentRow('claude',view.coordinator_link||{state:'unknown'}));
    steps.append(coordinatorAgentRow('codex',view.codex_link||{state:'unknown'}));
    steps.append(coordinatorAgentRow('cursor',view.cursor_link||{state:'unknown'}));
    const manual=el('details',undefined,'am-tool');manual.append(el('summary',tr('Подключить вручную через Терминал')));
    const claudeCmd='cd '+shellQuote(view.folder)+' && claude mcp add --scope local messenger -- '+shellQuote(view.messenger)+' --root '+shellQuote(view.data_root)+' coordinator --project '+shellQuote(view.project);
    const codexToml='[mcp_servers.messenger]\ncommand = '+JSON.stringify(view.messenger)+'\nargs = ["--root", '+JSON.stringify(view.data_root)+', "coordinator", "--project", '+JSON.stringify(view.project)+']\ndefault_tools_approval_mode = "approve"\ntool_timeout_sec = 180\n';
    const block=(title,text)=>{
      const copy=el('button',tr('Скопировать команду'));copy.type='button';
      copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(text);copy.textContent=tr('Скопировано');}catch(_){copy.textContent=tr('Скопируйте вручную');}setTimeout(()=>copy.textContent=tr('Скопировать команду'),2000);});
      const wrap=el('div');wrap.append(el('p',title,'am-small am-muted'),el('pre',text,'am-command'),copy);return wrap;
    };
    const cursorJSON=JSON.stringify({mcpServers:{messenger:{type:'stdio',command:view.messenger,args:['--root',view.data_root,'coordinator','--project',view.project],env:{AGENT_MESSENGER_WORKER_RUN:'${env:AGENT_MESSENGER_WORKER_RUN}'}}}},null,2);
    manual.append(block('Claude Code',claudeCmd),block('Codex (.codex/config.toml)',codexToml),block(tr('Cursor: добавьте messenger в .cursor/mcp.json, сохранив остальные записи'),cursorJSON));
    manual.hidden=!view.folder||(['claude','codex','cursor'].every(coordinatorAgentOn));
    const body=el('section',undefined,'am-card am-coord-box');body.id='coord-box';body.append(heading,steps,manual);
    if(box.querySelector('details')?.open)manual.open=true;
    if(box.innerHTML!==body.innerHTML)box.replaceWith(body);
  }
  function coordinatorLink(agent){return agent==='cursor'?view?.cursor_link:agent==='codex'?view?.codex_link:view?.coordinator_link;}
  function coordinatorAgentOn(agent){
    const link=coordinatorLink(agent);
    return link?.state==='connected';
  }
  function coordinatorAgentRow(agent,link){
    const title=names[agent]||agent;
    const connect=[tr('Подключить'),()=>connectCoordinator(agent)];
    const failure=linkFailure?.project===view.project&&linkFailure?.agent===agent?linkFailure:null;
    const missing='no_'+agent;
    if(linkBusy===agent)return stepRow(1,title,tr('Подключаем…'),null);
    if(link.state==='connected'){
      let sub=agent==='cursor'?tr('Записано. Откройте новый чат Cursor в этой папке и разрешите подключение Messenger'):tr('Подключён. Откройте папку проекта в новом чате');
      if(agent==='codex'&&(view.agents||[]).some(a=>a.agent==='codex'&&a.trust?.state==='not_trusted'))sub=tr('Записано. Подтвердите доверие папке кнопкой «Доверять» на карточке Codex во вкладке «Агенты».');
      return stepRow('ok',title,sub,null);
    }
    if(failure)return stepRow('no',title,tr('Не получилось: ')+failure.detail,[tr('Повторить'),()=>connectCoordinator(agent)]);
    if(link.state===missing)return stepRow(1,title,tr('Сначала настройте его во вкладке «Агенты»'),[tr('Агенты'),()=>show('agents')]);
    if(link.state==='no_folder')return stepRow(1,title,tr('Сначала укажите папку проекта'),null);
    if(link.state==='other')return stepRow('no',title,agent==='cursor'?tr('В настройках Cursor этой папки уже есть другое подключение «messenger». Messenger его не меняет'):agent==='codex'?tr('В настройках Codex этой папки уже есть другое подключение «messenger». Messenger его не меняет'):tr('В Claude у этой папки уже есть другое подключение «messenger». Messenger его не меняет'),connect);
    if(link.state==='off')return stepRow('no',title,agent==='cursor'?tr('Подключение «messenger» в Cursor выключено. Messenger его не включает'):tr('Подключение «messenger» в Codex выключено. Messenger его не включает'),null);
    if(link.state==='unknown')return stepRow('no',title,(agent==='cursor'?tr('Не удалось прочитать настройки Cursor'):agent==='codex'?tr('Не удалось прочитать настройки Codex'):tr('Не удалось прочитать настройки Claude'))+(link.detail?': '+link.detail:''),connect);
    return stepRow(1,title,tr('Не подключён'),connect);
  }
  // Whether this computer's Claude Code is the project's coordinator. Other
  // computers are not shown: any computer may have its own (owner, T-152).
  function coordinatorPlace(){
    return view?.coordinator?.connected||coordinatorAgentOn('claude')||coordinatorAgentOn('codex')||coordinatorAgentOn('cursor');
  }
  // The task pane before the first project or task (T-152, T-164): explain
  // the program first, then show setup steps with their current state.
  function welcome(){
    const box=el('div',undefined,'am-settings am-welcome');
    const intro=el('div',undefined,'am-welcome-intro');
    intro.append(el('h2',tr('Добро пожаловать в Agent Messenger'),'am-welcome-title'),
      el('p',tr('Ставьте задачи агентам на всех своих компьютерах из одного чата.'),'am-muted'));
    box.append(intro);
    const chain=el('div',undefined,'am-chain');
    const node=(title,sub)=>{const n=el('div',undefined,'am-chain-node');n.append(el('strong',title),el('span',sub,'am-small am-muted'));return n;};
    const arrow=()=>{const a=el('span','→','am-chain-arrow');a.setAttribute('aria-hidden','true');return a;};
    chain.append(node(tr('Вы'),tr('Пишете координатору, что нужно сделать.')),arrow(),node(tr('Координатор'),tr('Чат в Claude Code, Codex или Cursor. Раздаёт задачи и собирает отчёты.')),arrow(),node(tr('Исполнители'),tr('Claude Code, Codex и Cursor на ваших компьютерах.')));
    box.append(el('h3',tr('Как это работает')),chain);
    const steps=el('div',undefined,'am-steps am-welcome-steps');
    let number=0;
    const step=(ok,title,sub,label,go)=>{
      const row=el('div',undefined,'am-step');number++;
      row.append(el('span',ok?'✓':String(number),'am-mark '+(ok?'am-mark-ok':'am-mark-off')));
      const text=el('div',undefined,'am-step-text');text.append(el('p',title,'am-step-title'),el('p',sub,'am-step-sub'));row.append(text);
      if(label){const b=el('button',label);b.type='button';b.addEventListener('click',go);row.append(b);}else row.append(el('span'));
      steps.append(row);
    };
    const hasProject=!!view.project;
    step(hasProject,tr('Проект'),hasProject?view.project:tr('Проект связывает чат-координатора, папку проекта на этом компьютере и задачи.'),hasProject?'':tr('Новый проект'),()=>openProjectDialog());
    const agents=(view.agents||[]).length;
    step(agents>0,tr('Агенты на этом компьютере'),agents?tr('Подключено: ')+agents:tr('Установите и подключите хотя бы одного'),agents?'':tr('Агенты'),()=>show('agents'));
    const place=hasProject&&coordinatorPlace(),hasAny=(view.agents||[]).some(a=>(['claude','codex','cursor'].includes(a.agent))&&a.cli),folderOk=hasProject&&!!view.folder;
    const who=[coordinatorAgentOn('claude')&&'Claude Code',coordinatorAgentOn('codex')&&'Codex',coordinatorAgentOn('cursor')&&'Cursor'].filter(Boolean).join(', ');
    const failed=linkFailure?.project===view.project?linkFailure:null;
    const codexBare=coordinatorAgentOn('codex')&&!coordinatorAgentOn('claude')&&!coordinatorAgentOn('cursor')&&(view.agents||[]).some(a=>a.agent==='codex'&&a.trust?.state==='not_trusted');
    const coordSub=!hasProject?tr('Сначала создайте проект'):codexBare?tr('Записано. Подтвердите доверие папке кнопкой «Доверять» на карточке Codex во вкладке «Агенты».'):place?(who?tr('Подключён: ')+who:tr('Подключён на этом компьютере')):(failed?tr('Не получилось: ')+failed.detail:tr('Подключите Claude Code, Codex или Cursor координатором, чтобы давать задачи с этого компьютера'));
    step(!!place&&!codexBare,tr('Координатор'),coordSub,!hasProject||place||codexBare?'':(hasAny&&folderOk&&!linkBusy)?tr('Подключить'):hasAny?tr('Проект'):tr('Агенты'),()=>(hasAny&&folderOk&&!linkBusy)?connectReadyCoordinators():show(hasAny?'project':'agents'));
    const peers=(server?.peers||[]).length;
    step(peers>0,tr('Другие компьютеры'),peers?tr('В сети: ')+peers:tr('Необязательно: объедините компьютеры через сервер'),peers?'':tr('Сервер'),()=>show('server'));
    box.append(el('h3',tr('С чего начать')),steps);
    if(place&&!codexBare)box.append(el('p',tr('Откройте папку проекта в Claude Code, Codex или Cursor, начните новый чат и спросите: «Каких агентов ты видишь?»'),'am-small am-muted'));
    return box;
  }
  function renderThread(){
    const thread=$('thread'),head=$('task-head');
    const panelOpen=!!view?.project&&$('list-toggle').getAttribute('aria-expanded')==='true';
    $('task-panel').hidden=!panelOpen;$('task-layout').classList.toggle('am-queue-collapsed',!panelOpen);
    $('list-toggle').hidden=tab!=='tasks'||!view?.project;
    if(view?.project&&selectedTask&&feed?.task.id===selectedTask&&selectedTaskRow()){
      renderFeed();
      return;
    }
    $('compose').hidden=true;$('lost-bar').hidden=true;feedSignature='';$('to-bottom').hidden=true;
    if(view?.project!==project){head.replaceChildren();showPane(thread,'loading');return;}
    if(!view.project){
      head.replaceChildren();
      showPane(thread,'welcome');return;
    }
    const shared=sharedSelection();
    if(shared){renderSharedTask(shared.t,shared.p);return;}
    head.replaceChildren();
    // A chosen task whose conversation has not arrived keeps the previous
    // conversation on screen; an empty pane shows the loading indicator.
    if(selectedTask){if(!thread.firstElementChild||thread.firstElementChild.dataset.pane)showPane(thread,'loading');return;}
    showPane(thread,showWelcome()?'welcome':'empty');
  }
  // The welcome explains the setup until this computer has seen its first
  // task or serves the network; afterwards an empty project just says so
  // (owner, 2026-11-23).
  let welcomeDone=recall('messenger-welcome-done')==='1';
  function noteWelcomeDone(){
    if(welcomeDone)return;
    if((view?.tasks||[]).length||(view?.shared||[]).some(p=>(p.tasks||[]).length)||server?.host?.running){welcomeDone=true;remember('messenger-welcome-done','1');}
  }
  function showWelcome(){noteWelcomeDone();return !welcomeDone;}
  function showPane(thread,kind){
    if(kind!=='welcome'&&thread.firstElementChild?.dataset.pane===kind&&thread.childElementCount===1)return;
    let pane;
    if(kind==='welcome')pane=welcome();
    else if(kind==='loading'){
      pane=el('div',undefined,'am-pane-loading');pane.setAttribute('role','status');pane.setAttribute('aria-label',tr('Загрузка…'));
      const spin=el('span',undefined,'am-spinner');spin.setAttribute('aria-hidden','true');pane.append(spin);
    }else{
      pane=el('div',undefined,'am-pane-empty');
      pane.append(el('p',tr('Задач пока нет'),'am-pane-empty-title am-muted'));
    }
    pane.dataset.pane=kind;thread.replaceChildren(pane);
  }
  // The window response owns the task row. The feed response supplies
  // history, which can arrive before or after the row for the same task.
  function selectedTaskRow(){if(view?.project!==project)return;return view?.tasks?.find(t=>t.id===selectedTask)||currentTaskHistory()?.rows.get(selectedTask)||(feed?.task.id===selectedTask?feed.task:null);}
  // The chosen task of another computer, if it is still in the lists.
  function sharedSelection(){
    if(!selectedShared)return null;
    const p=(view?.shared||[]).find(x=>x.peer===selectedShared.peer),t=p?.tasks.find(x=>x.id===selectedShared.id);
    return t?{t,p}:null;
  }
  // Another computer's task (T-111): the window has only what that computer
  // shares, so the card shows it and says where the full history is.
  function renderSharedTask(t,p){
    const thread=$('thread'),head=$('task-head');
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    title.append(el('p',(names[t.agent]||t.agent)+(t.model?' ('+modelName(t.model)+')':'')+tr(' на «')+sharedComputer(t,p)+tr('», задачу дал компьютер «')+p.name+'»','am-small am-muted'));
    const [statusText,statusCls,statusTip,from]=sharedStatus(t,p);
    const when=t.finished_at?', '+clock(taskStart(t))+'–'+clock(t.finished_at):tr(', начата в ')+clock(taskStart(t));
    const line=el('p',undefined,'am-small');line.append(statusLabel(statusText,statusCls,from),el('span',when,'am-muted'));
    if(statusTip)line.append(el('span',', '+statusTip,'am-muted'));
    if(t.tokens)line.append(el('span',', '+tokensText(t.tokens)+tr(' токенов'),'am-muted'));
    title.append(line);
    const headRow=el('div',undefined,'am-task-head-row');headRow.append(title);
    head.replaceChildren(headRow);
    const box=el('div',undefined,'am-settings');
    if(t.report){box.append(el('h3',tr('Начало отчёта')),el('p',t.report,'am-shared-report'));}
    else box.append(el('p',tr('Отчёта пока нет.'),'am-muted'));
    box.append(el('p',tr('Полная история задачи — в окне компьютера «')+p.name+tr('». Здесь её можно только посмотреть.'),'am-small am-muted'));
    thread.replaceChildren(box);$('compose').hidden=true;$('lost-bar').hidden=true;
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
  // The command itself, without the shell that runs it.
  function commandText(line){
    const m=line.match(/^"?[^"]*(?:powershell|pwsh)(?:\.exe)?"?\s+-Command\s+([\s\S]*)$/i)||line.match(/^"?\S*\/(?:ba|z)?sh"?\s+-l?c\s+([\s\S]*)$/);
    if(!m)return line;
    const c=m[1].trim(),q=c[0];
    return (q==="'"||q==='"')&&c.endsWith(q)&&c.length>1?c.slice(1,-1):c;
  }
  const actionKinds={
    search:{icon:['M15 15l5 5M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0']},
    read:{icon:['M4 4h5a3 3 0 0 1 3 3v13a4 4 0 0 0-4-2H4zM20 4h-5a3 3 0 0 0-3 3v13a4 4 0 0 1 4-2h4z']},
    folder:{icon:['M3 7h7l2 2h9v10H3z']},
    edit:{icon:['M4 20l4-1L20 7l-3-3L5 16zM15 6l3 3']},
    script:{icon:['M4 5h16v14H4zM7 9l3 3-3 3m5 0h5']},
    test:{icon:['M4 12l5 5L20 6']},
    tool:{icon:['M8 7l-5 5 5 5m8-10 5 5-5 5']},
    command:{icon:['M4 5h16v14H4zM7 9l3 3-3 3m5 0h5']},
  };
  function actionFile(path){
    return (path||'').replace(/#cs:\d+$/,'').split(/[\\/]/).filter(Boolean).pop()||'';
  }
  function actionTitle(prefix,target){
    return prefix+(target?' '+target:'');
  }
  // Recognize a few common forms. Anything ambiguous keeps a neutral label.
  // The text is display data only and is never evaluated.
  function commandAction(raw){
    const first=commandText(raw.trim()).split(/\r?\n/,1)[0].trim();
    const words=(first.match(/'(?:[^']*)'|"(?:\\.|[^"\\])*"|&&|\|\||[|;&]|[^\s"'|;&]+/g)||[])
      .map(word=>/^(?:'[^']*'|"(?:\\.|[^"\\])*")$/.test(word)?word.slice(1,-1):word);
    const executable=actionFile(words[0]).replace(/\.exe$/i,'');
    if(/^(?:python(?:3(?:\.\d+)?)?|node|ruby)$/i.test(executable)){
      const lang=/^python/i.test(executable)?'Python':executable==='node'?'Node.js':'Ruby';
      return ['script',tr('Запустить скрипт {language}').replace('{language}',lang)];
    }
    if((executable==='go'&&words[1]==='test')||(executable==='pytest')){
      return ['test',tr('Запустить тесты')];
    }
    if(words.some(word=>['&&','||',';', '&'].includes(word))){
      return ['command',actionTitle(tr('Выполнить команду'),executable)];
    }
    const pipe=words.indexOf('|'), reader=executable==='cm'&&words[1]==='cat';
    if(executable==='rg'&&words[1]==='--files'){
      const filter=pipe===2&&words[3]==='rg'&&words.length===5?words[4].replace(/\\\./g,'.').replace(/\$$/,''):'';
      return ['search',/^[\w.-]+$/.test(filter)?actionTitle(tr('Найти файл'),filter):tr('Найти файлы')];
    }
    if((executable==='rg'&&pipe<0)||(reader&&words[pipe+1]==='rg'&&pipe===3)){
      const args=words.slice(reader?pipe+2:1);
      while(args[0]?.startsWith('-')){
        const option=args.shift();
        if(option==='--')break;
        if(/^-(?:C|A|B|m|g)$/.test(option))args.shift();
      }
      const candidate=reader?words[2]:args[1];
      const file=candidate?.startsWith('-')?'':actionFile(candidate);
      const terms=args[0]?.split('|');
      const shown=terms?.every(term=>/^[\p{L}\p{N}_ -]{1,32}$/u.test(term))&&terms.length<=3
        ? new Intl.ListFormat(amLocale).format(terms):'';
      if(shown)return ['search',actionTitle(tr('Найти упоминания'),shown)+(file?tr(' в файле ')+file:'')];
      return ['search',file?actionTitle(tr('Найти совпадения в'),file):tr('Поиск по содержимому')];
    }
    if(pipe<0&&(reader||executable==='cat')){
      const target=words.find((word,i)=>i>=(reader?2:1)&&!word.startsWith('-'))||'';
      const revision=target.match(/#cs:(\d+)$/)?.[1];
      const label=actionTitle(tr('Прочитать файл'),actionFile(target));
      return ['read',revision?label+tr(' из версии ')+revision:label];
    }
    if(executable==='ls'&&pipe<0){
      const path=words.find((word,i)=>i>0&&!word.startsWith('-'));
      const folder=path?.split(/[\\/]/).filter(Boolean).slice(-2).join('/');
      return ['folder',folder?actionTitle(tr('Посмотреть файлы в папке'),folder):tr('Посмотреть файлы в папке')];
    }
    return ['command',actionTitle(tr('Выполнить команду'),executable)];
  }
  function isCommandTool(name){return ['bash',"команда",'shell'].includes(name);}
  function toolAction(it){
    const name=(it.title||'').toLowerCase();
    const raw=it.text||'';
    const target=raw.split('\n',1)[0];
    let kind,label;
    // Names in old feed records include fixed Russian titles.
    if(isCommandTool(name)){
      [kind,label]=commandAction(raw);
    }else if(name==='read'||name==='readfile'){
      kind='read';label=actionTitle(tr('Прочитать файл'),actionFile(target));
    }else if(['edit','multiedit','write',"изменение файлов"].includes(name)){
      kind='edit';label=actionTitle(tr('Изменить файл'),actionFile(target));
      if(raw.includes('\n'))label=tr('Изменить файлы');
    }else if(['grep','glob',"поиск в интернете",'websearch'].includes(name)){
      kind='search';label=actionTitle(name==='glob'?tr('Найти файлы'):tr('Поиск по запросу'),target);
    }else{
      kind='tool';
      if(name==='webfetch')label=actionTitle(tr('Открыть страницу'),target);
      else if(name==='task')label=actionTitle(tr('Запустить подзадачу'),target);
      else if(name==='todowrite')label=tr('Обновить план');
      else label=it.title||tr('Вызвать инструмент');
    }
    const caption=it.description?.trim()||label;
    return {kind,label:caption.length>120?caption.slice(0,119)+'…':caption};
  }
  function toolLine(it,action){
    const icon=svgIcon(actionKinds[action.kind].icon);
    icon.classList.add('am-action-icon');
    const row=el(it.text?'details':'div',undefined,'am-tool am-action-entry');
    const label=el(it.text?'summary':'div',undefined,'am-action-label');
    label.append(icon,el('span',action.label,'am-tool-name'));
    row.append(label);
    if(it.text)row.append(el('pre',it.text));
    return row;
  }
  function actionCount(count,forms){
    const form=new Intl.PluralRules(amLocale).select(count);
    return count+' '+forms[form==='one'?0:form==='few'?1:2];
  }
  function commandElapsed(start){
    const seconds=Math.max(0,Math.floor((Date.now()-start)/1000));
    const minutes=Math.floor(seconds/60);
    return (minutes?minutes+tr(' мин')+' ':'')+(seconds%60)+tr(' с');
  }
  function updateTimers(){
    // T-275: a hidden window draws nothing; it catches up when shown.
    if(document.hidden)return;
    const now=Date.now();
    for(const label of root.querySelectorAll('[data-elapsed-from]')){
      const text=label.dataset.elapsedPrefix+minutes(label.dataset.elapsedFrom,null,now);
      if(text!==label.textContent)label.textContent=text;
    }
    for(const timer of $('thread').querySelectorAll('[data-command-start]')){
      timer.textContent=commandElapsed(Number(timer.dataset.commandStart));
    }
  }
  // Stop, like in a chat: a square button in the input field, or Esc.
  let stopping=false;
  function closeHere(){
    const t=selectedTaskRow();if(!t)return;
    if(!t.remote||t.remote.from){
      if(t.state!=='blocked'&&t.state!=='interrupted')return;
      askConfirm(tr('Закрыть задачу?'),tr('Messenger проверит, что процесса исполнителя на этом компьютере нет, и закроет задачу. Если процесс ещё есть, задача останется как была.')+(t.remote?.from?' '+tr('Компьютер «')+t.remote.name+tr('» узнает, что задача закрыта.'):''),tr('Закрыть'),async()=>{
        await postAPI('/api/task-cancel',{task:t.id});feedSignature='';await refresh();
      });
      return;
    }
    askConfirm(tr('Закрыть задачу?'),(t.remote.silent&&!t.remote.lost?tr('Компьютер «')+t.remote.name+tr('» не отвечает по этой задаче.'):tr('С компьютером «')+t.remote.name+tr('» нет связи.'))+' '+tr('Задача закроется в этом окне, а что с ней на том компьютере, неизвестно. Если связь вернётся, ему придёт отмена.'),tr('Закрыть'),async()=>{
      await postAPI('/api/task-cancel',{task:t.id});feedSignature='';await refresh();
    });
  }
  async function stopTask(){
    const t=selectedTaskRow();if(!t||stopping||$('task-stop').hidden)return;
    stopping=true;$('task-stop').disabled=true;$('task-stop').classList.add('am-tip-off');
    try{await postAPI('/api/task-cancel',{task:t.id});showError(null);feedSignature='';await refresh();}
    catch(e){showError(e);}
    finally{stopping=false;$('task-stop').disabled=false;}
  }
  // T-175: the worker's pictures. The page asks for them with its key and
  // shows them from memory; a click opens the viewer (T-268).
  const pictureURLs=new Map();
  async function loadPicture(id){
    if(pictureURLs.has(id))return pictureURLs.get(id);
    try{
      const response=await fetch('/api/task-image?id='+encodeURIComponent(id),{headers:{Authorization:'Bearer '+token},signal:AbortSignal.timeout(15000)});
      if(!response.ok)return '';
      const url=URL.createObjectURL(await response.blob());pictureURLs.set(id,url);return url;
    }catch(_){return '';}
  }
  // T-268: a picture opens in a large viewer. The wheel, the buttons or a
  // click zoom in; a zoomed picture is dragged; arrows step through the
  // pictures of the same message; Escape or a click beside it closes it.
  const viewer={list:[],at:0,scale:1,x:0,y:0};
  function viewerApply(){
    const img=$('picture-image');
    if(viewer.scale<=1){viewer.scale=1;viewer.x=0;viewer.y=0;}
    img.style.transform=`translate(${viewer.x}px,${viewer.y}px) scale(${viewer.scale})`;
    $('picture-stage').classList.toggle('am-zoomed',viewer.scale>1);
    $('picture-zoom-reset').textContent=Math.round(viewer.scale*100)+'%';
    $('picture-zoom-out').disabled=viewer.scale<=1;
    $('picture-zoom-in').disabled=viewer.scale>=8;
  }
  // Zoom keeps the point under the cursor (or the centre) in place.
  function viewerZoom(scale,cx,cy){
    const next=Math.min(8,Math.max(1,scale));
    const box=$('picture-stage').getBoundingClientRect();
    const px=(cx??box.left+box.width/2)-(box.left+box.width/2),py=(cy??box.top+box.height/2)-(box.top+box.height/2);
    viewer.x=px-(px-viewer.x)*next/viewer.scale;viewer.y=py-(py-viewer.y)*next/viewer.scale;
    viewer.scale=next;viewerApply();
  }
  function viewerShow(at){
    const n=viewer.list.length;if(!n)return;
    viewer.at=(at+n)%n;viewer.scale=1;
    const p=viewer.list[viewer.at],img=$('picture-image');
    img.alt=p.name||'';img.removeAttribute('src');
    $('picture-name').textContent=p.name||tr('Картинка');
    $('picture-count').textContent=n>1?tr('{n} из {total}').replace('{n}',viewer.at+1).replace('{total}',n):'';
    $('picture-prev').hidden=$('picture-next').hidden=n<2;
    const known=pictureURLs.get(p.id);
    if(known)img.src=known;else loadPicture(p.id).then(url=>{if(url&&viewer.list[viewer.at]===p)img.src=url;});
    viewerApply();
  }
  function openPicture(list,at){
    viewer.list=list;viewerShow(at);
    const d=$('picture-dialog');if(!d.open)d.showModal();
    $('picture-close').focus();
  }
  function setupPictureViewer(){
    const d=$('picture-dialog'),stage=$('picture-stage'),img=$('picture-image');
    $('picture-close').addEventListener('click',()=>d.close());
    $('picture-prev').addEventListener('click',()=>viewerShow(viewer.at-1));
    $('picture-next').addEventListener('click',()=>viewerShow(viewer.at+1));
    $('picture-zoom-in').addEventListener('click',()=>viewerZoom(viewer.scale*1.5));
    $('picture-zoom-out').addEventListener('click',()=>viewerZoom(viewer.scale/1.5));
    $('picture-zoom-reset').addEventListener('click',()=>viewerZoom(1));
    d.addEventListener('close',()=>{viewer.list=[];img.removeAttribute('src');});
    d.addEventListener('keydown',e=>{
      if(e.key==='ArrowLeft'){e.preventDefault();viewerShow(viewer.at-1);}
      else if(e.key==='ArrowRight'){e.preventDefault();viewerShow(viewer.at+1);}
      else if(e.key==='+'||e.key==='='){e.preventDefault();viewerZoom(viewer.scale*1.5);}
      else if(e.key==='-'){e.preventDefault();viewerZoom(viewer.scale/1.5);}
      else if(e.key==='0'){e.preventDefault();viewerZoom(1);}
    });
    // A click on the dimmed area outside the window closes it.
    d.addEventListener('click',e=>{if(e.target===d){const r=d.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)d.close();}});
    stage.addEventListener('wheel',e=>{e.preventDefault();viewerZoom(viewer.scale*Math.exp(-e.deltaY*(e.ctrlKey?0.01:0.002)),e.clientX,e.clientY);},{passive:false});
    let drag=null;
    stage.addEventListener('pointerdown',e=>{
      if(e.button!==0||e.target.closest('button'))return;
      drag={id:e.pointerId,sx:e.clientX,sy:e.clientY,x:viewer.x,y:viewer.y,moved:false,onImage:e.target===img};
      stage.setPointerCapture(e.pointerId);
    });
    stage.addEventListener('pointermove',e=>{
      if(!drag||drag.id!==e.pointerId)return;
      const dx=e.clientX-drag.sx,dy=e.clientY-drag.sy;
      if(!drag.moved&&Math.hypot(dx,dy)<4)return;
      drag.moved=true;
      if(viewer.scale>1){stage.classList.add('am-dragging');viewer.x=drag.x+dx;viewer.y=drag.y+dy;viewerApply();}
    });
    const end=e=>{
      if(!drag||drag.id!==e.pointerId)return;
      const was=drag;drag=null;stage.classList.remove('am-dragging');
      if(e.type!=='pointerup'||was.moved)return;
      // A plain click: on the picture it zooms in or back; beside it, closes.
      if(was.onImage)viewerZoom(viewer.scale>1?1:2.5,e.clientX,e.clientY);
      else if(viewer.scale<=1)d.close();
    };
    stage.addEventListener('pointerup',end);
    stage.addEventListener('pointercancel',end);
  }
  function pictures(list){
    const box=el('div',undefined,'am-pictures');
    for(const p of list){
      const img=document.createElement('img');img.className='am-picture';img.alt=p.name;img.title=p.name;
      img.addEventListener('click',()=>openPicture(list,list.indexOf(p)));
      const known=pictureURLs.get(p.id);
      if(known)img.src=known;else loadPicture(p.id).then(url=>{if(url)img.src=url;});
      box.append(img);
    }
    return box;
  }
  // T-172: a run of this computer whose worker went quiet, or wrote its
  // final message while its process kept running. Only described: the run
  // ends on an observed exit, never on this (D-021, Q-030).
  function quietLine(t){
    const q=t.quiet;if(!q||t.state!=='running')return null;
    if(q.final_at){
      const p=el('p',undefined,'am-small');
      p.append(el('span',tr('Итог написан в ')+clock(q.final_at)+tr(', но процесс исполнителя не завершился'),'am-status-wait'));
      const left=(q.processes||[]).map(x=>(x.name||'?')+' ('+x.pid+')'+(x.worker?tr(' — сам исполнитель'):''));
      p.append(el('span','. '+(left.length?tr('Ещё работают: ')+left.join(', ')+'. ':'')+tr('Закройте лишнее на этом компьютере или остановите задачу.'),'am-muted'));
      return p;
    }
    const at=Date.parse(q.output_at||'');if(!at)return null;
    const minutes=Math.floor((Date.now()-at)/60000);if(minutes<10)return null;
    return el('p',tr('Исполнитель ничего не пишет уже ')+minutes+tr(' мин.'),'am-small am-muted');
  }
  function renderFeed(){
    const t=selectedTaskRow(),thread=$('thread'),head=$('task-head');
    const signature=JSON.stringify([t,feed.items,online]);if(signature===feedSignature)return;
    const count=(feed.items||[]).filter(it=>it.kind!=='command_end').length,fresh=feedShown!==t.id,atBottom=fresh||feedStick||feedPinned,grew=!fresh&&!atBottom&&count>feedCount;feedShown=t.id;feedStick=false;feedCount=count;feedSignature=signature;
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    const where=t.remote&&!t.remote.from?tr(' на компьютере «')+t.remote.name+'»':tr(' на ')+(view.computer_name||tr('этом компьютере'))+(t.remote?.from?tr(', по заданию компьютера «')+t.remote.name+'»':'');
    const displayModel=taskModelLabel(t);
    title.append(el('p',(names[t.agent]||t.agent)+(displayModel?' ('+displayModel+')':'')+where+tr(', папка ')+(t.folder||tr('ещё не известна'))+(t.remote&&!t.remote.from?'':t.full_access?tr(', любые команды разрешены'):tr(', без «Любых команд»')),'am-small am-muted'));
    const [statusText,statusCls,statusTip,from]=taskStatus(t);
    const when=t.finished_at?', '+clock(taskStart(t))+'–'+clock(t.finished_at):tr(', начата в ')+clock(taskStart(t));
    const line=el('p',undefined,'am-small');line.append(statusLabel(statusText,statusCls,from),el('span',when,'am-muted'));
    if(statusTip)line.append(el('span',', '+statusTip,'am-muted'));
    if(t.usage){const used=el('span',', '+usageShort(t.usage),'am-muted');used.title=usageLong(t.usage);line.append(used);}
    title.append(line);
    const quiet=quietLine(t);if(quiet)title.append(quiet);
    const headRow=el('div',undefined,'am-task-head-row');headRow.append(title);
    head.replaceChildren(headRow);
    const opened=new Set([...thread.querySelectorAll('details[open][data-key]')].map(d=>d.dataset.key));
    const out=[];let group=null,tools=null;
    const pending=new Map(), unfinished=new Set();
    for(const [index,it] of feed.items.entries()){
      if(it.kind==='command_start'){pending.set(it.title,index);unfinished.add(index);}
      if(it.kind==='command_end'){unfinished.delete(pending.get(it.title));pending.delete(it.title);}
      if(['task','answer','report','question','outcome'].includes(it.kind))pending.clear();
    }
    const running=t.state==='running'&&!t.pending_launch&&!t.remote?.lost&&!t.remote?.silent&&online;
    // Only the latest worker action group can show a timer. Keep matching
    // late command ends so an earlier group can still show its finished count.
    let liveFrom=feed.items.length;
    if(running&&pending.size){
      for(let i=feed.items.length-1;i>=0;i--){
        const {kind}=feed.items[i];
        if(kind==='command_end')continue;
        if(kind==='tool'||kind==='command_start'){liveFrom=i;continue;}
        if(liveFrom===feed.items.length&&(kind==='note'||kind==='accepted'))continue;
        break;
      }
    }
    const live=new Set([...pending.values()].filter(index=>index>=liveFrom));
    // Lifecycle entries supply the heading only. Tool rows still come from the
    // provider's original feed, in the same order and with the same content.
    const collectTools=key=>tools??={key,lines:[],commandsOnly:true,unfinished:false,start:0};
    const flushTools=()=>{
      if(tools?.lines.length||tools?.start){
        const {key,lines,commandsOnly,unfinished,start}=tools;
        const host=workerGroup(tools.at);
        if(!start&&lines.length===1){host.append(lines[0]);}
        else{
          const box=el(lines.length?'details':'div',undefined,'am-tool-group');
          box.dataset.key='g'+key;
          const summary=el(lines.length?'summary':'div');
          const label=start?tr('Выполняются команды'):commandsOnly&&!unfinished
            ?tr('Выполнено {commands}').replace('{commands}',actionCount(lines.length,[tr('команда'),tr('команды'),tr('команд')]))
            :actionCount(lines.length,[tr('действие'),tr('действия'),tr('действий')]);
          const heading=el('span',undefined,'am-tool-group-title');
          heading.append(el('span',label,start?'am-tool-running-label':''));
          if(start){
            const timer=el('span',commandElapsed(start),'am-tool-timer');
            timer.dataset.commandStart=start;
            heading.append(timer);
          }
          summary.append(heading);box.append(summary,...lines);host.append(box);
        }
      }
      tools=null;
    };
    const messageAvatar=provider=>{
      const avatar=el('div',undefined,'am-avatar');
      avatar.append(providerIcon(provider));
      return avatar;
    };
    const outgoing=(author,role,text,at,model,provider)=>{
      group=null;const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message am-outgoing');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      if(provider)meta.append(messageAvatar(provider));
      const who=el('strong',author);meta.append(who);if(model)meta.append(el('span',modelName(model),'am-meta-model'));if(role)meta.append(roleTag(role));meta.append(el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');content.append(richText(text));body.append(meta,content);row.append(body);g.append(row);out.push(g);
    };
    const outgoingCoordinator=it=>{
      const sender=shortNames[it.sender_agent];
      outgoing(sender||tr('Координатор'),sender?tr('координатор'):'',it.text,it.at,sender?it.sender_model:'',sender?it.sender_agent:'');
    };
    const workerGroup=at=>{
      if(group)return group;
      const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      meta.append(messageAvatar(t.agent),el('strong',names[t.agent]||t.agent));
      if(displayModel){const m=el('span',displayModel,'am-meta-model');m.title=taskModelDetail(t);meta.append(m);}
      meta.append(roleTag(tr('исполнитель')),el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');body.append(meta,content);row.append(body);g.append(row);out.push(g);group=content;return content;
    };
    const delivery=(text,cls)=>el('div',text,'am-delivery '+cls);
    for(const [index,it] of feed.items.entries()){
      if(it.kind==='command_end')continue;
      if(it.kind==='command_start'){
        const batch=collectTools(index);
        if(unfinished.has(index))batch.unfinished=true;
        const start=Date.parse(it.at||'');
        if(live.has(index)&&start){
          batch.start=batch.start?Math.min(batch.start,start):start;
          batch.at??=it.at;
        }
        continue;
      }
      if(it.kind!=='tool')flushTools();
      if(it.kind==='task'){outgoingCoordinator(it);continue;}
      if(it.kind==='note'){if(it.author==='coordinator')outgoingCoordinator(it);else outgoing(tr('Вы'),'',it.text,it.at);const c=out[out.length-1].querySelector('.am-message-content');c.append(delivery(...({pending:[tr('Исполнитель прочитает, только когда закончит текущий ход'),'am-status-wait'],delivered:[tr('Передано исполнителю'),'am-status-good'],sent:[it.author==='owner'?tr('Поставлено в очередь для исполнителя'):tr('Отправлено на компьютер исполнителя'),'am-status-wait'],undelivered:[tr('Не передано исполнителю'),'am-status-bad']}[it.state]||[it.state,'am-muted'])));continue;}
      if(it.kind==='answer'){if(it.author==='owner'){outgoing(tr('Вы'),'',it.text,it.at);
        // A hint about this answer lives under it while the worker has not
        // gone on yet, so it never stays in another task (owner, 2026-11-09).
        if(feed.items.slice(index+1).every(n=>n.kind==='answer'||n.kind==='note')&&activeStates.includes(t.state))out[out.length-1].querySelector('.am-message-content').append(delivery(tr('Ответ записан. Исполнитель продолжит в той же сессии.'),'am-delivery-hint'));}else outgoingCoordinator(it);continue;}
      if(it.kind==='accepted'){const c=out[out.length-1]?.querySelector('.am-message-content');if(c)c.append(delivery(tr('Принято на компьютере «')+it.text+'»','am-status-good'));group=null;continue;}
      if(it.kind==='text'){workerGroup(it.at).append(richText(it.text));continue;}
      if(it.kind==='tool'){
        const batch=collectTools(index);
        const line=toolLine(it,toolAction(it));line.dataset.key='t'+index;
        batch.at??=it.at;
        batch.lines.push(line);
        batch.commandsOnly&&=isCommandTool((it.title||'').toLowerCase());
        continue;
      }
      if(it.kind==='report'||it.kind==='question'){
        const c=workerGroup(it.at);c.append(richText(it.text,'am-disposition'));
        if(it.images?.length)c.append(pictures(it.images));
        let note;
        if(!it.verified)note=delivery(it.kind==='report'?tr('Отчёт сохранён без подтверждения запуска'):tr('Не опубликовано: запуск не прошёл проверку'),'am-status-bad');
        else if(it.kind==='report'&&it.audience==='held')note=delivery(tr('Промежуточный отчёт: исполнитель продолжает с вашим сообщением'),'am-status-wait');
        else if(it.kind==='report')note=delivery(tr('Отчёт отправлен координатору, сессия закрыта'),'am-delivery-close');
        else if(it.audience==='owner')note=delivery(tr('Вопрос вам, координатор его не видит'),'am-status-wait');
        else note=delivery(tr('Вопрос координатору'),'am-status-wait');
        if(it.verified)appendEventTime(note,it.at,t.remote&&!t.remote.from);
        c.append(note);
        group=null;continue;
      }
      if(it.kind==='outcome'){
        group=null;
        const overloaded=it.state==='failed'&&capacityFailure(it.text);
        const failedTitle=overloaded?tr('Завершение запуска не подтверждено'):t.remote&&!t.remote.from?tr('Задача не выполнена'):tr('Запуск завершился с ошибкой');
        const text={failed:failedTitle,blocked:t.remote&&!t.remote.from?tr('Задача заблокирована: результат на другом компьютере неизвестен'):tr('Задача заблокирована: процесс не завершился полностью'),interrupted:tr('Клиент перезапустился во время работы; результат неизвестен'),cancelled:tr('Задача отменена')}[it.state]||it.state;
        const note=el('div',undefined,'am-outcome '+(it.state==='cancelled'?'am-muted':'am-status-bad')),line=el('p',text);
        appendEventTime(line,it.at,t.remote&&!t.remote.from);note.append(line);
        const why=t.remote&&!t.remote.from?refusalText(it.text,t.remote.name)||failureText(it.text):failureText(it.text);if(why)note.append(el('p',why));
        if(it.text)note.append(el('p',it.text,'am-small am-muted'));
        const g=el('article',undefined,'am-message-group');g.append(note);out.push(g);
      }
    }
    flushTools();
    thread.replaceChildren(...out);
    for(const d of thread.querySelectorAll('details[data-key]'))if(opened.has(d.dataset.key))d.open=true;
    // A task another computer started is answered there.
    const q=t.remote?.from?null:t.owner_question;
    // T-131: a task sent to another computer can be closed here also when
    // its outcome there is already unknown.
    // T-131: an own task on another computer that cannot be reached has no
    // input and no stop button: nothing would arrive. One action closes it here.
    const unreachable=!!t.remote&&!t.remote.from&&(t.state==='blocked'||t.state==='interrupted'||(activeStates.includes(t.state)&&(!!t.remote.lost||!!t.remote.silent)));
    // D-120: a stuck task of this computer is closed by hand once its worker
    // is certainly gone; the client checks that and refuses otherwise.
    const stuckHere=!unreachable&&(t.state==='blocked'||t.state==='interrupted');
    $('lost-bar').hidden=!unreachable&&!stuckHere;
    if(stuckHere)$('lost-text').textContent=(t.state==='interrupted'?tr('Клиент перезапустился во время работы, и исход задачи неизвестен.'):tr('Процесс исполнителя не завершился полностью, исход задачи неизвестен.'))+' '+tr('Закрыть её можно, когда исполнителя на этом компьютере точно нет.');
    if(unreachable){
      const since=t.remote.last_seen?stamp(t.remote.last_seen):'';
      if(t.remote.silent&&!t.remote.lost)$('lost-text').textContent='«'+t.remote.name+tr('» на связи, но не отвечает по этой задаче. Что с ней там, неизвестно: она могла и выполниться.');
      else $('lost-text').textContent=(since?tr('Связи с «')+t.remote.name+tr('» нет с ')+since+'. ':tr('Связи с «')+t.remote.name+tr('» нет. '))+tr('Что с задачей там, неизвестно.');
    }
    const active=activeStates.includes(t.state)&&!unreachable,box=$('answer-text');
    const note=canNote(t),wasNote=$('compose').dataset.mode==='note';
    $('compose').hidden=unreachable||(!q&&!active);
    box.disabled=!q&&!note;box.placeholder=q?tr('Ответить исполнителю…'):note&&t.state==='waiting_coordinator'?tr('Ответить вместо координатора…'):note?tr('Написать исполнителю…'):t.remote?.from&&t.owner_question?tr('Отвечают на компьютере «')+t.remote.name+'»':t.pending_launch?tr('Задача ждёт запуска…'):t.remote?.outbox_delayed_minutes&&!t.remote.from?tr('Ответы с «')+t.remote.name+tr('» задерживаются: не уходят уже ')+t.remote.outbox_delayed_minutes+tr(' мин.'):t.remote?.worker==='none'&&!t.remote.from?tr('На «')+t.remote.name+tr('» исполнитель этой задачи сейчас не запущен'):tr('Исполнитель работает…');
    $('answer-send').hidden=!q&&!note;$('task-stop').hidden=!active;
    $('compose').dataset.mode=q?'answer':note?'note':'';
    if($('compose').dataset.task!==t.id){$('compose').dataset.task=t.id;box.value='';$('answer-status').textContent='';}
    if(q&&$('compose').dataset.question!==q.id){$('compose').dataset.question=q.id;$('answer-text').value='';$('answer-status').textContent='';}
    else if(!q&&!note&&wasNote){$('answer-text').value='';}
    sizeAnswer();answerControls();
    if(atBottom){thread.scrollTop=thread.scrollHeight;feedPinned=true;}
    else if(grew)$('to-bottom').classList.add('am-has-new');
    toBottomState();
  }
  function nearBottom(thread){return thread.scrollHeight-thread.clientHeight-thread.scrollTop<40;}
  // The down button shows while the owner reads higher up in a task.
  function toBottomState(){
    const thread=$('thread'),b=$('to-bottom');feedPinned=nearBottom(thread);const show=!!feed&&!!selectedTask&&!feedPinned;
    b.hidden=!show;if(!show)b.classList.remove('am-has-new');
  }
  // Why a run was not accepted (T-091), in the window's language.
  function capacityFailure(text){
    return /the model provider was overloaded before confirming the worker's turn|Codex (?:error|turn\.failed):[^\n]*Selected model is at capacity/i.test((text||'').split('\nUnverified text from the worker:',1)[0]);
  }
  function failureText(text){
    text=text||'';const code=text.match(/stopped with an error \(exit code (\d+)\)/);
    if(capacityFailure(text))return tr('Модель перегружена. Провайдер не подтвердил завершение хода.');
    if(/not signed in/.test(text))return tr('Агент не вошёл в аккаунт на этом компьютере. Войдите на вкладке «Агенты».');
    if(/stopped at a provider limit/.test(text))return tr('Агент упёрся в лимит.');
    if(/different conversation/.test(text))return tr('Агент ответил не в той сессии, которую запустил Messenger; результату нельзя доверять.');
    if(code)return tr('Программа агента завершилась с ошибкой, код ')+code[1]+'.';
    if(/did not report a successful end/.test(text))return tr('Агент не сообщил об успешном окончании работы: возможно, упал или был остановлен.');
    if(/ended without sending its report/.test(text))return tr('Исполнитель закончил, но не отправил отчёт через Messenger. Координатор может попросить его прислать отчёт в той же сессии.');
    return '';
  }
  // Known refusals of another computer, explained in the window's language.
  function refusalText(text,name){
    if(/does not run tasks of other computers/.test(text||''))return tr('Компьютер «')+name+tr('» не принимает задачи других компьютеров. Включите на нём «Принимать задачи других компьютеров» на вкладке «Сервер» и дайте задачу снова.');
    if(/has no project/.test(text||''))return tr('На компьютере «')+name+tr('» нет проекта с таким названием. Создайте его там и выберите папку.');
    if(/has no folder for the project|project folder is missing/.test(text||''))return tr('На компьютере «')+name+tr('» у проекта не выбрана папка или её нет. Выберите папку проекта там и дайте задачу снова.');
    if(/is not set up on this computer/.test(text||''))return tr('На компьютере «')+name+tr('» этот агент не настроен. Настройте его там на вкладке агентов.');
    return '';
  }
  // The reply box is one line and grows only while its text does not fit;
  // the stop and send buttons stay at its bottom.
  function sizeAnswer(){const f=$('answer-text');f.style.height='';f.style.overflowY='';if(!f.value)return;const full=f.scrollHeight+(f.offsetHeight-f.clientHeight);f.style.height=Math.min(140,full)+'px';if(full>140)f.style.overflowY='auto';}
  // Owner notes follow a task given here, including one running on a peer.
  function canNote(t){
    if(!t||t.owner_question)return false;
    if(!t.remote)return t.state==='running'||t.state==='waiting_coordinator';
    return t.state==='running'&&!t.remote.from&&!t.remote.lost&&!t.remote.silent;
  }
  function answerControls(){const t=selectedTaskRow();$('answer-send').disabled=answering||!$('answer-text').value.trim()||!(t?.owner_question&&!t.remote?.from||canNote(t));}
  async function sendNote(){
    const t=selectedTaskRow();if(!canNote(t)||answering)return;
    const text=$('answer-text').value.trim();if(!text)return;
    const key='messenger-note-op:'+t.id+':'+text;
    let op=recall(key);
    if(!op){op='window-'+t.id+'-'+crypto.randomUUID();remember(key,op);}
    answering=true;answerControls();$('answer-status').textContent=tr('Отправляем сообщение…');
    try{await postAPI('/api/task-note',{task:t.id,op_id:op,text});try{localStorage.removeItem(key);}catch(_){}if(selectedTask===t.id){showError(null);$('answer-status').textContent='';$('answer-text').value='';sizeAnswer();feedSignature='';feedStick=true;}await refresh();}
    catch(error){if(selectedTask===t.id)$('answer-status').textContent='';showError({message:error.message+tr(' Повтор с тем же текстом безопасен.'),command:error.command,hint:error.hint});}
    finally{answering=false;answerControls();}
  }
  async function sendAnswer(){
    const t=selectedTaskRow(),q=t?.owner_question;
    if(!q&&canNote(t))return sendNote();
    if(!q||answering||t.remote?.from)return;
    const text=$('answer-text').value.trim();if(!text)return;
    let op=recall('messenger-answer-op:'+q.id);
    if(!op){op='window-'+q.id+'-'+crypto.randomUUID();remember('messenger-answer-op:'+q.id,op);}
    answering=true;answerControls();$('answer-status').textContent=tr('Отправляем ответ…');
    try{await postAPI('/api/task-answer',{task:t.id,question:q.id,op_id:op,text});if(selectedTask===t.id){showError(null);$('answer-status').textContent='';$('answer-text').value='';sizeAnswer();feedSignature='';feedStick=true;}await refresh();}
    catch(error){if(selectedTask===t.id)$('answer-status').textContent='';showError({message:error.message+tr(' Повтор с тем же текстом безопасен.'),command:error.command,hint:error.hint});}
    finally{answering=false;answerControls();}
  }
  // Agent cards (T-061, D-070): the agent's name with a check, settings and
  // an on/off switch; four steps of the same height (program, sign-in, work
  // with Messenger, limit) with a mark on the left and one action on the
  // right; one «Задачи» button under them. Steps that cannot be checked yet
  // are grey and numbered.
  const checkPath=['M21 12a9 9 0 1 1-3-6.7L21 8','M21 3v5h-5'];
  const gearPath=['M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z'];
  // The update mark: a down arrow.
  function appendUpdateGlyphs(node){node.append(svgIcon(['M12 4v11M7 10l5 5 5-5M5 20h14']));}
  function svgIcon(paths,circle){
    const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('aria-hidden','true');svg.setAttribute('focusable','false');
    if(circle){const c=document.createElementNS('http://www.w3.org/2000/svg','circle');c.setAttribute('cx','12');c.setAttribute('cy','12');c.setAttribute('r','3');svg.append(c);}
    for(const d of paths){const p=document.createElementNS('http://www.w3.org/2000/svg','path');p.setAttribute('d',d);svg.append(p);}return svg;
  }
  function iconButton(label,paths,circle,fn){const b=el('button',undefined,'am-icon-btn am-card-icon');b.type='button';b.title=label;b.setAttribute('aria-label',label);b.append(svgIcon(paths,circle));b.addEventListener('click',fn);return b;}
  // The limit ring: green below 60%, yellow to 85%, red above (D-070).
  function limitLevel(l){
    if(!l)return '';
    if(typeof l.used_percent==='number')return l.used_percent>85?'bad':l.used_percent>=60?'wait':'good';
    return l.state==='exhausted'?'bad':l.state==='warning'?'wait':'good';
  }
  // Which limit the agent named: Claude's rateLimitType or Codex's window
  // length (T-080). Unknown names are left out.
  function limitWindow(w){
    if(!w)return '';
    if(w==='five_hour'||w==='5h')return tr('Лимит на 5 часов');
    if(w==='seven_day'||w==='7d')return tr('Недельный лимит');
    if(w==='seven_day_opus')return tr('Недельный лимит Opus');
    if(w==='seven_day_sonnet')return tr('Недельный лимит Sonnet');
    return '';
  }
  function limitRing(l){
    const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 18 18');svg.setAttribute('class','am-ring');svg.setAttribute('aria-hidden','true');
    const ring=(cls,share)=>{const c=document.createElementNS('http://www.w3.org/2000/svg','circle');c.setAttribute('cx','9');c.setAttribute('cy','9');c.setAttribute('r','7');c.setAttribute('class',cls);
      if(share!==undefined){const len=2*Math.PI*7;c.setAttribute('stroke-dasharray',len.toFixed(2));c.setAttribute('stroke-dashoffset',(len*(1-share)).toFixed(2));c.setAttribute('transform','rotate(-90 9 9)');}svg.append(c);};
    ring('am-ring-track');
    if(l){const p=typeof l.used_percent==='number'?Math.min(100,Math.max(0,l.used_percent))/100:l.state==='exhausted'?1:0;if(p>0)ring('am-ring-value am-ring-'+limitLevel(l),p);}
    return svg;
  }
  function dayClock(at){const d=new Date(at);if(isNaN(d.getTime()))return '';return d.toDateString()===new Date().toDateString()?clock(at):stamp(at);}
  async function setAgentOn(provider,on){
    const a=(view?.agents||[]).find(x=>x.agent===provider);
    if(!a){if(on)await connectAgent(provider);return;}
    try{await postAPI('/api/agent-pause',{provider,paused:!on});viewSignature='';await refresh();}
    catch(error){showError(error);}
  }
  function uninstallAgent(provider){
    // Like installing, removing runs in a terminal: it shows what it will
    // remove and asks there (owner, 2026-11-15: no web pages).
    askConfirm(tr('Удалить ')+names[provider]+' CLI?',tr('Откроется терминал: он покажет, что будет удалено, и спросит подтверждение. Вход в агент и настройки Messenger сохранятся.'),tr('Открыть терминал'),async()=>{
      try{await postAPI('/api/agent-uninstall',{provider});noteRow(provider+':auth',tr('Терминал удаления открыт'),15000);}
      catch(error){showError(error);}
    });
  }
  function renderAgents(){
    const live=$('agents'),cards=live.cloneNode(false);
    if(computerFilter&&computerFilter!=='local'){renderPeerAgents(cards);swapChildren(live,cards);return;}
    for(const provider of ['claude','codex','cursor']){
      const a=(view?.agents||[]).find(x=>x.agent===provider);
      const found=discovery.find(d=>d.provider===provider);
      const cli=a?.cli||found?.cli||'',profile=a?.environment||found?.environment||'';
      const missing=!a&&found?.state==='not_found';
      const on=!!a&&!a.paused;
      const card=el('article',undefined,'am-card am-agent-card'+(on||!cli?'':' am-agent-off'));
      card.dataset.provider=provider;
      // Paths also identify the action handlers, even when the visible text is unchanged.
      card.dataset.selection=JSON.stringify([cli,profile]);
      const busy=checking.has(provider);
      card.setAttribute('aria-busy',String(busy));
      const head=el('div',undefined,'am-agent-head');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(provider),el('span',names[provider]));
      const sw=el('button',undefined,'am-switch');sw.type='button';sw.setAttribute('role','switch');sw.setAttribute('aria-checked',String(on));
      sw.setAttribute('aria-label',names[provider]+tr(': Messenger даёт задачи'));
      sw.title=on?tr('Messenger даёт задачи ')+names[provider]+tr(' на этом компьютере'):cli?tr('Включить: Messenger будет давать задачи ')+names[provider]:tr('Сначала установите ')+names[provider]+' CLI';
      sw.disabled=!a&&!cli;
      sw.addEventListener('click',async()=>{sw.disabled=true;try{await setAgentOn(provider,!on);}finally{sw.disabled=false;}});
      const recheck=iconButton(busy?tr('Проверяем'):cli?tr('Проверить снова'):tr('Найти'),checkPath,false,()=>{if(cli)checkCard(provider);else installAgent(provider,'find');});
      recheck.disabled=busy;
      head.append(heading,recheck,iconButton(tr('Настройки'),gearPath,true,()=>openAgentSetup(provider)),sw);
      card.append(head);
      const steps=el('div',undefined,'am-steps');
      // mark: 'ok', 'no', a step number, or a node (the limit ring).
      const step=(mark,title,sub,action,tip)=>steps.append(stepRow(mark,title,sub,action,tip));
      const check=agentChecks.get(provider),auth=check?.authentication?.state;
      // 1. The program.
      const programNote=rowNote.get(provider+':program');
      if(programNote)step(1,names[provider]+' CLI',programNote,setupRecoverable.has(provider)?[tr('Найти'),()=>installAgent(provider,'find')]:undefined);
      else if(missing)step('no',names[provider]+tr(' CLI не установлен'),tr('Не найден на этом компьютере'),[tr('Установить'),()=>installAgent(provider)]);
      else if(cli){
        // T-139: a CLI newer than the one this release was checked with.
        const newer=check?.tested==='newer';
        let sub=check?.version?.value||(busy?tr('Проверяем'):'');
        if(newer){
          const box=document.createDocumentFragment(),mark=el('span',', '+tr('не проверена'),'am-status-wait');
          mark.title=tr('Messenger проверен с версией ')+check.tested_version+tr('. Новая версия обычно работает, но если задачи этого агента начнут падать, причина может быть в ней.');
          box.append(el('span',sub),mark);sub=box;
        }
        step('ok',names[provider]+tr(' CLI установлен'),sub,[tr('Удалить'),()=>uninstallAgent(provider)]);
      }
      else step(1,names[provider]+' CLI',tr('Проверяем'));
      // 2. The sign-in. No confirmed login offers «Войти» (T-056).
      const authNote=rowNote.get(provider+':auth');
      if(!cli)step(2,tr('Вход в аккаунт'),tr('После установки'));
      else if(authNote)step(2,tr('Вход в аккаунт'),authNote);
      else if(busy&&!check)step(2,tr('Вход в аккаунт'),tr('Проверяем'));
      else if(auth==='authenticated'){
        const method=check.authentication.value==='chatgpt'?tr('Аккаунт ChatGPT'):check.authentication.value==='api_key'?tr('Ключ API'):'';
        step('ok',tr('Вход выполнен'),method,[tr('Выйти'),()=>logoutFromCard(provider,cli,profile)]);
      }
      else if(auth==='not_authenticated')step('no',tr('Вход не выполнен'),tr('Войдите в аккаунт агента'),[tr('Войти'),()=>loginFromCard(provider,cli,profile)]);
      else step(2,tr('Вход в аккаунт'),check?tr('Не удалось проверить'):tr('Ещё не проверен'),[tr('Войти'),()=>loginFromCard(provider,cli,profile)]);
      // 3. Work with Messenger: set only by what a task showed (D-070).
      const trust=a?.trust?.state;
      if(a&&view?.folder&&trust&&trust!=='trusted')step('no',tr('Не доверяет папке проекта'),tr('Подтвердите доверие в терминале'),[tr('Доверять'),()=>trustFolder(provider)],a.trust?.hint);
      else if(a?.last_done)step('ok',tr('Работает с Messenger'),tr('Последняя задача ')+dayClock(a.last_done));
      else step(3,tr('Работает с Messenger'),missing?tr('Проверится после установки'):tr('Проверится на первой задаче'));
      // 4. The limit the agent reported during its last task (T-044).
      const l=a?.limit,level=limitLevel(l);
      const today=a?.today_tokens?tr('сегодня ')+tokensText(a.today_tokens)+tr(' токенов'):'';
      if(l){
        const title=el('span');title.append(tr('Лимит '));
        const used=typeof l.used_percent==='number'?Math.round(l.used_percent)+'%':l.state==='exhausted'?tr('исчерпан'):l.state==='warning'?tr('близко к пределу'):tr('в норме');
        title.append(el('span',used,'am-status-'+level));
        const r=l.resets_at?tr('сброс ')+resetText(l.resets_at):'';
        const sub=[limitWindow(l.window),r,today].filter(Boolean).join(', ');
        step(limitRing(l),title,sub?sub[0].toUpperCase()+sub.slice(1):(l.observed_at?tr('По данным задачи в ')+clock(l.observed_at):''));
      }else step(limitRing(null),tr('Лимит'),today?today[0].toUpperCase()+today.slice(1):tr('Появится после задачи исполнителя на этом компьютере'));
      card.append(steps);
      if(busy)steps.querySelectorAll('button').forEach(button=>button.disabled=true);
      const running=a?.project_running||0,todayTasks=a?.project_today||0;
      const tasks=el('button',undefined,'am-agent-tasks');tasks.type='button';
      if(running||todayTasks){
        const label=el('span');label.append(tr('Задачи: '));
        if(running)label.append(el('span',running+tr(' идёт'),'am-status-run'));
        if(running&&todayTasks)label.append(', ');
        if(todayTasks)label.append(todayTasks+tr(' сегодня'));
        tasks.append(label);
        tasks.addEventListener('click',()=>{$('agent-filter').value=provider;syncSelects();show('tasks');renderTaskList();});
      }else{tasks.textContent=tr('Задачи');tasks.disabled=true;}
      card.append(tasks);cards.append(card);
    }
    // A check on one provider must not replace the other providers' cards.
    for(const [index,card] of [...cards.children].entries()){
      const previous=live.children[index];
      if(previous?.outerHTML===card.outerHTML)continue;
      if(previous)previous.replaceWith(card);else live.append(card);
    }
    while(live.children.length>3)live.lastElementChild.remove();
    renderAccessBar();
  }
  // Another computer's agents, as its reports show them: view only (T-076).
  function renderPeerAgents(cards){
    $('access-bar').hidden=true;
    const peer=(server?.peers||[]).find(p=>p.id===computerFilter);
    const fresh=!!peer&&peerFresh(peer);
    const busyTasks=busyTasksByComputer();
    cards.append(el('p',!peer?tr('Компьютер не найден в сети.'):fresh?tr('Только просмотр: агентов этого компьютера настраивают на нём самом.'):(peer.last_seen?tr('Нет связи. Был на связи ')+stamp(peer.last_seen)+'.':tr('Ещё не выходил на связь.')),'am-small am-muted am-peer-agents-note'));
    for(const a of peer?.agents||[]){
      const card=el('article',undefined,'am-card am-agent-card am-agent-remote'+(fresh?'':' am-agent-off'));
      const head=el('div',undefined,'am-agent-head');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(a.agent),el('span',names[a.agent]||a.agent));
      const {counts}=agentActivity(fresh?(busyTasks.get(peer.id+'\0'+a.agent)||[]):[]);
      const [state,text]=fresh?agentState(counts):['offline',tr('Нет связи')];
      const st=el('span',undefined,'am-peer-state am-small am-muted');st.append(el('span',undefined,'am-dot am-dot-'+state),el('span',text));
      head.append(heading,st);card.append(head);
      const steps=el('div',undefined,'am-steps');
      const row=(mark,title,sub)=>{
        const r=el('div',undefined,'am-step');const m=el('span',undefined,'am-mark am-mark-ring');m.append(mark);r.append(m);
        const t=el('div',undefined,'am-step-text');const line=el('p',undefined,'am-step-title');
        if(title instanceof Node)line.append(title);else line.textContent=title;t.append(line);
        if(sub){const s=el('p',sub,'am-step-sub');s.title=sub;t.append(s);}
        r.append(t,el('span'));steps.append(r);
      };
      const l=a.limit,level=limitLevel(l);
      if(l){
        const title=el('span');title.append(tr('Лимит '));
        title.append(el('span',typeof l.used_percent==='number'?Math.round(l.used_percent)+'%':l.state==='exhausted'?tr('исчерпан'):l.state==='warning'?tr('близко к пределу'):tr('в норме'),'am-status-'+level));
        const sub=[limitWindow(l.window),l.resets_at?tr('сброс ')+resetText(l.resets_at):''].filter(Boolean).join(', ');
        row(limitRing(l),title,sub?sub[0].toUpperCase()+sub.slice(1):l.observed_at?tr('По данным задачи в ')+dayClock(l.observed_at):'');
      }else row(limitRing(null),tr('Лимит'),tr('Появится после задачи исполнителя на том компьютере'));
      const models=el('span',undefined,'am-agent-models');models.textContent=a.models?.length?a.models.join(', '):tr('Модель по умолчанию');
      const mm=el('span','✓','am-mark am-mark-ok');
      const r=el('div',undefined,'am-step');const t=el('div',undefined,'am-step-text');t.append(el('p',tr('Модели'),'am-step-title'),el('p',models.textContent,'am-step-sub'));r.append(mm,t,el('span'));steps.append(r);
      card.append(steps);
      const n=counts.running+counts.waiting_owner;
      const tasks=el('button',undefined,'am-agent-tasks');tasks.type='button';
      // T-083: what runs there, as on this computer's cards.
      if(n){const label=el('span');label.append(tr('Задачи: '));
        if(counts.running)label.append(el('span',counts.running+tr(' идёт'),'am-status-run'));
        if(counts.running&&counts.waiting_owner)label.append(', ');
        if(counts.waiting_owner)label.append(el('span',counts.waiting_owner+tr(' ждёт ответа'),'am-status-wait'));
        tasks.append(label);tasks.addEventListener('click',()=>{$('agent-filter').value=a.agent;syncSelects();show('tasks');renderTaskList();});}
      else{tasks.textContent=tr('Задач сейчас нет');tasks.disabled=true;}
      card.append(tasks);cards.append(card);
    }
    if(fresh&&!(peer.agents||[]).length)cards.append(el('p',tr('Агенты не подключены'),'am-small am-muted'));
  }
  // T-056: one «Любые команды» switch for every agent of this computer.
  function renderAccessBar(){
    const agents=view?.agents||[],bar=$('access-bar');bar.hidden=!agents.length;
    const on=agents.filter(a=>a.full_access).length,all=!!agents.length&&on===agents.length;
    $('access-toggle').setAttribute('aria-checked',all?'true':on?'mixed':'false');
    $('access-title').textContent=on&&!all?tr('Любые команды: разрешены не всем'):tr('Любые команды');
    // D-109: other computers' tasks follow this computer's agents unless the
    // owner turns it off; on until the window has heard otherwise.
    const net=!server||server.net_full_access!==false;
    $('net-full-toggle').setAttribute('aria-checked',String(net));
    $('net-full-toggle').disabled=!server;
  }
  async function toggleNetAccess(){
    if(!server)return;
    const on=!server.net_full_access,toggle=$('net-full-toggle');
    toggle.disabled=true;toggle.setAttribute('aria-checked',String(on));
    try{const answer=await postAPI('/api/server-action',{action:'net_full_access',on},28000);if(answer?.result)server=answer.result;}
    catch(error){showError(error);}
    finally{renderAccessBar();}
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
  function askConfirm(title,text,go,action,safe){
    confirmAction=action;$('confirm-go').className=safe?'am-primary':'am-danger';$('confirm-title').textContent=title;$('confirm-text').textContent=text;$('confirm-go').textContent=go;
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
    try{
      const info=await api('/api/client-info');clientInfo=info;if(!info.outdated)restart.ask=null;
      // T-077: after an update the window restarts the old client itself,
      // once. The client refuses while tasks run or their count is unknown;
      // only then the owner is asked.
      if(info.outdated&&info.restart&&info.client==='running'&&!autoRestarted&&!restart.busy&&!restart.ask){autoRestarted=true;restartClient(false);return;}
      renderRestart();
    }catch(_){}
  }
  // T-123: «Дождаться и перезапустить» keeps asking the client every few
  // seconds; it restarts by itself once no task runs (the client checks
  // again), and nothing is cut. Closing the window stops the waiting only.
  setInterval(()=>{if(restart.wait&&!restart.busy&&clientInfo?.outdated)restartClient(false);},5000);
  async function restartClient(confirm){
    const wait=restart.wait&&!confirm;
    restart={busy:true,ask:restart.ask,error:'',wait};renderRestart();
    let response,result={};
    try{response=await fetch('/api/restart-client',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({project,confirm}),cache:'no-store',signal:AbortSignal.timeout(50000)});result=await response.json().catch(()=>({}));}
    catch(_){restart={busy:false,ask:null,error:tr('Ответ не получен, результат неизвестен. Подождите немного: окно проверит клиент само.'),wait:false};renderRestart();loadClientInfo();return;}
    if(response.ok&&result.ok){restart={busy:false,ask:null,error:'',wait:false};showError(null);await loadClientInfo();viewSignature='';refresh();return;}
    if(result.state==='confirm'){restart={busy:false,ask:{running:result.running||0,known:result.known!==false,tasks:result.tasks||[]},error:'',wait};renderRestart();return;}
    restart={busy:false,ask:null,error:result.detail||tr('Перезапуск не подтверждён. Проверьте состояние.'),wait:false};renderRestart();loadClientInfo();
  }
  // A window newer than its client cannot work properly, so this dialog
  // covers the whole window until the client is restarted or the app quits.
  // Quitting closes only the window; the background client keeps running.
  function renderRestart(){
    renderBoot();
    const dialog=$('restart-dialog');
    // While the update installs, the start screen shows it instead (T-079).
    const show=(!!clientInfo?.outdated&&!restart.busy)||!!restart.ask||!!restart.error;
    if(!show){if(dialog.open)dialog.close();return;}
    const ask=restart.ask,canRestart=!!clientInfo?.restart,answering=clientInfo?.client==='running';
    const canQuit=!!window.webkit?.messageHandlers?.app;
    let title=tr('Нужно перезапустить Messenger'),text;
    const version=clientInfo?.window_version?tr('Версия ')+versionName(clientInfo.window_version).slice(1)+tr(' установлена. '):'';
    if(ask&&restart.wait){title=tr('Ждём окончания задач');text=(ask.known?tr('Задач в работе: ')+ask.running+'. ':'')+tr('Messenger перезапустится сам, когда они закончатся. Если закрыть окно, задачи продолжат работать, но перезапуска не будет до следующего открытия.');}
    else if(ask&&ask.known){title=tr('Обновление ждёт задачи: ')+ask.running;text=version+tr('Чтобы она заработала, Messenger перезапустится и прервёт задачи, которые идут сейчас. Сами они не повторятся.');}
    else if(ask){title=tr('Не удалось проверить задачи');text=version+tr('Если задачи сейчас идут, перезапуск прервёт их. Сами они не повторятся.');}
    else if(restart.busy){title=tr('Обновляем Messenger');text=tr('Останавливаем старую версию и запускаем новую. Это займёт несколько секунд.');}
    else if(canRestart)text=tr('Приложение обновилось до версии ')+clientInfo.window_version+tr(', а в фоне работает ')+(clientInfo.client_version?tr('версия ')+clientInfo.client_version:tr('более ранняя версия'))+tr('. Пока она не перезапущена, Messenger не может работать. Задачи, история и устройства сохранятся.');
    else text=tr('В фоне работает более ранняя версия клиента. Закройте окно, выполните скопированную команду в Терминале и откройте окно снова.');
    $('restart-title').textContent=title;$('restart-text').textContent=text;
    const list=$('restart-tasks');list.replaceChildren();
    for(const t of ask?.tasks||[]){const row=el('div',undefined,'am-restart-task');row.append(el('span',(names[t.agent]||t.agent)+': '+t.title),el('span',minutes(t.started_at),'am-status-run'));list.append(row);}
    const others=ask?.known?ask.running-(ask.tasks||[]).length:0;
    if(others>0)list.append(el('p',tr('И ещё в других проектах: ')+others,'am-small am-muted'));
    list.hidden=!list.childElementCount;
    dialog.classList.toggle('am-restart-ask',!!ask);
    $('restart-error').textContent=restart.error;$('restart-error').hidden=!restart.error;
    const go=$('restart-go'),quit=$('restart-quit'),copy=$('restart-copy');
    // T-123: with tasks running the main button waits for them; cutting
    // them is the separate red button.
    const cut=$('restart-cut');
    go.hidden=!canRestart||!answering||(restart.busy&&!restart.wait)||(!!ask&&restart.wait);
    go.textContent=ask?tr('Дождаться и перезапустить'):tr('Перезапустить');go.disabled=restart.busy;
    cut.hidden=!ask||!canRestart||!answering;cut.disabled=restart.busy&&!restart.wait;
    cut.textContent=restart.wait?tr('Прервать сейчас'):tr('Прервать и перезапустить');
    quit.hidden=!canQuit;quit.disabled=restart.busy;
    copy.hidden=canRestart||!shutdownCommand();
    if(!dialog.open){dialog.showModal();(go.hidden?quit.hidden?copy:quit:go).focus();}
  }
  function renderNotice(){
    if(!online){notice(el('p',offlineReason||tr('Клиент Messenger не запущен или не отвечает. Данные не обновляются.')));return;}
    if(view?.pending_launch>0&&!view.tasks_enabled){notice(el('p',tr('Задачи записаны, но этот клиент не запускает исполнителей: он запущен без разрешения --allow-tasks.')));return;}
    if(view?.pending_launch>0&&view.paused){
      const box=el('div',undefined,'am-notice-row');box.append(el('p',tr('Обработка на паузе: новые задачи не запускаются.')));
      const go=el('button',tr('Продолжить'));go.type='button';go.addEventListener('click',async()=>{go.disabled=true;try{await postAPI('/api/resume',{});showError(null);await refresh();}catch(e){go.disabled=false;showError(e);}});
      box.append(go);notice(box);return;
    }
    notice(null);
  }
  function render(){
    renderProjects();renderMachines();renderTaskList();renderThread();renderAgents();renderProjectPage();renderNotice();autoCheck();
    $('connection').textContent=online?tr('Клиент доступен'):tr('Клиент недоступен');
  }
  let feedRequest=0;
  async function loadFeed(full=false){
    const request=++feedRequest;
    if(!selectedTask){feed=null;renderThread();return;}
    const id=selectedTask,name=project;
    const previous=!full&&feed?.task.id===id?feed:null;
    const q=new URLSearchParams({id});
    if(view?.change_watch)q.set('cursor',previous?.cursor||'');
    startWindowWatch();
    try{
      const f=await api('/api/task?'+q);
      if(request!==feedRequest||id!==selectedTask||name!==project||f.task.id!==id)return;
      if(f.append_from){
        if(!previous||previous.items.length!==f.append_from)return !full&&await loadFeed(true);
        f.items=[...previous.items,...f.items];
      }
      feed=f;renderThread();return true;
    }
    catch(_){}
    return false;
  }
  let refreshQueued=false,lastWindowRefresh=0;
  let windowWatch=null,watchRetryAt=0;
  function windowWatchScope(){return new URLSearchParams({project,task:tab==='tasks'?selectedTask:''}).toString();}
  function startWindowWatch(){
    if(!view?.change_watch||Date.now()<watchRetryAt)return;
    const scope=windowWatchScope();
    if(windowWatch?.scope===scope)return;
    windowWatch?.controller.abort();
    const watch={scope,controller:new AbortController()};
    windowWatch=watch;
    watchWindow(watch);
  }
  async function watchWindow(watch){
    let current=null;
    try{
      while(windowWatch===watch&&windowWatchScope()===watch.scope){
        const q=new URLSearchParams(watch.scope);
        if(current){q.set('window',current.window);q.set('feed',current.feed);}
        const requestTimer=setTimeout(()=>watch.controller.abort(),16000);
        let next;
        try{next=await api('/api/window-changes?'+q,16000,watch.controller.signal);}
        finally{clearTimeout(requestTimer);}
        if(windowWatch!==watch||windowWatchScope()!==watch.scope)break;
        if(typeof next.window!=='string'||typeof next.feed!=='string')throw Error('invalid window revision');
        if(!current||next.window!==current.window){
          if(await refresh(false)===false)throw Error('window unavailable');
        }
        if(windowWatch!==watch||windowWatchScope()!==watch.scope)break;
        if(q.get('task')&&(!current||next.feed!==current.feed)){
          const loaded=await loadFeed();
          if(loaded===false&&windowWatch===watch&&windowWatchScope()===watch.scope)throw Error('feed unavailable');
        }
        current=next;
      }
    }catch(_){
      if(windowWatch===watch)watchRetryAt=Date.now()+10000;
    }finally{
      if(windowWatch===watch){windowWatch=null;startWindowWatch();}
    }
  }
  async function refresh(pollFeed=!windowWatch){
    if(refreshing){refreshQueued=true;return;}refreshing=true;
    const requestScope=taskWindowQuery();
    try{
      let v;
      const q=new URLSearchParams(requestScope);
      if(windowHistoryScope===requestScope&&view?.history_version)q.set('history_version',view.history_version);
      try{v=await api('/api/window?'+q);}
      catch(error){if(requestScope!==taskWindowQuery())return;if(Date.now()-clientInfoAt>5000)loadClientInfo();online=false;offlineReason=error.name==='TimeoutError'||error.name==='TypeError'?tr('Окно потеряло связь с программой Messenger. Закройте окно и откройте снова.'):error.message;renderProjects();renderMachines();renderNotice();return false;}
      if(requestScope!==taskWindowQuery())return;
      const cameOnline=!online;
      online=true;lastWindowRefresh=Date.now();offlineReason='';if(Date.now()-clientInfoAt>10000)loadClientInfo();
      // The first answer: learn the client's version before the start screen
      // goes, so an update starts without the old window showing.
      if(booting){await loadClientInfo();booting=false;renderBoot();}
      if(requestScope!==taskWindowQuery())return;
      if(!v.projects.some(p=>p.name===project))setProject(v.project||'');
      if(v.history_unchanged){
        const live=new Set(v.tasks.map(t=>t.id));
        v.tasks.push(...(view?.tasks||[]).filter(t=>finishedStates.includes(t.state)&&!live.has(t.id)));
        if(view?.history_next)v.history_next=view.history_next;
        delete v.history_unchanged;
      }
      windowHistoryScope=taskWindowQuery();
      const signature=JSON.stringify(v);
      if(signature!==viewSignature){
        const history=currentTaskHistory();
        if(history&&view?.history_next!==v.history_next)taskHistory=null;
        viewSignature=signature;view=v;noteWelcomeDone();
        if(!v.history_paged&&selectedTask&&!v.tasks.some(t=>t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}
        if(selectedShared&&!sharedSelection())selectedShared=null;
        // Choose the top task before drawing, so a project switch goes from
        // loading straight to the conversation without an empty pane between.
        if(!selectedTask&&!selectedShared&&tab==='tasks'&&v.project)openTopTask(true);
        render();
      }
      else if(cameOnline){renderMachines();renderNotice();}
      // Nothing chosen yet (the first start, or the chosen task went away):
      // show the top task instead of an empty conversation.
      if(!selectedTask&&!selectedShared&&tab==='tasks'&&view?.project)openTopTask(true);
      if(tab==='tasks'&&selectedTask){
        const row=view?.tasks?.find(t=>t.id===selectedTask),current=feed?.task;
        if(current?.id!==selectedTask||pollFeed&&!finishedStates.includes(current.state)||row&&(row.state!==current.state||row.updated_at!==current.updated_at))await loadFeed();
      }
      startWindowWatch();
    }catch(error){notice(el('p',tr('Окно не смогло показать данные: ')+error.message));}
    finally{refreshing=false;if(refreshQueued||requestScope!==taskWindowQuery()){refreshQueued=false;refresh();}}
  }
  let discovering=false;
  async function loadDiscovery(){
    if(discovering)return;discovering=true;
    const previous=new Map(['claude','codex','cursor'].map(provider=>[provider,JSON.stringify(agentPaths(provider))]));
    try{
      const d=await api('/api/discovery',8000);discovery=d.agents||[];discoveryLoaded=true;
      for(const provider of ['claude','codex','cursor']){
        const selection=agentPaths(provider);
        if(autoChecked&&selection.cli&&previous.get(provider)!==JSON.stringify(selection)){
          agentChecks.delete(provider);checkCard(provider,true);
        }
      }
      renderAgents();autoCheck();
    }catch(_){}finally{discovering=false;}
  }
  function show(next){
    tab=next;root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);renderNotices();renderToasts();
    root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('list-toggle').hidden=tab!=='tasks'||!view?.project;if(tab==='server')loadServer();if(tab==='stats'){renderStats();loadStats();}if(tab==='agents')renderAgents();$('screen-title').textContent={tasks:tr('Задачи'),agents:tr('Агенты'),project:tr('Проект'),server:tr('Сервер'),stats:tr('Статистика')}[tab];startWindowWatch();
  }


  // T-068: tokens spent by the tasks of this computer over a period. Only
  // tokens, no dollars (D-074). One hue for every bar: the bars show size,
  // not identity, and the names stand next to them.
  let statsPeriod=['day','week','month'].includes(recall('messenger-stats-period'))?recall('messenger-stats-period'):'week',stats=null,statsGen=0;
  const statsOrders=['recent','oldest','tokens','fewest','longest','shortest'];
  let statsSort=recall('messenger-stats-sort');
  if(!statsOrders.includes(statsSort))statsSort='recent';
  // T-201: one agent and one of its models; the model is not remembered.
  let statsAgent=['claude','codex','cursor'].includes(recall('messenger-stats-agent'))?recall('messenger-stats-agent'):'',statsModel='';
  const periodWords={day:tr('сегодня'),week:tr('за 7 дней'),month:tr('за 30 дней')};
  function tasksWord(n){const a=n%10,b=n%100;if(amLang==='en')return n+(n===1?' task':' tasks');return n+' '+(a===1&&b!==11?'задача':a>=2&&a<=4&&(b<12||b>14)?'задачи':'задач');}
  function tokensWord(n){const a=n%10,b=n%100;if(amLang==='en')return n===1?'token':'tokens';return a===1&&b!==11?'токен':a>=2&&a<=4&&(b<12||b>14)?'токена':'токенов';}
  function tokensFull(n){return n.toLocaleString(amLocale)+' '+tokensWord(n);}
  function statsDuration(ms){
    const seconds=Math.floor(ms/1000);
    if(seconds<1)return '<1'+tr(' с');
    if(seconds<60)return seconds+tr(' с');
    const m=Math.floor(seconds/60),h=Math.floor(m/60);
    if(!h)return m+tr(' мин');
    return h+tr(' ч')+(m%60?' '+m%60+tr(' мин'):'');
  }
  function setProject(name){
    if(project===name)return;
    project=name;
    statsModel='';statsGen++;
    if(!name)stats=null;
    if(tab==='stats')reloadStats();
  }
  async function loadStats(){
    const gen=++statsGen;
    const name=project;
    if(!name)return;
    const params=new URLSearchParams({project:name,period:statsPeriod});
    const computer=computerFilter;
    if(computer)params.set('computer',computer);
    if(statsAgent)params.set('agent',statsAgent);
    if(statsModel)params.set('model',statsModel);
    try{
      const s=await api('/api/usage-stats?'+params,8000);
      if(gen!==statsGen)return;
      if(s.project!==name){stats=null;statsPending=false;renderStats();throw Error(tr('Фильтр не применён: обновите фоновый клиент.'));}
      s.forComputer=computer;stats=s;statsPending=false;renderStats();
    }
    catch(error){if(gen===statsGen){statsPending=false;setStatsBusy();$('stats-total-title').textContent=tr('Статистика недоступна');$('stats-total-note').textContent=error.message;}}
  }
  // A new period, filter, computer or project keeps the shown numbers until
  // the new ones arrive, so the blocks do not collapse and grow again.
  let statsPending=false;
  function reloadStats(){statsPending=!!stats;renderStats();loadStats();}
  function setStatsBusy(){
    const screen=root.querySelector('[data-screen="stats"]');
    if(statsPending)screen.setAttribute('aria-busy','true');else screen.removeAttribute('aria-busy');
  }
  function statsBars(box,groups,total,agents){
    box.replaceChildren();
    if(!groups.length){box.append(el('p',tr('Пока пусто'),'am-small am-muted'));return;}
    for(const g of groups){
      const row=el('div',undefined,'am-stats-row');
      const name=el('span',agents?(names[g.key]||g.key):(g.name||tr('Без имени')),'am-stats-name');
      if(g.this)name.append(el('span',tr(' (этот)'),'am-muted'));
      const bar=el('span',undefined,'am-stats-bar');const fill=el('span');fill.style.width=(total?Math.max(2,100*g.tokens/total):0)+'%';bar.append(fill);
      row.append(name,el('span',tokensText(g.tokens),'am-stats-value'),bar,el('span',tasksWord(g.tasks),'am-small am-muted am-stats-tasks'));
      row.title=usageLong(g);box.append(row);
    }
  }
  function statsDay(day){const d=new Date(day+'T12:00:00');return isNaN(d.getTime())?day:d.toLocaleDateString(amLocale,{day:'numeric',month:'short'});}
  function statsDays(box,days){
    box.replaceChildren();box.hidden=days.length<2;if(box.hidden)return;
    const max=Math.max(1,...days.map(d=>d.tokens));
    const plot=el('div',undefined,'am-stats-plot');plot.setAttribute('role','img');
    plot.setAttribute('aria-label',tr('Токены по дням, наибольшее за день ')+tokensText(max));
    const tip=el('div','','am-stats-tip');tip.hidden=true;
    for(const d of days){
      const col=el('button',undefined,'am-stats-col');col.type='button';
      const text=statsDay(d.day)+': '+(d.tokens?tokensFull(d.tokens)+', '+tasksWord(d.tasks):tr('нет задач'));
      col.setAttribute('aria-label',text);
      const bar=el('span');bar.style.height=(d.tokens?Math.max(2,100*d.tokens/max):0)+'%';col.append(bar);
      const showTip=()=>{tip.textContent=text;tip.hidden=false;const c=col.offsetLeft+col.offsetWidth/2,w=tip.offsetWidth;tip.style.left=Math.min(Math.max(0,c-w/2),plot.clientWidth-w)+'px';};
      col.addEventListener('mouseenter',showTip);col.addEventListener('focus',showTip);
      col.addEventListener('mouseleave',()=>tip.hidden=true);col.addEventListener('blur',()=>tip.hidden=true);
      plot.append(col);
    }
    plot.append(tip);
    const axis=el('div',undefined,'am-stats-axis am-small am-muted');
    axis.append(el('span',statsDay(days[0].day)),el('span',tr('до ')+tokensText(max)+tr(' в день')),el('span',statsDay(days[days.length-1].day)));
    box.append(plot,axis);
  }
  function renderStats(){
    root.querySelectorAll('[data-period]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.period===statsPeriod)));
    // The card follows the numbers on screen, not the filter still loading.
    $('stats-computers-card').hidden=stats?!!stats.forComputer:!!computerFilter;
    setStatsBusy();
    $('stats-foot').textContent=project?tr('Учитываются задачи выбранного проекта, известные этому компьютеру. Задача попадает в день, когда закончилась.'):'';
    $('stats-agent').value=statsAgent;
    root.querySelectorAll('[data-stats-sort]').forEach(b=>{
      const down=b.dataset.statsSort===statsSort,up=b.dataset.statsReverse===statsSort;
      if(down||up)b.closest('th').setAttribute('aria-sort',down?'descending':'ascending');
      else b.closest('th').removeAttribute('aria-sort');
      b.querySelector('span').textContent=down?'↓':up?'↑':'';
    });
    // Rebuilt only when the list changed: a rebuild closes an open menu (T-192).
    const models=[['',tr('Все модели')],...(stats?.models||[]).map(m=>[m.key,modelName(m.key)])];
    if(statsModel&&!models.some(([v])=>v===statsModel))models.push([statsModel,modelName(statsModel)]);
    const select=$('stats-model');
    if(select.options.length!==models.length||models.some(([v,l],i)=>select.options[i].value!==v||select.options[i].textContent!==l))select.replaceChildren(...models.map(([v,l])=>{const o=el('option',l);o.value=v;return o;}));
    select.value=statsModel;
    syncSelects();
    if(!stats){
      $('stats-total-title').textContent=project?tr('Загрузка статистики…'):tr('Выберите проект, чтобы увидеть статистику.');
      $('stats-total-note').textContent='';
      statsDays($('stats-days'),[]);
      statsBars($('stats-agents'),[],0,true);
      statsBars($('stats-computers'),[],0);
      $('stats-top').replaceChildren();
      return;
    }
    // An older background client counts everything; say so.
    const unfiltered=!statsPending&&((stats.agent||'')!==statsAgent||(stats.model||'')!==statsModel);
    if(unfiltered)$('stats-foot').textContent=tr('Фильтр не применён: обновите фоновый клиент.');
    const t=stats.total;
    $('stats-total-title').textContent=t.tasks?tokensFull(t.tokens)+' '+periodWords[statsPeriod]:tr('Нет задач с расходом ')+periodWords[statsPeriod];
    $('stats-total-note').textContent=t.tasks?tasksWord(t.tasks)+tr(', на входе ')+t.input_tokens.toLocaleString(amLocale)+(t.cached_tokens?tr(' (из кэша ')+t.cached_tokens.toLocaleString(amLocale)+')':'')+tr(', на выходе ')+t.output_tokens.toLocaleString(amLocale):tr('Агент сообщает расход, когда задача заканчивается.');
    statsDays($('stats-days'),statsPeriod==='day'?[]:stats.days);
    statsBars($('stats-agents'),stats.agents,t.tokens,true);
    statsBars($('stats-computers'),stats.computers,t.tokens);
    const list=statsSort==='recent'?(stats.recent||[]):statsSort==='tokens'?(stats.top||[]):stats.ranked?.[statsSort];
    const top=$('stats-top');top.replaceChildren();
    if(!list?.length){
      const empty=el('td',list?tr('Пока пусто'):tr('Обновите фоновый клиент, чтобы использовать эту сортировку.'),'am-small am-muted');
      empty.colSpan=4;
      const row=el('tr');row.append(empty);top.append(row);return;
    }
    for(const task of list){
      const row=el('tr',undefined,'am-stats-task');
      const words=el('button',undefined,'am-stats-task-text');words.type='button';
      const date=new Date(task.at);
      const when=el('td',undefined,'am-small am-muted');
      if(!Number.isNaN(date.getTime())){
        const stamp=el('time',date.toLocaleDateString(amLocale,{day:'numeric',month:'short'})+' '+clock(task.at));
        stamp.dateTime=task.at;when.append(stamp);
      }
      const detail=[task.project,shortNames[task.agent]||task.agent,task.model&&modelName(task.model),task.computer&&tr('на ')+task.computer].filter(Boolean).join(' ');
      words.append(el('span',task.title||tr('Без названия'),'am-stats-task-title'),el('span',detail,'am-small am-muted'));
      const titleCell=el('td');titleCell.append(words);
      const duration=el('td','—','am-stats-value am-muted');
      duration.title=tr('Длительность неизвестна');
      if(Number.isFinite(task.duration_ms)){
        duration.textContent=(task.duration_estimated?'≈ ':'')+statsDuration(task.duration_ms);
        duration.title=task.duration_estimated?tr('Примерно, по первому полученному действию и окончанию задачи'):tr('От первого запуска до завершения');
      }
      const tokens=el('td',task.usage_missing?'—':tokensText(task.tokens),'am-stats-value');
      row.append(when,titleCell,duration,tokens);
      words.title=(!task.usage_missing&&typeof task.input_tokens==='number'?usageLong(task)+'\n':'')+tr('Открыть задачу');
      row.addEventListener('click',()=>{
        selectedTask=task.id;selectedShared=null;feed=null;feedSignature='';show('tasks');refresh();
      });
      top.append(row);
    }
  }
  $('stats-agent').addEventListener('change',()=>{statsAgent=$('stats-agent').value;remember('messenger-stats-agent',statsAgent);statsModel='';reloadStats();});
  $('stats-model').addEventListener('change',()=>{statsModel=$('stats-model').value;reloadStats();});
  root.querySelectorAll('[data-stats-sort]').forEach(b=>b.addEventListener('click',()=>{
    statsSort=statsSort===b.dataset.statsSort?b.dataset.statsReverse:b.dataset.statsSort;
    remember('messenger-stats-sort',statsSort);renderStats();
  }));
  root.querySelectorAll('[data-period]').forEach(b=>b.addEventListener('click',()=>{statsPeriod=b.dataset.period;remember('messenger-stats-period',statsPeriod);reloadStats();}));

  // Server tab: the broker hosted by this client and this computer's
  // connection to a broker. Messages stay next to the section they belong to.
  let server=null,serverBusy=false,serverGen=0,serverPolling=false,serverEdited=false,nameFilled=false,invitation=null,inviteTimer=0,revokeAsk='',revokeTimer=0;
  const notes={host:null,member:null,accept:null},noteTimers={};
  function stamp(at){const d=new Date(at);return isNaN(d.getTime())?'':d.toLocaleString(amLocale,{day:'numeric',month:'short',hour:'2-digit',minute:'2-digit'});}
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
    $('host-scope').disabled=running;$('host-port').disabled=running;$('host-public').disabled=running;
    $('host-public-box').hidden=$('host-scope').value!=='internet';
    const accept=!!server.accept;$('accept-toggle').setAttribute('aria-checked',String(accept));
    $('log-share-toggle').setAttribute('aria-checked',String(!!server.log_share));
  }
  // Automatic updates are set here only, by the owner, on each computer
  // (T-277): no coordinator or other computer can turn them on.
  let autoUpdateBusy=false;
  function renderAutoUpdate(){
    $('auto-update-row').hidden=!server?.auto_update_supported;
    $('auto-update-toggle').setAttribute('aria-checked',String(!!server?.auto_update));
    $('auto-update-toggle').disabled=autoUpdateBusy;
  }
  async function toggleAutoUpdate(){
    if(!server||autoUpdateBusy)return;
    const on=!server.auto_update;autoUpdateBusy=true;serverGen++;$('auto-update-status').textContent='';renderAutoUpdate();
    try{const answer=await postAPI('/api/server-action',{action:'auto_update',on},28000);server=answer.result;renderServer();}
    catch(error){$('auto-update-status').textContent=capital(error.message);}
    finally{autoUpdateBusy=false;serverGen++;renderAutoUpdate();}
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
    renderAccessBar();renderAutoUpdate();
    if(!server)return;const h=server.host,m=server.member;
    const focused=document.activeElement?.dataset?.key||'';
    badge($('host-badge'),h.running?tr('Работает'):h.enabled?tr('Не запустился'):tr('Остановлен'),h.running?'good':h.enabled?'bad':'');
    // Explanations live in the status tooltip so the card does not change
    // height; only a problem the owner must fix stays visible.
    $('host-badge').title=!h.running?tr('Другие устройства смогут подключаться к этому компьютеру по одноразовому приглашению.'):tr('Принимает устройства: ')+(h.scope==='internet'?tr('из интернета и локальной сети'):h.scope==='lan'?tr('в локальной сети'):tr('только с этого компьютера'))+tr(', порт ')+h.port+'.';
    $('host-state').textContent=h.enabled&&m.connected?tr('Запущен свой сервер и есть подключение к другому. Задачи идут только через свой сервер. Остановите сервер или отключитесь от другого.'):h.running&&!h.addresses.length?tr('У компьютера нет адреса в локальной сети. Подключитесь к сети или выберите «Только этот компьютер».'):'';
    if(!serverEdited){$('host-scope').value=h.scope||'local';$('host-port').value=h.port||47100;$('host-public').value=h.public_address||'';$('host-public-box').hidden=$('host-scope').value!=='internet';}
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
    badge($('member-badge'),m.connected?(m.reachable||!m.checked_at?tr('Подключено'):tr('Нет связи')):tr('Не подключено'),m.connected?(m.reachable?'good':m.checked_at?'bad':''):'');
    // A computer hosts its network or joins another one, not both: the
    // other card only says how to switch.
    const hostOn=h.enabled||h.running;
    $('member-join').hidden=m.connected||hostOn;$('member-joined').hidden=!m.connected;
    $('member-blocked').hidden=!hostOn||m.connected;
    $('host-blocked').hidden=hostOn||!m.connected;
    $('host-running').closest('.am-card').querySelector('.am-server-grid').hidden=!hostOn&&m.connected;
    $('member-badge').title=m.connected?tr('Этот компьютер входит в сервер ')+m.address+'.':tr('Вставьте код приглашения, который выдал сервер на другом компьютере.');
    $('member-state').textContent='';
    const cn=$('computer-name');if(view?.computer_name&&document.activeElement!==cn&&!cn.dataset.edited)cn.value=view.computer_name;
    const dl=$('member-details');dl.replaceChildren();
    const rows=[[tr('Сервер'),m.address],[tr('Подключено'),stamp(m.connected_at)]];
    if(m.connected)rows.forEach(([k,v])=>{if(v){dl.append(el('dt',k),el('dd',v));}});
    const buttons=root.querySelectorAll('[data-screen=server] button');buttons.forEach(b=>b.disabled=serverBusy);
    if(!serverBusy)$('host-invite').disabled=!h.addresses.length;
    renderHostToggle();
    syncSelects();renderNotes();
    if(focused&&document.activeElement===document.body)root.querySelector('[data-key="'+CSS.escape(focused)+'"]')?.focus();
  }
  async function serverAction(body,section,done){
    if(serverBusy)return null;
    serverBusy=true;serverGen++;
    let focused=document.activeElement;
    // Starting and stopping show their result in the status dot only, so the
    // card does not grow and shrink around a passing message.
    note(section,body.action==='connect'||body.action==='check'?tr('Связываемся с сервером…'):'');
    renderServer();
    let result=null;
    try{
      const answer=await postAPI('/api/server-action',body,28000);
      result=answer.result;
      if(body.action!=='invite')server=result;
      if(body.action==='check'){const m=server.member;note(section,m.reachable?tr('Связь есть, устройство на сервере.'):(m.error||tr('Связи нет.')),!m.reachable,m.reachable);}
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
    if(!Number.isInteger(port)||port<1024||port>65535){note('host',tr('Порт должен быть от 1024 до 65535.'),true);return;}
    const scope=$('host-scope').value,publicAddress=$('host-public').value.trim();
    if(scope==='internet'&&!publicAddress){note('host',tr('Укажите внешний адрес: белый IP или домен этого компьютера.'),true);$('host-public').focus();return;}
    if(await serverAction(scope==='internet'?{action:'enable',scope,port,public_address:publicAddress}:{action:'enable',scope,port},'host','')){
      serverEdited=false;hideInvitation();
      if(scope==='internet')note('host',tr('Сервер запущен и принимает подключения из интернета по адресу ')+(server.host.public_address||publicAddress)+tr('. Проверьте, что порт на роутере ведёт на этот компьютер и его пропускает брандмауэр.'),false,true);
      else if(scope==='lan')note('host',server.host.addresses.length?tr('Сервер запущен и слушает локальную сеть. Доступ с другого компьютера зависит от брандмауэра.'):tr('Сервер запущен, но адреса в локальной сети у компьютера нет.'),false,true);
      renderServer();
    }
  }
  async function invite(){
    const code=await serverAction({action:'invite',address:$('host-address').value},'host','');
    if(!code)return;
    note('host','');invitation=code;
    $('invite-code').textContent=code.code;copyState('');
    // The 6-digit code for a computer on the same network (T-052).
    $('invite-short').hidden=!code.short&&!code.short_unavailable;
    $('invite-short-code').hidden=!code.short;
    if(code.short_unavailable)$('invite-short-note').textContent=code.short_unavailable;
    if(code.short)$('invite-short-code').textContent=code.short.slice(0,3)+' '+code.short.slice(3);
    clearInterval(inviteTimer);
    const tick=()=>{
      const left=Math.ceil((new Date(code.expires_at)-Date.now())/60000);if(left<=0){hideInvitation();return;}
      $('invite-expiry').textContent=tr('Длинный код действует ещё ')+left+tr(' мин., один раз.');
      if(code.short){const s=Math.ceil((new Date(code.short_expires_at)-Date.now())/60000);$('invite-short-note').textContent=s>0?tr('Для компьютера в этой же сети. Действует ещё ')+s+tr(' мин., три попытки.'):tr('Короткий код больше не действует.');}
    };
    tick();inviteTimer=setInterval(tick,15000);
    $('invite-dialog').showModal();$('invite-copy').focus();
    // The invitation is not in the status yet; show it until the next poll.
    server.host.open_invitations=[...(server.host.open_invitations||[]),code.id];
  }
  async function connectServer(){
    // The computer joins under its one name, «Имя в сети».
    const code=$('member-code').value.trim(),name=$('computer-name').value.trim()||view?.computer_name||'';
    if(!code){note('member',tr('Вставьте код приглашения.'),true);return;}
    if(!name){note('member',tr('Укажите имя в сети.'),true);return;}
    if(await serverAction({action:'connect',code,name},'member',tr('Подключено.')))$('member-code').value='';
  }
  // «Новый проект» opens a dialog; the current project is set up on its own
  // tab «Проект» (T-064).
  // Feedback about the program (T-135): text and a picture stay on this
  // computer; the export writes them all into one file.
  let fbImage=null;
  // An export takes only the remarks no earlier export had; all of them
  // when none is new (owner, 2026-11-16).
  function fbExportLabel(){const n=view?.feedback_count||0,fresh=view?.feedback_new??n;$('fb-export').textContent=(fresh?tr('Выгрузить новые отзывы')+' ('+fresh+')':tr('Выгрузить все отзывы')+(n?' ('+n+')':''));$('fb-export').disabled=!n;}
  // Once the file is passed on, the exported remarks can go (owner,
  // 2026-11-16); a remark written after the export stays.
  function clearFeedback(){
    askConfirm(tr('Очистить журнал отзывов?'),tr('Выгруженные отзывы удалятся с этого компьютера. Файл, который вы сохранили, останется.'),tr('Очистить'),async()=>{
      const r=await postAPI('/api/feedback-clear',{},15000);
      if(view){view.feedback_count=r.result?.left||0;view.feedback_new=r.result?.left||0;}
      $('fb-saved').hidden=true;fbExportLabel();
      $('fb-status').textContent=tr('Журнал очищен')+(r.result?.left?'. '+tr('Новых отзывов осталось: ')+r.result.left:'');
    });
  }
  function openFeedback(){
    $('fb-status').textContent='';$('fb-saved').hidden=true;fbExportLabel();$('fb-dialog').showModal();$('fb-text').focus();
  }
  function fbClear(){fbImage=null;$('fb-shot-full').hidden=true;$('fb-shot-empty').hidden=false;$('fb-thumb').removeAttribute('src');}
  // Large screenshots are scaled down and stored as JPEG, so a remark stays
  // well under the size limit.
  function fbTake(file){
    if(!file||!/^image\/(png|jpeg|webp)$/.test(file.type)){$('fb-status').textContent=tr('Нужна картинка PNG, JPEG или WebP.');return;}
    const url=URL.createObjectURL(file),img=new Image();
    img.onload=()=>{
      const scale=Math.min(1,1600/img.width),w=Math.round(img.width*scale),h=Math.round(img.height*scale);
      const c=document.createElement('canvas');c.width=w;c.height=h;c.getContext('2d').drawImage(img,0,0,w,h);URL.revokeObjectURL(url);
      const data=c.toDataURL('image/jpeg',0.85);
      fbImage={type:'image/jpeg',data:data.slice(data.indexOf(',')+1)};
      $('fb-thumb').src=data;$('fb-shot-info').textContent=tr('Снимок ')+img.width+'×'+img.height+', '+Math.round(fbImage.data.length*3/4/1024)+tr(' КБ');
      $('fb-shot-empty').hidden=true;$('fb-shot-full').hidden=false;$('fb-status').textContent='';
    };
    img.onerror=()=>{URL.revokeObjectURL(url);$('fb-status').textContent=tr('Картинку не удалось открыть.');};
    img.src=url;
  }
  async function saveFeedback(){
    const text=$('fb-text').value.trim();
    if(!text&&!fbImage){$('fb-status').textContent=tr('Напишите, что не так, или приложите снимок.');return;}
    $('fb-save').disabled=true;$('fb-status').textContent=tr('Сохраняем…');
    try{
      await postAPI('/api/feedback',{project:view?.project||'',text,image:fbImage?.data||'',image_type:fbImage?.type||''},15000);
      $('fb-text').value='';fbClear();if(view){view.feedback_count=(view.feedback_count||0)+1;view.feedback_new=(view.feedback_new||0)+1;}fbExportLabel();
      $('fb-status').textContent=tr('Отзыв сохранён. Спасибо!');
    }catch(e){$('fb-status').textContent=e.message;}
    finally{$('fb-save').disabled=false;}
  }
  async function exportFeedback(){
    // No «Выгружаем…» line: it appeared for a moment and made the dialog
    // jump (owner, T-152). The button is disabled meanwhile.
    $('fb-export').disabled=true;$('fb-status').textContent='';
    try{
      const r=await postAPI('/api/feedback-export',{},15000);
      if(view)view.feedback_new=0;
      const cut=Math.max(r.result.path.lastIndexOf('/'),r.result.path.lastIndexOf('\\'));$('fb-dir').textContent=tr('Файл сохранён в папке ')+r.result.path.slice(0,cut>0?cut:0)+':';$('fb-path').textContent=r.result.path.slice(cut+1);$('fb-path').title=r.result.path;$('fb-copy').dataset.path=r.result.path;$('fb-copy').removeAttribute('data-copied');$('fb-saved').hidden=false;$('fb-status').textContent='';
    }catch(e){$('fb-status').textContent=e.message;}
    finally{fbExportLabel();}
  }
  function openProjectDialog(){
    $('project-name').value='';$('project-folder').value='';$('project-rules').value='';$('project-status').textContent='';
    $('project-dialog').showModal();
  }
  async function createProject(){
    const name=$('project-name').value.trim(),folder=$('project-folder').value.trim();
    if(!name||!folder){$('project-status').textContent=tr('Укажите название и папку.');return;}
    $('project-save').disabled=true;$('project-status').textContent=tr('Сохраняем…');
    try{
      await postAPI('/api/project-create',{name,folder});
      const rules=$('project-rules').value.trim();
      if(rules)await postAPI('/api/project-rules',{name,rules});
      setProject(name);remember('messenger-project',name);
      $('project-dialog').close();viewSignature='';await refresh();
      // Connect the coordinator right away; the result is shown, not retried.
      if(view?.project===name)await connectReadyCoordinators();
    }catch(error){$('project-status').textContent=error.message;}
    finally{$('project-save').disabled=false;}
  }
  // The tab saves by itself (owner): the instructions shortly after typing
  // stops, the folder when the field is left, Enter is pressed or a folder is
  // picked, so a half-typed path is not stored. Saves go one at a time; a
  // failed or unanswered save is shown and not repeated; a good one shows
  // nothing (T-064).
  let projectPageFor='',projectSaving=Promise.resolve(),rulesTimer=0;
  // One row of a step list: a mark (✓, ✕, a number or a ring), a title with
  // an optional «?» tip, a subtitle and an optional button.
  function stepRow(mark,title,sub,action,tip){
    const row=el('div',undefined,'am-step');
    if(mark instanceof Node){const m=el('span',undefined,'am-mark am-mark-ring');m.append(mark);row.append(m);}
    else row.append(el('span',mark==='ok'?'✓':mark==='no'?'✕':String(mark),'am-mark '+(mark==='ok'?'am-mark-ok':mark==='no'?'am-mark-no':'am-mark-off')));
    const text=el('div',undefined,'am-step-text'+(typeof mark==='number'?' am-step-later':''));
    const line=el('p',undefined,'am-step-title');
    if(title instanceof Node)line.append(title);else line.textContent=title;
    line.title=line.textContent;
    if(tip){const q=el('span','?','am-q');q.title=tip;q.setAttribute('aria-label',tip);q.tabIndex=0;line.append(q);}
    text.append(line);
    if(sub instanceof Node){const s=el('p',undefined,'am-step-sub');s.append(sub);text.append(s);}
    else if(sub){const s=el('p',sub,'am-step-sub');s.title=sub;text.append(s);}
    row.append(text);
    if(action){const b=el('button',action[0]);b.type='button';b.addEventListener('click',action[1]);row.append(b);}else row.append(el('span'));
    return row;
  }
  function renderProjectPage(){
    const name=view?.project||'';
    $('project-none').hidden=!!name;$('project-page').hidden=!name;
    if(!name){projectPageFor='';return;}
    if(projectPageFor!==name){$('project-page-status').textContent='';$('project-rules-status').textContent='';clearTimeout(rulesTimer);}
    const fresh=projectPageFor!==name;projectPageFor=name;
    if(fresh||document.activeElement!==$('project-page-name'))$('project-page-name').value=view.project_title||name;
    // Other computers and the coordinator chat know the project by the name it
    // was created with; a rename shows it so the owner can match them (T-065).
    // Tasks reach another computer's project only by this exact name.
    const renamed=!!view.project_title&&view.project_title!==name;$('project-page-key').hidden=false;
    $('project-page-key').textContent=(renamed?tr('Для других компьютеров и координатора проект по-прежнему называется «')+name+'». ':'')+tr('На других компьютерах сети у проекта должно быть точно такое же название «')+name+tr('»: задачу принимает проект с тем же названием.');
    if(fresh||document.activeElement!==$('project-page-folder'))$('project-page-folder').value=view.folder||'';
    if(fresh||(document.activeElement!==$('project-page-rules')&&!rulesTimer))$('project-page-rules').value=view.worker_rules||'';
    renderCoordinatorRow();
  }
  function saveProjectField(field){
    const name=view?.project;if(!name)return;
    const status=$(field==='rules'?'project-rules-status':'project-page-status');
    projectSaving=projectSaving.then(async()=>{
      if(view?.project!==name)return;
      try{
        if(field==='title'){
          const title=$('project-page-name').value.trim();
          if(!title){status.textContent=tr('Укажите название.');return;}
          if(title===(view.project_title||name))return;
          status.textContent='';await postAPI('/api/project-title',{name,title});
        }else if(field==='folder'){
          const folder=$('project-page-folder').value.trim();
          if(!folder){status.textContent=tr('Укажите папку.');return;}
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
    askConfirm(tr('Удалить проект «')+(view.project_title||name)+'»?',tr('Проект пропадёт из списка. История задач сохранится и вернётся, если создать проект с тем же именем.'),tr('Удалить'),async()=>{
      await postAPI('/api/project-remove',{name});
      setProject('');remember('messenger-project','');selectedTask='';selectedShared=null;feed=null;feedSignature='';viewSignature='';projectPageFor='';await refresh();
    });
  }
  // Installation and sign-in use the agent's own terminal commands.
  // A row note shows for a while, then the row offers its action again.
  function noteRow(key,text,ms){rowNote.set(key,text);renderAgents();if(ms)setTimeout(()=>{if(rowNote.get(key)===text){rowNote.delete(key);renderAgents();}},ms);}
  async function installAgent(provider,action='install'){
    setupRecoverable.delete(provider);
    noteRow(provider+':program',tr('Открываем терминал…'));
    try{const state=await postAPI(action==='find'?'/api/agent-find':'/api/agent-install',{provider,action});setupPending=true;setupSeen.delete(state.id);await pollSetup();}
    catch(error){rowNote.delete(provider+':program');renderAgents();showError(error);}
  }
  const setupSeen=new Map(),setupFinishing=new Set(),setupRecoverable=new Set();let setupPolling=false,setupPending=false;
  async function pollSetup(){
    if(setupPolling)return;setupPolling=true;
    try{
      const states=await api('/api/agent-setup-status');if(!Array.isArray(states))return;
      setupPending=states.some(s=>['running','unknown','found'].includes(s.state));
      for(let state of states){
        const {provider,id}=state;if(!id||state.state==='idle')continue;
        if(state.state==='found'&&!setupFinishing.has(id)){
          setupFinishing.add(id);
          try{state=await postAPI('/api/agent-setup-finish',{provider,action:state.action,id});}
          catch(error){showError(error);}
        }
        if(setupSeen.get(id)===state.state)continue;setupSeen.set(id,state.state);
        if(state.state==='running'||state.state==='unknown'){
          if(state.action==='install')setupRecoverable.add(provider);
          noteRow(provider+':program',state.action==='install'?tr('Нет подтверждения из терминала. Если он закрыт, нажмите «Найти».'):tr('Поиск не подтверждён. Проверьте терминал или выберите файл CLI в настройках.'));
        }
        else if(state.state==='connected'){
          setupRecoverable.delete(provider);
          rowNote.delete(provider+':program');viewSignature='';await refresh();await loadDiscovery();checkCard(provider,true);
          if(setupProvider===provider&&$('agent-setup').open)$('agent-cli').value=state.cli;
        }else if(state.state==='failed')noteRow(provider+':program',tr('Установка или поиск не завершились. Проверьте терминал; можно выбрать файл CLI в настройках.'),15000);
        else if(state.state==='unconfirmed')noteRow(provider+':program',tr('Подключение не подтверждено. Проверьте настройки агента.'),15000);
      }
    }catch(_){}finally{setupPolling=false;}
  }
  function trustFolder(provider){
    const folder=view?.folder,selectedProject=view?.project;
    if(!folder||!selectedProject)return;
    askConfirm(tr('Доверять папке'),folder+'\n\n'+tr('Откроется CLI в терминале. Подтвердите доверие там: агент сможет читать настройки и выполнять команды проекта. Графическое приложение не требуется.'),tr('Открыть терминал'),async()=>{
      await postAPI('/api/agent-trust',{provider,project:selectedProject,folder});
    },true);
  }
  // After «Войти» or «Выйти» the window checks the agent again when the owner
  // comes back to it, so the card shows the new state without a click.
  const authOpened=new Map();
  async function loginFromCard(provider,cli,environment){
    noteRow(provider+':auth',tr('Открываем вход…'));
    try{await postAPI('/api/agent-login',{provider,cli,environment});authOpened.set(provider,Date.now());noteRow(provider+':auth',tr('Окно входа открыто'),15000);}
    catch(error){rowNote.delete(provider+':auth');renderAgents();showError(error);}
  }
  function logoutFromCard(provider,cli,environment){
    askConfirm(tr('Выйти из ')+names[provider]+'?',tr('Агент на этом компьютере перестанет брать задачи, пока вы снова не войдёте.'),tr('Выйти'),async()=>{
      await postAPI('/api/agent-logout',{provider,cli,environment});authOpened.set(provider,Date.now());noteRow(provider+':auth',tr('Окно выхода открыто'),15000);
    });
  }
  window.addEventListener('focus',()=>{
    for(const [provider,at] of authOpened){if(Date.now()-at>=10*60*1000)authOpened.delete(provider);else if(!checking.has(provider))checkCard(provider,true);}
    pollSetup();viewSignature='';refresh();
  });

  let setupProvider='';
  // Starting lists, the same as the store's (T-061, T-098); shown for an
  // agent whose list is still empty and saved with the dialog.
  const defaultModels={claude:['claude-haiku-5-5[effort=low]','claude-sonnet-5-5[effort=medium]','claude-opus-5-5[effort=medium]','claude-fable-5-2[effort=high]'],codex:['gpt-6-luna[effort=high]','gpt-6.2-sol[effort=low]','gpt-6.2-sol[effort=medium]','gpt-6.2-sol[effort=high]','gpt-6.2-astra[effort=medium]','gpt-6.2-astra[effort=high]'],cursor:['composer-2.5','cursor-grok-4.6-medium','grok-4.7-medium','grok-4.7-high']};
  function configuredModels(provider){
    const agent=(view?.agents||[]).find(a=>a.agent===provider);
    return view?.coordinator_models?.[provider]??(agent?agent.models||[]:defaultModels[provider]||[]);
  }
  function openAgentSetup(provider){
    setupProvider=provider;const a=(view?.agents||[]).find(x=>x.agent===provider),found=discovery.find(d=>d.provider===provider);
    $('agent-setup-title').textContent=tr('Настройки: ')+names[provider];
    $('agent-setup-note').hidden=provider!=='cursor';
    $('agent-cli').value=a?.cli||found?.cli||'';$('agent-profile').value=a?.environment||found?.environment||'';
    const models=configuredModels(provider);
    $('agent-models').value=models.join(', ');$('agent-models').disabled=!view;
    $('agent-models').placeholder=tr('Пусто: задачи идут на модель по умолчанию');
    $('agent-models-note').textContent=tr('Через запятую. От быстрых правок к сложным задачам. Доступ зависит от аккаунта.')+' '+(provider==='cursor'?tr('Имена из agent --list-models; High требует подходящего тарифа.'):tr('Уровень: [effort=medium] или [effort=high]. Messenger передаст его отдельным параметром.'));
    setStatus('agent-save-status','');
    if(!$('agent-setup').open)$('agent-setup').showModal();
  }
  // One check at a time: the client refuses a second one while one runs.
  let checkQueue=Promise.resolve();
  function runCheck(selection){
    const run=checkQueue.then(async()=>{
      const response=await fetch('/api/check-cli',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(selection),cache:'no-store',signal:AbortSignal.timeout(22000)});
      if(!response.ok)throw Error(response.status===409?tr('Проверка уже выполняется.'):response.status===422?tr('Проверьте путь к программе и профилю.'):tr('Проверка не выполнена.'));
      const result=await response.json();if(result.ok!==true)throw Error(tr('Ответ проверки не подтверждён.'));
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
    if(checking.has(provider))return;
    const selection=agentPaths(provider);if(!selection.cli)return;
    rowNote.delete(provider+':auth');
    checking.add(provider);renderAgents();
    try{await runCheck(selection);}catch(error){agentChecks.set(provider,{authentication:{state:'unknown'}});if(!quiet)showError(error);}
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
    noteRow(provider+':program',tr('Подключаем…'));
    try{await postAPI('/api/agent-set',{provider,cli:found.cli,environment:''});viewSignature='';await refresh();checkCard(provider);}
    catch(error){showError(error);}
    finally{rowNote.delete(provider+':program');renderAgents();}
  }
  // The dialog's statuses sit on the line of their button and replace each
  // other there, so the dialog never grows or jumps (owner, T-056).
  function setStatus(id,text){const n=$(id);n.textContent=text;n.title=text;}
  // Every change saves by itself (owner, T-056, T-061): the program and
  // profile when a field is left or a path is chosen, then the card checks
  // it; the models when their field is left. Only errors are shown.
  async function saveAgentPaths(){
    const provider=setupProvider,cli=$('agent-cli').value.trim();if(!cli)return;
    const found=discovery.find(d=>d.provider===provider);
    let environment=$('agent-profile').value.trim();if(found&&environment===found.environment)environment='';
    const a=(view?.agents||[]).find(x=>x.agent===provider);
    if(a&&a.cli===cli&&(a.environment||'')===environment)return;
    setStatus('agent-save-status','');
    try{await postAPI('/api/agent-set',{provider,cli,environment});
      viewSignature='';await refresh();if(setupProvider===provider)openAgentSetup(provider);checkCard(provider);}
    catch(error){setStatus('agent-save-status',error.message);}
  }
  async function saveAgentModels(){
    if(!view||!setupProvider)return;
    const models=$('agent-models').value.split(/[,\n]/).map(m=>m.trim()).filter(Boolean);
    if(models.join('\n')===configuredModels(setupProvider).join('\n'))return;
    setStatus('agent-save-status','');
    try{await postAPI('/api/agent-models',{provider:setupProvider,models});viewSignature='';await refresh();}
    catch(error){setStatus('agent-save-status',error.message);return;}
    // A name a letter or two away from a known one is likely a typo
    // (coordinator, 2026-11-12: «claude-opus-5-thinking-hig»). It is saved
    // anyway: the agent may know names this window does not.
    const known=defaultModels[setupProvider]||[];
    const typos=models.map(m=>[m,known.find(k=>k!==m&&!models.includes(k)&&editDistance(m.toLowerCase(),k.toLowerCase())<=2)]).filter(p=>p[1]);
    if(typos.length)setStatus('agent-save-status',typos.map(([m,k])=>tr('Похоже на опечатку: «')+m+tr('», может быть «')+k+'»?').join(' '));
  }
  function editDistance(a,b){
    let prev=Array.from({length:b.length+1},(_,j)=>j);
    for(let i=1;i<=a.length;i++){
      const cur=[i];
      for(let j=1;j<=b.length;j++)cur[j]=Math.min(prev[j]+1,cur[j-1]+1,prev[j-1]+(a[i-1]===b[j-1]?0:1));
      prev=cur;
    }
    return prev[b.length];
  }
  // Windows (T-062) and Linux (T-082) without the system title bar: the page
  // draws minimise, maximise and close at the top right, and the empty top of
  // the window (the tab row and the top of the sidebar) drags it; a double
  // click there maximises or restores it.
  function framelessWindow(){
    if(!['windows','linux'].includes(window.amDesktop)||!window.amFrameless||typeof window.amWindow!=='function')return;
    document.body.classList.add('am-frameless');
    const bar=el('div',undefined,'am-window-controls');
    const make=(action,label,paths,cls)=>{const b=el('button',undefined,'am-window-btn'+(cls?' '+cls:''));b.type='button';b.title=label;b.setAttribute('aria-label',label);b.tabIndex=-1;b.append(svgIcon(paths));b.addEventListener('click',()=>window.amWindow(action));bar.append(b);};
    make('minimize',tr('Свернуть'),['M5 12h14']);
    make('maximize',tr('Развернуть'),['M6 6h12v12H6z']);
    make('close',tr('Закрыть'),['M6 6l12 12M18 6L6 18'],'am-window-close');
    root.append(bar);
    const dragZone=e=>{
      if(e.button!==0||e.target.closest(titleInteractive))return false;
      if(e.target.closest('.am-tabs'))return true;
      const side=e.target.closest('.am-sidebar');
      return !!side&&e.clientY<side.getBoundingClientRect().top+40;
    };
    document.addEventListener('mousedown',e=>{
      if(!dragZone(e))return;
      e.preventDefault();
      window.amWindow(e.detail===2?'maximize':'drag');
    });
  }
  const titleInteractive='button,input,select,textarea,a,[role=switch],[popover],.am-machine-card,.am-project,.am-splitter,.am-window-controls';
  // Mac (T-082): the page runs under a transparent title bar with the
  // system's buttons at the top left. The app moves the window by the empty
  // parts of the top, so the page tells it where they are: the tab row and
  // the sidebar above its first item, less the controls in them.
  function macTitlebar(){
    const bridge=window.webkit?.messageHandlers?.titlebar;
    if(!window.amMacTitlebar||!bridge)return;
    document.body.classList.add('am-mac-titlebar');
    const box=r=>[r.left,r.top,r.width,r.height].map(Math.round);
    let last='',queued=false;
    const send=()=>{
      queued=false;
      const zones=[],holes=[];
      const tabs=document.querySelector('.am-tabs'),side=document.querySelector('.am-sidebar');
      if(tabs&&tabs.offsetParent){const r=tabs.getBoundingClientRect();zones.push(new DOMRect(r.left,0,r.width,r.bottom));}
      if(side){const r=side.getBoundingClientRect(),first=side.firstElementChild?.getBoundingClientRect();zones.push(new DOMRect(r.left,r.top,r.width,Math.max(16,(first?first.top:r.top+40)-r.top)));}
      for(const parent of [tabs,side])for(const node of parent?parent.querySelectorAll(titleInteractive):[]){
        const r=node.getBoundingClientRect();
        if(r.width&&r.height&&zones.some(z=>r.left<z.right&&r.right>z.left&&r.top<z.bottom&&r.bottom>z.top))holes.push(r);
      }
      const message={zones:zones.map(box),holes:holes.map(box)},text=JSON.stringify(message);
      if(text!==last){last=text;bridge.postMessage(message);}
    };
    const later=()=>{if(!queued){queued=true;requestAnimationFrame(send);}};
    new ResizeObserver(later).observe(document.documentElement);
    for(const node of document.querySelectorAll('.am-tabs,.am-sidebar'))new MutationObserver(later).observe(node,{childList:true,subtree:true,attributes:true});
    addEventListener('resize',later);
    later();
  }
  window.messengerRefresh=()=>{viewSignature='';refresh();};
  // «Выбрать»: the local server opens the system's own dialog (D-053); the
  // Mac app keeps its native folder dialog for the project folder.
  async function pick(field,kind,status){
    const input=$(field);$(status).textContent='';
    try{
      const response=await fetch('/api/pick',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({kind,current:input.value.trim()}),cache:'no-store',signal:AbortSignal.timeout(610000)});
      let result={};try{result=await response.json();}catch(_){}
      if(!response.ok||result.ok!==true)throw Error(result.detail||tr('Окно выбора не открылось. Вставьте путь вручную.'));
      if(result.path&&result.path!==input.value){input.value=result.path;input.dispatchEvent(new Event('change'));}
      $(status).textContent='';
    }catch(error){$(status).textContent=error.name==='TimeoutError'?tr('Окно выбора не ответило. Вставьте путь вручную.'):error.message;}
  }
  // The Mac shell's folder picker answers here, into the field that asked.
  let folderTarget='project-folder';
  window.messengerFolderPicked=path=>{if(typeof path==='string'&&path){$(folderTarget).value=path;$(folderTarget).dispatchEvent(new Event('input'));}};
  // The window names a build the short way, «v0.41» for 0.0.0-preview.41
  // (owner); the release tag stays as it is because updates compare it. The
  // Mac app already reports the short form (0.58).
  function versionName(version){
    const preview=version.match(/^0\.0\.0-preview\.(\d+)$/);
    return preview?'v0.'+preview[1]:version?'v'+version:'';
  }
  function versionText(installed){return versionName(installed);}
  // Comparing builds: 0.0.0-preview.41 and 0.41 are the same build.
  function versionRank(version){
    const v=String(version||'').replace(/^v/,'');
    const preview=v.match(/^0\.0\.0-preview\.(\d+)$/);
    return preview?[0,Number(preview[1])]:v.split(/[.-]/).map(x=>Number(x)||0);
  }
  function versionOlder(a,b){
    if(!a||!b)return false;
    const x=versionRank(a),y=versionRank(b);
    for(let i=0;i<Math.max(x.length,y.length);i++){if((x[i]||0)!==(y[i]||0))return (x[i]||0)<(y[i]||0);}
    return false;
  }
  // The newest build this window knows of: its own, the background
  // program's, an update that came out, or another computer's.
  let knownUpdate='';
  function newestVersion(){
    let best='';
    for(const v of [clientInfo?.window_version,clientInfo?.client_version,knownUpdate,...(server?.peers||[]).map(p=>p.version)])if(v&&(!best||versionOlder(best,v)))best=v;
    return best;
  }
  // The update icon says in its tooltip which version came out (T-066).
  function updateTip(version,busy){
    const name=versionName(version);
    let text=busy==='installing'?tr('Обновление устанавливается'):busy?tr('Обновление загружается'):name?tr('Вышла ')+name+tr('. Нажмите, чтобы обновить'):tr('Обновить');
    if(updateProgress)text=tr('Загружается обновление')+(updateProgress.version?' '+versionName(updateProgress.version):'')+': '+Math.floor(100*updateProgress.done/updateProgress.total)+'%';
    $('app-update-button').title=text;$('app-update-button').setAttribute('aria-label',text);
  }
  // While an update downloads, a ring around the same arrow fills from 0 to
  // 100% (owner, 2026-11-14). The download writes its progress wherever it
  // runs (the background check, or the Mac app's fetch helper); the window
  // asks for it often while it lasts and rarely otherwise.
  let updateProgress=null,progressTimer=0;
  function showUpdateProgress(){
    const b=$('app-update-button'),p=updateProgress;
    b.classList.toggle('am-update-loading',!!p);
    if(p)b.style.setProperty('--am-update-p',String(Math.min(1,Math.max(0,p.done/p.total))));
    else b.style.removeProperty('--am-update-p');
  }
  async function pollUpdateProgress(){
    clearTimeout(progressTimer);
    let next=null;
    try{const r=await api('/api/update-progress');if(r?.downloading&&r.total>0)next={version:String(r.version||''),done:Number(r.done)||0,total:Number(r.total)};}catch(_){}
    const changed=!!next!==!!updateProgress;
    updateProgress=next;
    // The Mac downloads from GitHub before Sparkle installs: the run shows
    // that download (T-213), and progress keeps it from looking stuck.
    if(next&&updateRun?.mac&&!updateRun.error&&updateRun.phase==='downloading'){updateRun.done=next.done;updateRun.total=next.total;if(next.version)updateRun.version=next.version;macRunQuiet();renderMacRun();}
    if(changed){if(window.webkit?.messageHandlers?.updates)renderMacUpdate();else if(selfUpdate)renderSelfUpdate();else showUpdateProgress();}
    else if(window.webkit?.messageHandlers?.updates){showUpdateProgress();if(next)updateTip(next.version,true);}
    else renderUpdateDialog();
    // A download that just ended may have made an update ready: ask again.
    if(changed&&!next&&!window.webkit?.messageHandlers?.updates)loadSelfUpdate();
    progressTimer=setTimeout(pollUpdateProgress,next?400:document.hidden?15000:2500);
  }
  let macUpdate=null;
  window.messengerUpdateStatus = status => {
    if(!window.webkit?.messageHandlers?.updates)return;
    macUpdate=status||{};renderMacUpdate();
    if(macUpdate.checking)pollUpdateProgress();
  };
  function renderMacUpdate(){
    const status=macUpdate||{};
    const version=typeof status?.version==='string'?status.version:'';
    const installed=typeof status?.installedVersion==='string'?status.installedVersion:'';
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Версия ')+versionName(installed).slice(1):'';
    // Only an update ready to install is shown, as an arrow next to
    // the version: a download in the background stays quiet, so the icon
    // does not appear once for the download and again for the install
    // (owner, 2026-11-15). Background checks and their failures stay quiet.
    const loading=!!updateProgress&&!!version;
    const shown=!!version;
    if(version)knownUpdate=version;
    $('app-update-version').hidden=true;$('app-update-button').hidden=!shown;
    $('app-installed-version').textContent=versionText(installed);
    updateTip(version,loading);
    showUpdateProgress();
    $('app-update-button').disabled=loading;
  }
  // Windows and Linux: the program updates itself from the window (D-048).
  // While the check downloads, the icon is the ring; the arrow appears when
  // a release is ready to install.
  let selfUpdate=null,selfUpdateBusy=false,selfUpdateTimer=0;
  function renderSelfUpdate(){
    const s=selfUpdate;if(!s)return;
    const installed=typeof s.installed_version==='string'?s.installed_version:'';
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Версия ')+versionName(installed).slice(1):'';
    const version=typeof s.version==='string'?s.version:'';
    if(version&&s.state==='available')knownUpdate=version;
    // A check that only asks GitHub shows nothing; a download fills the ring
    // around the arrow; installing dims the arrow.
    // The archive downloads only when the owner presses the button, so the
    // icon has no ring of its own: the update dialog shows the progress.
    const offered=(s.state==='available'||s.state==='found')&&!!version;
    if(offered)knownUpdate=version;
    const installing=selfUpdateBusy||s.state==='installing'||s.state==='downloading',loading=false;
    const done=s.state==='installed',shown=offered||installing||done||!!s.message;
    // The arrow while an update waits (owner, T-056, T-066). A line appears
    // only for an error or after the update.
    $('app-update-version').hidden=!s.message&&!done;$('app-update-button').hidden=!shown||done;
    $('app-installed-version').textContent=versionText(installed);
    $('app-update-version').textContent=s.message||(done?tr('Обновлено. Окно откроется заново.'):'');
    updateTip(version,installing?'installing':loading);
    $('app-update-button').classList.remove('am-update-loading');
    $('app-update-button').disabled=installing||loading;
    renderUpdateDialog();
  }
  async function loadSelfUpdate(){
    if(window.webkit?.messageHandlers?.updates||selfUpdateBusy)return;
    try{const s=await api('/api/self-update');selfUpdate={...s,message:selfUpdate?.message||''};renderSelfUpdate();}
    catch(_){if(!selfUpdate&&selfUpdateTimer){clearInterval(selfUpdateTimer);selfUpdateTimer=0;}}
  }
  // Windows and Linux: one button downloads, installs and restarts, and a
  // dialog shows each step (owner, 2026-11-15: a download first and the same
  // icon again after it was confusing).
  let updateRun=null,updateRunTimer=0;
  function renderUpdateDialog(){
    if(!updateRun)return;
    if(updateRun.mac){renderMacRun();return;}
    const s=selfUpdate||{},p=updateProgress,state=s.state||'';
    if(s.version)updateRun.version=s.version;
    if(state==='downloading'||state==='installing'||state==='installed')updateRun.busy=true;
    // Back to offered after the run began: the install failed and the old
    // program stayed. Seen twice, so the instant between download and
    // install is not mistaken for it.
    if(!updateRun.error&&updateRun.busy&&(state==='failed'||state==='found'||state==='available'||state==='idle')){
      updateRun.back=(updateRun.back||0)+1;
      if(state==='failed'||updateRun.back>=2)updateRun.error=s.error||tr('Обновление не установлено. Программа осталась прежней.');
    }else if(state!=='failed')updateRun.back=0;
    $('update-title').textContent=tr('Обновление')+(updateRun.version?' '+versionName(updateRun.version):'');
    let step,share;
    if(updateRun.error){step=tr('Не удалось обновить');share=0;}
    else if(state==='installed'){step=tr('Перезапуск…');share=1;}
    else if(state==='installing'){step=tr('Установка…');share=1;}
    else if(p&&p.total>0){share=Math.min(1,p.done/p.total);step=tr('Загрузка…')+' '+Math.floor(100*share)+'%';}
    else{step=tr('Загрузка…');share=0;}
    $('update-step').textContent=step;
    $('update-bar-fill').style.setProperty('--am-update-share',String(share));
    $('update-error').textContent=updateRun.error||'';$('update-error').hidden=!updateRun.error;
    $('update-actions').hidden=!updateRun.error;
    if(updateRun.error){clearTimeout(updateRunTimer);updateRunTimer=0;}
  }
  async function followUpdate(){
    clearTimeout(updateRunTimer);
    if(!updateRun||updateRun.error)return;
    try{const s=await api('/api/self-update');selfUpdate={...s,message:''};renderSelfUpdate();}catch(_){}
    if(!updateRun||updateRun.error)return;
    pollUpdateProgress();
    updateRunTimer=setTimeout(followUpdate,500);
  }
  async function startUpdate(){
    if(!updateRun)updateRun={version:selfUpdate?.version||''};
    Object.assign(updateRun,{error:'',busy:false,back:0});
    if(!$('update-dialog').open)$('update-dialog').showModal();
    renderUpdateDialog();
    try{
      const r=await postAPI('/api/self-update',{action:'update'},15000);
      if(selfUpdate)selfUpdate.state=r.state||'downloading';
      updateRun.busy=true;
    }catch(error){updateRun.error=error.message;renderUpdateDialog();return;}
    renderSelfUpdate();followUpdate();
  }
  // A Mac: the app's updater does the same steps and reports each one
  // (T-199); Sparkle shows no window of its own.
  function renderMacRun(){
    const r=updateRun;
    $('update-title').textContent=tr('Обновление')+(r.version?' '+versionName(r.version):'');
    let step,share;
    if(r.error){step=tr('Не удалось обновить');share=0;}
    else if(r.phase==='restarting'){step=tr('Перезапуск…');share=1;}
    else if(r.phase==='installing'){step=tr('Установка…');share=1;}
    else if(r.total>0){share=Math.min(1,r.done/r.total);step=tr('Загрузка…')+' '+Math.floor(100*share)+'%';}
    else{step=tr('Загрузка…');share=0;}
    $('update-step').textContent=step;
    $('update-bar-fill').style.setProperty('--am-update-share',String(share));
    $('update-error').textContent=r.error||'';$('update-error').hidden=!r.error;
    $('update-actions').hidden=!r.error;
  }
  // Without any word from the app for two minutes the dialog says so and
  // can be closed; nothing is retried.
  function macRunQuiet(){
    clearTimeout(updateRunTimer);
    updateRunTimer=setTimeout(()=>{if(updateRun?.mac&&!updateRun.error&&updateRun.phase!=='restarting'){updateRun.error=tr('Обновление не ответило. Программа осталась прежней.');renderMacRun();}},120000);
  }
  window.messengerUpdateRun=state=>{
    if(!state)return;
    // The menu's «Обновить» starts a run without the page's button (T-213).
    if(!updateRun?.mac){if(state.phase==='error')return;updateRun={mac:true,version:'',phase:'downloading',done:0,total:0,error:''};if(!$('update-dialog').open)$('update-dialog').showModal();}
    if(typeof state.version==='string'&&state.version)updateRun.version=state.version;
    updateRun.phase=String(state.phase||'');
    updateRun.done=Number(state.done)||0;updateRun.total=Number(state.total)||0;
    updateRun.error=updateRun.phase==='error'?String(state.error||tr('Обновление не установлено. Программа осталась прежней.')):'';
    if(updateRun.error)clearTimeout(updateRunTimer);else macRunQuiet();
    renderMacRun();
  };
  function startMacUpdate(){
    updateRun={mac:true,version:macUpdate?.version||'',phase:'downloading',done:0,total:0,error:''};
    if(!$('update-dialog').open)$('update-dialog').showModal();
    renderMacRun();macRunQuiet();
    window.webkit.messageHandlers.updates.postMessage('install');
  }
  $('update-retry').addEventListener('click',()=>updateRun?.mac?startMacUpdate():startUpdate());
  $('update-close').addEventListener('click',()=>{updateRun=null;clearTimeout(updateRunTimer);$('update-dialog').close();loadSelfUpdate();});
  // The download and the install are not cancelled halfway.
  $('update-dialog').addEventListener('cancel',event=>{if(!updateRun?.error)event.preventDefault();else{updateRun=null;}});
  $('app-update-button').addEventListener('click',()=>{
    if(window.webkit?.messageHandlers?.updates)startMacUpdate();
    else startUpdate();
  });
  if(!window.webkit?.messageHandlers?.updates){selfUpdateTimer=setInterval(loadSelfUpdate,60000);loadSelfUpdate();}
  pollUpdateProgress();
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('list-toggle').addEventListener('click',()=>{
    const expanded=$('list-toggle').getAttribute('aria-expanded')!=='true';
    $('list-toggle').setAttribute('aria-expanded',String(expanded));
    const label=expanded?tr('Скрыть список задач'):tr('Показать список задач');$('list-toggle').setAttribute('aria-label',label);$('list-toggle').title=label;
    $('task-panel').hidden=!expanded;$('task-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  $('project-select').addEventListener('change',()=>{
    const value=$('project-select').value;
    if(value==='__new'){$('project-select').value=view?.project||'__new';syncSelects();openProjectDialog();return;}
    setProject(value);remember('messenger-project',value);selectedTask='';selectedShared=null;feed=null;feedSignature='';viewSignature='';taskHistory=null;if($('device-menu')?.matches(':popover-open'))$('device-menu').hidePopover();renderMachines();if(tab==='agents')renderAgents();renderTaskList();renderThread();refresh();
  });
  $('agent-filter').addEventListener('change',()=>{taskHistory=null;renderTaskList();if(view?.history_paged)refresh();});
  // The sidebar and the task list are as wide as the owner drags them (T-063).
  // A double click or Home restores the usual width; the width is remembered.
  function splitter(pane,name,variable,usual,min,max,side){
    const handle=el('div',undefined,'am-splitter');
    handle.setAttribute('role','separator');handle.setAttribute('aria-orientation','vertical');handle.tabIndex=0;
    handle.title=tr('Потяните, чтобы изменить ширину. Двойной щелчок: как было');
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
  splitter(root.querySelector('.am-sidebar'),tr('Ширина левой панели'),'--am-nav',248,160,420,1);
  splitter($('task-panel'),tr('Ширина списка задач'),'--am-queue',290,200,520,-1);
  // A click on empty space in the sidebar shows all computers' tasks again.
  root.querySelector('.am-sidebar').addEventListener('click',event=>{if(computerFilter&&!event.target.closest('.am-machine-card,.am-project,.am-side-bottom,.am-splitter,button,select,input,a,[popover]'))clearComputer();});
  $('answer-text').addEventListener('input',()=>{sizeAnswer();answerControls();});
  $('answer-send').addEventListener('click',sendAnswer);
  // Enter sends; Shift+Enter adds a line. An IME composition keeps its Enter.
  $('task-stop').addEventListener('click',stopTask);
  $('lost-close').addEventListener('click',closeHere);
  $('task-stop').addEventListener('mouseleave',()=>$('task-stop').classList.remove('am-tip-off'));
  document.addEventListener('keydown',e=>{if(e.key==='Escape'&&!e.isComposing&&!$('task-stop').hidden&&!$('compose').hidden&&!document.querySelector('dialog[open]')&&!e.target.closest?.('[role=listbox],[aria-expanded=true]')){e.preventDefault();stopTask();}});
  $('answer-text').addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing&&e.keyCode!==229){e.preventDefault();sendAnswer();}});
  $('project-dialog-close').addEventListener('click',()=>$('project-dialog').close());
  $('settings-open').addEventListener('click',()=>{$('auto-update-status').textContent='';$('settings-dialog').showModal();loadServer();});
  $('auto-update-toggle').addEventListener('click',toggleAutoUpdate);
  $('settings-close').addEventListener('click',()=>$('settings-dialog').close());
  setupPictureViewer();
  $('fb-open').addEventListener('click',openFeedback);
  $('fb-close').addEventListener('click',()=>$('fb-dialog').close());
  $('fb-save').addEventListener('click',saveFeedback);
  $('fb-export').addEventListener('click',exportFeedback);
  $('fb-clear').addEventListener('click',clearFeedback);
  $('fb-pick').addEventListener('click',()=>$('fb-file').click());
  $('fb-file').addEventListener('change',e=>{fbTake(e.target.files[0]);e.target.value='';});
  $('fb-shot-remove').addEventListener('click',fbClear);
  $('fb-copy').addEventListener('click',async()=>{const b=$('fb-copy');try{await navigator.clipboard.writeText(b.dataset.path||'');b.setAttribute('data-copied','');}catch(_){$('fb-status').textContent=tr('Скопируйте путь вручную: ')+(b.dataset.path||'');}setTimeout(()=>b.removeAttribute('data-copied'),2000);});
  $('fb-dialog').addEventListener('paste',e=>{const f=[...(e.clipboardData?.files||[])].find(x=>x.type.startsWith('image/'));if(f){e.preventDefault();fbTake(f);}});
  const fbShot=$('fb-shot');
  fbShot.addEventListener('dragover',e=>{e.preventDefault();fbShot.classList.add('am-drop');});
  fbShot.addEventListener('dragleave',()=>fbShot.classList.remove('am-drop'));
  fbShot.addEventListener('drop',e=>{e.preventDefault();fbShot.classList.remove('am-drop');fbTake(e.dataTransfer.files[0]);});
  $('invite-close').addEventListener('click',()=>$('invite-dialog').close());
  // A closed invitation is not shown again; a new one can be made at any time.
  // A click on the dimmed area outside an open dialog closes it, like its ✕
  // (owner, T-152). The restart dialog stays: it waits for the restart.
  for(const dialog of document.querySelectorAll('dialog')){
    if(dialog.id==='restart-dialog')continue;
    let downOutside=false;
    const outside=event=>{const r=dialog.getBoundingClientRect();return event.target===dialog&&(event.clientX<r.left||event.clientX>r.right||event.clientY<r.top||event.clientY>r.bottom);};
    dialog.addEventListener('mousedown',event=>{downOutside=outside(event);});
    dialog.addEventListener('click',event=>{if(downOutside&&outside(event))dialog.close();downOutside=false;});
  }
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
  framelessWindow();
  macTitlebar();
  $('agent-cli-find').addEventListener('click',()=>{const provider=setupProvider;$('agent-setup').close();installAgent(provider,'find');});
  $('agent-cli-pick').addEventListener('click',()=>pick('agent-cli','file','agent-save-status'));
  $('agent-profile-pick').addEventListener('click',()=>pick('agent-profile','folder','agent-save-status'));
  $('restart-dialog').addEventListener('cancel',event=>event.preventDefault());
  $('restart-go').addEventListener('click',()=>{if(restart.ask){restart.wait=true;renderRestart();restartClient(false);}else restartClient(false);});
  $('restart-cut').addEventListener('click',()=>{restart.wait=false;restartClient(true);});
  $('restart-quit').addEventListener('click',()=>window.webkit?.messageHandlers?.app?.postMessage('quit'));
  $('restart-copy').addEventListener('click',async()=>{const b=$('restart-copy');try{await navigator.clipboard.writeText(shutdownCommand());b.textContent=tr('Скопировано');}catch(_){b.textContent=tr('Не удалось скопировать');}setTimeout(()=>b.textContent=tr('Скопировать команду'),2000);});
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-cli').addEventListener('change',saveAgentPaths);
  $('agent-profile').addEventListener('change',saveAgentPaths);
  $('agent-models').addEventListener('change',saveAgentModels);
  $('agent-models').addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();saveAgentModels();}});
  $('host-scope').addEventListener('change',()=>{serverEdited=true;$('host-public-box').hidden=$('host-scope').value!=='internet';});
  $('host-public').addEventListener('input',()=>{serverEdited=true;});
  $('host-port').addEventListener('input',()=>{serverEdited=true;});
  $('host-enable').addEventListener('click',enableServer);
  $('host-disable').addEventListener('click',()=>serverAction({action:'disable'},'host','').then(r=>{if(r){serverEdited=false;hideInvitation();renderServer();}}));
  $('host-invite').addEventListener('click',invite);
  // The copy icon turns into a check mark once the code is on the clipboard.
  function copyState(state){const b=$('invite-copy');b.toggleAttribute('data-copied',state==='done');
    const label=state==='done'?tr('Скопировано'):state==='failed'?tr('Не удалось скопировать — выделите код вручную'):tr('Скопировать');
    b.title=label;b.setAttribute('aria-label',state?label:tr('Скопировать код'));}
  $('invite-copy').addEventListener('click',async()=>{try{await navigator.clipboard.writeText($('invite-code').textContent);copyState('done');}catch(_){copyState('failed');}});
  $('member-connect').addEventListener('click',connectServer);
  $('computer-name').addEventListener('input',e=>{e.target.dataset.edited='1';renderServer();});
  // «Имя в сети» saves by itself when the field is left or Enter is pressed;
  // only an error is shown.
  async function renameComputer(){const cn=$('computer-name'),name=cn.value.trim();if(!name||name===view?.computer_name||!cn.dataset.edited)return;if(await serverAction({action:'rename',name},'computer','')){delete cn.dataset.edited;await refresh();renderServer();}}
  $('computer-name').addEventListener('change',renameComputer);
  $('computer-name').addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();renameComputer();}});
  $('log-share-toggle').addEventListener('click',()=>{if(!server)return;const on=!server.log_share;serverAction({action:'log_share',on},'accept',on?tr('Координаторы других компьютеров смогут прочитать журнал.'):'');});
  $('net-full-toggle').addEventListener('click',toggleNetAccess);
  $('accept-toggle').addEventListener('click',()=>{if(!server)return;const on=!server.accept;serverAction({action:'accept',on},'accept',on?'':tr('Новые задачи других компьютеров будут отклоняться. Уже принятые продолжат работу.'));});
  $('member-disconnect').addEventListener('click',()=>serverAction({action:'disconnect'},'member',tr('Отключено. Устройство остаётся в списке сервера, пока его там не отзовут.')));
  let serverPolls=0;
  // The sidebar lists devices from the server state, so it is read on every
  // tab, every 15 s outside «Сервер» (as often as computers report).
  // The window's own page menu offers «Перезагрузить», which only reloads
  // the page and reads as restarting the program. It stays where it helps:
  // in text fields and on selected text.
  document.addEventListener('contextmenu',event=>{
    if(event.defaultPrevented||event.target.closest?.('input,textarea,[contenteditable]')||String(window.getSelection?.()||''))return;
    event.preventDefault();
  });
  // T-275: a hidden window asks for the network every minute, not every
  // 5 or 15 s, and catches up when shown.
  setInterval(()=>{serverPolls++;if(document.hidden?serverPolls%12===0:(tab==='server'||serverPolls%3===0))loadServer();},5000);
  loadServer();
  let initialTheme=window.messengerInitialTheme;
  if(!initialTheme)try{initialTheme=localStorage.getItem('messenger-theme');}catch(_){}
  $('lang-select').value=amLang;enhanceSelect($('lang-select'));enhanceSelect($('theme-select'));enhanceSelect($('project-select'));enhanceSelect($('agent-filter'));enhanceSelect($('stats-agent'));enhanceSelect($('stats-model'));enhanceSelect($('host-scope'));enhanceSelect($('host-address'));
  centreControlText();
  applyTheme(initialTheme||'system');
  matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change',()=>{if(document.documentElement.dataset.theme==='system')applyTheme('system');});
  $('theme-select').addEventListener('change',()=>applyTheme($('theme-select').value,true));
  // The language applies on reload: every text is drawn again in it (T-059).
  $('lang-select').addEventListener('change',()=>{try{localStorage.setItem('messenger-lang',$('lang-select').value);}catch(_){}location.reload();});
  const gutterObserver=new ResizeObserver(entries=>{requestAnimationFrame(()=>{
    for(const {target} of entries){
      if(!target.getClientRects().length)continue;
      const style=getComputedStyle(target);
      const gutter=Math.max(0,target.offsetWidth-target.clientWidth-parseFloat(style.borderLeftWidth)-parseFloat(style.borderRightWidth));
      const value=gutter+'px';
      if(target.style.getPropertyValue('--am-gutter')!==value)target.style.setProperty('--am-gutter',value);
    }
  });});
  root.querySelectorAll('.am-screen,.am-thread,.am-queue-list,.am-dialog-body,.am-machines').forEach(element=>gutterObserver.observe(element));
  const chatBody=root.querySelector('.am-chat-body'),chatFooter=root.querySelector('.am-chat-footer');
  $('thread').addEventListener('scroll',toBottomState,{passive:true});
  $('to-bottom').addEventListener('click',()=>{const t=$('thread');t.scrollTo({top:t.scrollHeight,behavior:'smooth'});});
  // A taller reply box must not pull the owner off the end of the feed.
  new ResizeObserver(()=>requestAnimationFrame(()=>{chatBody.style.setProperty('--am-footer-height',chatFooter.getBoundingClientRect().height+'px');const t=$('thread');if(feedPinned)t.scrollTop=t.scrollHeight;})).observe(chatFooter);
  show('tasks');loadMachineOrder();refresh();loadDiscovery();pollSetup();setInterval(()=>{if(setupPending)pollSetup();},2000);
  setInterval(updateTimers,1000);
  setInterval(()=>{if(!windowWatch||Date.now()-lastWindowRefresh>=15000)refresh();},2000);
  setInterval(loadDiscovery,60000);setInterval(()=>{if(tab==='stats'&&!document.hidden)loadStats();},15000);
  document.addEventListener('visibilitychange',()=>{if(document.hidden)return;updateTimers();loadServer();if(tab==='stats')loadStats();});
})();
