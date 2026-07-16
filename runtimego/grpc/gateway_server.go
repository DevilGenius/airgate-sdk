package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
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
	if len(models) > 0 {
		resp.Models = make([]*pb.ModelInfoProto, 0, len(models))
	}
	for _, m := range models {
		resp.Models = append(resp.Models, &pb.ModelInfoProto{
			Id:              m.ID,
			Name:            m.Name,
			ContextWindow:   int64(m.ContextWindow),
			MaxOutputTokens: int64(m.MaxOutputTokens),
			Capabilities:    m.Capabilities,
			Metadata:        m.Metadata,
		})
	}
	return resp, nil
}

func (s *GatewayGRPCServer) GetRoutes(_ context.Context, _ *pb.Empty) (*pb.RoutesResponse, error) {
	routes := s.Impl.Routes()
	resp := &pb.RoutesResponse{}
	if len(routes) > 0 {
		resp.Routes = make([]*pb.RouteDefinitionProto, 0, len(routes))
	}
	for _, r := range routes {
		resp.Routes = append(resp.Routes, &pb.RouteDefinitionProto{
			Method:      r.Method,
			Path:        r.Path,
			Description: r.Description,
			Metadata:    r.Metadata,
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
		FailoverScope:      string(o.FailoverScope),
		RerouteClientModel: o.RerouteClientModel,
		Upstream:           upstreamToProto(o.Upstream),
		DurationMs:         o.Duration.Milliseconds(),
		RetryAfterMs:       o.RetryAfter.Milliseconds(),
		Reason:             o.Reason,
		UpdatedCredentials: o.UpdatedCredentials,
		SafetyRejected:     o.SafetyRejected,
	}
	if o.Usage != nil {
		out.Usage = usageToProto(*o.Usage)
	}
	if o.FinalErrorDiagnostic != nil {
		out.FinalErrorDiagnostic = finalErrorDiagnosticToProto(o.FinalErrorDiagnostic)
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
		FailoverScope:      sdk.FailoverScope(p.FailoverScope),
		RerouteClientModel: p.RerouteClientModel,
		Upstream:           upstreamFromProto(p.Upstream),
		Duration:           time.Duration(p.DurationMs) * time.Millisecond,
		RetryAfter:         time.Duration(p.RetryAfterMs) * time.Millisecond,
		Reason:             p.Reason,
		UpdatedCredentials: p.UpdatedCredentials,
		SafetyRejected:     p.SafetyRejected,
	}
	if p.Usage != nil {
		u := usageFromProto(p.Usage)
		out.Usage = &u
	}
	if p.FinalErrorDiagnostic != nil {
		out.FinalErrorDiagnostic = finalErrorDiagnosticFromProto(p.FinalErrorDiagnostic)
	}
	return out
}

func finalErrorDiagnosticToProto(d *sdk.FinalErrorDiagnostic) *pb.FinalErrorDiagnostic {
	if d == nil {
		return nil
	}
	out := &pb.FinalErrorDiagnostic{UpstreamErrorBody: d.UpstreamErrorBody}
	if len(d.OutboundRequests) > 0 {
		out.OutboundRequests = make([]*pb.OutboundRequestDiagnostic, 0, len(d.OutboundRequests))
		for _, request := range d.OutboundRequests {
			out.OutboundRequests = append(out.OutboundRequests, &pb.OutboundRequestDiagnostic{
				Transport:           request.Transport,
				Method:              request.Method,
				Url:                 request.URL,
				Headers:             httpHeadersToProto(request.Headers),
				Body:                request.Body,
				StatusCode:          int32(request.StatusCode),
				BodyRedacted:        request.BodyRedacted,
				BodyRedactionReason: request.BodyRedactionReason,
				BodyOriginalSize:    request.BodyOriginalSize,
			})
		}
	}
	return out
}

func finalErrorDiagnosticFromProto(d *pb.FinalErrorDiagnostic) *sdk.FinalErrorDiagnostic {
	if d == nil {
		return nil
	}
	out := &sdk.FinalErrorDiagnostic{UpstreamErrorBody: d.UpstreamErrorBody}
	if len(d.OutboundRequests) > 0 {
		out.OutboundRequests = make([]sdk.OutboundRequestDiagnostic, 0, len(d.OutboundRequests))
		for _, request := range d.OutboundRequests {
			if request == nil {
				continue
			}
			out.OutboundRequests = append(out.OutboundRequests, sdk.OutboundRequestDiagnostic{
				Transport:           request.Transport,
				Method:              request.Method,
				URL:                 request.Url,
				Headers:             protoHeadersToHTTP(request.Headers),
				Body:                request.Body,
				StatusCode:          int(request.StatusCode),
				BodyRedacted:        request.BodyRedacted,
				BodyRedactionReason: request.BodyRedactionReason,
				BodyOriginalSize:    request.BodyOriginalSize,
			})
		}
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
	case sdk.OutcomeFamilyTransient:
		return pb.OutcomeKind_OUTCOME_FAMILY_TRANSIENT
	case sdk.OutcomeAccountUnavailable:
		return pb.OutcomeKind_OUTCOME_ACCOUNT_UNAVAILABLE
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
	case pb.OutcomeKind_OUTCOME_FAMILY_TRANSIENT:
		return sdk.OutcomeFamilyTransient
	case pb.OutcomeKind_OUTCOME_ACCOUNT_UNAVAILABLE:
		return sdk.OutcomeAccountUnavailable
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
	out := &pb.Usage{
		Model:                 u.Model,
		AccountCost:           u.AccountCost,
		UserCost:              u.UserCost,
		BillingMultiplier:     u.BillingMultiplier,
		Currency:              u.Currency,
		Summary:               u.Summary,
		FirstEventMs:          u.FirstEventMs,
		FirstTokenMs:          u.FirstTokenMs,
		WsDialMs:              u.WSDialMs,
		Metadata:              u.Metadata,
		InputTokens:           int64(u.InputTokens),
		OutputTokens:          int64(u.OutputTokens),
		CachedInputTokens:     int64(u.CachedInputTokens),
		CacheCreationTokens:   int64(u.CacheCreationTokens),
		ReasoningOutputTokens: int64(u.ReasoningOutputTokens),
		ReasoningEffort:       u.ReasoningEffort,
		InputPrice:            u.InputPrice,
		OutputPrice:           u.OutputPrice,
		CachedInputPrice:      u.CachedInputPrice,
		CacheCreationPrice:    u.CacheCreationPrice,
		InputCost:             u.InputCost,
		OutputCost:            u.OutputCost,
		CachedInputCost:       u.CachedInputCost,
		CacheCreationCost:     u.CacheCreationCost,
	}
	return out
}

func usageFromProto(p *pb.Usage) sdk.Usage {
	out := sdk.Usage{
		Model:                 p.Model,
		AccountCost:           p.AccountCost,
		UserCost:              p.UserCost,
		BillingMultiplier:     p.BillingMultiplier,
		Currency:              p.Currency,
		Summary:               p.Summary,
		FirstEventMs:          p.FirstEventMs,
		FirstTokenMs:          p.FirstTokenMs,
		WSDialMs:              p.WsDialMs,
		Metadata:              p.Metadata,
		InputTokens:           int(p.InputTokens),
		OutputTokens:          int(p.OutputTokens),
		CachedInputTokens:     int(p.CachedInputTokens),
		CacheCreationTokens:   int(p.CacheCreationTokens),
		ReasoningOutputTokens: int(p.ReasoningOutputTokens),
		ReasoningEffort:       p.ReasoningEffort,
		InputPrice:            p.InputPrice,
		OutputPrice:           p.OutputPrice,
		CachedInputPrice:      p.CachedInputPrice,
		CacheCreationPrice:    p.CacheCreationPrice,
		InputCost:             p.InputCost,
		OutputCost:            p.OutputCost,
		CachedInputCost:       p.CachedInputCost,
		CacheCreationCost:     p.CacheCreationCost,
	}
	return out
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
	// 非流式：用 bufferWriter 兜底捕获插件可能意外写入 Writer 的内容。
	bw := &bufferWriter{}
	fwdReq := &sdk.ForwardRequest{
		Account:         buildAccount(req),
		Body:            req.Body,
		Headers:         protoHeadersToHTTP(req.Headers),
		Model:           req.Model,
		DispatchPlan:    dispatchPlanFromProto(req.DispatchPlan),
		Stream:          req.Stream,
		TraceFinalError: req.TraceFinalError,
		Writer:          bw,
	}

	outcome, err := s.Impl.Forward(ctx, fwdReq)
	// 契约：Kind 是权威。插件常见写法 `return (outcome, err)` 同时给 Kind 和 err 文本。
	// 只有当 Kind=Unknown 时才通过 gRPC error 上抛（真正的"插件自己崩了"）；
	// 否则把 err 合并进 outcome.Reason，保证 Core 端能拿到判决。
	if err != nil && outcome.Kind == sdk.OutcomeUnknown {
		// server 拦截器会打 grpc_server_handle_failed；这里补业务上下文，便于排查。
		sdk.LoggerFromContext(ctx).Error("gateway_forward_failed",
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	if err != nil && outcome.Reason == "" {
		outcome.Reason = err.Error()
	}
	if bw.err != nil {
		return nil, bw.err
	}

	pbOutcome := outcomeToProto(outcome)
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
		Account:         buildAccount(req),
		Body:            req.Body,
		Headers:         protoHeadersToHTTP(req.Headers),
		Model:           req.Model,
		DispatchPlan:    dispatchPlanFromProto(req.DispatchPlan),
		Stream:          true,
		TraceFinalError: req.TraceFinalError,
		Writer:          sw,
	}

	startTime := time.Now()
	ctx := stream.Context()
	outcome, err := s.Impl.Forward(ctx, fwdReq)
	// 同 Forward：Kind=Unknown 才走 gRPC error；否则合并 err 进 Reason，保留判决。
	if err != nil && outcome.Kind == sdk.OutcomeUnknown {
		sdk.LoggerFromContext(ctx).Error("gateway_forward_stream_failed",
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldDurationMs, time.Since(startTime).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	if err != nil && outcome.Reason == "" {
		outcome.Reason = err.Error()
	}
	if outcome.Duration == 0 {
		outcome.Duration = time.Since(startTime)
	}

	if sw.sent || sw.wroteHeader {
		if err := sw.flushMeta(); err != nil {
			sdk.LoggerFromContext(ctx).Error("gateway_forward_stream_meta_flush_failed",
				sdk.LogFieldModel, req.Model,
				sdk.LogFieldError, err,
			)
			return err
		}
	}
	if err := stream.Send(&pb.ForwardChunk{
		Done:         true,
		FinalOutcome: outcomeToProto(outcome),
	}); err != nil {
		sdk.LoggerFromContext(ctx).Error("gateway_forward_stream_send_final_failed",
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldError, err,
		)
		return err
	}
	return nil
}

func (s *GatewayGRPCServer) ValidateAccount(ctx context.Context, req *pb.CredentialsRequest) (*pb.Empty, error) {
	if err := s.Impl.ValidateAccount(ctx, req.Credentials); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// streamWriter 把 gRPC 流包装成 http.ResponseWriter。
type streamWriter struct {
	stream      pb.GatewayService_ForwardStreamServer
	headers     http.Header
	code        int
	wroteHeader bool
	sent        bool
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

func (w *streamWriter) WriteHeader(statusCode int) {
	if w.sent || w.wroteHeader {
		return
	}
	w.code = statusCode
	w.wroteHeader = true
}

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
	err     error
}

func (w *bufferWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *bufferWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if len(w.body)+len(data) > PluginGRPCMaxMessageBytes {
		w.err = fmt.Errorf("buffered gateway response exceeds %d bytes", PluginGRPCMaxMessageBytes)
		return 0, w.err
	}
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *bufferWriter) WriteHeader(statusCode int) { w.code = statusCode }
