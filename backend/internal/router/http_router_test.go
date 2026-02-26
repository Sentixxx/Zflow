package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testRegistrar struct {
	registered bool
	wrapped    bool
}

func (r *testRegistrar) RegisterRoutes(mux *http.ServeMux) {
	r.registered = true
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})
}

func (r *testRegistrar) WrapHTTPHandler(next http.Handler) http.Handler {
	r.wrapped = true
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Router-Wrapped", "1")
		next.ServeHTTP(w, req)
	})
}

func TestNewHTTPHandlerBuildsMuxAndWraps(t *testing.T) {
	reg := &testRegistrar{}
	h := NewHTTPHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !reg.registered {
		t.Fatalf("RegisterRoutes not called")
	}
	if !reg.wrapped {
		t.Fatalf("WrapHTTPHandler not called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "pong" {
		t.Fatalf("body = %q, want %q", rr.Body.String(), "pong")
	}
	if rr.Header().Get("X-Router-Wrapped") != "1" {
		t.Fatalf("X-Router-Wrapped = %q, want 1", rr.Header().Get("X-Router-Wrapped"))
	}
}
