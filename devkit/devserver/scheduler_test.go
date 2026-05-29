package devserver

import (
	"path/filepath"
	"strconv"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestSchedulerReportResultCoolsDownAccountUnavailable(t *testing.T) {
	t.Parallel()

	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	account := store.Create(DevAccount{Name: "oauth", AccountType: "oauth"})
	accountKey := strconv.FormatInt(account.ID, 10)
	s := NewScheduler(store, ScheduleWeightedRR)
	s.ReportResult(account.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountUnavailable})

	status := s.Status()
	cooldowns, ok := status["cooldowns"].(map[string]string)
	if !ok {
		t.Fatalf("cooldowns has type %T, want map[string]string", status["cooldowns"])
	}
	if cooldowns[accountKey] == "" {
		t.Fatalf("expected account %s to be in cooldown, got %v", accountKey, cooldowns)
	}

	s.ReportResult(account.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess})
	status = s.Status()
	cooldowns = status["cooldowns"].(map[string]string)
	if cooldowns[accountKey] != "" {
		t.Fatalf("expected success to clear cooldown, got %v", cooldowns)
	}
}

func TestSchedulerReportResultCoolsDownAccountUnavailableForAPIKey(t *testing.T) {
	t.Parallel()

	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	account := store.Create(DevAccount{Name: "api key", AccountType: "apikey"})
	accountKey := strconv.FormatInt(account.ID, 10)
	s := NewScheduler(store, ScheduleWeightedRR)
	s.ReportResult(account.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountUnavailable})

	status := s.Status()
	cooldowns, ok := status["cooldowns"].(map[string]string)
	if !ok {
		t.Fatalf("cooldowns has type %T, want map[string]string", status["cooldowns"])
	}
	if cooldowns[accountKey] == "" {
		t.Fatalf("expected api key account to enter cooldown, got %v", cooldowns)
	}
}
