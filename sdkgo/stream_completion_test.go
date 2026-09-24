package sdk

import (
	"net/http"
	"testing"
)

type completionTestWriter struct {
	next  http.ResponseWriter
	calls int
}

func (*completionTestWriter) Header() http.Header           { return http.Header{} }
func (*completionTestWriter) WriteHeader(int)               {}
func (*completionTestWriter) Write(p []byte) (int, error)   { return len(p), nil }
func (w *completionTestWriter) Unwrap() http.ResponseWriter { w.calls++; return w.next }

type nonComparableCompletionWriter []int

func (nonComparableCompletionWriter) Header() http.Header           { return http.Header{} }
func (nonComparableCompletionWriter) WriteHeader(int)               {}
func (nonComparableCompletionWriter) Write(p []byte) (int, error)   { return len(p), nil }
func (w nonComparableCompletionWriter) Unwrap() http.ResponseWriter { return w }

func TestCompletionUnwrapCyclesTerminate(t *testing.T) {
	self := &completionTestWriter{}
	self.next = self
	BeginStreamCompletion(self)
	if self.calls == 0 {
		t.Fatal("self cycle was not exercised")
	}
	a, b := &completionTestWriter{}, &completionTestWriter{}
	a.next, b.next = b, a
	BeginStreamCompletion(a)
	if a.calls == 0 || b.calls == 0 {
		t.Fatal("two-wrapper cycle was not exercised")
	}
	// An interface equality check would panic for this legal writer type.
	BeginStreamCompletion(nonComparableCompletionWriter{1})
	BeginStreamCompletion(nil)
}
