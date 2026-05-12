// 账号列表、CRUD、连通测试

import { API, getBadgeStyle, getTypeLabel, maskKey, iconLetter, escapeHtml } from './utils.js';
import { loadScheduler } from './scheduler.js';

export async function loadAccounts() {
  const res = await fetch(API + '/api/accounts');
  const accounts = await res.json();
  const el = document.getElementById('account-list');
  if (!accounts?.length) {
    el.innerHTML = '<div class="empty-state">暂无账号<small>点击「+ 添加」创建第一个账号</small></div>';
    return;
  }
  el.innerHTML = accounts.map(a => {
    const style = getBadgeStyle(a.account_type);
    const typeLabel = getTypeLabel(a.account_type);
    const credKeys = Object.keys(a.credentials || {});
    const credHint = credKeys.length > 0 ? maskKey(a.credentials[credKeys[0]]) : '';
    const weight = a.weight || 1;
    return `<div class="acct-item">
      <div class="acct-avatar" style="background:${style.bg};color:${style.fg}">${iconLetter(a.name, a.account_type)}</div>
      <div class="acct-detail">
        <div class="acct-name">${escapeHtml(a.name || '未命名')}</div>
        <div class="acct-sub">
          <span class="acct-type-badge" style="background:${style.bg};color:${style.fg}">${escapeHtml(typeLabel)}</span>
          ${credHint ? `<span>${escapeHtml(credHint)}</span>` : ''}
        </div>
      </div>
      <span id="test-result-${a.id}" class="acct-test-result" style="display:none"></span>
      <span class="acct-weight-tag" title="权重（点击修改）" data-id="${a.id}" data-weight="${weight}">W:${weight}</span>
      <div class="acct-ops">
        <button class="btn-sm" id="test-btn-${a.id}" data-action="test" data-id="${a.id}" title="连通测试">⚡</button>
        <button class="btn-sm" data-action="edit" data-id="${a.id}" title="编辑">✎</button>
        <button class="btn-sm danger" data-action="delete" data-id="${a.id}" title="删除">✕</button>
      </div>
    </div>`;
  }).join('');
}

export function editWeight(event, id, current) {
  event.stopPropagation();
  const el = event.currentTarget;
  const input = document.createElement('input');
  input.className = 'weight-edit';
  input.type = 'number';
  input.min = '0';
  input.value = current;
  el.replaceWith(input);
  input.focus();
  input.select();
  async function commit() {
    const val = parseInt(input.value) || 1;
    await fetch(API + '/api/scheduler/weight/' + id, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ weight: val })
    });
    loadAccounts();
    loadScheduler();
  }
  input.addEventListener('blur', commit);
  input.addEventListener('keydown', e => { if (e.key === 'Enter') input.blur(); });
}

export async function testAccount(id) {
  const btn = document.getElementById('test-btn-' + id);
  const result = document.getElementById('test-result-' + id);
  btn.classList.add('testing');
  result.style.display = 'inline-block';
  result.className = 'acct-test-result loading';
  result.textContent = '测试中...';
  try {
    const res = await fetch(API + '/api/accounts/test/' + id, { method: 'POST' });
    const data = await res.json();
    if (data.ok) {
      result.className = 'acct-test-result ok';
      result.textContent = '✓ ' + data.duration;
    } else {
      result.className = 'acct-test-result fail';
      result.textContent = '✗ ' + (data.error || '失败');
      result.title = data.error || '';
    }
  } catch {
    result.className = 'acct-test-result fail';
    result.textContent = '✗ 网络错误';
  }
  btn.classList.remove('testing');
  setTimeout(() => { result.style.display = 'none'; }, 8000);
}

export async function deleteAccount(id) {
  if (!confirm('确定删除此账号？')) return;
  await fetch(API + '/api/accounts/' + id, { method: 'DELETE' });
  loadAccounts();
  loadScheduler();
}
