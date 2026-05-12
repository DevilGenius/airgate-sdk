import type { CssVarOptions, StaticTokens, ThemeTokens } from './types.js';
/** 所有可用 token 名称 */
export type TokenName = keyof ThemeTokens | keyof StaticTokens;
/**
 * 获取带默认值的 CSS var() 引用。
 * 同时支持主题 token 和静态 token。
 *
 * @example
 * cssVar('primary')    // → 'var(--ag-primary, #3b82f6)'
 * cssVar('bgSurface')  // → 'var(--ag-bg-surface, #1c2237)'
 * cssVar('fieldRadius') // → 'var(--ag-field-radius, 0.5rem)'
 */
export declare function cssVar(token: TokenName, options?: CssVarOptions): string;
/**
 * 批量生成 CSSProperties 对象。
 *
 * @example
 * themeStyle({ color: 'text', backgroundColor: 'bgSurface', borderRadius: 'radiusMd' })
 */
export declare function themeStyle(mapping: Partial<Record<string, TokenName>>, options?: CssVarOptions): Record<string, string>;
