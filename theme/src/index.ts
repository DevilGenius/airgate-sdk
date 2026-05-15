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
  AccountSurfaceProps,
  AccountFormProps,
  BadgeProps,
  ButtonProps,
  ButtonVariant,
  CardProps,
  FieldProps,
  FormActionsProps,
  PluginBatchAccountInput,
  PluginBatchImportResult,
  PluginFrontendModule,
  PluginMenuItemDefinition,
  PluginOAuthBatchExchangeResult,
  PluginOAuthBridge,
  PluginOAuthExchangeResult,
  PluginOAuthStartResult,
  PanelHeaderProps,
  PluginPlatformIconProps,
  PluginStatusKind,
  PluginStyleFoundationOptions,
  PluginTailwindConfigOptions,
  PluginRouteDefinition,
  ResolvePluginThemeOptions,
  ScopedPluginThemeOptions,
  SectionProps,
  SelectableCardProps,
  StatusTextProps,
  UsageRecordSurfaceProps,
  UsageServiceTierFastResolver,
} from './plugin.js';

// 插件前端 SDK：样式注入、主题同步、Tailwind bridge 和公共组件
export {
  DEFAULT_PLUGIN_CLASS_PREFIX,
  DEFAULT_PLUGIN_FOUNDATION_STYLE_ID,
  DEFAULT_PLUGIN_THEME_ATTRIBUTE,
  DEFAULT_PLUGIN_THEME_STYLE_ID,
  Badge,
  Button,
  Card,
  Field,
  FormActions,
  PanelHeader,
  Section,
  SecretInput,
  SelectableCard,
  StatusText,
  TextInput,
  TextArea,
  cn,
  createPluginTailwindConfig,
  ensurePluginStyleFoundation,
  injectStyle,
  pluginFoundationCssText,
  resolvePluginTheme,
  useScopedPluginTheme,
} from './plugin.js';
