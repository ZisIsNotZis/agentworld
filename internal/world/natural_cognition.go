package world

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role A declared decision seam in the runner (spec section 4). The seam
// is optional and nil-tolerant: with it absent, allocate and evaluate take the
// frozen path and every recorded run stays byte-identical. The seam is
// integrated in two phases with exactly one model call per escalated claim
// decision:
//
//  1. Allocation phase (coordinator, before the batch, on the batch's common
//     snapshot): for each claim-wake actor selected by the wake policy, build
//     the observation, record the model call, validate it, and use the
//     resulting action in place of the frozen cascade's choice.
//  2. Evaluation phase (per-actor batch step): the claim case consumes the
//     decision recorded in phase 1 from runner state; it never calls the model
//     again and never re-derives the decision.
//
// The world never imports the network transport: the seam is an interface
// typed with the cognition protocol types, and the concrete recorded seam is
// wired by the caller.
//
// ClaimSeam is the Role A half of the cognition protocol the runner consumes:
// the subset of protocol.SeamDecider that decides the claim-instant action.
// The Role B proposal method belongs to the patch lane.
type ClaimSeam interface {
	DecideActorSeam(ctx context.Context, req protocol.ActorSeamRequest) (protocol.ActorSeamDecision, error)
}

// NaturalCognition wires the optional Role A seam into the runner. Both fields
// are required when it is non-nil. The wake policy is part of run identity;
// the escalation set is a pure function of (policy, hour, roster) and consumes
// no world random stream.
type NaturalCognition struct {
	Seam       ClaimSeam
	WakePolicy protocol.WakePolicy
	// Selector is optional. When non-nil the runner-provided online
	// accumulator (protocol.EscalationSelector) owns the policy PRNG, the
	// spent-call count, and the budget state; when nil the world uses the pure
	// WakePolicy.Select (or the VS1 hour gate when Priority is nil). Either way
	// the escalation set is reproducible and consumes no world random stream.
	Selector protocol.EscalationSelector
	// ClaimConcurrency is the declared bound on Role A calls issued at the
	// same time for one simulated hour (ticket 57). The decisions are
	// independent — each request is built from its actor's own observation
	// before any call is issued — so they may be decided concurrently, but the
	// responses are always APPLIED serially in the existing wake order, which
	// keeps the world state, accepted history and journals byte-identical to
	// the serial run. Zero or one means serial. It applies only to the
	// no-Priority wake policy (the R/FullWake reference path): a
	// priority-bearing arm (P/U/A) keeps its byte-for-byte serial selection and
	// call sequence, and a run with no seam (A0) makes no calls at all.
	ClaimConcurrency int
}

// validate refuses a half-wired seam: entering the seam without a decider or a
// declared wake policy is a construction-time error, not a runtime surprise. A
// priority-bearing policy without its escalation selector is refused too, so a
// budgeted arm can never silently degrade to the VS1 hour gate (VS2-EC-2).
func (c *NaturalCognition) validate() error {
	if c == nil {
		return nil
	}
	if c.Seam == nil {
		return fmt.Errorf("%w: cognition seam is nil", ErrNaturalRunner)
	}
	if err := c.WakePolicy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrNaturalRunner, err)
	}
	if c.WakePolicy.Priority != nil && c.Selector == nil {
		return fmt.Errorf("%w: priority policy requires an escalation selector", ErrNaturalRunner)
	}
	if c.ClaimConcurrency < 0 {
		return fmt.Errorf("%w: claim concurrency %d is negative", ErrNaturalRunner, c.ClaimConcurrency)
	}
	return nil
}

// naturalClaimWake is one claim-wake actor and the frozen observation the
// claim step computes: the exact set VS1's allocate enumerates (a
// WakeNeedThreshold or WakeBirth wake at the claim instant).
type naturalClaimWake struct {
	id    sim.EntityID
	actor NaturalActorState
	obs   strategy.NaturalObservation
}

// naturalClaimActions is the escalated action set: a property of the seam and
// the world mechanism, never of the bound behaviour ref. Provision is present
// only on a mechanism whose rule-515 provision contract is admitted (v6/v7).
func naturalClaimActions(mechanism NaturalMechanism) []string {
	if mechanism == NaturalMechanismV6 || mechanism == NaturalMechanismV7 || naturalHasProvisionLedger(mechanism) {
		return []string{"gather", "eat-stored", "provision", "wait"}
	}
	return []string{"gather", "eat-stored", "wait"}
}

// naturalSeamSelection resolves the actors the declared wake policy escalates
// for one candidate hour. It generalizes the VS1 hour-only gate
// (WakePolicy.Escalates) to the per-roster selection of the protocol surface: a
// wired EscalationSelector (a VS2 budgeted arm) owns the selection, and the
// degenerate WakePolicy.Select serves the nil-Priority path (the full
// reference and the VS1 path). A nil cognition selects nothing, so the frozen
// no-cognition path is byte-identical. The selection is a pure function of the
// policy, the hour, the roster, and the declared budget state: it consumes no
// world random stream.
func (f *Natural) naturalSeamSelection(hour int, roster []protocol.EscalationCandidate) (map[sim.EntityID]bool, error) {
	if f.cognition == nil {
		return nil, nil
	}
	budget := f.naturalSeamBudget()
	var (
		selection protocol.Selection
		err       error
	)
	if f.cognition.Selector != nil {
		selection, err = f.cognition.Selector.Select(hour, roster, budget)
	} else {
		selection, err = f.cognition.WakePolicy.Select(hour, roster, budget)
	}
	if err != nil {
		return nil, err
	}
	selected := make(map[sim.EntityID]bool, len(selection.Selected))
	for _, id := range selection.Selected {
		selected[id] = true
	}
	return selected, nil
}

// naturalSeamBudget is the declared budget view of one hour's selection: the
// declared call ceiling N of a priority policy, or the zero budget on the
// nil-Priority VS1 path. The selector owns the mutating spent-call count; this
// is the declared ceiling the selection is seeded from, not run state.
func (f *Natural) naturalSeamBudget() protocol.BudgetState {
	if f.cognition == nil || f.cognition.WakePolicy.Priority == nil {
		return protocol.BudgetState{}
	}
	ceiling := f.cognition.WakePolicy.Priority.CallCeiling
	return protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling}
}

// naturalEscalationRoster builds the per-hour escalation candidate roster from
// the claim-wake set, in the declared order (ascending actor id, spec section
// 4.5/8.1). Each candidate carries the actor's own observation-derived feature
// vector plus the declared world-level prior-reliability row and the actor's
// own escalation-request bit; the roster is a pure read of the frozen
// observation and mutates nothing.
func naturalEscalationRoster(hour int, wakes []naturalClaimWake) []protocol.EscalationCandidate {
	predicate := protocol.DeclaredEscalationPredicate()
	roster := make([]protocol.EscalationCandidate, 0, len(wakes))
	for _, w := range wakes {
		features, requested, reason := naturalCandidateFeatures(predicate, w.obs)
		roster = append(roster, protocol.EscalationCandidate{
			Actor: w.id, Hour: hour, Features: features, Requested: requested, Reason: reason,
		})
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].Actor < roster[j].Actor })
	return roster
}

// naturalCandidateFeatures derives a candidate's declared feature rows
// (Appendix C.1) from the actor's own claim observation and the declared
// escalation predicate. The observation-only rows (including the derived
// decision_sensitivity/urgency/impact/propagation_risk and the last_denial
// encoding) come from protocol.ObservationFeatures; the one scheduler-level row,
// prior_reliability, is the declared world-level cell statistic (which the §4.3
// predicate never reads); escalation_requested is the actor's own predicate
// result, returned with its typed reason for the candidate record.
func naturalCandidateFeatures(predicate protocol.PolicyPredicate, obs strategy.NaturalObservation) (protocol.ValueFeatures, bool, protocol.EscalationReason) {
	features := protocol.ObservationFeatures(obs)
	features.PriorReliability = protocol.DeclaredPriorReliability
	requested, reason := protocol.EscalationRequest(predicate, obs)
	if requested {
		features.EscalationRequested = 1
	}
	return features, requested, reason
}

// naturalClaimConcurrency is the effective Role A call concurrency of one hour:
// the declared ClaimConcurrency when it is at least two and the wake policy has
// no priority layer, otherwise the serial one. A priority-bearing arm keeps its
// byte-for-byte serial path, and a world with no cognition makes no calls.
func (f *Natural) naturalClaimConcurrency() int {
	if f.cognition == nil || f.cognition.WakePolicy.Priority != nil {
		return 1
	}
	if f.cognition.ClaimConcurrency < 2 {
		return 1
	}
	return f.cognition.ClaimConcurrency
}

// naturalSeamRequest builds the complete Role A request of one selected
// actor-hour, including the frozen cascade's own action as the comparison
// baseline for the load-bearing check (EC-2). The baseline is carried for the
// intervention record; it is not part of the canonical request bytes. Building
// a request is a pure read, so every request of an hour can be built before any
// call is issued.
func (f *Natural) naturalSeamRequest(id sim.EntityID, hour int, version sim.WorldVersion, actor NaturalActorState, obs strategy.NaturalObservation, choice *strategy.NaturalChoice) protocol.ActorSeamRequest {
	request := f.naturalActorSeamRequest(id, hour, version, actor, obs)
	request.FrozenAction = naturalChoiceAction(choice.Kind)
	return request
}

// naturalSeamDecide issues one recorded Role A call. It is the decide half of
// the old decide+apply pair: the world is untouched, so it is safe to run for
// every selected actor of an hour at the same time.
func (f *Natural) naturalSeamDecide(ctx context.Context, request protocol.ActorSeamRequest) (protocol.ActorSeamDecision, error) {
	return f.cognition.Seam.DecideActorSeam(ctx, request)
}

// naturalSeamApply is the apply half: it replaces the frozen claim choice with
// the seam decision for a selected actor-hour and records the effective choice
// for the evaluation phase. A level-1 (malformed/inadmissible) fallback leaves
// the frozen choice in place, mutating nothing. It always runs serially in the
// declared wake order, never inside a concurrent decide.
func (f *Natural) naturalSeamApply(id sim.EntityID, obs strategy.NaturalObservation, choice *strategy.NaturalChoice, decision protocol.ActorSeamDecision) error {
	if decision.Source == protocol.SourceModel {
		if err := naturalApplySeamAction(choice, decision.Action, obs); err != nil {
			return err
		}
	}
	f.claimDecisions[id] = *choice
	return nil
}

// naturalSeamOverride replaces the frozen claim choice with the seam decision
// for a selected actor-hour and records the effective choice for the
// evaluation phase. The caller selects the actor through naturalSeamSelection,
// so a nil seam never reaches it. A level-3 transport failure is returned to
// the caller as a fail-loud halt; a level-1 (malformed/inadmissible) failure
// returns the declared fallback and leaves the frozen choice in place, mutating
// nothing. It is the serial path: decide then apply, one actor at a time.
func (f *Natural) naturalSeamOverride(ctx context.Context, id sim.EntityID, hour int, version sim.WorldVersion, actor NaturalActorState, obs strategy.NaturalObservation, choice *strategy.NaturalChoice) error {
	decision, err := f.naturalSeamDecide(ctx, f.naturalSeamRequest(id, hour, version, actor, obs, choice))
	if err != nil {
		return err
	}
	return f.naturalSeamApply(id, obs, choice, decision)
}

// naturalSeamDecideConcurrent decides the hour's selected requests under a
// bounded worker pool (never more than the declared concurrency, never more
// than the pending count) and returns each decision at its wake index. The
// first failure cancels the remaining work and is returned as the fail-loud
// halt, so a typed ceiling hit or a transport failure stops the run exactly
// once; nothing here mutates the world.
func (f *Natural) naturalSeamDecideConcurrent(ctx context.Context, requests []protocol.ActorSeamRequest, pending []int) ([]protocol.ActorSeamDecision, error) {
	decisions := make([]protocol.ActorSeamDecision, len(requests))
	if len(pending) == 0 {
		return decisions, nil
	}
	workers := f.naturalClaimConcurrency()
	if workers > len(pending) {
		workers = len(pending)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)
	queue := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				decision, err := f.naturalSeamDecide(runCtx, requests[i])
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					mu.Unlock()
					continue
				}
				decisions[i] = decision
			}
		}()
	}
	for _, i := range pending {
		queue <- i
	}
	close(queue)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return decisions, nil
}

// naturalChoiceAction is the claim action string of a frozen choice kind. The
// escalated seam admits only the four declared actions; an unexpected kind is
// recorded as the empty string (never silently mapped).
func naturalChoiceAction(kind strategy.NaturalAction) string {
	switch kind {
	case strategy.NaturalGather:
		return "gather"
	case strategy.NaturalEatStored:
		return "eat-stored"
	case strategy.NaturalProvision:
		return "provision"
	case strategy.NaturalWait:
		return "wait"
	}
	return ""
}

// naturalApplySeamAction maps a level-1-admissible action onto the frozen
// choice structure. The model never names a slot: gather targets the
// lowest-index stocked home slot (the frozen rule) and the existing gather
// allocator performs its deterministic admission and rotation. An unknown
// action is a runner error (the recorded seam's level-1 ladder guarantees the
// action is in the declared set).
func naturalApplySeamAction(choice *strategy.NaturalChoice, action string, obs strategy.NaturalObservation) error {
	switch action {
	case "gather":
		target := naturalLowestStockedSlot(obs)
		if target == 0 {
			return fmt.Errorf("%w: seam gather without a stocked home slot", ErrNaturalRunner)
		}
		choice.Kind, choice.TargetSlot = strategy.NaturalGather, target
	case "eat-stored":
		choice.Kind = strategy.NaturalEatStored
	case "provision":
		choice.Kind = strategy.NaturalProvision
	case "wait":
		choice.Kind = strategy.NaturalWait
	default:
		return fmt.Errorf("%w: unknown seam action %q", ErrNaturalRunner, action)
	}
	return nil
}

// naturalLowestStockedSlot derives the claim's target slot from the same
// observation the frozen policy consumes: the lowest-index stocked home slot.
func naturalLowestStockedSlot(obs strategy.NaturalObservation) sim.EntityID {
	var target sim.EntityID
	for _, slot := range obs.HomeSlots {
		stock, err := slot.Stock.Integer()
		if err != nil || stock != 1 {
			continue
		}
		if target == 0 || slot.ID < target {
			target = slot.ID
		}
	}
	return target
}

// naturalActorSeamRequest builds the declared Role A request from the frozen
// observation value and the actor's ACTIVE behaviour logic (spec section 4.5):
// the runner's own bound strategy parameter vector and the active head ref. A
// bound actor carries the active generation's vector and ref; an unbound actor
// carries the frozen base vector and ref. The base ref in the identity block is
// always the runner's frozen base (a rebind never changes it, spec 5.4).
func (f *Natural) naturalActorSeamRequest(id sim.EntityID, hour int, version sim.WorldVersion, actor NaturalActorState, obs strategy.NaturalObservation) protocol.ActorSeamRequest {
	ref, params := f.activeClaimParameters(id)
	return protocol.ActorSeamRequest{
		Seam:        protocol.SeamNaturalClaim,
		Actor:       id,
		Hour:        hour,
		Version:     version,
		Observation: obs,
		Strategy:    params,
		Actions:     naturalClaimActions(f.mechanism),
		Identity:    f.naturalRunIdentity(ref),
		// The v4 persona card's input: the actor's own birth hour and three
		// inherited alleles. It never enters the canonical request bytes, and it
		// deliberately carries no death hour, no parent ids and no world seed.
		Portrait: protocol.ActorPortrait{
			BirthHour: int(actor.Genome.BirthHour),
			LocusM:    actor.Genome.LocusM,
			LocusY:    actor.Genome.LocusY,
			LocusW:    actor.Genome.LocusW,
		},
	}
}

// activeClaimParameters returns the actor's active head ref and parameter
// vector: the bound generation's own vector when the actor carries the active
// generation, otherwise the frozen base vector of the runner's base ref. The
// frozen vector is the declared Appendix C parent vector; the active vector is
// the generation's substituted vector.
func (f *Natural) activeClaimParameters(id sim.EntityID) (strategy.NaturalRef, map[protocol.ParamID]int64) {
	if f.cognitiveStrategy != nil && f.cognitiveStrategy.Binds(id) {
		gen := f.cognitiveStrategy.generation
		return gen.Ref, gen.Params.Values()
	}
	return f.ref, protocol.FrozenNaturalClaimParameters(f.ref)
}

// naturalRunIdentity is the declared run identity block. BaseRef is the
// runner's frozen base; HeadRef is the active head the request is made under
// (the bound generation's ref, else the frozen base). The identity digest binds
// the block.
func (f *Natural) naturalRunIdentity(headRef strategy.NaturalRef) protocol.RunIdentity {
	return protocol.RunIdentity{
		Fixture: protocol.FixtureIdentity{
			Patches: f.patches, R: int(f.r), Founders: f.founders,
			Mechanism: uint32(f.mechanism), Births: f.births,
		},
		Seed:    f.seed,
		BaseRef: f.ref,
		HeadRef: headRef,
	}
}
