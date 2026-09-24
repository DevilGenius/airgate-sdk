package sdk

import "net/http"

// BeginStreamCompletion marks the final protocol event before it is written.
// The gRPC writer retains a bounded tail and sends it with ForwardOutcome, so
// Core receives usage before the downstream can observe completion and cancel.
// Call only after upstream completion/error is known; return from Forward soon
// afterwards. Ordinary HTTP writers keep their existing behavior.
func BeginStreamCompletion(w http.ResponseWriter) {
	// Bound traversal instead of comparing interface values: a writer may be a
	// non-comparable value type. This also handles multi-wrapper cycles.
	for depth := 0; w != nil && depth < 32; depth++ {
		switch writer := w.(type) {
		case interface{ BeginStreamCompletion() }:
			writer.BeginStreamCompletion()
			return
		case interface{ Unwrap() http.ResponseWriter }:
			w = writer.Unwrap()
		default:
			return
		}
	}
}
