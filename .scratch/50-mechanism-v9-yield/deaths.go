// Ticket 50 death analysis: for one declared no-AI cell, print every
// starvation death with the dead actor's home patch, the wild (uncultivated)
// and wild-stocked slot count on that patch at the death hour, and the
// actor's granary stock, energy and capital at death. Read-only, zero model
// calls. Same construction as sweep.go.
//
//	go run ./.scratch/50-mechanism-v9-yield/deaths.go -f 10 -seed 0 -hours 144 -yield 3
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

type row struct {
	hour, homePatch, wildSlots, wildStocked, granary, energy, capital int64
}

func main() {
	founders := flag.Int("f", 10, "founders")
	seed := flag.Uint64("seed", 0, "seed")
	hours := flag.Int("hours", 144, "horizon")
	patches := flag.Int("patches", 2, "patches")
	r := flag.Int64("r", 8, "regeneration budget per patch-hour")
	yield := flag.Int64("yield", 3, "mechanism-v9 declared granary yield (0 = mechanism v8)")
	flag.Parse()

	pol := strategy.FrozenNaturalPolicy13()
	mech := world.NaturalMechanismV8
	if *yield > 0 {
		mech = world.NaturalMechanismV9
	}
	ctx := context.Background()
	root, err := world.NewNatural(world.NaturalOptions{Patches: *patches, R: *r, Yield: *yield, Seed: *seed,
		Workers: 1, Births: false, PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders})
	check(err)
	if ok, err := root.Step(ctx); err != nil || !ok {
		check(fmt.Errorf("neutral h0 pulse: %t %v", ok, err))
	}
	dir, err := os.MkdirTemp("", "deaths50")
	check(err)
	defer os.RemoveAll(dir)
	rootPath := dir + "/root.bundle"
	if _, err = root.SaveCheckpoint(rootPath); err != nil {
		check(err)
	}
	expected := world.NaturalRestoreOptions{Patches: *patches, R: *r, Yield: *yield, Seed: *seed, Workers: 1,
		Births: false, PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders}
	restore := expected
	restore.Births = true
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

	fmt.Println("hour\tactor\thome_patch\twild_slots\twild_stocked\tliving_on_patch\tgranary\tenergy\tcapital\tdeath_hour\tgenome_death_hour")
	deaths := 0
	prev := map[simID]bool{}
	for _, cp := range run.Checkpoints() {
		actors := actorRows(cp)
		for _, a := range actors {
			alive := a.diedHour == world.NaturalNeverHour || a.diedHour < 0
			wasAlive := prev[simID(a.id)]
			if wasAlive && !alive && a.diedHour == int64(cp.Hour) && a.diedHour != a.deathHour {
				// Starvation: died before the age clock. Report the land
				// state on the actor's home patch at the death hour.
				wild, stocked := 0, 0
				pi := -1
				for i := 0; i < *patches; i++ {
					if id, err := world.NaturalPatchID(*patches, i); err == nil && int64(id) == a.home {
						pi = i
					}
				}
				if pi >= 0 && pi < len(cp.Slots) {
					for _, s := range cp.Slots[pi] {
						if s.Mode == world.NaturalTenureWild {
							wild++
							if s.Stock > 0 {
								stocked++
							}
						}
					}
				}
				living := 0
				for _, b := range actors {
					if b.home == a.home && (b.diedHour == world.NaturalNeverHour || b.diedHour < 0) {
						living++
					}
				}
				fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", cp.Hour, a.id, a.home, wild, stocked, living,
					a.granary, a.energy, a.capital, a.diedHour, a.deathHour)
				deaths++
			}
			prev[simID(a.id)] = alive
		}
	}
	if deaths == 0 {
		fmt.Fprintln(os.Stderr, "deaths:", 0)
	}
}

type simID = uint64

type actorRow struct {
	id, home, diedHour, deathHour, granary, energy, capital int64
}

func actorRows(cp world.NaturalCheckpoint) []actorRow {
	out := make([]actorRow, 0, len(cp.Founders)+len(cp.Newborns))
	add := func(id uint64, a world.NaturalActorState) {
		out = append(out, actorRow{id: int64(id), home: int64(a.Bag.Source), diedHour: a.Genome.DiedHour,
			deathHour: a.Genome.DeathHour, granary: a.Granary.Stock, energy: a.Body.Energy,
			capital: a.Worksite.Capital})
	}
	for i, a := range cp.Founders {
		add(uint64(i+1), a)
	}
	for _, nb := range cp.Newborns {
		add(uint64(nb.ID), nb.State)
	}
	return out
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "deaths:", err)
		os.Exit(1)
	}
}
