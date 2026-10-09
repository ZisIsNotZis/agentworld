package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// The evidence extraction shared by the arms. Everything here is a pure
// function of the committed checkpoints, the accepted history, and the journal
// that the runner already publishes (spec section 5.1): no actor-private state
// beyond the published rows, and no world reader.

// declaredKMax is the capital histogram's declared upper edge (spec section
// 11.5 indicator 4). It is the frozen natural/7 capital gate k>=2's own
// reading of the reference's observed maximum (8 under natural/9's k=8 arc).
const declaredKMax = 8

// granaryHistogramEdges are the declared granary-stock buckets (indicator 5).
var granaryHistogramEdges = []int64{0, 1, 2, 3, 5, 9}

// worldRuleNames names every natural rule that can appear in accepted history.
// It mirrors cmd/natural's map; an unknown rule is a hard error, never a
// silent bucket.
var worldRuleNames = map[sim.RuleID]string{
	world.NaturalRegenSplitRule:       "regen_split",
	world.NaturalSpoilageRule:         "spoilage",
	world.NaturalWearRule:             "wear",
	world.NaturalBasalRule:            "basal",
	world.NaturalBuildRule:            "build",
	world.NaturalGatherRule:           "gather",
	world.NaturalConsumeRule:          "consume",
	world.NaturalGrantRule:            "grant",
	world.NaturalEatStoredRule:        "eat_stored",
	world.NaturalBirthRequestRule:     "birth_request",
	world.NaturalBirthReplyRule:       "birth_reply",
	world.NaturalBirthRefuseRule:      "birth_refuse",
	world.NaturalBirthExpireRule:      "birth_expiry",
	world.NaturalAgeDeathRule:         "age_death",
	world.NaturalProvisionRule:        "provision",
	world.NaturalLabourOfferRule:      "labour_offer",
	world.NaturalLabourDecisionRule:   "labour_decision",
	world.NaturalLabourSettlementRule: "labour_settlement",
	world.NaturalLabourExpireRule:     "labour_expiry",
}

// acceptedEvidence is the accepted-history identity and event inventory.
type acceptedEvidence struct {
	TipHash        string
	ProjectionHash string
	Version        uint64
	HistoryBytes   int
	EventCount     int
	EventBodyBytes int
	EventCounts    map[string]int64
}

// extractAccepted replays the accepted history into its event inventory. It
// refuses (fail loud) an unknown rule or a head mismatch.
func extractAccepted(handoff world.NaturalHandoff) (acceptedEvidence, error) {
	reg, err := world.NaturalRegistryFor(world.NaturalMechanismV6)
	if err != nil {
		return acceptedEvidence{}, err
	}
	k, head, err := kernel.RestoreHistory(reg, handoff.History)
	if err != nil {
		return acceptedEvidence{}, fmt.Errorf("accepted history replay: %w", err)
	}
	if head != handoff.Head {
		return acceptedEvidence{}, fmt.Errorf("restored history head mismatch")
	}
	events := k.Events()
	out := acceptedEvidence{
		TipHash:        hex.EncodeToString(head.TipHash[:]),
		ProjectionHash: hex.EncodeToString(head.ProjectionHash[:]),
		Version:        uint64(head.Version),
		HistoryBytes:   len(handoff.History),
		EventCount:     len(events),
		EventCounts:    make(map[string]int64),
	}
	for _, ev := range events {
		body, err := ev.Bytes()
		if err != nil {
			return acceptedEvidence{}, err
		}
		out.EventBodyBytes += len(body)
		name, ok := worldRuleNames[ev.Rule]
		if !ok {
			return acceptedEvidence{}, fmt.Errorf("unknown natural rule %d in accepted history", ev.Rule)
		}
		out.EventCounts[name]++
	}
	return out, nil
}

// projections are the hourly indicator sequences the fidelity vector (spec
// section 11.5) is built from.
type projections struct {
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

func aliveActors(cp world.NaturalCheckpoint) []world.NaturalActorState {
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

func extractProjections(checks []world.NaturalCheckpoint) projections {
	p := projections{
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
		for _, a := range aliveActors(cp) {
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

// buildEvidence assembles the Role B aggregate packet for the window
// [startHour, startHour+window] from the published checkpoints and journal
// (spec section 5.1). Only world-level aggregates enter it.
func buildEvidence(checks []world.NaturalCheckpoint, journal []world.NaturalBatch, startHour, window int) protocol.TrajectoryEvidence {
	ev := protocol.TrajectoryEvidence{
		WindowStartHour: startHour,
		WindowHours:     window,
		BirthsByCause:   map[string]int64{},
		DeathsByCause:   map[string]int64{},
	}
	first := -1
	last := -1
	for i := range checks {
		if checks[i].Hour == startHour {
			first = i
		}
		if checks[i].Hour == startHour+window {
			last = i
		}
	}
	if first < 0 || last < 0 || last < first {
		return ev
	}
	for i := first; i <= last; i++ {
		cp := checks[i]
		ev.AliveByHour = append(ev.AliveByHour, cp.Metrics.Alive)
		if cp.Metrics.Alive > ev.MaxAlive {
			ev.MaxAlive = cp.Metrics.Alive
		}
		for _, a := range aliveActors(cp) {
			if a.Worksite.Capital > ev.MaxCapital {
				ev.MaxCapital = a.Worksite.Capital
			}
		}
	}
	ev.BirthsTotal = checks[last].Metrics.Births - checks[first].Metrics.Births
	if ev.BirthsTotal > ev.MaxBirths {
		ev.MaxBirths = ev.BirthsTotal
	}
	ev.DeathsTotal = (checks[last].Metrics.DeathsStarvation + checks[last].Metrics.DeathsAge) -
		(checks[first].Metrics.DeathsStarvation + checks[first].Metrics.DeathsAge)
	ev.DeathsByCause["starvation"] = checks[last].Metrics.DeathsStarvation - checks[first].Metrics.DeathsStarvation
	ev.DeathsByCause["age"] = checks[last].Metrics.DeathsAge - checks[first].Metrics.DeathsAge
	ev.BirthsByCause["accepted"] = ev.BirthsTotal
	for _, a := range aliveActors(checks[last]) {
		ev.Actors = append(ev.Actors, protocol.ActorEvidence{Actor: a.Bag.Source, Capital: a.Worksite.Capital, GranaryStock: a.Granary.Stock})
	}
	mouths := map[sim.EntityID]int64{}
	patchCapital := map[sim.EntityID]int64{}
	for _, a := range aliveActors(checks[last]) {
		mouths[a.Bag.Source]++
		patchCapital[a.Bag.Source] += a.Worksite.Capital
	}
	patches := make([]sim.EntityID, 0, len(mouths))
	for patch := range mouths {
		patches = append(patches, patch)
	}
	sort.Slice(patches, func(i, j int) bool { return patches[i] < patches[j] })
	for _, patch := range patches {
		ev.Patches = append(ev.Patches, protocol.PatchEvidence{Patch: patch, Mouths: mouths[patch], Capital: patchCapital[patch]})
	}
	for _, batch := range journal {
		hour := int(batch.Time / sim.SimTime(world.NaturalHour))
		if hour <= startHour || hour > startHour+window {
			continue
		}
		for _, at := range batch.Attempts {
			if !at.Accepted {
				continue
			}
			switch at.Kind {
			case "build":
				ev.BuildsCommitted++
			case "eat", "eat-stored", "paired-meal":
				ev.MealsCommitted++
			}
		}
	}
	return ev
}

// triggerPredicateOf mirrors the declared Role B trigger predicate
// (internal/cognition.roleb.go TriggerPredicate); the returned name is
// recorded.
func triggerPredicateOf(ev protocol.TrajectoryEvidence) (string, bool) {
	switch {
	case ev.BirthsTotal == 0:
		return "no-birth", true
	case ev.BuildsCommitted == 0:
		return "no-build", true
	case ev.MaxCapital < 2:
		return "capital-below-birth-gate", true
	}
	return "", false
}

// branchStateDigest is the deterministic digest of a sandbox branch's state at
// its horizon: the version, the final hours' published aggregates, and the
// journal shape. It is recorded so an outside party can re-run the fork.
func branchStateDigest(checks []world.NaturalCheckpoint, journal []world.NaturalBatch) [32]byte {
	type row struct {
		Hour   int
		Alive  int
		Births int64
		Wages  int64
	}
	rows := make([]row, 0, len(checks))
	for _, cp := range checks {
		rows = append(rows, row{Hour: cp.Hour, Alive: cp.Metrics.Alive, Births: cp.Metrics.Births, Wages: cp.Balance.WagesPaid})
	}
	body, _ := json.Marshal(struct {
		Rows    []row
		Journal int
	}{rows, len(journal)})
	return sha256.Sum256(body)
}
