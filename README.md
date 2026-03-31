# AirGate SDK

AirGate 插件开发 SDK，定义插件与 Core 之间的全部接口契约。

## 项目概览

AirGate 是一个可扩展的 AI 网关平台，由以下仓库组成：

| 仓库 | 职责 |
| --- | --- |
| `airgate-core` | 运行时引擎：管理后台、账号调度、计费、插件生命周期管理 |
| **`airgate-sdk`** | **接口契约：插件接口、共享类型、gRPC 协议定义** |
| `airgate-openai` | 参考实现：OpenAI 兼容网关插件 |

> SDK 不包含任何业务逻辑，只定义协议边界。

## 安装

```bash
go get github.com/DouDOU-start/airgate-sdk@latest
```

## 架构

```text
┌─────────────────────────────────────────────────┐
│                  airgate-core                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │
│  │ 账号调度  │ │ 计费限流  │ │  路由 & API 网关  │ │
│  └────┬─────┘ └────┬─────┘ └───────┬──────────┘ │
│       └────────────┼───────────────┘             │
│                    │ gRPC (go-plugin)             │
└────────────────────┼─────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        ▼                         ▼
   ┌─────────┐               ┌─────────┐
   │ 网关插件 │               │ 扩展插件 │
   └─────────┘               └─────────┘
   AI API 代理                支付/监控/管理等
```

Core 通过 [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) 以独立进程方式运行插件，插件与 Core 之间通过 gRPC 通信。

## 核心概念

### 职责分工

```text
┌──────────── Core ────────────┐     ┌────────── 网关插件 ──────────┐
│                               │     │                               │
│  账号管理（增删改查、存储）     │     │  声明账号格式（AccountTypes）  │
│  账号调度（负载均衡、选号）     │     │  声明支持的模型（Models）      │
│  路由注册（HTTP 网关、鉴权）   │     │  声明 API 端点（Routes）       │
│  计费、限流、并发控制          │     │  转发请求到上游（Forward）      │
│  凭证验证调度（添加账号时）    │     │  反馈账号状态（ForwardResult）  │
│  账号状态处置（限流/封号/过期）│     │  验证账号凭证（ValidateAccount）│
│  额度巡检调度（定时任务）      │     │  查询账号额度（QueryQuota）     │
│  WebSocket 升级转发           │     │  WebSocket 通信（可选）        │
└───────────────────────────────┘     └───────────────────────────────┘
         通用平台能力                        上游 API 适配器
```

网关插件是一个**适配器**：告诉 Core 这个平台有什么模型、什么端点、账号怎么填，然后负责把请求翻译成上游 API 的格式并转发。Core 做所有通用逻辑。

### 请求生命周期

```text
用户请求 → Core 鉴权 → Core 选账号 → 插件 Forward() → 上游 AI API
                                          ↓
                                    ForwardResult
                                   ┌──────┴──────┐
                              token 用量     账号状态反馈
                              Core 计费      Core 更新账号状态
                                            （限流/封号/过期等）
```

### 账号数据结构

Core 使用**一张 accounts 表**存储所有平台的账号，通过 `platform` + `type` 区分：

```text
accounts 表（Core 数据库）
┌────────────────────────────────────────────────────────────┐
│ 基础标识    id, name, platform, type                        │
│ 凭证       credentials (JSONB，结构由 type 决定)             │
│ 网络       proxy_url                                       │
│ 调度参数    status, priority, rate_multiplier, max_concurrency│
│ 运行时状态  expires_at, rate_limited_at, error_message, ...  │
│            （调度参数 + 运行时状态均为 Core 内部管理）          │
└────────────────────────────────────────────────────────────┘
```

**SDK Account 是 Core 传给插件的"最小视图"**，只包含插件转发时需要的字段：

```go
type Account struct {
    ID          int64             `json:"id"`
    Name        string            `json:"name"`
    Platform    string            `json:"platform"`
    Type        string            `json:"type"`        // 账号类型（对应 AccountType.Key，如 "apikey"、"oauth"）
    Credentials map[string]string `json:"credentials"` // JSONB 透传，结构由 Type 决定
    ProxyURL    string            `json:"proxy_url"`
}
```

> 插件拿到的账号已经是 Core 调度好的结果。调度参数（`status`、`priority`、`rate_multiplier`、`max_concurrency`）是 Core 选号的输入，插件不需要感知；运行时状态（`expires_at`、`rate_limited_at`）是 Core 内部管理的，前端直接查库渲染。

**设计原则**：

- **SDK Account 最小化** — 插件只需"用什么凭证调上游"，调度和计费参数全部留在 Core
- **Core 表可自由扩展** — Core 的 accounts 表字段远多于 SDK Account，前端页面直接查库渲染完整数据
- **`type` 驱动凭证解析** — 插件通过 `AccountTypes` 声明支持的账号类型，Core 存储时带上 `type`，插件 Forward 时据此解析 `credentials`
- **插件 Widget 负责渲染** — 账号管理 UI（表单、详情）统一由插件 Widget 提供，Core 不感知不同插件的账号字段差异

### 关键标识

| 标识 | 含义 | 示例 |
| --- | --- | --- |
| `PluginInfo.ID` | 运行时唯一标识，Core 用于 API 路径、资源挂载、缓存键 | `"gateway-openai"` |
| `Platform()` | 业务平台键，用于账号关联、调度、计费 | `"openai"` |

> `ID` 和 `Platform` 是不同维度：一个插件 ID 对应一个进程，一个 Platform 对应一个上游平台。

### plugin.yaml

`plugin.yaml` 是由插件代码生成的**分发文件**，仅用于安装和市场展示。**运行时真相始终在插件代码里**，Core 不依赖 `plugin.yaml` 做运行时决策。

## 插件类型

SDK 定义两大类插件：

| 类型 | 接口 | 定位 |
| --- | --- | --- |
| 网关 | `GatewayPlugin` | AI API 代理。插件声明模型和路由、实现转发，Core 自动处理调度/计费/限流 |
| 扩展 | `ExtensionPlugin` | 一切非网关功能。拥有路由注册、数据库迁移、后台任务三大能力 |

> 只定义有实际需求的类型。网关专注 AI API 代理，其余全部归入扩展。

### 网关插件

`GatewayPlugin` 需要实现的核心方法：

| 方法 | 职责 |
| --- | --- |
| `Platform()` | 返回业务平台标识（如 `"openai"`） |
| `Models()` | 声明支持的模型列表（含价格，Core 用于计费） |
| `Routes()` | 声明 API 端点（如 `POST /v1/chat/completions`），Core 自动注册路由 |
| `Forward(ctx, req)` | 拿到 Core 调度好的账号，转发请求到上游，返回 token 用量和账号状态反馈 |
| `ValidateAccount(ctx, credentials)` | 验证凭证有效性，Core 在添加/导入账号时调用 |
| `QueryQuota(ctx, credentials)` | 查询账号额度，Core 定时巡检并存入运行时字段，用于调度决策和前端展示 |
| `HandleWebSocket(ctx, conn)` | 处理 WebSocket 双向通信（如 Responses API），连接结束后返回 `ForwardResult` |

`Forward` 和 `HandleWebSocket` 都返回 `ForwardResult`，结构统一：

```go
type ForwardResult struct {
    StatusCode            int           // HTTP 状态码
    InputTokens           int           // 输入 token 数（不含缓存部分）
    OutputTokens          int           // 输出 token 数
    CachedInputTokens     int           // 命中缓存的输入 token 数（仅 cache read）
    ReasoningOutputTokens int           // 推理输出 token 数（o1/o3 等模型）
    ServiceTier           string        // 服务层级，如 standard / flex / priority
    Model                 string        // 实际使用的模型
    Duration              time.Duration // 请求耗时

    // 账号状态反馈（插件识别，Core 处置）
    AccountStatus      string            // "" 正常 / "rate_limited" / "disabled" / "expired"
    ErrorMessage       string            // 上游错误信息（便于排查）
    RetryAfter         time.Duration     // 限流时建议的等待时间
    UpdatedCredentials map[string]string  // 凭证更新（如 token 刷新）
}
```

> 插件负责从上游响应中识别账号异常（429 限流、401 封号等），通过 `AccountStatus` 告诉 Core。Core 自动更新 accounts 表状态并调整调度策略。

Core 自动处理的能力：

- **账号调度** — 根据负载、可用性和账号状态选择上游账号
- **计费** — 基于 `ForwardResult` 中的 token 数自动计费
- **限流** — 按用户/分组维度限流
- **并发控制** — 按账号维度控制并发

### 扩展插件

`ExtensionPlugin` 提供三大基础能力，组合使用可覆盖各种非网关场景：

| 能力 | 方法 | 说明 |
| --- | --- | --- |
| 自定义路由 | `RegisterRoutes(r)` | 注册任意 HTTP API |
| 数据库迁移 | `Migrate()` | 创建插件专属数据表（插件通过 Config 获取 DSN 自行建连） |
| 后台任务 | `BackgroundTasks()` | 声明定时任务，Core 负责调度 |

典型场景：

- **支付接入** — 注册支付路由（创建订单、回调）+ 订单表 + 定时对账任务
- **监控告警** — 注册告警配置 API + 告警规则表 + 定时检查任务
- **数据分析** — 注册报表 API + 聚合表 + 定时聚合任务

### 可选能力

所有插件类型都可以额外实现以下可选接口，Core 通过类型断言自动检测：

| 接口 | 用途 | 典型场景 |
| --- | --- | --- |
| `WebAssetsProvider` | 提供前端静态资源 | 自定义管理页面、嵌入组件 |
| `ConfigWatcher` | 配置热更新 | 不重启改配置 |

### PluginContext

插件在 `Init` 阶段收到 `PluginContext`，提供运行时基础能力：

```go
type PluginContext interface {
    Logger() *slog.Logger    // 结构化日志
    Config() PluginConfig    // 配置读取
}
```

> 数据库连接：Core 通过 Config 传递 DSN（`config.GetString("db_dsn")`），插件自行 `sql.Open` 建连。这样在 gRPC 跨进程模式下也能正常工作。

## 快速开始：网关插件

```go
package main

import (
    "context"
    sdk "github.com/DouDOU-start/airgate-sdk"
    "github.com/DouDOU-start/airgate-sdk/grpc"
)

type MyGateway struct{}

// ---- Plugin 基础接口 ----

func (g *MyGateway) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        ID:      "gateway-myplatform",
        Name:    "My Platform 网关",
        Version: "1.0.0",
        Type:    sdk.PluginTypeGateway,
        AccountTypes: []sdk.AccountType{
            {
                Key:   "apikey",
                Label: "API Key",
                Fields: []sdk.CredentialField{
                    {Key: "api_key", Label: "API Key", Type: "password", Required: true},
                    {Key: "base_url", Label: "API 地址", Type: "text"},
                },
            },
        },
    }
}

func (g *MyGateway) Init(ctx sdk.PluginContext) error { return nil }
func (g *MyGateway) Start(_ context.Context) error   { return nil }
func (g *MyGateway) Stop(_ context.Context) error     { return nil }

// ---- GatewayPlugin 接口 ----

func (g *MyGateway) Platform() string { return "myplatform" }

func (g *MyGateway) Models() []sdk.ModelInfo {
    return []sdk.ModelInfo{
        {ID: "my-model-v1", Name: "My Model V1", MaxTokens: 128000, InputPrice: 1.0, OutputPrice: 3.0},
    }
}

func (g *MyGateway) Routes() []sdk.RouteDefinition {
    return []sdk.RouteDefinition{
        {Method: "POST", Path: "/v1/chat/completions", Description: "Chat API"},
    }
}

func (g *MyGateway) Forward(ctx context.Context, req *sdk.ForwardRequest) (*sdk.ForwardResult, error) {
    // req.Account — Core 已调度好的上游账号
    // req.Body / req.Headers — 原始请求
    // req.Writer — 流式写入 SSE 响应
    return &sdk.ForwardResult{
        StatusCode:   200,
        InputTokens:  100,
        OutputTokens: 50,
        Model:        "my-model-v1",
    }, nil
}

func (g *MyGateway) ValidateAccount(ctx context.Context, credentials map[string]string) error {
    // 用凭证调一次上游接口，验证是否有效
    return nil
}

func (g *MyGateway) QueryQuota(ctx context.Context, credentials map[string]string) (*sdk.QuotaInfo, error) {
    // 查询账号额度（不支持可返回 ErrNotSupported）
    return nil, sdk.ErrNotSupported
}

func (g *MyGateway) HandleWebSocket(ctx context.Context, conn sdk.WebSocketConn) (*sdk.ForwardResult, error) {
    // 处理 WebSocket 双向通信（不需要可返回 ErrNotSupported）
    return nil, sdk.ErrNotSupported
}

// ---- 启动 ----

func main() {
    grpc.Serve(&MyGateway{})  // 自动识别插件类型，注册 gRPC 服务
}
```

构建：

```bash
go build -o my-plugin .
```

## 本地开发验证

SDK 提供 `devserver` 包，模拟 AirGate Core 的最小行为，用于插件端到端验证。插件无需部署 Core 即可本地测试账号管理、HTTP/SSE 转发和 WebSocket 通信。

### 最小用法

```go
package main

import (
    "log"
    "github.com/DouDOU-start/airgate-sdk/devserver"
)

func main() {
    gw := &MyGateway{}
    if err := devserver.Run(devserver.Config{
        Plugin: gw,
    }); err != nil {
        log.Fatal(err)
    }
}
```

启动后访问 `http://localhost:18080` 即可看到管理页面，支持：

- 账号 CRUD（JSON 文件持久化）
- 代理转发（根据 `Routes()` 自动注册路径）
- WebSocket 升级
- 插件前端资源服务（如果实现了 `WebAssetsProvider`）

### Config 选项

```go
type Config struct {
    Plugin      sdk.GatewayPlugin                              // 必填：网关插件实例
    Addr        string                                         // 监听地址，默认 ":18080"
    DataDir     string                                         // 数据目录，默认 "./devdata"
    ExtraRoutes func(mux *http.ServeMux, store *AccountStore)  // 插件自定义路由
}
```

命令行参数 `-addr`、`-data`、`-log` 可覆盖 Config 中的默认值。

### 插件自定义路由

插件如有特殊的开发时路由（如 OAuth 授权流程），通过 `ExtraRoutes` 注入：

```go
devserver.Run(devserver.Config{
    Plugin: gw,
    ExtraRoutes: func(mux *http.ServeMux, store *devserver.AccountStore) {
        // 注册插件特有的 OAuth 路由
        h := &gateway.OAuthDevHandler{Gateway: gw, Store: store}
        h.RegisterRoutes(mux)
    },
})
```

### 内置路由

| 路径 | 说明 |
| --- | --- |
| `GET /` | 管理 UI（内嵌 HTML） |
| `GET /api/plugin/info` | 插件元信息（Info() JSON） |
| `GET/POST/PUT/DELETE /api/accounts` | 账号 CRUD |
| `/plugin-assets/*` | 插件前端资源（如有） |
| `/{routes-prefix}/*` | 代理转发（从 Routes() 提取前缀） |

## 前端集成

插件的前端能力分两种：**独立页面**和**组件嵌入**，两者通过同一套资源机制（`WebAssetsProvider`）提供 JS/CSS 文件。

| 模式 | 说明 | 谁控制布局 |
| --- | --- | --- |
| 独立页面 (`FrontendPages`) | 插件拥有完整页面，Core 分配路由和导航入口 | 插件 |
| 组件嵌入 (`FrontendWidgets`) | 插件提供组件片段，嵌入 Core 已有页面的指定插槽 | Core |

### 独立页面

在 `PluginInfo` 中声明，Core 据此注册前端路由并渲染侧边栏导航：

```go
FrontendPages: []sdk.FrontendPage{
    {Path: "/dashboard", Title: "仪表盘", Icon: "chart", Description: "使用统计"},
    {Path: "/settings", Title: "高级设置", Icon: "settings", Description: "插件配置"},
},
```

### 组件嵌入

Core 页面预留了**插槽（Slot）**，插件声明往哪个插槽注入自定义组件。SDK 用常量定义所有合法插槽：

```go
const (
    SlotAccountForm   = "account-form"    // 添加/编辑账号表单
    SlotAccountDetail = "account-detail"  // 账号详情/用量展示
)
```

在 `PluginInfo` 中声明要嵌入的组件：

```go
FrontendWidgets: []sdk.FrontendWidget{
    {Slot: sdk.SlotAccountForm, EntryFile: "widgets/account-form.js", Title: "账号添加"},
    {Slot: sdk.SlotAccountDetail, EntryFile: "widgets/account-detail.js", Title: "用量详情"},
},
```

Core 拥有宿主页面、弹窗骨架、插槽位置、加载/降级生命周期；插件只负责填充自己声明的 Slot 内容。账号管理页的实际运行顺序应当是：

```text
渲染账号管理页
  → Core 渲染页面与 ModalSection 外壳
  → 根据当前插件，加载其声明的 Slot Widget
  → Widget 存在：插件接管 slot 内内容
  → Widget 缺失但 schema 存在：Core 回退到 schema 驱动表单
  → 两者都缺失：Core 显示 placeholder
```

> `AccountTypes` 是元数据声明（告诉 Core 这个插件支持哪些账号类型的 key 和 label），用于筛选和列表展示；真正的交互形态由 Widget 决定。插件如果不提供 Widget，Core 只能根据 schema 做保底表单，不能推断插件特有的 OAuth 引导、多步骤辅助面板等行为。

#### 宿主边界（必须遵守）

- **Core 拥有宿主骨架**：路由注册、导航入口、页面标题、弹窗外壳、区块层级、Footer 动作区、加载态与空态都由 Core 决定。
- **Widget 只拥有 slot 内容**：嵌入型组件只能渲染插槽内部，不应假设自己控制整个页面、整个弹窗或宿主外层间距。
- **插件前端契约保持最小化**：前端模块只暴露 `routes`、`menuItems`、`accountForm` 等显式入口；Core 负责加载、挂载、销毁与错误隔离。
- **独立页面与嵌入组件分治**：`FrontendPages` 可以拥有页面主体内容；`FrontendWidgets` 只能拥有指定插槽。两者都必须服从 Core 的路由与运行时装配。

### 静态资源

无论独立页面还是组件嵌入，前端资源都通过 `WebAssetsProvider` 统一提供：

```go
import "embed"

//go:embed web/dist/*
var webAssets embed.FS

func (g *MyGateway) GetWebAssets() map[string][]byte {
    files := make(map[string][]byte)
    // 遍历 embed.FS 构建文件映射
    // files["index.js"] = ...
    // files["widgets/account-form.js"] = ...
    return files
}
```

### 构建与加载流程

```text
构建阶段：
  web/src → npm run build → web/dist → go:embed → 插件二进制

运行阶段：
  Core 启动插件
    → Info() 获取 FrontendPages + FrontendWidgets
    → GetWebAssets() 提取静态资源到本地
    → 用户访问时按需加载
```

### 主题系统 `@airgate/theme`

SDK 提供统一的前端主题包 `@airgate/theme`（位于 `frontend/` 目录），作为 Core 和所有插件的颜色/样式唯一来源，支持亮色/暗色主题切换。

#### 安装

```json
// 插件 package.json
{
  "dependencies": {
    "@airgate/theme": "file:../../airgate-sdk/frontend"
  }
}
```

#### 在插件中使用

```typescript
import { cssVar, themeStyle } from '@airgate/theme';

// 单个 token — 返回带 fallback 的 CSS var() 引用
color: cssVar('text')              // → 'var(--ag-text, #e8ecf4)'
backgroundColor: cssVar('bgSurface')  // → 'var(--ag-bg-surface, #1c2237)'
borderRadius: cssVar('radiusMd')      // → 'var(--ag-radius-md, 10px)'

// 批量生成 style 对象
const style = themeStyle({
  color: 'text',
  backgroundColor: 'bgSurface',
  borderColor: 'glassBorder',
});
```

`cssVar()` 的参数有完整的 TypeScript 类型提示，IDE 会自动补全所有可用 token。

#### 可用 token

**主题 token**（随亮暗色切换变化）：

| 分类 | Token | CSS 变量 |
| --- | --- | --- |
| 主色 | `primary` `primaryHover` `primarySubtle` `primaryGlow` | `--ag-primary` 等 |
| 语义色 | `success` `danger` `warning` `info` + 各自 `Subtle` 变体 | `--ag-success` 等 |
| 背景 | `bgDeep` `bg` `bgElevated` `bgSurface` `bgHover` `bgActive` | `--ag-bg-deep` 等 |
| 边框 | `border` `borderSubtle` `borderFocus` | `--ag-border` 等 |
| 文字 | `text` `textSecondary` `textTertiary` `textInverse` | `--ag-text` 等 |
| 玻璃态 | `glass` `glassBorder` | `--ag-glass` 等 |
| 阴影 | `shadowSm` `shadowMd` `shadowLg` `shadowGlow` | `--ag-shadow-sm` 等 |

**静态 token**（不随主题变化）：

| Token | CSS 变量 | 默认值 |
| --- | --- | --- |
| `radiusSm` `radiusMd` `radiusLg` `radiusXl` | `--ag-radius-*` | 6/10/14/20px |
| `fontSans` `fontMono` | `--ag-font-*` | DM Sans / JetBrains Mono |
| `transition` `transitionSlow` | `--ag-transition*` | 200ms / 400ms |
| `sidebarWidth` `sidebarCollapsed` `topbarHeight` | `--ag-sidebar-*` 等 | 260/72/64px |

#### 工作原理

1. Core 启动时由 `ThemeProvider` 调用 `injectThemeStyle()`，在 `<head>` 中注入包含亮暗两套值的 CSS 变量
2. `<html data-theme="dark|light">` 控制当前激活的主题
3. 插件通过 `cssVar()` 引用变量，自带 fallback 值确保独立测试也能正常渲染
4. 用户切换主题时，所有使用 CSS 变量的组件（Core + 插件）自动跟随

> 插件也可以继续手写 `var(--ag-primary, #3b82f6)` 格式，`cssVar()` 只是提供类型安全和自动 fallback 的便捷方式。

#### 插件样式基础层 `@airgate/theme/plugin`

对于嵌入 Core 的插件前端，推荐直接复用 `@airgate/theme/plugin` 提供的最小基础层，而不是每个插件都重新实现一套 scoped theme、样式注入和基础 primitives。

```ts
import {
  ensurePluginStyleFoundation,
  useScopedPluginTheme,
  createPluginTailwindConfig,
  Field,
  TextInput,
  Button,
} from '@airgate/theme/plugin';
```

它解决的是插件开发里反复出现的四类问题：

- **主题注入**：`ensurePluginStyleFoundation()` 负责把 token 变量和 plugin foundation CSS 注入到宿主文档。
- **亮暗跟随**：`useScopedPluginTheme()` 让 widget 自动跟随宿主 `data-theme` 变化，而不是自己维护一份全局主题状态。
- **Tailwind 桥接**：`createPluginTailwindConfig()` 为插件生成带 scope、prefix、token bridge 的配置，避免污染宿主样式。
- **基础组件**：`Field` / `TextInput` / `TextArea` / `Button` / `Badge` / `Section` 等 primitives 统一使用共享 token，减少每个插件重复造轮子。

#### 插件前端样式规范

- **唯一 token 源**：颜色、阴影、圆角、字体统一来自 `@airgate/theme`，不要在插件里私自定义第二套语义 token。
- **必须做作用域隔离**：插件根节点必须使用自己的 scope selector；Tailwind 必须配置 `important: scopeSelector` 与独立前缀，禁止污染宿主全局选择器。
- **优先复用共享 primitives**：输入框、按钮、卡片、状态徽标、表单分区等通用元素优先复用 `@airgate/theme/plugin`，只有确实是领域组件时再在插件内部组合。
- **不覆盖宿主骨架**：插件不要重写 Core Modal、Page、Sidebar、Header 的全局样式；slot 内部允许表达品牌和交互，但边界止于 slot。
- **亮暗主题必须天然可用**：不要写死浅色背景或深色文字；所有前景/背景/边框都应通过 token 驱动，确保随宿主主题自动切换。
- **把 Tailwind 当 recipe，不当真相**：插件可以用 Tailwind 组织布局，但最终视觉契约仍应落在共享 token 和 foundation class 上。

#### 插件前端 PR Review Checklist

- [ ] 颜色、阴影、圆角、字体是否全部来自 `@airgate/theme` token
- [ ] Widget 是否只影响自己的 scope，而没有污染宿主全局样式
- [ ] 是否优先复用了 `@airgate/theme/plugin` primitives，而不是重复造通用输入/按钮/卡片
- [ ] 亮色 / 暗色下是否都可读、可用，且能跟随 Core 切换
- [ ] 插件是否只接管 slot 内部内容，而没有假设自己拥有整个页面或弹窗骨架
- [ ] 如果插件未提供 Widget，Core 的 schema fallback 是否仍然可以正常工作

## 目录结构

### SDK 仓库

```text
airgate-sdk/
├── plugin.go          # Plugin 基础接口、PluginInfo、可选接口
├── gateway.go         # GatewayPlugin 接口
├── models.go          # 共享类型：Account, ModelInfo, ForwardRequest/Result 等
├── config.go          # PluginConfig 配置读取接口
├── extension.go       # ExtensionPlugin 接口
├── websocket.go       # WebSocketConn 类型定义
├── grpc/              # gRPC 桥接层（go-plugin 适配）
│   ├── go_plugin.go   # Serve() 入口
│   ├── common.go      # pluginBase 公共基类
│   └── *_client.go    # 各插件类型的 gRPC 客户端/服务端
├── devserver/         # 插件开发服务器（本地验证用）
│   ├── server.go      # Config + Run() 入口
│   ├── accounts.go    # 账号 CRUD（JSON 文件存储）
│   ├── proxy.go       # HTTP/SSE/WebSocket 代理
│   ├── context.go     # devPluginContext 实现
│   ├── logger.go      # 多级日志（console INFO + file DEBUG）
│   └── static/        # 内嵌管理 UI
├── shared/            # 握手配置
└── proto/             # protobuf 定义
```

### 推荐的插件项目结构

```text
my-plugin/
├── cmd/
│   ├── server/main.go         # 生产入口（gRPC 模式）
│   └── devserver/main.go      # 开发入口（devserver.Run，约 20 行）
├── internal/
│   └── gateway/
│       ├── gateway.go         # 接口实现
│       ├── metadata.go        # 元信息（推荐单源维护）
│       └── assets.go          # WebAssetsProvider + go:embed（可选）
├── web/                        # 前端源码（可选）
│   ├── src/
│   │   ├── pages/             # 独立页面
│   │   └── widgets/           # 嵌入组件
│   ├── package.json
│   └── dist/                   # 构建产物
├── go.mod
└── plugin.yaml                 # 由代码生成的分发文件
```

## 打包与发布

### plugin.yaml 示例

```yaml
id: gateway-myplatform
name: My Platform 网关
version: 1.0.0
type: gateway
min_core_version: "1.0.0"
platform: myplatform
routes:
  - method: POST
    path: /v1/chat/completions
models:
  - id: my-model-v1
    name: My Model V1
    max_tokens: 128000
    input_price: 1.0
    output_price: 3.0
account_types:
  - key: apikey
    label: API Key
    fields:
      - key: api_key
        label: API Key
        type: password
        required: true
      - key: base_url
        label: API 地址
        type: text
```

### 打包格式

```text
my-plugin.tar.gz
├── my-plugin           # 插件二进制（前端资源已通过 go:embed 打入）
└── plugin.yaml         # 分发元信息
```

### 发布检查清单

- [ ] `go test ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] 重新生成最新 `plugin.yaml`
- [ ] 构建目标平台二进制
- [ ] 如有前端，构建前端资源
- [ ] 打包并验证完整性

## 开发工具

```bash
make lint    # 代码检查
make fmt     # 代码格式化
make test    # 运行测试
make proto   # 重新生成 protobuf 代码
```

## 给 Core 开发者

Core 启动插件后的消费流程：

```text
启动插件进程（go-plugin）
  → Info()      获取元信息（ID、类型、账号格式、前端声明）
  → Platform()  获取业务平台键
  → Models()    获取模型列表（缓存，用于计费）
  → Routes()    获取路由声明（注册到 HTTP 网关）
  → GetWebAssets()  提取前端资源（如有）

添加/导入账号时：
  → ValidateAccount(ctx, credentials)  调用插件验证凭证有效性

定时巡检（Core 后台任务）：
  → QueryQuota(ctx, credentials)  查询账号额度，结果存入 accounts 表
  → 额度不足的账号自动降低优先级或暂停调度

HTTP 请求到达时：
  → Core 鉴权、限流
  → Core 调度账号（跳过 rate_limited / disabled 状态的账号）
  → Forward(ctx, req)  调用插件转发
  → Core 记录用量（基于 ForwardResult.token 数）
  → Core 处理账号状态反馈（基于 ForwardResult.AccountStatus）
    → rate_limited → 暂停调度，RetryAfter 后恢复
    → disabled     → 标记封号，停止调度
    → expired      → 标记过期，停止调度

WebSocket 升级请求时：
  → Core 鉴权、调度账号
  → HandleWebSocket(ctx, conn)  交给插件处理双向通信
  → 连接结束后，同样通过 ForwardResult 反馈用量和账号状态
```

Core 必须遵守的约定：

- 以 `PluginInfo.ID` 作为运行时键（API 路径、资源挂载、缓存）
- 以 `Platform()` 作为业务键（账号关联、调度、计费）
- 以插件运行时返回的元信息为准，**不依赖 `plugin.yaml` 做运行时决策**
- 添加账号时调用 `ValidateAccount` 验证凭证，验证失败拒绝保存
- 账号管理 UI 统一由插件 `FrontendWidgets` 渲染，Core 不做默认表单生成

## License

MIT
