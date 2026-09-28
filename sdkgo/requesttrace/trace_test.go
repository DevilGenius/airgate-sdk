package requesttrace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPTracePreservesFullWireRequest(t *testing.T) {
	for _, replayable := range []bool{false, true} {
		t.Run(fmt.Sprintf("get_body=%t", replayable), func(t *testing.T) {
			original := []byte("{\n  \"input\":\"" + strings.Repeat("complete-history ", 8192) + "\",\"custom\":{\"keep\":true}}")
			ctx, capture := Start(t.Context(), true)
			req, err := http.NewRequestWithContext(ctx, "POST", "https://user:secret@example.test/responses?token=secret", bytes.NewReader(original))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer secret")
			req.Header.Set("session_id", "private-session")
			if !replayable {
				req.GetBody = nil
			}
			rawError := `{"detail":"original validation error"}`
			transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				sent, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(sent, original) {
					t.Fatal("transport changed original body")
				}
				_ = r.Body.Close()
				return &http.Response{StatusCode: 422, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(rawError))}, nil
			})}
			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			received, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil || string(received) != rawError {
				t.Fatal("trace changed response")
			}
			req.Header.Set("Content-Type", "changed-after-send")
			expected := bytes.Clone(original)
			for i := range original {
				original[i] = 'x'
			}
			outcome := sdk.ForwardOutcome{Kind: sdk.OutcomeClientError}
			capture.Finish(&outcome, nil)
			trace := outcome.FinalErrorDiagnostic
			if trace == nil || len(trace.OutboundRequests) != 1 || string(trace.UpstreamErrorBody) != rawError {
				t.Fatalf("trace=%+v", trace)
			}
			request := trace.OutboundRequests[0]
			if !bytes.Equal(request.Body, expected) || request.BodyOriginalSize != int64(len(expected)) || request.StatusCode != 422 || request.Headers.Get("Content-Type") != "application/json" {
				t.Fatal("full request was lost or changed")
			}
			if request.Headers.Get("Authorization") != "Bearer secret" || request.Headers.Get("session_id") != "private-session" || request.URL != "https://user:secret@example.test/responses?token=secret" {
				t.Fatal("SDK modified raw diagnostic metadata")
			}
		})
	}
}

type oneByteReader struct{ io.Reader }

func (r oneByteReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:min(1, len(p))]) }

func TestSSEObservationIsIncrementalAndTransparent(t *testing.T) {
	raw := "{\"type\":\"response.failed\",\n\"error\":{\"message\":\"raw upstream\"}}"
	wire := ": heartbeat\r\nevent: error\r\ndata: {\"type\":\"response.failed\",\r\ndata: \"error\":{\"message\":\"raw upstream\"}}\r\n\r\ndata: [DONE]\r\n\r\n"
	for _, chunked := range []bool{false, true} {
		ctx, capture := Start(t.Context(), true)
		exchange := Record(ctx, sdk.OutboundRequestDiagnostic{Transport: "http", Method: "POST", URL: "https://example.test", Body: []byte("request")})
		var reader io.Reader = strings.NewReader(wire)
		if chunked {
			reader = oneByteReader{reader}
		}
		resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream; charset=utf-8"}}, Body: io.NopCloser(reader)}
		exchange.WrapResponse(resp)
		received, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || string(received) != wire {
			t.Fatal("observer modified or consumed SSE bytes")
		}
		if got := string(capture.Snapshot().UpstreamErrorBody); got != raw {
			t.Fatalf("raw payload=%q want=%q", got, raw)
		}
	}
}

func TestSharedTransportDoesNotLeakTraceBetweenRequests(t *testing.T) {
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"bad"}`))}, nil
	})}
	ctxA, a := Start(t.Context(), true)
	ctxB, b := Start(t.Context(), true)
	for i, ctx := range []context.Context{ctxA, t.Context(), ctxB} {
		req, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://example.test/%d", i), strings.NewReader(fmt.Sprintf("body-%d", i)))
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}
	for i, c := range []*Capture{a, b} {
		trace := c.Snapshot()
		if len(trace.OutboundRequests) != 1 || string(trace.OutboundRequests[0].Body) != fmt.Sprintf("body-%d", i*2) {
			t.Fatal("collector leaked across requests")
		}
	}
	success := sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}
	a.Finish(&success, nil)
	if success.FinalErrorDiagnostic != nil {
		t.Fatal("successful request published trace")
	}
	_, disabled := Start(t.Context(), false)
	failure := sdk.ForwardOutcome{Kind: sdk.OutcomeClientError}
	disabled.Finish(&failure, nil)
	if failure.FinalErrorDiagnostic != nil {
		t.Fatal("disabled trace was captured")
	}
}

func TestRetryCaptureDoesNotReusePreviousResponse(t *testing.T) {
	ctx, c := Start(t.Context(), true)
	for i := 0; i < 12; i++ {
		e := Record(ctx, sdk.OutboundRequestDiagnostic{URL: fmt.Sprintf("https://example.test/%d", i), Body: []byte(fmt.Sprint(i))})
		if i < 11 {
			e.ObserveEvent([]byte("old-response"))
		}
	}
	trace := c.Snapshot()
	if len(trace.OutboundRequests) != maxRequests || string(trace.OutboundRequests[0].Body) != "0" || string(trace.OutboundRequests[maxRequests-1].Body) != "11" || len(trace.UpstreamErrorBody) != 0 {
		t.Fatal("retry history or final error ownership is incorrect")
	}
}

type failingBody struct{ closed bool }

var errReadFixture = errors.New("read fixture")

func (*failingBody) Read(p []byte) (int, error) { return copy(p, "partial"), errReadFixture }
func (b *failingBody) Close() error             { b.closed = true; return nil }

func TestResponseReadAndCloseSemantics(t *testing.T) {
	ctx, c := Start(t.Context(), true)
	e := Record(ctx, sdk.OutboundRequestDiagnostic{URL: "https://example.test"})
	body := &failingBody{}
	resp := &http.Response{StatusCode: 502, Header: make(http.Header), Body: body}
	e.WrapResponse(resp)
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !errors.Is(err, errReadFixture) || !body.closed || string(raw) != "partial" || string(c.Snapshot().UpstreamErrorBody) != "partial" {
		t.Fatal("read or close semantics changed")
	}
}

func TestWebSocketTraceCapturesActualFrames(t *testing.T) {
	received := make(chan []byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for i := 0; i < 2; i++ {
			_, body, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received <- body
			_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"error","message":"attempt-%d"}`, i)))
		}
	}))
	defer server.Close()
	ctx, c := Start(t.Context(), true)
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	raw, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn := WrapWebSocket(ctx, raw, url, http.Header{"Authorization": {"secret"}})
	defer func() { _ = conn.Close() }()
	for i := 0; i < 2; i++ {
		if err := conn.WriteJSON(map[string]any{"type": "response.create", "input": strings.Repeat("full history ", 1024), "attempt": i}); err != nil {
			t.Fatal(err)
		}
		_, _, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
	}
	trace := c.Snapshot()
	if len(trace.OutboundRequests) != 2 || string(trace.UpstreamErrorBody) != `{"type":"error","message":"attempt-1"}` {
		t.Fatal("missing WebSocket diagnostics")
	}
	for _, request := range trace.OutboundRequests {
		if !bytes.Equal(request.Body, <-received) || request.Method != "response.create" || request.Transport != "websocket" || request.Headers.Get("Authorization") != "secret" {
			t.Fatal("WebSocket wire snapshot mismatch")
		}
	}
}

func TestIncompleteReaderRequestIsExplicitlyMarked(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://example.test", strings.NewReader("complete-request"))
	req.GetBody = nil
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_, _ = io.ReadFull(r.Body, make([]byte, 3))
		_ = r.Body.Close()
		return nil, io.ErrUnexpectedEOF
	})}
	_, err := transport.RoundTrip(req)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	request := capture.Snapshot().OutboundRequests[0]
	if request.BodyOriginalSize != req.ContentLength || len(request.Body) != 0 {
		t.Fatalf("partial request presented as complete: %+v", request)
	}
}

func TestSizeLimitedRequestRetainsExplicitOmission(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	e := Record(ctx, sdk.OutboundRequestDiagnostic{URL: "https://example.test", BodyOriginalSize: maxBodyBytes + 1})
	e.ObserveEvent([]byte("terminal-error"))
	trace := capture.Snapshot()
	request := trace.OutboundRequests[0]
	if len(request.Body) != 0 || request.BodyOriginalSize != maxBodyBytes+1 || string(trace.UpstreamErrorBody) != "terminal-error" {
		t.Fatalf("lost omission metadata: %+v", trace)
	}
}
