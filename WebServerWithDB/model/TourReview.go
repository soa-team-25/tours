package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TourReview struct {
	ID             uuid.UUID `json:"id"`
	Grade          float64   `json:"grade"`
	Comment        string    `json:"comment"`
	TouristID      int       `json:"touristId"`
	AttendanceDate time.Time `json:"attendanceDate"`
	ReviewDate     time.Time `json:"reviewDate"`
	Images         []string  `json:"images"`
	TourID         int64     `json:"tourId"`
}

func (tr *TourReview) BeforeCreate(tx *gorm.DB) (err error) {
	tr.ID = uuid.New()
	return nil
}
