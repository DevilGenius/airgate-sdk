# AirGate 插件前端样式规范

本文档为插件开发者提供完整的前端样式指南，确保插件 UI 与 AirGate Core 保持视觉一致。

---

## 1. 项目结构

```
your-plugin/
├── backend/           # Go 后端（gRPC 插件）
└── web/               # 前端
    ├── src/
    │   ├── index.ts              # 入口，导出 PluginFrontendModule
    │   ├── theme/
    │   │   └── runtime.ts        # 主题初始化
    │   ├── styles/
    │   │   └── tailwind.css      # Tailwind 组件层样式
    │   └── components/
    │       └── ...               # 业务组件
    ├── vite.config.ts
    ├── tailwind.config.ts
    ├── postcss.config.cjs
    ├── tsconfig.json
    └── package.json
```

## 2. 依赖配置

插件前端 SDK 的正式包名是 `@airgate/theme`。插件业务代码优先使用 `@airgate/theme/plugin`，该入口提供主题初始化、样式作用域、Tailwind bridge、插件前端类型和公共 UI 组件。

### package.json

```json
{
  "type": "module",
  "scripts": {
    "build": "vite build",
    "dev": "vite build --watch"
  },
  "dependencies": {
    "@airgate/theme": "file:../../airgate-sdk/frontend",
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@types/react": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.21",
    "postcss": "^8.5.6",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.7.0",
    "vite": "^6.0.0"
  }
}
```

> **注意**：`react` 和 `react-dom` 仅用于类型，运行时由 Core 通过 `window.__airgate_shared` 提供。

### vite.config.ts

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    lib: {
      entry: 'src/index.ts',
      formats: ['es'],
      fileName: 'index',
    },
    outDir: 'dist',
    rollupOptions: {
      // React 由 Core 提供，不要打包
      external: ['react', 'react-dom', 'react/jsx-runtime'],
    },
  },
});
```

### tailwind.config.ts

```ts
import type { Config } from 'tailwindcss';
import { createPluginTailwindConfig } from '@airgate/theme/plugin';

const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  ...createPluginTailwindConfig({
    scopeSelector: '[data-ag-YOUR_PLUGIN-root]',  // 替换为你的插件标识
  }),
};

export default config;
```

`createPluginTailwindConfig` 会自动生成：
- `prefix: 'agw-'` — 所有工具类加 `agw-` 前缀，避免与 Core 冲突
- `important: scopeSelector` — 确保样式仅在插件容器内生效
- `theme.extend` — 注入与 Core 一致的设计 token（颜色、圆角、阴影等）
- `corePlugins.preflight: false` — 不注入 CSS reset

### postcss.config.cjs

```js
module.exports = {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

## 3. 主题初始化

### theme/runtime.ts

```ts
import { ensurePluginStyleFoundation } from '@airgate/theme/plugin';
import tailwindCssText from '../styles/tailwind.css?inline';

export const THEME_SCOPE_SELECTOR = '[data-ag-YOUR_PLUGIN-root]';
export const THEME_ATTRIBUTE = 'data-theme';
export const STYLE_ID = 'ag-YOUR_PLUGIN-theme-vars';
export const FOUNDATION_STYLE_ID = 'ag-YOUR_PLUGIN-plugin-foundation';
export const TAILWIND_STYLE_ID = 'ag-YOUR_PLUGIN-tailwind';

export function ensurePluginStyles(): void {
  ensurePluginStyleFoundation({
    scopeSelector: THEME_SCOPE_SELECTOR,
    themeAttribute: THEME_ATTRIBUTE,
    themeStyleId: STYLE_ID,
    foundationStyleId: FOUNDATION_STYLE_ID,
    extraCssText: tailwindCssText,     // 注入编译好的 Tailwind CSS
    extraStyleId: TAILWIND_STYLE_ID,
  });
}
```

> `ensurePluginStyleFoundation` 会把主题 CSS 变量和基础组件样式注入到 `<head>`。只需调用一次。

### index.ts（入口）

```ts
import { YourComponent } from './components/YourComponent';
import { ensurePluginStyles } from './theme/runtime';

// 模块加载时立即注入样式
ensurePluginStyles();

export default {
  accountCreate: YourComponent,  // 或 accountEdit / accountIdentity / accountUsageWindow / usageMetricDetail / usageCostDetail / routes / menuItems
};
```

## 4. 组件根节点 — 作用域绑定

每个插件的根组件必须：
1. 设置 `data-ag-YOUR_PLUGIN-root` 属性（CSS 作用域锚点）
2. 使用 `useScopedPluginTheme` 跟随 Core 的明暗切换

```tsx
import { useScopedPluginTheme } from '@airgate/theme/plugin';

const THEME_ATTRIBUTE = 'data-theme';
const STORAGE_KEY = 'ag-YOUR_PLUGIN-theme';

function usePluginScopedTheme<T extends HTMLElement>() {
  return useScopedPluginTheme<T>({
    themeAttribute: THEME_ATTRIBUTE,
    storageKey: STORAGE_KEY,
  });
}

export function YourComponent(props) {
  const rootRef = usePluginScopedTheme<HTMLDivElement>();

  return (
    <div ref={rootRef} data-ag-YOUR_PLUGIN-root className="agw-form-shell">
      {/* 插件内容 */}
    </div>
  );
}
```

## 5. 设计 Token 参考

所有插件样式通过 CSS 变量 `--ag-*` 引用，Tailwind 工具类已映射好（带 `agw-` 前缀）。具体 token 值由 `frontend/src/tokens.ts` 生成，文档只说明语义，避免样式值漂移。

### 颜色

| Token | Tailwind 类 | 用途 |
|---|---|---|
| `--ag-primary` | `agw-text-primary` / `agw-bg-primary` | 主操作、链接、选中态 |
| `--ag-primary-hover` | `agw-bg-primary-hover` | 主色悬停 |
| `--ag-primary-subtle` | `agw-bg-primary-subtle` | 主色背景/高亮 |
| `--ag-primary-glow` | — | 发光阴影 |
| `--ag-success` | `agw-text-success` | 成功状态 |
| `--ag-warning` | `agw-text-warning` | 警告状态 |
| `--ag-danger` | `agw-text-danger` | 错误/删除 |
| `--ag-info` | `agw-text-info` | 信息/辅助色 |

### 背景层级

从深到浅，形成空间层次：

| Token | Tailwind 类 | 用途 |
|---|---|---|
| `--ag-bg-deep` | `agw-bg-bg-deep` | 页面最底层 |
| `--ag-bg` | `agw-bg-bg` | 侧边栏/主面板 |
| `--ag-bg-elevated` | `agw-bg-bg-elevated` | 卡片/弹窗/下拉 |
| `--ag-bg-surface` | `agw-bg-surface` | 输入框/表单区域 |
| `--ag-bg-hover` | `agw-bg-bg-hover` | 悬停态背景 |
| `--ag-bg-active` | `agw-bg-bg-active` | 按下/激活态 |

### 文字

| Token | Tailwind 类 | 用途 |
|---|---|---|
| `--ag-text` | `agw-text-text` | 主文字 |
| `--ag-text-secondary` | `agw-text-text-secondary` | 次要文字/标签 |
| `--ag-text-tertiary` | `agw-text-text-tertiary` | 提示文字/占位符 |
| `--ag-text-inverse` | `agw-text-text-inverse` | 反色（主色按钮文字） |

### 边框

| Token | Tailwind 类 | 用途 |
|---|---|---|
| `--ag-border` | `agw-border-border` | 标准边框 |
| `--ag-border-subtle` | `agw-border-border-subtle` | 极淡边框 |
| `--ag-border-focus` | `agw-border-border-focus` | 焦点态边框 |
| `--ag-glass-border` | `agw-border-glass-border` | 卡片/容器边框（推荐默认使用） |

### 其他

| Token | Tailwind 类 | 用途 |
|---|---|---|
| `--ag-radius-sm` | `agw-rounded-sm` | 小圆角 |
| `--ag-radius-md` | `agw-rounded-md` | 中圆角 |
| `--ag-radius-lg` | `agw-rounded-lg` | 大圆角 |
| `--ag-field-radius` | `agw-rounded-field` | 表单控件圆角 |
| `--ag-font-sans` | `agw-font-sans` | 正文字体 |
| `--ag-font-mono` | `agw-font-mono` | 等宽字体 |

## 6. SDK 提供的 UI 组件

SDK 提供了一套预制组件（`@airgate/theme/plugin`），样式与 Core 保持一致，属于插件前端稳定公共契约。插件业务 UI **优先使用这些组件**：

```tsx
import {
  cn,              // 类名合并工具
  Field,           // 表单字段（label + input + hint）
  TextInput,       // 文本输入框
  SecretInput,     // 密码输入框（monospace）
  TextArea,        // 多行文本
  Section,         // 分区容器（eyebrow + title + description + content）
  Card,            // 卡片容器
  SelectableCard,  // 可选择卡片（单选卡片组）
  Button,          // 按钮（primary / secondary / outline）
  FormActions,     // 表单操作区（flex wrap）
  Badge,           // 标签徽章（neutral / success / violet / info）
  StatusText,      // 内联状态文字（info / success / error）
} from '@airgate/theme/plugin';
```

### 使用示例

```tsx
// 表单分区
<Section
  title="API 凭证"
  description="输入你的 API Key 以接入服务。"
  eyebrow="认证信息 *"
  panel                          // 加上 panel 会有卡片背景
  contentClassName="agw-gap-3"
>
  <Field label="API Key" required hint="可在服务商控制台获取">
    <SecretInput
      value={apiKey}
      onChange={(e) => setApiKey(e.target.value)}
      placeholder="sk-..."
    />
  </Field>
</Section>

// 可选择卡片组
<div className="agw-grid agw-grid-cols-2 agw-gap-3">
  <SelectableCard active={type === 'a'} onClick={() => setType('a')}>
    <div className="agw-text-sm agw-font-semibold agw-text-text">选项 A</div>
    <div className="agw-text-xs agw-text-text-secondary">描述文字</div>
  </SelectableCard>
  <SelectableCard active={type === 'b'} onClick={() => setType('b')}>
    <div className="agw-text-sm agw-font-semibold agw-text-text">选项 B</div>
    <div className="agw-text-xs agw-text-text-secondary">描述文字</div>
  </SelectableCard>
</div>

// 按钮
<FormActions>
  <Button variant="primary" onClick={handleSave}>保存</Button>
  <Button variant="secondary" onClick={handleCancel}>取消</Button>
</FormActions>
```

## 7. 自定义样式规则

当 SDK 组件不够用时，用 Tailwind 工具类自定义样式：

### 必须遵守

1. **所有类名加 `agw-` 前缀**（Tailwind config 已自动处理）
2. **颜色只用 token**，不要硬编码 `#fff` / `#000`
3. **根节点必须有 `data-ag-YOUR_PLUGIN-root` 属性**

### 推荐做法

```tsx
// ✅ 正确 — 使用 token 颜色
<div className="agw-bg-bg-elevated agw-border agw-border-glass-border agw-rounded-lg agw-p-4">

// ❌ 错误 — 硬编码颜色
<div className="agw-bg-[#1a1a2e] agw-border agw-border-[#333]">

// ✅ 正确 — 使用 token 文字色
<span className="agw-text-text-secondary agw-text-xs">辅助文字</span>

// ❌ 错误 — 硬编码文字色
<span className="agw-text-gray-400 agw-text-xs">辅助文字</span>

// ✅ 正确 — 使用 CSS 变量做动态 style
<div style={{ boxShadow: `0 0 16px var(--ag-primary-glow)` }}>

// ✅ 正确 — 装饰性渐变用 CSS 变量
<div
  className="agw-h-px"
  style={{ background: 'linear-gradient(90deg, transparent, var(--ag-primary-subtle), transparent)' }}
/>
```

### 常用模式

| 场景 | 类名 |
|---|---|
| 卡片容器 | `agw-bg-bg-elevated agw-border agw-border-glass-border agw-rounded-lg` |
| 输入框底 | `agw-bg-surface agw-border agw-border-glass-border agw-rounded-md` |
| 区块标题 | `agw-text-sm agw-font-semibold agw-text-text` |
| 辅助标签 | `agw-text-xs agw-text-text-secondary agw-uppercase agw-tracking-wider` |
| Monospace 标记 | `agw-font-mono agw-text-[0.625rem] agw-uppercase agw-tracking-[0.1em] agw-text-text-tertiary` |
| 微型徽章 | `agw-inline-flex agw-items-center agw-rounded-full agw-px-2 agw-py-0.5 agw-text-[0.6875rem] agw-font-medium` |
| 主色发光按钮 | `agw-bg-primary agw-text-text-inverse` + `style={{ boxShadow: '0 0 16px var(--ag-primary-glow)' }}` |
| 悬停效果 | `hover:agw-bg-bg-hover hover:agw-border-border` |
| 焦点环 | `focus:agw-border-border-focus focus:agw-ring-[3px] focus:agw-ring-primary-subtle` |

## 8. 暗色 / 亮色适配

插件的主题跟随 Core 自动切换，**不需要手动处理**。前提是：

1. 根节点使用了 `useScopedPluginTheme`
2. 所有颜色通过 token 引用

如果用了硬编码颜色，暗/亮切换时会出问题。

## 9. 开发流程

```bash
# 首次安装
cd your-plugin/web && npm install

# 开发（watch 构建，改代码自动重建 dist/index.js）
npm run dev

# 一次性构建
npm run build
```

修改 SDK token 后的刷新流程：
```bash
cd airgate-sdk/frontend && npm run build     # 1. 编译 SDK
cd your-plugin/web && npm run build          # 2. 重建插件（打包新 token）
# 3. 刷新浏览器
```

## 10. Checklist

发布插件前检查：

- [ ] 根节点有 `data-ag-xxx-root` 属性
- [ ] 根节点使用了 `useScopedPluginTheme`
- [ ] 入口调用了 `ensurePluginStyles()`
- [ ] 没有硬编码颜色值（`#xxx`、`rgb()`）
- [ ] 没有使用无前缀的 Tailwind 类（如 `bg-white` 应为 `agw-bg-white`）
- [ ] 暗色和亮色模式下视觉正常
- [ ] `vite.config.ts` 中 React 已 external
- [ ] 构建产物只有一个 `dist/index.js`
