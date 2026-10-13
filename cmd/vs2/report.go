package main

import (
	"fmt"
	"sort"

	"agentworld/internal/cognition"
	"agentworld/internal/experiment"
)

// The VS2 evaluation report: one row per arm-in-cell with the realized call
// count and the under-budget flag (VS2-EC-2), the frozen eight-indicator
// divergence vector against the R reference with the declared no-divergence
// normalisation (VS2-EC-3/EC-4, spec section 6), the value-model calibration
// instrument (VS2-EC-8), and the VS2-EC-1..VS2-EC-9 bands. It is labelled
// synthetic/model-conditional (VS2-EC-9).

type reportBand struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Status    string `json:"status"`
	Evidence  string `json:"evidence"`
}

// indicatorReport is one divergence indicator plus the §6 normalised
// first-divergence hour (raw -1 -> H+1).
type indicatorReport struct {
	experiment.FidelityIndicator
	NormalizedFirstDivergenceHour int `json:"normalized_first_divergence_hour"`
}

// fidelityRow is the eight-indicator divergence vector of one budgeted arm at
// one cell against the R reference.
type fidelityRow struct {
	Arm        string            `json:"arm"`
	F          int               `json:"f"`
	Seed       uint64            `json:"seed"`
	Reference  string            `json:"reference"`
	Indicators []indicatorReport `json:"indicators"`
}

// direction is the declared lower/higher-is-better direction of a fidelity
// summary (spec sections 6/7 VS2-EC-3/EC-4).
type direction int

const (
	lowerIsBetter direction = iota
	higherIsBetter
)

// calibrationReport is the declared minimal VS2-EC-8 calibration instrument:
// the predicted priority of escalated decisions versus the realized downstream
// effect (the fraction of applied model decisions whose action diverged from
// the frozen cascade at the same seam). CalibrationError is the absolute
// difference of the two aggregate rates; 0 means the priority ordering is
// perfectly calibrated at the aggregate.
type calibrationReport struct {
	ValueModelVersion       string  `json:"value_model_version"`
	PredictedPriorityMin    int64   `json:"predicted_priority_min"`
	PredictedPriorityMax    int64   `json:"predicted_priority_max"`
	PredictedPriorityMean   float64 `json:"predicted_priority_mean"`
	NormalizedPredictedMean float64 `json:"normalized_predicted_priority_mean"`
	EscalatedCandidates     int     `json:"escalated_candidates"`
	AppliedModelDecisions   int     `json:"applied_model_decisions"`
	RealizedDivergenceRate  float64 `json:"realized_divergence_rate"`
	CalibrationError        float64 `json:"calibration_error"`
}

// cellReport is one arm-in-cell result.
type cellReport struct {
	Arm            string `json:"arm"`
	F              int    `json:"f"`
	Seed           uint64 `json:"seed"`
	Workers        int    `json:"workers"`
	Ran            bool   `json:"ran"`
	RootDigest     string `json:"root_digest,omitempty"`
	BranchDigest   string `json:"branch_digest,omitempty"`
	TipHash        string `json:"tip_hash,omitempty"`
	ProjectionHash string `json:"projection_hash,omitempty"`
	Hours          []int  `json:"hours,omitempty"`
	AliveByHour    []int  `json:"alive_by_hour,omitempty"`

	// Equal-budget accounting (VS2-EC-2): the declared ceiling N and the
	// realized call count from the call ledger. No arm's realized count is
	// assumed equal to N.
	CallCeiling     uint64 `json:"call_ceiling"`
	RealizedCalls   uint64 `json:"realized_calls"`
	UnderBudget     bool   `json:"under_budget"`
	BudgetShortfall uint64 `json:"budget_shortfall"`

	// The declared per-hour quota schedule and the per-hour shortfalls recorded
	// on the allocation records (VS2 section 5.1).
	QuotaSum       int `json:"quota_sum"`
	ShortfallTotal int `json:"shortfall_total"`

	Escalated         int `json:"escalated_decisions"`
	AppliedModel      int `json:"applied_model_decisions"`
	DivergedModel     int `json:"diverged_model_decisions"`
	DecisionErrors    int `json:"decision_errors"`
	AllocationRecords int `json:"allocation_records"`

	PromptTokens     uint64                   `json:"prompt_tokens"`
	CompletionTokens uint64                   `json:"completion_tokens"`
	CostMicros       int64                    `json:"cost_micros"`
	CeilingRows      []cognition.CognitiveRow `json:"ceiling_rows,omitempty"`

	CeilingHit       string            `json:"ceiling_hit,omitempty"`
	Halted           bool              `json:"halted"`
	FinalBundleError string            `json:"final_bundle_error,omitempty"`
	Calibration      calibrationReport `json:"calibration"`
	DurationMS       int64             `json:"duration_ms"`
}

type inventory struct {
	Declared int      `json:"declared"`
	Ran      int      `json:"ran"`
	Skipped  []string `json:"skipped"`
}

type evaluation struct {
	Status        string `json:"status"`
	Revision      string `json:"revision"`
	Label         string `json:"label"`
	HonestLimit   string `json:"honest_limit"`
	ModelID       string `json:"model_id"`
	WireMode      string `json:"wire_mode"`
	ModelIdentity string `json:"model_identity_digest"`
	BasePolicy    string `json:"base_policy"`
	Mechanism     int    `json:"mechanism"`
	HorizonHours  int    `json:"horizon_hours"`
	CallCeiling   uint64 `json:"call_ceiling"`
	ValueModel    string `json:"value_model_version"`

	Arms             []string      `json:"arms"`
	QuotaSchedule    []int         `json:"quota_schedule"`
	Inventory        inventory     `json:"inventory"`
	Cells            []cellReport  `json:"cells"`
	Fidelity         []fidelityRow `json:"fidelity"`
	Bands            []reportBand  `json:"bands"`
	ReplaySubset     []string      `json:"replay_subset"`
	ReplayMismatches []string      `json:"replay_mismatches"`
}

const evaluationLabel = "synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend); cost fields are MODELLED price-table charges over the fake usage, not real spend; the run demonstrates the equal-budget ALLOCATION MECHANISM is load-bearing, never the central claim, which awaits real-model spend authorisation (spec section 1.3/9, VS2-EC-9)"

const honestLimit = "fidelity is measured as divergence from the high-budget R reference, a comparative proxy and never ground truth: a smaller divergence means closer to the model-conditional reference, not more faithful to reality (spec section 6)"

// normalizeFirstDivergence applies the single declared §6 no-divergence
// encoding: raw -1 ("never diverged over H") normalises to H+1, strictly
// greater than any real first-divergence hour, so a never-diverged indicator is
// best under the declared higher-is-better direction.
func normalizeFirstDivergence(raw, horizonHours int) int {
	if raw == -1 {
		return horizonHours + 1
	}
	return raw
}

// summarizeVector adapts the shared experiment summaries into the report's
// normalised indicator rows.
func summarizeVector(indicators []experiment.FidelityIndicator, horizonHours int) []indicatorReport {
	out := make([]indicatorReport, 0, len(indicators))
	for _, ind := range indicators {
		out = append(out, indicatorReport{
			FidelityIndicator:             ind,
			NormalizedFirstDivergenceHour: normalizeFirstDivergence(ind.FirstDivergenceHour, horizonHours),
		})
	}
	return out
}

// cellReportOf adapts a gathered run to its report row.
func cellReportOf(cr *cellRun) cellReport {
	rep := cellReport{
		Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Workers: cr.fp.effectiveWorkers(), Ran: true,
		RootDigest: cr.rootDigest, BranchDigest: cr.branchDigest,
		Hours: cr.proj.Hours, AliveByHour: cr.proj.AliveByHour,
		Escalated: cr.escalated, AppliedModel: cr.appliedModel, DivergedModel: cr.divergedModel,
		DecisionErrors: cr.decisionErrors, AllocationRecords: cr.allocationRecords,
		PromptTokens: cr.totals.PromptTokens, CompletionTokens: cr.totals.CompletionTokens,
		CostMicros: cr.totals.CostMicros, CeilingRows: cr.ceilingRows,
		Halted: cr.halted, FinalBundleError: cr.finalBundleError, DurationMS: cr.durationMS,
		Calibration: calibrationOf(cr),
	}
	if cr.haveAccepted {
		rep.TipHash = cr.accepted.TipHash
		rep.ProjectionHash = cr.accepted.ProjectionHash
	}
	if cr.ceilingHit != nil {
		rep.CeilingHit = string(cr.ceilingHit.Ceiling)
	}
	if isBudgeted(cr.spec.Arm) {
		rep.CallCeiling = DeclaredCallCeiling
		rep.RealizedCalls = cr.realizedCalls
		rep.UnderBudget = cr.realizedCalls < DeclaredCallCeiling
		rep.BudgetShortfall = DeclaredCallCeiling - cr.realizedCalls
		rep.QuotaSum = sumInts(quotaSchedule(DeclaredCallCeiling, cr.fp.Hours))
		rep.ShortfallTotal = shortfallTotal(cr.interventions)
	}
	if cr.spec.Arm == ArmR {
		rep.CallCeiling = DeclaredReferenceCeiling
		rep.RealizedCalls = cr.realizedCalls
	}
	return rep
}

// calibrationOf derives the declared calibration instrument from the arm's
// allocation records.
func calibrationOf(cr *cellRun) calibrationReport {
	cal := calibrationReport{ValueModelVersion: VS2ValueModelVersion, PredictedPriorityMin: 0}
	var sum, count int64
	for _, rec := range cr.interventions {
		if rec.Kind != cognition.InterventionAllocation || rec.Allocation == nil {
			continue
		}
		selected := map[uint64]bool{}
		for _, id := range rec.Allocation.Selected {
			selected[uint64(id)] = true
		}
		for _, score := range rec.Allocation.Scores {
			if !selected[uint64(score.Actor)] {
				continue
			}
			if count == 0 || score.Priority < cal.PredictedPriorityMin {
				cal.PredictedPriorityMin = score.Priority
			}
			if score.Priority > cal.PredictedPriorityMax {
				cal.PredictedPriorityMax = score.Priority
			}
			sum += score.Priority
			count++
		}
	}
	cal.EscalatedCandidates = int(count)
	if count > 0 {
		cal.PredictedPriorityMean = float64(sum) / float64(count)
		if cal.PredictedPriorityMax > 0 {
			cal.NormalizedPredictedMean = cal.PredictedPriorityMean / float64(cal.PredictedPriorityMax)
		}
	}
	cal.AppliedModelDecisions = cr.appliedModel
	if cr.appliedModel > 0 {
		cal.RealizedDivergenceRate = float64(cr.divergedModel) / float64(cr.appliedModel)
	}
	cal.CalibrationError = absFloat(cal.NormalizedPredictedMean - cal.RealizedDivergenceRate)
	return cal
}

// shortfallTotal sums the recorded per-hour shortfalls over an arm's allocation
// records.
func shortfallTotal(records []cognition.InterventionRecord) int {
	total := 0
	for _, rec := range records {
		if rec.Kind == cognition.InterventionAllocation && rec.Allocation != nil {
			total += rec.Allocation.Shortfall
		}
	}
	return total
}

// buildFidelity computes the divergence vector of every budgeted arm against
// the R reference at the same cell, over the full declared horizon. An arm
// whose projection is shorter than H is reported by the band layer (the shared
// metric refuses instead of silently truncating, spec section 5.3).
func buildFidelity(runs []*cellRun, horizonHours int) ([]fidelityRow, []string) {
	var out []fidelityRow
	var short []string
	for _, cr := range runs {
		if !isBudgeted(cr.spec.Arm) {
			continue
		}
		ref := referenceRun(runs, cr.spec)
		if ref == nil {
			short = append(short, cr.spec.String()+":no-reference")
			continue
		}
		indicators, err := experiment.DivergenceVector(fidelityArm(cr), fidelityArm(ref), horizonHours)
		if err != nil {
			short = append(short, cr.spec.String()+":"+err.Error())
			continue
		}
		out = append(out, fidelityRow{
			Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Reference: ArmR,
			Indicators: summarizeVector(indicators, horizonHours),
		})
	}
	return out, short
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
		if cr.spec.Arm == ArmR && cr.spec.F == spec.F && cr.spec.Seed == spec.Seed {
			return cr
		}
	}
	return nil
}

// sortRuns orders runs by arm order then cell for stable reports.
func sortRuns(runs []*cellRun) {
	order := map[string]int{}
	for i, a := range declaredArms {
		order[a] = i
	}
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

func sumInts(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func band(id, criterion, status, evidence string) reportBand {
	return reportBand{ID: id, Criterion: criterion, Status: status, Evidence: evidence}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	sort.Strings(items)
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func fmtFloat(v float64) string { return fmt.Sprintf("%.4f", v) }
