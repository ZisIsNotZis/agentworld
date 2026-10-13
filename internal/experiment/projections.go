// Package experiment holds the arm-agnostic mechanics shared by the VS1 and VS2
// comparison runners: the hourly projection extraction every arm reads, and the
// frozen multi-indicator divergence vector measured against a reference arm
// (VS2 spec section 6, inherited from the VS1 scaffold in cmd/vs1). It carries
// no arm, fixture, or stack wiring of its own, so the fidelity metric has one
// home instead of a copy per runner (VS2 spec section 10, lane L4).
package experiment

import "agentworld/internal/world"

// declaredKMax is the capital histogram's declared upper edge (spec section
// 11.5 indicator 4). It is the frozen natural/7 capital gate k>=2's own
// reading of the reference's observed maximum (8 under natural/9's k=8 arc).
const declaredKMax = 8

// granaryHistogramEdges are the declared granary-stock buckets (indicator 5).
var granaryHistogramEdges = []int64{0, 1, 2, 3, 5, 9}

// Projections are the hourly indicator sequences the fidelity vector (spec
// section 11.5) is built from, one entry per published checkpoint.
type Projections struct {
	Hours           []int
	AliveByHour     []int
	BirthsCum       []int64
	DeathsStarveCum []int64
	DeathsAgeCum    []int64
	CapitalHist     [][]int64 // KMax+1 buckets per hour
	GranaryHist     [][]int64 // len(granaryHistogramEdges)+1 buckets per hour
	WagesCum        []int64
	MaxCapital      []int64
}

// AliveActors returns the checkpoint's living, fed actors (founders and
// newborns), the population the per-hour capital and granary histograms are
// drawn over.
func AliveActors(cp world.NaturalCheckpoint) []world.NaturalActorState {
	out := make([]world.NaturalActorState, 0, len(cp.Founders)+len(cp.Newborns))
	for _, a := range cp.Founders {
		if a.Genome.DiedHour == world.NaturalNeverHour && a.Body.Energy > 0 {
			out = append(out, a)
		}
	}
	for _, nb := range cp.Newborns {
		if nb.State.Genome.DiedHour == world.NaturalNeverHour && nb.State.Body.Energy > 0 {
			out = append(out, nb.State)
		}
	}
	return out
}

// ExtractProjections folds the published checkpoints into the hourly indicator
// sequences the fidelity vector and the Role B evidence packet are computed
// from.
func ExtractProjections(checks []world.NaturalCheckpoint) Projections {
	p := Projections{
		Hours:           make([]int, len(checks)),
		AliveByHour:     make([]int, len(checks)),
		BirthsCum:       make([]int64, len(checks)),
		DeathsStarveCum: make([]int64, len(checks)),
		DeathsAgeCum:    make([]int64, len(checks)),
		CapitalHist:     make([][]int64, len(checks)),
		GranaryHist:     make([][]int64, len(checks)),
		WagesCum:        make([]int64, len(checks)),
		MaxCapital:      make([]int64, len(checks)),
	}
	for i, cp := range checks {
		p.Hours[i] = cp.Hour
		p.AliveByHour[i] = cp.Metrics.Alive
		p.BirthsCum[i] = cp.Metrics.Births
		p.DeathsStarveCum[i] = cp.Metrics.DeathsStarvation
		p.DeathsAgeCum[i] = cp.Metrics.DeathsAge
		p.WagesCum[i] = cp.Balance.WagesPaid
		buckets := make([]int64, declaredKMax+1)
		granary := make([]int64, len(granaryHistogramEdges)+1)
		var maxCapital int64
		for _, a := range AliveActors(cp) {
			k := a.Worksite.Capital
			if k < 0 {
				k = 0
			}
			if k > declaredKMax {
				k = declaredKMax
			}
			buckets[k]++
			granary[granaryBucket(a.Granary.Stock)]++
			if a.Worksite.Capital > maxCapital {
				maxCapital = a.Worksite.Capital
			}
		}
		p.CapitalHist[i] = buckets
		p.GranaryHist[i] = granary
		p.MaxCapital[i] = maxCapital
	}
	return p
}

func granaryBucket(stock int64) int {
	b := 0
	for i := 0; i < len(granaryHistogramEdges); i++ {
		if stock >= granaryHistogramEdges[i] {
			b = i
		}
	}
	return b
}
