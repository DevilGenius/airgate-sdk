// 公共工具函数

export const API = '';
export const THEME_STORAGE_KEY = 'ag-theme';

export const typeColors = [
  { bg: 'var(--ag-primary-subtle)', fg: 'var(--ag-primary)' },
  { bg: 'var(--ag-info-subtle)', fg: 'var(--ag-info)' },
  { bg: 'var(--ag-warning-subtle)', fg: 'var(--ag-warning)' },
  { bg: 'var(--ag-success-subtle)', fg: 'var(--ag-success)' },
];

/** 全局插件信息（init 后赋值） */
export let pluginInfo = null;

export function setPluginInfo(info) {
  pluginInfo = info;
}

export function getBadgeStyle(typeKey) {
  if (!pluginInfo?.account_types) return typeColors[0];
  const idx = pluginInfo.account_types.findIndex(t => t.key === typeKey);
  return typeColors[Math.max(0, idx) % typeColors.length];
}

export function getTypeLabel(typeKey) {
  if (!pluginInfo?.account_types) return typeKey;
  return pluginInfo.account_types.find(t => t.key === typeKey)?.label || typeKey;
}

export function maskKey(key) {
  if (!key) return '';
  if (key.length <= 8) return '····';
  return key.slice(0, 4) + '···' + key.slice(-4);
}

export function iconLetter(name, typeKey) {
  if (name) return name[0].toUpperCase();
  return (typeKey || '?')[0].toUpperCase();
}

export function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export function applyTheme(theme) {
  const nextTheme = theme === 'light' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', nextTheme);
  localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
  const toggle = document.getElementById('theme-toggle');
  if (toggle) {
    toggle.textContent = nextTheme === 'light' ? 'Light' : 'Dark';
    toggle.setAttribute('aria-label', nextTheme === 'light' ? '切换到深色主题' : '切换到浅色主题');
  }
}

export function initTheme() {
  const stored = localStorage.getItem(THEME_STORAGE_KEY);
  applyTheme(stored === 'light' ? 'light' : stored === 'dark' ? 'dark' : 'light');
}

export function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  applyTheme(current === 'light' ? 'dark' : 'light');
}
