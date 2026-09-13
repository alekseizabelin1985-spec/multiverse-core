package parser

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"multiverse-core.io/shared/env"
)

// ErrLanguage is an answer whose texts are not Russian by the rule of NFR-090
// and ADR-016 p. 2: a character of Han, Hiragana, Katakana or Hangul, or a
// share of Latin letters above the limit. It maps to the reason language.
var ErrLanguage = errors.New("llm/parser: language")

// ErrLanguagePolicy is a value of MV_LLM_LATIN_MAX_RATIO the check cannot use.
var ErrLanguagePolicy = errors.New("llm/parser: invalid language policy")

// LanguagePolicy is the limit of the language check.
type LanguagePolicy struct {
	// MaxLatinRatio is the largest share of Latin letters among all the letters
	// of the texts, identifiers not counted; 0.10 means 10 %.
	MaxLatinRatio float64
	// Identifiers are ids the call knows. A word equal to one of them is not
	// counted whatever its shape, in addition to the words that have the shape
	// of an id. It is optional: the policy read from the environment has none,
	// and a caller that knows the ids of its call sets them on a copy.
	Identifiers []string
}

// LanguagePolicyFrom reads MV_LLM_LATIN_MAX_RATIO from src (nil means the
// process environment). The value is a share between 0 and 1.
func LanguagePolicyFrom(src env.Source) (LanguagePolicy, error) {
	raw := env.LLMLatinMaxRatio.StringFrom(src)
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(ratio) || ratio < 0 || ratio > 1 {
		return LanguagePolicy{}, fmt.Errorf("%w: %s must be a share between 0 and 1, got %q",
			ErrLanguagePolicy, env.LLMLatinMaxRatio.Name(), raw)
	}
	return LanguagePolicy{MaxLatinRatio: ratio}, nil
}

// wordRe is a run of the characters an id is made of. A word is matched as a
// whole against identifierRe: part of Wi-Fi is not an id.
var wordRe = regexp.MustCompile(`[0-9A-Za-z_:-]+`)

// identifierRe is the shape of the ids a Russian text may carry in Latin; their
// letters are not counted (ADR-016 p. 2, NFR-090 "кроме идентификаторов"):
//
//   - lower-case segments of letters and digits joined by - or :, at least
//     two of them, the last one possibly a single capital: wolf-alpha,
//     npc-999, player-A, region:dark-forest, encounter-wolf:solo:p1;
//   - a UUID;
//   - a ULID.
//
// Wi-Fi, Hello-World and Dark-Forest do not have this shape and are counted.
var identifierRe = regexp.MustCompile(`^(?:` +
	`[a-z0-9]+(?:[-:][a-z0-9]+)*[-:](?:[a-z0-9]+|[A-Z])` +
	`|[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}` +
	`|[0-9A-HJKMNP-TV-Z]{26}` +
	`)$`)

// cjk are the scripts of which a Russian answer holds no character.
var cjk = []*unicode.RangeTable{unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul}

// CheckLanguage checks the texts of one answer — the strings at the text paths
// of the call — as one text (ADR-016 p. 2):
//
//   - no character of Han, Hiragana, Katakana or Hangul;
//   - Latin letters are at most policy.MaxLatinRatio of all letters, the
//     letters of identifiers — words of the shape of an id and words equal to
//     one of policy.Identifiers — left out of both counts.
//
// A text without letters passes. The error matches ErrLanguage and gives the
// counts, not the text.
func CheckLanguage(texts []string, policy LanguagePolicy) error {
	known := make(map[string]struct{}, len(policy.Identifiers))
	for _, id := range policy.Identifiers {
		known[id] = struct{}{}
	}
	var ideographs, letters, latin int
	for _, text := range texts {
		for _, r := range text {
			if unicode.IsOneOf(cjk, r) {
				ideographs++
			}
			if !unicode.IsLetter(r) {
				continue
			}
			letters++
			if unicode.Is(unicode.Latin, r) {
				latin++
			}
		}
		for _, word := range wordRe.FindAllString(text, -1) {
			if id := strings.Trim(word, "-:"); isIdentifier(id, known) {
				n := asciiLetters(id)
				letters -= n
				latin -= n
			}
		}
	}
	if ideographs > 0 {
		return fmt.Errorf("%w: %d CJK characters", ErrLanguage, ideographs)
	}
	if letters > 0 && float64(latin)/float64(letters) > policy.MaxLatinRatio {
		return fmt.Errorf("%w: %d Latin letters of %d, more than the share %g",
			ErrLanguage, latin, letters, policy.MaxLatinRatio)
	}
	return nil
}

// isIdentifier reports whether word, trimmed of the - and : of the punctuation
// around it, is an id: a known one or one of the shape of an id.
func isIdentifier(word string, known map[string]struct{}) bool {
	if _, ok := known[word]; ok {
		return true
	}
	return identifierRe.MatchString(word)
}

// asciiLetters counts the letters of word, which wordRe keeps ASCII: each one
// was counted as a letter and as Latin.
func asciiLetters(word string) int {
	n := 0
	for i := 0; i < len(word); i++ {
		if c := word[i]; 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
			n++
		}
	}
	return n
}
