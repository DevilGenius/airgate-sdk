package requesttrace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type benchmarkBody struct {
	reader *bytes.Reader
	chunk  int
}

func (r *benchmarkBody) Read(p []byte) (int, error) { return r.reader.Read(p[:min(len(p), r.chunk)]) }
func (*benchmarkBody) Close() error                 { return nil }

func benchmarkRequest(size int) []byte {
	return []byte(`{"model":"gpt-test","input":[{"role":"user","content":"` + strings.Repeat("trace-history ", size/14) + `"}]}`)
}

// These benchmarks isolate local CPU/allocation overhead, with no upstream
// latency or DB I/O. Baseline and observed paths consume identical bytes.
func BenchmarkTraceHTTP(b *testing.B) {
	for _, size := range []int{4 << 10, 64 << 10, 1 << 20, 8 << 20} {
		body := benchmarkRequest(size)
		for _, mode := range []string{"baseline", "off", "on_success", "on_failure"} {
			b.Run(fmt.Sprintf("%dKiB/%s", size>>10, mode), func(b *testing.B) {
				failure := mode == "on_failure"
				status := http.StatusOK
				response := []byte(`{"id":"resp_fixture","status":"completed"}`)
				if failure {
					status = 422
					response = []byte(`{"detail":[{"msg":"invalid parameter"}]}`)
				}
				base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
					_, _ = io.Copy(io.Discard, req.Body)
					_ = req.Body.Close()
					return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: &benchmarkBody{bytes.NewReader(response), 4096}}, nil
				})
				var transport http.RoundTripper = base
				if mode != "baseline" {
					transport = &Transport{Base: base}
				}
				b.ReportAllocs()
				b.SetBytes(int64(len(body)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					ctx, capture := Start(context.Background(), strings.HasPrefix(mode, "on_"))
					req, _ := http.NewRequestWithContext(ctx, "POST", "https://fixture.invalid/responses", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					resp, err := transport.RoundTrip(req)
					if err != nil {
						b.Fatal(err)
					}
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
					kind := sdk.OutcomeSuccess
					if failure {
						kind = sdk.OutcomeClientError
					}
					outcome := sdk.ForwardOutcome{Kind: kind}
					capture.Finish(&outcome, nil)
					runtime.KeepAlive(outcome)
				}
			})
		}
	}
}

func BenchmarkTraceSSE(b *testing.B) {
	request := benchmarkRequest(64 << 10)
	for _, events := range []int{100, 1000} {
		wire := []byte(strings.Repeat("data: "+`{"type":"response.output_text.delta","delta":"hello world"}`+"\n\n", events) + "data: " + `{"type":"response.completed","response":{"status":"completed"}}` + "\n\n")
		for _, enabled := range []bool{false, true} {
			b.Run(fmt.Sprintf("%dEvents/on=%t", events, enabled), func(b *testing.B) {
				transport := &Transport{Base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					_, _ = io.Copy(io.Discard, req.Body)
					_ = req.Body.Close()
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: &benchmarkBody{bytes.NewReader(wire), 1024}}, nil
				})}
				b.ReportAllocs()
				b.SetBytes(int64(len(wire)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					ctx, capture := Start(context.Background(), enabled)
					req, _ := http.NewRequestWithContext(ctx, "POST", "https://fixture.invalid/responses", bytes.NewReader(request))
					req.Header.Set("Content-Type", "application/json")
					resp, err := transport.RoundTrip(req)
					if err != nil {
						b.Fatal(err)
					}
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
					outcome := sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}
					capture.Finish(&outcome, nil)
				}
			})
		}
	}
}

func BenchmarkTraceContextLookup(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		b.Run(fmt.Sprintf("on=%t", enabled), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctx, c := Start(context.Background(), enabled)
				runtime.KeepAlive(fromContext(ctx))
				c.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil)
			}
		})
	}
}

func BenchmarkTraceRepresentativeFailure(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("%dKiB", size>>10), func(b *testing.B) {
			body, _ := json.Marshal(map[string]any{"model": "gpt-test", "input": []any{map[string]any{"role": "user", "content": strings.Repeat("检查中文上下文与 JSON 工具调用 {\"query\":\"sample\"}\n", size/72)}, map[string]any{"type": "function_call_output", "call_id": "call_test", "output": "{\"status\":\"ok\"}"}}})
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, c := Start(context.Background(), true)
				e := Record(ctx, sdk.OutboundRequestDiagnostic{Transport: "http", Headers: http.Header{"Content-Type": {"application/json"}}, Body: body})
				e.ObserveEvent([]byte(`{"error":{"message":"invalid"}}`))
				o := sdk.ForwardOutcome{Kind: sdk.OutcomeClientError}
				c.Finish(&o, nil)
				runtime.KeepAlive(o)
			}
		})
	}
}

// Keep the parent context and consumed response alive, like a pooled caller.
// This measures reachable retained buffers, not an OS working-set guarantee.
func TestTraceRetainedHeapProfile(t *testing.T) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	ctx, capture, resp := retainedTraceFixture(t)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("retained_heap_bytes=%d capture_entries=%d", int64(after.HeapAlloc)-int64(before.HeapAlloc), len(capture.exchanges))
	runtime.KeepAlive(ctx)
	runtime.KeepAlive(resp)
}

func retainedTraceFixture(t *testing.T) (context.Context, *Capture, *http.Response) {
	t.Helper()
	body := benchmarkRequest(8 << 20)
	ctx, capture := Start(context.Background(), true)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://fixture.invalid/responses", bytes.NewReader(body))
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: &benchmarkBody{bytes.NewReader([]byte("data: {}\n\n")), 1024}}, nil
	})}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	capture.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil)
	return ctx, capture, resp
}
