package env

import (
	"os"
	"strings"
	"testing"
)

func parse(t *testing.T, body string) []Entry {
	t.Helper()
	entries, err := ParseExample(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseExample: %v", err)
	}
	return entries
}

func TestParseExampleReadsTheFormsTheFileUses(t *testing.T) {
	entries := parse(t, `# comment without an assignment
# ===== platform =====
MV_ENV=dev                           # dev|ci|prod
MV_MINIO_ACCESS_KEY=
MV_LLM_URL=http://host.docker.internal:1234/v1#fragment
#MV_OLLAMA_URL=http://127.0.0.1:11434  # only with the ollama provider
MV_QUOTED="value # not a comment"
# ===== legacy =====
CHROMA_HOST=chromadb
`)

	want := []Entry{
		{Name: "MV_ENV", Value: "dev", Section: "platform", Line: 3},
		{Name: "MV_MINIO_ACCESS_KEY", Value: "", Section: "platform", Line: 4},
		{Name: "MV_LLM_URL", Value: "http://host.docker.internal:1234/v1#fragment", Section: "platform", Line: 5},
		{Name: "MV_OLLAMA_URL", Value: "http://127.0.0.1:11434", Commented: true, Section: "platform", Line: 6},
		{Name: "MV_QUOTED", Value: "value # not a comment", Section: "platform", Line: 7},
		{Name: "CHROMA_HOST", Value: "chromadb", Section: "legacy", Line: 9},
	}
	if len(entries) != len(want) {
		t.Fatalf("ParseExample returned %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for i, w := range want {
		if entries[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, entries[i], w)
		}
	}
}

func TestCheckExampleReportsBothDirections(t *testing.T) {
	isolate(t)
	Declare("MV_PRESENT", "value", "doc")
	Declare("MV_MISSING", "value", "doc")
	Declare("MV_TOKEN", "", "doc", Secret())
	Declare("MV_NEEDED", "", "doc", Required())
	Declare("MV_CHOICE", "one", "doc", OneOf("one", "two"))
	Declare("MV_OPTIONAL", "", "doc")
	DeclareDeprecated("MV_GONE", "replaced by MV_PRESENT")

	problems := CheckExample(parse(t, `# ===== platform =====
MV_PRESENT=value
MV_TOKEN=leaked-token
MV_NEEDED=filled-in
MV_CHOICE=three
MV_GONE=still-here
MV_UNDECLARED=surprise
#MV_OPTIONAL=documented but not set
MV_PRESENT=again
# ===== legacy (профиль legacy; as-is имена, вне реестра shared/env) =====
LEGACY_ONLY=ignored
MV_LEGACY_LOOKING=also ignored
`))

	byName := make(map[string]string, len(problems))
	for _, p := range problems {
		byName[p.Name] += p.Text + ";"
	}
	cases := []struct{ name, want string }{
		{"MV_MISSING", "missing from .env.example"},
		{"MV_UNDECLARED", "not declared through env.Declare"},
		{"MV_TOKEN", "declared secret"},
		{"MV_NEEDED", "required and without a default"},
		{"MV_CHOICE", "outside the declared set"},
		{"MV_GONE", "declared deprecated"},
		{"MV_PRESENT", "assigned twice"},
	}
	for _, c := range cases {
		if !strings.Contains(byName[c.name], c.want) {
			t.Errorf("%s: problems %q, want one mentioning %q", c.name, byName[c.name], c.want)
		}
	}
	for _, name := range []string{"LEGACY_ONLY", "MV_LEGACY_LOOKING"} {
		if _, flagged := byName[name]; flagged {
			t.Errorf("%s of the legacy section is checked, the whole section must be skipped", name)
		}
	}
	if _, flagged := byName["MV_OPTIONAL"]; flagged {
		t.Error("a commented entry does not count as documented")
	}
	if strings.Contains(byName["MV_TOKEN"], "leaked-token") {
		t.Errorf("the report carries the value of a secret: %q", byName["MV_TOKEN"])
	}

	for i := 1; i < len(problems); i++ {
		if problems[i-1].Name > problems[i].Name {
			t.Fatalf("problems are not sorted by name: %v", problems)
		}
	}
}

func TestCheckExampleAcceptsACompleteFile(t *testing.T) {
	isolate(t)
	Declare("MV_PRESENT", "value", "doc")
	Declare("MV_TOKEN", "", "doc", Secret())
	Declare("MV_CONDITIONAL", "", "doc", RequiredWhen("MV_PRESENT", "other"))

	problems := CheckExample(parse(t, `# ===== platform =====
MV_PRESENT=value
MV_TOKEN=
#MV_CONDITIONAL=http://127.0.0.1:11434
`))
	if len(problems) != 0 {
		t.Fatalf("CheckExample = %v, want no problems", problems)
	}
}

// The manifest and the committed .env.example must agree: the check that
// mvctl env check (T-010) and the contracts job (T-012) run, run here first.
func TestRepositoryExampleMatchesTheManifest(t *testing.T) {
	problems, err := CheckExampleFile("../../" + ExamplePath)
	if err != nil {
		t.Fatalf("CheckExampleFile: %v", err)
	}
	for _, p := range problems {
		t.Errorf("%s", p)
	}
}

// The LLM block of infrastructure.md v0.3 §4.2, and the two keys that stopped
// being provider names with it (ADR-005 add. 2 p. 1).
func TestRepositoryExampleCarriesTheLLMBlockAndNoRetiredKeys(t *testing.T) {
	f, err := os.Open("../../" + ExamplePath)
	if err != nil {
		t.Fatalf("open %s: %v", ExamplePath, err)
	}
	defer func() { _ = f.Close() }()
	entries, err := ParseExample(f)
	if err != nil {
		t.Fatalf("ParseExample: %v", err)
	}

	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.Name] = true
	}
	for _, name := range []string{
		"MV_LLM_PROVIDER", "MV_LLM_URL", "MV_LLM_API_KEY", "MV_LLM_MODELS_DIR", "MV_LLM_CLOUD_ENABLED",
	} {
		if !present[name] {
			t.Errorf("%s is missing from %s", name, ExamplePath)
		}
	}
	for name := range DeprecatedNames() {
		if present[name] {
			t.Errorf("%s was retired and must not be in %s", name, ExamplePath)
		}
	}
}
