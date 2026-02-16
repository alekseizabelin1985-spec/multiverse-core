// Package semanticmemory handles Neo4j integration.
package semanticmemory

import (
	"fmt"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jClient handles communication with Neo4j.
type Neo4jClient struct {
	driver neo4j.Driver
}

// NewNeo4jClient creates a new Neo4jClient.
func NewNeo4jClient() (*Neo4jClient, error) {
	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		uri = "neo4j://neo4j:7687"
	}
	user := os.Getenv("NEO4J_USER")
	if user == "" {
		user = "neo4j"
	}
	password := os.Getenv("NEO4J_PASSWORD")
	if password == "" {
		password = "password"
	}

	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver creation failed: %w", err)
	}

	// Test connection (Neo4j v5 API)
	if err := driver.VerifyConnectivity(); err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("neo4j connectivity test failed: %w", err)
	}

	return &Neo4jClient{driver: driver}, nil
}

// UpsertEntity creates or updates an entity node in Neo4j.
func (n *Neo4jClient) UpsertEntity(entityID, entityType string, payload map[string]interface{}) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
MERGE (e:Entity {id: $entity_id})
SET e.type = $entity_type,
    e += $payload
RETURN e
`

	_, err := session.Run(query, map[string]any{
		"entity_id":   entityID,
		"entity_type": entityType,
		"payload":     payload,
	})
	return err
}

// CreateRelationship creates a relationship between entities.
func (n *Neo4jClient) CreateRelationship(fromID, toID, relType string) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := fmt.Sprintf(`
MATCH (a:Entity {id: $from_id})
MATCH (b:Entity {id: $to_id})
MERGE (a)-[r:%s]->(b)
RETURN r
`, relType)

	_, err := session.Run(query, map[string]any{
		"from_id": fromID,
		"to_id":   toID,
	})
	return err
}

// StoreEvent stores an event in Neo4j as a node with relationships to relevant entities.
func (n *Neo4jClient) StoreEvent(eventID string, eventType string, timestamp string, entityID string, eventData map[string]interface{}) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	// Create or merge the event node with all event data
	eventQuery := `
		MERGE (e:Event {id: $eventID})
		SET e.type = $eventType, e.timestamp = $timestamp
		SET e += $eventData
		RETURN e
	`

	_, err := session.Run(eventQuery, map[string]any{
		"eventID":   eventID,
		"eventType": eventType,
		"timestamp": timestamp,
		"eventData": eventData,
	})
	if err != nil {
		return fmt.Errorf("failed to create/merge event node: %w", err)
	}

	// Create relationship to the associated entity if entityID is provided
	if entityID != "" {
		relQuery := `
			MERGE (ev:Event {id: $eventID})
			MERGE (en:Entity {id: $entityID})
			MERGE (ev)-[:RELATED_TO]->(en)
		`

		_, err := session.Run(relQuery, map[string]any{
			"eventID":  eventID,
			"entityID": entityID,
		})
		if err != nil {
			return fmt.Errorf("failed to create relationship to entity: %w", err)
		}
	}

	return nil
}

// GetEventsByType retrieves events of a specific type from Neo4j, sorted by timestamp.
func (n *Neo4jClient) GetEventsByType(eventType string, limit int) ([]map[string]interface{}, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		query := `
			MATCH (e:Event {type: $eventType})
			RETURN e
			ORDER BY e.timestamp DESC
			LIMIT $limit
		`

		records, err := tx.Run(query, map[string]any{
			"eventType": eventType,
			"limit":     limit,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		var events []map[string]interface{}
		for records.Next() {
			record := records.Record()
			eventNode, ok := record.Get("e")
			if !ok {
				continue // Skip invalid records
			}

			// Convert node to map
			if node, ok := eventNode.(neo4j.Node); ok {
				events = append(events, node.Props)
			}
		}

		return events, nil
	})

	if err != nil {
		return nil, err
	}

	events, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to convert result to events slice")
	}

	return events, nil
}

// GetEntityCache retrieves entity information for a list of entity IDs.
// Neo4j v5 compatible: no context in method signatures.
func (n *Neo4jClient) GetEntityCache(entityIDs []string) (map[string]EntityInfo, error) {
	if n.driver == nil {
		return nil, fmt.Errorf("neo4j driver not initialized")
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
	MATCH (e:Entity)
	WHERE e.id IN $entity_ids
	RETURN e.id AS id, e.name AS name, e.type AS type, e.world_id AS world_id, e.description AS description, e.payload AS payload
	`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		res, err := tx.Run(query, map[string]any{
			"entity_ids": entityIDs,
		})
		if err != nil {
			return nil, err
		}

		cache := make(map[string]EntityInfo)
		for res.Next() {
			record := res.Record()

			var id, name, entityType, worldID, description string
			var payload map[string]interface{}

			if val, ok := record.Get("id"); ok && val != nil {
				if s, ok := val.(string); ok {
					id = s
				}
			}
			if val, ok := record.Get("name"); ok && val != nil {
				if s, ok := val.(string); ok {
					name = s
				}
			}
			if val, ok := record.Get("type"); ok && val != nil {
				if s, ok := val.(string); ok {
					entityType = s
				}
			}
			if val, ok := record.Get("world_id"); ok && val != nil {
				if s, ok := val.(string); ok {
					worldID = s
				}
			}
			if val, ok := record.Get("description"); ok && val != nil {
				if s, ok := val.(string); ok {
					description = s
				}
			}
			if val, ok := record.Get("payload"); ok && val != nil {
				if p, ok := val.(map[string]interface{}); ok {
					payload = p
				}
			}

			if id != "" {
				cache[id] = EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
			}
		}

		return cache, res.Err()
	})

	if err != nil {
		return nil, err
	}

	if cache, ok := result.(map[string]EntityInfo); ok {
		return cache, nil
	}

	return nil, fmt.Errorf("unexpected result type from GetEntityCache")
}

// Close closes the Neo4j driver.
func (n *Neo4jClient) Close() {
	_ = n.driver.Close()
}
