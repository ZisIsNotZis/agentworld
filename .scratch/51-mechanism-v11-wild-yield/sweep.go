// Ticket 51 sweep: one declared no-AI cell, one TSV line of outcome metrics
// followed by the final NaturalBalance food-ledger components. Read-only, zero
// model calls. Copied from ticket 50 with the mechanism-v11 wild fill added;
// same construction (neutral h0 pulse, births-enable branch, restore, step to
// H), reduced to the numbers the W1 criterion reads.
//
//	yield 0 keeps mechanism v10 (the frozen one-unit wild fill and the
//	declared cultivated grant); yield N>0 runs mechanism v11 with that
//	declared yield on the wild fill as well as the cultivated grant, so
//	yield 1 is v10 exactly.
//
//	go run ./.scratch/51-mechanism-v11-wild-yield/sweep.go -f 10 -seed 0 -hours 144 -policy natural/14
//	go run ./.scratch/51-mechanism-v11-wild-yield/sweep.go -f 10 -seed 0 -hours 144 -policy natural/14 -yield 2
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
	yield := flag.Int64("yield", 0, "mechanism-v11 declared yield, wild fill and cultivated grant (0 = mechanism v10, the frozen one-unit wild fill)")
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
			mech = world.NaturalMechanismV11
		}
	default:
		fail(fmt.Errorf("unknown policy %q", *policyName))
	}
	ctx := context.Background()
	base := world.NaturalOptions{Patches: *patches, R: *r, Yield: *yield, Seed: *seed, Workers: 1, Births: false,
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
	restore := world.NaturalRestoreOptions{Patches: *patches, R: *r, Yield: *yield, Seed: *seed, Workers: 1, Births: true,
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
	// Gate hours: under both floors, at the hourly boundary.
	worldGate, policyGate := 0, 0
	for _, cp := range checks {
		n, np := 0, 0
		add := func(a world.NaturalActorState) {
			if a.Genome.DiedHour != world.NaturalNeverHour && a.Genome.DiedHour >= 0 {
				return
			}
			if a.Body.Energy <= 0 {
				return
			}
			if a.Granary.Stock >= world.NaturalConsentMinGranary && a.Worksite.Capital >= world.NaturalConsentMinCapital && a.Body.Energy >= world.NaturalConsentMinEnergy {
				n++
			}
			if a.Granary.Stock >= world.NaturalConsentMinGranary && a.Worksite.Capital >= world.NaturalConsentMinCapital && a.Body.Energy >= 4 {
				np++
			}
		}
		for _, a := range cp.Founders {
			add(a)
		}
		for _, nb := range cp.Newborns {
			add(nb.State)
		}
		if n >= 2 {
			worldGate++
		}
		if np >= 2 {
			policyGate++
		}
	}
	b := last.Balance
	fmt.Printf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		*policyName, *founders, *seed, *founders,
		last.Metrics.Alive, last.Metrics.Births, last.Metrics.DeathsStarvation, last.Metrics.DeathsAge,
		worldGate, policyGate,
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
