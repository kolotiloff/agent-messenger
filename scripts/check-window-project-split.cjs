// Exercise the real project controls against fictional window and statistics data.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_ENGINE, SCREENSHOT_DIR.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
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

const counts = {game:1234,tools:2345,site:99999};
const agents = [{agent:'codex'},{agent:'claude'}];
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
  if(url.pathname==='/api/usage-stats'){
    const names=url.searchParams.getAll('projects'), scope=names.length?names:[p];
    const usage=(tasks,tokens)=>({tasks,tokens,input_tokens:tokens,output_tokens:0,cached_tokens:0});
    const total=usage(scope.length,scope.reduce((sum,name)=>sum+counts[name],0));
    return {
      ...(names.length?{projects:names}:{project:p}), total,
      agents:scope.map(name=>({key:tasks[name][0].agent,...usage(1,counts[name])})),
      computers:[{key:'local',name:'Мак',this:true,...total}], models:[],
      days:[{day:iso(1440).slice(0,10),...usage(0,0)},{day:iso(1).slice(0,10),...total}],
      recent:scope.map(name=>({...tasks[name][0],...usage(1,counts[name]),at:iso(1)}))
    };
  }
  if(url.pathname==='/api/usage-efficiency'){
    const names=url.searchParams.getAll('projects');
    return {...(names.length?{projects:names}:{project:p}),groups:[],total_groups:0};
  }
  if(url.pathname==='/api/usage-export')return {ok:true,result:{path:'/fixtures/statistics.json',tasks:2}};
  return {};
}
(async()=>{
  const engine=process.env.BROWSER_ENGINE||'chromium';
  const browser=await playwright[engine].launch();
  const errors=[],seen=[];
  async function open(available,viewport,colorScheme,beforeReply=()=>{}){
    const context=await browser.newContext({viewport,deviceScaleFactor:1,colorScheme,locale:'ru-RU'});
    await context.addInitScript(scheme=>{
      window.amDesktop='mac';window.messengerInitialTheme=scheme;
      window.nativeAppMessages=[];
      window.webkit={messageHandlers:{app:{postMessage:message=>window.nativeAppMessages.push(message)}}};
    },colorScheme);
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
      return file?route.fulfill({contentType:file[1],body:await fs.readFile(path.join(assets,file[0]))}):route.fulfill({status:404,body:''});
    });
    await page.goto('http://messenger-preview.test/#preview-only');
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
    const single=await open(projects.slice(0,1),{width:1100,height:706},'light');
    assert.equal(await split(single).isVisible(),false);
    assert.equal(await single.locator('#project-column-second').isVisible(),false);
    assert.equal(await single.locator('.am-machine-project').count(),0);
    await single.locator('[data-tab=stats]').click();
    await total(single,'1234');
    assert.equal(await single.locator('#stats-project-filter').isVisible(),false);
    await single.close();

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
    console.log(engine+': project split, active chat, agent groups, project stats/filter/efficiency/export, keyboard and narrow layout passed');
  }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
