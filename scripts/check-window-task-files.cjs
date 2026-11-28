// File actions with synthetic reply and file content. Does not start a client.
const {strict: assert} = require('node:assert');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assets = path.resolve(__dirname, '../internal/localui/assets');
const engine = process.env.BROWSER_ENGINE || 'chromium';

(async () => {
  const browser = await playwright[engine].launch({headless:true});
  try {
    const page = await browser.newPage({viewport:{width:1140,height:760}, colorScheme:'dark'});
    page.setDefaultTimeout(6000);
    await page.route('http://localhost/**', async route => {
      const file = new URL(route.request().url()).pathname;
      if (file === '/') return route.fulfill({contentType:'text/html; charset=utf-8',body:`<!doctype html><meta charset="utf-8">
        <link rel="stylesheet" href="/app.css"><link rel="stylesheet" href="/file-viewer.css">
        <div id="am-concept" class="am-task-sidebar-preview" data-chat-font="inter"><div id="task-layout" class="am-chat-layout am-queue-collapsed"><header class="am-task-toolbar"><h3 class="am-chat-head">Отчёт о проверке</h3><span></span></header><div class="am-chat"><div class="am-chat-body"><div id="report" class="am-thread"></div></div></div></div></div>
        <script src="/i18n.js"></script><script src="/file-viewer.js"></script>`});
      return route.fulfill({path:path.join(assets,file.slice(1))});
    });
    await page.goto('http://localhost/');
    await page.evaluate(() => {
      window.fileRequests = [];
      const source = {
        request: async (ref, action) => {
          window.fileRequests.push([ref.path, action]);
          if (action === 'download') return {name:'PLAN.md',data:'IyBQbGFuCg=='};
          if (ref.path === 'Docs/PLAN.md') return {
            path:'/fixture/Docs/PLAN.md', name:'PLAN.md', format:'markdown', can_edit:true, can_reveal:true, can_save:true,
            text:'# Plan\n\n**A marked line** and [More](Sub/More.txt:2)\n\n<script>window.fileInjected=true</script>\n\n| Project | Status |\n| --- | --- |\n| Alpha | Ready |',
          };
          if (ref.path === 'Docs/Sub/More.txt') return {
            path:'/fixture/Docs/Sub/More.txt', name:'More.txt', format:'text', text:'first\nsecond',
          };
          throw Error('Unavailable in this fixture');
        },
      };
      const report = document.querySelector('#report');
      report.append(
        window.taskFiles.paragraph(document.createElement('p'), 'See [Roadmap]\n(Docs/PLAN.md#L3).\nFiles: Docs/PLAN.md,\nDocs/Sub/More.txt', source, true),
        window.taskFiles.paragraph(document.createElement('p'), '[Remote](Docs/PLAN.md)', {...source,remote:'Remote fixture'}, true),
      );
    });
    assert.equal(await page.getByRole('button',{name:'Docs/PLAN.md'}).count(),1);
    assert.equal(await page.getByRole('button',{name:'Docs/Sub/More.txt'}).count(),1);
    const roadmap=page.getByRole('button',{name:'Roadmap'});
    await roadmap.hover();
    assert.deepEqual(await roadmap.evaluate(node=>{
      const style=getComputedStyle(node);
      return [style.backgroundColor,style.boxShadow,style.outlineStyle,style.textDecorationStyle];
    }),['rgba(0, 0, 0, 0)','none','none','solid']);
    await roadmap.click();
    const viewer = page.locator('.am-file-viewer');
    const first = page.getByRole('tab',{name:'PLAN.md'});
    await first.waitFor();
    assert.equal(await page.getByRole('dialog').count(),0);
    assert.equal(await page.locator('#report').isVisible(),true);
    await viewer.getByRole('heading',{name:'Plan',exact:true}).waitFor();
    assert.equal(await viewer.locator('.am-file-target strong').textContent(),'A marked line');
    assert.equal(await viewer.locator('script').count(),0);
    assert.equal(await page.evaluate(() => window.fileInjected),undefined);
    assert.deepEqual(await viewer.getByRole('row').last().getByRole('cell').allTextContents(),['Alpha','Ready']);
    assert.equal(await viewer.getByRole('button',{name:'Открыть в текстовом редакторе'}).count(),1);
    assert.equal(await viewer.getByRole('button',{name:'Показать в папке'}).count(),1);
    const downloadPromise = page.waitForEvent('download');
    await viewer.getByRole('button',{name:'Сохранить как…'}).click();
    const download = await downloadPromise;
    assert.equal(download.suggestedFilename(),'PLAN.md');
    await page.evaluate(() => {
      window.webkit = {messageHandlers:{reportFileSave:{postMessage: payload => {
        window.nativeFileSave = payload;
        window.taskFiles.saved(payload.id, 'saved');
      }}}};
    });
    await viewer.getByRole('button',{name:'Сохранить как…'}).click();
    await viewer.getByRole('status').getByText('Файл сохранён').waitFor();
    assert.equal(await page.evaluate(() => window.nativeFileSave.name),'PLAN.md');
    assert.equal(await page.evaluate(() => window.nativeFileSave.data),'IyBQbGFuCg==');
    await viewer.getByRole('button',{name:'More'}).click();
    await viewer.locator('pre .am-file-target').waitFor();
    assert.equal(await viewer.locator('pre .am-file-target').textContent(),'second');
    const second = page.getByRole('tab',{name:'More.txt'});
    assert.equal(await second.getAttribute('aria-selected'),'true');
    await first.click();
    await viewer.getByRole('heading',{name:'Plan',exact:true}).waitFor();
    await second.click();
    await viewer.locator('pre .am-file-target').waitFor();
    if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'file-tabs.png')});
    const popout = viewer.getByRole('button',{name:'Открыть в отдельном окне'});
    const popupPromise = page.waitForEvent('popup');
    await popout.click();
    const popup = await popupPromise;
    assert.equal(await popup.locator('mark').textContent(),'second');
    await popup.setViewportSize({width:600,height:500});
    assert.equal(await popup.locator('mark').isVisible(),true);
    await popup.close();
    await page.evaluate(() => {
      window.webkit.messageHandlers.reportFileWindow = {postMessage: payload => { window.separatePreview = payload; }};
    });
    await viewer.getByRole('button',{name:'Открыть в отдельном окне'}).click();
    assert.deepEqual(await page.evaluate(() => window.separatePreview),{name:'More.txt',text:'first\nsecond',line:2});
    await page.getByTitle('Закрыть More.txt',{exact:true}).click();
    await page.getByTitle('Закрыть PLAN.md',{exact:true}).click();
    assert.equal(await viewer.isVisible(),false);
    await page.getByRole('button',{name:'Remote',exact:true}).click();
    await viewer.getByText('Файл находится на компьютере «Remote fixture».').waitFor();
    assert.equal(await viewer.getByRole('button',{name:'Копировать путь'}).count(),1);
    assert.deepEqual(await page.evaluate(() => window.fileRequests),[
      ['Docs/PLAN.md','preview'],['Docs/PLAN.md','download'],['Docs/PLAN.md','download'],['Docs/Sub/More.txt','preview'],
    ]);
    console.log('Task file viewer passed ('+engine+')');
  } finally { await browser.close(); }
})().catch(error=>{console.error(error);process.exitCode=1;});
