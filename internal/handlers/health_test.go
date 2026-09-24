package handlers

import (
	"net/http"
	"testing"
	"time"
)

func TestHealthCheckOK(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
