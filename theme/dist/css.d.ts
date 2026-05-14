import type { AppShellTokens, CssVarOptions, FoundationTokens, StaticTokens, TailwindBridgeOptions, ThemeCSSOptions, ThemeInjectionOptions, ThemeName, ThemeSetOptions, ThemeStorageOptions, ThemeTokens } from './types.js';
/** 主题 token → CSS 变量名映射 */
export declare const tokenToCssVar: Record<keyof ThemeTokens, string>;
/** 静态 token → CSS 变量名映射 */
export declare const staticToCssVar: Record<keyof StaticTokens, string>;
/** 生成基础 token 的 CSS 变量名映射 */
export declare function createFoundationCssVarMap(options?: CssVarOptions): Record<keyof FoundationTokens, string>;
/** 生成应用壳层 token 的 CSS 变量名映射 */
export declare function createAppShellCssVarMap(options?: CssVarOptions): Record<keyof AppShellTokens, string>;
/** 生成主题 token 的 CSS 变量名映射 */
export declare function createThemeCssVarMap(options?: CssVarOptions): Record<keyof ThemeTokens, string>;
/** 生成静态 token 的 CSS 变量名映射 */
export declare function createStaticCssVarMap(options?: CssVarOptions): Record<keyof StaticTokens, string>;
/**
 * 生成完整的 CSS 变量定义字符串。
 * 默认输出：:root（静态）+ :root[data-theme="dark"] + :root[data-theme="light"]
 * 也支持在局部容器下生成作用域主题。
 */
export declare function generateThemeCSS(options?: ThemeCSSOptions): string;
/** 运行时注入主题 CSS 到 <head> */
export declare function injectThemeStyle(options?: ThemeInjectionOptions | string): void;
/** 设置当前主题（data-theme 属性 + localStorage） */
export declare function setTheme(theme: ThemeName, options?: ThemeSetOptions): void;
/** 读取已保存的主题偏好，默认 dark */
export declare function getStoredTheme(options?: ThemeStorageOptions): ThemeName;
/** 生成 Tailwind 可消费的 theme bridge */
export declare function createTailwindThemeBridge(options?: TailwindBridgeOptions): {
    readonly colors: {
        readonly primary: `var(${string})`;
        readonly 'primary-hover': `var(${string})`;
        readonly 'primary-subtle': `var(${string})`;
        readonly success: `var(${string})`;
        readonly 'success-subtle': `var(${string})`;
        readonly warning: `var(${string})`;
        readonly 'warning-subtle': `var(${string})`;
        readonly danger: `var(${string})`;
        readonly 'danger-subtle': `var(${string})`;
        readonly info: `var(${string})`;
        readonly 'info-subtle': `var(${string})`;
        readonly bg: `var(${string})`;
        readonly 'bg-deep': `var(${string})`;
        readonly 'bg-elevated': `var(${string})`;
        readonly surface: `var(${string})`;
        readonly 'bg-hover': `var(${string})`;
        readonly 'bg-active': `var(${string})`;
        readonly border: `var(${string})`;
        readonly 'border-subtle': `var(${string})`;
        readonly 'border-focus': `var(${string})`;
        readonly text: `var(${string})`;
        readonly 'text-secondary': `var(${string})`;
        readonly 'text-tertiary': `var(${string})`;
        readonly 'text-inverse': `var(${string})`;
        readonly glass: `var(${string})`;
        readonly 'glass-border': `var(${string})`;
    };
    readonly borderRadius: {
        readonly sm: `var(${string})`;
        readonly md: `var(${string})`;
        readonly lg: `var(${string})`;
        readonly xl: `var(${string})`;
        readonly field: `var(${string})`;
    };
    readonly fontFamily: {
        readonly sans: `var(${string})`;
        readonly mono: `var(${string})`;
    };
    readonly boxShadow: {
        readonly sm: `var(${string})`;
        readonly md: `var(${string})`;
        readonly lg: `var(${string})`;
        readonly glow: `var(${string})`;
    };
    readonly transitionDuration: {
        readonly DEFAULT: `var(${string})`;
        readonly slow: `var(${string})`;
    };
};
