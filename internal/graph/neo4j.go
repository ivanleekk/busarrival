package graph

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ivanleekk/ltadatamall"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

var Driver neo4j.DriverWithContext

func InitNeo4j() {
	uri := os.Getenv("NEO4J_URI")
	username := os.Getenv("NEO4J_USERNAME")
	password := os.Getenv("NEO4J_PASSWORD")

	if uri == "" {
		uri = "neo4j://localhost:7687"
		username = "neo4j"
		password = "password"
	}

	var err error
	Driver, err = neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}

	ctx := context.Background()

	// Retry logic for Neo4j startup since it takes a while to boot inside Docker
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		err = Driver.VerifyConnectivity(ctx)
		if err == nil {
			break
		}
		log.Printf("Neo4j not ready (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to connect to Neo4j after %d attempts: %v", maxRetries, err)
	}

	log.Println("Neo4j initialized.")

	// Create constraints if they do not exist
	createConstraints(ctx)
}

func createConstraints(ctx context.Context) {
	session := Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(transaction neo4j.ManagedTransaction) (any, error) {
		return transaction.Run(ctx, "CREATE CONSTRAINT bus_stop_code IF NOT EXISTS FOR (b:BusStop) REQUIRE b.Code IS UNIQUE", nil)
	})
	if err != nil {
		log.Printf("Warning: failed to create constraint on BusStop(Code): %v", err)
	}

	_, err = session.ExecuteWrite(ctx, func(transaction neo4j.ManagedTransaction) (any, error) {
		return transaction.Run(ctx, "CREATE CONSTRAINT service_stop_id IF NOT EXISTS FOR (s:ServiceStop) REQUIRE s.ID IS UNIQUE", nil)
	})
	if err != nil {
		log.Printf("Warning: failed to create constraint on ServiceStop(ID): %v", err)
	}
}

// Function to calculate Haversine distance in meters
const queryHaversine = `
WITH 6371000 AS R
MATCH (a:BusStop), (b:BusStop)
WHERE id(a) < id(b)
WITH a, b, R,
     radians(a.Latitude) AS lat1,
     radians(a.Longitude) AS lon1,
     radians(b.Latitude) AS lat2,
     radians(b.Longitude) AS lon2
WITH a, b, R, lat1, lon1, lat2, lon2,
     lat2 - lat1 AS dlat,
     lon2 - lon1 AS dlon
WITH a, b, R, dlat, dlon, lat1, lat2,
     sin(dlat/2)^2 + cos(lat1) * cos(lat2) * sin(dlon/2)^2 AS a_val
WITH a, b, R, a_val,
     2 * atan2(sqrt(a_val), sqrt(1 - a_val)) AS c
WITH a, b, R * c AS distance
WHERE distance <= 400
MERGE (a)-[r:TRANSFER_TO]->(b)
SET r.distance = distance
MERGE (b)-[r2:TRANSFER_TO]->(a)
SET r2.distance = distance
`

func BuildGraph(ctx context.Context, client *ltadatamall.APIClient) error {
	session := Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	log.Println("Fetching all bus stops to populate Neo4j...")
	busStopsRes, err := ltadatamall.GetAllBusStops(client)
	if err != nil {
		return fmt.Errorf("failed to fetch bus stops: %v", err)
	}

	for _, stop := range busStopsRes.BusStops {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := `
				MERGE (b:BusStop {Code: $code})
				SET b.Description = $description,
				    b.RoadName = $roadName,
				    b.Latitude = $lat,
				    b.Longitude = $lon
			`
			params := map[string]interface{}{
				"code":        stop.BusStopCode,
				"description": stop.Description,
				"roadName":    stop.RoadName,
				"lat":         stop.Latitude,
				"lon":         stop.Longitude,
			}
			return tx.Run(ctx, query, params)
		})
		if err != nil {
			log.Printf("Failed to insert bus stop %s: %v", stop.BusStopCode, err)
		}
	}

	log.Println("Fetching all bus routes to populate Neo4j...")
	busRoutesRes, err := ltadatamall.GetAllBusRoutes(client)
	if err != nil {
		return fmt.Errorf("failed to fetch bus routes: %v", err)
	}

	// Group routes by ServiceNo and Direction to create sequence
	routesByService := make(map[string]map[int][]ltadatamall.BusRoute)
	for _, route := range busRoutesRes.BusRoutes {
		if _, ok := routesByService[route.ServiceNo]; !ok {
			routesByService[route.ServiceNo] = make(map[int][]ltadatamall.BusRoute)
		}
		routesByService[route.ServiceNo][route.Direction] = append(routesByService[route.ServiceNo][route.Direction], route)
	}

	for serviceNo, dirs := range routesByService {
		for dir, routes := range dirs {
			// Sort routes by StopSequence manually since LTA API returns them mostly sorted but we need to ensure it
			// However, let's assume they are somewhat ordered for now, or just link them sequentially if we just loop
			// Ideally we sort them:
			// sort.SliceStable(routes, func(i, j int) bool { return routes[i].StopSequence < routes[j].StopSequence })

			for i := 0; i < len(routes); i++ {
				curr := routes[i]

				// Create the ServiceStop node representing this service arriving at this stop
				currID := fmt.Sprintf("%s-%d-%s", serviceNo, dir, curr.BusStopCode)

				_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
					// 1. Merge the ServiceStop
					// 2. Link it to the physical BusStop (BOARD and ALIGHT)
					query := `
						MATCH (bs:BusStop {Code: $code})
						MERGE (ss:ServiceStop {ID: $id})
						SET ss.ServiceNo = $service,
						    ss.Direction = $dir,
						    ss.StopCode = $code,
						    ss.StopSequence = $seq

						// Create BOARD relationship from physical stop to service stop
						// We add a 'penalty' distance (e.g. 100m) to discourage excessive transferring
						MERGE (bs)-[b:BOARD]->(ss)
						SET b.Distance = 100

						// Create ALIGHT relationship from service stop to physical stop
						MERGE (ss)-[a:ALIGHT]->(bs)
						SET a.Distance = 0
					`
					params := map[string]interface{}{
						"id":      currID,
						"code":    curr.BusStopCode,
						"service": serviceNo,
						"dir":     dir,
						"seq":     curr.StopSequence,
					}
					return tx.Run(ctx, query, params)
				})
				if err != nil {
					log.Printf("Failed to insert ServiceStop %s: %v", currID, err)
				}

				// If there's a next stop, connect the two ServiceStops directly
				if i < len(routes)-1 {
					next := routes[i+1]
					nextID := fmt.Sprintf("%s-%d-%s", serviceNo, dir, next.BusStopCode)

					_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
						query := `
							MATCH (curr:ServiceStop {ID: $currID}), (next:ServiceStop {ID: $nextID})
							MERGE (curr)-[r:ROUTE_TO]->(next)
							SET r.Distance = $dist,
							    r.ServiceNo = $service
						`
						params := map[string]interface{}{
							"currID":  currID,
							"nextID":  nextID,
							"dist":    next.Distance - curr.Distance, // Actual travel distance
							"service": serviceNo,
						}
						return tx.Run(ctx, query, params)
					})
					if err != nil {
						log.Printf("Failed to insert ROUTE_TO edge %s -> %s: %v", currID, nextID, err)
					}
				}
			}
		}
	}

	log.Println("Building transfer edges...")
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, queryHaversine, nil)
	})
	if err != nil {
		return fmt.Errorf("failed to build transfer edges: %v", err)
	}

	log.Println("Graph built successfully.")
	return nil
}
