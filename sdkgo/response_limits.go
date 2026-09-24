package sdk

const (
	// Body bytes include JSON/base64 encoding and are measured after HTTP decoding.
	MaxBufferedResponseBytes = 96 << 20
	// Leave room for headers, usage and diagnostics, below the 128 MiB gRPC ceiling.
	MaxResponseMessageBytes = 112 << 20
)
