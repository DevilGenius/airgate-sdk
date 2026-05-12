import type { AppShellTokens, ElevationContext, FoundationTokens, StaticTokenGroups, StaticTokens, ThemeName, ThemeTokens } from './types.js';
/** 暗色主题 — HeroUI Theme Builder preset */
export declare const darkTheme: ThemeTokens;
/** 亮色主题 — HeroUI Theme Builder preset */
export declare const lightTheme: ThemeTokens;
/** 通用基础 token：HeroUI Radius 为 Small，Radius Form 为 Medium。 */
export declare const foundationTokens: FoundationTokens;
/** 应用壳层 token */
export declare const appShellTokens: AppShellTokens;
/** 分组后的静态 token */
export declare const staticTokenGroups: StaticTokenGroups;
/** 不随主题变化的静态 token */
export declare const staticTokens: StaticTokens;
/** 图表/头像装饰色（与主题无关的固定调色板） */
export declare const decorativePalette: readonly ["#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#06b6d4", "#ec4899", "#84cc16", "#f97316", "#6366f1", "#0d9488", "#a855f7"];
/** 主题集合 */
export declare const themes: Record<ThemeName, ThemeTokens>;
/**
 * 亮色主题 elevation 上下文覆盖
 * 不同 UI 层级（页面 → 弹窗 → 下拉）需要不同的背景/边框/阴影值。
 * 宿主在容器上设置 .ag-elevation-{context} class，子组件自动继承正确的 token 值。
 */
export declare const lightElevationContexts: Record<ElevationContext, Partial<ThemeTokens>>;
