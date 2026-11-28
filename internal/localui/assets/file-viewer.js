'use strict';
window.taskFiles = (() => {
  const make = (tag, text, cls) => {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (cls) node.className = cls;
    return node;
  };
  const slash = /[\\/]/;
  const markdownLink = /\[([^\]\n]+)\][ \t]*(?:\r?\n[ \t]*)?\((<[^>\n]+>|(?:[^()\n]|\([^()\n]*\))+)\)/g;
  const legacyHeading = /^\s*(?:\*\*)?(?:файлы|files|changed files|modified files)(?:\*\*)?\s*:(?:\*\*)?/i;
  const filename = /(?:\.[\p{L}\p{N}][\p{L}\p{N}_-]{0,19}|^(?:Dockerfile|Makefile|LICENSE|NOTICE|README|CHANGELOG|AGENTS))$/iu;

  function reference(value) {
    const wrapped = value.trim();
    let path = wrapped.startsWith('<') && wrapped.endsWith('>') ? wrapped.slice(1, -1) : wrapped;
    const position = path.match(/(?:#L(\d+)(?:-L\d+)?|:(\d+))$/i);
    if (position) path = path.slice(0, position.index);
    // URLs and arbitrary Markdown fragment links are not project file references.
    if (!path || path.startsWith('//') || /^[a-z][\w+.-]*:/i.test(path) && !/^[a-z]:[\\/]/i.test(path)) return null;
    try { path = decodeURIComponent(path); } catch (_) {}
    if (!path || /[\x00-\x1f]/.test(path)) return null;
    const name = path.split(slash).pop();
    if (!filename.test(name)) return null;
    return {path, name, line:Number(position?.[1] || position?.[2]) || 0};
  }

  function fileButton(label, ref, source) {
    if (source.directory && !/^(?:[/\\]|[a-z]:[\\/])/i.test(ref.path)) {
      ref = {...ref, path:source.directory + ref.path};
    }
    const link = make('button', label, 'am-file-link');
    link.type = 'button';
    link.title = ref.path;
    link.addEventListener('click', () => open(ref, source));
    link.addEventListener('contextmenu', event => {
      event.preventDefault();
      showLinkMenu(ref, source, link, event.clientX, event.clientY);
    });
    return link;
  }

  function inline(parent, text, source, styled = false) {
    const pattern = new RegExp('(`[^`\\n]+`)|' + markdownLink.source + (styled ? '|(\\*\\*[^*\\n]+\\*\\*|\\*[^*\\n]+\\*)' : ''), 'g');
    let offset = 0;
    for (const match of text.matchAll(pattern)) {
      const [raw, code, label, uri, emphasis] = match;
      if (offset < match.index) parent.append(text.slice(offset, match.index));
      offset = match.index + raw.length;
      if (code) {
        parent.append(make('code', code.slice(1, -1), 'am-code'));
      } else if (label !== undefined) {
        const ref = reference(uri);
        parent.append(ref ? fileButton(label, ref, source) : document.createTextNode(raw));
      } else if (emphasis) {
        const strong = emphasis.startsWith('**');
        const trim = strong ? 2 : 1;
        parent.append(make(strong ? 'strong' : 'em', emphasis.slice(trim, -trim)));
      }
    }
    if (offset < text.length) parent.append(text.slice(offset));
    return parent;
  }

  function legacyLine(parent, line, source) {
    const spans = line.split(/(,\s*)/);
    const paths = spans.filter((_, index) => index % 2 === 0 && spans[index].trim());
    const entries = paths.map(path => {
      const pieces = path.match(/^(\s*(?:[-*]\s+)?)(.+?)(\s*)$/);
      if (!pieces) return null;
      const raw = pieces[2].replace(/^`([^`]+)`$/, '$1');
      const ref = reference(raw);
      if (!ref || !path.includes('/') && !path.includes('\\') && /\s/.test(raw)) return null;
      return {prefix:pieces[1], raw, suffix:pieces[3], ref};
    });
    if (!entries.length || entries.some(entry => !entry)) return false;
    let position = 0;
    for (const [index, value] of spans.entries()) {
      if (index % 2 || !value.trim()) { parent.append(value); continue; }
      const item = entries[position++];
      parent.append(item.prefix, fileButton(item.raw, item.ref, source), item.suffix);
    }
    return true;
  }

  function paragraph(parent, text, source, report) {
    let listing = false;
    // Keep a hard break in prose while accepting a wrapped file reference.
    const joined=text.replace(markdownLink,(raw,label,path)=>reference(path)?'['+label+']('+path+')':raw);
    joined.split('\n').forEach((line, index) => {
      if (index) parent.append('\n');
      const heading = report ? line.match(legacyHeading) : null;
      if (heading) { parent.append(heading[0]); line = line.slice(heading[0].length); listing = true; }
      if (listing && (!line.trim() || legacyLine(parent, line, source))) return;
      listing = false;
      inline(parent, line, source);
    });
    return parent;
  }

  function markdown(parent, text, source, line) {
    const lines = text.split(/\r?\n/);
    let target = null;
    for (let index = 0; index < lines.length;) {
      const start = index;
      const row = lines[index++];
      if (!row.trim()) continue;
      let node;
      const fence = row.match(/^\s*(`{3,}|~{3,})/);
      const heading = row.match(/^(#{1,6})\s+(.+)$/);
      const list = row.match(/^\s*(?:([-+*])|(\d+)[.)])\s+(.+)$/);
      const table = row.includes('|') && /^\s*\|?\s*:?-{3,}:?(?:\s*\|\s*:?-{3,}:?)+\s*\|?\s*$/.test(lines[index] || '');
      if (fence) {
        const contents = [];
        while (index < lines.length && !lines[index].trimStart().startsWith(fence[1])) contents.push(lines[index++]);
        if (index < lines.length) index++;
        node = make('pre', contents.join('\n'), 'am-code-block');
      } else if (heading) {
        node = inline(make('h' + heading[1].length), heading[2], source, true);
      } else if (table) {
        node = make('table', undefined, 'am-file-table');
        const addRow = (section, value, tag) => {
          const tr = make('tr');
          for (const cell of value.trim().replace(/^\||\|$/g, '').split('|')) {
            tr.append(inline(make(tag), cell.trim(), source, true));
          }
          section.append(tr);
          return tr;
        };
        const head = make('thead'), rows = make('tbody');
        addRow(head, row, 'th');
        index++;
        while (index < lines.length && lines[index].trim() && lines[index].includes('|')) {
          const tr = addRow(rows, lines[index++], 'td');
          if (index === line) target = tr;
        }
        node.append(head, rows);
      } else if (list) {
        const ordered = !!list[2];
        const marker = ordered ? /^\s*\d+[.)]\s+(.+)$/ : /^\s*[-+*]\s+(.+)$/;
        node = make(ordered ? 'ol' : 'ul');
        if (ordered) node.start = Number(list[2]);
        node.append(inline(make('li'), list[3], source, true));
        while (index < lines.length && marker.test(lines[index])) {
          const value = lines[index++].match(marker)[1];
          const li = inline(make('li'), value, source, true);
          if (index === line) target = li;
          node.append(li);
        }
      } else if (row.startsWith('> ')) {
        node = inline(make('blockquote'), row.slice(2), source, true);
      } else {
        node = inline(make('p'), row, source, true);
        while (index < lines.length && lines[index].trim() && !/^\s*(?:#{1,6}\s|`{3,}|~{3,}|>\s|[-+*]\s|\d+[.)]\s)/.test(lines[index])) {
          node.append(' ');
          inline(node, lines[index++], source, true);
        }
      }
      parent.append(node);
      if (!target && line > start && line <= index) target = node;
    }
    return target;
  }

  function plain(parent, text, targetLine) {
    const pre = make('pre', undefined, 'am-file-plain');
    let target = null;
    if (targetLine > 0) {
      const lines = text.split('\n');
      if (targetLine <= lines.length) {
        target = make('span', lines[targetLine - 1]);
        pre.append(lines.slice(0, targetLine - 1).join('\n') + (targetLine > 1 ? '\n' : ''), target,
          targetLine < lines.length ? '\n' + lines.slice(targetLine).join('\n') : '');
      }
    }
    if (!target) pre.textContent = text;
    parent.append(pre);
    return target;
  }

  let panel, tabs, pathLabel, body, status, actions, linkMenu;
  const documents = [];
  const nativeSaves = new Map();
  let active;

  function setup() {
    const layout = document.getElementById('task-layout') || document.getElementById('am-concept');
    panel = make('section', undefined, 'am-file-viewer');
    panel.hidden = true;
    panel.setAttribute('aria-label', tr('Содержимое файла'));
    tabs = make('div', undefined, 'am-pane-strip am-file-tabs');
    tabs.setAttribute('role', 'tablist');
    tabs.setAttribute('aria-label', tr('Открытые файлы'));
    tabs.hidden = true;
    const toolbar = layout.querySelector('.am-task-toolbar');
    if (toolbar) toolbar.insertBefore(tabs, toolbar.lastElementChild);
    else panel.append(tabs);
    pathLabel = make('div', undefined, 'am-file-path am-muted');
    actions = make('div', undefined, 'am-file-actions');
    status = make('p', undefined, 'am-file-status am-muted');
    status.setAttribute('role', 'status');
    body = make('div', undefined, 'am-dialog-body am-file-content');
    body.id = 'am-file-content';
    body.tabIndex = 0;
    body.setAttribute('role', 'tabpanel');
    panel.append(pathLabel, actions, status, body);
    layout.append(panel);
  }

  function closeTab(doc) {
    const index = documents.indexOf(doc);
    documents.splice(index, 1);
    doc.tab.remove();
    if (doc !== active) { active?.button.focus(); return; }
    const next = documents[index] || documents[index - 1];
    if (next) { show(next); next.button.focus(); return; }
    active = null;
    panel.hidden = tabs.hidden = true;
    body.replaceChildren();
    actions.replaceChildren();
    (doc.opener?.isConnected ? doc.opener : document.getElementById('thread'))?.focus({preventScroll:true});
  }

  function addTab(ref, source, key) {
    const tab = make('div', undefined, 'am-pane-tab');
    const button = make('button', ref.name, 'am-quiet-button');
    button.type = 'button';
    button.id = 'am-file-tab-' + crypto.randomUUID();
    button.title = ref.path;
    button.setAttribute('role', 'tab');
    button.setAttribute('aria-controls', body.id);
    const close = make('button', undefined, 'am-icon-btn am-close-x');
    close.type = 'button';
    close.setAttribute('aria-label', tr('Закрыть'));
    close.title = tr('Закрыть') + ' ' + ref.name;
    close.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>';
    const doc = {ref, source, key, tab, button, opener:document.activeElement, scroll:0};
    button.onclick = () => show(doc);
    button.onkeydown = event => {
      const offset = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
      if (!offset) return;
      event.preventDefault();
      const next = documents[(documents.indexOf(doc) + offset + documents.length) % documents.length];
      show(next);
      next.button.focus();
    };
    close.onclick = () => closeTab(doc);
    tab.append(button, close);
    tabs.append(tab);
    documents.push(doc);
    return doc;
  }

  function showLinkMenu(ref, source, target, x, y) {
    if (!linkMenu) {
      linkMenu = make('div', undefined, 'am-select-menu am-context-menu');
      linkMenu.setAttribute('popover', 'auto');
      linkMenu.setAttribute('role', 'menu');
      document.getElementById('am-concept').append(linkMenu);
    }
    if (linkMenu.matches(':popover-open')) linkMenu.hidePopover();
    const item = (label, run) => {
      const button = make('button', tr(label), 'am-select-option');
      button.type = 'button';
      button.setAttribute('role', 'menuitem');
      button.onclick = () => { linkMenu.hidePopover(); target.focus(); run(); };
      return button;
    };
    linkMenu.replaceChildren(
      item('Открыть файл', () => open(ref, source)),
      item('Копировать путь', () => { void navigator.clipboard.writeText(ref.path).catch(() => {}); }),
    );
    linkMenu.showPopover();
    linkMenu.style.left = Math.max(0, Math.min(x, innerWidth - linkMenu.offsetWidth)) + 'px';
    linkMenu.style.top = Math.max(0, Math.min(y, innerHeight - linkMenu.offsetHeight)) + 'px';
    linkMenu.firstElementChild.focus({preventScroll:true});
  }

  function action(label, listener) {
    const button = make('button', tr(label), 'am-inline-action');
    button.type = 'button';
    button.addEventListener('click', listener);
    actions.append(button);
    return button;
  }
  function setStatus(doc, message) {
    doc.message = message;
    if (active === doc) status.textContent = message;
  }
  function copy(doc, value) {
    return async () => {
      try { await navigator.clipboard.writeText(value); setStatus(doc, tr('Скопировано')); }
      catch (_) { setStatus(doc, tr('Не удалось скопировать')); }
    };
  }
  function save(doc, data, name) {
    const handler = window.webkit?.messageHandlers?.reportFileSave;
    if (handler) {
      const id = crypto.randomUUID();
      nativeSaves.set(id, doc);
      handler.postMessage({id, name, data:data || ''});
      return;
    }
    const url = URL.createObjectURL(new Blob([Uint8Array.from(atob(data || ''), ch => ch.charCodeAt(0))], {type:'application/octet-stream'}));
    const a = make('a'); a.download = name; a.href = url;
    panel.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function saved(id, result) {
    const doc = nativeSaves.get(id);
    nativeSaves.delete(id);
    if (!doc || result === 'cancelled') return;
    setStatus(doc, tr(result === 'saved' ? 'Файл сохранён' : 'Не удалось сохранить файл'));
  }

  async function open(ref, source) {
    if (!panel) setup();
    const key = JSON.stringify([source.scope, source.remote, ref.path]);
    const existing = documents.find(doc => doc.key === key);
    const doc = existing || addTab(ref, source, key);
    if (ref.line && ref.line !== doc.ref.line) {
      doc.ref = ref;
      doc.scroll = 0;
    }
    if (!existing) doc.message = source.remote
      ? tr('Файл находится на компьютере «{computer}».').replace('{computer}', () => source.remote)
      : tr('Открываем файл…');
    show(doc);
    doc.button.focus({preventScroll:true});
    if (existing || source.remote) return;
    try {
      doc.file = await source.request(ref, 'preview');
      doc.message = !doc.file.format ? tr(doc.file.can_save ? 'Этот формат нельзя прочитать в окне.' : 'Файл слишком большой для просмотра.') : '';
    } catch (error) { doc.message = error.message; }
    if (doc === active) render(doc, true);
  }

  function show(doc) {
    if (active && active !== doc) active.scroll = body.scrollTop;
    active = doc;
    panel.hidden = tabs.hidden = false;
    for (const item of documents) {
      item.button.tabIndex = item === doc ? 0 : -1;
      item.button.setAttribute('aria-selected', String(item === doc));
    }
    body.setAttribute('aria-labelledby', doc.button.id);
    render(doc, !doc.scroll);
  }

  function openSeparate(doc) {
    const {file, ref} = doc;
    const handler = window.webkit?.messageHandlers?.reportFileWindow;
    if (handler) {
      handler.postMessage({name:file.name || ref.name, text:file.text || '', line:ref.line});
      return;
    }
    const popup = window.open('', '_blank', 'popup,resizable,width=860,height=680');
    if (!popup) { setStatus(doc, tr('Не удалось открыть отдельное окно')); return; }
    popup.opener = null;
    const preview = popup.document;
    preview.title = file.name || ref.name;
    const theme = getComputedStyle(document.getElementById('am-concept'));
    preview.documentElement.style.colorScheme = theme.colorScheme;
    const pre = preview.createElement('pre');
    pre.style.whiteSpace = 'pre-wrap';
    pre.style.overflowWrap = 'anywhere';
    pre.style.fontFamily = theme.getPropertyValue('--am-mono');
    pre.style.fontSize = theme.getPropertyValue('--am-text-m');
    pre.style.lineHeight = '1.5';
    pre.style.margin = theme.getPropertyValue('--am-space-6');
    const text = file.text || '';
    const lines = ref.line ? text.split('\n') : [];
    const target = ref.line <= lines.length ? lines[ref.line - 1] : undefined;
    if (target !== undefined) {
      const marker = preview.createElement('mark');
      marker.textContent = target;
      pre.append(lines.slice(0,ref.line - 1).join('\n') + (ref.line > 1 ? '\n' : ''), marker, '\n' + lines.slice(ref.line).join('\n'));
      preview.body.append(pre);
      marker.scrollIntoView({block:'center'});
    } else {
      pre.textContent = text;
      preview.body.append(pre);
    }
  }

  function render(doc, revealLine) {
    const {ref, source, file} = doc;
    const path = file?.path || ref.path;
    pathLabel.textContent = path;
    status.textContent = doc.message || '';
    body.replaceChildren();
    actions.replaceChildren();
    action('Копировать путь', copy(doc, path));
    if (!file) return;
    if (file.format) {
      action('Открыть в отдельном окне', () => openSeparate(doc));
      action('Копировать содержимое файла', copy(doc, file.text || ''));
    }
    const run = (verb, label, then) => {
      const button = action(label, async () => {
        button.disabled = true;
        try {
          const result = await source.request(ref, verb);
          then?.(result);
        } catch (error) { setStatus(doc, error.message); }
        finally { button.disabled = false; }
      });
    };
    if (file.can_save) run('download', 'Сохранить как…', value => save(doc, value.data, value.name));
    if (file.can_edit) run('edit', 'Открыть в текстовом редакторе');
    if (file.can_reveal) run('reveal', 'Показать в папке');
    const directory = ref.path.slice(0, Math.max(ref.path.lastIndexOf('/'), ref.path.lastIndexOf('\\')) + 1);
    const target = file.format === 'markdown' ? markdown(body, file.text || '', {...source, directory}, ref.line)
      : file.format === 'text' ? plain(body, file.text || '', ref.line) : null;
    if (target) target.classList.add('am-file-target');
    body.scrollTop = doc.scroll;
    if (target && revealLine) body.scrollTop = target.getBoundingClientRect().top - body.getBoundingClientRect().top - body.clientHeight / 2;
  }
  function link(label, path, source) {
    const name = path.split(slash).pop();
    return name ? fileButton(label, {path, name, line:0}, source) : document.createTextNode(label);
  }
  return {paragraph, link, saved};
})();
