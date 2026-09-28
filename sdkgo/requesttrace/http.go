package requesttrace

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// Transport discovers tracing from each request's context, so pooled clients do
// not retain another user's collector. Without tracing it delegates unchanged.
type Transport struct{ Base http.RoundTripper }

func (t *Transport) CloseIdleConnections() {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if closer, ok := base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if fromContext(req.Context()) == nil {
		return base.RoundTrip(req)
	}
	diagnostic := sdk.OutboundRequestDiagnostic{Transport: "http", Method: req.Method, URL: req.URL.String(), Headers: req.Header}
	replayable := false
	if req.Body != nil && req.GetBody != nil {
		if reader, err := req.GetBody(); err == nil {
			diagnostic.Body, err = readRequestSnapshot(reader, req.ContentLength)
			_ = reader.Close()
			diagnostic.BodyOriginalSize = max(req.ContentLength, int64(len(diagnostic.Body)))
			replayable = err == nil
			if err != nil {
				diagnostic.Body = nil
			}
		}
	}
	// Initialize all exchange state before record publishes it to Capture.
	// Finish may release the exchange immediately after publication.
	var sent *bodyBuffer
	if req.Body != nil {
		diagnostic.BodyOriginalSize = max(diagnostic.BodyOriginalSize, req.ContentLength)
		if !replayable {
			sent = &bodyBuffer{}
		}
	}
	e := record(req.Context(), diagnostic, true, sent)
	if e == nil {
		return base.RoundTrip(req)
	}
	if sent != nil {
		copy := req.Clone(req.Context())
		// Bind the local buffer, not e.sent, which release is allowed to clear.
		// A released buffer already ignores late writes under its own mutex.
		copy.Body = &observedBody{ReadCloser: req.Body, consume: sent.write}
		req = copy
	}
	resp, err := base.RoundTrip(req)
	e.WrapResponse(resp)
	return resp, err
}

// Read the independent replay reader into a single owned allocation when the
// length is known; an extra byte detects mismatched Content-Length safely.
func readRequestSnapshot(reader io.Reader, size int64) ([]byte, error) {
	if size <= 0 || size > maxBodyBytes {
		return io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	}
	body := make([]byte, int(size)+1)
	n, err := io.ReadFull(reader, body)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return body[:n], nil
	}
	if err != nil {
		return nil, err
	}
	if len(body) > maxBodyBytes {
		return body, nil
	}
	return io.ReadAll(io.LimitReader(io.MultiReader(bytes.NewReader(body), reader), maxBodyBytes+1))
}

// WrapResponse observes bytes only as the caller consumes them. It never reads
// ahead, drains, buffers a whole stream, or changes cancellation/close behavior.
func (e *Exchange) WrapResponse(resp *http.Response) {
	if e == nil || resp == nil {
		return
	}
	e.SetStatus(resp.StatusCode)
	if resp.Body == nil {
		return
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	if strings.EqualFold(mediaType, "text/event-stream") {
		events := &eventObserver{exchange: e}
		e.events = events
		resp.Body = &observedBody{ReadCloser: resp.Body, consume: events.write, finish: events.finish}
	} else {
		buffer := &bodyBuffer{}
		e.response = buffer
		resp.Body = &observedBody{ReadCloser: resp.Body, consume: buffer.write}
	}
	e.mu.Unlock()
}

type observedBody struct {
	io.ReadCloser
	consume func([]byte)
	finish  func()
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.consume(p[:n])
	}
	if err != nil && b.finish != nil {
		b.finish()
		b.finish = nil
	}
	return n, err
}

// eventObserver is a bounded incremental SSE observer, independent of the
// protocol parser. Long lines/events are discarded without affecting traffic.
type eventObserver struct {
	mu        sync.Mutex
	closed    bool
	exchange  *Exchange
	line      []byte
	data      []byte
	skipLine  bool
	skipEvent bool
}

func (s *eventObserver) write(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	for len(p) > 0 {
		index := bytes.IndexByte(p, '\n')
		chunk := p
		if index >= 0 {
			chunk = p[:index]
		}
		if !s.skipLine {
			if len(s.line)+len(chunk) > maxEventBytes {
				s.line = nil
				s.skipLine = true
				s.skipEvent = true
			} else {
				s.line = append(s.line, chunk...)
			}
		}
		if index < 0 {
			return
		}
		if !s.skipLine {
			s.consumeLine(bytes.TrimSuffix(s.line, []byte{'\r'}))
		}
		s.line = s.line[:0]
		s.skipLine = false
		p = p[index+1:]
	}
}

func (s *eventObserver) consumeLine(line []byte) {
	if len(line) == 0 {
		s.flush()
		return
	}
	if !bytes.HasPrefix(line, []byte("data:")) || s.skipEvent {
		return
	}
	payload := bytes.TrimPrefix(line[5:], []byte{' '})
	if len(s.data)+len(payload)+1 > maxEventBytes {
		s.data = nil
		s.skipEvent = true
		return
	}
	s.data = append(s.data, payload...)
	s.data = append(s.data, '\n')
}

func (s *eventObserver) flush() {
	if !s.skipEvent {
		s.exchange.ObserveEvent(bytes.TrimSuffix(s.data, []byte{'\n'}))
	}
	s.data = s.data[:0]
	s.skipEvent = false
}

func (s *eventObserver) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if !s.skipLine && len(s.line) > 0 {
		s.consumeLine(bytes.TrimSuffix(s.line, []byte{'\r'}))
	}
	s.line = nil
	s.flush()
	s.line, s.data = nil, nil
}

func (s *eventObserver) release() {
	s.mu.Lock()
	s.closed = true
	s.line, s.data = nil, nil
	s.mu.Unlock()
}
