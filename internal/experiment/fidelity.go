package experiment

import (
	"fmt"
	"math"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// FidelityIndicator is one indicator's divergence sequence over the declared
// horizon plus its two lower-is-better summaries and the first hour it
// diverged (-1 when it never did; spec section 6).
type FidelityIndicator struct {
	Index               int       `json:"index"`
	Name                string    `json:"name"`
	FirstDivergenceHour int       `json:"first_divergence_hour"`
	Max                 float64   `json:"max"`
	Mean                float64   `json:"mean"`
	Sequence            []float64 `json:"sequence"`
}

// Arm is one run's input to the fidelity vector: its hourly projections, the
// fixture's patch count (the population normaliser), and the recorded
// interventions and accepted journal the decision-error rate is joined from.
type Arm struct {
	Label         string
	Projections   Projections
	Patches       int
	Interventions []cognition.InterventionRecord
	Journal       []world.NaturalBatch
}

// ShortProjectionError refuses a comparison over a truncated prefix: the arm's
// hourly projection did not reach the declared horizon (spec section 5.3).
type ShortProjectionError struct {
	Label           string
	ActualSamples   int
	RequiredSamples int
}

func (e *ShortProjectionError) Error() string {
	return fmt.Sprintf("experiment: arm %s hourly projection has %d samples, shorter than the declared horizon's %d; the divergence vector is refused rather than silently truncated to a shorter prefix", e.Label, e.ActualSamples, e.RequiredSamples)
}

// DivergenceVector computes the frozen eight-indicator divergence vector of a
// budgeted arm b against the reference arm r over the full declared horizon:
// hours 0..horizonHours, i.e. horizonHours+1 hourly samples. It computes over
// the whole horizon and refuses (never inherits the VS1 scaffold's
// min(len(b), len(r)) prefix) an arm whose hourly projection is shorter than
// the declared horizon, returning a *ShortProjectionError the caller reports
// (VS2 spec section 5.3/10).
func DivergenceVector(b, r Arm, horizonHours int) ([]FidelityIndicator, error) {
	n := horizonHours + 1
	for _, arm := range []Arm{b, r} {
		if len(arm.Projections.Hours) < n {
			return nil, &ShortProjectionError{Label: arm.Label, ActualSamples: len(arm.Projections.Hours), RequiredSamples: n}
		}
	}
	perActorSlots := float64(8 * b.Patches)
	population := make([]float64, n)
	births := make([]float64, n)
	deathsStarve := make([]float64, n)
	deathsAge := make([]float64, n)
	capital := make([]float64, n)
	granary := make([]float64, n)
	exchange := make([]float64, n)
	for i := 0; i < n; i++ {
		population[i] = math.Abs(float64(b.Projections.AliveByHour[i]-r.Projections.AliveByHour[i])) / perActorSlots
		births[i] = math.Abs(float64(b.Projections.BirthsCum[i] - r.Projections.BirthsCum[i]))
		deathsStarve[i] = math.Abs(float64(b.Projections.DeathsStarveCum[i] - r.Projections.DeathsStarveCum[i]))
		deathsAge[i] = math.Abs(float64(b.Projections.DeathsAgeCum[i] - r.Projections.DeathsAgeCum[i]))
		capital[i] = TotalVariation(b.Projections.CapitalHist[i], r.Projections.CapitalHist[i])
		granary[i] = TotalVariation(b.Projections.GranaryHist[i], r.Projections.GranaryHist[i])
		exchange[i] = math.Abs(float64(b.Projections.WagesCum[i] - r.Projections.WagesCum[i]))
	}
	decisionError, err := DecisionErrorIndicator(b, r, horizonHours)
	if err != nil {
		return nil, err
	}
	return []FidelityIndicator{
		Summarize(1, "population", population),
		Summarize(2, "births", births),
		Summarize(3, "deaths/starvation", deathsStarve),
		Summarize(4, "deaths/age", deathsAge),
		Summarize(5, "capital-histogram", capital),
		Summarize(6, "granary-histogram", granary),
		Summarize(7, "exchange-volume", exchange),
		decisionError,
	}, nil
}

// Summarize folds one indicator's hourly divergence sequence into its summary:
// the maximum, the mean, and the first hour the sequence is non-zero (-1 when
// it never diverges over the horizon).
func Summarize(index int, name string, seq []float64) FidelityIndicator {
	ind := FidelityIndicator{Index: index, Name: name, Sequence: append([]float64(nil), seq...), FirstDivergenceHour: -1}
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

// DecisionErrorIndicator is the escalation decision-error rate (indicator 8):
// the absolute difference of the two cumulative ratios over the declared
// horizon. The numerator counts both level-1 (malformed/inadmissible fallback)
// and level-2 (typed world refusal) outcomes of escalated decisions, the latter
// joined from the accepted journal by (hour, actor). It refuses a short
// projection exactly as DivergenceVector does.
func DecisionErrorIndicator(b, r Arm, horizonHours int) (FidelityIndicator, error) {
	n := horizonHours + 1
	for _, arm := range []Arm{b, r} {
		if len(arm.Projections.Hours) < n {
			return FidelityIndicator{}, &ShortProjectionError{Label: arm.Label, ActualSamples: len(arm.Projections.Hours), RequiredSamples: n}
		}
	}
	bSeq := make([]float64, n)
	rSeq := make([]float64, n)
	bec := cumulativeHourly(b.Interventions, b.Journal, n)
	rec := cumulativeHourly(r.Interventions, r.Journal, n)
	for i := 0; i < n; i++ {
		hour := b.Projections.Hours[i]
		bSeq[i] = rate(bec, hour)
		rSeq[i] = rate(rec, hour)
	}
	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		diff[i] = math.Abs(bSeq[i] - rSeq[i])
	}
	return Summarize(8, "decision-error-rate(level-1+level-2)", diff), nil
}

type hourlyCounts struct {
	escalated []int
	errors    []int
	level2    []int
}

func cumulativeHourly(interventions []cognition.InterventionRecord, journal []world.NaturalBatch, samples int) hourlyCounts {
	hc := hourlyCounts{escalated: make([]int, samples), errors: make([]int, samples), level2: make([]int, samples)}
	modelEscalated := map[[2]int]bool{}
	for _, rec := range interventions {
		if rec.Kind != cognition.InterventionRoleA {
			continue
		}
		h := rec.Cause.Hour
		if h < 0 || h >= samples {
			continue
		}
		hc.escalated[h]++
		if rec.RoleA != nil && rec.RoleA.Source == cognition.SourceModel {
			modelEscalated[[2]int{h, int(rec.Cause.Actor)}] = true
		} else {
			hc.errors[h]++
		}
	}
	hc.level2 = Level2RefusalCounts(journal, modelEscalated, samples)
	return hc
}

// ModelEscalatedKeySet is the set of (hour, actor) escalated decisions whose
// model action was applied (source == model).
func ModelEscalatedKeySet(interventions []cognition.InterventionRecord) map[[2]int]bool {
	out := map[[2]int]bool{}
	for _, rec := range interventions {
		if rec.Kind == cognition.InterventionRoleA && rec.RoleA != nil && rec.RoleA.Source == cognition.SourceModel {
			out[[2]int{rec.Cause.Hour, int(rec.Cause.Actor)}] = true
		}
	}
	return out
}

// Level2RefusalCounts joins the accepted journal to the escalated decisions
// whose model action was applied (source == model): an applied model claim whose
// committed attempt carries an explicit refusal marker is a level-2 world
// refusal (spec 4.3). A fallback/inert decision is already counted as a level-1
// error, and the frozen path's own denials are not model decision errors, so
// neither is double-counted here. The returned slice is indexed by hour over
// [0, samples).
func Level2RefusalCounts(journal []world.NaturalBatch, modelEscalated map[[2]int]bool, samples int) []int {
	counts := make([]int, samples)
	for _, batch := range journal {
		hour := int(batch.Time / sim.SimTime(world.NaturalHour))
		if hour < 0 || hour >= samples {
			continue
		}
		for _, at := range batch.Attempts {
			switch at.Kind {
			case "gather", "provision", "eat-stored":
			default:
				continue
			}
			if !modelEscalated[[2]int{hour, int(at.Actor)}] {
				continue
			}
			if at.Rejection != world.FoodFlowGatherAdmitted || at.ProvisionRefused != "" || at.NoMeal != "" {
				counts[hour]++
			}
		}
	}
	return counts
}

func rate(hc hourlyCounts, hour int) float64 {
	esc, errs := 0, 0
	for h := 0; h <= hour && h < len(hc.escalated); h++ {
		esc += hc.escalated[h]
		errs += hc.errors[h] + hc.level2[h]
	}
	if esc == 0 {
		return 0
	}
	return float64(errs) / float64(esc)
}

// TotalVariation is the total-variation distance between two histograms, each
// normalised by its own total (0 when both are empty).
func TotalVariation(a, b []int64) float64 {
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
