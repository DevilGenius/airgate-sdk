package grpc

import (
	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func checkResponseMessage(message proto.Message) error {
	if proto.Size(message) > sdk.MaxResponseMessageBytes {
		return status.Error(codes.ResourceExhausted, "plugin response exceeds payload budget")
	}
	return nil
}

func checkedOutcome(outcome sdk.ForwardOutcome) (*pb.ForwardOutcome, error) {
	if len(outcome.Upstream.Body) > sdk.MaxBufferedResponseBytes {
		return nil, status.Error(codes.ResourceExhausted, "plugin response body exceeds payload budget")
	}
	value := outcomeToProto(outcome)
	if checkResponseMessage(value) != nil {
		value.FinalErrorDiagnostic = nil
	}
	return value, checkResponseMessage(value)
}
