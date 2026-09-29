// Command capacity runs the frozen productive-capacity v3 pilot. It publishes
// a verified neutral h0 v3 checkpoint, records the enable/disable intervention
// (plus the recorded founder-B endowment variant, enabled-only, because the
// frozen conservation makes founder-A impossible and NewCapacity refuses
// founders on the disabled branch) as branch children sharing the parent
// digest, continues each child to the requested hourly horizon, and reports
// per-branch survival trajectories, per-actor capital and granary ledgers,
// the predeclared 48-hour surplus-flow window, distribution metrics, typed
// event counts and measured costs. Its reports are synthetic/model-conditional
// diagnostics of the frozen fixture: no historical calibration, no takeoff
// claim, and no P0 scale extrapolation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

const (
	neutralBundleName  = "capacity-neutral-h0.bundle"
	enabledBundleName  = "capacity-branch-enabled.bundle"
	disabledBundleName = "capacity-branch-disabled.bundle"
	enabledBranchFlag  = "enabled"
	disabledBranchFlag = "disabled"
	bothBranchesFlag   = "both"
	founderNoneFlag    = "none"
	founderBFlag       = "B"
)

// founderBEndowment is the recorded founder-B variant of the frozen matrix:
// actor 1 starts with a granary stock of 4. Founder-A (actor 1 capital 2) is
// not offered: an exogenous capital endowment violates the frozen h0
// conservation identity, so NewCapacity rejects it by construction.
var founderBEndowment = []world.CapacityFounder{{Actor: 1, Granary: 4}}

type options struct {
	yield         int64
	seed          uint64
	workers       int
	hours         int
	branch        string
	founder       string
	out           string
	checkpointDir string
}

type checkpointInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type actorFinal struct {
	ID                  int   `json:"id"`
	Energy              int64 `json:"energy"`
	Hunger              int64 `json:"hunger"`
	Capital             int64 `json:"capital"`
	Wip                 int64 `json:"wip"`
	GranaryStock        int64 `json:"granary_stock"`
	InvestedUnits       int64 `json:"invested_units"`
	PointsCreated       int64 `json:"points_created"`
	PointsDecayed       int64 `json:"points_decayed"`
	YieldTotal          int64 `json:"yield_total"`
	YieldUnrealized     int64 `json:"yield_unrealized"`
	StoredMeals         int64 `json:"stored_meals"`
	WildMeals           int64 `json:"wild_meals"`
	SurvivedThroughHour int   `json:"survived_through_hour"`
	DeathHour           int   `json:"death_hour"` // -1 while alive at the final hour
}

// totalsReport carries the aggregate frozen ledgers of the final hour.
// total_points_created/decayed are the G2 capital-point counters (created
// minus decayed equals standing capital); total_wear_debt is the final
// transient depreciation debt snapshot. The fixture has no cumulative
// wear-created counter: depreciation is accrual against standing points.
type totalsReport struct {
	Alive           int   `json:"alive"`
	TotalK          int64 `json:"total_k"`
	TotalGranary    int64 `json:"total_granary"`
	TotalWip        int64 `json:"total_wip"`
	TotalInvested   int64 `json:"total_invested_units"`
	PointsCreated   int64 `json:"total_points_created"`
	PointsDecayed   int64 `json:"total_points_decayed"`
	WearDebt        int64 `json:"total_wear_debt"`
	YieldTotal      int64 `json:"total_yield"`
	YieldUnrealized int64 `json:"total_yield_unrealized"`
	StoredMeals     int64 `json:"total_stored_meals"`
	WildMeals       int64 `json:"total_wild_meals"`
	Held            int64 `json:"total_held"`
	Stock           int64 `json:"total_patch_stock"`
	PatchUnrealized int64 `json:"total_patch_unrealized"`
	BasalSpent      int64 `json:"total_basal_spent"`
	EnergyCapLost   int64 `json:"total_energy_cap_lost"`
}

// surplusFlowReport is the predeclared 48-hour window: the sum over hours
// h in [120,168] of (capital yield − stored meals), computed from cumulative
// checkpoint deltas. Before h168 the window clamps to the available hours
// (an empty window below h120 reports zero with clamped=true).
type surplusFlowReport struct {
	WindowStartHour int   `json:"window_start_hour"`
	WindowEndHour   int   `json:"window_end_hour"`
	Hours           int   `json:"hours"`
	Clamped         bool  `json:"clamped"`
	CapitalYield    int64 `json:"capital_yield"`
	StoredMeals     int64 `json:"stored_meals"`
	Net             int64 `json:"net"`
}

// giniReport is the standard Gini coefficient over one population with an
// explicit zero-total definition: zero total is zero inequality, not an error.
type giniReport struct {
	Total  int64   `json:"total"`
	Actors int     `json:"actors"`
	Gini   float64 `json:"gini"`
}

type patchGiniReport struct {
	Patch    int     `json:"patch"`
	Consumed int64   `json:"consumed_total"`
	Actors   int     `json:"actors"`
	Gini     float64 `json:"wild_consumption_gini"`
}

type denialCounts struct {
	NoStock        int64 `json:"no_stock"`
	Capacity       int64 `json:"capacity"`
	Ineligible     int64 `json:"ineligible"`
	NoAccess       int64 `json:"no_access"`
	DuplicateActor int64 `json:"duplicate_actor"`
}

// eventCounts are typed journal counts over the whole continuation. Builds
// are accepted bag-unit investments; paired-meal and fallback-eatstored rows
// split into committed meals and typed no-stored-meal notes that mutate
// nothing.
type eventCounts struct {
	GatherClaims          int64        `json:"gather_claims"`
	GatherAccepted        int64        `json:"gather_accepted"`
	GatherDenied          denialCounts `json:"gather_denied"`
	Eat                   int64        `json:"eat"`
	EatStored             int64        `json:"eat_stored"`
	Wait                  int64        `json:"wait"`
	Rest                  int64        `json:"rest"`
	Builds                int64        `json:"builds"`
	PairedMeals           int64        `json:"paired_meals"`
	PairedMealNoMealNotes int64        `json:"paired_meal_no_meal_notes"`
	FallbackEatStored     int64        `json:"fallback_eatstored"`
	FallbackNoMealNotes   int64        `json:"fallback_no_meal_notes"`
}

type branchReport struct {
	Enabled              bool               `json:"enabled"`
	Founder              string             `json:"founder"`
	ParentDigest         string             `json:"parent_digest"`
	BranchDigest         string             `json:"branch_digest"`
	BranchCheckpoint     checkpointInfo     `json:"branch_checkpoint"`
	CompletedHour        int                `json:"completed_hour"`
	AliveByHour          []int              `json:"alive_by_hour"`
	FinalAlive           int                `json:"final_alive"`
	SurvivorIDs          []int              `json:"survivor_ids"`
	Actors               []actorFinal       `json:"actors"`
	Totals               totalsReport       `json:"totals"`
	SurplusFlow          surplusFlowReport  `json:"surplus_flow_48h"`
	PatchWildConsumption [2]patchGiniReport `json:"patch_wild_consumption_gini"`
	CapitalHeld          giniReport         `json:"capital_held_gini"`
	Events               eventCounts        `json:"event_counts"`
	ConservationVerified bool               `json:"conservation_verified"`
	EventCount           int                `json:"events"`
	EventBodyBytes       int                `json:"event_body_bytes"`
	AcceptedLinksChecked int                `json:"accepted_event_links_checked"`
	UnlinkedRejections   int                `json:"unlinked_rejections_checked"`
	HistoryBytes         int                `json:"history_bytes"`
	JournalBytes         int                `json:"journal_bytes"`
	WallTimeNS           int64              `json:"wall_time_ns"`
	CPUTimeNS            int64              `json:"cpu_time_ns"`
	PeakRSSBytes         int64              `json:"peak_rss_bytes"`
	TokenCost            int                `json:"token_cost"`
}

type aliveDifference struct {
	Hour     int `json:"hour"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
}

type comparisonReport struct {
	ParentDigestShared bool              `json:"parent_digest_shared"`
	AliveDifferences   []aliveDifference `json:"alive_differences"`
	FinalAliveEnabled  int               `json:"final_alive_enabled"`
	FinalAliveDisabled int               `json:"final_alive_disabled"`
	BuildsEnabled      int64             `json:"builds_enabled"`
	BuildsDisabled     int64             `json:"builds_disabled"`
	TotalKEnabled      int64             `json:"total_k_enabled"`
	TotalKDisabled     int64             `json:"total_k_disabled"`
}

type report struct {
	Status            string            `json:"status"`
	Revision          string            `json:"revision"`
	Yield             int64             `json:"yield_per_patch_per_hour"`
	Seed              uint64            `json:"seed"`
	Workers           int               `json:"workers"`
	RequestedHours    int               `json:"requested_hours"`
	CompletedHour     int               `json:"completed_hour"`
	Policy            string            `json:"policy"`
	BranchSelection   string            `json:"branch_selection"`
	Founder           string            `json:"founder"`
	CheckpointDir     string            `json:"checkpoint_dir"`
	NeutralCheckpoint checkpointInfo    `json:"neutral_checkpoint"`
	Branches          []branchReport    `json:"branches"`
	Comparison        *comparisonReport `json:"branch_comparison,omitempty"`
	WallTimeNS        int64             `json:"wall_time_ns"`
	CPUTimeNS         int64             `json:"cpu_time_ns"`
	PeakRSSBytes      int64             `json:"peak_rss_bytes"`
	TokenCost         int               `json:"token_cost"`
}

// cpuTime returns accumulated process CPU time and the Linux high-water RSS.
// Maxrss is process-wide, not per-branch.
func cpuTime() (time.Duration, int64, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, err
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second + time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond, usage.Maxrss * 1024, nil // Linux ru_maxrss is KiB.
}

// gini is the standard Gini coefficient: the ordered-pair absolute-difference
// sum divided by 2·n²·mean (maximum (n−1)/n). A zero total is defined as zero
// inequality, not an error.
func gini(values []int64) float64 {
	var total int64
	for _, v := range values {
		if v < 0 {
			return 0
		}
		total += v
	}
	if total == 0 {
		return 0
	}
	n := int64(len(values))
	var absSum int64
	for _, xi := range values {
		for _, xj := range values {
			d := xi - xj
			if d < 0 {
				d = -d
			}
			absSum += d
		}
	}
	mean := float64(total) / float64(n)
	return float64(absSum) / (2 * float64(n*n) * mean)
}

func digestString(d [32]byte) string { return fmt.Sprintf("%x", d) }

// prepareCheckpointDir creates the bundle directory. An existing path is never
// reused or populated: all published artifacts land at previously absent paths.
func prepareCheckpointDir(path string) (string, error) {
	if path == "" {
		return os.MkdirTemp("", "capacity-checkpoint-")
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("checkpoint directory already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

// gatherDenialName maps a typed gather rejection to its report label.
func gatherDenialName(r world.FoodFlowGatherRejection) string {
	switch r {
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

// capitalEvidence reports whether a journal attempt kind is capital evidence:
// build, stored-meal draws and their typed notes. A disabled branch must
// record none of them, and q0 must record none on either branch.
func capitalEvidence(a world.CapacityAttempt) bool {
	switch a.Kind {
	case "build", "eat-stored", "paired-meal", "fallback-eatstored":
		return true
	default:
		return false
	}
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.CapacityHandoff) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.CapacityRegistry()
	if err != nil {
		return nil, err
	}
	k, head, err := kernel.RestoreHistory(reg, handoff.History)
	if err != nil {
		return nil, fmt.Errorf("accepted history replay: %v", err)
	}
	if head != handoff.Head {
		return nil, errors.New("restored history head mismatch")
	}
	events := k.Events()
	br.EventCount = len(events)
	byID := make(map[sim.EventID]kernel.Event, len(events))
	for _, ev := range events {
		body, err := ev.Bytes()
		if err != nil {
			return nil, err
		}
		br.EventBodyBytes += len(body)
		byID[ev.ID] = ev
	}
	return byID, nil
}

// branchEvidence cross-checks the journal against the hourly projection and
// the restored accepted history, then assembles the branch report.
func branchEvidence(br *branchReport, f *world.Capacity, byID map[sim.EventID]kernel.Event, enabled bool, o options) error {
	checks := f.Checkpoints()
	if len(checks) != o.hours+1 || checks[o.hours].Hour != o.hours {
		return errors.New("missing hourly projection")
	}
	br.AliveByHour = make([]int, len(checks))
	for h, c := range checks {
		if c.Hour != h {
			return fmt.Errorf("hour sequence break at %d", h)
		}
		br.AliveByHour[h] = c.Alive
		// The frozen G1-G3 gates, re-verified independently at every hour.
		balance, err := world.CapacityCheckConservation(c.Patches, c.Slots, c.Actors)
		if err != nil || balance != c.Balance {
			return fmt.Errorf("hour %d conservation failure: %v", h, err)
		}
	}
	br.ConservationVerified = true
	final := checks[o.hours]
	br.FinalAlive, br.SurvivorIDs = final.Alive, []int{}
	for i, a := range final.Actors {
		survived, death := 0, -1
		for _, c := range checks {
			if c.Actors[i].Body.Energy > 0 {
				survived = c.Hour
			} else if death < 0 {
				death = c.Hour
			}
		}
		if a.Body.Energy > 0 {
			br.SurvivorIDs = append(br.SurvivorIDs, i+1)
		}
		br.Actors = append(br.Actors, actorFinal{
			ID: i + 1, Energy: a.Body.Energy, Hunger: a.Body.Hunger,
			Capital: a.Worksite.Capital, Wip: a.Worksite.Wip,
			GranaryStock: a.Granary.Stock, InvestedUnits: a.Worksite.InvestedUnits,
			PointsCreated: a.Worksite.PointsCreated, PointsDecayed: a.Worksite.PointsDecayed,
			YieldTotal: a.Granary.YieldTotal, YieldUnrealized: a.Granary.YieldUnrealized,
			StoredMeals: a.Granary.StoredMeals, WildMeals: a.Body.Consumed,
			SurvivedThroughHour: survived, DeathHour: death,
		})
	}
	b := final.Balance
	br.Totals = totalsReport{
		Alive: final.Alive, TotalK: b.Capital, TotalGranary: b.GranaryStock, TotalWip: b.Wip,
		TotalInvested: b.Invested, PointsCreated: b.PointsCreated, PointsDecayed: b.PointsDecayed,
		YieldTotal: b.YieldTotal, YieldUnrealized: b.YieldUnrealized, StoredMeals: b.StoredMeals,
		WildMeals: b.Consumed, Held: b.Held, Stock: b.Stock, PatchUnrealized: b.Unrealized,
		BasalSpent: b.BasalSpent, EnergyCapLost: b.EnergyCapLost,
	}
	var debt int64
	for _, a := range final.Actors {
		debt += a.Worksite.WearDebt
	}
	br.Totals.WearDebt = debt
	// Predeclared surplus-flow window: hours [120, 168], clamped to the
	// completed horizon; cumulative ledger deltas per hour.
	start, end := 120, o.hours
	if end > world.CapacityHorizonHours {
		end = world.CapacityHorizonHours
	}
	br.SurplusFlow = surplusFlowReport{WindowStartHour: start, WindowEndHour: end, Clamped: end != world.CapacityHorizonHours}
	if end >= start {
		br.SurplusFlow.Hours = end - start + 1
		for h := start; h <= end; h++ {
			br.SurplusFlow.CapitalYield += checks[h].Balance.YieldTotal - checks[h-1].Balance.YieldTotal
			br.SurplusFlow.StoredMeals += checks[h].Balance.StoredMeals - checks[h-1].Balance.StoredMeals
		}
	}
	br.SurplusFlow.Net = br.SurplusFlow.CapitalYield - br.SurplusFlow.StoredMeals
	for patch := 0; patch < world.CapacityPatchCount; patch++ {
		consumed := make([]int64, 0, world.CapacitySlotsPerPatch)
		var total int64
		for _, a := range final.Actors[patch*world.CapacitySlotsPerPatch : (patch+1)*world.CapacitySlotsPerPatch] {
			consumed = append(consumed, a.Body.Consumed)
			total += a.Body.Consumed
		}
		br.PatchWildConsumption[patch] = patchGiniReport{Patch: 1001 + patch, Consumed: total, Actors: len(consumed), Gini: gini(consumed)}
	}
	capital := make([]int64, 0, world.CapacityActorCount)
	for _, a := range final.Actors {
		capital = append(capital, a.Worksite.Capital)
	}
	br.CapitalHeld = giniReport{Total: final.Balance.Capital, Actors: len(capital), Gini: gini(capital)}
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Key != "" {
				ev, ok := byID[a.EventID]
				if !ok || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor {
					return fmt.Errorf("broken accepted event link: actor %d kind %s at %d", a.Actor, a.Kind, a.Time)
				}
				br.AcceptedLinksChecked++
			} else if a.EventID != 0 {
				return fmt.Errorf("rejected attempt linked: actor %d kind %s", a.Actor, a.Kind)
			} else {
				br.UnlinkedRejections++
			}
			switch a.Kind {
			case "gather":
				// Capacity journal discipline: a gather claim is admitted iff
				// its typed rejection label is FoodFlowGatherAdmitted. Gather
				// rows are unkeyed either way — the admitted gather commits
				// inside the claim event, and only attempts that carry their
				// own committed event (meals, builds) set Key/Accepted.
				br.Events.GatherClaims++
				if a.Rejection == world.FoodFlowGatherAdmitted {
					br.Events.GatherAccepted++
				} else {
					switch gatherDenialName(a.Rejection) {
					case "no_stock":
						br.Events.GatherDenied.NoStock++
					case "capacity":
						br.Events.GatherDenied.Capacity++
					case "ineligible":
						br.Events.GatherDenied.Ineligible++
					case "no_access":
						br.Events.GatherDenied.NoAccess++
					case "duplicate_actor":
						br.Events.GatherDenied.DuplicateActor++
					default:
						return fmt.Errorf("unknown gather denial %d", a.Rejection)
					}
				}
			case "eat":
				br.Events.Eat++
			case "eat-stored":
				br.Events.EatStored++
			case "wait":
				br.Events.Wait++
			case "rest":
				br.Events.Rest++
			case "build":
				br.Events.Builds++
			case "paired-meal":
				if a.NoMeal != "" {
					br.Events.PairedMealNoMealNotes++
				} else {
					br.Events.PairedMeals++
				}
			case "fallback-eatstored":
				if a.NoMeal != "" {
					br.Events.FallbackNoMealNotes++
				} else {
					br.Events.FallbackEatStored++
				}
			default:
				return fmt.Errorf("unknown journal attempt kind %q", a.Kind)
			}
		}
	}
	// Disabled branches are capital-free by the frozen intervention: no
	// build, stored-meal draw or paired meal may appear, and no capital
	// ledger may move. Any capital event at q0 is a defect on either branch.
	if !enabled && (br.Events.Builds != 0 || br.Events.EatStored != 0 || br.Events.PairedMeals != 0 ||
		br.Events.PairedMealNoMealNotes != 0 || br.Events.FallbackEatStored != 0 || br.Events.FallbackNoMealNotes != 0) {
		return errors.New("disabled branch recorded capital evidence")
	}
	if !enabled || o.yield == 0 {
		if br.Totals.TotalK != 0 || br.Totals.TotalWip != 0 || br.Totals.TotalInvested != 0 ||
			br.Totals.PointsCreated != 0 || br.Totals.YieldTotal != 0 || br.Totals.StoredMeals != 0 {
			return errors.New("capital ledger moved without capital evidence")
		}
	}
	if o.yield == 0 {
		if br.Events.Builds != 0 || br.Events.EatStored != 0 || br.Events.PairedMeals != 0 ||
			br.Events.PairedMealNoMealNotes != 0 || br.Events.FallbackEatStored != 0 || br.Events.FallbackNoMealNotes != 0 {
			return errors.New("q0 recorded capital events")
		}
		if o.hours >= 11 && br.AliveByHour[11] != 0 {
			return fmt.Errorf("q0 actors survived past h11: %d alive", br.AliveByHour[11])
		}
		if o.hours >= 12 && br.AliveByHour[12] != 0 {
			return fmt.Errorf("q0 actors survived past h12: %d alive", br.AliveByHour[12])
		}
	}
	return nil
}

// runBranch restores one intervention child, continues it to the requested
// horizon and reports trajectories, ledgers and measured costs. The restore,
// run, verification and serialization all sit inside the timing.
func runBranch(ctx context.Context, o options, enabled bool, founders []world.CapacityFounder, parentDigest string, childPath string, childDigest [32]byte, childBytes int64) (branchReport, error) {
	br := branchReport{Enabled: enabled, Founder: o.founder, ParentDigest: parentDigest, BranchDigest: digestString(childDigest), CompletedHour: o.hours,
		BranchCheckpoint: checkpointInfo{Name: filepath.Base(childPath), Digest: digestString(childDigest), Bytes: childBytes},
		TokenCost:        0}
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return br, err
	}
	opts := world.CapacityOptions{Yield: o.yield, Seed: o.seed, Workers: o.workers, Enabled: enabled, Founders: founders}
	f, restored, err := world.RestoreCapacityCheckpoint(childPath, opts)
	if err != nil {
		return br, err
	}
	if restored != childDigest {
		return br, errors.New("restored child digest differs from the published branch digest")
	}
	for len(f.Checkpoints()) <= o.hours {
		ok, err := f.Step(ctx)
		if err != nil {
			return br, err
		}
		if !ok {
			return br, errors.New("runner stopped before requested hour")
		}
	}
	handoff, err := f.Handoff()
	if err != nil {
		return br, err
	}
	br.HistoryBytes = len(handoff.History)
	byID, err := historyEvidence(&br, handoff)
	if err != nil {
		return br, err
	}
	if err := branchEvidence(&br, f, byID, enabled, o); err != nil {
		return br, err
	}
	journalBytes, err := f.ExportJournal()
	if err != nil {
		return br, err
	}
	br.JournalBytes = len(journalBytes)
	cpuEnd, rss, err := cpuTime()
	if err != nil {
		return br, err
	}
	br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes = time.Since(start).Nanoseconds(), (cpuEnd - cpuStart).Nanoseconds(), rss
	return br, nil
}

func compareBranches(r *report) {
	var enabled, disabled *branchReport
	for i := range r.Branches {
		if r.Branches[i].Enabled {
			enabled = &r.Branches[i]
		} else {
			disabled = &r.Branches[i]
		}
	}
	if enabled == nil || disabled == nil {
		return
	}
	c := &comparisonReport{ParentDigestShared: enabled.ParentDigest == disabled.ParentDigest,
		FinalAliveEnabled: enabled.FinalAlive, FinalAliveDisabled: disabled.FinalAlive,
		BuildsEnabled: enabled.Events.Builds, BuildsDisabled: disabled.Events.Builds,
		TotalKEnabled: enabled.Totals.TotalK, TotalKDisabled: disabled.Totals.TotalK}
	for h := 0; h <= enabled.CompletedHour && h <= disabled.CompletedHour; h++ {
		if enabled.AliveByHour[h] != disabled.AliveByHour[h] {
			c.AliveDifferences = append(c.AliveDifferences, aliveDifference{Hour: h, Enabled: enabled.AliveByHour[h], Disabled: disabled.AliveByHour[h]})
		}
	}
	r.Comparison = c
}

func run(ctx context.Context, o options) (report, error) {
	r := report{
		Status:   "synthetic/model-conditional; frozen productive-capacity v3 fixture; no historical calibration, takeoff, or P0 scale claim",
		Revision: "capacity format-v3/schema-v3/rule-v3/projection-v3/policy capacity@3",
		Yield:    o.yield, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: "capacity@3", BranchSelection: o.branch, Founder: o.founder, TokenCost: 0,
	}
	if o.workers < 1 || o.yield < 0 || o.yield > world.CapacitySlotsPerPatch || o.hours < 1 || o.hours > world.CapacityHorizonHours {
		return r, errors.New("invalid fixture flags")
	}
	switch o.branch {
	case enabledBranchFlag, disabledBranchFlag, bothBranchesFlag:
	default:
		return r, fmt.Errorf("invalid -branch value %q", o.branch)
	}
	var founders []world.CapacityFounder
	switch o.founder {
	case founderNoneFlag:
	case founderBFlag:
		founders = founderBEndowment
		// Founder endowments exist only on the enabled branch: NewCapacity
		// rejects founders×disabled, so the disabled cell is not runnable.
		if o.branch != enabledBranchFlag {
			return r, fmt.Errorf("founder cells are enabled-only; refused -branch=%q", o.branch)
		}
	default:
		return r, fmt.Errorf("invalid -founder value %q", o.founder)
	}
	if o.out != "" {
		if _, err := os.Stat(o.out); err == nil {
			return r, fmt.Errorf("output file already exists: %s", o.out)
		} else if !errors.Is(err, os.ErrNotExist) {
			return r, err
		}
	}
	dir, err := prepareCheckpointDir(o.checkpointDir)
	if err != nil {
		return r, err
	}
	r.CheckpointDir = dir
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return r, err
	}
	// The neutral h0 pulse: exactly one committed step, no claim and no
	// capital evidence, before any intervention. SaveCheckpoint verifies that
	// boundary. Founder variants construct with Enabled=true (NewCapacity
	// rejects founders on a disabled world) but are equally pre-intervention.
	rootEnabled := len(founders) > 0
	neutral, err := world.NewCapacity(world.CapacityOptions{Yield: o.yield, Seed: o.seed, Workers: 1, Enabled: rootEnabled, Founders: founders})
	if err != nil {
		return r, err
	}
	if ok, err := neutral.Step(ctx); err != nil || !ok {
		return r, fmt.Errorf("neutral h0 pulse: processed=%t err=%v", ok, err)
	}
	rootPath := filepath.Join(dir, neutralBundleName)
	rootDigest, err := neutral.SaveCheckpoint(rootPath)
	if err != nil {
		return r, err
	}
	rootStat, err := os.Stat(rootPath)
	if err != nil {
		return r, err
	}
	r.NeutralCheckpoint = checkpointInfo{Name: neutralBundleName, Digest: digestString(rootDigest), Bytes: rootStat.Size()}
	// Both intervention children branch from the same verified parent; the
	// recorded enable/disable decision is the only content difference.
	expected := world.CapacityOptions{Yield: o.yield, Seed: o.seed, Workers: 1, Enabled: rootEnabled, Founders: founders}
	type branchPlan struct {
		enabled bool
		name    string
	}
	var plans []branchPlan
	switch o.branch {
	case enabledBranchFlag:
		plans = []branchPlan{{enabled: true, name: enabledBundleName}}
	case disabledBranchFlag:
		plans = []branchPlan{{enabled: false, name: disabledBundleName}}
	default:
		plans = []branchPlan{{enabled: true, name: enabledBundleName}, {enabled: false, name: disabledBundleName}}
	}
	parent := digestString(rootDigest)
	for _, plan := range plans {
		childPath := filepath.Join(dir, plan.name)
		digest, err := world.BranchCapacityCheckpoint(rootPath, expected, plan.enabled, childPath)
		if err != nil {
			return r, err
		}
		stat, err := os.Stat(childPath)
		if err != nil {
			return r, err
		}
		br, err := runBranch(ctx, o, plan.enabled, founders, parent, childPath, digest, stat.Size())
		if err != nil {
			return r, err
		}
		r.Branches = append(r.Branches, br)
	}
	if len(r.Branches) == 0 {
		return r, fmt.Errorf("no branch selected for %q", o.branch)
	}
	r.CompletedHour = o.hours
	compareBranches(&r)
	cpuEnd, rss, err := cpuTime()
	if err != nil {
		return r, err
	}
	r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes = time.Since(start).Nanoseconds(), (cpuEnd - cpuStart).Nanoseconds(), rss
	return r, nil
}

func main() {
	yield := flag.Int64("q", 8, "hourly wild production per patch (0..8)")
	seed := flag.Uint64("seed", 0, "deterministic allocation seed")
	workers := flag.Int("workers", 1, "scheduler worker count for every branch continuation")
	hours := flag.Int("hours", world.CapacityHorizonHours, "absolute hourly horizon per branch (1..168)")
	branch := flag.String("branch", bothBranchesFlag, "which recorded intervention to run: enabled, disabled, or both")
	founder := flag.String("founder", founderNoneFlag, "recorded endowment variant: none, or B (actor 1 granary 4; enabled-only; founder-A is impossible under frozen conservation)")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the neutral bundle and the branch children; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	r, err := run(context.Background(), options{yield: *yield, seed: *seed, workers: *workers, hours: *hours, branch: *branch, founder: *founder, out: *out, checkpointDir: *checkpointDir})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded = append(encoded, '\n')
	os.Stdout.Write(encoded)
	if *out != "" {
		file, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if _, err := file.Write(encoded); err != nil {
			file.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := file.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
