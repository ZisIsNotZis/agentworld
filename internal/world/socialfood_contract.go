package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"fmt"
)

var ErrSocialFoodContract = errors.New("invalid social-food v2 contract")

const (
	SocialFoodFormatVersion     uint32                = 2
	SocialFoodSchemaVersion     sim.SchemaVersion     = 2
	SocialFoodRuleVersion       uint32                = 2
	SocialFoodProjectionVersion sim.ProjectionVersion = 2
	SocialFoodActorCount                              = 16
	SocialFoodPatchCount                              = 2
	SocialFoodSlotsPerPatch                           = 8
	SocialFoodHorizonHours                            = 168
	SocialFoodHour              sim.Duration          = 3_600_000_000
	SocialFoodGatherDuration    sim.Duration          = 20 * 60 * 1_000_000
	SocialFoodMealDuration      sim.Duration          = 10 * 60 * 1_000_000
	SocialFoodNeverGatheredHour int64                 = -1
	SocialFoodPatchTypeID       sim.ComponentTypeID   = 0x80000020
	SocialFoodSlotTypeID        sim.ComponentTypeID   = 0x80000021
	SocialFoodBagTypeID         sim.ComponentTypeID   = 0x80000022
	SocialFoodBodyTypeID        sim.ComponentTypeID   = 0x80000023
	SocialFoodRequestTypeID     sim.ComponentTypeID   = 0x80000024
	SocialFoodLedgerTypeID      sim.ComponentTypeID   = 0x80000025
	SocialFoodProduceRule       sim.RuleID            = 201
	SocialFoodGatherRule        sim.RuleID            = 202
	SocialFoodConsumeRule       sim.RuleID            = 203
	SocialFoodBasalRule         sim.RuleID            = 204
	SocialFoodRequestRule       sim.RuleID            = 205
	SocialFoodGiftRule          sim.RuleID            = 206
	SocialFoodRefuseRule        sim.RuleID            = 207
	SocialFoodExpireRule        sim.RuleID            = 208
	SocialFoodWithdrawRule      sim.RuleID            = 209
)

const (
	SocialFoodPatchYieldField sim.FieldID = 1 + iota
	SocialFoodPatchPulsesField
	SocialFoodPatchProducedField
	SocialFoodPatchUnrealizedField
)
const (
	SocialFoodSlotStockField sim.FieldID = 1 + iota
	SocialFoodSlotPatchField
	SocialFoodSlotGatheredField
)
const (
	SocialFoodBagUnitsField sim.FieldID = 1 + iota
	SocialFoodBagSourceField
	SocialFoodBagGathererField
	SocialFoodBagLastDonorField
)
const (
	SocialFoodBodyEnergyField sim.FieldID = 1 + iota
	SocialFoodBodyHungerField
	SocialFoodBodyBasalSpentField
	SocialFoodBodyCapLostField
	SocialFoodBodyConsumedField
	SocialFoodBodyLastGatherHourField
)
const (
	SocialFoodRequestIDField sim.FieldID = 1 + iota
	SocialFoodRequestHourField
	SocialFoodRequestAddresseeField
	SocialFoodRequestClaimField
	SocialFoodRequestStatusField
	SocialFoodRequestsWitnessedField
	SocialFoodGiftsWitnessedField
	SocialFoodRefusalsWitnessedField
)
const (
	SocialFoodLastReplyHourField sim.FieldID = 1 + iota
	SocialFoodReceivedField
	SocialFoodGivenField
	SocialFoodNetField
)

func SocialFoodPatchID(index int) (sim.EntityID, error) {
	if index < 0 || index >= SocialFoodPatchCount {
		return 0, ErrSocialFoodContract
	}
	return sim.EntityID(3001 + index), nil
}
func SocialFoodSlotID(patchIndex, slotIndex int) (sim.EntityID, error) {
	if patchIndex < 0 || patchIndex >= SocialFoodPatchCount || slotIndex < 0 || slotIndex >= SocialFoodSlotsPerPatch {
		return 0, ErrSocialFoodContract
	}
	return sim.EntityID(4001 + patchIndex*SocialFoodSlotsPerPatch + slotIndex), nil
}
func SocialFoodActorPatchID(actor sim.EntityID) (sim.EntityID, error) {
	if actor < 1 || actor > SocialFoodActorCount {
		return 0, ErrSocialFoodContract
	}
	return SocialFoodPatchID(int(actor-1) / SocialFoodSlotsPerPatch)
}

// The only known dyad for each requester points to the next actor in its patch ring.
func SocialFoodKnownAddressee(actor sim.EntityID) (sim.EntityID, int64, error) {
	if _, err := SocialFoodActorPatchID(actor); err != nil {
		return 0, 0, err
	}
	target := actor + 1
	if actor == 8 || actor == 16 {
		target = actor - 7
	}
	affinity := int64(1)
	if actor == 4 || actor == 8 || actor == 12 || actor == 16 {
		affinity = 3
	}
	return target, affinity, nil
}
func SocialFoodPhaseTime(hour int, phase int) (sim.SimTime, error) {
	if hour < 0 || hour >= SocialFoodHorizonHours || phase < 0 || phase > 4 {
		return 0, ErrSocialFoodContract
	}
	at := sim.SimTime(hour)*sim.SimTime(SocialFoodHour) + sim.SimTime(SocialFoodGatherDuration) + 1
	if phase == 4 {
		return at + 3 + sim.SimTime(SocialFoodMealDuration), nil
	}
	return at + sim.SimTime(phase), nil // gather +1µs, request +2µs, reply +3µs, meal start +4µs
}
func socialFoodBounds(min, max float64) component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: min, HasMaximum: true, Maximum: max}
}
func socialFoodInteger(id sim.FieldID, name, unit string, min, max float64) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.IntegerKind}, Unit: unit, Bounds: socialFoodBounds(min, max), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}
}
func socialFoodRef(id sim.FieldID, name string) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "entity-id", PermittedStates: []sim.ValueState{sim.Present, sim.Missing}, Uncertainty: component.UncertaintyForbidden}
}
func socialFoodRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: SocialFoodRuleVersion, Name: "socialfood-" + name + "-v2"}
}
func socialFoodDescriptor(id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	projections := make([]component.ProjectionDescriptor, 0, len(fields))
	for _, f := range fields {
		if f.Type.Kind != sim.IntegerKind {
			continue
		}
		projections = append(projections, component.ProjectionDescriptor{ID: sim.ProjectionID(f.ID), Version: SocialFoodProjectionVersion, Name: name + "-" + f.Name, SourceFields: []sim.FieldID{f.ID}, Coefficients: []float64{1}, Unit: f.Unit, Bounds: f.Bounds, MissingBehavior: component.PreserveSourceState})
	}
	return component.ComponentDescriptor{TypeID: id, SchemaVersion: SocialFoodSchemaVersion, Name: name, Fields: fields, AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: rules, Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}
func SocialFoodPatchDescriptor() component.ComponentDescriptor {
	return socialFoodDescriptor(SocialFoodPatchTypeID, "socialfood-patch-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "yield", "food/hour", 0, 8), socialFoodInteger(2, "pulses", "hours", 0, 168), socialFoodInteger(3, "produced", "food", 0, 1344), socialFoodInteger(4, "unrealized", "food", 0, 1344)}, socialFoodRule(SocialFoodProduceRule, "produce"))
}
func SocialFoodSlotDescriptor() component.ComponentDescriptor {
	return socialFoodDescriptor(SocialFoodSlotTypeID, "socialfood-slot-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "stock", "food", 0, 1), socialFoodRef(2, "patch"), socialFoodInteger(3, "gathered", "food", 0, 168)}, socialFoodRule(SocialFoodProduceRule, "produce"), socialFoodRule(SocialFoodGatherRule, "gather"))
}
func SocialFoodBagDescriptor() component.ComponentDescriptor {
	return socialFoodDescriptor(SocialFoodBagTypeID, "socialfood-bag-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "units", "food", 0, 1), socialFoodRef(2, "source-patch"), socialFoodRef(3, "original-gatherer"), socialFoodRef(4, "last-donor")}, socialFoodRule(SocialFoodGatherRule, "gather"), socialFoodRule(SocialFoodGiftRule, "gift"), socialFoodRule(SocialFoodConsumeRule, "consume"))
}
func SocialFoodBodyDescriptor() component.ComponentDescriptor {
	return socialFoodDescriptor(SocialFoodBodyTypeID, "socialfood-body-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "energy", "energy", 0, 12), socialFoodInteger(2, "hunger", "hours", 0, 24), socialFoodInteger(3, "basal-spent", "energy", 0, 168), socialFoodInteger(4, "cap-lost", "energy", 0, 168), socialFoodInteger(5, "consumed-home-patch", "food", 0, 168), socialFoodInteger(6, "last-gather-hour", "hour", -1, 167)}, socialFoodRule(SocialFoodGatherRule, "gather"), socialFoodRule(SocialFoodConsumeRule, "consume"), socialFoodRule(SocialFoodBasalRule, "basal"))
}
func SocialFoodRequestDescriptor() component.ComponentDescriptor {
	return socialFoodDescriptor(SocialFoodRequestTypeID, "socialfood-request-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "request-id", "request-id", 0, 168*16), socialFoodInteger(2, "hour", "hour", -1, 167), socialFoodRef(3, "addressee"), socialFoodInteger(4, "reported-urgency", "claim", 0, 2), socialFoodInteger(5, "status", "state", 0, 5), socialFoodInteger(6, "outgoing-requests", "requests", 0, 168), socialFoodInteger(7, "outgoing-gifts", "gifts", 0, 168), socialFoodInteger(8, "outgoing-refusals", "refusals", 0, 168)}, socialFoodRule(SocialFoodRequestRule, "request"), socialFoodRule(SocialFoodGiftRule, "gift"), socialFoodRule(SocialFoodRefuseRule, "refuse"), socialFoodRule(SocialFoodExpireRule, "expire"), socialFoodRule(SocialFoodWithdrawRule, "withdraw"))
}
func SocialFoodLedgerDescriptor() component.ComponentDescriptor {
	d := socialFoodDescriptor(SocialFoodLedgerTypeID, "socialfood-ledger-v2", []component.FieldDescriptor{
		socialFoodInteger(1, "last-reply-hour", "hour", -1, 167), socialFoodInteger(2, "received", "food", 0, 168), socialFoodInteger(3, "given", "food", 0, 168), socialFoodInteger(4, "net-received-minus-given", "food", -168, 168)}, socialFoodRule(SocialFoodGiftRule, "gift"), socialFoodRule(SocialFoodRefuseRule, "refuse"))
	// The stored net is checked by validSocialFoodLedger; the public metric is
	// independently derived from the two witnessed unit counters.
	for i := range d.Projections {
		if d.Projections[i].ID == sim.ProjectionID(SocialFoodNetField) {
			d.Projections[i].SourceFields = []sim.FieldID{SocialFoodReceivedField, SocialFoodGivenField}
			d.Projections[i].Coefficients = []float64{1, -1}
		}
	}
	return d
}
func SocialFoodRegistry() (component.Registry, error) {
	b := component.NewRegistryBuilder()
	for _, d := range []component.ComponentDescriptor{SocialFoodPatchDescriptor(), SocialFoodSlotDescriptor(), SocialFoodBagDescriptor(), SocialFoodBodyDescriptor(), SocialFoodRequestDescriptor(), SocialFoodLedgerDescriptor()} {
		if err := b.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrSocialFoodContract, err)
		}
	}
	return b.Freeze()
}

// SocialFoodExpireProposal emits only the checked pending->expired status
// transition. There is no bag, energy, or assistance-counter delta on expiry.
// The trusted runner must supply a fresh post-reply authoritative request,
// not a cached pending value that may already have been accepted or refused.
func SocialFoodExpireProposal(key string, hour int, at sim.SimTime, owner sim.EntityID, pending SocialFoodRequestState) (kernel.Proposal, error) {
	if key == "" {
		return kernel.Proposal{}, ErrSocialFoodContract
	}
	expired, err := SocialFoodExpire(hour, at, owner, pending)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{
		Key: key, Time: at, Cause: kernel.Cause{Actor: owner}, Rule: SocialFoodExpireRule, RuleVersion: SocialFoodRuleVersion,
		Patches: []component.Patch{{Entity: owner, Component: SocialFoodRequestTypeID, SchemaVersion: SocialFoodSchemaVersion, Field: SocialFoodRequestStatusField, Value: sim.IntegerValue(int64(expired.Status))}},
	}, nil
}

// SocialFoodGiftProposal builds an atomic proposal only from the result of a
// checked pending request and independent donor consent. The kernel validates
// schema and atomic patch admission, not cross-component semantics: its
// authority must remain confined to this trusted harness, never a callback.
// Raw in-range gift-rule patches bypass this helper in the kernel; the runner
// must use this constructor exclusively and journal replay must verify the
// complete consent/request/transfer linkage, not just rule IDs or schema.
func SocialFoodGiftProposal(key string, at sim.SimTime, hour int, donor, recipient sim.EntityID, consent SocialFoodConsent, pending SocialFoodRequestState, donorBefore, recipientBefore SocialFoodActorState, donorLedger, recipientLedger SocialFoodLedgerState) (kernel.Proposal, error) {
	if key == "" || at < 0 {
		return kernel.Proposal{}, ErrSocialFoodContract
	}
	want, err := SocialFoodPhaseTime(hour, 2)
	if err != nil || at != want {
		return kernel.Proposal{}, ErrSocialFoodContract
	}
	request, giver, receiver, given, received, err := SocialFoodAccept(hour, donor, recipient, consent, pending, donorBefore, recipientBefore, donorLedger, recipientLedger)
	if err != nil {
		return kernel.Proposal{}, err
	}
	integer := func(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, value int64) component.Patch {
		return component.Patch{Entity: entity, Component: typ, SchemaVersion: SocialFoodSchemaVersion, Field: field, Value: sim.IntegerValue(value)}
	}
	ref := func(entity sim.EntityID, field sim.FieldID, id sim.EntityID) component.Patch {
		var v sim.Value
		if id == 0 {
			v, _ = sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		} else {
			v, _ = sim.EntityRefValue(id)
		}
		return component.Patch{Entity: entity, Component: SocialFoodBagTypeID, SchemaVersion: SocialFoodSchemaVersion, Field: field, Value: v}
	}
	patches := []component.Patch{
		integer(recipient, SocialFoodRequestTypeID, SocialFoodRequestStatusField, int64(request.Status)), integer(recipient, SocialFoodRequestTypeID, SocialFoodGiftsWitnessedField, request.Gifts),
		integer(donor, SocialFoodBagTypeID, SocialFoodBagUnitsField, giver.Bag.Units), ref(donor, SocialFoodBagSourceField, 0), ref(donor, SocialFoodBagGathererField, 0), ref(donor, SocialFoodBagLastDonorField, 0),
		integer(recipient, SocialFoodBagTypeID, SocialFoodBagUnitsField, receiver.Bag.Units), ref(recipient, SocialFoodBagSourceField, receiver.Bag.Source), ref(recipient, SocialFoodBagGathererField, receiver.Bag.Gatherer), ref(recipient, SocialFoodBagLastDonorField, receiver.Bag.LastDonor),
		integer(donor, SocialFoodLedgerTypeID, SocialFoodLastReplyHourField, given.LastReplyHour), integer(donor, SocialFoodLedgerTypeID, SocialFoodGivenField, given.Given), integer(donor, SocialFoodLedgerTypeID, SocialFoodNetField, given.Net),
		integer(recipient, SocialFoodLedgerTypeID, SocialFoodReceivedField, received.Received), integer(recipient, SocialFoodLedgerTypeID, SocialFoodNetField, received.Net),
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: donor}, Rule: SocialFoodGiftRule, RuleVersion: SocialFoodRuleVersion, Patches: patches}, nil
}
