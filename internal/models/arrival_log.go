package models

import (
	"time"

	"gorm.io/gorm"
)

type ArrivalLog struct {
	gorm.Model
	StopCode           string    `gorm:"index"`
	BusNumber          string    `gorm:"index"`
	ExpectedArrival    time.Time `gorm:"index"`
	RecordedAt         time.Time `gorm:"index"`
	Load               string
	Feature            string
	Type               string
	VisitNumber        int
	OriginCode         string
	DestinationCode    string
	EstimatedLatitude  float64
	EstimatedLongitude float64
}
