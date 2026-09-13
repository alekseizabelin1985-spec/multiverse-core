package agent

import (
	"regexp"
	"sort"
)

// The fixed dictionary of prompt placeholders (C-11, swarm-llm-laws.md §11.4).
// A section of a blueprint writes a placeholder in braces: {world.weather}.
// The dictionary is closed: a name outside it is a validation error, and the
// prompt builder (internal/llm/prompt) substitutes exactly these names.
const (
	PlaceholderWorldName         = "world.name"
	PlaceholderWorldWeather      = "world.weather"
	PlaceholderWorldTimeOfDay    = "world.time_of_day"
	PlaceholderWorldDay          = "world.day"
	PlaceholderRegionName        = "region.name"
	PlaceholderRegionDescription = "region.description"
	PlaceholderPlayerName        = "player.name"
	PlaceholderPlayerHP          = "player.hp"
	PlaceholderPlayerHPMax       = "player.hp_max"
	PlaceholderEvents            = "events"
	PlaceholderState             = "state"
	PlaceholderAbsence           = "absence"
	PlaceholderCanon             = "canon"
	PlaceholderLawsVersion       = "laws_version"
	PlaceholderLocale            = "locale"
	PlaceholderNPCName           = "npc.name"
	PlaceholderEncounterRound    = "encounter.round"
)

var placeholderSet = map[string]struct{}{
	PlaceholderWorldName:         {},
	PlaceholderWorldWeather:      {},
	PlaceholderWorldTimeOfDay:    {},
	PlaceholderWorldDay:          {},
	PlaceholderRegionName:        {},
	PlaceholderRegionDescription: {},
	PlaceholderPlayerName:        {},
	PlaceholderPlayerHP:          {},
	PlaceholderPlayerHPMax:       {},
	PlaceholderEvents:            {},
	PlaceholderState:             {},
	PlaceholderAbsence:           {},
	PlaceholderCanon:             {},
	PlaceholderLawsVersion:       {},
	PlaceholderLocale:            {},
	PlaceholderNPCName:           {},
	PlaceholderEncounterRound:    {},
}

// placeholderRe matches a name in braces made of lowercase words joined by
// dots. JSON in a prompt ({"text": …}) and prose braces ({ }) do not match,
// so an example of the answer format is not mistaken for a placeholder.
var placeholderRe = regexp.MustCompile(`\{([a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*)\}`)

// suspiciousRe is the loose form of a placeholder: a word in braces that may
// carry capitals, hyphens or spaces inside the braces. What it matches and
// placeholderRe does not is a placeholder written wrong — the prompt builder
// would send it to the model literally. A JSON example still does not match:
// its braces open with a quote or a space followed by a quote. Letters are of
// any script, since prompts are written in Russian ({игрок.имя}); combining
// marks are letters too, so a decomposed й still matches.
var suspiciousRe = regexp.MustCompile(`\{\s*\p{L}[\p{L}\p{M}\p{N}_.\-]*\s*\}`)

// PlaceholderNames returns the dictionary, sorted.
func PlaceholderNames() []string {
	names := make([]string, 0, len(placeholderSet))
	for n := range placeholderSet {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// IsKnownPlaceholder reports whether name (without braces) is in the dictionary.
func IsKnownPlaceholder(name string) bool {
	_, ok := placeholderSet[name]
	return ok
}

// ExtractPlaceholders returns the placeholder names used in text, without
// braces, each once, in the order of first use.
func ExtractPlaceholders(text string) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range placeholderRe.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	return names
}

// UnknownPlaceholders returns the names used in text that are not in the
// dictionary, each once, in the order of first use.
func UnknownPlaceholders(text string) []string {
	var unknown []string
	for _, n := range ExtractPlaceholders(text) {
		if !IsKnownPlaceholder(n) {
			unknown = append(unknown, n)
		}
	}
	return unknown
}

// SuspiciousPlaceholders returns the fragments of text that look like a
// placeholder but are not written as one — {Player.name}, { player.name },
// {player-name} — each once, in the order of first use, braces included.
// A placeholder in the exact form, known or not, is not suspicious: it is
// reported by UnknownPlaceholders instead. The validator (rule 13) decides the
// severity.
func SuspiciousPlaceholders(text string) []string {
	var found []string
	seen := map[string]bool{}
	for _, m := range suspiciousRe.FindAllString(text, -1) {
		if seen[m] || isExactPlaceholder(m) {
			continue
		}
		seen[m] = true
		found = append(found, m)
	}
	return found
}

// isExactPlaceholder needs no anchors: a fragment of suspiciousRe has braces
// only at its ends and spaces only next to them, so an exact placeholder found
// inside it can only be the whole fragment.
func isExactPlaceholder(fragment string) bool {
	return placeholderRe.MatchString(fragment)
}
