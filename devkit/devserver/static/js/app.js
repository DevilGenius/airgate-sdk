// 入口模块

import { API, setPluginInfo, applyTheme, initTheme, toggleTheme } from './utils.js';
import { loadScheduler, setPolicy, togglePinnedSelect, closePinnedSelect, choosePinnedSelect } from './scheduler.js';
import { loadAccounts, editWeight, testAccount, deleteAccount } from './accounts.js';
import { renderTypeCards, selectType, showForm, hideForm, saveAccount, editAccount } from './form.js';

function renderEndpoints(pluginInfo) {
  const el = document.getElementById('endpoints-list');
  if (!pluginInfo?.routes?.length) {
    el.innerHTML = '<div class="empty-state">暂无路由</div>';
    document.getElementById('route-count').textContent = '0';
    return;
  }
  document.getElementById('route-count').textContent = pluginInfo.routes.length + ' 条';
  el.innerHTML = pluginInfo.routes.map(r => {
    const m = (r.method || '').toLowerCase();
    return `<div class="route-item">
      <span class="route-tag ${m}">${r.method}</span>
      <span class="route-path">${r.path}</span>
      <span class="route-desc">${r.description || ''}</span>
    </div>`;
  }).join('');
}

async function init() {
  initTheme();
  try {
    const res = await fetch(API + '/api/plugin/info');
    const info = await res.json();
    setPluginInfo(info);
    document.getElementById('header-title').textContent = info.name || 'AirGate';
    document.getElementById('header-mark').textContent = (info.name || 'AG').slice(0, 2).toUpperCase();
    document.title = (info.name || 'AirGate') + ' Dev';
    document.getElementById('header-version').textContent = 'v' + (info.version || '?');
    document.getElementById('header-desc').textContent = info.description || '';
    renderEndpoints(info);
    renderTypeCards();
  } catch {
    document.getElementById('header-desc').textContent = '无法加载插件信息';
  }
  loadAccounts();
  loadScheduler();
  setInterval(loadScheduler, 10000);
}

// ─── 事件绑定 ───

// 调度器：策略切换按钮
document.getElementById('sched-switch').addEventListener('click', (e) => {
  const btn = e.target.closest('.sched-mode-btn');
  if (btn) setPolicy(btn.dataset.policy);
});

// 调度器：目标账号选择
document.getElementById('sched-pinned-select').addEventListener('click', (e) => {
  const option = e.target.closest('[data-pinned-value]');
  if (option) {
    choosePinnedSelect(option.dataset.pinnedValue);
    return;
  }
  if (e.target.closest('.sched-pinned-trigger')) togglePinnedSelect();
});

document.addEventListener('click', (e) => {
  if (!e.target.closest('#sched-pinned-select')) closePinnedSelect();
});

// 添加账号按钮
document.getElementById('btn-add-account').addEventListener('click', () => showForm());

document.getElementById('theme-toggle').addEventListener('click', () => {
  toggleTheme();
});

// 表单：取消 / 保存
document.getElementById('btn-cancel').addEventListener('click', hideForm);
document.getElementById('btn-save').addEventListener('click', saveAccount);

// 账号列表的事件委托（按钮通过 data-action 和 data-id 触发）
document.getElementById('account-list').addEventListener('click', (e) => {
  const btn = e.target.closest('[data-action]');
  if (!btn) {
    const tag = e.target.closest('.acct-weight-tag');
    if (tag) {
      editWeight(e, parseInt(tag.dataset.id), parseInt(tag.dataset.weight));
    }
    return;
  }
  const id = parseInt(btn.dataset.id);
  switch (btn.dataset.action) {
    case 'test': testAccount(id); break;
    case 'edit': editAccount(id); break;
    case 'delete': deleteAccount(id); break;
  }
});

// 类型卡片事件委托
document.getElementById('type-cards').addEventListener('click', (e) => {
  const option = e.target.closest('.type-option');
  if (option) selectType(option.dataset.type);
});

// 弹窗外部点击关闭
document.getElementById('form-overlay').addEventListener('click', (e) => {
  if (e.target === e.currentTarget) hideForm();
});

// ESC 关闭弹窗
document.addEventListener('keydown', e => { if (e.key === 'Escape') hideForm(); });

init();
