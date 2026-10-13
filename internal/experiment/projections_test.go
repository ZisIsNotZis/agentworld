package experiment

import (
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// TestExtractProjectionsHistograms pins the hourly indicator extraction: only
// living, fed actors enter the capital and granary histograms, capital is
// clamped to the declared upper edge, and the cumulative metrics and wages are
// carried verbatim.
func TestExtractProjectionsHistograms(t *testing.T) {
	patch := sim.EntityID(1001)
	alive := func(capital, granary int64) world.NaturalActorState {
		return world.NaturalActorState{
			Body:     world.NaturalBodyState{Energy: 5},
			Bag:      world.NaturalBagState{Source: patch},
			Worksite: world.NaturalWorksiteState{Capital: capital},
			Granary:  world.NaturalGranaryState{Stock: granary},
			Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
		}
	}
	dead := alive(4, 0)
	dead.Genome.DiedHour = 3
	unfed := alive(4, 0)
	unfed.Body.Energy = 0
	checks := []world.NaturalCheckpoint{{
		Hour:     0,
		Founders: []world.NaturalActorState{alive(2, 3), alive(9, 0), dead, unfed},
		Newborns: []world.NaturalNewbornState{{ID: 5, State: alive(5, 5)}},
		Metrics:  world.NaturalMetrics{Alive: 3, Births: 1, DeathsStarvation: 2},
		Balance:  world.NaturalBalance{WagesPaid: 7},
	}}
	p := ExtractProjections(checks)
	if len(p.Hours) != 1 || p.Hours[0] != 0 {
		t.Fatalf("hours %v", p.Hours)
	}
	if p.AliveByHour[0] != 3 || p.BirthsCum[0] != 1 || p.DeathsStarveCum[0] != 2 || p.DeathsAgeCum[0] != 0 {
		t.Fatalf("metrics projection alive=%v births=%v starve=%v age=%v",
			p.AliveByHour, p.BirthsCum, p.DeathsStarveCum, p.DeathsAgeCum)
	}
	if p.WagesCum[0] != 7 {
		t.Fatalf("wages %v", p.WagesCum)
	}
	// Capital 9 clamps to the declared edge 8; the dead and unfed actors drop.
	wantCapital := []int64{0, 0, 1, 0, 0, 1, 0, 0, 1}
	if len(p.CapitalHist[0]) != len(wantCapital) {
		t.Fatalf("capital histogram %v", p.CapitalHist[0])
	}
	for i, want := range wantCapital {
		if p.CapitalHist[0][i] != want {
			t.Fatalf("capital histogram %v, want %v", p.CapitalHist[0], wantCapital)
		}
	}
	wantGranary := []int64{1, 0, 0, 1, 1, 0, 0}
	for i, want := range wantGranary {
		if p.GranaryHist[0][i] != want {
			t.Fatalf("granary histogram %v, want %v", p.GranaryHist[0], wantGranary)
		}
	}
	if p.MaxCapital[0] != 9 {
		t.Fatalf("max capital %v, want the unclamped maximum 9", p.MaxCapital)
	}
}
