package sdk

import "errors"

// ErrNotSupported 插件不支持某项可选能力时返回（如 HandleWebSocket）。
var ErrNotSupported = errors.New("not supported")

// ErrInvalidCredentials ValidateAccount 判定凭证格式/语义不合法时返回。
var ErrInvalidCredentials = errors.New("invalid credentials")
