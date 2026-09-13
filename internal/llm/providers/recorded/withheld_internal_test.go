package recorded

import (
	"context"
	"errors"
	"testing"

	"multiverse-core.io/internal/llm"
)

// The status of a record decides whether its text goes back, not whether the
// text is there: a quarantined or filter_error record that reached the index
// past parseRecord — a Writer of this package, say — still withholds it.
func TestGenerateWithholdsByTheStatus(t *testing.T) {
	for _, status := range []llm.ValidationStatus{llm.ValidationQuarantined, llm.ValidationFilterError} {
		key := Key{CorrelationID: "cid-1", AgentID: "personal-gm:p1", Phase: llm.PhaseNarrative, Attempt: 1}
		p := &Provider{records: map[Key]Record{key: {
			Key:      key,
			Status:   status,
			HasRaw:   true,
			Response: llm.Response{Content: "blocked text", Model: "qwen3.8-27b"},
		}}}
		ctx := WithCall(context.Background(), key.AgentID, key.Attempt)
		resp, err := p.Generate(ctx, llm.Request{Phase: key.Phase, CorrelationID: key.CorrelationID})
		if !errors.Is(err, ErrResponseWithheld) || resp != (llm.Response{}) {
			t.Errorf("%s with a text: Generate = %+v, %v; want ErrResponseWithheld and no text", status, resp, err)
		}
	}
}
