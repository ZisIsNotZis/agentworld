// Ticket 44 census: why do births stop? Read-only measurement over a no-AI
// (A0) run of the vs2 world. Prints per-cell gate reachability, eligible
// pair-hours and the typed birth-refusal histogram. Mutates nothing.
//
// Reproduce the recorded artefacts (Go 1.24, repo root):
//   go run ./.scratch/44-fertility-calibration/census.go -f 10 -seed 0 -hours 144 > census-f10-s0.json
//   go run ./.scratch/44-fertility-calibration/census.go -f 12 -seed 0 -hours 144 > census-f12-s0.json
//   go run ./.scratch/44-fertility-calibration/census.go -f 6  -seed 0 -hours 144 > census-f6-s0.json
// Policy-family sweep (section 3b): add -policy natural/5|6|7|7b|9 . The census JSONs
// were produced with -policy natural/7 (the vs2 base policy) unless a row says otherwise.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

var refusalNames = []string{"none", "not-pending", "duplicate-reply", "addressee-dead",
	"proposer-dead", "consent-own", "consent-claims", "kin", "cross-patch", "gate-thresholds"}

func main() {
	founders := flag.Int("f", 10, "founders")
	seed := flag.Uint64("seed", 0, "seed")
	hours := flag.Int("hours", 144, "horizon")
	patches := flag.Int("patches", 2, "patches")
	r := flag.Int64("r", 8, "regeneration budget per patch-hour")
	policyName := flag.String("policy", "natural/7", "policy ref: natural/5|6|7|7b|9")
	flag.Parse()

	var pol strategy.NaturalPolicy
	var mech world.NaturalMechanism
	switch *policyName {
	case "natural/5":
		pol, mech = strategy.FrozenNaturalPolicy(), world.NaturalMechanismV5
	case "natural/6":
		pol, mech = strategy.FrozenNaturalPolicyV6(), world.NaturalMechanismV6
	case "natural/7":
		pol, mech = strategy.FrozenNaturalPolicy7(), world.NaturalMechanismV6
	case "natural/7b":
		pol, mech = strategy.FrozenNaturalPolicy7b(), world.NaturalMechanismV6
	case "natural/9":
		pol, mech = strategy.FrozenNaturalPolicy9(), world.NaturalMechanismV7
	default:
		check(fmt.Errorf("unknown policy %q", *policyName))
	}
	ctx := context.Background()
	base := world.NaturalOptions{
		Patches: *patches, R: *r, Seed: *seed, Workers: 1, Births: false,
		PolicyRef: pol.Ref, Mechanism: mech,
		Founders: *founders,
	}
	root, err := world.NewNatural(base)
	check(err)
	if ok, err := root.Step(ctx); err != nil || !ok {
		check(fmt.Errorf("neutral h0 pulse: %t %v", ok, err))
	}
	dir, err := os.MkdirTemp("", "census44")
	check(err)
	defer os.RemoveAll(dir)
	rootPath := dir + "/root.bundle"
	if _, err = root.SaveCheckpoint(rootPath); err != nil {
		check(err)
	}
	restore := world.NaturalRestoreOptions{
		Patches: *patches, R: *r, Seed: *seed, Workers: 1, Births: true,
		PolicyRef: pol.Ref, Mechanism: mech,
		Founders: *founders,
	}
	expected := restore
	expected.Births = false
	branchPath := dir + "/branch.bundle"
	if _, err = world.BranchNaturalCheckpoint(rootPath, expected, true, branchPath); err != nil {
		check(err)
	}
	run, _, err := world.RestoreNaturalCheckpoint(branchPath, restore)
	check(err)

	for {
		checks := run.Checkpoints()
		if len(checks) > 0 && checks[len(checks)-1].Hour >= *hours {
			break
		}
		ok, err := run.Step(ctx)
		if err != nil {
			check(err)
		}
		if !ok {
			break
		}
	}

	type live struct{ id, source, pa, pb int; e, g, k int64 }
	var (
		maxEnergy, maxGranary, maxCapital     int64
		maxG3, maxAll3, maxAll4, maxK1, maxK1e4 int
		hoursG3x2, hoursAll3x2, hoursAll4x2, hoursK1x2, hoursK1e4x2 int
		hoursWithPair, pairHours, maxPairHour int
		householdPairHours, hoursHousehold, ahAll int
		peakAlive                             int
	)
	checks := run.Checkpoints()
	for _, cp := range checks {
		var g3, all3, all4, k1, k1e4, maxE, maxG, maxK int64
		var lives []live
		add := func(id int, a world.NaturalActorState) {
			if a.Genome.DiedHour != world.NaturalNeverHour || a.Body.Energy <= 0 {
				return
			}
			lives = append(lives, live{id: id, source: int(a.Bag.Source), pa: int(a.Genome.ParentA), pb: int(a.Genome.ParentB), e: a.Body.Energy, g: a.Granary.Stock, k: a.Worksite.Capital})
			e, g, k := a.Body.Energy, a.Granary.Stock, a.Worksite.Capital
			if e > maxE {
				maxE = e
			}
			if g > maxG {
				maxG = g
			}
			if k > maxK {
				maxK = k
			}
			if g >= 3 {
				g3++
			}
			if g >= 3 && k >= 2 && e >= 3 {
				all3++
			}
			if g >= 3 && k >= 2 && e >= 4 {
				all4++
			}
			if g >= 3 && k >= 1 && e >= 3 {
				k1++
			}
			if g >= 3 && k >= 1 && e >= 4 {
				k1e4++
			}
		}
		for i, a := range cp.Founders {
			add(i+1, a)
		}
		for i := range cp.Newborns {
			add(int(cp.Newborns[i].ID), cp.Newborns[i].State)
		}
		if len(lives) > peakAlive {
			peakAlive = len(lives)
		}
		// actor-hours by condition
		ahAll++
		if maxE > 0 {
		}
		householdPairs := 0
		for i := range lives {
			a := lives[i]
			if !(a.g >= 3 && a.k >= 2 && a.e >= 3) {
				continue
			}
			for j := range lives {
				if i == j {
					continue
				}
				b := lives[j]
				if a.source != b.source || b.e < 2 {
					continue
				}
				if a.id == b.pa || a.id == b.pb || b.id == a.pa || b.id == a.pb {
					continue
				}
				if (a.pa != 0 && a.pa == b.pa) || (a.pb != 0 && a.pb == b.pb) {
					continue
				}
				householdPairs++
			}
		}
		householdPairHours += householdPairs
		if householdPairs > 0 {
			hoursHousehold++
		}
		eligible := 0
		for i := 0; i < len(lives); i++ {
			for j := i + 1; j < len(lives); j++ {
				a, b := lives[i], lives[j]
				if a.source != b.source {
					continue
				}
				if a.id == b.pa || a.id == b.pb || b.id == a.pa || b.id == a.pb {
					continue
				}
				if (a.pa != 0 && a.pa == b.pa) || (a.pb != 0 && a.pb == b.pb) {
					continue
				}
				eligible++
			}
		}
		if g3 >= 2 {
			hoursG3x2++
		}
		if all3 >= 2 {
			hoursAll3x2++
		}
		if eligible > 0 {
			hoursWithPair++
			pairHours += eligible
			if eligible > maxPairHour {
				maxPairHour = eligible
			}
		}
		if int(g3) > maxG3 {
			maxG3 = int(g3)
		}
		if int(all3) > maxAll3 {
			maxAll3 = int(all3)
		}
		if all4 >= 2 {
			hoursAll4x2++
		}
		if k1 >= 2 {
			hoursK1x2++
		}
		if k1e4 >= 2 {
			hoursK1e4x2++
		}
		if int(all4) > maxAll4 {
			maxAll4 = int(all4)
		}
		if int(k1) > maxK1 {
			maxK1 = int(k1)
		}
		if int(k1e4) > maxK1e4 {
			maxK1e4 = int(k1e4)
		}
		if maxE > maxEnergy {
			maxEnergy = maxE
		}
		if maxG > maxGranary {
			maxGranary = maxG
		}
		if maxK > maxCapital {
			maxCapital = maxK
		}
	}

	kinds := map[string]int{}
	birthReasons := map[string]int{}
	buildRefusals := map[string]int{}
	acceptedBirths := 0
	for _, b := range run.Journal() {
		for _, a := range b.Attempts {
			kinds[a.Kind]++
			switch a.Kind {
			case "birth-reply":
				if a.Accepted {
					acceptedBirths++
				} else {
					n := fmt.Sprintf("%d->%d", int(a.Birth.Decision), int(a.Birth.Reason))
					if a.Birth.Reason == world.NaturalRefusalNone {
						n = fmt.Sprintf("decision-%d", int(a.Birth.Decision))
					}
					birthReasons[n]++
				}
			case "build":
				if !a.Accepted && a.Refused != "" {
					buildRefusals[a.Refused]++
				}
			}
		}
	}

	last := checks[len(checks)-1]
	ref := map[string]int64{}
	for k, v := range last.Metrics.Refusals {
		name := fmt.Sprintf("unknown-%d", int(k))
		if int(k) < len(refusalNames) {
			name = refusalNames[int(k)]
		}
		ref[name] = v
	}
	out := map[string]any{
		"f": *founders, "seed": *seed, "hours": *hours, "policy": *policyName,
		"alive_end": last.Metrics.Alive, "peak_alive": peakAlive,
		"births": last.Metrics.Births, "birth_requests": last.Metrics.BirthRequests,
		"starve": last.Metrics.DeathsStarvation, "age": last.Metrics.DeathsAge,
		"max_energy": maxEnergy, "max_granary": maxGranary, "max_capital": maxCapital,
		"max_actors_granary_ge3": maxG3, "hours_with_2plus_granary_ge3": hoursG3x2,
		"max_actors_all3": maxAll3, "hours_with_2plus_all3": hoursAll3x2,
		"hours_with_2plus_g3_k2_e4": hoursAll4x2, "max_actors_g3_k2_e4": maxAll4,
		"hours_with_2plus_g3_k1_e3": hoursK1x2, "max_actors_g3_k1_e3": maxK1,
		"hours_with_2plus_g3_k1_e4": hoursK1e4x2, "max_actors_g3_k1_e4": maxK1e4,
		"hours_with_eligible_pair": hoursWithPair, "eligible_pair_hours": pairHours,
		"max_eligible_pairs_in_an_hour": maxPairHour,
		"household_gate_pair_hours": householdPairHours, "hours_with_household_pair": hoursHousehold,
		"refusals": ref, "journal_kinds": kinds, "birth_reply_outcomes": birthReasons, "build_refusals": buildRefusals, "accepted_birth_replies": acceptedBirths,
	}
	b, _ := json.MarshalIndent(out, "", " ")
	fmt.Println(string(b))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "census:", err)
		os.Exit(1)
	}
}
