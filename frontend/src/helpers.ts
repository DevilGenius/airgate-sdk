import type { CssVarOptions, StaticTokens, ThemeTokens } from './types.js';
import { darkTheme, staticTokens } from './tokens.js';
import { createStaticCssVarMap, createThemeCssVarMap } from './css.js';

/** 所有可用 token 名称 */
export type TokenName = keyof ThemeTokens | keyof StaticTokens;

const defaultThemeCssVarMap = createThemeCssVarMap();
const defaultStaticCssVarMap = createStaticCssVarMap();

/**
 * 获取带 fallback 的 CSS var() 引用。
 * 同时支持主题 token 和静态 token。
 *
 * @example
 * cssVar('primary')    // → 'var(--ag-primary, #3b82f6)'
 * cssVar('bgSurface')  // → 'var(--ag-bg-surface, #1c2237)'
 * cssVar('fieldRadius') // → 'var(--ag-field-radius, 0.5rem)'
 */
export function cssVar(token: TokenName, options: CssVarOptions = {}): string {
  const themeCssVarMap = options.prefix ? createThemeCssVarMap(options) : defaultThemeCssVarMap;
  const staticCssVarMap = options.prefix ? createStaticCssVarMap(options) : defaultStaticCssVarMap;

  if (token in themeCssVarMap) {
    const t = token as keyof ThemeTokens;
    return `var(${themeCssVarMap[t]}, ${darkTheme[t]})`;
  }
  const s = token as keyof StaticTokens;
  return `var(${staticCssVarMap[s]}, ${staticTokens[s]})`;
}

/**
 * 批量生成 CSSProperties 对象。
 *
 * @example
 * themeStyle({ color: 'text', backgroundColor: 'bgSurface', borderRadius: 'radiusMd' })
 */
export function themeStyle(
  mapping: Partial<Record<string, TokenName>>,
  options: CssVarOptions = {},
): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [cssProp, token] of Object.entries(mapping)) {
    if (token) result[cssProp] = cssVar(token, options);
  }
  return result;
}
