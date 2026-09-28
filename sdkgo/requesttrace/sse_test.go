package requesttrace

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestSSELineEndingsAcrossEveryReadBoundary(t *testing.T) {
	for _, endings := range [][]string{{"\n", "\n", "\n", "\n"}, {"\r", "\r", "\r", "\r"}, {"\r\n", "\r\n", "\r\n", "\r\n"}, {"\r\n", "\n", "\r", "\r"}} {
		wire := "data: first" + endings[0] + endings[1] + "data: last" + endings[2] + endings[3]
		for split := 0; split <= len(wire); split++ {
			t.Run(fmt.Sprintf("%q/split=%d", endings, split), func(t *testing.T) {
				ctx, capture := Start(t.Context(), true)
				defer capture.release()
				exchange := Record(ctx, sdk.OutboundRequestDiagnostic{Transport: "http"})
				observer := &eventObserver{exchange: exchange}
				observer.write([]byte(wire[:split]))
				observer.write([]byte(wire[split:]))
				if got := string(capture.Snapshot().UpstreamErrorBody); got != "last" {
					t.Fatalf("last event before EOF = %q", got)
				}
				observer.finish()
				if got := string(capture.Snapshot().UpstreamErrorBody); got != "last" {
					t.Fatalf("EOF changed last event: %q", got)
				}
			})
		}
	}
}

func TestSSECRMultilineTerminatorsAndForwarding(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	defer capture.release()
	exchange := Record(ctx, sdk.OutboundRequestDiagnostic{Transport: "http"})
	wire := ": heartbeat\rdata: first\rdata: second\r\rdata: [DONE]\r\r"
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(oneByteReader{strings.NewReader(wire)})}
	exchange.WrapResponse(resp)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || string(raw) != wire {
		t.Fatalf("SSE forwarding changed: %q / %v", raw, err)
	}
	if got := string(capture.Snapshot().UpstreamErrorBody); got != "first\nsecond" {
		t.Fatalf("multiline payload overwritten by terminator: %q", got)
	}
}

func TestRawEventCollectorPreservesLiteralDone(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	defer capture.release()
	exchange := Record(ctx, sdk.OutboundRequestDiagnostic{Transport: "websocket"})
	exchange.ObserveEvent([]byte("previous"))
	exchange.ObserveEvent([]byte("[DONE]"))
	if got := string(capture.Snapshot().UpstreamErrorBody); got != "[DONE]" {
		t.Fatalf("raw collector interpreted a protocol sentinel: %q", got)
	}
}
