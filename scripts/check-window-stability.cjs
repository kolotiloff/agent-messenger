// Browser check with fictional HTTP responses only (T-266). No Messenger
// process, model or user profile is opened. Choosing computers, periods and
// projects must not collapse the statistics or the task list while the new
// data loads, and no scroll bar may appear and vanish again on the way.
// Requires Playwright. Optional: PLAYWRIGHT_MODULE, BROWSER_ENGINE
// (chromium/webkit), REPORT=1 prints every recorded change.
const {strict: assert} = require('node:assert');
const fs = require('node:fs/promises');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assets = path.resolve(__dirname, '../internal/localui/assets');

(async () => {
  const engine = process.env.BROWSER_ENGINE || 'chromium';
  const browser = await playwright[engine].launch({headless: true});
  try {
    const context = await browser.newContext({viewport: {width: 1100, height: 700}});
    const page = await context.newPage();
    page.setDefaultTimeout(5000);
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const now = Date.now();
    const at = minutes => new Date(now - minutes * 60000).toISOString();
    const tasks = {Frame: [], Site: []};
    for (let i = 0; i < 24; i++) {
      const remote = i % 3 === 0 ? {id: 'dev-1', name: 'Runner', worker: 'none'} : undefined;
      for (const project of ['Frame', 'Site'])
        tasks[project].push({id: `${project}-${i}`, project, title: `${project} задача ${i + 1}`, agent: i % 2 ? 'codex' : 'claude',
          state: i < 2 ? 'running' : 'done', created_at: at(10 + i * 7), started_at: at(10 + i * 7), finished_at: i < 2 ? undefined : at(5 + i * 7),
          updated_at: at(5 + i * 7), model: 'gpt-6.2-sol[effort=medium]', ...(remote ? {remote} : {})});
    }
    const usage = tokens => ({tokens: Math.round(tokens), input_tokens: Math.round(tokens * 0.9), output_tokens: Math.round(tokens * 0.1)});
    const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
    const statRow = (i, computer) => ({id: 'stat-' + i, title: 'Задача статистики ' + (i + 1), at: at(60 * i).slice(0, 19), project: 'Frame', agent: 'codex',
      model: 'gpt-6.2-sol[effort=medium]', computer, ...usage(100000 * (i + 1)), duration_ms: 60000 * (i + 1)});
    await context.route('**/*', async route => {
      const url = new URL(route.request().url());
      if (route.request().method() === 'POST') return route.fulfill({json: {ok: true}});
      if (url.pathname === '/api/window') {
        await delay(120);
        const name = url.searchParams.get('project') || 'Frame';
        return route.fulfill({json: {projects: [{name: 'Frame'}, {name: 'Site'}], project: name,
          folder: '/fictional/' + name, messenger: '/fictional/messenger', data_root: '/fictional/data',
          agents: [{agent: 'codex', cli: '/fictional/codex', models: []}, {agent: 'claude', cli: '/fictional/claude', models: []}],
          tasks: tasks[name], shared: [], coordinator: {connected: true, client: 'codex', active: true}, cursor_link: {state: 'connected'},
          computer_name: 'Fixture Mac', tasks_enabled: true}});
      }
      if (url.pathname === '/api/task') {
        await delay(120);
        const row = Object.values(tasks).flat().find(t => t.id === url.searchParams.get('id'));
        return route.fulfill({json: {task: row, items: [{kind: 'task', author: 'coordinator', text: 'Задача: ' + row.title, at: row.created_at}]}});
      }
      if (url.pathname === '/api/usage-stats') {
        await delay(150);
        const computer = url.searchParams.get('computer');
        const count = computer ? 4 : 9;
        const rows = Array.from({length: count}, (_, i) => statRow(i, computer === 'dev-1' ? 'Runner' : 'Fixture Mac'));
        const total = rows.reduce((sum, r) => sum + r.tokens, 0);
        return route.fulfill({json: {project: url.searchParams.get('project'), period: url.searchParams.get('period'),
          total: {tasks: count, ...usage(total)},
          agents: [{key: 'codex', ...usage(total * 0.7), tasks: 5}, {key: 'claude', ...usage(total * 0.3), tasks: 4}],
          computers: computer ? [] : [{key: 'local', name: 'Fixture Mac', this: true, ...usage(total * 0.6), tasks: 6}, {key: 'dev-1', name: 'Runner', ...usage(total * 0.4), tasks: 3}],
          days: Array.from({length: 7}, (_, i) => ({day: at(1440 * i).slice(0, 10), ...usage(1000 * (i + 1) * (computer ? 1 : 2)), tasks: i})),
          recent: rows, top: rows, ranked: {oldest: rows, fewest: rows, longest: rows, shortest: rows}}});
      }
      const fixtures = {
        '/api/client-info': {client: 'running', outdated: false},
        '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: true, reachable: true}, peers: [
          {id: 'dev-1', name: 'Runner', age: 0, version: '0.0.0-preview.96', agents: [{agent: 'codex'}]}], updates: [], accept: true},
        '/api/discovery': {agents: []},
        '/api/agent-setup-status': [],
        '/api/self-update': {current: '0.0.0-preview.96'},
        '/api/machine-order': {order: null},
      };
      if (url.pathname.startsWith('/api/')) return route.fulfill({json: fixtures[url.pathname] || {ok: true}});
      const filename = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
      if (!['index.html', 'app.js', 'app.css', 'i18n.js'].includes(filename)) return route.fulfill({status: 404, body: ''});
      await route.fulfill({body: await fs.readFile(path.join(assets, filename)), contentType:
        filename.endsWith('.js') ? 'text/javascript' : filename.endsWith('.css') ? 'text/css' : 'text/html'});
    });
    // Every animation frame, record the height of the content in each
    // scrolled region, whether it overflows, and the position of key blocks.
    await page.addInitScript(() => {
      window.changes = [];
      const watched = () => [...document.querySelectorAll('.am-screen:not([hidden]), .am-queue-list, .am-thread')]
        .filter(node => node.getClientRects().length);
      const describe = node => node.dataset.screen || node.id || node.className.split(' ').slice(0, 2).join('.');
      let last = new Map();
      const sample = () => {
        const next = new Map();
        for (const node of watched()) {
          const key = describe(node);
          next.set(key, {bar: node.scrollHeight > node.clientHeight + 1, height: node.scrollHeight});
          const before = last.get(key);
          if (before && before.bar !== next.get(key).bar) window.changes.push({kind: 'scrollbar', key, to: next.get(key).bar, t: performance.now()});
          if (before && next.get(key).height < before.height * 0.6) window.changes.push({kind: 'collapse', key, from: before.height, to: next.get(key).height, t: performance.now()});
        }
        last = next;
        requestAnimationFrame(sample);
      };
      requestAnimationFrame(sample);
    });
    await page.goto('http://messenger.test/#fictional');
    await page.locator('.am-queue-list').getByText('Frame задача 1', {exact: true}).waitFor();
    const settle = () => page.waitForTimeout(600);
    const card = name => page.locator('#machines [data-machine-order]').filter({hasText: name}).first();
    const results = {};
    const step = async (label, action) => {
      await settle();
      await page.evaluate(() => { window.changes = []; });
      await action();
      await settle();
      results[label] = await page.evaluate(() => window.changes);
    };

    await step('tasks: choose Runner', () => card('Runner').click());
    await step('tasks: all computers again', () => card('Runner').click());
    await step('tasks: switch project', async () => {
      await page.locator('[aria-controls="project-select-menu"]').click();
      await page.locator('#project-select-menu').getByRole('option', {name: 'Site', exact: true}).click();
    });
    await page.locator('[data-tab=stats]').click();
    await page.locator('#stats-top').getByText('Задача статистики 9', {exact: true}).waitFor();
    await step('stats: choose Runner', () => card('Runner').click());
    await step('stats: all computers again', () => card('Runner').click());
    await step('stats: choose this computer', () => card('Fixture Mac').click());
    await step('stats: all computers', () => card('Fixture Mac').click());
    await step('stats: period day', () => page.locator('[data-period=day]').click());
    await step('stats: period month', () => page.locator('[data-period=month]').click());
    await step('stats: switch project', async () => {
      await page.locator('[aria-controls="project-select-menu"]').click();
      await page.locator('#project-select-menu').getByRole('option', {name: 'Frame', exact: true}).click();
    });

    await page.locator('[data-tab=agents]').click();
    await step('agents: choose Runner', () => card('Runner').click());
    await step('agents: all computers again', () => card('Runner').click());
    await page.locator('[data-tab=server]').click();
    await step('server: open again', async () => {
      await page.locator('[data-tab=tasks]').click();
      await page.locator('[data-tab=server]').click();
    });
    await step('tasks: back to tasks', () => page.locator('[data-tab=tasks]').click());

    if (process.env.REPORT) for (const [label, list] of Object.entries(results)) console.log(label, JSON.stringify(list));
    // A bar that appears or goes once because the content changed keeps the
    // layout (its gutter is reserved). A bar that goes and comes back within
    // one action, or a block that collapses, is a jump.
    const jumps = list => list.filter(c => c.kind === 'collapse' ||
      list.some(o => o !== c && o.kind === 'scrollbar' && o.key === c.key && o.to !== c.to));
    const bad = Object.entries(results).map(([label, list]) => [label, jumps(list)]).filter(([, list]) => list.length);
    assert.deepEqual(errors, []);
    assert.deepEqual(bad.map(([label, list]) => `${label}: ${list.map(c => c.kind + ' ' + c.key).join(', ')}`), [], 'blocks jumped');
    console.log(JSON.stringify({engine, steps: Object.keys(results).length, result: 'passed'}));
    await context.close();
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
