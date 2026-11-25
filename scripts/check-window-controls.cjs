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
      let windowReadFails = true;
      let coordinatorActive = false, hostRunning = false;
      let machineOrder = null, machinePeers = undefined, peerLasting = true;
      let setupStates = [], installed = false;
      let updates = [];
      let autoUpdate = false, autoSupported = false, autoRefuse = false;
      const writes = [];
      // WebKit reports a background request that a reload cut off as a page
      // error ("… due to access control checks"); the app catches it, so it
      // is not counted. Every other page error fails the check.
      page.on('pageerror', error => {
        if (engine === 'webkit' && /\/api\/[a-z-]+ due to access control checks\.$/.test(error.message)) return;
        errors.push(error.message);
      });
      if (native) await page.addInitScript(() => {
        window.messengerInitialTheme = 'light';
        window.themeMessages = [];
        window.webkit = {messageHandlers: {appearance: {postMessage: theme => window.themeMessages.push(theme)}}};
      });
      await context.route('**/*', async route => {
        const pathname = new URL(route.request().url()).pathname;
        if (route.request().method() === 'POST') {
          writes.push({path: pathname, body: route.request().postDataJSON()});
          if (pathname === '/api/machine-order') {
            ({order: machineOrder, peers: machinePeers} = route.request().postDataJSON());
            return route.fulfill({json: {ok: true}});
          }
          if (pathname === '/api/server-action' && route.request().postDataJSON().action === 'auto_update') {
            if (autoRefuse) return route.fulfill({status: 409, json: {ok: false, detail: 'Fixture refusal'}});
            autoUpdate = route.request().postDataJSON().on;
            return route.fulfill({json: {ok: true, result: {host: {running: false, enabled: false, addresses: []}, member: {connected: false}, peers: [], updates, accept: true, auto_update: autoUpdate, auto_update_supported: autoSupported}}});
          }
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
          if (pathname === '/api/agent-login') {
            if(route.request().postDataJSON().provider==='cursor')return route.fulfill({status:400,json:{ok:false,detail:'Каталог профиля агента недоступен: /fictional/missing-profile. Проверьте путь к профилю в настройках агента.'}});
            return route.fulfill({status:501,json:{ok:false,detail:'Fixture terminal failed',command:'fictional-cli login',hint:'fixture launcher error'}});
          }
          if (pathname === '/api/coordinator-connect') connected = true;
        }
        if (pathname === '/api/window' && windowReadFails) {
          return route.fulfill({status: 502, body: 'client_rejected'});
        }
        const fixtures = {
          '/api/window': {projects: created ? [{name: 'Welcome test'}] : [], project: created ? 'Welcome test' : '',
            folder: created ? '/fictional/project' : '', messenger: '/fictional/messenger', data_root: '/fictional/data',
            agents: [...(registered ? [{agent: 'cursor', cli: '/fictional/cursor', models: []}] : []), ...(installed ? [{agent:'codex',cli:'/fictional/codex',models:[],trust:{state:trusted?'trusted':'not_trusted'}}] : [])], tasks: [],
            coordinator: {connected, client: 'cursor-vscode', active: coordinatorActive},
            cursor_link: {state: connected ? 'connected' : created ? 'missing' : 'no_folder'},
            computer_name: 'Theme test', tasks_enabled: true},
          '/api/agent-setup-status': setupStates,
          '/api/discovery': {agents: ['claude','codex','cursor'].map(provider => ({provider, state: 'not_found'}))},
          '/api/client-info': {client: 'running', outdated: false},
          '/api/machine-order': {order: machineOrder, peers: machinePeers},
          '/api/server': {host: {running: hostRunning, enabled: hostRunning, scope:'lan', addresses: []}, member: {connected: false}, peers: connected ? [{id:'fixture-peer',...(peerLasting ? {computer:'computer-fixture'} : {}),name:'Peer fixture',host:true,age:0,agents:[{agent:'cursor'}],coordinator:'cursor-vscode'},{id:'fixture-peer-2',name:'Paused agent fixture',age:0,agents:[],coordinator:'claude-code'}] : [], updates, accept: true, auto_update: autoUpdate, auto_update_supported: autoSupported},
          '/api/self-update': {current:'1.0.3-preview.106',installed_version:'1.0.3-preview.106'},
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
      await page.locator('#project-select option').filter({hasText: 'Проекты недоступны'}).waitFor({state: 'attached'});
      assert(await page.locator('#project-select').isDisabled());
      windowReadFails = false;
      await page.evaluate(() => window.messengerRefresh());
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
      // T-277: the automatic update switch shows only where the client can
      // update itself, and changes only through the owner's click.
      const autoRow = page.locator('#auto-update-row'), autoToggle = page.locator('#auto-update-toggle');
      const serverRead = page.waitForResponse(response => new URL(response.url()).pathname === '/api/server');
      await page.locator('#settings-open').click();
      await serverRead;
      assert.equal(await autoRow.isVisible(), false, 'no switch where automatic updates are unavailable');
      await page.locator('#settings-close').click();
      autoSupported = true;
      await page.locator('#settings-open').click();
      await autoRow.waitFor();
      assert.equal(await autoToggle.getAttribute('aria-checked'), 'false');
      assert.equal(await page.locator('#auto-update-status').isVisible(), false);
      await autoToggle.click();
      await page.waitForFunction(() => document.getElementById('auto-update-toggle').getAttribute('aria-checked') === 'true');
      assert.deepEqual(writes.filter(w => w.body?.action === 'auto_update').map(w => w.body), [{action: 'auto_update', on: true}]);
      const autoBox = await autoToggle.boundingBox();
      assert(autoBox.width < 60, 'the switch keeps its own width');
      autoRefuse = true;
      await autoToggle.click();
      await page.locator('#auto-update-status').filter({hasText: 'Fixture refusal'}).waitFor();
      assert.equal(await autoToggle.getAttribute('aria-checked'), 'true', 'a refused change keeps the switch');
      autoRefuse = false;
      await autoToggle.click();
      await page.waitForFunction(() => document.getElementById('auto-update-toggle').getAttribute('aria-checked') === 'false');
      await page.locator('#settings-close').click();
      await page.locator('#settings-open').click();
      assert.equal(await page.locator('#auto-update-status').isVisible(), false, 'an old refusal is gone when settings open again');
      await page.locator('#settings-close').click();
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
      await page.setViewportSize({width:1100,height:850});
      const projects = page.locator('[aria-controls="project-select-menu"]');
      await projects.click();
      const projectMenu = page.locator('#project-select-menu');
      await projectMenu.waitFor({state:'visible'});
      await Promise.all([
        page.waitForResponse(response => new URL(response.url()).pathname==='/api/window'),
        page.evaluate(() => window.messengerRefresh()),
      ]);
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert(await projectMenu.isVisible(), 'refreshing the same projects should keep the menu open');
      await page.keyboard.press('Escape');
      await page.setViewportSize({width:600,height:850});
      await page.locator('[data-tab=server]').click();
      await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').waitFor({state:'attached'});
      assert.equal(await page.locator('#machines [aria-current="true"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines [data-peer="fixture-peer"] .am-agent-line').count(),1);
      assert.equal(await page.locator('#machines .am-agent-line').first().innerText(),'Cursor');
      assert.equal(await page.locator('#machines').getByText('cursor-vscode',{exact:true}).count(),0);
      // An agent paused or removed on a computer is not listed there, even
      // when a chat of it once used Messenger's tools (owner, 2026-11-15).
      assert.equal(await page.locator('#machines [data-peer="fixture-peer-2"] .am-agent-line').count(),0);
      // T-036: the computer that runs the server is labelled; this one is not.
      assert.equal(await page.locator('#machines [data-peer="fixture-peer"] .am-machine-role').innerText(),'сервер');
      assert.equal(await page.locator('#machines [aria-current="true"] .am-machine-role').count(),0);
      await page.setViewportSize({width:1100,height:850});
      const coordinatorLabel = page.locator('#machines [aria-current="true"] .am-machine-role').filter({hasText:/^координатор$/});
      const localCard=page.locator('#machines [aria-current="true"]');
      coordinatorActive = true;
      await page.evaluate(() => window.messengerRefresh());
      await coordinatorLabel.waitFor();
      assert.equal(await page.locator('#machines [data-peer] .am-machine-role').count(),1);
      hostRunning = true;
      await page.locator('[data-tab=server]').click();
      const roles=localCard.locator('.am-machine-roles');
      await roles.waitFor();
      assert.equal(await localCard.locator('.am-machine-version').innerText(),'v1.0.3');
      const checkHeading=async()=>{
        const layout=await localCard.locator('.am-machine-name').evaluate(head=>{
          const visible=selector=>[...head.querySelectorAll(selector)].filter(node=>node.getClientRects().length&&getComputedStyle(node).visibility==='visible');
          const mid=node=>{const box=node.getBoundingClientRect();return box.top+box.height/2;};
          const labels=visible('.am-machine-label,.am-machine-role,.am-version');
          const icons=visible('.am-machine-presence > svg,.am-machine-role-icons svg,.am-card-update .am-update-icon > svg:not(.am-update-progress)');
          const baselines=labels.filter(node=>!node.matches('.am-version')).map(node=>{
            const marker=document.createElement('span');
            marker.style.cssText='display:inline-block;width:0;height:0';
            node.append(marker);const baseline=marker.getBoundingClientRect().top;marker.remove();
            return baseline;
          });
          return {
            centered:icons.every(node=>Math.abs(mid(node)-mid(head))<0.5)&&labels.every(node=>Math.abs(mid(node)-mid(head))<=1),
            baselineAligned:Math.max(...baselines)-Math.min(...baselines)<0.5,
            lineHeights:labels.map(node=>getComputedStyle(node).lineHeight),
            sizes:icons.map(node=>{const box=node.getBoundingClientRect();return [box.width,box.height];}),
          };
        });
        assert.equal(layout.centered,true,'every visible header item shares one vertical center');
        assert.equal(layout.baselineAligned,true,'names and visible role labels share one text baseline');
        assert.deepEqual([...new Set(layout.lineHeights)],['21px']);
        assert.deepEqual(layout.sizes,Array.from({length:layout.sizes.length},()=>[14,14]));
      };
      const splitter=page.getByRole('separator',{name:'Ширина левой панели'});
      const wide=localCard.locator('.am-machine-roles[data-form=full] > [data-form=full]');
      await splitter.press('Shift+ArrowRight');
      await splitter.press('Shift+ArrowRight');
      await wide.waitFor();
      assert.equal(await wide.textContent(),'сервер, координатор');
      await checkHeading();
      await splitter.press('Home');
      for(let i=0;i<4;i++)await splitter.press('ArrowRight');
      const compact=localCard.locator('.am-machine-roles[data-form=short] > [data-form=short]');
      await compact.waitFor();
      assert.equal(await compact.textContent(),'сервер, коорд.');
      await checkHeading();
      assert(await roles.evaluate(node=>{
        const head=node.parentElement,name=head.querySelector('.am-machine-label'),meta=head.querySelector('.am-machine-meta');
        const box=node.getBoundingClientRect(),top=name.getBoundingClientRect();
        return box.top<top.bottom&&box.bottom>top.top&&box.right<=meta.getBoundingClientRect().left;
      }),'abbreviated roles must sit beside the name and before the version');
      const originalHeading=await localCard.locator('.am-machine-name').elementHandle();
      await page.evaluate(() => window.messengerRefresh());
      assert(await localCard.locator('.am-machine-name').evaluate((node,original)=>node===original,originalHeading));
      await originalHeading.dispose();
      for(let i=0;i<3;i++)await splitter.press('Shift+ArrowLeft');
      assert.equal(await splitter.getAttribute('aria-valuenow'),'160');
      const icons=localCard.locator('.am-machine-roles[data-form=icons] > [data-form=icons]');
      await icons.waitFor();
      assert.equal(await icons.getByRole('img',{name:'сервер',exact:true}).isVisible(),true);
      assert.equal(await icons.getByRole('img',{name:'координатор',exact:true}).isVisible(),true);
      await checkHeading();
      const roleFits=()=>roles.evaluate(node=>{
        const head=node.parentElement,style=getComputedStyle(head);
        const meta=head.querySelector('.am-machine-meta').getBoundingClientRect();
        const edge=head.getBoundingClientRect().right-parseFloat(style.paddingRight);
        return node.getBoundingClientRect().right<=meta.left&&meta.right<=edge+1;
      });
      assert(await roleFits(),'both role icons and the version fit in the heading at the minimum width');
      await page.evaluate(()=>{
        window.webkit??={};
        window.webkit.messageHandlers??={};
        window.webkit.messageHandlers.updates={postMessage(){}};
        window.messengerUpdateStatus({installedVersion:'1.0.3-preview.106',version:'1.0.4-preview.107'});
      });
      const compactIcon=roles.getByRole('img',{name:'сервер, координатор',exact:true});
      await compactIcon.waitFor();
      assert(await localCard.locator('#app-update-button').isVisible());
      await checkHeading();
      assert(await roleFits(),'the role mark fits alongside the version and the available update');
      await page.evaluate(()=>{
        window.messengerUpdateStatus({installedVersion:'1.0.3-preview.106'});
        delete window.webkit.messageHandlers.updates;
      });
      await icons.waitFor();
      await splitter.press('Shift+ArrowRight');
      await splitter.press('Shift+ArrowRight');
      await splitter.press('Shift+ArrowRight');
      await wide.waitFor();
      await splitter.press('Home');
      hostRunning = false;
      await page.locator('[data-tab=server]').click();
      await roles.waitFor({state:'detached'});
      coordinatorActive = false;
      await page.evaluate(() => window.messengerRefresh());
      await coordinatorLabel.waitFor({state:'detached'});
      const peerCard=page.locator('#machines [data-peer="fixture-peer"]');
      await Promise.all([
        page.waitForResponse(response => response.url().endsWith('/api/machine-order') && response.request().method() === 'POST' && response.ok()),
        peerCard.dragTo(localCard,{sourcePosition:{x:50,y:12},targetPosition:{x:50,y:5}}),
      ]);
      assert.equal(await page.locator('#machines > :first-child').getAttribute('data-peer'),'fixture-peer');
      assert.equal(await peerCard.getAttribute('aria-pressed'),'false');
      // T-273: the order is kept under the lasting id, and a report that
      // briefly comes without it (a refused report) keeps the card's place.
      assert.equal(machineOrder[0],'computer-fixture');
      assert.deepEqual(machinePeers,{'fixture-peer':'computer-fixture'});
      peerLasting = false;
      // The desktop window opens on another loopback port after restarting.
      const restarted = await context.newPage();
      await restarted.goto('http://messenger.test:11342/#new-fictional-token');
      await restarted.locator('#machines > :first-child[data-peer="fixture-peer"]').waitFor();
      assert.equal(await restarted.evaluate(() => localStorage.getItem('messenger-machine-order')), null);
      await restarted.close();
      await page.reload();
      await peerCard.waitFor();
      assert.equal(await page.locator('#machines > :first-child').getAttribute('data-peer'),'fixture-peer');
      await peerCard.press('Alt+ArrowDown');
      assert.equal(await page.locator('#machines > :first-child').getAttribute('aria-current'),'true');
      await page.setViewportSize({width:600,height:850});
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
      await page.locator('article[data-provider=cursor]').getByRole('button',{name:'Войти',exact:true}).click();
      await page.locator('#toasts').getByText('Каталог профиля агента недоступен: /fictional/missing-profile. Проверьте путь к профилю в настройках агента.',{exact:true}).waitFor();
      assert.equal(await page.locator('#toasts').getByText('Проверьте введённые данные.',{exact:true}).count(),0);
      await page.locator('#toasts .am-toast-close').click();
      assert.equal(await codex.getByRole('button',{name:'Доверять',exact:true}).evaluate(n=>n.getBoundingClientRect().height),28);
      await codex.getByRole('button',{name:'Доверять',exact:true}).click();
      assert((await page.locator('#confirm-text').textContent()).includes('/fictional/project'));
      await page.locator('#confirm-go').click();
      await codex.getByRole('button',{name:'Доверять',exact:true}).waitFor({state:'detached',timeout:10000});
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-trust').map(w=>w.body),[{provider:'codex',project:'Welcome test',folder:'/fictional/project'}]);
      await codex.getByRole('button',{name:'Настройки',exact:true}).click();
      assert.equal(await page.locator('#agent-models').inputValue(),''); // An explicitly empty list stays empty.
      assert((await page.locator('#agent-models-note').textContent()).includes('[effort=high]'));
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'600-dark-agent-settings.png')});
      assert.equal(await page.locator('#agent-cli-find').evaluate(n=>n.getBoundingClientRect().height),32);
      await page.locator('#agent-cli-find').click();
      await codex.getByText('Поиск не подтверждён. Проверьте терминал или выберите файл CLI в настройках.',{exact:true}).waitFor();
      assert.deepEqual(writes.filter(w=>w.path==='/api/agent-find').map(w=>w.body),[{provider:'codex',action:'find'},{provider:'codex',action:'find'}]);
      // T-182: the internet scope asks for the external address and sends it.
      await page.locator('[data-tab=server]').click();
      assert.equal(await page.locator('#host-public-box').isHidden(), true);
      await page.locator('#host-scope').evaluate(n => {n.value = 'internet'; n.dispatchEvent(new Event('change', {bubbles: true}));});
      assert.equal(await page.locator('#host-public-box').isVisible(), true);
      await page.locator('#host-enable').click();
      await page.locator('#host-status').getByText('Укажите внешний адрес: белый IP или домен этого компьютера.', {exact: true}).waitFor();
      assert.equal(writes.filter(w => w.path === '/api/server-action' && w.body.action === 'enable').length, 0);
      await page.locator('#host-public').fill('203.0.113.7');
      await page.locator('#host-enable').click();
      await page.waitForTimeout(300);
      assert.deepEqual(writes.filter(w => w.path === '/api/server-action' && w.body.action === 'enable').map(w => w.body),
        [{action: 'enable', scope: 'internet', port: 47100, public_address: '203.0.113.7'}]);
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'server-internet.png')});
      // T-181: notices have no scroll of their own; a new one pushes out the oldest.
      updates = [1,2,3,4,5].map(i => ({request_id:'fixture-update-'+i, computer:'fixture-peer', state:'refused',
        text:'fixture refusal '+i+' '+'long reason '.repeat(40), updated_at:new Date(Date.now()-(10-i)*1000).toISOString()}));
      await page.setViewportSize({width:1100,height:850});
      await page.reload();
      await page.locator('[data-tab=tasks]').click();
      await page.locator('#notices .am-notice').nth(2).waitFor({timeout:10000});
      assert.equal(await page.locator('#notices .am-notice').count(), 3);
      assert.deepEqual(await page.locator('#notices .am-notice').evaluateAll(n => n.map(c => c.dataset.key)),
        ['update-fixture-update-3-refused','update-fixture-update-4-refused','update-fixture-update-5-refused']);
      assert.equal(await page.locator('.am-chat-footer').evaluate(n => getComputedStyle(n).overflowY), 'visible');
      assert.equal(await page.locator('#notices').evaluate(n => n.scrollHeight <= n.clientHeight), true);
      const clippedReason=page.locator('#notices .am-notice-text').first();
      assert.equal(await clippedReason.getAttribute('title'),await clippedReason.textContent(),'a clipped reason still exposes its full text');
      if(process.env.SCREENSHOT_DIR&&!native) await page.screenshot({animations:'disabled',path:path.join(process.env.SCREENSHOT_DIR,'notices.png')});
      await page.locator('#notices .am-toast-close').first().click();
      assert.equal(await page.locator('#notices .am-notice').count(), 2);
      updates = [];
      assert.deepEqual(errors, []);
      console.log(JSON.stringify({engine, nativeBridgeFixture: native, themes: ['light','dark','system'], widths: [1100,600], result: 'passed'}));
      await context.close();
    }
    // Windows without a title bar: the tab row and the task list toggle keep
    // a gap above them and stay centred on the window buttons (owner,
    // 2026-11-15: they were pressed against the top edge).
    {
      const context = await browser.newContext({viewport: {width: 1100, height: 600}, colorScheme: 'light'});
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      const hides = [];
      // The update offered, downloading, installing, restarting (owner,
      // 2026-11-15: one button and one dialog).
      const began = new Date(Date.now() - 3 * 60 * 1000).toISOString();
      let commandFinished = false;
      let laterActivity = false;
      let mixedActions = false;
      const commandStarted = new Date(Date.now() - 72 * 1000).toISOString();
      const ended = '2026-11-14T09:30:00Z';
      const stat = {id:'old-1',title:'Проверить сборку проекта',at:'2026-11-19T09:20:00',project:'Frame',agent:'codex',model:'gpt-6.2-sol[effort=medium]',computer:'Runner',tokens:420000,duration_ms:42000};
      const olderStat = {...stat,id:'old-2',title:'Проверить производительность сцены',at:'2026-11-18T10:10:00',tokens:18200000,duration_ms:10800000,duration_estimated:true};
      const untimedStat = {...stat,id:'old-3',title:'Уточнить настройки агента',at:'2026-11-18T09:00:00',tokens:3400000,duration_ms:undefined};
      const otherStat = {...stat,id:'other-1',title:'Собрать сайт',project:'Site',tokens:700};
      let holdFrameStats, frameStatsRequested, exportedStats;
      const localTask = {id:'local-task',title:'Local task',project:'Frame',agent:'codex',state:'running',created_at:began,started_at:began,model:'gpt-6.2-sol[effort=low]',ran_model:'gpt-6.2-sol[effort=medium]'};
      let localRow = localTask;
      let siteTask = {...localTask,id:'site-task',project:'Site',title:'Site worker task',
        pending_launch:true, waits_for_lock:'shared-cache', started_at:undefined};
      let releaseSite, delaySiteView = false;
      const siteView = new Promise(resolve => { releaseSite = resolve; });
      const remoteTask = {...localTask,id:'remote-task',title:'Remote task',created_at:new Date(Date.now()-8*60*1000).toISOString(),model:'',ran_model:'',remote:{id:'dev-1',name:'Runner',worker:'none'}};
      const waitingTask = {...remoteTask,id:'waiting-task',title:'Waiting for a reply',agent:'claude',state:'waiting_owner',model:'claude-sonnet-5-5[effort=medium]',ran_model:'claude-sonnet-5-5[effort=medium]'};
      let remoteRow = remoteTask;
      const stopRequests = [];
      let refuseStop = false;
      let releaseStop;
      const stopResponse = new Promise(resolve => { releaseStop = resolve; });
      let codexWaiting = true;
      let thirdCodexTask = false;
      let runnerAgents = [{agent:'cursor',running:1},{agent:'claude',waiting_owner:1},{agent:'codex',running:1,waiting_owner:1}];
      let runnerWorkers = [];
      let runnerAge = 0, runnerContactAge, sharedPending = false;
      const ownerNotes = [];
      const update = {state: 'found', version: '0.0.0-preview.99'};
      const updates = [];
      const clears = [];
      await page.addInitScript(() => { window.amDesktop = 'windows'; window.amFrameless = true; window.amWindow = () => {}; });
      await page.route('**/*', async route => {
        const {pathname, searchParams} = new URL(route.request().url());
        const project = searchParams.get('project') || 'Frame';
        if (pathname === '/api/feedback-export') return route.fulfill({json: {ok: true, result: {path: '/fictional/Downloads/feedback.zip', remarks: 2}}});
        if (pathname === '/api/feedback-clear') { clears.push(true); return route.fulfill({json: {ok: true, result: {deleted: 2, left: 0}}}); }
        if (pathname === '/api/window' && project === 'Site' && delaySiteView) {
          await siteView;
          return route.fulfill({json: {projects: [{name: 'Frame'}, {name: 'Site'}], project: 'Site', agents: [], coordinator: {connected: false}, tasks: [siteTask], shared: []}});
        }
        if (pathname === '/api/window') return route.fulfill({json: {projects: [{name: 'Frame'}, {name: 'Site'}], project, folder: '/fictional/frame',
          agents: [{agent:'codex',running:1,project_running:project==='Frame'?1:0}], coordinator: {connected: false}, computer_name: 'Ёж и Йога', tasks_enabled: true, feedback_count: 5, feedback_new: 2, tasks:project==='Frame'?[localRow,remoteRow,waitingTask]:[],
          shared: project==='Frame'?[{peer: 'host', name: 'Giver', age: 1, tasks: [
            {id: 'sent', project: 'Frame', title: 'Sent task', agent: 'cursor', state: 'running', created_at: began, started_at: sharedPending?undefined:began, pending_launch:sharedPending, waits_for_lock:sharedPending?'shared-cache':undefined, model: 'composer-2.5[effort=medium]', runs_on_peer: 'dev-1', runs_on: 'Old name'},
            ...(codexWaiting?[{id:'codex-waiting',project:'Frame',title:'Queued answer',agent:'codex',model:'gpt-6-luna[effort=high]',state:'waiting_coordinator',created_at:began,runs_on_peer:'dev-1'}]:[]),
            ...(thirdCodexTask?[{id:'third-codex',project:'Frame',title:'Third task',agent:'codex',state:'running',created_at:began,model:'gpt-6.2-sol[effort=high]',runs_on_peer:'dev-1'}]:[]),
            {id: 'own', project: 'Frame', title: 'Own task', agent: 'codex', state: 'running', created_at: '2026-11-15T09:00:00Z'},
            {id: 'old-1', project: 'Frame', title: 'Old 1', agent: 'codex', state: 'done', created_at: '2026-11-14T09:00:00Z', finished_at: ended},
            {id: 'old-2', project: 'Frame', title: 'Old 2', agent: 'codex', state: 'failed', created_at: '2026-11-14T08:00:00Z', finished_at: '2026-11-14T08:30:00Z'}]}]:[]}});
        if (pathname === '/api/server') return route.fulfill({json: {host: {running: false, enabled: false, addresses: []}, member: {connected: true, reachable: true, address:'203.0.113.7:47100', connected_at:began}, peers: [{id: 'host', name: 'Giver', age: 0, agents: []}, {id: 'dev-1', name: 'Runner', age: runnerAge, contact_age:runnerContactAge, version:'0.0.0-preview.97', agents: runnerAgents, workers:{runs:runnerWorkers}}], updates: [], accept: true}});
        if (pathname === '/api/usage-export') {
          exportedStats=route.request().postDataJSON();
          return route.fulfill({json:{ok:true,result:{path:'/fictional/messenger-statistics.json',tasks:125}}});
        }
        if (pathname === '/api/usage-stats') {
          if (project === 'Site') return route.fulfill({json:{
            project,total:{tasks:1,tokens:700,input_tokens:600,output_tokens:100},agents:[],computers:[],days:[],
            recent:[otherStat],top:[otherStat],ranked:{oldest:[otherStat],fewest:[otherStat],longest:[otherStat],shortest:[otherStat]},
          }});
          if (holdFrameStats) {
            frameStatsRequested();
            await holdFrameStats;
          }
          return route.fulfill({json:{
            project,total:{tasks:3,tokens:22020000,input_tokens:21100000,output_tokens:920000},agents:[],computers:[],days:[],
            models:['gpt-6.2-sol[effort=high]','claude-opus-5-5[effort=medium]','opus','grok-4.7-high-fast','cursor-grok-4.6-medium'].map(key=>({key,name:key,tokens:1})),
            recent:[stat,olderStat,untimedStat],top:[olderStat,untimedStat,stat],
            ranked:{oldest:[untimedStat,olderStat,stat],fewest:[stat,untimedStat,olderStat],longest:[olderStat,stat],shortest:[stat,olderStat]},
          }});
        }
        if (pathname === '/api/task') {
          const requested = new URL(route.request().url()).searchParams.get('id');
          if (requested === siteTask.id) return route.fulfill({json:{task:siteTask,items:[]}});
          const remote = requested === remoteTask.id;
          const items = [
            {kind:'task',author:'coordinator',text:'Original task',at:began,sender_agent:'claude',sender_model:'claude-opus-5-5'},
            {kind:'answer',author:'coordinator',text:'Follow-up from Cursor',at:began,sender_agent:'cursor'},
            {kind:'answer',author:'coordinator',text:'Earlier follow-up from unknown sender',at:began},
            {kind:'text',text:'Fixture reply',at:began},
          ];
          if (remote) items.push(...ownerNotes.map(note=>({kind:'note',text:note.text,at:began,author:'owner',state:'sent'})));
          else {
            items.push(
              {kind:'report',text:'Previous run finished',at:ended,verified:true},
              {kind:'outcome',text:'Previous run was interrupted',at:began,state:'interrupted',verified:true},
              {kind:'report',text:'Work summary saved',at:ended,verified:false},
              {kind:'outcome',text:'the agent did not report a successful end; Codex error: Reconnecting... 4/5 (Selected model is at capacity. Please try a different model.)',at:began,state:'failed'},
              {kind:'command_start',title:'fixture-command',at:commandStarted},
              {kind:'tool',title:'Команда',text:"rg --files | rg 'RuntimePanelUtils\\.cs$'",at:began},
              {kind:'tool',title:'Команда',text:"cm cat FreeCamera.cs#cs:3474 | rg -n -C 2 'Mouse|UIOwnsKeys'",at:began},
              {kind:'tool',title:'Bash',text:'cm cat Tools/40_tune.steps#cs:3474',description:'Прочитать шаги ручной проверки',at:began},
              {kind:'tool',title:'Команда',text:'ls Assets/Infantry/Client',at:began},
              {kind:'tool',title:'Команда',text:"python3 - <<'PY'\nprint('test')\nPY",at:began},
            );
            if (mixedActions) items.push({kind:'tool',title:'read',text:'ProjectSettings.asset',at:began});
            if (laterActivity) items.push(
              {kind:'text',text:'Moving on to regression tests',at:began},
              {kind:'command_start',title:'second-command',at:commandStarted},
              {kind:'tool',title:'Bash',text:'go test ./...',at:began}
            );
            if (commandFinished) items.push({kind:'command_end',title:'fixture-command',at:began});
          }
          return route.fulfill({json:{task:remote?remoteTask:localTask,items}});
        }
        if (pathname === '/api/task-note') ownerNotes.push(route.request().postDataJSON());
        if (pathname === '/api/task-cancel') {
          stopRequests.push(route.request().postDataJSON());
          if(refuseStop)return route.fulfill({status:409,json:{ok:false,detail:'Fixture stop refusal'}});
          await stopResponse;
          return route.fulfill({json:{ok:true}});
        }
        if (pathname === '/api/task-hide') hides.push(route.request().postDataJSON());
        if (pathname === '/api/self-update') {
          if (route.request().method() === 'POST') { updates.push(route.request().postDataJSON()); update.state = 'downloading'; return route.fulfill({json: {ok: true, state: 'downloading'}}); }
          return route.fulfill({json: {ok: true, installed_version: '0.0.0-preview.98', ...update}});
        }
        if (pathname === '/api/update-progress') return route.fulfill({json: update.state === 'downloading' && update.done ? {downloading: true, version: update.version, done: update.done, total: 100} : {downloading: false}});
        if (pathname.startsWith('/api/')) return route.fulfill({json: {ok: true}});
        const file = pathname === '/' ? 'index.html' : pathname.slice(1);
        const type = file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : file.endsWith('.svg') ? 'image/svg+xml' : 'text/html';
        try { return route.fulfill({body: await fs.readFile(path.join(assets, file)), contentType: type}); } catch (_) { return route.fulfill({status: 404}); }
      });
      await page.goto('http://127.0.0.1/#fixture-token');
      await page.locator('#list-toggle').waitFor({state: 'visible'});
      const tab = await page.locator('.am-tab').first().boundingBox();
      const toggle = await page.locator('#list-toggle').boundingBox();
      const button = await page.locator('.am-window-btn').first().boundingBox();
      assert(tab.y >= 8 && toggle.y >= 8, `tab row too close to the top: tab ${tab.y}, toggle ${toggle.y}`);
      assert(Math.abs((toggle.y + toggle.height / 2) - (button.y + button.height / 2)) <= 1, 'toggle off the window buttons');
      // T-186: another computer's task says where it runs, and who gave it
      // when that is another computer.
      const sharedLabels = await page.locator('.am-task-shared .am-task-agent > span:not(.am-task-when)').allInnerTexts();
      assert.deepEqual(sharedLabels.filter(Boolean), ['Cursor на Runner — от Giver', 'Codex на Runner — от Giver', 'Codex на Giver', 'Codex на Giver', 'Codex на Giver']);
      const completedTime=page.locator('.am-task-shared',{hasText:'Old 1'}).locator('.am-task-when');
      assert.equal(await completedTime.getAttribute('datetime'),ended);
      assert.match(await completedTime.innerText(),/\d{1,2}:\d{2}/);
      assert.equal(await page.locator('.am-task',{hasText:localTask.title}).locator('.am-task-when').getAttribute('datetime'),began);
      const taskRow=page.locator('.am-task',{hasText:localTask.title});
      const taskParts=taskRow.locator('.am-task-agent,.am-task-status,.am-task-model,.am-task-when');
      assert.deepEqual(await taskParts.evaluateAll(nodes=>nodes.map(node=>getComputedStyle(node).fontSize)),Array(4).fill('12px'));
      const type=locator=>locator.evaluate(node=>{
        const {fontFamily,fontSize,fontWeight,lineHeight}=getComputedStyle(node);
        return {fontFamily,fontSize,fontWeight,lineHeight};
      });
      const computerTitle=await type(page.locator('.am-machine-card[aria-current=true] .am-machine-name > span:nth-child(2)'));
      assert.deepEqual(await type(taskRow.locator('.am-task-title')),computerTitle);
      assert.equal(computerTitle.fontSize,'14px');
      assert.equal(computerTitle.fontWeight,'500');
      const waitingTaskStatus=page.locator('.am-task',{hasText:waitingTask.title}).locator('.am-task-status');
      assert.match(await waitingTaskStatus.innerText(),/Ждёт вашего ответа,\s*Sonnet 5\.5 Medium/);
      assert.equal(await waitingTaskStatus.locator('.am-dot').count(),0);
      await page.locator('[data-tab=server]').click();
      const runner=page.locator('[data-peer="dev-1"]');
      const running=runner.locator('.am-agent-run[data-provider=cursor]');
      const runningLabel=running.locator('.am-agent-running-label');
      const runningTimer=running.locator('.am-agent-status');
      await runningLabel.waitFor();
      assert.equal(await runningLabel.locator('.am-agent-model').textContent(),'Composer 2.5 Med');
      assert.equal(await runningTimer.textContent(),'3 мин');
      assert.equal(await runner.locator('.am-agent-wait').count(),1);
      assert.equal(await runner.locator('.am-agent-idle').count(),0);
      assert.equal(await runner.locator('.am-agent-running-label').count(),2);
      const waiting=runner.locator('.am-agent-wait[data-provider=claude]');
      const waitingStatus=waiting.locator('.am-agent-status');
      assert.equal(await waiting.locator('.am-agent-model').textContent(),'Sonnet 5.5 Med');
      assert.equal(await waitingStatus.textContent(),'ждёт ответа');
      assert.equal(await waitingStatus.evaluate(node=>getComputedStyle(node,'::before').content),'", "');
      const waitingLabelBox=await waiting.locator('span').first().boundingBox(),waitingStatusBox=await waitingStatus.boundingBox();
      assert(Math.abs(waitingStatusBox.x-waitingLabelBox.x-waitingLabelBox.width)<1,'waiting status must sit directly beside the model');
      const overlappingAgent=runner.locator('.am-agent-run[data-provider=codex]');
      assert.equal(await overlappingAgent.locator('.am-agent-model').first().isVisible(),false);
      const summary=overlappingAgent.locator('summary');
      assert.equal(await summary.innerText(),'Codex, 2 задачи');
      const details=overlappingAgent.locator('.am-agent-parallel-list');
      assert.equal(await details.isVisible(),false);
      const cardSelection=await runner.getAttribute('aria-pressed');
      await summary.click();
      assert.equal(await details.isVisible(),true);
      assert.equal(await runner.getAttribute('aria-pressed'),cardSelection);
      assert.match(await details.innerText(),/Remote task\s+3 мин/);
      const waitingRun=details.locator('.am-agent-parallel-waiting');
      assert.equal(await waitingRun.innerText(),'Luna 6 High, ждёт ответа');
      assert.equal(await waitingRun.getAttribute('title'),'Ждёт ответа координатора');
      assert.equal(await waitingRun.locator('.am-agent-model').evaluate(node=>getComputedStyle(node).color),
        await waitingStatus.evaluate(node=>getComputedStyle(node).color));
      const waitingRunParts=await waitingRun.locator('span').evaluateAll(nodes=>nodes.map(node=>node.getBoundingClientRect().y));
      assert.equal(waitingRunParts[0],waitingRunParts[1],'model and waiting status share a line when there is room');
      await summary.focus();
      thirdCodexTask=true;
      await summary.getByText('3 задачи',{exact:false}).waitFor();
      assert.equal(await details.isVisible(),true,'an expanded group stays open when another task arrives');
      assert.equal(await summary.evaluate(node=>document.activeElement===node),true);
      assert.equal(await details.getByText('Sol 6.2 High',{exact:true}).isVisible(),true);
      assert(await runner.evaluate(node=>{
        node.style.width='166px';
        const body=node.querySelector('.am-machine-body'),arrow=node.querySelector('.am-agent-parallel summary > svg');
        const gap=body.getBoundingClientRect().right-arrow.getBoundingClientRect().right;
        const fits=body.scrollWidth<=body.clientWidth&&Math.abs(gap-parseFloat(getComputedStyle(body).paddingRight))<1;
        node.style.removeProperty('width');
        return fits;
      }),'three tasks must retain the right inset on a narrow computer card');
      await summary.press('Enter');
      assert.equal(await details.isVisible(),false);
      thirdCodexTask=false;
      await summary.getByText('2 задачи',{exact:false}).waitFor();
      const chooseProject = async name => {
        await page.locator('[aria-controls="project-select-menu"]').click();
        await page.getByRole('option',{name,exact:true}).click();
      };
      await page.locator('#machines [aria-current="true"] .am-agent-run[data-provider=codex]').waitFor();
      await chooseProject('Site');
      assert.equal(await page.locator('#machines .am-agent-run, #machines .am-agent-wait').count(),0);
      await page.waitForFunction(() => document.querySelector('#project-select').value==='Site');
      assert.equal(await runner.locator('.am-agent-idle').count(),3);
      const emptyHost=page.locator('#machines [data-peer="host"] .am-machine-body .am-muted');
      assert.equal(await runner.locator('.am-agent-idle').first().evaluate(node=>getComputedStyle(node).color),
        await emptyHost.evaluate(node=>getComputedStyle(node).color));
      await chooseProject('Frame');
      await runningLabel.waitFor();
      assert.equal(await runningLabel.evaluate(node=>getComputedStyle(node).animationName),'none');
      assert.equal(await runningTimer.evaluate(node=>getComputedStyle(node).animationName),'none');
      const rowFontSize=await page.locator('.am-task-agent').first().evaluate(node=>getComputedStyle(node).fontSize);
      for(const item of [runningLabel,runningLabel.locator('.am-agent-model'),runningTimer]){
        assert.equal(await item.evaluate(node=>getComputedStyle(node).fontSize),rowFontSize);
      }
      assert.equal(await runningTimer.evaluate(node=>getComputedStyle(node).backgroundColor),'rgba(0, 0, 0, 0)');
      assert.equal(await runningTimer.evaluate(node=>getComputedStyle(node,'::before').content),'", "');
      const labelBox=await runningLabel.boundingBox(),timerBox=await runningTimer.boundingBox();
      assert(Math.abs(timerBox.x-labelBox.x-labelBox.width)<1,'timer separator must sit directly beside the model');
      assert(await running.evaluate(node=>{
        const label=node.querySelector('.am-agent-running-label'),timer=node.querySelector('.am-agent-status');
        const fullWidth=timer.getBoundingClientRect().width;
        node.style.width='100px';
        const fits=label.scrollWidth>label.clientWidth&&Math.abs(timer.getBoundingClientRect().width-fullWidth)<1&&
          timer.getBoundingClientRect().right<=node.getBoundingClientRect().right+1;
        node.style.removeProperty('width');
        return fits;
      }),'a long agent label must give way to the timer');
      assert.equal(await runner.locator('.am-machine-name .am-card-update').count(),1);
      assert.equal(await page.locator('.am-machine-name #app-update-button').count(),1);
      const peerUpdate=runner.getByRole('button',{name:'Обновить до v0.99'});
      const peerVersion=runner.locator('.am-machine-version');
      assert.equal(await peerVersion.textContent(),'v0.97');
      assert.equal(await peerUpdate.locator('.am-machine-version').count(),0);
      await page.emulateMedia({reducedMotion:'reduce'});
      await page.waitForFunction(() => {
        const label = document.querySelector('#machines [data-peer="dev-1"] [data-provider="cursor"] .am-agent-running-label');
        return label && getComputedStyle(label).animationName === 'none';
      });
      // WebKit can retain the previous color scheme in a hidden screen.
      await page.locator('[data-tab=tasks]').click();
      const runningTaskStatus=taskRow.locator('.am-task-status.am-status-run');
      await runningTaskStatus.waitFor({state:'visible'});
      for (const [theme,arrow,muted,runningColor,waitingColor] of [
        ['light','rgb(129, 83, 5)','rgb(107, 107, 107)','rgb(40, 123, 215)','rgb(201, 138, 0)'],
        ['dark','rgb(242, 198, 91)','rgb(160, 160, 160)','rgb(103, 170, 249)','rgb(242, 201, 76)']
      ]) {
        await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;},theme);
        assert.equal(await runningTaskStatus.evaluate(node=>getComputedStyle(node).color),runningColor);
        assert.equal(await runningLabel.evaluate(node=>getComputedStyle(node).color),runningColor);
        assert.equal(await runningLabel.locator('.am-agent-model').evaluate(node=>getComputedStyle(node).color),runningColor);
        assert.equal(await runningTimer.evaluate(node=>getComputedStyle(node).color),muted);
        for(const part of [waiting.locator('span').first(),waiting.locator('.am-agent-model'),waitingStatus]){
          assert.equal(await part.evaluate(node=>getComputedStyle(node).color),waitingColor);
        }
        assert.equal(await waitingStatus.evaluate(node=>getComputedStyle(node,'::before').color),waitingColor);
        await page.mouse.move(0,0);
        const style=await peerUpdate.evaluate(node=>{
          const css=getComputedStyle(node);
          return {color:css.color,background:css.backgroundColor};
        });
        assert.deepEqual(style,{color:arrow,background:'rgba(0, 0, 0, 0)'});
        assert.equal(await peerVersion.evaluate(node=>getComputedStyle(node).color),muted);
        await peerUpdate.hover();
        assert.notEqual(await peerUpdate.evaluate(node=>getComputedStyle(node).backgroundColor),style.background);
        assert.equal(await peerUpdate.evaluate(node=>getComputedStyle(node).borderRadius),'50%');
      }
      await page.evaluate(()=>{document.documentElement.dataset.theme='light';});
      await page.locator('[data-tab=server]').click();
      await peerUpdate.locator('svg').click();
      await page.locator('#confirm-dialog').waitFor({state:'visible'});
      assert.equal(await page.locator('#confirm-title').innerText(),'Обновить Runner до v0.99?');
      assert.deepEqual(await type(page.locator('#confirm-title')),await type(page.locator('#member-title')));
      await page.locator('#confirm-cancel').click();
      for (const computer of [page.locator('.am-machine-card[aria-current=true]'),runner]) {
        const name = computer.locator('.am-machine-name > span:nth-child(2)');
        assert(await name.evaluate(node => node.getBoundingClientRect().height >= parseFloat(getComputedStyle(node).lineHeight)-0.5),
          'computer name must keep the full line height');
      }
      const disconnect = page.locator('#member-disconnect');
      await disconnect.waitFor({state: 'visible'});
      for (const width of [1100, 600]) {
        await page.setViewportSize({width, height: 850});
        const details = await page.locator('#member-details').boundingBox();
        const action = await disconnect.boundingBox();
        const badge = await page.locator('#member-badge').boundingBox();
        assert(action.x >= details.x + details.width, 'disconnect action overlaps the connection details');
        assert(Math.abs(action.y + action.height - (details.y + details.height)) <= 1, 'disconnect action is not aligned with the bottom of the connection details');
        assert(Math.abs(action.x + action.width - (badge.x + badge.width)) <= 1, 'disconnect action is off the right edge of the status');
      }
      await page.setViewportSize({width: 1100, height: 600});
      if (process.env.SCREENSHOT_DIR) {
        await page.emulateMedia({colorScheme:'dark'});
        await page.locator('[aria-labelledby=member-title]').screenshot({path: path.join(process.env.SCREENSHOT_DIR,'server-connected.png')});
        await page.emulateMedia({colorScheme:'light'});
      }
      await page.locator('[data-tab=stats]').click();
      const detail = page.locator('#stats-top .am-stats-task-text .am-muted').first();
      // T-274: the model filter uses the same names as tasks and messages.
      await page.locator('#stats-model option[value=opus]').waitFor({state:'attached'});
      assert.deepEqual(await page.locator('#stats-model option').allTextContents(),
        ['Все модели','Sol 6.2 High','Opus 5.5 Medium','Opus','Grok 4.7 High Fast','Grok 4.6 Medium']);
      await detail.waitFor();
      assert.equal(await detail.textContent(),'Frame Codex Sol 6.2 Medium на Runner');
      assert.equal(await page.locator('#stats-top time').first().getAttribute('datetime'),stat.at);
      assert.equal(await page.locator('#stats-top tr').first().locator('td').nth(2).textContent(),'42 с');
      assert.equal(await page.locator('#stats-top tr').nth(1).locator('td').nth(2).textContent(),'≈ 3 ч');
      assert.equal(await page.locator('#stats-top tr').last().locator('td').nth(2).textContent(),'—');
      assert.deepEqual(await type(page.locator('#stats-top .am-stats-task-title').first()),computerTitle);
      const unfilteredComputer=await runner.getAttribute('aria-pressed')!=='true';
      if(unfilteredComputer)await runner.press('Enter');
      await page.locator('[data-period=month]').click();
      const chooseStatsOption=async (filter,name)=>{
        await page.locator('[aria-controls="stats-'+filter+'-menu"]').click();
        await page.getByRole('option',{name,exact:true}).click();
      };
      await chooseStatsOption('agent','Codex');
      await chooseStatsOption('model','Sol 6.2 High');
      await page.locator('#stats-export').click();
      await page.locator('#stats-export-status').getByText(/125 задач.*messenger-statistics\.json/).waitFor();
      assert.deepEqual(exportedStats,{project:'Frame',period:'month',computer:'dev-1',agent:'codex',model:'gpt-6.2-sol[effort=high]'});
      await chooseStatsOption('agent','Все агенты');
      await page.locator('[data-period=week]').click();
      if(unfilteredComputer)await runner.press('Enter');
      await page.locator('[data-tab=project]').click();
      const rules=await type(page.locator('#project-page-rules'));
      const projectName=await type(page.locator('#project-page-name'));
      assert.equal(rules.fontFamily,projectName.fontFamily);
      assert.equal(rules.fontSize,projectName.fontSize);
      await page.locator('[data-tab=stats]').click();
      const sorted = page.locator('.am-stats-table th[aria-sort]');
      const firstTitle = () => page.locator('#stats-top .am-stats-task-title').first().textContent();
      for (const header of await page.locator('.am-stats-sort').all()) {
        const quiet = await header.evaluate(button=>getComputedStyle(button).backgroundColor);
        await header.hover();
        const {paddingTop,paddingRight,paddingBottom,paddingLeft,backgroundColor} = await header.evaluate(button => {
          const style = getComputedStyle(button);
          return {
            paddingTop: style.paddingTop,
            paddingRight: style.paddingRight,
            paddingBottom: style.paddingBottom,
            paddingLeft: style.paddingLeft,
            backgroundColor: style.backgroundColor
          };
        });
        assert.equal(paddingLeft,paddingRight);
        assert.equal(paddingTop,paddingBottom);
        assert(parseFloat(paddingLeft)>0 && parseFloat(paddingTop)>0);
        assert.notEqual(backgroundColor,quiet);
      }
      for (const [column,direction,first] of [
        ['recent','ascending',untimedStat],['recent','descending',stat],
        ['tokens','descending',olderStat],['tokens','ascending',stat],
        ['longest','descending',olderStat],['longest','ascending',stat],
      ]) {
        await page.locator('[data-stats-sort='+column+']').click();
        assert.equal(await sorted.count(),1);
        assert.equal(await sorted.getAttribute('aria-sort'),direction);
        assert.equal(await firstTitle(),first.title);
      }
      await page.locator('[data-stats-sort=longest]').click();
      if(process.env.SCREENSHOT_DIR){
        await page.mouse.move(0,0);
        const previous = await page.evaluate(()=>{
          const prev=document.documentElement.dataset.theme;
          document.documentElement.dataset.theme='dark';
          return prev;
        });
        await page.locator('[aria-labelledby=stats-top-title].am-card').screenshot({path:path.join(process.env.SCREENSHOT_DIR,'stats-duration-dark.png')});
        await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;},previous);
      }
      await page.setViewportSize({width:600,height:600});
      assert(await page.locator('.am-stats-table-wrap').evaluate(box=>box.scrollWidth===box.clientWidth));
      await page.setViewportSize({width:1100,height:600});
            const pendingFrame = new Promise(resolve => { frameStatsRequested = resolve; });
      let releaseFrame;
      holdFrameStats = new Promise(resolve => { releaseFrame = resolve; });
      await page.locator('[data-tab=stats]').click();
      await pendingFrame;
      await chooseProject('Site');
      await page.locator('#stats-top').getByText(otherStat.title,{exact:true}).waitFor();
      assert.match(await page.locator('#stats-total-title').textContent(),/^700 /);
      const lateFrame = page.waitForResponse(r => new URL(r.url()).pathname==='/api/usage-stats' && new URL(r.url()).searchParams.get('project')==='Frame');
      releaseFrame();holdFrameStats=null;
      await (await lateFrame).finished();
      assert.equal(await firstTitle(),otherStat.title);
      await chooseProject('Frame');
      await page.locator('#stats-top').getByText(stat.title,{exact:true}).waitFor();
      await page.locator('[data-tab=tasks]').click();
      await page.locator('.am-task-title',{hasText:localTask.title}).click();
      const sender = page.locator('#thread .am-outgoing .am-meta').first();
      await sender.waitFor();
      assert.equal(await sender.locator('strong').textContent(),'Claude');
      assert.equal(await sender.locator('.am-avatar .am-provider-icon').getAttribute('data-provider'),'claude');
      assert.equal(await sender.locator('.am-avatar .am-provider-icon').isVisible(),true);
      assert.equal(await sender.locator('.am-meta-model').textContent(),'Opus 5.5');
      const followUp = page.locator('#thread .am-outgoing').filter({hasText:'Follow-up from Cursor'});
      assert.equal(await followUp.locator('strong').textContent(),'Cursor');
      assert.equal(await followUp.locator('.am-avatar .am-provider-icon[data-provider=cursor]').isVisible(),true);
      const unknownSender = page.locator('#thread .am-outgoing').filter({hasText:'Earlier follow-up from unknown sender'});
      assert.equal(await unknownSender.locator('strong').textContent(),'Координатор');
      assert.equal(await page.locator('#thread .am-message:not(.am-outgoing) .am-meta-model').first().textContent(),'Sol 6.2 Medium');
      assert.match(await page.locator('#task-head').innerText(),/Codex \(Sol 6\.2 Medium\)/);
      const closedTime=page.locator('#thread .am-delivery-close time');
      assert.equal(await closedTime.getAttribute('datetime'),ended);
      assert.match(await closedTime.innerText(),/\d{1,2}:\d{2}/);
      assert.equal(await page.locator('#thread .am-outcome time').first().getAttribute('datetime'),began);
      const overload = page.locator('#thread .am-outcome').last();
      assert.match(await overload.innerText(),/Завершение запуска не подтверждено/);
      assert.match(await overload.innerText(),/Модель перегружена/);
      assert.equal(await page.locator('#thread').getByText('Отчёт сохранён без подтверждения запуска',{exact:true}).count(),1);
      const toolGroups=page.locator('#thread .am-tool-group');
      const actionGroup=toolGroups.first();
      // A click that lands while the thread is redrawn reaches the old
      // group, so open it until the current one stays open.
      for(let tries=0;!(await actionGroup.evaluate(node=>node.open));tries++){
        assert.ok(tries<5,'the action group opens');
        await actionGroup.locator(':scope > summary').click();
      }
      const timer=actionGroup.locator('.am-tool-timer');
      assert.match(await timer.innerText(),/^1 мин \d{1,2} с$/);
      const label=actionGroup.getByText('Выполняются команды',{exact:true});
      const fontSize=locator=>locator.evaluate(node=>getComputedStyle(node).fontSize);
      const color=locator=>locator.evaluate(node=>getComputedStyle(node).color);
      const animation=locator=>locator.evaluate(node=>getComputedStyle(node).animationName);
      assert.equal(await fontSize(timer),await fontSize(label));
      await page.emulateMedia({reducedMotion:'no-preference'});
      assert.equal(await animation(label),'am-tool-running-sweep');
      assert.equal(await animation(timer),'none');
      await page.emulateMedia({reducedMotion:'reduce'});
      assert.equal(await animation(label),'none');
      assert.equal(await color(timer),await color(label));
      const startText=await timer.innerText();
      await page.waitForFunction(text=>document.querySelector('.am-tool-timer')?.textContent!==text,startText);
      laterActivity=true;
      await actionGroup.getByText('5 действий',{exact:true}).waitFor();
      assert.equal(await timer.count(),0);
      await toolGroups.nth(1).getByText('Выполняются команды',{exact:true}).waitFor();
      assert.equal(await page.locator('#thread .am-tool-timer').count(),1);
      commandFinished=true;
      const finishedLabel=actionGroup.getByText('Выполнено 5 команд',{exact:true});
      await finishedLabel.waitFor();
      assert.equal(await color(finishedLabel),await color(toolGroups.nth(1).locator('.am-tool-timer')));
      await page.emulateMedia({reducedMotion:'no-preference'});
      assert.equal(await animation(finishedLabel),'none');
      await page.emulateMedia({reducedMotion:'reduce'});
      assert.equal(await timer.count(),0);
      assert.equal(await actionGroup.getAttribute('open'),'');
      assert.equal(await actionGroup.locator('.am-tool-breakdown').count(),0);
      const labels=actionGroup.locator('.am-action-label');
      assert.deepEqual(await labels.allInnerTexts(),['Найти файл RuntimePanelUtils.cs','Найти упоминания Mouse и UIOwnsKeys в файле FreeCamera.cs','Прочитать шаги ручной проверки','Посмотреть файлы в папке Infantry/Client','Запустить скрипт Python']);
      assert.equal(await labels.locator('svg').count(),5);
      await labels.nth(1).click();
      assert.equal(await actionGroup.locator('details[open] pre').innerText(),"cm cat FreeCamera.cs#cs:3474 | rg -n -C 2 'Mouse|UIOwnsKeys'");
      mixedActions=true;
      await actionGroup.getByText('6 действий',{exact:true}).waitFor();
      if (process.env.SCREENSHOT_DIR) await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, 'frameless.png')});
      await page.locator('.am-task-title',{hasText:remoteTask.title}).click();
      const reply = page.locator('#answer-text');
      await reply.waitFor({state:'visible'});
      const chatType=locator=>locator.evaluate(node=>{
        const {fontSize,fontWeight,lineHeight}=getComputedStyle(node);
        return [fontSize,fontWeight,lineHeight];
      });
      const chatBody=page.locator('#thread .am-message-content').last();
      for(const width of [1100,600]){
        await page.setViewportSize({width,height:600});
        assert.deepEqual(await chatType(chatBody),['14px','400','21px']);
        assert.deepEqual(await chatType(reply),await chatType(chatBody));
      }
      await page.setViewportSize({width:1100,height:600});
      await reply.fill('Use blue');
      await page.locator('#answer-send').click();
      await page.getByText('Поставлено в очередь для исполнителя',{exact:true}).waitFor();
      assert.equal(ownerNotes.length,1);
      assert.equal(ownerNotes[0].task,remoteTask.id);
      assert.equal(ownerNotes[0].text,'Use blue');
      assert.match(ownerNotes[0].op_id,/^window-remote-task-/);
      const remoteAgent=runner.locator('[data-provider=codex]');
      const remoteRunningModel=remoteAgent.locator('.am-agent-parallel-running .am-agent-model');
      const remoteList=page.locator('.am-task',{hasText:remoteTask.title});
      const remoteHead=page.locator('#task-head');
      assert.equal(await remoteList.locator('.am-task-when').getAttribute('datetime'),began);
      remoteRow={...remoteTask,model:'gpt-6.2-sol[effort=low]'};
      await remoteList.getByText('модель не подтверждена',{exact:true}).waitFor();
      assert.equal(await remoteRunningModel.count(),0);
      // The feed may not yet have a model. All three surfaces take the model
      // and state from the task row, and tick without a new payload.
      remoteRow={...remoteRow,ran_model:'gpt-6.2-astra[effort=high]'};
      await remoteHead.getByText(/Astra 6\.2 High/).waitFor();
      assert.equal(await remoteList.locator('.am-task-model').textContent(),'Astra 6.2 High');
      assert.equal(await remoteRunningModel.textContent(),'Astra 6.2 High');
      await remoteAgent.locator('summary').click();
      await page.evaluate(() => {
        const now=Date.now.bind(Date);
        window.elapsedSample=document.querySelector('#thread .am-rich');
        Date.now=()=>now()+60_000;
      });
      await page.waitForFunction(() => {
        const sidebar=document.querySelector('[data-peer=dev-1] [data-provider=codex] [data-elapsed-from]');
        const row=[...document.querySelectorAll('.am-task')].find(node=>node.textContent.includes('Remote task'));
        const right=row?.querySelector('[data-elapsed-from]');
        const center=document.querySelector('#task-head [data-elapsed-from]');
        return sidebar?.textContent==='4 мин'&&right?.textContent==='Работает 4 мин'&&center?.textContent===right.textContent;
      });
      assert.equal(await page.evaluate(()=>window.elapsedSample===document.querySelector('#thread .am-rich')),true);
      // Keep the broker and feed stale while the task row records completion.
      runnerAgents=[...runnerAgents.slice(0,2),{agent:'codex',running:1}];
      runnerWorkers=[{task:remoteTask.id,output_age:0}];
      codexWaiting=false;
      await Promise.all([page.waitForResponse(response=>new URL(response.url()).pathname==='/api/server'),page.locator('[data-tab=server]').click()]);
      await page.locator('[data-tab=tasks]').click();
      remoteRow={...remoteRow,state:'done',finished_at:new Date(Date.now()).toISOString()};
      await remoteHead.getByText(/Работал/).waitFor();
      assert.match(await remoteList.innerText(),/Работал/);
      assert.equal(await remoteAgent.locator('.am-agent-status').count(),0);
      assert.equal(await remoteHead.locator('[data-elapsed-from]').count(),0);
      assert.equal(await remoteList.locator('[data-elapsed-from]').count(),0);

      // The publisher still shows a queued task when its executor reports a live worker.
      const sentRow=page.locator('.am-task',{hasText:'Sent task'});
      const loadRoster=async()=>{
        await Promise.all([page.waitForResponse(r=>new URL(r.url()).pathname==='/api/server'),page.locator('[data-tab=server]').click()]);
        await page.locator('[data-tab=tasks]').click();
      };
      sharedPending=true;
      await sentRow.getByText(/Ждёт ресурс «shared-cache»/).waitFor();
      await sentRow.click();
      runnerAge=60;
      runnerContactAge=0;
      runnerWorkers=[{task:'sent',output_age:0}];
      await loadRoster();
      await runner.getByText('Ждём сведения об агентах',{exact:true}).waitFor();
      assert.equal(await runner.locator('.am-machine-presence').getAttribute('aria-label'),'На связи');
      assert.match(await sentRow.innerText(),/Ждёт ресурс/,'old worker data cannot clear a resource wait');
      runnerAge=0;
      await loadRoster();
      await sentRow.locator('.am-status-run').getByText(/^Работает,/).waitFor();
      await page.locator('#task-head').getByText('Работает',{exact:true}).waitFor();
      assert.equal(await sentRow.locator('[data-elapsed-from]').count(),0,'wait for the actual start time');
      await runner.locator('[data-provider=cursor].am-agent-run').waitFor();
      sharedPending=false;
      await page.locator('.am-task-section',{hasText:'Запланированные'}).waitFor({state:'hidden'});
      await remoteList.click();
      remoteRow=remoteTask;
      await remoteHead.getByText(/Работает/).waitFor();
      // Stop another task from its row without selecting it. The chat button
      // shares the pending guard through both queuing and broker acceptance.
      const stopButton=page.locator('#task-stop'),stopStatus=page.locator('#task-stop-status');
      await page.locator('#task-list').getByText(localTask.title,{exact:true}).click();
      await page.locator('#task-head').getByText(localTask.title,{exact:true}).waitFor();
      const menu=page.locator('#device-menu');
      const stopAction=menu.getByRole('menuitem',{name:'Остановить задачу'});
      const readOnlyRow=page.locator('.am-task-shared').filter({hasText:'Sent task'});
      await readOnlyRow.click({button:'right'});
      assert.equal(await stopAction.count(),0);
      await page.keyboard.press('Escape');
      assert.deepEqual(stopRequests,[]);
      await remoteList.click({button:'right'});
      await stopAction.click();
      assert.equal(await page.locator('#task-head h3').textContent(),localTask.title);
      assert.match(await remoteList.locator('.am-task-status').innerText(),/Ждём завершения задачи/);
      await remoteList.click({button:'right'});
      await menu.getByText('Отправляем запрос на остановку…',{exact:true}).waitFor();
      assert.equal(await menu.getByRole('menuitem').count(),0);
      await page.keyboard.press('Escape');
      await remoteList.click();
      await stopStatus.getByText('Отправляем запрос на остановку…',{exact:true}).waitFor();
      assert.equal(await stopButton.isVisible(),false);
      assert.equal(await reply.isVisible(),false);
      assert.equal(await page.locator('#answer-send').isVisible(),false);
      await page.keyboard.press('Escape');
      releaseStop();
      await stopStatus.getByText(/в очереди/).waitFor();
      assert.equal(await stopButton.isVisible(),false);
      await remoteList.click({button:'right'});
      await menu.getByText(/в очереди/).waitFor();
      remoteRow={...remoteTask,remote:{...remoteTask.remote,cancel_state:'sent'}};
      await stopStatus.getByText('Запрос на остановку отправлен. Ждём завершения задачи.',{exact:true}).waitFor();
      assert.match(await remoteList.locator('.am-task-status').innerText(),/Ждём завершения задачи/);
      assert.equal(await stopButton.isVisible(),false);
      await menu.getByText('Запрос на остановку отправлен. Ждём завершения задачи.',{exact:true}).waitFor();
      assert.equal(await menu.getByRole('menuitem').count(),0);
      await page.keyboard.press('Escape');
      await page.keyboard.press('Escape');
      assert.deepEqual(stopRequests,[{task:remoteTask.id}]);
      remoteRow={...remoteRow,state:'cancelled',finished_at:new Date().toISOString()};
      await remoteHead.getByText('Отменена',{exact:true}).waitFor();
      assert.equal(await stopStatus.isVisible(),false);
      await remoteList.click({button:'right'});
      assert.equal(await stopAction.count(),0);
      await page.keyboard.press('Escape');
      remoteRow=remoteTask;
      await remoteHead.getByText(/Работает/).waitFor();
      refuseStop=true;
      await reply.fill('Unsent draft');
      await stopButton.click();
      await page.getByText('Fixture stop refusal',{exact:true}).waitFor();
      assert.equal(await stopButton.isVisible(),true);
      assert.equal(await page.locator('#answer-send').isVisible(),true);
      assert.equal(await reply.inputValue(),'Unsent draft');
      assert.equal(await stopStatus.isVisible(),false);
      refuseStop=false;
      await page.locator('#task-list').getByText(localTask.title,{exact:true}).click();
      await page.locator('#task-head h3').getByText(localTask.title,{exact:true}).waitFor();
      await stopButton.click();
      await stopStatus.getByText('Запрос на остановку отправлен. Ждём завершения задачи.',{exact:true}).waitFor();
      assert.equal(await reply.isVisible(),false);
      localRow={...localTask,state:'cancelled',finished_at:new Date().toISOString()};
      await page.locator('#task-head').getByText('Отменена',{exact:true}).waitFor();
      assert.equal(await stopStatus.isVisible(),false);
      localRow=localTask;
      await page.locator('#task-head').getByText(/Работает/).waitFor();
      // Owner, 2026-11-15: one button removes every finished task from the list.
      await page.locator('.am-task-section', {hasText: 'Завершённые'}).locator('.am-task-section-action').click();
      await page.locator('#confirm-go').click();
      await page.waitForFunction(() => !document.querySelector('#confirm-dialog').open);
      assert.deepEqual(hides, [{peer: 'host', task: 'old-1'}, {peer: 'host', task: 'old-2'}]);
      // Feedback: the export takes the new remarks, and the exported ones can
      // be cleared once passed on (owner, 2026-11-16).
      await page.locator('#fb-open').click();
      assert.equal(await page.locator('#fb-export').innerText(), 'Выгрузить новые отзывы (2)');
      await page.locator('#fb-export').click();
      await page.locator('#fb-clear').waitFor({state: 'visible'});
      await page.locator('#fb-clear').click();
      await page.locator('#confirm-go').click();
      await page.waitForFunction(() => document.querySelector('#fb-status').textContent === 'Журнал очищен');
      assert.equal(clears.length, 1);
      assert.equal(await page.locator('#fb-export').isDisabled(), true);
      await page.locator('#fb-close').click();
      await page.locator('#task-list').getByText(localTask.title,{exact:true}).click();
      const composer = page.locator('#answer-text');
      await composer.fill('Unsent note for Frame');
      delaySiteView = true;
      await page.locator('[aria-controls="project-select-menu"]').click();
      await page.locator('#project-select-menu').getByText('Site',{exact:true}).click();
      try {
        // The old rows stay in place so the list does not jump, but they
        // cannot be chosen while Site loads (T-266).
        assert(await page.locator('#task-list').evaluate(list => list.inert && list.getAttribute('aria-busy') === 'true'), 'keep the old project list inert while Site loads');
        assert(await composer.isHidden());
      } finally {
        releaseSite();
      }
      await page.locator('#task-head h3').filter({hasText:siteTask.title}).waitFor();
      const taskSection=title=>page.locator('.am-task-title',{hasText:title}).evaluate(node=>{
        let heading=node.closest('.am-task').previousElementSibling;
        while(heading&&!heading.classList.contains('am-task-section'))heading=heading.previousElementSibling;
        return heading?.querySelector('span')?.textContent;
      });
      assert.equal(await taskSection(siteTask.title),'Запланированные');
      assert.match(await page.locator('#task-head').innerText(),/Ждёт ресурс «shared-cache»/);
      await composer.waitFor();
      assert.equal(await composer.inputValue(), '', 'a draft must not follow a task from another project');
      assert(await page.locator('#answer-send').isDisabled());
      siteTask={...siteTask,pending_launch:false,waits_for_lock:undefined,started_at:began};
      await page.locator('#task-head').getByText(/Работает/).waitFor();
      assert.equal(await taskSection(siteTask.title),'Активные');
      assert.equal(await page.locator('.am-task-section',{hasText:'Запланированные'}).count(),0);
      siteTask={...siteTask,pending_launch:true};
      await page.locator('#task-head').getByText('Ждёт запуска',{exact:true}).waitFor();
      assert.equal(await taskSection(siteTask.title),'Активные');
      // One update button: the dialog follows the download, the install and
      // the restart; a failed install says so and offers to try again.
      const updateButton = page.locator('#app-update-button');
      await updateButton.waitFor({state: 'visible'});
      assert(!(await updateButton.evaluate(b => b.classList.contains('am-update-loading'))), 'a found update shows a download ring');
      assert.equal(await page.locator('#app-installed-version').innerText(),'v0.98');
      await updateButton.click();
      await page.locator('#update-dialog').waitFor({state: 'visible'});
      assert.deepEqual(updates, [{action: 'update'}]);
      update.done = 40;
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Загрузка… 40%');
      assert.equal(await page.locator('#update-title').innerText(), 'Обновление v0.99');
      update.state = 'installing';
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Установка…');
      update.state = 'available';
      await page.locator('#update-error').waitFor({state: 'visible'});
      assert.equal(await page.locator('#update-error').innerText(), 'Обновление не установлено. Программа осталась прежней.');
      await page.keyboard.press('Escape');
      await page.locator('#update-dialog').waitFor({state: 'hidden'});
      await updateButton.click();
      await page.locator('#update-dialog').waitFor({state: 'visible'});
      update.state = 'downloading';
      await page.keyboard.press('Escape');
      assert(await page.locator('#update-dialog').isVisible(), 'the update dialog closed while downloading');
      update.state = 'installed';
      await page.waitForFunction(() => document.querySelector('#update-step').textContent === 'Перезапуск…');
      assert(await page.locator('#update-actions').isHidden(), 'buttons shown while restarting');
      assert.equal(updates.length, 2);
      if (process.env.SCREENSHOT_DIR) await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, 'update-dialog.png')});
      console.log(JSON.stringify({engine, frameless: true, result: 'passed'}));
      await context.close();
    }
    // A completed page stays on screen when periodic replies carry only
    // active tasks. Loading and opening an older card is a one-off request.
    {
      const context = await browser.newContext();
      const page = await context.newPage();
      page.setDefaultTimeout(6000);
      const at = '2026-11-23T09:00:00Z';
      const makeTask = (id, title, state) => ({id,title,state,agent:'codex',created_at:at,updated_at:at});
      const recent = Array.from({length:10}, (_,i)=>makeTask(`recent-${i}`,`Recent task ${i+1}`,'done'));
      const active = makeTask('active','Active fixture','running');
      const older = makeTask('older','Older fixture','done');
      const historyRequests = [], feeds = [];
      let unchanged = 0;
      await page.route('**/*', async route=>{
        const {pathname,searchParams} = new URL(route.request().url());
        if(pathname==='/api/window'){
          const v={projects:[{name:'Game'}],project:'Game',folder:'/fictional/project',computer_name:'Fixture Mac',agents:[],shared:[],history_paged:true,history_version:'history-fixture'};
          if(searchParams.has('history_version')){unchanged++;return route.fulfill({json:{...v,tasks:[active],history_unchanged:true}});}
          return route.fulfill({json:{...v,tasks:[active,...recent],history_next:'recent-9'}});
        }
        if(pathname==='/api/task-history'){
          historyRequests.push(searchParams.get('before'));
          return route.fulfill({json:{tasks:[older]}});
        }
        if(pathname==='/api/task'){
          const id=searchParams.get('id');feeds.push(id);
          const task=[active,older,...recent].find(t=>t.id===id);
          return route.fulfill({json:{task,items:[{kind:'task',author:'coordinator',text:'Fixture '+id,at}]}});
        }
        if(pathname.startsWith('/api/'))return route.fulfill({json:{}});
        const file=pathname==='/'?'index.html':pathname.slice(1);
        return route.fulfill({path:path.join(assets,file)});
      });
      await page.goto('http://127.0.0.1:5476/#fixture-token');
      await page.getByText('Recent task 10',{exact:true}).waitFor();
      assert.equal(await page.locator('#task-list .am-task-title').count(),11);
      assert.deepEqual(historyRequests,[]);
      await page.getByRole('button',{name:'Показать ещё 10',exact:true}).click();
      const row=page.locator('#task-list .am-task').filter({hasText:'Older fixture'});
      await row.click();
      await page.locator('#thread').getByText('Fixture older',{exact:true}).waitFor();
      const poll = () => page.waitForResponse(response=>new URL(response.url()).searchParams.has('history_version'));
      await poll();await poll();
      assert(unchanged>=2);
      assert.deepEqual(historyRequests,['recent-9']);
      assert.equal(feeds.filter(id=>id==='older').length,1);
      assert.equal(await page.locator('#task-list .am-task-title').count(),12);
      await context.close();
    }
    // An idle window waits for a revision and requests just the appended
    // messages when the watched transcript changes.
    {
      const context=await browser.newContext();
      const page=await context.newPage();
      page.setDefaultTimeout(6000);
      const at='2026-11-23T09:00:00Z';
      const task={id:'active',title:'Event task',state:'running',agent:'codex',created_at:at,updated_at:at};
      const first={kind:'task',author:'coordinator',text:'Initial message',at};
      const progress={kind:'text',author:'worker',text:'Appended message',at};
      let revision='1', windows=0, setups=0;
      const cursors=[],waiting=new Set();
      await page.route('**/*',async route=>{
        const {pathname,searchParams:q}=new URL(route.request().url());
        if(pathname==='/api/window'){
          windows++;
          return route.fulfill({json:{projects:[{name:'Game'}],project:'Game',folder:'/fictional',agents:[],shared:[],tasks:[task],change_watch:true}});
        }
        if(pathname==='/api/window-changes'){
          if(q.get('window')==='1.0'&&q.get('feed')===revision)await new Promise(resolve=>waiting.add(resolve));
          return route.fulfill({json:{window:'1.0',feed:revision}}).catch(()=>{});
        }
        if(pathname==='/api/task'){
          const cursor=q.get('cursor');cursors.push(cursor);
          if(revision==='2'&&cursor==='first')return route.fulfill({json:{task,items:[progress],cursor:'second',append_from:1}});
          return route.fulfill({json:{task,items:revision==='2'?[first,progress]:[first],cursor:revision==='2'?'second':'first'}});
        }
        if(pathname==='/api/agent-setup-status'){setups++;return route.fulfill({json:[]});}
        if(pathname.startsWith('/api/'))return route.fulfill({json:{}});
        return route.fulfill({path:path.join(assets,pathname==='/'?'index.html':pathname.slice(1))});
      });
      try{
        const waitingForChanges=page.waitForRequest(request=>{
          const url=new URL(request.url());
          return url.pathname==='/api/window-changes'&&url.searchParams.get('task')==='active'&&url.searchParams.get('feed')==='1';
        });
        await page.goto('http://127.0.0.1:5477/#fixture-token');
        const thread=page.locator('#thread');
        await thread.getByText('Initial message',{exact:true}).waitFor();
        await waitingForChanges;
        const counts=[windows,cursors.length,setups];
        await page.waitForTimeout(2400);
        assert.deepEqual([windows,cursors.length,setups],counts,'idle requests were repeated');
        revision='2';waiting.forEach(done=>done());waiting.clear();
        await thread.getByText('Appended message',{exact:true}).waitFor();
        assert.equal(cursors.at(-1),'first');
        assert.equal(windows,counts[0],'feed-only change reloaded the task board');
        assert.equal(await thread.getByText('Initial message',{exact:true}).count(),1);
      } finally {
        waiting.forEach(done=>done());await context.close();
      }
    }
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
