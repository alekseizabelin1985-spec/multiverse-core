package privacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRulesCatchWhatTheyMustCatch is the table the rules exist for: one line
// per shape an external identifier takes in an artefact of the platform
// (SEC-01, NFR-041).
func TestRulesCatchWhatTheyMustCatch(t *testing.T) {
	cases := map[string]struct {
		content string
		rule    string
		value   string
	}{
		"telegram id in json": {
			content: `{"external_id": "482913776", "player_id": "player-A"}`,
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"chat id in a yaml value": {
			content: "chat_id: 482913776\n",
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"user id in an env line": {
			content: "MV_TEST_USER_ID=482913776\n",
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"update id in a yaml value": {
			content: "update_id: 482913776\n",
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"id inside the from object of a telegram update": {
			content: `{"update_id":700,"message":{"from":{"id":482913776,"is_bot":false}}}`,
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"id inside the chat object of a telegram update": {
			content: `{"message":{"chat":{"id":482913776,"type":"private"}}}`,
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"id inside a user object, written as a string": {
			content: `{"user": {"id": "482913776"}}`,
			rule:    CheckExternalID,
			value:   "482913776",
		},
		"bot token": {
			content: "token: 482913776:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1\n",
			rule:    CheckBotToken,
			value:   "482913776:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1",
		},
		// The two bounds shared/logging redacts and this scanner used to
		// miss: bot identifiers are not always nine or ten digits.
		"bot token of a seven digit bot": {
			content: "token: 4829137:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1\n",
			rule:    CheckBotToken,
			value:   "4829137:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1",
		},
		"bot token of an eleven digit bot, bot prefixed": {
			content: "url: https://api.telegram.org/bot12345678901:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1/getMe\n",
			rule:    CheckBotToken,
			value:   "bot12345678901:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw1",
		},
		"username behind a key": {
			content: `{"username": "vasya_pupkin"}`,
			rule:    CheckUsername,
			value:   "vasya_pupkin",
		},
		"first name behind a key": {
			content: "first_name: Aleksei\n",
			rule:    CheckUsername,
			value:   "Aleksei",
		},
		"handle in free text": {
			content: "the tester @vasya_pupkin played the wolf scene\n",
			rule:    CheckUsername,
			value:   "vasya_pupkin",
		},
		"e-mail address": {
			content: "reported by tester.one@gmail.com\n",
			rule:    CheckEmail,
			value:   "tester.one@gmail.com",
		},
		"provider key": {
			content: "MV_LLM_API_KEY=sk-abcabcabcabcabcabcabc\n",
			rule:    CheckAPIKey,
			value:   "sk-abcabcabcabcabcabcabc",
		},
		// The bound of shared/logging: eight characters after sk-, not
		// sixteen, or a key the logger hides would be committable.
		"short provider key": {
			content: "MV_LLM_API_KEY=sk-abcdefgh\n",
			rule:    CheckAPIKey,
			value:   "sk-abcdefgh",
		},
		"github token": {
			content: "GITHUB_TOKEN=ghp_0123456789abcdefghijklmnopqrstuvwxyz\n",
			rule:    CheckAPIKey,
			value:   "ghp_0123456789abcdefghijklmnopqrstuvwxyz",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := scanContent(t, "leak.txt", tc.content)
			if len(result.Findings) == 0 {
				t.Fatalf("no finding in %q", tc.content)
			}
			var got *Finding
			for i := range result.Findings {
				if result.Findings[i].Rule == tc.rule {
					got = &result.Findings[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("rule %s did not fire; findings: %+v", tc.rule, result.Findings)
			}
			if got.Value != tc.value {
				t.Errorf("matched %q, want %q", got.Value, tc.value)
			}
			if got.Line != 1 {
				t.Errorf("line %d, want 1", got.Line)
			}
		})
	}
}

// TestRulesLeaveTheFixturesAlone is the other half of the table: what CI scans
// every day must stay green, otherwise the job teaches its operator to skip it.
func TestRulesLeaveTheFixturesAlone(t *testing.T) {
	cases := map[string]string{
		"fixture player":              `{"external_id": "player-A", "username": "player-B"}`,
		"placeholder token":           "MV_TELEGRAM_BOT_TOKEN=123456789:AAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n",
		"placeholder key":             "MV_LLM_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx\n",
		"example address":             "owner: owner@example.com\n",
		"repeated digits":             "chat_id: 000000000\n",
		"image digest":                "image: debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a70188\n",
		"go module path":              "require github.com/google/uuid v1.6.0\n",
		"short numeric setting":       "MV_LLM_MAX_TOKENS=2048\n",
		"ci credential":               "MINIO_ROOT_PASSWORD=ci-only-not-a-secret\n",
		"variable reference":          "MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD:?}\n",
		"fixture id in a chat object": `{"chat": {"id": 000000000, "type": "private"}}`,
		"object without an id":        `{"chat": {"type": "private", "title": "solo"}}`,
		"timestamp with a colon":      "started_at: 2026-09-09T12:00:00Z\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			result := scanContent(t, "fixture.txt", content)
			if len(result.Findings) != 0 {
				t.Errorf("%q produced %+v", content, result.Findings)
			}
		})
	}
}

// TestScanReportsTheRightPlace: a finding an operator cannot locate is a
// finding they will not fix.
func TestScanReportsTheRightPlace(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "recordings")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(nested, "solo-30.jsonl")
	content := "{\"turn\": 1}\n{\"turn\": 2}\n{\"chat_id\": 482913776}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Scan([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(result.Findings), result.Findings)
	}
	got := result.Findings[0]
	if got.Line != 3 {
		t.Errorf("line %d, want 3", got.Line)
	}
	if !strings.HasSuffix(got.Path, "recordings/solo-30.jsonl") {
		t.Errorf("path %q does not name the file", got.Path)
	}
	if result.Files != 1 {
		t.Errorf("scanned %d files, want 1", result.Files)
	}
}

// TestScanSkipsBinaryFiles: the rules read text, and a model or an archive that
// happens to contain the bytes of a pattern is not a leak.
func TestScanSkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	content := append([]byte("chat_id: 482913776\n"), 0x00, 0x01, 0x02)
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Scan([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("binary file produced %+v", result.Findings)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("%d skipped, want 1", len(result.Skipped))
	}
}

// TestScanFailsOnAMissingRoot: a scan of a directory that is not there must not
// look like a scan that found nothing.
func TestScanFailsOnAMissingRoot(t *testing.T) {
	if _, err := Scan([]string{filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Fatal("a missing root scanned without an error")
	}
}

// TestRedactKeepsTheValueOut is the rule the whole package obeys: the report
// says where, never what.
func TestRedactKeepsTheValueOut(t *testing.T) {
	const value = "482913776"
	got := Redact(value)
	if strings.Contains(got, value) {
		t.Fatalf("Redact(%q) = %q, the value survived", value, got)
	}
	if !strings.HasPrefix(got, "48") {
		t.Errorf("Redact(%q) = %q, an operator cannot recognise the finding", value, got)
	}
	if short := Redact("abc"); strings.Contains(short, "a") {
		t.Errorf("Redact(%q) = %q, the value survived", "abc", short)
	}
}

// scanContent writes one file and scans the directory it is in.
func scanContent(t *testing.T, name, content string) Result {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
