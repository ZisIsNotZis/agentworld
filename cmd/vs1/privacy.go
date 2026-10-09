package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// EC-6 privacy-boundary evaluation. The committed L1/L2 tests prove the
// boundary in the unit suite; the harness re-derives the same proofs at
// evaluation time so the reported band is evidence the matrix run actually
// produced, not a pointer to a test it never executed. Three checks:
//
//  1. structural: the transitive agentworld import closure of the request
//     builder (internal/cognition) and of the transport-free protocol surface
//     (internal/cognition/protocol) never reaches internal/world, the protocol
//     surface never reaches the network or the transport package, and the
//     transitive closure of internal/world never reaches the transport client;
//  2. adversarial sentinel: a fully populated sentinel observation encoded by
//     the allow-list encoder carries exactly the declared observation key set
//     and no privileged marker;
//  3. every recorded Role A request in every run's ledger carries exactly the
//     declared request/observation key set and no privileged marker, and every
//     recorded Role B packet carries exactly the declared aggregate key set.
const (
	requestBuilderPkg = "agentworld/internal/cognition"
	protocolPkg       = "agentworld/internal/cognition/protocol"
	worldPkg          = "agentworld/internal/world"
)

// forbiddenObservationMarkers are the privileged-value spellings the sentinel
// test checks for (internal/cognition/observation_test.go): tenure/owner of a
// patch, debts, spoilage, allocator state, another actor's rows, kin, or genome.
var forbiddenObservationMarkers = []string{
	"tenure", "owner", "debt", "spoil", "allocator", "regeneration", "kin", "other", "parent", "genome",
}

// declaredSeamRequestKeys is the exact allow-list top-level key set of a Role A
// request (internal/cognition/observation.go seamRequestWire).
var declaredSeamRequestKeys = []string{
	"schema_version", "role", "seam", "prompt_template", "model", "run",
	"actor", "observation", "strategy", "actions", "sampling",
}

// declaredObservationKeys is the exact allow-list key set of the Role A
// observation block (internal/cognition/observation.go observationWire).
var declaredObservationKeys = []string{
	"energy", "hunger", "bag_units", "capital", "wip", "invested_units",
	"last_build_hour", "granary_stock", "last_stored_meal_hour", "home_patch", "home_slots", "last_denial",
}

// declaredPatchRequestKeys is the exact allow-list key set of a Role B packet
// (internal/cognition/roleb.go patchRequestWire): the declared aggregate
// evidence packet plus the parameter contract and head ref, nothing private.
var declaredPatchRequestKeys = []string{
	"schema_version", "role", "prompt_template", "model", "audit_hour",
	"run", "evidence", "parent_ref", "parameters", "sampling",
}

// privacyScan is the accumulated adversarial-scan result.
type privacyScan struct {
	roleARequests int
	roleBRequests int
	markers       []string
	keyViolations []string
}

// scanRoleARequest checks one recorded Role A request against the declared
// allow-list key sets and the privileged-marker list.
func (s *privacyScan) scanRoleARequest(key string, body []byte) {
	s.roleARequests++
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		s.keyViolations = append(s.keyViolations, key+":undecodable")
		return
	}
	if !sameKeySet(top, declaredSeamRequestKeys) {
		s.keyViolations = append(s.keyViolations, key+":request-keys:"+unexpectedKeys(top, declaredSeamRequestKeys))
		return
	}
	obs, ok := top["observation"]
	if !ok {
		s.keyViolations = append(s.keyViolations, key+":missing-observation")
		return
	}
	var observation map[string]json.RawMessage
	if err := json.Unmarshal(obs, &observation); err != nil {
		s.keyViolations = append(s.keyViolations, key+":undecodable-observation")
		return
	}
	if !sameKeySet(observation, declaredObservationKeys) {
		s.keyViolations = append(s.keyViolations, key+":observation-keys:"+unexpectedKeys(observation, declaredObservationKeys))
		return
	}
	lower := strings.ToLower(string(obs))
	for _, marker := range forbiddenObservationMarkers {
		if strings.Contains(lower, marker) {
			s.markers = append(s.markers, fmt.Sprintf("%s:%s", key, marker))
		}
	}
}

// scanRoleBRequest checks one recorded Role B packet against the declared
// aggregate key set.
func (s *privacyScan) scanRoleBRequest(key string, body []byte) {
	s.roleBRequests++
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		s.keyViolations = append(s.keyViolations, key+":undecodable-patch")
		return
	}
	if !sameKeySet(top, declaredPatchRequestKeys) {
		s.keyViolations = append(s.keyViolations, key+":patch-keys:"+unexpectedKeys(top, declaredPatchRequestKeys))
	}
}

func sameKeySet(m map[string]json.RawMessage, want []string) bool {
	if len(m) != len(want) {
		return false
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func unexpectedKeys(m map[string]json.RawMessage, want []string) string {
	wantSet := map[string]bool{}
	for _, k := range want {
		wantSet[k] = true
	}
	var extra []string
	for k := range m {
		if !wantSet[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return strings.Join(extra, ",")
}

// transitiveAgentworldImports computes the transitive closure of agentworld
// import paths reachable from root under srcRoot. Non-agentworld imports
// (standard library) are recorded but not traversed: an internal package can
// only be reached through the agentworld module.
func transitiveAgentworldImports(srcRoot, root string) (map[string]bool, error) {
	visited := map[string]bool{}
	queue := []string{root}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if visited[pkg] {
			continue
		}
		visited[pkg] = true
		if !strings.HasPrefix(pkg, "agentworld/") {
			continue
		}
		imps, err := packageImports(filepath.Join(srcRoot, strings.TrimPrefix(pkg, "agentworld/")))
		if err != nil {
			return nil, err
		}
		queue = append(queue, imps...)
	}
	return visited, nil
}

// packageImports parses the non-test imports of a package directory.
func packageImports(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("privacy import scan: %w", err)
	}
	seen := map[string]bool{}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	return out, nil
}

// privacyStructure runs the three structural import proofs and returns the
// closure sizes plus any violation.
func privacyStructure(srcRoot string) (int, int, int, []string, error) {
	builder, err := transitiveAgentworldImports(srcRoot, requestBuilderPkg)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	protocols, err := transitiveAgentworldImports(srcRoot, protocolPkg)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	world, err := transitiveAgentworldImports(srcRoot, worldPkg)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	var violations []string
	for pkg := range builder {
		if reachesWorld(pkg) {
			violations = append(violations, "request-builder->"+pkg)
		}
	}
	for pkg := range protocols {
		switch {
		case reachesWorld(pkg):
			violations = append(violations, "protocol->"+pkg)
		case pkg == "net" || strings.HasPrefix(pkg, "net/"):
			violations = append(violations, "protocol->"+pkg)
		case pkg != protocolPkg && (pkg == requestBuilderPkg || strings.HasPrefix(pkg, requestBuilderPkg+"/")):
			violations = append(violations, "protocol->"+pkg)
		}
	}
	for pkg := range world {
		if pkg == requestBuilderPkg || (strings.HasPrefix(pkg, requestBuilderPkg+"/") && pkg != protocolPkg) {
			violations = append(violations, "world->"+pkg)
		}
	}
	sort.Strings(violations)
	return len(builder), len(protocols), len(world), violations, nil
}

func reachesWorld(pkg string) bool {
	return pkg == worldPkg || strings.HasPrefix(pkg, worldPkg+"/")
}

// sentinelSeamRequest is the fully populated sentinel observation fed through
// the allow-list encoder (the harness twin of the committed sentinel test). It
// carries only actor-observable values so the encoder cannot emit a privileged
// field without a schema-version edit.
func sentinelSeamRequest() cognition.ActorSeamRequest {
	home, err := sim.EntityRefValue(1001)
	if err != nil {
		panic(err)
	}
	return cognition.ActorSeamRequest{
		Seam:  cognition.SeamNaturalClaim,
		Actor: 5, Hour: 6, Version: 42,
		Observation: strategy.NaturalObservation{
			Actor: 5, Ref: strategy.NaturalRef{ID: "natural", Version: 7}, Hour: 6, WorldVersion: 42,
			Energy: sim.IntegerValue(6), Hunger: sim.IntegerValue(2), BagUnits: sim.IntegerValue(0),
			BagSource: home, Capital: sim.IntegerValue(1), Wip: sim.IntegerValue(0),
			GranaryStock: sim.IntegerValue(3), InvestedUnits: sim.IntegerValue(1),
			LastBuildHour: sim.IntegerValue(4), LastStoredMealHour: sim.IntegerValue(2),
			LastDenial: strategy.NaturalNoDenial,
			HomeSlots: []strategy.NaturalSlotView{
				{ID: 2001, Stock: sim.IntegerValue(1)},
				{ID: 2002, Stock: sim.IntegerValue(0)},
			},
		},
		Strategy: cognition.FrozenNaturalClaimParameters(strategy.FrozenNaturalPolicy7().Ref),
		Actions:  []string{"gather", "eat-stored", "provision", "wait"},
		Identity: cognition.RunIdentity{
			Fixture: cognition.FixtureIdentity{Patches: 2, R: 8, Founders: 12, Mechanism: 6, Births: true},
			Seed:    0, BaseRef: strategy.FrozenNaturalPolicy7().Ref, HeadRef: strategy.FrozenNaturalPolicy7().Ref,
		},
	}
}

// encodeSentinel feeds the sentinel observation through the allow-list
// encoder.
func encodeSentinel() ([]byte, error) {
	return cognition.EncodeActorSeamRequest(sentinelSeamRequest())
}

// privacyBand executes the EC-6 checks and reports the band.
func privacyBand(srcRoot string, runs []*cellRun) reportBand {
	const criterion = "EC-6 privacy boundary: every Role A request is produced by the allow-list encoder from the frozen NaturalObservation; the import-graph test proves the request builder does not transitively import internal/world and the world does not import the client; the adversarial sentinel test finds no privileged marker in any request; no other actor's private state appears; Role B packets contain only the declared aggregate fields."

	builder, protocols, world, violations, err := privacyStructure(srcRoot)
	if err != nil {
		return band("EC-6", criterion, "FAIL", fmt.Sprintf("structural import scan error: %v", err))
	}

	scan := &privacyScan{}
	sentinel, err := encodeSentinel()
	if err != nil {
		return band("EC-6", criterion, "FAIL", fmt.Sprintf("sentinel encode error: %v", err))
	}
	scan.scanRoleARequest("sentinel", sentinel)

	for _, cr := range runs {
		for _, call := range cr.calls {
			switch call.Role {
			case cognition.RoleActorDecision:
				scan.scanRoleARequest(fmt.Sprintf("%s#%d", cr.spec.String(), call.Seq), call.RequestBytes)
			case cognition.RoleStrategyPatch:
				scan.scanRoleBRequest(fmt.Sprintf("%s#%d", cr.spec.String(), call.Seq), call.RequestBytes)
			}
		}
	}

	status := "PASS"
	if len(violations) > 0 || len(scan.markers) > 0 || len(scan.keyViolations) > 0 {
		status = "FAIL"
	}
	evidence := fmt.Sprintf(
		"import closure: request-builder=%d pkgs (no %s), protocol=%d pkgs (no net/transport/%s), world=%d pkgs (no transport client); violations=%d %s; sentinel+recorded Role A requests scanned=%d (incl. 1 adversarial sentinel), observation key set exact, privileged markers=%d %s; Role B packets scanned=%d, aggregate key set exact, key violations=%d %s",
		builder, worldPkg, protocols, worldPkg, world,
		len(violations), joinOrNone(violations),
		scan.roleARequests, len(scan.markers), joinOrNone(scan.markers),
		scan.roleBRequests, len(scan.keyViolations), joinOrNone(scan.keyViolations))
	return band("EC-6", criterion, status, evidence)
}
