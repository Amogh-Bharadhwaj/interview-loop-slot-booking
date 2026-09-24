package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"test/internal/models"
)

type BookingHandler struct {
	DB              *gorm.DB
	HoldWindow      time.Duration
	CacheWindowDays int
}

type createBookingRequest struct {
	RoomID             int64     `json:"room_id"`
	StartTime          time.Time `json:"start_time"`
	DurationSeconds    int64     `json:"duration_seconds"`
	HostUserID         int64     `json:"host_user_id"`
	ParticipantUserIDs []int64   `json:"participant_user_ids"`
}

// respondPgError maps the exclusion-constraint / FK-violation errors that
// can surface from a booking write to sensible HTTP statuses.
func respondPgError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23P01": // exclusion_violation - overlapping InProgress/Booked slot
			c.JSON(http.StatusConflict, gin.H{"error": "room not available for that time range"})
			return
		case "23503": // foreign_key_violation
			c.JSON(http.StatusBadRequest, gin.H{"error": "a referenced room or user does not exist"})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func replaceParticipants(tx *gorm.DB, slotID int64, ids []int64) error {
	if err := tx.Where("slot_id = ?", slotID).Delete(&models.SlotParticipant{}).Error; err != nil {
		return err
	}
	for _, uid := range ids {
		if err := tx.Create(&models.SlotParticipant{SlotID: slotID, UserID: uid}).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncRoomCacheOnCreate keeps MeetingRoom.Bookings from going stale for a
// booking made after the last daily cache refresh: without this, a brand-new
// same-day booking would be invisible to ViewSlots's cache path until the
// next cron tick, even though it's already blocking the room. The daily job
// remains the source of truth; this just avoids the same-day gap.
func (h *BookingHandler) syncRoomCacheOnCreate(slot models.Slot) {
	var room models.MeetingRoom
	if err := h.DB.First(&room, slot.RoomID).Error; err != nil {
		return
	}
	loc, err := time.LoadLocation(room.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	windowEnd := windowStart.AddDate(0, 0, h.CacheWindowDays)
	if slot.StartTime.Before(windowStart) || !slot.StartTime.Before(windowEnd) {
		return
	}
	for _, id := range room.Bookings {
		if id == slot.SlotID {
			return
		}
	}
	updated := append(append(models.Int64List{}, room.Bookings...), slot.SlotID)
	h.DB.Model(&models.MeetingRoom{}).Where("room_id = ?", slot.RoomID).Update("bookings", updated)
}

func (h *BookingHandler) fetchParticipantIDs(slotID int64) []int64 {
	var sps []models.SlotParticipant
	h.DB.Where("slot_id = ?", slotID).Find(&sps)
	ids := make([]int64, len(sps))
	for i, sp := range sps {
		ids[i] = sp.UserID
	}
	return ids
}

// CreateBooking inserts a brand-new Slot row for the requested room/time —
// there is nothing to "claim", since slots only exist once booked. Overlap
// safety comes from the DB's exclusion constraint, not app-level locking.
func (h *BookingHandler) CreateBooking(c *gin.Context) {
	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RoomID == 0 || req.HostUserID == 0 || req.StartTime.IsZero() || req.DurationSeconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id, start_time, duration_seconds (>0) and host_user_id are required"})
		return
	}

	var room models.MeetingRoom
	if err := h.DB.First(&room, req.RoomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	holdExpiry := time.Now().Add(h.HoldWindow)
	slot := models.Slot{
		RoomID:        req.RoomID,
		StartTime:     req.StartTime,
		EndTime:       req.StartTime.Add(time.Duration(req.DurationSeconds) * time.Second),
		Duration:      req.DurationSeconds,
		BookingState:  models.StateInProgress,
		HostUserID:    &req.HostUserID,
		HoldExpiresAt: &holdExpiry,
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&slot).Error; err != nil {
			return err
		}
		for _, uid := range req.ParticipantUserIDs {
			if err := tx.Create(&models.SlotParticipant{SlotID: slot.SlotID, UserID: uid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		respondPgError(c, err)
		return
	}
	h.syncRoomCacheOnCreate(slot)

	c.JSON(http.StatusCreated, gin.H{
		"slot_id":          slot.SlotID,
		"room_id":          slot.RoomID,
		"booking_state":    slot.BookingState,
		"start_time":       slot.StartTime,
		"duration_seconds": slot.Duration,
		"host_user_id":     *slot.HostUserID,
		"participant_ids":  req.ParticipantUserIDs,
		"hold_expires_at":  slot.HoldExpiresAt,
	})
}

type updateBookingRequest struct {
	Action             string  `json:"action"`
	HostUserID         int64   `json:"host_user_id"`
	ParticipantUserIDs []int64 `json:"participant_user_ids"`
}

// UpdateBooking either confirms an InProgress hold into Booked, or replaces
// the participant list on an InProgress/Booked booking.
func (h *BookingHandler) UpdateBooking(c *gin.Context) {
	slotID, err := strconv.ParseInt(c.Param("slotId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slotId"})
		return
	}
	var req updateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var slot models.Slot
	if err := h.DB.First(&slot, slotID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if slot.HostUserID == nil || *slot.HostUserID != req.HostUserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "host_user_id does not match the booking's host"})
		return
	}

	switch req.Action {
	case "confirm":
		if slot.BookingState != models.StateInProgress {
			c.JSON(http.StatusConflict, gin.H{"error": "booking is not in progress"})
			return
		}
		if slot.HoldExpiresAt == nil || slot.HoldExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusGone, gin.H{"error": "hold expired, please rebook"})
			return
		}
		err = h.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&slot).Updates(map[string]any{
				"booking_state":   models.StateBooked,
				"hold_expires_at": nil,
			}).Error; err != nil {
				return err
			}
			if req.ParticipantUserIDs != nil {
				return replaceParticipants(tx, slotID, req.ParticipantUserIDs)
			}
			return nil
		})
	case "update_participants":
		if slot.BookingState == models.StateCompleted {
			c.JSON(http.StatusConflict, gin.H{"error": "booking already completed"})
			return
		}
		err = replaceParticipants(h.DB, slotID, req.ParticipantUserIDs)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'confirm' or 'update_participants'"})
		return
	}
	if err != nil {
		respondPgError(c, err)
		return
	}

	h.DB.First(&slot, slotID)
	c.JSON(http.StatusOK, gin.H{
		"slot_id":          slot.SlotID,
		"room_id":          slot.RoomID,
		"booking_state":    slot.BookingState,
		"start_time":       slot.StartTime,
		"duration_seconds": slot.Duration,
		"host_user_id":     slot.HostUserID,
		"participant_ids":  h.fetchParticipantIDs(slotID),
		"hold_expires_at":  slot.HoldExpiresAt,
	})
}

// DeleteBooking frees the room by deleting the Slot row outright — there is
// no "Available" resting state to revert to, since availability is the
// absence of a row. SlotParticipant rows cascade-delete at the DB level.
func (h *BookingHandler) DeleteBooking(c *gin.Context) {
	slotID, err := strconv.ParseInt(c.Param("slotId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slotId"})
		return
	}
	var slot models.Slot
	if err := h.DB.First(&slot, slotID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if slot.BookingState == models.StateCompleted {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot delete a completed booking"})
		return
	}
	if err := h.DB.Delete(&models.Slot{}, slotID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
