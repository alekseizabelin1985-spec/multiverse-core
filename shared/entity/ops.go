package entity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strconv"
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
// On the wire old is present exactly when the path existed before the change and
// new exactly when it exists after it (C-02 v1.6). A null value and a missing
// key read the same and are different states, so presence is carried by HasOld
// and HasNew and by nothing else: an append has no old, a remove of a key has no
// new, and a set to null has a new that is null. A consumer that catches up on
// facts writes new when it is there and deletes the path when it is not
// (state-and-mechanics.md §4.8).
//
// The flags alone decide, whatever the values hold. A Change built by hand has
// to set them, and MarshalJSON refuses one that contradicts them — a value with
// its flag down, or neither flag at all — rather than guess which was meant:
// the schema of entity.updated would refuse the entry anyway, but far from the
// code that built it.
type Change struct {
	Path   string
	Old    any
	New    any
	HasOld bool
	HasNew bool
}

// wireChange is the JSON shape of a Change on the way out. A pointer to a
// RawMessage is left out by omitempty when nil and written as it is otherwise,
// null included.
type wireChange struct {
	Path string           `json:"path"`
	Old  *json.RawMessage `json:"old,omitempty"`
	New  *json.RawMessage `json:"new,omitempty"`
}

// MarshalJSON writes old and new only when their flags are up.
func (c Change) MarshalJSON() ([]byte, error) {
	switch {
	case !c.HasOld && !c.HasNew:
		return nil, fmt.Errorf("entity: change %q: neither old nor new is present", c.Path)
	case !c.HasOld && c.Old != nil:
		return nil, fmt.Errorf("entity: change %q: old holds a value but HasOld is false", c.Path)
	case !c.HasNew && c.New != nil:
		return nil, fmt.Errorf("entity: change %q: new holds a value but HasNew is false", c.Path)
	}
	out := wireChange{Path: c.Path}
	var err error
	if c.HasOld {
		if out.Old, err = rawOf(c.Old); err != nil {
			return nil, fmt.Errorf("entity: change %q: old: %w", c.Path, err)
		}
	}
	if c.HasNew {
		if out.New, err = rawOf(c.New); err != nil {
			return nil, fmt.Errorf("entity: change %q: new: %w", c.Path, err)
		}
	}
	return json.Marshal(out)
}

func rawOf(v any) (*json.RawMessage, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(encoded)
	return &raw, nil
}

// UnmarshalJSON sets HasOld and HasNew from the keys the object carries. It
// reads the object as a map of raw values because a decoder that unmarshals a
// null into a pointer leaves the pointer nil, and a null old is a present one.
//
// A null in place of the whole object leaves the Change as it was, the way
// encoding/json treats a null for every other type.
func (c *Change) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var decoded Change
	if raw, ok := fields["path"]; ok {
		if err := json.Unmarshal(raw, &decoded.Path); err != nil {
			return fmt.Errorf("entity: change: path: %w", err)
		}
	}
	if raw, ok := fields["old"]; ok {
		decoded.HasOld = true
		if err := json.Unmarshal(raw, &decoded.Old); err != nil {
			return fmt.Errorf("entity: change %q: old: %w", decoded.Path, err)
		}
	}
	if raw, ok := fields["new"]; ok {
		decoded.HasNew = true
		if err := json.Unmarshal(raw, &decoded.New); err != nil {
			return fmt.Errorf("entity: change %q: new: %w", decoded.Path, err)
		}
	}
	*c = decoded
	return nil
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
	ReasonBadPath      = "malformed path" // not in the canonical form of C-02 v1.8 p. 2
	ReasonReservedPath = "reserved path"
	ReasonBelowScalar  = "path below a scalar attribute"
	ReasonBadIndex     = "no such element"
	ReasonIndexOnPath  = "index into an object"
	ReasonNotJSON      = "value is not JSON-compatible"
	ReasonNotInteger   = "value is not an integer"
	ReasonNotNumber    = "current value is not a number"
	ReasonCurrentRange = "current value is past ±(2^53-1)"
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
//
// The type of the entity decides what its attributes are (data-model.md §3):
// every operation first passes CheckOp for that type, and once all of them have
// run, every scalar they touched holds a value of its kind or is gone. A
// scalar that is gone is not refused here — a norm that needs it says so — but
// set hp {x: 999} and set state {...} would leave every reader of the
// attribute unable to read it (C-02 v1.8 p. 2).
func ApplyOps(e *Entity, ops []Op) (map[string]any, []Change, error) {
	attrs := cloneAttrs(e.Attributes)
	tracker := &changeTracker{seen: map[string]struct{}{}}
	touched := map[string]Op{}
	var roots []string
	for _, op := range ops {
		if err := CheckOp(e.Type, op); err != nil {
			return nil, nil, err
		}
		if err := applyOp(attrs, op, tracker); err != nil {
			return nil, nil, err
		}
		root := rootOf(op.Path)
		if _, seen := touched[root]; !seen {
			roots = append(roots, root)
		}
		touched[root] = op
	}
	for _, root := range roots {
		spec, typed := AttributeSpecOf(e.Type, root)
		value, present := attrs[root]
		if typed && present && !spec.Holds(value) {
			return nil, nil, ErrInvalidOp{Op: touched[root], Reason: ReasonWrongKind}
		}
	}
	return attrs, tracker.changes(e.Attributes, attrs), nil
}

// CheckOp says whether the verb and the path of an operation are well-formed
// for an entity of a type, whatever the entity holds: a verb of C-02, a path in
// its canonical form, not under a field of the entity and not below a scalar of
// the type (C-02 v1.8 p. 2). State checks it at step 1, before it looks the
// entity up, with the type the change set names; ApplyOps checks it again with
// the type the entity has.
//
// The value is left to the verb: JSONCompatible is the rule for it, and inc
// names a value past the range of an integer as not an integer. What depends
// on what the entity holds — an inc on a text, an index past the end — is
// ApplyOps' to find.
func CheckOp(entityType string, op Op) error {
	switch {
	case op.Path == "":
		return ErrInvalidOp{Op: op, Reason: ReasonEmptyPath}
	case !CanonicalPath(op.Path):
		return ErrInvalidOp{Op: op, Reason: ReasonBadPath}
	}
	root := rootOf(op.Path)
	if slices.Contains(ReservedPaths, root) || strings.HasPrefix(root, "_") {
		return ErrInvalidOp{Op: op, Reason: ReasonReservedPath}
	}
	if spec, typed := AttributeSpecOf(entityType, root); typed && spec.Scalar() && root != op.Path {
		return ErrInvalidOp{Op: op, Reason: ReasonBelowScalar}
	}
	switch op.Op {
	case OpSet, OpInc, OpAppend, OpRemove:
	default:
		return ErrInvalidOp{Op: op, Reason: ReasonUnknownOp}
	}
	return nil
}

func applyOp(attrs map[string]any, op Op, tracker *changeTracker) error {
	tokens, err := splitPath(op.Path)
	if err != nil {
		return ErrInvalidOp{Op: op, Reason: ReasonBadPath}
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
	if !JSONCompatible(op.Value) {
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
	if !intInSafeRange(delta) {
		return ErrInvalidOp{Op: op, Reason: ReasonNotJSON}
	}
	old, exists := readPath(attrs, op.Path)
	var current int64
	if exists && old != nil {
		if current, ok = asInt64(old); !ok {
			if numberPastSafeRange(old) {
				return ErrInvalidOp{Op: op, Reason: ReasonCurrentRange}
			}
			return ErrInvalidOp{Op: op, Reason: ReasonNotNumber}
		}
	}
	// An attribute can be past the range when it was built that way in Go, and
	// the sum of two numbers inside the range can leave it. With both operands
	// inside, the sum cannot overflow int64, so the check of the result — the
	// value the fact is going to carry — is not fooled by a wrap that clampHP
	// would otherwise pull back to zero. The attribute, not the operation, is
	// what is wrong then, and the reason says so.
	if !intInSafeRange(current) {
		return ErrInvalidOp{Op: op, Reason: ReasonCurrentRange}
	}
	result := clampHP(attrs, op.Path, tokens, current+delta)
	if !intInSafeRange(result) {
		return ErrInvalidOp{Op: op, Reason: ReasonNotJSON}
	}
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
	if !JSONCompatible(op.Value) {
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

// JSONCompatible is the definition of the phrase: a value survives a round trip
// through the wire. It rules out NaN and infinities, channels, functions and
// cycles — everything an op could carry that a fact could not. State checks the
// value of every operation with it, and the attributes of a create too.
//
// It also rules out a number of magnitude 2^53 or more anywhere inside the value
// (C-02 v1.6). Every reader of the bus decodes a number into a float64, and a
// float64 holds every integer only up to 2^53: State would store 2^53+1 and the
// fact, the snapshot and every read-model would hold 2^53, with a state hash
// that no replay reproduces.
//
// The bound excludes 2^53 itself because the proposal reaches State decoded as
// well: 2^53 and 2^53+1 arrive as the same float64, so a bound that let 2^53 in
// would let 2^53+1 in whenever the proposal came over the bus. With the bound at
// 2^53-1 every integer past it is still past it after decoding, and the answer
// is the same for a direct call and for the bus. Past 2^53 every float64 is a
// whole number, so the rule is the same for integers and for floats.
func JSONCompatible(v any) bool {
	if v == nil {
		return true
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return numbersInSafeRange(encoded)
}

// maxSafeInteger is 2^53-1, the largest magnitude a proposal may carry
// (C-02 v1.6; I-JSON, RFC 7493 §2.2).
const maxSafeInteger = 1<<53 - 1

// numberPastSafeRange says whether v is a number, of any Go type a payload can
// hold, whose magnitude is past maxSafeInteger.
func numberPastSafeRange(v any) bool {
	if !isNumber(v) {
		return false
	}
	encoded, err := json.Marshal(v)
	return err == nil && !numbersInSafeRange(encoded)
}

func isNumber(v any) bool {
	if _, ok := v.(json.Number); ok {
		return true
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// numbersInSafeRange walks the tokens of an encoded value and checks each
// number as it is written, before any float64 has had the chance to round it.
func numbersInSafeRange(encoded []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil {
			return false
		}
		if number, ok := token.(json.Number); ok && !numberInSafeRange(number) {
			return false
		}
	}
}

func numberInSafeRange(number json.Number) bool {
	i, err := strconv.ParseInt(number.String(), 10, 64)
	if err == nil {
		return intInSafeRange(i)
	}
	if errors.Is(err, strconv.ErrRange) {
		return false
	}
	f, err := strconv.ParseFloat(number.String(), 64)
	return err == nil && math.Abs(f) <= maxSafeInteger
}

func intInSafeRange(i int64) bool {
	return i >= -maxSafeInteger && i <= maxSafeInteger
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

// createdAncestor finds the outermost ancestor of path that the proposal made
// into a container: absent or a scalar before, there after. A path that is gone
// at the end reports nothing a consumer could rebuild that ancestor from — an
// inc of fresh.deep undone by a remove of it leaves fresh = {} behind, and
// neither entry says so — so the ancestor is reported on its own.
func createdAncestor(before, after map[string]any, path string) (string, bool) {
	for i := 1; i < len(path); i++ {
		if path[i] != '.' && path[i] != '[' {
			continue
		}
		prefix := path[:i]
		if _, exists := readPath(after, prefix); !exists {
			return "", false
		}
		if was, existed := readPath(before, prefix); !existed || !isContainer(was) {
			return prefix, true
		}
	}
	return "", false
}

// isContainer is what setIn walks into rather than replaces with a map.
func isContainer(v any) bool {
	if _, ok := v.(map[string]any); ok {
		return true
	}
	_, ok := foreignSlice(v)
	return ok
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
//
// The same existence is what the entry carries: old is present when the path
// was there before and new when it is there after (C-02 v1.6), so an append
// reports no old and a remove of a key reports no new.
func (t *changeTracker) changes(before, after map[string]any) []Change {
	for _, path := range slices.Clone(t.paths) {
		if _, exists := readPath(after, path); exists {
			continue
		}
		if ancestor, created := createdAncestor(before, after, path); created {
			t.touch(ancestor)
		}
	}
	out := make([]Change, 0, len(t.paths))
	for _, path := range t.paths {
		old, hadOld := readPath(before, path)
		updated, hasNew := readPath(after, path)
		if hadOld == hasNew && sameCanonical(old, updated) {
			continue
		}
		out = append(out, Change{
			Path:   path,
			Old:    snapshot(old),
			New:    snapshot(updated),
			HasOld: hadOld,
			HasNew: hasNew,
		})
	}
	return out
}
