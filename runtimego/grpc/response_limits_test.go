package grpc

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestBufferedResponseStopsAtBusinessPayloadLimit(t *testing.T) {
	canceled := false
	w := &bufferWriter{onLimit: func() { canceled = true }}
	block := make([]byte, 1<<20)
	for range sdk.MaxBufferedResponseBytes / len(block) {
		if _, err := w.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Write([]byte{1}); status.Code(err) != codes.ResourceExhausted || !canceled || len(w.body) != sdk.MaxBufferedResponseBytes {
		t.Fatal("writer exceeded payload budget")
	}
	message := &pb.ForwardOutcome{Upstream: &pb.UpstreamResponse{Body: make([]byte, sdk.MaxResponseMessageBytes+1)}}
	if status.Code(checkResponseMessage(message)) != codes.ResourceExhausted {
		t.Fatal("oversized final message accepted")
	}
	if _, err := checkedOutcome(sdk.ForwardOutcome{Upstream: sdk.UpstreamResponse{Body: message.Upstream.Body[:sdk.MaxBufferedResponseBytes+1]}}); status.Code(err) != codes.ResourceExhausted {
		t.Fatal("returned body bypassed writer budget")
	}
}

func TestFullSizeBodyFitsResponseEnvelope(t *testing.T) {
	body := make([]byte, 96<<20)
	outcome := sdk.ForwardOutcome{
		Kind:     sdk.OutcomeSuccess,
		Upstream: sdk.UpstreamResponse{StatusCode: 200, Headers: http.Header{"Content-Type": {"application/json"}}, Body: body},
		Usage:    &sdk.Usage{Model: "test", InputTokens: 1, OutputTokens: 2},
	}
	message, err := checkedOutcome(outcome)
	if err != nil || proto.Size(message) <= len(body) {
		t.Fatalf("full body and metadata must fit: %v", err)
	}
	chunk := &pb.ForwardChunk{Done: true, FinalOutcome: message}
	if err := checkOutcomeMessage(message, chunk); err != nil {
		t.Fatal(err)
	}
	// Optional diagnostics must not displace the client's successful response.
	message.FinalErrorDiagnostic = &pb.FinalErrorDiagnostic{UpstreamErrorBody: make([]byte, 17<<20)}
	if err := checkOutcomeMessage(message, chunk); err != nil || message.FinalErrorDiagnostic != nil || len(message.Upstream.Body) != len(body) {
		t.Fatal("diagnostics displaced the response body")
	}
}

func TestFinalStreamEnvelopeIsIncludedInBudget(t *testing.T) {
	outcome := &pb.ForwardOutcome{Reason: strings.Repeat("x", sdk.MaxResponseMessageBytes-5)}
	if proto.Size(outcome) != sdk.MaxResponseMessageBytes {
		t.Fatal("fixture must fill the inner message exactly")
	}
	chunk := &pb.ForwardChunk{Done: true, FinalOutcome: outcome}
	if err := checkOutcomeMessage(outcome, chunk); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("final chunk wrapper escaped budget: %v", err)
	}
}

func TestForwardChecksDiagnosticsAfterWriterFallback(t *testing.T) {
	// This fits before the fallback writer body is attached, but not afterwards.
	outcome := sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess,
		FinalErrorDiagnostic: &sdk.FinalErrorDiagnostic{UpstreamErrorBody: make([]byte, sdk.MaxResponseMessageBytes)},
	}
	wire := outcomeToProto(outcome)
	overhead := proto.Size(wire) - sdk.MaxResponseMessageBytes
	outcome.FinalErrorDiagnostic.UpstreamErrorBody = outcome.FinalErrorDiagnostic.UpstreamErrorBody[:sdk.MaxResponseMessageBytes-overhead]
	server := &GatewayGRPCServer{Impl: &metadataGatewayPlugin{forwardOut: outcome}}
	response, err := server.Forward(context.Background(), &pb.ForwardRequest{Account: &pb.AccountProto{}})
	if err != nil || response.FinalErrorDiagnostic != nil || string(response.Upstream.Body) != "writer body" {
		t.Fatalf("fallback response was displaced by diagnostics: %v", err)
	}
}
