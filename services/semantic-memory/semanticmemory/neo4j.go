// Package semanticmemory handles Neo4j integration.
package semanticmemory

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"multiverse-core.io/shared/eventbus"

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

// LinkEventToEntities creates RELATED_TO edges from an Event node to Entity nodes
// whose IDs are found in the event payload. Entity stub nodes are created if missing.
// Recognised payload keys: entity_id, source_id, target_id, character_id, player_id, npc_id.
func (n *Neo4jClient) LinkEventToEntities(eventID string, payload map[string]interface{}) error {
	entityKeys := []string{"entity_id", "source_id", "target_id", "character_id", "player_id", "npc_id"}

	var entityIDs []string
	for _, key := range entityKeys {
		if val, ok := payload[key].(string); ok && val != "" {
			entityIDs = append(entityIDs, val)
		}
	}

	if len(entityIDs) == 0 {
		return nil
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
MATCH (ev:Event {id: $event_id})
UNWIND $entity_ids AS eid
MERGE (en:Entity {id: eid})
MERGE (ev)-[:RELATED_TO]->(en)
`
	_, err := session.Run(query, map[string]any{
		"event_id":   eventID,
		"entity_ids": entityIDs,
	})
	if err != nil {
		return fmt.Errorf("LinkEventToEntities: %w", err)
	}
	return nil
}

// GetEventsForEntities retrieves events related to the given entity IDs from Neo4j.
// Optionally filters by worldID (pass "" to skip) and returns events newer than `since`.
// Results are ordered by timestamp descending and capped at `limit`.
func (n *Neo4jClient) GetEventsForEntities(entityIDs []string, worldID string, since time.Time, limit int) ([]eventbus.Event, error) {
	if len(entityIDs) == 0 {
		return nil, nil
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
MATCH (ev:Event)-[:RELATED_TO]->(en:Entity)
WHERE en.id IN $entity_ids
  AND ($world_id = '' OR ev.world_id = $world_id)
  AND ev.timestamp >= $since
RETURN DISTINCT ev.raw_data AS raw_data
ORDER BY ev.timestamp DESC
LIMIT $limit
`
	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"entity_ids": entityIDs,
			"world_id":   worldID,
			"since":      since,
			"limit":      limit,
		})
		if err != nil {
			return nil, err
		}

		var events []eventbus.Event
		for records.Next() {
			record := records.Record()
			rawVal, ok := record.Get("raw_data")
			if !ok || rawVal == nil {
				continue
			}
			rawStr, ok := rawVal.(string)
			if !ok {
				continue
			}
			var ev eventbus.Event
			if err := json.Unmarshal([]byte(rawStr), &ev); err != nil {
				continue
			}
			events = append(events, ev)
		}
		return events, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetEventsForEntities: %w", err)
	}

	events, _ := result.([]eventbus.Event)
	return events, nil
}

// GetEventByID retrieves a single event by its ID from Neo4j.
// Returns nil, nil if the event is not found.
func (n *Neo4jClient) GetEventByID(eventID string) (*eventbus.Event, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(
			`MATCH (e:Event {id: $id}) RETURN e.raw_data AS raw_data LIMIT 1`,
			map[string]any{"id": eventID},
		)
		if err != nil {
			return nil, err
		}
		if records.Next() {
			record := records.Record()
			rawVal, ok := record.Get("raw_data")
			if !ok || rawVal == nil {
				return nil, nil
			}
			rawStr, ok := rawVal.(string)
			if !ok {
				return nil, nil
			}
			var ev eventbus.Event
			if err := json.Unmarshal([]byte(rawStr), &ev); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event: %w", err)
			}
			return &ev, nil
		}
		return nil, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetEventByID: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	ev, _ := result.(*eventbus.Event)
	return ev, nil
}

// GetEventsByWorldID retrieves events for a specific world from Neo4j,
// ordered by timestamp descending and capped at `limit`.
func (n *Neo4jClient) GetEventsByWorldID(worldID string, limit int) ([]map[string]interface{}, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(`
MATCH (e:Event {world_id: $world_id})
RETURN e
ORDER BY e.timestamp DESC
LIMIT $limit
`, map[string]any{
			"world_id": worldID,
			"limit":    limit,
		})
		if err != nil {
			return nil, err
		}

		var events []map[string]interface{}
		for records.Next() {
			record := records.Record()
			nodeVal, ok := record.Get("e")
			if !ok {
				continue
			}
			if node, ok := nodeVal.(neo4j.Node); ok {
				events = append(events, node.Props)
			}
		}
		return events, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetEventsByWorldID: %w", err)
	}

	events, _ := result.([]map[string]interface{})
	return events, nil
}

// GetEntityByID retrieves a single entity by its ID from Neo4j.
// Returns nil, nil when no entity with that ID exists.
func (n *Neo4jClient) GetEntityByID(entityID string) (*EntityInfo, error) {
	if entityID == "" {
		return nil, fmt.Errorf("GetEntityByID: entityID cannot be empty")
	}
	cache, err := n.GetEntityCache([]string{entityID})
	if err != nil {
		return nil, err
	}
	if info, ok := cache[entityID]; ok {
		return &info, nil
	}
	return nil, nil
}

// QueryEntities returns entities matching the provided EntityQuery filter from Neo4j.
// All filter fields are optional and combined with AND logic.
// When q.IDs is non-empty only those IDs are searched; other filters are still applied.
// Returns an empty slice (not an error) when nothing matches.
func (n *Neo4jClient) QueryEntities(q EntityQuery) ([]EntityInfo, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	// Build Cypher depending on whether an ID list is supplied.
	var query string
	params := map[string]any{
		"entity_type": q.Type,
		"world_id":    q.WorldID,
		"name":        q.Name,
		"limit":       q.Limit,
	}

	if len(q.IDs) > 0 {
		query = `
MATCH (e:Entity)
WHERE e.id IN $ids
  AND ($entity_type = '' OR e.type = $entity_type)
  AND ($world_id    = '' OR e.world_id = $world_id)
  AND ($name        = '' OR toLower(e.name) CONTAINS toLower($name))
RETURN e.id AS id, e.name AS name, e.type AS type,
       e.world_id AS world_id, e.description AS description, e.payload AS payload
LIMIT $limit
`
		params["ids"] = q.IDs
	} else {
		query = `
MATCH (e:Entity)
WHERE ($entity_type = '' OR e.type = $entity_type)
  AND ($world_id    = '' OR e.world_id = $world_id)
  AND ($name        = '' OR toLower(e.name) CONTAINS toLower($name))
RETURN e.id AS id, e.name AS name, e.type AS type,
       e.world_id AS world_id, e.description AS description, e.payload AS payload
LIMIT $limit
`
	}

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, params)
		if err != nil {
			return nil, fmt.Errorf("QueryEntities query failed: %w", err)
		}

		var entities []EntityInfo
		for records.Next() {
			record := records.Record()

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
				entities = append(entities, EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				})
			}
		}
		return entities, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("QueryEntities: %w", err)
	}

	entities, _ := result.([]EntityInfo)
	if entities == nil {
		entities = []EntityInfo{}
	}
	return entities, nil
}

// Close closes the Neo4j driver.
func (n *Neo4jClient) Close() {
	_ = n.driver.Close()
}
