package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"test/internal/models"
)

type RoomHandler struct {
	DB *gorm.DB
}

type createRoomRequest struct {
	OfficeID int64  `json:"office_id" binding:"required"`
	Timezone string `json:"timezone"`
}

func (h *RoomHandler) Create(c *gin.Context) {
	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var office models.Office
	if err := h.DB.First(&office, req.OfficeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "office_id does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	room := models.MeetingRoom{OfficeID: req.OfficeID, Timezone: timezone, Bookings: models.Int64List{}}
	if err := h.DB.Create(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, room)
}

func (h *RoomHandler) List(c *gin.Context) {
	var rooms []models.MeetingRoom
	if err := h.DB.Find(&rooms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rooms)
}
