package grpc

import (
	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

func payloadSchemaToProto(s sdk.PayloadSchema) *pb.PayloadSchemaProto {
	if s.ContentType == "" && s.Schema == "" && s.Example == "" && len(s.Metadata) == 0 {
		return nil
	}
	return &pb.PayloadSchemaProto{
		ContentType: s.ContentType,
		Schema:      s.Schema,
		Example:     s.Example,
		Metadata:    s.Metadata,
	}
}

func payloadSchemaFromProto(p *pb.PayloadSchemaProto) sdk.PayloadSchema {
	if p == nil {
		return sdk.PayloadSchema{}
	}
	return sdk.PayloadSchema{
		ContentType: p.ContentType,
		Schema:      p.Schema,
		Example:     p.Example,
		Metadata:    p.Metadata,
	}
}

func schemaToProto(s sdk.PluginSchema) *pb.PluginSchemaResponse {
	out := &pb.PluginSchemaResponse{Metadata: s.Metadata}
	if len(s.Routes) > 0 {
		out.Routes = make([]*pb.RouteSchemaProto, 0, len(s.Routes))
		for _, r := range s.Routes {
			out.Routes = append(out.Routes, &pb.RouteSchemaProto{
				Method:   r.Method,
				Path:     r.Path,
				Summary:  r.Summary,
				Request:  payloadSchemaToProto(r.Request),
				Response: payloadSchemaToProto(r.Response),
				Metadata: r.Metadata,
			})
		}
	}
	if len(s.Tasks) > 0 {
		out.Tasks = make([]*pb.TaskSchemaProto, 0, len(s.Tasks))
		for _, t := range s.Tasks {
			out.Tasks = append(out.Tasks, &pb.TaskSchemaProto{
				Type:     t.Type,
				Summary:  t.Summary,
				Input:    payloadSchemaToProto(t.Input),
				Output:   payloadSchemaToProto(t.Output),
				Metadata: t.Metadata,
			})
		}
	}
	if len(s.Events) > 0 {
		out.Events = make([]*pb.EventSchemaProto, 0, len(s.Events))
		for _, e := range s.Events {
			out.Events = append(out.Events, &pb.EventSchemaProto{
				Type:     e.Type,
				Source:   e.Source,
				Summary:  e.Summary,
				Payload:  payloadSchemaToProto(e.Payload),
				Metadata: e.Metadata,
			})
		}
	}
	if len(s.Invokes) > 0 {
		out.Invokes = make([]*pb.InvokeSchemaProto, 0, len(s.Invokes))
		for _, i := range s.Invokes {
			out.Invokes = append(out.Invokes, &pb.InvokeSchemaProto{
				Method:   i.Method,
				Summary:  i.Summary,
				Request:  payloadSchemaToProto(i.Request),
				Response: payloadSchemaToProto(i.Response),
				Metadata: i.Metadata,
			})
		}
	}
	return out
}

func schemaFromProto(p *pb.PluginSchemaResponse) sdk.PluginSchema {
	if p == nil {
		return sdk.PluginSchema{}
	}
	out := sdk.PluginSchema{Metadata: p.Metadata}
	if len(p.Routes) > 0 {
		out.Routes = make([]sdk.RouteSchema, 0, len(p.Routes))
		for _, r := range p.Routes {
			out.Routes = append(out.Routes, sdk.RouteSchema{
				Method:   r.Method,
				Path:     r.Path,
				Summary:  r.Summary,
				Request:  payloadSchemaFromProto(r.Request),
				Response: payloadSchemaFromProto(r.Response),
				Metadata: r.Metadata,
			})
		}
	}
	if len(p.Tasks) > 0 {
		out.Tasks = make([]sdk.TaskSchema, 0, len(p.Tasks))
		for _, t := range p.Tasks {
			out.Tasks = append(out.Tasks, sdk.TaskSchema{
				Type:     t.Type,
				Summary:  t.Summary,
				Input:    payloadSchemaFromProto(t.Input),
				Output:   payloadSchemaFromProto(t.Output),
				Metadata: t.Metadata,
			})
		}
	}
	if len(p.Events) > 0 {
		out.Events = make([]sdk.EventSchema, 0, len(p.Events))
		for _, e := range p.Events {
			out.Events = append(out.Events, sdk.EventSchema{
				Type:     e.Type,
				Source:   e.Source,
				Summary:  e.Summary,
				Payload:  payloadSchemaFromProto(e.Payload),
				Metadata: e.Metadata,
			})
		}
	}
	if len(p.Invokes) > 0 {
		out.Invokes = make([]sdk.InvokeSchema, 0, len(p.Invokes))
		for _, i := range p.Invokes {
			out.Invokes = append(out.Invokes, sdk.InvokeSchema{
				Method:   i.Method,
				Summary:  i.Summary,
				Request:  payloadSchemaFromProto(i.Request),
				Response: payloadSchemaFromProto(i.Response),
				Metadata: i.Metadata,
			})
		}
	}
	return out
}
