package world

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorldSourcesImportNoCognitionClient is the world-side twin of the
// protocol guard (EC-6): no non-test source file of the world package may
// import the transport-bearing internal/cognition package (or any of its
// subpackages other than the type-only protocol surface). The seam is an
// interface typed with protocol/sim/strategy values, wired by the caller, so
// the world never depends on the model client, the gateway stub, or the
// ledger. Test files may import the concrete seam to drive it.
func TestWorldSourcesImportNoCognitionClient(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch {
			case path == "agentworld/internal/cognition":
				t.Fatalf("%s imports %q: the world must not depend on the model client", name, path)
			case strings.HasPrefix(path, "agentworld/internal/cognition/") && path != "agentworld/internal/cognition/protocol":
				t.Fatalf("%s imports %q: only the type-only protocol surface may cross into the world", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files checked")
	}
}
