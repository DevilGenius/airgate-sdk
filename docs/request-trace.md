# 请求 Trace 的采集与内部传输

SDK 的 `sdkgo/requesttrace` 只采集有界原始诊断，不负责脱敏、持久化、保留期、计费或重试。Core 是落库前脱敏的唯一边界；策略与生命周期见 [Core 请求 Trace 文档](../../airgate-core/docs/request-trace.md)。

## 接入

每次主模型 `Forward` 入口调用 `requesttrace.Start(ctx, req.TraceFinalError)`，出口通过闭包 defer 调用 `capture.Finish(&outcome, err)`，读取最终 outcome 和 err。只有最终失败的诊断交给 Core，成功请求不落库。

主模型 HTTP 转发包装 `requesttrace.Transport`；WebSocket 使用 `requesttrace.WrapWebSocket`。复制实际发送的 URL、请求头、正文和读取到的响应/最近 SSE 或 WebSocket 数据事件，不改变网络载荷，不预读响应，不创建读协程。具有 GetBody 的请求读取独立 reader；其他请求随读取采集。

图片上传下载、BAS 附件上传、认证请求和辅助轮询不进入 trace。OpenAI 主请求使用 `buildForwardHTTPClient`；普通 `buildHTTPClient`、`imageDownloadHTTPClient` 和 imgen 下载客户端不安装 trace。新增模式复用主请求入口与 context 即可。

HTTP SSE 适配器识别 LF、CRLF、裸 CR 行分隔，包括跨读取边界的 CRLF；只在此适配器忽略流式 API 的 `[DONE]` 结束标记，避免覆盖最近载荷。通用 `ObserveEvent` 与 WebSocket 不解释这个标记，按原始载荷采集。采集与过滤都不会改写转发字节。

## 协议

`OutboundRequestDiagnostic` 只包含七个字段：transport=1、method=2、url=3、headers=4、body=5、status_code=6、body_original_size=7。该协议尚未发布，因此不保留已移除脱敏字段的编号或名称，也不提供兼容字段。

`body_original_size` 是采集元数据：用于区分完整正文与预算不足或读取不完整的正文，不是脱敏标记。SDK 不提供 BodyRedacted、BodyRedactionReason 或可信脱敏绕过。

原始诊断在内存和内部 gRPC 中可能包含凭证、会话标识和图片字节，不能直接记录到日志或数据库。Core 接收后统一清理 Header、URL、正文、错误摘要及事件明细，并在存储前生成 xxh3-128 会话指纹；插件不生成指纹。

## 限额与生命周期

每次插件 attempt 最多保留 8 个出站请求（最早一个及最近七个），诊断正文总预算 48 MiB，单个 SSE 数据事件最多 16 MiB。超过预算或已知采集不完整时省略请求正文并保留原长度；SDK gRPC 消息上限仍然适用。

Finish 无论成功失败都解除采集器对正文和工作缓冲区的引用，并拒绝迟到数据。失败快照的所有权转交 Core 异步队列，直到编码落库或丢弃；释放引用后由 Go GC 回收，不保证操作系统工作集立即下降。

正文/请求头复制在 Capture 锁外进行，发布时再次检查是否已完成；淘汰记录先从 Capture 中移除，再在锁外释放其缓冲区，避免 observer 锁阻塞整个采集器。

采集器原有性能数据见 [历史性能报告](../../airgate-core/docs/request-trace-performance.md)，迁移前的脱敏耗时不代表当前 SDK 热路径。
