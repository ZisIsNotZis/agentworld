// Ticket 55 census: W1 clause (c) measured on the implemented birth gate.
// followed by the final NaturalBalance food-ledger components. Read-only, zero
// model calls. Copied from ticket 53 with mechanism v14 (the commons floor)
// added; same construction (neutral h0 pulse, births-enable branch, restore,
// step to H), reduced to the numbers the W1 criterion reads.
//
//	yield 0 keeps mechanism v10 (the frozen one-unit wild fill and the
//	declared cultivated grant); yield N>0 under natural/14 runs mechanism v14,
//	i.e. v13's yield-scaled wild fill, two-energy meal and declared birth
//	price plus the commons floor (-birth-cost 0 is the frozen price of 2).
//
// The line is, tab-separated: policy, yield, birth-cost, founders, seed,
// alive-end, births, starvation deaths, age deaths, world-gate hours,
// policy-gate hours, then the final balance Produced, Granted, Unrealized,
// Gathered, Consumed, Spoiled, GranaryStock, StoredMeals, BirthGranaryPaid,
// BasalSpent, Energy.
//
//	go run ./.scratch/54-mechanism-v14-commons/sweep.go -f 10 -seed 0 -hours 144 -policy natural/14 -yield 2 -birth-cost 4
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

func main() {
	founders := flag.Int("f", 10, "founders")
	seed := flag.Uint64("seed", 0, "seed")
	hours := flag.Int("hours", 144, "horizon")
	patches := flag.Int("patches", 2, "patches")
	r := flag.Int64("r", 8, "regeneration budget per patch-hour")
	policyName := flag.String("policy", "natural/14", "policy ref: natural/7|11|12|13|14")
	yield := flag.Int64("yield", 0, "declared yield, wild fill and cultivated grant (0 = mechanism v10, the frozen one-unit wild fill; N>0 = mechanism v14 with that yield)")
	birthCost := flag.Int64("birth-cost", 0, "declared birth price in granary and energy units per paying parent (0 = the frozen price of 2, i.e. v12 exactly)")
	flag.Parse()

	var pol strategy.NaturalPolicy
	var mech world.NaturalMechanism
	switch *policyName {
	case "natural/7":
		pol, mech = strategy.FrozenNaturalPolicy7(), world.NaturalMechanismV6
	case "natural/11":
		pol, mech = strategy.FrozenNaturalPolicy11(), world.NaturalMechanismV6
	case "natural/12":
		pol, mech = strategy.FrozenNaturalPolicy12(), world.NaturalMechanismV6
	case "natural/13":
		pol, mech = strategy.FrozenNaturalPolicy13(), world.NaturalMechanismV8
		if *yield > 0 {
			mech = world.NaturalMechanismV9
		}
	case "natural/14":
		pol, mech = strategy.FrozenNaturalPolicy14(), world.NaturalMechanismV10
		if *yield > 0 {
			mech = world.NaturalMechanismV14
		}
	default:
		fail(fmt.Errorf("unknown policy %q", *policyName))
	}
	ctx := context.Background()
	base := world.NaturalOptions{Patches: *patches, R: *r, Yield: *yield, BirthCost: *birthCost, Seed: *seed, Workers: 1, Births: false,
		PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders}
	root, err := world.NewNatural(base)
	check(err)
	if ok, err := root.Step(ctx); err != nil || !ok {
		check(fmt.Errorf("neutral h0 pulse: %t %v", ok, err))
	}
	dir, err := os.MkdirTemp("", "sweep49")
	check(err)
	defer os.RemoveAll(dir)
	rootPath := dir + "/root.bundle"
	if _, err = root.SaveCheckpoint(rootPath); err != nil {
		check(err)
	}
	restore := world.NaturalRestoreOptions{Patches: *patches, R: *r, Yield: *yield, BirthCost: *birthCost, Seed: *seed, Workers: 1, Births: true,
		PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders}
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
		check(err)
		if !ok {
			break
		}
	}
	checks := run.Checkpoints()
	last := checks[len(checks)-1]
	// Gate hours: the implemented gate (household gate from v8 on), at the
	// hourly boundary. worldGate counts hours with a proposer-eligible actor
	// (k>=2, granary>=3, energy>=3) and at least one other alive actor fed
	// (energy>=2), i.e. the v8+ gate; policyGate uses energy>=4 for the
	// proposer. The symmetric v5 conjunction is kept as v5Gate for contrast.
	worldGate, policyGate, v5Gate := 0, 0, 0
	for _, cp := range checks {
		var rows []world.NaturalActorState
		for _, a := range cp.Founders {
			rows = append(rows, a)
		}
		for _, nb := range cp.Newborns {
			rows = append(rows, nb.State)
		}
		alive := func(a world.NaturalActorState) bool {
			if a.Genome.DiedHour != world.NaturalNeverHour && a.Genome.DiedHour >= 0 {
				return false
			}
			return a.Body.Energy > 0
		}
		n, np, n5, fed := 0, 0, 0, 0
		for _, a := range rows {
			if !alive(a) {
				continue
			}
			if a.Body.Energy >= 2 {
				fed++
			}
			if a.Granary.Stock >= world.NaturalConsentMinGranary && a.Worksite.Capital >= world.NaturalConsentMinCapital && a.Body.Energy >= world.NaturalConsentMinEnergy {
				n++
				n5++
			}
			if a.Granary.Stock >= world.NaturalConsentMinGranary && a.Worksite.Capital >= world.NaturalConsentMinCapital && a.Body.Energy >= 4 {
				np++
			}
		}
		if n >= 1 && fed >= 2 {
			worldGate++
		}
		if np >= 1 && fed >= 2 {
			policyGate++
		}
		if n5 >= 2 && fed >= 2 {
			v5Gate++
		}
	}
	b := last.Balance
	// policy yield birth-cost founders seed N0 alive births starvation age
	// household-gate-h policy-gate-h v5-symmetric-gate-h, then the eleven
	// NaturalBalance components.
	fmt.Printf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		*policyName, *yield, *birthCost, *founders, *seed, *founders,
		last.Metrics.Alive, last.Metrics.Births, last.Metrics.DeathsStarvation, last.Metrics.DeathsAge,
		worldGate, policyGate, v5Gate,
		b.Produced, b.Granted, b.Unrealized, b.Gathered, b.Consumed, b.Spoiled, b.GranaryStock,
		b.StoredMeals, b.BirthGranaryPaid, b.BasalSpent, b.Energy)
}

func check(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sweep:", err)
	os.Exit(1)
}
