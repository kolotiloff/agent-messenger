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
          '/api/client-info': {client: 'running', client_version: '1.0.1-preview.104', outdated: false},
          '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: true}, accept: true, updates: [],
            peers: [{id: 'vps', name: 'VPS', age: 1, agents: [{agent: 'codex'}]}, {id: 'pc2', name: 'ПК-2', age: 1, agents: [{agent: 'codex'}]},
              {id: 'laptop', name: 'Ноутбук', age: 90, agents: []}]},
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
      const headerColor = name => card(name).locator('.am-machine-name').evaluate(head => getComputedStyle(head).backgroundColor);
      const ownHeader = await headerColor('Fixture Mac');
      const peerHeader = await headerColor('ПК-2');
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
      // The local header stays tinted when a peer is selected. Every header
      // has the same padding and online/offline uses both color and shape.
      assert.equal(await card('Fixture Mac').getAttribute('aria-current'), 'true');
      assert.equal(await card('Fixture Mac').getAttribute('aria-pressed'), 'false');
      assert.equal(await card('ПК-2').getAttribute('aria-pressed'), 'true');
      assert.notEqual(ownHeader, peerHeader);
      assert.equal(await headerColor('Fixture Mac'), ownHeader);
      assert.equal(await headerColor('ПК-2'), peerHeader);
      assert.equal(await headerColor('Ноутбук'), peerHeader);
      const bodyColors = await page.locator('#machines .am-machine-card').evaluateAll(cards =>
        cards.map(card => getComputedStyle(card).backgroundColor));
      assert.equal(new Set(bodyColors).size, 1, 'selection changes only the border');
      const spacing = await page.locator('#machines :is(.am-machine-name,.am-machine-body)').evaluateAll(rows => rows.map(head => {
        const style = getComputedStyle(head);
        return [style.paddingTop, style.paddingBottom, style.paddingLeft, style.paddingRight];
      }));
      assert.deepEqual(spacing, Array.from({length: 8}, () => ['4px', '4px', '12px', '12px']));
      const onlineMark = card('ПК-2').getByRole('img', {name: 'На связи'});
      const offlineMark = card('Ноутбук').getByRole('img', {name: 'Нет связи'});
      const online = onlineMark.locator(':scope > svg');
      const offline = offlineMark.locator(':scope > svg');
      assert.equal((await online.boundingBox()).width, 14);
      assert.equal((await offline.boundingBox()).width, 14);
      assert.equal((await online.boundingBox()).height, 14);
      assert.equal((await offline.boundingBox()).height, 14);
      assert.equal(await online.evaluate(svg => getComputedStyle(svg).fill), 'none');
      assert.equal(await offline.evaluate(svg => getComputedStyle(svg).fill), 'none');
      assert.equal(await offline.locator('path').getAttribute('d'), await online.locator('path').getAttribute('d'));
      const dot = onlineMark.locator('.am-machine-signal');
      const cross = offlineMark.locator('.am-machine-signal');
      const geometry = signal => signal.evaluate(node => {
        const style = getComputedStyle(node);
        return [style.width, style.height, style.insetInlineEnd, style.insetBlockEnd];
      });
      assert.deepEqual(await geometry(dot), ['8px', '8px', '-3px', '-1px']);
      assert.deepEqual(await geometry(cross), await geometry(dot));
      assert.equal(await dot.evaluate(node => getComputedStyle(node).backgroundColor),
        scheme === 'dark' ? 'rgb(76, 219, 134)' : 'rgb(22, 132, 76)');
      assert.equal(await cross.evaluate(node => getComputedStyle(node).backgroundColor), 'rgba(0, 0, 0, 0)');
      const crossGlyph = await cross.locator('path').evaluate(path => {
        const style = getComputedStyle(path), stroke = parseFloat(style.strokeWidth);
        const shape = path.getBBox(), scale = path.getScreenCTM();
        return {color: style.stroke, cap: style.strokeLinecap,
          width: (shape.width + stroke) * scale.a, height: (shape.height + stroke) * scale.d};
      });
      assert.equal(crossGlyph.color, scheme === 'dark' ? 'rgb(242, 139, 130)' : 'rgb(180, 35, 24)');
      assert.equal(crossGlyph.cap, 'round');
      assert(Math.abs(crossGlyph.width - 8) < 0.001 && Math.abs(crossGlyph.height - 8) < 0.001);
      assert.equal(await offlineMark.evaluate(mark => getComputedStyle(mark.nextElementSibling).color),
        await offlineMark.evaluate(mark => getComputedStyle(mark).color));
      for (const mark of [onlineMark, offlineMark]) {
        assert.deepEqual(await mark.evaluate(icon => {
          const frame = icon.getBoundingClientRect();
          return [frame.left - icon.parentElement.getBoundingClientRect().left,
            icon.nextElementSibling.getBoundingClientRect().left - frame.right];
        }), [12, 10]);
        assert(await mark.evaluate(icon=>{
          const center=node=>{const box=node.getBoundingClientRect();return box.top+box.height/2;};
          return Math.abs(center(icon.querySelector('svg'))-center(icon.nextElementSibling))<0.5;
        }),'the online and offline icons align with their computer names');
      }
      if (shots) await page.screenshot({path: path.join(shots, `computer-switch-${scheme}.png`)});
      assert.deepEqual(errors, []);
      await context.close();
    }
    console.log(JSON.stringify({engine, computer_switch_without_flash: true}));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
