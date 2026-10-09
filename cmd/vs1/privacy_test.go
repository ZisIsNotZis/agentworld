package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// moduleRoot is the repository root (the harness runs from there; tests run
// from cmd/vs1).
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// TestPrivacyStructureRealTree pins the real import closure: the three
// structural proofs hold on the committed tree.
func TestPrivacyStructureRealTree(t *testing.T) {
	builder, protocols, world, violations, err := privacyStructure(moduleRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("unexpected structural violations: %v", violations)
	}
	if builder == 0 || protocols == 0 || world == 0 {
		t.Fatalf("empty closure: builder=%d protocol=%d world=%d", builder, protocols, world)
	}
}

// TestPrivacyStructureDetectsForbiddenImports is the mutation check for the
// structural guard: a synthetic tree whose request builder imports the world,
// whose protocol surface imports the network, and whose world imports the
// transport client must raise all three violations. With the guard disabled
// (the forbidden-import switch removed) this test would find none.
func TestPrivacyStructureDetectsForbiddenImports(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "internal/cognition/protocol/p.go", "package protocol\n")
	writePkg(t, root, "internal/cognition/c.go", "package cognition\n\nimport _ \"agentworld/internal/world\"\n")
	writePkg(t, root, "internal/world/w.go", "package world\n\nimport _ \"agentworld/internal/cognition\"\n")
	// A protocol-surface violation, kept in a separate synthetic package so it
	// does not change the closure shapes asserted below.
	writePkg(t, root, "internal/cognition/protocol/transport.go", "package protocol\n\nimport _ \"net/http\"\n")

	builder, protocols, world, violations, err := privacyStructure(root)
	if err != nil {
		t.Fatal(err)
	}
	if builder == 0 || protocols == 0 || world == 0 {
		t.Fatalf("empty closure: builder=%d protocol=%d world=%d", builder, protocols, world)
	}
	want := map[string]bool{
		"request-builder->agentworld/internal/world": false,
		"protocol->net/http":                         false,
		"world->agentworld/internal/cognition":       false,
	}
	for _, v := range violations {
		if _, ok := want[v]; ok {
			want[v] = true
		}
	}
	for v, found := range want {
		if !found {
			t.Fatalf("violation %q not detected; got %v", v, violations)
		}
	}
}

func writePkg(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSentinelRequestCarriesOnlyTheAllowList pins the adversarial sentinel:
// the allow-list encoder emits exactly the declared key sets and no privileged
// marker.
func TestSentinelRequestCarriesOnlyTheAllowList(t *testing.T) {
	body, err := encodeSentinel()
	if err != nil {
		t.Fatal(err)
	}
	scan := &privacyScan{}
	scan.scanRoleARequest("sentinel", body)
	if scan.roleARequests != 1 || len(scan.markers) != 0 || len(scan.keyViolations) != 0 {
		t.Fatalf("sentinel scan: markers=%v violations=%v", scan.markers, scan.keyViolations)
	}
}

// TestScanRoleADetectsPrivilegedMarker is the mutation check for the
// adversarial scan: a request with the exact allow-list key sets but a
// privileged spelling smuggled into a value must be caught.
func TestScanRoleADetectsPrivilegedMarker(t *testing.T) {
	body, err := encodeSentinel()
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatal(err)
	}
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(top["observation"], &obs); err != nil {
		t.Fatal(err)
	}
	obs["last_denial"] = json.RawMessage(`"owner:1001"`)
	mutated, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	top["observation"] = mutated
	mutant, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	scan := &privacyScan{}
	scan.scanRoleARequest("mutant", mutant)
	if len(scan.markers) == 0 {
		t.Fatal("privileged marker not detected")
	}
	if len(scan.keyViolations) != 0 {
		t.Fatalf("marker mutant unexpectedly has key violations: %v", scan.keyViolations)
	}
}

// TestScanRoleADetectsKeySetDrift is the mutation check for the allow-list key
// set: an extra observation field must be caught even without a known marker.
func TestScanRoleADetectsKeySetDrift(t *testing.T) {
	body, err := encodeSentinel()
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatal(err)
	}
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(top["observation"], &obs); err != nil {
		t.Fatal(err)
	}
	obs["smuggled"] = json.RawMessage(`7`)
	mutated, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	top["observation"] = mutated
	mutant, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	scan := &privacyScan{}
	scan.scanRoleARequest("mutant", mutant)
	if len(scan.keyViolations) == 0 {
		t.Fatal("observation key-set drift not detected")
	}
}

// TestScanRoleBDetectsDrift is the mutation check for the Role B aggregate key
// set.
func TestScanRoleBDetectsDrift(t *testing.T) {
	good := `{"schema_version":1,"role":"strategy-patch","prompt_template":"natural-strategy-patch/v1","model":"m","audit_hour":24,"run":{},"evidence":{},"parent_ref":{},"parameters":[],"sampling":{}}`
	scan := &privacyScan{}
	scan.scanRoleBRequest("good", []byte(good))
	if len(scan.keyViolations) != 0 || scan.roleBRequests != 1 {
		t.Fatalf("good packet flagged: %v", scan.keyViolations)
	}
	bad := `{"schema_version":1,"role":"strategy-patch","prompt_template":"natural-strategy-patch/v1","model":"m","audit_hour":24,"run":{},"evidence":{},"parent_ref":{},"parameters":[],"sampling":{},"genome":{}}`
	scan.scanRoleBRequest("bad", []byte(bad))
	if len(scan.keyViolations) == 0 {
		t.Fatal("patch packet key-set drift not detected")
	}
}
