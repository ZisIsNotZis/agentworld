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

// Social-food v2 pilot checkpoint bundles are a separately versioned format
// with a distinct manifest: they never load or migrate food-flow v1, S6 or any
// other pilot's sections. A bundle exists only at the neutral h0 pulse
// boundary, before any claim or social action. Enabling or disabling social
// behavior for a continued run happens only through an explicit recorded
// Branch whose intervention is carried in the manifest lineage and re-pinned
// in the journal configuration; the pinned policy bytes are never edited.
const socialFoodCheckpointFormat uint32 = 1

// socialFoodMaxBranchDepth bounds lineage chains so a forged manifest cannot
// claim an unbounded ancestry.
const socialFoodMaxBranchDepth = 63

// socialFoodInitialEnergy is the seeded body energy of every v2 actor. The
// contract's validSocialFoodActor and the conservation balance pin the same
// value; checkpoint reconstruction repeats it to detect seed drift.
const socialFoodInitialEnergy int64 = 11

var ErrSocialFoodCheckpoint = errors.New("invalid or incompatible social-food v2 checkpoint")

// Distinct v2 magics: a v1/S6 bundle cannot decode as a v2 manifest, and a v2
// bundle cannot decode as any earlier pilot's manifest.
var socialFoodCheckpointMagic = [4]byte{'A', 'W', 'S', '2'}
var socialFoodStrategyMagic = [4]byte{'A', 'W', 'S', 'F'}

const (
	socialFoodMaxFlowPolicyBytes          = 256
	socialFoodMaxSocialPolicyBytes        = 512
	socialFoodStrategyFormat       uint32 = 1
)

type socialFoodManifest struct {
	yield        int64
	seed         uint64
	enabled      bool
	steps        int
	time         sim.SimTime
	head         kernel.PortableHead
	flowPolicy   [32]byte // exact gather policy content fingerprint
	socialPolicy [32]byte // exact request/donor policy content fingerprint
	parent       [32]byte // parent bundle digest; zero only for a root capture
	parentHead   kernel.PortableHead
	depth        uint32
	sections     [4][32]byte // journal, history, scheduler, strategy
}

func socialFoodBundleFlag(enabled bool) byte {
	if enabled {
		return 1
	}
	return 0
}

// socialFoodFingerprints binds the exact policy content of both pinned
// strategies into the manifest, independently of the strategy section hash.
func socialFoodFingerprints(flow strategy.FoodFlowPolicy, social strategy.SocialFoodPolicy) ([32]byte, [32]byte, error) {
	flowBytes, err := strategy.EncodeFoodFlowPolicy(flow)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	socialBytes, err := strategy.EncodeSocialFoodPolicy(social)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	return sha256.Sum256(flowBytes), sha256.Sum256(socialBytes), nil
}

func encodeSocialFoodManifest(m socialFoodManifest) []byte {
	out := append([]byte(nil), socialFoodCheckpointMagic[:]...)
	for _, v := range []uint32{socialFoodCheckpointFormat, SocialFoodFormatVersion, SocialFoodJournalFormatVersion, uint32(SocialFoodSchemaVersion), SocialFoodRuleVersion, uint32(SocialFoodProjectionVersion), strategy.SocialFoodPolicyFormatV2, strategy.FoodFlowPolicyFormatV1, SocialFoodActorCount, SocialFoodPatchCount, SocialFoodSlotsPerPatch, SocialFoodHorizonHours} {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	for _, v := range []int64{int64(SocialFoodHour), int64(SocialFoodGatherDuration), int64(SocialFoodMealDuration)} {
		out = binary.BigEndian.AppendUint64(out, uint64(v))
	}
	out = binary.BigEndian.AppendUint64(out, uint64(m.yield))
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = append(out, socialFoodBundleFlag(m.enabled))
	out = binary.BigEndian.AppendUint32(out, uint32(m.steps))
	out = binary.BigEndian.AppendUint64(out, uint64(m.time))
	out = appendJournalHead(out, m.head)
	out = append(out, m.flowPolicy[:]...)
	out = append(out, m.socialPolicy[:]...)
	out = append(out, m.parent[:]...)
	out = appendJournalHead(out, m.parentHead)
	out = binary.BigEndian.AppendUint32(out, m.depth)
	for _, hash := range m.sections {
		out = append(out, hash[:]...)
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func decodeSocialFoodManifest(data []byte) (socialFoodManifest, error) {
	var m socialFoodManifest
	// A fixed-width encoding avoids ambiguous defaults and trailing fields.
	if len(data) != len(encodeSocialFoodManifest(m)) {
		return m, ErrSocialFoodCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return m, ErrSocialFoodCheckpoint
	}
	r := journalReader{data: body}
	if !bytes.Equal(r.take(4), socialFoodCheckpointMagic[:]) {
		return m, ErrSocialFoodCheckpoint
	}
	for _, expected := range []uint32{socialFoodCheckpointFormat, SocialFoodFormatVersion, SocialFoodJournalFormatVersion, uint32(SocialFoodSchemaVersion), SocialFoodRuleVersion, uint32(SocialFoodProjectionVersion), strategy.SocialFoodPolicyFormatV2, strategy.FoodFlowPolicyFormatV1, SocialFoodActorCount, SocialFoodPatchCount, SocialFoodSlotsPerPatch, SocialFoodHorizonHours} {
		if r.u32() != expected {
			return m, ErrSocialFoodCheckpoint
		}
	}
	for _, expected := range []int64{int64(SocialFoodHour), int64(SocialFoodGatherDuration), int64(SocialFoodMealDuration)} {
		if int64(r.u64()) != expected {
			return m, ErrSocialFoodCheckpoint
		}
	}
	m.yield, m.seed = int64(r.u64()), r.u64()
	switch flag := r.u8(); flag {
	case 0:
		m.enabled = false
	case 1:
		m.enabled = true
	default:
		return socialFoodManifest{}, ErrSocialFoodCheckpoint
	}
	m.steps, m.time = int(r.u32()), sim.SimTime(r.u64())
	m.head = r.head()
	copy(m.flowPolicy[:], r.take(32))
	copy(m.socialPolicy[:], r.take(32))
	copy(m.parent[:], r.take(32))
	m.parentHead = r.head()
	m.depth = r.u32()
	for i := range m.sections {
		copy(m.sections[i][:], r.take(32))
	}
	// Only the neutral h0 pulse, before any claim or social action, is a valid
	// bundle boundary; social behavior changes exclusively through Branch.
	if r.bad || r.remaining() != 0 || m.yield < 0 || m.yield > 8 || m.steps != 1 || m.time != 0 ||
		m.head.GenesisVersion != 0 || m.head.Version != sim.WorldVersion(m.head.TipID) || m.head.TipTime != m.time {
		return socialFoodManifest{}, ErrSocialFoodCheckpoint
	}
	if m.depth == 0 {
		if m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
			return socialFoodManifest{}, ErrSocialFoodCheckpoint
		}
	} else {
		// A branch child continues its parent at the same verified boundary.
		if m.depth > socialFoodMaxBranchDepth || m.parent == ([32]byte{}) || m.parentHead != m.head {
			return socialFoodManifest{}, ErrSocialFoodCheckpoint
		}
	}
	return m, nil
}

type socialFoodStrategyBinding struct {
	actor                   sim.EntityID
	flowVersion, refVersion uint32
}

// decodeSocialFoodStrategySection splits the pinned strategy section into the
// exact gather policy bytes, the exact social policy bytes, and the per-actor
// binding records. Content is verified against an independently constructed
// executable by verifySocialFoodStrategy.
func decodeSocialFoodStrategySection(data []byte) (flow, social []byte, bindings []socialFoodStrategyBinding, err error) {
	r := journalReader{data: data}
	if !bytes.Equal(r.take(4), socialFoodStrategyMagic[:]) || r.u32() != socialFoodStrategyFormat {
		return nil, nil, nil, ErrSocialFoodCheckpoint
	}
	flowLen := int(r.u16())
	if r.bad || flowLen < 1 || flowLen > socialFoodMaxFlowPolicyBytes {
		return nil, nil, nil, ErrSocialFoodCheckpoint
	}
	flow = r.take(flowLen)
	socialLen := int(r.u16())
	if r.bad || socialLen < 1 || socialLen > socialFoodMaxSocialPolicyBytes {
		return nil, nil, nil, ErrSocialFoodCheckpoint
	}
	social = r.take(socialLen)
	if r.bad || r.u32() != SocialFoodActorCount {
		return nil, nil, nil, ErrSocialFoodCheckpoint
	}
	bindings = make([]socialFoodStrategyBinding, 0, SocialFoodActorCount)
	for i := 0; i < SocialFoodActorCount; i++ {
		b := socialFoodStrategyBinding{actor: sim.EntityID(r.u64()), flowVersion: r.u32(), refVersion: r.u32()}
		if r.bad || b.actor != sim.EntityID(i+1) {
			return nil, nil, nil, ErrSocialFoodCheckpoint
		}
		bindings = append(bindings, b)
	}
	if r.bad || r.remaining() != 0 {
		return nil, nil, nil, ErrSocialFoodCheckpoint
	}
	return flow, social, bindings, nil
}

// verifySocialFoodStrategy compares the section's policy content and bindings
// against a freshly constructed executable runner. A well-formed same-ref
// changed policy fails closed here, not only at content decode time.
func verifySocialFoodStrategy(data []byte, fresh *SocialFood) error {
	flow, social, bindings, err := decodeSocialFoodStrategySection(data)
	if err != nil {
		return err
	}
	if strategy.VerifyFoodFlowPolicy(flow, fresh.flow[0].Policy()) != nil || strategy.VerifySocialFoodPolicy(social, fresh.social[0].Policy()) != nil {
		return ErrSocialFoodCheckpoint
	}
	for i, b := range bindings {
		if b.flowVersion != fresh.flowRef.Version || b.refVersion != fresh.ref.Version ||
			fresh.flow[i].Binding() != (strategy.FoodFlowBinding{Actor: b.actor, Ref: fresh.flowRef}) ||
			fresh.social[i].Binding() != (strategy.SocialFoodBinding{Actor: b.actor, Ref: fresh.ref}) {
			return ErrSocialFoodCheckpoint
		}
	}
	return nil
}

func encodeSocialFoodStrategy(f *SocialFood) ([]byte, error) {
	if f.flow[0] == nil || f.social[0] == nil {
		return nil, ErrSocialFoodCheckpoint
	}
	flowPolicy, err := strategy.EncodeFoodFlowPolicy(f.flow[0].Policy())
	if err != nil {
		return nil, err
	}
	socialPolicy, err := strategy.EncodeSocialFoodPolicy(f.social[0].Policy())
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), socialFoodStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, socialFoodStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(flowPolicy)))
	out = append(out, flowPolicy...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(socialPolicy)))
	out = append(out, socialPolicy...)
	out = binary.BigEndian.AppendUint32(out, SocialFoodActorCount)
	for i := 0; i < SocialFoodActorCount; i++ {
		if f.flow[i] == nil || f.social[i] == nil ||
			f.flow[i].Policy() != f.flow[0].Policy() || f.social[i].Policy() != f.social[0].Policy() ||
			f.flow[i].Binding() != (strategy.FoodFlowBinding{Actor: sim.EntityID(i + 1), Ref: f.flowRef}) ||
			f.social[i].Binding() != (strategy.SocialFoodBinding{Actor: sim.EntityID(i + 1), Ref: f.ref}) {
			return nil, ErrSocialFoodCheckpoint
		}
		out = binary.BigEndian.AppendUint64(out, uint64(i+1))
		out = binary.BigEndian.AppendUint32(out, f.flow[i].Binding().Ref.Version)
		out = binary.BigEndian.AppendUint32(out, f.social[i].Binding().Ref.Version)
	}
	return out, nil
}

// verifySocialFoodNeutralScheduler proves the portable queue is exactly the
// wake set produced by the runner's own neutral h0 clock protocol: one
// committed batch with no claim or social evidence, no open activities, every
// fiber alive at registration revision, and no missing or extra wakes.
func verifySocialFoodNeutralScheduler(k *kernel.Kernel, journal []SocialFoodBatch, snap scheduler.Snapshot) error {
	if k == nil || len(journal) != 1 || len(journal[0].Attempts) != 0 || journal[0].Time != 0 {
		return ErrSocialFoodCheckpoint
	}
	if snap.Time != 0 || !snap.HasClosed || snap.Closed != 0 || snap.NextToken != 0 || len(snap.Fibers) != SocialFoodActorCount+1 {
		return ErrSocialFoodCheckpoint
	}
	var phases [4]sim.SimTime
	for phase := 1; phase <= 3; phase++ {
		at, err := SocialFoodPhaseTime(0, phase)
		if err != nil {
			return err
		}
		phases[phase] = at
	}
	expected := make(map[scheduler.Wake]bool, SocialFoodActorCount*4+1)
	for i, fiber := range snap.Fibers {
		want := sim.EntityID(i + 1)
		if i == SocialFoodActorCount {
			want = socialFoodClockActor
		}
		if fiber.Actor != want || fiber.Lifecycle != scheduler.Alive || fiber.Revision != 1 || fiber.Activity != nil {
			return ErrSocialFoodCheckpoint
		}
		if i < SocialFoodActorCount {
			expected[scheduler.Wake{Actor: want, At: 1, Cause: scheduler.WakeNeedThreshold}] = true
			for phase := 1; phase <= 3; phase++ {
				expected[scheduler.Wake{Actor: want, At: phases[phase], Cause: scheduler.WakeAudit}] = true
			}
		} else {
			expected[scheduler.Wake{Actor: want, At: sim.SimTime(SocialFoodHour), Cause: scheduler.WakeAudit}] = true
		}
	}
	if len(expected) != len(snap.Wakes) {
		return ErrSocialFoodCheckpoint
	}
	for _, wake := range snap.Wakes {
		if !expected[wake] {
			return ErrSocialFoodCheckpoint
		}
		delete(expected, wake)
	}
	if len(expected) != 0 {
		return ErrSocialFoodCheckpoint
	}
	return nil
}

// socialFoodCheckpointsFromHistory derives the hourly projection trace from
// verified accepted deltas rather than a second mutable ledger, then compares
// the reconstruction against the authoritative snapshot. This mirrors the
// runner's in-memory hourly trace before publication and after process
// restore, so no checkpoint ledger is serialized.
func socialFoodCheckpointsFromHistory(k *kernel.Kernel, journal []SocialFoodBatch, yield int64) ([]SocialFoodCheckpoint, error) {
	var patches [SocialFoodPatchCount]SocialFoodPatchState
	var slots [SocialFoodPatchCount][SocialFoodSlotsPerPatch]SocialFoodSlotState
	var actors [SocialFoodActorCount]SocialFoodActorState
	var requests [SocialFoodActorCount]SocialFoodRequestState
	var ledgers [SocialFoodActorCount]SocialFoodLedgerState
	for i := range patches {
		patches[i].Yield = yield
	}
	for i := range actors {
		actors[i] = SocialFoodActorState{Energy: socialFoodInitialEnergy, LastGatherHour: SocialFoodNeverGatheredHour}
		requests[i].Hour = SocialFoodNeverGatheredHour
		ledgers[i].LastReplyHour = SocialFoodNeverGatheredHour
	}
	events := k.Events()
	index := 0
	out := make([]SocialFoodCheckpoint, 0, SocialFoodHorizonHours+1)
	for _, batch := range journal {
		for index < len(events) && events[index].Time == batch.Time {
			for _, delta := range events[index].Deltas {
				if err := applySocialFoodCheckpointDelta(&patches, &slots, &actors, &requests, &ledgers, delta); err != nil {
					return nil, err
				}
			}
			index++
		}
		if batch.Version != sim.WorldVersion(index) {
			return nil, ErrSocialFoodCheckpoint
		}
		if batch.Time%sim.SimTime(SocialFoodHour) != 0 {
			continue
		}
		hour := int(batch.Time / sim.SimTime(SocialFoodHour))
		if hour != len(out) {
			return nil, ErrSocialFoodCheckpoint
		}
		balance, err := SocialFoodCheckConservation(patches, slots, actors)
		if err != nil {
			return nil, err
		}
		if err := SocialFoodCheckWitnesses(requests, ledgers); err != nil {
			return nil, err
		}
		check := SocialFoodCheckpoint{Hour: hour, Version: batch.Version, Balance: balance, Patches: patches, Slots: slots, Actors: actors, Requests: requests, Ledgers: ledgers}
		for _, actor := range actors {
			if actor.Energy > 0 {
				check.Alive++
			}
		}
		out = append(out, check)
	}
	if index != len(events) {
		return nil, ErrSocialFoodCheckpoint
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	p, s, a, r, l, err := socialFoodStates(view)
	if err != nil || p != patches || s != slots || a != actors || r != requests || l != ledgers {
		return nil, ErrSocialFoodCheckpoint
	}
	return out, nil
}

func socialFoodDeltaRef(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing {
		return 0, nil
	}
	return v.EntityRef()
}

// applySocialFoodCheckpointDelta mirrors every authorized v2 event delta into
// the shadow projection state. Unlisted component/field combinations are
// rejected instead of ignored, so an unexpected accepted delta cannot pass
// silently; the caller's final authoritative-state comparison catches any
// remaining value divergence.
func applySocialFoodCheckpointDelta(p *[SocialFoodPatchCount]SocialFoodPatchState, s *[SocialFoodPatchCount][SocialFoodSlotsPerPatch]SocialFoodSlotState, a *[SocialFoodActorCount]SocialFoodActorState, requests *[SocialFoodActorCount]SocialFoodRequestState, ledgers *[SocialFoodActorCount]SocialFoodLedgerState, d component.FieldDelta) error {
	integer := func() (int64, error) { return d.After.Integer() }
	switch d.Component {
	case SocialFoodPatchTypeID:
		if d.Entity < 3001 || d.Entity >= 3001+SocialFoodPatchCount {
			return ErrSocialFoodCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &p[d.Entity-3001]
		switch d.Field {
		case SocialFoodPatchPulsesField:
			v.Pulses = n
		case SocialFoodPatchProducedField:
			v.Produced = n
		case SocialFoodPatchUnrealizedField:
			v.Unrealized = n
		default:
			return ErrSocialFoodCheckpoint
		}
	case SocialFoodSlotTypeID:
		if d.Entity < 4001 || d.Entity >= 4001+SocialFoodPatchCount*SocialFoodSlotsPerPatch {
			return ErrSocialFoodCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &s[(d.Entity-4001)/SocialFoodSlotsPerPatch][(d.Entity-4001)%SocialFoodSlotsPerPatch]
		switch d.Field {
		case SocialFoodSlotStockField:
			v.Stock = n
		case SocialFoodSlotGatheredField:
			v.Gathered = n
		default:
			return ErrSocialFoodCheckpoint
		}
	case SocialFoodBagTypeID:
		if d.Entity < 1 || d.Entity > SocialFoodActorCount {
			return ErrSocialFoodCheckpoint
		}
		v := &a[d.Entity-1].Bag
		switch d.Field {
		case SocialFoodBagUnitsField:
			n, err := integer()
			if err != nil {
				return err
			}
			v.Units = n
		case SocialFoodBagSourceField:
			id, err := socialFoodDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.Source = id
		case SocialFoodBagGathererField:
			id, err := socialFoodDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.Gatherer = id
		case SocialFoodBagLastDonorField:
			id, err := socialFoodDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.LastDonor = id
		default:
			return ErrSocialFoodCheckpoint
		}
	case SocialFoodBodyTypeID:
		if d.Entity < 1 || d.Entity > SocialFoodActorCount {
			return ErrSocialFoodCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &a[d.Entity-1]
		switch d.Field {
		case SocialFoodBodyEnergyField:
			v.Energy = n
		case SocialFoodBodyHungerField:
			v.Hunger = n
		case SocialFoodBodyBasalSpentField:
			v.BasalSpent = n
		case SocialFoodBodyCapLostField:
			v.CapLost = n
		case SocialFoodBodyConsumedField:
			v.Consumed = n
		case SocialFoodBodyLastGatherHourField:
			v.LastGatherHour = n
		default:
			return ErrSocialFoodCheckpoint
		}
	case SocialFoodRequestTypeID:
		if d.Entity < 1 || d.Entity > SocialFoodActorCount {
			return ErrSocialFoodCheckpoint
		}
		v := &requests[d.Entity-1]
		switch d.Field {
		case SocialFoodRequestIDField, SocialFoodRequestHourField, SocialFoodRequestClaimField, SocialFoodRequestsWitnessedField, SocialFoodGiftsWitnessedField, SocialFoodRefusalsWitnessedField:
			n, err := integer()
			if err != nil {
				return err
			}
			switch d.Field {
			case SocialFoodRequestIDField:
				v.ID = n
			case SocialFoodRequestHourField:
				v.Hour = n
			case SocialFoodRequestClaimField:
				v.Claim = n
			case SocialFoodRequestsWitnessedField:
				v.Requests = n
			case SocialFoodGiftsWitnessedField:
				v.Gifts = n
			case SocialFoodRefusalsWitnessedField:
				v.Refusals = n
			}
		case SocialFoodRequestStatusField:
			n, err := integer()
			if err != nil {
				return err
			}
			v.Status = SocialFoodRequestStatus(n)
		case SocialFoodRequestAddresseeField:
			id, err := socialFoodDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.Addressee = id
		default:
			return ErrSocialFoodCheckpoint
		}
	case SocialFoodLedgerTypeID:
		if d.Entity < 1 || d.Entity > SocialFoodActorCount {
			return ErrSocialFoodCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &ledgers[d.Entity-1]
		switch d.Field {
		case SocialFoodLastReplyHourField:
			v.LastReplyHour = n
		case SocialFoodReceivedField:
			v.Received = n
		case SocialFoodGivenField:
			v.Given = n
		case SocialFoodNetField:
			v.Net = n
		default:
			return ErrSocialFoodCheckpoint
		}
	default:
		return ErrSocialFoodCheckpoint
	}
	return nil
}

// SaveCheckpoint publishes the neutral h0 pilot bundle: all five sections are
// captured under the coordinator lock, only at the committed h0 pulse boundary
// before any claim or social action, and never at an existing destination. The
// returned digest identifies the bundle for Branch lineage records.
func (f *SocialFood) SaveCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	if f.steps != 1 || len(f.journal) != 1 || len(f.journal[0].Attempts) != 0 || f.journal[0].Time != 0 ||
		f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 ||
		len(f.current) != 0 || len(f.choices) != 0 || len(f.denied) != 0 {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later: the independent constructor pins the current policy contract.
	fresh, err := NewSocialFood(SocialFoodOptions{Yield: f.yield, Seed: f.seed, Workers: 1, Enabled: f.enabled, TestClaimOverrides: f.claimOverrides})
	if err != nil {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	if f.ref != fresh.ref || f.flowRef != fresh.flowRef {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	strategyBytes, err := encodeSocialFoodStrategy(f)
	if err != nil {
		return [32]byte{}, err
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	snap := f.sched.Snapshot()
	if err := verifySocialFoodNeutralScheduler(f.k, f.journal, snap); err != nil {
		return [32]byte{}, err
	}
	if err := verifySocialFoodStrategy(strategyBytes, fresh); err != nil {
		return [32]byte{}, err
	}
	journalBytes, err := EncodeSocialFoodJournal(f.journal, f.k, head, snap.Time, f.journalConfig())
	if err != nil {
		return [32]byte{}, err
	}
	checks, err := socialFoodCheckpointsFromHistory(f.k, f.journal, f.yield)
	if err != nil || !equalSocialFoodCheckpoints(checks, f.checkpoints) {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	flowFingerprint, socialFingerprint, err := socialFoodFingerprints(f.flow[0].Policy(), f.social[0].Policy())
	if err != nil {
		return [32]byte{}, err
	}
	m := socialFoodManifest{yield: f.yield, seed: f.seed, enabled: f.enabled, steps: f.steps, time: snap.Time, head: head, flowPolicy: flowFingerprint, socialPolicy: socialFingerprint}
	for i, part := range [][]byte{journalBytes, history, portable, strategyBytes} {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeSocialFoodManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: strategyBytes},
	})
}

// restoreSocialFoodSections rebuilds an independent executable runner from
// already hash-checked bundle sections. Every section is validated against
// independently constructed registries and the declared scenario before the
// returned runner becomes visible.
func restoreSocialFoodSections(sections []checkpoint.Section, m socialFoodManifest, expected SocialFoodOptions) (*SocialFood, error) {
	fail := func(err error) (*SocialFood, error) {
		return nil, fmt.Errorf("%w: %v", ErrSocialFoodCheckpoint, err)
	}
	fresh, err := NewSocialFood(expected)
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil {
		return fail(err)
	}
	if genesis.GenesisHash != m.head.GenesisHash || genesis.RegistryFingerprint != m.head.RegistryFingerprint {
		return nil, ErrSocialFoodCheckpoint
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil {
		return fail(err)
	}
	if head != m.head {
		return nil, ErrSocialFoodCheckpoint
	}
	fresh.k = k
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil {
		return fail(err)
	}
	if sched.Time() != m.time {
		return nil, ErrSocialFoodCheckpoint
	}
	fresh.sched = sched
	journal, err := DecodeSocialFoodJournalWithScheduler(sections[0].Data, k, head, sched, fresh.journalConfig())
	if err != nil {
		return fail(err)
	}
	if len(journal) != m.steps {
		return nil, ErrSocialFoodCheckpoint
	}
	snap := sched.Snapshot()
	if err := verifySocialFoodNeutralScheduler(k, journal, snap); err != nil {
		return fail(err)
	}
	code := sections[4].Data
	if err := verifySocialFoodStrategy(code, fresh); err != nil {
		return fail(err)
	}
	flowFingerprint, socialFingerprint, err := socialFoodFingerprints(fresh.flow[0].Policy(), fresh.social[0].Policy())
	if err != nil {
		return fail(err)
	}
	if flowFingerprint != m.flowPolicy || socialFingerprint != m.socialPolicy {
		return nil, ErrSocialFoodCheckpoint
	}
	checks, err := socialFoodCheckpointsFromHistory(k, journal, fresh.yield)
	if err != nil {
		return fail(err)
	}
	if len(checks) != 1 || checks[0].Hour != 0 {
		return nil, ErrSocialFoodCheckpoint
	}
	fresh.journal, fresh.checkpoints, fresh.steps = journal, checks, m.steps
	for i, fiber := range snap.Fibers {
		if i < SocialFoodActorCount {
			fresh.lifecycle[i] = fiber.Lifecycle
		}
	}
	return fresh, nil
}

func socialFoodCheckedSections(path string) ([]checkpoint.Section, [32]byte, socialFoodManifest, error) {
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return nil, [32]byte{}, socialFoodManifest{}, fmt.Errorf("%w: %v", ErrSocialFoodCheckpoint, err)
	}
	m, err := decodeSocialFoodManifest(sections[2].Data)
	if err != nil {
		return nil, [32]byte{}, socialFoodManifest{}, err
	}
	for i, part := range [][]byte{sections[0].Data, sections[1].Data, sections[3].Data, sections[4].Data} {
		if sha256.Sum256(part) != m.sections[i] {
			return nil, [32]byte{}, socialFoodManifest{}, ErrSocialFoodCheckpoint
		}
	}
	return sections, digest, m, nil
}

// RestoreSocialFoodCheckpoint rebuilds an independent executable runner from a
// verified bundle. A different worker count is allowed; the declared scenario,
// intervention flag and policy must identify the bundle exactly, so an
// enabled/disabled continuation always requires an explicit Branch record.
func RestoreSocialFoodCheckpoint(path string, expected SocialFoodOptions) (*SocialFood, [32]byte, error) {
	sections, digest, m, err := socialFoodCheckedSections(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if m.yield != expected.Yield || m.seed != expected.Seed || m.enabled != expected.Enabled {
		return nil, [32]byte{}, ErrSocialFoodCheckpoint
	}
	fresh, err := restoreSocialFoodSections(sections, m, expected)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return fresh, digest, nil
}

// BranchSocialFoodCheckpoint verifies a neutral bundle end to end under its
// recorded flag, then publishes a child bundle that continues the same world
// under the explicit enable/disable intervention. The child's manifest lineage
// records the parent digest and verified parent head, and its journal section
// is re-pinned to the intervention flag; the policy, history and scheduler
// sections stay byte-identical to the verified parent. Publication is atomic
// and never replaces an existing file; any verification failure publishes
// nothing.
func BranchSocialFoodCheckpoint(path string, expected SocialFoodOptions, enabled bool, dest string) ([32]byte, error) {
	sections, digest, m, err := socialFoodCheckedSections(path)
	if err != nil {
		return [32]byte{}, err
	}
	if expected.Workers < 1 || m.yield != expected.Yield || m.seed != expected.Seed || m.enabled != expected.Enabled {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	// Refuse before publication a child whose manifest depth would exceed the
	// restore-time decode bound; such a bundle could never be restored.
	if m.depth >= socialFoodMaxBranchDepth {
		return [32]byte{}, ErrSocialFoodCheckpoint
	}
	parent, err := restoreSocialFoodSections(sections, m, SocialFoodOptions{Yield: m.yield, Seed: m.seed, Workers: 1, Enabled: m.enabled, TestClaimOverrides: expected.TestClaimOverrides})
	if err != nil {
		return [32]byte{}, err
	}
	// The only permitted intervention is the recorded enable/disable decision;
	// a non-neutral prefix could not follow it and fails closed here.
	config := parent.journalConfig()
	config.Enabled = enabled
	journalBytes, err := EncodeSocialFoodJournal(parent.journal, parent.k, m.head, m.time, config)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrSocialFoodCheckpoint, err)
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
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeSocialFoodManifest(child)},
		{Name: checkpoint.Scheduler, Version: 1, Data: sections[3].Data},
		{Name: checkpoint.Strategy, Version: 1, Data: sections[4].Data},
	})
}

func equalSocialFoodCheckpoints(a, b []SocialFoodCheckpoint) bool {
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
