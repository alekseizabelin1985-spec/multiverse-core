package entity

import (
	"errors"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"multiverse-core.io/shared/jsonpath"
)

// An op path is written one way only (C-02 v1.8 p. 2): keys between single
// dots, and the index of a list element in brackets after its key, a[0][1].
// A key is not empty, holds no dot and no bracket, and is not written as a
// decimal integer of any length. CanonicalPath is that grammar; ApplyOps refuses every other
// spelling before it reads or writes anything.
//
// shared/jsonpath is more lenient: it reads status., .status and inventory.0 as
// second spellings of status and inventory[0]. A second spelling slips past
// every rule that compares a path as a string, and remove inventory.0 reported
// the element instead of the list, so a consumer catching up on the fact
// wrote into the slot instead of shifting the list (probe U-1 of T-056).
// Reading a canonical path is still delegated to jsonpath below.
//
// Writing is not: jsonpath.Accessor.Set walks maps only and would turn
// members[0] into a map key named "0", and it has no Delete for a list element.
// splitPath, setIn and deleteIn are the write half of the same grammar. The
// tests here check the shape of the container an operation leaves behind, not
// only that something can be read back from the path.

// canonicalSegment is one key of a path with its indices: a key without dots
// and brackets, then any number of [n], n being 0 or one to nine digits without
// a leading zero. inventory[01] would be inventory[1] under a second spelling,
// and an index past nine digits may not fit an int on every platform — past
// nineteen it fits none, and on an object it reads as a key made of digits.
var canonicalSegment = regexp.MustCompile(`^([^.\[\]]+)(?:\[(?:0|[1-9][0-9]{0,8})\])*$`)

// numericKey is a key written as a decimal integer: digits with an optional
// sign, of any length (C-02 v1.8b p. 2). On a list inventory.0 is inventory[0],
// and the index is written in brackets only. The rule is a pattern and not
// strconv.Atoi: where Atoi overflows depends on the size of int, so a key of
// ten digits would be canonical on a 32-bit platform and not on a 64-bit one,
// and one journal would be answered differently on two machines (NFR-061).
var numericKey = regexp.MustCompile(`^[+-]?[0-9]+$`)

// CanonicalPath reports whether path is written in the canonical form of C-02
// v1.8b p. 2: canonicalSegment for every segment, and no key that numericKey
// matches (0, 007, +1, -1, 99999999999999999999).
//
// It is the rule of an operation and of an entry of changed[]: State publishes
// only canonical paths, so a consumer catching up on a fact treats any other
// spelling as a corrupt fact.
func CanonicalPath(path string) bool {
	if path == "" {
		return false
	}
	for _, segment := range strings.Split(path, ".") {
		match := canonicalSegment.FindStringSubmatch(segment)
		if match == nil {
			return false
		}
		if numericKey.MatchString(match[1]) {
			return false
		}
	}
	return true
}

// rootOf is the attribute a path is under: hp of hp, inventory of
// inventory[0].kind.
func rootOf(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// errPathSyntax is an unterminated [ in a path. CanonicalPath refuses such a
// path before it gets here; splitPath still says so rather than guess.
var errPathSyntax = errors.New("malformed path")

// splitPath turns a.b[0].c into [a b 0 c]. On a canonical path a token that
// reads as an integer came out of brackets, because no key does.
func splitPath(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	var (
		tokens  []string
		current strings.Builder
	)
	for i := 0; i < len(path); i++ {
		switch ch := path[i]; ch {
		case '.':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '[':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			end := strings.IndexByte(path[i+1:], ']')
			if end < 0 {
				return nil, errPathSyntax
			}
			tokens = append(tokens, path[i+1:i+1+end])
			i += end + 1
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

// readPath returns the value at path and whether anything is there.
func readPath(attrs map[string]any, path string) (any, bool) {
	return jsonpath.New(attrs).GetAny(path)
}

// snapshot deep-copies a value so that a recorded old or new value survives the
// operations applied after it.
func snapshot(v any) any {
	if v == nil {
		return nil
	}
	copied, ok := jsonpath.New(v).Clone().GetAny("")
	if !ok {
		return v
	}
	return copied
}

// cloneAttrs deep-copies an attribute set; a nil map becomes an empty one, so
// an entity always has somewhere to write.
func cloneAttrs(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return map[string]any{}
	}
	copied, ok := jsonpath.New(attrs).Clone().GetAny("")
	if !ok {
		return map[string]any{}
	}
	out, ok := copied.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return out
}

// setIn writes value at tokens inside container and returns the container,
// which may be a different value than the one passed in: a scalar on the way
// becomes a map (state-and-mechanics.md §3.2, "creates intermediate maps").
func setIn(container any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	head := tokens[0]
	switch c := container.(type) {
	case map[string]any:
		if isIndex(head) {
			return nil, errIndexOnObject
		}
		if len(tokens) == 1 {
			c[head] = value
			return c, nil
		}
		child, err := setIn(c[head], tokens[1:], value)
		if err != nil {
			return nil, err
		}
		c[head] = child
		return c, nil
	case []any:
		idx, ok := sliceIndex(head, len(c))
		if !ok {
			return nil, errIndexOutOfRange
		}
		if len(tokens) == 1 {
			c[idx] = value
			return c, nil
		}
		child, err := setIn(c[idx], tokens[1:], value)
		if err != nil {
			return nil, err
		}
		c[idx] = child
		return c, nil
	default:
		if list, ok := foreignSlice(container); ok {
			// A list built in Go rather than read off the wire. Without this
			// it would fall through to the branch below and the whole list
			// would be replaced by an object.
			return setIn(list, tokens, value)
		}
		// Nothing, or a scalar, where a container is needed.
		return setIn(map[string]any{}, tokens, value)
	}
}

// errIndexOutOfRange is an index that no element of the list has. Growing a
// list is what append is for, so set does not do it silently.
var errIndexOutOfRange = errors.New("index out of range")

// errIndexOnObject is an index where an object is. The grammar spells the
// index of a list and a key of an object the same way once a path is split, so
// only the container underneath tells them apart — and an object never has
// one. Writing members[0] as the key "0" is exactly what jsonpath.Accessor.Set
// does and the reason this half of the grammar is written here.
var errIndexOnObject = errors.New("index into an object")

// isIndex says whether a token addresses an element of a list rather than a key
// of an object. On a canonical path only a token written in brackets does.
func isIndex(token string) bool {
	n, err := strconv.Atoi(token)
	return err == nil && n >= 0
}

// foreignSlice normalises a Go slice that is not []any — the []string a fixture
// or a test built in Go holds — into the []any this grammar walks. A nil, a
// scalar and a map are not slices and are left to the caller.
func foreignSlice(container any) ([]any, bool) {
	if container == nil {
		return nil, false
	}
	kind := reflect.ValueOf(container).Kind()
	if kind != reflect.Slice && kind != reflect.Array {
		return nil, false
	}
	return asAnySlice(container)
}

// listPathOf answers whether path addresses an element of a list and, if it
// does, the path of the list itself. It is what tells a remove of an element
// from a remove of a key: the first shifts a list and has to be reported on the
// list, the second empties a slot and is reported where it was.
func listPathOf(attrs map[string]any, path string, tokens []string) (string, bool) {
	open := strings.LastIndexByte(path, '[')
	if len(tokens) < 2 || open <= 0 || !strings.HasSuffix(path, "]") {
		return "", false
	}
	parent := path[:open]
	raw, ok := readPath(attrs, parent)
	if !ok {
		return "", false
	}
	list, ok := raw.([]any)
	if !ok {
		return "", false
	}
	if _, ok := sliceIndex(tokens[len(tokens)-1], len(list)); !ok {
		return "", false
	}
	return parent, true
}

// deleteIn removes what tokens address and reports whether anything went away.
// Like setIn it returns the container, because removing an element of a list
// produces a new slice that the parent has to store.
func deleteIn(container any, tokens []string) (any, bool) {
	if len(tokens) == 0 {
		return container, false
	}
	head := tokens[0]
	switch c := container.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			if _, ok := c[head]; !ok {
				return c, false
			}
			delete(c, head)
			return c, true
		}
		child, ok := deleteIn(c[head], tokens[1:])
		if !ok {
			return c, false
		}
		c[head] = child
		return c, true
	case []any:
		idx, ok := sliceIndex(head, len(c))
		if !ok {
			return c, false
		}
		if len(tokens) == 1 {
			return append(c[:idx:idx], c[idx+1:]...), true
		}
		child, ok := deleteIn(c[idx], tokens[1:])
		if !ok {
			return c, false
		}
		c[idx] = child
		return c, true
	default:
		return container, false
	}
}

// sliceIndex reads a token as an index into a list of length n.
func sliceIndex(token string, n int) (int, bool) {
	idx, err := strconv.Atoi(token)
	if err != nil || idx < 0 || idx >= n {
		return 0, false
	}
	return idx, true
}

// asAnySlice normalises any Go slice into []any. Attributes read from JSON are
// []any already; a fixture or a test built in Go may hold []string, and an
// append to it has to work the same way.
func asAnySlice(v any) ([]any, bool) {
	switch list := v.(type) {
	case nil:
		return nil, true
	case []any:
		return list, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}
