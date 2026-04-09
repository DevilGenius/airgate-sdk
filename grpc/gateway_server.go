package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// GatewayGRPCServer 将 GatewayPlugin 包装为 gRPC 服务端
type GatewayGRPCServer struct {
	pb.UnimplementedGatewayServiceServer
	Impl sdk.GatewayPlugin
}

func (s *GatewayGRPCServer) GetPlatform(_ context.Context, _ *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.Platform()}, nil
}

func (s *GatewayGRPCServer) GetModels(_ context.Context, _ *pb.Empty) (*pb.ModelsResponse, error) {
	models := s.Impl.Models()
	resp := &pb.ModelsResponse{}
	for _, m := range models {
		resp.Models = append(resp.Models, &pb.ModelInfoProto{
			Id:                       m.ID,
			Name:                     m.Name,
			ContextWindow:            int64(m.ContextWindow),
			MaxOutputTokens:          int64(m.MaxOutputTokens),
			InputPrice:               m.InputPrice,
			OutputPrice:              m.OutputPrice,
			CachedInputPrice:         m.CachedInputPrice,
			InputPricePriority:       m.InputPricePriority,
			OutputPricePriority:      m.OutputPricePriority,
			CachedInputPricePriority: m.CachedInputPricePriority,
		})
	}
	return resp, nil
}

func (s *GatewayGRPCServer) GetRoutes(_ context.Context, _ *pb.Empty) (*pb.RoutesResponse, error) {
	routes := s.Impl.Routes()
	resp := &pb.RoutesResponse{}
	for _, r := range routes {
		resp.Routes = append(resp.Routes, &pb.RouteDefinitionProto{
			Method:      r.Method,
			Path:        r.Path,
			Description: r.Description,
		})
	}
	return resp, nil
}

// buildAccount 从 proto AccountProto 构建 SDK Account
func buildAccount(req *pb.ForwardRequest) *sdk.Account {
	a := req.Account
	if a == nil {
		return &sdk.Account{}
	}
	var creds map[string]string
	if len(a.CredentialsJson) > 0 {
		if err := json.Unmarshal(a.CredentialsJson, &creds); err != nil {
			creds = make(map[string]string)
		}
	}
	return &sdk.Account{
		ID:          a.Id,
		Name:        a.Name,
		Platform:    a.Platform,
		Type:        a.Type,
		Credentials: creds,
		ProxyURL:    a.ProxyUrl,
	}
}

// toProtoResult 将 SDK ForwardResult 转为 proto ForwardResult
func toProtoResult(result *sdk.ForwardResult) *pb.ForwardResult {
	return &pb.ForwardResult{
		StatusCode:            int64(result.StatusCode),
		InputTokens:           int64(result.InputTokens),
		OutputTokens:          int64(result.OutputTokens),
		CachedInputTokens:     int64(result.CachedInputTokens),
		ReasoningOutputTokens: int64(result.ReasoningOutputTokens),
		Model:                 result.Model,
		DurationMs:            result.Duration.Milliseconds(),
		FirstTokenMs:          result.FirstTokenMs,
		AccountStatus:         string(result.AccountStatus),
		ErrorMessage:          result.ErrorMessage,
		RetryAfterMs:          result.RetryAfter.Milliseconds(),
		ServiceTier:           result.ServiceTier,
		UpdatedCredentials:    result.UpdatedCredentials,
		InputCost:             result.InputCost,
		OutputCost:            result.OutputCost,
		CachedInputCost:       result.CachedInputCost,
		InputPrice:            result.InputPrice,
		OutputPrice:           result.OutputPrice,
		CachedInputPrice:      result.CachedInputPrice,
	}
}

// protoHeadersToHTTP 将 proto 多值 Headers 转为 http.Header
func protoHeadersToHTTP(ph map[string]*pb.HeaderValues) http.Header {
	h := make(http.Header, len(ph))
	for k, v := range ph {
		if v != nil {
			h[k] = v.Values
		}
	}
	return h
}

// httpHeadersToProto 将 http.Header 转为 proto 多值 Headers
func httpHeadersToProto(h http.Header) map[string]*pb.HeaderValues {
	ph := make(map[string]*pb.HeaderValues, len(h))
	for k, v := range h {
		ph[k] = &pb.HeaderValues{Values: v}
	}
	return ph
}

func (s *GatewayGRPCServer) Forward(ctx context.Context, req *pb.ForwardRequest) (*pb.ForwardResult, error) {
	headers := protoHeadersToHTTP(req.Headers)

	// 非流式请求：使用 bufferWriter 捕获响应体
	bw := &bufferWriter{}
	fwdReq := &sdk.ForwardRequest{
		Account: buildAccount(req),
		Body:    req.Body,
		Headers: headers,
		Model:   req.Model,
		Stream:  req.Stream,
		Writer:  bw,
	}

	result, err := s.Impl.Forward(ctx, fwdReq)
	// 即便插件返回了 err，只要带回了 result（其中可能包含 AccountStatus、StatusCode、
	// ErrorMessage 等用于 core 端调度决策的关键字段），也要把 result 透传给客户端，
	// 不要让 err 把这些信号吞掉。err 文本回填到 ErrorMessage。
	if err != nil {
		if result == nil {
			return nil, err
		}
		if result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
	}
	pbResult := toProtoResult(result)
	// 优先使用 bufferWriter 捕获的响应，回退到 result 自身
	if len(bw.body) > 0 {
		pbResult.Body = bw.body
		pbResult.Headers = httpHeadersToProto(bw.Header())
	}
	if pbResult.StatusCode == 0 && bw.code > 0 {
		pbResult.StatusCode = int64(bw.code)
	}
	return pbResult, nil
}

func (s *GatewayGRPCServer) ForwardStream(req *pb.ForwardRequest, stream pb.GatewayService_ForwardStreamServer) error {
	headers := protoHeadersToHTTP(req.Headers)

	sw := &streamWriter{stream: stream}
	fwdReq := &sdk.ForwardRequest{
		Account: buildAccount(req),
		Body:    req.Body,
		Headers: headers,
		Model:   req.Model,
		Stream:  true,
		Writer:  sw,
	}

	startTime := time.Now()
	result, err := s.Impl.Forward(stream.Context(), fwdReq)
	// 与非流式 Forward 同理：err 非 nil 但 result 也非 nil 时，把 result 透传给客户端，
	// 让 AccountStatus 等调度信号能传递回 core，避免客户端侧的请求错误被记到账号头上。
	if err != nil {
		if result == nil {
			return err
		}
		if result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
	}

	// 补充耗时
	if result.Duration == 0 {
		result.Duration = time.Since(startTime)
	}

	if err := sw.flushMeta(); err != nil {
		return err
	}

	return stream.Send(&pb.ForwardChunk{
		Done:        true,
		FinalResult: toProtoResult(result),
	})
}

func (s *GatewayGRPCServer) ValidateAccount(ctx context.Context, req *pb.CredentialsRequest) (*pb.Empty, error) {
	if err := s.Impl.ValidateAccount(ctx, req.Credentials); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *GatewayGRPCServer) QueryQuota(ctx context.Context, req *pb.CredentialsRequest) (*pb.QuotaInfoResponse, error) {
	info, err := s.Impl.QueryQuota(ctx, req.Credentials)
	if err != nil {
		return nil, err
	}
	return &pb.QuotaInfoResponse{
		Total:     info.Total,
		Used:      info.Used,
		Remaining: info.Remaining,
		Currency:  info.Currency,
		ExpiresAt: info.ExpiresAt,
		Extra:     info.Extra,
	}, nil
}

// streamWriter 将 gRPC 流包装为 http.ResponseWriter
type streamWriter struct {
	stream  pb.GatewayService_ForwardStreamServer
	headers http.Header
	code    int
	sent    bool
}

func (w *streamWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

// streamChunkSize 是单条 ForwardChunk 携带的最大字节数。
// 故意远小于 PluginGRPCMaxMessageBytes（64 MB），既留足 proto 编码 / metadata 余量，
// 又能让上游一次性 dump 的大事件被切片成多条 gRPC 消息发送，避免击穿对端的接收上限。
const streamChunkSize = 256 * 1024

func (w *streamWriter) Write(data []byte) (int, error) {
	if err := w.flushMeta(); err != nil {
		return 0, err
	}
	// 切块发送：单条 gRPC 消息最大 streamChunkSize 字节，避免大事件击穿对端接收上限。
	// data 的所有内容都需要发送，所以即便长度为 0 也保留一次空 Send 的语义（与原行为一致）。
	total := len(data)
	if total == 0 {
		if err := w.stream.Send(&pb.ForwardChunk{Data: data}); err != nil {
			return 0, err
		}
		return 0, nil
	}
	for offset := 0; offset < total; offset += streamChunkSize {
		end := offset + streamChunkSize
		if end > total {
			end = total
		}
		if err := w.stream.Send(&pb.ForwardChunk{Data: data[offset:end]}); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func (w *streamWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *streamWriter) flushMeta() error {
	if w.sent {
		return nil
	}
	w.sent = true // 提前标记，防止失败后重复发送
	statusCode := w.code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	return w.stream.Send(&pb.ForwardChunk{
		StatusCode: int32(statusCode),
		Headers:    httpHeadersToProto(w.Header()),
	})
}

// bufferWriter 缓冲响应体的 http.ResponseWriter 实现（非流式 gRPC 用）
type bufferWriter struct {
	headers http.Header
	code    int
	body    []byte
}

func (w *bufferWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *bufferWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *bufferWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}
