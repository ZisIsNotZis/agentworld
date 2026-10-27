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
// The -mechanism v7 world (ticket 20, .scratch/20-exchange-design/spec.md)
// and the -policy natural/9 arm carry the frozen v9 wage contract: the
// natural/9 cascade files rule-516 labour offers, decides rule-517
// engagements under the derived stock≥4 floor, settles rule-518 atomic wage
// contracts and expires rule-519 leftovers. natural/9 is legal only under
// mechanism v7; natural/5 and natural/6 stay legal under every mechanism and
// natural/7 and natural/7b under v6 or v7 (the zero-use control arm). Under
// natural/9 the reporter adds, purely additively, the predeclared ticket-20
// metric sections (per cell/patch/actor offer/accept/refuse/expire/void/
// settlement counts with typed void causes, the wage ledgers, the
// file→accept→settle latencies with the journal-chain discipline counts, the
// wage-banked firsts and wage-meal draws, the employer capital trajectory
// with time-to-k=8, the counterparty generation table, the ticket-19
// coexistence figure recomputed as a cross-check, and SF″ = SF′ − wagesPaid
// + wagesEarned printed beside the frozen SF′, which is never redefined) and
// the predeclared v9 bands (V9-T1..T6) with the frozen verdict logic
// (SUPPORTED / NULL-TRADE / FAILED). Every natural/5, natural/6, natural/7,
// natural/7b and default-band field stays byte-compatible: a report outside
// natural/9 carries none of the new keys.
//
// The optional -founders flag (ticket 18 v8 founding-band fixture axis,
// .scratch/18-surplus-via-reproduction/spec.md) selects the declared
// founding band: the number of founder rows the genesis materializes from
// the frozen founder-stream prefix. The zero value is exactly the full
// frozen block 8·P, so every recorded run keeps its bytes and its report
// key set; an explicit band is range-checked through world.NaturalFoundersValid
// before any publication (no output file, no checkpoint directory), the
// founder-tenure fixture additionally refuses f=1 (its coherent pre-history
// seats founders 1 and 2), and the band is run identity end to end (journal
// config, checkpoint manifest, bidirectional mismatch rejection, threaded
// through every construction, restore, branch and fixture call site). At a
// non-default band the reporter adds, purely additively, the predeclared v8
// density sections: the axis and density_world = f/(8P), per-patch seating
// with hourly mouths and per-patch Σk, per-actor SF′ counters with true
// running maxima and the home-patch attribution the V8-A2b band reads, the
// generation index and birth-cohort table (birth hour, parents, filing kin
// distance), the 8·births endowment ledger cross-checked inside G3, the
// parent-death and paired-meal canaries, per-patch time-to-first fields,
// and the predeclared v8 bands (V8-A2/A2b/A3/A4/A5/K and the cell bands
// V8-B/C2/D/E12/F). A default-band report carries none of the new keys.
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
	"agentworld/internal/component"
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
	policy9Name            = "natural/9"
	mechanismV5Name        = "v5"
	mechanismV6Name        = "v6"
	mechanismV7Name        = "v7"
	mechanismV8Name        = "v8"
	mechanismV9Name        = "v9"
	mechanismV10Name       = "v10"
	mechanismV11Name       = "v11"
	mechanismV12Name       = "v12"
	mechanismV13Name       = "v13"
	// The frozen v9 verdict labels plus the corner label the ticket-20 record
	// DECLARED BEFORE THE MATRIX RAN (spec comment: "the corner is declared as
	// its own outcome label, TRADE-INERT - trade occurred and was correctly
	// recorded, but neither channel produced a measurable effect"). The
	// reporter emits the declared label, so the report bytes and the recorded
	// vocabulary agree; the frozen three branches are unchanged and this label
	// is reachable only in that corner.
	v9VerdictSupported  = "SUPPORTED"
	v9VerdictNullTrade  = "NULL-TRADE"
	v9VerdictFailed     = "FAILED"
	v9VerdictTradeInert = "TRADE-INERT"
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

// The predeclared ticket-20 v9 band constants (frozen design, quoted): the
// V9-T1 trade-occurrence bound (first settlement ≤ h96, ≤48 reported
// descriptively) and the V9-T2 protocol-discipline chain window (every
// settlement's journal chain 516→517-accept→518 within ≤2 h).
const (
	v9FirstSettlementHourBound = 96
	v9DescriptiveHourBound     = 48
	v9ChainWindowHours         = 2
	// v9MaxRecordedViolations caps the per-band recorded violation lists; the
	// counts themselves are uncapped.
	v9MaxRecordedViolations = 4
)

// The predeclared ticket-18 v8 band constants (frozen design, quoted): the
// V8-A2 per-actor band bound (first per-actor sustained hour ≤ 96, the
// frozen v7 deadline carried over with its ≤ 48 descriptive companion), the
// V8-A3 birth bounds (the frozen 48/96 numbers), the V8-K capital
// persistence clauses (first tenured slot on the under-capacity patch ≤ h4;
// Σk on that patch ≥ 6 by h24 — two founders at the derived melt equilibrium
// k*=3; the first surplus-bearing actor's share of alive hours with k≥1
// ≥ 0.5, below which the indicator would certify a windfall), and the
// V8-A2b saturated-patch horizon (no saturated-patch actor reaches the
// indicator before h168).
const (
	v8FirstSustainedHourBound   = v7FirstSustainedHourBound
	v8FirstFilingHourBound      = v7FirstFilingHourBound
	v8FirstBirthHourBound       = v7FirstBirthHourBound
	v8SustainedSurplusHours     = v7SustainedSurplusHours
	v8FirstTenuredSlotHourBound = 4
	v8PatchKFloor               = 6
	v8PatchKDeadlineHour        = 24
	v8HolderShareKGE1Floor      = 0.5
	v8A2bHorizonHour            = 168
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
	// yield is the mechanism-v9 declared granary yield (units granted per
	// cultivated slot per patch-hour); 0 is the frozen flat grant of 1.
	yield int64
	// birthCost is the mechanism-v13 declared birth price (granary units and
	// energy units per paying parent); 0 is the frozen price of 2.
	birthCost int64
	// founders is the declared founding band (ticket 18 fixture axis): the
	// zero value is the default spelling of the full frozen block 8·patches,
	// anything else must satisfy world.NaturalFoundersValid.
	founders      int
	out           string
	checkpointDir string
}

// foundersResolved resolves the declared founding band to the materialized
// founder count, exactly the world-side rule: the zero spelling is the full
// frozen block 8P.
// yieldResolved resolves the declared granary yield exactly like the world's
// NaturalOptions.yieldResolved: the zero value is the frozen flat grant of 1.
// It is the form the conservation evidence reads; the world constructors take
// the declared (raw) spelling below, because the world's own resolver refuses
// a non-zero yield under a mechanism that does not declare the axis.
func (o options) yieldResolved() int64 {
	if o.yield == 0 {
		return 1
	}
	return o.yield
}

func (o options) foundersResolved() int {
	if o.founders == 0 {
		return 8 * o.patches
	}
	return o.founders
}

// v8Requested reports whether the run declared a non-default founding band
// (the additive ticket-18 metric and band sections). The default spelling
// keeps every recorded report byte-identical: a default report carries none
// of the new keys (pinned by test).
func v8Requested(o options) bool {
	return o.founders != 0
}

// naturalPolicyName validates the declared --policy selection and returns
// its canonical name. The zero value is exactly natural/5 (the D1
// default-preservation contract, held at the CLI layer too): every recorded
// pre-ticket-16 invocation keeps its bytes. natural/7 and natural/7b are the
// ticket-17 provision candidate and mechanism-necessity ablation; natural/9
// is the ticket-20 wage-contract arm.
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
	case policy9Name:
		return policy9Name, nil
	default:
		return "", fmt.Errorf("invalid -policy value %q: the frozen natural ref set is {%s, %s, %s, %s, %s}", policy, policy5Name, policy6Name, policy7Name, policy7bName, policy9Name)
	}
}

// naturalMechanismName validates the declared --mechanism selection and
// returns its canonical name and world axis. The zero value is exactly v5
// (the D1 default-preservation contract): every recorded pre-ticket-17
// invocation keeps its bytes. v7 is the ticket-20 wage-contract world.
func naturalMechanismName(mechanism string) (string, world.NaturalMechanism, error) {
	switch mechanism {
	case "", mechanismV5Name:
		return mechanismV5Name, world.NaturalMechanismV5, nil
	case mechanismV6Name:
		return mechanismV6Name, world.NaturalMechanismV6, nil
	case mechanismV7Name:
		return mechanismV7Name, world.NaturalMechanismV7, nil
	case mechanismV8Name:
		return mechanismV8Name, world.NaturalMechanismV8, nil
	case mechanismV9Name:
		return mechanismV9Name, world.NaturalMechanismV9, nil
	case mechanismV10Name:
		return mechanismV10Name, world.NaturalMechanismV10, nil
	case mechanismV11Name:
		return mechanismV11Name, world.NaturalMechanismV11, nil
	case mechanismV12Name:
		return mechanismV12Name, world.NaturalMechanismV12, nil
	case mechanismV13Name:
		return mechanismV13Name, world.NaturalMechanismV13, nil
	default:
		return "", world.NaturalMechanismV5, fmt.Errorf("invalid -mechanism value %q: the declared mechanism axis is {%s, %s, %s, %s, %s, %s, %s, %s, %s}", mechanism, mechanismV5Name, mechanismV6Name, mechanismV7Name, mechanismV8Name, mechanismV9Name, mechanismV10Name, mechanismV11Name, mechanismV12Name, mechanismV13Name)
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
	case policy9Name:
		return strategy.FrozenNaturalPolicy9().Ref, nil
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
// consistency ladder (naturalRefLegalUnderMechanism) at the CLI layer so an
// illegal combination is refused before any publication (no output file, no
// checkpoint directory): natural/5 and natural/6 are legal under every
// mechanism; natural/7 and natural/7b require v6 or v7 (the rule-515
// affordance); natural/9 requires v7 (rules 516-519 and the wage fields).
func legalPolicyUnderMechanism(policyName string, m world.NaturalMechanism) bool {
	switch policyName {
	case policy5Name, policy6Name:
		return true
	case policy7Name, policy7bName:
		return m == world.NaturalMechanismV6 || m == world.NaturalMechanismV7
	case policy9Name:
		return m == world.NaturalMechanismV7
	}
	return false
}

// v7Requested reports whether the run selected a v7 provision arm
// (natural/7 or natural/7b; the additive ticket-17 metric and band sections).
func v7Requested(o options) bool {
	name, err := naturalPolicyName(o.policy)
	return err == nil && (name == policy7Name || name == policy7bName)
}

// v9Requested reports whether the run selected the wage-contract arm
// (natural/9; the additive ticket-20 metric and band sections).
func v9Requested(o options) bool {
	name, err := naturalPolicyName(o.policy)
	return err == nil && name == policy9Name
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
	Provisions       int64 `json:"provisions,omitempty"`
	ProvisionRefused int64 `json:"provision_refused,omitempty"`
	// The ticket-20 wage-contract counters (additive, omitted when zero so
	// every report outside natural/9 keeps its bytes): rule-516 filings,
	// rule-517 decisions with the accept/refuse split, committed rule-518
	// settlements, and typed zero-mutation voids by cause. LabourAccepts and
	// LabourRefusals read the accepted 517 events' engagement writes, so they
	// are filled only when the accepted history was available; LabourExpiries
	// is the rule-519 world-event count inside WorldEvents.
	LabourOffers      int64            `json:"labour_offers,omitempty"`
	LabourDecisions   int64            `json:"labour_decisions,omitempty"`
	LabourAccepts     int64            `json:"labour_accepts,omitempty"`
	LabourRefusals    int64            `json:"labour_refusals,omitempty"`
	LabourSettlements int64            `json:"labour_settlements,omitempty"`
	LabourVoids       int64            `json:"labour_voids,omitempty"`
	LabourVoidCauses  map[string]int64 `json:"labour_void_causes,omitempty"`
	Refusals          map[string]int64 `json:"refusals"`
	WorldEvents       map[string]int64 `json:"world_rule_events"`
	WorldEventsNote   string           `json:"world_rule_events_note,omitempty"`
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

// v8PatchReport is one patch's founding-band column (ticket 18): the static
// genesis seating, the hourly density series (mouths and Σk over the
// patch's alive actors), and the patch-attributable time-to-first fields.
// A patch with seating == slots is a closed saturated economy (supply
// ≤ slots units/h against seating mouths at the frozen genesis) — the
// V8-A2b internal control; a seated patch below that is the under-capacity
// treatment the V8-A2/A3/C2/K bands read.
type v8PatchReport struct {
	Patch                      int     `json:"patch"`
	FoundersSeated             int     `json:"founders_seated"`
	Slots                      int     `json:"slots"`
	Saturated                  bool    `json:"saturated"`
	MouthsByHour               []int   `json:"mouths_by_hour"`
	KTotalByHour               []int64 `json:"k_total_by_hour"`
	FirstTenuredSlotHour       int     `json:"first_tenured_slot_hour"`
	FirstBankedHour            int     `json:"first_banked_hour"`
	FirstK2HoldHour            int     `json:"first_k2_hold_hour"`
	FirstPerActorSustainedHour int     `json:"first_per_actor_sustained_hour"`
	FirstBirthFilingHour       int     `json:"first_birth_filing_hour"`
	FirstBirthHour             int     `json:"first_birth_hour"`
	// Review P2 (ticket-18 L2): the density-hypothesis falsifier (ii) needs
	// "same-patch starvation while unclaimed slack exists" to be evaluable
	// from the report, so each patch column carries its lifetime unrealised
	// regeneration budget (land that could have produced and did not) and its
	// starvation death count (died_hour set without reaching the scheduled
	// death hour).
	UnrealizedSlack  int64 `json:"unrealized_slack_units"`
	StarvationDeaths int   `json:"starvation_deaths"`
}

// v8ActorSurplusReport is one actor's ticket-18 surplus counters with the
// home-patch attribution the V8-A2b band reads. MaxConsecutiveSurplusHours
// is the actor's true running maximum over the whole run (the v7 GATING
// semantics, immune to later resets), and the alive-hours counters feed the
// V8-K windfall clause (share of alive hours with k≥1).
type v8ActorSurplusReport struct {
	ID                         int   `json:"id"`
	HomePatch                  int   `json:"home_patch"`
	Generation                 int   `json:"generation"`
	MaxConsecutiveSurplusHours int   `json:"max_consecutive_surplus_hours"`
	SurplusBearingHours        int   `json:"surplus_bearing_hours"`
	FirstSurplusBearingHour    int   `json:"first_surplus_bearing_hour"`
	FirstPerActorSustainedHour int   `json:"first_per_actor_sustained_hour"`
	AliveHours                 int64 `json:"alive_hours"`
	AliveHoursWithKGE1         int64 `json:"alive_hours_with_k_ge1"`
}

// v8BirthCohortRow is one newborn's cohort row of the frozen metric list:
// birth hour, parents, generation index and the filing kin distance.
type v8BirthCohortRow struct {
	ID                int   `json:"id"`
	BirthHour         int64 `json:"birth_hour"`
	HomePatch         int   `json:"home_patch"`
	ParentA           int   `json:"parent_a"`
	ParentB           int   `json:"parent_b"`
	Generation        int   `json:"generation"`
	FilingKinDistance int   `json:"filing_kin_distance"`
}

// v8TimeToFirstReport is the ticket-18 whole-run time-to-first block, in
// absolute hours (-1 = never): the six predeclared fields, each attributed
// per patch in the v8PatchReport columns where attributable.
type v8TimeToFirstReport struct {
	Definition                 string `json:"definition"`
	FirstTenuredSlotHour       int    `json:"first_tenured_slot_hour"`
	FirstBankedHour            int    `json:"first_banked_hour"`
	FirstK2HoldHour            int    `json:"first_k2_hold_hour"`
	FirstPerActorSustainedHour int    `json:"first_per_actor_sustained_hour"`
	FirstBirthFilingHour       int    `json:"first_birth_filing_hour"`
	FirstBirthHour             int    `json:"first_birth_hour"`
}

// v8MetricsReport carries the additive ticket-18 v8 founding-band sections
// of the frozen design: the density axis, per-patch seating and hourly
// mouths/Σk, per-actor SF′ counters with home-patch attribution, the
// generation index and birth-cohort table, the 8·births endowment ledger,
// the parent-death and paired-meal canaries, and the V8-A5 non-universality
// evidence. It exists only when a non-default founders band is declared; a
// default-band report carries no such key (pinned by test).
type v8MetricsReport struct {
	Definition              string          `json:"definition"`
	FoundersDeclared        int             `json:"founders_declared"`
	FoundersResolved        int             `json:"founders_resolved"`
	DensityWorld            float64         `json:"density_world"`
	Patches                 []v8PatchReport `json:"patches"`
	SFPrimeByHourTotal      []int64         `json:"sf_prime_by_hour_total"`
	SFPrimeRunTotal         int64           `json:"sf_prime_run_total"`
	FirstSurplusBearingHour int             `json:"first_surplus_bearing_hour"`
	// GATING per-actor reading (V8-A2): the same actor holds the full
	// conjunction k≥2 ∧ granary≥1 ∧ SF′>0 for ≥12 consecutive hours; entries
	// carry the true running maximum.
	FirstPerActorSustainedHour           int                    `json:"first_per_actor_sustained_hour"`
	FirstPerActorSustainedCompletionHour int                    `json:"first_per_actor_sustained_completion_hour"`
	ActorsWithPerActorSustainedRuns      int                    `json:"actors_with_per_actor_sustained_runs"`
	SurplusBearingActorHours             int64                  `json:"surplus_bearing_actor_hours"`
	PerActor                             []v8ActorSurplusReport `json:"per_actor_surplus"`
	// Non-gating union reading (ticket 16's reading, labelled): an hour
	// counts while at least one alive actor is surplus-bearing, with no
	// requirement that it be the same actor.
	FirstSustainedSurplusHourUnion  int                     `json:"first_sustained_surplus_hour_union"`
	MaxConsecutiveSurplusHoursUnion int                     `json:"max_consecutive_surplus_hours_union"`
	TimeToFirst                     v8TimeToFirstReport     `json:"time_to_first"`
	GenerationCount                 int                     `json:"generation_count"`
	BirthCohort                     []v8BirthCohortRow      `json:"birth_cohort"`
	EndowmentLedgerUnits            int64                   `json:"endowment_ledger_units"`
	EndowmentCrossCheckedInG3       bool                    `json:"endowment_cross_checked_in_g3"`
	PairedMeals                     int64                   `json:"paired_meals"`
	ParentDeaths                    v6ParentDeathsReport    `json:"parent_deaths_within_24h_of_birth_payment"`
	NonUniversality                 v7NonUniversalityReport `json:"non_universality"`
}

// firstHolderBy returns the per-actor entry with the earliest recorded value
// of the named first-hour field (lowest ID on ties); ok is false when no
// actor ever recorded one.
func (m *v8MetricsReport) firstHolderBy(get func(v8ActorSurplusReport) int) (v8ActorSurplusReport, bool) {
	found := false
	var out v8ActorSurplusReport
	for _, a := range m.PerActor {
		h := get(a)
		if h < 0 {
			continue
		}
		if !found || h < get(out) || (h == get(out) && a.ID < out.ID) {
			found = true
			out = a
		}
	}
	return out, found
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
	V8Metrics            *v8MetricsReport   `json:"v8_metrics,omitempty"`
	V8Bands              []bandReport       `json:"v8_bands,omitempty"`
	V9Metrics            *v9MetricsReport   `json:"v9_metrics,omitempty"`
	V9Bands              []bandReport       `json:"v9_bands,omitempty"`
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
	// V9Verdict is the run-level frozen v9 verdict (ticket 20); present only
	// under natural/9.
	V9Verdict    *v9VerdictReport `json:"v9_verdict,omitempty"`
	WallTimeNS   int64            `json:"wall_time_ns"`
	CPUTimeNS    int64            `json:"cpu_time_ns"`
	PeakRSSBytes int64            `json:"peak_rss_bytes"`
	TokenCost    int              `json:"token_cost"`
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
	// The mechanism-v7 wage-contract rules (ticket 20): every accepted event
	// of these rules must be named or historyEvidence refuses the run.
	world.NaturalLabourOfferRule:      "labour_offer",
	world.NaturalLabourDecisionRule:   "labour_decision",
	world.NaturalLabourSettlementRule: "labour_settlement",
	world.NaturalLabourExpireRule:     "labour_expiry",
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
// is reported, never tuned and never fatal. Under the wage-contract arm the
// primary predeclared V9-T1 band takes precedence (the ticket-20 gating band
// of the cell); at a declared non-default founding band the primary
// predeclared v8 band otherwise takes its place, and under a v7 provision
// arm the primary predeclared v7 band.
func bandFor(o options, br *branchReport) bandReport {
	if v9Requested(o) {
		return v9BandsFor(o, br)[0]
	}
	if v8Requested(o) {
		return v8BandsFor(o, br)[0]
	}
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
		descriptive := "≤48 descriptive: not met"
		if deref.FirstPerActorSustainedHour >= 0 && deref.FirstPerActorSustainedHour <= 48 {
			descriptive = "≤48 descriptive: MET"
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
				deref.NonUniversality.DeathsToHorizon, sumHours(pop.DeathsStarvationByHour), sumHours(pop.DeathsAgeByHour))
		}
		bands = append(bands, bandReport{Name: "V7-A5: non-universality (gating)", Criterion: "in every A cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first sustained hour is a strict subset of the alive set (zero mortality with universal surplus ⇒ suspect accounting or hidden subsidy ⇒ defect investigation)",
			Observed:   a5Observed,
			WithinBand: a5Within})
		return bands
	}
}

// --- ticket-18 v8 founding-band additive metrics (declared -founders != 0 only; never emitted at the default band) ---

const (
	v8DensityDefinition  = "founding-band density block (ticket 18 v8, purely additive; emitted only when a non-default -founders band is declared — a default report carries none of these keys): founders_declared is the -founders spelling, founders_resolved the materialized band (the zero spelling ≡ the full frozen block 8P), density_world = f/(8·P); per-patch columns carry the genesis seating, hourly mouths (alive actors per patch, each burning one energy unit per hour), per-patch Σk by hour (alive actors), and the patch-attributable time-to-first fields; per-actor surplus entries carry the SF′ counters with true running maxima and the home-patch attribution the V8-A2b band reads; a patch with seating == slots is a closed saturated economy (supply ≤ slots units/h against seating mouths at the frozen genesis), the saturation arithmetic's internal control"
	v8TimeToFirstDef     = "time-to-first, absolute hours (-1 = never), whole run and per patch where attributable: first tenured slot = first hour with a cultivated slot (per patch: on that patch); first banked hour = first hour with aggregate granary stock ≥ 1 (per patch: Σ granary stock of the patch's alive actors ≥ 1); first k≥2 hold = first hour some alive actor holds k≥2 (per patch: an actor seated on that patch); first per-actor sustained = the GATING reading's first hour (per patch: attributed to the holder's home patch); birth filing = first birth-request journal attempt (per patch: the filing actor's home patch); birth = first committed birth (per patch: the newborn's home patch, which is the proposer's)"
	v8CohortDefinition   = "birth cohort, one row per newborn ascending by ID (= registration order): birth hour, home patch (the proposer's), parents, generation index (founders are generation 0; a newborn's generation = max(parent generations) + 1), and the filing kin distance — the PUBLIC derived kin-walk distance (world.NaturalKinDistance, the frozen ≤10-ref walk the consent rule reads) between the two parents at the birth hour; -1 when unrelated beyond the walk"
	v8NonUniversalityDef = "V8-A5 (gating): in every A12 cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first per-actor sustained hour is a strict subset of the alive set; zero mortality with universal surplus would suspect accounting or a hidden subsidy"
)

// v8HomePatchIndex maps an actor's immutable home-patch entity to its patch
// index; anything outside the declared patch space is a runner invariant
// break the reporter refuses rather than mis-attributes.
func v8HomePatchIndex(a world.NaturalActorState, patches int) (int, error) {
	p := int(a.Bag.Source) - world.NaturalPatchIDBase
	if p < 0 || p >= patches {
		return -1, fmt.Errorf("actor home patch %d outside the declared patch space", int(a.Bag.Source))
	}
	return p, nil
}

// buildV8Metrics computes the additive ticket-18 metric sections from the
// hourly checkpoints and the journal: the density axis and per-patch
// columns, per-actor SF′ counters (the frozen v7 form, provisions disclosed
// as withdrawals) with true running maxima and home-patch attribution, the
// generation index and birth cohort with the filing kin distance, the
// endowment ledger, the parent-death and paired-meal canaries, and the
// V8-A5 non-universality evidence. Everything is derived from the same
// frozen evidence the v5/v6/v7 reporters already use; no earlier report
// field is touched, and the whole block exists only at a declared
// non-default band.
func buildV8Metrics(o options, br *branchReport, checks []world.NaturalCheckpoint, journal []world.NaturalBatch) (v8MetricsReport, error) {
	m := v8MetricsReport{
		Definition:                           v8DensityDefinition,
		FoundersDeclared:                     o.founders,
		FoundersResolved:                     o.foundersResolved(),
		FirstSurplusBearingHour:              -1,
		FirstPerActorSustainedHour:           -1,
		FirstPerActorSustainedCompletionHour: -1,
		FirstSustainedSurplusHourUnion:       -1,
		NonUniversality:                      v7NonUniversalityReport{Definition: v8NonUniversalityDef, FirstSustainedHour: -1},
		TimeToFirst: v8TimeToFirstReport{
			Definition:                 v8TimeToFirstDef,
			FirstTenuredSlotHour:       -1,
			FirstBankedHour:            -1,
			FirstK2HoldHour:            -1,
			FirstPerActorSustainedHour: -1,
			FirstBirthFilingHour:       -1,
			FirstBirthHour:             firstBirthHour(br.Population, br.FirstHour),
		},
		PairedMeals:               br.Events.PairedMeals,
		EndowmentLedgerUnits:      8 * br.Totals.Births,
		EndowmentCrossCheckedInG3: br.ConservationVerified,
	}
	if len(checks) == 0 {
		return m, errors.New("no hourly projection for v8 metrics")
	}
	patches := len(checks[0].Slots)
	if patches == 0 || o.patches != patches {
		return m, fmt.Errorf("declared patches %d disagree with the projection's %d", o.patches, patches)
	}
	m.DensityWorld = float64(m.FoundersResolved) / float64(world.NaturalSlotsPerPatch*patches)
	m.ParentDeaths = parentDeathsReport(checks)

	type patchFirsts struct{ tenured, banked, k2, sustained, filing, birth int }
	firsts := make([]patchFirsts, patches)
	for i := range firsts {
		firsts[i] = patchFirsts{tenured: -1, banked: -1, k2: -1, sustained: -1, filing: -1, birth: -1}
	}
	// Static seating from the first projection: the founder rows are the
	// declared genesis prefix, each ID-placed by the frozen founder geometry.
	seated := make([]int, patches)
	for j := range checks[0].Founders {
		p, err := v8HomePatchIndex(checks[0].Founders[j], patches)
		if err != nil {
			return m, err
		}
		seated[p]++
	}
	m.Patches = make([]v8PatchReport, patches)
	for i := range m.Patches {
		m.Patches[i] = v8PatchReport{
			Patch: world.NaturalPatchIDBase + i, FoundersSeated: seated[i],
			Slots: world.NaturalSlotsPerPatch, Saturated: seated[i] == world.NaturalSlotsPerPatch,
			FirstTenuredSlotHour: -1, FirstBankedHour: -1, FirstK2HoldHour: -1,
			FirstPerActorSustainedHour: -1, FirstBirthFilingHour: -1, FirstBirthHour: -1,
		}
	}

	type actorAcc struct {
		entry  v8ActorSurplusReport
		run    int
		window []int64
	}
	acc := map[sim.EntityID]*actorAcc{}
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
		mouths := make([]int, patches)
		kTotal := make([]int64, patches)
		granary := make([]int64, patches)
		var sfTotal int64
		hourBearing := false
		captureA5 := false
		for _, rs := range roster {
			a := rs.a
			hp, err := v8HomePatchIndex(a, patches)
			if err != nil {
				return m, err
			}
			// Falsifier-(ii) column: count each starvation death exactly once,
			// at its own death hour (a death with died_hour set but short of
			// the scheduled death hour is a starvation death). Review fix: the
			// first version incremented inside this per-hour loop over the
			// roster, so every already-dead actor was re-counted on every
			// later hour (458 versus an exact roster count of 3).
			if a.Genome.DiedHour != world.NaturalNeverHour && a.Genome.DiedHour != a.Genome.DeathHour &&
				a.Genome.DiedHour == int64(c.Hour) && hp >= 0 && hp < len(m.Patches) {
				m.Patches[hp].StarvationDeaths++
			}
			sn := naturalActorLedger{alive: naturalAlive(a), k: a.Worksite.Capital, granary: a.Granary.Stock,
				cult: a.Granary.CultivatedTotal, stored: a.Granary.StoredMeals, provisioned: a.Granary.Provisioned,
				spoiled: a.Granary.Spoiled, paid: a.Genome.BirthGranaryPaid}
			cur[rs.id] = sn
			p, had := prev[rs.id]
			flow := delta(p.cult, sn.cult, had) - delta(p.stored, sn.stored, had) - delta(p.provisioned, sn.provisioned, had) -
				delta(p.spoiled, sn.spoiled, had) - delta(p.paid, sn.paid, had)
			ac := acc[rs.id]
			if ac == nil {
				ac = &actorAcc{entry: v8ActorSurplusReport{ID: int(rs.id), HomePatch: world.NaturalPatchIDBase + hp,
					FirstSurplusBearingHour: -1, FirstPerActorSustainedHour: -1}}
				acc[rs.id] = ac
			}
			ac.window = append(ac.window, flow)
			var sf int64
			for j := len(ac.window) - 1; j >= 0 && len(ac.window)-1-j < sfWindowHours; j-- {
				sf += ac.window[j]
			}
			if sn.alive {
				sfTotal += sf
				mouths[hp]++
				kTotal[hp] += sn.k
				granary[hp] += sn.granary
				ac.entry.AliveHours++
				if sn.k >= 1 {
					ac.entry.AliveHoursWithKGE1++
				}
				if sn.k >= 2 {
					if firsts[hp].k2 < 0 {
						firsts[hp].k2 = c.Hour
					}
					if m.TimeToFirst.FirstK2HoldHour < 0 {
						m.TimeToFirst.FirstK2HoldHour = c.Hour
					}
				}
			}
			bearing := sn.alive && sn.k >= 2 && sn.granary >= 1 && sf > 0
			if bearing {
				hourBearing = true
				m.SurplusBearingActorHours++
				ac.entry.SurplusBearingHours++
				if ac.entry.FirstSurplusBearingHour < 0 {
					ac.entry.FirstSurplusBearingHour = c.Hour
				}
				if m.FirstSurplusBearingHour < 0 {
					m.FirstSurplusBearingHour = c.Hour
				}
				// GATING per-actor counter with the true running maximum
				// (the v7 semantics): the entry never loses a run to a later
				// reset, and an actor's first sustained hour never moves.
				ac.run++
				if ac.run > ac.entry.MaxConsecutiveSurplusHours {
					ac.entry.MaxConsecutiveSurplusHours = ac.run
				}
				if ac.run == v8SustainedSurplusHours {
					first := c.Hour - (v8SustainedSurplusHours - 1)
					if ac.entry.FirstPerActorSustainedHour < 0 {
						ac.entry.FirstPerActorSustainedHour = first
					}
					if m.FirstPerActorSustainedHour < 0 {
						m.FirstPerActorSustainedHour = first
						m.FirstPerActorSustainedCompletionHour = c.Hour
						// V8-A5 evidence is captured after the full roster's
						// SF′ windows have advanced for this hour.
						captureA5 = true
					}
					if firsts[hp].sustained < 0 {
						firsts[hp].sustained = first
					}
				}
			} else {
				ac.run = 0
			}
		}
		if captureA5 {
			m.NonUniversality.FirstSustainedHour = m.FirstPerActorSustainedHour
			m.NonUniversality.AliveAtFirstSustainedHour = c.Metrics.Alive
			for _, rs := range roster {
				win := acc[rs.id].window
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
			if unionRun == v8SustainedSurplusHours && m.FirstSustainedSurplusHourUnion < 0 {
				m.FirstSustainedSurplusHourUnion = c.Hour - (v8SustainedSurplusHours - 1)
			}
			if unionRun > m.MaxConsecutiveSurplusHoursUnion {
				m.MaxConsecutiveSurplusHoursUnion = unionRun
			}
		} else {
			unionRun = 0
		}
		m.SFPrimeByHourTotal = append(m.SFPrimeByHourTotal, sfTotal)
		for p := 0; p < patches; p++ {
			m.Patches[p].MouthsByHour = append(m.Patches[p].MouthsByHour, mouths[p])
			m.Patches[p].KTotalByHour = append(m.Patches[p].KTotalByHour, kTotal[p])
			if granary[p] >= 1 && firsts[p].banked < 0 {
				firsts[p].banked = c.Hour
			}
		}
		if m.TimeToFirst.FirstBankedHour < 0 && c.Balance.GranaryStock >= 1 {
			m.TimeToFirst.FirstBankedHour = c.Hour
		}
		for p, slots := range c.Slots {
			for _, slot := range slots {
				if slot.Mode == world.NaturalTenureCultivated && firsts[p].tenured < 0 {
					firsts[p].tenured = c.Hour
				}
			}
		}
		prev = cur
	}
	if m.FirstPerActorSustainedHour >= 0 {
		m.TimeToFirst.FirstPerActorSustainedHour = m.FirstPerActorSustainedHour
	}
	for p := range m.Patches {
		m.Patches[p].FirstTenuredSlotHour = firsts[p].tenured
		m.Patches[p].FirstBankedHour = firsts[p].banked
		m.Patches[p].FirstK2HoldHour = firsts[p].k2
		m.Patches[p].FirstPerActorSustainedHour = firsts[p].sustained
		m.Patches[p].FirstBirthFilingHour = firsts[p].filing
		m.Patches[p].FirstBirthHour = firsts[p].birth
		if firsts[p].tenured >= 0 && (m.TimeToFirst.FirstTenuredSlotHour < 0 || firsts[p].tenured < m.TimeToFirst.FirstTenuredSlotHour) {
			m.TimeToFirst.FirstTenuredSlotHour = firsts[p].tenured
		}
		if p < len(checks[len(checks)-1].Patches) {
			m.Patches[p].UnrealizedSlack = checks[len(checks)-1].Patches[p].Unrealized
		}
	}

	final := checks[len(checks)-1]
	m.SFPrimeRunTotal = final.Balance.CultivatedTotal - final.Balance.StoredMeals - final.Balance.Provisioned - final.Balance.Spoiled - final.Balance.BirthGranaryPaid
	m.NonUniversality.DeathsToHorizon = final.Metrics.DeathsStarvation + final.Metrics.DeathsAge

	// Generation index over the final roster: ascending ID is a
	// parent-before-child order (a parent must exist to file, and founder
	// IDs precede every newborn ID), founders are generation 0.
	generation := make(map[sim.EntityID]int, len(final.Founders)+len(final.Newborns))
	for j := range final.Founders {
		generation[sim.EntityID(j+1)] = 0
	}
	nbs := append([]world.NaturalNewbornState(nil), final.Newborns...)
	sort.Slice(nbs, func(i, j int) bool { return nbs[i].ID < nbs[j].ID })
	for _, nb := range nbs {
		ga, okA := generation[nb.State.Genome.ParentA]
		gb, okB := generation[nb.State.Genome.ParentB]
		if !okA || !okB {
			return m, fmt.Errorf("newborn %d references a parent outside the roster", int(nb.ID))
		}
		g := ga
		if gb > g {
			g = gb
		}
		generation[nb.ID] = g + 1
		if g+1 > m.GenerationCount {
			m.GenerationCount = g + 1
		}
	}
	for _, id := range sortedActorIDs(acc) {
		entry := acc[id].entry
		if entry.MaxConsecutiveSurplusHours >= v8SustainedSurplusHours {
			m.ActorsWithPerActorSustainedRuns++
		}
		m.PerActor = append(m.PerActor, entry)
	}
	for i := range m.PerActor {
		m.PerActor[i].Generation = generation[sim.EntityID(m.PerActor[i].ID)]
	}

	// Home patches over the final roster (every actor ever alive, dead
	// included) for the journal filing attribution and the birth cohort.
	homeByID := make(map[sim.EntityID]int, len(final.Founders)+len(final.Newborns))
	for j := range final.Founders {
		p, err := v8HomePatchIndex(final.Founders[j], patches)
		if err != nil {
			return m, err
		}
		homeByID[sim.EntityID(j+1)] = p
	}
	for _, nb := range nbs {
		p, err := v8HomePatchIndex(nb.State, patches)
		if err != nil {
			return m, err
		}
		homeByID[nb.ID] = p
		if firsts[p].birth < 0 || int(nb.State.Genome.BirthHour) < firsts[p].birth {
			firsts[p].birth = int(nb.State.Genome.BirthHour)
		}
	}
	// Birth cohort with the filing kin distance recomputed at each birth
	// hour, and the per-patch birth firsts.
	for _, nb := range nbs {
		row := v8BirthCohortRow{
			ID: int(nb.ID), BirthHour: nb.State.Genome.BirthHour,
			HomePatch: world.NaturalPatchIDBase + homeByID[nb.ID],
			ParentA:   int(nb.State.Genome.ParentA), ParentB: int(nb.State.Genome.ParentB),
			Generation:        generation[nb.ID],
			FilingKinDistance: -1,
		}
		if d, ok := v8FilingKinDistance(checks, nb); ok {
			row.FilingKinDistance = d
		}
		m.BirthCohort = append(m.BirthCohort, row)
	}
	for p := range m.Patches {
		m.Patches[p].FirstBirthHour = firsts[p].birth
	}
	// Time-to-first from the journal (available even after a typed halt).
	for _, b := range journal {
		for _, a := range b.Attempts {
			if a.Kind != "birth-request" {
				continue
			}
			h := journalHour(a.Time)
			if m.TimeToFirst.FirstBirthFilingHour < 0 {
				m.TimeToFirst.FirstBirthFilingHour = h
			}
			if p, ok := homeByID[a.Actor]; ok && firsts[p].filing < 0 {
				firsts[p].filing = h
			}
		}
	}
	for p := range m.Patches {
		m.Patches[p].FirstBirthFilingHour = firsts[p].filing
	}
	return m, nil
}

// v8FilingKinDistance recomputes the PUBLIC derived kin distance between a
// newborn's parents at its birth hour — the same bounded ≤10-ref walk the
// frozen consent rule reads. The parent graph is append-only and a newborn
// is its own parents' child (never their ancestor), so the birth-hour
// projection carries exactly the filing-time distance. ok is false when the
// birth hour predates the recorded projection or a parent row is missing.
func v8FilingKinDistance(checks []world.NaturalCheckpoint, nb world.NaturalNewbornState) (int, bool) {
	if nb.State.Genome.ParentA == 0 || nb.State.Genome.ParentB == 0 {
		return 0, false
	}
	idx := int(nb.State.Genome.BirthHour) - checks[0].Hour
	if idx < 0 || idx >= len(checks) {
		return 0, false
	}
	c := checks[idx]
	states := make(map[sim.EntityID]world.NaturalActorState, len(c.Founders)+len(c.Newborns))
	for j := range c.Founders {
		states[sim.EntityID(j+1)] = c.Founders[j]
	}
	for _, other := range c.Newborns {
		states[other.ID] = other.State
	}
	if _, ok := states[nb.State.Genome.ParentA]; !ok {
		return 0, false
	}
	if _, ok := states[nb.State.Genome.ParentB]; !ok {
		return 0, false
	}
	lookup := world.NaturalParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		a, ok := states[id]
		if !ok {
			return 0, 0, false
		}
		return a.Genome.ParentA, a.Genome.ParentB, true
	})
	distance, related := world.NaturalKinDistance(lookup, nb.State.Genome.ParentA, nb.State.Genome.ParentB)
	if !related {
		return -1, true
	}
	return distance, true
}

// v8BandsFor evaluates the predeclared ticket-18 v8 outcome bands of the
// frozen design verbatim for this branch's cell. Band misses are outcomes:
// reported, never tuned and never fatal. The organic A cells carry V8-A2
// (the GATING per-actor surplus band), V8-A2b (the GATING saturated-patch
// negative control), V8-A3, V8-A4 (descriptive), V8-A5 (the GATING
// non-universality band) and V8-K (the GATING capital-persistence band) in
// declared order; the frozen_band is the first. Cross-arm clauses (V8-B's
// births-twin identity, V8-D's D12≡C12 hourly identity and the store
// falsifier's natural/7-vs-7b contrast at f=12) are evaluated by the L3
// control comparison across reports and noted in the criterion text.
func v8BandsFor(o options, br *branchReport) []bandReport {
	m := br.V8Metrics
	if m == nil {
		return []bandReport{{Name: "V8: metrics missing", Criterion: "the v8 bands require the v8 metric block", Observed: "v8_metrics absent while a non-default founding band is declared", WithinBand: false}}
	}
	pop := br.Population
	builds := br.Events.Builds + br.Events.WorldEvents["build"]
	provisions := br.Events.Provisions + br.Events.ProvisionRefused
	underCap := func() []v8PatchReport {
		out := []v8PatchReport{}
		for _, p := range m.Patches {
			if !p.Saturated && p.FoundersSeated > 0 {
				out = append(out, p)
			}
		}
		return out
	}
	saturated := func() []v8PatchReport {
		out := []v8PatchReport{}
		for _, p := range m.Patches {
			if p.Saturated {
				out = append(out, p)
			}
		}
		return out
	}
	patchList := func(ps []v8PatchReport) string {
		parts := make([]string, 0, len(ps))
		for _, p := range ps {
			parts = append(parts, fmt.Sprintf("%d(seats=%d/%d)", p.Patch, p.FoundersSeated, p.Slots))
		}
		if len(parts) == 0 {
			return "none"
		}
		return strings.Join(parts, ", ")
	}
	switch {
	case br.CultivationDisabled:
		// V8-D: the denial loop is cultivation's sole signature; the frozen
		// v6-D counts do not bind at f=12 (12 actors, not 16 — no equality
		// claim to f=16), only the counts themselves are recorded verbatim.
		within := br.Events.WorldEvents["build"] == 0 && builds == 0 && provisions == 0
		return []bandReport{{Name: "V8-D: cultivation ablation at the founding band", Criterion: "zero rule-505 events ∧ zero builds ∧ zero provisions (any build/provision at the ablation = defect); D12 alive-trajectory hourly identity to C12 and the denial counts recorded verbatim (expected below the frozen 1,896/1,963 because 12 actors, not 16 — no equality claim to f=16) are the L3 cross-arm comparisons",
			Observed:   fmt.Sprintf("rule-505 events=%d, builds=%d, provisions=%d (refused=%d), denial-eaten attempts=%d, granted=%d, stored_meals=%d", br.Events.WorldEvents["build"], builds, br.Events.Provisions, br.Events.ProvisionRefused, br.Events.BuildDenialEats, br.Totals.Granted, br.Totals.StoredMeals),
			WithinBand: within}}
	case o.r == 0:
		within := pop.ExtinctionHour >= 0 && pop.ExtinctionHour <= 12 && br.Totals.Births == 0 && builds == 0 && provisions == 0
		return []bandReport{{Name: "V8-B: R=0 extinction at the founding band", Criterion: "extinction ≤ h12 ∧ 0 births ∧ 0 builds ∧ 0 provisions (any build/provision at R=0 = defect); the births{on,off} B12 twin identity is the L3 cross-arm comparison",
			Observed:   fmt.Sprintf("extinction_hour=%d, births=%d, builds=%d, provisions=%d (refused=%d)", pop.ExtinctionHour, br.Totals.Births, builds, br.Events.Provisions, br.Events.ProvisionRefused),
			WithinBand: within}}
	case br.Fixture:
		// V8-E12 descriptive: the frozen-shape E cells (f=8P) are the v7-E
		// control gate, not v8 cells; a declared-band fixture cell is
		// recorded descriptively with its patch attribution.
		holder, has := m.firstHolderBy(func(a v8ActorSurplusReport) int { return a.FirstPerActorSustainedHour })
		holderText := "no per-actor sustained surplus run"
		if has {
			holderText = fmt.Sprintf("first per-actor sustained h%d on patch %d", holder.FirstPerActorSustainedHour, holder.HomePatch)
		}
		starved := sumHours(pop.DeathsStarvationByHour)
		return []bandReport{{Name: "V8-E12: founder-tenure fixture at the founding band (descriptive, non-gating)", Criterion: "descriptive foothold record at the declared band: births, first birth, the first per-actor sustained surplus with its home-patch attribution, and starvation deaths reported verbatim",
			Observed:   fmt.Sprintf("births=%d, first_birth_hour=%d, %s, starvation_deaths_total=%d, first_tenured_slot_hour=%d", br.Totals.Births, firstBirthHour(pop, br.FirstHour), holderText, starved, m.TimeToFirst.FirstTenuredSlotHour),
			WithinBand: true}}
	case !br.Births:
		// V8-C2 (gating for the diagnosis, C12 × {n7}): surplus still flips
		// on the under-capacity patch with first ≤ h96 — banking needs only
		// slack, not births; missed ⇒ the actual lever was generational
		// turnover, not slack, recorded verbatim.
		// Review P2 (ticket-18 L2): the frozen criterion is a PER-PATCH claim
		// ("surplus still flips on patch 1002 with first <= h96"), so read the
		// minimum under-capacity-patch first-sustained hour. Selecting the
		// globally earliest holder and requiring it to sit on the
		// under-capacity patch could record a false MISS in an A2b-fired world
		// and would corrupt a recorded diagnosis band.
		uc := underCap()
		firstUnderCapSustained := -1
		holderID, holderPatch := 0, 0
		for i := range m.PerActor {
			a := m.PerActor[i]
			for _, p := range uc {
				if a.HomePatch != p.Patch || a.FirstPerActorSustainedHour < 0 {
					continue
				}
				if firstUnderCapSustained < 0 || a.FirstPerActorSustainedHour < firstUnderCapSustained {
					firstUnderCapSustained, holderID, holderPatch = a.FirstPerActorSustainedHour, a.ID, a.HomePatch
				}
			}
		}
		within := firstUnderCapSustained >= 0 && firstUnderCapSustained <= v8FirstSustainedHourBound
		note, hourText := "none", "-1"
		if firstUnderCapSustained >= 0 {
			note = fmt.Sprintf("holder actor %d on patch %d", holderID, holderPatch)
			hourText = fmt.Sprintf("%d", firstUnderCapSustained)
		}
		return []bandReport{{Name: "V8-C2: births-off surplus at the founding band (gating for the diagnosis)", Criterion: "per-actor sustained surplus (k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0 ≥12 consecutive hours) with first such hour ≤ 96 held by an under-capacity-patch actor — banking needs only slack, not births; missed ⇒ the actual lever was generational turnover, recorded verbatim with the diagnosis corrected",
			Observed:   fmt.Sprintf("first_per_actor_sustained_hour=%s, %s, under-capacity patches: %s, completed_hour=%d", hourText, note, patchList(uc), br.CompletedHour),
			WithinBand: within}}
	case br.HaltedByCeilingHit:
		verdict, twin := "none", false
		if br.CeilingHit != nil {
			verdict = br.CeilingHit.Verdict.Verdict
			twin = br.CeilingHit.Verdict.ReplayIdentical != nil && *br.CeilingHit.Verdict.ReplayIdentical
		}
		within := br.CeilingHit != nil && br.ConservationVerified && twin
		hitHour, ceiling := -1, "none"
		if br.CeilingHit != nil {
			hitHour, ceiling = br.CeilingHit.Hour, br.CeilingHit.Ceiling
		}
		return []bandReport{{Name: "V8-F: compute-ceiling instrumentation", Criterion: "typed NaturalCeilingHit with green conservation to the hit hour ∧ workers-twin checkpoint/journal-identical; verdict per the frozen P15 table, reported verbatim (non-gating)",
			Observed:   fmt.Sprintf("ceiling=%s hit_hour=%d p15_verdict=%s twin_identical=%t", ceiling, hitHour, verdict, twin),
			WithinBand: within}}
	default:
		// A cells at the declared band: V8-A2, V8-A2b, V8-A3, V8-A4, V8-A5,
		// V8-K, in declared order.
		if br.CompletedHour != world.NaturalHorizonHours {
			notEvaluable := fmt.Sprintf("not evaluable below h168 (completed_hour=%d)", br.CompletedHour)
			return []bandReport{
				{Name: "V8-A2: per-actor surplus band (gating)", Criterion: "the same actor holds k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0 sustained ≥12 consecutive hours, first such hour ≤ 96 (≤ 48 reported descriptively)", Observed: notEvaluable, WithinBand: false},
				{Name: "V8-A2b: saturated-patch negative control (gating)", Criterion: "no saturated-patch actor reaches the per-actor indicator before h168 — any that does falsifies the saturation arithmetic reading and triggers a full recorded re-examination (a positive shock, not a band relaxation)", Observed: notEvaluable, WithinBand: false},
				{Name: "V8-A3: births band (gating)", Criterion: "first birth filing ≤ h48 ∧ first committed birth ≤ h96 on the under-capacity patch in ≥1 cell (zero filings = policy defect; filings with truth-met margins but zero births = mechanism suspicion, investigate never tune)", Observed: notEvaluable, WithinBand: false},
				{Name: "V8-A4: dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control", Observed: notEvaluable, WithinBand: false},
				{Name: "V8-A5: non-universality (gating)", Criterion: "in every A12 cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first sustained hour is a strict subset of the alive set", Observed: notEvaluable, WithinBand: false},
				{Name: "V8-K: capital persistence on the under-capacity patch (gating)", Criterion: "first tenured slot on the under-capacity patch ≤ h4 ∧ max Σk over that patch's hours ≤ h24 ≥ 6 (two founders at the derived k*=3) ∧ the first surplus-bearing actor's share of alive hours with k≥1 ≥ 0.5", Observed: notEvaluable, WithinBand: false},
			}
		}
		bands := make([]bandReport, 0, 6)
		a2Within := m.FirstPerActorSustainedHour >= 0 && m.FirstPerActorSustainedHour <= v8FirstSustainedHourBound
		descriptive := "≤48 descriptive: not met"
		if m.FirstPerActorSustainedHour >= 0 && m.FirstPerActorSustainedHour <= 48 {
			descriptive = "≤48 descriptive: MET"
		}
		holder, has := m.firstHolderBy(func(a v8ActorSurplusReport) int { return a.FirstPerActorSustainedHour })
		holderText := "no per-actor sustained surplus run"
		if has {
			holderText = fmt.Sprintf("first holder actor %d on patch %d", holder.ID, holder.HomePatch)
		}
		bands = append(bands, bandReport{Name: "V8-A2: per-actor surplus band (gating)", Criterion: "the same actor holds k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0 sustained ≥12 consecutive hours, first such hour ≤ 96 (≤ 48 reported descriptively)",
			Observed:   fmt.Sprintf("first_per_actor_sustained_hour=%d (completion h%d, %s), %s, actors_with_per_actor_sustained_runs=%d, union_reading: first=%d max_run=%d, surplus_bearing_actor_hours=%d, first_surplus_bearing_hour=%d, sf_prime_run_total=%d", m.FirstPerActorSustainedHour, m.FirstPerActorSustainedCompletionHour, descriptive, holderText, m.ActorsWithPerActorSustainedRuns, m.FirstSustainedSurplusHourUnion, m.MaxConsecutiveSurplusHoursUnion, m.SurplusBearingActorHours, m.FirstSurplusBearingHour, m.SFPrimeRunTotal),
			WithinBand: a2Within})
		sat := saturated()
		a2bFiring := []int{}
		for _, a := range m.PerActor {
			if a.FirstPerActorSustainedHour < 0 {
				continue
			}
			for _, p := range sat {
				if a.HomePatch == p.Patch {
					a2bFiring = append(a2bFiring, a.ID)
				}
			}
		}
		a2bWithin, a2bObserved := true, "vacuous: no saturated patch at this band"
		if len(sat) > 0 {
			a2bWithin = len(a2bFiring) == 0
			a2bObserved = fmt.Sprintf("saturated patches: %s; saturated-patch actors with a per-actor sustained run: %v (first through h%d)", patchList(sat), a2bFiring, v8A2bHorizonHour)
		}
		bands = append(bands, bandReport{Name: "V8-A2b: saturated-patch negative control (gating)", Criterion: "no saturated-patch actor reaches the per-actor indicator before h168 — any that does falsifies the saturation arithmetic reading and triggers a full recorded re-examination (a positive shock, not a band relaxation)",
			Observed: a2bObserved, WithinBand: a2bWithin})
		filing := m.TimeToFirst.FirstBirthFilingHour
		uc := underCap()
		// Review P2 (ticket-18 L2): the frozen criterion binds BOTH clauses to
		// the under-capacity patch ("first birth filing <= h48 and first
		// committed birth <= h96 on the under-capacity patch"), so the filing
		// clause is the minimum under-capacity-patch filing, not the whole-run
		// first filing (which could be a saturated-patch actor's and would
		// report IN with wrong-patch evidence).
		firstUnderCapFiling, firstUnderCapBirth := -1, -1
		for _, p := range uc {
			if p.FirstBirthFilingHour >= 0 && (firstUnderCapFiling < 0 || p.FirstBirthFilingHour < firstUnderCapFiling) {
				firstUnderCapFiling = p.FirstBirthFilingHour
			}
			if p.FirstBirthHour >= 0 && (firstUnderCapBirth < 0 || p.FirstBirthHour < firstUnderCapBirth) {
				firstUnderCapBirth = p.FirstBirthHour
			}
		}
		a3Within := firstUnderCapFiling >= 0 && firstUnderCapFiling <= v8FirstFilingHourBound && firstUnderCapBirth >= 0 && firstUnderCapBirth <= v8FirstBirthHourBound
		bands = append(bands, bandReport{Name: "V8-A3: births band (gating)", Criterion: "first birth filing ≤ h48 ∧ first committed birth ≤ h96 on the under-capacity patch in ≥1 cell (zero filings = policy defect; filings with truth-met margins but zero births = mechanism suspicion, investigate never tune)",
			Observed:   fmt.Sprintf("first_birth_filing_hour=%d (whole run), first under-capacity-patch filing_hour=%d, birth_requests=%d, first_birth_hour=%d, first under-capacity-patch birth_hour=%d, births=%d, under-capacity patches: %s", filing, firstUnderCapFiling, br.Events.BirthRequests, m.TimeToFirst.FirstBirthHour, firstUnderCapBirth, br.Totals.Births, patchList(uc)),
			WithinBand: a3Within})
		alive168 := pop.AliveByHour[168-br.FirstHour]
		extinction := pop.ExtinctionHour
		var labels []string
		birthAfter96 := false
		for h := v8FirstBirthHourBound + 1; h <= world.NaturalHorizonHours; h++ {
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
		bands = append(bands, bandReport{Name: "V8-A4: dynamics classification (descriptive, non-gating)", Criterion: "takeoff-signal births≥8 ∧ ≥1 birth after h96 ∧ alive(168)≥8; stagnation births∈[1,7] ∨ alive(168)∈[1,7]; collapse-after-activity births≥1 ∧ alive(168)=0; sterile-null births=0; extinction ≤ h95 flagged collapse-faster-than-control",
			Observed:   fmt.Sprintf("births=%d, alive(168)=%d, extinction_hour=%d → %s", br.Totals.Births, alive168, extinction, classification),
			WithinBand: true})
		a5Within := true
		a5Observed := "no per-actor sustained surplus run — V8-A5 vacuous for this cell (V8-A2 already fails here)"
		if m.ActorsWithPerActorSustainedRuns >= 1 {
			a5Within = m.NonUniversality.DeathsToHorizon >= 1 && m.NonUniversality.StrictSubset
			a5Observed = fmt.Sprintf("at first sustained hour h%d: alive=%d, surplus_bearing=%v, strict_subset=%t; deaths_to_horizon=%d (starvation=%d, age=%d)",
				m.NonUniversality.FirstSustainedHour, m.NonUniversality.AliveAtFirstSustainedHour, m.NonUniversality.SurplusBearingAtFirstSustainedHour, m.NonUniversality.StrictSubset,
				m.NonUniversality.DeathsToHorizon, sumHours(pop.DeathsStarvationByHour), sumHours(pop.DeathsAgeByHour))
		}
		bands = append(bands, bandReport{Name: "V8-A5: non-universality (gating)", Criterion: "in every A12 cell hosting a sustained-surplus actor, ≥1 death before h168 ∧ the surplus-bearing set at the first sustained hour is a strict subset of the alive set (zero mortality with universal surplus ⇒ suspect accounting or hidden subsidy ⇒ defect investigation)",
			Observed:   a5Observed,
			WithinBand: a5Within})
		kClauses := make([]string, 0, 3)
		kWithin := true
		if len(uc) == 0 {
			kClauses = append(kClauses, "vacuous: no under-capacity patch at this band")
		} else {
			firstTenured := -1
			for _, p := range uc {
				if p.FirstTenuredSlotHour >= 0 && (firstTenured < 0 || p.FirstTenuredSlotHour < firstTenured) {
					firstTenured = p.FirstTenuredSlotHour
				}
			}
			tenuredOK := firstTenured >= 0 && firstTenured <= v8FirstTenuredSlotHourBound
			kWithin = kWithin && tenuredOK
			kClauses = append(kClauses, fmt.Sprintf("first_tenured_slot_hour_on_under_capacity=%d (≤ h%d: %t)", firstTenured, v8FirstTenuredSlotHourBound, tenuredOK))
			kMax := int64(0)
			deadline := v8PatchKDeadlineHour - br.FirstHour
			if deadline >= len(pop.AliveByHour) {
				deadline = len(pop.AliveByHour) - 1
			}
			for _, p := range uc {
				for i := 0; i <= deadline && i < len(p.KTotalByHour); i++ {
					if p.KTotalByHour[i] > kMax {
						kMax = p.KTotalByHour[i]
					}
				}
			}
			kOK := kMax >= v8PatchKFloor
			kWithin = kWithin && kOK
			kClauses = append(kClauses, fmt.Sprintf("max_Σk_under_capacity_through_h%d=%d (≥ %d: %t)", v8PatchKDeadlineHour, kMax, v8PatchKFloor, kOK))
		}
		// Review P2 (ticket-18 final): on a cell with NO under-capacity patch
		// (the saturated A16 control arm) every K clause is vacuous, so the
		// band must report vacuous-true rather than letting the degenerate
		// share clause (no surplus holder at all) record a false MISS. V8-K
		// gates only on under-capacity cells by construction.
		if len(uc) == 0 {
			return append(bands, bandReport{Name: "V8-K: capital persistence on the under-capacity patch (gating)", Criterion: "first tenured slot on the under-capacity patch ≤ h4 ∧ max Σk over that patch's hours ≤ h24 ≥ 6 ∧ the first surplus-bearing actor's share of alive hours with k≥1 ≥ 0.5",
				Observed:   "vacuous for this cell: no under-capacity patch exists (saturated control arm)",
				WithinBand: true})
		}
		share := 0.0
		shareOK := false
		if fh, ok := m.firstHolderBy(func(a v8ActorSurplusReport) int { return a.FirstSurplusBearingHour }); ok && fh.AliveHours > 0 {
			share = float64(fh.AliveHoursWithKGE1) / float64(fh.AliveHours)
			shareOK = share >= v8HolderShareKGE1Floor
		}
		kWithin = kWithin && shareOK
		kClauses = append(kClauses, fmt.Sprintf("first surplus-bearing actor's share of alive hours with k≥1 = %.3f (≥ %.2f: %t)", share, v8HolderShareKGE1Floor, shareOK))
		bands = append(bands, bandReport{Name: "V8-K: capital persistence on the under-capacity patch (gating)", Criterion: "first tenured slot on the under-capacity patch ≤ h4 ∧ max Σk over that patch's hours ≤ h24 ≥ 6 (two founders at the derived k*=3) ∧ the first surplus-bearing actor's share of alive hours with k≥1 ≥ 0.5 (below that the indicator would certify a windfall, not sustained tenure)",
			Observed:   fmt.Sprintf("under-capacity patches: %s; %s", patchList(uc), strings.Join(kClauses, "; ")),
			WithinBand: kWithin})
		return bands
	}
}

// --- ticket-20 v9 wage-contract additive metrics, bands and verdict (natural/9 only; never emitted for any other arm) ---

const (
	v9TradeDefinition       = "ticket-20 wage-contract trade evidence: offers = rule-516 filings, decisions = rule-517 events with the accept/refuse split read from the accepted events' engagement-row writes, expiries = rule-519 world events, voids = typed zero-mutation settlement refusals by cause, settlements = committed rule-518 atomic wage contracts; per-actor and per-patch rows attribute by the immutable home patch. Accepts, refusals and expiries read the accepted history (-1 when unavailable after a typed ceiling halt; the journal attempt counts remain exact)"
	v9ChainDefinition       = "V9-T2 discipline counters: chain_violations count settlements whose journal chain 516→517-accept→518 does not close within ≤2 h (offer and same-hour accept at the delivery hour minus one, engagement hour matching); accept_without_offer_violations count accepts with no filed offer addressed to that employer in the same hour; field_set_violations count any accepted 516/517/518/519 event delta outside the frozen enumerated field sets (the offer publishes no claim/price/terms fields, so claim-vs-truth divergence is structurally 0 under this pin)"
	v9WageLedgerDefinition  = "wage ledgers: cumulative granary fields 8/9 (wages-paid, wages-earned) read from the final roster per actor and aggregated per patch, with run totals from the mechanism-v7 balance; patch_hour_checks × hour cells verify the per-patch-hour identity Σwages-paid = Σwages-earned on hourly ledger deltas (imbalances listed, capped). The ledger is a count: per-unit provenance is not recorded, and the patch-hour equality is the conservation cross-check"
	v9LatencyDefinition     = "file→accept→settle latency per settlement, in hours: offer hour (the 516 filing) → accept hour (the same-hour 517 decision) → settle hour (the engaged-hour+1 518 delivery); latency_hours = settle − offer"
	v9FirstsDefinition      = "wage-banked firsts, the V9-T4 reading of 'a worker whose first granary≥1 hour coincides with wagesEarned>0', read exactly: a worker with ≥1 settlement whose first settlement hour precedes every pulse-boundary sighting of that worker's granary stock ≥1 — the worker's first granary unit was the wage (a wage drawn at E≤3 in the same hour as its settlement is boundary-invisible, and the settlement row captures it; any pre-wage stock must appear at a pulse boundary, so the reading is exact, not a bracket)"
	v9WageMealDefinition    = "worker meals drawn from wages: eat-stored journal attempts by actors whose lifetime cultivated inflow is 0 and cumulative wages earned >0 at the draw hour — for such actors every granary unit is wage-borne (the G2a″ inflow channels are cultivated grants and wages), so the draw is a wage meal by construction; unit-level provenance is not recorded, and tenured actors mixing grant and wage stock are disclosed out of the exact count. The prior_boundary_e_le_3 column is the strict subset of draws whose preceding pulse-boundary energy was ≤3: basal burns only at pulses, so a prior boundary ≤3 implies the mid-hour decision energy was ≤3 (the converse does not hold, so this is a subset, not the count)"
	v9EmployerDefinition    = "employer channel (V9-T5): employers are actors with cumulative wages-paid >0; employer_k_by_hour is the Σk of alive employers per hour (the channel's capital trajectory), and time_to_k8_hour is the first hour any alive actor holds k ≥ 8 (KMax, -1 = never). The trajectory comparison against the (v7, natural/7b) control arm is the matrix lane's cross-arm step across reports, not a within-report computation"
	v9CounterpartyDef       = "counterparty generation table: settlements and wage units (1:1 per settlement) by (employer generation, worker generation), generations from the derived parent map over the final roster (founders generation 0; a newborn = max(parent generations)+1); the table distinguishes wage transfers that reach newborns from those that only shift founder wealth (the ticket-19 premise evidence)"
	v9CoexistenceDefinition = "ticket-19 exchange-precondition coexistence figure recomputed exactly from the hourly projections as a cross-check for the ticket-19 bracketed-bounds measurement: an hour counts when an alive surplus-bearing actor (k≥2 ∧ granary stock≥1 ∧ trailing-24h SF′>0, the frozen per-actor indicator) coexists with an alive actor holding k=0 ∧ granary stock=0; first_hour is the earliest such hour (-1 = none)"
	v9SFDefinition          = "SF″ = SF′ − wagesPaid + wagesEarned: per-actor trailing-24h wage-ledger deltas applied beside the frozen SF′ (carried verbatim from the frozen v7 computation, never redefined); the frozen indicator text and SF′ definition are unchanged"
)

// v9SettlementRow is one committed rule-518 settlement's reconstructed
// journal chain and ledgers.
type v9SettlementRow struct {
	Worker             int   `json:"worker"`
	Employer           int   `json:"employer"`
	Patch              int   `json:"patch"`
	OfferHour          int   `json:"offer_hour"`
	AcceptHour         int   `json:"accept_hour"`
	SettleHour         int   `json:"settle_hour"`
	LatencyHours       int   `json:"latency_hours"`
	PointCompleted     bool  `json:"point_completed"`
	FoodDelta          int64 `json:"food_delta"`
	WorkerGeneration   int   `json:"worker_generation"`
	EmployerGeneration int   `json:"employer_generation"`
}

// v9ActorTradeRow merges the per-actor trade counters and wage ledgers.
// Accepts/Refusals/Expiries are -1 when the accepted history was unavailable.
type v9ActorTradeRow struct {
	ID          int   `json:"id"`
	HomePatch   int   `json:"home_patch"`
	Generation  int   `json:"generation"`
	Offers      int64 `json:"offers"`
	Accepts     int64 `json:"accepts"`
	Refusals    int64 `json:"refusals"`
	Expiries    int64 `json:"expiries"`
	Voids       int64 `json:"voids"`
	Settlements int64 `json:"settlements"`
	WagesPaid   int64 `json:"wages_paid"`
	WagesEarned int64 `json:"wages_earned"`
}

// v9PatchTradeRow aggregates the trade counters and wage ledgers per patch.
type v9PatchTradeRow struct {
	Patch       int   `json:"patch"`
	Offers      int64 `json:"offers"`
	Accepts     int64 `json:"accepts"`
	Refusals    int64 `json:"refusals"`
	Expiries    int64 `json:"expiries"`
	Voids       int64 `json:"voids"`
	Settlements int64 `json:"settlements"`
	WagesPaid   int64 `json:"wages_paid"`
	WagesEarned int64 `json:"wages_earned"`
}

// v9CounterpartyRow is one (employer generation, worker generation) cell of
// the counterparty generation table.
type v9CounterpartyRow struct {
	EmployerGeneration int    `json:"employer_generation"`
	WorkerGeneration   int    `json:"worker_generation"`
	EmployerClass      string `json:"employer_class"`
	WorkerClass        string `json:"worker_class"`
	Settlements        int64  `json:"settlements"`
	WageUnits          int64  `json:"wage_units"`
}

// v9LedgerImbalance is one per-patch-hour wage-ledger equality violation.
type v9LedgerImbalance struct {
	Hour        int   `json:"hour"`
	Patch       int   `json:"patch"`
	PaidDelta   int64 `json:"paid_delta"`
	EarnedDelta int64 `json:"earned_delta"`
}

// v9WageLedgerReport is the wage-ledger block with its per-patch-hour
// conservation cross-check.
type v9WageLedgerReport struct {
	Definition              string              `json:"definition"`
	WagesPaidTotal          int64               `json:"wages_paid_total"`
	WagesEarnedTotal        int64               `json:"wages_earned_total"`
	PatchWagesPaidFinal     []int64             `json:"per_patch_wages_paid_final"`
	PatchWagesEarnedFinal   []int64             `json:"per_patch_wages_earned_final"`
	PatchHourChecks         int                 `json:"patch_hour_checks"`
	PatchHourImbalanceCount int                 `json:"patch_hour_imbalance_count"`
	PatchHourImbalances     []v9LedgerImbalance `json:"patch_hour_imbalances"`
}

// v9CoexistenceReport is the ticket-19 coexistence cross-check figure.
type v9CoexistenceReport struct {
	Definition string `json:"definition"`
	Hours      int    `json:"hours"`
	FirstHour  int    `json:"first_hour"`
}

// v9MetricsReport carries the additive ticket-20 metric sections. It exists
// only under natural/9; every other report carries no such key.
type v9MetricsReport struct {
	Definition                    string              `json:"definition"`
	LinkEvidenceAvailable         bool                `json:"link_evidence_available"`
	Offers                        int64               `json:"offers"`
	Decisions                     int64               `json:"decisions"`
	Accepts                       int64               `json:"accepts"`
	Refusals                      int64               `json:"refusals"`
	Expiries                      int64               `json:"expiries"`
	Voids                         int64               `json:"voids"`
	VoidCauses                    map[string]int64    `json:"void_causes,omitempty"`
	Settlements                   int64               `json:"settlements"`
	FirstSettlementHour           int                 `json:"first_settlement_hour"`
	SettlementRows                []v9SettlementRow   `json:"settlement_rows"`
	LatencyDefinition             string              `json:"latency_definition"`
	LatencyMinHours               int                 `json:"latency_min_hours"`
	LatencyMaxHours               int                 `json:"latency_max_hours"`
	LatencyTotalHours             int                 `json:"latency_total_hours"`
	ChainDefinition               string              `json:"chain_discipline_definition"`
	ChainViolations               int64               `json:"chain_violations"`
	ChainViolationNote            string              `json:"chain_violation_note,omitempty"`
	AcceptWithoutOfferViolations  int64               `json:"accept_without_offer_violations"`
	AcceptWithoutOfferNote        string              `json:"accept_without_offer_note,omitempty"`
	FieldSetViolations            int64               `json:"field_set_violations"`
	FieldSetViolationNote         string              `json:"field_set_violation_note,omitempty"`
	SettlementFoodDeltaViolations int64               `json:"settlement_food_delta_violations"`
	SettlementFoodDeltaNote       string              `json:"settlement_food_delta_note,omitempty"`
	PerActor                      []v9ActorTradeRow   `json:"per_actor_trade"`
	PerPatch                      []v9PatchTradeRow   `json:"per_patch_trade"`
	WageLedger                    v9WageLedgerReport  `json:"wage_ledger"`
	WageBankedFirsts              int                 `json:"wage_banked_firsts"`
	WageBankedFirstActors         []int               `json:"wage_banked_first_actors"`
	WageBankedFirstDefinition     string              `json:"wage_banked_first_definition"`
	WageMealDraws                 int64               `json:"wage_meal_draws"`
	WageMealDrawsBoundaryE3       int64               `json:"wage_meal_draws_prior_boundary_e_le_3"`
	WageMealDefinition            string              `json:"wage_meal_definition"`
	EmployerKByHour               []int64             `json:"employer_k_by_hour"`
	EmployerDefinition            string              `json:"employer_channel_definition"`
	TimeToK8Hour                  int                 `json:"time_to_k8_hour"`
	CounterpartyDefinition        string              `json:"counterparty_definition"`
	Counterparties                []v9CounterpartyRow `json:"counterparties"`
	Coexistence                   v9CoexistenceReport `json:"coexistence"`
	SFPrimeByHourTotal            []int64             `json:"sf_prime_by_hour_total"`
	SFDoublePrimeByHourTotal      []int64             `json:"sf_double_prime_by_hour_total"`
	SFPrimeRunTotal               int64               `json:"sf_prime_run_total"`
	SFDoublePrimeRunTotal         int64               `json:"sf_double_prime_run_total"`
	SFDefinition                  string              `json:"sf_definition"`
}

// v9VerdictReport is the run-level frozen v9 verdict over the run's
// branches (ticket 20): SUPPORTED iff T1∧T2∧T3∧T6 IN and (T4∨T5) observed;
// NULL-TRADE iff T2/T3/T6 IN with zero settlements; FAILED iff any gating
// discipline band is OUT. The corner the three frozen implications do not
// partition (gates IN, settlements > 0, both descriptive channels null) is
// labelled TRADE-INERT (declared pre-run) with the rationale verbatim, the v5
// reporter's no-frozen-branch-fired convention — never a fourth frozen
// verdict label.
type v9VerdictReport struct {
	Verdict             string `json:"verdict"`
	T1In                bool   `json:"t1_trade_occurrence_in"`
	T2In                bool   `json:"t2_protocol_discipline_in"`
	T3In                bool   `json:"t3_conservation_in"`
	T6In                bool   `json:"t6_non_distortion_in"`
	T4Observed          bool   `json:"t4_worker_channel_observed"`
	T5Observed          bool   `json:"t5_employer_channel_observed"`
	Settlements         int64  `json:"settlements"`
	FirstSettlementHour int    `json:"first_settlement_hour"`
	Rationale           string `json:"rationale"`
}

// v9HourActor is one actor's per-hour reading used by the v9 checkpoint pass.
type v9HourActor struct {
	alive        bool
	k            int64
	stock        int64
	cultivated   int64
	wagesPaid    int64
	wagesEarned  int64
	energy       int64 // pulse-boundary energy of this hour
	paidWindow   int64
	earnedWindow int64
	sfPrime      int64
	sfDouble     int64
}

// v9RosterIndex builds the final-roster lookup tables (home patch, generation
// via the derived parent map, full states) the v9 attribution reads. Every
// actor ever alive is in the final roster, dead included, and ascending ID is
// a parent-before-child order, so the generation walk is the v8 form.
func v9RosterIndex(final world.NaturalCheckpoint, patches int) (map[sim.EntityID]int, map[sim.EntityID]int, map[sim.EntityID]world.NaturalActorState, error) {
	home := make(map[sim.EntityID]int)
	states := make(map[sim.EntityID]world.NaturalActorState)
	for j := range final.Founders {
		p, err := v8HomePatchIndex(final.Founders[j], patches)
		if err != nil {
			return nil, nil, nil, err
		}
		id := sim.EntityID(j + 1)
		home[id] = p
		states[id] = final.Founders[j]
	}
	nbs := append([]world.NaturalNewbornState(nil), final.Newborns...)
	sort.Slice(nbs, func(i, j int) bool { return nbs[i].ID < nbs[j].ID })
	for _, nb := range nbs {
		p, err := v8HomePatchIndex(nb.State, patches)
		if err != nil {
			return nil, nil, nil, err
		}
		home[nb.ID] = p
		states[nb.ID] = nb.State
	}
	generation := make(map[sim.EntityID]int, len(states))
	for j := range final.Founders {
		generation[sim.EntityID(j+1)] = 0
	}
	for _, nb := range nbs {
		ga, okA := generation[nb.State.Genome.ParentA]
		gb, okB := generation[nb.State.Genome.ParentB]
		if !okA || !okB {
			return nil, nil, nil, fmt.Errorf("newborn %d references a parent outside the roster", int(nb.ID))
		}
		g := ga
		if gb > g {
			g = gb
		}
		generation[nb.ID] = g + 1
	}
	return home, generation, states, nil
}

// v9LabourEvent is one accepted labour event's extracted evidence.
type v9LabourEvent struct {
	rule         sim.RuleID
	hour         int
	actor        sim.EntityID // the cause actor (516 worker, 517 employer, 518 worker)
	counterparty sim.EntityID // 516 addressee; 517 decided worker; 518 paying employer
	engagedHour  int          // 517 accept: the book's engaged-hour value (-1 otherwise)
	accepted     bool         // 517: the decision engaged the worker
	foodDelta    int64        // 518: aggregate food delta (worker bag + both granary stocks)
	completed    bool         // 518: a point completion rode on the settlement
}

// v9ParseLabourEvents walks the accepted labour events in (time, ID) order,
// extracts the frozen evidence, and pins every event's delta set against the
// frozen enumerated field sets. Any unexpected component/field/entity role is
// a recorded field-set violation (the first noted verbatim), never a silent
// pass; a value that fails to decode is a hard integrity error.
func v9ParseLabourEvents(byID map[sim.EventID]kernel.Event, patches int) ([]v9LabourEvent, int64, string, error) {
	type evKey struct {
		time sim.SimTime
		id   sim.EventID
	}
	keys := make([]evKey, 0, len(byID))
	for id, ev := range byID {
		switch ev.Rule {
		case world.NaturalLabourOfferRule, world.NaturalLabourDecisionRule, world.NaturalLabourSettlementRule, world.NaturalLabourExpireRule:
			keys = append(keys, evKey{ev.Time, id})
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].time != keys[j].time {
			return keys[i].time < keys[j].time
		}
		return keys[i].id < keys[j].id
	})
	out := make([]v9LabourEvent, 0, len(keys))
	var violations int64
	var firstNote string
	note := func(kind string, hour int, actor sim.EntityID) {
		violations++
		if firstNote == "" {
			firstNote = fmt.Sprintf("first field-set violation: %s at h%d actor %d", kind, hour, int(actor))
		}
	}
	for _, k := range keys {
		ev := byID[k.id]
		hour := journalHour(ev.Time)
		cause := ev.Cause.Actor
		switch ev.Rule {
		case world.NaturalLabourOfferRule:
			row := v9LabourEvent{rule: ev.Rule, hour: hour, actor: cause, engagedHour: -1}
			sawHour, sawAddressee, sawStatus, sawLastOffer := false, false, false, false
			for _, d := range ev.Deltas {
				if d.Component != world.NaturalLabourOfferTypeID || d.Entity != cause {
					note("rule-516 delta outside the worker's own offer row", hour, cause)
					continue
				}
				switch d.Field {
				case world.NaturalLabourOfferHourField:
					sawHour = true
					v, err := d.After.Integer()
					if err != nil {
						return nil, 0, "", err
					}
					if v != int64(hour) {
						note("rule-516 offer hour differs from the event hour", hour, cause)
					}
				case world.NaturalLabourOfferAddresseeField:
					sawAddressee = true
					v, err := d.After.EntityRef()
					if err != nil {
						return nil, 0, "", err
					}
					row.counterparty = v
				case world.NaturalLabourOfferStatusField:
					sawStatus = true
				case world.NaturalLabourOfferLastOfferHourField:
					sawLastOffer = true
				default:
					note("rule-516 undeclared offer field", hour, cause)
				}
			}
			if !sawHour || !sawAddressee || !sawStatus || !sawLastOffer || len(ev.Deltas) != 4 {
				note("rule-516 field set is not exactly {hour, addressee, status, last-offer-hour}", hour, cause)
			}
			out = append(out, row)
		case world.NaturalLabourDecisionRule:
			row := v9LabourEvent{rule: ev.Rule, hour: hour, actor: cause, engagedHour: -1}
			for _, d := range ev.Deltas {
				switch {
				case d.Component == world.NaturalLabourOfferTypeID && d.Entity != cause:
					switch d.Field {
					case world.NaturalLabourOfferStatusField:
						row.counterparty = d.Entity // the decided worker
					case world.NaturalLabourOfferLastReplyHourField:
					default:
						note("rule-517 undeclared worker offer field", hour, cause)
					}
				case d.Component == world.NaturalLabourBookTypeID && d.Entity == cause:
					switch d.Field {
					case world.NaturalLabourBookEngagedWorkerField:
						row.accepted = true
						v, err := d.After.EntityRef()
						if err != nil {
							return nil, 0, "", err
						}
						row.counterparty = v
					case world.NaturalLabourBookEngagedHourField:
						v, err := d.After.Integer()
						if err != nil {
							return nil, 0, "", err
						}
						row.engagedHour = int(v)
					case world.NaturalLabourBookLastDecisionHourField:
					default:
						note("rule-517 undeclared book field", hour, cause)
					}
				default:
					note("rule-517 delta outside the two enumerated rows", hour, cause)
				}
			}
			if row.accepted && row.engagedHour != hour {
				note("rule-517 engaged hour differs from the decision hour", hour, cause)
			}
			out = append(out, row)
		case world.NaturalLabourSettlementRule:
			row := v9LabourEvent{rule: ev.Rule, hour: hour, actor: cause, engagedHour: -1}
			sawBag, sawWorkerStock, sawWorkerWage, sawEmployerStock, sawEmployerWage, sawInvested, sawWip := false, false, false, false, false, false, false
			// The food arithmetic decodes lazily: the completion's tenure
			// deltas are entity refs, not integers.
			foodDeltaOf := func(d component.FieldDelta) (int64, error) {
				v, err := d.After.Integer()
				if err != nil {
					return 0, err
				}
				b, err := d.Before.Integer()
				if err != nil {
					return 0, err
				}
				return v - b, nil
			}
			for _, d := range ev.Deltas {
				switch {
				case d.Component == world.NaturalBagTypeID && d.Field == world.NaturalBagUnitsField && d.Entity == cause:
					sawBag = true
					diff, err := foodDeltaOf(d)
					if err != nil {
						return nil, 0, "", err
					}
					row.foodDelta += diff
				case d.Component == world.NaturalGranaryTypeID && d.Field == world.NaturalGranaryStockField && d.Entity == cause:
					sawWorkerStock = true
					diff, err := foodDeltaOf(d)
					if err != nil {
						return nil, 0, "", err
					}
					row.foodDelta += diff
				case d.Component == world.NaturalGranaryTypeID && d.Field == world.NaturalGranaryWagesEarnedField && d.Entity == cause:
					sawWorkerWage = true
				case d.Component == world.NaturalGranaryTypeID && d.Field == world.NaturalGranaryStockField && d.Entity != cause:
					sawEmployerStock = true
					if row.counterparty != 0 && row.counterparty != d.Entity {
						note("rule-518 employer rows name two entities", hour, cause)
					}
					row.counterparty = d.Entity
					diff, err := foodDeltaOf(d)
					if err != nil {
						return nil, 0, "", err
					}
					row.foodDelta += diff
				case d.Component == world.NaturalGranaryTypeID && d.Field == world.NaturalGranaryWagesPaidField && d.Entity != cause:
					sawEmployerWage = true
					if row.counterparty != 0 && row.counterparty != d.Entity {
						note("rule-518 employer rows name two entities", hour, cause)
					}
					row.counterparty = d.Entity
				case d.Component == world.NaturalWorksiteTypeID && d.Entity != cause &&
					(d.Field == world.NaturalWorksiteInvestedUnitsField || d.Field == world.NaturalWorksiteWipField):
					if d.Field == world.NaturalWorksiteInvestedUnitsField {
						sawInvested = true
						// The invested unit is food (the G2b unit ledger): the
						// employer's granary stock −1 lands here, so the
						// settlement's aggregate food delta reads bag −1 +
						// worker stock +1 + employer stock −1 + invested +1.
						diff, err := foodDeltaOf(d)
						if err != nil {
							return nil, 0, "", err
						}
						row.foodDelta += diff
					} else {
						sawWip = true
					}
					if row.counterparty != 0 && row.counterparty != d.Entity {
						note("rule-518 employer rows name two entities", hour, cause)
					}
					row.counterparty = d.Entity
				case d.Component == world.NaturalWorksiteTypeID && d.Entity != cause &&
					(d.Field == world.NaturalWorksiteCapitalField || d.Field == world.NaturalWorksitePointsCreatedField):
					row.completed = true
				case d.Component == world.NaturalSlotTypeID &&
					(d.Field == world.NaturalSlotTenureModeField || d.Field == world.NaturalSlotTenureOwnerField || d.Field == world.NaturalSlotTenureCultivatedField):
					idx := int(d.Entity) - world.NaturalSlotIDBase
					if idx < 0 || idx >= patches*world.NaturalSlotsPerPatch {
						note("rule-518 completion slot outside the declared slot space", hour, cause)
					}
					row.completed = true
				default:
					note("rule-518 delta outside the enumerated settlement field set", hour, cause)
				}
			}
			if !sawBag || !sawWorkerStock || !sawWorkerWage || !sawEmployerStock || !sawEmployerWage || !sawInvested || !sawWip {
				note("rule-518 enumerated settlement field set incomplete", hour, cause)
			}
			out = append(out, row)
		case world.NaturalLabourExpireRule:
			// Rule 519 expiry/book-clear rows: one row per expired worker
			// offer; the book-clear deltas are validated but not rows.
			for _, d := range ev.Deltas {
				switch {
				case d.Component == world.NaturalLabourOfferTypeID && d.Field == world.NaturalLabourOfferStatusField:
					v, err := d.After.Integer()
					if err != nil {
						return nil, 0, "", err
					}
					if v == int64(world.NaturalRequestExpired) {
						out = append(out, v9LabourEvent{rule: ev.Rule, hour: hour, actor: d.Entity, engagedHour: -1})
					}
				case d.Component == world.NaturalLabourBookTypeID &&
					(d.Field == world.NaturalLabourBookEngagedWorkerField || d.Field == world.NaturalLabourBookEngagedHourField):
				default:
					note("rule-519 delta outside the expiry/book-clear rows", hour, cause)
				}
			}
		}
	}
	return out, violations, firstNote, nil
}

// v9CounterpartyKey is one (employer generation, worker generation) cell of
// the counterparty generation table.
type v9CounterpartyKey struct {
	employerGen int
	workerGen   int
}

// v9GenerationClass names a generation index the frozen way: founders are
// generation 0, everything derived is newborn.
func v9GenerationClass(generation int) string {
	if generation == 0 {
		return "founder"
	}
	return "newborn"
}

// buildV9Metrics computes the additive ticket-20 metric sections from the
// hourly checkpoints, the journal and the accepted events. Everything is
// derived from the same frozen evidence the earlier reporters use (hourly
// projections, journal attempts, accepted event deltas); no earlier report
// field is touched, and the block exists only under natural/9. byID == nil
// means the accepted history was unavailable (post-halt): the attempt-derived
// counters stay exact and the event-derived evidence degrades to -1 with the
// band not evaluable, never to a fabricated count.
func buildV9Metrics(o options, br *branchReport, checks []world.NaturalCheckpoint, journal []world.NaturalBatch, byID map[sim.EventID]kernel.Event) (v9MetricsReport, error) {
	m := v9MetricsReport{
		Definition:                v9TradeDefinition,
		LinkEvidenceAvailable:     byID != nil,
		Accepts:                   -1,
		Refusals:                  -1,
		Expiries:                  -1,
		FirstSettlementHour:       -1,
		LatencyDefinition:         v9LatencyDefinition,
		LatencyMinHours:           -1,
		LatencyMaxHours:           -1,
		ChainDefinition:           v9ChainDefinition,
		TimeToK8Hour:              -1,
		WageLedger:                v9WageLedgerReport{Definition: v9WageLedgerDefinition},
		WageBankedFirstDefinition: v9FirstsDefinition,
		WageMealDefinition:        v9WageMealDefinition,
		EmployerDefinition:        v9EmployerDefinition,
		CounterpartyDefinition:    v9CounterpartyDef,
		Coexistence:               v9CoexistenceReport{Definition: v9CoexistenceDefinition, FirstHour: -1},
		SFDefinition:              v9SFDefinition,
	}
	if len(checks) == 0 {
		return m, errors.New("no hourly projection for v9 metrics")
	}
	patches := len(checks[0].Slots)
	if patches == 0 || o.patches != patches {
		return m, fmt.Errorf("declared patches %d disagree with the projection's %d", o.patches, patches)
	}
	// SF′ is carried verbatim from the frozen v7 computation; SF″ prints
	// beside it and never redefines it.
	v7, err := buildV7Metrics(br, checks, journal)
	if err != nil {
		return m, err
	}
	m.SFPrimeByHourTotal = v7.SFPrimeByHourTotal
	m.SFPrimeRunTotal = v7.SFPrimeRunTotal
	final := checks[len(checks)-1]
	home, generation, states, err := v9RosterIndex(final, patches)
	if err != nil {
		return m, err
	}
	// --- journal attempt pass: the attempt-derived evidence is exact even
	// post-halt (settlement hours for the firsts and the wage-meal reads). ---
	type v9SettleAttempt struct {
		worker sim.EntityID
		hour   int
	}
	type v9EatAttempt struct {
		actor sim.EntityID
		hour  int
	}
	var settleAttempts []v9SettleAttempt
	var eatAttempts []v9EatAttempt
	for _, batch := range journal {
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "labour-settlement":
				settleAttempts = append(settleAttempts, v9SettleAttempt{worker: a.Actor, hour: journalHour(a.Time)})
			case "eat-stored":
				eatAttempts = append(eatAttempts, v9EatAttempt{actor: a.Actor, hour: journalHour(a.Time)})
			}
		}
	}
	m.Offers = br.Events.LabourOffers
	m.Decisions = br.Events.LabourDecisions
	m.Settlements = br.Events.LabourSettlements
	m.Voids = br.Events.LabourVoids
	m.VoidCauses = br.Events.LabourVoidCauses
	// --- checkpoint pass: per-actor wage windows (SF″), the per-patch-hour
	// ledger equality, stock sightings (wage-banked firsts), the employer
	// trajectory, time-to-k=8, and the ticket-19 coexistence cross-check. ---
	type wageWindow struct{ paid, earned []int64 }
	windows := map[sim.EntityID]*wageWindow{}
	sfWindows := map[sim.EntityID][]int64{}
	firstStockSighting := map[sim.EntityID]int{}
	hourActors := make([]map[sim.EntityID]v9HourActor, len(checks))
	var prev map[sim.EntityID]world.NaturalActorState
	delta := func(old, new int64, existed bool) int64 {
		if !existed {
			return new
		}
		return new - old
	}
	trailing := func(win []int64) int64 {
		var sum int64
		for j := len(win) - 1; j >= 0 && len(win)-1-j < sfWindowHours; j-- {
			sum += win[j]
		}
		return sum
	}
	roster := func(c world.NaturalCheckpoint) []struct {
		id sim.EntityID
		a  world.NaturalActorState
	} {
		out := make([]struct {
			id sim.EntityID
			a  world.NaturalActorState
		}, 0, len(c.Founders)+len(c.Newborns))
		for j := range c.Founders {
			out = append(out, struct {
				id sim.EntityID
				a  world.NaturalActorState
			}{sim.EntityID(j + 1), c.Founders[j]})
		}
		for _, nb := range c.Newborns {
			out = append(out, struct {
				id sim.EntityID
				a  world.NaturalActorState
			}{nb.ID, nb.State})
		}
		return out
	}
	m.WageLedger.PatchWagesPaidFinal = make([]int64, patches)
	m.WageLedger.PatchWagesEarnedFinal = make([]int64, patches)
	for _, rs := range roster(final) {
		if home[rs.id] < 0 || home[rs.id] >= patches {
			return m, fmt.Errorf("actor %d home patch outside the declared patch space", int(rs.id))
		}
		m.WageLedger.PatchWagesPaidFinal[home[rs.id]] += rs.a.Granary.WagesPaid
		m.WageLedger.PatchWagesEarnedFinal[home[rs.id]] += rs.a.Granary.WagesEarned
	}
	patchPaidDelta := make([]int64, patches)
	patchEarnedDelta := make([]int64, patches)
	for i, c := range checks {
		actors := make(map[sim.EntityID]v9HourActor, len(c.Founders)+len(c.Newborns))
		for p := range patchPaidDelta {
			patchPaidDelta[p], patchEarnedDelta[p] = 0, 0
		}
		hourBearing, hourPoor := false, false
		var sfDoubleTotal, employerK int64
		for _, rs := range roster(c) {
			a := rs.a
			ha := v9HourActor{alive: naturalAlive(a), k: a.Worksite.Capital, stock: a.Granary.Stock,
				cultivated: a.Granary.CultivatedTotal, wagesPaid: a.Granary.WagesPaid, wagesEarned: a.Granary.WagesEarned,
				energy: a.Body.Energy}
			if home[rs.id] < 0 || home[rs.id] >= patches {
				return m, fmt.Errorf("actor %d home patch outside the declared patch space", int(rs.id))
			}
			p, had := prev[rs.id]
			// The SF′ flow is the frozen v7 form, recomputed here only for
			// the coexistence reading; the reported SF′ series above is the
			// frozen builder's own output.
			cultDelta := delta(p.Granary.CultivatedTotal, a.Granary.CultivatedTotal, had)
			storedDelta := delta(p.Granary.StoredMeals, a.Granary.StoredMeals, had)
			provDelta := delta(p.Granary.Provisioned, a.Granary.Provisioned, had)
			spoilDelta := delta(p.Granary.Spoiled, a.Granary.Spoiled, had)
			paidBirthDelta := delta(p.Genome.BirthGranaryPaid, a.Genome.BirthGranaryPaid, had)
			win := windows[rs.id]
			if win == nil {
				win = &wageWindow{}
				windows[rs.id] = win
			}
			win.paid = append(win.paid, delta(p.Granary.WagesPaid, a.Granary.WagesPaid, had))
			win.earned = append(win.earned, delta(p.Granary.WagesEarned, a.Granary.WagesEarned, had))
			ha.paidWindow = trailing(win.paid)
			ha.earnedWindow = trailing(win.earned)
			sfWin := append(sfWindows[rs.id], cultDelta-storedDelta-provDelta-spoilDelta-paidBirthDelta)
			sfWindows[rs.id] = sfWin
			ha.sfPrime = trailing(sfWin)
			// SF″ = SF′ − wagesPaid + wagesEarned, the same trailing window.
			ha.sfDouble = ha.sfPrime - ha.paidWindow + ha.earnedWindow
			bearing := ha.alive && ha.k >= 2 && ha.stock >= 1 && ha.sfPrime > 0
			poor := ha.alive && ha.k == 0 && ha.stock == 0
			hourBearing = hourBearing || bearing
			hourPoor = hourPoor || poor
			if ha.alive {
				sfDoubleTotal += ha.sfDouble
				if ha.wagesPaid > 0 {
					employerK += ha.k
				}
				if ha.k >= world.NaturalKMax && m.TimeToK8Hour < 0 {
					m.TimeToK8Hour = c.Hour
				}
			}
			if ha.stock >= 1 {
				if _, seen := firstStockSighting[rs.id]; !seen {
					firstStockSighting[rs.id] = c.Hour
				}
			}
			patchPaidDelta[home[rs.id]] += delta(p.Granary.WagesPaid, a.Granary.WagesPaid, had)
			patchEarnedDelta[home[rs.id]] += delta(p.Granary.WagesEarned, a.Granary.WagesEarned, had)
			actors[rs.id] = ha
		}
		hourActors[i] = actors
		for p := 0; p < patches; p++ {
			m.WageLedger.PatchHourChecks++
			if patchPaidDelta[p] != patchEarnedDelta[p] {
				m.WageLedger.PatchHourImbalanceCount++
				if len(m.WageLedger.PatchHourImbalances) < v9MaxRecordedViolations {
					m.WageLedger.PatchHourImbalances = append(m.WageLedger.PatchHourImbalances,
						v9LedgerImbalance{Hour: c.Hour, Patch: world.NaturalPatchIDBase + p, PaidDelta: patchPaidDelta[p], EarnedDelta: patchEarnedDelta[p]})
				}
			}
		}
		if hourBearing && hourPoor {
			m.Coexistence.Hours++
			if m.Coexistence.FirstHour < 0 {
				m.Coexistence.FirstHour = c.Hour
			}
		}
		m.SFDoublePrimeByHourTotal = append(m.SFDoublePrimeByHourTotal, sfDoubleTotal)
		m.EmployerKByHour = append(m.EmployerKByHour, employerK)
		cur := make(map[sim.EntityID]world.NaturalActorState, len(c.Founders)+len(c.Newborns))
		for j := range c.Founders {
			cur[sim.EntityID(j+1)] = c.Founders[j]
		}
		for _, nb := range c.Newborns {
			cur[nb.ID] = nb.State
		}
		prev = cur
	}
	m.SFDoublePrimeRunTotal = m.SFPrimeRunTotal - final.Balance.WagesPaid + final.Balance.WagesEarned
	m.WageLedger.WagesPaidTotal = final.Balance.WagesPaid
	m.WageLedger.WagesEarnedTotal = final.Balance.WagesEarned
	// --- accepted-event pass: typed accept/refuse/expiry evidence with its
	// per-actor attribution, the settlement rows with their reconstructed
	// chains, and the T2 discipline counts. Post-halt this whole pass is
	// skipped and the three event-derived counters stay -1. ---
	acceptsByEmployer := map[sim.EntityID]int64{}
	refusalsByWorker := map[sim.EntityID]int64{}
	expiriesByWorker := map[sim.EntityID]int64{}
	if byID != nil {
		events, fieldViolations, fieldNote, err := v9ParseLabourEvents(byID, patches)
		if err != nil {
			return m, err
		}
		m.FieldSetViolations = fieldViolations
		m.FieldSetViolationNote = fieldNote
		type workerHour struct {
			worker sim.EntityID
			hour   int
		}
		offers := map[workerHour]sim.EntityID{}
		type acceptRecord struct {
			employer sim.EntityID
			hour     int
		}
		acceptsByWorker := map[sim.EntityID][]acceptRecord{}
		var acceptCount, refusalCount, expiryCount int64
		for _, ev := range events {
			switch ev.rule {
			case world.NaturalLabourOfferRule:
				offers[workerHour{ev.actor, ev.hour}] = ev.counterparty
			case world.NaturalLabourDecisionRule:
				if ev.accepted {
					acceptCount++
					acceptsByEmployer[ev.actor]++
					acceptsByWorker[ev.counterparty] = append(acceptsByWorker[ev.counterparty], acceptRecord{employer: ev.actor, hour: ev.hour})
				} else {
					refusalCount++
					refusalsByWorker[ev.counterparty]++
				}
			case world.NaturalLabourSettlementRule:
			case world.NaturalLabourExpireRule:
				expiryCount++
				expiriesByWorker[ev.actor]++
			}
		}
		m.Accepts, m.Refusals, m.Expiries = acceptCount, refusalCount, expiryCount
		// Settlement rows in (hour, worker) order, each with its reconstructed
		// 516→517-accept→518 chain and the T2 violation counts.
		var settleEvents []v9LabourEvent
		for _, ev := range events {
			if ev.rule == world.NaturalLabourSettlementRule {
				settleEvents = append(settleEvents, ev)
			}
		}
		sort.Slice(settleEvents, func(i, j int) bool {
			if settleEvents[i].hour != settleEvents[j].hour {
				return settleEvents[i].hour < settleEvents[j].hour
			}
			return settleEvents[i].actor < settleEvents[j].actor
		})
		for _, se := range settleEvents {
			worker, employer, settleHour := se.actor, se.counterparty, se.hour
			row := v9SettlementRow{
				Worker: int(worker), Employer: int(employer),
				Patch:      world.NaturalPatchIDBase + home[worker],
				SettleHour: settleHour, PointCompleted: se.completed, FoodDelta: se.foodDelta,
				WorkerGeneration: generation[worker], EmployerGeneration: generation[employer],
				OfferHour: -1, AcceptHour: -1, LatencyHours: -1,
			}
			offerHour := settleHour - 1
			addr, hasOffer := offers[workerHour{worker, offerHour}]
			row.OfferHour = offerHour
			row.LatencyHours = settleHour - offerHour
			acceptedHere := false
			for _, rec := range acceptsByWorker[worker] {
				if rec.hour == offerHour {
					acceptedHere = rec.employer == employer
					row.AcceptHour = offerHour
					break
				}
			}
			chainOK := hasOffer && addr == employer && acceptedHere && settleHour-offerHour <= v9ChainWindowHours
			if !chainOK {
				m.ChainViolations++
				if m.ChainViolationNote == "" {
					m.ChainViolationNote = fmt.Sprintf("first chain violation: worker %d→employer %d settled at h%d (offer filed addressed-to-employer=%t, same-hour accept by that employer=%t, within %d h=%t)", int(worker), int(employer), settleHour, hasOffer && addr == employer, acceptedHere, v9ChainWindowHours, settleHour-offerHour <= v9ChainWindowHours)
				}
			}
			m.SettlementRows = append(m.SettlementRows, row)
			if m.LatencyMinHours < 0 || row.LatencyHours < m.LatencyMinHours {
				m.LatencyMinHours = row.LatencyHours
			}
			if row.LatencyHours > m.LatencyMaxHours {
				m.LatencyMaxHours = row.LatencyHours
			}
			m.LatencyTotalHours += row.LatencyHours
			if se.foodDelta != 0 {
				m.SettlementFoodDeltaViolations++
				if m.SettlementFoodDeltaNote == "" {
					m.SettlementFoodDeltaNote = fmt.Sprintf("first nonzero settlement food delta: worker %d→employer %d at h%d delta %+d", int(worker), int(employer), settleHour, se.foodDelta)
				}
			}
			if m.FirstSettlementHour < 0 || settleHour < m.FirstSettlementHour {
				m.FirstSettlementHour = settleHour
			}
		}
		// Zero accepts without a pending addressed offer: every accept needs
		// a same-hour filed offer addressed to that employer, walked in
		// (worker, hour) order so the recorded first note is deterministic.
		// Refused and expired offers are legitimate non-accepts and are not
		// violations.
		type acceptKey struct {
			worker   sim.EntityID
			employer sim.EntityID
			hour     int
		}
		var acceptKeys []acceptKey
		for worker, recs := range acceptsByWorker {
			for _, rec := range recs {
				acceptKeys = append(acceptKeys, acceptKey{worker, rec.employer, rec.hour})
			}
		}
		sort.Slice(acceptKeys, func(i, j int) bool {
			if acceptKeys[i].worker != acceptKeys[j].worker {
				return acceptKeys[i].worker < acceptKeys[j].worker
			}
			if acceptKeys[i].hour != acceptKeys[j].hour {
				return acceptKeys[i].hour < acceptKeys[j].hour
			}
			return acceptKeys[i].employer < acceptKeys[j].employer
		})
		for _, ak := range acceptKeys {
			addr, ok := offers[workerHour{ak.worker, ak.hour}]
			if !ok || addr != ak.employer {
				m.AcceptWithoutOfferViolations++
				if m.AcceptWithoutOfferNote == "" {
					note := "no filed offer"
					if ok {
						note = fmt.Sprintf("the filed offer is addressed to %d", int(addr))
					}
					m.AcceptWithoutOfferNote = fmt.Sprintf("first accept without a pending addressed offer: employer %d engaged worker %d at h%d, %s", int(ak.employer), int(ak.worker), ak.hour, note)
				}
			}
		}
		// Counterparty generation table from the settlement rows.
		counterparties := map[v9CounterpartyKey]*v9CounterpartyRow{}
		for _, row := range m.SettlementRows {
			key := v9CounterpartyKey{row.EmployerGeneration, row.WorkerGeneration}
			c := counterparties[key]
			if c == nil {
				c = &v9CounterpartyRow{EmployerGeneration: key.employerGen, WorkerGeneration: key.workerGen,
					EmployerClass: v9GenerationClass(key.employerGen), WorkerClass: v9GenerationClass(key.workerGen)}
				counterparties[key] = c
			}
			c.Settlements++
			c.WageUnits++
		}
		keys := make([]v9CounterpartyKey, 0, len(counterparties))
		for key := range counterparties {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].employerGen != keys[j].employerGen {
				return keys[i].employerGen < keys[j].employerGen
			}
			return keys[i].workerGen < keys[j].workerGen
		})
		for _, key := range keys {
			m.Counterparties = append(m.Counterparties, *counterparties[key])
		}
	}
	// --- wage-banked firsts (V9-T4): a worker's first granary unit was the
	// wage iff no pulse-boundary stock sighting precedes its first settlement
	// hour (the settlement row covers a same-hour boundary-invisible draw). ---
	firstSettle := map[sim.EntityID]int{}
	for _, sa := range settleAttempts {
		if _, ok := firstSettle[sa.worker]; !ok {
			firstSettle[sa.worker] = sa.hour
		}
	}
	settledIDs := make([]sim.EntityID, 0, len(firstSettle))
	for id := range firstSettle {
		settledIDs = append(settledIDs, id)
	}
	sort.Slice(settledIDs, func(i, j int) bool { return settledIDs[i] < settledIDs[j] })
	for _, id := range settledIDs {
		h0 := firstSettle[id]
		if s, seen := firstStockSighting[id]; !seen || s >= h0 {
			m.WageBankedFirsts++
			m.WageBankedFirstActors = append(m.WageBankedFirstActors, int(id))
		}
	}
	// --- wage meals (V9-T4): eat-stored draws by actors whose lifetime
	// cultivated inflow is 0 and cumulative wages earned >0 at the draw hour;
	// the prior-boundary E≤3 column is the strict subset. ---
	baseHour := checks[0].Hour
	for _, ea := range eatAttempts {
		idx := ea.hour - baseHour
		if idx < 0 || idx >= len(hourActors) {
			continue // outside the recorded hourly projection (fixture pre-history)
		}
		ha, ok := hourActors[idx][ea.actor]
		if !ok {
			continue
		}
		if ha.cultivated == 0 && ha.wagesEarned > 0 {
			m.WageMealDraws++
			if idx > 0 {
				if prior, ok2 := hourActors[idx-1][ea.actor]; ok2 && prior.energy <= 3 {
					m.WageMealDrawsBoundaryE3++
				}
			}
		}
	}
	// --- per-actor and per-patch trade rows. ---
	type actorCounts struct {
		offers, accepts, refusals, expiries, voids, settlements int64
	}
	byActor := map[sim.EntityID]*actorCounts{}
	countsFor := func(id sim.EntityID) *actorCounts {
		c := byActor[id]
		if c == nil {
			c = &actorCounts{}
			byActor[id] = c
		}
		return c
	}
	for _, batch := range journal {
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "labour-offer":
				countsFor(a.Actor).offers++
			case "labour-settlement":
				countsFor(a.Actor).settlements++
			case "labour-void":
				countsFor(a.Actor).voids++
			}
		}
	}
	ids := make([]sim.EntityID, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	patchAgg := map[int]*v9PatchTradeRow{}
	for _, id := range ids {
		c := byActor[id]
		if c == nil {
			c = &actorCounts{}
		}
		row := v9ActorTradeRow{
			ID: int(id), HomePatch: world.NaturalPatchIDBase + home[id], Generation: generation[id],
			Offers: c.offers, Voids: c.voids, Settlements: c.settlements,
			WagesPaid:   states[id].Granary.WagesPaid,
			WagesEarned: states[id].Granary.WagesEarned,
			Accepts:     -1, Refusals: -1, Expiries: -1,
		}
		if byID != nil {
			// Accepts count the employer's accepted 517 engagements; refusals
			// and expiries count the worker's decided and expired offers.
			row.Accepts = acceptsByEmployer[id]
			row.Refusals = refusalsByWorker[id]
			row.Expiries = expiriesByWorker[id]
		}
		m.PerActor = append(m.PerActor, row)
		pr := patchAgg[row.HomePatch]
		if pr == nil {
			pr = &v9PatchTradeRow{Patch: row.HomePatch, Accepts: -1, Refusals: -1, Expiries: -1}
			patchAgg[row.HomePatch] = pr
		}
		pr.Offers += row.Offers
		pr.Voids += row.Voids
		pr.Settlements += row.Settlements
		pr.WagesPaid += row.WagesPaid
		pr.WagesEarned += row.WagesEarned
		if row.Accepts >= 0 {
			if pr.Accepts < 0 {
				pr.Accepts, pr.Refusals, pr.Expiries = 0, 0, 0
			}
			pr.Accepts += row.Accepts
			pr.Refusals += row.Refusals
			pr.Expiries += row.Expiries
		}
	}
	patchIDs := make([]int, 0, len(patchAgg))
	for p := range patchAgg {
		patchIDs = append(patchIDs, p)
	}
	sort.Ints(patchIDs)
	for _, p := range patchIDs {
		m.PerPatch = append(m.PerPatch, *patchAgg[p])
	}
	return m, nil
}

// v9BandByPrefix returns the band whose name carries the frozen prefix, or
// nil when the branch carries no such band.
func v9BandByPrefix(bands []bandReport, prefix string) *bandReport {
	for i := range bands {
		if strings.HasPrefix(bands[i].Name, prefix) {
			return &bands[i]
		}
	}
	return nil
}

// v9BandsFor evaluates the predeclared ticket-20 v9 bands of the frozen
// design verbatim for this branch's cell: T1 trade occurrence, T2 protocol
// discipline, T3 conservation (the three gating discipline bands with T1),
// T4/T5 as the named-null descriptive channels, and T6 ecology
// non-distortion with its cell-specific clause (B9 zero trade attempts at
// R=0; D9 zero accepts under the cultivation ablation; zero settlements on
// the fully saturated founding band; the patch-1001 and death clauses
// everywhere). Band misses are outcomes: reported, never tuned and never
// fatal. Cross-arm clauses (T5's control trajectory, B9's B12 identity) are
// evaluated by the matrix lane across reports and noted in the criterion
// text.
func v9BandsFor(o options, br *branchReport) []bandReport {
	m := br.V9Metrics
	if m == nil {
		return []bandReport{{Name: "V9: metrics missing", Criterion: "the v9 bands require the v9 metric block", Observed: "v9_metrics absent while natural/9 is declared", WithinBand: false}}
	}
	deaths := sumHours(br.Population.DeathsStarvationByHour) + sumHours(br.Population.DeathsAgeByHour)
	settlementsOnPatch1001 := int64(0)
	for _, row := range m.SettlementRows {
		if row.Patch == world.NaturalPatchIDBase {
			settlementsOnPatch1001++
		}
	}
	// T1: trade occurrence (gating). The per-designated-cell aggregation
	// across the cell's matrix runs is the matrix lane's cross-report step.
	t1Within := m.Settlements >= 1 && m.FirstSettlementHour >= 0 && m.FirstSettlementHour <= v9FirstSettlementHourBound
	descriptive := "≤48 descriptive: not met"
	if m.FirstSettlementHour >= 0 && m.FirstSettlementHour <= v9DescriptiveHourBound {
		descriptive = "≤48 descriptive: MET"
	}
	t1 := bandReport{Name: "V9-T1: trade occurrence (gating)", Criterion: "≥1 accepted-and-settled contract in the designated cell with first settlement ≤ h96 (≤48 reported descriptively); zero settlements in all designated cells ⇒ the NULL-TRADE verdict reported verbatim and valid; the per-cell aggregation over seeds/workers is the matrix lane's cross-report step",
		Observed:   fmt.Sprintf("settlements=%d, first_settlement_hour=%d (%s), voids=%d, completed_hour=%d", m.Settlements, m.FirstSettlementHour, descriptive, m.Voids, br.CompletedHour),
		WithinBand: t1Within}
	// T2: protocol discipline (gating).
	var t2 bandReport
	if !m.LinkEvidenceAvailable {
		t2 = bandReport{Name: "V9-T2: protocol discipline (gating)", Criterion: "every settlement's journal chain 516→517-accept→518 within ≤2 h ∧ zero settlements without a matched accepted offer ∧ zero accepts without a pending addressed offer ∧ typed refusal/expire/void counts ∧ the privacy-pinned offer field sets with claim-vs-truth divergence structurally 0",
			Observed:   "not evaluable: the typed-halt halt refuses accepted-history export, so the accepted 516/517/518/519 event evidence is unavailable; journal counts remain exact (offers=" + fmt.Sprint(m.Offers) + ", settlements=" + fmt.Sprint(m.Settlements) + ", voids=" + fmt.Sprint(m.Voids) + ")",
			WithinBand: false}
	} else {
		t2Within := m.ChainViolations == 0 && m.AcceptWithoutOfferViolations == 0 && m.FieldSetViolations == 0
		t2 = bandReport{Name: "V9-T2: protocol discipline (gating)", Criterion: "every settlement's journal chain 516→517-accept→518 within ≤2 h ∧ zero settlements without a matched accepted offer ∧ zero accepts without a pending addressed offer ∧ typed refusal/expire/void counts ∧ the privacy-pinned offer field sets (only {requester, addressee, hour, fixed terms} published; no claim fields exist, so claim-vs-truth divergence is structurally 0 under the field-set pin)",
			Observed:   fmt.Sprintf("settlements=%d, accepts=%d, refusals=%d, expiries=%d, voids=%d (%s), chain_violations=%d, accept_without_offer_violations=%d, field_set_violations=%d", m.Settlements, m.Accepts, m.Refusals, m.Expiries, m.Voids, voidCauseSummary(m.VoidCauses), m.ChainViolations, m.AcceptWithoutOfferViolations, m.FieldSetViolations),
			WithinBand: t2Within}
	}
	// T3: conservation (gating).
	t3Within := br.ConservationVerified && m.WageLedger.PatchHourImbalanceCount == 0 && m.SettlementFoodDeltaViolations == 0
	t3 := bandReport{Name: "V9-T3: conservation (gating)", Criterion: "hourly G1/G2a″/G2b-with-land/G3 green everywhere (the frozen per-hour gate) ∧ per patch-hour Σwages-paid = Σwages-earned ∧ the aggregate food delta of every settlement 0 (any mint or burn is an implementation-defect stop)",
		Observed:   fmt.Sprintf("conservation_green=%t, patch_hours_checked=%d, patch_hour_imbalances=%d, settlement_food_delta_checks=%d, nonzero_deltas=%d", br.ConservationVerified, m.WageLedger.PatchHourChecks, m.WageLedger.PatchHourImbalanceCount, m.Settlements, m.SettlementFoodDeltaViolations),
		WithinBand: t3Within}
	// T4: worker channel (descriptive, named null).
	t4 := bandReport{Name: "V9-T4: worker channel (descriptive, named null)", Criterion: "workers whose first granary≥1 hour coincides with wages earned (the wage-banked firsts) and worker meals drawn at E≤3 from wages; within_band=true reports the channel observed, false is the named null reported verbatim (\"trade occurred, worker channel inert\")",
		Observed:   fmt.Sprintf("wage_banked_firsts=%d (actors %v), wage_meal_draws=%d, prior_boundary_e_le_3_subset=%d", m.WageBankedFirsts, m.WageBankedFirstActors, m.WageMealDraws, m.WageMealDrawsBoundaryE3),
		WithinBand: m.WageBankedFirsts >= 1 || m.WageMealDraws >= 1}
	// T5: employer channel (descriptive, named null).
	finalEmployerK := int64(0)
	if len(m.EmployerKByHour) > 0 {
		finalEmployerK = m.EmployerKByHour[len(m.EmployerKByHour)-1]
	}
	t5 := bandReport{Name: "V9-T5: employer channel (descriptive, named null)", Criterion: "employer Σk and time-to-k=8 against the (v7, natural/7b) control; within_band=true reports time-to-k=8 recorded; the indistinguishable-trajectory null and the trajectory comparison are the matrix lane's cross-report step, reported verbatim",
		Observed:   fmt.Sprintf("time_to_k8_hour=%d, employer_k_final=%d, wages_paid_total=%d", m.TimeToK8Hour, finalEmployerK, m.WageLedger.WagesPaidTotal),
		WithinBand: m.TimeToK8Hour >= 0}
	// T6: ecology non-distortion (gating), with the cell-specific clause.
	var cellName, cellClause string
	cellWithin := true
	switch {
	case br.CultivationDisabled:
		// D9: grants are the only employer stock; the ablation leaves no
		// cultivated slot, so the accept floor is unreachable.
		cellName = "D9 (cultivation ablation × natural/9)"
		cellClause = "D9: zero accepts (grants are the only employer stock; the ablation cultivates nothing, so the stock≥4 accept floor is unreachable)"
		cellWithin = m.Accepts == 0
	case o.r == 0:
		// B9: stock ≡ 0 makes the accept floor unreachable; the frozen clause
		// names zero trade attempts with trajectories identical to the
		// (v7, natural/7b) B12 arm — the identity is the cross-report step.
		cellName = "B9 (R=0 × natural/9)"
		cellClause = "B9: zero trade attempts (offers+decisions+settlements = 0); trajectory identity to the (v7, natural/7b) B12 arm is the matrix lane's cross-report step"
		cellWithin = m.Offers+m.Decisions+m.Settlements == 0
	case o.foundersResolved() == world.NaturalSlotsPerPatch*o.patches:
		// The fully saturated founding band (the f=16 control): every patch
		// saturated, settlements structurally forced to zero (the trade-side
		// A2b mirror).
		cellName = "saturated founding-band control"
		cellClause = "saturated control: zero settlements anywhere (every patch saturated; structurally forced by the accept floor)"
		cellWithin = m.Settlements == 0
	default:
		cellName = "designated trade cell"
		cellClause = "designated trade cell: no extra clause (the patch-1001 and death clauses bind)"
	}
	deathClause := m.Settlements == 0 || deaths >= 1
	patchClause := settlementsOnPatch1001 == 0
	t6Within := deathClause && patchClause && cellWithin
	t6 := bandReport{Name: "V9-T6: ecology non-distortion (gating)", Criterion: "≥1 death before h168 in every settlement-hosting cell (the V8-A5 mirror) ∧ zero settlements on patch 1001 (the saturated patch, in every cell) ∧ " + cellClause,
		Observed:   fmt.Sprintf("cell=%s, deaths_to_horizon=%d (death clause: %t), settlements_on_patch_1001=%d (patch clause: %t), cell clause holds: %t; settlements=%d, births=%d, extinction_hour=%d", cellName, deaths, deathClause, settlementsOnPatch1001, patchClause, cellWithin, m.Settlements, br.Totals.Births, br.Population.ExtinctionHour),
		WithinBand: t6Within}
	return []bandReport{t1, t2, t3, t4, t5, t6}
}

// voidCauseSummary renders the typed void-cause histogram for band text.
func voidCauseSummary(causes map[string]int64) string {
	if len(causes) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(causes))
	for k := range causes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, causes[k]))
	}
	return strings.Join(parts, ",")
}

// v9VerdictFor applies the frozen ticket-20 verdict logic to the run's
// branches: SUPPORTED iff T1∧T2∧T3∧T6 IN and (T4∨T5) observed; NULL-TRADE
// iff T2/T3/T6 IN with zero settlements; FAILED iff any gating discipline
// band is OUT. The corner the three frozen implications do not partition
// (gates IN, settlements > 0, both descriptive channels null) carries the
// label the ticket-20 record declared pre-run, TRADE-INERT, with the
// rationale verbatim.
func v9VerdictFor(r report) *v9VerdictReport {
	v := &v9VerdictReport{FirstSettlementHour: -1, T1In: true, T2In: true, T3In: true, T6In: true}
	var missing []string
	for i := range r.Branches {
		br := &r.Branches[i]
		if br.V9Metrics == nil || len(br.V9Bands) == 0 {
			missing = append(missing, fmt.Sprintf("births=%t branch carries no v9 evidence", br.Births))
			v.T1In, v.T2In, v.T3In, v.T6In = false, false, false, false
			continue
		}
		m := br.V9Metrics
		for _, gate := range []struct {
			prefix string
			flag   *bool
		}{
			{"V9-T1", &v.T1In}, {"V9-T2", &v.T2In}, {"V9-T3", &v.T3In}, {"V9-T6", &v.T6In},
		} {
			if b := v9BandByPrefix(br.V9Bands, gate.prefix); b == nil || !b.WithinBand {
				*gate.flag = false
			}
		}
		v.T4Observed = v.T4Observed || m.WageBankedFirsts >= 1 || m.WageMealDraws >= 1
		v.T5Observed = v.T5Observed || m.TimeToK8Hour >= 0
		v.Settlements += m.Settlements
		if m.FirstSettlementHour >= 0 && (v.FirstSettlementHour < 0 || m.FirstSettlementHour < v.FirstSettlementHour) {
			v.FirstSettlementHour = m.FirstSettlementHour
		}
	}
	gates := v.T1In && v.T2In && v.T3In && v.T6In
	discipline := v.T2In && v.T3In && v.T6In
	switch {
	case !v.T1In && !v.T2In && !v.T3In && !v.T6In && len(missing) > 0:
		v.Verdict = v9VerdictFailed
		v.Rationale = "FAILED: " + strings.Join(missing, "; ")
	case discipline && v.Settlements == 0:
		v.Verdict = v9VerdictNullTrade
		v.Rationale = fmt.Sprintf("NULL-TRADE, reported verbatim and valid: T2∧T3∧T6 IN with zero settlements across the run's branches (T1 OUT by the same zero; worker channel observed=%t, employer channel observed=%t)", v.T4Observed, v.T5Observed)
	case !gates:
		v.Verdict = v9VerdictFailed
		v.Rationale = fmt.Sprintf("FAILED: a gating discipline band is OUT (T1=%t, T2=%t, T3=%t, T6=%t; settlements=%d, first_settlement_hour=%d)", v.T1In, v.T2In, v.T3In, v.T6In, v.Settlements, v.FirstSettlementHour)
	case v.T4Observed || v.T5Observed:
		v.Verdict = v9VerdictSupported
		v.Rationale = fmt.Sprintf("SUPPORTED: T1∧T2∧T3∧T6 IN with settlements=%d (first at h%d) and a channel observed (T4=%t, T5=%t)", v.Settlements, v.FirstSettlementHour, v.T4Observed, v.T5Observed)
	default:
		v.Verdict = v9VerdictTradeInert
		v.Rationale = fmt.Sprintf("TRADE-INERT (declared pre-run): the gates are IN with settlements=%d (first at h%d) but neither descriptive channel observed evidence (T4=%t, T5=%t) — trade occurred and was correctly recorded, yet neither channel produced a measurable effect; the named nulls inside T4/T5 carry the outcome verbatim", v.Settlements, v.FirstSettlementHour, v.T4Observed, v.T5Observed)
	}
	return v
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.NaturalHandoff, mech world.NaturalMechanism) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.NaturalRegistry()
	if mech != world.NaturalMechanismV5 {
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

// validLabourVoidCause reports whether a labour-void note is inside the
// frozen taxonomy: the seven NaturalLabourSettle contract refusals plus the
// two runner-level structural notes (the completion-instant settlement
// precedence and the engagement-busy E12-class world refusal of a policy
// accept). An unknown note is a hard error, never silently bucketed.
func validLabourVoidCause(note string) bool {
	switch note {
	case "contract-not-pending", "worker-dead-or-cross-patch", "employer-dead-or-cross-patch",
		"worker-bag-empty", "employer-stock-short", "employer-at-kmax", "no-free-slot",
		"settlement-precedence", "engagement-busy":
		return true
	}
	return false
}

// labourDecisionAcceptEvent reports whether an accepted rule-517 event wrote
// the employer's engagement row (an accept) or status rows only (a typed
// refusal) — the 512-shape discriminator.
func labourDecisionAcceptEvent(ev kernel.Event) bool {
	for _, d := range ev.Deltas {
		if d.Component == world.NaturalLabourBookTypeID && d.Field == world.NaturalLabourBookEngagedWorkerField {
			return true
		}
	}
	return false
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
			case "labour-offer":
				// The ticket-20 rule-516 filing: keyed, always committed.
				br.Events.LabourOffers++
			case "labour-decision":
				// The ticket-20 rule-517 decision: the accept/refuse split
				// reads the accepted event's engagement-row write, so it is
				// filled only when the accepted history was available.
				br.Events.LabourDecisions++
				if byID != nil && a.Accepted {
					if ev, ok := byID[a.EventID]; ok && labourDecisionAcceptEvent(ev) {
						br.Events.LabourAccepts++
					} else {
						br.Events.LabourRefusals++
					}
				}
			case "labour-settlement":
				// The ticket-20 committed rule-518 atomic wage settlement.
				br.Events.LabourSettlements++
			case "labour-void":
				// The ticket-20 typed zero-mutation settlement refusal (a
				// contract-taxonomy reason, the completion-instant fold's
				// settlement-precedence note, or the engagement-busy world
				// refusal of a policy accept).
				if !validLabourVoidCause(a.Refused) {
					return fmt.Errorf("unknown labour-void cause %q", a.Refused)
				}
				br.Events.LabourVoids++
				if br.Events.LabourVoidCauses == nil {
					br.Events.LabourVoidCauses = map[string]int64{}
				}
				br.Events.LabourVoidCauses[a.Refused]++
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
func populationEvidence(br *branchReport, checks []world.NaturalCheckpoint, mech world.NaturalMechanism, yield int64) error {
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
		// Independent frozen G1/G2a(/G2a′/G2a″)/G2b/G3 re-verification at every
		// hour: the frozen v5 signature under the v5 zero value (its bytes
		// frozen), the mechanism-aware extended form under v6 (G2a′) and v7
		// (G2a″, the ticket-20 wage terms).
		var balance world.NaturalBalance
		var err error
		if mech != world.NaturalMechanismV5 {
			balance, err = world.NaturalCheckConservationYieldFor(mech, yield, len(c.Patches), c.Patches, c.Slots, c.Founders, c.Newborns, c.AllocatorLast)
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
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Yield: o.yield, BirthCost: o.birthCost, Seed: o.seed, Workers: o.workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech,
		Founders: o.founders}
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
	var eventsByID map[sim.EventID]kernel.Event
	if err := populationEvidence(&br, checks, mech, o.yieldResolved()); err != nil {
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
		eventsByID, err = historyEvidence(&br, handoff, mech)
		if err != nil {
			return br, err
		}
		if err := classifyJournal(&br, f.Journal(), eventsByID); err != nil {
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
	// The additive v6/v7/v8 sections are computed after the cost clock stops:
	// pure post-processing of the same frozen evidence, never a v5 field.
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
	if v8Requested(o) {
		vm, err := buildV8Metrics(o, &br, checks, f.Journal())
		if err != nil {
			return br, err
		}
		br.V8Metrics = &vm
	}
	if v9Requested(o) {
		vm, err := buildV9Metrics(o, &br, checks, f.Journal(), eventsByID)
		if err != nil {
			return br, err
		}
		br.V9Metrics = &vm
	}
	br.Band = bandFor(o, &br)
	br.CostBudget = costBudget(o, &br)
	if v6Requested(o) {
		br.V6Bands = v6BandsFor(o, &br)
	}
	if v7Requested(o) {
		br.V7Bands = v7BandsFor(o, &br)
	}
	if v8Requested(o) {
		br.V8Bands = v8BandsFor(o, &br)
	}
	if v9Requested(o) {
		br.V9Bands = v9BandsFor(o, &br)
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
	expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Yield: o.yield, BirthCost: o.birthCost, Seed: o.seed, Workers: workers,
		Births: births, FounderTenure: fixture, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech,
		Founders: o.founders}
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
	// directory (the frozen world-side ladder naturalRefLegalUnderMechanism
	// refuses the same combinations at construction).
	if !legalPolicyUnderMechanism(policyName, mech) {
		return report{}, fmt.Errorf("policy %s is not legal under -mechanism=%s (natural/7 and natural/7b require v6 or v7; natural/9 requires v7: the wage contract needs rules 516-519)", policyName, mechanismName)
	}
	revision := fmt.Sprintf("natural format-v5/schema-v5/rule-v5/projection-v5/policy %s", policyName)
	switch mech {
	case world.NaturalMechanismV6:
		revision = fmt.Sprintf("natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy %s", policyName)
	case world.NaturalMechanismV7:
		revision = fmt.Sprintf("natural mechanism-v7/format-v7/schema-v7/rule-v7/projection-v7/policy %s", policyName)
	case world.NaturalMechanismV8:
		revision = fmt.Sprintf("natural mechanism-v8/format-v8/schema-v8/rule-v8/projection-v8/policy %s", policyName)
	case world.NaturalMechanismV9:
		revision = fmt.Sprintf("natural mechanism-v9/format-v9/schema-v9/rule-v9/projection-v9/policy %s", policyName)
	case world.NaturalMechanismV10:
		revision = fmt.Sprintf("natural mechanism-v10/format-v10/schema-v10/rule-v10/projection-v10/policy %s", policyName)
	case world.NaturalMechanismV11:
		revision = fmt.Sprintf("natural mechanism-v11/format-v11/schema-v11/rule-v11/projection-v11/policy %s", policyName)
	case world.NaturalMechanismV12:
		revision = fmt.Sprintf("natural mechanism-v12/format-v12/schema-v12/rule-v12/projection-v12/policy %s", policyName)
	case world.NaturalMechanismV13:
		revision = fmt.Sprintf("natural mechanism-v13/format-v13/schema-v13/rule-v13/projection-v13/policy %s", policyName)
	}
	r := report{
		Status:   "synthetic/model-conditional; frozen natural-limits v5 fixture; no historical calibration, takeoff, or P0 scale claim",
		Revision: revision,
		Patches:  o.patches, R: o.r, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: policyName, Mechanism: mechanismReportName(mechanismName), BranchSelection: o.births, FounderTenure: o.fixture,
		CultivationDisabled: o.cultivationDisabled, TokenCost: 0,
	}
	founders := o.foundersResolved()
	r.Founders = founders
	if o.workers < 1 || o.patches < 1 || o.patches > world.NaturalMaxPatchesFixedIDSpace ||
		o.r < 0 || o.r > world.NaturalSlotsPerPatch || o.hours < 1 || o.hours > world.NaturalHorizonHours {
		return r, errors.New("invalid fixture flags")
	}
	// The founding band is range-checked through the frozen world-side bound
	// before any publication: an out-of-band -founders value leaves no output
	// file and no checkpoint directory.
	if !world.NaturalFoundersValid(o.patches, o.founders) {
		return r, fmt.Errorf("invalid -founders value %d: the derived founding-band bound is [1,%d] at -patches=%d (0 = the default spelling of %d)", o.founders, world.NaturalSlotsPerPatch*o.patches, o.patches, world.NaturalSlotsPerPatch*o.patches)
	}
	if o.fixture && o.founders == 1 {
		return r, errors.New("founder-tenure fixture at founders 1 is not a declared scenario (the fixture pre-history seats founders 1 and 2)")
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
		// so every continuation of this root is pinned to it. The genesis is
		// built through the ticket-18 general seam (NewNaturalFixtureForConfig),
		// which carries the full declared identity — policy ref, mechanism
		// and founding band; the earlier ticket-16/17 wrappers stay
		// delegating shims over it with the default band, whose bytes are
		// frozen and digest-pinned.
		fx, err := world.NewNaturalFixtureForConfig(o.patches, o.r, o.seed, true, 1, ref, mech, o.founders)
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
		root, err := world.NewNatural(world.NaturalOptions{Patches: o.patches, R: o.r, Yield: o.yield, BirthCost: o.birthCost, Seed: o.seed,
			Workers: 1, Births: false, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech,
			Founders: o.founders})
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
			expected := world.NaturalRestoreOptions{Patches: o.patches, R: o.r, Yield: o.yield, BirthCost: o.birthCost, Seed: o.seed, Workers: 1,
				Births: false, CultivationDisabled: o.cultivationDisabled, PolicyRef: ref, Mechanism: mech,
				Founders: o.founders}
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
	if v9Requested(o) {
		r.V9Verdict = v9VerdictFor(r)
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
	policy := flag.String("policy", policy5Name, "frozen policy ref (ticket-16 policy-selection seam): natural/5 (default, byte-compatible with every recorded run), natural/6, the ticket-17 provision arms natural/7 and natural/7b (both require -mechanism=v6 or v7), or the ticket-20 wage-contract arm natural/9 (requires -mechanism=v7)")
	mechanism := flag.String("mechanism", mechanismV5Name, "declared world mechanism axis (ticket-17, ticket-20, ticket-50): v5 (default, the frozen v5 world, byte-compatible with every recorded run), v6 (the rule-515 provision world), v7 (the wage-contract world, rules 516-519), v8 (the band-wide mate pool and household consent gate), v9 (v8 plus the declared granary yield), v10 (round-robin founding band), v11 (yield-scaled wild fill), v12 (the two-energy meal), or v13 (v12 plus the declared birth price)")
	yield := flag.Int64("yield", 0, "mechanism-v9 declared granary yield: units granted per cultivated slot per patch-hour (0 = the frozen flat grant of 1; 1..8 admitted under mechanism v9 and later)")
	birthCost := flag.Int64("birth-cost", 0, "mechanism-v13 declared birth price: granary units and energy units paid by each paying parent per birth (0 = the frozen price of 2; 1..8 admitted under mechanism v13 only)")
	founders := flag.Int("founders", 0, "declared founding band (ticket-18 fixture axis, run identity): founder rows materialized from the frozen founder-stream prefix; 0 = the default spelling of the full block 8·patches, otherwise 1..8·patches, range-checked before any publication")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the root and branch bundles; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	rep, err := run(context.Background(), options{patches: *patches, r: *r, seed: *seed, workers: *workers, hours: *hours,
		births: *births, fixture: *fixture, cultivationDisabled: *cultivationDisabled, policy: *policy, mechanism: *mechanism,
		yield: *yield, birthCost: *birthCost, founders: *founders, out: *out, checkpointDir: *checkpointDir})
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
