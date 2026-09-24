package devserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type completionDevGateway struct {
	proxyTestGateway
	ctxBeforeReturn error
	returned        bool
}

func (g *completionDevGateway) Forward(ctx context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	sdk.BeginStreamCompletion(req.Writer)
	_, _ = req.Writer.Write([]byte("data: [DONE]\n\n"))
	g.ctxBeforeReturn = ctx.Err()
	g.returned = true
	return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess, Usage: &sdk.Usage{OutputTokens: 7}}, nil
}

type completionDevWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w completionDevWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(p)
	w.cancel()
	return n, err
}

func TestDevCompletionSupportsBufferedAndDirectHTTP(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		g := &completionDevGateway{}
		p := &ProxyHandler{plugin: g, store: newProxyTestStore(t)}
		body := `{"stream":false}`
		if streaming {
			body = `{"stream":true}`
		}
		request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body)).WithContext(ctx)
		w := completionDevWriter{httptest.NewRecorder(), cancel}
		p.ServeHTTP(w, request)
		cancel()
		if !g.returned || w.Body.String() != "data: [DONE]\n\n" {
			t.Fatal("dev completion lost response")
		}
		if streaming && g.ctxBeforeReturn != context.Canceled {
			t.Fatal("direct dev fixture did not cancel on terminal write")
		}
		if !streaming && g.ctxBeforeReturn != nil {
			t.Fatal("buffered completion was exposed before Forward returned")
		}
	}
}
