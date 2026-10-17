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
		arms          = flag.String("arms", strings.Join(declaredArms, ","), "comma-separated arms to run")
		cells         = flag.String("cells", "", "comma-separated cells as f<founders>-s<seed> (default: the 12 declared primary cells)")
		sensitivity   = flag.Bool("sensitivity", false, "also run the declared f=12 sensitivity-seed extension (seeds 4..7)")
		outJSON       = flag.String("out", ".tmp/30-vs2-evaluation.json", "evaluation JSON output path")
		outText       = flag.String("out-txt", ".tmp/30-vs2-evaluation.txt", "evaluation text output path")
		root          = flag.String("root", ".tmp/30-vs2-runs", "per-cell run directory root")
		recorded      = flag.String("recorded", ".tmp", "recorded ticket-19 evidence directory (VS2-EC-1)")
		srcRoot       = flag.String("src", ".", "module root for the VS2-EC-6 import-closure scan")
		budgetMin     = flag.Int("budget-min", 60, "wall-clock budget for the matrix in minutes; unrun cells are declared skipped")
		parallel      = flag.Int("parallel", 6, "cells to run concurrently (each cell is deterministic and writes its own directory)")
		runReplay     = flag.Bool("replay", true, "run the priority-arm replay lane (VS2-EC-5)")
		live          = flag.Bool("live", false, "run against the real gateway; refused unless "+EnvSpendAuthorization+" is the declared token (spend is NOT authorised)")
		wire          = flag.String("wire", string(cognition.WireResponses), "declared gateway wire recorded by the cognition stack: responses (default) or chat-completions")
		effort        = flag.String("effort", "", "applied reasoning effort of a live run: none|low|medium|high|xhigh|max (empty leaves the model default); recorded in run identity and in every call record")
		maxOutput     = flag.Int("max-output", 0, "applied output-token bound of a live run (0: the wire default, 512 on responses); the applied value is recorded in run identity and every call record")
		reps          = flag.Int("reps", 1, "repeated runs per arm-in-cell, each in its own <arm>-f<F>-s<S>-r<N> directory with its own ledger and bundles")
		votes         = flag.Int("votes", 1, "recorded samples per escalated Role A decision combined by majority (default 1, the declared single-call seam)")
		hours         = flag.Int("hours", DeclaredHorizonHours, "declared horizon H of the run (default 168, the frozen VS2 horizon)")
		attemptSecs   = flag.Int("attempt-timeout", 30, "per-attempt transport timeout in seconds (the frozen harness default is 30; a live max-effort run declares a longer bound)")
		deadlineSecs  = flag.Int("call-deadline", 90, "per-call deadline across attempts in seconds (the frozen harness default is 90)")
		respBound     = flag.Int("response-bound", 0, "applied recorded-response byte bound (0: the spec's 64 KiB hermetic default); a live max-effort run declares 1 MiB because reasoning items exceed 64 KiB")
		curveIn       = flag.String("curve-in", "", "comma-separated evaluation JSON paths: do not run cells, emit the curve report over them instead")
		curveOut      = flag.String("curve-out", "", "curve report text output path (default: <out-txt> with a -curve suffix)")
		claimContract = flag.String("claim-contract", "natural-claim/v1", "declared Role A request contract: natural-claim/v1 (legal action set only) or natural-claim/v2 (also states the observation's admissible action subset); recorded in run identity")
		relabelIn     = flag.String("relabel", "", "comma-separated evaluation JSON paths: re-emit their reports from the recorded data with the mode-corrected status/label/EC-9 text (no cells run, no spend); writes <name>-relabelled.{json,txt}")
	)
	flag.Parse()
	if *parallel < 1 {
		*parallel = 1
	}
	if *reps < 1 {
		*reps = 1
	}
	if *votes < 1 {
		*votes = 1
	}
	appliedEffort, err := cognition.ParseReasoningEffort(*effort)
	if err != nil {
		fail("effort: %v", err)
	}
	declaredReasoningEffort = appliedEffort
	declaredOutputBound = *maxOutput
	declaredVotes = *votes
	declaredReplicates = *reps
	if *hours < 1 {
		fail("hours: %d", *hours)
	}
	declaredHorizonHours = *hours
	if *attemptSecs < 1 || *deadlineSecs < 1 {
		fail("attempt-timeout/call-deadline must be positive")
	}
	declaredAttemptTimeout = time.Duration(*attemptSecs) * time.Second
	declaredCallDeadline = time.Duration(*deadlineSecs) * time.Second
	if *respBound < 0 {
		fail("response-bound must not be negative")
	}
	declaredResponseBound = *respBound
	contract, err := cognition.ParseClaimContract(*claimContract)
	if err != nil {
		fail("claim-contract: %v", err)
	}
	declaredClaimContract = contract
	mode, err := cognition.ParseWireMode(*wire)
	if err != nil {
		fail("wire: %v", err)
	}
	declaredGatewayWire = mode

	// The relabel mode re-emits recorded reports with the mode-corrected
	// status/label/EC-9 text: a report-only regeneration path that runs no cell and
	// spends nothing, so a report produced by an older binary can be corrected
	// without re-running the experiment or rewriting the original artefact.
	if *relabelIn != "" {
		for _, path := range splitList(*relabelIn) {
			out, changes, err := relabelEvaluationFile(path)
			if err != nil {
				fail("relabel %s: %v", path, err)
			}
			fmt.Printf("%s -> %s\n", path, out)
			for _, change := range changes {
				fmt.Printf("  changed: %s\n", change)
			}
		}
		return
	}

	// The curve report mode reads recorded evaluations and runs nothing: it
	// computes the declared band, stability, SNR, first-separation and
	// Wasserstein statistics over the replicates already on disk.
	if *curveIn != "" {
		report, err := curveReportFromFiles(splitList(*curveIn))
		if err != nil {
			fail("curve report: %v", err)
		}
		text := renderCurveReport(report)
		path := *curveOut
		if path == "" {
			path = strings.TrimSuffix(*outText, ".txt") + "-curve.txt"
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			fail("curve report: %v", err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			fail("curve report: %v", err)
		}
		fmt.Print(text)
		return
	}

	if err := authorizeLive(*live); err != nil {
		fail("%v", err)
	}
	if *live {
		cfg, err := newLiveConfig()
		if err != nil {
			fail("live transport: %v", err)
		}
		activeLive = cfg
		evaluationLabel = liveEvaluationLabel(cfg.endpoint, declaredReasoningEffort, declaredOutputBound, declaredResponseBound,
			"spend was authorised in-session by the supervisor within the lane's declared hard cap")
		evaluationStatus = "live/real-model"
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

	declared := make([]string, 0, len(armList)*len(cellList)*declaredReplicates)
	var tasks []cellSpec
	for _, arm := range armList {
		for _, c := range cellList {
			for rep := 1; rep <= declaredReplicates; rep++ {
				spec := cellSpec{Arm: arm, F: int(c[0]), Seed: c[1], Replicate: rep}
				declared = append(declared, spec.String())
				tasks = append(tasks, spec)
			}
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
	eval := buildEvaluation(runs, *recorded, skipped, declared, replay, *srcRoot, declaredHorizonHours)
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
		Status:               evaluationStatus,
		Label:                evaluationLabel,
		HonestLimit:          honestLimit,
		ModelID:              cognition.DefaultModelID,
		WireMode:             string(declaredGatewayWire),
		ModelIdentity:        hex.EncodeToString(identity[:]),
		ModelFeatureIdentity: hex.EncodeToString(featureIdentity[:]),
		FeatureSet:           protocol.FeatureSetVersion,
		BasePolicy:           basePolicy().ID,
		Mechanism:            int(mechanism()),
		Live:                 activeLive != nil,
		HorizonHours:         horizonHours,
		CallCeiling:          DeclaredCallCeiling,
		DeclaredEffort:       declaredReasoningEffort,
		ClaimContract:        string(declaredClaimContract),
		AppliedMaxOutput:     appliedOutputBound(),
		AppliedResponseBound: appliedResponseBound(),
		Votes:                declaredVotes,
		Replicates:           declaredReplicates,
		ValueModel:           declaredValueModel().Version,
		Arms:                 declaredArms,
		QuotaSchedule:        quotaSchedule(DeclaredCallCeiling, declaredHorizonHours),
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
	spend := "no authorised spend"
	if activeLive != nil {
		spend = "live gateway run, spend authorised for the run only"
	}
	effort := declaredReasoningEffort
	if effort == "" {
		effort = "model-default"
	}
	return fmt.Sprintf("vs2-hermetic-fake/1; model %s wire %s; effort %s; output-bound %d; response-bound %d; claim-contract %s; votes %d; replicates %d; attempt-timeout %s; call-deadline %s; base natural/7; mechanism v6; H=%d; N=%d; value-model %s; feature-set %s; %s",
		cognition.DefaultModelID, declaredGatewayWire, effort, appliedOutputBound(), appliedResponseBound(), declaredClaimContract, declaredVotes, declaredReplicates,
		declaredAttemptTimeout, declaredCallDeadline,
		declaredHorizonHours, DeclaredCallCeiling, declaredValueModel().Version, protocol.FeatureSetVersion, spend)
}

// appliedOutputBound is the output-token bound the run applies: the declared
// -max-output when one was given, otherwise the wire's own default (0 means the
// transport's declared bound). It is recorded so run identity names what the
// wire carried.
// declaredModelID is the application model id recorded in run identity.
func declaredModelID() string { return cognition.DefaultModelID }

// effortName renders the applied effort for the report, naming the declared
// "not sent" state instead of an empty string.
func effortName(effort string) string {
	if effort == "" {
		return "model-default (field omitted)"
	}
	return effort
}

func appliedOutputBound() int {
	return declaredOutputBound
}

// appliedResponseBound is the recorded-body bound the run applies: the declared
// -response-bound when one was given, otherwise the spec's 64 KiB default (0
// means the transport's own default). It is run identity, and each call record
// carries the applied bound plus the body's actual byte length.
func appliedResponseBound() int {
	return declaredResponseBound
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
