package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// GatewayGRPCServer 将 GatewayPlugin 包装为 gRPC 服务端。
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
			CacheCreationPrice:       m.CacheCreationPrice,
			CacheCreation_1HPrice:    m.CacheCreation1hPrice,
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

// outcomeToProto 把 SDK 判决转为 proto 消息。
func outcomeToProto(o sdk.ForwardOutcome) *pb.ForwardOutcome {
	out := &pb.ForwardOutcome{
		Kind:               outcomeKindToProto(o.Kind),
		Upstream:           upstreamToProto(o.Upstream),
		DurationMs:         o.Duration.Milliseconds(),
		RetryAfterMs:       o.RetryAfter.Milliseconds(),
		Reason:             o.Reason,
		UpdatedCredentials: o.UpdatedCredentials,
	}
	if o.Usage != nil {
		out.Usage = usageToProto(*o.Usage)
	}
	return out
}

// outcomeFromProto 把 proto 消息转为 SDK 判决。
func outcomeFromProto(p *pb.ForwardOutcome) sdk.ForwardOutcome {
	if p == nil {
		return sdk.ForwardOutcome{}
	}
	out := sdk.ForwardOutcome{
		Kind:               outcomeKindFromProto(p.Kind),
		Upstream:           upstreamFromProto(p.Upstream),
		Duration:           time.Duration(p.DurationMs) * time.Millisecond,
		RetryAfter:         time.Duration(p.RetryAfterMs) * time.Millisecond,
		Reason:             p.Reason,
		UpdatedCredentials: p.UpdatedCredentials,
	}
	if p.Usage != nil {
		u := usageFromProto(p.Usage)
		out.Usage = &u
	}
	return out
}

func outcomeKindToProto(k sdk.OutcomeKind) pb.OutcomeKind {
	switch k {
	case sdk.OutcomeSuccess:
		return pb.OutcomeKind_OUTCOME_SUCCESS
	case sdk.OutcomeClientError:
		return pb.OutcomeKind_OUTCOME_CLIENT_ERROR
	case sdk.OutcomeAccountRateLimited:
		return pb.OutcomeKind_OUTCOME_ACCOUNT_RATE_LIMITED
	case sdk.OutcomeAccountDead:
		return pb.OutcomeKind_OUTCOME_ACCOUNT_DEAD
	case sdk.OutcomeUpstreamTransient:
		return pb.OutcomeKind_OUTCOME_UPSTREAM_TRANSIENT
	case sdk.OutcomeStreamAborted:
		return pb.OutcomeKind_OUTCOME_STREAM_ABORTED
	default:
		return pb.OutcomeKind_OUTCOME_UNKNOWN
	}
}

func outcomeKindFromProto(k pb.OutcomeKind) sdk.OutcomeKind {
	switch k {
	case pb.OutcomeKind_OUTCOME_SUCCESS:
		return sdk.OutcomeSuccess
	case pb.OutcomeKind_OUTCOME_CLIENT_ERROR:
		return sdk.OutcomeClientError
	case pb.OutcomeKind_OUTCOME_ACCOUNT_RATE_LIMITED:
		return sdk.OutcomeAccountRateLimited
	case pb.OutcomeKind_OUTCOME_ACCOUNT_DEAD:
		return sdk.OutcomeAccountDead
	case pb.OutcomeKind_OUTCOME_UPSTREAM_TRANSIENT:
		return sdk.OutcomeUpstreamTransient
	case pb.OutcomeKind_OUTCOME_STREAM_ABORTED:
		return sdk.OutcomeStreamAborted
	default:
		return sdk.OutcomeUnknown
	}
}

func upstreamToProto(u sdk.UpstreamResponse) *pb.UpstreamResponse {
	return &pb.UpstreamResponse{
		StatusCode: int32(u.StatusCode),
		Headers:    httpHeadersToProto(u.Headers),
		Body:       u.Body,
	}
}

func upstreamFromProto(p *pb.UpstreamResponse) sdk.UpstreamResponse {
	if p == nil {
		return sdk.UpstreamResponse{}
	}
	return sdk.UpstreamResponse{
		StatusCode: int(p.StatusCode),
		Headers:    protoHeadersToHTTP(p.Headers),
		Body:       p.Body,
	}
}

func usageToProto(u sdk.Usage) *pb.Usage {
	return &pb.Usage{
		InputTokens:            int64(u.InputTokens),
		OutputTokens:           int64(u.OutputTokens),
		CachedInputTokens:      int64(u.CachedInputTokens),
		CacheCreationTokens:    int64(u.CacheCreationTokens),
		CacheCreation_5MTokens: int64(u.CacheCreation5mTokens),
		CacheCreation_1HTokens: int64(u.CacheCreation1hTokens),
		ReasoningOutputTokens:  int64(u.ReasoningOutputTokens),
		InputCost:              u.InputCost,
		OutputCost:             u.OutputCost,
		CachedInputCost:        u.CachedInputCost,
		CacheCreationCost:      u.CacheCreationCost,
		InputPrice:             u.InputPrice,
		OutputPrice:            u.OutputPrice,
		CachedInputPrice:       u.CachedInputPrice,
		CacheCreationPrice:     u.CacheCreationPrice,
		CacheCreation_1HPrice:  u.CacheCreation1hPrice,
		Model:                  u.Model,
		ServiceTier:            u.ServiceTier,
		FirstTokenMs:           u.FirstTokenMs,
	}
}

func usageFromProto(p *pb.Usage) sdk.Usage {
	return sdk.Usage{
		InputTokens:           int(p.InputTokens),
		OutputTokens:          int(p.OutputTokens),
		CachedInputTokens:     int(p.CachedInputTokens),
		CacheCreationTokens:   int(p.CacheCreationTokens),
		CacheCreation5mTokens: int(p.CacheCreation_5MTokens),
		CacheCreation1hTokens: int(p.CacheCreation_1HTokens),
		ReasoningOutputTokens: int(p.ReasoningOutputTokens),
		InputCost:             p.InputCost,
		OutputCost:            p.OutputCost,
		CachedInputCost:       p.CachedInputCost,
		CacheCreationCost:     p.CacheCreationCost,
		InputPrice:            p.InputPrice,
		OutputPrice:           p.OutputPrice,
		CachedInputPrice:      p.CachedInputPrice,
		CacheCreationPrice:    p.CacheCreationPrice,
		CacheCreation1hPrice:  p.CacheCreation_1HPrice,
		Model:                 p.Model,
		ServiceTier:           p.ServiceTier,
		FirstTokenMs:          p.FirstTokenMs,
	}
}

func protoHeadersToHTTP(ph map[string]*pb.HeaderValues) http.Header {
	h := make(http.Header, len(ph))
	for k, v := range ph {
		if v != nil {
			h[k] = v.Values
		}
	}
	return h
}

func httpHeadersToProto(h http.Header) map[string]*pb.HeaderValues {
	ph := make(map[string]*pb.HeaderValues, len(h))
	for k, v := range h {
		ph[k] = &pb.HeaderValues{Values: v}
	}
	return ph
}

func (s *GatewayGRPCServer) Forward(ctx context.Context, req *pb.ForwardRequest) (*pb.ForwardOutcome, error) {
	// 非流式：用 bufferWriter 兜底捕获插件可能意外写入 Writer 的内容
	// （正常实现应通过 Outcome.Upstream.Body 返回）。
	bw := &bufferWriter{}
	fwdReq := &sdk.ForwardRequest{
		Account: buildAccount(req),
		Body:    req.Body,
		Headers: protoHeadersToHTTP(req.Headers),
		Model:   req.Model,
		Stream:  req.Stream,
		Writer:  bw,
	}

	outcome, err := s.Impl.Forward(ctx, fwdReq)
	if err != nil {
		return nil, err
	}

	pbOutcome := outcomeToProto(outcome)
	// 如果插件用了 Writer 而不是 Outcome.Upstream，把 buffer 补回 Upstream
	if len(bw.body) > 0 && (pbOutcome.Upstream == nil || len(pbOutcome.Upstream.Body) == 0) {
		if pbOutcome.Upstream == nil {
			pbOutcome.Upstream = &pb.UpstreamResponse{}
		}
		pbOutcome.Upstream.Body = bw.body
		if pbOutcome.Upstream.StatusCode == 0 && bw.code > 0 {
			pbOutcome.Upstream.StatusCode = int32(bw.code)
		}
		if len(pbOutcome.Upstream.Headers) == 0 {
			pbOutcome.Upstream.Headers = httpHeadersToProto(bw.Header())
		}
	}
	return pbOutcome, nil
}

func (s *GatewayGRPCServer) ForwardStream(req *pb.ForwardRequest, stream pb.GatewayService_ForwardStreamServer) error {
	sw := &streamWriter{stream: stream}
	fwdReq := &sdk.ForwardRequest{
		Account: buildAccount(req),
		Body:    req.Body,
		Headers: protoHeadersToHTTP(req.Headers),
		Model:   req.Model,
		Stream:  true,
		Writer:  sw,
	}

	startTime := time.Now()
	outcome, err := s.Impl.Forward(stream.Context(), fwdReq)
	if err != nil {
		return err
	}
	if outcome.Duration == 0 {
		outcome.Duration = time.Since(startTime)
	}

	if err := sw.flushMeta(); err != nil {
		return err
	}
	return stream.Send(&pb.ForwardChunk{
		Done:         true,
		FinalOutcome: outcomeToProto(outcome),
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

// streamWriter 把 gRPC 流包装成 http.ResponseWriter。
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

// streamChunkSize 单条 ForwardChunk 携带的最大字节数。远小于 gRPC message 上限，
// 给 proto 编码 / metadata 留足余量，同时把上游的大事件切片避免击穿对端接收限制。
const streamChunkSize = 256 * 1024

func (w *streamWriter) Write(data []byte) (int, error) {
	if err := w.flushMeta(); err != nil {
		return 0, err
	}
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

func (w *streamWriter) WriteHeader(statusCode int) { w.code = statusCode }

func (w *streamWriter) flushMeta() error {
	if w.sent {
		return nil
	}
	w.sent = true // 提前置位，失败后不再重发
	statusCode := w.code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	return w.stream.Send(&pb.ForwardChunk{
		StatusCode: int32(statusCode),
		Headers:    httpHeadersToProto(w.Header()),
	})
}

// bufferWriter 兜底捕获插件意外写入 Writer 的非流式响应。
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

func (w *bufferWriter) WriteHeader(statusCode int) { w.code = statusCode }
