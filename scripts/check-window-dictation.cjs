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
    const models = state => page.evaluate(data => window.messengerDictationModels(data), state);
    const answer = page.locator('#answer-text');
    const mic = page.locator('#compose .am-dictation-button');
    const tiny = {id:'whisper-tiny',name:'Whisper Tiny',size:75000000,state:'available'};
    const base = {id:'whisper-base',name:'Whisper Base',size:142000000,state:'available'};
    const state = {enabled:false,ready:false,supported:true,selected:null,models:[tiny,base]};
    assert.deepEqual(await last(),{action:'models'});
    assert.equal(await mic.isDisabled(),true);
    await page.locator('#settings-open').click();
    await page.locator('#settings-menu-open').click();
    await page.getByRole('button',{name:'Голос',exact:true}).click();
    assert.deepEqual(await last(),{action:'models'});
    const voiceSwitch = page.getByRole('switch',{name:'Диктовка'});
    await models({...state,supported:false});
    await page.getByText('Диктовка недоступна в этой версии Kotel').waitFor();
    assert.equal(await voiceSwitch.isDisabled(),true);
    await models(state);
    for (const id of ['voice-models-title','voice-microphone-title']) {
      assert(await page.locator('#'+id).evaluate(heading=>heading.closest('.am-card')!==null));
    }
    for (const width of [1200, 800, 600]) {
      await page.setViewportSize({width,height:800});
      const rows = await page.locator('#voice-page .am-inline-setting').evaluateAll(rows => rows.map(row => {
        const label = row.querySelector('label').getBoundingClientRect();
        const trigger = row.querySelector('.am-select-trigger').getBoundingClientRect();
        const bounds = row.getBoundingClientRect();
        return {
          contained: trigger.left >= bounds.left - 1 && trigger.right <= bounds.right + 1,
          endsAtRight: Math.abs(trigger.right - bounds.right) < 1,
          stacked: label.bottom <= trigger.top,
        };
      }));
      for (const row of rows) {
        assert.deepEqual(row, {contained:true,endsAtRight:true,stacked:width===600}, `Voice selector at ${width}px`);
      }
    }
    await page.setViewportSize({width:1200,height:800});
    await page.getByRole('button',{name:'Проверить микрофон'}).click();
    const microphoneCheck=await last();
    assert.equal(microphoneCheck.action,'test-microphone');
    await emit(microphoneCheck.id,'recording','10');
    await page.getByText('Скажите несколько слов').waitFor();
    await page.locator('#voice-mic-time').filter({hasText:/0:\d\d из 0:10/}).waitFor();
    await page.getByRole('button',{name:'Остановить запись'}).click();
    assert.deepEqual(await last(),{...microphoneCheck,action:'stop'});
    const wave=Buffer.alloc(44+32000);
    wave.write('RIFF');wave.writeUInt32LE(wave.length-8,4);wave.write('WAVEfmt ',8);
    wave.writeUInt32LE(16,16);wave.writeUInt16LE(1,20);wave.writeUInt16LE(1,22);
    wave.writeUInt32LE(16000,24);wave.writeUInt32LE(32000,28);
    wave.writeUInt16LE(2,32);wave.writeUInt16LE(16,34);
    wave.write('data',36);wave.writeUInt32LE(32000,40);
    await emit(microphoneCheck.id,'microphone-audio',wave.toString('base64'));
    const playback=page.locator('#voice-mic-audio');
    await playback.waitFor();
    await page.waitForFunction(()=>document.querySelector('#voice-mic-audio').duration===1);
    await emit(microphoneCheck.id,'text','Проверка записи');
    await page.getByText('Проверка записи',{exact:true}).waitFor();
    await page.getByRole('button',{name:'Проверить микрофон'}).click();
    await playback.waitFor({state:'hidden'});
    assert.equal(await playback.getAttribute('src'),null);
    await emit((await last()).id,'microphone-unavailable');
    await page.locator('#voice-mic-status').getByText('Микрофон недоступен.').waitFor();
    for(const [label,option] of [['Режим сочетания','Удерживать во время речи'],['Язык речи','Русский'],['Остановка после тишины','15 секунд']]){
      await page.getByRole('button',{name:label,exact:true}).click();
      await page.getByRole('option',{name:option,exact:true}).click();
    }
    assert.deepEqual(await page.evaluate(()=>['voice-mode','voice-language','voice-silence'].map(id=>localStorage.getItem(id))),['hold','ru','15']);
    await page.setViewportSize({width:600,height:800});
    assert(await page.locator('#voice-page').evaluate(page=>page.scrollWidth<=page.clientWidth));
    await page.setViewportSize({width:1200,height:800});
    await voiceSwitch.click();
    assert.deepEqual(await last(),{action:'set-enabled',enabled:'true'});
    const enabled = {...state,enabled:true};
    await models(enabled);
    assert.equal(await voiceSwitch.getAttribute('aria-checked'),'true');
    await page.getByRole('button',{name:'Загрузить Whisper Tiny'}).click();
    assert.deepEqual(await last(),{action:'download',model:tiny.id});
    await models({...enabled,models:[{...tiny,state:'downloading',bytesDownloaded:0},base]});
    const progress = page.getByRole('progressbar',{name:'Загрузка модели Whisper Tiny'});
    assert.match(await progress.textContent(), /Загрузка, 0 из 75\s+МБ \(0\s*%\)/);
    await models({...enabled,models:[{...tiny,state:'downloading',bytesDownloaded:28000000},base]});
    assert.match(await progress.textContent(), /Загрузка, 28 из 75\s+МБ \(37\s*%\)/);
    assert.equal(await progress.getAttribute('aria-valuenow'),'37');
    await page.getByRole('button',{name:'Отменить Whisper Tiny'}).click();
    assert.deepEqual(await last(),{action:'cancel-download',model:tiny.id});
    await models({...enabled,models:[{...tiny,state:'error',error:'download-failed'},base]});
    await page.getByText('Не удалось загрузить модель').waitFor();
    await page.getByRole('button',{name:'Повторить Whisper Tiny'}).click();
    assert.deepEqual(await last(),{action:'download',model:tiny.id});
    const installed = [{...tiny,state:'installed'},{...base,state:'installed'}];
    await models({...enabled,ready:true,selected:tiny.id,models:installed});
    await page.getByText('Используется',{exact:true}).waitFor();
    await page.getByText('Скачана',{exact:true}).waitFor();
    const deleteTiny=page.getByRole('button',{name:'Удалить модель Whisper Tiny'});
    assert.equal(await deleteTiny.locator('svg path').getAttribute('d'),
      await page.locator('#archive-clear svg path').getAttribute('d'));
    await deleteTiny.click();
    assert.deepEqual(await last(),{action:'remove-model',model:tiny.id});
    await models({...enabled,ready:true,selected:tiny.id,models:[{...installed[0],removalError:'recording'},installed[1]]});
    await page.getByText('Завершите запись перед удалением модели').waitFor();
    await deleteTiny.click();
    await models({...enabled,ready:false,selected:null,models:[tiny,installed[1]]});
    await page.getByRole('button',{name:'Загрузить Whisper Tiny'}).waitFor();
    assert.equal(await deleteTiny.count(),0);
    assert.equal(await mic.isDisabled(),true);
    await page.getByRole('button',{name:'Выбрать Whisper Base'}).click();
    assert.deepEqual(await last(),{action:'select',model:base.id});
    const ready = {...enabled,ready:true,selected:base.id,models:[tiny,installed[1]]};
    await models(ready);
    await voiceSwitch.click();
    assert.deepEqual(await last(),{action:'set-enabled',enabled:'false'});
    await models({...ready,enabled:false,ready:false});
    await page.getByText('Выбрана',{exact:true}).waitFor();
    assert.equal(await mic.isDisabled(),true);
    await voiceSwitch.click();
    await models(ready);
    await page.locator('#settings-close').click();
    assert.equal(await mic.isEnabled(),true);
    await answer.fill('До после');
    await answer.press('Meta+Shift+KeyD');
    const held=await last();
    assert.equal(held.action,'start');
    await emit(held.id,'recording');
    assert.deepEqual(await last(),{...held,action:'stop'});
    await emit(held.id,'empty');
    const field = page.locator('#compose .am-compose-field');
    const fieldAtRest = await field.boundingBox();
    await answer.evaluate(input => input.setSelectionRange(3,3));
    await mic.click();
    const first = await last();
    assert.equal(first.action,'start');
    assert.equal(first.language,'ru');
    assert.equal(first.silence,15);
    await emit(first.id,'recording');
    assert.deepEqual(await field.boundingBox(),fieldAtRest);
    await answer.press('Enter');
    assert.equal((await last()).action,'stop');
    assert.deepEqual(await field.boundingBox(),fieldAtRest);
    assert.equal(await answer.inputValue(),'До после');
    await emit(first.id,'text','середина');
    assert.equal(await answer.inputValue(),'До середина после');
    await answer.evaluate(input => input.setSelectionRange(input.value.length,input.value.length));
    await answer.press('!');
    assert.equal(await answer.inputValue(),'До середина после!');

    for (const [event,message] of [
      ['microphone-denied','Разрешите Kotel доступ к микрофону'],
      ['microphone-unavailable','Микрофон недоступен'],
      ['model-not-ready','Выберите загруженную модель в разделе «Голос»'],
    ]) {
      await mic.click();
      await emit((await last()).id,event);
      await page.locator('#compose').getByRole('status').getByText(message).waitFor();
      assert.deepEqual(await field.boundingBox(),fieldAtRest);
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
    const interrupted = await last();
    await emit(interrupted.id,'recording');
    await models({...ready,enabled:false,ready:false});
    assert.equal(await mic.isDisabled(),true);
    assert.deepEqual(await last(),{...interrupted,action:'cancel'});
    await emit(interrupted.id,'text','Не вставлять');
    assert.equal(await answer.inputValue(),'До середина после!');
    await models(ready);

    await mic.click();
    const oldChat = await last();
    await page.locator('#task-new').click();
    assert.deepEqual(await last(),{...oldChat,action:'cancel'});
    const draft = page.locator('#task-draft-text');
    await emit(oldChat.id,'text','Чужой черновик');
    assert.equal(await draft.inputValue(),'');
    const draftMic = page.locator('#task-draft .am-dictation-button');
    const draftField = page.locator('#task-draft .am-compose-field');
    const draftAtRest = await draftField.boundingBox();
    await draftMic.click();
    const draftRecording = await last();
    await emit(draftRecording.id,'recording');
    assert.deepEqual(await draftField.boundingBox(),draftAtRest);
    const draftAccess = await page.locator('#task-draft-access').boundingBox();
    const draftStatus = await page.locator('#task-draft .am-dictation-status').boundingBox();
    assert(draftStatus.y+draftStatus.height<=draftAccess.y);
    await draftMic.click();
    assert.deepEqual(await last(),{...draftRecording,action:'stop'});
    await emit(draftRecording.id,'text','Новая задача');
    assert.equal(await draft.inputValue(),'Новая задача');
    assert.equal(writes.filter(url=>url.startsWith('/api/task-')).length,0);
    await page.evaluate(() => sessionStorage.setItem('fixture-no-dictation','true'));
    await page.reload();
    await page.getByText('Сообщение из game').waitFor();
    assert.equal(await page.locator('.am-dictation-button').count(),0);
    assert.equal(await page.locator('[data-tab=voice]').count(),0);
    console.log(JSON.stringify({engine,result:'passed'}));
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error);process.exitCode=1;});
