package entity

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding"
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
// of integer and float, the lists and maps a Go caller builds, and anything else
// that only encoding/json knows how to write (a time.Time in an attribute, say).
//
// Each of them is written as the value the wire gives back, because that is
// what the same entity holds once it has been read from a snapshot: a typed nil
// list or map is null there, not an empty one; a float32 is the decimal the
// encoder wrote, not the float64 the conversion widens it to; a []byte is a
// base64 string; a type with its own MarshalJSON or MarshalText is whatever it
// says it is. Nothing that comes off the wire takes any of these branches, so
// the canonical form of a decoded value is the one it always was.
func writeCanonicalOther(buf *bytes.Buffer, value any) {
	if encodesItself(value) {
		writeCanonicalEncoded(buf, value)
		return
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(rv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		buf.WriteString(strconv.FormatUint(rv.Uint(), 10))
	case reflect.Float32:
		writeCanonicalEncoded(buf, value)
	case reflect.Float64:
		writeCanonicalFloat(buf, rv.Float())
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			writeCanonicalNilSlice(buf, value)
			return
		}
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			writeCanonicalEncoded(buf, value)
			return
		}
		list := make([]any, rv.Len())
		for i := range list {
			list[i] = rv.Index(i).Interface()
		}
		writeCanonicalArray(buf, list)
	case reflect.Map:
		if rv.IsNil() {
			buf.WriteString("null")
			return
		}
		if rv.Type().Key().Kind() != reflect.String {
			// The encoder writes integer keys as strings; the object it
			// produces is the one a snapshot holds.
			writeCanonicalEncoded(buf, value)
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
		writeCanonicalEncoded(buf, value)
	}
}

// writeCanonicalNilSlice writes a typed nil list the way the entity ends up
// holding it.
//
// For most types that is null, which is what the wire makes of it and what
// the entity keeps: a []string(nil) survives Clone and Commit as it is. The
// exception is []map[string]any, which jsonpath.Clone rebuilds as an empty
// list — and with it []any and map[string]any, which writeCanonical meets
// before this point and writes as [] and {} for the same reason. An entity
// never holds a nil of those three once Commit has copied its attributes, so
// hashing that nil as null would make a set of the same value look like a
// change on every repeat.
func writeCanonicalNilSlice(buf *bytes.Buffer, value any) {
	if _, rebuilt := value.([]map[string]any); rebuilt {
		buf.WriteString("[]")
		return
	}
	buf.WriteString("null")
}

// encodesItself says whether encoding/json writes a value through a method of
// its own rather than by its kind: json.RawMessage, time.Time, and any type
// with MarshalJSON or MarshalText on its value. A method on the pointer alone
// is not asked for a value held in an interface, and it is not asked here.
func encodesItself(value any) bool {
	switch value.(type) {
	case json.Marshaler, encoding.TextMarshaler:
		return true
	}
	return false
}

// writeCanonicalEncoded writes a value only the encoder understands — a struct
// such as the entity.Item a proposer appends, or a time.Time — as the value it
// becomes once it has been through the wire and back.
//
// Writing the encoder's output as it stands would keep the fields of a struct
// in declaration order, and the same item read back from a snapshot is a map
// whose keys are sorted: one entity, two hashes, and a recovery that reports a
// divergence nobody caused. Decoding the output first makes the struct and its
// wire form one value under the one rule of CanonicalJSON.
//
// The value cannot reach here from the wire, and ApplyOps refuses anything the
// encoder itself refuses, so a failure here is a nil that hashes.
func writeCanonicalEncoded(buf *bytes.Buffer, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		buf.WriteString("null")
		return
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		buf.WriteString("null")
		return
	}
	writeCanonical(buf, decoded)
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
