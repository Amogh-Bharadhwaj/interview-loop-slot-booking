package jobs

import (
	"testing"
	"time"

	"test/internal/models"
	"test/internal/testutil"
)

func TestRefreshBookingsCachePopulatesOnlyWithinWindow(t *testing.T) {
	gdb := testutil.SetupDB(t)
	office := testutil.CreateOffice(t, gdb, "Blr HQ")
	room := testutil.CreateRoom(t, gdb, office.ID, "UTC")
	host := testutil.CreateUser(t, gdb, "Host", "host@example.com")

	inWindowStart := time.Now().Add(time.Hour).UTC()
	inWindow := models.Slot{
		RoomID: room.RoomID, StartTime: inWindowStart, EndTime: inWindowStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateBooked, HostUserID: &host.ID,
	}
	if err := gdb.Create(&inWindow).Error; err != nil {
		t.Fatalf("create in-window slot: %v", err)
	}

	outOfWindowStart := time.Now().Add(10 * 24 * time.Hour).UTC()
	outOfWindow := models.Slot{
		RoomID: room.RoomID, StartTime: outOfWindowStart, EndTime: outOfWindowStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateBooked, HostUserID: &host.ID,
	}
	if err := gdb.Create(&outOfWindow).Error; err != nil {
		t.Fatalf("create out-of-window slot: %v", err)
	}

	refreshBookingsCache(gdb, 3)

	var refreshed models.MeetingRoom
	if err := gdb.First(&refreshed, room.RoomID).Error; err != nil {
		t.Fatalf("reload room: %v", err)
	}

	found := map[int64]bool{}
	for _, id := range refreshed.Bookings {
		found[id] = true
	}
	if !found[inWindow.SlotID] {
		t.Errorf("expected in-window slot %d to be cached, got %v", inWindow.SlotID, refreshed.Bookings)
	}
	if found[outOfWindow.SlotID] {
		t.Errorf("expected out-of-window slot %d NOT to be cached, got %v", outOfWindow.SlotID, refreshed.Bookings)
	}
}
