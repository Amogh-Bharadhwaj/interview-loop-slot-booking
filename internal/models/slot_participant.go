package models

import "time"

type SlotParticipant struct {
	SlotID    int64     `json:"slot_id" gorm:"column:slot_id;primaryKey;autoIncrement:false"`
	UserID    int64     `json:"user_id" gorm:"column:user_id;primaryKey;autoIncrement:false"`
	CreatedAt time.Time `json:"created_at"`
}

func (SlotParticipant) TableName() string { return "slot_participants" }
