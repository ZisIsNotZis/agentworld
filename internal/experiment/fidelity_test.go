package experiment

import (
	"errors"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// fullArm builds an arm whose hourly projection covers hours 0..hours, with
// every sequence the divergence vector reads allocated.
func fullArm(label string, hours int) Arm {
	n := hours + 1
	p := Projections{
		Hours:           make([]int, n),
		AliveByHour:     make([]int, n),
		BirthsCum:       make([]int64, n),
		DeathsStarveCum: make([]int64, n),
		DeathsAgeCum:    make([]int64, n),
		CapitalHist:     make([][]int64, n),
		GranaryHist:     make([][]int64, n),
		WagesCum:        make([]int64, n),
		MaxCapital:      make([]int64, n),
	}
	for i := 0; i < n; i++ {
		p.Hours[i] = i
		p.CapitalHist[i] = []int64{1}
		p.GranaryHist[i] = []int64{1}
	}
	return Arm{Label: label, Patches: 1, Projections: p}
}

// TestDivergenceVectorUsesTheDeclaredHorizon pins the full-H computation: two
// arms carrying more samples than the declared horizon are compared over
// exactly H+1 samples, never the scaffold's min(len(b), len(r)) prefix.
func TestDivergenceVectorUsesTheDeclaredHorizon(t *testing.T) {
	got, err := DivergenceVector(fullArm("b", 6), fullArm("r", 6), 4)
	if err != nil {
		t.Fatalf("DivergenceVector: %v", err)
	}
	if len(got) != 8 {
		t.Fatalf("indicators %d, want the eight declared indicators", len(got))
	}
	for _, ind := range got {
		if len(ind.Sequence) != 5 {
			t.Fatalf("indicator %d has %d samples, want the declared horizon's 5", ind.Index, len(ind.Sequence))
		}
		if ind.FirstDivergenceHour != -1 || ind.Max != 0 || ind.Mean != 0 {
			t.Fatalf("identical arms diverged on indicator %d: %+v", ind.Index, ind)
		}
	}
}

// TestDivergenceVectorReportsFullHorizonDivergence pins that a difference at
// the final declared hour is reported, not dropped.
func TestDivergenceVectorReportsFullHorizonDivergence(t *testing.T) {
	b := fullArm("b", 4)
	r := fullArm("r", 4)
	r.Projections.BirthsCum[4] = 3
	got, err := DivergenceVector(b, r, 4)
	if err != nil {
		t.Fatalf("DivergenceVector: %v", err)
	}
	if births := got[1]; births.Name != "births" || births.FirstDivergenceHour != 4 || births.Max != 3 {
		t.Fatalf("births indicator %+v, want divergence first at hour 4 with max 3", births)
	}
}

// TestDivergenceVectorRefusesShortProjection pins the fail-loud refusal of a
// projection shorter than the declared horizon, naming the arm, its actual
// samples, and the required count.
func TestDivergenceVectorRefusesShortProjection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		b, r     Arm
		wantArm  string
		wantHave int
	}{
		{"budgeted short", fullArm("budgeted", 3), fullArm("reference", 4), "budgeted", 4},
		{"reference short", fullArm("budgeted", 4), fullArm("reference", 2), "reference", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DivergenceVector(tc.b, tc.r, 4)
			var short *ShortProjectionError
			if !errors.As(err, &short) {
				t.Fatalf("short projection not refused: err=%v", err)
			}
			if short.Label != tc.wantArm || short.ActualSamples != tc.wantHave || short.RequiredSamples != 5 {
				t.Fatalf("short projection error %+v, want arm %s %d of 5", short, tc.wantArm, tc.wantHave)
			}
		})
	}
	if _, err := DecisionErrorIndicator(fullArm("b", 3), fullArm("r", 4), 4); err == nil {
		t.Fatal("DecisionErrorIndicator accepted a short projection")
	}
}

// TestDecisionErrorIndicatorRatesAndLevel2 pins indicator 8's cumulative ratio
// and that the numerator counts both level-1 fallbacks and level-2 world
// refusals.
func TestDecisionErrorIndicatorRatesAndLevel2(t *testing.T) {
	model := func(hour int, actor uint8) cognition.InterventionRecord {
		return cognition.InterventionRecord{
			Kind:  cognition.InterventionRoleA,
			Cause: cognition.InterventionCause{Hour: hour, Actor: sim.EntityID(actor)},
			RoleA: &cognition.RoleARecord{Source: cognition.SourceModel},
		}
	}
	fallback := func(hour int, actor uint8) cognition.InterventionRecord {
		return cognition.InterventionRecord{
			Kind:  cognition.InterventionRoleA,
			Cause: cognition.InterventionCause{Hour: hour, Actor: sim.EntityID(actor)},
			RoleA: &cognition.RoleARecord{Source: "fallback"},
		}
	}
	// Level-1: b escalates a clean model decision at hour 0 and a fallback at
	// hour 1, so its cumulative error rate never diverges from the reference
	// until hour 1, then stays at 1/2.
	b := fullArm("b", 4)
	b.Interventions = []cognition.InterventionRecord{model(0, 1), fallback(1, 2)}
	r := fullArm("r", 4)
	r.Interventions = []cognition.InterventionRecord{model(0, 1)}
	ind, err := DecisionErrorIndicator(b, r, 4)
	if err != nil {
		t.Fatalf("DecisionErrorIndicator: %v", err)
	}
	if ind.Name != "decision-error-rate(level-1+level-2)" {
		t.Fatalf("indicator name %q", ind.Name)
	}
	if ind.FirstDivergenceHour != 1 || ind.Sequence[0] != 0 || ind.Sequence[1] != 0.5 || ind.Sequence[4] != 0.5 {
		t.Fatalf("decision-error sequence %v", ind.Sequence)
	}
	// Level-2: the reference's clean model decision at hour 0 is applied but
	// the world refuses the committed attempt, so b diverges immediately.
	b2 := fullArm("b2", 4)
	b2.Interventions = []cognition.InterventionRecord{model(0, 1)}
	b2.Journal = []world.NaturalBatch{{
		Time: 0,
		Attempts: []world.NaturalAttempt{
			{Actor: 1, Kind: "gather", Rejection: 1},
		},
	}}
	ind2, err := DecisionErrorIndicator(b2, r, 4)
	if err != nil {
		t.Fatalf("DecisionErrorIndicator: %v", err)
	}
	if ind2.Sequence[0] != 1 || ind2.FirstDivergenceHour != 0 {
		t.Fatalf("level-2 refusal sequence %v", ind2.Sequence)
	}
}

// TestLevel2RefusalCountsJoinsJournal pins the journal join: only applied
// model decisions on a refused committed attempt count, and only for hours
// inside the declared sample window.
func TestLevel2RefusalCountsJoinsJournal(t *testing.T) {
	model := map[[2]int]bool{{0, 7}: true, {9, 7}: true}
	journal := []world.NaturalBatch{
		{Time: 0, Attempts: []world.NaturalAttempt{
			{Actor: 7, Kind: "gather", Rejection: world.FoodFlowGatherAdmitted},
			{Actor: 7, Kind: "provision", ProvisionRefused: "no-stock"},
			{Actor: 9, Kind: "gather", Rejection: 1},
		}},
		{Time: sim.SimTime(world.NaturalHour) * 9, Attempts: []world.NaturalAttempt{
			{Actor: 7, Kind: "gather", Rejection: 1},
		}},
	}
	counts := Level2RefusalCounts(journal, model, 5)
	if len(counts) != 5 || counts[0] != 1 {
		t.Fatalf("counts %v, want exactly the hour-0 provision refusal", counts)
	}
}

// TestSummarizeNoDivergenceSentinel pins the -1 no-divergence encoding: a
// never-diverged sequence reports FirstDivergenceHour -1, the best outcome, and
// its Max/Mean stay zero.
func TestSummarizeNoDivergenceSentinel(t *testing.T) {
	ind := Summarize(2, "births", []float64{0, 0, 0})
	if ind.FirstDivergenceHour != -1 || ind.Max != 0 || ind.Mean != 0 {
		t.Fatalf("no-divergence summary %+v", ind)
	}
	if seq := Summarize(2, "births", []float64{0, 2, 2, 0}); seq.FirstDivergenceHour != 1 || seq.Max != 2 {
		t.Fatalf("diverged summary %+v", seq)
	}
}
