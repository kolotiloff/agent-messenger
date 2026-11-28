'use strict';
(() => {
  const root = document.getElementById('am-concept');
  const taskSidebarPreview=root.classList.contains('am-task-sidebar-preview');
  // The Mac app and the Windows window draw their own frame and title (D-054).
  if(window.amTransparentWindow)document.body.classList.add('am-transparent-window');
  if(window.webkit?.messageHandlers?.updates||window.amDesktop)document.body.classList.add('am-native');
  const $ = id => root.querySelector('#' + id);
  const selectViews = new Map();
  function syncSelects(){selectViews.forEach(view=>view.sync());}
  // Keep the existing select as the source of truth for delivery guards. The
  // visible list is a top-layer popover, so neither pane clips it or grows.
  function positionPopover(menu,trigger,minWidth,alignEnd=false){
    const rect=trigger.getBoundingClientRect(),gap=6,margin=8;
    menu.style.width=minWidth==null?'':Math.min(innerWidth-2*margin,Math.max(rect.width,minWidth))+'px';
    menu.style.maxHeight='none';
    const below=innerHeight-rect.bottom-gap-margin,above=rect.top-gap-margin;
    const upwards=below<Math.min(300,menu.scrollHeight)&&above>below;
    const maxHeight=menu.classList.contains('am-searchable-select')?460:360;
    menu.style.maxHeight=Math.max(0,Math.min(maxHeight,upwards?above:below))+'px';
    const width=menu.getBoundingClientRect().width,left=alignEnd?rect.right-width:rect.left;
    menu.style.left=Math.max(margin,Math.min(left,innerWidth-width-margin))+'px';
    menu.style.top=upwards?'auto':rect.bottom+gap+'px';
    menu.style.bottom=upwards?innerHeight-rect.top+gap+'px':'auto';
  }
  function enhanceSelect(select, searchable=false, iconPaths=null) {
    const trigger=document.createElement('button');trigger.type='button';trigger.className='am-select-trigger';
    const label=select.labels[0]?.textContent||tr(select.getAttribute('aria-label')||'')||tr('Выбор');
    trigger.setAttribute('aria-label',label);trigger.setAttribute('aria-haspopup',searchable?'dialog':'listbox');trigger.setAttribute('aria-expanded','false');
    const value=document.createElement('span');value.className='am-select-value';
    const chevron=document.createElement('span');chevron.className='am-select-chevron';chevron.setAttribute('aria-hidden','true');trigger.append(value,chevron);
    if(iconPaths){
      trigger.classList.add('am-sidebar-picker','am-icon-btn','am-chrome-icon');
      trigger.replaceChildren(svgIcon(iconPaths));
    }
    const menu=document.createElement('div');menu.id=select.id+'-menu';menu.className='am-select-menu';menu.setAttribute('popover','auto');menu.setAttribute('role',searchable?'dialog':'listbox');menu.setAttribute('aria-label',label);menu.tabIndex=-1;
    let search,empty;
    const list=searchable?el('div',undefined,'am-draft-model-options'):menu;
    const grouped=select.id==='task-draft-model';
    const groups=new Map();
    if(searchable){
      menu.classList.add('am-searchable-select');
      const field=el('div',undefined,'am-field am-model-search');
      const searchLabel=tr('Поиск моделей');
      search=el('input');search.type='search';search.placeholder=searchLabel;search.setAttribute('aria-label',searchLabel);
      field.append(svgIcon(['M17 17l4 4M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0Z']),search);
      list.setAttribute('role','listbox');list.setAttribute('aria-label',label);
      empty=el('p',tr('Модели не найдены'),'am-select-empty am-muted');empty.setAttribute('role','status');
      menu.append(field,list,empty);
      search.addEventListener('input',()=>{
        const query=search.value.trim().toLocaleLowerCase();
        for(const row of options)row.hidden=!row.dataset.searchLabel.includes(query);
        for(const group of groups.values())group.hidden=!group.querySelector('[role=option]:not([hidden])');
        empty.hidden=options.some(row=>!row.hidden);
        list.scrollTop=0;
      });
    }
    // A modal dialog makes outside nodes inert, even top-layer popovers.
    // Keep its menus inside the dialog so pointer and keyboard input reach them.
    // The native light-dismiss algorithm must recognize a second click on its invoker.
    trigger.setAttribute('popovertarget',menu.id);
    trigger.setAttribute('aria-controls',menu.id);select.after(trigger);(select.closest('dialog')||root).append(menu);select.hidden=true;
    select.labels[0]?.addEventListener('click',event=>{event.preventDefault();trigger.focus();});
    let options=[];
    const isOpen=()=>menu.matches(':popover-open');
    function close(restore=false){if(isOpen())menu.hidePopover();trigger.setAttribute('aria-expanded','false');if(restore)trigger.focus();}
    function renderLabel(node,option,provider=''){
      const fullLabel=option?.textContent||tr('Нет данных');
      const prefix=provider?names[provider]+' ':'';
      const label=prefix&&fullLabel.startsWith(prefix)?fullLabel.slice(prefix.length):fullLabel;
      const effort=searchable&&option?.value.match(/\[effort=([a-z]+)\]$/)?.[1];
      node.textContent=label;
      if(effort&&label.toLowerCase().endsWith(' '+effort)){
        node.textContent=label.slice(0,-effort.length);
        node.append(el('span',label.slice(-effort.length),'am-muted'));
      }
    }
    function sync(){
      renderLabel(value,select.selectedOptions[0]);
      if(iconPaths){
        trigger.title=label+', '+value.textContent;
        trigger.setAttribute('aria-label',trigger.title);
        trigger.classList.toggle('am-filter-active',select.id!=='sidebar-group'&&select.selectedIndex>0);
        if(select.id==='agent-filter'){
          const chosen=$('archive-agent-name');
          chosen.hidden=!select.value;
          chosen.textContent=select.value?value.textContent:'';
        }
      }
      trigger.disabled=select.disabled;
      if(select.disabled)close();
    }
    function choose(index){if(select.disabled||select.options[index]?.disabled)return;select.selectedIndex=index;sync();close(true);select.dispatchEvent(new Event('change',{bubbles:true}));}
    function open(keyboard){
      if(select.disabled||isOpen())return;list.replaceChildren();options=[];groups.clear();
      const indexes=Array.from(select.options,(_,index)=>index);
      if(grouped){
        const order=Object.keys(names);
        const rank=index=>order.indexOf(select.options[index].value.split(':',1)[0]);
        indexes.sort((left,right)=>rank(left)-rank(right));
      }
      indexes.forEach(index=>{
        const option=select.options[index];
        const row=document.createElement('div');row.className='am-select-option';row.setAttribute('role','option');row.setAttribute('aria-selected',String(option.selected));row.tabIndex=-1;
        const provider=grouped?option.value.split(':',1)[0]:'';
        let section=list;
        if(provider&&names[provider]){
          section=groups.get(provider);
          if(!section){
            section=el('div',undefined,'am-model-group');section.setAttribute('role','group');
            const header=el('div',undefined,'am-model-group-title');header.id=menu.id+'-'+provider;
            header.append(providerIcon(provider),names[provider]);
            section.setAttribute('aria-labelledby',header.id);section.append(header);
            groups.set(provider,section);list.append(section);
          }
          row.setAttribute('aria-label',option.textContent);
        }
        row.dataset.searchLabel=option.textContent.toLocaleLowerCase();
        const text=document.createElement('span');renderLabel(text,option,provider);
        if(option.dataset.provider)row.append(providerIcon(option.dataset.provider));
        const check=document.createElement('span');check.className='am-select-check';check.setAttribute('aria-hidden','true');if(option.selected)check.innerHTML='<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3 8.5l3.2 3.2L13 4.5"/></svg>';row.append(text,check);
        if(option.disabled)row.setAttribute('aria-disabled','true');
        row.addEventListener('click',()=>choose(index));section.append(row);row.dataset.index=String(index);options.push(row);
      });
      if(search){search.value='';empty.hidden=options.length>0;}
      menu.showPopover();trigger.setAttribute('aria-expanded','true');
      let minWidth=searchable?276:220;
      if(select.id==='chat-font-select'||select.id==='sidebar-group'){
        menu.style.width='max-content';
        minWidth=Math.max(minWidth,Math.ceil(menu.getBoundingClientRect().width));
      }
      positionPopover(menu,trigger,minWidth,searchable||!!iconPaths);
      if(search){list.scrollTop=0;search.focus({preventScroll:true});}
      else{
        const row=options[select.selectedIndex]||options[0];row?.scrollIntoView({block:'nearest'});
        if(keyboard)row?.focus({preventScroll:true});else menu.focus({preventScroll:true});
      }
    }
    trigger.addEventListener('click',event=>{event.preventDefault();if(isOpen())close();else open();});
    trigger.addEventListener('keydown',event=>{if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();open(true);if(search)return;if(event.key==='Home')options[0]?.focus();if(event.key==='End')options.at(-1)?.focus();}});
    menu.addEventListener('keydown',event=>{
      if(event.key==='Escape'){event.preventDefault();close(true);return;}
      if(event.key==='Tab'){close(true);return;}
      const fromSearch=event.target===search;
      if(fromSearch&&!['ArrowDown','ArrowUp','Enter'].includes(event.key))return;
      const visible=search?options.filter(row=>!row.hidden):options;
      const index=visible.indexOf(document.activeElement);let next=index;
      if(fromSearch&&event.key==='ArrowUp')next=visible.length-1;
      else if(index<0&&(event.key==='ArrowDown'||event.key==='ArrowUp'))next=search?0:Math.max(0,select.selectedIndex);
      else if(event.key==='ArrowDown')next=Math.min(visible.length-1,index+1);
      else if(event.key==='ArrowUp')next=Math.max(0,index-1);
      else if(event.key==='Home')next=0;
      else if(event.key==='End')next=visible.length-1;
      else if(event.key==='Enter'||event.key===' '){
        event.preventDefault();
        const choice=visible[index]||(fromSearch&&visible.length===1?visible[0]:null);
        if(choice)choose(Number(choice.dataset.index));
        return;
      }else return;
      event.preventDefault();visible[next]?.focus();
    });
    menu.addEventListener('toggle',()=>trigger.setAttribute('aria-expanded',String(isOpen())));
    new MutationObserver(()=>{close();sync();}).observe(select,{childList:true,subtree:true,attributes:true,attributeFilter:['disabled']});
    window.addEventListener('resize',()=>close());select.addEventListener('change',sync);
    selectViews.set(select.id,{sync});sync();
    return trigger;
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
  function applyChatFont(persist=false){
    const select=$('chat-font-select');
    if(!select.value)select.value='inter';
    root.dataset.chatFont=select.value;
    if(persist){
      if(window.webkit?.messageHandlers?.chatFont)window.webkit.messageHandlers.chatFont.postMessage(select.value);
      else remember('messenger-chat-font',select.value);
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
  let token=location.hash.slice(1)||root.dataset.previewToken||'';
  try{if(token)sessionStorage.setItem('messenger-ui-token',token);else token=sessionStorage.getItem('messenger-ui-token')||'';}catch(_){}
  if(location.hash)history.replaceState(null,'',location.pathname);
  const nativePreferences=new Set(['kotel-task-panes','kotel-active-pane','kotel-pane-layout','messenger-notice-dismissal']);
  function remember(key,value){
    try{localStorage.setItem(key,value);}catch(_){}
    if(nativePreferences.has(key)&&window.messengerPreferences&&window.messengerPreferences[key]!==value){
      window.messengerPreferences[key]=value;
      window.webkit?.messageHandlers?.windowPreference?.postMessage({key,value});
    }
  }
  function recall(key){if(nativePreferences.has(key)&&window.messengerPreferences?.[key]!=null)return window.messengerPreferences[key];try{return localStorage.getItem(key)||'';}catch(_){return '';}}
  let view=null,project=recall('messenger-project'),selectedTask='',selectedShared=null,feed=null,feedSignature='',viewSignature='';
  // T-133: the task shown last, and whether the feed should go to its end on
  // the next render (a new task opened, or the owner just wrote).
  let feedShown='',feedStick=false,feedPinned=true,feedCount=0;
  let online=false,offlineReason='',discovery=[],tab='tasks',refreshing=false,answering=false;
  const agentChecks=new Map(),checking=new Set(),rowNote=new Map();let autoChecked=false,discoveryLoaded=false;
  let clientInfo=null,clientInfoAt=0,restart={busy:false,ask:null,error:'',wait:false},autoRestarted=false,restartWaitDetails=false;
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
    if(clientInfo?.restart)return Error(tr('В фоне старая версия Kotel.'));
    const e=Error(tr('В фоне старая версия клиента, перезапустите его'));
    e.hint=tr('Закройте Kotel, выполните скопированную команду в Терминале и откройте Kotel снова.');
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
    if(tab==='tasks'||(taskSidebarPreview&&tab==='stats'))root.querySelector('.am-chat-footer').prepend(box);
    else if(taskSidebarPreview)$('settings-dialog').querySelector('.am-settings-views').prepend(box);
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
      if(body.startsWith('client_rejected'))throw Error(tr('Клиент Kotel отклонил запрос окна. Подробности: messenger doctor.'));
      throw Error(tr('Клиент Kotel не запущен или не отвечает. Данные не обновляются.'));
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
    if(m<1)return to?tr('<1 мин'):'1'+tr(' мин');
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
  const waitingForStartTime=t=>!t.started_at&&t.state==='running'&&(t.worker_observed||t.remote?.worker==='running');
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
  function relativeWhen(at,includeClock,archive=false){
    const d=new Date(at);if(!at||isNaN(d.getTime()))return '';
    const today=new Date(),yesterday=new Date();yesterday.setDate(today.getDate()-1);
    const separator=archive?tr(' в '):' ';
    const nearby=sameDay(d,today)?tr('сегодня '):sameDay(d,yesterday)?tr('вчера '):'';
    if(nearby)return (archive?capital(nearby.trim())+separator:nearby)+clock(at);
    const date=d.toLocaleDateString(amLocale,d.getFullYear()===today.getFullYear()?{day:'numeric',month:'short'}:{day:'numeric',month:'short',year:'numeric'});
    return date+(includeClock?separator+clock(at):'');
  }
  function appendTaskWhen(agent,t,archive=false){
    const ended=finishedStates.includes(t.state)||(!activeStates.includes(t.state)&&!!t.finished_at);
    const at=ended?t.finished_at:taskStart(t),label=relativeWhen(at,ended,archive);
    if(!label)return;
    const time=el('time',label,'am-task-when');
    time.dateTime=at;time.title=taskWhenTitle(t);
    agent.append(time);
    return time;
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
    if(activeStates.includes(t.state)&&stopProgress(t)){
      if(t.remote?.silent)return [tr('Остановка не подтверждена'),'am-status-wait'];
      return [tr('Остановка запрошена'),'am-status-wait'];
    }
    // T-137: sent to another computer that has not said it took the task.
    // It may run there with its replies lost, so neither «Работает» nor
    // «not accepted»: say what is known.
    if(t.remote?.unaccepted&&!t.remote.from&&t.state==='running'){
      if(Date.now()-Date.parse(t.created_at)<5*60*1000)return [tr('Ожидает принятия'),'am-status-wait'];
      return [tr('Приём не подтверждён'),'am-status-bad',tr('Приём не подтверждён. Задача может выполняться на удалённом компьютере, даже если ответы не доходят. Автоматической повторной отправки не будет.')];
    }
    switch(t.state){
      case 'running':
        if(t.pending_launch)return t.waits_for_lock?[tr('Ожидает ресурс'),'am-status-wait',t.waits_for_lock]:[tr('Ждёт запуска'),'am-status-wait'];
        return waitingForStartTime(t)?[tr('Работает'),'am-status-run']:[tr('Работает ')+minutes(from),'am-status-run','',from];
      case 'waiting_owner':return [tr('Ждёт вашего ответа'),'am-status-wait'];
      case 'waiting_coordinator':return [tr('Ждёт ответа координатора'),'am-status-wait'];
      case 'done':return [tr('Работал ')+minutes(from,end),'am-muted'];
      case 'failed':return failureStatus(t)||[tr('Ошибка через ')+minutes(from,end),'am-status-bad'];
      case 'interrupted':return [tr('Прервана через ')+minutes(from,end),'am-status-bad',connectionTimestamp(end)];
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
    return taskStatus(sharedDisplayTask(t));
  }
  // T-131: a task leaves the list only when it no longer runs; a running
  // one is stopped first. Another computer's task can also go when that
  // computer is out of contact, since its last state is stale.
  const finishedStates=['done','failed','cancelled'];
  function sharedLost(t,p){return activeStates.includes(t.state)&&!peerFresh(p)&&!sharedWorkerRunning(t);}
  function sharedWorkerRunning(t){
    if(t.state!=='running'||!t.runs_on_peer)return false;
    const executor=server?.peers?.find(p=>p.id===t.runs_on_peer);
    return !!executor&&peerFresh(executor)&&!!executor.workers?.runs?.some(r=>r.task===t.id);
  }
  function sharedDisplayTask(t){
    return sharedWorkerRunning(t)?{...t,pending_launch:false,worker_observed:true}:t;
  }
  const taskPanes=new Map();
  const paneLayouts=['columns','stack','grid'];
  const restartOutcome='the client restarted during this run; the outcome is unknown and nothing is retried';
  const restartLabel=()=>tr('Клиент перезапустился во время работы; результат неизвестен');
  let paneBoard=null,paneStrip=null,paneToolbar=null,paneGrid=null,paneTimer=null,paneActive=null,paneDragging=null;
  function savePanes(){
    remember('kotel-task-panes',JSON.stringify([...taskPanes.values()].map(({id,project,title,placement})=>({id,project,title,placement}))));
    remember('kotel-active-pane',paneActive?.id||'');
  }
  function preservePaneFocus(move){
    const focused=document.activeElement;
    const selection=focused instanceof HTMLTextAreaElement?[focused.selectionStart,focused.selectionEnd,focused.selectionDirection]:null;
    move();
    if(focused?.isConnected&&focused!==document.body&&document.activeElement!==focused){
      focused.focus({preventScroll:true});
      if(selection)focused.setSelectionRange(...selection);
    }
  }
  function closePane(pane){
    const before=[...taskPanes.values()],index=before.indexOf(pane);
    const hadFocus=pane.tab.contains(document.activeElement)||pane.node.contains(document.activeElement);
    abortPaneRequest(pane);
    taskPanes.delete(pane.id);
    pane.tab.remove();pane.node.remove();
    if(paneActive===pane)paneActive=before[index+1]||before[index-1]||null;
    if(!taskPanes.size&&paneBoard){
      const primary=paneGrid.querySelector('.am-chat');
      primary.hidden=false;primary.style.removeProperty('order');
      primary.removeAttribute('role');primary.removeAttribute('aria-labelledby');
      $('task-head').hidden=false;paneToolbar.remove();
      preservePaneFocus(()=>paneBoard.replaceWith(primary));
      paneBoard=paneStrip=paneToolbar=paneGrid=null;
      clearInterval(paneTimer);paneTimer=null;
    }
    syncTaskPanes();renderMachines();
    if(hadFocus){
      if(paneActive)paneActive.label.focus({preventScroll:true});
      else if(!$('compose').hidden&&!$('answer-text').disabled)$('answer-text').focus({preventScroll:true});
      else $('thread').focus({preventScroll:true});
    }
    savePanes();
  }
  function paneVisible(pane){
    return pane.node.isConnected&&!pane.node.hidden&&!document.hidden&&tab==='tasks';
  }
  function abortPaneRequest(pane){
    pane.request?.abort();pane.request=null;
  }
  function renderPaneControls(pane){
    const task=pane.task;
    const canSend=canSendTaskMessage(task);
    pane.input.placeholder=taskMessagePlaceholder(task);
    pane.compose.hidden=!canDraftTaskMessage(task)&&!pane.input.value.trim();
    pane.input.disabled=!canDraftTaskMessage(task);
    pane.input.readOnly=pane.busy;
    pane.send.disabled=!canSend||pane.busy||!pane.input.value.trim();
  }
  async function sendPaneReply(pane){
    if(pane.send.disabled)return;
    const task=pane.task,text=pane.input.value.trim(),question=task.owner_question;
    const key=question?'messenger-answer-op:'+question.id:'messenger-note-op:'+task.id+':'+text;
    let op=recall(key);
    if(!op){op='window-'+(question?.id||task.id)+'-'+crypto.randomUUID();remember(key,op);}
    pane.busy=true;pane.status.textContent=tr('Отправляем сообщение…');renderPaneControls(pane);
    try{
      await postAPI(question?'/api/task-answer':'/api/task-note',{task:task.id,op_id:op,text,...(question?{question:question.id}:{})});
      if(!question)try{localStorage.removeItem(key);}catch(_){}
      pane.input.value='';await updatePane(pane);
    }catch(error){pane.status.textContent=error.message+tr(' Повтор с тем же текстом безопасен.');}
    finally{pane.busy=false;renderPaneControls(pane);}
  }
  function renderPaneSnapshot(pane,snapshot,offset=0){
    const scroll=pane.body.scrollTop;
    const fileSource=taskFileSource(snapshot.task);
    const stick=pane.body.scrollHeight-scroll-pane.body.clientHeight<32;
    if(!offset)pane.body.replaceChildren();
    for(const it of snapshot.items||[]){
      if(!['task','answer','text','question','report','note','outcome'].includes(it.kind)||(!it.text&&it.kind!=='outcome'))continue;
      const message=el('div',undefined,'am-pane-message');
      if(it.kind==='outcome'&&it.state==='cancelled')message.append(stoppedNote(it,snapshot.task));
      else{
        const role=it.author==='owner'?tr('Вы'):it.author==='coordinator'?tr('Координатор'):it.kind==='outcome'?tr('Статус'):tr('Исполнитель');
        message.append(el('span',role,'am-small am-muted'),richText(it.kind==='outcome'&&it.text===restartOutcome?restartLabel():it.text,undefined,fileSource));
      }
      pane.body.append(message);
    }
    pane.count=offset+(snapshot.items?.length||0);
    pane.cursor=snapshot.cursor||'';
    pane.task=snapshot.task;
    pane.label.textContent=pane.heading.textContent=snapshot.task.title||pane.title;
    if(pane.title!==pane.label.textContent){pane.title=pane.label.textContent;savePanes();}
    pane.label.title=pane.title;
    pane.close.setAttribute('aria-label',tr('Закрыть вкладку')+' '+pane.title);
    pane.status.textContent=taskStatus(snapshot.task)[0];
    renderPaneControls(pane);
    pane.body.scrollTop=stick?pane.body.scrollHeight:scroll;
  }
  async function updatePane(pane){
    if(!paneVisible(pane))return;
    if(pane.request)return;
    const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),5000);
    pane.request=controller;
    try{
      const query=new URLSearchParams({id:pane.id,cursor:pane.cursor||''});
      const snapshot=await api('/api/task?'+query,5000,controller.signal);
      if(controller.signal.aborted||!paneVisible(pane)||snapshot.task?.id!==pane.id)return;
      if(snapshot.append_from&&snapshot.append_from!==pane.count){pane.cursor='';return;}
      renderPaneSnapshot(pane,snapshot,snapshot.append_from||0);
    }catch(error){if(!controller.signal.aborted)pane.status.textContent=tr('Не удалось загрузить задачу');}
    finally{clearTimeout(timer);if(pane.request===controller)pane.request=null;}
  }
  function syncTaskPanes(){
    if(!paneBoard)return;
    const enabled=tab==='tasks'&&!document.hidden;
    const primary=paneGrid.querySelector('.am-chat');
    const current=taskPanes.get(selectedTask);
    const usePrimary=!!current&&current.project===project;
    const showPrimary=usePrimary&&(current===paneActive||current.placement==='alongside');
    primary.hidden=!!paneActive&&!draftOpen&&!showPrimary;
    if(showPrimary){
      primary.setAttribute('role','tabpanel');
      primary.setAttribute('aria-labelledby',current.label.id);
    }
    if(!showPrimary){
      primary.removeAttribute('role');primary.removeAttribute('aria-labelledby');
      primary.style.removeProperty('order');
    }
    $('task-head').hidden=!draftOpen;
    paneToolbar.hidden=draftOpen;
    let index=0,visible=0;
    for(const pane of taskPanes.values()){
      const active=pane===paneActive;
      const showing=!draftOpen&&!!paneActive&&(active||pane.placement==='alongside');
      const full=usePrimary&&pane===current&&showPrimary;
      pane.node.hidden=!showing||full;
      pane.node.style.order=index;
      if(full)primary.style.order=index;
      pane.label.setAttribute('aria-controls',full?primary.id:pane.node.id);
      if(showing)visible++;
      index++;
      pane.label.setAttribute('aria-selected',String(active));
      pane.label.tabIndex=active?0:-1;
      pane.close.tabIndex=active?0:-1;
      if(!enabled||pane.node.hidden)abortPaneRequest(pane);
      else updatePane(pane);
    }
    paneBoard.style.setProperty('--am-watch-visible',Math.max(1,visible));
    paneBoard.toggleAttribute('data-single',visible<=1);
    paneBoard.style.setProperty('--am-watch-order',index);
    paneBoard.toggleAttribute('data-odd-grid',visible>1&&visible%2!==0);
    if(!enabled&&paneTimer){clearInterval(paneTimer);paneTimer=null;}
    if(enabled&&!paneTimer)paneTimer=setInterval(()=>taskPanes.forEach(updatePane),5000);
  }
  function activatePane(pane){
    paneActive=pane;
    if(draftOpen){closeTaskDraft();renderThread();}
    syncTaskPanes();renderMachines();savePanes();
  }
  function reorderPane(pane,target){
    const ordered=[...taskPanes.values()];
    const from=ordered.indexOf(pane),to=ordered.indexOf(target);
    if(from<0||to<0||from===to)return;
    ordered.splice(from,1);ordered.splice(to,0,pane);
    const next=ordered[to+1];
    taskPanes.clear();
    ordered.forEach(item=>taskPanes.set(item.id,item));
    preservePaneFocus(()=>{
      paneStrip.insertBefore(pane.tab,next?.tab||null);
      paneGrid.insertBefore(pane.node,next?.node||null);
    });
    savePanes();
  }
  function setPaneLayout(layout){
    if(!paneLayouts.includes(layout))layout='grid';
    paneBoard.dataset.layout=layout;
    for(const button of paneToolbar.querySelectorAll('[data-pane-layout]')){
      button.setAttribute('aria-pressed',String(button.dataset.paneLayout===layout));
    }
    remember('kotel-pane-layout',layout);
  }
  function openTaskPane(task,name,placement='tab',restore=false){
    if(!paneBoard){
      paneBoard=el('div',undefined,'am-pane-board');
      paneGrid=el('div',undefined,'am-pane-grid');
      paneStrip=el('div',undefined,'am-pane-strip');
      paneStrip.setAttribute('role','tablist');paneStrip.setAttribute('aria-label',tr('Вкладки задач'));
      paneToolbar=el('div',undefined,'am-pane-toolbar');
      const layouts=el('div',undefined,'am-pane-layouts');
      for(const [mode,text] of [['columns',tr('Столбцы')],['stack',tr('Строки')],['grid',tr('Сетка')]]){
        const button=el('button',text,'am-quiet-button');
        button.type='button';button.dataset.paneLayout=mode;button.onclick=()=>{
          for(const pane of taskPanes.values())pane.placement='alongside';
          setPaneLayout(mode);syncTaskPanes();savePanes();
        };layouts.append(button);
      }
      paneToolbar.append(paneStrip,layouts);paneBoard.append(paneGrid);
      $('task-head').after(paneToolbar);
      const primary=$('task-layout').querySelector('.am-chat');
      primary.id='task-primary-pane';
      preservePaneFocus(()=>{primary.replaceWith(paneBoard);paneGrid.append(primary);});
      setPaneLayout(recall('kotel-pane-layout'));
      const current=selectedTaskRow();
      if(!restore&&current&&current.id!==task.id)openTaskPane(current,project,placement,true);
    }
    if(placement==='alongside'&&paneActive)paneActive.placement='alongside';
    if(taskPanes.has(task.id)){
      const pane=taskPanes.get(task.id);
      pane.placement=placement;activatePane(pane);return;
    }
    const node=el('section',undefined,'am-watch-pane'), tabNode=el('div',undefined,'am-pane-tab');
    const label=el('button',task.title,'am-quiet-button');
    const close=el('button',undefined,'am-icon-btn am-close-x');
    const status=el('span',tr('Загружаем задачу…'),'am-small am-muted');
    const heading=el('h3',task.title);
    const body=el('div',undefined,'am-watch-body');
    const input=el('textarea',undefined,'am-watch-input');
    const compose=el('form',undefined,'am-watch-compose');
    const send=el('button',tr('Отправить ответ'));
    const uid=crypto.randomUUID();
    node.id='kotel-pane-'+uid;node.setAttribute('role','tabpanel');
    label.type=close.type='button';label.id='kotel-tab-'+uid;
    label.title=task.title;label.setAttribute('role','tab');label.setAttribute('aria-controls',node.id);
    node.setAttribute('aria-labelledby',label.id);
    tabNode.setAttribute('role','presentation');tabNode.draggable=true;
    close.setAttribute('aria-label',tr('Закрыть вкладку')+' '+task.title);
    close.append(svgIcon(['M6 6l12 12M18 6L6 18']));
    input.rows=1;input.maxLength=20000;input.setAttribute('aria-label',tr('Ответ исполнителю'));
    input.placeholder=tr('Написать исполнителю…');send.type='submit';
    compose.hidden=true;compose.append(input,send);
    status.setAttribute('role','status');
    body.tabIndex=0;
    const pane={id:task.id,project:name,title:task.title,placement,node,tab:tabNode,label,close,heading,body,status,input,send,compose,count:0};
    label.onclick=()=>activatePane(pane);
    close.onclick=()=>closePane(pane);
    input.oninput=()=>renderPaneControls(pane);
    compose.onsubmit=event=>{event.preventDefault();sendPaneReply(pane);};
    input.onkeydown=event=>{
      if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();sendPaneReply(pane);}
    };
    label.onkeydown=event=>{
      const ordered=[...taskPanes.values()],index=ordered.indexOf(pane);
      if(event.key==='Delete'){event.preventDefault();closePane(pane);return;}
      const direction=event.key==='ArrowLeft'?-1:event.key==='ArrowRight'?1:0;
      const next=direction?ordered[(index+direction+ordered.length)%ordered.length]:
        event.key==='Home'?ordered[0]:event.key==='End'?ordered.at(-1):null;
      if(!next)return;
      event.preventDefault();
      if(event.altKey&&direction)reorderPane(pane,ordered[index+direction]);
      else{
        activatePane(next);next.label.focus({preventScroll:true});
        next.label.scrollIntoView({block:'nearest',inline:'nearest'});
      }
    };
    tabNode.ondragstart=event=>{
      startPaneDrag(event,pane,pane.project);
    };
    tabNode.ondragover=event=>{
      if(paneDragging&&paneDragging.task.id!==pane.id){event.preventDefault();event.dataTransfer.dropEffect='move';tabNode.classList.add('am-pane-drop');}
    };
    tabNode.ondragleave=()=>tabNode.classList.remove('am-pane-drop');
    tabNode.ondrop=event=>{
      if(paneDragging){
        event.preventDefault();
        const source=taskPanes.get(paneDragging.task.id);
        if(source)reorderPane(source,pane);
        else openTaskPane(paneDragging.task,paneDragging.project);
        finishPaneDrag();
      }
    };
    tabNode.ondragend=finishPaneDrag;
    tabNode.append(label,close);node.append(heading,status,body,compose);
    paneStrip.append(tabNode);paneGrid.append(node);taskPanes.set(task.id,pane);
    if(!restore)activatePane(pane);
  }
  function startPaneDrag(event,task,name){
    paneDragging={task,project:name};
    $('task-layout').classList.add('am-pane-dragging');
    event.dataTransfer.effectAllowed='move';
    event.dataTransfer.setData('text/plain',task.id);
  }
  function finishPaneDrag(){
    paneDragging=null;
    $('task-layout').classList.remove('am-pane-dragging');
    $('task-layout').querySelectorAll('.am-pane-drop').forEach(node=>node.classList.remove('am-pane-drop'));
  }
  function installPaneDropTargets(){
    const layout=$('task-layout');
    const targets=el('div',undefined,'am-pane-targets');
    const layouts=[
      ['columns',tr('Столбцы'),'M12 5v14'],
      ['stack',tr('Строки'),'M4 12h16'],
      ['grid',tr('Сетка'),'M12 5v14M4 12h16'],
    ];
    for(const [mode,title,divider] of layouts){
      const target=el('div',undefined,'am-pane-target');
      const preview=el('div',undefined,'am-pane-target-preview');
      preview.append(svgIcon(['M4 5h16v14H4z',divider]),el('span',title));
      target.append(preview);
      target.dataset.dropLayout=mode;
      target.ondragover=event=>{
        if(!paneDragging)return;
        event.preventDefault();event.dataTransfer.dropEffect='move';target.classList.add('am-pane-drop');
      };
      target.ondragleave=event=>{if(!target.contains(event.relatedTarget))target.classList.remove('am-pane-drop');};
      target.ondrop=event=>{
        if(!paneDragging)return;
        event.preventDefault();
        const {task,project:name}=paneDragging;
        finishPaneDrag();
        openTaskPane(task,name,'alongside');
        for(const pane of taskPanes.values())pane.placement='alongside';
        setPaneLayout(mode);syncTaskPanes();savePanes();
      };
      targets.append(target);
    }
    layout.append(targets);
  }
  function taskMenu(event,t,p,name,inArchive=false){
    event.preventDefault();
    // Unchanged rows keep their listeners across refreshes. The underlying
    // stop request can advance even when its short label stays the same.
    if(!p){
      const current=taskRows(taskListView(name)).find(task=>task.id===t.id);
      if(!current)return;
      t=current;
    }
    const entries=[];
    if(taskSidebarPreview&&!p){
      entries.push({text:tr('Открыть во вкладке'),pick:()=>openTaskPane(t,name)},
        {text:tr('Открыть рядом'),pick:()=>openTaskPane(t,name,'alongside')});
    }
    if(!p&&!t.remote?.from){
      entries.push({text:tr('Переименовать задачу'),pick:()=>renameTask(t)});
    }
    if(!p&&canStopTask(t)){
      const progress=stopProgress(t);
      if(progress){
        popMenu(event.clientX,event.clientY,[],{task:t.id,text:stopMessage(progress)});
        return;
      }
      entries.push({text:tr('Остановить задачу'),pick:()=>stopTask(t,name)});
    }
    const interrupted=t.state==='blocked'||t.state==='interrupted';
    if(!p&&(interrupted||taskUnreachable(t))){
      entries.push({text:tr('Закрыть задачу'),pick:()=>closeHere(t,name)});
    }
    if(p&&interrupted){
      entries.push({text:tr('Скрыть на этом компьютере'),pick:()=>askRemoveTask(t,p,name)});
    }
    if(!p&&!t.remote?.from&&finishedStates.includes(t.state)){
      if(t.archived){
        entries.push({text:tr('Восстановить из архива'),pick:()=>changeTaskArchive(t,name,'restore')});
        if(inArchive)entries.push({text:tr('Убрать из архива навсегда'),pick:()=>{
          askConfirm(tr('Убрать задачу из архива?'),tr('Карточка исчезнет из списков после синхронизации. Переписка, файлы и статистика сохранятся.'),tr('Убрать из архива'),()=>changeTaskArchive(t,name,'remove'));
        }});
      }else entries.push({text:tr('Архивировать задачу'),pick:()=>changeTaskArchive(t,name,'archive')});
    }else if(!t.archived&&(finishedStates.includes(t.state)||(p&&!interrupted&&sharedLost(t,p)))){
      entries.push({text:tr('Удалить из списка'),pick:()=>askRemoveTask(t,p,name)});
    }
    popMenu(event.clientX,event.clientY,entries);
  }
  function renameTask(task){
    const field=el('input');
    field.type='text';field.value=task.title;field.maxLength=120;
    field.setAttribute('aria-label',tr('Название задачи'));
    askConfirm(tr('Переименовать задачу'),'',tr('Сохранить'),async()=>{
      await postAPI('/api/task-rename',{task:task.id,previous:task.title,title:field.value});
      viewSignature='';await refresh();loadProjectActivity();
    },true);
    $('confirm-text').append(field);field.select();
    field.addEventListener('keydown',event=>{
      if(event.key==='Enter'&&!event.isComposing&&field.value.trim())$('confirm-go').click();
    });
  }
  function askRemoveTask(t,p,name){
    const text=p?tr('Задача пропадёт из списка на этом компьютере. У других она останется, пока её не удалят на компьютере «')+p.name+'».'
      :tr('Задача пропадёт из списка на всех компьютерах сети. Её история останется в базе этого компьютера.');
    askConfirm(tr('Удалить задачу из списка?'),text,tr('Удалить'),async()=>{
      await postAPI('/api/task-hide',{peer:p?.peer||'',task:t.id});
      if(p){if(selectedShared?.peer===p.peer&&selectedShared?.id===t.id)selectedShared=null;}
      else{
        taskHistories.get(name)?.rows.delete(t.id);
        if(selectedTask===t.id){selectedTask='';feed=null;feedSignature='';}
      }
      viewSignature='';await refresh();loadProjectActivity();
    });
  }
  async function changeTaskArchive(task,name,action){
    try{
      await postAPI('/api/task-archive',{task:task.id,action,version:task.archive_version||0});
      taskHistories.delete(name);
      if(action==='remove'&&selectedTask===task.id){selectedTask='';feed=null;feedSignature='';}
      viewSignature='';await refresh();loadProjectActivity();
    }catch(error){
      if(action==='remove')throw error;
      showError(error);
    }
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
  let splitProjects=[],auxActivity=null,auxError='',auxRequest=null,auxTaskScope='';
  const groupSelect=$('sidebar-group');
  const savedGrouping=recall('kotel-sidebar-group');
  if([...groupSelect.options].some(option=>option.value===savedGrouping))groupSelect.value=savedGrouping;
  const collapsedFolders=new Set();
  const sidebarSnapshots=new Map(),collapsedComputers=new Set();
  let sidebarProject='',sidebarRequest=null,draftOpen=false,draftSending=false,draftAttempt=null;
  let composerDictation=null;
  function sidebarProjects(){
    return sidebarProject?[sidebarProject]:(view?.projects||[]).map(entry=>entry.name);
  }
  async function loadSidebarProjects(){
    if(sidebarRequest||!online||document.hidden)return;
    const names=sidebarProjects().filter(name=>name!==project),scope=names.map(taskWindowQuery);
    if(!names.length)return;
    sidebarRequest=Promise.allSettled(names.map(async(name,index)=>{
      const query=new URLSearchParams(scope[index]);
      query.set('history','1');
      const snapshot=await api('/api/project-activity?'+query,4500);
      if(snapshot.project!==name||taskWindowQuery(name)!==scope[index])return;
      sidebarSnapshots.set(name,{snapshot,scope:scope[index]});
      syncTaskHistory(snapshot);
    }));
    await sidebarRequest;
    sidebarRequest=null;
    renderMachines();renderTaskList();
  }
  function projectOptions(select,options,value,disabled=false){
    if(select.options.length!==options.length||options.some(([value,label],i)=>select.options[i].value!==value||select.options[i].textContent!==label)){
      select.replaceChildren(...options.map(([value,label])=>{
        const option=el('option',label);option.value=value;return option;
      }));
    }
    select.value=value;
    if(select.disabled!==disabled)select.disabled=disabled;
  }
  function updateSplitProjects(next){
    const before=statsProjectScope();
    splitProjects=next;
    if(!splitProjects.includes(statsProject))statsProject='';
    if(!next.length){auxRequest?.controller.abort();auxRequest=null;auxActivity=null;auxError='';auxTaskScope='';}
    for(const name of taskHistories.keys())if(name!==project&&!next.includes(name))taskHistories.delete(name);
    updateStatsProjectScope(before);
  }
  function passiveProject(){return splitProjects.find(name=>name!==project)||'';}
  async function loadProjectActivity(){
    if(taskSidebarPreview)return loadSidebarProjects();
    const name=passiveProject();
    const scope=auxTaskQuery();
    if(auxRequest?.name===name&&auxRequest.scope===scope)return;
    auxRequest?.controller.abort();
    if(!name||!online||document.hidden)return;
    if(auxError!==name)auxError='';
    const controller=new AbortController(),request={name,scope,controller};
    auxRequest=request;
    const timeout=setTimeout(()=>controller.abort(),4500);
    try{
      const q=new URLSearchParams(scope||{project:name});
      if(scope)q.set('history','1');
      const result=await api('/api/project-activity?'+q,4500,controller.signal);
      if(passiveProject()!==name||auxTaskQuery()!==scope||auxRequest!==request)return;
      if(result.project!==name)throw Error('unexpected project activity');
      auxActivity=result;auxTaskScope=scope;auxError='';
      if(scope){syncTaskHistory(result);syncStopSubmissions(result.tasks);}
    }catch(_){
      if(passiveProject()!==name||auxTaskQuery()!==scope||auxRequest!==request)return;
      if(auxActivity?.project===name)auxActivity=null;
      auxError=name;
    }finally{
      clearTimeout(timeout);
      if(auxRequest===request){auxRequest=null;renderMachines();if(scope)renderTaskList();}
    }
  }
  function projectTitle(name){return view?.projects.find(p=>p.name===name)?.title||name;}
  function renderProjects(){
    const options=view?view.projects.map(p=>[p.name,p.title||p.name]):[['',offlineReason?tr('Проекты недоступны'):tr('Загружаем проекты…')]];
    if(taskSidebarPreview){
      if(view&&!options.some(([name])=>name===sidebarProject))sidebarProject='';
      $('project-heading').textContent=sidebarProject?projectTitle(sidebarProject):tr('Проекты');
      $('project-count').hidden=!view||!!sidebarProject;
      $('project-count').textContent=view?String(view.projects.length):'';
      $('split-projects').hidden=true;
      projectOptions($('project-select'),view?[['__all',tr('Все проекты')],...options,['__new',tr('Новый проект')]]:options,view?sidebarProject||'__all':'',!view);
      $('task-new').disabled=!view?.projects.length;
      syncSelects();
      return;
    }
    const canSplit=!!view&&options.length>1&&options.some(([name])=>name===project);
    if(splitProjects.length){
      const other=options.find(([name])=>name===passiveProject())?.[0]||options.find(([name])=>name!==project)?.[0];
      if(!canSplit)updateSplitProjects([]);
      else if(!splitProjects.includes(project)||!splitProjects.includes(other)){
        updateSplitProjects(splitProjects[1]===project?[other,project]:[project,other]);
        loadProjectActivity();
      }
    }
    const split=splitProjects.length>0;
    const toggle=$('split-projects');
    toggle.hidden=!canSplit;
    toggle.setAttribute('aria-pressed',String(split));
    $('project-heading').textContent=split?tr('Проекты'):tr('Проект');
    root.classList.toggle('am-split-projects',split);
    $('project-column-second').hidden=!split;
    $('project-column-first').toggleAttribute('data-active-project',split&&splitProjects[0]===project);
    $('project-column-second').toggleAttribute('data-active-project',split&&splitProjects[1]===project);
    if(split){
      projectOptions($('project-select'),options.filter(([name])=>name!==splitProjects[1]),splitProjects[0]);
      projectOptions($('project-select-second'),options.filter(([name])=>name!==splitProjects[0]),splitProjects[1]);
    }else{
      if(view)options.push(['__new',tr('Новый проект')]);
      projectOptions($('project-select'),options,project||(view?'__new':''),!view);
    }
    syncSelects();
  }
  // A right-click menu; entries are {text,pick}.
  function popMenu(x,y,entries,status=null,device=''){
    if(!entries.length&&!status&&!device)return;
    let menu=$('device-menu');
    if(!menu){
      // A manual popover: an automatic one closes on the same right click
      // that opened it.
      menu=el('div',undefined,'am-select-menu am-context-menu');menu.id='device-menu';menu.setAttribute('popover','manual');menu.tabIndex=-1;root.append(menu);
      const close=()=>{if(menu.matches(':popover-open'))menu.hidePopover();};
      document.addEventListener('pointerdown',e=>{if(!menu.contains(e.target))close();});
      menu.addEventListener('keydown',e=>{if(e.key==='Escape'){e.preventDefault();e.stopPropagation();close();}});
      menu.addEventListener('focusout',e=>{if(!menu.contains(e.relatedTarget))close();});
      document.addEventListener('pointermove',event=>{
        if(event.pointerType==='mouse'&&menu.matches(':popover-open')&&
          menu.querySelector('.am-context-device')&&!menu.contains(event.target))close();
      });
      window.addEventListener('blur',close);
      window.addEventListener('resize',close);
    }
    const items=[];
    for(const it of entries){
      items.push(contextMenuItem(menu,it.text,it.pick));
    }
    menu.setAttribute('role',status?'status':'menu');
    menu.dataset.stopTask=status?.task||'';
    menu.replaceChildren(...(status?[el('div',status.text,'am-context-status')]:items));
    if(device){
      const footer=el('div',undefined,'am-context-device am-version-row');
      footer.dataset.device=device;
      menu.append(footer);
      renderDeviceMenuFooter(footer);
    }
    menu.showPopover();
    menu.style.left=Math.max(0,Math.min(x,window.innerWidth-menu.offsetWidth))+'px';
    menu.style.top=Math.max(0,Math.min(y,window.innerHeight-menu.offsetHeight))+'px';
    (menu.querySelector('[role=menuitem]')||menu).focus();
  }
  function contextMenuItem(menu,text,pick){
    const item=el('button',text,'am-select-option');item.type='button';item.setAttribute('role','menuitem');
    item.onclick=()=>{menu.hidePopover();pick();};
    return item;
  }
  function deviceMenu(d,x,y,can){
    const peer=(server?.peers||[]).find(other=>other.id===d.id);
    if(!peer)return;
    const items=[];
    const add=(text,pick)=>items.push({text,pick});
    if(can.revoke)add(tr('Отозвать устройство'),()=>{revokeAsk=d.id;clearTimeout(revokeTimer);revokeTimer=setTimeout(()=>{revokeAsk='';renderMachines();},8000);renderMachines();$('machines').querySelector('[data-key="'+CSS.escape('revoke-yes-'+d.id)+'"]')?.focus();});
    popMenu(x,y,items,null,peer.id);
  }
  function renderDeviceMenuFooter(footer=$('device-menu')?.matches(':popover-open')?$('device-menu').querySelector('.am-context-device'):null){
    if(!footer)return;
    const device=footer.dataset.device;
    const local=device==='local';
    const peer=local?null:server?.peers?.find(item=>item.id===device);
    const version=local?$('app-installed-version').textContent:(peer?versionName(peerInstalledVersion(peer)||''):'');
    const menu=footer.parentNode,mark=local||peer?updateMark(peer):null;
    const original=$('app-update-button');
    let caption='',pick=null;
    if(updateInProgress(mark)){
      const state=updatePresentation(peer,mark);
      caption=state.canCancel?tr('Отменить обновление'):state.caption;
      if(state.canCancel)pick=()=>void cancelUpdate(state);
    }else if(local&&!original.hidden){
      caption=original.title;
      if(!original.disabled)pick=()=>original.click();
    }else if(peer){
      const newest=peerUpdateOffer(peer,peerInstalledVersion(peer),mark);
      if(newest){
        caption=tr('Обновить версию');
        pick=()=>confirmPeerUpdate(peer,newest);
      }
    }
    const label=el('span',version||'—','am-version');
    label.title=version?tr('Версия ')+version.replace(/^v/,''):tr('Версия неизвестна');
    if(!label.isEqualNode(footer.firstChild))footer.replaceChildren(label);
    const next=caption?(pick?contextMenuItem(menu,caption,pick):el('div',caption,'am-context-status')):null;
    const previous=menu.querySelector('[data-device-update]');
    if(next)next.dataset.deviceUpdate=JSON.stringify([device,mark?.id]);
    if((!next&&!previous)||next?.isEqualNode(previous))return;
    const restoreFocus=previous===document.activeElement;
    if(restoreFocus)menu.focus({preventScroll:true});
    previous?.remove();
    if(next)footer.before(next);
    if(restoreFocus&&pick)next.focus({preventScroll:true});
  }
  // Asking another computer to update itself (T-078). Its answers arrive
  // with the server status and show as notices in the window;
  // the card only says while it is under way.
  async function requestUpdate(peer){
    try{const answer=await postAPI('/api/server-action',{action:'update',device:peer.id},28000);if(answer?.result)server=answer.result;}
    catch(error){addNotice('refused-'+peer.id+'-'+Date.now(),'bad',(peer.name||'')+tr(' не обновлён'),capital(error.message));}
    renderMachines();
  }
  // The broker wakes all open windows when the same request changes phase.
  // Keep the last snapshot across ordinary server/status refreshes.
  let updateBoard=null,watchingUpdates=false,watchRetry=0;
  function updateNetwork(){
    return (server?.host?.running?server.host.broker_id:server?.member?.broker_id)||'';
  }
  function watchUpdateBoard(){
    if(watchingUpdates||document.hidden||!updateNetwork()||Date.now()<watchRetry)return;
    watchingUpdates=true;
    (async()=>{
      try{
        while(!document.hidden&&updateNetwork()){
          const network=updateNetwork(),cursor=updateBoard?.broker===network?updateBoard.cursor:'';
          const {result}=await postAPI('/api/server-action',{action:'update-watch',cursor},18000);
          if(network!==updateNetwork())continue;
          if(!result?.cursor||result.broker!==network)throw Error('unsupported');
          if(result.cursor!==cursor){updateBoard=result;renderMachines();}
        }
      }catch(_){watchRetry=Date.now()+15000;}
      finally{watchingUpdates=false;}
    })();
  }
  function sharedUpdate(device){
    if(!updateBoard||updateBoard.broker!==updateNetwork())return null;
    return updateBoard.marks?.[device||updateBoard.self];
  }
  function updateInProgress(mark){
    return mark?.state==='waiting'||mark?.state==='queued'||mark?.state==='updating';
  }
  function updateMark(peer){
    const device=peer?.id||'', shared=sharedUpdate(device);
    const own=peer?(server?.updates||[]).find(x=>x.computer===device&&!x.stale):view?.local_update;
    const pending=peer?(peer.update_pending?{state:'queued',id:peer.update_request_id}:null):
      {state:view?.pending_update,id:view?.pending_update_id};
    const reply=own?.request_id?{state:own.state,id:own.request_id,version:own.version}:null;
    const final=reply&&!updateInProgress(reply);
    // The notice and update watch arrive independently. An exact reply
    // supersedes a lagging snapshot, without closing a different request.
    if(reply?.id&&reply.id===shared?.id){
      if(final||reply.state==='updating'&&(shared.state==='legacy'||updateInProgress(shared)))return reply;
    }
    if(shared&&shared.state!=='legacy'){
      // The initiating window can lead the broker by one exchange.
      const newer=peer&&reply?.state==='waiting'&&reply.id!==shared.id&&Date.parse(own.updated_at)>=shared.at*1000;
      if(!newer){
        if(updateInProgress(shared)){
          if(shared.state==='waiting'&&Date.now()-shared.at*1000>=900000)return null;
          return shared;
        }
        if(peer||!pending.id||shared.id===pending.id)return shared;
      }
    }
    if(peer&&updateInProgress(reply))return reply;
    if(updateInProgress(pending)&&(!final||reply.id!==pending.id))return pending;
    return final?reply:null;
  }
  const updateCancelAttempts=new Map();
  function cancelUpdateKey(broker,device,id){return JSON.stringify([broker,device,id]);}
  function updatePresentation(peer,mark){
    const broker=updateNetwork(),device=peer?.id||'';
    const key=cancelUpdateKey(broker,device,mark.id);
    const attempt=updateCancelAttempts.get(key);
    const queued=server?.update_cancellations?.some(r=>r.device===device&&r.request_id===mark.id);
    const installing=mark.state==='updating';
    if(!installing&&(attempt||queued)){
      const caption=attempt==='sending'?tr('Отправляем запрос на отмену…'):
        queued||attempt==='queued'?tr('Отмена запрошена, ждём подтверждения'):tr('Проверяем, принят ли запрос на отмену…');
      return {caption,cancelling:true,canCancel:false};
    }
    const caption=installing?tr('Обновление устанавливается'):mark.state==='waiting'?tr('Обновление запрошено'):tr('Ждёт обновления');
    // The update board can confirm the queue while the roster is delayed.
    const canCancel=!installing&&!!mark.id&&(!peer||updateBuildNumber(peer.version)>=100);
    return {caption,canCancel,key,device,broker,requestID:mark.id};
  }
  async function cancelUpdate({key,device,broker,requestID}){
    if(updateCancelAttempts.has(key)||broker!==updateNetwork())return;
    updateCancelAttempts.set(key,'sending');renderMachines();
    try{
      await postAPI('/api/server-action',{action:'cancel-update',device,broker,request_id:requestID},28000);
      updateCancelAttempts.set(key,'queued');
    }catch(error){
      updateCancelAttempts.set(key,'unknown');
      addNotice('cancel-'+requestID,'bad',tr('Отмена обновления'),capital(error.message));
    }
    renderMachines();void loadServer();
  }
  function updateControl(peer,mark){
    const state=updatePresentation(peer,mark),{caption,canCancel}=state;
    if(state.cancelling){
      const status=el('span',undefined,'am-update-icon am-update-wait am-update-cancelling');
      status.title=caption;status.setAttribute('role','img');status.setAttribute('aria-label',caption);
      status.append(updateSpinner());
      status.addEventListener('click',e=>e.stopPropagation());
      return status;
    }
    const b=el(canCancel?'button':'span',undefined,'am-update-icon am-peer-update '+(canCancel?'am-update-cancel':'am-update-wait'));
    const label=canCancel?tr('Отменить обновление'):caption;
    b.title=label;b.setAttribute('aria-label',label);
    b.append(updateSpinner());
    if(!canCancel){b.setAttribute('role','img');return b;}
    b.setAttribute('aria-description',caption);
    b.type='button';b.dataset.key='cancel-update-'+(peer?.id||'local');
    b.append(svgIcon(['M6 6l12 12M18 6L6 18']));
    b.addEventListener('keydown',e=>e.stopPropagation());
    b.addEventListener('click',e=>{e.stopPropagation();void cancelUpdate(state);});
    return b;
  }
  const notices=[],noticeSince=Date.now();
  const noticePreference='messenger-notice-dismissal',noticeDuration=8000;
  let autoCloseNotices=recall(noticePreference)==='automatic';
  const waitingUpdateNotices=new Map();
  function closedNotices(){try{return JSON.parse(localStorage.getItem('messenger-notices-closed')||'{}')||{};}catch(_){return {};}}
  // T-181: no scroll of their own; at most noticeLimit are shown and a new
  // one pushes out the oldest, which then counts as closed.
  const noticeLimit=3;
  function closeNotice(key){
    const i=notices.findIndex(n=>n.key===key);
    if(i>=0){clearTimeout(notices[i].timer);notices.splice(i,1);}
    const closed=closedNotices(),cut=Date.now()-2*24*3600*1000;closed[key]=Date.now();
    for(const k of Object.keys(closed))if(closed[k]<cut)delete closed[k];
    remember('messenger-notices-closed',JSON.stringify(closed));renderNotices();
  }
  function scheduleNoticeClose(notice){
    clearTimeout(notice.timer);
    if(!autoCloseNotices)return;
    notice.timer=setTimeout(()=>{
      const card=$('notices').querySelector('[data-key="'+CSS.escape(notice.key)+'"]');
      if(card?.matches(':hover,:focus-within'))scheduleNoticeClose(notice);
      else closeNotice(notice.key);
    },noticeDuration);
  }
  function addNotice(key,kind,title,text){
    if(notices.some(n=>n.key===key)||closedNotices()[key])return;
    const notice={key,kind,title,text:text||''};
    notices.push(notice);scheduleNoticeClose(notice);
    while(notices.length>noticeLimit)closeNotice(notices[0].key);
    renderNotices();
  }
  function collectNotices(){
    // T-129: the background program was started inside an agent's sandbox.
    if(clientInfo?.sandbox)addNotice('sandbox-'+(clientInfo.client_version||''),'bad',tr('Kotel запущен из песочницы агента'),tr('Исполнители унаследуют её: без сети и без части папок, даже с «Любыми командами». Закройте Kotel полностью и откройте его снова сами, не из агента.'));
    const names=Object.fromEntries((server?.peers||[]).map(p=>[p.id,p.name]));
    const showUpdate=(u,name,source)=>{
      const at=Date.parse(u.updated_at);
      if(!(Date.now()-at<24*3600*1000))return;
      const key='update-'+u.request_id+'-'+(u.stale?'stale':u.state);
      // A waiting row keeps its original timestamp when it becomes stale.
      const observedWaiting=waitingUpdateNotices.get(source)===u.request_id;
      if(u.state==='waiting'&&!u.stale)waitingUpdateNotices.set(source,u.request_id);
      else waitingUpdateNotices.delete(source);
      if(u.stale){
        if(at>=noticeSince||observedWaiting)addNotice(key,'bad',name+tr(' не ответил об обновлении'),tr('Проверьте, что программа там запущена, и попросите ещё раз.'));
      }else if(u.state==='done'&&at>=noticeSince-60000){
        const same=/последн/i.test(u.text||'');
        addNotice(key,same?'info':'good',same?name+tr(': уже последняя версия'):name+tr(' обновлён до ')+versionName(u.version||''),same?'':u.text||'');
      }else if(u.state==='queued'&&at>=noticeSince-60000)addNotice(key,'info',name+tr(' ждёт обновления'),u.text||tr('Программа обновится, когда задачи закончатся; до тех пор компьютер не берёт новых задач.'));
      else if(u.state==='cancelled'&&at>=noticeSince-60000)addNotice(key,'info',name+tr(': обновление отменено'),'');
      else if((u.state==='refused'||u.state==='failed')&&at>=noticeSince)addNotice(key,'bad',name+tr(' не обновлён'),capital(u.text||''));
    };
    for(const u of server?.updates||[])showUpdate(u,names[u.computer]||tr('Компьютер'),u.computer);
    if(view?.local_update)showUpdate(view.local_update,view.computer_name||tr('Этот компьютер'),'local');
  }
  const noticeIcons={good:'<path d="M8 12.5l2.7 2.7L16 9.8"/>',info:'<path d="M12 11v5M12 8h.01"/>',bad:'<path d="M12 7.5v5.5M12 16.5h.01"/>'};
  function updateNoticeTooltips(){
    for(const text of $('notices').querySelectorAll('.am-notice-text')){
      if(text.clientHeight&&text.scrollHeight>text.clientHeight)text.title=text.textContent;
      else text.removeAttribute('title');
    }
  }
  function renderNotices(){
    const box=$('notices');if(!box)return;
    box.replaceChildren();box.hidden=!notices.length;
    for(const n of notices){
      const card=el('div',undefined,'am-notice am-notice-'+n.kind);card.dataset.key=n.key;
      const icon=document.createElementNS('http://www.w3.org/2000/svg','svg');icon.setAttribute('viewBox','0 0 24 24');icon.setAttribute('aria-hidden','true');icon.classList.add('am-notice-icon');
      icon.innerHTML='<circle cx="12" cy="12" r="9"/>'+noticeIcons[n.kind];
      const body=el('div',undefined,'am-notice-body');body.append(el('span',n.title,'am-notice-title'));if(n.text)body.append(el('span',n.text,'am-small am-muted am-notice-text'));
      const close=el('button',undefined,'am-icon-btn am-close-x');close.type='button';
      close.setAttribute('aria-label',tr('Закрыть'));
      close.append(svgIcon(['M6 6l12 12M18 6L6 18']));
      close.addEventListener('click',()=>closeNotice(n.key));
      card.append(icon,body,close);
      box.append(card);
    }
    updateNoticeTooltips();
  }
  // One line per agent program, with its worker state.
  // live is false when the computer cannot be reached.
  function busyTasksByComputer(snapshot){
    const byComputer=new Map();
    if(!snapshot?.project)return byComputer;
    function add(t,computer){
      const key=computer+'\0'+t.agent;
      if(!byComputer.has(key))byComputer.set(key,[]);
      byComputer.get(key).push(t);
    }
    for(const task of snapshot.tasks||[])add(task,taskComputer(task));
    for(const peer of snapshot.shared||[]){
      for(const task of peer.tasks||[]){
        if(!activeStates.includes(task.state)||sharedLost(task,peer))continue;
        add({...sharedDisplayTask(task),shared:true},sharedRunsOn(task,peer));
      }
    }
    return byComputer;
  }
  function agentActivity(tasks){
    const active=tasks.filter(t=>activeStates.includes(t.state)&&!t.pending_launch&&!t.remote?.unaccepted&&!t.remote?.lost&&!t.remote?.silent);
    const running=active.filter(t=>t.state==='running'),waiting=active.filter(t=>t.state!=='running');
    return {running,waiting,counts:{running:running.length,waiting_owner:waiting.length}};
  }
  function sidebarTaskModel(task){
    // Shared rows already carry the worker's confirmed model in model.
    const model=task.shared?task.model:task.ran_model;
    return model?modelName(model).replace(/ Medium$/,' Med'):'';
  }
  function parallelAgentLine(computer,provider,state,running,waiting,name){
    const agent=shortNames[provider]||provider;
    const group=el('details',undefined,'am-agent-parallel am-agent-'+state);
    group.dataset.provider=provider;
    group.dataset.agentGroup=JSON.stringify([name,computer,provider]);
    group.addEventListener('click',event=>event.stopPropagation());
    const summary=el('summary'),label=el('span',agent,state==='run'?'am-agent-running-label':'');
    label.append(el('span',', '+tasksWord(running.length+waiting.length),'am-agent-count'));
    summary.append(label,svgIcon(['M6 9l6 6 6-6']));
    const list=el('ul',undefined,'am-agent-parallel-list');
    for(const task of [...running,...waiting]){
      const model=sidebarTaskModel(task),working=task.state==='running';
      const row=el('li',undefined,working?'am-agent-parallel-running':'am-agent-parallel-waiting');
      const modelLabel=model?el('span',model,'am-agent-model'):null;
      if(!splitProjects.length&&modelLabel&&task.title)modelLabel.title=task.title;
      const [text,status,,from]=taskStatus(task);
      if(working){
        const name=el('span',agent);
        if(modelLabel)name.append(' ',modelLabel);
        const timing=from&&Number.isFinite(Date.parse(from))?elapsedLabel(from,'','am-agent-status'):el('span',', '+text,status);
        row.append(name,timing);
      }else{
        row.title=text;
        row.append(modelLabel||el('span',agent),', ',el('span',tr('ждёт ответа')));
      }
      list.append(row);
    }
    group.append(summary,list);
    return group;
  }
  function agentLines(agents,coordinatorClient,live,computer,busyTasks,name=project){
    const lines=el('div',undefined,'am-machine-agents');
    const [coordProvider,coordLabel]=coordinatorClient!==null?coordinatorName(coordinatorClient):['',''];
    const providers=agents.map(a=>a.agent);
    // Only agents the owner keeps enabled are listed: a chat of an agent
    // that was paused or removed once used Kotel's tools, but listing
    // it showed the agent as if it were still there (owner, 2026-11-15).
    // A coordinator chat of a program that is not an agent stays listed.
    for(const provider of providers){
      const a=agents.find(x=>x.agent===provider);
      // Blue means a task is running (T-060). A chat that once used
      // Kotel's tools is listed but not lit: nothing tells whether it is
      // still open.
      const {running,waiting,counts}=agentActivity(busyTasks.get(computer+'\0'+provider)||[]);
      const [state,text]=live?agentState(counts):['offline',tr('Нет связи')];
      if(live&&running.length+waiting.length>1){
        lines.append(parallelAgentLine(computer,provider,state,running,waiting,name));
        continue;
      }
      // A busy agent says so to the right of its name: «работает» in blue,
      // «ждёт ответа» in yellow. A free agent is just its name.
      const line=el('div',undefined,'am-agent-line am-agent-'+state);line.dataset.provider=provider;
      line.title=(names[provider]||provider)+(a?': '+text:'');
      const identity=el('span',shortNames[provider]||provider,state==='run'?'am-agent-running-label':'');
      line.append(identity);
      if(state==='run'||state==='wait'){
        const task=running[0]||waiting[0],model=sidebarTaskModel(task),from=taskStart(task);
        if(model)identity.append(' ',el('span',model,'am-agent-model'));
        if(state==='run'&&!waitingForStartTime(task)&&Number.isFinite(Date.parse(from)))line.append(elapsedLabel(from,'','am-agent-status'));
        else line.append(el('span',state==='wait'?tr('ждёт ответа'):tr('работает'),'am-agent-status'));
      }else if(!live){
        line.append(el('span',tr('нет связи'),'am-agent-status'));
      }
      lines.append(line);
    }
    if(coordinatorClient!==null&&!coordProvider){
      const line=el('div',undefined,'am-agent-line am-agent-'+(live?'idle':'offline'));line.title=tr('Чат с инструментами Kotel');line.append(el('span',coordLabel));lines.append(line);
    }
    return lines;
  }
  // Agent reports age independently of authenticated contact with the server.
  const peerFresh=peer=>peer.age>=0&&peer.age<=45;
  const peerConnected=peer=>peerFresh(peer)||(Number.isFinite(peer.contact_age)&&peer.contact_age>=0&&peer.contact_age<=45);
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
  function clearMachineDrag(){
    draggedMachine='';
    clearMachineDrop();
    machineList.querySelector('.am-machine-drag-image')?.classList.remove('am-machine-drag-image');
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
    const card=event.target.closest('[draggable=true]')?.closest('[data-machine-order]');
    if(!card)return;
    draggedMachine=card.dataset.machineOrder;
    event.dataTransfer.setData('text/plain',draggedMachine);
    event.dataTransfer.effectAllowed='move';
    if(taskSidebarPreview){
      // The disclosure is a transparent button over the visible heading.
      const head=card.querySelector('.am-machine-name');
      const rect=head.getBoundingClientRect();
      head.classList.add('am-machine-drag-image');
      event.dataTransfer.setDragImage(head,event.clientX-rect.left,event.clientY-rect.top);
    }
  });
  machineList.addEventListener('dragover',event=>{
    if(!draggedMachine)return;
    let card=event.target.closest('[data-machine-order]'),gapAfter;
    if(!card&&taskSidebarPreview){
      const cards=[...machineList.children].filter(node=>node.dataset.machineOrder!==draggedMachine);
      card=cards.find(node=>event.clientY<node.getBoundingClientRect().top);
      gapAfter=!card;
      card||=cards.at(-1);
    }
    clearMachineDrop();
    if(!card||card.dataset.machineOrder===draggedMachine)return;
    event.preventDefault();
    event.dataTransfer.dropEffect='move';
    const target=taskSidebarPreview?card.querySelector('.am-machine-name'):card;
    const rect=target.getBoundingClientRect(),after=gapAfter??event.clientY>=rect.top+rect.height/2;
    card.dataset.dropEdge=after?'after':'before';
    machineDrop={card,after};
  });
  machineList.addEventListener('dragleave',event=>{
    if(machineList.contains(event.relatedTarget))return;
    const rect=machineList.getBoundingClientRect();
    if(!event.relatedTarget&&event.clientX>=rect.left&&event.clientX<rect.right&&
        event.clientY>=rect.top&&event.clientY<rect.bottom)return;
    clearMachineDrop();
  });
  machineList.addEventListener('drop',event=>{
    if(!draggedMachine||!machineDrop)return;
    event.preventDefault();
    const from=draggedMachine,{card,after}=machineDrop;
    clearMachineDrag();
    moveMachine(from,card.dataset.machineOrder,after);
  });
  machineList.addEventListener('dragend',()=>{
    clearMachineDrag();renderMachines();
  });
  // T-271: the computers that have a card in the current render.
  let pickedComputers=new Set();
  function pickable(card,id,orderKey=id){
    pickedComputers.add(id);
    card.dataset.machineOrder=orderKey;
    if(taskSidebarPreview)return;
    const split=splitProjects.length>0;
    card.tabIndex=0;
    if(!split){
      card.setAttribute('role','button');card.setAttribute('aria-pressed',String(computerFilter===id));
      card.title=computerFilter===id?tr('Показать все компьютеры'):tr('Показать только этот компьютер');
      card.addEventListener('click',event=>{if(!event.target.closest('button'))filterComputer(id);});
    }
    card.addEventListener('keydown',event=>{
      if(event.target!==card)return;
      if(!split&&(event.key==='Enter'||event.key===' ')){event.preventDefault();filterComputer(id);}
    });
    enableMachineOrder(card,card);
  }
  function enableMachineOrder(card,handle){
    handle.draggable=machineOrderLoaded;
    handle.setAttribute('aria-keyshortcuts','Alt+ArrowUp Alt+ArrowDown');
    handle.addEventListener('keydown',event=>{
      if(event.target!==handle||!machineOrderLoaded||!event.altKey||!['ArrowUp','ArrowDown'].includes(event.key))return;
      event.preventDefault();
      const after=event.key==='ArrowDown',next=after?card.nextElementSibling:card.previousElementSibling;
      if(!next)return;
      const key=card.dataset.machineOrder;
      moveMachine(key,next.dataset.machineOrder,after);
      if(handle===card)machineList.querySelector('[data-machine-order="'+CSS.escape(key)+'"]')?.focus();
    });
  }
  // The window redraws every two seconds. A part whose content did not change
  // keeps its nodes: rebuilding them anyway reset hover, tooltips and
  // animations, and the window seemed to flicker (owner, 2026-11-12).
  function swapChildren(live,next){
    if(next.innerHTML!==live.innerHTML)live.replaceChildren(...next.childNodes);
  }
  const taskListScopes=new WeakMap();
  function replaceTaskRows(live,next,scope){
    const previousScope=taskListScopes.get(live);
    taskListScopes.set(live,scope);
    const clip=live.getBoundingClientRect();
    if(!taskSidebarPreview||previousScope!==scope||!clip.height||document.hidden||
      matchMedia('(prefers-reduced-motion:reduce)').matches||!live.animate){
      live.replaceChildren(...next.childNodes);
      return;
    }
    const rows=root=>new Map(Array.from(root.querySelectorAll('[data-motion-task]'),node=>[node.dataset.motionTask,node]));
    const old=rows(live),incoming=rows(next);
    const changed=old.size!==incoming.size||Array.from(old.keys()).some(key=>!incoming.has(key));
    if(!changed){live.replaceChildren(...next.childNodes);return;}
    if(live.id==='machines'){
      for(const [container,map] of [[live,old],[next,incoming]]){
        for(const head of container.querySelectorAll('[data-machine-order] > .am-machine-name')){
          map.set('computer:'+head.parentElement.dataset.machineOrder,head);
        }
      }
    }
    const visible=r=>r.height>0&&r.top<clip.bottom&&r.bottom>clip.top;
    const before=new Map();
    for(const [key,node] of old){
      const rect=node.getBoundingClientRect();
      if(!node.closest('[inert]')&&visible(rect))before.set(key,rect);
    }
    live.replaceChildren(...next.childNodes);
    const leaving=Array.from(before.keys()).some(key=>!incoming.has(key));
    const delay=leaving?90:0;
    const timing={duration:180,delay,fill:'backwards',easing:'ease-in-out'};
    const groups=new Set();
    for(const [key,node] of incoming){
      const rect=node.getBoundingClientRect();
      if(node.closest('[inert]')||!visible(rect))continue;
      const from=before.get(key);
      if(from&&Math.abs(from.top-rect.top)>1){
        node.animate([{transform:'translateY('+(from.top-rect.top)+'px)'},{transform:'translateY(0)'}],timing);
        if(node.parentElement.classList.contains('am-sidebar-task-rows')&&!node.closest('[inert]'))groups.add(node.parentElement);
      }else if(!old.has(key)){
        node.animate([{opacity:0},{opacity:1}],{duration:100,delay:delay+timing.duration,fill:'backwards'});
      }
    }
    // A translated task can cross its group's new edge until the other
    // computers settle. Disclosure clipping still applies when collapsed.
    for(const group of groups)group.animate([{overflow:'visible'},{overflow:'visible'}],{duration:delay+timing.duration});
    // Fade removed rows outside the live list so they never take up space,
    // receive focus or participate in the next render's HTML comparison.
    const layer=el('div',undefined,'am-task-exits'+(live.id==='task-list'?' am-task-list':''));
    layer.inert=true;layer.setAttribute('aria-hidden','true');
    for(const [key,rect] of before){
      if(incoming.has(key)||!old.get(key).hasAttribute('data-motion-task'))continue;
      const node=old.get(key).cloneNode(true);
      node.removeAttribute('data-motion-task');
      Object.assign(node.style,{position:'absolute',top:(rect.top-clip.top)+'px',left:(rect.left-clip.left)+'px',
        width:rect.width+'px',height:rect.height+'px'});
      layer.append(node);
    }
    if(!layer.childElementCount)return;
    Object.assign(layer.style,{top:clip.top+'px',left:clip.left+'px',width:clip.width+'px',height:clip.height+'px'});
    live.after(layer);
    const fade=layer.animate([{opacity:1},{opacity:0}],{duration:delay,easing:'ease-out'});
    const dismiss=()=>fade.cancel();
    document.addEventListener('scroll',dismiss,true);
    window.addEventListener('resize',dismiss);
    fade.onfinish=fade.oncancel=()=>{
      layer.remove();
      document.removeEventListener('scroll',dismiss,true);
      window.removeEventListener('resize',dismiss);
    };
  }
  function machineRole(label,tip){
    const tag=taskSidebarPreview
      ?el('span',', '+label,'am-muted am-inline-role')
      :el('span',label,'am-small am-muted am-machine-role');
    tag.title=tip;
    return tag;
  }
  function serverTag(){
    return machineRole(tr('сервер'),tr('На этом компьютере работает сервер сети'));
  }
  function combinedMachineRoles(serverRole,coordinatorRole){
    const group=el('span',undefined,'am-machine-roles');
    serverRole.append(',');
    for(const [form,coordinator] of [['full',coordinatorRole],['short',machineRole(tr('коорд.'),coordinatorRole.title)]]){
      const option=el('span',undefined,'am-machine-role-option');option.dataset.form=form;
      option.append(serverRole.cloneNode(true),' ',coordinator);
      group.append(option);
    }
    const icons=el('span',undefined,'am-machine-role-option am-machine-role-icons');icons.dataset.form='icons';
    const roleIcon=(label,tip,paths)=>{
      const icon=el('span');icon.setAttribute('role','img');icon.setAttribute('aria-label',label);icon.title=tip;
      icon.append(svgIcon(paths));return icon;
    };
    icons.append(
      roleIcon(tr('сервер'),serverRole.title,['M4 4h16v6H4zM4 14h16v6H4zM8 7h.01M8 17h.01']),
      roleIcon(tr('координатор'),coordinatorRole.title,['M9 3h6v6H9zM3 15h6v6H3zM15 15h6v6h-6zM12 9v3M6 15v-3h12v3'])
    );
    group.append(icons);
    const compact=el('span',undefined,'am-machine-role-option am-machine-role-icons');compact.dataset.form='mark';
    compact.append(roleIcon(tr('сервер')+', '+tr('координатор'),serverRole.title+'\n'+coordinatorRole.title,
      ['M9 3h6v6H9zM3 15h18v6H3zM12 9v6M7 18h.01']));
    group.append(compact);
    return group;
  }
  function layoutMachineRoles(head){
    if(!head.clientWidth)return;
    const [presence,name,roles,meta]=head.children;
    const style=getComputedStyle(head),gap=parseFloat(style.getPropertyValue('--am-space-1'));
    const insets=2*parseFloat(style.getPropertyValue('--am-space-3'));
    const available=head.clientWidth-insets;
    const range=document.createRange();
    range.selectNodeContents(name);
    const nameWidth=range.getBoundingClientRect().width;
    const markWidth=presence.getBoundingClientRect().width;
    const room=available-markWidth-parseFloat(getComputedStyle(presence).marginInlineEnd)
      -(meta?.getBoundingClientRect().width||0)-(meta?3:2)*gap;
    const [full,short,icons]=roles.children;
    const form=full.getBoundingClientRect().width<=room-nameWidth?'full':
      short.getBoundingClientRect().width<=room-nameWidth?'short':
      icons.getBoundingClientRect().width<=room?'icons':'mark';
    if(roles.dataset.form!==form)roles.dataset.form=form;
  }
  const machineRolesObserver=new ResizeObserver(()=>requestAnimationFrame(()=>{
    const head=machineList.querySelector('.am-machine-roles')?.parentElement;
    if(head)layoutMachineRoles(head);
  }));
  const serverIconPaths=['M4 3h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2ZM4 14h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2ZM6 7h1M6 18h1'];
  function machinePresence(online,local=false,lastSeen=''){
    const mark=el('span',undefined,'am-machine-presence'+(online?' online':''));
    const lastOnline=!online&&lastSeen?stamp(lastSeen):'';
    const connection=online?tr('На связи'):lastOnline?tr('Нет связи. Был на связи ')+lastOnline:tr('Нет связи');
    mark.title=local?tr('Этот компьютер')+' — '+connection:connection;
    mark.setAttribute('role','img');
    mark.setAttribute('aria-label',mark.title);
    if(taskSidebarPreview){
      const paths=online?serverIconPaths:['M6 3h14a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-6M9 11H4a2 2 0 0 1-2-2V5M19 14h1a2 2 0 0 1 2 2v2M18 22H4a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h6M6 18h1M2 2l20 20'];
      mark.append(svgIcon(paths));
      return mark;
    }
    const signal=el('span',undefined,'am-machine-signal');
    signal.setAttribute('aria-hidden','true');
    // With the seven-unit round stroke, both diagonals span the 24-unit view box.
    if(!online)signal.append(svgIcon(['M3.5 3.5l17 17M20.5 3.5l-17 17']));
    const glyph=['M4 4h16a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2ZM12 17v3M8 20h8'];
    mark.append(svgIcon(glyph),signal);
    return mark;
  }
  function currentTasksByComputer(){
    const computers=new Map();
    for(const name of sidebarProjects()){
      const snapshot=taskListView(name);
      if(!snapshot)continue;
      const {active,planned,attention}=taskGroups(snapshot,false);
      for(const item of [...attention,...active,...planned]){
        const computer=item.peer?sharedRunsOn(item.t,item.peer):taskComputer(item.t);
        if(!computers.has(computer))computers.set(computer,[]);
        computers.get(computer).push({...item,name});
      }
    }
    return computers;
  }
  function sidebarTaskCount(total){
    const count=el('span',String(total),'am-sidebar-task-count am-queue-count');
    count.setAttribute('aria-hidden','true');
    return count;
  }
  function sidebarTaskRows(body,head,card,computer,items=[],projectGroup=''){
    const key=projectGroup?JSON.stringify([projectGroup,computer]):computer;
    if(!items.length)return;
    const rows=el('div',undefined,'am-sidebar-task-rows');
    body.append(rows);
    const disclosure=el('button',undefined,'am-sidebar-disclosure');disclosure.type='button';
    disclosure.dataset.sidebarFocus=JSON.stringify([key]);
    if(head.title)disclosure.title=head.title;
    const label=head.querySelector('.am-machine-label'),count=tasksWord(items.length);
    const countLabel=sidebarTaskCount(items.length);
    function update(){
      const collapsed=collapsedComputers.has(key);
      body.inert=collapsed;
      if(collapsed)label.after(countLabel);
      else countLabel.remove();
      disclosure.setAttribute('aria-expanded',String(!collapsed));
      const action=collapsed?tr('Показать текущие задачи'):tr('Скрыть текущие задачи');
      disclosure.setAttribute('aria-label',computerName(computer)+(collapsed?', '+count:'')+' — '+action);
    }
    disclosure.addEventListener('click',()=>{
      if(collapsedComputers.has(key))collapsedComputers.delete(key);
      else collapsedComputers.add(key);
      update();
    });
    head.append(disclosure);card.classList.add('am-sidebar-has-tasks');update();
    if(groupSelect.value==='nested'){
      appendProjectFolders(rows,items,computer);
    }else{
      appendSidebarRows(rows,items);
    }
  }
  function appendProjectFolders(container,items,computer,appendRows=appendSidebarRows){
    const folders=new Map();
    for(const item of items){
      if(!folders.has(item.name))folders.set(item.name,[]);
      folders.get(item.name).push(item);
    }
    for(const [name,tasks] of folders){
      const key=JSON.stringify([computer,name]);
      const folder=el('div',undefined,'am-sidebar-folder');
      const button=el('button',undefined,'am-folder-heading');button.type='button';
      // Keep the tab in place. The open front extends past the rear wall.
      const icon=svgIcon(['M2 20V5a2 2 0 0 1 2-2h5l3 3h5','']);
      const front=icon.lastElementChild;
      button.append(icon,el('span',projectTitle(name)));
      button.dataset.sidebarFocus='folder:'+key;
      const rows=el('div',undefined,'am-sidebar-task-rows');
      const count=sidebarTaskCount(tasks.length);
      function showFolder(expanded){
        front.setAttribute('d',expanded
          ? 'M17 6a2 2 0 0 1 2 2v3M2 20l2.7-7.5Q5.2 11 6.8 11h13.8Q22 11 22 12.3q0 .4-.2.9l-2.7 7.2Q18.6 22 17 22H4a2 2 0 0 1-2-2Z'
          : 'M17 6h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2');
        rows.hidden=!expanded;
        count.hidden=expanded;
        button.setAttribute('aria-expanded',String(expanded));
        if(expanded)button.removeAttribute('aria-description');
        else button.setAttribute('aria-description',tasksWord(tasks.length));
      }
      button.append(count);
      showFolder(!collapsedFolders.has(key));
      button.addEventListener('click',()=>{
        const expand=rows.hidden;
        if(expand)collapsedFolders.delete(key);else collapsedFolders.add(key);
        showFolder(expand);
      });
      const openProjectMenu=(x,y)=>{
        if(!view?.projects.some(item=>item.name===name))return;
        popMenu(x,y,[
          {text:tr('Настройки проекта'),pick:()=>{if(project!==name)selectProject(name);show('project');}},
          {text:tr('Удалить локальный проект'),pick:()=>askRemoveProject(name)},
        ]);
      };
      button.addEventListener('contextmenu',event=>{
        event.preventDefault();
        openProjectMenu(event.clientX,event.clientY);
      });
      button.addEventListener('keydown',event=>{
        if(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')){
          event.preventDefault();
          const bounds=button.getBoundingClientRect();
          openProjectMenu(bounds.left,bounds.bottom);
        }
      });
      appendRows(rows,tasks,name);
      folder.append(button,rows);container.append(folder);
    }
  }
  function appendSidebarRows(rows,items){
    for(const {t,peer,name} of items){
      const row=el('button',undefined,'am-sidebar-task');row.type='button';
      const selection=peer?{peer:peer.peer,id:t.id}:null;
      row.dataset.sidebarFocus=JSON.stringify([name,peer?.peer||'',t.id]);
      row.dataset.motionTask=row.dataset.sidebarFocus;
      const selected=paneActive?!selection&&paneActive.project===name&&paneActive.id===t.id:
        name===project&&(selection?selectedShared?.peer===selection.peer&&selectedShared?.id===selection.id:t.id===selectedTask);
      row.setAttribute('aria-pressed',String(!draftOpen&&selected));
      const title=el('span',undefined,'am-sidebar-task-title');
      if(groupSelect.value==='project'){
        const machine=peer?sharedComputer(t,peer):computerName(taskComputer(t))||t.remote?.name||taskComputer(t);
        title.append(el('span',machine+', ','am-muted'));
      }else if(groupSelect.value==='computer'&&!sidebarProject)title.append(el('span',projectTitle(name)+', ','am-muted'));
      title.append(t.title);
      const meta=el('span',undefined,'am-small am-muted am-sidebar-task-meta am-task-model-line');
      const details=el('span');
      const model=sidebarTaskModel(peer?{...t,shared:true}:t);
      const identity=(shortNames[t.agent]||t.agent)+(model?' '+model:'');
      const from=taskStart(t);
      const [status,color,tip,timer]=peer?sharedStatus(t,peer):taskStatus(t);
      if(color==='am-status-run'){
        details.append(el('span',identity,color));
        if(timer&&Number.isFinite(Date.parse(from)))details.append(elapsedLabel(from,', '));
      }else{
        const detail=el('span',status,color);
        if(tip)detail.title=tip;
        details.append(identity+', ',detail);
      }
      meta.append(providerIcon(t.agent),details);
      title.title=title.textContent;
      meta.title=meta.textContent;
      row.append(title,meta);
      row.addEventListener('click',()=>{
        if(tab!=='tasks')show('tasks');
        if(paneBoard&&!selection)openTaskPane(t,name,taskPanes.get(t.id)?.placement||'tab');
        else selectProject(name,selection?'':t.id,'',selection);
      });
      if(!peer){
        row.draggable=true;
        row.ondragstart=event=>startPaneDrag(event,t,name);
        row.ondragend=finishPaneDrag;
      }
      row.addEventListener('contextmenu',event=>taskMenu(event,t,peer,name));
      rows.append(row);
    }
  }
  function peerInstalledVersion(peer){
    const mark=updateMark(peer),shared=sharedUpdate(peer.id);
    const confirmed=mark?.state==='done'?mark.version:shared?.state==='done'?shared.version:'';
    return confirmed&&(!peer.version||versionOlder(peer.version,confirmed))?confirmed:peer.version;
  }
  function peerUpdateOffer(peer,installed,mark){
    const newest=newestVersion();
    return peerFresh(peer)&&!updateInProgress(mark)&&installed&&versionOlder(installed,newest)?versionName(newest):'';
  }
  function confirmPeerUpdate(peer,newest){
    askConfirm(tr('Обновить ')+(peer.name||tr('компьютер'))+tr(' до ')+newest+'?',tr('Программа на том компьютере обновится и перезапустится. Задачи и история сохранятся.')+(peer.host?' '+tr('На нём работает сервер сети: пока он перезапускается, компьютеры не видят друг друга. Если там попросят подтверждение, сервер не вернётся, пока кто-нибудь не ответит у того компьютера.'):''),tr('Обновить'),()=>requestUpdate(peer),true);
  }
  function peerUpdateButton(peer,installed){
    const mark=updateMark(peer);
    if(updateInProgress(mark))return updateControl(peer,mark);
    const newest=peerUpdateOffer(peer,installed,mark);
    if(!newest)return null;
    const label=tr('Обновить версию');
    const b=el('button',undefined,'am-update-icon am-peer-update');
    b.type='button';b.title=label;b.setAttribute('aria-label',label);
    appendUpdateGlyphs(b);
    b.addEventListener('click',event=>{event.stopPropagation();confirmPeerUpdate(peer,newest);});
    b.addEventListener('keydown',event=>event.stopPropagation());
    return b;
  }
  function renderLocalUpdateMark(container,mark){
    const pending=container.querySelector('[data-network-mark]');
    if(!updateInProgress(mark)){pending?.remove();return;}
    const next=updateControl(null,mark);
    next.dataset.networkMark='';next.dataset.request=mark.id||'';next.dataset.network=updateNetwork();
    if(!pending)container.prepend(next);
    else if(!pending.isEqualNode(next))pending.replaceWith(next);
  }
  function renderMachines(){
    if(draggedMachine)return;
    collectNotices();
    pickedComputers=new Set();
    const live=$('machines'),box=live.cloneNode(false),busyTasks=busyTasksByComputer(view?.project===project?view:null);
    const sidebarTasks=taskSidebarPreview?currentTasksByComputer():null;
    const projectFirst=['project','project-computer'].includes(groupSelect.value);
    const projectActivity=splitProjects.map(name=>{
      const snapshot=view?.project===name?view:auxActivity?.project===name?auxActivity:null;
      return {name,snapshot,tasks:busyTasksByComputer(snapshot)};
    });
    const scope=JSON.stringify([splitProjects.length?null:project,updateNetwork()]);
    const sameScope=live.dataset.agentScope===scope;
    const expanded=sameScope?new Set([...live.querySelectorAll('.am-agent-parallel[open]')].map(group=>group.dataset.agentGroup)):new Set();
    const focus=live.contains(document.activeElement)?document.activeElement:null;
    const focused=sameScope?focus?.closest('.am-agent-parallel')?.dataset.agentGroup:'';
    const focusedCell=focus?.dataset.projectCell;
    const focusedSidebar=focus?.dataset.sidebarFocus;
    live.dataset.agentScope=scope;
    const machine=el('div',undefined,'am-machine-card');machine.setAttribute('aria-current','true');pickable(machine,'local');
    // This computer's version and update button are moved into its header
    // by placeLocalParts, alongside the same status icon as on remote cards.
    const head=el('div',undefined,'am-machine-name');
    const localName=el('span',view?.computer_name||tr('Этот компьютер'),'am-machine-label');
    head.append(machinePresence(online,true),localName);
    // The computer that runs the network's server is labelled as such.
    const serverRole=server?.host?.running&&server.host.scope!=='local'?serverTag():null;
    const coordinator=view?.project===project?view?.coordinator:null;
    const coordinatorProjects=(taskSidebarPreview?sidebarProjects().map(name=>({name,snapshot:taskListView(name)})):projectActivity)
      .filter(({snapshot})=>snapshot?.coordinator?.active).map(({name})=>projectTitle(name));
    let coordinatorRole=null;
    if(coordinatorProjects.length){
      coordinatorRole=machineRole(tr('координатор'),tr('На этом компьютере открыт чат-координатор')+': '+coordinatorProjects.join(', '));
    }else if(!projectActivity.length&&coordinator?.active){
      coordinatorRole=machineRole(tr('координатор'),tr('На этом компьютере открыт чат-координатор выбранного проекта'));
    }
    if(taskSidebarPreview)localName.append(...[coordinatorRole,serverRole].filter(Boolean));
    else if(serverRole&&coordinatorRole)head.append(combinedMachineRoles(serverRole,coordinatorRole));
    else if(serverRole||coordinatorRole)head.append(serverRole||coordinatorRole);
    const version=el('div',undefined,'am-machine-version am-local-version');
    const corner=el('div',undefined,'am-card-update am-local-update');
    const localMark=updateMark(null),localBusy=updateInProgress(localMark);
    if(taskSidebarPreview)renderLocalUpdateMark(document.querySelector('.am-version-row'),localMark);
    if(localBusy&&!taskSidebarPreview)corner.append(updateControl(null,localMark));
    root.classList.toggle('am-local-update-pending',localBusy);
    const localMeta=el('div',undefined,'am-machine-meta');localMeta.append(corner,version);
    // A paused agent is left out, as other computers leave it out of what
    // they report (owner, 2026-11-18: two of three agents were off and the
    // Mac still listed all three). One still running a task stays until it
    // ends.
    if(sidebarTasks){
      const body=el('div',undefined,'am-sidebar-tasks');
      head.append(localMeta);machine.append(head,body);
      if(!projectFirst)sidebarTaskRows(body,head,machine,'local',sidebarTasks.get('local'));
    }else if(projectActivity.length)appendProjectCells(machine,head,localMeta,'local',projectActivity);
    else{
      const body=el('div',undefined,'am-machine-body');
      const shownAgents=(view?.agents||[]).filter(a=>!a.paused||busyTasks.get('local\0'+a.agent)?.some(t=>activeStates.includes(t.state)));
      const lines=agentLines(shownAgents,coordinator?.connected?(coordinator.client||''):null,online,'local',busyTasks);
      if(!lines.children.length)lines.append(el('p',online?tr('Агенты не подключены'):tr('Нет связи'),'am-small am-muted'));
      body.append(lines);
      head.append(localMeta);machine.append(head,body);
    }
    head.tabIndex=0;
    head.addEventListener('contextmenu',event=>{
      event.preventDefault();popMenu(event.clientX,event.clientY,[],null,'local');
    });
    head.addEventListener('keydown',event=>{
      if(event.target!==head||!(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')))return;
      event.preventDefault();
      const bounds=head.getBoundingClientRect();
      popMenu(bounds.left,bounds.bottom,[],null,'local');
    });
    box.append(machine);
    // The other computers of this computer's network, with their agents.
    // Only the computer that runs the server can revoke one, from the
    // right-click menu.
    const hosted=new Set((server?.host?.devices||[]).filter(d=>!d.revoked_at).map(d=>d.id));
    for(const peer of server?.peers||[]){
      const fresh=peerFresh(peer),connected=peerConnected(peer);
      const card=el('div',undefined,'am-machine-card am-peer-card');card.dataset.peer=peer.id;if(peer.host)card.dataset.host='';pickable(card,peer.id,peerOrderKey(peer));
      const name=el('div',undefined,'am-machine-name');
      const peerName=el('span',peer.name,'am-machine-label');
      const presence=machinePresence(connected,false,peer.last_seen);
      if(!connected)name.title=presence.title;
      if(taskSidebarPreview&&!connected&&peer.last_seen){
        const seen=new Date(peer.last_seen);
        if(!Number.isNaN(seen.getTime())){
          const date=seen.toLocaleDateString(amLocale,{day:'numeric',month:'short'});
          peerName.append(el('span',', '+tr('был в сети {time} {date}').replace('{time}',clock(peer.last_seen)).replace('{date}',date),'am-machine-last-seen'));
        }
      }
      name.append(presence,peerName);
      if(peer.host)(taskSidebarPreview?peerName:name).append(serverTag());
      const meta=el('div',undefined,'am-machine-meta');
      const corner=el('div',undefined,'am-card-update');
      // T-128: an older computer gets the update icon next to its dot; the
      // owner confirms in a popup.
      if(!taskSidebarPreview){
        const installed=peerInstalledVersion(peer),control=peerUpdateButton(peer,installed);
        if(control){corner.append(control);meta.append(corner);}
        if(installed)meta.append(el('span',versionName(installed),'am-version am-machine-version'));
      }
      const peerBody=el('div',undefined,'am-machine-body');
      if(revokeAsk===peer.id&&hosted.has(peer.id)){
        name.append(meta);card.append(name,peerBody);
        peerBody.append(el('p',tr('Отозвать устройство? Вернуть его можно только новым приглашением.'),'am-small am-muted'));
        const actions=el('div',undefined,'am-actions am-device-confirm');
        const yes=el('button',tr('Отозвать'),'am-danger');yes.type='button';yes.dataset.key='revoke-yes-'+peer.id;
        yes.addEventListener('click',()=>{revokeAsk='';serverAction({action:'revoke',device:peer.id},'host','');});
        const no=el('button',tr('Отмена'));no.type='button';
        no.addEventListener('click',()=>{revokeAsk='';renderMachines();$('machines').querySelector('[data-key="'+CSS.escape('revoke-'+peer.id)+'"]')?.focus();});
        actions.append(no,yes);peerBody.append(actions);box.append(card);continue;
      }
      if(sidebarTasks){
        peerBody.className='am-sidebar-tasks';
        if(!projectFirst)sidebarTaskRows(peerBody,name,card,peer.id,sidebarTasks.get(peer.id));
      }else if(projectActivity.length){
        appendProjectCells(card,name,meta,peer.id,projectActivity,peer);
      }else if(fresh){
        const plines=agentLines(peer.agents||[],peer.coordinator?peer.coordinator:null,true,peer.id,busyTasks);
        if(!plines.children.length)plines.append(el('p',tr('Агенты не подключены'),'am-small am-muted'));
        peerBody.append(plines);
      }else if(connected){
        peerBody.append(el('p',tr('Ждём сведения об агентах'),'am-small am-muted'));
      }else{
        peerBody.append(el('p',peer.last_seen?tr('Был на связи ')+stamp(peer.last_seen):tr('Ещё не выходил на связь'),'am-small am-muted'));
      }
      if(!projectActivity.length){name.append(meta);card.append(name,peerBody);}
      const can={revoke:hosted.has(peer.id)};
      card.classList.add('am-device-card');
      if(can.revoke)card.dataset.key='revoke-'+peer.id;
      const menuTarget=taskSidebarPreview?name:card;
      menuTarget.tabIndex=0;
      if(!menuTarget.title)menuTarget.title=tr('Правая кнопка мыши — действия');
      card.addEventListener('contextmenu',event=>{if(!event.defaultPrevented){event.preventDefault();deviceMenu(peer,event.clientX,event.clientY,can);}});
      menuTarget.addEventListener('keydown',event=>{if(event.target===menuTarget&&(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10'))){event.preventDefault();const r=menuTarget.getBoundingClientRect();deviceMenu(peer,r.left+16,r.top+24,can);}});
      box.append(card);
    }
    const rank=new Map(machineOrder.map((key,index)=>[key,index]));
    const ranks=card=>rank.get(card.dataset.machineOrder)??machineOrder.length;
    box.append(...[...box.children].sort((a,b)=>ranks(a)-ranks(b)));
    if(taskSidebarPreview&&projectFirst){
      const cards=[...box.children];
      box.replaceChildren();
      const appendRows=groupSelect.value==='project-computer'?(rows,tasks,name)=>{
        const groups=new Map();
        for(const item of tasks){
          const computer=item.peer?sharedRunsOn(item.t,item.peer):taskComputer(item.t);
          if(!groups.has(computer))groups.set(computer,[]);
          groups.get(computer).push(item);
        }
        const roster=new Map(cards.map(card=>[card.dataset.peer||'local',card]));
        const order=new Set([...roster.keys(),...groups.keys()]);
        for(const computer of order){
          const items=groups.get(computer);
          if(!items)continue;
          const card=el('div',undefined,'am-machine-card am-sidebar-project-computer');
          card.dataset.computer=computer;
          const original=roster.get(computer);
          const heading=original?.querySelector('.am-machine-name').cloneNode(true)||el('div',undefined,'am-machine-name');
          heading.querySelector('.am-machine-meta')?.remove();
          heading.removeAttribute('tabindex');
          if(!original){
            const {t,peer}=items[0];
            const label=(peer?sharedComputer(t,peer):computerName(computer)||t.remote?.name)||computer;
            heading.append(machinePresence(false,false,t.remote?.last_seen),el('span',label,'am-machine-label'));
          }
          const body=el('div',undefined,'am-sidebar-tasks');
          sidebarTaskRows(body,heading,card,computer,items,name);
          function openMenu(x,y){
            const peer=server?.peers?.find(item=>item.id===computer);
            if(peer)deviceMenu(peer,x,y,{revoke:false});
            else if(computer==='local')popMenu(x,y,[],null,'local');
          }
          heading.addEventListener('contextmenu',event=>{
            event.preventDefault();
            openMenu(event.clientX,event.clientY);
          });
          heading.addEventListener('keydown',event=>{
            if(event.key!=='ContextMenu'&&!(event.shiftKey&&event.key==='F10'))return;
            event.preventDefault();
            const bounds=heading.getBoundingClientRect();
            openMenu(bounds.left,bounds.bottom);
          });
          card.append(heading,body);
          rows.append(card);
        }
      }:appendSidebarRows;
      appendProjectFolders(box,Array.from(sidebarTasks.values()).flat(),'',appendRows);
      if(!box.childElementCount)box.append(el('p',tr('Нет текущих задач'),'am-small am-muted'));
    }else if(taskSidebarPreview){
      for(const card of box.children){
        const head=card.querySelector('.am-machine-name');
        const handle=head.querySelector('.am-sidebar-disclosure')||head;
        if(handle===head){
          head.tabIndex=0;
          head.dataset.sidebarFocus=JSON.stringify([card.dataset.machineOrder]);
        }
        enableMachineOrder(card,handle);
      }
    }
    for(const group of box.querySelectorAll('.am-agent-parallel'))group.open=expanded.has(group.dataset.agentGroup);
    // The moved elements are left out of the comparison, so an unchanged
    // list keeps its nodes; before a rebuild they go back to the bottom row,
    // or they would leave the page with the old card.
    const copy=live.cloneNode(true);
    for(const id of localParts)copy.querySelector('#'+id)?.remove();
    // The measured layout is local to the live card and is not a data change.
    copy.querySelector('.am-machine-roles')?.removeAttribute('data-form');
    if(copy.innerHTML!==box.innerHTML){
      machineRolesObserver.disconnect();
      if(!taskSidebarPreview)parkLocalParts();
      replaceTaskRows(live,box,JSON.stringify([scope,sidebarProject,computerFilter,groupSelect.value]));
      if(!taskSidebarPreview&&serverRole&&coordinatorRole){
        machineRolesObserver.observe(head);
        if(!projectActivity.length)machineRolesObserver.observe(localMeta);
      }
      if(focusedSidebar)live.querySelector('[data-sidebar-focus="'+CSS.escape(focusedSidebar)+'"]')?.focus({preventScroll:true});
      else if(focused)live.querySelector('[data-agent-group="'+CSS.escape(focused)+'"] > summary')?.focus({preventScroll:true});
      else if(focusedCell)live.querySelector('[data-project-cell="'+CSS.escape(focusedCell)+'"]')?.focus({preventScroll:true});
    }
    placeLocalParts();
    renderDeviceMenuFooter();
    // T-271: only the highlighted card shows the chosen computer, so a
    // computer that left the list no longer filters anything.
    if(computerFilter&&!pickedComputers.has(computerFilter))queueMicrotask(()=>{if(computerFilter&&!pickedComputers.has(computerFilter))clearComputer();});
  }
  function appendProjectCells(card,identity,meta,computer,activity,peer){
    card.classList.add('am-machine-pair');
    const status=el('div',undefined,'am-machine-name');status.append(meta);
    const headings=[identity,status];
    // The roster is per computer. Only the task activity belongs to a project.
    // A paused agent with work in either column stays visible in both.
    const agents=(peer?peer.agents||[]:view?.agents||[]).filter(a=>!a.paused||activity.some(({tasks})=>
      tasks.get(computer+'\0'+a.agent)?.some(t=>activeStates.includes(t.state))));
    for(const [index,{name,snapshot,tasks}] of activity.entries()){
      const cell=el('div',undefined,'am-machine-project');
      const body=el('div',undefined,'am-machine-body');
      cell.append(headings[index],body);
      const current=project===name&&computerFilter===computer;
      cell.dataset.projectCell=JSON.stringify([computer,name]);cell.tabIndex=0;
      cell.setAttribute('role','button');cell.setAttribute('aria-pressed',String(current));
      cell.setAttribute('aria-label',projectTitle(name)+' — '+computerName(computer));
      const choose=()=>{
        if(project===name){filterComputer(computer,true);return;}
        if(tab!=='stats'&&tab!=='agents')show('tasks');
        selectProject(name,'',computer);
      };
      cell.addEventListener('click',event=>{
        if(!event.target.closest('button'))choose();
      });
      cell.addEventListener('keydown',event=>{
        if(event.target===cell&&(event.key==='Enter'||event.key===' ')){event.preventDefault();event.stopPropagation();choose();}
      });
      let message='';
      if(!online)message=tr('Нет связи');
      else if(peer&&!peerFresh(peer))message=peerConnected(peer)?tr('Ждём сведения об агентах'):tr('Нет связи');
      else{
        const coord=peer?null:snapshot?.coordinator;
        const lines=agentLines(agents,coord?.connected?(coord.client||''):null,true,computer,tasks,name);
        if(!snapshot&&lines.children.length){
          cell.title=auxError===name?tr('Статусы недоступны'):tr('Загружаем статусы…');
          for(const line of lines.children)line.removeAttribute('title');
        }
        if(lines.children.length)body.append(lines);
        else message=tr('Агенты не подключены');
      }
      if(message)body.append(el('p',message,'am-small am-muted'));
      card.append(cell);
    }
  }
  // The grouped sidebar keeps local updates in the footer. The earlier
  // layout places them in the local card when the window is wide enough.
  const localParts=['app-installed-version','app-update-button'];
  const narrowSidebar=matchMedia('(max-width:620px)');
  function parkLocalParts(){
    const row=document.querySelector('.am-version-row');
    row.prepend(...localParts.map(id=>$(id)).filter(node=>node.parentNode!==row));
  }
  function placeLocalParts(){
    const card=$('machines').querySelector('[aria-current="true"]');
    if(taskSidebarPreview||narrowSidebar.matches||!card){parkLocalParts();return;}
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
  const projectComputers=new Map();
  // On the statistics and agents tabs the choice filters that tab (T-076);
  // elsewhere it opens the tasks.
  function filterComputer(id,projectTile=false){
    const before=statsProjectScope();
    if(tab==='stats'&&projectTile)statsProject=project;
    computerFilter=computerFilter===id?'':id;
    // T-096: a new computer starts with all its agents, so an agent chosen
    // earlier cannot hide what runs there.
    if($('agent-filter').value){$('agent-filter').value='';syncSelects();}
    if(computerFilter&&tab!=='stats'&&tab!=='agents')show('tasks');
    computerChanged(before);
  }
  function clearComputer(){computerFilter='';computerChanged();}
  // T-279: with paged history the finished tasks of the new choice come
  // from the program, so the list and its top task wait for that answer;
  // the previous list and conversation stay until then instead of flashing
  // a filtered remainder and an empty pane.
  let topTaskScope='';
  function computerChanged(before=statsProjectScope()){
    renderMachines();
    if(tab==='stats'&&!updateStatsProjectScope(before))reloadStats();
    if(tab==='agents')renderAgents();
    loadProjectActivity();
    if(view?.history_paged){topTaskScope=taskWindowQuery();refresh();return;}
    if(tab==='tasks')openTopTask();else renderTaskList();
  }
  function applyTopTaskScope(){
    if(!topTaskScope||topTaskScope!==windowHistoryScope)return;
    topTaskScope='';
    if(tab!=='tasks'){renderTaskList();return;}
    const g=taskGroups();
    openTopTask(!!(g.active.length||g.planned.length||g.attention.length||g.done.length));
    renderTaskList();
  }
  function computerName(id){
    return id==='local'?(view?.computer_name||tr('Этот компьютер')):(server?.peers||[]).find(p=>p.id===id)?.name||(view?.tasks||[]).find(t=>taskComputer(t)===id)?.remote?.name||'';
  }
  function computerOptions(){
    return [['local',computerName('local')],...(server?.peers||[]).map(peer=>[peer.id,peer.name||peer.id])];
  }
  const taskHistories=new Map();
  let windowHistoryScope='';
  function taskWindowQuery(name=project){return new URLSearchParams({project:name,agent:$('agent-filter').value,computer:computerFilter}).toString();}
  function combinedTasks(){return splitProjects.length===2&&!computerFilter;}
  function auxTaskQuery(){return tab==='tasks'&&combinedTasks()?taskWindowQuery(passiveProject()):'';}
  function taskListView(name){
    if(name===project&&view?.project===name)return view;
    if(taskSidebarPreview){
      const entry=sidebarSnapshots.get(name);
      return entry?.scope===taskWindowQuery(name)?entry.snapshot:null;
    }
    if(name===passiveProject()&&auxActivity?.project===name&&auxTaskScope&&auxTaskScope===auxTaskQuery())return auxActivity;
    return null;
  }
  function currentTaskHistory(snapshot){
    if(!snapshot)return null;
    const history=taskHistories.get(snapshot.project);
    return history?.scope===taskWindowQuery(snapshot.project)?history:null;
  }
  function syncTaskHistory(snapshot){
    const history=currentTaskHistory(snapshot);
    if(history&&(history.first!==(snapshot.history_next||'')||history.version!==snapshot.history_version))taskHistories.delete(snapshot.project);
  }
  function taskRows(snapshot=view){
    const recent=snapshot?.tasks||[],history=currentTaskHistory(snapshot);
    if(!history?.rows.size)return recent;
    const visible=new Map(history.rows);
    for(const t of recent)visible.set(t.id,t);
    return [...visible.values()];
  }
  async function loadOlderTasks(name){
    const snapshot=taskListView(name),scope=taskWindowQuery(name);
    if(!snapshot||(name===project&&windowHistoryScope!==scope))return;
    let history=currentTaskHistory(snapshot);
    if(!history){
      const first=snapshot.history_next||'';
      history={scope,rows:new Map(),first,next:first,version:snapshot.history_version,loading:false};
      taskHistories.set(name,history);
    }
    if(history.loading||!history.next)return;
    history.loading=true;renderTaskList();
    try{
      const q=new URLSearchParams(scope);q.set('before',history.next);
      const page=await api('/api/task-history?'+q);
      if(currentTaskHistory(taskListView(name))!==history)return;
      for(const t of page.tasks||[])history.rows.set(t.id,t);
      history.next=page.next||'';
    }catch(error){if(currentTaskHistory(taskListView(name))===history)showError(error);}
    finally{history.loading=false;if(currentTaskHistory(taskListView(name))===history)renderTaskList();}
  }
  // The task list's groups, newest first: own tasks and other computers'
  // tasks of the project together (T-047, T-110).
  function taskGroups(snapshot=view,filtered=true){
    const agentFilter=filtered?$('agent-filter').value:'',computer=filtered?computerFilter:'';
    const items=taskRows(snapshot).filter(t=>(!agentFilter||t.agent===agentFilter)&&(!computer||taskComputer(t)===computer)).map(t=>({t}));
    for(const p of snapshot?.shared||[]){
      for(const t of p.tasks){
        if(computer&&computer!==sharedRunsOn(t,p))continue;
        if(!agentFilter||t.agent===agentFilter)items.push({t,peer:p});
      }
    }
    items.sort((x,y)=>String(y.t.created_at).localeCompare(String(x.t.created_at)));
    const groups={active:[],planned:[],attention:[],done:[]};
    for(const it of items){
      if(needsDecision(it))groups.attention.push(it);
      else if(it.t.state==='running'&&!it.t.started_at&&(it.t.pending_launch||it.t.remote?.unaccepted)&&!(it.peer&&sharedWorkerRunning(it.t)))groups.planned.push(it);
      else if(activeStates.includes(it.t.state))groups.active.push(it);
      else groups.done.push(it);
    }
    return groups;
  }
  // An open task whose outcome this computer cannot know: sent to a computer
  // out of contact or silent about it, or another computer's task while that
  // computer is out of contact.
  function needsDecision(it){
    // Blocked and interrupted tasks hold their computer (it will not update)
    // until someone closes them (D-120).
    if((!it.peer||taskSidebarPreview)&&(it.t.state==='blocked'||it.t.state==='interrupted'))return true;
    if(it.peer)return sharedLost(it.t,it.peer);
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
      viewSignature='';await refresh();loadProjectActivity();
      if(failed.length)showError(new Error(tr('Не закрылись: ')+failed.length+'. '+failed[0]));
    });
  }
  function askClearDone(batches){
    const agent=$('agent-filter').value,computer=computerFilter;
    const count=batches.reduce((total,{items})=>total+items.length,0);
    const text=batches.some(({snapshot})=>snapshot.history_paged)
      ?tr('Завершённые задачи без отметки архива исчезнут из списка по текущему фильтру, включая ещё не загруженные. История сохранится.')
      :tr('Из списка на этом компьютере пропадут завершённые задачи: ')+count+tr('. Задачи этого компьютера пропадут и у других компьютеров; задачи других компьютеров останутся у них.');
    const action=taskSidebarPreview?tr('Очистить архив'):tr('Убрать все');
    const heading=taskSidebarPreview?tr('Очистить архив?'):tr('Убрать все завершённые задачи?');
    askConfirm(heading,text,action,async()=>{
      const failed=[];
      for(const {items,snapshot} of batches){
        const paged=snapshot.history_paged;
        if(paged){
          try{
            await postAPI('/api/task-hide-history',{project:snapshot.project,agent,computer});
            taskHistories.delete(snapshot.project);
            if(items.some(it=>!it.peer&&it.t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}
          }catch(e){failed.push(e.message);}
        }
        for(const it of items){
          if(paged&&!it.peer)continue;
          try{await postAPI('/api/task-hide',{peer:it.peer?.peer||'',task:it.t.id});}
          catch(e){failed.push(e.message);}
        }
      }
      viewSignature='';await refresh();loadProjectActivity();
      if(failed.length)showError(new Error(tr('Не убрались: ')+failed.length+'. '+failed[0]));
    });
  }
  // Owner, 2026-11-12: choosing a computer opens its newest active task, or
  // its newest task; with no computer chosen, the top task of the list.
  // On refresh (quiet) the caller loads the conversation itself, and an
  // empty list changes nothing.
  function openTopTask(quiet){
    const g=taskGroups(view,!taskSidebarPreview),it=g.active[0]||g.planned[0]||[...g.attention,...g.done].sort((x,y)=>String(y.t.created_at).localeCompare(String(x.t.created_at)))[0];
    if(!it){if(quiet)return;selectedTask='';selectedShared=null;feed=null;feedSignature='';renderTaskList();renderThread();return;}
    if(it.peer){selectedShared={peer:it.peer.peer,id:it.t.id};selectedTask='';feed=null;feedSignature='';renderTaskList();renderThread();startWindowWatch();return;}
    if(selectedTask===it.t.id&&!selectedShared)return;
    selectedTask=it.t.id;selectedShared=null;feed=null;feedSignature='';renderTaskList();if(!quiet){renderThread();loadFeed();}
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
  function appendArchiveTask(row,t,name,computer,status,peer){
    row.dataset.motionTask=JSON.stringify([name,peer?.peer||'',t.id]);
    const time=appendTaskWhen(row,t,true);
    const computerDetail=tr('Компьютер')+': '+computer;
    row.setAttribute('aria-description',computerDetail);
    if(time)time.title=[computerDetail,time.title].filter(Boolean).join('\n');
    else if(!row.title)row.title=computerDetail;
    const title=el('span',undefined,'am-task-title am-sidebar-task-title');
    if(!sidebarProject)title.append(el('span',projectTitle(name)+', ','am-muted'));
    title.append(t.title);
    const model=peer?(t.model?modelName(t.model):''):taskModelLabel(t);
    const author=(shortNames[t.agent]||t.agent)+(model?' '+model:'');
    const tokens=peer?t.tokens:t.usage&&t.usage.input_tokens+t.usage.output_tokens;
    const [text,color,tip]=status;
    const meta=el('span',undefined,'am-archive-task-meta am-muted am-task-model-line');
    const details=el('span');
    const identity=el('span',author);
    if(!peer)identity.title=taskModelDetail(t);
    const duration=t.state==='done'
      ?el('span',minutes(taskStart(t),t.finished_at||t.updated_at),'am-task-usage')
      :el('span',text,color);
    if(tip)duration.title=tip;
    details.append(identity,', ',duration);
    if(t.archived)details.append(', ',tr('В архиве'));
    if(tokens!=null){
      const usage=el('span',tokensText(tokens)+tr(' т.'),'am-task-usage');
      usage.title=peer?tokensFull(tokens):usageLong(t.usage);
      details.append(', ',usage);
    }
    meta.append(providerIcon(t.agent),details);
    title.title=title.textContent;
    meta.title=meta.textContent;
    row.classList.add('am-archive-task');
    row.append(title,meta);
  }
  function renderTaskList(){
    const live=$('task-list'),box=live.cloneNode(false);
    // While another project loads, its rows stay in place but cannot be
    // chosen, so the list does not collapse and grow again.
    if(view?.project!==project){if(!live.childElementCount)$('task-count').textContent='';$('archive-clear').disabled=true;live.inert=true;live.setAttribute('aria-busy','true');return;}
    live.inert=false;live.removeAttribute('aria-busy');
    // T-096: say when the agent filter hides active tasks, with a way back.
    const agentChosen=$('agent-filter').value;
    const hidden=!taskSidebarPreview&&agentChosen?(view?.tasks||[]).filter(t=>t.agent!==agentChosen&&activeStates.includes(t.state)&&(!computerFilter||taskComputer(t)===computerFilter)).length:0;
    if(hidden){
      const all=el('button',undefined,'am-task am-task-hidden');all.type='button';
      all.append(el('span',tr('Фильтр «')+(names[agentChosen]||agentChosen)+tr('» скрывает незавершённые задачи: ')+hidden,'am-task-agent'),el('span',tr('Показать всех агентов'),'am-status-run'));
      all.addEventListener('click',()=>{
        const filter=$('agent-filter');filter.value='';syncSelects();filter.dispatchEvent(new Event('change'));
      });
      box.append(all);
    }
    const projects=taskSidebarPreview?sidebarProjects():combinedTasks()?splitProjects:[project],combined=projects.length>1,menu=$('device-menu');
    const snapshots=projects.map(taskListView).filter(Boolean);
    const ownTasks=snapshots.flatMap(snapshot=>taskRows(snapshot));
    let archived=0;
    const archiveBatches=[],archivePages=[];
    if(!taskSidebarPreview)$('task-count').textContent=ownTasks.length+snapshots.reduce((n,snapshot)=>n+(snapshot.shared||[]).reduce((count,p)=>count+p.tasks.length,0),0)||'';
    if(menu?.dataset.stopTask&&menu.matches(':popover-open')){
      const task=ownTasks.find(t=>t.id===menu.dataset.stopTask);
      const text=task&&canStopTask(task)?stopMessage(stopProgress(task)):'';
      if(!text)menu.hidePopover();
      else if(menu.firstElementChild.textContent!==text)menu.firstElementChild.textContent=text;
    }
    // Another computer's task is read-only and says which computer gave it.
    const ownRow=(t,name)=>{
      const row=el('button',undefined,'am-task');row.type='button';row.setAttribute('aria-pressed',String(!draftOpen&&name===project&&t.id===selectedTask));
      if(taskSidebarPreview){
        const computer=t.remote&&!t.remote.from?(t.remote.name||t.remote.id):computerName('local');
        appendArchiveTask(row,t,name,computer,taskStatus(t),null);
      }else{
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
      }
      row.addEventListener('click',()=>{
        if(name!==project){selectProject(name,t.id,'');return;}
        if(draftOpen)closeTaskDraft();
        selectedTask=t.id;selectedShared=null;feedSignature='';renderTaskList();renderThread();loadFeed();if(taskSidebarPreview)renderMachines();
      });
      row.addEventListener('contextmenu',e=>taskMenu(e,t,null,name,taskSidebarPreview));
      return row;
    };
    const sharedRow=(t,p,name)=>{
      const row=el('button',undefined,'am-task am-task-shared');row.type='button';row.title=tr('Задачу дал компьютер «')+p.name+tr('». Здесь её можно только посмотреть');
      row.setAttribute('aria-pressed',String(!draftOpen&&name===project&&selectedShared?.peer===p.peer&&selectedShared?.id===t.id));
      // T-111: another computer's task opens as a read-only card.
      row.addEventListener('click',()=>{
        const selection={peer:p.peer,id:t.id};
        if(name!==project){selectProject(name,'','',selection);return;}
        if(draftOpen)closeTaskDraft();
        selectedShared=selection;selectedTask='';feed=null;feedSignature='';renderTaskList();renderThread();startWindowWatch();if(taskSidebarPreview)renderMachines();
      });
      row.addEventListener('contextmenu',e=>taskMenu(e,t,p,name));
      if(taskSidebarPreview)appendArchiveTask(row,t,name,sharedComputer(t,p),sharedStatus(t,p),p);
      else{
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
      }
      return row;
    };
    const section=(target,name,label,items,action)=>{
      if(!items.length)return;
      if(label){
        const head=el('div',undefined,'am-label am-task-section');head.append(el('span',label));
        if(action)head.append(action);
        target.append(head);
      }
      for(const it of items)target.append(it.peer?sharedRow(it.t,it.peer,name):ownRow(it.t,name));
    };
    function appendHistoryButton(target,pages){
      const loading=pages.some(page=>page.loading);
      const label=pages.length===1?tr('Показать ещё 10'):tr('Показать ещё');
      const older=el('button',loading?tr('Загружаем…'):label,'am-task-section-action');
      const names=pages.map(page=>page.name);
      older.type='button';older.disabled=loading;older.dataset.projects=JSON.stringify(names);
      older.addEventListener('click',()=>Promise.all(names.map(loadOlderTasks)));
      const more=el('div',undefined,'am-label am-task-section');more.append(older);target.append(more);
    }
    for(const name of projects){
      const snapshot=taskListView(name);
      const target=combined?el('section',undefined,'am-task-project'):box;
      if(combined){
        target.dataset.project=name;target.setAttribute('aria-label',projectTitle(name));
        if(!taskSidebarPreview){
          target.append(el('h3',projectTitle(name),'am-label am-task-section'));
          box.append(target);
        }
      }
      if(!snapshot){
        if(combined){
          target.append(el('p',auxError===name?tr('Проекты недоступны'):tr('Загружаем…'),'am-small am-muted'));
          if(taskSidebarPreview)box.append(target);
        }
        continue;
      }
      const groups=taskGroups(snapshot),before=target.children.length;
      const actions=(items,label,pick)=>{
        if(!items.length)return null;
        const button=el('button',label,'am-task-section-action');button.type='button';
        button.addEventListener('click',pick);return button;
      };
      if(taskSidebarPreview){
        const done=groups.done.filter(({t})=>finishedStates.includes(t.state));
        archived+=done.length;
        const clearable=done.filter(({t})=>!t.archived);
        if(clearable.length)archiveBatches.push({items:clearable,snapshot});
        section(target,name,'',done);
      }else{
        section(target,name,tr('Активные'),groups.active);
        section(target,name,tr('Запланированные'),groups.planned);
        const closableItems=groups.attention.filter(closable);
        section(target,name,tr('Требуют решения'),groups.attention,actions(closableItems,tr('Закрыть все'),()=>askCloseAll(closableItems)));
        const clearable=groups.done.filter(({t})=>!t.archived);
        section(target,name,tr('Завершённые'),groups.done,actions(clearable.length>1?clearable:[],tr('Убрать все'),()=>askClearDone([{items:clearable,snapshot}])));
      }
      const history=currentTaskHistory(snapshot);
      if(snapshot.history_paged&&(name!==project||windowHistoryScope===taskWindowQuery())&&(history?history.next:snapshot.history_next)){
        if(taskSidebarPreview)archivePages.push({name,loading:!!history?.loading});
        else appendHistoryButton(target,[{name,loading:!!history?.loading}]);
      }
      if(taskSidebarPreview){
        if(combined&&target.children.length>before)box.append(target);
      }else if(target.children.length===before)target.append(el('p',name?(combined||computerFilter||$('agent-filter').value?tr('Здесь задач нет'):tr('Задач пока нет. Когда координатор раздаст первые, они появятся здесь.')):'','am-small am-muted'));
    }
    if(taskSidebarPreview){
      if(archivePages.length)appendHistoryButton(box,archivePages);
      $('task-count').textContent=archived||'';
      if(!box.childElementCount)box.append(el('p',tr('В архиве пока нет задач'),'am-small am-muted'));
      const clear=$('archive-clear');
      clear.hidden=false;
      clear.disabled=!archiveBatches.length;
      clear.onclick=archiveBatches.length?()=>askClearDone(archiveBatches):null;
    }
    if(live.innerHTML!==box.innerHTML)replaceTaskRows(live,box,JSON.stringify([projects,agentChosen,computerFilter]));
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
    heading.append(el('p',tr('Подключение координатора к Kotel'),'am-small am-muted'));
    const steps=el('div',undefined,'am-steps');
    steps.append(coordinatorAgentRow('claude',view.coordinator_link||{state:'unknown'}));
    steps.append(coordinatorAgentRow('codex',view.codex_link||{state:'unknown'}));
    steps.append(coordinatorAgentRow('cursor',view.cursor_link||{state:'unknown'}));
    const manual=el('details',undefined,'am-tool');manual.append(el('summary',tr('Подключить вручную через Терминал')));
    const claudeCmd='cd '+shellQuote(view.folder)+' && claude mcp add --scope local kotel -- '+shellQuote(view.messenger)+' --root '+shellQuote(view.data_root)+' coordinator --project '+shellQuote(view.project);
    const codexToml='[mcp_servers.kotel]\ncommand = '+JSON.stringify(view.messenger)+'\nargs = ["--root", '+JSON.stringify(view.data_root)+', "coordinator", "--project", '+JSON.stringify(view.project)+']\ndefault_tools_approval_mode = "approve"\ntool_timeout_sec = 180\n';
    const block=(title,text)=>{
      const copy=el('button',tr('Скопировать команду'));copy.type='button';
      copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(text);copy.textContent=tr('Скопировано');}catch(_){copy.textContent=tr('Скопируйте вручную');}setTimeout(()=>copy.textContent=tr('Скопировать команду'),2000);});
      const wrap=el('div');wrap.append(el('p',title,'am-small am-muted'),el('pre',text,'am-command'),copy);return wrap;
    };
    const cursorJSON=JSON.stringify({mcpServers:{kotel:{type:'stdio',command:view.messenger,args:['--root',view.data_root,'coordinator','--project',view.project],env:{AGENT_MESSENGER_WORKER_RUN:'${env:AGENT_MESSENGER_WORKER_RUN}'}}}},null,2);
    for(const [agent,title,example] of [
      ['claude','Claude Code',claudeCmd],
      ['codex','Codex (.codex/config.toml)',codexToml],
      ['cursor','Cursor (.cursor/mcp.json)',cursorJSON]
    ]){
      const state=coordinatorLink(agent)?.state;
      if(state==='missing'||state==='no_'+agent)manual.append(block(title,example));
    }
    manual.hidden=!view.folder||manual.childElementCount===1;
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
      let sub=agent==='cursor'?tr('Записано. Откройте новый чат Cursor в этой папке и разрешите подключение Kotel'):tr('Подключён. Откройте папку проекта в новом чате');
      if(agent==='claude')sub=tr('Откройте эту папку в локальном сеансе Claude Code. Проверьте Kotel в списке MCP.');
      if(link.server_name==='messenger')sub=tr('В проекте сохранено подключение «messenger». Для существующего чата оставьте это имя.');
      if(agent==='codex'&&(view.agents||[]).some(a=>a.agent==='codex'&&a.trust?.state==='not_trusted'))sub=tr('Записано. Подтвердите доверие папке кнопкой «Доверять» на карточке Codex во вкладке «Агенты».');
      return stepRow('ok',title,sub,null);
    }
    if(failure)return stepRow('no',title,tr('Не получилось: ')+failure.detail,[tr('Повторить'),()=>connectCoordinator(agent)]);
    if(link.state===missing)return stepRow(1,title,tr('Сначала настройте его во вкладке «Агенты»'),[tr('Агенты'),()=>show('agents')]);
    if(link.state==='no_folder')return stepRow(1,title,tr('Сначала укажите папку проекта'),null);
    if(link.state==='other')return stepRow('no',title,tr('Запись MCP относится к другому подключению. Проверьте настройки агента.'),null);
    if(link.state==='off')return stepRow('no',title,tr('Подключение MCP выключено в настройках агента'),null);
    if(link.state==='unknown')return stepRow('no',title,(agent==='cursor'?tr('Не удалось прочитать настройки Cursor'):agent==='codex'?tr('Не удалось прочитать настройки Codex'):tr('Не удалось прочитать настройки Claude'))+(link.detail?': '+link.detail:''),connect);
    if(agent==='claude'&&link.state==='missing')return stepRow(1,title,tr('MCP не подключён для этой папки в Claude Code'),connect);
    return stepRow(1,title,tr('Не подключён'),connect);
  }
  // Whether this computer's Claude Code is the project's coordinator. Other
  // computers are not shown: any computer may have its own (owner, T-152).
  function coordinatorPlace(){
    return view?.coordinator?.connected||coordinatorAgentOn('claude')||coordinatorAgentOn('codex')||coordinatorAgentOn('cursor');
  }
  // First-run setup links to the same settings used during normal work.
  function welcome(){
    const box=el('section',undefined,'am-settings am-welcome');
    const intro=el('div',undefined,'am-welcome-intro');
    intro.append(el('h2',tr('Добро пожаловать в Kotel'),'am-welcome-title'),
      el('p',tr('Настройте проект и добавьте агента, чтобы отправить первую задачу.'),'am-muted'));
    const steps=el('div',undefined,'am-card am-steps am-welcome-steps');
    const hasProject=!!view.project,hasFolder=hasProject&&!!view.folder;
    const projectAction=hasProject?[tr('Указать папку'),()=>show('project')]:[tr('Создать проект'),openProjectDialog];
    steps.append(stepRow(hasFolder?'ok':1,tr('Проект'),
      hasFolder?projectTitle(view.project):hasProject?tr('Укажите папку проекта на этом компьютере.'):tr('Задайте название и выберите папку проекта.'),
      hasFolder?null:projectAction));

    const agents=view.agents||[];
    const localModels=taskDraftModels({id:'',agents});
    steps.append(stepRow(localModels.length?'ok':2,tr('Агенты на этом компьютере'),
      localModels.length?tr('Модель для задачи доступна.'):tr('Настройте агента и включите хотя бы одну модель.'),
      localModels.length?null:[tr('Настроить агентов'),()=>show('agents')]));

    const peers=welcomePeerCount();
    steps.append(stepRow(peers?'ok':3,tr('Другие компьютеры'),
      peers?tr('В сети: ')+peers:tr('По желанию подключите другие компьютеры через сервер.'),
      peers?null:[tr('Настроить подключение'),()=>show('server')]));

    const place=hasProject&&coordinatorPlace();
    const who=[coordinatorAgentOn('claude')&&'Claude Code',coordinatorAgentOn('codex')&&'Codex',coordinatorAgentOn('cursor')&&'Cursor'].filter(Boolean).join(', ');
    const codexBare=coordinatorAgentOn('codex')&&!coordinatorAgentOn('claude')&&!coordinatorAgentOn('cursor')&&agents.some(a=>a.agent==='codex'&&a.trust?.state==='not_trusted');
    const coordSub=codexBare?tr('Записано. Подтвердите доверие папке кнопкой «Доверять» на карточке Codex во вкладке «Агенты».'):
      place?(who?tr('Подключён: ')+who:tr('Подключён на этом компьютере')):
      tr('По желанию подключите чат, который будет распределять задачи.');
    steps.append(stepRow(place&&!codexBare?'ok':4,tr('Координатор'),coordSub,
      !hasProject||place&&!codexBare?null:codexBare?[tr('Проверить доверие'),()=>show('agents')]:[tr('Настроить координатора'),()=>show('project')]));
    box.append(intro,steps);
    return box;
  }
  function showComposerAccess(node,enabled,remote='',compact=false){
    node.hidden=enabled==null&&!remote;
    const remoteDetail=tr('Доступ задаётся на {computer}').replace('{computer}',()=>remote);
    const label=remote?(compact?tr('Доступ на {computer}').replace('{computer}',()=>remote):remoteDetail):enabled?tr('Любые команды'):tr('Обычный доступ');
    const detail=remote?(compact?remoteDetail:tr('Разрешения задаются на компьютере исполнителя.')):enabled?label:tr('Используются обычные разрешения агента.');
    node.classList.toggle('am-full-access',!remote&&enabled===true);
    node.title=node.hidden?'':detail;
    node.replaceChildren();
    if(node.hidden)return;
    const icon=svgIcon(['M12 2.25 3.75 6v5.25c0 4.5 3 7.5 8.25 10.5 5.25-3 8.25-6 8.25-10.5V6z']);
    node.append(icon,el('span',label));
  }
  function taskDraftComputers(name){
    const local={id:'',name:view?.computer_name||tr('Этот компьютер'),agents:view?.agents||[]};
    return [...((view?.projects||[]).find(p=>p.name===name)?.folder&&!['queued','updating'].includes(view.local_update?.state)?[local]:[]),
      ...(server?.peers||[]).filter(p=>peerFresh(p)&&!p.update_pending&&p.ready?.accept!==false&&
        (!Array.isArray(p.ready?.projects)||p.ready.projects.includes(name)))].filter(p=>p.agents?.some(a=>!a.paused));
  }
  // Menu order is a presentation choice. Keep custom IDs and saved defaults intact.
  const modelFamilies=['auto','composer','grok','luna','terra','sol','astra','haiku','sonnet','opus','fable'];
  const effortLevels=['','minimal','low','medium','high','xhigh','max','ultra'];
  const modelCollator=new Intl.Collator('en',{numeric:true});
  function orderedModels(ids){
    const entries=Array.from(ids,id=>{
      const family=id.split(/[-[]/).find(word=>modelFamilies.includes(word));
      const effort=id.match(/(?:\[effort=|-)(minimal|low|medium|high|xhigh|max|ultra)(?:\]|-fast)?$/);
      const base=(effort?id.slice(0,effort.index):id).replace(/^cursor-/,'').replace(/-fast$/,'');
      return {id,base,family:family?modelFamilies.indexOf(family):modelFamilies.length,effort:effortLevels.indexOf(effort?.[1]||'')};
    });
    return entries.sort((a,b)=>a.family-b.family||modelCollator.compare(a.base,b.base)||
      a.effort-b.effort||modelCollator.compare(a.id,b.id)).map(entry=>entry.id);
  }
  function taskDraftModels(computer){
    const usesCoordinatorModels=computer?.version==='dev'||
      (/^(?:0\.0\.0|[1-9]\d*\.\d+\.\d+)-preview\.\d+$/.test(computer?.version)&&updateBuildNumber(computer.version)>=82);
    return (computer?.agents||[]).filter(a=>!a.paused).flatMap(a=>{
      const agent=names[a.agent]||a.agent;
      const configured=view?.coordinator_models?.[a.agent]??a.models??[];
      const available=computer.id&&!usesCoordinatorModels
        ?configured.filter(id=>a.models?.includes(id)):configured;
      return orderedModels(available)
        .map(model=>[a.agent+':'+model,agent+' '+modelName(model)]);
    });
  }
  function canDraftTask(name){
    return online&&taskDraftComputers(name).some(computer=>taskDraftModels(computer).length);
  }
  function renderTaskDraft(preferredProject){
    const projectSelect=$('task-draft-project'),computerSelect=$('task-draft-computer'),modelSelect=$('task-draft-model');
    const projects=(view?.projects||[]).map(p=>[p.name,p.title||p.name]);
    const requested=typeof preferredProject==='string'?preferredProject:projectSelect.value;
    const name=projects.some(([key])=>key===requested)?requested:sidebarProject||project;
    projectSelect.dataset.selectedProject=name;
    projectOptions(projectSelect,[...projects,['__new',tr('Открыть папку…')]],name,draftSending||!online);
    const computers=taskDraftComputers(name);
    const current=computerSelect.selectedIndex<0?null:computers.find(p=>p.id===computerSelect.value);
    const computer=current||computers.find(p=>taskDraftModels(p).length)||computers[0];
    projectOptions(computerSelect,computers.map(p=>[p.id,p.name]),computer?.id||'',draftSending||!computer);
    const models=taskDraftModels(computer);
    const providerName=models[0]?.[0].split(':')[0];
    const choices=models.map(([key])=>key).filter(key=>key.startsWith(providerName+':'));
    const preferred=choices.find(key=>key===providerName+':'+preferredTaskModels[providerName])||
      choices.find(key=>/\bmedium\b/.test(key))||choices[0]||'';
    const model=models.some(([key])=>key===modelSelect.value)?modelSelect.value:preferred;
    projectOptions(modelSelect,models,model,draftSending||!models.length);
    const provider=model.split(':',1)[0];
    const agent=computer?.agents.find(a=>a.agent===provider);
    const access=$('task-draft-access');
    const route=JSON.stringify([name,computer?.id,provider]);
    if(access.dataset.route!==route){access.dataset.route=route;access.value='';}
    access.disabled=draftSending;
    showComposerAccess(access,access.value?access.value==='full':agent&&!computer.id?!!agent.full_access:null,
      !access.value&&agent&&computer.id?(computer.name||tr('компьютере исполнителя')):'');
    const input=$('task-draft-text');
    input.readOnly=draftSending;
    $('task-draft-add').disabled=draftSending;
    $('task-draft-send').disabled=draftSending||!online||!name||!computer||!model||!input.value.trim();
    const status=$('task-draft-status');
    status.textContent=draftSending?tr('Отправляем задачу…'):draftAttempt?.error||(!computer?tr('Для этого проекта нет доступных компьютеров с агентами.'):!models.length?tr('Включите модель в настройках агента.'):!online?tr('Клиент недоступен'):'');
    status.classList.toggle('am-status-bad',!draftSending&&!!draftAttempt?.error);
    composerDictation?.sync();
    syncSelects();
  }
  $('task-draft-access').addEventListener('click',event=>{
    const button=event.currentTarget,rect=button.getBoundingClientRect();
    const choices=[['',tr('По настройкам исполнителя')],['standard',tr('Обычный доступ')]];
    const provider=$('task-draft-model').value.split(':')[0];
    if($('task-draft-computer').value||view?.agents?.some(a=>a.agent===provider&&a.full_access))
      choices.push(['full',tr('Любые команды в пределах разрешений')]);
    popMenu(rect.left,rect.bottom,choices.map(([value,text])=>({text,pick:()=>{button.value=value;renderTaskDraft();button.focus();}})));
  });
  function closeTaskDraft(){
    composerDictation?.cancel();
    draftOpen=false;
    $('task-new').setAttribute('aria-expanded','false');
    $('task-draft').hidden=true;
    if($('task-draft-add').getAttribute('aria-expanded')==='true')$('task-draft-add-menu').hidePopover();
    feedSignature='';
  }
  function openTaskDraft(focus=true){
    draftOpen=true;
    $('task-new').setAttribute('aria-expanded','true');
    $('task-draft').hidden=false;
    if(!$('task-draft-text').value)$('task-draft-project').value=sidebarProject||project;
    renderThread();
    if(focus)$('task-draft-text').focus();
    renderMachines();renderTaskList();
  }
  function setupComposerPaths(){
    const menu=$('task-draft-add-menu'),pathInput=$('task-draft-path'),manual=$('task-draft-manual-path');
    const status=$('task-draft-path-status'),insertButton=$('task-draft-insert-path'),showPath=$('task-draft-show-path');
    const draftComputer=$('task-draft-computer');
    const pickers=[['file',$('task-draft-file')],['folder',$('task-draft-folder')]];
    const isOpen=()=>menu.matches(':popover-open');
    let target,busy=false;
    function valid(current){
      if(!current||current!==target||!isOpen())return false;
      return current.isDraft?draftOpen&&!draftSending&&draftComputer.value===current.computer:
        !draftOpen&&!answering&&!$('compose').hidden&&!current.input.disabled&&selectedTask===current.task;
    }
    const place=trigger=>positionPopover(menu,trigger,manual.hidden?280:320);
    function insert(path,current){
      if(!path||!valid(current))return;
      const {input,trigger}=current;
      const addition=(input.value&&!input.value.endsWith('\n')?'\n':'')+path;
      if(input.value.length+addition.length>input.maxLength){status.textContent=tr('Путь не помещается в сообщение.');place(trigger);return;}
      input.setRangeText(addition,input.value.length,input.value.length,'end');
      menu.hidePopover();input.focus();input.dispatchEvent(new Event('input',{bubbles:true}));
    }
    for(const [trigger,input,isDraft] of [[$('task-draft-add'),$('task-draft-text'),true],[$('answer-add'),$('answer-text'),false]]){
      trigger.addEventListener('click',()=>{
        if(isOpen()){menu.hidePopover();return;}
        if(busy)return;
        target={trigger,input,isDraft,computer:draftComputer.value,task:selectedTask};
        const local=isDraft?!draftComputer.disabled&&!draftComputer.value:!selectedTaskRow()?.remote;
        $('task-draft-local-pick').hidden=!local;
        manual.hidden=true;showPath.setAttribute('aria-expanded','false');
        status.textContent='';pathInput.value='';insertButton.disabled=true;
        menu.showPopover();trigger.setAttribute('aria-expanded','true');place(trigger);
        (local?pickers[0][1]:showPath).focus({preventScroll:true});
      });
    }
    menu.addEventListener('toggle',()=>{
      if(!isOpen())target?.trigger.setAttribute('aria-expanded','false');
    });
    window.addEventListener('resize',()=>menu.hidePopover());
    draftComputer.addEventListener('change',()=>menu.hidePopover());
    showPath.addEventListener('click',()=>{
      manual.hidden=!manual.hidden;
      showPath.setAttribute('aria-expanded',String(!manual.hidden));
      place(target.trigger);
      if(!manual.hidden)pathInput.focus({preventScroll:true});
    });
    pathInput.addEventListener('input',()=>{insertButton.disabled=!pathInput.value.trim();});
    insertButton.addEventListener('click',()=>insert(pathInput.value.trim(),target));
    menu.addEventListener('keydown',event=>{
      if(event.key==='Escape'){event.preventDefault();menu.hidePopover();target.trigger.focus();}
      if(event.key==='Enter'&&event.target===pathInput){event.preventDefault();insert(pathInput.value.trim(),target);}
    });
    for(const [kind,button] of pickers)button.addEventListener('click',async()=>{
      const current=target;
      if(!valid(current))return;
      busy=true;for(const [,picker] of pickers)picker.disabled=true;
      status.textContent='';
      try{insert(await requestPath(kind,pathInput.value.trim()),current);}
      catch(error){if(valid(current)){status.textContent=pickerError(error);place(current.trigger);}}
      finally{busy=false;for(const [,picker] of pickers)picker.disabled=false;}
    });
  }
  function setupComposerTokens(){
    const trigger=$('answer-tokens'),menu=$('answer-token-menu');
    trigger.addEventListener('click',()=>{
      if(menu.matches(':popover-open')){menu.hidePopover();return;}
      menu.showPopover({source:trigger});
      positionPopover(menu,trigger,null,true);
      menu.focus({preventScroll:true});
    });
    menu.addEventListener('toggle',()=>trigger.setAttribute('aria-expanded',String(menu.matches(':popover-open'))));
    menu.addEventListener('keydown',event=>{
      if(event.key!=='Escape')return;
      event.preventDefault();event.stopPropagation();menu.hidePopover();trigger.focus();
    });
    window.addEventListener('resize',()=>menu.hidePopover());
  }
  function renderComposerTokens(task,model,available){
    const trigger=$('answer-tokens'),menu=$('answer-token-menu');
    const current=task.state==='running'?task.quiet?.live_tokens:null;
    const {used_tokens:used,capacity_tokens:capacity}=task.context_window??{};
    const hasContext=Number.isSafeInteger(used)&&used>=0&&Number.isSafeInteger(capacity)&&capacity>0;
    const hidden=!available||(!model&&!hasContext&&current==null&&!task.usage);
    if(trigger.dataset.task!==task.id||hidden)menu.hidePopover();
    trigger.dataset.task=task.id;
    trigger.hidden=hidden;
    trigger.toggleAttribute('data-context-known',hasContext);
    const progress=$('answer-context-progress');
    progress.toggleAttribute('hidden',!hasContext);
    let occupancy=tr('нет данных'),count='';
    if(hasContext){
      const fraction=Math.min(used/capacity,1);
      const percent=fraction.toLocaleString(amLocale,{style:'percent'});
      progress.setAttribute('stroke-dasharray',fraction*100+' 100');
      occupancy=tr('{percent} заполнено').replace('{percent}',percent);
      count=tr('{used} / {capacity} токенов').replace('{used}',tokensText(used)).replace('{capacity}',tokensText(capacity));
    }else if(current!=null){
      count=tr('{tokens} за текущий ход').replace('{tokens}',tokensFull(current));
    }
    $('answer-context-summary').textContent=tr('Контекстное окно')+' '+occupancy;
    if(task.agent==='cursor'&&!hasContext)$('answer-context-summary').textContent=tr('Cursor не передал замер контекстного окна.');
    const spent=$('answer-task-total');
    spent.hidden=!task.usage;
    spent.textContent=spent.hidden?'':tr('{tokens} по задаче').replace('{tokens}',tokensFull(task.usage.input_tokens+task.usage.output_tokens));
    if(hasContext&&task.state!=='running')$('answer-context-summary').textContent=tr('Последний замер контекста')+': '+occupancy;
    const details=$('answer-context-count');
    details.textContent=count;
    details.hidden=!count;
    if(menu.matches(':popover-open'))positionPopover(menu,trigger,null,true);
  }
  async function sendTaskDraft(event){
    event.preventDefault();
    if(composerDictation?.finishFor($('task-draft-text')))return;
    if(!draftOpen||$('task-draft-send').disabled)return;
    const selection=$('task-draft-model').value;
    const split=selection.indexOf(':');
    const payload={
      project:$('task-draft-project').value,computer:$('task-draft-computer').value,
      agent:selection.slice(0,split),model:selection.slice(split+1),text:$('task-draft-text').value.trim()
    };
    if($('task-draft-access').value)payload.access=$('task-draft-access').value;
    const key=JSON.stringify(payload);
    if(draftAttempt?.key!==key)draftAttempt={key,op:'window-'+crypto.randomUUID()};
    draftSending=true;renderTaskDraft();
    try{
      const {result:id}=await postAPI('/api/task-start',{...payload,op_id:draftAttempt.op},10000);
      if(typeof id!=='string'||!id)throw Error(tr('Действие не подтверждено. Проверьте состояние и повторите.'));
      draftAttempt=null;
      $('task-draft-access').value='';
      const input=$('task-draft-text');input.value='';sizeComposer(input);
      if(draftOpen){
        if(sidebarProject)sidebarProject=payload.project;
        selectProject(payload.project,id,'');
        if(tab!=='tasks')show('tasks');
      }else await refresh();
    }catch(error){
      if(draftAttempt)draftAttempt.error=error.message;
    }finally{
      draftSending=false;
      if(draftOpen)renderTaskDraft();
    }
  }
  function renderThread(){
    composerDictation?.sync();
    if(paneBoard)syncTaskPanes();
    const thread=$('thread'),head=$('task-head');
    const panelOpen=!!view?.project&&$('list-toggle').getAttribute('aria-expanded')==='true';
    $('task-panel').hidden=!panelOpen;$('task-layout').classList.toggle('am-queue-collapsed',!panelOpen);
    $('list-toggle').hidden=!view?.project||(!taskSidebarPreview&&tab!=='tasks');
    if(draftOpen){
      if($('answer-add').getAttribute('aria-expanded')==='true')$('task-draft-add-menu').hidePopover();
      $('answer-token-menu').hidePopover();
      $('compose').hidden=true;$('lost-bar').hidden=true;$('to-bottom').hidden=true;
      if(thread.dataset.draft!=='true'){
        thread.dataset.draft='true';thread.replaceChildren();thread.setAttribute('aria-busy','false');
        head.replaceChildren(conversationTitle(tr('Новая задача')));
        feedShown='';feedSignature='';
      }
      renderTaskDraft();
      return;
    }
    if(thread.dataset.draft==='true'){delete thread.dataset.draft;head.replaceChildren();}
    const sameProject=view?.project===project;
    const task=sameProject&&selectedTask&&feed?.task.id===selectedTask?selectedTaskRow():null;
    const shared=sameProject?sharedSelection():null;
    const pending=!sameProject||!!(selectedTask&&!task)||!!(selectedShared&&!shared);
    thread.setAttribute('aria-busy',String(pending));
    $('compose').inert=pending;$('lost-bar').inert=pending;
    // Keep the last complete conversation and its layout until this one is ready.
    if(pending){if($('answer-add').getAttribute('aria-expanded')==='true')$('task-draft-add-menu').hidePopover();$('answer-token-menu').hidePopover();if(!head.firstElementChild)showPane(thread,'loading');return;}
    if(task){renderFeed({...task,quiet:feed.task.quiet,context_window:feed.task.context_window});return;}
    $('answer-token-menu').hidePopover();
    $('compose').hidden=true;$('lost-bar').hidden=true;feedSignature='';$('to-bottom').hidden=true;
    feedShown='';
    if(shared){renderSharedTask(shared.t,shared.p);return;}
    head.replaceChildren();
    if(!view.project){
      showPane(thread,'welcome');return;
    }
    if(taskSidebarPreview&&tab==='tasks'&&canDraftTask(sidebarProject||project)){
      openTaskDraft(false);return;
    }
    showPane(thread,showWelcome()?'welcome':'empty');
  }
  // The welcome explains the setup until this computer has seen its first
  // task or serves the network; afterwards an empty project just says so
  // (owner, 2026-11-23).
  let welcomeDone=recall('messenger-welcome-done')==='1';
  const welcomePeerCount=()=>(server?.peers||[]).filter(peerFresh).length;
  const setupPaneShown=()=>['welcome','empty'].includes($('thread').firstElementChild?.dataset.pane);
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
  function conversationTitle(title){
    const heading=el('h3',title,'am-conversation-title');
    heading.title=title;
    return heading;
  }
  function conversationDate(t){
    let at=Date.parse(t.created_at);
    if(!Number.isFinite(at))at=Date.parse(t.started_at);
    if(!Number.isFinite(at))return null;
    const date=el('time',undefined,'am-chat-date am-small am-muted');
    date.id='conversation-date';
    const day=new Date(at);
    const options={weekday:'short',day:'numeric',month:'short'};
    if(day.getFullYear()!==new Date().getFullYear())options.year='numeric';
    date.dateTime=day.toISOString();
    date.textContent=day.toLocaleDateString(amLocale,options)+tr(' в ')+day.toLocaleTimeString(amLocale,{hour:'2-digit',minute:'2-digit'});
    return date;
  }
  function sharedTaskHeader(t,p){
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    title.append(el('p',(names[t.agent]||t.agent)+(t.model?' ('+modelName(t.model)+')':'')+tr(' на «')+sharedComputer(t,p)+tr('», задачу дал компьютер «')+p.name+'»','am-small am-muted'));
    const [statusText,statusCls,statusTip,from]=sharedStatus(t,p);
    const when=waitingForStartTime(t)?'':t.finished_at?', '+clock(taskStart(t))+'–'+clock(t.finished_at):tr(', начата в ')+clock(taskStart(t));
    const line=el('p',undefined,'am-small');line.append(statusLabel(statusText,statusCls,from),el('span',when,'am-muted'));
    if(statusTip)line.append(el('span',', '+statusTip,'am-muted'));
    if(t.tokens)line.append(el('span',', '+tokensText(t.tokens)+tr(' токенов'),'am-muted'));
    title.append(line);
    const headRow=el('div',undefined,'am-task-head-row');headRow.append(title);
    return headRow;
  }
  // Another computer's task (T-111): the window has only what that computer
  // shares, so the card shows it and says where the full history is.
  function renderSharedTask(t,p){
    t=sharedDisplayTask(t);
    const thread=$('thread'),head=$('task-head');
    head.replaceChildren(taskSidebarPreview?conversationTitle(t.title):sharedTaskHeader(t,p));
    const box=el('div',undefined,'am-settings');
    if(t.report){box.append(el('h3',tr('Начало отчёта')),el('p',t.report,'am-shared-report'));}
    else box.append(el('p',tr('Отчёта пока нет.'),'am-muted'));
    box.append(el('p',tr('Полная история задачи — в окне компьютера «')+p.name+tr('». Здесь её можно только посмотреть.'),'am-small am-muted'));
    thread.replaceChildren(box);
    if(taskSidebarPreview){const date=conversationDate(t);if(date)thread.prepend(date); }
    $('compose').hidden=true;$('lost-bar').hidden=true;
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
  function taskFileSource(t){
    if(!t)return undefined;
    return {
      scope:t.id,
      remote:t.remote&&!t.remote.from?(t.remote.name||t.remote.id||tr('Другой компьютер')):'',
      request:async(ref,action)=>{
        const response=await fetch('/api/task-file',{
          method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},
          body:JSON.stringify({task:t.id,path:ref.path,action}),cache:'no-store',signal:AbortSignal.timeout(4500),
        });
        if(response.status===502)throw outdatedMessage();
        if(!response.ok)throw Error(tr('Файл недоступен в локальной папке этой задачи.'));
        return response.json();
      },
    };
  }
  function richText(text,cls,fileSource){
    const box=el('div',undefined,'am-rich'+(cls?' '+cls:''));
    const pieces=String(text||'').split(/^```[^\n]*\n?/m);
    pieces.forEach((piece,i)=>{
      if(i%2===1){box.append(el('pre',piece.replace(/\n$/,''),'am-code-block'));return;}
      for(const para of piece.split(/\n{2,}/)){
        if(!para.trim())continue;
        const paragraph=el('p'),body=para.replace(/^\n+|\n+$/g,'');
        box.append(fileSource&&window.taskFiles?window.taskFiles.paragraph(paragraph,body,fileSource,true):inlineCode(paragraph,body));
      }
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
    }else if(['edit','multiedit','write',"изменение файлов","applied_file_change"].includes(name)){
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
    const caption=name==='applied_file_change'?label:(it.description?.trim()||label);
    return {kind,label:caption.length>120?caption.slice(0,119)+'…':caption};
  }
  // This is a display-only tokenizer. Keep the original command intact
  // for selection and copying, including quotes, spaces and line breaks.
  function highlightCommand(pre,text){
    pre.textContent='';
    const tokens=/"(?:\\[\s\S]|[^"\\])*"|'[^']*'|\$\{[^}\r\n]*\}|\$[\w?#!@*]+|[;|&()<>\n]+|[\t\r ]+|\\[^\n]|[^"'\$;|&()<>\s\\]+|[\s\S]/g;
    let commandStart=true;
    for(const [value] of text.matchAll(tokens)){
      let style;
      if(/^[;|&()\n]/.test(value))commandStart=true;
      else if(!/^\s|^[<>]/.test(value)){
        if(/^\$./.test(value))style='variable';
        else if(/^['"]/.test(value))style=commandStart?'name':'literal';
        else if(/^\d+(?:\.\d+)?$/.test(value))style='number';
        else if(commandStart)style='name';
        else if(/^-[\w-]/.test(value))style='literal';
        if(!/^[A-Za-z_]\w*=/.test(value))commandStart=false;
      }
      pre.append(style?el('span',value,'am-shell-'+style):value);
    }
  }
  function toolLine(it,action){
    const icon=svgIcon(actionKinds[action.kind].icon);
    icon.classList.add('am-action-icon');
    const row=el(it.text?'details':'div',undefined,'am-tool');
    const label=el(it.text?'summary':'div',undefined,'am-action-label');
    label.append(icon,el('span',action.label,'am-tool-name'));
    row.append(label);
    if(it.text){
      const body=el('pre',it.text);
      row.append(body);
      if(isCommandTool((it.title||'').toLowerCase()))row.addEventListener('toggle',function showSyntax(){
        if(!row.open)return;
        highlightCommand(body,it.text);
        row.removeEventListener('toggle',showSyntax);
      });
    }
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
  const messageAgeFormat=new Intl.RelativeTimeFormat(amLocale);
  function messageAge(sent,now){
    if(!Number.isFinite(sent))return '';
    const minutes=Math.max(0,Math.floor((now-sent)/60000));
    if(!minutes)return tr('Только что');
    if(minutes<60)return messageAgeFormat.format(-minutes,'minute');
    if(minutes<1440)return messageAgeFormat.format(-Math.floor(minutes/60),'hour');
    if(minutes<10080)return messageAgeFormat.format(-Math.floor(minutes/1440),'day');
    return stamp(sent);
  }
  let lastMessageAgeMinute=-1;
  function updateTimers(){
    // T-275: a hidden window draws nothing; it catches up when shown.
    if(document.hidden)return;
    const now=Date.now();
    for(const label of root.querySelectorAll('[data-elapsed-from]')){
      const text=label.dataset.elapsedPrefix+minutes(label.dataset.elapsedFrom,null,now);
      if(text!==label.textContent){
        label.textContent=text;
        const meta=label.closest('.am-sidebar-task-meta');
        if(meta)meta.title=meta.textContent;
      }
    }
    const thread=$('thread');
    for(const timer of thread.querySelectorAll('[data-command-start]')){
      timer.textContent=commandElapsed(Number(timer.dataset.commandStart));
    }
    const minute=Math.floor(now/60000);
    if(minute!==lastMessageAgeMinute){
      lastMessageAgeMinute=minute;
      for(const label of thread.querySelectorAll('.am-message-time')){
        const age=messageAge(Date.parse(label.dateTime),now);
        if(age!==label.textContent)label.textContent=age;
      }
    }
  }
  // Stop, like in a chat: a square button in the input field, or Esc.
  const stopSubmissions=new Map();
  function taskUnreachable(t){
    return !!t.remote&&!t.remote.from&&(t.state==='blocked'||t.state==='interrupted'||(activeStates.includes(t.state)&&(!!t.remote.lost||!!t.remote.silent)));
  }
  function canStopTask(t){return activeStates.includes(t.state)&&!taskUnreachable(t);}
  function stopProgress(t){return t.remote?.cancel_state||stopSubmissions.get(t.id);}
  function stopMessage(progress){
    return {
      sending:tr('Отправляем запрос на остановку…'),
      queued:tr('Запрос на остановку в очереди. Ждём завершения задачи…'),
      sent:tr('Запрос на остановку отправлен. Ждём завершения задачи.'),
    }[progress]||'';
  }
  function renderStopControls(t){
    const field=$('answer-text').parentElement,status=$('task-stop-status');
    const canStop=canStopTask(t),progress=canStop?stopProgress(t):'';
    const text=stopMessage(progress),wasHidden=field.hidden;
    $('task-stop').hidden=!canStop||!!text;
    field.hidden=!!text;
    $('answer-status').hidden=!!text;
    if(wasHidden&&!field.hidden)sizeAnswer();
    if(status.textContent!==text)status.textContent=text;
  }
  function closeHere(row=selectedTaskRow(),name=project){
    if(!row)return;
    const t=taskRows(taskListView(name)).find(task=>task.id===row.id);
    if(!t||(!taskUnreachable(t)&&!['interrupted','blocked'].includes(t.state)))return;
    if(!t.remote||t.remote.from){
      if(t.state!=='blocked'&&t.state!=='interrupted')return;
      askConfirm(tr('Закрыть задачу?'),tr('Kotel проверит, что процесса исполнителя на этом компьютере нет, и закроет задачу. Если процесс ещё есть, задача останется как была.')+(t.remote?.from?' '+tr('Компьютер «')+t.remote.name+tr('» узнает, что задача закрыта.'):''),tr('Закрыть'),async()=>{
        await postAPI('/api/task-cancel',{task:t.id});feedSignature='';await refresh();loadProjectActivity();
      });
      return;
    }
    askConfirm(tr('Закрыть задачу?'),(!t.remote.silent&&!t.remote.lost?tr('Выполнение задачи было прервано на компьютере «')+t.remote.name+'».':t.remote.silent&&!t.remote.lost?tr('Компьютер «')+t.remote.name+tr('» не отвечает по этой задаче.'):tr('С компьютером «')+t.remote.name+tr('» нет связи.'))+' '+tr('Задача закроется в этом окне, а что с ней на том компьютере, неизвестно. Если связь вернётся, ему придёт отмена.'),tr('Закрыть'),async()=>{
      await postAPI('/api/task-cancel',{task:t.id});feedSignature='';await refresh();loadProjectActivity();
    });
  }
  function stopSelectedTask(){
    if(!$('compose').hidden&&!$('task-stop').hidden)void stopTask(selectedTaskRow());
  }
  async function stopTask(row,name=project){
    if(!row)return;
    const t=taskRows(taskListView(name)).find(task=>task.id===row.id)||(name===project&&selectedTask===row.id?selectedTaskRow():null);
    if(!t||!canStopTask(t)||stopProgress(t))return;
    stopSubmissions.set(t.id,'sending');
    if(selectedTask===t.id)renderStopControls(t);
    renderTaskList();
    try{
      await postAPI('/api/task-cancel',{task:t.id});
      stopSubmissions.set(t.id,t.remote&&!t.remote.from?'queued':'sent');
      if(selectedTask===t.id)showError(null);
    }catch(error){
      if(selectedTask===t.id)showError(error);
      else addNotice('task-stop-'+t.id+'-'+Date.now(),'bad',tr('Не удалось подтвердить остановку')+' «'+t.title+'»',capital(error.message));
    }finally{
      if(stopSubmissions.get(t.id)==='sending')stopSubmissions.delete(t.id);
      if(selectedTask===t.id)renderStopControls(selectedTaskRow()||t);
      renderTaskList();
      feedSignature='';
      await refresh();loadProjectActivity();
    }
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
  let pictureCopyPending=false;
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
    $('picture-copy').disabled=true;
    $('picture-copy-status').textContent='';
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
  }
  async function copyPicture(){
    const img=$('picture-image'),button=$('picture-copy'),status=$('picture-copy-status');
    if(pictureCopyPending||!img.complete||!img.naturalWidth)return;
    const source=img.src;
    pictureCopyPending=true;
    button.disabled=true;
    status.textContent='';
    let message=tr('Скопировано');
    try{
      const canvas=document.createElement('canvas');
      canvas.width=img.naturalWidth;
      canvas.height=img.naturalHeight;
      canvas.getContext('2d').drawImage(img,0,0);
      // WebKit requires write() to start within the click, before PNG encoding finishes.
      const clipboard=navigator.clipboard,Item=window.ClipboardItem;
      if(!clipboard?.write||!Item)throw new Error('Image clipboard unavailable');
      const png=new Promise((resolve,reject)=>canvas.toBlob(blob=>{
        if(blob)resolve(blob);
        else reject(new Error('PNG encoding failed'));
      },'image/png'));
      await clipboard.write([new Item({'image/png':png})]);
    }catch(_){
      message=tr('Не удалось скопировать');
    }finally{
      pictureCopyPending=false;
      button.disabled=!img.complete||!img.naturalWidth;
      if(img.src===source)status.textContent=message;
    }
  }
  function setupPictureViewer(){
    const d=$('picture-dialog'),stage=$('picture-stage'),img=$('picture-image');
    $('picture-close').addEventListener('click',()=>d.close());
    $('picture-copy').addEventListener('click',copyPicture);
    img.addEventListener('load',()=>{$('picture-copy').disabled=pictureCopyPending;});
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
  function taskFeedHeader(t,displayModel,quiet){
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    const where=t.remote&&!t.remote.from?tr(' на компьютере «')+t.remote.name+'»':tr(' на ')+(view.computer_name||tr('этом компьютере'))+(t.remote?.from?tr(', по заданию компьютера «')+t.remote.name+'»':'');
    title.append(el('p',(names[t.agent]||t.agent)+(displayModel?' ('+displayModel+')':'')+where+tr(', папка ')+(t.folder||tr('ещё не известна'))+(t.remote&&!t.remote.from?'':t.full_access?tr(', любые команды разрешены'):tr(', без «Любых команд»')),'am-small am-muted'));
    const [statusText,statusCls,statusTip,from]=taskStatus(t);
    const when=waitingForStartTime(t)?'':t.finished_at?', '+clock(taskStart(t))+'–'+clock(t.finished_at):tr(', начата в ')+clock(taskStart(t));
    const line=el('p',undefined,'am-small');line.append(statusLabel(statusText,statusCls,from),el('span',when,'am-muted'));
    if(statusTip)line.append(el('span',', '+statusTip,'am-muted'));
    if(t.usage){const used=el('span',', '+usageShort(t.usage),'am-muted');used.title=usageLong(t.usage);line.append(used);}
    title.append(line);
    if(quiet)title.append(quiet);
    const headRow=el('div',undefined,'am-task-head-row');headRow.append(title);
    return headRow;
  }
  function fileChangeCard(files,run,fileSource){
    const card=el('section',undefined,'am-file-changes');
    const header=el('div',tr('Изменённые файлы')+' ('+files.size+')','am-file-change-row');
    const counts=(added,removed)=>{
      const pair=el('span',undefined,'am-file-counts');
      pair.append(el('span','+'+added,'am-status-good'),el('span','−'+removed,'am-status-bad'));
      return pair;
    };
    const values=[...files.values()];
    if(values.every(file=>file.counted)){
      header.append(counts(values.reduce((n,file)=>n+file.added,0),values.reduce((n,file)=>n+file.removed,0)));
    }
    card.append(header);
    const more=el('details');
    more.dataset.key='changes-'+run;
    more.append(el('summary',tr('Показать ещё')+' ('+(files.size-3)+')'));
    let index=0;
    for(const [path,file] of files){
      const row=el('div',undefined,'am-file-change-row am-small');
      row.append(fileSource&&!fileSource.remote&&window.taskFiles?window.taskFiles.link(path,path,fileSource):el('span',path));
      if(file.counted)row.append(counts(file.added,file.removed));
      (index++<3?card:more).append(row);
    }
    if(files.size>3)card.append(more);
    return card;
  }
  function renderFeed(t){
    const thread=$('thread'),head=$('task-head');
    const signature=JSON.stringify([t,feed.items,online]);if(signature===feedSignature)return;
    const count=(feed.items||[]).filter(it=>it.kind!=='command_end').length,fresh=feedShown!==t.id,atBottom=fresh||feedStick||feedPinned,grew=!fresh&&!atBottom&&count>feedCount;feedShown=t.id;feedStick=false;feedCount=count;feedSignature=signature;
    const displayModel=taskModelLabel(t),quiet=quietLine(t);
    const header=taskSidebarPreview?conversationTitle(t.title):taskFeedHeader(t,displayModel,quiet);
    const opened=new Set(fresh?[]:[...thread.querySelectorAll('details[open][data-key]')].map(d=>d.dataset.key));
    const direct=feed.items[0]?.author==='owner';
    // Keep live output visible until a verified reply for this run is available.
    const replyProgress=new Map();
    if(direct)for(const it of feed.items){
      if(it.run>0&&it.kind==='report'&&it.verified&&it.audience!=='held'&&it.text?.trim())replyProgress.set(it.run,[]);
    }
    const out=[];let group=null,tools=null;
    const appliedFiles=new Map();
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
    const messageAuthor=(name,model,role='',detail='')=>{
      const author=el('span',undefined,'am-meta-author');
      if(taskSidebarPreview&&role){
        author.append(name);
        if(model){
          const label=el('span',model);
          if(detail)label.title=detail;
          author.append(' ',label);
        }
        author.append(el('span',', '+role,'am-muted am-inline-role'));
        return author;
      }
      author.append(el('strong',name));
      if(model){
        const label=el('span',model,'am-meta-model');
        if(detail)label.title=detail;
        author.append(' ',label);
      }
      return author;
    };
    const appendMessageTime=(group,meta,at,text)=>{
      const time=el('time',taskSidebarPreview?messageAge(Date.parse(at),Date.now()):clock(at),taskSidebarPreview?'am-message-time':'');
      if(at)time.dateTime=at;
      if(taskSidebarPreview){
        time.title=connectionTimestamp(at);
        const actions=el('div',undefined,'am-message-actions');
        const copy=el('button',undefined,'am-quiet-button');copy.type='button';
        const label=tr('Скопировать сообщение');copy.title=label;copy.setAttribute('aria-label',label);
        const copyPaths=['M10.5 8h7a2.5 2.5 0 0 1 2.5 2.5v7a2.5 2.5 0 0 1-2.5 2.5h-7A2.5 2.5 0 0 1 8 17.5v-7A2.5 2.5 0 0 1 10.5 8Z','M16 8V6.5A2.5 2.5 0 0 0 13.5 4h-7A2.5 2.5 0 0 0 4 6.5v7A2.5 2.5 0 0 0 6.5 16H8'];
        copy.append(svgIcon(copyPaths));
        const status=el('span',undefined,'am-visually-hidden');status.setAttribute('role','status');
        let reset;
        copy.addEventListener('click',async()=>{
          clearTimeout(reset);
          try{
            await navigator.clipboard.writeText(text);
            status.textContent=tr('Скопировано');
            copy.dataset.copied='';
            copy.replaceChildren(svgIcon(['M5 12l4 4L19 6']));
          }catch(_){
            delete copy.dataset.copied;
            status.textContent=tr('Не удалось скопировать');
            copy.replaceChildren(svgIcon(copyPaths));
          }
          copy.title=status.textContent;
          reset=setTimeout(()=>{delete copy.dataset.copied;copy.title=label;copy.replaceChildren(svgIcon(copyPaths));status.textContent='';},1500);
        });
        actions.append(time,copy,status);group.append(actions);
      }else{
        meta.append(time);
      }
    };
    const reportFiles=taskFileSource(t);
    const outgoing=(author,role,text,at,model,provider)=>{
      group=null;const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message am-outgoing');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      if(provider)meta.append(messageAvatar(provider));
      if(author)meta.append(messageAuthor(author,model?modelName(model):'',role));
      if(role&&!taskSidebarPreview)meta.append(roleTag(role));
      if(author)body.append(meta);
      const content=el('div',undefined,'am-message-content');content.append(richText(text,undefined,reportFiles));body.append(content);row.append(body);g.append(row);
      appendMessageTime(g,meta,at,text);out.push(g);
    };
    const outgoingOwner=it=>outgoing(taskSidebarPreview?'':tr('Вы'),'',it.text,it.at);
    const outgoingCoordinator=it=>{
      const sender=shortNames[it.sender_agent];
      outgoing(sender||tr('Координатор'),sender?tr('координатор'):'',it.text,it.at,sender?it.sender_model:'',sender?it.sender_agent:'');
    };
    const workerGroup=at=>{
      if(group)return group;
      const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      const role=tr('исполнитель');
      meta.append(messageAvatar(t.agent),messageAuthor(names[t.agent]||t.agent,displayModel,role,displayModel?taskModelDetail(t):''));
      if(!taskSidebarPreview)meta.append(roleTag(role));
      const content=el('div',undefined,'am-message-content');body.append(meta,content);row.append(body);g.append(row);
      if(!taskSidebarPreview)appendMessageTime(g,meta,at);
      out.push(g);group=content;return content;
    };
    const workerMessages=new Map();
    function appendWorkerText(it,cls){
      const content=workerGroup(it.at);
      content.append(richText(it.text,cls,reportFiles));
      if(taskSidebarPreview&&it.text){
        const article=content.closest('article');
        const message=workerMessages.get(article)||{parts:[]};
        message.at=it.at;
        message.parts.push(it.text);
        workerMessages.set(article,message);
      }
      return content;
    }
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
      if(it.kind==='task'){if(it.author==='owner')outgoingOwner(it);else outgoingCoordinator(it);continue;}
      if(it.kind==='note'){if(it.author==='coordinator')outgoingCoordinator(it);else outgoingOwner(it);const c=out[out.length-1].querySelector('.am-message-content');c.append(delivery(...({pending:[tr('Исполнитель прочитает, только когда закончит текущий ход'),'am-status-wait'],delivered:[tr('Передано исполнителю'),'am-status-good'],sent:[it.author==='owner'?tr('Поставлено в очередь для исполнителя'):tr('Отправлено на компьютер исполнителя'),'am-status-wait'],undelivered:[tr('Не передано исполнителю'),'am-status-bad']}[it.state]||[it.state,'am-muted'])));continue;}
      if(it.kind==='answer'){if(it.author==='owner'){outgoingOwner(it);
        // A hint about this answer lives under it while the worker has not
        // gone on yet, so it never stays in another task (owner, 2026-11-09).
        if(feed.items.slice(index+1).every(n=>n.kind==='answer'||n.kind==='note')&&activeStates.includes(t.state))out[out.length-1].querySelector('.am-message-content').append(delivery(tr('Ответ записан. Исполнитель продолжит в той же сессии.'),'am-delivery-hint'));}else outgoingCoordinator(it);continue;}
      if(it.kind==='accepted'){const c=out[out.length-1]?.querySelector('.am-message-content');if(c)c.append(delivery(tr('Принято на компьютере {computer}').replace('{computer}',()=>it.text),'am-status-good'));group=null;continue;}
      if(it.kind==='text'){
        const progress=replyProgress.get(it.run);
        if(progress)progress.push(it.text);
        else appendWorkerText(it);
        continue;
      }
      if(it.kind==='tool'){
        const batch=collectTools(index);
        if(it.title==='applied_file_change'&&it.run>0&&it.text){
          const files=appliedFiles.get(it.run)||new Map();
          const file=files.get(it.text)||{added:0,removed:0,counted:true};
          const counts=/^\+(\d+) −(\d+)$/.exec(it.description||'');
          if(counts){file.added+=Number(counts[1]);file.removed+=Number(counts[2]);}
          else file.counted=false;
          files.set(it.text,file);
          appliedFiles.set(it.run,files);
        }
        const line=toolLine(it,toolAction(it));line.dataset.key='t'+index;
        batch.at??=it.at;
        batch.lines.push(line);
        batch.commandsOnly&&=isCommandTool((it.title||'').toLowerCase());
        continue;
      }
      if(it.kind==='report'||it.kind==='question'){
        const c=appendWorkerText(it,'am-disposition');
        if(it.images?.length)c.append(pictures(it.images));
        if(it.kind==='report'&&replyProgress.has(it.run)){
          const earlier=replyProgress.get(it.run).filter(text=>text.trim()&&text.trim()!==it.text.trim());
          if(earlier.length){
            const details=el('details',undefined,'am-tool');
            details.dataset.key='progress-'+it.run;
            details.append(el('summary',tr('Ход работы')),...earlier.map(text=>richText(text)));
            c.querySelector('.am-disposition')?.before(details);
          }
        }
        if(it.verified&&it.kind==='report'&&it.audience!=='held'&&appliedFiles.has(it.run)){
          c.append(fileChangeCard(appliedFiles.get(it.run),it.run,reportFiles));
          appliedFiles.delete(it.run);
        }
        let note;
        if(!it.verified)note=delivery(it.kind==='report'?tr('Отчёт сохранён без подтверждения запуска'):tr('Не опубликовано: запуск не прошёл проверку'),'am-status-bad');
        else if(it.kind==='report'&&it.audience==='held')note=delivery(tr('Промежуточный отчёт: исполнитель продолжает с вашим сообщением'),'am-status-wait');
        else if(it.kind==='report')note=delivery(direct?tr('Задача завершена'):tr('Отчёт отправлен координатору'),'am-delivery-close');
        else if(it.audience==='owner')note=delivery(direct?tr('Вопрос вам'):tr('Вопрос вам, координатор его не видит'),'am-status-wait');
        else note=delivery(tr('Вопрос координатору'),'am-status-wait');
        if(it.verified){
          if(taskSidebarPreview)note.title=relativeWhen(it.at,true);
          else appendEventTime(note,it.at,t.remote&&!t.remote.from);
        }
        c.append(note);
        group=null;continue;
      }
      if(it.kind==='outcome'){
        group=null;
        const overloaded=it.state==='failed'&&capacityFailure(it.text);
        const failedTitle=overloaded?tr('Завершение запуска не подтверждено'):t.remote&&!t.remote.from?tr('Задача не выполнена'):tr('Запуск завершился с ошибкой');
        const text={failed:failedTitle,blocked:t.remote&&!t.remote.from?tr('Задача заблокирована: результат на другом компьютере неизвестен'):tr('Задача заблокирована: процесс не завершился полностью'),interrupted:restartLabel(),continue_refused:tr('Не удалось продолжить сессию')}[it.state]||it.state;
        if(it.state==='cancelled'){out.push(stoppedNote(it,t));continue;}
        const note=el('div',undefined,'am-outcome am-status-bad'),line=el('p',text);
        appendEventTime(line,it.at,t.remote&&!t.remote.from);note.append(line);
        const why=t.remote&&!t.remote.from?refusalText(it.text,t.remote.name)||failureText(it.text):failureText(it.text);if(why)note.append(el('p',why));
        if(it.text&&it.text!==restartOutcome)note.append(el('p',it.text,'am-small am-muted'));
        const g=el('article',undefined,'am-message-group');g.append(note);out.push(g);
      }
    }
    flushTools();
    if(taskSidebarPreview){
      for(const [article,message] of workerMessages)appendMessageTime(article,null,message.at,message.parts.join('\n\n'));
      const date=conversationDate(t);
      if(date)out.unshift(date);
      if(quiet)out.push(quiet);
    }
    head.replaceChildren(header);
    thread.replaceChildren(...out);
    for(const d of thread.querySelectorAll('details[data-key]'))if(opened.has(d.dataset.key))d.open=true;
    // A task another computer started is answered there.
    const q=t.remote?.from?null:t.owner_question;
    // T-131: a task sent to another computer can be closed here also when
    // its outcome there is already unknown.
    // T-131: an unreachable task can be closed here. Sending waits for contact.
    const unreachable=taskUnreachable(t);
    // D-120: a stuck task of this computer is closed by hand once its worker
    // is certainly gone; the client checks that and refuses otherwise.
    const stuckHere=!unreachable&&(t.state==='blocked'||t.state==='interrupted');
    $('lost-bar').hidden=!unreachable&&!stuckHere;
    if(stuckHere)$('lost-text').textContent=(t.state==='interrupted'?tr('Клиент перезапустился во время работы, и исход задачи неизвестен.'):tr('Процесс исполнителя не завершился полностью, исход задачи неизвестен.'))+' '+tr('Закрыть её можно, когда исполнителя на этом компьютере точно нет.');
    if(unreachable){
      const since=t.remote.last_seen?stamp(t.remote.last_seen):'';
      if(t.remote.silent&&!t.remote.lost)$('lost-text').textContent='«'+t.remote.name+tr('» на связи, но не отвечает по этой задаче. Что с ней там, неизвестно: она могла и выполниться.');
      else if(!t.remote.lost&&['interrupted','blocked'].includes(t.state))$('lost-text').textContent=(t.state==='interrupted'?tr('Клиент перезапустился во время работы, и исход задачи неизвестен.'):tr('Процесс исполнителя не завершился полностью, исход задачи неизвестен.'));
      else $('lost-text').textContent=(since?tr('Связи с «')+t.remote.name+tr('» нет с ')+since+'. ':tr('Связи с «')+t.remote.name+tr('» нет. '))+tr('Что с задачей там, неизвестно.');
    }
    if(t.can_continue&&(unreachable||stuckHere))$('lost-text').textContent+=' '+tr('Новое сообщение продолжит сессию после проверки процесса.');
    const active=activeStates.includes(t.state),box=$('answer-text'),compose=$('compose');
    if(compose.dataset.task!==t.id&&$('answer-add').getAttribute('aria-expanded')==='true')$('task-draft-add-menu').hidePopover();
    const note=canNote(t),canDraft=canDraftTaskMessage(t);
    const liveTokens=$('answer-live-tokens'),reportedTokens=t.quiet?.live_tokens;
    liveTokens.hidden=taskSidebarPreview||reportedTokens==null||t.state!=='running';
    liveTokens.textContent=liveTokens.hidden?'':tokensText(reportedTokens)+tr(' т.');
    compose.hidden=!canDraft&&(!active||unreachable);
    if(taskSidebarPreview){
      const contextOnly=compose.hidden&&!!(t.context_window||t.usage);
      compose.toggleAttribute('data-context-only',contextOnly);
      if(contextOnly)compose.hidden=false;
      const model=$('answer-model');
      model.textContent=displayModel;
      model.title=displayModel?taskModelDetail(t):'';
      showComposerAccess($('answer-access'),!!t.full_access,t.remote&&!t.remote.from?(t.remote.name||tr('компьютере исполнителя')):'',true);
      renderComposerTokens(t,displayModel,!compose.hidden);
      $('answer-context').hidden=!displayModel&&$('answer-tokens').hidden&&$('answer-access').hidden;
    }
    box.disabled=!canDraft;
    box.placeholder=taskMessagePlaceholder(t);
    $('answer-send').hidden=!canDraft;
    $('answer-add').hidden=!taskSidebarPreview||!canDraft;
    if(compose.dataset.task!==t.id){compose.dataset.task=t.id;box.value='';$('answer-status').textContent='';}
    box.title=canDraft&&(unreachable&&!t.can_continue||!q&&!note)?tr('Отправка пока недоступна. Черновик сохранится, пока открыта задача.'):'';
    $('answer-send').title=box.title||tr('Отправить ответ');
    renderStopControls(t);
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
    if(/different conversation/.test(text))return tr('Агент ответил не в той сессии, которую запустил Kotel; результату нельзя доверять.');
    if(code)return tr('Программа агента завершилась с ошибкой, код ')+code[1]+'.';
    if(/did not report a successful end/.test(text))return tr('Агент не сообщил об успешном окончании работы: возможно, упал или был остановлен.');
    if(/ended without sending its report/.test(text))return tr('Исполнитель закончил, но не отправил отчёт через Kotel. Координатор может попросить его прислать отчёт в той же сессии.');
    return '';
  }
  // Known refusals of another computer, explained in the window's language.
  function refusalText(text,name){
    if(/does not run tasks of other computers/.test(text||''))return tr('Компьютер «')+name+tr('» не принимает задачи других компьютеров. Включите на нём «Принимать задачи других компьютеров» на вкладке «Сеть» и дайте задачу снова.');
    if(/has no project/.test(text||''))return tr('На компьютере «')+name+tr('» нет проекта с таким названием. Создайте его там и выберите папку.');
    if(/has no folder for the project|project folder is missing/.test(text||''))return tr('На компьютере «')+name+tr('» у проекта не выбрана папка или её нет. Выберите папку проекта там и дайте задачу снова.');
    if(/is not set up on this computer/.test(text||''))return tr('На компьютере «')+name+tr('» этот агент не настроен. Настройте его там на вкладке агентов.');
    return '';
  }
  // The reply box is one line and grows only while its text does not fit;
  // the stop and send buttons stay at its bottom.
  function sizeAnswer(){sizeComposer($('answer-text'));}
  function sizeComposer(input){
    input.style.height='';input.style.overflowY='';
    const base=input.offsetHeight;
    const full=input.scrollHeight+(base-input.clientHeight);
    const expanded=!!input.value&&base>0&&full>base;
    if(expanded){
      input.style.height=Math.min(140,full)+'px';
      if(full>140)input.style.overflowY='auto';
    }
  }
  // Owner notes follow a task given here, including one running on a peer.
  function canNote(t){
    if(!t||t.owner_question)return false;
    if(t.can_continue)return true;
    if(!t.remote)return t.state==='running'||t.state==='waiting_coordinator';
    return t.state==='running'&&!t.remote.from&&!t.remote.lost&&!t.remote.silent;
  }
  function stoppedNote(item,task){
    const note=el('p',tr('Работа остановлена'),'am-delivery am-delivery-close am-system-event');
    if(taskSidebarPreview)note.title=connectionTimestamp(item.at);
    else appendEventTime(note,item.at,task.remote&&!task.remote.from);
    return note;
  }
  function canDraftTaskMessage(t){
    return !!t&&!t.archived&&!t.remote?.from&&!!(t.owner_question||activeStates.includes(t.state)||canNote(t)||
      (t.remote&&(t.remote.lost||t.remote.silent)));
  }
  function taskMessagePlaceholder(t){
    if(t?.owner_question)return tr('Ответить исполнителю…');
    if(t?.can_continue)return tr('Продолжить в этой сессии…');
    if(t?.state==='waiting_coordinator'&&canNote(t))return tr('Ответить вместо координатора…');
    if(t?.remote?.from)return tr('Писать исполнителю можно на компьютере «')+t.remote.name+'»';
    return tr('Написать исполнителю…');
  }
  function canSendTaskMessage(t){return !!t&&!t.archived&&!t.remote?.from&&!stopProgress(t)&&(!taskUnreachable(t)||t.can_continue)&&!!(t.owner_question||canNote(t));}
  function answerControls(){$('answer-send').disabled=answering||!canSendTaskMessage(selectedTaskRow())||!$('answer-text').value.trim();composerDictation?.sync();}
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
    if(composerDictation?.finishFor($('answer-text')))return;
    const t=selectedTaskRow(),q=t?.owner_question;
    if($('compose').inert||!canSendTaskMessage(t))return;
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
  // with Kotel, limit) with a mark on the left and one action on the
  // right; one «Задачи» button under them. Steps that cannot be checked yet
  // are grey and numbered.
  const checkPath=['M21 12a9 9 0 1 1-3-6.7L21 8','M21 3v5h-5'];
  const gearPath=['M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z'];
  // The update mark: a down arrow.
  function appendUpdateGlyphs(node){node.append(svgIcon(['M12 3v12m-5-5 5 5 5-5M3 15v4a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-4']));}
  function updateSpinner(){
    // Keep the rotation axis at the center of the 24-unit viewBox.
    const icon=svgIcon(['M12 4A8 8 0 0 0 4 12M12 20a8 8 0 0 0 8-8','M1 9l3 3 3-3M17 15l3-3 3 3']);
    icon.classList.add('am-update-spin');
    return icon;
  }
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
    askConfirm(tr('Удалить ')+names[provider]+' CLI?',tr('Откроется терминал: он покажет, что будет удалено, и спросит подтверждение. Вход в агент и настройки Kotel сохранятся.'),tr('Открыть терминал'),async()=>{
      try{await postAPI('/api/agent-uninstall',{provider});noteRow(provider+':auth',tr('Терминал удаления открыт'),15000);}
      catch(error){showError(error);}
    });
  }
  let agentComputer='local';
  function renderAgents(){
    const live=$('agents'),cards=live.cloneNode(false);
    if(taskSidebarPreview){
      const options=computerOptions();
      if(view?.computer_name)options[0][1]+=' ('+tr('Этот компьютер').toLocaleLowerCase(amLocale)+')';
      if(!options.some(([id])=>id===agentComputer))agentComputer='local';
      projectOptions($('agents-computer'),options,agentComputer);
      $('agents-computer-filter').hidden=false;
      syncSelects();
    }
    const computer=taskSidebarPreview?agentComputer:computerFilter;
    if(computer&&computer!=='local'){renderPeerAgents(cards,computer);swapChildren(live,cards);return;}
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
      sw.setAttribute('aria-label',names[provider]+tr(': Kotel даёт задачи'));
      sw.title=on?tr('Kotel даёт задачи ')+names[provider]+tr(' на этом компьютере'):cli?tr('Включить: Kotel будет давать задачи ')+names[provider]:tr('Сначала установите ')+names[provider]+' CLI';
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
          mark.title=tr('Kotel проверен с версией ')+check.tested_version+tr('. Новая версия обычно работает, но если задачи этого агента начнут падать, причина может быть в ней.');
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
      // 3. Work with Kotel: set only by what a task showed (D-070).
      const trust=a?.trust?.state;
      if(a&&view?.folder&&trust&&trust!=='trusted')step('no',tr('Не доверяет папке проекта'),tr('Подтвердите доверие в терминале'),[tr('Доверять'),()=>trustFolder(provider)],a.trust?.hint);
      else if(a?.last_done)step('ok',tr('Работает с Kotel'),tr('Последняя задача ')+dayClock(a.last_done));
      else step(3,tr('Работает с Kotel'),missing?tr('Проверится после установки'):tr('Проверится на первой задаче'));
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
  function renderPeerAgents(cards,computer){
    $('access-bar').hidden=true;
    const peer=(server?.peers||[]).find(p=>p.id===computer);
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
      const models=el('span',undefined,'am-agent-models');models.textContent=a.models?.length?orderedModels(a.models).join(', '):tr('Модель по умолчанию');
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
    const agents=view?.agents||[],bar=$('access-bar');bar.hidden=!agents.length||(taskSidebarPreview&&agentComputer!=='local');
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
      const info=await api('/api/client-info');clientInfo=info;
      if(info.client==='running'&&!info.outdated){restart.ask=null;restart.wait=false;}
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
    if((response.ok&&result.ok)||result.state==='current'){
      restart={busy:false,ask:null,error:'',wait:false};clientInfo={...clientInfo,outdated:false};renderRestart();
      showError(null);await loadClientInfo();viewSignature='';refresh();return;
    }
    if(result.state==='confirm'){restart={busy:false,ask:{running:result.running||0,known:result.known!==false,tasks:result.tasks||[]},error:'',wait};renderRestart();return;}
    restart={busy:false,ask:null,error:result.detail||tr('Перезапуск не подтверждён. Проверьте состояние.'),wait:false};renderRestart();loadClientInfo();
  }
  function renderRestart(){
    renderBoot();
    const dialog=$('restart-dialog');
    $('restart-wait-status').hidden=!restart.wait;
    if(!restart.wait)restartWaitDetails=false;
    // While the update installs, the start screen shows it instead (T-079).
    const show=restart.wait?restartWaitDetails:(!!clientInfo?.outdated&&!restart.busy)||!!restart.ask||!!restart.error;
    if(!show){if(dialog.open)dialog.close();return;}
    const ask=restart.ask,canRestart=!!clientInfo?.restart,answering=clientInfo?.client==='running';
    const canQuit=!!window.webkit?.messageHandlers?.app;
    let title=tr('Нужно перезапустить Kotel'),text;
    const version=clientInfo?.window_version?tr('Версия ')+versionName(clientInfo.window_version).slice(1)+tr(' установлена. '):'';
    if(ask&&restart.wait){title=tr('Ждём окончания задач');text=(ask.known?tr('Задач в работе: ')+ask.running+'. ':'')+tr('Kotel перезапустится сам, когда они закончатся. Если выйти из приложения, следующая проверка будет при запуске.');}
    else if(ask&&ask.known){title=tr('Обновление ждёт задачи: ')+ask.running;text=version+tr('Чтобы она заработала, Kotel перезапустится и прервёт задачи, которые идут сейчас. Сами они не повторятся.');}
    else if(ask){title=tr('Не удалось проверить задачи');text=version+tr('Если задачи сейчас идут, перезапуск прервёт их. Сами они не повторятся.');}
    else if(restart.busy){title=tr('Обновляем Kotel');text=tr('Останавливаем старую версию и запускаем новую. Это займёт несколько секунд.');}
    else if(canRestart)text=tr('Приложение обновилось до версии ')+clientInfo.window_version+tr(', а в фоне работает ')+(clientInfo.client_version?tr('версия ')+clientInfo.client_version:tr('более ранняя версия'))+tr('. Пока она не перезапущена, Kotel не может работать. Задачи, история и устройства сохранятся.');
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
    cut.hidden=!ask||!canRestart||!answering;cut.disabled=restart.busy;
    cut.textContent=restart.wait?tr('Прервать сейчас'):tr('Прервать и перезапустить');
    quit.hidden=!restart.wait&&!canQuit;quit.disabled=restart.busy&&!restart.wait;
    quit.setAttribute('aria-label',restart.wait?tr('Закрыть'):tr('Закрыть окно'));
    quit.title=restart.wait?tr('Закрыть'):tr('Закрыть окно, оставив задачи в работе');
    copy.hidden=canRestart||!shutdownCommand();
    if(!dialog.open){dialog.showModal();(go.hidden?quit.hidden?copy:quit:go).focus();}
  }
  function renderNotice(){
    if(!online){notice(el('p',offlineReason||tr('Клиент Kotel не запущен или не отвечает. Данные не обновляются.')));return;}
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
  function syncStopSubmissions(tasks){
    for(const t of tasks||[]){
      const stopping=stopSubmissions.get(t.id);
      if(stopping&&stopping!=='sending'&&(t.remote?.cancel_state||!activeStates.includes(t.state)))stopSubmissions.delete(t.id);
    }
  }
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
      catch(error){if(requestScope!==taskWindowQuery())return;if(Date.now()-clientInfoAt>5000)loadClientInfo();online=false;offlineReason=error.name==='TimeoutError'||error.name==='TypeError'?tr('Окно потеряло связь с программой Kotel. Закройте окно и откройте снова.'):error.message;renderProjects();renderMachines();renderNotice();return false;}
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
      syncStopSubmissions(v.tasks);
      const previousScope=windowHistoryScope;
      windowHistoryScope=taskWindowQuery();
      const signature=JSON.stringify(v);
      if(signature!==viewSignature){
        syncTaskHistory(v);
        if(splitProjects.length&&view?.project===passiveProject()&&auxActivity?.project!==view.project){
          auxActivity=view;
          auxTaskScope=previousScope===taskWindowQuery(view.project)?previousScope:'';
        }
        viewSignature=signature;view=v;noteWelcomeDone();
        if(!v.history_paged&&selectedTask&&!v.tasks.some(t=>t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}
        if(selectedShared&&!sharedSelection())selectedShared=null;
        // Choose the top task before drawing, so a project switch goes from
        // loading straight to the conversation without an empty pane between.
        if(!selectedTask&&!selectedShared&&tab==='tasks'&&v.project)openTopTask(true);
        render();if(taskSidebarPreview)loadSidebarProjects();
      }
      else if(cameOnline){renderMachines();renderNotice();}
      applyTopTaskScope();
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
    if(next!=='tasks')composerDictation?.cancel();
    tab=next;root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab&&!(taskSidebarPreview&&n.dataset.screen==='tasks'));renderToasts();
    syncTaskPanes();
    root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    if(taskSidebarPreview){
      const dialog=$('settings-dialog');
      const statistics=$('stats-dialog');
      if(tab!=='stats'&&statistics.open)statistics.close();
      if(tab==='tasks'||tab==='stats'){
        if(dialog.open)dialog.close();
        if(tab==='stats'&&!statistics.open)statistics.showModal();
      }else if(!dialog.open)dialog.showModal();
    }
    $('list-toggle').hidden=!view?.project||(!taskSidebarPreview&&tab!=='tasks');if(tab==='server'||tab==='settings')loadServer();if(tab==='project')renderProjectPage();if(tab==='stats')reloadStats();if(tab==='agents')renderAgents();$('screen-title').textContent={tasks:tr('Задачи'),settings:tr('Настройки программы'),agents:tr('Агенты'),project:tr('Проект'),server:tr('Сеть'),stats:tr('Статистика')}[tab];startWindowWatch();
    if(tab==='tasks'&&combinedTasks()){renderTaskList();loadProjectActivity();}
    if(tab==='tasks'&&setupPaneShown())renderThread();
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
  let statsProject='';
  let statsComputer='';
  const statsProjects=()=>view?.projects||[];
  function statisticsComputer(){return taskSidebarPreview?statsComputer:computerFilter;}
  function statsProjectScope(){
    if(taskSidebarPreview){
      const names=statsProjects().map(entry=>entry.name);
      if(names.includes(statsProject))return {project:statsProject};
      return names.length>1?{projects:names}:{project:names[0]||''};
    }
    if(splitProjects.length&&!statsProject)return {projects:splitProjects};
    return {project:statsProject||project};
  }
  function updateStatsProjectScope(before){
    if(JSON.stringify(before)===JSON.stringify(statsProjectScope()))return false;
    if(!project)stats=null;
    statsModel='';statsGen++;
    if(tab==='stats')reloadStats();
    return true;
  }
  function setProject(name,computer){
    if(project===name)return;
    const before=statsProjectScope();
    const oldComputer=computerFilter;
    if(project&&oldComputer)projectComputers.set(project,oldComputer);
    else projectComputers.delete(project);
    computerFilter=computer??projectComputers.get(name)??'';
    $('agent-filter').value='';
    project=name;
    if(tab==='stats'&&computer!==undefined&&splitProjects.includes(name))statsProject=name;
    if(!updateStatsProjectScope(before)&&tab==='stats'&&computerFilter!==oldComputer)reloadStats();
  }
  function statsFilters(){
    return {...statsProjectScope(),period:statsPeriod,computer:statisticsComputer(),agent:statsAgent,model:statsModel};
  }
  function statsQuery(filters){
    const {projects=[],...rest}=filters,query=new URLSearchParams(rest);
    for(const name of projects)query.append('projects',name);
    return query;
  }
  function matchesStatsProject(result,filters){
    return filters.projects?JSON.stringify(result.projects)===JSON.stringify(filters.projects):result.project===filters.project;
  }
  let statsExporting=false;
  let efficiencyScope='',efficiencyGen=0;
  function refreshEfficiencyScope(){
    const card=$('stats-efficiency-card'),button=$('stats-efficiency');
    if(!project)card.hidden=true;
    $('stats-usage').hidden=!card.hidden;
    button.disabled=!project;
    button.setAttribute('aria-expanded',String(!card.hidden));
    if(card.hidden){
      if(efficiencyScope){efficiencyScope='';efficiencyGen++;}
      card.removeAttribute('aria-busy');
      return;
    }
    const filters=statsFilters(),scope=JSON.stringify(filters);
    if(efficiencyScope!==scope)loadEfficiency(filters,scope);
  }
  function compareModels(){
    if(!project)return;
    const card=$('stats-efficiency-card');
    card.hidden=!card.hidden;
    refreshEfficiencyScope();
  }
  async function loadEfficiency(filters,scope){
    const card=$('stats-efficiency-card'),status=$('stats-efficiency-status'),wrap=$('stats-efficiency-table');
    const gen=++efficiencyGen;
    efficiencyScope=scope;
    card.setAttribute('aria-busy','true');
    wrap.hidden=true;
    status.textContent=tr('Загрузка статистики…');
    try{
      const result=await api('/api/usage-efficiency?'+statsQuery(filters),25000);
      if(gen!==efficiencyGen)return;
      if(!matchesStatsProject(result,filters)||!Array.isArray(result.groups))throw Error(tr('Эффективность моделей недоступна.'));
      renderEfficiencyGroups(result.groups);
      wrap.hidden=!result.groups.length;
      status.textContent=!result.groups.length?tr('В этом периоде нет исходных задач для сравнения.'):
        result.total_groups>result.groups.length?tr('Показана часть групп. Полный список есть в экспорте JSON.'):
        tr('Токены и время — медианы по принятым задачам. Расход включает повторные попытки.');
    }catch(e){
      if(gen===efficiencyGen){wrap.hidden=true;status.textContent=e.message;}
    }finally{
      if(gen===efficiencyGen)card.removeAttribute('aria-busy');
    }
  }
  function renderEfficiencyGroups(groups){
    const table=$('stats-efficiency-grid');
    for(const body of Array.from(table.tBodies))body.remove();
    const types={bug_fix:tr('Исправление'),feature:tr('Функция'),refactor:tr('Рефакторинг'),tests:tr('Тесты'),documentation:tr('Документация'),build:tr('Сборка'),audit:tr('Аудит'),research:tr('Исследование'),maintenance:tr('Поддержка')};
    const difficulties={simple:tr('Простая задача'),medium:tr('Средняя сложность'),complex:tr('Сложная задача')};
    const cohorts=new Map();
    const ordered=[...groups].sort((a,b)=>Number(!!b.task_type)-Number(!!a.task_type));
    ordered.forEach((group,i)=>{
      const key=JSON.stringify([group.project,group.task_type,group.difficulty]);
      if(!cohorts.has(key)){
        const body=el('tbody'),heading=el('tr',undefined,'am-efficiency-cohort');
        const type=Object.hasOwn(types,group.task_type)?types[group.task_type]:group.task_type||tr('Без типа');
        const difficulty=difficulties[group.difficulty]||tr('Без оценки сложности');
        const prefix=group.project?projectTitle(group.project)+', ':'';
        const label=el('th',prefix+type+', '+difficulty.toLocaleLowerCase(amLocale));label.scope='rowgroup';label.colSpan=5;
        heading.append(label);body.append(heading);cohorts.set(key,body);
      }
      cohorts.get(key).append(...efficiencyRows(group,i));
    });
    table.append(...cohorts.values());
  }
  function efficiencyRows(group,index){
    const row=el('tr',undefined,'am-efficiency-row'),details=el('tr',undefined,'am-efficiency-detail');
    details.id='stats-efficiency-detail-'+index;details.hidden=true;
    const model=(names[group.agent]||group.agent)+' '+(group.model?modelName(group.model):tr('Модель неизвестна'));
    const count=', '+tasksWord(group.tasks);
    const button=el('button',undefined,'am-efficiency-expand');button.type='button';button.title=model+count;
    button.setAttribute('aria-label',button.title);button.setAttribute('aria-expanded','false');button.setAttribute('aria-controls',details.id);
    const label=el('span',undefined,'am-efficiency-model');
    label.append(el('span',model,'am-efficiency-model-name'),el('span',count,'am-muted'));
    button.append(label);
    row.addEventListener('click',()=>{details.hidden=!details.hidden;button.setAttribute('aria-expanded',String(!details.hidden));});
    const name=el('th');name.scope='row';name.append(button);row.append(name);
    const rate=(value,total)=>{
      const cell=el('td');
      if(!total){cell.textContent='—';return cell;}
      cell.textContent=Math.round(100*value/total)+'%';
      return cell;
    };
    const metric=(data,format)=>data?.samples?format(Math.round(data.median||0)):'—';
    row.append(rate(group.accepted,group.reviewed),rate(group.first_pass_accepted,group.first_pass_known),
      el('td',metric(group.accepted_new_input_plus_output,tokensText)),el('td',metric(group.time_to_acceptance_ms,statsDuration)));

    const panel=el('td');panel.colSpan=5;
    const breakdown=el('div',undefined,'am-efficiency-breakdown');
    const countLabel=(label,value,total)=>label+': '+(total?value+'/'+total:'—');
    const acceptanceCounts=[
      countLabel(tr('Принято'),group.accepted,group.reviewed),
      countLabel(tr('С первого раза'),group.first_pass_accepted,group.first_pass_known)
    ];
    const metrics=el('table',undefined,'am-efficiency-metrics');
    metrics.setAttribute('aria-label',model);
    const headers=el('tr');metrics.createTHead().append(headers);
    for(const title of [tr('Показатель'),tr('Среднее'),tr('Медиана'),tr('Порог для 90%'),tr('Задач')]){
      const heading=el('th',title);heading.scope='col';headers.append(heading);
    }
    const measurements=metrics.createTBody();
    const addMetric=(title,data,format)=>{
      const line=el('tr'),name=el('th',title);name.scope='row';line.append(name);
      for(const value of [data?.mean,data?.median,data?.p90])line.append(el('td',data?.samples?format(Math.round(value||0)):'—'));
      line.append(el('td',(data?.samples??0).toLocaleString(amLocale),'am-muted'));measurements.append(line);
    };
    addMetric(tr('Токены до приёмки'),group.accepted_new_input_plus_output,tokensText);
    addMetric(tr('Активная работа'),group.accepted_active_runtime_ms,statsDuration);
    addMetric(tr('Время до приёмки'),group.time_to_acceptance_ms,statsDuration);
    const note=el('p',acceptanceCounts.join(', '),'am-small am-muted');
    note.append(el('br'),tr('Проверено ')+group.reviewed+'/'+group.tasks+'. '+
      tr('В 90% задач расход или время были не выше указанного порога.'));
    breakdown.append(metrics,note);
    panel.append(breakdown);details.append(panel);
    return [row,details];
  }
  async function exportStats(){
    if(!project||statsExporting)return;
    statsExporting=true;$('stats-export').disabled=true;
    const status=$('stats-export-status');
    status.textContent=tr('Сохраняем…');
    try{
      const {result}=await postAPI('/api/usage-export',statsFilters(),25000);
      status.textContent=tasksWord(result.tasks)+'. '+tr('Файл сохранён: ')+result.path;
    }catch(error){status.textContent=error.message;}
    finally{statsExporting=false;$('stats-export').disabled=!project;}
  }
  async function loadStats(){
    const gen=++statsGen;
    const filters=statsFilters();
    if(!filters.project&&!filters.projects)return;
    try{
      const s=await api('/api/usage-stats?'+statsQuery(filters),8000);
      if(gen!==statsGen)return;
      if(!matchesStatsProject(s,filters)){stats=null;statsPending=false;renderStats();throw Error(tr('Фильтр не применён: обновите фоновый клиент.'));}
      s.forComputer=filters.computer;stats=s;statsPending=false;renderStats();
    }
    catch(error){
      if(gen!==statsGen)return;
      if(stats&&!matchesStatsProject(stats,filters))stats=null;
      statsPending=false;renderStats();
      $('stats-total-title').textContent=tr('Статистика недоступна');$('stats-total-note').textContent=error.message;
    }
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
    $('stats-export').disabled=!project||statsExporting;
    refreshEfficiencyScope();
    root.querySelectorAll('[data-period]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.period===statsPeriod)));
    // The card follows the numbers on screen, not the filter still loading.
    $('stats-computers-card').hidden=stats?!!stats.forComputer:!!statisticsComputer();
    setStatsBusy();
    const projects=taskSidebarPreview?statsProjects().map(entry=>entry.name):splitProjects;
    const projectChoice=projects.includes(statsProject)?statsProject:'';
    $('stats-project-filter').hidden=taskSidebarPreview?false:!splitProjects.length;
    const allProjectsLabel=taskSidebarPreview?tr('Все проекты'):tr('Выбранные проекты');
    projectOptions($('stats-project'),[['',allProjectsLabel],...projects.map(name=>[name,projectTitle(name)])],projectChoice);
    $('stats-computer-filter').hidden=!taskSidebarPreview;
    if(taskSidebarPreview){
      const computers=[['',tr('Все компьютеры')],...computerOptions()];
      if(statsComputer&&!computers.some(([id])=>id===statsComputer))computers.push([statsComputer,computerName(statsComputer)||statsComputer]);
      projectOptions($('stats-computer'),computers,statsComputer);
    }
    const statsNote=projects.length>1&&!projectChoice?
      tr('Учитываются задачи выбранных проектов, известные этому компьютеру. Задача попадает в день, когда закончилась.'):
      tr('Учитываются задачи выбранного проекта, известные этому компьютеру. Задача попадает в день, когда закончилась.');
    const changingProjects=!!stats&&!matchesStatsProject(stats,statsProjectScope());
    $('stats-foot').textContent=project?(changingProjects?tr('Загрузка статистики…'):statsNote):'';
    $('stats-top').inert=changingProjects;
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
      const detail=[task.project&&projectTitle(task.project),shortNames[task.agent]||task.agent,task.model&&modelName(task.model),task.computer&&tr('на ')+task.computer].filter(Boolean).join(' ');
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
      if(!task.usage_missing&&typeof task.input_tokens==='number')tokens.title=usageLong(task);
      row.addEventListener('click',()=>{
        if(task.project&&task.project!==project&&(taskSidebarPreview
          ?statsProjects().some(entry=>entry.name===task.project):splitProjects.includes(task.project))){
          show('tasks');selectProject(task.project,task.id);return;
        }
        selectedTask=task.id;selectedShared=null;feed=null;feedSignature='';show('tasks');renderThread();refresh();
      });
      top.append(row);
    }
  }
  $('stats-project').addEventListener('change',()=>{
    const before=statsProjectScope();
    statsProject=$('stats-project').value;updateStatsProjectScope(before);
  });
  $('stats-computer').addEventListener('change',()=>{statsComputer=$('stats-computer').value;statsModel='';reloadStats();});
  $('stats-agent').addEventListener('change',()=>{statsAgent=$('stats-agent').value;remember('messenger-stats-agent',statsAgent);statsModel='';reloadStats();});
  $('stats-model').addEventListener('change',()=>{statsModel=$('stats-model').value;reloadStats();});
  $('stats-export').addEventListener('click',exportStats);
  $('stats-efficiency').addEventListener('click',compareModels);
  root.querySelectorAll('[data-stats-sort]').forEach(b=>b.addEventListener('click',()=>{
    statsSort=statsSort===b.dataset.statsSort?b.dataset.statsReverse:b.dataset.statsSort;
    remember('messenger-stats-sort',statsSort);renderStats();
  }));
  root.querySelectorAll('[data-period]').forEach(b=>b.addEventListener('click',()=>{statsPeriod=b.dataset.period;remember('messenger-stats-period',statsPeriod);reloadStats();}));

  // Server tab: the broker hosted by this client and this computer's
  // connection to a broker. Messages stay next to the section they belong to.
  let server=null,serverBusy=false,serverGen=0,serverPolling=false,serverEdited=false,nameFilled=false,invitation=null,inviteTimer=0,revokeAsk='',revokeTimer=0;
  const notes={host:null,member:null,accept:null},noteTimers={};
  function connectionTimestamp(at){
    const date=new Date(at);
    if(!at||Number.isNaN(date.getTime()))return '';
    const day=amLocale==='ru-RU'
      ? date.toLocaleDateString(amLocale,{day:'numeric',month:'long'})+' '+date.getFullYear()+tr(' года')
      : date.toLocaleDateString(amLocale,{day:'numeric',month:'long',year:'numeric'});
    return day+', '+date.toLocaleTimeString(amLocale,{hour:'2-digit',minute:'2-digit'});
  }
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
    const cancelAttempts=[...updateCancelAttempts];
    try{
      const next=await api('/api/server');
      if(gen===serverGen&&!serverBusy){
        const sharedRuns=()=>JSON.stringify([view,taskListView(passiveProject())].flatMap(snapshot=>(snapshot?.shared||[]).flatMap(p=>p.tasks.filter(t=>(!t.started_at||t.pending_launch||!peerFresh(p))&&sharedWorkerRunning(t)).map(t=>t.id))));
        const previous=sharedRuns(),previousPeers=welcomePeerCount(),wasHosting=!!server?.host?.running;
        server=next;
        if(draftOpen)renderTaskDraft();
        else if(tab==='tasks'&&setupPaneShown()&&(previousPeers!==welcomePeerCount()||wasHosting!==!!next.host?.running||
          taskSidebarPreview&&view?.project&&canDraftTask(sidebarProject||project)))renderThread();
        // This snapshot started after the submission returned. Trust the
        // durable queue from here, regardless of the submission's reply.
        for(const [key,state] of cancelAttempts){
          if(state!=='sending'&&updateCancelAttempts.get(key)===state&&(next.update_cancellations||state==='unknown'))updateCancelAttempts.delete(key);
        }
        renderServer();
        if(previous!==sharedRuns()){
          renderTaskList();
          if(selectedShared)renderThread();
        }
      }
    }
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
  const networkModes=['member','host'];
  let preferredNetworkMode=null;
  function showNetworkMode(mode){
    const group=$('member-tab').parentElement;
    group.style.setProperty('--am-selected-segment',String(networkModes.indexOf(mode)));
    for(const name of networkModes){
      const selected=name===mode,button=$(name+'-tab');
      button.setAttribute('aria-selected',String(selected));
      button.tabIndex=selected?0:-1;
      $(name+'-panel').hidden=!selected;
    }
  }
  for(const mode of networkModes){
    const button=$(mode+'-tab');
    button.addEventListener('click',()=>{
      button.parentElement.dataset.animated='';
      preferredNetworkMode=mode;showNetworkMode(mode);
    });
    button.addEventListener('keydown',event=>{
      if(!['Home','End','ArrowLeft','ArrowRight'].includes(event.key))return;
      event.preventDefault();
      const next=event.key==='Home'?'member':event.key==='End'?'host':mode==='host'?'member':'host';
      $(next+'-tab').click();$(next+'-tab').focus();
    });
  }
  function renderServer(){
    renderAccessBar();renderAutoUpdate();watchUpdateBoard();
    if(!server)return;const h=server.host,m=server.member;
    const focused=document.activeElement?.dataset?.key||'';
    const hostBadge=$('host-badge');
    hostBadge.hidden=!h.running&&!h.enabled;
    if(!hostBadge.hidden)badge(hostBadge,h.running?tr('Работает'):tr('Не запустился'),h.running?'good':'bad');
    // Explanations live in the status tooltip so the card does not change
    // height; only a problem the owner must fix stays visible.
    $('host-badge').title=!h.running?tr('Подключение по одноразовому приглашению'):tr('Принимает устройства: ')+(h.scope==='internet'?tr('из интернета и локальной сети'):h.scope==='lan'?tr('в локальной сети'):tr('только с этого компьютера'))+tr(', порт ')+h.port;
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
    if(taskSidebarPreview&&tab==='agents')renderAgents();
    // The certificate fingerprint travels inside the invitation code and the
    // joining computer checks it itself, so the window does not show it.
    $('member-since-label').classList.toggle('am-good',!!m.reachable);
    $('member-unreachable').hidden=m.reachable||!m.checked_at;
    // A computer hosts its network or joins another one, not both: the
    // inactive role's panel explains how to switch.
    const hostOn=h.enabled||h.running;
    showNetworkMode(preferredNetworkMode||(hostOn?'host':'member'));
    $('member-join').hidden=m.connected||hostOn;$('member-joined').hidden=!m.connected;
    $('member-blocked').hidden=!hostOn||m.connected;
    $('host-blocked').hidden=hostOn||!m.connected;
    $('host-panel').querySelector('.am-server-grid').hidden=!hostOn&&m.connected;
    $('member-state').textContent='';
    const cn=$('computer-name');if(view?.computer_name&&document.activeElement!==cn&&!cn.dataset.edited)cn.value=view.computer_name;
    $('member-address').textContent=m.address||'';
    $('member-since').textContent=connectionTimestamp(m.connected_at)||'—';
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
      else ($(section+'-panel')||$(section+'-title'))?.focus();
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
    if(await serverAction({action:'connect',code,name},'member',''))$('member-code').value='';
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
  function openProjectDialog(fromDraft=false){
    $('project-dialog').toggleAttribute('data-from-draft',fromDraft===true);
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
      const fromDraft=$('project-dialog').hasAttribute('data-from-draft');
      $('project-dialog').close();viewSignature='';await refresh();
      if(fromDraft&&draftOpen){renderTaskDraft(name);$('task-draft-text').focus();}
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
    const name=view?.project===project?project:'';
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
    const name=view?.project;if(!name||name!==project)return;
    const status=$(field==='rules'?'project-rules-status':'project-page-status');
    projectSaving=projectSaving.then(async()=>{
      if(view?.project!==name||project!==name)return;
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
  function askRemoveProject(name=view?.project){
    if(!name)return;
    askConfirm(tr('Удалить проект «')+(name===view?.project&&view.project_title||projectTitle(name))+'»?',tr('Проект пропадёт из списка. История задач сохранится и вернётся, если создать проект с тем же именем.'),tr('Удалить'),async()=>{
      await postAPI('/api/project-remove',{name});
      if(project===name){
        setProject('');remember('messenger-project','');selectedTask='';selectedShared=null;feed=null;feedSignature='';projectPageFor='';
      }
      if(sidebarProject===name)sidebarProject='';
      viewSignature='';await refresh();
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
  const preferredTaskModels={codex:'gpt-6.2-sol[effort=medium]',claude:'claude-sonnet-5-5[effort=medium]',cursor:'cursor-grok-4.6-medium'};
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
    $('agent-models-open').disabled=!view;
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
  const modelPicker=$('agent-model-picker');
  const modelSearch=$('agent-model-search');
  let modelEdit=null;
  function renderModelOptions(){
    const list=$('agent-model-options'),query=modelSearch.value.trim().toLowerCase();
    const previouslyFocused=document.activeElement?.dataset.model;
    const options=new Map(modelEdit.options.map(option=>[option.model,option.name]));
    for(const id of modelEdit.enabled)if(!options.has(id))options.set(id,modelName(id));
    const buttons=[];
    list.replaceChildren();
    for(const id of orderedModels(options.keys())){
      const name=options.get(id);
      if(query&&!name.toLowerCase().includes(query)&&!id.toLowerCase().includes(query))continue;
      const row=el('div',undefined,'am-model-option');
      const title=el('span',name);title.title=id;
      const toggle=el('button',undefined,'am-switch');toggle.type='button';toggle.dataset.model=id;
      toggle.setAttribute('role','switch');toggle.setAttribute('aria-label',name);
      toggle.setAttribute('aria-checked',String(modelEdit.enabled.has(id)));
      toggle.disabled=modelEdit.busy;
      toggle.addEventListener('click',()=>{
        if(modelEdit.enabled.delete(id)){toggle.setAttribute('aria-checked','false');modelStatus();return;}
        if(enableModel(id))toggle.setAttribute('aria-checked','true');
      });
      row.append(title,toggle);list.append(row);buttons.push(toggle);
    }
    const add=$('agent-model-add');
    add.hidden=!query||options.has(modelSearch.value.trim());
    add.disabled=modelEdit.busy;
    buttons.find(button=>button.dataset.model===previouslyFocused)?.focus({preventScroll:true});
  }
  function modelStatus(message){
    const count=tr('Включено моделей: ')+modelEdit.enabled.size+' / '+modelEdit.limit;
    setStatus('agent-model-status',message||count);
  }
  function enableModel(id){
    if(modelEdit.enabled.size>=modelEdit.limit){
      modelStatus(tr('Достигнут предел. Выключите одну модель.'));
      return false;
    }
    modelEdit.enabled.add(id);modelStatus();return true;
  }
  async function openModelPicker(){
    const provider=setupProvider;
    const edit={provider,enabled:new Set(configuredModels(provider)),options:[],limit:12,busy:false};
    modelEdit=edit;modelSearch.value='';
    renderModelOptions();modelStatus();
    modelPicker.showModal();
    try{
      const catalog=await api('/api/model-catalog?provider='+encodeURIComponent(provider));
      if(modelEdit!==edit)return;
      edit.options=catalog.models||[];edit.limit=catalog.limit;
      renderModelOptions();
      if(!edit.busy)modelStatus();
    }catch(error){if(modelEdit===edit&&!edit.busy)modelStatus(error.message);}
  }
  async function saveAgentModels(){
    const edit=modelEdit;
    if(!edit||edit.busy)return;
    const models=[...edit.enabled];
    if(models.join('\n')===configuredModels(edit.provider).join('\n')){modelPicker.close();return;}
    edit.busy=true;
    $('agent-model-cancel').disabled=$('agent-model-save').disabled=true;
    renderModelOptions();
    try{
      await postAPI('/api/agent-models',{provider:edit.provider,models});
      if(view)view.coordinator_models={...view.coordinator_models,[edit.provider]:models};
      viewSignature='';await refresh();
      modelPicker.close();
    }catch(error){modelStatus(error.message);}
    finally{
      edit.busy=false;
      $('agent-model-cancel').disabled=$('agent-model-save').disabled=false;
      if(modelPicker.open)renderModelOptions();
    }
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
    if(taskSidebarPreview)bar.prepend($('list-toggle'));
    root.append(bar);
    const dragZone=e=>{
      if(e.button!==0||e.target.closest(titleInteractive))return false;
      if(e.target.closest('.am-tabs'))return true;
      if(taskSidebarPreview&&tab==='tasks'&&e.target.closest('.am-main')&&e.clientY<bar.getBoundingClientRect().bottom)return true;
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
    const sidebar=root.querySelector('.am-sidebar');
    const toolbar=root.querySelector('.am-task-toolbar');
    let navToggle;
    if(taskSidebarPreview&&toolbar){
      sidebar.id='computer-sidebar';
      navToggle=$('list-toggle').cloneNode(true);
      navToggle.id='computer-sidebar-toggle';
      navToggle.hidden=false;
      navToggle.setAttribute('aria-controls',sidebar.id);
      const setExpanded=expanded=>{
        root.classList.toggle('am-nav-collapsed',!expanded);
        navToggle.setAttribute('aria-expanded',String(expanded));
        const label=expanded?tr('Скрыть список компьютеров'):tr('Показать список компьютеров');
        navToggle.title=label;
        navToggle.setAttribute('aria-label',label);
      };
      setExpanded(true);
      navToggle.addEventListener('click',()=>setExpanded(navToggle.getAttribute('aria-expanded')!=='true'));
      sidebar.before(navToggle);
    }
    const box=r=>[r.left,r.top,r.width,r.height].map(Math.round);
    let last='',queued=false;
    const send=()=>{
      queued=false;
      const zones=[],holes=[];
      // A modal covers the title row, so its controls must receive clicks.
      if(!root.querySelector('dialog:modal')){
        const tabs=root.querySelector('.am-tabs'),side=root.querySelector('.am-sidebar');
        if(taskSidebarPreview&&tab==='tasks'){
          const main=root.querySelector('.am-main'),r=main.getBoundingClientRect();
          const height=parseFloat(getComputedStyle(main).paddingBlockStart);
          const bar=toolbar?.getBoundingClientRect();
          zones.push(bar?.width&&matchMedia('(min-width:621px)').matches?bar:new DOMRect(r.left,0,r.width,height));
        }else if(tabs&&tabs.offsetParent){const r=tabs.getBoundingClientRect();zones.push(new DOMRect(r.left,0,r.width,r.bottom));}
        if(side){const r=side.getBoundingClientRect(),first=side.firstElementChild?.getBoundingClientRect();zones.push(new DOMRect(r.left,r.top,r.width,Math.max(16,(first?first.top:r.top+40)-r.top)));}
        if(navToggle?.offsetWidth)holes.push(navToggle.getBoundingClientRect());
        for(const parent of [tabs,side,toolbar])for(const node of parent?parent.querySelectorAll(titleInteractive):[]){
          const r=node.getBoundingClientRect();
          if(r.width&&r.height&&zones.some(z=>r.left<z.right&&r.right>z.left&&r.top<z.bottom&&r.bottom>z.top))holes.push(r);
        }
      }
      const message={zones:zones.map(box),holes:holes.map(box)},text=JSON.stringify(message);
      if(text!==last){last=text;bridge.postMessage(message);}
    };
    const later=()=>{if(!queued){queued=true;requestAnimationFrame(send);}};
    new ResizeObserver(later).observe(document.documentElement);
    if(toolbar)new ResizeObserver(later).observe(toolbar);
    if(navToggle)new MutationObserver(later).observe(navToggle,{attributeFilter:['aria-expanded']});
    for(const node of document.querySelectorAll('.am-tabs,.am-sidebar,.am-task-toolbar'))new MutationObserver(later).observe(node,{childList:true,subtree:true,attributes:true});
    const modalObserver=new MutationObserver(later);
    for(const dialog of root.querySelectorAll('dialog'))modalObserver.observe(dialog,{attributeFilter:['open']});
    addEventListener('resize',later);
    later();
  }
  window.messengerRefresh=()=>{viewSignature='';refresh();};
  // «Выбрать»: the local server opens the system's own dialog (D-053); the
  // Mac app keeps its native folder dialog for the project folder.
  async function requestPath(kind,current){
    const response=await fetch('/api/pick',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({kind,current}),cache:'no-store',signal:AbortSignal.timeout(610000)});
    const result=await response.json().catch(()=>({}));
    if(!response.ok||result.ok!==true)throw Error(result.detail||tr('Окно выбора не открылось. Вставьте путь вручную.'));
    return result.path;
  }
  function pickerError(error){return error.name==='TimeoutError'?tr('Окно выбора не ответило. Вставьте путь вручную.'):error.message;}
  async function pick(field,kind,status){
    const input=$(field);$(status).textContent='';
    try{
      const path=await requestPath(kind,input.value.trim());
      if(path&&path!==input.value){input.value=path;input.dispatchEvent(new Event('change'));}
      $(status).textContent='';
    }catch(error){$(status).textContent=pickerError(error);}
  }
  // The Mac shell's folder picker answers here, into the field that asked.
  let folderTarget='project-folder';
  window.messengerFolderPicked=path=>{if(typeof path==='string'&&path){$(folderTarget).value=path;$(folderTarget).dispatchEvent(new Event('input'));}};
  // The window names a build the short way, «v0.41» for 0.0.0-preview.41
  // (owner); the release tag stays as it is because updates compare it. The
  // Mac app already reports the short form (0.58).
  function versionName(version){
    const preview=version.match(/^0\.0\.0-preview\.(\d+)$/);
    if(preview)return 'v0.'+preview[1];
    const release=version.match(/^([1-9]\d*\.\d+\.\d+)-preview\.\d+$/);
    return release?'v'+release[1].replace(/\.0$/,''):version?'v'+version:'';
  }
  function versionText(installed){return versionName(installed);}
  // The signed build number orders updates even when their display names
  // are identical. Older peer reports can use the short 0.N form.
  function updateBuildNumber(version){
    const match=String(version).match(/^v?(?:\d+\.\d+\.\d+-preview\.|0\.)(\d+)$/);
    return match?Number(match[1]):0;
  }
  function versionRank(version){
    const v=versionName(String(version||'').replace(/^v/,'')).replace(/^v/,'');
    return v.split(/[.-]/).map(x=>Number(x)||0);
  }
  function versionOlder(a,b){
    if(!a||!b)return false;
    const aBuild=updateBuildNumber(a),bBuild=updateBuildNumber(b);
    if(aBuild&&bBuild)return aBuild<bBuild;
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
  // The update icon names the action or shows its active progress.
  function updateTip(version,busy){
    let text=busy==='installing'?tr('Обновление устанавливается'):busy?tr('Обновление загружается'):version?tr('Обновить версию'):tr('Обновить');
    if(busy==='downloading'&&updateProgress)text=tr('Загружается обновление')+(updateProgress.version?' '+versionName(updateProgress.version):'')+': '+Math.floor(100*updateProgress.done/updateProgress.total)+'%';
    $('app-update-button').title=text;$('app-update-button').setAttribute('aria-label',text);
  }
  // The download helper writes progress for the dialog. The window checks
  // often while a transfer is visible and rarely otherwise.
  let updateProgress=null,progressTimer=0;
  async function pollUpdateProgress(){
    clearTimeout(progressTimer);
    let next=null;
    try{const r=await api('/api/update-progress');if(r?.downloading&&r.total>0)next={version:String(r.version||''),done:Number(r.done)||0,total:Number(r.total)};}catch(_){}
    const changed=!!next!==!!updateProgress;
    updateProgress=next;
    // The Mac downloads from GitHub before Sparkle installs: the run shows
    // that download (T-213), and progress keeps it from looking stuck.
    if(next&&updateRun?.mac&&!updateRun.error&&updateRun.phase==='downloading'){updateRun.done=next.done;updateRun.total=next.total;if(next.version)updateRun.version=next.version;macRunQuiet();renderMacRun();}
    else if(window.webkit?.messageHandlers?.updates){if(changed||next)renderMacUpdate();}
    else if(changed&&selfUpdate)renderSelfUpdate();
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
    const run=updateRun?.mac?updateRun:null;
    let busy='';
    if(run&&!run.error)busy=run.phase==='restarting'?'installing':run.phase;
    if(!run&&status.checking&&updateProgress&&version)busy='downloading';
    const button=$('app-update-button');
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Версия ')+versionName(installed).slice(1):'';
    if(version)knownUpdate=version;
    $('app-update-version').hidden=true;button.hidden=!version&&!busy;
    $('app-installed-version').textContent=versionText(installed);
    updateTip(version,busy);
    button.classList.toggle('am-update-active',!!busy);
    button.disabled=!!busy;
    renderDeviceMenuFooter();
  }
  // Windows and Linux: the program updates itself from the window (D-048).
  let selfUpdate=null,selfUpdateBusy=false,selfUpdateTimer=0;
  function renderSelfUpdate(){
    const s=selfUpdate;if(!s)return;
    const installed=typeof s.installed_version==='string'?s.installed_version:'';
    $('app-installed-version').hidden=!installed;
    $('app-installed-version').title=installed?tr('Версия ')+versionName(installed).slice(1):'';
    const version=typeof s.version==='string'?s.version:'';
    if(version&&s.state==='available')knownUpdate=version;
    // The dialog shows the progress after the owner presses the button.
    const offered=(s.state==='available'||s.state==='found')&&!!version;
    if(offered)knownUpdate=version;
    const installing=selfUpdateBusy||s.state==='installing'||s.state==='downloading';
    const done=s.state==='installed',shown=offered||installing||done||!!s.message;
    // The arrow while an update waits (owner, T-056, T-066). A line appears
    // only for an error or after the update.
    $('app-update-version').hidden=!s.message&&!done;$('app-update-button').hidden=!shown||done;
    $('app-installed-version').textContent=versionText(installed);
    $('app-update-version').textContent=s.message||(done?tr('Обновлено. Окно откроется заново.'):'');
    updateTip(version,s.state==='downloading'?'downloading':installing?'installing':'');
    $('app-update-button').disabled=installing;
    renderDeviceMenuFooter();
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
    renderMacUpdate();
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
  $('app-update-button').append(updateSpinner());
  if(!window.webkit?.messageHandlers?.updates){selfUpdateTimer=setInterval(loadSelfUpdate,60000);loadSelfUpdate();}
  pollUpdateProgress();
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('list-toggle').addEventListener('click',()=>{
    const expanded=$('list-toggle').getAttribute('aria-expanded')!=='true';
    $('list-toggle').setAttribute('aria-expanded',String(expanded));
    const label=taskSidebarPreview?(expanded?tr('Скрыть архив'):tr('Показать архив')):(expanded?tr('Скрыть список задач'):tr('Показать список задач'));$('list-toggle').setAttribute('aria-label',label);$('list-toggle').title=label;
    $('task-panel').hidden=!expanded;$('task-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  if(taskSidebarPreview){
    const toggle=$('list-toggle'),label=tr('Показать архив');
    const toolbar=el('header',undefined,'am-task-toolbar');
    toolbar.append($('task-head'),toggle);
    $('task-layout').prepend(toolbar);
    installPaneDropTargets();
    toggle.setAttribute('aria-expanded','false');toggle.setAttribute('aria-label',label);toggle.title=label;
    $('task-panel').setAttribute('aria-label',tr('Архив'));$('task-list-heading').textContent=tr('Архив');
    const plus=$('task-new');
    plus.hidden=false;
    plus.addEventListener('click',()=>{
      show('tasks');
      if(draftOpen){$('task-draft-text').focus();return;}
      openTaskDraft();loadServer();
    });
  }
  function selectProject(name,task='',computer,shared=null){
    if(draftOpen)closeTaskDraft();
    if(!taskSidebarPreview&&(!combinedTasks()||computer!==''))taskHistories.clear();
    setProject(name,computer);remember('messenger-project',name);
    paneActive=shared?null:taskPanes.get(task)||null;
    selectedTask=task;selectedShared=shared;feed=null;feedSignature='';viewSignature='';topTaskScope='';
    if($('device-menu')?.matches(':popover-open'))$('device-menu').hidePopover();
    renderProjects();renderMachines();if(tab==='agents')renderAgents();renderTaskList();renderThread();loadProjectActivity();refresh();
  }
  $('split-projects').addEventListener('click',()=>{
    if(splitProjects.length){
      remember('messenger-second-project',passiveProject());
      updateSplitProjects([]);
    }else{
      const options=(view?.projects||[]).filter(p=>p.name!==project);
      if(!project||!options.length)return;
      const saved=recall('messenger-second-project');
      updateSplitProjects([project,options.find(p=>p.name===saved)?.name||options[0].name]);
    }
    renderProjects();renderMachines();renderTaskList();loadProjectActivity();
  });
  for(const [index,id] of ['project-select','project-select-second'].entries()){
    $(id).addEventListener('change',()=>{
      const name=$(id).value;
      if(name==='__new'){renderProjects();openProjectDialog();return;}
      if(taskSidebarPreview){
        sidebarProject=name==='__all'?'':name;
        if(sidebarProject&&sidebarProject!==project&&!draftOpen)selectProject(sidebarProject);
        else{renderProjects();renderMachines();renderTaskList();loadProjectActivity();}
        return;
      }
      if(!name)return;
      if(!splitProjects.length){selectProject(name);return;}
      if(splitProjects[index]===name)return;
      const active=splitProjects[index]===project;
      const next=[...splitProjects];next[index]=name;
      updateSplitProjects(next);
      if(active)selectProject(name);
      else{renderProjects();renderMachines();renderTaskList();loadProjectActivity();}
      remember('messenger-second-project',passiveProject());
    });
  }
  $('agent-filter').addEventListener('change',()=>{taskHistories.clear();renderTaskList();loadProjectActivity();if(view?.history_paged)refresh();});
  // The sidebar and the task list are as wide as the owner drags them (T-063).
  // A double click or Home restores the usual width; the width is remembered.
  function splitter(pane,name,variable,usual,min,max,side){
    const handle=el('div',undefined,'am-splitter');
    handle.setAttribute('role','separator');handle.setAttribute('aria-orientation','vertical');handle.tabIndex=0;
    handle.title=tr('Потяните, чтобы изменить ширину. Дважды щёлкните, чтобы восстановить исходную ширину.');
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
    return handle;
  }
  const sidebar=root.querySelector('.am-sidebar'),sidebarWidth=tr('Ширина левой панели');
  const sidebarDefault=320;
  splitter(sidebar,sidebarWidth,'--am-nav',taskSidebarPreview?sidebarDefault:248,160,420,1).classList.add('am-nav-splitter');
  splitter(sidebar,sidebarWidth,'--am-nav-split',384,300,480,1).classList.add('am-projects-splitter');
  splitter($('task-panel'),tr('Ширина списка задач'),'--am-queue',taskSidebarPreview?sidebarDefault:290,200,520,-1);
  // A click on empty space in the sidebar shows all computers' tasks again.
  root.querySelector('.am-sidebar').addEventListener('click',event=>{if(computerFilter&&!event.target.closest('.am-machine-card,.am-project,.am-side-bottom,.am-splitter,button,select,input,a,[popover]'))clearComputer();});
  $('answer-text').addEventListener('input',()=>{sizeAnswer();answerControls();});
  $('answer-send').addEventListener('click',sendAnswer);
  // Enter sends; Shift+Enter adds a line. An IME composition keeps its Enter.
  $('task-stop').addEventListener('click',stopSelectedTask);
  $('lost-close').addEventListener('click',()=>closeHere());
  document.addEventListener('keydown',e=>{
    if(tab==='tasks'&&e.key==='Escape'&&!e.isComposing&&!$('task-stop').hidden&&!$('compose').hidden&&!$('compose').inert&&
      !document.querySelector('dialog[open]')&&!e.target.closest?.('[role=listbox],[aria-expanded=true],.am-pane-toolbar,.am-watch-pane')){
      e.preventDefault();stopSelectedTask();
    }
  });
  $('answer-text').addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing&&e.keyCode!==229){e.preventDefault();sendAnswer();}});
  $('project-dialog-close').addEventListener('click',()=>$('project-dialog').close());
  $('settings-open').addEventListener('click',()=>{
    $('auto-update-status').textContent='';
    if(taskSidebarPreview)show('settings');
    else{$('settings-dialog').showModal();loadServer();}
  });
  $('settings-dialog').addEventListener('close',renderNotices);
  if(taskSidebarPreview){
    const dialog=$('settings-dialog');
    const previousBody=dialog.querySelector('.am-dialog-body');
    const preferences=previousBody.querySelector('.am-prefs');
    const panel=el('div',undefined,'am-settings');
    const card=el('div',undefined,'am-card');
    card.append(preferences);
    panel.append(el('h3',tr('Общие')),card);
    $('settings-page').append(panel);
    const layout=el('div',undefined,'am-settings-layout');
    const sections=el('nav',undefined,'am-settings-nav');
    const views=el('div',undefined,'am-settings-views');
    sections.setAttribute('aria-label',tr('Разделы'));
    $('settings-general').hidden=false;
    const sectionIcons=[
      ['settings',$('settings-open').querySelector('svg').cloneNode(true)],
      ['project',svgIcon(['M4 6h5l2 2h9a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2Z'])],
      ['agents',svgIcon(['M7 7h10a4 4 0 0 1 4 4v5a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4v-5a4 4 0 0 1 4-4ZM12 7V3m-2 0h4M8.5 12v1m7-1v1M10 16.5h4'])],
      ['server',svgIcon(['M9 2h6v5H9zM12 7v5M5 16v-4h14v4M2 16h6v5H2zM16 16h6v5h-6z'])],
    ];
    for(const [section,icon] of sectionIcons){
      const button=root.querySelector(`.am-tabs [data-tab=${section}]`);
      button.prepend(icon);
      sections.append(button);
      views.append(root.querySelector(`[data-screen=${section}]`));
    }
    layout.append(sections,views);
    previousBody.replaceWith(layout);
    dialog.classList.add('am-settings-wide');
    dialog.addEventListener('close',()=>{
      if(dialog.open)return;
      if(tab!=='stats'){
        if(tab!=='tasks')show('tasks');
        $('settings-open').focus();
      }
    });
    const statistics=$('stats-dialog');
    statistics.append(root.querySelector('[data-screen=stats]'));
    $('stats-open').hidden=false;
    $('stats-open').addEventListener('click',()=>show('stats'));
    $('stats-close').addEventListener('click',()=>statistics.close());
    statistics.addEventListener('close',()=>{
      if(!statistics.open&&tab==='stats'){
        show('tasks');
        $('stats-open').focus();
      }
    });
  }
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
  // (owner, T-152). Keep the restart and active update dialogs open.
  for(const dialog of document.querySelectorAll('dialog')){
    if(dialog.id==='restart-dialog')continue;
    let downOutside=false;
    const outside=event=>{const r=dialog.getBoundingClientRect();return event.target===dialog&&(event.clientX<r.left||event.clientX>r.right||event.clientY<r.top||event.clientY>r.bottom);};
    dialog.addEventListener('mousedown',event=>{downOutside=outside(event);});
    dialog.addEventListener('click',event=>{
      if(downOutside&&outside(event)&&(dialog.id!=='update-dialog'||updateRun?.error))dialog.close();
      downOutside=false;
    });
  }
  $('invite-dialog').addEventListener('close',()=>{invitation=null;clearInterval(inviteTimer);});
  $('project-save').addEventListener('click',createProject);
  $('project-page-rules').addEventListener('input',()=>{clearTimeout(rulesTimer);rulesTimer=setTimeout(()=>{rulesTimer=0;saveProjectField('rules');},800);});
  $('project-page-rules').addEventListener('blur',()=>{if(rulesTimer){clearTimeout(rulesTimer);rulesTimer=0;saveProjectField('rules');}});
  $('project-page-name').addEventListener('change',()=>saveProjectField('title'));
  $('project-page-name').addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();saveProjectField('title');}});
  $('project-page-folder').addEventListener('change',()=>saveProjectField('folder'));
  $('project-page-folder').addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();saveProjectField('folder');}});
  $('project-page-remove').addEventListener('click',()=>askRemoveProject());
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
  $('restart-dialog').addEventListener('cancel',event=>{
    event.preventDefault();if(restart.wait){restartWaitDetails=false;renderRestart();}
  });
  $('restart-go').addEventListener('click',()=>{if(restart.ask){restart.wait=true;renderRestart();restartClient(false);}else restartClient(false);});
  $('restart-cut').addEventListener('click',()=>{restart.wait=false;restartClient(true);});
  $('restart-wait-show').addEventListener('click',()=>{restartWaitDetails=true;renderRestart();});
  $('restart-quit').addEventListener('click',()=>{
    if(restart.wait){restartWaitDetails=false;renderRestart();return;}
    window.webkit?.messageHandlers?.app?.postMessage('quit');
  });
  $('restart-copy').addEventListener('click',async()=>{const b=$('restart-copy');try{await navigator.clipboard.writeText(shutdownCommand());b.textContent=tr('Скопировано');}catch(_){b.textContent=tr('Не удалось скопировать');}setTimeout(()=>b.textContent=tr('Скопировать команду'),2000);});
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-cli').addEventListener('change',saveAgentPaths);
  $('agent-profile').addEventListener('change',saveAgentPaths);
  $('agent-models-open').addEventListener('click',openModelPicker);
  modelSearch.addEventListener('input',()=>{renderModelOptions();$('agent-model-options').scrollTop=0;});
  $('agent-model-add').addEventListener('click',()=>{
    const id=modelSearch.value.trim();
    if(!id||modelEdit.busy||!enableModel(id))return;
    modelSearch.value='';renderModelOptions();modelSearch.focus();
  });
  $('agent-model-save').addEventListener('click',saveAgentModels);
  $('agent-model-cancel').addEventListener('click',()=>modelPicker.close());
  modelPicker.addEventListener('cancel',event=>{if(modelEdit?.busy)event.preventDefault();});
  modelPicker.addEventListener('close',()=>{if(!modelPicker.open)modelEdit=null;});
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
  const sidebarFilterIcon=taskSidebarPreview?['M3 5h9m4 0h5M14 2v6M3 12h3m4 0h11M8 9v6M3 19h10m4 0h4M15 16v6']:null;
  if(taskSidebarPreview){
    $('sidebar-group-control').hidden=false;
    enhanceSelect(groupSelect,false,['M12 3 2 8l10 5 10-5-10-5ZM2 12l10 5 10-5M2 16l10 5 10-5']);
    groupSelect.addEventListener('change',()=>{remember('kotel-sidebar-group',groupSelect.value);renderMachines();});
  }
  $('lang-select').value=amLang;enhanceSelect($('lang-select'));enhanceSelect($('theme-select'));enhanceSelect($('project-select'),false,sidebarFilterIcon);enhanceSelect($('project-select-second'));enhanceSelect($('agent-filter'),false,sidebarFilterIcon);enhanceSelect($('stats-project'));enhanceSelect($('stats-agent'));enhanceSelect($('stats-model'));enhanceSelect($('host-scope'));enhanceSelect($('host-address'));
  if(taskSidebarPreview){
    enhanceSelect($('stats-computer'));enhanceSelect($('agents-computer'));
    const draftContextIcons={
      'task-draft-project':['M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z'],
      'task-draft-computer':serverIconPaths
    };
    for(const id of ['task-draft-project','task-draft-computer','task-draft-model']){
      const select=$(id),paths=draftContextIcons[id];
      const trigger=enhanceSelect(select,id==='task-draft-model');
      if(paths)trigger.prepend(svgIcon(paths));
      select.addEventListener('change',()=>{
        if(id==='task-draft-project'&&select.value==='__new'){
          renderTaskDraft(select.dataset.selectedProject);
          openProjectDialog(true);
          $('project-pick').click();
        }else renderTaskDraft();
      });
    }
    setupComposerPaths();
    setupComposerTokens();
    $('task-draft').addEventListener('submit',sendTaskDraft);
    $('task-draft-text').addEventListener('input',event=>{
      sizeComposer(event.currentTarget);
      if(draftAttempt?.error)draftAttempt.error='';
      renderTaskDraft();
    });
    $('task-draft-text').addEventListener('keydown',event=>{
      if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();$('task-draft').requestSubmit();}
    });
    $('agents-computer').addEventListener('change',()=>{agentComputer=$('agents-computer').value;renderAgents();});
  }
  function setupComposerDictation(){
    const bridge=window.webkit?.messageHandlers?.composerDictation;
    if(!bridge)return null;
    const messages={
      recording:tr('Идёт диктовка. Нажмите на микрофон, чтобы вставить текст.'),
      processing:tr('Распознаём речь…'),
      unavailable:tr('Для языка интерфейса недоступно распознавание речи на этом Mac.'),
      'microphone-denied':tr('Разрешите Kotel доступ к микрофону в настройках macOS.'),
      'speech-denied':tr('Разрешите Kotel распознавание речи в настройках macOS.'),
      'microphone-unavailable':tr('Микрофон недоступен.'),
      empty:tr('Не удалось расслышать речь.'),
      failed:tr('Не удалось распознать речь. Попробуйте снова.')
    };
    const microphone=['M9 6a3 3 0 0 1 6 0v6a3 3 0 0 1-6 0z','M5 11v1a7 7 0 0 0 14 0v-1m-7 8v3'];
    const square=['M7 7h10v10H7z'];
    const targets=[];
    let active=null;
    function context(target){
      const input=target.input;
      if(tab!=='tasks'||input.disabled||input.readOnly||input.closest('[hidden],[inert]'))return '';
      if(input.id==='task-draft-text'){
        return draftOpen&&!draftSending?JSON.stringify(['draft',...['project','computer','model'].map(key=>$('task-draft-'+key).value)]):'';
      }
      return !draftOpen&&!answering&&selectedTask&&$('compose').dataset.task===selectedTask?
        JSON.stringify(['answer',project,selectedTask]):'';
    }
    function paint(target,event=''){
      const pressed=active?.target===target;
      const label=event==='recording'?tr('Завершить диктовку'):pressed?tr('Отменить диктовку'):tr('Диктовать');
      target.button.setAttribute('aria-pressed',String(pressed));
      target.button.setAttribute('aria-label',label);
      target.button.title=label;
      target.button.replaceChildren(svgIcon(event==='recording'?square:microphone));
      target.status.textContent=messages[event]||'';
      target.status.hidden=!target.status.textContent;
      target.status.classList.toggle('am-status-bad',!!event&&event!=='recording'&&event!=='processing');
    }
    function send(action,id){
      bridge.postMessage({action,id,language:document.documentElement.lang});
    }
    function cancel(){
      if(!active)return;
      const {id,target}=active;
      active=null;
      paint(target);
      send('cancel',id);
    }
    function sync(){
      for(const target of targets){
        const key=context(target);
        if(active?.target===target&&active.key!==key)cancel();
        if(target.key!==key&&active?.target!==target)paint(target);
        target.key=key;
        target.button.hidden=!key;
      }
    }
    function finishFor(input){
      if(active?.target.input!==input)return false;
      if(active.event==='recording'){
        active.event='processing';
        paint(active.target,'processing');
        send('stop',active.id);
      }
      return true;
    }
    for(const input of [$('task-draft-text'),$('answer-text')]){
      const button=el('button',undefined,'am-send-icon am-quiet-button am-dictation-button');
      const status=el('p',undefined,'am-small am-muted am-dictation-status');
      button.type='button';
      button.hidden=true;
      status.setAttribute('role','status');
      input.after(button);
      input.parentElement.after(status);
      const target={input,button,status,key:''};
      targets.push(target);
      paint(target);
      button.addEventListener('click',()=>{
        if(active?.target===target){
          if(active.event==='recording')finishFor(input);else cancel();
          return;
        }
        const key=context(target);
        if(!key)return;
        cancel();
        active={id:crypto.randomUUID(),target,key,event:'',value:input.value,start:input.selectionStart,end:input.selectionEnd};
        paint(target);
        send('start',active.id);
      });
    }
    window.messengerDictationEvent=({id,event,text})=>{
      if(active?.id!==id)return;
      const {target,key,value,start,end}=active;
      if(context(target)!==key){cancel();return;}
      if(event==='recording'||event==='processing'){
        active.event=event;
        paint(target,event);
        return;
      }
      active=null;
      paint(target,event);
      if(event!=='text'||!text)return;
      const input=target.input;
      const from=value===input.value?start:input.selectionStart;
      const to=value===input.value?end:input.selectionEnd;
      const before=input.value.slice(0,from),after=input.value.slice(to);
      const insertion=(before&&!/\s$/.test(before)?' ':'')+text+(/^\S/.test(after)?' ':'');
      const limit=input.maxLength-before.length-after.length;
      if(limit>0){
        input.setRangeText(insertion.slice(0,limit),from,to,'end');
        input.dispatchEvent(new Event('input',{bubbles:true}));
      }
      input.focus();
    };
    window.messengerCancelDictation=cancel;
    document.addEventListener('keydown',event=>{
      if(event.key!=='Escape'||!active||event.isComposing)return;
      event.preventDefault();
      event.stopImmediatePropagation();
      cancel();
    },true);
    window.addEventListener('pagehide',cancel);
    return {cancel,sync,finishFor};
  }
  composerDictation=setupComposerDictation();
  centreControlText();
  applyTheme(initialTheme||'system');
  const chatFont=$('chat-font-select');
  chatFont.value=window.messengerInitialChatFont||recall('messenger-chat-font');
  applyChatFont();
  enhanceSelect(chatFont);
  chatFont.addEventListener('change',()=>applyChatFont(true));
  const noticeDismissal=$('notice-dismissal');
  noticeDismissal.value=autoCloseNotices?'automatic':'manual';
  enhanceSelect(noticeDismissal);
  noticeDismissal.addEventListener('change',()=>{
    autoCloseNotices=noticeDismissal.value==='automatic';
    remember(noticePreference,noticeDismissal.value);
    notices.forEach(scheduleNoticeClose);
  });
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
  root.querySelectorAll('.am-screen:not(.am-settings-views > .am-screen),.am-thread,.am-queue-list,.am-dialog-body,.am-machines,.am-stats-reports').forEach(element=>gutterObserver.observe(element));
  const chatBody=root.querySelector('.am-chat-body'),chatFooter=root.querySelector('.am-chat-footer');
  $('thread').addEventListener('scroll',toBottomState,{passive:true});
  $('to-bottom').addEventListener('click',()=>{const t=$('thread');t.scrollTo({top:t.scrollHeight,behavior:'smooth'});});
  // A taller reply box must not pull the owner off the end of the feed.
  new ResizeObserver(()=>requestAnimationFrame(()=>{
    updateNoticeTooltips();
    chatBody.style.setProperty('--am-footer-height',chatFooter.getBoundingClientRect().height+'px');
    const t=$('thread');if(feedPinned)t.scrollTop=t.scrollHeight;
  })).observe(chatFooter);
  new ResizeObserver(updateNoticeTooltips).observe($('notices'));
  if(taskSidebarPreview){
    try{
      for(const saved of JSON.parse(recall('kotel-task-panes')||'[]')){
        if(saved?.id&&saved.project&&saved.title)openTaskPane(saved,saved.project,saved.placement==='tab'?'tab':'alongside',true);
      }
      paneActive=taskPanes.get(recall('kotel-active-pane'))||taskPanes.values().next().value||null;
    }catch(_){}
  }
  show('tasks');loadMachineOrder();refresh();loadDiscovery();pollSetup();setInterval(()=>{if(setupPending)pollSetup();},2000);
  setInterval(updateTimers,1000);
  setInterval(loadProjectActivity,3000);
  setInterval(()=>{if(!windowWatch||Date.now()-lastWindowRefresh>=15000)refresh();},2000);
  setInterval(loadDiscovery,60000);setInterval(()=>{if(tab==='stats'&&!document.hidden)loadStats();},15000);
  document.addEventListener('visibilitychange',()=>{syncTaskPanes();if(document.hidden)return;updateTimers();loadServer();loadProjectActivity();if(tab==='stats')loadStats();});
})();
