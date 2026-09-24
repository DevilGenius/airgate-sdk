package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func checkResponseMessage(message proto.Message) error {
	if proto.Size(message) > sdk.MaxResponseMessageBytes {
		return status.Error(codes.ResourceExhausted, "plugin response exceeds payload budget")
	}
	return nil
}

func checkedOutcome(outcome sdk.ForwardOutcome) (*pb.ForwardOutcome, error) {
	value := outcomeToProto(outcome)
	return value, checkOutcomeMessage(value, value)
}

// Check the fully assembled wire message, including a ForwardChunk wrapper.
// Diagnostics are optional; never reject a valid response just to retain them.
func checkOutcomeMessage(outcome *pb.ForwardOutcome, message proto.Message) error {
	if len(outcome.GetUpstream().GetBody()) > sdk.MaxBufferedResponseBytes {
		return status.Error(codes.ResourceExhausted, "plugin response body exceeds payload budget")
	}
	if checkResponseMessage(message) != nil {
		outcome.FinalErrorDiagnostic = nil
	}
	return checkResponseMessage(message)
}
