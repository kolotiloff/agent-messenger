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
        fixture.items.push(
          {kind:'tool',title:'Команда',text:'Проверить структуру проекта',at},
          {kind:'tool',title:'Команда',text:'Сверить список файлов',at},
          {kind:'text',text:'Проверил структуру проекта и список файлов.',at},
          {kind:'tool',title:'Команда',text:'Проверить настройки',at},
          {kind:'tool',title:'Команда',text:'Сверить результаты',at},
          {kind:'text',text:'Настройки и результаты совпадают. Задачу можно проверить.',at},
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
