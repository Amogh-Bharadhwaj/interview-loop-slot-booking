package db

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"test/internal/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return gdb, nil
}

func Migrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(
		&models.Office{},
		&models.MeetingRoom{},
		&models.User{},
		&models.Slot{},
		&models.SlotParticipant{},
	); err != nil {
		return err
	}

	statements := []string{
		`ALTER TABLE slots DROP CONSTRAINT IF EXISTS chk_booking_state`,
		`ALTER TABLE slots ADD CONSTRAINT chk_booking_state
			CHECK (booking_state IN ('InProgress','Booked','Completed'))`,
		`CREATE EXTENSION IF NOT EXISTS btree_gist`,
		`ALTER TABLE slots DROP CONSTRAINT IF EXISTS slots_no_overlap`,
		`ALTER TABLE slots ADD CONSTRAINT slots_no_overlap EXCLUDE USING gist (
			room_id WITH =,
			tstzrange(start_time, end_time) WITH &&
		) WHERE (booking_state IN ('InProgress', 'Booked'))`,
		// Referential integrity by ID only — no Go-level struct associations,
		// but the DB still enforces valid FKs. Cascade on slot deletion so
		// freeing a slot (DeleteBooking / reaper) automatically clears its participants.
		`ALTER TABLE meeting_rooms DROP CONSTRAINT IF EXISTS fk_rooms_office`,
		`ALTER TABLE meeting_rooms ADD CONSTRAINT fk_rooms_office
			FOREIGN KEY (office_id) REFERENCES offices(id)`,
		`ALTER TABLE slots DROP CONSTRAINT IF EXISTS fk_slots_room`,
		`ALTER TABLE slots ADD CONSTRAINT fk_slots_room
			FOREIGN KEY (room_id) REFERENCES meeting_rooms(room_id)`,
		`ALTER TABLE slots DROP CONSTRAINT IF EXISTS fk_slots_host`,
		`ALTER TABLE slots ADD CONSTRAINT fk_slots_host
			FOREIGN KEY (host_user_id) REFERENCES users(id)`,
		`ALTER TABLE slot_participants DROP CONSTRAINT IF EXISTS fk_sp_slot`,
		`ALTER TABLE slot_participants ADD CONSTRAINT fk_sp_slot
			FOREIGN KEY (slot_id) REFERENCES slots(slot_id) ON DELETE CASCADE`,
		`ALTER TABLE slot_participants DROP CONSTRAINT IF EXISTS fk_sp_user`,
		`ALTER TABLE slot_participants ADD CONSTRAINT fk_sp_user
			FOREIGN KEY (user_id) REFERENCES users(id)`,
	}
	for _, stmt := range statements {
		if err := gdb.Exec(stmt).Error; err != nil {
			return err
		}
	}
	log.Println("migrations applied")
	return nil
}
