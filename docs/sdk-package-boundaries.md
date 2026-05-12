# SDK 包边界

本文定义 `airgate-sdk` 单仓库内的包职责边界。新增能力必须先判断归属，不能默认修改 `sdkgo` 根接口。

## 分层

| 包 | 职责 | 可依赖 | 不应包含 |
| --- | --- | --- | --- |
| `sdkgo` | 插件作者 API、共享类型、capability helper、日志 helper | Go 标准库、少量稳定依赖 | protobuf、gRPC、go-plugin、devserver、Core 产品逻辑 |
| `protocol/proto` | protobuf schema 与生成代码 | protobuf runtime | 插件业务 helper、Core 实现细节 |
| `runtimego/grpc` | go-plugin/gRPC 适配、stream bridge、proto 转换、Core 反向调用 broker | `sdkgo`、`protocol/proto` | 插件业务逻辑、devserver UI |
| `devkit/devserver` | 本地开发服务器和 fake core 能力 | `sdkgo` | 生产运行时依赖、Core 数据库访问 |
| `frontend` | 前端插件 API、主题 token、样式注入、Tailwind bridge、公共 UI 组件 | TypeScript 生态、React peer dependency | Go runtime、Core 后端逻辑、具体插件业务页面 |

## 新需求判断

只有以下变化可以修改 `sdkgo` 稳定接口：

- 新增稳定插件角色或生命周期钩子。
- 新增跨插件通用领域原语。
- 新增需要 SDK 表达的跨插件通用 capability。
- 修正已有公开契约的错误或缺口。

只有以下变化可以修改 `frontend` 稳定接口：

- 新增跨插件通用的主题 token、CSS 变量或 Tailwind bridge 能力。
- 新增多个插件都会复用的基础 UI 组件。
- 新增插件前端模块加载、账号表单、路由、菜单、OAuth 桥接等公共类型。
- 修正已有公共组件、样式 helper 或主题契约的错误。

以下变化不应直接修改 `sdkgo` 根接口：

- 某个 provider 新增参数、route、错误格式或模型字段。
- 某个 UI 页面需要额外产品数据。
- 某个插件需要私有后台任务状态。
- 某个平台新增计费规则、套餐、价格档位、重置窗口或用量展示字段。
- 某个实验性功能只服务单个官方插件。
- Core 内部实现为了方便调用而需要的 helper。

这些变化应优先放到 manifest、插件私有 metadata、Core 方法注册表、插件私有数据库或明确 schema 的插件 API 中。

前端页面需求应优先进入具体插件前端代码。只有当样式、组件或类型能被多个插件稳定复用时，才进入 `frontend`。

## 弱契约扩展点

SDK 提供少量弱契约扩展点，用来承接展示、分类和通用计量等变化，避免为每个插件需求新增强类型字段：

- `PluginInfo.Metadata`：插件市场分类、标签、展示提示等。
- `ModelInfo.Metadata`：模型家族、展示分组、供应商标签等。
- `RouteDefinition.Metadata`：路由文档链接、展示分组、调试提示等。
- `Usage.Attributes`：模型、思考层级、分辨率、质量档、服务档位等非数值审计维度。
- `Usage.Metrics`：图片张数、视频秒数、音频分钟数、工具调用次数、token 等插件计算后的通用计量结果。
- `Usage.CostDetails`：费用明细；插件填账号成本，Core 填用户扣费和倍率。
- `Usage.Metadata`：单次调用的展示或审计辅助信息。
- `EventHandler`：Core 向插件推送标准事件。
- `Host.Invoke` / `Host.InvokeStream`：插件用 `method + payload` 调用 Core 开放的方法，必须由 `host.invoke` 或 `host.invoke.<method>` capability 门控。
- `SchemaProvider`：插件声明 routes、tasks、events、invokes 的 payload schema；流式 Host method 用 `InvokeSchema.Transport`、`ClientFrame`、`ServerFrame` 描述传输模式和帧结构。

这些字段不能用于权限、调度、账号状态机或敏感数据传递。平台计费规则不得进入 SDK；网关插件负责计算标准账号成本 `Usage.AccountCost` / `Currency` 和审计明细。Core 统一入库、索引、汇总，并写入 `UserCost` / `BillingMultiplier`。

## 用量与计费边界

SDK 不提供 `CalculateCost`、价格档位、token 拆分公式或平台套餐模型。

- 模型声明只包含 `ID`、`Name`、上下文窗口、最大输出和能力标签。
- 单次调用的标准账号成本由网关插件写入 `Usage.AccountCost` / `Usage.Currency`。
- Core 根据用户、分组、模型等倍率计算用户侧扣费，写入 `Usage.UserCost` / `Usage.BillingMultiplier`。
- 模型、思考层级、分辨率、质量档等非数值维度统一放入 `Usage.Attributes`。
- token、图片、音频、视频、请求数等数值明细统一放入 `Usage.Metrics`。
- 标准账号成本和用户扣费拆分统一放入 `Usage.CostDetails`：插件填 `AccountCost`，Core 填 `UserCost` / `BillingMultiplier`。
- 使用记录和账号管理页面由插件前端与插件私有 API 实现，SDK 不定义账号用量查询 RPC。
- Core 不应把平台规则写入 SDK 或 Core 公共逻辑；Core 统一入库 `Usage`，需要页面时加载插件的静态资源和 API 代理。

## 网关前端边界

网关插件可以通过 `FrontendPages` / `FrontendWidgets` 声明账号相关入口，通过 `WebAssetsProvider` 提供静态资源。Core 负责加载资源、传递登录上下文和代理插件 API，不理解页面内部数据结构。

账号管理不作为整页 slot。Core 保留通用账号列表和详情框架，插件只补平台差异片段：

- `account-identity`：账号标识、套餐、状态等平台差异信息。
- `account-create`：添加账号。
- `account-edit`：编辑账号。
- `account-usage-window`：账号用量窗口、额度、重置时间等平台差异信息。
- `usage-metric-detail`：使用记录里的计量明细，例如 token、模型、思考层级、分辨率、图片张数。
- `usage-cost-detail`：使用记录里的费用明细，例如单价、账号成本、Core 倍率、用户扣费。

新增平台页面优先放在插件自己的 `FrontendPages`；只有多个插件都需要同一宿主位置时，才新增通用 slot。

## Host 调用约束

SDK 的 `Host` 接口只保留通用调用通道：`Invoke(ctx, req)` 和 `InvokeStream(ctx, req)`。新增 Core 宿主能力不得向 `Host` 追加业务方法；应在 Core 注册一个 method，并声明 method 的权限、schema、传输模式和实现。

Core method 必须至少定义：

- method 名称，例如 `scheduler.select_account`、`tasks.update`。
- 允许的插件类型和可调用插件范围。
- 所需 capability，至少 `host.invoke`，敏感方法应使用 `host.invoke.<method>`。
- 请求、响应和流式 frame payload schema。
- 传输模式：普通 request/response、server stream、client stream 或 bidirectional stream。
- 幂等策略、敏感字段暴露规则和审计日志。

SDK 只提供传输契约和自检 helper，不承载 Core 方法枚举。这样 Core 后续扩展 method 时，不需要为每个能力修改 SDK。

## 当前导入路径

- 插件业务代码使用 `github.com/DouDOU-start/airgate-sdk/sdkgo`。
- 插件运行入口使用 `github.com/DouDOU-start/airgate-sdk/runtimego/grpc`。
- 本地开发工具使用 `github.com/DouDOU-start/airgate-sdk/devkit/devserver`。
- 普通插件业务代码不直接导入 `protocol/proto`。
- 插件前端使用 `@airgate/theme/plugin` 引用样式隔离、主题同步、Tailwind helper 和公共 UI 组件。
- 宿主前端或工具代码可使用 `@airgate/theme`、`@airgate/theme/css`、`@airgate/theme/tailwind` 引用 token 与主题桥接能力。
