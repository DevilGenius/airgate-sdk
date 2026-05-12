import {
  useLayoutEffect,
  useRef,
  type ButtonHTMLAttributes,
  type ComponentType,
  type CSSProperties,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
} from 'react';
import { createTailwindThemeBridge, getStoredTheme, injectThemeStyle, setTheme } from './css.js';
import type { ThemeName } from './types.js';

export const DEFAULT_PLUGIN_THEME_ATTRIBUTE = 'data-theme';
export const DEFAULT_PLUGIN_CLASS_PREFIX = 'agw-';
export const DEFAULT_PLUGIN_THEME_STYLE_ID = 'ag-plugin-theme-vars';
export const DEFAULT_PLUGIN_FOUNDATION_STYLE_ID = 'ag-plugin-foundation';

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
  batchImportRefresh?: (
    refreshTokens: string[],
    clientId?: string,
  ) => Promise<PluginOAuthBatchExchangeResult[]>;
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

export interface PluginFrontendModule {
  routes?: PluginRouteDefinition[];
  menuItems?: PluginMenuItemDefinition[];
  accountIdentity?: ComponentType<AccountSurfaceProps>;
  accountCreate?: ComponentType<AccountFormProps>;
  accountEdit?: ComponentType<AccountFormProps>;
  accountUsageWindow?: ComponentType<AccountSurfaceProps>;
  usageMetricDetail?: ComponentType<UsageRecordSurfaceProps>;
  usageCostDetail?: ComponentType<UsageRecordSurfaceProps>;
  platformIcon?: ComponentType<PluginPlatformIconProps>;
}

export const pluginFoundationCssText = `
/* ── AirGate — Plugin Foundation ── */

.agw-form-shell {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
  font-family: var(--ag-font-sans);
  font-size: 0.875rem;
  color: var(--ag-text);
  letter-spacing: 0;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.agw-form-shell > * {
  min-width: 0;
}

.agw-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.agw-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.agw-section-content {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.agw-panel-header {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.agw-panel-title {
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0;
  color: var(--ag-text);
}

.agw-panel-eyebrow {
  font-size: 0.625rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0;
  color: var(--ag-text-tertiary);
  font-family: var(--ag-font-mono);
}

.agw-panel-description {
  font-size: 0.75rem;
  line-height: 1.65;
  color: var(--ag-text-secondary);
}

.agw-label {
  font-size: 0.6875rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0;
  color: var(--ag-text-secondary);
}

.agw-label-required {
  margin-left: 0.25rem;
  color: var(--ag-danger);
}

.agw-hint {
  font-size: 0.75rem;
  line-height: 1.65;
  color: var(--ag-text-tertiary);
}

.agw-input {
  display: block;
  width: 100%;
  border: 1px solid color-mix(in oklab, var(--ag-border) 88%, transparent);
  border-radius: var(--ag-field-radius, 0.5rem);
  background: var(--ag-field-background);
  padding: 0.5rem 0.75rem;
  color: var(--ag-field-foreground);
  font-size: 0.875rem;
  outline: none;
  box-shadow: var(--ag-shadow-sm);
  transition: border-color var(--ag-transition), box-shadow var(--ag-transition), background-color var(--ag-transition);
}

.agw-input::placeholder {
  color: var(--ag-field-placeholder);
}

.agw-input:hover {
  background: color-mix(in oklab, var(--ag-field-background) 86%, var(--ag-surface) 14%);
  border-color: color-mix(in oklab, var(--ag-border) 92%, var(--ag-text) 8%);
}

.agw-input:focus,
.agw-input:focus-visible {
  border-color: var(--ag-border-focus);
  box-shadow: 0 0 0 2px color-mix(in oklab, var(--ag-primary) 22%, transparent);
}

.agw-input-mono {
  font-family: var(--ag-font-mono);
}

.agw-textarea {
  min-height: 76px;
  resize: vertical;
}

.agw-card {
  border: 1px solid var(--ag-border);
  border-radius: var(--ag-radius-sm);
  background: var(--ag-surface);
  padding: 1rem;
  transition: border-color var(--ag-transition), background-color var(--ag-transition), box-shadow var(--ag-transition);
}

.agw-status-inline {
  display: inline-flex;
  align-items: center;
  padding: 0.25rem 0.75rem;
  border: 1px solid var(--ag-border);
  border-radius: var(--ag-radius-sm);
  background: var(--ag-surface-secondary);
  font-size: 0.75rem;
  font-weight: 500;
}

.agw-status-inline-info {
  color: var(--ag-text-secondary);
}

.agw-status-inline-success {
  color: var(--ag-success);
}

.agw-status-inline-error {
  color: var(--ag-danger);
}

.agw-panel {
  gap: 0;
  padding: 1.25rem;
  background: var(--ag-surface);
  border: 1px solid var(--ag-border);
  border-radius: var(--ag-radius-sm);
}

.agw-card-active {
  border-color: var(--ag-border-focus);
  background: var(--ag-primary-subtle);
  box-shadow: 0 0 0 1px color-mix(in oklab, var(--ag-primary) 22%, transparent);
}

.agw-selectable-card {
  position: relative;
  width: 100%;
  overflow: hidden;
  text-align: left;
  cursor: pointer;
}

.agw-selectable-card:hover {
  border-color: var(--ag-border);
  background: var(--ag-bg-surface);
}

.agw-focus-ring:focus-visible {
  outline: 1.5px solid var(--ag-primary);
  outline-offset: 2px;
  box-shadow: 0 0 0 2px color-mix(in oklab, var(--ag-primary) 18%, transparent);
}

.agw-button-primary,
.agw-button-secondary,
.agw-button-outline {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  border-radius: var(--ag-radius-sm);
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 200ms, color 200ms, background-color 200ms, opacity 200ms, box-shadow 200ms;
}

.agw-button-primary {
  border: 1px solid transparent;
  background: var(--ag-primary);
  color: var(--ag-primary-foreground);
  box-shadow: none;
}

.agw-button-primary:hover {
  background: var(--ag-primary-hover);
}

.agw-button-secondary {
  border: 1px solid var(--ag-border);
  background: var(--ag-default-bg);
  color: var(--ag-default-foreground);
}

.agw-button-secondary:hover {
  border-color: var(--ag-border);
  background: var(--ag-bg-hover);
}

.agw-button-outline {
  border: 1px solid var(--ag-border);
  background: transparent;
  color: var(--ag-text);
}

.agw-button-outline:hover {
  background: var(--ag-primary-subtle);
}

.agw-form-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.625rem;
}

.agw-badge {
  display: inline-flex;
  align-items: center;
  border-radius: var(--ag-radius-sm);
  padding: 0.25rem 0.625rem;
  font-size: 0.6875rem;
  font-weight: 500;
  letter-spacing: 0;
}

.agw-badge-neutral {
  background: var(--ag-default-bg);
  color: var(--ag-default-foreground);
}

.agw-badge-success {
  background: var(--ag-success-subtle);
  color: var(--ag-success);
}

.agw-badge-violet {
  background: var(--ag-info-subtle);
  color: var(--ag-info);
}

.agw-badge-info {
  background: var(--ag-primary-subtle);
  color: var(--ag-primary);
}

.agw-button-primary:disabled,
.agw-button-secondary:disabled,
.agw-button-outline:disabled,
.agw-input:disabled,
.agw-selectable-card:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

/* ── Light theme and modal elevation follow HeroUI bridge tokens. ── */

[data-theme="light"] .agw-card,
[data-theme="light"] .agw-panel {
  box-shadow: var(--ag-shadow-sm);
}

.ag-elevation-modal .agw-input {
  background: var(--ag-field-background);
  border-color: color-mix(in oklab, var(--ag-border) 88%, transparent);
  box-shadow: var(--ag-shadow-sm);
}

.ag-elevation-modal .agw-card,
.ag-elevation-modal .agw-panel {
  background: var(--ag-surface);
  border-color: var(--ag-border);
  box-shadow: none;
}

.ag-elevation-modal .agw-card:hover,
.ag-elevation-modal .agw-selectable-card:hover {
  background: var(--ag-surface-secondary);
  border-color: var(--ag-border);
}

.ag-elevation-modal .agw-card-active {
  background: var(--ag-primary-subtle);
  border-color: var(--ag-border-focus);
}

.ag-elevation-modal .agw-button-secondary {
  background: var(--ag-default-bg);
  border-color: var(--ag-border);
}

.ag-elevation-modal .agw-button-secondary:hover {
  background: var(--ag-bg-hover);
  border-color: var(--ag-border);
}
`;

export function injectStyle(id: string, cssText: string, targetDocument?: Document): void {
  const doc = targetDocument || (typeof document !== 'undefined' ? document : undefined);
  if (!doc) return;

  let element = doc.getElementById(id) as HTMLStyleElement | null;
  if (!element) {
    element = doc.createElement('style');
    element.id = id;
    doc.head.appendChild(element);
  }

  if (element.textContent !== cssText) {
    element.textContent = cssText;
  }
}

export function ensurePluginStyleFoundation({
  scopeSelector,
  themeAttribute = DEFAULT_PLUGIN_THEME_ATTRIBUTE,
  themeStyleId = DEFAULT_PLUGIN_THEME_STYLE_ID,
  foundationStyleId = DEFAULT_PLUGIN_FOUNDATION_STYLE_ID,
  extraCssText,
  extraStyleId,
  targetDocument,
}: PluginStyleFoundationOptions): void {
  injectThemeStyle({
    styleId: themeStyleId,
    scopeSelector,
    themeAttribute,
    targetDocument,
  });

  injectStyle(foundationStyleId, pluginFoundationCssText, targetDocument);

  if (extraCssText && extraStyleId) {
    injectStyle(extraStyleId, extraCssText, targetDocument);
  }
}

export function resolvePluginTheme({ storageKey }: ResolvePluginThemeOptions = {}): ThemeName {
  const theme = getStoredTheme({ storageKey });
  return theme === 'light' ? 'light' : 'dark';
}

function resolveInheritedTheme(
  element: HTMLElement,
  themeAttribute: string,
  storageKey?: string,
): ThemeName {
  const scopedAncestor = element.parentElement?.closest(`[${themeAttribute}]`);
  const hostTheme = scopedAncestor?.getAttribute(themeAttribute)
    || document.documentElement.getAttribute(themeAttribute);

  return hostTheme === 'light'
    ? 'light'
    : hostTheme === 'dark'
      ? 'dark'
      : resolvePluginTheme({ storageKey });
}

export function useScopedPluginTheme<T extends HTMLElement>(
  options: ScopedPluginThemeOptions = {},
) {
  const { themeAttribute = DEFAULT_PLUGIN_THEME_ATTRIBUTE, storageKey } = options;
  const ref = useRef<T | null>(null);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;

    const applyTheme = () => {
      setTheme(resolveInheritedTheme(element, themeAttribute, storageKey), {
        target: element,
        themeAttribute,
        storageKey,
      });
    };

    applyTheme();

    const hostElement = element.parentElement?.closest(`[${themeAttribute}]`)
      || document.documentElement;
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === 'attributes' && mutation.attributeName === themeAttribute) {
          applyTheme();
          break;
        }
      }
    });

    observer.observe(hostElement, { attributes: true, attributeFilter: [themeAttribute] });
    return () => observer.disconnect();
  }, [themeAttribute, storageKey]);

  return ref;
}

export function createPluginTailwindConfig({
  scopeSelector,
  classPrefix = DEFAULT_PLUGIN_CLASS_PREFIX,
  tokenPrefix,
}: PluginTailwindConfigOptions) {
  return {
    prefix: classPrefix,
    important: scopeSelector,
    theme: {
      extend: {
        ...createTailwindThemeBridge(tokenPrefix ? { prefix: tokenPrefix } : {}),
      },
    },
    corePlugins: {
      preflight: false,
    },
  } as const;
}

export function cn(...values: Array<string | false | null | undefined>): string {
  return values.filter(Boolean).join(' ');
}

export interface FieldProps {
  label: ReactNode;
  required?: boolean;
  hint?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function Field({ label, required = false, hint, children, className }: FieldProps) {
  return (
    <div className={cn('agw-field', className)}>
      <label className="agw-label">
        {label}
        {required && <span className="agw-label-required">*</span>}
      </label>
      {children}
      {hint ? <div className="agw-hint">{hint}</div> : null}
    </div>
  );
}

export function TextInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cn('agw-input', className)} />;
}

export function SecretInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} type="password" className={cn('agw-input agw-input-mono', className)} />;
}

export function TextArea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={cn('agw-input agw-input-mono agw-textarea', className)} />;
}

export interface PanelHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  eyebrow?: ReactNode;
  className?: string;
}

export function PanelHeader({ title, description, eyebrow, className }: PanelHeaderProps) {
  return (
    <div className={cn('agw-panel-header', className)}>
      {eyebrow ? <div className="agw-panel-eyebrow">{eyebrow}</div> : null}
      <div className="agw-panel-title">{title}</div>
      {description ? <div className="agw-panel-description">{description}</div> : null}
    </div>
  );
}

export interface SectionProps extends PanelHeaderProps {
  children: ReactNode;
  panel?: boolean;
  contentClassName?: string;
}

export function Section({
  title,
  description,
  eyebrow,
  children,
  panel = false,
  className,
  contentClassName,
}: SectionProps) {
  return (
    <div className={cn(panel ? 'agw-panel agw-section' : 'agw-section', className)}>
      <PanelHeader title={title} description={description} eyebrow={eyebrow} />
      <div className={cn('agw-section-content', contentClassName)}>{children}</div>
    </div>
  );
}

export interface CardProps {
  children: ReactNode;
  className?: string;
}

export function Card({ children, className }: CardProps) {
  return <div className={cn('agw-card', className)}>{children}</div>;
}

export interface SelectableCardProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
}

export function SelectableCard({ active = false, className, children, ...props }: SelectableCardProps) {
  return (
    <button
      {...props}
      type={props.type || 'button'}
      className={cn('agw-card agw-selectable-card agw-focus-ring', active && 'agw-card-active', className)}
    >
      {children}
    </button>
  );
}

export type ButtonVariant = 'primary' | 'secondary' | 'outline';

const buttonClassMap: Record<ButtonVariant, string> = {
  primary: 'agw-button-primary',
  secondary: 'agw-button-secondary',
  outline: 'agw-button-outline',
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
}

export function Button({ variant = 'secondary', className, children, ...props }: ButtonProps) {
  return (
    <button
      {...props}
      type={props.type || 'button'}
      className={cn('agw-focus-ring', buttonClassMap[variant], className)}
    >
      {children}
    </button>
  );
}

export interface FormActionsProps {
  children: ReactNode;
  className?: string;
}

export function FormActions({ children, className }: FormActionsProps) {
  return <div className={cn('agw-form-actions', className)}>{children}</div>;
}

export interface BadgeProps {
  children: ReactNode;
  tone?: 'neutral' | 'success' | 'violet' | 'info';
  className?: string;
}

const badgeToneClassMap: Record<NonNullable<BadgeProps['tone']>, string> = {
  neutral: 'agw-badge-neutral',
  success: 'agw-badge-success',
  violet: 'agw-badge-violet',
  info: 'agw-badge-info',
};

export function Badge({ children, tone = 'neutral', className }: BadgeProps) {
  return <span className={cn('agw-badge', badgeToneClassMap[tone], className)}>{children}</span>;
}

export interface StatusTextProps {
  type: PluginStatusKind;
  text: string;
}

const statusClassMap: Record<StatusTextProps['type'], string> = {
  info: 'agw-status-inline-info',
  success: 'agw-status-inline-success',
  error: 'agw-status-inline-error',
};

export function StatusText({ type, text }: StatusTextProps) {
  return <div className={cn('agw-status-inline', statusClassMap[type])}>{text}</div>;
}
