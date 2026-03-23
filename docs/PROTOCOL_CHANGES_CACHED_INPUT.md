# 协议字段变更说明

本次协议对齐 `openai/codex` 与 OpenAI 官方 usage 口径。

## ForwardResult
新增：
- `cached_input_tokens`
- `service_tier`
- `updated_credentials`（兼容 core 已有逻辑）

语义：
- `input_tokens`: 普通输入 token
- `cached_input_tokens`: 命中缓存的输入 token
- `output_tokens`: 输出 token

不再推荐 OpenAI 路径使用：
- `cache_creation_input_tokens`
- `cache_read_input_tokens`

## ModelInfo
新增：
- `cached_input_price`
- `input_price_priority`
- `output_price_priority`
- `cached_input_price_priority`

语义：
- `cached_input_price` 对齐 OpenAI 官方 Cached input 定价
- `*_priority` 用于 priority service tier 定价

## 插件实现要求
OpenAI/Codex 类插件应：
- 优先从上游 usage 中提取 `input_tokens_details.cached_tokens` 或等价字段
- 统一回传 `cached_input_tokens`
- 如请求携带 `service_tier`，应同步回传 `service_tier`

## core 侧要求
core 计费统一按三段：
- input
- cached input
- output

兼容字段 `cache_tokens` / `cache_cost` 可在内部暂保留，但对外接口建议统一使用 `cached_input_*` 命名。
