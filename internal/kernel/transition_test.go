package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"bytes"
	"errors"
	"reflect"
	"testing"
)

type fixture struct {
	descriptor component.ComponentDescriptor
	field      sim.FieldID
	projection sim.ProjectionID
}

func fixtures() []fixture {
	energy := component.EnergyDescriptor()
	dynamic := component.EnergyDescriptor()
	dynamic.StorageClass = component.DynamicStorage
	return []fixture{{energy, component.EnergyReserveField, component.EnergyProjection}, {dynamic, component.EnergyReserveField, component.EnergyProjection}, {component.FatigueDescriptor(), component.FatigueLevelField, component.FatigueProjection}}
}
func scalar(t *testing.T, n float64) sim.Value {
	t.Helper()
	v, e := sim.ScalarValue(n)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func absent(t *testing.T, state sim.ValueState) sim.Value {
	t.Helper()
	v, e := sim.AbsentValue(sim.ScalarKind, state)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func setup(t *testing.T, f fixture) (*Kernel, component.Registry, []component.ComponentSeed) {
	t.Helper()
	b := component.NewRegistryBuilder()
	if err := b.Register(f.descriptor); err != nil {
		t.Fatal(err)
	}
	r, err := b.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	seeds := []component.ComponentSeed{
		{Entity: 1, Component: f.descriptor.TypeID, Fields: []component.FieldSeed{{Field: f.field, Value: scalar(t, 0)}}},
		{Entity: 2, Component: f.descriptor.TypeID},
		{Entity: 3, Component: f.descriptor.TypeID, Fields: []component.FieldSeed{{Field: f.field, Value: absent(t, sim.Unknown)}}},
		{Entity: 4, Component: f.descriptor.TypeID, Fields: []component.FieldSeed{{Field: f.field, Value: absent(t, sim.Inapplicable)}}},
	}
	k, err := New(r, 7, seeds)
	if err != nil {
		t.Fatal(err)
	}
	return k, r, seeds
}
func proposal(f fixture, key string, entity sim.EntityID, value sim.Value) Proposal {
	return Proposal{Key: key, Time: 19, Cause: Cause{Actor: 1}, Rule: 1, RuleVersion: 1, Patches: []component.Patch{{Entity: entity, Component: f.descriptor.TypeID, SchemaVersion: 1, Field: f.field, Value: value}}}
}
func read(t *testing.T, k *Kernel, f fixture, entity sim.EntityID) sim.Value {
	t.Helper()
	r, a, v := k.Snapshot()
	view, e := r.Read(component.ReadRequest{Entity: entity, Component: f.descriptor.TypeID, WorldVersion: v, Authority: a})
	if e != nil {
		t.Fatal(e)
	}
	value, e := view.Value(f.field)
	if e != nil {
		t.Fatal(e)
	}
	return value
}
func eventBytes(t *testing.T, events []Event) [][]byte {
	t.Helper()
	out := make([][]byte, len(events))
	for i, e := range events {
		b, err := e.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		out[i] = b
	}
	return out
}

func TestTransactionalTransitionConformance(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.descriptor.Name+"/"+string(rune('0'+f.descriptor.StorageClass)), func(t *testing.T) {
			k, r, seeds := setup(t, f)
			old, oldAuth, oldVersion := k.Snapshot()
			changes := []struct {
				entity sim.EntityID
				value  sim.Value
			}{{1, scalar(t, .25)}, {2, scalar(t, 0)}, {3, absent(t, sim.Missing)}, {4, absent(t, sim.Unknown)}}
			for i, change := range changes {
				p := proposal(f, string(rune('a'+i)), change.entity, change.value)
				p.Time += sim.SimTime(i)
				plan, err := k.Plan(p, func() component.Authority { _, a, _ := k.Snapshot(); return a }())
				if err != nil {
					t.Fatal(err)
				}
				p.Patches[0].Value = scalar(t, .99) // plan must own its input
				events, err := k.CommitBatch([]Plan{plan})
				if err != nil || len(events) != 1 {
					t.Fatalf("commit: %v, %v", events, err)
				}
				e := events[0]
				if e.ID != sim.EventID(i+1) || e.BeforeVersion != sim.WorldVersion(7+i) || e.AfterVersion != sim.WorldVersion(8+i) || len(e.Deltas) != 1 || len(e.Metrics) != 1 {
					t.Fatalf("event: %+v", e)
				}
				if !e.Deltas[0].After.Equal(change.value) || !read(t, k, f, change.entity).Equal(change.value) {
					t.Fatal("wrong after value")
				}
				m := e.Metrics[0]
				if !m.After.Value().Equal(change.value) || m.Before.Provenance().WorldVersion != e.BeforeVersion || m.After.Provenance().WorldVersion != e.AfterVersion || m.After.ProjectionVersion() != 1 || m.After.Provenance().SchemaVersion != 1 {
					t.Fatalf("metric: %+v", m)
				}
				if i == 0 {
					oldView, err := old.Read(component.ReadRequest{Entity: 1, Component: f.descriptor.TypeID, WorldVersion: oldVersion, Authority: oldAuth})
					if err != nil {
						t.Fatal(err)
					}
					oldValue, _ := oldView.Value(f.field)
					if !oldValue.Equal(scalar(t, 0)) {
						t.Fatal("old reader mutated")
					}
					current, _, v := k.Snapshot()
					if _, err = current.Read(component.ReadRequest{Entity: 1, Component: f.descriptor.TypeID, WorldVersion: v, Authority: oldAuth}); !errors.Is(err, component.ErrUnauthorized) {
						t.Fatalf("old authority worked on new snapshot: %v", err)
					}
				}
			}
			actual := k.Events()
			replayed, err := Replay(r, 7, seeds, actual)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(eventBytes(t, actual), eventBytes(t, replayed.Events())) || !reflect.DeepEqual(actual, replayed.Events()) {
				t.Fatal("replay events differ")
			}
			for _, change := range changes {
				if !read(t, k, f, change.entity).Equal(read(t, replayed, f, change.entity)) {
					t.Fatal("replay state differs")
				}
			}
			a, auth, v := k.Snapshot()
			b, bAuth, bVersion := replayed.Snapshot()
			left, err := a.Project(component.ProjectRequest{Component: f.descriptor.TypeID, Projection: f.projection, WorldVersion: v, Authority: auth})
			if err != nil {
				t.Fatal(err)
			}
			right, err := b.Project(component.ProjectRequest{Component: f.descriptor.TypeID, Projection: f.projection, WorldVersion: bVersion, Authority: bAuth})
			if err != nil || !reflect.DeepEqual(left, right) {
				t.Fatalf("replay metrics: %v", err)
			}
		})
	}
}
func TestEnergyStorageTransitionEventEquivalence(t *testing.T) {
	f := fixtures()
	var events [][]byte
	var hashes [][32]byte
	for _, mode := range f[:2] {
		k, _, _ := setup(t, mode)
		_, a, _ := k.Snapshot()
		p, e := k.Plan(proposal(mode, "same", 3, scalar(t, .3)), a)
		if e != nil {
			t.Fatal(e)
		}
		out, e := k.CommitBatch([]Plan{p})
		if e != nil {
			t.Fatal(e)
		}
		events = append(events, eventBytes(t, out)[0])
		hashes = append(hashes, out[0].Hash)
	}
	if !bytes.Equal(events[0], events[1]) || hashes[0] != hashes[1] {
		t.Fatal("physical storage changed logical event")
	}
}
func TestCommitRejectsInvalidPlansAtomically(t *testing.T) {
	f := fixtures()[0]
	k, _, _ := setup(t, f)
	other, _, _ := setup(t, f)
	_, a, _ := k.Snapshot()
	_, foreign, _ := other.Snapshot()
	good, err := k.Plan(proposal(f, "a", 1, scalar(t, .5)), a)
	if err != nil {
		t.Fatal(err)
	}
	failures := []struct {
		name     string
		proposal Proposal
		auth     component.Authority
		want     error
	}{
		{"zero authority", proposal(f, "b", 2, scalar(t, .1)), component.Authority{}, component.ErrUnauthorized},
		{"foreign authority", proposal(f, "b", 2, scalar(t, .1)), foreign, component.ErrUnauthorized},
		{"unknown rule", func() Proposal { p := proposal(f, "b", 2, scalar(t, .1)); p.Rule = 99; return p }(), a, component.ErrUnknownRule},
		{"wrong schema", func() Proposal { p := proposal(f, "b", 2, scalar(t, .1)); p.Patches[0].SchemaVersion = 2; return p }(), a, component.ErrSchemaVersion},
		{"out of bounds", proposal(f, "b", 2, scalar(t, -1)), a, component.ErrValueOutsideSchema},
		{"missing component", proposal(f, "b", 99, scalar(t, .1)), a, component.ErrComponentMissing},
		{"unknown actor", func() Proposal { p := proposal(f, "b", 2, scalar(t, .1)); p.Cause.Actor = 99; return p }(), a, ErrInvalidProposal},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := k.Plan(tc.proposal, tc.auth); !errors.Is(err, tc.want) {
				t.Fatalf("plan error: %v", err)
			}
			if len(k.Events()) != 0 || !read(t, k, f, 1).Equal(scalar(t, 0)) {
				t.Fatal("state mutated")
			}
		})
	}
	if _, err := k.CommitBatch([]Plan{good, {}}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("mixed valid/invalid batch: %v", err)
	}
	if _, err := k.CommitBatch([]Plan{good, good}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("duplicate key: %v", err)
	}
	if _, err := k.CommitBatch([]Plan{good, func() Plan {
		p := good
		p.proposal = cloneProposal(good.proposal)
		p.proposal.Patches[0].Value = scalar(t, -1)
		p.proposal.Key = "b" // would lose the collision, but invalidity must abort first
		return p
	}()}); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("forged invalid collision loser: %v", err)
	}
	if len(k.Events()) != 0 || !read(t, k, f, 1).Equal(scalar(t, 0)) {
		t.Fatal("rejection advanced world")
	}
	if _, err := k.CommitBatch([]Plan{good}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.CommitBatch([]Plan{good}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("stale plan: %v", err)
	}
	if _, err := other.CommitBatch([]Plan{good}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("cross-kernel plan: %v", err)
	}
	_, freshAuthority, _ := k.Snapshot()
	duplicate, err := k.Plan(proposal(f, "a", 2, scalar(t, .3)), freshAuthority)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.CommitBatch([]Plan{duplicate}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("reused accepted key: %v", err)
	}
}
func TestConflictAndOrderingIndependentOfInputOrder(t *testing.T) {
	f := fixtures()[0]
	var baseline []Event
	for permutation := 0; permutation < 2; permutation++ {
		k, _, _ := setup(t, f)
		_, a, _ := k.Snapshot()
		// b collides with a on entity 1 and must also lose its patch on entity 2.
		alpha := proposal(f, "a", 1, scalar(t, .1))
		beta := proposal(f, "b", 1, scalar(t, .9))
		beta.Patches = append(beta.Patches, component.Patch{Entity: 2, Component: f.descriptor.TypeID, SchemaVersion: 1, Field: f.field, Value: scalar(t, .8)})
		gamma := proposal(f, "c", 3, scalar(t, .3))
		plans := make([]Plan, 0, 3)
		for _, p := range []Proposal{alpha, beta, gamma} {
			plan, err := k.Plan(p, a)
			if err != nil {
				t.Fatal(err)
			}
			plans = append(plans, plan)
		}
		if permutation == 1 {
			plans[0], plans[2] = plans[2], plans[0]
		}
		events, err := k.CommitBatch(plans)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 || events[0].Key != "a" || events[1].Key != "c" || !read(t, k, f, 2).Equal(absent(t, sim.Missing)) {
			t.Fatalf("collision not atomic: %v", events)
		}
		if permutation == 0 {
			baseline = events
		} else if !reflect.DeepEqual(eventBytes(t, events), eventBytes(t, baseline)) || !reflect.DeepEqual(events, baseline) {
			t.Fatal("input order changed events")
		}
		_, registry, seeds := setup(t, f)
		replayed, err := Replay(registry, 7, seeds, events)
		if err != nil || !reflect.DeepEqual(replayed.Events(), events) || !reflect.DeepEqual(eventBytes(t, replayed.Events()), eventBytes(t, events)) {
			t.Fatalf("same-time accepted batch replay: %v", err)
		}
		altered := k.Events()
		altered[0].Key, altered[1].Key = altered[1].Key, altered[0].Key
		altered[0].Hash, _ = hashEvent(altered[0])
		altered[1].PreviousHash = altered[0].Hash
		altered[1].Hash, _ = hashEvent(altered[1])
		if _, err := Replay(registry, 7, seeds, altered); !errors.Is(err, ErrEventIntegrity) {
			t.Fatalf("accepted rehashed same-time key reorder: %v", err)
		}
	}
}
func TestSameTimestampCannotReopenCommittedBatch(t *testing.T) {
	f := fixtures()[0]
	k, registry, seeds := setup(t, f)
	_, auth, _ := k.Snapshot()
	b, err := k.Plan(proposal(f, "b", 1, scalar(t, .8)), auth)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := k.CommitBatch([]Plan{b})
	if err != nil || len(committed) != 1 {
		t.Fatalf("initial batch: %v", err)
	}
	originalBytes := eventBytes(t, k.Events())
	_, auth, version := k.Snapshot()
	for _, time := range []sim.SimTime{19, 18} {
		a := proposal(f, "a", 1, scalar(t, .1))
		a.Time = time
		plan, err := k.Plan(a, auth)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := k.CommitBatch([]Plan{plan}); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("reopened timestamp %d: %v", time, err)
		}
		_, _, afterVersion := k.Snapshot()
		if afterVersion != version || !read(t, k, f, 1).Equal(scalar(t, .8)) || !reflect.DeepEqual(eventBytes(t, k.Events()), originalBytes) || !reflect.DeepEqual(k.Events(), committed) {
			t.Fatal("rejected timestamp changed state or log")
		}
	}
	if _, err := Replay(registry, 7, seeds, committed); err != nil {
		t.Fatal(err)
	}
}

func TestVersionOverflowReservesOnlyWinners(t *testing.T) {
	f := fixtures()[0]
	_, registry, seeds := setup(t, f)
	k, err := New(registry, ^sim.WorldVersion(0)-1, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, auth, _ := k.Snapshot()
	plans := make([]Plan, 0, 3)
	for _, p := range []Proposal{proposal(f, "a", 1, scalar(t, 1)), proposal(f, "b", 1, scalar(t, 2)), proposal(f, "c", 2, scalar(t, 3))} {
		plan, err := k.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	if _, err := k.CommitBatch(plans); !errors.Is(err, sim.ErrOverflow) || len(k.Events()) != 0 {
		t.Fatalf("expected atomic overflow rejection: %v", err)
	}
	events, err := k.CommitBatch(plans[:2])
	if err != nil || len(events) != 1 || events[0].AfterVersion != ^sim.WorldVersion(0) {
		t.Fatalf("winner at version limit: %v, %v", events, err)
	}
}

func TestMultiComponentPatchStagesAtomically(t *testing.T) {
	energy, fatigue := fixtures()[0], fixtures()[2]
	builder := component.NewRegistryBuilder()
	for _, descriptor := range []component.ComponentDescriptor{energy.descriptor, fatigue.descriptor} {
		if err := builder.Register(descriptor); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	seeds := []component.ComponentSeed{{Entity: 1, Component: energy.descriptor.TypeID}, {Entity: 1, Component: fatigue.descriptor.TypeID}}
	k, err := New(registry, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, auth, _ := k.Snapshot()
	p := proposal(energy, "both", 1, scalar(t, 1))
	p.Patches = append(p.Patches, component.Patch{Entity: 1, Component: fatigue.descriptor.TypeID, SchemaVersion: 1, Field: fatigue.field, Value: scalar(t, 2)})
	if _, err := k.Plan(p, auth); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("second field invalid: %v", err)
	}
	if !read(t, k, energy, 1).Equal(absent(t, sim.Missing)) || len(k.Events()) != 0 {
		t.Fatal("failed stage changed authoritative state")
	}
	p.Patches[1].Value = scalar(t, .5)
	plan, err := k.Plan(p, auth)
	if err != nil {
		t.Fatal(err)
	}
	events, err := k.CommitBatch([]Plan{plan})
	if err != nil || len(events) != 1 || len(events[0].Deltas) != 2 || len(events[0].Metrics) != 2 {
		t.Fatalf("multi-component event: %v, %v", events, err)
	}
	if _, err := Replay(registry, 0, seeds, events); err != nil {
		t.Fatal(err)
	}
}

func TestReplayRejectsTampering(t *testing.T) {
	f := fixtures()[0]
	k, registry, seeds := setup(t, f)
	_, a, _ := k.Snapshot()
	first, _ := k.Plan(proposal(f, "a", 1, scalar(t, .4)), a)
	if _, err := k.CommitBatch([]Plan{first}); err != nil {
		t.Fatal(err)
	}
	_, a, _ = k.Snapshot()
	secondProposal := proposal(f, "b", 2, scalar(t, .2))
	secondProposal.Time = 20
	second, _ := k.Plan(secondProposal, a)
	if _, err := k.CommitBatch([]Plan{second}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]Event){
		"value": func(es []Event) { es[0].Deltas[0].After = scalar(t, .7) },
		"before": func(es []Event) {
			es[0].Deltas[0].Before = scalar(t, .7)
			es[0].Hash, _ = hashEvent(es[0])
			es[1].PreviousHash = es[0].Hash
			es[1].Hash, _ = hashEvent(es[1])
		},
		"cause": func(es []Event) {
			es[0].Cause.Actor = 99
			es[0].Hash, _ = hashEvent(es[0])
			es[1].PreviousHash = es[0].Hash
			es[1].Hash, _ = hashEvent(es[1])
		},
		"version": func(es []Event) { es[0].AfterVersion++; es[0].Hash, _ = hashEvent(es[0]) },
		"metric": func(es []Event) {
			es[0].Metrics[0] = es[1].Metrics[0]
			es[0].Hash, _ = hashEvent(es[0])
			es[1].PreviousHash = es[0].Hash
			es[1].Hash, _ = hashEvent(es[1])
		},
		"hash":  func(es []Event) { es[0].Hash[0] ^= 0x80 },
		"order": func(es []Event) { es[0], es[1] = es[1], es[0] },
		"schema": func(es []Event) {
			es[0].Deltas[0].SchemaVersion = 2
			es[0].Hash, _ = hashEvent(es[0])
			es[1].PreviousHash = es[0].Hash
			es[1].Hash, _ = hashEvent(es[1])
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			es := k.Events()
			mutate(es)
			if _, err := Replay(registry, 7, seeds, es); !errors.Is(err, ErrEventIntegrity) {
				t.Fatalf("accepted tamper: %v", err)
			}
		})
	}
	// Returned events may be changed but must not alter the kernel's log.
	changed := k.Events()
	changed[0].Deltas[0].After = scalar(t, .9)
	if !k.Events()[0].Deltas[0].After.Equal(scalar(t, .4)) {
		t.Fatal("events leaked mutable state")
	}
	newProjection := f.descriptor
	newProjection.Projections[0].Version = 2
	builder := component.NewRegistryBuilder()
	if err := builder.Register(newProjection); err != nil {
		t.Fatal(err)
	}
	incompatible, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Replay(incompatible, 7, seeds, k.Events()); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted changed projection version: %v", err)
	}
}
