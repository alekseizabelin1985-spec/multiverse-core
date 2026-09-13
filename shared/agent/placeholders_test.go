package agent_test

import (
	"reflect"
	"strings"
	"testing"

	"multiverse-core.io/shared/agent"
)

// The dictionary of swarm-llm-laws.md §11.4, written out: a name added to or
// dropped from placeholders.go without the design changing fails here.
func TestPlaceholderDictionaryIsTheDesignList(t *testing.T) {
	design := strings.Fields("{world.name} {world.weather} {world.time_of_day} {world.day} {region.name} " +
		"{region.description} {player.name} {player.hp} {player.hp_max} {events} {state} {absence} " +
		"{canon} {laws_version} {locale} {npc.name} {encounter.round}")
	got := agent.PlaceholderNames()
	if len(got) != len(design) {
		t.Fatalf("PlaceholderNames() has %d names, the design %d: %v", len(got), len(design), got)
	}
	gotSet := map[string]bool{}
	for _, n := range got {
		gotSet[n] = true
	}
	for _, p := range design {
		n := strings.Trim(p, "{}")
		if !gotSet[n] || !agent.IsKnownPlaceholder(n) {
			t.Errorf("design placeholder %q is not in the dictionary", n)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("PlaceholderNames() is not sorted: %v", got)
		}
	}
	got[0] = "mutated"
	if agent.PlaceholderNames()[0] == "mutated" {
		t.Fatal("PlaceholderNames() exposes the dictionary instead of a copy")
	}
}

func TestIsKnownPlaceholderRejectsNearMisses(t *testing.T) {
	for _, name := range []string{"", "{events}", "Events", "world", "world.name.first", "player_name", "region.descr"} {
		if agent.IsKnownPlaceholder(name) {
			t.Errorf("IsKnownPlaceholder(%q) = true", name)
		}
	}
}

func TestExtractPlaceholders(t *testing.T) {
	text := "Игрок {player.name} ({player.hp}/{player.hp_max}) видит {events}.\n" +
		"Снова {player.name}. Ответ: {\"text\": \"...\", \"mentions\": []} и { } и {Events} и {1x}.\n" +
		"Старый формат: {player_name}, {nearby.entities}"
	want := []string{"player.name", "player.hp", "player.hp_max", "events", "player_name", "nearby.entities"}
	if got := agent.ExtractPlaceholders(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractPlaceholders = %v, want %v", got, want)
	}
	if got := agent.UnknownPlaceholders(text); !reflect.DeepEqual(got, []string{"player_name", "nearby.entities"}) {
		t.Fatalf("UnknownPlaceholders = %v", got)
	}
	if got := agent.ExtractPlaceholders("без плейсхолдеров"); got != nil {
		t.Fatalf("ExtractPlaceholders of plain text = %v, want nil", got)
	}
}

// The sections of the valid corpus use only names of the dictionary.
func TestValidCorpusSectionsUseOnlyKnownPlaceholders(t *testing.T) {
	for _, file := range []string{"global-world.md", "domain-region.md", "personal-gm.md", "group-narrator.md", "personal-gm.yaml", "domain-fair.md"} {
		p := parseValid(t, file).Prompts
		texts := append([]string{p.System, p.Phase1, p.Phase2, p.Tick, p.Description}, p.Canon...)
		for _, text := range texts {
			if unknown := agent.UnknownPlaceholders(text); len(unknown) != 0 {
				t.Errorf("%s: unknown placeholders %v", file, unknown)
			}
			if suspicious := agent.SuspiciousPlaceholders(text); len(suspicious) != 0 {
				t.Errorf("%s: suspicious placeholders %q", file, suspicious)
			}
		}
	}
}

// A placeholder written wrong is not a placeholder to the exact pattern, so
// UnknownPlaceholders cannot see it; SuspiciousPlaceholders does, for rule 13.
func TestSuspiciousPlaceholdersCatchTypos(t *testing.T) {
	text := "Игрок {Player.name}, { player.name }, {player-name}, {region.Description}.\n" +
		"Снова {Player.name}. Верно: {events}, {player_name}. JSON: {\"text\": \"...\"}, { \"a\": 1 }, {}, { }, {1x}."
	want := []string{"{Player.name}", "{ player.name }", "{player-name}", "{region.Description}"}
	if got := agent.SuspiciousPlaceholders(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("SuspiciousPlaceholders = %q, want %q", got, want)
	}
	if got := agent.SuspiciousPlaceholders("Ответь JSON {\"text\": \"...\", \"mentions\": []} по {state}"); got != nil {
		t.Fatalf("a JSON example and an exact placeholder are not suspicious, got %q", got)
	}
}

// Mi-6 of review #2: prompts are written in Russian, so a placeholder mistyped
// in Cyrillic is the likeliest typo of all. Braces that are not a placeholder
// stay unreported: JSON, positional {0}, regex quantifiers, Go templates.
func TestSuspiciousPlaceholdersCatchCyrillicTypos(t *testing.T) {
	decomposed := "{\u0438\u0306од}"
	text := "Игрок {игрок.имя}, {Игрок}, { имя }, {player.имя}, " + decomposed + ".\n" +
		"Снова {Игрок}. Не плейсхолдеры: {\"текст\": \"...\"}, { \"а\": 1 }, {0}, {1}, [A-Z]{2,5}, [а-я]{1,3}, " +
		"{{.Name}}, {{ .Player.HP }}, {{- .Имя -}}, { }, {}, {см. выше}, {-имя}, {_имя}."
	want := []string{"{игрок.имя}", "{Игрок}", "{ имя }", "{player.имя}", decomposed}
	if got := agent.SuspiciousPlaceholders(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("SuspiciousPlaceholders = %q, want %q", got, want)
	}
	if got := agent.UnknownPlaceholders(text); got != nil {
		t.Fatalf("a Cyrillic name is not in the exact form, UnknownPlaceholders = %q", got)
	}
}
