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
// The optional -policy flag (ticket 16 D1 policy-selection seam) selects the
// frozen behaviour ref: natural/5 (the default, byte-identical to every
// recorded pre-ticket-16 run) or natural/6, the surplus behaviour policy of
// .scratch/16-surplus-behaviour/spec.md over the unchanged v5 world. Ticket 17
// extends the frozen ref set with natural/7 (the provision candidate) and
// natural/7b (the mechanism-necessity ablation, ref {natural,8}), both of
// which require the -mechanism v6 world (the rule-515 "provision" mechanism
// axis of .scratch/17-surplus-mechanism/spec.md; natural/5 and natural/6 are
// legal under either mechanism, and the zero mechanism value is exactly v5 —
// the D1 default-preservation contract, held at the CLI layer too). The policy
// ref and the mechanism are run identity end to end (journal config,
// checkpoint manifest, bidirectional mismatch rejection). Under natural/6 the
// reporter adds, purely additively, the predeclared ticket-16 metric sections:
// per-actor trailing-24h surplus flow (SF) with its sustained-hours
// counter and the ecology time-to-first-sustained-surplus, capital
// persistence (share of alive hours with k≥1, Σk by hour, slot turnover),
// time-to-first build choice/committed build/slot tenure/birth
// filing/birth, and parent deaths within 24h of a birth payment — plus the
// predeclared v6 outcome bands (A2 surplus, A3 births, A4 descriptive
// dynamics, and the B/C/D/E/F cell bands of the frozen design). Every
// natural/5 field and behaviour stays byte-compatible: a natural/5 report
// carries none of the new keys.
//
// Under natural/7 or natural/7b the reporter instead adds the predeclared
// ticket-17 v7 sections, purely additively and never under natural/5 or
// natural/6: SF′ (the SF form with provisioned draws disclosed as
// withdrawals), the GATING per-actor sustained-12h surplus counter (the same
// actor holds the full conjunction k≥2 ∧ granary≥1 ∧ SF′>0 for 12 consecutive
// hours; entries carry the true running maximum) alongside the non-gating
// hour-level union counter labelled as ticket 16's reading, provisioned
// totals per branch and per actor, paired meals per cell (the workhouse
// falsifier), time-to-first provision/banked hour/k≥2 hold/sustained
// surplus/birth filing/birth, the capital-persistence, slot-turnover,
// cause-attributed death and parent-death-within-24h blocks, the
// non-universality evidence of band V7-A5, and the predeclared v7 bands
// (V7-A2/A3/A4/A5 and V7-B/C/D/E/F). A falsified band is reported verbatim,
// never tuned.
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
	"sort"
	"strings"
	"syscall"
	"time"

	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
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
	policy5Name            = "natural/5"
	policy6Name            = "natural/6"
	policy7Name            = "natural/7"
	policy7bName           = "natural/7b"
	mechanismV5Name        = "v5"
	mechanismV6Name        = "v6"
)

// The predeclared ticket-16 v6 gate constants (frozen design, quoted): the
// A2 surplus band (≥12 consecutive surplus-bearing hours, first such hour
// ≤ 48), the A3 birth band (filings by h48, births by h96), the sustained
// surplus run length, the C-cell granted/stock expectations, and the E-cell
// starvation-free window.
const (
	v6SustainedSurplusHours   = 12
	v6FirstSustainedHourBound = 48
	v6FirstFilingHourBound    = 48
	v6FirstBirthHourBound     = 96
	v6CGroundedGrantFloor     = 288
	v6CGranaryStockFloor      = 24
	v6CExtinctionBound        = 143
	v6EStarvationFreeHours    = 24
	v6ParentDeathWindowHours  = 24
	sfWindowHours             = 24
)

// The predeclared ticket-17 v7 band constants (frozen design, quoted): the
// V7-A2 per-actor band bound (first per-actor sustained hour ≤ 96; ≤ 48 is
// reported descriptively alongside), the V7-A3 birth band bounds (the frozen
// 48/96 numbers shared with the v6 band), and the V7-C run-max aggregate
// granary floor (v6 measured 16).
const (
	v7FirstSustainedHourBound = 96
	v7FirstFilingHourBound    = v6FirstFilingHourBound
	v7FirstBirthHourBound     = v6FirstBirthHourBound
	v7CGranaryStockFloor      = 20
	v7CExtinctionBound        = v6CExtinctionBound
	v7CGroundedGrantFloor     = v6CGroundedGrantFloor
	v7SustainedSurplusHours   = v6SustainedSurplusHours
	// The V7-D denial-loop counts recorded for the frozen v6-D cells (seed 0
	// / seed 7; worker-invariant) that the ablation band requires verbatim.
	v6DDenialEatsSeed0 = 1896
	v6DDenialEatsSeed7 = 1963
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
	policy              string
	mechanism           string
	out                 string
	checkpointDir       string
}

// naturalPolicyName validates the declared --policy selection and returns
// its canonical name. The zero value is exactly natural/5 (the D1
// default-preservation contract, held at the CLI layer too): every recorded
// pre-ticket-16 invocation keeps its bytes. natural/7 and natural/7b are the
// ticket-17 provision candidate and mechanism-necessity ablation.
func naturalPolicyName(policy string) (string, error) {
	switch policy {
	case "", policy5Name:
		return policy5Name, nil
	case policy6Name:
		return policy6Name, nil
	case policy7Name:
		return policy7Name, nil
	case policy7bName:
		return policy7bName, nil
	default:
		return "", fmt.Errorf("invalid -policy value %q: the frozen natural ref set is {%s, %s, %s, %s}", policy, policy5Name, policy6Name, policy7Name, policy7bName)
	}
}

// naturalMechanismName validates the declared --mechanism selection and
// returns its canonical name and world axis. The zero value is exactly v5
// (the D1 default-preservation contract): every recorded pre-ticket-17
// invocation keeps its bytes.
func naturalMechanismName(mechanism string) (string, world.NaturalMechanism, error) {
	switch mechanism {
	case "", mechanismV5Name:
		return mechanismV5Name, world.NaturalMechanismV5, nil
	case mechanismV6Name:
		return mechanismV6Name, world.NaturalMechanismV6, nil
	default:
		return "", world.NaturalMechanismV5, fmt.Errorf("invalid -mechanism value %q: the declared mechanism axis is {%s, %s}", mechanism, mechanismV5Name, mechanismV6Name)
	}
}

// policyRef resolves the declared policy selection to its frozen ref for the
// world seam (NaturalOptions.PolicyRef / NaturalRestoreOptions.PolicyRef).
func (o options) policyRef() (strategy.NaturalRef, error) {
	name, err := naturalPolicyName(o.policy)
	if err != nil {
		return strategy.NaturalRef{}, err
	}
	switch name {
	case policy6Name:
		return strategy.FrozenNaturalPolicyV6().Ref, nil
	case policy7Name:
		return strategy.FrozenNaturalPolicy7().Ref, nil
	case policy7bName:
		return strategy.FrozenNaturalPolicy7b().Ref, nil
	}
	return strategy.FrozenNaturalPolicy().Ref, nil
}

// mechanismAxis resolves the declared mechanism selection to its world axis
// value for the world seam (NaturalOptions.Mechanism /
// NaturalRestoreOptions.Mechanism).
func (o options) mechanismAxis() (world.NaturalMechanism, error) {
	_, m, err := naturalMechanismName(o.mechanism)
	return m, err
}

// legalPolicyUnderMechanism mirrors the frozen world-side ref/mechanism
// consistency rule (naturalRefLegalUnderMechanism) at the CLI layer so an
// illegal combination is refused before any publication (no output file, no
// checkpoint directory): natural/7 and natural/7b require mechanism v6.
func legalPolicyUnderMechanism(policyName string, m world.NaturalMechanism) bool {
	if m == world.NaturalMechanismV6 {
		return true
	}
	return policyName == policy5Name || policyName == policy6Name
}

// v7Requested reports whether the run selected a v7 provision arm
// (natural/7 or natural/7b; the additive ticket-17 metric and band sections).
func v7Requested(o options) bool {
	name, err := naturalPolicyName(o.policy)
	return err == nil && (name == policy7Name || name == policy7bName)
}

// mechanismReportName maps the mechanism to its report spelling: the v5 zero
// value serializes as empty so every recorded pre-ticket-17 report keeps its
// exact key set (omitempty).
func mechanismReportName(name string) string {
	if name == mechanismV5Name {
		return ""
	}
	return name
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
	// Provisioned is the ticket-17 rule-515 ledger total of the actor
	// (additive, omitted when zero so every v5/v6 report keeps its bytes).
	Provisioned int64 `json:"provisioned,omitempty"`
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
	GatherClaims        int64 `json:"gather_claims"`
	GatherAdmitted      int64 `json:"gather_admitted"`
	GatherDenied        int64 `json:"gather_denied"`
	Eat                 int64 `json:"eat"`
	EatStored           int64 `json:"eat_stored"`
	Wait                int64 `json:"wait"`
	Rest                int64 `json:"rest"`
	Builds              int64 `json:"builds"`
	BuildDenialEats     int64 `json:"build_denial_eats"`
	PairedMeals         int64 `json:"paired_meals"`
	FallbackEatStored   int64 `json:"fallback_eatstored"`
	BirthRequests       int64 `json:"birth_requests"`
	BirthReplies        int64 `json:"birth_replies"`
	BirthsAccepted      int64 `json:"births_accepted"`
	BirthRefusals       int64 `json:"birth_refusals"`
	BirthRefusalsSilent int64 `json:"birth_refusals_silent"`
	BirthWakeClaims     int64 `json:"birth_wake_claims"`
	LostBirths          int64 `json:"lost_births"`
	// Provisions/ProvisionRefused are the ticket-17 rule-515 counters
	// (additive, omitted when zero so every v5/v6 report keeps its bytes):
	// accepted provisions and typed provision-refused attempts. They are
	// always zero outside mechanism v6, and zero under every arm except
	// natural/7.
	Provisions       int64            `json:"provisions,omitempty"`
	ProvisionRefused int64            `json:"provision_refused,omitempty"`
	Refusals         map[string]int64 `json:"refusals"`
	WorldEvents      map[string]int64 `json:"world_rule_events"`
	WorldEventsNote  string           `json:"world_rule_events_note,omitempty"`
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

// v6ActorSurplusReport is one actor's sustained-hours counter: the longest
// run of consecutive surplus-bearing hours and the total surplus-bearing
// hours over the run.
type v6ActorSurplusReport struct {
	ID                         int `json:"id"`
	MaxConsecutiveSurplusHours int `json:"max_consecutive_surplus_hours"`
	SurplusBearingHours        int `json:"surplus_bearing_hours"`
}

// v6CapitalReport is the additive capital-persistence metric block: the
// actor share of alive hours with k≥1, aggregate Σk by hour, and slot
// turnover (tenure-mode transitions per patch).
type v6CapitalReport struct {
	Definition                   string  `json:"definition"`
	ShareAliveHoursKGE1          float64 `json:"share_alive_hours_k_ge1"`
	KTotalByHour                 []int64 `json:"k_total_by_hour"`
	SlotTurnoverTotal            int     `json:"slot_turnover_total"`
	SlotTurnoverPerPatchFinal24h []int   `json:"slot_turnover_per_patch_final_24h"`
}

// v6TimeToFirstReport is the additive time-to-first metric block, in
// absolute hours (-1 = never). The D-cell build time is the first denial
// attempt, per the frozen design.
type v6TimeToFirstReport struct {
	Definition        string `json:"definition"`
	BuildChoiceHour   int    `json:"first_build_choice_hour"`
	BuildAttemptHour  int    `json:"first_build_attempt_hour"`
	BuildEventHour    int    `json:"first_committed_build_hour"`
	DenialAttemptHour int    `json:"first_build_denial_attempt_hour"`
	SlotTenureHour    int    `json:"first_tenured_slot_hour"`
	BirthFilingHour   int    `json:"first_birth_filing_hour"`
	BirthHour         int    `json:"first_birth_hour"`
}

// v6ParentDeathEntry records one (parent, birth payment, death) triple: a
// parent that died within 24 h of paying a birth's granary price.
type v6ParentDeathEntry struct {
	Parent    int    `json:"parent"`
	BirthHour int64  `json:"birth_hour"`
	DeathHour int64  `json:"death_hour"`
	Cause     string `json:"cause"`
}

type v6ParentDeathsReport struct {
	Definition string               `json:"definition"`
	Count      int                  `json:"parents"`
	Entries    []v6ParentDeathEntry `json:"entries"`
}

// v6MetricsReport carries the additive ticket-16 v6 metric sections of the
// frozen design (surplus flow with the sustained-hours counter, capital
// persistence, time-to-first, parent deaths). It exists only under
// natural/6; a natural/5 report carries no such key.
type v6MetricsReport struct {
	Definition                          string                 `json:"definition"`
	SFByHourTotal                       []int64                `json:"sf_by_hour_total"`
	SFRunTotal                          int64                  `json:"sf_run_total"`
	RunMaxGranaryStock                  int64                  `json:"run_max_granary_stock"`
	FirstSurplusBearingHour             int                    `json:"first_surplus_bearing_hour"`
	FirstSustainedSurplusHour           int                    `json:"first_sustained_surplus_hour"`
	FirstSustainedSurplusCompletionHour int                    `json:"first_sustained_surplus_completion_hour"`
	MaxConsecutiveSurplusHours          int                    `json:"max_consecutive_surplus_hours"`
	SurplusBearingActorHours            int                    `json:"surplus_bearing_actor_hours"`
	PerActor                            []v6ActorSurplusReport `json:"per_actor_sustained_hours"`
	CapitalPersistence                  v6CapitalReport        `json:"capital_persistence"`
	TimeToFirst                         v6TimeToFirstReport    `json:"time_to_first"`
	ParentDeaths                        v6ParentDeathsReport   `json:"parent_deaths_within_24h_of_birth_payment"`
}

// naturalActorLedger is the per-actor G2a ledger slice one SF window needs.
type naturalActorLedger struct {
	alive       bool
	k           int64
	granary     int64
	cult        int64
	stored      int64
	provisioned int64
	spoiled     int64
	paid        int64
}

// v7ActorSurplusReport is one actor's ticket-17 surplus counters. Unlike the
// v6 reporter (whose per-actor field carries the reset end-state counter),
// MaxConsecutiveSurplusHours is the actor's true running maximum over the
// whole run — this is the GATING V7-A2 reading: the same actor held the full
// conjunction k≥2 ∧ granary≥1 ∧ SF′>0 for that many consecutive hours.
type v7ActorSurplusReport struct {
	ID                         int `json:"id"`
	MaxConsecutiveSurplusHours int `json:"max_consecutive_surplus_hours"`
	SurplusBearingHours        int `json:"surplus_bearing_hours"`
}

// v7NonUniversalityReport is the V7-A5 evidence block: total cause-attributed
// deaths to the horizon, and the surplus-bearing set versus the alive set at
// the first per-actor sustained hour (strict subset required in every A cell
// hosting a sustained-surplus actor).
type v7NonUniversalityReport struct {
	Definition                         string `json:"definition"`
	DeathsToHorizon                    int64  `json:"deaths_to_horizon"`
	FirstSustainedHour                 int    `json:"at_first_per_actor_sustained_hour"`
	AliveAtFirstSustainedHour          int    `json:"alive_at_first_sustained_hour"`
	SurplusBearingAtFirstSustainedHour []int  `json:"surplus_bearing_actors_at_first_sustained_hour"`
	StrictSubset                       bool   `json:"surplus_bearing_strict_subset_of_alive"`
}

// v7TimeToFirstReport is the ticket-17 time-to-first block, in absolute hours
// (-1 = never): the six predeclared provisions of the frozen design.
type v7TimeToFirstReport struct {
	Definition      string `json:"definition"`
	ProvisionHour   int    `json:"first_provision_hour"`
	BankedHour      int    `json:"first_banked_hour"`
	K2HoldHour      int    `json:"first_k2_hold_hour"`
	SustainedHour   int    `json:"first_per_actor_sustained_hour"`
	BirthFilingHour int    `json:"first_birth_filing_hour"`
	BirthHour       int    `json:"first_birth_hour"`
}

// v7MetricsReport carries the additive ticket-17 v7 metric sections of the
// frozen design. It exists only under natural/7 or natural/7b; a natural/5 or
// natural/6 report carries no such key. The capital-persistence and
// parent-death blocks reuse the v6 report shapes verbatim (same definitions,
// same computation).
type v7MetricsReport struct {
	Definition              string  `json:"definition"`
	SFPrimeByHourTotal      []int64 `json:"sf_prime_by_hour_total"`
	SFPrimeRunTotal         int64   `json:"sf_prime_run_total"`
	RunMaxGranaryStock      int64   `json:"run_max_granary_stock"`
	ProvisionedTotal        int64   `json:"provisioned_total"`
	PairedMeals             int64   `json:"paired_meals"`
	FirstSurplusBearingHour int     `json:"first_surplus_bearing_hour"`
	// GATING per-actor reading (V7-A2): the same actor holds the full
	// conjunction k≥2 ∧ granary≥1 ∧ SF′>0 for ≥12 consecutive hours.
	FirstPerActorSustainedHour           int                    `json:"first_per_actor_sustained_hour"`
	FirstPerActorSustainedCompletionHour int                    `json:"first_per_actor_sustained_completion_hour"`
	ActorsWithPerActorSustainedRuns      int                    `json:"actors_with_per_actor_sustained_runs"`
	SurplusBearingActorHours             int64                  `json:"surplus_bearing_actor_hours"`
	PerActor                             []v7ActorSurplusReport `json:"per_actor_sustained_hours"`
	PerActorReadingNote                  string                 `json:"per_actor_reading_note"`
	// Non-gating union reading (ticket 16's reading, labelled): an hour
	// counts while at least one alive actor is surplus-bearing, with no
	// requirement that it be the same actor.
	FirstSustainedSurplusHourUnion    int                     `json:"first_sustained_surplus_hour_union"`
	FirstSustainedCompletionHourUnion int                     `json:"first_sustained_surplus_completion_hour_union"`
	MaxConsecutiveSurplusHoursUnion   int                     `json:"max_consecutive_surplus_hours_union"`
	UnionReadingNote                  string                  `json:"union_reading_note"`
	NonUniversality                   v7NonUniversalityReport `json:"non_universality"`
	CapitalPersistence                v6CapitalReport         `json:"capital_persistence"`
	TimeToFirst                       v7TimeToFirstReport     `json:"time_to_first"`
	ParentDeaths                      v6ParentDeathsReport    `json:"parent_deaths_within_24h_of_birth_payment"`
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
	V6Metrics            *v6MetricsReport   `json:"v6_metrics,omitempty"`
	V6Bands              []bandReport       `json:"v6_bands,omitempty"`
	V7Metrics            *v7MetricsReport   `json:"v7_metrics,omitempty"`
	V7Bands              []bandReport       `json:"v7_bands,omitempty"`
}

type report struct {
	Status         string `json:"status"`
	Revision       string `json:"revision"`
	Patches        int    `json:"patches"`
	Founders       int    `json:"founders"`
	R              int64  `json:"r_per_patch_per_hour"`
	Seed           uint64 `json:"seed"`
	Workers        int    `json:"workers"`
	RequestedHours int    `json:"requested_hours"`
	CompletedHour  int    `json:"completed_hour"`
	Policy         string `json:"policy"`
	// Mechanism names the declared world axis; empty for the v5 zero value so
	// every recorded pre-ticket-17 report keeps its exact key set.
	Mechanism           string         `json:"mechanism,omitempty"`
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
	world.NaturalProvisionRule:    "provision",
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
// is reported, never tuned and never fatal. Under a v7 provision arm the
// primary predeclared v7 band takes its place.
func bandFor(o options, br *branchReport) bandReport {
	if name, err := naturalPolicyName(o.policy); err == nil && name == policy6Name {
		return v6BandsFor(o, br)[0]
	}
	if v7Requested(o) {
		return v7BandsFor(o, br)[0]
	}
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

// --- ticket-16 v6 additive metrics (natural/6 only; never emitted for natural/5) ---

const (
	v6SurplusDefinition = "SF(actor, trailing 24h) = per-actor G2a ledger deltas over the trailing 24h: ΣcultivatedGranted − ΣstoredMealsDrawn − Σspoiled − ΣbirthGranaryPaid; windows shorter than 24h at run start or actor birth; surplus-bearing at h = alive ∧ k≥2 ∧ granary≥1 ∧ SF>0; sustained = ≥12 consecutive surplus-bearing hours; time-to-first-sustained-surplus = the first hour of the earliest ≥12-hour run (its completion hour reported alongside)"
	v6CapitalDefinition = "capital persistence: actor share of alive hours with k≥1; Σk by hour (the aggregate balance, conservation-verified hourly); slot turnover = tenure-mode transitions (cultivated↔wild) per hour, totaled over the run and over the final trailing-24h window per patch"
	v6TimeToFirstDef    = "time-to-first, absolute hours (-1 = never): first build choice = the first journal build attempt of any recording (committed \"build\" attempt or \"build-denied-eat\"; D cells record only the denial attempt); committed build = the first accepted build (the rule-505 event); tenured slot = first hour with cultivated slots > 0 (fixture founders hold tenure from the genesis pre-history); birth filing = first birth-request journal attempt; birth = first committed birth"
	v6ParentDeathDef    = "parent deaths within 24h of a birth payment: a parent of some newborn (BirthHour bh) whose death hour d satisfies bh ≤ d ≤ bh+24; cause 'age' iff d equals the immutable scheduled death hour, else 'starvation'; parents counts distinct parents, entries list every qualifying (payment, death) pair"
)

// journalHour converts a journal timestamp to its absolute hour.
func journalHour(t sim.SimTime) int {
	return int(t / sim.SimTime(world.NaturalHour))
}

// buildV6Metrics computes the additive v6 metric sections from the hourly
// checkpoints and the journal. Everything is derived from the same frozen
// evidence the v5 reporter already uses; no world state is read twice and no
// v5 field is touched.
func buildV6Metrics(br *branchReport, checks []world.NaturalCheckpoint, journal []world.NaturalBatch) (v6MetricsReport, error) {
	m := v6MetricsReport{
		Definition:                          v6SurplusDefinition,
		FirstSurplusBearingHour:             -1,
		FirstSustainedSurplusHour:           -1,
		FirstSustainedSurplusCompletionHour: -1,
		TimeToFirst: v6TimeToFirstReport{
			Definition:        v6TimeToFirstDef,
			BuildChoiceHour:   -1,
			BuildAttemptHour:  -1,
			BuildEventHour:    -1,
			DenialAttemptHour: -1,
			SlotTenureHour:    -1,
			BirthFilingHour:   -1,
			BirthHour:         firstBirthHour(br.Population, br.FirstHour),
		},
		ParentDeaths: v6ParentDeathsReport{Definition: v6ParentDeathDef},
	}
	if len(checks) == 0 {
		return m, errors.New("no hourly projection for v6 metrics")
	}
	capital, err := capitalPersistenceReport(checks)
	if err != nil {
		return m, err
	}
	m.CapitalPersistence = capital
	windows := map[sim.EntityID][]int64{}
	counters := map[sim.EntityID]*v6ActorSurplusReport{}
	var prev map[sim.EntityID]naturalActorLedger
	delta := func(old, new int64, existed bool) int64 {
		if !existed {
			return new
		}
		return new - old
	}
	for _, c := range checks {
		type idState struct {
			id sim.EntityID
			a  world.NaturalActorState
		}
		roster := make([]idState, 0, len(c.Founders)+len(c.Newborns))
		for j := range c.Founders {
			roster = append(roster, idState{sim.EntityID(j + 1), c.Founders[j]})
		}
		for _, nb := range c.Newborns {
			roster = append(roster, idState{nb.ID, nb.State})
		}
		cur := make(map[sim.EntityID]naturalActorLedger, len(roster))
		var sfTotal int64
		for _, rs := range roster {
			a := rs.a
			sn := naturalActorLedger{alive: naturalAlive(a), k: a.Worksite.Capital, granary: a.Granary.Stock,
				cult: a.Granary.CultivatedTotal, stored: a.Granary.StoredMeals, spoiled: a.Granary.Spoiled,
				paid: a.Genome.BirthGranaryPaid}
			cur[rs.id] = sn
			p, had := prev[rs.id]
			flow := delta(p.cult, sn.cult, had) - delta(p.stored, sn.stored, had) -
				delta(p.spoiled, sn.spoiled, had) - delta(p.paid, sn.paid, had)
			windows[rs.id] = append(windows[rs.id], flow)
			win := windows[rs.id]
			var sf int64
			for j := len(win) - 1; j >= 0 && len(win)-1-j < sfWindowHours; j-- {
				sf += win[j]
			}
			if sn.alive {
				sfTotal += sf
			}
			bearing := sn.alive && sn.k >= 2 && sn.granary >= 1 && sf > 0
			ctr := counters[rs.id]
			if ctr == nil {
				ctr = &v6ActorSurplusReport{ID: int(rs.id)}
				counters[rs.id] = ctr
			}
			if bearing {
				ctr.MaxConsecutiveSurplusHours++
				ctr.SurplusBearingHours++
				m.SurplusBearingActorHours++
				if m.FirstSurplusBearingHour < 0 {
					m.FirstSurplusBearingHour = c.Hour
				}
				if ctr.MaxConsecutiveSurplusHours == v6SustainedSurplusHours && m.FirstSustainedSurplusHour < 0 {
					m.FirstSustainedSurplusHour = c.Hour - (v6SustainedSurplusHours - 1)
					m.FirstSustainedSurplusCompletionHour = c.Hour
				}
			} else {
				ctr.MaxConsecutiveSurplusHours = 0
			}
			if ctr.MaxConsecutiveSurplusHours > m.MaxConsecutiveSurplusHours {
				m.MaxConsecutiveSurplusHours = ctr.MaxConsecutiveSurplusHours
			}
		}
		m.SFByHourTotal = append(m.SFByHourTotal, sfTotal)
		if c.Balance.GranaryStock > m.RunMaxGranaryStock {
			m.RunMaxGranaryStock = c.Balance.GranaryStock
		}
		prev = cur
	}
	for _, id := range sortedActorIDs(counters) {
		m.PerActor = append(m.PerActor, *counters[id])
	}
	final := checks[len(checks)-1]
	m.SFRunTotal = final.Balance.CultivatedTotal - final.Balance.StoredMeals - final.Balance.Spoiled - final.Balance.BirthGranaryPaid
	// Time-to-first from the journal (available even after a typed halt).
	for _, b := range journal {
		for _, a := range b.Attempts {
			h := journalHour(a.Time)
			switch a.Kind {
			case "build":
				if m.TimeToFirst.BuildAttemptHour < 0 {
					m.TimeToFirst.BuildAttemptHour = h
				}
				if a.Accepted && m.TimeToFirst.BuildEventHour < 0 {
					m.TimeToFirst.BuildEventHour = h
				}
			case "build-denied-eat":
				if m.TimeToFirst.DenialAttemptHour < 0 {
					m.TimeToFirst.DenialAttemptHour = h
				}
			case "birth-request":
				if m.TimeToFirst.BirthFilingHour < 0 {
					m.TimeToFirst.BirthFilingHour = h
				}
			}
		}
	}
	m.TimeToFirst.BuildChoiceHour = m.TimeToFirst.BuildAttemptHour
	if m.TimeToFirst.BuildChoiceHour < 0 || (m.TimeToFirst.DenialAttemptHour >= 0 && m.TimeToFirst.DenialAttemptHour < m.TimeToFirst.BuildChoiceHour) {
		m.TimeToFirst.BuildChoiceHour = m.TimeToFirst.DenialAttemptHour
	}
	for _, c := range checks {
		tenured := 0
		for _, slots := range c.Slots {
			for _, slot := range slots {
				if slot.Mode == world.NaturalTenureCultivated {
					tenured++
				}
			}
		}
		if tenured > 0 {
			m.TimeToFirst.SlotTenureHour = c.Hour
			break
		}
	}
	// Parent deaths within 24h of a birth payment, from the final roster.
	m.ParentDeaths = parentDeathsReport(checks)
	return m, nil
}

// capitalPersistenceReport computes the additive capital-persistence block
// from the hourly checkpoints: the actor share of alive hours with k≥1, the
// aggregate Σk by hour, and slot turnover (tenure-mode transitions per hour,
// totaled over the run and over the final trailing-24h window per patch).
// Pure and shared verbatim by the v6 and v7 metric builders.
func capitalPersistenceReport(checks []world.NaturalCheckpoint) (v6CapitalReport, error) {
	cap := v6CapitalReport{Definition: v6CapitalDefinition}
	if len(checks) == 0 {
		return cap, errors.New("no hourly projection for capital persistence")
	}
	patches := len(checks[0].Slots)
	turnoverByHour := make([][]int, patches)
	var aliveHours, kge1Hours int64
	for i, c := range checks {
		cap.KTotalByHour = append(cap.KTotalByHour, c.Balance.Capital)
		rosterAlive := func(a world.NaturalActorState) {
			if naturalAlive(a) {
				aliveHours++
				if a.Worksite.Capital >= 1 {
					kge1Hours++
				}
			}
		}
		for j := range c.Founders {
			rosterAlive(c.Founders[j])
		}
		for _, nb := range c.Newborns {
			rosterAlive(nb.State)
		}
		if i > 0 {
			pc := checks[i-1]
			if len(c.Slots) != len(pc.Slots) {
				return cap, fmt.Errorf("slot patch count changed at hour %d", c.Hour)
			}
			for p := range c.Slots {
				if len(c.Slots[p]) != len(pc.Slots[p]) {
					return cap, fmt.Errorf("slot count changed at hour %d patch %d", c.Hour, p)
				}
				for s := range c.Slots[p] {
					if c.Slots[p][s].Mode != pc.Slots[p][s].Mode {
						cap.SlotTurnoverTotal++
						turnoverByHour[p] = append(turnoverByHour[p], c.Hour)
					}
				}
			}
		}
	}
	if aliveHours > 0 {
		cap.ShareAliveHoursKGE1 = float64(kge1Hours) / float64(aliveHours)
	}
	cap.SlotTurnoverPerPatchFinal24h = make([]int, patches)
	tail := len(checks) - sfWindowHours
	if tail < 0 {
		tail = 0
	}
	for p := 0; p < patches; p++ {
		for _, h := range turnoverByHour[p] {
			if h >= checks[tail].Hour {
				cap.SlotTurnoverPerPatchFinal24h[p]++
			}
		}
	}
	return cap, nil
}

// parentDeathsReport computes the parent-deaths-within-24h-of-a-birth-payment
// block from the final checkpoint roster. Pure and shared verbatim by the v6
// and v7 metric builders.
func parentDeathsReport(checks []world.NaturalCheckpoint) v6ParentDeathsReport {
	rep := v6ParentDeathsReport{Definition: v6ParentDeathDef}
	if len(checks) == 0 {
		return rep
	}
	final := checks[len(checks)-1]
	states := make(map[sim.EntityID]world.NaturalActorState, len(final.Founders)+len(final.Newborns))
	for j := range final.Founders {
		states[sim.EntityID(j+1)] = final.Founders[j]
	}
	for _, nb := range final.Newborns {
		states[nb.ID] = nb.State
	}
	parents := map[int]bool{}
	for _, nb := range final.Newborns {
		bh := nb.State.Genome.BirthHour
		for _, p := range [2]sim.EntityID{sim.EntityID(nb.State.Genome.ParentA), sim.EntityID(nb.State.Genome.ParentB)} {
			if p == 0 {
				continue
			}
			par, ok := states[p]
			if !ok {
				continue
			}
			d := par.Genome.DiedHour
			if d == world.NaturalNeverHour || d < bh || d > bh+v6ParentDeathWindowHours {
				continue
			}
			cause := "starvation"
			if d == par.Genome.DeathHour {
				cause = "age"
			}
			rep.Entries = append(rep.Entries, v6ParentDeathEntry{Parent: int(p), BirthHour: bh, DeathHour: d, Cause: cause})
			parents[int(p)] = true
		}
	}
	sort.Slice(rep.Entries, func(i, j int) bool {
		e, f := rep.Entries[i], rep.Entries[j]
		if e.BirthHour != f.BirthHour {
			return e.BirthHour < f.BirthHour
		}
		return e.Parent < f.Parent
	})
	rep.Count = len(parents)
	return rep
}

// --- ticket-17 v7 additive metrics (natural/7 and natural/7b only; never emitted for natural/5 or natural/6) ---

const (
	v7SurplusDefinition   = "SF′(actor, trailing 24h) = per-actor G2a′ ledger deltas over the trailing 24h: ΣcultivatedGranted − ΣstoredMeals − Σprovisioned − Σspoiled − ΣbirthGranaryPaid (provisions disclosed as withdrawals so the indicator cannot certify surplus during a drawdown); windows shorter than 24h at run start or actor birth; surplus-bearing at h = alive ∧ k≥2 ∧ granary≥1 ∧ SF′>0; the GATING sustained reading is per-actor: the same actor holds the full conjunction ≥12 consecutive hours"
	v7TimeToFirstDef      = "time-to-first, absolute hours (-1 = never): first provision = the first accepted rule-515 provision attempt (refused attempts are counted in event_counts, never here); first banked hour = the first hour with aggregate granary stock ≥ 1; first k≥2 hold = the first hour some single alive actor holds k≥2; first per-actor sustained = the GATING reading's first hour; birth filing = first birth-request journal attempt; birth = first committed birth"
	v7NonUniversalityDef  = "V7-A5 (gating): in every A cell hosting a sustained-surplus actor, ≥1 death before h168 and the surplus-bearing set at the first per-actor sustained hour is a strict subset of the alive set; zero mortality with universal surplus would suspect accounting or a hidden subsidy"
	v7PerActorReadingNote = "v7 per-actor entries carry the true running maximum of each actor's consecutive surplus-bearing hours; the v6 reporter's per-actor field carried the reset end-state counter (a run broken before the final hour reads 0), which the ticket-17 corrigendum misread as per-actor zero — supervisor-confirmed 2026-09-29; the v6 reporter's field semantics are frozen and unchanged"
	v7UnionReadingNote    = "ticket 16's reading, reported alongside and non-gating: an hour counts while at least one alive actor is surplus-bearing, with no requirement that it be the same actor"
)

// buildV7Metrics computes the additive ticket-17 metric sections from the
// hourly checkpoints and the journal: SF′ with the GATING per-actor sustained
// counter (true running maxima) alongside the non-gating hour-level union
// counter, provisioned totals, paired meals, time-to-first, the shared
// capital-persistence and parent-death blocks, and the V7-A5
// non-universality evidence. Everything is derived from the same frozen
// evidence the v5 reporter already uses; no world state is read twice and no
// v5/v6 field is touched.
func buildV7Metrics(br *branchReport, checks []world.NaturalCheckpoint, journal []world.NaturalBatch) (v7MetricsReport, error) {
	m := v7MetricsReport{
		Definition:                           v7SurplusDefinition,
		FirstSurplusBearingHour:              -1,
		FirstPerActorSustainedHour:           -1,
		FirstPerActorSustainedCompletionHour: -1,
		FirstSustainedSurplusHourUnion:       -1,
		FirstSustainedCompletionHourUnion:    -1,
		NonUniversality:                      v7NonUniversalityReport{Definition: v7NonUniversalityDef, FirstSustainedHour: -1},
		TimeToFirst: v7TimeToFirstReport{
			Definition:      v7TimeToFirstDef,
			ProvisionHour:   -1,
			BankedHour:      -1,
			K2HoldHour:      -1,
			SustainedHour:   -1,
			BirthFilingHour: -1,
			BirthHour:       firstBirthHour(br.Population, br.FirstHour),
		},
		ParentDeaths:        parentDeathsReport(checks),
		PerActorReadingNote: v7PerActorReadingNote,
		UnionReadingNote:    v7UnionReadingNote,
	}
	if len(checks) == 0 {
		return m, errors.New("no hourly projection for v7 metrics")
	}
	capital, err := capitalPersistenceReport(checks)
	if err != nil {
		return m, err
	}
	m.CapitalPersistence = capital
	windows := map[sim.EntityID][]int64{}
	counters := map[sim.EntityID]*v7ActorSurplusReport{}
	curRun := map[sim.EntityID]int{}
	var prev map[sim.EntityID]naturalActorLedger
	unionRun := 0
	delta := func(old, new int64, existed bool) int64 {
		if !existed {
			return new
		}
		return new - old
	}
	for _, c := range checks {
		type idState struct {
			id sim.EntityID
			a  world.NaturalActorState
		}
		roster := make([]idState, 0, len(c.Founders)+len(c.Newborns))
		for j := range c.Founders {
			roster = append(roster, idState{sim.EntityID(j + 1), c.Founders[j]})
		}
		for _, nb := range c.Newborns {
			roster = append(roster, idState{nb.ID, nb.State})
		}
		cur := make(map[sim.EntityID]naturalActorLedger, len(roster))
		var sfTotal int64
		hourBearing := false
		captureA5 := false
		for _, rs := range roster {
			a := rs.a
			sn := naturalActorLedger{alive: naturalAlive(a), k: a.Worksite.Capital, granary: a.Granary.Stock,
				cult: a.Granary.CultivatedTotal, stored: a.Granary.StoredMeals, provisioned: a.Granary.Provisioned,
				spoiled: a.Granary.Spoiled, paid: a.Genome.BirthGranaryPaid}
			cur[rs.id] = sn
			p, had := prev[rs.id]
			flow := delta(p.cult, sn.cult, had) - delta(p.stored, sn.stored, had) - delta(p.provisioned, sn.provisioned, had) -
				delta(p.spoiled, sn.spoiled, had) - delta(p.paid, sn.paid, had)
			windows[rs.id] = append(windows[rs.id], flow)
			win := windows[rs.id]
			var sf int64
			for j := len(win) - 1; j >= 0 && len(win)-1-j < sfWindowHours; j-- {
				sf += win[j]
			}
			if sn.alive {
				sfTotal += sf
				if sn.k >= 2 && m.TimeToFirst.K2HoldHour < 0 {
					m.TimeToFirst.K2HoldHour = c.Hour
				}
			}
			bearing := sn.alive && sn.k >= 2 && sn.granary >= 1 && sf > 0
			if bearing {
				hourBearing = true
				m.SurplusBearingActorHours++
				if m.FirstSurplusBearingHour < 0 {
					m.FirstSurplusBearingHour = c.Hour
				}
			}
			ctr := counters[rs.id]
			if ctr == nil {
				ctr = &v7ActorSurplusReport{ID: int(rs.id)}
				counters[rs.id] = ctr
			}
			if bearing {
				ctr.SurplusBearingHours++
				// GATING per-actor counter with the true running maximum:
				// unlike the v6 reporter the entry never loses a run to a
				// later reset.
				curRun[rs.id]++
				if curRun[rs.id] > ctr.MaxConsecutiveSurplusHours {
					ctr.MaxConsecutiveSurplusHours = curRun[rs.id]
				}
				if curRun[rs.id] == v7SustainedSurplusHours && m.FirstPerActorSustainedHour < 0 {
					m.FirstPerActorSustainedHour = c.Hour - (v7SustainedSurplusHours - 1)
					m.FirstPerActorSustainedCompletionHour = c.Hour
					// V7-A5 evidence is captured after the full roster's SF′
					// windows have advanced for this hour.
					captureA5 = true
				}
			} else {
				curRun[rs.id] = 0
			}
		}
		if captureA5 {
			m.NonUniversality.FirstSustainedHour = m.FirstPerActorSustainedHour
			m.NonUniversality.AliveAtFirstSustainedHour = c.Metrics.Alive
			for _, rs := range roster {
				win := windows[rs.id]
				var sf int64
				for j := len(win) - 1; j >= 0 && len(win)-1-j < sfWindowHours; j-- {
					sf += win[j]
				}
				oa := rs.a
				if naturalAlive(oa) && oa.Worksite.Capital >= 2 && oa.Granary.Stock >= 1 && sf > 0 {
					m.NonUniversality.SurplusBearingAtFirstSustainedHour = append(m.NonUniversality.SurplusBearingAtFirstSustainedHour, int(rs.id))
				}
			}
			m.NonUniversality.StrictSubset = len(m.NonUniversality.SurplusBearingAtFirstSustainedHour) < c.Metrics.Alive
		}
		if hourBearing {
			// Non-gating union reading (ticket 16's reading, labelled): an
			// hour counts while at least one alive actor is surplus-bearing.
			unionRun++
			if unionRun == v7SustainedSurplusHours && m.FirstSustainedSurplusHourUnion < 0 {
				m.FirstSustainedSurplusHourUnion = c.Hour - (v7SustainedSurplusHours - 1)
				m.FirstSustainedCompletionHourUnion = c.Hour
			}
			if unionRun > m.MaxConsecutiveSurplusHoursUnion {
				m.MaxConsecutiveSurplusHoursUnion = unionRun
			}
		} else {
			unionRun = 0
		}
		m.SFPrimeByHourTotal = append(m.SFPrimeByHourTotal, sfTotal)
		if c.Balance.GranaryStock > m.RunMaxGranaryStock {
			m.RunMaxGranaryStock = c.Balance.GranaryStock
		}
		if m.TimeToFirst.BankedHour < 0 && c.Balance.GranaryStock >= 1 {
			m.TimeToFirst.BankedHour = c.Hour
		}
		prev = cur
	}
	for _, id := range sortedActorIDs(counters) {
		entry := *counters[id]
		if entry.MaxConsecutiveSurplusHours >= v7SustainedSurplusHours {
			m.ActorsWithPerActorSustainedRuns++
		}
		m.PerActor = append(m.PerActor, entry)
	}
	if m.FirstPerActorSustainedHour >= 0 {
		m.TimeToFirst.SustainedHour = m.FirstPerActorSustainedHour
	}
	final := checks[len(checks)-1]
	m.SFPrimeRunTotal = final.Balance.CultivatedTotal - final.Balance.StoredMeals - final.Balance.Provisioned - final.Balance.Spoiled - final.Balance.BirthGranaryPaid
	m.ProvisionedTotal = final.Balance.Provisioned
	m.NonUniversality.DeathsToHorizon = final.Metrics.DeathsStarvation + final.Metrics.DeathsAge
	m.PairedMeals = br.Events.PairedMeals
	// Time-to-first from the journal (available even after a typed halt).
	for _, b := range journal {
		for _, a := range b.Attempts {
			h := journalHour(a.Time)
			switch a.Kind {
			case "provision":
				if a.ProvisionRefused == "" && m.TimeToFirst.ProvisionHour < 0 {
					m.TimeToFirst.ProvisionHour = h
				}
			case "birth-request":
				if m.TimeToFirst.BirthFilingHour < 0 {
					m.TimeToFirst.BirthFilingHour = h
				}
			}
		}
	}
	return m, nil
}

func sortedActorIDs[T any](counters map[sim.EntityID]*T) []sim.EntityID {
	ids := make([]sim.EntityID, 0, len(counters))
	for id := range counters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// v6BandsFor evaluates the predeclared ticket-16 v6 outcome bands of the
// frozen design verbatim for this branch's cell. Band misses are outcomes:
// reported, never tuned and never fatal. Cross-arm clauses (B/D hourly
// identity to their v5/v6 twin cells) are evaluated by the L3 control
// comparison across reports and noted in the criterion text.
func v6BandsFor(o options, br *branchReport) []bandReport {
	m := br.V6Metrics
	deref := v6MetricsReport{FirstSurplusBearingHour: -1, FirstSustainedSurplusHour: -1, FirstSustainedSurplusCompletionHour: -1, TimeToFirst: v6TimeToFirstReport{BuildChoiceHour: -1, BuildAttemptHour: -1, BuildEventHour: -1, DenialAttemptHour: -1, SlotTenureHour: -1, BirthFilingHour: -1, BirthHour: -1}}
	if m != nil {
		deref = *m
	}
	pop := br.Population
	builds := br.Events.Builds + br.Events.WorldEvents["build"]
	switch {
	case br.CultivationDisabled:
		within := builds == 0 && br.Events.BuildDenialEats > 0 && br.Totals.Granted == 0 && br.Totals.StoredMeals == 0
		return []bandReport{{Name: "D: cultivation ablation (v6)", Criterion: "zero rule-505 events ∧ zero committed build attempts ∧ denial loop engaged ∧ granted=0 ∧ stored=0 — the C>0/D=0 ledger delta is cultivation's sole signature (expected new finding: v6 still attempts builds, denial-eaten at high energy); alive-trajectory identity to v6-C is the L3 cross-arm comparison",
			Observed:   fmt.Sprintf("rule-505 events=%d, committed builds=%d, denial-eaten attempts=%d (first at h%d), granted=%d, stored_meals=%d", br.Events.WorldEvents["build"], br.Events.Builds, br.Events.BuildDenialEats, deref.TimeToFirst.DenialAttemptHour, br.Totals.Granted, br.Totals.StoredMeals),
			WithinBand: within}}
	case o.r == 0:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= 12 && br.Totals.Births == 0 && builds == 0
		return []bandReport{{Name: "B: R=0 extinction (v6)", Criterion: "extinction ≤ h12 ∧ 0 births ∧ 0 committed builds (within-run); hourly identity to the v5-B control arm is the L3 cross-arm comparison (rule-coverage proof: the v6 deltas are unreachable at R=0)",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, committed builds=%d, build choice recorded=%t (first at h%d), denial attempts=%d", pop.ExtinctionHour, br.Totals.Births, builds, deref.TimeToFirst.BuildChoiceHour >= 0, deref.TimeToFirst.BuildChoiceHour, br.Events.BuildDenialEats),
			WithinBand: within}}
	case br.Fixture:
		starved := int64(0)
		for i, n := range pop.DeathsStarvationByHour {
			if br.FirstHour+i < v6EStarvationFreeHours {
				starved += n
			}
		}
		within := starved == 0 && br.Totals.Births >= 1
		return []bandReport{{Name: "E: founder-tenure fixture (v6)", Criterion: "zero starvation deaths before h24 (the TFile=4 margin: paying from E≥4 must never leave a parent below the reserve within a day) ∧ births ≥ 1",
			Observed:   fmt.Sprintf("starvation_deaths_before_h24=%d, births=%d, first_birth_hour=%d", starved, br.Totals.Births, firstBirthHour(pop, br.FirstHour)),
			WithinBand: within}}
	case !br.Births:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= v6CExtinctionBound && br.Totals.Births == 0 &&
			br.Totals.Granted >= v6CGroundedGrantFloor && deref.RunMaxGranaryStock >= v6CGranaryStockFloor &&
			deref.FirstSustainedSurplusHour >= 0
		return []bandReport{{Name: "C: births-off cultivation baseline (v6)", Criterion: "extinct ≤ h143 ∧ 0 births ∧ cumulative granted ≥ 288 ∧ run-max aggregate granary stock ≥ 24 ∧ A2-style sustained surplus (∃ actor ≥ 12 consecutive surplus-bearing hours)",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, granted=%d, run_max_granary_stock=%d, first_sustained_surplus_hour=%d, max_consecutive_surplus_hours=%d", pop.ExtinctionHour, br.Totals.Births, br.Totals.Granted, deref.RunMaxGranaryStock, deref.FirstSustainedSurplusHour, deref.MaxConsecutiveSurplusHours),
			WithinBand: within}}
	case br.HaltedByCeilingHit:
		verdict, twin := "none", false
		if br.CeilingHit != nil {
			verdict = br.CeilingHit.Verdict.Verdict
			twin = br.CeilingHit.Verdict.ReplayIdentical != nil && *br.CeilingHit.Verdict.ReplayIdentical
		}
		within := br.CeilingHit != nil && br.ConservationVerified && twin
		return []bandReport{{Name: "F: compute-ceiling instrumentation (v6)", Criterion: "typed NaturalCeilingHit with green conservation to the hit hour ∧ workers-twin checkpoint/journal-identical; verdict per the frozen P15 table, reported verbatim (hit-hour estimate h33–80 non-binding)",
			Observed: fmt.Sprintf("ceiling=%s hit_hour=%d p15_verdict=%s twin_identical=%t", func() string {
				if br.CeilingHit != nil {
					return br.CeilingHit.Ceiling
				}
				return "none"
			}(), func() int {
				if br.CeilingHit != nil {
					return br.CeilingHit.Hour
				}
				return -1
			}(), verdict, twin),
			WithinBand: within}}
	default:
		// A cells (organic births-on R=8): A2, A3, A4.
		bands := make([]bandReport, 0, 3)
		actorsSustained := 0
		for _, a := range deref.PerActor {
			if a.MaxConsecutiveSurplusHours >= v6SustainedSurplusHours {
				actorsSustained++
			}
		}
		if br.CompletedHour != world.NaturalHorizonHours {
			bands = append(bands,
				bandReport{Name: "A2: v6 surplus band", Criterion: "∃ actor with k≥2 ∧ granary≥1 ∧ trailing-24h SF>0 sustained ≥12 consecutive hours, first such hour ≤ 48",
					Observed: fmt.Sprintf("not evaluable below h168 (completed_hour=%d)", br.CompletedHour), WithinBand: false},
				bandReport{Name: "A3: v6 births band", Criterion: "filings > 0 by h48 ∧ births ≥ 1 by h96",
					Observed: fmt.Sprintf("not evaluable below h168 (completed_hour=%d)", br.CompletedHour), WithinBand: false},
				bandReport{Name: "A4: v6 dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control",
					Observed: fmt.Sprintf("truncated at h%d — not evaluable", br.CompletedHour), WithinBand: false})
			return bands
		}
		a2Within := deref.FirstSustainedSurplusHour >= 0 && deref.FirstSustainedSurplusHour <= v6FirstSustainedHourBound
		bands = append(bands, bandReport{Name: "A2: v6 surplus band", Criterion: "∃ actor with k≥2 ∧ granary≥1 ∧ trailing-24h SF>0 sustained ≥12 consecutive hours, first such hour ≤ 48",
			Observed:   fmt.Sprintf("first_sustained_surplus_hour=%d (completion h%d), max_consecutive_surplus_hours=%d, actors_with_12h_runs=%d, surplus_bearing_actor_hours=%d, first_surplus_bearing_hour=%d", deref.FirstSustainedSurplusHour, deref.FirstSustainedSurplusCompletionHour, deref.MaxConsecutiveSurplusHours, actorsSustained, deref.SurplusBearingActorHours, deref.FirstSurplusBearingHour),
			WithinBand: a2Within})
		filing, birth := deref.TimeToFirst.BirthFilingHour, deref.TimeToFirst.BirthHour
		a3Within := filing >= 0 && filing <= v6FirstFilingHourBound && birth >= 0 && birth <= v6FirstBirthHourBound
		bands = append(bands, bandReport{Name: "A3: v6 births band", Criterion: "filings > 0 by h48 ∧ births ≥ 1 by h96 (filings with margins met but zero births = mechanism suspicion, investigated, never tuned)",
			Observed:   fmt.Sprintf("first_birth_filing_hour=%d, birth_requests=%d, first_birth_hour=%d, births=%d", filing, br.Events.BirthRequests, birth, br.Totals.Births),
			WithinBand: a3Within})
		alive168 := pop.AliveByHour[168-br.FirstHour]
		extinction := pop.ExtinctionHour
		var labels []string
		birthAfter96 := false
		for h := v6FirstBirthHourBound + 1; h <= world.NaturalHorizonHours; h++ {
			if pop.BirthsByHour[h-br.FirstHour] > 0 {
				birthAfter96 = true
				break
			}
		}
		if br.Totals.Births >= 8 && birthAfter96 && alive168 >= 8 {
			labels = append(labels, "takeoff-signal")
		}
		if (br.Totals.Births >= 1 && br.Totals.Births <= 7) || (alive168 >= 1 && alive168 <= 7) {
			labels = append(labels, "stagnation")
		}
		if br.Totals.Births >= 1 && alive168 == 0 {
			labels = append(labels, "collapse-after-activity")
		}
		if br.Totals.Births == 0 {
			labels = append(labels, "sterile-null")
		}
		if extinction >= 0 && extinction <= 95 {
			labels = append(labels, "collapse faster than control (extinction ≤ h95)")
		}
		classification := "none fired"
		if len(labels) > 0 {
			classification = strings.Join(labels, " + ")
		}
		bands = append(bands, bandReport{Name: "A4: v6 dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control",
			Observed:   fmt.Sprintf("births=%d, alive(168)=%d, extinction_hour=%d → %s", br.Totals.Births, alive168, extinction, classification),
			WithinBand: true})
		return bands
	}
}

// v7BandsFor evaluates the predeclared ticket-17 v7 outcome bands of the
// frozen design verbatim for this branch's cell. Band misses are outcomes:
// reported, never tuned and never fatal. The A cells carry V7-A2 (the GATING
// per-actor surplus band), V7-A3, V7-A4 (descriptive) and V7-A5 (the GATING
// non-universality band) in declared order; the frozen_band is the first.
// Cross-arm clauses (V7-B's three-way hourly identity, V7-D's D≡C identity
// and ledger delta) are evaluated by the control comparison across reports
// and noted in the criterion text.
func v7BandsFor(o options, br *branchReport) []bandReport {
	m := br.V7Metrics
	deref := v7MetricsReport{FirstSurplusBearingHour: -1, FirstPerActorSustainedHour: -1, FirstPerActorSustainedCompletionHour: -1,
		FirstSustainedSurplusHourUnion: -1, FirstSustainedCompletionHourUnion: -1,
		TimeToFirst: v7TimeToFirstReport{ProvisionHour: -1, BankedHour: -1, K2HoldHour: -1, SustainedHour: -1, BirthFilingHour: -1, BirthHour: -1}}
	if m != nil {
		deref = *m
	}
	pop := br.Population
	builds := br.Events.Builds + br.Events.WorldEvents["build"]
	provisions := br.Events.Provisions + br.Events.ProvisionRefused
	switch {
	case br.CultivationDisabled:
		// V7-D: the denial loop is cultivation's sole signature; the frozen
		// v6-D denial counts are the per-seed expectation.
		expected := int64(-1)
		note := "(seed outside {0,7}: no frozen v6-D count)"
		if o.seed == 0 {
			expected = v6DDenialEatsSeed0
			note = "(frozen v6-D s0: 1896)"
		} else if o.seed == 7 {
			expected = v6DDenialEatsSeed7
			note = "(frozen v6-D s7: 1963)"
		}
		within := br.Events.WorldEvents["build"] == 0 && builds == 0 && provisions == 0 && br.Events.BuildDenialEats == expected
		return []bandReport{{Name: "V7-D: cultivation ablation", Criterion: "zero rule-505 events ∧ zero builds ∧ zero provisions ∧ build-denial-eat count equal to the frozen v6-D cell " + note + " — D ≡ C hourly and the granted/stored ledger delta C>0/D=0 are the cross-arm comparisons",
			Observed:   fmt.Sprintf("rule-505 events=%d, builds=%d, provisions=%d (refused=%d), denial-eaten attempts=%d, granted=%d, stored_meals=%d", br.Events.WorldEvents["build"], builds, br.Events.Provisions, br.Events.ProvisionRefused, br.Events.BuildDenialEats, br.Totals.Granted, br.Totals.StoredMeals),
			WithinBand: within}}
	case o.r == 0:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= 12 && br.Totals.Births == 0 && builds == 0 && provisions == 0
		return []bandReport{{Name: "V7-B: R=0 extinction", Criterion: "extinction ≤ h12 ∧ 0 births ∧ 0 builds ∧ 0 provisions (stock ≡ 0 means the affordance is unreachable; any provision at R=0 = defect); hourly identity to the v5-B AND v6-B controls is the three-way cross-arm comparison",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, builds=%d, provisions=%d (refused=%d)", pop.ExtinctionHour, br.Totals.Births, builds, br.Events.Provisions, br.Events.ProvisionRefused),
			WithinBand: within}}
	case br.Fixture:
		starved := int64(0)
		for i, n := range pop.DeathsStarvationByHour {
			if br.FirstHour+i < v6EStarvationFreeHours {
				starved += n
			}
		}
		withins := deref.ActorsWithPerActorSustainedRuns >= 1 && br.Totals.Births >= 1
		return []bandReport{{Name: "V7-E: founder-tenure fixture", Criterion: "≥1 per-actor sustained ≥12h surplus run ∧ births ≥ 1; starvation deaths reported verbatim (the v6-E h19/h20 parent deaths remain the canary; zero starvation before h24 was the v6-E margin)",
			Observed:   fmt.Sprintf("actors_with_per_actor_sustained_runs=%d, first_per_actor_sustained_hour=%d, births=%d, first_birth_hour=%d, starvation_deaths_total=%d, starvation_deaths_before_h24=%d", deref.ActorsWithPerActorSustainedRuns, deref.FirstPerActorSustainedHour, br.Totals.Births, firstBirthHour(pop, br.FirstHour), sumHours(pop.DeathsStarvationByHour), starved),
			WithinBand: withins}}
	case !br.Births:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= v7CExtinctionBound && br.Totals.Births == 0 &&
			br.Totals.Granted >= v7CGroundedGrantFloor && deref.RunMaxGranaryStock >= v7CGranaryStockFloor
		return []bandReport{{Name: "V7-C: births-off cultivation baseline", Criterion: "extinct ≤ h143 ∧ 0 births ∧ cumulative granted ≥ 288 ∧ run-max aggregate granary stock ≥ 20 (v6 measured 16)",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, granted=%d, run_max_granary_stock=%d, sf_prime_run_total=%d", pop.ExtinctionHour, br.Totals.Births, br.Totals.Granted, deref.RunMaxGranaryStock, deref.SFPrimeRunTotal),
			WithinBand: within}}
	case br.HaltedByCeilingHit:
		verdict, twin := "none", false
		if br.CeilingHit != nil {
			verdict = br.CeilingHit.Verdict.Verdict
			twin = br.CeilingHit.Verdict.ReplayIdentical != nil && *br.CeilingHit.Verdict.ReplayIdentical
		}
		within := br.CeilingHit != nil && br.ConservationVerified && twin
		hitHour := -1
		ceiling := "none"
		if br.CeilingHit != nil {
			hitHour, ceiling = br.CeilingHit.Hour, br.CeilingHit.Ceiling
		}
		return []bandReport{{Name: "V7-F: compute-ceiling instrumentation", Criterion: "typed NaturalCeilingHit with green conservation to the hit hour ∧ workers-twin checkpoint/journal-identical; verdict per the frozen P15 table, reported verbatim (hit hour expected ≤ 66, descriptive: provisions add events)",
			Observed:   fmt.Sprintf("ceiling=%s hit_hour=%d p15_verdict=%s twin_identical=%t", ceiling, hitHour, verdict, twin),
			WithinBand: within}}
	default:
		// A cells (organic births-on R=8): V7-A2 (gating), V7-A3, V7-A4,
		// V7-A5 (gating), in declared order.
		bands := make([]bandReport, 0, 4)
		if br.CompletedHour != world.NaturalHorizonHours {
			notEvaluable := fmt.Sprintf("not evaluable below h168 (completed_hour=%d)", br.CompletedHour)
			return []bandReport{
				{Name: "V7-A2: per-actor surplus band (gating)", Criterion: "the same actor holds k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0 sustained ≥12 consecutive hours, first such hour ≤ 96 (≤ 48 reported descriptively), in all four A cells", Observed: notEvaluable, WithinBand: false},
				{Name: "V7-A3: births band", Criterion: "≥1 filing by h48 ∧ ≥1 birth by h96 in ≥1 A cell (zero filings = policy defect; filings with margins truth-met but zero births = mechanism suspicion, investigate never tune)", Observed: notEvaluable, WithinBand: false},
				{Name: "V7-A4: dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control", Observed: notEvaluable, WithinBand: false},
				{Name: "V7-A5: non-universality (gating)", Criterion: "in every A cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first sustained hour is a strict subset of the alive set", Observed: notEvaluable, WithinBand: false},
			}
		}
		a2Within := deref.FirstPerActorSustainedHour >= 0 && deref.FirstPerActorSustainedHour <= v7FirstSustainedHourBound
		descriptive := "≤48 descriptive:"
		if deref.FirstPerActorSustainedHour >= 0 && deref.FirstPerActorSustainedHour <= 48 {
			descriptive = "≤48 descriptive: MET"
		} else {
			descriptive = "≤48 descriptive: not met"
		}
		bands = append(bands, bandReport{Name: "V7-A2: per-actor surplus band (gating)", Criterion: "the same actor holds k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0 sustained ≥12 consecutive hours, first such hour ≤ 96 (≤ 48 reported descriptively), in all four A cells",
			Observed:   fmt.Sprintf("first_per_actor_sustained_hour=%d (completion h%d, %s), actors_with_per_actor_sustained_runs=%d, union_reading: first=%d max_run=%d, surplus_bearing_actor_hours=%d, first_surplus_bearing_hour=%d, sf_prime_run_total=%d, provisions=%d", deref.FirstPerActorSustainedHour, deref.FirstPerActorSustainedCompletionHour, descriptive, deref.ActorsWithPerActorSustainedRuns, deref.FirstSustainedSurplusHourUnion, deref.MaxConsecutiveSurplusHoursUnion, deref.SurplusBearingActorHours, deref.FirstSurplusBearingHour, deref.SFPrimeRunTotal, br.Events.Provisions),
			WithinBand: a2Within})
		filing, birth := deref.TimeToFirst.BirthFilingHour, deref.TimeToFirst.BirthHour
		a3Within := filing >= 0 && filing <= v7FirstFilingHourBound && birth >= 0 && birth <= v7FirstBirthHourBound
		bands = append(bands, bandReport{Name: "V7-A3: births band", Criterion: "≥1 filing by h48 ∧ ≥1 birth by h96 in ≥1 A cell (zero filings = policy defect; filings with margins truth-met but zero births = mechanism suspicion, investigate never tune)",
			Observed:   fmt.Sprintf("first_birth_filing_hour=%d, birth_requests=%d, first_birth_hour=%d, births=%d", filing, br.Events.BirthRequests, birth, br.Totals.Births),
			WithinBand: a3Within})
		alive168 := pop.AliveByHour[168-br.FirstHour]
		extinction := pop.ExtinctionHour
		var labels []string
		birthAfter96 := false
		for h := v7FirstBirthHourBound + 1; h <= world.NaturalHorizonHours; h++ {
			if pop.BirthsByHour[h-br.FirstHour] > 0 {
				birthAfter96 = true
				break
			}
		}
		if br.Totals.Births >= 8 && birthAfter96 && alive168 >= 8 {
			labels = append(labels, "takeoff-signal")
		}
		if (br.Totals.Births >= 1 && br.Totals.Births <= 7) || (alive168 >= 1 && alive168 <= 7) {
			labels = append(labels, "stagnation")
		}
		if br.Totals.Births >= 1 && alive168 == 0 {
			labels = append(labels, "collapse-after-activity")
		}
		if br.Totals.Births == 0 {
			labels = append(labels, "sterile-null")
		}
		if extinction >= 0 && extinction <= 95 {
			labels = append(labels, "collapse faster than control (extinction ≤ h95)")
		}
		classification := "none fired"
		if len(labels) > 0 {
			classification = strings.Join(labels, " + ")
		}
		bands = append(bands, bandReport{Name: "V7-A4: dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control",
			Observed:   fmt.Sprintf("births=%d, alive(168)=%d, extinction_hour=%d → %s", br.Totals.Births, alive168, extinction, classification),
			WithinBand: true})
		a5Within := true
		a5Observed := "no per-actor sustained surplus run — V7-A5 vacuous for this cell (V7-A2 already fails here)"
		if deref.ActorsWithPerActorSustainedRuns >= 1 {
			a5Within = deref.NonUniversality.DeathsToHorizon >= 1 && deref.NonUniversality.StrictSubset
			a5Observed = fmt.Sprintf("at first sustained hour h%d: alive=%d, surplus_bearing=%v, strict_subset=%t; deaths_to_horizon=%d (starvation=%d, age=%d)",
				deref.NonUniversality.FirstSustainedHour, deref.NonUniversality.AliveAtFirstSustainedHour, deref.NonUniversality.SurplusBearingAtFirstSustainedHour, deref.NonUniversality.StrictSubset,
				deref.NonUniversality.DeathsToHorizon, pop.DeathsStarvationByHour[len(pop.DeathsStarvationByHour)-1], pop.DeathsAgeByHour[len(pop.DeathsAgeByHour)-1])
		}
		bands = append(bands, bandReport{Name: "V7-A5: non-universality (gating)", Criterion: "in every A cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first sustained hour is a strict subset of the alive set (zero mortality with universal surplus ⇒ suspect accounting or hidden subsidy ⇒ defect investigation)",
			Observed:   a5Observed,
			WithinBand: a5Within})
		return bands
	}
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.NaturalHandoff, mech world.NaturalMechanism) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.NaturalRegistry()
	if mech == world.NaturalMechanismV6 {
		reg, err = world.NaturalRegistryFor(mech)
	}
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
			case "provision":
				// The ticket-17 rule-515 attempt: accepted provisions carry
				// the proposal key; a world-side refusal is keyless with the
				// typed provision-refused note (zero mutation, no fallback,
				// no bag-decision wake).
				if a.ProvisionRefused != "" {
					br.Events.ProvisionRefused++
				} else {
					br.Events.Provisions++
				}
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
func populationEvidence(br *branchReport, checks []world.NaturalCheckpoint, mech world.NaturalMechanism) error {
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
		// Independent frozen G1/G2a(/G2a′)/G2b/G3 re-verification at every
		// hour: the frozen v5 signature under the v5 zero value (its bytes
		// frozen), the mechanism-aware G2a′ form under v6.
		var balance world.NaturalBalance
		var err error
		if mech == world.NaturalMechanismV6 {
			balance, err = world.NaturalCheckConservationFor(mech, len(c.Patches), c.Patches, c.Slots, c.Founders, c.Newborns, c.AllocatorLast)
		} else {
			balance, err = world.NaturalCheckConservation(len(c.Patches), c.Patches, c.Slots, c.Founders, c.Newborns, c.AllocatorLast)
		}
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
		Provisioned:      a.Granary.Provisioned,
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
	ref, err := o.policyRef()
	if err != nil {
		return br, err
	}
	mech, err := o.mechanismAxis()
	if err != nil {
		return br, err
	}
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Seed: o.seed, Workers: o.workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech}
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
	if err := populationEvidence(&br, checks, mech); err != nil {
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
		byID, err := historyEvidence(&br, handoff, mech)
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
	// The additive v6/v7 sections are computed after the cost clock stops: pure
	// post-processing of the same frozen evidence, never a v5 field.
	if v6Requested(o) {
		vm, err := buildV6Metrics(&br, checks, f.Journal())
		if err != nil {
			return br, err
		}
		br.V6Metrics = &vm
	}
	if v7Requested(o) {
		vm, err := buildV7Metrics(&br, checks, f.Journal())
		if err != nil {
			return br, err
		}
		br.V7Metrics = &vm
	}
	br.Band = bandFor(o, &br)
	br.CostBudget = costBudget(o, &br)
	if v6Requested(o) {
		br.V6Bands = v6BandsFor(o, &br)
	}
	if v7Requested(o) {
		br.V7Bands = v7BandsFor(o, &br)
	}
	return br, nil
}

// v6Requested reports whether the run selected natural/6 (the additive
// ticket-16 metric and band sections).
func v6Requested(o options) bool {
	name, err := naturalPolicyName(o.policy)
	return err == nil && name == policy6Name
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
	ref, err := o.policyRef()
	if err != nil {
		return twin, err
	}
	mech, err := o.mechanismAxis()
	if err != nil {
		return twin, err
	}
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Seed: o.seed, Workers: workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech}
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
	policyName, err := naturalPolicyName(o.policy)
	if err != nil {
		return report{}, err
	}
	mechanismName, mech, err := naturalMechanismName(o.mechanism)
	if err != nil {
		return report{}, err
	}
	ref, err := o.policyRef()
	if err != nil {
		return report{}, err
	}
	// Ref/mechanism consistency is enforced here, before any publication, so
	// an illegal combination leaves no output file and no checkpoint
	// directory (the frozen world-side rule naturalRefLegalUnderMechanism
	// refuses the same combinations at construction).
	if !legalPolicyUnderMechanism(policyName, mech) {
		return report{}, fmt.Errorf("policy %s requires -mechanism=%s (the rule-515 provision affordance does not exist under the v5 world)", policyName, mechanismV6Name)
	}
	revision := fmt.Sprintf("natural format-v5/schema-v5/rule-v5/projection-v5/policy %s", policyName)
	if mech == world.NaturalMechanismV6 {
		revision = fmt.Sprintf("natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy %s", policyName)
	}
	r := report{
		Status:   "synthetic/model-conditional; frozen natural-limits v5 fixture; no historical calibration, takeoff, or P0 scale claim",
		Revision: revision,
		Patches:  o.patches, R: o.r, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: policyName, Mechanism: mechanismReportName(mechanismName), BranchSelection: o.births, FounderTenure: o.fixture,
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
		// so every continuation of this root is pinned to it. Under mechanism
		// v6 the genesis is built through the ticket-17 mechanism-aware seam
		// (NewNaturalFixtureForMechanism); the v5 path keeps the
		// ticket-16 constructor, whose bytes are frozen.
		var fx *world.Natural
		if mech == world.NaturalMechanismV6 {
			fx, err = world.NewNaturalFixtureForMechanism(o.patches, o.r, o.seed, true, 1, ref, mech)
		} else {
			fx, err = world.NewNaturalFixtureForPolicy(o.patches, o.r, o.seed, true, 1, ref)
		}
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
			Workers: 1, Births: false, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech})
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
				Births: false, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech}
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
	policy := flag.String("policy", policy5Name, "frozen policy ref (ticket-16 policy-selection seam): natural/5 (default, byte-compatible with every recorded run), natural/6, or the ticket-17 provision arms natural/7 and natural/7b (both require -mechanism=v6)")
	mechanism := flag.String("mechanism", mechanismV5Name, "declared world mechanism axis (ticket-17): v5 (default, the frozen v5 world, byte-compatible with every recorded run) or v6 (the rule-515 provision world)")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the root and branch bundles; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	rep, err := run(context.Background(), options{patches: *patches, r: *r, seed: *seed, workers: *workers, hours: *hours,
		births: *births, fixture: *fixture, cultivationDisabled: *cultivationDisabled, policy: *policy, mechanism: *mechanism,
		out: *out, checkpointDir: *checkpointDir})
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
