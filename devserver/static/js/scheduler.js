// 调度器面板逻辑

import { API } from './utils.js';

let schedulerData = null;
let pinnedOptions = [];
let pinnedOpen = false;

export async function loadScheduler() {
  try {
    const res = await fetch(API + '/api/scheduler');
    schedulerData = await res.json();
    renderScheduler();
  } catch {
    document.getElementById('sched-stats').innerHTML =
      '<div class="sched-chip" style="grid-column:1/-1"><div class="sched-chip-label">状态</div><div class="sched-chip-val" style="color:var(--ag-text-tertiary)">不可用</div></div>';
  }
}

function renderScheduler() {
  if (!schedulerData) return;
  const policy = schedulerData.policy || 'none';
  const isNone = policy === 'none';

  document.getElementById('sched-badge').textContent = isNone ? '直连' : '加权轮询';

  document.querySelectorAll('.sched-mode-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.policy === policy);
  });

  const cooldowns = schedulerData.cooldowns || {};
  const cooldownCount = Object.keys(cooldowns).length;
  const weights = schedulerData.weights || {};
  const accountCount = Object.keys(weights).length;

  if (isNone) {
    document.getElementById('sched-stats').innerHTML = `
      <div class="sched-chip" style="grid-column:1/-1">
        <div class="sched-chip-label">模式</div>
        <div class="sched-chip-val" style="color:var(--ag-primary)">直连 · 所有请求发往单一账号</div>
      </div>
    `;
  } else {
    document.getElementById('sched-stats').innerHTML = `
      <div class="sched-chip">
        <div class="sched-chip-label">可用账号</div>
        <div class="sched-chip-val" style="color:${accountCount > 0 ? 'var(--ag-success)' : 'var(--ag-text-tertiary)'}">${accountCount}</div>
      </div>
      <div class="sched-chip">
        <div class="sched-chip-label">冷却中</div>
        <div class="sched-chip-val" style="color:${cooldownCount > 0 ? 'var(--ag-warning)' : 'var(--ag-success)'}">${cooldownCount}</div>
      </div>
    `;
  }

  const pinnedEl = document.getElementById('sched-pinned');
  pinnedEl.style.display = isNone ? 'block' : 'none';
  if (isNone) {
    renderPinnedSelect(schedulerData.pinned_id || 0);
  }

  const cdEl = document.getElementById('sched-cooldowns');
  if (!isNone && cooldownCount > 0) {
    cdEl.innerHTML = `<div class="cooldown-list">${
      Object.entries(cooldowns).map(([id, t]) => `#${id} ${t}`).join(' · ')
    }</div>`;
  } else {
    cdEl.innerHTML = '';
  }
}

export async function setPolicy(policy) {
  await fetch(API + '/api/scheduler/policy', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ policy })
  });
  loadScheduler();
}

async function renderPinnedSelect(pinnedId) {
  try {
    const res = await fetch(API + '/api/accounts');
    const accounts = await res.json();
    pinnedOptions = [
      { value: '0', label: '第一个账号（默认）' },
      ...(accounts || []).map(a => ({ value: String(a.id), label: `${a.name || '未命名'} (#${a.id})` }))
    ];
  } catch {
    pinnedOptions = [{ value: '0', label: '无法加载账号' }];
  }
  drawPinnedSelect(String(pinnedId || 0));
}

function drawPinnedSelect(value) {
  const el = document.getElementById('sched-pinned-select');
  const selected = pinnedOptions.find(option => option.value === value) || pinnedOptions[0];
  const menu = pinnedOpen ? `<div class="sched-pinned-menu" role="listbox">${pinnedOptions.map(option => {
    const active = option.value === (selected?.value || '0');
    return `<button type="button" class="sched-pinned-option${active ? ' active' : ''}" role="option" aria-selected="${active}" data-pinned-value="${option.value}">${option.label}</button>`;
  }).join('')}</div>` : '';
  el.innerHTML = `<button type="button" class="sched-pinned-trigger" aria-haspopup="listbox" aria-expanded="${pinnedOpen}">
    <span class="sched-pinned-label">${selected?.label || ''}</span>
    <span class="sched-pinned-caret" aria-hidden="true">v</span>
  </button>${menu}`;
}

export function togglePinnedSelect() {
  pinnedOpen = !pinnedOpen;
  drawPinnedSelect(String(schedulerData?.pinned_id || 0));
}

export function closePinnedSelect() {
  if (!pinnedOpen) return;
  pinnedOpen = false;
  drawPinnedSelect(String(schedulerData?.pinned_id || 0));
}

export function choosePinnedSelect(value) {
  pinnedOpen = false;
  setPinned(value);
}

export async function setPinned(val) {
  await fetch(API + '/api/scheduler/pinned', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ account_id: parseInt(val) || 0 })
  });
  loadScheduler();
}
