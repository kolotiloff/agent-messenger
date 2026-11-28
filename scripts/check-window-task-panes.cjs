// Verify task tabs and comparisons against a fictional client.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {projects, reply} = require('./check-window-project-split.cjs');
const assets = path.resolve(__dirname, '../internal/localui/assets');

(async () => {
  const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const engine = process.env.BROWSER_ENGINE || 'chromium';
  const macTitlebar = process.env.MAC_TITLEBAR === '1';
  const browser = await playwright[engine].launch();
  try {
    const page = await browser.newPage({viewport:{width:1450,height:900},colorScheme:'dark'});
    page.setDefaultTimeout(6500);
    const output=process.env.SCREENSHOT_DIR;
    if(output)await fs.mkdir(output,{recursive:true});
    const reads = [], writes = [], errors=[];
    page.on('pageerror',error=>errors.push(error.message));
    let stoppedPrimary=false;
    await page.addInitScript(macTitlebar => {
      if(window===window.top){
        window.amMacTitlebar=macTitlebar;
        window.dictationCommands=[];
        window.webkit={messageHandlers:{
          composerDictation:{postMessage:message=>window.dictationCommands.push(message)},
          titlebar:{postMessage:message=>{window.titlebarAreas=message;}},
        }};
      }
      const every = window.setInterval;
      window.setInterval = (callback, delay, ...args) => every(callback, delay===2000 ? 200 : delay, ...args);
    },macTitlebar);
    await page.route('https://messenger-preview.test/**', async route => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.startsWith('/api/')) {
        if (url.pathname==='/api/task') reads.push({id:url.searchParams.get('id'),pane:request.frame().parentFrame()!=null});
        if (request.method()==='POST') {
          writes.push({path:url.pathname, body:request.postDataJSON()});
          return route.fulfill({json:{ok:true}});
        }
        const json=reply(url,url.searchParams.get('project') || 'game',projects);
        if(url.pathname==='/api/window')json.tasks=json.tasks.map(task=>{
          if(request.frame().parentFrame()&&task.id==='game-1')return {...task,state:'interrupted',can_continue:true,remote:{name:'Проверочный компьютер'}};
          if(request.frame().parentFrame()&&task.id==='game-2')return {...task,owner_question:{id:'fixture-question',text:'Подтвердите проверку'}};
          if(task.id==='game-4'&&stoppedPrimary)return {...task,state:'cancelled',can_continue:true};
          return task;
        });
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
      if (url.pathname === '/' || url.pathname === '/task-pane') {
        const body = (await fs.readFile(path.join(assets,'index.html'),'utf8'))
          .replace('id="am-concept"','id="am-concept" class="am-task-sidebar-preview"');
        const ancestors=url.pathname==='/'?"'none'":"'self'";
        const frameSource=url.pathname==='/'?"; frame-src 'self'":"";
        return route.fulfill({contentType:'text/html; charset=utf-8',body,headers:{
          'content-security-policy':"default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; font-src 'self'; img-src 'self' blob:; frame-ancestors "+ancestors+frameSource
        }});
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
    const strip=page.locator('.am-pane-strip');
    const tabs=page.locator('.am-pane-tab').getByRole('tab');
    const tab = title => page.getByRole('tab',{name:title,exact:true});
    const panel = title => page.getByRole('tabpanel',{name:title,exact:true});
    const content = title => title===third?panel(title):panel(title).frameLocator('iframe');
    const replyBox = title => content(title).getByRole('textbox',{name:'Ответ исполнителю'});
    const count = id => reads.filter(read=>read.id===id&&read.pane).length;
    await open(first);
    await content(first).getByText('Ответ по задаче game-1').waitFor();
    assert.equal(await mainTitle.isVisible(),false);
    assert.equal(await strip.evaluate(node=>!!node.closest('.am-task-toolbar')),true);
    assert.equal(await primary.inputValue(),'Основной черновик');
    const draft=replyBox(first);
    assert.equal(await draft.getAttribute('placeholder'),'Продолжить в этой сессии…');
    await draft.fill('Отдельный черновик');
    await draft.press('Escape');
    assert.equal(await page.locator('dialog[open]').count(),0);
    assert.equal(writes.some(write=>write.path==='/api/task-stop'),false);
    await open(second);
    await content(second).getByText('Ответ по задаче game-2').waitFor();
    assert.equal(await panel(first).count(),0);
    const hiddenReads=count('game-1'), visibleReads=count('game-2');
    await page.waitForTimeout(350);
    assert.equal(count('game-1'),hiddenReads);
    assert(count('game-2')>visibleReads);
    const ownerReply=replyBox(second);
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
    await content(first).getByRole('button',{name:'Отправить ответ'}).click();
    assert.deepEqual(writes.filter(write=>write.path==='/api/task-note').map(write=>[
      write.path,write.body.task,write.body.text
    ]),[['/api/task-note','game-1','Отдельный черновик']]);
    assert.equal(await primary.inputValue(),'Основной черновик');
    await tab(first).focus();
    await tab(first).press('Alt+ArrowRight');
    assert.deepEqual(await strip.getByRole('tab').allTextContents(),[third,second,first]);
    async function startDrag(locator){
      const rect=await locator.boundingBox(),x=rect.x+rect.width/2,y=rect.y+rect.height/2;
      await page.mouse.move(x,y);
      await page.mouse.down();
      await page.mouse.move(x+16,y,{steps:5});
    }
    async function dragTo(x,y){
      await page.mouse.move(x,y,{steps:8});
      await page.mouse.move(x+1,y);
    }
    const board=page.locator('.am-pane-board');
    const cell=title=>panel(title).locator('xpath=ancestor::*[contains(@class,"am-pane-cell")]');
    const preview=page.locator('.am-pane-preview');
    await tab(third).click();
    const original=await cell(third).boundingBox();
    await startDrag(tab(first));
    await dragTo(original.x+original.width/2,original.y+original.height/2);
    assert(!await preview.isVisible(),'center offers no split');
    await dragTo(original.x+original.width/2,original.y+original.height-12);
    await preview.waitFor();
    assert(Math.abs((await preview.boundingBox()).height-original.height/2)<2);
    assert.equal(await page.locator('.am-pane-targets,.am-pane-layouts').count(),0);
    await page.mouse.up();
    await preview.waitFor({state:'hidden'});
    const top=await cell(third).boundingBox(),bottom=await cell(first).boundingBox();
    assert(bottom.y>=top.y+top.height&&bottom.width===top.width);
    assert.equal(count('game-4'),0,'the originally selected chat retains the primary document');
    await replyBox(first).fill('Черновик в нижней панели');
    const sidebarTask=page.locator('#machines .am-sidebar-task').filter({hasText:second});
    await startDrag(sidebarTask);
    await dragTo(top.x+top.width-12,top.y+top.height/2);
    await preview.waitFor();
    const highlight=await preview.boundingBox();
    assert(Math.abs(highlight.width-top.width/2)<2&&Math.abs(highlight.height-top.height)<2);
    if(output)await page.screenshot({path:path.join(output,'task-split-preview.png')});
    await page.mouse.up();
    await preview.waitFor({state:'hidden'});
    await content(second).getByText('Ответ по задаче game-2').waitFor();
    assert.equal(await replyBox(first).inputValue(),'Черновик в нижней панели');
    assert.equal(await primary.inputValue(),'Основной черновик');
    assert.equal(await strip.isVisible(),false);
    assert.equal(await sidebarTask.getAttribute('aria-pressed'),'true');
    async function verifySplit(){
      const positions=[];
      for(const title of [third,second,first]){
        const header=tab(title).locator('xpath=ancestor::*[@role="tablist"]');
        assert.equal(await panel(title).getAttribute('id'),await tab(title).getAttribute('aria-controls'));
        const head=await header.boundingBox(),body=await panel(title).boundingBox();
        assert(Math.abs(head.x-body.x)<1&&Math.abs(head.width-body.width)<1);
        assert(Math.abs(head.y+head.height-body.y)<1);
        positions.push(head);
      }
      const [left,right,below]=positions;
      assert(right.x>left.x&&right.y===left.y&&below.y>left.y&&below.x===left.x&&below.width>left.width+right.width-2);
    }
    await verifySplit();
    assert.equal(await page.locator('.am-task-toolbar').isVisible(),false);
    const captionTop=await page.locator('#task-layout').evaluate(node=>node.getBoundingClientRect().top);
    const archive=page.locator('#list-toggle');
    const archiveBox=await archive.boundingBox();
    for(const title of [third,second]){
      const header=await tab(title).locator('xpath=ancestor::*[@role="tablist"]').boundingBox();
      assert(Math.abs(header.y-captionTop)<1&&archiveBox.y>=header.y&&archiveBox.y+archiveBox.height<=header.y+header.height);
    }
    assert.equal(await archive.evaluate(node=>node.closest('.am-pane-header')?.hasAttribute('data-caption-end')),true);
    if(macTitlebar){
      assert.equal(await page.locator('body').evaluate(node=>node.classList.contains('am-mac-titlebar')),true);
      await page.waitForFunction(()=>[...document.querySelectorAll('[data-caption] .am-close-x')].every(button=>{
        const box=button.getBoundingClientRect();
        return window.titlebarAreas?.holes.some(([x,y,w,h])=>Math.abs(x-box.x)<1&&Math.abs(y-box.y)<1&&Math.abs(w-box.width)<1&&Math.abs(h-box.height)<1);
      }));
    }
    const close=tab(first).locator('xpath=..').locator('.am-close-x');
    await tab(first).hover();
    const idleCloseColor=await close.evaluate(node=>getComputedStyle(node).color);
    await close.hover();
    const closeStyle=await close.evaluate(node=>{
      const style=getComputedStyle(node);
      return {color:style.color, background:style.backgroundColor};
    });
    assert.equal(closeStyle.background,'rgba(0, 0, 0, 0)');
    assert.notEqual(closeStyle.color,idleCloseColor);
    for(const title of [first,second,third]){
      const composer=content(title).locator('.am-compose-field:visible');
      assert(await composer.locator('#answer-send svg').isVisible());
      assert.equal(await content(title).locator('.am-watch-compose').count(),0);
      const thread=content(title).locator('#thread');
      assert(await thread.locator('.am-message-content').count());
      assert(await thread.evaluate(node=>{
        const footer=node.parentElement.querySelector('.am-chat-footer').getBoundingClientRect();
        return footer.height>0&&Math.abs(node.getBoundingClientRect().bottom-footer.bottom)<1;
      }),title+' has a scrollbar beside its composer');
    }
    const leftComposer=await content(third).locator('.am-compose-field:visible').boundingBox();
    const rightComposer=await content(second).locator('.am-compose-field:visible').boundingBox();
    assert(Math.abs(leftComposer.y-rightComposer.y)<2,'same-height panes align their reply fields');
    await page.evaluate(()=>window.messengerDictationModels({enabled:true,ready:true,supported:true,selected:null,models:[]}));
    await content(second).locator('#compose .am-dictation-button').click();
    const recording=await page.evaluate(()=>window.dictationCommands.at(-1));
    assert.equal(recording.action,'start');
    await page.evaluate(id=>{
      window.messengerDictationEvent({id,event:'recording'});
      window.messengerDictationEvent({id,event:'text',text:'Проверочный голос'});
    },recording.id);
    assert.equal(await replyBox(second).inputValue(),'Проверочный голос');
    assert.equal(await primary.inputValue(),'Основной черновик');
    assert.equal(await replyBox(first).inputValue(),'Черновик в нижней панели');
    if(output)await page.screenshot({path:path.join(output,'task-split.png')});
    await page.setViewportSize({width:760,height:850});
    assert.equal(await page.locator('.am-pane-grid').evaluate(grid=>getComputedStyle(grid).gridTemplateColumns.split(' ').length),1);
    await page.waitForFunction(()=>document.querySelectorAll('.am-pane-header[data-caption]').length===1);
    assert.equal(await archive.evaluate(node=>node.closest('.am-pane-header')?.hasAttribute('data-caption-start')),true);
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
    assert.deepEqual(errors,[]);
    await page.reload();
    await tabs.first().waitFor();
    await verifySplit();
    assert.equal(await tab(first).getAttribute('aria-selected'),'true');
    await panel(first).waitFor();
    await panel(third).waitFor();
    for(const title of [first,second,third]){
      if(title===third)assert.equal(await strip.getByRole('tab',{name:third,exact:true}).isVisible(),true);
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
