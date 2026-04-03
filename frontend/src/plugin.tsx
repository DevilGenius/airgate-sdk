import {
  useLayoutEffect,
  useRef,
  type ButtonHTMLAttributes,
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
  storageKey?: string;
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
  letter-spacing: -0.01em;
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
  letter-spacing: -0.02em;
  color: var(--ag-text);
}

.agw-panel-eyebrow {
  font-size: 0.625rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.1em;
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
  letter-spacing: 0.06em;
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
  border: 1px solid var(--ag-glass-border);
  border-radius: var(--ag-radius-md);
  background: var(--ag-bg-surface);
  padding: 0.5rem 0.75rem;
  color: var(--ag-text);
  font-size: 0.875rem;
  outline: none;
  transition: border-color 200ms, box-shadow 200ms, background-color 200ms;
}

.agw-input::placeholder {
  color: var(--ag-text-tertiary);
}

.agw-input:hover {
  border-color: var(--ag-border);
}

.agw-input:focus,
.agw-input:focus-visible {
  border-color: var(--ag-border-focus);
  box-shadow: 0 0 0 3px var(--ag-primary-subtle);
}

.agw-input-mono {
  font-family: var(--ag-font-mono);
}

.agw-textarea {
  min-height: 76px;
  resize: vertical;
}

.agw-card {
  border: 1px solid var(--ag-glass-border);
  border-radius: var(--ag-radius-lg);
  background: var(--ag-bg-elevated);
  padding: 1rem;
  transition: border-color 200ms, background-color 200ms, box-shadow 200ms;
}

.agw-status-inline {
  display: inline-flex;
  align-items: center;
  padding: 0.25rem 0.75rem;
  border: 1px solid var(--ag-glass-border);
  border-radius: 999px;
  background: var(--ag-bg-surface);
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
  background: var(--ag-bg-elevated);
  border: 1px solid var(--ag-glass-border);
  border-radius: var(--ag-radius-lg);
}

.agw-card-active {
  border-color: var(--ag-border-focus);
  background: var(--ag-bg-surface);
  box-shadow: 0 0 0 1px var(--ag-primary-subtle);
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
}

.agw-button-primary,
.agw-button-secondary,
.agw-button-outline {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  border-radius: var(--ag-radius-md);
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 200ms, color 200ms, background-color 200ms, opacity 200ms, box-shadow 200ms;
}

.agw-button-primary {
  border: 1px solid transparent;
  background: var(--ag-primary);
  color: var(--ag-text-inverse);
  box-shadow: var(--ag-shadow-md);
}

.agw-button-primary:hover {
  background: var(--ag-primary-hover);
  box-shadow: var(--ag-shadow-lg);
}

.agw-button-secondary {
  border: 1px solid var(--ag-glass-border);
  background: var(--ag-bg-surface);
  color: var(--ag-text);
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
  border-radius: 999px;
  padding: 0.25rem 0.625rem;
  font-size: 0.6875rem;
  font-weight: 500;
  letter-spacing: 0.01em;
}

.agw-badge-neutral {
  background: var(--ag-glass);
  color: var(--ag-text-secondary);
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
  opacity: 0.5;
}

/* ── 亮色主题：磨砂玻璃质感 ── */

[data-theme="light"] .agw-input {
  background: rgba(255, 255, 255, 0.35);
  border-color: rgba(180, 195, 220, 0.30);
  box-shadow: 0 1px 2px rgba(100, 116, 160, 0.05) inset;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

[data-theme="light"] .agw-input:focus,
[data-theme="light"] .agw-input:focus-visible {
  background: rgba(255, 255, 255, 0.55);
}

[data-theme="light"] .agw-card,
[data-theme="light"] .agw-panel {
  background: rgba(255, 255, 255, 0.30);
  border-color: rgba(180, 195, 220, 0.25);
  box-shadow:
    0 2px 24px rgba(100, 116, 160, 0.07),
    0 0.5px 0 rgba(255, 255, 255, 0.55) inset;
  backdrop-filter: blur(28px) saturate(1.5);
  -webkit-backdrop-filter: blur(28px) saturate(1.5);
}

[data-theme="light"] .agw-card:hover,
[data-theme="light"] .agw-selectable-card:hover {
  background: rgba(255, 255, 255, 0.42);
  border-color: rgba(180, 195, 220, 0.35);
}

[data-theme="light"] .agw-status-inline {
  background: rgba(255, 255, 255, 0.35);
  border-color: rgba(180, 195, 220, 0.25);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

[data-theme="light"] .agw-button-secondary {
  background: rgba(255, 255, 255, 0.38);
  border-color: rgba(180, 195, 220, 0.30);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

[data-theme="light"] .agw-button-secondary:hover {
  background: rgba(255, 255, 255, 0.55);
  border-color: rgba(180, 195, 220, 0.40);
}

/* ── Elevation: modal — 弹窗内撤销 glass morphism ──
 * 弹窗背景不透明，glass 效果（backdrop-filter, 半透明 background, 投影）既无意义又浪费 GPU。
 * 上面的基础规则使用硬编码 rgba/blur 值，无法通过 .ag-elevation-modal 的变量重定义级联覆盖，
 * 因此这里需要显式：
 *   1. backdrop-filter: none — 撤销模糊
 *   2. box-shadow: none — 去掉多余投影
 *   3. background / border-color 改为 var(--ag-*) 引用 — 让 elevation context 变量生效
 */

[data-theme="light"] .ag-elevation-modal .agw-input {
  background: var(--ag-bg-surface);
  border-color: var(--ag-border);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.04);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

[data-theme="light"] .ag-elevation-modal .agw-input:focus,
[data-theme="light"] .ag-elevation-modal .agw-input:focus-visible {
  background: rgba(0, 0, 0, 0.02);
  border-color: var(--ag-border-focus);
  box-shadow: 0 0 0 3px var(--ag-primary-subtle);
}

[data-theme="light"] .ag-elevation-modal .agw-card,
[data-theme="light"] .ag-elevation-modal .agw-panel {
  background: var(--ag-bg-elevated);
  border-color: var(--ag-glass-border);
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

[data-theme="light"] .ag-elevation-modal .agw-card:hover,
[data-theme="light"] .ag-elevation-modal .agw-selectable-card:hover {
  background: var(--ag-bg-hover);
  border-color: var(--ag-border);
}

[data-theme="light"] .ag-elevation-modal .agw-card-active {
  background: var(--ag-primary-subtle);
  border-color: var(--ag-border-focus);
}

[data-theme="light"] .ag-elevation-modal .agw-button-secondary {
  background: rgba(255, 255, 255, 0.50);
  border-color: var(--ag-glass-border);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

[data-theme="light"] .ag-elevation-modal .agw-button-secondary:hover {
  background: rgba(255, 255, 255, 0.70);
  border-color: var(--ag-border);
}

[data-theme="light"] .ag-elevation-modal .agw-status-inline {
  background: rgba(255, 255, 255, 0.40);
  border-color: var(--ag-glass-border);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}
`;

export function injectStyle(id: string, cssText: string, targetDocument: Document = document): void {
  if (typeof document === 'undefined') return;

  let element = targetDocument.getElementById(id) as HTMLStyleElement | null;
  if (!element) {
    element = targetDocument.createElement('style');
    element.id = id;
    targetDocument.head.appendChild(element);
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

interface FieldProps {
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

interface PanelHeaderProps {
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

interface SectionProps extends PanelHeaderProps {
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

interface CardProps {
  children: ReactNode;
  className?: string;
}

export function Card({ children, className }: CardProps) {
  return <div className={cn('agw-card', className)}>{children}</div>;
}

interface SelectableCardProps extends ButtonHTMLAttributes<HTMLButtonElement> {
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

type ButtonVariant = 'primary' | 'secondary' | 'outline';

const buttonClassMap: Record<ButtonVariant, string> = {
  primary: 'agw-button-primary',
  secondary: 'agw-button-secondary',
  outline: 'agw-button-outline',
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
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

interface FormActionsProps {
  children: ReactNode;
  className?: string;
}

export function FormActions({ children, className }: FormActionsProps) {
  return <div className={cn('agw-form-actions', className)}>{children}</div>;
}

interface BadgeProps {
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

interface StatusTextProps {
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
