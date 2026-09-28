# Trace 历史性能报告

报告随脱敏策略迁移至 [Core 文档](../../airgate-core/docs/request-trace-performance.md)。报告数值来自迁移前的实现，不能用作当前 SDK 热路径或端到端性能结论。

当前 SDK 仅保留原始采集器基准；正文脱敏基准位于 Core 的 `internal/app/monitor/traceredaction` 包。
