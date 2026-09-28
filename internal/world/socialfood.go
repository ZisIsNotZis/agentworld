package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var ErrSocialFoodRunner = errors.New("invalid social-food runner")

const socialFoodClockActor sim.EntityID = 5001
const socialFoodStepLimit = 1300 // seven phase boundaries per hour plus the final basal pulse

type SocialFoodOptions struct {
	Yield   int64
	Seed    uint64
	Workers int
	Enabled bool
	// TestClaimOverrides changes only reported urgency for the named requesters.
	TestClaimOverrides map[sim.EntityID]int64
}
type SocialFoodAttempt struct {
	Actor, Target                  sim.EntityID
	Time                           sim.SimTime
	Key                            string
	Kind                           string
	RequestID, Claim, Truth, Score int64 // Truth is privileged audit evidence; never passed to EvaluateReply or placed in an event.
	DonorEnergy                    int64 // Privileged audit of the replying actor's own reserve.
	Rejection                      FoodFlowGatherRejection
	Accepted                       bool
	EventID                        sim.EventID
}
type SocialFoodBatch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []SocialFoodAttempt
}
type SocialFoodCheckpoint struct {
	Hour     int
	Version  sim.WorldVersion
	Balance  SocialFoodBalance
	Patches  [2]SocialFoodPatchState
	Slots    [2][8]SocialFoodSlotState
	Actors   [16]SocialFoodActorState
	Requests [16]SocialFoodRequestState
	Ledgers  [16]SocialFoodLedgerState
	Alive    int
}

// Runner-only state accompanies the authoritative history and scheduler in a
// quiescent persistence handoff. Neither policy callbacks nor callers receive Authority.
type SocialFoodHandoff struct {
	FormatVersion           uint32
	Yield                   int64
	Seed                    uint64
	Enabled                 bool
	Policy                  strategy.SocialFoodRef
	Steps                   int
	History, SchedulerBytes []byte
	Head                    kernel.PortableHead
	Current                 map[sim.EntityID]strategy.FoodFlowAction
	Choices                 map[sim.EntityID]strategy.FoodFlowChoice
	Denied                  map[sim.EntityID]int64
	TestClaimOverrides      map[sim.EntityID]int64
	Journal                 []SocialFoodBatch
	Checkpoints             []SocialFoodCheckpoint
}
type SocialFood struct {
	mu             sync.Mutex
	k              *kernel.Kernel
	sched          *scheduler.Scheduler
	registry       component.Registry
	seeds          []component.ComponentSeed
	flow           [16]*strategy.FoodFlowBound
	social         [16]*strategy.SocialFoodBound
	flowRef        strategy.FoodFlowRef
	ref            strategy.SocialFoodRef
	yield          int64
	seed           uint64
	enabled        bool
	current        map[sim.EntityID]strategy.FoodFlowAction
	choices        map[sim.EntityID]strategy.FoodFlowChoice
	denied         map[sim.EntityID]int64
	claimOverrides map[sim.EntityID]int64
	admissions     map[sim.EntityID]FoodFlowGatherAdmission
	lifecycle      [16]scheduler.Lifecycle
	pending        *socialFoodPendingBatch
	journal        []SocialFoodBatch
	checkpoints    []SocialFoodCheckpoint
	steps          int
}
type socialFoodPendingBatch struct {
	mu        sync.Mutex
	attempts  []SocialFoodAttempt
	starts    map[sim.EntityID]strategy.FoodFlowAction
	choices   map[sim.EntityID]strategy.FoodFlowChoice
	completes []sim.EntityID
	denied    []sim.EntityID
}

func socialFoodSeed(id sim.EntityID, typ sim.ComponentTypeID, fields ...component.FieldSeed) component.ComponentSeed {
	return component.ComponentSeed{Entity: id, Component: typ, Fields: fields}
}
func socialFoodField(id sim.FieldID, n int64) component.FieldSeed {
	return component.FieldSeed{Field: id, Value: sim.IntegerValue(n)}
}
func socialFoodSeedRef(id sim.FieldID, value sim.EntityID) component.FieldSeed {
	return component.FieldSeed{Field: id, Value: socialFoodValueRef(value)}
}
func socialFoodValueRef(id sim.EntityID) sim.Value {
	if id == 0 {
		v, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		return v
	}
	v, _ := sim.EntityRefValue(id)
	return v
}
func socialFoodPatch(id sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, v sim.Value) component.Patch {
	return component.Patch{Entity: id, Component: typ, SchemaVersion: SocialFoodSchemaVersion, Field: field, Value: v}
}
func socialFoodNumber(id sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, n int64) component.Patch {
	return socialFoodPatch(id, typ, field, sim.IntegerValue(n))
}
func socialFoodRefPatch(id sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, target sim.EntityID) component.Patch {
	return socialFoodPatch(id, typ, field, socialFoodValueRef(target))
}
func socialFoodKey(kind string, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("socialfood/%s/%016x/%04x", kind, uint64(at), uint64(actor))
}
func socialFoodProposal(kind string, at sim.SimTime, actor sim.EntityID, rule sim.RuleID, patches ...component.Patch) kernel.Proposal {
	return kernel.Proposal{Key: socialFoodKey(kind, at, actor), Time: at, Cause: kernel.Cause{Actor: actor, World: actor == 0}, Rule: rule, RuleVersion: SocialFoodRuleVersion, Patches: patches}
}
func NewSocialFood(o SocialFoodOptions) (*SocialFood, error) {
	if o.Workers < 1 || o.Yield < 0 || o.Yield > 8 {
		return nil, ErrSocialFoodRunner
	}
	for id, claim := range o.TestClaimOverrides {
		if _, err := SocialFoodActorPatchID(id); err != nil || claim < 0 || claim > 2 {
			return nil, ErrSocialFoodRunner
		}
	}
	reg, err := SocialFoodRegistry()
	if err != nil {
		return nil, err
	}
	flowRef := strategy.FoodFlowRef{ID: "social-food-gather", Version: 1}
	flowPolicies := strategy.NewFoodFlowRegistry()
	err = flowPolicies.Register(strategy.FoodFlowPolicy{FormatVersion: strategy.FoodFlowPolicyFormatV1, Ref: flowRef, Budget: strategy.FoodFlowBudget{Candidates: 16, Evaluations: 16}, RestDuration: strategy.FoodFlowRestDuration})
	if err != nil {
		return nil, err
	}
	ref := strategy.SocialFoodRef{ID: "social-food", Version: 2}
	policies := strategy.NewSocialFoodRegistry()
	if err = policies.Register(strategy.FrozenSocialFoodPolicy(ref)); err != nil {
		return nil, err
	}
	f := &SocialFood{registry: reg, flowRef: flowRef, ref: ref, yield: o.Yield, seed: o.Seed, enabled: o.Enabled, current: make(map[sim.EntityID]strategy.FoodFlowAction), choices: make(map[sim.EntityID]strategy.FoodFlowChoice), denied: make(map[sim.EntityID]int64), claimOverrides: make(map[sim.EntityID]int64, len(o.TestClaimOverrides))}
	for id, claim := range o.TestClaimOverrides {
		f.claimOverrides[id] = claim
	}
	for i := 0; i < 2; i++ {
		patch, _ := SocialFoodPatchID(i)
		f.seeds = append(f.seeds, socialFoodSeed(patch, SocialFoodPatchTypeID, socialFoodField(SocialFoodPatchYieldField, o.Yield), socialFoodField(SocialFoodPatchPulsesField, 0), socialFoodField(SocialFoodPatchProducedField, 0), socialFoodField(SocialFoodPatchUnrealizedField, 0)))
		for j := 0; j < 8; j++ {
			slot, _ := SocialFoodSlotID(i, j)
			f.seeds = append(f.seeds, socialFoodSeed(slot, SocialFoodSlotTypeID, socialFoodField(SocialFoodSlotStockField, 0), socialFoodSeedRef(SocialFoodSlotPatchField, patch), socialFoodField(SocialFoodSlotGatheredField, 0)))
		}
	}
	for i := 0; i < 16; i++ {
		actor := sim.EntityID(i + 1)
		f.seeds = append(f.seeds, socialFoodSeed(actor, SocialFoodBagTypeID, socialFoodField(SocialFoodBagUnitsField, 0), socialFoodSeedRef(SocialFoodBagSourceField, 0), socialFoodSeedRef(SocialFoodBagGathererField, 0), socialFoodSeedRef(SocialFoodBagLastDonorField, 0)), socialFoodSeed(actor, SocialFoodBodyTypeID, socialFoodField(SocialFoodBodyEnergyField, 11), socialFoodField(SocialFoodBodyHungerField, 0), socialFoodField(SocialFoodBodyBasalSpentField, 0), socialFoodField(SocialFoodBodyCapLostField, 0), socialFoodField(SocialFoodBodyConsumedField, 0), socialFoodField(SocialFoodBodyLastGatherHourField, -1)), socialFoodSeed(actor, SocialFoodRequestTypeID, socialFoodField(SocialFoodRequestIDField, 0), socialFoodField(SocialFoodRequestHourField, -1), socialFoodSeedRef(SocialFoodRequestAddresseeField, 0), socialFoodField(SocialFoodRequestClaimField, 0), socialFoodField(SocialFoodRequestStatusField, 0), socialFoodField(SocialFoodRequestsWitnessedField, 0), socialFoodField(SocialFoodGiftsWitnessedField, 0), socialFoodField(SocialFoodRefusalsWitnessedField, 0)), socialFoodSeed(actor, SocialFoodLedgerTypeID, socialFoodField(SocialFoodLastReplyHourField, -1), socialFoodField(SocialFoodReceivedField, 0), socialFoodField(SocialFoodGivenField, 0), socialFoodField(SocialFoodNetField, 0)))
		f.flow[i], err = flowPolicies.Bind(strategy.FoodFlowBinding{Actor: actor, Ref: flowRef})
		if err != nil {
			return nil, err
		}
		f.social[i], err = policies.Bind(strategy.SocialFoodBinding{Actor: actor, Ref: ref})
		if err != nil {
			return nil, err
		}
	}
	f.k, err = kernel.New(reg, 0, f.seeds)
	if err != nil {
		return nil, err
	}
	f.sched, err = scheduler.New(f.k, o.Workers, f.evaluate)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= 16; i++ {
		if err = f.sched.Register(sim.EntityID(i)); err != nil {
			return nil, err
		}
	}
	if err = f.sched.Register(socialFoodClockActor); err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: socialFoodClockActor, At: 0, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}
func socialFoodRead(view scheduler.SnapshotView, id sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]sim.Value, error) {
	row, err := view.Reader.Read(component.ReadRequest{Entity: id, Component: typ, Fields: fields, WorldVersion: view.Version, Authority: view.Authority})
	if err != nil {
		return nil, err
	}
	out := make([]sim.Value, len(fields))
	for i, field := range fields {
		out[i], err = row.Value(field)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func socialFoodInts(view scheduler.SnapshotView, id sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]int64, error) {
	values, err := socialFoodRead(view, id, typ, fields...)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(values))
	for i, v := range values {
		out[i], err = v.Integer()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func socialFoodRefValue(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing && v.Kind() == sim.EntityRefKind {
		return 0, nil
	}
	return v.EntityRef()
}
func socialFoodStates(view scheduler.SnapshotView) (patches [2]SocialFoodPatchState, slots [2][8]SocialFoodSlotState, actors [16]SocialFoodActorState, requests [16]SocialFoodRequestState, ledgers [16]SocialFoodLedgerState, err error) {
	for i := range patches {
		id, _ := SocialFoodPatchID(i)
		var n []int64
		n, err = socialFoodInts(view, id, SocialFoodPatchTypeID, 1, 2, 3, 4)
		if err != nil {
			return
		}
		patches[i] = SocialFoodPatchState{n[0], n[1], n[2], n[3]}
		for j := range slots[i] {
			id, _ = SocialFoodSlotID(i, j)
			n, err = socialFoodInts(view, id, SocialFoodSlotTypeID, SocialFoodSlotStockField, SocialFoodSlotGatheredField)
			if err != nil {
				return
			}
			slots[i][j] = SocialFoodSlotState{n[0], n[1]}
		}
	}
	for i := range actors {
		id := sim.EntityID(i + 1)
		var n []int64
		var values []sim.Value
		n, err = socialFoodInts(view, id, SocialFoodBodyTypeID, 1, 2, 3, 4, 5, 6)
		if err != nil {
			return
		}
		values, err = socialFoodRead(view, id, SocialFoodBagTypeID, 1, 2, 3, 4)
		if err != nil {
			return
		}
		var b SocialFoodBagState
		b.Units, err = values[0].Integer()
		if err != nil {
			return
		}
		b.Source, err = socialFoodRefValue(values[1])
		if err != nil {
			return
		}
		b.Gatherer, err = socialFoodRefValue(values[2])
		if err != nil {
			return
		}
		b.LastDonor, err = socialFoodRefValue(values[3])
		if err != nil {
			return
		}
		actors[i] = SocialFoodActorState{Bag: b, Energy: n[0], Hunger: n[1], BasalSpent: n[2], CapLost: n[3], Consumed: n[4], LastGatherHour: n[5]}
		n, err = socialFoodInts(view, id, SocialFoodRequestTypeID, 1, 2, 4, 5, 6, 7, 8)
		if err != nil {
			return
		}
		values, err = socialFoodRead(view, id, SocialFoodRequestTypeID, SocialFoodRequestAddresseeField)
		if err != nil {
			return
		}
		var target sim.EntityID
		target, err = socialFoodRefValue(values[0])
		if err != nil {
			return
		}
		requests[i] = SocialFoodRequestState{ID: n[0], Hour: n[1], Addressee: target, Claim: n[2], Status: SocialFoodRequestStatus(n[3]), Requests: n[4], Gifts: n[5], Refusals: n[6]}
		n, err = socialFoodInts(view, id, SocialFoodLedgerTypeID, 1, 2, 3, 4)
		if err != nil {
			return
		}
		ledgers[i] = SocialFoodLedgerState{n[0], n[1], n[2], n[3]}
	}
	return
}
func socialFoodView(f *SocialFood, view scheduler.SnapshotView, actor sim.EntityID, at sim.SimTime) (strategy.FoodFlowObservation, error) {
	body, err := socialFoodRead(view, actor, SocialFoodBodyTypeID, SocialFoodBodyEnergyField, SocialFoodBodyHungerField, SocialFoodBodyLastGatherHourField)
	if err != nil {
		return strategy.FoodFlowObservation{}, err
	}
	bag, err := socialFoodRead(view, actor, SocialFoodBagTypeID, SocialFoodBagUnitsField, SocialFoodBagSourceField)
	if err != nil {
		return strategy.FoodFlowObservation{}, err
	}
	patch, _ := SocialFoodActorPatchID(actor)
	index := int(patch - 3001)
	obs := strategy.FoodFlowObservation{Actor: actor, Time: at, WorldVersion: view.Version, Energy: body[0], Hunger: body[1], LastGatherHour: body[2], BagUnits: bag[0], BagSource: bag[1], Patches: []strategy.FoodFlowPatchView{{ID: sim.EntityID(1001 + index)}}}
	if bag[1].State() == sim.Present {
		obs.BagSource = socialFoodValueRef(sim.EntityID(1001 + index))
	}
	for j := 0; j < 8; j++ {
		id, _ := SocialFoodSlotID(index, j)
		stock, e := socialFoodRead(view, id, SocialFoodSlotTypeID, SocialFoodSlotStockField)
		if e != nil {
			return obs, e
		}
		obs.Patches[0].Slots = append(obs.Patches[0].Slots, strategy.FoodFlowSlotView{ID: sim.EntityID(2001 + index*8 + j), Stock: stock[0]})
	}
	return obs, nil
}
func socialFoodEffect(actor sim.EntityID, at sim.SimTime, cause scheduler.WakeCause) scheduler.Effect {
	return scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: actor, Wake: scheduler.Wake{Actor: actor, At: at, Cause: cause}}
}
func (f *SocialFood) pulse(ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	at := ready.At
	h := int(at / sim.SimTime(SocialFoodHour))
	if h > 168 || at != sim.SimTime(h)*sim.SimTime(SocialFoodHour) {
		return scheduler.Evaluation{}, ErrSocialFoodRunner
	}
	patches, slots, actors, requests, _, err := socialFoodStates(view)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	var out scheduler.Evaluation
	for i, a := range actors {
		id := sim.EntityID(i + 1)
		if (f.lifecycle[i] == scheduler.Alive) != (a.Energy > 0) {
			return out, ErrSocialFoodRunner
		}
		if requests[i].Status == SocialFoodPending {
			return out, ErrSocialFoodRunner // finalization must have closed every request before this pulse
		}
		if h == 0 || a.Energy == 0 {
			continue
		}
		next, e := SocialFoodBasal(id, a, 1)
		if e != nil {
			return out, e
		}
		p := socialFoodProposal("basal", at, id, SocialFoodBasalRule, socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyEnergyField, next.Energy), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyHungerField, next.Hunger), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyBasalSpentField, next.BasalSpent))
		p.Cause = kernel.Cause{World: true}
		out.Proposals = append(out.Proposals, p)
		if next.Energy == 0 {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
		}
		actors[i] = next
	}
	if h < 168 {
		for i, p := range patches {
			next, stock, e := SocialFoodProduce(h, p, slots[i])
			if e != nil {
				return out, e
			}
			id, _ := SocialFoodPatchID(i)
			ps := []component.Patch{socialFoodNumber(id, SocialFoodPatchTypeID, SocialFoodPatchPulsesField, next.Pulses), socialFoodNumber(id, SocialFoodPatchTypeID, SocialFoodPatchProducedField, next.Produced), socialFoodNumber(id, SocialFoodPatchTypeID, SocialFoodPatchUnrealizedField, next.Unrealized)}
			for j, s := range stock {
				if s.Stock != slots[i][j].Stock {
					slot, _ := SocialFoodSlotID(i, j)
					ps = append(ps, socialFoodNumber(slot, SocialFoodSlotTypeID, SocialFoodSlotStockField, s.Stock))
				}
			}
			out.Proposals = append(out.Proposals, socialFoodProposal(fmt.Sprintf("produce-%d", i), at, 0, SocialFoodProduceRule, ps...))
		}
		phases := []sim.SimTime{}
		for phase := 0; phase < 4; phase++ {
			t, _ := SocialFoodPhaseTime(h, phase)
			phases = append(phases, t)
		}
		for i, a := range actors {
			if a.Energy > 0 {
				id := sim.EntityID(i + 1)
				out.Effects = append(out.Effects, socialFoodEffect(id, at+1, scheduler.WakeNeedThreshold))
				for phase := 1; phase < 4; phase++ {
					out.Effects = append(out.Effects, socialFoodEffect(id, phases[phase], scheduler.WakeAudit))
				}
			}
		}
	}
	if h < 168 {
		out.Effects = append(out.Effects, socialFoodEffect(socialFoodClockActor, sim.SimTime(h+1)*sim.SimTime(SocialFoodHour), scheduler.WakeAudit))
	}
	return out, nil
}
func (f *SocialFood) record(a SocialFoodAttempt) {
	f.pending.mu.Lock()
	f.pending.attempts = append(f.pending.attempts, a)
	f.pending.mu.Unlock()
}
func (f *SocialFood) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	if ready.Fiber.Actor == socialFoodClockActor {
		return f.pulse(ready, view)
	}
	id := ready.Fiber.Actor
	if id < 1 || id > 16 || len(ready.Causes) != 1 {
		return scheduler.Evaluation{}, ErrSocialFoodRunner
	}
	h := int(ready.At / sim.SimTime(SocialFoodHour))
	at := ready.At
	phase := func(n int) bool { t, e := SocialFoodPhaseTime(h, n); return e == nil && t == at }
	var out scheduler.Evaluation
	switch {
	case ready.Causes[0] == scheduler.WakeNeedThreshold && at == sim.SimTime(h)*sim.SimTime(SocialFoodHour)+1:
		obs, e := socialFoodView(f, view, id, at)
		if e != nil {
			return out, e
		}
		choice, e := f.flow[id-1].Evaluate(obs)
		if e != nil || choice.Ref != f.flowRef || choice.ObservedVersion != view.Version {
			return out, ErrSocialFoodRunner
		}
		if choice.Kind == strategy.FoodFlowGather {
			f.pending.mu.Lock()
			f.pending.starts[id] = strategy.FoodFlowGather
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: SocialFoodGatherDuration})
		} else if choice.Kind != strategy.FoodFlowWait && choice.Kind != strategy.FoodFlowEat {
			return out, ErrSocialFoodRunner
		}
		f.pending.mu.Lock()
		if choice.Kind == strategy.FoodFlowGather {
			f.pending.attempts = append(f.pending.attempts, SocialFoodAttempt{Actor: id, Time: at, Kind: "claim", Target: sim.EntityID(3001 + (choice.TargetPatch - 1001))})
			f.pending.choices[id] = choice
		}
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeCompletion && phase(0):
		if f.current[id] != strategy.FoodFlowGather {
			return out, ErrSocialFoodRunner
		}
		admission, ok := f.admissions[id]
		if !ok {
			return out, ErrSocialFoodRunner
		}
		a := SocialFoodAttempt{Actor: id, Time: at, Kind: "gather", Rejection: admission.Rejection}
		if admission.Rejection == FoodFlowGatherAdmitted {
			_, slots, actors, _, _, e := socialFoodStates(view)
			if e != nil {
				return out, e
			}
			patch := sim.EntityID(admission.Patch + 2000)
			slot := sim.EntityID(admission.Slot + 2000)
			pi := int(patch - 3001)
			si := int(slot-4001) - pi*8
			if pi < 0 || pi >= 2 || si < 0 || si >= 8 {
				return out, ErrSocialFoodRunner
			}
			ns, na, e := SocialFoodGather(h, patch, slot, id, slots[pi][si], actors[id-1])
			if e != nil {
				return out, e
			}
			p := socialFoodProposal("gather", at, id, SocialFoodGatherRule, socialFoodNumber(slot, SocialFoodSlotTypeID, SocialFoodSlotStockField, ns.Stock), socialFoodNumber(slot, SocialFoodSlotTypeID, SocialFoodSlotGatheredField, ns.Gathered), socialFoodNumber(id, SocialFoodBagTypeID, SocialFoodBagUnitsField, na.Bag.Units), socialFoodRefPatch(id, SocialFoodBagTypeID, SocialFoodBagSourceField, na.Bag.Source), socialFoodRefPatch(id, SocialFoodBagTypeID, SocialFoodBagGathererField, na.Bag.Gatherer), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyLastGatherHourField, na.LastGatherHour))
			a.Key = p.Key
			out.Proposals = append(out.Proposals, p)
		} else {
			f.pending.mu.Lock()
			f.pending.denied = append(f.pending.denied, id)
			f.pending.mu.Unlock()
		}
		f.pending.mu.Lock()
		f.pending.completes = append(f.pending.completes, id)
		f.pending.mu.Unlock()
		f.record(a)
	case ready.Causes[0] == scheduler.WakeAudit && phase(1):
		deniedHour, wasDenied := f.denied[id]
		if !f.enabled || !wasDenied || deniedHour != int64(h) {
			return out, nil
		}
		_, _, actors, requests, _, e := socialFoodStates(view)
		if e != nil {
			return out, e
		}
		patch, _ := SocialFoodActorPatchID(id)
		obs := strategy.SocialFoodRequestObservation{Actor: id, Patch: patch, Ref: f.ref, Hour: h, WorldVersion: view.Version, OwnEnergy: sim.IntegerValue(actors[id-1].Energy), OwnBagUnits: sim.IntegerValue(actors[id-1].Bag.Units), DeniedGatherHour: deniedHour, OwnRequest: strategy.SocialFoodRequestGuard{Hour: requests[id-1].Hour, Status: strategy.SocialFoodRequestStatus(requests[id-1].Status)}}
		if claim, ok := f.claimOverrides[id]; ok {
			obs.TestClaimOverride = &claim
		}
		choice, e := f.social[id-1].EvaluateRequest(obs)
		if e != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrSocialFoodRunner
		}
		if choice.Kind != strategy.SocialFoodRequest {
			return out, nil
		}
		next, e := SocialFoodRequest(h, id, choice.Target, true, choice.Claim, requests[id-1], actors[id-1])
		if e != nil {
			return out, e
		}
		p := socialFoodProposal("request", at, id, SocialFoodRequestRule, socialFoodNumber(id, SocialFoodRequestTypeID, SocialFoodRequestIDField, next.ID), socialFoodNumber(id, SocialFoodRequestTypeID, SocialFoodRequestHourField, next.Hour), socialFoodRefPatch(id, SocialFoodRequestTypeID, SocialFoodRequestAddresseeField, next.Addressee), socialFoodNumber(id, SocialFoodRequestTypeID, SocialFoodRequestClaimField, next.Claim), socialFoodNumber(id, SocialFoodRequestTypeID, SocialFoodRequestStatusField, int64(next.Status)), socialFoodNumber(id, SocialFoodRequestTypeID, SocialFoodRequestsWitnessedField, next.Requests))
		out.Proposals = append(out.Proposals, p)
		f.record(SocialFoodAttempt{Actor: id, Target: choice.Target, Time: at, Kind: "request", Key: p.Key, RequestID: next.ID, Claim: next.Claim, Truth: actors[id-1].Energy})
	case ready.Causes[0] == scheduler.WakeAudit && phase(2):
		if !f.enabled {
			return out, nil
		}
		_, _, actors, requests, ledgers, e := socialFoodStates(view)
		if e != nil {
			return out, e
		}
		// A ring has exactly one incoming known dyad. Never scan or prefer a low ID.
		requester := id - 1
		if id == 1 || id == 9 {
			requester = id + 7
		}
		r := requests[requester-1]
		if r.Status != SocialFoodPending || r.Hour != int64(h) || r.Addressee != id {
			return out, nil
		}
		patch, _ := SocialFoodActorPatchID(id)
		obs := strategy.SocialFoodReplyObservation{Actor: id, Patch: patch, Ref: f.ref, Hour: h, WorldVersion: view.Version, OwnEnergy: sim.IntegerValue(actors[id-1].Energy), OwnHunger: sim.IntegerValue(actors[id-1].Hunger), OwnBagUnits: sim.IntegerValue(actors[id-1].Bag.Units), OwnLastReplyHour: ledgers[id-1].LastReplyHour, Pending: []strategy.SocialFoodPendingView{{Requester: requester, Addressee: id, RequestID: r.ID, Hour: h, Claim: r.Claim}}}
		choice, e := f.social[id-1].EvaluateReply(obs)
		if e != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version || choice.Target != requester || choice.RequestID != r.ID {
			return out, ErrSocialFoodRunner
		}
		consent := SocialFoodConsent{RequestID: r.ID, Donor: id, Hour: h, Accept: choice.Kind == strategy.SocialFoodAccept}
		kind := "refuse"
		var p kernel.Proposal
		if consent.Accept {
			kind = "gift"
			// This constructor rechecks the entire fresh authoritative snapshot, not a cached claim.
			p, e = SocialFoodGiftProposal(socialFoodKey(kind, at, id), at, h, id, requester, consent, r, actors[id-1], actors[requester-1], ledgers[id-1], ledgers[requester-1])
		} else if choice.Kind == strategy.SocialFoodRefuse {
			var nr SocialFoodRequestState
			var nl SocialFoodLedgerState
			nr, nl, e = SocialFoodRefuse(h, id, requester, consent, r, actors[id-1], ledgers[id-1])
			if e == nil {
				p = socialFoodProposal(kind, at, id, SocialFoodRefuseRule, socialFoodNumber(requester, SocialFoodRequestTypeID, SocialFoodRequestStatusField, int64(nr.Status)), socialFoodNumber(requester, SocialFoodRequestTypeID, SocialFoodRefusalsWitnessedField, nr.Refusals), socialFoodNumber(id, SocialFoodLedgerTypeID, SocialFoodLastReplyHourField, nl.LastReplyHour))
			}
		} else {
			return out, ErrSocialFoodRunner
		}
		if e != nil {
			return out, e
		}
		out.Proposals = append(out.Proposals, p)
		f.record(SocialFoodAttempt{Actor: id, Target: requester, Time: at, Kind: kind, Key: p.Key, RequestID: r.ID, Claim: r.Claim, Truth: actors[requester-1].Energy, Score: choice.Score, DonorEnergy: actors[id-1].Energy})
	case ready.Causes[0] == scheduler.WakeAudit && phase(3):
		// The common snapshot is after the entire reply batch. Do not expire a
		// cached request: its addressed donor may have accepted or refused it.
		_, _, actors, requests, _, e := socialFoodStates(view)
		if e != nil {
			return out, e
		}
		if requests[id-1].Status == SocialFoodPending {
			p, err := SocialFoodExpireProposal(socialFoodKey("expire", at, id), h, at, id, requests[id-1])
			if err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, p)
			f.record(SocialFoodAttempt{Actor: id, Target: requests[id-1].Addressee, Time: at, Kind: "expire", Key: p.Key, RequestID: requests[id-1].ID, Claim: requests[id-1].Claim, Truth: actors[id-1].Energy})
		}
		a := actors[id-1]
		if a.Energy == 0 {
			return out, nil
		}
		kind := strategy.FoodFlowRest
		if a.Bag.Units == 1 {
			kind = strategy.FoodFlowEat
		}
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: SocialFoodMealDuration})
		f.pending.mu.Lock()
		f.pending.starts[id] = kind
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeCompletion && phase(4):
		kind := f.current[id]
		if kind != strategy.FoodFlowEat && kind != strategy.FoodFlowRest {
			return out, ErrSocialFoodRunner
		}
		f.pending.mu.Lock()
		f.pending.completes = append(f.pending.completes, id)
		f.pending.mu.Unlock()
		if kind == strategy.FoodFlowEat {
			_, _, actors, _, _, e := socialFoodStates(view)
			if e != nil {
				return out, e
			}
			next, e := SocialFoodConsume(id, actors[id-1])
			if e != nil {
				return out, e
			}
			p := socialFoodProposal("eat", at, id, SocialFoodConsumeRule, socialFoodNumber(id, SocialFoodBagTypeID, SocialFoodBagUnitsField, 0), socialFoodRefPatch(id, SocialFoodBagTypeID, SocialFoodBagSourceField, 0), socialFoodRefPatch(id, SocialFoodBagTypeID, SocialFoodBagGathererField, 0), socialFoodRefPatch(id, SocialFoodBagTypeID, SocialFoodBagLastDonorField, 0), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyConsumedField, next.Consumed), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyEnergyField, next.Energy), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyCapLostField, next.CapLost), socialFoodNumber(id, SocialFoodBodyTypeID, SocialFoodBodyHungerField, next.Hunger))
			out.Proposals = append(out.Proposals, p)
			f.record(SocialFoodAttempt{Actor: id, Time: at, Kind: "eat", Key: p.Key})
		}
	default:
		return out, ErrSocialFoodRunner
	}
	return out, nil
}

func (f *SocialFood) allocate(at sim.SimTime) error {
	f.admissions = make(map[sim.EntityID]FoodFlowGatherAdmission)
	var gather []sim.EntityID
	for _, w := range f.sched.Snapshot().Wakes {
		if w.At == at && w.Cause == scheduler.WakeCompletion && f.current[w.Actor] == strategy.FoodFlowGather {
			gather = append(gather, w.Actor)
		}
	}
	if len(gather) == 0 {
		return nil
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	_, slots, actors, _, _, err := socialFoodStates(view)
	if err != nil {
		return err
	}
	var stock []FoodFlowStockedSlot
	var claims []FoodFlowGatherClaim
	for i := range slots {
		for j, s := range slots[i] {
			if s.Stock == 1 {
				stock = append(stock, FoodFlowStockedSlot{ID: sim.EntityID(2001 + i*8 + j), Patch: sim.EntityID(1001 + i)})
			}
		}
	}
	for _, id := range gather {
		choice, ok := f.choices[id]
		if !ok || choice.Kind != strategy.FoodFlowGather {
			return ErrSocialFoodRunner
		}
		a := actors[id-1]
		claims = append(claims, FoodFlowGatherClaim{Actor: id, TargetPatch: choice.TargetPatch, TargetSlot: choice.TargetSlot, Energy: a.Energy, BagUnits: a.Bag.Units, LastGatherHour: a.LastGatherHour})
	}
	admitted, err := FoodFlowAllocateGathers(int(at/sim.SimTime(SocialFoodHour)), f.seed, stock, claims)
	if err != nil {
		return err
	}
	for _, a := range admitted {
		f.admissions[a.Actor] = a
	}
	return nil
}

// Step publishes the batch only after the scheduler atomically closes its timestamp.
func (f *SocialFood) Step(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.steps >= socialFoodStepLimit {
		return false, ErrSocialFoodRunner
	}
	snap := f.sched.Snapshot()
	if len(snap.Wakes) == 0 {
		return false, nil
	}
	at := snap.Wakes[0].At
	if at%sim.SimTime(SocialFoodHour) == 0 {
		if len(snap.Fibers) != 17 {
			return false, ErrSocialFoodRunner
		}
		for i := range f.lifecycle {
			fiber := snap.Fibers[i]
			if fiber.Actor != sim.EntityID(i+1) || fiber.Activity != nil {
				return false, ErrSocialFoodRunner
			}
			f.lifecycle[i] = fiber.Lifecycle
		}
	}
	if err := f.allocate(at); err != nil {
		return false, err
	}
	f.pending = &socialFoodPendingBatch{starts: make(map[sim.EntityID]strategy.FoodFlowAction), choices: make(map[sim.EntityID]strategy.FoodFlowChoice)}
	defer func() { f.pending = nil }()
	events, processed, err := f.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	head := f.k.SnapshotHead()
	batch := SocialFoodBatch{Time: at, Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for _, a := range f.pending.attempts {
		if a.Key != "" {
			a.EventID = ids[a.Key]
			if a.EventID == 0 {
				return false, ErrSocialFoodRunner
			}
			a.Accepted = true
		}
		batch.Attempts = append(batch.Attempts, a)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool {
		a, b := batch.Attempts[i], batch.Attempts[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		return a.Kind < b.Kind
	})
	for _, id := range f.pending.completes {
		delete(f.current, id)
		delete(f.choices, id)
	}
	for id, kind := range f.pending.starts {
		fiber, ok := f.sched.Fiber(id)
		if !ok || fiber.Activity == nil {
			return false, ErrSocialFoodRunner
		}
		f.current[id] = kind
		if kind == strategy.FoodFlowGather {
			f.choices[id] = f.pending.choices[id]
		}
	}
	for _, id := range f.pending.denied {
		f.denied[id] = int64(at / sim.SimTime(SocialFoodHour))
	}
	f.steps++
	f.journal = append(f.journal, batch)
	if err := f.checkBalance(); err != nil {
		return false, err
	}
	if at%sim.SimTime(SocialFoodHour) == 0 {
		check, err := f.checkpoint(int(at / sim.SimTime(SocialFoodHour)))
		if err != nil {
			return false, err
		}
		f.checkpoints = append(f.checkpoints, check)
	}
	return true, nil
}
func (f *SocialFood) checkBalance() error {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	p, s, a, r, l, err := socialFoodStates(view)
	if err != nil {
		return err
	}
	if _, err = SocialFoodCheckConservation(p, s, a); err != nil {
		return err
	}
	return SocialFoodCheckWitnesses(r, l)
}
func (f *SocialFood) checkpoint(h int) (SocialFoodCheckpoint, error) {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	p, s, a, r, l, err := socialFoodStates(view)
	if err != nil {
		return SocialFoodCheckpoint{}, err
	}
	balance, err := SocialFoodCheckConservation(p, s, a)
	if err != nil {
		return SocialFoodCheckpoint{}, err
	}
	if err = SocialFoodCheckWitnesses(r, l); err != nil {
		return SocialFoodCheckpoint{}, err
	}
	check := SocialFoodCheckpoint{Hour: h, Version: head.Version, Balance: balance, Patches: p, Slots: s, Actors: a, Requests: r, Ledgers: l}
	for _, actor := range a {
		if actor.Energy > 0 {
			check.Alive++
		}
	}
	return check, nil
}
func (f *SocialFood) Run(ctx context.Context) error {
	for {
		ok, err := f.Step(ctx)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
	}
	if len(f.checkpoints) != 169 || f.checkpoints[168].Hour != 168 {
		return ErrSocialFoodRunner
	}
	return nil
}
func (f *SocialFood) SchedulerSnapshot() scheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sched.Snapshot()
}
func (f *SocialFood) Journal() []SocialFoodBatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneSocialFoodJournal(f.journal)
}
func cloneSocialFoodJournal(batches []SocialFoodBatch) []SocialFoodBatch {
	out := make([]SocialFoodBatch, len(batches))
	for i, b := range batches {
		out[i] = b
		out[i].Attempts = append([]SocialFoodAttempt(nil), b.Attempts...)
	}
	return out
}
func (f *SocialFood) Checkpoints() []SocialFoodCheckpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SocialFoodCheckpoint(nil), f.checkpoints...)
}
func (f *SocialFood) Handoff() (SocialFoodHandoff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return SocialFoodHandoff{}, err
	}
	portable, schedHead, err := f.sched.ExportPortable()
	if err != nil || head != schedHead {
		return SocialFoodHandoff{}, ErrSocialFoodRunner
	}
	out := SocialFoodHandoff{FormatVersion: SocialFoodFormatVersion, Yield: f.yield, Seed: f.seed, Enabled: f.enabled, Policy: f.ref, Steps: f.steps, History: history, SchedulerBytes: portable, Head: head, Current: make(map[sim.EntityID]strategy.FoodFlowAction, len(f.current)), Choices: make(map[sim.EntityID]strategy.FoodFlowChoice, len(f.choices)), Denied: make(map[sim.EntityID]int64, len(f.denied)), TestClaimOverrides: make(map[sim.EntityID]int64, len(f.claimOverrides)), Journal: cloneSocialFoodJournal(f.journal), Checkpoints: append([]SocialFoodCheckpoint(nil), f.checkpoints...)}
	for id, v := range f.current {
		out.Current[id] = v
	}
	for id, v := range f.choices {
		out.Choices[id] = v
	}
	for id, v := range f.denied {
		out.Denied[id] = v
	}
	for id, v := range f.claimOverrides {
		out.TestClaimOverrides[id] = v
	}
	return out, nil
}
