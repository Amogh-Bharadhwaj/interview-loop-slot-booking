package jobs

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// RunReaper periodically releases abandoned checkouts and marks finished
// bookings complete. Freeing a slot means deleting its row outright — there
// is no "Available" resting state to reset it to.
func RunReaper(ctx context.Context, gdb *gorm.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepExpiredHolds(gdb)
			sweepCompletedBookings(gdb)
		}
	}
}

func sweepExpiredHolds(gdb *gorm.DB) {
	res := gdb.Exec(`DELETE FROM slots WHERE booking_state = 'InProgress' AND hold_expires_at < now()`)
	if res.Error != nil {
		log.Printf("reaper: expired-hold sweep failed: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("reaper: released %d expired hold(s)", res.RowsAffected)
	}
}

func sweepCompletedBookings(gdb *gorm.DB) {
	res := gdb.Exec(`UPDATE slots SET booking_state = 'Completed'
		WHERE booking_state = 'Booked' AND end_time < now()`)
	if res.Error != nil {
		log.Printf("reaper: completion sweep failed: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("reaper: marked %d booking(s) completed", res.RowsAffected)
	}
}
