package world

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The VS2 lane L2 tests: the generalized escalation gate (the
// protocol.EscalationSelector seam over the claim-wake roster), the
// construction-time refusal of a priority policy without its selector, and the
// additive cognition-block wire binding of the priority-policy identity
// (VS2-EC-2 / VS2-EC-5).

// allRosterSelector selects every candidate of every hour, so a run wired with
// it reproduces the VS1 escalation set.
type allRosterSelector struct{}

func (allRosterSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	selected := make([]sim.EntityID, 0, len(roster))
	for _, c := range roster {
		selected = append(selected, c.Actor)
	}
	return protocol.Selection{Hour: hour, Selected: selected, Budget: budget}, nil
}

// scriptedEscalationSelector records every Select call and returns the actor
// set selectFn derives from the hour and roster.
type scriptedEscalationSelector struct {
	mu       sync.Mutex
	hours    []int
	rosters  [][]protocol.EscalationCandidate
	budgets  []protocol.BudgetState
	selectFn func(hour int, roster []protocol.EscalationCandidate) []sim.EntityID
}

func (s *scriptedEscalationSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	s.mu.Lock()
	s.hours = append(s.hours, hour)
	s.rosters = append(s.rosters, append([]protocol.EscalationCandidate(nil), roster...))
	s.budgets = append(s.budgets, budget)
	s.mu.Unlock()
	var selected []sim.EntityID
	if s.selectFn != nil {
		selected = s.selectFn(hour, roster)
	}
	return protocol.Selection{Hour: hour, Selected: selected, Budget: budget}, nil
}

// naturalRequest pairs a captured seam request's actor and hour.
type naturalRequest struct {
	Actor sim.EntityID
	Hour  int
}

func capturedRequests(s *capturingSeam) []naturalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]naturalRequest, 0, len(s.reqs))
	for _, req := range s.reqs {
		out = append(out, naturalRequest{Actor: req.Actor, Hour: req.Hour})
	}
	return out
}

func priorityPolicy(seed uint64) *protocol.PriorityPolicy {
	return &protocol.PriorityPolicy{
		Strategy: protocol.StrategyAdaptive, ExplorationFraction: 250000,
		RandSeed: seed, CallCeiling: 256,
	}
}

// TestNaturalCognitionRequiresSelectorForPriorityPolicy pins the
// construction-time refusal: a priority policy without its selector is a typed
// ErrNaturalRunner, so a budgeted arm can never silently fall back to the VS1
// hour gate (VS2-EC-2).
func TestNaturalCognitionRequiresSelectorForPriorityPolicy(t *testing.T) {
	wake := protocol.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0, Priority: priorityPolicy(3)}

	halfWired := naturalCognitionTestOptions()
	halfWired.Cognition = &NaturalCognition{Seam: stubClaimSeam{}, WakePolicy: wake}
	if _, err := NewNatural(halfWired); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("priority policy without selector: err = %v, want ErrNaturalRunner", err)
	}

	full := naturalCognitionTestOptions()
	full.Cognition = &NaturalCognition{Seam: stubClaimSeam{}, WakePolicy: wake, Selector: allRosterSelector{}}
	if _, err := NewNatural(full); err != nil {
		t.Fatalf("priority policy with selector: %v", err)
	}
}

// TestNaturalSelectorGateReproducesVS1EscalationPath is the generalized-gate
// equivalence pin: with a selector that applies the same hour gate as the
// policy and escalates every candidate, the wired path reproduces the
// nil-selector VS1 path (same seam requests, same recorded history), and it
// receives the per-hour roster. Deleting the roster feed or the selection
// routing fails this.
func TestNaturalSelectorGateReproducesVS1EscalationPath(t *testing.T) {
	wake := protocol.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0} // nil Priority: the VS1 path

	vs1Seam := &capturingSeam{}
	vs1Opts := naturalCognitionTestOptions()
	vs1Opts.Cognition = &NaturalCognition{Seam: vs1Seam, WakePolicy: wake}
	vs1, err := NewNatural(vs1Opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, vs1, 12)

	selector := &scriptedEscalationSelector{selectFn: func(hour int, roster []protocol.EscalationCandidate) []sim.EntityID {
		if !wake.Escalates(hour) {
			return nil
		}
		selected := make([]sim.EntityID, 0, len(roster))
		for _, c := range roster {
			selected = append(selected, c.Actor)
		}
		return selected
	}}
	selSeam := &capturingSeam{}
	selOpts := naturalCognitionTestOptions()
	selOpts.Cognition = &NaturalCognition{Seam: selSeam, WakePolicy: wake, Selector: selector}
	gated, err := NewNatural(selOpts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, gated, 12)

	want := capturedRequests(vs1Seam)
	if len(want) == 0 {
		t.Fatal("the VS1 path captured no request; the test is vacuous")
	}
	if got := capturedRequests(selSeam); !reflect.DeepEqual(got, want) {
		t.Fatalf("selector gate diverged from the VS1 escalation set:\n selector=%v\n      vs1=%v", got, want)
	}
	if !reflect.DeepEqual(vs1.Checkpoints(), gated.Checkpoints()) {
		t.Fatal("the selector gate changed the recorded history")
	}
}

// TestNaturalSelectorSeamDrivesEscalation drives a real roster: the selector
// picks only the lowest-id claim-wake actor on even hours, and only those
// actor-hours reach the seam. It also pins the declared budget seeding and the
// roster's per-hour actor order.
func TestNaturalSelectorSeamDrivesEscalation(t *testing.T) {
	wake := protocol.WakePolicy{Name: "periodic", PeriodHours: 1, PhaseHours: 0, Priority: priorityPolicy(5)}
	selector := &scriptedEscalationSelector{selectFn: func(hour int, roster []protocol.EscalationCandidate) []sim.EntityID {
		if hour%2 != 0 || len(roster) == 0 {
			return nil
		}
		return []sim.EntityID{roster[0].Actor}
	}}
	seam := &capturingSeam{}
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: wake, Selector: selector}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 12)

	requests := capturedRequests(seam)
	if len(requests) == 0 {
		t.Fatal("no escalation reached the seam")
	}

	selector.mu.Lock()
	defer selector.mu.Unlock()
	if len(selector.hours) == 0 {
		t.Fatal("the selector was never consulted")
	}
	selectedActor := make(map[int]sim.EntityID)
	for i, hour := range selector.hours {
		roster := selector.rosters[i]
		for j := 1; j < len(roster); j++ {
			if roster[j].Actor < roster[j-1].Actor {
				t.Fatalf("hour %d roster not sorted by actor", hour)
			}
		}
		for _, c := range roster {
			if c.Hour != hour {
				t.Fatalf("roster candidate hour %d, want %d", c.Hour, hour)
			}
		}
		if selector.budgets[i].CallsBudget != 256 || selector.budgets[i].CallsRemaining != 256 {
			t.Fatalf("hour %d budget %+v, want the declared ceiling 256", hour, selector.budgets[i])
		}
		if hour%2 == 0 && len(roster) > 0 {
			selectedActor[hour] = roster[0].Actor
		}
	}
	for _, req := range requests {
		if req.Hour%2 != 0 {
			t.Fatalf("escalation at a non-selected odd hour %d", req.Hour)
		}
		if got, ok := selectedActor[req.Hour]; !ok || got != req.Actor {
			t.Fatalf("hour %d: escalated actor %d, want the selected %d (selected=%d ok=%t)", req.Hour, req.Actor, got, got, ok)
		}
	}
}

// TestNaturalCandidateFeaturesObservationOnly pins the declared C.1 rows: the
// observation-only rows come from the actor's own observation (through the
// protocol projection), the scheduler-level prior_reliability row is the
// declared world-level constant, and escalation_requested is the actor's own
// predicate result.
func TestNaturalCandidateFeaturesObservationOnly(t *testing.T) {
	obs := strategy.NaturalObservation{
		Energy: sim.IntegerValue(7), Hunger: sim.IntegerValue(3), Capital: sim.IntegerValue(5),
		GranaryStock: sim.IntegerValue(2), LastStoredMealHour: sim.IntegerValue(9),
		LastDenial: strategy.NaturalGatherDenied,
		HomeSlots: []strategy.NaturalSlotView{
			{ID: 1, Stock: sim.IntegerValue(1)},
			{ID: 2, Stock: sim.IntegerValue(0)},
			{ID: 3, Stock: sim.IntegerValue(1)},
		},
	}
	got, _, _ := naturalCandidateFeatures(protocol.DeclaredEscalationPredicate(), obs)
	want := protocol.ObservationFeatures(obs)
	want.PriorReliability = protocol.DeclaredPriorReliability
	if requested, _ := protocol.EscalationRequest(protocol.DeclaredEscalationPredicate(), obs); requested {
		want.EscalationRequested = 1
	}
	if got != want {
		t.Fatalf("candidate features = %+v, want %+v", got, want)
	}
	if got.Energy != 7 || got.Hunger != 3 || got.Capital != 5 || got.GranaryStock != 2 ||
		got.LastStoredMealHour != 9 || got.StockedHomeSlots != 2 {
		t.Fatalf("raw rows = %+v", got)
	}
	if got.LastDenial != "gather" {
		t.Fatalf("last_denial = %q, want gather", got.LastDenial)
	}
	if got.PriorReliability != protocol.DeclaredPriorReliability {
		t.Fatalf("prior_reliability = %d, want the declared world-level %d", got.PriorReliability, protocol.DeclaredPriorReliability)
	}
	if got.Urgency == 0 || got.Impact == 0 || got.PropagationRisk == 0 || got.DecisionSensitivity == 0 {
		t.Fatalf("derived rows are vacuous: %+v", got)
	}
}

// TestNaturalFeatureThresholdConstantsCrossCheck ties the protocol's restated
// frozen claim thresholds to the world's exported frozen constants, so the
// derivation can never silently drift from the substrate.
func TestNaturalFeatureThresholdConstantsCrossCheck(t *testing.T) {
	checks := []struct {
		name     string
		protocol int64
		world    int64
	}{
		{"energy capacity", protocol.FeatureEnergyCapacity, CapacityEnergyCapacity},
		{"hunger capacity", protocol.FeatureHungerCapacity, CapacityHungerCapacity},
		{"slots per patch", protocol.FeatureSlotsPerPatch, CapacitySlotsPerPatch},
		{"build energy floor", protocol.FeatureBuildEnergyFloor, CapacityPolicyTLow},
		{"consent min energy", protocol.FeatureConsentMinEnergy, NaturalConsentMinEnergy},
		{"consent min capital", protocol.FeatureConsentMinCapital, NaturalConsentMinCapital},
		{"consent min granary", protocol.FeatureConsentMinGranary, NaturalConsentMinGranary},
	}
	for _, c := range checks {
		if c.protocol != c.world {
			t.Fatalf("%s: protocol=%d, world=%d", c.name, c.protocol, c.world)
		}
	}
}

// TestNaturalEscalationRosterCarriesDerivedFeatures pins that the world roster
// carries the derived feature rows, the declared world-level prior, and the
// actor's own escalation request (with its typed reason), so the value model
// can rank and the request is auditable.
func TestNaturalEscalationRosterCarriesDerivedFeatures(t *testing.T) {
	obs := strategy.NaturalObservation{
		Energy: sim.IntegerValue(5), Hunger: sim.IntegerValue(24),
		Capital: sim.IntegerValue(2), GranaryStock: sim.IntegerValue(3),
		LastStoredMealHour: sim.IntegerValue(5), Hour: 5,
		HomeSlots: []strategy.NaturalSlotView{{ID: 2001, Stock: sim.IntegerValue(1)}},
	}
	roster := naturalEscalationRoster(5, []naturalClaimWake{{id: 3, obs: obs}})
	if len(roster) != 1 {
		t.Fatalf("roster = %+v", roster)
	}
	c := roster[0]
	if c.Features.PriorReliability != protocol.DeclaredPriorReliability {
		t.Fatalf("prior_reliability = %d", c.Features.PriorReliability)
	}
	if !c.Requested || c.Reason != protocol.EscalationRequestedByActor || c.Features.EscalationRequested != 1 {
		t.Fatalf("escalation request not carried: requested=%t reason=%q bit=%d", c.Requested, c.Reason, c.Features.EscalationRequested)
	}
	if c.Features.Urgency == 0 || c.Features.DecisionSensitivity == 0 || c.Features.Impact == 0 {
		t.Fatalf("derived rows are vacuous on the roster: %+v", c.Features)
	}
}

// TestNaturalCognitiveBlockPriorityIdentityRoundTrip pins the additive
// cognition-block wire extension: a priority-bearing policy's canonical
// identity is carried and reconstructed, so the fingerprint is recomputed over
// the full policy, and the declared policy verifies against it.
func TestNaturalCognitiveBlockPriorityIdentityRoundTrip(t *testing.T) {
	policy := protocol.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0, Priority: &protocol.PriorityPolicy{
		Strategy: protocol.StrategyAdaptive, ExplorationFraction: 250000, RandSeed: 11,
		ValueModel: protocol.ValueModelSpec{
			Version: "vs2-value/1",
			Weights: []protocol.FeatureWeight{{Feature: "urgency", Weight: 3}, {Feature: "escalation_requested", Weight: 5}},
		},
		CallCeiling: 256,
	}}
	block := naturalCognitiveBlock{
		WakePolicy: policy, WakeFingerprint: policy.Fingerprint(),
		BaseRef: strategy.FrozenNaturalPolicy7().Ref, HeadRef: strategy.FrozenNaturalPolicy7().Ref, ApplyHour: 24,
	}
	wire, err := encodeNaturalCognitiveBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	decoded, consumed, err := decodeNaturalCognitiveBlock(wire)
	if err != nil || consumed != len(wire) {
		t.Fatalf("decode: err=%v consumed=%d len=%d", err, consumed, len(wire))
	}
	if decoded.WakePolicy.Priority == nil {
		t.Fatal("the priority identity was not carried")
	}
	if !reflect.DeepEqual(*decoded.WakePolicy.Priority, *policy.Priority) {
		t.Fatalf("priority = %+v, want %+v", *decoded.WakePolicy.Priority, *policy.Priority)
	}
	if decoded.WakePolicy.Fingerprint() != block.WakeFingerprint {
		t.Fatal("the fingerprint does not recompute over the full decoded policy")
	}
	if err := verifyCognitiveWakePolicy(&decoded, policy); err != nil {
		t.Fatalf("verify with the declared policy: %v", err)
	}
}

// TestNaturalCognitiveBlockNilPriorityBytesUnchanged pins VS2-EC-1 on the wire:
// a nil-Priority (VS1) block carries no trailing priority section, so the
// pre-extension bytes are unchanged; a priority-bearing block is longer by
// exactly the length-prefixed canonical identity.
func TestNaturalCognitiveBlockNilPriorityBytesUnchanged(t *testing.T) {
	ref := strategy.FrozenNaturalPolicy7().Ref
	vs1 := protocol.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0}
	vs1Block := naturalCognitiveBlock{WakePolicy: vs1, WakeFingerprint: vs1.Fingerprint(), BaseRef: ref, HeadRef: ref}
	vs1Wire, err := encodeNaturalCognitiveBlock(vs1Block)
	if err != nil {
		t.Fatal(err)
	}
	decoded, consumed, err := decodeNaturalCognitiveBlock(vs1Wire)
	if err != nil || consumed != len(vs1Wire) {
		t.Fatalf("decode VS1 block: err=%v consumed=%d len=%d", err, consumed, len(vs1Wire))
	}
	if decoded.WakePolicy.Priority != nil {
		t.Fatal("a nil-Priority block decoded with a priority policy")
	}
	if decoded.WakePolicy.Identity() != vs1.Identity() {
		t.Fatalf("nil-Priority identity changed: %s", decoded.WakePolicy.Identity())
	}

	priorityBlock := vs1Block
	priorityBlock.WakePolicy.Priority = priorityPolicy(1)
	priorityBlock.WakeFingerprint = priorityBlock.WakePolicy.Fingerprint()
	priorityWire, err := encodeNaturalCognitiveBlock(priorityBlock)
	if err != nil {
		t.Fatal(err)
	}
	wantDelta := 4 + len(priorityBlock.WakePolicy.Identity())
	if got := len(priorityWire) - len(vs1Wire); got != wantDelta {
		t.Fatalf("priority block added %d bytes, want exactly the identity section %d", got, wantDelta)
	}
}

// TestNaturalVerifyCognitiveWakePolicy pins the restore-time refusal: the
// decoded policy must reproduce the recorded fingerprint and a supplied policy,
// when present, must match it; a mismatch is refused.
func TestNaturalVerifyCognitiveWakePolicy(t *testing.T) {
	vs1 := protocol.VS1WakePolicy()
	other := protocol.FullWakePolicy()
	block := &naturalCognitiveBlock{WakePolicy: vs1, WakeFingerprint: vs1.Fingerprint()}

	if err := verifyCognitiveWakePolicy(nil, protocol.WakePolicy{}); err != nil {
		t.Fatalf("nil block: %v", err)
	}
	if err := verifyCognitiveWakePolicy(block, protocol.WakePolicy{}); err != nil {
		t.Fatalf("no supplied policy: %v", err)
	}
	if err := verifyCognitiveWakePolicy(block, vs1); err != nil {
		t.Fatalf("matching supplied policy: %v", err)
	}
	if err := verifyCognitiveWakePolicy(block, other); !errors.Is(err, ErrNaturalCognitiveBlock) {
		t.Fatalf("mismatched supplied policy: err = %v, want ErrNaturalCognitiveBlock", err)
	}
	tampered := *block
	tampered.WakePolicy = other
	if err := verifyCognitiveWakePolicy(&tampered, protocol.WakePolicy{}); !errors.Is(err, ErrNaturalCognitiveBlock) {
		t.Fatalf("tampered block: err = %v, want ErrNaturalCognitiveBlock", err)
	}
}

// TestNaturalSeamRestoreRefusesMismatchedWakePolicy is the end-to-end
// restore-time binding: a saved VS1 bundle restores under the recorded policy
// but is refused under a different supplied policy (VS2-EC-5).
func TestNaturalSeamRestoreRefusesMismatchedWakePolicy(t *testing.T) {
	f, ledger, _, ledgerPath := seamReplayTestRunner(t)
	summary, err := ledger.Summary()
	if err != nil {
		t.Fatal(err)
	}
	f.SetCognitiveCheckpoint(&NaturalCognitiveCheckpoint{
		WakePolicy: cognition.VS1WakePolicy(), LedgerDigest: summary.Digest, LedgerCount: summary.Count,
	})
	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, cognition.VS1WakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "wake-binding.bundle")
	if _, err := f.SaveResumeCheckpointWithReplay(bundle, replaySeam, cognition.VS1WakePolicy()); err != nil {
		t.Fatal(err)
	}

	base := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveReplaySeam: replaySeam, CognitiveLedgerPath: ledgerPath}

	mismatched := base
	mismatched.CognitiveReplayWake = cognition.FullWakePolicy()
	if _, _, err := RestoreNaturalCheckpoint(bundle, mismatched); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("restore under a mismatched wake policy: err = %v, want ErrNaturalCheckpoint", err)
	}

	matched := base
	matched.CognitiveReplayWake = cognition.VS1WakePolicy()
	if _, _, err := RestoreNaturalCheckpoint(bundle, matched); err != nil {
		t.Fatalf("restore under the recorded wake policy: %v", err)
	}
}
