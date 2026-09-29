package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// Capacity v3 checkpoint bundles (ticket 13 persistence lane) are a
// separately versioned format with a distinct manifest: they never load or
// migrate food-flow v1, social-food v2, S6, or any other pilot's sections. A
// bundle exists only at the neutral h0 pulse boundary, before any claim or
// capital event. Enabling or disabling the capacity branch for a continued
// run happens only through an explicit recorded Branch whose intervention is
// carried in the manifest lineage and re-pinned in the journal configuration;
// the pinned policy bytes are never edited.
const capacityCheckpointFormat uint32 = 1

// capacityMaxBranchDepth bounds lineage chains so a forged manifest cannot
// claim an unbounded ancestry, and so Branch refuses pre-publication any
// child that could never be restored.
const capacityMaxBranchDepth = 63

var ErrCapacityCheckpoint = errors.New("invalid or incompatible capacity v3 checkpoint")

// Distinct v3 magics: no earlier pilot's bundle, journal, or strategy section
// can decode as a capacity v3 manifest, and vice versa.
var capacityCheckpointMagic = [4]byte{'A', 'W', 'C', '3'}
var capacityStrategyMagic = [4]byte{'A', 'W', 'C', 'T'}

const (
	capacityMaxStrategyPolicyBytes        = 128
	capacityStrategyFormat         uint32 = 1
)

type capacityManifest struct {
	yield             int64
	seed              uint64
	enabled           bool
	founders          []CapacityFounder // canonical order, validated
	steps             int
	time              sim.SimTime
	head              kernel.PortableHead
	policyFingerprint [32]byte // exact frozen policy content fingerprint
	parent            [32]byte // parent bundle digest; zero only for a root capture
	parentHead        kernel.PortableHead
	depth             uint32
	sections          [4][32]byte // journal, history, scheduler, strategy
}

func capacityBundleFlag(enabled bool) byte {
	if enabled {
		return 1
	}
	return 0
}

func encodeCapacityManifest(m capacityManifest) []byte {
	founders := capacityNormalizeFounders(m.founders)
	out := append([]byte(nil), capacityCheckpointMagic[:]...)
	for _, v := range []uint32{capacityCheckpointFormat, CapacityFormatVersion, CapacityJournalFormatVersion, uint32(CapacitySchemaVersion), CapacityRuleVersion, uint32(CapacityProjectionVersion), strategy.CapacityPolicyFormatV3, CapacityActorCount, CapacityPatchCount, CapacitySlotsPerPatch, CapacityHorizonHours} {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	for _, v := range []int64{int64(CapacityHour), int64(CapacityGatherDuration), int64(CapacityMealDuration), int64(CapacityBuildDuration), CapacityInitialEnergy, CapacityEnergyCapacity, CapacityHungerCapacity, CapacityBagCapacity, CapacityPointCostWip, CapacityYieldPerPoint, CapacityKMax, CapacityWipMax, CapacityGranaryMax, CapacityWearPeriod, CapacityWearDebtMax, CapacityPolicyTLow, CapacityNeverHour} {
		out = binary.BigEndian.AppendUint64(out, uint64(v))
	}
	out = binary.BigEndian.AppendUint64(out, uint64(m.yield))
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = append(out, capacityBundleFlag(m.enabled))
	out = binary.BigEndian.AppendUint32(out, uint32(len(founders)))
	for _, founder := range founders {
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Actor))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Capital))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Granary))
	}
	out = binary.BigEndian.AppendUint32(out, uint32(m.steps))
	out = binary.BigEndian.AppendUint64(out, uint64(m.time))
	out = appendJournalHead(out, m.head)
	out = append(out, m.policyFingerprint[:]...)
	out = append(out, m.parent[:]...)
	out = appendJournalHead(out, m.parentHead)
	out = binary.BigEndian.AppendUint32(out, m.depth)
	for _, hash := range m.sections {
		out = append(out, hash[:]...)
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func decodeCapacityManifest(data []byte) (capacityManifest, error) {
	var m capacityManifest
	if len(data) < sha256.Size {
		return m, ErrCapacityCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(data)-sha256.Size:]) {
		return m, ErrCapacityCheckpoint
	}
	// The founder list is the only variable-width part; the zero-founders
	// encoding bounds the fixed prefix, and exact consumption rejects
	// truncation, padding, and trailing fields.
	if len(body) < len(encodeCapacityManifest(capacityManifest{}))-sha256.Size {
		return m, ErrCapacityCheckpoint
	}
	r := journalReader{data: body}
	if !bytes.Equal(r.take(4), capacityCheckpointMagic[:]) {
		return m, ErrCapacityCheckpoint
	}
	for _, expected := range []uint32{capacityCheckpointFormat, CapacityFormatVersion, CapacityJournalFormatVersion, uint32(CapacitySchemaVersion), CapacityRuleVersion, uint32(CapacityProjectionVersion), strategy.CapacityPolicyFormatV3, CapacityActorCount, CapacityPatchCount, CapacitySlotsPerPatch, CapacityHorizonHours} {
		if r.u32() != expected {
			return m, ErrCapacityCheckpoint
		}
	}
	for _, expected := range []int64{int64(CapacityHour), int64(CapacityGatherDuration), int64(CapacityMealDuration), int64(CapacityBuildDuration), CapacityInitialEnergy, CapacityEnergyCapacity, CapacityHungerCapacity, CapacityBagCapacity, CapacityPointCostWip, CapacityYieldPerPoint, CapacityKMax, CapacityWipMax, CapacityGranaryMax, CapacityWearPeriod, CapacityWearDebtMax, CapacityPolicyTLow, CapacityNeverHour} {
		if int64(r.u64()) != expected {
			return m, ErrCapacityCheckpoint
		}
	}
	m.yield, m.seed = int64(r.u64()), r.u64()
	switch flag := r.u8(); flag {
	case 0:
		m.enabled = false
	case 1:
		m.enabled = true
	default:
		return capacityManifest{}, ErrCapacityCheckpoint
	}
	count := int(r.u32())
	if r.bad || count > CapacityActorCount {
		return capacityManifest{}, ErrCapacityCheckpoint
	}
	m.founders = make([]CapacityFounder, 0, count)
	for i := 0; i < count; i++ {
		founder := CapacityFounder{Actor: sim.EntityID(r.u64()), Capital: int64(r.u64()), Granary: int64(r.u64())}
		if r.bad {
			return capacityManifest{}, ErrCapacityCheckpoint
		}
		m.founders = append(m.founders, founder)
	}
	if !capacityValidFounders(m.founders) {
		return capacityManifest{}, ErrCapacityCheckpoint
	}
	m.steps, m.time = int(r.u32()), sim.SimTime(r.u64())
	m.head = r.head()
	copy(m.policyFingerprint[:], r.take(32))
	copy(m.parent[:], r.take(32))
	m.parentHead = r.head()
	m.depth = r.u32()
	for i := range m.sections {
		copy(m.sections[i][:], r.take(32))
	}
	// Only the neutral h0 pulse, before any claim or capital event, is a valid
	// bundle boundary; the branch flag changes exclusively through Branch.
	if r.bad || r.remaining() != 0 || m.yield < 0 || m.yield > CapacitySlotsPerPatch || m.steps != 1 || m.time != 0 ||
		m.head.GenesisVersion != 0 || m.head.Version != sim.WorldVersion(m.head.TipID) || m.head.TipTime != m.time {
		return capacityManifest{}, ErrCapacityCheckpoint
	}
	if m.depth == 0 {
		if m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
			return capacityManifest{}, ErrCapacityCheckpoint
		}
	} else {
		// A branch child continues its parent at the same verified boundary.
		if m.depth > capacityMaxBranchDepth || m.parent == ([32]byte{}) || m.parentHead != m.head {
			return capacityManifest{}, ErrCapacityCheckpoint
		}
	}
	return m, nil
}

type capacityStrategyBinding struct {
	actor      sim.EntityID
	refVersion uint32
}

// decodeCapacityStrategySection splits the pinned strategy section into the
// exact frozen policy bytes and the per-actor binding records. Content is
// verified against an independently constructed executable by
// verifyCapacityStrategy.
func decodeCapacityStrategySection(data []byte) (policy []byte, bindings []capacityStrategyBinding, err error) {
	r := journalReader{data: data}
	if !bytes.Equal(r.take(4), capacityStrategyMagic[:]) || r.u32() != capacityStrategyFormat {
		return nil, nil, ErrCapacityCheckpoint
	}
	policyLen := int(r.u16())
	if r.bad || policyLen < 1 || policyLen > capacityMaxStrategyPolicyBytes {
		return nil, nil, ErrCapacityCheckpoint
	}
	policy = r.take(policyLen)
	if r.bad || r.u32() != CapacityActorCount {
		return nil, nil, ErrCapacityCheckpoint
	}
	bindings = make([]capacityStrategyBinding, 0, CapacityActorCount)
	for i := 0; i < CapacityActorCount; i++ {
		b := capacityStrategyBinding{actor: sim.EntityID(r.u64()), refVersion: r.u32()}
		if r.bad || b.actor != sim.EntityID(i+1) {
			return nil, nil, ErrCapacityCheckpoint
		}
		bindings = append(bindings, b)
	}
	if r.bad || r.remaining() != 0 {
		return nil, nil, ErrCapacityCheckpoint
	}
	return policy, bindings, nil
}

// verifyCapacityStrategy compares the section's policy content and bindings
// against a freshly constructed executable runner. A well-formed same-ref
// changed policy fails closed here, not only at content decode time.
func verifyCapacityStrategy(data []byte, fresh *Capacity) error {
	policy, bindings, err := decodeCapacityStrategySection(data)
	if err != nil {
		return err
	}
	if fresh.bound[0] == nil || strategy.VerifyCapacityPolicy(policy, fresh.bound[0].Policy()) != nil {
		return ErrCapacityCheckpoint
	}
	for i, b := range bindings {
		if fresh.bound[i] == nil || b.refVersion != fresh.ref.Version ||
			fresh.bound[i].Binding() != (strategy.CapacityBinding{Actor: b.actor, Ref: fresh.ref}) {
			return ErrCapacityCheckpoint
		}
	}
	return nil
}

func encodeCapacityStrategy(f *Capacity) ([]byte, error) {
	if f.bound[0] == nil {
		return nil, ErrCapacityCheckpoint
	}
	policy, err := strategy.EncodeCapacityPolicy(f.bound[0].Policy())
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), capacityStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, capacityStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policy)))
	out = append(out, policy...)
	out = binary.BigEndian.AppendUint32(out, CapacityActorCount)
	for i := 0; i < CapacityActorCount; i++ {
		if f.bound[i] == nil || f.bound[i].Policy() != f.bound[0].Policy() ||
			f.bound[i].Binding() != (strategy.CapacityBinding{Actor: sim.EntityID(i + 1), Ref: f.ref}) {
			return nil, ErrCapacityCheckpoint
		}
		out = binary.BigEndian.AppendUint64(out, uint64(i+1))
		out = binary.BigEndian.AppendUint32(out, f.bound[i].Binding().Ref.Version)
	}
	return out, nil
}

// verifyCapacityNeutralScheduler proves the portable queue is exactly the
// wake set produced by the runner's own neutral h0 clock protocol: one
// committed pulse batch with no claim or capital evidence, no open
// activities, every roster fiber alive at registration revision, the clock
// fiber armed for the next pulse, and one claim wake per actor — no missing
// or extra wakes.
func verifyCapacityNeutralScheduler(k *kernel.Kernel, journal []CapacityBatch, snap scheduler.Snapshot) error {
	if k == nil || len(journal) != 1 || len(journal[0].Attempts) != 0 || journal[0].Time != 0 {
		return ErrCapacityCheckpoint
	}
	if snap.Time != 0 || !snap.HasClosed || snap.Closed != 0 || snap.NextToken != 0 || len(snap.Fibers) != CapacityActorCount+1 {
		return ErrCapacityCheckpoint
	}
	claim, err := CapacityPhaseTime(0, CapacityPhaseClaim)
	if err != nil {
		return err
	}
	nextPulse, err := CapacityHourTime(1)
	if err != nil {
		return err
	}
	expected := make(map[scheduler.Wake]bool, CapacityActorCount+1)
	for i, fiber := range snap.Fibers {
		want := sim.EntityID(i + 1)
		if i == CapacityActorCount {
			want = capacityClockActor
		}
		if fiber.Actor != want || fiber.Lifecycle != scheduler.Alive || fiber.Revision != 1 || fiber.Activity != nil {
			return ErrCapacityCheckpoint
		}
		if i < CapacityActorCount {
			expected[scheduler.Wake{Actor: want, At: claim, Cause: scheduler.WakeNeedThreshold}] = true
		} else {
			expected[scheduler.Wake{Actor: want, At: nextPulse, Cause: scheduler.WakeAudit}] = true
		}
	}
	if len(expected) != len(snap.Wakes) {
		return ErrCapacityCheckpoint
	}
	for _, wake := range snap.Wakes {
		if !expected[wake] {
			return ErrCapacityCheckpoint
		}
		delete(expected, wake)
	}
	if len(expected) != 0 {
		return ErrCapacityCheckpoint
	}
	return nil
}

// capacityBoundsTable compiles gate G7's declared field bounds from the frozen
// v3 registry. Integer fields only; ref fields carry no bounds.
type capacityBoundsTable map[sim.ComponentTypeID]map[sim.FieldID][2]int64

func capacityBoundsTableForRegistry() (capacityBoundsTable, error) {
	reg, err := CapacityRegistry()
	if err != nil {
		return nil, err
	}
	out := make(capacityBoundsTable)
	for _, typ := range reg.TypeIDs() {
		descriptor, err := reg.Describe(typ)
		if err != nil {
			return nil, err
		}
		fields := make(map[sim.FieldID][2]int64)
		for _, f := range descriptor.Fields {
			if f.Type.Kind != sim.IntegerKind || !f.Bounds.HasMinimum || !f.Bounds.HasMaximum {
				continue
			}
			fields[f.ID] = [2]int64{int64(f.Bounds.Minimum), int64(f.Bounds.Maximum)}
		}
		out[typ] = fields
	}
	return out, nil
}

// capacityProjectionBounds checks gate G7 on one reconstructed hourly
// projection: every integer field of every patch, slot, and actor row sits
// inside its declared descriptor bounds.
func capacityProjectionBounds(check CapacityCheckpoint, table capacityBoundsTable) error {
	within := func(typ sim.ComponentTypeID, field sim.FieldID, value int64) error {
		bounds, ok := table[typ][field]
		if !ok || value < bounds[0] || value > bounds[1] {
			return fmt.Errorf("%w: component %d field %d value %d outside declared bounds", ErrCapacityCheckpoint, typ, field, value)
		}
		return nil
	}
	for i, p := range check.Patches {
		if _, err := CapacityPatchID(i); err != nil {
			return err
		}
		for field, value := range map[sim.FieldID]int64{CapacityPatchYieldField: p.Yield, CapacityPatchPulsesField: p.Pulses, CapacityPatchProducedField: p.Produced, CapacityPatchUnrealizedField: p.Unrealized} {
			if err := within(CapacityPatchTypeID, field, value); err != nil {
				return err
			}
		}
		for j, s := range check.Slots[i] {
			if _, err := CapacitySlotID(i, j); err != nil {
				return err
			}
			if err := within(CapacitySlotTypeID, CapacitySlotStockField, s.Stock); err != nil {
				return err
			}
			if err := within(CapacitySlotTypeID, CapacitySlotGatheredField, s.Gathered); err != nil {
				return err
			}
		}
	}
	for i, a := range check.Actors {
		actor := sim.EntityID(i + 1)
		for _, spec := range []struct {
			typ   sim.ComponentTypeID
			field sim.FieldID
			value int64
		}{
			{CapacityBodyTypeID, CapacityBodyEnergyField, a.Body.Energy},
			{CapacityBodyTypeID, CapacityBodyHungerField, a.Body.Hunger},
			{CapacityBodyTypeID, CapacityBodyBasalSpentField, a.Body.BasalSpent},
			{CapacityBodyTypeID, CapacityBodyCapLostField, a.Body.CapLost},
			{CapacityBodyTypeID, CapacityBodyConsumedField, a.Body.Consumed},
			{CapacityBodyTypeID, CapacityBodyLastGatherHourField, a.Body.LastGatherHour},
			{CapacityWorksiteTypeID, CapacityWorksiteCapitalField, a.Worksite.Capital},
			{CapacityWorksiteTypeID, CapacityWorksiteWipField, a.Worksite.Wip},
			{CapacityWorksiteTypeID, CapacityWorksiteWearDebtField, a.Worksite.WearDebt},
			{CapacityWorksiteTypeID, CapacityWorksiteInvestedUnitsField, a.Worksite.InvestedUnits},
			{CapacityWorksiteTypeID, CapacityWorksitePointsCreatedField, a.Worksite.PointsCreated},
			{CapacityWorksiteTypeID, CapacityWorksitePointsDecayedField, a.Worksite.PointsDecayed},
			{CapacityWorksiteTypeID, CapacityWorksiteLastBuildHourField, a.Worksite.LastBuildHour},
			{CapacityGranaryTypeID, CapacityGranaryStockField, a.Granary.Stock},
			{CapacityGranaryTypeID, CapacityGranaryYieldTotalField, a.Granary.YieldTotal},
			{CapacityGranaryTypeID, CapacityGranaryYieldUnrealizedField, a.Granary.YieldUnrealized},
			{CapacityGranaryTypeID, CapacityGranaryStoredMealsField, a.Granary.StoredMeals},
			{CapacityGranaryTypeID, CapacityGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour},
		} {
			if err := within(spec.typ, spec.field, spec.value); err != nil {
				return fmt.Errorf("actor %d: %w", actor, err)
			}
		}
	}
	return nil
}

// capacityCheckpointsFromHistory derives the hourly projection trace from
// verified accepted deltas rather than a second mutable ledger, then compares
// the reconstruction against the authoritative snapshot. The mirrored h0 seed
// state (including any recorded founder endowment) must match the bundle's
// genesis or the final authoritative comparison fails. Gates G1-G3 are
// re-checked at every hour and gate G7 bounds every field, so a raw-kernel
// in-range patch bypass cannot survive replay silently.
func capacityCheckpointsFromHistory(k *kernel.Kernel, journal []CapacityBatch, yield int64, founders []CapacityFounder) ([]CapacityCheckpoint, error) {
	table, err := capacityBoundsTableForRegistry()
	if err != nil {
		return nil, err
	}
	var patches [CapacityPatchCount]CapacityPatchState
	var slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	var actors [CapacityActorCount]CapacityActorState
	for i := range patches {
		patches[i].Yield = yield
	}
	endowment := make(map[sim.EntityID]CapacityFounder, len(founders))
	for _, founder := range founders {
		endowment[founder.Actor] = founder
	}
	for i := range actors {
		founder := endowment[sim.EntityID(i+1)]
		actors[i] = CapacityActorState{
			Body:     CapacityBodyState{Energy: CapacityInitialEnergy, LastGatherHour: CapacityNeverHour},
			Worksite: CapacityWorksiteState{Capital: founder.Capital, InvestedUnits: CapacityPointCostWip * founder.Capital, PointsCreated: founder.Capital, LastBuildHour: CapacityNeverHour},
			Granary:  CapacityGranaryState{Stock: founder.Granary, YieldTotal: founder.Granary, LastStoredMealHour: CapacityNeverHour},
		}
	}
	events := k.Events()
	index := 0
	out := make([]CapacityCheckpoint, 0, CapacityHorizonHours+1)
	for _, batch := range journal {
		for index < len(events) && events[index].Time == batch.Time {
			for _, delta := range events[index].Deltas {
				if err := applyCapacityCheckpointDelta(&patches, &slots, &actors, delta); err != nil {
					return nil, err
				}
			}
			index++
		}
		if batch.Version != sim.WorldVersion(index) {
			return nil, ErrCapacityCheckpoint
		}
		if batch.Time%sim.SimTime(CapacityHour) != 0 {
			continue
		}
		hour := int(batch.Time / sim.SimTime(CapacityHour))
		if hour != len(out) {
			return nil, ErrCapacityCheckpoint
		}
		balance, err := CapacityCheckConservation(patches, slots, actors)
		if err != nil {
			return nil, err
		}
		check := CapacityCheckpoint{Hour: hour, Version: batch.Version, Balance: balance, Patches: patches, Slots: slots, Actors: actors}
		for _, actor := range actors {
			if actor.Body.Energy > 0 {
				check.Alive++
			}
		}
		if err := capacityProjectionBounds(check, table); err != nil {
			return nil, fmt.Errorf("capacity hour %d: %w", hour, err)
		}
		out = append(out, check)
	}
	if index != len(events) {
		return nil, ErrCapacityCheckpoint
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	p, s, a, err := capacityStates(view)
	if err != nil || p != patches || s != slots || a != actors {
		return nil, ErrCapacityCheckpoint
	}
	return out, nil
}

func capacityDeltaRef(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing {
		return 0, nil
	}
	return v.EntityRef()
}

// applyCapacityCheckpointDelta mirrors every authorized v3 event delta into
// the shadow projection state. Unlisted component/field combinations are
// rejected instead of ignored, so an unexpected accepted delta cannot pass
// silently; the caller's final authoritative-state comparison catches any
// remaining value divergence.
func applyCapacityCheckpointDelta(p *[CapacityPatchCount]CapacityPatchState, s *[CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState, a *[CapacityActorCount]CapacityActorState, d component.FieldDelta) error {
	integer := func() (int64, error) { return d.After.Integer() }
	switch d.Component {
	case CapacityPatchTypeID:
		if d.Entity < 1001 || d.Entity >= 1001+CapacityPatchCount {
			return ErrCapacityCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &p[d.Entity-1001]
		switch d.Field {
		case CapacityPatchPulsesField:
			v.Pulses = n
		case CapacityPatchProducedField:
			v.Produced = n
		case CapacityPatchUnrealizedField:
			v.Unrealized = n
		default:
			return ErrCapacityCheckpoint
		}
	case CapacitySlotTypeID:
		if d.Entity < 2001 || d.Entity >= 2001+CapacityPatchCount*CapacitySlotsPerPatch {
			return ErrCapacityCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &s[(d.Entity-2001)/CapacitySlotsPerPatch][(d.Entity-2001)%CapacitySlotsPerPatch]
		switch d.Field {
		case CapacitySlotStockField:
			v.Stock = n
		case CapacitySlotGatheredField:
			v.Gathered = n
		default:
			return ErrCapacityCheckpoint
		}
	case CapacityBagTypeID:
		if d.Entity < 1 || d.Entity > CapacityActorCount {
			return ErrCapacityCheckpoint
		}
		v := &a[d.Entity-1].Bag
		switch d.Field {
		case CapacityBagUnitsField:
			n, err := integer()
			if err != nil {
				return err
			}
			v.Units = n
		case CapacityBagSourceField:
			id, err := capacityDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.Source = id
		default:
			return ErrCapacityCheckpoint
		}
	case CapacityBodyTypeID:
		if d.Entity < 1 || d.Entity > CapacityActorCount {
			return ErrCapacityCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &a[d.Entity-1].Body
		switch d.Field {
		case CapacityBodyEnergyField:
			v.Energy = n
		case CapacityBodyHungerField:
			v.Hunger = n
		case CapacityBodyBasalSpentField:
			v.BasalSpent = n
		case CapacityBodyCapLostField:
			v.CapLost = n
		case CapacityBodyConsumedField:
			v.Consumed = n
		case CapacityBodyLastGatherHourField:
			v.LastGatherHour = n
		default:
			return ErrCapacityCheckpoint
		}
	case CapacityWorksiteTypeID:
		if d.Entity < 1 || d.Entity > CapacityActorCount {
			return ErrCapacityCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &a[d.Entity-1].Worksite
		switch d.Field {
		case CapacityWorksiteCapitalField:
			v.Capital = n
		case CapacityWorksiteWipField:
			v.Wip = n
		case CapacityWorksiteWearDebtField:
			v.WearDebt = n
		case CapacityWorksiteInvestedUnitsField:
			v.InvestedUnits = n
		case CapacityWorksitePointsCreatedField:
			v.PointsCreated = n
		case CapacityWorksitePointsDecayedField:
			v.PointsDecayed = n
		case CapacityWorksiteLastBuildHourField:
			v.LastBuildHour = n
		default:
			return ErrCapacityCheckpoint
		}
	case CapacityGranaryTypeID:
		if d.Entity < 1 || d.Entity > CapacityActorCount {
			return ErrCapacityCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &a[d.Entity-1].Granary
		switch d.Field {
		case CapacityGranaryStockField:
			v.Stock = n
		case CapacityGranaryYieldTotalField:
			v.YieldTotal = n
		case CapacityGranaryYieldUnrealizedField:
			v.YieldUnrealized = n
		case CapacityGranaryStoredMealsField:
			v.StoredMeals = n
		case CapacityGranaryLastStoredMealHourField:
			v.LastStoredMealHour = n
		default:
			return ErrCapacityCheckpoint
		}
	default:
		return ErrCapacityCheckpoint
	}
	return nil
}

// SaveCheckpoint publishes the neutral h0 pilot bundle: all five sections are
// captured under the coordinator lock, only at the committed h0 pulse boundary
// before any claim or capital event, and never at an existing destination. The
// returned digest identifies the bundle for Branch lineage records.
func (f *Capacity) SaveCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	if f.steps != 1 || len(f.journal) != 1 || len(f.journal[0].Attempts) != 0 || f.journal[0].Time != 0 ||
		f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 ||
		len(f.current) != 0 || len(f.denied) != 0 || len(f.meals) != 0 {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later: the independent constructor pins the current policy contract.
	fresh, err := NewCapacity(CapacityOptions{Yield: f.yield, Seed: f.seed, Workers: 1, Enabled: f.enabled, Founders: f.founders})
	if err != nil {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	if f.ref != fresh.ref {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	strategyBytes, err := encodeCapacityStrategy(f)
	if err != nil {
		return [32]byte{}, err
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	snap := f.sched.Snapshot()
	if err := verifyCapacityNeutralScheduler(f.k, f.journal, snap); err != nil {
		return [32]byte{}, err
	}
	if err := verifyCapacityStrategy(strategyBytes, fresh); err != nil {
		return [32]byte{}, err
	}
	journalBytes, err := EncodeCapacityJournal(f.journal, f.k, head, snap.Time, f.journalConfig())
	if err != nil {
		return [32]byte{}, err
	}
	checks, err := capacityCheckpointsFromHistory(f.k, f.journal, f.yield, f.founders)
	if err != nil || !equalCapacityCheckpoints(checks, f.checkpoints) {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	fingerprint, err := strategy.CapacityPolicyFingerprint(f.bound[0].Policy())
	if err != nil {
		return [32]byte{}, err
	}
	m := capacityManifest{yield: f.yield, seed: f.seed, enabled: f.enabled, founders: capacityNormalizeFounders(f.founders),
		steps: f.steps, time: snap.Time, head: head, policyFingerprint: fingerprint}
	for i, part := range [][]byte{journalBytes, history, portable, strategyBytes} {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeCapacityManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: strategyBytes},
	})
}

// restoreCapacitySections rebuilds an independent executable runner from
// already hash-checked bundle sections. Every section is validated against
// independently constructed registries and the declared scenario before the
// returned runner becomes visible. The worker count comes from the caller,
// never from the bundle.
func restoreCapacitySections(sections []checkpoint.Section, m capacityManifest, expected CapacityOptions) (*Capacity, error) {
	fail := func(err error) (*Capacity, error) {
		return nil, fmt.Errorf("%w: %v", ErrCapacityCheckpoint, err)
	}
	fresh, err := NewCapacity(expected)
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil {
		return fail(err)
	}
	if genesis.GenesisHash != m.head.GenesisHash || genesis.RegistryFingerprint != m.head.RegistryFingerprint {
		return nil, ErrCapacityCheckpoint
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil {
		return fail(err)
	}
	if head != m.head {
		return nil, ErrCapacityCheckpoint
	}
	fresh.k = k
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil {
		return fail(err)
	}
	if sched.Time() != m.time {
		return nil, ErrCapacityCheckpoint
	}
	fresh.sched = sched
	journal, err := DecodeCapacityJournalWithScheduler(sections[0].Data, k, head, sched, fresh.journalConfig())
	if err != nil {
		return fail(err)
	}
	if len(journal) != m.steps {
		return nil, ErrCapacityCheckpoint
	}
	snap := sched.Snapshot()
	if err := verifyCapacityNeutralScheduler(k, journal, snap); err != nil {
		return fail(err)
	}
	if err := verifyCapacityStrategy(sections[4].Data, fresh); err != nil {
		return fail(err)
	}
	fingerprint, err := strategy.CapacityPolicyFingerprint(fresh.bound[0].Policy())
	if err != nil {
		return fail(err)
	}
	if fingerprint != m.policyFingerprint {
		return nil, ErrCapacityCheckpoint
	}
	checks, err := capacityCheckpointsFromHistory(k, journal, fresh.yield, fresh.founders)
	if err != nil {
		return fail(err)
	}
	if len(checks) != 1 || checks[0].Hour != 0 {
		return nil, ErrCapacityCheckpoint
	}
	fresh.journal, fresh.checkpoints, fresh.steps = journal, checks, m.steps
	for i, fiber := range snap.Fibers {
		if i < CapacityActorCount {
			fresh.pulseLifecycle[i] = fiber.Lifecycle
		}
	}
	return fresh, nil
}

func capacityCheckedSections(path string) ([]checkpoint.Section, [32]byte, capacityManifest, error) {
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return nil, [32]byte{}, capacityManifest{}, fmt.Errorf("%w: %v", ErrCapacityCheckpoint, err)
	}
	m, err := decodeCapacityManifest(sections[2].Data)
	if err != nil {
		return nil, [32]byte{}, capacityManifest{}, err
	}
	for i, part := range [][]byte{sections[0].Data, sections[1].Data, sections[3].Data, sections[4].Data} {
		if sha256.Sum256(part) != m.sections[i] {
			return nil, [32]byte{}, capacityManifest{}, ErrCapacityCheckpoint
		}
	}
	return sections, digest, m, nil
}

// capacityManifestMatches reports whether the bundle's recorded scenario is
// exactly the declared one: yield, seed, intervention flag, and founder
// endowments (canonical order on both sides).
func capacityManifestMatches(m capacityManifest, expected CapacityOptions) bool {
	if m.yield != expected.Yield || m.seed != expected.Seed || m.enabled != expected.Enabled {
		return false
	}
	founders := capacityNormalizeFounders(expected.Founders)
	if len(m.founders) != len(founders) {
		return false
	}
	for i, founder := range founders {
		if m.founders[i] != founder {
			return false
		}
	}
	return true
}

// RestoreCapacityCheckpoint rebuilds an independent executable runner from a
// verified bundle. A different worker count is allowed; the declared scenario,
// endowment and intervention flag must identify the bundle exactly, so an
// enabled/disabled continuation always requires an explicit Branch record.
func RestoreCapacityCheckpoint(path string, expected CapacityOptions) (*Capacity, [32]byte, error) {
	if expected.Workers < 1 {
		return nil, [32]byte{}, ErrCapacityCheckpoint
	}
	sections, digest, m, err := capacityCheckedSections(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if !capacityManifestMatches(m, expected) {
		return nil, [32]byte{}, ErrCapacityCheckpoint
	}
	fresh, err := restoreCapacitySections(sections, m, expected)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return fresh, digest, nil
}

// BranchCapacityCheckpoint verifies a neutral bundle end to end under its
// recorded flag, then publishes a child bundle that continues the same world
// under the explicit enable/disable intervention. The child's manifest lineage
// records the parent digest and verified parent head, and its journal section
// is re-pinned to the intervention flag; the policy, history and scheduler
// sections stay byte-identical to the verified parent. Publication is atomic
// and never replaces an existing file; any verification failure publishes
// nothing, including a child whose lineage depth would exceed the
// predeclared restore bound.
func BranchCapacityCheckpoint(path string, expected CapacityOptions, enabled bool, dest string) ([32]byte, error) {
	sections, digest, m, err := capacityCheckedSections(path)
	if err != nil {
		return [32]byte{}, err
	}
	if expected.Workers < 1 || !capacityManifestMatches(m, expected) {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	// Refuse before publication a child whose manifest depth would exceed the
	// restore-time decode bound; such a bundle could never be restored.
	if m.depth >= capacityMaxBranchDepth {
		return [32]byte{}, ErrCapacityCheckpoint
	}
	parent, err := restoreCapacitySections(sections, m, CapacityOptions{Yield: m.yield, Seed: m.seed, Workers: 1, Enabled: m.enabled, Founders: m.founders})
	if err != nil {
		return [32]byte{}, err
	}
	// The only permitted intervention is the recorded enable/disable decision;
	// a non-neutral prefix could not follow it and fails closed here.
	config := parent.journalConfig()
	config.Enabled = enabled
	journalBytes, err := EncodeCapacityJournal(parent.journal, parent.k, m.head, m.time, config)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrCapacityCheckpoint, err)
	}
	child := m
	child.enabled = enabled
	child.parent = digest
	child.parentHead = m.head
	child.depth = m.depth + 1
	for i, part := range [][]byte{journalBytes, sections[1].Data, sections[3].Data, sections[4].Data} {
		child.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(dest, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: sections[1].Data},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeCapacityManifest(child)},
		{Name: checkpoint.Scheduler, Version: 1, Data: sections[3].Data},
		{Name: checkpoint.Strategy, Version: 1, Data: sections[4].Data},
	})
}

func equalCapacityCheckpoints(a, b []CapacityCheckpoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
