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
      const writes = [];
      page.on('pageerror', error => errors.push(error.message));
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
          if (pathname === '/api/agent-login') return route.fulfill({status:501,json:{ok:false,detail:'Fixture terminal failed',command:'fictional-cli login',hint:'fixture launcher error'}});
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
          '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: false}, peers: connected ? [{id:'fixture-peer',name:'Peer fixture',age:0,agents:[{agent:'cursor'}],coordinator:'cursor-vscode'}] : [], accept: true},
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
      await page.locator('[data-tab=server]').click();
      await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').waitFor({state:'attached'});
      assert.equal(await page.locator('#machines [aria-current="true"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines .am-agent-line').first().innerText(),'Cursor');
      assert.equal(await page.locator('#machines').getByText('cursor-vscode',{exact:true}).count(),0);
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
      assert.equal(await codex.getByRole('button',{name:'Доверять',exact:true}).evaluate(n=>n.getBoundingClientRect().height),28);
      await codex.getByRole('button',{name:'Доверять',exact:true}).click();
      assert((await page.locator('#confirm-text').textContent()).includes('/fictional/project'));
      await page.locator('#confirm-go').click();
      await codex.getByRole('button',{name:'Доверять',exact:true}).waitFor({state:'detached',timeout:10000});
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-trust').map(w=>w.body),[{provider:'codex',project:'Welcome test',folder:'/fictional/project'}]);
      await codex.getByRole('button',{name:'Настройки',exact:true}).click();
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'600-dark-agent-settings.png')});
      assert.equal(await page.locator('#agent-cli-find').evaluate(n=>n.getBoundingClientRect().height),32);
      await page.locator('#agent-cli-find').click();
      await codex.getByText('Поиск не подтверждён. Проверьте терминал или выберите файл CLI в настройках.',{exact:true}).waitFor();
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-find').map(w=>w.body),[{provider:'codex',action:'find'},{provider:'codex',action:'find'}]);
      assert.deepEqual(errors, []);
      console.log(JSON.stringify({engine, nativeBridgeFixture: native, themes: ['light','dark','system'], widths: [1100,600], result: 'passed'}));
      await context.close();
    }
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
