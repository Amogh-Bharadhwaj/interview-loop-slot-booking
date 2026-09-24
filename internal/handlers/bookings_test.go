package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"test/internal/models"
	"test/internal/testutil"
)

func doJSON(t *testing.T, method, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	var parsed map[string]any
	if resp.ContentLength != 0 {
		json.NewDecoder(resp.Body).Decode(&parsed)
	}
	return resp, parsed
}

func setupRoomAndUsers(t *testing.T, gdb *gorm.DB) (roomID, hostID, participantID int64) {
	office := testutil.CreateOffice(t, gdb, "Blr HQ")
	room := testutil.CreateRoom(t, gdb, office.ID, "UTC")
	host := testutil.CreateUser(t, gdb, "Host", fmt.Sprintf("host-%d@example.com", time.Now().UnixNano()))
	participant := testutil.CreateUser(t, gdb, "Participant", fmt.Sprintf("participant-%d@example.com", time.Now().UnixNano()))
	return room.RoomID, host.ID, participant.ID
}

func TestCreateBookingSuccess(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, participantID := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	resp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id":              roomID,
		"start_time":           start.Format(time.RFC3339),
		"duration_seconds":     1800,
		"host_user_id":         hostID,
		"participant_user_ids": []int64{participantID},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, created)
	}
	if created["booking_state"] != "InProgress" {
		t.Errorf("expected InProgress, got %v", created["booking_state"])
	}
	if created["hold_expires_at"] == nil {
		t.Error("expected hold_expires_at to be set")
	}
}

func TestCreateBookingMissingFields(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, _, _ := setupRoomAndUsers(t, gdb)

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{"room_id": roomID})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestCreateBookingUnknownRoom(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	_, hostID, _ := setupRoomAndUsers(t, gdb)

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id":          999999,
		"start_time":       time.Now().Add(time.Hour).Format(time.RFC3339),
		"duration_seconds": 1800,
		"host_user_id":     hostID,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown room, got %d", resp.StatusCode)
	}
}

func TestCreateBookingUnknownHostRejectedByForeignKey(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, _, _ := setupRoomAndUsers(t, gdb)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id":          roomID,
		"start_time":       time.Now().Add(time.Hour).Format(time.RFC3339),
		"duration_seconds": 1800,
		"host_user_id":     999999,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown host_user_id, got %d: %v", resp.StatusCode, body)
	}
}

func TestCreateBookingOverlapConflict(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	first, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("expected first booking to succeed with 201, got %d", first.StatusCode)
	}

	overlapStart := start.Add(10 * time.Minute)
	second, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": overlapStart.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if second.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for overlapping booking, got %d", second.StatusCode)
	}
}

// TestCreateBookingConcurrentOverlapOnlyOneWins fires two requests for the
// exact same room/time range at once and asserts the DB exclusion constraint
// lets exactly one through — the core "hold" correctness guarantee. Run with
// `go test ./internal/handlers/... -run Concurrent -v` to see the full
// blow-by-blow: dispatch/response timestamps, per-goroutine status and body,
// and the resulting DB row.
func TestCreateBookingConcurrentOverlapOnlyOneWins(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, host1, host2 := setupRoomAndUsers(t, gdb)
	hostIDs := []int64{host1, host2}

	start := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)
	t.Logf("target window: room_id=%d start=%s duration=30m (identical for both requests)", roomID, start.Format(time.RFC3339))

	type result struct {
		goroutine  int
		hostID     int64
		dispatchAt time.Time
		respondAt  time.Time
		status     int
		body       map[string]any
	}

	var wg sync.WaitGroup
	results := make([]result, 2)
	var startGate sync.WaitGroup
	startGate.Add(1)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := map[string]any{
				"room_id": roomID, "start_time": start.Format(time.RFC3339),
				"duration_seconds": 1800, "host_user_id": hostIDs[idx],
			}
			startGate.Wait() // release both goroutines at the same instant
			dispatchAt := time.Now()
			resp, body := doJSON(t, http.MethodPost, srv.URL+"/bookings", payload)
			results[idx] = result{
				goroutine: idx, hostID: hostIDs[idx],
				dispatchAt: dispatchAt, respondAt: time.Now(),
				status: resp.StatusCode, body: body,
			}
		}(i)
	}
	raceStart := time.Now()
	startGate.Done()
	wg.Wait()
	t.Logf("both requests dispatched and completed in %v", time.Since(raceStart))

	created, conflicted := 0, 0
	for _, r := range results {
		t.Logf("goroutine %d (host_user_id=%d): dispatched at %s, responded at %s (latency %v) -> HTTP %d, body=%v",
			r.goroutine, r.hostID, r.dispatchAt.Format(time.RFC3339Nano), r.respondAt.Format(time.RFC3339Nano),
			r.respondAt.Sub(r.dispatchAt), r.status, r.body)
		switch r.status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicted++
		}
	}

	var rows []models.Slot
	gdb.Where("room_id = ?", roomID).Find(&rows)
	t.Logf("final DB state: %d slot row(s) for room_id=%d", len(rows), roomID)
	for _, row := range rows {
		t.Logf("  slot_id=%d booking_state=%s start_time=%s host_user_id=%v",
			row.SlotID, row.BookingState, row.StartTime.Format(time.RFC3339), row.HostUserID)
	}

	if created != 1 || conflicted != 1 {
		t.Errorf("expected exactly one 201 and one 409, got statuses %v", []int{results[0].status, results[1].status})
	}
	if len(rows) != 1 {
		t.Errorf("expected exactly one slot row to exist after the race, found %d", len(rows))
	}
}

func TestUpdateBookingConfirm(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, participantID := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := int64(created["slot_id"].(float64))

	resp, body := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/bookings/%d", srv.URL, slotID), map[string]any{
		"action": "confirm", "host_user_id": hostID, "participant_user_ids": []int64{participantID},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["booking_state"] != "Booked" {
		t.Errorf("expected Booked, got %v", body["booking_state"])
	}
	if body["hold_expires_at"] != nil {
		t.Errorf("expected hold_expires_at cleared, got %v", body["hold_expires_at"])
	}
}

func TestUpdateBookingConfirmWrongHostForbidden(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, otherUserID := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := int64(created["slot_id"].(float64))

	resp, _ := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/bookings/%d", srv.URL, slotID), map[string]any{
		"action": "confirm", "host_user_id": otherUserID,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched host, got %d", resp.StatusCode)
	}
}

func TestUpdateBookingConfirmAfterHoldExpiredReturns410(t *testing.T) {
	// Use a very short hold window so the hold expires almost immediately,
	// without needing to wait for the background reaper to sweep it.
	srv, gdb := newTestServer(t, 50*time.Millisecond, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := int64(created["slot_id"].(float64))

	time.Sleep(100 * time.Millisecond) // let the hold expire (reaper is not running in this test)

	resp, body := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/bookings/%d", srv.URL, slotID), map[string]any{
		"action": "confirm", "host_user_id": hostID,
	})
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 for expired hold, got %d: %v", resp.StatusCode, body)
	}
}

func TestUpdateBookingUpdateParticipantsReplacesSet(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, participant1 := setupRoomAndUsers(t, gdb)
	participant2 := testutil.CreateUser(t, gdb, "P2", fmt.Sprintf("p2-%d@example.com", time.Now().UnixNano())).ID

	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800,
		"host_user_id": hostID, "participant_user_ids": []int64{participant1},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := int64(created["slot_id"].(float64))

	resp, body := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/bookings/%d", srv.URL, slotID), map[string]any{
		"action": "update_participants", "host_user_id": hostID, "participant_user_ids": []int64{participant2},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	ids, ok := body["participant_ids"].([]any)
	if !ok || len(ids) != 1 || int64(ids[0].(float64)) != participant2 {
		t.Errorf("expected participant set replaced with [%d], got %v", participant2, body["participant_ids"])
	}
}

func TestUpdateBookingNotFound(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	resp, _ := doJSON(t, http.MethodPatch, srv.URL+"/bookings/999999", map[string]any{
		"action": "confirm", "host_user_id": 1,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteBookingFreesTheSlotAndCascadesParticipants(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, participantID := setupRoomAndUsers(t, gdb)

	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	createResp, created := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800,
		"host_user_id": hostID, "participant_user_ids": []int64{participantID},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup booking failed: %d", createResp.StatusCode)
	}
	slotID := int64(created["slot_id"].(float64))

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/bookings/%d", srv.URL, slotID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	var count int64
	gdb.Model(&models.Slot{}).Where("slot_id = ?", slotID).Count(&count)
	if count != 0 {
		t.Errorf("expected slot row to be deleted, still found %d row(s)", count)
	}
	var participantCount int64
	gdb.Model(&models.SlotParticipant{}).Where("slot_id = ?", slotID).Count(&participantCount)
	if participantCount != 0 {
		t.Errorf("expected participant rows to cascade-delete, found %d", participantCount)
	}

	// Deleting the room's only booking should free the room back up so a new
	// overlapping booking can now succeed.
	again, _ := doJSON(t, http.MethodPost, srv.URL+"/bookings", map[string]any{
		"room_id": roomID, "start_time": start.Format(time.RFC3339), "duration_seconds": 1800, "host_user_id": hostID,
	})
	if again.StatusCode != http.StatusCreated {
		t.Errorf("expected room to be bookable again after delete, got %d", again.StatusCode)
	}
}

func TestDeleteBookingNotFound(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/bookings/999999", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteBookingCompletedIsRejected(t *testing.T) {
	srv, gdb := newTestServer(t, 90*time.Second, 3)
	roomID, hostID, _ := setupRoomAndUsers(t, gdb)

	// Fabricate an already-Completed booking directly, since reaching that
	// state naturally requires the reaper sweep to run after the meeting ends.
	past := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	slot := models.Slot{
		RoomID: roomID, StartTime: past, EndTime: past.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateCompleted, HostUserID: &hostID,
	}
	if err := gdb.Create(&slot).Error; err != nil {
		t.Fatalf("fabricate completed slot: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/bookings/%d", srv.URL, slot.SlotID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 deleting a completed booking, got %d", resp.StatusCode)
	}
}
