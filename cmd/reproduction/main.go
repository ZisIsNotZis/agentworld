// Command reproduction runs the frozen v4 reproduction-and-genetics pilot.
// It publishes a verified neutral h0 v4 root captured with BIRTHS DISABLED
// (the v4 h0 pulse is births-asymmetric by frozen design, so a disabled root
// is the only legal branch parent), records the recorded interventions as
// branch children sharing the parent digest — births-disabled (same-flag
// lineage) and the births-ENABLE intervention (the reverse direction is
// refused by the persistence layer) — continues each child to the requested
// hourly horizon, and reports per branch: the population trajectory
// (alive-by-hour with births and cause-attributed deaths per hour and the
// extinction hour), genome diversity (integer-exact mean pairwise L1,
// distinct genomes, per-locus allele frequencies at the frozen hours),
// kinship density (related living-pair fraction, mean finite kin distance,
// per-actor living children, inbreeding-refusal count), birth economics
// (births, filings, refusal histogram, privileged claim-vs-truth
// divergences), per-actor ledgers with genomes and lineage, typed event
// counts, and measured costs. The optional -genome-variant flag selects the
// recorded founder fixture (actors 1-8 locus-m=7, actors 9-16 locus-m=5) as
// a runner option — a declared founder-allele override, not a policy
// change; variant cells are births-enabled-only in the frozen matrix. The
// optional -v3-reference flag compares a births-disabled child against a
// recorded ticket-13 capacity report on the v3-visible trajectories. Its
// reports are synthetic/model-conditional diagnostics of the frozen
// fixture: no historical calibration, no takeoff claim, and no P0 scale
// extrapolation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

const (
	neutralBundleName  = "reproduction-neutral-h0.bundle"
	enabledBundleName  = "reproduction-branch-enabled.bundle"
	disabledBundleName = "reproduction-branch-disabled.bundle"
	enabledBranchFlag  = "enabled"
	disabledBranchFlag = "disabled"
	bothBranchesFlag   = "both"
)

// genomeVariantLoci is the recorded genome-variant fixture of the frozen
// matrix: actors 1-8 carry locus-m allele 7, actors 9-16 allele 5; loci y/w
// and every lifetime stay on the frozen seed derivation. Declared founder
// alleles are a runner option (ReproductionOptions.FounderLocusM), not a
// policy change; the declared set is part of run identity, pinned by the
// checkpoint manifest and journal verification.
func genomeVariantLoci() []int64 {
	loci := make([]int64, world.ReproductionFounderCount)
	for i := range loci {
		if i < 8 {
			loci[i] = 7
		} else {
			loci[i] = 5
		}
	}
	return loci
}

// diversityHours are the frozen per-locus frequency sample hours; samples
// clamp to the completed horizon.
var diversityHours = [...]int{1, 4, 12, 24, 72, 96, 120, 144, 168}

type options struct {
	yield         int64
	seed          uint64
	workers       int
	hours         int
	births        string
	genomeVariant bool
	out           string
	v3Reference   string
	checkpointDir string
}

type checkpointInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type actorFinal struct {
	ID                 int   `json:"id"`
	Alive              bool  `json:"alive"`
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
	YieldTotal         int64 `json:"yield_total"`
	YieldUnrealized    int64 `json:"yield_unrealized"`
	StoredMeals        int64 `json:"stored_meals"`
	LastStoredMealHour int64 `json:"last_stored_meal_hour"`
	LocusM             int64 `json:"locus_m"`
	LocusY             int64 `json:"locus_y"`
	LocusW             int64 `json:"locus_w"`
	ParentA            int   `json:"parent_a"`
	ParentB            int   `json:"parent_b"`
	BirthHour          int64 `json:"birth_hour"`
	DeathHour          int64 `json:"death_hour"`          // immutable lifetime bound from creation
	DiedHour           int64 `json:"died_hour"`           // -1 while alive at the final hour
	ObservedDeathHour  int   `json:"observed_death_hour"` // first projection hour not alive; -1 if alive throughout
	BirthGranaryPaid   int64 `json:"birth_granary_paid"`
	LivingChildren     int   `json:"living_children"`
}

// totalsReport carries the aggregate frozen ledgers of the final hour;
// total_wear_debt sums the per-actor transient wear debts.
type totalsReport struct {
	Alive            int   `json:"alive"`
	Produced         int64 `json:"total_produced"`
	Unrealized       int64 `json:"total_patch_unrealized"`
	Gathered         int64 `json:"total_gathered"`
	Consumed         int64 `json:"total_wild_meals"`
	Stock            int64 `json:"total_patch_stock"`
	Held             int64 `json:"total_held"`
	Invested         int64 `json:"total_invested_units"`
	YieldTotal       int64 `json:"total_yield"`
	YieldUnrealized  int64 `json:"total_yield_unrealized"`
	GranaryStock     int64 `json:"total_granary"`
	StoredMeals      int64 `json:"total_stored_meals"`
	Capital          int64 `json:"total_k"`
	Wip              int64 `json:"total_wip"`
	PointsCreated    int64 `json:"total_points_created"`
	PointsDecayed    int64 `json:"total_points_decayed"`
	InitialEnergy    int64 `json:"total_initial_energy"`
	Energy           int64 `json:"total_energy"`
	BasalSpent       int64 `json:"total_basal_spent"`
	EnergyCapLost    int64 `json:"total_energy_cap_lost"`
	BirthGranaryPaid int64 `json:"total_birth_granary_paid"`
	Births           int64 `json:"total_births"`
	TotalWearDebt    int64 `json:"total_wear_debt"`
}

// int64x3 converts one frozen metrics frequency row to the report type.
func int64x3(v [3]int) [3]int64 { return [3]int64{int64(v[0]), int64(v[1]), int64(v[2])} }

// populationReport is the frozen trajectory: alive-by-hour (alive iff
// died-hour == -1 and energy > 0), births and cause-attributed deaths per
// hour, and the extinction hour (-1 while alive at the final hour).
type populationReport struct {
	AliveByHour            []int   `json:"alive_by_hour"`
	BirthsByHour           []int64 `json:"births_by_hour"`
	DeathsStarvationByHour []int64 `json:"deaths_starvation_by_hour"`
	DeathsAgeByHour        []int64 `json:"deaths_age_by_hour"`
	FinalAlive             int     `json:"final_alive"`
	SurvivorIDs            []int   `json:"survivor_ids"`
	ExtinctionHour         int     `json:"extinction_hour"`
}

type locusFrequencies struct {
	Hour   int      `json:"hour"`
	LocusM [3]int64 `json:"locus_m"` // allele 5,6,7 counts over the living
	LocusY [3]int64 `json:"locus_y"`
	LocusW [3]int64 `json:"locus_w"`
}

// genomeDiversityReport is the frozen diversity metric: the integer-exact
// mean pairwise L1 over living actors (SumPairwiseL1/GenomePairs, each
// distance in [0,6]), distinct genomes, and per-locus allele frequencies at
// the frozen sample hours present in the completed horizon.
type genomeDiversityReport struct {
	SumPairwiseL1    int64              `json:"sum_pairwise_l1"`
	GenomePairs      int64              `json:"genome_pairs"`
	MeanPairwiseL1   float64            `json:"mean_pairwise_l1"`
	DistinctGenomes  int                `json:"distinct_genomes"`
	LocusFrequencies []locusFrequencies `json:"locus_frequencies"`
}

type actorChildren struct {
	ID             int `json:"id"`
	LivingChildren int `json:"living_children"`
}

// kinshipReport is the derived kinship density over the living roster at the
// final hour: related living-pair fraction within the ≤10-ref walk, the mean
// finite walk distance over related pairs, and the cumulative typed
// inbreeding-refusal count. Kinship derives only from genome parent rows.
type kinshipReport struct {
	LivingPairs            int64           `json:"living_pairs"`
	KinPairs               int64           `json:"kin_pairs"`
	RelatedPairFraction    float64         `json:"related_pair_fraction"`
	MeanFiniteKinDistance  float64         `json:"mean_finite_kin_distance"`
	InbreedingRefusals     int64           `json:"inbreeding_refusals"`
	PerActorLivingChildren []actorChildren `json:"per_actor_living_children"`
}

// birthEconomicsReport is the frozen birth economics: committed births,
// filings, the typed refusal histogram, and the privileged claim-vs-truth
// divergence count (never actor-visible).
type birthEconomicsReport struct {
	Births        int64            `json:"births"`
	BirthRequests int64            `json:"birth_requests"`
	Divergences   int64            `json:"claim_vs_truth_divergences"`
	Refusals      map[string]int64 `json:"refusals"`
}

type eventCounts struct {
	GatherClaims        int64 `json:"gather_claims"`
	GatherAdmitted      int64 `json:"gather_admitted"`
	GatherDenied        int64 `json:"gather_denied"`
	Eat                 int64 `json:"eat"`
	EatStored           int64 `json:"eat_stored"`
	Wait                int64 `json:"wait"`
	Rest                int64 `json:"rest"`
	Build               int64 `json:"builds"`
	PairedMeals         int64 `json:"paired_meals"`
	PairedNoMealNotes   int64 `json:"paired_meal_no_meal_notes"`
	FallbackEatStored   int64 `json:"fallback_eatstored"`
	FallbackNoMealNotes int64 `json:"fallback_no_meal_notes"`
	BirthRequests       int64 `json:"birth_requests"`
	BirthReplies        int64 `json:"birth_replies"`
	BirthsAccepted      int64 `json:"births_accepted"`
	BirthRefusals       int64 `json:"birth_refusals"`
	BirthRefusalsSilent int64 `json:"birth_refusals_silent"`
	BirthWakeClaims     int64 `json:"birth_wake_claims"`
	LostBirths          int64 `json:"lost_births"`
}

type branchReport struct {
	Births                    bool                  `json:"births"`
	GenomeVariant             bool                  `json:"genome_variant"`
	ParentDigest              string                `json:"parent_digest"`
	BranchDigest              string                `json:"branch_digest"`
	BranchCheckpoint          checkpointInfo        `json:"branch_checkpoint"`
	CompletedHour             int                   `json:"completed_hour"`
	Population                populationReport      `json:"population"`
	Actors                    []actorFinal          `json:"actors"`
	Totals                    totalsReport          `json:"totals"`
	GenomeDiversity           genomeDiversityReport `json:"genome_diversity"`
	Kinship                   kinshipReport         `json:"kinship"`
	BirthEconomics            birthEconomicsReport  `json:"birth_economics"`
	Events                    eventCounts           `json:"event_counts"`
	ConservationVerified      bool                  `json:"conservation_verified"`
	EventCount                int                   `json:"events"`
	EventBodyBytes            int                   `json:"event_body_bytes"`
	AcceptedLinksChecked      int                   `json:"accepted_event_links_checked"`
	UnlinkedRejectionsChecked int                   `json:"unlinked_rejections_checked"`
	HistoryBytes              int                   `json:"history_bytes"`
	JournalBytes              int                   `json:"journal_bytes"`
	WallTimeNS                int64                 `json:"wall_time_ns"`
	CPUTimeNS                 int64                 `json:"cpu_time_ns"`
	PeakRSSBytes              int64                 `json:"peak_rss_bytes"`
	TokenCost                 int                   `json:"token_cost"`
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
	BirthsEnabled      int64             `json:"births_enabled"`
	BirthsDisabled     int64             `json:"births_disabled"`
}

type v3EqualityMismatch struct {
	Hour int `json:"hour"`
	V4   int `json:"v4"`
	V3   int `json:"v3"`
}

type v3EqualityActorMismatch struct {
	Actor int    `json:"actor"`
	Field string `json:"field"`
	V4    int64  `json:"v4"`
	V3    int64  `json:"v3"`
}

// v3EqualityReport records the equality-fixture comparison of a
// births-disabled v4 child against a recorded ticket-13 capacity report:
// alive-by-hour through the common horizon, plus the full per-actor and
// aggregate v3-visible final state when the horizons match (q0's h12).
type v3EqualityReport struct {
	ReferencePath          string                    `json:"reference_path"`
	ReferenceCompletedHour int                       `json:"reference_completed_hour"`
	ReferenceBranchEnabled bool                      `json:"reference_branch_enabled"`
	ComparedThroughHour    int                       `json:"compared_through_hour"`
	AliveHoursCompared     int                       `json:"alive_hours_compared"`
	AliveEqual             bool                      `json:"alive_equal"`
	AliveMismatches        []v3EqualityMismatch      `json:"alive_mismatches"`
	FinalStateCompared     bool                      `json:"final_state_compared"`
	FinalStateEqual        bool                      `json:"final_state_equal"`
	ActorMismatches        []v3EqualityActorMismatch `json:"actor_mismatches"`
	TotalsMismatches       []v3EqualityActorMismatch `json:"totals_mismatches"`
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
	Births            string            `json:"branch_selection"`
	GenomeVariant     bool              `json:"genome_variant"`
	GenomeVariantLoci []int64           `json:"genome_variant_loci,omitempty"`
	V3Reference       *v3EqualityReport `json:"v3_equality,omitempty"`
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

func digestString(d [32]byte) string { return fmt.Sprintf("%x", d) }

// prepareCheckpointDir creates the bundle directory. An existing path is never
// reused or populated: all published artifacts land at previously absent paths.
func prepareCheckpointDir(path string) (string, error) {
	if path == "" {
		return os.MkdirTemp("", "reproduction-checkpoint-")
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
func refusalName(r world.ReproductionRefusalReason) (string, error) {
	switch r {
	case world.ReproductionRefusalNone:
		return "none", nil
	case world.ReproductionRefusalNotPending:
		return "not_pending", nil
	case world.ReproductionRefusalDuplicateReply:
		return "duplicate_reply", nil
	case world.ReproductionRefusalAddresseeDead:
		return "addressee_dead", nil
	case world.ReproductionRefusalProposerDead:
		return "proposer_dead", nil
	case world.ReproductionRefusalConsentOwn:
		return "consent_own", nil
	case world.ReproductionRefusalConsentClaims:
		return "consent_claims", nil
	case world.ReproductionRefusalKin:
		return "kin", nil
	case world.ReproductionRefusalBirthCap:
		return "birth_cap", nil
	case world.ReproductionRefusalCrossPatch:
		return "cross_patch", nil
	case world.ReproductionRefusalGateThresholds:
		return "gate_thresholds", nil
	default:
		return "", fmt.Errorf("unknown birth refusal reason %d", r)
	}
}

// birthEvidenceKind reports whether a journal attempt kind is birth
// evidence. A births-disabled branch must record none of them, and q0 must
// record none on either branch.
func birthEvidenceKind(kind string) bool {
	switch kind {
	case "birth-request", "birth-reply":
		return true
	default:
		return false
	}
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.ReproductionHandoff) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.ReproductionRegistry()
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
		if br.Births {
			continue
		}
		// A births-disabled branch commits no birth event of any rule.
		switch ev.Rule {
		case world.ReproductionBirthRequestRule, world.ReproductionBirthReplyRule,
			world.ReproductionBirthRefuseRule, world.ReproductionBirthExpireRule:
			return nil, fmt.Errorf("births-disabled branch committed birth event rule %d", ev.Rule)
		}
	}
	return byID, nil
}

// reproductionParentLookup derives the kinship walk input from the final
// roster's genome parent rows only.
func reproductionParentLookup(actors map[sim.EntityID]world.ReproductionActorState) world.ReproductionParentLookup {
	return func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		a, ok := actors[id]
		if !ok {
			return 0, 0, false
		}
		return a.Genome.ParentA, a.Genome.ParentB, true
	}
}

// branchEvidence cross-checks the journal against the hourly projection and
// the restored accepted history, then assembles the branch report.
func branchEvidence(br *branchReport, f *world.Reproduction, byID map[sim.EventID]kernel.Event, births bool, o options) error {
	checks := f.Checkpoints()
	if len(checks) != o.hours+1 || checks[o.hours].Hour != o.hours {
		return errors.New("missing hourly projection")
	}
	pop := &br.Population
	pop.AliveByHour = make([]int, len(checks))
	pop.BirthsByHour = make([]int64, len(checks))
	pop.DeathsStarvationByHour = make([]int64, len(checks))
	pop.DeathsAgeByHour = make([]int64, len(checks))
	pop.ExtinctionHour = -1
	for h, c := range checks {
		if c.Hour != h {
			return fmt.Errorf("hour sequence break at %d", h)
		}
		pop.AliveByHour[h] = c.Metrics.Alive
		if h == 0 {
			pop.BirthsByHour[h] = c.Metrics.Births
			pop.DeathsStarvationByHour[h] = c.Metrics.DeathsStarvation
			pop.DeathsAgeByHour[h] = c.Metrics.DeathsAge
		} else {
			prev := checks[h-1].Metrics
			pop.BirthsByHour[h] = c.Metrics.Births - prev.Births
			pop.DeathsStarvationByHour[h] = c.Metrics.DeathsStarvation - prev.DeathsStarvation
			pop.DeathsAgeByHour[h] = c.Metrics.DeathsAge - prev.DeathsAge
		}
		if pop.AliveByHour[h] == 0 && pop.ExtinctionHour < 0 {
			pop.ExtinctionHour = h
		}
		// The frozen G1-G3 extended gates, re-verified independently at
		// every hour.
		balance, err := world.ReproductionCheckConservation(c.Patches, c.Slots, c.Founders, c.Newborns)
		if err != nil || balance != c.Balance {
			return fmt.Errorf("hour %d conservation failure: %v", h, err)
		}
	}
	br.ConservationVerified = true
	if pop.ExtinctionHour >= 0 {
		for h := pop.ExtinctionHour; h < len(pop.AliveByHour); h++ {
			if pop.AliveByHour[h] != 0 {
				return fmt.Errorf("population resurrected after extinction at h%d", h)
			}
		}
	}
	// Diversity at the frozen sample hours present in the completed horizon.
	for _, h := range diversityHours {
		if h > o.hours {
			continue
		}
		m := checks[h].Metrics
		br.GenomeDiversity.SumPairwiseL1 = m.SumPairwiseL1
		br.GenomeDiversity.GenomePairs = m.GenomePairs
		br.GenomeDiversity.DistinctGenomes = m.DistinctGenomes
		if m.GenomePairs > 0 {
			br.GenomeDiversity.MeanPairwiseL1 = float64(m.SumPairwiseL1) / float64(m.GenomePairs)
		}
		br.GenomeDiversity.LocusFrequencies = append(br.GenomeDiversity.LocusFrequencies,
			locusFrequencies{Hour: h, LocusM: int64x3(m.LocusFrequencies[0]), LocusY: int64x3(m.LocusFrequencies[1]), LocusW: int64x3(m.LocusFrequencies[2])})
	}
	// Final-hour ledgers, kinship density and per-actor metrics.
	final := checks[o.hours]
	br.Totals = totalsReport{
		Alive: final.Metrics.Alive, Produced: final.Balance.Produced, Unrealized: final.Balance.Unrealized,
		Gathered: final.Balance.Gathered, Consumed: final.Balance.Consumed, Stock: final.Balance.Stock,
		Held: final.Balance.Held, Invested: final.Balance.Invested,
		YieldTotal: final.Balance.YieldTotal, YieldUnrealized: final.Balance.YieldUnrealized,
		GranaryStock: final.Balance.GranaryStock, StoredMeals: final.Balance.StoredMeals,
		Capital: final.Balance.Capital, Wip: final.Balance.Wip,
		PointsCreated: final.Balance.PointsCreated, PointsDecayed: final.Balance.PointsDecayed,
		InitialEnergy: final.Balance.InitialEnergy, Energy: final.Balance.Energy,
		BasalSpent: final.Balance.BasalSpent, EnergyCapLost: final.Balance.EnergyCapLost,
		BirthGranaryPaid: final.Balance.BirthGranaryPaid, Births: final.Balance.Births,
	}
	actors := make(map[sim.EntityID]world.ReproductionActorState, world.ReproductionFounderCount+len(final.Newborns))
	pop.SurvivorIDs = []int{}
	for i := range final.Founders {
		id := sim.EntityID(i + 1)
		actors[id] = final.Founders[i]
		if reproductionAlive(final.Founders[i]) {
			pop.SurvivorIDs = append(pop.SurvivorIDs, i+1)
		}
	}
	for _, nb := range final.Newborns {
		actors[nb.ID] = nb.State
		if reproductionAlive(nb.State) {
			pop.SurvivorIDs = append(pop.SurvivorIDs, int(nb.ID))
		}
	}
	for _, a := range actors {
		br.Totals.TotalWearDebt += a.Worksite.WearDebt
	}
	// Per-actor living children, from genome parent rows over the living.
	livingChildren := make(map[sim.EntityID]int)
	for _, a := range actors {
		if !reproductionAlive(a) {
			continue
		}
		if a.Genome.ParentA != 0 {
			livingChildren[a.Genome.ParentA]++
		}
		if a.Genome.ParentB != 0 {
			livingChildren[a.Genome.ParentB]++
		}
	}
	// Kinship density over all living pairs.
	lookup := reproductionParentLookup(actors)
	living := make([]sim.EntityID, 0, len(actors))
	for id, a := range actors {
		if reproductionAlive(a) {
			living = append(living, id)
		}
	}
	sort.Slice(living, func(i, j int) bool { return living[i] < living[j] })
	br.Kinship.InbreedingRefusals = final.Metrics.Refusals[world.ReproductionRefusalKin]
	var distanceSum int64
	for i := range living {
		for j := i + 1; j < len(living); j++ {
			br.Kinship.LivingPairs++
			if distance, related := world.ReproductionKinDistance(lookup, living[i], living[j]); related {
				br.Kinship.KinPairs++
				distanceSum += int64(distance)
			}
		}
	}
	if br.Kinship.LivingPairs > 0 {
		br.Kinship.RelatedPairFraction = float64(br.Kinship.KinPairs) / float64(br.Kinship.LivingPairs)
	}
	if br.Kinship.KinPairs > 0 {
		br.Kinship.MeanFiniteKinDistance = float64(distanceSum) / float64(br.Kinship.KinPairs)
	}
	for _, id := range sortedActorIDs(actors) {
		a := actors[id]
		br.Kinship.PerActorLivingChildren = append(br.Kinship.PerActorLivingChildren, actorChildren{ID: int(id), LivingChildren: livingChildren[id]})
		br.Actors = append(br.Actors, actorFinalFrom(id, a, checks))
	}
	// Birth economics from the final cumulative metrics.
	br.BirthEconomics = birthEconomicsReport{
		Births: final.Metrics.Births, BirthRequests: final.Metrics.BirthRequests, Divergences: final.Metrics.Divergences,
		Refusals: map[string]int64{},
	}
	for reason, n := range final.Metrics.Refusals {
		name, err := refusalName(reason)
		if err != nil {
			return err
		}
		br.BirthEconomics.Refusals[name] = n
	}
	pop.FinalAlive = final.Metrics.Alive
	// Typed journal classification with accepted-event link verification.
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Key != "" && a.Lost == "" {
				ev, ok := byID[a.EventID]
				if !ok || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor {
					return fmt.Errorf("broken accepted event link: actor %d kind %s at %d", a.Actor, a.Kind, a.Time)
				}
				br.AcceptedLinksChecked++
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
				br.UnlinkedRejectionsChecked++
			}
			switch a.Kind {
			case "gather":
				// Reproduction journal discipline mirrors capacity: a gather
				// claim is admitted iff its typed rejection label is
				// FoodFlowGatherAdmitted; gather rows are unkeyed either way.
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
				br.Events.Build++
			case "paired-meal":
				if a.NoMeal != "" {
					br.Events.PairedNoMealNotes++
				} else {
					br.Events.PairedMeals++
				}
			case "fallback-eatstored":
				if a.NoMeal != "" {
					br.Events.FallbackNoMealNotes++
				} else {
					br.Events.FallbackEatStored++
				}
			case "birth-request":
				br.Events.BirthRequests++
			case "birth-reply":
				br.Events.BirthReplies++
				switch a.Birth.Decision {
				case world.ReproductionReplyAccept:
					br.Events.BirthsAccepted++
				case world.ReproductionReplyRefuse:
					br.Events.BirthRefusals++
				case world.ReproductionReplyRefuseSilent:
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
	// A births-disabled branch is birth-free by the frozen intervention: no
	// birth attempt may appear and no birth ledger may move. Any birth event
	// at q0 is a defect on either branch, and q0 goes extinct by h11.
	if !births && (br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 ||
		br.Totals.Births != 0 || br.BirthEconomics.Births != 0 || br.BirthEconomics.BirthRequests != 0) {
		return errors.New("births-disabled branch recorded birth evidence")
	}
	if o.yield == 0 {
		if br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 || br.Totals.Births != 0 {
			return errors.New("q0 recorded birth events")
		}
		if o.hours >= 11 && pop.AliveByHour[11] != 0 {
			return fmt.Errorf("q0 actors survived past h11: %d alive", pop.AliveByHour[11])
		}
		if o.hours >= 12 && pop.AliveByHour[12] != 0 {
			return fmt.Errorf("q0 actors survived past h12: %d alive", pop.AliveByHour[12])
		}
	}
	return nil
}

// actorFinalFrom freezes one actor's final rows and survival window.
func actorFinalFrom(id sim.EntityID, a world.ReproductionActorState, checks []world.ReproductionCheckpoint) actorFinal {
	out := actorFinal{
		ID: int(id), Alive: reproductionAlive(a),
		Energy: a.Body.Energy, Hunger: a.Body.Hunger,
		BasalSpent: a.Body.BasalSpent, EnergyCapLost: a.Body.CapLost,
		WildMeals: a.Body.Consumed, LastGatherHour: a.Body.LastGatherHour,
		BagUnits: a.Bag.Units,
		Capital:  a.Worksite.Capital, Wip: a.Worksite.Wip,
		WearDebt: a.Worksite.WearDebt, InvestedUnits: a.Worksite.InvestedUnits,
		PointsCreated: a.Worksite.PointsCreated, PointsDecayed: a.Worksite.PointsDecayed,
		LastBuildHour: a.Worksite.LastBuildHour,
		GranaryStock:  a.Granary.Stock, YieldTotal: a.Granary.YieldTotal,
		YieldUnrealized: a.Granary.YieldUnrealized, StoredMeals: a.Granary.StoredMeals,
		LastStoredMealHour: a.Granary.LastStoredMealHour,
		LocusM:             a.Genome.LocusM, LocusY: a.Genome.LocusY, LocusW: a.Genome.LocusW,
		ParentA: int(a.Genome.ParentA), ParentB: int(a.Genome.ParentB),
		BirthHour: a.Genome.BirthHour, DeathHour: a.Genome.DeathHour, DiedHour: a.Genome.DiedHour,
		BirthGranaryPaid:  a.Genome.BirthGranaryPaid,
		ObservedDeathHour: -1,
	}
	if out.Alive {
		return out
	}
	for _, c := range checks {
		var state world.ReproductionActorState
		if id <= sim.EntityID(world.ReproductionFounderCount) {
			state = c.Founders[id-1]
		} else {
			found := false
			for _, nb := range c.Newborns {
				if nb.ID == id {
					state, found = nb.State, true
					break
				}
			}
			if !found {
				// The actor did not exist yet at this hour.
				continue
			}
		}
		if !reproductionAlive(state) {
			out.ObservedDeathHour = c.Hour
			break
		}
	}
	return out
}

func reproductionAlive(a world.ReproductionActorState) bool {
	return a.Genome.DiedHour == world.ReproductionNeverHour && a.Body.Energy > 0
}

func sortedActorIDs(actors map[sim.EntityID]world.ReproductionActorState) []sim.EntityID {
	out := make([]sim.EntityID, 0, len(actors))
	for id := range actors {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// publishedJournalBytes reads the journal section of a published branch
// bundle: the h0-prefix evidence re-pinned to the recorded flags by the
// branch intervention itself. ExportJournal deliberately refuses a restored
// branch child (its boundary topology is the parent's, and the v4 h0 pulse
// is births-asymmetric by frozen design), so the published artifact is the
// honest journal-bytes metric for every branch cell.
func publishedJournalBytes(path string) (int, error) {
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, s := range sections {
		if s.Name == checkpoint.Journal {
			return len(s.Data), nil
		}
	}
	return 0, fmt.Errorf("published bundle %s carries no journal section", path)
}

// runBranch restores one intervention child, continues it to the requested
// horizon and reports trajectories, ledgers and measured costs. The restore,
// run, verification and serialization all sit inside the timing.
func runBranch(ctx context.Context, o options, births bool, loci []int64, parentDigest string, childPath string, childDigest [32]byte, childBytes int64) (branchReport, error) {
	br := branchReport{Births: births, GenomeVariant: o.genomeVariant, ParentDigest: parentDigest, BranchDigest: digestString(childDigest), CompletedHour: o.hours,
		BranchCheckpoint: checkpointInfo{Name: filepath.Base(childPath), Digest: digestString(childDigest), Bytes: childBytes},
		TokenCost:        0}
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return br, err
	}
	expected := world.ReproductionOptions{Yield: o.yield, Seed: o.seed, Workers: o.workers, Births: births, FounderLocusM: loci}
	f, restored, err := world.RestoreReproductionCheckpoint(childPath, expected)
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
	if err := branchEvidence(&br, f, byID, births, o); err != nil {
		return br, err
	}
	journalBytes, err := publishedJournalBytes(childPath)
	if err != nil {
		return br, err
	}
	br.JournalBytes = journalBytes
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
		if r.Branches[i].Births {
			enabled = &r.Branches[i]
		} else {
			disabled = &r.Branches[i]
		}
	}
	if enabled == nil || disabled == nil {
		return
	}
	c := &comparisonReport{ParentDigestShared: enabled.ParentDigest == disabled.ParentDigest,
		FinalAliveEnabled: enabled.Population.FinalAlive, FinalAliveDisabled: disabled.Population.FinalAlive,
		BirthsEnabled: enabled.Totals.Births, BirthsDisabled: disabled.Totals.Births}
	for h := 0; h <= enabled.CompletedHour && h <= disabled.CompletedHour; h++ {
		if enabled.Population.AliveByHour[h] != disabled.Population.AliveByHour[h] {
			c.AliveDifferences = append(c.AliveDifferences, aliveDifference{Hour: h, Enabled: enabled.Population.AliveByHour[h], Disabled: disabled.Population.AliveByHour[h]})
		}
	}
	r.Comparison = c
}

// v3ReferenceBranch mirrors one branch of a recorded ticket-13 capacity
// report, restricted to the v3-visible trajectory fields.
type v3ReferenceBranch struct {
	Enabled     bool               `json:"enabled"`
	AliveByHour []int              `json:"alive_by_hour"`
	Actors      []v3ReferenceActor `json:"actors"`
	Totals      v3ReferenceTotals  `json:"totals"`
}

type v3ReferenceActor struct {
	ID              int   `json:"id"`
	Energy          int64 `json:"energy"`
	Hunger          int64 `json:"hunger"`
	Capital         int64 `json:"capital"`
	Wip             int64 `json:"wip"`
	GranaryStock    int64 `json:"granary_stock"`
	InvestedUnits   int64 `json:"invested_units"`
	PointsCreated   int64 `json:"points_created"`
	PointsDecayed   int64 `json:"points_decayed"`
	YieldTotal      int64 `json:"yield_total"`
	YieldUnrealized int64 `json:"yield_unrealized"`
	StoredMeals     int64 `json:"stored_meals"`
	WildMeals       int64 `json:"wild_meals"`
}

type v3ReferenceTotals struct {
	Alive            int   `json:"alive"`
	TotalK           int64 `json:"total_k"`
	TotalGranary     int64 `json:"total_granary"`
	TotalWip         int64 `json:"total_wip"`
	TotalInvested    int64 `json:"total_invested_units"`
	PointsCreated    int64 `json:"total_points_created"`
	PointsDecayed    int64 `json:"total_points_decayed"`
	TotalWearDebt    int64 `json:"total_wear_debt"`
	TotalYield       int64 `json:"total_yield"`
	TotalUnrealized  int64 `json:"total_yield_unrealized"`
	TotalStoredMeals int64 `json:"total_stored_meals"`
	TotalWildMeals   int64 `json:"total_wild_meals"`
	TotalHeld        int64 `json:"total_held"`
	TotalStock       int64 `json:"total_patch_stock"`
	TotalPatchUnreal int64 `json:"total_patch_unrealized"`
	TotalBasalSpent  int64 `json:"total_basal_spent"`
	TotalCapLost     int64 `json:"total_energy_cap_lost"`
}

type v3ReferenceReport struct {
	CompletedHour int                 `json:"completed_hour"`
	Branches      []v3ReferenceBranch `json:"branches"`
}

// fieldCheck is one named v3-visible field comparison.
type fieldCheck struct {
	Name string
	V4   int64
	V3   int64
}

// compareV3Reference loads a recorded ticket-13 capacity report and compares
// the v4 births-disabled child against it: alive-by-hour through the common
// horizon, and — when the horizons match — the complete per-actor and
// aggregate v3-visible final state. Every divergence is recorded verbatim;
// the equality fixture is a predeclared band, and a mismatch is reported,
// never tuned.
func compareV3Reference(path string, o options, disabled *branchReport) (*v3EqualityReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ref v3ReferenceReport
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, fmt.Errorf("v3 reference %s: %v", path, err)
	}
	if len(ref.Branches) != 1 {
		return nil, fmt.Errorf("v3 reference %s must carry exactly one recorded branch, got %d", path, len(ref.Branches))
	}
	rb := &ref.Branches[0]
	eq := &v3EqualityReport{ReferencePath: path, ReferenceCompletedHour: ref.CompletedHour,
		ReferenceBranchEnabled: rb.Enabled, AliveEqual: true, FinalStateEqual: true}
	through := o.hours
	if ref.CompletedHour < through {
		through = ref.CompletedHour
	}
	eq.ComparedThroughHour = through
	eq.AliveHoursCompared = through + 1
	for h := 0; h <= through; h++ {
		v4, v3 := 0, 0
		if h < len(disabled.Population.AliveByHour) {
			v4 = disabled.Population.AliveByHour[h]
		}
		if h < len(rb.AliveByHour) {
			v3 = rb.AliveByHour[h]
		}
		if v4 != v3 {
			eq.AliveEqual = false
			eq.AliveMismatches = append(eq.AliveMismatches, v3EqualityMismatch{Hour: h, V4: v4, V3: v3})
		}
	}
	if through == o.hours && through == ref.CompletedHour {
		eq.FinalStateCompared = true
		for _, ra := range rb.Actors {
			if ra.ID < 1 || ra.ID > len(disabled.Actors) {
				return nil, fmt.Errorf("v3 reference actor id %d outside the founder roster", ra.ID)
			}
			v4a := disabled.Actors[ra.ID-1]
			if v4a.ID != ra.ID {
				return nil, fmt.Errorf("actor ordering break: report id %d, reference id %d", v4a.ID, ra.ID)
			}
			for _, c := range []fieldCheck{
				{"energy", v4a.Energy, ra.Energy},
				{"hunger", v4a.Hunger, ra.Hunger},
				{"capital", v4a.Capital, ra.Capital},
				{"wip", v4a.Wip, ra.Wip},
				{"granary_stock", v4a.GranaryStock, ra.GranaryStock},
				{"invested_units", v4a.InvestedUnits, ra.InvestedUnits},
				{"points_created", v4a.PointsCreated, ra.PointsCreated},
				{"points_decayed", v4a.PointsDecayed, ra.PointsDecayed},
				{"yield_total", v4a.YieldTotal, ra.YieldTotal},
				{"yield_unrealized", v4a.YieldUnrealized, ra.YieldUnrealized},
				{"stored_meals", v4a.StoredMeals, ra.StoredMeals},
				{"wild_meals", v4a.WildMeals, ra.WildMeals},
			} {
				if c.V4 != c.V3 {
					eq.FinalStateEqual = false
					eq.ActorMismatches = append(eq.ActorMismatches, v3EqualityActorMismatch{Actor: ra.ID, Field: c.Name, V4: c.V4, V3: c.V3})
				}
			}
		}
		for _, t := range []fieldCheck{
			{"total_k", disabled.Totals.Capital, rb.Totals.TotalK},
			{"total_granary", disabled.Totals.GranaryStock, rb.Totals.TotalGranary},
			{"total_wip", disabled.Totals.Wip, rb.Totals.TotalWip},
			{"total_invested_units", disabled.Totals.Invested, rb.Totals.TotalInvested},
			{"total_points_created", disabled.Totals.PointsCreated, rb.Totals.PointsCreated},
			{"total_points_decayed", disabled.Totals.PointsDecayed, rb.Totals.PointsDecayed},
			{"total_wear_debt", disabled.Totals.TotalWearDebt, rb.Totals.TotalWearDebt},
			{"total_yield", disabled.Totals.YieldTotal, rb.Totals.TotalYield},
			{"total_yield_unrealized", disabled.Totals.YieldUnrealized, rb.Totals.TotalUnrealized},
			{"total_stored_meals", disabled.Totals.StoredMeals, rb.Totals.TotalStoredMeals},
			{"total_wild_meals", disabled.Totals.Consumed, rb.Totals.TotalWildMeals},
			{"total_held", disabled.Totals.Held, rb.Totals.TotalHeld},
			{"total_patch_stock", disabled.Totals.Stock, rb.Totals.TotalStock},
			{"total_patch_unrealized", disabled.Totals.Unrealized, rb.Totals.TotalPatchUnreal},
			{"total_basal_spent", disabled.Totals.BasalSpent, rb.Totals.TotalBasalSpent},
			{"total_energy_cap_lost", disabled.Totals.EnergyCapLost, rb.Totals.TotalCapLost},
		} {
			if t.V4 != t.V3 {
				eq.FinalStateEqual = false
				eq.TotalsMismatches = append(eq.TotalsMismatches, v3EqualityActorMismatch{Actor: 0, Field: t.Name, V4: t.V4, V3: t.V3})
			}
		}
	}
	return eq, nil
}

func run(ctx context.Context, o options) (report, error) {
	r := report{
		Status:   "synthetic/model-conditional; frozen reproduction-and-genetics v4 fixture; no historical calibration, takeoff, or P0 scale claim",
		Revision: "reproduction format-v4/schema-v4/rule-v4/projection-v4/policy reproduction@4",
		Yield:    o.yield, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: "reproduction@4", Births: o.births, GenomeVariant: o.genomeVariant, TokenCost: 0,
	}
	if o.genomeVariant {
		r.GenomeVariantLoci = genomeVariantLoci()
	}
	if o.workers < 1 || o.yield < 0 || o.yield > world.ReproductionSlotsPerPatch || o.hours < 1 || o.hours > world.ReproductionHorizonHours {
		return r, errors.New("invalid fixture flags")
	}
	switch o.births {
	case enabledBranchFlag, disabledBranchFlag, bothBranchesFlag:
	default:
		return r, fmt.Errorf("invalid -births value %q", o.births)
	}
	// The recorded genome-variant cells are births-enabled-only: the frozen
	// matrix declares no variant-disabled cell, and the variant comparison
	// baselines are the neutral enabled cells already in the 32-cell grid.
	var loci []int64
	if o.genomeVariant {
		loci = genomeVariantLoci()
		if o.births != enabledBranchFlag {
			return r, fmt.Errorf("genome-variant cells are births-enabled-only; refused -births=%q", o.births)
		}
	}
	if o.v3Reference != "" {
		// The equality fixture is a births-disabled neutral child against a
		// recorded v3 baseline; no other cell is comparable.
		if o.births != disabledBranchFlag || o.genomeVariant {
			return r, errors.New("-v3-reference requires -births=disabled without -genome-variant")
		}
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
	// The neutral h0 pulse is captured with BIRTHS DISABLED: the v4 h0 pulse
	// is births-asymmetric by frozen design (sixteen request wakes exist only
	// when births are enabled), so a disabled root is the only legal branch
	// parent and every births-enabled cell is the recorded enable
	// intervention branching from this same root. SaveCheckpoint verifies
	// that boundary. Variant roots carry the declared founder fixture and
	// their own digest; the intervention discipline is identical.
	root, err := world.NewReproduction(world.ReproductionOptions{Yield: o.yield, Seed: o.seed, Workers: 1, Births: false, FounderLocusM: loci})
	if err != nil {
		return r, err
	}
	if ok, err := root.Step(ctx); err != nil || !ok {
		return r, fmt.Errorf("neutral h0 pulse: processed=%t err=%v", ok, err)
	}
	rootPath := filepath.Join(dir, neutralBundleName)
	rootDigest, err := root.SaveCheckpoint(rootPath)
	if err != nil {
		return r, err
	}
	rootStat, err := os.Stat(rootPath)
	if err != nil {
		return r, err
	}
	r.NeutralCheckpoint = checkpointInfo{Name: neutralBundleName, Digest: digestString(rootDigest), Bytes: rootStat.Size()}
	// Both intervention children branch from the same verified parent; the
	// recorded births decision is the only content difference.
	rootExpected := world.ReproductionOptions{Yield: o.yield, Seed: o.seed, Workers: 1, Births: false, FounderLocusM: loci}
	type branchPlan struct {
		births bool
		name   string
	}
	var plans []branchPlan
	switch o.births {
	case enabledBranchFlag:
		plans = []branchPlan{{births: true, name: enabledBundleName}}
	case disabledBranchFlag:
		plans = []branchPlan{{births: false, name: disabledBundleName}}
	default:
		plans = []branchPlan{{births: true, name: enabledBundleName}, {births: false, name: disabledBundleName}}
	}
	parent := digestString(rootDigest)
	for _, plan := range plans {
		childPath := filepath.Join(dir, plan.name)
		digest, err := world.BranchReproductionCheckpoint(rootPath, rootExpected, plan.births, childPath)
		if err != nil {
			return r, err
		}
		stat, err := os.Stat(childPath)
		if err != nil {
			return r, err
		}
		br, err := runBranch(ctx, o, plan.births, loci, parent, childPath, digest, stat.Size())
		if err != nil {
			return r, err
		}
		r.Branches = append(r.Branches, br)
	}
	if len(r.Branches) == 0 {
		return r, fmt.Errorf("no branch selected for %q", o.births)
	}
	r.CompletedHour = o.hours
	compareBranches(&r)
	if o.v3Reference != "" {
		var disabled *branchReport
		for i := range r.Branches {
			if !r.Branches[i].Births {
				disabled = &r.Branches[i]
			}
		}
		eq, err := compareV3Reference(o.v3Reference, o, disabled)
		if err != nil {
			return r, err
		}
		r.V3Reference = eq
	}
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
	hours := flag.Int("hours", world.ReproductionHorizonHours, "absolute hourly horizon per branch (1..168)")
	births := flag.String("births", bothBranchesFlag, "which recorded intervention to run: enabled, disabled, or both (children always branch from the births-disabled neutral root)")
	genomeVariant := flag.Bool("genome-variant", false, "recorded founder fixture: actors 1-8 locus-m=7, actors 9-16 locus-m=5 (runner option, not a policy change; births-enabled-only in the frozen matrix)")
	v3Reference := flag.String("v3-reference", "", "recorded ticket-13 capacity report JSON to compare the births-disabled child against (equality fixture; disabled neutral cells only)")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the neutral bundle and the branch children; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	r, err := run(context.Background(), options{yield: *yield, seed: *seed, workers: *workers, hours: *hours, births: *births, genomeVariant: *genomeVariant, out: *out, v3Reference: *v3Reference, checkpointDir: *checkpointDir})
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
	// A failed equality fixture is reported verbatim in the JSON and then
	// failed loudly: the predeclared band decides, not the reporter.
	if r.V3Reference != nil && (!r.V3Reference.AliveEqual || (r.V3Reference.FinalStateCompared && !r.V3Reference.FinalStateEqual)) {
		fmt.Fprintln(os.Stderr, "v3 equality fixture mismatch: see v3_equality in the report")
		os.Exit(1)
	}
}
