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

const naturalTestPatches = world.NaturalDefaultPatches // P=2: founders 1..16, genesis 2016

func naturalFixtureP(t *testing.T, patches int, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy()
	r := strategy.NewNaturalRegistry(patches)
	if r == nil {
		t.Fatal("nil registry for a valid patch count")
	}
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.NaturalBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p.Ref
}

func naturalFixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	return naturalFixtureP(t, naturalTestPatches, actor)
}

func naturalHomeSlots(t *testing.T, patches int, home sim.EntityID) []strategy.NaturalSlotView {
	t.Helper()
	slots := make([]strategy.NaturalSlotView, 0, world.NaturalSlotsPerPatch)
	for i := 0; i < world.NaturalSlotsPerPatch; i++ {
		slot, err := world.NaturalSlotID(patches, int(home-1001), i)
		if err != nil {
			t.Fatal(err)
		}
		slots = append(slots, strategy.NaturalSlotView{ID: slot, Stock: sim.IntegerValue(0)})
	}
	return slots
}

func naturalObservation(t *testing.T, actor sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalObservation {
	t.Helper()
	home, err := world.NaturalFounderPatchID(naturalTestPatches, actor)
	if err != nil {
		t.Fatal(err)
	}
	return naturalObservationAtHome(t, actor, home, ref, hour)
}

// naturalObservationAtHome builds a valid mid-run claim view at the frozen
// bounds defaults; newborns pass their bag-carried home patch.
func naturalObservationAtHome(t *testing.T, actor, home sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalObservation {
	t.Helper()
	source, err := sim.EntityRefValue(home)
	if err != nil {
		t.Fatal(err)
	}
	return strategy.NaturalObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(world.NaturalFounderInitialEnergy), Hunger: sim.IntegerValue(0),
		BagUnits: sim.IntegerValue(0), BagSource: source,
		Capital: sim.IntegerValue(0), Wip: sim.IntegerValue(0),
		GranaryStock: sim.IntegerValue(0), InvestedUnits: sim.IntegerValue(0),
		LastBuildHour: sim.IntegerValue(world.NaturalNeverHour), LastStoredMealHour: sim.IntegerValue(world.NaturalNeverHour),
		HomeSlots: naturalHomeSlots(t, naturalTestPatches, home),
	}
}

func naturalStocked(obs strategy.NaturalObservation, slotIndexes ...int) strategy.NaturalObservation {
	for _, i := range slotIndexes {
		obs.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	return obs
}

func naturalHeldAtMargins(t *testing.T, actor sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalObservation {
	t.Helper()
	obs := naturalObservation(t, actor, ref, hour)
	home, err := world.NaturalFounderPatchID(naturalTestPatches, actor)
	if err != nil {
		t.Fatal(err)
	}
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(home)
	return obs
}

// naturalRequestView builds a proposing founder that passes the frozen
// proposer condition (k>=2, granary>=3, energy>=3) with its same-patch
// founder roster.
func naturalRequestView(t *testing.T, actor sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalRequestObservation {
	t.Helper()
	home, err := world.NaturalFounderPatchID(naturalTestPatches, actor)
	if err != nil {
		t.Fatal(err)
	}
	var mates []sim.EntityID
	for id := sim.EntityID(1); id <= sim.EntityID(world.NaturalSlotsPerPatch*naturalTestPatches); id++ {
		if id == actor {
			continue
		}
		mateHome, err := world.NaturalFounderPatchID(naturalTestPatches, id)
		if err != nil {
			t.Fatal(err)
		}
		if mateHome == home {
			mates = append(mates, id)
		}
	}
	return strategy.NaturalRequestObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9, HomePatch: home,
		Energy: sim.IntegerValue(8), Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		OwnRequest: strategy.NaturalRequestGuard{Status: strategy.NaturalRequestIdle, LastRequestHour: -1},
		PatchMates: mates,
	}
}

// naturalReplyView builds a consenter whose own state sits exactly at the
// frozen consent margins (granary 3, k 2) with energy to spare, and one
// unrelated pending request passing every reported-claim margin.
func naturalReplyView(consenter, requester sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalReplyObservation {
	return strategy.NaturalReplyObservation{
		Actor: consenter, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(8), Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		OwnLastReplyHour: -1,
		Pending: []strategy.NaturalPendingView{{
			Requester: requester, Addressee: consenter, Hour: hour,
			ReportedCapital: 2, ReportedGranary: 3, KinDistance: 0, KinRelated: false,
		}},
	}
}

// naturalWorldStateAtMargins builds a valid world-contract founder state
// sitting above the consent margins: granary stock 3, capital 2, energy 8,
// with the extended G2a/G2b/G3 identities intact.
func naturalWorldStateAtMargins(t *testing.T, actor sim.EntityID) world.NaturalActorState {
	t.Helper()
	a, err := world.NaturalFounderState(naturalTestPatches, actor, world.NaturalFounderLifetimeMax)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = world.NaturalWorksiteState{Capital: 2, PointsCreated: 2, InvestedUnits: 4, LastBuildHour: 1}
	a.Granary = world.NaturalGranaryState{Stock: 3, CultivatedTotal: 3, LastStoredMealHour: world.NaturalNeverHour}
	a.Body.Energy = 8
	a.Body.BasalSpent = 3 // G3: 11 initial = 8 energy + 3 basal spent
	if !world.NaturalSurplusGateAllows(a, a, true) {
		t.Fatalf("margin state for actor %d must pass the world surplus gate", actor)
	}
	return a
}

func TestNaturalFrozenPolicyRegistration(t *testing.T) {
	p := strategy.FrozenNaturalPolicy()
	if p != (strategy.NaturalPolicy{FormatVersion: 5, Ref: strategy.NaturalRef{ID: "natural", Version: 5},
		Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen natural policy drifted: %+v", p)
	}
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for _, invalid := range []strategy.NaturalPolicy{
		func() strategy.NaturalPolicy { c := p; c.FormatVersion = 4; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Ref.ID = ""; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Ref.ID = "natural-v5"; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Ref.ID = longID; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Ref.Version = 0; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Ref.Version = 6; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Budget.Candidates = 0; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Budget.Candidates = 33; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Budget.Evaluations = 0; return c }(),
		func() strategy.NaturalPolicy { c := p; c.Budget.Evaluations = 33; return c }(),
	} {
		if err := r.Register(invalid); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("invalid natural policy accepted: %+v %v", invalid, err)
		}
	}
	if err := r.Register(p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("duplicate reference accepted: %v", err)
	}
	if _, err := r.Bind(strategy.NaturalBinding{Actor: 1, Ref: strategy.NaturalRef{ID: p.Ref.ID, Version: 6}}); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
		t.Fatalf("unregistered version bound: %v", err)
	}
	if _, err := r.Bind(strategy.NaturalBinding{Actor: 1, Ref: strategy.NaturalRef{ID: "", Version: 5}}); !errors.Is(err, strategy.ErrInvalidNaturalBinding) {
		t.Fatalf("empty ref bound: %v", err)
	}
	if strategy.NewNaturalRegistry(0) != nil || strategy.NewNaturalRegistry(-1) != nil {
		t.Fatal("a degenerate patch count must yield no registry")
	}
	var nilRegistry *strategy.NaturalRegistry
	if err := nilRegistry.Register(p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil registry registered")
	}
	if _, err := nilRegistry.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref}); !errors.Is(err, strategy.ErrInvalidNaturalBinding) {
		t.Fatal("nil registry bound")
	}
	b, err := r.Bind(strategy.NaturalBinding{Actor: 16, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	mutated := b.Policy()
	mutated.Budget.Candidates = 1
	if b.Policy() != p || b.Binding() != (strategy.NaturalBinding{Actor: 16, Ref: p.Ref}) {
		t.Fatal("bound policy is not value-only")
	}
	other, err := r.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
	if err != nil || other.Policy() != p {
		t.Fatalf("registry content changed: %+v %v", other.Policy(), err)
	}
	var nilBound *strategy.NaturalBound
	if nilBound.Binding() != (strategy.NaturalBinding{}) || nilBound.Policy() != (strategy.NaturalPolicy{}) {
		t.Fatal("nil bound must degrade to zero values")
	}
	if _, err := nilBound.Evaluate(naturalObservation(t, 1, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("nil bound evaluated a claim view")
	}
	if _, err := nilBound.EvaluateBirthRequest(naturalRequestView(t, 1, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("nil bound evaluated a request view")
	}
	if _, err := nilBound.EvaluateBirthReply(naturalReplyView(1, 2, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("nil bound evaluated a reply view")
	}
}

// The binding roster is cap-free: founder IDs 1..8P and ANY allocator-issued
// newborn ID bind the frozen ref — a newborn just above the allocator genesis
// and a synthetic high-ordinal ID alike — while every fixed-block ID
// (patches, slots, the unassigned stretch, zero, and the MaxUint64
// allocator-failure sentinel) is rejected.
func TestNaturalCapFreeBindingRoster(t *testing.T) {
	for _, patches := range []int{1, 2, 32} {
		p := strategy.FrozenNaturalPolicy()
		r := strategy.NewNaturalRegistry(patches)
		if err := r.Register(p); err != nil {
			t.Fatal(err)
		}
		founders, err := world.NaturalFounderCount(patches)
		if err != nil {
			t.Fatal(err)
		}
		genesis, err := world.NaturalHighestFixedEntityID(patches)
		if err != nil {
			t.Fatal(err)
		}
		firstNewborn, err := world.NaturalNextNewbornID(func() *sim.EntityAllocator {
			a, err := world.NaturalGenesisAllocator(patches)
			if err != nil {
				t.Fatal(err)
			}
			return &a
		}())
		if err != nil || firstNewborn != genesis+1 {
			t.Fatalf("allocator genesis for P=%d: newborn %d, highest fixed %d", patches, firstNewborn, genesis)
		}
		for actor := sim.EntityID(1); actor <= sim.EntityID(founders); actor++ {
			if _, err := r.Bind(strategy.NaturalBinding{Actor: actor, Ref: p.Ref}); err != nil {
				t.Fatalf("P=%d founder %d binding: %v", patches, actor, err)
			}
		}
		// A newborn just above the allocator genesis and a synthetic
		// high-ordinal ID both bind; the fixed blocks and the sentinel do not.
		for _, tc := range []struct {
			actor sim.EntityID
			ok    bool
		}{
			{firstNewborn, true},
			{genesis, false},    // the highest fixed slot is not an actor
			{genesis + 2, true}, // the next allocator draw
			{5000, true},        // synthetic high ordinal, far past every fixed block
			{sim.EntityID(^uint64(0) - 1), true},
			{sim.EntityID(^uint64(0)), false}, // allocator typed-failure sentinel
			{0, false},
		} {
			_, err := r.Bind(strategy.NaturalBinding{Actor: tc.actor, Ref: p.Ref})
			if tc.ok && err != nil {
				t.Fatalf("P=%d newborn %d must bind: %v", patches, tc.actor, err)
			}
			if !tc.ok && !errors.Is(err, strategy.ErrInvalidNaturalBinding) {
				t.Fatalf("P=%d fixed-block/out-of-space ID %d accepted: %v", patches, tc.actor, err)
			}
		}
		for _, fixed := range []sim.EntityID{
			sim.EntityID(17), // the unassigned stretch between founders and patches (P<=125)
			sim.EntityID(world.NaturalPatchIDBase),
			sim.EntityID(world.NaturalPatchIDBase + patches - 1), // last patch ID
			sim.EntityID(world.NaturalSlotIDBase),                // first slot ID
			genesis,                                              // last slot ID
		} {
			if fixed >= 1 && fixed <= sim.EntityID(founders) {
				continue // founder collision impossible for the tested P (8P <= 1000)
			}
			if _, err := r.Bind(strategy.NaturalBinding{Actor: fixed, Ref: p.Ref}); !errors.Is(err, strategy.ErrInvalidNaturalBinding) {
				t.Fatalf("P=%d fixed-block ID %d bound: %v", patches, fixed, err)
			}
		}
	}
	// A newborn binds a fresh policy instance, never an inherited binding.
	b, ref := naturalFixture(t, 2017)
	if b.Binding() != (strategy.NaturalBinding{Actor: 2017, Ref: ref}) || b.Policy() != strategy.FrozenNaturalPolicy() {
		t.Fatalf("newborn binding: %+v %+v", b.Binding(), b.Policy())
	}
}

// The policy lane must stay identical to the trusted world contract: frozen
// identity, ID-space partition, consent margins, kin limits, horizon, phases,
// the ten-minute v1 rest window, and the slot geometry.
func TestNaturalLaneMatchesWorldContract(t *testing.T) {
	p := strategy.FrozenNaturalPolicy()
	if uint32(world.NaturalFormatVersion) != p.FormatVersion || uint32(world.NaturalSchemaVersion) != p.Ref.Version ||
		uint32(world.NaturalRuleVersion) != 5 {
		t.Fatalf("v5 identity drift: world %d/%d/%d", world.NaturalFormatVersion, world.NaturalSchemaVersion, world.NaturalRuleVersion)
	}
	if world.NaturalConsentMinGranary != 3 || world.NaturalConsentMinCapital != 2 || world.NaturalConsentMinEnergy != 3 ||
		world.NaturalKinProhibitedDepth != 2 || world.NaturalKinWalkLimit != 10 ||
		world.NaturalKMax != 8 || world.NaturalWipMax != 1 || world.NaturalBagCapacity != 1 ||
		world.NaturalEnergyCapacity != 12 || world.NaturalHungerCapacity != 24 ||
		world.NaturalSlotsPerPatch != 8 || world.NaturalHorizonHours != 168 ||
		world.NaturalGranaryStockMax != 8*168 || world.NaturalSpoilagePeriod != 24 {
		t.Fatal("frozen consent/gate/cascade thresholds changed in the world contract")
	}
	if strategy.NaturalRestDuration != sim.Duration(600*1e6) {
		t.Fatalf("rest window drifted from the ten-minute v1 discipline: %d", strategy.NaturalRestDuration)
	}
	if int(world.NaturalPhaseBirthRequest) != 6 || int(world.NaturalPhaseBirthReply) != 7 {
		t.Fatalf("birth phases must extend the untouched v3 phases 0-5: %d %d",
			world.NaturalPhaseBirthRequest, world.NaturalPhaseBirthReply)
	}
	for status, want := range map[strategy.NaturalRequestStatus]int64{
		strategy.NaturalRequestIdle:     int64(world.NaturalRequestIdle),
		strategy.NaturalRequestPending:  int64(world.NaturalRequestPending),
		strategy.NaturalRequestAccepted: int64(world.NaturalRequestAccepted),
		strategy.NaturalRequestRefused:  int64(world.NaturalRequestRefused),
		strategy.NaturalRequestExpired:  int64(world.NaturalRequestExpired),
	} {
		if int64(status) != want {
			t.Fatalf("request status %d diverges from the world row value %d", status, want)
		}
	}
	// The lane's range-partition predicate must agree with the world's
	// published ID arithmetic for every interesting region of the space.
	for _, patches := range []int{1, 2, 32} {
		founders, err := world.NaturalFounderCount(patches)
		if err != nil {
			t.Fatal(err)
		}
		genesis, err := world.NaturalHighestFixedEntityID(patches)
		if err != nil {
			t.Fatal(err)
		}
		r := strategy.NewNaturalRegistry(patches)
		if err := r.Register(strategy.FrozenNaturalPolicy()); err != nil {
			t.Fatal(err)
		}
		sweep := []sim.EntityID{0, 1, sim.EntityID(founders), sim.EntityID(founders + 1), 999, 1000,
			sim.EntityID(world.NaturalPatchIDBase), sim.EntityID(world.NaturalPatchIDBase + patches - 1),
			sim.EntityID(world.NaturalPatchIDBase + patches), 2000, sim.EntityID(world.NaturalSlotIDBase),
			genesis - 1, genesis, genesis + 1, genesis + 2, 5000, 1 << 40, sim.EntityID(^uint64(0) - 1), sim.EntityID(^uint64(0))}
		for _, id := range sweep {
			wantFounder := id >= 1 && id <= sim.EntityID(founders)
			wantNewborn := id > genesis && id != sim.EntityID(^uint64(0))
			_, err := r.Bind(strategy.NaturalBinding{Actor: id, Ref: strategy.FrozenNaturalPolicy().Ref})
			if got := err == nil; got != (wantFounder || wantNewborn) {
				t.Fatalf("P=%d ID %d: lane binding=%v disagrees with the world partition (founder=%v newborn=%v)",
					patches, id, got, wantFounder, wantNewborn)
			}
		}
	}
	// Every home-patch slot the world names is gatherable by a home actor's
	// view and by nobody else; the admissible set is exactly the home patch.
	for actor := sim.EntityID(1); actor <= 16; actor++ {
		home, err := world.NaturalFounderPatchID(naturalTestPatches, actor)
		if err != nil {
			t.Fatal(err)
		}
		bound, ref := naturalFixture(t, actor)
		for i := 0; i < world.NaturalSlotsPerPatch; i++ {
			slot, err := world.NaturalSlotID(naturalTestPatches, int(home-1001), i)
			if err != nil {
				t.Fatal(err)
			}
			obs := naturalStocked(naturalObservation(t, actor, ref, 0), i)
			choice, err := bound.Evaluate(obs)
			if err != nil || choice.Kind != strategy.NaturalGather || choice.TargetSlot != slot || choice.Evaluated != 1 {
				t.Fatalf("actor %d slot %d: %+v %v", actor, slot, choice, err)
			}
		}
		if _, err := bound.Evaluate(naturalObservation(t, actor, ref, world.NaturalHorizonHours-1)); err != nil {
			t.Fatalf("final claimable hour %d rejected: %v", world.NaturalHorizonHours-1, err)
		}
		if _, err := bound.Evaluate(naturalObservation(t, actor, ref, world.NaturalHorizonHours)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("hour %d (basal only) accepted: %v", world.NaturalHorizonHours, err)
		}
	}
	// A newborn's placement is its bag-carried home patch, not its ID; the
	// same slot geometry applies.
	bound, ref := naturalFixture(t, 2017)
	obs := naturalStocked(naturalObservationAtHome(t, 2017, 1002, ref, 3), 4)
	slot, err := world.NaturalSlotID(naturalTestPatches, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if choice, err := bound.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalGather || choice.TargetSlot != slot {
		t.Fatalf("newborn claim on its carried home patch: %+v %v", choice, err)
	}
}

func TestNaturalClaimCascade(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	// Visible stock wins over the granary, ties break on the lowest slot ID,
	// and the whole candidate set is evaluated (all-or-nothing budget).
	obs := naturalStocked(naturalObservation(t, 1, ref, 5), 2, 0, 5)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalGather, TargetSlot: 2001,
		ObservedVersion: 9, Ref: ref, Evaluated: 3}) {
		t.Fatalf("gather must take the lowest stocked home slot: %+v %v", choice, err)
	}
	// Input order never matters: the same view shuffled picks the same slot.
	shuffled := naturalStocked(naturalObservation(t, 1, ref, 5), 5, 2, 0)
	shuffled.HomeSlots[3], shuffled.HomeSlots[6] = shuffled.HomeSlots[6], shuffled.HomeSlots[3]
	if choice, err := b.Evaluate(shuffled); err != nil || choice.TargetSlot != 2001 || choice.Evaluated != 3 {
		t.Fatalf("shuffled slots must not change the choice: %+v %v", choice, err)
	}
	// No visible stock, granary holds a unit: the only granary draw.
	obs = naturalObservation(t, 1, ref, 5)
	obs.GranaryStock = sim.IntegerValue(1)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("empty wild and stocked granary must draw a stored meal: %+v %v", choice, err)
	}
	// Nothing anywhere: Wait is a fallback with no evaluation.
	choice, err = b.Evaluate(naturalObservation(t, 1, ref, 5))
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalWait,
		ObservedVersion: 9, Ref: ref, Fallback: strategy.NaturalNoCandidate}) {
		t.Fatalf("nothing available must wait: %+v %v", choice, err)
	}
	// Gather priority over stored meals when both are visible.
	obs = naturalStocked(naturalObservation(t, 1, ref, 5), 7)
	obs.GranaryStock = sim.IntegerValue(1344) // the horizon envelope is reachable own state
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalGather || choice.TargetSlot != 2008 {
		t.Fatalf("wild gather precedes stored meals: %+v %v", choice, err)
	}
	// A dead actor never acts.
	obs = naturalStocked(naturalObservation(t, 1, ref, 5), 0)
	obs.Energy = sim.IntegerValue(0)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalWait || choice.Evaluated != 0 || choice.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("zero-energy actor must not act: %+v %v", choice, err)
	}
}

// The frozen bag decision (h+20m+2µs), verbatim v3: Build iff (energy>=5 or
// granary>=1) and k+wip<8, else Eat — plus the v5 land leg: after a build
// completion was refused for lack of a free wild slot (LastDenial =
// NaturalBuildDenied, zero mutation, bag still held) the re-evaluation
// reports Build unavailable and eats the held unit, never a second Build.
func TestNaturalBagDecisionTLowBoundaryAndBuildDenial(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	base := naturalHeldAtMargins(t, 1, ref, 5)
	for _, tc := range []struct {
		name                       string
		energy, capital, wip, gran int64
		want                       strategy.NaturalAction
	}{
		{"one below the frozen floor eats", 4, 0, 0, 0, strategy.NaturalEat},
		{"at the frozen floor builds", 5, 0, 0, 0, strategy.NaturalBuild},
		{"granary fallback funds a build at energy 4", 4, 0, 0, 1, strategy.NaturalBuild},
		{"full energy builds", 12, 0, 0, 0, strategy.NaturalBuild},
		{"seven points plus wip is saturated", 12, 7, 1, 0, strategy.NaturalEat},
		{"eight points is saturated", 12, 8, 0, 0, strategy.NaturalEat},
		{"seven points without wip still builds", 12, 7, 0, 0, strategy.NaturalBuild},
		{"outstanding wip completes the second unit", 5, 0, 1, 0, strategy.NaturalBuild},
		{"low energy with stocked granary builds for the runner-paired meal", 4, 0, 0, 1344, strategy.NaturalBuild},
	} {
		obs := base
		obs.Energy = sim.IntegerValue(tc.energy)
		obs.Capital = sim.IntegerValue(tc.capital)
		obs.Wip = sim.IntegerValue(tc.wip)
		obs.GranaryStock = sim.IntegerValue(tc.gran)
		choice, err := b.Evaluate(obs)
		if err != nil || choice != (strategy.NaturalChoice{Kind: tc.want,
			ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		if choice.Kind == strategy.NaturalEatStored {
			t.Fatalf("%s: bag decision emitted a stored meal", tc.name)
		}
	}
	// Build unavailable: the world refused the completion (no free wild slot
	// with stock 0), the bag kept its unit, and the runner re-asks. The
	// policy eats instead of building, at every build-eligible margin.
	denied := base
	denied.LastDenial = strategy.NaturalBuildDenied
	for _, margins := range [][4]int64{{5, 0, 0, 0}, {12, 0, 0, 0}, {12, 7, 0, 0}, {5, 0, 1, 0}, {4, 0, 0, 1344}} {
		obs := denied
		obs.Energy = sim.IntegerValue(margins[0])
		obs.Capital = sim.IntegerValue(margins[1])
		obs.Wip = sim.IntegerValue(margins[2])
		obs.GranaryStock = sim.IntegerValue(margins[3])
		choice, err := b.Evaluate(obs)
		if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalEat,
			ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
			t.Fatalf("build denial at margins %v must eat, not build: %+v %v", margins, choice, err)
		}
	}
	// The refusal left the bag held: a BuildDenied view with an empty bag is
	// a structurally inconsistent runner observation.
	empty := naturalObservation(t, 1, ref, 5)
	empty.LastDenial = strategy.NaturalBuildDenied
	if _, err := b.Evaluate(empty); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("build denial without a held unit accepted: %v", err)
	}
	// A GatherDenied view with bag=1 stays inert on the frozen path order
	// (the bag decision precedes the denial branch).
	gatherDenied := base
	gatherDenied.LastDenial = strategy.NaturalGatherDenied
	if choice, err := b.Evaluate(gatherDenied); err != nil || choice.Kind != strategy.NaturalBuild {
		t.Fatalf("gather denial must not disturb the bag decision: %+v %v", choice, err)
	}
}

func TestNaturalDeniedGatherFallback(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	// Denied with a stocked granary: the fallback draw, never a re-gather.
	obs := naturalStocked(naturalObservation(t, 1, ref, 5), 0, 3)
	obs.GranaryStock = sim.IntegerValue(1)
	obs.LastDenial = strategy.NaturalGatherDenied
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer with a stocked granary must eat stored: %+v %v", choice, err)
	}
	// Denied with an empty granary: the frozen ten-minute rest.
	obs.GranaryStock = sim.IntegerValue(0)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalRest,
		RestDuration: strategy.NaturalRestDuration, ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer without stores must rest: %+v %v", choice, err)
	}
	// Without a denial the same view gathers again instead.
	obs.LastDenial = strategy.NaturalNoDenial
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalGather || choice.TargetSlot != 2001 {
		t.Fatalf("undecided claim must gather: %+v %v", choice, err)
	}
}

// At most one EatStored per actor-hour: every stored-meal emission requires
// LastStoredMealHour < the observed hour, so a re-evaluation at any later
// frozen slot of the same hour can never double-draw.
func TestNaturalOneStoredMealPerActorHour(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	const hour = 5
	obs := naturalObservation(t, 1, ref, hour)
	obs.GranaryStock = sim.IntegerValue(2)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalEatStored {
		t.Fatalf("claim draw: %+v %v", choice, err)
	}
	for _, gran := range []int64{0, 1, 2, 1344} {
		reobserved := obs
		reobserved.GranaryStock = sim.IntegerValue(gran)
		reobserved.LastStoredMealHour = sim.IntegerValue(hour)
		choice, err := b.Evaluate(reobserved)
		if err != nil || choice.Kind == strategy.NaturalEatStored {
			t.Fatalf("granary %d: second draw in hour %d: %+v %v", gran, hour, choice, err)
		}
		if choice.Kind != strategy.NaturalWait {
			t.Fatalf("granary %d: spent-hour claim must wait, got %+v", gran, choice)
		}
	}
	denied := obs
	denied.LastDenial = strategy.NaturalGatherDenied
	denied.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(denied); err != nil || choice.Kind != strategy.NaturalRest || choice.RestDuration != strategy.NaturalRestDuration {
		t.Fatalf("denied fallback must not double-draw: %+v %v", choice, err)
	}
	held := naturalHeldAtMargins(t, 1, ref, hour)
	held.GranaryStock = sim.IntegerValue(3)
	held.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.NaturalBuild {
		t.Fatalf("bag decision with granary fallback builds: %+v %v", choice, err)
	}
	held.Capital = sim.IntegerValue(8) // saturated worksite: eat the held unit itself
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.NaturalEat {
		t.Fatalf("saturated bag decision eats the held unit itself: %+v %v", choice, err)
	}
	next := obs
	next.Hour = hour + 1
	if choice, err := b.Evaluate(next); err != nil || choice.Kind != strategy.NaturalEatStored {
		t.Fatalf("hour %d draw after hour %d meal: %+v %v", hour+1, hour, choice, err)
	}
}

func TestNaturalBudgetExhaustion(t *testing.T) {
	limited := strategy.NewNaturalRegistry(naturalTestPatches)
	p := strategy.FrozenNaturalPolicy()
	p.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	if err := limited.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := limited.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	// Two stocked slots against a one-slot budget: the whole candidate set is
	// declined, never a sorted prefix.
	obs := naturalStocked(naturalObservation(t, 1, p.Ref, 5), 0, 1)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalWait, ObservedVersion: 9,
		Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing claim budget: %+v %v", choice, err)
	}
	if choice, err := b.Evaluate(naturalStocked(naturalObservation(t, 1, p.Ref, 5), 4)); err != nil ||
		choice.Kind != strategy.NaturalGather || choice.TargetSlot != 2005 || choice.Evaluated != 1 {
		t.Fatalf("bounded candidate set: %+v %v", choice, err)
	}
	// Two same-patch mates against a one-mate request budget: no filing.
	request := naturalRequestView(t, 1, p.Ref, 5)
	choice2, err := b.EvaluateBirthRequest(request)
	if err != nil || choice2 != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing request budget: %+v %v", choice2, err)
	}
	request.PatchMates = []sim.EntityID{2}
	if choice, err := b.EvaluateBirthRequest(request); err != nil || choice.Kind != strategy.NaturalRequestFile || choice.Addressee != 2 {
		t.Fatalf("bounded request set: %+v %v", choice, err)
	}
	// Two pending requests against a one-reply budget: no reply at all.
	reply := naturalReplyView(1, 2, p.Ref, 5)
	reply.Pending = append(reply.Pending, strategy.NaturalPendingView{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	choice3, err := b.EvaluateBirthReply(reply)
	if err != nil || choice3 != (strategy.NaturalReplyChoice{Kind: strategy.NaturalReplyNone,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing reply budget: %+v %v", choice3, err)
	}
	// The frozen 32/32 budget covers every maximal legal candidate set: all
	// eight home slots, a 32-mate courtship roster, and a 32-pending list
	// evaluate without exhaustion — and a 33-mate roster is a typed budget
	// event, never a structural error (the roster is cap-free).
	fb, ref := naturalFixture(t, 1)
	all := naturalObservation(t, 1, ref, 5)
	for i := range all.HomeSlots {
		all.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	if choice, err := fb.Evaluate(all); err != nil || choice.Kind != strategy.NaturalGather ||
		choice.TargetSlot != 2001 || choice.Evaluated != 8 || choice.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen budget must never truncate the legal slot set: %+v %v", choice, err)
	}
	big := naturalRequestView(t, 1, ref, 5) // 7 same-patch founders
	for id := sim.EntityID(2017); len(big.PatchMates) < 32; id++ {
		big.PatchMates = append(big.PatchMates, id) // allocator-issued newborns join the patch roster
	}
	if choice, err := fb.EvaluateBirthRequest(big); err != nil || choice.Kind != strategy.NaturalRequestFile ||
		choice.Addressee != 2 || choice.Evaluated != 32 || choice.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen budget must never truncate the legal roster: %+v %v", choice, err)
	}
	big.PatchMates = append(big.PatchMates, 2049) // 33 mates: past the declared lane budget
	reqChoice, err := fb.EvaluateBirthRequest(big)
	if err != nil || reqChoice != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
		ObservedVersion: 9, Ref: ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("over-budget cap-free roster must be a typed budget event: %+v %v", reqChoice, err)
	}
	full := naturalReplyView(1, 2, ref, 5)
	for id := sim.EntityID(3); id <= 8; id++ {
		full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	}
	for id := sim.EntityID(2017); len(full.Pending) < 32; id++ {
		full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	}
	if choice, err := fb.EvaluateBirthReply(full); err != nil || choice.Kind != strategy.NaturalReplyAccept ||
		choice.Requester != 2 || choice.Evaluated != 1 || choice.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen budget must never truncate the legal pending set: %+v %v", choice, err)
	}
}

// The frozen proposer decision at h+40m+5µs: living, free request slot,
// one request per actor-hour, own state at the surplus-gate margins, and a
// deterministic same-patch addressee carrying the actor's reported claims.
func TestNaturalBirthRequestGatesDeterministicAddresseeAndClaims(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	obs := naturalRequestView(t, 1, ref, 5)
	choice, err := b.EvaluateBirthRequest(obs)
	if err != nil || choice != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestFile, Addressee: 2,
		ReportedCapital: 2, ReportedGranary: 3, ObservedVersion: 9, Ref: ref, Evaluated: 7, Fallback: strategy.NaturalNoFallback}) {
		t.Fatalf("eligible proposer must file: %+v %v", choice, err)
	}
	// The directed successor: actor 8 wraps to the lowest same-patch ID.
	b8, ref8 := naturalFixture(t, 8)
	if choice, err := b8.EvaluateBirthRequest(naturalRequestView(t, 8, ref8, 5)); err != nil ||
		choice.Addressee != 1 || choice.Kind != strategy.NaturalRequestFile {
		t.Fatalf("wraparound addressee: %+v %v", choice, err)
	}
	// Newborns join the same-patch roster unbounded: an allocator-issued
	// newborn mate files normally and never counts as malformed.
	withNewborn := naturalRequestView(t, 1, ref, 5)
	withNewborn.PatchMates = append(withNewborn.PatchMates, 2017, 5000)
	if choice, err := b.EvaluateBirthRequest(withNewborn); err != nil || choice.Kind != strategy.NaturalRequestFile ||
		choice.Addressee != 2 || choice.Evaluated != 9 {
		t.Fatalf("newborn mates must file: %+v %v", choice, err)
	}
	// A newborn proposer proposes on its carried home patch.
	nb, nbRef := naturalFixture(t, 2017)
	newbornView := strategy.NaturalRequestObservation{
		Actor: 2017, Ref: nbRef, Hour: 5, WorldVersion: 9, HomePatch: 1002,
		Energy: sim.IntegerValue(8), Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		OwnRequest: strategy.NaturalRequestGuard{Status: strategy.NaturalRequestIdle, LastRequestHour: -1},
		PatchMates: []sim.EntityID{9, 10, 11, 5000},
	}
	if choice, err := nb.EvaluateBirthRequest(newbornView); err != nil || choice.Kind != strategy.NaturalRequestFile || choice.Addressee != 5000 {
		t.Fatalf("newborn proposer: %+v %v", choice, err) // successor above the newborn ID
	}
	// Input order never matters.
	shuffled := naturalRequestView(t, 1, ref, 5)
	sort.Slice(shuffled.PatchMates, func(i, j int) bool { return shuffled.PatchMates[i] > shuffled.PatchMates[j] })
	if choice, err := b.EvaluateBirthRequest(shuffled); err != nil || choice.Addressee != 2 || choice.Evaluated != 7 {
		t.Fatalf("shuffled roster must not change the choice: %+v %v", choice, err)
	}
	// Frozen proposer condition boundaries (the same margins the runner's
	// surplus gate re-checks on fresh truth).
	for _, tc := range []struct {
		name                     string
		energy, capital, granary int64
		want                     strategy.NaturalRequestAction
	}{
		{"capital one below the margin waits", 8, 1, 3, strategy.NaturalRequestWait},
		{"capital at the margin files", 8, 2, 3, strategy.NaturalRequestFile},
		{"granary one below the margin waits", 8, 2, 2, strategy.NaturalRequestWait},
		{"granary at the margin files", 8, 2, 3, strategy.NaturalRequestFile},
		{"energy one below the margin waits", 2, 2, 3, strategy.NaturalRequestWait},
		{"energy at the margin files", 3, 2, 3, strategy.NaturalRequestFile},
		{"a dead actor never proposes", 0, 8, 8, strategy.NaturalRequestWait},
	} {
		view := naturalRequestView(t, 1, ref, 5)
		view.Energy = sim.IntegerValue(tc.energy)
		view.Capital = sim.IntegerValue(tc.capital)
		view.GranaryStock = sim.IntegerValue(tc.granary)
		choice, err := b.EvaluateBirthRequest(view)
		if err != nil || choice.Kind != tc.want {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		if tc.want == strategy.NaturalRequestWait && choice.Fallback != strategy.NaturalNoCandidate {
			t.Fatalf("%s: condition miss must be a plain wait: %+v", tc.name, choice)
		}
	}
	// One request per actor-hour: a pending request blocks, and any state
	// that already requested this hour blocks; yesterday's expired request
	// frees the slot again.
	blocked := naturalRequestView(t, 1, ref, 5)
	for _, guard := range []strategy.NaturalRequestGuard{
		{Status: strategy.NaturalRequestPending, LastRequestHour: 4},
		{Status: strategy.NaturalRequestPending, LastRequestHour: 5},
		{Status: strategy.NaturalRequestAccepted, LastRequestHour: 5},
		{Status: strategy.NaturalRequestRefused, LastRequestHour: 5},
		{Status: strategy.NaturalRequestExpired, LastRequestHour: 5},
	} {
		blocked.OwnRequest = guard
		if choice, err := b.EvaluateBirthRequest(blocked); err != nil || choice.Kind != strategy.NaturalRequestWait {
			t.Fatalf("guard %+v must block: %+v %v", guard, choice, err)
		}
	}
	expired := naturalRequestView(t, 1, ref, 5)
	expired.OwnRequest = strategy.NaturalRequestGuard{Status: strategy.NaturalRequestExpired, LastRequestHour: 4}
	if choice, err := b.EvaluateBirthRequest(expired); err != nil || choice.Kind != strategy.NaturalRequestFile || choice.Addressee != 2 {
		t.Fatalf("expired request frees the slot: %+v %v", choice, err)
	}
	// The CLAIM is the actor's reported values: the override replaces the
	// reported claims but never the actor's own eligibility or addressee.
	forged := naturalRequestView(t, 1, ref, 5)
	forged.TestClaims = &strategy.NaturalClaimOverride{Capital: 0, Granary: 0}
	choice, err = b.EvaluateBirthRequest(forged)
	if err != nil || choice.Kind != strategy.NaturalRequestFile || choice.Addressee != 2 ||
		choice.ReportedCapital != 0 || choice.ReportedGranary != 0 || choice.Evaluated != 7 {
		t.Fatalf("override changes only the reported claims: %+v %v", choice, err)
	}
	if !reflect.DeepEqual(obs.Capital, sim.IntegerValue(2)) || !reflect.DeepEqual(obs.GranaryStock, sim.IntegerValue(3)) {
		t.Fatal("observation mutated by evaluation")
	}
	lowTruth := naturalRequestView(t, 1, ref, 5)
	lowTruth.Capital = sim.IntegerValue(1) // own truth below the proposer condition
	lowTruth.TestClaims = &strategy.NaturalClaimOverride{Capital: 8, Granary: 1344}
	if choice, err := b.EvaluateBirthRequest(lowTruth); err != nil || choice.Kind != strategy.NaturalRequestWait {
		t.Fatalf("a forged claim must not rescue own eligibility: %+v %v", choice, err)
	}
	// Structural malformations are errors, never choices.
	selfMate := naturalRequestView(t, 1, ref, 5)
	selfMate.PatchMates = []sim.EntityID{1, 2}
	dupMate := naturalRequestView(t, 1, ref, 5)
	dupMate.PatchMates = []sim.EntityID{2, 2}
	crossPatchMate := naturalRequestView(t, 1, ref, 5)
	crossPatchMate.PatchMates = []sim.EntityID{9} // patch 1002 actor in a patch-1001 roster
	foreignRosterID := naturalRequestView(t, 1, ref, 5)
	foreignRosterID.PatchMates = []sim.EntityID{17} // the unassigned stretch
	patchMate := naturalRequestView(t, 1, ref, 5)
	patchMate.PatchMates = []sim.EntityID{1001} // a patch ID is not an actor
	slotMate := naturalRequestView(t, 1, ref, 5)
	slotMate.PatchMates = []sim.EntityID{2001} // a slot ID is not an actor
	genesisMate := naturalRequestView(t, 1, ref, 5)
	genesisMate.PatchMates = []sim.EntityID{2016} // the highest fixed slot
	sentinelMate := naturalRequestView(t, 1, ref, 5)
	sentinelMate.PatchMates = []sim.EntityID{sim.EntityID(^uint64(0))}
	badStatus := naturalRequestView(t, 1, ref, 5)
	badStatus.OwnRequest.Status = 5
	futureGuard := naturalRequestView(t, 1, ref, 5)
	futureGuard.OwnRequest.LastRequestHour = 6
	idleWithHour := naturalRequestView(t, 1, ref, 5)
	idleWithHour.OwnRequest.LastRequestHour = 4
	fileWithNoHour := naturalRequestView(t, 1, ref, 5)
	fileWithNoHour.OwnRequest = strategy.NaturalRequestGuard{Status: strategy.NaturalRequestRefused, LastRequestHour: -1}
	wrongPatch := naturalRequestView(t, 1, ref, 5)
	wrongPatch.HomePatch = 1002
	badPatch := naturalRequestView(t, 1, ref, 5)
	badPatch.HomePatch = 1003
	highClaim := naturalRequestView(t, 1, ref, 5)
	highClaim.TestClaims = &strategy.NaturalClaimOverride{Capital: 9, Granary: 3}
	negativeClaim := naturalRequestView(t, 1, ref, 5)
	negativeClaim.TestClaims = &strategy.NaturalClaimOverride{Capital: 2, Granary: -1}
	envelopeClaim := naturalRequestView(t, 1, ref, 5)
	envelopeClaim.TestClaims = &strategy.NaturalClaimOverride{Capital: 2, Granary: 1345}
	actorMismatch := naturalRequestView(t, 1, ref, 5)
	actorMismatch.Actor = 2
	refMismatch := naturalRequestView(t, 1, ref, 5)
	refMismatch.Ref = strategy.NaturalRef{ID: "other", Version: 5}
	for name, view := range map[string]strategy.NaturalRequestObservation{
		"self mate": selfMate, "duplicate mate": dupMate, "cross-patch mate": crossPatchMate,
		"stretch mate": foreignRosterID, "patch mate": patchMate, "slot mate": slotMate,
		"genesis mate": genesisMate, "sentinel mate": sentinelMate, "unknown status": badStatus,
		"future guard": futureGuard, "idle with hour": idleWithHour, "filed without hour": fileWithNoHour,
		"wrong founder patch": wrongPatch, "invalid patch": badPatch,
		"claim capital high": highClaim, "claim granary negative": negativeClaim,
		"claim past the envelope": envelopeClaim,
		"actor mismatch":          actorMismatch, "ref mismatch": refMismatch,
	} {
		if _, err := b.EvaluateBirthRequest(view); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %+v %v", name, view, err)
		}
	}
	hourBound := naturalRequestView(t, 1, ref, world.NaturalHorizonHours)
	if _, err := b.EvaluateBirthRequest(hourBound); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("hour %d accepted: %v", world.NaturalHorizonHours, err)
	}
	// Out-of-range own state degrades to Wait + InvalidSelf.
	invalidSelf := naturalRequestView(t, 1, ref, 5)
	invalidSelf.Capital = sim.IntegerValue(9)
	choice, err = b.EvaluateBirthRequest(invalidSelf)
	if err != nil || choice.Kind != strategy.NaturalRequestWait || choice.Fallback != strategy.NaturalInvalidSelf || choice.Evaluated != 0 {
		t.Fatalf("invalid own capital: %+v %v", choice, err)
	}
	invalidSelf = naturalRequestView(t, 1, ref, 5)
	invalidSelf.GranaryStock = sim.IntegerValue(1345)
	if choice, err := b.EvaluateBirthRequest(invalidSelf); err != nil || choice.Fallback != strategy.NaturalInvalidSelf {
		t.Fatalf("own granary past the envelope: %+v %v", choice, err)
	}
	invalidSelf = naturalRequestView(t, 1, ref, 5)
	invalidSelf.Energy = sim.BoolValue(true)
	if choice, err := b.EvaluateBirthRequest(invalidSelf); err != nil || choice.Fallback != strategy.NaturalInvalidSelf {
		t.Fatalf("unknown own energy: %+v %v", choice, err)
	}
}

// The frozen consent formula as a truth table, each row cross-checked
// against the world contract's evaluator: own granary>=3 ∧ own k>=2 ∧ own
// energy>=3 ∧ reported k>=2 ∧ reported granary>=3 ∧ (kin-distance>=3 ∨
// unrelated).
func TestNaturalBirthReplyConsentTruthTable(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	for _, tc := range []struct {
		name                             string
		granary, capital, energy         int64
		reportedCapital, reportedGranary int64
		kinRelated                       bool
		kinDistance                      int
		want                             strategy.NaturalReplyAction
		reason                           strategy.NaturalReplyReason
	}{
		{"all margins met accepts", 3, 2, 3, 2, 3, false, 0, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{"spare state accepts", 8, 8, 8, 8, 8, false, 0, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{"envelope granary accepts", 1344, 2, 3, 2, 3, false, 0, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{"low reserve granary refuses", 2, 2, 3, 2, 3, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{"capital one below refuses", 3, 1, 3, 2, 3, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{"energy one below refuses", 3, 2, 2, 2, 3, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{"reported capital one below refuses", 3, 2, 3, 1, 3, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentClaims},
		{"reported granary one below refuses", 3, 2, 3, 2, 2, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentClaims},
		{"parent-child distance 1 prohibited", 3, 2, 3, 2, 3, true, 1, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonKin},
		{"sibling distance 2 prohibited", 3, 2, 3, 2, 3, true, 2, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonKin},
		{"cousin distance 3 allowed", 3, 2, 3, 2, 3, true, 3, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{"distant kin allowed", 3, 2, 3, 2, 3, true, 10, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{"own state precedes claims and kin", 2, 1, 2, 1, 2, true, 1, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{"claims precede kin", 3, 2, 3, 1, 2, true, 1, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentClaims},
	} {
		view := naturalReplyView(1, 2, ref, 5)
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
		kinAllowed := !world.NaturalKinProhibited(tc.kinDistance, tc.kinRelated)
		if allowed := world.NaturalConsentAllows(tc.granary, tc.capital, tc.energy, tc.reportedCapital, tc.reportedGranary, kinAllowed); allowed != (tc.want == strategy.NaturalReplyAccept) {
			t.Fatalf("%s: policy diverges from the world consent evaluator: world=%v", tc.name, allowed)
		}
	}
	// Exhaustive small-grid cross-check against the world evaluator.
	for _, granary := range []int64{2, 3, 8} {
		for _, capital := range []int64{1, 2, 8} {
			for _, energy := range []int64{2, 3, 8} {
				for _, reportedCapital := range []int64{1, 2} {
					for _, reportedGranary := range []int64{2, 3, 1344} {
						for _, kin := range []struct {
							related  bool
							distance int
						}{{false, 0}, {true, 1}, {true, 2}, {true, 3}, {true, 10}} {
							view := naturalReplyView(1, 2, ref, 5)
							view.GranaryStock = sim.IntegerValue(granary)
							view.Capital = sim.IntegerValue(capital)
							view.Energy = sim.IntegerValue(energy)
							view.Pending[0].ReportedCapital = reportedCapital
							view.Pending[0].ReportedGranary = reportedGranary
							view.Pending[0].KinRelated = kin.related
							view.Pending[0].KinDistance = kin.distance
							choice, err := b.EvaluateBirthReply(view)
							kinAllowed := !world.NaturalKinProhibited(kin.distance, kin.related)
							allowed := world.NaturalConsentAllows(granary, capital, energy, reportedCapital, reportedGranary, kinAllowed)
							if err != nil || (choice.Kind == strategy.NaturalReplyAccept) != allowed {
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
	// typed reason; input order never matters. Newborn requesters join the
	// ascending order by ID.
	multi := naturalReplyView(1, 3, ref, 5)
	multi.Pending = []strategy.NaturalPendingView{
		{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 1, ReportedGranary: 3}, // claims fail
		{Requester: 5, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3}, // acceptable
		{Requester: 2017, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3},
	}
	choice, err := b.EvaluateBirthReply(multi)
	if err != nil || choice.Kind != strategy.NaturalReplyAccept || choice.Requester != 5 || choice.Evaluated != 2 {
		t.Fatalf("first acceptable pending must win: %+v %v", choice, err)
	}
	reversed := multi
	reversed.Pending = []strategy.NaturalPendingView{multi.Pending[2], multi.Pending[1], multi.Pending[0]}
	if choice, err := b.EvaluateBirthReply(reversed); err != nil || choice.Requester != 5 || choice.Evaluated != 2 {
		t.Fatalf("input order must not change the choice: %+v %v", choice, err)
	}
	multi.Pending[1].ReportedCapital = 0 // claims fail
	multi.Pending[2].ReportedGranary = 0 // now nothing is acceptable
	choice, err = b.EvaluateBirthReply(multi)
	if err != nil || choice.Kind != strategy.NaturalReplyRefuse || choice.Requester != 3 ||
		choice.Reason != strategy.NaturalReplyReasonConsentClaims || choice.Evaluated != 3 {
		t.Fatalf("lowest-ID refusal when nothing is acceptable: %+v %v", choice, err)
	}
	// One reply per actor-hour: the ledger guard blocks, yesterday's reply
	// does not.
	replied := naturalReplyView(1, 2, ref, 5)
	replied.OwnLastReplyHour = 5
	if choice, err := b.EvaluateBirthReply(replied); err != nil || choice.Kind != strategy.NaturalReplyNone {
		t.Fatalf("second reply in one hour: %+v %v", choice, err)
	}
	replied.OwnLastReplyHour = 4
	if choice, err := b.EvaluateBirthReply(replied); err != nil || choice.Kind != strategy.NaturalReplyAccept {
		t.Fatalf("reply after yesterday's: %+v %v", choice, err)
	}
	// A dead consenter never replies; an empty addressed set waits.
	dead := naturalReplyView(1, 2, ref, 5)
	dead.Energy = sim.IntegerValue(0)
	if choice, err := b.EvaluateBirthReply(dead); err != nil || choice.Kind != strategy.NaturalReplyNone || choice.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("dead consenter: %+v %v", choice, err)
	}
	empty := naturalReplyView(1, 2, ref, 5)
	empty.Pending = nil
	if choice, err := b.EvaluateBirthReply(empty); err != nil || choice.Kind != strategy.NaturalReplyNone {
		t.Fatalf("no pending request: %+v %v", choice, err)
	}
	// Structural malformations are errors, never choices.
	mutators := map[string]func(*strategy.NaturalReplyObservation){
		"actor mismatch": func(o *strategy.NaturalReplyObservation) { o.Actor = 2 },
		"ref mismatch": func(o *strategy.NaturalReplyObservation) {
			o.Ref = strategy.NaturalRef{ID: "other", Version: 5}
		},
		"future reply hour":  func(o *strategy.NaturalReplyObservation) { o.OwnLastReplyHour = 6 },
		"requester self":     func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 1 },
		"requester stretch":  func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 17 },
		"requester patch":    func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 1001 },
		"requester slot":     func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 2016 },
		"requester sentinel": func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = sim.EntityID(^uint64(0)) },
		"requester zero":     func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 0 },
		"addressee other":    func(o *strategy.NaturalReplyObservation) { o.Pending[0].Addressee = 2 },
		"stale hour":         func(o *strategy.NaturalReplyObservation) { o.Pending[0].Hour = 4 },
		"claim high":         func(o *strategy.NaturalReplyObservation) { o.Pending[0].ReportedCapital = 9 },
		"claim negative":     func(o *strategy.NaturalReplyObservation) { o.Pending[0].ReportedGranary = -1 },
		"claim past envelope": func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].ReportedGranary = 1345
		},
		"related zero distance": func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 0
		},
		"distance beyond walk": func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 11
		},
		"unrelated with distance": func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = false, 1
		},
		"duplicate requester": func(o *strategy.NaturalReplyObservation) {
			o.Pending = append(o.Pending, o.Pending[0])
		},
	}
	for name, mutate := range mutators {
		view := naturalReplyView(1, 2, ref, 5)
		mutate(&view)
		if _, err := b.EvaluateBirthReply(view); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %+v %v", name, view, err)
		}
	}
	hourBound := naturalReplyView(1, 2, ref, world.NaturalHorizonHours)
	if _, err := b.EvaluateBirthReply(hourBound); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("hour %d accepted: %v", world.NaturalHorizonHours, err)
	}
	// Out-of-range own state degrades to None + InvalidSelf.
	invalidSelf := naturalReplyView(1, 2, ref, 5)
	invalidSelf.GranaryStock = sim.IntegerValue(1345)
	choice, err = b.EvaluateBirthReply(invalidSelf)
	if err != nil || choice.Kind != strategy.NaturalReplyNone || choice.Fallback != strategy.NaturalInvalidSelf || choice.Evaluated != 0 {
		t.Fatalf("invalid own granary: %+v %v", choice, err)
	}
}

// The claim is the actor's reported values, and the consenter decides on
// claims alone: two views identical except for the claims decide differently,
// and the reported claims land in the world's request row verbatim — truth
// never travels with them, and the world refuses the same claim failure on
// the same typed reason without the policy ever seeing truth.
func TestNaturalClaimReportingNeverLeaksTruth(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	forged := naturalRequestView(t, 1, ref, 5)
	forged.Capital = sim.IntegerValue(5) // own truth: k=5, granary=4
	forged.GranaryStock = sim.IntegerValue(4)
	forged.TestClaims = &strategy.NaturalClaimOverride{Capital: 2, Granary: 0}
	choice, err := b.EvaluateBirthRequest(forged)
	if err != nil || choice.Kind != strategy.NaturalRequestFile || choice.Addressee != 2 ||
		choice.ReportedCapital != 2 || choice.ReportedGranary != 0 {
		t.Fatalf("reported claims must be the choice's claims: %+v %v", choice, err)
	}
	// The world's request row carries exactly the reported values, never the
	// actor's truth: the frozen contract takes claims as given and never
	// consults them against truth at filing.
	filer := naturalWorldStateAtMargins(t, 1) // truth k=2, granary=3 differs from the claims
	filed, err := world.NaturalBirthRequest(naturalTestPatches, 5, 1, 2, choice.ReportedCapital, choice.ReportedGranary, filer)
	if err != nil {
		t.Fatal(err)
	}
	if filed.Request.ReportedCapital != 2 || filed.Request.ReportedGranary != 0 || filed.Request.Status != world.NaturalRequestPending {
		t.Fatalf("world request row must carry the reported claims: %+v", filed.Request)
	}
	// The consenter decides on the claims alone: identical own state, only
	// the reported values differ.
	donor, donorRef := naturalFixture(t, 2)
	lowClaim := naturalReplyView(2, 1, donorRef, 5)
	lowClaim.Pending[0].ReportedCapital, lowClaim.Pending[0].ReportedGranary = 2, 0
	lowDecision, err := donor.EvaluateBirthReply(lowClaim)
	if err != nil || lowDecision.Kind != strategy.NaturalReplyRefuse || lowDecision.Reason != strategy.NaturalReplyReasonConsentClaims {
		t.Fatalf("reported granary 0 must refuse on claims: %+v %v", lowDecision, err)
	}
	highClaim := lowClaim
	highClaim.Pending[0].ReportedGranary = 3
	highDecision, err := donor.EvaluateBirthReply(highClaim)
	if err != nil || highDecision.Kind != strategy.NaturalReplyAccept {
		t.Fatalf("reported granary 3 must accept on claims: %+v %v", highDecision, err)
	}
	// The world's reply evaluator refuses the same claim failure on the same
	// typed reason, independent of any truth the policy never saw.
	proposerState := naturalWorldStateAtMargins(t, 1)
	proposerState.Request = filed.Request
	consenterState := naturalWorldStateAtMargins(t, 2)
	allocator, err := world.NaturalGenesisAllocator(naturalTestPatches)
	if err != nil {
		t.Fatal(err)
	}
	newbornID, err := world.NaturalNextNewbornID(&allocator)
	if err != nil {
		t.Fatal(err)
	}
	result, err := world.NaturalBirthReply(naturalTestPatches, 5, true, make([]uint64, world.NaturalDrawsPerBirth), newbornID, 1, 2, proposerState, consenterState)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != world.NaturalReplyRefuse || result.Reason != world.NaturalRefusalConsentClaims {
		t.Fatalf("world must refuse the same claim failure: %+v", result)
	}
}

func TestNaturalClaimObservationValidation(t *testing.T) {
	b, ref := naturalFixture(t, 1)
	valid := naturalObservation(t, 1, ref, 5)
	if _, err := b.Evaluate(valid); err != nil {
		t.Fatal(err)
	}
	actorMismatch := valid
	actorMismatch.Actor = 2
	refMismatch := valid
	refMismatch.Ref = strategy.NaturalRef{ID: "other", Version: 5}
	badDenial := valid
	badDenial.LastDenial = 255
	foreign := valid
	foreign.HomeSlots = append([]strategy.NaturalSlotView(nil), valid.HomeSlots...)
	foreign.HomeSlots[0].ID = 2009 // patch 1002 slot inside actor 1's view
	duplicate := valid
	duplicate.HomeSlots[1].ID = duplicate.HomeSlots[0].ID
	nineSlots := valid
	nineSlots.HomeSlots = append(append([]strategy.NaturalSlotView(nil), valid.HomeSlots...), strategy.NaturalSlotView{ID: 2017, Stock: sim.IntegerValue(0)})
	badStock := valid
	badStock.HomeSlots[2].Stock = sim.IntegerValue(2)
	for name, obs := range map[string]strategy.NaturalObservation{
		"actor mismatch": actorMismatch, "ref mismatch": refMismatch, "unknown denial": badDenial,
		"foreign slot": foreign, "duplicate slot": duplicate, "nine slots": nineSlots, "slot stock 2": badStock,
	} {
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %+v %v", name, obs, err)
		}
	}
	// Out-of-range or non-integer own state degrades to Wait + InvalidSelf.
	mutators := map[string]func(*strategy.NaturalObservation){
		"energy high":  func(o *strategy.NaturalObservation) { o.Energy = sim.IntegerValue(13) },
		"energy low":   func(o *strategy.NaturalObservation) { o.Energy = sim.IntegerValue(-1) },
		"energy text":  func(o *strategy.NaturalObservation) { o.Energy = sim.BoolValue(true) },
		"hunger high":  func(o *strategy.NaturalObservation) { o.Hunger = sim.IntegerValue(25) },
		"bag high":     func(o *strategy.NaturalObservation) { o.BagUnits = sim.IntegerValue(2) },
		"bag low":      func(o *strategy.NaturalObservation) { o.BagUnits = sim.IntegerValue(-1) },
		"capital high": func(o *strategy.NaturalObservation) { o.Capital = sim.IntegerValue(9) },
		"wip high":     func(o *strategy.NaturalObservation) { o.Wip = sim.IntegerValue(2) },
		"granary high": func(o *strategy.NaturalObservation) { o.GranaryStock = sim.IntegerValue(1345) },
		"granary low":  func(o *strategy.NaturalObservation) { o.GranaryStock = sim.IntegerValue(-1) },
		"invested high": func(o *strategy.NaturalObservation) {
			o.InvestedUnits = sim.IntegerValue(world.NaturalHorizonHours + 1)
		},
		"build hour high": func(o *strategy.NaturalObservation) {
			o.LastBuildHour = sim.IntegerValue(world.NaturalHorizonHours)
		},
		"meal hour high": func(o *strategy.NaturalObservation) {
			o.LastStoredMealHour = sim.IntegerValue(world.NaturalHorizonHours)
		},
		"build hour future": func(o *strategy.NaturalObservation) { o.LastBuildHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"meal hour future":  func(o *strategy.NaturalObservation) { o.LastStoredMealHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"source wrong kind": func(o *strategy.NaturalObservation) { o.BagSource = sim.IntegerValue(1001) },
		"source invalid patch": func(o *strategy.NaturalObservation) {
			o.BagSource, _ = sim.EntityRefValue(1003)
		},
		"founder foreign patch": func(o *strategy.NaturalObservation) {
			o.BagSource, _ = sim.EntityRefValue(1002)
		},
		"held source missing": func(o *strategy.NaturalObservation) {
			o.BagUnits = sim.IntegerValue(1)
			absent, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
			o.BagSource = absent // the v3 empty-bag shape is not admissible in v5
		},
	}
	for name, mutate := range mutators {
		obs := naturalObservation(t, 1, ref, 5)
		mutate(&obs)
		choice, err := b.Evaluate(obs)
		if err != nil || choice.Kind != strategy.NaturalWait || choice.Fallback != strategy.NaturalInvalidSelf || choice.Evaluated != 0 {
			t.Fatalf("%s: %+v %v", name, choice, err)
		}
	}
	// The granary horizon envelope itself is valid own state.
	envelope := naturalObservation(t, 1, ref, 5)
	envelope.GranaryStock = sim.IntegerValue(world.NaturalGranaryStockMax)
	if choice, err := b.Evaluate(envelope); err != nil || choice.Kind != strategy.NaturalEatStored {
		t.Fatalf("envelope granary accepted: %+v %v", choice, err)
	}
	// An exact scalar projection of an integer is acceptable.
	obs := naturalObservation(t, 1, ref, 5)
	obs.Energy, _ = sim.ScalarValue(5)
	home, _ := world.NaturalFounderPatchID(naturalTestPatches, 1)
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(home)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.NaturalBuild {
		t.Fatalf("exact scalar energy accepted: %+v %v", choice, err)
	}
	// Structural errors fire before the cascade: hours outside the frozen
	// horizon, including the basal-only h=168 boundary.
	for _, hour := range []int{-1, world.NaturalHorizonHours} {
		obs := naturalObservation(t, 1, ref, hour)
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("hour %d accepted: %v", hour, err)
		}
	}
}

// Privacy discipline: every observation and view type can carry only the
// frozen actor-visible list — no field exists through which slot tenure (mode
// or owner), the assigned cultivation slot, a debt carry (basal/yield/wear/
// spoil), a spoilage counter, allocator state, R, a stream, another actor's
// state, any death-hour (including the actor's own), or claim-vs-truth
// divergence could arrive — and a bound never evaluates another actor's
// observation.
func TestNaturalObservationPrivacyFieldSets(t *testing.T) {
	want := map[reflect.Type][]string{
		reflect.TypeOf(strategy.NaturalObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Hunger", "BagUnits",
			"BagSource", "Capital", "Wip", "GranaryStock", "InvestedUnits", "LastBuildHour", "LastStoredMealHour", "LastDenial", "HomeSlots"},
		reflect.TypeOf(strategy.NaturalSlotView{}): {"ID", "Stock"},
		// Ticket 47 adds the PUBLIC kin walk to the proposer's observation
		// (NaturalMateView, one entry per PatchMates entry): the pin is
		// extended deliberately, never deleted.
		reflect.TypeOf(strategy.NaturalRequestObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "HomePatch", "Energy", "Capital",
			"GranaryStock", "OwnRequest", "PatchMates", "MateViews", "TestClaims"},
		reflect.TypeOf(strategy.NaturalMateView{}):      {"ID", "KinRelated", "KinDistance"},
		reflect.TypeOf(strategy.NaturalRequestGuard{}):  {"Status", "LastRequestHour"},
		reflect.TypeOf(strategy.NaturalClaimOverride{}): {"Capital", "Granary"},
		reflect.TypeOf(strategy.NaturalReplyObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Capital", "GranaryStock",
			"OwnLastReplyHour", "Pending"},
		reflect.TypeOf(strategy.NaturalPendingView{}): {"Requester", "Addressee", "Hour", "ReportedCapital", "ReportedGranary",
			"KinDistance", "KinRelated"},
		reflect.TypeOf(strategy.NaturalChoice{}): {"Kind", "TargetSlot", "RestDuration", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
		reflect.TypeOf(strategy.NaturalRequestChoice{}): {"Kind", "Addressee", "ReportedCapital", "ReportedGranary", "ObservedVersion",
			"Ref", "Evaluated", "Fallback"},
		reflect.TypeOf(strategy.NaturalReplyChoice{}): {"Kind", "Requester", "Reason", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
	}
	for typ, fields := range want {
		got := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		if !reflect.DeepEqual(got, fields) {
			t.Fatalf("%s field set drifted: %v", typ, got)
		}
		forbiddenNames := []string{"Tenure", "TenureMode", "TenureOwner", "Owner", "Mode", "Cultivated", "CultivatedSlot",
			"Debt", "BasalDebt", "YieldDebt", "WearDebt", "SpoilDebt", "Spoil", "Spoiled", "Allocator", "Seed", "R",
			"RegenBudget", "DeathHour", "DiedHour", "Stream", "ClaimDelta", "ClaimMatchesTruth"}
		for _, forbidden := range forbiddenNames {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("%s leaked %s", typ, forbidden)
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, word := range []string{"Tenure", "Owner", "Mode", "Debt", "Spoil", "Allocator", "Seed", "Death", "Died", "Stream"} {
				if strings.Contains(name, word) {
					t.Fatalf("%s field %s carries privileged %s data", typ, name, word)
				}
			}
		}
	}
	b1, ref := naturalFixture(t, 1)
	b2, _ := naturalFixture(t, 2)
	if _, err := b2.Evaluate(naturalObservation(t, 1, ref, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a bound evaluated another actor's claim view")
	}
	if _, err := b2.EvaluateBirthRequest(naturalRequestView(t, 1, ref, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a bound evaluated another actor's request view")
	}
	if _, err := b2.EvaluateBirthReply(naturalReplyView(1, 3, ref, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a bound evaluated another actor's reply view")
	}
	if b1.Policy() != strategy.FrozenNaturalPolicy() || b1.Binding() != (strategy.NaturalBinding{Actor: 1, Ref: ref}) {
		t.Fatalf("accessors exposed non-frozen or foreign content: %+v %+v", b1.Policy(), b1.Binding())
	}
	mutated := b1.Policy()
	mutated.Ref.ID = "mutated"
	if b1.Policy() != strategy.FrozenNaturalPolicy() {
		t.Fatal("returned policy mutated bound content")
	}
	// Home-patch slot stocks are the only shared state in the claim view;
	// another patch's slots cannot enter it at all.
	foreignSlot, err := world.NaturalSlotID(naturalTestPatches, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	obs := naturalObservation(t, 1, ref, 5)
	obs.HomeSlots[0].ID = foreignSlot
	if _, err := b1.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("foreign patch slot accepted: %v", err)
	}
}
