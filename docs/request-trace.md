# 请求 Trace 的边界与接入

## 保存内容

Trace 开关沿用最终失败请求诊断语义，不是保存所有成功请求的审计开关。

- Core 在入口、任何协议转换之前复制客户端请求体，写入 `request.body`。正文、完整历史、客户端扩展字段及原始 JSON 排版保留。
- 插件通过 `runtimego/requesttrace` 记录实际发送的 HTTP 请求或 WebSocket 消息，写入 `attempts[].outbound_requests[].body`。客户端原文与上游协议转换后的正文分别保存，不互相覆盖。
- HTTP 状态、上游原始响应或最近读取的 SSE/WebSocket 数据事件作为辅助诊断保存；它们不能替代请求正文。
- 图片输入、认证字段、敏感请求头统一脱敏。Core 与插件使用同一个 `SanitizeBody`，不保存图片二进制、base64 图片或 access/refresh token。
- 本地校验未发起上游请求时，Core 仍有客户端原文，但不会伪造出站请求。

## SDK 采集器

每次 `Forward` 入口调用 `requesttrace.Start(ctx, req.TraceFinalError)`，出口在所有结果策略之后调用 `capture.Finish(&outcome, err)`。应使用闭包 defer，以读取最终的 outcome 和 err。

只有主模型转发使用 `buildForwardHTTPClient` 包装 `requesttrace.Transport`。采集器从当前请求的 context 获取，不绑定在共享连接池或账号对象上。普通 `buildHTTPClient` 不采集 trace；图片上传/下载、BAS 附件上传、认证请求和辅助轮询均不进入出站 trace。`imageDownloadHTTPClient` 与 imgen 客户端也不安装 trace 传输，因此 `images_web_reverse` 使用的图片辅助链路不会覆盖主请求诊断。

可重放请求体在发送前读取独立 reader，保证后续修改原始缓冲区不会污染 trace。没有 `GetBody` 的 reader 使用随读采集，不预读或阻塞真实发送。响应也只随读取观察，不提前消耗 SSE、不新增读协程、不改变流关闭和取消行为。

WebSocket 在统一拨号入口用 `requesttrace.WrapWebSocket` 包装。它在消息读写边界采集实际字节，普通 OAuth、图片生成与其他使用同一连接封装的模式不再手动埋点。失败握手通过 `Record` / `WrapResponse` 保存。

OpenAI 插件仅在公共 `Forward` 入口读取 trace 开关，业务模式不调用专用 trace API。BAS 转换器也不再包含诊断 observer。旧的 `final_error_trace` 模块和重复脱敏实现已经移除，没有旧函数别名或双轨兼容逻辑。

## 资源与落盘边界

每次插件 attempt 最多保留 8 个出站请求（最早一个与最近七个），诊断正文总预算 48 MiB，单个 SSE 数据事件最多 16 MiB。正常大小的文本请求逐字节保存；超过预算的请求标记 `trace_size_limit`，不可完整读取的已知长度请求标记 `trace_capture_incomplete`，不将其伪装为完整正文。SDK 的 gRPC 消息上限仍然适用。

Core 负责失败 attempt 汇总、最终失败判定、gzip 压缩、去重、保留期和监控页面读取。SDK 采集器不访问数据库、不决定计费或重试，也不区分 BAS、OAuth、Chat 或 Anthropic。

## 保留期与内存生命周期

trace 表不设置过期时间，不进行自动清理，永久保留直到管理员手动清空。普通监控事件的保留策略独立，不受此改动影响。升级迁移只移除 trace 的 expires_at 列及其索引，不删除历史 payload。

插件 `Finish` 无论成功失败都主动断开采集器对请求、响应和 SSE 工作缓冲区的引用，并拒绝迟到的数据。Core 同时清空 Gin 池化 context 中的 trace 引用。成功请求的 trace 不落盘，缓冲区随后由 Go GC 回收；解除引用不等于操作系统工作集立即下降。

失败请求的诊断快照仍归 Core 异步队列所有，直到编码、落库或丢弃完成。队列容量 32 条，原始正文预算 512 MiB，满载时非阻塞降级为仅记录事件。该预算不是进程内存上限：并发请求、序列化临时分配及压缩器工作区另计。压缩器复用时会先解除对输出 payload 的引用。

开启 trace 会增加正文复制、SSE 观察和失败诊断脱敏开销；关闭时走传输层快速返回。基准方法和结果见 [性能测试报告](request-trace-performance.md)。

## 指纹一致性

Core 入口与插件出站统一使用 SDK 的 xxh3-128 实现，输出 32 位小写十六进制。HeaderFingerprints 统一字段别名和优先级，多值使用 NUL 分隔且保留值顺序；落盘键名分别为 x-airgate-trace-session-id-xxh3-128、x-airgate-trace-conversation-id-xxh3-128、x-airgate-trace-x-codex-turn-state-xxh3-128。同一原始值在两侧可直接比较。

已落盘的历史 trace 不重写：旧 SHA256 摘要无法反推原值，且修改 payload 会改变内容 hash。统一规则对更新后的 Core 与插件生成的新 trace 生效。

增加新的主模型请求模式时，使用 `buildForwardHTTPClient` 或主请求 WebSocket 入口并传递当前 context。辅助网络操作使用普通客户端，不加入 trace；无需引入错误分类、根因选择或新的 trace 数据结构。
