package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ivanleekk/busarrival/internal/services"
	"github.com/ivanleekk/ltadatamall"
)

type RouteRequest struct {
	OriginCode      string `form:"originCode" binding:"required"`
	DestinationCode string `form:"destinationCode" binding:"required"`
}

type RouteResponse struct {
	RoutePath        *services.RoutePath
	CurrentArrivals  *ltadatamall.BusArrivalResponse
	PredictedArrival *services.PredictionResult
	Error            string `json:"error,omitempty"`
}

func GetIdealRouteHandler(client *ltadatamall.APIClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RouteRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "originCode and destinationCode are required query parameters"})
			return
		}

		ctx := context.Background()

		// 1. Find ideal route
		path, err := services.FindIdealRoute(ctx, req.OriginCode, req.DestinationCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find route: " + err.Error()})
			return
		}

		// Check if we found a path
		if path == nil || len(path.Stops) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no path found between origin and destination"})
			return
		}

		// 2. Fetch current arrivals at the starting stop
		arrivals, err := ltadatamall.GetBusArrivalAtBusStop(client, req.OriginCode)
		if err != nil {
			// We can proceed without live arrivals but should log it
			// For simplicity we attach nil
		}

		// 3. Try to predict arrival at destination using historical data
		// Find the service taken from the first stop
		var predictedArrival *services.PredictionResult
		if len(path.Services) > 0 && arrivals.Services != nil {
			firstService := ""
			// Relationships format is type_serviceNo e.g. BOARD_WALK or ROUTE_TO_190 or TRANSFER_TO_WALK
			// We can parse it naively here or simply use the first valid bus service in the path
			for _, srv := range path.Services {
				if srv != "TRANSFER_TO_WALK" && srv != "BOARD_WALK" && srv != "ALIGHT_WALK" {
					// extract service part
					// Simple string manipulation based on `type(r) + '_' + COALESCE(r.ServiceNo, 'WALK')`
					// e.g. "ROUTE_TO_190"
					if len(srv) > 9 && srv[:9] == "ROUTE_TO_" {
						firstService = srv[9:]
						break
					}
				}
			}

			if firstService != "" {
				// Find the expected arrival time of this service at origin
				var originExpected time.Time
				for _, arrSvc := range arrivals.Services {
					if arrSvc.ServiceNo == firstService && arrSvc.NextBus.EstimatedArrival != "" {
						t, err := time.Parse(time.RFC3339, arrSvc.NextBus.EstimatedArrival)
						if err == nil {
							originExpected = t
							break
						}
					}
				}

				if !originExpected.IsZero() {
					// Predict destination arrival
					prediction, err := services.PredictArrival(ctx, req.OriginCode, req.DestinationCode, firstService, originExpected)
					if err == nil {
						predictedArrival = prediction
					}
				}
			}
		}

		resp := RouteResponse{
			RoutePath:        path,
			CurrentArrivals:  &arrivals,
			PredictedArrival: predictedArrival,
		}

		c.JSON(http.StatusOK, resp)
	}
}
