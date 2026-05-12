import { appShellTokens, foundationTokens, themes, staticTokens, lightElevationContexts } from './tokens.js';
/** camelCase → kebab-case */
function toKebab(key) {
    return key.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase());
}
function resolvePrefix(prefix = 'ag') {
    return prefix.trim() || 'ag';
}
function variableName(prefix, key) {
    return `--${prefix}-${toKebab(key)}`;
}
function selectorForScope(scopeSelector = ':root', themeAttribute = 'data-theme', theme) {
    if (!theme)
        return scopeSelector;
    if (scopeSelector === ':root') {
        return `:root[${themeAttribute}="${theme}"]`;
    }
    return `${scopeSelector}[${themeAttribute}="${theme}"]`;
}
function varsBlock(values, prefix) {
    return Object.entries(values)
        .map(([key, value]) => `  ${variableName(prefix, key)}: ${value};`)
        .join('\n');
}
/** 主题 token → CSS 变量名映射 */
export const tokenToCssVar = Object.keys(themes.dark).reduce((acc, key) => {
    acc[key] = variableName('ag', key);
    return acc;
}, {});
/** 静态 token → CSS 变量名映射 */
export const staticToCssVar = Object.keys(staticTokens).reduce((acc, key) => {
    acc[key] = variableName('ag', key);
    return acc;
}, {});
/** 生成基础 token 的 CSS 变量名映射 */
export function createFoundationCssVarMap(options = {}) {
    const prefix = resolvePrefix(options.prefix);
    return Object.keys(foundationTokens).reduce((acc, key) => {
        acc[key] = variableName(prefix, key);
        return acc;
    }, {});
}
/** 生成应用壳层 token 的 CSS 变量名映射 */
export function createAppShellCssVarMap(options = {}) {
    const prefix = resolvePrefix(options.prefix);
    return Object.keys(appShellTokens).reduce((acc, key) => {
        acc[key] = variableName(prefix, key);
        return acc;
    }, {});
}
/** 生成主题 token 的 CSS 变量名映射 */
export function createThemeCssVarMap(options = {}) {
    const prefix = resolvePrefix(options.prefix);
    return Object.keys(themes.dark).reduce((acc, key) => {
        acc[key] = variableName(prefix, key);
        return acc;
    }, {});
}
/** 生成静态 token 的 CSS 变量名映射 */
export function createStaticCssVarMap(options = {}) {
    const prefix = resolvePrefix(options.prefix);
    return Object.keys(staticTokens).reduce((acc, key) => {
        acc[key] = variableName(prefix, key);
        return acc;
    }, {});
}
/**
 * 生成完整的 CSS 变量定义字符串。
 * 默认输出：:root（静态）+ :root[data-theme="dark"] + :root[data-theme="light"]
 * 也支持在局部容器下生成作用域主题。
 */
export function generateThemeCSS(options = {}) {
    const prefix = resolvePrefix(options.prefix);
    const scopeSelector = options.scopeSelector || ':root';
    const themeAttribute = options.themeAttribute || 'data-theme';
    const blocks = [
        `${selectorForScope(scopeSelector)} {\n${varsBlock(staticTokens, prefix)}\n}`,
        `${selectorForScope(scopeSelector, themeAttribute, 'dark')} {\n${varsBlock(themes.dark, prefix)}\n}`,
        `${selectorForScope(scopeSelector, themeAttribute, 'light')} {\n${varsBlock(themes.light, prefix)}\n}`,
    ];
    // Elevation context blocks (light theme only)
    const lightSelector = selectorForScope(scopeSelector, themeAttribute, 'light');
    for (const [ctx, overrides] of Object.entries(lightElevationContexts)) {
        if (Object.keys(overrides).length === 0)
            continue;
        blocks.push(`${lightSelector} .ag-elevation-${ctx} {\n${varsBlock(overrides, prefix)}\n}`);
    }
    return blocks.join('\n\n');
}
/** 运行时注入主题 CSS 到 <head> */
export function injectThemeStyle(options = 'ag-theme-vars') {
    if (typeof document === 'undefined')
        return;
    const resolvedOptions = typeof options === 'string'
        ? { styleId: options }
        : options;
    const targetDocument = resolvedOptions.targetDocument || document;
    const styleId = resolvedOptions.styleId || 'ag-theme-vars';
    let el = targetDocument.getElementById(styleId);
    if (!el) {
        el = targetDocument.createElement('style');
        el.id = styleId;
        targetDocument.head.appendChild(el);
    }
    el.textContent = generateThemeCSS(resolvedOptions);
}
/** 设置当前主题（data-theme 属性 + localStorage） */
export function setTheme(theme, options = {}) {
    if (typeof document === 'undefined')
        return;
    const themeAttribute = options.themeAttribute || 'data-theme';
    const target = options.target || document.documentElement;
    target.setAttribute(themeAttribute, theme);
    try {
        if (typeof localStorage !== 'undefined') {
            localStorage.setItem(options.storageKey || 'ag-theme', theme);
        }
    }
    catch {
        // Theme switching should keep working when storage is unavailable.
    }
}
/** 读取已保存的主题偏好，默认 dark */
export function getStoredTheme(options = {}) {
    if (typeof localStorage === 'undefined')
        return 'dark';
    try {
        return localStorage.getItem(options.storageKey || 'ag-theme') || 'dark';
    }
    catch {
        return 'dark';
    }
}
/** 生成 Tailwind 可消费的 theme bridge */
export function createTailwindThemeBridge(options = {}) {
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
    };
}
