package api_test

import (
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/api"
)

// The rule of a character name (api-contracts.md §1.3, FR-060) is the
// expression of the bot, with the stricter reading of the two open points:
// no combining marks, no two spaces in a row. The function does not trim: a
// space at an end is refused (Mi-4 of review #1 of T-306). Names of hyphens or
// digits alone pass: the contract allows both characters and forbids neither
// name, until architect#3 decides otherwise.
func TestValidCharacterName(t *testing.T) {
	if api.NamePattern != `^[\p{L}\p{Nd} -]+$` {
		t.Errorf("NamePattern = %s, want the expression of the bot (T-311)", api.NamePattern)
	}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Вася", true},
		{"Ли", true},
		{"Anna-Maria 2", true},
		{"李小龍", true},
		{strings.Repeat("я", 32), true},
		{"Й", false},
		{strings.Repeat("я", 33), false},
		{"", false},
		{"Вася!", false},
		{"Вася_Пупкин", false},
		{"Вася\tПупкин", false},
		{"Вася\nПупкин", false},
		{"Вася  Пупкин", false},
		{"Вася Пупкин", true},
		{"\u0438\u0306\u0432\u0430", false}, // "йва" in NFD: a combining mark
		{"йва", true},                       // the same name composed
		{" Вася", false},
		{"Вася ", false},
		{" Вася ", false},
		{" Ли", false},
		{"Ли ", false},
		{"--", true},
		{"12", true},
		{"\xffab", false},
		{"<b>", false},
		// Carried over from the test of the copy the bot had (acceptance of
		// T-312): the bot checks a name by this function now.
		{"Вася-2", true},
		{"Анна Мария", true},
		{"John Smith 3", true},
		{"Вася\u0007", false},
		{"Вася\t", false},
		{"[x](y)", false},
		{"Вася.", false},
	} {
		if got := api.ValidCharacterName(tc.name); got != tc.want {
			t.Errorf("ValidCharacterName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
