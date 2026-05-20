package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPprofMuxRegistersIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()

	NewPprofMux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected pprof index 200, got %d", rec.Code)
	}
}

func TestNewPprofServerDisabledReturnsNil(t *testing.T) {
	server, err := NewPprofServer("api", false, "127.0.0.1:0")

	if err != nil {
		t.Fatalf("disabled pprof should not error: %v", err)
	}
	if server != nil {
		t.Fatalf("expected nil server when disabled")
	}
}
