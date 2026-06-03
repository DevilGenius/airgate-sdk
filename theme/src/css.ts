import type {
  AppShellTokens,
  CssVarOptions,
  FoundationTokens,
  StaticTokens,
  TailwindBridgeOptions,
  ThemeCSSOptions,
  ThemeInjectionOptions,
  ThemeName,
  ThemeSetOptions,
  ThemeStorageOptions,
  ThemeTokens,
} from './types.js';
import { appShellTokens, foundationTokens, themes, staticTokens, lightElevationContexts } from './tokens.js';

/** camelCase → kebab-case */
function toKebab(key: string): string {
  return key.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase());
}

function resolvePrefix(prefix = 'ag'): string {
  return prefix.trim() || 'ag';
}

function variableName(prefix: string, key: string): string {
  return `--${prefix}-${toKebab(key)}`;
}

function selectorForScope(scopeSelector = ':root', themeAttribute = 'data-theme', theme?: ThemeName): string {
  if (!theme) return scopeSelector;
  if (scopeSelector === ':root') {
    return `:root[${themeAttribute}="${theme}"]`;
  }
  return `${scopeSelector}[${themeAttribute}="${theme}"]`;
}

function varsBlock(
  values: ThemeTokens | StaticTokens | FoundationTokens | AppShellTokens | Partial<ThemeTokens>,
  prefix: string,
): string {
  return Object.entries(values)
    .map(([key, value]) => `  ${variableName(prefix, key)}: ${value};`)
    .join('\n');
}

const cjkPunctuationFontFace = `@font-face {
  font-family: "AirGate CJK Punctuation";
  src:
    local("PingFang SC"),
    local("Microsoft YaHei UI"),
    local("Microsoft YaHei"),
    local("Noto Sans CJK SC"),
    local("Source Han Sans SC"),
    local("Hiragino Sans GB"),
    local("WenQuanYi Micro Hei");
  unicode-range: U+00B7, U+2018-201F, U+2026, U+3000-303F, U+FF00-FFEF;
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
}`;

/** 主题 token → CSS 变量名映射 */
export const tokenToCssVar = Object.keys(themes.dark).reduce(
  (acc, key) => {
    acc[key as keyof ThemeTokens] = variableName('ag', key);
    return acc;
  },
  {} as Record<keyof ThemeTokens, string>,
);

/** 静态 token → CSS 变量名映射 */
export const staticToCssVar = Object.keys(staticTokens).reduce(
  (acc, key) => {
    acc[key as keyof StaticTokens] = variableName('ag', key);
    return acc;
  },
  {} as Record<keyof StaticTokens, string>,
);

/** 生成基础 token 的 CSS 变量名映射 */
export function createFoundationCssVarMap(options: CssVarOptions = {}): Record<keyof FoundationTokens, string> {
  const prefix = resolvePrefix(options.prefix);
  return Object.keys(foundationTokens).reduce(
    (acc, key) => {
      acc[key as keyof FoundationTokens] = variableName(prefix, key);
      return acc;
    },
    {} as Record<keyof FoundationTokens, string>,
  );
}

/** 生成应用壳层 token 的 CSS 变量名映射 */
export function createAppShellCssVarMap(options: CssVarOptions = {}): Record<keyof AppShellTokens, string> {
  const prefix = resolvePrefix(options.prefix);
  return Object.keys(appShellTokens).reduce(
    (acc, key) => {
      acc[key as keyof AppShellTokens] = variableName(prefix, key);
      return acc;
    },
    {} as Record<keyof AppShellTokens, string>,
  );
}

/** 生成主题 token 的 CSS 变量名映射 */
export function createThemeCssVarMap(options: CssVarOptions = {}): Record<keyof ThemeTokens, string> {
  const prefix = resolvePrefix(options.prefix);
  return Object.keys(themes.dark).reduce(
    (acc, key) => {
      acc[key as keyof ThemeTokens] = variableName(prefix, key);
      return acc;
    },
    {} as Record<keyof ThemeTokens, string>,
  );
}

/** 生成静态 token 的 CSS 变量名映射 */
export function createStaticCssVarMap(options: CssVarOptions = {}): Record<keyof StaticTokens, string> {
  const prefix = resolvePrefix(options.prefix);
  return Object.keys(staticTokens).reduce(
    (acc, key) => {
      acc[key as keyof StaticTokens] = variableName(prefix, key);
      return acc;
    },
    {} as Record<keyof StaticTokens, string>,
  );
}

/**
 * 生成完整的 CSS 变量定义字符串。
 * 默认输出：:root（静态）+ :root[data-theme="dark"] + :root[data-theme="light"]
 * 也支持在局部容器下生成作用域主题。
 */
export function generateThemeCSS(options: ThemeCSSOptions = {}): string {
  const prefix = resolvePrefix(options.prefix);
  const scopeSelector = options.scopeSelector || ':root';
  const themeAttribute = options.themeAttribute || 'data-theme';

  const blocks = [
    cjkPunctuationFontFace,
    `${selectorForScope(scopeSelector)} {\n${varsBlock(staticTokens, prefix)}\n}`,
    `${selectorForScope(scopeSelector, themeAttribute, 'dark')} {\n${varsBlock(themes.dark, prefix)}\n}`,
    `${selectorForScope(scopeSelector, themeAttribute, 'light')} {\n${varsBlock(themes.light, prefix)}\n}`,
  ];

  // Elevation context blocks (light theme only)
  const lightSelector = selectorForScope(scopeSelector, themeAttribute, 'light');
  for (const [ctx, overrides] of Object.entries(lightElevationContexts)) {
    if (Object.keys(overrides).length === 0) continue;
    blocks.push(`${lightSelector} .ag-elevation-${ctx} {\n${varsBlock(overrides, prefix)}\n}`);
  }

  return blocks.join('\n\n');
}

/** 运行时注入主题 CSS 到 <head> */
export function injectThemeStyle(options: ThemeInjectionOptions | string = 'ag-theme-vars'): void {
  if (typeof document === 'undefined') return;

  const resolvedOptions = typeof options === 'string'
    ? { styleId: options }
    : options;
  const targetDocument = resolvedOptions.targetDocument || document;
  const styleId = resolvedOptions.styleId || 'ag-theme-vars';

  let el = targetDocument.getElementById(styleId) as HTMLStyleElement | null;
  if (!el) {
    el = targetDocument.createElement('style');
    el.id = styleId;
    targetDocument.head.appendChild(el);
  }
  el.textContent = generateThemeCSS(resolvedOptions);
}

/** 设置当前主题（data-theme 属性 + localStorage） */
export function setTheme(theme: ThemeName, options: ThemeSetOptions = {}): void {
  if (typeof document === 'undefined') return;
  const themeAttribute = options.themeAttribute || 'data-theme';
  const target = options.target || document.documentElement;
  target.setAttribute(themeAttribute, theme);
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(options.storageKey || 'ag-theme', theme);
    }
  } catch {
    // Theme switching should keep working when storage is unavailable.
  }
}

/** 读取已保存的主题偏好，默认 dark */
export function getStoredTheme(options: ThemeStorageOptions = {}): ThemeName {
  if (typeof localStorage === 'undefined') return 'dark';
  try {
    return (localStorage.getItem(options.storageKey || 'ag-theme') as ThemeName) || 'dark';
  } catch {
    return 'dark';
  }
}

/** 生成 Tailwind 可消费的 theme bridge */
export function createTailwindThemeBridge(options: TailwindBridgeOptions = {}) {
  const prefix = resolvePrefix(options.prefix);
  const themeVars = createThemeCssVarMap({ prefix });
  const staticVars = createStaticCssVarMap({ prefix });
  const foundationVars = createFoundationCssVarMap({ prefix });

  return {
    colors: {
      primary: `var(${themeVars.primary})`,
      'primary-hover': `var(${themeVars.primaryHover})`,
      'primary-subtle': `var(${themeVars.primarySubtle})`,
      success: `var(${themeVars.success})`,
      'success-subtle': `var(${themeVars.successSubtle})`,
      warning: `var(${themeVars.warning})`,
      'warning-subtle': `var(${themeVars.warningSubtle})`,
      danger: `var(${themeVars.danger})`,
      'danger-subtle': `var(${themeVars.dangerSubtle})`,
      info: `var(${themeVars.info})`,
      'info-subtle': `var(${themeVars.infoSubtle})`,
      bg: `var(${themeVars.bg})`,
      'bg-deep': `var(${themeVars.bgDeep})`,
      'bg-elevated': `var(${themeVars.bgElevated})`,
      surface: `var(${themeVars.bgSurface})`,
      'bg-hover': `var(${themeVars.bgHover})`,
      'bg-active': `var(${themeVars.bgActive})`,
      border: `var(${themeVars.border})`,
      'border-subtle': `var(${themeVars.borderSubtle})`,
      'border-focus': `var(${themeVars.borderFocus})`,
      text: `var(${themeVars.text})`,
      'text-secondary': `var(${themeVars.textSecondary})`,
      'text-tertiary': `var(${themeVars.textTertiary})`,
      'text-inverse': `var(${themeVars.textInverse})`,
      glass: `var(${themeVars.glass})`,
      'glass-border': `var(${themeVars.glassBorder})`,
    },
    borderRadius: {
      sm: `var(${foundationVars.radiusSm})`,
      md: `var(${foundationVars.radiusMd})`,
      lg: `var(${foundationVars.radiusLg})`,
      xl: `var(${foundationVars.radiusXl})`,
      field: `var(${foundationVars.fieldRadius})`,
    },
    fontFamily: {
      sans: `var(${staticVars.fontSans})`,
      mono: `var(${staticVars.fontMono})`,
    },
    boxShadow: {
      sm: `var(${themeVars.shadowSm})`,
      md: `var(${themeVars.shadowMd})`,
      lg: `var(${themeVars.shadowLg})`,
      glow: `var(${themeVars.shadowGlow})`,
    },
    transitionDuration: {
      DEFAULT: `var(${staticVars.transition})`,
      slow: `var(${staticVars.transitionSlow})`,
    },
  } as const;
}
