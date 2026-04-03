import type {
  AppShellTokens,
  ElevationContext,
  FoundationTokens,
  StaticTokenGroups,
  StaticTokens,
  ThemeName,
  ThemeTokens,
} from './types.js';

/** 暗色主题 — Deep Ocean */
export const darkTheme: ThemeTokens = {
  primary: '#3ecfb4',
  primaryHover: '#62dcc4',
  primarySubtle: 'rgba(62, 207, 180, 0.08)',
  primaryGlow: 'rgba(62, 207, 180, 0.14)',

  success: '#34d399',
  successSubtle: 'rgba(52, 211, 153, 0.12)',
  warning: '#fbbf24',
  warningSubtle: 'rgba(251, 191, 36, 0.12)',
  danger: '#fb7185',
  dangerSubtle: 'rgba(251, 113, 133, 0.12)',
  info: '#7dd3fc',
  infoSubtle: 'rgba(125, 211, 252, 0.12)',

  // 背景：深蓝黑，带微蓝底调增加深度感
  bgDeep: '#06080e',
  bg: '#0c0f17',
  bgElevated: '#131722',
  bgSurface: '#1a1e2a',
  bgHover: '#232836',
  bgActive: '#2c3240',

  // 边框：蓝调透明
  border: 'rgba(148, 175, 225, 0.08)',
  borderSubtle: 'rgba(148, 175, 225, 0.05)',
  borderFocus: 'rgba(62, 207, 180, 0.40)',

  // 文字：微蓝白，长时间阅读更舒适
  text: '#e2e6f0',
  textSecondary: '#8d93a8',
  textTertiary: '#565d73',
  textInverse: '#06080e',

  glass: 'rgba(148, 175, 225, 0.03)',
  glassBorder: 'rgba(148, 175, 225, 0.06)',

  shadowSm: '0 2px 8px rgba(0, 0, 0, 0.36)',
  shadowMd: '0 8px 24px rgba(0, 0, 0, 0.48)',
  shadowLg: '0 20px 48px rgba(0, 0, 0, 0.60)',
  shadowGlow: '0 0 0 1px rgba(62, 207, 180, 0.08), 0 8px 32px rgba(62, 207, 180, 0.10)',
};

/** 亮色主题 — Deep Ocean Light */
export const lightTheme: ThemeTokens = {
  primary: '#0d9488',
  primaryHover: '#0b7e74',
  primarySubtle: 'rgba(13, 148, 136, 0.05)',
  primaryGlow: 'rgba(13, 148, 136, 0.10)',

  success: '#16a34a',
  successSubtle: 'rgba(22, 163, 74, 0.06)',
  warning: '#d97706',
  warningSubtle: 'rgba(217, 119, 6, 0.06)',
  danger: '#e11d48',
  dangerSubtle: 'rgba(225, 29, 72, 0.06)',
  info: '#2563eb',
  infoSubtle: 'rgba(37, 99, 235, 0.06)',

  // 背景：冷蓝灰，与暗色主题色温统一
  bgDeep: '#f1f3f8',
  bg: '#f6f7fb',
  bgElevated: '#ffffff',
  bgSurface: '#ffffff',
  bgHover: '#eaedf5',
  bgActive: '#e0e4ee',

  // 边框：冷蓝调
  border: '#d6dae6',
  borderSubtle: '#e6e9f2',
  borderFocus: 'rgba(13, 148, 136, 0.45)',

  // 文字：深蓝黑，非纯黑，阅读更柔和
  text: '#131830',
  textSecondary: '#424866',
  textTertiary: '#6e7490',
  textInverse: '#ffffff',

  glass: 'rgba(255, 255, 255, 0.92)',
  glassBorder: 'rgba(10, 20, 60, 0.06)',

  // 阴影：带蓝调，更有层次
  shadowSm: '0 1px 3px rgba(10, 20, 60, 0.06), 0 1px 2px rgba(10, 20, 60, 0.04)',
  shadowMd: '0 4px 12px rgba(10, 20, 60, 0.07), 0 2px 4px rgba(10, 20, 60, 0.04)',
  shadowLg: '0 16px 40px rgba(10, 20, 60, 0.09), 0 4px 8px rgba(10, 20, 60, 0.04)',
  shadowGlow: '0 0 0 1px rgba(13, 148, 136, 0.10), 0 8px 24px rgba(13, 148, 136, 0.07)',
};

/** 通用基础 token */
export const foundationTokens: FoundationTokens = {
  radiusSm: '12px',
  radiusMd: '18px',
  radiusLg: '22px',
  radiusXl: '28px',
  fontSans: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
  fontMono: "'JetBrains Mono', 'SF Mono', 'Cascadia Code', monospace",
  transition: '200ms cubic-bezier(0.4, 0, 0.2, 1)',
  transitionSlow: '400ms cubic-bezier(0.4, 0, 0.2, 1)',
};

/** 应用壳层 token */
export const appShellTokens: AppShellTokens = {
  sidebarWidth: '260px',
  sidebarCollapsed: '72px',
  topbarHeight: '64px',
};

/** 分组后的静态 token */
export const staticTokenGroups: StaticTokenGroups = {
  foundation: foundationTokens,
  appShell: appShellTokens,
};

/** 不随主题变化的静态 token（向后兼容的扁平导出） */
export const staticTokens: StaticTokens = {
  ...foundationTokens,
  ...appShellTokens,
};

/** 图表/头像装饰色（与主题无关的固定调色板） */
export const decorativePalette = [
  '#3b82f6', // blue
  '#10b981', // emerald
  '#f59e0b', // amber
  '#ef4444', // red
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#ec4899', // pink
  '#84cc16', // lime
  '#f97316', // orange
  '#6366f1', // indigo
  '#0d9488', // teal (primary)
  '#a855f7', // purple
] as const;

/** 主题集合 */
export const themes: Record<ThemeName, ThemeTokens> = {
  dark: darkTheme,
  light: lightTheme,
};

/**
 * 亮色主题 elevation 上下文覆盖
 * 不同 UI 层级（页面 → 弹窗 → 下拉）需要不同的背景/边框/阴影值。
 * 宿主在容器上设置 .ag-elevation-{context} class，子组件自动继承正确的 token 值。
 */
export const lightElevationContexts: Record<ElevationContext, Partial<ThemeTokens>> = {
  modal: {
    bgElevated: '#eef0f7',
    bgSurface: '#e6e9f2',
    bgHover: '#e0e4ee',
    glassBorder: '#d2d6e3',
    border: '#c8cdd9',
    shadowSm: 'none',
    shadowMd: 'none',
  },
  dropdown: {
    // dropdown 背景由宿主的 .ag-glass-dropdown 容器类处理
    // 预留空位，未来可扩展
  },
};
