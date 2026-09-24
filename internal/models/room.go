package models

import "time"

type MeetingRoom struct {
	RoomID    int64     `json:"room_id" gorm:"column:room_id;primaryKey;autoIncrement"`
	OfficeID  int64     `json:"office_id" gorm:"not null;index"`
	Timezone  string    `json:"timezone" gorm:"type:varchar(64);not null;default:'UTC'"`
	Bookings  Int64List `json:"-" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MeetingRoom) TableName() string { return "meeting_rooms" }
