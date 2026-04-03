# 🔀 Migration Guide: Hierarchical Event Architecture

> 📅 **Last Updated**: 2026-02-22  
> 🎯 **Status**: ✅ Complete — All services migrated with backward compatibility

---

## 📋 Overview

This guide documents the migration from **flat event keys** to **hierarchical dot-notation paths** across the Multiverse-Core project.

### Why This Change?

| Problem with Flat Keys | Solution with Hierarchical Paths |
|------------------------|----------------------------------|
| ❌ Ambiguous semantics: `entity_id` could mean player, NPC, or item | ✅ Clear structure: `entity:{id:"x", type:"player", name:"Вася"}` |
| ❌ Hard to extend: adding nested data requires new top-level keys | ✅ Natural nesting: `entity.stats.hp`, `entity.inventory[0].durability` |
| ❌ LLM confusion: flat keys provide poor context for AI generation | ✅ Semantic clarity: hierarchical JSON improves prompt understanding |
| ❌ Manual parsing: type assertions everywhere, risk of panics | ✅ Type-safe accessors: `GetString()`, `GetInt()`, etc. with `(value, ok)` pattern |
| ❌ Inconsistent across services: each service parsed events differently | ✅ Universal `jsonpath` package: one API for all nested data access |

---

## 🔄 Before vs After: Event Structure

### ❌ OLD: Flat Format (Deprecated but Supported)

```json
{
  "event_id": "evt-abc123",
  "event_type": "player.entered_region",
  "entity_id": "player-123",
  "entity_type": "player",
  "entity_name": "Вася",
  "world_id": "world-abc",
  "scope_id": "city-xyz",
  "target_id": "region-456",
  "payload": {
    "description": "Вася вошёл в Тёмный лес",
    "weather": "пасмурно"
  }
}
```

### ✅ NEW: Hierarchical Format (Preferred)

```json
{
  "event_id": "evt-abc123",
  "event_type": "player.entered_region",
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "world": {
    "id": "world-abc"
  },
  "scope": {
    "id": "city-xyz",
    "type": "city"
  },
  "target": {
    "entity": {
      "id": "region-456",
      "type": "region",
      "name": "Тёмный лес"
    }
  },
  "payload": {
    "description": "Вася вошёл в Тёмный лес",
    "weather": "пасмурно"
  }
}
```

> ♻️ **Backward Compatibility**: Both formats work simultaneously. Old events are automatically handled via fallback logic.

---

## 📦 Universal Access: `shared/jsonpath` Package

The `jsonpath` package provides type-safe, universal access to ANY nested `map[string]any` data structure.

### Installation & Import

```go
import "multiverse-core.io/shared/jsonpath"
```

### Creating an Accessor

```go
// From event payload:
pa := event.Path()  // *jsonpath.Accessor (via eventbus.Event.Path())

// From any map[string]any:
pa := jsonpath.New(anyData)
```

### Reading Data: Type-Safe Getters

| Method | Returns | Example Path | Use Case |
|--------|---------|-------------|----------|
| `GetString(path)` | `(string, bool)` | `"entity.id"` | IDs, names, text |
| `GetInt(path)` | `(int, bool)` | `"entity.stats.level"` | Levels, counts, scores |
| `GetFloat(path)` | `(float64, bool)` | `"weather.temp.value"` | Coordinates, metrics |
| `GetBool(path)` | `(bool, bool)` | `"entity.active"` | Flags, states |
| `GetMap(path)` | `(map[string]any, bool)` | `"entity.metadata"` | Nested objects |
| `GetSlice(path)` | `([]any, bool)` | `"entity.inventory"` | Arrays/lists |
| `GetAny(path)` | `(any, bool)` | `"entity"` | Unknown/dynamic types |

**All getters return `(value, ok)` — check `ok` before using the value!**

### Advanced Access Patterns

```go
pa := event.Path()

// ✅ Fallback chain: try new structure, then old, then default
entityID, ok := pa.GetString("entity.id")
if !ok {
    entityID, _ = pa.GetString("entity_id")  // fallback to flat key
}
if entityID == "" {
    entityID = "unknown"  // final default
}

// ✅ Array access by index
firstItem, _ := pa.GetString("entity.inventory[0].name")
secondTag, _ := pa.GetString("entity.tags[1]")

// ✅ Quick existence check (faster than Get*)
if pa.Has("quest.objectives") {
    // Process quest...
}

// ✅ Debug: list all available paths
for _, path := range pa.GetAllPaths() {
    fmt.Printf("Available: %s\n", path)
    // Output: entity, entity.id, entity.type, entity.stats, entity.stats.hp, ...
}
```

### Writing Data: Builder Pattern + Dot-Notation

```go
// 1. Create payload with hierarchical structure
payload := eventbus.NewEventPayload().
    WithEntity("player-123", "player", "Вася").
    WithScope("city-xyz", "city").      // solo/group/city/region/quest
    WithWorld("world-abc")

// 2. Add custom fields with dot-notation
eventbus.SetNested(payload.GetCustom(), "action", "talk")
eventbus.SetNested(payload.GetCustom(), "dialogue.text", "Hello!")
eventbus.SetNested(payload.GetCustom(), "dialogue.options", []string{"Ask", "Leave"})

// 3. Optional: explicitly add hierarchical paths for LLM clarity
eventbus.SetNested(payload.GetCustom(), "entity.id", "player-123")
eventbus.SetNested(payload.GetCustom(), "world.id", "world-abc")

// 4. Create and publish event
event := eventbus.NewStructuredEvent(
    "player.talked",
    "entity-actor",
    "world-abc",
    payload,
)
bus.Publish(ctx, eventbus.TopicWorldEvents, event)
```

---

## 🔄 Migration Patterns: Service-by-Service

### Pattern 1: Reading Event Data (Consumer Services)

```go
// ❌ OLD: Direct map access (fragile)
func handlePlayerAction(event eventbus.Event) {
    entityID := event.Payload["entity_id"].(string)  // PANIC if missing/wrong type!
    worldID := event.WorldID                          // Top-level field only
    scopeID := *event.ScopeID                         // Pointer, may be nil
    // ...
}

// ✅ NEW: Universal access with fallback (safe)
func handlePlayerAction(event eventbus.Event) {
    pa := event.Path()
    
    // Entity: new structure → old structure → empty
    entityID, _ := pa.GetString("entity.id")
    if entityID == "" {
        entityID, _ = pa.GetString("entity_id")
    }
    
    // World: unified helper handles both structures
    worldID := eventbus.GetWorldIDFromEvent(event)
    
    // Scope: unified helper + type info
    scope := eventbus.GetScopeFromEvent(event)
    if scope != nil {
        log.Printf("Scope: %s (%s)", scope.ID, scope.Type)
    }
    
    // Custom fields: dot-notation access
    action, _ := pa.GetString("action")
    skillID, _ := pa.GetString("skill.id")
    
    // ... rest of logic
}
```

### Pattern 2: Creating Events (Producer Services)

```go
// ❌ OLD: Manual map construction (error-prone)
func publishEntityCreated(bus *eventbus.EventBus, worldID, entityID, entityType string) {
    event := eventbus.Event{
        EventID:   uuid.New().String(),
        EventType: "entity.created",
        WorldID:   worldID,  // Top-level only
        Payload: map[string]interface{}{
            "entity_id":   entityID,
            "entity_type": entityType,
            // Easy to forget fields or use wrong types
        },
    }
    bus.Publish(ctx, eventbus.TopicSystemEvents, event)
}

// ✅ NEW: Builder pattern (type-safe, consistent)
func publishEntityCreated(bus *eventbus.EventBus, worldID, entityID, entityType string) {
    payload := eventbus.NewEventPayload().
        WithEntity(entityID, entityType, "").
        WithWorld(worldID)
    
    // Optional: add hierarchical paths for LLM
    eventbus.SetNested(payload.GetCustom(), "entity.id", entityID)
    eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
    
    event := eventbus.NewStructuredEvent(
        "entity.created",
        "entity-manager",
        worldID,
        payload,
    )
    bus.Publish(ctx, eventbus.TopicSystemEvents, event)
}
```

### Pattern 3: LLM Prompt Generation (Narrative Orchestrator)

```go
// ❌ OLD: Flat-key schema in prompts
systemPrompt := `{
  "world_id": "xxx",
  "scope_id": "yyy",
  "entity_id": "zzz"
}`

// ✅ NEW: Hierarchical schema with examples
systemPrompt := `{
  "world": {"id": "xxx"},
  "scope": {"id": "yyy", "type": "city"},
  "entity": {"id": "zzz", "type": "player", "name": "Вася"},
  "target": {"entity": {"id": "aaa", "type": "npc"}}
}`

// With concrete examples for better AI understanding:
examples := `
Example 1 (player entered region):
{
  "event_type": "player.entered_region",
  "world": {"id": "world-abc"},
  "scope": {"id": "solo-xyz", "type": "solo"},
  "entity": {"id": "player-123", "type": "player", "name": "Вася"},
  "target": {"entity": {"id": "region-456", "type": "region", "name": "Тёмный лес"}},
  "payload": {"description": "...", "weather": "пасмурно"}
}
`
```

---

## ♻️ Backward Compatibility Strategy

### How Fallback Works

```go
// eventbus.ExtractEntityID() — supports BOTH structures:
func ExtractEntityID(payload map[string]any) *EntityInfo {
    // 1. Try NEW structure first
    if entity, ok := payload["entity"].(map[string]any); ok {
        if id, ok := entity["id"].(string); ok {
            return &EntityInfo{ID: id, Type: entity["type"], ...}
        }
    }
    // 2. Fallback to OLD flat keys
    if id, ok := payload["entity_id"].(string); ok {
        return &EntityInfo{ID: id, Type: payload["entity_type"], ...}
    }
    return nil
}
```

### Migration Timeline

```
Phase 1 (✅ Complete): Foundation
├─ ✅ Create shared/jsonpath package
├─ ✅ Update eventbus with helpers + type aliases
├─ ✅ Update narrative-orchestrator prompts
├─ ✅ Migrate semantic-memory indexer

Phase 2 (✅ Complete): Service Migration
├─ ✅ entity-actor: 7 methods updated
├─ ✅ cultivation-module: 5 methods updated  
├─ ✅ city-governor: 2 methods updated
├─ ✅ ban-of-world: 2 methods updated
├─ ✅ entity-manager: template + 1 method

Phase 3 (🔄 Future): Deprecation & Cleanup
├─ 🔄 Add deprecation warnings for flat-key usage in logs
├─ 🔄 Add metrics: % events in new vs old format
├─ 🔄 After 100% new-format adoption: remove flat-key fallback code
├─ 🔄 Remove deprecated Event.WorldID/ScopeID fields
```

### Testing Backward Compatibility

```bash
# 1. Run existing tests — all should pass with old-format events
go test ./services/... -v

# 2. Send mixed-format test events via Redpanda:
# Old format:
echo '{"event_type":"test","entity_id":"p1","world_id":"w1"}' | \
  rpk topic produce world_events

# New format:  
echo '{"event_type":"test","entity":{"id":"p1"},"world":{"id":"w1"}}' | \
  rpk topic produce world_events

# 3. Verify both are processed correctly in logs/monitoring
```

---

## 🧪 Testing & Validation

### Unit Tests

```go
// shared/jsonpath/accessor_test.go — 20+ test cases
func TestAccessor_GetString(t *testing.T) {
    data := map[string]any{
        "entity": map[string]any{"id": "p1"},
        "flat_key": "value",
    }
    acc := jsonpath.New(data)
    
    id, ok := acc.GetString("entity.id")
    assert.True(t, ok)
    assert.Equal(t, "p1", id)
    
    flat, ok := acc.GetString("flat_key")
    assert.True(t, ok)
    assert.Equal(t, "value", flat)
}
```

### Integration Tests

```bash
# Test eventbus helpers with both formats:
go test ./shared/eventbus -run TestExtractEntityID -v
go test ./shared/eventbus -run TestGetWorldIDFromEvent -v

# Test full service flow:
go test ./services/entity-actor -v
```

### Manual Validation

```bash
# 1. Start infrastructure:
docker-compose up redpanda minio chromadb

# 2. Run a service locally:
cd services/entity-actor
go run cmd/main.go

# 3. Publish test events (old and new format) via rpk/kafkacat
# 4. Check logs for correct processing of both formats
```

---

## 📚 Additional Resources

| Resource | Description |
|----------|-------------|
| [`shared/jsonpath/README.md`](../shared/jsonpath/README.md) | Full API reference + examples |
| [`shared/eventbus/README.md`](../shared/eventbus/README.md) | Event patterns + migration guide |
| [`shared/eventbus/MIGRATION.md`](../shared/eventbus/MIGRATION.md) | Detailed event migration steps |
| [`services/narrative-orchestrator/narrativeorchestrator/prompt_builder.go`](../services/narrative-orchestrator/narrativeorchestrator/prompt_builder.go) | Updated LLM prompt schemas |
| [`shared/eventbus/examples/universal_paths_example.go`](../shared/eventbus/examples/universal_paths_example.go) | Working example code |
| [`shared/jsonpath/examples/usage_examples.go`](../shared/jsonpath/examples/usage_examples.go) | jsonpath usage examples |

---

## ❓ FAQ

**Q: Do I need to migrate ALL my code at once?**  
A: No! Backward compatibility means old code continues to work. Migrate incrementally: start with new event producers, then update consumers.

**Q: What if I forget to use `event.Path()` and use direct map access?**  
A: Your code will still work for old-format events, but will fail for new-format events. Use `event.Path()` for forward compatibility.

**Q: How do I know if an event uses the new format?**  
A: Check for `entity:{id:...}` vs `entity_id:"..."`. Use `eventbus.ExtractEntityID()` which handles both automatically.

**Q: Can I use `jsonpath` for non-event data?**  
A: Yes! `jsonpath.New(anyData)` works with configs, API responses, any `map[string]any` structure.

**Q: What about performance?**  
A: `jsonpath` access is O(d) where d=depth — negligible for typical event depths (2-4 levels). `Has()` is optimized for quick existence checks.

---

## 🚀 Quick Start Checklist

For new code or migrations:

- [ ] Import `jsonpath`: `import "multiverse-core.io/shared/jsonpath"`
- [ ] Read events via `event.Path()` + getters with fallback
- [ ] Use `eventbus.GetWorldIDFromEvent()` / `GetScopeFromEvent()` for unified access
- [ ] Create events via `eventbus.NewEventPayload().WithEntity().WithWorld()` builder
- [ ] Add hierarchical paths explicitly for LLM clarity: `eventbus.SetNested(payload, "entity.id", ...)`
- [ ] Update LLM prompts to use hierarchical JSON schema + examples
- [ ] Test with BOTH old and new format events

---

> 💡 **Pro Tip**: Use `pa.GetAllPaths()` during development to discover available fields in complex payloads — great for debugging and documentation!

```go
// Debug helper:
func debugPayload(event eventbus.Event) {
    for _, path := range event.Path().GetAllPaths() {
        if val, ok := event.Path().GetAny(path); ok {
            log.Printf("%s = %v (%T)", path, val, val)
        }
    }
}
```

---

**🎯 Migration Status**: ✅ **Complete** — All core services updated, backward compatibility maintained, documentation ready.

*Last verified: 2026-02-22*
