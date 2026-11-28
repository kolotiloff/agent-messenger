// Exercise the composer against a fictional native dictation bridge.
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
    const page = await browser.newPage({viewport:{width:1200,height:800}});
    const writes = [];
    await page.addInitScript(() => {
      window.amDesktop = 'mac';
      if (sessionStorage.getItem('fixture-no-dictation')) return;
      window.dictationCommands = [];
      window.webkit = {messageHandlers:{composerDictation:{
        postMessage:message => window.dictationCommands.push(message),
      }}};
    });
    await page.route('https://messenger-preview.test/**', async route => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.startsWith('/api/')) {
        if (request.method() === 'POST') writes.push(url.pathname);
        return route.fulfill({json:reply(url,url.searchParams.get('project') || 'game',projects)});
      }
      if (url.pathname === '/') {
        const body = (await fs.readFile(path.join(assets,'index.html'),'utf8'))
          .replace('id="am-concept"','id="am-concept" class="am-task-sidebar-preview"');
        return route.fulfill({contentType:'text/html; charset=utf-8',body});
      }
      return route.fulfill({path:path.join(assets,url.pathname.slice(1))});
    });
    await page.goto('https://messenger-preview.test/#preview-only');
    await page.getByText('Сообщение из game').waitFor();
    const last = () => page.evaluate(() => window.dictationCommands.at(-1));
    const emit = (id,event,text) => page.evaluate(data => window.messengerDictationEvent(data), {id,event,text});
    const answer = page.locator('#answer-text');
    const mic = page.locator('#compose .am-dictation-button');
    await answer.fill('До после');
    await answer.evaluate(input => input.setSelectionRange(3,3));
    await mic.click();
    const first = await last();
    assert.equal(first.action,'start');
    assert.equal(first.language,'ru');
    await emit(first.id,'recording');
    await answer.press('Enter');
    assert.equal((await last()).action,'stop');
    assert.equal(await answer.inputValue(),'До после');
    await emit(first.id,'text','середина');
    assert.equal(await answer.inputValue(),'До середина после');
    await answer.evaluate(input => input.setSelectionRange(input.value.length,input.value.length));
    await answer.press('!');
    assert.equal(await answer.inputValue(),'До середина после!');

    for (const [event,message] of [
      ['microphone-denied','Разрешите Kotel доступ к микрофону'],
      ['speech-denied','Разрешите Kotel распознавание речи'],
      ['unavailable','Для языка интерфейса недоступно распознавание'],
    ]) {
      await mic.click();
      await emit((await last()).id,event);
      await page.locator('#compose').getByRole('status').getByText(message).waitFor();
      assert.equal(await answer.inputValue(),'До середина после!');
    }

    await mic.click();
    const cancelled = await last();
    await emit(cancelled.id,'recording');
    await answer.press('Escape');
    assert.deepEqual(await last(),{...cancelled,action:'cancel'});
    await emit(cancelled.id,'text','Не вставлять');
    assert.equal(await answer.inputValue(),'До середина после!');

    await mic.click();
    const oldChat = await last();
    await page.locator('#task-new').click();
    assert.deepEqual(await last(),{...oldChat,action:'cancel'});
    const draft = page.locator('#task-draft-text');
    await emit(oldChat.id,'text','Чужой черновик');
    assert.equal(await draft.inputValue(),'');
    const draftMic = page.locator('#task-draft .am-dictation-button');
    await draftMic.click();
    const draftRecording = await last();
    await emit(draftRecording.id,'recording');
    await draftMic.click();
    assert.deepEqual(await last(),{...draftRecording,action:'stop'});
    await emit(draftRecording.id,'text','Новая задача');
    assert.equal(await draft.inputValue(),'Новая задача');
    assert.equal(writes.filter(url=>url.startsWith('/api/task-')).length,0);
    await page.evaluate(() => sessionStorage.setItem('fixture-no-dictation','true'));
    await page.reload();
    await page.getByText('Сообщение из game').waitFor();
    assert.equal(await page.locator('.am-dictation-button').count(),0);
    console.log(JSON.stringify({engine,result:'passed'}));
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error);process.exitCode=1;});
