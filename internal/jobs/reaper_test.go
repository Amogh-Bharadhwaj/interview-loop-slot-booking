package jobs

import (
	"testing"
	"time"

	"test/internal/models"
	"test/internal/testutil"
)

func TestSweepExpiredHoldsDeletesOnlyExpiredInProgress(t *testing.T) {
	gdb := testutil.SetupDB(t)
	office := testutil.CreateOffice(t, gdb, "Blr HQ")
	room := testutil.CreateRoom(t, gdb, office.ID, "UTC")
	host := testutil.CreateUser(t, gdb, "Host", "host@example.com")

	expiredStart := time.Now().Add(time.Hour).UTC()
	expired := models.Slot{
		RoomID: room.RoomID, StartTime: expiredStart, EndTime: expiredStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateInProgress, HostUserID: &host.ID,
		HoldExpiresAt: timePtr(time.Now().Add(-time.Minute)),
	}
	if err := gdb.Create(&expired).Error; err != nil {
		t.Fatalf("create expired slot: %v", err)
	}
	if err := gdb.Create(&models.SlotParticipant{SlotID: expired.SlotID, UserID: host.ID}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	activeStart := time.Now().Add(3 * time.Hour).UTC()
	active := models.Slot{
		RoomID: room.RoomID, StartTime: activeStart, EndTime: activeStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateInProgress, HostUserID: &host.ID,
		HoldExpiresAt: timePtr(time.Now().Add(time.Hour)),
	}
	if err := gdb.Create(&active).Error; err != nil {
		t.Fatalf("create active slot: %v", err)
	}

	sweepExpiredHolds(gdb)

	var expiredCount int64
	gdb.Model(&models.Slot{}).Where("slot_id = ?", expired.SlotID).Count(&expiredCount)
	if expiredCount != 0 {
		t.Errorf("expected expired hold to be deleted, still found %d row(s)", expiredCount)
	}
	var participantCount int64
	gdb.Model(&models.SlotParticipant{}).Where("slot_id = ?", expired.SlotID).Count(&participantCount)
	if participantCount != 0 {
		t.Errorf("expected participants of the expired hold to cascade-delete, found %d", participantCount)
	}

	var activeCount int64
	gdb.Model(&models.Slot{}).Where("slot_id = ?", active.SlotID).Count(&activeCount)
	if activeCount != 1 {
		t.Errorf("expected non-expired hold to remain, found %d row(s)", activeCount)
	}
}

func TestSweepCompletedBookingsFlipsOnlyPastBookedRows(t *testing.T) {
	gdb := testutil.SetupDB(t)
	office := testutil.CreateOffice(t, gdb, "Blr HQ")
	room := testutil.CreateRoom(t, gdb, office.ID, "UTC")
	host := testutil.CreateUser(t, gdb, "Host", "host@example.com")

	pastStart := time.Now().Add(-2 * time.Hour).UTC()
	past := models.Slot{
		RoomID: room.RoomID, StartTime: pastStart, EndTime: pastStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateBooked, HostUserID: &host.ID,
	}
	if err := gdb.Create(&past).Error; err != nil {
		t.Fatalf("create past booked slot: %v", err)
	}

	futureStart := time.Now().Add(2 * time.Hour).UTC()
	future := models.Slot{
		RoomID: room.RoomID, StartTime: futureStart, EndTime: futureStart.Add(30 * time.Minute), Duration: 1800,
		BookingState: models.StateBooked, HostUserID: &host.ID,
	}
	if err := gdb.Create(&future).Error; err != nil {
		t.Fatalf("create future booked slot: %v", err)
	}

	sweepCompletedBookings(gdb)

	var pastState string
	gdb.Model(&models.Slot{}).Select("booking_state").Where("slot_id = ?", past.SlotID).Scan(&pastState)
	if pastState != string(models.StateCompleted) {
		t.Errorf("expected past booking to be Completed, got %s", pastState)
	}

	var futureState string
	gdb.Model(&models.Slot{}).Select("booking_state").Where("slot_id = ?", future.SlotID).Scan(&futureState)
	if futureState != string(models.StateBooked) {
		t.Errorf("expected future booking to remain Booked, got %s", futureState)
	}
}

func timePtr(t time.Time) *time.Time { return &t }
