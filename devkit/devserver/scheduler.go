package devserver

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// SchedulePolicy 调度策略
type SchedulePolicy string

const (
	ScheduleNone       SchedulePolicy = "none"        // 直连指定账号（默认第一个）
	ScheduleWeightedRR SchedulePolicy = "weighted_rr" // 加权轮询 + failover
)

// Scheduler 账号调度器
type Scheduler struct {
	mu       sync.Mutex
	policy   SchedulePolicy
	store    *AccountStore
	cooldown map[int64]time.Time // 账号ID → 冷却截止时间
	counter  uint64              // 轮询计数器
	pinnedID int64               // 直连模式指定的账号ID，0 表示使用第一个
}

// NewScheduler 创建调度器
func NewScheduler(store *AccountStore, policy SchedulePolicy) *Scheduler {
	if policy == "" {
		policy = ScheduleNone
	}
	return &Scheduler{
		policy:   policy,
		store:    store,
		cooldown: make(map[int64]time.Time),
	}
}

// Select 根据策略选择账号
func (s *Scheduler) Select() *DevAccount {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.policy == ScheduleNone {
		if s.pinnedID > 0 {
			if a := s.store.Get(s.pinnedID); a != nil {
				return a
			}
		}
		return s.store.First()
	}

	return s.selectWeightedRR()
}

// selectWeightedRR 加权轮询选择（调用前已持锁）
func (s *Scheduler) selectWeightedRR() *DevAccount {
	accounts := s.store.List()
	if len(accounts) == 0 {
		return nil
	}

	now := time.Now()

	// 构建可用账号列表（排除冷却中的）和加权展开
	type slot struct {
		account DevAccount
	}
	var slots []slot
	for _, a := range accounts {
		if coolUntil, ok := s.cooldown[a.ID]; ok && now.Before(coolUntil) {
			continue // 冷却中，跳过
		}
		// 清理已过期的冷却
		delete(s.cooldown, a.ID)

		w := a.Weight
		if w <= 0 {
			w = 1
		}
		for range w {
			slots = append(slots, slot{account: a})
		}
	}

	if len(slots) == 0 {
		// 所有账号都在冷却，回退到第一个
		cp := accounts[0]
		return &cp
	}

	idx := s.counter % uint64(len(slots))
	s.counter++
	cp := slots[idx].account
	return &cp
}

// ReportResult 上报转发结果，用于标记冷却。
func (s *Scheduler) ReportResult(accountID int64, outcome sdk.ForwardOutcome) {
	if s.policy == ScheduleNone {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch outcome.Kind {
	case sdk.OutcomeAccountRateLimited:
		cooldown := outcome.RetryAfter
		if cooldown <= 0 {
			cooldown = 60 * time.Second
		}
		s.cooldown[accountID] = time.Now().Add(cooldown)
		log.Printf("[调度] 账号 %d 被限流，冷却 %s", accountID, cooldown)

	case sdk.OutcomeAccountDead:
		s.cooldown[accountID] = time.Now().Add(5 * time.Minute)
		log.Printf("[调度] 账号 %d 凭证失效，冷却 5 分钟", accountID)

	case sdk.OutcomeAccountUnavailable:
		s.cooldown[accountID] = time.Now().Add(60 * time.Second)
		log.Printf("[调度] 账号 %d 暂时 403，冷却 60 秒", accountID)

	default:
		delete(s.cooldown, accountID)
	}
}

// IsRetryable 判断转发结果是否可 failover。
func (s *Scheduler) IsRetryable(outcome sdk.ForwardOutcome, err error) bool {
	if s.policy == ScheduleNone {
		return false
	}
	if err != nil {
		return true
	}
	return outcome.Kind.ShouldFailover()
}

// SetPolicy 运行时切换策略
func (s *Scheduler) SetPolicy(policy SchedulePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy
	log.Printf("[调度] 策略切换为 %s", policy)
}

// Policy 返回当前策略
func (s *Scheduler) Policy() SchedulePolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

// SetPinned 设置直连模式使用的账号，0 表示使用第一个
func (s *Scheduler) SetPinned(accountID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pinnedID = accountID
	log.Printf("[调度] 直连账号设为 %d", accountID)
}

// Status 返回调度状态快照
func (s *Scheduler) Status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cooldowns := make(map[string]string)
	for id, until := range s.cooldown {
		if now.Before(until) {
			cooldowns[strconv.FormatInt(id, 10)] = until.Sub(now).Truncate(time.Second).String()
		}
	}

	return map[string]any{
		"policy":    s.policy,
		"cooldowns": cooldowns,
		"pinned_id": s.pinnedID,
	}
}

// ──────────────────────────────────────────────────────
// HTTP API
// ──────────────────────────────────────────────────────

// SchedulerHandler 调度管理 API
type SchedulerHandler struct {
	scheduler *Scheduler
	store     *AccountStore
}

func (h *SchedulerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/scheduler")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		h.getStatus(w)
	case path == "policy" && r.Method == http.MethodPut:
		h.setPolicy(w, r)
	case path == "pinned" && r.Method == http.MethodPut:
		h.setPinned(w, r)
	case strings.HasPrefix(path, "weight/") && r.Method == http.MethodPut:
		idStr := strings.TrimPrefix(path, "weight/")
		h.setWeight(w, r, idStr)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (h *SchedulerHandler) getStatus(w http.ResponseWriter) {
	status := h.scheduler.Status()

	// 附加账号权重信息
	accounts := h.store.List()
	weights := make(map[string]int)
	for _, a := range accounts {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		weights[strconv.FormatInt(a.ID, 10)+" ("+a.Name+")"] = w
	}
	status["weights"] = weights

	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("写入调度状态响应失败: %v", err)
	}
}

func (h *SchedulerHandler) setPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Policy SchedulePolicy `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	switch req.Policy {
	case ScheduleNone, ScheduleWeightedRR:
		h.scheduler.SetPolicy(req.Policy)
		if err := json.NewEncoder(w).Encode(map[string]string{"policy": string(req.Policy)}); err != nil {
			log.Printf("写入策略切换响应失败: %v", err)
		}
	default:
		http.Error(w, `{"error":"unknown policy, use: none, weighted_rr"}`, http.StatusBadRequest)
	}
}

func (h *SchedulerHandler) setPinned(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64 `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	// account_id=0 表示恢复为"第一个账号"
	if req.AccountID > 0 {
		if a := h.store.Get(req.AccountID); a == nil {
			http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
			return
		}
	}
	h.scheduler.SetPinned(req.AccountID)
	if err := json.NewEncoder(w).Encode(map[string]any{"pinned_id": req.AccountID}); err != nil {
		log.Printf("写入固定账号响应失败: %v", err)
	}
}

func (h *SchedulerHandler) setWeight(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Weight int `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Weight < 0 {
		http.Error(w, `{"error":"weight must be >= 0"}`, http.StatusBadRequest)
		return
	}

	account := h.store.Get(id)
	if account == nil {
		http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		return
	}

	account.Weight = req.Weight
	h.store.Update(id, *account)
	log.Printf("[调度] 账号 %d 权重设为 %d", id, req.Weight)

	if err := json.NewEncoder(w).Encode(map[string]any{"id": id, "weight": req.Weight}); err != nil {
		log.Printf("写入权重设置响应失败: %v", err)
	}
}
