// services/narrativeorchestrator/prompt_builder_test.go

package narrativeorchestrator

import (
	"strings"
	"testing"
)

func minimalSections() PromptSections {
	return PromptSections{
		WorldFacts:     "Мир боли. Вечная тьма.",
		EntityStates:   "Кейн — у двери.",
		ScopeID:        "player:kain-777",
		ScopeType:      "player",
		WorldID:        "pain-realm",
		TimeContext:    "Ночь. 03:00",
		TriggerEvent:   "Кейн открыл дверь",
		MaxEvents:      3,
		DefaultSource:  "narrative-orchestrator",
		DefaultWorldID: "pain-realm",
	}
}

func TestBuildStructuredPrompt_SystemContainsRoleAndSchema(t *testing.T) {
	sys, _ := BuildStructuredPrompt(minimalSections())

	for _, tag := range []string{"<role>", "</role>", "<rules>", "</rules>", "<schema>", "</schema>"} {
		if !strings.Contains(sys, tag) {
			t.Errorf("system prompt missing tag %q", tag)
		}
	}
}

func TestBuildStructuredPrompt_UserContainsFacts(t *testing.T) {
	_, usr := BuildStructuredPrompt(minimalSections())

	for _, tag := range []string{"<facts>", "</facts>", "<situation>", "</situation>", "<task>"} {
		if !strings.Contains(usr, tag) {
			t.Errorf("user prompt missing tag %q", tag)
		}
	}
}

func TestBuildStructuredPrompt_SystemNotContainsDynamicSections(t *testing.T) {
	sys, _ := BuildStructuredPrompt(minimalSections())

	for _, bad := range []string{"<facts>", "<situation>", "<task>"} {
		if strings.Contains(sys, bad) {
			t.Errorf("system prompt must NOT contain dynamic tag %q", bad)
		}
	}
}

func TestBuildStructuredPrompt_UserNotContainsStaticSections(t *testing.T) {
	_, usr := BuildStructuredPrompt(minimalSections())

	for _, bad := range []string{"<role>", "<schema>"} {
		if strings.Contains(usr, bad) {
			t.Errorf("user prompt must NOT contain static tag %q", bad)
		}
	}
}

func TestBuildStructuredPrompt_CanonIncludedWhenPresent(t *testing.T) {
	s := minimalSections()
	s.Canon = []string{"Смерть необратима", "Магия истощает"}
	sys, _ := BuildStructuredPrompt(s)

	if !strings.Contains(sys, "<canon>") {
		t.Error("system prompt missing <canon> when Canon is non-empty")
	}
	if !strings.Contains(sys, "Смерть необратима") {
		t.Error("system prompt missing canon fact")
	}
}

func TestBuildStructuredPrompt_CanonOmittedWhenEmpty(t *testing.T) {
	s := minimalSections()
	s.Canon = nil
	sys, _ := BuildStructuredPrompt(s)

	if strings.Contains(sys, "<canon>") {
		t.Error("system prompt must NOT contain <canon> when Canon is empty")
	}
}

func TestBuildStructuredPrompt_MaxEventsDefault(t *testing.T) {
	s := minimalSections()
	s.MaxEvents = 0 // должно применить default 3
	sys, _ := BuildStructuredPrompt(s)

	if !strings.Contains(sys, "МАКСИМУМ 3") {
		t.Error("expected default MaxEvents=3 in rules")
	}
}

func TestBuildStructuredPrompt_XMLTagsPresent(t *testing.T) {
	sys, usr := BuildStructuredPrompt(minimalSections())

	systemTags := []string{"<role>", "<rules>", "<schema>"}
	for _, tag := range systemTags {
		if !strings.Contains(sys, tag) {
			t.Errorf("system missing XML tag %q", tag)
		}
	}

	userTags := []string{"<facts>", "<situation>", "<task>"}
	for _, tag := range userTags {
		if !strings.Contains(usr, tag) {
			t.Errorf("user missing XML tag %q", tag)
		}
	}
}

func TestMigratePromptInput(t *testing.T) {
	old := PromptInput{
		WorldContext:    "Мир",
		ScopeID:         "scope-1",
		ScopeType:       "location",
		EntitiesContext: "Сущности",
		EventClusters:   []EventCluster{{RelativeTime: "сейчас", Events: []EventDetail{{EventID: "ev1", Description: "событие"}}}},
		TimeContext:     "Утро",
		TriggerEvent:    "Триггер",
	}

	sections := MigratePromptInput(old)

	if sections.WorldFacts != old.WorldContext {
		t.Errorf("WorldFacts mismatch: got %q want %q", sections.WorldFacts, old.WorldContext)
	}
	if sections.EntityStates != old.EntitiesContext {
		t.Errorf("EntityStates mismatch: got %q want %q", sections.EntityStates, old.EntitiesContext)
	}
	if sections.ScopeID != old.ScopeID {
		t.Errorf("ScopeID mismatch: got %q want %q", sections.ScopeID, old.ScopeID)
	}
	if sections.ScopeType != old.ScopeType {
		t.Errorf("ScopeType mismatch: got %q want %q", sections.ScopeType, old.ScopeType)
	}
	if sections.TimeContext != old.TimeContext {
		t.Errorf("TimeContext mismatch: got %q want %q", sections.TimeContext, old.TimeContext)
	}
	if sections.TriggerEvent != old.TriggerEvent {
		t.Errorf("TriggerEvent mismatch: got %q want %q", sections.TriggerEvent, old.TriggerEvent)
	}
	if len(sections.EventClusters) != len(old.EventClusters) {
		t.Errorf("EventClusters length mismatch: got %d want %d", len(sections.EventClusters), len(old.EventClusters))
	}
	if sections.MaxEvents != 3 {
		t.Errorf("MaxEvents should default to 3, got %d", sections.MaxEvents)
	}
}

func TestCleanJSONResponse_MarkdownBlock(t *testing.T) {
	input := "```json\n{\"narrative\": \"test\"}\n```"
	got := cleanJSONResponse(input)
	want := `{"narrative": "test"}`
	if got != want {
		t.Errorf("cleanJSONResponse markdown: got %q want %q", got, want)
	}
}

func TestCleanJSONResponse_MarkdownBlockNoLang(t *testing.T) {
	input := "```\n{\"narrative\": \"test\"}\n```"
	got := cleanJSONResponse(input)
	want := `{"narrative": "test"}`
	if got != want {
		t.Errorf("cleanJSONResponse no-lang markdown: got %q want %q", got, want)
	}
}

func TestCleanJSONResponse_CleanJSON(t *testing.T) {
	input := `{"narrative": "already clean"}`
	got := cleanJSONResponse(input)
	if got != input {
		t.Errorf("cleanJSONResponse clean: got %q want %q", got, input)
	}
}
