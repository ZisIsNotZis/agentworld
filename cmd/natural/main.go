// Command natural runs the frozen v5 natural-limits pilot (ticket 15).
// It publishes a verified neutral h0 v5 root captured with BIRTHS DISABLED
// (the v5 h0 pulse is births-asymmetric by frozen design, so a disabled root
// is the only legal branch parent), records the recorded interventions as
// branch children sharing the parent digest — births-disabled (same-flag
// lineage) and the births-ENABLE intervention (the reverse direction is
// refused by the persistence layer) — and continues each child to the
// requested hourly horizon. The optional -founder-tenure flag selects the
// persistence lane's declared founder-tenure fixture genesis (births-enabled
// only), the only genesis under which organic births are reachable; the
// published root is then the fixture's first pulse-boundary continuation.
// The optional -cultivation-disabled flag selects the declared cultivation
// ablation (frozen matrix cell D, births-disabled only): the frozen policy
// is unchanged, the runner routes every Build choice through the documented
// denial loop, and any build event or attempt is a hard defect.
//
// Per branch the reporter re-verifies the extended frozen conservation
// identities at every hour, classifies typed journal evidence, verifies
// keyed attempts against the restored accepted history, and reports: the
// population trajectory (alive-by-hour with births and cause-attributed
// deaths per hour and the extinction hour), the per-hour regen split
// (cultivated/wild/unrealized) with cultivated slots per patch, granary
// inflow/spoiled/stored aggregates, the Malthus index (Σmeals − Σbasal over
// the trailing 24 h), per-actor ledgers (energy, capital, wip, granary,
// tenure count), typed event counts, the frozen compute-ceiling headroom
// ledger, and every typed NaturalCeilingHit with its P15 verdict
// classification (outgrew-harness vs design-defect vs implementation-defect
// vs runtime-defect), plus event/history/journal/checkpoint bytes and
// wall/CPU/peak-RSS costs against the predeclared v4-relative budgets. A
// halting ceiling hit is a first-class outcome: the run records the hit,
// then runs a workers-4 twin (workers-1 when the cell ran at 4) truncated at
// the hit hour for replay equality. The frozen bands are evaluated verbatim;
// a falsified band is reported, never tuned. Its reports are
// synthetic/model-conditional diagnostics of the frozen fixture: no
// historical calibration, no takeoff claim, and no P0 scale extrapolation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

const (
	organicRootBundleName  = "natural-neutral-h0.bundle"
	enabledBundleName      = "natural-branch-enabled.bundle"
	disabledBundleName     = "natural-branch-disabled.bundle"
	fixtureRootBundleName  = "natural-fixture-root.bundle"
	enabledBranchFlag      = "enabled"
	disabledBranchFlag     = "disabled"
	bothBranchesFlag       = "both"
	rootKindNeutral        = "neutral-h0-births-disabled"
	rootKindFixture        = "founder-tenure-pulse-boundary-births-enabled"
	verdictOutgrew         = "outgrew-harness"
	verdictDesignDefect    = "design-defect"
	verdictImplementation  = "implementation-defect"
	verdictRuntime         = "runtime-defect"
	verdictNoBranchFired   = "no-frozen-branch-fired"
	evidencePostHaltNote   = "runner halted by a typed ceiling hit; the fail-loud halt refuses accepted-history export, so history-derived evidence is unavailable"
	extrapolationOnlyLabel = "extrapolation-only"
)

// Recorded ticket-14 cost baselines (frozen spec budgets): v4 measured
// 0.165 s CPU per simulated hour (27.8 s / 168 h, q8-enabled w1) and
// 14,592,912 accepted-history bytes over 168 h. The predeclared v5 budgets
// are CPU ≤ 3× v4 and bytes ≤ 2× v4 per simulated hour; misses are reported
// verbatim, never tuned.
const (
	v4RecordedCPUSecondsPerSimulatedHour = 0.165
	v4RecordedHistoryBytes               = 14592912
	v4RecordedHistoryHours               = 168
	cpuBudgetFactor                      = 3.0
	bytesBudgetFactor                    = 2.0
)

type options struct {
	patches             int
	r                   int64
	seed                uint64
	workers             int
	hours               int
	births              string
	fixture             bool
	cultivationDisabled bool
	out                 string
	checkpointDir       string
}

type checkpointInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type rootReport struct {
	Kind      string `json:"kind"`
	FirstHour int    `json:"first_hour"`
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	Bytes     int64  `json:"bytes"`
}

type populationReport struct {
	AliveByHour            []int   `json:"alive_by_hour"`
	BirthsByHour           []int64 `json:"births_by_hour"`
	DeathsStarvationByHour []int64 `json:"deaths_starvation_by_hour"`
	DeathsAgeByHour        []int64 `json:"deaths_age_by_hour"`
	FinalAlive             int     `json:"final_alive"`
	SurvivorIDs            []int   `json:"survivor_ids"`
	ExtinctionHour         int     `json:"extinction_hour"`
}

type regenReport struct {
	CultivatedByHour             []int64 `json:"cultivated_by_hour"`
	WildByHour                   []int64 `json:"wild_by_hour"`
	UnrealizedByHour             []int64 `json:"unrealized_by_hour"`
	CultivatedSlotsTotalByHour   []int   `json:"cultivated_slots_total_by_hour"`
	CultivatedSlotsPerPatchFinal []int   `json:"cultivated_slots_per_patch_final"`
	TotalCultivated              int64   `json:"total_cultivated"`
	TotalWild                    int64   `json:"total_wild"`
	TotalUnrealized              int64   `json:"total_unrealized"`
}

type granaryReport struct {
	InflowTotal      int64   `json:"inflow_total"`
	SpoiledTotal     int64   `json:"spoiled_total"`
	StoredMealsTotal int64   `json:"stored_meals_total"`
	StockFinal       int64   `json:"stock_final"`
	SpoilDebtFinal   int64   `json:"spoil_debt_final"`
	InflowByHour     []int64 `json:"inflow_by_hour"`
	SpoiledByHour    []int64 `json:"spoiled_by_hour"`
	StoredByHour     []int64 `json:"stored_meals_by_hour"`
}

type malthusReport struct {
	Definition  string  `json:"definition"`
	IndexByHour []int64 `json:"index_by_hour"`
	Final       int64   `json:"final"`
}

type actorFinal struct {
	ID                 int   `json:"id"`
	Alive              bool  `json:"alive"`
	HomePatch          int   `json:"home_patch"`
	Energy             int64 `json:"energy"`
	Hunger             int64 `json:"hunger"`
	BasalSpent         int64 `json:"basal_spent"`
	EnergyCapLost      int64 `json:"energy_cap_lost"`
	WildMeals          int64 `json:"wild_meals"`
	LastGatherHour     int64 `json:"last_gather_hour"`
	BagUnits           int64 `json:"bag_units"`
	Capital            int64 `json:"capital"`
	Wip                int64 `json:"wip"`
	WearDebt           int64 `json:"wear_debt"`
	InvestedUnits      int64 `json:"invested_units"`
	PointsCreated      int64 `json:"points_created"`
	PointsDecayed      int64 `json:"points_decayed"`
	LastBuildHour      int64 `json:"last_build_hour"`
	GranaryStock       int64 `json:"granary_stock"`
	CultivatedTotal    int64 `json:"cultivated_total"`
	StoredMeals        int64 `json:"stored_meals"`
	LastStoredMealHour int64 `json:"last_stored_meal_hour"`
	SpoilDebt          int64 `json:"spoil_debt"`
	Spoiled            int64 `json:"spoiled"`
	CultivatedSlots    int   `json:"cultivated_slots"`
	LocusM             int64 `json:"locus_m"`
	LocusY             int64 `json:"locus_y"`
	LocusW             int64 `json:"locus_w"`
	ParentA            int   `json:"parent_a"`
	ParentB            int   `json:"parent_b"`
	BirthHour          int64 `json:"birth_hour"`
	DeathHour          int64 `json:"death_hour"`
	DiedHour           int64 `json:"died_hour"`
	BirthGranaryPaid   int64 `json:"birth_granary_paid"`
}

type totalsReport struct {
	Alive            int   `json:"alive"`
	Produced         int64 `json:"total_produced"`
	Granted          int64 `json:"total_granted"`
	Unrealized       int64 `json:"total_unrealized"`
	Gathered         int64 `json:"total_gathered"`
	Consumed         int64 `json:"total_wild_meals"`
	Stock            int64 `json:"total_patch_stock"`
	Held             int64 `json:"total_held"`
	Invested         int64 `json:"total_invested_units"`
	CultivatedTotal  int64 `json:"total_cultivated_inflow"`
	GranaryStock     int64 `json:"total_granary"`
	StoredMeals      int64 `json:"total_stored_meals"`
	Spoiled          int64 `json:"total_spoiled"`
	Capital          int64 `json:"total_k"`
	Wip              int64 `json:"total_wip"`
	PointsCreated    int64 `json:"total_points_created"`
	PointsDecayed    int64 `json:"total_points_decayed"`
	TenuredSlots     int64 `json:"total_tenured_slots"`
	InitialEnergy    int64 `json:"total_initial_energy"`
	Energy           int64 `json:"total_energy"`
	BasalSpent       int64 `json:"total_basal_spent"`
	EnergyCapLost    int64 `json:"total_energy_cap_lost"`
	BirthGranaryPaid int64 `json:"total_birth_granary_paid"`
	Births           int64 `json:"total_births"`
	BirthRequests    int64 `json:"total_birth_requests"`
	Divergences      int64 `json:"total_claim_vs_truth_divergences"`
}

type eventCounts struct {
	GatherClaims        int64            `json:"gather_claims"`
	GatherAdmitted      int64            `json:"gather_admitted"`
	GatherDenied        int64            `json:"gather_denied"`
	Eat                 int64            `json:"eat"`
	EatStored           int64            `json:"eat_stored"`
	Wait                int64            `json:"wait"`
	Rest                int64            `json:"rest"`
	Builds              int64            `json:"builds"`
	BuildDenialEats     int64            `json:"build_denial_eats"`
	PairedMeals         int64            `json:"paired_meals"`
	FallbackEatStored   int64            `json:"fallback_eatstored"`
	BirthRequests       int64            `json:"birth_requests"`
	BirthReplies        int64            `json:"birth_replies"`
	BirthsAccepted      int64            `json:"births_accepted"`
	BirthRefusals       int64            `json:"birth_refusals"`
	BirthRefusalsSilent int64            `json:"birth_refusals_silent"`
	BirthWakeClaims     int64            `json:"birth_wake_claims"`
	LostBirths          int64            `json:"lost_births"`
	Refusals            map[string]int64 `json:"refusals"`
	WorldEvents         map[string]int64 `json:"world_rule_events"`
	WorldEventsNote     string           `json:"world_rule_events_note,omitempty"`
}

type ceilingRowReport struct {
	Ceiling              string `json:"ceiling"`
	Threshold            uint64 `json:"threshold"`
	Observed             uint64 `json:"observed"`
	RateLastHour         int64  `json:"rate_last_hour"`
	ProjectedBindingHour int64  `json:"projected_binding_hour"`
}

type verdictReport struct {
	Verdict                   string  `json:"verdict"`
	ConservationGreen         bool    `json:"conservation_green"`
	ReplayIdentical           *bool   `json:"replay_identical_to_hit_hour"`
	ReplayNote                string  `json:"replay_note"`
	CarryingCapacityEstimate  int64   `json:"carrying_capacity_estimate_p_times_r"`
	PopulationRatioOfEstimate float64 `json:"alive_over_carrying_capacity_estimate"`
	TurnoverHolds             bool    `json:"turnover_births_approx_deaths"`
	TurnoverBirths            int64   `json:"turnover_births_to_hit_hour"`
	TurnoverDeaths            int64   `json:"turnover_deaths_to_hit_hour"`
	StarvationExceedsBirths   bool    `json:"starvation_deaths_exceed_births"`
	UnrealizedShareOfBudget   float64 `json:"lifetime_regen_unrealized_share_of_p_times_r"`
	PopulationFlat            bool    `json:"population_flat"`
	FiberWakeExplosion        bool    `json:"fiber_wake_explosion"`
	PovertyTrapContext        bool    `json:"poverty_trap_context_zero_organic_births"`
	Rationale                 string  `json:"rationale"`
}

type ceilingHitReport struct {
	Ceiling            string        `json:"ceiling"`
	Threshold          uint64        `json:"threshold"`
	Observed           uint64        `json:"observed"`
	Hour               int           `json:"hour"`
	WorldVersion       uint64        `json:"world_version"`
	Alive              int           `json:"alive"`
	Births             int64         `json:"births"`
	Deaths             int64         `json:"deaths"`
	Last24hInflow      int64         `json:"last_24h_inflow"`
	Last24hConsumption int64         `json:"last_24h_consumption"`
	RegenUnrealized    int64         `json:"regen_unrealized_lifetime"`
	Verdict            verdictReport `json:"p15_verdict"`
}

type costBudgetReport struct {
	CPUPerSimulatedHour float64 `json:"cpu_seconds_per_simulated_hour"`
	CPUBudget           float64 `json:"cpu_budget_3x_v4"`
	CPUWithinBudget     bool    `json:"cpu_within_budget"`
	HistoryBytesPerHour float64 `json:"history_bytes_per_simulated_hour"`
	BytesBudget         float64 `json:"bytes_budget_2x_v4"`
	BytesWithinBudget   bool    `json:"bytes_within_budget"`
	BytesEvaluable      bool    `json:"bytes_budget_evaluable"`
	ActorHours          int64   `json:"actor_hours_extrapolation_only"`
	BytesPerActorHour   float64 `json:"bytes_per_actor_hour"`
	CPUPerActorHour     float64 `json:"cpu_seconds_per_actor_hour"`
	ExtrapolationLabel  string  `json:"extrapolation_label"`
}

type bandReport struct {
	Name       string `json:"name"`
	Criterion  string `json:"criterion"`
	Observed   string `json:"observed"`
	WithinBand bool   `json:"within_band"`
}

type replayTwinReport struct {
	Workers         int    `json:"workers"`
	HitHour         int    `json:"hit_hour"`
	HoursCompared   int    `json:"checkpoint_hours_compared"`
	JournalCompared int    `json:"journal_batches_compared"`
	Identical       bool   `json:"identical"`
	Divergences     int    `json:"divergences"`
	Note            string `json:"note"`
}

type branchReport struct {
	Births               bool               `json:"births"`
	Fixture              bool               `json:"founder_tenure_genesis"`
	CultivationDisabled  bool               `json:"cultivation_ablation"`
	ParentDigest         string             `json:"parent_digest"`
	BranchDigest         string             `json:"branch_digest"`
	BranchCheckpoint     checkpointInfo     `json:"branch_checkpoint"`
	FirstHour            int                `json:"first_hour"`
	CompletedHour        int                `json:"completed_hour"`
	HaltedByCeilingHit   bool               `json:"halted_by_ceiling_hit"`
	CeilingHit           *ceilingHitReport  `json:"ceiling_hit"`
	CeilingLedger        []ceilingRowReport `json:"ceiling_ledger"`
	Population           populationReport   `json:"population"`
	Regen                regenReport        `json:"regen_split"`
	Granary              granaryReport      `json:"granary"`
	Malthus              malthusReport      `json:"malthus_index"`
	Actors               []actorFinal       `json:"actors"`
	Totals               totalsReport       `json:"totals"`
	Events               eventCounts        `json:"event_counts"`
	Band                 bandReport         `json:"frozen_band"`
	CostBudget           costBudgetReport   `json:"cost_budget"`
	ConservationVerified bool               `json:"conservation_verified"`
	EventCount           int                `json:"events"`
	EventBodyBytes       int                `json:"event_body_bytes"`
	AcceptedLinksChecked int                `json:"accepted_event_links_checked"`
	UnlinkedRejections   int                `json:"unlinked_rejections_checked"`
	HistoryBytes         int                `json:"history_bytes"`
	JournalBytes         int64              `json:"journal_bytes"`
	WallTimeNS           int64              `json:"wall_time_ns"`
	CPUTimeNS            int64              `json:"cpu_time_ns"`
	PeakRSSBytes         int64              `json:"peak_rss_bytes"`
	TokenCost            int                `json:"token_cost"`
}

type report struct {
	Status              string         `json:"status"`
	Revision            string         `json:"revision"`
	Patches             int            `json:"patches"`
	Founders            int            `json:"founders"`
	R                   int64          `json:"r_per_patch_per_hour"`
	Seed                uint64         `json:"seed"`
	Workers             int            `json:"workers"`
	RequestedHours      int            `json:"requested_hours"`
	CompletedHour       int            `json:"completed_hour"`
	Policy              string         `json:"policy"`
	BranchSelection     string         `json:"branch_selection"`
	FounderTenure       bool           `json:"founder_tenure"`
	CultivationDisabled bool           `json:"cultivation_disabled"`
	CheckpointDir       string         `json:"checkpoint_dir"`
	Root                rootReport     `json:"root_checkpoint"`
	Branches            []branchReport `json:"branches"`
	WallTimeNS          int64          `json:"wall_time_ns"`
	CPUTimeNS           int64          `json:"cpu_time_ns"`
	PeakRSSBytes        int64          `json:"peak_rss_bytes"`
	TokenCost           int            `json:"token_cost"`
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

func digestString(d [32]byte) string { return fmt.Sprintf("%x", d) }

// prepareCheckpointDir creates the bundle directory. An existing path is never
// reused or populated: all published artifacts land at previously absent paths.
func prepareCheckpointDir(path string) (string, error) {
	if path == "" {
		return os.MkdirTemp("", "natural-checkpoint-")
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

// refusalName maps a typed birth refusal to its report label; an unknown
// value is a hard error, never silently bucketed.
func refusalName(r world.NaturalRefusalReason) (string, error) {
	switch r {
	case world.NaturalRefusalNone:
		return "none", nil
	case world.NaturalRefusalNotPending:
		return "not_pending", nil
	case world.NaturalRefusalDuplicateReply:
		return "duplicate_reply", nil
	case world.NaturalRefusalAddresseeDead:
		return "addressee_dead", nil
	case world.NaturalRefusalProposerDead:
		return "proposer_dead", nil
	case world.NaturalRefusalConsentOwn:
		return "consent_own", nil
	case world.NaturalRefusalConsentClaims:
		return "consent_claims", nil
	case world.NaturalRefusalKin:
		return "kin", nil
	case world.NaturalRefusalCrossPatch:
		return "cross_patch", nil
	case world.NaturalRefusalGateThresholds:
		return "gate_thresholds", nil
	default:
		return "", fmt.Errorf("unknown birth refusal reason %d", r)
	}
}

var worldRuleNames = map[sim.RuleID]string{
	world.NaturalRegenSplitRule:   "regen_split",
	world.NaturalSpoilageRule:     "spoilage",
	world.NaturalWearRule:         "wear",
	world.NaturalBasalRule:        "basal",
	world.NaturalBuildRule:        "build",
	world.NaturalGatherRule:       "gather",
	world.NaturalConsumeRule:      "consume",
	world.NaturalGrantRule:        "grant",
	world.NaturalEatStoredRule:    "eat_stored",
	world.NaturalBirthRequestRule: "birth_request",
	world.NaturalBirthReplyRule:   "birth_reply",
	world.NaturalBirthRefuseRule:  "birth_refuse",
	world.NaturalBirthExpireRule:  "birth_expiry",
	world.NaturalAgeDeathRule:     "age_death",
}

// birthEvidenceKind reports whether a journal attempt kind is birth
// evidence. A births-disabled branch must record none of them.
func birthEvidenceKind(kind string) bool {
	switch kind {
	case "birth-request", "birth-reply":
		return true
	default:
		return false
	}
}

func naturalAlive(a world.NaturalActorState) bool {
	return a.Genome.DiedHour == world.NaturalNeverHour && a.Body.Energy > 0
}

// publishedJournalBytes reads the journal section of a published root or
// branch bundle: the first-hour-prefix evidence re-pinned to the recorded
// flags by the branch intervention itself (organic cells) or the fixture
// continuation (fixture cells). ExportJournal deliberately refuses a restored
// branch child (its boundary topology is the parent's, and the v5 h0 pulse
// is births-asymmetric by frozen design), so the published artifact is the
// honest journal-bytes metric for every branch cell.
func publishedJournalBytes(path string) (int64, error) {
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, s := range sections {
		if s.Name == checkpoint.Journal {
			return int64(len(s.Data)), nil
		}
	}
	return 0, fmt.Errorf("published bundle %s carries no journal section", path)
}

// classifyVerdict applies the frozen P15 verdict table (design Q5) to one
// typed hit's evidence, in the table's own order: implementation defect on
// any conservation/replay failure, then outgrew-harness, then design defect,
// then runtime defect. N* is the derived inflow bound P·R (each adult
// metabolizes about one unit per hour), so the outgrew-harness population
// bar is deliberately conservative; every intermediate quantity is reported
// so the classification is checkable, and a falsified branch is labeled
// no-frozen-branch-fired rather than forced into a bucket.
func classifyVerdict(o options, hit world.NaturalCeilingHit, conservationGreen bool, replayIdentical *bool, checks []world.NaturalCheckpoint) verdictReport {
	v := verdictReport{
		ConservationGreen:        conservationGreen,
		ReplayIdentical:          replayIdentical,
		CarryingCapacityEstimate: int64(o.patches) * o.r,
		TurnoverBirths:           hit.Births,
		TurnoverDeaths:           hit.Deaths,
	}
	if v.CarryingCapacityEstimate > 0 {
		v.PopulationRatioOfEstimate = float64(hit.Alive) / float64(v.CarryingCapacityEstimate)
	}
	if v.CarryingCapacityEstimate > 0 {
		v.UnrealizedShareOfBudget = float64(hit.RegenUnrealized) / float64(v.CarryingCapacityEstimate*int64(hit.Hour+1))
	}
	maxTurnover := hit.Births
	if hit.Deaths > maxTurnover {
		maxTurnover = hit.Deaths
	}
	v.TurnoverHolds = maxTurnover == 0 || float64(abs64(hit.Births-hit.Deaths)) <= 0.1*float64(maxTurnover)
	starvationDeaths := checks[len(checks)-1].Metrics.DeathsStarvation
	v.StarvationExceedsBirths = starvationDeaths > hit.Births
	v.PovertyTrapContext = hit.Births == 0
	flat := true
	for _, c := range checks {
		if c.Metrics.Alive != checks[0].Metrics.Alive {
			flat = false
			break
		}
	}
	v.PopulationFlat = flat
	fibers := uint64(0)
	wakes := uint64(0)
	for _, row := range checks[len(checks)-1].Ledger {
		if row.Ceiling == world.NaturalCeilingPortableFibers {
			fibers = row.Observed
		}
		if row.Ceiling == world.NaturalCeilingPortableWakes {
			wakes = row.Observed
		}
	}
	// Explosion convention (declared, not frozen): the fiber set at a
	// quiescent boundary is exactly roster+1, and the wake set is bounded by
	// the hourly activity lattice; a multiple beyond that lattice's shape is
	// a storm, not population.
	v.FiberWakeExplosion = uint64(hit.Alive) > 0 && (fibers > 4*(uint64(hit.Alive)+1) || wakes > 16*(uint64(hit.Alive)+1))
	populationBar := v.PopulationRatioOfEstimate >= 0.9
	replayOK := replayIdentical != nil && *replayIdentical
	switch {
	case !conservationGreen || (replayIdentical != nil && !*replayIdentical):
		v.Verdict = verdictImplementation
		v.Rationale = "conservation or replay equality failed; the frozen table classifies any conservation/replay/ownership/linkage failure as an implementation defect"
	case conservationGreen && populationBar && v.TurnoverHolds && (replayIdentical == nil || replayOK):
		v.Verdict = verdictOutgrew
		v.Rationale = fmt.Sprintf("conservation green, alive %d ≥ 0.9·N* (N* = P·R = %d, ratio %.3f), turnover births≈deaths (%d≈%d), replay identical to the hit hour", hit.Alive, v.CarryingCapacityEstimate, v.PopulationRatioOfEstimate, hit.Births, hit.Deaths)
	case conservationGreen && hit.Deaths > hit.Births && v.UnrealizedShareOfBudget > 0.25:
		v.Verdict = verdictDesignDefect
		v.Rationale = "conservation green with starvation deaths exceeding births and large unrealized regen: production/land mis-specified per the frozen table"
	case conservationGreen && v.PopulationFlat && v.FiberWakeExplosion:
		v.Verdict = verdictRuntime
		v.Rationale = "flat population with fiber/wake explosion: wake storm per the frozen table"
	default:
		v.Verdict = verdictNoBranchFired
		v.Rationale = fmt.Sprintf("no frozen branch fired verbatim: conservation green=%t, population bar (≥0.9·N*=%d)=%t (ratio %.3f), turnover births=%d deaths=%d (holds=%t, zero-turnover is the recorded poverty trap, not healthy plateau turnover), starvation>births=%t, unrealized share %.3f, flat=%t, fiber/wake explosion=%t, replay compared=%t", conservationGreen, v.CarryingCapacityEstimate, populationBar, v.PopulationRatioOfEstimate, hit.Births, hit.Deaths, v.TurnoverHolds, v.StarvationExceedsBirths, v.UnrealizedShareOfBudget, v.PopulationFlat, v.FiberWakeExplosion, replayIdentical != nil)
	}
	return v
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// bandFor evaluates the frozen band of the cell verbatim. A falsified band
// is reported, never tuned and never fatal.
func bandFor(o options, br *branchReport) bandReport {
	pop := br.Population
	last := len(pop.AliveByHour) - 1
	switch {
	case br.CultivationDisabled:
		within := br.Events.Builds == 0 && br.Events.WorldEvents["build"] == 0
		return bandReport{Name: "D: cultivation-disabled ablation", Criterion: "zero build events and zero build attempts anywhere; any build event = defect",
			Observed:   fmt.Sprintf("build attempts=%d, rule-505 events=%d", br.Events.Builds, br.Events.WorldEvents["build"]),
			WithinBand: within}
	case o.r == 0:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= 12 && (last < 12 || pop.AliveByHour[12] == 0) && br.Totals.Births == 0
		return bandReport{Name: "B: R=0 extinction", Criterion: "extinction by h12 with zero births",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, alive(final)=%d", pop.ExtinctionHour, br.Totals.Births, pop.FinalAlive),
			WithinBand: within}
	case br.Fixture:
		changed := br.Totals.Births > 0 || pop.FinalAlive != 0
		return bandReport{Name: "E: founder-tenure foothold", Criterion: "whether an initial tenure foothold (k=2, granary=3 at the gate margins for founders 1-2) changes the organic outcome at all: births committed and the trajectory leaves the all-wild poverty trap",
			Observed:   fmt.Sprintf("births=%d, final_alive=%d, extinction_hour=%d, first_birth_hour=%d", br.Totals.Births, pop.FinalAlive, pop.ExtinctionHour, firstBirthHour(pop, br.FirstHour)),
			WithinBand: changed}
	case !br.Births:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= 143
		return bandReport{Name: "C: births-off aging decline", Criterion: "aging decline to 0 by h143",
			Observed:   fmt.Sprintf("extinction_hour=%d, alive(final)=%d", pop.ExtinctionHour, pop.FinalAlive),
			WithinBand: within}
	case br.HaltedByCeilingHit:
		within := br.CeilingHit != nil && br.CeilingHit.Verdict.Verdict == verdictOutgrew
		verdict := "none"
		if br.CeilingHit != nil {
			verdict = br.CeilingHit.Verdict.Verdict
		}
		return bandReport{Name: "F: compute-ceiling verdict", Criterion: "green conservation to the hit hour ∧ population ≥ 0.9·(carrying capacity) ⇒ outgrew-harness",
			Observed:   fmt.Sprintf("ceiling=%s hour=%d verdict=%s", br.CeilingHit.Ceiling, br.CeilingHit.Hour, verdict),
			WithinBand: within}
	default:
		if br.CompletedHour != world.NaturalHorizonHours {
			return bandReport{Name: "A: resource saturation (organic births-on R=8)", Criterion: "alive(168) ∈ [8,18] ∧ |alive(168) − alive(120)| ≤ 6",
				Observed:   fmt.Sprintf("not evaluable below h168 (completed_hour=%d)", br.CompletedHour),
				WithinBand: false}
		}
		alive168, alive120 := pop.AliveByHour[168-br.FirstHour], pop.AliveByHour[120-br.FirstHour]
		within := alive168 >= 8 && alive168 <= 18 && abs64(int64(alive168)-int64(alive120)) <= 6
		return bandReport{Name: "A: resource saturation (organic births-on R=8)", Criterion: "alive(168) ∈ [8,18] ∧ |alive(168) − alive(120)| ≤ 6",
			Observed:   fmt.Sprintf("alive(168)=%d, alive(120)=%d, |Δ|=%d — predicted FALSIFIED by the recorded runner-lane poverty-trap finding (extinction ~h143, zero organic births)", alive168, alive120, abs64(int64(alive168)-int64(alive120))),
			WithinBand: within}
	}
}

func firstBirthHour(pop populationReport, firstHour int) int {
	for h, n := range pop.BirthsByHour {
		if n > 0 {
			return firstHour + h
		}
	}
	return -1
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.NaturalHandoff) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.NaturalRegistry()
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
	br.Events.WorldEvents = make(map[string]int64)
	byID := make(map[sim.EventID]kernel.Event, len(events))
	for _, ev := range events {
		body, err := ev.Bytes()
		if err != nil {
			return nil, err
		}
		br.EventBodyBytes += len(body)
		byID[ev.ID] = ev
		name, ok := worldRuleNames[ev.Rule]
		if !ok {
			return nil, fmt.Errorf("unknown natural rule %d in accepted history", ev.Rule)
		}
		br.Events.WorldEvents[name]++
	}
	return byID, nil
}

// classifyJournal classifies every typed journal attempt and, when the
// accepted history is available, verifies keyed attempts against it. After a
// typed-halt halt the link set is not checkable (the fail-loud halt refuses
// accepted-history export), so kinds are still classified but links are
// reported as unchecked with the post-halt note.
func classifyJournal(br *branchReport, batches []world.NaturalBatch, byID map[sim.EventID]kernel.Event) error {
	br.Events.Refusals = map[string]int64{}
	if br.Events.WorldEvents == nil {
		br.Events.WorldEvents = map[string]int64{}
	}
	for _, batch := range batches {
		for _, a := range batch.Attempts {
			if a.Key != "" && a.Lost == "" {
				if byID == nil {
					// Post-halt: the link target set is unavailable, not
					// broken. Counted as neither checked nor broken.
				} else {
					ev, ok := byID[a.EventID]
					if !ok || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor {
						return fmt.Errorf("broken accepted event link: actor %d kind %s at %d", a.Actor, a.Kind, a.Time)
					}
					br.AcceptedLinksChecked++
				}
			} else if a.Key != "" && a.Lost != "" {
				// The one legitimate keyed keyless case: a birth that lost its
				// kernel batch to a newborn-ID collision stages nothing
				// structurally and carries the typed lost note, no event.
				if a.Accepted || a.EventID != 0 {
					return fmt.Errorf("malformed lost-birth evidence: actor %d kind %s", a.Actor, a.Kind)
				}
			} else if a.EventID != 0 {
				return fmt.Errorf("rejected attempt linked: actor %d kind %s", a.Actor, a.Kind)
			} else {
				br.UnlinkedRejections++
			}
			switch a.Kind {
			case "gather":
				br.Events.GatherClaims++
				if a.Rejection == world.FoodFlowGatherAdmitted {
					br.Events.GatherAdmitted++
				} else {
					br.Events.GatherDenied++
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
			case "build-denied-eat":
				br.Events.BuildDenialEats++
			case "paired-meal":
				br.Events.PairedMeals++
			case "fallback-eatstored":
				br.Events.FallbackEatStored++
			case "birth-request":
				br.Events.BirthRequests++
			case "birth-reply":
				br.Events.BirthReplies++
				switch a.Birth.Decision {
				case world.NaturalReplyAccept:
					br.Events.BirthsAccepted++
				case world.NaturalReplyRefuse:
					br.Events.BirthRefusals++
				case world.NaturalReplyRefuseSilent:
					br.Events.BirthRefusalsSilent++
				default:
					return fmt.Errorf("unknown birth reply decision %d", a.Birth.Decision)
				}
			default:
				return fmt.Errorf("unknown journal attempt kind %q", a.Kind)
			}
			if a.BirthWake {
				br.Events.BirthWakeClaims++
			}
			if a.Lost != "" {
				br.Events.LostBirths++
			}
		}
	}
	return nil
}

// populationEvidence assembles the frozen trajectory with per-hour deltas.
func populationEvidence(br *branchReport, checks []world.NaturalCheckpoint) error {
	pop := &br.Population
	pop.AliveByHour = make([]int, len(checks))
	pop.BirthsByHour = make([]int64, len(checks))
	pop.DeathsStarvationByHour = make([]int64, len(checks))
	pop.DeathsAgeByHour = make([]int64, len(checks))
	pop.ExtinctionHour = -1
	br.Regen.CultivatedByHour = make([]int64, len(checks))
	br.Regen.WildByHour = make([]int64, len(checks))
	br.Regen.UnrealizedByHour = make([]int64, len(checks))
	br.Regen.CultivatedSlotsTotalByHour = make([]int, len(checks))
	br.Granary.InflowByHour = make([]int64, len(checks))
	br.Granary.SpoiledByHour = make([]int64, len(checks))
	br.Granary.StoredByHour = make([]int64, len(checks))
	br.Malthus.IndexByHour = make([]int64, len(checks))
	br.Malthus.Definition = "trailing-24h Σmeals − Σbasal: meals = Δ(consumed + stored meals), basal = Δ(basal spent); windows shorter than 24 h at the start"
	for h, c := range checks {
		if c.Hour != br.FirstHour+h {
			return fmt.Errorf("hour sequence break at index %d (hour %d, want %d)", h, c.Hour, br.FirstHour+h)
		}
		pop.AliveByHour[h] = c.Metrics.Alive
		if h == 0 {
			pop.BirthsByHour[h] = c.Metrics.Births
			pop.DeathsStarvationByHour[h] = c.Metrics.DeathsStarvation
			pop.DeathsAgeByHour[h] = c.Metrics.DeathsAge
			br.Regen.CultivatedByHour[h] = c.Metrics.RegenCultivated
			br.Regen.WildByHour[h] = c.Metrics.RegenWild
			br.Regen.UnrealizedByHour[h] = c.Metrics.RegenUnrealized
			br.Granary.InflowByHour[h] = c.Balance.CultivatedTotal
			br.Granary.SpoiledByHour[h] = c.Balance.Spoiled
			br.Granary.StoredByHour[h] = c.Balance.StoredMeals
		} else {
			prev := checks[h-1]
			pop.BirthsByHour[h] = c.Metrics.Births - prev.Metrics.Births
			pop.DeathsStarvationByHour[h] = c.Metrics.DeathsStarvation - prev.Metrics.DeathsStarvation
			pop.DeathsAgeByHour[h] = c.Metrics.DeathsAge - prev.Metrics.DeathsAge
			br.Regen.CultivatedByHour[h] = c.Metrics.RegenCultivated
			br.Regen.WildByHour[h] = c.Metrics.RegenWild
			br.Regen.UnrealizedByHour[h] = c.Metrics.RegenUnrealized
			br.Granary.InflowByHour[h] = c.Balance.CultivatedTotal - prev.Balance.CultivatedTotal
			br.Granary.SpoiledByHour[h] = c.Balance.Spoiled - prev.Balance.Spoiled
			br.Granary.StoredByHour[h] = c.Balance.StoredMeals - prev.Balance.StoredMeals
		}
		if pop.AliveByHour[h] == 0 && pop.ExtinctionHour < 0 {
			pop.ExtinctionHour = c.Hour
		}
		tenured := 0
		for _, slots := range c.Slots {
			for _, slot := range slots {
				if slot.Mode == world.NaturalTenureCultivated {
					tenured++
				}
			}
		}
		br.Regen.CultivatedSlotsTotalByHour[h] = tenured
		// Independent frozen G1/G2a/G2b/G3 re-verification at every hour.
		balance, err := world.NaturalCheckConservation(len(c.Patches), c.Patches, c.Slots, c.Founders, c.Newborns, c.AllocatorLast)
		if err != nil || balance != c.Balance {
			return fmt.Errorf("hour %d conservation failure: %v", c.Hour, err)
		}
		// Trailing-24h Malthus index from the cumulative balance deltas.
		var meals, basal int64
		for k := h; k >= 0 && (h-k) < 24; k-- {
			if k == 0 {
				meals += checks[0].Balance.Consumed + checks[0].Balance.StoredMeals
				basal += checks[0].Balance.BasalSpent
				break
			}
			meals += (checks[k].Balance.Consumed + checks[k].Balance.StoredMeals) - (checks[k-1].Balance.Consumed + checks[k-1].Balance.StoredMeals)
			basal += checks[k].Balance.BasalSpent - checks[k-1].Balance.BasalSpent
		}
		br.Malthus.IndexByHour[h] = meals - basal
	}
	br.ConservationVerified = true
	if pop.ExtinctionHour >= 0 {
		for h := pop.ExtinctionHour - br.FirstHour; h < len(pop.AliveByHour); h++ {
			if pop.AliveByHour[h] != 0 {
				return fmt.Errorf("population resurrected after extinction at h%d", h)
			}
		}
	}
	return nil
}

// finalEvidence assembles the final-hour totals, per-actor ledgers and
// survivor sets from the last available hourly checkpoint.
func finalEvidence(br *branchReport, checks []world.NaturalCheckpoint) {
	final := checks[len(checks)-1]
	b := final.Balance
	br.Totals = totalsReport{
		Alive: final.Metrics.Alive, Produced: b.Produced, Granted: b.Granted, Unrealized: b.Unrealized,
		Gathered: b.Gathered, Consumed: b.Consumed, Stock: b.Stock, Held: b.Held, Invested: b.Invested,
		CultivatedTotal: b.CultivatedTotal, GranaryStock: b.GranaryStock, StoredMeals: b.StoredMeals, Spoiled: b.Spoiled,
		Capital: b.Capital, Wip: b.Wip, PointsCreated: b.PointsCreated, PointsDecayed: b.PointsDecayed,
		TenuredSlots: b.TenuredSlots, InitialEnergy: b.InitialEnergy, Energy: b.Energy,
		BasalSpent: b.BasalSpent, EnergyCapLost: b.EnergyCapLost, BirthGranaryPaid: b.BirthGranaryPaid,
		Births: b.Births, BirthRequests: final.Metrics.BirthRequests, Divergences: final.Metrics.Divergences,
	}
	// Tenure counts over the final slot state (privileged runner view).
	tenure := make(map[sim.EntityID]int)
	br.Regen.CultivatedSlotsPerPatchFinal = make([]int, len(final.Slots))
	for p, slots := range final.Slots {
		for _, slot := range slots {
			if slot.Mode == world.NaturalTenureCultivated {
				br.Regen.CultivatedSlotsPerPatchFinal[p]++
				tenure[slot.Owner]++
			}
		}
	}
	pop := &br.Population
	pop.SurvivorIDs = []int{}
	add := func(id sim.EntityID, a world.NaturalActorState) {
		br.Actors = append(br.Actors, actorFinalFrom(id, a, tenure[id]))
		if naturalAlive(a) {
			pop.SurvivorIDs = append(pop.SurvivorIDs, int(id))
		}
	}
	for i := range final.Founders {
		add(sim.EntityID(i+1), final.Founders[i])
	}
	for _, nb := range final.Newborns {
		add(nb.ID, nb.State)
	}
	pop.FinalAlive = final.Metrics.Alive
	br.Regen.TotalCultivated = sumHours(br.Regen.CultivatedByHour)
	br.Regen.TotalWild = sumHours(br.Regen.WildByHour)
	br.Regen.TotalUnrealized = sumHours(br.Regen.UnrealizedByHour)
	br.Granary.InflowTotal = b.CultivatedTotal
	br.Granary.SpoiledTotal = b.Spoiled
	br.Granary.StoredMealsTotal = b.StoredMeals
	br.Granary.StockFinal = b.GranaryStock
	var spoilDebt int64
	for i := range final.Founders {
		spoilDebt += final.Founders[i].Granary.SpoilDebt
	}
	for _, nb := range final.Newborns {
		spoilDebt += nb.State.Granary.SpoilDebt
	}
	br.Granary.SpoilDebtFinal = spoilDebt
	br.Malthus.Final = br.Malthus.IndexByHour[len(br.Malthus.IndexByHour)-1]
}

func sumHours(v []int64) int64 {
	var total int64
	for _, x := range v {
		total += x
	}
	return total
}

func actorFinalFrom(id sim.EntityID, a world.NaturalActorState, cultivatedSlots int) actorFinal {
	return actorFinal{
		ID: int(id), Alive: naturalAlive(a), HomePatch: int(a.Bag.Source),
		Energy: a.Body.Energy, Hunger: a.Body.Hunger,
		BasalSpent: a.Body.BasalSpent, EnergyCapLost: a.Body.CapLost,
		WildMeals: a.Body.Consumed, LastGatherHour: a.Body.LastGatherHour,
		BagUnits: a.Bag.Units,
		Capital:  a.Worksite.Capital, Wip: a.Worksite.Wip,
		WearDebt: a.Worksite.WearDebt, InvestedUnits: a.Worksite.InvestedUnits,
		PointsCreated: a.Worksite.PointsCreated, PointsDecayed: a.Worksite.PointsDecayed,
		LastBuildHour: a.Worksite.LastBuildHour,
		GranaryStock:  a.Granary.Stock, CultivatedTotal: a.Granary.CultivatedTotal,
		StoredMeals: a.Granary.StoredMeals, LastStoredMealHour: a.Granary.LastStoredMealHour,
		SpoilDebt: a.Granary.SpoilDebt, Spoiled: a.Granary.Spoiled,
		CultivatedSlots: cultivatedSlots,
		LocusM:          a.Genome.LocusM, LocusY: a.Genome.LocusY, LocusW: a.Genome.LocusW,
		ParentA: int(a.Genome.ParentA), ParentB: int(a.Genome.ParentB),
		BirthHour: a.Genome.BirthHour, DeathHour: a.Genome.DeathHour, DiedHour: a.Genome.DiedHour,
		BirthGranaryPaid: a.Genome.BirthGranaryPaid,
	}
}

// ledgerRows quotes the frozen headroom ledger of a checkpoint verbatim.
func ledgerRows(rows []world.NaturalCeilingRow) []ceilingRowReport {
	out := make([]ceilingRowReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, ceilingRowReport{Ceiling: string(row.Ceiling), Threshold: row.Threshold, Observed: row.Observed,
			RateLastHour: row.RateLastHour, ProjectedBindingHour: row.ProjectedBindingHour})
	}
	return out
}

// stepToHorizon continues a restored runner to the requested absolute hour,
// treating a typed NaturalCeilingHit as a first-class halt.
func stepToHorizon(ctx context.Context, f *world.Natural, hours int) (world.NaturalCeilingHit, bool, error) {
	for {
		checks := f.Checkpoints()
		if len(checks) > 0 && checks[len(checks)-1].Hour >= hours {
			return world.NaturalCeilingHit{}, false, nil
		}
		ok, err := f.Step(ctx)
		if err != nil {
			var hitErr *world.NaturalCeilingHitError
			if errors.As(err, &hitErr) {
				return hitErr.Hit, true, nil
			}
			return world.NaturalCeilingHit{}, false, err
		}
		if !ok {
			return world.NaturalCeilingHit{}, false, errors.New("runner stopped before the requested hour")
		}
	}
}

// runBranch restores one recorded child, continues it to the requested
// horizon (or the typed ceiling halt), and assembles the branch report. The
// restore, run, verification and serialization all sit inside the timing.
func runBranch(ctx context.Context, o options, births bool, parentDigest string, childPath string, childDigest [32]byte, childBytes int64, firstHour int, fixture bool) (branchReport, error) {
	br := branchReport{Births: births, Fixture: fixture, CultivationDisabled: o.cultivationDisabled,
		ParentDigest: parentDigest, BranchDigest: digestString(childDigest), FirstHour: firstHour,
		BranchCheckpoint: checkpointInfo{Name: filepath.Base(childPath), Digest: digestString(childDigest), Bytes: childBytes},
		TokenCost:        0}
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return br, err
	}
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Seed: o.seed, Workers: o.workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled}
	f, restored, err := world.RestoreNaturalCheckpoint(childPath, expected)
	if err != nil {
		return br, err
	}
	if restored != childDigest {
		return br, errors.New("restored child digest differs from the published child digest")
	}
	hit, halted, err := stepToHorizon(ctx, f, o.hours)
	if err != nil {
		return br, err
	}
	br.HaltedByCeilingHit = halted
	checks := f.Checkpoints()
	if len(checks) == 0 {
		return br, errors.New("no hourly projection")
	}
	if !halted && checks[len(checks)-1].Hour != o.hours {
		return br, fmt.Errorf("missing hourly projection: last hour %d, want %d", checks[len(checks)-1].Hour, o.hours)
	}
	br.CompletedHour = checks[len(checks)-1].Hour
	journalBytes, err := publishedJournalBytes(childPath)
	if err != nil {
		return br, err
	}
	br.JournalBytes = journalBytes
	if err := populationEvidence(&br, checks); err != nil {
		return br, err
	}
	finalEvidence(&br, checks)
	br.CeilingLedger = ledgerRows(checks[len(checks)-1].Ledger)
	if halted {
		// Post-halt the accepted-history export is refused, so the journal is
		// classified without link verification (byID nil).
		if err := classifyJournal(&br, f.Journal(), nil); err != nil {
			return br, err
		}
		hr := ceilingHitReport{
			Ceiling: string(hit.Ceiling), Threshold: hit.Threshold, Observed: hit.Observed, Hour: hit.Hour,
			WorldVersion: uint64(hit.WorldVersion), Alive: hit.Alive, Births: hit.Births, Deaths: hit.Deaths,
			Last24hInflow: hit.Last24hInflow, Last24hConsumption: hit.Last24hConsumption, RegenUnrealized: hit.RegenUnrealized,
		}
		// Replay equality twin: the same recorded child at the opposite
		// worker count, truncated at the hit hour.
		twinWorkers := 4
		if o.workers == 4 {
			twinWorkers = 1
		}
		twin, err := replayTwin(ctx, o, childPath, births, fixture, twinWorkers, hit.Hour, checks, f.Journal())
		if err != nil {
			return br, err
		}
		hr.Verdict = classifyVerdict(o, hit, br.ConservationVerified, &twin.Identical, checks)
		note := ""
		if twin.Note != "" {
			note = fmt.Sprintf(" (%s)", twin.Note)
		}
		hr.Verdict.ReplayNote = fmt.Sprintf("workers-%d twin truncated at h%d: %d checkpoint hours and %d journal batches compared%s", twinWorkers, hit.Hour, twin.HoursCompared, twin.JournalCompared, note)
		br.CeilingHit = &hr
		br.Events.WorldEventsNote = evidencePostHaltNote
	} else {
		handoff, err := f.Handoff()
		if err != nil {
			return br, err
		}
		br.HistoryBytes = len(handoff.History)
		byID, err := historyEvidence(&br, handoff)
		if err != nil {
			return br, err
		}
		if err := classifyJournal(&br, f.Journal(), byID); err != nil {
			return br, err
		}
	}
	if err := classifyJournalRefusals(&br, checks); err != nil {
		return br, err
	}
	if err := disciplineChecks(o, &br); err != nil {
		return br, err
	}
	cpuEnd, rss, err := cpuTime()
	if err != nil {
		return br, err
	}
	br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes = time.Since(start).Nanoseconds(), (cpuEnd - cpuStart).Nanoseconds(), rss
	br.Band = bandFor(o, &br)
	br.CostBudget = costBudget(o, &br)
	return br, nil
}

// classifyJournalRefusals carries the cumulative typed refusal histogram.
func classifyJournalRefusals(br *branchReport, checks []world.NaturalCheckpoint) error {
	if len(checks) == 0 {
		return nil
	}
	final := checks[len(checks)-1].Metrics
	if br.Events.Refusals == nil {
		br.Events.Refusals = map[string]int64{}
	}
	for reason, n := range final.Refusals {
		name, err := refusalName(reason)
		if err != nil {
			return err
		}
		br.Events.Refusals[name] = n
	}
	return nil
}

// replayTwin restores the same recorded child at a different worker count,
// truncates at the hit hour, and compares the hourly projections and the
// journal prefix for exact equality (timing-free replay evidence).
func replayTwin(ctx context.Context, o options, bundlePath string, births, fixture bool, workers, hitHour int, checks []world.NaturalCheckpoint, journal []world.NaturalBatch) (replayTwinReport, error) {
	twin := replayTwinReport{Workers: workers, HitHour: hitHour, Identical: true}
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Seed: o.seed, Workers: workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled}
	f, _, err := world.RestoreNaturalCheckpoint(bundlePath, expected)
	if err != nil {
		return twin, err
	}
	for {
		tc := f.Checkpoints()
		if len(tc) > 0 && tc[len(tc)-1].Hour >= hitHour {
			break
		}
		ok, err := f.Step(ctx)
		if err != nil {
			var hitErr *world.NaturalCeilingHitError
			if errors.As(err, &hitErr) {
				break
			}
			return twin, err
		}
		if !ok {
			break
		}
	}
	twinChecks := f.Checkpoints()
	twinJournal := f.Journal()
	hourTime, err := world.NaturalHourTime(hitHour)
	if err != nil {
		return twin, err
	}
	common := len(checks)
	if len(twinChecks) < common {
		common = len(twinChecks)
	}
	for h := 0; h < common; h++ {
		if !reflect.DeepEqual(checks[h], twinChecks[h]) {
			twin.Identical = false
			twin.Divergences++
		}
	}
	twin.HoursCompared = common
	var mine, theirs []world.NaturalBatch
	for _, b := range journal {
		if b.Time < hourTime {
			mine = append(mine, b)
		}
	}
	for _, b := range twinJournal {
		if b.Time < hourTime {
			theirs = append(theirs, b)
		}
	}
	n := len(mine)
	if len(theirs) < n {
		n = len(theirs)
	}
	for i := 0; i < n; i++ {
		if !reflect.DeepEqual(mine[i], theirs[i]) {
			twin.Identical = false
			twin.Divergences++
		}
	}
	twin.JournalCompared = n
	if len(mine) != len(theirs) && twin.Identical {
		twin.Note = fmt.Sprintf("journal lengths differ under the hit-hour window (w1 %d, twin %d); compared the common prefix", len(mine), len(theirs))
	}
	return twin, nil
}

// disciplineChecks enforces the frozen two-flag and ablation discipline: a
// births-disabled branch records no birth evidence anywhere, a
// cultivation-ablation cell records no build evidence anywhere, and an R=0
// cell commits no births at all.
func disciplineChecks(o options, br *branchReport) error {
	if !br.Births && (br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 ||
		br.Totals.Births != 0 || br.Events.BirthsAccepted != 0) {
		return errors.New("births-disabled branch recorded birth evidence")
	}
	if br.CultivationDisabled && (br.Events.Builds != 0 || br.Events.WorldEvents["build"] != 0) {
		return errors.New("cultivation-ablation cell recorded build evidence")
	}
	if o.r == 0 && (br.Totals.Births != 0 || br.Events.BirthsAccepted != 0) {
		return errors.New("R=0 cell committed births")
	}
	return nil
}

// costBudget measures the branch against the predeclared v4-relative
// budgets; misses are reported verbatim inside the branch report.
func costBudget(o options, br *branchReport) costBudgetReport {
	hours := br.CompletedHour - br.FirstHour + 1
	if hours < 1 {
		hours = 1
	}
	cpuPerHour := float64(br.CPUTimeNS) / 1e9 / float64(hours)
	cpuBudget := cpuBudgetFactor * v4RecordedCPUSecondsPerSimulatedHour
	// A fail-loud ceiling halt refuses accepted-history export, so the branch
	// HistoryBytes is zero and a naive comparison would report the budget as
	// satisfied vacuously. Record the bytes budget as not evaluable in that
	// case and point at the ceiling ledger, whose trailing rate is the honest
	// measurement.
	bytesEvaluable := !br.HaltedByCeilingHit
	historyBytesPerHour := float64(br.HistoryBytes) / float64(hours)
	bytesBudget := bytesBudgetFactor * float64(v4RecordedHistoryBytes) / float64(v4RecordedHistoryHours)
	bytesWithinBudget := bytesEvaluable && historyBytesPerHour <= bytesBudget
	var actorHours int64
	for _, alive := range br.Population.AliveByHour {
		actorHours += int64(alive)
	}
	totalEvidenceBytes := int64(br.HistoryBytes) + int64(br.EventBodyBytes) + br.JournalBytes + br.BranchCheckpoint.Bytes
	var bytesPerActorHour, cpuPerActorHour float64
	if actorHours > 0 {
		bytesPerActorHour = float64(totalEvidenceBytes) / float64(actorHours)
		cpuPerActorHour = float64(br.CPUTimeNS) / 1e9 / float64(actorHours)
	}
	return costBudgetReport{
		CPUPerSimulatedHour: cpuPerHour, CPUBudget: cpuBudget, CPUWithinBudget: cpuPerHour <= cpuBudget,
		HistoryBytesPerHour: historyBytesPerHour, BytesBudget: bytesBudget, BytesWithinBudget: bytesWithinBudget, BytesEvaluable: bytesEvaluable,
		ActorHours: actorHours, BytesPerActorHour: bytesPerActorHour, CPUPerActorHour: cpuPerActorHour,
		ExtrapolationLabel: extrapolationOnlyLabel,
	}
}

func run(ctx context.Context, o options) (report, error) {
	r := report{
		Status:   "synthetic/model-conditional; frozen natural-limits v5 fixture; no historical calibration, takeoff, or P0 scale claim",
		Revision: "natural format-v5/schema-v5/rule-v5/projection-v5/policy natural/5",
		Patches:  o.patches, R: o.r, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: "natural/5", BranchSelection: o.births, FounderTenure: o.fixture,
		CultivationDisabled: o.cultivationDisabled, TokenCost: 0,
	}
	founders := 8 * o.patches
	r.Founders = founders
	if o.workers < 1 || o.patches < 1 || o.patches > world.NaturalMaxPatchesFixedIDSpace ||
		o.r < 0 || o.r > world.NaturalSlotsPerPatch || o.hours < 1 || o.hours > world.NaturalHorizonHours {
		return r, errors.New("invalid fixture flags")
	}
	switch o.births {
	case enabledBranchFlag, disabledBranchFlag, bothBranchesFlag:
	default:
		return r, fmt.Errorf("invalid -births value %q", o.births)
	}
	if o.fixture && o.cultivationDisabled {
		return r, errors.New("the founder-tenure fixture and the cultivation ablation are never a declared combination")
	}
	if o.fixture && o.births != enabledBranchFlag {
		return r, fmt.Errorf("founder-tenure cells are births-enabled-only; refused -births=%q", o.births)
	}
	if o.cultivationDisabled && o.births != disabledBranchFlag {
		return r, fmt.Errorf("cultivation-ablation cells are births-disabled-only in the frozen matrix; refused -births=%q", o.births)
	}
	if o.fixture && o.hours < 4 {
		return r, errors.New("fixture cells continue from the first publishable pulse boundary (h3); -hours must be at least 4")
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
	var rootPath, parent string
	var rootDigest [32]byte
	var rootBytes int64
	firstHour := 0
	if o.fixture {
		// The declared founder-tenure fixture genesis: births-capable by
		// construction, first pulse at h2, first publishable pulse boundary
		// at h3. The fixture is run identity (journal config and manifest),
		// so every continuation of this root is pinned to it.
		fx, err := world.NewNaturalFixture(o.patches, o.r, o.seed, true, 1)
		if err != nil {
			return r, err
		}
		for {
			checks := fx.Checkpoints()
			if len(checks) >= 2 {
				break
			}
			ok, err := fx.Step(ctx)
			if err != nil || !ok {
				return r, fmt.Errorf("fixture pre-boundary pulses: processed=%t err=%v", ok, err)
			}
		}
		checks := fx.Checkpoints()
		firstHour = checks[0].Hour
		rootPath = filepath.Join(dir, fixtureRootBundleName)
		rootDigest, err = fx.SaveResumeCheckpoint(rootPath)
		if err != nil {
			return r, err
		}
		r.Root = rootReport{Kind: rootKindFixture, FirstHour: firstHour, Name: fixtureRootBundleName, Digest: digestString(rootDigest)}
		parent = digestString(rootDigest)
	} else {
		// The neutral h0 pulse is captured with BIRTHS DISABLED: the v5 h0
		// pulse is births-asymmetric by frozen design, so a disabled root is
		// the only legal branch parent and every births-enabled cell is the
		// recorded enable intervention branching from this same root.
		root, err := world.NewNatural(world.NaturalOptions{Patches: o.patches, R: o.r, Seed: o.seed,
			Workers: 1, Births: false, CultivationDisabled: o.cultivationDisabled})
		if err != nil {
			return r, err
		}
		if ok, err := root.Step(ctx); err != nil || !ok {
			return r, fmt.Errorf("neutral h0 pulse: processed=%t err=%v", ok, err)
		}
		rootPath = filepath.Join(dir, organicRootBundleName)
		rootDigest, err = root.SaveCheckpoint(rootPath)
		if err != nil {
			return r, err
		}
		r.Root = rootReport{Kind: rootKindNeutral, FirstHour: 0, Name: organicRootBundleName, Digest: digestString(rootDigest)}
		parent = digestString(rootDigest)
	}
	stat, err := os.Stat(rootPath)
	if err != nil {
		return r, err
	}
	rootBytes = stat.Size()
	r.Root.Bytes = rootBytes
	type branchPlan struct {
		births bool
		name   string
	}
	var plans []branchPlan
	if o.fixture {
		plans = []branchPlan{{births: true, name: fixtureRootBundleName}}
	} else {
		switch o.births {
		case enabledBranchFlag:
			plans = []branchPlan{{births: true, name: enabledBundleName}}
		case disabledBranchFlag:
			plans = []branchPlan{{births: false, name: disabledBundleName}}
		default:
			plans = []branchPlan{{births: true, name: enabledBundleName}, {births: false, name: disabledBundleName}}
		}
	}
	for _, plan := range plans {
		childPath := filepath.Join(dir, plan.name)
		digest := rootDigest
		childBytes := rootBytes
		if !o.fixture {
			expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Seed: o.seed, Workers: 1,
				Births: false, CultivationDisabled: o.cultivationDisabled}
			digest, err = world.BranchNaturalCheckpoint(rootPath, expected, plan.births, childPath)
			if err != nil {
				return r, err
			}
			stat, err := os.Stat(childPath)
			if err != nil {
				return r, err
			}
			childBytes = stat.Size()
		}
		br, err := runBranch(ctx, o, plan.births, parent, childPath, digest, childBytes, firstHour, o.fixture)
		if err != nil {
			return r, err
		}
		r.Branches = append(r.Branches, br)
	}
	if len(r.Branches) == 0 {
		return r, fmt.Errorf("no branch selected for %q", o.births)
	}
	r.CompletedHour = 0
	for _, br := range r.Branches {
		if br.CompletedHour > r.CompletedHour {
			r.CompletedHour = br.CompletedHour
		}
	}
	cpuEnd, rss, err := cpuTime()
	if err != nil {
		return r, err
	}
	r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes = time.Since(start).Nanoseconds(), (cpuEnd - cpuStart).Nanoseconds(), rss
	return r, nil
}

func main() {
	patches := flag.Int("patches", world.NaturalDefaultPatches, "patch count P (founders = 8P; 1..125)")
	r := flag.Int64("r", 8, "patch regeneration budget per patch-hour (0..8)")
	seed := flag.Uint64("seed", 0, "deterministic allocation seed")
	workers := flag.Int("workers", 1, "scheduler worker count for every branch continuation")
	hours := flag.Int("hours", world.NaturalHorizonHours, "absolute hourly horizon per branch (1..168)")
	births := flag.String("births", bothBranchesFlag, "which recorded intervention to run: enabled, disabled, or both (children always branch from the births-disabled neutral root)")
	fixture := flag.Bool("founder-tenure", false, "declared founder-tenure fixture genesis (births-enabled-only; the only genesis under which organic births are reachable)")
	cultivationDisabled := flag.Bool("cultivation-disabled", false, "declared cultivation ablation (births-disabled-only): the frozen policy is unchanged; any build event is a defect")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the root and branch bundles; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	rep, err := run(context.Background(), options{patches: *patches, r: *r, seed: *seed, workers: *workers, hours: *hours,
		births: *births, fixture: *fixture, cultivationDisabled: *cultivationDisabled, out: *out, checkpointDir: *checkpointDir})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(rep, "", "  ")
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
