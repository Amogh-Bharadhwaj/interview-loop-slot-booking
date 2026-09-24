package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"test/internal/models"
)

type OfficeHandler struct {
	DB *gorm.DB
}

type createOfficeRequest struct {
	Location string `json:"location" binding:"required"`
}

func (h *OfficeHandler) Create(c *gin.Context) {
	var req createOfficeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	office := models.Office{Location: req.Location}
	if err := h.DB.Create(&office).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, office)
}

func (h *OfficeHandler) List(c *gin.Context) {
	var offices []models.Office
	if err := h.DB.Find(&offices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offices)
}
