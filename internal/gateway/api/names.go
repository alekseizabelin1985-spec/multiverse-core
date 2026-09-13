package api

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The bounds of a character name in characters (api-contracts.md §1.3).
const (
	NameMinRunes = 2
	NameMaxRunes = 32
)

// NamePattern is the rule of a character name of api-contracts.md §1.3 and
// FR-060: letters, digits, space and hyphen. It is the expression of the bot
// (T-311); the gateway and the bot check a name by ValidCharacterName, which
// lives here because both import this package.
const NamePattern = `^[\p{L}\p{Nd} -]+$`

var namePattern = regexp.MustCompile(NamePattern)

// ValidCharacterName reports whether a name is a character name: 2 to 32
// characters of NamePattern, without a space at either end and without two
// spaces in a row. The caller answers name_invalid when it is not.
//
// The function does not trim: api-contracts.md §1.3 lists the characters of a
// name and says nothing of trimming it, so a space at an end is refused like
// any other character outside the rule (Mi-4 of review #1 of T-306). Trimming
// the input of a player is the step of the caller before the check: the gateway
// trims the name of the request and the result of the input filter, and the
// bot trims the text of the message.
//
// Two points api-contracts.md §1.3 leaves open are read the stricter way until
// architect#3 decides (T-306):
//   - combining marks are refused: NamePattern has no \p{M}, so a name in NFD
//     ("й" as "и" and U+0306) does not pass, and a name must come composed.
//     Letters whose canonical form is another letter (U+212B ANGSTROM SIGN,
//     the CJK compatibility ideographs) are letters and still pass: telling
//     them apart needs golang.org/x/text/unicode/norm, which the module has as
//     an indirect dependency only;
//   - two or more spaces in a row are refused.
func ValidCharacterName(name string) bool {
	n := utf8.RuneCountInString(name)
	return n >= NameMinRunes && n <= NameMaxRunes && namePattern.MatchString(name) &&
		name == strings.TrimSpace(name) && !strings.Contains(name, "  ")
}
