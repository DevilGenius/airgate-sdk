// 类型
export type {
  AppShellTokens,
  CssVarOptions,
  ElevationContext,
  FoundationTokens,
  StaticTokenGroups,
  StaticTokens,
  TailwindBridgeOptions,
  ThemeCSSOptions,
  ThemeInjectionOptions,
  ThemeName,
  ThemeSetOptions,
  ThemeStorageOptions,
  ThemeTokens,
} from './types.js';

// Token 常量
export {
  appShellTokens,
  darkTheme,
  foundationTokens,
  lightElevationContexts,
  lightTheme,
  staticTokenGroups,
  staticTokens,
  decorativePalette,
  themes,
} from './tokens.js';

// CSS 生成与运行时
export {
  createAppShellCssVarMap,
  createFoundationCssVarMap,
  createStaticCssVarMap,
  createTailwindThemeBridge,
  createThemeCssVarMap,
  generateThemeCSS,
  getStoredTheme,
  injectThemeStyle,
  setTheme,
  staticToCssVar,
  tokenToCssVar,
} from './css.js';

// 插件 Helper
export type { TokenName } from './helpers.js';
export { cssVar, themeStyle } from './helpers.js';

export type {
  AccountFormProps,
  PluginBatchAccountInput,
  PluginBatchImportResult,
  PluginFrontendModule,
  PluginMenuItemDefinition,
  PluginOAuthBatchExchangeResult,
  PluginOAuthBridge,
  PluginOAuthExchangeResult,
  PluginOAuthStartResult,
  PluginPlatformIconProps,
  PluginRouteDefinition,
} from './plugin.js';
