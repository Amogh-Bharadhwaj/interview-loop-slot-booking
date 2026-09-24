package models

type BookingState string

const (
	StateInProgress BookingState = "InProgress"
	StateBooked     BookingState = "Booked"
	StateCompleted  BookingState = "Completed"
)
