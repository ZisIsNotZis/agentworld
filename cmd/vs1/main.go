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
		runProbeF  = flag.Bool("probe", true, "run the declared scale/ceiling probe")
		probeCalls = flag.Uint64("probe-max-calls", 32, "lowered MaxCalls of the probe")
		budgetMin  = flag.Int("budget-min", 60, "wall-clock budget for the matrix in minutes; unrun cells are declared skipped")
		parallel   = flag.Int("parallel", 6, "cells to run concurrently (each cell is deterministic and writes its own directory)")
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

	if err := os.RemoveAll(*root); err != nil {
		fail("clean run root: %v", err)
	}
	var runs []*cellRun
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
			cr, err := runCell(ctx, spec, declaredFixture(spec.F, spec.Seed), *root, nil)
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

	var probe *probeReport
	if *runProbeF {
		p, err := runProbe(ctx, *root, *probeCalls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		} else {
			probe = p
		}
	}

	replay := runReplaySubset(ctx, runs, *root, armList)
	eval := buildEvaluation(runs, *recorded, skipped, declared, probe, replay)
	eval.Revision = revisionString()
	if err := writeEvaluation(eval, *outJSON, *outText); err != nil {
		fail("write evaluation: %v", err)
	}
	fmt.Print(renderText(eval))
}

func buildEvaluation(runs []*cellRun, recordedDir string, skipped, declared []string, probe *probeReport, replay map[string]string) evaluation {
	reports := make([]cellReport, 0, len(runs))
	for _, cr := range runs {
		reports = append(reports, cellReportOf(cr))
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
	ev := evaluation{
		Status:           "synthetic/model-conditional",
		Label:            evaluationLabel,
		HonestLimit:      "the hermetic FakeClient proves the mechanism is load-bearing; it does not and cannot claim a real model is load-bearing (no spend authorised, spec section 16). cost_micros is a modelled price-table charge over the fake usage, not real spend; no gateway was contacted",
		Arms:             declaredArms,
		Inventory:        inventory{Declared: len(declared), Ran: len(runs), Skipped: skipped},
		Cells:            reports,
		Fidelity:         buildFidelity(runs),
		Probe:            probe,
		Bands:            evaluateBands(runs, recordedDir, replay, probe),
		ReplaySubset:     replayKeys,
		ReplayMismatches: mismatches,
	}
	return ev
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
	// Replayable: the chain verifies and every recorded request resolves.
	if _, err := cognition.OpenReplay(stack.ledgerPath); err != nil {
		return nil, err
	}
	rep.Replayable = true
	return rep, nil
}

func defaultFounders(fp fixture) int {
	if fp.Founders == 0 {
		return world.NaturalSlotsPerPatch * fp.Patches
	}
	return fp.Founders
}

func revisionString() string {
	return "vs1-hermetic-fake/1; base natural/7; mechanism v6; no authorised spend"
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
