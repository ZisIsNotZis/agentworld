package main

import (
	"fmt"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// The declared VS2 arms (spec section 8.1). A0 and R carry a nil Priority (no
// priority layer); P, U and A each carry a non-nil PriorityPolicy, because only
// a priority-bearing online selector can realize the arm's declared rule.
const (
	ArmA0 = "A0" // no-AI control: frozen cascade only, nil cognition
	ArmR  = "R"  // reference: FullWakePolicy, every candidate in its own stream
	ArmP  = "P"  // periodic: period-gated, stop at N
	ArmU  = "U"  // uniform-random: per-hour quota via the policy PRNG
	ArmA  = "A"  // adaptive-priority: protected exploration + priority ranking
	// ArmA1 is the same adaptive arm under the SAME request contract but the
	// vs2-value/1 model: it isolates the value-model change from the claim-contract
	// change, so a v2 run reports the cost-benefit pair (ticket 35).
	ArmA1 = "A1"
)

// declaredArms is the arm order of the matrix (spec section 8.1).
var declaredArms = []string{ArmA0, ArmR, ArmP, ArmU, ArmA}

// budgetedArms are the arms carrying the declared call ceiling N (P/U/A).
var budgetedArms = []string{ArmP, ArmU, ArmA}

// DeclaredCallCeiling is N, the declared call ceiling of every budgeted arm
// (spec section 5.1, fixed before any run). It is not derived from any
// model-bearing arm's realized candidate count. It is the fail-loud ceiling on
// physical attempts; the schedule selects at most N less the declared transport
// reserve (cognition.DeclaredTransportReserveAttempts, ticket 40), so a retried
// attempt lands inside the reserve instead of truncating a completing cell.
const DeclaredCallCeiling uint64 = 256

// DeclaredExplorationFraction is q = 25% protected exploration for arm A
// (spec Appendix C.2), over the ExplorationScale fixed-point denominator.
const DeclaredExplorationFraction int64 = 250_000

// DeclaredReferenceCeiling is R's separately declared, generous fail-loud
// ceiling (spec section 5.2): a full-universe reference is never truncated to a
// cheaper one.
const DeclaredReferenceCeiling uint64 = 4_096

// PeriodHours is the declared periodic gate of the P arm (spec section 8.1),
// chosen from the model-free A0 trajectory so the selected hours supply at
// least N candidates. It is inert for U and A.
const PeriodHours = 6

// VS2ValueModelVersion is the declared value-model version; it is part of run
// identity (VS2-EC-8).
const VS2ValueModelVersion = "vs2-value/1"

// VS2ValueModelVersionV2 is the second declared value model (ticket 35): the
// vs2-value/1 projection plus a marginal/staleness term and a per-actor coverage
// floor. It exists because the vs2-value/1 ranking collapsed coverage (43-49
// calls on five actors, 1-3 on three others at f10-s0).
const VS2ValueModelVersionV2 = "vs2-value/2"

// declaredValueModel returns the declared Appendix C.2 value-model instance:
// the versioned integer fixed-point weights and the unpriced cost proxy. The
// derived feature rows are produced by the world's roster builder (lane L2); an
// unknown feature name contributes nothing to the projection.
func declaredValueModel() protocol.ValueModelSpec {
	return declaredValueModelV2()
}

// declaredValueModelV2 is the declared vs2-value/2 instance: the same features,
// the same weights and the same cost proxy, plus the two declared selection
// terms. Only the ranking changes; the feature set version is untouched.
func declaredValueModelV2() protocol.ValueModelSpec {
	spec := declaredValueModelV1()
	spec.Version = VS2ValueModelVersionV2
	spec.StalenessRefreshHours = protocol.DefaultStalenessRefreshHours
	spec.StalenessFloorBasisPoints = protocol.DefaultStalenessFloorBasisPoints
	spec.CoverageFloor = true
	return spec
}

// valueModelFor is the declared value model of an arm: ArmA1 keeps vs2-value/1 so
// a v2 run can isolate the value-model change; every other arm uses vs2-value/2.
func valueModelFor(arm string) protocol.ValueModelSpec {
	if arm == ArmA1 {
		return declaredValueModelV1()
	}
	return declaredValueModelV2()
}

// declaredValueModelV1 is the declared Appendix C.2 vs2-value/1 instance.
func declaredValueModelV1() protocol.ValueModelSpec {
	return protocol.ValueModelSpec{
		Version:    VS2ValueModelVersion,
		FeatureSet: protocol.FeatureSetVersion,
		Weights: []protocol.FeatureWeight{
			{Feature: "decision_sensitivity", Weight: 4},
			{Feature: "urgency", Weight: 3},
			{Feature: "impact", Weight: 2},
			{Feature: "propagation_risk", Weight: 2},
			{Feature: "prior_reliability", Weight: 1},
			{Feature: "escalation_requested", Weight: 5},
		},
		CostProxy: protocol.CostProxy{
			Version:               "vs2-cost-proxy/1",
			ConstantMicrosPerCall: 1,
		},
	}
}

// declaredPriorityPolicy returns the declared PriorityPolicy of a budgeted arm.
// N is the frozen ceiling; RandSeed is the cell seed (part of run identity) so
// U and A draw a stable declared PRNG stream. A carries the declared
// exploration fraction; U and P isolate pure exploitation-ish baselines with
// q = 0 (only A reserves exploration).
func declaredPriorityPolicy(arm string, seed uint64) *protocol.PriorityPolicy {
	strategy := protocol.StrategyPeriodic
	fraction := int64(0)
	switch arm {
	case ArmU:
		strategy = protocol.StrategyUniformRandom
	case ArmA, ArmA1:
		strategy = protocol.StrategyAdaptive
		fraction = DeclaredExplorationFraction
	}
	return &protocol.PriorityPolicy{
		Strategy:            strategy,
		ExplorationFraction: fraction,
		RandSeed:            seed,
		ValueModel:          valueModelFor(arm),
		CallCeiling:         DeclaredCallCeiling,
	}
}

// declaredPolicy is the declared WakePolicy of an arm. A0 carries none and R is
// the VS1 full reference (period 1, nil Priority); every budgeted arm carries a
// non-nil Priority (spec section 8.1).
func declaredPolicy(arm string, seed uint64) protocol.WakePolicy {
	switch arm {
	case ArmA0:
		return protocol.WakePolicy{}
	case ArmR:
		return cognition.FullWakePolicy()
	default:
		return protocol.WakePolicy{
			Name: "vs2-" + arm, PeriodHours: PeriodHours, PhaseHours: 0,
			Priority: declaredPriorityPolicy(arm, seed),
		}
	}
}

// isBudgeted reports whether an arm carries the declared call ceiling N. ArmA1 is
// budgeted like ArmA but is not part of the declared matrix inventory, so the
// declared-arms list is unchanged.
func isBudgeted(arm string) bool {
	if arm == ArmA1 {
		return true
	}
	for _, a := range budgetedArms {
		if a == arm {
			return true
		}
	}
	return false
}

// budgetFor returns the declared fail-loud cognitive ceiling of an arm. A hit
// on a budgeted arm is a gate failure: the declared rule must stop before N on
// its own (spec section 5.2). R's ceiling is the generous reference bound. A0 has
// no cognition and no ceiling.
func budgetFor(arm string) cognition.CognitiveBudget {
	calls := DeclaredCallCeiling
	if arm == ArmR {
		calls = DeclaredReferenceCeiling
	}
	return cognition.CognitiveBudget{
		MaxCalls:            calls,
		MaxPromptTokens:     1 << 60,
		MaxCompletionTokens: 1 << 60,
		MaxCostMicros:       1 << 60,
	}
}

// The declared gateway wire of a VS2 run. The application model advertises
// mode=responses with only /v1/responses, so the declared VS2 default is the
// responses wire; the hermetic fake ignores the wire, but the recorded endpoint
// and the model+wire identity digest name the wire a run was configured for.
var declaredGatewayWire = cognition.WireResponses

// The declared applied wire settings of a live run (ticket 35). They are run
// identity, not world mechanisms: DeclaredReasoningEffort is the applied
// `reasoning.effort`, DeclaredOutputBound the applied `max_output_tokens`
// (32768 by supervisor decision, so the bound is never the truncation cause),
// DeclaredVotes the samples per escalated decision (`-votes`, default 1), and
// DeclaredReplicates the number of repeated runs per cell (`-reps`, default 1).
// Every one of them is recorded in the revision string and in each cell's
// report row; the applied effort and bound also reach every call record.
var (
	declaredReasoningEffort = ""
	declaredOutputBound     = 0
	declaredVotes           = 1
	declaredReplicates      = 1
)

// DeclaredOutputBoundDefault is the output-token bound of the curve study
// (ticket 35 supervisor decision 1). A cap is free headroom: the measured max
// effort p99 is 13 826 output tokens, so this bound truncates no measured call.
const DeclaredOutputBoundDefault = 32768

// hermeticEndpoint is the declared recorded endpoint host+path of the hermetic
// cognition stack; it names the wire mode.
func hermeticEndpoint(mode cognition.WireMode) string {
	if mode == cognition.WireResponses {
		return "hermetic.local/v1/responses"
	}
	return "hermetic.local/v1/chat/completions"
}

// The declared fixture family (spec section 8.2) is the W1 declared world
// (ticket 55, docs/VERTICAL-SLICES.md W2): mechanism v14, base policy natural/14,
// Yield 1, BirthCost 2, -r 8, patches 2, births enabled, horizon H=144,
// workers 1, founders from the declared cell. The one constructor is
// declaredWorld; no run-level flag can move the world identity.
//
// DeclaredHorizonHours is H, the single declared horizon every arm's divergence
// vector is computed over (spec section 5.3).
const DeclaredHorizonHours = 144

// The declared world axes of the W1 world (ticket 55 §3). Yield is the
// mechanism-v14 cultivated/wild grant per patch-hour; BirthCost is the price one
// paying parent hands over for a birth. Both are recorded in the evaluation and
// the revision string, and both reach the runner's journal/checkpoint identity.
const (
	DeclaredYield     int64 = 1
	DeclaredBirthCost int64 = 2
)

// The declared call-liveness bounds of a live run (ticket 35). The frozen
// harness defaults (30 s per attempt, 90 s per call) were sized for the hermetic
// fake and for medium-effort calls; a measured max-effort call reaches 155 s at
// the 32768 bound, so a live run declares a longer attempt timeout and call
// deadline. Both are recorded in the revision string and in the report. A
// shorter bound is the frozen default.
var (
	declaredAttemptTimeout = 30 * time.Second
	declaredCallDeadline   = 90 * time.Second
	declaredResponseBound  = 0 // 0: the spec's 64 KiB hermetic recorded-body bound
	declaredClaimContract  = cognition.ClaimContractV1
)

// declaredHorizonHours is the horizon a run uses: the declared H by default, the
// -hours override for a shorter declared study horizon (ticket 35 runs 96 h, or
// 72 h when the measured max-effort latency makes 96 h exceed the declared wall
// bound). It is recorded in the revision string and in every cell's fixture.
var declaredHorizonHours = DeclaredHorizonHours

// declaredFounders and declaredSeeds are the declared primary cells (spec
// section 8.2): f in {10,11,12} x seeds {0,1,2,3}.
var (
	declaredFounders = []int{10, 11, 12}
	declaredSeeds    = []uint64{0, 1, 2, 3}
)

// sensitivityCells are the declared sensitivity-seed extension (spec section
// 8.2): f=12 x seeds {4,5,6,7}.
func sensitivityCells() [][2]uint64 {
	var out [][2]uint64
	for _, s := range []uint64{4, 5, 6, 7} {
		out = append(out, [2]uint64{12, s})
	}
	return out
}

// VS2DeclaredCells is the frozen primary cell list.
func VS2DeclaredCells() [][2]uint64 {
	var out [][2]uint64
	for _, f := range declaredFounders {
		for _, s := range declaredSeeds {
			out = append(out, [2]uint64{uint64(f), s})
		}
	}
	return out
}

// cellSpec names one declared cell plus the arm that ran in it. Replicate is
// the 1-based replicate index of a repeated run (0 for a single unreplicated
// run), so a replicate is a distinct run directory, ledger, and checkpoint
// bundle.
type cellSpec struct {
	Arm       string
	F         int
	Seed      uint64
	Workers   int
	Replicate int
}

func (c cellSpec) String() string {
	if c.Workers > 1 {
		return fmt.Sprintf("%s-f%d-s%d-w%d", c.Arm, c.F, c.Seed, c.Workers)
	}
	if c.Replicate >= 1 {
		return fmt.Sprintf("%s-f%d-s%d-r%d", c.Arm, c.F, c.Seed, c.Replicate)
	}
	return fmt.Sprintf("%s-f%d-s%d", c.Arm, c.F, c.Seed)
}

// fixture is the declared world shape of a VS2 cell: the substrate axes plus
// the two declared world rules (mechanism and base policy) and the declared
// yield/birth price. One value names everything a cell's world is built from, so
// no part of the world identity can drift from the rest.
type fixture struct {
	Patches   int
	R         int64
	Founders  int
	Seed      uint64
	Hours     int
	Workers   int
	Yield     int64
	BirthCost int64
	Mechanism world.NaturalMechanism
	PolicyRef strategy.NaturalRef
}

// effectiveWorkers is the declared worker count, defaulting to the matrix's 1.
func (f fixture) effectiveWorkers() int {
	if f.Workers < 1 {
		return 1
	}
	return f.Workers
}

// declaredWorld is the one home of the declared W1 world shape at (f, seed) and
// the declared H. Every matrix cell, the model-call fixture identity (built by
// the world's naturalRunIdentity from these axes) and the evaluation's world
// record come from this constructor.
func declaredWorld(f int, seed uint64) fixture {
	return fixture{
		Patches: 2, R: 8, Founders: f, Seed: seed, Hours: declaredHorizonHours, Workers: 1,
		Yield: DeclaredYield, BirthCost: DeclaredBirthCost,
		Mechanism: declaredMechanism(), PolicyRef: declaredBasePolicy(),
	}
}

// declaredBasePolicy is the declared base policy natural/14 (ticket 55 §3):
// natural/13 verbatim except the round-robin founder-placement validation the v14
// mechanism declares.
func declaredBasePolicy() strategy.NaturalRef { return strategy.FrozenNaturalPolicy14().Ref }

// declaredMechanism is the declared world mechanism v14: the commons floor that
// keeps at least half of every patch wild.
func declaredMechanism() world.NaturalMechanism { return world.NaturalMechanismV14 }

// quotaSchedule is the declared fixed per-hour quota schedule
// q(h) = floor((N-R)/H) + (1 if h < (N-R) mod H else 0), so sum_{h<H} q(h) =
// N - R (spec section 5.1, ticket 40): the schedule stops the declared transport
// reserve R of attempts short of the ceiling N and leaves them to retries. It
// depends on no arm's realized stream and is frozen before any run. It mirrors
// the L3 accumulator's own schedule; a test pins the two together.
func quotaSchedule(ceiling uint64, horizon int) []int {
	q := make([]int, horizon)
	decisions := cognition.DecisionBudget(ceiling)
	base := decisions / uint64(horizon)
	extra := decisions % uint64(horizon)
	for h := 0; h < horizon; h++ {
		q[h] = int(base)
		if uint64(h) < extra {
			q[h]++
		}
	}
	return q
}

// reservedExploration is E = ceil(q*N) = (N*frac + scale - 1)/scale, the declared
// reserved protected-exploration calls of arm A (spec section 4.5).
func reservedExploration(ceiling uint64, fraction int64) int {
	return int((int64(ceiling)*fraction + cognition.ExplorationScale - 1) / cognition.ExplorationScale)
}

// modelIdentityDigestBytes derives the recorded model identity from the
// declared application model and the declared gateway wire, so a run's revision
// binding names the model+wire+sampling identity it used.
func modelIdentityDigestBytes() [32]byte {
	return cognition.ModelIdentityDigest(cognition.DefaultModelID, declaredGatewayWire, cognition.FrozenSampling())
}
