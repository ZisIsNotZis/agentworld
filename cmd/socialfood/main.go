// Command socialfood runs the frozen social-food v2 pilot comparison. It
// publishes a verified neutral h0 checkpoint, records explicit enable/disable
// branch interventions against it, continues each child to the requested
// hourly horizon, and reports per-branch trajectories, witnessed social
// evidence, distribution metrics and measured costs. Its reports are
// synthetic/model-conditional diagnostics of the frozen fixture: no
// historical calibration, no genealogy claim, no P0 scale extrapolation.
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
	neutralBundleName  = "social-neutral-h0.bundle"
	enabledBundleName  = "social-branch-enabled.bundle"
	disabledBundleName = "social-branch-disabled.bundle"
	enabledBranchFlag  = "enabled"
	disabledBranchFlag = "disabled"
	bothBranchesFlag   = "both"
	socialHourMicros   = int64(world.SocialFoodHour)
)

type options struct {
	yield         int64
	seed          uint64
	workers       int
	hours         int
	branch        string
	out           string
	checkpointDir string
}

type checkpointInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type actorHour struct {
	Hour       int   `json:"hour"`
	Energy     int64 `json:"energy"`
	Hunger     int64 `json:"hunger"`
	Held       int64 `json:"held"`
	Consumed   int64 `json:"consumed"`
	LastDonor  int   `json:"bag_last_donor"`
	LastGather int64 `json:"last_gather_hour"`
}

type actorFinal struct {
	ID                  int    `json:"id"`
	Energy              int64  `json:"energy"`
	Hunger              int64  `json:"hunger"`
	Consumed            int64  `json:"consumed"`
	Held                int64  `json:"held"`
	LastGatherHour      int64  `json:"last_gather_hour"`
	BagSource           int    `json:"bag_source_patch"`
	BagGatherer         int    `json:"bag_gatherer"`
	BagLastDonor        int    `json:"bag_last_donor"`
	GivenUnits          int64  `json:"given_units"`
	ReceivedUnits       int64  `json:"received_units"`
	NetUnits            int64  `json:"net_units"`
	Requests            int64  `json:"requests"`
	Gifts               int64  `json:"gifts"`
	Refusals            int64  `json:"refusals"`
	RequestStatus       string `json:"request_status"`
	SurvivedThroughHour int    `json:"survived_through_hour"`
	DeathHour           int    `json:"death_hour"` // -1 while alive at the final hour
}

type dyadReport struct {
	Requester int     `json:"requester"`
	Addressee int     `json:"addressee"`
	Affinity  int64   `json:"affinity"`
	Requests  int64   `json:"requests"`
	Gifts     int64   `json:"gifts"`
	Refusals  int64   `json:"refusals"`
	Fraction  float64 `json:"assistance_fraction"` // witnessed (gifts+1)/(requests+2), not moral trust
}

type giftEvent struct {
	Hour                 int         `json:"hour"`
	Donor                int         `json:"donor"`
	Requester            int         `json:"requester"`
	Claim                int64       `json:"reported_urgency"`
	RequesterTruthEnergy int64       `json:"requester_truth_energy"` // privileged audit; never donor-visible
	Score                int64       `json:"consent_score"`
	DonorEnergy          int64       `json:"donor_energy"` // privileged audit of the donor's own reserve
	EventID              sim.EventID `json:"event_id"`
}

type denialCounts struct {
	NoStock        int `json:"no_stock"`
	Capacity       int `json:"capacity"`
	Ineligible     int `json:"ineligible"`
	NoAccess       int `json:"no_access"`
	DuplicateActor int `json:"duplicate_actor"`
	Unknown        int `json:"unknown"`
}

type socialCounts struct {
	Requests       int64        `json:"requests"`
	Gifts          int64        `json:"gifts"`
	Refusals       int64        `json:"refusals"`
	Expiries       int64        `json:"expiries"`
	GatherClaims   int64        `json:"gather_claims"`
	Gathers        int64        `json:"gather_attempts"`
	GatherAccepted int64        `json:"gather_accepted"`
	GatherDenied   denialCounts `json:"gather_denied"`
	Eats           int64        `json:"eats"`
}

type patchGini struct {
	Patch    int     `json:"patch"`
	Consumed int64   `json:"consumed_total"`
	Actors   int     `json:"actors"`
	Gini     float64 `json:"consumed_gini"` // 0 by definition when consumed_total is 0
}

type balanceReport struct {
	Produced      int64 `json:"produced"`
	Consumed      int64 `json:"consumed"`
	Held          int64 `json:"held"`
	Stock         int64 `json:"stock"`
	Gathered      int64 `json:"gathered"`
	Unrealized    int64 `json:"unrealized"`
	InitialEnergy int64 `json:"initial_energy"`
	Energy        int64 `json:"energy"`
	BasalSpent    int64 `json:"basal_spent"`
	CapLost       int64 `json:"cap_lost"`
}

type branchReport struct {
	Enabled              bool                   `json:"enabled"`
	ParentDigest         string                 `json:"parent_digest"`
	BranchDigest         string                 `json:"branch_digest"`
	BranchCheckpoint     checkpointInfo         `json:"branch_checkpoint"`
	CompletedHour        int                    `json:"completed_hour"`
	AliveByHour          []int                  `json:"alive_by_hour"`
	FinalAlive           int                    `json:"final_alive"`
	SurvivorIDs          []int                  `json:"survivor_ids"`
	Actors               []actorFinal           `json:"actors"`
	ActorTraces          map[string][]actorHour `json:"actor_traces"`
	Dyads                []dyadReport           `json:"dyads"`
	Social               socialCounts           `json:"social"`
	GiftEvents           []giftEvent            `json:"gift_events"`
	ConsumedGini         [2]patchGini           `json:"patch_consumed_gini"`
	FinalBalance         balanceReport          `json:"final_balance"`
	ConservationVerified bool                   `json:"conservation_verified"`
	Events               int                    `json:"events"`
	EventBodyBytes       int                    `json:"event_body_bytes"`
	AcceptedLinksChecked int                    `json:"accepted_event_links_checked"`
	UnlinkedRejections   int                    `json:"unlinked_rejections_checked"`
	HistoryBytes         int                    `json:"history_bytes"`
	JournalBytes         int                    `json:"journal_bytes"`
	WallTimeNS           int64                  `json:"wall_time_ns"`
	CPUTimeNS            int64                  `json:"cpu_time_ns"`
	PeakRSSBytes         int64                  `json:"peak_rss_bytes"`
	TokenCost            int                    `json:"token_cost"`
}

type aliveDifference struct {
	Hour     int `json:"hour"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
}

type comparisonReport struct {
	ParentDigestShared      bool              `json:"parent_digest_shared"`
	AliveDifferences        []aliveDifference `json:"alive_differences"`
	FinalAliveEnabled       int               `json:"final_alive_enabled"`
	FinalAliveDisabled      int               `json:"final_alive_disabled"`
	SurvivorsEnabled        []int             `json:"survivor_ids_enabled"`
	SurvivorsDisabled       []int             `json:"survivor_ids_disabled"`
	GiftsEnabled            int64             `json:"gifts_enabled"`
	GiftsDisabled           int64             `json:"gifts_disabled"`
	DonorGivenUnitsEnabled  int64             `json:"donor_given_units_enabled"`
	DonorGivenUnitsDisabled int64             `json:"donor_given_units_disabled"`
	TotalConsumedEnabled    int64             `json:"total_consumed_enabled"`
	TotalConsumedDisabled   int64             `json:"total_consumed_disabled"`
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

// consumedGini is the standard Gini coefficient of the eight per-actor
// consumed-unit totals on one patch: the ordered-pair absolute-difference sum
// divided by 2·n²·mean (maximum (n−1)/n). A zero total is defined as zero
// inequality, not an error.
func consumedGini(consumed []int64) float64 {
	var total int64
	for _, v := range consumed {
		if v < 0 {
			return 0
		}
		total += v
	}
	if total == 0 {
		return 0
	}
	n := int64(len(consumed))
	var absSum int64
	for _, xi := range consumed {
		for _, xj := range consumed {
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

func requestStatusName(s world.SocialFoodRequestStatus) string {
	switch s {
	case world.SocialFoodIdle:
		return "idle"
	case world.SocialFoodPending:
		return "pending"
	case world.SocialFoodAccepted:
		return "accepted"
	case world.SocialFoodRefused:
		return "refused"
	case world.SocialFoodExpired:
		return "expired"
	case world.SocialFoodWithdrawn:
		return "withdrawn"
	default:
		return "unknown"
	}
}

func denialName(r world.FoodFlowGatherRejection) string {
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

func digestString(d [32]byte) string { return fmt.Sprintf("%x", d) }

// prepareCheckpointDir creates the bundle directory. An existing path is never
// reused or populated: all published artifacts land at previously absent paths.
func prepareCheckpointDir(path string) (string, error) {
	if path == "" {
		return os.MkdirTemp("", "socialfood-checkpoint-")
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

// socialEvidence cross-checks the journal against the hourly projection and
// the restored accepted history, then assembles the branch report.
func socialEvidence(br *branchReport, f *world.SocialFood, byID map[sim.EventID]kernel.Event, enabled bool, hours int) error {
	checks := f.Checkpoints()
	if len(checks) != hours+1 || checks[hours].Hour != hours {
		return errors.New("missing hourly projection")
	}
	br.AliveByHour = make([]int, len(checks))
	for h, c := range checks {
		if c.Hour != h {
			return fmt.Errorf("hour sequence break at %d", h)
		}
		br.AliveByHour[h] = c.Alive
		balance, err := world.SocialFoodCheckConservation(c.Patches, c.Slots, c.Actors)
		if err != nil || balance != c.Balance {
			return fmt.Errorf("hour %d conservation failure: %v", h, err)
		}
		if balance.Produced != balance.Consumed+balance.Held+balance.Stock ||
			balance.InitialEnergy+balance.Consumed != balance.Energy+balance.BasalSpent+balance.CapLost {
			return fmt.Errorf("hour %d balance identity failure", h)
		}
	}
	br.ConservationVerified = true
	final := checks[hours]
	br.FinalAlive, br.SurvivorIDs = final.Alive, []int{}
	for i, a := range final.Actors {
		survived, death := 0, -1
		for _, c := range checks {
			if c.Actors[i].Energy > 0 {
				survived = c.Hour
			} else if death < 0 {
				death = c.Hour
			}
		}
		if a.Energy > 0 {
			br.SurvivorIDs = append(br.SurvivorIDs, i+1)
		}
		br.Actors = append(br.Actors, actorFinal{
			ID: i + 1, Energy: a.Energy, Hunger: a.Hunger, Consumed: a.Consumed, Held: a.Bag.Units,
			LastGatherHour: a.LastGatherHour, BagSource: int(a.Bag.Source), BagGatherer: int(a.Bag.Gatherer), BagLastDonor: int(a.Bag.LastDonor),
			GivenUnits: final.Ledgers[i].Given, ReceivedUnits: final.Ledgers[i].Received, NetUnits: final.Ledgers[i].Net,
			Requests: final.Requests[i].Requests, Gifts: final.Requests[i].Gifts, Refusals: final.Requests[i].Refusals,
			RequestStatus:       requestStatusName(final.Requests[i].Status),
			SurvivedThroughHour: survived, DeathHour: death,
		})
	}
	for _, id := range []int{1, 8, 16} {
		rows := make([]actorHour, 0, len(checks))
		for _, c := range checks {
			a := c.Actors[id-1]
			rows = append(rows, actorHour{Hour: c.Hour, Energy: a.Energy, Hunger: a.Hunger, Held: a.Bag.Units, Consumed: a.Consumed, LastDonor: int(a.Bag.LastDonor), LastGather: a.LastGatherHour})
		}
		br.ActorTraces[fmt.Sprint(id)] = rows
	}
	br.FinalBalance = balanceReport{
		Produced: final.Balance.Produced, Consumed: final.Balance.Consumed, Held: final.Balance.Held,
		Stock: final.Balance.Stock, Gathered: final.Balance.Gathered, Unrealized: final.Balance.Unrealized,
		InitialEnergy: final.Balance.InitialEnergy, Energy: final.Balance.Energy,
		BasalSpent: final.Balance.BasalSpent, CapLost: final.Balance.CapLost,
	}
	for patch := 0; patch < 2; patch++ {
		consumed := make([]int64, 0, 8)
		var total int64
		for _, a := range final.Actors[patch*8 : (patch+1)*8] {
			consumed = append(consumed, a.Consumed)
			total += a.Consumed
		}
		br.ConsumedGini[patch] = patchGini{Patch: 1001 + patch, Consumed: total, Actors: len(consumed), Gini: consumedGini(consumed)}
	}
	var givenUnits, receivedUnits int64
	for i, l := range final.Ledgers {
		givenUnits += l.Given
		receivedUnits += l.Received
		if l.Net != l.Received-l.Given {
			return fmt.Errorf("actor %d ledger net broken", i+1)
		}
	}
	var witnessedRequests, witnessedGifts, witnessedRefusals int64
	for i := range final.Requests {
		assistance, err := world.SocialFoodProjectAssistance(sim.EntityID(i+1), final.Version, final.Requests[i])
		if err != nil {
			return fmt.Errorf("dyad %d assistance projection: %v", i+1, err)
		}
		target, affinity, err := world.SocialFoodKnownAddressee(sim.EntityID(i + 1))
		if err != nil || assistance.Addressee != target {
			return fmt.Errorf("dyad %d addressee mismatch", i+1)
		}
		br.Dyads = append(br.Dyads, dyadReport{Requester: i + 1, Addressee: int(target), Affinity: affinity,
			Requests: assistance.Requests, Gifts: assistance.Gifts, Refusals: assistance.Refusals, Fraction: assistance.Fraction})
		witnessedRequests += assistance.Requests
		witnessedGifts += assistance.Gifts
		witnessedRefusals += assistance.Refusals
	}
	var pending int64
	for _, r := range final.Requests {
		if r.Status == world.SocialFoodPending {
			pending++
		}
	}
	if pending != 0 {
		return fmt.Errorf("%d requests still pending after the final hour", pending)
	}
	journal := f.Journal()
	for _, batch := range journal {
		hour := int(batch.Time / sim.SimTime(socialHourMicros))
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
			case "request":
				br.Social.Requests++
				if !a.Accepted {
					return errors.New("uncommitted social request attempt")
				}
			case "gift":
				br.Social.Gifts++
				br.GiftEvents = append(br.GiftEvents, giftEvent{Hour: hour, Donor: int(a.Actor), Requester: int(a.Target),
					Claim: a.Claim, RequesterTruthEnergy: a.Truth, Score: a.Score, DonorEnergy: a.DonorEnergy, EventID: a.EventID})
				if !a.Accepted {
					return errors.New("uncommitted gift attempt")
				}
			case "refuse":
				br.Social.Refusals++
			case "expire":
				br.Social.Expiries++
			case "claim":
				br.Social.GatherClaims++
			case "gather":
				br.Social.Gathers++
				if a.Accepted {
					br.Social.GatherAccepted++
				} else {
					switch denialName(a.Rejection) {
					case "no_stock":
						br.Social.GatherDenied.NoStock++
					case "capacity":
						br.Social.GatherDenied.Capacity++
					case "ineligible":
						br.Social.GatherDenied.Ineligible++
					case "no_access":
						br.Social.GatherDenied.NoAccess++
					case "duplicate_actor":
						br.Social.GatherDenied.DuplicateActor++
					default:
						br.Social.GatherDenied.Unknown++
					}
				}
			case "eat":
				br.Social.Eats++
			default:
				return fmt.Errorf("unknown journal attempt kind %q", a.Kind)
			}
		}
	}
	if !enabled {
		if br.Social.Requests+br.Social.Gifts+br.Social.Refusals+br.Social.Expiries != 0 {
			return errors.New("disabled branch recorded social evidence")
		}
	} else if br.Social.Requests != br.Social.Gifts+br.Social.Refusals+br.Social.Expiries {
		return fmt.Errorf("resolved request mismatch: %d requests vs %d gifts, %d refusals, %d expiries",
			br.Social.Requests, br.Social.Gifts, br.Social.Refusals, br.Social.Expiries)
	}
	// Journal attempts and final witnessed dyad counters are independent
	// ledgers of the same directed evidence; they must agree exactly.
	if witnessedRequests != br.Social.Requests || witnessedGifts != br.Social.Gifts || witnessedRefusals != br.Social.Refusals {
		return fmt.Errorf("dyad witness mismatch: journal requests %d gifts %d refusals %d vs witnessed %d/%d/%d",
			br.Social.Requests, br.Social.Gifts, br.Social.Refusals, witnessedRequests, witnessedGifts, witnessedRefusals)
	}
	if givenUnits != br.Social.Gifts || receivedUnits != br.Social.Gifts {
		return fmt.Errorf("gift ledger mismatch: given %d received %d gifts %d", givenUnits, receivedUnits, br.Social.Gifts)
	}
	return nil
}

// historyEvidence restores the accepted history to count events and verify
// the journal link target set; the restored head must match the handoff head.
func historyEvidence(br *branchReport, handoff world.SocialFoodHandoff) (map[sim.EventID]kernel.Event, error) {
	reg, err := world.SocialFoodRegistry()
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
	br.Events = len(events)
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

// runBranch restores one intervention child, continues it to the requested
// horizon and reports trajectories, social evidence and measured costs. The
// restore, run, verification and serialization all sit inside the timing.
func runBranch(ctx context.Context, o options, enabled bool, parentDigest string, childPath string, childDigest [32]byte, childBytes int64) (branchReport, error) {
	br := branchReport{Enabled: enabled, ParentDigest: parentDigest, BranchDigest: digestString(childDigest), CompletedHour: o.hours,
		BranchCheckpoint: checkpointInfo{Name: filepath.Base(childPath), Digest: digestString(childDigest), Bytes: childBytes},
		ActorTraces:      make(map[string][]actorHour, 3), TokenCost: 0}
	start := time.Now()
	cpuStart, _, err := cpuTime()
	if err != nil {
		return br, err
	}
	opts := world.SocialFoodOptions{Yield: o.yield, Seed: o.seed, Workers: o.workers, Enabled: enabled}
	f, _, err := world.RestoreSocialFoodCheckpoint(childPath, opts)
	if err != nil {
		return br, err
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
	if err := socialEvidence(&br, f, byID, enabled, o.hours); err != nil {
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
		SurvivorsEnabled: enabled.SurvivorIDs, SurvivorsDisabled: disabled.SurvivorIDs,
		GiftsEnabled: enabled.Social.Gifts, GiftsDisabled: disabled.Social.Gifts,
		DonorGivenUnitsEnabled: 0, DonorGivenUnitsDisabled: 0,
		TotalConsumedEnabled: enabled.FinalBalance.Consumed, TotalConsumedDisabled: disabled.FinalBalance.Consumed}
	for _, a := range enabled.Actors {
		c.DonorGivenUnitsEnabled += a.GivenUnits
	}
	for _, a := range disabled.Actors {
		c.DonorGivenUnitsDisabled += a.GivenUnits
	}
	for h := 0; h <= enabled.CompletedHour && h <= disabled.CompletedHour; h++ {
		if enabled.AliveByHour[h] != disabled.AliveByHour[h] {
			c.AliveDifferences = append(c.AliveDifferences, aliveDifference{Hour: h, Enabled: enabled.AliveByHour[h], Disabled: disabled.AliveByHour[h]})
		}
	}
	r.Comparison = c
}

func run(ctx context.Context, o options) (report, error) {
	r := report{
		Status:   "synthetic/model-conditional; frozen social-food v2 fixture; no historical calibration, genealogy, or P0 scale claim",
		Revision: "social-food format-v2/schema-v2/rule-v2/projection-v2/policy social-food@2 with gather food-flow-gather@1",
		Yield:    o.yield, Seed: o.seed, Workers: o.workers, RequestedHours: o.hours,
		Policy: "social-food@2", BranchSelection: o.branch, TokenCost: 0,
	}
	if o.workers < 1 || o.yield < 0 || o.yield > 8 || o.hours < 1 || o.hours > world.SocialFoodHorizonHours {
		return r, errors.New("invalid fixture flags")
	}
	switch o.branch {
	case enabledBranchFlag, disabledBranchFlag, bothBranchesFlag:
	default:
		return r, fmt.Errorf("invalid -branch value %q", o.branch)
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
	// The neutral h0 pulse: exactly one committed step, no claim, no social
	// evidence, before any intervention. SaveCheckpoint verifies that boundary.
	neutral, err := world.NewSocialFood(world.SocialFoodOptions{Yield: o.yield, Seed: o.seed, Workers: 1, Enabled: false})
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
	expected := world.SocialFoodOptions{Yield: o.yield, Seed: o.seed, Workers: 1}
	enabledPath := filepath.Join(dir, enabledBundleName)
	disabledPath := filepath.Join(dir, disabledBundleName)
	enabledDigest, err := world.BranchSocialFoodCheckpoint(rootPath, expected, true, enabledPath)
	if err != nil {
		return r, err
	}
	disabledDigest, err := world.BranchSocialFoodCheckpoint(rootPath, expected, false, disabledPath)
	if err != nil {
		return r, err
	}
	enabledStat, err := os.Stat(enabledPath)
	if err != nil {
		return r, err
	}
	disabledStat, err := os.Stat(disabledPath)
	if err != nil {
		return r, err
	}
	parent := digestString(rootDigest)
	type branchPlan struct {
		enabled bool
		digest  [32]byte
		path    string
		bytes   int64
	}
	plans := []branchPlan{
		{enabled: true, digest: enabledDigest, path: enabledPath, bytes: enabledStat.Size()},
		{enabled: false, digest: disabledDigest, path: disabledPath, bytes: disabledStat.Size()},
	}
	for _, plan := range plans {
		if o.branch != bothBranchesFlag && o.branch != enabledBranchFlag && plan.enabled {
			continue
		}
		if o.branch != bothBranchesFlag && o.branch != disabledBranchFlag && !plan.enabled {
			continue
		}
		br, err := runBranch(ctx, o, plan.enabled, parent, plan.path, plan.digest, plan.bytes)
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
	yield := flag.Int64("q", 3, "hourly production per patch (0..8)")
	seed := flag.Uint64("seed", 0, "deterministic allocation seed")
	workers := flag.Int("workers", 1, "scheduler worker count for both branch continuations")
	hours := flag.Int("hours", world.SocialFoodHorizonHours, "absolute hourly horizon per branch (1..168)")
	branch := flag.String("branch", bothBranchesFlag, "which recorded intervention to run: enabled, disabled, or both")
	out := flag.String("out", "", "new JSON report path; refused if it exists; the report also goes to stdout")
	checkpointDir := flag.String("checkpoint-dir", "", "new directory for the neutral bundle and both branch children; must not exist; default creates a fresh temporary directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	r, err := run(context.Background(), options{yield: *yield, seed: *seed, workers: *workers, hours: *hours, branch: *branch, out: *out, checkpointDir: *checkpointDir})
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
