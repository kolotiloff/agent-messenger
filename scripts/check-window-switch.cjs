// Browser check with fictional HTTP responses only (T-265). No Messenger
// process, model or user profile is opened. Switching projects keeps the
// previous conversation until the next one is ready, and an
// empty project on a computer that has seen tasks says that it has none.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_ENGINE
// (chromium/webkit), SCREENSHOT_DIR.
const {strict: assert} = require('node:assert');
const fs = require('node:fs/promises');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assets = path.resolve(__dirname, '../internal/localui/assets');
const shots = process.env.SCREENSHOT_DIR;

(async () => {
  const engine = process.env.BROWSER_ENGINE || 'chromium';
  const browser = await playwright[engine].launch({headless: true});
  try {
    for (const scheme of ['light', 'dark']) {
      const context = await browser.newContext({viewport: {width: 1100, height: 700}, colorScheme: scheme, deviceScaleFactor: shots ? 2 : 1});
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      const began = new Date(Date.now() - 4 * 60 * 1000).toISOString();
      const task = (id, project, title, state) => ({id, project, title, agent: 'codex', state, created_at: began, started_at: began, updated_at: began, model: 'gpt-6.2-sol[effort=medium]'});
      const tasks = {
        Frame: [task('frame-1', 'Frame', 'Проверить сборку сцены', 'running'), task('frame-2', 'Frame', 'Обновить меню', 'done')],
        Site: [task('site-1', 'Site', 'Собрать страницу загрузки', 'done')],
        Server: [],
      };
      let feedDelay = 0, windowDelay = 0;
      let firstRun = false, folder = '', modelReady = false;
      const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
      await context.route('**/*', async route => {
        const url = new URL(route.request().url());
        if (route.request().method() === 'POST') {
          if (firstRun && url.pathname === '/api/project-folder') folder = route.request().postDataJSON().folder;
          return route.fulfill({json: {ok: true}});
        }
        if (url.pathname === '/api/window') {
          if (firstRun) return route.fulfill({json: {projects: [{name: 'Frame', folder}], project: 'Frame', folder,
            agents: modelReady ? [{agent: 'codex', models: ['gpt-6.2-sol[effort=medium]']}] : [],
            tasks: [], shared: [], computer_name: 'Fixture Mac', tasks_enabled: true}});
          const name = url.searchParams.get('project') || 'Frame';
          if (windowDelay) await wait(windowDelay);
          return route.fulfill({json: {projects: Object.keys(tasks).map(name => ({name})), project: name,
            folder: '/fictional/' + name, messenger: '/fictional/messenger', data_root: '/fictional/data',
            agents: [{agent: 'codex', cli: '/fictional/codex', models: []}], tasks: tasks[name], shared: [],
            coordinator: {connected: true, client: 'codex', active: true}, cursor_link: {state: 'connected'},
            computer_name: 'Fixture Mac', tasks_enabled: true}});
        }
        if (url.pathname === '/api/task') {
          if (feedDelay) await wait(feedDelay);
          const id = url.searchParams.get('id');
          const row = Object.values(tasks).flat().find(t => t.id === id);
          return route.fulfill({json: {task: row, items: [
            {kind: 'task', author: 'coordinator', text: 'Задача: ' + row.title, at: began, sender_agent: 'codex'},
            {kind: 'text', text: 'Работаю над задачей.', at: began},
          ]}});
        }
        const fixtures = {
          '/api/client-info': {client: 'running', outdated: false},
          '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: false}, peers: [], updates: [], accept: true},
          '/api/discovery': {agents: []},
          '/api/agent-setup-status': [],
          '/api/self-update': {current: '0.0.0-preview.96'},
          '/api/machine-order': {order: null},
        };
        if (url.pathname.startsWith('/api/')) return route.fulfill({json: fixtures[url.pathname] || {ok: true}});
        if (url.pathname === '/blank') return route.fulfill({body: '', contentType: 'text/html'});
        const filename = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
        if (!['index.html', 'app.js', 'app.css', 'i18n.js'].includes(filename)) return route.fulfill({status: 404, body: ''});
        const source = await fs.readFile(path.join(assets, filename), 'utf8');
        const body = firstRun && filename === 'index.html'
          ? source.replace('id="am-concept"', 'id="am-concept" class="am-task-sidebar-preview"') : source;
        await route.fulfill({body, contentType:
          filename.endsWith('.js') ? 'text/javascript' : filename.endsWith('.css') ? 'text/css' : 'text/html'});
      });
      // Record every welcome that is ever attached to the conversation pane.
      await page.addInitScript(() => {
        window.welcomeSeen = 0;
        new MutationObserver(() => { if (document.querySelector('#thread .am-welcome')) window.welcomeSeen++; })
          .observe(document, {childList: true, subtree: true});
      });
      await page.goto('http://messenger.test/#fictional');
      await page.locator('#thread').getByText('Работаю над задачей.').waitFor();
      const select = async name => {
        await page.locator('[aria-controls="project-select-menu"]').click();
        await page.locator('#project-select-menu').getByRole('option', {name, exact: true}).click();
      };

      // A slow switch keeps the old conversation on screen until the feed is ready.
      windowDelay = 700; feedDelay = 700;
      await page.evaluate(() => { window.welcomeSeen = 0; });
      await select('Site');
      await page.waitForFunction(() => document.querySelector('#thread').getAttribute('aria-busy') === 'true');
      assert.equal(await page.locator('#thread').getByText('Задача: Проверить сборку сцены').count(), 1);
      assert.equal(await page.locator('#thread .am-pane-loading').count(), 0);
      if (shots) await page.screenshot({path: path.join(shots, `switch-pending-${scheme}.png`)});
      await page.locator('#thread').getByText('Задача: Собрать страницу загрузки').waitFor();
      assert.equal(await page.evaluate(() => window.welcomeSeen), 0, 'welcome flashed during a slow switch');

      // A fast switch goes straight to the conversation.
      windowDelay = 0; feedDelay = 0;
      await select('Frame');
      await page.locator('#thread').getByText('Задача: Проверить сборку сцены').waitFor();
      assert.equal(await page.evaluate(() => window.welcomeSeen), 0, 'welcome flashed during a fast switch');
      if (shots) await page.screenshot({path: path.join(shots, `switch-task-${scheme}.png`)});

      // An empty project on a computer that has seen tasks says so.
      await select('Server');
      const empty = page.locator('#thread .am-pane-empty');
      await empty.getByText('Задач пока нет', {exact: true}).waitFor();
      assert.equal(await page.locator('#thread .am-welcome').count(), 0);
      if (shots) await page.screenshot({path: path.join(shots, `empty-project-${scheme}.png`)});

      // A computer that has never seen a task still gets the welcome.
      // Clear storage from an empty page, so the running window cannot
      // record the earlier tasks again.
      tasks.Frame = []; tasks.Site = [];
      // Leaving the page cancels the window's pending requests; WebKit
      // reports each as an access control failure. Only those, and only
      // while the check itself leaves and comes back, are expected.
      const before = errors.length;
      await page.goto('http://messenger.test/blank');
      await page.evaluate(() => localStorage.clear());
      await page.goto('http://messenger.test/#fictional');
      errors.splice(before, Infinity, ...errors.slice(before).filter(message => !/\/api\/[^ ]* due to access control checks\.$/.test(message)));
      await page.locator('#thread .am-welcome').getByRole('heading', {name: 'Добро пожаловать в Agent Messenger', exact: true}).waitFor();
      if (shots && scheme === 'light') await page.screenshot({path: path.join(shots, `first-run-welcome-${scheme}.png`)});

      // A new desktop window guides setup, then opens the real task composer.
      firstRun = true;
      await page.reload();
      const welcome = page.locator('#thread .am-welcome');
      await welcome.getByRole('heading', {name: 'Добро пожаловать в Agent Messenger', exact: true}).waitFor();
      const setup = page.locator('#settings-dialog');
      const projectStep = welcome.locator('.am-step').filter({hasText: 'Укажите папку проекта на этом компьютере.'});
      await projectStep.getByRole('button', {name: 'Указать папку', exact: true}).click();
      await setup.locator('[data-screen=project]').waitFor();
      const projectFolder = setup.locator('#project-page-folder');
      await projectFolder.fill('/fictional/Frame');
      const saving = page.waitForResponse(response => response.url().endsWith('/api/project-folder'));
      await projectFolder.press('Enter');
      await saving;
      await setup.locator('#settings-close').click();
      await welcome.getByText('Frame', {exact: true}).waitFor();
      await welcome.getByRole('button', {name: 'Настроить подключение', exact: true}).click();
      await setup.locator('[data-screen=server]').waitFor();
      await setup.locator('#settings-close').click();
      await page.setViewportSize({width: 600, height: 720});
      await welcome.getByRole('button', {name: 'Настроить агентов', exact: true}).click();
      await setup.locator('[data-screen=agents]').waitFor();
      modelReady = true;
      await page.evaluate(() => window.messengerRefresh());
      assert.equal(await page.locator('#task-draft').isVisible(), false, 'the settings stay open while setup changes');
      await setup.locator('#settings-close').click();
      await page.locator('#task-draft').waitFor();
      await page.locator('#task-head h3').getByText('Новая задача', {exact: true}).waitFor();
      assert.equal(await welcome.count(), 0);
      await page.locator('#task-draft-text').fill('Первая задача');
      assert.equal(await page.locator('#task-draft-send').isEnabled(), true);

      assert.deepEqual(errors, []);
      await context.close();
    }
    console.log(JSON.stringify({engine, retains_previous_conversation: true, empty_project_message: true, first_run_welcome: true}));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
