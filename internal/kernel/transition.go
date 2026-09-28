package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"errors"
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

var ErrInvalidProposal = errors.New("invalid transition proposal")
var ErrStalePlan = errors.New("stale or foreign transition plan")
var ErrDuplicateKey = errors.New("duplicate transition key")

type Proposal struct {
	Key         string
	Time        sim.SimTime
	Cause       Cause
	Rule        sim.RuleID
	RuleVersion uint32
	Patches     []component.Patch
}

type Plan struct {
	owner     *Kernel
	snapshot  component.Reader
	authority component.Authority
	proposal  Proposal
}

var nextKernelOrigin atomic.Uint64

type Kernel struct {
	mu             sync.Mutex
	origin         uint64
	registry       component.Registry
	reader         component.Reader
	authority      component.Authority
	version        sim.WorldVersion
	genesisVersion sim.WorldVersion
	genesis        []byte // canonical snapshot owns the seed values, not caller slices
	events         []Event
	acceptedKeys   map[string]struct{} // published winners only, protected by mu
}

func New(registry component.Registry, version sim.WorldVersion, seeds []component.ComponentSeed) (*Kernel, error) {
	reader, authority, err := component.NewReader(registry, version, seeds)
	if err != nil {
		return nil, err
	}
	genesis, err := component.EncodeSnapshot(registry, reader, authority, version)
	if err != nil {
		return nil, err
	}
	return &Kernel{registry: registry, reader: reader, authority: authority, version: version, genesisVersion: version, genesis: genesis, acceptedKeys: make(map[string]struct{}), origin: nextKernelOrigin.Add(1)}, nil
}

// Head binds one immutable reader to its event-log tip under the kernel lock.
type Head struct {
	Reader    component.Reader
	Authority component.Authority
	Version   sim.WorldVersion
	OriginID  uint64 // process-local kernel identity for in-memory scheduler restoration
	TipID     sim.EventID
	TipTime   sim.SimTime
	TipHash   [32]byte
}

// Same compares the snapshot authority (which binds the reader) and event tip.
func (h Head) Same(other Head) bool {
	return h.Authority == other.Authority && h.Version == other.Version && h.OriginID == other.OriginID &&
		h.TipID == other.TipID && h.TipTime == other.TipTime && h.TipHash == other.TipHash
}

func (k *Kernel) head() Head {
	h := Head{Reader: k.reader, Authority: k.authority, Version: k.version, OriginID: k.origin}
	if len(k.events) != 0 {
		e := k.events[len(k.events)-1]
		h.TipID, h.TipTime, h.TipHash = e.ID, e.Time, e.Hash
	}
	return h
}

func (k *Kernel) SnapshotHead() Head {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.head()
}

func (k *Kernel) Snapshot() (component.Reader, component.Authority, sim.WorldVersion) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reader, k.authority, k.version
}
func (k *Kernel) Events() []Event {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]Event, len(k.events))
	for i, e := range k.events {
		out[i] = cloneEvent(e)
	}
	return out
}
func cloneProposal(p Proposal) Proposal {
	p.Patches = append([]component.Patch(nil), p.Patches...)
	return p
}
func (k *Kernel) check(p Proposal, reader component.Reader, auth component.Authority, version sim.WorldVersion) error {
	if p.Key == "" || len(p.Key) > 4096 || p.Time < 0 || !p.Cause.valid() || p.Rule == 0 || p.RuleVersion == 0 || len(p.Patches) == 0 {
		return ErrInvalidProposal
	}
	if !p.Cause.World {
		// A causal actor must exist in the snapshot, even if the patch targets another entity.
		found := false
		for _, typeID := range k.registry.TypeIDs() {
			has, err := reader.Has(component.HasRequest{Entity: p.Cause.Actor, Component: typeID, WorldVersion: version, Authority: auth})
			if err != nil {
				return err
			}
			if has {
				found = true
				break
			}
		}
		if !found {
			return ErrInvalidProposal
		}
	}
	_, _, _, _, err := component.Stage(reader, auth, version, p.Rule, p.RuleVersion, p.Patches)
	return err
}

// Plan validates against the current snapshot and defensively copies the proposal.
func (k *Kernel) Plan(p Proposal, authority component.Authority) (Plan, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.check(p, k.reader, authority, k.version); err != nil {
		return Plan{}, err
	}
	return Plan{owner: k, snapshot: k.reader, authority: authority, proposal: cloneProposal(p)}, nil
}

type fieldKey struct {
	entity    sim.EntityID
	component sim.ComponentTypeID
	field     sim.FieldID
}

// CommitBatch publishes all winners and the hash chain together, or neither.
// A collision discards the entire later proposal; invalid plans never become losers.
func (k *Kernel) CommitBatch(plans []Plan) ([]Event, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.commitBatch(plans)
}

// CommitBatchAtHead checks even empty batches and returns the exact post-commit
// head while holding the kernel lock. An external writer cannot enter between
// committing and capturing this head.
func (k *Kernel) CommitBatchAtHead(expected Head, plans []Plan) ([]Event, Head, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.head().Same(expected) {
		return nil, Head{}, ErrStalePlan
	}
	events, err := k.commitBatch(plans)
	if err != nil {
		return nil, Head{}, err
	}
	return events, k.head(), nil
}

func (k *Kernel) commitBatch(plans []Plan) ([]Event, error) {
	if len(plans) == 0 {
		return []Event{}, nil
	}
	ordered := append([]Plan(nil), plans...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].proposal.Key < ordered[j].proposal.Key })
	for i, plan := range ordered {
		if plan.owner != k || plan.snapshot != k.reader {
			return nil, ErrStalePlan
		}
		if _, exists := k.acceptedKeys[plan.proposal.Key]; exists {
			return nil, ErrDuplicateKey
		}
		if i > 0 && plan.proposal.Key == ordered[i-1].proposal.Key {
			return nil, ErrDuplicateKey
		}
		if plan.proposal.Time != ordered[0].proposal.Time {
			return nil, ErrInvalidProposal
		}
		if len(k.events) > 0 && plan.proposal.Time <= k.events[len(k.events)-1].Time {
			return nil, ErrInvalidProposal
		}
		if err := k.check(plan.proposal, k.reader, plan.authority, k.version); err != nil {
			return nil, err
		}
	}
	seen := make(map[fieldKey]struct{})
	winners := make([]Plan, 0, len(ordered))
	for _, plan := range ordered {
		conflict := false
		for _, p := range plan.proposal.Patches {
			if _, ok := seen[fieldKey{p.Entity, p.Component, p.Field}]; ok {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		winners = append(winners, plan)
		for _, p := range plan.proposal.Patches {
			seen[fieldKey{p.Entity, p.Component, p.Field}] = struct{}{}
		}
	}
	if uint64(len(k.events)) > math.MaxUint64-uint64(len(winners)) || uint64(k.version) > math.MaxUint64-uint64(len(winners)) {
		return nil, sim.ErrOverflow
	}
	reader, authority, version := k.reader, k.authority, k.version
	events := make([]Event, 0, len(winners))
	var previous [32]byte
	if len(k.events) > 0 {
		previous = k.events[len(k.events)-1].Hash
	}
	for _, plan := range winners {
		next, nextAuthority, deltas, metrics, err := component.Stage(reader, authority, version, plan.proposal.Rule, plan.proposal.RuleVersion, plan.proposal.Patches)
		if err != nil {
			return nil, err
		}
		event := Event{ID: sim.EventID(uint64(len(k.events) + len(events) + 1)), Time: plan.proposal.Time, BeforeVersion: version, AfterVersion: version + 1, Kind: "component-patch", Key: plan.proposal.Key, Cause: plan.proposal.Cause, Rule: plan.proposal.Rule, RuleVersion: plan.proposal.RuleVersion, Deltas: deltas, Metrics: metrics, PreviousHash: previous}
		event.Hash, err = hashEvent(event)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
		previous = event.Hash
		reader, authority, version = next, nextAuthority, version+1
	}
	k.reader, k.authority, k.version = reader, authority, version
	k.events = append(k.events, events...)
	for _, event := range events {
		k.acceptedKeys[event.Key] = struct{}{}
	}
	out := make([]Event, len(events))
	for i, e := range events {
		out[i] = cloneEvent(e)
	}
	return out, nil
}
