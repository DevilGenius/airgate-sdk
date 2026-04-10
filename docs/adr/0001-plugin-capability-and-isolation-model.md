# ADR-0001：插件能力模型与隔离边界

- **状态**：Accepted
- **日期**：2026-04-10
- **作者**：AirGate 核心团队
- **影响范围**：airgate-sdk（proto / pluginsdk / grpc bridge）、airgate-core（plugin manager / host service / forwarder）、所有现有插件（airgate-openai、airgate-health、airgate-epay）

---

## 1. Context（为什么要写这份 ADR）

### 1.1 插件系统的演进历史

AirGate 从一开始就把"可插拔"作为架构目标。插件以 hashicorp/go-plugin 子进程形式运行，通过 gRPC 与 core 通信。已经上线的插件有：

- **airgate-openai**（gateway 类型）：OpenAI/Anthropic upstream adapter
- **airgate-epay**（extension 类型）：第三方支付接入 + 订单表
- **airgate-health**（extension 类型）：分组级健康监控探测 + 时序表

最初的协议（`plugin.proto` v1）只定义了 **Core → Plugin** 方向的三个服务：`PluginService`（生命周期）、`GatewayService`（upstream 转发）、`ExtensionService`（后台任务 + HTTP 代理）。没有 **Plugin → Core** 的反向通道。

### 1.2 Step 1 补上了反向通道，但留下了四个结构性问题

2026-04-09 的 Step 1 重构里引入了 `HostService`：Core 通过 hashicorp/go-plugin 的 `GRPCBroker` 向每个插件子进程暴露一条反向 gRPC stream，让插件可以回调 core 能力（`SelectAccount` / `ProbeForward` / `ListGroups` / `ReportAccountResult`）。Step 2 用这套机制把 airgate-health 从账号级探测重写为分组级黑盒探测。

Step 2 落地后，我们识别出 4 个结构性问题需要在 Step 3 之前解决：

**问题 1：插件直连 core 业务数据库**

现状：`airgate-health` 和 `airgate-epay` 都通过 `db_dsn` 注入直接拿 core 的 PostgreSQL 连接字符串，然后用 `database/sql` 自己跑 SQL。后果：

- 插件能读写 core 任何表（`accounts` / `users` / `api_keys` / `usage_logs`），没有权限边界
- 插件与 core schema 强耦合。core 重命名字段或改表结构，插件就挂。`airgate-health/aggregator.go` 里 `SELECT ... FROM groups g JOIN account_groups ag ON ...` 就是这类反模式的典型
- 没有审计、没有事务边界、没有最小权限原则

**问题 2：HostService 权限完全无差别**

HostService 的 5 个 RPC 对任何插件一视同仁。这导致：

- 一个监控用途的扩展插件可以调 `ReportAccountResult` 污染账号状态机
- 一个 upstream gateway 插件可以调 `ListGroups` 读到所有分组元信息
- 无法在审计时回答"这个插件到底用了哪些 core 能力"

**问题 3：Forward 路径没有插件 hook 点**

我们想给 AirGate 加"请求/响应中间层"——例如：

- 内容审计（记录完整 request/response 到独立日志库）
- 敏感数据脱敏
- 跨 request 合规标签注入
- 旁路回放 / 流量镜像 / 异常样本采样

这些需求天然就是 middleware 模式。但当前 `forwarder.go:143 Forward(c *gin.Context)` 是一个闭合流程，内部直接 `buildForwardState → ensureAllowed → selectAccount → prepare → execute → finish`，没有任何第三方插件插桩的地方。

此外，当前的 `GatewayService.Forward` 是为"插件作为 upstream adapter"设计的——一个 gateway 插件**替代**了 upstream。我们想要的 middleware 插件是另一种角色：**旁路观察 + 允许修改 request/response**，不替代 upstream。SDK 里现在根本没有这种角色。

**问题 4：插件类型只有两种**

`sdk.PluginType` 当前只有 `gateway` 和 `extension`。问题 3 里的"请求中间层"既不是 gateway 也不是 extension。强行塞进 extension 会让 extension 的语义越来越模糊。

### 1.3 现在做还是以后做

这些问题不是 Step 2 引入的，是 Step 1 为了"最快打通反向通道"一并带过的历史债。**现在做最便宜**，理由：

- 插件生态还小（3 个官方插件），改动面可控
- 还没有第三方插件作者，没有向前兼容压力
- 正准备写"请求中间层"这种新类型的插件，这是最后的窗口

一旦第三方作者开始写插件、或者中间层插件写完再回头改，代价会翻倍。

---

## 2. Design Principles（未来所有 SDK 演进的宪法）

### 原则 1：插件永远不直连 core 业务表

core 的业务数据（`accounts` / `groups` / `users` / `usage_logs` / `api_keys` 等）一律通过 `HostService` RPC 访问。**插件不拿 core schema 的 DSN**。

例外：插件仍然可以把**自己的数据**存在 PostgreSQL 里，但必须在独立 schema 下，通过受限 postgres 角色访问（参见原则 5）。

### 原则 2：能力按插件类型分级授权（Capability-based）

不同的 PluginType 拿到不同的 HostService 接口。具体机制：

1. 插件在 `PluginInfo.Capabilities` 里声明它**将要使用**的所有 HostService 能力
2. Core 启动插件时校验：
   - SDK 版本是否支持这些 capability
   - 插件类型是否有权使用这些 capability（用"插件类型 → 允许的 capability 集合"映射表）
3. Core 用一个 gRPC interceptor，在每次 HostService RPC 调用时检查"这个插件声明了这个 capability 吗？"，未声明直接返回 `PermissionDenied`

这是**最小权限原则**的落地：一个插件拿到的能力是它"明确索取的 ∩ 插件类型允许的"。

### 原则 3：引入 middleware 插件类型

为"请求/响应记录 / 审计 / 脱敏 / 流量采样"这类场景新建一种插件角色：

- 不替代 upstream（那是 gateway 的工作）
- 不跑后台任务（那是 extension 的工作）
- **核心职责**：在每次 forward 的前后被 core 回调，可观察、可修改请求/响应

这把"中间件"从"代码里的 interface"抬到了**协议层的 RPC**。任何语言写的插件都能做 middleware，不局限于 Go。

### 原则 4：SDK 协议向前兼容

HostService / MiddlewareService 一旦暴露就按"契约"对待：

- **只加字段不删字段**（protobuf 天然支持）
- **加新 RPC 用新 rpc name**，不 hijack 旧的
- **新能力必须伴随新 capability flag**，旧插件不声明就不启用
- deprecated 字段保留至少 2 个大版本
- SDK 版本号（`sdk.SDKVersion`）与 proto 的向后兼容性同步

### 原则 5：Core 是 trust root，插件是 untrusted

即便是自己写的插件，也按"不可信"对待：

- 所有 HostService 输入做参数校验（边界 / 类型 / 长度）
- 插件声明的 capability 不能超过 SDK 版本允许的集合
- credentials / password_hash / admin_api_key 等敏感字段**永远不通过 RPC 流向插件**（即使 `HostService.GetAccount` 也会脱敏）
- 插件的 frontend 资源在独立 iframe 或加 sandbox 属性
- 插件写自己的表用独立 postgres 角色，只授 `USAGE + ALL` 在自己的 schema 上，`public` schema **全部 REVOKE**

### 原则 6：所有决策记录在 ADR 里

未来的 SDK / plugin 架构改动都必须有一份对应的 ADR，遵循本文档的结构（Context → Design → Decision → Migration → Risks）。ADR 一旦 accepted 就不改历史，通过新 ADR `supersedes` 旧 ADR。

---

## 3. Decisions（5 个拍板的决策）

### Decision 1（Q1）：插件禁止直连 core 业务数据库

**选择**：插件不能读写 `public` schema 下的 core 表。core 业务数据一律通过 `HostService` RPC 访问。插件自己的数据存在独立 schema（见 Decision 5）。

**rejected 方案**：
- "允许只读" → 仍然有 schema 耦合问题（core 改表名插件就挂）
- "文档约定" → 靠自觉永远不可靠

**影响**：
- `airgate-health` 的 `aggregator.go` 中所有 `SELECT ... FROM groups` / `SELECT ... FROM accounts` 要迁移到 HostService RPC。预计改 ~10 个 SQL 语句
- `airgate-epay` 继续用独立 schema 存 `payment_orders`，不受影响
- core 不再向插件注入 `db_dsn` 字段。改为注入 `plugin_dsn`（见 Decision 5）

### Decision 2（Q2）：middleware 插件起步只做 2 个 hook 点

**选择**：`OnForwardBegin` + `OnForwardEnd`

**签名**：

```go
type MiddlewarePlugin interface {
    Plugin

    // OnForwardBegin 在 core 选完账号、但还没调 upstream 插件之前触发。
    // 允许返回 decision 修改请求（加 header、改 body），或拒绝放行（返回 Deny）。
    OnForwardBegin(ctx context.Context, req *MiddlewareRequest) (*MiddlewareDecision, error)

    // OnForwardEnd 在 upstream 插件返回之后、core 写 usage_log 之前触发。
    // 拿到完整的请求 + 响应元数据（但不一定包含 body，见 Decision 3）。
    // 返回 error 不影响主流程（只 log warn），保证 middleware 永远不 block 生产流量。
    OnForwardEnd(ctx context.Context, evt *MiddlewareEvent) error
}
```

**不做**（留给未来 ADR）：`OnSelectAccount` / `OnFailover` / `OnStreamChunk`

**理由**：
- Begin + End 覆盖 90% 的用例（日志 / 审计 / 合规标签 / 脱敏）
- 流式 chunk 级 hook 性能代价大，只在少数场景必要，延后引入
- 每加一个 hook 都是永久协议承诺，宁缺毋滥

**顺序**：
- 多个 middleware 按 `Priority`（PluginInfo 新字段）从小到大排序依次调用
- `OnForwardBegin` 按 priority 升序
- `OnForwardEnd` 按 priority **降序**（LIFO，像 middleware stack 展开）

**失败语义**：
- `OnForwardBegin` 返回 error → 该 middleware 被跳过，流程继续（不 block 生产）
- `OnForwardBegin` 返回 `MiddlewareDecision{Action: Deny}` → 整个请求被拒绝，返回给用户的错误信息来自 decision
- `OnForwardEnd` 返回 error → log warn，其余 middleware 仍然照常跑

### Decision 3（Q3）：middleware payload 采用两段式

**选择**：默认只传元数据；需要 body 的 middleware 显式声明 capability `middleware.read_body`

**MiddlewareRequest / MiddlewareEvent 字段设计**：

```proto
message MiddlewareRequest {
    // 元数据（默认传）
    string request_id       = 1;
    int64  user_id          = 2;
    int64  group_id         = 3;
    int64  account_id       = 4;
    string platform         = 5;
    string model            = 6;
    bool   stream           = 7;
    int64  input_tokens_est = 8;  // core 侧的粗略估算

    // 按需传（声明了 middleware.read_body 的插件才会收到）
    bytes  request_body     = 100;
    map<string, HeaderValues> request_headers = 101;
}

message MiddlewareEvent {
    string request_id   = 1;
    // ... 同上的元数据

    int64  status_code  = 20;
    int64  duration_ms  = 21;
    int64  input_tokens = 22;  // 实际值（插件计算后）
    int64  output_tokens = 23;
    int64  first_token_ms = 24;
    string error_kind   = 25;  // 失败时填
    string error_msg    = 26;

    // 按需传
    bytes  response_body = 100;
    map<string, HeaderValues> response_headers = 101;
}
```

**理由**：
- 大部分 middleware（日志 / 计费 / 审计标签）只需要元数据。0 额外开销
- 内容审计 / 脱敏 / replay 这类真的需要 body 的场景，显式 opt-in
- 管理员在"插件管理"页能清楚看到"这个 middleware 插件声明了读取 body 的权限"，有知情同意

**取舍**：
- 声明了 `middleware.read_body` 的插件会让该次请求在 core 侧多一次 body 序列化 + gRPC 传输。对开发者透明但成本真实
- 流式 body 的处理：Begin 阶段能拿到完整 request body；End 阶段流式响应的 response_body 只传**聚合后的首次非空 chunk 拼装**的文本摘要（完整流式内容 hook 留给未来的 `OnStreamChunk`）

### Decision 4（Q4）：capability 系统现在就做

**选择**：Step 3 一并落地 capability 模型

**proto 改动**：

```proto
message PluginInfoResponse {
    // ... 现有字段
    repeated string capabilities = 14;  // 新增：插件声明的能力列表
}
```

**capability 命名规范**：`<domain>.<action>`

当前（Step 1 + Step 2）已经隐式使用的能力，现在要求 **显式声明**：

| Capability | Owner RPC | 允许的插件类型 |
|---|---|---|
| `host.list_groups` | `HostService.ListGroups` | extension, middleware |
| `host.probe_forward` | `HostService.ProbeForward` | extension（只给 probe 子类，详见未来的 PluginSubtype） |
| `host.select_account` | `HostService.SelectAccount` | extension |
| `host.report_account_result` | `HostService.ReportAccountResult` | extension（只给 probe 子类） |
| `middleware.read_body` | 改变 MiddlewareRequest/Event 的 body 字段填充 | middleware |

**未来可能新增**（不在 Step 3 范围内，仅作预留）：
- `host.list_accounts` / `host.get_account` / `host.list_users`（只读业务数据）
- `host.write_usage_log`（某些计费中间件）
- `middleware.rewrite_request` / `middleware.rewrite_response`（有副作用的 middleware）

**core 侧实现**：
1. 插件启动时 `PluginInfo.Capabilities` 和"插件类型 → 允许集合"做交集，产出**有效 capability set**
2. `HostService` 注册一个 gRPC unary interceptor，每次 RPC 调用时从 `context` 里取出本插件的 capability set，检查当前 method 是否允许
3. 未允许 → 返回 `status.Errorf(codes.PermissionDenied, "plugin %s lacks capability %s", pluginID, cap)`
4. 管理员页能看到每个插件的 capability 列表 + 一个"capability 校验失败次数"计数

**迁移**：
- `airgate-health` 的 `metadata.go` 加 `Capabilities: []string{"host.list_groups", "host.probe_forward", "host.report_account_result"}`
- `airgate-openai` / `airgate-epay` 当前没用 HostService，`Capabilities` 留空即可
- 旧版本插件（未声明 Capabilities）：**SDK 版本 <= 0.2.x 的插件豁免**（向后兼容），SDK 版本 >= 0.3.x 的插件必须声明。Core 侧按 sdk_version 字段区分

### Decision 5（Q5）：插件数据库使用独立 schema

**选择**：同一个 PostgreSQL 实例下为每个有 DB 需求的插件创建独立 schema + 独立受限角色

**机制**：

1. Core 启动时，对每个已加载的插件：
   - 检查 DB 里是否存在 `plugin_<plugin_id>` schema，不存在则创建（`CREATE SCHEMA IF NOT EXISTS`）
   - 检查是否存在 `plugin_<plugin_id>_role` 角色，不存在则创建，密码随机生成存 settings 表
   - `GRANT USAGE, CREATE ON SCHEMA plugin_<plugin_id> TO plugin_<plugin_id>_role`
   - `REVOKE ALL ON SCHEMA public FROM plugin_<plugin_id>_role`
2. Core 向插件注入 `plugin_dsn`（不再叫 `db_dsn`），DSN 里：
   - 用户名 = `plugin_<plugin_id>_role`
   - 密码 = 上面生成的随机值
   - `search_path=plugin_<plugin_id>`（所有 SQL 默认查这个 schema）
3. 插件 Init 时拿到 `plugin_dsn`，用 `database/sql` 正常连接。插件代码里写 `CREATE TABLE group_health_probes (...)` 会被自动建在 `plugin_airgate_health.group_health_probes`，而 `SELECT * FROM groups` 会因为 `public` schema 的 REVOKE 而直接被 PostgreSQL 拒绝

**rejected 方案**：
- **独立 DB 实例**：需要额外运维成本（备份 / 连接池 / 监控），对单机部署不友好
- **KV-only RPC**：airgate-health 的日桶聚合 SQL 没法做

**好处**：
- 权限在 PostgreSQL 层面强制执行，core 代码层不需要任何审查
- 插件开发体验几乎不变：原来 `CREATE TABLE health_probes` 现在建在 `plugin_airgate_health.health_probes`，对 SQL 透明
- 未来想迁移到独立 DB 实例也容易：只要改 DSN，插件代码不变

**迁移**：
- **airgate-health**：旧表 `public.health_probes` 已在 Step 2 被 DROP；新表 `public.group_health_probes` 在 Step 2 建在了 `public` schema。Step 3 需要把它迁到 `plugin_airgate_health.group_health_probes`。迁移 SQL：`ALTER TABLE public.group_health_probes SET SCHEMA plugin_airgate_health;`
- **airgate-epay**：类似，`payment_*` 表 SET SCHEMA 到 `plugin_airgate_epay`
- **airgate-openai**：有 `plugin_openai_session_states` / `plugin_anthropic_digest_sessions`，同样迁移
- **core 自己的 `plugins` / `plugin_sources` / `plugin_account_usage_snapshots` 表保留在 `public`**（这是 core 自己的表，不是插件的表）

---

## 4. Implementation Plan（分步落地）

### Step 3：Capability + Middleware + DB 隔离（这份 ADR 的实现）

**核心产物**：
- proto 加 `capabilities` 字段、`MiddlewareService` 服务、Middleware 相关 messages
- sdk/pluginsdk 加 `MiddlewarePlugin` interface、`PluginTypeMiddleware` 常量、capability 常量表
- airgate-core 加 HostService interceptor + middleware chain + DB isolation 逻辑
- 文档和样例

**范围边界**：
- **做**：capability 模型、middleware 接口、DB schema 隔离、已有 3 个插件的迁移
- **不做**：`OnSelectAccount` / `OnFailover` / `OnStreamChunk`（留给未来 ADR）
- **不做**：`host.list_accounts` / `host.get_account` 等新业务查询 RPC（等真的有插件要用再加）

### Step 4+：未来 ADR 的候选议题

- ADR-0002：Middleware 流式 hook 点（`OnStreamChunk`）
- ADR-0003：插件级资源配额与故障隔离（一个失控的 middleware 不能拖垮 core）
- ADR-0004：插件热更新与 capability 变更（管理员开关某个 capability 后的生效路径）
- ADR-0005：跨插件事件总线（如果出现 plugin A 需要订阅 plugin B 产生的事件）

---

## 5. Migration Plan（三个现有插件怎么升级）

### 5.1 airgate-openai（gateway 插件）

- 不使用 HostService，不使用插件 DB
- **唯一改动**：`PluginInfo.Capabilities = []string{}`（可空）
- 风险：低

### 5.2 airgate-epay（extension 插件，payment）

- 不使用 HostService，但使用 `db_dsn`
- **改动**：
  1. 配置键从 `db_dsn` 改为 `plugin_dsn`（或者 SDK 新增 helper `ctx.PluginDB()` 返回已配好 search_path 的 `*sql.DB`）
  2. `PluginInfo.Capabilities = []string{}`
  3. 启动时 core 自动迁移 `payment_*` 表到 `plugin_airgate_epay` schema
- 风险：中（DB 迁移需要停机或小心做）

### 5.3 airgate-health（extension 插件，monitoring）

- 使用 HostService + 插件 DB
- **改动**：
  1. `PluginInfo.Capabilities = []string{"host.list_groups", "host.probe_forward", "host.report_account_result"}`
  2. 配置键从 `db_dsn` 改为 `plugin_dsn`
  3. `aggregator.go` 里 `SELECT ... FROM groups` 的 SQL 迁移到 `host.ListGroups()` RPC 调用
  4. `group_health_probes` 表 `SET SCHEMA plugin_airgate_health`
  5. `fillMissingPlatforms` 里 `SELECT platform, COUNT(*) FROM groups GROUP BY platform` 迁到 HostService 新 RPC `ListPlatforms()` —— 或者干脆从 `ListGroups()` 的结果里在 Go 侧聚合
- 风险：中高（SQL 改动面广）

### 5.4 迁移顺序

1. **Step 3a**：先做 proto + sdk + core 的基础设施改动（capability、middleware 接口、DB 隔离的 core 侧）。向后兼容：旧插件不声明 capability 仍然跑（sdk_version 豁免）
2. **Step 3b**：迁移 airgate-openai（最简单，验证 capability 声明路径）
3. **Step 3c**：迁移 airgate-epay（验证 DB schema 隔离的自动迁移路径）
4. **Step 3d**：迁移 airgate-health（验证"SQL 迁到 RPC"路径，这是最复杂的）
5. **Step 3e**：写第一个 middleware 插件（建议写 `airgate-audit`，最小 MVP 就是 `OnForwardEnd` 写一行到独立 schema 的 `audit_events` 表）来端到端验证 middleware 接口

---

## 6. Risks and Open Questions

### 6.1 风险

**R1：DB 迁移失败导致插件起不来**
- 缓解：迁移 SQL 写成幂等（`SET SCHEMA` 本身幂等）、遇到已存在的目标表报错时 log + skip
- 兜底：保留一个"恢复模式"——管理员可以强制插件以旧 DSN 启动一次，手动干预数据

**R2：middleware 插件拖慢关键路径**
- 缓解：每次 OnForwardBegin/End 调用设 deadline（默认 200ms），超时即跳过该 middleware（log warn）
- 缓解：middleware chain 总超时预算（默认 500ms），超预算剩下的 middleware 全部跳过
- 监控：core 暴露 `middleware_latency_ms` / `middleware_timeout_total` 指标

**R3：capability 声明漂移**
- 问题：插件代码里调 `host.list_groups`，但忘了声明 capability → 每次调用都被 interceptor 拒绝
- 缓解：插件首次 Dispense 后做一次 self-check——把声明的 capabilities 发给 core，core 返回"你声明的里有 X 个是我不认识的" + "你没声明但未来可能用到的 Y 个是这些"（只 hint）

**R4：向后兼容打破存量插件**
- 缓解：用 `sdk_version` 字段区分新旧行为。SDK 0.2.x 的插件豁免 capability 校验，SDK 0.3.x 起强制
- 存量插件给一个 grace period（一个大版本），期间只 log warn 不 block

### 6.2 Open Questions

**Q-open-1：middleware 的链式修改如何 merge？**

如果 middleware A 在 Begin 阶段改了 request header，middleware B 也改了同一个 header，谁赢？
- **倾向**：按 priority 顺序后来者覆盖，不做自动 merge。但这需要在 `MiddlewareDecision` 里明确语义
- **延后**：Step 3 MVP 只支持 header 的"追加不覆盖"，修改 body 留到有真实需求时再设计

**Q-open-2：capability 需要 versioning 吗？**

如果未来 `host.list_groups` 的返回结构加字段，算不算 breaking？
- **倾向**：proto 本身的字段增加是向前兼容的，不需要新 capability。但如果语义变化（比如"以后 ListGroups 默认只返回启用的分组"），就必须引入 `host.list_groups.v2`
- **当前不解决**：等真的遇到了再说

**Q-open-3：middleware 之间能互相通信吗？**

比如 A 在 Begin 里给 request 打了个 tag，B 在 End 里要读这个 tag。
- **倾向**：在 MiddlewareRequest/Event 里加一个 `map<string, string> metadata` 字段，所有 middleware 共享一个 KV bag
- **Step 3**：加这个字段，但不规定命名空间规则。让它自由生长，三个月后复盘是否需要规则

---

## 7. References

- Step 1（HostService 引入）：提交 `TBD`（本 ADR approve 后在这里回填）
- Step 2（airgate-health 分组级重写）：提交 `TBD`
- hashicorp/go-plugin GRPCBroker 文档：https://github.com/hashicorp/go-plugin/blob/master/docs/grpc-broker.md
- Capability-based security（Wikipedia）：https://en.wikipedia.org/wiki/Capability-based_security

---

## 8. Changelog

- **2026-04-10**：v1 draft，AirGate 核心团队
