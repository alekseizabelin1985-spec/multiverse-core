package state

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"multiverse-core.io/shared/entity"
)

// ErrCorruptFact is an entry of changed[] no fact of State carries (C-02 v1.6):
// an element past the end of its list, an index into what is not a list, a
// malformed path. Catching up on it is state_divergence (§4.8).
var ErrCorruptFact = errors.New("state: corrupt fact")

// CatchUpChanged applies the changed[] of an entity.updated to attributes by
// the rule of catching up (C-02 v1.6, state-and-mechanics.md §4.8) and returns
// the attributes after it; the attributes passed in are not changed.
//
// Entry by entry, in order:
//   - new present: written at its path. A node on the way that is missing or is
//     not a container is replaced with an object, as set does. The element a[n]
//     with n equal to the length of the list a — for n = 0 also a list that is
//     not there or null — is appended as it is, without the deduplication of
//     the op append: the fact already carries the outcome of it;
//   - new absent: the path is deleted; a path that is not there — an earlier
//     entry of the same fact wrote its container without it — is left alone.
//
// old is not read: it is there for a comparison, not for the write.
//
// This is the rule the vectors of shared/entity pin
// (TestChangedReportsTheAncestorAProposalCreated,
// TestCatchingUpAppendsWithoutDeduplication,
// TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved,
// TestCatchingUpRefusesAnElementPastTheEndOfItsList). It lives here until a
// production function of catching up is accepted into shared/entity (backlog of
// T-448); it does not go through entity.ApplyOps, whose grammar of paths by the
// type of an entity is not the business of a fact already decided.
func CatchUpChanged(attrs map[string]any, changed []entity.Change) (map[string]any, error) {
	root, ok := deepCopy(attrs).(map[string]any)
	if !ok || root == nil {
		root = map[string]any{}
	}
	for _, change := range changed {
		tokens, err := pathTokens(change.Path)
		if err != nil {
			return nil, err
		}
		if change.HasNew {
			written, err := writeAt(root, tokens, deepCopy(change.New))
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %w", ErrCorruptFact, change.Path, err)
			}
			root = written.(map[string]any)
			continue
		}
		deleteAt(root, tokens)
	}
	return root, nil
}

// pathToken is one step of a path: a key of an object or an index of a list.
type pathToken struct {
	key     string
	index   int
	isIndex bool
}

// pathTokens reads a canonical path a.b[0].c (C-02 v1.8 p. 2).
func pathTokens(path string) ([]pathToken, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: an empty path", ErrCorruptFact)
	}
	var tokens []pathToken
	for part := range strings.SplitSeq(path, ".") {
		key, rest, _ := strings.Cut(part, "[")
		if key == "" {
			return nil, fmt.Errorf("%w: %q has an empty key", ErrCorruptFact, path)
		}
		tokens = append(tokens, pathToken{key: key})
		for rest != "" {
			digits, after, closed := strings.Cut(rest, "]")
			n, err := strconv.Atoi(digits)
			if !closed || err != nil || n < 0 || (after != "" && !strings.HasPrefix(after, "[")) {
				return nil, fmt.Errorf("%w: %q has a malformed index", ErrCorruptFact, path)
			}
			tokens = append(tokens, pathToken{index: n, isIndex: true})
			rest = strings.TrimPrefix(after, "[")
		}
	}
	return tokens, nil
}

// writeAt writes value at tokens inside node and returns the node, which is a
// new object when node was not a container.
func writeAt(node any, tokens []pathToken, value any) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	head := tokens[0]
	if !head.isIndex {
		object, ok := node.(map[string]any)
		if !ok {
			object = map[string]any{}
		}
		child, err := writeAt(object[head.key], tokens[1:], value)
		if err != nil {
			return nil, err
		}
		object[head.key] = child
		return object, nil
	}
	var list []any
	switch held := node.(type) {
	case nil:
	case []any:
		list = held
	default:
		return nil, fmt.Errorf("index [%d] into a %T", head.index, node)
	}
	switch {
	case head.index < len(list):
		child, err := writeAt(list[head.index], tokens[1:], value)
		if err != nil {
			return nil, err
		}
		list[head.index] = child
		return list, nil
	case head.index == len(list) && len(tokens) == 1:
		return append(list, value), nil
	default:
		return nil, fmt.Errorf("element [%d] past the end of a list of %d", head.index, len(list))
	}
}

// deleteAt removes the value at tokens inside node, if it is there.
func deleteAt(node any, tokens []pathToken) {
	if len(tokens) == 0 {
		return
	}
	head, last := tokens[0], len(tokens) == 1
	switch held := node.(type) {
	case map[string]any:
		if head.isIndex {
			return
		}
		if last {
			delete(held, head.key)
			return
		}
		child := held[head.key]
		if list, ok := child.([]any); ok && len(tokens) == 2 && tokens[1].isIndex {
			if i := tokens[1].index; i < len(list) {
				held[head.key] = append(list[:i:i], list[i+1:]...)
			}
			return
		}
		deleteAt(child, tokens[1:])
	case []any:
		if head.isIndex && head.index < len(held) && !last {
			deleteAt(held[head.index], tokens[1:])
		}
	}
}

// deepCopy copies the maps and lists of a JSON value, so that a value written
// by catching up shares nothing with the fact or the entity it came from.
func deepCopy(v any) any {
	switch held := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(held))
		for k, x := range held {
			out[k] = deepCopy(x)
		}
		return out
	case []any:
		out := make([]any, len(held))
		for i, x := range held {
			out[i] = deepCopy(x)
		}
		return out
	default:
		return v
	}
}
