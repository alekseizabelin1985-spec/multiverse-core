package api_test

import (
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/api"
)

// codesOf are the error codes an operation of the spec declares through the
// responses it references.
func codesOf(t *testing.T, doc map[string]any, operationID string) []string {
	t.Helper()
	var node map[string]any
	for _, op := range operations(t, doc) {
		if op.OperationID == operationID {
			node = op.Node
		}
	}
	if node == nil {
		t.Fatalf("no operation %s", operationID)
	}
	var codes []string
	for _, resp := range mapping(t, node, "responses") {
		ref, _ := resp.(map[string]any)["$ref"].(string)
		if ref == "" {
			continue
		}
		name := strings.TrimPrefix(ref, "#/components/responses/")
		codes = append(codes, stringList(t, doc, "components", "responses", name, "x-error-codes")...)
	}
	return codes
}

// The operations of T-306 declare every code their handlers answer
// (characters.Service, handlers.Characters), and their descriptions state the
// behaviour the tests pin down: the wait, 202 creating, the deadline, the
// order of the checks, the input filter and the key of the link.
func TestOpenAPIOperationsOfCharactersDeclareTheirCodes(t *testing.T) {
	doc := loadSpec(t)
	for id, codes := range map[string][]string{
		"createCharacter": {api.CodeInvalidRequest, api.CodeNameRequired, api.CodeNameInvalid, api.CodeConsentRequired,
			api.CodeWorldNotFound, api.CodeFilterError, api.CodeInternal, api.CodeBusUnavailable, api.CodePayloadTooLarge},
		"getPlayer":  {api.CodePlayerNotFound, api.CodeInternal},
		"listWorlds": {api.CodeInternal},
	} {
		declared := codesOf(t, doc, id)
		for _, code := range codes {
			if !slices.Contains(declared, code) {
				t.Errorf("%s does not declare %s", id, code)
			}
		}
	}
	for id, phrases := range map[string][]string{
		"createCharacter": {"MV_GATEWAY_CHARACTER_WAIT", "status: creating", "MV_GATEWAY_CHARACTER_DEADLINE", "consent_required",
			"world_not_found", "name_required", "name_invalid", "filter_error", "(link_id, action_key)", "proposal_id"},
		"getPlayer":  {"status=creating", "player_not_found", "session"},
		"listWorlds": {"regions", "cloud_enabled"},
	} {
		var description string
		for _, op := range operations(t, doc) {
			if op.OperationID == id {
				description, _ = op.Node["description"].(string)
			}
		}
		for _, phrase := range phrases {
			if !strings.Contains(description, phrase) {
				t.Errorf("the description of %s does not mention %q", id, phrase)
			}
		}
	}
}
