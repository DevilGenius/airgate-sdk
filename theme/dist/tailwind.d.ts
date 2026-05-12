import { createTailwindThemeBridge } from './css.js';
declare const tailwindThemeBridge: {
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
export default tailwindThemeBridge;
export { createTailwindThemeBridge };
