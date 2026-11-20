// Browser regression checks using fictional HTTP responses only. No Messenger
// process, registry, model, working chat or user profile is opened.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_EXECUTABLE,
// BROWSER_ENGINE (chromium/webkit), SCREENSHOT_DIR.
const {strict: assert} = require('node:assert');
const fs = require('node:fs/promises');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assets = path.resolve(__dirname, '../internal/localui/assets');

(async () => {
  const engine = process.env.BROWSER_ENGINE || 'chromium';
  const browser = await playwright[engine].launch({headless: true,
    ...(process.env.BROWSER_EXECUTABLE ? {executablePath: process.env.BROWSER_EXECUTABLE} : {})});
  try {
    for (const native of [false, true]) {
      const context = await browser.newContext({viewport: {width: 1100, height: 850}, colorScheme: 'light'});
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      const errors = [];
      let created = false, registered = false, connected = false, trusted = false;
      let setupStates = [], installed = false;
      let updates = [];
      const writes = [];
      // WebKit reports a background request that a reload cut off as a page
      // error ("… due to access control checks"); the app catches it, so it
      // is not counted. Every other page error fails the check.
      page.on('pageerror', error => {
        if (engine === 'webkit' && /\/api\/[a-z-]+ due to access control checks\.$/.test(error.message)) return;
        errors.push(error.message);
      });
      if (native) await page.addInitScript(() => {
        window.messengerInitialTheme = 'light';
        window.themeMessages = [];
        window.webkit = {messageHandlers: {appearance: {postMessage: theme => window.themeMessages.push(theme)}}};
      });
      await page.route('**/*', async route => {
        const pathname = new URL(route.request().url()).pathname;
        if (route.request().method() === 'POST') {
          writes.push({path: pathname, body: route.request().postDataJSON()});
          if (pathname === '/api/project-create') created = true;
          if (pathname === '/api/agent-install' || pathname === '/api/agent-find') {
            const input=route.request().postDataJSON();
            const state={ok:true,...input,id:'fake-'+input.provider+'-'+input.action,state:'unknown'};
            setupStates=setupStates.filter(s=>s.provider!==input.provider||s.action!==input.action);
            setupStates.push(state);
            return route.fulfill({json:state});
          }
          if (pathname === '/api/agent-setup-finish') {
            const state=setupStates.find(s=>s.id===route.request().postDataJSON().id);
            installed=true;state.state='connected';
            if(state.action==='find')setupStates.filter(s=>s.provider===state.provider&&s.action==='install').forEach(s=>s.state='recovered');
            return route.fulfill({json:state});
          }
          if (pathname === '/api/agent-trust') trusted=true;
          if (pathname === '/api/agent-login') {
            if(route.request().postDataJSON().provider==='cursor')return route.fulfill({status:400,json:{ok:false,detail:'Каталог профиля агента недоступен: /fictional/missing-profile. Проверьте путь к профилю в настройках агента.'}});
            return route.fulfill({status:501,json:{ok:false,detail:'Fixture terminal failed',command:'fictional-cli login',hint:'fixture launcher error'}});
          }
          if (pathname === '/api/coordinator-connect') connected = true;
        }
        const fixtures = {
          '/api/window': {projects: created ? [{name: 'Welcome test'}] : [], project: created ? 'Welcome test' : '',
            folder: created ? '/fictional/project' : '', messenger: '/fictional/messenger', data_root: '/fictional/data',
            agents: [...(registered ? [{agent: 'cursor', cli: '/fictional/cursor', models: []}] : []), ...(installed ? [{agent:'codex',cli:'/fictional/codex',models:[],trust:{state:trusted?'trusted':'not_trusted'}}] : [])], tasks: [],
            coordinator: {connected, client: 'cursor-vscode'},
            cursor_link: {state: connected ? 'connected' : created ? 'missing' : 'no_folder'},
            computer_name: 'Theme test', tasks_enabled: true},
          '/api/agent-setup-status': setupStates,
          '/api/discovery': {agents: ['claude','codex','cursor'].map(provider => ({provider, state: 'not_found'}))},
          '/api/client-info': {client: 'running', outdated: false},
          '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: false}, peers: connected ? [{id:'fixture-peer',name:'Peer fixture',host:true,age:0,agents:[{agent:'cursor'}],coordinator:'cursor-vscode'},{id:'fixture-peer-2',name:'Paused agent fixture',age:0,agents:[],coordinator:'claude-code'}] : [], updates, accept: true},
          '/api/self-update': {current: '0.0.0-preview.test'},
        };
        if (pathname.startsWith('/api/')) return route.fulfill({json: fixtures[pathname] || {ok: true}});
        const filename = pathname === '/' ? 'index.html' : pathname.slice(1);
        if (!['index.html', 'app.js', 'app.css', 'i18n.js'].includes(filename)) return route.fulfill({status: 404, body: ''});
        await route.fulfill({body: await fs.readFile(path.join(assets, filename)), contentType:
          filename.endsWith('.js') ? 'text/javascript' : filename.endsWith('.css') ? 'text/css' : 'text/html'});
      });
      const theme = () => page.evaluate(() => document.documentElement.dataset.theme);
      const settings = page.locator('#settings-dialog');
      const trigger = page.locator('[aria-controls="theme-select-menu"]');
      async function choose(label) {
        await trigger.click();
        await page.getByRole('option', {name: label, exact: true}).click();
        assert(await settings.evaluate(node => node.open), 'choosing an option must not close settings');
        assert.equal(await trigger.evaluate(node => node.getBoundingClientRect().height), 32);
      }
      await page.goto('http://messenger.test/#fictional');
      const welcome = page.locator('#thread .am-welcome');
      await welcome.getByRole('heading', {name: 'Добро пожаловать в Agent Messenger', exact: true}).waitFor();
      assert(await welcome.getByText('Как это работает', {exact: true}).isVisible());
      assert(await welcome.getByText('Сначала создайте проект', {exact: true}).isVisible());
      assert.equal(await page.locator('#task-panel').isVisible(), false, 'an empty task pane must not cover onboarding');
      await welcome.getByRole('button', {name: 'Новый проект', exact: true}).click();
      assert(await page.locator('#project-dialog').evaluate(node => node.open));
      await page.locator('#project-dialog-close').click();
      assert.equal(writes.length, 0, 'opening/cancelling onboarding must not change backend state');
      await page.locator('#settings-open').click();
      await choose('Тёмная');
      assert.equal(await theme(), 'dark');
      const dark = await page.locator('body').evaluate(node => getComputedStyle(node).backgroundColor);
      await choose('Светлая');
      assert.equal(await theme(), 'light');
      const light = await page.locator('body').evaluate(node => getComputedStyle(node).backgroundColor);
      assert.notEqual(light, dark, 'the page palette must change');
      if (native) {
        assert.deepEqual(await page.evaluate(() => window.themeMessages), ['dark', 'light']);
      } else {
        assert.equal(await page.evaluate(() => localStorage.getItem('messenger-theme')), 'light');
        await page.reload();
        assert.equal(await theme(), 'light');
        await page.locator('#settings-open').click();
      }
      // Keyboard selection and Escape must work inside the modal as well.
      await trigger.focus();
      await page.keyboard.press('ArrowDown');
      await page.keyboard.press('End');
      await page.keyboard.press('Enter');
      assert.equal(await theme(), 'dark');
      await trigger.click();
      await page.keyboard.press('Escape');
      assert(await settings.evaluate(node => node.open));
      assert.equal(await page.locator('#theme-select-menu').evaluate(node => node.matches(':popover-open')), false);
      await choose('Как в системе');
      await page.emulateMedia({colorScheme: 'dark'});
      assert.equal(await theme(), 'system');
      assert.equal(await page.locator('body').evaluate(node => getComputedStyle(node).backgroundColor), dark);
      await page.emulateMedia({colorScheme: 'light'});
      assert.equal(await page.locator('body').evaluate(node => getComputedStyle(node).backgroundColor), light);
      // The language menu shares the same modal parent and used to be inert too.
      await page.locator('[aria-controls="lang-select-menu"]').click();
      await page.getByRole('option', {name: 'English', exact: true}).click();
      await page.waitForFunction(() => document.documentElement.lang === 'en');
      await page.locator('#settings-open').click();
      await page.locator('[aria-controls="lang-select-menu"]').click();
      await page.getByRole('option', {name: 'Русский', exact: true}).click();
      await page.waitForFunction(() => document.documentElement.lang === 'ru');
      for (const width of [1100, 600]) {
        await page.setViewportSize({width, height: 850});
        for (const [value, label] of [['light', 'Светлая'], ['dark', 'Тёмная']]) {
          await page.locator('#settings-open').click();
          await choose(label);
          assert.equal(await theme(), value);
          if (process.env.SCREENSHOT_DIR && !native) {
            await fs.mkdir(process.env.SCREENSHOT_DIR, {recursive: true});
            const shot = name => page.screenshot({animations: 'disabled', path: path.join(process.env.SCREENSHOT_DIR, `${width}-${value}-${name}.png`)});
            await shot('dialog');
            await trigger.click();
            await shot('menu');
            await page.keyboard.press('Escape');
            await page.locator('#settings-close').click();
            for (const tab of ['tasks', 'agents', 'server']) {
              await page.locator(`[data-tab=${tab}]`).click();
              if (tab === 'tasks') await page.locator('#thread').evaluate(node => node.scrollTop = 0);
              await shot(tab);
            }
          } else await page.locator('#settings-close').click();
          // A non-modal select must continue to accept pointer input.
          await page.locator('[aria-controls="project-select-menu"]').click();
          await page.keyboard.press('Escape');
        }
      }
      // Even with an installed agent, connecting a coordinator requires a project.
      registered = true;
      await page.reload();
      await welcome.getByRole('heading', {name: 'Добро пожаловать в Agent Messenger', exact: true}).waitFor();
      assert.equal(await welcome.getByRole('button', {name: 'Подключить', exact: true}).count(), 0);
      await welcome.getByRole('button', {name: 'Новый проект', exact: true}).click();
      await page.locator('#project-name').fill('Welcome test');
      await page.locator('#project-folder').fill('/fictional/project');
      await page.locator('#project-save').click();
      await welcome.getByText('Подключён: Cursor', {exact: true}).waitFor();
      assert(await welcome.getByRole('heading', {name: 'Добро пожаловать в Agent Messenger', exact: true}).isVisible());
      assert.equal(await welcome.getByRole('button', {name: 'Новый проект', exact: true}).count(), 0);
      assert.deepEqual(writes.filter(write => write.path === '/api/project-create'),
        [{path: '/api/project-create', body: {name: 'Welcome test', folder: '/fictional/project'}}]);
      assert.deepEqual(writes.filter(write => write.path === '/api/coordinator-connect'),
        [{path: '/api/coordinator-connect', body: {project: 'Welcome test', agent: 'cursor'}}]);
      await page.setViewportSize({width:1100,height:850});
      const projects = page.locator('[aria-controls="project-select-menu"]');
      await projects.click();
      const projectMenu = page.locator('#project-select-menu');
      await projectMenu.waitFor({state:'visible'});
      await Promise.all([
        page.waitForResponse(response => new URL(response.url()).pathname==='/api/window'),
        page.evaluate(() => window.messengerRefresh()),
      ]);
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert(await projectMenu.isVisible(), 'refreshing the same projects should keep the menu open');
      await page.keyboard.press('Escape');
      await page.setViewportSize({width:600,height:850});
      await page.locator('[data-tab=server]').click();
      await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').waitFor({state:'attached'});
      assert.equal(await page.locator('#machines [aria-current="true"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines .am-agent-line').first().innerText(),'Cursor');
      assert.equal(await page.locator('#machines').getByText('cursor-vscode',{exact:true}).count(),0);
      // An agent paused or removed on a computer is not listed there, even
      // when a chat of it once used Messenger's tools (owner, 2026-11-15).
      assert.equal(await page.locator('#machines [data-peer="fixture-peer-2"] .am-agent-line').count(),0);
      // T-036: the computer that runs the server is labelled; this one is not.
      assert.equal(await page.locator('#machines [data-peer="fixture-peer"] .am-machine-role').innerText(),'сервер');
      assert.equal(await page.locator('#machines [aria-current="true"] .am-machine-role').count(),0);
      await page.locator('[data-tab=tasks]').click();
      assert(await page.locator('#task-panel').isVisible(), 'the task panel returns after creating a project');
      await page.locator('#list-toggle').click();
      assert.equal(await page.locator('#task-panel').isVisible(), false);
      await page.locator('[data-tab=agents]').click();
      const codex=page.locator('[data-provider=codex]');
      await codex.getByRole('button',{name:'Установить',exact:true}).click();
      await codex.getByText('Нет подтверждения из терминала. Если он закрыт, нажмите «Найти».',{exact:true}).waitFor();
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-install').map(w=>w.body),[{provider:'codex',action:'install'}]);
      assert.equal(writes.filter(w=>w.path==='/api/agent-page').length,0);
      // Recover a missing install receipt through an explicit search, never reinstall.
      await codex.locator('.am-steps').getByRole('button',{name:'Найти',exact:true}).click();
      await codex.getByText('Поиск не подтверждён. Проверьте терминал или выберите файл CLI в настройках.',{exact:true}).waitFor();
      assert.equal(writes.filter(w=>w.path==='/api/agent-install').length,1);
      const foundSetup=setupStates.find(s=>s.action==='find');
      foundSetup.state='found';foundSetup.cli='/fictional/codex';
      await codex.getByText('Codex CLI установлен',{exact:true}).waitFor({timeout:10000});
      assert.equal(writes.filter(w=>w.path==='/api/agent-setup-finish').length,1);
      await page.reload();
      await page.locator('[data-tab=agents]').click();
      await codex.getByText('Codex CLI установлен',{exact:true}).waitFor({timeout:10000});
      await codex.getByRole('button',{name:'Войти',exact:true}).click();
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-login').map(w=>w.body),[{provider:'codex',cli:'/fictional/codex',environment:''}]);
      await page.locator('#toasts').getByText('Fixture terminal failed',{exact:true}).waitFor();
      assert(await page.locator('#toasts button').first().isVisible());
      await page.locator('#toasts .am-toast-close').click();
      await page.locator('article[data-provider=cursor]').getByRole('button',{name:'Войти',exact:true}).click();
      await page.locator('#toasts').getByText('Каталог профиля агента недоступен: /fictional/missing-profile. Проверьте путь к профилю в настройках агента.',{exact:true}).waitFor();
      assert.equal(await page.locator('#toasts').getByText('Проверьте введённые данные.',{exact:true}).count(),0);
      await page.locator('#toasts .am-toast-close').click();
      assert.equal(await codex.getByRole('button',{name:'Доверять',exact:true}).evaluate(n=>n.getBoundingClientRect().height),28);
      await codex.getByRole('button',{name:'Доверять',exact:true}).click();
      assert((await page.locator('#confirm-text').textContent()).includes('/fictional/project'));
      await page.locator('#confirm-go').click();
      await codex.getByRole('button',{name:'Доверять',exact:true}).waitFor({state:'detached',timeout:10000});
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-trust').map(w=>w.body),[{provider:'codex',project:'Welcome test',folder:'/fictional/project'}]);
      await codex.getByRole('button',{name:'Настройки',exact:true}).click();
      assert.equal(await page.locator('#agent-models').inputValue(),''); // An explicitly empty list stays empty.
      assert((await page.locator('#agent-models-note').textContent()).includes('[effort=high]'));
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'600-dark-agent-settings.png')});
      assert.equal(await page.locator('#agent-cli-find').evaluate(n=>n.getBoundingClientRect().height),32);
      await page.locator('#agent-cli-find').click();
      await codex.getByText('Поиск не подтверждён. Проверьте терминал или выберите файл CLI в настройках.',{exact:true}).waitFor();
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-find').map(w=>w.body),[{provider:'codex',action:'find'},{provider:'codex',action:'find'}]);
      // T-182: the internet scope asks for the external address and sends it.
      await page.locator('[data-tab=server]').click();
      assert.equal(await page.locator('#host-public-box').isHidden(), true);
      await page.locator('#host-scope').evaluate(n => {n.value = 'internet'; n.dispatchEvent(new Event('change', {bubbles: true}));});
      assert.equal(await page.locator('#host-public-box').isVisible(), true);
      await page.locator('#host-enable').click();
      await page.locator('#host-status').getByText('Укажите внешний адрес: белый IP или домен этого компьютера.', {exact: true}).waitFor();
      assert.equal(writes.filter(w => w.path === '/api/server-action' && w.body.action === 'enable').length, 0);
      await page.locator('#host-public').fill('203.0.113.7');
      await page.locator('#host-enable').click();
      await page.waitForTimeout(300);
      assert.deepEqual(writes.filter(w => w.path === '/api/server-action' && w.body.action === 'enable').map(w => w.body),
        [{action: 'enable', scope: 'internet', port: 47100, public_address: '203.0.113.7'}]);
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'server-internet.png')});
      // T-181: notices have no scroll of their own; a new one pushes out the oldest.
      updates = [1,2,3,4,5].map(i => ({request_id:'fixture-update-'+i, computer:'fixture-peer', state:'refused',
        text:'fixture refusal '+i+' '+'long reason '.repeat(40), updated_at:new Date(Date.now()-(10-i)*1000).toISOString()}));
      await page.setViewportSize({width:1100,height:850});
      await page.reload();
      await page.locator('[data-tab=tasks]').click();
      await page.locator('#notices .am-notice').nth(2).waitFor({timeout:10000});
      assert.equal(await page.locator('#notices .am-notice').count(), 3);
      assert.deepEqual(await page.locator('#notices .am-notice').evaluateAll(n => n.map(c => c.dataset.key)),
        ['update-fixture-update-3-refused','update-fixture-update-4-refused','update-fixture-update-5-refused']);
      assert.equal(await page.locator('.am-chat-footer').evaluate(n => getComputedStyle(n).overflowY), 'visible');
      assert.equal(await page.locator('#notices').evaluate(n => n.scrollHeight <= n.clientHeight), true);
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'notices.png')});
      await page.locator('#notices .am-toast-close').first().click();
      assert.equal(await page.locator('#notices .am-notice').count(), 2);
      updates = [];
      assert.deepEqual(errors, []);
      console.log(JSON.stringify({engine, nativeBridgeFixture: native, themes: ['light','dark','system'], widths: [1100,600], result: 'passed'}));
      await context.close();
    }
    // Windows without a title bar: the tab row and the task list toggle keep
    // a gap above them and stay centred on the window buttons (owner,
    // 2026-11-15: they were pressed against the top edge).
    {
      const context = await browser.newContext({viewport: {width: 1100, height: 600}, colorScheme: 'light'});
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      const hides = [];
      // The update offered, downloading, installing, restarting (owner,
      // 2026-11-15: one button and one dialog).
      const began = new Date(Date.now() - 3 * 60 * 1000).toISOString();
      const ended = '2026-11-14T09:30:00Z';
      const stat = {id:'old-1',title:'Old 1',at:'2026-11-19T09:20:00',project:'Frame',agent:'codex',model:'gpt-6.2-sol[effort=medium]',computer:'Runner',tokens:12};
      const localTask = {id:'local-task',title:'Local task',project:'Frame',agent:'codex',state:'running',created_at:began,started_at:began,model:'gpt-6.2-sol[effort=low]',ran_model:'gpt-6.2-sol[effort=medium]'};
      const remoteTask = {...localTask,id:'remote-task',title:'Remote task',remote:{peer:'dev-1',name:'Runner',worker:'none'}};
      const ownerNotes = [];
      const update = {state: 'found', version: '0.0.0-preview.99'};
      const updates = [];
      const clears = [];
      await page.addInitScript(() => { window.amDesktop = 'windows'; window.amFrameless = true; window.amWindow = () => {}; });
      await page.route('**/*', async route => {
        const pathname = new URL(route.request().url()).pathname;
        if (pathname === '/api/feedback-export') return route.fulfill({json: {ok: true, result: {path: '/fictional/Downloads/feedback.zip', remarks: 2}}});
        if (pathname === '/api/feedback-clear') { clears.push(true); return route.fulfill({json: {ok: true, result: {deleted: 2, left: 0}}}); }
        if (pathname === '/api/window') return route.fulfill({json: {projects: [{name: 'Frame'}], project: 'Frame', folder: '/fictional/frame', agents: [], coordinator: {connected: false}, computer_name: 'Ёж и Йога', tasks_enabled: true, feedback_count: 5, feedback_new: 2, tasks:[localTask,remoteTask],
          shared: [{peer: 'host', name: 'Giver', age: 1, tasks: [
            {id: 'sent', project: 'Frame', title: 'Sent task', agent: 'cursor', state: 'running', created_at: began, started_at: began, model: 'composer-2.5[effort=medium]', runs_on_peer: 'dev-1', runs_on: 'Old name'},
            {id: 'own', project: 'Frame', title: 'Own task', agent: 'codex', state: 'running', created_at: '2026-11-15T09:00:00Z'},
            {id: 'old-1', project: 'Frame', title: 'Old 1', agent: 'codex', state: 'done', created_at: '2026-11-14T09:00:00Z', finished_at: ended},
            {id: 'old-2', project: 'Frame', title: 'Old 2', agent: 'codex', state: 'failed', created_at: '2026-11-14T08:00:00Z', finished_at: '2026-11-14T08:30:00Z'}]}]}});
        if (pathname === '/api/server') return route.fulfill({json: {host: {running: false, enabled: false, addresses: []}, member: {connected: true, reachable: true, address:'203.0.113.7:47100', connected_at:began}, peers: [{id: 'host', name: 'Giver', age: 0, agents: []}, {id: 'dev-1', name: 'Runner', age: 0, version:'0.0.0-preview.97', agents: [{agent:'cursor',running:1}]}], updates: [], accept: true}});
        if (pathname === '/api/usage-stats') return route.fulfill({json:{total:{tasks:1,tokens:12,input_tokens:8,output_tokens:4},agents:[],computers:[],days:[],recent:[stat],top:[stat]}});
        if (pathname === '/api/task') {
          const remote = new URL(route.request().url()).searchParams.get('id') === remoteTask.id;
          const items = [{kind:'text',text:'Fixture reply',at:began}];
          if (remote) items.push(...ownerNotes.map(note=>({kind:'note',text:note.text,at:began,author:'owner',state:'sent'})));
          else items.push(
          {kind:'tool',title:'Команда',text:"rg --files | rg 'RuntimePanelUtils\\.cs$'",at:began},
          {kind:'tool',title:'Команда',text:"cm cat FreeCamera.cs#cs:3474 | rg -n -C 2 'Mouse|UIOwnsKeys'",at:began},
          {kind:'tool',title:'Bash',text:'cm cat Tools/40_tune.steps#cs:3474',description:'Прочитать шаги ручной проверки',at:began},
          {kind:'tool',title:'Команда',text:'ls Assets/Infantry/Client',at:began},
          {kind:'tool',title:'Команда',text:"python3 - <<'PY'\nprint('test')\nPY",at:began},
            {kind:'report',text:'Previous run finished',at:ended,verified:true},
            {kind:'outcome',text:'Previous run was interrupted',at:began,state:'interrupted',verified:true}
          );
          return route.fulfill({json:{task:remote?remoteTask:localTask,items}});
        }
        if (pathname === '/api/task-note') ownerNotes.push(route.request().postDataJSON());
        if (pathname === '/api/task-hide') hides.push(route.request().postDataJSON());
        if (pathname === '/api/self-update') {
          if (route.request().method() === 'POST') { updates.push(route.request().postDataJSON()); update.state = 'downloading'; return route.fulfill({json: {ok: true, state: 'downloading'}}); }
          return route.fulfill({json: {ok: true, installed_version: '0.0.0-preview.98', ...update}});
        }
        if (pathname === '/api/update-progress') return route.fulfill({json: update.state === 'downloading' && update.done ? {downloading: true, version: update.version, done: update.done, total: 100} : {downloading: false}});
        if (pathname.startsWith('/api/')) return route.fulfill({json: {ok: true}});
        const file = pathname === '/' ? 'index.html' : pathname.slice(1);
        const type = file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : file.endsWith('.svg') ? 'image/svg+xml' : 'text/html';
        try { return route.fulfill({body: await fs.readFile(path.join(assets, file)), contentType: type}); } catch (_) { return route.fulfill({status: 404}); }
      });
      await page.goto('http://127.0.0.1/#fixture-token');
      await page.locator('#list-toggle').waitFor({state: 'visible'});
      const tab = await page.locator('.am-tab').first().boundingBox();
      const toggle = await page.locator('#list-toggle').boundingBox();
      const button = await page.locator('.am-window-btn').first().boundingBox();
      assert(tab.y >= 8 && toggle.y >= 8, `tab row too close to the top: tab ${tab.y}, toggle ${toggle.y}`);
      assert(Math.abs((toggle.y + toggle.height / 2) - (button.y + button.height / 2)) <= 1, 'toggle off the window buttons');
      // T-186: another computer's task says where it runs, and who gave it
      // when that is another computer.
      const sharedLabels = await page.locator('.am-task-shared .am-task-agent > span:not(.am-task-when)').allInnerTexts();
      assert.deepEqual(sharedLabels.filter(Boolean), ['Cursor на Runner — от Giver', 'Codex на Giver', 'Codex на Giver', 'Codex на Giver']);
      const completedTime=page.locator('.am-task-shared',{hasText:'Old 1'}).locator('.am-task-when');
      assert.equal(await completedTime.getAttribute('datetime'),ended);
      assert.match(await completedTime.innerText(),/\d{1,2}:\d{2}/);
      assert.equal(await page.locator('.am-task',{hasText:localTask.title}).locator('.am-task-when').getAttribute('datetime'),began);
      await page.locator('[data-tab=server]').click();
      const runner=page.locator('[data-peer="dev-1"]');
      await runner.locator('.am-agent-model').waitFor();
      assert.equal(await runner.locator('.am-agent-model').textContent(),'composer-2.5 Med');
      assert.equal(await runner.locator('.am-agent-status').textContent(),'3 мин');
      assert.equal(await runner.locator('.am-machine-name .am-card-update').count(),1);
      assert.equal(await page.locator('.am-machine-name #app-update-button').count(),1);
      const peerUpdate=runner.getByRole('button',{name:'Обновить до v0.99'});
      assert.equal(await peerUpdate.locator('.am-update-label').textContent(),'v0.97');
      assert.equal(await runner.locator('.am-machine-version').count(),0);
      for (const part of ['svg','.am-update-label']) {
        await peerUpdate.locator(part).click();
        await page.locator('#confirm-dialog').waitFor({state:'visible'});
        assert.equal(await page.locator('#confirm-title').innerText(),'Обновить Runner до v0.99?');
        await page.locator('#confirm-cancel').click();
      }
      for (const theme of ['light','dark']) {
        await page.evaluate(theme => { document.documentElement.dataset.theme=theme; },theme);
        const style=await peerUpdate.evaluate(node=>{
          const css=getComputedStyle(node);
          return {color:css.color,weight:css.fontWeight,background:css.backgroundColor};
        });
        assert.deepEqual(style,{color:theme==='light'?'rgb(129, 83, 5)':'rgb(242, 198, 91)',weight:'400',background:'rgba(0, 0, 0, 0)'});
      }
      await page.evaluate(()=>{document.documentElement.dataset.theme='light';});
      for (const computer of [page.locator('.am-machine-card[aria-current=true]'),runner]) {
        const name = computer.locator('.am-machine-name > span:nth-child(2)');
        assert(await name.evaluate(node => node.getBoundingClientRect().height >= parseFloat(getComputedStyle(node).lineHeight)-0.5),
          'computer name must keep the full line height');
      }
      const disconnect = page.locator('#member-disconnect');
      await disconnect.waitFor({state: 'visible'});
      for (const width of [1100, 600]) {
        await page.setViewportSize({width, height: 850});
        const details = await page.locator('#member-details').boundingBox();
        const action = await disconnect.boundingBox();
        const badge = await page.locator('#member-badge').boundingBox();
        assert(action.x >= details.x + details.width, 'disconnect action overlaps the connection details');
        assert(Math.abs(action.y + action.height / 2 - (details.y + details.height / 2)) <= 1, 'disconnect action is on a separate row');
        assert(Math.abs(action.x + action.width - (badge.x + badge.width)) <= 1, 'disconnect action is off the right edge of the status');
      }
      await page.setViewportSize({width: 1100, height: 600});
      if (process.env.SCREENSHOT_DIR) {
        await page.emulateMedia({colorScheme:'dark'});
        await page.locator('[aria-labelledby=member-title]').screenshot({path: path.join(process.env.SCREENSHOT_DIR,'server-connected.png')});
        await page.emulateMedia({colorScheme:'light'});
      }
      await page.locator('[data-tab=stats]').click();
      const detail = page.locator('#stats-top .am-stats-task-text .am-muted');
      await detail.waitFor();
      assert.equal(await detail.textContent(),'19 ноября 9:20 Frame Codex Sol 6.2 Medium на Runner');
      await page.locator('[data-stats-sort=tokens]').click();
      assert.equal(await detail.textContent(),'19 ноября 9:20 Frame Codex Sol 6.2 Medium на Runner');
      await page.locator('[data-tab=tasks]').click();
      await page.locator('.am-task-title',{hasText:localTask.title}).click();
      await page.locator('#thread .am-meta-model').first().waitFor();
      assert.equal(await page.locator('#thread .am-meta-model').first().textContent(),'Sol 6.2 Medium');
      assert.match(await page.locator('#task-head').innerText(),/Codex \(Sol 6\.2 Medium\)/);
      const closedTime=page.locator('#thread .am-delivery-close time');
      assert.equal(await closedTime.getAttribute('datetime'),ended);
      assert.match(await closedTime.innerText(),/\d{1,2}:\d{2}/);
      assert.equal(await page.locator('#thread .am-outcome time').getAttribute('datetime'),began);
      const actionGroup=page.locator('#thread .am-tool-group');
      await actionGroup.locator(':scope > summary').click();
      assert.equal(await actionGroup.locator('.am-tool-group-title').innerText(),'5 действий');
      assert.equal(await actionGroup.locator('.am-tool-breakdown').innerText(),'2 поиска, чтение файла, просмотр папки и запуск скрипта');
      const labels=actionGroup.locator('.am-action-label');
      assert.deepEqual(await labels.allInnerTexts(),['Найти файл RuntimePanelUtils.cs','Найти упоминания Mouse и UIOwnsKeys в файле FreeCamera.cs','Прочитать шаги ручной проверки','Посмотреть файлы в папке Infantry/Client','Запустить скрипт Python']);
      assert.equal(await labels.locator('svg').count(),5);
      await labels.nth(1).click();
      assert.equal(await actionGroup.locator('details[open] pre').innerText(),"cm cat FreeCamera.cs#cs:3474 | rg -n -C 2 'Mouse|UIOwnsKeys'");
      if (process.env.SCREENSHOT_DIR) await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, 'frameless.png')});
      await page.locator('.am-task-title',{hasText:remoteTask.title}).click();
      const reply = page.locator('#answer-text');
      await reply.fill('Use blue');
      await page.locator('#answer-send').click();
      await page.getByText('Поставлено в очередь для исполнителя',{exact:true}).waitFor();
      assert.equal(ownerNotes.length,1);
      assert.equal(ownerNotes[0].task,remoteTask.id);
      assert.equal(ownerNotes[0].text,'Use blue');
      assert.match(ownerNotes[0].op_id,/^window-remote-task-/);
      // Owner, 2026-11-15: one button removes every finished task from the list.
      await page.locator('.am-task-section', {hasText: 'Завершённые'}).locator('.am-task-section-action').click();
      await page.locator('#confirm-go').click();
      await page.waitForFunction(() => !document.querySelector('#confirm-dialog').open);
      assert.deepEqual(hides, [{peer: 'host', task: 'old-1'}, {peer: 'host', task: 'old-2'}]);
      // Feedback: the export takes the new remarks, and the exported ones can
      // be cleared once passed on (owner, 2026-11-16).
      await page.locator('#fb-open').click();
      assert.equal(await page.locator('#fb-export').innerText(), 'Выгрузить новые отзывы (2)');
      await page.locator('#fb-export').click();
      await page.locator('#fb-clear').waitFor({state: 'visible'});
      await page.locator('#fb-clear').click();
      await page.locator('#confirm-go').click();
      await page.waitForFunction(() => document.querySelector('#fb-status').textContent === 'Журнал очищен');
      assert.equal(clears.length, 1);
      assert.equal(await page.locator('#fb-export').isDisabled(), true);
      await page.locator('#fb-close').click();
      // One update button: the dialog follows the download, the install and
      // the restart; a failed install says so and offers to try again.
      const updateButton = page.locator('#app-update-button');
      await updateButton.waitFor({state: 'visible'});
      assert(!(await updateButton.evaluate(b => b.classList.contains('am-update-loading'))), 'a found update shows a download ring');
      assert.equal(await updateButton.locator('.am-update-label').textContent(),'v0.98');
      assert(await page.locator('#app-installed-version').isHidden());
      await updateButton.locator('.am-update-glyph').click();
      await page.locator('#update-dialog').waitFor({state: 'visible'});
      assert.deepEqual(updates, [{action: 'update'}]);
      update.done = 40;
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Загрузка… 40%');
      assert.equal(await page.locator('#update-title').innerText(), 'Обновление v0.99');
      update.state = 'installing';
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Установка…');
      update.state = 'available';
      await page.locator('#update-error').waitFor({state: 'visible'});
      assert.equal(await page.locator('#update-error').innerText(), 'Обновление не установлено. Программа осталась прежней.');
      await page.keyboard.press('Escape');
      await page.locator('#update-dialog').waitFor({state: 'hidden'});
      await updateButton.locator('.am-update-label').click();
      await page.locator('#update-dialog').waitFor({state: 'visible'});
      update.state = 'downloading';
      await page.keyboard.press('Escape');
      assert(await page.locator('#update-dialog').isVisible(), 'the update dialog closed while downloading');
      update.state = 'installed';
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Перезапуск…');
      assert(await page.locator('#update-actions').isHidden(), 'buttons shown while restarting');
      assert.equal(updates.length, 2);
      if (process.env.SCREENSHOT_DIR) await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, 'update-dialog.png')});
      console.log(JSON.stringify({engine, frameless: true, result: 'passed'}));
      await context.close();
    }
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
