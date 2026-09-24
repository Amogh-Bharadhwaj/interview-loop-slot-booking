package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"test/internal/testutil"
)

func TestRoomCreateAndList(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	office := testutil.CreateOffice(t, gdb, "Blr HQ")

	body, _ := json.Marshal(map[string]any{"office_id": office.ID, "timezone": "Asia/Kolkata"})
	resp, err := http.Post(srv.URL+"/rooms", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /rooms: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	if created["timezone"] != "Asia/Kolkata" {
		t.Errorf("expected timezone Asia/Kolkata, got %v", created["timezone"])
	}

	listResp, err := http.Get(srv.URL + "/rooms")
	if err != nil {
		t.Fatalf("GET /rooms: %v", err)
	}
	defer listResp.Body.Close()
	var rooms []map[string]any
	json.NewDecoder(listResp.Body).Decode(&rooms)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}
}

func TestRoomCreateDefaultsTimezoneToUTC(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	office := testutil.CreateOffice(t, gdb, "Blr HQ")

	body, _ := json.Marshal(map[string]any{"office_id": office.ID})
	resp, err := http.Post(srv.URL+"/rooms", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /rooms: %v", err)
	}
	defer resp.Body.Close()
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	if created["timezone"] != "UTC" {
		t.Errorf("expected default timezone UTC, got %v", created["timezone"])
	}
}

func TestRoomCreateWithUnknownOfficeID(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]any{"office_id": 99999})
	resp, err := http.Post(srv.URL+"/rooms", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /rooms: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown office_id, got %d", resp.StatusCode)
	}
}

