/** 随主题变化的语义 token（颜色、阴影等） */
export interface ThemeTokens {
  // 主色调
  primary: string;
  primaryForeground: string;
  primaryHover: string;
  primarySubtle: string;
  primaryGlow: string;

  // 语义色
  success: string;
  successForeground: string;
  successSubtle: string;
  warning: string;
  warningForeground: string;
  warningSubtle: string;
  danger: string;
  dangerForeground: string;
  dangerSubtle: string;
  info: string;
  infoSubtle: string;

  // HeroUI 语义表面
  defaultBg: string;
  defaultForeground: string;
  fieldBackground: string;
  fieldForeground: string;
  fieldPlaceholder: string;
  muted: string;
  overlay: string;
  overlayForeground: string;
  scrollbar: string;
  segment: string;
  segmentForeground: string;
  surface: string;
  surfaceForeground: string;
  surfaceSecondary: string;
  surfaceSecondaryForeground: string;
  surfaceTertiary: string;
  surfaceTertiaryForeground: string;

  // 背景层次
  bgDeep: string;
  bg: string;
  bgElevated: string;
  bgSurface: string;
  bgHover: string;
  bgActive: string;

  // 边框
  border: string;
  borderSubtle: string;
  borderFocus: string;

  // 文字
  text: string;
  textSecondary: string;
  textTertiary: string;
  textInverse: string;

  // 玻璃态
  glass: string;
  glassBorder: string;

  // 阴影
  shadowSm: string;
  shadowMd: string;
  shadowLg: string;
  shadowGlow: string;
}

/** 通用基础 token：组件和布局都可复用 */
export interface FoundationTokens {
  radiusSm: string;
  radiusMd: string;
  radiusLg: string;
  radiusXl: string;
  fieldRadius: string;
  fontSans: string;
  fontMono: string;
  transition: string;
  transitionSlow: string;
}

/** 应用壳层 token：不建议在通用组件中直接依赖 */
export interface AppShellTokens {
  sidebarWidth: string;
  sidebarCollapsed: string;
  topbarHeight: string;
}

/** 不随主题变化的 token */
export interface StaticTokens extends FoundationTokens, AppShellTokens {}

export interface StaticTokenGroups {
  foundation: FoundationTokens;
  appShell: AppShellTokens;
}

export type ThemeName = 'dark' | 'light';

export interface ThemeScopeOptions {
  scopeSelector?: string;
  themeAttribute?: string;
  prefix?: string;
}

export interface ThemeCSSOptions extends ThemeScopeOptions {}

export interface ThemeInjectionOptions extends ThemeScopeOptions {
  styleId?: string;
  targetDocument?: Document;
}

export interface ThemeSetOptions {
  target?: HTMLElement;
  themeAttribute?: string;
  storageKey?: string;
}

export interface ThemeStorageOptions {
  storageKey?: string;
}

export interface CssVarOptions {
  prefix?: string;
}

export interface TailwindBridgeOptions {
  prefix?: string;
}

/** Elevation context — UI 层级上下文，用于自动调整子组件的 token 值 */
export type ElevationContext = 'modal' | 'dropdown';
