package sdk

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"bogus", slog.LevelInfo},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := parseLevel(tc.input); got != tc.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLogFormatDefaultAndInitLogger(t *testing.T) {
	formatMu.Lock()
	currentFormat = ""
	formatMu.Unlock()
	if got := LogFormat(); got != "text" {
		t.Fatalf("default LogFormat() = %q, want text", got)
	}

	InitLogger("module-test", "debug", "json")
	if got := LogFormat(); got != "json" {
		t.Fatalf("LogFormat after json InitLogger = %q", got)
	}

	InitLogger("module-test", "debug", "")
	if got := LogFormat(); got != "text" {
		t.Fatalf("LogFormat after empty-format InitLogger = %q", got)
	}
}

func TestContextLoggerAndRequestID(t *testing.T) {
	//nolint:staticcheck // This test verifies nil context fallback behavior.
	if LoggerFromContext(nil) == nil {
		t.Fatal("LoggerFromContext(nil) returned nil")
	}
	//nolint:staticcheck // This test verifies nil context fallback behavior.
	if RequestIDFromContext(nil) != "" {
		t.Fatal("RequestIDFromContext(nil) should be empty")
	}

	ctx := context.Background()
	if got := WithLogger(ctx, nil); got != ctx {
		t.Fatal("WithLogger should return the original context for nil logger")
	}
	if got := WithRequestID(ctx, ""); got != ctx {
		t.Fatal("WithRequestID should return the original context for empty id")
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	ctx = WithLogger(ctx, logger)
	if LoggerFromContext(ctx) != logger {
		t.Fatal("LoggerFromContext did not return the context logger")
	}
	ctx = WithRequestID(ctx, "rid-1")
	if got := RequestIDFromContext(ctx); got != "rid-1" {
		t.Fatalf("RequestIDFromContext = %q, want rid-1", got)
	}

	next, derived := LoggerWithRequestID(ctx)
	if derived == nil || LoggerFromContext(next) != derived {
		t.Fatal("LoggerWithRequestID should install derived logger in context")
	}
	derived.Info("request-scoped")
	if !strings.Contains(buf.String(), `request_id=rid-1`) {
		t.Fatalf("derived logger output missing request_id: %q", buf.String())
	}
}

func TestLoggerWithRequestIDGeneratesWhenMissing(t *testing.T) {
	ctx, logger := LoggerWithRequestID(context.Background())
	if logger == nil {
		t.Fatal("LoggerWithRequestID returned nil logger")
	}
	if got := RequestIDFromContext(ctx); got == "" {
		t.Fatal("expected generated request id")
	}
}

func TestExtractOrGenerateRequestID(t *testing.T) {
	headers := http.Header{}
	headers.Set(HeaderRequestID, "  req-123  ")
	if got := ExtractOrGenerateRequestID(headers); got != "req-123" {
		t.Fatalf("ExtractOrGenerateRequestID existing = %q", got)
	}
	if got := ExtractOrGenerateRequestID(http.Header{}); got == "" {
		t.Fatal("expected generated id for empty headers")
	}
	if got := ExtractOrGenerateRequestID(nil); got == "" {
		t.Fatal("expected generated id for nil headers")
	}
}

func TestPrettyHandlerEnabledAndHandlePlain(t *testing.T) {
	var buf bytes.Buffer
	handler := newPrettyHandler(&buf, slog.LevelInfo, false)
	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled below info threshold")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn should be enabled")
	}

	record := slog.NewRecord(time.Date(2026, 6, 20, 1, 2, 3, 4_000_000, time.UTC), slog.LevelWarn, "hello world", 0)
	record.AddAttrs(
		slog.String(LogFieldRequestID, "rid with space"),
		slog.Int64("int", -5),
		slog.Uint64("uint", 6),
		slog.Float64("float", 1.25),
		slog.Bool("bool", true),
		slog.Duration("duration", 1500*time.Millisecond),
		slog.Time("time", time.Unix(10, 0).UTC()),
		slog.Any("err", errors.New("boom")),
		slog.Any("slice", []int{1, 2}),
	)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"01:02:03.004 WARN  hello world",
		`request_id="rid with space"`,
		"int=-5",
		"uint=6",
		"float=1.25",
		"bool=true",
		"duration=1.5s",
		"time=1970-01-01T00:00:10Z",
		"err=boom",
		`slice="[1 2]"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pretty output missing %q: %q", want, out)
		}
	}
}

func TestPrettyHandlerWithAttrsWithGroupAndColor(t *testing.T) {
	var buf bytes.Buffer
	handler := newPrettyHandler(&buf, slog.LevelDebug, true)
	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("module", "sdk")})
	if withAttrs == handler {
		t.Fatal("WithAttrs should return a cloned handler when attrs are provided")
	}
	if handler.WithAttrs(nil) != handler {
		t.Fatal("WithAttrs(nil) should return original handler")
	}
	if withAttrs.WithGroup("ignored") != withAttrs {
		t.Fatal("WithGroup should return the same handler")
	}

	record := slog.NewRecord(time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC), slog.LevelError, "failed", 0)
	record.AddAttrs(slog.String(LogFieldRequestID, "rid"), slog.Any("err", errors.New("bad thing")))
	if err := withAttrs.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := buf.String()
	for _, want := range []string{ansiBoldRed, ansiCyan, ansiRed, "module", "sdk", "failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("colored pretty output missing %q: %q", want, out)
		}
	}
}

func TestPrettyHelpers(t *testing.T) {
	for _, tc := range []struct {
		level slog.Level
		label string
		color string
	}{
		{slog.LevelDebug, "DEBUG", ansiBoldCyan},
		{slog.LevelInfo, "INFO ", ansiBoldGreen},
		{slog.LevelWarn, "WARN ", ansiBoldYellow},
		{slog.LevelError, "ERROR", ansiBoldRed},
		{slog.LevelError + 4, "ERROR", ansiBoldRed},
	} {
		if got := levelLabel(tc.level); got != tc.label {
			t.Fatalf("levelLabel(%v) = %q, want %q", tc.level, got, tc.label)
		}
		if got := levelColor(tc.level); got != tc.color {
			t.Fatalf("levelColor(%v) = %q, want %q", tc.level, got, tc.color)
		}
	}

	for _, tc := range []struct {
		input string
		want  string
	}{
		{"", `""`},
		{"plain", "plain"},
		{"has space", `"has space"`},
		{`has"quote`, `"has\"quote"`},
		{"k=v", `"k=v"`},
	} {
		if got := quoteIfNeeded(tc.input); got != tc.want {
			t.Fatalf("quoteIfNeeded(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}

	var b strings.Builder
	writeAttr(&b, slog.Attr{}, false)
	if b.Len() != 0 {
		t.Fatalf("empty attr should not write, got %q", b.String())
	}
	writeValue(&b, slog.GroupValue(slog.String("nested", "value")), false, "group")
	if !strings.Contains(b.String(), "nested") {
		t.Fatalf("default/group value output = %q", b.String())
	}
}

func TestShouldColorEnvironmentBranches(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if shouldColor(nil) {
		t.Fatal("NO_COLOR should disable color even for nil file")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if shouldColor(nil) {
		t.Fatal("TERM=dumb should disable color even for nil file")
	}

	t.Setenv("TERM", "xterm")
	file, err := os.CreateTemp(t.TempDir(), "not-a-tty")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()
	if shouldColor(file) {
		t.Fatal("regular file should not be treated as a color-capable TTY")
	}
}
