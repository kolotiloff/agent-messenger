'use strict';
(() => {
  const root = document.getElementById('am-concept');
  if(window.webkit?.messageHandlers?.updates)document.body.classList.add('am-native');
  const $ = id => root.querySelector('#' + id);
  const selectViews = new Map();
  function syncSelects(){selectViews.forEach(view=>view.sync());}
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
  const providerPaths = {"claude":["m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z"],"cursor":["M11.503.131 1.891 5.678a.84.84 0 0 0-.42.726v11.188c0 .3.162.575.42.724l9.609 5.55a1 1 0 0 0 .998 0l9.61-5.55a.84.84 0 0 0 .42-.724V6.404a.84.84 0 0 0-.42-.726L12.497.131a1.01 1.01 0 0 0-.996 0M2.657 6.338h18.55c.263 0 .43.287.297.515L12.23 22.918c-.062.107-.229.064-.229-.06V12.335a.59.59 0 0 0-.295-.51l-9.11-5.257c-.109-.063-.064-.23.061-.23"],"codex":["M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"]};
  function applyTheme(theme, persist=false) {
    if(!['system','light','dark'].includes(theme))theme='system';
    document.documentElement.dataset.theme=theme;$('theme-select').value=theme;syncSelects();
    if(persist) {
      if(window.webkit?.messageHandlers?.appearance)window.webkit.messageHandlers.appearance.postMessage(theme);
      else try {localStorage.setItem('messenger-theme',theme);} catch(_) {}
    }
  }
  // Task window (D-024, D-026, T-029). The window observes tasks; the owner
  // talks to the coordinator in the Claude app. Its only actions are creating
  // a project, changing the project folder, connecting an agent program and
  // answering a worker's question to the owner.
  const names={claude:'Claude Code',codex:'Codex'};
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
  let online=false,discovery=[],tab='tasks',refreshing=false,answering=false;
  const agentChecks=new Map();
  async function api(path,timeout=4500){
    if(!token)throw Error('Откройте приватную ссылку, которую выдала команда ui.');
    const response=await fetch(path,{headers:{Authorization:'Bearer '+token},cache:'no-store',signal:AbortSignal.timeout(timeout)});
    if(!response.ok){if(response.status===401)throw Error('Ссылка доступа устарела. Откройте окно заново.');throw Error('Клиент Messenger недоступен.');}
    return response.json();
  }
  async function postAPI(path,body,timeout=6000){
    const response=await fetch(path,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body),cache:'no-store',signal:AbortSignal.timeout(timeout)});
    let result={};try{result=await response.json();}catch(_){}
    if(response.ok&&result.ok===true)return result;
    if(response.status===409&&result.detail)throw Error(result.detail);
    if(response.status===400)throw Error('Проверьте введённые данные.');
    throw Error('Действие не подтверждено. Проверьте состояние и повторите.');
  }
  function minutes(from,to){
    const a=Date.parse(from),b=to?Date.parse(to):Date.now();
    if(isNaN(a)||isNaN(b))return '';
    const m=Math.max(0,Math.round((b-a)/60000));
    if(m<1)return 'меньше минуты';
    if(m<60)return m+' мин';
    const h=Math.floor(m/60);return h+' ч'+(m%60?' '+m%60+' мин':'');
  }
  function clock(at){const d=new Date(at);return isNaN(d.getTime())?'':d.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});}
  const activeStates=['running','waiting_owner','waiting_coordinator'];
  function taskStatus(t){
    const end=t.finished_at||t.updated_at;
    switch(t.state){
      case 'running':return ['Работает '+minutes(t.created_at),'am-status-run'];
      case 'waiting_owner':return ['Ждёт вашего ответа','am-status-wait'];
      case 'waiting_coordinator':return ['Ждёт ответа координатора','am-status-run'];
      case 'done':return ['Завершил за '+minutes(t.created_at,end),'am-muted'];
      case 'failed':return ['Не прошёл проверку через '+minutes(t.created_at,end),'am-status-bad'];
      case 'interrupted':return ['Прервался через '+minutes(t.created_at,end),'am-status-bad'];
      case 'blocked':return ['Заблокирован через '+minutes(t.created_at,end),'am-status-bad'];
      case 'cancelled':return ['Отменён','am-muted'];
    }
    return [t.state,'am-muted'];
  }
  function coordinatorName(){
    const c=(view?.coordinator?.client||'').toLowerCase();
    if(c.includes('codex'))return ['codex','Codex'];
    if(c.includes('claude')||!c)return ['claude','Claude'];
    return ['',view.coordinator.client];
  }
  function agentState(a){
    if(!online)return ['offline','Нет связи'];
    if(a.waiting_owner>0)return ['wait','Ждёт ответа'];
    if(a.running>0)return ['run',a.running>1?'Работает ('+a.running+')':'Работает'];
    return ['idle','Свободен'];
  }
  function renderProjects(){
    const select=$('project-select');select.replaceChildren();
    for(const p of view?.projects||[]){const o=el('option',p.name);o.value=p.name;select.append(o);}
    const add=el('option','Новый проект…');add.value='__new';select.append(add);
    select.value=view?.project||'__new';
    if(!view?.project&&select.options.length>1)select.selectedIndex=0;
    select.disabled=!view;syncSelects();
  }
  function renderMachines(){
    const box=$('machines');box.replaceChildren();
    const machine=el('div',undefined,'am-machine am-machine-card');machine.setAttribute('aria-current','true');
    const head=el('div',undefined,'am-machine-name');head.append(el('span',view?.computer_name||'Этот компьютер'));
    const dot=el('span',undefined,'am-dot'+(online?' online':''));dot.title=online?'На связи':'Нет связи';head.append(dot);machine.append(head);
    const lines=el('div',undefined,'am-machine-agents');
    if(view?.coordinator?.connected){
      const [provider,label]=coordinatorName();
      const line=el('div',undefined,'am-agent-line');const d=el('span',undefined,'am-dot am-dot-'+(online?'run':'offline'));
      line.title=online?'Координатор подключён к проекту':'Нет связи';line.append(d,el('span',label),el('span','координатор','am-muted am-small'));
      if(provider)line.dataset.provider=provider;lines.append(line);
    }
    for(const a of view?.agents||[]){
      const [state,text]=agentState(a);
      const line=el('div',undefined,'am-agent-line');line.title=(names[a.agent]||a.agent)+': '+text;
      line.append(el('span',undefined,'am-dot am-dot-'+state),el('span',names[a.agent]||a.agent));lines.append(line);
    }
    if(!lines.children.length)lines.append(el('p',online?'Агенты не подключены':'Нет связи','am-small am-muted'));
    machine.append(lines);box.append(machine);
  }
  function visibleTasks(){
    const filter=$('agent-filter').value;
    const list=(view?.tasks||[]).filter(t=>!filter||t.agent===filter);
    const active=list.filter(t=>activeStates.includes(t.state)),done=list.filter(t=>!activeStates.includes(t.state));
    return [active,done];
  }
  function renderTaskList(){
    const box=$('task-list');box.replaceChildren();
    const [active,done]=visibleTasks();
    $('task-count').textContent=view?.tasks?.length?String(view.tasks.length):'';
    const section=(label,items)=>{
      if(!items.length)return;
      box.append(el('div',label,'am-label am-task-section'));
      for(const t of items){
        const row=el('button',undefined,'am-row am-task');row.type='button';row.setAttribute('aria-pressed',String(t.id===selectedTask));
        const agent=el('span',undefined,'am-task-agent');agent.append(providerIcon(t.agent),el('span',names[t.agent]||t.agent));
        const [text,cls]=taskStatus(t);
        const status=el('span',undefined,'am-task-status '+cls);
        if(t.state==='waiting_owner')status.append(el('span',undefined,'am-dot am-dot-wait'));
        status.append(el('span',text));
        row.append(agent,el('span',t.title,'am-task-title'),status);
        row.addEventListener('click',()=>{selectedTask=t.id;feedSignature='';renderTaskList();loadFeed();});
        box.append(row);
      }
    };
    section('Активные',active);section('Завершённые',done);
    if(!box.children.length)box.append(el('p',view?.project?'Задач пока нет':'','am-small am-muted'));
  }
  function connectHelp(){
    const box=el('div',undefined,'am-settings am-connect');
    box.append(el('h3','Подключите координатора'));
    box.append(el('p','Задачи проекта даёт ваш чат в приложении Claude. Откройте папку проекта в Claude (вкладка Code, этот компьютер) и выполните в Терминале команду ниже. Затем напишите в чате, что нужно сделать.'));
    const command='cd "'+view.folder+'" && claude mcp add --scope local messenger -- "'+view.messenger+'" --root "'+view.data_root+'" coordinator --project "'+view.project+'"';
    const pre=el('pre',command,'am-command');box.append(pre);
    const copy=el('button','Скопировать команду');copy.type='button';
    copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(command);copy.textContent='Скопировано';}catch(_){copy.textContent='Скопируйте вручную';}setTimeout(()=>copy.textContent='Скопировать команду',2000);});
    box.append(copy,el('p','Чат-координатор видит только задачи этого проекта.','am-small am-muted'));
    return box;
  }
  function renderThread(){
    const thread=$('thread'),head=$('task-head');
    $('compose').hidden=true;feedSignature='';
    if(!view){head.replaceChildren();return;}
    if(!view.project){
      head.replaceChildren();
      const box=el('div',undefined,'am-settings am-connect');box.append(el('h3','Создайте проект'),el('p','Проект связывает чат-координатора, папку проекта на этом компьютере и задачи.'));
      const b=el('button','Новый проект…','am-primary');b.type='button';b.addEventListener('click',()=>openProjectDialog(false));box.append(b);
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
  function renderFeed(){
    const t=feed.task,thread=$('thread'),head=$('task-head');
    const signature=JSON.stringify(feed);if(signature===feedSignature)return;
    const atBottom=thread.scrollHeight-thread.clientHeight-thread.scrollTop<40||!feedSignature;feedSignature=signature;
    const title=el('div',undefined,'am-task-head');title.append(el('h3',t.title));
    title.append(el('p',(names[t.agent]||t.agent)+' на '+(view.computer_name||'этом компьютере')+', папка '+t.folder,'am-small am-muted'));
    const [statusText,statusCls]=taskStatus(t);
    const when=t.finished_at?'Начата в '+clock(t.created_at)+', закончена в '+clock(t.finished_at):'Начата в '+clock(t.created_at);
    const line=el('p',undefined,'am-small');line.append(el('span',statusText,statusCls),el('span',', '+when.charAt(0).toLowerCase()+when.slice(1),'am-muted'));title.append(line);
    head.replaceChildren(title);
    const [coordProvider,coordLabel]=coordinatorName();
    const out=[];let group=null;
    const outgoing=(author,provider,role,text,at)=>{
      group=null;const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message am-outgoing');
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      const who=el('strong',author);meta.append(who);if(role)meta.append(roleTag(role));meta.append(el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');content.append(el('p',text));body.append(meta,content);row.append(body);g.append(row);out.push(g);
    };
    const workerGroup=at=>{
      if(group)return group;
      const g=el('article',undefined,'am-message-group');const row=el('div',undefined,'am-message');
      const avatar=el('div',undefined,'am-avatar');avatar.append(providerIcon(t.agent));
      const body=el('div',undefined,'am-message-body');const meta=el('div',undefined,'am-meta');
      meta.append(el('strong',names[t.agent]||t.agent),roleTag('исполнитель'),el('time',clock(at)));
      const content=el('div',undefined,'am-message-content');body.append(meta,content);row.append(avatar,body);g.append(row);out.push(g);group=content;return content;
    };
    const delivery=(text,cls)=>el('div',text,'am-delivery '+cls);
    for(const it of feed.items){
      if(it.kind==='task'){outgoing(coordLabel,coordProvider,'координатор',it.text,it.at);continue;}
      if(it.kind==='answer'){if(it.author==='owner')outgoing('Вы','owner','',it.text,it.at);else outgoing(coordLabel,coordProvider,'координатор',it.text,it.at);continue;}
      if(it.kind==='text'){workerGroup(it.at).append(el('p',it.text));continue;}
      if(it.kind==='tool'){
        const d=el('details',undefined,'am-tool');const s=el('summary',it.title||'Действие');d.append(s);
        if(it.text)d.append(el('pre',it.text));workerGroup(it.at).append(d);continue;
      }
      if(it.kind==='report'||it.kind==='question'){
        const c=workerGroup(it.at);c.append(el('p',it.text,'am-disposition'));
        if(!it.verified)c.append(delivery('Не опубликовано: запуск не прошёл проверку','am-status-bad'));
        else if(it.kind==='report')c.append(delivery('Отчёт отправлен координатору, сессия закрыта','am-status-good'));
        else if(it.audience==='owner')c.append(delivery('Вопрос вам, координатор его не видит','am-status-wait'));
        else c.append(delivery('Вопрос координатору','am-status-wait'));
        group=null;continue;
      }
      if(it.kind==='outcome'){
        group=null;const text={failed:'Запуск не прошёл проверку',blocked:'Задача заблокирована: процесс не завершился полностью',interrupted:'Клиент перезапустился во время работы; результат неизвестен',cancelled:'Задача отменена'}[it.state]||it.state;
        const note=el('div',undefined,'am-outcome '+(it.state==='cancelled'?'am-muted':'am-status-bad'));note.append(el('p',text));if(it.text)note.append(el('p',it.text,'am-small am-muted'));
        const g=el('article',undefined,'am-message-group');g.append(note);out.push(g);
      }
    }
    thread.replaceChildren(...out);
    const q=t.owner_question;
    $('compose').hidden=!q;
    if(q&&$('compose').dataset.question!==q.id){$('compose').dataset.question=q.id;$('answer-text').value='';$('answer-status').textContent='';}
    answerControls();
    if(atBottom)thread.scrollTop=thread.scrollHeight;
  }
  function answerControls(){$('answer-send').disabled=answering||!$('answer-text').value.trim()||!feed?.task?.owner_question;}
  async function sendAnswer(){
    const q=feed?.task?.owner_question;if(!q||answering)return;
    const text=$('answer-text').value.trim();if(!text)return;
    let op=recall('messenger-answer-op:'+q.id);
    if(!op){op='window-'+q.id+'-'+crypto.randomUUID();remember('messenger-answer-op:'+q.id,op);}
    answering=true;answerControls();$('answer-status').textContent='Отправляем ответ…';
    try{await postAPI('/api/task-answer',{task:feed.task.id,question:q.id,op_id:op,text});$('answer-status').textContent='Ответ записан. Исполнитель продолжит в той же сессии.';$('answer-text').value='';feedSignature='';await refresh();}
    catch(error){$('answer-status').textContent=error.message+' Повтор с тем же текстом безопасен.';}
    finally{answering=false;answerControls();}
  }
  const folderIcon='<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>';
  function renderAgents(){
    const bar=$('folder-bar');bar.hidden=!view?.project;bar.replaceChildren();
    if(view?.project){
      const icon=el('span',undefined,'am-folder-icon');icon.innerHTML=folderIcon;
      const edit=el('button','Изменить');edit.type='button';edit.addEventListener('click',()=>openProjectDialog(true));
      bar.append(icon,el('span','Рабочая папка проекта «'+view.project+'»','am-folder-title'),el('span',view.folder||'не выбрана','am-folder-path'),edit);
      bar.querySelector('.am-folder-path').title=view.folder||'';
    }
    const cards=$('agents');cards.replaceChildren();
    for(const provider of ['claude','codex']){
      const a=(view?.agents||[]).find(x=>x.agent===provider);
      const found=discovery.find(d=>d.provider===provider);
      const coordinator=view?.coordinator?.connected&&coordinatorName()[0]===provider;
      const card=el('article',undefined,'am-agent');
      const heading=el('h3',undefined,'am-provider-heading');heading.append(providerIcon(provider),el('span',names[provider]));card.append(heading);
      card.append(el('p',coordinator&&a?'Координатор и исполнитель':coordinator?'Координатор':a?'Исполнитель':'Не подключён','am-muted'));
      const dl=el('dl');
      const pair=(label,value,cls,extra)=>{const p=el('div');const dd=el('dd',undefined,cls);dd.append(el('span',value));if(extra)dd.append(...extra);p.append(el('dt',label),dd);dl.append(p);};
      if(!a){
        pair('Программа',found?.state==='candidate_found'?'Найдена, не подключена':found?.state==='not_found'?'Не найдена':'Не проверена',found?.state==='not_found'?'am-status-bad':'am-status-wait');
      }else{
        pair('Программа','Подключена','am-status-good');
      }
      const check=agentChecks.get(provider);
      const auth=check?.authentication?.state;
      pair('Вход в аккаунт',auth==='authenticated'?'Выполнен':auth==='not_authenticated'?'Не выполнен':'Не проверен',!a?'am-muted':auth==='authenticated'?'am-status-good':auth==='not_authenticated'?'am-status-bad':'am-status-wait');
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
      }
      card.append(dl);
      const running=a?.project_running||0,today=a?.project_today||0;
      const tasks=el('button',running||today?'Задачи: '+running+' идёт, '+today+' сегодня':'Задач нет','am-agent-action');tasks.type='button';tasks.disabled=!running&&!today;
      tasks.addEventListener('click',()=>{$('agent-filter').value=provider;syncSelects();show('tasks');renderTaskList();});
      const setup=el('button','Настроить');setup.type='button';setup.addEventListener('click',()=>openAgentSetup(provider));
      card.append(tasks,setup);cards.append(card);
    }
  }
  function renderNotice(){
    if(!online){notice(el('p','Клиент Messenger недоступен. Данные не обновляются.'));return;}
    if(view?.pending_launch>0&&!view.tasks_enabled){notice(el('p','Задачи записаны, но этот клиент не запускает исполнителей: он запущен без разрешения --allow-tasks.'));return;}
    if(view?.pending_launch>0&&view.paused){
      const box=el('div',undefined,'am-notice-row');box.append(el('p','Обработка на паузе: новые задачи не запускаются.'));
      const go=el('button','Продолжить');go.type='button';go.addEventListener('click',async()=>{go.disabled=true;try{await postAPI('/api/resume',{});await refresh();}catch(e){go.disabled=false;}});
      box.append(go);notice(box);return;
    }
    notice(null);
  }
  function render(){
    renderProjects();renderMachines();renderTaskList();renderThread();renderAgents();renderNotice();
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
      const v=await api('/api/window?project='+encodeURIComponent(project));online=true;
      if(!v.projects.some(p=>p.name===project))project=v.project||'';
      const signature=JSON.stringify(v);
      if(signature!==viewSignature){viewSignature=signature;view=v;if(selectedTask&&!v.tasks.some(t=>t.id===selectedTask)){selectedTask='';feed=null;feedSignature='';}render();}
      else{renderMachines();renderNotice();}
      if(selectedTask)await loadFeed();
    }catch(error){online=false;renderMachines();renderNotice();}
    finally{refreshing=false;}
  }
  async function loadDiscovery(){try{const d=await api('/api/discovery',8000);discovery=d.agents||[];renderAgents();}catch(_){}}
  function show(next){
    tab=next;root.querySelectorAll('[data-screen]').forEach(n=>n.hidden=n.dataset.screen!==tab);
    root.querySelectorAll('[data-tab]').forEach(n=>n.setAttribute('aria-pressed',String(n.dataset.tab===tab)));
    $('list-toggle').hidden=tab!=='tasks';$('screen-title').textContent={tasks:'Задачи',agents:'Агенты',server:'Сервер'}[tab];
  }
  let editingFolder=false;
  function openProjectDialog(edit){
    editingFolder=edit;
    $('project-dialog-title').textContent=edit?'Папка проекта «'+view.project+'»':'Новый проект';
    $('project-name-field').hidden=edit;$('project-name').value='';$('project-folder').value=edit?view.folder:'';
    $('project-save').textContent=edit?'Сохранить папку':'Создать проект';$('project-status').textContent='';
    $('project-pick').hidden=!window.webkit?.messageHandlers?.folder;
    $('project-dialog').showModal();
  }
  async function saveProject(){
    const name=editingFolder?view.project:$('project-name').value.trim(),folder=$('project-folder').value.trim();
    if(!name||!folder){$('project-status').textContent='Укажите имя и папку.';return;}
    $('project-save').disabled=true;$('project-status').textContent='Сохраняем…';
    try{
      await postAPI(editingFolder?'/api/project-folder':'/api/project-create',{name,folder});
      if(!editingFolder){project=name;remember('messenger-project',name);}
      $('project-dialog').close();viewSignature='';await refresh();
    }catch(error){$('project-status').textContent=error.message;}
    finally{$('project-save').disabled=false;}
  }
  let setupProvider='';
  function openAgentSetup(provider){
    setupProvider=provider;const a=(view?.agents||[]).find(x=>x.agent===provider),found=discovery.find(d=>d.provider===provider);
    $('agent-setup-title').textContent='Настройка: '+names[provider];
    $('agent-cli').value=a?.cli||found?.cli||'';$('agent-profile').value=a?.environment||found?.environment||'';
    $('agent-check-status').textContent='';$('agent-save-status').textContent='';$('agent-check-result').hidden=true;
    $('agent-setup').showModal();
  }
  async function checkAgent(){
    const selection={provider:setupProvider,cli:$('agent-cli').value.trim(),environment:$('agent-profile').value.trim()};
    if(!selection.cli||!selection.environment){$('agent-check-status').textContent='Укажите программу и каталог профиля.';return;}
    $('agent-check').disabled=true;$('agent-check-status').textContent='Проверяем версию и вход…';
    try{
      const response=await fetch('/api/check-cli',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(selection),cache:'no-store',signal:AbortSignal.timeout(22000)});
      if(!response.ok)throw Error(response.status===409?'Проверка уже выполняется.':response.status===422?'Проверьте путь к программе и профилю.':'Проверка не выполнена.');
      const result=await response.json();if(result.ok!==true)throw Error('Ответ проверки не подтверждён.');
      agentChecks.set(setupProvider,result.check);
      const dl=$('agent-check-result');dl.replaceChildren();
      for(const [label,value] of [['Версия',result.check.version?.value||'Не определена'],['Вход в аккаунт',{authenticated:'Выполнен',not_authenticated:'Не выполнен'}[result.check.authentication?.state]||'Не удалось определить']]){const p=el('div');p.append(el('dt',label),el('dd',value));dl.append(p);}
      dl.hidden=false;$('agent-check-status').textContent='Проверка завершена.';renderAgents();
    }catch(error){$('agent-check-status').textContent=error.name==='TimeoutError'?'Проверка не завершилась вовремя.':error.message;}
    finally{$('agent-check').disabled=false;}
  }
  async function saveAgent(){
    const found=discovery.find(d=>d.provider===setupProvider);
    let environment=$('agent-profile').value.trim();if(found&&environment===found.environment)environment='';
    $('agent-save').disabled=true;$('agent-save-status').textContent='Сохраняем…';
    try{await postAPI('/api/agent-set',{provider:setupProvider,cli:$('agent-cli').value.trim(),environment});$('agent-save-status').textContent='Агент подключён исполнителем.';viewSignature='';await refresh();}
    catch(error){$('agent-save-status').textContent=error.message;}
    finally{$('agent-save').disabled=false;}
  }
  window.messengerRefresh=()=>{viewSignature='';refresh();};
  window.messengerFolderPicked=path=>{if(typeof path==='string'&&path)$('project-folder').value=path;};
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
  $('app-update-button').addEventListener('click',()=>window.webkit?.messageHandlers?.updates?.postMessage('check'));
  root.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>show(b.dataset.tab)));
  $('list-toggle').addEventListener('click',()=>{
    const expanded=$('list-toggle').getAttribute('aria-expanded')!=='true';
    $('list-toggle').setAttribute('aria-expanded',String(expanded));
    const label=expanded?'Скрыть список задач':'Показать список задач';$('list-toggle').setAttribute('aria-label',label);$('list-toggle').title=label;
    $('task-panel').hidden=!expanded;$('task-layout').classList.toggle('am-queue-collapsed',!expanded);
  });
  $('project-select').addEventListener('change',()=>{
    const value=$('project-select').value;
    if(value==='__new'){$('project-select').value=view?.project||'__new';syncSelects();openProjectDialog(false);return;}
    project=value;remember('messenger-project',value);selectedTask='';feed=null;feedSignature='';viewSignature='';refresh();
  });
  $('agent-filter').addEventListener('change',renderTaskList);
  $('answer-text').addEventListener('input',()=>{const f=$('answer-text');f.style.height='auto';f.style.height=Math.min(140,Math.max(44,f.scrollHeight))+'px';answerControls();});
  $('answer-send').addEventListener('click',sendAnswer);
  // Enter sends; Shift+Enter adds a line. An IME composition keeps its Enter.
  $('answer-text').addEventListener('keydown',e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing&&e.keyCode!==229){e.preventDefault();sendAnswer();}});
  $('project-dialog-close').addEventListener('click',()=>$('project-dialog').close());
  $('project-save').addEventListener('click',saveProject);
  $('project-pick').addEventListener('click',()=>window.webkit?.messageHandlers?.folder?.postMessage('pick'));
  $('agent-setup-close').addEventListener('click',()=>$('agent-setup').close());
  $('agent-check').addEventListener('click',checkAgent);
  $('agent-save').addEventListener('click',saveAgent);
  let initialTheme=window.messengerInitialTheme;
  if(!initialTheme)try{initialTheme=localStorage.getItem('messenger-theme');}catch(_){}
  enhanceSelect($('theme-select'));enhanceSelect($('project-select'));enhanceSelect($('agent-filter'));
  applyTheme(initialTheme||'system');
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
