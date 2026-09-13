package actions

import (
	"context"
	"fmt"
)

// The kinds of text an InputFilter checks.
const (
	KindSay  = "say"
	KindName = "name"
)

// The outcomes of a check.
const (
	FilterPass     = "pass"
	FilterBlocked  = "blocked"
	FilterReplaced = "replaced"
)

// FilterNoop is the only filter of MVP-1 (MV_GATEWAY_INPUT_FILTER).
const FilterNoop = "noop"

// FilterResult is the answer of a filter. Text is what goes to the bus in
// place of the text of the player, whatever the status.
type FilterResult struct {
	Status string
	Text   string
}

// InputFilter is the insertion point of an input filter of the operator
// (US-016, FR-056): it is called before player.said and before the name of a
// new character is proposed, and the bus gets FilterResult.Text, so the text of
// the player never leaves the gateway past it. An error fails the action
// closed: 422 filter_error.
type InputFilter interface {
	Check(ctx context.Context, kind, text string) (FilterResult, error)
}

// NoopFilter passes every text unchanged.
type NoopFilter struct{}

// Check passes text.
func (NoopFilter) Check(_ context.Context, _, text string) (FilterResult, error) {
	return FilterResult{Status: FilterPass, Text: text}, nil
}

// FilterFor returns the filter named by MV_GATEWAY_INPUT_FILTER. Any name but
// noop is an error of the start: a gateway configured with a filter it does not
// have must not run unfiltered.
func FilterFor(name string) (InputFilter, error) {
	if name != FilterNoop {
		return nil, fmt.Errorf("actions: input filter %q is unknown; MVP-1 has only %q", name, FilterNoop)
	}
	return NoopFilter{}, nil
}
