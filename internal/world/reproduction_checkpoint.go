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
	"reflect"
)

// Reproduction v4 checkpoint bundles (ticket 14 persistence lane) are a
// separately versioned format with a distinct manifest: they never load or
// migrate food-flow v1, social-food v2, survival S6, capacity v3, or any
// other pilot's sections, and no earlier pilot loads theirs. The pilot root
// (SaveCheckpoint) exists only at the neutral h0 pulse boundary, before any
// claim, capital, or birth event; a pulse-boundary continuation bundle
// (SaveResumeCheckpoint) may be captured at any later hourly boundary, after
// births included. Enabling births for a continued run happens only through
// an explicit recorded Branch whose intervention is carried in the manifest
// lineage and re-pinned in the journal configuration; the pinned policy bytes
// are never edited. There is no endowment-variant section in v4: births
// on/off and founder genomes are runner options recorded as run identity, and
// a restore refuses any manifest whose recorded scenario does not exactly
// match the declared options and the seed-derived founder genome rows.
const reproductionCheckpointFormat uint32 = 1

// reproductionMaxBranchDepth bounds lineage chains so a forged manifest
// cannot claim an unbounded ancestry, and so Branch refuses pre-publication
// any child that could never be restored.
const reproductionMaxBranchDepth = 63

var ErrReproductionCheckpoint = errors.New("invalid or incompatible reproduction v4 checkpoint")

// Distinct v4 magics: no earlier pilot's bundle or strategy section can
// decode as a reproduction v4 manifest or strategy section, and vice versa.
var reproductionCheckpointMagic = [4]byte{'A', 'W', 'R', '4'}
var reproductionStrategyMagic = [4]byte{'A', 'W', 'R', 'T'}

const (
	reproductionMaxStrategyPolicyBytes        = 128
	reproductionStrategyFormat         uint32 = 1
)

type reproductionManifest struct {
	yield             int64
	seed              uint64
	births            bool                           // the continuation flag: the recorded intervention on a branch child
	boundary          bool                           // the births flag of the captured boundary's wake topology
	founders          []ReproductionFounderGenomeRow // canonical order, validated
	steps             int
	time              sim.SimTime
	head              kernel.PortableHead
	policyFingerprint [32]byte // exact frozen policy content fingerprint
	parent            [32]byte // parent bundle digest; zero only for a root capture
	parentHead        kernel.PortableHead
	depth             uint32
	sections          [4][32]byte // journal, history, scheduler, strategy
}

func reproductionBundleFlag(births bool) byte {
	if births {
		return 1
	}
	return 0
}

func encodeReproductionManifest(m reproductionManifest) []byte {
	founders := reproductionNormalizeFounders(m.founders)
	out := append([]byte(nil), reproductionCheckpointMagic[:]...)
	for _, v := range []uint32{reproductionCheckpointFormat, ReproductionFormatVersion, ReproductionJournalFormatVersion, uint32(ReproductionSchemaVersion), ReproductionRuleVersion, uint32(ReproductionProjectionVersion), strategy.ReproductionPolicyFormatV4, ReproductionFounderCount, ReproductionPatchCount, ReproductionSlotsPerPatch, ReproductionHorizonHours} {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	for _, v := range []int64{int64(ReproductionHour), int64(ReproductionGatherDuration), int64(ReproductionMealDuration), int64(ReproductionBuildDuration), ReproductionFounderInitialEnergy, ReproductionEnergyCapacity, ReproductionHungerCapacity, ReproductionBagCapacity, ReproductionPointCostWip, ReproductionYieldPerPoint, ReproductionKMax, ReproductionWipMax, ReproductionGranaryMax, ReproductionWearPeriod, ReproductionWearDebtMax, ReproductionPolicyTLow, ReproductionNeverHour, ReproductionGenomeLocusMin, ReproductionGenomeLocusMax, ReproductionLifetimeBase, ReproductionLifetimeSpan} {
		out = binary.BigEndian.AppendUint64(out, uint64(v))
	}
	out = binary.BigEndian.AppendUint64(out, uint64(m.yield))
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = append(out, reproductionBundleFlag(m.births))
	out = append(out, reproductionBundleFlag(m.boundary))
	out = binary.BigEndian.AppendUint32(out, uint32(len(founders)))
	for _, founder := range founders {
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Actor))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.LocusM))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.LocusY))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.LocusW))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.ParentA))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.ParentB))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.BirthHour))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.DeathHour))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.DiedHour))
		out = binary.BigEndian.AppendUint64(out, uint64(founder.Genome.BirthGranaryPaid))
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

func decodeReproductionManifest(data []byte) (reproductionManifest, error) {
	var m reproductionManifest
	if len(data) < sha256.Size {
		return m, ErrReproductionCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(data)-sha256.Size:]) {
		return m, ErrReproductionCheckpoint
	}
	// The founder list is the only variable-width part; the zero-founders
	// encoding bounds the fixed prefix, and exact consumption rejects
	// truncation, padding, and trailing fields.
	if len(body) < len(encodeReproductionManifest(reproductionManifest{}))-sha256.Size {
		return m, ErrReproductionCheckpoint
	}
	r := journalReader{data: body}
	if !bytes.Equal(r.take(4), reproductionCheckpointMagic[:]) {
		return m, ErrReproductionCheckpoint
	}
	for _, expected := range []uint32{reproductionCheckpointFormat, ReproductionFormatVersion, ReproductionJournalFormatVersion, uint32(ReproductionSchemaVersion), ReproductionRuleVersion, uint32(ReproductionProjectionVersion), strategy.ReproductionPolicyFormatV4, ReproductionFounderCount, ReproductionPatchCount, ReproductionSlotsPerPatch, ReproductionHorizonHours} {
		if r.u32() != expected {
			return m, ErrReproductionCheckpoint
		}
	}
	for _, expected := range []int64{int64(ReproductionHour), int64(ReproductionGatherDuration), int64(ReproductionMealDuration), int64(ReproductionBuildDuration), ReproductionFounderInitialEnergy, ReproductionEnergyCapacity, ReproductionHungerCapacity, ReproductionBagCapacity, ReproductionPointCostWip, ReproductionYieldPerPoint, ReproductionKMax, ReproductionWipMax, ReproductionGranaryMax, ReproductionWearPeriod, ReproductionWearDebtMax, ReproductionPolicyTLow, ReproductionNeverHour, ReproductionGenomeLocusMin, ReproductionGenomeLocusMax, ReproductionLifetimeBase, ReproductionLifetimeSpan} {
		if int64(r.u64()) != expected {
			return m, ErrReproductionCheckpoint
		}
	}
	m.yield, m.seed = int64(r.u64()), r.u64()
	switch flag := r.u8(); flag {
	case 0:
		m.births = false
	case 1:
		m.births = true
	default:
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	switch flag := r.u8(); flag {
	case 0:
		m.boundary = false
	case 1:
		m.boundary = true
	default:
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	count := int(r.u32())
	// The founder set is frozen: exactly the sixteen ID-placed founders.
	if r.bad || count != ReproductionFounderCount {
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	m.founders = make([]ReproductionFounderGenomeRow, 0, count)
	for i := 0; i < count; i++ {
		founder := ReproductionFounderGenomeRow{Actor: sim.EntityID(r.u64()), Genome: ReproductionGenomeState{
			LocusM: int64(r.u64()), LocusY: int64(r.u64()), LocusW: int64(r.u64()),
			ParentA: sim.EntityID(r.u64()), ParentB: sim.EntityID(r.u64()),
			BirthHour: int64(r.u64()), DeathHour: int64(r.u64()), DiedHour: int64(r.u64()),
			BirthGranaryPaid: int64(r.u64())}}
		if r.bad {
			return reproductionManifest{}, ErrReproductionCheckpoint
		}
		m.founders = append(m.founders, founder)
	}
	if !validReproductionFounderGenomes(m.founders) {
		return reproductionManifest{}, ErrReproductionCheckpoint
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
	// Boundary and lineage rules. time == 0 ↔ steps == 1: the only zero-time
	// bundle is the neutral h0 pulse capture (or a Branch child of one); any
	// later bundle is a pulse-boundary continuation. Continuation bundles
	// stand alone in the lineage: the births intervention is recorded only on
	// h0 children, so a Branch parent is always the neutral root.
	if r.bad || r.remaining() != 0 || m.yield < 0 || m.yield > ReproductionSlotsPerPatch ||
		m.steps < 1 || m.steps > reproductionStepLimit ||
		m.time < 0 || m.time > sim.SimTime(ReproductionHorizonHours)*sim.SimTime(ReproductionHour) ||
		m.time%sim.SimTime(ReproductionHour) != 0 {
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	if (m.time == 0) != (m.steps == 1) {
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	// A root capture is taken as-is: the recorded intervention flag equals
	// the boundary's wake topology. Only a branch child (or a continuation of
	// one, whose journal carries both flags) may record an intervention that
	// differs from the boundary.
	if m.depth == 0 && m.time == 0 && m.births != m.boundary {
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	if m.time == 0 {
		if m.depth == 0 {
			if m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
				return reproductionManifest{}, ErrReproductionCheckpoint
			}
		} else {
			// A branch child continues its parent at the same verified boundary.
			if m.depth > reproductionMaxBranchDepth || m.parent == ([32]byte{}) || m.parentHead != m.head {
				return reproductionManifest{}, ErrReproductionCheckpoint
			}
		}
	} else if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
		// A pulse-boundary continuation stands alone in the lineage: the
		// births intervention is recorded only on h0 children.
		return reproductionManifest{}, ErrReproductionCheckpoint
	}
	return m, nil
}

type reproductionStrategyBinding struct {
	actor      sim.EntityID
	refVersion uint32
}

// decodeReproductionStrategySection splits the pinned strategy section into
// the exact frozen policy bytes and the variable-length per-entity binding
// table (entity→ref). Content is verified against an independently
// constructed executable and against the world's genome rows by
// verifyReproductionStrategy.
func decodeReproductionStrategySection(data []byte) (policy []byte, bindings []reproductionStrategyBinding, err error) {
	r := journalReader{data: data}
	if !bytes.Equal(r.take(4), reproductionStrategyMagic[:]) || r.u32() != reproductionStrategyFormat {
		return nil, nil, ErrReproductionCheckpoint
	}
	policyLen := int(r.u16())
	if r.bad || policyLen < 1 || policyLen > reproductionMaxStrategyPolicyBytes {
		return nil, nil, ErrReproductionCheckpoint
	}
	policy = r.take(policyLen)
	if r.bad {
		return nil, nil, ErrReproductionCheckpoint
	}
	count := int(r.u32())
	if r.bad || count < 1 || count > reproductionMaxRoster {
		return nil, nil, ErrReproductionCheckpoint
	}
	bindings = make([]reproductionStrategyBinding, 0, count)
	previous := sim.EntityID(0)
	for i := 0; i < count; i++ {
		b := reproductionStrategyBinding{actor: sim.EntityID(r.u64()), refVersion: r.u32()}
		if r.bad || b.actor <= previous {
			return nil, nil, ErrReproductionCheckpoint
		}
		previous = b.actor
		bindings = append(bindings, b)
	}
	if r.bad || r.remaining() != 0 {
		return nil, nil, ErrReproductionCheckpoint
	}
	return policy, bindings, nil
}

// verifyReproductionStrategy compares the section's policy content and the
// binding table against a freshly constructed executable runner and against
// the world's own genome rows: the bound entity set must be exactly the
// roster implied by the genome scan (every bound entity has genome rows, and
// no born genome row is left unbound). Founders are additionally checked
// against the fresh constructor's own bindings. A well-formed same-ref
// changed policy fails closed here, not only at content decode time.
func verifyReproductionStrategy(data []byte, fresh *Reproduction, view scheduler.SnapshotView) error {
	policy, bindings, err := decodeReproductionStrategySection(data)
	if err != nil {
		return err
	}
	if fresh == nil || fresh.bound[1] == nil || strategy.VerifyReproductionPolicy(policy, fresh.bound[1].Policy()) != nil {
		return ErrReproductionCheckpoint
	}
	births, err := reproductionBirthCount(view)
	if err != nil {
		return err
	}
	roster := make([]sim.EntityID, 0, ReproductionFounderCount+int(births))
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		roster = append(roster, actor)
	}
	for i := int64(0); i < births; i++ {
		id, err := ReproductionNewbornID(i)
		if err != nil {
			return err
		}
		roster = append(roster, id)
	}
	if len(bindings) != len(roster) {
		return ErrReproductionCheckpoint
	}
	for i, b := range bindings {
		if b.actor != roster[i] || b.refVersion != fresh.ref.Version {
			return ErrReproductionCheckpoint
		}
		if b.actor <= ReproductionFounderCount &&
			(fresh.bound[b.actor] == nil ||
				fresh.bound[b.actor].Binding() != (strategy.ReproductionBinding{Actor: b.actor, Ref: fresh.ref})) {
			return ErrReproductionCheckpoint
		}
	}
	return nil
}

// encodeReproductionStrategy pins the frozen policy content and one binding
// record per roster entity (founders first, then newborns in registration
// order — both ascending).
func encodeReproductionStrategy(f *Reproduction) ([]byte, error) {
	if f == nil || f.bound[1] == nil {
		return nil, ErrReproductionCheckpoint
	}
	policy := f.bound[1].Policy()
	policyBytes, err := strategy.EncodeReproductionPolicy(policy)
	if err != nil {
		return nil, ErrReproductionCheckpoint
	}
	if len(f.roster) < ReproductionFounderCount || len(f.roster) > reproductionMaxRoster {
		return nil, ErrReproductionCheckpoint
	}
	out := append([]byte(nil), reproductionStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, reproductionStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policyBytes)))
	out = append(out, policyBytes...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(f.roster)))
	previous := sim.EntityID(0)
	for _, id := range f.roster {
		bound := f.bound[id]
		if id <= previous || bound == nil || bound.Policy() != policy ||
			bound.Binding() != (strategy.ReproductionBinding{Actor: id, Ref: f.ref}) {
			return nil, ErrReproductionCheckpoint
		}
		previous = id
		out = binary.BigEndian.AppendUint64(out, uint64(id))
		out = binary.BigEndian.AppendUint32(out, bound.Binding().Ref.Version)
	}
	return out, nil
}

// reproductionExpectedBoundaryWakes builds the exact wake set at a pulse
// boundary: one claim wake per living actor — the reserved WakeBirth class
// for a newborn whose first claim is still pending, a NeedThreshold claim
// otherwise — the birth-request audit wake when births are enabled, and the
// clock's next pulse. The frozen WakeBirth class is part of every
// expectation set, so a missing, extra, or mis-caused reservation fails the
// exact match.
func reproductionExpectedBoundaryWakes(births bool, hour int, living []sim.EntityID, reserved map[sim.EntityID]int64) (map[scheduler.Wake]bool, error) {
	claim, err := ReproductionPhaseTime(hour, ReproductionPhaseClaim)
	if err != nil {
		return nil, err
	}
	var requestAt sim.SimTime
	if births {
		requestAt, err = ReproductionPhaseTime(hour, ReproductionPhaseBirthRequest)
		if err != nil {
			return nil, err
		}
	}
	expected := make(map[scheduler.Wake]bool, len(living)+2)
	for _, id := range living {
		if reserved[id] != 0 {
			// A reserved first wake is always the current hour's claim; any
			// other reservation would be a wake-topology violation.
			if reserved[id] != int64(hour) {
				return nil, ErrReproductionCheckpoint
			}
			expected[scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeBirth}] = true
		} else {
			expected[scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeNeedThreshold}] = true
		}
		if births {
			expected[scheduler.Wake{Actor: id, At: requestAt, Cause: scheduler.WakeAudit}] = true
		}
	}
	if hour < ReproductionHorizonHours {
		next, err := ReproductionHourTime(hour + 1)
		if err != nil {
			return nil, err
		}
		expected[scheduler.Wake{Actor: reproductionClockActor, At: next, Cause: scheduler.WakeAudit}] = true
	}
	return expected, nil
}

// verifyReproductionBoundaryTopology proves a pulse-boundary portable state
// is exactly the runner's own clock protocol: one idle fiber per roster
// entity with the frozen alive invariant, plus the clock fiber, and the
// exact expected wake set with no missing or extra wakes.
func verifyReproductionBoundaryTopology(snap scheduler.Snapshot, births bool, hour int, roster []sim.EntityID, lifecycle map[sim.EntityID]scheduler.Lifecycle, reserved map[sim.EntityID]int64) error {
	if snap.Time != sim.SimTime(hour)*sim.SimTime(ReproductionHour) || len(snap.Fibers) != len(roster)+1 {
		return ErrReproductionCheckpoint
	}
	byActor := make(map[sim.EntityID]scheduler.Fiber, len(snap.Fibers))
	for _, fiber := range snap.Fibers {
		byActor[fiber.Actor] = fiber
	}
	if _, ok := byActor[reproductionClockActor]; !ok {
		return ErrReproductionCheckpoint
	}
	living := make([]sim.EntityID, 0, len(roster))
	for _, id := range roster {
		fiber, ok := byActor[id]
		if !ok || fiber.Activity != nil || fiber.Lifecycle != lifecycle[id] {
			return ErrReproductionCheckpoint
		}
		if lifecycle[id] == scheduler.Alive {
			living = append(living, id)
		}
	}
	expected, err := reproductionExpectedBoundaryWakes(births, hour, living, reserved)
	if err != nil {
		return err
	}
	if len(expected) != len(snap.Wakes) {
		return ErrReproductionCheckpoint
	}
	for _, wake := range snap.Wakes {
		if !expected[wake] {
			return ErrReproductionCheckpoint
		}
		delete(expected, wake)
	}
	if len(expected) != 0 {
		return ErrReproductionCheckpoint
	}
	return nil
}

// verifyReproductionNeutralScheduler proves the portable queue is exactly the
// wake set produced by the runner's own neutral h0 clock protocol: one
// committed pulse batch with no claim, capital, or birth evidence, no open
// activities, every founder fiber alive at registration revision, the clock
// fiber armed for the next pulse, one claim wake per founder, the
// births-enabled request wakes exactly when the flag demands them, and no
// WakeBirth reservation (no newborn exists at h0).
func verifyReproductionNeutralScheduler(k *kernel.Kernel, journal []ReproductionBatch, snap scheduler.Snapshot, births bool) error {
	if k == nil || len(journal) != 1 || len(journal[0].Attempts) != 0 || journal[0].Time != 0 {
		return ErrReproductionCheckpoint
	}
	if snap.Time != 0 || !snap.HasClosed || snap.Closed != 0 || snap.NextToken != 0 {
		return ErrReproductionCheckpoint
	}
	roster := make([]sim.EntityID, 0, ReproductionFounderCount)
	lifecycle := make(map[sim.EntityID]scheduler.Lifecycle, ReproductionFounderCount)
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		roster = append(roster, actor)
		lifecycle[actor] = scheduler.Alive
	}
	if err := verifyReproductionBoundaryTopology(snap, births, 0, roster, lifecycle, nil); err != nil {
		return err
	}
	for _, fiber := range snap.Fibers {
		if fiber.Revision != 1 {
			return ErrReproductionCheckpoint
		}
	}
	return nil
}

// reproductionBoundsTable compiles gate G7's declared field bounds from the
// frozen v4 registry. Integer fields only; ref fields carry no bounds.
type reproductionBoundsTable map[sim.ComponentTypeID]map[sim.FieldID][2]int64

func reproductionBoundsTableForRegistry() (reproductionBoundsTable, error) {
	reg, err := ReproductionRegistry()
	if err != nil {
		return nil, err
	}
	out := make(reproductionBoundsTable)
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

// reproductionProjectionBounds checks gate G7 on one reconstructed hourly
// projection: every integer field of every patch, slot, and actor row
// (founder or newborn, genome rows included) sits inside its declared
// descriptor bounds.
func reproductionProjectionBounds(check ReproductionCheckpoint, table reproductionBoundsTable) error {
	within := func(typ sim.ComponentTypeID, field sim.FieldID, value int64) error {
		bounds, ok := table[typ][field]
		if !ok || value < bounds[0] || value > bounds[1] {
			return fmt.Errorf("%w: component %d field %d value %d outside declared bounds", ErrReproductionCheckpoint, typ, field, value)
		}
		return nil
	}
	for i, p := range check.Patches {
		if _, err := ReproductionPatchID(i); err != nil {
			return err
		}
		for field, value := range map[sim.FieldID]int64{ReproductionPatchYieldField: p.Yield, ReproductionPatchPulsesField: p.Pulses, ReproductionPatchProducedField: p.Produced, ReproductionPatchUnrealizedField: p.Unrealized} {
			if err := within(ReproductionPatchTypeID, field, value); err != nil {
				return err
			}
		}
		for j, s := range check.Slots[i] {
			if _, err := ReproductionSlotID(i, j); err != nil {
				return err
			}
			if err := within(ReproductionSlotTypeID, ReproductionSlotStockField, s.Stock); err != nil {
				return err
			}
			if err := within(ReproductionSlotTypeID, ReproductionSlotGatheredField, s.Gathered); err != nil {
				return err
			}
		}
	}
	actors := make([]ReproductionNewbornState, 0, ReproductionFounderCount+len(check.Newborns))
	for i := range check.Founders {
		actors = append(actors, ReproductionNewbornState{ID: sim.EntityID(i + 1), State: check.Founders[i]})
	}
	actors = append(actors, check.Newborns...)
	for _, entry := range actors {
		a := entry.State
		for _, spec := range []struct {
			typ   sim.ComponentTypeID
			field sim.FieldID
			value int64
		}{
			{ReproductionBagTypeID, ReproductionBagUnitsField, a.Bag.Units},
			{ReproductionBodyTypeID, ReproductionBodyEnergyField, a.Body.Energy},
			{ReproductionBodyTypeID, ReproductionBodyHungerField, a.Body.Hunger},
			{ReproductionBodyTypeID, ReproductionBodyBasalSpentField, a.Body.BasalSpent},
			{ReproductionBodyTypeID, ReproductionBodyCapLostField, a.Body.CapLost},
			{ReproductionBodyTypeID, ReproductionBodyConsumedField, a.Body.Consumed},
			{ReproductionBodyTypeID, ReproductionBodyLastGatherHourField, a.Body.LastGatherHour},
			{ReproductionBodyTypeID, ReproductionBodyBasalDebtField, a.Body.BasalDebt},
			{ReproductionWorksiteTypeID, ReproductionWorksiteCapitalField, a.Worksite.Capital},
			{ReproductionWorksiteTypeID, ReproductionWorksiteWipField, a.Worksite.Wip},
			{ReproductionWorksiteTypeID, ReproductionWorksiteWearDebtField, a.Worksite.WearDebt},
			{ReproductionWorksiteTypeID, ReproductionWorksiteInvestedUnitsField, a.Worksite.InvestedUnits},
			{ReproductionWorksiteTypeID, ReproductionWorksitePointsCreatedField, a.Worksite.PointsCreated},
			{ReproductionWorksiteTypeID, ReproductionWorksitePointsDecayedField, a.Worksite.PointsDecayed},
			{ReproductionWorksiteTypeID, ReproductionWorksiteLastBuildHourField, a.Worksite.LastBuildHour},
			{ReproductionGranaryTypeID, ReproductionGranaryStockField, a.Granary.Stock},
			{ReproductionGranaryTypeID, ReproductionGranaryYieldTotalField, a.Granary.YieldTotal},
			{ReproductionGranaryTypeID, ReproductionGranaryYieldUnrealizedField, a.Granary.YieldUnrealized},
			{ReproductionGranaryTypeID, ReproductionGranaryStoredMealsField, a.Granary.StoredMeals},
			{ReproductionGranaryTypeID, ReproductionGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour},
			{ReproductionGranaryTypeID, ReproductionGranaryYieldDebtField, a.Granary.YieldDebt},
			{ReproductionGenomeTypeID, ReproductionGenomeLocusMField, a.Genome.LocusM},
			{ReproductionGenomeTypeID, ReproductionGenomeLocusYField, a.Genome.LocusY},
			{ReproductionGenomeTypeID, ReproductionGenomeLocusWField, a.Genome.LocusW},
			{ReproductionGenomeTypeID, ReproductionGenomeBirthHourField, a.Genome.BirthHour},
			{ReproductionGenomeTypeID, ReproductionGenomeDeathHourField, a.Genome.DeathHour},
			{ReproductionGenomeTypeID, ReproductionGenomeDiedHourField, a.Genome.DiedHour},
			{ReproductionGenomeTypeID, ReproductionGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestHourField, a.Request.Hour},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestReportedCapitalField, a.Request.ReportedCapital},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestReportedGranaryField, a.Request.ReportedGranary},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, int64(a.Request.Status)},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestLastRequestHourField, a.Request.LastRequestHour},
			{ReproductionBirthRequestTypeID, ReproductionBirthRequestLastReplyHourField, a.Request.LastReplyHour},
		} {
			if err := within(spec.typ, spec.field, spec.value); err != nil {
				return fmt.Errorf("actor %d: %w", entry.ID, err)
			}
		}
	}
	return nil
}

// reproductionProjection is the shadow state of the hourly projection trace,
// rebuilt from verified accepted deltas rather than a second mutable ledger.
type reproductionProjection struct {
	patches  [ReproductionPatchCount]ReproductionPatchState
	slots    [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState
	founders [ReproductionFounderCount]ReproductionActorState
	newborns []ReproductionNewbornState
	deaths   struct {
		age        int64
		starvation int64
	}
	staging map[sim.EntityID]*ReproductionActorState // newborn rows of the entity-create event in flight
}

func reproductionDeltaRef(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing {
		return 0, nil
	}
	return v.EntityRef()
}

// knownEntity reports whether the entity's rows pre-exist in the shadow
// projection: the two patches, their slots, the founders, and every newborn
// born so far.
func (p *reproductionProjection) knownEntity(id sim.EntityID) bool {
	switch {
	case id >= 1001 && id < 1001+ReproductionPatchCount,
		id >= 2001 && id < 2001+ReproductionPatchCount*ReproductionSlotsPerPatch,
		reproductionFounderEntity(id):
		return true
	}
	_, ok := p.newbornIndex(id)
	return ok
}

func (p *reproductionProjection) actor(id sim.EntityID) (*ReproductionActorState, error) {
	if reproductionFounderEntity(id) {
		return &p.founders[id-1], nil
	}
	for i := range p.newborns {
		if p.newborns[i].ID == id {
			return &p.newborns[i].State, nil
		}
	}
	return nil, ErrReproductionCheckpoint
}

// applyPatchDelta mirrors every authorized v3-verbatim plus v4 event delta
// into the shadow projection state. Unlisted component/field/rule
// combinations are rejected instead of ignored, so an unexpected accepted
// delta cannot pass silently; the caller's final authoritative-state
// comparison catches any remaining value divergence. Genome writes are
// restricted to the frozen ownership surface: died-hour only by rules
// 404/413 (attributing the death cause) and birth-granary-paid only by
// rule 410.
func (p *reproductionProjection) applyPatchDelta(rule sim.RuleID, d component.FieldDelta) error {
	integer := func() (int64, error) { return d.After.Integer() }
	switch d.Component {
	case ReproductionPatchTypeID:
		if d.Entity < 1001 || d.Entity >= 1001+ReproductionPatchCount {
			return ErrReproductionCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &p.patches[d.Entity-1001]
		switch d.Field {
		case ReproductionPatchPulsesField:
			v.Pulses = n
		case ReproductionPatchProducedField:
			v.Produced = n
		case ReproductionPatchUnrealizedField:
			v.Unrealized = n
		default:
			return ErrReproductionCheckpoint
		}
	case ReproductionSlotTypeID:
		if d.Entity < 2001 || d.Entity >= 2001+ReproductionPatchCount*ReproductionSlotsPerPatch {
			return ErrReproductionCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &p.slots[(d.Entity-2001)/ReproductionSlotsPerPatch][(d.Entity-2001)%ReproductionSlotsPerPatch]
		switch d.Field {
		case ReproductionSlotStockField:
			v.Stock = n
		case ReproductionSlotGatheredField:
			v.Gathered = n
		default:
			return ErrReproductionCheckpoint
		}
	case ReproductionBagTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionBagUnitsField:
			n, err := integer()
			if err != nil {
				return err
			}
			a.Bag.Units = n
		default:
			// The bag source row is the actor's home patch; only the newborn
			// allocation writes it (rule 410), never a patch.
			return ErrReproductionCheckpoint
		}
	case ReproductionBodyTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionBodyEnergyField:
			a.Body.Energy = n
		case ReproductionBodyHungerField:
			a.Body.Hunger = n
		case ReproductionBodyBasalSpentField:
			a.Body.BasalSpent = n
		case ReproductionBodyCapLostField:
			a.Body.CapLost = n
		case ReproductionBodyConsumedField:
			a.Body.Consumed = n
		case ReproductionBodyLastGatherHourField:
			a.Body.LastGatherHour = n
		case ReproductionBodyBasalDebtField:
			a.Body.BasalDebt = n
		default:
			return ErrReproductionCheckpoint
		}
	case ReproductionWorksiteTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionWorksiteCapitalField:
			a.Worksite.Capital = n
		case ReproductionWorksiteWipField:
			a.Worksite.Wip = n
		case ReproductionWorksiteWearDebtField:
			a.Worksite.WearDebt = n
		case ReproductionWorksiteInvestedUnitsField:
			a.Worksite.InvestedUnits = n
		case ReproductionWorksitePointsCreatedField:
			a.Worksite.PointsCreated = n
		case ReproductionWorksitePointsDecayedField:
			a.Worksite.PointsDecayed = n
		case ReproductionWorksiteLastBuildHourField:
			a.Worksite.LastBuildHour = n
		default:
			return ErrReproductionCheckpoint
		}
	case ReproductionGranaryTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionGranaryStockField:
			a.Granary.Stock = n
		case ReproductionGranaryYieldTotalField:
			a.Granary.YieldTotal = n
		case ReproductionGranaryYieldUnrealizedField:
			a.Granary.YieldUnrealized = n
		case ReproductionGranaryStoredMealsField:
			a.Granary.StoredMeals = n
		case ReproductionGranaryLastStoredMealHourField:
			a.Granary.LastStoredMealHour = n
		case ReproductionGranaryYieldDebtField:
			a.Granary.YieldDebt = n
		default:
			return ErrReproductionCheckpoint
		}
	case ReproductionGenomeTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionGenomeDiedHourField:
			switch rule {
			case ReproductionBasalRule:
				p.deaths.starvation++
			case ReproductionAgeDeathRule:
				p.deaths.age++
			default:
				return ErrReproductionCheckpoint
			}
			a.Genome.DiedHour = n
		case ReproductionGenomeBirthGranaryPaidField:
			if rule != ReproductionBirthReplyRule {
				return ErrReproductionCheckpoint
			}
			a.Genome.BirthGranaryPaid = n
		default:
			// Loci, birth-hour, death-hour, and parent refs are immutable
			// post-creation; only rule 410's allocation writes them (on the
			// newborn), never a patch.
			return ErrReproductionCheckpoint
		}
	case ReproductionBirthRequestTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case ReproductionBirthRequestAddresseeField:
			id, err := reproductionDeltaRef(d.After)
			if err != nil {
				return err
			}
			a.Request.Addressee = id
		default:
			n, err := integer()
			if err != nil {
				return err
			}
			switch d.Field {
			case ReproductionBirthRequestHourField:
				a.Request.Hour = n
			case ReproductionBirthRequestReportedCapitalField:
				a.Request.ReportedCapital = n
			case ReproductionBirthRequestReportedGranaryField:
				a.Request.ReportedGranary = n
			case ReproductionBirthRequestStatusField:
				a.Request.Status = ReproductionRequestStatus(n)
			case ReproductionBirthRequestLastRequestHourField:
				a.Request.LastRequestHour = n
			case ReproductionBirthRequestLastReplyHourField:
				a.Request.LastReplyHour = n
			default:
				return ErrReproductionCheckpoint
			}
		}
	default:
		return ErrReproductionCheckpoint
	}
	return nil
}

// applyAllocationDelta mirrors one entity-create delta: a complete newborn
// row written absent-before. The assembled newborn is validated and appended
// by finishBirth.
func (p *reproductionProjection) applyAllocationDelta(d component.FieldDelta) error {
	if !reproductionActorEntity(d.Entity) || reproductionFounderEntity(d.Entity) {
		return ErrReproductionCheckpoint
	}
	if _, exists := p.newbornIndex(d.Entity); exists {
		return ErrReproductionCheckpoint // an existing row is never allocated twice
	}
	state, ok := p.staging[d.Entity]
	if !ok {
		if len(p.staging) >= 1 {
			return ErrReproductionCheckpoint // one entity-create event allocates exactly one newborn
		}
		if p.staging == nil {
			p.staging = make(map[sim.EntityID]*ReproductionActorState)
		}
		state = &ReproductionActorState{}
		p.staging[d.Entity] = state
	}
	integer := func() (int64, error) { return d.After.Integer() }
	set := func(target *int64) error {
		n, err := integer()
		if err != nil {
			return err
		}
		*target = n
		return nil
	}
	switch d.Component {
	case ReproductionBagTypeID:
		switch d.Field {
		case ReproductionBagUnitsField:
			return set(&state.Bag.Units)
		case ReproductionBagSourceField:
			id, err := reproductionDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Bag.Source = id
			return nil
		}
	case ReproductionBodyTypeID:
		switch d.Field {
		case ReproductionBodyEnergyField:
			return set(&state.Body.Energy)
		case ReproductionBodyHungerField:
			return set(&state.Body.Hunger)
		case ReproductionBodyBasalSpentField:
			return set(&state.Body.BasalSpent)
		case ReproductionBodyCapLostField:
			return set(&state.Body.CapLost)
		case ReproductionBodyConsumedField:
			return set(&state.Body.Consumed)
		case ReproductionBodyLastGatherHourField:
			return set(&state.Body.LastGatherHour)
		case ReproductionBodyBasalDebtField:
			return set(&state.Body.BasalDebt)
		}
	case ReproductionWorksiteTypeID:
		switch d.Field {
		case ReproductionWorksiteCapitalField:
			return set(&state.Worksite.Capital)
		case ReproductionWorksiteWipField:
			return set(&state.Worksite.Wip)
		case ReproductionWorksiteWearDebtField:
			return set(&state.Worksite.WearDebt)
		case ReproductionWorksiteInvestedUnitsField:
			return set(&state.Worksite.InvestedUnits)
		case ReproductionWorksitePointsCreatedField:
			return set(&state.Worksite.PointsCreated)
		case ReproductionWorksitePointsDecayedField:
			return set(&state.Worksite.PointsDecayed)
		case ReproductionWorksiteLastBuildHourField:
			return set(&state.Worksite.LastBuildHour)
		}
	case ReproductionGranaryTypeID:
		switch d.Field {
		case ReproductionGranaryStockField:
			return set(&state.Granary.Stock)
		case ReproductionGranaryYieldTotalField:
			return set(&state.Granary.YieldTotal)
		case ReproductionGranaryYieldUnrealizedField:
			return set(&state.Granary.YieldUnrealized)
		case ReproductionGranaryStoredMealsField:
			return set(&state.Granary.StoredMeals)
		case ReproductionGranaryLastStoredMealHourField:
			return set(&state.Granary.LastStoredMealHour)
		case ReproductionGranaryYieldDebtField:
			return set(&state.Granary.YieldDebt)
		}
	case ReproductionGenomeTypeID:
		switch d.Field {
		case ReproductionGenomeLocusMField:
			return set(&state.Genome.LocusM)
		case ReproductionGenomeLocusYField:
			return set(&state.Genome.LocusY)
		case ReproductionGenomeLocusWField:
			return set(&state.Genome.LocusW)
		case ReproductionGenomeParentAField:
			id, err := reproductionDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Genome.ParentA = id
			return nil
		case ReproductionGenomeParentBField:
			id, err := reproductionDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Genome.ParentB = id
			return nil
		case ReproductionGenomeBirthHourField:
			return set(&state.Genome.BirthHour)
		case ReproductionGenomeDeathHourField:
			return set(&state.Genome.DeathHour)
		case ReproductionGenomeDiedHourField:
			return set(&state.Genome.DiedHour)
		case ReproductionGenomeBirthGranaryPaidField:
			return set(&state.Genome.BirthGranaryPaid)
		}
	case ReproductionBirthRequestTypeID:
		switch d.Field {
		case ReproductionBirthRequestHourField:
			return set(&state.Request.Hour)
		case ReproductionBirthRequestAddresseeField:
			id, err := reproductionDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Request.Addressee = id
			return nil
		case ReproductionBirthRequestReportedCapitalField:
			return set(&state.Request.ReportedCapital)
		case ReproductionBirthRequestReportedGranaryField:
			return set(&state.Request.ReportedGranary)
		case ReproductionBirthRequestStatusField:
			n, err := integer()
			if err != nil {
				return err
			}
			state.Request.Status = ReproductionRequestStatus(n)
			return nil
		case ReproductionBirthRequestLastRequestHourField:
			return set(&state.Request.LastRequestHour)
		case ReproductionBirthRequestLastReplyHourField:
			return set(&state.Request.LastReplyHour)
		}
	default:
		return ErrReproductionCheckpoint
	}
	return ErrReproductionCheckpoint
}

func (p *reproductionProjection) newbornIndex(id sim.EntityID) (int, bool) {
	for i := range p.newborns {
		if p.newborns[i].ID == id {
			return i, true
		}
	}
	return 0, false
}

// finishBirth validates the staged newborn of one entity-create event against
// the frozen next-ID rule, the frozen birth economics, and the actor schema,
// then appends it in birth order.
func (p *reproductionProjection) finishBirth(hour int) error {
	if len(p.staging) != 1 {
		return ErrReproductionCheckpoint
	}
	var id sim.EntityID
	var state *ReproductionActorState
	for staged, s := range p.staging {
		id, state = staged, s
	}
	next, err := ReproductionNewbornID(int64(len(p.newborns)))
	if err != nil {
		return err
	}
	if id != next || state == nil {
		return ErrReproductionCheckpoint
	}
	if state.Genome.ParentA == 0 || state.Genome.ParentB == 0 || state.Genome.BirthHour != int64(hour) ||
		state.Genome.DiedHour != ReproductionNeverHour || state.Genome.BirthGranaryPaid != 0 ||
		!reproductionValidPatch(state.Bag.Source) {
		return ErrReproductionCheckpoint
	}
	if !validReproductionActor(id, *state) {
		return ErrReproductionCheckpoint
	}
	p.newborns = append(p.newborns, ReproductionNewbornState{ID: id, State: *state})
	p.staging = nil
	return nil
}

// reproductionReplayMetrics mirrors Reproduction.metrics exactly (frozen v4
// pilot metrics over the living roster). The reconstruction must equal the
// runner's own hourly projection or the bundle is refused, so any drift
// between this mirror and the frozen computation fails the save and restore
// comparisons.
func reproductionReplayMetrics(deathsAge, deathsStarvation, birthRequests, divergences int64,
	refusals map[ReproductionRefusalReason]int64, balance ReproductionBalance,
	founders [ReproductionFounderCount]ReproductionActorState, newborns []ReproductionNewbornState) ReproductionMetrics {
	m := ReproductionMetrics{
		DeathsAge: deathsAge, DeathsStarvation: deathsStarvation,
		Births: balance.Births, BirthRequests: birthRequests,
		Refusals:    make(map[ReproductionRefusalReason]int64, len(refusals)),
		Divergences: divergences,
	}
	for reason, n := range refusals {
		m.Refusals[reason] = n
	}
	type livingEntry struct {
		id    sim.EntityID
		state ReproductionActorState
	}
	var living []livingEntry
	for i := range founders {
		if reproductionAlive(founders[i]) {
			living = append(living, livingEntry{id: sim.EntityID(i + 1), state: founders[i]})
		}
	}
	for _, newborn := range newborns {
		if reproductionAlive(newborn.State) {
			living = append(living, livingEntry{id: newborn.ID, state: newborn.State})
		}
	}
	alleleIndex := func(v int64) int { return int(v - ReproductionGenomeLocusMin) }
	distinct := make(map[[3]int64]struct{}, len(living))
	for _, entry := range living {
		m.LocusFrequencies[0][alleleIndex(entry.state.Genome.LocusM)]++
		m.LocusFrequencies[1][alleleIndex(entry.state.Genome.LocusY)]++
		m.LocusFrequencies[2][alleleIndex(entry.state.Genome.LocusW)]++
		distinct[[3]int64{entry.state.Genome.LocusM, entry.state.Genome.LocusY, entry.state.Genome.LocusW}] = struct{}{}
	}
	m.DistinctGenomes = len(distinct)
	for i := 0; i < len(living); i++ {
		for j := i + 1; j < len(living); j++ {
			m.SumPairwiseL1 += abs64(living[i].state.Genome.LocusM-living[j].state.Genome.LocusM) +
				abs64(living[i].state.Genome.LocusY-living[j].state.Genome.LocusY) +
				abs64(living[i].state.Genome.LocusW-living[j].state.Genome.LocusW)
		}
	}
	m.GenomePairs = int64(len(living) * (len(living) - 1) / 2)
	m.LivingPairs = m.GenomePairs
	if len(living) > 0 {
		lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
			if reproductionFounderEntity(id) && id >= 1 && id <= ReproductionFounderCount {
				g := founders[id-1].Genome
				return g.ParentA, g.ParentB, true
			}
			for _, newborn := range newborns {
				if newborn.ID == id {
					return newborn.State.Genome.ParentA, newborn.State.Genome.ParentB, true
				}
			}
			return 0, 0, false
		})
		for i := 0; i < len(living); i++ {
			for j := i + 1; j < len(living); j++ {
				if _, related := ReproductionKinDistance(lookup, living[i].id, living[j].id); related {
					m.KinPairs++
				}
			}
		}
	}
	return m
}

// reproductionCheckpointsFromHistory derives the hourly projection trace from
// verified accepted deltas rather than a second mutable ledger, then compares
// the reconstruction against the authoritative snapshot. The mirrored h0 seed
// state (founder genome rows from the frozen founder stream) must match the
// bundle's genesis or the final authoritative comparison fails. Gates
// G1/G2a/G2b/G3 are re-checked at every hour and gate G7 bounds every field,
// so a raw-kernel in-range patch bypass cannot survive replay silently.
func reproductionCheckpointsFromHistory(k *kernel.Kernel, journal []ReproductionBatch, yield int64, seed uint64) ([]ReproductionCheckpoint, error) {
	table, err := reproductionBoundsTableForRegistry()
	if err != nil {
		return nil, err
	}
	var p reproductionProjection
	for i := range p.patches {
		p.patches[i].Yield = yield
	}
	// The h0 seed state mirrors NewReproduction exactly: the founder stream's
	// first sixteen draws are the immutable founder lifetimes.
	stream := sim.NewRandomStream(sim.RandomState{Seed: seed, Stream: ReproductionFounderStream})
	for i := range p.founders {
		lifetime := ReproductionLifetimeBase + int64(stream.Uint64()%ReproductionLifetimeSpan)
		a, err := ReproductionFounderState(sim.EntityID(i+1), lifetime)
		if err != nil {
			return nil, err
		}
		p.founders[i] = a
	}
	events := k.Events()
	index := 0
	birthRequests, divergences := int64(0), int64(0)
	refusals := make(map[ReproductionRefusalReason]int64)
	out := make([]ReproductionCheckpoint, 0, ReproductionHorizonHours+1)
	for _, batch := range journal {
		hour := int(batch.Time / sim.SimTime(ReproductionHour))
		for index < len(events) && events[index].Time == batch.Time {
			event := events[index]
			wrap := func(err error) error {
				return fmt.Errorf("reproduction event %d (t=%d, rule=%d): %w", index, event.Time, event.Rule, err)
			}
			switch event.Kind {
			case kernel.KindEntityCreate:
				// An entity-create carries both the parent cost patches
				// (existing rows) and the newborn's complete allocation rows
				// (absent-before); each delta routes by whether its row
				// pre-exists, exactly as the kernel partitions the proposal.
				for _, delta := range event.Deltas {
					if p.knownEntity(delta.Entity) {
						if err := p.applyPatchDelta(event.Rule, delta); err != nil {
							return nil, wrap(err)
						}
					} else {
						if err := p.applyAllocationDelta(delta); err != nil {
							return nil, wrap(err)
						}
					}
				}
				if err := p.finishBirth(hour); err != nil {
					return nil, wrap(err)
				}
			case kernel.KindComponentPatch:
				for _, delta := range event.Deltas {
					if err := p.applyPatchDelta(event.Rule, delta); err != nil {
						return nil, wrap(err)
					}
				}
			default:
				return nil, wrap(ErrReproductionCheckpoint)
			}
			index++
		}
		if batch.Version != sim.WorldVersion(index) {
			return nil, fmt.Errorf("reproduction batch t=%d version %d != %d: %w", batch.Time, batch.Version, index, ErrReproductionCheckpoint)
		}
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "birth-request":
				birthRequests++
			case "birth-reply":
				if a.Birth.Decision != ReproductionReplyAccept {
					refusals[a.Birth.Reason]++
				}
				if a.Birth.Diverged {
					divergences++
				}
			}
		}
		if batch.Time%sim.SimTime(ReproductionHour) != 0 {
			continue
		}
		if hour != len(out) {
			return nil, fmt.Errorf("reproduction hourly order at t=%d: %w", batch.Time, ErrReproductionCheckpoint)
		}
		balance, err := ReproductionCheckConservation(p.patches, p.slots, p.founders, p.newborns)
		if err != nil {
			return nil, err
		}
		// The newborn slice mirrors the runner's own hourly projection shape:
		// never nil, so the save/restore comparisons are exact.
		newborns := make([]ReproductionNewbornState, 0, len(p.newborns))
		newborns = append(newborns, p.newborns...)
		check := ReproductionCheckpoint{Hour: hour, Version: batch.Version, Balance: balance,
			Patches: p.patches, Slots: p.slots, Founders: p.founders, Newborns: newborns}
		alive := 0
		for i := range p.founders {
			if reproductionAlive(p.founders[i]) {
				alive++
			}
		}
		for _, newborn := range p.newborns {
			if reproductionAlive(newborn.State) {
				alive++
			}
		}
		check.Metrics = reproductionReplayMetrics(p.deaths.age, p.deaths.starvation, birthRequests, divergences, refusals, balance, p.founders, p.newborns)
		check.Metrics.Alive = alive
		if err := reproductionProjectionBounds(check, table); err != nil {
			return nil, fmt.Errorf("reproduction hour %d: %w", hour, err)
		}
		out = append(out, check)
	}
	if index != len(events) {
		return nil, fmt.Errorf("reproduction unconsumed events %d..%d: %w", index, len(events), ErrReproductionCheckpoint)
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	patches, slots, actors, err := reproductionWorldState(view)
	if err != nil {
		return nil, err
	}
	if patches != p.patches || slots != p.slots || len(actors) != ReproductionFounderCount+len(p.newborns) {
		return nil, fmt.Errorf("reproduction authoritative prefix mismatch: %w", ErrReproductionCheckpoint)
	}
	for i := range p.founders {
		if actors[sim.EntityID(i+1)] != p.founders[i] {
			return nil, fmt.Errorf("reproduction founder %d diverged: %w", i+1, ErrReproductionCheckpoint)
		}
	}
	for _, newborn := range p.newborns {
		if actors[newborn.ID] != newborn.State {
			return nil, fmt.Errorf("reproduction newborn %d diverged: %w", newborn.ID, ErrReproductionCheckpoint)
		}
	}
	return out, nil
}

// SaveCheckpoint publishes the neutral h0 pilot bundle: all five sections are
// captured under the coordinator lock, only at the committed h0 pulse
// boundary before any claim, capital, or birth event, and never at an
// existing destination. The returned digest identifies the bundle for Branch
// lineage records.
func (f *Reproduction) SaveCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	if f.steps != 1 || len(f.journal) != 1 || len(f.journal[0].Attempts) != 0 || f.journal[0].Time != 0 ||
		f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 ||
		len(f.current) != 0 || len(f.denied) != 0 || len(f.meals) != 0 ||
		len(f.newborns) != 0 || len(f.birthWakes) != 0 ||
		f.deathsAge != 0 || f.deathsStarve != 0 || f.birthFiles != 0 || f.divergences != 0 || len(f.refusals) != 0 {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later: the independent constructor pins the current policy contract.
	fresh, err := NewReproduction(ReproductionOptions{Yield: f.yield, Seed: f.seed, Workers: 1, Births: f.births})
	if err != nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	if f.ref != fresh.ref {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	strategyBytes, err := encodeReproductionStrategy(f)
	if err != nil {
		return [32]byte{}, err
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	snap := f.sched.Snapshot()
	if err := verifyReproductionNeutralScheduler(f.k, f.journal, snap, f.births); err != nil {
		return [32]byte{}, err
	}
	if err := verifyReproductionStrategy(strategyBytes, fresh, f.stateView()); err != nil {
		return [32]byte{}, err
	}
	journalBytes, err := EncodeReproductionJournal(f.journal, f.k, head, snap.Time, f.journalConfig())
	if err != nil {
		return [32]byte{}, err
	}
	checks, err := reproductionCheckpointsFromHistory(f.k, f.journal, f.yield, f.seed)
	if err != nil || !equalReproductionCheckpoints(checks, f.checkpoints) {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	fingerprint, err := strategy.ReproductionPolicyFingerprint(f.bound[1].Policy())
	if err != nil {
		return [32]byte{}, err
	}
	m := reproductionManifest{yield: f.yield, seed: f.seed, births: f.births, boundary: f.births,
		founders: reproductionNormalizeFounders(f.journalConfig().Founders),
		steps:    f.steps, time: snap.Time, head: head, policyFingerprint: fingerprint}
	for i, part := range [][]byte{journalBytes, history, portable, strategyBytes} {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeReproductionManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: strategyBytes},
	})
}

// stateView is the authoritative read view at the runner's committed head.
func (f *Reproduction) stateView() scheduler.SnapshotView {
	khead := f.k.SnapshotHead()
	return scheduler.SnapshotView{Reader: khead.Reader, Authority: khead.Authority, Version: khead.Version}
}

// SaveResumeCheckpoint publishes a pulse-boundary continuation bundle: the
// same five sections at any committed hourly boundary strictly after the
// neutral root, births included, with the dynamic roster (founders and
// newborns) in the strategy section's binding table. In-flight activity
// between instants is refused: the capture boundary is quiescent by
// construction (the hour's pulse just committed), and publication never
// replaces an existing destination.
func (f *Reproduction) SaveResumeCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	at := f.sched.Time()
	if at <= 0 || at%sim.SimTime(ReproductionHour) != 0 || f.steps < 2 || f.steps != len(f.journal) ||
		len(f.current) != 0 || len(f.meals) != 0 {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	hour := int(at / sim.SimTime(ReproductionHour))
	if len(f.checkpoints) != hour+1 || f.checkpoints[hour].Hour != hour {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	fresh, err := NewReproduction(ReproductionOptions{Yield: f.yield, Seed: f.seed, Workers: 1, Births: f.births})
	if err != nil {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	if f.ref != fresh.ref {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	strategyBytes, err := encodeReproductionStrategy(f)
	if err != nil {
		return [32]byte{}, err
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	snap := f.sched.Snapshot()
	if err := verifyReproductionBoundaryTopology(snap, f.births, hour, f.roster, f.pulseLifecycle, f.birthWakes); err != nil {
		return [32]byte{}, err
	}
	if err := verifyReproductionStrategy(strategyBytes, fresh, f.stateView()); err != nil {
		return [32]byte{}, err
	}
	// The boundary flag is recovered from the journal itself: a continued
	// Branch lineage shares its h0 prefix with the parent's boundary, so the
	// resume manifest records both the continuation and the boundary flags.
	boundary := reproductionBoundaryBirths(f.journal)
	resumeConfig := f.journalConfig()
	resumeConfig.Boundary = boundary
	journalBytes, err := EncodeReproductionJournal(f.journal, f.k, head, at, resumeConfig)
	if err != nil {
		return [32]byte{}, err
	}
	checks, err := reproductionCheckpointsFromHistory(f.k, f.journal, f.yield, f.seed)
	if err != nil || !equalReproductionCheckpoints(checks, f.checkpoints) {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	fingerprint, err := strategy.ReproductionPolicyFingerprint(f.bound[1].Policy())
	if err != nil {
		return [32]byte{}, err
	}
	m := reproductionManifest{yield: f.yield, seed: f.seed, births: f.births, boundary: boundary,
		founders: reproductionNormalizeFounders(f.journalConfig().Founders),
		steps:    f.steps, time: at, head: head, policyFingerprint: fingerprint}
	for i, part := range [][]byte{journalBytes, history, portable, strategyBytes} {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeReproductionManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: strategyBytes},
	})
}

// restoreReproductionSections rebuilds an independent executable runner from
// already hash-checked bundle sections. Every section is validated against
// independently constructed registries and the declared scenario before the
// returned runner becomes visible; the dynamic roster, denial evidence,
// reserved WakeBirth reservations, and cumulative birth economics come from
// the trusted journal replay and are cross-checked against the restored
// portable state. The worker count comes from the caller, never from the
// bundle.
func restoreReproductionSections(sections []checkpoint.Section, m reproductionManifest, expected ReproductionOptions) (*Reproduction, error) {
	fail := func(err error) (*Reproduction, error) {
		return nil, fmt.Errorf("%w: %v", ErrReproductionCheckpoint, err)
	}
	fresh, err := NewReproduction(expected)
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil {
		return fail(err)
	}
	if genesis.GenesisHash != m.head.GenesisHash || genesis.RegistryFingerprint != m.head.RegistryFingerprint {
		return nil, ErrReproductionCheckpoint
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil {
		return fail(err)
	}
	if head != m.head {
		return nil, ErrReproductionCheckpoint
	}
	fresh.k = k
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil {
		return fail(err)
	}
	if sched.Time() != m.time {
		return nil, ErrReproductionCheckpoint
	}
	fresh.sched = sched
	// Run identity decodes under the recorded continuation flag; the
	// scheduler-aware reconciliation replays under the boundary flag, because
	// a Branch child's portable bytes carry the parent's boundary topology
	// (the v4 h0 pulse is births-asymmetric by frozen design).
	identityConfig := fresh.journalConfig()
	identityConfig.Boundary = m.boundary
	journal, err := DecodeReproductionJournal(sections[0].Data, k, head, sched.Time(), identityConfig)
	if err != nil {
		return fail(err)
	}
	if len(journal) != m.steps {
		return nil, ErrReproductionCheckpoint
	}
	// The identity config already carries both flags: Boundary governs the
	// shared h0 batch, Births every later batch.
	reference, err := replayReproductionJournal(journal, identityConfig)
	if err != nil {
		return fail(err)
	}
	before, head2, err := sched.ExportPortable()
	if err != nil || head2 != head {
		return nil, ErrReproductionCheckpoint
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrReproductionCheckpoint
	}
	snap := sched.Snapshot()
	hour := int(m.time / sim.SimTime(ReproductionHour))
	if m.time == 0 {
		if err := verifyReproductionNeutralScheduler(k, journal, snap, m.boundary); err != nil {
			return fail(err)
		}
	} else {
		// A continuation's captured topology reflects the continuation flag:
		// the pending wakes were scheduled by the latest pulse under it. Only
		// the journal's shared h0 batch replays under the boundary flag.
		if err := verifyReproductionBoundaryTopology(snap, m.births, hour, reference.roster, reference.pulseLifecycle, reference.birthWakes); err != nil {
			return fail(err)
		}
	}
	khead := k.SnapshotHead()
	headView := scheduler.SnapshotView{Reader: khead.Reader, Authority: khead.Authority, Version: khead.Version}
	if err := verifyReproductionStrategy(sections[4].Data, fresh, headView); err != nil {
		return fail(err)
	}
	fingerprint, err := strategy.ReproductionPolicyFingerprint(fresh.bound[1].Policy())
	if err != nil {
		return fail(err)
	}
	if fingerprint != m.policyFingerprint {
		return nil, ErrReproductionCheckpoint
	}
	checks, err := reproductionCheckpointsFromHistory(k, journal, fresh.yield, fresh.seed)
	if err != nil {
		return fail(err)
	}
	if len(checks) != hour+1 || checks[hour].Hour != hour {
		return nil, ErrReproductionCheckpoint
	}
	// Re-bind every roster entity (founders and newborns) to the carried
	// frozen policy ref; the genome, not the ref, carries heredity.
	for _, id := range reference.roster {
		if id <= ReproductionFounderCount {
			continue // bound by the fresh constructor
		}
		bound, err := fresh.policies.Bind(strategy.ReproductionBinding{Actor: id, Ref: fresh.ref})
		if err != nil {
			return fail(err)
		}
		if _, exists := fresh.bound[id]; exists {
			return nil, ErrReproductionCheckpoint
		}
		fresh.bound[id] = bound
	}
	fresh.roster = append([]sim.EntityID(nil), reference.roster...)
	fresh.newborns = append([]sim.EntityID(nil), reference.newborns...)
	fresh.pulseLifecycle = make(map[sim.EntityID]scheduler.Lifecycle, len(reference.pulseLifecycle))
	for id, state := range reference.pulseLifecycle {
		fresh.pulseLifecycle[id] = state
	}
	fresh.denied = make(map[sim.EntityID]int64, len(reference.denied))
	for id, denied := range reference.denied {
		fresh.denied[id] = denied
	}
	fresh.birthWakes = make(map[sim.EntityID]int64, len(reference.birthWakes))
	for id, wake := range reference.birthWakes {
		fresh.birthWakes[id] = wake
	}
	fresh.refusals = make(map[ReproductionRefusalReason]int64, len(reference.refusals))
	for reason, n := range reference.refusals {
		fresh.refusals[reason] = n
	}
	fresh.deathsAge, fresh.deathsStarve = reference.deathsAge, reference.deathsStarve
	fresh.birthFiles, fresh.divergences = reference.birthFiles, reference.divergences
	fresh.journal, fresh.checkpoints, fresh.steps = journal, checks, m.steps
	if len(fresh.roster)+1 != len(snap.Fibers) {
		return nil, ErrReproductionCheckpoint
	}
	return fresh, nil
}

func reproductionCheckedSections(path string) ([]checkpoint.Section, [32]byte, reproductionManifest, error) {
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return nil, [32]byte{}, reproductionManifest{}, fmt.Errorf("%w: %v", ErrReproductionCheckpoint, err)
	}
	m, err := decodeReproductionManifest(sections[2].Data)
	if err != nil {
		return nil, [32]byte{}, reproductionManifest{}, err
	}
	for i, part := range [][]byte{sections[0].Data, sections[1].Data, sections[3].Data, sections[4].Data} {
		if sha256.Sum256(part) != m.sections[i] {
			return nil, [32]byte{}, reproductionManifest{}, ErrReproductionCheckpoint
		}
	}
	return sections, digest, m, nil
}

// reproductionManifestMatches reports whether the bundle's recorded scenario
// is exactly the declared one: yield, seed, births flag, and the frozen
// founder genome rows. The founders must equal the seed derivation of the
// declared options, so a genome-variant (or otherwise forged) founder set is
// rejected as a scenario mismatch.
func reproductionManifestMatches(m reproductionManifest, expected ReproductionOptions) bool {
	return m.yield == expected.Yield && m.seed == expected.Seed && m.births == expected.Births &&
		equalReproductionFounderGenomes(m.founders, mustReproductionFounderGenomes(expected.Seed))
}

// RestoreReproductionCheckpoint rebuilds an independent executable runner
// from a verified bundle — the neutral h0 root, a Branch child, or a
// pulse-boundary continuation with newborns present. A different worker count
// is allowed; the declared scenario (yield, seed, births flag, founder
// genomes) must identify the bundle exactly, so an enabled continuation of a
// disabled root always requires an explicit Branch record.
func RestoreReproductionCheckpoint(path string, expected ReproductionOptions) (*Reproduction, [32]byte, error) {
	if expected.Workers < 1 {
		return nil, [32]byte{}, ErrReproductionCheckpoint
	}
	sections, digest, m, err := reproductionCheckedSections(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if !reproductionManifestMatches(m, expected) {
		return nil, [32]byte{}, ErrReproductionCheckpoint
	}
	fresh, err := restoreReproductionSections(sections, m, expected)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return fresh, digest, nil
}

// BranchReproductionCheckpoint verifies a neutral bundle end to end under its
// recorded flag, then publishes a child bundle that continues the same world
// under the explicit births intervention. The child's manifest lineage
// records the parent digest and verified parent head, and its journal section
// is re-pinned to the intervention flag; the policy, history and scheduler
// sections stay byte-identical to the verified parent. Publication is atomic
// and never replaces an existing file; any verification failure publishes
// nothing, including a child whose lineage depth would exceed the
// predeclared restore bound.
//
// The v4 intervention is asymmetric by frozen design: the birth-request phase
// exists only when births are enabled, so an enabled root's h0 boundary
// carries sixteen pending request wakes that a births-disabled continuation
// would refuse to evaluate. The disabling direction is therefore refused
// before publication — it could only produce a child that bricks at the h0
// request instant — while enabling births (and same-flag lineage links)
// branches from the verified neutral root exactly as ticket-12/13 discipline
// demands.
func BranchReproductionCheckpoint(path string, expected ReproductionOptions, births bool, dest string) ([32]byte, error) {
	sections, digest, m, err := reproductionCheckedSections(path)
	if err != nil {
		return [32]byte{}, err
	}
	if expected.Workers < 1 || !reproductionManifestMatches(m, expected) {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	// The only permitted intervention point is the neutral h0 boundary; a
	// non-neutral prefix could not follow the recorded decision and fails
	// closed here.
	if m.time != 0 || m.steps != 1 {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	// Refuse before publication a child whose manifest depth would exceed the
	// restore-time decode bound, and a disabling flip whose pending request
	// wakes no births-disabled continuation may evaluate.
	if m.depth >= reproductionMaxBranchDepth {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	if m.boundary && !births {
		return [32]byte{}, ErrReproductionCheckpoint
	}
	parent, err := restoreReproductionSections(sections, m, ReproductionOptions{Yield: m.yield, Seed: m.seed, Workers: 1, Births: m.births})
	if err != nil {
		return [32]byte{}, err
	}
	config := parent.journalConfig()
	config.Boundary = m.boundary
	config.Births = births
	journalBytes, err := EncodeReproductionJournal(parent.journal, parent.k, m.head, m.time, config)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrReproductionCheckpoint, err)
	}
	child := m
	child.births = births
	child.boundary = m.boundary
	child.parent = digest
	child.parentHead = m.head
	child.depth = m.depth + 1
	for i, part := range [][]byte{journalBytes, sections[1].Data, sections[3].Data, sections[4].Data} {
		child.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(dest, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: sections[1].Data},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeReproductionManifest(child)},
		{Name: checkpoint.Scheduler, Version: 1, Data: sections[3].Data},
		{Name: checkpoint.Strategy, Version: 1, Data: sections[4].Data},
	})
}

// equalReproductionCheckpoints compares two projection traces including the
// per-hour metrics maps.
func equalReproductionCheckpoints(a, b []ReproductionCheckpoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
