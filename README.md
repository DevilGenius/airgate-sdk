<div align="center">
  <h1>AirGate SDK</h1>

  <p><strong>AirGate 插件生态的接口契约与开发套件</strong></p>

  <p>
    <a href="https://github.com/DouDOU-start/airgate-sdk/releases"><img src="https://img.shields.io/github/v/release/DouDOU-start/airgate-sdk?style=flat-square" alt="release" /></a>
    <a href="https://pkg.go.dev/github.com/DouDOU-start/airgate-sdk"><img src="https://img.shields.io/badge/pkg.go.dev-reference-007d9c?style=flat-square&logo=go" alt="godoc" /></a>
    <a href="https://github.com/DouDOU-start/airgate-sdk/blob/master/LICENSE"><img src="https://img.shields.io/github/license/DouDOU-start/airgate-sdk?style=flat-square" alt="license" /></a>
    <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="go" />
    <img src="https://img.shields.io/badge/gRPC-go--plugin-4c1?style=flat-square" alt="grpc" />
  </p>
</div>

---

AirGate SDK 是 [airgate-core](https://github.com/DouDOU-start/airgate-core) 插件生态的**协议层**，定义了插件与 core 之间的全部边界：接口契约、共享类型、gRPC 桥接、本地开发服务器和统一前端主题。

- **Core** = 用户、账号、调度、计费、限流、订阅、管理后台 —— 平台无关的通用能力
- **SDK**（本仓库）= 插件如何被装载、被调度、被回调的全部规则
- **Plugin** = 依赖 SDK 实现接口的独立 Go 进程，提供具体平台的能力

同一份契约在 core 和插件两端使用，保证升级不会偏离。底层走 [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) 的 gRPC 模式，每个插件运行在自己的子进程里，崩溃不影响 core 与其他插件。

```bash
go get github.com/DouDOU-start/airgate-sdk@latest
```

## ✨ 核心特性

- **🔌 三类插件模型** — `GatewayPlugin`（upstream 适配）/ `ExtensionPlugin`（路由 + 后台任务）/ `MiddlewarePlugin`（forward 路径拦截层，旁路观察 / 审计 / 脱敏），详见 [ADR-0001](docs/adr/0001-plugin-capability-and-isolation-model.md)
- **🛂 能力模型** — 插件显式声明所需的 HostService / Middleware capability，Core 侧 gRPC interceptor 按"插件类型 → 允许集合"做最小权限校验（SDK `0.3.0` 起强制）
- **🔁 反向通道 HostService** — 插件通过 `HostAware` 可选接口拿到 `Host`，直接回调 core 能力（选号 / 探测 / 列分组），走 hashicorp/go-plugin GRPCBroker 子进程隧道，无需 admin HTTP + API key
- **🧩 最小契约** — 插件只需声明账号格式 / 模型 / 路由，并实现 `Forward`，core 自动接管账号管理、调度、计费、限流
- **🎨 前端集成** — 独立页面 (`FrontendPages`) + 组件嵌入 (`FrontendWidgets`)，通过 `WebAssetsProvider` 统一打包到二进制
- **🎭 统一主题** — 内置 `@airgate/theme` 包提供共享 token、亮暗切换、Tailwind 桥接和插件作用域隔离
- **🛠 本地开发服务器** — `devserver` 包模拟 core 行为，**插件无需部署 core 即可端到端测试**账号、HTTP/SSE 转发、WebSocket
- **📦 进程隔离** — 基于 hashicorp/go-plugin gRPC 模式，崩溃隔离、独立热更、独立发版

## 🧩 三类插件

| 类型 | 接口 | 定位 | 参考实现 |
|---|---|---|---|
| **网关插件** | `GatewayPlugin` | AI API 代理。声明模型/路由/账号格式 + 实现 `Forward`，core 自动调度 + 计费 + 限流 | [airgate-openai](https://github.com/DouDOU-start/airgate-openai) |
| **扩展插件** | `ExtensionPlugin` | 一切非网关场景。提供路由注册、数据库迁移、后台任务三大基础能力 | [airgate-epay](https://github.com/DouDOU-start/airgate-epay) · [airgate-health](https://github.com/DouDOU-start/airgate-health) |
| **中间件插件** | `MiddlewarePlugin` | Forward 路径的旁路拦截层：请求/响应记录、审计、脱敏、流量采样、合规标签注入 | （示例计划：`airgate-audit`） |

三种角色的边界是**互斥**的：gateway **替代** upstream；extension **并行** 扩展（独立路由表 / 定时任务）；middleware **拦截** 每次 forward 的前后事件，**永远不能 block 生产流量**（详见 Decision 2 的失败语义）。

### 网关插件 `GatewayPlugin`

| 方法 | 职责 |
|---|---|
| `Platform()` | 返回业务平台键（如 `"openai"`） |
| `Models()` | 声明支持的模型 + 单价（core 用于计费） |
| `Routes()` | 声明 API 端点（如 `POST /v1/chat/completions`），core 自动注册 |
| `Forward(ctx, req)` | 拿到 core 调度好的账号，转发请求并返回 token 用量 + 账号状态反馈 |
| `ValidateAccount(ctx, cred)` | 添加/导入账号时由 core 调用验证凭证 |
| `QueryQuota(ctx, cred)` | core 定时巡检账号额度 |
| `HandleWebSocket(ctx, conn)` | 处理 WebSocket 双向通信（如 Realtime API） |

### 扩展插件 `ExtensionPlugin`

| 能力 | 方法 | 说明 |
|---|---|---|
| 自定义路由 | `RegisterRoutes(r)` | 注册任意 HTTP API |
| 数据库迁移 | `Migrate()` | 创建插件专属表（通过 Config 获取 DSN 自行建连） |
| 后台任务 | `BackgroundTasks()` | 声明定时任务，core 负责调度 |

### 中间件插件 `MiddlewarePlugin`

| 方法 | 职责 |
|---|---|
| `OnForwardBegin(ctx, req)` | 选完账号 / 还没调 upstream 之前触发。返回 `Decision` 可放行 / 拒绝 / 追加 header |
| `OnForwardEnd(ctx, evt)` | upstream 返回之后 / 写 usage_log 之前触发。拿到完整的请求 + 响应元数据 |

**关键设计约定**（详见 [ADR-0001 Decision 2/3](docs/adr/0001-plugin-capability-and-isolation-model.md)）：

- **失败即跳过**：`OnForwardBegin` / `OnForwardEnd` 返回 `error` 只 log warn，不阻塞主流程。唯一例外是 `OnForwardBegin` 显式返回 `DecisionDeny`
- **LIFO 链顺序**：多个 middleware 按 `PluginInfo.Priority` 升序调 Begin、**降序**调 End（像 middleware stack 展开）
- **Payload 两段式**：默认只传元数据（`request_id` / `user_id` / `group_id` / `account_id` / `platform` / `model` / 用量）；声明 `CapabilityMiddlewareReadBody` 的插件额外收到 `request_body` / `response_body` + headers
- **流式响应的 body 摘要**：End 阶段流式响应的 `ResponseBody` 只给首次非空 chunk 拼装的摘要，完整流式内容留给未来的 `OnStreamChunk`（ADR-0002）
- **跨 hook 上下文**：`Metadata` 字段是所有 middleware 共享的 KV bag，从 Begin 贯穿到 End

### 可选能力

所有插件类型都可额外实现以下接口，core 通过类型断言自动检测：

| 接口 | 用途 |
|---|---|
| `WebAssetsProvider` | 提供前端静态资源（独立页面 / 嵌入组件） |
| `ConfigWatcher` | 配置热更新 |
| `HealthChecker` | 自定义健康检查逻辑 |
| `RequestHandler` | 处理 `/api/v1/admin/plugins/:name/rpc/*` 透传请求 |
| `HostAware` | 通过 `ctx.(sdk.HostAware).Host()` 拿到反向调用 core 的 `Host` 客户端 |

## 🛂 能力模型（Capability）

`SDKVersion = "0.3.0"` 起，插件调用 `HostService` 或使用 middleware 特殊 payload 必须**显式声明** capability，否则 Core 的 gRPC interceptor 会返回 `PermissionDenied`。

```go
func (p *MyExtension) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        ID:   "ext-monitor",
        Type: sdk.PluginTypeExtension,
        Capabilities: []string{
            sdk.CapabilityHostListGroups,
            sdk.CapabilityHostProbeForward,
            sdk.CapabilityHostReportAccountResult,
        },
        // ...
    }
}
```

当前 capability 清单（Core 按"插件类型 → 允许集合"做交集后得到有效权限）：

| Capability | 用途 | 允许的插件类型 |
|---|---|---|
| `host.list_groups` | `Host.ListGroups()` 列出分组 | `extension`, `middleware` |
| `host.select_account` | `Host.SelectAccount()` 走真实调度选号 | `extension` |
| `host.probe_forward` | `Host.ProbeForward()` 黑盒探测 | `extension`（probe 子类） |
| `host.report_account_result` | `Host.ReportAccountResult()` 反馈状态机 | `extension`（probe 子类） |
| `middleware.read_body` | middleware 接收 `request_body` / `response_body` | `middleware` |

**向后兼容**：SDK `<= 0.2.x` 的旧插件不声明 `Capabilities` 仍然可以跑（通过 `sdk_version` 字段豁免），但管理后台会显示"兼容模式"警告。`>= 0.3.x` 起强制校验。

**命名规范**：`<domain>.<action>`。新增 capability 必须在 ADR 里说明语义 / owner / 允许的插件类型。

## 🔁 反向通道 HostService

过去插件要回调 core（列分组、选号、探测）只能走 admin HTTP API + admin key —— 管理员要手工生成 key、插件要拼 URL 签 Bearer、同机两个进程也被迫走完整 HTTP+JSON 栈。`HostService` 通过 hashicorp/go-plugin 的 `GRPCBroker` 为每个插件子进程架起一条**反向 gRPC stream**，子进程隧道天然互信。

```go
type MyExtension struct {
    host sdk.Host
}

func (p *MyExtension) Init(ctx sdk.PluginContext) error {
    // HostAware 是可选接口：旧版 Core / devserver / 测试 mock 都可以不实现
    if h, ok := ctx.(sdk.HostAware); ok {
        p.host = h.Host() // 仍可能为 nil，调用方需 nil-check
    }
    return nil
}

func (p *MyExtension) probe(ctx context.Context) {
    if p.host == nil { return }

    groups, err := p.host.ListGroups(ctx)
    if err != nil { /* ... */ }

    for _, g := range groups {
        result, _ := p.host.ProbeForward(ctx, sdk.HostProbeForwardRequest{GroupID: g.ID})
        p.host.ReportAccountResult(ctx, result.AccountID, result.Success, result.ErrorMsg)
    }
}
```

当前 v1 暴露的 4 个 RPC（克制暴露面，等真实需求再加）：

| RPC | 语义 |
|---|---|
| `SelectAccount` | 走和真实用户请求完全相同的调度路径选号 |
| `ProbeForward` | 黑盒探测 chat completion：跳过 `usage_log` / 余额扣款，但仍触发 `ReportResult` |
| `ListGroups` | 列出所有分组（id / name / platform / 是否独占 / 倍率） |
| `ReportAccountResult` | 把账号调用结果反馈给 scheduler 的失败计数器 / 状态机 |

**设计原则**（详见 [ADR-0001 §2](docs/adr/0001-plugin-capability-and-isolation-model.md)）：
- **只加字段不删字段**（protobuf 天然向前兼容）
- **加新 RPC 用新 rpc name**，不 hijack 旧的
- **新能力必须伴随新 capability flag**，旧插件不声明就不启用
- **Core 是 trust root**：HostService 所有输入做参数校验，credentials / password_hash / admin key 等敏感字段永远不通过 RPC 流向插件

## 🛠 技术栈

| 层 | 技术 |
|---|---|
| 语言 | Go 1.25 |
| 插件协议 | hashicorp/go-plugin (gRPC + protobuf) |
| 序列化 | protobuf v3 |
| 前端主题 | TypeScript · CSS Variables · Tailwind 桥接 |
| 开发服务器 | net/http + 内嵌 HTML 管理 UI |

## 🚀 快速开始

### 1. 编写一个最小网关插件

```go
package main

import (
    "context"
    sdk "github.com/DouDOU-start/airgate-sdk"
    "github.com/DouDOU-start/airgate-sdk/grpc"
)

type MyGateway struct{}

func (g *MyGateway) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        ID:      "gateway-myplatform",
        Name:    "My Platform 网关",
        Version: "1.0.0",
        Type:    sdk.PluginTypeGateway,
        AccountTypes: []sdk.AccountType{{
            Key:   "apikey",
            Label: "API Key",
            Fields: []sdk.CredentialField{
                {Key: "api_key", Label: "API Key", Type: "password", Required: true},
            },
        }},
    }
}

func (g *MyGateway) Init(ctx sdk.PluginContext) error { return nil }
func (g *MyGateway) Start(_ context.Context) error    { return nil }
func (g *MyGateway) Stop(_ context.Context) error     { return nil }

func (g *MyGateway) Platform() string { return "myplatform" }

func (g *MyGateway) Models() []sdk.ModelInfo {
    return []sdk.ModelInfo{{
        ID: "my-model-v1", Name: "My Model V1",
        ContextWindow: 128000, MaxOutputTokens: 16384,
        InputPrice: 1.0, OutputPrice: 3.0,
    }}
}

func (g *MyGateway) Routes() []sdk.RouteDefinition {
    return []sdk.RouteDefinition{
        {Method: "POST", Path: "/v1/chat/completions"},
    }
}

func (g *MyGateway) Forward(ctx context.Context, req *sdk.ForwardRequest) (*sdk.ForwardResult, error) {
    // req.Account — Core 已调度好的账号
    // req.Body / req.Headers — 原始请求
    // req.Writer — 流式写入 SSE
    return &sdk.ForwardResult{
        StatusCode:   200,
        InputTokens:  100, OutputTokens: 50,
        InputCost: 0.0001, OutputCost: 0.00015,
        Model: "my-model-v1",
    }, nil
}

func (g *MyGateway) ValidateAccount(ctx context.Context, cred map[string]string) error { return nil }
func (g *MyGateway) QueryQuota(ctx context.Context, cred map[string]string) (*sdk.QuotaInfo, error) {
    return nil, sdk.ErrNotSupported
}
func (g *MyGateway) HandleWebSocket(ctx context.Context, conn sdk.WebSocketConn) (*sdk.ForwardResult, error) {
    return nil, sdk.ErrNotSupported
}

func main() { grpc.Serve(&MyGateway{}) }
```

### 2. 本地开发验证（无需部署 core）

```go
package main

import (
    "log"
    "github.com/DouDOU-start/airgate-sdk/devserver"
)

func main() {
    if err := devserver.Run(devserver.Config{Plugin: &MyGateway{}}); err != nil {
        log.Fatal(err)
    }
}
```

启动后访问 `http://localhost:18080`，即可看到管理 UI，支持账号 CRUD、HTTP/SSE 代理转发、WebSocket 升级、插件前端资源服务。命令行参数 `-addr` / `-data` / `-log` 可覆盖默认配置。

### 3. 构建与发布

```bash
go build -o my-plugin .
# 打包：my-plugin.tar.gz 包含二进制 + plugin.yaml
```

完整范例（含 Makefile / release workflow / 前端嵌入）见 [airgate-openai](https://github.com/DouDOU-start/airgate-openai)。

## 🏗 架构

```text
┌─────────────────────── Core ────────────────────────┐
│  账号管理 / 调度 / 计费 / 限流 / 订阅 / 管理后台      │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ PluginService│  │GatewayService│  │ ExtService │ │   Core → Plugin
│  │ Middleware-  │  │              │  │            │ │
│  │ Service      │  │              │  │            │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
│         ▲                                            │
│         │ HostService（反向 stream，经 GRPCBroker）  │   Plugin → Core
└─────────┼────────────────────────────────────────────┘
          │
  ┌───────┴────────────────────────────────────────┐
  │        Plugin subprocess (hashicorp/go-plugin)  │
  │                                                 │
  │  GatewayPlugin / ExtensionPlugin / MiddlewarePl │
  │  Capabilities: [host.list_groups, ...]          │
  └─────────────────────────────────────────────────┘
```

**请求生命周期（含 middleware chain）**：

```text
用户请求
  │
  ▼
Core 鉴权 + 限流 + 选账号
  │
  ▼
Middleware.OnForwardBegin  (按 Priority 升序依次调)
  │ ├─ Decision=Allow  → 继续
  │ ├─ Decision=Mutate → 追加 header 继续
  │ └─ Decision=Deny   → 直接返回给用户
  ▼
Gateway.Forward()  ──►  上游 AI API
  │
  ▼
Middleware.OnForwardEnd    (按 Priority 降序依次调，LIFO)
  │  拿到完整 metadata + 按需 body
  ▼
Core 写 usage_log / 计费 / 账号状态处置
```

**反向调用（插件 → Core）**：

```text
Plugin.probe()
  └─ ctx.(HostAware).Host().ListGroups(ctx)
        │
        │  gRPC stream  (GRPCBroker 子进程隧道，无需 admin key)
        ▼
  Core: HostService interceptor
        │
        │  检查 plugin capability set
        │  未声明 → PermissionDenied
        ▼
  Core: groupRepo.List()
```

**账号模型**：Core 用一张 `accounts` 表存所有平台账号，靠 `platform` + `type` 区分。SDK `Account` 是 core 传给插件的**最小视图**，只包含 `ID / Name / Platform / Type / Credentials / ProxyURL` —— 调度和计费参数全部留在 core。

## 🎨 前端集成

插件的前端能力分两种，通过同一套 `WebAssetsProvider` 资源机制提供：

| 模式 | 说明 | 谁控制布局 |
|---|---|---|
| **独立页面** `FrontendPages` | 插件拥有完整页面，core 分配路由和导航入口 | 插件 |
| **组件嵌入** `FrontendWidgets` | 插件提供组件片段，嵌入 core 已有页面的指定 Slot | Core |

```go
// 独立页面
FrontendPages: []sdk.FrontendPage{
    {Path: "/dashboard", Title: "仪表盘", Icon: "chart"},
},

// 嵌入到 core 账号管理页的指定插槽
FrontendWidgets: []sdk.FrontendWidget{
    {Slot: sdk.SlotAccountForm,   EntryFile: "widgets/account-form.js"},
    {Slot: sdk.SlotAccountDetail, EntryFile: "widgets/account-detail.js"},
},
```

**宿主边界**：Core 拥有路由、导航、弹窗骨架、Slot 位置和生命周期；Widget 只渲染 slot 内部内容，不假设控制整个页面。详见 [docs/plugin-style-guide.md](docs/plugin-style-guide.md)。

## 🎭 主题系统 `@airgate/theme`

SDK 在 `frontend/` 目录提供统一的前端主题包，作为 core 和所有插件的颜色/样式**唯一来源**，支持亮暗切换。

```json
// 插件 package.json
{ "dependencies": { "@airgate/theme": "file:../../airgate-sdk/frontend" } }
```

```typescript
import { cssVar, themeStyle } from '@airgate/theme';

color: cssVar('text')                  // → 'var(--ag-text, #e8ecf4)'
backgroundColor: cssVar('bgSurface')   // → 'var(--ag-bg-surface, #1c2237)'
```

`@airgate/theme/plugin` 子包额外提供：`ensurePluginStyleFoundation()` 主题注入、`useScopedPluginTheme()` 亮暗跟随、`createPluginTailwindConfig()` Tailwind 桥接，以及 `Field` / `TextInput` / `Button` 等基础 primitives。

| 规范 | 说明 |
|---|---|
| 唯一 token 源 | 颜色/阴影/圆角/字体统一来自 `@airgate/theme` |
| 作用域隔离 | 插件根节点必须用自己的 scope selector，Tailwind 配 `important` |
| 不覆盖宿主骨架 | 插件不得重写 core Modal / Page / Sidebar 全局样式 |
| 亮暗天然可用 | 不写死颜色，所有前景/背景/边框走 token |

## 📁 项目结构

```text
airgate-sdk/
├── plugin.go              # Plugin 基础接口 + PluginInfo + Capability 常量 + 可选接口
├── gateway.go             # GatewayPlugin 接口
├── extension.go           # ExtensionPlugin 接口
├── middleware.go          # MiddlewarePlugin 接口 + MiddlewareRequest/Event/Decision
├── host.go                # HostService 客户端接口（反向通道）+ HostAware 可选接口
├── models.go              # 共享类型：Account / ForwardRequest / ForwardResult
├── billing.go             # 计费相关类型 + 账号用量视图
├── errors.go              # 标准错误（ErrNotSupported 等）
├── log.go                 # 日志桥接
├── grpc/                  # gRPC 桥接层（hashicorp/go-plugin 适配）
│   ├── go_plugin.go       # Serve() 入口 + GRPCBroker 反向 stream
│   ├── host_client.go     # 插件侧的 HostService 客户端封装
│   ├── middleware_*.go    # MiddlewareService client / server
│   └── *_client.go        # 各插件类型的 client / server
├── devserver/             # 本地开发服务器
│   ├── server.go          # Config + Run() 入口
│   ├── accounts.go        # 账号 CRUD（JSON 文件持久化）
│   ├── proxy.go           # HTTP / SSE / WebSocket 代理
│   └── static/            # 内嵌管理 UI
├── frontend/              # @airgate/theme + @airgate/theme/plugin
├── proto/                 # protobuf 定义（5 个 service: Plugin/Gateway/Extension/Middleware/Host）
└── docs/
    ├── adr/               # 架构决策记录（ADR-0001 起）
    └── plugin-style-guide.md
```

**推荐的插件项目结构**：

```text
my-plugin/
├── backend/
│   ├── main.go                    # gRPC 入口（grpc.Serve(...)）
│   ├── cmd/devserver/main.go      # 开发入口（约 20 行）
│   └── internal/gateway/          # 接口实现
├── web/                           # 前端源码（可选）
│   ├── src/{pages,widgets}/
│   └── dist/                      # 构建产物（go:embed 打入二进制）
├── .github/workflows/             # ci.yml + release.yml
├── Makefile
└── plugin.yaml                    # 由代码生成的分发文件
```

## 📦 打包与发布

`plugin.yaml` 是由插件代码生成的**分发文件**，仅用于安装和市场展示。**运行时真相始终在插件代码里**，core 不依赖 `plugin.yaml` 做运行时决策。

```yaml
id: gateway-myplatform
name: My Platform 网关
version: 1.0.0
type: gateway
min_core_version: "1.0.0"
platform: myplatform
routes:
  - { method: POST, path: /v1/chat/completions }
models:
  - { id: my-model-v1, name: My Model V1, input_price: 1.0, output_price: 3.0 }
account_types:
  - key: apikey
    fields:
      - { key: api_key, label: API Key, type: password, required: true }
```

**打包格式**：

```text
my-plugin.tar.gz
├── my-plugin           # 插件二进制（前端资源已 go:embed 打入）
└── plugin.yaml         # 分发元信息
```

**发布检查清单**：

- [ ] `go test ./...` / `go vet ./...` 通过
- [ ] 重新生成最新 `plugin.yaml`
- [ ] 构建多架构二进制（amd64 / arm64）
- [ ] 如有前端，构建并嵌入 `dist/`
- [ ] 打包并验证完整性

## 🔧 SDK 开发工具

```bash
make lint    # 代码检查
make fmt     # 代码格式化
make test    # 运行测试
make proto   # 重新生成 protobuf 代码
```

## 👀 给 Core 开发者

Core 启动插件后的消费流程：

```text
启动插件进程（go-plugin）
  → 通过 GRPCBroker 注册 HostService 反向 stream（若启用）
  → Info()       获取元信息（ID、类型、Capabilities、账号格式、前端声明）
  → Capability 校验：
        有效集 = Info.Capabilities ∩ 插件类型允许集合
        注册到 HostService interceptor 的 per-plugin context
  → Init(ctx)    注入 config + log_level + host_broker_id
  → Start(ctx)

Gateway 插件专属：
  → Platform() / Models() / Routes() / GetWebAssets()
  → ValidateAccount(ctx, cred)  添加账号时
  → QueryQuota(ctx, cred)       定时巡检

Extension 插件专属：
  → Migrate()
  → GetBackgroundTasks() + 调度器按 Interval 触发 RunBackgroundTask(name)
  → HandleRequest / HandleStreamRequest（/api/v1/admin/plugins/:name/rpc/* 透传）

HTTP 请求到达时（forward 路径）：
  → Core 鉴权 + 限流 + 调度账号
  → Middleware.OnForwardBegin（按 Priority 升序；Deny → 直接拒绝）
  → Gateway.Forward(ctx, req)
  → Middleware.OnForwardEnd（按 Priority 降序，LIFO）
  → Core 写 usage_log + 处置账号状态（rate_limited / disabled / expired）

插件发起反向调用时：
  → HostService gRPC interceptor 从 context 取出该插件的 capability set
  → 未声明 → status.PermissionDenied
  → 放行 → core 业务层处理
```

Core 必须遵守的约定：

- 以 `PluginInfo.ID` 作为运行时键（API 路径、资源挂载、缓存）
- 以 `Platform()` 作为业务键（账号关联、调度、计费）
- 以插件运行时返回的元信息为准，**不依赖 `plugin.yaml` 做运行时决策**
- 添加账号时必须调用 `ValidateAccount`，验证失败拒绝保存
- 账号管理 UI 统一由插件 `FrontendWidgets` 渲染，core 不做默认表单生成
- **middleware 的失败永远不能 block 生产流量**：`OnForwardBegin/End` 返回 error 只 log warn；唯一阻断途径是 `OnForwardBegin` 返回 `Decision{Action: Deny}`
- **capability 校验在 interceptor 层强制**：core 业务代码不应再做 capability 判断（单一真相源）
- 插件**不得**拿到 core 业务数据库的 DSN；core 业务数据一律通过 `HostService` RPC 暴露（详见 [ADR-0001 Decision 1/5](docs/adr/0001-plugin-capability-and-isolation-model.md)）

## 🤝 贡献 / 反馈

- Bug / Feature: [Issues](https://github.com/DouDOU-start/airgate-sdk/issues)
- 主仓库: [airgate-core](https://github.com/DouDOU-start/airgate-core)
- 插件参考实现: [airgate-openai](https://github.com/DouDOU-start/airgate-openai) · [airgate-epay](https://github.com/DouDOU-start/airgate-epay) · [airgate-health](https://github.com/DouDOU-start/airgate-health)

## 📜 License

MIT
