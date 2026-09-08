package grpc

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
