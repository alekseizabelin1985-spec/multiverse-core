package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const contentHashDomain = "multiverse-core/agent-blueprint/v2\n"

var timeType = reflect.TypeFor[time.Time]()

// ContentHash returns "sha256:<64 lowercase hex>" of the content of the
// blueprint: every field and every section, but not SourceFile, ContentHash or
// ParseIssues (yaml:"-").
//
// The hash is taken over a canonical form built from the value itself, so that
// equal values have equal hashes and different values different ones:
//   - a struct is an object keyed by the yaml tag names, in sorted order;
//   - a field holding its zero value (nil pointer, nil slice or map, "", 0,
//     false, an all-zero struct) is not written at all;
//   - an empty but non-nil slice is written as [], so "owned_entity_types: []"
//     and a blueprint without the key hash apart, as the parser tells them apart;
//   - a pointer is written as what it points to, so "temperature: 0" differs
//     from no temperature;
//   - untyped values (any) are written by their dynamic type, map keys sorted;
//   - a number is written by its value, not by its Go type (see writeFloat);
//   - a time.Time, which YAML gives for an unquoted date, is written unquoted
//     as time:<RFC 3339 with nanoseconds, in the offset of the value>;
//   - a map key that is not a string is written unquoted with its type, as
//     int:1, and so cannot equal the string key "int:1".
//
// Evolution: a new field changes the hash only of the blueprints that set it to
// a non-zero value; renaming a yaml tag changes the hash of every blueprint
// using the key. The encoding is pinned by TestContentHashEncodingIsPinned,
// because the hash is published in agent.spawned and kept in the swarm snapshot.
//
// A blueprint the parser produced always has a hash. A hand-built one holding a
// value with no canonical form (a func, a channel, a complex number, a struct
// with unexported fields inside an untyped value) gets "".
func ContentHash(bp *AgentBlueprint) string {
	if bp == nil {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString(contentHashDomain)
	if err := writeCanonical(&buf, reflect.ValueOf(*bp), false); err != nil {
		return ""
	}
	sum := sha256.Sum256(buf.Bytes())
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeCanonical writes v. untyped is set below an interface: there a struct
// comes from outside the blueprint types, and the fields it hides would be
// content the hash cannot see.
func writeCanonical(buf *bytes.Buffer, v reflect.Value, untyped bool) error {
	switch v.Kind() {
	case reflect.Invalid:
		buf.WriteString("null")
	case reflect.Bool:
		buf.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		buf.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		writeFloat(buf, v.Float())
	case reflect.String:
		buf.WriteString(strconv.Quote(v.String()))
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return writeCanonical(buf, v.Elem(), untyped || v.Kind() == reflect.Interface)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			buf.WriteString("null")
			return nil
		}
		buf.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, v.Index(i), untyped); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case reflect.Map:
		if v.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return writeMap(buf, v, untyped)
	case reflect.Struct:
		if v.Type() == timeType {
			writeTime(buf, v.Interface().(time.Time))
			return nil
		}
		return writeStruct(buf, v, untyped)
	default:
		return fmt.Errorf("content hash: no canonical form for %s", v.Type())
	}
	return nil
}

// writeFloat writes a number by its value, as JSON and YAML read it: a float
// holding an integer within int64 or uint64 is written as that integer, so 1
// and 1.0, 1000000 and 1000000.0, -0.0 and 0 are one number. Any other finite
// float is written in its shortest form, which then has a "." or an exponent
// and cannot be taken for an integer. NaN and the infinities have no JSON form;
// they are written unquoted and so cannot collide with a string.
func writeFloat(buf *bytes.Buffer, f float64) {
	integral := f == math.Trunc(f)
	switch {
	case math.IsNaN(f):
		buf.WriteString("NaN")
	case math.IsInf(f, 1):
		buf.WriteString("+Inf")
	case math.IsInf(f, -1):
		buf.WriteString("-Inf")
	case integral && f >= -0x1p63 && f < 0x1p63:
		buf.WriteString(strconv.FormatInt(int64(f), 10))
	case integral && f >= 0x1p63 && f < 0x1p64:
		buf.WriteString(strconv.FormatUint(uint64(f), 10))
	default:
		buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	}
}

// writeTime writes the instant with the offset the value carries: the same
// instant written with another offset is another value, as it is in the JSON
// of an event. The form does not depend on the notation of the date in YAML
// (2001-1-1, 2001-01-01, 2001-01-01T00:00:00.000Z) nor on the zone of the
// machine, which only names the offset. The "time:" label keeps it apart from a
// string of the same text, and the missing quotes from the string "time:…".
func writeTime(buf *bytes.Buffer, t time.Time) {
	buf.WriteString("time:")
	buf.WriteString(t.Format(time.RFC3339Nano))
}

type canonicalEntry struct {
	key   string
	value []byte
}

func writeStruct(buf *bytes.Buffer, v reflect.Value, untyped bool) error {
	t := v.Type()
	entries := make([]canonicalEntry, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			if untyped {
				return fmt.Errorf("content hash: no canonical form for %s: unexported field %s", t, field.Name)
			}
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fv := v.Field(i)
		if fv.IsZero() {
			continue
		}
		value, err := canonicalBytes(fv, untyped)
		if err != nil {
			return err
		}
		entries = append(entries, canonicalEntry{key: strconv.Quote(name), value: value})
	}
	writeEntries(buf, entries)
	return nil
}

// writeMap writes a string key as the string and any other key unquoted, as
// <type>:<canonical key>, so that {1: a} and {"int:1": a} stay distinct while
// a map[any]any holding only string keys equals the map[string]any with the
// same content.
func writeMap(buf *bytes.Buffer, v reflect.Value, untyped bool) error {
	entries := make([]canonicalEntry, 0, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		key, err := canonicalKey(iter.Key(), untyped)
		if err != nil {
			return err
		}
		value, err := canonicalBytes(iter.Value(), untyped)
		if err != nil {
			return err
		}
		entries = append(entries, canonicalEntry{key: key, value: value})
	}
	writeEntries(buf, entries)
	return nil
}

func canonicalKey(k reflect.Value, untyped bool) (string, error) {
	if k.Kind() == reflect.Interface {
		if k.IsNil() {
			return "null", nil
		}
		k, untyped = k.Elem(), true
	}
	if k.Type() == reflect.TypeFor[string]() {
		return strconv.Quote(k.String()), nil
	}
	text, err := canonicalBytes(k, untyped)
	if err != nil {
		return "", err
	}
	return k.Type().String() + ":" + string(text), nil
}

func canonicalBytes(v reflect.Value, untyped bool) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v, untyped); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeEntries orders the entries by key and, for keys of one form, by value:
// two NaN keys of a map are distinct keys with one form, and without the second
// order their place would follow the random order of the map.
func writeEntries(buf *bytes.Buffer, entries []canonicalEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].key != entries[j].key {
			return entries[i].key < entries[j].key
		}
		return bytes.Compare(entries[i].value, entries[j].value) < 0
	})
	buf.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(e.key)
		buf.WriteByte(':')
		buf.Write(e.value)
	}
	buf.WriteByte('}')
}
