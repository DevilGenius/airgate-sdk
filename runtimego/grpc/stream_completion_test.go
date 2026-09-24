package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestStreamCompletionCarriesTailAndUsageTogether(t *testing.T) {
	for _, size := range []int{32, streamChunkSize - 8, streamChunkSize, streamChunkSize*3 + 7} {
		terminal := []byte("data: " + strings.Repeat("x", size) + "\n\n")
		stream := &stubForwardStreamServer{}
		server := &GatewayGRPCServer{Impl: stubGatewayPlugin{forward: func(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
			_, _ = req.Writer.Write([]byte("data: delta\n\n"))
			sdk.BeginStreamCompletion(req.Writer)
			if _, err := req.Writer.Write(terminal); err != nil {
				return sdk.ForwardOutcome{}, err
			}
			if _, err := req.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
				return sdk.ForwardOutcome{}, err
			}
			var before bytes.Buffer
			for _, chunk := range stream.chunks {
				before.Write(chunk.Data)
			}
			if bytes.Contains(before.Bytes(), terminal) {
				t.Fatal("completion exposed before outcome")
			}
			return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess, Usage: &sdk.Usage{InputTokens: 10, OutputTokens: 3}}, nil
		}}}
		if err := server.ForwardStream(&pb.ForwardRequest{}, stream); err != nil {
			t.Fatal(err)
		}
		last := stream.chunks[len(stream.chunks)-1]
		if !last.Done || len(last.Data) == 0 || len(last.Data) > streamChunkSize || last.FinalOutcome.GetUsage().GetOutputTokens() != 3 {
			t.Fatal("terminal bytes and usage must share the final chunk")
		}
		var body bytes.Buffer
		for _, chunk := range stream.chunks {
			body.Write(chunk.Data)
		}
		if body.String() != "data: delta\n\n"+string(terminal)+"data: [DONE]\n\n" {
			t.Fatal("chunking changed response bytes")
		}
	}
}

type cancelOnCompletionWriter struct {
	captureWriter
	cancel context.CancelFunc
}

func (w *cancelOnCompletionWriter) Flush() {
	if strings.Contains(w.body.String(), "data: [DONE]\n\n") {
		w.cancel()
	}
}

// Exercise an actual RPC, with cancellation triggered by delivery of the final
// SSE event. The old Data-then-FinalOutcome ordering loses usage in this race.
func TestStreamCompletionSurvivesImmediateDownstreamCancel(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	pb.RegisterGatewayServiceServer(server, &GatewayGRPCServer{Impl: stubGatewayPlugin{forward: func(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
		_, _ = req.Writer.Write([]byte("data: delta\n\n"))
		sdk.BeginStreamCompletion(req.Writer)
		_, _ = req.Writer.Write([]byte("data: [DONE]\n\n"))
		return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess, Usage: &sdk.Usage{InputTokens: 40, OutputTokens: 9}}, nil
	}}})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///completion", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := &GatewayGRPCClient{gateway: pb.NewGatewayServiceClient(conn)}
	for range 20 {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		writer := &cancelOnCompletionWriter{cancel: cancel}
		outcome, err := client.Forward(ctx, &sdk.ForwardRequest{Account: &sdk.Account{}, Stream: true, Writer: writer})
		cancel()
		if err != nil || outcome.Kind != sdk.OutcomeSuccess || outcome.Usage == nil || outcome.Usage.OutputTokens != 9 {
			t.Fatalf("completed request lost outcome/usage: kind=%v err=%v", outcome.Kind, err)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("fixture did not cancel on completion")
		}
	}
}

func TestFinalWriteFailurePreservesConfirmedUsage(t *testing.T) {
	client := &GatewayGRPCClient{gateway: &stubGatewayServiceClient{stream: &stubForwardStreamClient{chunks: []*pb.ForwardChunk{{
		Done: true, Data: []byte("data: [DONE]\n\n"), FinalOutcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS, Usage: &pb.Usage{InputTokens: 40, OutputTokens: 9}},
	}}}}}
	for _, writeErr := range []error{io.ErrClosedPipe, nil} {
		client.gateway.(*stubGatewayServiceClient).stream.(*stubForwardStreamClient).index = 0
		outcome, err := client.forwardStream(t.Context(), &pb.ForwardRequest{}, &sdk.ForwardRequest{Writer: &errorHTTPWriter{err: writeErr}})
		if err == nil || outcome.Kind != sdk.OutcomeStreamAborted || outcome.FailoverScope != sdk.FailoverScopeTerminal || outcome.Usage == nil || outcome.Usage.OutputTokens != 9 {
			t.Fatalf("write failure lost confirmed usage: kind=%v err=%v", outcome.Kind, err)
		}
	}
}

type unwrappedCompletionWriter struct{ http.ResponseWriter }

func (w unwrappedCompletionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func TestCompletionMarkerTraversesHTTPWrappers(t *testing.T) {
	w := &streamWriter{stream: &stubForwardStreamServer{}}
	sdk.BeginStreamCompletion(unwrappedCompletionWriter{w})
	if !w.completing {
		t.Fatal("wrapper lost completion marker")
	}
}

func TestStreamWriteFailureIsStickyAndReportsCurrentWriteBytes(t *testing.T) {
	for _, test := range []struct {
		name                        string
		completing                  bool
		errAtSend, inputBytes, want int
	}{
		{"old_tail", true, 2, 1, 0},
		{"new_data", true, 4, 3 * streamChunkSize, streamChunkSize},
		{"ordinary_data", false, 3, 3 * streamChunkSize, streamChunkSize},
	} {
		t.Run(test.name, func(t *testing.T) {
			wantErr := io.ErrClosedPipe
			stream := &stubForwardStreamServer{sendErr: wantErr, errAtSend: test.errAtSend}
			w := &streamWriter{stream: stream}
			if test.completing {
				w.BeginStreamCompletion()
				if _, err := w.Write(make([]byte, streamChunkSize)); err != nil {
					t.Fatal(err)
				}
			}
			n, err := w.Write(make([]byte, test.inputBytes))
			if n != test.want || !errors.Is(err, wantErr) || w.completionTail != nil {
				t.Fatalf("Write = %d/%v, want %d/closed pipe and no tail", n, err, test.want)
			}
			calls := stream.sendCalls
			if n, err := w.Write([]byte("retry")); n != 0 || !errors.Is(err, wantErr) || stream.sendCalls != calls {
				t.Fatal("failed stream retried a send")
			}
		})
	}
}

func TestForwardStreamDoesNotSendSuccessAfterIgnoredWriteError(t *testing.T) {
	stream := &stubForwardStreamServer{sendErr: io.ErrClosedPipe, errAtSend: 2}
	server := &GatewayGRPCServer{Impl: stubGatewayPlugin{forward: func(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
		sdk.BeginStreamCompletion(req.Writer)
		_, _ = req.Writer.Write(make([]byte, 3*streamChunkSize))
		return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil
	}}}
	if err := server.ForwardStream(&pb.ForwardRequest{}, stream); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if stream.sendCalls != 2 {
		t.Fatal("sent final success on a failed stream")
	}
}
