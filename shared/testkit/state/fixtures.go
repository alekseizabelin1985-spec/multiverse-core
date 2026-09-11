package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"multiverse-core.io/shared/entity"
)

// FixtureFiles are the entity files of testdata/fixtures in the order a
// bootstrap reads them: a world before its regions, a region before what
// stands in it (state-and-mechanics.md §4.10).
var FixtureFiles = []string{"world.json", "region.json", "npc.json", "players.json"}

// LoadFixtures reads the world of testdata/fixtures out of dir, in bootstrap
// order, ready for Seed.
//
// It lives here rather than in each test because Seed is what consumes it and
// because the harness of T-018 seeds the same six entities: one loader means
// one answer to "which world are we testing against".
//
// Unknown fields are refused. A fixture with a misspelled attribute would
// otherwise decode into an entity that quietly lacks it, and every number
// derived from that entity — a stat, a hash, a snapshot — would be wrong in a
// way no assertion names.
func LoadFixtures(dir string) ([]*entity.Entity, error) {
	var all []*entity.Entity
	for _, name := range FixtureFiles {
		path := filepath.Join(dir, name)
		batch, err := loadEntityFile(path)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("testkit/state: %s holds no entities", path)
		}
		all = append(all, batch...)
	}
	return all, nil
}

func loadEntityFile(path string) ([]*entity.Entity, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a fixture path chosen by the test that calls this
	if err != nil {
		return nil, fmt.Errorf("testkit/state: read fixtures: %w", err)
	}
	// A checkout on Windows may hand text back with CRLF; the model does not
	// care, but going through the same normalisation as the fixture test keeps
	// the two readers of these files identical.
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var batch []*entity.Entity
	if err := dec.Decode(&batch); err != nil {
		return nil, fmt.Errorf("testkit/state: decode %s: %w", path, err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("testkit/state: %s holds more than one JSON document", path)
	}
	return batch, nil
}
