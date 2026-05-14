// 表单弹窗逻辑

import { API, pluginInfo } from './utils.js';
import { loadAccounts } from './accounts.js';
import { loadScheduler } from './scheduler.js';
import { tryLoadPluginWidget, unmountWidget } from './widget.js';

let editingId = null;
let selectedType = '';
let widgetMode = false;

// Widget 与原生表单共享的凭证状态
let widgetCredentials = {};

function setSelectedType(typeKey) {
  selectedType = typeKey || '';
  document.querySelectorAll('.type-option').forEach((card) => {
    card.classList.toggle('active', card.dataset.type === selectedType);
  });
}

export function renderTypeCards() {
  const el = document.getElementById('type-cards');
  if (!pluginInfo?.account_types) return;
  el.innerHTML = pluginInfo.account_types.map((t, index) =>
    `<div class="type-option" data-type="${t.key}">
      <div class="type-option-meta">
        <span class="type-option-index">${String(index + 1).padStart(2, '0')}</span>
        <span class="type-option-kicker">${t.key}</span>
      </div>
      <div class="type-option-title">${t.label}</div>
      <div class="type-option-desc">${t.description}</div>
    </div>`
  ).join('');
}

export async function selectType(typeKey) {
  setSelectedType(typeKey);
  renderCredFields(typeKey);
  widgetMode = false;
  widgetCredentials = {};

  const loaded = await tryLoadPluginWidget(typeKey, {
    credentials: widgetCredentials,
    onChange: (creds) => { widgetCredentials = creds; },
    mode: editingId ? 'edit' : 'create',
    onAccountTypeChange: (type) => {
      setSelectedType(type);
    },
    onSuggestedName: (name) => {
      const nameInput = document.getElementById('f-name');
      if (nameInput && !nameInput.value) {
        nameInput.value = name;
      }
    },
  });

  widgetMode = loaded;
}

function renderCredFields(typeKey) {
  const el = document.getElementById('cred-fields');
  const at = pluginInfo?.account_types?.find(t => t.key === typeKey);
  if (!at) { el.innerHTML = ''; return; }
  el.innerHTML = at.fields.map(f =>
    `<div class="dialog-section dialog-section--fallback-field">
      <label class="field-label">${f.label}${f.required ? ' *' : ''}</label>
      <input id="cred-${f.key}" class="field-input${f.type === 'password' ? '' : ' mono'}"
             type="${f.type === 'password' ? 'password' : 'text'}"
             placeholder="${f.placeholder || ''}">
    </div>`
  ).join('');
}

export function getSelectedType() {
  return selectedType;
}

/** 获取凭证：优先从 Widget 状态取，否则从 DOM 取 */
export function getCredentials() {
  if (widgetMode) {
    return widgetCredentials;
  }
  // 否则从默认输入框取
  const creds = {};
  document.querySelectorAll('#cred-fields input').forEach(input => {
    const key = input.id.replace('cred-', '');
    if (input.value) creds[key] = input.value;
  });
  return creds;
}

export async function showForm(account) {
  editingId = account ? account.id : null;
  widgetMode = false;
  widgetCredentials = { ...(account?.credentials || {}) };
  selectedType = account?.account_type || '';
  document.getElementById('form-overlay').style.display = 'flex';
  document.getElementById('form-title').textContent = account ? '编辑账号' : '添加账号';
  document.getElementById('f-name').value = account?.name || '';
  document.getElementById('f-proxy').value = account?.proxy_url || '';
  document.getElementById('f-weight').value = account?.weight || 1;
  setSelectedType(selectedType);

  if (!selectedType) {
    document.getElementById('cred-fields').innerHTML = '';
    const loaded = await tryLoadPluginWidget('', {
      credentials: widgetCredentials,
      onChange: (creds) => { widgetCredentials = creds; },
      mode: editingId ? 'edit' : 'create',
      onAccountTypeChange: (type) => setSelectedType(type),
      onSuggestedName: (name) => {
        const nameInput = document.getElementById('f-name');
        if (nameInput && !nameInput.value) nameInput.value = name;
      },
    });
    widgetMode = loaded;
    return;
  }

  renderCredFields(selectedType);
  Object.entries(account?.credentials || {}).forEach(([k, v]) => {
    const input = document.getElementById('cred-' + k);
    if (input) input.value = v;
  });

  const loaded = await tryLoadPluginWidget(selectedType, {
    credentials: widgetCredentials,
    onChange: (creds) => { widgetCredentials = creds; },
    mode: editingId ? 'edit' : 'create',
    onAccountTypeChange: (type) => setSelectedType(type),
    onSuggestedName: (name) => {
      const nameInput = document.getElementById('f-name');
      if (nameInput && !nameInput.value) nameInput.value = name;
    },
  });

  widgetMode = loaded;
}

export function hideForm() {
  document.getElementById('form-overlay').style.display = 'none';
  editingId = null;
  selectedType = '';
  widgetMode = false;
  widgetCredentials = {};
  document.getElementById('f-name').value = '';
  document.getElementById('f-proxy').value = '';
  document.getElementById('f-weight').value = '1';
  document.getElementById('cred-fields').innerHTML = '';
  document.getElementById('cred-fields').style.display = '';
  document.querySelectorAll('.type-option').forEach(c => c.classList.remove('active'));
  unmountWidget();
}

export async function editAccount(id) {
  const res = await fetch(API + '/api/accounts/' + id);
  const account = await res.json();
  showForm(account);
}

export async function saveAccount() {
  const data = {
    name: document.getElementById('f-name').value,
    account_type: getSelectedType(),
    credentials: getCredentials(),
    proxy_url: document.getElementById('f-proxy').value,
    weight: parseInt(document.getElementById('f-weight').value) || 1,
  };
  if (!data.account_type) { alert('请选择账号类型'); return; }
  const url = editingId ? API + '/api/accounts/' + editingId : API + '/api/accounts';
  const method = editingId ? 'PUT' : 'POST';
  await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
  hideForm();
  loadAccounts();
  loadScheduler();
}
