'use strict';
window.taskFiles = (() => {
  const make = (tag, text, cls) => {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (cls) node.className = cls;
    return node;
  };
  const slash = /[\\/]/;
  const markdownLink = /\[([^\]\n]+)\]\((<[^>\n]+>|(?:[^()\n]|\([^()\n]*\))+)\)/g;
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
    const link = make('button', label, 'am-file-link');
    link.type = 'button';
    link.title = ref.path;
    link.addEventListener('click', () => open(ref, source));
    link.addEventListener('contextmenu', event => {
      event.preventDefault();
      open(ref, source);
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
    text.split('\n').forEach((line, index) => {
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

  let dialog, title, pathLabel, body, status, actions;
  let selected = 0;
  let nativeSave;
  function setup() {
    dialog = make('dialog', undefined, 'am-viewer am-file-viewer');
    dialog.setAttribute('aria-labelledby', 'am-file-title');
    const header = make('div', undefined, 'am-viewer-head');
    title = make('h3');
    title.id = 'am-file-title';
    const close = make('button', '×', 'am-icon-btn am-close-x');
    close.type = 'button';
    close.autofocus = true;
    close.setAttribute('aria-label', tr('Закрыть'));
    close.title = tr('Закрыть');
    close.addEventListener('click', () => dialog.close());
    header.append(title, close);
    pathLabel = make('div', undefined, 'am-file-path am-muted');
    actions = make('div', undefined, 'am-file-actions');
    status = make('p', undefined, 'am-file-status am-muted');
    status.setAttribute('role', 'status');
    body = make('div', undefined, 'am-dialog-body am-file-content');
    body.tabIndex = 0;
    body.setAttribute('aria-label', tr('Содержимое файла'));
    dialog.append(header, pathLabel, actions, status, body);
    dialog.addEventListener('close', () => { selected++; body.replaceChildren(); actions.replaceChildren(); });
    document.getElementById('am-concept').append(dialog);
  }

  function action(label, listener) {
    const button = make('button', tr(label), 'am-inline-action');
    button.type = 'button';
    button.addEventListener('click', listener);
    actions.append(button);
    return button;
  }
  function copy(value) {
    const original = selected;
    return async () => {
      try { await navigator.clipboard.writeText(value); if (selected === original) status.textContent = tr('Скопировано'); }
      catch (_) { if (selected === original) status.textContent = tr('Не удалось скопировать'); }
    };
  }
  function save(data, name) {
    const handler = window.webkit?.messageHandlers?.reportFileSave;
    if (handler) {
      const id = crypto.randomUUID();
      nativeSave = {id, selection:selected};
      handler.postMessage({id, name, data:data || ''});
      return;
    }
    const url = URL.createObjectURL(new Blob([Uint8Array.from(atob(data || ''), ch => ch.charCodeAt(0))], {type:'application/octet-stream'}));
    const a = make('a'); a.download = name; a.href = url;
    dialog.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function saved(id, result) {
    if (nativeSave?.id !== id) return;
    const selection = nativeSave.selection;
    nativeSave = null;
    if (selection !== selected || result === 'cancelled') return;
    status.textContent = tr(result === 'saved' ? 'Файл сохранён' : 'Не удалось сохранить файл');
  }

  function relativeSource(src, path) {
    const dir = path.slice(0, Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1);
    return {...src, request: (ref, action) => src.request({...ref, path:/^(?:[/\\]|[a-z]:[\\/])/i.test(ref.path) ? ref.path : dir + ref.path}, action)};
  }

  async function open(ref, source) {
    if (!dialog) setup();
    const current = ++selected;
    title.textContent = ref.name;
    pathLabel.textContent = ref.path;
    status.textContent = '';
    body.replaceChildren();
    actions.replaceChildren();
    action('Копировать путь', copy(ref.path));
    if (!dialog.open) dialog.showModal();
    if (source.remote) {
      status.textContent = tr('Файл находится на компьютере «{computer}».').replace('{computer}', () => source.remote);
      return;
    }
    status.textContent = tr('Открываем файл…');
    let file;
    try { file = await source.request(ref, 'preview'); }
    catch (error) {
      if (current === selected) status.textContent = error.message;
      return;
    }
    if (current !== selected) return;
    pathLabel.textContent = file.path;
    status.textContent = !file.format ? tr(file.can_save ? 'Этот формат нельзя прочитать в окне.' : 'Файл слишком большой для просмотра.') : '';
    actions.replaceChildren();
    action('Копировать путь', copy(file.path));
    if (file.format) action('Копировать содержимое файла', copy(file.text || ''));
    const run = (verb, label, then) => {
      const button = action(label, async () => {
        button.disabled = true;
        try {
          const result = await source.request(ref, verb);
          if (current === selected) then?.(result);
        } catch (error) {
          if (current === selected) status.textContent = error.message;
        } finally { button.disabled = false; }
      });
    };
    if (file.can_save) run('download', 'Сохранить как…', value => save(value.data, value.name));
    if (file.can_edit) run('edit', 'Открыть в текстовом редакторе');
    if (file.can_reveal) run('reveal', 'Показать в папке');
    const target = file.format === 'markdown' ? markdown(body, file.text || '', relativeSource(source, ref.path), ref.line)
      : file.format === 'text' ? plain(body, file.text || '', ref.line) : null;
    if (target) {
      target.classList.add('am-file-target');
      target.scrollIntoView({block:'center'});
    } else body.scrollTop = 0;
  }
  function link(label, path, source) {
    const name = path.split(slash).pop();
    return name ? fileButton(label, {path, name, line:0}, source) : document.createTextNode(label);
  }
  return {paragraph, link, saved};
})();
