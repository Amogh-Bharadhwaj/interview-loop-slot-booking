package testutil

import (
	"testing"

	"gorm.io/gorm"

	"test/internal/models"
)

func CreateOffice(t *testing.T, gdb *gorm.DB, location string) models.Office {
	t.Helper()
	office := models.Office{Location: location}
	if err := gdb.Create(&office).Error; err != nil {
		t.Fatalf("create fixture office: %v", err)
	}
	return office
}

func CreateRoom(t *testing.T, gdb *gorm.DB, officeID int64, timezone string) models.MeetingRoom {
	t.Helper()
	if timezone == "" {
		timezone = "UTC"
	}
	room := models.MeetingRoom{OfficeID: officeID, Timezone: timezone, Bookings: models.Int64List{}}
	if err := gdb.Create(&room).Error; err != nil {
		t.Fatalf("create fixture room: %v", err)
	}
	return room
}

func CreateUser(t *testing.T, gdb *gorm.DB, name, email string) models.User {
	t.Helper()
	user := models.User{Name: name, Email: email}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create fixture user: %v", err)
	}
	return user
}
