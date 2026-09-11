package entity

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"
)

// CanonicalJSON is the entity reduced to what the game actually is: identity,
// version and attributes, with every object key sorted, no whitespace and no
// exponent in a number (state-and-mechanics.md §3.3).
//
// What it leaves out is as important as what it keeps. updated_at, history,
// last_change and last_event_id say how the entity got here, not where it is,
// so two runs that replayed the same journal at different wall-clock times hash
// the same and recovery_state_identical means what it says.
//
// Sorting is applied to every object, the top level included: one rule, applied
// everywhere, is the only kind a second implementation can reproduce.
func CanonicalJSON(e *Entity) []byte {
	var buf bytes.Buffer
	writeCanonical(&buf, map[string]any{
		"id":         e.ID,
		"type":       e.Type,
		"version":    e.Version,
		"attributes": e.Attributes,
	})
	return buf.Bytes()
}

// StateHash is the hash of a whole world: the canonical form of every entity,
// concatenated in (type, id) order, through SHA-256. The result is
// "sha256:<hex>" — the value a snapshot carries and a recovery compares
// (state-and-mechanics.md §3.3, §4.4).
//
// The order of the input does not matter, and neither does the order in which
// the attribute maps happen to iterate: the same world hashes the same however
// it was assembled.
func StateHash(entities []*Entity) string {
	sorted := make([]*Entity, 0, len(entities))
	for _, e := range entities {
		if e != nil {
			sorted = append(sorted, e)
		}
	}
	slices.SortFunc(sorted, func(a, b *Entity) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	sum := sha256.New()
	for _, e := range sorted {
		sum.Write(CanonicalJSON(e))
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil))
}

func writeCanonical(buf *bytes.Buffer, value any) {
	switch v := value.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	case string:
		writeCanonicalString(buf, v)
	case json.Number:
		buf.WriteString(v.String())
	case map[string]any:
		writeCanonicalObject(buf, v)
	case []any:
		writeCanonicalArray(buf, v)
	default:
		writeCanonicalOther(buf, value)
	}
}

// writeCanonicalOther covers the values a type switch cannot list: every width
// of integer and float, and anything else that only encoding/json knows how to
// write (a time.Time in an attribute, say).
func writeCanonicalOther(buf *bytes.Buffer, value any) {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(rv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		buf.WriteString(strconv.FormatUint(rv.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		writeCanonicalFloat(buf, rv.Float())
	case reflect.Slice, reflect.Array:
		list := make([]any, rv.Len())
		for i := range list {
			list[i] = rv.Index(i).Interface()
		}
		writeCanonicalArray(buf, list)
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			buf.WriteString("null")
			return
		}
		object := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			object[iter.Key().String()] = iter.Value().Interface()
		}
		writeCanonicalObject(buf, object)
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			buf.WriteString("null")
			return
		}
		writeCanonical(buf, rv.Elem().Interface())
	default:
		// A struct, or something else that only the encoder understands. It
		// cannot reach here from the wire, and ApplyOps refuses anything the
		// encoder itself refuses, so a failure here is a nil that hashes.
		encoded, err := json.Marshal(value)
		if err != nil {
			buf.WriteString("null")
			return
		}
		buf.Write(encoded)
	}
}

func writeCanonicalObject(buf *bytes.Buffer, object map[string]any) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buf.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		writeCanonicalString(buf, key)
		buf.WriteByte(':')
		writeCanonical(buf, object[key])
	}
	buf.WriteByte('}')
}

func writeCanonicalArray(buf *bytes.Buffer, list []any) {
	buf.WriteByte('[')
	for i, item := range list {
		if i > 0 {
			buf.WriteByte(',')
		}
		writeCanonical(buf, item)
	}
	buf.WriteByte(']')
}

func writeCanonicalString(buf *bytes.Buffer, s string) {
	// encoding/json escapes deterministically, which is the only property a
	// canonical form needs from it.
	encoded, err := json.Marshal(s)
	if err != nil {
		buf.WriteString(`""`)
		return
	}
	buf.Write(encoded)
}

// writeCanonicalFloat writes a number without an exponent, so that a value that
// arrived as 10 and a value that arrived as 10.0 hash the same: JSON has one
// number type and the canonical form keeps it that way.
func writeCanonicalFloat(buf *bytes.Buffer, f float64) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		buf.WriteString("null")
		return
	}
	buf.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
}

// sameCanonical says whether two values mean the same thing on the wire. It is
// how a no-op is recognised: a number that arrived as 10.0 and a number written
// back as 10 are the same hit points, and reflect.DeepEqual would disagree.
func sameCanonical(a, b any) bool {
	var left, right bytes.Buffer
	writeCanonical(&left, a)
	writeCanonical(&right, b)
	return bytes.Equal(left.Bytes(), right.Bytes())
}
