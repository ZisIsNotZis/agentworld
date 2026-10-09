package main

import (
	"fmt"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// The declared VS1 arms (spec section 11.2). The seam is L1's deterministic
// FakeClient in every arm; the real gateway is not authorised (spec section
// 16), so no AI arm claims real-model load-bearing.
const (
	ArmA0  = "A0"  // no-AI control: frozen cascade only
	ArmA1  = "A1"  // role-a: model decides the claim instant
	ArmA1i = "A1i" // role-a-inert: model called and recorded, actions discarded
	ArmA2  = "A2"  // role-a + role-b: the full VS1 mechanism
	ArmA3  = "A3"  // reference: wake policy = full (period 1)
	// ArmA2f is the scripted-proposer fixture (EC-5a): the full A2 mechanism
	// driven by a Role B script whose one delta the concrete sandbox admits, so
	// the live propose->validate->sandbox->apply->measure->rollback path runs
	// end to end. It is a test fixture exercising the machinery, never a tuning
	// of a declared arm, so it is NOT part of declaredArms: the declared 60-cell
	// matrix and its honest-null A2 counts are untouched.
	ArmA2f = "A2f"
	// ArmA2g is the admitted-measurement fixture: the same A2 mechanism scripted
	// so the h72 sandbox re-check ADMITS, leaving the head on the child
	// generation, after which the h72 proposal is sandboxed against that child (a
	// cognitive generation, not the frozen base) -- the divergent-parent case of
	// spec 10.2, exercised end to end rather than only with a fake sandbox.
	ArmA2g = "A2g"
)

// declaredArms is the arm order of the matrix.
var declaredArms = []string{ArmA0, ArmA1, ArmA1i, ArmA2, ArmA3}

// FixtureLabel labels the scripted-proposer fixtures everywhere they appear:
// they are hand-authored scripts that make the machinery's lifecycle paths
// reachable, not model judgments and not results tuned into the arms.
const FixtureLabel = "scripted-proposer-fixture (hand-authored Role B script; not a declared arm)"

// fixtureLabelFor distinguishes the two scripted fixtures: A2f exercises
// apply->measure->rollback; A2g exercises an admitted measurement and the
// divergent-parent sandbox branch.
func fixtureLabelFor(arm string) string {
	if arm == ArmA2g {
		return FixtureLabel + " [admitted-measurement: the h72 measurement admits and the h72 proposal is sandboxed against the diverged child parent]"
	}
	return FixtureLabel + " [rollback: h48 apply, h72 measure+rollback]"
}

// scriptedFixtureFounders and scriptedFixtureSeed pin the one declared cell
// (spec section 11.3) the scripted-proposer fixtures run in: f=10, s=0, whose
// audit-hour trajectory lets the hand-authored deltas pass the concrete sandbox
// admission rule (measured offline against the cached audit bundles).
const (
	scriptedFixtureFounders = 10
	scriptedFixtureSeed     = 0
)

// divergentSwitchHour is the hour the A2g Role A script switches from
// provision-first to gather; it replenishes the world across [48,72] so the h72
// sandbox re-check is run over a populated window (spec 10.2).
const divergentSwitchHour = 48

// twinFounders, twinSeed, and twinWorkers pin the declared determinism twin
// (spec section 11.3): f=12, s=0 at 4 workers, compared against its workers-1
// matrix counterpart.
const (
	twinFounders = 12
	twinSeed     = 0
	twinWorkers  = 4
)

// scriptedFixture returns the fixture of the scripted-proposer fixture cells.
// The horizon runs past the h72 audit (so the measurement resolves) to the next
// declared audit boundary (VS1AuditHours[3] = 96), where the run stops. The
// post-measurement save is not reconcilable while an applied generation is
// active (spec section 14.1 reduction 2), so it fails and is reported, never
// swallowed; the fixture report carries that failure rather than pretending it
// saved.
func scriptedFixture() fixture {
	fp := declaredFixture(scriptedFixtureFounders, scriptedFixtureSeed)
	fp.Hours = cognition.VS1AuditHours[3]
	return fp
}

// twinFixture returns the workers-4 twin fixture of a cell.
func twinFixture(f int, seed uint64) fixture {
	fp := declaredFixture(f, seed)
	fp.Workers = twinWorkers
	return fp
}

// declaredFounders and declaredSeeds are the declared cells (spec section
// 11.3): f in {10,11,12} x seeds {0,1,2,3} at -r 8, mechanism v6, policy
// natural/7, births enabled, horizon 168.
var (
	declaredFounders = []int{10, 11, 12}
	declaredSeeds    = []uint64{0, 1, 2, 3}
)

// VS1DeclaredCells is the frozen cell list.
func VS1DeclaredCells() [][2]uint64 {
	var out [][2]uint64
	for _, f := range declaredFounders {
		for _, s := range declaredSeeds {
			out = append(out, [2]uint64{uint64(f), s})
		}
	}
	return out
}

// cellSpec names one declared cell plus the arm that ran in it. Workers is the
// declared worker count of the twin cells (1 for the matrix, so the spec
// string of every matrix cell is unchanged).
type cellSpec struct {
	Arm     string
	F       int
	Seed    uint64
	Workers int
}

func (c cellSpec) String() string {
	if c.Workers > 1 {
		return fmt.Sprintf("%s-f%d-s%d-w%d", c.Arm, c.F, c.Seed, c.Workers)
	}
	return fmt.Sprintf("%s-f%d-s%d", c.Arm, c.F, c.Seed)
}

// fixture is the declared world shape of the VS1 cells.
type fixture struct {
	Patches  int
	R        int64
	Founders int
	Seed     uint64
	Hours    int
	Workers  int
}

// effectiveWorkers is the declared worker count, defaulting to the matrix's 1
// when a fixture leaves it unset.
func (f fixture) effectiveWorkers() int {
	if f.Workers < 1 {
		return 1
	}
	return f.Workers
}

func declaredFixture(f int, seed uint64) fixture {
	return fixture{Patches: 2, R: 8, Founders: f, Seed: seed, Hours: 168, Workers: 1}
}

// probeFixture is the declared scale/ceiling probe shape (spec section 11.4):
// P=32, the full 256-founder block, workers 1.
func probeFixture(seed uint64) fixture {
	return fixture{Patches: 32, R: 8, Founders: 0, Seed: seed, Hours: 168, Workers: 1}
}

// basePolicy is the declared base policy natural/7 (spec section 11.3).
func basePolicy() strategy.NaturalRef { return strategy.FrozenNaturalPolicy7().Ref }

// mechanism is the declared world mechanism v6 (rule 515 present).
func mechanism() world.NaturalMechanism { return world.NaturalMechanismV6 }

// wakeFor returns the declared wake policy of an arm (spec section 11.2):
// periodic(6) for A1/A1i/A2, full for A3, none for A0.
func wakeFor(arm string) protocol.WakePolicy {
	switch arm {
	case ArmA3:
		return cognition.FullWakePolicy()
	case ArmA0:
		return protocol.WakePolicy{}
	default:
		return cognition.VS1WakePolicy()
	}
}

// usesModel reports whether an arm wires the recorded seam at all.
func usesModel(arm string) bool { return arm != ArmA0 }

// usesRoleB reports whether an arm runs the Role B lifecycle.
func usesRoleB(arm string) bool { return arm == ArmA2 || arm == ArmA3 || isFixtureArm(arm) }

// isFixtureArm reports whether an arm is a labeled hand-authored fixture rather
// than a declared matrix arm. Fixture runs are reported separately and never
// enter the declared-arm counts or the fidelity vector.
func isFixtureArm(arm string) bool { return arm == ArmA2f || arm == ArmA2g }

// budgetFor returns the declared cognitive ceiling of an arm. The VS1 matrix
// ceiling is a declared compute/authorised-spend bound (the hermetic fake
// consumes no spend); it is high enough to fund the arm and is instrumented,
// fail-loud, and reported (spec section 8).
func budgetFor(arm string) cognition.CognitiveBudget {
	b := cognition.CognitiveBudget{
		MaxCalls:            4_000_000,
		MaxPromptTokens:     4_000_000_000,
		MaxCompletionTokens: 2_000_000_000,
		MaxCostMicros:       1_000_000_000_000,
	}
	if arm == ArmA3 {
		// The high-budget reference is the arm most likely to bind the
		// ceiling; it shares the matrix ceiling (no arm is silently cheaper).
		b = cognition.CognitiveBudget{
			MaxCalls:            8_000_000,
			MaxPromptTokens:     8_000_000_000,
			MaxCompletionTokens: 4_000_000_000,
			MaxCostMicros:       2_000_000_000_000,
		}
	}
	return b
}
