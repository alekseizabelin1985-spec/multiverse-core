package laws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Document is one laws document as a source holds it, not yet parsed.
//
// World and Version are what the source's own naming says; the keeper
// refuses a document whose content says otherwise, so that the file
// dark-forest-world.v2.yaml cannot quietly carry v3.
type Document struct {
	// Name is what errors and mvctl laws show call the document: the file
	// name for FileSource.
	Name    string
	World   string
	Version string
	Data    []byte
	// NameErr is set when the source found something that looks like a laws
	// document but is not named like one. The keeper reports it instead of
	// ignoring the file: a misnamed version is a version nobody loads.
	NameErr error
}

// Source is where the keeper reads laws documents from.
type Source interface {
	// Documents returns every laws document the source holds. An error means
	// the source itself could not be read; a bad document is a Document the
	// keeper rejects, not an error here.
	Documents(ctx context.Context) ([]Document, error)
}

// ObjectSource is the source of the versions the breach writes: objects
// laws-{world}/vN.json in the object store (ADR-008 p. 1). MVP-1 declares the
// interface only; the implementation arrives with E-B (swarm-llm-laws.md
// §12.3).
type ObjectSource interface {
	Source
	// Put stores the document of a version.
	Put(ctx context.Context, world, version string, data []byte) error
}

// FileSource reads laws/{world}.v{N}.yaml from a directory (MV_LAWS_DIR).
type FileSource struct {
	Dir string
}

// fileNameRe is {world}.v{N}.yaml, the name shared/agent builds from the
// laws_ref laws/<world>@vN of a blueprint (T-202).
var fileNameRe = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)\.(v[1-9][0-9]*)\.yaml$`)

// Documents implements Source. Files that are not YAML (a README) are not
// laws documents and are skipped; a YAML file with another name is returned
// with NameErr.
func (s FileSource) Documents(ctx context.Context) ([]Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("laws: read the laws directory: %w", err)
	}
	var docs []Document
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		m := fileNameRe.FindStringSubmatch(name)
		if m == nil {
			docs = append(docs, Document{Name: name,
				NameErr: errors.New("the name is not <world>.v<N>.yaml")})
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
		if err != nil {
			return nil, fmt.Errorf("laws: read %s: %w", name, err)
		}
		docs = append(docs, Document{Name: name, World: m[1], Version: m[2], Data: data})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })
	return docs, nil
}

// FileName is the name FileSource gives the document of a version.
func FileName(world, version string) string {
	return world + "." + version + ".yaml"
}
