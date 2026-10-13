package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
)

// The VS2 CLI (spec sections 8-10). The declared default is the full primary
// matrix (arms A0/R/P/U/A over the 12 declared cells); -arms/-cells select a
// declared subset, which the report records honestly as a partial inventory.
func main() {
	var (
		arms        = flag.String("arms", strings.Join(declaredArms, ","), "comma-separated arms to run")
		cells       = flag.String("cells", "", "comma-separated cells as f<founders>-s<seed> (default: the 12 declared primary cells)")
		sensitivity = flag.Bool("sensitivity", false, "also run the declared f=12 sensitivity-seed extension (seeds 4..7)")
		outJSON     = flag.String("out", ".tmp/30-vs2-evaluation.json", "evaluation JSON output path")
		outText     = flag.String("out-txt", ".tmp/30-vs2-evaluation.txt", "evaluation text output path")
		root        = flag.String("root", ".tmp/30-vs2-runs", "per-cell run directory root")
		recorded    = flag.String("recorded", ".tmp", "recorded ticket-19 evidence directory (VS2-EC-1)")
		srcRoot     = flag.String("src", ".", "module root for the VS2-EC-6 import-closure scan")
		budgetMin   = flag.Int("budget-min", 60, "wall-clock budget for the matrix in minutes; unrun cells are declared skipped")
		parallel    = flag.Int("parallel", 6, "cells to run concurrently (each cell is deterministic and writes its own directory)")
		runReplay   = flag.Bool("replay", true, "run the priority-arm replay lane (VS2-EC-5)")
		live        = flag.Bool("live", false, "run against the real gateway; refused unless "+EnvSpendAuthorization+" is the declared token (spend is NOT authorised)")
		wire        = flag.String("wire", string(cognition.WireResponses), "declared gateway wire recorded by the cognition stack: responses (default) or chat-completions")
	)
	flag.Parse()
	if *parallel < 1 {
		*parallel = 1
	}
	mode, err := cognition.ParseWireMode(*wire)
	if err != nil {
		fail("wire: %v", err)
	}
	declaredGatewayWire = mode

	if err := authorizeLive(*live); err != nil {
		fail("%v", err)
	}
	if *live {
		cfg, err := newLiveConfig()
		if err != nil {
			fail("live transport: %v", err)
		}
		activeLive = cfg
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*budgetMin)*time.Minute)
	defer cancel()

	armList := splitList(*arms)
	cellList := parseCells(*cells)
	if len(cellList) == 0 {
		cellList = VS2DeclaredCells()
		if *sensitivity {
			cellList = append(cellList, sensitivityCells()...)
		}
	}

	declared := make([]string, 0, len(armList)*len(cellList))
	var tasks []cellSpec
	for _, arm := range armList {
		for _, c := range cellList {
			declared = append(declared, fmt.Sprintf("%s-f%d-s%d", arm, c[0], c[1]))
			tasks = append(tasks, cellSpec{Arm: arm, F: int(c[0]), Seed: c[1]})
		}
	}

	if err := os.RemoveAll(*root); err != nil {
		fail("clean run root: %v", err)
	}

	var runs []*cellRun
	var skipped []string
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

	var replay map[string]string
	if *runReplay {
		replay = runReplaySubset(ctx, runs, *root, *parallel)
	}
	eval := buildEvaluation(runs, *recorded, skipped, declared, replay, *srcRoot, DeclaredHorizonHours)
	eval.Revision = revisionString()
	if err := writeEvaluation(eval, *outJSON, *outText); err != nil {
		fail("write evaluation: %v", err)
	}
	fmt.Print(renderText(eval))
}

// buildEvaluation assembles the report. horizonHours is the declared horizon
// every arm's divergence vector is computed over (the declared H for a matrix
// run, a shorter horizon for a bounded test).
func buildEvaluation(runs []*cellRun, recordedDir string, skipped, declared []string, replay map[string]string, srcRoot string, horizonHours int) evaluation {
	reports := make([]cellReport, 0, len(runs))
	for _, cr := range runs {
		reports = append(reports, cellReportOf(cr))
	}
	fidelity, shortProjections := buildFidelity(runs, horizonHours)
	replayKeys := make([]string, 0, len(replay))
	for k := range replay {
		replayKeys = append(replayKeys, k)
	}
	sort.Strings(replayKeys)
	var mismatches []string
	for k, v := range replay {
		if v != "identical" {
			mismatches = append(mismatches, k+":"+v)
		}
	}
	sort.Strings(mismatches)
	identity := modelIdentityDigestBytes()
	featureIdentity := cognition.ModelFeatureIdentityDigest(cognition.DefaultModelID, declaredGatewayWire, cognition.FrozenSampling(), protocol.FeatureSetVersion)
	return evaluation{
		Status:               "synthetic/model-conditional",
		Label:                evaluationLabel,
		HonestLimit:          honestLimit,
		ModelID:              cognition.DefaultModelID,
		WireMode:             string(declaredGatewayWire),
		ModelIdentity:        hex.EncodeToString(identity[:]),
		ModelFeatureIdentity: hex.EncodeToString(featureIdentity[:]),
		FeatureSet:           protocol.FeatureSetVersion,
		BasePolicy:           basePolicy().ID,
		Mechanism:            int(mechanism()),
		HorizonHours:         horizonHours,
		CallCeiling:          DeclaredCallCeiling,
		ValueModel:           declaredValueModel().Version,
		Arms:                 declaredArms,
		QuotaSchedule:        quotaSchedule(DeclaredCallCeiling, DeclaredHorizonHours),
		Inventory:            inventory{Declared: len(declared), Ran: len(runs), Skipped: skipped},
		Cells:                reports,
		Fidelity:             fidelity,
		ShortProjections:     shortProjections,
		Bands:                evaluateBands(runs, runs, recordedDir, replay, srcRoot, fidelity),
		ReplaySubset:         replayKeys,
		ReplayMismatches:     mismatches,
	}
}

func revisionString() string {
	return fmt.Sprintf("vs2-hermetic-fake/1; model %s wire %s; base natural/7; mechanism v6; H=%d; N=%d; value-model %s; feature-set %s; no authorised spend",
		cognition.DefaultModelID, declaredGatewayWire, DeclaredHorizonHours, DeclaredCallCeiling, declaredValueModel().Version, protocol.FeatureSetVersion)
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
	fmt.Fprintf(os.Stderr, "vs2: "+format+"\n", args...)
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
