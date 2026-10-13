package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"agentworld/internal/cognition"
	"agentworld/internal/experiment"
)

// The VS1 evaluation report: one row per arm-in-cell plus the frozen
// exit-criteria bands (spec section 12) and the roadmap's VS1 (a)-(f), the
// seven-indicator divergence-vector scaffold (spec section 11.5), and the
// declared scale/ceiling probe. It is labelled synthetic/model-conditional
// (EC-11): the hermetic run proves the mechanism is load-bearing, never that a
// real model is.

type reportBand struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Status    string `json:"status"`
	Evidence  string `json:"evidence"`
}

type cellReport struct {
	Arm            string `json:"arm"`
	F              int    `json:"f"`
	Seed           uint64 `json:"seed"`
	Workers        int    `json:"workers"`
	Ran            bool   `json:"ran"`
	Fixture        string `json:"fixture,omitempty"`
	RootDigest     string `json:"root_digest,omitempty"`
	BranchDigest   string `json:"branch_digest,omitempty"`
	TipHash        string `json:"tip_hash,omitempty"`
	ProjectionHash string `json:"projection_hash,omitempty"`
	HistoryBytes   int    `json:"history_bytes"`
	EventCount     int    `json:"event_count"`
	EventBodyBytes int    `json:"event_body_bytes"`
	AliveByHour    []int  `json:"alive_by_hour,omitempty"`
	Hours          []int  `json:"hours,omitempty"`

	CallCount         uint64                   `json:"call_count"`
	CallAttempts      int                      `json:"call_attempts"`
	CallLatencyUS     int64                    `json:"call_latency_total_micros"`
	PromptTokens      uint64                   `json:"prompt_tokens"`
	CompletionTokens  uint64                   `json:"completion_tokens"`
	CostMicros        int64                    `json:"cost_micros"`
	CostPerCallMicros float64                  `json:"cost_per_call_micros"`
	CallsPerActorHour float64                  `json:"calls_per_actor_hour"`
	CostPerActorHour  float64                  `json:"cost_per_actor_hour_micros"`
	ActorHours        int64                    `json:"actor_hours"`
	CeilingRows       []cognition.CognitiveRow `json:"ceiling_rows,omitempty"`

	Escalated    int `json:"escalated_decisions"`
	AppliedModel int `json:"applied_model_decisions"`
	// DivergedModel counts applied model decisions whose action differs from
	// the frozen cascade's own action at the same seam instant (EC-2).
	DivergedModel  int `json:"diverged_model_decisions"`
	DecisionErrors int `json:"decision_errors"`
	// Level2Refusals counts escalated claims whose committed world attempt was
	// a typed level-2 refusal (the world verdict, joined from the accepted
	// journal; spec 4.3/11.5 indicator 7).
	Level2Refusals int `json:"level2_world_refusals"`
	RoleBApplied   int `json:"role_b_applied"`
	RoleBRollback  int `json:"role_b_rolled_back"`
	RoleBProposed  int `json:"role_b_proposed"`
	RoleBRejected  int `json:"role_b_rejected"`
	RoleBValidated int `json:"role_b_validated"`
	RoleBMeasured  int `json:"role_b_measured"`
	RoleBNoTrigger int `json:"role_b_no_trigger"`
	RoleBWindowH   int `json:"role_b_measured_window_hours"`

	CeilingHit string `json:"ceiling_hit,omitempty"`
	Halted     bool   `json:"halted"`
	// FinalBundleError reports a failed final-bundle save (spec section 13: a
	// post-halt export is refused, but the failure is never swallowed).
	FinalBundleError string `json:"final_bundle_error,omitempty"`
	TokenCost        int64  `json:"token_cost"`
	DurationMS       int64  `json:"duration_ms"`
}

type fidelityRow struct {
	Arm        string                         `json:"arm"`
	F          int                            `json:"f"`
	Seed       uint64                         `json:"seed"`
	Reference  string                         `json:"reference"`
	Indicators []experiment.FidelityIndicator `json:"indicators"`
}

type probeReport struct {
	Ran          bool   `json:"ran"`
	Patches      int    `json:"patches"`
	Founders     int    `json:"founders"`
	MaxCalls     uint64 `json:"max_calls"`
	Ceiling      string `json:"ceiling"`
	HitHour      int    `json:"hit_hour"`
	Threshold    uint64 `json:"threshold"`
	Observed     uint64 `json:"observed"`
	LedgerCount  uint64 `json:"ledger_count"`
	LedgerDigest string `json:"ledger_digest"`
	// LedgerChainVerifies reports that the probe's recorded-call ledger chain
	// parses and verifies; the probe is not world-replayed (spec 11.4).
	LedgerChainVerifies bool   `json:"ledger_chain_verifies"`
	P15Verdict          string `json:"p15_verdict"`
}

type inventory struct {
	Declared int      `json:"declared"`
	Ran      int      `json:"ran"`
	Skipped  []string `json:"skipped"`
}

// twinReport is the EC-8 determinism twin evidence: the declared workers-4
// run against its workers-1 matrix counterpart at the same cell and arm.
type twinReport struct {
	Ran            bool   `json:"ran"`
	Arm            string `json:"arm"`
	F              int    `json:"f"`
	Seed           uint64 `json:"seed"`
	Workers        int    `json:"workers"`
	W1TipHash      string `json:"w1_tip_hash,omitempty"`
	W4TipHash      string `json:"w4_tip_hash,omitempty"`
	W1BranchDigest string `json:"w1_branch_digest,omitempty"`
	W4BranchDigest string `json:"w4_branch_digest,omitempty"`
	Identical      bool   `json:"identical"`
	Note           string `json:"note,omitempty"`
}

// fixtureReport is the labeled scripted-proposer fixture evidence (EC-5a). It
// carries the live lifecycle counts and, separately, the unscripted A2
// honest-null counts so the fixture never masks the declared arms. The A2g
// fixture also reports its divergent-parent sandbox count, whether the
// measurement kept the head on the child generation, and its reported
// final-bundle save failure.
type fixtureReport struct {
	Label               string `json:"label"`
	Arm                 string `json:"arm"`
	F                   int    `json:"f"`
	Seed                uint64 `json:"seed"`
	Ran                 bool   `json:"ran"`
	Proposed            int    `json:"proposed"`
	Admitted            int    `json:"admitted"`
	Applied             int    `json:"applied"`
	Measured            int    `json:"measured"`
	RolledBack          int    `json:"rolled_back"`
	MeasuredWindowHours int    `json:"measured_window_hours"`
	// DivergentParentSandbox counts patch-validate records whose proposal parent
	// was a cognitive generation, i.e. the sandbox parent branch was the current
	// head after an apply that was not rolled back (spec 10.2).
	DivergentParentSandbox int `json:"divergent_parent_sandbox"`
	// MeasurementKeptHead reports that a measurement resolved without a
	// rollback, so the applied generation stayed bound.
	MeasurementKeptHead bool `json:"measurement_kept_head"`
	// FinalBundleError reports a failed final-bundle save (spec section 13: a
	// post-measurement export is refused, but the failure is never swallowed).
	FinalBundleError string `json:"final_bundle_error,omitempty"`

	UnscriptedA2Proposed  int `json:"unscripted_a2_proposed"`
	UnscriptedA2Applied   int `json:"unscripted_a2_applied"`
	UnscriptedA2Rolled    int `json:"unscripted_a2_rolled_back"`
	UnscriptedA2Rejected  int `json:"unscripted_a2_rejected"`
	UnscriptedA2Validated int `json:"unscripted_a2_validated"`
}

type evaluation struct {
	Status           string          `json:"status"`
	Revision         string          `json:"revision"`
	Label            string          `json:"label"`
	HonestLimit      string          `json:"honest_limit"`
	Arms             []string        `json:"arms"`
	Inventory        inventory       `json:"inventory"`
	Cells            []cellReport    `json:"cells"`
	Fixtures         []fixtureReport `json:"fixtures,omitempty"`
	Twins            []twinReport    `json:"twins,omitempty"`
	Fidelity         []fidelityRow   `json:"fidelity"`
	Probe            *probeReport    `json:"probe,omitempty"`
	Bands            []reportBand    `json:"bands"`
	ReplaySubset     []string        `json:"replay_subset"`
	ReplayMismatches []string        `json:"replay_mismatches"`
}

const evaluationLabel = "synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend); cost fields are MODELLED price-table charges over the fake usage, not real spend; the run demonstrates the MECHANISM is load-bearing, never that a real model is load-bearing (spec section 16)"

func cellReportOf(cr *cellRun) cellReport {
	rep := cellReport{
		Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Workers: cr.fp.effectiveWorkers(), Ran: true,
		RootDigest: cr.rootDigest, BranchDigest: cr.branchDigest,
		HistoryBytes: cr.accepted.HistoryBytes, EventCount: cr.accepted.EventCount,
		EventBodyBytes: cr.accepted.EventBodyBytes,
		AliveByHour:    cr.proj.AliveByHour, Hours: cr.proj.Hours,
		CallCount: cr.totals.Calls, CallAttempts: cr.callAttempts, CallLatencyUS: cr.callLatencyUS,
		PromptTokens:     cr.totals.PromptTokens,
		CompletionTokens: cr.totals.CompletionTokens, CostMicros: cr.totals.CostMicros,
		ActorHours: cr.sumAliveHours, Escalated: cr.escalated, AppliedModel: cr.appliedModel,
		DivergedModel:  cr.divergedModel,
		DecisionErrors: cr.decisionErrors, Level2Refusals: level2RefusalTotal(cr),
		RoleBApplied: cr.roleBApplied, RoleBRollback: cr.roleBRollback,
		RoleBProposed: cr.roleBProposed, RoleBRejected: cr.roleBRejected, RoleBValidated: cr.roleBValidated,
		RoleBMeasured: cr.roleBMeasured, RoleBWindowH: cr.roleBMeasureHour, RoleBNoTrigger: cr.roleBNoTrigger,
		Halted: cr.halted, TokenCost: cr.totals.CostMicros, DurationMS: cr.durationMS,
		FinalBundleError: cr.finalBundleError,
		CeilingRows:      cr.ceilingRows,
	}
	if isFixtureArm(cr.spec.Arm) {
		rep.Fixture = FixtureLabel
	}
	if cr.haveAccepted {
		rep.TipHash = cr.accepted.TipHash
		rep.ProjectionHash = cr.accepted.ProjectionHash
	}
	if cr.ceilingHit != nil {
		rep.CeilingHit = string(cr.ceilingHit.Ceiling)
	}
	if cr.totals.Calls > 0 {
		rep.CostPerCallMicros = float64(cr.totals.CostMicros) / float64(cr.totals.Calls)
	}
	if cr.sumAliveHours > 0 {
		rep.CallsPerActorHour = float64(cr.totals.Calls) / float64(cr.sumAliveHours)
		rep.CostPerActorHour = float64(cr.totals.CostMicros) / float64(cr.sumAliveHours)
	}
	return rep
}

// recordedFacts is the subset of the recorded ticket-19 evidence EC-1 compares
// against (the births-enabled branch of .tmp/natural9-n7-*.json).
type recordedFacts struct {
	Found          bool
	BranchDigest   string
	Events         int
	EventBodyBytes int
	HistoryBytes   int
	AliveByHour    []int
}

func loadRecorded(dir string, f int, seed uint64) recordedFacts {
	path := filepath.Join(dir, fmt.Sprintf("natural9-n7-f%d-s%d-w1.json", f, seed))
	data, err := os.ReadFile(path)
	if err != nil {
		return recordedFacts{}
	}
	var doc struct {
		Branches []struct {
			Births         bool   `json:"births"`
			BranchDigest   string `json:"branch_digest"`
			Events         int    `json:"events"`
			EventBodyBytes int    `json:"event_body_bytes"`
			HistoryBytes   int    `json:"history_bytes"`
			Population     struct {
				AliveByHour []int `json:"alive_by_hour"`
			} `json:"population"`
		} `json:"branches"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return recordedFacts{}
	}
	for _, b := range doc.Branches {
		if b.Births {
			return recordedFacts{Found: true, BranchDigest: b.BranchDigest, Events: b.Events,
				EventBodyBytes: b.EventBodyBytes, HistoryBytes: b.HistoryBytes, AliveByHour: b.Population.AliveByHour}
		}
	}
	return recordedFacts{}
}

func band(id, criterion, status, evidence string) reportBand {
	return reportBand{ID: id, Criterion: criterion, Status: status, Evidence: evidence}
}

// buildFidelity emits the seven-indicator divergence vector of a budgeted arm
// against the A3 reference at the same cell (spec section 11.5). It is emitted,
// never gated, in VS1.
func buildFidelity(runs []*cellRun) []fidelityRow {
	byCell := map[string]*cellRun{}
	for _, cr := range runs {
		byCell[fmt.Sprintf("f%d-s%d", cr.spec.F, cr.spec.Seed)] = cr
	}
	var out []fidelityRow
	for _, cr := range runs {
		if cr.spec.Arm == ArmA3 || cr.spec.Arm == ArmA0 || isFixtureArm(cr.spec.Arm) {
			continue
		}
		ref := referenceRun(runs, cr.spec)
		if ref == nil {
			continue
		}
		indicators, err := experiment.DivergenceVector(fidelityArm(cr), fidelityArm(ref), cr.fp.Hours)
		if err != nil {
			// The shared metric computes over the full declared horizon and
			// refuses a shorter projection rather than truncating to a prefix
			// (spec section 5.3); every declared VS1 arm runs the full horizon,
			// so a refused arm is dropped rather than compared over a prefix.
			continue
		}
		out = append(out, fidelityRow{
			Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Reference: ArmA3,
			Indicators: indicators,
		})
	}
	return out
}

// fidelityArm adapts a gathered cell run to the shared experiment metric input.
func fidelityArm(cr *cellRun) experiment.Arm {
	return experiment.Arm{
		Label:         cr.spec.String(),
		Projections:   cr.proj,
		Patches:       cr.fp.Patches,
		Interventions: cr.interventions,
		Journal:       cr.journal,
	}
}

func referenceRun(runs []*cellRun, spec cellSpec) *cellRun {
	for _, cr := range runs {
		if cr.spec.Arm == ArmA3 && cr.spec.F == spec.F && cr.spec.Seed == spec.Seed {
			return cr
		}
	}
	return nil
}

// level2RefusalTotal is the per-cell count of level-2 world refusals of
// escalated model-applied claims. It reads the declared horizon's sample count
// from the cell fixture, the same full horizon the fidelity vector is computed
// over (spec section 5.3).
func level2RefusalTotal(cr *cellRun) int {
	total := 0
	for _, n := range experiment.Level2RefusalCounts(cr.journal, experiment.ModelEscalatedKeySet(cr.interventions), cr.fp.Hours+1) {
		total += n
	}
	return total
}

// sortRuns orders runs by arm then cell for stable reports.
func sortRuns(runs []*cellRun) {
	order := map[string]int{}
	for i, a := range declaredArms {
		order[a] = i
	}
	order[ArmA2f] = len(declaredArms)
	order[ArmA2g] = len(declaredArms) + 1
	sort.Slice(runs, func(i, j int) bool {
		if order[runs[i].spec.Arm] != order[runs[j].spec.Arm] {
			return order[runs[i].spec.Arm] < order[runs[j].spec.Arm]
		}
		if runs[i].spec.F != runs[j].spec.F {
			return runs[i].spec.F < runs[j].spec.F
		}
		return runs[i].spec.Seed < runs[j].spec.Seed
	})
}
