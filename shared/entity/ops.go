package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"multiverse-core.io/shared/eventbus"
)

// OpKind is the verb of an operation. The four below are the whole vocabulary
// of entity.update.proposed (C-02); the schema rejects anything else on the
// wire, and ApplyOps rejects it in Go.
type OpKind string

const (
	OpSet    OpKind = "set"
	OpInc    OpKind = "inc"
	OpAppend OpKind = "append"
	OpRemove OpKind = "remove"
)

// Op is one operation of a proposal: what to do, where, and with what.
type Op struct {
	Op    OpKind `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// Change is one path that ended up different, as entity.updated carries it.
//
// Old is null when there was nothing at the path before — including the element
// of an append, where the schema allows old to be absent altogether.
type Change struct {
	Path string `json:"path"`
	Old  any    `json:"old"`
	New  any    `json:"new"`
}

// ChangeSet is one element of the changes array of entity.update.proposed: the
// entity, the version the proposer expects it to be at, and the operations
// (C-02, api-contracts.md §2.3.4).
type ChangeSet struct {
	Entity          eventbus.Entity `json:"entity"`
	ExpectedVersion *int64          `json:"expected_version,omitempty"`
	Ops             []Op            `json:"ops"`
}

// Propose builds the change set that carries ops for this entity. pinVersion
// adds the optimistic lock, which the contract makes mandatory for hp, status,
// inventory and the position of a fighter (ADR-013 p. 1).
func (e *Entity) Propose(ops []Op, pinVersion bool) ChangeSet {
	set := ChangeSet{Entity: e.EventEntity(), Ops: ops}
	if pinVersion {
		version := e.Version
		set.ExpectedVersion = &version
	}
	return set
}

// Ref is the entity a change set addresses.
func (c ChangeSet) Ref() Ref { return RefFrom(c.Entity.Entity) }

// Reasons an operation is malformed. They are the message of ErrInvalidOp, not
// the reason of a rejection: every one of them becomes invalid_op on the wire
// (C-02), and the text is what tells a developer which of them it was.
const (
	ReasonUnknownOp    = "unknown op"
	ReasonEmptyPath    = "empty path"
	ReasonBadPath      = "malformed path"
	ReasonReservedPath = "reserved path"
	ReasonBadIndex     = "no such element"
	ReasonIndexOnPath  = "index into an object"
	ReasonNotJSON      = "value is not JSON-compatible"
	ReasonNotInteger   = "value is not an integer"
	ReasonNotNumber    = "current value is not a number"
	ReasonNotList      = "current value is not a list"
	ReasonMissingPath  = "nothing at path"
)

// ErrInvalidOp is an operation that cannot be applied to any entity: a bad
// verb, a bad path, a value of the wrong shape. State answers it with
// entity.update.rejected reason=invalid_op (state-and-mechanics.md §3.2).
//
// It is not the error of a state that happens to be wrong right now — a
// version conflict, a dead entity or a broken invariant are decisions of
// internal/state and never come out of ApplyOps.
type ErrInvalidOp struct {
	Op     Op
	Reason string
}

func (e ErrInvalidOp) Error() string {
	return fmt.Sprintf("invalid op %s %q: %s", e.Op.Op, e.Op.Path, e.Reason)
}

// ReservedPaths are the roots an operation may not touch: they are fields of
// the entity, not attributes, and only State moves them
// (state-and-mechanics.md §3.2).
var ReservedPaths = []string{
	"id", "type", "world_id", "version", "created_at", "updated_at",
	"last_event_id", "history", "last_change", "schema_version",
}

// dedupeKeys identify an element of a list by identity rather than by value:
// two items with the same item_id are the same item however their other fields
// read. append skips a duplicate and remove takes it out by id.
var dedupeKeys = []string{"item_id", "player_id", "npc_id"}

// ApplyOps computes what ops do to an entity without touching it: the returned
// attributes are a fresh copy, and the returned changes are what
// entity.updated would carry.
//
// The changed list is built after every operation has run, by reading every
// path the operations reported on out of the entity as it was and out of the
// copy as it is: one entry per path, the first old and the last new, and a path
// that ends up where it started is not in the list at all — including a path
// that a later operation put back. An empty list is not an error — the turn
// counted, the version does not move and the fact is published with changed: []
// (C-02, UC-011 E2).
//
// The first malformed operation stops the whole set: a proposal is either
// well-formed or it is not, and a half-applied copy would be neither.
func ApplyOps(e *Entity, ops []Op) (map[string]any, []Change, error) {
	attrs := cloneAttrs(e.Attributes)
	tracker := &changeTracker{seen: map[string]struct{}{}}
	for _, op := range ops {
		if err := applyOp(attrs, op, tracker); err != nil {
			return nil, nil, err
		}
	}
	return attrs, tracker.changes(e.Attributes, attrs), nil
}

func applyOp(attrs map[string]any, op Op, tracker *changeTracker) error {
	tokens, err := splitPath(op.Path)
	if err != nil {
		return ErrInvalidOp{Op: op, Reason: ReasonBadPath}
	}
	if len(tokens) == 0 {
		return ErrInvalidOp{Op: op, Reason: ReasonEmptyPath}
	}
	if slices.Contains(ReservedPaths, tokens[0]) || strings.HasPrefix(tokens[0], "_") {
		return ErrInvalidOp{Op: op, Reason: ReasonReservedPath}
	}
	switch op.Op {
	case OpSet:
		return applySet(attrs, op, tokens, tracker)
	case OpInc:
		return applyInc(attrs, op, tokens, tracker)
	case OpAppend:
		return applyAppend(attrs, op, tokens, tracker)
	case OpRemove:
		return applyRemove(attrs, op, tokens, tracker)
	default:
		return ErrInvalidOp{Op: op, Reason: ReasonUnknownOp}
	}
}

func applySet(attrs map[string]any, op Op, tokens []string, tracker *changeTracker) error {
	if !jsonCompatible(op.Value) {
		return ErrInvalidOp{Op: op, Reason: ReasonNotJSON}
	}
	if _, err := setIn(attrs, tokens, op.Value); err != nil {
		return ErrInvalidOp{Op: op, Reason: writeReason(err)}
	}
	tracker.touch(op.Path)
	return nil
}

func applyInc(attrs map[string]any, op Op, tokens []string, tracker *changeTracker) error {
	delta, ok := asInt64(op.Value)
	if !ok {
		return ErrInvalidOp{Op: op, Reason: ReasonNotInteger}
	}
	old, exists := readPath(attrs, op.Path)
	var current int64
	if exists && old != nil {
		if current, ok = asInt64(old); !ok {
			return ErrInvalidOp{Op: op, Reason: ReasonNotNumber}
		}
	}
	result := clampHP(attrs, op.Path, tokens, current+delta)
	if _, err := setIn(attrs, tokens, result); err != nil {
		return ErrInvalidOp{Op: op, Reason: writeReason(err)}
	}
	tracker.touch(op.Path)
	return nil
}

// clampHP holds hit points inside [0, hp_max] (state-and-mechanics.md §3.2,
// §6.4): the invariant is a property of the attribute, so a rest that
// overshoots or a hit that overkills lands on the boundary instead of being
// rejected. Only hp is clamped, and only against the hp_max next to it.
func clampHP(attrs map[string]any, path string, tokens []string, value int64) int64 {
	if tokens[len(tokens)-1] != AttrHP {
		return value
	}
	if value < 0 {
		value = 0
	}
	if raw, ok := readPath(attrs, siblingPath(path, AttrHP, AttrHPMax)); ok {
		if limit, ok := asInt64(raw); ok && value > limit {
			value = limit
		}
	}
	return value
}

// siblingPath replaces the last segment of a path, which is a key and not an
// index whenever this is called.
func siblingPath(path, last, sibling string) string {
	if path == last {
		return sibling
	}
	return strings.TrimSuffix(path, "."+last) + "." + sibling
}

func applyAppend(attrs map[string]any, op Op, tokens []string, tracker *changeTracker) error {
	if !jsonCompatible(op.Value) {
		return ErrInvalidOp{Op: op, Reason: ReasonNotJSON}
	}
	current, _ := readPath(attrs, op.Path)
	list, ok := asAnySlice(current)
	if !ok {
		return ErrInvalidOp{Op: op, Reason: ReasonNotList}
	}
	if hasSameIdentity(list, op.Value) {
		// The proposal is a repeat, not a second item: no change, no error
		// (state-and-mechanics.md §3.2).
		return nil
	}
	list = append(list, op.Value)
	if _, err := setIn(attrs, tokens, list); err != nil {
		return ErrInvalidOp{Op: op, Reason: writeReason(err)}
	}
	// The path of the change is the path of the element, so that a read-model
	// can apply it without holding the list (C-02 v1.1).
	tracker.touch(fmt.Sprintf("%s[%d]", op.Path, len(list)-1))
	return nil
}

func applyRemove(attrs map[string]any, op Op, tokens []string, tracker *changeTracker) error {
	current, exists := readPath(attrs, op.Path)
	if !exists {
		return ErrInvalidOp{Op: op, Reason: ReasonMissingPath}
	}
	if op.Value == nil {
		// An element of a list is reported on the path of the list, with the
		// list before and after. Taking one out shifts every element behind
		// it, and a read-model that applies changed[] literally has to shift
		// them too rather than write a null into the slot that was vacated
		// (C-02, state-and-mechanics.md §3.3).
		report, isElement := listPathOf(attrs, op.Path, tokens)
		if !isElement {
			report = op.Path
		}
		if _, ok := deleteIn(attrs, tokens); !ok {
			return ErrInvalidOp{Op: op, Reason: ReasonMissingPath}
		}
		tracker.touch(report)
		return nil
	}
	list, ok := asAnySlice(current)
	if !ok {
		return ErrInvalidOp{Op: op, Reason: ReasonNotList}
	}
	kept := make([]any, 0, len(list))
	for _, item := range list {
		if sameIdentity(item, op.Value) {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == len(list) {
		return nil
	}
	if _, err := setIn(attrs, tokens, kept); err != nil {
		return ErrInvalidOp{Op: op, Reason: writeReason(err)}
	}
	tracker.touch(op.Path)
	return nil
}

// writeReason names the malformed operation behind a refused write.
func writeReason(err error) string {
	if errors.Is(err, errIndexOnObject) {
		return ReasonIndexOnPath
	}
	return ReasonBadIndex
}

// sameIdentity says whether two list elements are the same thing: objects by
// their item_id, player_id or npc_id when both carry one, everything else by
// what they mean on the wire.
//
// Both halves look at the wire form, not at the Go value. A list read off the
// wire holds maps and float64, while a proposer built in Go appends an
// entity.Item and removes an int: compared as Go values, the second append of
// one trophy is a second trophy (inv-03), and removing 3 from [3] is silently
// nothing. The ids themselves are strings in every shape, so they compare as
// they are.
func sameIdentity(a, b any) bool {
	am, aok := asObject(a)
	bm, bok := asObject(b)
	if aok && bok {
		for _, key := range dedupeKeys {
			av, ahas := am[key]
			bv, bhas := bm[key]
			if ahas && bhas {
				return reflect.DeepEqual(av, bv)
			}
		}
	}
	return sameCanonical(a, b)
}

// asObject reads a list element as a JSON object: a map read off the wire as it
// is, and a struct or a map built in Go as the object the wire will make of it.
// Anything that is not an object on the wire — a scalar, a list, a time.Time —
// is not one here either.
func asObject(v any) (map[string]any, bool) {
	if object, ok := v.(map[string]any); ok {
		return object, true
	}
	if v == nil {
		return nil, false
	}
	switch reflect.Indirect(reflect.ValueOf(v)).Kind() {
	case reflect.Struct, reflect.Map:
	default:
		return nil, false
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return nil, false
	}
	return object, true
}

func hasSameIdentity(list []any, value any) bool {
	if _, ok := asObject(value); !ok {
		// Only objects are deduplicated: a list of scalars is a list, and
		// appending the same tag twice is the caller's business. An object
		// with an item_id, player_id or npc_id is a repeat when an element
		// carries the same id; an object without one is a repeat only when an
		// element is equal to it whole, canonically ({"at": 2} and
		// {"at": 2.0} are equal).
		return false
	}
	return slices.ContainsFunc(list, func(item any) bool { return sameIdentity(item, value) })
}

// jsonCompatible is the definition of the phrase: a value survives a round trip
// through the wire. It rules out NaN and infinities, channels, functions and
// cycles — everything an op could carry that a fact could not.
func jsonCompatible(v any) bool {
	if v == nil {
		return true
	}
	_, err := json.Marshal(v)
	return err == nil
}

// asInt64 reads a JSON number that is a whole number. A payload decoded from
// the wire holds float64, code built in Go holds int, and both mean the same
// hit points. A number outside int64 is not one: converting it would wrap it
// into a different number, and a wrong hit point is worse than a refused one.
func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return uintToInt64(uint64(n))
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return uintToInt64(n)
	case float32:
		return floatToInt64(float64(n))
	case float64:
		return floatToInt64(n)
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func uintToInt64(n uint64) (int64, bool) {
	if n > math.MaxInt64 {
		return 0, false
	}
	return int64(n), true
}

// floatToInt64 checks the range before converting: a float64 conversion out of
// the range of int64 is implementation-defined in Go, not an error.
func floatToInt64(f float64) (int64, bool) {
	if f < math.MinInt64 || f >= math.MaxInt64 {
		return 0, false
	}
	i := int64(f)
	if float64(i) != f {
		return 0, false
	}
	return i, true
}

// changeTracker collects the paths the operations reported on, in the order
// they were first touched. What each of them ended up meaning is read from the
// state once every operation has run: a path is a report, not a result, so an
// append that a later remove undoes leaves no entry behind, and the first old
// and the last new fall out of one comparison instead of being accumulated.
type changeTracker struct {
	paths []string
	seen  map[string]struct{}
}

func (t *changeTracker) touch(path string) {
	if _, ok := t.seen[path]; ok {
		return
	}
	t.seen[path] = struct{}{}
	t.paths = append(t.paths, path)
}

// changes compares every reported path in the entity as it was against the copy
// as it is.
//
// Two things make a change. The value, compared canonically rather than with
// reflect.DeepEqual: an inc that lands back where it started wrote an int64
// over a float64, and a value that does not move the state hash is not a
// change. And existence: a path that is not there and a path holding null read
// as the same value and are not the same state, so setting an absent key to
// null, appending a null element and removing a key that held null all count
// (state-and-mechanics.md §3.2, §3.3; C-02, where version grows by one exactly
// when changed[] is not empty).
func (t *changeTracker) changes(before, after map[string]any) []Change {
	out := make([]Change, 0, len(t.paths))
	for _, path := range t.paths {
		old, hadOld := readPath(before, path)
		updated, hasNew := readPath(after, path)
		if hadOld == hasNew && sameCanonical(old, updated) {
			continue
		}
		out = append(out, Change{Path: path, Old: snapshot(old), New: snapshot(updated)})
	}
	return out
}
