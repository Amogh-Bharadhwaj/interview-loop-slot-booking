package jobs

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"test/internal/models"
)

// RunCacheRefresh keeps MeetingRoom.Bookings in sync with whatever Slot rows
// (bookings) currently exist in the near window. It caches slot-ID
// membership only, never booking state, so it never goes stale on state
// even though it only refreshes once a day.
func RunCacheRefresh(ctx context.Context, gdb *gorm.DB, cacheWindowDays int) {
	refreshBookingsCache(gdb, cacheWindowDays)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshBookingsCache(gdb, cacheWindowDays)
		}
	}
}

func refreshBookingsCache(gdb *gorm.DB, cacheWindowDays int) {
	var rooms []models.MeetingRoom
	if err := gdb.Find(&rooms).Error; err != nil {
		log.Printf("cache refresh: failed to list rooms: %v", err)
		return
	}
	for _, room := range rooms {
		loc, err := time.LoadLocation(room.Timezone)
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		windowEnd := windowStart.AddDate(0, 0, cacheWindowDays)

		var slotIDs []int64
		if err := gdb.Model(&models.Slot{}).
			Where("room_id = ? AND start_time >= ? AND start_time < ?", room.RoomID, windowStart, windowEnd).
			Pluck("slot_id", &slotIDs).Error; err != nil {
			log.Printf("cache refresh: room %d query failed: %v", room.RoomID, err)
			continue
		}
		if err := gdb.Model(&models.MeetingRoom{}).Where("room_id = ?", room.RoomID).
			Update("bookings", models.Int64List(slotIDs)).Error; err != nil {
			log.Printf("cache refresh: room %d update failed: %v", room.RoomID, err)
		}
	}
}
