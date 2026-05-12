// Package grpc 提供 AirGate Go 插件运行时的 gRPC 适配层。
//
// 本包负责 hashicorp/go-plugin 集成、protobuf 与 sdkgo 类型转换、
// Core 反向调用通道、流式响应桥接和插件进程启动。
package grpc
