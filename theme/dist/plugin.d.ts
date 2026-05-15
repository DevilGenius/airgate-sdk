import { type ButtonHTMLAttributes, type ComponentType, type CSSProperties, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react';
import type { ThemeName } from './types.js';
export declare const DEFAULT_PLUGIN_THEME_ATTRIBUTE = "data-theme";
export declare const DEFAULT_PLUGIN_CLASS_PREFIX = "agw-";
export declare const DEFAULT_PLUGIN_THEME_STYLE_ID = "ag-plugin-theme-vars";
export declare const DEFAULT_PLUGIN_FOUNDATION_STYLE_ID = "ag-plugin-foundation";
export interface PluginStyleFoundationOptions {
    scopeSelector: string;
    themeAttribute?: string;
    themeStyleId?: string;
    foundationStyleId?: string;
    extraCssText?: string;
    extraStyleId?: string;
    targetDocument?: Document;
}
export interface ResolvePluginThemeOptions {
    storageKey?: string;
}
export interface ScopedPluginThemeOptions {
    themeAttribute?: string;
    storageKey?: string;
}
export interface PluginTailwindConfigOptions {
    scopeSelector: string;
    classPrefix?: string;
    tokenPrefix?: string;
}
export type PluginStatusKind = 'info' | 'success' | 'error';
export interface PluginOAuthStartResult {
    authorizeURL: string;
    state: string;
}
export interface PluginOAuthExchangeResult {
    accountType: string;
    accountName: string;
    credentials: Record<string, string>;
}
export interface PluginOAuthBatchExchangeResult {
    accountType: string;
    accountName: string;
    credentials: Record<string, string>;
    status: 'ok' | 'failed';
    error?: string;
}
export interface PluginOAuthBridge {
    start: () => Promise<PluginOAuthStartResult>;
    exchange: (callbackURL: string) => Promise<PluginOAuthExchangeResult>;
    batchExchange?: (sessionKeys: string[]) => Promise<PluginOAuthBatchExchangeResult[]>;
    importRefresh?: (refreshToken: string, clientId?: string) => Promise<PluginOAuthExchangeResult>;
    batchImportRefresh?: (refreshTokens: string[], clientId?: string) => Promise<PluginOAuthBatchExchangeResult[]>;
}
export interface PluginBatchAccountInput {
    name: string;
    type: string;
    credentials: Record<string, string>;
}
export interface PluginBatchImportResult {
    imported: number;
    failed: number;
}
export interface AccountFormProps {
    credentials: Record<string, string>;
    onChange: (credentials: Record<string, string>) => void;
    mode: 'create' | 'edit';
    accountType?: string;
    onAccountTypeChange?: (type: string) => void;
    onSuggestedName?: (name: string) => void;
    onBatchModeChange?: (isBatch: boolean) => void;
    onBatchImport?: (accounts: PluginBatchAccountInput[]) => Promise<PluginBatchImportResult>;
    oauth?: PluginOAuthBridge;
}
export interface PluginRouteDefinition {
    path: string;
    component: ComponentType;
}
export interface PluginMenuItemDefinition {
    path: string;
    title: string;
    icon: string;
}
export interface PluginPlatformIconProps {
    className?: string;
    style?: CSSProperties;
}
export interface AccountSurfaceProps {
    accountId?: string | number;
    accountType?: string;
    context?: Record<string, unknown>;
}
export interface UsageRecordSurfaceProps {
    recordId?: string | number;
    context?: Record<string, unknown>;
}
/**
 * 平台插件自行判断某条 usage 的 service_tier 是否应展示为 fast。
 *
 * 不同上游平台可能使用不同字段值表达 fast/priority/flex 等服务层级，
 * Core 只负责调用该函数并套用统一 fast 样式，不内置平台语义。
 */
export type UsageServiceTierFastResolver = (context?: Record<string, unknown>) => boolean;
export interface PluginFrontendModule {
    routes?: PluginRouteDefinition[];
    menuItems?: PluginMenuItemDefinition[];
    accountIdentity?: ComponentType<AccountSurfaceProps>;
    accountCreate?: ComponentType<AccountFormProps>;
    accountEdit?: ComponentType<AccountFormProps>;
    accountUsageWindow?: ComponentType<AccountSurfaceProps>;
    /**
     * 使用记录列表中“模型”列的行级别扩展渲染器。
     *
     * 用途：为平台补充展示模型分档信息（例如图像分辨率、推理强度、服务层级等），
     * Core 只提供表格骨架与回退渲染，不内置平台语义。
     */
    usageModelMeta?: ComponentType<UsageRecordSurfaceProps>;
    isUsageServiceTierFast?: UsageServiceTierFastResolver;
    usageMetricDetail?: ComponentType<UsageRecordSurfaceProps>;
    usageCostDetail?: ComponentType<UsageRecordSurfaceProps>;
    platformIcon?: ComponentType<PluginPlatformIconProps>;
}
export declare const pluginFoundationCssText = "\n/* \u2500\u2500 AirGate \u2014 Plugin Foundation \u2500\u2500 */\n\n.agw-form-shell {\n  display: flex;\n  flex-direction: column;\n  gap: 1rem;\n  min-width: 0;\n  font-family: var(--ag-font-sans);\n  font-size: 0.875rem;\n  color: var(--ag-text);\n  letter-spacing: 0;\n  -webkit-font-smoothing: antialiased;\n  -moz-osx-font-smoothing: grayscale;\n}\n\n.agw-form-shell > * {\n  min-width: 0;\n}\n\n.agw-field {\n  display: flex;\n  flex-direction: column;\n  gap: 0.375rem;\n}\n\n.agw-section {\n  display: flex;\n  flex-direction: column;\n  gap: 0.75rem;\n}\n\n.agw-section-content {\n  display: flex;\n  flex-direction: column;\n  gap: 0.75rem;\n}\n\n.agw-panel-header {\n  display: flex;\n  flex-direction: column;\n  gap: 0.25rem;\n}\n\n.agw-panel-title {\n  font-size: 0.875rem;\n  font-weight: 600;\n  letter-spacing: 0;\n  color: var(--ag-text);\n}\n\n.agw-panel-eyebrow {\n  font-size: 0.625rem;\n  font-weight: 500;\n  text-transform: uppercase;\n  letter-spacing: 0;\n  color: var(--ag-text-tertiary);\n  font-family: var(--ag-font-mono);\n}\n\n.agw-panel-description {\n  font-size: 0.75rem;\n  line-height: 1.65;\n  color: var(--ag-text-secondary);\n}\n\n.agw-label {\n  font-size: 0.6875rem;\n  font-weight: 500;\n  text-transform: uppercase;\n  letter-spacing: 0;\n  color: var(--ag-text-secondary);\n}\n\n.agw-label-required {\n  margin-left: 0.25rem;\n  color: var(--ag-danger);\n}\n\n.agw-hint {\n  font-size: 0.75rem;\n  line-height: 1.65;\n  color: var(--ag-text-tertiary);\n}\n\n.agw-input {\n  display: block;\n  width: 100%;\n  border: 1px solid color-mix(in oklab, var(--ag-border) 88%, transparent);\n  border-radius: var(--ag-field-radius, 0.5rem);\n  background: var(--ag-field-background);\n  padding: 0.5rem 0.75rem;\n  color: var(--ag-field-foreground);\n  font-size: 0.875rem;\n  outline: none;\n  box-shadow: var(--ag-shadow-sm);\n  transition: border-color var(--ag-transition), box-shadow var(--ag-transition), background-color var(--ag-transition);\n}\n\n.agw-input::placeholder {\n  color: var(--ag-field-placeholder);\n}\n\n.agw-input:hover {\n  background: color-mix(in oklab, var(--ag-field-background) 86%, var(--ag-surface) 14%);\n  border-color: color-mix(in oklab, var(--ag-border) 92%, var(--ag-text) 8%);\n}\n\n.agw-input:focus,\n.agw-input:focus-visible {\n  border-color: var(--ag-border-focus);\n  box-shadow: 0 0 0 2px color-mix(in oklab, var(--ag-primary) 22%, transparent);\n}\n\n.agw-input-mono {\n  font-family: var(--ag-font-mono);\n}\n\n.agw-textarea {\n  min-height: 76px;\n  resize: vertical;\n}\n\n.agw-card {\n  border: 1px solid var(--ag-border);\n  border-radius: var(--ag-radius-sm);\n  background: var(--ag-surface);\n  padding: 1rem;\n  transition: border-color var(--ag-transition), background-color var(--ag-transition), box-shadow var(--ag-transition);\n}\n\n.agw-status-inline {\n  display: inline-flex;\n  align-items: center;\n  padding: 0.25rem 0.75rem;\n  border: 1px solid var(--ag-border);\n  border-radius: var(--ag-radius-sm);\n  background: var(--ag-surface-secondary);\n  font-size: 0.75rem;\n  font-weight: 500;\n}\n\n.agw-status-inline-info {\n  color: var(--ag-text-secondary);\n}\n\n.agw-status-inline-success {\n  color: var(--ag-success);\n}\n\n.agw-status-inline-error {\n  color: var(--ag-danger);\n}\n\n.agw-panel {\n  gap: 0;\n  padding: 1.25rem;\n  background: var(--ag-surface);\n  border: 1px solid var(--ag-border);\n  border-radius: var(--ag-radius-sm);\n}\n\n.agw-card-active {\n  border-color: var(--ag-border-focus);\n  background: var(--ag-primary-subtle);\n  box-shadow: 0 0 0 1px color-mix(in oklab, var(--ag-primary) 22%, transparent);\n}\n\n.agw-selectable-card {\n  position: relative;\n  width: 100%;\n  overflow: hidden;\n  text-align: left;\n  cursor: pointer;\n}\n\n.agw-selectable-card:hover {\n  border-color: var(--ag-border);\n  background: var(--ag-bg-surface);\n}\n\n.agw-focus-ring:focus-visible {\n  outline: 1.5px solid var(--ag-primary);\n  outline-offset: 2px;\n  box-shadow: 0 0 0 2px color-mix(in oklab, var(--ag-primary) 18%, transparent);\n}\n\n.agw-button-primary,\n.agw-button-secondary,\n.agw-button-outline {\n  display: inline-flex;\n  align-items: center;\n  justify-content: center;\n  gap: 0.5rem;\n  border-radius: var(--ag-radius-sm);\n  padding: 0.5rem 1rem;\n  font-size: 0.875rem;\n  font-weight: 500;\n  cursor: pointer;\n  transition: border-color 200ms, color 200ms, background-color 200ms, opacity 200ms, box-shadow 200ms;\n}\n\n.agw-button-primary {\n  border: 1px solid transparent;\n  background: var(--ag-primary);\n  color: var(--ag-primary-foreground);\n  box-shadow: none;\n}\n\n.agw-button-primary:hover {\n  background: var(--ag-primary-hover);\n}\n\n.agw-button-secondary {\n  border: 1px solid var(--ag-border);\n  background: var(--ag-default-bg);\n  color: var(--ag-default-foreground);\n}\n\n.agw-button-secondary:hover {\n  border-color: var(--ag-border);\n  background: var(--ag-bg-hover);\n}\n\n.agw-button-outline {\n  border: 1px solid var(--ag-border);\n  background: transparent;\n  color: var(--ag-text);\n}\n\n.agw-button-outline:hover {\n  background: var(--ag-primary-subtle);\n}\n\n.agw-form-actions {\n  display: flex;\n  flex-wrap: wrap;\n  align-items: center;\n  gap: 0.625rem;\n}\n\n.agw-badge {\n  display: inline-flex;\n  align-items: center;\n  border-radius: var(--ag-radius-sm);\n  padding: 0.25rem 0.625rem;\n  font-size: 0.6875rem;\n  font-weight: 500;\n  letter-spacing: 0;\n}\n\n.agw-badge-neutral {\n  background: var(--ag-default-bg);\n  color: var(--ag-default-foreground);\n}\n\n.agw-badge-success {\n  background: var(--ag-success-subtle);\n  color: var(--ag-success);\n}\n\n.agw-badge-violet {\n  background: var(--ag-info-subtle);\n  color: var(--ag-info);\n}\n\n.agw-badge-info {\n  background: var(--ag-primary-subtle);\n  color: var(--ag-primary);\n}\n\n.agw-button-primary:disabled,\n.agw-button-secondary:disabled,\n.agw-button-outline:disabled,\n.agw-input:disabled,\n.agw-selectable-card:disabled {\n  cursor: not-allowed;\n  opacity: 0.45;\n}\n\n/* \u2500\u2500 Light theme and modal elevation follow HeroUI bridge tokens. \u2500\u2500 */\n\n[data-theme=\"light\"] .agw-card,\n[data-theme=\"light\"] .agw-panel {\n  box-shadow: var(--ag-shadow-sm);\n}\n\n.ag-elevation-modal .agw-input {\n  background: var(--ag-field-background);\n  border-color: color-mix(in oklab, var(--ag-border) 88%, transparent);\n  box-shadow: var(--ag-shadow-sm);\n}\n\n.ag-elevation-modal .agw-card,\n.ag-elevation-modal .agw-panel {\n  background: var(--ag-surface);\n  border-color: var(--ag-border);\n  box-shadow: none;\n}\n\n.ag-elevation-modal .agw-card:hover,\n.ag-elevation-modal .agw-selectable-card:hover {\n  background: var(--ag-surface-secondary);\n  border-color: var(--ag-border);\n}\n\n.ag-elevation-modal .agw-card-active {\n  background: var(--ag-primary-subtle);\n  border-color: var(--ag-border-focus);\n}\n\n.ag-elevation-modal .agw-button-secondary {\n  background: var(--ag-default-bg);\n  border-color: var(--ag-border);\n}\n\n.ag-elevation-modal .agw-button-secondary:hover {\n  background: var(--ag-bg-hover);\n  border-color: var(--ag-border);\n}\n";
export declare function injectStyle(id: string, cssText: string, targetDocument?: Document): void;
export declare function ensurePluginStyleFoundation({ scopeSelector, themeAttribute, themeStyleId, foundationStyleId, extraCssText, extraStyleId, targetDocument, }: PluginStyleFoundationOptions): void;
export declare function resolvePluginTheme({ storageKey }?: ResolvePluginThemeOptions): ThemeName;
export declare function useScopedPluginTheme<T extends HTMLElement>(options?: ScopedPluginThemeOptions): import("react").RefObject<T | null>;
export declare function createPluginTailwindConfig({ scopeSelector, classPrefix, tokenPrefix, }: PluginTailwindConfigOptions): {
    readonly prefix: string;
    readonly important: string;
    readonly theme: {
        readonly extend: {
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
    };
    readonly corePlugins: {
        readonly preflight: false;
    };
};
export declare function cn(...values: Array<string | false | null | undefined>): string;
export interface FieldProps {
    label: ReactNode;
    required?: boolean;
    hint?: ReactNode;
    children: ReactNode;
    className?: string;
}
export declare function Field({ label, required, hint, children, className }: FieldProps): import("react/jsx-runtime").JSX.Element;
export declare function TextInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>): import("react/jsx-runtime").JSX.Element;
export declare function SecretInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>): import("react/jsx-runtime").JSX.Element;
export declare function TextArea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>): import("react/jsx-runtime").JSX.Element;
export interface PanelHeaderProps {
    title: ReactNode;
    description?: ReactNode;
    eyebrow?: ReactNode;
    className?: string;
}
export declare function PanelHeader({ title, description, eyebrow, className }: PanelHeaderProps): import("react/jsx-runtime").JSX.Element;
export interface SectionProps extends PanelHeaderProps {
    children: ReactNode;
    panel?: boolean;
    contentClassName?: string;
}
export declare function Section({ title, description, eyebrow, children, panel, className, contentClassName, }: SectionProps): import("react/jsx-runtime").JSX.Element;
export interface CardProps {
    children: ReactNode;
    className?: string;
}
export declare function Card({ children, className }: CardProps): import("react/jsx-runtime").JSX.Element;
export interface SelectableCardProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    active?: boolean;
}
export declare function SelectableCard({ active, className, children, ...props }: SelectableCardProps): import("react/jsx-runtime").JSX.Element;
export type ButtonVariant = 'primary' | 'secondary' | 'outline';
export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: ButtonVariant;
}
export declare function Button({ variant, className, children, ...props }: ButtonProps): import("react/jsx-runtime").JSX.Element;
export interface FormActionsProps {
    children: ReactNode;
    className?: string;
}
export declare function FormActions({ children, className }: FormActionsProps): import("react/jsx-runtime").JSX.Element;
export interface BadgeProps {
    children: ReactNode;
    tone?: 'neutral' | 'success' | 'violet' | 'info';
    className?: string;
}
export declare function Badge({ children, tone, className }: BadgeProps): import("react/jsx-runtime").JSX.Element;
export interface StatusTextProps {
    type: PluginStatusKind;
    text: string;
}
export declare function StatusText({ type, text }: StatusTextProps): import("react/jsx-runtime").JSX.Element;
