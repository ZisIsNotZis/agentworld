package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestNaturalV13Mechanism proves mechanism v13 is admitted as world version
// 13, carries the v12 rule table verbatim, and declares the one parameter
// change: the price one paying parent hands over for a birth, in granary
// units and in energy units.
func TestNaturalV13Mechanism(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV13) || NaturalMechanismV13.worldVersion() != 13 {
		t.Fatal("mechanism v13 is not admitted as world version 13")
	}
	if NaturalMechanismV13.FormatVersion() != 13 || NaturalMechanismV13.RuleVersion() != 13 ||
		NaturalMechanismV13.ProjectionVersion() != 13 {
		t.Fatal("mechanism v13 version fields")
	}
	if _, err := NaturalRegistryFor(NaturalMechanismV13); err != nil {
		t.Fatalf("v13 registry: %v", err)
	}
	// v13 keeps the v12 rule table: the two-energy meal, the yield axis, the
	// yield-scaled wild fill and the round-robin founding band.
	if naturalMealEnergyFor(NaturalMechanismV13) != naturalMealEnergyV12 ||
		!naturalHasYieldAxis(NaturalMechanismV13) || !naturalWildFillYield(NaturalMechanismV13) ||
		!naturalRoundRobinFounders(NaturalMechanismV13) || naturalSlotStockMaxFor(NaturalMechanismV13) != NaturalYieldMax {
		t.Fatal("v13 does not carry the v12 rule table")
	}
	// The declared price reader: the zero spelling is the frozen default of 2
	// under v13, any admitted value is taken verbatim, and every mechanism
	// below v13 keeps the frozen 2 whatever is passed.
	if naturalBirthCostFor(NaturalMechanismV13, 0) != NaturalParentGranaryCost || NaturalParentGranaryCost != 2 {
		t.Fatal("v13 default birth price is not the frozen two units")
	}
	for _, cost := range []int64{1, 2, 4, 6, 8} {
		if got := naturalBirthCostFor(NaturalMechanismV13, cost); got != cost {
			t.Fatalf("v13 declared birth price %d read as %d", cost, got)
		}
	}
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7,
		NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10, NaturalMechanismV11, NaturalMechanismV12} {
		if naturalBirthCostFor(m, 8) != 2 {
			t.Fatalf("mechanism %d changed the frozen birth price", m)
		}
		if naturalHasBirthCostAxis(m) {
			t.Fatalf("mechanism %d declares the birth-cost axis", m)
		}
		if naturalBirthGranaryPaidMaxFor(m) != NaturalBirthGranaryPaidMax {
			t.Fatalf("mechanism %d widened the birth-granary-paid envelope", m)
		}
	}
	// The paying parent's wealth floor carries the declared price plus the
	// frozen one-unit surplus, and stays exactly the frozen gate below v13.
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7,
		NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10, NaturalMechanismV11, NaturalMechanismV12} {
		if granary, energy := naturalBirthPaymentFloorsFor(m, 8); granary != NaturalConsentMinGranary || energy != NaturalConsentMinEnergy {
			t.Fatalf("mechanism %d widened the paying-parent floors to %d/%d", m, granary, energy)
		}
	}
	if granary, energy := naturalBirthPaymentFloorsFor(NaturalMechanismV13, 8); granary != 9 || energy != 9 {
		t.Fatalf("v13 paying-parent floors %d/%d", granary, energy)
	}
	// The v13 envelope is derived from the declared maximum price: two paying
	// parents, each at NaturalBirthCostMax, for the whole horizon.
	if naturalBirthGranaryPaidMaxFor(NaturalMechanismV13) != 2*NaturalHorizonHours*NaturalBirthCostMax {
		t.Fatalf("v13 birth-granary-paid envelope %d", naturalBirthGranaryPaidMaxFor(NaturalMechanismV13))
	}
	// The genome descriptor carries the same declared envelope (one home).
	genome := NaturalGenomeDescriptorFor(NaturalMechanismV13)
	for _, f := range genome.Fields {
		if f.Name == "birth-granary-paid" && f.Bounds.Maximum != float64(2*NaturalHorizonHours*NaturalBirthCostMax) {
			t.Fatalf("v13 birth-granary-paid field bound %v", f.Bounds.Maximum)
		}
	}
}

// v13ParentEnergy is the fixture energy/granary level of a v13 parent: the
// declared price plus the frozen one-unit surplus the wealth gate demands of
// a paying parent (the frozen 3 for the frozen 2).
func v13ParentEnergy(price int64) int64 {
	surplus := price + 1
	if surplus < NaturalConsentMinGranary {
		return NaturalConsentMinGranary
	}
	return surplus
}

// naturalBirthPriceParent builds a valid paying parent under one mechanism:
// energy and granary stock at the declared price (or above the proposer's
// frozen wealth gate), the G1/G3 ledgers closing around it (base 11 = energy
// + basal-spent for a founder who never ate). The founding band is seated by
// the mechanism's own placement rule, so the home patch is the declared one.
func naturalBirthPriceParent(t *testing.T, m NaturalMechanism, patches int, actor sim.EntityID, price int64) NaturalActorState {
	t.Helper()
	level := v13ParentEnergy(price)
	a, err := NaturalFounderStateFor(m, patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = NaturalWorksiteState{Capital: 3, InvestedUnits: 6, PointsCreated: 3, LastBuildHour: NaturalNeverHour}
	a.Granary = NaturalGranaryState{Stock: level, CultivatedTotal: level, LastStoredMealHour: NaturalNeverHour}
	a.Body = NaturalBodyState{Energy: level, BasalSpent: 11 - level, LastGatherHour: NaturalNeverHour}
	if !validNaturalActorFor(m, patches, actor, a) {
		t.Fatalf("mechanism %d parent fixture invalid at price %d: %+v", m, price, a)
	}
	return a
}

// naturalV13Parent is naturalBirthPriceParent under mechanism v13.
func naturalV13Parent(t *testing.T, patches int, actor sim.EntityID, price int64) NaturalActorState {
	t.Helper()
	return naturalBirthPriceParent(t, NaturalMechanismV13, patches, actor, price)
}

// TestNaturalV13BirthPrice proves the payment site: one birth charges the
// declared price through both parent ledgers — the granary stock and the
// genome birth-granary-paid accumulator on one side, the body energy and its
// basal-spent ledger on the other — so the G1 and G3 identities keep
// balancing at every price. Under the household gate the proposer is the one
// paying parent; the frozen mechanisms keep the frozen two-unit price.
func TestNaturalV13BirthPrice(t *testing.T) {
	patches := 2
	draws := naturalDrawsForSeed(t, 0)
	for _, price := range []int64{2, 4, 6, 8} {
		proposer := naturalV13Parent(t, patches, 1, price)
		consenter := naturalV13Parent(t, patches, 2, 2)
		request, err := NaturalBirthRequestFor(NaturalMechanismV13, patches, 5, 1, 2, 3, 5, proposer)
		if err != nil {
			t.Fatalf("price %d request: %v", price, err)
		}
		result, err := NaturalBirthReplyForCost(NaturalMechanismV13, price, patches, 5, true, draws, 2017, 1, 2, request, consenter)
		if err != nil {
			t.Fatalf("price %d reply: %v", price, err)
		}
		if result.Decision != NaturalReplyAccept {
			t.Fatalf("price %d refused: %+v", price, result)
		}
		level := v13ParentEnergy(price)
		if result.Proposer.Granary.Stock != level-price || result.Proposer.Genome.BirthGranaryPaid != price ||
			result.Proposer.Body.Energy != level-price || result.Proposer.Body.BasalSpent != 11-level+price {
			t.Fatalf("price %d proposer payment: %+v", price, result.Proposer)
		}
		// The household gate keeps the consenter a non-payer.
		consenterLevel := v13ParentEnergy(2)
		if result.Consenter.Granary.Stock != consenterLevel || result.Consenter.Genome.BirthGranaryPaid != 0 ||
			result.Consenter.Body.Energy != consenterLevel {
			t.Fatalf("price %d consenter paid: %+v", price, result.Consenter)
		}
		if !validNaturalActorFor(NaturalMechanismV13, patches, 1, result.Proposer) ||
			!validNaturalActorFor(NaturalMechanismV13, patches, 2017, result.Newborn) {
			t.Fatalf("price %d broke the G1/G3 identities", price)
		}
		if result.Proposer.Genome.BirthGranaryPaid != result.Proposer.Granary.CultivatedTotal-result.Proposer.Granary.Stock {
			t.Fatalf("price %d did not book the granary side of the identity", price)
		}
	}
	// A proposer that cannot afford the declared price is refused before any
	// mutation, exactly like the frozen unaffordable-parent case.
	poor := naturalV13Parent(t, patches, 1, 2)
	consenter := naturalV13Parent(t, patches, 2, 2)
	childGenome, _, err := NaturalChildGenome(patches, 1, 2, poor.Genome, consenter.Genome, 5, draws)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NaturalBirthForCost(NaturalMechanismV13, 8, patches, 5, true, 2017, 1, 2, poor,
		consenter, childGenome); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("unaffordable declared price paid: %v", err)
	}
	// The frozen two-unit price is affordable from the same rows.
	if _, err := NaturalBirthForCost(NaturalMechanismV13, 0, patches, 5, true, 2017, 1, 2, poor,
		consenter, childGenome); err != nil {
		t.Fatalf("default price refused from affordable rows: %v", err)
	}
	// The frozen mechanisms keep the frozen price through the same wrapper.
	frozenProposer := naturalBirthPriceParent(t, NaturalMechanismV12, patches, 1, 2)
	frozenConsenter := naturalBirthPriceParent(t, NaturalMechanismV12, patches, 2, 2)
	request12, err := NaturalBirthRequestFor(NaturalMechanismV12, patches, 5, 1, 2, 3, 5, frozenProposer)
	if err != nil {
		t.Fatal(err)
	}
	result12, err := NaturalBirthReplyFor(NaturalMechanismV12, patches, 5, true, draws, 2017, 1, 2, request12, frozenConsenter)
	if err != nil || result12.Decision != NaturalReplyAccept {
		t.Fatalf("v12 reply: %+v %v", result12, err)
	}
	if result12.Proposer.Granary.Stock != 1 || result12.Proposer.Genome.BirthGranaryPaid != 2 ||
		result12.Proposer.Body.Energy != 1 || result12.Proposer.Body.BasalSpent != 10 {
		t.Fatalf("v12 paid more than the frozen two units: %+v", result12.Proposer)
	}
}

// TestNaturalV13WorldBalances proves a v13 world runs with a declared price:
// every pulse's conservation gate (G1/G2a/G2b/G3) balances, so the declared
// birth price is booked exactly once in each ledger.
func TestNaturalV13WorldBalances(t *testing.T) {
	ctx := context.Background()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, BirthCost: 4, Seed: 0, Workers: 1, Founders: 10,
		PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: NaturalMechanismV13})
	if err != nil {
		t.Fatalf("v13 world: %v", err)
	}
	if f.yield != 2 || f.birthCost != 4 {
		t.Fatalf("v13 world identity %d/%d", f.yield, f.birthCost)
	}
	for i := 0; i < 40; i++ {
		if ok, err := f.Step(ctx); err != nil || !ok {
			t.Fatalf("v13 step %d: %t %v", i, ok, err)
		}
	}
	last := f.Checkpoints()[len(f.Checkpoints())-1]
	if last.Balance.Consumed == 0 {
		t.Fatal("v13 world consumed nothing over forty hours")
	}
	if last.Balance.InitialEnergy+NaturalNewbornEnergy*last.Balance.Births+naturalMealEnergyV12*last.Balance.Consumed+last.Balance.StoredMeals !=
		last.Balance.Energy+last.Balance.BasalSpent+last.Balance.EnergyCapLost {
		t.Fatalf("v13 balance does not close: %+v", last.Balance)
	}
	// The declared price is a refused parameter outside v13.
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, BirthCost: 4, Seed: 0, Workers: 1,
		Founders: 10, PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: NaturalMechanismV12}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("v12 accepted a declared birth cost: %v", err)
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, BirthCost: NaturalBirthCostMax + 1, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: NaturalMechanismV13}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("v13 accepted an out-of-range birth cost: %v", err)
	}
}

// TestNaturalV13CheckpointIdentity proves the declared birth price is run
// identity end to end: the bundle round-trips under natural/14, the journal
// config carries the price, and a mismatched price or mechanism fails closed
// before any restore work.
func TestNaturalV13CheckpointIdentity(t *testing.T) {
	ctx := context.Background()
	ref14 := strategy.FrozenNaturalPolicy14().Ref
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, BirthCost: 4, Seed: 0, Workers: 1,
		Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV13})
	if err != nil {
		t.Fatalf("v13/natural/14 world: %v", err)
	}
	if config := f.journalConfig(); config.BirthCost != 4 || config.Yield != 2 || config.Mechanism != NaturalMechanismV13 {
		t.Fatalf("v13 journal identity %+v", config)
	}
	if ok, err := f.Step(ctx); err != nil || !ok {
		t.Fatalf("v13 h0 pulse: %t %v", ok, err)
	}
	path := t.TempDir() + "/v13.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatalf("v13 save: %v", err)
	}
	restore := NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, BirthCost: 4, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV13}
	restored, _, err := RestoreNaturalCheckpoint(path, restore)
	if err != nil {
		t.Fatalf("v13 restore: %v", err)
	}
	if restored.mechanism != NaturalMechanismV13 || restored.yield != 2 || restored.birthCost != 4 || restored.ref != ref14 {
		t.Fatalf("restored v13 identity %d/%d/%d/%+v", restored.mechanism, restored.yield, restored.birthCost, restored.ref)
	}
	mismatched := restore
	mismatched.BirthCost = 6
	if _, _, err := RestoreNaturalCheckpoint(path, mismatched); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v13 birth-cost-6 restore accepted a birth-cost-4 bundle: %v", err)
	}
	defaulted := restore
	defaulted.BirthCost = 0
	if _, _, err := RestoreNaturalCheckpoint(path, defaulted); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v13 default-price restore accepted a birth-cost-4 bundle: %v", err)
	}
	asV12 := restore
	asV12.Mechanism = NaturalMechanismV12
	if _, _, err := RestoreNaturalCheckpoint(path, asV12); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v12 restore accepted a v13 bundle: %v", err)
	}
}
