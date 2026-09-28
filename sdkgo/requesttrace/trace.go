// Package requesttrace provides opt-in raw diagnostics for main upstream requests.
// Plugins select traced transports; Core owns redaction and persistence.
package requesttrace

import (
	"bytes"
	"context"
	"sync"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

const (
	maxRequests   = 8
	maxBodyBytes  = 48 << 20
	maxEventBytes = 16 << 20
)

type contextKey struct{}

type Capture struct {
	mu        sync.Mutex
	exchanges []*Exchange
	closed    bool
}

// Start scopes tracing to one Forward attempt. Disabled tracing allocates no collector.
func Start(ctx context.Context, enabled bool) (context.Context, *Capture) {
	if !enabled {
		return ctx, nil
	}
	c := &Capture{}
	return context.WithValue(ctx, contextKey{}, c), c
}

func fromContext(ctx context.Context) *Capture {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(contextKey{}).(*Capture)
	return c
}

// Finish publishes diagnostics only for failed attempts, after all outcome policies.
func (c *Capture) Finish(outcome *sdk.ForwardOutcome, err error) {
	if c == nil {
		return
	}
	defer c.release()
	if outcome == nil || err == nil && outcome.Kind == sdk.OutcomeSuccess {
		return
	}
	outcome.FinalErrorDiagnostic = c.Snapshot()
}

// Drop request-owned buffers even when an outer context or response is retained.
// Failed diagnostics own their snapshots; their persistence lifetime is separate.
func (c *Capture) release() {
	c.mu.Lock()
	exchanges := c.exchanges
	c.exchanges = nil
	c.closed = true
	c.mu.Unlock()
	for _, e := range exchanges {
		e.release()
	}
}

// Exchange is one main transport request, including an internal retry.
type Exchange struct {
	mu       sync.Mutex
	request  sdk.OutboundRequestDiagnostic
	sent     *bodyBuffer
	response *bodyBuffer
	event    []byte
	events   *eventObserver
	closed   bool
}

// Record copies raw request bytes and headers. Only Core may persist them,
// after applying its redaction policy.
func Record(ctx context.Context, request sdk.OutboundRequestDiagnostic) *Exchange {
	return record(ctx, request, false, nil)
}

// sent is initialized before publication. Callers must keep their local buffer
// reference rather than reading or assigning e.sent after this function returns.
func record(ctx context.Context, request sdk.OutboundRequestDiagnostic, ownsBody bool, sent *bodyBuffer) *Exchange {
	c := fromContext(ctx)
	if c == nil {
		return nil
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}
	// Copy outside the collector lock; publication below rechecks Finish.
	request.Headers = request.Headers.Clone()
	if len(request.Body) > maxBodyBytes {
		request.BodyOriginalSize = max(request.BodyOriginalSize, int64(len(request.Body)))
		request.Body = nil
	} else if !ownsBody {
		request.Body = bytes.Clone(request.Body)
	}
	e := &Exchange{request: request, sent: sent}
	var evicted *Exchange
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		e.release()
		return nil
	}
	if len(c.exchanges) < maxRequests {
		c.exchanges = append(c.exchanges, e)
	} else {
		evicted = c.exchanges[1]
		copy(c.exchanges[1:maxRequests-1], c.exchanges[2:])
		c.exchanges[maxRequests-1] = e
	}
	c.mu.Unlock()
	// Buffer/observer locks must never extend the collector critical section.
	if evicted != nil {
		evicted.release()
	}
	return e
}

func (e *Exchange) release() {
	e.mu.Lock()
	e.closed = true
	sent, response, events := e.sent, e.response, e.events
	e.request = sdk.OutboundRequestDiagnostic{}
	e.sent, e.response, e.events, e.event = nil, nil, nil, nil
	e.mu.Unlock()
	if sent != nil {
		sent.release()
	}
	if response != nil {
		response.release()
	}
	if events != nil {
		events.release()
	}
}

func (e *Exchange) SetStatus(status int) {
	if e == nil {
		return
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.request.StatusCode = status
	e.mu.Unlock()
}

// ObserveEvent retains the most recent raw data payload, before any translation.
// It deliberately has no vendor-specific event names or protocol success rules.
func (e *Exchange) ObserveEvent(data []byte) {
	if e == nil || len(data) == 0 {
		return
	}
	if len(data) > maxEventBytes {
		return
	}
	e.mu.Lock()
	if !e.closed {
		e.event = append(e.event[:0], data...)
	}
	e.mu.Unlock()
}

func (c *Capture) Snapshot() *sdk.FinalErrorDiagnostic {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	exchanges := append([]*Exchange(nil), c.exchanges...)
	c.mu.Unlock()
	if len(exchanges) == 0 {
		return nil
	}
	diagnostic := &sdk.FinalErrorDiagnostic{OutboundRequests: make([]sdk.OutboundRequestDiagnostic, len(exchanges))}
	remaining := maxBodyBytes
	// Prioritize the terminal request without borrowing a previous request's error.
	for i := len(exchanges) - 1; i >= 0; i-- {
		e := exchanges[i]
		e.mu.Lock()
		req, sent, response := e.request, e.sent, e.response
		event := bytes.Clone(e.event)
		e.mu.Unlock()
		body := req.Body
		originalSize := max(req.BodyOriginalSize, int64(len(body)))
		if sent != nil {
			var sentSize int64
			body, sentSize = sent.snapshot()
			originalSize = max(originalSize, sentSize)
		}
		req.Body = nil
		req.BodyOriginalSize = originalSize
		if originalSize <= int64(len(body)) && len(body) <= remaining {
			req.Body = body
			remaining -= len(body)
		}
		diagnostic.OutboundRequests[i] = req
		if i == len(exchanges)-1 {
			raw := event
			if response != nil {
				var size int64
				raw, size = response.snapshot()
				if size > int64(len(raw)) {
					raw = nil
				}
			}
			if len(raw) <= remaining {
				diagnostic.UpstreamErrorBody = raw
				remaining -= len(raw)
			}
		}
	}
	return diagnostic
}

type bodyBuffer struct {
	mu     sync.Mutex
	data   []byte
	size   int64
	closed bool
}

func (b *bodyBuffer) write(p []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.size += int64(len(p))
	if available := maxBodyBytes - len(b.data); available > 0 {
		b.data = append(b.data, p[:min(available, len(p))]...)
	}
}

func (b *bodyBuffer) release() {
	b.mu.Lock()
	b.data = nil
	b.size = 0
	b.closed = true
	b.mu.Unlock()
}

func (b *bodyBuffer) snapshot() ([]byte, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.data), b.size
}
