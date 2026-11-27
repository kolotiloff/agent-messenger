// Exercise the real project controls against fictional window and statistics data.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_ENGINE, SCREENSHOT_DIR.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const output = process.env.SCREENSHOT_DIR;
const assets = path.resolve(__dirname, '../internal/localui/assets');
const projects = [{name:'game',title:'Игра',folder:'/Проекты/Игра'},{name:'tools',title:'Инструменты',folder:'/Проекты/Инструменты'},{name:'site',title:'Сайт',folder:'/Проекты/Сайт'}];
const iso = minutes => new Date(Date.now() - minutes * 60000).toISOString();
function archiveDate(days,minutes=0){
  const date=new Date();
  date.setDate(date.getDate()-days);
  date.setHours(3,minutes,0,0);
  return date.toISOString();
}
const makeTask = (id, project, agent, title, minutes, extra = {}) => ({
  id,project,agent,title,state:'running',created_at:iso(minutes),started_at:iso(minutes),
  folder:projects.find(p => p.name===project).folder,worker_observed:true,
  ran_model:agent==='codex'?'gpt-6.2-sol[effort=medium]':'claude-opus-4-8',...extra
});
const pc1 = {id:'pc1',name:'ПК-1'}, pc2 = {id:'pc2',name:'ПК-2'}, linux = {id:'linux',name:'Ноут-Linux'};
const tasks = {
  game:[makeTask('game-1','game','codex','Экран настроек',9), makeTask('game-2','game','codex','Проверка навигации',15,{state:'waiting_owner',remote:pc1}), makeTask('game-4','game','codex','Параллельная задача',8), makeTask('game-3','game','codex','Звуки меню',37,{state:'done',finished_at:iso(25),remote:pc2,usage:{input_tokens:4100000,output_tokens:200000,cached_tokens:1200000}})],
  site:[makeTask('site-1','site','codex','Подготовка сайта',7)],
  tools:[makeTask('tools-1','tools','claude','Экспорт отчёта',21,{state:'waiting_owner'}),makeTask('tools-2','tools','codex','Проверка файлов',3,{remote:linux})]
};
const toolDone=makeTask('tools-done','tools','codex','Готовый отчёт',90,{state:'done',finished_at:iso(60)});
const toolOlder=makeTask('tools-old','tools','codex','Старый отчёт',180,{state:'done',created_at:archiveDate(2,-30),started_at:archiveDate(2,-30),finished_at:archiveDate(2)});
const toolShared={peer:'pc1',name:pc1.name,age:0,tasks:[{...toolDone,id:'tools-shared',title:'Отчёт другого компьютера',created_at:archiveDate(1,-30),started_at:archiveDate(1,-30),finished_at:archiveDate(1),model:'gpt-6.2-sol[effort=medium]',tokens:342000,report:'Общий результат'}]};

const counts = {game:1234,tools:2345,site:99999};
const pc1Counts = {game:321,tools:654,site:0};
const orderedCodex=['gpt-6-luna[effort=high]','gpt-6.1-terra[effort=medium]','gpt-6.2-sol[effort=medium]',
  'gpt-6.2-sol[effort=high]','gpt-6.2-astra[effort=medium]','gpt-6.2-astra[effort=high]'];
const agents = [{agent:'codex',models:[orderedCodex[2],orderedCodex[5],orderedCodex[0],orderedCodex[4],orderedCodex[3],orderedCodex[1]]},{agent:'claude',models:['claude-opus-4-8']}];
const coordinator = p=>({connected:true,active:p==='tools',client:'codex'});
function reply(url,p,available){
  if(url.pathname==='/api/window')return {project:p,projects:available,computer_name:'Мак',agents,tasks:tasks[p],shared:[],tasks_enabled:true,history_paged:true,coordinator:coordinator(p)};
  if(url.pathname==='/api/project-activity')return {project:p,tasks:tasks[p],shared:[],coordinator:coordinator(p)};
  if(url.pathname==='/api/task'){
    const task=Object.values(tasks).flat().find(t=>t.id===url.searchParams.get('id'));
    return {task,items:[
      {kind:'task',author:'coordinator',sender_agent:'codex',at:task.created_at,text:task.title},
      {kind:'text',author:task.agent,at:iso(2),text:'Сообщение из '+task.project},
    ]};
  }
  if(url.pathname==='/api/server')return {host:{addresses:[],devices:[]},member:{},peers:[
    {...pc1,ready:{projects:['game','tools']}},
    {...pc2,ready:{projects:['game']}},
    {...linux,ready:{projects:['tools','site']},agents:[{agent:'codex',paused:true},{agent:'claude'}]},
    {id:'vps',name:'VPS-PC2',host:true,ready:{projects:[]},agents:[]}
  ].map(peer=>({age:0,version:'1.0.5',agents,...peer}))};
  if(url.pathname==='/api/discovery')return {agents:[]};
  if(url.pathname==='/api/client-info')return {client:'running',outdated:false,window_version:'1.0.9'};
  if(url.pathname==='/api/self-update')return {installed_version:'1.0.9',state:'idle'};
  if(url.pathname==='/api/usage-stats'){
    const names=url.searchParams.getAll('projects'), scope=names.length?names:[p];
    const computer=url.searchParams.get('computer'), visibleCounts=computer==='pc1'?pc1Counts:counts;
    const usage=(tasks,tokens)=>({tasks,tokens,input_tokens:tokens,output_tokens:0,cached_tokens:0});
    const total=usage(scope.length,scope.reduce((sum,name)=>sum+visibleCounts[name],0));
    return {
      ...(names.length?{projects:names}:{project:p}), total,
      agents:scope.map(name=>({key:tasks[name][0].agent,...usage(1,visibleCounts[name])})),
      computers:[{key:'local',name:'Мак',this:true,...total}], models:[],
      days:[{day:iso(1440).slice(0,10),...usage(0,0)},{day:iso(1).slice(0,10),...total}],
      recent:scope.map(name=>({...tasks[name][0],...usage(1,visibleCounts[name]),at:iso(1)}))
    };
  }
  if(url.pathname==='/api/usage-efficiency'){
    const names=url.searchParams.getAll('projects');
    return {...(names.length?{projects:names}:{project:p}),groups:[],total_groups:0};
  }
  if(url.pathname==='/api/usage-export')return {ok:true,result:{path:'/fixtures/statistics.json',tasks:2}};
  return {};
}
module.exports = { projects, reply };
if (require.main === module) (async()=>{
  const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const engine=process.env.BROWSER_ENGINE||'chromium';
  const browser=await playwright[engine].launch();
  const errors=[],seen=[];
  async function open(available,viewport,colorScheme,beforeReply=()=>{},preview=false,desktop='mac'){
    const context=await browser.newContext({viewport,deviceScaleFactor:1,colorScheme,locale:'ru-RU'});
    await context.addInitScript(({scheme,desktop})=>{
      window.amDesktop=desktop==='mac-titlebar'?'mac':desktop;window.messengerInitialTheme=scheme;
      if(desktop==='windows'){window.amFrameless=true;window.amWindow=()=>{};}
      window.nativeAppMessages=[];
      window.webkit={messageHandlers:{app:{postMessage:message=>window.nativeAppMessages.push(message)}}};
      if(desktop==='mac-titlebar'){
        window.amMacTitlebar=true;
        window.webkit.messageHandlers.titlebar={postMessage:geometry=>{window.titlebarGeometry=geometry;}};
      }
    },{scheme:colorScheme,desktop});
    const page=await context.newPage();
    page.setDefaultTimeout(6500);
    page.on('pageerror',error=>errors.push(error.message));
    await page.route('**/*',async route=>{
      const req=route.request(),url=new URL(req.url());
      if(url.hostname!=='messenger-preview.test')return route.abort();
      if(url.pathname.startsWith('/api/')){
        seen.push({path:url.pathname,params:url.searchParams,body:req.postData()});
        const p=url.searchParams.get('project')||available[0].name;
        const override=await beforeReply(url,p);
        return route.fulfill(override||{json:reply(url,p,available)});
      }
      const files={'/':['index.html','text/html; charset=utf-8'],'/app.js':['app.js','application/javascript'],'/i18n.js':['i18n.js','application/javascript'],'/app.css':['app.css','text/css'],'/icon.svg':['icon.svg','image/svg+xml']};
      const file=files[url.pathname];
      if(!file)return route.fulfill({status:404,body:''});
      let body=await fs.readFile(path.join(assets,file[0]),'utf8');
      if(preview&&url.pathname==='/')body=body.replace('id="am-concept"','id="am-concept" class="am-task-sidebar-preview"');
      return route.fulfill({contentType:file[1],body});
    });
    await page.goto('https://messenger-preview.test/#preview-only');
    await page.getByText('Сообщение из game').waitFor();
    return page;
  }
  const select=async(page,id,label,keyboard=false)=>{
    const trigger=page.locator('button[aria-controls="'+id+'-menu"]');
    if(keyboard){await trigger.focus();await trigger.press('ArrowDown');}
    else await trigger.click();
    await page.locator('#'+id+'-menu').getByRole('option',{name:label,exact:true}).click();
  };
  const split=page=>page.locator('#split-projects');
  const card=(page,id)=>page.locator('#machines .am-machine-card[data-machine-order="'+id+'"]');
  const cells=(page,id)=>card(page,id).locator('.am-machine-project');
  const fills=target=>target.evaluate(tile=>[tile,tile.querySelector('.am-machine-name')]
    .map(part=>getComputedStyle(part).backgroundColor));
  const chat=(page,name)=>page.locator('#thread').getByText('Сообщение из '+name);
  const total=async(page,n)=>page.waitForFunction(value=>document.querySelector('#stats-total-title').textContent.split(' токен')[0].replace(/[^0-9]/g,'')===value,n);
  try{
    const hold=()=>{let release;const response=new Promise(resolve=>{release=resolve;});return {response,release};};
    const sameFeed=hold(),otherWindow=hold(),otherFeed=hold();
    let ownerQuestion,remoteLost=false;
    const preview=await open(projects,{width:1100,height:706},'dark',(url,p)=>{
      if(url.pathname==='/api/machine-order')return {json:{ok:true}};
      if(url.pathname==='/api/task-answer')return {json:{ok:true}};
      if(p==='tools'&&url.pathname==='/api/window'&&ownerQuestion){
        const window=reply(url,p,projects);
        return {json:{...window,tasks:window.tasks.map(task=>{
          if(task.id==='tools-1')return {...task,owner_question:ownerQuestion};
          if(task.id==='tools-2')return {...task,remote:{...task.remote,lost:remoteLost}};
          return task;
        })}};
      }
      if(url.pathname==='/api/window'&&p==='tools')return otherWindow.response;
      if(url.pathname!=='/api/task')return;
      if(url.searchParams.get('id')==='game-4'){
        const initial=reply(url,p,projects);
        initial.items.push({kind:'tool',title:'Bash',description:'Проверить изменения в макете',text:'git diff --stat',at:initial.items.at(-1).at});
        initial.items.push({kind:'report',author:'worker',verified:true,text:'Проверка завершена',at:initial.items.at(-1).at});
        return {json:{...initial,task:{...initial.task,quiet:{live_tokens:342000},context_window:{used_tokens:212000,capacity_tokens:238000}}}};
      }
      if(url.searchParams.get('id')==='game-1')return sameFeed.response;
      if(url.searchParams.get('id')==='tools-1')return otherFeed.response;
    },true);
    const thread=preview.locator('#thread'),chatDate=preview.locator('#conversation-date');
    const oldDate=await chatDate.getAttribute('datetime');
    const sameDate=tasks.game.find(t=>t.id==='game-1').created_at;
    const otherDate=tasks.tools.find(t=>t.id==='tools-1').created_at;
    assert.equal(await preview.locator('#task-head > h3').innerText(),'Параллельная задача');
    assert.deepEqual(await preview.locator('#task-head > *').evaluateAll(nodes=>nodes.map(node=>node.tagName)),['H3']);
    assert.equal(await chatDate.evaluate(node=>getComputedStyle(node).alignSelf),'center');
    assert.equal(await thread.locator('.am-outgoing .am-message-content').first().evaluate(node=>getComputedStyle(node).padding),'12px');
    const reportStatus=thread.locator('.am-delivery-close');
    await reportStatus.waitFor();
    assert.equal(await reportStatus.locator('time').count(),0);
    assert.match(await reportStatus.getAttribute('title'),/\d{2}:\d{2}/);
    const standalone=thread.locator('.am-message-content > .am-tool');
    assert.deepEqual(await standalone.evaluate(node=>{
      const style=getComputedStyle(node),label=node.querySelector('.am-action-label');
      const icon=label.querySelector('svg').getBoundingClientRect(),text=label.querySelector('.am-tool-name');
      const firstLineCenter=text.getBoundingClientRect().top+parseFloat(getComputedStyle(text).lineHeight)/2;
      return [style.marginBlockStart,style.marginBlockEnd,icon.top+icon.height/2-firstLineCenter];
    }),['16px','16px',0]);
    const closingGap=await reportStatus.evaluate(n=>n.getBoundingClientRect().top-n.previousElementSibling.getBoundingClientRect().bottom);
    assert.equal(closingGap,await standalone.evaluate(n=>parseFloat(getComputedStyle(n).marginBlockStart)));
    await standalone.locator('summary').click();
    assert.equal(await standalone.locator('pre').textContent(),'git diff --stat');
    assert.equal(await standalone.locator('pre .am-shell-name').textContent(),'git');
    assert.equal(await standalone.locator('pre .am-shell-literal').textContent(),'--stat');
    await standalone.locator('summary').click();
    const tokenCount=preview.locator('#answer-tokens');
    const answerField=preview.locator('#compose .am-compose-field');
    const composerBottom=()=>answerField.evaluate(field=>field.getBoundingClientRect().bottom);
    await tokenCount.waitFor();
    assert((await tokenCount.boundingBox()).y>(await answerField.boundingBox()).y);
    const contextProgress=tokenCount.locator('#answer-context-progress');
    await contextProgress.waitFor();
    const [drawn,track]=(await contextProgress.getAttribute('stroke-dasharray')).split(' ').map(Number);
    assert.equal(track,100);
    assert(Math.abs(drawn-212/238*100)<0.01);
    await tokenCount.click();
    const tokenMenu=preview.locator('#answer-token-menu');
    await tokenMenu.waitFor();
    assert.deepEqual(await tokenMenu.locator('p:visible').allTextContents(),[
      'Контекстное окно 89\u00a0% заполнено',
      '212 тыс. / 238 тыс. токенов'
    ]);
    await preview.keyboard.press('Escape');
    await tokenMenu.waitFor({state:'hidden'});
    assert.equal(await tokenCount.evaluate(node=>node===document.activeElement),true);
    assert.match(await preview.locator('#answer-model').textContent(),/Sol 6\.2 Medium/);
    const initialComposerBottom=await composerBottom();
    const localHeader=card(preview,'local').locator('.am-sidebar-disclosure[draggable=true]');
    const peerHeader=card(preview,'pc1').locator('.am-sidebar-disclosure[draggable=true]');
    const order=()=>preview.locator('#machines > .am-machine-card').evaluateAll(cards=>cards.map(card=>card.dataset.machineOrder));
    const padding=target=>target.evaluate(node=>[getComputedStyle(node).paddingBlockStart,getComputedStyle(node).paddingBlockEnd]);
    const sidebarTask=title=>preview.locator('#machines .am-sidebar-task').filter({hasText:title});
    const expectPrevious=async(date)=>{
      assert.equal(await chatDate.getAttribute('datetime'),date);
      assert.equal(await chat(preview,'game').isVisible(),true);
      assert.equal(await thread.locator('[data-pane]').count(),0);
      assert.equal(await thread.getAttribute('aria-busy'),'true');
      assert.equal(await preview.locator('#compose').evaluate(node=>node.inert),true);
    };
    try{
      await preview.evaluate(()=>{
        document.addEventListener('dragover',()=>{
          const target=document.querySelector('[data-drop-edge]');
          if(target){
            const line=getComputedStyle(target,'::after');
            window.machineDropLine={edge:target.dataset.dropEdge,height:line.height,color:line.backgroundColor};
          }
        });
        const setDragImage=DataTransfer.prototype.setDragImage;
        DataTransfer.prototype.setDragImage=function(node,x,y){
          const canvas=document.createElement('canvas').getContext('2d');
          canvas.fillStyle=getComputedStyle(node).backgroundColor;
          canvas.fillRect(0,0,1,1);
          window.machineDragImage={
            head:node.matches('.am-machine-name.am-machine-drag-image'),
            name:node.querySelector('.am-machine-label')?.textContent,
            icon:!!node.querySelector('.am-machine-presence svg'),
            version:node.querySelector('.am-machine-version')?.textContent,
            alpha:canvas.getImageData(0,0,1,1).data[3],
          };
          return setDragImage.call(this,node,x,y);
        };
      });
      const requestOrder=preview.waitForRequest(req=>req.method()==='POST'&&new URL(req.url()).pathname==='/api/machine-order');
      await peerHeader.dragTo(localHeader,{sourcePosition:{x:40,y:12},targetPosition:{x:40,y:4}});
      assert.deepEqual(await preview.evaluate(()=>window.machineDragImage),{head:true,name:'ПК-1',icon:true,version:'v1.0.5',alpha:102});
      assert.deepEqual(await preview.evaluate(()=>window.machineDropLine),{edge:'before',height:'2px',color:'rgb(103, 170, 249)'});
      assert.equal(await preview.locator('.am-machine-drag-image').count(),0);
      assert.deepEqual((await requestOrder).postDataJSON().order.slice(0,2),['pc1','local']);
      assert.deepEqual((await order()).slice(0,2),['pc1','local']);
      await peerHeader.press('Alt+ArrowDown');
      assert.deepEqual((await order()).slice(0,2),['local','pc1']);
      assert.equal(await peerHeader.evaluate(node=>node===document.activeElement),true);
      assert.deepEqual(await padding(card(preview,'pc1').locator('.am-machine-name')),['4px','4px']);
      assert.deepEqual(await padding(sidebarTask('Проверка навигации')),['4px','4px']);
      await sidebarTask('Экран настроек').click();
      await expectPrevious(oldDate);
      assert.equal(await composerBottom(),initialComposerBottom);
      sameFeed.release();
      await preview.locator('#answer-tokens[data-task="game-1"]').waitFor();
      await contextProgress.waitFor({state:'hidden'});
      await tokenCount.click();
      assert.equal(await tokenMenu.locator('#answer-context-summary').textContent(),'Контекстное окно нет данных');
      await preview.keyboard.press('Escape');
      assert.equal(await composerBottom(),initialComposerBottom);
      await preview.locator('#conversation-date[datetime="'+sameDate+'"]').waitFor();
      await sidebarTask('Экспорт отчёта').click();
      await expectPrevious(sameDate);
      const requestedFeed=preview.waitForRequest(req=>new URL(req.url()).searchParams.get('id')==='tools-1');
      otherWindow.release();
      await requestedFeed;
      await expectPrevious(sameDate);
      otherFeed.release();
      await chat(preview,'tools').waitFor();
      assert.equal(await chatDate.getAttribute('datetime'),otherDate);
      assert.equal(await thread.getAttribute('aria-busy'),'false');
      assert.equal(await preview.locator('#compose').evaluate(node=>node.inert),false);
      const answer=preview.locator('#answer-text'),send=preview.locator('#answer-send');
      assert.deepEqual(await preview.locator('#thread .am-meta .am-provider-icon svg').evaluateAll(icons=>icons.map(icon=>{
        const {width,height}=icon.getBoundingClientRect();
        return [width,height];
      })),[[16,16],[16,16]]);
      for(const width of [1100,1800]){
        await preview.setViewportSize({width,height:770});
        await preview.waitForFunction(()=>{
          const feed=document.getElementById('thread');
          const footer=document.querySelector('.am-chat-footer');
          const pane=feed.getBoundingClientRect(),field=footer.getBoundingClientRect();
          const style=getComputedStyle(feed);
          return Math.abs(field.left-pane.left-parseFloat(style.paddingLeft))<1
            && Math.abs(field.right-pane.right+parseFloat(style.paddingRight)+feed.offsetWidth-feed.clientWidth)<1
            && Math.abs(field.width-Math.min(760,pane.width-48))<1
            && Math.abs(field.left-pane.left-(pane.right-field.right))<1;
        });
      }
      await answer.fill('Оставь оба формата');
      assert.equal(await send.isDisabled(),true);
      ownerQuestion={id:'question-1',text:'Какой формат оставить?'};
      await preview.waitForFunction(()=>!document.getElementById('answer-send').disabled);
      assert.equal(await answer.inputValue(),'Оставь оба формата');
      const submitted=preview.waitForRequest(req=>new URL(req.url()).pathname==='/api/task-answer');
      await send.click();
      const {task,question,text}=(await submitted).postDataJSON();
      assert.deepEqual([task,question,text],['tools-1','question-1','Оставь оба формата']);
      await preview.waitForFunction(()=>document.getElementById('answer-text').value==='');
      await sidebarTask('Проверка файлов').click();
      await preview.locator('#compose[data-task="tools-2"]').waitFor();
      await answer.fill('Черновик для удалённого компьютера');
      assert.equal(await send.isEnabled(),true);
      remoteLost=true;
      await preview.locator('#lost-bar').waitFor();
      assert.equal(await send.isDisabled(),true);
      await answer.fill('Изменённый черновик без связи');
      remoteLost=false;
      await preview.locator('#lost-bar').waitFor({state:'hidden'});
      assert.equal(await send.isEnabled(),true);
      assert.equal(await answer.inputValue(),'Изменённый черновик без связи');
    }finally{sameFeed.release();otherWindow.release();otherFeed.release();await preview.close();}

    const directTask=makeTask('owner-task','tools','codex','Новая задача в окне',0,{remote:pc1});
    const directReply='Настройки сохранены\n\nПроверен **размер окна**.';
    const verticalCenter=locator=>locator.evaluate(node=>{
      const {top,height}=node.getBoundingClientRect();
      return top+height/2;
    });
    const expectCentered=async(text,control)=>{
      assert(Math.abs(await verticalCenter(text)-await verticalCenter(control))<1);
    };
    const queued=makeTask('queued-task','tools','codex','Следующая проверка',1,{started_at:null,pending_launch:true,worker_observed:false,remote:pc1});
    const justStarted=new Date().toISOString();
    const draftPage=await open(projects,{width:1150,height:770},'dark',(url,p)=>{
      if(p==='game'&&url.pathname==='/api/window'){
        const window=reply(url,p,projects);
        return {json:{...window,history_paged:true,history_next:!url.searchParams.get('agent')?'game-page':'',tasks:window.tasks.map(t=>({...t,full_access:!t.remote,started_at:t.id==='game-1'?justStarted:t.started_at}))}};
      }
      if(p==='tools'&&['/api/window','/api/project-activity'].includes(url.pathname)){
        const showCodex=!url.searchParams.get('agent')||url.searchParams.get('agent')==='codex';
        return {json:{...reply(url,p,projects),tasks:[...tasks.tools,queued,...(showCodex?[toolDone]:[])],shared:[toolShared],history_paged:true,history_next:showCodex?toolDone.id:''}};
      }
      if(p==='game'&&url.pathname==='/api/task-history')return {json:{tasks:[]}};
      if(p==='tools'&&url.pathname==='/api/task-history')return {json:{tasks:[toolOlder]}};
      if(url.pathname==='/api/self-update')return {json:{installed_version:'1.0.9',version:'1.0.10',state:'available'}};
      if(url.pathname==='/api/task-start')return {json:{ok:true,result:directTask.id}};
      if(url.pathname==='/api/task-note')return {json:{ok:true}};
      if(url.pathname==='/api/pick')return {json:{ok:true,path:'/fictional/project/notes.txt'}};
      if(url.pathname==='/api/task'&&url.searchParams.get('id')===directTask.id){
        const at=directTask.created_at;
        return {json:{task:directTask,items:[{kind:'task',author:'owner',text:directTask.title,at},
          {kind:'text',run:1,text:'Проверяю настройки',at},{kind:'text',run:1,text:directReply,at},
          {kind:'report',run:1,verified:true,text:directReply,at},
          {kind:'answer',run:2,author:'owner',text:'А сейчас?',at},{kind:'text',run:2,text:'Ищу последнюю запись',at}]}};
      }
    },true);
    try{
      const plus=draftPage.locator('#task-new');
      await plus.waitFor();
      assert.equal(await plus.evaluate(node=>getComputedStyle(node).opacity),'1');
      const local=draftPage.locator('.am-machine-card[aria-current=true]');
      await draftPage.locator('#task-stop').waitFor();
      const composerStyle=field=>field.evaluate(node=>{
        const style=getComputedStyle(node),input=getComputedStyle(node.querySelector('textarea'));
        const icons=[node.firstElementChild,node.lastElementChild].map(button=>{
          const style=getComputedStyle(button);
          return [style.width,style.height,style.borderRadius,button.querySelector('svg').innerHTML];
        });
        return {padding:style.padding,radius:style.borderRadius,gap:style.gap,inputPadding:input.padding,icons};
      });
      const replyField=draftPage.locator('#compose .am-compose-field');
      const replyStyle=await composerStyle(replyField);
      const labelStyle=label=>label.evaluate(node=>{
        const style=getComputedStyle(node);
        return [style.fontFamily,style.fontSize,style.fontWeight,style.lineHeight];
      });
      const replyModelStyle=await labelStyle(draftPage.locator('#answer-model'));
      const replyAccess=draftPage.locator('#answer-access');
      await replyAccess.getByText('Любые команды',{exact:true}).waitFor();
      assert.equal(await local.locator('.am-sidebar-task').filter({hasText:'Экран настроек'}).locator('[data-elapsed-from]').textContent(),', 1 мин');
      await expectCentered(replyAccess.locator('svg'),replyAccess.locator('span'));
      for(const theme of ['light','dark']){
        const colors=await replyAccess.evaluate((node,theme)=>{
          const root=document.documentElement,original=root.dataset.theme;
          root.dataset.theme=theme;
          try{
            const ctx=document.createElement('canvas').getContext('2d');
            const rgb=value=>{ctx.fillStyle=value;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data].slice(0,3);};
            const style=getComputedStyle(node),text=rgb(style.color);
            return {text,stroke:rgb(getComputedStyle(node.querySelector('svg')).stroke),bg:rgb(getComputedStyle(node.closest('.am-window')).backgroundColor)};
          }finally{root.dataset.theme=original;}
        },theme);
        assert.deepEqual(colors.stroke,colors.text);
        assert(colors.text[0]>colors.text[1]+35&&colors.text[0]>colors.text[2]+35,'muted red for full access');
        const luminance=rgb=>rgb.map(channel=>channel/255).map(c=>c<=.04045?c/12.92:((c+.055)/1.055)**2.4)
          .reduce((sum,c,i)=>sum+c*[.2126,.7152,.0722][i],0);
        const [dark,light]=[luminance(colors.text),luminance(colors.bg)].sort((a,b)=>a-b);
        assert((light+.05)/(dark+.05)>=4.5,'readable access label in '+theme);
      }
      const accessAbove=access=>access.evaluate(node=>{
        const rect=node.getBoundingClientRect();
        const field=node.nextElementSibling.getBoundingClientRect();
        return rect.height>0&&rect.bottom<field.top&&rect.left===field.left&&rect.width<field.width;
      });
      assert.equal(await accessAbove(replyAccess),true);
      assert.deepEqual(await draftPage.locator('#compose .am-compose-field').evaluate(field=>{
        const stop=field.querySelector('#task-stop').getBoundingClientRect();
        const send=field.querySelector('#answer-send').getBoundingClientRect();
        const icons=Array.from(field.querySelectorAll('#task-stop > svg,#answer-send > svg'),svg=>svg.getBoundingClientRect().width);
        const box=field.getBoundingClientRect();
        return [stop.width,stop.height,send.width,send.height,send.left-stop.right,box.right-send.right,...icons];
      }),[32,32,32,32,4,5,16,16]);
      const disclosure=local.locator('.am-sidebar-disclosure');
      assert.equal(await disclosure.locator('svg').count(),0);
      assert.deepEqual(await disclosure.boundingBox(),await local.locator('.am-machine-name').boundingBox());
      const selected=local.locator('.am-sidebar-task[aria-pressed=true]');
      const groupGap=()=>local.evaluate(node=>node.nextElementSibling.getBoundingClientRect().top-node.getBoundingClientRect().bottom);
      const selectedFill=await selected.evaluate(node=>getComputedStyle(node).backgroundColor);
      assert.equal(await selected.evaluate(node=>getComputedStyle(node).transitionDuration),'0s');
      assert.equal(await groupGap(),4);
      await disclosure.click();
      assert.equal(await disclosure.getAttribute('aria-expanded'),'false');
      assert.equal(await local.locator('.am-sidebar-task-count').textContent(),' (4 задачи)');
      assert.equal(await local.locator('.am-sidebar-task-count').evaluate(n=>getComputedStyle(n).fontWeight),'400');
      assert.match(await disclosure.getAttribute('aria-label'),/4 задачи/);
      assert.equal(await groupGap(),4);
      await disclosure.click();
      assert.equal(await disclosure.getAttribute('aria-expanded'),'true');
      assert.equal(await local.locator('.am-sidebar-task-count').count(),0);
      assert.equal(await selected.evaluate(node=>getComputedStyle(node).backgroundColor),selectedFill);
      assert.equal(await groupGap(),4);
      await draftPage.mouse.move(1000,30);
      const nav=draftPage.locator('.am-tabs');
      assert.equal(await draftPage.locator('.am-sidebar > #task-new').count(),1);
      assert.equal(await draftPage.getByRole('button',{name:'Новая задача',exact:true}).count(),1);
      const picker=await draftPage.locator('#project-column-first .am-select-trigger').boundingBox(),plusBox=await plus.boundingBox();
      const projectLabel=await draftPage.locator('#project-heading').boundingBox();
      assert.equal(plusBox.x,projectLabel.x);
      const newTaskInset=await plus.evaluate(n=>getComputedStyle(n).paddingInlineStart);
      const rowInset=await local.locator('.am-machine-name').evaluate(n=>getComputedStyle(n).paddingInlineStart);
      assert.equal(newTaskInset,rowInset);
      assert.equal((await plus.locator('svg').boundingBox()).x-plusBox.x,parseFloat(rowInset));
      await plus.hover();
      assert.deepEqual(await plus.boundingBox(),plusBox);
      const newTaskHover=await plus.evaluate(async n=>{
        await Promise.all(n.getAnimations().map(a=>a.finished));
        return getComputedStyle(n).backgroundColor;
      });
      assert(plusBox.y+plusBox.height<picker.y);
      assert.equal(await plus.innerText(),'Новая задача');
      const plusGlyph=plus.locator('svg');
      assert.deepEqual(await plusGlyph.evaluate(icon=>{
        const style=getComputedStyle(icon.querySelector('path'));
        return [getComputedStyle(icon).width,style.stroke,style.strokeWidth];
      }),['18px',await plus.evaluate(button=>getComputedStyle(button).color),'2px']);
      assert.equal(await nav.evaluate(node=>getComputedStyle(node).display),'contents');
      assert.equal(await nav.locator('[data-tab]:visible').count(),0);
      const footer=draftPage.locator('.am-side-footer');
      await draftPage.locator('.am-sidebar').evaluate(sidebar=>{
        const {left,right}=sidebar.getBoundingClientRect();
        for(const selector of ['.am-computers','.am-side-bottom']){
          const divider=sidebar.querySelector(selector);
          const rect=divider.getBoundingClientRect(),style=getComputedStyle(divider);
          if(rect.left!==left||rect.right!==right-parseFloat(getComputedStyle(sidebar).borderRightWidth)||style.borderTopWidth!=='1px'){
            throw Error('Sidebar divider does not span the panel: '+selector);
          }
        }
      });
      await footer.getByText('v1.0.9',{exact:true}).waitFor();
      const localUpdate=footer.getByRole('button',{name:'Обновить до v1.0.10'});
      await localUpdate.waitFor();
      const downloadBox=await localUpdate.boundingBox(),versionBox=await footer.locator('.am-version').boundingBox();
      assert.equal(versionBox.x-downloadBox.x-downloadBox.width,4);
      assert.deepEqual(await footer.evaluate(node=>{
        const sidebar=node.closest('.am-sidebar'),bounds=sidebar.getBoundingClientRect();
        const icon=node.querySelector('#settings-open svg').getBoundingClientRect();
        const version=node.querySelector('.am-version'),box=version.getBoundingClientRect();
        return [icon.left-bounds.left,bounds.right-parseFloat(getComputedStyle(sidebar).borderRightWidth)-
          box.right+parseFloat(getComputedStyle(version).paddingInlineEnd)];
      }),[17,17]);
      const buttonShape=button=>button.evaluate(node=>{
        const style=getComputedStyle(node);
        return [style.width,style.height,style.borderRadius];
      });
      const settings=footer.locator('#settings-open'),feedback=footer.locator('#fb-open');
      const footerIcons=[settings,feedback,localUpdate];
      const hoverFills=[];
      for(const button of footerIcons){
        assert.deepEqual(await buttonShape(button),['28px','28px','8px']);
        const art=await button.locator('svg').first().boundingBox();
        assert.deepEqual([art.width,art.height],[18,18]);
        await button.hover();
        hoverFills.push(await button.evaluate(async node=>{
          await Promise.all(node.getAnimations().map(animation=>animation.finished));
          return getComputedStyle(node).backgroundColor;
        }));
      }
      assert.deepEqual(hoverFills,[hoverFills[0],hoverFills[0],hoverFills[0]]);
      await local.getByText('Мак, координатор',{exact:true}).waitFor();
      await card(draftPage,'vps').getByText('VPS-PC2, сервер',{exact:true}).waitFor();
      const roleWeights=await draftPage.locator('.am-machine-label .am-inline-role').evaluateAll(roles=>roles.map(role=>[getComputedStyle(role).fontWeight,getComputedStyle(role.parentElement).fontWeight]));
      assert(roleWeights.length>=2 && roleWeights.every(([role,name])=>role==='400'&&Number(name)>400),JSON.stringify(roleWeights));
      const remote=card(draftPage,'pc1').locator('.am-machine-name');
      const metadata=remote.locator('.am-machine-meta');
      const opacity=()=>metadata.evaluate(node=>getComputedStyle(node).opacity);
      const versionSize=node=>getComputedStyle(node).fontSize;
      assert.equal(await footer.locator('.am-version').evaluate(versionSize),await metadata.locator('.am-version').evaluate(versionSize));
      assert.equal(await opacity(),'0');
      assert(await remote.evaluate(node=>getComputedStyle(node).backgroundColor===getComputedStyle(node.closest('.am-sidebar')).backgroundColor));
      const download=remote.getByRole('button',{name:'Обновить до v1.0.10'});
      const initialButton=await download.boundingBox();
      const directPoint={x:initialButton.x+initialButton.width/2,y:initialButton.y+initialButton.height/2};
      await draftPage.mouse.move(directPoint.x,directPoint.y);
      assert.equal(await opacity(),'1','direct entry over the download button reveals it');
      assert.deepEqual(await download.boundingBox(),initialButton,'revealing the controls must keep their hit targets in place');
      assert(await download.evaluate((node,p)=>node.contains(document.elementFromPoint(p.x,p.y)),directPoint),'the button stays under the pointer');
      await draftPage.locator('#settings-open').hover();
      assert.equal(await opacity(),'0','leaving the download target hides its metadata without selecting the computer');
      await remote.hover();
      assert.equal(await opacity(),'1');
      assert.notEqual(await remote.evaluate(node=>getComputedStyle(node).backgroundColor),await remote.evaluate(node=>getComputedStyle(node.closest('.am-sidebar')).backgroundColor));
      const machineHover=await remote.evaluate(n=>getComputedStyle(n).backgroundColor);
      assert.equal(machineHover,newTaskHover);
      const inactiveTask=local.locator('.am-sidebar-task:not([aria-pressed=true])').first();
      await inactiveTask.hover();
      assert.equal(await inactiveTask.evaluate(n=>getComputedStyle(n).backgroundColor),machineHover);
      await remote.locator('.am-sidebar-disclosure').click();
      await draftPage.locator('#settings-open').hover();
      assert.equal(await opacity(),'0');
      await draftPage.keyboard.press('Tab');
      const remoteUpdate=remote.getByRole('button',{name:'Обновить до v1.0.10'});
      await remoteUpdate.focus();
      assert.equal(await opacity(),'1');
      assert.deepEqual(await buttonShape(remoteUpdate),['28px','28px','8px']);
      assert.equal(await metadata.evaluate(node=>node.querySelector('.am-version').getBoundingClientRect().left
        - node.querySelector('.am-update-icon').getBoundingClientRect().right),4);
      const remoteGlyph=await remoteUpdate.locator('svg').first().boundingBox();
      assert.deepEqual([remoteGlyph.width,remoteGlyph.height],[18,18]);
      assert.equal(await remote.evaluate(node=>node.getBoundingClientRect().height),29);
      await remoteUpdate.hover();
      const remoteFill=await remoteUpdate.evaluate(async node=>{
        await Promise.all(node.getAnimations().map(animation=>animation.finished));
        return getComputedStyle(node).backgroundColor;
      });
      assert.equal(remoteFill,hoverFills[0]);
      assert.match(await draftPage.locator('#thread .am-meta-author').last().textContent(),/, исполнитель$/);
      const authorWeights=await draftPage.locator('#thread .am-meta-author .am-inline-role').evaluateAll(roles=>roles.map(role=>[getComputedStyle(role).fontWeight,getComputedStyle(role.parentElement).fontWeight]));
      assert(authorWeights.length&&authorWeights.every(([role,author])=>role==='400'&&Number(author)>400),JSON.stringify(authorWeights));
      assert.equal(await draftPage.locator('#thread .am-role').count(),0);
      for(const panel of [local.locator('.am-machine-label'),...await draftPage.locator('#thread .am-meta-author').all()]){
        const [base,role]=await panel.evaluate(node=>{
          const typography=element=>{
            const style=getComputedStyle(element);
            return [style.fontFamily,style.fontSize,style.fontWeight,style.color];
          };
          return [typography(node),typography(node.querySelector('.am-inline-role'))];
        });
        assert.deepEqual(base.slice(0,2),role.slice(0,2));
        assert.equal(role[2],'400');
        assert(Number(base[2])>Number(role[2]));
        assert.notEqual(base[3],role[3]);
      }
      const message=draftPage.locator('#thread .am-message-group').first();
      const timestamp=message.locator('time.am-message-time');
      assert.deepEqual(await timestamp.evaluate(node=>[getComputedStyle(node.parentElement).opacity,getComputedStyle(node).fontSize]),['0','12px']);
      await message.hover();
      assert(await timestamp.evaluate(node=>getComputedStyle(node.parentElement).opacity==='1'&&
        node.getBoundingClientRect().top>node.closest('article').getBoundingClientRect().bottom));
      const workerMessage=draftPage.locator('#thread .am-message-group').last();
      await workerMessage.hover();
      const workerActions=workerMessage.locator('.am-message-actions');
      assert.equal(await workerActions.locator('time').count(),1);
      assert(await workerActions.evaluate(node=>getComputedStyle(node).opacity==='1'&&node.getBoundingClientRect().left===node.closest('article').getBoundingClientRect().left));
      assert.equal(await workerMessage.evaluate(node=>node.getBoundingClientRect().top-node.previousElementSibling.getBoundingClientRect().bottom),24);
      const settingsDialog=draftPage.getByRole('dialog',{name:'Настройки программы'});
      const settingsOpen=draftPage.locator('#settings-open');
      const settingsNav=settingsDialog.getByRole('navigation',{name:'Разделы'});
      await settingsOpen.click();
      const general=settingsDialog.locator('#settings-page');
      await general.waitFor();
      assert.deepEqual(await general.evaluate(screen=>{
        const card=screen.querySelector('.am-card');
        return [getComputedStyle(screen).paddingTop,getComputedStyle(screen).paddingLeft,
          getComputedStyle(card).paddingTop,getComputedStyle(card).paddingLeft];
      }),['16px','16px','16px','16px']);
      for(const row of await general.locator('.am-theme-setting:has(.am-select-trigger)').all()){
        await expectCentered(row.locator('label'),row.locator('.am-select-trigger'));
      }
      const settingsControlWidth=(await general.locator('.am-select-trigger').first().boundingBox()).width;
      const topLeft=async locator=>{
        const {x,y}=await locator.boundingBox();
        return [x,y];
      };
      const generalTitle=await topLeft(general.getByRole('heading',{name:'Общие',exact:true}));
      const generalField=await topLeft(general.locator('.am-select-trigger').first());
      assert.equal(await general.locator('.am-card h3').count(),0);
      const headerHeight=selector=>draftPage.locator(selector).evaluate(node=>node.getBoundingClientRect().height);
      const dialogHeaderHeight=await headerHeight('#settings-dialog .am-dialog-header');
      assert.equal(dialogHeaderHeight,44);
      assert.equal(await settingsDialog.evaluate(node=>node.matches(':modal')),true);
      assert.equal(await settingsDialog.locator('.am-sidebar,#project-select').count(),0);
      assert.equal(await draftPage.locator('[data-screen=tasks]').isVisible(),true);
      assert((await settingsDialog.boundingBox()).width>600);
      for(const section of ['server','project','agents']){
        await settingsNav.locator('[data-tab='+section+']').click();
        const screen=settingsDialog.locator('[data-screen='+section+']');
        await screen.waitFor();
        assert.deepEqual(await screen.evaluate(node=>{
          const style=getComputedStyle(node);
          return [style.paddingBlockStart,style.paddingInlineStart];
        }),['16px','16px'],section);
        const height=selector=>screen.locator(selector).evaluate(node=>node.getBoundingClientRect().height);
        const firstHeading={server:'#computer-title',project:'#project-page-title',agents:'#agents-settings-title'}[section];
        const firstControl={server:'#computer-name',project:'#project-page-name',agents:'#agents-computer + .am-select-trigger'}[section];
        assert.deepEqual(await topLeft(screen.locator(firstHeading)),generalTitle,section);
        assert(await screen.locator(firstHeading).evaluate(node=>!node.closest('.am-card')),section);
        assert.deepEqual(await topLeft(screen.locator(firstControl)),generalField,section);
        if(section==='server'){
          for(const selector of ['#computer-name','#host-scope + .am-select-trigger','#host-port','#host-enable']){
            assert.equal(await height(selector),28,selector);
          }
          const name=screen.getByRole('textbox',{name:'Имя в сети',exact:true});
          const row=screen.locator('label[for=computer-name]');
          const field=await name.boundingBox();
          assert.equal(field.width,settingsControlWidth);
          const rowBox=await row.boundingBox();
          assert.equal(field.x+field.width,rowBox.x+rowBox.width);
          await expectCentered(row.locator('span'),name);
        }
        if(section==='project'){
          for(const selector of ['#project-page-name','#project-page-folder','#project-page-pick']){
            assert.equal(await height(selector),28,selector);
          }
          await expectCentered(screen.locator('[for=project-page-name]'),screen.locator('#project-page-name'));
          const rows=screen.locator('#coord-box .am-step:has(> button)');
          assert.equal(await rows.count(),3);
          for(const row of await rows.all()){
            await expectCentered(row.locator('.am-step-text'),row.locator(':scope > button'));
          }
          assert.equal(await screen.locator('#coord-box summary').evaluate(node=>getComputedStyle(node,'::after').content),'""');
        }
      }
      await draftPage.keyboard.press('Escape');
      await settingsDialog.waitFor({state:'hidden'});
      await draftPage.waitForFunction(()=>document.activeElement?.id==='settings-open');
      await feedback.click();
      await draftPage.locator('#fb-dialog').waitFor();
      assert.equal(await headerHeight('#fb-dialog .am-dialog-header'),dialogHeaderHeight);
      await draftPage.locator('#fb-close').click();
      const statisticsOpen=draftPage.getByRole('button',{name:'Статистика',exact:true});
      const standaloneStats=draftPage.getByRole('dialog',{name:'Статистика',exact:true});
      await statisticsOpen.click();
      await standaloneStats.waitFor();
      assert.equal(await headerHeight('#stats-dialog .am-dialog-header'),dialogHeaderHeight);
      assert(await standaloneStats.evaluate(node=>node.matches(':modal')));
      assert(await standaloneStats.evaluate(node=>{
        const {width,height}=node.getBoundingClientRect();
        return width<=Math.min(900,innerWidth*.84)+1&&height<=Math.min(640,innerHeight*.8)+1;
      }));
      assert.deepEqual(await standaloneStats.locator('[data-screen=stats]').evaluate(node=>{
        const style=getComputedStyle(node);
        return [style.paddingBlockStart,style.paddingInlineStart];
      }),['16px','16px']);
      await total(draftPage,'103578');
      const toolbar=standaloneStats.locator('.am-stats-head');
      const reportTitle=selector=>standaloneStats.locator(selector).evaluate(node=>{
        const style=getComputedStyle(node);
        return [node.getBoundingClientRect().top,style.fontSize,style.fontWeight,style.lineHeight];
      });
      const toolbarPosition=await toolbar.boundingBox();
      const projectPicker=draftPage.locator('[aria-controls="project-select-menu"]');
      const controlStyle=node=>node.evaluate(item=>{
        const css=getComputedStyle(item);
        return [css.height,css.borderRadius,css.backgroundColor];
      });
      assert.equal(await projectPicker.count(),1);
      const sidebarControl=await controlStyle(projectPicker);
      for(const group of ['.am-stats-periods','.am-stats-filters']){
        const items=standaloneStats.locator(group+' button:visible');
        const first=await items.nth(0).boundingBox(),second=await items.nth(1).boundingBox();
        assert.equal(second.x-first.x-first.width,8);
      }
      for(const selector of ['#stats-project','#stats-computer','#stats-agent','#stats-model']){
        const control=standaloneStats.locator('[aria-controls="'+selector.slice(1)+'-menu"]');
        assert.deepEqual(await controlStyle(control),sidebarControl);
      }
      assert.equal(await standaloneStats.locator('#stats-efficiency').evaluate(node=>node.getBoundingClientRect().height),28);
      const usageTitle=await reportTitle('#stats-total-title');
      const efficiency=standaloneStats.locator('#stats-efficiency');
      await efficiency.click();
      await standaloneStats.locator('#stats-efficiency-status').getByText('В этом периоде нет исходных задач для сравнения.').waitFor();
      await standaloneStats.locator('#stats-usage').waitFor({state:'hidden'});
      assert.deepEqual(await reportTitle('#stats-efficiency-title'),usageTitle);
      assert.deepEqual(await toolbar.boundingBox(),toolbarPosition);
      const otherPeriod=standaloneStats.locator('[data-period][aria-pressed=false]').first();
      const period=await otherPeriod.getAttribute('data-period');
      await Promise.all([
        draftPage.waitForResponse(response=>response.url().includes('/api/usage-efficiency?')&&new URL(response.url()).searchParams.get('period')===period),
        otherPeriod.click()
      ]);
      assert.equal(await efficiency.getAttribute('aria-expanded'),'true');
      assert.deepEqual(await toolbar.boundingBox(),toolbarPosition);
      await efficiency.click();
      await standaloneStats.locator('#stats-usage').waitFor();
      assert.deepEqual(await reportTitle('#stats-total-title'),usageTitle);
      await select(draftPage,'stats-project','Инструменты');
      await select(draftPage,'stats-computer','ПК-1');
      await total(draftPage,'654');
      await draftPage.keyboard.press('Escape');
      await standaloneStats.waitFor({state:'hidden'});
      await draftPage.waitForFunction(()=>document.activeElement?.id==='stats-open');
      await select(draftPage,'project-select','Игра');
      await statisticsOpen.click();
      assert.equal(await draftPage.locator('#stats-project').inputValue(),'tools');
      assert.equal(await draftPage.locator('#stats-computer').inputValue(),'pc1');
      await standaloneStats.locator('#stats-close').click();
      await settingsOpen.click();
      await settingsNav.locator('[data-tab=agents]').click();
      assert.equal(await draftPage.locator('#agents-computer').inputValue(),'local');
      const agentIcon=settingsDialog.locator('.am-provider-heading .am-provider-icon svg').first();
      assert.deepEqual(await agentIcon.evaluate(icon=>{
        const {width,height}=icon.getBoundingClientRect();
        return [width,height];
      }),[16,16]);
      const agentSettings=draftPage.locator('#agent-setup');
      await settingsDialog.locator('.am-agent-card[data-provider=codex]').getByRole('button',{name:'Настройки',exact:true}).click();
      await agentSettings.waitFor();
      assert.equal(await headerHeight('#agent-setup .am-dialog-header'),dialogHeaderHeight);
      assert(await agentSettings.locator('.am-agent-settings-rows').evaluate(rows=>
        Array.from(rows.children).every(row=>{
          const [label,controls]=row.children;
          const center=element=>{const box=element.getBoundingClientRect();return box.top+box.height/2;};
          return getComputedStyle(row).borderTopWidth==='1px'&&Math.abs(center(label)-center(controls))<1;
        })
      ));
      await agentSettings.getByRole('button',{name:'Изменить модели'}).click();
      const modelPicker=draftPage.locator('#agent-model-picker');
      await modelPicker.waitFor();
      assert.equal(await headerHeight('#agent-model-picker .am-dialog-header'),dialogHeaderHeight);
      const search=modelPicker.getByRole('searchbox',{name:'Поиск моделей',exact:true});
      assert(await search.evaluate(node=>{
        const style=getComputedStyle(node);
        return style.borderRadius==='8px'&&style.height==='28px'&&parseFloat(style.paddingInlineStart)>18;
      }));
      assert.equal(await modelPicker.locator('.am-model-options').evaluate(node=>getComputedStyle(node).paddingInlineEnd),'8px');
      const modelRows=modelPicker.locator('.am-model-option');
      assert.deepEqual(await modelRows.locator('[role=switch][aria-checked=true]').evaluateAll(nodes=>nodes.map(n=>n.dataset.model)),orderedCodex);
      assert(await modelRows.evaluateAll(rows=>rows.every(row=>getComputedStyle(row).borderBlockStartWidth==='0px')));
      await search.fill('учебная-модель');
      await modelPicker.getByRole('button',{name:'Добавить свой вариант',exact:true}).waitFor();
      await search.clear();
      await draftPage.mouse.move(0,0);
      await modelPicker.locator('#agent-model-save').evaluate(async button=>{
        await Promise.all(button.getAnimations().map(animation=>animation.finished));
      });
      assert.deepEqual(await controlStyle(modelPicker.locator('#agent-model-save')),sidebarControl);
      await modelPicker.getByRole('button',{name:'Отмена',exact:true}).click();
      await agentSettings.getByRole('button',{name:'Закрыть',exact:true}).click();
      await select(draftPage,'agents-computer','ПК-1');
      assert.equal(await draftPage.locator('#agents-computer').inputValue(),'pc1');
      const selectedChat=await draftPage.locator('#thread').textContent();
      await settingsDialog.locator('#settings-close').click();
      await settingsDialog.waitFor({state:'hidden'});
      await draftPage.locator('#thread').waitFor();
      assert.equal(await draftPage.locator('#thread').textContent(),selectedChat);
      assert.equal(await nav.locator('[data-tab]:visible').count(),0);
      await select(draftPage,'project-select','Инструменты');
      await plus.click();
      const form=draftPage.locator('#task-draft');
      assert.equal(await form.isVisible(),true);
      assert.equal(await form.locator('#task-draft-project').inputValue(),'tools');
      await draftPage.locator('#task-head').getByRole('heading',{name:'Новая задача',exact:true}).waitFor();
      assert.equal(await form.locator('.am-compose-field').evaluate(node=>getComputedStyle(node).borderRadius),'16px');
      assert.equal(await draftPage.locator('#compose .am-compose-field').evaluate(node=>getComputedStyle(node).borderRadius),'16px');
      const oneLineHeight=(await form.locator('textarea').boundingBox()).height;
      assert.equal(oneLineHeight,32);
      await form.locator('textarea').fill('Первая строка\nВторая строка\nТретья строка');
      assert((await form.locator('textarea').boundingBox()).height>oneLineHeight);
      assert.equal(await form.locator('.am-compose-field').evaluate(node=>getComputedStyle(node).borderRadius),'16px');
      await form.locator('textarea').fill('');
      assert.equal((await form.locator('textarea').boundingBox()).height,oneLineHeight);
      await card(draftPage,'linux').locator('.am-sidebar-task').getByText('Проверка файлов',{exact:true}).click();
      await form.waitFor({state:'hidden'});
      await chat(draftPage,'tools').waitFor();
      assert.equal(await plus.getAttribute('aria-expanded'),'false');
      assert(await draftPage.locator('#conversation-date').isVisible());
      assert.equal(await draftPage.locator('#task-head time').count(),0);
      await select(draftPage,'project-select','Все');
      await plus.click();
      assert.equal(await draftPage.locator('#conversation-date').isVisible(),false);
      const modelTrigger=form.getByRole('button',{name:'Модель',exact:true});
      assert.equal(await modelTrigger.evaluate(node=>{
        const context=node.closest('.am-draft-options'),field=context.previousElementSibling;
        const [plus,text,send]=Array.from(field.children,item=>item.getBoundingClientRect());
        return plus.right<=text.left&&text.right<=send.left&&plus.top===text.top&&text.top===send.top&&
          context.querySelectorAll('.am-select-trigger').length===3&&
          field.getBoundingClientRect().bottom<context.getBoundingClientRect().top;
      }),true);
      assert.equal(await draftPage.locator('#task-draft-send').isDisabled(),true);
      assert.deepEqual(await composerStyle(form.locator('.am-compose-field')),replyStyle);
      const draftLabels=await form.locator('.am-draft-options .am-select-value').all();
      assert.equal(draftLabels.length,3);
      for(const label of draftLabels)assert.deepEqual(await labelStyle(label),replyModelStyle);
      await select(draftPage,'task-draft-project','Инструменты');
      const pathMenu=draftPage.getByRole('dialog',{name:'Добавить путь',exact:true});
      const addPath=form.getByRole('button',{name:'Добавить путь',exact:true});
      await addPath.hover();
      assert(await addPath.evaluate(async button=>{
        await Promise.all(button.getAnimations().map(animation=>animation.finished));
        const style=getComputedStyle(button),field=getComputedStyle(button.closest('.am-compose-field'));
        return parseFloat(style.borderRadius)===parseFloat(field.borderRadius)-4&&style.backgroundColor!=='rgba(0, 0, 0, 0)';
      }));
      await select(draftPage,'task-draft-computer','Мак');
      assert.equal(await form.locator('#task-draft-access').textContent(),'Обычный доступ');
      assert.equal(await accessAbove(form.locator('#task-draft-access')),true);
      await addPath.click();
      const pickRequest=draftPage.waitForRequest(req=>new URL(req.url()).pathname==='/api/pick');
      await pathMenu.getByRole('button',{name:'Выбрать файл…',exact:true}).click();
      assert.deepEqual((await pickRequest).postDataJSON(),{kind:'file',current:''});
      await pathMenu.waitFor({state:'hidden'});
      assert.equal(await form.locator('textarea').inputValue(),'/fictional/project/notes.txt');
      await form.locator('textarea').fill('');
      await select(draftPage,'task-draft-computer','ПК-1');
      assert.equal(await form.locator('#task-draft-access').textContent(),'Доступ задаётся на ПК-1');
      await addPath.click();
      assert.equal(await pathMenu.getByRole('button',{name:'Выбрать файл…'}).isVisible(),false);
      assert.equal(await pathMenu.getByRole('textbox').isVisible(),false);
      await pathMenu.getByRole('button',{name:'Указать путь…'}).click();
      const remotePath=pathMenu.getByRole('textbox',{name:'Путь на компьютере исполнителя'});
      await remotePath.fill('docs/notes.md');
      await remotePath.press('Enter');
      await pathMenu.waitFor({state:'hidden'});
      assert.equal(await form.locator('textarea').inputValue(),'docs/notes.md');
      await form.locator('textarea').fill('');
      await modelTrigger.click();
      const modelMenu=draftPage.locator('#task-draft-model-menu');
      const draftSearch=modelMenu.getByRole('searchbox',{name:'Поиск моделей'});
      assert.equal(await draftSearch.evaluate(node=>node===document.activeElement),true);
      assert.deepEqual(await form.locator('#task-draft-model option').evaluateAll(nodes=>nodes.map(n=>n.value).filter(key=>key.startsWith('codex:'))),orderedCodex.map(id=>'codex:'+id));
      const checkedModel=modelMenu.locator('[aria-selected=true]');
      assert.equal(await checkedModel.textContent(),'Codex Sol 6.2 Medium');
      assert.equal(await checkedModel.locator('.am-select-check svg').count(),1);
      assert.equal(await checkedModel.evaluate(node=>getComputedStyle(node).backgroundColor),'rgba(0, 0, 0, 0)');
      const [searchBox,optionBox]=await Promise.all([draftSearch.boundingBox(),checkedModel.boundingBox()]);
      assert.deepEqual([searchBox.x,searchBox.width],[optionBox.x,optionBox.width]);
      await draftSearch.fill('неизвестная модель');
      assert.equal(await modelMenu.getByRole('option').count(),0);
      assert.equal(await modelMenu.getByRole('status').textContent(),'Модели не найдены');
      await draftSearch.press('Escape');
      assert.equal(await modelTrigger.evaluate(node=>node===document.activeElement),true);
      await modelTrigger.press('ArrowDown');
      await draftSearch.fill('astra 6.2 high');
      const astra=modelMenu.getByRole('option',{name:'Codex Astra 6.2 High',exact:true});
      await draftSearch.press('ArrowDown');
      assert.equal(await astra.evaluate(node=>node===document.activeElement),true);
      await astra.press('Enter');
      await modelMenu.waitFor({state:'hidden'});
      await form.locator('textarea').fill(directTask.title);
      const start=draftPage.waitForRequest(req=>new URL(req.url()).pathname==='/api/task-start');
      await form.locator('textarea').press('Enter');
      const {op_id,...payload}=(await start).postDataJSON();
      assert.match(op_id,/^window-/);
      assert.deepEqual(payload,{project:'tools',computer:'pc1',agent:'codex',model:'gpt-6.2-astra[effort=high]',text:directTask.title});
      const ownerBubble=draftPage.locator('#thread .am-outgoing').filter({hasText:directTask.title});
      const conversation=draftPage.locator('#thread');
      await conversation.getByText('Настройки сохранены',{exact:true}).waitFor();
      assert.equal(await conversation.getByText('Настройки сохранены',{exact:true}).count(),1);
      assert.equal(await conversation.getByText('Проверяю настройки',{exact:true}).isVisible(),false);
      await conversation.getByText('Ищу последнюю запись',{exact:true}).waitFor();
      await conversation.getByText('Ход работы',{exact:true}).click();
      await conversation.getByText('Проверяю настройки',{exact:true}).waitFor();
      await ownerBubble.waitFor();
      assert.equal(await ownerBubble.locator('.am-meta').count(),0);
      const ownerMessage=ownerBubble.locator('..');
      assert.equal(await ownerMessage.locator('.am-message-actions time').count(),1);
      await draftPage.evaluate(()=>{
        Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.testCopiedMessage=text;}}});
      });
      const reply=conversation.locator('article').filter({has:draftPage.getByText('Настройки сохранены',{exact:true})});
      const workerCopy=reply.getByRole('button',{name:'Скопировать сообщение'});
      await workerCopy.focus();
      await workerCopy.press('Enter');
      await reply.getByRole('status').getByText('Скопировано',{exact:true}).waitFor();
      assert.equal(await draftPage.evaluate(()=>window.testCopiedMessage),directReply);
      await ownerBubble.hover();
      const copy=ownerMessage.getByRole('button',{name:'Скопировать сообщение'});
      assert.deepEqual(await copy.locator('svg').evaluate(icon=>{
        const style=getComputedStyle(icon);
        return [style.fill,style.stroke===style.color];
      }),['none',true]);
      await copy.click();
      await ownerMessage.getByRole('status').getByText('Скопировано',{exact:true}).waitFor();
      assert.equal(await draftPage.evaluate(()=>window.testCopiedMessage),directTask.title);
      await draftPage.locator('#answer-text').fill('Уточнение исполнителю');
      assert.equal(await draftPage.locator('#answer-access').textContent(),'Доступ задаётся на ПК-1');
      assert.equal(await accessAbove(replyAccess),true);
      await draftPage.locator('#answer-add').click();
      assert.equal(await pathMenu.getByRole('button',{name:'Выбрать файл…'}).isVisible(),false);
      await pathMenu.getByRole('button',{name:'Указать путь…'}).click();
      await remotePath.fill('docs/reply.md');
      await remotePath.press('Enter');
      assert.equal(await draftPage.locator('#answer-text').inputValue(),'Уточнение исполнителю\ndocs/reply.md');
      const note=draftPage.waitForRequest(req=>new URL(req.url()).pathname==='/api/task-note');
      await draftPage.locator('#answer-send').click();
      const {task:noteTask,text:noteText}=(await note).postDataJSON();
      assert.deepEqual([noteTask,noteText],[directTask.id,'Уточнение исполнителю\ndocs/reply.md']);
      assert.equal(await form.isVisible(),false);
      const drawer=draftPage.locator('#list-toggle');
      assert.equal(await drawer.getAttribute('aria-label'),'Показать архив');
      await drawer.click();
      assert.equal(await drawer.getAttribute('aria-label'),'Скрыть архив');
      const archive=draftPage.getByRole('complementary',{name:'Архив'});
      assert.equal((await archive.boundingBox()).width,(await draftPage.locator('.am-sidebar').boundingBox()).width);
      const archiveGeometry=()=>archive.evaluate(panel=>{
        const rect=selector=>document.querySelector(selector).getBoundingClientRect();
        const main=rect('.am-main'),aside=rect('.am-sidebar'),bounds=panel.getBoundingClientRect();
        const toolbar=rect('.am-task-toolbar'),header=rect('.am-queue-header'),filter=rect('#agent-filter + .am-select-trigger');
        const projects=rect('.am-project'),heading=rect('#task-list-heading');
        return {
          matchesWorkspace:[bounds.top-toolbar.bottom,main.bottom-bounds.bottom,main.right-bounds.right],
          headerEdges:[header.left-bounds.left-panel.clientLeft,bounds.right-header.right],
          fields:[heading.left-bounds.left-panel.clientLeft,bounds.right-filter.right,projects.left-aside.left,aside.right-projects.right-1],
          controls:[filter.height,rect('#project-select + .am-select-trigger').height],
          toolbar:[toolbar.left-main.left,main.right-toolbar.right,projects.top-rect('#task-new').bottom],
          rowAlignment:[header.top-bounds.top,filter.top-header.top],
        };
      });
      assert.deepEqual(await archiveGeometry(),{
        matchesWorkspace:[0,0,0],headerEdges:[0,0],fields:[16,16,16,16],controls:[28,28],toolbar:[0,0,8],rowAlignment:[0,16],
      });
      assert.equal(await archive.locator('#task-list-heading').textContent(),'Архив');
      const trashBounds=await archive.locator('#archive-clear').boundingBox();
      const filterBounds=await archive.locator('#agent-filter + button').boundingBox();
      assert(trashBounds.x+trashBounds.width<filterBounds.x);
      for(const theme of ['light','dark']){
        const styles=await draftPage.evaluate(async theme=>{
          const doc=document.documentElement,old=doc.dataset.theme;
          doc.dataset.theme=theme;
          try{
            const controls=['#project-column-first button','#agent-filter + button'].map(sel=>document.querySelector(sel));
            await Promise.all(controls.flatMap(n=>n.getAnimations().map(a=>a.finished)));
            return controls.map(n=>{
              const style=getComputedStyle(n);
              return [style.backgroundColor,style.color,style.borderColor];
            });
          }finally{doc.dataset.theme=old;}
        },theme);
        assert.deepEqual(styles[0],styles[1]);
      }
      const clearArchive=archive.getByRole('button',{name:'Очистить архив',exact:true});
      for(const id of ['project-select','agent-filter']){
        const trigger=draftPage.locator('[aria-controls="'+id+'-menu"]'),menu=draftPage.locator('#'+id+'-menu');
        await trigger.click();
        await menu.waitFor();
        await trigger.click();
        await menu.waitFor({state:'hidden'});
        assert.equal(await trigger.getAttribute('aria-expanded'),'false');
        await trigger.press('ArrowDown');
        await menu.waitFor();
        await draftPage.keyboard.press('Escape');
        await menu.waitFor({state:'hidden'});
      }
      const clearBox=await clearArchive.boundingBox(),filterBox=await archive.locator('.am-select-trigger').boundingBox();
      assert.equal(filterBox.x-clearBox.x-clearBox.width,8);
      assert.equal(clearBox.y,filterBox.y);
      assert.equal(clearBox.width,28);
      assert.equal(clearBox.height,28);
      assert.equal(await clearArchive.locator('svg').count(),1);
      assert.deepEqual(await archive.locator('.am-queue-header').evaluate(n=>{
        const s=getComputedStyle(n);
        return [s.borderBottomWidth,s.backgroundColor,s.paddingBlockStart,s.paddingBlockEnd,s.paddingInlineStart,s.paddingInlineEnd];
      }),['0px','rgba(0, 0, 0, 0)','16px','16px','16px','16px']);
      const hides=()=>seen.filter(req=>['/api/task-hide','/api/task-hide-history'].includes(req.path)).length;
      const initialHides=hides();
      await clearArchive.click();
      await draftPage.getByText('Очистить архив?',{exact:true}).waitFor();
      assert.deepEqual(await draftPage.locator('#confirm-dialog').evaluate(dialog=>{
        const title=dialog.querySelector('h3').getBoundingClientRect();
        const text=dialog.querySelector('#confirm-text').getBoundingClientRect();
        const actions=dialog.querySelector('.am-alert-actions').getBoundingClientRect();
        return [text.top-title.bottom,actions.top-text.bottom];
      }),[12,12]);
      await draftPage.getByRole('button',{name:'Отмена',exact:true}).click();
      assert.equal(hides(),initialHides);
      const headingStyles=()=>draftPage.evaluate(()=>{
        return ['#project-heading','#task-head > h3','#task-list-heading'].map(selector=>{
          const heading=document.querySelector(selector),style=getComputedStyle(heading);
          const bounds=heading.getBoundingClientRect();
          return [style.fontSize,style.fontWeight,style.lineHeight,bounds.top+bounds.height/2];
        });
      });
      const [projectHeading,taskHeading,archiveHeading]=await headingStyles();
      assert.deepEqual(taskHeading.slice(0,3),projectHeading.slice(0,3));
      assert.equal(taskHeading[3],await verticalCenter(plus));
      assert.deepEqual(archiveHeading.slice(0,3),projectHeading.slice(0,3));
      assert(archiveHeading[3]>taskHeading[3]);
      await archive.getByText('Инструменты, Готовый отчёт',{exact:true}).waitFor();
      assert.deepEqual((await archive.locator('.am-task-title').allTextContents()).sort(),['Игра, Звуки меню','Инструменты, Готовый отчёт','Инструменты, Отчёт другого компьютера']);
      assert.equal(await archive.locator('#task-count').textContent(),'3');
      assert.equal(await draftPage.locator('#project-count').textContent(),String(projects.length));
      for(const [title,count] of [['#project-heading','#project-count'],['#task-list-heading','#task-count']]){
        const baselines=await draftPage.evaluate(selectors=>selectors.map(selector=>{
          const probe=document.createElement('span');
          probe.style.display='inline-block';
          document.querySelector(selector).append(probe);
          const baseline=probe.getBoundingClientRect().bottom;
          probe.remove();
          return baseline;
        }),[title,count]);
        assert(Math.abs(baselines[0]-baselines[1])<=1,JSON.stringify(baselines));
      }
      await card(draftPage,'pc1').getByText('Ждёт запуска',{exact:false}).waitFor();
      const moreArchive=archive.getByRole('button',{name:/^Показать ещё/});
      assert.equal(await moreArchive.count(),1);
      assert(await moreArchive.evaluate(button=>button.parentElement===document.getElementById('task-list').lastElementChild));
      await moreArchive.click();
      await archive.getByText('Инструменты, Старый отчёт',{exact:true}).waitFor();
      const older=archive.locator('.am-task').filter({hasText:'Старый отчёт'});
      assert.match(await older.locator('time').textContent(),/^\d{1,2} .+ в 03:00$/);
      await select(draftPage,'agent-filter','Claude Code');
      const emptyArchive=archive.getByText('В архиве пока нет задач');
      await emptyArchive.waitFor();
      assert.equal(await emptyArchive.evaluate(node=>node.getBoundingClientRect().left+
        parseFloat(getComputedStyle(node).paddingLeft)-
        document.getElementById('task-list-heading').getBoundingClientRect().left),0);
      assert.equal(await card(draftPage,'pc1').getByText('Ждёт запуска',{exact:false}).count(),1);
      await select(draftPage,'agent-filter','Все агенты');
      await archive.getByText('Игра, Звуки меню',{exact:true}).waitFor();
      const row=archive.locator('.am-task').filter({hasText:'Звуки меню'});
      await row.waitFor();
      assert.equal(await row.evaluate(node=>getComputedStyle(node).padding),'4px 12px');
      for(const rowSelector of ['.am-archive-task','.am-sidebar-task']){
        await draftPage.waitForFunction(selector=>{
          const row=document.querySelector(selector),panel=row?.closest('aside');
          if(!row||!panel)return false;
          const box=row.getBoundingClientRect(),outer=panel.getBoundingClientRect(),s=getComputedStyle(panel);
          return box.width>0 && box.left-outer.left-parseFloat(s.borderLeftWidth)===16 &&
            outer.right-box.right-parseFloat(s.borderRightWidth)===16;
        },rowSelector);
      }
      const time=row.locator('time');
      assert.match(await time.textContent(),/^(Сегодня|Вчера) в \d{2}:\d{2}$/);
      assert.equal(await time.getAttribute('datetime'),tasks.game.find(t=>t.id==='game-3').finished_at);
      assert.equal(await time.evaluate(node=>node.parentElement.firstElementChild===node),true);
      assert.match(await time.getAttribute('title'),/^Компьютер: ПК-2\n/);
      assert.equal(await row.getAttribute('aria-description'),'Компьютер: ПК-2');
      assert.equal(await row.locator('.am-archive-task-meta').textContent(),'Codex Sol 6.2 Medium, 12 мин, 4,3 млн т.');
      assert.match(await row.locator('.am-task-usage[title]').getAttribute('title'),/на входе/);
      const shared=archive.locator('.am-task').filter({hasText:'Отчёт другого компьютера'});
      assert.equal(await shared.locator('time').textContent(),'Вчера в 03:00');
      assert.match(await shared.locator('time').getAttribute('title'),/^Компьютер: ПК-1\n/);
      assert.match(await shared.locator('.am-archive-task-meta').textContent(),/, 342 тыс\. т\.$/);
      for(const task of [row,local.locator('.am-sidebar-task').first()]){
        const icon=task.locator('.am-task-model-line [data-provider=codex] svg');
        assert.deepEqual(await icon.evaluate(node=>{
          const rect=node.getBoundingClientRect();
          return [rect.width,rect.height];
        }),[12,12]);
      }
      const metrics=target=>target.evaluate(node=>{
        const title=node.querySelector('.am-task-title,.am-sidebar-task-title');
        const meta=title.nextElementSibling;
        return [meta.getBoundingClientRect().top-title.getBoundingClientRect().bottom,
          getComputedStyle(meta).lineHeight,getComputedStyle(node).paddingInlineStart,
          getComputedStyle(node).paddingInlineEnd,node.getBoundingClientRect().width];
      });
      const [gap,line,left,right,width]=await metrics(row);
      assert.deepEqual([gap,line,left,right],[2,'18px','12px','12px']);
      for(const machine of [local,card(draftPage,'pc1')]){
        const disclosure=machine.locator('.am-sidebar-disclosure');
        if(await disclosure.getAttribute('aria-expanded')==='false')await disclosure.click();
        const task=machine.locator('.am-sidebar-task').first();
        await task.waitFor();
        await task.evaluate(async node=>{
          await Promise.all(node.closest('.am-sidebar-tasks').getAnimations().map(animation=>animation.finished.catch(()=>{})));
        });
        assert.deepEqual((await metrics(task)).slice(0,4),[2,'18px','36px','12px'],await machine.getAttribute('data-machine-order'));
        const [heading,taskTitle]=await Promise.all([
          machine.locator('.am-machine-label').boundingBox(),
          task.locator('.am-sidebar-task-title').boundingBox(),
        ]);
        assert.equal(taskTitle.x,heading.x);
        const [button,machineRow]=await Promise.all([task.boundingBox(),machine.locator('.am-machine-name').boundingBox()]);
        assert.deepEqual([button.x,button.width],[machineRow.x,machineRow.width]);
      }
      for(const meta of [local.locator('.am-sidebar-task-meta').first(),row.locator('.am-archive-task-meta')]){
        assert.deepEqual(await meta.evaluate(node=>[getComputedStyle(node).whiteSpace,getComputedStyle(node).textOverflow]),['nowrap','ellipsis']);
        assert.equal(await meta.getAttribute('title'),await meta.textContent());
      }
      await draftPage.locator('#task-panel > .am-splitter').press('Shift+ArrowLeft');
      const [, ,wideLeft,wideRight,wideWidth]=await metrics(row);
      assert.deepEqual([wideLeft,wideRight],['12px','12px']);
      assert(wideWidth>width);
      const resize=draftPage.locator('#task-panel > .am-splitter');
      await resize.press('Home');
      assert.equal((await archive.boundingBox()).width,(await draftPage.locator('.am-sidebar').boundingBox()).width);
      await resize.press('Shift+ArrowRight');
      assert(await row.locator('.am-archive-task-meta').evaluate(node=>node.scrollWidth>node.clientWidth));
      await select(draftPage,'project-select','Игра');
      await row.getByText('Звуки меню',{exact:true}).waitFor();
      assert.equal(await draftPage.locator('#project-count').textContent(),String(projects.length));
      assert.equal(await shared.count(),0);
      await select(draftPage,'project-select','Все');
      await shared.click();
      await draftPage.locator('#thread').getByText('Общий результат',{exact:true}).waitFor();
    }finally{await draftPage.close();}

    const mac=await open(projects,{width:1040,height:780},'dark',(url,p)=>{
      if(url.pathname==='/api/task'&&url.searchParams.get('id')==='game-1'){
        const response=reply(url,p,projects);
        response.items.splice(1,0,{kind:'accepted',at:response.task.created_at,text:'ПК-1 $&'});
        response.items.push({...response.items.at(-1),text:'Длинное сообщение '.repeat(1000)});
        return {json:response};
      }
      if(p!=='game'||!['/api/window','/api/project-activity'].includes(url.pathname))return;
      const view=reply(url,p,projects);
      return {json:{...view,tasks:view.tasks.map(t=>t.id==='game-2'?{...t,state:'running',created_at:iso(0),remote:{...pc2,unaccepted:true}}:t)}};
    },true,'mac-titlebar');
    try{
      const toolbar=mac.locator('.am-task-toolbar'),projectsRow=mac.locator('.am-project'),nav=mac.locator('#computer-sidebar-toggle');
      const box=await toolbar.boundingBox(),row=await projectsRow.boundingBox(),control=await nav.boundingBox();
      assert.match(await mac.locator('.am-sidebar-task').filter({hasText:'Проверка навигации'}).textContent(),/Ожидает принятия$/);
      for(const selector of ['.am-outgoing','.am-message:not(.am-outgoing)']){
        assert.equal(await mac.locator('#thread '+selector+' .am-meta').first().evaluate(n=>
          n.nextElementSibling.getBoundingClientRect().top-n.getBoundingClientRect().bottom),8);
      }
      assert.equal(box.height,44);
      const actionBox=await mac.locator('#task-new').boundingBox();
      assert.equal(actionBox.y-(box.y+box.height),16);
      assert.equal(row.y-actionBox.y-actionBox.height,8);
      const firstComputer=await mac.locator('#machines .am-machine-name').first().boundingBox();
      assert.equal(firstComputer.y-row.y-row.height,8);
      assert.equal(control.y+control.height/2,box.y+box.height/2);
      assert.equal(await mac.locator('.am-computers').evaluate(n=>getComputedStyle(n).borderBlockStartWidth),'0px');
      await mac.waitForFunction(()=>window.titlebarGeometry?.holes.some(([x,y,w,h])=>{
        const r=document.querySelector('#computer-sidebar-toggle').getBoundingClientRect();
        return x===Math.round(r.x)&&y===Math.round(r.y)&&w===r.width&&h===r.height;
      }));
      await mac.locator('#list-toggle').click();
      const archive=mac.locator('#task-panel');
      await archive.waitFor();
      for(const selector of ['#project-heading','#project-column-first .am-select-trigger']){
        assert.equal(await verticalCenter(mac.locator(selector)),row.y+row.height/2);
      }
      const projectFilter=await mac.locator('#project-column-first .am-select-trigger').boundingBox(),agentFilter=await mac.locator('.am-task-filter .am-select-trigger').boundingBox();
      assert.equal(projectFilter.width,agentFilter.width);
      assert.equal(agentFilter.y,(await mac.locator('#task-new').boundingBox()).y);
      if(output){
        await mac.setViewportSize({width:1440,height:840});
        await mac.screenshot({path:path.join(output,'kotel-sidebar-117.png')});
        await mac.setViewportSize({width:1040,height:780});
      }
      for(const [container,heading,selector] of [
        ['.am-sidebar','#project-heading','#project-select + button'],
        ['.am-queue-header','#task-list-heading','#agent-filter + button']
      ]){
        const pane=mac.locator(container);
        await pane.evaluate(el=>el.style.inlineSize='180px');
        const trigger=await mac.locator(selector).boundingBox();
        const label=await mac.locator(heading).boundingBox();
        assert(trigger.y>=label.y+label.height);
        const innerWidth=await pane.evaluate(el=>el.clientWidth-parseFloat(getComputedStyle(el).paddingLeft)-parseFloat(getComputedStyle(el).paddingRight));
        assert.equal(trigger.width,innerWidth);
        await pane.evaluate(el=>el.style.removeProperty('inline-size'));
      }
      assert.equal(await mac.locator('.am-sidebar').evaluate(n=>getComputedStyle(n,'::before').content),'none');
      assert.equal(await toolbar.evaluate(n=>getComputedStyle(n).borderBottomWidth),'1px');
      const date=mac.locator('#conversation-date');
      await mac.locator('#thread').evaluate(n=>n.scrollTop=0);
      const start=await date.boundingBox();
      assert.equal(start.y-box.y-box.height,12);
      await mac.locator('.am-sidebar-task').filter({hasText:'Экран настроек'}).click();
      await mac.locator('#task-head h3').getByText('Экран настроек').waitFor();
      await mac.locator('#thread').getByText('Принято на компьютере ПК-1 $&',{exact:true}).waitFor();
      assert.equal(await date.getAttribute('datetime'),tasks.game[0].created_at);
      assert(await date.evaluate(n=>n===n.parentNode.firstElementChild&&n.parentNode.id==='thread'));
      const scrolled=await mac.locator('#thread').evaluate(n=>{
        n.scrollTop=40;
        return n.scrollTop;
      });
      assert.equal(scrolled,40);
      assert.equal((await date.boundingBox()).y,start.y-scrolled);
      assert.equal((await toolbar.boundingBox()).y,box.y);
      await nav.press('Enter');
      assert.equal(await projectsRow.isVisible(),false);
      assert.equal(await nav.getAttribute('aria-expanded'),'false');
      assert(await archive.isVisible());
      assert((await toolbar.boundingBox()).width>box.width);
      await nav.press('Space');
      assert.deepEqual(await projectsRow.boundingBox(),row);
      await mac.evaluate(()=>localStorage.setItem('messenger-lang','en'));
      await mac.reload();
      await mac.locator('.am-sidebar-task-meta').filter({hasText:'Awaiting acceptance'}).waitFor();
      await mac.locator('.am-sidebar-task').filter({hasText:'Экран настроек'}).click();
      await mac.locator('#thread').getByText('Accepted on ПК-1 $&',{exact:true}).waitFor();
    }finally{await mac.close();}

    let moving=null;
    const taskMotion=await open(projects,{width:1260,height:900},'dark',(url,p)=>{
      if(p!=='game'||!['/api/window','/api/project-activity'].includes(url.pathname))return;
      const view=reply(url,p,projects);
      return {json:{...view,tasks:moving?[moving,...view.tasks]:view.tasks}};
    },true);
    try{
      await taskMotion.locator('#list-toggle').click();
      await taskMotion.locator('#task-list').waitFor();
      await taskMotion.evaluate(()=>{
        const animate=Element.prototype.animate;
        window.taskTransitions=[];
        window.motionGroupOverflow=[];
        Element.prototype.animate=function(frames,options){
          if(this.dataset.motionTask||this.classList.contains('am-task-exits')){
            window.taskTransitions.push([this.closest('.am-computers')?'left':'archive',this.dataset.motionTask||'exit']);
          }
          if(frames[0].transform&&this.classList.contains('am-sidebar-task')){
            const group=this.parentElement;
            requestAnimationFrame(()=>{if(group.isConnected)window.motionGroupOverflow.push(getComputedStyle(group).overflow);});
          }
          return animate.call(this,frames,options);
        };
      });
      const liveTask=makeTask('motion-probe','game','codex','Новая проверка движения',0);
      const side=taskMotion.locator('#machines [data-motion-task]').filter({hasText:liveTask.title});
      const archived=taskMotion.locator('#task-list [data-motion-task]').filter({hasText:liveTask.title});
      moving=liveTask;
      await side.waitFor();
      moving={...moving,state:'done',finished_at:iso(0)};
      await archived.waitFor();
      await side.waitFor({state:'detached'});
      moving=null;
      await archived.waitFor({state:'detached'});
      await taskMotion.locator('.am-task-exits').first().waitFor({state:'detached'});
      const transitions=await taskMotion.evaluate(()=>window.taskTransitions);
      for(const panel of ['left','archive']){
        assert(transitions.some(([list,key])=>list===panel&&key.includes('motion-probe')));
        assert(transitions.some(([list,key])=>list===panel&&key==='exit'));
      }
      const groupOverflow=await taskMotion.evaluate(()=>window.motionGroupOverflow);
      assert(groupOverflow.length>0&&groupOverflow.every(value=>value==='visible'));
      await taskMotion.emulateMedia({reducedMotion:'reduce'});
      await taskMotion.evaluate(()=>window.taskTransitions=[]);
      moving=liveTask;
      await side.waitFor();
      moving=null;
      await side.waitFor({state:'detached'});
      assert.deepEqual(await taskMotion.evaluate(()=>window.taskTransitions),[]);
    }finally{await taskMotion.close();}

    const windows=await open(projects,{width:1040,height:780},'dark',()=>{},true,'windows');
    try{
      const toggle=windows.locator('.am-window-controls > #list-toggle');
      await toggle.waitFor();
      const list=await toggle.boundingBox();
      const minimize=await windows.getByRole('button',{name:'Свернуть',exact:true}).boundingBox();
      assert(list.x+list.width<=minimize.x);
      assert.equal(list.y+list.height/2,minimize.y+minimize.height/2);
      await toggle.click();
      await windows.locator('#task-panel').waitFor();
    }finally{await windows.close();}

    const browserPreview=await open(projects,{width:1150,height:770},'dark',(url,p)=>{
      if(url.pathname!=='/api/server')return;
      const server=reply(url,p,projects);
      return {json:{...server,peers:server.peers.map(peer=>peer.id==='pc2'?{...peer,age:90}:peer)}};
    },true,null);
    try{
      const toggle=browserPreview.locator('#list-toggle');
      assert.equal(await browserPreview.locator('.am-titlebar').isVisible(),false);
      const localMark=card(browserPreview,'local').getByRole('img',{name:'Этот компьютер — На связи'});
      const remoteMark=card(browserPreview,'pc1').getByRole('img',{name:'На связи',exact:true});
      const offlineMark=card(browserPreview,'pc2').getByRole('img',{name:'Нет связи',exact:true});
      await offlineMark.locator('.am-machine-signal svg').waitFor();
      for(const mark of [localMark,remoteMark,offlineMark]){
        assert.match(await mark.locator(':scope > svg path').getAttribute('d'),/M4 3.*M4 14/);
      }
      const markColor=mark=>mark.evaluate(node=>getComputedStyle(node).color);
      assert.deepEqual(await Promise.all([localMark,remoteMark,offlineMark].map(markColor)),
        Array(3).fill(await markColor(remoteMark)));
      assert.equal(await remoteMark.locator('.am-machine-signal').evaluate(node=>getComputedStyle(node).borderRadius),'50%');
      assert.equal(await browserPreview.locator('.am-shell').evaluate(shell=>
        shell.getBoundingClientRect().top-shell.closest('.am-window').getBoundingClientRect().top),1);
      await toggle.click();
      const filter=await browserPreview.locator('#agent-filter + .am-select-trigger').boundingBox();
      const button=await toggle.boundingBox();
      assert(filter.y>=button.y+button.height+12);
      assert.deepEqual(await browserPreview.locator('#thread .am-message-group').first().evaluate(message=>{
        const pane=message.closest('.am-chat').getBoundingClientRect();
        const chat=message.getBoundingClientRect();
        const footer=document.querySelector('.am-chat-footer').getBoundingClientRect();
        return [chat.left-pane.left,pane.right-chat.right,chat.left-footer.left,footer.right-chat.right];
      }),[24,24,0,0]);
      await toggle.click();
      await browserPreview.locator('#task-panel').waitFor({state:'hidden'});
      await browserPreview.setViewportSize({width:450,height:720});
      const plus=await browserPreview.locator('#task-new').boundingBox();
      const narrowToggle=await toggle.boundingBox();
      assert(plus.y+plus.height<=narrowToggle.y);
      await toggle.click();
      const narrowArchive=browserPreview.locator('#task-panel');
      await narrowArchive.waitFor();
      const toolbar=await browserPreview.locator('.am-task-toolbar').boundingBox();
      assert.equal((await narrowArchive.boundingBox()).y,toolbar.y+toolbar.height);
    }finally{await browserPreview.close();}

    const compactPreview=await open(projects,{width:450,height:720},'light',(url,p)=>{
      if(url.pathname==='/api/window')return {json:{...reply(url,p,projects),coordinator_link:{
        state:'unknown',detail:'Файл настроек проекта недоступен. Проверьте разрешения на папку проекта и попробуйте подключиться ещё раз.',
      }}};
    },true);
    try{
      const plus=compactPreview.getByRole('button',{name:'Новая задача',exact:true});
      await plus.waitFor();
      assert.equal(await compactPreview.locator('.am-project').evaluate(node=>node.scrollWidth>node.clientWidth),false);
      assert.equal(await compactPreview.locator('body').evaluate(node=>node.scrollWidth>innerWidth),false);
      await plus.click();
      await compactPreview.locator('#task-draft-text').waitFor();
      await compactPreview.locator('#task-draft-text').fill('Черновик');
      await compactPreview.locator('#settings-open').click();
      await compactPreview.waitForFunction(()=>document.activeElement.id==='settings-title');
      const closeSettings=compactPreview.locator('#settings-close');
      assert.equal(await closeSettings.evaluate(n=>n.matches(':hover,:focus-visible')),false);
      await closeSettings.hover();
      assert(await closeSettings.evaluate(n=>n.matches(':hover')));
      await compactPreview.mouse.move(1,1);
      // WebKit's default Mac keyboard setting reaches buttons with Option-Tab.
      await compactPreview.keyboard.press(engine==='webkit'?'Alt+Tab':'Tab');
      assert(await closeSettings.evaluate(n=>n.matches(':focus-visible')));
      const modal=compactPreview.locator('#settings-dialog');
      await modal.waitFor();
      assert.equal(await modal.evaluate(node=>node.scrollWidth>node.clientWidth),false);
      assert.deepEqual(await modal.locator('.am-settings-nav [data-tab]').evaluateAll(tabs=>tabs.map(tab=>tab.dataset.tab)),['settings','project','agents','server']);
      assert.deepEqual(await modal.locator('.am-settings-nav [data-tab] > svg').evaluateAll(icons=>icons.map(icon=>{
        const {width,height}=icon.getBoundingClientRect();
        return [width,height,icon.getAttribute('aria-hidden')];
      })),Array(4).fill([18,18,'true']));
      await modal.locator('[data-tab=server]').click();
      const name=modal.getByRole('textbox',{name:'Имя в сети',exact:true});
      const nameLabel=modal.locator('label[for=computer-name] > span');
      await expectCentered(nameLabel,name);
      assert((await name.boundingBox()).x>(await nameLabel.boundingBox()).x);
      await modal.locator('[data-tab=project]').click();
      const plugin=modal.locator('#coord-box .am-step').first();
      const subtitle=plugin.locator('.am-step-sub');
      await subtitle.waitFor();
      assert(await subtitle.evaluate(node=>node.clientHeight>parseFloat(getComputedStyle(node).lineHeight)));
      await expectCentered(plugin.locator('.am-step-text'),plugin.locator(':scope > button'));
      await modal.locator('#settings-close').click();
      await compactPreview.locator('#stats-open').click();
      const statisticsDialog=compactPreview.locator('#stats-dialog');
      await statisticsDialog.waitFor();
      assert.equal(await modal.isVisible(),false);
      await total(compactPreview,'103578');
      await statisticsDialog.locator('#stats-close').click();
      assert.equal(await compactPreview.locator('#task-draft-text').inputValue(),'Черновик');
    }finally{await compactPreview.close();}

    const single=await open(projects.slice(0,1),{width:1100,height:706},'light');
    assert.equal(await split(single).isVisible(),false);
    assert.equal(await single.locator('#project-column-second').isVisible(),false);
    assert.equal(await single.locator('.am-machine-project').count(),0);
    await single.locator('[data-tab=stats]').click();
    await total(single,'1234');
    assert.equal(await single.locator('#stats-project-filter').isVisible(),false);
    await single.close();

    const combined=await open(projects,{width:1197,height:706},'dark',(url,p)=>{
      if(['/api/task-hide-history','/api/task-hide'].includes(url.pathname))return {json:{ok:true}};
      if(url.pathname==='/api/task'&&[toolDone.id,toolOlder.id].includes(url.searchParams.get('id'))){
        const task=url.searchParams.get('id')===toolOlder.id?toolOlder:toolDone;
        return {json:{task,items:[{kind:'text',author:task.agent,at:iso(2),text:'Сообщение из tools'}]}};
      }
      if(p!=='tools')return;
      const result=reply(url,p,projects),computer=url.searchParams.get('computer');
      if(url.pathname==='/api/task-history')return {json:{tasks:[toolOlder]}};
      if(url.pathname==='/api/window'||url.pathname==='/api/project-activity'&&url.searchParams.get('history')==='1'){
        return {json:{...result,tasks:[...result.tasks,...(!computer?[toolDone]:[])],shared:[toolShared],history_paged:true,history_next:computer?'':toolDone.id}};
      }
    });
    const list=combined.locator('#task-list');
    const task=(title)=>list.getByText(title,{exact:true});
    const gameTasks=list.getByRole('region',{name:'Игра'}),toolsTasks=list.getByRole('region',{name:'Инструменты'});
    assert.equal(await task('Проверка файлов').count(),0);
    await split(combined).click();
    await gameTasks.getByText('Звуки меню',{exact:true}).waitFor();
    await toolsTasks.getByText('Готовый отчёт',{exact:true}).waitFor();
    await toolsTasks.getByRole('button',{name:'Показать ещё 10'}).click();
    await task('Старый отчёт').waitFor();
    assert(seen.some(r=>r.path==='/api/task-history'&&r.params.get('project')==='tools'&&r.params.get('before')==='tools-done'&&!r.params.get('computer')));
    assert.equal(await chat(combined,'game').isVisible(),true);
    await task('Старый отчёт').click();
    await chat(combined,'tools').waitFor();
    await task('Старый отчёт').waitFor();
    await task('Экран настроек').click();
    await chat(combined,'game').waitFor();
    await toolsTasks.getByText('Отчёт другого компьютера',{exact:true}).click();
    await combined.locator('#thread').getByText('Общий результат',{exact:true}).waitFor();
    await gameTasks.getByText('Проверка навигации',{exact:true}).waitFor();
    await cells(combined,'pc1').nth(0).locator('.am-machine-name').click();
    await toolsTasks.waitFor({state:'hidden'});
    await task('Проверка навигации').waitFor();
    assert.deepEqual(await list.locator('.am-task-title').allTextContents(),['Проверка навигации']);
    await cells(combined,'pc1').nth(0).locator('.am-machine-name').click();
    await toolsTasks.getByText('Проверка файлов',{exact:true}).waitFor();
    await gameTasks.getByText('Экран настроек',{exact:true}).waitFor();
    await cells(combined,'linux').nth(1).locator('.am-machine-name').click();
    await gameTasks.waitFor({state:'hidden'});
    await task('Проверка файлов').waitFor();
    assert.deepEqual(await list.locator('.am-task-title').allTextContents(),['Проверка файлов']);
    await cells(combined,'linux').nth(1).locator('.am-machine-name').click();
    await gameTasks.getByText('Экран настроек',{exact:true}).waitFor();
    await task('Экран настроек').click();
    await chat(combined,'game').waitFor();
    await toolsTasks.getByText('Готовый отчёт',{exact:true}).waitFor();
    assert.equal(await combined.locator('.am-machine-project[aria-pressed=true]').count(),0);
    await toolsTasks.getByRole('button',{name:'Убрать все',exact:true}).click();
    await combined.locator('#confirm-go').click();
    await combined.locator('#confirm-dialog').waitFor({state:'hidden'});
    const cleared=seen.findLast(r=>r.path==='/api/task-hide-history');
    assert.deepEqual(JSON.parse(cleared.body),{project:'tools',agent:'',computer:''});
    await select(combined,'project-select-second','Сайт');
    await task('Подготовка сайта').waitFor();
    assert.equal(await toolsTasks.count(),0);
    await split(combined).click();
    assert.equal(await task('Подготовка сайта').count(),0);
    assert.equal(await task('Экран настроек').count(),1);
    await combined.close();

    const available=[...projects];
    let loadTools;
    const toolsActivity=new Promise(resolve=>{loadTools=resolve;});
    const page=await open(available,{width:1197,height:706},'dark',
      (url,project)=>{if(url.pathname==='/api/project-activity'&&project==='tools')return toolsActivity;});
    assert.equal(await card(page,'local').count(),1);
    await card(page,'pc1').getByText('ждёт ответа').waitFor();
    const stableCards=['local','pc1','pc2'];
    const cardSizes=()=>Promise.all(stableCards.map(id=>card(page,id).boundingBox()));
    const singleSizes=await cardSizes();
    await split(page).click();
    await cells(page,'pc2').nth(1).getByText('Codex',{exact:true}).waitFor();
    assert.equal(await cells(page,'pc2').nth(1).getAttribute('title'),'Загружаем статусы…');
    assert.equal(await card(page,'vps').getByText('Агенты не подключены',{exact:true}).count(),2);
    const coordinatorRole=card(page,'local').locator('.am-machine-name').getByText('координатор',{exact:true});
    assert.equal(await coordinatorRole.count(),0);
    loadTools();
    await cells(page,'local').nth(1).getByText('ждёт ответа').waitFor();
    await coordinatorRole.waitFor();
    assert.match(await coordinatorRole.getAttribute('title'),/Инструменты$/);
    assert.equal(await card(page,'vps').getByText('сервер',{exact:true}).count(),1);
    assert.equal(await chat(page,'game').isVisible(),true);
    await cells(page,'local').nth(0).locator('.am-agent-parallel summary').waitFor();
    const roster=cell=>cell.locator('[data-provider]').evaluateAll(lines=>lines.map(line=>line.dataset.provider));
    for(const id of ['local','pc1','pc2','linux']){
      assert.deepEqual(await roster(cells(page,id).nth(0)),await roster(cells(page,id).nth(1)),id+' agent roster');
    }
    assert.equal(await cells(page,'linux').nth(0).getByText('Codex',{exact:true}).count(),1);
    assert.equal(await cells(page,'linux').nth(0).locator('.am-agent-run').count(),0);
    assert.equal(await cells(page,'linux').nth(1).locator('.am-agent-run[data-provider=codex]').count(),1);
    assert.equal(await card(page,'vps').getByText('Агенты не подключены',{exact:true}).count(),2);
    assert.equal(await page.locator('#machines').getByText('Нет в проекте',{exact:true}).count(),0);
    const splitSizes=await cardSizes();
    assert(splitSizes.every((size,i)=>size.width>singleSizes[i].width));
    assert.deepEqual(splitSizes.map(size=>size.height),singleSizes.map(size=>size.height),
      'switching to two project columns should preserve the height of equivalent cards');
    assert.equal(await page.locator('#machines .am-machine-card').count(),5);
    assert.equal(await cells(page,'local').count(),2);
    const [leftTile,rightTile]=await cells(page,'local').all();
    const [left,right]=await Promise.all([leftTile.boundingBox(),rightTile.boundingBox()]);
    const [firstMenu,secondMenu]=await Promise.all(['project-select','project-select-second']
      .map(id=>page.locator('button[aria-controls="'+id+'-menu"]').boundingBox()));
    const gap=right.x-left.x-left.width;
    assert(gap>0);
    assert(Math.abs(gap-(secondMenu.x-firstMenu.x-firstMenu.width))<0.1);
    assert.equal(left.height,right.height);
    assert(await cells(page,'local').evaluateAll(tiles=>tiles.every(tile=>parseFloat(getComputedStyle(tile).borderTopLeftRadius)>0)));
    assert.equal(await cells(page,'pc1').nth(0).getByText('ПК-1',{exact:true}).count(),1);
    assert.equal(await cells(page,'pc1').nth(0).locator('.am-machine-version').count(),0);
    assert.equal(await cells(page,'pc1').nth(1).getByText('v1.0.5',{exact:true}).count(),1);
    await cells(page,'pc1').nth(1).getByRole('button',{name:'Обновить до v1.0.9'}).click();
    await page.locator('#confirm-dialog').getByText('Обновить ПК-1 до v1.0.9?').waitFor();
    await page.locator('#confirm-cancel').click();
    assert.equal(await cells(page,'pc1').nth(1).getAttribute('aria-pressed'),'false');
    assert.equal(await chat(page,'game').isVisible(),true);
    await page.locator('#list-toggle').click();
    assert.equal(await page.locator('#task-panel').isVisible(),false);
    assert.equal(await chat(page,'game').isVisible(),true);
    await page.locator('#list-toggle').click();
    assert.equal(await page.locator('#task-panel').isVisible(),true);
    assert.equal(await page.locator('#machines').getByText('Параллельная задача',{exact:true}).count(),0);
    const closedCard=await card(page,'local').boundingBox();
    const otherAgent=cells(page,'local').nth(1).locator('.am-agent-line').first();
    const otherAgentTop=(await otherAgent.boundingBox()).y;
    await cells(page,'local').nth(0).locator('summary').click();
    assert.equal(await cells(page,'local').nth(0).locator('details[open]').count(),1);
    assert.equal(await cells(page,'local').nth(0).locator('.am-agent-model[title]').count(),0);
    assert((await card(page,'local').boundingBox()).height>closedCard.height);
    assert.equal((await otherAgent.boundingBox()).y,otherAgentTop);
    assert.equal(await cells(page,'local').nth(0).getAttribute('aria-pressed'),'false');
    if(output)await page.screenshot({path:path.join(output,'messenger-t316-expanded.png')});
    await cells(page,'local').nth(0).locator('summary').click();
    assert.equal(await chat(page,'game').isVisible(),true);
    await page.mouse.move(810,5);
    if(output)await page.screenshot({path:path.join(output,'messenger-t316-projects.png')});

    await page.locator('#answer-text').fill('Черновик в первом проекте');
    await page.locator('button[aria-controls=project-select-second-menu]').click();
    assert.equal(await page.locator('#project-select-second-menu').getByRole('option',{name:'Игра',exact:true}).count(),0);
    await page.locator('#project-select-second-menu').getByRole('option',{name:'Сайт',exact:true}).click();
    await cells(page,'local').nth(1).locator('.am-agent-run').waitFor();
    await coordinatorRole.waitFor({state:'detached'});
    assert.equal(await chat(page,'game').isVisible(),true);
    assert.equal(await page.locator('#answer-text').inputValue(),'Черновик в первом проекте');
    await page.locator('#answer-text').fill('');
    await select(page,'project-select-second','Инструменты',true);
    await cells(page,'local').nth(1).getByText('ждёт ответа').waitFor();
    await page.locator('[data-tab=stats]').click();
    await total(page,'3579');
    assert.equal(await page.locator('button[aria-controls=stats-project-menu]').innerText(),'Выбранные проекты');
    await page.locator('#stats-efficiency').click();
    await page.locator('#stats-efficiency-status').getByText('В этом периоде нет исходных задач для сравнения.').waitFor();
    await page.locator('#stats-efficiency').click();
    await page.mouse.move(810,5);
    if(output)await page.screenshot({path:path.join(output,'messenger-t316-statistics.png')});
    await page.locator('#stats-export').click();
    await page.locator('#stats-export-status').getByText(/Файл сохранён/).waitFor();
    const exportBody=JSON.parse(seen.findLast(r=>r.path==='/api/usage-export').body);
    assert.deepEqual(exportBody.projects,['game','tools']);
    assert.equal(exportBody.project,undefined);
    assert.deepEqual(seen.findLast(r=>r.path==='/api/usage-efficiency').params.getAll('projects'),['game','tools']);
    await select(page,'stats-project','Инструменты');
    await total(page,'2345');
    assert.equal(seen.findLast(r=>r.path==='/api/usage-stats').params.get('project'),'tools');
    await page.locator('[data-tab=tasks]').click();
    assert.equal(await chat(page,'game').isVisible(),true);
    await page.locator('[data-tab=stats]').click();
    await select(page,'stats-project','Выбранные проекты');
    await total(page,'3579');
    const statsTask=page.locator('#stats-top tr').filter({hasText:'Экспорт отчёта'});
    const taskButton=statsTask.locator('.am-stats-task-text');
    assert.equal(await taskButton.evaluate(button=>getComputedStyle(button.closest('.am-card')).padding),'12px');
    const dateCell=statsTask.locator('td').first();
    await dateCell.hover();
    assert.deepEqual(await taskButton.evaluate(button=>{
      const row=button.closest('tr'),highlight=getComputedStyle(row.cells[0],'::before');
      const title=button.firstElementChild.getBoundingClientRect();
      const header=button.closest('table').tHead.rows[0].cells[1];
      const titleColumnStart=header.getBoundingClientRect().left+parseFloat(getComputedStyle(header).paddingLeft);
      return [highlight.insetBlockStart,highlight.insetBlockEnd,highlight.insetInlineStart,highlight.insetInlineEnd,title.left-titleColumnStart];
    }),['4px','4px','0px','0px',0]);
    await dateCell.click();
    await chat(page,'tools').waitFor();
    await cells(page,'local').nth(0).press('Enter');
    await chat(page,'game').waitFor();
    assert.equal(await card(page,'local').getAttribute('aria-pressed'),null);
    assert.equal(await cells(page,'local').nth(0).getAttribute('aria-pressed'),'true');
    const localCard=card(page,'local');
    await page.mouse.move(810,5);
    const pcTiles=cells(page,'pc1');
    assert.deepEqual(await fills(pcTiles.nth(0)),await fills(pcTiles.nth(1)));
    const neutralFill=await fills(rightTile);
    const selectedFill=await fills(leftTile);
    assert.notDeepEqual(selectedFill,neutralFill);
    const clickHeading=async(id,right)=>{
      const heading=cells(page,id).nth(right?1:0).locator('.am-machine-name');
      const {width,height}=await heading.boundingBox();
      await heading.click({position:{x:width*.25,y:height/2}});
    };
    await clickHeading('local',true);
    await chat(page,'tools').waitFor();
    assert.equal(await cells(page,'local').nth(1).getAttribute('aria-pressed'),'true');
    await page.mouse.move(810,5);
    assert.deepEqual(await fills(leftTile),neutralFill);
    assert.deepEqual(await fills(rightTile),selectedFill);
    const cardBeforeHover=await localCard.boundingBox();
    for(const part of ['.am-machine-name','.am-machine-body']){
      await leftTile.locator(part).hover();
      const hoveredFill=await fills(leftTile);
      assert(hoveredFill.every((color,index)=>color!==neutralFill[index]));
      assert.deepEqual(await fills(rightTile),selectedFill);
      await rightTile.locator(part).hover();
      assert.deepEqual(await fills(leftTile),neutralFill);
      assert.deepEqual(await fills(rightTile),selectedFill);
    }
    assert.deepEqual(await localCard.boundingBox(),cardBeforeHover);
    const gapX=cardBeforeHover.x+cardBeforeHover.width/2;
    for(const inset of [2,cardBeforeHover.height/2,cardBeforeHover.height-2]){
      const y=cardBeforeHover.y+inset;
      assert.equal(await page.evaluate(([x,y])=>!!document.elementFromPoint(x,y)?.closest('.am-machine-project'),[gapX,y]),false);
      await page.mouse.click(gapX,y);
      assert.deepEqual(await fills(leftTile),neutralFill);
      assert.equal(await rightTile.getAttribute('aria-pressed'),'true');
    }
    if(output){
      await page.mouse.move(810,5);
      await page.screenshot({path:path.join(output,'messenger-t318-split.png')});
    }
    const selectedSizes=await cardSizes();
    assert.deepEqual(selectedSizes.map(size=>size.height),splitSizes.map(size=>size.height));
    await clickHeading('linux',true);
    await page.locator('#task-list').getByText('Проверка файлов',{exact:true}).waitFor();
    assert.equal(await cells(page,'linux').nth(1).getAttribute('aria-pressed'),'true');
    assert.equal(await cells(page,'local').nth(1).getAttribute('aria-pressed'),'false');
    assert(seen.some(r=>r.path==='/api/window'&&r.params.get('project')==='tools'&&r.params.get('computer')==='linux'));
    await clickHeading('local',false);
    await chat(page,'game').waitFor();
    await select(page,'project-select','Сайт');
    await chat(page,'site').waitFor();
    assert.equal(await page.locator('.am-machine-project[aria-pressed=true]').count(),0);
    await select(page,'project-select','Игра');
    await chat(page,'game').waitFor();
    assert.equal(await cells(page,'local').nth(0).getAttribute('aria-pressed'),'true');
    await split(page).click();
    assert.equal(await cells(page,'local').count(),0);
    assert.equal(await card(page,'local').getAttribute('aria-pressed'),'true');
    const selectedBody=await card(page,'local').evaluate(card=>getComputedStyle(card).backgroundColor);
    assert.notEqual(selectedBody,await card(page,'pc1').evaluate(card=>getComputedStyle(card).backgroundColor));
    assert.equal(await chat(page,'game').isVisible(),true);
    if(output){
      await page.mouse.move(810,5);
      await page.screenshot({path:path.join(output,'messenger-t318-single.png')});
    }
    await page.locator('[data-tab=stats]').click();
    await total(page,'1234');
    await split(page).click();
    await total(page,'3579');
    available.splice(1);
    await split(page).waitFor({state:'hidden'});
    await total(page,'1234');
    assert.equal(await page.locator('#stats-project-filter').isVisible(),false);
    await page.close();

    const statistics=await open(projects,{width:1197,height:706},'dark');
    await split(statistics).click();
    await statistics.locator('[data-tab=stats]').click();
    await total(statistics,'3579');
    const statisticsProject=statistics.locator('button[aria-controls=stats-project-menu]');
    const lastStatisticsQuery=()=>seen.findLast(r=>r.path==='/api/usage-stats').params;
    for(const [index,name,label,tokens] of [[0,'game','Игра','321'],[1,'tools','Инструменты','654'],[0,'game','Игра','321']]){
      await cells(statistics,'pc1').nth(index).locator('.am-machine-body').click();
      await total(statistics,tokens);
      assert.equal(await statisticsProject.innerText(),label);
      assert.equal(lastStatisticsQuery().get('project'),name);
      assert.equal(lastStatisticsQuery().get('computer'),'pc1');
      assert.equal(lastStatisticsQuery().has('projects'),false);
    }
    await cells(statistics,'pc1').nth(0).press('Enter');
    await total(statistics,'1234');
    assert.equal(await statisticsProject.innerText(),'Игра');
    assert.equal(lastStatisticsQuery().get('computer'),'');
    await select(statistics,'stats-project','Выбранные проекты');
    await total(statistics,'3579');
    await statistics.close();

    const narrow=await open(projects,{width:600,height:720},'light');
    await split(narrow).click();
    await cells(narrow,'local').nth(1).getByText('ждёт ответа').waitFor();
    assert.equal(await narrow.locator('#project-columns').evaluate(el=>el.scrollWidth>el.clientWidth),false);
    assert.equal(await narrow.locator('body').evaluate(el=>el.scrollWidth>innerWidth),false);
    assert.equal(await split(narrow).evaluate(el=>el.getBoundingClientRect().height),28);
    const divider=await cells(narrow,'local').nth(1).evaluate(el=>getComputedStyle(el,'::before').content);
    assert.equal(divider,'none');
    await narrow.mouse.move(590,5);
    assert.deepEqual(await fills(cells(narrow,'local').nth(0)),await fills(cells(narrow,'local').nth(1)));
    await narrow.locator('[data-tab=agents]').click();
    assert.equal(await narrow.locator('[data-screen=agents]').isVisible(),true);
    await narrow.locator('[data-tab=server]').click();
    assert.equal(await narrow.locator('[data-screen=server]').isVisible(),true);
    await narrow.close();

    let busy=true,outdated=true;
    const restartPage=await open(projects,{width:1197,height:706},'light',url=>{
      if(url.pathname==='/api/client-info')return {json:{client:'running',restart:true,outdated}};
      if(url.pathname==='/api/restart-client'){
        if(busy)return {status:409,json:{ok:false,state:'confirm',running:1,known:true,tasks:[]}};
        outdated=false;
        return {json:{ok:true}};
      }
    });
    const restartDialog=restartPage.locator('#restart-dialog');
    const waitStatus=restartPage.locator('#restart-wait-show');
    await restartPage.locator('#restart-go').click();
    await restartDialog.waitFor({state:'hidden'});
    await waitStatus.waitFor();
    await select(restartPage,'project-select','Инструменты');
    await chat(restartPage,'tools').waitFor();
    await waitStatus.click();
    await restartDialog.getByRole('button',{name:'Закрыть',exact:true}).click();
    await restartDialog.waitFor({state:'hidden'});
    assert.deepEqual(await restartPage.evaluate(()=>window.nativeAppMessages),[]);
    busy=false;
    await waitStatus.waitFor({state:'hidden',timeout:11000});
    const restartCalls=seen.filter(r=>r.path==='/api/restart-client').map(r=>JSON.parse(r.body));
    assert(restartCalls.length>=3&&restartCalls.every(call=>call.confirm===false));
    assert.equal(restartCalls.at(-1).project,'tools');
    assert.equal(await chat(restartPage,'tools').isVisible(),true);
    await restartPage.close();
    assert.deepEqual(errors,[]);
    console.log(engine+': sidebar and archive, direct tasks, drag and keyboard, project split, chat transitions, statistics, agents, restart and narrow layouts passed');
  }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
