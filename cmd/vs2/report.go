package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// shortProjection records one budgeted arm whose hourly projection did not
// reach the declared horizon, so the divergence vector was refused rather than
// silently truncated to a shorter prefix (spec section 5.3). The arm, its
// actual projection length and the missing horizon samples are named so a
// short arm is visible in the report instead of silently absent.
type shortProjection struct {
	Arm               string `json:"arm"`
	ProjectionSamples int    `json:"projection_samples"`
	RequiredSamples   int    `json:"required_samples"`
	MissingHours      int    `json:"missing_hours"`
	Reason            string `json:"reason,omitempty"`
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
	// Replicate is the 1-based replicate index of a repeated run (0 when the run
	// was not replicated); Effort and MaxOutputTokens are the applied wire
	// settings of this cell's calls, recorded per cell so a curve comparison
	// never has to assume them from the command line.
	Replicate       int    `json:"replicate,omitempty"`
	Effort          string `json:"effort"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	ResponseBound   int    `json:"response_bound"`
	ClaimContract   string `json:"claim_contract"`

	// The full hourly KPI series of the cell (ALIVE/BIRTHS/STARVATION/AGE
	// deaths/capital histogram/granary histogram/wages/max capital). The bands
	// only ever reported alive_by_hour; the curve criterion (ticket 35) needs
	// every KPI, so all of them are exported from experiment.Projections.
	Hours           []int     `json:"hours,omitempty"`
	AliveByHour     []int     `json:"alive_by_hour,omitempty"`
	BirthsCum       []int64   `json:"births_cum,omitempty"`
	DeathsStarveCum []int64   `json:"deaths_starve_cum,omitempty"`
	DeathsAgeCum    []int64   `json:"deaths_age_cum,omitempty"`
	CapitalHist     [][]int64 `json:"capital_hist,omitempty"`
	GranaryHist     [][]int64 `json:"granary_hist,omitempty"`
	WagesCum        []int64   `json:"wages_cum,omitempty"`
	MaxCapital      []int64   `json:"max_capital,omitempty"`

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
	// ModelFeatureIdentity is the declared model+feature run-identity digest:
	// the model identity digest folded with the candidate feature-set version
	// (protocol.FeatureSetVersion). It is a separate digest; ModelIdentity is
	// unchanged, so VS1's model/sampling identity digest stays byte-identical.
	ModelFeatureIdentity string `json:"model_feature_identity_digest"`
	FeatureSet           string `json:"feature_set"`
	BasePolicy           string `json:"base_policy"`
	Mechanism            int    `json:"mechanism"`
	// Live records whether the report came from a live gateway run (real calls)
	// or the hermetic fake, so a recorded evaluation can be re-labelled honestly
	// without the process that produced it.
	Live         bool   `json:"live"`
	HorizonHours int    `json:"horizon_hours"`
	CallCeiling  uint64 `json:"call_ceiling"`
	// The applied wire settings and replicate count of the run (ticket 35):
	// declared effort, applied output bound, samples per decision, and repeated
	// runs per cell. DeclaredEffort empty means the model default was left to
	// itself, which is a recorded state, not an inference.
	DeclaredEffort       string `json:"declared_effort"`
	AppliedMaxOutput     int    `json:"applied_max_output_tokens"`
	AppliedResponseBound int    `json:"applied_response_bound"`
	ClaimContract        string `json:"claim_contract"`
	Votes                int    `json:"votes"`
	Replicates           int    `json:"replicates"`
	ValueModel           string `json:"value_model_version"`

	Arms             []string          `json:"arms"`
	QuotaSchedule    []int             `json:"quota_schedule"`
	Inventory        inventory         `json:"inventory"`
	Cells            []cellReport      `json:"cells"`
	Fidelity         []fidelityRow     `json:"fidelity"`
	ShortProjections []shortProjection `json:"short_projections"`
	Bands            []reportBand      `json:"bands"`
	ReplaySubset     []string          `json:"replay_subset"`
	ReplayMismatches []string          `json:"replay_mismatches"`
}

// hermeticEvaluationLabel is the honest label of a hermetic run. A live run must
// NOT carry it (it says "no authorised spend"), so the label is a variable set
// from the run mode and EC-9 checks it against that mode (ticket-34 review
// finding: the stale hermetic label made a live report lie).
const hermeticEvaluationLabel = "synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend); cost fields are MODELLED price-table charges over the fake usage, not real spend; the run demonstrates the equal-budget ALLOCATION MECHANISM is load-bearing, never the central claim, which awaits real-model spend authorisation (spec section 1.3/9, VS2-EC-9)"

// liveEvaluationLabel is the honest label of a live gateway run: it names the
// real model and endpoint, the applied wire settings, the spend authorisation and
// the unpriced cost state, and it refuses on its face to state a verdict.
func liveEvaluationLabel(endpoint, effort string, outputBound, responseBound int, spendNote string) string {
	return fmt.Sprintf("live/real-model; model %s via %s, wire %s (real gateway calls, spend authorised for the run only); applied effort %q, output-bound %d, response-bound %d; cost fields are UNPRICED because the gateway exposes no price entry for this model, and per-call and per-cell token totals are recorded so spend is derivable against any price table, never fabricated; %s; this run measures the allocation mechanism and the curve noise floor under real model decisions, never the central claim on its own (VS2-EC-9)",
		declaredModelID(), endpoint, declaredGatewayWire, effortName(effort), outputBound, responseBound, spendNote)
}

// evaluationLabel and evaluationStatus are the report's honest mode labels, set
// once from the run mode before any cell runs.
var (
	evaluationLabel  = hermeticEvaluationLabel
	evaluationStatus = "synthetic/model-conditional"
)

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
		Replicate: cr.spec.Replicate, Effort: declaredReasoningEffort, MaxOutputTokens: appliedOutputBound(), ResponseBound: appliedResponseBound(), ClaimContract: string(declaredClaimContract),
		Hours: cr.proj.Hours, AliveByHour: cr.proj.AliveByHour,
		BirthsCum: cr.proj.BirthsCum, DeathsStarveCum: cr.proj.DeathsStarveCum,
		DeathsAgeCum: cr.proj.DeathsAgeCum, CapitalHist: cr.proj.CapitalHist,
		GranaryHist: cr.proj.GranaryHist, WagesCum: cr.proj.WagesCum, MaxCapital: cr.proj.MaxCapital,
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
// whose projection is shorter than H is refused (never silently truncated, spec
// section 5.3) and returned as a structured short-projection record so the
// report can name the arm, its actual projection length and the missing hours.
func buildFidelity(runs []*cellRun, horizonHours int) ([]fidelityRow, []shortProjection) {
	var out []fidelityRow
	var short []shortProjection
	for _, cr := range runs {
		if !isBudgeted(cr.spec.Arm) {
			continue
		}
		ref := referenceRun(runs, cr.spec)
		if ref == nil {
			short = append(short, shortProjection{Arm: cr.spec.String(), Reason: "no-reference"})
			continue
		}
		indicators, err := experiment.DivergenceVector(fidelityArm(cr), fidelityArm(ref), horizonHours)
		if err != nil {
			var shortErr *experiment.ShortProjectionError
			if errors.As(err, &shortErr) {
				short = append(short, shortProjection{
					Arm:               shortErr.Label,
					ProjectionSamples: shortErr.ActualSamples,
					RequiredSamples:   shortErr.RequiredSamples,
					MissingHours:      shortErr.RequiredSamples - shortErr.ActualSamples,
				})
			} else {
				short = append(short, shortProjection{Arm: cr.spec.String(), Reason: err.Error()})
			}
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

// anyCellLive reports whether any cell carries the applied wire settings only a
// live gateway run records.
func anyCellLive(cells []cellReport) bool {
	for _, c := range cells {
		if c.ResponseBound > 0 || c.MaxOutputTokens > 0 || c.Effort != "" {
			return true
		}
	}
	return false
}

// relabelEvaluationFile re-emits one recorded evaluation with the mode-corrected
// report text. Measured fields are carried through untouched; only the fields
// that describe the run mode are recomputed, and the difference is returned so
// the caller can record it beside the original artefact.
func relabelEvaluationFile(jsonPath string) (string, []string, error) {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", nil, err
	}
	var ev evaluation
	if err := json.Unmarshal(raw, &ev); err != nil {
		return "", nil, err
	}
	changes := make([]string, 0, 4)
	effort := ev.DeclaredEffort
	// Legacy evaluations predate the recorded Live field; a live run is still
	// identifiable from its cells, because only a live run applies a non-default
	// output bound or response bound and records an effort.
	if !ev.Live && (ev.AppliedResponseBound > 0 || ev.AppliedMaxOutput > 0 || anyCellLive(ev.Cells)) {
		ev.Live = true
		changes = append(changes, "live: true (inferred from the recorded applied bounds; the evaluation predates the recorded live flag)")
	}
	if ev.Live {
		label := liveEvaluationLabel(liveEndpoint(), effort, ev.AppliedMaxOutput, ev.AppliedResponseBound,
			"spend was authorised in-session by the supervisor within the lane's declared hard cap")
		if ev.Label != label {
			changes = append(changes, "label: hermetic text -> live/real-model text naming model, wire, applied effort/output/response bounds, run-only spend authorisation and the unpriced cost state")
			ev.Label = label
		}
		if ev.Status != "live/real-model" {
			changes = append(changes, "status: "+ev.Status+" -> live/real-model")
			ev.Status = "live/real-model"
		}
	}
	for i := range ev.Bands {
		if ev.Bands[i].ID != "VS2-EC-9" {
			continue
		}
		status, evidence := ec9Evidence(ev.Label, ev.Live, liveEndpoint())
		if ev.Bands[i].Evidence != evidence {
			changes = append(changes, "VS2-EC-9 arm text: 'blocked on spend' -> the live-model arm RAN, no acceptance claimed here")
			ev.Bands[i].Evidence = evidence
			ev.Bands[i].Status = status
		}
	}
	base := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath))
	jsonOut := base + "-relabelled.json"
	textOut := base + "-relabelled.txt"
	body, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(jsonOut, append(body, '\n'), 0o644); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(textOut, []byte(renderText(ev)), 0o644); err != nil {
		return "", nil, err
	}
	return jsonOut, changes, nil
}
