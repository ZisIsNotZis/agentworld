package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"
	"time"
)

const (
	HungerTypeID      sim.ComponentTypeID = 0x80000003
	HungerField       sim.FieldID         = 1
	HungerProjection  sim.ProjectionID    = 1
	survivalRule      sim.RuleID          = component.WithdrawRuleID
	capacity                              = 8
	hour                                  = sim.Duration(3600 * 1000000)
	maxSurvivalActors                     = 256
	maxSurvivalHours                      = 240
)

var ErrSurvival = errors.New("invalid survival scenario")

// HungerDescriptor belongs to this scenario; rule 1 is rest and rule 2 is
// eating. Both update the same versioned projection from the committed field.
func HungerDescriptor() component.ComponentDescriptor {
	return component.ComponentDescriptor{
		TypeID: HungerTypeID, SchemaVersion: 1, Name: "survival-hunger",
		Fields:          []component.FieldDescriptor{{ID: HungerField, Name: "pressure", Type: component.ValueType{Kind: sim.ScalarKind}, Unit: "joules", Bounds: component.Bounds{HasMinimum: true, Minimum: 0, HasMaximum: true, Maximum: capacity}, PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}},
		AccessPolicy:    component.AccessAuthorizedReadProject,
		TransitionRules: []component.TransitionRuleDescriptor{{ID: 1, Version: 1, Name: "survival-rest-v1"}, {ID: survivalRule, Version: 1, Name: "survival-eat-v1"}},
		Projections:     []component.ProjectionDescriptor{{ID: HungerProjection, Version: 1, Name: "hunger-pressure", SourceFields: []sim.FieldID{HungerField}, Coefficients: []float64{1}, Unit: "joules", Bounds: component.Bounds{HasMinimum: true, Minimum: 0, HasMaximum: true, Maximum: capacity}, MissingBehavior: component.PreserveSourceState}},
		MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1,
	}
}

type SurvivalOptions struct {
	Actors  int
	Seed    uint64
	Hours   int
	Workers int
	// EatWeight changes the actor-specific utility of hunger in the Eat action.
	EatWeight float64
}

type SurvivalReason uint8

const (
	SurvivalNoReason SurvivalReason = iota
	SurvivalWait
	SurvivalStale
	SurvivalInvalidChoice
	SurvivalInvisible
	SurvivalIneligible
	SurvivalInsufficientFood
	SurvivalInvalidNumber
	SurvivalCollision
)

func (r SurvivalReason) MarshalText() ([]byte, error) {
	labels := []string{"none", "wait", "stale-observation", "invalid-choice", "invisible-cache", "ineligible-actor", "insufficient-food", "invalid-number", "cache-collision"}
	if int(r) >= len(labels) {
		return nil, ErrSurvival
	}
	return []byte(labels[r]), nil
}

type SurvivalAttempt struct {
	Actor   sim.EntityID
	Time    sim.SimTime
	Key     string
	Ref     strategy.Ref
	Choice  strategy.Choice
	Status  Status
	Reason  SurvivalReason
	EventID sim.EventID
}

type SurvivalBatch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []SurvivalAttempt
}

type SurvivalMetrics struct {
	Alive  int
	Energy int
	Hunger [capacity + 1]int
	Food   int
}

type SurvivalReport struct {
	Seed                      uint64
	SeedAlgorithm             string
	Policy                    strategy.Ref
	PolicyFormat              uint32
	EatWeight                 float64
	RestRuleVersion           uint32
	EatRuleVersion            uint32
	Actors                    int
	SimulatedHours            int
	Initial, Final            SurvivalMetrics
	AcceptedEat, AcceptedRest int
	Rejected                  map[SurvivalReason]int
	Stopped                   int
	Events                    int
	Dissipated                int
	ReplayOK                  bool
	WallTime                  time.Duration `json:"wall_time_ns"`
	CPUTime                   time.Duration `json:"cpu_time_ns"`
	TokenCost                 int
}

// Survival is a separate trusted adapter; the S5 Runner and its observation
// remain unchanged. The attempt journal is in-memory, not a durable checkpoint.
type Survival struct {
	kernel   *kernel.Kernel
	sched    *scheduler.Scheduler
	registry component.Registry
	seeds    []component.ComponentSeed
	bound    map[sim.EntityID]*strategy.Bound
	public   map[sim.EntityID]map[sim.EntityID]bool
	ref      strategy.Ref
	seed     uint64
	weight   float64
	hours    int
	actors   int
	initial  SurvivalMetrics
	journal  []SurvivalBatch
	pending  *survivalPending
}

type survivalPending struct {
	mu       sync.Mutex
	attempts map[sim.EntityID]SurvivalAttempt
	proposed map[sim.EntityID]bool
}

func scalarUnit(n int) sim.Value { v, _ := sim.ScalarValue(float64(n)); return v }

func survivalRegistry() (component.Registry, error) {
	builder := component.NewRegistryBuilder()
	energy := component.EnergyDescriptor()
	// Rule 1 already exists on energy, rule 2 already exists on cache.
	for _, d := range []component.ComponentDescriptor{energy, component.CacheStockDescriptor(), HungerDescriptor()} {
		if err := builder.Register(d); err != nil {
			return component.Registry{}, err
		}
	}
	return builder.Freeze()
}

const (
	hungerWeight strategy.ParamID = 1
	stockWeight  strategy.ParamID = 2
	restWeight   strategy.ParamID = 3
)

func survivalPolicy() strategy.Policy {
	return strategy.Policy{FormatVersion: strategy.FormatV1, Ref: strategy.Ref{ID: "survival", Version: 1}, Budget: strategy.Budget{Candidates: 5, Evaluations: 5},
		Actions: []strategy.Action{
			{Kind: strategy.Eat, Terms: []strategy.UtilityTerm{{Feature: strategy.OwnHunger, Param: hungerWeight}, {Feature: strategy.TargetFoodStock, Param: stockWeight}}},
			{Kind: strategy.Rest, Terms: []strategy.UtilityTerm{{Feature: strategy.Constant, Param: restWeight}}},
		}, Defaults: map[strategy.ParamID]float64{hungerWeight: 1, stockWeight: .01, restWeight: 1},
	}
}

func NewSurvival(o SurvivalOptions) (*Survival, error) {
	if o.Actors < 1 || o.Actors > maxSurvivalActors || o.Hours < 1 || o.Hours > maxSurvivalHours || o.Workers < 1 || math.IsNaN(o.EatWeight) || math.IsInf(o.EatWeight, 0) || o.EatWeight < 0 || o.EatWeight > 100 {
		return nil, ErrSurvival
	}
	reg, err := survivalRegistry()
	if err != nil {
		return nil, err
	}
	policies := strategy.NewRegistry()
	policy := survivalPolicy()
	if err = policies.Register(policy); err != nil {
		return nil, err
	}
	s := &Survival{registry: reg, ref: policy.Ref, seed: o.Seed, weight: o.EatWeight, hours: o.Hours, actors: o.Actors, bound: make(map[sim.EntityID]*strategy.Bound), public: make(map[sim.EntityID]map[sim.EntityID]bool)}
	random := sim.NewRandomStream(sim.RandomState{Seed: o.Seed, Stream: 1})
	// Cache IDs cannot overlap actors, even at the maximum bounded actor count.
	caches := []sim.EntityID{1001, 1002, 1003, 1004}
	for _, id := range caches {
		stock := 24 + int(random.Uint64()%17)
		s.seeds = append(s.seeds, component.ComponentSeed{Entity: id, Component: component.CacheStockTypeID, Fields: []component.FieldSeed{{Field: component.CacheStockField, Value: scalarUnit(stock)}}})
	}
	for a := 1; a <= o.Actors; a++ {
		actor := sim.EntityID(a)
		energy := 2 + int(random.Uint64()%5)
		s.seeds = append(s.seeds,
			component.ComponentSeed{Entity: actor, Component: component.EnergyTypeID, Fields: []component.FieldSeed{{Field: component.EnergyReserveField, Value: scalarUnit(energy)}}},
			component.ComponentSeed{Entity: actor, Component: HungerTypeID, Fields: []component.FieldSeed{{Field: HungerField, Value: scalarUnit(capacity - energy)}}})
		s.public[actor] = map[sim.EntityID]bool{caches[(a-1)%len(caches)]: true, caches[a%len(caches)]: true}
		b, e := policies.Bind(strategy.Binding{Ref: policy.Ref, Overrides: map[strategy.ParamID]float64{hungerWeight: o.EatWeight}})
		if e != nil {
			return nil, e
		}
		s.bound[actor] = b
	}
	s.kernel, err = kernel.New(reg, 0, s.seeds)
	if err != nil {
		return nil, err
	}
	s.sched, err = scheduler.New(s.kernel, o.Workers, s.evaluate)
	if err != nil {
		return nil, err
	}
	for a := 1; a <= o.Actors; a++ {
		actor := sim.EntityID(a)
		if err = s.sched.Register(actor); err != nil {
			return nil, err
		}
		if err = s.sched.Schedule(scheduler.Wake{Actor: actor, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}); err != nil {
			return nil, err
		}
	}
	s.initial, err = s.metrics(s.kernel)
	return s, err
}

func (s *Survival) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	actor := ready.Fiber.Actor
	energy, ok, err := readValue(view, actor, component.EnergyTypeID, component.EnergyReserveField)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	if !ok {
		return scheduler.Evaluation{}, ErrSurvival
	}
	hunger, ok, err := readValue(view, actor, HungerTypeID, HungerField)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	if !ok {
		return scheduler.Evaluation{}, ErrSurvival
	}
	ids := make([]sim.EntityID, 0, len(s.public[actor]))
	for id := range s.public[actor] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	food := make([]strategy.FoodView, 0, len(ids))
	for _, id := range ids {
		value, e := stockView(view, id)
		if e != nil {
			return scheduler.Evaluation{}, e
		}
		food = append(food, strategy.FoodView{ID: id, Stock: value})
	}
	choice, err := s.bound[actor].Evaluate(strategy.Observation{Actor: actor, Time: ready.At, WorldVersion: view.Version, Energy: energy, Hunger: hunger, PublicFood: food})
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	p, reason, err := s.validateChoice(view, actor, ready.At, choice)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	key := survivalKey(s.ref, ready.At, actor)
	attempt := SurvivalAttempt{Actor: actor, Time: ready.At, Key: key, Ref: s.ref, Choice: choice, Status: Rejected, Reason: reason}
	effects := []scheduler.Effect{}
	n, e := unit(energy)
	if e != nil {
		return scheduler.Evaluation{}, e
	}
	if n == 0 {
		effects = append(effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: actor})
	} else if ready.At < sim.SimTime(int64(s.hours)*int64(hour)) {
		next, e := ready.At.Add(hour)
		if e != nil {
			return scheduler.Evaluation{}, e
		}
		effects = append(effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: actor, Wake: scheduler.Wake{Actor: actor, At: next, Cause: scheduler.WakeNeedThreshold}})
	}
	if reason == SurvivalNoReason {
		p.Key = key
		// A winning Rest at one reaches zero. Stop in the same timestamp,
		// including at the horizon; losing proposals retain their retry wake.
		if choice.Kind == strategy.Rest && n == 1 {
			effects = append(effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: actor, IfKey: key})
		}
		return scheduler.Evaluation{Proposals: []kernel.Proposal{p}, Effects: effects}, s.collect(attempt, true)
	}
	return scheduler.Evaluation{Effects: effects}, s.collect(attempt, false)
}

// collect is supplied by Step; scheduler evaluates actors in parallel.
func (s *Survival) collect(a SurvivalAttempt, proposed bool) error {
	s.pending.mu.Lock()
	defer s.pending.mu.Unlock()
	s.pending.attempts[a.Actor] = a
	s.pending.proposed[a.Actor] = proposed
	return nil
}

func unit(v sim.Value) (int, error) {
	n, e := v.Scalar()
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > float64(1<<53-2) || n != math.Trunc(n) {
		return 0, ErrSurvival
	}
	return int(n), nil
}

func (s *Survival) validateChoice(view scheduler.SnapshotView, actor sim.EntityID, at sim.SimTime, c strategy.Choice) (kernel.Proposal, SurvivalReason, error) {
	if c.Ref != s.ref || c.ObservedVersion != view.Version {
		return kernel.Proposal{}, SurvivalStale, nil
	}
	if c.Kind == strategy.Wait && c.Target == 0 {
		return kernel.Proposal{}, SurvivalWait, nil
	}
	if c.Kind != strategy.Eat && c.Kind != strategy.Rest {
		return kernel.Proposal{}, SurvivalInvalidChoice, nil
	}
	if c.Kind == strategy.Rest && c.Target != 0 {
		return kernel.Proposal{}, SurvivalInvalidChoice, nil
	}
	if c.Kind == strategy.Eat && (sim.ValidateEntityID(c.Target) != nil || !s.public[actor][c.Target]) {
		return kernel.Proposal{}, SurvivalInvisible, nil
	}
	energy, exists, err := readValue(view, actor, component.EnergyTypeID, component.EnergyReserveField)
	if err != nil {
		return kernel.Proposal{}, 0, err
	}
	if !exists {
		return kernel.Proposal{}, SurvivalIneligible, nil
	}
	hunger, exists, err := readValue(view, actor, HungerTypeID, HungerField)
	if err != nil {
		return kernel.Proposal{}, 0, err
	}
	if !exists {
		return kernel.Proposal{}, SurvivalIneligible, nil
	}
	e, errE := unit(energy)
	h, errH := unit(hunger)
	if errE != nil || errH != nil || e > capacity || h > capacity || e+h != capacity {
		return kernel.Proposal{}, SurvivalInvalidNumber, nil
	}
	var patches []component.Patch
	if c.Kind == strategy.Rest {
		if e < 1 {
			return kernel.Proposal{}, SurvivalIneligible, nil
		}
		patches = []component.Patch{{Entity: actor, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: scalarUnit(e - 1)}, {Entity: actor, Component: HungerTypeID, SchemaVersion: 1, Field: HungerField, Value: scalarUnit(h + 1)}}
	} else {
		stock, exists, er := readValue(view, c.Target, component.CacheStockTypeID, component.CacheStockField)
		if er != nil {
			return kernel.Proposal{}, 0, er
		}
		if !exists || !stock.IsPresent() {
			return kernel.Proposal{}, SurvivalInsufficientFood, nil
		}
		quantity, er := unit(stock)
		if er != nil {
			return kernel.Proposal{}, SurvivalInvalidNumber, nil
		}
		if quantity < 2 {
			return kernel.Proposal{}, SurvivalInsufficientFood, nil
		}
		if e < 1 || h < 1 || e >= capacity {
			return kernel.Proposal{}, SurvivalIneligible, nil
		}
		patches = []component.Patch{{Entity: c.Target, Component: component.CacheStockTypeID, SchemaVersion: 1, Field: component.CacheStockField, Value: scalarUnit(quantity - 2)}, {Entity: actor, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: scalarUnit(e + 1)}, {Entity: actor, Component: HungerTypeID, SchemaVersion: 1, Field: HungerField, Value: scalarUnit(h - 1)}}
	}
	rule := sim.RuleID(1)
	if c.Kind == strategy.Eat {
		rule = survivalRule
	}
	return kernel.Proposal{Time: at, Cause: kernel.Cause{Actor: actor}, Rule: rule, RuleVersion: 1, Patches: patches}, SurvivalNoReason, nil
}

func survivalKey(ref strategy.Ref, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("survival/%s/%d/%016x/%016x", ref.ID, ref.Version, uint64(at), uint64(actor))
}

func (s *Survival) Step(ctx context.Context) (bool, error) {
	s.pending = &survivalPending{attempts: make(map[sim.EntityID]SurvivalAttempt), proposed: make(map[sim.EntityID]bool)}
	defer func() { s.pending = nil }()
	events, processed, err := s.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	head := s.kernel.SnapshotHead()
	batch := SurvivalBatch{Time: s.sched.Time(), Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for actor, a := range s.pending.attempts {
		if s.pending.proposed[actor] {
			if id := ids[a.Key]; id != 0 {
				a.Status = Accepted
				a.Reason = SurvivalNoReason
				a.EventID = id
			} else {
				a.Reason = SurvivalCollision
			}
		}
		batch.Attempts = append(batch.Attempts, a)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool { return batch.Attempts[i].Key < batch.Attempts[j].Key })
	s.journal = append(s.journal, batch)
	return true, nil
}

func (s *Survival) Journal() []SurvivalBatch {
	out := make([]SurvivalBatch, len(s.journal))
	for i, b := range s.journal {
		out[i] = b
		out[i].Attempts = append([]SurvivalAttempt(nil), b.Attempts...)
	}
	return out
}

func (s *Survival) metrics(k *kernel.Kernel) (SurvivalMetrics, error) {
	head := k.SnapshotHead()
	result := SurvivalMetrics{}
	energyByActor := make(map[sim.EntityID]int, s.actors)
	hungerByActor := make(map[sim.EntityID]int, s.actors)
	for _, spec := range []struct {
		typ        sim.ComponentTypeID
		field      sim.FieldID
		projection sim.ProjectionID
	}{
		{component.EnergyTypeID, component.EnergyReserveField, component.EnergyProjection}, {HungerTypeID, HungerField, HungerProjection}, {component.CacheStockTypeID, component.CacheStockField, component.CacheProjection},
	} {
		batch, err := head.Reader.Scan(component.ScanRequest{Component: spec.typ, Fields: []sim.FieldID{spec.field}, WorldVersion: head.Version, Authority: head.Authority})
		if err != nil {
			return result, err
		}
		projected, err := head.Reader.Project(component.ProjectRequest{Component: spec.typ, Projection: spec.projection, Entities: batch.EntityIDs(), WorldVersion: head.Version, Authority: head.Authority})
		if err != nil {
			return result, err
		}
		for i := 0; i < batch.Len(); i++ {
			value, er := batch.Value(i, spec.field)
			if er != nil {
				return result, er
			}
			n, er := unit(value)
			if er != nil {
				return result, er
			}
			metric, er := projected.At(i)
			if er != nil {
				return result, er
			}
			mn, er := unit(metric.Value())
			if er != nil || mn != n || metric.ProjectionVersion() != 1 || metric.Provenance().WorldVersion != head.Version {
				return result, ErrSurvival
			}
			switch spec.typ {
			case component.EnergyTypeID:
				energyByActor[batch.EntityIDs()[i]] = n
				result.Energy += n
				if n > 0 {
					result.Alive++
				}
			case HungerTypeID:
				hungerByActor[batch.EntityIDs()[i]] = n
				if n > capacity {
					return result, ErrSurvival
				}
				result.Hunger[n]++
			case component.CacheStockTypeID:
				result.Food += n
			}
		}
	}
	if len(energyByActor) != s.actors || len(hungerByActor) != s.actors {
		return result, ErrSurvival
	}
	for actor, energy := range energyByActor {
		if energy+hungerByActor[actor] != capacity {
			return result, ErrSurvival
		}
	}
	return result, nil
}

// sameSurvivalState checks every scanned field and numerical projection, not
// just aggregates. Kernel.Replay has already checked each canonical event byte.
func sameSurvivalState(a, b *kernel.Kernel) (bool, error) {
	ah, bh := a.SnapshotHead(), b.SnapshotHead()
	if ah.Version != bh.Version || ah.TipID != bh.TipID || ah.TipHash != bh.TipHash {
		return false, nil
	}
	for _, spec := range []struct {
		typ        sim.ComponentTypeID
		field      sim.FieldID
		projection sim.ProjectionID
	}{
		{component.EnergyTypeID, component.EnergyReserveField, component.EnergyProjection},
		{HungerTypeID, HungerField, HungerProjection},
		{component.CacheStockTypeID, component.CacheStockField, component.CacheProjection},
	} {
		x, err := ah.Reader.Scan(component.ScanRequest{Component: spec.typ, Fields: []sim.FieldID{spec.field}, WorldVersion: ah.Version, Authority: ah.Authority})
		if err != nil {
			return false, err
		}
		y, err := bh.Reader.Scan(component.ScanRequest{Component: spec.typ, Fields: []sim.FieldID{spec.field}, WorldVersion: bh.Version, Authority: bh.Authority})
		if err != nil {
			return false, err
		}
		if !reflect.DeepEqual(x.EntityIDs(), y.EntityIDs()) {
			return false, nil
		}
		xp, err := ah.Reader.Project(component.ProjectRequest{Component: spec.typ, Projection: spec.projection, Entities: x.EntityIDs(), WorldVersion: ah.Version, Authority: ah.Authority})
		if err != nil {
			return false, err
		}
		yp, err := bh.Reader.Project(component.ProjectRequest{Component: spec.typ, Projection: spec.projection, Entities: y.EntityIDs(), WorldVersion: bh.Version, Authority: bh.Authority})
		if err != nil {
			return false, err
		}
		for i := 0; i < x.Len(); i++ {
			xv, err := x.Value(i, spec.field)
			if err != nil {
				return false, err
			}
			yv, err := y.Value(i, spec.field)
			if err != nil {
				return false, err
			}
			xm, err := xp.At(i)
			if err != nil {
				return false, err
			}
			ym, err := yp.At(i)
			if err != nil {
				return false, err
			}
			if !reflect.DeepEqual(xv, yv) || !reflect.DeepEqual(xm, ym) {
				return false, nil
			}
		}
	}
	return true, nil
}

// An accepted Eat dissipates one unit; Rest dissipates one unit too.
func checkSurvivalEvent(ev kernel.Event) error {
	if ev.RuleVersion != 1 || ev.Cause.World || len(ev.Metrics) != len(ev.Deltas) {
		return ErrSurvival
	}
	want := map[sim.ComponentTypeID]int{component.EnergyTypeID: -1, HungerTypeID: 1}
	if ev.Rule == survivalRule {
		want = map[sim.ComponentTypeID]int{component.CacheStockTypeID: -2, component.EnergyTypeID: 1, HungerTypeID: -1}
	} else if ev.Rule != 1 {
		return ErrSurvival
	}
	if len(ev.Deltas) != len(want) {
		return ErrSurvival
	}
	for _, d := range ev.Deltas {
		delta, ok := want[d.Component]
		if !ok || d.SchemaVersion != 1 {
			return ErrSurvival
		}
		delete(want, d.Component)
		before, err := unit(d.Before)
		if err != nil {
			return err
		}
		after, err := unit(d.After)
		if err != nil || after-before != delta {
			return ErrSurvival
		}
		if d.Component == component.CacheStockTypeID {
			if d.Entity == ev.Cause.Actor {
				return ErrSurvival
			}
		} else if d.Entity != ev.Cause.Actor {
			return ErrSurvival
		}
	}
	if len(want) != 0 {
		return ErrSurvival
	}
	return nil
}

func (s *Survival) Report(wall, cpu time.Duration) (SurvivalReport, error) {
	final, err := s.metrics(s.kernel)
	if err != nil {
		return SurvivalReport{}, err
	}
	events := s.kernel.Events()
	replayed, err := kernel.Replay(s.registry, 0, s.seeds, events)
	if err != nil {
		return SurvivalReport{}, err
	}
	replayEvents := replayed.Events()
	replayMetrics, err := s.metrics(replayed)
	if err != nil {
		return SurvivalReport{}, err
	}
	same, err := sameSurvivalState(s.kernel, replayed)
	if err != nil {
		return SurvivalReport{}, err
	}
	report := SurvivalReport{Seed: s.seed, SeedAlgorithm: "sim.RandomStream/SplitMix64 (source-versioned)", Policy: s.ref, PolicyFormat: strategy.FormatV1, EatWeight: s.weight, RestRuleVersion: 1, EatRuleVersion: 1, Actors: s.actors, SimulatedHours: int(s.sched.Time() / sim.SimTime(hour)), Initial: s.initial, Final: final, Rejected: make(map[SurvivalReason]int), Events: len(events), WallTime: wall, CPUTime: cpu, ReplayOK: same && reflect.DeepEqual(final, replayMetrics)}
	for i, ev := range events {
		if err := checkSurvivalEvent(ev); err != nil {
			return report, err
		}
		original, er := ev.Bytes()
		if er != nil {
			return report, er
		}
		clone, er := replayEvents[i].Bytes()
		if er != nil {
			return report, er
		}
		if !bytes.Equal(original, clone) {
			report.ReplayOK = false
		}
		if ev.Rule == survivalRule {
			report.AcceptedEat++
		} else if ev.Rule == 1 {
			report.AcceptedRest++
		} else {
			return report, ErrSurvival
		}
	}
	accepted := 0
	for _, batch := range s.journal {
		batchAccepted := 0
		for _, a := range batch.Attempts {
			if a.Ref != s.ref || a.Key != survivalKey(s.ref, a.Time, a.Actor) || a.Choice.Ref != s.ref {
				return report, ErrSurvival
			}
			if a.Status == Rejected {
				if a.EventID != 0 {
					return report, ErrSurvival
				}
				report.Rejected[a.Reason]++
			} else if a.Status == Accepted {
				accepted++
				batchAccepted++
			} else {
				return report, ErrSurvival
			}
		}
		if int(batch.Version) < batchAccepted || batch.TipID != sim.EventID(batch.Version) {
			return report, ErrSurvival
		}
	}
	if accepted != len(events) {
		return report, ErrSurvival
	}
	report.Dissipated = report.AcceptedEat + report.AcceptedRest
	if report.Initial.Energy+report.Initial.Food-report.Final.Energy-report.Final.Food != report.Dissipated || report.Initial.Energy+report.Initial.Food < report.Final.Energy+report.Final.Food {
		return report, ErrSurvival
	}
	for a := 1; a <= s.actors; a++ {
		f, _ := s.sched.Fiber(sim.EntityID(a))
		if f.Lifecycle == scheduler.Stopped {
			report.Stopped++
		}
	}
	return report, nil
}

// RunSurvival is a bounded, reproducible scenario. Timings are measured, never
// part of its deterministic event stream.
func RunSurvival(ctx context.Context, o SurvivalOptions, cpuClock func() (time.Duration, error)) (SurvivalReport, error) {
	if ctx == nil || cpuClock == nil {
		return SurvivalReport{}, ErrSurvival
	}
	startWall := time.Now()
	startCPU, err := cpuClock()
	if err != nil {
		return SurvivalReport{}, err
	}
	s, err := NewSurvival(o)
	if err != nil {
		return SurvivalReport{}, err
	}
	for {
		processed, e := s.Step(ctx)
		if e != nil {
			return SurvivalReport{}, e
		}
		if !processed {
			break
		}
	}
	report, err := s.Report(0, 0)
	if err != nil {
		return report, err
	}
	endCPU, err := cpuClock()
	if err != nil {
		return SurvivalReport{}, err
	}
	report.CPUTime = endCPU - startCPU
	report.WallTime = time.Since(startWall)
	return report, nil
}
