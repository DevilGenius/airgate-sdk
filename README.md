<div align="center">
  <h1>AirGate SDK</h1>

  <p><strong>AirGate 插件生态的公共契约与开发工具包</strong></p>

  <p>
    <a href="https://github.com/DouDOU-start/airgate-sdk/releases"><img src="https://img.shields.io/github/v/release/DouDOU-start/airgate-sdk?style=flat-square" alt="发布版本" /></a>
    <a href="https://pkg.go.dev/github.com/DouDOU-start/airgate-sdk"><img src="https://img.shields.io/badge/pkg.go.dev-reference-007d9c?style=flat-square&logo=go" alt="Go 文档" /></a>
    <a href="https://github.com/DouDOU-start/airgate-sdk/blob/master/LICENSE"><img src="https://img.shields.io/github/license/DouDOU-start/airgate-sdk?style=flat-square" alt="许可证" /></a>
    <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go 版本" />
    <img src="https://img.shields.io/badge/gRPC-go--plugin-4c1?style=flat-square" alt="gRPC 插件协议" />
  </p>
</div>

---

AirGate SDK 是 [AirGate Core](https://github.com/DouDOU-start/airgate-core) 和插件之间的公共契约。它定义插件要实现什么接口、Core 如何启动插件进程、双方如何通过 gRPC 通信，以及插件前端如何复用统一主题和公共组件。

简单理解：

- **Core** 负责用户、账号、调度、限流、插件管理、记录存储和管理后台。
- **SDK** 定义接口、协议、运行时适配、本地开发工具和前端基础包。
- **Plugin** 是独立 Go 进程，依赖 SDK 实现具体平台或扩展能力。

## 安装

```bash
go get github.com/DouDOU-start/airgate-sdk@latest
```

Go 插件通常只需要两个包：

```go
import (
    sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
    runtime "github.com/DouDOU-start/airgate-sdk/runtimego/grpc"
)
```

前端插件正式使用 npm 公共包：

```json
{
  "dependencies": {
    "@doudou-start/airgate-theme": "^1.0.0"
  }
}
```

本地联调 SDK 源码时可以临时改为：

```json
{
  "dependencies": {
    "@doudou-start/airgate-theme": "file:../../airgate-sdk/theme"
  }
}
```

## 仓库结构

| 目录 | 用途 | 谁会用 |
|---|---|---|
| `sdkgo/` | Go 插件接口、共享类型、Capability、Host 调用类型 | 插件作者 |
| `protocol/proto/` | `airgate.plugin.v1` protobuf 协议和生成代码 | Core / runtime |
| `runtimego/grpc/` | hashicorp/go-plugin、gRPC bridge、proto 转换、Core 反向调用通道 | 插件入口 / Core 加载器 |
| `devkit/devserver/` | 本地开发服务器，无需启动完整 Core 即可调试插件 | 插件作者 |
| `theme/` | `@doudou-start/airgate-theme`：主题 token、样式隔离、Tailwind helper、公共组件 | 插件前端 |
| `docs/` | 设计边界和前端样式规范 | 维护者 |

普通插件业务代码不直接依赖 `protocol/proto`。

## 插件类型

| 类型 | 接口 | 用途 |
|---|---|---|
| `gateway` | `sdk.GatewayPlugin` | 接入 AI 平台，声明模型、路由、账号字段，并转发请求 |
| `extension` | `sdk.ExtensionPlugin` | 后台任务、自定义 API、支付、健康监控等非网关能力 |
| `middleware` | `sdk.MiddlewarePlugin` | forward 前后的旁路拦截，例如审计、脱敏、采样、合规标签 |

## 如何写一个 Gateway 插件

Gateway 插件的核心工作只有三件事：

1. 在 `Info` 中声明插件信息和账号字段。
2. 在 `Models` / `Routes` 中声明模型和 API 路由。
3. 在 `Forward` 中把请求转发到真实上游，并返回 `ForwardOutcome`。

账号管理、添加账号、编辑账号和使用记录页面由插件前端承接。插件通过 `FrontendPages`
/ `FrontendWidgets` 声明入口，通过 `WebAssetsProvider` 提供静态资源；页面需要的数据走
插件自己的 API，不进入 `GatewayService`。

入口代码：

```go
package main

import (
    sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
    runtime "github.com/DouDOU-start/airgate-sdk/runtimego/grpc"
)

type Gateway struct{}

func main() {
    runtime.Serve(&Gateway{})
}
```

关键方法。下面只展示接口形状，不是完整可编译文件：

```go
func (g *Gateway) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        ID:         "gateway-demo",
        Name:       "Demo Gateway",
        Version:    "0.1.0",
        SDKVersion: sdk.SDKVersion,
        Type:       sdk.PluginTypeGateway,
        AccountTypes: []sdk.AccountType{{
            Key:   "apikey",
            Label: "API Key",
            Fields: []sdk.CredentialField{
                {Key: "api_key", Label: "API Key", Type: "password", Required: true},
            },
        }},
    }
}

func (g *Gateway) Platform() string { return "demo" }

func (g *Gateway) Models() []sdk.ModelInfo {
    return []sdk.ModelInfo{{
        ID:              "demo-model",
        Name:            "Demo Model",
        ContextWindow:   128000,
        MaxOutputTokens: 8192,
        Capabilities:    []string{sdk.ModelCapChat},
    }}
}

func (g *Gateway) Routes() []sdk.RouteDefinition {
    return []sdk.RouteDefinition{{Method: "POST", Path: "/v1/chat/completions"}}
}

func (g *Gateway) Forward(ctx context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
    // 这里请求真实上游；示例用固定响应代替。
    body := []byte(`{"id":"demo","choices":[]}`)
    headers := http.Header{"Content-Type": []string{"application/json"}}
    if req.Stream && req.Writer != nil {
        for key, values := range headers {
            for _, value := range values {
                req.Writer.Header().Add(key, value)
            }
        }
        req.Writer.WriteHeader(http.StatusOK)
        _, _ = req.Writer.Write(body)
    }

    return sdk.ForwardOutcome{
        Kind: sdk.OutcomeSuccess,
        Upstream: sdk.UpstreamResponse{
            StatusCode: http.StatusOK,
            Headers:    headers,
            Body:       body,
        },
        Usage: &sdk.Usage{
            Model:    "demo-model",
            AccountCost: 0.000035,
            Currency: "USD",
            Summary:  "输入 10 token，输出 5 token",
            Attributes: []sdk.UsageAttribute{
                {Key: "reasoning_effort", Label: "思考层级", Kind: "reasoning", Value: "high"},
                {Key: "resolution", Label: "分辨率", Kind: "resolution", Value: "1024x1024"},
            },
            Metrics: []sdk.UsageMetric{
                {Key: "input_tokens", Label: "输入 token", Kind: "token", Unit: "token", Value: 10},
                {Key: "output_tokens", Label: "输出 token", Kind: "token", Unit: "token", Value: 5},
            },
            CostDetails: []sdk.UsageCostDetail{
                {Key: "input", Label: "输入费用", AccountCost: 0.00001, Currency: "USD"},
                {Key: "output", Label: "输出费用", AccountCost: 0.000025, Currency: "USD"},
            },
        },
    }, nil
}
```

完整 `GatewayPlugin` 还需要实现：

| 方法 | 用途 |
|---|---|
| `Init` / `Start` / `Stop` | 插件生命周期 |
| `ValidateAccount` | 添加账号时验证凭证 |
| `HandleWebSocket` | 处理 WebSocket；不支持时返回 `sdk.ErrNotSupported` |

本地调试使用 devserver：

```go
package main

import "github.com/DouDOU-start/airgate-sdk/devkit/devserver"

func main() {
    if err := devserver.Run(devserver.Config{Plugin: &Gateway{}}); err != nil {
        panic(err)
    }
}
```

## 运行原理

AirGate 的插件运行链路如下：

```text
Core 启动插件子进程
  -> runtimego/grpc 完成 go-plugin handshake
  -> Core 调用 Info / Init / Start 获取插件信息
  -> Core 挂载插件页面、静态资源、schema、健康检查和 API 代理
  -> Core 按插件类型注册网关、middleware、扩展路由、迁移、后台任务或事件订阅
  -> 用户请求进入 Core
  -> Core 完成鉴权、限流和账号调度
  -> Core 调用 Gateway.Forward
  -> 插件请求上游并返回 ForwardOutcome
  -> Core 存储插件返回的 usage/account_cost，按倍率计算用户扣费，更新账号状态，返回用户响应
```

关键边界：

- Core 默认只管理插件生命周期、页面入口、静态资源、schema、健康检查和 API 代理。
- Gateway 插件是主请求链路，Core 会主动调用 `Forward`、账号验证和 WebSocket 处理。
- SDK 不内置平台计费规则；网关插件计算标准账号成本并填入 `Usage.AccountCost`、`Usage.Attributes`、`Usage.Metrics`、`Usage.CostDetails`。
- Core 统一入库后，根据用户、分组、模型等倍率写入 `UserCost` / `BillingMultiplier`；倍率规则不进入 SDK。
- 账号管理和使用记录 UI 由插件提供静态资源，Core 只加载页面、插槽和插件 API 代理，不解释平台字段。
- Middleware、扩展路由、后台任务、事件订阅、异步任务处理都属于插件显式暴露的能力；没有暴露就不会被 Core 调度。
- Extension 插件做独立业务扩展，业务入口来自页面、插件 API、后台任务或事件，不应绕过 Core 直接访问核心业务库；需要 Core 能力时通过 `Host.Invoke` 或 `Host.InvokeStream` 回调。

## 插件回调 Core

插件要回调 Core 能力时，通过 `Host.Invoke` 或 `Host.InvokeStream` 调用。SDK 不为 Core 方法定义强类型接口；Core 自己维护方法注册表，并在加载和调用时校验插件权限、插件类型、请求 schema、是否允许流式和幂等规则。

这意味着后续扩展 Core 能力通常不需要改 SDK：

- Core 新增 method，例如 `scheduler.select_account`、`tasks.update`、`notifications.publish`。
- 插件声明 `host.invoke`，必要时再声明 `host.invoke.<method>` 做细粒度授权。
- 普通调用使用 `Invoke`，通过 `Payload` 传 JSON 对象语义的参数，通过 `Response.Payload` 读取结果。
- 流式调用使用 `InvokeStream`，通过 `HostStreamFrame` 双向传递 chunk、progress、result 等帧。

```go
Capabilities: []sdk.Capability{
    sdk.CapabilityHostInvoke,
    sdk.CapabilityForHostMethod("tasks.update"),
}
```

插件在 `Init` 中获取 Host：

```go
func (p *Plugin) Init(ctx sdk.PluginContext) error {
    if h, ok := ctx.(sdk.HostAware); ok {
        p.host = h.Host()
    }
    return nil
}
```

调用 Core 方法：

```go
resp, err := p.host.Invoke(ctx, sdk.HostInvokeRequest{
    Method: "tasks.update",
    Payload: map[string]interface{}{
        "task_id":  taskID,
        "status":   sdk.TaskStatusCompleted.String(),
        "progress": 100,
    },
})
```

调用 Core 流式方法：

```go
stream, err := p.host.InvokeStream(ctx, sdk.HostStreamRequest{
    Method:  "chat.stream",
    Payload: map[string]interface{}{"prompt": "hello"},
})
if err != nil {
    return err
}
defer stream.CloseSend()

for {
    frame, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if frame.Done {
        break
    }
    // 使用 frame.Event / frame.Payload 处理流式数据。
}
```

当前内置 capability：

| Capability | 用途 |
|---|---|
| `host.invoke` | 允许插件调用 Core 开放的方法 |
| `host.invoke.<method>` | Core method 级细粒度授权，例如 `host.invoke.tasks.update` |
| `middleware.read_body` | middleware 接收请求/响应 body |

完整规则见 [SDK 包边界](docs/sdk-package-boundaries.md)。

## 插件私有数据

插件私有数据库使用 `plugin_dsn`。Core 注入的 DSN 指向插件独立 schema，插件不得读取 Core 业务库 DSN。

```go
func (p *Plugin) Init(ctx sdk.PluginContext) error {
    dsn := sdk.GetPluginDSN(ctx)
    if dsn == "" {
        return nil
    }
    // 使用插件私有 schema 建表和读写数据。
    return nil
}
```

## 前端插件 SDK

`theme/` 发布为 npm 公共包 `@doudou-start/airgate-theme`，用于插件前端复用 AirGate 的主题和公共组件。

| 入口 | 用途 |
|---|---|
| `@doudou-start/airgate-theme` | token、CSS 工具、Tailwind bridge、插件前端类型和公共组件统一出口 |
| `@doudou-start/airgate-theme/plugin` | 插件样式隔离、主题同步、Tailwind helper、公共 UI 组件 |
| `@doudou-start/airgate-theme/css` | CSS 变量生成和运行时主题注入 |
| `@doudou-start/airgate-theme/tailwind` | Tailwind 主题桥接 |

推荐插件前端使用：

```tsx
import {
    Button,
    Field,
    SecretInput,
    createPluginTailwindConfig,
    ensurePluginStyleFoundation,
    useScopedPluginTheme,
} from "@doudou-start/airgate-theme/plugin";
```

完整样式规则见 [插件前端样式规范](docs/plugin-style-guide.md)。

网关插件账号相关 UI 建议使用这些稳定插槽。Core 仍提供通用账号列表和详情框架，
插件只补平台差异片段；需要完整独立页面时使用 `FrontendPages`。devserver
可直接预览 `account-create` / `account-edit`，其他插槽由 Core 对应页面加载。

| Slot | 用途 |
|---|---|
| `account-identity` | 账号标识、套餐、状态等平台差异信息 |
| `account-create` | 添加账号 |
| `account-edit` | 编辑账号 |
| `account-usage-window` | 账号用量窗口、额度、重置时间等平台差异信息 |
| `usage-metric-detail` | 使用记录里的计量明细，例如 token、模型、思考层级、分辨率、图片张数 |
| `usage-cost-detail` | 使用记录里的费用明细，例如单价、账号成本、Core 倍率、用户扣费 |

## 开发命令

```bash
make ci                            # 运行 Go、proto、前端和主题漂移检查
make proto                         # 重新生成 protocol/proto
make theme                         # 重新生成 DevServer 主题 CSS
cd theme && pnpm build          # 构建 @doudou-start/airgate-theme
```

## License

MIT
