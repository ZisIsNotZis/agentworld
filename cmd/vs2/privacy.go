package main

import (
	"encoding/json"
	"errors"
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

// EC-6 observation-boundary evaluation (spec section 7, VS2-EC-6). Three
// checks, mirroring the committed VS1 boundary:
//
//  1. structural: the transitive agentworld import closure of the request
//     builder (internal/cognition) and of the transport-free protocol surface
//     (internal/cognition/protocol) never reaches internal/world, the protocol
//     surface never reaches the network, and the world's closure never reaches
//     the cognition client;
//  2. adversarial sentinel: a fully populated sentinel observation encoded by
//     the allow-list encoder carries exactly the declared observation key set
//     and no privileged marker;
//  3. every recorded Role A request and every recorded allocation record
//     carries exactly the declared key set and no privileged marker.
const (
	requestBuilderPkg = "agentworld/internal/cognition"
	protocolPkg       = "agentworld/internal/cognition/protocol"
	worldPkg          = "agentworld/internal/world"
)

// forbiddenObservationMarkers are the privileged-value spellings the sentinel
// scan checks for.
var forbiddenObservationMarkers = []string{
	"tenure", "owner", "debt", "spoil", "allocator", "regeneration", "kin", "other", "parent", "genome",
}

// The allow-list is per contract and comes from internal/cognition, where the
// encoders live and where TestDeclaredSeamRequestKeySetsMatchTheEncoders proves
// each row equals what that contract writes. A single v1 list here certified the
// wrong contract: every natural-claim/v4 request failed the v1 key check, the
// scan returned before the privileged-marker test, and the band reported "key
// violations=1325" beside a hard-coded claim that the observation key set was
// exact. The boundary check was therefore silently disabled for the whole
// ticket-36 run, which is the defect this per-contract selection repairs.

// privacyScan is the accumulated adversarial-scan result.
type privacyScan struct {
	roleARequests int
	recordedRoleA int
	allocations   int
	markers       []string
	keyViolations []string
	// byContract counts the requests that passed the key check per contract, and
	// examined counts every request offered to the scan per contract, so the band
	// can state what was actually certified instead of asserting it.
	byContract map[cognition.ClaimContract]int
	examined   map[cognition.ClaimContract]int
	// markersScanned counts the requests whose observation block reached the
	// privileged-marker scan. A request that fails the key check never does, so
	// this number is the honest scope of the marker test.
	markersScanned int
}

func newPrivacyScan() *privacyScan {
	return &privacyScan{
		byContract: map[cognition.ClaimContract]int{},
		examined:   map[cognition.ClaimContract]int{},
	}
}

// scanRoleARequest checks one recorded Role A request against the key sets of the
// contract the request itself declares (prompt_template), then against the
// privileged-marker list. An undeclared contract is a violation, never a pass.
func (s *privacyScan) scanRoleARequest(key string, body []byte) {
	s.roleARequests++
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		s.keyViolations = append(s.keyViolations, key+":undecodable")
		return
	}
	contract, err := declaredContractOf(top)
	if err != nil {
		s.keyViolations = append(s.keyViolations, key+":"+err.Error())
		return
	}
	s.examined[contract]++
	keys, ok := cognition.DeclaredSeamRequestKeySet(contract)
	if !ok {
		s.keyViolations = append(s.keyViolations, key+":undeclared-contract")
		return
	}
	if !sameKeySet(top, keys.Request) {
		s.keyViolations = append(s.keyViolations, key+":request-keys:"+unexpectedKeys(top, keys.Request))
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
	if !sameKeySet(observation, keys.Observation) {
		s.keyViolations = append(s.keyViolations, key+":observation-keys:"+unexpectedKeys(observation, keys.Observation))
		return
	}
	s.byContract[contract]++
	s.markersScanned++
	lower := strings.ToLower(string(obs))
	for _, marker := range forbiddenObservationMarkers {
		if strings.Contains(lower, marker) {
			s.markers = append(s.markers, fmt.Sprintf("%s:%s", key, marker))
		}
	}
}

// declaredContractOf reads the contract a request declares about itself. The
// recorded bytes are what is being certified, so their own declaration selects
// the allow-list; a request that declares nothing is a violation.
func declaredContractOf(top map[string]json.RawMessage) (cognition.ClaimContract, error) {
	raw, ok := top["prompt_template"]
	if !ok {
		return "", fmt.Errorf("missing-prompt-template")
	}
	var template string
	if err := json.Unmarshal(raw, &template); err != nil {
		return "", fmt.Errorf("undecodable-prompt-template")
	}
	contract, err := cognition.ParseClaimContract(template)
	if err != nil || string(contract) != template {
		return "", fmt.Errorf("undeclared-contract:%s", template)
	}
	return contract, nil
}

// contractCounts renders a per-contract count map in declared contract order.
func contractCounts(counts map[cognition.ClaimContract]int) string {
	parts := make([]string, 0, len(counts))
	for _, row := range cognition.DeclaredSeamRequestKeySets() {
		if n := counts[row.Contract]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", row.Contract, n))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

// scanAllocation checks one recorded allocation record for privileged markers.
func (s *privacyScan) scanAllocation(key string, rec cognition.InterventionRecord) {
	s.allocations++
	body, err := json.Marshal(rec.Allocation)
	if err != nil {
		s.keyViolations = append(s.keyViolations, key+":undecodable-allocation")
		return
	}
	lower := strings.ToLower(string(body))
	for _, marker := range forbiddenObservationMarkers {
		if strings.Contains(lower, marker) {
			s.markers = append(s.markers, fmt.Sprintf("%s:%s", key, marker))
		}
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
// import paths reachable from root under srcRoot.
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

// privacyStructure runs the structural import proofs and returns the closure
// sizes plus any violation.
func privacyStructure(srcRoot string) (int, int, int, []string, error) {
	builder, err := transitiveAgentworldImports(srcRoot, requestBuilderPkg)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	protocols, err := transitiveAgentworldImports(srcRoot, protocolPkg)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	worldClosure, err := transitiveAgentworldImports(srcRoot, worldPkg)
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
	for pkg := range worldClosure {
		if pkg == requestBuilderPkg || (strings.HasPrefix(pkg, requestBuilderPkg+"/") && pkg != protocolPkg) {
			violations = append(violations, "world->"+pkg)
		}
	}
	sort.Strings(violations)
	return len(builder), len(protocols), len(worldClosure), violations, nil
}

func reachesWorld(pkg string) bool {
	return pkg == worldPkg || strings.HasPrefix(pkg, worldPkg+"/")
}

// sentinelSeamRequest is the fully populated sentinel observation fed through
// the allow-list encoder.
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

// privacyBand executes the EC-6 checks and reports the band.
func privacyBand(srcRoot string, runs []*cellRun) reportBand {
	const criterion = "VS2-EC-6 observation-bounded escalation: every Role A request is produced by the allow-list encoder from the frozen NaturalObservation; the import graph proves the request builder does not transitively import internal/world and the world does not import the client; the adversarial sentinel and every recorded request/allocation carry no privileged marker."

	builder, protocols, worldClosure, violations, err := privacyStructure(srcRoot)
	if err != nil {
		return band("VS2-EC-6", criterion, "FAIL", fmt.Sprintf("structural import scan error: %v", err))
	}
	scan := newPrivacyScan()
	// The sentinel is encoded under EVERY declared contract, not once under the
	// default: the point of the sentinel is to prove the encoder's output matches
	// the declared allow-list, so a contract whose encoder has drifted must fail
	// here rather than pass because another contract's sentinel was checked.
	for _, row := range cognition.DeclaredSeamRequestKeySets() {
		body, err := cognition.EncodeActorSeamRequestFor(sentinelSeamRequest(), row.Contract)
		if err != nil {
			return band("VS2-EC-6", criterion, "FAIL", fmt.Sprintf("sentinel encode error (%s): %v", row.Contract, err))
		}
		scan.scanRoleARequest("sentinel:"+string(row.Contract), body)
	}
	for _, cr := range runs {
		for _, call := range cr.calls {
			if call.Role == cognition.RoleActorDecision {
				scan.scanRoleARequest(fmt.Sprintf("%s#%d", cr.spec.String(), call.Seq), call.RequestBytes)
				scan.recordedRoleA++
			}
		}
		for _, rec := range cr.interventions {
			if rec.Kind == cognition.InterventionAllocation && rec.Allocation != nil {
				scan.scanAllocation(fmt.Sprintf("%s#%d", cr.spec.String(), rec.Seq), rec)
			}
		}
	}
	status := "PASS"
	switch {
	case len(violations) > 0 || len(scan.markers) > 0 || len(scan.keyViolations) > 0:
		status = "FAIL"
	case scan.recordedRoleA == 0:
		status = "FAIL"
	}
	// The evidence states what was certified, per contract, and never a claim the
	// check did not establish: the marker scan's scope is the requests that passed
	// the key check, which is why markersScanned is reported beside it.
	evidence := fmt.Sprintf(
		"import closure: request-builder=%d pkgs (no %s), protocol=%d pkgs (no net/%s), world=%d pkgs (no client); violations=%d [%s]; Role A requests scanned=%d (recorded=%d), declared key set matched by contract [%s] against declared contracts [%s]; privileged markers=%d over %d requests that passed the key check [%s]; allocation records scanned=%d, markers=%d; key violations=%d [%s]",
		builder, worldPkg, protocols, worldPkg, worldClosure,
		len(violations), joinOrNone(violations),
		scan.roleARequests, scan.recordedRoleA,
		contractCounts(scan.byContract), contractCounts(scan.examined),
		len(scan.markers), scan.markersScanned, joinOrNone(scan.markers),
		scan.allocations, len(scan.markers), len(scan.keyViolations), joinOrNone(scan.keyViolations))
	return band("VS2-EC-6", criterion, status, evidence)
}

// recheckEC6 recomputes the VS2-EC-6 band over recorded run directories. It
// reads each cell directory's hash-chained ledger and rebuilds the minimal cell
// run the band consumes (its spec for key prefixes, its call records and its
// intervention records), so the check certifies the bytes on disk - the same
// bytes the run produced - without running a cell or spending anything. A
// missing or undeserialisable ledger is an error, never an empty pass.
func recheckEC6(srcRoot string, roots []string) (reportBand, error) {
	var runs []*cellRun
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return reportBand{}, fmt.Errorf("%s: %w", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			spec, ok := parseRecordedCellName(entry.Name())
			if !ok {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			ledger := filepath.Join(dir, "calls.jsonl")
			if _, statErr := os.Stat(ledger); statErr != nil {
				// The no-AI control makes no model call, so it has no ledger to
				// certify. Every other arm must have one: a missing ledger there is
				// an error, never an empty pass.
				if spec.Arm == ArmA0 && errors.Is(statErr, os.ErrNotExist) {
					continue
				}
				return reportBand{}, fmt.Errorf("%s: %w", dir, statErr)
			}
			calls, err := cognition.ReadCallRecords(ledger)
			if err != nil {
				return reportBand{}, fmt.Errorf("%s: %w", dir, err)
			}
			interventions, err := cognition.ReadInterventionRecords(ledger)
			if err != nil {
				return reportBand{}, fmt.Errorf("%s: %w", dir, err)
			}
			runs = append(runs, &cellRun{spec: spec, dir: dir, calls: calls, interventions: interventions})
		}
	}
	if len(runs) == 0 {
		return reportBand{}, fmt.Errorf("no recorded cell directories under %v", roots)
	}
	return privacyBand(srcRoot, runs), nil
}

// parseRecordedCellName parses a recorded cell directory name of the declared
// shape <arm>-f<F>-s<S>[-r<N>|-w<N>] into its spec.
func parseRecordedCellName(name string) (cellSpec, bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		return cellSpec{}, false
	}
	spec := cellSpec{Arm: parts[0]}
	for _, part := range parts[1:] {
		if len(part) < 2 {
			return cellSpec{}, false
		}
		value := 0
		for _, r := range part[1:] {
			if r < '0' || r > '9' {
				return cellSpec{}, false
			}
			value = value*10 + int(r-'0')
		}
		switch part[0] {
		case 'f':
			spec.F = value
		case 's':
			spec.Seed = uint64(value)
		case 'r':
			spec.Replicate = value
		case 'w':
			spec.Workers = value
		default:
			return cellSpec{}, false
		}
	}
	return spec, spec.Arm != ""
}

// recheckReport is the written form of a recomputed band: the band itself plus
// what it was recomputed from, so a reader can bind the verdict to the bytes.
type recheckReport struct {
	Kind    string     `json:"kind"`
	Roots   []string   `json:"roots"`
	Band    reportBand `json:"band"`
	NoSpend bool       `json:"no_spend"`
}

// writeRecheckReport writes the recomputed band as JSON and as text.
func writeRecheckReport(jsonPath, textPath string, b reportBand) error {
	report := recheckReport{Kind: "vs2-band-recheck/1", Roots: nil, Band: b, NoSpend: true}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(body, '\n'), 0o600); err != nil {
		return err
	}
	text := fmt.Sprintf("VS2 band recheck (no cells run, no spend)\n%s %s\n%s\n", b.ID, b.Status, b.Evidence)
	return os.WriteFile(textPath, []byte(text), 0o600)
}
