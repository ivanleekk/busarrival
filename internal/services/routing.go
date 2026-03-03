package services

import (
	"context"
	"fmt"

	"github.com/ivanleekk/busarrival/internal/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type RoutePath struct {
	Stops    []string
	Services []string
	Cost     float64
}

// FindIdealRoute uses Neo4j GDS or simple Cypher to find the shortest path between two bus stops
func FindIdealRoute(ctx context.Context, originCode string, destinationCode string) (*RoutePath, error) {
	session := graph.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Since we don't know if GDS is installed, we can use a basic shortestPath algorithm built into Cypher
	// This finds a path minimizing the number of hops (which is a basic approximation).
	// For better routing (minimizing distance or wait times), we'd need Dijkstra with edge weights.
	// But let's implement a Dijkstra-like Cypher using apoc or native if available.
	// Native shortestPath considers hop count. Let's use it for the initial version.

	// Using basic shortestPath directly for stability and simplicity without requiring APOC plugin.
	fallbackQuery := `
		MATCH p = shortestPath((start:BusStop {Code: $originCode})-[:ROUTE_TO|TRANSFER_TO*..50]->(end:BusStop {Code: $destCode}))
		RETURN [n IN nodes(p) | n.Code] AS stops,
		       [r IN relationships(p) | type(r) + '_' + COALESCE(r.ServiceNo, 'WALK')] AS services,
		       0.0 AS weight
	`

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Since APOC isn't guaranteed and Tx API is slightly different
		// let's use the fallback query directly for stability and simplicity without APOC
		params := map[string]any{"originCode": originCode, "destCode": destinationCode}
		result, err := tx.Run(ctx, fallbackQuery, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			record := result.Record()
			return record, nil
		}
		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute path query: %v", err)
	}

	if res == nil {
		return nil, fmt.Errorf("no path found between %s and %s", originCode, destinationCode)
	}

	record := res.(*neo4j.Record)

		stopsRaw, _ := record.Get("stops")
		servicesRaw, _ := record.Get("services")
		weightRaw, _ := record.Get("weight")

		var stops []string
		for _, s := range stopsRaw.([]interface{}) {
			stops = append(stops, s.(string))
		}

		var services []string
		for _, s := range servicesRaw.([]interface{}) {
			services = append(services, s.(string))
		}

		weight := 0.0
		switch v := weightRaw.(type) {
		case float64:
			weight = v
		case int64:
			weight = float64(v)
		}

		return &RoutePath{
			Stops:    stops,
			Services: services,
			Cost:     weight,
		}, nil
}
