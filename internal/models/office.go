package models

import "time"

type Office struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Location  string `json:"location" gorm:"type:varchar(255);not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
