package requesttrace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestEvictionDoesNotHoldCollectorLockDuringRelease(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	var evicted *Exchange
	for i := 0; i < maxRequests; i++ {
		e := Record(ctx, sdk.OutboundRequestDiagnostic{URL: fmt.Sprint(i)})
		if i == 1 {
			evicted = e
		}
	}
	evicted.mu.Lock()
	unlock := sync.OnceFunc(evicted.mu.Unlock)
	defer unlock()
	recorded := make(chan struct{})
	go func() {
		Record(ctx, sdk.OutboundRequestDiagnostic{URL: "new"})
		close(recorded)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		published := false
		if capture.mu.TryLock() {
			published = capture.exchanges[maxRequests-1].request.URL == "new"
			capture.mu.Unlock()
		}
		if published {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("eviction blocked the collector bookkeeping lock")
		}
		time.Sleep(time.Millisecond)
	}
	finished := make(chan struct{})
	go func() { capture.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil); close(finished) }()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Finish waited for the detached exchange")
	}
	unlock()
	select {
	case <-recorded:
	case <-time.After(3 * time.Second):
		t.Fatal("retired exchange was not released")
	}
	if !evicted.closed {
		t.Fatal("retired exchange accepted late data")
	}
}

func TestRequestCaptureIsInitializedBeforePublication(t *testing.T) {
	ctx, capture := Start(t.Context(), true)
	sent := &bodyBuffer{}
	sent.write([]byte("before-finish"))
	e := record(ctx, sdk.OutboundRequestDiagnostic{BodyOriginalSize: 13}, true, sent)
	if e == nil {
		t.Fatal("exchange was not published")
	}
	// Snapshot can run as soon as record returns; no post-publication setter is
	// necessary to find the buffer or the original length.
	snapshot := capture.Snapshot()
	if snapshot == nil || len(snapshot.OutboundRequests) != 1 || string(snapshot.OutboundRequests[0].Body) != "before-finish" {
		t.Fatal("published exchange was not fully initialized")
	}
	capture.Finish(&sdk.ForwardOutcome{Kind: sdk.OutcomeStreamAborted}, context.Canceled)

	// Force the exact ordering: publication -> Finish -> reader binding/use.
	reader := &observedBody{ReadCloser: io.NopCloser(strings.NewReader("late-body")), consume: sent.write}
	raw, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(raw) != "late-body" {
		t.Fatalf("forwarding changed after Finish: %q %v", raw, err)
	}
	if e.sent != nil || e.request.BodyOriginalSize != 0 || !e.closed {
		t.Fatal("released exchange was repopulated")
	}
	if raw, size := sent.snapshot(); len(raw) != 0 || size != 0 {
		t.Fatal("released buffer accepted late body bytes")
	}
}

func TestHTTPBodyCaptureSurvivesConcurrentFinish(t *testing.T) {
	for _, mode := range []string{"replayable", "streaming", "getbody-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx, capture := Start(t.Context(), true)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			original := []byte("unchanged forwarded body")
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fixture.invalid/responses", bytes.NewReader(original))
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "streaming":
				req.GetBody = nil
			case "getbody-failed":
				req.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("fixture snapshot failure") }
			}
			started, resume := make(chan struct{}), make(chan struct{})
			resumeWorker := sync.OnceFunc(func() { close(resume) })
			defer resumeWorker()
			transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				close(started)
				<-resume // The caller finalizes while the worker still owns this reader.
				raw, err := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if err != nil {
					return nil, err
				}
				if !bytes.Equal(raw, original) {
					return nil, fmt.Errorf("forwarded body changed: %q", raw)
				}
				return nil, r.Context().Err()
			})}
			done := make(chan error, 1)
			go func() { _, err := transport.RoundTrip(req); done <- err }()
			<-started
			capture.mu.Lock()
			e := capture.exchanges[0]
			capture.mu.Unlock()
			e.mu.Lock()
			sent, size := e.sent, e.request.BodyOriginalSize
			e.mu.Unlock()
			if size != int64(len(original)) || (sent != nil) != (mode != "replayable") {
				t.Fatal("transport published incomplete capture state")
			}
			cancel()
			outcome := sdk.ForwardOutcome{Kind: sdk.OutcomeStreamAborted}
			capture.Finish(&outcome, context.Canceled)
			resumeWorker()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("late worker result: %v", err)
			}
			e.mu.Lock()
			cleared := e.closed && e.sent == nil && e.response == nil && e.request.BodyOriginalSize == 0
			e.mu.Unlock()
			if !cleared {
				t.Fatal("late worker mutated released exchange")
			}
			if sent != nil {
				sent.mu.Lock()
				released := sent.closed && sent.size == 0 && sent.data == nil
				sent.mu.Unlock()
				if !released {
					t.Fatal("late worker repopulated request buffer")
				}
			}
			if outcome.FinalErrorDiagnostic == nil || len(outcome.FinalErrorDiagnostic.OutboundRequests) != 1 {
				t.Fatal("final diagnostic was lost")
			}
		})
	}
}
