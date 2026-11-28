// Serve the real window assets against the browser checks' fictional projects.
const http = require('node:http');
const fs = require('node:fs/promises');
const path = require('node:path');
const { json } = require('node:stream/consumers');
const { randomUUID } = require('node:crypto');
const { projects, reply } = require('./check-window-project-split.cjs');
const modelCatalog = require('../internal/modelcatalog/catalog.json');

const host = '127.0.0.1';
const assets = path.resolve(__dirname, '../internal/localui/assets');
const previewExamples = process.argv[3] ? fs.readFile(process.argv[3], 'utf8').then(JSON.parse) : Promise.resolve({});
const files = new Map([
  ['/', ['index.html', 'text/html; charset=utf-8']],
  ['/app.js', ['app.js', 'text/javascript; charset=utf-8']],
  ['/i18n.js', ['i18n.js', 'text/javascript; charset=utf-8']],
  ['/app.css', ['app.css', 'text/css; charset=utf-8']],
  ['/file-viewer.js', ['file-viewer.js', 'text/javascript; charset=utf-8']],
  ['/file-viewer.css', ['file-viewer.css', 'text/css; charset=utf-8']],
  ['/icon.svg', ['icon.svg', 'image/svg+xml']],
  ['/fonts/Inter.woff2', ['fonts/Inter.woff2', 'font/woff2']],
  ['/fonts/SourceSans3.woff2', ['fonts/SourceSans3.woff2', 'font/woff2']],
]);

const conversations = {
  'game-1': {
    request: 'Собери экран настроек игры. Нужны громкость музыки, громкость эффектов и переключатель субтитров.',
    commands: [['Открыть макет экрана настроек', 'cat docs/settings-screen.md'], ['Проверить сохранение громкости', 'npm test -- settings']],
    progress: 'Разместил ползунки и переключатель в одном блоке. Добавил подписи, чтобы было сразу понятно, за что отвечает каждый ползунок.',
    followup: 'Подписи читаются и в узком окне. Теперь проверяю, что после перезапуска игры громкость и субтитры восстанавливаются.',
  },
  'game-2': {
    request: 'Проверь навигацию по главному меню с клавиатуры. После выхода из настроек фокус должен вернуться на кнопку, с которой игрок вошёл.',
    commands: [['Проверить управление с клавиатуры', 'npm test -- keyboard-navigation'], ['Проверить возврат фокуса', 'npm test -- focus-restore']],
    progress: 'Со стрелками всё работает. Нашёл один сбой: после выхода из настроек фокус перескакивает на первую кнопку меню.',
    followup: 'Возврат на исходную кнопку исправил. Подтверди, пожалуйста, что в подменю управления клавиша Escape тоже должна возвращать на предыдущий экран.',
  },
  'game-3': {
    request: 'Добавь звуки наведения и нажатия в главное меню. Звук наведения должен быть тише звука нажатия.',
    commands: [['Прослушать звуки меню', 'ffplay -autoexit assets/audio/menu-confirm.wav'], ['Проверить громкость в настройках', 'npm test -- menu-volume']],
    progress: 'Подключил короткий звук наведения и отдельный звук подтверждения. Оба подчиняются общей громкости эффектов.',
    followup: 'Проверил мышь и клавиатуру. При быстром перемещении по меню звуки не накладываются друг на друга. Задача готова.',
  },
  'game-4': {
    request: 'Проверь экран паузы, пока настраивается основное меню. При возврате в игру таймер должен продолжиться с прежней отметки.',
    commands: [['Открыть код экрана паузы', 'cat src/pause-screen.js'], ['Проверить показания таймера', 'npm test -- pause-timer']],
    progress: 'Поставил игру на паузу на отметке 42 секунды. При открытии меню таймер остановился, а после возврата продолжил отсчёт с той же отметки.',
    followup: 'Проверяю повторную паузу и возврат с клавиатуры. Хочу убедиться, что таймер не перескакивает после нескольких открытий подряд.',
  },
  'site-1': {
    request: 'Подготовь главную страницу сайта. В первом экране нужны описание проекта, кнопка скачивания и ссылка на документацию.',
    commands: [['Собрать сайт', 'npm run build'], ['Проверить мобильную ширину', 'npm test -- mobile-layout']],
    progress: 'Собрал первый экран и блок со ссылками. На телефоне кнопка скачивания остаётся видимой, а описание переносится целиком.',
    followup: 'Сейчас проверяю ссылки и заголовки. Затем открою страницу на широком экране и проверю, что текст остаётся удобным для чтения.',
  },
  'tools-1': {
    request: 'Добавь выгрузку еженедельного отчёта в CSV. В отчёте нужны даты, названия задач и время работы.',
    commands: [['Собрать пробный отчёт', 'node scripts/export-weekly-report.js'], ['Проверить CSV с русскими названиями', 'npm test -- csv-encoding']],
    progress: 'Выгрузка уже собирает строки за выбранную неделю. Пробный файл открывается в таблице, русские названия читаются.',
    followup: 'Остался вопрос про незавершённые задачи. Добавляем их с текущим временем работы или включаем в выгрузку только завершённые?',
  },
  'tools-2': {
    request: 'Проверь комплект файлов перед передачей сборки. Сверь список с манифестом и выпиши всё, чего не хватает.',
    commands: [['Сверить список файлов сборки с манифестом', 'diff -u manifest.txt build-files.txt'], ['Сверить контрольные суммы', 'shasum -a 256 -c checksums.txt']],
    progress: 'Список файлов совпал с манифестом. Два файла конфигурации отличаются от предыдущей сборки, оба входят в этот выпуск.',
    followup: 'Проверяю контрольные суммы архива. После сверки приложу список файлов и отмечу время проверки.',
  },
};

let machineOrder = null;
const newTasks = new Map();
const notes = new Map();
const hiddenArchive = new Set();
const coordinatorModels = Object.fromEntries(
  reply(new URL('http://127.0.0.1/api/window'), projects[0].name, projects).agents.map(agent => [agent.agent, agent.models]));
const previewTask = ({ task, finish }) => Date.now() >= finish
  ? { ...task, state: 'done', finished_at: new Date(finish).toISOString(), can_continue:true }
  : task;

// Opt-in display data stays outside the repository and never supplies routing IDs.
function withPreviewExample(task, examples) {
  const example = examples[task.id];
  if (!example) return task;
  const { title, agent, ran_model, created_at, started_at, finished_at } = example;
  return {
    ...task,
    ...(title && { title }),
    ...(agent && { agent }),
    ...(ran_model && { ran_model }),
    ...(created_at && { created_at }),
    ...(started_at && { started_at }),
    ...(finished_at && { finished_at }),
  };
}

// Fictional results for exercising all report filters in the same preview.
const sampleResults = [
  ['game','game-3','Звуки меню','pc2','ПК-2','codex','gpt-6.2-sol[effort=medium]',0,4100000,1200000,200000,12,true],
  ['tools','tools-done','Готовый отчёт','local','Мак','claude','claude-opus-4-8',0,180000,84000,12000,33,true],
  ['tools','tools-old','Старый отчёт','linux','Ноут-Linux','codex','gpt-6.2-astra[effort=high]',2,92000,40000,3200,9,false],
].map(([project,id,title,key,computer,agent,model,days,input,cached,output,minutes,accepted])=>{
  const at=new Date(Date.now()-(days*24*60+25)*60000);
  return {project,id,title,key,computer,agent,model,at:at.toISOString(),duration_ms:minutes*60000,
    accepted,input_tokens:input,cached_tokens:cached,output_tokens:output,tokens:input+output};
});
const previewCompleted = row => {
  const started=new Date(Date.parse(row.at)-row.duration_ms).toISOString();
  return {...row,folder:projects.find(p=>p.name===row.project).folder,state:'done',can_continue:true,
    ran_model:row.model,created_at:started,started_at:started,finished_at:row.at,usage:row,
    ...(row.key==='local'?{}:{remote:{id:row.key,name:row.computer}})};
};
function previewWindowTask(task){
  const followups=notes.get(task.id);
  if(followups?.size){
    const {item}=Array.from(followups.values()).at(-1);
    task={...task,updated_at:item.at};
  }
  if (task.state==='done') return {...task,can_continue:true};
  if (task.state!=='waiting_owner'||!Object.hasOwn(conversations,task.id)) return task;
  return followups?.size ? {...task,state:'running'}
    : {...task,owner_question:{id:'q-'+task.id,text:conversations[task.id].followup}};
}
const totals=rows=>rows.reduce((sum,row)=>{
  sum.tasks++;
  for(const key of ['tokens','input_tokens','cached_tokens','output_tokens'])sum[key]+=row[key];
  return sum;
},{tasks:0,tokens:0,input_tokens:0,cached_tokens:0,output_tokens:0});
function reportInputs(params){
  const names=params.getAll('projects'), project=params.get('project')||projects[0].name;
  const computer=params.get('computer')||'',agent=params.get('agent')||'',model=params.get('model')||'';
  const since=new Date();
  since.setHours(0,0,0,0);
  since.setDate(since.getDate()-({day:0,week:6,month:29}[params.get('period')]??6));
  const scope=sampleResults.filter(row=>(names.length?names.includes(row.project):row.project===project) &&
    Date.parse(row.at)>=since && (!computer||computer===row.key) && (!agent||agent===row.agent));
  return {projects:names.length?{projects:names}:{project}, computer, agent, model, scope,
    rows:scope.filter(row=>!model||row.model===model)};
}
function previewReport(url){
  const {projects,computer,agent,model,scope,rows}=reportInputs(url.searchParams);
  if(url.pathname==='/api/usage-efficiency'){
    const sample=value=>({samples:1,mean:value,median:value,p90:value});
    const groups=rows.map(row=>({project:url.searchParams.has('projects')?row.project:undefined,
      agent:row.agent,model:row.model,task_type:'feature',difficulty:'medium',tasks:1,
      reviewed:Number(row.accepted),accepted:Number(row.accepted),first_pass_known:Number(row.accepted),
      first_pass_accepted:Number(row.accepted),
      ...(row.accepted?{
        accepted_new_input_plus_output:sample(row.tokens-row.cached_tokens),
        accepted_active_runtime_ms:sample(row.duration_ms),time_to_acceptance_ms:sample(row.duration_ms+60000)
      }:{})}));
    return {...projects,groups,total_groups:groups.length};
  }
  const by=(items,key,name=key)=>Array.from(new Set(items.map(row=>row[key])),value=>{
    const matches=items.filter(row=>row[key]===value);
    return {key:value,name:matches[0][name],this:value==='local',...totals(matches)};
  });
  const sorted=(key,sign)=>[...rows].sort((a,b)=>sign*(a[key]-b[key]));
  const days=rows.map(row=>({...row,day:row.at.slice(0,10)}));
  return {...projects,computer,agent,model,total:totals(rows),agents:by(rows,'agent'),computers:by(rows,'key','computer'),
    models:by(scope,'model'),days:by(days,'day').map(row=>({...row,day:row.key})),
    recent:[...rows].sort((a,b)=>b.at.localeCompare(a.at)),top:sorted('tokens',-1),
    ranked:{oldest:[...rows].sort((a,b)=>a.at.localeCompare(b.at)),fewest:sorted('tokens',1),
      longest:sorted('duration_ms',-1),shortest:sorted('duration_ms',1)}};
}

const server = http.createServer(async (request, response) => {
  response.setHeader('Cache-Control', 'no-store');
  if (request.headers.host !== host + ':' + server.address().port) {
    response.writeHead(403).end();
    return;
  }
  try {
    const url = new URL(request.url, 'http://' + request.headers.host);
    if (url.pathname === '/api/model-catalog' && request.method === 'GET') {
      const provider = url.searchParams.get('provider');
      if (!Object.hasOwn(modelCatalog, provider)) {
        response.writeHead(400).end();
        return;
      }
      response.writeHead(200, { 'Content-Type': 'application/json' })
        .end(JSON.stringify({ models: modelCatalog[provider], limit: 12 }));
      return;
    }
    if (url.pathname === '/api/agent-models' && request.method === 'POST') {
      const body = await json(request).catch(() => null);
      if (!Object.hasOwn(modelCatalog, body?.provider) || !Array.isArray(body.models) ||
          body.models.length > 12 || !body.models.every(model => typeof model === 'string' && model.length <= 80)) {
        response.writeHead(400).end();
        return;
      }
      coordinatorModels[body.provider] = [...new Set(body.models)];
      response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ ok: true }));
      return;
    }
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
      if (!agents?.some(a => a.agent === body.agent && !a.paused && coordinatorModels[a.agent]?.includes(body.model))) {
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
    if (request.method === 'POST' && ['/api/task-note','/api/task-answer'].includes(url.pathname)) {
      const length=Number(request.headers['content-length']);
      const body=length>0&&length<=262144?await json(request).catch(()=>null):null;
      if ((!Object.hasOwn(conversations,body?.task)&&!sampleResults.some(row=>row.id===body?.task)&&
          ![...newTasks.values()].some(entry=>entry.task.id===body?.task)) ||
          typeof body?.text!=='string'||!body.text.trim()||typeof body.op_id!=='string'||!body.op_id) {
        response.writeHead(400).end();return;
      }
      const thread=notes.get(body.task)||new Map();
      const existing=thread.get(body.op_id);
      if(existing&&existing.item.text!==body.text){response.writeHead(409).end();return;}
      if(!existing){
        const custom=[...newTasks.values()].find(entry=>entry.task.id===body.task);
        const task=custom?previewTask(custom):Object.hasOwn(conversations,body.task)
          ?reply(new URL('/api/task?id='+encodeURIComponent(body.task),url),projects[0].name,projects).task
          :null;
        const ongoing=url.pathname==='/api/task-note'&&task?.state==='running';
        thread.set(body.op_id,{
          ongoing,
          item:{kind:'note',author:'owner',text:body.text,at:new Date().toISOString()},
        });
      }
      notes.set(body.task,thread);
      response.writeHead(200,{'Content-Type':'application/json'}).end(JSON.stringify({ok:true}));
      return;
    }
    if (request.method==='POST'&&['/api/task-hide','/api/task-hide-history','/api/usage-export'].includes(url.pathname)) {
      const length=Number(request.headers['content-length']);
      const body=length>0&&length<=262144?await json(request).catch(()=>null):null;
      if(!body||typeof body!=='object'){response.writeHead(400).end();return;}
      if(url.pathname==='/api/task-hide'&&typeof body.task==='string')hiddenArchive.add(body.task);
      if(url.pathname==='/api/task-hide-history'){
        for(const row of sampleResults){
          if(row.project===body.project&&(!body.agent||row.agent===body.agent)&&(!body.computer||row.key===body.computer))hiddenArchive.add(row.id);
        }
        for(const entry of newTasks.values()){
          const task=previewTask(entry);
          if(task.state==='done'&&task.project===body.project&&(!body.agent||task.agent===body.agent)&&
             (!body.computer||(task.remote?.id||'local')===body.computer))hiddenArchive.add(task.id);
        }
      }
      if(url.pathname==='/api/usage-export'){
        response.writeHead(409,{'Content-Type':'application/json'}).end(JSON.stringify({error:'В предпросмотре используются примеры. Экспорт в файл доступен в программе.'}));
        return;
      }
      response.writeHead(200,{'Content-Type':'application/json'}).end(JSON.stringify({ok:true}));
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
      if (url.pathname === '/api/usage-stats' || url.pathname === '/api/usage-efficiency') {
        response.writeHead(200,{'Content-Type':'application/json'}).end(JSON.stringify(previewReport(url)));
        return;
      }
      const custom = url.pathname === '/api/task' && [...newTasks.values()].find(entry => entry.task.id === url.searchParams.get('id'));
      const sample = url.pathname === '/api/task' && !Object.hasOwn(conversations,url.searchParams.get('id')) &&
        sampleResults.find(row=>row.id===url.searchParams.get('id'));
      const fixture = custom ? { task: previewTask(custom), items: [
        { kind: 'task', author: 'owner', text: custom.request.text, at: custom.task.created_at },
      ] } : sample ? {task:previewCompleted(sample),items:[
        {kind:'task',author:'owner',text:sample.title,at:sample.at},
        {kind:'report',audience:'owner',verified:true,text:'Готово. Проверил результат и сохранил отчёт.',at:sample.at},
      ]} : url.pathname === '/api/machine-order' ? { order: machineOrder } : reply(url, project, projects);
      if (custom && fixture.task.finished_at) {
        fixture.items.push({ kind: 'report', text: 'Это пример ответа в предпросмотре. Задача завершена.', verified: true, audience: 'owner', at: fixture.task.finished_at });
      }
      if (url.pathname === '/api/window' || url.pathname === '/api/project-activity') {
        const created = [...newTasks.values()].filter(entry => entry.task.project === project).map(previewTask).reverse();
        fixture.tasks = [...created, ...fixture.tasks];
      }
      if (url.pathname === '/api/window') fixture.coordinator_models = coordinatorModels;
      if (url.pathname === '/api/server') {
        for (const peer of fixture.peers) peer.version += '-preview.108';
        fixture.peers.push({id:'offline',name:'Ноут-Windows',age:600,last_seen:new Date(Date.now()-600000).toISOString(),
          ready:{projects:[]},agents:[]});
      }
      const conversation = url.pathname === '/api/task' && conversations[fixture.task?.id];
      if (conversation) {
        const [{ at }] = fixture.items;
        const state = fixture.task.state;
        const disposition = state === 'waiting_owner' ? 'question' : state === 'done' ? 'report' : null;
        const [firstCommand, secondCommand] = conversation.commands.map(([description,text])=>({ kind: 'tool', title: 'Команда', description, text, at }));
        fixture.items = [
          { kind: 'task', sender_agent: 'codex', sender_model: 'gpt-6.2-sol[effort=medium]', text: conversation.request, at },
          firstCommand,
          { kind: 'text', text: conversation.progress, at },
          secondCommand,
          disposition
            ? { kind: disposition, text: conversation.followup, verified: true, audience: 'owner', at }
            : { kind: 'text', text: conversation.followup, at },
        ];
        fixture.task = { ...fixture.task };
        if (state === 'running' && fixture.task.agent === 'codex') {
          fixture.task.quiet = { live_tokens: 342000 };
          fixture.task.context_window = { used_tokens: 212000, capacity_tokens: 238000 };
        }
      }
      const examples = await previewExamples;
      if (fixture.task) {
        fixture.task = previewWindowTask(fixture.task);
        const example = examples[fixture.task.id];
        fixture.task = withPreviewExample(fixture.task, examples);
        if (Array.isArray(example?.items)) fixture.items = [...example.items];
        const followups=notes.get(fixture.task.id);
        if (followups) {
          const now=Date.now(), appended=[];
          for (const {ongoing,item} of followups.values()) {
            const sent=Date.parse(item.at);
            appended.push({...item,state:ongoing&&now<sent+10000?'pending':'delivered'});
            if(!ongoing)continue;
            if(now>=sent+2000)appended.push({
              kind:'tool',title:'Команда',description:'Проверить изменения в макете',
              text:'git diff --stat',at:new Date(sent+2000).toISOString(),
            });
            if(now>=sent+4000)appended.push({
              kind:'text',text:'Пример в макете: закончил текущую проверку. Ваше сообщение пока ожидает прочтения.',
              at:new Date(sent+4000).toISOString(),
            });
          }
          fixture.items.push(...appended.sort((a,b)=>a.at.localeCompare(b.at)));
        }
      }
      if (url.pathname === '/api/window' || url.pathname === '/api/project-activity') {
        const completed = sampleResults.filter(row=>row.project===project&&!fixture.tasks.some(task=>task.id===row.id)).map(previewCompleted);
        fixture.tasks = [...fixture.tasks, ...completed].map(previewWindowTask).filter(task=>!hiddenArchive.has(task.id));
      }
      if (fixture.tasks) fixture.tasks = fixture.tasks.map(task => withPreviewExample(task, examples));
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
    if (file === 'index.html') body = body.replace('id="am-concept"', 'id="am-concept" class="am-task-sidebar-preview" data-preview-token="preview-only"');
    response.writeHead(200, { 'Content-Type': contentType }).end(body);
  } catch (error) {
    console.error(error.message);
    response.writeHead(500).end();
  }
});

previewExamples.then(() => {
  server.listen(Number(process.argv[2] || 0), host, () => {
    console.log('http://' + host + ':' + server.address().port + '/');
  });
}).catch(error => {
  console.error(error.message);
  process.exitCode = 1;
});
