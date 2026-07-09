package rpc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkJSONRPCMethodNotFoundDispatch(b *testing.B) {
	s := &Server{}
	body := []byte(`{"jsonrpc":"1.0","id":"bench","method":"doesnotexist","params":[]}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		s.handle(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", rec.Code)
		}
	}
}

func BenchmarkJSONRPCHelpDispatch(b *testing.B) {
	s := &Server{}
	body := []byte(`{"jsonrpc":"1.0","id":"bench","method":"help","params":[]}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		s.handle(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", rec.Code)
		}
	}
}
