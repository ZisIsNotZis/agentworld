package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"agentworld/internal/cognition"
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
	Ran            bool   `json:"ran"`
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

	Escalated      int `json:"escalated_decisions"`
	AppliedModel   int `json:"applied_model_decisions"`
	DecisionErrors int `json:"decision_errors"`
	RoleBApplied   int `json:"role_b_applied"`
	RoleBRollback  int `json:"role_b_rolled_back"`
	RoleBProposed  int `json:"role_b_proposed"`
	RoleBRejected  int `json:"role_b_rejected"`
	RoleBNoTrigger int `json:"role_b_no_trigger"`

	CeilingHit string `json:"ceiling_hit,omitempty"`
	Halted     bool   `json:"halted"`
	TokenCost  int64  `json:"token_cost"`
	DurationMS int64  `json:"duration_ms"`
}

type fidelityIndicator struct {
	Index               int       `json:"index"`
	Name                string    `json:"name"`
	FirstDivergenceHour int       `json:"first_divergence_hour"`
	Max                 float64   `json:"max"`
	Mean                float64   `json:"mean"`
	Sequence            []float64 `json:"sequence"`
}

type fidelityRow struct {
	Arm        string              `json:"arm"`
	F          int                 `json:"f"`
	Seed       uint64              `json:"seed"`
	Reference  string              `json:"reference"`
	Indicators []fidelityIndicator `json:"indicators"`
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
	Replayable   bool   `json:"replayable"`
	P15Verdict   string `json:"p15_verdict"`
}

type inventory struct {
	Declared int      `json:"declared"`
	Ran      int      `json:"ran"`
	Skipped  []string `json:"skipped"`
}

type evaluation struct {
	Status           string        `json:"status"`
	Revision         string        `json:"revision"`
	Label            string        `json:"label"`
	HonestLimit      string        `json:"honest_limit"`
	Arms             []string      `json:"arms"`
	Inventory        inventory     `json:"inventory"`
	Cells            []cellReport  `json:"cells"`
	Fidelity         []fidelityRow `json:"fidelity"`
	Probe            *probeReport  `json:"probe,omitempty"`
	Bands            []reportBand  `json:"bands"`
	ReplaySubset     []string      `json:"replay_subset"`
	ReplayMismatches []string      `json:"replay_mismatches"`
}

const evaluationLabel = "synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend); cost fields are MODELLED price-table charges over the fake usage, not real spend; the run demonstrates the MECHANISM is load-bearing, never that a real model is load-bearing (spec section 16)"

func cellReportOf(cr *cellRun) cellReport {
	rep := cellReport{
		Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Ran: true,
		RootDigest: cr.rootDigest, BranchDigest: cr.branchDigest,
		HistoryBytes: cr.accepted.HistoryBytes, EventCount: cr.accepted.EventCount,
		EventBodyBytes: cr.accepted.EventBodyBytes,
		AliveByHour:    cr.proj.AliveByHour, Hours: cr.proj.Hours,
		CallCount: cr.totals.Calls, CallAttempts: cr.callAttempts, CallLatencyUS: cr.callLatencyUS,
		PromptTokens:     cr.totals.PromptTokens,
		CompletionTokens: cr.totals.CompletionTokens, CostMicros: cr.totals.CostMicros,
		ActorHours: cr.sumAliveHours, Escalated: cr.escalated, AppliedModel: cr.appliedModel,
		DecisionErrors: cr.decisionErrors, RoleBApplied: cr.roleBApplied, RoleBRollback: cr.roleBRollback,
		RoleBProposed: cr.roleBProposed, RoleBRejected: cr.roleBRejected, RoleBNoTrigger: cr.roleBNoTrigger,
		Halted: cr.halted, TokenCost: cr.totals.CostMicros, DurationMS: cr.durationMS,
		CeilingRows: cr.ceilingRows,
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
		if cr.spec.Arm == ArmA3 || cr.spec.Arm == ArmA0 {
			continue
		}
		ref := referenceRun(runs, cr.spec)
		if ref == nil {
			continue
		}
		out = append(out, fidelityRow{
			Arm: cr.spec.Arm, F: cr.spec.F, Seed: cr.spec.Seed, Reference: ArmA3,
			Indicators: divergenceVector(cr, ref),
		})
	}
	return out
}

func referenceRun(runs []*cellRun, spec cellSpec) *cellRun {
	for _, cr := range runs {
		if cr.spec.Arm == ArmA3 && cr.spec.F == spec.F && cr.spec.Seed == spec.Seed {
			return cr
		}
	}
	return nil
}

func divergenceVector(b, r *cellRun) []fidelityIndicator {
	n := min(len(b.proj.Hours), len(r.proj.Hours))
	perActorSlots := float64(8 * b.fp.Patches)
	population := make([]float64, n)
	births := make([]float64, n)
	deathsStarve := make([]float64, n)
	deathsAge := make([]float64, n)
	capital := make([]float64, n)
	granary := make([]float64, n)
	exchange := make([]float64, n)
	for i := 0; i < n; i++ {
		population[i] = math.Abs(float64(b.proj.AliveByHour[i]-r.proj.AliveByHour[i])) / perActorSlots
		births[i] = math.Abs(float64(b.proj.BirthsCum[i] - r.proj.BirthsCum[i]))
		deathsStarve[i] = math.Abs(float64(b.proj.DeathsStarveCum[i] - r.proj.DeathsStarveCum[i]))
		deathsAge[i] = math.Abs(float64(b.proj.DeathsAgeCum[i] - r.proj.DeathsAgeCum[i]))
		capital[i] = totalVariation(b.proj.CapitalHist[i], r.proj.CapitalHist[i])
		granary[i] = totalVariation(b.proj.GranaryHist[i], r.proj.GranaryHist[i])
		exchange[i] = math.Abs(float64(b.proj.WagesCum[i] - r.proj.WagesCum[i]))
	}
	indicators := []fidelityIndicator{
		summarize(1, "population", population),
		summarize(2, "births", births),
		summarize(3, "deaths/starvation", deathsStarve),
		summarize(4, "deaths/age", deathsAge),
		summarize(5, "capital-histogram", capital),
		summarize(6, "granary-histogram", granary),
		summarize(7, "exchange-volume", exchange),
		decisionErrorIndicator(b, r, n),
	}
	return indicators
}

func summarize(index int, name string, seq []float64) fidelityIndicator {
	ind := fidelityIndicator{Index: index, Name: name, Sequence: append([]float64(nil), seq...), FirstDivergenceHour: -1}
	var sum float64
	for i, v := range seq {
		if v > ind.Max {
			ind.Max = v
		}
		if v != 0 && ind.FirstDivergenceHour < 0 {
			ind.FirstDivergenceHour = i
		}
		sum += v
	}
	if len(seq) > 0 {
		ind.Mean = sum / float64(len(seq))
	}
	return ind
}

// decisionErrorIndicator is the escalation decision-error rate (indicator 7):
// the absolute difference of the two cumulative ratios. Level-2 world refusals
// are not recorded as escalations, so this is the recorded level-1 rate; the
// label states that.
func decisionErrorIndicator(b, r *cellRun, n int) fidelityIndicator {
	bSeq := make([]float64, n)
	rSeq := make([]float64, n)
	bec := cumulativeHourly(b.interventions)
	rec := cumulativeHourly(r.interventions)
	for i := 0; i < n; i++ {
		hour := b.proj.Hours[i]
		bSeq[i] = rate(bec, hour)
		rSeq[i] = rate(rec, hour)
	}
	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		diff[i] = math.Abs(bSeq[i] - rSeq[i])
	}
	ind := summarize(8, "decision-error-rate(level-1)", diff)
	return ind
}

type hourlyCounts struct {
	escalated []int
	errors    []int
}

func cumulativeHourly(interventions []cognition.InterventionRecord) hourlyCounts {
	hc := hourlyCounts{escalated: make([]int, 169), errors: make([]int, 169)}
	for _, rec := range interventions {
		if rec.Kind != cognition.InterventionRoleA {
			continue
		}
		h := rec.Cause.Hour
		if h < 0 || h > 168 {
			continue
		}
		hc.escalated[h]++
		if rec.RoleA != nil && rec.RoleA.Source != cognition.SourceModel {
			hc.errors[h]++
		}
	}
	return hc
}

func rate(hc hourlyCounts, hour int) float64 {
	esc, errs := 0, 0
	for h := 0; h <= hour && h < len(hc.escalated); h++ {
		esc += hc.escalated[h]
		errs += hc.errors[h]
	}
	if esc == 0 {
		return 0
	}
	return float64(errs) / float64(esc)
}

func totalVariation(a, b []int64) float64 {
	var sa, sb int64
	for _, v := range a {
		sa += v
	}
	for _, v := range b {
		sb += v
	}
	if sa == 0 && sb == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < len(a) && i < len(b); i++ {
		pa, pb := 0.0, 0.0
		if sa > 0 {
			pa = float64(a[i]) / float64(sa)
		}
		if sb > 0 {
			pb = float64(b[i]) / float64(sb)
		}
		sum += math.Abs(pa - pb)
	}
	return sum / 2
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sortRuns orders runs by arm then cell for stable reports.
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
