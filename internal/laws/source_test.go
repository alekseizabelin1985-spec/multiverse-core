package laws

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceReadsLawsDocumentsAndNamesTheMisnamed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("w.v2.yaml", "two")
	write("w.v1.yaml", "one")
	write("README.md", "not a law")
	write("w.yml", "misnamed")
	write("World.v3.yaml", "misnamed")
	if err := os.Mkdir(filepath.Join(dir, "x.v1.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}

	docs, err := FileSource{Dir: dir}.Documents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Document{}
	var order []string
	for _, d := range docs {
		got[d.Name] = d
		order = append(order, d.Name)
	}
	if len(docs) != 4 {
		t.Fatalf("documents %v, want the two laws files and the two misnamed YAML files", order)
	}
	for i := 1; i < len(order); i++ {
		if order[i-1] > order[i] {
			t.Errorf("documents are not sorted by name: %v", order)
		}
	}
	if d := got["w.v1.yaml"]; d.World != "w" || d.Version != "v1" || string(d.Data) != "one" || d.NameErr != nil {
		t.Errorf("w.v1.yaml read as %+v", d)
	}
	if d := got["w.v2.yaml"]; d.Version != "v2" || string(d.Data) != "two" {
		t.Errorf("w.v2.yaml read as %+v", d)
	}
	for _, name := range []string{"w.yml", "World.v3.yaml"} {
		if d := got[name]; d.NameErr == nil || d.Data != nil {
			t.Errorf("%s read as %+v, want a name error and no data", name, d)
		}
	}
}

func TestFileSourceFailsOnAMissingDirectory(t *testing.T) {
	if _, err := (FileSource{Dir: filepath.Join(t.TempDir(), "none")}).Documents(context.Background()); err == nil {
		t.Error("a missing laws directory is not an error")
	}
}

func TestFileSourceStopsOnACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (FileSource{Dir: shippedDir}).Documents(ctx); err == nil {
		t.Error("a cancelled context did not stop the source")
	}
}

func TestFileName(t *testing.T) {
	if got := FileName("dark-forest-world", "v2"); got != "dark-forest-world.v2.yaml" {
		t.Errorf("FileName = %q", got)
	}
}
