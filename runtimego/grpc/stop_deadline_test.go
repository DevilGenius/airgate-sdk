package grpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	"google.golang.org/grpc"
)

type deadlinePluginClient struct {
	pb.PluginServiceClient
	remaining time.Duration
}

func (c *deadlinePluginClient) Stop(ctx context.Context, _ *pb.Empty, _ ...grpc.CallOption) (*pb.Empty, error) {
	deadline, ok := ctx.Deadline()
	if ok {
		c.remaining = time.Until(deadline)
	}
	return &pb.Empty{}, nil
}
func TestPluginStopSuppliesBoundedContext(t *testing.T) {
	client := &deadlinePluginClient{}
	base := &pluginBase{plugin: client}
	if err := base.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.remaining <= 0 || client.remaining > defaultGRPCTimeout {
		t.Fatal("default stop deadline missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := base.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if client.remaining > time.Second {
		t.Fatal("stop extended caller deadline")
	}
}
