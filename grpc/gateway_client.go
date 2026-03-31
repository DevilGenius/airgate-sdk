package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// GatewayGRPCClient 将 gRPC 客户端包装为 GatewayPlugin 接口（核心侧使用）
type GatewayGRPCClient struct {
	pluginBase // 嵌入公共基类，自动获得 Info/Init/Start/Stop/GetWebAssets
	gateway    pb.GatewayServiceClient

	// 缓存（mu 保护并发安全）
	mu             sync.RWMutex
	cachedPlatform string
	cachedModels   []sdk.ModelInfo
	cachedRoutes   []sdk.RouteDefinition
}

// InvalidateCache 清除所有缓存的元数据（Platform/Models/Routes/Info），
// 下次调用时将重新从插件获取最新数据。
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
		}
	}
	c.mu.Lock()
	c.cachedRoutes = routes
	c.mu.Unlock()
	return routes
}

// buildProtoRequest 将 SDK ForwardRequest 转为 proto ForwardRequest
func buildProtoRequest(req *sdk.ForwardRequest) *pb.ForwardRequest {
	credsJSON, _ := json.Marshal(req.Account.Credentials) //nolint:errcheck // Credentials 是 map[string]string，Marshal 不会失败
	return &pb.ForwardRequest{
		AccountId:       req.Account.ID,
		AccountName:     req.Account.Name,
		AccountPlatform: req.Account.Platform,
		AccountType:     req.Account.Type,
		CredentialsJson: credsJSON,
		ProxyUrl:        req.Account.ProxyURL,
		Body:            req.Body,
		Headers:         httpHeadersToProto(req.Headers),
		Model:           req.Model,
		Stream:          req.Stream,
	}
}

// fromProtoResult 将 proto ForwardResult 转为 SDK ForwardResult
func fromProtoResult(r *pb.ForwardResult) *sdk.ForwardResult {
	result := &sdk.ForwardResult{
		StatusCode:            int(r.StatusCode),
		InputTokens:           int(r.InputTokens),
		OutputTokens:          int(r.OutputTokens),
		CachedInputTokens:     int(r.CachedInputTokens),
		ReasoningOutputTokens: int(r.ReasoningOutputTokens),
		Model:                 r.Model,
		Duration:              time.Duration(r.DurationMs) * time.Millisecond,
		AccountStatus:         r.AccountStatus,
		ErrorMessage:          r.ErrorMessage,
		RetryAfter:            time.Duration(r.RetryAfterMs) * time.Millisecond,
		ServiceTier:           r.ServiceTier,
		Body:                  r.Body,
		UpdatedCredentials:    r.UpdatedCredentials,
	}
	if len(r.Headers) > 0 {
		result.Headers = protoHeadersToHTTP(r.Headers)
	}
	return result
}

func (c *GatewayGRPCClient) Forward(ctx context.Context, req *sdk.ForwardRequest) (*sdk.ForwardResult, error) {
	pbReq := buildProtoRequest(req)

	// 流式请求
	if req.Stream && req.Writer != nil {
		return c.forwardStream(ctx, pbReq, req)
	}

	// 非流式请求
	resp, err := c.gateway.Forward(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("gRPC Forward 调用失败: %w", err)
	}
	return fromProtoResult(resp), nil
}

func (c *GatewayGRPCClient) forwardStream(ctx context.Context, pbReq *pb.ForwardRequest, req *sdk.ForwardRequest) (*sdk.ForwardResult, error) {
	stream, err := c.gateway.ForwardStream(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("gRPC ForwardStream 调用失败: %w", err)
	}

	var finalResult *sdk.ForwardResult
	responseStarted := false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gRPC 流接收失败: %w", err)
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
				return nil, fmt.Errorf("写入响应失败: %w", writeErr)
			}
			if flusher, ok := req.Writer.(interface{ Flush() }); ok {
				flusher.Flush()
			}
		}

		if chunk.Done && chunk.FinalResult != nil {
			finalResult = fromProtoResult(chunk.FinalResult)
		}
	}

	if finalResult == nil {
		return nil, fmt.Errorf("未收到最终结果")
	}
	return finalResult, nil
}

func (c *GatewayGRPCClient) ValidateAccount(ctx context.Context, credentials map[string]string) error {
	_, err := c.gateway.ValidateAccount(ctx, &pb.CredentialsRequest{
		Credentials: credentials,
	})
	return err
}

func (c *GatewayGRPCClient) QueryQuota(ctx context.Context, credentials map[string]string) (*sdk.QuotaInfo, error) {
	resp, err := c.gateway.QueryQuota(ctx, &pb.CredentialsRequest{
		Credentials: credentials,
	})
	if err != nil {
		return nil, err
	}
	return &sdk.QuotaInfo{
		Total:     resp.Total,
		Used:      resp.Used,
		Remaining: resp.Remaining,
		Currency:  resp.Currency,
		ExpiresAt: resp.ExpiresAt,
		Extra:     resp.Extra,
	}, nil
}

// HandleWebSocket 通过 gRPC 双向流处理 WebSocket（Core 侧调用）
func (c *GatewayGRPCClient) HandleWebSocket(ctx context.Context, conn sdk.WebSocketConn) (*sdk.ForwardResult, error) {
	stream, err := c.gateway.HandleWebSocket(ctx)
	if err != nil {
		return nil, fmt.Errorf("gRPC HandleWebSocket 调用失败: %w", err)
	}

	info := conn.ConnectInfo()
	credsJSON, _ := json.Marshal(info.Account.Credentials) //nolint:errcheck // Credentials 是 map[string]string，Marshal 不会失败

	// 发送 CONNECT 帧
	if err := stream.Send(&pb.WebSocketFrame{
		Type: pb.WebSocketFrame_CONNECT,
		ConnectInfo: &pb.WebSocketConnectInfo{
			Path:            info.Path,
			Query:           info.Query,
			Headers:         httpHeadersToProto(info.Headers),
			RemoteAddr:      info.RemoteAddr,
			ConnectionId:    info.ConnectionID,
			AccountId:       info.Account.ID,
			AccountName:     info.Account.Name,
			AccountPlatform: info.Account.Platform,
			AccountType:     info.Account.Type,
			CredentialsJson: credsJSON,
			ProxyUrl:        info.Account.ProxyURL,
		},
	}); err != nil {
		return nil, fmt.Errorf("发送 CONNECT 帧失败: %w", err)
	}

	// 启动客户端 → 插件的消息转发
	clientConn := &grpcClientWebSocketConn{
		stream: stream,
		info:   info,
	}

	// 双向转发 goroutine：读取客户端消息发到插件
	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel()
	errCh := make(chan error, 1)
	go func() {
		defer func() { close(errCh) }()
		for {
			select {
			case <-readerCtx.Done():
				return
			default:
			}
			msgType, data, readErr := conn.ReadMessage()
			if readErr != nil {
				_ = stream.Send(&pb.WebSocketFrame{
					Type: pb.WebSocketFrame_CLOSE,
				})
				errCh <- readErr
				return
			}
			frameType := pb.WebSocketFrame_TEXT
			if msgType == sdk.WSMessageBinary {
				frameType = pb.WebSocketFrame_BINARY
			}
			if sendErr := stream.Send(&pb.WebSocketFrame{
				Type: frameType,
				Data: data,
			}); sendErr != nil {
				errCh <- sendErr
				return
			}
		}
	}()

	// 读取插件 → 客户端的消息
	var result *sdk.ForwardResult
	for {
		frame, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return nil, fmt.Errorf("gRPC WebSocket 流接收失败: %w", recvErr)
		}

		switch frame.Type {
		case pb.WebSocketFrame_TEXT:
			if writeErr := conn.WriteMessage(sdk.WSMessageText, frame.Data); writeErr != nil {
				return nil, writeErr
			}
		case pb.WebSocketFrame_BINARY:
			if writeErr := conn.WriteMessage(sdk.WSMessageBinary, frame.Data); writeErr != nil {
				return nil, writeErr
			}
		case pb.WebSocketFrame_CLOSE:
			_ = conn.Close(int(frame.CloseCode), frame.CloseReason)
			goto done
		case pb.WebSocketFrame_RESULT:
			if frame.Result != nil {
				result = fromProtoResult(frame.Result)
			}
			goto done
		}
	}

done:
	_ = clientConn // 保持引用
	readerCancel() // 通知读取 goroutine 退出
	<-errCh        // 等待 goroutine 完成

	if result == nil {
		return &sdk.ForwardResult{}, nil
	}
	return result, nil
}

// grpcClientWebSocketConn 用于保持 gRPC 流引用
type grpcClientWebSocketConn struct {
	stream pb.GatewayService_HandleWebSocketClient
	info   *sdk.WebSocketConnectInfo
}
