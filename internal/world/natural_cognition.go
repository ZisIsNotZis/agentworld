package world

import (
	"context"
	"fmt"
	"sort"

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
	if mechanism == NaturalMechanismV6 || mechanism == NaturalMechanismV7 {
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
// vector; the roster is a pure read of the frozen observation and mutates
// nothing.
func naturalEscalationRoster(hour int, wakes []naturalClaimWake) []protocol.EscalationCandidate {
	roster := make([]protocol.EscalationCandidate, 0, len(wakes))
	for _, w := range wakes {
		roster = append(roster, protocol.EscalationCandidate{
			Actor: w.id, Hour: hour, Features: naturalCandidateFeatures(w.obs),
		})
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].Actor < roster[j].Actor })
	return roster
}

// naturalCandidateFeatures derives a candidate's declared observation-only
// feature rows (Appendix C.1) from the actor's own claim observation. The raw
// rows are read directly; the derived rows (decision_sensitivity, urgency,
// impact, propagation_risk), the world-level aggregate prior_reliability, the
// last_denial string encoding, and the actor-raised escalation_requested bit
// (its predicate is not yet declared) stay zero.
func naturalCandidateFeatures(obs strategy.NaturalObservation) protocol.ValueFeatures {
	var features protocol.ValueFeatures
	features.Energy = naturalFeatureInt(obs.Energy)
	features.Hunger = naturalFeatureInt(obs.Hunger)
	features.Capital = naturalFeatureInt(obs.Capital)
	features.GranaryStock = naturalFeatureInt(obs.GranaryStock)
	features.LastStoredMealHour = naturalFeatureInt(obs.LastStoredMealHour)
	for _, slot := range obs.HomeSlots {
		if naturalFeatureInt(slot.Stock) >= 1 {
			features.StockedHomeSlots++
		}
	}
	return features
}

// naturalFeatureInt reads an observation value as its integer projection. A
// non-integer observation value contributes zero, matching the frozen
// stocked-slot read (naturalLowestStockedSlot).
func naturalFeatureInt(v sim.Value) int64 {
	n, err := v.Integer()
	if err != nil {
		return 0
	}
	return n
}

// naturalSeamOverride replaces the frozen claim choice with the seam decision
// for a selected actor-hour and records the effective choice for the
// evaluation phase. The caller selects the actor through naturalSeamSelection,
// so a nil seam never reaches it. A level-3 transport failure is returned to
// the caller as a fail-loud halt; a level-1 (malformed/inadmissible) failure
// returns the declared fallback and leaves the frozen choice in place, mutating
// nothing.
func (f *Natural) naturalSeamOverride(ctx context.Context, id sim.EntityID, hour int, version sim.WorldVersion, obs strategy.NaturalObservation, choice *strategy.NaturalChoice) error {
	// Record the frozen cascade's own action before the override: it is the
	// comparison baseline for the load-bearing check (EC-2). The seam request
	// carries it for the intervention record; it is not part of the canonical
	// request bytes.
	request := f.naturalActorSeamRequest(id, hour, version, obs)
	request.FrozenAction = naturalChoiceAction(choice.Kind)
	decision, err := f.cognition.Seam.DecideActorSeam(ctx, request)
	if err != nil {
		return err
	}
	if decision.Source == protocol.SourceModel {
		if err := naturalApplySeamAction(choice, decision.Action, obs); err != nil {
			return err
		}
	}
	f.claimDecisions[id] = *choice
	return nil
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
func (f *Natural) naturalActorSeamRequest(id sim.EntityID, hour int, version sim.WorldVersion, obs strategy.NaturalObservation) protocol.ActorSeamRequest {
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
