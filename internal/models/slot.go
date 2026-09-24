package models

import "time"

// Slot represents a booking. Rows are only created when a booking is
// started — there is no pre-generated grid and no "Available" state.
// A room/time is free simply because no Slot row covers it.
type Slot struct {
	SlotID        int64        `json:"slot_id" gorm:"column:slot_id;primaryKey;autoIncrement"`
	RoomID        int64        `json:"room_id" gorm:"not null;index"`
	BookingState  BookingState `json:"booking_state" gorm:"type:varchar(20);not null;index"`
	StartTime     time.Time    `json:"start_time" gorm:"not null;index"`
	Duration      int64        `json:"duration_seconds" gorm:"column:duration;not null"`
	// EndTime is persisted (rather than computed inline) because the exclusion
	// constraint's index expression must be IMMUTABLE, and `start_time + interval`
	// is only STABLE in Postgres — a plain column reference satisfies that.
	EndTime       time.Time    `json:"-" gorm:"not null;index"`
	HostUserID    *int64       `json:"host_user_id" gorm:"index"`
	HoldExpiresAt *time.Time   `json:"hold_expires_at,omitempty" gorm:"index"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

func (Slot) TableName() string { return "slots" }
