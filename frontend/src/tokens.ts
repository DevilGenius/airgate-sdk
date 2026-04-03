import type {
  AppShellTokens,
  ElevationContext,
  FoundationTokens,
  StaticTokenGroups,
  StaticTokens,
  ThemeName,
  ThemeTokens,
} from './types.js';

/** 暗色主题 — Neutral Dark */
export const darkTheme: ThemeTokens = {
  primary: '#2dd4a8',
  primaryHover: '#5de8c2',
  primarySubtle: 'rgba(45, 212, 168, 0.10)',
  primaryGlow: 'rgba(45, 212, 168, 0.18)',

  success: '#22c55e',
  successSubtle: 'rgba(34, 197, 94, 0.14)',
  warning: '#f59e0b',
  warningSubtle: 'rgba(245, 158, 11, 0.14)',
  danger: '#ef4444',
  dangerSubtle: 'rgba(239, 68, 68, 0.14)',
  info: '#60a5fa',
  infoSubtle: 'rgba(96, 165, 250, 0.14)',

  bgDeep: '#0a0a0c',
  bg: '#111113',
  bgElevated: '#18181b',
  bgSurface: '#1e1e21',
  bgHover: '#27272a',
  bgActive: '#303033',

  border: 'rgba(255, 255, 255, 0.08)',
  borderSubtle: 'rgba(255, 255, 255, 0.05)',
  borderFocus: 'rgba(45, 212, 168, 0.40)',

  text: '#ececf0',
  textSecondary: '#a1a1aa',
  textTertiary: '#63636e',
  textInverse: '#0a0a0c',

  glass: 'rgba(255, 255, 255, 0.03)',
  glassBorder: 'rgba(255, 255, 255, 0.07)',

  shadowSm: '0 2px 8px rgba(0, 0, 0, 0.32)',
  shadowMd: '0 8px 24px rgba(0, 0, 0, 0.44)',
  shadowLg: '0 20px 48px rgba(0, 0, 0, 0.56)',
  shadowGlow: '0 0 0 1px rgba(45, 212, 168, 0.08), 0 8px 32px rgba(45, 212, 168, 0.12)',
};

/** 亮色主题 — Clean Neutral White */
export const lightTheme: ThemeTokens = {
  primary: '#0d9488',
  primaryHover: '#0f766e',
  primarySubtle: 'rgba(13, 148, 136, 0.06)',
  primaryGlow: 'rgba(13, 148, 136, 0.12)',

  success: '#15803d',
  successSubtle: 'rgba(21, 128, 61, 0.07)',
  warning: '#b45309',
  warningSubtle: 'rgba(180, 83, 9, 0.07)',
  danger: '#b91c1c',
  dangerSubtle: 'rgba(185, 28, 28, 0.07)',
  info: '#2563eb',
  infoSubtle: 'rgba(37, 99, 235, 0.07)',

  // 背景：接近纯白，中性灰调
  bgDeep: '#f9f9f9',
  bg: '#fcfcfc',
  bgElevated: '#ffffff',
  bgSurface: '#ffffff',
  bgHover: '#f5f5f5',
  bgActive: '#efefef',

  // 边框：中性灰边界
  border: '#e5e5e5',
  borderSubtle: '#f0f0f0',
  borderFocus: 'rgba(13, 148, 136, 0.50)',

  // 文字：纯黑灰阶梯，高对比度
  text: '#1a1a1a',
  textSecondary: '#4a4a4a',
  textTertiary: '#7a7a7a',
  textInverse: '#ffffff',

  glass: 'rgba(255, 255, 255, 0.90)',
  glassBorder: 'rgba(0, 0, 0, 0.06)',

  // 阴影：中性灰调，干净简洁
  shadowSm: '0 1px 3px rgba(0, 0, 0, 0.06), 0 1px 2px rgba(0, 0, 0, 0.04)',
  shadowMd: '0 4px 12px rgba(0, 0, 0, 0.06), 0 2px 4px rgba(0, 0, 0, 0.04)',
  shadowLg: '0 16px 40px rgba(0, 0, 0, 0.08), 0 4px 8px rgba(0, 0, 0, 0.04)',
  shadowGlow: '0 0 0 1px rgba(13, 148, 136, 0.12), 0 8px 24px rgba(13, 148, 136, 0.08)',
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
    bgElevated: '#f9f9f9',
    bgSurface: '#f5f5f5',
    bgHover: '#efefef',
    glassBorder: '#e5e5e5',
    border: '#d9d9d9',
    shadowSm: 'none',
    shadowMd: 'none',
  },
  dropdown: {
    // dropdown 背景由宿主的 .ag-glass-dropdown 容器类处理
    // 预留空位，未来可扩展
  },
};
