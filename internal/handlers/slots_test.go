package handlers

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestViewSlotsEmptyRoomIsOneFreeWindow(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, _, _ := setupRoomAndUsers(t, gdb)

	date := time.Now().UTC().Format("2006-01-02")
	resp, err := http.Get(fmt.Sprintf("%s/rooms/%d/slots?date=%s", srv.URL, roomID, date))
	if err != nil {
		t.Fatalf("GET slots: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Timeline []map[string]any `json:"timeline"`
	}
	decodeJSON(t, resp, &body)
	if len(body.Timeline) != 1 || body.Timeline[0]["type"] != "free" {
		t.Errorf("expected a single free entry for an empty room, got %+v", body.Timeline)
	}
}

func TestViewSlotsShowsBookingCreatedTodayViaCachePath(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := created["slot_id"].(float64)

	date := start.Format("2006-01-02")
	resp, err := http.Get(fmt.Sprintf("%s/rooms/%d/slots?date=%s", srv.URL, roomID, date))
	if err != nil {
		t.Fatalf("GET slots: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Timeline []map[string]any `json:"timeline"`
	}
	decodeJSON(t, resp, &body)

	found := false
	for _, entry := range body.Timeline {
		if entry["type"] == "booking" && entry["slot_id"] == slotID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected freshly created booking to appear in ViewSlots (cache path), got %+v", body.Timeline)
	}
}

func TestViewSlotsDirectPathBeyondCacheWindow(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	// 10 days out is well beyond the 3-day cache window, exercising the
	// direct-query path instead of the MeetingRoom.Bookings cache.
	start := time.Now().Add(10 * 24 * time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := created["slot_id"].(float64)

	date := start.Format("2006-01-02")
	resp, err := http.Get(fmt.Sprintf("%s/rooms/%d/slots?date=%s", srv.URL, roomID, date))
	if err != nil {
		t.Fatalf("GET slots: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Timeline []map[string]any `json:"timeline"`
	}
	decodeJSON(t, resp, &body)

	found := false
	for _, entry := range body.Timeline {
		if entry["type"] == "booking" && entry["slot_id"] == slotID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected far-future booking to appear via the direct query path, got %+v", body.Timeline)
	}
}

func TestViewSlotsUnknownRoom(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	resp, err := http.Get(srv.URL + "/rooms/999999/slots?date=2026-01-01")
	if err != nil {
		t.Fatalf("GET slots: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestViewSlotsMissingDate(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, _, _ := setupRoomAndUsers(t, gdb)

	resp, err := http.Get(fmt.Sprintf("%s/rooms/%d/slots", srv.URL, roomID))
	if err != nil {
		t.Fatalf("GET slots: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing date, got %d", resp.StatusCode)
	}
}

func TestGetAvailableSlotsExcludesBookedWindowAndRespectsFromTo(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	// Book 09:00-09:30 UTC tomorrow.
	tomorrow := time.Now().Add(24 * time.Hour).UTC()
	bookedStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.UTC)
	createResp, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": bookedStart.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}

	date := bookedStart.Format("2006-01-02")
	resp, err := http.Get(fmt.Sprintf("%s/rooms/%d/slots/available?date=%s&from=08:00&to=10:00", srv.URL, roomID, date))
	if err != nil {
		t.Fatalf("GET available: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		FreeWindows []struct {
			StartTime time.Time `json:"start_time"`
			EndTime   time.Time `json:"end_time"`
		} `json:"free_windows"`
	}
	decodeJSON(t, resp, &body)

	if len(body.FreeWindows) != 2 {
		t.Fatalf("expected 2 free windows (before and after the booking) clipped to 08:00-10:00, got %d: %+v", len(body.FreeWindows), body.FreeWindows)
	}
	if !body.FreeWindows[0].EndTime.Equal(bookedStart) {
		t.Errorf("expected first free window to end at the booking start (%v), got %v", bookedStart, body.FreeWindows[0].EndTime)
	}
	if !body.FreeWindows[1].StartTime.Equal(bookedStart.Add(30 * time.Minute)) {
		t.Errorf("expected second free window to start at the booking end, got %v", body.FreeWindows[1].StartTime)
	}
}
