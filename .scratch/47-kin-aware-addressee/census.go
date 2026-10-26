// Ticket 47 census: the kin-aware addressee differential (natural/12), read at
// the decision instant. Copied verbatim from ticket 46's census with natural/12
// (mechanism v6) added to the policy switch.
//
// Read-only measurement over a no-AI (A0) run of the vs2 world. For every
// birth-reply decision it brackets the consenter's state between the two
// adjacent hourly checkpoints and records what the consenter did inside the
// hour (build completion, paired meal), then classifies the typed refusal.
// Mutates nothing; no model call.
//
// The bracket is monotone in the frozen rules:
//
//	reply granary <= checkpoint(h+1) granary   (the h+1 pulse grants regen)
//	reply capital >= checkpoint(h+1) capital   (the h+1 pulse wears points)
//	reply energy  =  checkpoint(h+1) energy + the h+1 basal cost
//
// so a refusal whose checkpoint(h+1) state already fails the gate is
// explained by the state, and a refusal whose checkpoint(h+1) state passes it
// is not.
//
// Reproduce (Go 1.24, repo root):
//
//	go run ./.scratch/47-kin-aware-addressee/census.go -f 10 -seed 0 -hours 144 -policy natural/7 > census-f10-s0-natural7.json
//	go run ./.scratch/47-kin-aware-addressee/census.go -f 10 -seed 0 -hours 144 -policy natural/11 > census-f10-s0-natural11.json
//	go run ./.scratch/47-kin-aware-addressee/census.go -f 10 -seed 0 -hours 144 -policy natural/12 > census-f10-s0-natural12.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

var refusalNames = []string{"none", "not-pending", "duplicate-reply", "addressee-dead",
	"proposer-dead", "consent-own", "consent-claims", "kin", "cross-patch", "gate-thresholds"}

type triple struct {
	Energy  int64 `json:"energy"`
	Granary int64 `json:"granary"`
	Capital int64 `json:"capital"`
	Source  int   `json:"source"`
	ParentA int   `json:"parent_a"`
	ParentB int   `json:"parent_b"`
	Alive   bool  `json:"alive"`
}

func (t triple) gateWorld() bool {
	return t.Granary >= world.NaturalConsentMinGranary && t.Capital >= world.NaturalConsentMinCapital && t.Energy >= world.NaturalConsentMinEnergy
}

// gatePolicy is the v6/v7 policy's own floors: the world's consent triple with
// the policy's filing/consent energy floor of 4 (natural7.go TFile).
func (t triple) gatePolicy() bool {
	return t.Granary >= world.NaturalConsentMinGranary && t.Capital >= world.NaturalConsentMinCapital && t.Energy >= 4
}

type reply struct {
	Hour                        int    `json:"hour"`
	Consenter                   int    `json:"consenter"`
	AlternativesWorldGateAtEnd  int    `json:"alternative_mates_world_gate_at_end"`
	AlternativesPolicyGateAtEnd int    `json:"alternative_mates_policy_gate_at_end"`
	Decision                    string `json:"decision"`
	Reason                      string `json:"reason"`
	Start                       triple `json:"at_h_start"`
	End                         triple `json:"at_hp1"`
	Built                       bool   `json:"consenter_built_this_hour"`
	Meal                        bool   `json:"consenter_paired_meal_this_hour"`
	Class                       string `json:"class"`
}

type report struct {
	Cell      string `json:"cell"`
	Policy    string `json:"policy"`
	Founders  int    `json:"founders"`
	Seed      uint64 `json:"seed"`
	Hours     int    `json:"hours"`
	AliveAtH  int    `json:"alive_at_h"`
	GateHours struct {
		TwoPlusWorld  int `json:"two_plus_world_gate"`
		TwoPlusPolicy int `json:"two_plus_policy_gate"`
	} `json:"gate_hours"`
	Replies struct {
		Accept           int `json:"accept"`
		RefuseConsentOwn int `json:"refuse_consent_own"`
		RefuseOther      int `json:"refuse_other"`
	} `json:"replies"`
	Classes map[string]int `json:"classes"`
	Rows    []reply        `json:"rows"`
}

func main() {
	founders := flag.Int("f", 10, "founders")
	seed := flag.Uint64("seed", 0, "seed")
	hours := flag.Int("hours", 144, "horizon")
	patches := flag.Int("patches", 2, "patches")
	r := flag.Int64("r", 8, "regeneration budget per patch-hour")
	policyName := flag.String("policy", "natural/7", "policy ref: natural/5|6|7|7b|9|11|12")
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
	case "natural/11":
		pol, mech = strategy.FrozenNaturalPolicy11(), world.NaturalMechanismV6
	case "natural/12":
		pol, mech = strategy.FrozenNaturalPolicy12(), world.NaturalMechanismV6
	default:
		check(fmt.Errorf("unknown policy %q", *policyName))
	}
	ctx := context.Background()
	base := world.NaturalOptions{
		Patches: *patches, R: *r, Seed: *seed, Workers: 1, Births: false,
		PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders,
	}
	root, err := world.NewNatural(base)
	check(err)
	if ok, err := root.Step(ctx); err != nil || !ok {
		check(fmt.Errorf("neutral h0 pulse: %t %v", ok, err))
	}
	dir, err := os.MkdirTemp("", "census45")
	check(err)
	defer os.RemoveAll(dir)
	rootPath := dir + "/root.bundle"
	if _, err = root.SaveCheckpoint(rootPath); err != nil {
		check(err)
	}
	restore := world.NaturalRestoreOptions{
		Patches: *patches, R: *r, Seed: *seed, Workers: 1, Births: true,
		PolicyRef: pol.Ref, Mechanism: mech, Founders: *founders,
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
		check(err)
		if !ok {
			break
		}
	}

	// Hourly checkpoint index: hour -> actor -> state.
	type actorState = triple
	byHour := map[int]map[sim.EntityID]actorState{}
	var last *world.NaturalCheckpoint
	for i := range run.Checkpoints() {
		cp := run.Checkpoints()[i]
		m := map[sim.EntityID]actorState{}
		alive := 0
		for j, a := range cp.Founders {
			m[sim.EntityID(j+1)] = stateOf(a)
			if stateOf(a).Alive {
				alive++
			}
		}
		for _, nb := range cp.Newborns {
			m[nb.ID] = stateOf(nb.State)
			if stateOf(nb.State).Alive {
				alive++
			}
		}
		byHour[cp.Hour] = m
		c := cp
		last = &c
	}
	if last == nil {
		check(fmt.Errorf("no checkpoints"))
	}

	rep := report{Cell: fmt.Sprintf("f%d-s%d", *founders, *seed), Policy: *policyName,
		Founders: *founders, Seed: *seed, Hours: *hours, Classes: map[string]int{}}
	// Alive at the horizon, and the joint-gate hours under both floors.
	if m, ok := byHour[last.Hour]; ok {
		for _, s := range m {
			if s.Alive {
				rep.AliveAtH++
			}
		}
	}
	for h := 0; h <= *hours; h++ {
		worldN, policyN := 0, 0
		for _, s := range byHour[h] {
			if !s.Alive {
				continue
			}
			if s.gateWorld() {
				worldN++
			}
			if s.gatePolicy() {
				policyN++
			}
		}
		if worldN >= 2 {
			rep.GateHours.TwoPlusWorld++
		}
		if policyN >= 2 {
			rep.GateHours.TwoPlusPolicy++
		}
	}

	// Per-hour actor activity from the journal, then the reply classification.
	type key struct {
		hour  int
		actor sim.EntityID
	}
	built := map[key]bool{}
	meal := map[key]bool{}
	for _, b := range run.Journal() {
		h := int(int64(b.Time) / int64(world.NaturalHour))
		for _, a := range b.Attempts {
			k := key{h, a.Actor}
			switch a.Kind {
			case "build":
				if a.Accepted {
					built[k] = true
				}
			case "paired-meal":
				if a.Accepted {
					meal[k] = true
				}
			}
		}
	}
	for _, b := range run.Journal() {
		h := int(int64(b.Time) / int64(world.NaturalHour))
		for _, a := range b.Attempts {
			if a.Kind != "birth-reply" {
				continue
			}
			row := reply{Hour: h, Consenter: int(a.Actor), Built: built[key{h, a.Actor}], Meal: meal[key{h, a.Actor}]}
			row.Start = byHour[h][a.Actor]
			row.End = byHour[h+1][a.Actor]
			self := row.End
			for mateID, mate := range byHour[h+1] {
				if mateID == a.Actor || !mate.Alive || mate.Source != self.Source {
					continue
				}
				if int(mateID) == self.ParentA || int(mateID) == self.ParentB ||
					self.ParentA != 0 && self.ParentA == mate.ParentA ||
					self.ParentB != 0 && self.ParentB == mate.ParentB {
					continue
				}
				if mate.gateWorld() {
					row.AlternativesWorldGateAtEnd++
				}
				if mate.gatePolicy() {
					row.AlternativesPolicyGateAtEnd++
				}
			}
			switch a.Birth.Decision {
			case world.NaturalReplyAccept:
				row.Decision = "accept"
				rep.Replies.Accept++
			case world.NaturalReplyRefuseSilent:
				row.Decision = "refuse-silent"
				row.Reason = refusalName(a.Birth.Reason)
				rep.Replies.RefuseOther++
			case world.NaturalReplyRefuse:
				row.Decision = "refuse"
				row.Reason = refusalName(a.Birth.Reason)
				if a.Birth.Reason == world.NaturalRefusalConsentOwn {
					rep.Replies.RefuseConsentOwn++
				} else {
					rep.Replies.RefuseOther++
				}
			default:
				row.Decision = "none"
			}
			row.Class = classify(row)
			rep.Classes[row.Class]++
			rep.Rows = append(rep.Rows, row)
		}
	}
	sort.Slice(rep.Rows, func(i, j int) bool {
		if rep.Rows[i].Hour != rep.Rows[j].Hour {
			return rep.Rows[i].Hour < rep.Rows[j].Hour
		}
		return rep.Rows[i].Consenter < rep.Rows[j].Consenter
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	check(enc.Encode(rep))
}

// classify names the explanation the bracket permits, in the declared order:
//
//	explained-end     the checkpoint(h+1) state already fails the world gate
//	post-build-dip    the consenter built this hour, so granary was spent
//	                  below the gate at the reply instant at +40m+3us
//	explained-policy  the checkpoint(h+1) state passes the world gate but
//	                  fails the policy's energy-4 floor
//	unexplained       both checkpoints pass every floor: the refusal cannot be
//	                  read from boundary state and needs the instant record
//	accepted          the reply was accepted
func classify(r reply) string {
	if r.Decision == "accept" {
		return "accepted"
	}
	if r.Decision != "refuse" {
		return "refuse-other"
	}
	if r.Reason != "consent-own" {
		return "refuse-other"
	}
	if !r.End.gateWorld() {
		if r.Built && r.Start.gateWorld() {
			return "post-build-dip"
		}
		return "explained-end"
	}
	if !r.End.gatePolicy() {
		return "explained-policy"
	}
	return "unexplained"
}

func stateOf(a world.NaturalActorState) triple {
	return triple{Energy: a.Body.Energy, Granary: a.Granary.Stock, Capital: a.Worksite.Capital,
		Source: int(a.Bag.Source), ParentA: int(a.Genome.ParentA), ParentB: int(a.Genome.ParentB),
		Alive: a.Genome.DiedHour == world.NaturalNeverHour && a.Body.Energy > 0}
}

func refusalName(r world.NaturalRefusalReason) string {
	if int(r) >= 0 && int(r) < len(refusalNames) {
		return refusalNames[r]
	}
	return fmt.Sprintf("reason-%d", int(r))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "census:", err)
		os.Exit(1)
	}
}
