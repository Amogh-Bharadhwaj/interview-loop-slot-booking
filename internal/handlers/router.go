package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(gdb *gorm.DB, holdWindow time.Duration, cacheWindowDays int) *gin.Engine {
	health := &HealthHandler{DB: gdb}
	offices := &OfficeHandler{DB: gdb}
	rooms := &RoomHandler{DB: gdb}
	users := &UserHandler{DB: gdb}
	slots := &SlotHandler{DB: gdb, CacheWindowDays: cacheWindowDays}
	bookings := &BookingHandler{DB: gdb, HoldWindow: holdWindow, CacheWindowDays: cacheWindowDays}

	r := gin.Default()

	r.GET("/healthz", health.HealthCheck)

	r.POST("/offices", offices.Create)
	r.GET("/offices", offices.List)

	r.POST("/rooms", rooms.Create)
	r.GET("/rooms", rooms.List)

	r.POST("/users", users.Create)
	r.GET("/users", users.List)

	r.GET("/rooms/:roomId/slots", slots.ViewSlots)
	r.GET("/rooms/:roomId/slots/available", slots.GetAvailableSlots)

	r.POST("/bookings", bookings.CreateBooking)
	r.PATCH("/bookings/:slotId", bookings.UpdateBooking)
	r.DELETE("/bookings/:slotId", bookings.DeleteBooking)

	return r
}
