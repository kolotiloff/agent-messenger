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
    let cursor = 1, mark, wake, releaseCancel, downloadProgress;
    const cancellations=[];
    const updateRequests=[];
    let cancelResponse='ok';
    let published='1.0.1-preview.104';
    function publish(state, version, id = 'upd-fixture') {
      mark = {id, state, version, at: Date.now() / 1000};
      cursor++;
      wake?.();
    }
    const peer = {id:'dev-pc', name:'ПК-1', age:0, version:'1.0.0-preview.103', agents:[{agent:'codex'}]};
    const task = {id:'task-fixture',project:'Проект',state:'waiting_coordinator',agent:'codex',title:'Проверить переход анимации',ran_model:'gpt-6.2-sol[effort=medium]',usage:{input_tokens:5300000,output_tokens:100000},created_at:new Date().toISOString()};
    const server = {host:{running:true,broker_id:'fixture',addresses:[]}, member:{}, peers:[peer], updates:[], update_cancellations:[]};
    function reply(state, version, id = 'upd-fixture', text) {
      server.updates = [{computer:peer.id,request_id:id,state,version,text,updated_at:new Date().toISOString()}];
    }
    await page.route('**/*', async route => {
      const url = new URL(route.request().url());
      if (url.pathname === '/api/server-action') {
        const body = route.request().postDataJSON();
        if (body.action === 'update-watch') {
          if (body.cursor === String(cursor)) await new Promise(resolve => {wake = resolve;});
          return route.fulfill({json:{ok:true,result:{broker:'fixture', self:'host', cursor:String(cursor), marks:mark?{'dev-pc':mark}:{}}}});
        }
        if (body.action === 'update') {
          updateRequests.push(body.device);
          publish('waiting');
        }
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
        '/api/self-update':{installed_version:'1.0.1-preview.104'},
        '/api/update-check':{ok:true,installed_version:'1.0.1-preview.104',version:published},
        '/api/update-progress':downloadProgress,
        '/api/server':server,
        '/api/task':{task,items:[]},
      };
      if (url.pathname.startsWith('/api/')) return route.fulfill({json:fixtures[url.pathname] || {ok:true}});
      const file = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
      return route.fulfill({body:await fs.readFile(path.join(assets,file)),contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html'});
    });
    const previousTime=new Date(Date.now()-10000).toISOString();
    server.updates=['refused','failed','waiting'].map((state,i)=>({computer:'old-'+i,request_id:'previous-'+i,state,stale:state==='waiting',updated_at:previousTime}));
    await page.goto('http://messenger.test/#fixture');
    const card = page.locator('.am-peer-card[data-peer="dev-pc"]');
    async function refreshServer() {
      await page.locator('#settings-open').click();
      await page.locator('#settings-menu-open').click();
      await page.locator('#settings-close').click();
    }
    async function shot(name) {
      if (!process.env.SCREENSHOT_DIR) return;
      await fs.mkdir(process.env.SCREENSHOT_DIR,{recursive:true});
      await card.screenshot({path:path.join(process.env.SCREENSHOT_DIR,name+'.png')});
    }
    const arrow=card.getByRole('button',{name:/Обновить версию/});
    await arrow.waitFor();
    assert.equal(await page.locator('#notices .am-notice-bad').count(),0,'previous update errors and timeouts must stay quiet on startup');
    const pending={computer:peer.id,request_id:'previous-pending',state:'waiting',updated_at:previousTime};
    server.updates.push(pending);
    await refreshServer();
    await card.getByRole('button',{name:'Отменить обновление'}).waitFor();
    pending.stale=true;
    await refreshServer();
    const timeout=page.locator('#notices .am-notice-bad').filter({hasText:'ПК-1 не ответил об обновлении'});
    await timeout.waitFor();
    await timeout.getByRole('button',{name:'Закрыть'}).click();
    server.updates=[];
    await refreshServer();
    await arrow.waitFor();
    assert.match(await arrow.getAttribute('title'),/^Обновить версию/);
    assert.deepEqual(await arrow.locator('svg').evaluate(svg=>{
      const box=svg.getBoundingClientRect(),head=svg.closest('.am-machine-name').getBoundingClientRect();
      return [box.width,box.height,Math.abs(box.top+box.height/2-head.top-head.height/2)<0.5];
    }),[14,14,true]);
    const row=page.locator('#task-list .am-task').filter({hasText:task.title});
    if(!await row.isVisible() && await page.locator('#list-toggle').getAttribute('aria-expanded')==='false')await page.locator('#list-toggle').click();
    await row.waitFor();
    assert.equal(await row.locator('.am-task-status').evaluate(e=>getComputedStyle(e).color),'rgb(206, 178, 113)');
    assert.match(await arrow.locator('path').getAttribute('d'),/M3 15v4/);
    await shot('available');
    const menu=page.locator('#device-menu');
    await card.locator('.am-machine-name').click({button:'right'});
    const menuOffer=menu.getByRole('menuitem',{name:/Обновить версию/});
    await menuOffer.hover();
    assert.equal(await menuOffer.locator('svg').count(),0);
    if(process.env.SCREENSHOT_DIR)await menu.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'menu-available.png')});
    await page.keyboard.press('Escape');
    if(process.env.SCREENSHOT_DIR)await row.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'task-waiting.png')});
    publish('queued', undefined, '');
    const waiting=card.getByRole('img',{name:'Ждёт обновления'});
    await waiting.waitFor();
    assert.equal(await waiting.getAttribute('title'),'Ждёт обновления');
    publish('cancelled');
    await arrow.waitFor();
    await arrow.click();
    await page.locator('#confirm-go').click();
    await page.mouse.move(0,0);
    const cancelUpdate=card.getByRole('button',{name:'Отменить обновление'});
    await cancelUpdate.waitFor();
    assert.equal(await cancelUpdate.getAttribute('title'),'Отменить обновление');
    assert.equal(await cancelUpdate.getAttribute('aria-description'),'Обновление запрошено');
    const rotation=icon=>icon.evaluate(e=>getComputedStyle(e).animationName);
    assert.equal(await rotation(cancelUpdate.locator('.am-update-spin')),'am-update-spin');
    await shot('waiting');
    await cancelUpdate.hover();
    assert.equal(await cancelUpdate.locator('svg').last().isVisible(),true);
    assert.equal(await cancelUpdate.locator('svg').first().isVisible(),false);
    assert.equal(await cancelUpdate.locator('svg').last().evaluate(e=>getComputedStyle(e).color),'rgb(242, 139, 130)');
    await shot('cancel');
    await card.locator('.am-machine-name').click({button:'right'});
    await menu.locator('.am-version').hover();
    await page.mouse.move(0,0);
    assert.equal(await menu.isVisible(),true,'moving outside must leave the menu open');
    await page.keyboard.press('Escape');
    await menu.waitFor({state:'hidden'});
    await card.locator('.am-machine-name').click({button:'right'});
    const menuCancel=menu.getByRole('menuitem',{name:'Отменить обновление'});
    await menuCancel.hover();
    if(process.env.SCREENSHOT_DIR)await menu.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'menu-cancel.png')});
    const target=await menuCancel.boundingBox();
    await menuCancel.click();
    await card.locator('.am-machine-name').click({button:'right'});
    await menu.getByText('Отправляем запрос на отмену…',{exact:true}).waitFor();
    assert.equal(await menu.locator('.am-version').textContent(),'v1.0');
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
    const center=await indicator.evaluate(svg=>{
      const arc=svg.querySelector('path');
      const length=arc.getTotalLength();
      const radiusError=Math.max(...Array.from({length:24},(_,index)=>{
        const point=arc.getPointAtLength(index*length/23);
        return Math.abs(Math.hypot(point.x-12,point.y-12)-8);
      }));
      const control=svg.parentElement.getBoundingClientRect();
      const animation=svg.getAnimations()[0];
      animation.pause();
      const offsets=[0,375,750,1125].map(time=>{
        animation.currentTime=time;
        const point=new DOMPoint(12,12).matrixTransform(svg.getScreenCTM());
        return Math.hypot(point.x-control.left-control.width/2,point.y-control.top-control.height/2);
      });
      return {radiusError,offsets};
    });
    assert(center.radiusError<0.05,'the curved segment must stay on a centered circle');
    assert(center.offsets.every(offset=>offset<0.5),'the rotation axis must stay centered in the control');
    await page.emulateMedia({reducedMotion:'reduce'});
    assert.equal(await rotation(indicator),'none');
    // The requester's inbox can confirm the install while the watch and
    // presence responses still carry the old state.
    peer.update_pending=true;
    peer.update_request_id='upd-fixture';
    reply('done','1.0.1-preview.104');
    await refreshServer();
    await page.getByText('ПК-1 обновлён до v1.0.1',{exact:true}).waitFor();
    assert.equal(await indicator.count(),0,'the success notice and stopped spinner must appear together');
    assert.equal(await arrow.count(),0,'old presence must not bring back the download button');
    assert.equal(await card.locator('.am-machine-version').textContent(),'v1.0.1');
    publish('queued',undefined,'upd-next');
    await cancelUpdate.waitFor();
    assert.equal(await cancelUpdate.getAttribute('aria-description'),'Ждёт обновления');
    peer.age=60;
    await refreshServer();
    await card.getByText('Ещё не выходил на связь',{exact:true}).waitFor();
    await cancelUpdate.hover();
    assert.equal(await cancelUpdate.locator('svg').last().isVisible(),true,'a delayed roster must not hide cancellation of a confirmed queue');
    peer.age=0;
    await refreshServer();
    await card.getByText('Codex',{exact:true}).waitFor();
    publish('updating',undefined,'upd-next');
    await installing.waitFor();
    publish('done','1.0.1-preview.104','upd-next');
    await installing.waitFor({state:'hidden'});
    assert.equal(await arrow.count(),0,'a shared confirmation must also outrank old presence');
    // Keep the signed build ordering even when the displayed version matches.
    server.updates=[];
    peer.update_pending=false;
    peer.version='1.0.0-preview.103';
    publish('done','1.0.0-preview.104');
    await refreshServer();
    await card.getByText('v1.0',{exact:true}).waitFor();
    assert.equal(await arrow.count(),0,'the confirmation must outrank the older presence build even with the same display name');
    publish('queued',undefined,'upd-failure');
    await cancelUpdate.waitFor();
    reply('updating',undefined,'upd-failure');
    await refreshServer();
    await installing.waitFor();
    assert.equal(await cancelUpdate.count(),0,'the installer has already taken over this request');
    const reason='Установка не началась за 5 минут. Запрос снят, обновление можно запросить снова.';
    reply('failed',undefined,'upd-failure',reason);
    await refreshServer();
    const failure=page.locator('#notices .am-notice-bad').filter({hasText:'ПК-1 не обновлён'});
    await failure.waitFor();
    assert.equal(await failure.locator('.am-notice-text').textContent(),reason);
    assert.equal(await failure.locator('[title]').count(),0,'a fully visible reason must not show a duplicate tooltip');
    assert.equal(await indicator.count(),0,'a failed request must also stop the spinner');
    assert.equal(await arrow.count(),1,'a recorded failure leaves the older version eligible');
    await page.reload();
    await arrow.waitFor();
    assert.equal(await failure.count(),0,'opening the window again must not replay the latest failure');
    reply('done','1.0.1-preview.104','fixture-success');
    await page.locator('#settings-open').click();
    await page.locator('#settings-menu-open').click();
    await page.locator('#settings-close').click();
    const success=page.locator('.am-window > #notices .am-notice-good').filter({hasText:'ПК-1 обновлён до v1.0.1'});
    await success.waitFor();
    if(process.env.SCREENSHOT_DIR){
      const clip=await success.boundingBox();
      if(clip)await page.screenshot({clip,path:path.join(process.env.SCREENSHOT_DIR,'update-notice.png')});
    }
    assert.equal(await success.locator('.am-notice-timer').count(),0);
    await success.getByRole('button',{name:'Закрыть'}).click();
    server.updates=[];
    publish('done','1.0.1-preview.104','fixture-success');
    peer.version='1.0.1-preview.104';
    peer.update_pending=false;
    await page.locator('#settings-open').click();
    await page.locator('#settings-menu-open').click();
    assert.equal(await page.locator('#settings-program-version').textContent(),'v1.0.1');
    const networkCheck=page.locator('#settings-network-update-check');
    assert(await networkCheck.isVisible());
    assert.equal(await networkCheck.textContent(),await page.locator('#settings-update-check').textContent());
    assert.equal(await page.locator('.am-program-version + #settings-network-update-row').count(),1);
    const localCheckBox=await page.locator('#settings-update-check').boundingBox();
    const networkCheckBox=await networkCheck.boundingBox();
    assert.equal(localCheckBox.x+localCheckBox.width,networkCheckBox.x+networkCheckBox.width);
    const networkLabel=await page.locator('#settings-network-update-label').boundingBox();
    assert(Math.abs(networkLabel.y+networkLabel.height/2-networkCheckBox.y-networkCheckBox.height/2)<1);
    await page.locator('#settings-update-check').click();
    const checked=page.locator('#notices').getByText('Установлена последняя версия',{exact:true});
    await checked.waitFor();
    const noticeStack=page.locator('#notices');
    const noticeInsideSettings=await noticeStack.boundingBox();
    assert.equal(noticeInsideSettings.y,44);
    assert.equal(noticeInsideSettings.x+noticeInsideSettings.width,1120-16);
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'manual-local-current.png')});
    await page.locator('#settings-close').click();
    await checked.waitFor();
    assert.deepEqual(await noticeStack.boundingBox(),noticeInsideSettings,'opening settings must not move window notices');
    await page.locator('#settings-open').click();
    const gearRows=page.locator('#settings-menu .am-select-option span');
    const gearHeights=await gearRows.evaluateAll(rows=>rows.map(row=>row.getBoundingClientRect().height));
    assert(gearHeights.every(height=>height===gearHeights[0]),'gear menu labels should each occupy one line');
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'settings-menu.png')});
    const gearCheck=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/update-check');
    await page.locator('#settings-menu-update').click();
    await gearCheck;
    await checked.waitFor();
    await page.locator('#notices .am-notice').filter({hasText:'Установлена последняя версия'}).getByRole('button',{name:'Закрыть'}).click();
    published='1.0.7-preview.110';
    const priorRequests=updateRequests.length;
    await card.locator('.am-machine-name').click({button:'right'});
    await menu.getByRole('menuitem',{name:'Проверить обновления'}).click();
    await page.locator('#notices').getByText('Опубликована v1.0.7',{exact:true}).waitFor();
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'manual-peer-available.png')});
    assert.equal(updateRequests.length,priorRequests,'checking a release must not request an update');
    server.peers.push({id:'offline',name:'ПК без связи',age:99,version:'1.0.0-preview.103'});
    await page.locator('#settings-open').click();
    await page.locator('#settings-menu-open').click();
    await networkCheck.click();
    const review=page.locator('#network-update-dialog');
    await review.waitFor();
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'manual-network-review.png')});
    assert.equal(await review.locator('li').filter({hasText:'ПК без связи'}).getByText('Не в сети',{exact:true}).count(),1);
    assert.equal(updateRequests.length,priorRequests,'review must precede any update requests');
    await review.getByRole('button',{name:'Обновить компьютеры'}).click();
    await review.getByText('Запрос отправлен',{exact:true}).waitFor();
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'manual-network-requested.png')});
    assert.deepEqual(updateRequests.slice(priorRequests),[peer.id]);
    await review.getByRole('button',{name:'Закрыть',exact:true}).click();
    await page.locator('#settings-close').click();
    // Use the actual window with simulated macOS updater callbacks.
    await page.emulateMedia({reducedMotion:'no-preference'});
    await page.evaluate(()=>{
      window.webkit??={};
      window.webkit.messageHandlers??={};
      window.webkit.messageHandlers.updates={postMessage(){window.installRequests=(window.installRequests||0)+1;}};
      window.messengerUpdateStatus({installedVersion:'1.0.4-preview.107',version:'1.0.5-preview.108'});
    });
    const localUpdate=page.locator('#app-update-button');
    const dialog=page.locator('#update-dialog');
    const localSpin=localUpdate.locator('.am-update-spin');
    const download=localUpdate.locator('svg').first();
    await download.waitFor();
    await localUpdate.click();
    await dialog.waitFor();
    assert(await localSpin.isVisible()&&await download.isHidden()&&await localUpdate.isDisabled());
    assert.equal(await rotation(localSpin),'am-update-spin');
    downloadProgress={downloading:true,version:'1.0.5-preview.108',done:32,total:100};
    await page.evaluate(()=>window.messengerUpdateStatus({installedVersion:'1.0.4-preview.107',version:'1.0.5-preview.108',checking:true}));
    await page.getByText('Загрузка… 32%',{exact:true}).waitFor();
    const bounds=await dialog.boundingBox();
    await page.mouse.click(bounds.x+bounds.width+10,bounds.y+10);
    assert(await dialog.isVisible(),'a backdrop click must keep the active update dialog open');
    assert.equal(await localUpdate.locator('svg:visible').count(),1,'download progress must keep exactly one visible glyph');
    await page.keyboard.press('Escape');
    assert(await dialog.isVisible());
    downloadProgress=null;
    await page.evaluate(()=>{
      window.messengerUpdateStatus({installedVersion:'1.0.4-preview.107'});
      window.messengerUpdateRun({phase:'installing'});
    });
    await page.getByText('Установка…',{exact:true}).waitFor();
    assert(await localSpin.isVisible()&&await download.isHidden()&&await localUpdate.isDisabled());
    await page.evaluate(()=>{
      window.messengerUpdateRun({phase:'error',error:'Пакет не установлен'});
      window.messengerUpdateStatus({installedVersion:'1.0.4-preview.107',version:'1.0.5-preview.108'});
    });
    await page.getByText('Пакет не установлен',{exact:true}).waitFor();
    await page.mouse.click(bounds.x+bounds.width+10,bounds.y+10);
    await dialog.waitFor({state:'hidden'});
    assert(await download.isVisible()&&await localSpin.isHidden()&&await localUpdate.isEnabled());
    assert.equal(await page.evaluate(()=>window.installRequests),1);
    await localUpdate.click();
    await dialog.waitFor();
    assert.equal(await page.evaluate(()=>window.installRequests),2);
    assert(await localUpdate.isDisabled());
    console.log('Sidebar update, hover, exact cancellation and installation states passed.');
    await page.close();
    wake?.();
  } finally {
    await browser.close();
  }
})().catch(err => {console.error(err);process.exitCode=1;});
