// Exercise the real project controls against fictional window and statistics data.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_ENGINE, SCREENSHOT_DIR.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const output = process.env.SCREENSHOT_DIR;
const assets = path.resolve(__dirname, '../internal/localui/assets');
const projects = [{name:'game',title:'Игра',folder:'/Проекты/Игра'},{name:'tools',title:'Инструменты',folder:'/Проекты/Инструменты'},{name:'site',title:'Сайт',folder:'/Проекты/Сайт'}];
const iso = minutes => new Date(Date.now() - minutes * 60000).toISOString();
const makeTask = (id, project, agent, title, minutes, extra = {}) => ({
  id,project,agent,title,state:'running',created_at:iso(minutes),started_at:iso(minutes),
  folder:projects.find(p => p.name===project).folder,worker_observed:true,
  ran_model:agent==='codex'?'gpt-6.2-sol[effort=medium]':'claude-opus-4-8',...extra
});
const pc1 = {id:'pc1',name:'ПК-1'}, pc2 = {id:'pc2',name:'ПК-2'}, linux = {id:'linux',name:'Ноут-Linux'};
const tasks = {
  game:[makeTask('game-1','game','codex','Экран настроек',9), makeTask('game-2','game','codex','Проверка навигации',15,{state:'waiting_owner',remote:pc1}), makeTask('game-4','game','codex','Параллельная задача',8), makeTask('game-3','game','codex','Звуки меню',37,{state:'done',finished_at:iso(25),remote:pc2})],
  site:[makeTask('site-1','site','codex','Подготовка сайта',7)],
  tools:[makeTask('tools-1','tools','claude','Экспорт отчёта',21,{state:'waiting_owner'}),makeTask('tools-2','tools','codex','Проверка файлов',3,{remote:linux})]
};
const toolDone=makeTask('tools-done','tools','codex','Готовый отчёт',90,{state:'done',finished_at:iso(60)});
const toolOlder=makeTask('tools-old','tools','codex','Старый отчёт',180,{state:'done',finished_at:iso(150)});
const toolShared={peer:'pc1',name:pc1.name,age:0,tasks:[{...toolDone,id:'tools-shared',title:'Отчёт другого компьютера',report:'Общий результат'}]};

const counts = {game:1234,tools:2345,site:99999};
const pc1Counts = {game:321,tools:654,site:0};
const agents = [{agent:'codex',models:['gpt-6.2-sol[effort=medium]','gpt-6.2-astra[effort=high]']},{agent:'claude',models:['claude-opus-4-8']}];
const coordinator = p=>({connected:true,active:p==='tools',client:'codex'});
function reply(url,p,available){
  if(url.pathname==='/api/window')return {project:p,projects:available,computer_name:'Мак',agents,tasks:tasks[p],shared:[],tasks_enabled:true,history_paged:true,coordinator:coordinator(p)};
  if(url.pathname==='/api/project-activity')return {project:p,tasks:tasks[p],shared:[],coordinator:coordinator(p)};
  if(url.pathname==='/api/task'){
    const task=Object.values(tasks).flat().find(t=>t.id===url.searchParams.get('id'));
    return {task,items:[{kind:'text',author:task.agent,at:iso(2),text:'Сообщение из '+task.project}]};
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
      window.amDesktop=desktop;window.messengerInitialTheme=scheme;
      if(desktop==='windows'){window.amFrameless=true;window.amWindow=()=>{};}
      window.nativeAppMessages=[];
      window.webkit={messageHandlers:{app:{postMessage:message=>window.nativeAppMessages.push(message)}}};
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
    const preview=await open(projects,{width:1100,height:706},'dark',(url,p)=>{
      if(url.pathname==='/api/machine-order')return {json:{ok:true}};
      if(url.pathname==='/api/window'&&p==='tools')return otherWindow.response;
      if(url.pathname!=='/api/task')return;
      if(url.searchParams.get('id')==='game-1')return sameFeed.response;
      if(url.searchParams.get('id')==='tools-1')return otherFeed.response;
    },true);
    const header=preview.locator('#task-head time'),thread=preview.locator('#thread');
    const oldDate=await header.getAttribute('datetime');
    const sameDate=tasks.game.find(t=>t.id==='game-1').created_at;
    const otherDate=tasks.tools.find(t=>t.id==='tools-1').created_at;
    assert.equal(await preview.locator('#task-head > :not(time)').count(),0);
    const localHeader=card(preview,'local').locator('.am-sidebar-disclosure[draggable=true]');
    const peerHeader=card(preview,'pc1').locator('.am-sidebar-disclosure[draggable=true]');
    const order=()=>preview.locator('#machines > .am-machine-card').evaluateAll(cards=>cards.map(card=>card.dataset.machineOrder));
    const padding=target=>target.evaluate(node=>[getComputedStyle(node).paddingBlockStart,getComputedStyle(node).paddingBlockEnd]);
    const sidebarTask=title=>preview.locator('#machines .am-sidebar-task').filter({hasText:title});
    const expectPrevious=async(date)=>{
      assert.equal(await header.getAttribute('datetime'),date);
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
      sameFeed.release();
      await preview.locator('#task-head time[datetime="'+sameDate+'"]').waitFor();
      await sidebarTask('Экспорт отчёта').click();
      await expectPrevious(sameDate);
      const requestedFeed=preview.waitForRequest(req=>new URL(req.url()).searchParams.get('id')==='tools-1');
      otherWindow.release();
      await requestedFeed;
      await expectPrevious(sameDate);
      otherFeed.release();
      await chat(preview,'tools').waitFor();
      assert.equal(await header.getAttribute('datetime'),otherDate);
      assert.equal(await thread.getAttribute('aria-busy'),'false');
      assert.equal(await preview.locator('#compose').evaluate(node=>node.inert),false);
    }finally{sameFeed.release();otherWindow.release();otherFeed.release();await preview.close();}

    const directTask=makeTask('owner-task','tools','codex','Новая задача в окне',0,{remote:pc1});
    const queued=makeTask('queued-task','tools','codex','Следующая проверка',1,{started_at:null,pending_launch:true,worker_observed:false,remote:pc1});
    const draftPage=await open(projects,{width:1150,height:770},'dark',(url,p)=>{
      if(p==='tools'&&['/api/window','/api/project-activity'].includes(url.pathname)){
        const showCodex=!url.searchParams.get('agent')||url.searchParams.get('agent')==='codex';
        return {json:{...reply(url,p,projects),tasks:[...tasks.tools,queued,...(showCodex?[toolDone]:[])],shared:[toolShared],history_paged:true,history_next:showCodex?toolDone.id:''}};
      }
      if(p==='tools'&&url.pathname==='/api/task-history')return {json:{tasks:[toolOlder]}};
      if(url.pathname==='/api/self-update')return {json:{installed_version:'1.0.9',version:'1.0.10',state:'available'}};
      if(url.pathname==='/api/task-start')return {json:{ok:true,result:directTask.id}};
      if(url.pathname==='/api/task'&&url.searchParams.get('id')===directTask.id){
        return {json:{task:directTask,items:[{kind:'task',author:'owner',text:directTask.title,at:directTask.created_at}]}};
      }
    },true);
    try{
      const plus=draftPage.locator('#task-new');
      await plus.waitFor();
      assert.equal(await plus.evaluate(node=>getComputedStyle(node).opacity),'1');
      const local=draftPage.locator('.am-machine-card[aria-current=true]');
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
      assert.equal(await groupGap(),4);
      await disclosure.click();
      assert.equal(await disclosure.getAttribute('aria-expanded'),'true');
      assert.equal(await selected.evaluate(node=>getComputedStyle(node).backgroundColor),selectedFill);
      assert.equal(await groupGap(),4);
      await draftPage.mouse.move(1000,30);
      const nav=draftPage.locator('.am-tabs');
      assert.equal(await draftPage.locator('.am-project #task-new').count(),1);
      assert.equal(await draftPage.getByRole('button',{name:'Новая задача',exact:true}).count(),1);
      const picker=await draftPage.locator('#project-column-first .am-select-trigger').boundingBox(),plusBox=await plus.boundingBox();
      assert(plusBox.x>=picker.x+picker.width);
      assert.equal(plusBox.y+plusBox.height/2,picker.y+picker.height/2);
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
      const remote=card(draftPage,'pc1').locator('.am-machine-name');
      const metadata=remote.locator('.am-machine-meta');
      const opacity=()=>metadata.evaluate(node=>getComputedStyle(node).opacity);
      const versionSize=node=>getComputedStyle(node).fontSize;
      assert.equal(await footer.locator('.am-version').evaluate(versionSize),await metadata.locator('.am-version').evaluate(versionSize));
      assert.equal(await opacity(),'0');
      assert(await remote.evaluate(node=>getComputedStyle(node).backgroundColor===getComputedStyle(node.closest('.am-sidebar')).backgroundColor));
      await remote.hover();
      assert.equal(await opacity(),'1');
      assert.notEqual(await remote.evaluate(node=>getComputedStyle(node).backgroundColor),await remote.evaluate(node=>getComputedStyle(node.closest('.am-sidebar')).backgroundColor));
      await remote.locator('.am-sidebar-disclosure').click();
      await draftPage.locator('#settings-open').hover();
      assert.equal(await opacity(),'0');
      await draftPage.keyboard.press('Tab');
      const remoteUpdate=remote.getByRole('button',{name:'Обновить до v1.0.10'});
      await remoteUpdate.focus();
      assert.equal(await opacity(),'1');
      assert.deepEqual(await buttonShape(remoteUpdate),['28px','28px','8px']);
      const remoteDownloadBox=await remoteUpdate.boundingBox(),remoteVersionBox=await metadata.locator('.am-version').boundingBox();
      assert.equal(remoteVersionBox.x-remoteDownloadBox.x-remoteDownloadBox.width,4);
      const remoteGlyph=await remoteUpdate.locator('svg').first().boundingBox();
      assert.deepEqual([remoteGlyph.width,remoteGlyph.height],[18,18]);
      assert.equal(await remote.evaluate(node=>node.getBoundingClientRect().height),29);
      await remoteUpdate.hover();
      const remoteFill=await remoteUpdate.evaluate(async node=>{
        await Promise.all(node.getAnimations().map(animation=>animation.finished));
        return getComputedStyle(node).backgroundColor;
      });
      assert.equal(remoteFill,hoverFills[0]);
      assert.match(await draftPage.locator('#thread .am-meta-author').first().textContent(),/, исполнитель$/);
      assert.equal(await draftPage.locator('#thread .am-role').count(),0);
      for(const panel of [local.locator('.am-machine-label'),draftPage.locator('#thread .am-meta-author').first()]){
        const [base,role]=await panel.evaluate(node=>{
          const typography=element=>{
            const style=getComputedStyle(element);
            return [style.fontFamily,style.fontSize,style.fontWeight,style.color];
          };
          return [typography(node),typography(node.querySelector('.am-inline-role'))];
        });
        assert.deepEqual(base.slice(0,3),role.slice(0,3));
        assert.notEqual(base[3],role[3]);
      }
      const settingsDialog=draftPage.getByRole('dialog',{name:'Настройки программы'});
      const settingsOpen=draftPage.locator('#settings-open');
      const settingsNav=settingsDialog.getByRole('navigation',{name:'Разделы'});
      await settingsOpen.click();
      await settingsDialog.locator('#settings-page').waitFor();
      assert.equal(await settingsDialog.evaluate(node=>node.matches(':modal')),true);
      assert.equal(await settingsDialog.locator('.am-sidebar,#project-select').count(),0);
      assert.equal(await draftPage.locator('[data-screen=tasks]').isVisible(),true);
      assert((await settingsDialog.boundingBox()).width>600);
      for(const section of ['agents','project','server','stats']){
        await settingsNav.locator('[data-tab='+section+']').click();
        await settingsDialog.locator('[data-screen='+section+']').waitFor();
      }
      await total(draftPage,'103578');
      await select(draftPage,'stats-project','Инструменты');
      await select(draftPage,'stats-computer','ПК-1');
      await total(draftPage,'654');
      await draftPage.keyboard.press('Escape');
      await settingsDialog.waitFor({state:'hidden'});
      await draftPage.waitForFunction(()=>document.activeElement?.id==='settings-open');
      await select(draftPage,'project-select','Игра');
      await settingsOpen.click();
      await settingsNav.locator('[data-tab=stats]').click();
      assert.equal(await draftPage.locator('#stats-project').inputValue(),'tools');
      assert.equal(await draftPage.locator('#stats-computer').inputValue(),'pc1');
      await settingsNav.locator('[data-tab=agents]').click();
      assert.equal(await draftPage.locator('#agents-computer').inputValue(),'local');
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
      assert.equal(await draftPage.locator('#task-head').evaluate(head=>head.childElementCount),0);
      assert.equal(await form.locator('.am-compose-field').evaluate(node=>getComputedStyle(node).borderRadius),'8px');
      assert.equal(await draftPage.locator('#compose .am-compose-field').evaluate(node=>getComputedStyle(node).borderRadius),'8px');
      await card(draftPage,'linux').locator('.am-sidebar-task').getByText('Проверка файлов',{exact:true}).click();
      await form.waitFor({state:'hidden'});
      await chat(draftPage,'tools').waitFor();
      assert.equal(await plus.getAttribute('aria-expanded'),'false');
      await select(draftPage,'project-select','Все');
      await plus.click();
      const modelTrigger=form.getByRole('button',{name:'Модель',exact:true});
      assert.equal(await modelTrigger.evaluate(node=>{
        const option=node.getBoundingClientRect(),footer=node.parentElement.getBoundingClientRect();
        return footer.right-option.right;
      }),0);
      assert.equal(await draftPage.locator('#task-draft-send').isDisabled(),true);
      await select(draftPage,'task-draft-project','Инструменты');
      await select(draftPage,'task-draft-computer','ПК-1');
      await select(draftPage,'task-draft-model','Codex · Astra 6.2 High');
      await form.locator('textarea').fill(directTask.title);
      const start=draftPage.waitForRequest(req=>new URL(req.url()).pathname==='/api/task-start');
      await form.locator('textarea').press('Enter');
      const {op_id,...payload}=(await start).postDataJSON();
      assert.match(op_id,/^window-/);
      assert.deepEqual(payload,{project:'tools',computer:'pc1',agent:'codex',model:'gpt-6.2-astra[effort=high]',text:directTask.title});
      await draftPage.locator('#thread').getByText('Вы',{exact:true}).waitFor();
      assert.equal(await form.isVisible(),false);
      const drawer=draftPage.locator('#list-toggle');
      assert.equal(await drawer.getAttribute('aria-label'),'Показать архив');
      await drawer.click();
      assert.equal(await drawer.getAttribute('aria-label'),'Скрыть архив');
      const archive=draftPage.getByRole('complementary',{name:'Архив'});
      const archiveGeometry=()=>archive.evaluate(panel=>{
        const rect=selector=>document.querySelector(selector).getBoundingClientRect();
        const main=rect('.am-main'),aside=rect('.am-sidebar'),bounds=panel.getBoundingClientRect();
        const header=rect('.am-queue-header'),filter=rect('#agent-filter + .am-select-trigger');
        const projects=rect('.am-project'),heading=rect('#task-list-heading');
        return {
          matchesWorkspace:[bounds.top-main.top,main.bottom-bounds.bottom,main.right-bounds.right],
          headerLine:[header.left-bounds.left-panel.clientLeft,bounds.right-header.right],
          fields:[heading.left-bounds.left-panel.clientLeft,bounds.right-filter.right,projects.left-aside.left,aside.right-projects.right-1],
          controls:[filter.height,rect('#project-select + .am-select-trigger').height],
          rowAlignment:[filter.top-rect('#project-select + .am-select-trigger').top,
            header.bottom-rect('.am-computers').top-1],
        };
      });
      assert.deepEqual(await archiveGeometry(),{
        matchesWorkspace:[0,0,0],headerLine:[0,0],fields:[12,12,12,12],controls:[28,28],rowAlignment:[0,0],
      });
      assert.equal(await archive.locator('#task-list-heading').textContent(),'Архив');
      await archive.getByText('Готовый отчёт',{exact:true}).waitFor();
      assert.deepEqual((await archive.locator('.am-task-title').allTextContents()).sort(),['Готовый отчёт','Звуки меню','Отчёт другого компьютера']);
      assert.equal(await archive.locator('#task-count').textContent(),'3');
      await card(draftPage,'pc1').getByText('Ждёт запуска',{exact:false}).waitFor();
      await archive.getByRole('button',{name:'Показать ещё 10'}).click();
      await archive.getByText('Старый отчёт',{exact:true}).waitFor();
      await select(draftPage,'agent-filter','Claude Code');
      await archive.getByText('В архиве пока нет задач').waitFor();
      assert.equal(await card(draftPage,'pc1').getByText('Ждёт запуска',{exact:false}).count(),1);
      await select(draftPage,'agent-filter','Все агенты');
      await archive.getByText('Звуки меню',{exact:true}).waitFor();
      const row=draftPage.locator('#task-list .am-task').first();
      await row.waitFor();
      const lineGap=target=>target.evaluate(node=>{
        const title=node.querySelector('.am-task-title,.am-sidebar-task-title');
        const meta=title.nextElementSibling;
        return [meta.getBoundingClientRect().top-title.getBoundingClientRect().bottom,getComputedStyle(meta).lineHeight];
      });
      assert.deepEqual(await lineGap(row),[2,'18px']);
      assert.deepEqual(await lineGap(local.locator('.am-sidebar-task').first()),[2,'18px']);
      assert.equal(await draftPage.locator('.am-task-project > h3').first().evaluate(node=>
        node.nextElementSibling.getBoundingClientRect().top-node.getBoundingClientRect().bottom),8);
      const insets=()=>row.evaluate(node=>{
        const outer=node.getBoundingClientRect(),title=node.querySelector('.am-task-title').getBoundingClientRect();
        const date=node.querySelector('.am-task-when').getBoundingClientRect();
        return [title.left-outer.left,outer.right-date.right,outer.width];
      });
      const [left,right,width]=await insets();
      assert.deepEqual([left,right],[12,12]);
      await draftPage.locator('#task-panel > .am-splitter').press('Shift+ArrowLeft');
      const [wideLeft,wideRight,wideWidth]=await insets();
      assert.deepEqual([wideLeft,wideRight],[12,12]);
      assert(wideWidth>width);
      await archive.getByText('Отчёт другого компьютера',{exact:true}).click();
      await draftPage.locator('#thread').getByText('Общий результат',{exact:true}).waitFor();
    }finally{await draftPage.close();}

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

    const compactPreview=await open(projects,{width:450,height:720},'light',()=>{},true);
    try{
      const plus=compactPreview.getByRole('button',{name:'Новая задача',exact:true});
      await plus.waitFor();
      assert.equal(await compactPreview.locator('.am-project').evaluate(node=>node.scrollWidth>node.clientWidth),false);
      assert.equal(await compactPreview.locator('body').evaluate(node=>node.scrollWidth>innerWidth),false);
      await plus.click();
      await compactPreview.locator('#task-draft-text').waitFor();
      await compactPreview.locator('#task-draft-text').fill('Черновик');
      await compactPreview.locator('#settings-open').click();
      const modal=compactPreview.locator('#settings-dialog');
      await modal.waitFor();
      assert.equal(await modal.evaluate(node=>node.scrollWidth>node.clientWidth),false);
      await modal.locator('[data-tab=stats]').click();
      await total(compactPreview,'103578');
      await modal.locator('#settings-close').click();
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
    await page.locator('#stats-top tr').filter({hasText:'Экспорт отчёта'}).click();
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
