package api

import (
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ivanleekk/busarrival/internal/database"
	"github.com/ivanleekk/busarrival/internal/models"
)

type IntervalAnalysisRequest struct {
	StopCode  string `form:"stopCode" binding:"required"`
	BusNumber string `form:"busNumber" binding:"required"`
}

type GroupedInterval struct {
	Hour           int     `json:"hour"`
	AverageMinutes float64 `json:"averageMinutes"`
	Variance       float64 `json:"variance"`
	StandardDev    float64 `json:"standardDev"`
	Count          int     `json:"count"`
}

type IntervalAnalysisResponse struct {
	StopCode   string            `json:"stopCode"`
	BusNumber  string            `json:"busNumber"`
	HourlyData []GroupedInterval `json:"hourlyData"`
}

func GetArrivalIntervalsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req IntervalAnalysisRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stopCode and busNumber are required query parameters"})
			return
		}

		// Calculate intervals for the last 30 days
		thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
		var logs []models.ArrivalLog

		err := database.DB.Where("stop_code = ? AND bus_number = ? AND expected_arrival > ?", req.StopCode, req.BusNumber, thirtyDaysAgo).
			Order("expected_arrival ASC").Find(&logs).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch arrival logs"})
			return
		}

		if len(logs) < 2 {
			c.JSON(http.StatusOK, IntervalAnalysisResponse{
				StopCode:   req.StopCode,
				BusNumber:  req.BusNumber,
				HourlyData: []GroupedInterval{},
			})
			return
		}

		intervalsByHour := make(map[int][]float64)

		for i := 1; i < len(logs); i++ {
			prev := logs[i-1].ExpectedArrival
			curr := logs[i].ExpectedArrival

			diff := curr.Sub(prev)
			// Consider diffs between 1 min and 2 hours (ignore duplicates/same-bus updates and night gaps)
			if diff > time.Minute && diff < 2*time.Hour {
				hour := prev.Hour()
				intervalsByHour[hour] = append(intervalsByHour[hour], diff.Minutes())
			}
		}

		var hourlyData []GroupedInterval

		for hour, intervals := range intervalsByHour {
			if len(intervals) == 0 {
				continue
			}

			var sum float64
			for _, t := range intervals {
				sum += t
			}
			mean := sum / float64(len(intervals))

			var sqSum float64
			for _, t := range intervals {
				sqSum += math.Pow(t-mean, 2)
			}
			variance := sqSum / float64(len(intervals))
			stdDev := math.Sqrt(variance)

			hourlyData = append(hourlyData, GroupedInterval{
				Hour:           hour,
				AverageMinutes: mean,
				Variance:       variance,
				StandardDev:    stdDev,
				Count:          len(intervals),
			})
		}

		sort.Slice(hourlyData, func(i, j int) bool {
			return hourlyData[i].Hour < hourlyData[j].Hour
		})

		c.JSON(http.StatusOK, IntervalAnalysisResponse{
			StopCode:   req.StopCode,
			BusNumber:  req.BusNumber,
			HourlyData: hourlyData,
		})
	}
}
