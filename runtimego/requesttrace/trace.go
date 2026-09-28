// Package requesttrace captures upstream wire requests at transport boundaries.
// Core owns incoming requests and persistence; transports own wire diagnostics.
package requesttrace

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
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

// Exchange is one transport request, including internal retries or attachments.
type Exchange struct {
	mu           sync.Mutex
	request      sdk.OutboundRequestDiagnostic
	sent         *bodyBuffer
	response     *bodyBuffer
	event        []byte
	responseType string
	events       *eventObserver
	closed       bool
}

// Record starts an exchange. The request body and safe headers are copied.
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
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	request.Headers = SafeHeaders(request.Headers)
	request.URL = SafeURL(request.URL)
	if len(request.Body) > maxBodyBytes {
		request.BodyOriginalSize = max(request.BodyOriginalSize, int64(len(request.Body)))
		request.Body = nil
		request.BodyRedacted, request.BodyRedactionReason = true, "trace_size_limit"
	} else if !ownsBody {
		request.Body = bytes.Clone(request.Body)
	}
	e := &Exchange{request: request, sent: sent}
	if len(c.exchanges) < maxRequests {
		c.exchanges = append(c.exchanges, e)
	} else {
		c.exchanges[1].release()
		copy(c.exchanges[1:maxRequests-1], c.exchanges[2:])
		c.exchanges[maxRequests-1] = e
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
	if e == nil || len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
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
		event, responseType := bytes.Clone(e.event), e.responseType
		e.mu.Unlock()
		body := req.Body
		originalSize := max(req.BodyOriginalSize, int64(len(body)))
		omission := req.BodyRedactionReason
		if sent != nil {
			var sentSize int64
			body, sentSize = sent.snapshot()
			originalSize = max(originalSize, sentSize)
			if originalSize > maxBodyBytes {
				omission = "trace_size_limit"
			} else if originalSize > int64(len(body)) {
				omission = "trace_capture_incomplete"
			}
		}
		snapshot := BodySnapshot{Body: body}
		if omission == "" {
			snapshot = SanitizeBody(body, req.Headers.Get("Content-Type"), IsImageURL(req.URL))
		}
		req.Body = nil
		req.BodyOriginalSize = 0
		req.BodyRedacted, req.BodyRedactionReason = snapshot.Redacted, snapshot.RedactionReason
		if snapshot.Redacted {
			req.BodyOriginalSize = snapshot.OriginalSize
			req.Headers = req.Headers.Clone()
			if req.Headers == nil {
				req.Headers = make(http.Header)
			}
			req.Headers.Set("Content-Type", snapshot.ContentType)
		}
		if omission != "" || len(snapshot.Body) > remaining {
			if omission == "" {
				omission = "trace_size_limit"
			}
			req.BodyRedacted, req.BodyRedactionReason, req.BodyOriginalSize = true, omission, originalSize
		} else {
			req.Body = snapshot.Body
			remaining -= len(req.Body)
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
			raw = SanitizeBody(raw, responseType, false).Body
			if len(raw) <= remaining {
				diagnostic.UpstreamErrorBody = raw
				remaining -= len(raw)
			}
		}
	}
	return diagnostic
}

// SafeHeaders keeps transport metadata only; credentials never leave the plugin.
func SafeHeaders(headers http.Header) http.Header {
	safe := make(http.Header)
	for name, values := range headers {
		key := strings.ToLower(strings.TrimSpace(name))
		switch key {
		case "accept", "content-type", "openai-beta", "originator", "user-agent":
			safe[name] = append([]string(nil), values...)
		default:
			if strings.HasPrefix(key, "x-airgate-trace-") {
				safe[name] = append([]string(nil), values...)
			}
		}
	}
	for key, digest := range HeaderFingerprints(headers) {
		safe.Set(key, digest)
	}
	return safe
}

func SafeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
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
