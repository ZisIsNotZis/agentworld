package main

import (
	"testing"

	"agentworld/internal/cognition/protocol"
)

// TestDeclaredCellInventory pins the frozen cell set (spec section 11.3).
func TestDeclaredCellInventory(t *testing.T) {
	cells := VS1DeclaredCells()
	if len(cells) != 12 {
		t.Fatalf("declared cells %d, want 12", len(cells))
	}
	seen := map[[2]uint64]bool{}
	for _, c := range cells {
		if c[0] < 10 || c[0] > 12 || c[1] > 3 {
			t.Fatalf("cell outside the declared band: %v", c)
		}
		if seen[c] {
			t.Fatalf("duplicate cell %v", c)
		}
		seen[c] = true
	}
}

func TestParseCells(t *testing.T) {
	got := parseCells("f12-s3,f10-s0")
	if len(got) != 2 || got[0] != [2]uint64{12, 3} || got[1] != [2]uint64{10, 0} {
		t.Fatalf("parseCells %v", got)
	}
}

// TestTriggerIndicatorMirrorsDeclaredRule pins the sandbox scoring rule to the
// declared Role B trigger indicator (internal/cognition/roleb.go).
func TestTriggerIndicatorMirrorsDeclaredRule(t *testing.T) {
	if got := triggerIndicatorOf(protocol.TrajectoryEvidence{BirthsTotal: 0, MaxCapital: 4}); got != 4 {
		t.Fatalf("no-birth indicator %d, want MaxCapital 4", got)
	}
	if got := triggerIndicatorOf(protocol.TrajectoryEvidence{BirthsTotal: 3, BuildsCommitted: 0, MaxCapital: 4}); got != 0 {
		t.Fatalf("no-build indicator %d, want BuildsCommitted 0", got)
	}
	if got := triggerIndicatorOf(protocol.TrajectoryEvidence{BirthsTotal: 3, BuildsCommitted: 2, MaxCapital: 1}); got != 1 {
		t.Fatalf("capital indicator %d, want MaxCapital 1", got)
	}
}

func TestTriggerPredicate(t *testing.T) {
	if _, fired := triggerPredicateOf(protocol.TrajectoryEvidence{BirthsTotal: 1, BuildsCommitted: 1, MaxCapital: 3}); fired {
		t.Fatal("predicate fired on a healthy packet")
	}
	if name, fired := triggerPredicateOf(protocol.TrajectoryEvidence{BirthsTotal: 0}); !fired || name != "no-birth" {
		t.Fatalf("no-birth predicate %q %t", name, fired)
	}
}

// TestTotalVariationIsBounded pins the indicator-4/5 distance normalisation.
func TestTotalVariationIsBounded(t *testing.T) {
	if got := totalVariation([]int64{0, 2}, []int64{0, 2}); got != 0 {
		t.Fatalf("identical histograms distance %v", got)
	}
	if got := totalVariation([]int64{2, 0}, []int64{0, 2}); got != 1 {
		t.Fatalf("disjoint histograms distance %v, want 1", got)
	}
}

// TestDivergesDetectsIndicatorChange pins the EC-2 divergence comparison.
func TestDivergesDetectsIndicatorChange(t *testing.T) {
	a := &cellRun{}
	b := &cellRun{}
	if diverges(a, b) {
		t.Fatal("identical empty runs reported as divergent")
	}
	a.proj.AliveByHour = []int{1, 2}
	b.proj.AliveByHour = []int{1, 3}
	if !diverges(a, b) {
		t.Fatal("an indicator change was not detected")
	}
}
