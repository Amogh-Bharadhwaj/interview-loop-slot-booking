package handlers

import (
	"testing"
	"time"

	"test/internal/models"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func TestComputeTimelineNoBookings(t *testing.T) {
	start := mustTime(t, "2026-01-01T00:00:00Z")
	end := mustTime(t, "2026-01-02T00:00:00Z")

	got := computeTimeline(nil, start, end, nil)

	if len(got) != 1 {
		t.Fatalf("expected 1 free entry, got %d: %+v", len(got), got)
	}
	if got[0].Type != "free" || !got[0].StartTime.Equal(start) || !got[0].EndTime.Equal(end) {
		t.Errorf("unexpected free entry: %+v", got[0])
	}
}

func TestComputeTimelineSingleBookingWithGapsOnBothSides(t *testing.T) {
	windowStart := mustTime(t, "2026-01-01T00:00:00Z")
	windowEnd := mustTime(t, "2026-01-02T00:00:00Z")
	bStart := mustTime(t, "2026-01-01T09:00:00Z")

	host := int64(7)
	booking := models.Slot{
		SlotID:       1,
		StartTime:    bStart,
		EndTime:      bStart.Add(30 * time.Minute),
		Duration:     1800,
		BookingState: models.StateBooked,
		HostUserID:   &host,
	}

	got := computeTimeline([]models.Slot{booking}, windowStart, windowEnd, map[int64][]int64{1: {7, 9}})

	if len(got) != 3 {
		t.Fatalf("expected 3 entries (free, booking, free), got %d: %+v", len(got), got)
	}
	if got[0].Type != "free" || !got[0].StartTime.Equal(windowStart) || !got[0].EndTime.Equal(bStart) {
		t.Errorf("unexpected leading free entry: %+v", got[0])
	}
	if got[1].Type != "booking" || *got[1].SlotID != 1 || *got[1].BookingState != "Booked" {
		t.Errorf("unexpected booking entry: %+v", got[1])
	}
	if len(got[1].ParticipantIDs) != 2 {
		t.Errorf("expected 2 participants, got %v", got[1].ParticipantIDs)
	}
	if got[2].Type != "free" || !got[2].StartTime.Equal(bStart.Add(30*time.Minute)) || !got[2].EndTime.Equal(windowEnd) {
		t.Errorf("unexpected trailing free entry: %+v", got[2])
	}
}

func TestComputeTimelineBookingSpansEntireWindow(t *testing.T) {
	windowStart := mustTime(t, "2026-01-01T00:00:00Z")
	windowEnd := mustTime(t, "2026-01-02T00:00:00Z")

	booking := models.Slot{
		SlotID:    1,
		StartTime: windowStart,
		EndTime:   windowEnd,
		Duration:  86400,
	}

	got := computeTimeline([]models.Slot{booking}, windowStart, windowEnd, nil)

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 booking entry with no free gaps, got %d: %+v", len(got), got)
	}
	if got[0].Type != "booking" {
		t.Errorf("expected booking entry, got %+v", got[0])
	}
}

func TestComputeTimelineBackToBackBookingsNoGapBetween(t *testing.T) {
	windowStart := mustTime(t, "2026-01-01T00:00:00Z")
	windowEnd := mustTime(t, "2026-01-02T00:00:00Z")
	firstStart := mustTime(t, "2026-01-01T09:00:00Z")
	firstEnd := firstStart.Add(30 * time.Minute)

	bookings := []models.Slot{
		{SlotID: 2, StartTime: firstEnd, EndTime: firstEnd.Add(30 * time.Minute), Duration: 1800},
		{SlotID: 1, StartTime: firstStart, EndTime: firstEnd, Duration: 1800},
	}

	got := computeTimeline(bookings, windowStart, windowEnd, nil)

	// Bookings are sorted by StartTime, and no free gap should appear between
	// two back-to-back bookings — only leading and trailing free entries.
	if len(got) != 4 {
		t.Fatalf("expected 4 entries (free, booking, booking, free), got %d: %+v", len(got), got)
	}
	if got[1].Type != "booking" || *got[1].SlotID != 1 {
		t.Errorf("expected first booking to be slot 1, got %+v", got[1])
	}
	if got[2].Type != "booking" || *got[2].SlotID != 2 {
		t.Errorf("expected second booking to be slot 2, got %+v", got[2])
	}
}

func TestComputeTimelineOverlappingBookingsDoNotDoubleCountCursor(t *testing.T) {
	// Defensive case: even though the DB prevents overlapping InProgress/Booked
	// rows, computeTimeline itself should not regress the cursor if it ever
	// sees out-of-order or overlapping input (e.g. a Completed row).
	windowStart := mustTime(t, "2026-01-01T00:00:00Z")
	windowEnd := mustTime(t, "2026-01-02T00:00:00Z")
	longStart := mustTime(t, "2026-01-01T09:00:00Z")
	longEnd := longStart.Add(2 * time.Hour)
	shortStart := longStart.Add(10 * time.Minute)
	shortEnd := shortStart.Add(10 * time.Minute)

	bookings := []models.Slot{
		{SlotID: 1, StartTime: longStart, EndTime: longEnd, Duration: 7200},
		{SlotID: 2, StartTime: shortStart, EndTime: shortEnd, Duration: 600},
	}

	got := computeTimeline(bookings, windowStart, windowEnd, nil)

	last := got[len(got)-1]
	if last.Type != "free" || !last.StartTime.Equal(longEnd) {
		t.Errorf("expected trailing free entry to start at the longer booking's end (%v), got %+v", longEnd, last)
	}
}
