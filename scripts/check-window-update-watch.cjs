// Drive the real sidebar against a fixture update stream and save card screenshots.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assets = path.resolve(__dirname, '../internal/localui/assets');

(async () => {
  const browser = await playwright[process.env.BROWSER_ENGINE || 'chromium'].launch({
    ...(process.env.BROWSER_EXECUTABLE ? {executablePath: process.env.BROWSER_EXECUTABLE} : {}),
  });
  try {
    const page = await browser.newPage({viewport: {width: 1120, height: 720}, deviceScaleFactor: 2, colorScheme: 'dark'});
    page.setDefaultTimeout(5000);
    let cursor = 1, mark, wake, releaseCancel;
    const cancellations=[];
    let cancelResponse='ok';
    function publish(state, version) {
      mark = {id: 'upd-fixture', state, version, at: Date.now() / 1000};
      cursor++;
      wake?.();
    }
    const peer = {id:'dev-pc', name:'ПК-1', age:0, version:'1.0.0-preview.103', agents:[{agent:'codex'}]};
    const task = {id:'task-fixture',project:'Проект',state:'waiting_coordinator',agent:'codex',title:'Проверить переход анимации',ran_model:'gpt-6.2-sol[effort=medium]',usage:{input_tokens:5300000,output_tokens:100000},created_at:new Date().toISOString()};
    const server = {host:{running:true,broker_id:'fixture',addresses:[]}, member:{}, peers:[peer], updates:[], update_cancellations:[]};
    await page.route('**/*', async route => {
      const url = new URL(route.request().url());
      if (url.pathname === '/api/server-action') {
        const body = route.request().postDataJSON();
        if (body.action === 'update-watch') {
          if (body.cursor === String(cursor)) await new Promise(resolve => {wake = resolve;});
          return route.fulfill({json:{ok:true,result:{broker:'fixture', self:'host', cursor:String(cursor), marks:mark?{'dev-pc':mark}:{}}}});
        }
        if (body.action === 'update') publish('waiting');
        if (body.action === 'cancel-update') {
          cancellations.push(body);
          if(cancelResponse==='refused')return route.fulfill({status:400,json:{detail:'Компьютер сейчас не на связи.'}});
          server.update_cancellations=[{device:body.device,request_id:body.request_id}];
          if(cancelResponse==='lost')return route.abort();
          await new Promise(resolve=>{releaseCancel=resolve;});
        }
        return route.fulfill({json:{ok:true,result:server}});
      }
      const fixtures = {
        '/api/window':{projects:[{name:'Проект'}],project:'Проект',computer_name:'Мак',agents:[],tasks:[task],coordinator:{},folder:'/fixture'},
        '/api/client-info':{client:'running',client_version:'1.0.0-preview.104'},
        '/api/self-update':{installed_version:'1.0.0-preview.104'},
        '/api/server':server,
        '/api/task':{task,items:[]},
      };
      if (url.pathname.startsWith('/api/')) return route.fulfill({json:fixtures[url.pathname] || {ok:true}});
      const file = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
      return route.fulfill({body:await fs.readFile(path.join(assets,file)),contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html'});
    });
    await page.goto('http://messenger.test/#fixture');
    const card = page.locator('.am-peer-card[data-peer="dev-pc"]');
    async function shot(name) {
      if (!process.env.SCREENSHOT_DIR) return;
      await fs.mkdir(process.env.SCREENSHOT_DIR,{recursive:true});
      await card.screenshot({path:path.join(process.env.SCREENSHOT_DIR,name+'.png')});
    }
    const arrow=card.getByRole('button',{name:/Обновить до/});
    await arrow.waitFor();
    const row=page.locator('#task-list .am-task').filter({hasText:task.title});
    if(!await row.isVisible() && await page.locator('#list-toggle').getAttribute('aria-expanded')==='false')await page.locator('#list-toggle').click();
    await row.waitFor();
    assert.equal(await row.locator('.am-task-status').evaluate(e=>getComputedStyle(e).color),'rgb(242, 201, 76)');
    assert.match(await arrow.locator('path').getAttribute('d'),/M5 20h14/);
    await shot('available');
    if(process.env.SCREENSHOT_DIR)await row.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'task-waiting.png')});
    await arrow.click();
    await page.locator('#confirm-go').click();
    await page.mouse.move(0,0);
    const cancelUpdate=card.getByRole('button',{name:/Отменить запрос/});
    await cancelUpdate.waitFor();
    const rotation=icon=>icon.evaluate(e=>getComputedStyle(e).animationName);
    assert.equal(await rotation(cancelUpdate.locator('.am-update-spin')),'am-update-spin');
    await shot('waiting');
    await cancelUpdate.hover();
    assert.equal(await cancelUpdate.locator('svg').last().isVisible(),true);
    assert.equal(await cancelUpdate.locator('svg').first().isVisible(),false);
    assert.equal(await cancelUpdate.locator('svg').last().evaluate(e=>getComputedStyle(e).color),'rgb(242, 139, 130)');
    await shot('cancel');
    const target=await cancelUpdate.boundingBox();
    await cancelUpdate.click();
    await card.getByRole('img',{name:'Отправляем запрос на отмену…'}).waitFor();
    publish('queued');
    await page.mouse.dblclick(target.x+target.width/2,target.y+target.height/2);
    assert.equal(cancellations.length,1,'another click after a redraw must not send a duplicate');
    releaseCancel();
    const awaiting=card.getByRole('img',{name:'Отмена запрошена, ждём подтверждения'});
    await awaiting.waitFor();
    assert.equal(await cancelUpdate.count(),0);
    await shot('cancel-pending');
    assert.equal(cancellations[0].request_id,'upd-fixture');
    assert.equal(cancellations[0].device,'dev-pc');
    await page.reload();
    await awaiting.waitFor();
    server.update_cancellations=[];
    publish('cancelled');
    await arrow.waitFor();
    await page.reload();
    await arrow.waitFor();
    publish('waiting');
    cancelResponse='refused';
    await cancelUpdate.click();
    await page.getByText('Компьютер сейчас не на связи.',{exact:true}).waitFor();
    await cancelUpdate.waitFor();
    assert.equal(cancellations.length,2,'a refusal allows the owner to try again');
    cancelResponse='lost';
    await cancelUpdate.click();
    await awaiting.waitFor();
    assert.equal(cancellations.length,3,'a lost reply must use the queued cancellation status');
    server.update_cancellations=[];
    publish('cancelled');
    await arrow.waitFor();
    publish('updating');
    const installing=card.getByRole('img',{name:'Обновление устанавливается'});
    await installing.waitFor();
    assert.equal(await card.getByRole('button').count(),0);
    await page.mouse.move(0,0);
    const indicator=installing.locator('.am-update-spin');
    assert.equal(await rotation(indicator),'am-update-spin');
    await shot('installing');
    await page.emulateMedia({reducedMotion:'reduce'});
    assert.equal(await rotation(indicator),'none');
    publish('done','1.0.0-preview.104');
    await installing.waitFor({state:'hidden'});
    assert.equal(await arrow.count(),0,'the confirmation must outrank the older presence build even with the same display name');
    console.log('Sidebar update, hover, exact cancellation and installation states passed.');
    await page.close();
    wake?.();
  } finally {
    await browser.close();
  }
})().catch(err => {console.error(err);process.exitCode=1;});
