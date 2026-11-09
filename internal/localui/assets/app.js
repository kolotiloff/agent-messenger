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
    trigger.setAttribute('aria-controls',menu.id);select.after(trigger);root.append(menu);select.hidden=true;
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
  let view=null,project=recall('messenger-project'),selectedTask='',feed=null,feedSignature='',viewSignature='';
  let online=false,offlineReason='',discovery=[],tab='tasks',refreshing=false,answering=false;
  const agentChecks=new Map(),checking=new Set(),rowNote=new Map();let autoChecked=false,discoveryLoaded=false;
  let clientInfo=null,clientInfoAt=0,restart={busy:false,ask:null,error:''},autoRestarted=false;
  // The start screen covers the window until the client first answers, for
  // at most bootLimit; then the usual notices take over (T-079).
  let booting=true;const bootLimit=20000;setTimeout(()=>{booting=false;renderBoot();},bootLimit);
  function renderBoot(){
    const updating=restart.busy&&!restart.ask;
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
    const box=$('toasts');box.replaceChildren();box.hidden=!actionError;if(!actionError)return;
    const chip=el('div',undefined,'am-toast');chip.append(el('span',actionError.text,'am-toast-text'));if(actionError.hint)chip.title=actionError.hint;
    if(actionError.command){
      const command=actionError.command,copy=el('button',tr('Скопировать команду'));copy.type='button';copy.title=command;
      copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent=tr('Скопировано');}catch(_){copy.textContent=tr('Не удалось скопировать');}setTimeout(()=>copy.textContent=tr('Скопировать команду'),2000);});
      chip.append(copy);
    }
    const close=el('button','×','am-toast-close');close.type='button';close.setAttribute('aria-label',tr('Закрыть'));close.addEventListener('click',()=>showError(null));chip.append(close);
    box.append(chip);
  }
  async function api(path,timeout=4500){
    if(!token)throw Error(tr('Откройте приватную ссылку, которую выдала команда ui.'));
    const response=await fetch(path,{headers:{Authorization:'Bearer '+token},cache:'no-store',signal:AbortSignal.timeout(timeout)});
    if(!response.ok){
      if(response.status===401)throw Error(tr('Ссылка доступа устарела. Откройте окно заново.'));
      const body=await response.text().catch(()=>'');
      if(body.startsWith('client_outdated'))throw outdatedMessage();
      if(body.startsWith('client_rejected'))throw Error(tr('Клиент Messenger отклонил запрос окна. Подробности: messenger doctor.'));
      throw Error(tr('Клиент Messenger не запущен или не отвечает. Данные не обновляются.'));
    }
    return response.json();
  }
  async function postAPI(path,body,timeout=6000){
    let response;
    // No answer means the result is unknown, not that nothing happened.
    try{response=await fetch(path,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(timeout)});}
    catch(_){throw Error(tr('Ответ не получен, результат неизвестен. Проверьте состояние, прежде чем повторять.'));}
    let result={};try{result=await response.json();}catch(_){}
    if(response.ok&&result.ok===true)return result;
    if(result.state==='client_outdated')throw outdatedMessage();
    if(response.status===409&&result.detail)throw Error(result.detail);
    if(response.status===400)throw Error(tr('Проверьте введённые данные.'));
    throw Error(tr('Действие не подтверждено. Проверьте состояние и повторите.'));
  }
  function minutes(from,to){
    const a=Date.parse(from),b=to?Date.parse(to):Date.now();
    if(isNaN(a)||isNaN(b))return '';
    const m=Math.max(0,Math.round((b-a)/60000));
    if(m<1)return tr('<1 мин');
    if(m<60)return m+tr(' мин');
    const h=Math.floor(m/60);return h+tr(' ч')+(m%60?' '+m%60+tr(' мин'):'');
  }
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
  // When a task was given and when it ended, with the date unless today.
  function taskWhen(t){
    const from=dayClock(t.created_at);if(!from)return '';
    if(!t.finished_at)return tr('дана ')+from;
    const end=new Date(t.finished_at).toDateString()===new Date(t.created_at).toDateString()?clock(t.finished_at):dayClock(t.finished_at);
    return from+'–'+end;
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
  function taskStatus(t){
    const end=t.finished_at||t.updated_at,from=t.started_at||t.created_at;
    switch(t.state){
      case 'running':return t.pending_launch?[t.waits_for_lock?tr('Ждёт ресурс «')+t.waits_for_lock+'»':tr('Ждёт запуска'),'am-status-wait']:[tr('Работает ')+minutes(from),'am-status-run'];
      case 'waiting_owner':return [tr('Ждёт вашего ответа'),'am-status-wait'];
      case 'waiting_coordinator':return [tr('Ждёт ответа координатора'),'am-status-run'];
      case 'done':return [tr('Готово за ')+minutes(from,end),'am-muted'];
      case 'failed':return failureStatus(t)||[tr('Ошибка через ')+minutes(from,end),'am-status-bad'];
      case 'interrupted':return [tr('Прервана через ')+minutes(from,end),'am-status-bad'];
      case 'blocked':return [tr('Заблокирована через ')+minutes(from,end),'am-status-bad'];
      case 'cancelled':return [tr('Отменена'),'am-muted'];
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
    if(!online)return ['offline',tr('Нет связи')];
    if(a.waiting_owner>0)return ['wait',tr('Ждёт ответа')];
    if(a.running>0)return ['run',a.running>1?tr('Работает (')+a.running+')':tr('Работает')];
    return ['idle',tr('Свободен')];
  }
  function renderProjects(){
    const select=$('project-select');select.replaceChildren();
    for(const p of view?.projects||[]){const o=el('option',p.title||p.name);o.value=p.name;select.append(o);}
    const add=el('option',tr('Новый проект'));add.value='__new';select.append(add);
    select.value=view?.project||'__new';
    if(!view?.project&&select.options.length>1)select.selectedIndex=0;
    select.disabled=!view;syncSelects();
  }
  // Right-click menu on another computer of the network. Revoking cannot be
  // undone, so its item only opens a confirmation inside the card; updating
  // (T-078) asks that computer to install the newest signed update.
  function deviceMenu(d,x,y,can){
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
    const add=(text,pick)=>{
      const item=el('div',text,'am-select-option');item.setAttribute('role','menuitem');item.tabIndex=0;
      const run=()=>{menu.hidePopover();pick();};
      item.addEventListener('click',run);item.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();run();}});
      items.push(item);
    };
    if(can.update)add(tr('Обновить программу'),()=>requestUpdate(d));
    if(can.revoke)add(tr('Отозвать устройство'),()=>{revokeAsk=d.id;clearTimeout(revokeTimer);revokeTimer=setTimeout(()=>{revokeAsk='';renderMachines();},8000);renderMachines();$('machines').querySelector('[data-key="'+CSS.escape('revoke-yes-'+d.id)+'"]')?.focus();});
    if(!items.length)return;
    menu.replaceChildren(...items);menu.style.left=x+'px';menu.style.top=y+'px';menu.showPopover();items[0].focus();
  }
  // Asking another computer to update itself (T-078). Its answers arrive
  // with the server status; a refused request shows on the card for a while.
  let updateErrors={};
  async function requestUpdate(peer){
    delete updateErrors[peer.id];
    try{const answer=await postAPI('/api/server-action',{action:'update',device:peer.id},28000);if(answer?.result)server=answer.result;}
    catch(error){const id=peer.id;updateErrors[id]=error.message;setTimeout(()=>{if(updateErrors[id]===error.message){delete updateErrors[id];renderMachines();}},10000);}
    renderMachines();
  }
  // The card's update line: the last request to that computer, while recent.
  function updateLine(peer){
    if(updateErrors[peer.id])return [tr('Не обновлено: ')+updateErrors[peer.id],'am-status-bad'];
    const u=(server?.updates||[]).find(x=>x.computer===peer.id);
    if(!u)return null;
    const at=Date.parse(u.updated_at);
    if(!(Date.now()-at<24*3600*1000))return null;
    if(u.stale)return [tr('Нет ответа об обновлении'),'am-status-bad'];
    switch(u.state){
      case 'waiting':return [tr('Обновление запрошено…'),'am-status-wait'];
      case 'updating':return [tr('Обновляется…'),'am-status-run'];
      case 'done':return u.text?[tr('Версия ')+versionName(u.version||'')+': '+u.text,'am-muted']:[tr('Обновлено до ')+versionName(u.version||''),'am-muted'];
      case 'refused':case 'failed':return [tr('Не обновлено: ')+(u.text||u.state),'am-status-bad'];
    }
    return null;
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
      const [state,text]=!live?['offline',tr('Нет связи')]:a?agentState(a):['idle',tr('Чат с инструментами Messenger')];
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
    card.title=computerFilter===id?tr('Показать все компьютеры'):tr('Показать только этот компьютер');
    card.addEventListener('click',event=>{if(!event.target.closest('button'))filterComputer(id);});
    card.addEventListener('keydown',event=>{if(event.target===card&&(event.key==='Enter'||event.key===' ')){event.preventDefault();filterComputer(id);}});
  }
  function renderMachines(){
    const box=$('machines');box.replaceChildren();
    const machine=el('div',undefined,'am-machine-card');machine.setAttribute('aria-current','true');pickable(machine,'local');
    const head=el('div',undefined,'am-machine-name');head.append(el('span',view?.computer_name||tr('Этот компьютер')));
    const dot=el('span',undefined,'am-dot'+(online?' online':''));dot.title=online?tr('На связи'):tr('Нет связи');head.append(dot);machine.append(head);
    const lines=agentLines(view?.agents||[],view?.coordinator?.connected?(view.coordinator.client||''):null,online);
    if(!lines.children.length)lines.append(el('p',online?tr('Агенты не подключены'):tr('Нет связи'),'am-small am-muted'));
    machine.append(lines);box.append(machine);
    // The other computers of this computer's network, with their agents.
    // Only the computer that runs the server can revoke one, from the
    // right-click menu.
    const hosted=new Set((server?.host?.devices||[]).filter(d=>!d.revoked_at).map(d=>d.id));
    for(const peer of server?.peers||[]){
      const fresh=peerFresh(peer);
      const card=el('div',undefined,'am-machine-card am-peer-card');card.dataset.peer=peer.id;pickable(card,peer.id);
      const name=el('div',undefined,'am-machine-name');
      const pdot=el('span',undefined,'am-dot'+(fresh?' online':''));pdot.title=fresh?tr('На связи'):tr('Нет связи');
      name.append(el('span',peer.name));
      if(peer.version)name.append(el('span',versionName(peer.version),'am-small am-muted am-machine-version'));
      name.append(pdot);card.append(name);
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
        const plines=agentLines(peer.agents||[],peer.coordinator?peer.coordinator:null,true);
        if(!plines.children.length)plines.append(el('p',tr('Агенты не подключены'),'am-small am-muted'));
        card.append(plines);
      }else{
        card.append(el('p',peer.last_seen?tr('Был на связи ')+stamp(peer.last_seen):tr('Ещё не выходил на связь'),'am-small am-muted'));
      }
      const status=updateLine(peer);
      if(status)card.append(el('p',status[0],'am-small am-update-line '+status[1]));
      const can={revoke:hosted.has(peer.id),update:fresh&&!!peer.version};
      if(can.revoke||can.update){
        card.classList.add('am-device-card');card.tabIndex=0;card.dataset.key='revoke-'+peer.id;card.title=tr('Правая кнопка мыши — действия');
        card.addEventListener('contextmenu',event=>{event.preventDefault();deviceMenu(peer,event.clientX,event.clientY,can);});
        card.addEventListener('keydown',event=>{if(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')){event.preventDefault();const r=card.getBoundingClientRect();deviceMenu(peer,r.left+16,r.top+24,can);}});
      }
      box.append(card);
    }
  }
  // The computer that runs a task: this one, or the other one it was sent to.
  function taskComputer(t){return t.remote&&!t.remote.from?t.remote.id:'local';}
  // A click on a computer in the sidebar shows only its tasks; a second click,
  // the chip's cross or a click on empty sidebar space shows all again (T-056).
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
    renderMachines();renderTaskList();
    if(tab==='stats'){stats=null;renderStats();loadStats();}
    if(tab==='agents')renderAgents();
  }
  function computerName(id){
    return id==='local'?(view?.computer_name||tr('Этот компьютер')):(server?.peers||[]).find(p=>p.id===id)?.name||(view?.tasks||[]).find(t=>taskComputer(t)===id)?.remote?.name||'';
  }
  // The chip of a tab filtered by computer; its cross shows all again.
  function computerChip(chip){chip.hidden=!computerFilter;chip.querySelector('span').textContent=computerFilter?tr('Компьютер: ')+computerName(computerFilter):'';}
  function visibleTasks(){
    const filter=$('agent-filter').value;
    const list=(view?.tasks||[]).filter(t=>(!filter||t.agent===filter)&&(!computerFilter||taskComputer(t)===computerFilter));
    const active=list.filter(t=>activeStates.includes(t.state)),done=list.filter(t=>!activeStates.includes(t.state));
    return [active,done];
  }
  function renderTaskList(){
    const box=$('task-list');box.replaceChildren();
    const [active,done]=visibleTasks();
    computerChip($('computer-chip'));
    // T-096: say when the agent filter hides active tasks, with a way back.
    const agentChosen=$('agent-filter').value;
    const hidden=agentChosen?(view?.tasks||[]).filter(t=>t.agent!==agentChosen&&activeStates.includes(t.state)&&(!computerFilter||taskComputer(t)===computerFilter)).length:0;
    if(hidden){
      const all=el('button',undefined,'am-task am-task-hidden');all.type='button';
      all.append(el('span',tr('Фильтр «')+(names[agentChosen]||agentChosen)+tr('» скрывает активные задачи: ')+hidden,'am-task-agent'),el('span',tr('Показать всех агентов'),'am-status-run'));
      all.addEventListener('click',()=>{$('agent-filter').value='';syncSelects();renderTaskList();});
      box.append(all);
    }
    $('task-count').textContent=view?.tasks?.length?String(view.tasks.length):'';
    const section=(label,items)=>{
      if(!items.length)return;
      box.append(el('div',label,'am-label am-task-section'));
      for(const t of items){
        const row=el('button',undefined,'am-task');row.type='button';row.setAttribute('aria-pressed',String(t.id===selectedTask));
        const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),el('span',(names[t.agent]||t.agent)+(t.remote&&!t.remote.from?tr(' на ')+t.remote.name:'')));
        if(t.remote)agent.title=t.remote.from?tr('Задачу дал компьютер «')+t.remote.name+'»':tr('Выполняется на компьютере «')+t.remote.name+'»';
        const when=taskWhen(t);if(when){const w=el('span',when,'am-task-when');w.title=tr('Когда задача дана')+(t.finished_at?tr(' и когда завершилась'):'');agent.append(w);}
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
    section(tr('Активные'),active);section(tr('Завершённые'),done);
    // Other computers' tasks of the project, read-only (T-047).
    const agentFilter=$('agent-filter').value;
    for(const p of view?.shared||[]){
      if(computerFilter&&computerFilter!==p.peer)continue;
      const items=p.tasks.filter(t=>!agentFilter||t.agent===agentFilter);
      if(!items.length)continue;
      box.append(el('div',tr('На компьютере «')+p.name+'»','am-label am-task-section'));
      for(const t of items){
        const row=el('div',undefined,'am-task am-task-shared');row.title=tr('Задачу ведёт компьютер «')+p.name+tr('», здесь её можно только посмотреть');
        const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),el('span',names[t.agent]||t.agent));
        const sharedWhen=taskWhen(t);if(sharedWhen)agent.append(el('span',sharedWhen,'am-task-when'));
        const [text,cls]=taskStatus(t);
        const words=el('span',text);
        if(t.tokens)words.append(el('span',', '+tokensText(t.tokens)+tr(' токенов'),'am-task-usage'));
        const status=el('span',undefined,'am-task-status '+cls);status.append(words);
        row.append(agent,el('span',t.title,'am-task-title'),status);
        if(t.report)row.append(el('span',t.report,'am-task-report'));
        box.append(row);
      }
    }
    if(!box.children.length)box.append(el('p',view?.project?(computerFilter||$('agent-filter').value?tr('Здесь задач нет'):tr('Задач пока нет')):'','am-small am-muted'));
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
      box.append(el('p',tr('Подключаем Claude к проекту…'),'am-connect-title'),el('p',tr('Messenger добавляет этот проект в настройки Claude. Это займёт несколько секунд.')));
    }else if(link.state==='connected'){
      box.append(el('p',tr('Claude подключён к проекту'),'am-connect-title'));
      box.append(el('p',tr('Откройте папку проекта в Claude (вкладка Code, этот компьютер) и начните новый чат. Напишите в нём, что нужно сделать. Чтобы проверить подключение, спросите: «Каких агентов ты видишь?»')));
      box.append(el('p',tr('Чат, открытый до подключения, проект не увидит: нужен новый.'),'am-small am-muted'));
    }else{
      box.append(el('p',tr('Подключите Claude к проекту'),'am-connect-title'));
      if(failure){box.append(el('p',tr('Не получилось подключить: ')+failure.detail),connect(tr('Попробовать ещё раз')));}
      else if(link.state==='no_claude'){box.append(el('p',tr('Сначала подключите Claude Code во вкладке «Агенты», затем нажмите кнопку ниже.')),connect(tr('Подключить к Claude')));}
      else if(link.state==='other'){box.append(el('p',tr('В настройках Claude для этой папки уже есть другое подключение с именем «messenger». Messenger его не меняет. Если оно больше не нужно, удалите его командой «claude mcp remove messenger» в этой папке и нажмите кнопку ниже.')),connect(tr('Подключить к Claude')));}
      else if(link.state==='unknown'){box.append(el('p',tr('Не удалось прочитать настройки Claude')+(link.detail?': '+link.detail:'.')),connect(tr('Подключить к Claude')));}
      else if(link.state==='no_folder'){box.append(el('p',tr('У проекта нет папки на этом компьютере. Укажите её в настройках проекта.')));}
      else{box.append(el('p',tr('Задачи проекта даёт ваш чат в Claude. Messenger сам добавит этот проект в настройки Claude.')),connect(tr('Подключить к Claude')));}
    }
    const manual=el('details',undefined,'am-tool');manual.append(el('summary',tr('Подключить вручную через Терминал')));
    const command='cd '+shellQuote(view.folder)+' && claude mcp add --scope local messenger -- '+shellQuote(view.messenger)+' --root '+shellQuote(view.data_root)+' coordinator --project '+shellQuote(view.project);
    const copy=el('button',tr('Скопировать команду'));copy.type='button';
    copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent=tr('Скопировано');}catch(_){copy.textContent=tr('Скопируйте вручную');}setTimeout(()=>copy.textContent=tr('Скопировать команду'),2000);});
    manual.append(el('pre',command,'am-command'),copy);
    box.append(manual,el('p',tr('Чат-координатор видит только задачи этого проекта.'),'am-small am-muted'));
    return g;
  }
  function renderThread(){
    const thread=$('thread'),head=$('task-head');
    $('compose').hidden=true;feedSignature='';
    if(!view){head.replaceChildren();return;}
    if(!view.project){
      head.replaceChildren();
      const box=el('div',undefined,'am-settings am-connect');box.append(el('h3',tr('Создайте проект')),el('p',tr('Проект связывает чат-координатора, папку проекта на этом компьютере и задачи.')));
      const b=el('button',tr('Новый проект'),'am-primary');b.type='button';b.addEventListener('click',()=>openProjectDialog(false));box.append(b);
      thread.replaceChildren(box);return;
    }
    if(!view.coordinator.connected&&!view.tasks.length){head.replaceChildren();thread.replaceChildren(connectHelp());return;}
    if(!selectedTask||!feed||feed.task.id!==selectedTask){
      head.replaceChildren();
      if(!selectedTask)thread.replaceChildren(el('p',view.tasks.length?tr('Выберите задачу в списке справа.'):tr('Задач пока нет. Их создаёт чат-координатор.'),'am-muted am-empty'));
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
  const toolNames={Bash:tr('Команда'),Read:tr('Чтение'),Edit:tr('Правка'),MultiEdit:tr('Правка'),Write:tr('Запись'),Grep:tr('Поиск'),Glob:tr('Поиск файлов'),WebFetch:tr('Загрузка страницы'),WebSearch:tr('Поиск в интернете'),Task:tr('Подзадача'),TodoWrite:tr('План')};
  // The command itself, without the shell that runs it.
  function commandText(line){
    const m=line.match(/^"?[^"]*(?:powershell|pwsh)(?:\.exe)?"?\s+-Command\s+([\s\S]*)$/i)||line.match(/^"?\S*\/(?:ba|z)?sh"?\s+-l?c\s+([\s\S]*)$/);
    if(!m)return line;
    const c=m[1].trim(),q=c[0];
    return (q==="'"||q==='"')&&c.endsWith(q)&&c.length>1?c.slice(1,-1):c;
  }
  function toolName(it){return toolNames[it.title]||it.title||tr('Действие');}
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
    const where=t.remote&&!t.remote.from?tr(' на компьютере «')+t.remote.name+'»':tr(' на ')+(view.computer_name||tr('этом компьютере'))+(t.remote?.from?tr(', по заданию компьютера «')+t.remote.name+'»':'');
    title.append(el('p',(names[t.agent]||t.agent)+(t.model?' ('+t.model+')':'')+where+tr(', папка ')+(t.folder||tr('ещё не известна')),'am-small am-muted'));
    const [statusText,statusCls]=taskStatus(t);
    const when=t.finished_at?', '+clock(t.created_at)+'–'+clock(t.finished_at):tr(', начата в ')+clock(t.created_at);
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
      tools.sum.textContent=tr('Действий: ')+tools.count+' — '+[...tools.names].join(', ');
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
      meta.append(el('strong',names[t.agent]||t.agent),roleTag(tr('исполнитель')),el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');meta.prepend(avatar);body.append(meta,content);row.append(body);g.append(row);out.push(g);group=content;return content;
    };
    const delivery=(text,cls)=>el('div',text,'am-delivery '+cls);
    for(const [index,it] of feed.items.entries()){
      if(it.kind!=='tool')tools=null;
      if(it.kind==='task'){outgoing(coordLabel,coordProvider,tr('координатор'),it.text,it.at);continue;}
      if(it.kind==='note'){outgoing(tr('Вы'),'owner','',it.text,it.at);const c=out[out.length-1].querySelector('.am-message-content');c.append(delivery(...({pending:[tr('Исполнитель прочитает, когда закончит текущий шаг'),'am-status-wait'],delivered:[tr('Передано исполнителю'),'am-status-good'],undelivered:[tr('Не передано: задача остановилась'),'am-status-bad']}[it.state]||[it.state,'am-muted'])));continue;}
      if(it.kind==='answer'){if(it.author==='owner')outgoing(tr('Вы'),'owner','',it.text,it.at);else outgoing(coordLabel,coordProvider,tr('координатор'),it.text,it.at);continue;}
      if(it.kind==='accepted'){const c=out[out.length-1]?.querySelector('.am-message-content');if(c)c.append(delivery(tr('Принято на компьютере «')+it.text+'»','am-status-good'));group=null;continue;}
      if(it.kind==='text'){workerGroup(it.at).append(richText(it.text));continue;}
      if(it.kind==='tool'){addTool(workerGroup(it.at),it,index);continue;}
      if(it.kind==='report'||it.kind==='question'){
        const c=workerGroup(it.at);c.append(richText(it.text,'am-disposition'));
        if(!it.verified)c.append(delivery(tr('Не опубликовано: запуск не прошёл проверку'),'am-status-bad'));
        else if(it.kind==='report'&&it.audience==='held')c.append(delivery(tr('Промежуточный отчёт: исполнитель продолжает с вашим сообщением'),'am-status-wait'));
        else if(it.kind==='report')c.append(delivery(tr('Отчёт отправлен координатору, сессия закрыта'),'am-status-good'));
        else if(it.audience==='owner')c.append(delivery(tr('Вопрос вам, координатор его не видит'),'am-status-wait'));
        else c.append(delivery(tr('Вопрос координатору'),'am-status-wait'));
        group=null;continue;
      }
      if(it.kind==='outcome'){
        group=null;const text={failed:t.remote&&!t.remote.from?tr('Задача не выполнена'):tr('Запуск завершился с ошибкой'),blocked:t.remote&&!t.remote.from?tr('Задача заблокирована: результат на другом компьютере неизвестен'):tr('Задача заблокирована: процесс не завершился полностью'),interrupted:tr('Клиент перезапустился во время работы; результат неизвестен'),cancelled:tr('Задача отменена')}[it.state]||it.state;
        const note=el('div',undefined,'am-outcome '+(it.state==='cancelled'?'am-muted':'am-status-bad'));note.append(el('p',text));
        const why=t.remote&&!t.remote.from?refusalText(it.text,t.remote.name)||failureText(it.text):failureText(it.text);if(why)note.append(el('p',why));
        if(it.text)note.append(el('p',it.text,'am-small am-muted'));
        const g=el('article',undefined,'am-message-group');g.append(note);out.push(g);
      }
    }
    thread.replaceChildren(...out);
    for(const d of thread.querySelectorAll('details[data-key]'))if(opened.has(d.dataset.key))d.open=true;
    // A task another computer started is answered there.
    const q=t.remote?.from?null:t.owner_question;
    const active=activeStates.includes(t.state),box=$('answer-text');
    const note=canNote(t),wasNote=$('compose').dataset.mode==='note';
    $('compose').hidden=!q&&!active;
    box.disabled=!q&&!note;box.placeholder=q?tr('Ответить исполнителю…'):note?tr('Написать исполнителю…'):t.remote?.from&&t.owner_question?tr('Отвечают на компьютере «')+t.remote.name+'»':t.pending_launch?tr('Задача ждёт запуска…'):tr('Исполнитель работает…');
    $('answer-send').hidden=!q&&!note;$('answer-hint').hidden=!q&&!note;$('task-stop').hidden=!active;
    $('answer-hint').textContent=q?tr('Enter отправляет, Shift+Enter переносит строку. Ответ продолжит сессию исполнителя.'):tr('Enter отправляет, Shift+Enter переносит строку. Исполнитель прочитает, когда закончит текущий шаг.');
    $('compose').dataset.mode=q?'answer':note?'note':'';
    if(q&&$('compose').dataset.question!==q.id){$('compose').dataset.question=q.id;$('answer-text').value='';$('answer-status').textContent='';}
    else if(!q&&!note&&wasNote){$('answer-text').value='';}
    sizeAnswer();answerControls();
    if(atBottom)thread.scrollTop=thread.scrollHeight;
  }
  // Why a run was not accepted (T-091), in the window's language.
  function failureText(text){
    text=text||'';const code=text.match(/stopped with an error \(exit code (\d+)\)/);
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
    if(/does not run tasks of other computers/.test(text||''))return tr('Компьютер «')+name+tr('» не принимает задачи других компьютеров. Включите на нём «Задачи других компьютеров» на вкладке «Сервер» и дайте задачу снова.');
    if(/has no project/.test(text||''))return tr('На компьютере «')+name+tr('» нет проекта с таким названием. Создайте его там и выберите папку.');
    if(/has no folder for the project|project folder is missing/.test(text||''))return tr('На компьютере «')+name+tr('» у проекта не выбрана папка или её нет. Выберите папку проекта там и дайте задачу снова.');
    if(/is not set up on this computer/.test(text||''))return tr('На компьютере «')+name+tr('» этот агент не настроен. Настройте его там на вкладке агентов.');
    return '';
  }
  // The reply box is one line and grows only while its text does not fit;
  // the stop and send buttons stay at its bottom.
  function sizeAnswer(){const f=$('answer-text');f.style.height='';if(f.value)f.style.height=Math.min(140,f.scrollHeight+(f.offsetHeight-f.clientHeight))+'px';}
  // T-085: the owner may write to a working worker of this computer; the
  // message waits until the worker's current step ends.
  function canNote(t){return !!t&&!t.remote&&!t.owner_question&&(t.state==='running'||t.state==='waiting_coordinator');}
  function answerControls(){const t=feed?.task;$('answer-send').disabled=answering||!$('answer-text').value.trim()||!(t?.owner_question&&!t.remote?.from||canNote(t));}
  async function sendNote(){
    const t=feed?.task;if(!canNote(t)||answering)return;
    const text=$('answer-text').value.trim();if(!text)return;
    const key='messenger-note-op:'+t.id+':'+text;
    let op=recall(key);
    if(!op){op='window-'+t.id+'-'+crypto.randomUUID();remember(key,op);}
    answering=true;answerControls();$('answer-status').textContent=tr('Отправляем сообщение…');
    try{await postAPI('/api/task-note',{task:t.id,op_id:op,text});showError(null);try{localStorage.removeItem(key);}catch(_){}$('answer-status').textContent='';$('answer-text').value='';sizeAnswer();feedSignature='';await refresh();}
    catch(error){$('answer-status').textContent='';showError({message:error.message+tr(' Повтор с тем же текстом безопасен.'),command:error.command,hint:error.hint});}
    finally{answering=false;answerControls();}
  }
  async function sendAnswer(){
    if(!feed?.task?.owner_question&&canNote(feed?.task))return sendNote();
    const q=feed?.task?.owner_question;if(!q||answering||feed.task.remote?.from)return;
    const text=$('answer-text').value.trim();if(!text)return;
    let op=recall('messenger-answer-op:'+q.id);
    if(!op){op='window-'+q.id+'-'+crypto.randomUUID();remember('messenger-answer-op:'+q.id,op);}
    answering=true;answerControls();$('answer-status').textContent=tr('Отправляем ответ…');
    try{await postAPI('/api/task-answer',{task:feed.task.id,question:q.id,op_id:op,text});showError(null);$('answer-status').textContent=tr('Ответ записан. Исполнитель продолжит в той же сессии.');$('answer-text').value='';sizeAnswer();feedSignature='';await refresh();}
    catch(error){$('answer-status').textContent='';showError({message:error.message+tr(' Повтор с тем же текстом безопасен.'),command:error.command,hint:error.hint});}
    finally{answering=false;answerControls();}
  }
  // Agent cards (T-061, D-070): the agent's name with a check, settings and
  // an on/off switch; four steps of the same height (program, sign-in, work
  // with Messenger, limit) with a mark on the left and one action on the
  // right; one «Задачи» button under them. Steps that cannot be checked yet
  // are grey and numbered.
  const checkPath=['M21 12a9 9 0 1 1-3-6.7L21 8','M21 3v5h-5'];
  const gearPath=['M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z'];
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
    askConfirm(tr('Удалить ')+names[provider]+' CLI?',tr('Откроется официальная страница агента. Удалите его командой оттуда. Настройки Messenger сохранятся.'),tr('Открыть'),async()=>{
      await postAPI('/api/agent-install',{provider});
    });
  }
  function renderAgents(){
    const cards=$('agents');cards.replaceChildren();
    computerChip($('agents-computer-chip'));
    if(computerFilter&&computerFilter!=='local'){renderPeerAgents(cards);return;}
    for(const provider of ['claude','codex','cursor']){
      const a=(view?.agents||[]).find(x=>x.agent===provider);
      const found=discovery.find(d=>d.provider===provider);
      const cli=a?.cli||found?.cli||'',profile=a?.environment||found?.environment||'';
      const missing=!a&&found?.state==='not_found';
      const on=!!a&&!a.paused;
      const card=el('article',undefined,'am-card am-agent-card'+(on||!cli?'':' am-agent-off'));
      const head=el('div',undefined,'am-agent-head');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(provider),el('span',names[provider]));
      const sw=el('button',undefined,'am-switch');sw.type='button';sw.setAttribute('role','switch');sw.setAttribute('aria-checked',String(on));
      sw.setAttribute('aria-label',names[provider]+tr(': Messenger даёт задачи'));
      sw.title=on?tr('Messenger даёт задачи ')+names[provider]+tr(' на этом компьютере'):cli?tr('Включить: Messenger будет давать задачи ')+names[provider]:tr('Сначала установите ')+names[provider]+' CLI';
      sw.disabled=!a&&!cli;
      sw.addEventListener('click',async()=>{sw.disabled=true;try{await setAgentOn(provider,!on);}finally{sw.disabled=false;}});
      head.append(heading,iconButton(tr('Проверить снова'),checkPath,false,()=>{if(cli)checkCard(provider);else loadDiscovery();}),iconButton(tr('Настройки'),gearPath,true,()=>openAgentSetup(provider)),sw);
      card.append(head);
      const steps=el('div',undefined,'am-steps');
      // mark: 'ok', 'no', a step number, or a node (the limit ring).
      const step=(mark,title,sub,action,tip)=>{
        const row=el('div',undefined,'am-step');
        if(mark instanceof Node){const m=el('span',undefined,'am-mark am-mark-ring');m.append(mark);row.append(m);}
        else row.append(el('span',mark==='ok'?'✓':mark==='no'?'✕':String(mark),'am-mark '+(mark==='ok'?'am-mark-ok':mark==='no'?'am-mark-no':'am-mark-off')));
        const text=el('div',undefined,'am-step-text'+(typeof mark==='number'?' am-step-later':''));
        const line=el('p',undefined,'am-step-title');
        if(title instanceof Node)line.append(title);else line.textContent=title;
        if(tip){const q=el('span','?','am-q');q.title=tip;q.setAttribute('aria-label',tip);q.tabIndex=0;line.append(q);}
        text.append(line);
        if(sub){const s=el('p',sub,'am-step-sub');s.title=sub;text.append(s);}
        row.append(text);
        if(action){const b=el('button',action[0]);b.type='button';b.addEventListener('click',action[1]);row.append(b);}else row.append(el('span'));
        steps.append(row);
      };
      const check=agentChecks.get(provider),auth=check?.authentication?.state;
      const busy=checking.has(provider);
      // 1. The program.
      const programNote=rowNote.get(provider+':program');
      if(programNote)step(1,names[provider]+' CLI',programNote);
      else if(missing)step('no',names[provider]+tr(' CLI не установлен'),tr('Не найден на этом компьютере'),[tr('Установить'),()=>installAgent(provider)]);
      else if(cli)step('ok',names[provider]+tr(' CLI установлен'),busy?tr('Проверяем'):check?.version?.value||'',[tr('Удалить'),()=>uninstallAgent(provider)]);
      else step(1,names[provider]+' CLI',tr('Проверяем'));
      // 2. The sign-in. No confirmed login offers «Войти» (T-056).
      const authNote=rowNote.get(provider+':auth');
      if(!cli)step(2,tr('Вход в аккаунт'),tr('После установки'));
      else if(authNote)step(2,tr('Вход в аккаунт'),authNote);
      else if(busy)step(2,tr('Вход в аккаунт'),tr('Проверяем'));
      else if(auth==='authenticated'){
        const method=check.authentication.value==='chatgpt'?tr('Аккаунт ChatGPT'):check.authentication.value==='api_key'?tr('Ключ API'):'';
        step('ok',tr('Вход выполнен'),method,[tr('Выйти'),()=>logoutFromCard(provider,cli,profile)]);
      }
      else if(auth==='not_authenticated')step('no',tr('Вход не выполнен'),tr('Войдите в аккаунт агента'),[tr('Войти'),()=>loginFromCard(provider,cli,profile)]);
      else step(2,tr('Вход в аккаунт'),check?tr('Не удалось проверить'):tr('Ещё не проверен'),[tr('Войти'),()=>loginFromCard(provider,cli,profile)]);
      // 3. Work with Messenger: set only by what a task showed (D-070).
      const trust=a?.trust?.state;
      if(a&&trust&&trust!=='trusted')step('no',tr('Не доверяет папке проекта'),trust==='not_trusted'?tr('Откройте папку в агенте и подтвердите доверие'):tr('Не удалось определить'),null,a.trust?.hint||tr('Откройте папку проекта в этой программе и подтвердите доверие.'));
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
    renderAccessBar();
  }
  // Another computer's agents, as its reports show them: view only (T-076).
  function renderPeerAgents(cards){
    $('access-bar').hidden=true;
    const peer=(server?.peers||[]).find(p=>p.id===computerFilter);
    const fresh=!!peer&&peerFresh(peer);
    cards.append(el('p',!peer?tr('Компьютер не найден в сети.'):fresh?tr('Только просмотр: агентов этого компьютера настраивают на нём самом.'):(peer.last_seen?tr('Нет связи. Был на связи ')+stamp(peer.last_seen)+'.':tr('Ещё не выходил на связь.')),'am-small am-muted am-peer-agents-note'));
    for(const a of peer?.agents||[]){
      const card=el('article',undefined,'am-card am-agent-card am-agent-remote'+(fresh?'':' am-agent-off'));
      const head=el('div',undefined,'am-agent-head');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(a.agent),el('span',names[a.agent]||a.agent));
      const [state,text]=fresh?agentState(a):['offline',tr('Нет связи')];
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
      const n=(a.running||0)+(a.waiting_owner||0);
      const tasks=el('button',undefined,'am-agent-tasks');tasks.type='button';
      // T-083: what runs there, as on this computer's cards.
      if(n){const label=el('span');label.append(tr('Задачи: '));
        if(a.running)label.append(el('span',a.running+tr(' идёт'),'am-status-run'));
        if(a.running&&a.waiting_owner)label.append(', ');
        if(a.waiting_owner)label.append(el('span',a.waiting_owner+tr(' ждёт ответа'),'am-status-wait'));
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
    try{
      const info=await api('/api/client-info');clientInfo=info;if(!info.outdated)restart.ask=null;
      // T-077: after an update the window restarts the old client itself,
      // once. The client refuses while tasks run or their count is unknown;
      // only then the owner is asked.
      if(info.outdated&&info.restart&&info.client==='running'&&!autoRestarted&&!restart.busy&&!restart.ask){autoRestarted=true;restartClient(false);return;}
      renderRestart();
    }catch(_){}
  }
  async function restartClient(confirm){
    restart={busy:true,ask:restart.ask,error:''};renderRestart();
    let response,result={};
    try{response=await fetch('/api/restart-client',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({project,confirm}),cache:'no-store',signal:AbortSignal.timeout(50000)});result=await response.json().catch(()=>({}));}
    catch(_){restart={busy:false,ask:null,error:tr('Ответ не получен, результат неизвестен. Подождите немного: окно проверит клиент само.')};renderRestart();loadClientInfo();return;}
    if(response.ok&&result.ok){restart={busy:false,ask:null,error:''};showError(null);await loadClientInfo();viewSignature='';refresh();return;}
    if(result.state==='confirm'){restart={busy:false,ask:{running:result.running||0,known:result.known!==false},error:''};renderRestart();return;}
    restart={busy:false,ask:null,error:result.detail||tr('Перезапуск не подтверждён. Проверьте состояние.')};renderRestart();loadClientInfo();
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
    if(ask){title=ask.known?tr('Сейчас выполняются задачи'):tr('Не удалось проверить задачи');text=(ask.known?tr('Задач в работе: ')+ask.running+tr('. Перезапуск прервёт их.'):tr('Если задачи сейчас идут, перезапуск прервёт их.'))+tr(' Сами они не повторятся и будут ждать вашего решения.');}
    else if(restart.busy){title=tr('Обновляем Messenger');text=tr('Останавливаем старую версию и запускаем новую. Это займёт несколько секунд.');}
    else if(canRestart)text=tr('Приложение обновилось до версии ')+clientInfo.window_version+tr(', а в фоне работает ')+(clientInfo.client_version?tr('версия ')+clientInfo.client_version:tr('более ранняя версия'))+tr('. Пока она не перезапущена, Messenger не может работать. Задачи, история и устройства сохранятся.');
    else text=tr('В фоне работает более ранняя версия клиента. Закройте окно, выполните скопированную команду в Терминале и откройте окно снова.');
    $('restart-title').textContent=title;$('restart-text').textContent=text;
    $('restart-error').textContent=restart.error;$('restart-error').hidden=!restart.error;
    const go=$('restart-go'),quit=$('restart-quit'),copy=$('restart-copy');
    go.hidden=!canRestart||!answering||restart.busy;
    go.textContent=ask?tr('Перезапустить всё равно'):tr('Перезапустить');
    go.classList.toggle('am-primary',!ask);go.classList.toggle('am-danger',!!ask);
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
      catch(error){if(Date.now()-clientInfoAt>5000)loadClientInfo();online=false;offlineReason=error.name==='TimeoutError'||error.name==='TypeError'?tr('Окно потеряло связь с программой Messenger. Закройте окно и откройте снова.'):error.message;renderMachines();renderNotice();return;}
      online=true;offlineReason='';if(Date.now()-clientInfoAt>10000)loadClientInfo();
      // The first answer: learn the client's version before the start screen
      // goes, so an update starts without the old window showing.
      if(booting){await loadClientInfo();booting=false;renderBoot();}
      if(!v.projects.some(p=>p.name===project))project=v.project||'';
      const signature=JSON.stringify(v);
      if(signature!==viewSignature){viewSignature=signature;view=v;if(selectedTask&&!v.tasks.some(t=>t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}render();}
      else{renderMachines();renderNotice();}
      if(selectedTask)await loadFeed();
    }catch(error){notice(el('p',tr('Окно не смогло показать данные: ')+error.message));}
    finally{refreshing=false;}
  }
  async function loadDiscovery(){try{const d=await api('/api/discovery',8000);discovery=d.agents||[];discoveryLoaded=true;renderAgents();autoCheck();}catch(_){}}
  function show(next){
    tab=next;root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
    root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('list-toggle').hidden=tab!=='tasks';if(tab==='server')loadServer();if(tab==='stats')loadStats();if(tab==='agents')renderAgents();$('screen-title').textContent={tasks:tr('Задачи'),agents:tr('Агенты'),project:tr('Проект'),server:tr('Сервер'),stats:tr('Статистика')}[tab];
  }


  // T-068: tokens spent by the tasks of this computer over a period. Only
  // tokens, no dollars (D-074). One hue for every bar: the bars show size,
  // not identity, and the names stand next to them.
  let statsPeriod=['day','week','month'].includes(recall('messenger-stats-period'))?recall('messenger-stats-period'):'week',stats=null,statsGen=0;
  const periodWords={day:tr('сегодня'),week:tr('за 7 дней'),month:tr('за 30 дней')};
  function tasksWord(n){const a=n%10,b=n%100;if(amLang==='en')return n+(n===1?' task':' tasks');return n+' '+(a===1&&b!==11?'задача':a>=2&&a<=4&&(b<12||b>14)?'задачи':'задач');}
  function tokensWord(n){const a=n%10,b=n%100;if(amLang==='en')return n===1?'token':'tokens';return a===1&&b!==11?'токен':a>=2&&a<=4&&(b<12||b>14)?'токена':'токенов';}
  function tokensFull(n){return n.toLocaleString(amLocale)+' '+tokensWord(n);}
  async function loadStats(){
    const gen=++statsGen;
    try{const s=await api('/api/usage-stats?period='+statsPeriod+(computerFilter?'&computer='+encodeURIComponent(computerFilter):''),8000);if(gen===statsGen){stats=s;renderStats();}}
    catch(error){if(gen===statsGen){$('stats-total-title').textContent=tr('Статистика недоступна');$('stats-total-note').textContent=error.message;}}
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
    computerChip($('stats-computer-chip'));
    $('stats-computers-card').hidden=!!computerFilter;
    $('stats-foot').textContent=!computerFilter?tr('Считаются все компьютеры сети: каждый сообщает расход своих задач по отчётам агентов. Задача попадает в день, когда закончилась.'):computerFilter==='local'?tr('Задачи, которые выполнялись на этом компьютере. Задача попадает в день, когда закончилась.'):tr('Расход сообщает сам компьютер, когда он меняется. В самых затратных задачах — только отправленные туда с этого компьютера.');
    if(!stats)return;
    const t=stats.total;
    $('stats-total-title').textContent=t.tasks?tokensFull(t.tokens)+' '+periodWords[statsPeriod]:tr('Нет задач с расходом ')+periodWords[statsPeriod];
    $('stats-total-note').textContent=t.tasks?tasksWord(t.tasks)+tr(', на входе ')+t.input_tokens.toLocaleString(amLocale)+(t.cached_tokens?tr(' (из кэша ')+t.cached_tokens.toLocaleString(amLocale)+')':'')+tr(', на выходе ')+t.output_tokens.toLocaleString(amLocale):tr('Агент сообщает расход, когда задача заканчивается.');
    statsDays($('stats-days'),statsPeriod==='day'?[]:stats.days);
    statsBars($('stats-agents'),stats.agents,t.tokens,true);
    statsBars($('stats-computers'),stats.computers,t.tokens);
    const top=$('stats-top');top.replaceChildren();
    if(!stats.top.length)top.append(el('p',tr('Пока пусто'),'am-small am-muted'));
    for(const task of stats.top){
      const row=el('button',undefined,'am-stats-task');row.type='button';
      const words=el('span',undefined,'am-stats-task-text');
      words.append(el('span',task.title||tr('Без названия'),'am-stats-task-title'),el('span',[task.project,shortNames[task.agent]||task.agent,task.computer].filter(Boolean).join(', '),'am-small am-muted'));
      row.append(words,el('span',tokensText(task.tokens),'am-stats-value'));
      row.title=tr('Открыть задачу');
      row.addEventListener('click',()=>{
        if(task.project&&task.project!==project){project=task.project;remember('messenger-project',project);viewSignature='';}
        selectedTask=task.id;feed=null;feedSignature='';show('tasks');refresh();
      });
      top.append(row);
    }
  }
  root.querySelectorAll('[data-period]').forEach(b=>b.addEventListener('click',()=>{statsPeriod=b.dataset.period;remember('messenger-stats-period',statsPeriod);stats=null;renderStats();loadStats();}));

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
    $('host-scope').disabled=running;$('host-port').disabled=running;
    const accept=!!server.accept;$('accept-toggle').setAttribute('aria-checked',String(accept));$('net-full-toggle').setAttribute('aria-checked',String(!!server.net_full_access));
    $('log-share-toggle').setAttribute('aria-checked',String(!!server.log_share));
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
    badge($('host-badge'),h.running?tr('Работает'):h.enabled?tr('Не запустился'):tr('Остановлен'),h.running?'good':h.enabled?'bad':'');
    // Explanations live in the status tooltip so the card does not change
    // height; only a problem the owner must fix stays visible.
    $('host-badge').title=!h.running?tr('Другие устройства смогут подключаться к этому компьютеру по одноразовому приглашению.'):tr('Принимает устройства: ')+(h.scope==='lan'?tr('в локальной сети'):tr('только с этого компьютера'))+tr(', порт ')+h.port+'.';
    $('host-state').textContent=h.running&&!h.addresses.length?tr('У компьютера нет адреса в локальной сети. Подключитесь к сети или выберите «Только этот компьютер».'):'';
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
    badge($('member-badge'),m.connected?(m.reachable||!m.checked_at?tr('Подключено'):tr('Нет связи')):tr('Не подключено'),m.connected?(m.reachable?'good':m.checked_at?'bad':''):'');
    $('member-join').hidden=m.connected;$('member-joined').hidden=!m.connected;
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
    const scope=$('host-scope').value;
    if(await serverAction({action:'enable',scope,port},'host','')){
      serverEdited=false;hideInvitation();
      if(scope==='lan')note('host',server.host.addresses.length?tr('Сервер запущен и слушает локальную сеть. Доступ с другого компьютера зависит от брандмауэра.'):tr('Сервер запущен, но адреса в локальной сети у компьютера нет.'),false,true);
      renderServer();
    }
  }
  async function invite(){
    const code=await serverAction({action:'invite',address:$('host-address').value},'host','');
    if(!code)return;
    note('host','');invitation=code;
    $('invite-code').textContent=code.code;copyState('');
    // The 6-digit code for a computer on the same network (T-052).
    $('invite-short').hidden=!code.short;
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
    $('project-page-key').textContent=renamed?tr('Для других компьютеров и координатора проект по-прежнему называется «')+name+'».':'';
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
      project='';remember('messenger-project','');selectedTask='';feed=null;feedSignature='';viewSignature='';projectPageFor='';await refresh();
    });
  }
  // «Установить» opens the agent's official page; «Войти» opens a terminal
  // with the agent's own sign-in (D-057).
  // A row note shows for a while, then the row offers its action again.
  function noteRow(key,text,ms){rowNote.set(key,text);renderAgents();if(ms)setTimeout(()=>{if(rowNote.get(key)===text){rowNote.delete(key);renderAgents();}},ms);}
  async function installAgent(provider){
    noteRow(provider+':program',tr('Открываем страницу…'));
    try{await postAPI('/api/agent-install',{provider});noteRow(provider+':program',tr('Страница установки открыта'),8000);}
    catch(error){rowNote.delete(provider+':program');renderAgents();showError(error);}
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
    for(const [provider,at] of authOpened){if(Date.now()-at<10*60*1000&&!checking.has(provider))checkCard(provider,true);else authOpened.delete(provider);}
  });

  let setupProvider='';
  // The models Claude itself accepts by these names; Codex and Cursor names
  // are not invented here (T-061).
  const defaultModels={claude:['haiku','sonnet','opus']};
  function openAgentSetup(provider){
    setupProvider=provider;const a=(view?.agents||[]).find(x=>x.agent===provider),found=discovery.find(d=>d.provider===provider);
    $('agent-setup-title').textContent=tr('Настройки: ')+names[provider];
    $('agent-setup-note').hidden=provider!=='cursor';
    $('agent-cli').value=a?.cli||found?.cli||'';$('agent-profile').value=a?.environment||found?.environment||'';
    const models=a?.models?.length?a.models:defaultModels[provider]||[];
    $('agent-models').value=models.join(', ');$('agent-models').disabled=!a;
    $('agent-models').placeholder=a?tr('Пусто: задачи идут на модель по умолчанию'):tr('Сначала включите агента на карточке');
    $('agent-models-note').textContent=tr('Через запятую, как их пишет сам ')+shortNames[provider]+'.'+(defaultModels[provider]?tr(' Уже вписаны по умолчанию.'):'');
    setStatus('agent-save-status','');
    $('agent-setup').showModal();
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
    const a=(view?.agents||[]).find(x=>x.agent===setupProvider);if(!a)return;
    const models=$('agent-models').value.split(/[,\n]/).map(m=>m.trim()).filter(Boolean);
    if(models.join('\n')===(a.models||[]).join('\n'))return;
    setStatus('agent-save-status','');
    try{await postAPI('/api/agent-models',{provider:setupProvider,models});viewSignature='';await refresh();}
    catch(error){setStatus('agent-save-status',error.message);}
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
  // The window names a build the usual short way, «v0.41» for
  // 0.0.0-preview.41 (owner); the release tag stays as it is because updates
  // compare it. The tooltip says it is an alpha and gives the exact tag.
  function versionName(version){
    const preview=version.match(/^0\.0\.0-preview\.(\d+)$/);
    return preview?'v0.'+preview[1]:version?'v'+version:'';
  }
  function versionText(installed){return versionName(installed);}
  // The update icon says in its tooltip which version came out (T-066).
  function updateTip(version,busy){
    const name=versionName(version);
    const text=busy?tr('Обновление загружается'):name?tr('Вышла ')+name+tr('. Нажмите, чтобы обновить'):tr('Обновить');
    $('app-update-button').title=text;$('app-update-button').setAttribute('aria-label',text);
  }
  window.messengerUpdateStatus = status => {
    if(!window.webkit?.messageHandlers?.updates)return;
    const version=typeof status?.version==='string'?status.version:'';
    const installed=typeof status?.installedVersion==='string'?status.installedVersion:'';
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Альфа-версия, точный номер ')+installed:'';
    const detail=typeof status?.detail==='string'?status.detail:'';
    const waiting=detail===tr('Обновление загружается в фоне');
    // Only an available update is shown, as the blue icon next to the version.
    // Background checks and their failures stay quiet; the app checks again
    // by itself.
    const shown=!!version||waiting;
    $('app-update-version').hidden=true;$('app-update-button').hidden=!shown;
    $('app-installed-version').textContent=versionText(installed);
    updateTip(version,waiting);
    $('app-update-button').disabled=waiting;
  };
  // Windows and Linux: the program updates itself from the window (D-048).
  // The update is downloaded and verified in the background; only an update
  // ready to install is shown.
  let selfUpdate=null,selfUpdateBusy=false,selfUpdateTimer=0;
  function renderSelfUpdate(){
    const s=selfUpdate;if(!s)return;
    const installed=typeof s.installed_version==='string'?s.installed_version:'';
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Альфа-версия, точный номер ')+installed:'';
    const version=typeof s.version==='string'?s.version:'';
    const done=s.state==='installed',shown=(s.state==='available'&&!!version)||selfUpdateBusy||done||!!s.message;
    // Only the update icon while an update waits (owner, T-056, T-066); a line
    // appears only for an error or after the update.
    $('app-update-version').hidden=!s.message&&!done;$('app-update-button').hidden=!shown||done;
    $('app-installed-version').textContent=versionText(installed);
    $('app-update-version').textContent=s.message||(done?tr('Обновлено. Окно откроется заново.'):'');
    updateTip(version,selfUpdateBusy);
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
    const label=expanded?tr('Скрыть список задач'):tr('Показать список задач');$('list-toggle').setAttribute('aria-label',label);$('list-toggle').title=label;
    $('task-panel').hidden=!expanded;$('task-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  $('project-select').addEventListener('change',()=>{
    const value=$('project-select').value;
    if(value==='__new'){$('project-select').value=view?.project||'__new';syncSelects();openProjectDialog();return;}
    project=value;remember('messenger-project',value);selectedTask='';feed=null;feedSignature='';viewSignature='';refresh();
  });
  $('agent-filter').addEventListener('change',renderTaskList);
  for(const id of ['computer-chip','stats-computer-chip','agents-computer-chip'])$(id).querySelector('button').addEventListener('click',clearComputer);
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
  splitter(root.querySelector('.am-sidebar'),tr('Ширина левой панели'),'--am-nav',204,160,420,1);
  splitter($('task-panel'),tr('Ширина списка задач'),'--am-queue',240,200,520,-1);
  // A click on empty space in the sidebar shows all computers' tasks again.
  root.querySelector('.am-sidebar').addEventListener('click',event=>{if(computerFilter&&!event.target.closest('.am-machine-card,.am-project,.am-side-bottom,.am-splitter,button,select,input,a,[popover]'))clearComputer();});
  $('answer-text').addEventListener('input',()=>{sizeAnswer();answerControls();});
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
  framelessWindow();
  macTitlebar();
  $('agent-cli-pick').addEventListener('click',()=>pick('agent-cli','file','agent-save-status'));
  $('agent-profile-pick').addEventListener('click',()=>pick('agent-profile','folder','agent-save-status'));
  $('restart-dialog').addEventListener('cancel',event=>event.preventDefault());
  $('restart-go').addEventListener('click',()=>restartClient(!!restart.ask));
  $('restart-quit').addEventListener('click',()=>window.webkit?.messageHandlers?.app?.postMessage('quit'));
  $('restart-copy').addEventListener('click',async()=>{const b=$('restart-copy');try{await navigator.clipboard.writeText(shutdownCommand());b.textContent=tr('Скопировано');}catch(_){b.textContent=tr('Не удалось скопировать');}setTimeout(()=>b.textContent=tr('Скопировать команду'),2000);});
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-cli').addEventListener('change',saveAgentPaths);
  $('agent-profile').addEventListener('change',saveAgentPaths);
  $('agent-models').addEventListener('change',saveAgentModels);
  $('agent-models').addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();saveAgentModels();}});
  $('host-scope').addEventListener('change',()=>{serverEdited=true;});
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
  $('net-full-toggle').addEventListener('click',()=>{if(!server)return;const on=!server.net_full_access;serverAction({action:'net_full_access',on},'accept',on?tr('Задачи других компьютеров смогут выполнять любые команды на этом компьютере.'):tr('Новые запуски задач других компьютеров пойдут с проверками разрешений агента.'));});
  $('accept-toggle').addEventListener('click',()=>{if(!server)return;const on=!server.accept;serverAction({action:'accept',on},'accept',on?'':tr('Новые задачи других компьютеров будут отклоняться. Уже принятые продолжат работу.'));});
  $('member-disconnect').addEventListener('click',()=>serverAction({action:'disconnect'},'member',tr('Отключено. Устройство остаётся в списке сервера, пока его там не отзовут.')));
  let serverPolls=0;
  // The sidebar lists devices from the server state, so it is read on every
  // tab, every 15 s outside «Сервер» (as often as computers report).
  setInterval(()=>{serverPolls++;if(tab==='server'||serverPolls%3===0)loadServer();},5000);
  loadServer();
  let initialTheme=window.messengerInitialTheme;
  if(!initialTheme)try{initialTheme=localStorage.getItem('messenger-theme');}catch(_){}
  $('lang-select').value=amLang;enhanceSelect($('lang-select'));enhanceSelect($('theme-select'));enhanceSelect($('project-select'));enhanceSelect($('agent-filter'));enhanceSelect($('host-scope'));enhanceSelect($('host-address'));
  centreControlText();
  applyTheme(initialTheme||'system');
  matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change',()=>{if(document.documentElement.dataset.theme==='system')applyTheme('system');});
  $('theme-select').addEventListener('change',()=>applyTheme($('theme-select').value,true));
  // The language applies on reload: every text is drawn again in it (T-059).
  $('lang-select').addEventListener('change',()=>{try{localStorage.setItem('messenger-lang',$('lang-select').value);}catch(_){}location.reload();});
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
  setInterval(refresh,2000);setInterval(loadDiscovery,60000);setInterval(()=>{if(tab==='stats')loadStats();},15000);
})();
