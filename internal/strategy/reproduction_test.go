package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func reproductionFixture(t *testing.T, actor sim.EntityID) (*strategy.ReproductionBound, strategy.ReproductionRef) {
	t.Helper()
	p := strategy.FrozenReproductionPolicy()
	r := strategy.NewReproductionRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.ReproductionBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p.Ref
}

func reproductionHomeSlots(home sim.EntityID) []strategy.ReproductionSlotView {
	slots := make([]strategy.ReproductionSlotView, 0, world.ReproductionSlotsPerPatch)
	for i := 0; i < world.ReproductionSlotsPerPatch; i++ {
		slot, err := world.ReproductionSlotID(int(home-1001), i)
		if err != nil {
			panic(err)
		}
		slots = append(slots, strategy.ReproductionSlotView{ID: slot, Stock: sim.IntegerValue(0)})
	}
	return slots
}

func reproductionObservation(actor sim.EntityID, ref strategy.ReproductionRef, hour int) strategy.ReproductionObservation {
	home, err := world.ReproductionFounderPatchID(actor)
	if err != nil {
		panic(err)
	}
	return reproductionObservationAtHome(actor, home, ref, hour)
}

// reproductionObservationAtHome builds a valid mid-run claim view at the
// frozen bounds defaults; newborns pass their bag-carried home patch.
func reproductionObservationAtHome(actor, home sim.EntityID, ref strategy.ReproductionRef, hour int) strategy.ReproductionObservation {
	source, err := sim.EntityRefValue(home)
	if err != nil {
		panic(err)
	}
	return strategy.ReproductionObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(world.ReproductionFounderInitialEnergy), Hunger: sim.IntegerValue(0),
		BagUnits: sim.IntegerValue(0), BagSource: source,
		Capital: sim.IntegerValue(0), Wip: sim.IntegerValue(0),
		GranaryStock: sim.IntegerValue(0), InvestedUnits: sim.IntegerValue(0),
		LastBuildHour: sim.IntegerValue(world.ReproductionNeverHour), LastStoredMealHour: sim.IntegerValue(world.ReproductionNeverHour),
		HomeSlots: reproductionHomeSlots(home),
	}
}

func reproductionStocked(obs strategy.ReproductionObservation, slotIndexes ...int) strategy.ReproductionObservation {
	for _, i := range slotIndexes {
		obs.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	return obs
}

// reproductionRequestView builds a proposing actor that passes the frozen
// proposer condition (k>=2, granary>=3, energy>=3) with its same-patch
// founder roster.
func reproductionRequestView(actor sim.EntityID, ref strategy.ReproductionRef, hour int) strategy.ReproductionRequestObservation {
	home, err := world.ReproductionFounderPatchID(actor)
	if err != nil {
		panic(err)
	}
	var mates []sim.EntityID
	for id := sim.EntityID(1); id <= world.ReproductionFounderCount; id++ {
		if id == actor {
			continue
		}
		mateHome, err := world.ReproductionFounderPatchID(id)
		if err != nil {
			panic(err)
		}
		if mateHome == home {
			mates = append(mates, id)
		}
	}
	return strategy.ReproductionRequestObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9, HomePatch: home,
		Energy: sim.IntegerValue(8), Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		OwnRequest: strategy.ReproductionRequestGuard{Status: strategy.ReproductionRequestIdle, LastRequestHour: -1},
		PatchMates: mates,
	}
}

// reproductionReplyView builds a consenter whose own state sits exactly at
// the frozen consent margins (granary 3, k 2) with energy to spare, and one
// unrelated pending request passing every reported-claim margin.
func reproductionReplyView(consenter, requester sim.EntityID, ref strategy.ReproductionRef, hour int) strategy.ReproductionReplyObservation {
	return strategy.ReproductionReplyObservation{
		Actor: consenter, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(8), Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		OwnLastReplyHour: -1,
		Pending: []strategy.ReproductionPendingView{{
			Requester: requester, Addressee: consenter, Hour: hour,
			ReportedCapital: 2, ReportedGranary: 3, KinDistance: 0, KinRelated: false,
		}},
	}
}

// reproductionWorldStateAtMargins builds a valid world-contract founder
// state sitting above the consent margins: granary stock 3, capital 2,
// energy 8, with the extended G2a/G2b/G3 identities intact.
func reproductionWorldStateAtMargins(t *testing.T, actor sim.EntityID) world.ReproductionActorState {
	t.Helper()
	a, err := world.ReproductionFounderState(actor, world.ReproductionFounderLifetimeMax)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = world.ReproductionWorksiteState{Capital: 2, PointsCreated: 2, InvestedUnits: 4, LastBuildHour: 1}
	a.Granary = world.ReproductionGranaryState{Stock: 3, YieldTotal: 3, LastStoredMealHour: world.ReproductionNeverHour}
	a.Body.Energy = 8
	a.Body.BasalSpent = 3 // G3: 11 initial = 8 energy + 3 basal spent
	if !world.ReproductionSurplusGateAllows(a, a, true, 0) {
		t.Fatalf("margin state for actor %d must pass the world surplus gate", actor)
	}
	return a
}

func TestReproductionFrozenPolicyRegistration(t *testing.T) {
	p := strategy.FrozenReproductionPolicy()
	if p != (strategy.ReproductionPolicy{FormatVersion: 4, Ref: strategy.ReproductionRef{ID: "reproduction", Version: 4},
		Budget: strategy.ReproductionBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen reproduction policy drifted: %+v", p)
	}
	r := strategy.NewReproductionRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for _, invalid := range []strategy.ReproductionPolicy{
		func() strategy.ReproductionPolicy { c := p; c.FormatVersion = 3; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Ref.ID = ""; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Ref.ID = "reproduction-v4"; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Ref.ID = longID; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Ref.Version = 0; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Ref.Version = 5; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Budget.Candidates = 0; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Budget.Candidates = 33; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Budget.Evaluations = 0; return c }(),
		func() strategy.ReproductionPolicy { c := p; c.Budget.Evaluations = 33; return c }(),
	} {
		if err := r.Register(invalid); !errors.Is(err, strategy.ErrInvalidReproductionPolicy) {
			t.Fatalf("invalid reproduction policy accepted: %+v %v", invalid, err)
		}
	}
	if err := r.Register(p); !errors.Is(err, strategy.ErrInvalidReproductionPolicy) {
		t.Fatalf("duplicate reference accepted: %v", err)
	}
	if _, err := r.Bind(strategy.ReproductionBinding{Actor: 1, Ref: strategy.ReproductionRef{ID: p.Ref.ID, Version: 5}}); !errors.Is(err, strategy.ErrUnknownReproductionPolicy) {
		t.Fatalf("unregistered version bound: %v", err)
	}
	if _, err := r.Bind(strategy.ReproductionBinding{Actor: 1, Ref: strategy.ReproductionRef{ID: "", Version: 4}}); !errors.Is(err, strategy.ErrInvalidReproductionBinding) {
		t.Fatalf("empty ref bound: %v", err)
	}
	// The full roster {1-16} ∪ {10001-10016} binds the same frozen ref;
	// everything outside it — founder overflow, newborn neighbors, zero —
	// is rejected.
	for _, actor := range []sim.EntityID{0, 17, 10000, 10017, 999999} {
		if _, err := r.Bind(strategy.ReproductionBinding{Actor: actor, Ref: p.Ref}); !errors.Is(err, strategy.ErrInvalidReproductionBinding) {
			t.Fatalf("actor %d outside the fixed roster bound: %v", actor, err)
		}
	}
	for actor := sim.EntityID(1); actor <= world.ReproductionFounderCount; actor++ {
		if _, err := r.Bind(strategy.ReproductionBinding{Actor: actor, Ref: p.Ref}); err != nil {
			t.Fatalf("founder %d binding: %v", actor, err)
		}
	}
	for births := int64(0); births < world.ReproductionMaxBirths; births++ {
		newborn, err := world.ReproductionNewbornID(births)
		if err != nil {
			t.Fatal(err)
		}
		b, err := r.Bind(strategy.ReproductionBinding{Actor: newborn, Ref: p.Ref})
		if err != nil {
			t.Fatalf("newborn %d fresh binding: %v", newborn, err)
		}
		if b.Binding() != (strategy.ReproductionBinding{Actor: newborn, Ref: p.Ref}) || b.Policy() != p {
			t.Fatalf("newborn %d binding roundtrip: %+v %+v", newborn, b.Binding(), b.Policy())
		}
	}
	b, err := r.Bind(strategy.ReproductionBinding{Actor: 16, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	mutated := b.Policy()
	mutated.Budget.Candidates = 1
	if b.Policy() != p || b.Binding() != (strategy.ReproductionBinding{Actor: 16, Ref: p.Ref}) {
		t.Fatal("bound policy is not value-only")
	}
	other, err := r.Bind(strategy.ReproductionBinding{Actor: 1, Ref: p.Ref})
	if err != nil || other.Policy() != p {
		t.Fatalf("registry content changed: %+v %v", other.Policy(), err)
	}
	var nilBound *strategy.ReproductionBound
	if nilBound.Binding() != (strategy.ReproductionBinding{}) || nilBound.Policy() != (strategy.ReproductionPolicy{}) {
		t.Fatal("nil bound must degrade to zero values")
	}
	if _, err := nilBound.Evaluate(reproductionObservation(1, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("nil bound evaluated a claim view")
	}
	if _, err := nilBound.EvaluateBirthRequest(reproductionRequestView(1, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("nil bound evaluated a request view")
	}
	if _, err := nilBound.EvaluateBirthReply(reproductionReplyView(1, 2, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("nil bound evaluated a reply view")
	}
}

// The policy lane must stay identical to the trusted world contract: frozen
// identity, roster geometry, consent margins, kin limits, horizon, phases,
// and the ten-minute v1 rest window.
func TestReproductionLaneMatchesWorldContract(t *testing.T) {
	p := strategy.FrozenReproductionPolicy()
	if uint32(world.ReproductionFormatVersion) != p.FormatVersion || uint32(world.ReproductionSchemaVersion) != p.Ref.Version ||
		uint32(world.ReproductionRuleVersion) != 4 {
		t.Fatalf("v4 identity drift: world %d/%d/%d", world.ReproductionFormatVersion, world.ReproductionSchemaVersion, world.ReproductionRuleVersion)
	}
	if strategy.ReproductionMaxFounders != world.ReproductionFounderCount || strategy.ReproductionMaxBirths != world.ReproductionMaxBirths ||
		strategy.ReproductionFirstNewbornID != world.ReproductionFirstNewbornID {
		t.Fatalf("roster drift: %d/%d/%d", strategy.ReproductionMaxFounders, strategy.ReproductionMaxBirths, strategy.ReproductionFirstNewbornID)
	}
	if strategy.ReproductionRestDuration != world.ReproductionMealDuration || world.ReproductionMealDuration != sim.Duration(600*1e6) {
		t.Fatalf("rest window drifted from the ten-minute v1 discipline: %d vs %d", strategy.ReproductionRestDuration, world.ReproductionMealDuration)
	}
	if world.ReproductionConsentMinGranary != 3 || world.ReproductionConsentMinCapital != 2 || world.ReproductionConsentMinEnergy != 3 ||
		world.ReproductionKinProhibitedDepth != 2 || world.ReproductionKinWalkLimit != 10 || world.ReproductionPolicyTLow != 5 ||
		world.ReproductionKMax != 8 || world.ReproductionGranaryMax != 8 || world.ReproductionWipMax != 1 {
		t.Fatal("frozen consent/gate/cascade thresholds changed in the world contract")
	}
	if int(world.ReproductionPhaseBirthRequest) != 6 || int(world.ReproductionPhaseBirthReply) != 7 {
		t.Fatalf("birth phases must extend the untouched v3 phases 0-5: %d %d",
			world.ReproductionPhaseBirthRequest, world.ReproductionPhaseBirthReply)
	}
	if world.ReproductionHorizonHours != 168 {
		t.Fatalf("horizon drift: %d", world.ReproductionHorizonHours)
	}
	// Every home-patch slot the world names is gatherable by a home actor's
	// view and by nobody else; the admissible set is exactly the home patch.
	for actor := sim.EntityID(1); actor <= world.ReproductionFounderCount; actor++ {
		home, err := world.ReproductionFounderPatchID(actor)
		if err != nil {
			t.Fatal(err)
		}
		bound, ref := reproductionFixture(t, actor)
		for i := 0; i < world.ReproductionSlotsPerPatch; i++ {
			slot, err := world.ReproductionSlotID(int(home-1001), i)
			if err != nil {
				t.Fatal(err)
			}
			obs := reproductionStocked(reproductionObservation(actor, ref, 0), i)
			choice, err := bound.Evaluate(obs)
			if err != nil || choice.Kind != strategy.ReproductionGather || choice.TargetSlot != slot || choice.Evaluated != 1 {
				t.Fatalf("actor %d slot %d: %+v %v", actor, slot, choice, err)
			}
		}
		if _, err := bound.Evaluate(reproductionObservation(actor, ref, world.ReproductionHorizonHours-1)); err != nil {
			t.Fatalf("final claimable hour %d rejected: %v", world.ReproductionHorizonHours-1, err)
		}
		if _, err := bound.Evaluate(reproductionObservation(actor, ref, world.ReproductionHorizonHours)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
			t.Fatalf("hour %d (basal only) accepted: %v", world.ReproductionHorizonHours, err)
		}
	}
	// A newborn's placement is its bag-carried home patch, not its ID; the
	// same slot geometry applies.
	newborn, _ := world.ReproductionNewbornID(0)
	bound, ref := reproductionFixture(t, newborn)
	obs := reproductionStocked(reproductionObservationAtHome(newborn, 1002, ref, 3), 4)
	slot, err := world.ReproductionSlotID(1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if choice, err := bound.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionGather || choice.TargetSlot != slot {
		t.Fatalf("newborn claim on its carried home patch: %+v %v", choice, err)
	}
}

func TestReproductionClaimCascade(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	// Visible stock wins over the granary, ties break on the lowest slot ID,
	// and the whole candidate set is evaluated (all-or-nothing budget).
	obs := reproductionStocked(reproductionObservation(1, ref, 5), 2, 0, 5)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionGather, TargetSlot: 2001,
		ObservedVersion: 9, Ref: ref, Evaluated: 3}) {
		t.Fatalf("gather must take the lowest stocked home slot: %+v %v", choice, err)
	}
	// Input order never matters: the same view shuffled picks the same slot.
	shuffled := reproductionStocked(reproductionObservation(1, ref, 5), 5, 2, 0)
	shuffled.HomeSlots[3], shuffled.HomeSlots[6] = shuffled.HomeSlots[6], shuffled.HomeSlots[3]
	if choice, err := b.Evaluate(shuffled); err != nil || choice.TargetSlot != 2001 || choice.Evaluated != 3 {
		t.Fatalf("shuffled slots must not change the choice: %+v %v", choice, err)
	}
	// No visible stock, granary holds a unit: the only granary draw.
	obs = reproductionObservation(1, ref, 5)
	obs.GranaryStock = sim.IntegerValue(1)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("empty wild and stocked granary must draw a stored meal: %+v %v", choice, err)
	}
	// Nothing anywhere: Wait is a fallback with no evaluation.
	choice, err = b.Evaluate(reproductionObservation(1, ref, 5))
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionWait,
		ObservedVersion: 9, Ref: ref, Fallback: strategy.ReproductionNoCandidate}) {
		t.Fatalf("nothing available must wait: %+v %v", choice, err)
	}
	// Gather priority over stored meals when both are visible.
	obs = reproductionStocked(reproductionObservation(1, ref, 5), 7)
	obs.GranaryStock = sim.IntegerValue(8)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionGather || choice.TargetSlot != 2008 {
		t.Fatalf("wild gather precedes stored meals: %+v %v", choice, err)
	}
	// A dead actor never acts.
	obs = reproductionStocked(reproductionObservation(1, ref, 5), 0)
	obs.Energy = sim.IntegerValue(0)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionWait || choice.Evaluated != 0 || choice.Fallback != strategy.ReproductionNoCandidate {
		t.Fatalf("zero-energy actor must not act: %+v %v", choice, err)
	}
}

// The frozen bag decision (h+20m+2µs), verbatim v3: Build iff (energy>=5 or
// granary>=1) and k+wip<8, else Eat. The policy never emits EatStored here:
// a build's paired stored meal is drawn fresh by the runner at completion.
func TestReproductionBagDecisionTLowBoundaryAndCapRefusal(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	home, _ := world.ReproductionFounderPatchID(1)
	source, _ := sim.EntityRefValue(home)
	base := reproductionObservation(1, ref, 5)
	base.BagUnits = sim.IntegerValue(1)
	base.BagSource = source
	for _, tc := range []struct {
		name                       string
		energy, capital, wip, gran int64
		want                       strategy.ReproductionAction
	}{
		{"one below the frozen floor eats", 4, 0, 0, 0, strategy.ReproductionEat},
		{"at the frozen floor builds", 5, 0, 0, 0, strategy.ReproductionBuild},
		{"granary fallback funds a build at energy 4", 4, 0, 0, 1, strategy.ReproductionBuild},
		{"full energy builds", 12, 0, 0, 0, strategy.ReproductionBuild},
		{"seven points plus wip is saturated", 12, 7, 1, 0, strategy.ReproductionEat},
		{"eight points is saturated", 12, 8, 0, 0, strategy.ReproductionEat},
		{"seven points without wip still builds", 12, 7, 0, 0, strategy.ReproductionBuild},
		{"outstanding wip completes the second unit", 5, 0, 1, 0, strategy.ReproductionBuild},
		{"low energy with stocked granary builds for the runner-paired meal", 4, 0, 0, 8, strategy.ReproductionBuild},
	} {
		obs := base
		obs.Energy = sim.IntegerValue(tc.energy)
		obs.Capital = sim.IntegerValue(tc.capital)
		obs.Wip = sim.IntegerValue(tc.wip)
		obs.GranaryStock = sim.IntegerValue(tc.gran)
		choice, err := b.Evaluate(obs)
		if err != nil || choice != (strategy.ReproductionChoice{Kind: tc.want,
			ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		if choice.Kind == strategy.ReproductionEatStored {
			t.Fatalf("%s: bag decision emitted a stored meal", tc.name)
		}
	}
}

func TestReproductionDeniedGatherFallback(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	// Denied with a stocked granary: the fallback draw, never a re-gather.
	obs := reproductionStocked(reproductionObservation(1, ref, 5), 0, 3)
	obs.GranaryStock = sim.IntegerValue(1)
	obs.LastDenial = strategy.ReproductionGatherDenied
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer with a stocked granary must eat stored: %+v %v", choice, err)
	}
	// Denied with an empty granary: the frozen ten-minute rest.
	obs.GranaryStock = sim.IntegerValue(0)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionRest,
		RestDuration: strategy.ReproductionRestDuration, ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer without stores must rest: %+v %v", choice, err)
	}
	// Without a denial the same view gathers again instead.
	obs.LastDenial = strategy.ReproductionNoDenial
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionGather || choice.TargetSlot != 2001 {
		t.Fatalf("undecided claim must gather: %+v %v", choice, err)
	}
}

// At most one EatStored per actor-hour: every stored-meal emission requires
// LastStoredMealHour < the observed hour, so a re-evaluation at any later
// frozen slot of the same hour can never double-draw.
func TestReproductionOneStoredMealPerActorHour(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	const hour = 5
	obs := reproductionObservation(1, ref, hour)
	obs.GranaryStock = sim.IntegerValue(2)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionEatStored {
		t.Fatalf("claim draw: %+v %v", choice, err)
	}
	for _, gran := range []int64{0, 1, 2, 8} {
		reobserved := obs
		reobserved.GranaryStock = sim.IntegerValue(gran)
		reobserved.LastStoredMealHour = sim.IntegerValue(hour)
		choice, err := b.Evaluate(reobserved)
		if err != nil || choice.Kind == strategy.ReproductionEatStored {
			t.Fatalf("granary %d: second draw in hour %d: %+v %v", gran, hour, choice, err)
		}
		if choice.Kind != strategy.ReproductionWait {
			t.Fatalf("granary %d: spent-hour claim must wait, got %+v", gran, choice)
		}
	}
	denied := obs
	denied.LastDenial = strategy.ReproductionGatherDenied
	denied.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(denied); err != nil || choice.Kind != strategy.ReproductionRest || choice.RestDuration != strategy.ReproductionRestDuration {
		t.Fatalf("denied fallback must not double-draw: %+v %v", choice, err)
	}
	held := reproductionObservation(1, ref, hour)
	home, _ := world.ReproductionFounderPatchID(1)
	held.BagUnits = sim.IntegerValue(1)
	held.BagSource, _ = sim.EntityRefValue(home)
	held.GranaryStock = sim.IntegerValue(3)
	held.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.ReproductionBuild {
		t.Fatalf("bag decision with granary fallback builds: %+v %v", choice, err)
	}
	held.Capital = sim.IntegerValue(8) // saturated worksite: eat the held unit itself
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.ReproductionEat {
		t.Fatalf("saturated bag decision eats the held unit itself: %+v %v", choice, err)
	}
	next := obs
	next.Hour = hour + 1
	if choice, err := b.Evaluate(next); err != nil || choice.Kind != strategy.ReproductionEatStored {
		t.Fatalf("hour %d draw after hour %d meal: %+v %v", hour+1, hour, choice, err)
	}
}

func TestReproductionBudgetExhaustion(t *testing.T) {
	limited := strategy.NewReproductionRegistry()
	p := strategy.FrozenReproductionPolicy()
	p.Budget = strategy.ReproductionBudget{Candidates: 1, Evaluations: 1}
	if err := limited.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := limited.Bind(strategy.ReproductionBinding{Actor: 1, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	// Two stocked slots against a one-slot budget: the whole candidate set is
	// declined, never a sorted prefix.
	obs := reproductionStocked(reproductionObservation(1, p.Ref, 5), 0, 1)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.ReproductionChoice{Kind: strategy.ReproductionWait, ObservedVersion: 9,
		Ref: p.Ref, Fallback: strategy.ReproductionBudgetExhausted}) {
		t.Fatalf("all-or-nothing claim budget: %+v %v", choice, err)
	}
	if choice, err := b.Evaluate(reproductionStocked(reproductionObservation(1, p.Ref, 5), 4)); err != nil ||
		choice.Kind != strategy.ReproductionGather || choice.TargetSlot != 2005 || choice.Evaluated != 1 {
		t.Fatalf("bounded candidate set: %+v %v", choice, err)
	}
	// Two same-patch mates against a one-mate request budget: no filing.
	request := reproductionRequestView(1, p.Ref, 5)
	choice2, err := b.EvaluateBirthRequest(request)
	if err != nil || choice2 != (strategy.ReproductionRequestChoice{Kind: strategy.ReproductionRequestWait,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.ReproductionBudgetExhausted}) {
		t.Fatalf("all-or-nothing request budget: %+v %v", choice2, err)
	}
	request.PatchMates = []sim.EntityID{2}
	if choice, err := b.EvaluateBirthRequest(request); err != nil || choice.Kind != strategy.ReproductionRequestFile || choice.Addressee != 2 {
		t.Fatalf("bounded request set: %+v %v", choice, err)
	}
	// Two pending requests against a one-reply budget: no reply at all.
	reply := reproductionReplyView(1, 2, p.Ref, 5)
	reply.Pending = append(reply.Pending, strategy.ReproductionPendingView{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	choice3, err := b.EvaluateBirthReply(reply)
	if err != nil || choice3 != (strategy.ReproductionReplyChoice{Kind: strategy.ReproductionReplyNone,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.ReproductionBudgetExhausted}) {
		t.Fatalf("all-or-nothing reply budget: %+v %v", choice3, err)
	}
	// The frozen 32/32 budget covers every maximal legal candidate set: all
	// eight home slots, the full founder+newborn same-patch roster, and a
	// full pending list evaluate without exhaustion.
	fb, ref := reproductionFixture(t, 1)
	all := reproductionObservation(1, ref, 5)
	for i := range all.HomeSlots {
		all.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	if choice, err := fb.Evaluate(all); err != nil || choice.Kind != strategy.ReproductionGather ||
		choice.TargetSlot != 2001 || choice.Evaluated != 8 || choice.Fallback != strategy.ReproductionNoFallback {
		t.Fatalf("frozen budget must never truncate the legal slot set: %+v %v", choice, err)
	}
	big := reproductionRequestView(1, ref, 5)
	for births := int64(0); births < world.ReproductionMaxBirths; births++ {
		id, err := world.ReproductionNewbornID(births)
		if err != nil {
			t.Fatal(err)
		}
		big.PatchMates = append(big.PatchMates, id) // newborns join the proposer's patch roster
	}
	if choice, err := fb.EvaluateBirthRequest(big); err != nil || choice.Kind != strategy.ReproductionRequestFile ||
		choice.Addressee != 2 || choice.Evaluated != 23 || choice.Fallback != strategy.ReproductionNoFallback {
		t.Fatalf("frozen budget must never truncate the legal roster: %+v %v", choice, err)
	}
	full := reproductionReplyView(1, 2, ref, 5)
	for id := sim.EntityID(3); id <= 16; id++ {
		full.Pending = append(full.Pending, strategy.ReproductionPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	}
	full.Pending = append(full.Pending, strategy.ReproductionPendingView{Requester: 10001, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	if choice, err := fb.EvaluateBirthReply(full); err != nil || choice.Kind != strategy.ReproductionReplyAccept ||
		choice.Requester != 2 || choice.Evaluated != 1 || choice.Fallback != strategy.ReproductionNoFallback {
		t.Fatalf("frozen budget must never truncate the legal pending set: %+v %v", choice, err)
	}
}

// The frozen proposer decision at h+40m+5µs: living, free request slot,
// one request per actor-hour, own state at the surplus-gate margins, and a
// deterministic same-patch addressee carrying the actor's reported claims.
func TestReproductionBirthRequestGatesDeterministicAddresseeAndClaims(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	obs := reproductionRequestView(1, ref, 5)
	choice, err := b.EvaluateBirthRequest(obs)
	if err != nil || choice != (strategy.ReproductionRequestChoice{Kind: strategy.ReproductionRequestFile, Addressee: 2,
		ReportedCapital: 2, ReportedGranary: 3, ObservedVersion: 9, Ref: ref, Evaluated: 7, Fallback: strategy.ReproductionNoFallback}) {
		t.Fatalf("eligible proposer must file: %+v %v", choice, err)
	}
	// The directed successor: actor 8 wraps to the lowest same-patch ID.
	b8, ref8 := reproductionFixture(t, 8)
	if choice, err := b8.EvaluateBirthRequest(reproductionRequestView(8, ref8, 5)); err != nil ||
		choice.Addressee != 1 || choice.Kind != strategy.ReproductionRequestFile {
		t.Fatalf("wraparound addressee: %+v %v", choice, err)
	}
	// Input order never matters.
	shuffled := reproductionRequestView(1, ref, 5)
	sort.Slice(shuffled.PatchMates, func(i, j int) bool { return shuffled.PatchMates[i] > shuffled.PatchMates[j] })
	if choice, err := b.EvaluateBirthRequest(shuffled); err != nil || choice.Addressee != 2 || choice.Evaluated != 7 {
		t.Fatalf("shuffled roster must not change the choice: %+v %v", choice, err)
	}
	// Frozen proposer condition boundaries (the same margins the runner's
	// surplus gate re-checks on fresh truth).
	for _, tc := range []struct {
		name                     string
		energy, capital, granary int64
		want                     strategy.ReproductionRequestAction
	}{
		{"capital one below the margin waits", 8, 1, 3, strategy.ReproductionRequestWait},
		{"capital at the margin files", 8, 2, 3, strategy.ReproductionRequestFile},
		{"granary one below the margin waits", 8, 2, 2, strategy.ReproductionRequestWait},
		{"granary at the margin files", 8, 2, 3, strategy.ReproductionRequestFile},
		{"energy one below the margin waits", 2, 2, 3, strategy.ReproductionRequestWait},
		{"energy at the margin files", 3, 2, 3, strategy.ReproductionRequestFile},
		{"a dead actor never proposes", 0, 8, 8, strategy.ReproductionRequestWait},
	} {
		view := reproductionRequestView(1, ref, 5)
		view.Energy = sim.IntegerValue(tc.energy)
		view.Capital = sim.IntegerValue(tc.capital)
		view.GranaryStock = sim.IntegerValue(tc.granary)
		choice, err := b.EvaluateBirthRequest(view)
		if err != nil || choice.Kind != tc.want {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		if tc.want == strategy.ReproductionRequestWait && choice.Fallback != strategy.ReproductionNoCandidate {
			t.Fatalf("%s: condition miss must be a plain wait: %+v", tc.name, choice)
		}
	}
	// One request per actor-hour: a pending request blocks, and any state
	// that already requested this hour blocks; yesterday's expired request
	// frees the slot again.
	blocked := reproductionRequestView(1, ref, 5)
	for _, guard := range []strategy.ReproductionRequestGuard{
		{Status: strategy.ReproductionRequestPending, LastRequestHour: 4},
		{Status: strategy.ReproductionRequestPending, LastRequestHour: 5},
		{Status: strategy.ReproductionRequestAccepted, LastRequestHour: 5},
		{Status: strategy.ReproductionRequestRefused, LastRequestHour: 5},
		{Status: strategy.ReproductionRequestExpired, LastRequestHour: 5},
	} {
		blocked.OwnRequest = guard
		if choice, err := b.EvaluateBirthRequest(blocked); err != nil || choice.Kind != strategy.ReproductionRequestWait {
			t.Fatalf("guard %+v must block: %+v %v", guard, choice, err)
		}
	}
	expired := reproductionRequestView(1, ref, 5)
	expired.OwnRequest = strategy.ReproductionRequestGuard{Status: strategy.ReproductionRequestExpired, LastRequestHour: 4}
	if choice, err := b.EvaluateBirthRequest(expired); err != nil || choice.Kind != strategy.ReproductionRequestFile || choice.Addressee != 2 {
		t.Fatalf("expired request frees the slot: %+v %v", choice, err)
	}
	// The CLAIM is the actor's reported values: the override replaces the
	// reported claims but never the actor's own eligibility or addressee.
	forged := reproductionRequestView(1, ref, 5)
	forged.TestClaims = &strategy.ReproductionClaimOverride{Capital: 0, Granary: 0}
	choice, err = b.EvaluateBirthRequest(forged)
	if err != nil || choice.Kind != strategy.ReproductionRequestFile || choice.Addressee != 2 ||
		choice.ReportedCapital != 0 || choice.ReportedGranary != 0 || choice.Evaluated != 7 {
		t.Fatalf("override changes only the reported claims: %+v %v", choice, err)
	}
	if !reflect.DeepEqual(obs.Capital, sim.IntegerValue(2)) || !reflect.DeepEqual(obs.GranaryStock, sim.IntegerValue(3)) {
		t.Fatal("observation mutated by evaluation")
	}
	lowTruth := reproductionRequestView(1, ref, 5)
	lowTruth.Capital = sim.IntegerValue(1) // own truth below the proposer condition
	lowTruth.TestClaims = &strategy.ReproductionClaimOverride{Capital: 8, Granary: 8}
	if choice, err := b.EvaluateBirthRequest(lowTruth); err != nil || choice.Kind != strategy.ReproductionRequestWait {
		t.Fatalf("a forged claim must not rescue own eligibility: %+v %v", choice, err)
	}
	// Structural malformations are errors, never choices.
	selfMate := reproductionRequestView(1, ref, 5)
	selfMate.PatchMates = []sim.EntityID{1, 2}
	dupMate := reproductionRequestView(1, ref, 5)
	dupMate.PatchMates = []sim.EntityID{2, 2}
	crossPatchMate := reproductionRequestView(1, ref, 5)
	crossPatchMate.PatchMates = []sim.EntityID{9} // patch 1002 actor in a patch-1001 roster
	foreignRosterID := reproductionRequestView(1, ref, 5)
	foreignRosterID.PatchMates = []sim.EntityID{17}
	foreignNewborn := reproductionRequestView(1, ref, 5)
	foreignNewborn.PatchMates = []sim.EntityID{10017}
	neighborNewborn := reproductionRequestView(1, ref, 5)
	neighborNewborn.PatchMates = []sim.EntityID{10000}
	badStatus := reproductionRequestView(1, ref, 5)
	badStatus.OwnRequest.Status = 5
	futureGuard := reproductionRequestView(1, ref, 5)
	futureGuard.OwnRequest.LastRequestHour = 6
	idleWithHour := reproductionRequestView(1, ref, 5)
	idleWithHour.OwnRequest.LastRequestHour = 4
	fileWithNoHour := reproductionRequestView(1, ref, 5)
	fileWithNoHour.OwnRequest = strategy.ReproductionRequestGuard{Status: strategy.ReproductionRequestRefused, LastRequestHour: -1}
	wrongPatch := reproductionRequestView(1, ref, 5)
	wrongPatch.HomePatch = 1002
	badPatch := reproductionRequestView(1, ref, 5)
	badPatch.HomePatch = 1003
	highClaim := reproductionRequestView(1, ref, 5)
	highClaim.TestClaims = &strategy.ReproductionClaimOverride{Capital: 9, Granary: 3}
	negativeClaim := reproductionRequestView(1, ref, 5)
	negativeClaim.TestClaims = &strategy.ReproductionClaimOverride{Capital: 2, Granary: -1}
	actorMismatch := reproductionRequestView(1, ref, 5)
	actorMismatch.Actor = 2
	refMismatch := reproductionRequestView(1, ref, 5)
	refMismatch.Ref = strategy.ReproductionRef{ID: "other", Version: 4}
	tooManyMates := reproductionRequestView(1, ref, 5)
	for i := 0; i < 33; i++ {
		tooManyMates.PatchMates = append(tooManyMates.PatchMates, sim.EntityID(200+i))
	}
	for name, view := range map[string]strategy.ReproductionRequestObservation{
		"self mate": selfMate, "duplicate mate": dupMate, "cross-patch mate": crossPatchMate,
		"roster-invalid mate": foreignRosterID, "newborn overflow mate": foreignNewborn,
		"newborn underflow mate": neighborNewborn, "unknown status": badStatus,
		"future guard": futureGuard, "idle with hour": idleWithHour, "filed without hour": fileWithNoHour,
		"wrong founder patch": wrongPatch, "invalid patch": badPatch,
		"claim capital high": highClaim, "claim granary negative": negativeClaim,
		"actor mismatch": actorMismatch, "ref mismatch": refMismatch, "roster overflow": tooManyMates,
	} {
		if _, err := b.EvaluateBirthRequest(view); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
			t.Fatalf("%s accepted: %+v %v", name, view, err)
		}
	}
	hourBound := reproductionRequestView(1, ref, world.ReproductionHorizonHours)
	if _, err := b.EvaluateBirthRequest(hourBound); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatalf("hour %d accepted: %v", world.ReproductionHorizonHours, err)
	}
	// Out-of-range own state degrades to Wait + InvalidSelf.
	invalidSelf := reproductionRequestView(1, ref, 5)
	invalidSelf.Capital = sim.IntegerValue(9)
	choice, err = b.EvaluateBirthRequest(invalidSelf)
	if err != nil || choice.Kind != strategy.ReproductionRequestWait || choice.Fallback != strategy.ReproductionInvalidSelf || choice.Evaluated != 0 {
		t.Fatalf("invalid own capital: %+v %v", choice, err)
	}
	invalidSelf = reproductionRequestView(1, ref, 5)
	invalidSelf.Energy = sim.BoolValue(true)
	if choice, err := b.EvaluateBirthRequest(invalidSelf); err != nil || choice.Fallback != strategy.ReproductionInvalidSelf {
		t.Fatalf("unknown own energy: %+v %v", choice, err)
	}
}

// The frozen consent formula as a truth table, each row cross-checked
// against the world contract's evaluator: own granary>=3 ∧ own k>=2 ∧ own
// energy>=3 ∧ reported k>=2 ∧ reported granary>=3 ∧ (kin-distance>=3 ∨
// unrelated).
func TestReproductionBirthReplyConsentTruthTable(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	for _, tc := range []struct {
		name                             string
		granary, capital, energy         int64
		reportedCapital, reportedGranary int64
		kinRelated                       bool
		kinDistance                      int
		want                             strategy.ReproductionReplyAction
		reason                           strategy.ReproductionReplyReason
	}{
		{"all margins met accepts", 3, 2, 3, 2, 3, false, 0, strategy.ReproductionReplyAccept, strategy.ReproductionReplyReasonNone},
		{"spare state accepts", 8, 8, 8, 8, 8, false, 0, strategy.ReproductionReplyAccept, strategy.ReproductionReplyReasonNone},
		{"low reserve granary refuses", 2, 2, 3, 2, 3, false, 0, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentOwn},
		{"capital one below refuses", 3, 1, 3, 2, 3, false, 0, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentOwn},
		{"energy one below refuses", 3, 2, 2, 2, 3, false, 0, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentOwn},
		{"reported capital one below refuses", 3, 2, 3, 1, 3, false, 0, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentClaims},
		{"reported granary one below refuses", 3, 2, 3, 2, 2, false, 0, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentClaims},
		{"parent-child distance 1 prohibited", 3, 2, 3, 2, 3, true, 1, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonKin},
		{"sibling distance 2 prohibited", 3, 2, 3, 2, 3, true, 2, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonKin},
		{"cousin distance 3 allowed", 3, 2, 3, 2, 3, true, 3, strategy.ReproductionReplyAccept, strategy.ReproductionReplyReasonNone},
		{"distant kin allowed", 3, 2, 3, 2, 3, true, 10, strategy.ReproductionReplyAccept, strategy.ReproductionReplyReasonNone},
		{"own state precedes claims and kin", 2, 1, 2, 1, 2, true, 1, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentOwn},
		{"claims precede kin", 3, 2, 3, 1, 2, true, 1, strategy.ReproductionReplyRefuse, strategy.ReproductionReplyReasonConsentClaims},
	} {
		view := reproductionReplyView(1, 2, ref, 5)
		view.GranaryStock = sim.IntegerValue(tc.granary)
		view.Capital = sim.IntegerValue(tc.capital)
		view.Energy = sim.IntegerValue(tc.energy)
		view.Pending[0].ReportedCapital = tc.reportedCapital
		view.Pending[0].ReportedGranary = tc.reportedGranary
		view.Pending[0].KinRelated = tc.kinRelated
		view.Pending[0].KinDistance = tc.kinDistance
		choice, err := b.EvaluateBirthReply(view)
		if err != nil || choice.Kind != tc.want || choice.Reason != tc.reason || choice.Requester != 2 {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		kinAllowed := !world.ReproductionKinProhibited(tc.kinDistance, tc.kinRelated)
		if allowed := world.ReproductionConsentAllows(tc.granary, tc.capital, tc.energy, tc.reportedCapital, tc.reportedGranary, kinAllowed); allowed != (tc.want == strategy.ReproductionReplyAccept) {
			t.Fatalf("%s: policy diverges from the world consent evaluator: world=%v", tc.name, allowed)
		}
	}
	// Exhaustive small-grid cross-check against the world evaluator.
	for _, granary := range []int64{2, 3, 8} {
		for _, capital := range []int64{1, 2, 8} {
			for _, energy := range []int64{2, 3, 8} {
				for _, reportedCapital := range []int64{1, 2} {
					for _, reportedGranary := range []int64{2, 3} {
						for _, kin := range []struct {
							related  bool
							distance int
						}{{false, 0}, {true, 1}, {true, 2}, {true, 3}, {true, 10}} {
							view := reproductionReplyView(1, 2, ref, 5)
							view.GranaryStock = sim.IntegerValue(granary)
							view.Capital = sim.IntegerValue(capital)
							view.Energy = sim.IntegerValue(energy)
							view.Pending[0].ReportedCapital = reportedCapital
							view.Pending[0].ReportedGranary = reportedGranary
							view.Pending[0].KinRelated = kin.related
							view.Pending[0].KinDistance = kin.distance
							choice, err := b.EvaluateBirthReply(view)
							kinAllowed := !world.ReproductionKinProhibited(kin.distance, kin.related)
							allowed := world.ReproductionConsentAllows(granary, capital, energy, reportedCapital, reportedGranary, kinAllowed)
							if err != nil || (choice.Kind == strategy.ReproductionReplyAccept) != allowed {
								t.Fatalf("consent grid diverges: own %d/%d/%d claims %d/%d kin %v/%d: %+v %v world=%v",
									granary, capital, energy, reportedCapital, reportedGranary, kin.related, kin.distance, choice, err, allowed)
							}
						}
					}
				}
			}
		}
	}
	// Multiple pendings: the first acceptable in ascending requester order
	// wins; with none acceptable the lowest-ID request is refused with its
	// typed reason; input order never matters.
	multi := reproductionReplyView(1, 3, ref, 5)
	multi.Pending = []strategy.ReproductionPendingView{
		{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 1, ReportedGranary: 3}, // claims fail
		{Requester: 5, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3}, // acceptable
	}
	choice, err := b.EvaluateBirthReply(multi)
	if err != nil || choice.Kind != strategy.ReproductionReplyAccept || choice.Requester != 5 || choice.Evaluated != 2 {
		t.Fatalf("first acceptable pending must win: %+v %v", choice, err)
	}
	reversed := multi
	reversed.Pending = []strategy.ReproductionPendingView{multi.Pending[1], multi.Pending[0]}
	if choice, err := b.EvaluateBirthReply(reversed); err != nil || choice.Requester != 5 || choice.Evaluated != 2 {
		t.Fatalf("input order must not change the choice: %+v %v", choice, err)
	}
	multi.Pending[1].ReportedCapital = 0 // now nothing is acceptable
	choice, err = b.EvaluateBirthReply(multi)
	if err != nil || choice.Kind != strategy.ReproductionReplyRefuse || choice.Requester != 3 ||
		choice.Reason != strategy.ReproductionReplyReasonConsentClaims || choice.Evaluated != 2 {
		t.Fatalf("lowest-ID refusal when nothing is acceptable: %+v %v", choice, err)
	}
	// One reply per actor-hour: the ledger guard blocks, yesterday's reply
	// does not.
	replied := reproductionReplyView(1, 2, ref, 5)
	replied.OwnLastReplyHour = 5
	if choice, err := b.EvaluateBirthReply(replied); err != nil || choice.Kind != strategy.ReproductionReplyNone {
		t.Fatalf("second reply in one hour: %+v %v", choice, err)
	}
	replied.OwnLastReplyHour = 4
	if choice, err := b.EvaluateBirthReply(replied); err != nil || choice.Kind != strategy.ReproductionReplyAccept {
		t.Fatalf("reply after yesterday's: %+v %v", choice, err)
	}
	// A dead consenter never replies; an empty addressed set waits.
	dead := reproductionReplyView(1, 2, ref, 5)
	dead.Energy = sim.IntegerValue(0)
	if choice, err := b.EvaluateBirthReply(dead); err != nil || choice.Kind != strategy.ReproductionReplyNone || choice.Fallback != strategy.ReproductionNoCandidate {
		t.Fatalf("dead consenter: %+v %v", choice, err)
	}
	empty := reproductionReplyView(1, 2, ref, 5)
	empty.Pending = nil
	if choice, err := b.EvaluateBirthReply(empty); err != nil || choice.Kind != strategy.ReproductionReplyNone {
		t.Fatalf("no pending request: %+v %v", choice, err)
	}
	// Structural malformations are errors, never choices.
	mutators := map[string]func(*strategy.ReproductionReplyObservation){
		"actor mismatch": func(o *strategy.ReproductionReplyObservation) { o.Actor = 2 },
		"ref mismatch": func(o *strategy.ReproductionReplyObservation) {
			o.Ref = strategy.ReproductionRef{ID: "other", Version: 4}
		},
		"future reply hour":   func(o *strategy.ReproductionReplyObservation) { o.OwnLastReplyHour = 6 },
		"requester self":      func(o *strategy.ReproductionReplyObservation) { o.Pending[0].Requester = 1 },
		"requester overflow":  func(o *strategy.ReproductionReplyObservation) { o.Pending[0].Requester = 17 },
		"requester underflow": func(o *strategy.ReproductionReplyObservation) { o.Pending[0].Requester = 0 },
		"addressee other":     func(o *strategy.ReproductionReplyObservation) { o.Pending[0].Addressee = 2 },
		"stale hour":          func(o *strategy.ReproductionReplyObservation) { o.Pending[0].Hour = 4 },
		"claim high":          func(o *strategy.ReproductionReplyObservation) { o.Pending[0].ReportedCapital = 9 },
		"claim negative":      func(o *strategy.ReproductionReplyObservation) { o.Pending[0].ReportedGranary = -1 },
		"related zero distance": func(o *strategy.ReproductionReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 0
		},
		"distance beyond walk": func(o *strategy.ReproductionReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 11
		},
		"unrelated with distance": func(o *strategy.ReproductionReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = false, 1
		},
		"duplicate requester": func(o *strategy.ReproductionReplyObservation) {
			o.Pending = append(o.Pending, o.Pending[0])
		},
	}
	for name, mutate := range mutators {
		view := reproductionReplyView(1, 2, ref, 5)
		mutate(&view)
		if _, err := b.EvaluateBirthReply(view); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
			t.Fatalf("%s accepted: %+v %v", name, view, err)
		}
	}
	hourBound := reproductionReplyView(1, 2, ref, world.ReproductionHorizonHours)
	if _, err := b.EvaluateBirthReply(hourBound); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatalf("hour %d accepted: %v", world.ReproductionHorizonHours, err)
	}
	// Out-of-range own state degrades to None + InvalidSelf.
	invalidSelf := reproductionReplyView(1, 2, ref, 5)
	invalidSelf.GranaryStock = sim.IntegerValue(9)
	choice, err = b.EvaluateBirthReply(invalidSelf)
	if err != nil || choice.Kind != strategy.ReproductionReplyNone || choice.Fallback != strategy.ReproductionInvalidSelf || choice.Evaluated != 0 {
		t.Fatalf("invalid own granary: %+v %v", choice, err)
	}
}

// The claim is the actor's reported values, and the consenter decides on
// claims alone: two views identical except for the claims decide differently,
// and the reported claims land in the world's request row verbatim — truth
// never travels with them, and the world refuses the same claim failure on
// the same typed reason without the policy ever seeing truth.
func TestReproductionClaimReportingNeverLeaksTruth(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	forged := reproductionRequestView(1, ref, 5)
	forged.Capital = sim.IntegerValue(5) // own truth: k=5, granary=4
	forged.GranaryStock = sim.IntegerValue(4)
	forged.TestClaims = &strategy.ReproductionClaimOverride{Capital: 2, Granary: 0}
	choice, err := b.EvaluateBirthRequest(forged)
	if err != nil || choice.Kind != strategy.ReproductionRequestFile || choice.Addressee != 2 ||
		choice.ReportedCapital != 2 || choice.ReportedGranary != 0 {
		t.Fatalf("reported claims must be the choice's claims: %+v %v", choice, err)
	}
	// The world's request row carries exactly the reported values, never the
	// actor's truth: the frozen contract takes claims as given and never
	// consults them against truth at filing.
	filer := reproductionWorldStateAtMargins(t, 1) // truth k=3, granary=3 differs from the claims
	filed, err := world.ReproductionBirthRequest(5, 1, 2, choice.ReportedCapital, choice.ReportedGranary, filer)
	if err != nil {
		t.Fatal(err)
	}
	if filed.Request.ReportedCapital != 2 || filed.Request.ReportedGranary != 0 || filed.Request.Status != world.ReproductionRequestPending {
		t.Fatalf("world request row must carry the reported claims: %+v", filed.Request)
	}
	// The consenter decides on the claims alone: identical own state, only
	// the reported values differ.
	donor, ref := reproductionFixture(t, 2)
	lowClaim := reproductionReplyView(2, 1, ref, 5)
	lowClaim.Pending[0].ReportedCapital, lowClaim.Pending[0].ReportedGranary = 2, 0
	lowDecision, err := donor.EvaluateBirthReply(lowClaim)
	if err != nil || lowDecision.Kind != strategy.ReproductionReplyRefuse || lowDecision.Reason != strategy.ReproductionReplyReasonConsentClaims {
		t.Fatalf("reported granary 0 must refuse on claims: %+v %v", lowDecision, err)
	}
	highClaim := lowClaim
	highClaim.Pending[0].ReportedGranary = 3
	highDecision, err := donor.EvaluateBirthReply(highClaim)
	if err != nil || highDecision.Kind != strategy.ReproductionReplyAccept {
		t.Fatalf("reported granary 3 must accept on claims: %+v %v", highDecision, err)
	}
	// The world's reply evaluator refuses the same claim failure on the same
	// typed reason, independent of any truth the policy never saw.
	proposerState := reproductionWorldStateAtMargins(t, 1)
	proposerState.Request = filed.Request
	consenterState := reproductionWorldStateAtMargins(t, 2)
	result, err := world.ReproductionBirthReply(5, 0, true, make([]uint64, world.ReproductionDrawsPerBirth), 1, 2, proposerState, consenterState)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != world.ReproductionReplyRefuse || result.Reason != world.ReproductionRefusalConsentClaims {
		t.Fatalf("world must refuse the same claim failure: %+v", result)
	}
}

func TestReproductionClaimObservationValidation(t *testing.T) {
	b, ref := reproductionFixture(t, 1)
	valid := reproductionObservation(1, ref, 5)
	if _, err := b.Evaluate(valid); err != nil {
		t.Fatal(err)
	}
	actorMismatch := valid
	actorMismatch.Actor = 2
	refMismatch := valid
	refMismatch.Ref = strategy.ReproductionRef{ID: "other", Version: 4}
	badDenial := valid
	badDenial.LastDenial = 255
	foreign := valid
	foreign.HomeSlots = append([]strategy.ReproductionSlotView(nil), valid.HomeSlots...)
	foreign.HomeSlots[0].ID = 2009 // patch 1002 slot inside actor 1's view
	duplicate := valid
	duplicate.HomeSlots[1].ID = duplicate.HomeSlots[0].ID
	nineSlots := valid
	nineSlots.HomeSlots = append(append([]strategy.ReproductionSlotView(nil), valid.HomeSlots...), strategy.ReproductionSlotView{ID: 2017, Stock: sim.IntegerValue(0)})
	badStock := valid
	badStock.HomeSlots[2].Stock = sim.IntegerValue(2)
	for name, obs := range map[string]strategy.ReproductionObservation{
		"actor mismatch": actorMismatch, "ref mismatch": refMismatch, "unknown denial": badDenial,
		"foreign slot": foreign, "duplicate slot": duplicate, "nine slots": nineSlots, "slot stock 2": badStock,
	} {
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
			t.Fatalf("%s accepted: %+v %v", name, obs, err)
		}
	}
	// Out-of-range or non-integer own state degrades to Wait + InvalidSelf.
	mutators := map[string]func(*strategy.ReproductionObservation){
		"energy high":  func(o *strategy.ReproductionObservation) { o.Energy = sim.IntegerValue(13) },
		"energy low":   func(o *strategy.ReproductionObservation) { o.Energy = sim.IntegerValue(-1) },
		"energy text":  func(o *strategy.ReproductionObservation) { o.Energy = sim.BoolValue(true) },
		"hunger high":  func(o *strategy.ReproductionObservation) { o.Hunger = sim.IntegerValue(25) },
		"bag high":     func(o *strategy.ReproductionObservation) { o.BagUnits = sim.IntegerValue(2) },
		"bag low":      func(o *strategy.ReproductionObservation) { o.BagUnits = sim.IntegerValue(-1) },
		"capital high": func(o *strategy.ReproductionObservation) { o.Capital = sim.IntegerValue(9) },
		"wip high":     func(o *strategy.ReproductionObservation) { o.Wip = sim.IntegerValue(2) },
		"granary high": func(o *strategy.ReproductionObservation) { o.GranaryStock = sim.IntegerValue(9) },
		"granary low":  func(o *strategy.ReproductionObservation) { o.GranaryStock = sim.IntegerValue(-1) },
		"invested high": func(o *strategy.ReproductionObservation) {
			o.InvestedUnits = sim.IntegerValue(world.ReproductionHorizonHours + 1)
		},
		"build hour high": func(o *strategy.ReproductionObservation) {
			o.LastBuildHour = sim.IntegerValue(world.ReproductionHorizonHours)
		},
		"meal hour high": func(o *strategy.ReproductionObservation) {
			o.LastStoredMealHour = sim.IntegerValue(world.ReproductionHorizonHours)
		},
		"build hour future": func(o *strategy.ReproductionObservation) { o.LastBuildHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"meal hour future":  func(o *strategy.ReproductionObservation) { o.LastStoredMealHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"source wrong kind": func(o *strategy.ReproductionObservation) { o.BagSource = sim.IntegerValue(1001) },
		"source invalid patch": func(o *strategy.ReproductionObservation) {
			o.BagSource, _ = sim.EntityRefValue(1003)
		},
		"founder foreign patch": func(o *strategy.ReproductionObservation) {
			o.BagSource, _ = sim.EntityRefValue(1002)
		},
		"held source missing": func(o *strategy.ReproductionObservation) {
			o.BagUnits = sim.IntegerValue(1)
			absent, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
			o.BagSource = absent // the v3 empty-bag shape is not admissible in v4
		},
	}
	for name, mutate := range mutators {
		obs := reproductionObservation(1, ref, 5)
		mutate(&obs)
		choice, err := b.Evaluate(obs)
		if err != nil || choice.Kind != strategy.ReproductionWait || choice.Fallback != strategy.ReproductionInvalidSelf || choice.Evaluated != 0 {
			t.Fatalf("%s: %+v %v", name, choice, err)
		}
	}
	// An exact scalar projection of an integer is acceptable.
	obs := reproductionObservation(1, ref, 5)
	obs.Energy, _ = sim.ScalarValue(5)
	home, _ := world.ReproductionFounderPatchID(1)
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(home)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.ReproductionBuild {
		t.Fatalf("exact scalar energy accepted: %+v %v", choice, err)
	}
	// Structural errors fire before the cascade: hours outside the frozen
	// horizon, including the basal-only h=168 boundary.
	for _, hour := range []int{-1, world.ReproductionHorizonHours} {
		obs := reproductionObservation(1, ref, hour)
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
			t.Fatalf("hour %d accepted: %v", hour, err)
		}
	}
}

// Privacy discipline: every observation and view type can carry only the
// frozen actor-visible list — no field exists through which another actor's
// state, any death-hour (including the actor's own), a debt accumulator, a
// stream, or claim-vs-truth divergence could arrive — and a bound never
// evaluates another actor's observation.
func TestReproductionObservationPrivacyFieldSets(t *testing.T) {
	want := map[reflect.Type][]string{
		reflect.TypeOf(strategy.ReproductionObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Hunger", "BagUnits",
			"BagSource", "Capital", "Wip", "GranaryStock", "InvestedUnits", "LastBuildHour", "LastStoredMealHour", "LastDenial", "HomeSlots"},
		reflect.TypeOf(strategy.ReproductionRequestObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "HomePatch", "Energy", "Capital",
			"GranaryStock", "OwnRequest", "PatchMates", "TestClaims"},
		reflect.TypeOf(strategy.ReproductionReplyObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Capital", "GranaryStock",
			"OwnLastReplyHour", "Pending"},
		reflect.TypeOf(strategy.ReproductionPendingView{}): {"Requester", "Addressee", "Hour", "ReportedCapital", "ReportedGranary",
			"KinDistance", "KinRelated"},
		reflect.TypeOf(strategy.ReproductionRequestGuard{}):  {"Status", "LastRequestHour"},
		reflect.TypeOf(strategy.ReproductionClaimOverride{}): {"Capital", "Granary"},
	}
	for typ, fields := range want {
		got := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		if !reflect.DeepEqual(got, fields) {
			t.Fatalf("%s field set drifted: %v", typ, got)
		}
		for _, forbidden := range []string{"RequesterEnergy", "RequesterHunger", "RequesterBag", "RequesterLocus", "RequesterCapital",
			"RequesterGranary", "RequesterWorksite", "RequesterRequest", "RequesterTruth", "DeathHour", "DiedHour", "BasalDebt",
			"YieldDebt", "WearDebt", "Stream", "ClaimDelta", "ClaimMatchesTruth"} {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("%s leaked %s", typ, forbidden)
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, word := range []string{"Death", "Died", "Debt", "Stream"} {
				if strings.Contains(name, word) {
					t.Fatalf("%s field %s carries privileged %s data", typ, name, word)
				}
			}
		}
	}
	b1, ref := reproductionFixture(t, 1)
	b2, _ := reproductionFixture(t, 2)
	if _, err := b2.Evaluate(reproductionObservation(1, ref, 5)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("a bound evaluated another actor's claim view")
	}
	if _, err := b2.EvaluateBirthRequest(reproductionRequestView(1, ref, 5)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("a bound evaluated another actor's request view")
	}
	if _, err := b2.EvaluateBirthReply(reproductionReplyView(1, 3, ref, 5)); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatal("a bound evaluated another actor's reply view")
	}
	if b1.Policy() != strategy.FrozenReproductionPolicy() || b1.Binding() != (strategy.ReproductionBinding{Actor: 1, Ref: ref}) {
		t.Fatalf("accessors exposed non-frozen or foreign content: %+v %+v", b1.Policy(), b1.Binding())
	}
	mutated := b1.Policy()
	mutated.Ref.ID = "mutated"
	if b1.Policy() != strategy.FrozenReproductionPolicy() {
		t.Fatal("returned policy mutated bound content")
	}
	// Home-patch slot stocks are the only shared state in the claim view;
	// another patch's slots cannot enter it at all.
	foreign, err := world.ReproductionSlotID(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	obs := reproductionObservation(1, ref, 5)
	obs.HomeSlots[0].ID = foreign
	if _, err := b1.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidReproductionObservation) {
		t.Fatalf("foreign patch slot accepted: %v", err)
	}
}
