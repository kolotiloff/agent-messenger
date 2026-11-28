// Verify task tabs and comparisons against a fictional client.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {projects, reply} = require('./check-window-project-split.cjs');
const assets = path.resolve(__dirname, '../internal/localui/assets');

(async () => {
  const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const engine = process.env.BROWSER_ENGINE || 'chromium';
  const browser = await playwright[engine].launch();
  try {
    const page = await browser.newPage({viewport:{width:1450,height:900},colorScheme:'dark'});
    page.setDefaultTimeout(6500);
    const output=process.env.SCREENSHOT_DIR;
    if(output)await fs.mkdir(output,{recursive:true});
    const reads = [], writes = [];
    let stoppedPrimary=false;
    await page.addInitScript(() => {
      const every = window.setInterval;
      window.setInterval = (callback, delay, ...args) => every(callback, delay===5000 ? 100 : delay, ...args);
    });
    await page.route('https://messenger-preview.test/**', async route => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.startsWith('/api/')) {
        if (url.pathname==='/api/task') reads.push({id:url.searchParams.get('id'),pane:url.searchParams.has('cursor')});
        if (request.method()==='POST') {
          writes.push({path:url.pathname, body:request.postDataJSON()});
          return route.fulfill({json:{ok:true}});
        }
        const json=reply(url,url.searchParams.get('project') || 'game',projects);
        if(url.pathname==='/api/window'&&stoppedPrimary)json.tasks=json.tasks.map(task=>task.id==='game-4'?{...task,state:'cancelled',can_continue:true}:task);
        if(url.pathname==='/api/task'){
          json.task={...json.task};
          delete json.task.project;
          json.items[1].text='Ответ по задаче '+json.task.id;
          if(json.task.id==='game-1'){
            json.task.state='interrupted';
            json.task.can_continue=true;
            json.task.remote={name:'Проверочный компьютер'};
          }
          if(json.task.id==='game-4'&&stoppedPrimary){
            json.task.state='cancelled';
            json.task.can_continue=true;
            json.items.push({kind:'outcome',state:'cancelled',at:new Date().toISOString(),text:'the task was cancelled; its report, if any, was not published'});
          }
          if(json.task.id==='game-2')json.task.owner_question={id:'fixture-question',text:'Подтвердите проверку'};
        }
        return route.fulfill({json});
      }
      if (url.pathname === '/') {
        const body = (await fs.readFile(path.join(assets,'index.html'),'utf8'))
          .replace('id="am-concept"','id="am-concept" class="am-task-sidebar-preview"');
        return route.fulfill({contentType:'text/html; charset=utf-8',body});
      }
      return route.fulfill({path:path.join(assets,url.pathname.slice(1))});
    });
    await page.goto('https://messenger-preview.test/#preview-only');
    await page.getByText(/Ответ по задаче game-/).waitFor();
    const mainTitle=page.locator('#task-head h3');
    const primary=page.locator('#answer-text');
    const initialTitle=await mainTitle.textContent();
    await primary.fill('Основной черновик');
    const open = async (title, command='Открыть во вкладке') => {
      const task=page.locator('#machines .am-sidebar-task').filter({hasText:title});
      await task.click({button:'right'});
      await page.getByRole('menuitem',{name:command,exact:true}).click();
    };
    const first='Экран настроек', second='Проверка навигации', third='Параллельная задача';
    const strip=page.getByRole('tablist',{name:'Вкладки задач'});
    const tabs=strip.getByRole('tab');
    const tab = title => strip.getByRole('tab',{name:title,exact:true});
    const panel = title => page.getByRole('tabpanel',{name:title,exact:true});
    const count = id => reads.filter(read=>read.id===id&&read.pane).length;
    await open(first);
    await panel(first).getByText('Ответ по задаче game-1').waitFor();
    assert.equal(await mainTitle.isVisible(),false);
    assert.equal(await strip.evaluate(node=>!!node.closest('.am-task-toolbar')),true);
    assert.equal(await primary.inputValue(),'Основной черновик');
    const draft=panel(first).getByRole('textbox',{name:'Ответ исполнителю'});
    assert.equal(await draft.getAttribute('placeholder'),'Продолжить в этой сессии…');
    await draft.fill('Отдельный черновик');
    await draft.press('Escape');
    assert.equal(await page.locator('dialog[open]').count(),0);
    assert.equal(writes.some(write=>write.path==='/api/task-stop'),false);
    await open(second);
    await panel(second).getByText('Ответ по задаче game-2').waitFor();
    assert.equal(await panel(first).count(),0);
    const hiddenReads=count('game-1'), visibleReads=count('game-2');
    await page.waitForTimeout(350);
    assert.equal(count('game-1'),hiddenReads);
    assert(count('game-2')>visibleReads);
    const ownerReply=panel(second).getByRole('textbox',{name:'Ответ исполнителю'});
    await ownerReply.fill('Подтверждаю');
    const ownerRequest=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/task-answer');
    await ownerReply.press('Enter');
    const answer=(await ownerRequest).postDataJSON();
    assert.equal(answer.task,'game-2');
    assert.equal(answer.question,'fixture-question');
    assert.equal(answer.text,'Подтверждаю');
    await tab(first).click();
    assert.equal(await draft.inputValue(),'Отдельный черновик');
    assert.equal(await primary.inputValue(),'Основной черновик');
    await panel(first).getByRole('button',{name:'Отправить ответ'}).click();
    assert.deepEqual(writes.filter(write=>write.path==='/api/task-note').map(write=>[
      write.path,write.body.task,write.body.text
    ]),[['/api/task-note','game-1','Отдельный черновик']]);
    assert.equal(await primary.inputValue(),'Основной черновик');
    await open(third,'Открыть рядом');
    await panel(third).getByText('Ответ по задаче game-4').waitFor();
    assert.equal(count('game-4'),0,'the pane for the current task should use the primary feed');
    await tab(first).click();
    await panel(first).waitFor();
    await panel(third).waitFor();
    const board=page.locator('.am-pane-board');
    for(const [label,layout] of [['Столбцы','columns'],['Строки','stack'],['Сетка','grid']]){
      await page.locator('.am-task-toolbar').getByRole('button',{name:label,exact:true}).click();
      assert.equal(await board.getAttribute('data-layout'),layout);
      assert.equal(await board.locator('.am-pane-grid').evaluate(grid=>getComputedStyle(grid).gap),'1px');
      for(const title of [first,third]) {
        assert.equal(await panel(title).evaluate(node=>getComputedStyle(node).borderWidth),'0px');
      }
    }
    await tab(first).focus();
    await tab(first).press('Alt+ArrowRight');
    assert.deepEqual(await tabs.allTextContents(),[third,second,first]);
    const source=await tab(first).boundingBox(),target=await tab(third).boundingBox();
    const fromX=source.x+source.width/2,fromY=source.y+source.height/2;
    const toX=target.x+target.width/2,toY=target.y+target.height/2;
    await page.mouse.move(fromX,fromY);
    await page.mouse.down();
    await page.mouse.move(fromX-12,fromY,{steps:4});
    await page.mouse.move(toX,toY,{steps:6});
    await page.mouse.move(toX+1,toY);
    await page.mouse.up();
    assert.deepEqual(await tabs.allTextContents(),[first,third,second]);
    const dragTask=page.locator('#machines .am-sidebar-task').filter({hasText:second});
    const start=await dragTask.boundingBox();
    await page.mouse.move(start.x+start.width/2,start.y+start.height/2);
    await page.mouse.down();
    await page.mouse.move(start.x+start.width/2+16,start.y+start.height/2,{steps:5});
    const drop=page.locator('[data-drop-layout=stack]');
    await drop.waitFor();
    const toolbarBounds=await page.locator('.am-task-toolbar').boundingBox();
    assert.equal(Math.round((await drop.boundingBox()).y-toolbarBounds.y-toolbarBounds.height),12);
    if(output)await page.screenshot({path:path.join(output,'task-drop-layouts.png')});
    const end=await drop.boundingBox();
    await page.mouse.move(end.x+end.width/2,end.y+end.height/2,{steps:8});
    await page.mouse.move(end.x+end.width/2+1,end.y+end.height/2);
    await page.mouse.up();
    await drop.waitFor({state:'hidden'});
    assert.equal(await board.getAttribute('data-layout'),'stack');
    assert.equal(await tab(second).getAttribute('aria-selected'),'true');
    assert.equal(await dragTask.getAttribute('aria-pressed'),'true');
    const upper=await panel(first).boundingBox(), lower=await panel(third).boundingBox();
    assert(lower.y>=upper.y+upper.height,'rows display distinct conversations vertically');
    await page.locator('.am-task-toolbar').getByRole('button',{name:'Сетка',exact:true}).click();
    await tab(first).click();
    if(output){
      await page.mouse.move(0,0);
      await page.screenshot({path:path.join(output,'task-grid.png')});
    }
    await page.setViewportSize({width:760,height:850});
    const panes=page.locator('.am-pane-grid');
    const narrow=await panes.evaluate(grid=>getComputedStyle(grid).gridTemplateColumns.split(' ').length);
    assert.equal(narrow,1);
    await page.setViewportSize({width:1450,height:900});
    await page.evaluate(()=>{
      Object.defineProperty(document,'hidden',{configurable:true,value:true});
      document.dispatchEvent(new Event('visibilitychange'));
    });
    const whileHidden=count('game-1');
    await page.waitForTimeout(350);
    assert.equal(count('game-1'),whileHidden);
    await page.evaluate(()=>{
      delete document.hidden;
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await page.locator('#stats-open').click();
    const before=count('game-1')+count('game-2')+count('game-3');
    await page.waitForTimeout(350);
    assert.equal(count('game-1')+count('game-2')+count('game-3'),before);
    await page.reload();
    await tabs.first().waitFor();
    assert.deepEqual(await tabs.allTextContents(),[first,third,second]);
    assert.equal(await tab(first).getAttribute('aria-selected'),'true');
    await panel(first).waitFor();
    await panel(third).waitFor();
    for(const title of [first,second,third]){
      await tab(title).focus();
      await tab(title).press('Delete');
    }
    assert.equal(await board.count(),0);
    assert.equal(await mainTitle.textContent(),initialTitle);
    assert.equal(await primary.evaluate(input=>input===document.activeElement),true);
    stoppedPrimary=true;
    const stopped=page.locator('#thread .am-system-event');
    await stopped.getByText('Работа остановлена',{exact:true}).waitFor();
    assert(!await page.getByText(/its report, if any/).count());
    assert.equal(await primary.getAttribute('placeholder'),'Продолжить в этой сессии…');
    assert(await primary.isEnabled());
    if(output)await page.locator('.am-chat-body').screenshot({path:path.join(output,'stopped-conversation.png')});
    await primary.fill('Продолжи проверку');
    const followup=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/task-note');
    await page.locator('#answer-send').click();
    const continuation=(await followup).postDataJSON();
    assert.equal(continuation.task,'game-4');
    assert.equal(continuation.text,'Продолжи проверку');
    console.log(JSON.stringify({engine,result:'passed'}));
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error);process.exitCode=1;});
