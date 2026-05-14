package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// GatewayGRPCClient 把 gRPC 客户端包装成 GatewayPlugin 接口，供 Core 消费。
type GatewayGRPCClient struct {
	pluginBase
	gateway pb.GatewayServiceClient

	mu             sync.RWMutex
	cachedPlatform string
	cachedModels   []sdk.ModelInfo
	cachedRoutes   []sdk.RouteDefinition
}

// InvalidateCache 清除所有元数据缓存，下次调用时重新从插件获取。
// 典型场景：ConfigWatcher.OnConfigUpdate 后调用。
func (c *GatewayGRPCClient) InvalidateCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedPlatform = ""
	c.cachedModels = nil
	c.cachedRoutes = nil
	c.cachedInfo = nil
}

func (c *GatewayGRPCClient) Platform() string {
	c.mu.RLock()
	if c.cachedPlatform != "" {
		defer c.mu.RUnlock()
		return c.cachedPlatform
	}
	c.mu.RUnlock()

	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := c.gateway.GetPlatform(ctx, &pb.Empty{})
	if err != nil {
		return ""
	}
	c.mu.Lock()
	c.cachedPlatform = resp.Value
	c.mu.Unlock()
	return resp.Value
}

func (c *GatewayGRPCClient) Models() []sdk.ModelInfo {
	c.mu.RLock()
	if c.cachedModels != nil {
		defer c.mu.RUnlock()
		return c.cachedModels
	}
	c.mu.RUnlock()

	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := c.gateway.GetModels(ctx, &pb.Empty{})
	if err != nil {
		return nil
	}
	models := convertModels(resp.Models)
	c.mu.Lock()
	c.cachedModels = models
	c.mu.Unlock()
	return models
}

func (c *GatewayGRPCClient) Routes() []sdk.RouteDefinition {
	c.mu.RLock()
	if c.cachedRoutes != nil {
		defer c.mu.RUnlock()
		return c.cachedRoutes
	}
	c.mu.RUnlock()

	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := c.gateway.GetRoutes(ctx, &pb.Empty{})
	if err != nil {
		return nil
	}
	routes := make([]sdk.RouteDefinition, len(resp.Routes))
	for i, r := range resp.Routes {
		routes[i] = sdk.RouteDefinition{
			Method:      r.Method,
			Path:        r.Path,
			Description: r.Description,
			Metadata:    r.Metadata,
		}
	}
	c.mu.Lock()
	c.cachedRoutes = routes
	c.mu.Unlock()
	return routes
}

func buildProtoRequest(req *sdk.ForwardRequest) *pb.ForwardRequest {
	credsJSON, _ := json.Marshal(req.Account.Credentials) //nolint:errcheck // map[string]string Marshal 不会失败
	return &pb.ForwardRequest{
		Account: &pb.AccountProto{
			Id:              req.Account.ID,
			Name:            req.Account.Name,
			Platform:        req.Account.Platform,
			Type:            req.Account.Type,
			CredentialsJson: credsJSON,
			ProxyUrl:        req.Account.ProxyURL,
		},
		Body:    req.Body,
		Headers: httpHeadersToProto(req.Headers),
		Model:   req.Model,
		Stream:  req.Stream,
	}
}

func (c *GatewayGRPCClient) Forward(ctx context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	pbReq := buildProtoRequest(req)

	if req.Writer != nil {
		return c.forwardStream(ctx, pbReq, req)
	}

	resp, err := c.gateway.Forward(ctx, pbReq)
	if err != nil {
		return sdk.ForwardOutcome{}, fmt.Errorf("gRPC Forward 调用失败: %w", err)
	}
	return outcomeFromProto(resp), nil
}

func (c *GatewayGRPCClient) forwardStream(ctx context.Context, pbReq *pb.ForwardRequest, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	stream, err := c.gateway.ForwardStream(ctx, pbReq)
	if err != nil {
		return sdk.ForwardOutcome{}, fmt.Errorf("gRPC ForwardStream 调用失败: %w", err)
	}

	var finalOutcome sdk.ForwardOutcome
	haveFinal := false
	responseStarted := false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sdk.ForwardOutcome{}, fmt.Errorf("gRPC 流接收失败: %w", err)
		}

		if !responseStarted && req.Writer != nil && (chunk.StatusCode != 0 || len(chunk.Headers) > 0 || len(chunk.Data) > 0) {
			for k, vals := range protoHeadersToHTTP(chunk.Headers) {
				for _, v := range vals {
					req.Writer.Header().Add(k, v)
				}
			}
			statusCode := int(chunk.StatusCode)
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			req.Writer.WriteHeader(statusCode)
			responseStarted = true
		}

		if len(chunk.Data) > 0 && req.Writer != nil {
			if _, writeErr := req.Writer.Write(chunk.Data); writeErr != nil {
				return sdk.ForwardOutcome{}, fmt.Errorf("写入响应失败: %w", writeErr)
			}
			if flusher, ok := req.Writer.(interface{ Flush() }); ok {
				flusher.Flush()
			}
		}

		if chunk.Done && chunk.FinalOutcome != nil {
			finalOutcome = outcomeFromProto(chunk.FinalOutcome)
			haveFinal = true
		}
	}

	if !haveFinal {
		return sdk.ForwardOutcome{}, fmt.Errorf("未收到最终 outcome")
	}
	return finalOutcome, nil
}

func (c *GatewayGRPCClient) ValidateAccount(ctx context.Context, credentials map[string]string) error {
	_, err := c.gateway.ValidateAccount(ctx, &pb.CredentialsRequest{Credentials: credentials})
	return err
}

// HandleWebSocket 通过 gRPC 双向流处理 WebSocket（Core 侧调用）。
func (c *GatewayGRPCClient) HandleWebSocket(ctx context.Context, conn sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	stream, err := c.gateway.HandleWebSocket(ctx)
	if err != nil {
		return sdk.ForwardOutcome{}, fmt.Errorf("gRPC HandleWebSocket 调用失败: %w", err)
	}

	info := conn.ConnectInfo()
	credsJSON, _ := json.Marshal(info.Account.Credentials) //nolint:errcheck

	if err := stream.Send(&pb.WebSocketFrame{
		Type: pb.WebSocketFrame_CONNECT,
		ConnectInfo: &pb.WebSocketConnectInfo{
			Path:         info.Path,
			Query:        info.Query,
			Headers:      httpHeadersToProto(info.Headers),
			RemoteAddr:   info.RemoteAddr,
			ConnectionId: info.ConnectionID,
			Account: &pb.AccountProto{
				Id:              info.Account.ID,
				Name:            info.Account.Name,
				Platform:        info.Account.Platform,
				Type:            info.Account.Type,
				CredentialsJson: credsJSON,
				ProxyUrl:        info.Account.ProxyURL,
			},
		},
	}); err != nil {
		return sdk.ForwardOutcome{}, fmt.Errorf("发送 CONNECT 帧失败: %w", err)
	}

	clientConn := &grpcClientWebSocketConn{stream: stream, info: info}

	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel()
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		for {
			select {
			case <-readerCtx.Done():
				return
			default:
			}
			msgType, data, readErr := conn.ReadMessage()
			if readErr != nil {
				_ = stream.Send(&pb.WebSocketFrame{Type: pb.WebSocketFrame_CLOSE})
				errCh <- readErr
				return
			}
			frameType := pb.WebSocketFrame_TEXT
			if msgType == sdk.WSMessageBinary {
				frameType = pb.WebSocketFrame_BINARY
			}
			if sendErr := stream.Send(&pb.WebSocketFrame{Type: frameType, Data: data}); sendErr != nil {
				errCh <- sendErr
				return
			}
		}
	}()

	var outcome sdk.ForwardOutcome
	haveOutcome := false
	for {
		frame, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return sdk.ForwardOutcome{}, fmt.Errorf("gRPC WebSocket 流接收失败: %w", recvErr)
		}

		switch frame.Type {
		case pb.WebSocketFrame_TEXT:
			if writeErr := conn.WriteMessage(sdk.WSMessageText, frame.Data); writeErr != nil {
				return sdk.ForwardOutcome{}, writeErr
			}
		case pb.WebSocketFrame_BINARY:
			if writeErr := conn.WriteMessage(sdk.WSMessageBinary, frame.Data); writeErr != nil {
				return sdk.ForwardOutcome{}, writeErr
			}
		case pb.WebSocketFrame_CLOSE:
			_ = conn.Close(int(frame.CloseCode), frame.CloseReason)
			goto done
		case pb.WebSocketFrame_RESULT:
			if frame.Outcome != nil {
				outcome = outcomeFromProto(frame.Outcome)
				haveOutcome = true
			}
			goto done
		}
	}

done:
	_ = clientConn
	readerCancel()
	<-errCh

	if !haveOutcome {
		return sdk.ForwardOutcome{}, nil
	}
	return outcome, nil
}

// grpcClientWebSocketConn 保持 gRPC 流引用，便于后续扩展。
type grpcClientWebSocketConn struct {
	stream pb.GatewayService_HandleWebSocketClient
	info   *sdk.WebSocketConnectInfo
}
