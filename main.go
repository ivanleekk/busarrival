package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ivanleekk/ltadatamall"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env from the project root
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

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

	router := gin.Default()
	router.GET("/busstops/:skip", getBusStopsPaginatedHandler(client))
	router.GET("/busstops", getBusStopsHandler(client))
	err := router.Run("0.0.0.0:8080")
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
