// ProtoMock UI: browse, create, edit, validate and try mocks.
// Plain JavaScript, no build step. All data is inserted as text (never as HTML).
'use strict';

const API = '/protomock-ui/api';
const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];

const state = {
  data: null,          // GET /mocks response
  filter: '',
  collapsed: new Set(),// collapsed mock keys
  sel: null,           // {type:'mock'|'stub', kind, mock, file?}
  editor: null,        // stub editor state (see openStub)
};

// ---------- tiny DOM helper ----------

// h('div', {class: 'x', onclick: fn}, 'text', child, [children])
function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'value') el.value = v;
    else if (k === 'checked' || k === 'disabled' || k === 'selected') el[k] = Boolean(v);
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === undefined || c === null || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}
const $ = (sel) => document.querySelector(sel);
const mount = (el, ...children) => { el.replaceChildren(...children.flat(Infinity).filter(Boolean)); };

function toast(msg, isError) {
  const t = h('div', {class: 'toast' + (isError ? ' toast-err' : '')}, msg);
  $('#toasts').append(t);
  setTimeout(() => t.remove(), isError ? 7000 : 3000);
}

function debounce(fn, ms) {
  let t;
  return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

// ---------- API ----------

async function api(method, path, {query, body, raw} = {}) {
  let url = API + path;
  if (query) url += '?' + new URLSearchParams(query);
  const opts = {method, headers: {}};
  if (body !== undefined) {
    opts.body = raw ? body : JSON.stringify(body);
    opts.headers['Content-Type'] = raw ? 'text/plain' : 'application/json';
  }
  const res = await fetch(url, opts);
  const text = await res.text();
  let data;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) throw new Error((data && data.error) || `${res.status} ${res.statusText}`);
  return data;
}

async function load(data) {
  state.data = data || await api('GET', '/mocks');
  renderHeader();
  renderTree();
}

// ---------- helpers ----------

const mockKey = (kind, name) => `${kind}/${name}`;
const findMock = (kind, name) => state.data?.mocks.find(m => m.kind === kind && m.name === name);
const isRegex = (url) => /[*+?()[\]{}]/.test(url || '');

function parseJSON(text) {
  try { return {value: JSON.parse(text)}; } catch (e) { return {error: e.message}; }
}
const pretty = (v) => JSON.stringify(v, null, 2);

function methodBadge(kind, method) {
  if (kind === 'grpc') return h('span', {class: 'badge m-GRPC'}, 'gRPC');
  const m = (method || '?').toUpperCase();
  return h('span', {class: `badge m-${HTTP_METHODS.includes(m) ? m : 'OTHER'}`}, m);
}

function uniqueFileName(mock, base) {
  const taken = new Set((mock?.stubs || []).map(s => s.file));
  const stem = base.replace(/\.json$/, '').replace(/[^A-Za-z0-9._-]+/g, '_').replace(/^[^A-Za-z0-9]+/, '') || 'stub';
  let name = `${stem}.json`;
  for (let i = 2; taken.has(name); i++) name = `${stem}_${i}.json`;
  return name;
}

function suggestFileName(stub, mock) {
  const req = stub.request || {};
  let base = (req.url || 'stub').split('/').filter(Boolean).pop() || 'root';
  if (req.method && mock.kind === 'http') base = `${req.method.toLowerCase()}_${base}`;
  return uniqueFileName(mock, base);
}

// Set a nested key; undefined or '' deletes it (other values like {}, 0 or false are kept).
function setPath(obj, path, value) {
  const keys = path.split('.');
  let o = obj;
  for (const k of keys.slice(0, -1)) {
    if (typeof o[k] !== 'object' || o[k] === null || Array.isArray(o[k])) o[k] = {};
    o = o[k];
  }
  const last = keys[keys.length - 1];
  if (value === undefined || value === '') delete o[last]; else o[last] = value;
}
const getPath = (obj, path) => path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj);

function confirmDiscard() {
  return !state.editor?.dirty || confirm('You have unsaved changes. Discard them?');
}

// ---------- header & sidebar ----------

function renderHeader() {
  const d = state.data;
  $('#mocks-dir').textContent = `mocks: ${d.mocks_dir}`;
  $('#readonly-badge').classList.toggle('hidden', d.writable);
  const btn = $('#problems-btn');
  btn.classList.toggle('hidden', d.problems.length === 0);
  btn.textContent = `⚠ ${d.problems.length} problem${d.problems.length === 1 ? '' : 's'}`;
}

function stubMatches(mock, stub, q) {
  return [stub.file, stub.url, stub.method, mock.name].some(v => (v || '').toLowerCase().includes(q));
}

function renderTree() {
  const tree = $('#tree');
  const q = state.filter.trim().toLowerCase();
  const groups = [['http', 'HTTP'], ['grpc', 'gRPC']].map(([kind, title]) => {
    const mocks = state.data.mocks.filter(m => m.kind === kind);
    const items = mocks.map(mock => {
      const stubs = q ? mock.stubs.filter(s => stubMatches(mock, s, q)) : mock.stubs;
      if (q && stubs.length === 0 && !mock.name.toLowerCase().includes(q)) return null;
      const key = mockKey(kind, mock.name);
      const collapsed = state.collapsed.has(key) && !q;
      const hasError = mock.proto_error || mock.stubs.some(s => s.error);
      const sel = state.sel;
      const row = h('div', {
        class: 'tree-mock' + (sel?.type === 'mock' && sel.kind === kind && sel.mock === mock.name ? ' selected' : ''),
        onclick: () => navigate({type: 'mock', kind, mock: mock.name}),
        title: `${kind}/${mock.name}`,
      },
        h('span', {class: 'caret', onclick: (e) => {
          e.stopPropagation();
          state.collapsed.has(key) ? state.collapsed.delete(key) : state.collapsed.add(key);
          renderTree();
        }}, collapsed ? '▶' : '▼'),
        h('span', {class: 'name'}, mock.name),
        hasError && h('span', {class: 'dot-error', title: 'Has problems'}),
        h('span', {class: 'count'}, String(mock.stubs.length)),
      );
      const stubRows = collapsed ? [] : stubs.map(s => h('div', {
        class: 'tree-stub' + (sel?.type === 'stub' && sel.kind === kind && sel.mock === mock.name && sel.file === s.file ? ' selected' : ''),
        onclick: () => navigate({type: 'stub', kind, mock: mock.name, file: s.file}),
        title: s.error ? `Error: ${s.error}` : s.file,
      },
        methodBadge(kind, s.method),
        h('span', {class: 'url'}, s.url || '(no url)'),
        s.priority ? h('span', {class: 'badge badge-outline', title: 'priority'}, `p${s.priority}`) : null,
        s.error && h('span', {class: 'dot-error'}),
      ));
      return [row, stubRows];
    }).filter(Boolean);
    return h('div', {class: 'tree-group'},
      h('div', {class: 'tree-group-title'}, `${title} · ${mocks.length}`),
      items.length ? items : h('div', {class: 'tree-empty small'}, q ? 'No matches' : `No ${title} mocks yet`));
  });
  mount(tree, groups);
}

// ---------- routing (URL hash: #/kind/mock[/~file]) ----------

function selToHash(sel) {
  if (!sel) return '';
  const base = `#/${sel.kind}/${sel.mock.split('/').map(encodeURIComponent).join('/')}`;
  return sel.type === 'stub' ? `${base}/~${encodeURIComponent(sel.file || '')}${sel.isNew ? '?new' : ''}` : base;
}

function hashToSel() {
  const m = location.hash.match(/^#\/(http|grpc)\/(.+?)(?:\/~([^?]*))?(\?new)?$/);
  if (!m) return null;
  const mock = m[2].split('/').map(decodeURIComponent).join('/');
  return m[3] !== undefined
    ? {type: 'stub', kind: m[1], mock, file: decodeURIComponent(m[3]), isNew: Boolean(m[4])}
    : {type: 'mock', kind: m[1], mock};
}

function navigate(sel, {force} = {}) {
  if (!force && !confirmDiscard()) return;
  state.sel = sel;
  const hash = selToHash(sel);
  if (location.hash !== hash) history.replaceState(null, '', hash || location.pathname);
  renderTree();
  renderMain();
}

function renderMain() {
  const sel = state.sel;
  if (!sel) return renderWelcome();
  const mock = findMock(sel.kind, sel.mock);
  if (!mock) { state.sel = null; return renderWelcome(); }
  if (sel.type === 'mock') return renderMockPage(mock);
  if (sel.isNew && state.editor?.isNew && state.editor.mock === mock.name) return renderStubEditor();
  const stub = mock.stubs.find(s => s.file === sel.file);
  if (!stub) { state.sel = {type: 'mock', kind: sel.kind, mock: sel.mock}; return renderMockPage(mock); }
  openStub(mock, stub);
}

// ---------- welcome / guide ----------

function renderWelcome() {
  state.editor = null;
  const d = state.data;
  const total = d.mocks.reduce((n, m) => n + m.stubs.length, 0);
  mount($('#main'), h('div', {class: 'empty-state'},
    h('h1', {class: 'page-title'}, 'Welcome to ProtoMock'),
    h('p', {class: 'muted'}, `${d.mocks.length} mock folders · ${total} stubs loaded from `, h('code', {}, d.mocks_dir)),
    h('div', {class: 'steps'},
      h('div', {class: 'step'}, h('b', {}, '1. Create a mock'), 'A mock is a folder: HTTP or gRPC, with optional .proto files.'),
      h('div', {class: 'step'}, h('b', {}, '2. Add stubs'), 'Each stub matches a request (method, URL, headers, body) and returns a response.'),
      h('div', {class: 'step'}, h('b', {}, '3. Try it'), 'Send a test request from the editor. Changes are live immediately — no restart.'),
    ),
    h('div', {class: 'toolbar'},
      h('button', {class: 'btn btn-primary', onclick: openNewMockModal, disabled: !d.writable}, '+ New mock'),
      h('button', {class: 'btn', onclick: openGuide}, 'Read the guide'),
    ),
    !d.writable && h('div', {class: 'status-line status-err'}, 'The mocks folder is read-only, so editing is disabled. In Docker, mount it without ":ro".'),
  ));
}

function openGuide() {
  const code = (t) => h('code', {}, t);
  openModal('ProtoMock guide', h('div', {class: 'guide'},
    h('h3', {}, 'Folder layout'),
    h('pre', {}, 'mocks/\n├── http/<mock>/\n│   ├── *.proto        (optional: protobuf responses)\n│   └── stubs/*.json\n└── grpc/<mock>/\n    ├── *.proto        (services + messages)\n    └── stubs/*.json'),
    h('h3', {}, 'Matching'),
    h('ul', {},
      h('li', {}, h('b', {}, 'URL: '), 'exact path, or a regex (anchored, e.g. ', code('/users/[0-9]+'), '). gRPC: ', code('/package.Service/Method'), '.'),
      h('li', {}, h('b', {}, 'Headers: '), 'every listed header must be present with that value (gRPC: request metadata).'),
      h('li', {}, h('b', {}, 'Body: '), 'partial JSON match — listed fields must be in the request (nested too); extra fields are ignored; arrays must be equal.'),
      h('li', {}, h('b', {}, 'Order: '), 'higher ', code('priority'), ' first, then file name. HTTP exact URLs are tried before regex URLs.'),
    ),
    h('h3', {}, 'Responses'),
    h('ul', {},
      h('li', {}, 'HTTP: JSON by default; tick ', h('i', {}, 'Protobuf'), ' and pick a message type to return protobuf (needs a .proto in the mock).'),
      h('li', {}, 'gRPC: always protobuf; the message type defaults to the method\'s output type.'),
      h('li', {}, code('headers'), ' are sent as HTTP headers / gRPC response metadata; ', code('delay_ms'), ' waits before replying.'),
    ),
    h('h3', {}, 'Tips'),
    h('ul', {},
      h('li', {}, 'Use ', h('b', {}, 'Insert template'), ' to fill a body with every field of a protobuf message.'),
      h('li', {}, 'Files edited on disk are picked up automatically (hot reload).'),
      h('li', {}, 'gRPC reflection is on: ', code('grpcurl -plaintext localhost:8086 list'), '.'),
      h('li', {}, h('kbd', {}, '⌘/Ctrl + S'), ' saves the stub you are editing.'),
    ),
  ), [h('button', {class: 'btn btn-primary', onclick: closeModal}, 'Got it')]);
}

// ---------- mock page ----------

function renderMockPage(mock) {
  state.editor = null;
  const writable = state.data.writable;
  const addStub = (prefill) => newStub(mock, prefill);

  const protoCards = mock.proto_files.map(pf => protoCard(mock, pf));
  mount($('#main'),
    h('h1', {class: 'page-title'}, mock.kind === 'grpc' ? h('span', {class: 'badge m-GRPC'}, 'gRPC') : h('span', {class: 'badge m-GET'}, 'HTTP'), mock.name),
    h('div', {class: 'page-sub'}, `mocks/${mock.kind}/${mock.name}`),
    h('div', {class: 'toolbar'},
      h('button', {class: 'btn btn-primary', disabled: !writable, onclick: () => addStub()}, '+ New stub'),
      h('button', {class: 'btn', disabled: !writable, onclick: () => openProtoModal(mock)}, '+ Add .proto file'),
    ),
    mock.proto_error && h('div', {class: 'status-line status-err'}, 'Proto error: ' + mock.proto_error),

    h('div', {class: 'card'},
      h('div', {class: 'card-head'}, `Stubs (${mock.stubs.length})`),
      h('div', {class: 'card-body'}, mock.stubs.length
        ? h('table', {class: 'list'},
            h('tr', {}, h('th', {}, ''), h('th', {}, 'URL'), h('th', {}, 'File'), h('th', {}, 'Priority'), h('th', {}, 'Status')),
            mock.stubs.map(s => h('tr', {class: 'clickable', onclick: () => navigate({type: 'stub', kind: mock.kind, mock: mock.name, file: s.file})},
              h('td', {}, methodBadge(mock.kind, s.method)),
              h('td', {class: 'mono'}, s.url || ''),
              h('td', {class: 'mono'}, s.file),
              h('td', {}, s.priority ? String(s.priority) : ''),
              h('td', {}, s.error ? h('span', {class: 'error-text'}, s.error) : h('span', {class: 'badge badge-ok'}, 'loaded')),
            )))
        : h('div', {class: 'muted'}, 'No stubs yet.')),
    ),

    mock.kind === 'grpc' && h('div', {class: 'card'},
      h('div', {class: 'card-head'}, `Methods (${mock.methods.length})`),
      h('div', {class: 'card-body'}, mock.methods.length
        ? h('table', {class: 'list'},
            h('tr', {}, h('th', {}, 'Method'), h('th', {}, 'Request → Response'), h('th', {}, 'Stubs'), h('th', {}, '')),
            mock.methods.map(m => {
              const n = mock.stubs.filter(s => s.url === m.path).length;
              const streaming = m.client_streaming || m.server_streaming;
              return h('tr', {},
                h('td', {class: 'mono'}, m.path),
                h('td', {class: 'mono small'}, `${m.input} → ${m.output}`),
                h('td', {}, n ? String(n) : h('span', {class: 'muted'}, '0')),
                h('td', {}, streaming
                  ? h('span', {class: 'muted small'}, 'streaming (not supported)')
                  : h('button', {class: 'btn btn-small', disabled: !writable, onclick: () => addStub({request: {url: m.path}, response: {}})}, '+ Stub')),
              );
            }))
        : h('div', {class: 'muted'}, 'No services defined. Add a .proto with a service to mock its methods.')),
    ),

    mock.kind === 'http' && mock.messages.length > 0 && h('div', {class: 'card'},
      h('div', {class: 'card-head'}, `Message types (${mock.messages.length})`),
      h('div', {class: 'card-body mono small'}, mock.messages.join('  ·  ')),
    ),

    protoCards,
  );
}

function protoCard(mock, pf) {
  const writable = state.data.writable;
  const area = h('textarea', {class: 'code', rows: Math.min(30, pf.content.split('\n').length + 2), spellcheck: 'false', disabled: !writable}, pf.content);
  const status = h('div');
  const save = async () => {
    try {
      const data = await api('PUT', '/protos', {query: {kind: mock.kind, mock: mock.name, file: pf.name}, body: area.value, raw: true});
      toast(`Saved ${pf.name}`);
      await load(data);
      renderMain();
    } catch (e) { mount(status, h('div', {class: 'status-line status-err'}, e.message)); }
  };
  const del = async () => {
    if (!confirm(`Delete ${pf.name}? Stubs that use its types will stop loading.`)) return;
    try { await load(await api('DELETE', '/protos', {query: {kind: mock.kind, mock: mock.name, file: pf.name}})); renderMain(); toast(`Deleted ${pf.name}`); }
    catch (e) { toast(e.message, true); }
  };
  return h('div', {class: 'card'},
    h('div', {class: 'card-head'}, h('span', {class: 'mono'}, pf.name), h('span', {class: 'spacer'}),
      h('button', {class: 'btn btn-small btn-danger', disabled: !writable, onclick: del}, 'Delete'),
      h('button', {class: 'btn btn-small btn-primary', disabled: !writable, onclick: save}, 'Save .proto')),
    h('div', {class: 'card-body'}, area, status),
  );
}

// ---------- stub editor ----------

function newStub(mock, prefill) {
  if (!confirmDiscard()) return;
  const stub = prefill || (mock.kind === 'grpc'
    ? {request: {url: mock.methods[0]?.path || ''}, response: {}}
    : {request: {method: 'GET', url: '/'}, response: {status: 200, body: {}}});
  const file = suggestFileName(stub, mock);
  state.editor = makeEditor(mock, file, pretty(stub), true);
  state.sel = {type: 'stub', kind: mock.kind, mock: mock.name, file, isNew: true};
  history.replaceState(null, '', selToHash(state.sel));
  renderTree();
  renderStubEditor();
}

function openStub(mock, stub) {
  const same = state.editor && !state.editor.isNew && state.editor.kind === mock.kind &&
    state.editor.mock === mock.name && state.editor.originalFile === stub.file;
  if (!same) state.editor = makeEditor(mock, stub.file, stub.content, false);
  renderStubEditor();
}

function makeEditor(mock, file, content, isNew) {
  const parsed = parseJSON(content);
  return {
    kind: mock.kind, mock: mock.name, isNew,
    file, originalFile: isNew ? null : file,
    text: content,                         // the stub JSON text (source of truth)
    obj: parsed.value ?? null,             // parsed stub (null if text is invalid JSON)
    view: parsed.error ? 'json' : 'form',
    dirty: isNew,
    validation: null,                      // {valid, error}
    try: null,                             // try-it panel state
  };
}

// Update the editor from the form (obj changed) or the JSON tab (text changed).
function editorChanged({fromText} = {}) {
  const ed = state.editor;
  if (fromText) {
    const p = parseJSON(ed.text);
    ed.obj = p.value ?? null;
    ed.parseError = p.error || null;
  } else {
    ed.text = pretty(ed.obj);
    ed.parseError = null;
  }
  ed.dirty = true;
  updateEditorStatus();
  validateSoon();
}

const validateSoon = debounce(async () => {
  const ed = state.editor;
  if (!ed || ed.parseError) return;
  try {
    ed.validation = await api('POST', '/validate', {body: {kind: ed.kind, mock: ed.mock, content: ed.text}});
  } catch (e) { ed.validation = {valid: false, error: e.message}; }
  updateEditorStatus();
}, 350);

function updateEditorStatus() {
  const ed = state.editor;
  const el = $('#editor-status');
  if (!ed || !el) return;
  let line;
  if (ed.parseError) line = h('div', {class: 'status-line status-err'}, 'Invalid JSON: ' + ed.parseError);
  else if (!ed.validation) line = h('div', {class: 'status-line status-info'}, 'Checking…');
  else if (ed.validation.valid) line = h('div', {class: 'status-line status-ok'}, '✓ Valid stub' + (ed.dirty ? ' — unsaved changes' : ''));
  else line = h('div', {class: 'status-line status-err'}, '✗ ' + ed.validation.error);
  mount(el, line);
  const saveBtn = $('#save-btn');
  if (saveBtn) saveBtn.disabled = !state.data.writable || Boolean(ed.parseError);
  // The form needs a parsed stub object.
  const formTab = $('#form-tab');
  if (formTab) {
    const canForm = !ed.parseError && ed.obj !== null && typeof ed.obj === 'object' && !Array.isArray(ed.obj);
    formTab.disabled = !canForm;
    formTab.title = canForm ? '' : 'Fix the JSON first';
  }
}

function renderStubEditor() {
  const ed = state.editor;
  const mock = findMock(ed.kind, ed.mock);
  const writable = state.data.writable;

  const fileInput = h('input', {type: 'text', class: 'mono', value: ed.file, disabled: !writable,
    oninput: (e) => { ed.file = e.target.value.trim(); ed.dirty = true; updateEditorStatus(); }});

  const tabs = h('div', {class: 'tabs'},
    h('button', {id: 'form-tab', class: ed.view === 'form' ? 'active' : '', onclick: () => { ed.view = 'form'; renderStubEditor(); }}, 'Form'),
    h('button', {class: ed.view === 'json' ? 'active' : '', onclick: () => { ed.view = 'json'; renderStubEditor(); }}, 'JSON'),
  );

  mount($('#main'),
    h('h1', {class: 'page-title'}, methodBadge(ed.kind, ed.obj?.request?.method), ed.isNew ? 'New stub' : ed.originalFile),
    h('div', {class: 'page-sub'}, `mocks/${ed.kind}/${ed.mock}/stubs/`),
    h('div', {class: 'toolbar'},
      h('label', {class: 'field', style: 'flex: 0 1 320px'}, h('span', {}, 'File name'), fileInput),
      h('div', {class: 'spacer'}),
      tabs,
      h('button', {class: 'btn', onclick: () => openTry(mock)}, '▶ Try it'),
      !ed.isNew && h('button', {class: 'btn', disabled: !writable, onclick: () => duplicateStub(mock)}, 'Duplicate'),
      !ed.isNew && h('button', {class: 'btn btn-danger', disabled: !writable, onclick: () => deleteStub(mock)}, 'Delete'),
      h('button', {class: 'btn btn-primary', id: 'save-btn', disabled: !writable, onclick: saveStub, title: '⌘/Ctrl+S'}, 'Save'),
    ),
    h('div', {id: 'editor-status', style: 'margin-bottom: 14px'}),
    ed.view === 'json' ? jsonEditor(ed) : formEditor(ed, mock),
    h('div', {id: 'try-panel'}),
  );
  updateEditorStatus();
  if (!ed.validation) validateSoon();
  if (ed.try) renderTry(mock);
}

function jsonEditor(ed) {
  return h('div', {class: 'card'},
    h('div', {class: 'card-head'}, 'Stub JSON', h('span', {class: 'spacer'}),
      h('button', {class: 'btn btn-small', onclick: () => {
        const p = parseJSON(ed.text);
        if (p.error) return toast('Invalid JSON: ' + p.error, true);
        ed.text = pretty(p.value); renderStubEditor();
      }}, 'Format')),
    h('div', {class: 'card-body'},
      h('textarea', {class: 'code', rows: 28, spellcheck: 'false', value: ed.text,
        oninput: (e) => { ed.text = e.target.value; editorChanged({fromText: true}); },
        onkeydown: tabIndent}),
    ),
  );
}

function tabIndent(e) {
  if (e.key !== 'Tab') return;
  e.preventDefault();
  const t = e.target, s = t.selectionStart;
  t.setRangeText('  ', s, t.selectionEnd, 'end');
  t.dispatchEvent(new Event('input'));
}

// A JSON body field bound to obj[path]; empty text removes the key.
function jsonField(ed, path, {label, hint, template}) {
  const current = getPath(ed.obj, path);
  const area = h('textarea', {class: 'code', rows: 8, spellcheck: 'false', value: current === undefined ? '' : pretty(current), onkeydown: tabIndent});
  const err = h('div', {class: 'error-text'});
  area.addEventListener('input', () => {
    const text = area.value.trim();
    if (text === '') { area.classList.remove('invalid'); err.textContent = ''; setPath(ed.obj, path, undefined); return editorChanged(); }
    const p = parseJSON(text);
    area.classList.toggle('invalid', Boolean(p.error));
    err.textContent = p.error ? 'Invalid JSON: ' + p.error : '';
    if (!p.error) { setPath(ed.obj, path, p.value); editorChanged(); }
  });
  const insertTemplate = template && h('button', {class: 'btn btn-small', onclick: async () => {
    const msg = template();
    if (!msg) return toast('Pick a message type first', true);
    try {
      const tmpl = await api('GET', '/template', {query: {kind: ed.kind, mock: ed.mock, message: msg}});
      if (area.value.trim() && !confirm('Replace the current body with a template?')) return;
      area.value = pretty(tmpl); area.dispatchEvent(new Event('input'));
    } catch (e) { toast(e.message, true); }
  }}, 'Insert template');
  return h('label', {class: 'field'},
    h('span', {}, label),
    h('div', {class: 'row', style: 'align-items:center'}, h('div', {class: 'hint'}, hint || ''), h('div', {class: 'shrink'}, insertTemplate)),
    area, err);
}

function kvEditor(ed, path, {label, hint, keyPlaceholder}) {
  const obj = getPath(ed.obj, path) || {};
  const rows = Object.entries(obj).map(([k, v]) => ({k, v: String(v)}));
  const box = h('div', {class: 'kv'});
  const commit = () => {
    const out = {};
    for (const r of rows) if (r.k.trim()) out[r.k.trim()] = r.v;
    setPath(ed.obj, path, Object.keys(out).length ? out : undefined);
    editorChanged();
  };
  const draw = () => mount(box,
    rows.map((r, i) => h('div', {class: 'kv-row'},
      h('input', {type: 'text', placeholder: keyPlaceholder || 'Name', value: r.k, oninput: (e) => { r.k = e.target.value; commit(); }}),
      h('input', {type: 'text', placeholder: 'Value', value: r.v, oninput: (e) => { r.v = e.target.value; commit(); }}),
      h('button', {class: 'btn-icon', title: 'Remove', onclick: () => { rows.splice(i, 1); commit(); draw(); }}, '✕'),
    )),
    h('div', {}, h('button', {class: 'btn btn-small', onclick: () => { rows.push({k: '', v: ''}); draw(); box.querySelector('.kv-row:last-of-type input')?.focus(); }}, '+ Add')),
  );
  draw();
  return h('div', {class: 'field'}, h('label', {class: 'field'}, h('span', {}, label)), hint && h('div', {class: 'hint'}, hint), box);
}

function formEditor(ed, mock) {
  const o = ed.obj;
  o.request = o.request && typeof o.request === 'object' ? o.request : {};
  o.response = o.response && typeof o.response === 'object' ? o.response : {};
  const isGRPC = ed.kind === 'grpc';

  // rerender: redraw the form after the change (for selects/checkboxes that change which
  // fields are shown); never for text inputs, which would lose focus on every keystroke.
  const bind = (path, transform, rerender) => (e) => {
    const v = e.target.type === 'checkbox' ? (e.target.checked || undefined) : e.target.value;
    setPath(o, path, transform ? transform(v) : v);
    editorChanged();
    if (rerender) renderStubEditor();
  };
  const num = (v) => (v === '' ? undefined : Number(v));

  // --- request ---
  let urlField;
  if (isGRPC) {
    const known = mock.methods.some(m => m.path === o.request.url);
    urlField = h('label', {class: 'field'}, h('span', {}, 'Method'),
      h('select', {onchange: bind('request.url', null, true)},
        !known && h('option', {value: o.request.url || '', selected: true}, o.request.url || '— choose a method —'),
        mock.methods.map(m => h('option', {value: m.path, selected: m.path === o.request.url}, m.path))),
      h('div', {class: 'hint'}, mock.methods.length ? 'Methods come from the mock\'s .proto services.' : 'Add a .proto with a service to this mock first.'));
  } else {
    urlField = h('label', {class: 'field'}, h('span', {}, 'URL'),
      h('input', {type: 'text', class: 'mono', value: o.request.url || '', placeholder: '/api/users or /users/[0-9]+', oninput: bind('request.url')}),
      h('div', {class: 'hint'}, isRegex(o.request.url) ? 'Regex: the whole path must match.' : 'Exact path. Use regex characters (e.g. [0-9]+) for patterns.'));
  }
  const requestCard = h('div', {class: 'card'},
    h('div', {class: 'card-head'}, 'Request — when should this stub answer?'),
    h('div', {class: 'card-body'},
      h('div', {class: 'row'},
        !isGRPC && h('label', {class: 'field shrink', style: 'width: 130px'}, h('span', {}, 'Method'),
          h('select', {onchange: bind('request.method', null, true)},
            HTTP_METHODS.map(m => h('option', {value: m, selected: (o.request.method || '').toUpperCase() === m}, m)))),
        urlField,
        h('label', {class: 'field shrink', style: 'width: 110px'}, h('span', {}, 'Priority'),
          h('input', {type: 'number', value: o.priority ?? '', placeholder: '0', oninput: bind('priority', num)})),
      ),
      kvEditor(ed, 'request.headers', {label: isGRPC ? 'Metadata must include' : 'Headers must include', keyPlaceholder: isGRPC ? 'x-tenant' : 'X-Tenant'}),
      jsonField(ed, 'request.body', {
        label: 'Body must contain',
        hint: 'Partial match: listed fields must be in the request. Leave empty to match any body.',
        template: isGRPC ? () => mock.methods.find(m => m.path === o.request.url)?.input : null,
      }),
    ));

  // --- response ---
  const method = isGRPC ? mock.methods.find(m => m.path === o.request.url) : null;
  const msgOf = () => isGRPC ? (o.response.message || method?.output) : o.response.message;
  const responseCard = h('div', {class: 'card'},
    h('div', {class: 'card-head'}, 'Response — what should it return?'),
    h('div', {class: 'card-body'},
      h('div', {class: 'row'},
        !isGRPC && h('label', {class: 'field shrink', style: 'width: 120px'}, h('span', {}, 'Status'),
          h('input', {type: 'number', value: o.response.status ?? '', placeholder: '200', oninput: bind('response.status', num)})),
        h('label', {class: 'field shrink', style: 'width: 140px'}, h('span', {}, 'Delay (ms)'),
          h('input', {type: 'number', min: 0, value: o.response.delay_ms ?? '', placeholder: '0', oninput: bind('response.delay_ms', num)})),
        !isGRPC && h('div', {class: 'field shrink'}, h('label', {class: 'field'}, h('span', {}, 'Format')),
          h('label', {class: 'checkbox', title: mock.messages.length ? '' : 'Add a .proto to this mock to return protobuf'},
            h('input', {type: 'checkbox', checked: Boolean(o.response.proto), disabled: !mock.messages.length && !o.response.proto, onchange: bind('response.proto', null, true)}),
            'Protobuf')),
        (isGRPC || o.response.proto) && h('label', {class: 'field'}, h('span', {}, 'Message type'),
          isGRPC
            ? h('input', {type: 'text', class: 'mono', disabled: true, value: method ? method.output : (o.response.message || '')})
            : h('select', {onchange: bind('response.message')},
                h('option', {value: ''}, '— choose —'),
                mock.messages.map(m => h('option', {value: m, selected: m === o.response.message}, m)))),
      ),
      kvEditor(ed, 'response.headers', {label: isGRPC ? 'Response metadata' : 'Response headers', keyPlaceholder: isGRPC ? 'x-trace-id' : 'Content-Type'}),
      jsonField(ed, 'response.body', {
        label: 'Body',
        hint: isGRPC || o.response.proto ? 'JSON form of the protobuf message (.proto field names).' : 'Any JSON value.',
        template: isGRPC || o.response.proto ? msgOf : null,
      }),
    ));

  return [requestCard, responseCard];
}

async function saveStub() {
  const ed = state.editor;
  if (!ed || ed.parseError) return;
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(ed.file)) {
    return toast('File name must be a plain name ending in .json (letters, digits, . _ -)', true);
  }
  const mock = findMock(ed.kind, ed.mock);
  const renamed = !ed.isNew && ed.file !== ed.originalFile;
  if ((ed.isNew || renamed) && mock.stubs.some(s => s.file === ed.file) &&
      !confirm(`${ed.file} already exists. Overwrite it?`)) return;
  try {
    let data = await api('PUT', '/stubs', {query: {kind: ed.kind, mock: ed.mock, file: ed.file}, body: ed.text, raw: true});
    if (renamed) data = await api('DELETE', '/stubs', {query: {kind: ed.kind, mock: ed.mock, file: ed.originalFile}});
    await load(data);
    ed.isNew = false; ed.originalFile = ed.file; ed.dirty = false;
    navigate({type: 'stub', kind: ed.kind, mock: ed.mock, file: ed.file}, {force: true});
    toast(`Saved ${ed.file} — live now`);
  } catch (e) { toast(e.message, true); }
}

function duplicateStub(mock) {
  const ed = state.editor;
  if (ed.parseError) return toast('Fix the JSON first', true);
  const copy = JSON.parse(ed.text);
  state.editor = null;
  newStub(mock, copy);
  state.editor.file = uniqueFileName(mock, ed.file.replace(/\.json$/, '') + '_copy');
  state.sel.file = state.editor.file;
  history.replaceState(null, '', selToHash(state.sel));
  renderStubEditor();
}

async function deleteStub(mock) {
  const ed = state.editor;
  if (!confirm(`Delete ${ed.originalFile}? This removes the file from disk.`)) return;
  try {
    await load(await api('DELETE', '/stubs', {query: {kind: ed.kind, mock: ed.mock, file: ed.originalFile}}));
    state.editor = null;
    navigate({type: 'mock', kind: mock.kind, mock: mock.name}, {force: true});
    toast(`Deleted ${ed.originalFile}`);
  } catch (e) { toast(e.message, true); }
}

// ---------- try it ----------

function openTry(mock) {
  const ed = state.editor;
  const o = ed.obj || {};
  const req = o.request || {};
  if (ed.kind === 'grpc') {
    ed.try = {method: req.url || '', headers: {...(req.headers || {})}, body: req.body !== undefined ? pretty(req.body) : '{}', result: null};
  } else {
    let path = req.url || '/';
    if (isRegex(path)) path = path.replace(/\[0-9\][+*]|\\d[+*]/g, '1').replace(/\.[+*]/g, 'x');
    ed.try = {method: (req.method || 'GET').toUpperCase(), path, headers: {...(req.headers || {})},
      body: req.body !== undefined ? pretty(req.body) : '', result: null};
  }
  renderTry(mock);
  $('#try-panel')?.scrollIntoView({behavior: 'smooth', block: 'start'});
}

function renderTry(mock) {
  const ed = state.editor, t = ed.try, panel = $('#try-panel');
  if (!panel || !t) return;
  const isGRPC = ed.kind === 'grpc';
  const headerRows = Object.entries(t.headers).map(([k, v]) => ({k, v}));
  const commitHeaders = () => { t.headers = {}; for (const r of headerRows) if (r.k.trim()) t.headers[r.k.trim()] = r.v; };
  const headersBox = h('div', {class: 'kv'});
  const drawHeaders = () => mount(headersBox,
    headerRows.map((r, i) => h('div', {class: 'kv-row'},
      h('input', {type: 'text', placeholder: 'Name', value: r.k, oninput: (e) => { r.k = e.target.value; commitHeaders(); }}),
      h('input', {type: 'text', placeholder: 'Value', value: r.v, oninput: (e) => { r.v = e.target.value; commitHeaders(); }}),
      h('button', {class: 'btn-icon', onclick: () => { headerRows.splice(i, 1); commitHeaders(); drawHeaders(); }}, '✕'))),
    h('div', {}, h('button', {class: 'btn btn-small', onclick: () => { headerRows.push({k: '', v: ''}); drawHeaders(); }}, '+ Add')));
  drawHeaders();

  const result = h('div');
  const send = async () => {
    mount(result, h('div', {class: 'status-line status-info'}, 'Sending…'));
    try {
      let res;
      if (isGRPC) {
        const p = parseJSON(t.body || '{}');
        if (p.error) throw new Error('Request body: invalid JSON: ' + p.error);
        res = await api('POST', '/try/grpc', {body: {method: t.method, metadata: t.headers, body: p.value}});
      } else {
        const o = ed.obj || {};
        res = await api('POST', '/try/http', {body: {method: t.method, path: t.path, headers: t.headers, body: t.body,
          kind: ed.kind, mock: ed.mock, message: o.response?.proto ? o.response.message : ''}});
      }
      t.result = res;
      drawResult();
    } catch (e) { mount(result, h('div', {class: 'status-line status-err'}, e.message)); }
  };
  const drawResult = () => {
    const r = t.result;
    if (!r) return mount(result);
    const ok = isGRPC ? r.code === 'OK' : r.status < 400;
    let body;
    if (isGRPC) body = r.body !== undefined ? pretty(r.body) : r.message;
    else if (r.body_json !== undefined) body = pretty(r.body_json);
    else if (r.body !== undefined) { const p = parseJSON(r.body); body = p.error ? r.body : pretty(p.value); }
    else body = `(${r.body_size} bytes of binary data${r.decode_error ? ': ' + r.decode_error : ''})`;
    const hdrs = Object.entries(r.headers || {});
    mount(result,
      h('div', {class: 'row', style: 'align-items:center; margin-bottom: 8px'},
        h('div', {}, h('span', {class: 'badge ' + (ok ? 'badge-ok' : 'badge-err')}, isGRPC ? r.code : String(r.status)),
          ' ', h('span', {class: 'muted small'}, `${r.duration_ms} ms`),
          isGRPC && !ok && r.message ? h('span', {class: 'error-text'}, '  ' + r.message) : null,
          r.body_json !== undefined ? h('span', {class: 'muted small'}, '  · protobuf, decoded to JSON') : null)),
      hdrs.length > 0 && h('table', {class: 'list small', style: 'margin-bottom: 8px'},
        hdrs.map(([k, v]) => h('tr', {}, h('td', {class: 'mono'}, k), h('td', {class: 'mono'}, v)))),
      h('pre', {class: 'output'}, body || '(empty)'));
  };

  mount(panel, h('div', {class: 'card'},
    h('div', {class: 'card-head'}, '▶ Try it', h('span', {class: 'muted small'}, isGRPC
      ? ' — calls the running gRPC server'
      : ' — sends the request through the mock router'), h('span', {class: 'spacer'}),
      h('button', {class: 'btn btn-small', onclick: () => { ed.try = null; mount(panel); }}, 'Close')),
    h('div', {class: 'card-body'},
      isGRPC
        ? h('label', {class: 'field'}, h('span', {}, 'Method'),
            h('select', {onchange: (e) => { t.method = e.target.value; }},
              mock.methods.map(m => h('option', {value: m.path, selected: m.path === t.method}, m.path))))
        : h('div', {class: 'row'},
            h('label', {class: 'field shrink', style: 'width: 130px'}, h('span', {}, 'Method'),
              h('select', {onchange: (e) => { t.method = e.target.value; }},
                HTTP_METHODS.map(m => h('option', {value: m, selected: m === t.method}, m)))),
            h('label', {class: 'field'}, h('span', {}, 'Path'),
              h('input', {type: 'text', class: 'mono', value: t.path, oninput: (e) => { t.path = e.target.value; }}))),
      h('div', {class: 'field'}, h('label', {class: 'field'}, h('span', {}, isGRPC ? 'Metadata' : 'Headers')), headersBox),
      h('label', {class: 'field'}, h('span', {}, isGRPC ? 'Request (JSON)' : 'Body'),
        h('textarea', {class: 'code', rows: 6, spellcheck: 'false', value: t.body, onkeydown: tabIndent, oninput: (e) => { t.body = e.target.value; }})),
      h('div', {}, h('button', {class: 'btn btn-primary', onclick: send}, 'Send'),
        ed.dirty ? h('span', {class: 'hint'}, '  Unsaved changes are not live yet — save first to try them.') : null),
      result,
    )));
  drawResult();
}

// ---------- modals ----------

function openModal(title, body, footer) {
  const root = $('#modal-root');
  const backdrop = h('div', {class: 'modal-backdrop', onclick: (e) => { if (e.target === backdrop) closeModal(); }},
    h('div', {class: 'modal', role: 'dialog', 'aria-label': title},
      h('div', {class: 'modal-head'}, title, h('span', {class: 'spacer'}), h('button', {class: 'btn-icon', onclick: closeModal, title: 'Close'}, '✕')),
      h('div', {class: 'modal-body'}, body),
      footer && h('div', {class: 'modal-foot'}, footer)));
  mount(root, backdrop);
  backdrop.querySelector('input, textarea, select')?.focus();
}
function closeModal() { mount($('#modal-root')); }

const SAMPLE_PROTO = (pkg) => `syntax = "proto3";

package ${pkg};

message GetItemRequest {
  string id = 1;
}

message Item {
  string id = 1;
  string name = 2;
}

// For gRPC mocks: each rpc becomes a method you can stub.
service ItemService {
  rpc GetItem (GetItemRequest) returns (Item);
}
`;

function openNewMockModal() {
  let kind = 'http';
  const name = h('input', {type: 'text', class: 'mono', placeholder: 'e.g. users or team/users'});
  const withProto = h('input', {type: 'checkbox'});
  const protoName = h('input', {type: 'text', class: 'mono', placeholder: 'service.proto'});
  const proto = h('textarea', {class: 'code', rows: 14, spellcheck: 'false', onkeydown: tabIndent});
  const err = h('div', {class: 'error-text'});
  const protoBox = h('div', {class: 'field'});
  const pkgName = () => (name.value.trim().split('/').pop() || 'mock').replace(/[^A-Za-z0-9_]/g, '_').replace(/^[0-9]/, '_$&');
  let sample = ''; // the generated sample, so we only replace it while the user hasn't edited it
  const refreshSample = () => {
    if (proto.value === '' || proto.value === sample) { sample = SAMPLE_PROTO(pkgName()); proto.value = sample; }
  };
  name.addEventListener('input', refreshSample);
  const drawProto = () => {
    const show = kind === 'grpc' || withProto.checked;
    if (show) refreshSample();
    mount(protoBox,
      kind === 'http' && h('label', {class: 'checkbox'}, withProto, 'Add a .proto file (only needed for protobuf responses)'),
      show && h('div', {class: 'row'}, h('label', {class: 'field'}, h('span', {}, '.proto file name'), protoName)),
      show && h('label', {class: 'field'}, h('span', {}, kind === 'grpc' ? 'Service definition (.proto)' : '.proto'), proto));
  };
  withProto.addEventListener('change', drawProto);
  const kindSel = h('div', {class: 'tabs'},
    ...['http', 'grpc'].map(k => h('button', {class: k === kind ? 'active' : '', onclick: (e) => {
      kind = k; [...kindSel.children].forEach(b => b.classList.toggle('active', b === e.target)); drawProto();
    }}, k === 'http' ? 'HTTP' : 'gRPC')));
  drawProto();

  const create = async () => {
    const n = name.value.trim();
    const includeProto = kind === 'grpc' || withProto.checked;
    if (!n) { err.textContent = 'Enter a mock name.'; return; }
    try {
      const data = await api('POST', '/mocks', {body: {kind, name: n,
        proto_file: includeProto ? (protoName.value.trim() || `${n.split('/').pop()}.proto`) : '',
        proto: includeProto ? proto.value : ''}});
      closeModal();
      await load(data);
      navigate({type: 'mock', kind, mock: n}, {force: true});
      toast(`Created ${kind}/${n}`);
    } catch (e) { err.textContent = e.message; }
  };
  openModal('New mock', [
    h('div', {class: 'field'}, h('label', {class: 'field'}, h('span', {}, 'Type')), kindSel),
    h('label', {class: 'field'}, h('span', {}, 'Name'), name, h('div', {class: 'hint'}, 'Folder name under mocks/http or mocks/grpc. Letters, digits, . _ - and / for nesting.')),
    protoBox, err,
  ], [h('button', {class: 'btn', onclick: closeModal}, 'Cancel'), h('button', {class: 'btn btn-primary', onclick: create}, 'Create mock')]);
}

function openProtoModal(mock) {
  const name = h('input', {type: 'text', class: 'mono', placeholder: 'common.proto'});
  const proto = h('textarea', {class: 'code', rows: 16, spellcheck: 'false', onkeydown: tabIndent}, SAMPLE_PROTO(mock.name.split('/').pop().replace(/[^A-Za-z0-9_]/g, '_')));
  const err = h('div', {class: 'error-text'});
  const save = async () => {
    const file = name.value.trim();
    if (!file.endsWith('.proto')) { err.textContent = 'File name must end in .proto'; return; }
    try {
      const data = await api('PUT', '/protos', {query: {kind: mock.kind, mock: mock.name, file}, body: proto.value, raw: true});
      closeModal(); await load(data); renderMain(); toast(`Added ${file}`);
    } catch (e) { err.textContent = e.message; }
  };
  openModal(`Add .proto to ${mock.name}`, [
    h('label', {class: 'field'}, h('span', {}, 'File name'), name),
    h('label', {class: 'field'}, h('span', {}, 'Content'), proto,
      h('div', {class: 'hint'}, 'Imports resolve relative to the mock folder; google/protobuf/*.proto are built in.')),
    err,
  ], [h('button', {class: 'btn', onclick: closeModal}, 'Cancel'), h('button', {class: 'btn btn-primary', onclick: save}, 'Add')]);
}

function openProblems() {
  const problems = state.data.problems;
  openModal(`Problems (${problems.length})`, problems.length
    ? h('table', {class: 'list'}, h('tr', {}, h('th', {}, 'File'), h('th', {}, 'Error')),
        problems.map(p => h('tr', {}, h('td', {class: 'mono small'}, p.path), h('td', {class: 'error-text'}, p.error))))
    : h('div', {class: 'muted'}, 'No problems 🎉'),
  [h('button', {class: 'btn', onclick: closeModal}, 'Close')]);
}

// ---------- boot ----------

async function boot() {
  $('#search').addEventListener('input', (e) => { state.filter = e.target.value; renderTree(); });
  $('#new-mock-btn').addEventListener('click', openNewMockModal);
  $('#help-btn').addEventListener('click', openGuide);
  $('#problems-btn').addEventListener('click', openProblems);
  $('#reload-btn').addEventListener('click', async () => {
    try { await load(await api('POST', '/reload')); if (!state.editor?.dirty) renderMain(); toast('Reloaded from disk'); }
    catch (e) { toast(e.message, true); }
  });
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 's' && state.editor) { e.preventDefault(); saveStub(); }
    if (e.key === 'Escape') closeModal();
  });
  window.addEventListener('beforeunload', (e) => { if (state.editor?.dirty) e.preventDefault(); });
  window.addEventListener('hashchange', () => {
    const sel = hashToSel();
    if (selToHash(sel) !== selToHash(state.sel)) navigate(sel);
  });

  try { await load(); } catch (e) {
    mount($('#main'), h('div', {class: 'status-line status-err'}, 'Failed to load mocks: ' + e.message));
    return;
  }
  const sel = hashToSel();
  if (sel && !sel.isNew) { state.sel = sel; renderTree(); }
  renderMain();

  // Keep the list in sync with changes made on disk (hot reload), without disturbing edits.
  setInterval(async () => {
    if (document.hidden) return;
    try {
      const data = await api('GET', '/mocks');
      if (JSON.stringify(data) === JSON.stringify(state.data)) return;
      state.data = data; renderHeader(); renderTree();
      if (!state.editor?.dirty && state.sel?.type === 'mock') renderMain();
    } catch { /* server restarting */ }
  }, 3000);
}

boot();
