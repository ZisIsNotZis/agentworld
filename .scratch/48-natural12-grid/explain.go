// Ticket 48 diagnostic: which rule binds the second generation?
//
// For every hourly checkpoint of one no-AI cell it counts, over living actors:
//
//	proposers   actors holding the proposer gate (k>=2, granary>=3, energy>=4)
//	mates_any   co-located living mates of those proposers
//	mates_kin   ... that are within the prohibited kin depth
//	mates_nonkin ... that are not
//	mates_consent ... non-kin mates that also hold the consenter wealth gate
//	pairs       proposer/non-kin-consenting pairs
//
// and prints one line per hour plus a total. Read-only, zero model calls.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

type actor struct {
	id      int
	source  int
	pa, pb  int
	energy  int64
	granary int64
	capital int64
}

func kin(a, b actor) bool {
	if a.id == b.pa || a.id == b.pb || b.id == a.pa || b.id == a.pb {
		return true
	}
	if a.pa != 0 && a.pa == b.pa || a.pb != 0 && a.pb == b.pb {
		return true
	}
	return false
}

func main() {
	founders := flag.Int("f", 10, "founders")
	seed := flag.Uint64("seed", 0, "seed")
	hours := flag.Int("hours", 144, "horizon")
	policyName := flag.String("policy", "natural/12", "natural/7|11|12")
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
	}
	ctx := context.Background()
	root, err := world.NewNatural(world.NaturalOptions{Patches: 2, R: 8, Seed: *seed, Workers: 1,
		Births: false, PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders})
	check(err)
	if ok, err := root.Step(ctx); err != nil || !ok {
		check(fmt.Errorf("pulse: %t %v", ok, err))
	}
	dir, err := os.MkdirTemp("", "explain48")
	check(err)
	defer os.RemoveAll(dir)
	rootPath := dir + "/root.bundle"
	if _, err = root.SaveCheckpoint(rootPath); err != nil {
		check(err)
	}
	restore := world.NaturalRestoreOptions{Patches: 2, R: 8, Seed: *seed, Workers: 1, Births: true,
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
		if ok, err := run.Step(ctx); err != nil || !ok {
			check(err)
			break
		}
	}
	fmt.Println("hour\talive\tproposers\tmates_any\tmates_kin\tmates_nonkin\tmates_consent\tpairs")
	var tAlive, tProp, tAny, tKin, tNon, tCon, tPairs int
	for _, cp := range run.Checkpoints() {
		var as []actor
		add := func(id int, a world.NaturalActorState) {
			if a.Genome.DiedHour != world.NaturalNeverHour && a.Genome.DiedHour >= 0 || a.Body.Energy <= 0 {
				return
			}
			as = append(as, actor{id: id, source: int(a.Bag.Source), pa: int(a.Genome.ParentA), pb: int(a.Genome.ParentB),
				energy: a.Body.Energy, granary: a.Granary.Stock, capital: a.Worksite.Capital})
		}
		for i, a := range cp.Founders {
			add(i+1, a)
		}
		for _, nb := range cp.Newborns {
			add(int(nb.ID), nb.State)
		}
		prop, any, k, non, con, pairs := 0, 0, 0, 0, 0, 0
		for _, p := range as {
			if !(p.capital >= 2 && p.granary >= 3 && p.energy >= 4) {
				continue
			}
			prop++
			day := 0
			for _, m := range as {
				if m.id == p.id || m.source != p.source {
					continue
				}
				day++
				if kin(p, m) {
					k++
					continue
				}
				non++
				if m.capital >= 2 && m.granary >= 3 && m.energy >= 3 {
					con++
					pairs++
				}
			}
			any += day
		}
		tAlive += len(as)
		tProp += prop
		tAny += any
		tKin += k
		tNon += non
		tCon += con
		tPairs += pairs
		fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", cp.Hour, len(as), prop, any, k, non, con, pairs)
	}
	fmt.Fprintf(os.Stderr, "total alive=%d proposer-hours=%d mate-slots=%d kin=%d nonkin=%d consenting-nonkin=%d pairs=%d\n",
		tAlive, tProp, tAny, tKin, tNon, tCon, tPairs)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "explain:", err)
		os.Exit(1)
	}
}
