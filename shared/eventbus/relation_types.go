// Package eventbus provides constants for relation types used in explicit event relations.
// These relations are embedded in events to create typed edges in Neo4j.
package eventbus

// Relation type constants — semantic edge types for the knowledge graph.
const (
	// Action relations — player/NPC actions on entities
	RelActedOn   = "ACTED_ON"
	RelFound     = "FOUND"
	RelMovedTo   = "MOVED_TO"
	RelUsedItem  = "USED_ITEM"
	RelAttacked  = "ATTACKED"
	RelTalkedTo  = "TALKED_TO"

	// Ownership & location relations
	RelPossesses = "POSSESSES"
	RelLocatedIn = "LOCATED_IN"
	RelWorldOf   = "WORLD_OF"
	RelContains  = "CONTAINS"

	// Social relations
	RelAlliedWith = "ALLIED_WITH" // undirected
	RelHostileTo  = "HOSTILE_TO"

	// Spatial relations
	RelAdjacentTo = "ADJACENT_TO" // undirected
	RelConnected  = "CONNECTED_TO"
)
