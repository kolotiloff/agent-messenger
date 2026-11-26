// Serve the real window assets against the browser checks' fictional projects.
const http = require('node:http');
const fs = require('node:fs/promises');
const path = require('node:path');
const { json } = require('node:stream/consumers');
const { randomUUID } = require('node:crypto');
const { projects, reply } = require('./check-window-project-split.cjs');

const host = '127.0.0.1';
const assets = path.resolve(__dirname, '../internal/localui/assets');
const files = new Map([
  ['/', ['index.html', 'text/html; charset=utf-8']],
  ['/app.js', ['app.js', 'text/javascript; charset=utf-8']],
  ['/i18n.js', ['i18n.js', 'text/javascript; charset=utf-8']],
  ['/app.css', ['app.css', 'text/css; charset=utf-8']],
  ['/icon.svg', ['icon.svg', 'image/svg+xml']],
]);

const conversations = {
  'game-1': {
    request: 'Собери экран настроек игры. Нужны громкость музыки, громкость эффектов и переключатель субтитров.',
    commands: ['Открыть макет экрана настроек', 'Проверить сохранение громкости'],
    progress: 'Разместил ползунки и переключатель в одном блоке. Добавил подписи, чтобы было сразу понятно, за что отвечает каждый ползунок.',
    followup: 'Подписи читаются и в узком окне. Теперь проверяю, что после перезапуска игры громкость и субтитры восстанавливаются.',
  },
  'game-2': {
    request: 'Проверь навигацию по главному меню с клавиатуры. После выхода из настроек фокус должен вернуться на кнопку, с которой игрок вошёл.',
    commands: ['Пройти меню клавишами', 'Проверить возврат фокуса'],
    progress: 'Со стрелками всё работает. Нашёл один сбой: после выхода из настроек фокус перескакивает на первую кнопку меню.',
    followup: 'Возврат на исходную кнопку исправил. Подтверди, пожалуйста, что в подменю управления клавиша Escape тоже должна возвращать на предыдущий экран.',
  },
  'game-3': {
    request: 'Добавь звуки наведения и нажатия в главное меню. Звук наведения должен быть тише звука нажатия.',
    commands: ['Прослушать звуки меню', 'Проверить громкость в настройках'],
    progress: 'Подключил короткий звук наведения и отдельный звук подтверждения. Оба подчиняются общей громкости эффектов.',
    followup: 'Проверил мышь и клавиатуру. При быстром перемещении по меню звуки не накладываются друг на друга. Задача готова.',
  },
  'game-4': {
    request: 'Проверь экран паузы, пока настраивается основное меню. При возврате в игру таймер должен продолжиться с прежней отметки.',
    commands: ['Открыть экран паузы', 'Сверить показания таймера'],
    progress: 'Поставил игру на паузу на отметке 42 секунды. При открытии меню таймер остановился, а после возврата продолжил отсчёт с той же отметки.',
    followup: 'Проверяю повторную паузу и возврат с клавиатуры. Хочу убедиться, что таймер не перескакивает после нескольких открытий подряд.',
  },
  'site-1': {
    request: 'Подготовь главную страницу сайта. В первом экране нужны описание проекта, кнопка скачивания и ссылка на документацию.',
    commands: ['Собрать первый экран сайта', 'Проверить мобильную ширину'],
    progress: 'Собрал первый экран и блок со ссылками. На телефоне кнопка скачивания остаётся видимой, а описание переносится целиком.',
    followup: 'Сейчас проверяю ссылки и заголовки. Затем открою страницу на широком экране и проверю, что текст остаётся удобным для чтения.',
  },
  'tools-1': {
    request: 'Добавь выгрузку еженедельного отчёта в CSV. В отчёте нужны даты, названия задач и время работы.',
    commands: ['Собрать пробный отчёт', 'Проверить CSV с русскими названиями'],
    progress: 'Выгрузка уже собирает строки за выбранную неделю. Пробный файл открывается в таблице, русские названия читаются.',
    followup: 'Остался вопрос про незавершённые задачи. Добавляем их с текущим временем работы или включаем в выгрузку только завершённые?',
  },
  'tools-2': {
    request: 'Проверь комплект файлов перед передачей сборки. Сверь список с манифестом и выпиши всё, чего не хватает.',
    commands: ['Сравнить папку сборки с манифестом', 'Сверить контрольные суммы'],
    progress: 'Список файлов совпал с манифестом. Два файла конфигурации отличаются от предыдущей сборки, оба входят в этот выпуск.',
    followup: 'Проверяю контрольные суммы архива. После сверки приложу список файлов и отмечу время проверки.',
  },
};

let machineOrder = null;
const newTasks = new Map();
const previewTask = ({ task, finish }) => Date.now() >= finish
  ? { ...task, state: 'done', finished_at: new Date(finish).toISOString() }
  : task;

const server = http.createServer(async (request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  if (request.headers.host !== host + ':' + server.address().port) {
    response.writeHead(403).end();
    return;
  }
  try {
    const url = new URL(request.url, 'http://' + request.headers.host);
    if (request.method === 'POST' && url.pathname === '/api/task-start') {
      const length = Number(request.headers['content-length']);
      if (request.headers['content-type'] !== 'application/json' || !(length > 0 && length <= 262144)) {
        response.writeHead(400).end();
        return;
      }
      const body = await json(request).catch(() => null);
      if (!projects.some(p => p.name === body?.project) || typeof body.text !== 'string' ||
          !body.text.trim() || body.text.length > 20000 || typeof body.op_id !== 'string' || !body.op_id) {
        response.writeHead(400).end();
        return;
      }
      const folder = projects.find(p => p.name === body.project).folder;
      const { peers } = reply(new URL('/api/server', url), body.project, projects);
      const peer = peers.find(p => p.id === body.computer && p.ready?.projects.includes(body.project));
      const agents = body.computer ? peer?.agents : reply(new URL('/api/window', url), body.project, projects).agents;
      if (!agents?.some(a => a.agent === body.agent && !a.paused && (!body.model || a.models?.includes(body.model)))) {
        response.writeHead(400).end();
        return;
      }

      const existing = newTasks.get(body.op_id);
      if (existing && JSON.stringify(existing.request) !== JSON.stringify(body)) {
        response.writeHead(409).end();
        return;
      }
      if (!existing) {
        const at = new Date().toISOString();
        const task = {
          id: 'preview-' + randomUUID(), project: body.project, folder,
          agent: body.agent, title: body.text.split('\n')[0].slice(0, 80), state: 'running',
          ran_model: body.model, created_at: at, started_at: at, worker_observed: true,
          ...(peer ? { remote: { id: peer.id, name: peer.name } } : {}),
        };
        newTasks.set(body.op_id, { task, request: body, finish: Date.now() + 8000 });
      }
      response.writeHead(200, { 'Content-Type': 'application/json' })
        .end(JSON.stringify({ ok: true, result: newTasks.get(body.op_id).task.id }));
      return;
    }
    if (url.pathname === '/api/self-update') {
      response.setHeader('Content-Type', 'application/json; charset=utf-8');
      if (request.method === 'GET') {
        response.writeHead(200).end(JSON.stringify({ installed_version: '1.0.9', version: '1.0.10', state: 'available' }));
      } else if (request.method === 'POST') {
        response.writeHead(409).end(JSON.stringify({ error: 'Это пример обновления в макете.' }));
      } else {
        response.writeHead(405, { Allow: 'GET, POST' }).end();
      }
      return;
    }
    if (request.method === 'POST' && url.pathname === '/api/machine-order') {
      const length = Number(request.headers['content-length']);
      if (request.headers['content-type'] !== 'application/json' || !(length > 0 && length <= 4096)) {
        response.writeHead(400).end();
        return;
      }
      const body = await json(request).catch(() => null);
      if (!Array.isArray(body?.order) || !body.order.every(key => typeof key === 'string')) {
        response.writeHead(400).end();
        return;
      }
      machineOrder = body.order;
      response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ ok: true }));
      return;
    }
    if (request.method !== 'GET') {
      response.writeHead(405, { Allow: 'GET, POST' }).end();
      return;
    }
    if (url.pathname.startsWith('/api/')) {
      const project = url.searchParams.get('project') || projects[0].name;
      if (!projects.some(entry => entry.name === project)) {
        response.writeHead(404).end();
        return;
      }
      const custom = url.pathname === '/api/task' && [...newTasks.values()].find(entry => entry.task.id === url.searchParams.get('id'));
      const fixture = custom ? { task: previewTask(custom), items: [
        { kind: 'task', author: 'owner', text: custom.request.text, at: custom.task.created_at },
      ] } : url.pathname === '/api/machine-order' ? { order: machineOrder } : reply(url, project, projects);
      if (custom && fixture.task.finished_at) {
        fixture.items.push({ kind: 'report', text: 'Это пример ответа в предпросмотре. Задача завершена.', verified: true, audience: 'owner', at: fixture.task.finished_at });
      }
      if (url.pathname === '/api/window' || url.pathname === '/api/project-activity') {
        const created = [...newTasks.values()].filter(entry => entry.task.project === project).map(previewTask).reverse();
        fixture.tasks = [...created, ...fixture.tasks];
      }
      const conversation = url.pathname === '/api/task' && conversations[fixture.task?.id];
      if (conversation) {
        const [{ at }] = fixture.items;
        const state = fixture.task.state;
        const disposition = state === 'waiting_owner' ? 'question' : state === 'done' ? 'report' : null;
        const [firstCommand, secondCommand] = conversation.commands;
        fixture.items = [
          { kind: 'task', sender_agent: 'codex', sender_model: 'gpt-6.2-sol[effort=medium]', text: conversation.request, at },
          { kind: 'tool', title: 'Команда', text: firstCommand, at },
          { kind: 'text', text: conversation.progress, at },
          { kind: 'tool', title: 'Команда', text: secondCommand, at },
          disposition
            ? { kind: disposition, text: conversation.followup, verified: true, audience: 'owner', at }
            : { kind: 'text', text: conversation.followup, at },
        ];
      }
      const body = JSON.stringify(fixture);
      response.writeHead(200, { 'Content-Type': 'application/json' }).end(body);
      return;
    }
    const asset = files.get(url.pathname);
    if (!asset) {
      response.writeHead(404).end();
      return;
    }
    const [file, contentType] = asset;
    let body = await fs.readFile(path.join(assets, file), 'utf8');
    if (file === 'index.html') body = body.replace('id="am-concept"', 'id="am-concept" class="am-task-sidebar-preview"');
    response.writeHead(200, { 'Content-Type': contentType }).end(body);
  } catch (error) {
    console.error(error.message);
    response.writeHead(500).end();
  }
});

server.listen(Number(process.argv[2] || 0), host, () => {
  console.log('http://' + host + ':' + server.address().port + '/#preview-only');
});
