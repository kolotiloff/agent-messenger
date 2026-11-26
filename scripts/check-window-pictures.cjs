// Browser check with fictional HTTP responses only (T-268, T-271). No
// Messenger process, model or user profile is opened. A worker's picture
// opens in the viewer, zooms, steps to the next picture and closes; the
// chosen computer is shown only by its card and is dropped when its card
// leaves the list. Requires Playwright. Optional: PLAYWRIGHT_MODULE,
// BROWSER_ENGINE (chromium/webkit).
const {strict: assert} = require('node:assert');
const fs = require('node:fs/promises');
const path = require('node:path');
const zlib = require('node:zlib');

// A plain PNG picture: a task picture is PNG, JPEG, GIF or WebP, never SVG.
function png(width, height, paint) {
  const crcTable = Array.from({length: 256}, (_, n) => { for (let k = 0; k < 8; k++) n = n & 1 ? 0xedb88320 ^ (n >>> 1) : n >>> 1; return n >>> 0; });
  const crc = bytes => { let c = 0xffffffff; for (const b of bytes) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
  const chunk = (type, data) => {
    const body = Buffer.concat([Buffer.from(type, 'latin1'), data]);
    const out = Buffer.alloc(body.length + 8);
    out.writeUInt32BE(data.length, 0); body.copy(out, 4); out.writeUInt32BE(crc(body), body.length + 4);
    return out;
  };
  const rows = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) rows.set(paint(x, y), y * (width * 3 + 1) + 1 + x * 3);
  const head = Buffer.alloc(13);
  head.writeUInt32BE(width, 0); head.writeUInt32BE(height, 4); head[8] = 8; head[9] = 2;
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', head), chunk('IDAT', zlib.deflateSync(rows)), chunk('IEND', Buffer.alloc(0))]);
}
const pictures = [false, true].map(dark => png(1600, 1000, (x, y) => {
  const panel = x >= 80 && x < 500 && y >= 80 && y < 920;
  const circle = (x - 1040) ** 2 + (y - 580) ** 2 < 220 ** 2;
  return dark ? (circle ? [0x5a, 0x7b, 0xd8] : panel ? [0x2b, 0x34, 0x45] : [0x1d, 0x24, 0x30])
    : (circle ? [0x7f, 0xb3, 0x7a] : panel ? [0xff, 0xff, 0xff] : [0xe8, 0xee, 0xf7]);
}));
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
    await page.addInitScript(() => {
      window.amMacTitlebar = true;
      window.webkit = {messageHandlers: {
        titlebar: {postMessage: message => { window.testTitlebar = message; }},
        updates: {postMessage() {}},
      }};
      // Exercise both browsers' ClipboardItem without replacing the owner's clipboard.
      Object.defineProperty(navigator.clipboard, 'write', {value: async ([item]) => {
        if (window.failPictureCopy) throw new Error('Clipboard is unavailable');
        const blob = await item.getType('image/png');
        const bitmap = await createImageBitmap(blob);
        const canvas = document.createElement('canvas');
        canvas.width = canvas.height = 1;
        const context = canvas.getContext('2d');
        context.drawImage(bitmap, 0, 0);
        window.copiedPicture = {width: bitmap.width, height: bitmap.height, type: blob.type,
          pixel: [...context.getImageData(0, 0, 1, 1).data]};
        bitmap.close();
      }});
    });
    const now = Date.now();
    const at = minutes => new Date(now - minutes * 60000).toISOString();
    const tasks = {Frame: [], Site: []};
    for (let i = 0; i < 24; i++) {
      const remote = i % 3 === 0 ? {id: 'dev-1', name: 'Ноут-Linux', worker: 'none'} : undefined;
      for (const project of ['Frame', 'Site'])
        tasks[project].push({id: `${project}-${i}`, project, title: `${project} задача ${i + 1}`, agent: i % 2 ? 'codex' : 'claude',
          state: i < 2 ? 'running' : 'done', created_at: at(10 + i * 7), started_at: at(10 + i * 7), finished_at: i < 2 ? undefined : at(5 + i * 7),
          updated_at: at(5 + i * 7), model: 'gpt-6.2-sol[effort=medium]', ...(remote ? {remote} : {})});
    }
    const usage = tokens => ({tokens: Math.round(tokens), input_tokens: Math.round(tokens * 0.9), output_tokens: Math.round(tokens * 0.1)});
    let withSpare = true;
    const statsComputers = [];
    const peers = () => [{id: 'dev-1', name: 'Ноут-Linux', age: 0, version: '0.0.0-preview.96', agents: [{agent: 'codex'}]},
      ...(withSpare ? [{id: 'dev-2', name: 'ПК-2', age: 0, version: '0.0.0-preview.96', agents: [{agent: 'codex'}]}] : [])];
    const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
    const statRow = (i, computer) => ({id: 'stat-' + i, title: 'Задача статистики ' + (i + 1), at: at(60 * i).slice(0, 19), project: 'Frame', agent: 'codex',
      model: 'gpt-6.2-sol[effort=medium]', computer, ...usage(100000 * (i + 1)), duration_ms: 60000 * (i + 1)});
    await context.route('**/*', async route => {
      // WebKit also routes the window's own blob: pictures; they are not
      // the fixture's to answer.
      if (route.request().url().startsWith('blob:')) return route.continue();
      const url = new URL(route.request().url());
      if (route.request().method() === 'POST') return route.fulfill({json: {ok: true}});
      if (url.pathname === '/api/window') {
        await delay(120);
        const name = url.searchParams.get('project') || 'Frame';
        return route.fulfill({json: {projects: [{name: 'Frame'}, {name: 'Site'}], project: name,
          folder: '/fictional/' + name, messenger: '/fictional/messenger', data_root: '/fictional/data',
          agents: [{agent: 'codex', cli: '/fictional/codex', models: []}, {agent: 'claude', cli: '/fictional/claude', models: []}],
          tasks: tasks[name], shared: [], coordinator: {connected: true, client: 'codex', active: true}, cursor_link: {state: 'connected'},
          computer_name: 'Мак-студия', tasks_enabled: true}});
      }
      if (url.pathname === '/api/task') {
        await delay(120);
        const row = Object.values(tasks).flat().find(t => t.id === url.searchParams.get('id'));
        return route.fulfill({json: {task: row, items: [{kind: 'task', author: 'coordinator', text: 'Задача: ' + row.title, at: row.created_at},
          {kind: 'report', author: 'worker', text: 'Готово, снимки экрана ниже.', at: row.created_at, images: [{id: 'img-1', name: 'окно-светлое.png'}, {id: 'img-2', name: 'окно-тёмное.png'}]}]}});
      }
      if (url.pathname === '/api/task-image') {
        return route.fulfill({body: pictures[url.searchParams.get('id') === 'img-2' ? 1 : 0], contentType: 'image/png'});
      }
      if (url.pathname === '/api/usage-stats') {
        await delay(150);
        const computer = url.searchParams.get('computer');
        statsComputers.push(computer || '');
        const count = computer ? 4 : 9;
        const rows = Array.from({length: count}, (_, i) => statRow(i, computer === 'dev-1' ? 'Ноут-Linux' : 'Мак-студия'));
        const total = rows.reduce((sum, r) => sum + r.tokens, 0);
        return route.fulfill({json: {project: url.searchParams.get('project'), period: url.searchParams.get('period'),
          total: {tasks: count, ...usage(total)},
          agents: [{key: 'codex', ...usage(total * 0.7), tasks: 5}, {key: 'claude', ...usage(total * 0.3), tasks: 4}],
          computers: computer ? [] : [{key: 'local', name: 'Мак-студия', this: true, ...usage(total * 0.6), tasks: 6}, {key: 'dev-1', name: 'Ноут-Linux', ...usage(total * 0.4), tasks: 3}],
          days: Array.from({length: 7}, (_, i) => ({day: at(1440 * i).slice(0, 10), ...usage(1000 * (i + 1) * (computer ? 1 : 2)), tasks: i})),
          recent: rows, top: rows, ranked: {oldest: rows, fewest: rows, longest: rows, shortest: rows}}});
      }
      const fixtures = {
        '/api/client-info': {client: 'running', outdated: false},
        '/api/server': {host: {running: false, enabled: false, addresses: []}, member: {connected: true, reachable: true}, peers: peers(), updates: [], accept: true},
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
    await page.goto('http://127.0.0.1/#fictional');
    await page.locator('.am-queue-list').getByText('Frame задача 3', {exact: true}).waitFor();
    const card = name => page.locator('#machines [data-machine-order]').filter({hasText: name}).first();
    const result = {engine};

    // No chip repeats the chosen computer; its card is pressed instead.
    await card('Ноут-Linux').click();
    await page.waitForFunction(() => document.querySelector('#machines [aria-pressed=true]'));
    assert.equal(await card('Ноут-Linux').getAttribute('aria-pressed'), 'true');
    assert.equal(await page.locator('.am-filter-chip').count(), 0);
    result.no_chip = true;

    // The viewer: open, zoom with the wheel, step on (zoom resets), close.
    await page.locator('.am-queue-list').getByText('Frame задача 4', {exact: true}).click();
    const thumbs = page.locator('.am-picture');
    await thumbs.nth(1).waitFor();
    await page.waitForFunction(() => window.testTitlebar?.zones.length > 0);
    await thumbs.first().click();
    const dialog = page.locator('#picture-dialog');
    await dialog.waitFor();
    await page.waitForFunction(() => document.getElementById('picture-image').naturalWidth > 0).catch(async error => {
      // Say what the viewer and the thumbnails hold, so a failure in one
      // engine names its cause.
      console.error(JSON.stringify(await page.evaluate(() => [...document.querySelectorAll('#picture-image, .am-picture')].map(img =>
        ({id: img.id, src: img.getAttribute('src'), complete: img.complete, natural: [img.naturalWidth, img.naturalHeight], box: [img.clientWidth, img.clientHeight]})))));
      throw error;
    });
    assert.equal(await page.locator('#picture-count').textContent(), '1 из 2');
    assert.equal(await page.locator('#picture-name').textContent(), 'окно-светлое.png');
    const box = await page.locator('#picture-stage').boundingBox();
    const thumb = await thumbs.first().boundingBox();
    assert.ok(box.width > thumb.width * 2, 'the viewer is larger than the thumbnail');
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.wheel(0, -400);
    await page.waitForFunction(() => document.getElementById('picture-zoom-reset').textContent !== '100%');
    assert.equal(await page.locator('#picture-zoom-out').isDisabled(), false);
    const copy = page.locator('#picture-copy');
    const copyStatus = page.locator('#picture-copy-status');
    await copy.click();
    await copyStatus.filter({hasText: /^Скопировано$/}).waitFor();
    assert.deepEqual(await page.evaluate(() => window.copiedPicture),
      {type: 'image/png', width: 1600, height: 1000, pixel: [0xe8, 0xee, 0xf7, 255]});
    await page.locator('#picture-close').focus();
    await page.keyboard.press('ArrowRight');
    assert.equal(await page.locator('#picture-count').textContent(), '2 из 2');
    assert.equal(await page.locator('#picture-zoom-reset').textContent(), '100%');
    assert.equal(await copyStatus.textContent(), '');
    await page.evaluate(() => { window.failPictureCopy = true; });
    await copy.click();
    await copyStatus.filter({hasText: /^Не удалось скопировать$/}).waitFor();
    await page.evaluate(() => { window.failPictureCopy = false; });
    await copy.click();
    await copyStatus.filter({hasText: /^Скопировано$/}).waitFor();
    assert.deepEqual(await page.evaluate(() => window.copiedPicture),
      {type: 'image/png', width: 1600, height: 1000, pixel: [0x1d, 0x24, 0x30, 255]});
    await page.locator('#picture-zoom-in').click();
    assert.equal(await page.locator('#picture-zoom-reset').textContent(), '150%');
    await page.locator('#picture-zoom-reset').click();
    assert.equal(await page.locator('#picture-zoom-reset').textContent(), '100%');
    await page.keyboard.press('Escape');
    await dialog.waitFor({state: 'hidden'});
    await page.waitForFunction(() => window.testTitlebar?.zones.length > 0);

    // The native title bar must pass the click through to the modal. After
    // it closes, dragging the main window from the title bar works again.
    await thumbs.first().click();
    await dialog.waitFor();
    await page.waitForFunction(() => window.testTitlebar?.zones.length === 0);
    await page.locator('#picture-zoom-in').click();
    await page.locator('#picture-close').click();
    await dialog.waitFor({state: 'hidden'});
    await page.waitForFunction(() => window.testTitlebar?.zones.length > 0);
    result.viewer = true;

    // A chosen computer that leaves the list stops filtering.
    await card('ПК-2').click();
    assert.equal(await card('ПК-2').getAttribute('aria-pressed'), 'true');
    withSpare = false;
    await page.locator('[data-tab=server]').click();
    await card('ПК-2').waitFor({state: 'detached', timeout: 12000});
    assert.equal(await page.locator('#machines [aria-pressed=true]').count(), 0);
    statsComputers.length = 0;
    await page.locator('[data-tab=stats]').click();
    await page.locator('#stats-top').getByText('Задача статистики 9', {exact: true}).waitFor();
    assert.ok(statsComputers.length && statsComputers.every(c => c === ''), 'statistics for every computer again: ' + statsComputers);
    result.gone_computer_cleared = true;

    // T-275: a hidden window stops asking for statistics and asks for the
    // network at most once a minute; shown again, it catches up at once.
    let serverAsks = 0;
    page.on('request', request => { if (new URL(request.url()).pathname === '/api/server') serverAsks++; });
    const setHidden = hidden => page.evaluate(hidden => {
      Object.defineProperty(document, 'hidden', {configurable: true, get: () => hidden});
      Object.defineProperty(document, 'visibilityState', {configurable: true, get: () => hidden ? 'hidden' : 'visible'});
      document.dispatchEvent(new Event('visibilitychange'));
    }, hidden);
    await setHidden(true);
    statsComputers.length = 0; serverAsks = 0;
    await delay(16000);
    assert.equal(statsComputers.length, 0, 'a hidden window asked for statistics');
    assert.ok(serverAsks <= 1, 'a hidden window asked for the network ' + serverAsks + ' times');
    await setHidden(false);
    await page.waitForFunction(() => true);
    await delay(500);
    assert.ok(statsComputers.length >= 1 && serverAsks >= 1, 'a shown window catches up');
    result.hidden_window_quiet = true;

    assert.deepEqual(errors, []);
    console.log(JSON.stringify({...result, result: 'passed'}));
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
