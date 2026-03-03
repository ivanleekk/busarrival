package poller

import (
	"log"
	"sync"
	"time"

	"github.com/ivanleekk/busarrival/internal/database"
	"github.com/ivanleekk/busarrival/internal/models"
	"github.com/ivanleekk/ltadatamall"
)

func StartPoller(client *ltadatamall.APIClient) {
	log.Println("Starting bus arrival poller...")

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		// Run once immediately
		poll(client)
		for range ticker.C {
			poll(client)
		}
	}()
}

func poll(client *ltadatamall.APIClient) {
	log.Println("Polling bus arrivals...")

	busStopsRes, err := ltadatamall.GetAllBusStops(client)
	if err != nil {
		log.Printf("Poller failed to fetch bus stops: %v", err)
		return
	}

	var wg sync.WaitGroup
	// Limit concurrency to avoid overwhelming the API or getting rate limited
	sem := make(chan struct{}, 2)

	for _, stop := range busStopsRes.BusStops {
		wg.Add(1)
		sem <- struct{}{}
		go func(stopCode string) {
			defer wg.Done()
			defer func() { <-sem }()

			fetchAndStoreArrivals(client, stopCode)
		}(stop.BusStopCode)
	}

	wg.Wait()
	log.Println("Finished polling cycle.")
}

func fetchAndStoreArrivals(client *ltadatamall.APIClient, stopCode string) {
	arrivalRes, err := ltadatamall.GetBusArrivalAtBusStop(client, stopCode)
	if err != nil {
		log.Printf("Failed to fetch arrival for stop %s: %v", stopCode, err)
		return
	}

	var logs []models.ArrivalLog
	now := time.Now()

	for _, service := range arrivalRes.Services {
		// Process NextBus
		if service.NextBus.EstimatedArrival != "" {
			t, err := time.Parse(time.RFC3339, service.NextBus.EstimatedArrival)
			if err == nil {
				logs = append(logs, models.ArrivalLog{
					StopCode:        stopCode,
					BusNumber:       service.ServiceNo,
					ExpectedArrival: t,
					RecordedAt:      now,
					Load:            service.NextBus.Load,
					Feature:         service.NextBus.Feature,
					Type:            service.NextBus.Type,
				})
			}
		}

		// Optionally process NextBus2 and NextBus3 if needed
		// For detailed logging we might just care about the immediate next bus,
		// or log all of them. Let's log them all to have richer data for predictions.
		if service.NextBus2.EstimatedArrival != "" {
			t, err := time.Parse(time.RFC3339, service.NextBus2.EstimatedArrival)
			if err == nil {
				logs = append(logs, models.ArrivalLog{
					StopCode:        stopCode,
					BusNumber:       service.ServiceNo,
					ExpectedArrival: t,
					RecordedAt:      now,
					Load:            service.NextBus2.Load,
					Feature:         service.NextBus2.Feature,
					Type:            service.NextBus2.Type,
				})
			}
		}

		if service.NextBus3.EstimatedArrival != "" {
			t, err := time.Parse(time.RFC3339, service.NextBus3.EstimatedArrival)
			if err == nil {
				logs = append(logs, models.ArrivalLog{
					StopCode:        stopCode,
					BusNumber:       service.ServiceNo,
					ExpectedArrival: t,
					RecordedAt:      now,
					Load:            service.NextBus3.Load,
					Feature:         service.NextBus3.Feature,
					Type:            service.NextBus3.Type,
				})
			}
		}
	}

	if len(logs) > 0 {
		if err := database.DB.Create(&logs).Error; err != nil {
			log.Printf("Failed to insert arrival logs for stop %s: %v", stopCode, err)
		}
	}
}
