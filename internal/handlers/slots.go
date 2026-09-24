package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"test/internal/models"
)

type SlotHandler struct {
	DB              *gorm.DB
	CacheWindowDays int
}

type timelineEntry struct {
	Type            string     `json:"type"`
	SlotID          *int64     `json:"slot_id,omitempty"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	DurationSeconds *int64     `json:"duration_seconds,omitempty"`
	BookingState    *string    `json:"booking_state,omitempty"`
	HostUserID      *int64     `json:"host_user_id,omitempty"`
	ParticipantIDs  []int64    `json:"participant_ids,omitempty"`
}

type freeWindow struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func roomLocation(room models.MeetingRoom) *time.Location {
	loc, err := time.LoadLocation(room.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (h *SlotHandler) loadRoom(c *gin.Context) (models.MeetingRoom, int64, bool) {
	roomID, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid roomId"})
		return models.MeetingRoom{}, 0, false
	}
	var room models.MeetingRoom
	if err := h.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return models.MeetingRoom{}, 0, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return models.MeetingRoom{}, 0, false
	}
	return room, roomID, true
}

func parseRequestedDate(c *gin.Context, loc *time.Location) (time.Time, bool) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date query param is required (YYYY-MM-DD)"})
		return time.Time{}, false
	}
	requestedDate, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be in YYYY-MM-DD format"})
		return time.Time{}, false
	}
	return requestedDate, true
}

func (h *SlotHandler) fetchParticipants(bookings []models.Slot) map[int64][]int64 {
	result := map[int64][]int64{}
	if len(bookings) == 0 {
		return result
	}
	slotIDs := make([]int64, len(bookings))
	for i, b := range bookings {
		slotIDs[i] = b.SlotID
	}
	var sps []models.SlotParticipant
	h.DB.Where("slot_id IN ?", slotIDs).Find(&sps)
	for _, sp := range sps {
		result[sp.SlotID] = append(result[sp.SlotID], sp.UserID)
	}
	return result
}

// computeTimeline merges real bookings with the computed free gaps between
// them across [windowStart, windowEnd). Slots are never pre-generated, so
// "free" is simply the absence of a booking row over that stretch of time.
func computeTimeline(bookings []models.Slot, windowStart, windowEnd time.Time, participantsBySlot map[int64][]int64) []timelineEntry {
	sort.Slice(bookings, func(i, j int) bool { return bookings[i].StartTime.Before(bookings[j].StartTime) })

	var out []timelineEntry
	cursor := windowStart
	for _, b := range bookings {
		bStart, bEnd := b.StartTime, b.EndTime
		if bStart.After(cursor) {
			gapEnd := bStart
			out = append(out, timelineEntry{Type: "free", StartTime: cursor, EndTime: &gapEnd})
		}
		state := string(b.BookingState)
		duration := b.Duration
		out = append(out, timelineEntry{
			Type:            "booking",
			SlotID:          &b.SlotID,
			StartTime:       bStart,
			DurationSeconds: &duration,
			BookingState:    &state,
			HostUserID:      b.HostUserID,
			ParticipantIDs:  participantsBySlot[b.SlotID],
		})
		if bEnd.After(cursor) {
			cursor = bEnd
		}
	}
	if windowEnd.After(cursor) {
		out = append(out, timelineEntry{Type: "free", StartTime: cursor, EndTime: &windowEnd})
	}
	return out
}

// ViewSlots reads from the MeetingRoom.Bookings ID cache when the requested
// date falls within the cache window, otherwise queries Slot directly. Both
// paths always fetch live rows — the cache only narrows candidate IDs.
func (h *SlotHandler) ViewSlots(c *gin.Context) {
	room, roomID, ok := h.loadRoom(c)
	if !ok {
		return
	}
	loc := roomLocation(room)
	dayStart, ok := parseRequestedDate(c, loc)
	if !ok {
		return
	}
	dayEnd := dayStart.AddDate(0, 0, 1)

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	diffDays := int(dayStart.Sub(today).Hours() / 24)

	var bookings []models.Slot
	if diffDays >= 0 && diffDays < h.CacheWindowDays {
		slotIDs := []int64(room.Bookings)
		if len(slotIDs) > 0 {
			h.DB.Where("slot_id IN ? AND start_time >= ? AND start_time < ?", slotIDs, dayStart, dayEnd).Find(&bookings)
		}
	} else {
		h.DB.Where("room_id = ? AND start_time >= ? AND start_time < ?", roomID, dayStart, dayEnd).Find(&bookings)
	}

	participantsBySlot := h.fetchParticipants(bookings)
	timeline := computeTimeline(bookings, dayStart, dayEnd, participantsBySlot)

	c.JSON(http.StatusOK, gin.H{
		"room_id":  roomID,
		"date":     c.Query("date"),
		"timezone": room.Timezone,
		"timeline": timeline,
	})
}

// GetAvailableSlots returns just the free gaps for a room/day, optionally
// clipped to a from/to time-of-day window. Always queried directly since the
// 3-day cache offers no benefit for a single-room, single-day lookup.
func (h *SlotHandler) GetAvailableSlots(c *gin.Context) {
	room, roomID, ok := h.loadRoom(c)
	if !ok {
		return
	}
	loc := roomLocation(room)
	dayStart, ok := parseRequestedDate(c, loc)
	if !ok {
		return
	}
	dayEnd := dayStart.AddDate(0, 0, 1)

	windowStart, windowEnd := dayStart, dayEnd
	if fromStr := c.Query("from"); fromStr != "" {
		t, err := time.ParseInLocation("15:04", fromStr, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be in HH:MM format"})
			return
		}
		windowStart = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	}
	if toStr := c.Query("to"); toStr != "" {
		t, err := time.ParseInLocation("15:04", toStr, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be in HH:MM format"})
			return
		}
		windowEnd = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	}
	if !windowStart.Before(windowEnd) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
		return
	}

	var bookings []models.Slot
	h.DB.Where("room_id = ? AND start_time >= ? AND start_time < ? AND booking_state IN ?",
		roomID, dayStart, dayEnd, []models.BookingState{models.StateInProgress, models.StateBooked}).Find(&bookings)

	timeline := computeTimeline(bookings, dayStart, dayEnd, nil)

	var free []freeWindow
	for _, e := range timeline {
		if e.Type != "free" {
			continue
		}
		start, end := e.StartTime, *e.EndTime
		if start.Before(windowStart) {
			start = windowStart
		}
		if end.After(windowEnd) {
			end = windowEnd
		}
		if start.Before(end) {
			free = append(free, freeWindow{StartTime: start, EndTime: end})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id":      roomID,
		"date":         c.Query("date"),
		"free_windows": free,
	})
}
