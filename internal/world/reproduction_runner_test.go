package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

// The reproduction runner tests (ticket 14). Fast fixtures with small
// horizons: the full 168h matrix belongs to the experiment lane, so no unit
// test here runs the complete horizon; the longest walks are the h95 v3
// neutrality equality and the h144 age-death attribution.

func reproductionRunTo(t *testing.T, options ReproductionOptions, hour int) *Reproduction {
	t.Helper()
	f, err := NewReproduction(options)
	if err != nil {
		t.Fatal(err)
	}
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("hour %d step %d: processed=%v err=%v", hour, f.steps, ok, err)
		}
	}
	return f
}

func reproductionEventBytes(t *testing.T, f *Reproduction) [][]byte {
	t.Helper()
	events := f.k.Events()
	out := make([][]byte, len(events))
	for i, ev := range events {
		b, err := ev.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		out[i] = b
	}
	return out
}

// reproductionAssertConservation re-verifies the extended frozen gates at
// every hourly checkpoint and the frozen alive invariant on every row.
func reproductionAssertConservation(t *testing.T, f *Reproduction) {
	t.Helper()
	for _, check := range f.Checkpoints() {
		b, err := ReproductionCheckConservation(check.Patches, check.Slots, check.Founders, check.Newborns)
		if err != nil {
			t.Fatalf("h%d conservation: %v", check.Hour, err)
		}
		if b != check.Balance {
			t.Fatalf("h%d balance drift: %+v != %+v", check.Hour, b, check.Balance)
		}
		if check.Balance.InitialEnergy+ReproductionNewbornEnergy*check.Balance.Births+check.Balance.Consumed+check.Balance.StoredMeals !=
			check.Balance.Energy+check.Balance.BasalSpent+check.Balance.EnergyCapLost {
			t.Fatalf("h%d G3 extended: %+v", check.Hour, check.Balance)
		}
		aliveInvariant := func(actor sim.EntityID, a ReproductionActorState) {
			if reproductionAlive(a) != (a.Genome.DiedHour == ReproductionNeverHour && a.Body.Energy > 0) {
				t.Fatalf("h%d actor %d violates the alive invariant: %+v", check.Hour, actor, a)
			}
		}
		for i := range check.Founders {
			aliveInvariant(sim.EntityID(i+1), check.Founders[i])
		}
		for _, newborn := range check.Newborns {
			aliveInvariant(newborn.ID, newborn.State)
		}
	}
}

// reproductionAssertJournalClock pins every attempt to its frozen phase
// instant and every keyed attempt to its committed event (gate G4 linkage);
// a keyed birth may legitimately carry the typed lost-birth evidence.
func reproductionAssertJournalClock(t *testing.T, f *Reproduction) {
	t.Helper()
	events := make(map[sim.EventID]kernel.Event)
	for _, ev := range f.k.Events() {
		events[ev.ID] = ev
	}
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			hour := int(a.Time / sim.SimTime(ReproductionHour))
			if hour < 0 || hour >= ReproductionHorizonHours {
				t.Fatalf("attempt outside horizon: %+v", a)
			}
			var want sim.SimTime
			switch a.Kind {
			case "gather", "eat-stored", "wait":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhaseClaim)
			case "rest":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhaseGathered)
			case "eat":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhaseBagDecision)
			case "build":
				start, _ := ReproductionPhaseTime(hour, ReproductionPhaseBuildStart)
				want = ReproductionBuildCompletion(start)
			case "paired-meal", "fallback-eatstored":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhasePairedMeal)
			case "birth-request":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhaseBirthRequest)
			case "birth-reply":
				want, _ = ReproductionPhaseTime(hour, ReproductionPhaseBirthReply)
			default:
				t.Fatalf("unknown attempt kind %+v", a)
			}
			if a.Time != want {
				t.Fatalf("attempt off the frozen clock: %+v want %d", a, want)
			}
			if a.Key == "" {
				if a.EventID != 0 || a.Accepted || a.Lost != "" {
					t.Fatalf("uncommitted attempt carries evidence of a commit: %+v", a)
				}
				continue
			}
			if a.Lost != "" {
				if a.Lost != reproductionLostCollision || a.Accepted || a.EventID != 0 {
					t.Fatalf("malformed lost-birth evidence: %+v", a)
				}
				continue
			}
			ev, ok := events[a.EventID]
			if !ok || !a.Accepted || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor || ev.Rule == 0 {
				t.Fatalf("broken event link: %+v event %+v", a, ev)
			}
		}
	}
}

// TestReproductionRunnerV3NeutralIdentityThroughH95 pins the equality
// fixture of the frozen matrix: births-disabled v4 with all-neutral founder
// genomes is identical to a v3-constructed reference on every v3-visible
// trajectory (basal, wild flow, yield, wear, energy, granary, capital,
// patches, slots, alive) through h95 — the last hour before the first
// possible founder age-death. q8 exercises the surplus takeoff, q0 the
// extinction and starvation path.
func TestReproductionRunnerV3NeutralIdentityThroughH95(t *testing.T) {
	for _, q := range []int64{8, 0} {
		f := reproductionRunTo(t, ReproductionOptions{Yield: q, Seed: 0, Workers: 1}, 95)
		cap := capacityRunTo(t, CapacityOptions{Yield: q, Seed: 0, Workers: 1, Enabled: true}, 95)
		for _, ev := range f.k.Events() {
			switch ev.Rule {
			case ReproductionBirthRequestRule, ReproductionBirthReplyRule, ReproductionBirthRefuseRule, ReproductionBirthExpireRule:
				t.Fatalf("q%d births-disabled run emitted birth event %+v", q, ev)
			}
		}
		for h := 0; h <= 95; h++ {
			rc, cc := f.Checkpoints()[h], cap.Checkpoints()[h]
			for i := range rc.Founders {
				a, b := rc.Founders[i], cc.Actors[i]
				if a.Body.Energy != b.Body.Energy || a.Body.Hunger != b.Body.Hunger || a.Body.BasalSpent != b.Body.BasalSpent ||
					a.Body.CapLost != b.Body.CapLost || a.Body.Consumed != b.Body.Consumed || a.Body.LastGatherHour != b.Body.LastGatherHour ||
					a.Bag.Units != b.Bag.Units ||
					a.Worksite.Wip != b.Worksite.Wip || a.Worksite.Capital != b.Worksite.Capital || a.Worksite.WearDebt != b.Worksite.WearDebt ||
					a.Worksite.InvestedUnits != b.Worksite.InvestedUnits || a.Worksite.PointsCreated != b.Worksite.PointsCreated ||
					a.Worksite.PointsDecayed != b.Worksite.PointsDecayed || a.Worksite.LastBuildHour != b.Worksite.LastBuildHour ||
					a.Granary.Stock != b.Granary.Stock || a.Granary.YieldTotal != b.Granary.YieldTotal ||
					a.Granary.YieldUnrealized != b.Granary.YieldUnrealized || a.Granary.StoredMeals != b.Granary.StoredMeals ||
					a.Granary.LastStoredMealHour != b.Granary.LastStoredMealHour {
					t.Fatalf("q%d h%d actor %d divergence:\n v4 %+v\n v3 %+v", q, h, i+1, a, b)
				}
				// All-neutral founder genomes: the divisor loops are
				// byte-identical to v3 exactly at locus 6.
				if a.Genome.LocusM != ReproductionGenomeNeutral || a.Genome.LocusY != ReproductionGenomeNeutral || a.Genome.LocusW != ReproductionGenomeNeutral {
					t.Fatalf("q%d h%d actor %d not neutral: %+v", q, h, i+1, a.Genome)
				}
			}
			for pi := range rc.Patches {
				p4, p3 := rc.Patches[pi], cc.Patches[pi]
				if p4.Yield != p3.Yield || p4.Pulses != p3.Pulses || p4.Produced != p3.Produced || p4.Unrealized != p3.Unrealized {
					t.Fatalf("q%d h%d patch %d divergence", q, h, pi)
				}
				for si := range rc.Slots[pi] {
					if rc.Slots[pi][si].Stock != cc.Slots[pi][si].Stock || rc.Slots[pi][si].Gathered != cc.Slots[pi][si].Gathered {
						t.Fatalf("q%d h%d slot %d/%d divergence", q, h, pi, si)
					}
				}
			}
			if rc.Metrics.Alive != cc.Alive {
				t.Fatalf("q%d h%d alive divergence", q, h)
			}
			b4, c4 := rc.Balance, cc.Balance
			if b4.Produced != c4.Produced || b4.Gathered != c4.Gathered || b4.Consumed != c4.Consumed || b4.Stock != c4.Stock ||
				b4.Held != c4.Held || b4.Invested != c4.Invested || b4.YieldTotal != c4.YieldTotal || b4.GranaryStock != c4.GranaryStock ||
				b4.StoredMeals != c4.StoredMeals || b4.Capital != c4.Capital || b4.Wip != c4.Wip ||
				b4.PointsCreated != c4.PointsCreated || b4.PointsDecayed != c4.PointsDecayed ||
				b4.Energy != c4.Energy || b4.BasalSpent != c4.BasalSpent || b4.EnergyCapLost != c4.EnergyCapLost {
				t.Fatalf("q%d h%d balance divergence:\n v4 %+v\n v3 %+v", q, h, b4, c4)
			}
		}
		reproductionAssertConservation(t, f)
		reproductionAssertJournalClock(t, f)
	}
	// The v3 hand trajectory of actor 1 (bootstrap trough at E=4 under the
	// frozen TLow=5 floor) must hold verbatim in the v4 runner.
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1}, 12)
	want := []struct{ energy, capital, granary, yieldTotal int64 }{
		{11, 0, 0, 0}, {10, 0, 0, 0}, {9, 1, 1, 1}, {8, 1, 2, 2}, {7, 2, 4, 4},
		{6, 1, 6, 6}, {5, 2, 8, 8}, {4, 2, 8, 10}, {4, 2, 8, 13}, {4, 2, 8, 15},
		{4, 2, 8, 18}, {4, 2, 8, 20}, {4, 3, 8, 23},
	}
	for h, w := range want {
		a := f.Checkpoints()[h].Founders[0]
		if a.Body.Energy != w.energy || a.Worksite.Capital != w.capital || a.Granary.Stock != w.granary || a.Granary.YieldTotal != w.yieldTotal {
			t.Fatalf("h%d actor1 hand trajectory: got {e=%d k=%d g=%d yt=%d} want %+v", h, a.Body.Energy, a.Worksite.Capital, a.Granary.Stock, a.Granary.YieldTotal, w)
		}
	}
}

// TestReproductionRunnerBirthEndToEndWakeBirthAndNextPulse walks the first
// frozen q8 birth end-to-end: the typed request commits at h+40m+5µs, the
// atomic rule-410 entity-create commits at h+40m+6µs with the hand-computed
// 8-unit conversion (each parent pays 2 granary + 2 energy booked through
// basal-spent; the newborn starts at exactly 8 energy with zero capital),
// the newborn fiber registers at the quiescent boundary with its first wake
// reserved as WakeBirth at the next claim, and the newborn is inside the
// very next pulse's conservation. A colliding second birth at the same
// instant stages nothing structurally.
func TestReproductionRunnerBirthEndToEndWakeBirthAndNextPulse(t *testing.T) {
	f, err := NewReproduction(ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	hour := sim.SimTime(ReproductionHour)
	birthHour := -1
	var postBirthSnap scheduler.Snapshot
	for birthHour < 0 || len(f.Checkpoints()) <= birthHour+2 {
		if birthHour < 0 && len(f.Checkpoints()) > 30 {
			t.Fatal("no birth by h30 in the q8 births-enabled fixture")
		}
		if _, err := f.Step(context.Background()); err != nil {
			t.Fatalf("step: %v", err)
		}
		if birthHour < 0 {
			for _, ev := range f.k.Events() {
				if ev.Kind == kernel.KindEntityCreate {
					birthHour = int(ev.Time / hour)
					postBirthSnap = f.SchedulerSnapshot() // quiescent boundary right after the birth commit
					break
				}
			}
		}
	}
	replyAt, _ := ReproductionPhaseTime(birthHour, ReproductionPhaseBirthReply)
	var birth kernel.Event
	birthsAtReply := 0
	for _, ev := range f.k.Events() {
		if ev.Kind != kernel.KindEntityCreate {
			continue
		}
		if ev.Time > replyAt {
			break // only the first reply instant is under test
		}
		if ev.Time != replyAt || ev.Rule != ReproductionBirthReplyRule {
			t.Fatalf("birth off the frozen reply instant: %+v", ev)
		}
		birth, birthsAtReply = ev, birthsAtReply+1
	}
	if birthsAtReply != 1 {
		t.Fatalf("expected exactly one committed birth at the first reply instant, got %d", birthsAtReply)
	}
	consenter := birth.Cause.Actor
	proposer, newborn := sim.EntityID(0), sim.EntityID(0)
	for _, d := range birth.Deltas {
		if d.Component == ReproductionBirthRequestTypeID && d.Field == ReproductionBirthRequestStatusField && d.Entity < sim.EntityID(ReproductionFirstNewbornID) {
			proposer = d.Entity // only the proposer's request status is patched
		}
		if d.Component == ReproductionGenomeTypeID && d.Entity >= sim.EntityID(ReproductionFirstNewbornID) {
			newborn = d.Entity
		}
	}
	if proposer == 0 || consenter == 0 || proposer == consenter || newborn != ReproductionFirstNewbornID {
		t.Fatalf("birth parties misidentified: proposer=%d consenter=%d newborn=%d", proposer, consenter, newborn)
	}
	// Hand-computed parent ledger deltas: the eight converted units are two
	// granary + two energy per parent, the energy booked through the
	// parent's basal-spent ledger, and the gate margins held on fresh truth.
	type deltaKey struct {
		component sim.ComponentTypeID
		field     sim.FieldID
	}
	deltas := map[sim.EntityID]map[deltaKey][2]int64{}
	for _, d := range birth.Deltas {
		if d.Entity != proposer && d.Entity != consenter {
			continue
		}
		before, errBefore := d.Before.Integer()
		after, errAfter := d.After.Integer()
		if errBefore != nil || errAfter != nil {
			continue // ref fields (addressee) are not part of the cost math
		}
		if deltas[d.Entity] == nil {
			deltas[d.Entity] = map[deltaKey][2]int64{}
		}
		key := deltaKey{d.Component, d.Field}
		if _, dup := deltas[d.Entity][key]; dup {
			t.Fatalf("duplicate parent delta %+v", d)
		}
		deltas[d.Entity][key] = [2]int64{before, after}
	}
	for _, parent := range []sim.EntityID{proposer, consenter} {
		d := deltas[parent]
		bodyEnergy := d[deltaKey{ReproductionBodyTypeID, ReproductionBodyEnergyField}]
		bodySpent := d[deltaKey{ReproductionBodyTypeID, ReproductionBodyBasalSpentField}]
		granary := d[deltaKey{ReproductionGranaryTypeID, ReproductionGranaryStockField}]
		paid := d[deltaKey{ReproductionGenomeTypeID, ReproductionGenomeBirthGranaryPaidField}]
		if bodyEnergy[1]-bodyEnergy[0] != -ReproductionParentEnergyCost ||
			bodySpent[1]-bodySpent[0] != ReproductionParentEnergyCost ||
			granary[1]-granary[0] != -ReproductionParentGranaryCost ||
			paid[1]-paid[0] != ReproductionParentGranaryCost {
			t.Fatalf("parent %d cost deltas break the 8-unit conversion: %+v", parent, d)
		}
		if bodyEnergy[0] < ReproductionConsentMinEnergy || granary[0] < ReproductionConsentMinGranary {
			t.Fatalf("parent %d paid from below the frozen gate margins: %+v", parent, d)
		}
	}
	statusDelta, ok := deltas[proposer][deltaKey{ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField}]
	if !ok || statusDelta != [2]int64{int64(ReproductionRequestPending), int64(ReproductionRequestAccepted)} {
		t.Fatalf("proposer request status delta: %+v", statusDelta)
	}
	// The newborn's complete initial rows: exactly 8 energy converted from
	// the parents, hunger 0, every capital and granary ledger at zero, home
	// patch = the proposer's, and the heredity of exactly seven runner-owned
	// draws at stream position 7×(ordinal−1).
	before := f.Checkpoints()[birthHour]
	after := f.Checkpoints()[birthHour+1]
	var nb ReproductionActorState
	found := false
	for _, n := range after.Newborns {
		if n.ID == newborn {
			nb, found = n.State, true
		}
	}
	if !found {
		t.Fatal("newborn missing from the next pulse")
	}
	// The h+1 pulse already paid exactly one basal hour on the newborn
	// (energy 8 -> 7, hunger 0 -> 1); every capital ledger is still zero.
	if nb.Body.Energy != ReproductionNewbornEnergy-1 || nb.Body.Hunger != 1 || nb.Body.BasalDebt != 0 ||
		nb.Worksite != (ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour}) ||
		nb.Granary != (ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour}) ||
		nb.Request != ReproductionIdleRequestState() {
		t.Fatalf("newborn rows at h%d break the frozen conversion: %+v", birthHour+1, nb)
	}
	proposerBefore := before.Founders[proposer-1]
	consenterBefore := before.Founders[consenter-1]
	position, err := ReproductionBirthDrawPosition(after.Balance.Births - 1)
	if err != nil {
		t.Fatal(err)
	}
	stream := sim.NewRandomStream(sim.RandomState{Seed: 0, Stream: ReproductionBirthStream, Position: position})
	draws := make([]uint64, ReproductionDrawsPerBirth)
	for i := range draws {
		draws[i] = stream.Uint64()
	}
	wantGenome, _, err := ReproductionChildGenome(proposer, consenter, proposerBefore.Genome, consenterBefore.Genome, birthHour, draws)
	if err != nil {
		t.Fatal(err)
	}
	// The h+1 pulse already paid one basal hour on the newborn; the
	// immutable genome fields are compared against the pure seven draws.
	if nb.Genome.LocusM != wantGenome.LocusM || nb.Genome.LocusY != wantGenome.LocusY || nb.Genome.LocusW != wantGenome.LocusW ||
		nb.Genome.ParentA != wantGenome.ParentA || nb.Genome.ParentB != wantGenome.ParentB || nb.Genome.DeathHour != wantGenome.DeathHour {
		t.Fatalf("newborn genome diverges from the seven runner-owned draws:\n got %+v\n want %+v", nb.Genome, wantGenome)
	}
	home, err := ReproductionFounderPatchID(proposer)
	if err != nil || nb.Bag.Source != home {
		t.Fatalf("newborn home patch %d, want the proposer's %d", nb.Bag.Source, home)
	}
	// Registration happened at the quiescent boundary: the newborn fiber is
	// alive and its FIRST wake is the reserved WakeBirth at the next claim —
	// and only that wake.
	newbornWakes := 0
	firstClaim, _ := ReproductionPhaseTime(birthHour+1, ReproductionPhaseClaim)
	for _, w := range postBirthSnap.Wakes {
		if w.Actor != newborn {
			continue
		}
		newbornWakes++
		if w.Cause != scheduler.WakeBirth || w.At != firstClaim {
			t.Fatalf("newborn first wake is not the reserved WakeBirth claim: %+v", w)
		}
	}
	for _, fiber := range postBirthSnap.Fibers {
		if fiber.Actor == newborn && fiber.Lifecycle != scheduler.Alive {
			t.Fatal("newborn fiber not registered alive at the quiescent boundary")
		}
	}
	if newbornWakes != 1 {
		t.Fatalf("newborn has %d pending wakes, want exactly the reserved first claim", newbornWakes)
	}
	// The first claim is handled exactly as a claim, distinguished only in
	// evidence.
	firstClaimAttempts := 0
	for _, batch := range f.Journal() {
		if batch.Time != firstClaim {
			continue
		}
		for _, a := range batch.Attempts {
			if a.Actor == newborn {
				firstClaimAttempts++
				if !a.BirthWake {
					t.Fatalf("newborn first claim not distinguished as a WakeBirth: %+v", a)
				}
			}
			if a.BirthWake && a.Actor != newborn {
				t.Fatalf("WakeBirth evidence on a non-newborn: %+v", a)
			}
		}
	}
	if firstClaimAttempts == 0 {
		t.Fatal("newborn never claimed at its reserved first wake")
	}
	// The colliding second birth at the same instant staged nothing
	// structurally: typed lost evidence, no second newborn.
	lost := 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Lost == reproductionLostCollision {
				lost++
				if a.Birth.Decision != ReproductionReplyAccept || a.Accepted || a.EventID != 0 {
					t.Fatalf("malformed collision-loser evidence: %+v", a)
				}
			}
		}
	}
	if lost == 0 {
		t.Fatal("fixture produced no allocation-collision loser evidence")
	}
	if after.Balance.Births != 1 {
		t.Fatalf("h%d balance births %d, want 1 (the loser staged nothing)", birthHour+1, after.Balance.Births)
	}
	reproductionAssertConservation(t, f)
	reproductionAssertJournalClock(t, f)
}

// TestReproductionRunnerDeathAttributionAndDeadOwnerConventions pins
// cause-attributed mortality and the frozen dead-owner conventions:
// starvation (rule 404) and age death (rule 413) both set the died-hour at
// the pulse with energy retained, yield stops, the granary freezes, and
// dead owners' worksites keep wearing.
func TestReproductionRunnerDeathAttributionAndDeadOwnerConventions(t *testing.T) {
	// q0: everyone starves in the trough; all deaths are rule-404 starvation.
	q0 := reproductionRunTo(t, ReproductionOptions{Yield: 0, Seed: 0, Workers: 1}, 12)
	q0Checks := q0.Checkpoints()
	if q0Checks[12].Metrics.Alive != 0 {
		t.Fatalf("q0 alive at h12: %d", q0Checks[12].Metrics.Alive)
	}
	if q0Checks[12].Metrics.DeathsStarvation != ReproductionFounderCount || q0Checks[12].Metrics.DeathsAge != 0 {
		t.Fatalf("q0 attribution: %+v", q0Checks[12].Metrics)
	}
	for i, a := range q0Checks[12].Founders {
		if a.Body.Energy != 0 || a.Genome.DiedHour < 1 || a.Genome.DiedHour >= 12 {
			t.Fatalf("q0 actor %d starvation bookkeeping: died=%d energy=%d", i+1, a.Genome.DiedHour, a.Body.Energy)
		}
	}
	// q8: nobody starves; the immutable lifetimes age the roster out. By
	// h105 at least one founder has died of age; by h144 all have.
	q8 := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1}, 144)
	checks := q8.Checkpoints()
	if checks[105].Metrics.DeathsStarvation != 0 || checks[105].Metrics.DeathsAge == 0 {
		t.Fatalf("q8 h105 attribution: %+v", checks[105].Metrics)
	}
	if checks[144].Metrics.DeathsAge != ReproductionFounderCount || checks[144].Metrics.Alive != 0 {
		t.Fatalf("q8 h144: alive=%d age-deaths=%d", checks[144].Metrics.Alive, checks[144].Metrics.DeathsAge)
	}
	for i := range checks[144].Founders {
		a := checks[144].Founders[i]
		if a.Genome.DiedHour != a.Genome.DeathHour || a.Genome.DiedHour < 96 || a.Genome.DiedHour > 143 {
			t.Fatalf("actor %d age-death bookkeeping: died=%d death-hour=%d", i+1, a.Genome.DiedHour, a.Genome.DeathHour)
		}
		// Energy retained at the pulse of death (v1 stopped-state convention)
		// and frozen ever after; yield stops; the granary freezes; wear goes
		// on while capital stands.
		atDeath := checks[a.Genome.DiedHour].Founders[i]
		if atDeath.Body.Energy <= 0 {
			t.Fatalf("actor %d died with no retained energy", i+1)
		}
		last := checks[144].Founders[i]
		if last.Body.Energy != atDeath.Body.Energy || last.Body.BasalSpent != atDeath.Body.BasalSpent {
			t.Fatalf("actor %d kept metabolizing after death", i+1)
		}
		if last.Granary.YieldTotal != atDeath.Granary.YieldTotal || last.Granary.Stock != atDeath.Granary.Stock ||
			last.Granary.StoredMeals != atDeath.Granary.StoredMeals {
			t.Fatalf("actor %d granary moved after death", i+1)
		}
		if last.Worksite.PointsDecayed < atDeath.Worksite.PointsDecayed {
			t.Fatalf("actor %d wear stopped after death", i+1)
		}
	}
	reproductionAssertConservation(t, q0)
	reproductionAssertConservation(t, q8)
	reproductionAssertJournalClock(t, q8)
}

// reproductionFixtureActor builds one ledger-coherent founder state at the
// requested margins (the G2a/G2b/G3 per-actor identities hold by
// construction: basal-spent absorbs the missing initial energy).
func reproductionFixtureActor(energy, capital, granary int64) ReproductionActorState {
	home, _ := ReproductionPatchID(0)
	return ReproductionActorState{
		Body:     ReproductionBodyState{Energy: energy, BasalSpent: ReproductionFounderInitialEnergy - energy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: home},
		Worksite: ReproductionWorksiteState{Capital: capital, InvestedUnits: ReproductionPointCostWip * capital, PointsCreated: capital, LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{Stock: granary, YieldTotal: granary, LastStoredMealHour: ReproductionNeverHour},
		Genome:   ReproductionGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, BirthHour: 0, DeathHour: 120, DiedHour: ReproductionNeverHour},
		Request:  ReproductionIdleRequestState(),
	}
}

// reproductionFixtureNewborn builds a ledger-coherent newborn state born at
// hour 5 on the proposer patch (base energy 8).
func reproductionFixtureNewborn(id, parentA, parentB sim.EntityID, capital, granary int64) ReproductionActorState {
	home, _ := ReproductionPatchID(0)
	return ReproductionActorState{
		Body:     ReproductionBodyState{Energy: ReproductionNewbornEnergy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: home},
		Worksite: ReproductionWorksiteState{Capital: capital, InvestedUnits: ReproductionPointCostWip * capital, PointsCreated: capital, LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{Stock: granary, YieldTotal: granary, LastStoredMealHour: ReproductionNeverHour},
		Genome:   ReproductionGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, ParentA: parentA, ParentB: parentB, BirthHour: 5, DeathHour: 105, DiedHour: ReproductionNeverHour},
		Request:  ReproductionIdleRequestState(),
	}
}

func reproductionFixtureRequest(requester, addressee sim.EntityID, reportedCapital, reportedGranary int64, hour int) ReproductionBirthRequestState {
	return ReproductionBirthRequestState{
		Hour: int64(hour), Addressee: addressee,
		ReportedCapital: reportedCapital, ReportedGranary: reportedGranary,
		Status: ReproductionRequestPending, LastRequestHour: int64(hour), LastReplyHour: ReproductionNeverHour,
	}
}

// reproductionFixtureSeeds builds the genesis seeds of a hand-set fixture
// world: two patches with their slots plus the six rows of every state
// (founders default to bare idle states when absent).
func reproductionFixtureSeeds(t *testing.T, states map[sim.EntityID]ReproductionActorState) []component.ComponentSeed {
	t.Helper()
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	var seeds []component.ComponentSeed
	for i := 0; i < ReproductionPatchCount; i++ {
		patch, _ := ReproductionPatchID(i)
		seeds = append(seeds, reproductionSeed(patch, ReproductionPatchTypeID, reproductionField(ReproductionPatchYieldField, 8), reproductionField(ReproductionPatchPulsesField, 0), reproductionField(ReproductionPatchProducedField, 0), reproductionField(ReproductionPatchUnrealizedField, 0)))
		for j := 0; j < ReproductionSlotsPerPatch; j++ {
			slot, _ := ReproductionSlotID(i, j)
			seeds = append(seeds, reproductionSeed(slot, ReproductionSlotTypeID, reproductionField(ReproductionSlotStockField, 0), reproductionField(ReproductionSlotPatchField, 0), reproductionField(ReproductionSlotGatheredField, 0)))
			seeds[len(seeds)-1].Fields[1].Value = missing
		}
	}
	refSeed := func(id sim.EntityID, field sim.FieldID) component.FieldSeed {
		v, err := genomeRefValue(id)
		if err != nil {
			t.Fatal("fixture ref")
		}
		return component.FieldSeed{Field: field, Value: v}
	}
	actorSeeds := func(id sim.EntityID, a ReproductionActorState) {
		seeds = append(seeds,
			reproductionSeed(id, ReproductionBagTypeID, reproductionField(ReproductionBagUnitsField, a.Bag.Units), refSeed(a.Bag.Source, ReproductionBagSourceField)),
			reproductionSeed(id, ReproductionBodyTypeID,
				reproductionField(ReproductionBodyEnergyField, a.Body.Energy), reproductionField(ReproductionBodyHungerField, a.Body.Hunger),
				reproductionField(ReproductionBodyBasalSpentField, a.Body.BasalSpent), reproductionField(ReproductionBodyCapLostField, a.Body.CapLost),
				reproductionField(ReproductionBodyConsumedField, a.Body.Consumed), reproductionField(ReproductionBodyLastGatherHourField, a.Body.LastGatherHour),
				reproductionField(ReproductionBodyBasalDebtField, a.Body.BasalDebt)),
			reproductionSeed(id, ReproductionWorksiteTypeID,
				reproductionField(ReproductionWorksiteCapitalField, a.Worksite.Capital), reproductionField(ReproductionWorksiteWipField, a.Worksite.Wip),
				reproductionField(ReproductionWorksiteWearDebtField, a.Worksite.WearDebt), reproductionField(ReproductionWorksiteInvestedUnitsField, a.Worksite.InvestedUnits),
				reproductionField(ReproductionWorksitePointsCreatedField, a.Worksite.PointsCreated), reproductionField(ReproductionWorksitePointsDecayedField, a.Worksite.PointsDecayed),
				reproductionField(ReproductionWorksiteLastBuildHourField, a.Worksite.LastBuildHour)),
			reproductionSeed(id, ReproductionGranaryTypeID,
				reproductionField(ReproductionGranaryStockField, a.Granary.Stock), reproductionField(ReproductionGranaryYieldTotalField, a.Granary.YieldTotal),
				reproductionField(ReproductionGranaryYieldUnrealizedField, a.Granary.YieldUnrealized), reproductionField(ReproductionGranaryStoredMealsField, a.Granary.StoredMeals),
				reproductionField(ReproductionGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour), reproductionField(ReproductionGranaryYieldDebtField, a.Granary.YieldDebt)),
			reproductionSeed(id, ReproductionGenomeTypeID,
				reproductionField(ReproductionGenomeLocusMField, a.Genome.LocusM), reproductionField(ReproductionGenomeLocusYField, a.Genome.LocusY),
				reproductionField(ReproductionGenomeLocusWField, a.Genome.LocusW),
				refSeed(a.Genome.ParentA, ReproductionGenomeParentAField), refSeed(a.Genome.ParentB, ReproductionGenomeParentBField),
				reproductionField(ReproductionGenomeBirthHourField, a.Genome.BirthHour), reproductionField(ReproductionGenomeDeathHourField, a.Genome.DeathHour),
				reproductionField(ReproductionGenomeDiedHourField, a.Genome.DiedHour), reproductionField(ReproductionGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid)),
			reproductionSeed(id, ReproductionBirthRequestTypeID,
				reproductionField(ReproductionBirthRequestHourField, a.Request.Hour), refSeed(a.Request.Addressee, ReproductionBirthRequestAddresseeField),
				reproductionField(ReproductionBirthRequestReportedCapitalField, a.Request.ReportedCapital), reproductionField(ReproductionBirthRequestReportedGranaryField, a.Request.ReportedGranary),
				reproductionField(ReproductionBirthRequestStatusField, int64(a.Request.Status)), reproductionField(ReproductionBirthRequestLastRequestHourField, a.Request.LastRequestHour),
				reproductionField(ReproductionBirthRequestLastReplyHourField, a.Request.LastReplyHour)))
	}
	for i := 1; i <= ReproductionFounderCount; i++ {
		a, ok := states[sim.EntityID(i)]
		if !ok {
			a = reproductionFixtureActor(ReproductionFounderInitialEnergy, 0, 0)
		}
		actorSeeds(sim.EntityID(i), a)
	}
	for id := sim.EntityID(ReproductionFirstNewbornID); ; id++ {
		a, ok := states[id]
		if !ok {
			break
		}
		actorSeeds(id, a)
	}
	return seeds
}

// reproductionFixtureWorld builds a hand-set v4 kernel snapshot for surgical
// reply-phase fixtures.
func reproductionFixtureWorld(t *testing.T, states map[sim.EntityID]ReproductionActorState) (scheduler.SnapshotView, *strategy.ReproductionRegistry) {
	t.Helper()
	reg, err := ReproductionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	k, err := kernel.New(reg, 0, reproductionFixtureSeeds(t, states))
	if err != nil {
		t.Fatal(err)
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	policies := strategy.NewReproductionRegistry()
	if err := policies.Register(strategy.FrozenReproductionPolicy()); err != nil {
		t.Fatal(err)
	}
	return view, policies
}

// reproductionFixtureReply runs the trusted reply-phase decision for one
// consenter over a fixture snapshot.
func reproductionFixtureReply(t *testing.T, policies *strategy.ReproductionRegistry, view scheduler.SnapshotView, consenter sim.EntityID, hour int) (kernel.Proposal, ReproductionAttempt) {
	t.Helper()
	bound, err := policies.Bind(strategy.ReproductionBinding{Actor: consenter, Ref: strategy.FrozenReproductionPolicy().Ref})
	if err != nil {
		t.Fatal(err)
	}
	proposal, attempt, err := reproductionBirthReply(view, 0, strategy.FrozenReproductionPolicy().Ref, bound, hour, consenter)
	if err != nil {
		t.Fatalf("reply decision: %v", err)
	}
	return proposal, attempt
}

// TestReproductionRunnerKinProhibitionAndBirthCap pins the runner-gate
// refusals the policy cannot see: the derived kinship walk (shared ancestor
// at walk depth ≤2: parent-child 1, siblings 2) refuses with the typed
// rule-411 two-row refusal, and the 16-birth cap refuses even when consent,
// claims, and margins all pass.
func TestReproductionRunnerKinProhibitionAndBirthCap(t *testing.T) {
	t.Run("kin-parent-child-and-siblings", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			proposer  sim.EntityID
			consenter sim.EntityID
			wantDist  int
		}{{"parent-child", 1, ReproductionFirstNewbornID, 1}, {"siblings", ReproductionFirstNewbornID, ReproductionFirstNewbornID + 1, 2}} {
			states := map[sim.EntityID]ReproductionActorState{
				1: reproductionFixtureActor(11, 4, 4),
				2: reproductionFixtureActor(11, 4, 4),
			}
			// 10001 and 10002 are both children of founders 1 and 2.
			states[ReproductionFirstNewbornID] = reproductionFixtureNewborn(ReproductionFirstNewbornID, 1, 2, 4, 4)
			states[ReproductionFirstNewbornID+1] = reproductionFixtureNewborn(ReproductionFirstNewbornID+1, 1, 2, 4, 4)
			reproductionFixtureWithRequest(t, states, tc.proposer, reproductionFixtureRequest(tc.proposer, tc.consenter, 4, 4, 7))
			view, policies := reproductionFixtureWorld(t, states)
			distance, related := reproductionFixtureKin(t, view, tc.proposer, tc.consenter)
			if !related || distance != tc.wantDist {
				t.Fatalf("%s: kin walk gave (%d,%v), want (%d,true)", tc.name, distance, related, tc.wantDist)
			}
			proposal, attempt := reproductionFixtureReply(t, policies, view, tc.consenter, 7)
			if attempt.Birth.Decision != ReproductionReplyRefuse || attempt.Birth.Reason != ReproductionRefusalKin {
				t.Fatalf("%s: typed refusal missing: %+v", tc.name, attempt.Birth)
			}
			if proposal.Key == "" || proposal.Rule != ReproductionBirthRefuseRule || len(proposal.Patches) != 2 {
				t.Fatalf("%s: refusal must patch exactly the two request rows: %+v", tc.name, proposal)
			}
		}
	})
	t.Run("cap", func(t *testing.T) {
		states := map[sim.EntityID]ReproductionActorState{
			1: reproductionFixtureActor(11, 4, 4),
			2: reproductionFixtureActor(11, 4, 4),
		}
		reproductionFixtureWithRequest(t, states, 1, reproductionFixtureRequest(1, 2, 4, 4, 7))
		// Sixteen committed births by genome scan: the frozen cap.
		for i := 0; i < ReproductionMaxBirths; i++ {
			id := sim.EntityID(ReproductionFirstNewbornID + i)
			states[id] = reproductionFixtureNewborn(id, 1, 2, 0, 0)
		}
		view, policies := reproductionFixtureWorld(t, states)
		bc, err := reproductionBirthCount(view)
		if err != nil || bc != ReproductionMaxBirths {
			t.Fatalf("fixture births %d err %v", bc, err)
		}
		proposal, attempt := reproductionFixtureReply(t, policies, view, 2, 7)
		if attempt.Birth.Decision != ReproductionReplyRefuse || attempt.Birth.Reason != ReproductionRefusalBirthCap {
			t.Fatalf("cap refusal: %+v", attempt.Birth)
		}
		if proposal.Rule != ReproductionBirthRefuseRule || len(proposal.Patches) != 2 {
			t.Fatalf("cap refusal proposal: %+v", proposal)
		}
	})
}

// reproductionFixtureKin derives the kin walk between two fixture actors.
func reproductionFixtureKin(t *testing.T, view scheduler.SnapshotView, a, b sim.EntityID) (int, bool) {
	t.Helper()
	_, _, actors, err := reproductionWorldState(view)
	if err != nil {
		t.Fatal(err)
	}
	lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		state, ok := actors[id]
		if !ok {
			return 0, 0, false
		}
		return state.Genome.ParentA, state.Genome.ParentB, true
	})
	return ReproductionKinDistance(lookup, a, b)
}

// TestReproductionRunnerFreshSnapshotRaceAndDivergence pins the
// fresh-snapshot duties of the reply phase: a consenter whose own granary
// raced below the consent margins gets the typed rule-411 refusal with zero
// cost mutation; a proposer whose claims diverge from fresh truth is
// refused by the runner gate with the divergence recorded in privileged
// evidence only; a proposer who died between filing and the reply is the
// typed zero-proposal refusal.
func TestReproductionRunnerFreshSnapshotRaceAndDivergence(t *testing.T) {
	t.Run("consenter-raced-below-margins", func(t *testing.T) {
		states := map[sim.EntityID]ReproductionActorState{
			1: reproductionFixtureActor(11, 4, 4),
			2: reproductionFixtureActor(11, 4, 0), // granary raced to empty
		}
		reproductionFixtureWithRequest(t, states, 1, reproductionFixtureRequest(1, 2, 4, 4, 7))
		view, policies := reproductionFixtureWorld(t, states)
		proposal, attempt := reproductionFixtureReply(t, policies, view, 2, 7)
		if attempt.Birth.Decision != ReproductionReplyRefuse || attempt.Birth.Reason != ReproductionRefusalConsentOwn {
			t.Fatalf("raced consenter: %+v", attempt.Birth)
		}
		if proposal.Rule != ReproductionBirthRefuseRule || len(proposal.Patches) != 2 {
			t.Fatalf("refusal must be the two request rows only: %+v", proposal)
		}
	})
	t.Run("claims-diverge-from-truth", func(t *testing.T) {
		states := map[sim.EntityID]ReproductionActorState{
			1: reproductionFixtureActor(11, 0, 0), // truth: no capital, no granary
			2: reproductionFixtureActor(11, 4, 4),
		}
		reproductionFixtureWithRequest(t, states, 1, reproductionFixtureRequest(1, 2, 8, 8, 7)) // inflated claims
		view, policies := reproductionFixtureWorld(t, states)
		proposal, attempt := reproductionFixtureReply(t, policies, view, 2, 7)
		if attempt.Birth.Decision != ReproductionReplyRefuse || attempt.Birth.Reason != ReproductionRefusalGateThresholds {
			t.Fatalf("divergent claims must hit the fresh-truth gate: %+v", attempt.Birth)
		}
		if !attempt.Birth.Diverged || attempt.Birth.ReportedCapital != 8 || attempt.Birth.ReportedGranary != 8 ||
			attempt.Birth.TrueCapital != 0 || attempt.Birth.TrueGranary != 0 {
			t.Fatalf("divergence evidence: %+v", attempt.Birth)
		}
		if proposal.Rule != ReproductionBirthRefuseRule || len(proposal.Patches) != 2 {
			t.Fatalf("gate refusal proposal: %+v", proposal)
		}
		for _, patch := range proposal.Patches {
			if patch.Entity != 1 && patch.Entity != 2 {
				t.Fatalf("refusal leaked beyond the two request rows: %+v", patch)
			}
		}
	})
	t.Run("proposer-died-between-phases", func(t *testing.T) {
		dead := reproductionFixtureActor(5, 4, 4) // stopped at h5 with energy retained
		dead.Genome.DiedHour = 5
		states := map[sim.EntityID]ReproductionActorState{
			1: dead,
			2: reproductionFixtureActor(11, 4, 4),
		}
		reproductionFixtureWithRequest(t, states, 1, reproductionFixtureRequest(1, 2, 4, 4, 7))
		view, policies := reproductionFixtureWorld(t, states)
		proposal, attempt := reproductionFixtureReply(t, policies, view, 2, 7)
		if attempt.Birth.Decision != ReproductionReplyRefuseSilent || attempt.Birth.Reason != ReproductionRefusalProposerDead {
			t.Fatalf("dead proposer: %+v", attempt.Birth)
		}
		if proposal.Key != "" || len(proposal.Patches) != 0 {
			t.Fatalf("silent refusal must be zero mutation: %+v", proposal)
		}
	})
}

// TestReproductionRunnerDeterminismWorkersWithBirths runs the births-enabled
// q8 world under 1 and 4 workers and requires identical journals, hourly
// checkpoints, event bytes, and exported history — including the birth
// economics (committed births, collision losers, refusals).
func TestReproductionRunnerDeterminismWorkersWithBirths(t *testing.T) {
	one := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 12)
	four := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 4, Births: true}, 12)
	births, lostBirths := 0, 0
	for _, batch := range one.Journal() {
		for _, a := range batch.Attempts {
			if a.Lost == reproductionLostCollision {
				lostBirths++
			}
		}
	}
	for _, ev := range one.k.Events() {
		if ev.Kind == kernel.KindEntityCreate {
			births++
		}
	}
	if births == 0 || lostBirths == 0 {
		t.Fatalf("determinism fixture uncovered the birth paths: births=%d losers=%d", births, lostBirths)
	}
	if !reflect.DeepEqual(reproductionEventBytes(t, one), reproductionEventBytes(t, four)) ||
		!reflect.DeepEqual(one.Journal(), four.Journal()) ||
		!reflect.DeepEqual(one.Checkpoints(), four.Checkpoints()) {
		t.Fatal("worker-dependent evidence with births enabled")
	}
	historyOne, headOne, err := one.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	historyFour, headFour, err := four.k.ExportHistory()
	if err != nil || headOne != headFour || !bytes.Equal(historyOne, historyFour) {
		t.Fatalf("history divergence: %v", err)
	}
	reproductionAssertConservation(t, one)
	reproductionAssertJournalClock(t, one)
}

// TestReproductionRunnerAllocationCollisionZeroMutation pins the kernel-side
// consequence of the enumerated newborn ID: two accepted replies at one
// instant enumerate the same newborn, the kernel keeps exactly one whole
// proposal, and the losing birth stages nothing structurally — its parent
// cost patches and allocations are all discarded. The runner-side evidence
// duty (typed lost-birth note, no phantom commit) is pinned separately.
func TestReproductionRunnerAllocationCollisionZeroMutation(t *testing.T) {
	states := map[sim.EntityID]ReproductionActorState{
		1: reproductionFixtureActor(11, 4, 4),
		2: reproductionFixtureActor(11, 4, 4),
		3: reproductionFixtureActor(11, 4, 4),
		4: reproductionFixtureActor(11, 4, 4),
	}
	reproductionFixtureWithRequest(t, states, 1, reproductionFixtureRequest(1, 2, 4, 4, 7))
	reproductionFixtureWithRequest(t, states, 3, reproductionFixtureRequest(3, 4, 4, 4, 7))
	reg, err := ReproductionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	seeds := reproductionFixtureSeeds(t, states)
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	draws := make([]uint64, ReproductionDrawsPerBirth)
	for i := range draws {
		draws[i] = uint64(i)
	}
	replyAt := mustReplyAt(t, 7)
	var plans []kernel.Plan
	_, authority, _ := k.Snapshot()
	for _, pair := range [][2]sim.EntityID{{1, 2}, {3, 4}} {
		result, err := ReproductionBirthReply(7, 0, true, draws, pair[0], pair[1], states[pair[0]], states[pair[1]])
		if err != nil || result.Decision != ReproductionReplyAccept {
			t.Fatalf("fixture reply %v: %v", pair, err)
		}
		proposal, err := ReproductionBirthReplyProposal(reproductionKey("birth-reply", replyAt, pair[1]), 7, pair[0], pair[1], result)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := k.Plan(proposal, authority)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	events, err := k.CommitBatch(plans)
	if err != nil {
		t.Fatal(err)
	}
	creates := 0
	for _, ev := range events {
		if ev.Kind == kernel.KindEntityCreate {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("committed %d entity-creates, want exactly one winner", creates)
	}
	head := k.SnapshotHead()
	viewAfter := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	bc, err := reproductionBirthCount(viewAfter)
	if err != nil || bc != 1 {
		t.Fatalf("post-commit births %d err %v", bc, err)
	}
	_, _, actors, err := reproductionWorldState(viewAfter)
	if err != nil {
		t.Fatal(err)
	}
	var losers []sim.EntityID
	if actors[1].Genome.BirthGranaryPaid == ReproductionParentGranaryCost {
		losers = []sim.EntityID{3, 4}
	} else {
		losers = []sim.EntityID{1, 2}
	}
	for _, id := range losers {
		a := actors[id]
		want := states[id]
		if a.Body.Energy != want.Body.Energy || a.Body.BasalSpent != want.Body.BasalSpent ||
			a.Granary.Stock != want.Granary.Stock || a.Genome.BirthGranaryPaid != want.Genome.BirthGranaryPaid {
			t.Fatalf("losing parent %d was mutated: %+v", id, a)
		}
		if a.Request.LastReplyHour != ReproductionNeverHour {
			t.Fatalf("losing consenter %d replied: %+v", id, a.Request)
		}
	}
	loserProposer := losers[0]
	if actors[loserProposer].Request.Status != ReproductionRequestPending {
		t.Fatalf("losing proposer %d request consumed: %+v", loserProposer, actors[loserProposer].Request)
	}
	// Runner evidence: the unlinked keyed birth attempt becomes the typed
	// lost-birth note; any other unlinked keyed attempt is a linkage break.
	won := ReproductionAttempt{Actor: 2, Kind: "birth-reply", Key: "k1", Birth: ReproductionBirthEvidence{Decision: ReproductionReplyAccept, Newborn: ReproductionFirstNewbornID}}
	lostBirth := ReproductionAttempt{Actor: 4, Kind: "birth-reply", Key: "k2", Birth: ReproductionBirthEvidence{Decision: ReproductionReplyAccept, Newborn: ReproductionFirstNewbornID}}
	gather := ReproductionAttempt{Actor: 5, Kind: "gather", Key: "k3"}
	if err := reproductionLinkAttempt(&won, map[string]sim.EventID{"k1": 7}); err != nil || !won.Accepted || won.EventID != 7 {
		t.Fatalf("winner link: %+v %v", won, err)
	}
	if err := reproductionLinkAttempt(&lostBirth, map[string]sim.EventID{"k1": 7}); err != nil || lostBirth.Lost != reproductionLostCollision || lostBirth.Accepted {
		t.Fatalf("loser link: %+v %v", lostBirth, err)
	}
	if err := reproductionLinkAttempt(&gather, map[string]sim.EventID{"k1": 7}); !errors.Is(err, ErrReproductionRunner) {
		t.Fatalf("unlinked gather must be a linkage break: %v", err)
	}
}

func mustReplyAt(t *testing.T, hour int) sim.SimTime {
	t.Helper()
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthReply)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// TestReproductionRunnerBoundsEveryHour walks gate G7 on a births-enabled
// run: every field of every hourly projection sits inside its declared
// descriptor bounds, genomes are immutable post-creation, the died-hour only
// ever moves -1→hour (never into the future), and the newborn IDs stay a
// strict prefix in birth order.
func TestReproductionRunnerBoundsEveryHour(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 30)
	reg, err := ReproductionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	bounds := map[sim.ComponentTypeID][]component.FieldDescriptor{}
	for _, typ := range reg.TypeIDs() {
		descriptor, err := reg.Describe(typ)
		if err != nil {
			t.Fatal(err)
		}
		bounds[typ] = descriptor.Fields
	}
	within := func(typ sim.ComponentTypeID, field sim.FieldID, value int64) bool {
		for _, fd := range bounds[typ] {
			if fd.ID != field || !fd.Bounds.HasMinimum || !fd.Bounds.HasMaximum {
				continue
			}
			return value >= int64(fd.Bounds.Minimum) && value <= int64(fd.Bounds.Maximum)
		}
		return false
	}
	type genomePin struct {
		loci      [3]int64
		parents   [2]sim.EntityID
		birthHour int64
		deathHr   int64
	}
	pins := map[sim.EntityID]genomePin{}
	diedAt := map[sim.EntityID]int64{}
	for _, check := range f.Checkpoints() {
		assert := func(actor sim.EntityID, a ReproductionActorState) {
			fields := []struct {
				typ   sim.ComponentTypeID
				field sim.FieldID
				value int64
			}{
				{ReproductionBodyTypeID, ReproductionBodyEnergyField, a.Body.Energy},
				{ReproductionBodyTypeID, ReproductionBodyHungerField, a.Body.Hunger},
				{ReproductionBodyTypeID, ReproductionBodyBasalSpentField, a.Body.BasalSpent},
				{ReproductionBodyTypeID, ReproductionBodyCapLostField, a.Body.CapLost},
				{ReproductionBodyTypeID, ReproductionBodyConsumedField, a.Body.Consumed},
				{ReproductionBodyTypeID, ReproductionBodyLastGatherHourField, a.Body.LastGatherHour},
				{ReproductionBodyTypeID, ReproductionBodyBasalDebtField, a.Body.BasalDebt},
				{ReproductionWorksiteTypeID, ReproductionWorksiteCapitalField, a.Worksite.Capital},
				{ReproductionWorksiteTypeID, ReproductionWorksiteWipField, a.Worksite.Wip},
				{ReproductionWorksiteTypeID, ReproductionWorksiteWearDebtField, a.Worksite.WearDebt},
				{ReproductionWorksiteTypeID, ReproductionWorksiteInvestedUnitsField, a.Worksite.InvestedUnits},
				{ReproductionWorksiteTypeID, ReproductionWorksitePointsCreatedField, a.Worksite.PointsCreated},
				{ReproductionWorksiteTypeID, ReproductionWorksitePointsDecayedField, a.Worksite.PointsDecayed},
				{ReproductionWorksiteTypeID, ReproductionWorksiteLastBuildHourField, a.Worksite.LastBuildHour},
				{ReproductionGranaryTypeID, ReproductionGranaryStockField, a.Granary.Stock},
				{ReproductionGranaryTypeID, ReproductionGranaryYieldTotalField, a.Granary.YieldTotal},
				{ReproductionGranaryTypeID, ReproductionGranaryYieldUnrealizedField, a.Granary.YieldUnrealized},
				{ReproductionGranaryTypeID, ReproductionGranaryStoredMealsField, a.Granary.StoredMeals},
				{ReproductionGranaryTypeID, ReproductionGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour},
				{ReproductionGranaryTypeID, ReproductionGranaryYieldDebtField, a.Granary.YieldDebt},
				{ReproductionGenomeTypeID, ReproductionGenomeLocusMField, a.Genome.LocusM},
				{ReproductionGenomeTypeID, ReproductionGenomeLocusYField, a.Genome.LocusY},
				{ReproductionGenomeTypeID, ReproductionGenomeLocusWField, a.Genome.LocusW},
				{ReproductionGenomeTypeID, ReproductionGenomeBirthHourField, a.Genome.BirthHour},
				{ReproductionGenomeTypeID, ReproductionGenomeDeathHourField, a.Genome.DeathHour},
				{ReproductionGenomeTypeID, ReproductionGenomeDiedHourField, a.Genome.DiedHour},
				{ReproductionGenomeTypeID, ReproductionGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid},
				{ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, int64(a.Request.Status)},
			}
			for _, spec := range fields {
				if !within(spec.typ, spec.field, spec.value) {
					t.Fatalf("h%d actor %d component %d field %d out of bounds: %d", check.Hour, actor, spec.typ, spec.field, spec.value)
				}
			}
			if !validReproductionActor(actor, a) {
				t.Fatalf("h%d actor %d failed semantic validation", check.Hour, actor)
			}
			pin := genomePin{loci: [3]int64{a.Genome.LocusM, a.Genome.LocusY, a.Genome.LocusW}, parents: [2]sim.EntityID{a.Genome.ParentA, a.Genome.ParentB}, birthHour: a.Genome.BirthHour, deathHr: a.Genome.DeathHour}
			if before, seen := pins[actor]; seen {
				if pin != before {
					t.Fatalf("h%d actor %d genome mutated: %+v -> %+v", check.Hour, actor, before, pin)
				}
			} else {
				pins[actor] = pin
			}
			if a.Genome.DiedHour != ReproductionNeverHour {
				if prev, ok := diedAt[actor]; ok && prev != a.Genome.DiedHour {
					t.Fatalf("h%d actor %d died-hour moved %d -> %d", check.Hour, actor, prev, a.Genome.DiedHour)
				}
				diedAt[actor] = a.Genome.DiedHour
				if a.Genome.DiedHour > int64(check.Hour) {
					t.Fatalf("h%d actor %d died in the future", check.Hour, actor)
				}
			}
		}
		for i := range check.Founders {
			assert(sim.EntityID(i+1), check.Founders[i])
		}
		for i, newborn := range check.Newborns {
			if newborn.ID != sim.EntityID(ReproductionFirstNewbornID+i) {
				t.Fatalf("h%d newborn %d breaks the frozen ID order", check.Hour, newborn.ID)
			}
			assert(newborn.ID, newborn.State)
		}
	}
	if len(f.Checkpoints()[30].Newborns) == 0 {
		t.Fatal("fixture produced no births; genome immutability uncovered")
	}
	reproductionAssertConservation(t, f)
	reproductionAssertJournalClock(t, f)
}

// TestReproductionRunnerHandoffSeam exercises the value-only quiescent
// export for the persistence lane: portable kernel and scheduler bytes share
// the verified head, the dynamic roster and in-flight state are carried, and
// every exported collection is a defensive copy.
func TestReproductionRunnerHandoffSeam(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 3, Seed: 0, Workers: 1, Births: true}, 9)
	handoff, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if handoff.FormatVersion != ReproductionFormatVersion || handoff.Policy != f.ref || handoff.Steps != f.steps || !handoff.Births {
		t.Fatalf("handoff header: %+v", handoff)
	}
	if handoff.Head.TipID == 0 || len(handoff.History) == 0 || len(handoff.SchedulerBytes) == 0 {
		t.Fatal("handoff missing portable state")
	}
	if len(handoff.Journal) != len(f.Journal()) || len(handoff.Checkpoints) != len(f.Checkpoints()) {
		t.Fatal("journal or checkpoints not exported")
	}
	snapshot := f.SchedulerSnapshot()
	if snapshot.Kernel.Version != sim.WorldVersion(handoff.Head.Version) {
		t.Fatalf("scheduler and kernel heads disagree: %d vs %d", snapshot.Kernel.Version, handoff.Head.Version)
	}
	if len(handoff.Newborns) != len(f.newborns) {
		t.Fatalf("roster newborns not exported: %d vs %d", len(handoff.Newborns), len(f.newborns))
	}
	if len(handoff.BirthWakes) != len(f.birthWakes) {
		t.Fatalf("reserved first wakes not exported: %d vs %d", len(handoff.BirthWakes), len(f.birthWakes))
	}
	for id, kind := range f.current {
		if handoff.Current[id] != kind {
			t.Fatalf("in-flight activity %d not exported: %v", id, kind)
		}
	}
	// Defensive copies: mutating one export must not touch the runner or a
	// later export; the reproducibility comparison uses pristine exports.
	export := f.Journal()
	if len(export) > 0 && len(export[0].Attempts) > 0 {
		export[0].Attempts[0].Actor = 99
		if f.Journal()[0].Attempts[0].Actor == 99 {
			t.Fatal("journal export aliases runner state")
		}
	}
	exportChecks := f.Checkpoints()
	for k := range exportChecks[0].Metrics.Refusals {
		exportChecks[0].Metrics.Refusals[k] = 42
	}
	for _, v := range f.Checkpoints()[0].Metrics.Refusals {
		if v == 42 {
			t.Fatal("checkpoint export aliases runner state")
		}
	}
	again, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(handoff, again) {
		t.Fatal("quiescent handoff is not reproducible")
	}
}

// reproductionFixtureWithRequest installs a pending request row on a fixture
// state (map values are not addressable in Go).
func reproductionFixtureWithRequest(t *testing.T, states map[sim.EntityID]ReproductionActorState, id sim.EntityID, r ReproductionBirthRequestState) {
	t.Helper()
	a := states[id]
	a.Request = r
	states[id] = a
}
