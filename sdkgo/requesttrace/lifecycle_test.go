package requesttrace

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestFinishReleasesContextAndResponseBuffers(t *testing.T) {
	for _, failed := range []bool{false, true} {
		for _, sse := range []bool{false, true} {
			ctx, capture := Start(t.Context(), true)
			original := []byte(`{"input":"keep full request"}`)
			e := Record(ctx, sdk.OutboundRequestDiagnostic{Headers: http.Header{"Content-Type": {"application/json"}}, Body: original})
			wire := "raw response"
			contentType := "text/plain"
			if sse {
				wire = "data: raw response\n\n"
				contentType = "text/event-stream"
			}
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(wire))}
			e.WrapResponse(resp)
			events, buffer := e.events, e.response
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			kind := sdk.OutcomeSuccess
			if failed {
				kind = sdk.OutcomeClientError
			}
			outcome := sdk.ForwardOutcome{Kind: kind}
			capture.Finish(&outcome, nil)
			if !capture.closed || len(capture.exchanges) != 0 || !e.closed || len(e.request.Body) != 0 || len(e.event) != 0 || e.response != nil || e.events != nil {
				t.Fatal("completed request retained buffers")
			}
			if events != nil && (len(events.line) != 0 || cap(events.line) != 0 || cap(events.data) != 0 || !events.closed) {
				t.Fatal("SSE observer retained scratch buffers")
			}
			if buffer != nil && (cap(buffer.data) != 0 || !buffer.closed) {
				t.Fatal("response retained body buffer")
			}
			if failed {
				d := outcome.FinalErrorDiagnostic
				if d == nil || !bytes.Equal(d.OutboundRequests[0].Body, original) || string(d.UpstreamErrorBody) != "raw response" {
					t.Fatal("release invalidated failed diagnostic")
				}
			} else if outcome.FinalErrorDiagnostic != nil {
				t.Fatal("success produced diagnostic")
			}
			e.ObserveEvent([]byte("late event"))
			if len(e.event) != 0 || Record(ctx, sdk.OutboundRequestDiagnostic{Body: original}) != nil {
				t.Fatal("finished trace accepted late data")
			}
		}
	}
}

func TestReplacedExchangeReleasesBuffers(t *testing.T) {
	ctx, c := Start(t.Context(), true)
	Record(ctx, sdk.OutboundRequestDiagnostic{Body: []byte("first")})
	evicted := Record(ctx, sdk.OutboundRequestDiagnostic{Body: make([]byte, 1<<20)})
	for i := 0; i < maxRequests; i++ {
		Record(ctx, sdk.OutboundRequestDiagnostic{Body: []byte("retry")})
	}
	if !evicted.closed || evicted.request.Body != nil {
		t.Fatal("evicted exchange retained body")
	}
	c.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil)
}

func TestFinishDuringResponseRead(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/event-stream"} {
		for i := 0; i < 20; i++ {
			ctx, c := Start(t.Context(), true)
			e := Record(ctx, sdk.OutboundRequestDiagnostic{Body: []byte("request")})
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(strings.Repeat("data: {}\n\n", 1000)))}
			e.WrapResponse(resp)
			done := make(chan error, 1)
			go func() { _, err := io.Copy(io.Discard, resp.Body); _ = resp.Body.Close(); done <- err }()
			c.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if len(e.event) != 0 || e.request.Body != nil {
				t.Fatal("read repopulated a finished trace")
			}
		}
	}
}
