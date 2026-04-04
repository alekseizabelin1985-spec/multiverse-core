// Package semanticmemory handles Neo4j integration.
package semanticmemory

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
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

	client := &Neo4jClient{driver: driver}
	// Create indexes for efficient queries
	if err := client.createIndexes(); err != nil {
		log.Printf("Warning: failed to create indexes: %v", err)
	}

	return client, nil
}

// createIndexes creates indexes for efficient graph queries
func (n *Neo4jClient) createIndexes() error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	indexes := []string{
		"CREATE INDEX event_id IF NOT EXISTS FOR (e:Event) ON (e.id)",
		"CREATE INDEX entity_id IF NOT EXISTS FOR (e:Entity) ON (e.id)",
		"CREATE INDEX event_type IF NOT EXISTS FOR (e:Event) ON (e.type)",
		"CREATE INDEX world_id IF NOT EXISTS FOR (e:Event) ON (e.world_id)",
		"CREATE INDEX event_timestamp IF NOT EXISTS FOR (e:Event) ON (e.timestamp)",
		"CREATE INDEX entity_type IF NOT EXISTS FOR (e:Entity) ON (e.type)",
		"CREATE INDEX entity_world_id IF NOT EXISTS FOR (e:Entity) ON (e.world_id)",
		"CREATE INDEX entity_x IF NOT EXISTS FOR (e:Entity) ON (e.x)",
		"CREATE INDEX entity_y IF NOT EXISTS FOR (e:Entity) ON (e.y)",
		"CREATE INDEX entity_z IF NOT EXISTS FOR (e:Entity) ON (e.z)",
	}

	var lastErr error
	for _, idx := range indexes {
		_, err := session.Run(idx, nil)
		if err != nil {
			lastErr = err
			log.Printf("Index creation failed: %v", err)
			continue
		}
	}
	return lastErr
}

// UpsertEntity creates or updates an entity node in Neo4j.
func (n *Neo4jClient) UpsertEntity(entityID, entityType string, payload map[string]any) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	// Serialize payload to JSON string (Neo4j only accepts primitives, not Maps)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Extract world_id from payload if present
	var worldID string
	if wid, ok := payload["world_id"].(string); ok {
		worldID = wid
	}

	// Extract coordinates from position field if present
	var x, y, z float64
	if pos, ok := payload["position"].(map[string]any); ok {
		if xVal, ok := pos["x"].(float64); ok {
			x = xVal
		}
		if yVal, ok := pos["y"].(float64); ok {
			y = yVal
		}
		if zVal, ok := pos["z"].(float64); ok {
			z = zVal
		}
	}

	query := `
MERGE (e:Entity {id: $entity_id})
SET e.type = $entity_type,
    e.payload = $payload_json,
    e.world_id = $world_id,
    e.x = $x,
    e.y = $y,
    e.z = $z
RETURN e
`

	// Use WriteTransaction for MERGE operation (write)
	_, err = session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"entity_id":    entityID,
			"entity_type":  entityType,
			"payload_json": string(payloadJSON),
			"world_id":     worldID,
			"x":            x,
			"y":            y,
			"z":            z,
		})
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
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

	// Use WriteTransaction for MERGE operation (write)
	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"from_id": fromID,
			"to_id":   toID,
		})
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
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
	RETURN e.id AS id, e.name AS name, e.type AS type, e.world_id AS world_id, e.description AS description, e.payload AS payload, e.x AS x, e.y AS y, e.z AS z
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
			var x, y, z float64

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
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}

			if id != "" {
				entityInfo := EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
				// Set coordinates if any coordinate is present
				if x != 0 || y != 0 || z != 0 {
					entityInfo.Coordinates = &Coordinates{X: x, Y: y, Z: z}
				}
				cache[id] = entityInfo
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
// Поддерживает как старые ключи (entity_id, source_id), так и новые вложенные структуры
// (entity: {id}, target: {id}, source: {id}, world: {id}).
func (n *Neo4jClient) LinkEventToEntities(eventID string, payload map[string]interface{}) error {
	entityIDs := extractEntityIDsFromPayload(payload)

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
	// Use transaction to guarantee the data is committed
	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"event_id":   eventID,
			"entity_ids": entityIDs,
		})
		if runErr != nil {
			return nil, runErr
		}
		// Consume result to ensure query executes and transaction commits
		_, consumeErr := result.Consume()
		return nil, consumeErr
	})
	if err != nil {
		return fmt.Errorf("LinkEventToEntities: %w", err)
	}
	return nil
}

// extractEntityIDsFromPayload извлекает все entity ID из payload события.
// Поддерживает старые ключи (entity_id, source_id) и новые вложенные структуры
// (entity: {id}, target: {id}, source: {id}, world: {id}).
func extractEntityIDsFromPayload(payload map[string]interface{}) []string {
	seen := make(map[string]bool)
	var entityIDs []string

	// Старые ключи для обратной совместимости
	oldKeys := []string{
		"entity_id", "source_id", "target_id",
		"character_id", "player_id", "npc_id",
		"actor_id", "subject_id", "object_id", "world_id",
	}
	for _, key := range oldKeys {
		if val, ok := payload[key].(string); ok && val != "" {
			realID := normalizeEntityID(val)
			if !seen[realID] {
				seen[realID] = true
				entityIDs = append(entityIDs, realID)
			}
		}
		if val, ok := payload[key].([]string); ok {
			for _, v := range val {
				if v != "" {
					realID := normalizeEntityID(v)
					if !seen[realID] {
						seen[realID] = true
						entityIDs = append(entityIDs, realID)
					}
				}
			}
		}
	}

	// Новые вложенные структуры: entity, target, source, world
	newKeys := []string{"entity", "target", "source", "world"}
	for _, key := range newKeys {
		if nested, ok := payload[key].(map[string]interface{}); ok {
			if id, ok := nested["id"].(string); ok && id != "" {
				realID := normalizeEntityID(id)
				if !seen[realID] {
					seen[realID] = true
					entityIDs = append(entityIDs, realID)
				}
			}
		}
	}

	return entityIDs
}

// normalizeEntityID нормализует ID сущности, убирая префикс типа если он совпадает с world_id.
// Например: "world:world-xxx" → "world-xxx"
// Это предотвращает дублирование сущностей в графе.
func normalizeEntityID(entityID string) string {
	// Если ID имеет формат "world:world-xxx" → "world-xxx"
	if strings.HasPrefix(entityID, "world:") {
		realID := strings.TrimPrefix(entityID, "world:")
		return realID
	}
	// Для других префиксов (player:p1, npc:n1) оставляем как есть
	return entityID
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
		"x":           q.X,
		"y":           q.Y,
		"z":           q.Z,
	}

	// Helper to check if coordinate filter is set
	hasCoordFilter := q.X != nil || q.Y != nil || q.Z != nil

	if len(q.IDs) > 0 {
		if hasCoordFilter {
			query = `
MATCH (e:Entity)
WHERE e.id IN $ids
  AND ($entity_type = '' OR e.type = $entity_type)
  AND ($world_id    = '' OR e.world_id = $world_id)
  AND ($name        = '' OR toLower(e.name) CONTAINS toLower($name))
  AND ($x IS NULL   OR e.x = $x)
  AND ($y IS NULL   OR e.y = $y)
  AND ($z IS NULL   OR e.z = $z)
RETURN e.id AS id, e.name AS name, e.type AS type,
       e.world_id AS world_id, e.description AS description, e.payload AS payload,
       e.x AS x, e.y AS y, e.z AS z
LIMIT $limit
`
		} else {
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
		}
		params["ids"] = q.IDs
	} else {
		if hasCoordFilter {
			query = `
MATCH (e:Entity)
WHERE ($entity_type = '' OR e.type = $entity_type)
  AND ($world_id    = '' OR e.world_id = $world_id)
  AND ($name        = '' OR toLower(e.name) CONTAINS toLower($name))
  AND ($x IS NULL   OR e.x = $x)
  AND ($y IS NULL   OR e.y = $y)
  AND ($z IS NULL   OR e.z = $z)
RETURN e.id AS id, e.name AS name, e.type AS type,
       e.world_id AS world_id, e.description AS description, e.payload AS payload,
       e.x AS x, e.y AS y, e.z AS z
LIMIT $limit
`
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
			var x, y, z float64

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
			// Check if x, y, z are present in the result (when coordinate filter is used)
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}

			if id != "" {
				entityInfo := EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
				// Set coordinates if any coordinate is present
				if x != 0 || y != 0 || z != 0 {
					entityInfo.Coordinates = &Coordinates{X: x, Y: y, Z: z}
				}
				entities = append(entities, entityInfo)
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

// GetEventsByTypes retrieves events matching any of the given types, optionally filtered by world_id.
// Returns full eventbus.Event structs deserialized from raw_data.
func (n *Neo4jClient) GetEventsByTypes(eventTypes []string, worldID string, limit int) ([]eventbus.Event, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
MATCH (e:Event)
WHERE e.type IN $types AND ($world_id = '' OR e.world_id = $world_id)
RETURN e.raw_data AS raw_data
ORDER BY e.timestamp DESC
LIMIT $limit
`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"types":    eventTypes,
			"world_id": worldID,
			"limit":    limit,
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
		return nil, err
	}
	events, _ := result.([]eventbus.Event)
	return events, nil
}

// Close closes the Neo4j driver.
func (n *Neo4jClient) Close() {
	_ = n.driver.Close()
}

// GetEventsByWorldAndType retrieves events for a specific world and optionally by event type.
// Returns full eventbus.Event structs deserialized from raw_data.
func (n *Neo4jClient) GetEventsByWorldAndType(worldID, eventType string, limit int) ([]eventbus.Event, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	var query string
	if eventType != "" {
		query = `MATCH (e:Event) WHERE e.world_id = $world_id AND e.type = $event_type RETURN e.raw_data AS raw_data ORDER BY e.timestamp DESC LIMIT $limit`
	} else {
		query = `MATCH (e:Event) WHERE e.world_id = $world_id RETURN e.raw_data AS raw_data ORDER BY e.timestamp DESC LIMIT $limit`
	}

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{"world_id": worldID, "event_type": eventType, "limit": limit})
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
		return nil, err
	}
	events, _ := result.([]eventbus.Event)
	return events, nil
}

// GetEventsByTypeNeo4j retrieves events by type from Neo4j graph nodes.
// Returns full eventbus.Event structs deserialized from raw_data.
func (n *Neo4jClient) GetEventsByTypeNeo4j(eventType string, limit int) ([]eventbus.Event, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `MATCH (e:Event) WHERE e.type = $event_type RETURN e.raw_data AS raw_data ORDER BY e.timestamp DESC LIMIT $limit`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{"event_type": eventType, "limit": limit})
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
		return nil, err
	}
	events, _ := result.([]eventbus.Event)
	return events, nil
}

// GetEventsByEntity retrieves events related to a specific entity via graph relationships.
// Returns full eventbus.Event structs deserialized from raw_data.
func (n *Neo4jClient) GetEventsByEntity(entityID string, limit int) ([]eventbus.Event, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `MATCH (e:Event)-[:RELATED_TO]->(en:Entity) WHERE en.id = $entity_id RETURN e.raw_data AS raw_data ORDER BY e.timestamp DESC LIMIT $limit`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{"entity_id": entityID, "limit": limit})
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
		return nil, err
	}
	events, _ := result.([]eventbus.Event)
	return events, nil
}

// GetEntitiesByType retrieves entities by type from Neo4j graph.
func (n *Neo4jClient) GetEntitiesByType(entityType, worldID string, limit int) ([]EntityInfo, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `MATCH (e:Entity) WHERE ($entity_type = '' OR e.type = $entity_type) AND ($world_id = '' OR e.world_id = $world_id) RETURN e.id AS id, e.type AS type, e.payload AS payload, e.x AS x, e.y AS y, e.z AS z LIMIT $limit`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{"entity_type": entityType, "world_id": worldID, "limit": limit})
		if err != nil {
			return nil, err
		}
		var entities []EntityInfo
		for records.Next() {
			record := records.Record()
			var id, entityType string
			var payload map[string]any
			var x, y, z float64
			if val, ok := record.Get("id"); ok && val != nil {
				if s, ok := val.(string); ok {
					id = s
				}
			}
			if val, ok := record.Get("type"); ok && val != nil {
				if s, ok := val.(string); ok {
					entityType = s
				}
			}
			if val, ok := record.Get("payload"); ok && val != nil {
				if p, ok := val.(map[string]any); ok {
					payload = p
				}
			}
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}
			if id != "" {
				entityInfo := EntityInfo{ID: id, Type: entityType, Payload: payload}
				if x != 0 || y != 0 || z != 0 {
					entityInfo.Coordinates = &Coordinates{X: x, Y: y, Z: z}
				}
				entities = append(entities, entityInfo)
			}
		}
		return entities, records.Err()
	})
	if err != nil {
		return nil, err
	}
	entities, _ := result.([]EntityInfo)
	return entities, nil
}

// SaveEventAsGraph saves an event as a graph node with relationships to entities
func (n *Neo4jClient) SaveEventAsGraph(ev eventbus.Event, payloadJSON string) error {
	// Serialize full event as raw_data for complete retrieval
	rawData, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal event to raw_data: %w", err)
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	_, err = session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		query := `
MERGE (e:Event {id: $event_id})
SET e.type = $event_type,
    e.timestamp = $timestamp,
    e.source = $source,
    e.world_id = $world_id,
    e.payload_json = $payload_json,
    e.raw_data = $raw_data
`
		params := map[string]any{
			"event_id":     ev.ID,
			"event_type":   ev.Type,
			"timestamp":    ev.Timestamp,
			"source":       ev.Source,
			"world_id":     eventbus.GetWorldIDFromEvent(ev),
			"payload_json": payloadJSON,
			"raw_data":     string(rawData),
		}

		// Add scope_id if present
		if scope := eventbus.GetScopeFromEvent(ev); scope != nil {
			params["scope_id"] = scope.ID
			query = `
MERGE (e:Event {id: $event_id})
SET e.type = $event_type,
    e.timestamp = $timestamp,
    e.source = $source,
    e.world_id = $world_id,
    e.scope_id = $scope_id,
    e.payload_json = $payload_json,
    e.raw_data = $raw_data
`
		}

		result, runErr := tx.Run(query, params)
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
	})
	if err != nil {
		return err
	}

	// Link event to entities in payload
	if linkErr := n.LinkEventToEntities(ev.ID, ev.Payload); linkErr != nil {
		log.Printf("Warning: failed to link event %s to entities: %v", ev.ID, linkErr)
	}

	return nil
}

// extractEntitiesFromPayload extracts entity IDs from event payload
// It handles various common patterns in event payloads
func extractEntitiesFromPayload(payload map[string]interface{}) []string {
	entityKeys := []string{
		"entity_id", "source_id", "target_id",
		"player_id", "character_id", "npc_id",
		"actor_id", "object_id", "item_id", "actor_id", "subject_id", "focus_entities",
	}

	var entityIDs []string
	seen := make(map[string]bool)

	for _, key := range entityKeys {
		if val, ok := payload[key]; ok {
			switch v := val.(type) {
			case string:
				if v != "" && !seen[v] {
					entityIDs = append(entityIDs, v)
					seen[v] = true
				}
			case []interface{}:
				for _, item := range v {
					if s, ok := item.(string); ok && s != "" && !seen[s] {
						entityIDs = append(entityIDs, s)
						seen[s] = true
					}
				}
			case map[string]interface{}:
				// Handle nested structures like { "player": { "id": "xxx" } }
				if nested, ok := v["id"]; ok {
					if s, ok := nested.(string); ok && s != "" && !seen[s] {
						entityIDs = append(entityIDs, s)
						seen[s] = true
					}
				}
			}
		}
	}

	// Also check inventory array
	if inv, ok := payload["inventory"]; ok {
		switch v := inv.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" && !seen[s] {
					entityIDs = append(entityIDs, s)
					seen[s] = true
				}
			}
		case []string:
			for _, s := range v {
				if s != "" && !seen[s] {
					entityIDs = append(entityIDs, s)
					seen[s] = true
				}
			}
		}
	}

	return entityIDs
}

// ExtractNestedEntityIDs extracts entity IDs from nested payload structures
// Handles patterns like payload.player.id, payload.target.id, etc.
func ExtractNestedEntityIDs(payload map[string]interface{}) map[string]string {
	ids := make(map[string]string)

	// Direct string fields that might be entity IDs
	directFields := []string{"player_id", "target_id", "source_id", "entity_id",
		"character_id", "npc_id", "item_id", "object_id", "subject_id", "focus_entities"}
	for _, field := range directFields {
		if val, ok := payload[field]; ok {
			if s, ok := val.(string); ok && s != "" {
				ids[field] = s
			}
		}
	}

	// Nested structures like { "player": { "id": "xxx", "name": "yyy" } }
	nestedFields := []string{"player", "target", "source", "entity", "actor",
		"character", "npc", "item", "object", "metadata"}
	for _, field := range nestedFields {
		if nested, ok := payload[field].(map[string]interface{}); ok {
			if id, ok := nested["id"].(string); ok && id != "" {
				key := fmt.Sprintf("%s.id", field)
				ids[key] = id
			}
			if name, ok := nested["name"].(string); ok && name != "" {
				key := fmt.Sprintf("%s.name", field)
				ids[key] = name
			}
		}
	}

	// Array of entities
	if entities, ok := payload["entities"].([]interface{}); ok {
		for i, entity := range entities {
			if ent, ok := entity.(map[string]interface{}); ok {
				if id, ok := ent["id"].(string); ok && id != "" {
					key := fmt.Sprintf("entities[%d].id", i)
					ids[key] = id
				}
			}
		}
	}

	return ids
}

// GetWorldByID retrieves world entity from Neo4j by ID
func (n *Neo4jClient) GetWorldByID(worldID string) (*WorldInfo, error) {
	if n.driver == nil {
		return nil, fmt.Errorf("neo4j driver not initialized")
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	var worldInfo *WorldInfo
	query := `
	MATCH (w:World {id: $world_id})
	RETURN w.id AS id, w.type AS type, w.payload AS payload, w.x AS x, w.y AS y, w.z AS z
	`

	_, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"world_id": worldID,
		})
		if err != nil {
			return nil, err
		}

		if records.Next() {
			record := records.Record()
			var id, entityType string
			var payload map[string]interface{}
			var x, y, z float64

			if val, ok := record.Get("id"); ok && val != nil {
				if s, ok := val.(string); ok {
					id = s
				}
			}
			if val, ok := record.Get("type"); ok && val != nil {
				if s, ok := val.(string); ok {
					entityType = s
				}
			}
			if val, ok := record.Get("payload"); ok && val != nil {
				if p, ok := val.(map[string]interface{}); ok {
					payload = p
				}
			}
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}

			worldInfo = &WorldInfo{
				ID:          id,
				Type:        entityType,
				Payload:     payload,
				Coordinates: &Coordinates{X: x, Y: y, Z: z},
			}
		}

		return nil, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetWorldByID: %w", err)
	}

	return worldInfo, nil
}

// GetWorldByEvent retrieves world data from world creation/update events in Neo4j
func (n *Neo4jClient) GetWorldByEvent(worldID string) (*WorldInfo, error) {
	if n.driver == nil {
		return nil, fmt.Errorf("neo4j driver not initialized")
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	var worldInfo *WorldInfo
	query := `
	MATCH (w:Event)
	WHERE w.world_id = $world_id AND w.type IN ['entity.created', 'entity.updated']
	ORDER BY w.timestamp DESC
	LIMIT 1
	RETURN w.raw_data AS raw_data
	`

	_, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"world_id": worldID,
		})
		if err != nil {
			return nil, err
		}

		if records.Next() {
			record := records.Record()
			if val, ok := record.Get("raw_data"); ok && val != nil {
				if rawStr, ok := val.(string); ok {
					var ev eventbus.Event
					if err := json.Unmarshal([]byte(rawStr), &ev); err != nil {
						return nil, err
					}

					// Extract entity_id and payload from world event
					entityID, _ := ev.Payload["entity_id"].(string)
					payload, _ := ev.Payload["payload"].(map[string]interface{})
					// Use world_id as entity_id for consistency
					if entityID == "" {
						entityID = worldID
					}

					// Try to get coordinates from position in payload
					var x, y, z float64
					if pos, ok := payload["position"].(map[string]interface{}); ok {
						if xVal, ok := pos["x"].(float64); ok {
							x = xVal
						}
						if yVal, ok := pos["y"].(float64); ok {
							y = yVal
						}
						if zVal, ok := pos["z"].(float64); ok {
							z = zVal
						}
					}

					worldInfo = &WorldInfo{
						ID:          entityID,
						Type:        "world",
						Payload:     payload,
						Coordinates: &Coordinates{X: x, Y: y, Z: z},
					}
				}
			}
		}

		return nil, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetWorldByEvent: %w", err)
	}

	return worldInfo, nil
}

// GetWorldContext retrieves world context from world events
func (n *Neo4jClient) GetWorldContext(worldID string) (string, error) {
	worldInfo, err := n.GetWorldByEvent(worldID)
	if err != nil {
		return "", fmt.Errorf("failed to get world info: %w", err)
	}

	if worldInfo == nil || worldInfo.Payload == nil {
		return "", nil
	}

	// Build context string from world payload
	var parts []string
	parts = append(parts, fmt.Sprintf("World ID: %s", worldInfo.ID))

	if core, ok := worldInfo.Payload["core"].(string); ok {
		parts = append(parts, fmt.Sprintf("Core: %s", core))
	}
	if era, ok := worldInfo.Payload["era"].(string); ok {
		parts = append(parts, fmt.Sprintf("Era: %s", era))
	}
	if mode, ok := worldInfo.Payload["mode"].(string); ok {
		parts = append(parts, fmt.Sprintf("Mode: %s", mode))
	}
	if theme, ok := worldInfo.Payload["theme"].(string); ok {
		parts = append(parts, fmt.Sprintf("Theme: %s", theme))
	}
	if seed, ok := worldInfo.Payload["seed"].(string); ok {
		parts = append(parts, fmt.Sprintf("Seed: %s", seed))
	}

	// Add unique_traits if present
	if traits, ok := worldInfo.Payload["unique_traits"]; ok {
		parts = append(parts, "Unique Traits:")
		if traitSlice, ok := traits.([]interface{}); ok {
			for _, t := range traitSlice {
				parts = append(parts, fmt.Sprintf("  - %v", t))
			}
		}
	}

	return strings.Join(parts, "\n"), nil
}

// GetRegionsByWorldID retrieves all region entities for a given world
func (n *Neo4jClient) GetRegionsByWorldID(worldID string, limit int) ([]EntityInfo, error) {
	if n.driver == nil {
		return nil, fmt.Errorf("neo4j driver not initialized")
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
	MATCH (r:Region)
	WHERE r.world_id = $world_id
	RETURN r.id AS id, r.name AS name, r.type AS type, r.world_id AS world_id, r.description AS description, r.payload AS payload, r.x AS x, r.y AS y, r.z AS z
	`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"world_id": worldID,
		})
		if err != nil {
			return nil, err
		}

		regions := make([]EntityInfo, 0)
		for records.Next() {
			record := records.Record()
			var id, name, entityType, worldID, description string
			var payload map[string]interface{}
			var x, y, z float64

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
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}

			if id != "" {
				entityInfo := EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
				if x != 0 || y != 0 || z != 0 {
					entityInfo.Coordinates = &Coordinates{X: x, Y: y, Z: z}
				}
				regions = append(regions, entityInfo)
			}
		}

		return regions, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetRegionsByWorldID: %w", err)
	}

	regions, ok := result.([]EntityInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return regions, nil
}

// GetLocationsByWorldID retrieves all location entities for a given world
func (n *Neo4jClient) GetLocationsByWorldID(worldID string, limit int) ([]EntityInfo, error) {
	if n.driver == nil {
		return nil, fmt.Errorf("neo4j driver not initialized")
	}

	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
	MATCH (l:Location)
	WHERE l.world_id = $world_id
	RETURN l.id AS id, l.name AS name, l.type AS type, l.world_id AS world_id, l.description AS description, l.payload AS payload, l.x AS x, l.y AS y, l.z AS z
	`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		records, err := tx.Run(query, map[string]any{
			"world_id": worldID,
		})
		if err != nil {
			return nil, err
		}

		locations := make([]EntityInfo, 0)
		for records.Next() {
			record := records.Record()
			var id, name, entityType, worldID, description string
			var payload map[string]interface{}
			var x, y, z float64

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
			if val, ok := record.Get("x"); ok && val != nil {
				if xVal, ok := val.(float64); ok {
					x = xVal
				}
			}
			if val, ok := record.Get("y"); ok && val != nil {
				if yVal, ok := val.(float64); ok {
					y = yVal
				}
			}
			if val, ok := record.Get("z"); ok && val != nil {
				if zVal, ok := val.(float64); ok {
					z = zVal
				}
			}

			if id != "" {
				entityInfo := EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
				if x != 0 || y != 0 || z != 0 {
					entityInfo.Coordinates = &Coordinates{X: x, Y: y, Z: z}
				}
				locations = append(locations, entityInfo)
			}
		}

		return locations, records.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("GetLocationsByWorldID: %w", err)
	}

	locations, ok := result.([]EntityInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return locations, nil
}

// CreateWorld creates a world node in Neo4j
func (n *Neo4jClient) CreateWorld(worldID string, worldData map[string]interface{}) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	payloadJSON, err := json.Marshal(worldData)
	if err != nil {
		return fmt.Errorf("failed to marshal world data: %w", err)
	}

	var x, y, z float64
	if pos, ok := worldData["position"].(map[string]interface{}); ok {
		if xVal, ok := pos["x"].(float64); ok {
			x = xVal
		}
		if yVal, ok := pos["y"].(float64); ok {
			y = yVal
		}
		if zVal, ok := pos["z"].(float64); ok {
			z = zVal
		}
	}

	query := `
	MERGE (w:World {id: $world_id})
	SET w.type = $type,
	    w.payload = $payload_json,
	    w.x = $x,
	    w.y = $y,
	    w.z = $z
	RETURN w
	`

	_, err = session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"world_id":     worldID,
			"type":         "world",
			"payload_json": string(payloadJSON),
			"x":            x,
			"y":            y,
			"z":            z,
		})
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
	})

	return err
}

// CreateWorldRelationship creates a WORLD_OF relationship between a region/location and a world
func (n *Neo4jClient) CreateWorldRelationship(regionOrLocationID, worldID string) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
	MATCH (r:Region {id: $region_id})
	MATCH (w:World {id: $world_id})
	MERGE (r)-[rel:WORLD_OF]->(w)
	RETURN rel
	`

	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"region_id": regionOrLocationID,
			"world_id":  worldID,
		})
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
	})

	return err
}

// =============================================================================
// Explicit Relations — Этап 2: Явные связи для knowledge graph
// =============================================================================

// CreateRelation создаёт семантическую связь между двумя Entity в графе.
// Использует MERGE для идемпотентности. Directed=false создаёт ненаправленную связь.
// Metadata сериализуется в свойства связи.
func (n *Neo4jClient) CreateRelation(fromID, toID, relType string, directed bool, metadata map[string]any) error {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	arrow := "->"
	if !directed {
		arrow = "-"
	}

	// Безопасный relType: только alphanumeric + underscore
	safeRelType := sanitizeRelType(relType)

	query := fmt.Sprintf(`
		MATCH (a {id: $from_id})
		MATCH (b {id: $to_id})
		MERGE (a)-[r:%s]%s(b)
		SET r += $metadata
		RETURN r
	`, safeRelType, arrow)

	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
		result, runErr := tx.Run(query, map[string]any{
			"from_id":  fromID,
			"to_id":    toID,
			"metadata": metadata,
		})
		if runErr != nil {
			return nil, runErr
		}
		_, consumeErr := result.Consume()
		return nil, consumeErr
	})
	return err
}

// EntityExists проверяет существует ли Entity с указанным ID.
func (n *Neo4jClient) EntityExists(entityID string) (bool, error) {
	session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close()

	query := `
		MATCH (e:Entity {id: $entity_id})
		RETURN count(e) > 0 AS exists
	`

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (any, error) {
		res, runErr := tx.Run(query, map[string]any{
			"entity_id": entityID,
		})
		if runErr != nil {
			return false, runErr
		}
		if res.Next() {
			return res.Record().Values[0].(bool), nil
		}
		return false, nil
	})
	if err != nil {
		return false, err
	}
	exists, _ := result.(bool)
	return exists, nil
}

// EnsureEntity создаёт Entity-заглушку если её ещё нет.
// Если Entity уже существует — ничего не делает.
func (n *Neo4jClient) EnsureEntity(entityID, entityType, worldID string, payload map[string]any) error {
	exists, err := n.EntityExists(entityID)
	if err != nil {
		return fmt.Errorf("check entity existence: %w", err)
	}
	if exists {
		return nil
	}

	// Создаём сущность-заглушку
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["stub"] = true
	payload["world_id"] = worldID

	return n.UpsertEntity(entityID, entityType, payload)
}

// sanitizeRelType очищает тип связи от потенциально опасных символов.
// Допускаются только alphanumeric и underscore.
func sanitizeRelType(relType string) string {
	var result []rune
	for _, r := range relType {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result = append(result, r)
		}
	}
	if len(result) == 0 {
		return "RELATED"
	}
	return string(result)
}
