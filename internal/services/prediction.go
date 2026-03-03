package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ivanleekk/busarrival/internal/database"
	"github.com/ivanleekk/busarrival/internal/models"
)

type PredictionResult struct {
	PredictedArrival time.Time
	LowerBound       time.Time
	UpperBound       time.Time
	Confidence       string
}

// PredictArrival analyzes historical data to predict the arrival time at a destination bus stop.
// For initial simplicity, we will calculate the average travel time and standard deviation
// between origin and destination for the given bus service, based on past logs.
func PredictArrival(ctx context.Context, originCode string, destinationCode string, serviceNo string, originExpectedTime time.Time) (*PredictionResult, error) {
	// Let's get the historical logs for this service at both stops
	// We need pairs of arrivals: a bus arriving at origin and then arriving at destination.
	// Since we don't have direct trip IDs from the API, we can approximate by matching
	// arrivals on the same day within a reasonable timeframe.
	// For this initial implementation, we'll calculate a moving average of travel times if we have data.

	// In a real scenario, this would involve a complex query to match sequences.
	// Let's implement a simplified statistical approach:

	// Since this is a new system and we might not have data yet, we need a fallback.
	// Fallback: assume average speed is ~20km/h and distance is roughly proportional to stop sequence.
	// But without coordinates passed here, let's just return an empty prediction if no data is found.

	var logsOrigin []models.ArrivalLog
	var logsDest []models.ArrivalLog

	// Only query recent data for relevance (e.g., past 30 days)
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)

	err := database.DB.Where("stop_code = ? AND bus_number = ? AND expected_arrival > ?", originCode, serviceNo, thirtyDaysAgo).
		Order("expected_arrival ASC").Find(&logsOrigin).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch origin logs: %v", err)
	}

	err = database.DB.Where("stop_code = ? AND bus_number = ? AND expected_arrival > ?", destinationCode, serviceNo, thirtyDaysAgo).
		Order("expected_arrival ASC").Find(&logsDest).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch destination logs: %v", err)
	}

	if len(logsOrigin) == 0 || len(logsDest) == 0 {
		return nil, fmt.Errorf("insufficient historical data for prediction")
	}

	var travelTimes []float64

	// Match pairs naively by chronological order, assuming the bus reaches destination after origin.
	// We find a destination arrival that occurs after the origin arrival within 2 hours.
	for _, oLog := range logsOrigin {
		for _, dLog := range logsDest {
			diff := dLog.ExpectedArrival.Sub(oLog.ExpectedArrival)
			if diff > 0 && diff < 2*time.Hour {
				travelTimes = append(travelTimes, diff.Seconds())
				break // matched this trip
			}
		}
	}

	if len(travelTimes) == 0 {
		return nil, fmt.Errorf("could not correlate any trips for prediction")
	}

	// Calculate Mean and Standard Deviation
	var sum float64
	for _, t := range travelTimes {
		sum += t
	}
	meanSecs := sum / float64(len(travelTimes))

	var sqSum float64
	for _, t := range travelTimes {
		sqSum += math.Pow(t-meanSecs, 2)
	}
	variance := sqSum / float64(len(travelTimes))
	stdDevSecs := math.Sqrt(variance)

	predictedTime := originExpectedTime.Add(time.Duration(meanSecs) * time.Second)
	// 95% confidence interval roughly +/- 1.96 * stdDev
	marginOfError := 1.96 * stdDevSecs

	lowerBound := originExpectedTime.Add(time.Duration(meanSecs-marginOfError) * time.Second)
	upperBound := originExpectedTime.Add(time.Duration(meanSecs+marginOfError) * time.Second)

	return &PredictionResult{
		PredictedArrival: predictedTime,
		LowerBound:       lowerBound,
		UpperBound:       upperBound,
		Confidence:       "95%",
	}, nil
}
