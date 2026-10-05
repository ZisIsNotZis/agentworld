package world

// The birth-reply policy-refusal discipline (ticket 18 E12 defect). The
// v6/v7 policy consent floor (own energy >= TFile = 4) is strictly stronger
// than the contract's frozen floor (NaturalConsentMinEnergy = 3), so a pair
// can exist where the policy refuses while the contract's whole truth gate
// accepts. The reply decision must resolve such a reply as the policy's own
// typed refusal — never by consulting the contract's birth execution, which
// would build (and needs an allocator candidate for) a birth no decision
// committed. The f=12 founding-band cell reaches that band organically at
// hour 9: the crossed-pair reply instants at hours 6 and 7 pay four parents
// and leave the consenter at exactly energy 3.

import (
	"errors"
	"testing"

	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// naturalHybridV7 is the naturalHybrid shape on the declared v6/natural-7
// identity: the v6 registry, the frozen natural/7 policy, and the mechanism
// axis set, so a test can drive birthRequestDecision and birthReplyDecision
// directly against the policy floors that diverge from the contract's.
func naturalHybridV7(t *testing.T, seeds []component.ComponentSeed, ceilings map[NaturalCeiling]int64) (*Natural, *kernel.Kernel) {
	t.Helper()
	reg, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	policy := strategy.FrozenNaturalPolicy7()
	policies := strategy.NewNaturalRegistry(2)
	if err := policies.RegisterFrozenNatural(policy); err != nil {
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
	f := &Natural{registry: reg, policies: policies, ref: policy.Ref, mechanism: NaturalMechanismV6, founders: naturalFoundersResolved(2, 0), patches: 2, seed: 0, r: 8, births: true,
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
		if err = sched.Register(id); err != nil {
			t.Fatal(err)
		}
	}
	if err = sched.Register(naturalClockActor); err != nil {
		t.Fatal(err)
	}
	return f, k
}

// naturalGateV6Seeds lifts naturalGateSeeds (founders 1-4 at the frozen
// surplus-gate margins k>=2, granary>=3, energy>=5 on patch 1001) to the v6
// granary genesis schema (the provisioned ledger at zero on every founder
// row) and rewrites one founder's body rows to the given energy with the
// G3-exact basal spent (base 11 = energy + basal).
func naturalGateV6Seeds(t *testing.T, rewritten sim.EntityID, energy, basal int64) []component.ComponentSeed {
	t.Helper()
	seeds := naturalGateSeeds(t)
	for i := range seeds {
		if seeds[i].Component == NaturalGranaryTypeID && seeds[i].Entity >= 1 && seeds[i].Entity <= 16 {
			seeds[i].Fields = append(seeds[i].Fields, component.FieldSeed{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(0)})
		}
	}
	if rewritten != 0 {
		for i := range seeds {
			if seeds[i].Entity == rewritten && seeds[i].Component == NaturalBodyTypeID {
				for j := range seeds[i].Fields {
					switch seeds[i].Fields[j].Field {
					case NaturalBodyEnergyField:
						seeds[i].Fields[j].Value = sim.IntegerValue(energy)
					case NaturalBodyBasalSpentField:
						seeds[i].Fields[j].Value = sim.IntegerValue(basal)
					}
				}
			}
		}
	}
	return seeds
}

// TestNaturalRunnerPolicyRefusalDivergentBand drives the reply decision in
// the band where the policy consent floor is strictly stronger than the
// contract's frozen one: a consenter at exactly energy 3 whose pair passes
// the contract's entire truth gate resolves as the policy's typed
// ConsentOwn refusal, committed as the two-row rule 512 with zero payment,
// no newborn, and no allocator movement. This is the exact shape that
// crashed the E12 cell before the fix: consulting the contract's birth
// execution made NaturalBirthFor refuse the unallocated newborn candidate 0
// with a bare contract error and killed the run. An energy-2 consenter
// (both floors refuse) pins the agreeing refusal on the same path.
func TestNaturalRunnerPolicyRefusalDivergentBand(t *testing.T) {
	requestAt, err := NaturalPhaseTime(5, NaturalPhaseBirthRequest)
	if err != nil {
		t.Fatal(err)
	}
	replyAt, err := NaturalPhaseTime(5, NaturalPhaseBirthReply)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		energy int64
		basal  int64 // 11 - energy keeps the per-actor G3 identity exact
	}{
		{"divergent band: policy refuses, contract gate would accept", 3, 8},
		{"both floors refuse", 2, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, k := naturalHybridV7(t, naturalGateV6Seeds(t, 2, tc.energy, tc.basal), nil)
			f.pending = &naturalPending{started: make(map[sim.EntityID]strategy.NaturalAction), meals: make(map[sim.EntityID]NaturalMealIntent)}
			head := k.SnapshotHead()
			view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
			// Founder 1 files honestly at the filing floor (E=5 >= TFile=4,
			// granary 3, k=3 capital) to the deterministic successor 2.
			proposal, filed, err := f.birthRequestDecision(view, 5, requestAt, 1)
			if err != nil || !filed {
				t.Fatalf("request 1->2: filed=%v err=%v", filed, err)
			}
			plan, err := k.Plan(proposal, head.Authority)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := k.CommitBatch([]kernel.Plan{plan}); err != nil {
				t.Fatal(err)
			}
			head = k.SnapshotHead()
			view = scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
			replyProposal, attempt, err := f.birthReplyDecision(view, 5, replyAt, 2)
			if err != nil {
				t.Fatalf("reply decision divergent band: %v", err) // pre-fix: bare ErrNaturalContract
			}
			if attempt.Birth.Decision != NaturalReplyRefuse || attempt.Birth.Reason != NaturalRefusalConsentOwn {
				t.Fatalf("reply evidence: %+v", attempt.Birth)
			}
			if attempt.Birth.Newborn != 0 || attempt.Birth.Diverged {
				t.Fatalf("refusal evidence: %+v", attempt.Birth)
			}
			if attempt.Actor != 2 || attempt.Key == "" {
				t.Fatalf("refusal attempt: %+v", attempt)
			}
			// The refusal commits as the two-row rule 512 exactly.
			if replyProposal.Rule != NaturalBirthRefuseRule || len(replyProposal.Patches) != 2 {
				t.Fatalf("refusal proposal: rule %v patches %d", replyProposal.Rule, len(replyProposal.Patches))
			}
			if err := f.guard(replyProposal); err != nil {
				t.Fatal(err)
			}
			plan, err = k.Plan(replyProposal, head.Authority)
			if err != nil {
				t.Fatal(err)
			}
			events, err := k.CommitBatch([]kernel.Plan{plan})
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 || events[0].Rule != NaturalBirthRefuseRule {
				t.Fatalf("refusal events: %+v", events)
			}
			// Zero payment: both parents' paying rows are unchanged, the
			// request is consumed as refused, and no newborn exists.
			head = k.SnapshotHead()
			view = scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
			proposer, err := naturalActorState(view, NaturalMechanismV6, 1)
			if err != nil {
				t.Fatal(err)
			}
			consenter, err := naturalActorState(view, NaturalMechanismV6, 2)
			if err != nil {
				t.Fatal(err)
			}
			if proposer.Granary.Stock != 3 || proposer.Body.Energy != 5 || proposer.Genome.BirthGranaryPaid != 0 ||
				proposer.Request.Status != NaturalRequestRefused {
				t.Fatalf("proposer after refusal: %+v", proposer)
			}
			if consenter.Granary.Stock != 3 || consenter.Body.Energy != tc.energy || consenter.Genome.BirthGranaryPaid != 0 ||
				consenter.Request.LastReplyHour != 5 {
				t.Fatalf("consenter after refusal: %+v", consenter)
			}
			if _, err := naturalActorState(view, NaturalMechanismV6, 2017); err == nil {
				t.Fatal("a refusal allocated a newborn")
			}
			if f.allocator.Last() != 2016 {
				t.Fatalf("allocator moved on a refusal: %d", f.allocator.Last())
			}
		})
	}
}

// TestNaturalBirthForNeverPaysUnaffordableParents pins the frozen
// affordability discipline end to end: the reply's fresh-truth gate refuses
// a below-margin pair with the typed GateThresholds reason before any birth
// is built and mutates nothing, NaturalBirthFor refuses gate-failing rows
// and the unallocated candidate 0 (the exact guard the pre-fix divergent
// path hit), and a pair at exactly the frozen margins affords the 2/2 cost
// on the same rows the gate accepted — gate-passing rows always afford the
// payment, so no birth can commit from rows that cannot pay.
func TestNaturalBirthForNeverPaysUnaffordableParents(t *testing.T) {
	patches := 2
	proposer := naturalGateParent(t, patches, 1)
	consenter := naturalGateParent(t, patches, 2)
	request, err := NaturalBirthRequest(patches, 5, 1, 2, proposer.Worksite.Capital, proposer.Granary.Stock, proposer)
	if err != nil {
		t.Fatal(err)
	}
	// A G2a'-exact below-margin proposer (granary 1): the gate refuses the
	// pair typed, before a birth is built, and mutates nothing.
	poor := request
	poor.Granary.Stock = NaturalParentGranaryCost - 1
	poor.Granary.CultivatedTotal = poor.Granary.Stock + poor.Granary.StoredMeals + poor.Granary.Provisioned + poor.Granary.Spoiled + poor.Genome.BirthGranaryPaid
	draws := naturalDrawsForSeed(t, 0)
	result, err := NaturalBirthReplyFor(NaturalMechanismV5, patches, 5, true, draws, 2017, 1, 2, poor, consenter)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != NaturalReplyRefuse || result.Reason != NaturalRefusalGateThresholds {
		t.Fatalf("gate on sub-cost rows: %+v", result)
	}
	if result.Proposer.Granary.Stock != poor.Granary.Stock || result.Proposer.Body.Energy != poor.Body.Energy {
		t.Fatalf("refusal mutated the paying rows: %+v", result.Proposer)
	}
	// The birth itself refuses gate-failing rows, the unaffordable spell,
	// and the unallocated candidate 0 — all typed ErrNaturalContract.
	child, _, err := NaturalChildGenome(patches, 1, 2, poor.Genome, consenter.Genome, 5, draws)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NaturalBirthFor(NaturalMechanismV5, patches, 5, true, 2017, 1, 2, poor, consenter, child); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("below-margin pair admitted: %v", err)
	}
	if _, err := NaturalBirthFor(NaturalMechanismV5, patches, 5, true, 0, 1, 2, poor, consenter, child); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("unallocated newborn candidate admitted: %v", err)
	}
	// A pair at exactly the frozen margins (granary 3, energy 3) affords the
	// 2/2 cost on the gate-accepted rows and pays symmetrically.
	margin := naturalGateParent(t, patches, 1)
	margin.Granary.Stock = NaturalConsentMinGranary
	margin.Granary.CultivatedTotal = NaturalConsentMinGranary
	margin.Body.Energy = NaturalConsentMinEnergy
	margin.Body.BasalSpent = 11 - NaturalConsentMinEnergy
	marginRequest, err := NaturalBirthRequest(patches, 5, 1, 2, margin.Worksite.Capital, margin.Granary.Stock, margin)
	if err != nil {
		t.Fatal(err)
	}
	marginConsenter := naturalGateParent(t, patches, 2)
	marginConsenter.Granary.Stock = NaturalConsentMinGranary
	marginConsenter.Granary.CultivatedTotal = NaturalConsentMinGranary
	marginConsenter.Body.Energy = NaturalConsentMinEnergy
	marginConsenter.Body.BasalSpent = 11 - NaturalConsentMinEnergy
	birth, err := NaturalBirthFor(NaturalMechanismV5, patches, 5, true, 2017, 1, 2, marginRequest, marginConsenter, child)
	if err != nil {
		t.Fatalf("margin pair refused: %v", err)
	}
	if birth.Proposer.Granary.Stock != NaturalConsentMinGranary-NaturalParentGranaryCost ||
		birth.Proposer.Body.Energy != NaturalConsentMinEnergy-NaturalParentEnergyCost ||
		birth.Proposer.Genome.BirthGranaryPaid != NaturalParentGranaryCost {
		t.Fatalf("margin proposer after paying: %+v", birth.Proposer)
	}
	if birth.Consenter.Body.Energy != NaturalConsentMinEnergy-NaturalParentEnergyCost {
		t.Fatalf("margin consenter after paying: %+v", birth.Consenter)
	}
}

// TestNaturalFixtureE12FoundingBandBirthsComplete reproduces the exact
// failing E12 configuration (founder-tenure fixture, f=12, natural/7,
// mechanism v6, seed 0, one worker, 40 committed hours) and asserts the run
// completes with green hourly conservation. The chosen semantics is
// completion: a policy refusal in the divergent band is a normal typed
// outcome (rule 512), not a run-ending refusal. The same run pins the
// single-payment-per-reply-instant discipline: at most one birth commits per
// reply instant, every same-instant accept loser carries the typed
// allocation-collision evidence, all same-instant accept decisions computed
// the same newborn candidate, and the accepted decisions partition exactly
// into committed births plus typed losers — so two parents can never pay
// twice in one reply instant, proven hourly by conservation.
func TestNaturalFixtureE12FoundingBandBirthsComplete(t *testing.T) {
	f, err := NewNaturalFixtureForConfig(2, 8, 0, true, 1, strategy.FrozenNaturalPolicy7().Ref, NaturalMechanismV6, 12)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 41) // fixture checkpoints start at hour 2: 40 committed hours
	naturalAssertConservationForMechanism(t, f, NaturalMechanismV6)
	naturalAssertJournalClock(t, f)
	checks := f.Checkpoints()
	if len(checks) < 40 {
		t.Fatalf("E12 horizon: %d checkpoints", len(checks))
	}
	last := checks[len(checks)-1]
	if last.Balance.Births < 1 {
		t.Fatalf("no committed birth in the E12 cell: %+v", last.Balance)
	}
	if last.Metrics.BirthRequests < 1 {
		t.Fatalf("no birth filing in the E12 cell: %+v", last.Metrics)
	}
	committedPerInstant := make(map[sim.SimTime]int)
	committedKeys := make(map[string]bool)
	for _, ev := range f.k.Events() {
		if ev.Rule == NaturalBirthReplyRule && ev.Kind == kernel.KindEntityCreate {
			committedPerInstant[ev.Time]++
			committedKeys[ev.Key] = true
		}
	}
	for at, n := range committedPerInstant {
		if n != 1 {
			t.Fatalf("reply instant %d committed %d births", at, n)
		}
	}
	acceptsPerInstant := make(map[sim.SimTime]int)
	candidatesPerInstant := make(map[sim.SimTime]sim.EntityID)
	accepted, lost := 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Kind != "birth-reply" || a.Birth.Decision != NaturalReplyAccept {
				continue
			}
			accepted++
			acceptsPerInstant[a.Time]++
			if prev, ok := candidatesPerInstant[a.Time]; ok && prev != a.Birth.Newborn {
				t.Fatalf("instant %d computed two newborn candidates: %d and %d", a.Time, prev, a.Birth.Newborn)
			}
			candidatesPerInstant[a.Time] = a.Birth.Newborn
			if a.Lost != "" {
				lost++
			}
		}
	}
	if accepted != len(committedKeys)+lost {
		t.Fatalf("accepted birth decisions %d != committed %d + lost %d", accepted, len(committedKeys), lost)
	}
	crossed := false
	for _, n := range acceptsPerInstant {
		if n >= 2 {
			crossed = true
		}
	}
	if !crossed {
		t.Fatal("no crossed-pair reply instant in the E12 cell: the single-payment discipline is untested here")
	}
	if f.refusals[NaturalRefusalConsentOwn] == 0 {
		t.Fatal("no typed consent-own refusal recorded in the E12 cell")
	}
}

// TestNaturalFixtureFoundingBandBracketsComplete walks the f=11 and f=13
// bracket cells of the founding band: both complete the same 40 committed
// hours of the exact failing configuration with green hourly conservation.
func TestNaturalFixtureFoundingBandBracketsComplete(t *testing.T) {
	for _, founders := range []int{11, 13} {
		f, err := NewNaturalFixtureForConfig(2, 8, 0, true, 1, strategy.FrozenNaturalPolicy7().Ref, NaturalMechanismV6, founders)
		if err != nil {
			t.Fatal(err)
		}
		naturalRunTo(t, f, 41)
		naturalAssertConservationForMechanism(t, f, NaturalMechanismV6)
		naturalAssertJournalClock(t, f)
		checks := f.Checkpoints()
		if len(checks) < 40 {
			t.Fatalf("f=%d horizon: %d checkpoints", founders, len(checks))
		}
	}
}
