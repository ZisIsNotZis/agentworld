package world

import (
	"agentworld/internal/component"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

type diagnosticLink struct {
	Actor sim.EntityID `json:"actor"`
	Event sim.EventID  `json:"event"`
}

type diagnosticCache struct {
	ID    sim.EntityID     `json:"id"`
	Stock int              `json:"stock"`
	Eats  []diagnosticLink `json:"accepted_eats"`
}

type diagnosticActor struct {
	Sample  string           `json:"sample"`
	ID      sim.EntityID     `json:"id"`
	Energy  int              `json:"energy"`
	Hunger  int              `json:"hunger"`
	Stopped bool             `json:"stopped"`
	Attempt *SurvivalAttempt `json:"attempt"`
}

type diagnosticHour struct {
	Hour      int               `json:"hour"`
	Living    int               `json:"living"`
	Stopped   int               `json:"stopped"`
	Energy    int               `json:"energy"`
	Food      int               `json:"food"`
	Hunger    [capacity + 1]int `json:"hunger"`
	Eat       int               `json:"accepted_eat"`
	Rest      int               `json:"accepted_rest"`
	Collision int               `json:"rejected_collision"`
	Caches    []diagnosticCache `json:"caches"`
	Actors    []diagnosticActor `json:"actors"`
}

func diagnosticValue(t *testing.T, s *Survival, actor sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID) int {
	t.Helper()
	head := s.kernel.SnapshotHead()
	v, ok, err := readValue(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, actor, typ, field)
	if err != nil || !ok {
		t.Fatalf("read %d/%d: exists=%v err=%v", actor, typ, ok, err)
	}
	n, err := unit(v)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func diagnosticActorAt(t *testing.T, s *Survival, id sim.EntityID, sample string, byActor map[sim.EntityID]SurvivalAttempt) diagnosticActor {
	t.Helper()
	fiber, ok := s.sched.Fiber(id)
	if !ok {
		t.Fatal("missing actor fiber")
	}
	actor := diagnosticActor{Sample: sample, ID: id, Energy: diagnosticValue(t, s, id, component.EnergyTypeID, component.EnergyReserveField), Hunger: diagnosticValue(t, s, id, HungerTypeID, HungerField), Stopped: fiber.Lifecycle == scheduler.Stopped}
	if a, ok := byActor[id]; ok {
		actor.Attempt = &a
	}
	return actor
}

func diagnosticUsage(t *testing.T) syscall.Rusage {
	t.Helper()
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		t.Fatal(err)
	}
	return u
}

func diagnosticCPU(u syscall.Rusage) time.Duration {
	return time.Duration(u.Utime.Sec+u.Stime.Sec)*time.Second + time.Duration(u.Utime.Usec+u.Stime.Usec)*time.Microsecond
}

func assertDiagnosticWorld(t *testing.T, row diagnosticHour, living, stopped, energy, food, eat, rest, collision int) {
	t.Helper()
	if row.Living != living || row.Stopped != stopped || row.Energy != energy || row.Food != food || row.Eat != eat || row.Rest != rest || row.Collision != collision {
		t.Fatalf("hour %d world trajectory changed: %+v", row.Hour, row)
	}
}

func assertDiagnosticAttempt(t *testing.T, a *SurvivalAttempt, kind strategy.ActionKind, target sim.EntityID, status Status, reason SurvivalReason, event sim.EventID) {
	t.Helper()
	if a == nil || a.Choice.Kind != kind || a.Choice.Target != target || a.Status != status || a.Reason != reason || a.EventID != event {
		t.Fatalf("attempt trajectory changed: %+v", a)
	}
}

func assertDiagnosticActor(t *testing.T, row diagnosticHour, id sim.EntityID, energy int, stopped bool, kind strategy.ActionKind, target sim.EntityID, status Status, reason SurvivalReason, event sim.EventID) {
	t.Helper()
	for _, a := range row.Actors {
		if a.ID == id {
			if a.Energy != energy || a.Hunger != capacity-energy || a.Stopped != stopped {
				t.Fatalf("hour %d actor %d trajectory changed: %+v", row.Hour, id, a)
			}
			assertDiagnosticAttempt(t, a.Attempt, kind, target, status, reason, event)
			return
		}
	}
	t.Fatalf("hour %d actor %d not sampled", row.Hour, id)
}

func assertDiagnosticCache(t *testing.T, row diagnosticHour, id sim.EntityID, stock int, actor sim.EntityID, event sim.EventID) {
	t.Helper()
	for _, c := range row.Caches {
		if c.ID == id {
			if c.Stock != stock || (actor == 0 && len(c.Eats) != 0) || (actor != 0 && (len(c.Eats) != 1 || c.Eats[0] != (diagnosticLink{actor, event}))) {
				t.Fatalf("hour %d cache %d stock/winner changed: %+v", row.Hour, id, c)
			}
			return
		}
	}
	t.Fatalf("hour %d cache %d not sampled", row.Hour, id)
}

func assertDiagnosticMilestone(t *testing.T, row diagnosticHour, byActor map[sim.EntityID]SurvivalAttempt) {
	t.Helper()
	switch row.Hour {
	case 1:
		second := byActor[2]
		assertDiagnosticAttempt(t, &second, strategy.Eat, 1003, Accepted, SurvivalNoReason, 2)
		assertDiagnosticCache(t, row, 1001, 31, 1, 1)
		assertDiagnosticCache(t, row, 1003, 30, 2, 2)
	case 12:
		assertDiagnosticWorld(t, row, 256, 0, 1061, 54, 3, 2, 251)
		assertDiagnosticActor(t, row, 1, 7, false, strategy.Rest, 0, Accepted, SurvivalNoReason, 45)
		assertDiagnosticActor(t, row, 128, 6, false, strategy.Eat, 1004, Rejected, SurvivalCollision, 0)
		assertDiagnosticActor(t, row, 256, 3, false, strategy.Eat, 1004, Rejected, SurvivalCollision, 0)
		assertDiagnosticCache(t, row, 1001, 13, 5, 48)
		assertDiagnosticCache(t, row, 1002, 13, 6, 49)
		assertDiagnosticCache(t, row, 1004, 14, 3, 47)
	case 21:
		assertDiagnosticWorld(t, row, 256, 0, 999, 2, 2, 66, 188)
		assertDiagnosticActor(t, row, 1, 6, false, strategy.Rest, 0, Accepted, SurvivalNoReason, 96)
		assertDiagnosticActor(t, row, 128, 6, false, strategy.Eat, 1004, Rejected, SurvivalCollision, 0)
		assertDiagnosticActor(t, row, 256, 3, false, strategy.Eat, 1004, Rejected, SurvivalCollision, 0)
		assertDiagnosticCache(t, row, 1001, 1, 0, 0)
		assertDiagnosticCache(t, row, 1002, 1, 0, 0)
		assertDiagnosticCache(t, row, 1003, 0, 2, 97)
		assertDiagnosticCache(t, row, 1004, 0, 4, 99)
	case 24:
		assertDiagnosticWorld(t, row, 150, 106, 307, 2, 0, 199, 0)
		assertDiagnosticActor(t, row, 1, 3, false, strategy.Rest, 0, Accepted, SurvivalNoReason, 657)
		assertDiagnosticActor(t, row, 128, 3, false, strategy.Rest, 0, Accepted, SurvivalNoReason, 758)
		assertDiagnosticActor(t, row, 256, 0, true, strategy.Rest, 0, Accepted, SurvivalNoReason, 855)
		assertDiagnosticCache(t, row, 1001, 1, 0, 0)
		assertDiagnosticCache(t, row, 1003, 0, 0, 0)
	case 28:
		assertDiagnosticWorld(t, row, 2, 254, 2, 2, 0, 6, 0)
		assertDiagnosticActor(t, row, 2, 1, false, strategy.Rest, 0, Accepted, SurvivalNoReason, 1155)
		for _, a := range row.Actors {
			if a.Sample == "fixed" && (a.Energy != 0 || !a.Stopped || a.Attempt != nil) {
				t.Fatalf("hour 28 fixed actor changed: %+v", a)
			}
		}
	}
}

// TestSurvivalScaleTrajectory is a frozen S6 diagnostic, not a P0 endurance
// test. The boundary rows and per-step checks distinguish extinction from
// reaching the requested horizon and preserve the first contention evidence.
func TestSurvivalScaleTrajectory(t *testing.T) {
	start, usageBefore := time.Now(), diagnosticUsage(t)
	s, err := NewSurvival(SurvivalOptions{Actors: 256, Seed: 7, Hours: 240, Workers: 4, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	previous, initialStock := s.initial, make(map[sim.EntityID]int)
	for id := sim.EntityID(1001); id <= 1004; id++ {
		initialStock[id] = diagnosticValue(t, s, id, component.CacheStockTypeID, component.CacheStockField)
	}
	milestones := map[int]bool{1: true, 4: true, 12: true, 24: true, 28: true, 29: true}
	firstCollisionHour, firstNoEdibleCacheHour := 0, 0
	var rows []diagnosticHour
	var totalEat, totalRest, totalCollision, attempts int
	for {
		processed, err := s.Step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !processed {
			break
		}
		batch := s.journal[len(s.journal)-1]
		hourNumber := int(batch.Time / sim.SimTime(hour))
		metrics, err := s.metrics(s.kernel)
		if err != nil {
			t.Fatal(err)
		}
		row := diagnosticHour{Hour: hourNumber, Living: metrics.Alive, Energy: metrics.Energy, Food: metrics.Food, Hunger: metrics.Hunger}
		linked := make(map[sim.EventID]bool)
		events := s.kernel.Events()
		byActor := make(map[sim.EntityID]SurvivalAttempt, len(batch.Attempts))
		cacheLinks := make(map[sim.EntityID][]diagnosticLink)
		for _, a := range batch.Attempts {
			attempts++
			byActor[a.Actor] = a
			if a.Status == Accepted {
				if a.EventID == 0 || linked[a.EventID] || int(a.EventID) > len(events) {
					t.Fatalf("hour %d invalid event link: %+v", hourNumber, a)
				}
				linked[a.EventID] = true
				ev := events[a.EventID-1]
				if ev.ID != a.EventID || ev.Cause.Actor != a.Actor || ev.Time != a.Time || ev.Key != a.Key {
					t.Fatalf("hour %d event/attempt mismatch: %+v %+v", hourNumber, a, ev)
				}
				switch a.Choice.Kind {
				case strategy.Eat:
					if ev.Rule != survivalRule || !eventUsesSurvivalTarget(ev, a.Choice.Target) {
						t.Fatal("Eat linked to non-Eat event")
					}
					row.Eat++
					cacheLinks[a.Choice.Target] = append(cacheLinks[a.Choice.Target], diagnosticLink{a.Actor, a.EventID})
				case strategy.Rest:
					if ev.Rule != 1 {
						t.Fatal("Rest linked to non-Rest event")
					}
					row.Rest++
				default:
					t.Fatalf("accepted non-action: %+v", a)
				}
			} else if a.Reason == SurvivalCollision {
				row.Collision++
			}
		}
		if row.Collision > 0 && firstCollisionHour == 0 {
			firstCollisionHour = hourNumber
		}
		if int(batch.Version) != totalEat+totalRest+row.Eat+row.Rest || len(linked) != row.Eat+row.Rest || previous.Energy+previous.Food-metrics.Energy-metrics.Food != row.Eat+row.Rest {
			t.Fatalf("hour %d event/energy accounting: %+v previous=%+v", hourNumber, row, previous)
		}
		edibleCaches, stockSum := 0, 0
		for id := sim.EntityID(1001); id <= 1004; id++ {
			stock := diagnosticValue(t, s, id, component.CacheStockTypeID, component.CacheStockField)
			stockSum += stock
			if stock >= 2 {
				edibleCaches++
			}
			if initialStock[id]-stock != 2*len(cacheLinks[id]) || len(cacheLinks[id]) > 1 {
				t.Fatalf("hour %d cache %d stock/link mismatch: %d -> %d, links=%v", hourNumber, id, initialStock[id], stock, cacheLinks[id])
			}
			row.Caches = append(row.Caches, diagnosticCache{id, stock, cacheLinks[id]})
			initialStock[id] = stock
		}
		if stockSum != metrics.Food {
			t.Fatalf("hour %d cache sum %d != food metric %d", hourNumber, stockSum, metrics.Food)
		}
		if edibleCaches == 0 && firstNoEdibleCacheHour == 0 {
			firstNoEdibleCacheHour = hourNumber
			milestones[hourNumber] = true // preserve the resource-exhaustion boundary
		}
		stopped := 0
		for id := sim.EntityID(1); id <= 256; id++ {
			fiber, _ := s.sched.Fiber(id)
			if fiber.Lifecycle == scheduler.Stopped {
				stopped++
			}
		}
		if metrics.Alive+stopped != 256 {
			t.Fatalf("hour %d living/stopped mismatch: %+v stopped=%d", hourNumber, metrics, stopped)
		}
		row.Stopped = stopped
		if hourNumber == 1 && (row.Eat != 2 || row.Collision != 254 || byActor[1].Status != Accepted || byActor[128].Reason != SurvivalCollision || byActor[256].Reason != SurvivalCollision) {
			t.Fatalf("first contention divergence changed: %+v", row)
		}
		if metrics.Alive == 0 {
			milestones[hourNumber] = true // capture extinction even if the model changes its hour
		}
		if milestones[hourNumber] {
			fixed := []sim.EntityID{1, 128, 256}
			selected := make(map[sim.EntityID]bool)
			for _, id := range fixed {
				selected[id] = true
			}
			// Include currently waking actors when the fixed cohort has died.
			// Attempts are already ordered by the fixed-width actor key.
			for _, id := range fixed {
				row.Actors = append(row.Actors, diagnosticActorAt(t, s, id, "fixed", byActor))
			}
			for _, index := range []int{0, len(batch.Attempts) / 2, len(batch.Attempts) - 1} {
				id := batch.Attempts[index].Actor
				if selected[id] {
					continue
				}
				row.Actors = append(row.Actors, diagnosticActorAt(t, s, id, "active", byActor))
				selected[id] = true
			}
			assertDiagnosticMilestone(t, row, byActor)
			rows = append(rows, row)
			data, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("trajectory %s", data)
		}
		totalEat, totalRest, totalCollision = totalEat+row.Eat, totalRest+row.Rest, totalCollision+row.Collision
		previous = metrics
	}
	report, err := s.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReplayOK || report.SimulatedHours != 29 || report.Final.Alive != 0 || report.Stopped != 256 || report.Final.Food != 2 || firstCollisionHour != 1 || firstNoEdibleCacheHour != 21 || report.AcceptedEat != 61 || report.AcceptedRest != 1101 || report.Rejected[SurvivalCollision] != 5213 || totalEat != report.AcceptedEat || totalRest != report.AcceptedRest || totalCollision != report.Rejected[SurvivalCollision] || report.Events != totalEat+totalRest || attempts <= report.Events || len(rows) != 7 {
		t.Fatalf("scale/replay/trajectory regression: %+v first_collision=%d rows=%d attempts=%d", report, firstCollisionHour, len(rows), attempts)
	}
	t.Logf("boundaries first_collision_hour=%d first_no_edible_cache_hour=%d extinction_hour=%d", firstCollisionHour, firstNoEdibleCacheHour, report.SimulatedHours)
	var eventBytes int
	for _, ev := range s.kernel.Events() {
		b, err := ev.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		eventBytes += len(b)
	}
	journal, err := s.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	history, _, err := s.kernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "scale.checkpoint")
	if _, err := s.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	file, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if eventBytes == 0 || len(journal) == 0 || len(history) == 0 || file.Size() == 0 {
		t.Fatal("empty event, journal, history, or checkpoint artifact")
	}
	var heap runtime.MemStats
	runtime.ReadMemStats(&heap)
	usageAfter := diagnosticUsage(t)
	cost := struct {
		WallNS          int64  `json:"wall_ns"`
		CPUNS           int64  `json:"cpu_ns"`
		PeakRSSKB       int64  `json:"peak_rss_kb"`
		HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
		HeapSysBytes    uint64 `json:"heap_sys_bytes"`
		EventBytes      int    `json:"canonical_event_bytes"`
		JournalBytes    int    `json:"journal_bytes"`
		HistoryBytes    int    `json:"history_bytes"`
		CheckpointBytes int64  `json:"checkpoint_bytes"`
	}{time.Since(start).Nanoseconds(), (diagnosticCPU(usageAfter) - diagnosticCPU(usageBefore)).Nanoseconds(), usageAfter.Maxrss, heap.HeapAlloc, heap.HeapSys, eventBytes, len(journal), len(history), file.Size()}
	data, err := json.Marshal(cost)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cost %s", data)
}
