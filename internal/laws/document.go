// Package laws holds the laws of a world: the versioned documents
// laws/{world}.v{N}.yaml, the current version of each world, the strain counter
// and the author's bump (ADR-008, contracts.md C-12, swarm-llm-laws.md §12).
//
// The package is a library of the swarm, not the other way round (ADR-001
// p. 3): it imports no internal/* package. The current version comes from the
// state of the world through WorldVersions, an interface declared here and
// implemented by the WorldView of the swarm; the invariants the checks name are
// implemented by internal/mechanics and handed to the guardian by the swarm
// (§10.4). The guardian and mechanics do not import this package either.
package laws

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// The kinds of a law.
const (
	// KindInvariant is a law a check enforces; its Check names the key of the
	// implementation.
	KindInvariant = "invariant"
	// KindDeclarative is a law that only the text carries: it goes into the
	// <laws> section of a prompt and nothing checks it mechanically.
	KindDeclarative = "declarative"
)

// The statuses of a version (ADR-008 p. 6). MVP-1 has the enum and nothing
// else: the machine that moves a version between them is the breach of
// EPIC-007 (swarm-llm-laws.md §12.3).
const (
	StatusApproved      = "approved"
	StatusPendingReview = "pending_review"
	StatusRolledBack    = "rolled_back"
	StatusSuperseded    = "superseded"
)

// Who created a version: the author through mvctl laws bump, or the breach
// mechanics of E-B (the enum of world.laws.changed.created_by).
const (
	CreatedByAuthor = "author"
	CreatedByBreach = "breach"
)

// CheckLawsVersionCurrent is the key of inv-11. The guardian enforces it
// itself, not mechanics, which is why it is not an invariant identifier
// (swarm-llm-laws.md §12.1).
const CheckLawsVersionCurrent = "laws_version_current"

// KnownChecks returns the check keys a version may name when the caller does
// not pass its own set: the ten invariants of internal/mechanics/invariants.go
// and the key of the guardian. The package cannot read the register of
// mechanics (the depguard rule internal-laws), so the list is repeated here;
// the integration test of the swarm, which sees both packages, compares them
// (tasks.md T-205, T-238).
func KnownChecks() []string {
	return []string{
		"inv-01", "inv-02", "inv-03", "inv-04", "inv-05",
		"inv-06", "inv-07", "inv-08", "inv-09", "inv-10",
		CheckLawsVersionCurrent,
	}
}

// LawsVersion is one version of the laws of one world.
type LawsVersion struct {
	Version   string `yaml:"version" json:"version"`
	WorldID   string `yaml:"world_id" json:"world_id"`
	Status    string `yaml:"status" json:"status"`
	CreatedBy string `yaml:"created_by" json:"created_by"`
	// BasedOn is the version this one replaces; empty for v1.
	BasedOn   string    `yaml:"based_on,omitempty" json:"based_on,omitempty"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	Laws      []Law     `yaml:"laws" json:"laws"`
}

// Law is one law of a version.
type Law struct {
	ID   string `yaml:"id" json:"id"`
	Kind string `yaml:"kind" json:"kind"`
	Text string `yaml:"text" json:"text"`
	// Check is set on an invariant and only there.
	Check string `yaml:"check,omitempty" json:"check,omitempty"`
	// Source says where a declarative law came from (author).
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
}

// Number is the N of vN. It is 0 for a version that does not have that form,
// which a parsed document never is.
func (v LawsVersion) Number() int {
	n, err := VersionNumber(v.Version)
	if err != nil {
		return 0
	}
	return n
}

// IDs returns the identifiers of the laws in the order of the document.
func (v LawsVersion) IDs() []string {
	ids := make([]string, 0, len(v.Laws))
	for _, law := range v.Laws {
		ids = append(ids, law.ID)
	}
	return ids
}

// CheckKeys returns the check keys of the invariants of the version, in the
// order of the document. The swarm hands them to the guardian as
// guardian.Input.CheckKeys: the laws choose which checks run, mechanics
// implements them (swarm-llm-laws.md §10.4).
func (v LawsVersion) CheckKeys() []string {
	var keys []string
	for _, law := range v.Laws {
		if law.Kind == KindInvariant {
			keys = append(keys, law.Check)
		}
	}
	return keys
}

// ErrInvalidDocument is what every complaint about a laws document matches.
// The detail is in the DocumentError it wraps.
var ErrInvalidDocument = errors.New("laws: invalid document")

// ErrUnknownCheck marks a law whose check no implementation is registered
// under. A version that names one does not load, and /health says
// unknown_check (tasks.md T-205, consumed by T-239).
var ErrUnknownCheck = errors.New("laws: unknown check")

// DocumentError is one rejected field of a laws document. Name is the document
// (the file name), Field the key in the dotted form a reader finds in the YAML.
type DocumentError struct {
	Name   string
	Field  string
	Reason string
	err    error
}

func (e *DocumentError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("laws: invalid document %s: %s", e.Name, e.Reason)
	}
	return fmt.Sprintf("laws: invalid document %s: %s: %s", e.Name, e.Field, e.Reason)
}

// Is makes every DocumentError match ErrInvalidDocument, and an unknown check
// match ErrUnknownCheck as well.
func (e *DocumentError) Is(target error) bool {
	return target == ErrInvalidDocument || (e.err != nil && target == e.err)
}

var (
	versionRe = regexp.MustCompile(`^v([1-9][0-9]*)$`)
	// worldRe is the shape of a world identifier that can be part of a file
	// name: the blueprint validator builds laws/<world>.vN.yaml from it
	// (shared/agent, T-202).
	worldRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// VersionNumber parses vN.
func VersionNumber(version string) (int, error) {
	m := versionRe.FindStringSubmatch(version)
	if m == nil {
		return 0, fmt.Errorf("laws: version %q is not vN", version)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("laws: version %q: %w", version, err)
	}
	return n, nil
}

// Parse decodes and checks one laws document. name is what errors call the
// document; checks is the set of known check keys (KnownChecks when nil).
//
// Unknown keys are rejected: a misspelt based_on would otherwise read as a v1
// without a base, and a misspelt check as a declarative law.
func Parse(name string, data []byte, checks []string) (LawsVersion, error) {
	if checks == nil {
		checks = KnownChecks()
	}
	var doc LawsVersion
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return LawsVersion{}, &DocumentError{Name: name, Reason: err.Error()}
	}
	// A second document after --- would be silently ignored by Decode.
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return LawsVersion{}, &DocumentError{Name: name, Reason: "more than one YAML document"}
	}
	if err := check(name, doc, checks); err != nil {
		return LawsVersion{}, err
	}
	return doc, nil
}

// check is the validation of a parsed document, field by field, in the order a
// reader meets them.
func check(name string, doc LawsVersion, checks []string) error {
	bad := func(field, format string, args ...any) error {
		return &DocumentError{Name: name, Field: field, Reason: fmt.Sprintf(format, args...)}
	}
	n, err := VersionNumber(doc.Version)
	if err != nil {
		return bad("version", "%q is not vN", doc.Version)
	}
	if !worldRe.MatchString(doc.WorldID) {
		return bad("world_id", "%q is not a world identifier", doc.WorldID)
	}
	if !slices.Contains([]string{StatusApproved, StatusPendingReview, StatusRolledBack, StatusSuperseded}, doc.Status) {
		return bad("status", "%q is not one of approved, pending_review, rolled_back, superseded", doc.Status)
	}
	if doc.CreatedBy != CreatedByAuthor && doc.CreatedBy != CreatedByBreach {
		return bad("created_by", "%q is not author or breach", doc.CreatedBy)
	}
	if doc.BasedOn != "" {
		base, err := VersionNumber(doc.BasedOn)
		if err != nil {
			return bad("based_on", "%q is not vN", doc.BasedOn)
		}
		if base >= n {
			return bad("based_on", "%s is not older than %s", doc.BasedOn, doc.Version)
		}
	} else if n > 1 {
		return bad("based_on", "required for %s", doc.Version)
	}
	if doc.CreatedAt.IsZero() {
		return bad("created_at", "required")
	}
	if len(doc.Laws) == 0 {
		return bad("laws", "a version without laws")
	}
	seen := make(map[string]bool, len(doc.Laws))
	for i, law := range doc.Laws {
		field := fmt.Sprintf("laws[%d]", i)
		switch {
		case law.ID == "":
			return bad(field+".id", "required")
		case seen[law.ID]:
			return bad(field+".id", "%s appears twice", law.ID)
		case law.Text == "":
			return bad(field+".text", "required for %s", law.ID)
		}
		seen[law.ID] = true
		switch law.Kind {
		case KindInvariant:
			if law.Check == "" {
				return bad(field+".check", "required for the invariant %s", law.ID)
			}
			if !slices.Contains(checks, law.Check) {
				return &DocumentError{Name: name, Field: field + ".check",
					Reason: fmt.Sprintf("%s names %q, which no implementation is registered under", law.ID, law.Check),
					err:    ErrUnknownCheck}
			}
		case KindDeclarative:
			if law.Check != "" {
				return bad(field+".check", "the declarative law %s has a check", law.ID)
			}
		default:
			return bad(field+".kind", "%q is not invariant or declarative", law.Kind)
		}
	}
	return nil
}
