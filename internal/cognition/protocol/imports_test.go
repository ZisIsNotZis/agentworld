package protocol

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProtocolSourcesImportNoTransportOrWorld is the structural guard for the
// extracted protocol surface (EC-6): internal/world consumes these types, so
// this package must never gain a network dependency, the model client, or a
// world dependency. The world package, in turn, must never import the
// transport-bearing cognition package (see the world-side twin
// TestWorldSourcesImportNoCognitionClient).
func TestProtocolSourcesImportNoTransportOrWorld(t *testing.T) {
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
			case path == "net" || strings.HasPrefix(path, "net/"):
				t.Fatalf("%s imports %q: the protocol surface must not depend on the network", name, path)
			case path == "agentworld/internal/cognition" || strings.HasPrefix(path, "agentworld/internal/cognition/"):
				t.Fatalf("%s imports %q: the protocol surface must not depend on the transport package", name, path)
			case path == "agentworld/internal/world" || strings.HasPrefix(path, "agentworld/internal/world/"):
				t.Fatalf("%s imports %q: the protocol surface must not depend on the world", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files checked")
	}
}
