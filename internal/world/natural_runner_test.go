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

// The natural runner tests (ticket 15). Fast fixtures with small horizons:
// the full 168h matrix belongs to the experiment lane. The longest walk runs
// past the first founder age-death to cover the fallow window and the
// wear-driven slot revert; the birth, build-denial, and long-granary
// mechanics run on directed hybrid fixtures (a real kernel and scheduler
// over hand-seeded ledger-coherent worlds, driving the runner's own decision
// and pulse code) because the frozen founder geometry (8P founders, 8P
// slots) saturates every patch at k=1 within two hours, so births, build
// refusals, and granary rots are unreachable before the h96+ die-off.

// naturalHandHour is the derived v5 hand trajectory of actor 1 under the
// saturated cycle (R=8, P=2, seed 0, births off): all eight founders of each
// patch build two points' worth of work in h0-h1, tenure the whole patch at
// h1+40m (fold in ascending ID order), live on flat regen grants plus one
// stored meal per hour, mass-decay at h7 (wear debt 6 at locus 6), rebuild,
// and re-tenure at h9+40m. Values are {energy, capital, wip, granary stock,
// stored meals}.
var naturalHandHour = []struct{ energy, capital, wip, granary, stored int64 }{
	{11, 0, 0, 0, 0}, // h0: no basal before the first pulse; wild fill 8
	{10, 0, 1, 0, 0}, // h1: build 1 completed at h0+40m; refill 8
	{9, 1, 0, 1, 0},  // h2: k1 tenured; first flat grant; basal resumes
	{9, 1, 0, 1, 1},  // h3: claim eats the stored grant
	{9, 1, 0, 1, 2},
	{9, 1, 0, 1, 3},
	{9, 1, 0, 1, 4},
	{9, 0, 0, 1, 5}, // h7: grant first (regen precedes wear), then mass decay
	{9, 0, 0, 0, 6}, // h8: no grant at k0; wild refill; gather+build
	{8, 0, 1, 0, 6}, // h9: rebuild wip; no stored meal during the window
	{7, 1, 0, 1, 6}, // h10: re-tenured at h9+40m; grants resume
}

// naturalRunTo steps until the checkpoint of the given hour exists.
func naturalRunTo(t *testing.T, f *Natural, hour int) {
	t.Helper()
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil {
			t.Fatalf("hour %d step %d: %v", hour, f.steps, err)
		}
		if !ok {
			t.Fatalf("hour %d: run ended early at step %d", hour, f.steps)
		}
	}
}

// naturalAssertConservation re-verifies the extended frozen gates at every
// hourly checkpoint, the balance seam, and the allocator cross-check both
// ways: the runner's last-ID must satisfy conservation exactly, and any
// tampered last-ID must fail closed.
func naturalAssertConservation(t *testing.T, f *Natural) {
	t.Helper()
	for _, check := range f.Checkpoints() {
		b, err := NaturalCheckConservation(f.patches, check.Patches, check.Slots, check.Founders, check.Newborns, check.AllocatorLast)
		if err != nil {
			t.Fatalf("h%d conservation: %v", check.Hour, err)
		}
		if b != check.Balance {
			t.Fatalf("h%d balance drift: %+v != %+v", check.Hour, b, check.Balance)
		}
		for _, tampered := range []sim.EntityID{check.AllocatorLast - 1, check.AllocatorLast + 1} {
			if _, err := NaturalCheckConservation(f.patches, check.Patches, check.Slots, check.Founders, check.Newborns, tampered); !errors.Is(err, ErrNaturalContract) {
				t.Fatalf("h%d allocator cross-check accepted tampered last-ID %d: %v", check.Hour, tampered, err)
			}
		}
	}
}

// naturalAssertJournalClock pins every attempt to its frozen phase instant
// and every keyed attempt to its committed event (gate G4 linkage). The
// legitimate keyless key-carriers are the allocator-collision birth loser
// and the typed refused build.
func naturalAssertJournalClock(t *testing.T, f *Natural) {
	t.Helper()
	events := make(map[sim.EventID]kernel.Event)
	for _, ev := range f.k.Events() {
		events[ev.ID] = ev
	}
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			hour := int(a.Time / sim.SimTime(NaturalHour))
			if hour < 0 || hour >= NaturalHorizonHours {
				t.Fatalf("attempt outside horizon: %+v", a)
			}
			var want sim.SimTime
			switch a.Kind {
			case "gather", "eat-stored", "provision", "wait":
				want, _ = NaturalPhaseTime(hour, NaturalPhaseClaim)
			case "rest":
				want, _ = NaturalPhaseTime(hour, NaturalPhaseGathered)
			case "eat":
				want, _ = NaturalPhaseTime(hour, NaturalPhaseBagDecision)
			case "build":
				start, _ := NaturalPhaseTime(hour, NaturalPhaseBuildStart)
				want = NaturalBuildCompletion(start)
			case "build-denied-eat", "paired-meal", "fallback-eatstored":
				want, _ = NaturalPhaseTime(hour, NaturalPhasePairedMeal)
			case "birth-request":
				want, _ = NaturalPhaseTime(hour, NaturalPhaseBirthRequest)
			case "birth-reply":
				want, _ = NaturalPhaseTime(hour, NaturalPhaseBirthReply)
			default:
				t.Fatalf("unknown attempt kind %+v", a)
			}
			if a.Time != want {
				t.Fatalf("attempt off the frozen clock: %+v want %d", a, want)
			}
			if a.Key == "" {
				if a.EventID != 0 || a.Accepted || a.Lost != "" {
					t.Fatalf("uncommitted attempt carries commit evidence: %+v", a)
				}
				// Keyless key-carriers are legitimate only with their typed
				// evidence: a denied gather carries its rejection reason, a
				// refused build its note, a raced meal its no-meal note, a
				// refused provision its note.
				if a.Rejection == 0 && a.Refused == "" && a.NoMeal == "" && a.ProvisionRefused == "" && a.Kind != "wait" && a.Kind != "rest" && !a.BirthWake {
					t.Fatalf("keyless attempt without typed evidence: %+v", a)
				}
				continue
			}
			if a.Lost != "" {
				if a.Lost != naturalLostCollision || a.Accepted || a.EventID != 0 || a.Birth.Decision != NaturalReplyAccept {
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

// naturalRulesAt returns the committed rule IDs at one timestamp in commit
// order.
func naturalRulesAt(t *testing.T, f *Natural, at sim.SimTime) []sim.RuleID {
	t.Helper()
	var rules []sim.RuleID
	for _, ev := range f.k.Events() {
		if ev.Time == at {
			rules = append(rules, ev.Rule)
		}
	}
	return rules
}

// TestNaturalRunnerPulseOrderAndHandTrajectory pins the frozen pulse order
// (basal 504 -> regen-split 501 -> flat grants 508 -> spoilage 502 -> wear
// 503 at one pulse), the ascending-ID build-completion fold (all eight
// founders of a patch tenure the lowest-index free wild slots in ID order at
// h1+40m), and the derived v5 hand trajectory of actor 1 through the first
// saturation cycle, including the wear-driven decay at h7, the wild refill,
// and the re-tenure at h9+40m.
func TestNaturalRunnerPulseOrderAndHandTrajectory(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 10)
	naturalAssertConservation(t, f)
	naturalAssertJournalClock(t, f)

	// The h1+40m fold: every founder completes its first point at the same
	// frozen instant and claims the lowest-index free wild stock-0 slot of
	// its home patch in ascending-ID order.
	completion, err := NaturalPhaseTime(1, NaturalPhaseBuildStart)
	if err != nil {
		t.Fatal(err)
	}
	completion = NaturalBuildCompletion(completion)
	tenure := map[sim.EntityID]sim.EntityID{}
	builds := 0
	for _, ev := range f.k.Events() {
		if ev.Time != completion || ev.Rule != NaturalBuildRule {
			continue
		}
		builds++
		for _, d := range ev.Deltas {
			if d.Component == NaturalSlotTypeID && d.Field == NaturalSlotTenureOwnerField {
				owner, err := d.After.EntityRef()
				if err != nil {
					t.Fatal(err)
				}
				tenure[d.Entity] = owner
			}
		}
	}
	if builds != 16 || len(tenure) != 16 {
		t.Fatalf("h1+40m fold: %d build events, %d tenure writes, want 16/16", builds, len(tenure))
	}
	for i := 0; i < 16; i++ {
		slot, err := NaturalSlotID(2, i/NaturalSlotsPerPatch, i%NaturalSlotsPerPatch)
		if err != nil {
			t.Fatal(err)
		}
		if tenure[slot] != sim.EntityID(i+1) {
			t.Fatalf("fold claimed slot %d for actor %d, want actor %d", slot, tenure[slot], i+1)
		}
	}

	// The h2 pulse: one proposal per subject per stage. Event order within
	// one timestamp is the kernel's key order (basal < grant < regen <
	// spoil < wear); the frozen STAGE order itself is pinned semantically:
	// each h2 spoil proposal carries spoil-debt 1 — the same pulse's granted
	// unit already sits in the stock the debt accrues against — and the h7
	// hand row (grant before the mass decay, reverted slots not refilled
	// in-pulse) pins regen-before-wear.
	pulse2, err := NaturalHourTime(2)
	if err != nil {
		t.Fatal(err)
	}
	want := []sim.RuleID{}
	for i := 0; i < 16; i++ {
		want = append(want, NaturalBasalRule)
	}
	for i := 0; i < 16; i++ {
		want = append(want, NaturalGrantRule)
	}
	want = append(want, NaturalRegenSplitRule, NaturalRegenSplitRule)
	for i := 0; i < 16; i++ {
		want = append(want, NaturalSpoilageRule)
	}
	for i := 0; i < 16; i++ {
		want = append(want, NaturalWearRule)
	}
	got := naturalRulesAt(t, f, pulse2)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("h2 pulse rule order:\n got %v\nwant %v", got, want)
	}
	for _, ev := range f.k.Events() {
		if ev.Time != pulse2 || ev.Rule != NaturalSpoilageRule {
			continue
		}
		for _, d := range ev.Deltas {
			if d.Field == NaturalGranarySpoilDebtField {
				after, err := d.After.Integer()
				if err != nil || after != 1 {
					t.Fatalf("h2 spoil debt %v, want 1 (the granted unit must accrue in the same pulse)", d.After)
				}
			}
		}
	}

	// The derived hand trajectory through the first saturation cycle.
	checks := f.Checkpoints()
	for h, w := range naturalHandHour {
		a := checks[h].Founders[0]
		got := [5]int64{a.Body.Energy, a.Worksite.Capital, a.Worksite.Wip, a.Granary.Stock, a.Granary.StoredMeals}
		want := [5]int64{w.energy, w.capital, w.wip, w.granary, w.stored}
		if got != want {
			t.Fatalf("h%d actor1 hand: got %v want %v (full: %+v)", h, got, want, a)
		}
	}
	// Founder symmetry inside a patch: actor 2 rides the identical cycle.
	a1, a2 := checks[5].Founders[0], checks[5].Founders[1]
	if a1.Body.Energy != a2.Body.Energy || a1.Worksite != a2.Worksite || a1.Granary != a2.Granary {
		t.Fatalf("patch-mate asymmetry at h5:\n a1 %+v\n a2 %+v", a1, a2)
	}
	// The saturated cycle carries no surplus and no spillover: cap-lost and
	// spoilage stay zero through h10 while grants exactly feed the meals.
	b := checks[10].Balance
	if b.EnergyCapLost != 0 || b.Spoiled != 0 {
		t.Fatalf("saturated cycle leaked energy or stock: %+v", b)
	}
	if b.Granted != b.CultivatedTotal {
		t.Fatalf("grant seam broken: granted %d, cultivatedTotal %d", b.Granted, b.CultivatedTotal)
	}
}

// TestNaturalRunnerRegenSplitIdentitiesAndSaturatedEcology pins the per-hour
// regen-split identity on the real runner (cultivated + wild + unrealized ==
// R·P at every hour), the grant seam, and the land-gated ecology of the
// frozen founder geometry: with 8P founders on 8P slots the birth gate
// (k>=2, granary>=3) is unreachable while every slot is tenured, so the
// births-enabled saturated run files no request and commits no birth —
// exactly the predeclared "land the direct bottleneck of reproduction".
func TestNaturalRunnerRegenSplitIdentitiesAndSaturatedEcology(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 7, Workers: 2, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 12)
	naturalAssertConservation(t, f)
	naturalAssertJournalClock(t, f)
	checks := f.Checkpoints()
	for h := 1; h <= 12; h++ {
		m := checks[h].Metrics
		if m.RegenCultivated+m.RegenWild+m.RegenUnrealized != 8*2 {
			t.Fatalf("h%d regen split %d+%d+%d != R·P", h, m.RegenCultivated, m.RegenWild, m.RegenUnrealized)
		}
	}
	last := checks[12]
	if last.Balance.Births != 0 || last.Metrics.BirthRequests != 0 {
		t.Fatalf("saturated ecology produced births: %+v", last.Metrics)
	}
	for _, ev := range f.k.Events() {
		switch ev.Rule {
		case NaturalBirthRequestRule, NaturalBirthReplyRule, NaturalBirthRefuseRule, NaturalBirthExpireRule:
			t.Fatalf("birth event in the saturated phase: %+v", ev)
		case NaturalRegenSplitRule:
			if ev.Kind != kernel.KindComponentPatch {
				t.Fatalf("regen-split must never allocate: %+v", ev)
			}
		}
	}
}

// TestNaturalRunnerZeroRExtinction walks the R=0 cells: extinction by h11
// (eleven basal hours drain the founder energy), zero births in both birth
// modes, and an all-unrealized (empty) regen split at every hour.
func TestNaturalRunnerZeroRExtinction(t *testing.T) {
	for _, births := range []bool{false, true} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 0, Seed: 0, Workers: 1, Births: births})
		if err != nil {
			t.Fatal(err)
		}
		naturalRunTo(t, f, 12)
		naturalAssertConservation(t, f)
		naturalAssertJournalClock(t, f)
		checks := f.Checkpoints()
		for h := 1; h <= 12; h++ {
			m := checks[h].Metrics
			if m.RegenCultivated+m.RegenWild+m.RegenUnrealized != 0 {
				t.Fatalf("births=%v h%d nonempty regen split at R=0", births, h)
			}
		}
		if alive := checks[12].Metrics.Alive; alive != 0 {
			t.Fatalf("births=%v alive at h12: %d", births, alive)
		}
		if deaths := checks[12].Metrics.DeathsStarvation; deaths != 16 {
			t.Fatalf("births=%v starvation deaths %d, want 16", births, deaths)
		}
		if births := checks[12].Balance.Births; births != 0 {
			t.Fatalf("births=%v births at R=0: %d", births, births)
		}
		// Extinction is exactly at h11: the pulse of h11 drains the last
		// energy unit (11 founder energy, one basal hour per pulse from h1).
		if got := checks[10].Metrics.Alive; got != 16 {
			t.Fatalf("births=%v alive at h10: %d, want 16", births, got)
		}
		if got := checks[11].Metrics.Alive; got != 0 {
			t.Fatalf("births=%v alive at h11: %d, want 0", births, got)
		}
	}
}

// naturalHybrid assembles the runner's own decision machinery over a
// hand-seeded kernel: the frozen registry and policy, the genesis allocator,
// and the full roster bound, without the fixture's neutral founder state.
func naturalHybrid(t *testing.T, seeds []component.ComponentSeed, ceilings map[NaturalCeiling]int64) (*Natural, *kernel.Kernel) {
	t.Helper()
	reg, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	policy := strategy.FrozenNaturalPolicy()
	policies := strategy.NewNaturalRegistry(2)
	if err := policies.Register(policy); err != nil {
		t.Fatal(err)
	}
	ledger, err := newNaturalLedger(ceilings)
	if err != nil {
		t.Fatal(err)
	}
	thresholds := make(map[NaturalCeiling]uint64, len(ledger.thresholds))
	for ceiling, value := range ledger.thresholds {
		thresholds[ceiling] = value
	}
	allocator, err := NaturalGenesisAllocator(2)
	if err != nil {
		t.Fatal(err)
	}
	f := &Natural{registry: reg, policies: policies, ref: policy.Ref, founders: naturalFoundersResolved(2, 0), patches: 2, seed: 0, r: 8, births: true,
		bound: make(map[sim.EntityID]*strategy.NaturalBound), allocator: allocator, ledger: ledger, thresholds: thresholds,
		pulseLifecycle: make(map[sim.EntityID]scheduler.Lifecycle), current: make(map[sim.EntityID]strategy.NaturalAction),
		denied: make(map[sim.EntityID]int64), deniedBuild: make(map[sim.EntityID]int64), meals: make(map[sim.EntityID]NaturalMealIntent),
		birthWakes: make(map[sim.EntityID]int64), refusals: make(map[NaturalRefusalReason]int64)}
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	sched, err := scheduler.New(k, 1, f.evaluate)
	if err != nil {
		t.Fatal(err)
	}
	f.k, f.sched = k, sched
	for i := 1; i <= 16; i++ {
		id := sim.EntityID(i)
		bound, err := policies.Bind(strategy.NaturalBinding{Actor: id, Ref: policy.Ref})
		if err != nil {
			t.Fatal(err)
		}
		f.bound[id] = bound
		f.roster = append(f.roster, id)
		f.pulseLifecycle[id] = scheduler.Alive
	}
	return f, k
}

// naturalHybridWorld reads the hybrid world state and re-checks conservation
// with the genesis allocator (no births unless the caller says otherwise).
func naturalHybridWorld(t *testing.T, f *Natural, births int64) (naturalWorld, kernel.Head) {
	t.Helper()
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	w, err := f.worldState(view)
	if err != nil {
		t.Fatal(err)
	}
	founders := make([]NaturalActorState, 16)
	for i := range founders {
		a, err := naturalActorState(view, NaturalMechanismV5, sim.EntityID(i+1))
		if err != nil {
			t.Fatal(err)
		}
		founders[i] = a
	}
	last, err := NaturalHighestFixedEntityID(2)
	if err != nil {
		t.Fatal(err)
	}
	var newborns []NaturalNewbornState
	for id := last + 1; ; id++ {
		a, err := naturalActorState(view, NaturalMechanismV5, id)
		if err != nil {
			break
		}
		newborns = append(newborns, NaturalNewbornState{ID: id, State: a})
	}
	if births > 0 {
		last += sim.EntityID(births)
	}
	if _, err := NaturalCheckConservation(2, w.patches, w.slots, founders, newborns, last); err != nil {
		t.Fatalf("hybrid world not conserved (last %d, births %d): %v", last, births, err)
	}
	return w, head
}

// naturalHighestFixedEntityIDMust is a test convenience over the typed
// contract constructor.
func naturalHighestFixedEntityIDMust(t *testing.T) sim.EntityID {
	t.Helper()
	last, err := NaturalHighestFixedEntityID(2)
	if err != nil {
		t.Fatal(err)
	}
	return last
}

// naturalGateSeeds builds the four-couple gate world: founders 1-4 hold two
// tenured slots and a stocked granary each (exactly the surplus-gate margins
// k>=2, granary>=3, energy>=3) on patch 1001 with an integer-exact patch
// ledger (R·4 pulses = 32 = gathered 16 + granted 12 + unrealized 4), the
// remaining founders neutral on the idle patch 1002.
func naturalGateSeeds(t *testing.T) []component.ComponentSeed {
	t.Helper()
	seeds := naturalNeutralSeeds(2, 8, 140)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		t.Helper()
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	setTenure := func(slotIndex int, owner sim.EntityID, gathered int64) {
		slotID, err := NaturalSlotID(2, 0, slotIndex)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: gathered})
		if owner == 0 {
			return
		}
		ownerRef, err := sim.EntityRefValue(owner)
		if err != nil {
			t.Fatal(err)
		}
		for i := range seeds {
			if seeds[i].Entity != slotID || seeds[i].Component != NaturalSlotTypeID {
				continue
			}
			for j := range seeds[i].Fields {
				switch seeds[i].Fields[j].Field {
				case NaturalSlotTenureOwnerField:
					seeds[i].Fields[j].Value = ownerRef
				case NaturalSlotTenureModeField:
					seeds[i].Fields[j].Value = sim.IntegerValue(int64(NaturalTenureCultivated))
				case NaturalSlotTenureCultivatedField:
					seeds[i].Fields[j].Value = sim.IntegerValue(1)
				}
			}
		}
	}
	// Four founders at the gate margins, two tenured slots each: the whole
	// patch is cultivated at k=8 total (the land identity at its bound).
	for i, founder := range []sim.EntityID{1, 2, 3, 4} {
		setTenure(2*i, founder, 2)
		setTenure(2*i+1, founder, 2)
		rewrite(founder, NaturalWorksiteTypeID, map[sim.FieldID]int64{
			NaturalWorksiteCapitalField: 2, NaturalWorksiteInvestedUnitsField: 4, NaturalWorksitePointsCreatedField: 2})
		rewrite(founder, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 3, NaturalGranaryCultivatedTotalField: 3})
		rewrite(founder, NaturalBodyTypeID, map[sim.FieldID]int64{
			NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6})
	}
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 4, NaturalPatchProducedField: 16, NaturalPatchGrantedField: 12, NaturalPatchUnrealizedField: 4})
	return seeds
}

// TestNaturalRunnerBirthAllocatorCollisionAndRestoreCrossCheck drives the
// runner's own birth path over the gate world: the request decision files
// with honest claims and the deterministic successor addressee, the reply
// decision accepts on the frozen consent order, both same-instant replies
// compute the SAME allocator-fresh newborn ID (genesis 2016 + committed
// births + 1) so the kernel's whole-proposal collision rule keeps exactly
// one winner with typed lost-birth evidence for the loser, registration
// advances the allocator exactly once, and conservation cross-checks the
// allocator last-ID fail closed on any tampering.
func TestNaturalRunnerBirthAllocatorCollisionAndRestoreCrossCheck(t *testing.T) {
	f, k := naturalHybrid(t, naturalGateSeeds(t), nil)
	f.pending = &naturalPending{started: make(map[sim.EntityID]strategy.NaturalAction), meals: make(map[sim.EntityID]NaturalMealIntent)}
	requestAt, err := NaturalPhaseTime(5, NaturalPhaseBirthRequest)
	if err != nil {
		t.Fatal(err)
	}
	replyAt, err := NaturalPhaseTime(5, NaturalPhaseBirthReply)
	if err != nil {
		t.Fatal(err)
	}
	// Two courting couples file at the request instant.
	proposals := []kernel.Proposal{}
	for _, pair := range [][2]sim.EntityID{{1, 2}, {3, 4}} {
		head := k.SnapshotHead()
		view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
		proposal, filed, err := f.birthRequestDecision(view, 5, requestAt, pair[0])
		if err != nil || !filed {
			t.Fatalf("request %d->%d: filed=%v err=%v", pair[0], pair[1], filed, err)
		}
		if err := f.guard(proposal); err != nil {
			t.Fatal(err)
		}
		proposals = append(proposals, proposal)
	}
	head := k.SnapshotHead()
	plan := make([]kernel.Plan, 0, len(proposals))
	replyBatch := make([]kernel.Plan, 0, 2)
	for _, p := range proposals {
		planned, err := k.Plan(p, head.Authority)
		if err != nil {
			t.Fatal(err)
		}
		plan = append(plan, planned)
	}
	if _, err := k.CommitBatch(plan); err != nil {
		t.Fatal(err)
	}
	// Both consenters decide against the same post-request snapshot: both
	// accept, both allocate newborn 2017 (one committed birth so far).
	head = k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	proposal1, attempt1, err := f.birthReplyDecision(view, 5, replyAt, 2)
	if err != nil || attempt1.Birth.Decision != NaturalReplyAccept {
		t.Fatalf("reply 2: %+v %v", attempt1, err)
	}
	if attempt1.Birth.Newborn != 2017 || attempt1.Birth.Diverged {
		t.Fatalf("reply 2 evidence: %+v", attempt1.Birth)
	}
	proposal2, attempt2, err := f.birthReplyDecision(view, 5, replyAt, 4)
	if err != nil || attempt2.Birth.Decision != NaturalReplyAccept {
		t.Fatalf("reply 4: %+v %v", attempt2, err)
	}
	if attempt2.Birth.Newborn != 2017 {
		t.Fatalf("concurrent reply allocated %d, want the same candidate 2017", attempt2.Birth.Newborn)
	}
	for _, p := range []kernel.Proposal{proposal1, proposal2} {
		if err := f.guard(p); err != nil {
			t.Fatal(err)
		}
		planned, err := k.Plan(p, head.Authority)
		if err != nil {
			t.Fatal(err)
		}
		replyBatch = append(replyBatch, planned)
	}
	events, err := k.CommitBatch(replyBatch)
	if err != nil {
		t.Fatal(err)
	}
	// Exactly one newborn committed; the loser carries the typed evidence.
	ids := make(map[string]sim.EventID, len(events))
	entityCreates := 0
	for _, ev := range events {
		ids[ev.Key] = ev.ID
		if ev.Kind == kernel.KindEntityCreate {
			entityCreates++
		}
	}
	if entityCreates != 1 {
		t.Fatalf("committed births: %d, want 1", entityCreates)
	}
	if err := naturalLinkAttempt(&attempt1, ids); err != nil || !attempt1.Accepted {
		t.Fatalf("winner attempt: %+v %v", attempt1, err)
	}
	if err := naturalLinkAttempt(&attempt2, ids); err != nil || attempt2.Lost != naturalLostCollision || attempt2.Accepted {
		t.Fatalf("loser attempt: %+v %v", attempt2, err)
	}
	// Registration advances the allocator exactly once and reserves the
	// WakeBirth first claim.
	var newbornEvent kernel.Event
	for _, ev := range events {
		if ev.Kind == kernel.KindEntityCreate {
			newbornEvent = ev
		}
	}
	if err := f.registerNewborn(newbornEvent, 5); err != nil {
		t.Fatal(err)
	}
	if f.allocator.Last() != 2017 || len(f.newborns) != 1 || f.newborns[0] != 2017 {
		t.Fatalf("allocator after registration: last %d newborns %v", f.allocator.Last(), f.newborns)
	}
	claim6, err := NaturalPhaseTime(6, NaturalPhaseClaim)
	if err != nil {
		t.Fatal(err)
	}
	reserved := 0
	for _, w := range f.sched.Snapshot().Wakes {
		if w.Actor == 2017 {
			reserved++
			if w.Cause != scheduler.WakeBirth || w.At != claim6 {
				t.Fatalf("newborn first wake: %+v", w)
			}
		}
	}
	if reserved != 1 {
		t.Fatalf("newborn reserved wakes: %d", reserved)
	}
	// Restore cross-check: conservation holds for the true last-ID and
	// fails closed for either neighbor.
	w, _ := naturalHybridWorld(t, f, 1)
	if w.actors[2017].Genome.ParentA != 1 || w.actors[2017].Genome.ParentB != 2 || w.actors[2017].Bag.Source != 1001 {
		t.Fatalf("newborn rows: %+v", w.actors[2017])
	}
	if w.actors[1].Granary.Stock != 1 || w.actors[1].Genome.BirthGranaryPaid != 2 || w.actors[1].Request.Status != NaturalRequestAccepted {
		t.Fatalf("proposer after birth: %+v", w.actors[1])
	}
	// The allocator-ids ceiling override fires typed evidence at the reply
	// decision when the candidate would pass the declared bound.
	f2, _ := naturalHybrid(t, naturalGateSeeds(t), map[NaturalCeiling]int64{NaturalCeilingAllocatorIDs: 2016})
	f2.pending = &naturalPending{started: make(map[sim.EntityID]strategy.NaturalAction), meals: make(map[sim.EntityID]NaturalMealIntent)}
	requestProposal, filed, err := f2.birthRequestDecision(
		scheduler.SnapshotView{Reader: f2.k.SnapshotHead().Reader, Authority: f2.k.SnapshotHead().Authority, Version: f2.k.SnapshotHead().Version},
		5, requestAt, 1)
	if err != nil || !filed {
		t.Fatal(err)
	}
	auth := f2.k.SnapshotHead().Authority
	planned, err := f2.k.Plan(requestProposal, auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f2.k.CommitBatch([]kernel.Plan{planned}); err != nil {
		t.Fatal(err)
	}
	head = f2.k.SnapshotHead()
	_, _, err = f2.birthReplyDecision(
		scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, 5, replyAt, 2)
	var hitErr *NaturalCeilingHitError
	if !errors.As(err, &hitErr) || hitErr.Hit.Ceiling != NaturalCeilingAllocatorIDs || hitErr.Hit.Threshold != 2016 || hitErr.Hit.Observed != 2017 {
		t.Fatalf("allocator ceiling hit: %v", err)
	}
	if len(f2.pending.hits) != 1 {
		t.Fatalf("pending hits: %v", f2.pending.hits)
	}
}

// naturalBuilderSeeds builds the build-denial world: founder 1 holds a bag
// unit and work in progress (last build hour 3, so the completion at hour 4
// is frozen-legal), founders 2-5 hold the patch's slots. With everyFreeSlot
// false the patch is fully tenured and the folded completion is refused with
// zero mutation; with everyFreeSlot true slot 0 is wild and stock-0 and the
// fold claims exactly it. Gathered counts keep G1 exact: gathered (slots) =
// held (founder 1's bag unit) + invested (2·created + wip over every homed
// actor).
func naturalBuilderSeeds(t *testing.T, everyFreeSlot bool) []component.ComponentSeed {
	t.Helper()
	seeds := naturalNeutralSeeds(2, 8, 140)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		t.Helper()
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	setTenure := func(slotIndex int, owner sim.EntityID, gathered int64) {
		slotID, err := NaturalSlotID(2, 0, slotIndex)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: gathered})
		if owner == 0 {
			return
		}
		ownerRef, err := sim.EntityRefValue(owner)
		if err != nil {
			t.Fatal(err)
		}
		for i := range seeds {
			if seeds[i].Entity != slotID || seeds[i].Component != NaturalSlotTypeID {
				continue
			}
			for j := range seeds[i].Fields {
				switch seeds[i].Fields[j].Field {
				case NaturalSlotTenureOwnerField:
					seeds[i].Fields[j].Value = ownerRef
				case NaturalSlotTenureModeField:
					seeds[i].Fields[j].Value = sim.IntegerValue(int64(NaturalTenureCultivated))
				case NaturalSlotTenureCultivatedField:
					seeds[i].Fields[j].Value = sim.IntegerValue(1)
				}
			}
		}
	}
	tenure := map[int]sim.EntityID{}
	gathered := map[int]int64{}
	if everyFreeSlot {
		tenure[1] = 2
		for i := 2; i <= 7; i++ {
			tenure[i] = sim.EntityID(2 + (i+1)/2) // slots 2,3 -> 3; 4,5 -> 4; 6,7 -> 5
		}
		gathered[0], gathered[1] = 1, 3
		for i := 2; i <= 7; i++ {
			gathered[i] = 2
		}
	} else {
		for i := 0; i <= 7; i++ {
			tenure[i] = sim.EntityID(2 + i/2) // slots 0,1 -> 2; 2,3 -> 3; 4,5 -> 4; 6,7 -> 5
			gathered[i] = 2
		}
		gathered[0] = 4 // two pre-tenure gathers ride this slot
	}
	capital := map[sim.EntityID]int64{}
	for _, owner := range tenure {
		capital[owner]++
	}
	for slotIndex, owner := range tenure {
		setTenure(slotIndex, owner, gathered[slotIndex])
	}
	if everyFreeSlot {
		setTenure(0, 0, gathered[0]) // the free wild slot stays unowned
	}
	for founder, k := range capital {
		rewrite(founder, NaturalWorksiteTypeID, map[sim.FieldID]int64{
			NaturalWorksiteCapitalField: k, NaturalWorksiteInvestedUnitsField: 2 * k, NaturalWorksitePointsCreatedField: k})
		rewrite(founder, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 3, NaturalGranaryCultivatedTotalField: 3})
		rewrite(founder, NaturalBodyTypeID, map[sim.FieldID]int64{
			NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6})
	}
	// Founder 1: bag unit in hand, one unit of work in progress, and its
	// gather and build of this hour already staged.
	rewrite(1, NaturalBagTypeID, map[sim.FieldID]int64{NaturalBagUnitsField: 1})
	rewrite(1, NaturalBodyTypeID, map[sim.FieldID]int64{NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: 3})
	rewrite(1, NaturalWorksiteTypeID, map[sim.FieldID]int64{NaturalWorksiteWipField: 1, NaturalWorksiteInvestedUnitsField: 1, NaturalWorksiteLastBuildHourField: 3})
	// Patch ledger: 4 pulses at R=8 -> 32 units = produced + granted (one
	// stocked granary per slot-holding founder) + unrealized; produced =
	// gathered = held 1 + invested (2·Σcreated + Σwip).
	invested := int64(1) // founder 1's work in progress
	for _, k := range capital {
		invested += 2 * k
	}
	gath := invested + 1
	granted := int64(3 * len(capital))
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 4, NaturalPatchProducedField: gath, NaturalPatchGrantedField: granted,
		NaturalPatchUnrealizedField: 8*4 - gath - granted})
	return seeds
}

// TestNaturalRunnerBuildClaimFoldAndDenialLoop drives the coordinator's
// build-completion fold and the documented denial loop on the runner's own
// code: a fully tenured home patch refuses the completion with the typed
// zero-mutation refusal (nothing converts, the bag keeps its unit) and the
// same-hour re-evaluation with LastDenial = NaturalBuildDenied eats the held
// unit under rule 507 at the paired instant; a free lowest-index slot is
// claimed atomically by the same fold.
func TestNaturalRunnerBuildClaimFoldAndDenialLoop(t *testing.T) {
	for _, everyFreeSlot := range []bool{false, true} {
		f, k := naturalHybrid(t, naturalBuilderSeeds(t, everyFreeSlot), nil)
		f.current[1] = strategy.NaturalBuild
		start, err := NaturalPhaseTime(4, NaturalPhaseBuildStart)
		if err != nil {
			t.Fatal(err)
		}
		at := NaturalBuildCompletion(start)
		head := k.SnapshotHead()
		if err := f.arbitrateCompletions(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, at, []sim.EntityID{1}, nil); err != nil {
			t.Fatal(err)
		}
		outcome, ok := f.builds[1]
		if !ok {
			t.Fatal("no fold outcome")
		}
		if everyFreeSlot {
			if outcome.refused != NaturalBuildRefusalNone {
				t.Fatalf("free-slot variant refused: %d", outcome.refused)
			}
			if err := f.guard(outcome.proposal); err != nil {
				t.Fatal(err)
			}
			planned, err := k.Plan(outcome.proposal, head.Authority)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := k.CommitBatch([]kernel.Plan{planned}); err != nil {
				t.Fatal(err)
			}
			w, _ := naturalHybridWorld(t, f, 0)
			slot := w.slots[0][0]
			if slot.Mode != NaturalTenureCultivated || slot.Owner != 1 || slot.Cultivated != 1 || slot.Stock != 0 {
				t.Fatalf("claimed slot: %+v", slot)
			}
			a := w.actors[1]
			if a.Worksite.Capital != 1 || a.Worksite.Wip != 0 || a.Worksite.InvestedUnits != 2 || a.Worksite.PointsCreated != 1 || a.Bag.Units != 0 {
				t.Fatalf("builder after claim: %+v", a)
			}
			continue
		}
		if outcome.refused != NaturalBuildRefusalNoWildSlot {
			t.Fatalf("saturated variant: refused %d, want NoWildSlot", outcome.refused)
		}
		if outcome.proposal.Key != "" {
			t.Fatal("refused completion produced a proposal")
		}
		// Zero mutation: nothing converted, the bag keeps its unit.
		w, _ := naturalHybridWorld(t, f, 0)
		if w.actors[1].Bag.Units != 1 || w.actors[1].Worksite.Wip != 1 {
			t.Fatalf("refusal mutated state: %+v", w.actors[1])
		}
		// The same-hour re-eat at the paired instant: the policy reports
		// NaturalEat under the BuildDenied denial and the runner commits
		// rule 507 with the contract's exact patch set.
		paired, err := NaturalPhaseTime(4, NaturalPhasePairedMeal)
		if err != nil {
			t.Fatal(err)
		}
		// The approved placement decision: the contract's consume proposal
		// pins the frozen bag-decision instant (h+20m+2µs), which lies in
		// the past at the completion instant and cannot be committed (the
		// kernel requires monotonic history times), so the documented
		// denial loop commits the re-decided eat at the NEXT frozen instant
		// of the same hour — exactly h+40m+4µs, the paired-meal slot. The
		// pins below fail the test if the placement ever drifts.
		if !(at < paired) {
			t.Fatalf("completion instant %d is not before the paired instant %d", at, paired)
		}
		head = k.SnapshotHead()
		proposal, attempt, err := f.naturalDenialEat(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, 4, paired, 1)
		if err != nil {
			t.Fatal(err)
		}
		if attempt.Kind != "build-denied-eat" || attempt.Key != proposal.Key {
			t.Fatalf("denial-eat attempt: %+v", attempt)
		}
		if proposal.Time != paired || attempt.Time != paired {
			t.Fatalf("denial-eat placement %d/%d, want exactly h+40m+4µs = %d", proposal.Time, attempt.Time, paired)
		}
		if err := f.guard(proposal); err != nil {
			t.Fatal(err)
		}
		planned, err := k.Plan(proposal, head.Authority)
		if err != nil {
			t.Fatal(err)
		}
		events, err := k.CommitBatch([]kernel.Plan{planned})
		if err != nil || len(events) != 1 || events[0].Rule != NaturalConsumeRule {
			t.Fatalf("denial-eat commit: %v %v", events, err)
		}
		w, _ = naturalHybridWorld(t, f, 0)
		a := w.actors[1]
		if a.Bag.Units != 0 || a.Body.Energy != 6 || a.Body.Consumed != 1 || a.Worksite.Wip != 1 {
			t.Fatalf("builder after denial-eat: %+v", a)
		}
	}
}

// naturalSpoilSeeds builds the long-granary world: founder 1 holds all eight
// slots (k=8, the land bound) with a stocked granary of 30 cultivated units,
// so the runner's spoilage stage rots on every pulse while wear decays the
// standing capital and reverts slots.
func naturalSpoilSeeds(t *testing.T) []component.ComponentSeed {
	t.Helper()
	seeds := naturalNeutralSeeds(2, 8, 140)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		t.Helper()
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	ownerRef, err := sim.EntityRefValue(1)
	if err != nil {
		t.Fatal(err)
	}
	for s := 0; s < NaturalSlotsPerPatch; s++ {
		slotID, err := NaturalSlotID(2, 0, s)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: 2})
		for i := range seeds {
			if seeds[i].Entity != slotID || seeds[i].Component != NaturalSlotTypeID {
				continue
			}
			for j := range seeds[i].Fields {
				switch seeds[i].Fields[j].Field {
				case NaturalSlotTenureOwnerField:
					seeds[i].Fields[j].Value = ownerRef
				case NaturalSlotTenureModeField:
					seeds[i].Fields[j].Value = sim.IntegerValue(int64(NaturalTenureCultivated))
				case NaturalSlotTenureCultivatedField:
					seeds[i].Fields[j].Value = sim.IntegerValue(1)
				}
			}
		}
	}
	// The idle patch keeps its produced standing stock across six pulses.
	rewrite(1002, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 6, NaturalPatchProducedField: 8, NaturalPatchUnrealizedField: 40})
	for s := 0; s < NaturalSlotsPerPatch; s++ {
		slotID, err := NaturalSlotID(2, 1, s)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotStockField: 1})
	}
	rewrite(1, NaturalWorksiteTypeID, map[sim.FieldID]int64{
		NaturalWorksiteCapitalField: 8, NaturalWorksiteInvestedUnitsField: 16, NaturalWorksitePointsCreatedField: 8})
	rewrite(1, NaturalGranaryTypeID, map[sim.FieldID]int64{
		NaturalGranaryStockField: 30, NaturalGranaryCultivatedTotalField: 30})
	rewrite(1, NaturalBodyTypeID, map[sim.FieldID]int64{NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6})
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 6, NaturalPatchProducedField: 16, NaturalPatchGrantedField: 30, NaturalPatchUnrealizedField: 2})
	return seeds
}

// naturalManualPulse drives one clock pulse of the hybrid runner and commits
// its proposals at the pulse instant.
func naturalManualPulse(t *testing.T, f *Natural, hour int) {
	t.Helper()
	f.pending = &naturalPending{started: make(map[sim.EntityID]strategy.NaturalAction), meals: make(map[sim.EntityID]NaturalMealIntent)}
	defer func() { f.pending = nil }()
	at, err := NaturalHourTime(hour)
	if err != nil {
		t.Fatal(err)
	}
	ready := scheduler.ReadyFiber{Fiber: scheduler.Fiber{Actor: naturalClockActor}, At: at}
	head := f.k.SnapshotHead()
	eval, err := f.pulse(ready, scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version})
	if err != nil {
		t.Fatalf("pulse h%d: %v", hour, err)
	}
	plans := make([]kernel.Plan, 0, len(eval.Proposals))
	for _, p := range eval.Proposals {
		if err := f.guard(p); err != nil {
			t.Fatal(err)
		}
		planned, err := f.k.Plan(p, head.Authority)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, planned)
	}
	if _, err := f.k.CommitBatch(plans); err != nil {
		t.Fatal(err)
	}
}

// TestNaturalRunnerSpoilageOverLongGranary walks the runner's spoilage stage
// over a stocked granary (stock 30 against S=24): every pulse rots, the
// spoiled counter and the debt carry stay exact against G2a, the at-rest
// carry never reaches the period, and the identity R = grants + wild +
// unrealized holds while wear decays the standing capital and reverts its
// lowest-index slots.
func TestNaturalRunnerSpoilageOverLongGranary(t *testing.T) {
	f, _ := naturalHybrid(t, naturalSpoilSeeds(t), nil)
	for h := 6; h <= 8; h++ {
		naturalManualPulse(t, f, h)
		w, _ := naturalHybridWorld(t, f, 0)
		a := w.actors[1]
		if a.Granary.Spoiled < 1 {
			t.Fatalf("h%d: no spoilage over the long granary: %+v", h, a.Granary)
		}
		if a.Granary.SpoilDebt >= NaturalSpoilagePeriod {
			t.Fatalf("h%d: at-rest carry %d reached the period", h, a.Granary.SpoilDebt)
		}
		if a.Granary.CultivatedTotal != a.Granary.Stock+a.Granary.StoredMeals+a.Granary.Spoiled+a.Genome.BirthGranaryPaid {
			t.Fatalf("h%d: G2a broken: %+v", h, a.Granary)
		}
		// The grant seam moves every entitlement into the granary in the
		// same pulse.
		if w.patches[0].Granted != a.Granary.CultivatedTotal {
			t.Fatalf("h%d: grant seam %d != cultivatedTotal %d", h, w.patches[0].Granted, a.Granary.CultivatedTotal)
		}
	}
	w, _ := naturalHybridWorld(t, f, 0)
	if w.actors[1].Worksite.Capital >= 8 || w.actors[1].Worksite.PointsDecayed < 1 {
		t.Fatalf("wear did not decay the standing capital: %+v", w.actors[1].Worksite)
	}
}

// naturalFallowSeeds builds the dead-owner window: founder 1 is dead
// (died-hour 3, energy retained per the stopped-state convention) while
// still holding two tenured slots (k=2), and founders 2-4 hold two tenured
// slots and a stocked granary each. Every slot is tenured (the land
// identity at its bound over ALL tenure holders), and the patch ledger is
// integer-exact: 6 pulses at R=8 -> 48 = produced 16 + granted 12 +
// unrealized 20, with gathered 16 = invested 16.
func naturalFallowSeeds(t *testing.T) []component.ComponentSeed {
	t.Helper()
	seeds := naturalNeutralSeeds(2, 8, 140)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		t.Helper()
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	setTenure := func(slotIndex int, owner sim.EntityID, gathered int64) {
		slotID, err := NaturalSlotID(2, 0, slotIndex)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: gathered})
		ownerRef, err := sim.EntityRefValue(owner)
		if err != nil {
			t.Fatal(err)
		}
		for i := range seeds {
			if seeds[i].Entity != slotID || seeds[i].Component != NaturalSlotTypeID {
				continue
			}
			for j := range seeds[i].Fields {
				switch seeds[i].Fields[j].Field {
				case NaturalSlotTenureOwnerField:
					seeds[i].Fields[j].Value = ownerRef
				case NaturalSlotTenureModeField:
					seeds[i].Fields[j].Value = sim.IntegerValue(int64(NaturalTenureCultivated))
				case NaturalSlotTenureCultivatedField:
					seeds[i].Fields[j].Value = sim.IntegerValue(1)
				}
			}
		}
	}
	for i, founder := range []sim.EntityID{1, 2, 3, 4} {
		setTenure(2*i, founder, 2)
		setTenure(2*i+1, founder, 2)
		rewrite(founder, NaturalWorksiteTypeID, map[sim.FieldID]int64{
			NaturalWorksiteCapitalField: 2, NaturalWorksiteInvestedUnitsField: 4, NaturalWorksitePointsCreatedField: 2})
		rewrite(founder, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 3, NaturalGranaryCultivatedTotalField: 3})
		rewrite(founder, NaturalBodyTypeID, map[sim.FieldID]int64{
			NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6})
	}
	// The dead founder: died-hour 3 with its energy retained and its slots
	// still tenured (the frozen dead-owner window). The idle patch keeps
	// six pulses of standing stock so both patches sit at pulses 6.
	rewrite(1, NaturalGenomeTypeID, map[sim.FieldID]int64{NaturalGenomeDiedHourField: 3})
	rewrite(1002, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 6, NaturalPatchProducedField: 8, NaturalPatchUnrealizedField: 40})
	for i := 0; i < NaturalSlotsPerPatch; i++ {
		slotID, err := NaturalSlotID(2, 1, i)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotStockField: 1})
	}
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 6, NaturalPatchProducedField: 16, NaturalPatchGrantedField: 12, NaturalPatchUnrealizedField: 20})
	return seeds
}

// TestNaturalRunnerFallowGrantSkipAndDeadRevert drives the dead-owner window
// through the runner's own pulse: the fallow slots of the dead owner receive
// no grant (exactly their share of the budget unrealizes each pulse), wear
// keeps decaying the dead capital and reverts the LOWEST-index slot to wild
// with stock 0, the reverted slot refills on the next pulse, and the land
// identity holds over ALL tenure holders while the alive-only form fails by
// exactly the dead owner's standing capital.
func TestNaturalRunnerFallowGrantSkipAndDeadRevert(t *testing.T) {
	f, _ := naturalHybrid(t, naturalFallowSeeds(t), nil)
	f.pulseLifecycle[1] = scheduler.Stopped // the dead founder's fiber is stopped
	for h := 6; h <= 9; h++ {
		// The fallow share of THIS pulse's budget is the dead owner's
		// tenured count at the pulse (regen runs before the pulse's wear,
		// so the h8 decay only shrinks the window from h9 on).
		before, _ := naturalHybridWorld(t, f, 0)
		fallowBefore := 0
		for _, slot := range before.slots[0] {
			if slot.Mode == NaturalTenureCultivated && slot.Owner == 1 {
				fallowBefore++
			}
		}
		unrealizedBefore := before.patches[0].Unrealized
		naturalManualPulse(t, f, h)
		w, _ := naturalHybridWorld(t, f, 0)
		dead := w.actors[1]
		if dead.Genome.DiedHour != 3 {
			t.Fatalf("h%d: dead owner rows: %+v", h, dead.Genome)
		}
		fallow := 0
		for _, slot := range w.slots[0] {
			if slot.Mode == NaturalTenureCultivated && slot.Owner == 1 {
				fallow++
			}
		}
		var aliveCapital, tenured int64
		for id := sim.EntityID(2); id <= 16; id++ {
			aliveCapital += w.actors[id].Worksite.Capital
		}
		for _, slot := range w.slots[0] {
			if slot.Mode == NaturalTenureCultivated {
				tenured++
			}
		}
		if tenured-aliveCapital != int64(fallow) {
			t.Fatalf("h%d: alive-only land gap %d != dead capital %d", h, tenured-aliveCapital, fallow)
		}
		// Exactly the pre-pulse fallow share of the budget unrealized.
		if got := w.patches[0].Unrealized - unrealizedBefore; got != int64(fallowBefore) {
			t.Fatalf("h%d: unrealized delta %d != pre-pulse fallow share %d", h, got, fallowBefore)
		}
		// The dead granary receives no fallow grant (stock and the
		// cultivated-received ledger stay at their death-hour values) while
		// the spoilage stage still rots it (debt accrues by stock each
		// pulse; three pulses of stock 3 stay under S=24).
		if dead.Granary.Stock != 3 || dead.Granary.CultivatedTotal != 3 {
			t.Fatalf("h%d: dead granary moved: %+v", h, dead.Granary)
		}
		if dead.Granary.SpoilDebt != 3*int64(h-5) {
			t.Fatalf("h%d: dead granary spoil debt %d", h, dead.Granary.SpoilDebt)
		}
	}
	w, _ := naturalHybridWorld(t, f, 0)
	// h8's wear decayed the dead capital 2 -> 1 and reverted slot 0, which
	// refilled wild on h9; slot 1 is still fallow-tenured.
	slot0, slot1 := w.slots[0][0], w.slots[0][1]
	if slot0.Mode != NaturalTenureWild || slot0.Owner != 0 || slot0.Stock != 1 {
		t.Fatalf("reverted slot: %+v", slot0)
	}
	if slot1.Mode != NaturalTenureCultivated || slot1.Owner != 1 {
		t.Fatalf("fallow slot: %+v", slot1)
	}
	if w.actors[1].Worksite.Capital != 1 || w.actors[1].Worksite.PointsDecayed != 1 {
		t.Fatalf("dead owner wear: %+v", w.actors[1].Worksite)
	}
	for _, id := range []sim.EntityID{2, 3, 4} {
		if w.actors[id].Worksite.Capital != 1 {
			t.Fatalf("founder %d wear: %+v", id, w.actors[id].Worksite)
		}
	}
}

// TestNaturalRunnerDeepTimeConservationAndMortality walks the real runner
// past the first founder age-deaths into the long run and pins what holds
// there: the extended conservation gates at every hour boundary, the journal
// clock and linkage, and cause-attributed mortality — at every hour exactly
// the founders whose immutable death-hour has passed are dead, all by age in
// this window (the saturated-cycle collapse leaves the world all-wild long
// before the die-off, so no fallow window arises organically here; the
// fallow mechanics are pinned by the dead-owner hybrid above and by the
// wear-revert rows of the hand trajectory at h7-h10).
func TestNaturalRunnerDeepTimeConservationAndMortality(t *testing.T) {
	// The runner-resolved founder lifetimes (stream 0x464F554E, seed 0).
	lifetimes := sim.NewRandomStream(sim.RandomState{Seed: 0, Stream: NaturalFounderStream})
	deathHours := make([]int64, 16)
	firstDeath := int64(NaturalHorizonHours)
	for i := range deathHours {
		deathHours[i] = NaturalLifetimeBase + int64(lifetimes.Uint64()%NaturalLifetimeSpan)
		if deathHours[i] < firstDeath {
			firstDeath = deathHours[i]
		}
	}
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	end := int(firstDeath + 25)
	if end > NaturalHorizonHours {
		end = NaturalHorizonHours
	}
	naturalRunTo(t, f, end)
	naturalAssertConservation(t, f)
	naturalAssertJournalClock(t, f)
	checks := f.Checkpoints()
	for h := int(firstDeath); h <= end; h++ {
		want := 0
		for _, d := range deathHours {
			if d <= int64(h) {
				want++
			}
		}
		got := checks[h].Metrics.DeathsAge + checks[h].Metrics.DeathsStarvation
		if got != int64(want) {
			t.Fatalf("h%d deaths %d, want %d", h, got, want)
		}
		if checks[h].Metrics.DeathsStarvation != 0 {
			t.Fatalf("h%d starvation deaths in the walked window", h)
		}
		for _, id := range f.roster {
			died := checks[h].Founders[id-1].Genome.DiedHour
			want_died := NaturalNeverHour
			if deathHours[id-1] <= int64(h) {
				want_died = int64(h)
				if deathHours[id-1] != int64(h) && deathHours[id-1] < int64(h) {
					want_died = deathHours[id-1]
				}
			}
			if died != want_died {
				t.Fatalf("h%d actor %d died-hour %d, want %d", h, id, died, want_died)
			}
		}
	}
}

// TestNaturalRunnerCeilingLedgerTypedHit covers the compute-ceiling
// instrumentation: a declared lowered threshold fires the typed CeilingHit at
// the first crossing (fail-loud, recorded evidence, halted run), the frozen
// ledger carries every frozen threshold, and the projection arithmetic is
// exact.
func TestNaturalRunnerCeilingLedgerTypedHit(t *testing.T) {
	// Option validation: unknown ceilings and raises are refused.
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Workers: 1, CeilingThresholds: map[NaturalCeiling]int64{"unknown": 1}}); err == nil {
		t.Fatal("unknown ceiling accepted")
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Workers: 1, CeilingThresholds: map[NaturalCeiling]int64{NaturalCeilingHistoryEvents: kernel.MaxHistoryEvents + 1}}); err == nil {
		t.Fatal("raised ceiling accepted")
	}
	// The declared lowered history-events bound fires at the first commit
	// that crosses it.
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1,
		CeilingThresholds: map[NaturalCeiling]int64{NaturalCeilingHistoryEvents: 25}})
	if err != nil {
		t.Fatal(err)
	}
	var hitErr *NaturalCeilingHitError
	for {
		_, err := f.Step(context.Background())
		if err == nil {
			if len(f.Checkpoints()) > 6 {
				t.Fatal("no ceiling hit within six hours of a 25-event bound")
			}
			continue
		}
		if !errors.As(err, &hitErr) {
			t.Fatalf("untyped error: %v", err)
		}
		break
	}
	hit := hitErr.Hit
	if hit.Ceiling != NaturalCeilingHistoryEvents || hit.Threshold != 25 || hit.Observed <= 25 {
		t.Fatalf("typed hit: %+v", hit)
	}
	if hit.WorldVersion == 0 || hit.Hour < 0 || hit.Births != 0 {
		t.Fatalf("hit context: %+v", hit)
	}
	if hits := f.CeilingHits(); len(hits) != 1 || hits[0] != hit {
		t.Fatalf("recorded hits: %v", hits)
	}
	// The run is halted fail-loud: further steps and the handoff refuse.
	if _, err := f.Step(context.Background()); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("post-hit step: %v", err)
	}
	if _, err := f.Handoff(); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("post-hit handoff: %v", err)
	}
	// The projection arithmetic (pure): with a positive rate the ceiling
	// binds after ceil(headroom/rate) hours; over binds now; without a rate
	// it never projects.
	if got := naturalProjectBindingHour(100, 90, 5, 7); got != 9 {
		t.Fatalf("projection: %d", got)
	}
	if got := naturalProjectBindingHour(100, 96, 5, 7); got != 8 {
		t.Fatalf("projection at exact headroom: %d", got)
	}
	if got := naturalProjectBindingHour(100, 101, 0, 3); got != 3 {
		t.Fatalf("projection when over: %d", got)
	}
	if got := naturalProjectBindingHour(100, 50, 0, 3); got != -1 {
		t.Fatalf("projection without rate: %d", got)
	}
}

// TestNaturalRunnerDeterminismWorkers1And4 pins worker-count independence:
// identical event bytes, journals, checkpoints (including the ceiling ledger
// and allocator last-ID) for the births-enabled saturated fixture.
func TestNaturalRunnerDeterminismWorkers1And4(t *testing.T) {
	runs := make([]*Natural, 0, 2)
	for _, workers := range []int{1, 4} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 7, Workers: workers, Births: true})
		if err != nil {
			t.Fatal(err)
		}
		naturalRunTo(t, f, 8)
		runs = append(runs, f)
	}
	a, b := runs[0], runs[1]
	eventsA, eventsB := a.k.Events(), b.k.Events()
	if len(eventsA) != len(eventsB) {
		t.Fatalf("event count divergence: %d vs %d", len(eventsA), len(eventsB))
	}
	for i := range eventsA {
		ba, errA := eventsA[i].Bytes()
		bb, errB := eventsB[i].Bytes()
		if errA != nil || errB != nil || !bytes.Equal(ba, bb) {
			t.Fatalf("event %d divergence", i)
		}
	}
	if !reflect.DeepEqual(a.Journal(), b.Journal()) {
		t.Fatal("journal divergence")
	}
	if !reflect.DeepEqual(a.Checkpoints(), b.Checkpoints()) {
		t.Fatal("checkpoint divergence")
	}
}

// TestNaturalRunnerHourlyBoundsAndHandoffSeam revalidates the frozen bounds
// on every row at every hour boundary, the complete frozen ceiling ledger in
// every checkpoint, and the value-only handoff seam (portable bytes plus the
// verified head, roster state, allocator last-ID, declared thresholds,
// journal, and checkpoints).
func TestNaturalRunnerHourlyBoundsAndHandoffSeam(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 3, Workers: 2, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 12)
	naturalAssertConservation(t, f)
	naturalAssertJournalClock(t, f)
	frozen := NaturalFrozenCeilingThresholds()
	genesisLast, err := NaturalHighestFixedEntityID(2)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range f.Checkpoints() {
		if len(check.Ledger) != len(naturalCeilingOrder) {
			t.Fatalf("h%d ledger rows: %d", check.Hour, len(check.Ledger))
		}
		for i, row := range check.Ledger {
			if row.Ceiling != naturalCeilingOrder[i] || row.Threshold != frozen[row.Ceiling] {
				t.Fatalf("h%d ledger row %d: %+v", check.Hour, i, row)
			}
		}
		actor := func(id sim.EntityID, a NaturalActorState) {
			if !validNaturalActor(2, id, a) {
				t.Fatalf("h%d actor %d invalid: %+v", check.Hour, id, a)
			}
			if a.Granary.Stock > NaturalGranaryStockMax || a.Granary.SpoilDebt >= NaturalSpoilagePeriod ||
				a.Granary.Spoiled > NaturalSpoiledMax || a.Body.BasalSpent > NaturalBasalSpentMax ||
				a.Granary.StoredMeals > NaturalHorizonHours {
				t.Fatalf("h%d actor %d outside the horizon envelopes: %+v", check.Hour, id, a)
			}
		}
		for i, a := range check.Founders {
			actor(sim.EntityID(i+1), a)
		}
		for _, n := range check.Newborns {
			actor(n.ID, n.State)
		}
		for _, patch := range check.Slots {
			for _, slot := range patch {
				if !validNaturalSlot(slot) {
					t.Fatalf("h%d slot invalid: %+v", check.Hour, slot)
				}
			}
		}
		if check.AllocatorLast < genesisLast {
			t.Fatalf("h%d allocator last %d below genesis", check.Hour, check.AllocatorLast)
		}
	}
	handoff, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if handoff.FormatVersion != NaturalFormatVersion || handoff.Patches != 2 || handoff.R != 8 || handoff.Seed != 3 {
		t.Fatalf("handoff identity: %+v", handoff)
	}
	if handoff.AllocatorLast != genesisLast || len(handoff.History) == 0 || len(handoff.SchedulerBytes) == 0 {
		t.Fatalf("handoff seam: last %d history %d portable %d", handoff.AllocatorLast, len(handoff.History), len(handoff.SchedulerBytes))
	}
	if len(handoff.CeilingThresholds) != len(frozen) {
		t.Fatalf("handoff thresholds: %d", len(handoff.CeilingThresholds))
	}
	for ceiling, value := range frozen {
		if handoff.CeilingThresholds[ceiling] != value {
			t.Fatalf("handoff threshold %q: %d", ceiling, handoff.CeilingThresholds[ceiling])
		}
	}
	if !reflect.DeepEqual(handoff.Journal, f.Journal()) || !reflect.DeepEqual(handoff.Checkpoints, f.Checkpoints()) {
		t.Fatal("handoff journal/checkpoint drift")
	}
	head := f.k.SnapshotHead()
	if handoff.Head.TipID != head.TipID || handoff.Head.TipHash != head.TipHash {
		t.Fatal("handoff head drift")
	}
}

// Review P2: the hourly ledger must not fabricate a trailing-hour rate or a
// projected binding hour for ceilings that are not sampled hourly, and the
// allocator-ids row must report the real allocator state.
func TestNaturalCeilingLedgerRowsAreHonest(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 3)
	check := f.Checkpoints()[3]
	byCeiling := map[NaturalCeiling]NaturalCeilingRow{}
	for _, row := range check.Ledger {
		if _, dup := byCeiling[row.Ceiling]; dup {
			t.Fatalf("duplicate ceiling row %s", row.Ceiling)
		}
		byCeiling[row.Ceiling] = row
	}
	for _, ceiling := range naturalCeilingOrder {
		row, ok := byCeiling[ceiling]
		if !ok {
			t.Fatalf("missing ceiling row %s", ceiling)
		}
		if naturalCeilingIsHourlyProjected(ceiling) {
			continue
		}
		if row.RateLastHour != 0 || row.ProjectedBindingHour != -1 {
			t.Fatalf("%s: fabricated rate/binding hour %+v", ceiling, row)
		}
	}
	if got := byCeiling[NaturalCeilingAllocatorIDs].Observed; got != uint64(check.AllocatorLast) {
		t.Fatalf("allocator-ids observed %d, want %d", got, check.AllocatorLast)
	}
}
