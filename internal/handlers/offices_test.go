package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestOfficeCreateAndList(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]string{"location": "Blr HQ"})
	resp, err := http.Post(srv.URL+"/offices", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /offices: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created["location"] != "Blr HQ" {
		t.Errorf("expected location Blr HQ, got %v", created["location"])
	}

	listResp, err := http.Get(srv.URL + "/offices")
	if err != nil {
		t.Fatalf("GET /offices: %v", err)
	}
	defer listResp.Body.Close()
	var offices []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&offices); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(offices) != 1 {
		t.Fatalf("expected 1 office, got %d", len(offices))
	}
}

func TestOfficeCreateMissingLocation(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]string{})
	resp, err := http.Post(srv.URL+"/offices", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /offices: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing location, got %d", resp.StatusCode)
	}
}
