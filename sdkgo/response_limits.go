package sdk

const (
	// Leave room below the 64 MiB gRPC ceiling for headers, usage and diagnostics.
	MaxBufferedResponseBytes = 32 << 20
	MaxResponseMessageBytes  = 48 << 20
)
