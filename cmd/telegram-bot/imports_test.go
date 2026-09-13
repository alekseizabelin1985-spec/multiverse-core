package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// allowedInternal is all the bot may reach of the platform code (component
// §3, tasks.md T-310): the client of C-08 and its wire types.
var allowedInternal = map[string]bool{
	"multiverse-core.io/internal/gateway/api":    true,
	"multiverse-core.io/internal/gateway/client": true,
}

// forbiddenModules are dependencies that would mean the gateway itself, not
// its client, has been pulled into the bot: the SQLite driver of links.db and
// the migrator.
var forbiddenModules = []string{"modernc.org/sqlite", "github.com/pressly/goose"}

// TestTheBotReachesOnlyTheGatewayClientAndAPI is the import boundary of the
// bot behind the depguard rule cmd-telegram-bot of .golangci.yml, its second
// line: depguard sees the direct imports of a file, this test the transitive
// dependencies of the shipped code — tests excluded, since a test of the bot
// may use FakeGateway, which runs the whole gateway in-process. It lives in the
// package of the binary, which it is about (review #1 of T-310, N-3; T-312).
func TestTheBotReachesOnlyTheGatewayClientAndAPI(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH: the boundary is checked where the module is built")
	}
	cmd := exec.Command(goBin, "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/telegram-bot/...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	deps := strings.Fields(string(out))
	sawBot := false
	for _, dep := range deps {
		if strings.HasPrefix(dep, "multiverse-core.io/cmd/telegram-bot/") {
			sawBot = true
		}
		if strings.HasPrefix(dep, "multiverse-core.io/internal/") && !allowedInternal[dep] {
			t.Errorf("the bot depends on %s: of internal/* only internal/gateway/client and internal/gateway/api are allowed", dep)
		}
		for _, mod := range forbiddenModules {
			if dep == mod || strings.HasPrefix(dep, mod+"/") {
				t.Errorf("the bot depends on %s: that is the gateway, not its client", dep)
			}
		}
	}
	if !sawBot {
		t.Fatalf("go list returned no package of the bot, the check proves nothing:\n%s", out)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}
