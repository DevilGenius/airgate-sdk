import type {
  AppShellTokens,
  ElevationContext,
  FoundationTokens,
  StaticTokenGroups,
  StaticTokens,
  ThemeName,
  ThemeTokens,
} from './types.js';

/** 暗色主题 — HeroUI Theme Builder preset */
export const darkTheme: ThemeTokens = {
  primary: 'oklch(62.04% 0.1950 262.25)',
  primaryForeground: 'oklch(99.11% 0 0)',
  primaryHover: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 90%, oklch(99.11% 0 0) 10%)',
  primarySubtle: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 15%, transparent)',
  primaryGlow: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 22%, transparent)',

  success: 'oklch(73.29% 0.1942 151.82)',
  successForeground: 'oklch(21.03% 0.0059 151.82)',
  successSubtle: 'color-mix(in oklab, oklch(73.29% 0.1942 151.82) 15%, transparent)',
  warning: 'oklch(82.03% 0.1393 77.35)',
  warningForeground: 'oklch(21.03% 0.0059 77.35)',
  warningSubtle: 'color-mix(in oklab, oklch(82.03% 0.1393 77.35) 15%, transparent)',
  danger: 'oklch(59.40% 0.1974 25.64)',
  dangerForeground: 'oklch(99.11% 0 0)',
  dangerSubtle: 'color-mix(in oklab, oklch(59.40% 0.1974 25.64) 15%, transparent)',
  info: 'oklch(62.04% 0.1950 262.25)',
  infoSubtle: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 15%, transparent)',

  defaultBg: 'oklch(27.40% 0.0017 262.25)',
  defaultForeground: 'oklch(99.11% 0 0)',
  fieldBackground: 'oklch(21.03% 0.0034 262.25)',
  fieldForeground: 'oklch(99.11% 0.0017 262.25)',
  fieldPlaceholder: 'oklch(70.50% 0.0034 262.25)',
  muted: 'oklch(70.50% 0.0034 262.25)',
  overlay: 'oklch(21.03% 0.0034 262.25)',
  overlayForeground: 'oklch(99.11% 0.0017 262.25)',
  scrollbar: 'oklch(70.50% 0.0017 262.25)',
  segment: 'oklch(39.64% 0.0017 262.25)',
  segmentForeground: 'oklch(99.11% 0.0017 262.25)',
  surface: 'oklch(21.03% 0.0034 262.25)',
  surfaceForeground: 'oklch(99.11% 0.0017 262.25)',
  surfaceSecondary: 'oklch(25.70% 0.0025 262.25)',
  surfaceSecondaryForeground: 'oklch(99.11% 0.0017 262.25)',
  surfaceTertiary: 'oklch(27.21% 0.0025 262.25)',
  surfaceTertiaryForeground: 'oklch(99.11% 0.0017 262.25)',

  bgDeep: 'oklch(12.00% 0.0017 262.25)',
  bg: 'oklch(12.00% 0.0017 262.25)',
  bgElevated: 'oklch(21.03% 0.0034 262.25)',
  bgSurface: 'oklch(21.03% 0.0034 262.25)',
  bgHover: 'oklch(25.70% 0.0025 262.25)',
  bgActive: 'oklch(27.21% 0.0025 262.25)',

  border: 'oklch(28.00% 0.0017 262.25)',
  borderSubtle: 'oklch(25.00% 0.0017 262.25)',
  borderFocus: 'oklch(62.04% 0.1950 262.25)',

  text: 'oklch(99.11% 0.0017 262.25)',
  textSecondary: 'oklch(70.50% 0.0034 262.25)',
  textTertiary: 'oklch(70.50% 0.0034 262.25)',
  textInverse: 'oklch(99.11% 0 0)',

  glass: 'color-mix(in oklab, oklch(21.03% 0.0034 262.25) 92%, transparent)',
  glassBorder: 'oklch(28.00% 0.0017 262.25)',

  shadowSm: '0 0 0 0 transparent inset',
  shadowMd: '0 0 0 0 transparent inset',
  shadowLg: '0 0 1px 0 #ffffff4d inset',
  shadowGlow: '0 0 0 1px color-mix(in oklab, oklch(62.04% 0.1950 262.25) 18%, transparent)',
};

/** 亮色主题 — HeroUI Theme Builder preset */
export const lightTheme: ThemeTokens = {
  primary: 'oklch(62.04% 0.1950 262.25)',
  primaryForeground: 'oklch(99.11% 0 0)',
  primaryHover: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 90%, oklch(99.11% 0 0) 10%)',
  primarySubtle: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 15%, transparent)',
  primaryGlow: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 20%, transparent)',

  success: 'oklch(73.29% 0.1942 151.82)',
  successForeground: 'oklch(21.03% 0.0059 151.82)',
  successSubtle: 'color-mix(in oklab, oklch(73.29% 0.1942 151.82) 15%, transparent)',
  warning: 'oklch(78.19% 0.1590 73.34)',
  warningForeground: 'oklch(21.03% 0.0059 73.34)',
  warningSubtle: 'color-mix(in oklab, oklch(78.19% 0.1590 73.34) 15%, transparent)',
  danger: 'oklch(65.32% 0.2336 26.75)',
  dangerForeground: 'oklch(99.11% 0 0)',
  dangerSubtle: 'color-mix(in oklab, oklch(65.32% 0.2336 26.75) 15%, transparent)',
  info: 'oklch(62.04% 0.1950 262.25)',
  infoSubtle: 'color-mix(in oklab, oklch(62.04% 0.1950 262.25) 15%, transparent)',

  defaultBg: 'oklch(94.00% 0.0017 262.25)',
  defaultForeground: 'oklch(21.03% 0.0059 262.25)',
  fieldBackground: 'oklch(96.20% 0.0014 262.25)',
  fieldForeground: 'oklch(21.03% 0.0017 262.25)',
  fieldPlaceholder: 'oklch(55.17% 0.0034 262.25)',
  muted: 'oklch(55.17% 0.0034 262.25)',
  overlay: 'oklch(100.00% 0.0005 262.25)',
  overlayForeground: 'oklch(21.03% 0.0017 262.25)',
  scrollbar: 'oklch(87.10% 0.0017 262.25)',
  segment: 'oklch(100.00% 0.0017 262.25)',
  segmentForeground: 'oklch(21.03% 0.0017 262.25)',
  surface: 'oklch(100.00% 0.0008 262.25)',
  surfaceForeground: 'oklch(21.03% 0.0017 262.25)',
  surfaceSecondary: 'oklch(95.24% 0.0014 262.25)',
  surfaceSecondaryForeground: 'oklch(21.03% 0.0017 262.25)',
  surfaceTertiary: 'oklch(93.73% 0.0014 262.25)',
  surfaceTertiaryForeground: 'oklch(21.03% 0.0017 262.25)',

  bgDeep: 'oklch(97.02% 0.0017 262.25)',
  bg: 'oklch(97.02% 0.0017 262.25)',
  bgElevated: 'oklch(100.00% 0.0008 262.25)',
  bgSurface: 'oklch(100.00% 0.0008 262.25)',
  bgHover: 'oklch(95.24% 0.0014 262.25)',
  bgActive: 'oklch(93.73% 0.0014 262.25)',

  border: 'oklch(90.00% 0.0017 262.25)',
  borderSubtle: 'oklch(92.00% 0.0017 262.25)',
  borderFocus: 'oklch(62.04% 0.1950 262.25)',

  text: 'oklch(21.03% 0.0017 262.25)',
  textSecondary: 'oklch(55.17% 0.0034 262.25)',
  textTertiary: 'oklch(55.17% 0.0034 262.25)',
  textInverse: 'oklch(99.11% 0 0)',

  glass: 'color-mix(in oklab, oklch(100.00% 0.0008 262.25) 92%, transparent)',
  glassBorder: 'oklch(90.00% 0.0017 262.25)',

  shadowSm: '0 2px 4px 0 #0000000a, 0 1px 2px 0 #0000000f, 0 0 1px 0 #0000000f',
  shadowMd: '0 2px 4px 0 #0000000a, 0 1px 2px 0 #0000000f, 0 0 1px 0 #0000000f',
  shadowLg: '0 2px 8px 0 #0000000f, 0 -6px 12px 0 #00000008, 0 14px 28px 0 #00000014',
  shadowGlow: '0 0 0 1px color-mix(in oklab, oklch(62.04% 0.1950 262.25) 14%, transparent)',
};

/** 通用基础 token：HeroUI Radius 为 Small，Radius Form 为 Small。 */
export const foundationTokens: FoundationTokens = {
  radiusSm: '0.25rem',
  radiusMd: '0.25rem',
  radiusLg: '0.25rem',
  radiusXl: '0.25rem',
  fieldRadius: '0.25rem',
  fontSans: "'Geist Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
  fontMono: "'Geist Mono', 'SF Mono', 'Cascadia Code', monospace",
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
    // HeroUI preset already provides overlay/surface tokens for modal elevation.
    // Keep this empty unless a plugin foundation rule needs a scoped correction.
  },
  dropdown: {
    // HeroUI dropdown/tooltip surfaces are now handled by the host bridge.
  },
};
