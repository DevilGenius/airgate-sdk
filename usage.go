package sdk

import "time"

// AccountUsageWindow 描述账号的单个用量窗口。
// 插件负责把平台专属窗口语义归一化到这个结构，Core 只做通用展示。
type AccountUsageWindow struct {
	Key          string  `json:"key,omitempty"`
	Label        string  `json:"label"`
	UsedPercent  float64 `json:"used_percent"`
	ResetAt      string  `json:"reset_at,omitempty"`
	ResetSeconds int     `json:"reset_seconds,omitempty"`
}

// AccountUsageCredits 描述账号的额度信息。
type AccountUsageCredits struct {
	Balance   float64 `json:"balance"`
	Unlimited bool    `json:"unlimited"`
}

// AccountUsageInfo 描述单个账号的通用用量视图。
type AccountUsageInfo struct {
	UpdatedAt string               `json:"updated_at,omitempty"`
	Windows   []AccountUsageWindow `json:"windows,omitempty"`
	Credits   *AccountUsageCredits `json:"credits,omitempty"`
}

// AccountUsageError 描述插件在探测账号用量时发现的单账号错误。
type AccountUsageError struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

// AccountUsageAccountsResponse 是 usage/accounts 之类账号批量用量接口的通用响应。
type AccountUsageAccountsResponse struct {
	Accounts map[string]AccountUsageInfo `json:"accounts"`
	Errors   []AccountUsageError         `json:"errors,omitempty"`
}

// ResetAtFromBase 根据基准时间和 reset_after_seconds 计算绝对重置时间。
// 负数会被钳制为 0。
func ResetAtFromBase(base time.Time, resetAfterSeconds int) *time.Time {
	if base.IsZero() {
		base = time.Now()
	}
	sec := resetAfterSeconds
	if sec < 0 {
		sec = 0
	}
	resetAt := base.UTC().Add(time.Duration(sec) * time.Second)
	return &resetAt
}

// RemainingSecondsUntil 返回从 now 到 resetAt 的剩余秒数。
// 过期或 nil 一律返回 0。
func RemainingSecondsUntil(resetAt *time.Time, now time.Time) int {
	if resetAt == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	diff := int(resetAt.UTC().Sub(now.UTC()).Seconds())
	if diff < 0 {
		return 0
	}
	return diff
}

// NewAccountUsageWindow 构建通用用量窗口，并同时填充 reset_at / reset_seconds。
func NewAccountUsageWindow(key, label string, usedPercent float64, resetAt *time.Time, now time.Time) AccountUsageWindow {
	window := AccountUsageWindow{
		Key:         key,
		Label:       label,
		UsedPercent: usedPercent,
	}
	if resetAt != nil {
		window.ResetAt = resetAt.UTC().Format(time.RFC3339)
		window.ResetSeconds = RemainingSecondsUntil(resetAt, now)
	}
	return window
}
