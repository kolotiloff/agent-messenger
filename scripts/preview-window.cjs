// Serve the real window assets against the browser checks' fictional projects.
const http = require('node:http');
const fs = require('node:fs/promises');
const path = require('node:path');
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

const server = http.createServer(async (request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  if (request.headers.host !== host + ':' + server.address().port) {
    response.writeHead(403).end();
    return;
  }
  if (request.method !== 'GET') {
    response.writeHead(405, { Allow: 'GET' }).end();
    return;
  }
  try {
    const url = new URL(request.url, 'http://' + request.headers.host);
    if (url.pathname.startsWith('/api/')) {
      const project = url.searchParams.get('project') || projects[0].name;
      if (!projects.some(entry => entry.name === project)) {
        response.writeHead(404).end();
        return;
      }
      const fixture = reply(url, project, projects);
      if (url.pathname === '/api/task' && fixture.task) {
        const [{at}] = fixture.items;
        fixture.items.unshift({
          kind:'task',sender_agent:'codex',sender_model:'gpt-6.2-sol[effort=medium]',
          text:'Посмотри, удобно ли читать переписку после выбора задачи в списке компьютеров.',at,
        });
        fixture.items.push(
          {kind:'answer',author:'owner',text:'Проверь, как выглядит список задач в обычном и узком окне. Выбери задачу с длинным названием и убедись, что её можно открыть прямо из списка компьютеров.',at},
          {kind:'tool',title:'Команда',text:'Проверить структуру проекта',at},
          {kind:'tool',title:'Команда',text:'Сверить список файлов',at},
          {kind:'text',text:'Открыл список задач и выбрал задачу на другом компьютере. Чат сразу переключился на нужный проект, а выделение осталось в той группе, где я выбрал задачу.\n\nЗатем сузил окно. Длинные названия и подписи перенеслись на следующую строку, сохранив общий левый край.',at},
          {kind:'tool',title:'Команда',text:'Проверить настройки',at},
          {kind:'tool',title:'Команда',text:'Сверить результаты',at},
          {kind:'text',text:'Вернул окну обычную ширину и открыл полный список справа. Длинные подписи снова поместились на одной строке там, где хватило места.\n\nОтдельно проверил свёрнутый список. Выбранная задача осталась открыта, и переписку можно читать на всю доступную ширину.',at},
        );
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
