package laws

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// shippedDir is the laws directory of the repository, seen from the package.
const shippedDir = "../../laws"

// The identifiers of the invariants of laws@v1: inv-01…inv-10 are the register
// of internal/mechanics, inv-11 is the guardian's (tasks.md T-205, revision 4).
// This package cannot read the register itself (depguard internal-laws); the
// integration test of the swarm compares the two (T-238).
var wantInvariants = []string{
	"inv-01", "inv-02", "inv-03", "inv-04", "inv-05", "inv-06",
	"inv-07", "inv-08", "inv-09", "inv-10", "inv-11",
}

func TestShippedLawsFileHoldsTheInvariantsOfV1(t *testing.T) {
	k, err := New(context.Background(), Config{Source: FileSource{Dir: shippedDir}})
	if err != nil {
		t.Fatal(err)
	}
	if p := k.Problems(); len(p) != 0 {
		t.Fatalf("the shipped laws do not load: %v", p)
	}
	doc, err := k.Get("dark-forest-world", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != StatusApproved || doc.CreatedBy != CreatedByAuthor || doc.BasedOn != "" {
		t.Errorf("v1 is %s by %s based on %q, want approved by author with no base",
			doc.Status, doc.CreatedBy, doc.BasedOn)
	}
	var invariants, declarative []string
	for _, law := range doc.Laws {
		switch law.Kind {
		case KindInvariant:
			invariants = append(invariants, law.ID)
		case KindDeclarative:
			declarative = append(declarative, law.ID)
			if law.Source != CreatedByAuthor {
				t.Errorf("%s: source %q, want author", law.ID, law.Source)
			}
		}
	}
	if !slices.Equal(invariants, wantInvariants) {
		t.Errorf("invariants %v, want %v", invariants, wantInvariants)
	}
	if !slices.Equal(declarative, []string{"law-1", "law-2"}) {
		t.Errorf("declarative laws %v, want law-1, law-2", declarative)
	}
	// inv-01…inv-10 are checked under their own identifiers, the keys of
	// mechanics.Invariant.ID; inv-11 under the key of the guardian (§10.4).
	for _, law := range doc.Laws {
		if law.Kind != KindInvariant {
			continue
		}
		want := law.ID
		if law.ID == "inv-11" {
			want = CheckLawsVersionCurrent
		}
		if law.Check != want {
			t.Errorf("%s: check %q, want %q", law.ID, law.Check, want)
		}
	}
	if !slices.Equal(doc.CheckKeys(), KnownChecks()) {
		t.Errorf("check keys %v, want the known checks %v", doc.CheckKeys(), KnownChecks())
	}
	cur, origin, err := k.CurrentFrom("dark-forest-world")
	if err != nil || cur.Version != "v1" || origin != OriginFiles {
		t.Errorf("CurrentFrom = %s from %s, %v; want v1 from files", cur.Version, origin, err)
	}
}

// validDoc is a document every rejection below starts from by changing one
// line.
const validDoc = `version: v2
world_id: w
status: approved
created_by: author
based_on: v1
created_at: 2026-09-09T00:00:00Z
laws:
  - { id: inv-01, kind: invariant, check: inv-01, text: "a" }
  - { id: law-1, kind: declarative, source: author, text: "b" }
`

func TestParseAcceptsTheValidDocument(t *testing.T) {
	doc, err := Parse("w.v2.yaml", []byte(validDoc), nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Number() != 2 || !slices.Equal(doc.IDs(), []string{"inv-01", "law-1"}) ||
		!slices.Equal(doc.CheckKeys(), []string{"inv-01"}) {
		t.Errorf("parsed %+v", doc)
	}
}

func TestParseAcceptsEveryStatusAndCreator(t *testing.T) {
	for _, status := range []string{StatusApproved, StatusPendingReview, StatusRolledBack, StatusSuperseded} {
		for _, creator := range []string{CreatedByAuthor, CreatedByBreach} {
			data := strings.Replace(validDoc, "status: approved", "status: "+status, 1)
			data = strings.Replace(data, "created_by: author", "created_by: "+creator, 1)
			doc, err := Parse("w.v2.yaml", []byte(data), nil)
			if err != nil || doc.Status != status || doc.CreatedBy != creator {
				t.Errorf("status %s, created by %s: %+v, %v", status, creator, doc, err)
			}
		}
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		name    string
		replace [2]string
		field   string
		unknown bool
	}{
		{"unknown key", [2]string{"based_on: v1", "based_om: v1"}, "", false},
		{"bad version", [2]string{"version: v2", "version: 2"}, "version", false},
		{"v0", [2]string{"version: v2", "version: v0"}, "version", false},
		{"bad world", [2]string{"world_id: w", "world_id: W/1"}, "world_id", false},
		{"no world", [2]string{"world_id: w\n", ""}, "world_id", false},
		{"bad status", [2]string{"status: approved", "status: ok"}, "status", false},
		{"bad creator", [2]string{"created_by: author", "created_by: player"}, "created_by", false},
		{"base not older", [2]string{"based_on: v1", "based_on: v2"}, "based_on", false},
		{"bad base", [2]string{"based_on: v1", "based_on: one"}, "based_on", false},
		{"no base above v1", [2]string{"based_on: v1\n", ""}, "based_on", false},
		{"no created_at", [2]string{"created_at: 2026-09-09T00:00:00Z\n", ""}, "created_at", false},
		{"no laws", [2]string{"laws:\n  - { id: inv-01, kind: invariant, check: inv-01, text: \"a\" }\n  - { id: law-1, kind: declarative, source: author, text: \"b\" }\n", "laws: []\n"}, "laws", false},
		{"no id", [2]string{"id: law-1, ", ""}, "laws[1].id", false},
		{"duplicate id", [2]string{"id: law-1", "id: inv-01"}, "laws[1].id", false},
		{"no text", [2]string{`text: "b"`, `text: ""`}, "laws[1].text", false},
		{"invariant without check", [2]string{"check: inv-01, ", ""}, "laws[0].check", false},
		{"declarative with check", [2]string{"kind: declarative, ", "kind: declarative, check: inv-02, "}, "laws[1].check", false},
		{"bad kind", [2]string{"kind: declarative", "kind: rule"}, "laws[1].kind", false},
		{"unknown check", [2]string{"check: inv-01", "check: dead_does_not_act"}, "laws[0].check", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(validDoc, tc.replace[0], tc.replace[1], 1)
			if data == validDoc {
				t.Fatalf("the case changes nothing: %q not in the document", tc.replace[0])
			}
			_, err := Parse("w.v2.yaml", []byte(data), nil)
			if !errors.Is(err, ErrInvalidDocument) {
				t.Fatalf("error %v, want ErrInvalidDocument", err)
			}
			var de *DocumentError
			if !errors.As(err, &de) {
				t.Fatalf("error %T, want *DocumentError", err)
			}
			if de.Name != "w.v2.yaml" || de.Field != tc.field {
				t.Errorf("rejected %s field %q, want w.v2.yaml field %q (%v)", de.Name, de.Field, tc.field, err)
			}
			if got := errors.Is(err, ErrUnknownCheck); got != tc.unknown {
				t.Errorf("errors.Is(ErrUnknownCheck) = %v, want %v (%v)", got, tc.unknown, err)
			}
		})
	}
}

func TestParseRejectsASecondDocument(t *testing.T) {
	_, err := Parse("w.v2.yaml", []byte(validDoc+"---\nversion: v3\n"), nil)
	if !errors.Is(err, ErrInvalidDocument) || !strings.Contains(err.Error(), "more than one") {
		t.Errorf("error %v, want a rejection of the second document", err)
	}
}

// The set of known checks belongs to the caller: the swarm may pass the keys
// of mechanics it actually has.
func TestParseTakesTheCallersChecks(t *testing.T) {
	if _, err := Parse("w.v2.yaml", []byte(validDoc), []string{"inv-02"}); !errors.Is(err, ErrUnknownCheck) {
		t.Errorf("inv-01 outside the caller's checks: %v, want ErrUnknownCheck", err)
	}
	if _, err := Parse("w.v2.yaml", []byte(validDoc), []string{"inv-01"}); err != nil {
		t.Errorf("inv-01 inside the caller's checks: %v", err)
	}
}

func TestVersionNumber(t *testing.T) {
	for in, want := range map[string]int{"v1": 1, "v12": 12} {
		if got, err := VersionNumber(in); err != nil || got != want {
			t.Errorf("VersionNumber(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1", "v0", "v01", "V1", "v1a", "v-1"} {
		if _, err := VersionNumber(in); err == nil {
			t.Errorf("VersionNumber(%q) accepted", in)
		}
	}
	if (LawsVersion{Version: "bad"}).Number() != 0 {
		t.Error("Number of a malformed version is not 0")
	}
}

func TestDocumentErrorText(t *testing.T) {
	withField := &DocumentError{Name: "a.v1.yaml", Field: "status", Reason: "bad"}
	if withField.Error() != "laws: invalid document a.v1.yaml: status: bad" {
		t.Errorf("%q", withField.Error())
	}
	without := &DocumentError{Name: "a.v1.yaml", Reason: "bad"}
	if without.Error() != "laws: invalid document a.v1.yaml: bad" {
		t.Errorf("%q", without.Error())
	}
	if errors.Is(withField, ErrUnknownCheck) {
		t.Error("a document error without a cause matches ErrUnknownCheck")
	}
}
