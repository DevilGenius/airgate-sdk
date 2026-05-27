package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
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
	case sdk.OutcomeAccountModelUnsupported: //nolint:staticcheck // 兼容旧插件
		return pb.OutcomeKind_OUTCOME_CLIENT_ERROR
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
	case pb.OutcomeKind_OUTCOME_ACCOUNT_MODEL_UNSUPPORTED:
		return sdk.OutcomeClientError
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
		Model:             u.Model,
		AccountCost:       u.AccountCost,
		UserCost:          u.UserCost,
		BillingMultiplier: u.BillingMultiplier,
		Currency:          u.Currency,
		Summary:           u.Summary,
		FirstTokenMs:      u.FirstTokenMs,
		Metadata:          u.Metadata,
	}
	out.Attributes = usageAttributesToProto(u.Attributes)
	out.Metrics = usageMetricsToProto(u.Metrics)
	out.CostDetails = usageCostDetailsToProto(u.CostDetails)
	return out
}

func usageFromProto(p *pb.Usage) sdk.Usage {
	out := sdk.Usage{
		Model:             p.Model,
		AccountCost:       p.AccountCost,
		UserCost:          p.UserCost,
		BillingMultiplier: p.BillingMultiplier,
		Currency:          p.Currency,
		Summary:           p.Summary,
		FirstTokenMs:      p.FirstTokenMs,
		Metadata:          p.Metadata,
	}
	out.Attributes = usageAttributesFromProto(p.Attributes)
	out.Metrics = usageMetricsFromProto(p.Metrics)
	out.CostDetails = usageCostDetailsFromProto(p.CostDetails)
	return out
}

func usageAttributesToProto(attrs []sdk.UsageAttribute) []*pb.UsageAttribute {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]*pb.UsageAttribute, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, usageAttributeToProto(a))
	}
	return out
}

func usageAttributesFromProto(attrs []*pb.UsageAttribute) []sdk.UsageAttribute {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]sdk.UsageAttribute, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, usageAttributeFromProto(a))
	}
	return out
}

func usageAttributeToProto(a sdk.UsageAttribute) *pb.UsageAttribute {
	return &pb.UsageAttribute{
		Key:      a.Key,
		Label:    a.Label,
		Kind:     a.Kind,
		Value:    a.Value,
		Metadata: a.Metadata,
	}
}

func usageAttributeFromProto(p *pb.UsageAttribute) sdk.UsageAttribute {
	if p == nil {
		return sdk.UsageAttribute{}
	}
	return sdk.UsageAttribute{
		Key:      p.Key,
		Label:    p.Label,
		Kind:     p.Kind,
		Value:    p.Value,
		Metadata: p.Metadata,
	}
}

func usageMetricsToProto(metrics []sdk.UsageMetric) []*pb.UsageMetric {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]*pb.UsageMetric, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, usageMetricToProto(m))
	}
	return out
}

func usageMetricsFromProto(metrics []*pb.UsageMetric) []sdk.UsageMetric {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]sdk.UsageMetric, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, usageMetricFromProto(m))
	}
	return out
}

func usageMetricToProto(m sdk.UsageMetric) *pb.UsageMetric {
	return &pb.UsageMetric{
		Key:         m.Key,
		Label:       m.Label,
		Kind:        m.Kind,
		Unit:        m.Unit,
		Value:       m.Value,
		AccountCost: m.AccountCost,
		Currency:    m.Currency,
		Metadata:    m.Metadata,
	}
}

func usageMetricFromProto(p *pb.UsageMetric) sdk.UsageMetric {
	if p == nil {
		return sdk.UsageMetric{}
	}
	return sdk.UsageMetric{
		Key:         p.Key,
		Label:       p.Label,
		Kind:        p.Kind,
		Unit:        p.Unit,
		Value:       p.Value,
		AccountCost: p.AccountCost,
		Currency:    p.Currency,
		Metadata:    p.Metadata,
	}
}

func usageCostDetailsToProto(details []sdk.UsageCostDetail) []*pb.UsageCostDetail {
	if len(details) == 0 {
		return nil
	}
	out := make([]*pb.UsageCostDetail, 0, len(details))
	for _, c := range details {
		out = append(out, usageCostDetailToProto(c))
	}
	return out
}

func usageCostDetailsFromProto(details []*pb.UsageCostDetail) []sdk.UsageCostDetail {
	if len(details) == 0 {
		return nil
	}
	out := make([]sdk.UsageCostDetail, 0, len(details))
	for _, c := range details {
		out = append(out, usageCostDetailFromProto(c))
	}
	return out
}

func usageCostDetailToProto(c sdk.UsageCostDetail) *pb.UsageCostDetail {
	return &pb.UsageCostDetail{
		Key:               c.Key,
		Label:             c.Label,
		AccountCost:       c.AccountCost,
		UserCost:          c.UserCost,
		BillingMultiplier: c.BillingMultiplier,
		Currency:          c.Currency,
		Metadata:          c.Metadata,
	}
}

func usageCostDetailFromProto(p *pb.UsageCostDetail) sdk.UsageCostDetail {
	if p == nil {
		return sdk.UsageCostDetail{}
	}
	return sdk.UsageCostDetail{
		Key:               p.Key,
		Label:             p.Label,
		AccountCost:       p.AccountCost,
		UserCost:          p.UserCost,
		BillingMultiplier: p.BillingMultiplier,
		Currency:          p.Currency,
		Metadata:          p.Metadata,
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
	// 非流式：用 bufferWriter 兜底捕获插件可能意外写入 Writer 的内容。
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
		Account: buildAccount(req),
		Body:    req.Body,
		Headers: protoHeadersToHTTP(req.Headers),
		Model:   req.Model,
		Stream:  true,
		Writer:  sw,
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
