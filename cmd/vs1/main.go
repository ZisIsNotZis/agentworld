package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/world"
)

func main() {
	var (
		arms       = flag.String("arms", strings.Join(declaredArms, ","), "comma-separated arms to run")
		cells      = flag.String("cells", "", "comma-separated cells as f<founders>-s<seed> (default: the 12 declared cells)")
		outJSON    = flag.String("out", ".tmp/21-vs1-evaluation.json", "evaluation JSON output path")
		outText    = flag.String("out-txt", ".tmp/21-vs1-evaluation.txt", "evaluation text output path")
		root       = flag.String("root", ".tmp/21-vs1-runs", "per-cell run directory root")
		recorded   = flag.String("recorded", ".tmp", "recorded ticket-19 evidence directory")
		srcRoot    = flag.String("src", ".", "module root for the EC-6 import-closure scan")
		runProbeF  = flag.Bool("probe", true, "run the declared scale/ceiling probe")
		probeCalls = flag.Uint64("probe-max-calls", 32, "lowered MaxCalls of the probe")
		budgetMin  = flag.Int("budget-min", 60, "wall-clock budget for the matrix in minutes; unrun cells are declared skipped")
		parallel   = flag.Int("parallel", 6, "cells to run concurrently (each cell is deterministic and writes its own directory)")
		runTwins   = flag.Bool("twin", true, "run the declared workers-4 determinism twins (EC-8)")
		runFixture = flag.Bool("fixture", true, "run the scripted-proposer fixture cell (EC-5a)")
		reuse      = flag.String("reuse", "", "prior evaluation JSON: reuse its committed matrix cells (restore each final bundle) instead of re-simulating; A0 cells are re-run because the control keeps no final bundle")
	)
	flag.Parse()
	if *parallel < 1 {
		*parallel = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*budgetMin)*time.Minute)
	defer cancel()

	armList := splitList(*arms)
	cellList := parseCells(*cells)
	if len(cellList) == 0 {
		for _, c := range VS1DeclaredCells() {
			cellList = append(cellList, [2]uint64{c[0], c[1]})
		}
	}
	total := len(armList) * len(cellList)

	priorCells := map[string]cellReport{}
	var priorEval evaluation
	if *reuse != "" {
		pe, err := readEvaluation(*reuse)
		if err != nil {
			fail("read reuse evaluation %s: %v", *reuse, err)
		}
		priorEval = pe
		for _, c := range pe.Cells {
			priorCells[cellSpec{Arm: c.Arm, F: c.F, Seed: c.Seed, Workers: c.Workers}.String()] = c
		}
	} else if err := os.RemoveAll(*root); err != nil {
		fail("clean run root: %v", err)
	}

	var runs []*cellRun
	var extraRuns []*cellRun
	var skipped []string
	declared := make([]string, 0, total)
	for _, arm := range armList {
		for _, c := range cellList {
			declared = append(declared, fmt.Sprintf("%s-f%d-s%d", arm, c[0], c[1]))
		}
	}
	tasks := make([]cellSpec, 0, total)
	for _, arm := range armList {
		for _, c := range cellList {
			tasks = append(tasks, cellSpec{Arm: arm, F: int(c[0]), Seed: c[1]})
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, *parallel)
	for _, spec := range tasks {
		label := spec.String()
		if ctx.Err() != nil {
			skipped = append(skipped, label+":budget")
			continue
		}
		wg.Add(1)
		go func(spec cellSpec, label string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				mu.Lock()
				skipped = append(skipped, label+":budget")
				mu.Unlock()
				return
			}
			fp := declaredFixture(spec.F, spec.Seed)
			var cr *cellRun
			var err error
			if prior, ok := priorCells[label]; ok && *reuse != "" && hasFinalBundle(*root, spec) {
				cr, err = loadCell(prior, spec, fp, *root)
			} else {
				if *reuse != "" {
					_ = os.RemoveAll(filepath.Join(*root, spec.String()))
				}
				cr, err = runCell(ctx, spec, fp, *root, nil)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				skipped = append(skipped, label+":"+err.Error())
				return
			}
			runs = append(runs, cr)
		}(spec, label)
	}
	wg.Wait()
	sortRuns(runs)

	var twins []twinReport
	if *runTwins {
		for _, arm := range []string{ArmA0, ArmA1} {
			tr, trun, err := runTwin(ctx, arm, *root, runs)
			if err != nil {
				skipped = append(skipped, "twin-"+arm+":"+err.Error())
				continue
			}
			extraRuns = append(extraRuns, trun)
			twins = append(twins, tr)
		}
	}

	var fixtures []fixtureReport
	if *runFixture {
		frs, fruns, err := runScriptedFixtures(ctx, *root)
		if err != nil {
			skipped = append(skipped, "fixtures:"+err.Error())
		} else {
			extraRuns = append(extraRuns, fruns...)
			fixtures = frs
		}
	}

	var probe *probeReport
	if *runProbeF {
		if *reuse != "" {
			_ = os.RemoveAll(filepath.Join(*root, "probe"))
		}
		p, err := runProbe(ctx, *root, *probeCalls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		} else {
			probe = p
		}
	}

	var replay map[string]string
	if *reuse != "" {
		// The reuse path re-derives the EC-3 replay subset from the reloaded
		// bundles and ledgers; a reuse-mode verdict is never carried over from a
		// prior evaluation's own claim. The replay directory is rebuilt so a
		// second reuse run does not collide with the first.
		_ = os.RemoveAll(filepath.Join(*root, "replay"))
	}
	replay = runReplaySubset(ctx, runs, *root, armList, *parallel)
	if *reuse != "" {
		replay = mergeReuseReplay(priorEval, replay)
	}
	eval := buildEvaluation(runs, extraRuns, *recorded, skipped, declared, probe, replay, *srcRoot, fixtures, twins)
	eval.Revision = revisionString(*reuse)
	if err := writeEvaluation(eval, *outJSON, *outText); err != nil {
		fail("write evaluation: %v", err)
	}
	fmt.Print(renderText(eval))
}

// hasFinalBundle reports whether a cell directory carries a committed final
// bundle, the precondition for the reuse load path.
func hasFinalBundle(root string, spec cellSpec) bool {
	_, err := os.Stat(filepath.Join(root, spec.String(), "natural-final.bundle"))
	return err == nil
}

// readEvaluation reads a prior evaluation JSON.
func readEvaluation(path string) (evaluation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return evaluation{}, err
	}
	var ev evaluation
	if err := json.Unmarshal(data, &ev); err != nil {
		return evaluation{}, err
	}
	return ev, nil
}

// mergeReuseReplay guards the -reuse path against stale evidence. The
// re-derived map is the only source of a verdict; a prior evaluation's own
// replay subset is treated as a claim to re-check, never as a fact. Any prior
// cell the re-derivation did not reproduce is recorded as not-re-derived (and
// so fails EC-3) rather than assumed identical.
func mergeReuseReplay(prior evaluation, rederived map[string]string) map[string]string {
	for _, k := range prior.ReplaySubset {
		if _, ok := rederived[k]; !ok {
			rederived[k] = "not-re-derived:stale-prior-verdict"
		}
	}
	return rederived
}

// runTwin runs the declared workers-4 determinism twin of one arm and compares
// it against the workers-1 matrix counterpart.
func runTwin(ctx context.Context, arm, root string, runs []*cellRun) (twinReport, *cellRun, error) {
	spec := cellSpec{Arm: arm, F: twinFounders, Seed: twinSeed, Workers: twinWorkers}
	_ = os.RemoveAll(filepath.Join(root, spec.String()))
	cr, err := runCell(ctx, spec, twinFixture(twinFounders, twinSeed), root, nil)
	if err != nil {
		return twinReport{}, nil, err
	}
	tr := twinReport{Ran: true, Arm: arm, F: twinFounders, Seed: twinSeed, Workers: twinWorkers,
		W4TipHash: cr.accepted.TipHash, W4BranchDigest: cr.branchDigest}
	w1 := findRun(runs, arm, twinFounders, twinSeed)
	if w1 == nil || !w1.haveAccepted {
		tr.Note = "workers-1 counterpart not in the matrix run"
		return tr, cr, nil
	}
	tr.W1TipHash = w1.accepted.TipHash
	tr.W1BranchDigest = w1.branchDigest
	tr.Identical = w1.accepted.TipHash == cr.accepted.TipHash && w1.branchDigest == cr.branchDigest
	return tr, cr, nil
}

func findRun(runs []*cellRun, arm string, f int, seed uint64) *cellRun {
	for _, cr := range runs {
		if cr.spec.Arm == arm && cr.spec.F == f && cr.spec.Seed == seed && cr.spec.Workers <= 1 {
			return cr
		}
	}
	return nil
}

// runScriptedFixtures runs the declared scripted-proposer fixture cells
// through the live lifecycle and reports each one's admitted/applied/measured/
// rolled-back counts, its measurement outcome, its divergent-parent sandbox
// evidence, and its reported final-bundle save result.
func runScriptedFixtures(ctx context.Context, root string) ([]fixtureReport, []*cellRun, error) {
	var reports []fixtureReport
	var runs []*cellRun
	for _, arm := range []string{ArmA2f, ArmA2g} {
		spec := cellSpec{Arm: arm, F: scriptedFixtureFounders, Seed: scriptedFixtureSeed, Workers: 1}
		_ = os.RemoveAll(filepath.Join(root, spec.String()))
		cr, err := runCell(ctx, spec, scriptedFixture(), root, nil)
		if err != nil {
			return reports, runs, fmt.Errorf("%s: %w", arm, err)
		}
		reports = append(reports, fixtureReport{
			Label: fixtureLabelFor(arm), Arm: arm, F: spec.F, Seed: spec.Seed, Ran: true,
			Proposed: cr.roleBProposed, Admitted: cr.roleBValidated, Applied: cr.roleBApplied,
			Measured: cr.roleBMeasured, RolledBack: cr.roleBRollback, MeasuredWindowHours: cr.roleBMeasureHour,
			DivergentParentSandbox: cr.roleBDivergentParent,
			MeasurementKeptHead:    cr.roleBMeasured > 0 && cr.roleBRollback == 0,
			FinalBundleError:       cr.finalBundleError,
		})
		runs = append(runs, cr)
	}
	return reports, runs, nil
}

// buildEvaluation assembles the report. runs are the declared matrix cells;
// extraRuns are the labeled twin and fixture runs (kept out of the matrix
// bands and the fidelity vector, but included in the EC-6 request scan).
func buildEvaluation(runs, extraRuns []*cellRun, recordedDir string, skipped, declared []string, probe *probeReport, replay map[string]string, srcRoot string, fixtures []fixtureReport, twins []twinReport) evaluation {
	reports := make([]cellReport, 0, len(runs))
	for _, cr := range runs {
		reports = append(reports, cellReportOf(cr))
	}
	for i := range fixtures {
		fixtures[i].UnscriptedA2Proposed, fixtures[i].UnscriptedA2Applied, fixtures[i].UnscriptedA2Rolled,
			fixtures[i].UnscriptedA2Rejected, fixtures[i].UnscriptedA2Validated = unscriptedA2Counts(runs)
	}
	replayKeys := make([]string, 0, len(replay))
	for k := range replay {
		replayKeys = append(replayKeys, k)
	}
	var mismatches []string
	for k, v := range replay {
		if v != "identical" {
			mismatches = append(mismatches, k+":"+v)
		}
	}
	sortStrings(replayKeys)
	allRuns := append(append([]*cellRun{}, runs...), extraRuns...)
	ev := evaluation{
		Status:           "synthetic/model-conditional",
		Label:            evaluationLabel,
		HonestLimit:      "the hermetic FakeClient proves the mechanism is load-bearing; it does not and cannot claim a real model is load-bearing (no spend authorised, spec section 16). cost_micros is a modelled price-table charge over the fake usage, not real spend; no gateway was contacted",
		Arms:             declaredArms,
		Inventory:        inventory{Declared: len(declared), Ran: len(runs), Skipped: skipped},
		Cells:            reports,
		Fidelity:         buildFidelity(runs),
		Probe:            probe,
		Bands:            evaluateBands(runs, allRuns, recordedDir, replay, probe, srcRoot, fixtures, twins),
		ReplaySubset:     replayKeys,
		ReplayMismatches: mismatches,
	}
	ev.Fixtures = fixtures
	ev.Twins = twins
	return ev
}

// unscriptedA2Counts sums the honest-null Role B counts of the declared A2
// cells, kept separately visible beside the scripted-proposer fixture.
func unscriptedA2Counts(runs []*cellRun) (proposed, applied, rolledBack, rejected, validated int) {
	for _, cr := range runs {
		if cr.spec.Arm != ArmA2 {
			continue
		}
		proposed += cr.roleBProposed
		applied += cr.roleBApplied
		rolledBack += cr.roleBRollback
		rejected += cr.roleBRejected
		validated += cr.roleBValidated
	}
	return
}

// runProbe runs the declared scale/ceiling probe (spec section 11.4): P=32,
// the full 256-founder block, a lowered MaxCalls, the A1 arm. Its required
// outcome is the typed fail-loud cognitive ceiling halt with the ledger intact
// and replayable.
func runProbe(ctx context.Context, root string, maxCalls uint64) (*probeReport, error) {
	fp := probeFixture(0)
	spec := cellSpec{Arm: ArmA1, F: 0, Seed: 0}
	dir, err := prepareRunDir(filepath.Join(root, "probe"), spec)
	if err != nil {
		return nil, err
	}
	f, _, _, err := buildBranchRunner(ctx, fp, dir)
	if err != nil {
		return nil, err
	}
	budget := cognition.CognitiveBudget{MaxCalls: maxCalls, MaxPromptTokens: 1 << 60, MaxCompletionTokens: 1 << 60, MaxCostMicros: 1 << 60}
	stack, err := wireStack(f, spec, fp, dir, budget, nil)
	if err != nil {
		return nil, err
	}
	rep := &probeReport{Ran: true, Patches: fp.Patches, Founders: defaultFounders(fp), MaxCalls: maxCalls}
	for {
		checks := f.Checkpoints()
		if len(checks) == 0 {
			break
		}
		if checks[len(checks)-1].Hour >= fp.Hours {
			break
		}
		_, err := f.Step(ctx)
		if err != nil {
			var hit *cognition.CognitiveCeilingHitError
			if asCeiling(err, &hit) {
				rep.Ceiling = string(hit.Hit.Ceiling)
				rep.HitHour = hit.Hit.Hour
				rep.Threshold = hit.Hit.Threshold
				rep.Observed = hit.Hit.Observed
				rep.P15Verdict = "declared compute/authorised-spend ceiling; the harness is outgrown, not a world design defect (P15)"
				break
			}
			return nil, err
		}
	}
	if err := stack.ledger.Flush(); err != nil {
		return nil, err
	}
	summary, err := stack.ledger.Summary()
	if err != nil {
		return nil, err
	}
	rep.LedgerCount = summary.Count
	rep.LedgerDigest = hexString(summary.Digest[:])
	// Ledger-chain verification: the chain parses and verifies. This is not a
	// world replay of the probe run (spec 11.4).
	if _, err := cognition.OpenReplay(stack.ledgerPath); err != nil {
		return nil, err
	}
	rep.LedgerChainVerifies = true
	return rep, nil
}

func defaultFounders(fp fixture) int {
	if fp.Founders == 0 {
		return world.NaturalSlotsPerPatch * fp.Patches
	}
	return fp.Founders
}

func revisionString(reuse string) string {
	base := "vs1-hermetic-fake/2; base natural/7; mechanism v6; no authorised spend"
	if reuse != "" {
		base += "; declared matrix cells reused from the prior evaluation revision (committed bundles reloaded and re-validated); EC-5 scripted-proposer fixture, EC-6 privacy evidence, and EC-8 determinism twin computed under this revision"
	}
	return base
}

func writeEvaluation(ev evaluation, jsonPath, textPath string) error {
	body, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(body, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(textPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(textPath, []byte(renderText(ev)), 0o644)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vs1: "+format+"\n", args...)
	os.Exit(1)
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseCells(s string) [][2]uint64 {
	var out [][2]uint64
	for _, part := range splitList(s) {
		var f int
		var seed uint64
		if _, err := fmt.Sscanf(part, "f%d-s%d", &f, &seed); err != nil {
			continue
		}
		out = append(out, [2]uint64{uint64(f), seed})
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
