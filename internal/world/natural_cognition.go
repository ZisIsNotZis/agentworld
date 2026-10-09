package world

import (
	"context"
	"fmt"

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
}

// validate refuses a half-wired seam: entering the seam without a decider or a
// declared wake policy is a construction-time error, not a runtime surprise.
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
	return nil
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

// naturalSeamEscalates reports whether the declared wake policy selects the
// hour and a seam is wired. It is the only gate into the seam.
func (f *Natural) naturalSeamEscalates(hour int) bool {
	return f.cognition != nil && f.cognition.WakePolicy.Escalates(hour)
}

// naturalSeamOverride replaces the frozen claim choice with the seam decision
// for a selected actor-hour and records the effective choice for the
// evaluation phase. It is a no-op when the seam is nil or the hour is not
// selected, so the frozen path is byte-identical with a nil seam. A level-3
// transport failure is returned to the caller as a fail-loud halt; a level-1
// (malformed/inadmissible) failure returns the declared fallback and leaves
// the frozen choice in place, mutating nothing.
func (f *Natural) naturalSeamOverride(ctx context.Context, id sim.EntityID, hour int, version sim.WorldVersion, obs strategy.NaturalObservation, choice *strategy.NaturalChoice) error {
	if !f.naturalSeamEscalates(hour) {
		return nil
	}
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
