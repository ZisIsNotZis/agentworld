// Command foodflow runs the fixed, synthetic food-flow v1 fixture. Its reports
// are model-conditional diagnostics, not historical or P0 scale validation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

type options struct {
	yield          int64
	seed           uint64
	workers        int
	hours          int
	checkpointHour int
	checkpointPath string
	resume         string
}

type actorTrace struct {
	ID             int           `json:"id"`
	Energy         int64         `json:"energy"`
	Hunger         int64         `json:"hunger"`
	Held           int64         `json:"held"`
	Consumed       int64         `json:"consumed"`
	LastGatherHour int64         `json:"last_gather_hour"`
	PriorHour      []actionTrace `json:"prior_hour_attempts,omitempty"`
}

type actionTrace struct {
	TimeMicroseconds int64       `json:"time_microseconds"`
	Action           string      `json:"action"`
	Outcome          string      `json:"outcome"`
	EventID          sim.EventID `json:"event_id,omitempty"`
}

type patchTrace struct {
	ID         int   `json:"id"`
	Stock      int64 `json:"stock"`
	Produced   int64 `json:"produced"`
	Unrealized int64 `json:"unrealized"`
	Gathered   int64 `json:"gathered"`
	Consumed   int64 `json:"consumed"`
}

type hourTrace struct {
	Hour           int                   `json:"hour"`
	Alive          int                   `json:"alive"`
	AliveIDs       []int                 `json:"alive_ids"`
	Balance        world.FoodFlowBalance `json:"balance"`
	Patches        [2]patchTrace         `json:"patches"`
	Actors         [3]actorTrace         `json:"actors"`
	PriorHour      counts                `json:"prior_hour_accepted"`
	PriorRejection counts                `json:"prior_hour_rejected"`
}

type counts struct {
	Gather         int `json:"gather"`
	Eat            int `json:"eat"`
	Wait           int `json:"wait"`
	NoStock        int `json:"no_stock"`
	Capacity       int `json:"capacity"`
	Ineligible     int `json:"ineligible"`
	NoAccess       int `json:"no_access"`
	DuplicateActor int `json:"duplicate_actor"`
}

type report struct {
	Status                string      `json:"status"`
	Revision              string      `json:"revision"`
	Yield                 int64       `json:"yield_per_patch_per_hour"`
	Seed                  uint64      `json:"seed"`
	Workers               int         `json:"workers"`
	RequestedHours        int         `json:"requested_hours"`
	CompletedHour         int         `json:"completed_hour"`
	Policy                string      `json:"policy"`
	Traces                []hourTrace `json:"traces"`
	ExtinctionHours       []int       `json:"extinction_hours"`
	PopulationChangeHours []int       `json:"population_change_hours"`
	Accepted              counts      `json:"accepted"`
	Rejected              counts      `json:"rejected"`
	FirstEightClaims      [16]int     `json:"first_eight_gather_claims"`
	FirstEightGather      [16]int     `json:"first_eight_gather_successes"`
	AcceptedLinksChecked  int         `json:"accepted_event_links_checked"`
	UnlinkedRejections    int         `json:"unlinked_rejections_checked"`
	Events                int         `json:"events"`
	JournalBatches        int         `json:"journal_batches"`
	EventBodyBytes        int         `json:"event_body_bytes"`
	HistoryBytes          int         `json:"history_bytes"`
	JournalBytes          int         `json:"journal_bytes"`
	CheckpointBytes       int64       `json:"checkpoint_bytes"`
	CheckpointDigest      string      `json:"checkpoint_digest"`
	WallTimeNS            int64       `json:"wall_time_ns"`
	CPUTimeNS             int64       `json:"cpu_time_ns"`
	PeakRSSBytes          int64       `json:"peak_rss_bytes"`
	PeakHeapAllocSampled  uint64      `json:"peak_heap_alloc_sampled_bytes"`
	TokenCost             int         `json:"token_cost"`
}

func cpuTime() (time.Duration, int64, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, err
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second + time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond, usage.Maxrss * 1024, nil // Linux ru_maxrss is KiB.
}

func trace(c world.FoodFlowCheckpoint) hourTrace {
	t := hourTrace{Hour: c.Hour, Alive: c.Alive, Balance: c.Balance, AliveIDs: []int{}}
	for i, a := range c.Actors {
		if a.Energy > 0 {
			t.AliveIDs = append(t.AliveIDs, i+1)
		}
	}
	for i := range t.Patches {
		p := c.Patches[i]
		t.Patches[i] = patchTrace{ID: 1001 + i, Produced: p.Produced, Unrealized: p.Unrealized}
		for _, s := range c.Slots[i] {
			t.Patches[i].Stock += s.Stock
			t.Patches[i].Gathered += s.Gathered
		}
		for _, a := range c.Actors[i*8 : (i+1)*8] {
			t.Patches[i].Consumed += a.Consumed
		}
	}
	for i, id := range []int{1, 8, 16} {
		a := c.Actors[id-1]
		t.Actors[i] = actorTrace{ID: id, Energy: a.Energy, Hunger: a.Hunger, Held: a.Bag, Consumed: a.Consumed, LastGatherHour: a.LastGatherHour}
	}
	return t
}

func actionName(kind strategy.FoodFlowAction) string {
	switch kind {
	case strategy.FoodFlowGather:
		return "Gather"
	case strategy.FoodFlowEat:
		return "Eat"
	case strategy.FoodFlowWait:
		return "Wait"
	default:
		return "unknown"
	}
}

func outcomeName(a world.FoodFlowAttempt) string {
	if a.Accepted {
		return "accepted"
	}
	if a.Choice.Kind == strategy.FoodFlowWait {
		return "fallback"
	}
	switch a.Rejection {
	case world.FoodFlowGatherNoStock:
		return "no_stock"
	case world.FoodFlowGatherCapacity:
		return "capacity"
	case world.FoodFlowGatherIneligible:
		return "ineligible"
	case world.FoodFlowGatherNoAccess:
		return "no_access"
	case world.FoodFlowGatherDuplicateActor:
		return "duplicate_actor"
	default:
		return "unknown"
	}
}

func evidence(r *report, f *world.FoodFlow) error {
	checks := f.Checkpoints()
	if len(checks) == 0 || checks[len(checks)-1].Hour != r.CompletedHour {
		return errors.New("missing hourly projection")
	}
	selected := map[int]bool{0: true, 1: true, 4: true, 12: true, 24: true, 72: true, 120: true, 168: true, r.CompletedHour: true}
	for h, c := range checks {
		if c.Hour != h || c.Balance.Produced != c.Balance.Consumed+c.Balance.Held+c.Balance.Stock || c.Balance.InitialEnergy+c.Balance.Consumed != c.Balance.Energy+c.Balance.BasalSpent+c.Balance.EnergyCapLost {
			return fmt.Errorf("hour %d conservation/sequence failure", h)
		}
		if h > 0 && checks[h-1].Alive != c.Alive {
			r.PopulationChangeHours = append(r.PopulationChangeHours, h)
			selected[h-1], selected[h] = true, true
			if c.Alive == 0 {
				r.ExtinctionHours = append(r.ExtinctionHours, h)
			}
		}
	}
	for h, c := range checks {
		if selected[h] {
			r.Traces = append(r.Traces, trace(c))
		}
	}
	events := f.Kernel().Events()
	byID := make(map[sim.EventID]kernel.Event, len(events))
	for _, ev := range events {
		b, err := ev.Bytes()
		if err != nil {
			return err
		}
		r.EventBodyBytes += len(b)
		byID[ev.ID] = ev
	}
	r.Events = len(events)
	journal := f.Journal()
	r.JournalBatches = len(journal)
	for _, batch := range journal {
		for _, a := range batch.Attempts {
			c := &r.Rejected
			hour := int(a.Time / sim.SimTime(world.FoodFlowHour))
			var prior *hourTrace
			for i := range r.Traces {
				if r.Traces[i].Hour == hour+1 {
					prior = &r.Traces[i]
					break
				}
			}
			if prior != nil {
				c = &prior.PriorRejection
			}
			if a.Accepted {
				c = &r.Accepted
				ev, ok := byID[a.EventID]
				if !ok || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor {
					return fmt.Errorf("broken accepted event link: actor %d at %d", a.Actor, a.Time)
				}
				r.AcceptedLinksChecked++
				if prior != nil {
					c = &prior.PriorHour
				}
			} else {
				if a.EventID != 0 {
					return fmt.Errorf("rejected event linked: actor %d", a.Actor)
				}
				r.UnlinkedRejections++
			}
			if prior != nil {
				for i := range prior.Actors {
					if prior.Actors[i].ID == int(a.Actor) {
						prior.Actors[i].PriorHour = append(prior.Actors[i].PriorHour, actionTrace{TimeMicroseconds: int64(a.Time), Action: actionName(a.Choice.Kind), Outcome: outcomeName(a), EventID: a.EventID})
					}
				}
			}
			// Count full-run outcomes separately from the selected hourly windows.
			total := &r.Rejected
			if a.Accepted {
				total = &r.Accepted
			}
			destinations := []*counts{total}
			if prior != nil {
				destinations = append(destinations, c)
			}
			for _, dst := range destinations {
				switch a.Choice.Kind {
				case strategy.FoodFlowGather:
					dst.Gather++
					if !a.Accepted {
						switch a.Rejection {
						case world.FoodFlowGatherNoStock:
							dst.NoStock++
						case world.FoodFlowGatherCapacity:
							dst.Capacity++
						case world.FoodFlowGatherIneligible:
							dst.Ineligible++
						case world.FoodFlowGatherNoAccess:
							dst.NoAccess++
						case world.FoodFlowGatherDuplicateActor:
							dst.DuplicateActor++
						default:
							return fmt.Errorf("unknown gather denial %d", a.Rejection)
						}
					}
				case strategy.FoodFlowEat:
					dst.Eat++
				case strategy.FoodFlowWait:
					dst.Wait++
				default:
					return fmt.Errorf("unknown attempt %d", a.Choice.Kind)
				}
			}
			if a.Choice.Kind == strategy.FoodFlowGather && hour < 8 {
				r.FirstEightClaims[a.Actor-1]++
				if a.Accepted {
					r.FirstEightGather[a.Actor-1]++
				}
			}
		}
	}
	if r.CompletedHour == world.FoodFlowHorizonHours {
		switch r.Yield {
		case 8:
			if checks[168].Alive < 15 || r.Rejected.Capacity != 0 {
				return errors.New("abundant frozen gate failed")
			}
		case 3:
			if checks[72].Alive > 8 {
				return errors.New("scarce survival gate failed")
			}
			for i, n := range r.FirstEightGather {
				if n != 3 || r.FirstEightClaims[i] != 8 {
					return fmt.Errorf("scarce fairness gate failed for actor %d: claims %d successes %d", i+1, r.FirstEightClaims[i], n)
				}
			}
		case 0:
			if checks[10].Alive == 0 || checks[11].Alive != 0 || r.Accepted.Gather != 0 || r.Accepted.Eat != 0 {
				return errors.New("zero-flow gate failed")
			}
		}
	}
	return nil
}

func run(ctx context.Context, o options) (report, error) {
	r := report{Status: "synthetic/model-conditional; no historical calibration or P0 1,000-actor/decades extrapolation", Revision: "food-flow format-v1/rule-v1/projection-v1/policy food-flow@1", Yield: o.yield, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours, Policy: "food-flow@1", TokenCost: 0}
	if o.workers < 1 || o.yield < 0 || o.yield > 8 || o.hours < 1 || o.hours > world.FoodFlowHorizonHours || o.checkpointHour < 0 || o.checkpointHour > o.hours || (o.checkpointHour > 0 && o.checkpointPath == "") {
		return r, errors.New("invalid fixture or checkpoint flags")
	}
	target := o.hours
	if o.checkpointHour > 0 {
		target = o.checkpointHour
	}
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return r, err
	}
	var f *world.FoodFlow
	config := world.FoodFlowOptions{Yield: o.yield, Seed: o.seed, Workers: o.workers}
	if o.resume != "" {
		f, _, err = world.RestoreFoodFlowCheckpoint(o.resume, config)
	} else {
		f, err = world.NewFoodFlow(config)
	}
	if err != nil {
		return r, err
	}
	peakHeap := uint64(0)
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.HeapAlloc > peakHeap {
			peakHeap = m.HeapAlloc
		}
	}
	sample()
	for {
		checks := f.Checkpoints()
		if len(checks) > 0 && checks[len(checks)-1].Hour >= target {
			break
		}
		processed, stepErr := f.Step(ctx)
		if stepErr != nil {
			return r, stepErr
		}
		if !processed {
			return r, errors.New("runner stopped before requested hour")
		}
		sample()
	}
	r.CompletedHour = target
	if err := evidence(&r, f); err != nil {
		return r, err
	}
	history, _, err := f.Kernel().ExportHistory()
	if err != nil {
		return r, err
	}
	r.HistoryBytes = len(history)
	journal, err := f.ExportJournal()
	if err != nil {
		return r, err
	}
	r.JournalBytes = len(journal)
	path := o.checkpointPath
	if path == "" {
		if err := os.MkdirAll(".tmp", 0700); err != nil {
			return r, err
		}
		dir, err := os.MkdirTemp(".tmp", "foodflow-measure-")
		if err != nil {
			return r, err
		}
		defer os.RemoveAll(dir)
		path = filepath.Join(dir, "checkpoint")
	}
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		return r, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return r, err
	}
	r.CheckpointBytes, r.CheckpointDigest = stat.Size(), fmt.Sprintf("%x", digest)
	sample()
	cpuEnd, rss, err := cpuTime()
	if err != nil {
		return r, err
	}
	r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes, r.PeakHeapAllocSampled = time.Since(start).Nanoseconds(), (cpuEnd - cpuStart).Nanoseconds(), rss, peakHeap
	return r, nil
}

func main() {
	yield := flag.Int64("q", 8, "hourly production per patch (0..8)")
	seed := flag.Uint64("seed", 0, "deterministic allocation seed")
	workers := flag.Int("workers", 1, "scheduler worker count")
	hours := flag.Int("hours", world.FoodFlowHorizonHours, "absolute hourly report horizon (1..168)")
	checkpointHour := flag.Int("checkpoint-hour", 0, "absolute hour to stop and save (requires -checkpoint)")
	checkpointPath := flag.String("checkpoint", "", "new immutable checkpoint path; default measures a temporary checkpoint in .tmp")
	resume := flag.String("resume", "", "resume a compatible food-flow checkpoint")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	r, err := run(context.Background(), options{yield: *yield, seed: *seed, workers: *workers, hours: *hours, checkpointHour: *checkpointHour, checkpointPath: *checkpointPath, resume: *resume})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
