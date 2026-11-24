// Browser check with fictional HTTP responses only (T-279). No Messenger
// process, model or user profile is opened. With paged history the finished
// tasks of a computer come from the program, so choosing a computer in the
// sidebar must keep the previous list and conversation until they arrive and
// then show that computer's task, never an empty list or an empty pane in
// between.
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
      const task = (id, title, remote) => ({id, project: 'Frame', title, agent: 'codex', state: 'done', created_at: began, started_at: began,
        updated_at: began, finished_at: began, model: 'gpt-6.2-sol[effort=medium]', ...(remote ? {remote} : {})});
      const all = [task('here-1', 'Собрать сцену', null), task('vps-1', 'Проверить сервер', {id: 'vps', name: 'VPS'}),
        task('pc2-1', 'Обновить меню', {id: 'pc2', name: 'ПК-2'})];
      let windowDelay = 0;
      const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
      await context.route('**/*', async route => {
        const url = new URL(route.request().url());
        if (route.request().method() === 'POST') return route.fulfill({json: {ok: true}});
        if (url.pathname === '/api/window') {
          const computer = url.searchParams.get('computer') || '';
          if (windowDelay) await wait(windowDelay);
          // The program filters finished tasks by the chosen computer.
          const tasks = all.filter(t => !computer || (t.remote ? t.remote.id : 'local') === computer);
          return route.fulfill({json: {projects: [{name: 'Frame'}], project: 'Frame', history_paged: true, history_version: computer,
            folder: '/fictional/Frame', messenger: '/fictional/messenger', data_root: '/fictional/data',
            agents: [{agent: 'codex', cli: '/fictional/codex', models: []}], tasks, shared: [],
            coordinator: {connected: true, client: 'codex', active: true}, cursor_link: {state: 'connected'},
            computer_name: 'Fixture Mac', tasks_enabled: true}});
        }
        if (url.pathname === '/api/task') {
          const row = all.find(t => t.id === url.searchParams.get('id'));
          return route.fulfill({json: {task: row, items: [
            {kind: 'task', author: 'coordinator', text: 'Задача: ' + row.title, at: began, sender_agent: 'codex'},
          ]}});
        }
        const fixtures = {
          '/api/client-info': {client: 'running', outdated: false},
          '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: true}, accept: true, updates: [],
            peers: [{id: 'vps', name: 'VPS', age: 1, agents: [{agent: 'codex'}]}, {id: 'pc2', name: 'ПК-2', age: 1, agents: [{agent: 'codex'}]}]},
          '/api/discovery': {agents: []},
          '/api/agent-setup-status': [],
          '/api/self-update': {current: '0.0.0-preview.98'},
          '/api/machine-order': {order: null},
        };
        if (url.pathname.startsWith('/api/')) return route.fulfill({json: fixtures[url.pathname] || {ok: true}});
        const filename = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
        if (!['index.html', 'app.js', 'app.css', 'i18n.js'].includes(filename)) return route.fulfill({status: 404, body: ''});
        await route.fulfill({body: await fs.readFile(path.join(assets, filename)), contentType:
          filename.endsWith('.js') ? 'text/javascript' : filename.endsWith('.css') ? 'text/css' : 'text/html'});
      });
      // Record every empty pane, loading pane and empty task list ever drawn.
      await page.addInitScript(() => {
        window.flashes = [];
        new MutationObserver(() => {
          if (document.querySelector('#thread .am-pane-empty')) window.flashes.push('empty pane');
          if (document.querySelector('#thread .am-pane-loading') && window.watching) window.flashes.push('loading pane');
          const list = document.querySelector('#task-list');
          if (window.watching && list && !list.querySelector('.am-task')) window.flashes.push('empty list');
        }).observe(document, {childList: true, subtree: true});
      });
      await page.goto('http://messenger.test/#fictional');
      await page.locator('#thread').getByText('Задача: Собрать сцену').waitFor();
      const card = name => page.locator('#machines .am-machine-card', {hasText: name});
      await card('ПК-2').waitFor();
      // Straight from one chosen computer to another, then to this one: the
      // list the window holds has none of the next computer's tasks.
      const steps = [['VPS', 600, 'Проверить сервер'], ['ПК-2', 600, 'Обновить меню'], ['Fixture Mac', 600, 'Собрать сцену'],
        ['VPS', 0, 'Проверить сервер'], ['ПК-2', 0, 'Обновить меню']];
      for (const [name, delay, title] of steps) {
        windowDelay = delay;
        await page.evaluate(() => { window.flashes = []; window.watching = true; });
        await card(name).click();
        await page.locator('#thread').getByText('Задача: ' + title).waitFor();
        await page.locator('#task-list .am-task', {hasText: title}).waitFor();
        await page.waitForTimeout(100);
        assert.deepEqual(await page.evaluate(() => [...new Set(window.flashes)]), [], `choosing ${name} (delay ${delay} ms) flashed`);
        await page.evaluate(() => { window.watching = false; });
      }
      // Selecting a peer leaves the local identity marker on this computer.
      const stripe = await page.locator('#machines .am-machine-card').evaluateAll(cards => cards
        .filter(card => getComputedStyle(card, '::before').content === '""')
        .map(card => [card.dataset.machineOrder, card.getAttribute('aria-pressed')]));
      assert.deepEqual(stripe, [['local', 'false']], 'only the local card has the identity stripe');
      if (shots) await page.screenshot({path: path.join(shots, `computer-switch-${scheme}.png`)});
      assert.deepEqual(errors, []);
      await context.close();
    }
    console.log(JSON.stringify({engine, computer_switch_without_flash: true}));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
