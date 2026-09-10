package privacy

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The rules a finding is filed under. They are the names CI prints, so they
// change only together with the documentation that refers to them.
const (
	// CheckBotToken is a Telegram bot token: <digits>:<35 characters>.
	CheckBotToken = "bot_token"
	// CheckExternalID is a numeric external identifier: behind a key that
	// names one (external_id, chat_id, update_id), or as the id of the
	// from, chat or user object of a Telegram update (SEC-01).
	CheckExternalID = "external_id"
	// CheckUsername is a platform handle: behind a key that names one, or
	// written as @handle in free text.
	CheckUsername = "username"
	// CheckEmail is an e-mail address.
	CheckEmail = "email"
	// CheckAPIKey is a provider key of the shape every provider uses.
	CheckAPIKey = "api_key"
)

// maxFileSize bounds what the scanner reads into memory. A recording larger
// than this is a defect of whatever wrote it, and reading it would turn a
// one-minute job into a memory incident; the file is reported as skipped
// rather than silently passed.
const maxFileSize = 16 << 20

// A key of a key/value pair is matched with a leading (?:^|[^a-z0-9]) rather
// than with a word boundary, because the keys of this platform live inside
// longer names — MV_TEST_USER_ID, from_id — and an underscore is a word
// character, so a boundary would never fire in front of the key.
//
// rule is one pattern the scanner looks for. group is the capture group that
// holds the value itself — 0 when the whole match is the value — so that the
// key of a key/value pair stays out of the redacted output and out of the
// placeholder test.
type rule struct {
	name  string
	re    *regexp.Regexp
	group int
	what  string
}

// rules are the patterns of the minimal scan (ADR-010 add. 1: numeric external
// identifiers and usernames).
//
// They are deliberately shallow: this command exists so that the security job
// of CI has something to run from wave 0 on, and EPIC-005 (T-139) replaces it
// with the full scan over the bus, the object store, the memory index and the
// logs. Two consequences follow from that scope, and both are on purpose: the
// scanner reads text as text (no JSON or JSONL parsing), and it prefers a false
// positive to a miss — a fixture that trips a rule is renamed, which costs a
// minute, while a leaked identifier in Git costs a history rewrite.
//
// The two credential shapes carry the ranges of shared/logging.credentialPatterns
// verbatim — a bot token is (?:bot)?\d{6,}: and a provider key is sk-…{8,} —
// so that what the logger redacts at run time and what this scanner refuses to
// commit are the same set. The bounds matter: bot identifiers are already ten
// digits and growing, and both a seven- and an eleven-digit one used to pass
// here while the logger caught them. The key lists still differ on purpose —
// the logger hides the value of an attribute, this scanner hunts a value
// wherever it is written — and EPIC-005 (T-139) is where the single exported
// source of keys belongs.
var rules = []rule{
	{
		// The suffix of a token is 35 characters today; the range is wider
		// because the length has moved before and a scanner that misses a
		// token is worse than one that reads a long identifier as a token.
		// The shape is the one shared/logging redacts, bot prefix included.
		name:  CheckBotToken,
		re:    regexp.MustCompile(`(?i)\b(?:bot)?\d{6,}:[A-Za-z0-9_-]{30,}`),
		group: 0,
		what:  "a Telegram bot token",
	},
	{
		name:  CheckExternalID,
		re:    regexp.MustCompile(`(?im)(?:^|[^a-z0-9])(?:external_?id|chat_?id|telegram_?id|tg_?id|from_?id|user_?id|update_?id)\b["']?\s*[:=]\s*["']?(\d{5,15})\b`),
		group: 1,
		what:  "a numeric external identifier",
	},
	{
		// The canonical shape of a Telegram update: the identifier sits
		// inside the from, chat or user object rather than behind a key
		// that names it, so the rule above never sees it. [^{}] keeps the
		// match inside one object, which is as far as a regexp may go —
		// the structural read of JSON and JSONL is EPIC-005 (T-139).
		name:  CheckExternalID,
		re:    regexp.MustCompile(`(?is)"(?:from|chat|user|sender)"\s*:\s*\{[^{}]*?"id"\s*:\s*"?(\d{5,15})`),
		group: 1,
		what:  "a numeric external identifier inside a from, chat or user object",
	},
	{
		name:  CheckUsername,
		re:    regexp.MustCompile(`(?im)(?:^|[^a-z0-9])(?:user_?name|first_?name|last_?name|nick_?name)\b["']?\s*[:=]\s*["']?@?([A-Za-z][A-Za-z0-9_.-]{2,31})\b`),
		group: 1,
		what:  "a platform handle behind a key that names one",
	},
	{
		// The trailing class excludes ':' and '/' so that image digests
		// (debian@sha256:...) and paths are not read as handles; RE2 has no
		// lookahead, so the character after the handle is part of the match.
		name:  CheckUsername,
		re:    regexp.MustCompile(`(?m)(?:^|[^\w@./-])@([A-Za-z][A-Za-z0-9_]{3,30})(?:[^\w:./-]|$)`),
		group: 1,
		what:  "a platform handle in free text",
	},
	{
		name:  CheckEmail,
		re:    regexp.MustCompile(`(?i)\b[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,24}\b`),
		group: 0,
		what:  "an e-mail address",
	},
	{
		// sk-…{8,} is the bound of shared/logging; pk- and the GitHub and
		// Slack shapes are this scanner's own.
		name:  CheckAPIKey,
		re:    regexp.MustCompile(`\b(?:sk|pk)-[A-Za-z0-9_-]{8,}\b|\bgh[pousr]_[A-Za-z0-9]{20,}\b|\bxox[abpsr]-[A-Za-z0-9-]{10,}\b`),
		group: 0,
		what:  "an API key",
	},
}

// placeholderWords mark a value written to look like the thing a rule hunts
// without being one. The dotenv example, the fixtures of compose-lint and the
// examples in the documentation are full of them, and a scanner that fails on
// them teaches its operator to pass --ignore.
var placeholderWords = []string{
	"xxxx", "example", "placeholder", "redacted", "fixture", "dummy",
	"sample", "your-", "your_", "changeme", "not-a-secret", "notasecret",
	"ci-only",
}

// fixtureNames are the players a recording is allowed to carry (SEC-23,
// ADR-010 add. 1: mvctl record accepts CI sessions with these three only).
var fixtureNames = map[string]bool{
	"player-a": true, "player-b": true, "player-c": true, "ci-harness": true,
}

// isPlaceholder reports whether a matched value is a stand-in rather than
// somebody's identifier.
func isPlaceholder(value string) bool {
	lower := strings.ToLower(value)
	if fixtureNames[lower] {
		return true
	}
	for _, word := range placeholderWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return allSameDigit(value)
}

// allSameDigit reports whether a value is one digit repeated — 000000000 and
// 999999999 are how a document writes "a number goes here".
func allSameDigit(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' || value[i] != value[0] {
			return false
		}
	}
	return true
}

// Finding is one match: which rule fired, where, and the value it matched.
// Value is kept whole so that a caller can decide how to show it; nothing in
// this package prints it without Redact.
type Finding struct {
	Rule  string `json:"rule"`
	Path  string `json:"path"`
	Line  int    `json:"line"`
	What  string `json:"what"`
	Value string `json:"-"`
}

// Result is what one run of the scanner saw.
type Result struct {
	Roots    []string  `json:"roots"`
	Files    int       `json:"files_scanned"`
	Skipped  []string  `json:"skipped,omitempty"`
	Findings []Finding `json:"-"`
}

// Redact turns a matched value into something that can be written to a log, a
// pull request comment or a CI transcript: enough to find the line, not enough
// to be the identifier again.
func Redact(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return fmt.Sprintf("%s*** (%d chars)", string(runes[:2]), len(runes))
}

// Scan walks the roots and applies every rule to every text file it finds.
//
// An unreadable path is an error rather than a finding: a scan that silently
// skipped half a directory would report "nothing found" for the wrong reason.
func Scan(roots []string) (Result, error) {
	result := Result{Roots: roots}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			return scanFile(path, entry, &result)
		})
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// scanFile reads one file and appends what the rules found in it.
func scanFile(path string, entry fs.DirEntry, result *Result) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if info.Size() > maxFileSize {
		result.Skipped = append(result.Skipped,
			fmt.Sprintf("%s: larger than %d bytes", filepath.ToSlash(path), int64(maxFileSize)))
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if isBinary(data) {
		result.Skipped = append(result.Skipped, filepath.ToSlash(path)+": binary")
		return nil
	}
	result.Files++
	result.Findings = append(result.Findings, match(filepath.ToSlash(path), data)...)
	return nil
}

// isBinary reports whether a file is one the rules cannot read. A NUL byte in
// the head of the file is the same test grep makes, and it is enough here: the
// artefacts of the platform are JSON, JSONL, YAML and Markdown.
func isBinary(data []byte) bool {
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// match applies every rule to the content of one file.
func match(path string, data []byte) []Finding {
	var findings []Finding
	for _, r := range rules {
		for _, idx := range r.re.FindAllSubmatchIndex(data, -1) {
			start, end := idx[2*r.group], idx[2*r.group+1]
			if start < 0 {
				continue
			}
			value := string(data[start:end])
			if isPlaceholder(value) {
				continue
			}
			findings = append(findings, Finding{
				Rule:  r.name,
				Path:  path,
				Line:  lineOf(data, start),
				What:  r.what,
				Value: value,
			})
		}
	}
	return findings
}

// lineOf returns the 1-based line the byte at offset sits on.
func lineOf(data []byte, offset int) int {
	return bytes.Count(data[:offset], []byte("\n")) + 1
}
