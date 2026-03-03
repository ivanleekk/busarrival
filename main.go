package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ivanleekk/busarrival/internal/api"
	"github.com/gin-contrib/cors"
	"github.com/ivanleekk/busarrival/internal/database"
	"github.com/ivanleekk/busarrival/internal/graph"
	"github.com/ivanleekk/busarrival/internal/poller"
	"github.com/ivanleekk/ltadatamall"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env from the project root
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

	// Initialize databases
	database.InitDB()
	graph.InitNeo4j()

	// Initialize the client once
	accountKey := os.Getenv("LTA_DATAMALL_ACCOUNT_KEY")
	if accountKey == "" {
		log.Fatal("LTA_DATAMALL_ACCOUNT_KEY not set in environment")
	}

	baseUrl := os.Getenv("LTA_DATAMALL_BASE_URL")
	if baseUrl == "" {
		log.Fatal("LTA_DATAMALL_BASE_URL not set in environment")
	}

	var client = ltadatamall.NewClient(baseUrl, accountKey)

	// Build Graph on startup (can be refactored to an explicit API endpoint/cron later)
	ctx := context.Background()
	log.Println("Checking and building graph...")
	err := graph.BuildGraph(ctx, client)
	if err != nil {
		log.Printf("Failed to build graph: %v", err)
	}

	// Start Poller
	poller.StartPoller(client)

	router := gin.Default()

	// Setup CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Adjust in production
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.GET("/busstops/:skip", getBusStopsPaginatedHandler(client))
	router.GET("/busstops", getBusStopsHandler(client))

	// New routing and prediction endpoint
	router.GET("/api/route", api.GetIdealRouteHandler(client))

	// Analysis endpoints
	router.GET("/api/analysis/intervals", api.GetArrivalIntervalsHandler())

	// Live arrivals endpoint
	router.GET("/api/arrivals/:stopCode", func(c *gin.Context) {
		stopCode := c.Param("stopCode")
		arrivals, err := ltadatamall.GetBusArrivalAtBusStop(client, stopCode)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, arrivals)
	})

	err = router.Run("0.0.0.0:8080")
	if err != nil {
		return
	}
}

func getBusStopsPaginatedHandler(client *ltadatamall.APIClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		skipParam, err := strconv.Atoi(c.Param("skip"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		busStopsResponse, err := ltadatamall.GetBusStopsPaginated(client, skipParam)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, busStopsResponse)
	}
}

func getBusStopsHandler(client *ltadatamall.APIClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		busStopsResponse, err := ltadatamall.GetAllBusStops(client)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, busStopsResponse)
	}
}
