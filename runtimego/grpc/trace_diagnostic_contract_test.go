package grpc

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"google.golang.org/protobuf/proto"
)

func TestRawTraceDiagnosticProtoRoundTrip(t *testing.T) {
	descriptor := (&pb.OutboundRequestDiagnostic{}).ProtoReflect().Descriptor()
	if descriptor.Fields().Len() != 7 || descriptor.Fields().ByName("body_original_size").Number() != 7 || descriptor.ReservedNames().Len() != 0 || descriptor.ReservedRanges().Len() != 0 {
		t.Fatal("unreleased trace protocol contains obsolete fields")
	}
	body := []byte(`{"access_token":"fixture_secret","partial_image_b64":"fixture_image"}`)
	original := &sdk.FinalErrorDiagnostic{OutboundRequests: []sdk.OutboundRequestDiagnostic{{Transport: "http", Method: "POST", URL: "https://fixture.test/responses?token=fixture", Headers: http.Header{"Authorization": {"Bearer fixture"}, "Session_id": {"raw-session"}}, Body: body, BodyOriginalSize: int64(len(body)), StatusCode: 400}}, UpstreamErrorBody: body}
	wire, err := proto.Marshal(finalErrorDiagnosticToProto(original))
	if err != nil {
		t.Fatal(err)
	}
	var decoded pb.FinalErrorDiagnostic
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	got := finalErrorDiagnosticFromProto(&decoded)
	if !reflect.DeepEqual(got, original) || !bytes.Equal(got.UpstreamErrorBody, body) {
		t.Fatal("SDK altered raw diagnostic during transport")
	}
}
