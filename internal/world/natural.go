package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Natural v5 runner (ticket 15): the cap-free natural-limits world over the
// frozen contract (natural_contract*.go) and the frozen policy
// (internal/strategy/natural*.go), per the frozen design section of
// .scratch/15-natural-limits/spec.md including the recorded corrigenda. This
// file owns the dynamic roster (maps keyed by allocator-issued entity IDs,
// sorted iteration), the frozen pulse order, the build slot-claim arbitration
// and its documented denial loop, the runner-owned birth streams over the
// kernel entity allocator, the compute-ceiling instrumentation (P15), and the
// value-only evidence seams (journal, hourly checkpoints, quiescent handoff).
// Encoding, authentication, and restore belong to the persistence lane.
//
// Frozen hour lattice for hour h (NaturalPhaseTime): the pulse at h+0 runs
//
//	1 age-death (514, world; EffectStop, energy retained — v4 stage order,
//	             so a dying actor never pays basal again),
//	2 basal (504, world; may set died-hour on starvation),
//	3 regen-split (501, world) with the flat granary grants (508, world):
//	             cultivated slots of living owners first, then vacant wild
//	             slots ascending, remainder regen-unrealized (frozen order),
//	4 spoilage (502, world; every owner, dead included),
//	5 wear (503, world; lowest-index slot revert, dead owners included),
//	6 pending-request expiry (513, world);
//
// then the v4 activity phases verbatim: claim at h+1µs, gather completion /
// denied-gather fallback at h+20m+1µs, bag decision at h+20m+2µs, build start
// at h+20m+3µs, build completion with fresh-snapshot slot-claim revalidation
// at h+40m+3µs, paired meal at h+40m+4µs, birth request at h+40m+5µs, birth
// reply at h+40m+6µs, with the v4 two-flag births/two-flag journal semantics
// (keyed attempts linked to committed events; the one legitimate keyless
// key-carrier is a birth that lost an allocator-ID collision).
//
// Build denial loop (documented runner/policy contract, strategy/natural.go):
// a rule-505 completion that cannot claim the lowest-index free wild slot
// with stock 0 is refused by NaturalBuild with a typed
// NaturalBuildRefusalError and ZERO mutation (the bag keeps its unit). The
// runner then re-evaluates the bag decision for the same actor-hour with
// LastDenial = NaturalBuildDenied; the policy reports Build unavailable and
// emits NaturalEat. The contract's NaturalConsumeProposal pins the frozen
// bag-decision instant, which lies in the past at completion time, so the
// runner commits the re-decided consume under its own phase discipline at
// the next frozen instant (the h+40m+4µs paired-meal slot, exactly where the
// v4 denied-gather fallback meal lives), as a rule-507 proposal carrying the
// contract's NaturalConsume patch set. The paired stored meal of a refused
// build is dropped: the denial re-evaluation IS the bag decision, and an Eat
// decision has no paired meal.
//
// Build-completion arbitration: every builder of an hour completes at the
// same frozen instant h+40m+3µs, and all proposals of one batch plan against
// the same pre-batch snapshot. Without arbitration two builders could claim
// the same lowest-index free slot and the kernel would discard the whole
// later proposal (a silent lost conversion). The coordinator therefore folds
// the contract's own NaturalBuild over the builders in ascending actor ID at
// the step boundary: builder i's call sees builder i-1's claim in its slot
// array, so claims partition the free slots deterministically and a builder
// that finds no free slot at its folded turn enters the typed denial loop.
// The contract math, not the runner, decides every claim and refusal.
//
// Grant/spoil field attribution: the pulse's flat grant (508) and the rot
// (502) both move the same owner's granary stock within one pulse (the
// frozen order grants before the spoil debt accrues), and the kernel's
// single-winner rule forbids two same-timestamp proposals on one field. The
// two proposals therefore commit with disjoint granary field sets: rule 508
// carries the net stock (post-grant, post-rot) plus the cultivated-received
// ledger, and rule 502 carries the debt carry and the spoiled counter (plus
// the stock only when no grant exists for that owner this pulse). The
// committed state is exactly the frozen sequential semantics — the h2 pulse
// pin in the runner tests asserts the spoil debt already includes the same
// pulse's granted unit — and the grant seam (granted =
// ΣcultivatedTotal) stays on rule 508.
var ErrNaturalRunner = errors.New("invalid natural runner")

// naturalClockActor drives the hourly pulse. The frozen ID partition leaves
// [1001+P, 2000] permanently unassigned: founders are ≤ 8P ≤ 1000, patches
// ≤ 1000+P ≤ 1125 for every NaturalPatchCountValid P, slots start at 2001,
// and allocator newborns are strictly above 2000+8P ≥ 2009, so 2000 is
// disjoint from every roster and kernel entity forever — no hidden
// population cap (P15).
const naturalClockActor = sim.EntityID(NaturalSlotIDBase - 1)

// NaturalStepCeiling is the frozen runner step ceiling: 6048 = 4·168·9
// (four activity-hour lattice instants beyond the pulse's own nine across
// the 168-hour horizon, with headroom over the worst case). It is one of the
// instrumented compute ceilings.
const NaturalStepCeiling = 6048

// naturalPairIntentTLow is the frozen policy bag-decision floor (strategy
// naturalPolicyTLow, v3/v4 world PolicyTLow = 5): the paired stored meal of
// a builder is deferred exactly when the builder sits below the floor with
// an uneaten granary.
const naturalPairIntentTLow int64 = 5

// naturalLostCollision is the typed evidence note on a keyed birth proposal
// that lost its kernel batch to a newborn-ID collision: the whole colliding
// birth (cost patches and allocations) stages nothing structurally and
// retries next hour, deterministically.
const naturalLostCollision = "allocation-collision"

// naturalProvisionRefusalNote is the typed journal note on a refused
// rule-515 provision at the claim (zero mutation, nothing converted, no
// fallback): the world-side precondition revalidation disagreed with the
// policy's claim-time view.
const naturalProvisionRefusalNote = "provision-refused"

// naturalBuildRefusalNotes are the typed journal notes of a refused build
// completion (zero mutation, bag kept).
func naturalBuildRefusalNote(reason NaturalBuildRefusalReason) string {
	switch reason {
	case NaturalBuildRefusalNoWildSlot:
		return "no-wild-slot"
	case NaturalBuildRefusalWildAllStocked:
		return "wild-all-stocked"
	}
	return ""
}

// naturalMealKinds are the journal attempt kinds that convert exactly one
// food unit into energy; they drive the trailing-24h consumption counter of
// the ceiling evidence.
var naturalMealKinds = map[string]bool{
	"eat": true, "eat-stored": true, "paired-meal": true,
	"fallback-eatstored": true, "build-denied-eat": true,
}

// # Compute-ceiling instrumentation (frozen design Q5, P15)
//
// Ceilings are the existing frozen constants, values unchanged. The runner
// keeps an hourly headroom ledger (per ceiling: threshold, current value,
// trailing-hour rate, projected binding hour) and emits a typed
// NaturalCeilingHit — fail-loud — at the first export/commit that would
// exceed a ceiling, or proactively when projected headroom < 1 hour. Silent
// truncation anywhere is a stop condition. The reporter-level verdict
// classification ("outgrew the harness" vs defect) is the experiment lane's
// job; this lane exposes the typed evidence and the headroom counters.
//
// Exactness map: history-events/-patches/-value-nodes/-bytes(event-chain
// contribution) are accumulated incrementally from committed events;
// event-bytes/value-bytes are per-item maxima over committed events;
// snapshot-bytes and the projection loads are measured hourly at the
// checkpoint; fibers/wakes come from the scheduler snapshot; portable-bytes
// is measured at export. The kernel and scheduler enforce the true export
// ceilings authoritatively; the ledger fires typed hits on the crossings it
// can see, never truncates, and classifies an export failure against the
// same thresholds. Declared threshold overrides (options) may only LOWER a
// frozen ceiling — a declared compute-bound fixture for tests, never a
// raised bound and never a modeling cap.
type NaturalCeiling string

const (
	NaturalCeilingHistoryEvents     NaturalCeiling = "history-events"
	NaturalCeilingHistoryPatches    NaturalCeiling = "history-patches"
	NaturalCeilingHistoryValueNodes NaturalCeiling = "history-value-nodes"
	NaturalCeilingHistoryBytes      NaturalCeiling = "history-bytes"
	NaturalCeilingProjectionBytes   NaturalCeiling = "history-projection-bytes"
	NaturalCeilingProjectedMetrics  NaturalCeiling = "history-projected-metrics"
	NaturalCeilingSnapshotBytes     NaturalCeiling = "component-snapshot-bytes"
	NaturalCeilingEventBytes        NaturalCeiling = "event-bytes"
	NaturalCeilingValueBytes        NaturalCeiling = "value-bytes"
	NaturalCeilingPortableBytes     NaturalCeiling = "portable-bytes"
	NaturalCeilingPortableFibers    NaturalCeiling = "portable-fibers"
	NaturalCeilingPortableWakes     NaturalCeiling = "portable-wakes"
	NaturalCeilingRunnerSteps       NaturalCeiling = "runner-steps"
	NaturalCeilingAllocatorIDs      NaturalCeiling = "allocator-ids"
)

// naturalCeilingOrder is the fixed ledger order (deterministic rows).
var naturalCeilingOrder = []NaturalCeiling{
	NaturalCeilingHistoryEvents, NaturalCeilingHistoryPatches, NaturalCeilingHistoryValueNodes,
	NaturalCeilingHistoryBytes, NaturalCeilingProjectionBytes, NaturalCeilingProjectedMetrics,
	NaturalCeilingSnapshotBytes, NaturalCeilingEventBytes, NaturalCeilingValueBytes,
	NaturalCeilingPortableBytes, NaturalCeilingPortableFibers, NaturalCeilingPortableWakes,
	NaturalCeilingRunnerSteps, NaturalCeilingAllocatorIDs,
}

// naturalHourlyProjectedCeilings are the ceilings with an hourly observed
// value and therefore a trailing-hour projection; the others are per-item or
// export-time quantities checked exactly where they arise.
var naturalHourlyProjectedCeilings = []NaturalCeiling{
	NaturalCeilingHistoryEvents, NaturalCeilingHistoryPatches, NaturalCeilingHistoryValueNodes,
	NaturalCeilingHistoryBytes, NaturalCeilingProjectionBytes, NaturalCeilingProjectedMetrics,
	NaturalCeilingSnapshotBytes, NaturalCeilingPortableFibers, NaturalCeilingPortableWakes,
	NaturalCeilingRunnerSteps,
}

// naturalCeilingIsHourlyProjected reports whether a ceiling has an hourly
// observed value and therefore a real trailing-hour rate (and a legitimate
// projected binding hour).
func naturalCeilingIsHourlyProjected(ceiling NaturalCeiling) bool {
	for _, candidate := range naturalHourlyProjectedCeilings {
		if candidate == ceiling {
			return true
		}
	}
	return false
}

// NaturalFrozenCeilingThresholds quotes the frozen compute ceilings: kernel
// history constants, the component snapshot bound, the kernel event-value
// bounds (kernel maxEventBytes is unexported; its frozen value 1<<20 is
// pinned here and enforced authoritatively by the kernel at encode/export),
// the scheduler portable bounds, the runner step ceiling, and the allocator
// MaxUint64 sentinel (typed failure only there, P15).
func NaturalFrozenCeilingThresholds() map[NaturalCeiling]uint64 {
	return map[NaturalCeiling]uint64{
		NaturalCeilingHistoryEvents:     kernel.MaxHistoryEvents,
		NaturalCeilingHistoryPatches:    kernel.MaxHistoryPatches,
		NaturalCeilingHistoryValueNodes: kernel.MaxHistoryValueNodes,
		NaturalCeilingHistoryBytes:      kernel.MaxHistoryBytes,
		NaturalCeilingProjectionBytes:   kernel.MaxHistoryProjectionBytes,
		NaturalCeilingProjectedMetrics:  kernel.MaxHistoryProjectedMetrics,
		NaturalCeilingSnapshotBytes:     component.MaxSnapshotBytes,
		NaturalCeilingEventBytes:        1 << 20,
		NaturalCeilingValueBytes:        sim.MaxValueBytes,
		NaturalCeilingPortableBytes:     scheduler.MaxPortableBytes,
		NaturalCeilingPortableFibers:    scheduler.MaxPortableFibers,
		NaturalCeilingPortableWakes:     scheduler.MaxPortableWakes,
		NaturalCeilingRunnerSteps:       NaturalStepCeiling,
		NaturalCeilingAllocatorIDs:      ^uint64(0),
	}
}

// NaturalCeilingRow is one hourly ledger row.
type NaturalCeilingRow struct {
	Ceiling              NaturalCeiling
	Threshold            uint64
	Observed             uint64
	RateLastHour         int64 // trailing-hour delta; 0 at the first checkpoint
	ProjectedBindingHour int64 // hour at which the rate binds; -1 when not projectable
}

// NaturalCeilingHit is the typed first-class evidence of a ceiling event.
type NaturalCeilingHit struct {
	Ceiling            NaturalCeiling
	Threshold          uint64
	Observed           uint64
	Hour               int
	WorldVersion       sim.WorldVersion
	Alive              int
	Births             int64
	Deaths             int64
	Last24hInflow      int64
	Last24hConsumption int64
	RegenUnrealized    int64
}

// NaturalCeilingHitError carries the typed hit; the run stops fail-loud.
type NaturalCeilingHitError struct{ Hit NaturalCeilingHit }

func (e *NaturalCeilingHitError) Error() string {
	return fmt.Sprintf("%s: ceiling %q threshold %d observed %d at h%d (v%d, alive %d, births %d, deaths %d, 24h inflow %d, 24h consumed %d, regen-unrealized %d)",
		ErrNaturalRunner.Error(), string(e.Hit.Ceiling), e.Hit.Threshold, e.Hit.Observed, e.Hit.Hour, e.Hit.WorldVersion,
		e.Hit.Alive, e.Hit.Births, e.Hit.Deaths, e.Hit.Last24hInflow, e.Hit.Last24hConsumption, e.Hit.RegenUnrealized)
}
func (e *NaturalCeilingHitError) Is(target error) bool { return target == ErrNaturalRunner }

// naturalProjectBindingHour is the pure projection: with a positive trailing
// rate the ceiling binds after ceil(headroom/rate) further hours; already
// over binds now; without a rate it never projects.
func naturalProjectBindingHour(threshold, observed uint64, rate int64, hour int) int64 {
	if observed > threshold {
		return int64(hour)
	}
	if rate <= 0 {
		return -1
	}
	headroom := int64(threshold - observed)
	return int64(hour) + (headroom+rate-1)/rate
}

// naturalLedger is the runner-owned headroom state. thresholds is read-only
// after construction (worker goroutines read it); observed/prev are only
// touched under the coordinator lock.
type naturalLedger struct {
	thresholds map[NaturalCeiling]uint64
	observed   map[NaturalCeiling]uint64
	prev       map[NaturalCeiling]uint64
}

func newNaturalLedger(overrides map[NaturalCeiling]int64) (*naturalLedger, error) {
	frozen := NaturalFrozenCeilingThresholds()
	l := &naturalLedger{thresholds: make(map[NaturalCeiling]uint64, len(frozen)), observed: make(map[NaturalCeiling]uint64, len(frozen)), prev: make(map[NaturalCeiling]uint64, len(frozen))}
	for _, ceiling := range naturalCeilingOrder {
		l.thresholds[ceiling] = frozen[ceiling]
	}
	for ceiling, value := range overrides {
		if _, ok := l.thresholds[ceiling]; !ok {
			return nil, fmt.Errorf("%w: unknown ceiling %q", ErrNaturalRunner, string(ceiling))
		}
		if value < 1 || uint64(value) > l.thresholds[ceiling] {
			return nil, fmt.Errorf("%w: ceiling %q override %d must lower the frozen threshold %d", ErrNaturalRunner, string(ceiling), value, l.thresholds[ceiling])
		}
		l.thresholds[ceiling] = uint64(value)
	}
	return l, nil
}

func (l *naturalLedger) add(ceiling NaturalCeiling, delta uint64) { l.observed[ceiling] += delta }
func (l *naturalLedger) max(ceiling NaturalCeiling, value uint64) {
	if value > l.observed[ceiling] {
		l.observed[ceiling] = value
	}
}
func (l *naturalLedger) set(ceiling NaturalCeiling, value uint64) { l.observed[ceiling] = value }

// crossing reports the first ceiling whose observed value exceeds its
// (possibly lowered) threshold, in the fixed ledger order.
func (l *naturalLedger) crossing() (NaturalCeiling, bool) {
	for _, ceiling := range naturalCeilingOrder {
		if l.observed[ceiling] > l.thresholds[ceiling] {
			return ceiling, true
		}
	}
	return "", false
}

// projected reports the first ceiling whose projected value one hour ahead
// would exceed its threshold (projected headroom < 1 hour).
func (l *naturalLedger) projected() (NaturalCeiling, bool) {
	for _, ceiling := range naturalHourlyProjectedCeilings {
		observed, rate := l.observed[ceiling], int64(l.observed[ceiling])-int64(l.prev[ceiling])
		if rate > 0 && observed+uint64(rate) > l.thresholds[ceiling] {
			return ceiling, true
		}
	}
	return "", false
}

func (l *naturalLedger) rows(hour int) []NaturalCeilingRow {
	out := make([]NaturalCeilingRow, 0, len(naturalCeilingOrder))
	for _, ceiling := range naturalCeilingOrder {
		observed := l.observed[ceiling]
		// Only the hourly-projected ceilings have a meaningful trailing-hour
		// rate; the others (per-item byte maxima, allocator IDs, portable
		// bytes) are not sampled hourly, so reporting a rate or a binding
		// hour for them would be a fabricated forecast.
		rate := int64(0)
		if naturalCeilingIsHourlyProjected(ceiling) {
			rate = int64(observed) - int64(l.prev[ceiling])
		}
		out = append(out, NaturalCeilingRow{Ceiling: ceiling, Threshold: l.thresholds[ceiling], Observed: observed,
			RateLastHour: rate, ProjectedBindingHour: naturalProjectBindingHour(l.thresholds[ceiling], observed, rate, hour)})
	}
	return out
}

// NaturalMealIntent is the deferred stored meal of one hour (v4 verbatim).
type NaturalMealIntent struct {
	Hour   int
	Paired bool
}

// NaturalBirthEvidence is privileged journal evidence for one decided birth
// reply (v4 semantics; claims vs truth live only here and in the aggregate
// metrics — never in any actor-visible observation).
type NaturalBirthEvidence struct {
	Decision        NaturalReplyDecision
	Reason          NaturalRefusalReason
	ReportedCapital int64
	ReportedGranary int64
	TrueCapital     int64
	TrueGranary     int64
	Diverged        bool
	Newborn         sim.EntityID
}

// NaturalAttempt is one typed piece of runner evidence (v4 two-flag
// semantics: Accepted + EventID link a keyed attempt to its committed event;
// the one legitimate keyless key-carrier is a birth that lost an allocator
// collision; refused builds and unkeyed denials mutate nothing).
type NaturalAttempt struct {
	Actor     sim.EntityID
	Time      sim.SimTime
	Kind      string // gather | eat-stored | provision | wait | eat | build | build-denied-eat | paired-meal | fallback-eatstored | rest | birth-request | birth-reply | labour-offer | labour-decision | labour-settlement | labour-void
	Key       string
	Target    sim.EntityID
	Rejection FoodFlowGatherRejection
	NoMeal    string
	Lost      string
	Refused   string // typed build-refusal note; empty otherwise
	// ProvisionRefused is the typed note on a refused rule-515 provision
	// (zero mutation, nothing converted, no fallback). It is omitted when
	// empty so every recorded v5 attempt keeps its journal bytes.
	ProvisionRefused string `json:"ProvisionRefused,omitempty"`
	BirthWake        bool
	Birth            NaturalBirthEvidence
	Accepted         bool
	EventID          sim.EventID
}

// NaturalBatch is one committed timestamp's journal evidence.
type NaturalBatch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []NaturalAttempt
}

// NaturalMetrics are the frozen runner metrics carried by every hourly
// checkpoint; the hourly regen split is the committed per-hour delta of the
// patch ledgers.
type NaturalMetrics struct {
	Alive            int
	DeathsAge        int64
	DeathsStarvation int64
	Births           int64
	BirthRequests    int64
	Divergences      int64
	Refusals         map[NaturalRefusalReason]int64
	RegenCultivated  int64 // this hour's cultivated grants (flat, rule 508)
	RegenWild        int64 // this hour's wild fills
	RegenUnrealized  int64 // this hour's unrealized budget
}

// NaturalCheckpoint is the hourly projection at the pulse boundary: the
// complete authoritative v5 state, the extended conservation balance, the
// allocator last-ID cross-check input, the frozen metrics, and the headroom
// ledger.
type NaturalCheckpoint struct {
	Hour          int
	Version       sim.WorldVersion
	Balance       NaturalBalance
	Patches       []NaturalPatchState
	Slots         [][]NaturalSlotState
	Founders      []NaturalActorState
	Newborns      []NaturalNewbornState
	AllocatorLast sim.EntityID
	Metrics       NaturalMetrics
	Ledger        []NaturalCeilingRow
}

// NaturalHandoff is the value-only, quiescent input to the persistence lane.
// It carries the portable kernel and scheduler bytes, the verified head, the
// full in-flight and roster runner state, the allocator last-ID (restore
// cross-checks it against the max roster ID, fail closed), and the ceiling
// ledger and hits.
type NaturalHandoff struct {
	FormatVersion     uint32
	Patches           int
	R                 int64
	Seed              uint64
	Births            bool
	Policy            strategy.NaturalRef
	Steps             int
	History           []byte
	SchedulerBytes    []byte
	Head              kernel.PortableHead
	Newborns          []sim.EntityID
	BirthWakes        map[sim.EntityID]int64
	Current           map[sim.EntityID]strategy.NaturalAction
	DeniedGather      map[sim.EntityID]int64
	DeniedBuild       map[sim.EntityID]int64
	Meals             map[sim.EntityID]NaturalMealIntent
	AllocatorLast     sim.EntityID
	CeilingThresholds map[NaturalCeiling]uint64
	Ledger            []NaturalCeilingRow
	Hits              []NaturalCeilingHit
	Journal           []NaturalBatch
	Checkpoints       []NaturalCheckpoint
}

// NaturalOptions is the declared fixture shape. P and R are fixture
// parameters (the frozen matrix uses P ∈ {2, 32} and R ∈ {0, 8}); the
// ceiling-threshold overrides are a declared compute-bound instrumentation
// fixture that may only lower frozen ceilings.
type NaturalOptions struct {
	Patches int
	R       int64 // patch regeneration budget per patch-hour, [0, NaturalSlotsPerPatch]
	Seed    uint64
	Workers int
	Births  bool

	// PolicyRef is the declared policy identity (ticket 16 D1 seam): the
	// single binding site resolves it through the frozen natural ref set
	// {natural/5, natural/6, natural/7, natural/7b, natural/9} and binds every roster
	// entity by it. The zero value is exactly natural/5, so every recorded
	// v5 scenario keeps its bytes; the ref is run identity end to end
	// (journal config, checkpoint manifest, bidirectional mismatch
	// rejection).
	PolicyRef strategy.NaturalRef

	// Mechanism is the declared mechanism-version axis (ticket 17 v7
	// design): the world version the runner runs on. The zero value is
	// exactly NaturalMechanismV5 — the frozen v5 world — so every recorded
	// v5 scenario keeps its bytes; NaturalMechanismV6 additively carries
	// rule 515 "provision" and the granary provisioned ledger. The axis is
	// run identity end to end (checkpoint manifest mechanism block, journal
	// config, bidirectional {5,6} mismatch rejection) and is resolved and
	// validated at construction together with the ref/mechanism consistency
	// rule: natural/5 and natural/6 are legal under either mechanism, while
	// natural/7 and natural/7b require mechanism v6 and are refused under
	// the v5 zero value (fail closed) — the provision affordance does not
	// exist there.
	Mechanism NaturalMechanism

	// Founders is the declared founding-band fixture axis (ticket 18 v8
	// design): the number of founder rows the genesis materializes from the
	// frozen founder-stream positions 0..f−1, each seated by the frozen
	// ID-placed home-patch mapping (1–8 → patch 1001, 9–16 → patch 1002,
	// …). The zero value is exactly the full frozen block 8·Patches, so
	// every recorded world is unchanged; an explicit 8·Patches is the same
	// spelling of the same world (byte-identical bundles, pinned by test).
	// The bound is derived from the frozen ID-space partition (the founder
	// block must stay below the patch base, so 1 ≤ f ≤ 8·P; checked by
	// NaturalFoundersValid). The axis is an initial condition of the
	// experiment, never a population cap: newborns stay allocator-issued
	// above the frozen genesis last-ID 2000+8P regardless of f, and the
	// ceiling remains the derived carrying capacity. Run identity end to
	// end (checkpoint manifest founders field, journal config,
	// bidirectional mismatch rejection); no rule, constant, or policy byte
	// changes with it.
	Founders int

	// CultivationDisabled is the declared cultivation ablation of the frozen
	// matrix (cell D): an experiment intervention, never a mechanism or
	// policy change. The frozen policy bytes are unchanged — the runner
	// routes every policy Build choice through the existing documented
	// build-denial loop (zero mutation, bag kept, LastDenial =
	// NaturalBuildDenied, same-hour denial-eat), so no rule-505 event and no
	// committed build attempt can exist; a fail-loud guard turns either into
	// a runner error. The flag is part of run identity (journal config and
	// checkpoint manifest, bidirectionally mismatch-checked like the births
	// flag).
	CultivationDisabled bool

	// CeilingThresholds lowers frozen ceiling thresholds for the typed
	// CeilingHit path (declared run identity; carried by the handoff). Only
	// lowering is valid; an unknown ceiling or a raise is refused.
	CeilingThresholds map[NaturalCeiling]int64

	// Cognition is the optional Role A decision seam (spec section 4). The
	// zero value (nil) leaves every code path and every recorded run
	// byte-identical; when non-nil the declared wake policy selects the
	// escalated claim decisions, which are decided by the model through the
	// recorded-call path. It is not a mechanism or policy change.
	Cognition *NaturalCognition
}

// policyRef resolves the declared policy identity: the zero value is the
// frozen natural/5 ref (D1 default-preservation), anything else must be a
// member of the frozen natural ref set, checked at NewNatural.
func (o NaturalOptions) policyRef() strategy.NaturalRef {
	if o.PolicyRef == (strategy.NaturalRef{}) {
		return strategy.FrozenNaturalPolicy().Ref
	}
	return o.PolicyRef
}

// mechanism resolves the declared mechanism axis: the zero value is the
// frozen v5 mechanism (default-preservation); literal 5 is not a second v5
// spelling and fails validation with every other non-admitted value.
func (o NaturalOptions) mechanism() NaturalMechanism {
	if o.Mechanism == 0 {
		return NaturalMechanismV5
	}
	return o.Mechanism
}

// NaturalFoundersValid is the derived bound of the founding-band fixture

// axis: f == 0 is the default spelling of the full frozen founder block 8P,
// and any explicit band must materialize a non-empty prefix of that block —
// 1 ≤ f ≤ 8P, the bound derived from the frozen ID-space partition (the
// founder block stays below the patch base, so NaturalFounderCount is the
// exact admissible maximum). Everything else is refused.
func NaturalFoundersValid(patches, founders int) bool {
	if !NaturalPatchCountValid(patches) {
		return false
	}
	if founders == 0 {
		return true
	}
	block, err := NaturalFounderCount(patches)
	return err == nil && founders >= 1 && founders <= block
}

// naturalFoundersResolved resolves the declared founding band to the
// materialized founder count: the zero value is exactly the full frozen
// founder block 8P, so the runner always carries a concrete 1 ≤ f ≤ 8P.
func naturalFoundersResolved(patches, founders int) int {
	if founders == 0 {
		return NaturalSlotsPerPatch * patches
	}
	return founders
}

// declaredFounders reports the runner's founding band in its declared
// (omitempty) form: zero at the full frozen block 8P default, so both
// default spellings publish byte-identical manifest and journal fields.
func (f *Natural) declaredFounders() int {
	if f.founders == NaturalSlotsPerPatch*f.patches {
		return 0
	}
	return f.founders
}

// naturalRefLegalUnderMechanism is the ref/mechanism consistency ladder of
// the frozen v9 design (ticket 20; the explicit rungs recorded by the L2
// lane, replacing ticket 17's v6 blanket): natural/5 and natural/6 are the
// base policies legal under ANY admitted mechanism (including mechanism v7,
// so the zero-use control can construct); natural/7 and natural/7b select
// the rule-515 affordance and are legal only under mechanism >= v6 (refused
// under the v5 zero value, fail closed); natural/9 selects the wage-contract
// rules 516-519 and the wage ledgers that only mechanism v7 registers and is
// legal ONLY under mechanism v7. An unknown ref fails for every mechanism.
func naturalRefLegalUnderMechanism(m NaturalMechanism, ref strategy.NaturalRef) bool {
	if !NaturalMechanismValid(m) {
		return false
	}
	policy, err := strategy.FrozenNaturalPolicyForRef(ref)
	if err != nil {
		return false
	}
	switch policy.Ref {
	case strategy.FrozenNaturalPolicy().Ref, strategy.FrozenNaturalPolicyV6().Ref:
		return true
	case strategy.FrozenNaturalPolicy7().Ref, strategy.FrozenNaturalPolicy7b().Ref:
		return m == NaturalMechanismV6 || m == NaturalMechanismV7
	case strategy.FrozenNaturalPolicy9().Ref:
		return m == NaturalMechanismV7
	}
	return false
}

func (o NaturalOptions) validate() error {
	if !NaturalPatchCountValid(o.Patches) {
		return fmt.Errorf("%w: patches %d outside the derived ID-space precondition", ErrNaturalRunner, o.Patches)
	}
	if _, err := strategy.FrozenNaturalPolicyForRef(o.policyRef()); err != nil {
		return fmt.Errorf("%w: policy ref %+v is not in the frozen natural ref set {natural/5, natural/6, natural/7, natural/7b, natural/9}: %v", ErrNaturalRunner, o.PolicyRef, err)
	}
	if !NaturalMechanismValid(o.mechanism()) {
		return fmt.Errorf("%w: mechanism %d is not an admitted mechanism version", ErrNaturalRunner, uint16(o.Mechanism))
	}
	if !naturalRefLegalUnderMechanism(o.mechanism(), o.policyRef()) {
		return fmt.Errorf("%w: policy ref %+v is not legal under mechanism %d (natural/7 and natural/7b require mechanism >= v6; natural/9 requires mechanism v7)", ErrNaturalRunner, o.policyRef(), uint16(o.mechanism()))
	}
	if !NaturalFoundersValid(o.Patches, o.Founders) {
		return fmt.Errorf("%w: founders %d outside the derived founding-band bound [1,%d] (zero = default 8P)", ErrNaturalRunner, o.Founders, NaturalSlotsPerPatch*o.Patches)
	}
	if o.R < 0 || o.R > NaturalSlotsPerPatch {
		return fmt.Errorf("%w: R %d outside [0,%d]", ErrNaturalRunner, o.R, NaturalSlotsPerPatch)
	}
	if o.Workers < 1 {
		return fmt.Errorf("%w: workers %d", ErrNaturalRunner, o.Workers)
	}
	if err := o.Cognition.validate(); err != nil {
		return err
	}
	return nil
}

type naturalBuildOutcome struct {
	proposal kernel.Proposal
	refused  NaturalBuildRefusalReason
	// voidNote is the runner-level structural void of a completion the fold
	// refused before it was proposed (the recorded settlement precedence);
	// zero mutation, the bag kept for the denial loop. It is empty unless
	// refused is NaturalBuildRefusalNone and no proposal exists.
	voidNote string
}

type naturalDeath struct {
	Actor sim.EntityID
	Cause string // "starvation" (504) | "age" (514)
}

type naturalHourFlow struct{ inflow, meals int64 }

// naturalPending accumulates one step's evidence and post-commit runner
// state under its own lock (v4 discipline): worker goroutines append, the
// coordinator applies after the scheduler atomically closes the timestamp.
type naturalPending struct {
	mu              sync.Mutex
	attempts        []NaturalAttempt
	started         map[sim.EntityID]strategy.NaturalAction
	completed       []sim.EntityID
	denied          []sim.EntityID
	deniedBuild     []sim.EntityID
	meals           map[sim.EntityID]NaturalMealIntent
	mealsCleared    []sim.EntityID
	birthWakesUsed  []sim.EntityID
	deaths          []naturalDeath
	replyReasons    []NaturalRefusalReason
	divergedReplies int
	birthFiles      int
	hits            []NaturalCeilingHit
}

// Natural is the v5 runner. All evidence and state below is value-only; the
// roster and every table are maps/slices keyed by allocator-issued IDs and
// iterated in ascending ID order.
type Natural struct {
	mu                  sync.Mutex
	k                   *kernel.Kernel
	sched               *scheduler.Scheduler
	registry            component.Registry
	policies            *strategy.NaturalRegistry
	ref                 strategy.NaturalRef
	mechanism           NaturalMechanism // the declared mechanism axis; zero ≡ v5
	founders            int              // the declared founding band, resolved: always 1 ≤ f ≤ 8P (zero declared ≡ 8P)
	bound               map[sim.EntityID]*strategy.NaturalBound
	patches             int
	seed                uint64
	r                   int64
	births              bool
	roster              []sim.EntityID // founders 1..8P then newborns ascending (sorted invariant)
	newborns            []sim.EntityID // registration (= birth) order == ascending IDs
	pulseLifecycle      map[sim.EntityID]scheduler.Lifecycle
	current             map[sim.EntityID]strategy.NaturalAction // Gather|Build activity in flight
	denied              map[sim.EntityID]int64                  // last denied-gather hour
	deniedBuild         map[sim.EntityID]int64                  // last refused-build hour (same-hour re-eat)
	meals               map[sim.EntityID]NaturalMealIntent      // deferred paired/fallback meals
	birthWakes          map[sim.EntityID]int64                  // newborn -> hour of its reserved WakeBirth claim
	admissions          map[sim.EntityID]FoodFlowGatherAdmission
	builds              map[sim.EntityID]naturalBuildOutcome
	settles             map[sim.EntityID]naturalSettleOutcome
	allocator           sim.EntityAllocator // last-ID cross-checked against the max roster ID at restore
	pending             *naturalPending
	journal             []NaturalBatch
	checkpoints         []NaturalCheckpoint
	steps               int
	deathsAge           int64
	deathsStarve        int64
	birthFiles          int64
	divergences         int64
	refusals            map[NaturalRefusalReason]int64
	ledger              *naturalLedger
	thresholds          map[NaturalCeiling]uint64 // declared copy for the handoff
	hits                []NaturalCeilingHit
	halted              error
	cultivationDisabled bool
	flow                [24]naturalHourFlow // trailing-24-pulse inflow/consumption ring
	hourMeals           int64               // meals committed during the running hour

	// Optional Role A decision seam (spec section 4) and the seam-overridden
	// claim choice recorded for the evaluation phase. A nil cognition and an
	// empty map are the frozen path.
	cognition      *NaturalCognition
	claimDecisions map[sim.EntityID]strategy.NaturalChoice

	// Optional Role B active generation (spec section 5). Nil is the frozen
	// path; non-nil overrides the claim/bag and birth-request cascades for the
	// actors it binds (see natural_cognitive_strategy.go).
	cognitiveStrategy *NaturalCognitiveStrategy
	// cognitiveCheckpoint is the caller-supplied cognition binding the next
	// checkpoint manifest records (spec section 7.2). Nil leaves the manifest
	// byte-unchanged.
	cognitiveCheckpoint *NaturalCognitiveCheckpoint
}

func naturalField(field sim.FieldID, n int64) component.FieldSeed {
	return component.FieldSeed{Field: field, Value: sim.IntegerValue(n)}
}

// granaryProvisionedSeed is the genesis seed of the provisioned ledger
// (granary field 7) for every mechanism that registers it: v6 and — with
// the wage ledgers — v7 (the nine-field granary). The v5 zero value returns
// nothing: the frozen v5 granary seeds keep exactly their six fields.
func granaryProvisionedSeed(mechanism NaturalMechanism, provisioned int64) []component.FieldSeed {
	if mechanism != NaturalMechanismV6 && mechanism != NaturalMechanismV7 {
		return nil
	}
	return []component.FieldSeed{{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(provisioned)}}
}

// granaryWageLedgerSeeds are the mechanism-v7-only genesis seeds of the wage
// ledgers (granary fields 8 wages-paid and 9 wages-earned, ticket 20): the
// v5 and v6 zero values return nothing, so their granary rows keep exactly
// their frozen field lists (six and seven fields).
func granaryWageLedgerSeeds(mechanism NaturalMechanism, wagesPaid, wagesEarned int64) []component.FieldSeed {
	if mechanism != NaturalMechanismV7 {
		return nil
	}
	return []component.FieldSeed{
		{Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(wagesPaid)},
		{Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(wagesEarned)}}
}

// labourRowSeeds are the mechanism-v7-only genesis seeds of the two
// wage-contract rows every actor carries (ticket 20): the canonical
// never-filed offer row on 0x80000058 and the never-engaged book row on
// 0x80000059, both seeded at their idle shapes. Every other mechanism
// returns nothing: the rows do not exist there.
func labourRowSeeds(mechanism NaturalMechanism, actor sim.EntityID, a NaturalActorState) ([]component.ComponentSeed, error) {
	if mechanism != NaturalMechanismV7 {
		return nil, nil
	}
	offeree, err := genomeRefValue(a.Offer.Addressee)
	if err != nil {
		return nil, err
	}
	engaged, err := genomeRefValue(a.Book.EngagedWorker)
	if err != nil {
		return nil, err
	}
	return []component.ComponentSeed{
		{Entity: actor, Component: NaturalLabourOfferTypeID, Fields: []component.FieldSeed{
			naturalField(NaturalLabourOfferHourField, a.Offer.Hour), {Field: NaturalLabourOfferAddresseeField, Value: offeree},
			naturalField(NaturalLabourOfferStatusField, int64(a.Offer.Status)), naturalField(NaturalLabourOfferLastOfferHourField, a.Offer.LastOfferHour),
			naturalField(NaturalLabourOfferLastReplyHourField, a.Offer.LastReplyHour)}},
		{Entity: actor, Component: NaturalLabourBookTypeID, Fields: []component.FieldSeed{
			{Field: NaturalLabourBookEngagedWorkerField, Value: engaged},
			naturalField(NaturalLabourBookEngagedHourField, a.Book.EngagedHour),
			naturalField(NaturalLabourBookLastDecisionHourField, a.Book.LastDecisionHour)}}}, nil
}

// naturalNumber writes one integer field patch at the runner's mechanism
// schema version: the v5 zero value writes the frozen NaturalSchemaVersion,
// so the frozen v5 bytes are unchanged; mechanism v6 writes version 6.
func (f *Natural) naturalNumber(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, n int64) component.Patch {
	return component.Patch{Entity: entity, Component: typ, SchemaVersion: f.mechanism.SchemaVersion(), Field: field, Value: sim.IntegerValue(n)}
}

func naturalKey(kind string, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("natural/%s/%016x/%04x", kind, uint64(at), uint64(actor))
}

// naturalProposal frames one runner proposal at the runner's mechanism rule
// version: the v5 zero value writes the frozen NaturalRuleVersion; mechanism
// v6 writes version 6.
func (f *Natural) naturalProposal(kind string, at sim.SimTime, actor sim.EntityID, rule sim.RuleID, patches ...component.Patch) kernel.Proposal {
	return kernel.Proposal{Key: naturalKey(kind, at, actor), Time: at, Cause: kernel.Cause{Actor: actor, World: actor == 0}, Rule: rule, RuleVersion: f.mechanism.RuleVersion(), Patches: patches}
}

// naturalGenesisSeeds builds the neutral h0 world: P patches with budget R,
// every slot wild and empty (present patch ref, absent owner), and the
// founding band of f founders in the v3/v4 initial state with founder-stream
// lifetimes — f materializes the prefix of the frozen founder block, drawing
// stream positions 0..f−1, so founder i's lifetime equals the frozen world's
// founder i lifetime for every i ≤ f (the zero-value spelling materializes
// the whole block, 8P). The mechanism selects the seed field lists: the v5
// zero value keeps the frozen six-field granaries; mechanism v6 requires the
// provisioned ledger (field 7, complete-row seeds at genesis), seeded 0;
// mechanism v7 (ticket 20) additionally requires the wage ledgers (fields 8
// and 9, the nine-field granary) and the two idle wage-contract rows
// (0x80000058/59) every v7 actor carries, so the V7 validators' genesis gate
// passes and the (v7, natural/7b) zero-use control constructs unchanged.
func naturalGenesisSeeds(mechanism NaturalMechanism, patches int, r int64, seed uint64, founders int) ([]component.ComponentSeed, []NaturalActorState, error) {
	var seeds []component.ComponentSeed
	if !NaturalFoundersValid(patches, founders) {
		return nil, nil, fmt.Errorf("%w: founders %d outside the derived founding-band bound", ErrNaturalRunner, founders)
	}
	if founders == 0 {
		block, err := NaturalFounderCount(patches)
		if err != nil {
			return nil, nil, err
		}
		founders = block
	}
	lifetimes := sim.NewRandomStream(sim.RandomState{Seed: seed, Stream: NaturalFounderStream})
	actors := make([]NaturalActorState, founders)
	for i := range actors {
		lifetime := NaturalLifetimeBase + int64(lifetimes.Uint64()%NaturalLifetimeSpan)
		actor, err := NaturalFounderState(patches, sim.EntityID(i+1), lifetime)
		if err != nil {
			return nil, nil, err
		}
		// Mechanism v7: the founder's wage-contract rows start at their
		// canonical idle shapes (the frozen constructor's zero rows are the
		// v5/v6 spelling and invalid under the V7 validators).
		if mechanism == NaturalMechanismV7 {
			actor.Offer = NaturalIdleLabourOfferState()
			actor.Book = NaturalIdleLabourBookState()
		}
		actors[i] = actor
	}
	for p := 0; p < patches; p++ {
		patchID, err := NaturalPatchID(patches, p)
		if err != nil {
			return nil, nil, err
		}
		patchRef, err := sim.EntityRefValue(patchID)
		if err != nil {
			return nil, nil, err
		}
		seeds = append(seeds, component.ComponentSeed{Entity: patchID, Component: NaturalPatchTypeID, Fields: []component.FieldSeed{
			naturalField(NaturalPatchYieldField, r), naturalField(NaturalPatchPulsesField, 0),
			naturalField(NaturalPatchProducedField, 0), naturalField(NaturalPatchGrantedField, 0),
			naturalField(NaturalPatchUnrealizedField, 0)}})
		for s := 0; s < NaturalSlotsPerPatch; s++ {
			slotID, err := NaturalSlotID(patches, p, s)
			if err != nil {
				return nil, nil, err
			}
			owner, err := genomeRefValue(0)
			if err != nil {
				return nil, nil, err
			}
			seeds = append(seeds, component.ComponentSeed{Entity: slotID, Component: NaturalSlotTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalSlotStockField, 0), {Field: NaturalSlotPatchField, Value: patchRef},
				naturalField(NaturalSlotGatheredField, 0), naturalField(NaturalSlotTenureModeField, int64(NaturalTenureWild)),
				{Field: NaturalSlotTenureOwnerField, Value: owner}, naturalField(NaturalSlotTenureCultivatedField, 0)}})
		}
	}
	for i, a := range actors {
		actor := sim.EntityID(i + 1)
		homeRef, err := sim.EntityRefValue(a.Bag.Source)
		if err != nil {
			return nil, nil, err
		}
		addressee, err := genomeRefValue(a.Request.Addressee)
		if err != nil {
			return nil, nil, err
		}
		parentA, err := genomeRefValue(a.Genome.ParentA)
		if err != nil {
			return nil, nil, err
		}
		parentB, err := genomeRefValue(a.Genome.ParentB)
		if err != nil {
			return nil, nil, err
		}
		seeds = append(seeds,
			component.ComponentSeed{Entity: actor, Component: NaturalBagTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalBagUnitsField, a.Bag.Units), {Field: NaturalBagSourceField, Value: homeRef}}},
			component.ComponentSeed{Entity: actor, Component: NaturalBodyTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalBodyEnergyField, a.Body.Energy), naturalField(NaturalBodyHungerField, a.Body.Hunger),
				naturalField(NaturalBodyBasalSpentField, a.Body.BasalSpent), naturalField(NaturalBodyCapLostField, a.Body.CapLost),
				naturalField(NaturalBodyConsumedField, a.Body.Consumed), naturalField(NaturalBodyLastGatherHourField, a.Body.LastGatherHour),
				naturalField(NaturalBodyBasalDebtField, a.Body.BasalDebt)}},
			component.ComponentSeed{Entity: actor, Component: NaturalWorksiteTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalWorksiteCapitalField, a.Worksite.Capital), naturalField(NaturalWorksiteWipField, a.Worksite.Wip),
				naturalField(NaturalWorksiteWearDebtField, a.Worksite.WearDebt), naturalField(NaturalWorksiteInvestedUnitsField, a.Worksite.InvestedUnits),
				naturalField(NaturalWorksitePointsCreatedField, a.Worksite.PointsCreated), naturalField(NaturalWorksitePointsDecayedField, a.Worksite.PointsDecayed),
				naturalField(NaturalWorksiteLastBuildHourField, a.Worksite.LastBuildHour)}},
			component.ComponentSeed{Entity: actor, Component: NaturalGranaryTypeID, Fields: append(append([]component.FieldSeed{
				naturalField(NaturalGranaryStockField, a.Granary.Stock), naturalField(NaturalGranaryCultivatedTotalField, a.Granary.CultivatedTotal),
				naturalField(NaturalGranaryStoredMealsField, a.Granary.StoredMeals), naturalField(NaturalGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour),
				naturalField(NaturalGranarySpoilDebtField, a.Granary.SpoilDebt), naturalField(NaturalGranarySpoiledField, a.Granary.Spoiled)},
				granaryProvisionedSeed(mechanism, a.Granary.Provisioned)...),
				granaryWageLedgerSeeds(mechanism, a.Granary.WagesPaid, a.Granary.WagesEarned)...)},
			component.ComponentSeed{Entity: actor, Component: NaturalGenomeTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalGenomeLocusMField, a.Genome.LocusM), naturalField(NaturalGenomeLocusYField, a.Genome.LocusY),
				naturalField(NaturalGenomeLocusWField, a.Genome.LocusW),
				{Field: NaturalGenomeParentAField, Value: parentA}, {Field: NaturalGenomeParentBField, Value: parentB},
				naturalField(NaturalGenomeBirthHourField, a.Genome.BirthHour), naturalField(NaturalGenomeDeathHourField, a.Genome.DeathHour),
				naturalField(NaturalGenomeDiedHourField, a.Genome.DiedHour), naturalField(NaturalGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid)}},
			component.ComponentSeed{Entity: actor, Component: NaturalBirthRequestTypeID, Fields: []component.FieldSeed{
				naturalField(NaturalBirthRequestHourField, a.Request.Hour), {Field: NaturalBirthRequestAddresseeField, Value: addressee},
				naturalField(NaturalBirthRequestReportedCapitalField, a.Request.ReportedCapital), naturalField(NaturalBirthRequestReportedGranaryField, a.Request.ReportedGranary),
				naturalField(NaturalBirthRequestStatusField, int64(a.Request.Status)), naturalField(NaturalBirthRequestLastRequestHourField, a.Request.LastRequestHour),
				naturalField(NaturalBirthRequestLastReplyHourField, a.Request.LastReplyHour)}})
		labourSeeds, err := labourRowSeeds(mechanism, actor, a)
		if err != nil {
			return nil, nil, err
		}
		seeds = append(seeds, labourSeeds...)
	}
	return seeds, actors, nil
}

// NewNatural builds the neutral v5 h0 world: P patches with budget R, the
// declared founding band of 8P founders (the axis zero value; any explicit
// f materializes stream positions 0..f−1) whose immutable lifetimes are the
// runner-owned founder stream positions, the genesis allocator at the
// highest fixed entity ID (2000+8P regardless of f — the last fixed slot of
// the frozen partition, so the first newborn is 2000+8P+1), and the frozen
// policy bound to every roster entity. The h0 world is gated by the frozen
// extended conservation — including the allocator cross-check — before any
// kernel exists.
func NewNatural(o NaturalOptions) (*Natural, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	ledger, err := newNaturalLedger(o.CeilingThresholds)
	if err != nil {
		return nil, err
	}
	thresholds := make(map[NaturalCeiling]uint64, len(ledger.thresholds))
	for ceiling, value := range ledger.thresholds {
		thresholds[ceiling] = value
	}
	reg, err := NaturalRegistryFor(o.mechanism())
	if err != nil {
		return nil, err
	}
	// The single binding site (ticket 16 D1 seam): the declared policy ref is
	// resolved against the executable's frozen ref set and every roster
	// entity below binds the same resolved ref. An unknown ref fails closed
	// before any world exists.
	policy, err := strategy.FrozenNaturalPolicyForRef(o.policyRef())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaturalRunner, err)
	}
	policies := strategy.NewNaturalRegistry(o.Patches)
	if err = policies.RegisterFrozenNatural(policy); err != nil {
		return nil, err
	}
	seeds, actors, err := naturalGenesisSeeds(o.mechanism(), o.Patches, o.R, o.Seed, o.Founders)
	if err != nil {
		return nil, err
	}
	allocator, err := NaturalGenesisAllocator(o.Patches)
	if err != nil {
		return nil, err
	}
	genesisLast := allocator.Last()
	slots := make([][]NaturalSlotState, o.Patches)
	for i := range slots {
		slots[i] = make([]NaturalSlotState, NaturalSlotsPerPatch)
	}
	if _, err = NaturalCheckConservationFor(o.mechanism(), o.Patches, make([]NaturalPatchState, o.Patches), slots, actors, nil, genesisLast); err != nil {
		return nil, fmt.Errorf("%w: neutral h0 world breaks frozen conservation: %v", ErrNaturalRunner, err)
	}
	f := &Natural{registry: reg, policies: policies, ref: policy.Ref, mechanism: o.mechanism(), founders: naturalFoundersResolved(o.Patches, o.Founders), patches: o.Patches, seed: o.Seed, r: o.R, births: o.Births,
		cultivationDisabled: o.CultivationDisabled, cognition: o.Cognition,
		claimDecisions: make(map[sim.EntityID]strategy.NaturalChoice),
		bound:          make(map[sim.EntityID]*strategy.NaturalBound), allocator: allocator, ledger: ledger, thresholds: thresholds,
		pulseLifecycle: make(map[sim.EntityID]scheduler.Lifecycle), current: make(map[sim.EntityID]strategy.NaturalAction),
		denied: make(map[sim.EntityID]int64), deniedBuild: make(map[sim.EntityID]int64), meals: make(map[sim.EntityID]NaturalMealIntent),
		birthWakes: make(map[sim.EntityID]int64), refusals: make(map[NaturalRefusalReason]int64)}
	f.k, err = kernel.New(reg, 0, seeds)
	if err != nil {
		return nil, err
	}
	f.sched, err = scheduler.New(f.k, o.Workers, f.evaluate)
	if err != nil {
		return nil, err
	}
	for i := range actors {
		actor := sim.EntityID(i + 1)
		bound, err := policies.Bind(strategy.NaturalBinding{Actor: actor, Ref: policy.Ref})
		if err != nil {
			return nil, err
		}
		f.bound[actor] = bound
		if err = f.sched.Register(actor); err != nil {
			return nil, err
		}
		f.roster = append(f.roster, actor)
		f.pulseLifecycle[actor] = scheduler.Alive
	}
	if err = f.sched.Register(naturalClockActor); err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: naturalClockActor, At: 0, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}

// # The policy cascade dispatch (ticket 16 D1 seam, extended by tickets 17
// and 20)
//
// The frozen v5 cascade bytes cannot detect a v6 bound: their only ref check
// is obs.Ref against the bound's own ref, which a v6 bound satisfies. The
// ref discipline therefore lives in the dispatch, not the cascade — every
// evaluation site routes through these helpers, which call the matching
// cascade for the runner's bound ref: the frozen v5 Evaluate for
// {natural,5}, EvaluateV6 for {natural,6}, — ticket 17 — Evaluate7 for
// {natural,7} and Evaluate7b for {natural,8} (the v6/v7 cascades' own
// bound-identity guards fail closed under any other bound content), and —
// ticket 20 — Evaluate9 for {natural,9}. Every dispatch site still re-pins
// choice.Ref == f.ref.
func (f *Natural) naturalEvaluate(id sim.EntityID, obs strategy.NaturalObservation, ownOffer strategy.NaturalLabourOfferGuard) (strategy.NaturalChoice, error) {
	if f.cognitiveStrategy != nil {
		if choice, handled, err := f.cognitiveStrategy.evaluateClaim(f.patches, id, obs); handled || err != nil {
			return choice, err
		}
	}
	switch f.ref {
	case strategy.FrozenNaturalPolicyV6().Ref:
		return f.bound[id].EvaluateV6(obs)
	case strategy.FrozenNaturalPolicy7().Ref:
		return f.bound[id].Evaluate7(obs)
	case strategy.FrozenNaturalPolicy7b().Ref:
		return f.bound[id].Evaluate7b(obs)
	case strategy.FrozenNaturalPolicy9().Ref:
		return f.bound[id].Evaluate9(strategy.NaturalLabourObservation{NaturalObservation: obs, OwnOffer: ownOffer})
	}
	return f.bound[id].Evaluate(obs)
}

// naturalLabourOfferGuard projects one actor's wage-contract offer row into
// the policy guard of the rule-516/518 observations.
func naturalLabourOfferGuard(a NaturalActorState) strategy.NaturalLabourOfferGuard {
	return strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestStatus(a.Offer.Status), LastOfferHour: a.Offer.LastOfferHour, LastReplyHour: a.Offer.LastReplyHour}
}

// naturalEvaluateBirthRequest dispatches the frozen proposer decision at
// h+40m+5µs for the bound ref (EvaluateBirthRequestV6 and its natural/7,
// natural/7b and natural/9 twins file only at own energy ≥ 4).
func (f *Natural) naturalEvaluateBirthRequest(id sim.EntityID, obs strategy.NaturalRequestObservation) (strategy.NaturalRequestChoice, error) {
	if f.cognitiveStrategy != nil {
		if choice, handled, err := f.cognitiveStrategy.evaluateBirthRequest(f.patches, id, obs); handled || err != nil {
			return choice, err
		}
	}
	switch f.ref {
	case strategy.FrozenNaturalPolicyV6().Ref:
		return f.bound[id].EvaluateBirthRequestV6(obs)
	case strategy.FrozenNaturalPolicy7().Ref:
		return f.bound[id].EvaluateBirthRequest7(obs)
	case strategy.FrozenNaturalPolicy7b().Ref:
		return f.bound[id].EvaluateBirthRequest7b(obs)
	case strategy.FrozenNaturalPolicy9().Ref:
		return f.bound[id].EvaluateBirthRequest9(obs)
	}
	return f.bound[id].EvaluateBirthRequest(obs)
}

// naturalEvaluateBirthReply dispatches the frozen consenter decision at
// h+40m+6µs for the bound ref (EvaluateBirthReplyV6 and its natural/7,
// natural/7b and natural/9 twins accept only at own energy ≥ 4).
func (f *Natural) naturalEvaluateBirthReply(id sim.EntityID, obs strategy.NaturalReplyObservation) (strategy.NaturalReplyChoice, error) {
	switch f.ref {
	case strategy.FrozenNaturalPolicyV6().Ref:
		return f.bound[id].EvaluateBirthReplyV6(obs)
	case strategy.FrozenNaturalPolicy7().Ref:
		return f.bound[id].EvaluateBirthReply7(obs)
	case strategy.FrozenNaturalPolicy7b().Ref:
		return f.bound[id].EvaluateBirthReply7b(obs)
	case strategy.FrozenNaturalPolicy9().Ref:
		return f.bound[id].EvaluateBirthReply9(obs)
	}
	return f.bound[id].EvaluateBirthReply(obs)
}

// naturalEvaluateLabourOffer dispatches the frozen natural/9 worker filing
// decision at h+40m+5µs (rule 516). The dispatch (not the cascade) carries
// the ref discipline: the 9 bound's own identity guard fails closed under
// any other bound content, and the site re-pins choice.Ref == f.ref.
func (f *Natural) naturalEvaluateLabourOffer(id sim.EntityID, obs strategy.NaturalLabourOfferObservation) (strategy.NaturalLabourOfferChoice, error) {
	return f.bound[id].EvaluateLabourOffer9(obs)
}

// naturalEvaluateLabourDecision dispatches the frozen natural/9 employer
// decision at h+40m+6µs (rule 517) under the same ref discipline.
func (f *Natural) naturalEvaluateLabourDecision(id sim.EntityID, obs strategy.NaturalLabourDecisionObservation) (strategy.NaturalLabourDecisionChoice, error) {
	return f.bound[id].EvaluateLabourDecision9(obs)
}

// naturalLabourLane reports whether the runner's bound ref selects the
// wage-contract lane (ticket 20). Only natural/9 emits the labour actions,
// so every labour wake, pulse transition, and dispatch below is gated on
// it: the (v7, natural/7b) zero-use control keeps the v6 code paths and
// trajectories byte for byte.
func (f *Natural) naturalLabourLane() bool {
	return f.ref == strategy.FrozenNaturalPolicy9().Ref
}

// naturalActorState reads one actor's six component rows at one version.
// The mechanism selects the granary field list: the v5 zero value reads the
// frozen six fields; mechanism v6 additionally reads the provisioned ledger
// (field 7), so G2a′ and the balance carry the real counter; mechanism v7
// (ticket 20) additionally reads the wage ledgers (fields 8 and 9) and the
// actor's two wage-contract rows, so the V7 validators and the labour
// observations see the real counters.
func naturalActorState(view scheduler.SnapshotView, mechanism NaturalMechanism, actor sim.EntityID) (NaturalActorState, error) {
	body, err := capacityInts(view, actor, NaturalBodyTypeID,
		NaturalBodyEnergyField, NaturalBodyHungerField, NaturalBodyBasalSpentField, NaturalBodyCapLostField, NaturalBodyConsumedField, NaturalBodyLastGatherHourField, NaturalBodyBasalDebtField)
	if err != nil {
		return NaturalActorState{}, err
	}
	bagValues, err := capacityRead(view, actor, NaturalBagTypeID, NaturalBagUnitsField, NaturalBagSourceField)
	if err != nil {
		return NaturalActorState{}, err
	}
	units, err := bagValues[0].Integer()
	if err != nil {
		return NaturalActorState{}, err
	}
	source, err := capacityRefValue(bagValues[1])
	if err != nil {
		return NaturalActorState{}, err
	}
	worksite, err := capacityInts(view, actor, NaturalWorksiteTypeID,
		NaturalWorksiteCapitalField, NaturalWorksiteWipField, NaturalWorksiteWearDebtField, NaturalWorksiteInvestedUnitsField, NaturalWorksitePointsCreatedField, NaturalWorksitePointsDecayedField, NaturalWorksiteLastBuildHourField)
	if err != nil {
		return NaturalActorState{}, err
	}
	granaryFields := []sim.FieldID{NaturalGranaryStockField, NaturalGranaryCultivatedTotalField, NaturalGranaryStoredMealsField,
		NaturalGranaryLastStoredMealHourField, NaturalGranarySpoilDebtField, NaturalGranarySpoiledField}
	if mechanism == NaturalMechanismV6 {
		granaryFields = append(granaryFields, NaturalGranaryProvisionedField)
	}
	if mechanism == NaturalMechanismV7 {
		granaryFields = append(granaryFields, NaturalGranaryProvisionedField, NaturalGranaryWagesPaidField, NaturalGranaryWagesEarnedField)
	}
	granary, err := capacityInts(view, actor, NaturalGranaryTypeID, granaryFields...)
	if err != nil {
		return NaturalActorState{}, err
	}
	provisioned := int64(0) // the ledger does not exist under the v5 zero value
	wagesPaid, wagesEarned := int64(0), int64(0)
	if mechanism == NaturalMechanismV6 {
		provisioned = granary[6]
	}
	if mechanism == NaturalMechanismV7 {
		provisioned, wagesPaid, wagesEarned = granary[6], granary[7], granary[8]
	}
	genomeValues, err := capacityRead(view, actor, NaturalGenomeTypeID,
		NaturalGenomeLocusMField, NaturalGenomeLocusYField, NaturalGenomeLocusWField, NaturalGenomeParentAField, NaturalGenomeParentBField,
		NaturalGenomeBirthHourField, NaturalGenomeDeathHourField, NaturalGenomeDiedHourField, NaturalGenomeBirthGranaryPaidField)
	if err != nil {
		return NaturalActorState{}, err
	}
	genomeInts := make([]int64, len(genomeValues))
	for i, v := range genomeValues {
		if i == 3 || i == 4 {
			continue
		}
		if genomeInts[i], err = v.Integer(); err != nil {
			return NaturalActorState{}, err
		}
	}
	parentA, err := capacityRefValue(genomeValues[3])
	if err != nil {
		return NaturalActorState{}, err
	}
	parentB, err := capacityRefValue(genomeValues[4])
	if err != nil {
		return NaturalActorState{}, err
	}
	requestValues, err := capacityRead(view, actor, NaturalBirthRequestTypeID,
		NaturalBirthRequestHourField, NaturalBirthRequestAddresseeField, NaturalBirthRequestReportedCapitalField, NaturalBirthRequestReportedGranaryField,
		NaturalBirthRequestStatusField, NaturalBirthRequestLastRequestHourField, NaturalBirthRequestLastReplyHourField)
	if err != nil {
		return NaturalActorState{}, err
	}
	requestInts := make([]int64, len(requestValues))
	for i, v := range requestValues {
		if i == 1 {
			continue
		}
		if requestInts[i], err = v.Integer(); err != nil {
			return NaturalActorState{}, err
		}
	}
	addressee, err := capacityRefValue(requestValues[1])
	if err != nil {
		return NaturalActorState{}, err
	}
	a := NaturalActorState{
		Body:     NaturalBodyState{Energy: body[0], Hunger: body[1], BasalSpent: body[2], CapLost: body[3], Consumed: body[4], LastGatherHour: body[5], BasalDebt: body[6]},
		Bag:      NaturalBagState{Units: units, Source: source},
		Worksite: NaturalWorksiteState{Capital: worksite[0], Wip: worksite[1], WearDebt: worksite[2], InvestedUnits: worksite[3], PointsCreated: worksite[4], PointsDecayed: worksite[5], LastBuildHour: worksite[6]},
		Granary:  NaturalGranaryState{Stock: granary[0], CultivatedTotal: granary[1], StoredMeals: granary[2], LastStoredMealHour: granary[3], SpoilDebt: granary[4], Spoiled: granary[5], Provisioned: provisioned, WagesPaid: wagesPaid, WagesEarned: wagesEarned},
		Genome:   NaturalGenomeState{LocusM: genomeInts[0], LocusY: genomeInts[1], LocusW: genomeInts[2], ParentA: parentA, ParentB: parentB, BirthHour: genomeInts[5], DeathHour: genomeInts[6], DiedHour: genomeInts[7], BirthGranaryPaid: genomeInts[8]},
		Request:  NaturalBirthRequestState{Hour: requestInts[0], Addressee: addressee, ReportedCapital: requestInts[2], ReportedGranary: requestInts[3], Status: NaturalRequestStatus(requestInts[4]), LastRequestHour: requestInts[5], LastReplyHour: requestInts[6]},
	}
	if mechanism == NaturalMechanismV7 {
		offereValues, err := capacityRead(view, actor, NaturalLabourOfferTypeID,
			NaturalLabourOfferHourField, NaturalLabourOfferAddresseeField, NaturalLabourOfferStatusField, NaturalLabourOfferLastOfferHourField, NaturalLabourOfferLastReplyHourField)
		if err != nil {
			return NaturalActorState{}, err
		}
		offerInts := make([]int64, len(offereValues))
		for i, v := range offereValues {
			if i == 1 {
				continue
			}
			if offerInts[i], err = v.Integer(); err != nil {
				return NaturalActorState{}, err
			}
		}
		offeree, err := capacityRefValue(offereValues[1])
		if err != nil {
			return NaturalActorState{}, err
		}
		bookValues, err := capacityRead(view, actor, NaturalLabourBookTypeID,
			NaturalLabourBookEngagedWorkerField, NaturalLabourBookEngagedHourField, NaturalLabourBookLastDecisionHourField)
		if err != nil {
			return NaturalActorState{}, err
		}
		bookInts := make([]int64, len(bookValues))
		for i, v := range bookValues {
			if i == 0 {
				continue
			}
			if bookInts[i], err = v.Integer(); err != nil {
				return NaturalActorState{}, err
			}
		}
		engagedWorker, err := capacityRefValue(bookValues[0])
		if err != nil {
			return NaturalActorState{}, err
		}
		a.Offer = NaturalLabourOfferState{Hour: offerInts[0], Addressee: offeree, Status: NaturalRequestStatus(offerInts[2]), LastOfferHour: offerInts[3], LastReplyHour: offerInts[4]}
		a.Book = NaturalLabourBookState{EngagedWorker: engagedWorker, EngagedHour: bookInts[1], LastDecisionHour: bookInts[2]}
		if a.Offer.Status < NaturalRequestIdle || a.Offer.Status > NaturalRequestExpired {
			return NaturalActorState{}, ErrNaturalContract
		}
	}
	if a.Request.Status < NaturalRequestIdle || a.Request.Status > NaturalRequestExpired {
		return NaturalActorState{}, ErrNaturalContract
	}
	return a, nil
}

// naturalWorld is one snapshot's full authoritative v5 state.
type naturalWorld struct {
	patches []NaturalPatchState
	slots   [][]NaturalSlotState
	actors  map[sim.EntityID]NaturalActorState
}

// worldState reads the full state for the runner's roster (founders 1..8P by
// ID order plus the registered newborns; the allocator cross-check at the
// checkpoint fails closed on any divergence).
func (f *Natural) worldState(view scheduler.SnapshotView) (naturalWorld, error) {
	return worldStateFor(view, f.mechanism, f.patches, f.roster)
}

// worldStateFor reads the full state for one roster at one version with one
// mechanism's granary field list (the runner's own worldState, generalized
// to an explicit mechanism and roster).
func worldStateFor(view scheduler.SnapshotView, mechanism NaturalMechanism, patches int, roster []sim.EntityID) (naturalWorld, error) {
	w := naturalWorld{patches: make([]NaturalPatchState, patches), slots: make([][]NaturalSlotState, patches), actors: make(map[sim.EntityID]NaturalActorState, len(roster))}
	for i := 0; i < patches; i++ {
		patch, err := NaturalPatchID(patches, i)
		if err != nil {
			return naturalWorld{}, err
		}
		n, err := capacityInts(view, patch, NaturalPatchTypeID, NaturalPatchYieldField, NaturalPatchPulsesField, NaturalPatchProducedField, NaturalPatchGrantedField, NaturalPatchUnrealizedField)
		if err != nil {
			return naturalWorld{}, err
		}
		w.patches[i] = NaturalPatchState{Yield: n[0], Pulses: n[1], Produced: n[2], Granted: n[3], Unrealized: n[4]}
		w.slots[i] = make([]NaturalSlotState, NaturalSlotsPerPatch)
		for j := range w.slots[i] {
			slot, err := NaturalSlotID(patches, i, j)
			if err != nil {
				return naturalWorld{}, err
			}
			ints, err := capacityInts(view, slot, NaturalSlotTypeID, NaturalSlotStockField, NaturalSlotGatheredField, NaturalSlotTenureModeField, NaturalSlotTenureCultivatedField)
			if err != nil {
				return naturalWorld{}, err
			}
			ownerValues, err := capacityRead(view, slot, NaturalSlotTypeID, NaturalSlotTenureOwnerField)
			if err != nil {
				return naturalWorld{}, err
			}
			owner, err := capacityRefValue(ownerValues[0])
			if err != nil {
				return naturalWorld{}, err
			}
			w.slots[i][j] = NaturalSlotState{Stock: ints[0], Gathered: ints[1], Mode: NaturalTenureMode(ints[2]), Owner: owner, Cultivated: ints[3]}
		}
	}
	for _, id := range roster {
		a, err := naturalActorState(view, mechanism, id)
		if err != nil {
			return naturalWorld{}, err
		}
		w.actors[id] = a
	}
	return w, nil
}

// observe builds the actor-visible v5 claim view: the v4 own-set plus the
// home-patch slot stocks and the denial guard. Tenure (mode/owner), debts,
// spoilage counters, allocator state, R, and seeds have no field here at all
// (frozen Q7); cultivated slots read exactly like empty wild slots.
func (f *Natural) observe(view scheduler.SnapshotView, actor sim.EntityID, hour int, denial strategy.NaturalDenial) (strategy.NaturalObservation, error) {
	a, err := naturalActorState(view, f.mechanism, actor)
	if err != nil {
		return strategy.NaturalObservation{}, err
	}
	home := a.Bag.Source
	index := int(home - NaturalPatchIDBase)
	if index < 0 || index >= f.patches {
		return strategy.NaturalObservation{}, ErrNaturalRunner
	}
	obs := strategy.NaturalObservation{
		Actor: actor, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		BagSource: bagSourceValue(a.Bag.Source), LastDenial: denial,
	}
	body, err := capacityRead(view, actor, NaturalBodyTypeID, NaturalBodyEnergyField, NaturalBodyHungerField)
	if err != nil {
		return strategy.NaturalObservation{}, err
	}
	worksite, err := capacityRead(view, actor, NaturalWorksiteTypeID, NaturalWorksiteCapitalField, NaturalWorksiteWipField, NaturalWorksiteInvestedUnitsField, NaturalWorksiteLastBuildHourField)
	if err != nil {
		return strategy.NaturalObservation{}, err
	}
	granary, err := capacityRead(view, actor, NaturalGranaryTypeID, NaturalGranaryStockField, NaturalGranaryLastStoredMealHourField)
	if err != nil {
		return strategy.NaturalObservation{}, err
	}
	bag, err := capacityRead(view, actor, NaturalBagTypeID, NaturalBagUnitsField)
	if err != nil {
		return strategy.NaturalObservation{}, err
	}
	obs.Energy, obs.Hunger, obs.BagUnits = body[0], body[1], bag[0]
	obs.Capital, obs.Wip, obs.InvestedUnits, obs.LastBuildHour = worksite[0], worksite[1], worksite[2], worksite[3]
	obs.GranaryStock, obs.LastStoredMealHour = granary[0], granary[1]
	for j := 0; j < NaturalSlotsPerPatch; j++ {
		slot, err := NaturalSlotID(f.patches, index, j)
		if err != nil {
			return strategy.NaturalObservation{}, err
		}
		stock, err := capacityRead(view, slot, NaturalSlotTypeID, NaturalSlotStockField)
		if err != nil {
			return strategy.NaturalObservation{}, err
		}
		obs.HomeSlots = append(obs.HomeSlots, strategy.NaturalSlotView{ID: slot, Stock: stock[0]})
	}
	return obs, nil
}

// guard is the runner-side ownership duty: every natural proposal passes
// NaturalCheckOwnership before it can be planned (the kernel validates
// schema and rule admission only).
// guard is mechanism-parameterized: the v5 zero value is exactly the frozen
// guard; mechanism v6 admits the rule-515 provision writer set.
func (f *Natural) guard(p kernel.Proposal) error {
	if p.RuleVersion != f.mechanism.RuleVersion() {
		return ErrNaturalRunner
	}
	return NaturalCheckOwnershipFor(f.mechanism, p, f.patches)
}

func (f *Natural) rosterContains(id sim.EntityID) bool {
	_, ok := f.bound[id]
	return ok
}

// pulse drives the hourly pulse; the clock fiber is disjoint from the roster.
func (f *Natural) pulse(ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	var out scheduler.Evaluation
	at := ready.At
	h := int(at / sim.SimTime(NaturalHour))
	if h > NaturalHorizonHours || at != sim.SimTime(h)*sim.SimTime(NaturalHour) {
		return out, ErrNaturalRunner
	}
	w, err := f.worldState(view)
	if err != nil {
		return out, err
	}
	if len(w.actors) != len(f.roster) {
		return out, ErrNaturalRunner
	}
	for _, hour := range f.birthWakes {
		if hour < int64(h) {
			return out, ErrNaturalRunner // a reserved first wake never fired
		}
	}
	if !f.births && len(f.birthWakes) != 0 {
		return out, ErrNaturalRunner
	}
	// Lifecycle must match the frozen alive invariant exactly at the pulse.
	for _, id := range f.roster {
		a, ok := w.actors[id]
		if !ok {
			return out, ErrNaturalRunner
		}
		if (f.pulseLifecycle[id] == scheduler.Alive) != naturalAlive(a) {
			return out, ErrNaturalRunner
		}
	}
	live := make(map[sim.EntityID]bool, len(f.roster))
	death := func(id sim.EntityID, cause string) {
		f.pending.mu.Lock()
		f.pending.deaths = append(f.pending.deaths, naturalDeath{Actor: id, Cause: cause})
		f.pending.mu.Unlock()
	}
	// Stage 1: world-caused age-death (rule 514). The death-hour is
	// immutable; a dying actor stops with its energy retained and never
	// pays basal again.
	for _, id := range f.roster {
		a := w.actors[id]
		if !naturalAlive(a) {
			continue
		}
		if int64(h) < a.Genome.DeathHour {
			live[id] = true
			continue
		}
		genome, died, err := NaturalAgeDeath(f.patches, h, id, a.Genome)
		if err != nil || !died {
			return out, ErrNaturalRunner
		}
		proposal := f.naturalProposal("age-death", at, id, NaturalAgeDeathRule,
			f.naturalNumber(id, NaturalGenomeTypeID, NaturalGenomeDiedHourField, genome.DiedHour))
		proposal.Cause = kernel.Cause{World: true}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
		a.Genome = genome
		w.actors[id] = a
		death(id, "age")
	}
	// Stage 2: basal for the living (rule 504). A starved actor writes its
	// died-hour (only rules 504/514 may) and stops; energy hits zero.
	if h >= 1 {
		for _, id := range f.roster {
			if !live[id] {
				continue
			}
			a := w.actors[id]
			next, _, starved, err := NaturalBasalFor(f.mechanism, f.patches, h, id, a, 1)
			if err != nil {
				return out, err
			}
			ps := []component.Patch{
				f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyEnergyField, next.Body.Energy),
				f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyHungerField, next.Body.Hunger),
				f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyBasalSpentField, next.Body.BasalSpent)}
			if next.Body.BasalDebt != a.Body.BasalDebt {
				ps = append(ps, f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyBasalDebtField, next.Body.BasalDebt))
			}
			if starved {
				ps = append(ps, f.naturalNumber(id, NaturalGenomeTypeID, NaturalGenomeDiedHourField, next.Genome.DiedHour))
			}
			proposal := f.naturalProposal("basal", at, id, NaturalBasalRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			if starved {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
				live[id] = false
				death(id, "starvation")
			}
			w.actors[id] = next
		}
	}
	// Stage 3: regen-split (rule 501) with the flat grants (rule 508):
	// cultivated slots of living owners first, vacant wild slots ascending,
	// remainder regen-unrealized. Fallow slots (dead owners) are skipped by
	// the contract and stay unrealized until wear reverts them. The grant
	// proposals are DEFERRED to stage 4: the kernel's single-winner rule
	// discards any second same-timestamp proposal that writes a field an
	// earlier winner already wrote, and the grant (508) and the rot (502)
	// both move the same owner's granary stock within one pulse, so the two
	// rules must commit with disjoint granary field sets (see stage 4).
	grantsByOwner := make(map[sim.EntityID]int64, len(f.roster))
	if h < NaturalHorizonHours {
		aliveLookup := NaturalAliveLookup(func(id sim.EntityID) bool {
			a, ok := w.actors[id]
			return ok && naturalAlive(a)
		})
		for i := range w.patches {
			next, stock, _, grants, err := NaturalRegenSplit(f.patches, h, w.patches[i], slotArray(w.slots[i]), aliveLookup)
			if err != nil {
				return out, err
			}
			patch, _ := NaturalPatchID(f.patches, i)
			ps := []component.Patch{f.naturalNumber(patch, NaturalPatchTypeID, NaturalPatchPulsesField, next.Pulses)}
			if next.Produced != w.patches[i].Produced {
				ps = append(ps, f.naturalNumber(patch, NaturalPatchTypeID, NaturalPatchProducedField, next.Produced))
			}
			if next.Granted != w.patches[i].Granted {
				ps = append(ps, f.naturalNumber(patch, NaturalPatchTypeID, NaturalPatchGrantedField, next.Granted))
			}
			if next.Unrealized != w.patches[i].Unrealized {
				ps = append(ps, f.naturalNumber(patch, NaturalPatchTypeID, NaturalPatchUnrealizedField, next.Unrealized))
			}
			for j, s := range stock {
				if s.Stock != w.slots[i][j].Stock {
					slot, _ := NaturalSlotID(f.patches, i, j)
					ps = append(ps, f.naturalNumber(slot, NaturalSlotTypeID, NaturalSlotStockField, s.Stock))
				}
			}
			proposal := f.naturalProposal(fmt.Sprintf("regen-%d", i), at, 0, NaturalRegenSplitRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			w.patches[i], w.slots[i] = next, slotSlice(stock)
			// The entitlement moves into the owner's granary on this
			// snapshot (a fallow return here would be a linkage break
			// because the split already filtered by liveness on it); the
			// rule-508 proposal committing it is emitted in stage 4.
			for _, grant := range grants {
				owner := w.actors[grant.Owner]
				granted, grantResult, err := NaturalCultivatedGrantFor(f.mechanism, f.patches, grant.Owner, owner, grant.Units)
				if err != nil || grantResult.Fallowed != 0 {
					return out, fmt.Errorf("%w: fallow grant inside a live-filtered regen split: %v", ErrNaturalRunner, err)
				}
				w.actors[grant.Owner] = granted
				grantsByOwner[grant.Owner] += grant.Units
			}
		}
	}
	// Stage 4: spoilage (rule 502) for every owner, dead included, computed
	// on the post-grant state (the frozen pulse order: grants land before
	// the spoil debt accrues). Field attribution with a same-pulse grant:
	// the kernel forbids two winners on one field, so rule 508 carries the
	// net granary stock (post-grant, post-rot) plus the cultivated-received
	// ledger, and rule 502 carries only the debt carry and the spoiled
	// counter. Without a grant, 502 owns the stock as before. An all-zero
	// granary produces no proposals.
	if h < NaturalHorizonHours {
		for _, id := range f.roster {
			a := w.actors[id]
			next := a
			if h >= 1 {
				var err error
				next, _, err = NaturalSpoilageFor(f.mechanism, f.patches, id, a)
				if err != nil {
					return out, err
				}
			}
			units := grantsByOwner[id]
			var grantPatches, spoilPatches []component.Patch
			if units > 0 {
				grantPatches = append(grantPatches,
					f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranaryStockField, next.Granary.Stock),
					f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranaryCultivatedTotalField, next.Granary.CultivatedTotal))
				if next.Granary.SpoilDebt != a.Granary.SpoilDebt {
					spoilPatches = append(spoilPatches, f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranarySpoilDebtField, next.Granary.SpoilDebt))
				}
				if next.Granary.Spoiled != a.Granary.Spoiled {
					spoilPatches = append(spoilPatches, f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranarySpoiledField, next.Granary.Spoiled))
				}
			} else {
				if next.Granary.SpoilDebt != a.Granary.SpoilDebt {
					spoilPatches = append(spoilPatches, f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranarySpoilDebtField, next.Granary.SpoilDebt))
				}
				if next.Granary.Stock != a.Granary.Stock {
					spoilPatches = append(spoilPatches, f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranaryStockField, next.Granary.Stock))
				}
				if next.Granary.Spoiled != a.Granary.Spoiled {
					spoilPatches = append(spoilPatches, f.naturalNumber(id, NaturalGranaryTypeID, NaturalGranarySpoiledField, next.Granary.Spoiled))
				}
			}
			if len(grantPatches) > 0 {
				gProposal := f.naturalProposal("grant", at, id, NaturalGrantRule, grantPatches...)
				gProposal.Cause = kernel.Cause{World: true}
				if err = f.guard(gProposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, gProposal)
			}
			if len(spoilPatches) > 0 {
				proposal := f.naturalProposal("spoil", at, id, NaturalSpoilageRule, spoilPatches...)
				proposal.Cause = kernel.Cause{World: true}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
			}
			w.actors[id] = next
		}
	}
	// Stage 5: wear (rule 503) for every owner with standing capital, dead
	// owners included; each decayed point reverts the owner's lowest-index
	// cultivated slot to wild with stock 0 in the same atomic pulse.
	if h >= 1 && h < NaturalHorizonHours {
		for _, id := range f.roster {
			a := w.actors[id]
			if a.Worksite.Capital <= 0 {
				continue
			}
			index := int(a.Bag.Source - NaturalPatchIDBase)
			if index < 0 || index >= f.patches {
				return out, ErrNaturalRunner
			}
			next, nextSlots, result, err := NaturalWearFor(f.mechanism, f.patches, id, a, a.Bag.Source, slotArray(w.slots[index]))
			if err != nil {
				return out, err
			}
			ps := []component.Patch{f.naturalNumber(id, NaturalWorksiteTypeID, NaturalWorksiteWearDebtField, next.Worksite.WearDebt)}
			if next.Worksite.Capital != a.Worksite.Capital {
				ps = append(ps, f.naturalNumber(id, NaturalWorksiteTypeID, NaturalWorksiteCapitalField, next.Worksite.Capital))
			}
			if next.Worksite.PointsDecayed != a.Worksite.PointsDecayed {
				ps = append(ps, f.naturalNumber(id, NaturalWorksiteTypeID, NaturalWorksitePointsDecayedField, next.Worksite.PointsDecayed))
			}
			for _, slotIndex := range result.RevertedSlots {
				slot, err := NaturalSlotID(f.patches, index, slotIndex)
				if err != nil {
					return out, err
				}
				owner, err := genomeRefValue(0)
				if err != nil {
					return out, err
				}
				ps = append(ps,
					f.naturalNumber(slot, NaturalSlotTypeID, NaturalSlotTenureModeField, int64(NaturalTenureWild)),
					component.Patch{Entity: slot, Component: NaturalSlotTypeID, SchemaVersion: f.mechanism.SchemaVersion(), Field: NaturalSlotTenureOwnerField, Value: owner},
					f.naturalNumber(slot, NaturalSlotTypeID, NaturalSlotTenureCultivatedField, 0))
			}
			proposal := f.naturalProposal("wear", at, id, NaturalWearRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			w.actors[id] = next
			w.slots[index] = slotSlice(nextSlots)
		}
	}
	// Stage 6: world-caused expiry (rule 513). Every request still pending
	// from h-1 expires at this pulse.
	if h >= 1 {
		for _, id := range f.roster {
			r := w.actors[id].Request
			if r.Status != NaturalRequestPending || r.Hour != int64(h)-1 {
				continue
			}
			next, err := NaturalBirthExpiry(h, r)
			if err != nil {
				return out, err
			}
			proposal := f.naturalProposal("expiry", at, id, NaturalBirthExpireRule,
				f.naturalNumber(id, NaturalBirthRequestTypeID, NaturalBirthRequestStatusField, int64(next.Status)))
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			expired := w.actors[id]
			expired.Request = next
			w.actors[id] = expired
		}
	}
	// Stage 6b: world-caused labour expiry and book clearing (rule 519,
	// ticket 20, the wage-contract lane only). The 513 discipline verbatim
	// plus the void clearing: a pending offer from h-1 expires, an accepted
	// contract whose single delivery hour (accept hour + 1) has passed
	// clears to idle-with-audit, and the matching engagement of hour h-2
	// clears in the same pulse. An offer and its engagement can clear
	// together, so one actor commits at most one rule-519 proposal with the
	// disjoint status patches (offer status first, then the book's cleared
	// engagement fields). Idle rows produce no proposals, so the (v7,
	// natural/7b) zero-use control commits nothing here.
	if h >= 1 && f.naturalLabourLane() {
		for _, id := range f.roster {
			a := w.actors[id]
			offere, offerChanged, err := NaturalLabourOfferExpire(h, a.Offer)
			if err != nil {
				return out, err
			}
			book, bookChanged, err := NaturalLabourBookClear(h, a.Book)
			if err != nil {
				return out, err
			}
			if !offerChanged && !bookChanged {
				continue
			}
			var ps []component.Patch
			if offerChanged {
				ps = append(ps, f.naturalNumber(id, NaturalLabourOfferTypeID, NaturalLabourOfferStatusField, int64(offere.Status)))
			}
			if bookChanged {
				engaged, err := genomeRefValue(book.EngagedWorker)
				if err != nil {
					return out, err
				}
				ps = append(ps,
					component.Patch{Entity: id, Component: NaturalLabourBookTypeID, SchemaVersion: f.mechanism.SchemaVersion(), Field: NaturalLabourBookEngagedWorkerField, Value: engaged},
					f.naturalNumber(id, NaturalLabourBookTypeID, NaturalLabourBookEngagedHourField, book.EngagedHour))
			}
			proposal := f.naturalProposal("labour-expiry", at, id, NaturalLabourExpireRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			a.Offer, a.Book = offere, book
			w.actors[id] = a
		}
	}
	// Stage 7: wakes. Every living actor claims at h+1µs — except a newborn
	// whose first claim is already reserved as its WakeBirth wake — and,
	// when births are enabled, evaluates the birth-request cascade at
	// h+40m+5µs; a wage-contract lane (natural/9) evaluates its rule-516
	// offer filing at the same shared instant even with births off, so the
	// negotiation wake exists exactly when a lane can file at it.
	if h < NaturalHorizonHours {
		claim, err := NaturalPhaseTime(h, NaturalPhaseClaim)
		if err != nil {
			return out, err
		}
		var requestAt sim.SimTime
		if f.births || f.naturalLabourLane() {
			requestAt, err = NaturalPhaseTime(h, NaturalPhaseBirthRequest)
			if err != nil {
				return out, err
			}
		}
		for _, id := range f.roster {
			if !live[id] {
				continue
			}
			reserved, hasReserved := f.birthWakes[id]
			if f.births && hasReserved && reserved == int64(h) {
				// The reserved WakeBirth first claim is already scheduled;
				// never double-schedule this instant.
			} else {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeNeedThreshold}})
			}
			if f.births || f.naturalLabourLane() {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: requestAt, Cause: scheduler.WakeAudit}})
			}
		}
		next, err := NaturalHourTime(h + 1)
		if err != nil {
			return out, err
		}
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: naturalClockActor, Wake: scheduler.Wake{Actor: naturalClockActor, At: next, Cause: scheduler.WakeAudit}})
	}
	return out, nil
}

func slotArray(s []NaturalSlotState) [NaturalSlotsPerPatch]NaturalSlotState {
	var out [NaturalSlotsPerPatch]NaturalSlotState
	copy(out[:], s)
	return out
}

func slotSlice(a [NaturalSlotsPerPatch]NaturalSlotState) []NaturalSlotState {
	out := make([]NaturalSlotState, NaturalSlotsPerPatch)
	copy(out, a[:])
	return out
}

// naturalPairIntent is the frozen complement of the bag-decision Build
// condition (policy floor 5), verbatim v3/v4.
func naturalPairIntent(a NaturalActorState, hour int) bool {
	return a.Body.Energy < naturalPairIntentTLow && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// naturalPairedMealGate revalidates the fresh snapshot before any paired
// EatStored at h+40m+4µs, verbatim v3/v4.
func naturalPairedMealGate(a NaturalActorState, hour int) bool {
	return a.Body.Energy > 0 && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// pairedMealDecision consumes one deferred meal intent against a fresh
// snapshot (v4 verbatim). A raced granary yields the typed no-meal note with
// zero mutation.
func (f *Natural) pairedMealDecision(actor sim.EntityID, hour int, intent NaturalMealIntent, a NaturalActorState) (kernel.Proposal, NaturalAttempt, error) {
	kind := "fallback-eatstored"
	if intent.Paired {
		kind = "paired-meal"
	}
	paired, err := NaturalPhaseTime(hour, NaturalPhasePairedMeal)
	if err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	attempt := NaturalAttempt{Actor: actor, Time: paired, Kind: kind}
	if !naturalPairedMealGate(a, hour) {
		attempt.NoMeal = capacityNoStoredMealNote
		return kernel.Proposal{}, attempt, nil
	}
	proposal, err := NaturalEatStoredProposalFor(f.mechanism, naturalKey(kind, paired, actor), f.patches, hour, paired, actor, a)
	if err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	if err = f.guard(proposal); err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	attempt.Key = proposal.Key
	return proposal, attempt, nil
}

// naturalDenialEat commits the documented build-denial loop's re-decided
// consume at the paired instant: the policy re-evaluated the bag decision
// with LastDenial = NaturalBuildDenied and reported NaturalEat; the runner's
// phase discipline places the rule-507 consume (the contract's exact patch
// set) at the next frozen instant after the refused completion.
func (f *Natural) naturalDenialEat(view scheduler.SnapshotView, hour int, at sim.SimTime, id sim.EntityID) (kernel.Proposal, NaturalAttempt, error) {
	a, err := naturalActorState(view, f.mechanism, id)
	if err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	obs, err := f.observe(view, id, hour, strategy.NaturalBuildDenied)
	if err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	choice, err := f.naturalEvaluate(id, obs, naturalLabourOfferGuard(a))
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return kernel.Proposal{}, NaturalAttempt{}, ErrNaturalRunner
	}
	if choice.Kind != strategy.NaturalEat {
		return kernel.Proposal{}, NaturalAttempt{}, fmt.Errorf("%w: build-denied re-evaluation chose %d, want NaturalEat", ErrNaturalRunner, choice.Kind)
	}
	next, _, err := NaturalConsumeFor(f.mechanism, f.patches, id, a)
	if err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	proposal := kernel.Proposal{Key: naturalKey("build-denied-eat", at, id), Time: at, Cause: kernel.Cause{Actor: id}, Rule: NaturalConsumeRule, RuleVersion: f.mechanism.RuleVersion(),
		Patches: []component.Patch{
			f.naturalNumber(id, NaturalBagTypeID, NaturalBagUnitsField, next.Bag.Units),
			f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyEnergyField, next.Body.Energy),
			f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyHungerField, next.Body.Hunger),
			f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyCapLostField, next.Body.CapLost),
			f.naturalNumber(id, NaturalBodyTypeID, NaturalBodyConsumedField, next.Body.Consumed),
		}}
	if err = f.guard(proposal); err != nil {
		return kernel.Proposal{}, NaturalAttempt{}, err
	}
	return proposal, NaturalAttempt{Actor: id, Time: at, Kind: "build-denied-eat", Key: proposal.Key}, nil
}

func (f *Natural) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	if ready.Fiber.Actor == naturalClockActor {
		return f.pulse(ready, view)
	}
	var out scheduler.Evaluation
	id := ready.Fiber.Actor
	if !f.rosterContains(id) || len(ready.Causes) != 1 {
		return out, ErrNaturalRunner
	}
	h := int(ready.At / sim.SimTime(NaturalHour))
	phase := func(n int) (sim.SimTime, error) { return NaturalPhaseTime(h, n) }
	claim, err := phase(NaturalPhaseClaim)
	if err != nil {
		return out, err
	}
	gathered, _ := phase(NaturalPhaseGathered)
	bagDecision, _ := phase(NaturalPhaseBagDecision)
	buildStart, _ := phase(NaturalPhaseBuildStart)
	buildCompletion := NaturalBuildCompletion(buildStart)
	labourSettle, _ := phase(NaturalPhaseLabourSettle)
	paired, _ := phase(NaturalPhasePairedMeal)
	birthRequestAt, _ := phase(NaturalPhaseBirthRequest)
	birthReplyAt, _ := phase(NaturalPhaseBirthReply)
	at := ready.At
	record := func(a NaturalAttempt) {
		f.pending.mu.Lock()
		f.pending.attempts = append(f.pending.attempts, a)
		f.pending.mu.Unlock()
	}
	switch {
	case at == claim && (ready.Causes[0] == scheduler.WakeNeedThreshold || ready.Causes[0] == scheduler.WakeBirth):
		// Claim: policy Gather|EatStored|Wait. A newborn's first claim
		// arrives as the reserved WakeBirth cause and is handled exactly as
		// a claim NeedThreshold wake, distinguished only in evidence.
		birthWake := ready.Causes[0] == scheduler.WakeBirth
		reserved, hasReserved := f.birthWakes[id]
		if birthWake != (hasReserved && reserved == int64(h)) {
			return out, ErrNaturalRunner // wake-topology violation
		}
		if birthWake {
			f.pending.mu.Lock()
			f.pending.birthWakesUsed = append(f.pending.birthWakesUsed, id)
			f.pending.mu.Unlock()
		}
		if f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		w, err := f.worldState(view)
		if err != nil {
			return out, err
		}
		a, ok := w.actors[id]
		if !ok {
			return out, ErrNaturalRunner
		}
		if a.Bag.Units != 0 {
			return out, ErrNaturalRunner
		}
		var choice strategy.NaturalChoice
		if stored, ok := f.claimDecisions[id]; ok {
			// The seam decision recorded in the allocation phase decides; the
			// evaluation phase never calls the model again and never re-derives
			// the decision (spec section 4.4).
			choice = stored
		} else {
			obs, err := f.observe(view, id, h, strategy.NaturalNoDenial)
			if err != nil {
				return out, err
			}
			choice, err = f.naturalEvaluate(id, obs, naturalLabourOfferGuard(a))
			if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
				return out, ErrNaturalRunner
			}
		}
		switch choice.Kind {
		case strategy.NaturalGather:
			admission, ok := f.admissions[id]
			if !ok || admission.Actor != id {
				return out, ErrNaturalRunner
			}
			attempt := NaturalAttempt{Actor: id, Time: at, Kind: "gather", Target: admission.Slot, Rejection: admission.Rejection, BirthWake: birthWake}
			if admission.Rejection == FoodFlowGatherAdmitted {
				if admission.Patch != a.Bag.Source {
					return out, ErrNaturalRunner
				}
				index := int(admission.Slot - NaturalSlotIDBase)
				if index < 0 || index >= f.patches*NaturalSlotsPerPatch {
					return out, ErrNaturalRunner
				}
				slotBefore := w.slots[index/NaturalSlotsPerPatch][index%NaturalSlotsPerPatch]
				proposal, err := NaturalGatherProposalFor(f.mechanism, naturalKey("gather", at, id), f.patches, h, id, admission.Slot, slotBefore, a)
				if err != nil {
					return out, err
				}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				attempt.Key = proposal.Key
				// The gather activity is guarded by its own commit: no
				// phantom window without the atomic slot→bag transfer.
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: NaturalGatherDuration, IfKey: proposal.Key})
				f.pending.mu.Lock()
				f.pending.started[id] = strategy.NaturalGather
				f.pending.mu.Unlock()
			} else {
				f.pending.mu.Lock()
				f.pending.denied = append(f.pending.denied, id)
				f.pending.mu.Unlock()
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: gathered, Cause: scheduler.WakeAudit}})
			}
			record(attempt)
		case strategy.NaturalEatStored:
			proposal, err := NaturalEatStoredProposalFor(f.mechanism, naturalKey("eat-stored", at, id), f.patches, h, at, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(NaturalAttempt{Actor: id, Time: at, Kind: "eat-stored", Key: proposal.Key, BirthWake: birthWake})
		case strategy.NaturalProvision:
			// Rule 515 "provision" (mechanism v6, natural/7): the claim-side
			// conversion of exactly one granary stock unit into the bag at the
			// frozen claim instant. The converted unit takes the normal
			// bag-decision wake (mirror of the gather-completion path without
			// an activity window; no new phase instant), so a build can follow
			// in the same hour. A world-side refusal (typed contract
			// precondition failure) is zero-mutation typed evidence: nothing
			// stages, no bag-decision wake is scheduled, and no fallback
			// choice is silently substituted — the refusal IS the hour's
			// outcome for this actor. Under the v5 zero value the affordance
			// cannot exist (the v5/v6 cascades never emit it and the proposal
			// builder refuses the zero value), so this case is inert there.
			//
			// The refusal branch is unreachable in every live run, by
			// construction rather than by luck: the policy floor (granary>=1
			// and E>=4) is evaluated on the same snapshot the contract call
			// reads; bag != 0 is runner-guarded before the cascade; claim
			// wakes go only to live actors; the mechanism is v6 whenever
			// natural/7 or natural/7b is bound (constructor legality); the
			// shared draw guard cannot bind at a claim instant (one claim
			// wake per actor-hour and every write of that field happens at
			// instants at or after it); and the 168 envelope cannot bind
			// because at most one provision per actor-hour means provisioned
			// <= h <= 167 at the hour-h claim. It is kept as typed fail-closed
			// evidence (the project's precedent for unreachable diagnostics)
			// so that any future affordance change cannot silently stage a
			// precondition-violating transfer.
			proposal, err := NaturalProvisionProposal(f.mechanism, naturalKey("provision", at, id), f.patches, h, id, a)
			if err != nil {
				if !errors.Is(err, ErrNaturalContract) {
					return out, err
				}
				record(NaturalAttempt{Actor: id, Time: at, Kind: "provision", ProvisionRefused: naturalProvisionRefusalNote, BirthWake: birthWake})
				break
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(NaturalAttempt{Actor: id, Time: at, Kind: "provision", Key: proposal.Key, BirthWake: birthWake})
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: bagDecision, Cause: scheduler.WakeAudit}})
		case strategy.NaturalWait:
			record(NaturalAttempt{Actor: id, Time: at, Kind: "wait", BirthWake: birthWake})
		default:
			return out, ErrNaturalRunner
		}
	case ready.Causes[0] == scheduler.WakeCompletion && at == gathered:
		// Admitted gather completion: the atomic gather committed at claim;
		// the actor now receives its frozen bag-decision wake.
		if f.current[id] != strategy.NaturalGather {
			return out, ErrNaturalRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, id)
		f.pending.mu.Unlock()
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: bagDecision, Cause: scheduler.WakeAudit}})
	case ready.Causes[0] == scheduler.WakeAudit && at == gathered:
		// Denied-gather completion: typed denial evidence was recorded at
		// the claim; the policy fallback fires: Rest, or a deferred stored
		// meal drawn fresh at the paired instant.
		if f.denied[id] != int64(h) || f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		deniedState, err := naturalActorState(view, f.mechanism, id)
		if err != nil {
			return out, err
		}
		obs, err := f.observe(view, id, h, strategy.NaturalGatherDenied)
		if err != nil {
			return out, err
		}
		choice, err := f.naturalEvaluate(id, obs, naturalLabourOfferGuard(deniedState))
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrNaturalRunner
		}
		switch choice.Kind {
		case strategy.NaturalRest:
			if choice.RestDuration != strategy.NaturalRestDuration {
				return out, ErrNaturalRunner
			}
			record(NaturalAttempt{Actor: id, Time: at, Kind: "rest"})
		case strategy.NaturalEatStored:
			f.pending.mu.Lock()
			f.pending.meals[id] = NaturalMealIntent{Hour: h, Paired: false}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		default:
			return out, ErrNaturalRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == bagDecision:
		// Bag decision: Build | Eat per the frozen policy on the held unit.
		if f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		w, err := f.worldState(view)
		if err != nil {
			return out, err
		}
		a, ok := w.actors[id]
		if !ok {
			return out, ErrNaturalRunner
		}
		if a.Bag.Units != 1 {
			return out, ErrNaturalRunner
		}
		obs, err := f.observe(view, id, h, strategy.NaturalNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.naturalEvaluate(id, obs, naturalLabourOfferGuard(a))
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrNaturalRunner
		}
		switch choice.Kind {
		case strategy.NaturalBuild:
			if f.cultivationDisabled {
				// Declared cultivation ablation (frozen matrix cell D): the
				// frozen policy is unchanged — it chose Build and the runner
				// treats the choice as the documented unavailable case. Zero
				// mutation, the bag kept, the hour's denial recorded, and the
				// paired-instant audit wake so the documented denial loop
				// (LastDenial = NaturalBuildDenied → policy emits Eat) decides
				// the meal. No build-start wake exists, so no completion, no
				// slot claim, and no rule-505 event can arise; the Step-level
				// ablation guard fails loud if one ever does.
				f.pending.mu.Lock()
				f.pending.deniedBuild = append(f.pending.deniedBuild, id)
				f.pending.mu.Unlock()
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
				break
			}
			f.pending.mu.Lock()
			if naturalPairIntent(a, h) {
				f.pending.meals[id] = NaturalMealIntent{Hour: h, Paired: true}
			}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: buildStart, Cause: scheduler.WakeAudit}})
		case strategy.NaturalDeliver:
			// Rule 518 delivery (ticket 20, natural/9 only): the worker's own
			// accepted contract is inside its single delivery hour and the bag
			// holds exactly the gathered unit, so the delivery IS the hour's
			// unit conversion — the settlement attempt fires at the frozen
			// build-completion instant (h+40m+3µs, rule 518's own phase; no
			// new phase instant). The coordinator folds the attempt against
			// the hour's completing builds and settlements before the batch;
			// the folded outcome commits the enumerated two-party proposal or
			// records the typed void with zero mutation (the worker keeps the
			// bag and re-enters the documented denial path at the paired
			// instant).
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: labourSettle, Cause: scheduler.WakeAudit}})
		case strategy.NaturalEat:
			proposal, err := NaturalConsumeProposalFor(f.mechanism, naturalKey("eat", at, id), f.patches, h, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(NaturalAttempt{Actor: id, Time: at, Kind: "eat", Key: proposal.Key})
		default:
			return out, ErrNaturalRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == buildStart:
		// Build start: the 20-minute window opens exactly at the frozen
		// instant so its completion is NaturalBuildCompletion.
		w, err := f.worldState(view)
		if err != nil {
			return out, err
		}
		a, ok := w.actors[id]
		if !ok || a.Bag.Units != 1 || f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: NaturalBuildDuration})
		f.pending.mu.Lock()
		f.pending.started[id] = strategy.NaturalBuild
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeCompletion && at == buildCompletion:
		// Build completion: the atomic bag→wip/k conversion and slot claim
		// from the coordinator's folded arbitration (ascending actor ID, so
		// claims partition the free slots and refusals are the contract's
		// own fresh-snapshot verdicts). The refused completion enters the
		// documented denial loop: zero mutation, the bag kept, and a
		// same-hour re-eat at the paired instant.
		if f.current[id] != strategy.NaturalBuild {
			return out, ErrNaturalRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, id)
		f.pending.mu.Unlock()
		outcome, ok := f.builds[id]
		if !ok {
			return out, ErrNaturalRunner
		}
		attempt := NaturalAttempt{Actor: id, Time: at, Kind: "build"}
		if outcome.voidNote != "" {
			// The completion-instant fold voided this completion before it
			// was proposed (the recorded settlement precedence: the settlement
			// into this builder owns its worksite fields). Zero mutation, the
			// bag retained, and the documented denial loop decides the meal at
			// the paired instant.
			attempt.Refused = outcome.voidNote
			f.pending.mu.Lock()
			f.pending.deniedBuild = append(f.pending.deniedBuild, id)
			if intent, ok := f.meals[id]; ok && intent.Hour == h && intent.Paired {
				f.pending.mealsCleared = append(f.pending.mealsCleared, id)
			}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
			record(attempt)
			return out, nil
		}
		if outcome.refused != NaturalBuildRefusalNone {
			attempt.Refused = naturalBuildRefusalNote(outcome.refused)
			f.pending.mu.Lock()
			f.pending.deniedBuild = append(f.pending.deniedBuild, id)
			if intent, ok := f.meals[id]; ok && intent.Hour == h && intent.Paired {
				f.pending.mealsCleared = append(f.pending.mealsCleared, id)
			}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
			record(attempt)
			return out, nil
		}
		if err := f.guard(outcome.proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, outcome.proposal)
		attempt.Key = outcome.proposal.Key
		record(attempt)
		if intent, ok := f.meals[id]; ok && intent.Hour == h && intent.Paired {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == paired:
		// Paired instant: the build-denial re-eat first, then the v4 paired
		// stored meal with fresh-snapshot granary revalidation.
		if f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		if f.deniedBuild[id] == int64(h) {
			if intent, ok := f.meals[id]; ok && intent.Hour == h {
				return out, ErrNaturalRunner // the refused build dropped its paired meal
			}
			proposal, attempt, err := f.naturalDenialEat(view, h, at, id)
			if err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(attempt)
			return out, nil
		}
		intent, ok := f.meals[id]
		if !ok || intent.Hour != h || f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		f.pending.mu.Lock()
		f.pending.mealsCleared = append(f.pending.mealsCleared, id)
		f.pending.mu.Unlock()
		w, err := f.worldState(view)
		if err != nil {
			return out, err
		}
		a, ok := w.actors[id]
		if !ok {
			return out, ErrNaturalRunner
		}
		proposal, attempt, err := f.pairedMealDecision(id, h, intent, a)
		if err != nil {
			return out, err
		}
		if proposal.Key != "" {
			out.Proposals = append(out.Proposals, proposal)
		}
		record(attempt)
	case ready.Causes[0] == scheduler.WakeAudit && at == birthRequestAt:
		// The shared negotiation-filing instant (h+40m+5µs): rule 516 labour-
		// offer and rule 510 birth-request. Budget composition (frozen design,
		// supervisor decision recorded 2026-09-29): at most one negotiation
		// filing per actor-hour, and the wage lane has precedence — the
		// natural/9 offer cascade evaluates first and a filing consumes the
		// hour's negotiation slot (the birth lane simply retries next hour);
		// only a waiting offer cascade falls through to the birth-request
		// cascade. Waits are not journaled.
		if (!f.births && !f.naturalLabourLane()) || f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		if f.naturalLabourLane() {
			proposal, filed, err := f.labourOfferDecision(view, h, at, id)
			if err != nil {
				return out, err
			}
			if filed {
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				record(NaturalAttempt{Actor: id, Time: at, Kind: "labour-offer", Key: proposal.Key})
				// The decision wake goes to a LIVING addressee only. Nothing
				// can die between +5µs and +6µs, so this snapshot verdict is
				// final for the hour. The scheduler's wake queue dedupes an
				// identical (actor, at, cause) entry, so an employer also
				// addressed by a birth request this hour holds one wake whose
				// single audit cause carries both pending decisions under the
				// one-decision budget.
				wake, err := f.labourDecisionWake(view, h, proposal)
				if err != nil {
					return out, err
				}
				if wake != (scheduler.Wake{}) {
					out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: wake.Actor, Wake: wake})
				}
				return out, nil
			}
		}
		if !f.births {
			return out, nil
		}
		// Birth request (rule 510): the proposer files an addressed request
		// with self-reported claims. Waits are not journaled.
		proposal, filed, err := f.birthRequestDecision(view, h, at, id)
		if err != nil {
			return out, err
		}
		if !filed {
			return out, nil
		}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		record(NaturalAttempt{Actor: id, Time: at, Kind: "birth-request", Key: proposal.Key})
		f.pending.mu.Lock()
		f.pending.birthFiles++
		f.pending.mu.Unlock()
		// The reply wake goes to a LIVING addressee only. Nothing can die
		// between +5µs and +6µs, so this snapshot verdict is final for the
		// hour; a dead addressee lets the request expire at the next pulse.
		wake, err := f.birthReplyWake(view, h, proposal)
		if err != nil {
			return out, err
		}
		if wake != (scheduler.Wake{}) {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: wake.Actor, Wake: wake})
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == birthReplyAt:
		// The shared negotiation-decision instant (h+40m+6µs): rule 517
		// labour-decision and rules 511/512 birth-reply. Budget composition
		// (frozen design, supervisor decision recorded 2026-09-29): at most
		// one negotiation decision per actor-hour, and the wage lane has
		// precedence — the natural/9 employer cascade decides first and a
		// decision consumes the hour's negotiation slot; only a None verdict
		// (no pending addressed offer) falls through to the birth-reply
		// cascade.
		if f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		if f.naturalLabourLane() {
			proposal, attempt, err := f.labourDecisionDecision(view, h, at, id)
			if err != nil {
				return out, err
			}
			if attempt.Actor != 0 {
				if proposal.Key != "" {
					if err = f.guard(proposal); err != nil {
						return out, err
					}
					out.Proposals = append(out.Proposals, proposal)
				}
				record(attempt)
				return out, nil
			}
		}
		if !f.births {
			return out, nil
		}
		// Birth reply (rules 511/512): the consenter decides; the runner
		// revalidates the full surplus gate on fresh truth through the
		// contract. Acceptance commits one atomic allocator-based
		// entity-create; refusal commits the typed two-row rule 512; death
		// between phases would be typed zero-mutation evidence. No pending
		// request: no evidence.
		proposal, attempt, err := f.birthReplyDecision(view, h, at, id)
		if err != nil {
			return out, err
		}
		if attempt.Actor == 0 {
			return out, nil
		}
		if proposal.Key != "" {
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
		}
		record(attempt)
		f.pending.mu.Lock()
		if attempt.Birth.Decision == NaturalReplyAccept {
			if attempt.Birth.Diverged {
				f.pending.divergedReplies++
			}
		} else {
			f.pending.replyReasons = append(f.pending.replyReasons, attempt.Birth.Reason)
			if attempt.Birth.Diverged {
				f.pending.divergedReplies++
			}
		}
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeAudit && at == labourSettle:
		// Rule 518 labour-settlement (ticket 20, natural/9 only): the folded
		// atomic wage settlement or its typed void, decided by the
		// coordinator's completion-instant arbitration before the batch. A
		// committed settlement carries exactly the enumerated two-party patch
		// set (plus the completion's tenure claim); a void is ZERO mutation
		// with the typed note: the worker keeps the bag and re-enters the
		// documented denial path (the paired-instant re-eat) whenever it
		// still holds the unit — a denied-gather void has no unit and the
		// frozen denied-gather fallback already decided the hour.
		if f.current[id] != 0 {
			return out, ErrNaturalRunner
		}
		outcome, ok := f.settles[id]
		if !ok {
			return out, ErrNaturalRunner // wake-topology violation: no folded settlement outcome
		}
		if outcome.void != "" {
			record(NaturalAttempt{Actor: id, Time: at, Kind: "labour-void", Refused: outcome.void})
			a, err := naturalActorState(view, f.mechanism, id)
			if err != nil {
				return out, err
			}
			if a.Bag.Units == 1 {
				f.pending.mu.Lock()
				f.pending.deniedBuild = append(f.pending.deniedBuild, id)
				f.pending.mu.Unlock()
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
			}
			return out, nil
		}
		if err := f.guard(outcome.proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, outcome.proposal)
		record(NaturalAttempt{Actor: id, Time: at, Kind: "labour-settlement", Key: outcome.proposal.Key})
	default:
		return out, ErrNaturalRunner
	}
	return out, nil
}

// birthRequestDecision evaluates the frozen proposer cascade at h+40m+5µs
// over the cap-free roster (same-patch mates in ascending ID order) and
// builds the validated rule-510 proposal when the policy files.
func (f *Natural) birthRequestDecision(view scheduler.SnapshotView, hour int, at sim.SimTime, proposer sim.EntityID) (kernel.Proposal, bool, error) {
	w, err := f.worldState(view)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	a, ok := w.actors[proposer]
	if !ok || !naturalAlive(a) {
		return kernel.Proposal{}, false, ErrNaturalRunner
	}
	mates := make([]sim.EntityID, 0, len(f.roster))
	for _, mateID := range f.roster {
		if mateID == proposer {
			continue
		}
		mate, ok := w.actors[mateID]
		if !ok {
			return kernel.Proposal{}, false, ErrNaturalRunner
		}
		if mate.Bag.Source == a.Bag.Source {
			mates = append(mates, mateID)
		}
	}
	obs := strategy.NaturalRequestObservation{
		Actor: proposer, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		HomePatch:    a.Bag.Source,
		Energy:       naturalIntegerObservation(a.Body.Energy),
		Capital:      naturalIntegerObservation(a.Worksite.Capital),
		GranaryStock: naturalIntegerObservation(a.Granary.Stock),
		OwnRequest:   strategy.NaturalRequestGuard{Status: strategy.NaturalRequestStatus(a.Request.Status), LastRequestHour: a.Request.LastRequestHour},
		PatchMates:   mates,
	}
	choice, err := f.naturalEvaluateBirthRequest(proposer, obs)
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return kernel.Proposal{}, false, ErrNaturalRunner
	}
	if choice.Kind != strategy.NaturalRequestFile {
		return kernel.Proposal{}, false, nil
	}
	proposal, err := NaturalBirthRequestProposalFor(f.mechanism, naturalKey("birth-request", at, proposer), f.patches, hour, proposer, choice.Addressee, choice.ReportedCapital, choice.ReportedGranary, a)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	return proposal, true, nil
}

func naturalIntegerObservation(n int64) sim.Value { return sim.IntegerValue(n) }

// birthReplyWake derives the consenter's reply wake from the committed
// request proposal: the addressee, if alive in this snapshot.
func (f *Natural) birthReplyWake(view scheduler.SnapshotView, hour int, request kernel.Proposal) (scheduler.Wake, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBirthReply)
	if err != nil {
		return scheduler.Wake{}, err
	}
	var addressee sim.EntityID
	for _, patch := range request.Patches {
		if patch.Field == NaturalBirthRequestAddresseeField {
			addressee, err = capacityRefValue(patch.Value)
			if err != nil {
				return scheduler.Wake{}, err
			}
		}
	}
	if addressee == 0 {
		return scheduler.Wake{}, ErrNaturalRunner
	}
	state, err := naturalActorState(view, f.mechanism, addressee)
	if err != nil {
		return scheduler.Wake{}, err
	}
	if !naturalAlive(state) {
		return scheduler.Wake{}, nil
	}
	return scheduler.Wake{Actor: addressee, At: at, Cause: scheduler.WakeAudit}, nil
}

// birthReplyDecision is the trusted reply-phase decision for one consenter
// at h+40m+6µs: fresh full state, the derived pending views (claims plus the
// PUBLIC kin walk), the frozen policy cascade. A policy refusal is final and
// typed — a donor's refusal is always permissible and needs no truth
// verification — so it resolves here from the policy's own reason and never
// consults the contract's birth execution: the contract's frozen consent
// floor is strictly WEAKER than the v6/v7 policy floor (energy >= 3 versus
// TFile = 4), so a pair in that divergent band can pass the contract's
// entire truth gate while the policy refused it, and evaluating the birth
// anyway would execute — or crash building — a birth no decision committed.
// Only a policy-accepted reply revalidates through the contract's consent +
// surplus-gate on fresh truth; the newborn ID comes from the runner-owned
// allocator as births-before + genesis: two same-instant replies compute the
// SAME candidate, so the kernel's whole-proposal collision rule keeps
// exactly one winner with typed lost-birth evidence for the loser (the
// allocator advances only at committed registration).
func (f *Natural) birthReplyDecision(view scheduler.SnapshotView, hour int, at sim.SimTime, consenter sim.EntityID) (kernel.Proposal, NaturalAttempt, error) {
	var empty kernel.Proposal
	w, err := f.worldState(view)
	if err != nil {
		return empty, NaturalAttempt{}, err
	}
	self, ok := w.actors[consenter]
	if !ok {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	lookup := NaturalParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		a, ok := w.actors[id]
		if !ok {
			return 0, 0, false
		}
		return a.Genome.ParentA, a.Genome.ParentB, true
	})
	var pending []strategy.NaturalPendingView
	for _, id := range f.roster { // ascending ID order
		if id == consenter {
			continue
		}
		r := w.actors[id].Request
		if r.Status != NaturalRequestPending || r.Hour != int64(hour) || r.Addressee != consenter {
			continue
		}
		distance, related := NaturalKinDistance(lookup, id, consenter)
		pending = append(pending, strategy.NaturalPendingView{
			Requester: id, Addressee: consenter, Hour: hour,
			ReportedCapital: r.ReportedCapital, ReportedGranary: r.ReportedGranary,
			KinDistance: distance, KinRelated: related,
		})
	}
	obs := strategy.NaturalReplyObservation{
		Actor: consenter, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		Energy:           naturalIntegerObservation(self.Body.Energy),
		Capital:          naturalIntegerObservation(self.Worksite.Capital),
		GranaryStock:     naturalIntegerObservation(self.Granary.Stock),
		OwnLastReplyHour: self.Request.LastReplyHour,
		Pending:          pending,
	}
	choice, err := f.naturalEvaluateBirthReply(consenter, obs)
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	if choice.Kind == strategy.NaturalReplyNone {
		return empty, NaturalAttempt{}, nil
	}
	requester := choice.Requester
	proposer, ok := w.actors[requester]
	if !ok || requester == consenter {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	distance, related := NaturalKinDistance(lookup, requester, consenter)
	kinAllowed := !NaturalKinProhibited(distance, related)
	var result NaturalReplyResult
	switch choice.Kind {
	case strategy.NaturalReplyRefuse:
		// The policy's consent floor may be strictly STRONGER than the
		// contract's frozen one (v6/v7 file/accept at own energy >= 4 versus
		// the contract's NaturalConsentMinEnergy = 3), so the contract can
		// derive an acceptance from its truth gate exactly where the policy
		// refused. A donor's refusal is always permissible and needs no
		// truth verification, so the policy's decision wins and the refusal
		// is recorded with the policy's own typed reason (mapped into the
		// contract's vocabulary). The structural guards stay exact: the
		// pending views were built from exactly the request rows re-checked
		// here, a duplicate reply returned None above, and a death between
		// the request and reply phases — impossible within one hour, since
		// deaths fire only at the pulse — is kept as the typed zero-mutation
		// refusal with no recorded attempt (a dead consenter never acts).
		if proposer.Request.Status != NaturalRequestPending || proposer.Request.Hour != int64(hour) || proposer.Request.Addressee != consenter {
			return empty, NaturalAttempt{}, &NaturalRefusalError{Reason: NaturalRefusalNotPending}
		}
		switch {
		case !naturalAlive(self):
			return empty, NaturalAttempt{}, nil
		case !naturalAlive(proposer):
			return empty, NaturalAttempt{}, nil
		default:
			mapped, ok := naturalPolicyRefusalReason(choice.Reason)
			if !ok {
				return empty, NaturalAttempt{}, ErrNaturalRunner
			}
			result = NaturalReplyResult{Decision: NaturalReplyRefuse, Reason: mapped, Proposer: proposer, Consenter: self}
			result.Proposer.Request.Status = NaturalRequestRefused
			result.Consenter.Request.LastReplyHour = int64(hour)
		}
	case strategy.NaturalReplyAccept:
		// The consent formula accepted on claims and the PUBLIC kin
		// distance; the runner's own kin walk must agree before any draw or
		// ID is spent.
		if !kinAllowed {
			return empty, NaturalAttempt{}, ErrNaturalRunner
		}
		genesisLast, err := NaturalHighestFixedEntityID(f.patches)
		if err != nil {
			return empty, NaturalAttempt{}, err
		}
		births := int64(f.allocator.Last() - genesisLast)
		candidate := f.allocator.Last() + 1
		if threshold, ok := f.thresholds[NaturalCeilingAllocatorIDs]; ok && uint64(candidate) > threshold {
			hit := f.ceilingHit(NaturalCeilingAllocatorIDs, threshold, uint64(candidate), hour, view.Version)
			f.pending.mu.Lock()
			f.pending.hits = append(f.pending.hits, hit)
			f.pending.mu.Unlock()
			return empty, NaturalAttempt{}, &NaturalCeilingHitError{Hit: hit}
		}
		scratch := sim.NewEntityAllocator(f.allocator.Last())
		var newbornID sim.EntityID
		newbornID, err = NaturalNextNewbornID(&scratch)
		if err != nil {
			hit := f.ceilingHit(NaturalCeilingAllocatorIDs, ^uint64(0), uint64(f.allocator.Last()), hour, view.Version)
			f.pending.mu.Lock()
			f.pending.hits = append(f.pending.hits, hit)
			f.pending.mu.Unlock()
			return empty, NaturalAttempt{}, &NaturalCeilingHitError{Hit: hit}
		}
		// The heredity draws are a pure function of the birth ordinal
		// (7 x (ordinal-1) on the frozen birth stream) and are drawn only for
		// a policy-accepted reply: the contract's birth execution needs them,
		// and a refused reply never reaches it.
		position, err := NaturalBirthDrawPosition(births)
		if err != nil {
			return empty, NaturalAttempt{}, err
		}
		stream := sim.NewRandomStream(sim.RandomState{Seed: f.seed, Stream: NaturalBirthStream, Position: position})
		draws := make([]uint64, NaturalDrawsPerBirth)
		for i := range draws {
			draws[i] = stream.Uint64()
		}
		accepted, err := NaturalBirthReplyFor(f.mechanism, f.patches, hour, kinAllowed, draws, newbornID, requester, consenter, proposer, self)
		if err != nil {
			return empty, NaturalAttempt{}, err
		}
		// Decision-level agreement (v4 verbatim, no birth-cap reason): the
		// policy may accept while the runner-side fresh-truth gate refuses
		// (CrossPatch, GateThresholds), but a consent-phase disagreement is a
		// linkage break.
		switch accepted.Decision {
		case NaturalReplyAccept:
		case NaturalReplyRefuse:
			switch accepted.Reason {
			case NaturalRefusalCrossPatch, NaturalRefusalGateThresholds:
			default:
				return empty, NaturalAttempt{}, ErrNaturalRunner
			}
		case NaturalReplyRefuseSilent:
			switch accepted.Reason {
			case NaturalRefusalProposerDead, NaturalRefusalAddresseeDead:
			default:
				return empty, NaturalAttempt{}, ErrNaturalRunner
			}
		default:
			return empty, NaturalAttempt{}, ErrNaturalRunner
		}
		result = accepted
	default:
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	var proposal kernel.Proposal
	if result.Decision != NaturalReplyRefuseSilent {
		// A zero-mutation refusal (death between phases) has no proposal at
		// all; accept and refuse commit their typed proposals.
		proposal, err = NaturalBirthReplyProposalFor(f.mechanism, naturalKey("birth-reply", at, consenter), f.patches, hour, requester, consenter, result)
		if err != nil {
			return empty, NaturalAttempt{}, err
		}
	}
	request := proposer.Request
	attempt := NaturalAttempt{
		Actor: consenter, Time: at, Kind: "birth-reply",
		Birth: NaturalBirthEvidence{
			Decision:        result.Decision,
			Reason:          result.Reason,
			ReportedCapital: request.ReportedCapital, ReportedGranary: request.ReportedGranary,
			TrueCapital: proposer.Worksite.Capital, TrueGranary: proposer.Granary.Stock,
			Diverged: request.ReportedCapital != proposer.Worksite.Capital || request.ReportedGranary != proposer.Granary.Stock,
			Newborn:  result.NewbornID,
		},
	}
	if proposal.Key != "" {
		attempt.Key = proposal.Key
	}
	return proposal, attempt, nil
}

// naturalLabourRefusalNote is the typed journal note of a refused wage
// settlement (zero mutation, the bag kept, the documented denial path). The
// note names the contract's own typed refusal taxonomy; the one runner-level
// structural note (settlement-precedence) is the completion-instant fold's
// supervisor-ruled settlement precedence, never a contract-taxonomy reason.
func naturalLabourRefusalNote(reason NaturalLabourRefusalReason) string {
	switch reason {
	case NaturalLabourRefusalContractNotPending:
		return "contract-not-pending"
	case NaturalLabourRefusalWorkerDeadOrCrossPatch:
		return "worker-dead-or-cross-patch"
	case NaturalLabourRefusalEmployerDeadOrCrossPatch:
		return "employer-dead-or-cross-patch"
	case NaturalLabourRefusalWorkerBagEmpty:
		return "worker-bag-empty"
	case NaturalLabourRefusalEmployerStockShort:
		return "employer-stock-short"
	case NaturalLabourRefusalEmployerAtKMax:
		return "employer-at-kmax"
	case NaturalLabourRefusalNoFreeSlot:
		return "no-free-slot"
	}
	return ""
}

// naturalSettlementPrecedenceNote is the runner-level structural note of a
// completion-instant flow voided by the fold's declared settlement
// precedence (supervisor decision recorded 2026-09-29): a settlement whose
// enumerated field set intersects an earlier-folded same-instant winner, or
// an employer's own rule-505 completion whose field set intersects the
// settlement into it. The voided flow stages nothing (zero mutation); a
// voided build keeps its bag and re-enters the documented denial loop.
const naturalSettlementPrecedenceNote = "settlement-precedence"

// naturalLabourEngagementBusyNote is the typed zero-mutation note on a
// policy accept the world refuses (the E12 class, frozen design): the
// employer's one engagement slot is still occupied by the previous hour's
// live engagement (engaged-hour h-1, its delivery hour), so the contract's
// structural book-idle precondition fails. The decision stages nothing, the
// book's decision audit stays unwritten, and the pending offer stays
// pending — rule 519 expires it at the next pulse.
const naturalLabourEngagementBusyNote = "engagement-busy"

// naturalSettleOutcome carries one worker's folded rule-518 outcome for the
// settle instant (mirror of naturalBuildOutcome): either the validated
// proposal or the typed void note.
type naturalSettleOutcome struct {
	proposal kernel.Proposal
	void     string
}

// naturalFlowFields lists the (entity, component, field) keys one proposal
// writes; the completion-instant fold uses the sets to declare its
// precedence: a flow whose set intersects an already-folded winner is voided
// BEFORE it is proposed (the kernel's whole-proposal collision rule must
// never silently discard a settlement).
// naturalFlowKey names one (entity, component, field) write of a proposal;
// the completion-instant fold tracks the union of its winners' keys so the
// declared precedence can void a flow before it is proposed.
type naturalFlowKey struct {
	entity    sim.EntityID
	component sim.ComponentTypeID
	field     sim.FieldID
}

// naturalFlowFields lists the field keys one proposal writes.
func naturalFlowFields(p kernel.Proposal) map[naturalFlowKey]bool {
	out := make(map[naturalFlowKey]bool, len(p.Patches))
	for _, patch := range p.Patches {
		out[naturalFlowKey{patch.Entity, patch.Component, patch.Field}] = true
	}
	return out
}

// arbitrateCompletions folds the hour's completing flows at the frozen
// h+40m+3µs instant in the declared settlement-precedence order (supervisor
// decision recorded 2026-09-29, pre-run): every folded call resolves on the
// folded fresh state — settlements first in ascending worker ID, then the
// surviving employers' own rule-505 completions in ascending builder ID —
// and any flow whose enumerated field set intersects an already-folded
// winner is voided BEFORE it is proposed, so the kernel's whole-proposal
// collision rule can never silently discard a settlement. A voided
// settlement is typed labour-void evidence with zero mutation (the worker
// keeps the bag and re-enters the documented denial path); a voided build
// keeps the builder's bag and enters the same denial loop, its worksite
// resting at the settlement's state. Slot-claim exhaustion stays the
// contract's own typed refusal: each folded call sees the earlier claims in
// its slot array, exactly the frozen rule-505 arbitration discipline.
func (f *Natural) arbitrateCompletions(view scheduler.SnapshotView, at sim.SimTime, completions, settleActors []sim.EntityID) error {
	f.settles = make(map[sim.EntityID]naturalSettleOutcome, len(settleActors))
	f.builds = make(map[sim.EntityID]naturalBuildOutcome, len(completions))
	h := int(at / sim.SimTime(NaturalHour))
	w, err := f.worldState(view)
	if err != nil {
		return err
	}
	written := make(map[naturalFlowKey]bool)
	intersects := func(fields map[naturalFlowKey]bool) bool {
		for key := range fields {
			if written[key] {
				return true
			}
		}
		return false
	}
	mark := func(fields map[naturalFlowKey]bool) {
		for key := range fields {
			written[key] = true
		}
	}
	// Stage 1: settlements in ascending worker ID on the folded state. A
	// contract refusal is the contract's own typed taxonomy; a field-set
	// intersection with an earlier winner (the chained employer-worker
	// case) is the runner-level precedence note. Either way the folded
	// state keeps the pre-settlement rows and slots.
	sortedSettles := append([]sim.EntityID(nil), settleActors...)
	sort.Slice(sortedSettles, func(i, j int) bool { return sortedSettles[i] < sortedSettles[j] })
	for _, id := range sortedSettles {
		a, ok := w.actors[id]
		if !ok || a.Offer.Status != NaturalRequestAccepted {
			return ErrNaturalRunner
		}
		employerID := a.Offer.Addressee
		employer, ok := w.actors[employerID]
		if !ok {
			return ErrNaturalRunner
		}
		index := int(a.Bag.Source - NaturalPatchIDBase)
		if index < 0 || index >= f.patches {
			return ErrNaturalRunner
		}
		before := slotArray(w.slots[index])
		outcome := naturalSettleOutcome{}
		nextWorker, nextEmployer, foldedSlots, _, err := NaturalLabourSettle(f.mechanism, f.patches, h, id, employerID, a, employer, before)
		var refusal *NaturalLabourRefusalError
		switch {
		case errors.As(err, &refusal):
			outcome.void = naturalLabourRefusalNote(refusal.Reason)
		case err != nil:
			return err
		default:
			// The proposal builder re-runs the same pure handshake; it must
			// see the settlement's PRE-claim slots, or the folded claim
			// would refuse against itself.
			proposal, err := NaturalLabourSettleProposalFor(f.mechanism, naturalKey("labour-settlement", at, id), f.patches, h, id, employerID, a, employer, before)
			if err != nil {
				return err
			}
			fields := naturalFlowFields(proposal)
			if intersects(fields) {
				outcome.void = naturalSettlementPrecedenceNote
			} else {
				outcome.proposal = proposal
				mark(fields)
				w.slots[index] = slotSlice(foldedSlots)
				w.actors[id] = nextWorker
				w.actors[employerID] = nextEmployer
			}
		}
		f.settles[id] = outcome
	}
	// Stage 2: the surviving builders' own rule-505 completions, ascending
	// builder ID. A builder whose worksite fields an already-folded
	// settlement owns (the settlement into it wrote invested/wip) is voided
	// before any contract call: zero mutation, the bag retained for the
	// documented denial re-eat, the worksite at the settlement's state.
	sorted := append([]sim.EntityID(nil), completions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	for _, id := range sorted {
		if f.current[id] != strategy.NaturalBuild {
			return ErrNaturalRunner
		}
		if written[naturalFlowKey{id, NaturalWorksiteTypeID, NaturalWorksiteInvestedUnitsField}] ||
			written[naturalFlowKey{id, NaturalWorksiteTypeID, NaturalWorksiteWipField}] {
			f.builds[id] = naturalBuildOutcome{voidNote: naturalSettlementPrecedenceNote}
			continue
		}
		a, ok := w.actors[id]
		if !ok || a.Bag.Units != 1 {
			return ErrNaturalRunner
		}
		index := int(a.Bag.Source - NaturalPatchIDBase)
		if index < 0 || index >= f.patches {
			return ErrNaturalRunner
		}
		before := slotArray(w.slots[index])
		foldedActor, folded, _, err := NaturalBuildFor(f.mechanism, f.patches, h, id, a, a.Bag.Source, before)
		outcome := naturalBuildOutcome{}
		var refusal *NaturalBuildRefusalError
		switch {
		case errors.As(err, &refusal):
			outcome.refused = refusal.Reason
		case err != nil:
			return err
		default:
			// The proposal builder re-runs the same pure claim; it must see
			// the builder's PRE-claim slots, or the builder's own folded
			// claim would refuse against itself.
			proposal, err := NaturalBuildProposalFor(f.mechanism, naturalKey("build", at, id), f.patches, h, id, a, before)
			if err != nil {
				return err
			}
			outcome.proposal = proposal
			mark(naturalFlowFields(proposal))
			w.slots[index] = slotSlice(folded)
			w.actors[id] = foldedActor
		}
		f.builds[id] = outcome
	}
	return nil
}

// labourOfferDecision evaluates the frozen natural/9 worker cascade at
// h+40m+5µs over the cap-free roster (same-patch mates in ascending ID
// order) and builds the validated rule-516 proposal when the policy files.
func (f *Natural) labourOfferDecision(view scheduler.SnapshotView, hour int, at sim.SimTime, worker sim.EntityID) (kernel.Proposal, bool, error) {
	w, err := f.worldState(view)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	a, ok := w.actors[worker]
	if !ok || !naturalAlive(a) {
		return kernel.Proposal{}, false, ErrNaturalRunner
	}
	mates := make([]sim.EntityID, 0, len(f.roster))
	for _, mateID := range f.roster {
		if mateID == worker {
			continue
		}
		mate, ok := w.actors[mateID]
		if !ok {
			return kernel.Proposal{}, false, ErrNaturalRunner
		}
		if mate.Bag.Source == a.Bag.Source {
			mates = append(mates, mateID)
		}
	}
	obs := strategy.NaturalLabourOfferObservation{
		Actor: worker, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		HomePatch:  a.Bag.Source,
		Energy:     naturalIntegerObservation(a.Body.Energy),
		BagUnits:   naturalIntegerObservation(a.Bag.Units),
		OwnOffer:   naturalLabourOfferGuard(a),
		PatchMates: mates,
	}
	choice, err := f.naturalEvaluateLabourOffer(worker, obs)
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return kernel.Proposal{}, false, ErrNaturalRunner
	}
	if choice.Kind != strategy.NaturalLabourOfferFile {
		return kernel.Proposal{}, false, nil
	}
	// addresseeHome is the trusted runner's fresh view of the addressee's
	// home patch: the mates were derived same-patch, so the addressee's home
	// is the worker's own bag source (the L1-recorded caller-owned seam).
	proposal, err := NaturalLabourOfferProposalFor(f.mechanism, naturalKey("labour-offer", at, worker), f.patches, hour, worker, choice.Addressee, a.Bag.Source, a)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	return proposal, true, nil
}

// labourDecisionWake derives the employer's decision wake from the committed
// offer proposal: the addressee, if alive in this snapshot.
func (f *Natural) labourDecisionWake(view scheduler.SnapshotView, hour int, offer kernel.Proposal) (scheduler.Wake, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseLabourDecision)
	if err != nil {
		return scheduler.Wake{}, err
	}
	var addressee sim.EntityID
	for _, patch := range offer.Patches {
		if patch.Field == NaturalLabourOfferAddresseeField {
			addressee, err = capacityRefValue(patch.Value)
			if err != nil {
				return scheduler.Wake{}, err
			}
		}
	}
	if addressee == 0 {
		return scheduler.Wake{}, ErrNaturalRunner
	}
	state, err := naturalActorState(view, f.mechanism, addressee)
	if err != nil {
		return scheduler.Wake{}, err
	}
	if !naturalAlive(state) {
		return scheduler.Wake{}, nil
	}
	return scheduler.Wake{Actor: addressee, At: at, Cause: scheduler.WakeAudit}, nil
}

// labourDecisionDecision is the trusted decision-phase duty for one employer
// at h+40m+6µs (rule 517): fresh full state, the pending addressed offers'
// public identities (ascending worker order), the frozen policy cascade, and
// the contract's own revalidation on the same truth. An accept or refusal
// commits the enumerated rule-517 proposal; no pending offer resolves to no
// evidence (the birth-reply cascade holds the hour's negotiation slot then,
// per the recorded precedence). A contract refusal here is a runner
// invariant break (the policy floor implies the world's revalidation) and
// fails loud with zero mutation.
func (f *Natural) labourDecisionDecision(view scheduler.SnapshotView, hour int, at sim.SimTime, employer sim.EntityID) (kernel.Proposal, NaturalAttempt, error) {
	var empty kernel.Proposal
	w, err := f.worldState(view)
	if err != nil {
		return empty, NaturalAttempt{}, err
	}
	self, ok := w.actors[employer]
	if !ok {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	if !naturalAlive(self) {
		return empty, NaturalAttempt{}, nil // a dead employer never acts
	}
	var pending []strategy.NaturalLabourPendingView
	for _, id := range f.roster { // ascending ID order
		offer := w.actors[id].Offer
		if offer.Status != NaturalRequestPending || offer.Hour != int64(hour) || offer.Addressee != employer {
			continue
		}
		pending = append(pending, strategy.NaturalLabourPendingView{Worker: id, Addressee: employer, Hour: hour})
	}
	obs := strategy.NaturalLabourDecisionObservation{
		Actor: employer, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		Energy:              naturalIntegerObservation(self.Body.Energy),
		GranaryStock:        naturalIntegerObservation(self.Granary.Stock),
		Capital:             naturalIntegerObservation(self.Worksite.Capital),
		Wip:                 naturalIntegerObservation(self.Worksite.Wip),
		OwnLastDecisionHour: self.Book.LastDecisionHour,
		Pending:             pending,
	}
	choice, err := f.naturalEvaluateLabourDecision(employer, obs)
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	if choice.Kind == strategy.NaturalLabourDecisionNone {
		return empty, NaturalAttempt{}, nil
	}
	worker, ok := w.actors[choice.Worker]
	if !ok || choice.Worker == employer {
		return empty, NaturalAttempt{}, ErrNaturalRunner
	}
	if choice.Kind == strategy.NaturalLabourDecisionAccept && !naturalLabourBookIdle(self.Book, hour) {
		// A policy accept that the world refuses voids with zero mutation
		// (the E12 class): the one engagement slot is still occupied by the
		// live previous-hour engagement, so the contract's structural accept
		// precondition fails. Nothing stages — the pending offer stays
		// pending and expires at the next pulse (rule 519), and the book's
		// decision audit stays unwritten.
		return empty, NaturalAttempt{Actor: employer, Time: at, Kind: "labour-void", Refused: naturalLabourEngagementBusyNote}, nil
	}
	proposal, err := NaturalLabourDecisionProposalFor(f.mechanism, naturalKey("labour-decision", at, employer), f.patches, hour, employer, choice.Worker, choice.Kind == strategy.NaturalLabourDecisionAccept, self, worker)
	if err != nil {
		return empty, NaturalAttempt{}, err
	}
	return proposal, NaturalAttempt{Actor: employer, Time: at, Kind: "labour-decision", Key: proposal.Key}, nil
}

// naturalAllocateGathers admits gather claims over the cap-free roster with
// the v1 rotation discipline generalized without ID-range arithmetic: the
// rotation start is (seed+hour+patch) mod 8, claimants rotate by
// actor mod 8 with ascending-ID tie-break — for any fixed claimant set this
// is the v1/v4 cyclic order re-based, because the removed constant ID-range
// offsets shifted every position equally. Pure and order-independent; there
// is no claims-count cap (P15): contention is resolved by the rotation, not
// by a bound.
func naturalAllocateGathers(patches, hour int, seed uint64, stocked []FoodFlowStockedSlot, claims []FoodFlowGatherClaim) ([]FoodFlowGatherAdmission, error) {
	if _, err := NaturalPhaseTime(hour, NaturalPhaseClaim); err != nil {
		return nil, ErrNaturalContract
	}
	slotsByPatch := make(map[int][]sim.EntityID, patches)
	seenSlots := make(map[sim.EntityID]bool, len(stocked))
	for _, slot := range stocked {
		index := int(slot.Patch) - NaturalPatchIDBase
		if index < 0 || index >= patches || seenSlots[slot.ID] {
			return nil, ErrNaturalContract
		}
		first, err := NaturalSlotID(patches, index, 0)
		if err != nil {
			return nil, err
		}
		last, err := NaturalSlotID(patches, index, NaturalSlotsPerPatch-1)
		if err != nil || slot.ID < first || slot.ID > last {
			return nil, ErrNaturalContract
		}
		seenSlots[slot.ID] = true
		slotsByPatch[index] = append(slotsByPatch[index], slot.ID)
	}
	for i := range slotsByPatch {
		sort.Slice(slotsByPatch[i], func(a, b int) bool { return slotsByPatch[i][a] < slotsByPatch[i][b] })
	}
	ordered := append([]FoodFlowGatherClaim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		if a.TargetPatch != b.TargetPatch {
			return a.TargetPatch < b.TargetPatch
		}
		if a.TargetSlot != b.TargetSlot {
			return a.TargetSlot < b.TargetSlot
		}
		if a.Energy != b.Energy {
			return a.Energy < b.Energy
		}
		if a.BagUnits != b.BagUnits {
			return a.BagUnits < b.BagUnits
		}
		return a.LastGatherHour < b.LastGatherHour
	})
	out := make([]FoodFlowGatherAdmission, len(ordered))
	eligible := make(map[int][]int, patches)
	for i, claim := range ordered {
		out[i] = FoodFlowGatherAdmission{Actor: claim.Actor, Patch: claim.TargetPatch}
		// Reject *all* duplicate claims, including inconsistent copies, so
		// an actor cannot gain priority by submitting more than one
		// intention.
		if (i > 0 && ordered[i-1].Actor == claim.Actor) || (i+1 < len(ordered) && ordered[i+1].Actor == claim.Actor) {
			out[i].Rejection = FoodFlowGatherDuplicateActor
			continue
		}
		index := int(claim.TargetPatch) - NaturalPatchIDBase
		first, err := NaturalSlotID(patches, index, 0)
		if err != nil || !naturalActorEntity(patches, claim.Actor) || !naturalPatchEntity(patches, claim.TargetPatch) {
			out[i].Rejection = FoodFlowGatherNoAccess
			continue
		}
		last, err := NaturalSlotID(patches, index, NaturalSlotsPerPatch-1)
		if err != nil || claim.TargetSlot < first || claim.TargetSlot > last {
			out[i].Rejection = FoodFlowGatherNoAccess
			continue
		}
		if claim.Energy <= 0 || claim.Energy > NaturalEnergyCapacity || claim.BagUnits != 0 || claim.LastGatherHour < NaturalNeverHour || claim.LastGatherHour >= int64(hour) {
			out[i].Rejection = FoodFlowGatherIneligible
			continue
		}
		eligible[index] = append(eligible[index], i)
	}
	for patch := range eligible {
		start := (seed + uint64(hour) + uint64(patch)) % NaturalSlotsPerPatch
		sort.Slice(eligible[patch], func(a, b int) bool {
			posA := uint64(ordered[eligible[patch][a]].Actor) % NaturalSlotsPerPatch
			posB := uint64(ordered[eligible[patch][b]].Actor) % NaturalSlotsPerPatch
			left := (posA + NaturalSlotsPerPatch - start) % NaturalSlotsPerPatch
			right := (posB + NaturalSlotsPerPatch - start) % NaturalSlotsPerPatch
			if left != right {
				return left < right
			}
			return ordered[eligible[patch][a]].Actor < ordered[eligible[patch][b]].Actor
		})
		available := slotsByPatch[patch]
		for rank, i := range eligible[patch] {
			switch {
			case len(available) == 0:
				out[i].Rejection = FoodFlowGatherNoStock
			case rank >= len(available):
				out[i].Rejection = FoodFlowGatherCapacity
			default:
				position := (int((seed+2*uint64(hour)+uint64(patch))%uint64(len(available))) + rank) % len(available)
				out[i].Slot = available[position]
			}
		}
	}
	return out, nil
}

// allocate precomputes the claim-step gather admissions with the cap-free
// allocator. The claim commits the atomic gather at h+1µs, so contention
// must be resolved before the claim step: the coordinator evaluates each
// waking actor's frozen policy choice on the same authoritative head the
// workers will see (pure, deterministic), then admits.
func (f *Natural) allocate(ctx context.Context, at sim.SimTime) error {
	f.admissions = make(map[sim.EntityID]FoodFlowGatherAdmission)
	f.claimDecisions = make(map[sim.EntityID]strategy.NaturalChoice)
	h := int(at / sim.SimTime(NaturalHour))
	claim, err := NaturalPhaseTime(h, NaturalPhaseClaim)
	if err != nil || at != claim {
		return nil
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	w, err := f.worldState(view)
	if err != nil {
		return err
	}
	var stocked []FoodFlowStockedSlot
	for i := range w.slots {
		patch, _ := NaturalPatchID(f.patches, i)
		for j := range w.slots[i] {
			if w.slots[i][j].Stock == 1 {
				slot, _ := NaturalSlotID(f.patches, i, j)
				stocked = append(stocked, FoodFlowStockedSlot{ID: slot, Patch: patch})
			}
		}
	}
	var claims []FoodFlowGatherClaim
	for _, wake := range f.sched.Snapshot().Wakes {
		if wake.At != at || (wake.Cause != scheduler.WakeNeedThreshold && wake.Cause != scheduler.WakeBirth) {
			continue
		}
		id := wake.Actor
		if !f.rosterContains(id) {
			return ErrNaturalRunner
		}
		a, ok := w.actors[id]
		if !ok || !naturalAlive(a) || a.Bag.Units != 0 {
			return ErrNaturalRunner
		}
		obs, err := f.observe(view, id, h, strategy.NaturalNoDenial)
		if err != nil {
			return err
		}
		choice, err := f.naturalEvaluate(id, obs, naturalLabourOfferGuard(a))
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return ErrNaturalRunner
		}
		if err := f.naturalSeamOverride(ctx, id, h, view.Version, obs, &choice); err != nil {
			return err
		}
		switch choice.Kind {
		case strategy.NaturalGather:
			if choice.TargetSlot == 0 {
				return ErrNaturalRunner
			}
			claims = append(claims, FoodFlowGatherClaim{Actor: id, TargetPatch: a.Bag.Source, TargetSlot: choice.TargetSlot, Energy: a.Body.Energy, BagUnits: a.Bag.Units, LastGatherHour: a.Body.LastGatherHour})
		case strategy.NaturalEatStored, strategy.NaturalWait, strategy.NaturalProvision:
			// No allocation needed; the worker re-evaluates and commits. A
			// provision converts granary stock, not a slot; the slot rotation
			// is untouched (and the v5/v6 cascades never emit it).
		default:
			return ErrNaturalRunner
		}
	}
	if len(claims) == 0 {
		return nil
	}
	admitted, err := naturalAllocateGathers(f.patches, h, f.seed, stocked, claims)
	if err != nil {
		return err
	}
	for _, a := range admitted {
		f.admissions[a.Actor] = a
	}
	return nil
}

// naturalLinkAttempt binds a keyed attempt to its committed event (v4
// two-flag semantics). The one legitimate keyless outcome is a birth that
// lost its kernel batch to a newborn-ID collision; a refused build or a
// typed labour void is legitimately unkeyed with its typed note. Anything
// else unlinked is a
// linkage break.
func naturalLinkAttempt(a *NaturalAttempt, ids map[string]sim.EventID) error {
	if a.Key == "" {
		if a.EventID != 0 || a.Accepted || a.Lost != "" {
			return ErrNaturalRunner
		}
		return nil
	}
	if id, ok := ids[a.Key]; ok {
		a.EventID, a.Accepted = id, true
		return nil
	}
	if a.Kind == "birth-reply" && a.Birth.Decision == NaturalReplyAccept {
		a.Lost = naturalLostCollision
		return nil
	}
	return ErrNaturalRunner
}

// ceilingHit assembles the typed evidence with the runner's current
// run-state context.
// naturalAblationEventGuard fails loud on any rule-505 event while the
// declared cultivation ablation is on (frozen matrix cell D): the ablation
// routes every policy Build choice through the documented denial loop, so a
// committed build event is a runner defect, never an ecology outcome.
func (f *Natural) naturalAblationEventGuard(rule sim.RuleID) error {
	if rule == NaturalBuildRule {
		return fmt.Errorf("%w: cultivation ablation committed a rule-505 build event", ErrNaturalRunner)
	}
	return nil
}

// naturalAblationAttemptGuard is the attempt-level twin of
// naturalAblationEventGuard: no committed build attempt may be recorded
// while the ablation is on.
func (f *Natural) naturalAblationAttemptGuard(kind string) error {
	if kind == "build" {
		return fmt.Errorf("%w: cultivation ablation recorded a committed build attempt", ErrNaturalRunner)
	}
	return nil
}

func (f *Natural) ceilingHit(ceiling NaturalCeiling, threshold, observed uint64, hour int, version sim.WorldVersion) NaturalCeilingHit {
	hit := NaturalCeilingHit{Ceiling: ceiling, Threshold: threshold, Observed: observed, Hour: hour, WorldVersion: version}
	hit.Births = int64(len(f.newborns))
	hit.Deaths = f.deathsAge + f.deathsStarve
	hit.Alive = len(f.roster) - int(hit.Deaths)
	for _, slot := range f.flow {
		hit.Last24hInflow += slot.inflow
		hit.Last24hConsumption += slot.meals
	}
	if len(f.checkpoints) > 0 {
		hit.RegenUnrealized = f.checkpoints[len(f.checkpoints)-1].Balance.Unrealized
	}
	return hit
}

func (f *Natural) haltWithHit(hit NaturalCeilingHit) error {
	f.hits = append(f.hits, hit)
	f.halted = &NaturalCeilingHitError{Hit: hit}
	return f.halted
}

// naturalEventLoad accounts one committed event for the ledger: encoded
// body bytes, patch count, value-node count (the export codec counts the
// before and after encodings of every delta), and per-item maxima.
func naturalEventLoad(e kernel.Event) (patches, nodes, bodyBytes, maxValueBytes uint64, err error) {
	body, err := e.Bytes()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	patches = uint64(len(e.Deltas))
	bodyBytes = uint64(len(body)) + 68 // the export budget charges 4+32+32 framing per event
	for _, d := range e.Deltas {
		for _, v := range []sim.Value{d.Before, d.After} {
			encoded, err := sim.EncodeValue(v)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			if uint64(len(encoded)) > maxValueBytes {
				maxValueBytes = uint64(len(encoded))
			}
			used, err := sim.CountValueNodes(encoded, sim.MaxValueBytes)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			nodes += used
		}
	}
	return patches, nodes, bodyBytes, maxValueBytes, nil
}

// naturalProjectionLoad mirrors the kernel's projection-hash walk
// (count-only): the total projected metric rows and their encoded bytes,
// which are exactly the quantities MaxHistoryProjectedMetrics and
// MaxHistoryProjectionBytes bound at export.
func naturalProjectionLoad(reg component.Registry, reader component.Reader, authority component.Authority, version sim.WorldVersion) (count, size uint64, err error) {
	for _, id := range reg.TypeIDs() {
		descriptor, err := reg.Describe(id)
		if err != nil {
			return 0, 0, err
		}
		projections := append([]component.ProjectionDescriptor(nil), descriptor.Projections...)
		sort.Slice(projections, func(i, j int) bool { return projections[i].ID < projections[j].ID })
		for _, projection := range projections {
			batch, err := reader.Project(component.ProjectRequest{Component: id, Projection: projection.ID, WorldVersion: version, Authority: authority})
			if err != nil {
				return 0, 0, err
			}
			count += uint64(batch.Len())
			for i := 0; i < batch.Len(); i++ {
				metric, err := batch.At(i)
				if err != nil {
					return 0, 0, err
				}
				body, err := (kernel.Event{Metrics: []component.MetricDelta{{Before: metric, After: metric}}}).Bytes()
				if err != nil {
					return 0, 0, err
				}
				size += uint64(len(body))
			}
		}
	}
	return count, size, nil
}

// Step closes one timestamp; the frozen 168h horizon takes at most
// NaturalStepCeiling steps. Newborn fibers are registered at the quiescent
// boundary after a birth commits, with the first wake reserved at the next
// claim as the WakeBirth cause; the allocator advances exactly there, so its
// last-ID always equals the highest committed newborn (restore cross-check).
func (f *Natural) Step(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.halted != nil {
		return false, f.halted
	}
	snap := f.sched.Snapshot()
	if len(snap.Wakes) == 0 {
		return false, nil
	}
	at := snap.Wakes[0].At
	hour := sim.SimTime(NaturalHour)
	h := int(at / hour)
	if uint64(f.steps+1) > f.ledger.thresholds[NaturalCeilingRunnerSteps] {
		return false, f.haltWithHit(f.ceilingHit(NaturalCeilingRunnerSteps, f.ledger.thresholds[NaturalCeilingRunnerSteps], uint64(f.steps+1), h, snap.Kernel.Version))
	}
	if at%hour == 0 {
		// Quiescent pulse boundary: the fiber set equals the roster plus the
		// clock exactly (set equality, no fixed counts), every fiber idle,
		// and the lifecycle matching the frozen alive invariant.
		if len(snap.Fibers) != len(f.roster)+1 || len(f.meals) != 0 {
			return false, ErrNaturalRunner
		}
		byActor := make(map[sim.EntityID]scheduler.Fiber, len(snap.Fibers))
		for _, fiber := range snap.Fibers {
			byActor[fiber.Actor] = fiber
		}
		if _, ok := byActor[naturalClockActor]; !ok {
			return false, ErrNaturalRunner
		}
		for _, id := range f.roster {
			fiber, ok := byActor[id]
			if !ok || fiber.Activity != nil {
				return false, ErrNaturalRunner
			}
			f.pulseLifecycle[id] = fiber.Lifecycle
		}
	}
	if err := f.allocate(ctx, at); err != nil {
		return false, err
	}
	// Completion-instant arbitration before the workers run (same batch,
	// same pre-batch snapshot, the declared settlement-precedence fold:
	// settlements ascending worker ID first, then builds ascending builder
	// ID, with field-set intersections voided before they are proposed).
	if start, err := NaturalPhaseTime(h, NaturalPhaseBuildStart); err == nil && at == NaturalBuildCompletion(start) {
		var completions, settleActors []sim.EntityID
		for _, wake := range snap.Wakes {
			if wake.At != at || !f.rosterContains(wake.Actor) {
				continue
			}
			switch wake.Cause {
			case scheduler.WakeCompletion:
				completions = append(completions, wake.Actor)
			case scheduler.WakeAudit:
				// The only audit wake at the completion instant is a rule-518
				// settlement attempt (the bag decision's Deliver wake).
				settleActors = append(settleActors, wake.Actor)
			}
		}
		head := f.k.SnapshotHead()
		if err := f.arbitrateCompletions(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, at, completions, settleActors); err != nil {
			return false, err
		}
	}
	f.pending = &naturalPending{started: make(map[sim.EntityID]strategy.NaturalAction), meals: make(map[sim.EntityID]NaturalMealIntent)}
	defer func() { f.pending = nil; f.builds = nil; f.settles = nil }()
	events, processed, err := f.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	// Newborn registration at the quiescent boundary, post-commit.
	for _, ev := range events {
		if ev.Kind != kernel.KindEntityCreate || ev.Rule != NaturalBirthReplyRule {
			continue
		}
		if err := f.registerNewborn(ev, h); err != nil {
			return false, err
		}
	}
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	if f.cultivationDisabled {
		for _, ev := range events {
			if err := f.naturalAblationEventGuard(ev.Rule); err != nil {
				return false, err
			}
		}
	}
	head := f.k.SnapshotHead()
	batch := NaturalBatch{Time: at, Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for _, a := range f.pending.attempts {
		if f.cultivationDisabled {
			if err := f.naturalAblationAttemptGuard(a.Kind); err != nil {
				return false, err
			}
		}
		if err := naturalLinkAttempt(&a, ids); err != nil {
			return false, err
		}
		batch.Attempts = append(batch.Attempts, a)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool {
		a, b := batch.Attempts[i], batch.Attempts[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Time < b.Time
	})
	for _, a := range batch.Attempts {
		if a.Accepted && naturalMealKinds[a.Kind] {
			f.hourMeals++
		}
	}
	for _, id := range f.pending.completed {
		if _, ok := f.current[id]; !ok {
			return false, ErrNaturalRunner
		}
		delete(f.current, id)
	}
	for id, kind := range f.pending.started {
		fiber, ok := f.sched.Fiber(id)
		if !ok || fiber.Activity == nil {
			return false, ErrNaturalRunner
		}
		f.current[id] = kind
	}
	for _, id := range f.pending.denied {
		f.denied[id] = int64(h)
	}
	for _, id := range f.pending.deniedBuild {
		f.deniedBuild[id] = int64(h)
	}
	for id, intent := range f.pending.meals {
		f.meals[id] = intent
	}
	for _, id := range f.pending.mealsCleared {
		delete(f.meals, id)
	}
	for _, id := range f.pending.birthWakesUsed {
		if _, ok := f.birthWakes[id]; !ok {
			return false, ErrNaturalRunner
		}
		delete(f.birthWakes, id)
	}
	for _, d := range f.pending.deaths {
		switch d.Cause {
		case "age":
			f.deathsAge++
		case "starvation":
			f.deathsStarve++
		default:
			return false, ErrNaturalRunner
		}
		fiber, ok := f.sched.Fiber(d.Actor)
		if !ok || fiber.Lifecycle != scheduler.Stopped {
			return false, ErrNaturalRunner // every recorded death stopped its fiber
		}
		f.pulseLifecycle[d.Actor] = scheduler.Stopped
	}
	f.birthFiles += int64(f.pending.birthFiles)
	f.divergences += int64(f.pending.divergedReplies)
	for _, reason := range f.pending.replyReasons {
		f.refusals[reason]++
	}
	if len(f.pending.hits) > 0 {
		// A worker recorded a typed ceiling hit (e.g. the allocator bound at
		// the reply decision); fail loud before any further bookkeeping.
		hit := f.pending.hits[0]
		f.hits = append(f.hits, f.pending.hits...)
		f.halted = &NaturalCeilingHitError{Hit: hit}
		return false, f.halted
	}
	f.steps++
	f.journal = append(f.journal, batch)
	// Ledger: incremental counters from the committed events, then the
	// typed fail-loud check (never silent truncation).
	for _, ev := range events {
		patches, nodes, bodyBytes, maxValueBytes, err := naturalEventLoad(ev)
		if err != nil {
			return false, err
		}
		f.ledger.add(NaturalCeilingHistoryEvents, 1)
		f.ledger.add(NaturalCeilingHistoryPatches, patches)
		f.ledger.add(NaturalCeilingHistoryValueNodes, nodes)
		f.ledger.add(NaturalCeilingHistoryBytes, bodyBytes)
		f.ledger.max(NaturalCeilingEventBytes, bodyBytes)
		f.ledger.max(NaturalCeilingValueBytes, maxValueBytes)
	}
	if ceiling, crossed := f.ledger.crossing(); crossed {
		return false, f.haltWithHit(f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, head.Version))
	}
	if at%hour == 0 {
		// Trailing-24-pulse flow ring: this pulse's inflow is the committed
		// regen delta; the meals ring slot closes the previous hour.
		check, err := f.checkpoint(h, snap)
		if err != nil {
			var hitErr *NaturalCeilingHitError
			if errors.As(err, &hitErr) {
				return false, f.haltWithHit(hitErr.Hit)
			}
			return false, fmt.Errorf("natural hour %d: %w", h, err)
		}
		f.checkpoints = append(f.checkpoints, check)
	}
	return true, nil
}

// registerNewborn allocates the newborn's fiber from a committed
// entity-create event: the newborn is the unique genome-row entity not yet
// on the roster, its ID must be exactly the allocator's next (committed
// births advance it in order), the frozen policy is bound fresh, and the
// first wake is reserved at the next claim as WakeBirth.
func (f *Natural) registerNewborn(ev kernel.Event, hour int) error {
	newborn := sim.EntityID(0)
	for _, d := range ev.Deltas {
		if d.Component != NaturalGenomeTypeID {
			continue
		}
		// Rule 511 also patches the parents' genome rows (the
		// birth-granary-paid ledger), and a parent may itself be a
		// registered newborn. The allocated newborn is the unique
		// genome-row entity not yet in the roster; two distinct
		// unregistered newborn rows in one event remain a hard error.
		if f.rosterContains(d.Entity) {
			continue
		}
		if newborn != 0 && d.Entity != newborn {
			return ErrNaturalRunner
		}
		newborn = d.Entity
	}
	if !naturalNewbornEntity(f.patches, newborn) || f.rosterContains(newborn) {
		return ErrNaturalRunner
	}
	if expected := f.allocator.Last() + 1; newborn != expected {
		return fmt.Errorf("%w: committed newborn %d diverges from the allocator next %d", ErrNaturalRunner, newborn, expected)
	}
	if _, err := NaturalNextNewbornID(&f.allocator); err != nil {
		return fmt.Errorf("%w: allocator advance failed at %d: %v", ErrNaturalRunner, f.allocator.Last(), err)
	}
	if _, exists := f.sched.Fiber(newborn); exists {
		return ErrNaturalRunner
	}
	if len(f.roster) > 0 && newborn <= f.roster[len(f.roster)-1] {
		return ErrNaturalRunner // the roster stays ascending by ID
	}
	bound, err := f.policies.Bind(strategy.NaturalBinding{Actor: newborn, Ref: f.ref})
	if err != nil {
		return err
	}
	if err = f.sched.Register(newborn); err != nil {
		return err
	}
	f.bound[newborn] = bound
	f.roster = append(f.roster, newborn)
	f.newborns = append(f.newborns, newborn)
	f.pulseLifecycle[newborn] = scheduler.Alive
	if hour+1 <= NaturalHorizonHours-1 {
		claim, err := NaturalPhaseTime(hour+1, NaturalPhaseClaim)
		if err != nil {
			return err
		}
		if err = f.sched.Schedule(scheduler.Wake{Actor: newborn, At: claim, Cause: scheduler.WakeBirth}); err != nil {
			return err
		}
		f.birthWakes[newborn] = int64(hour + 1)
	}
	return nil
}

// checkpoint projects the hourly boundary: the complete state, the extended
// conservation gates (including the allocator cross-check), the hourly
// ledger measurement with its fail-loud projection, and the frozen metrics.
func (f *Natural) checkpoint(h int, snap scheduler.Snapshot) (NaturalCheckpoint, error) {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	w, err := f.worldState(view)
	if err != nil {
		return NaturalCheckpoint{}, err
	}
	if len(w.actors) != len(f.roster) {
		return NaturalCheckpoint{}, ErrNaturalRunner
	}
	founders := make([]NaturalActorState, f.founders)
	for i := range founders {
		a, ok := w.actors[sim.EntityID(i+1)]
		if !ok {
			return NaturalCheckpoint{}, ErrNaturalRunner
		}
		founders[i] = a
	}
	newborns := make([]NaturalNewbornState, 0, len(f.newborns))
	for _, id := range f.newborns {
		a, ok := w.actors[id]
		if !ok {
			return NaturalCheckpoint{}, ErrNaturalRunner
		}
		newborns = append(newborns, NaturalNewbornState{ID: id, State: a})
	}
	// Gates G1/G2a/G2b/G3 extended, every row revalidated inside, and the
	// allocator cross-check (mismatch fails closed).
	balance, err := NaturalCheckConservationFor(f.mechanism, f.patches, w.patches, w.slots, founders, newborns, f.allocator.Last())
	if err != nil {
		return NaturalCheckpoint{}, err
	}
	// Trailing-24-pulse flow ring: this pulse's committed inflow delta and
	// the meals of the closing hour.
	var prevProducedGranted int64
	if len(f.checkpoints) > 0 {
		prev := f.checkpoints[len(f.checkpoints)-1].Balance
		prevProducedGranted = prev.Produced + prev.Granted
	}
	f.flow[h%len(f.flow)] = naturalHourFlow{inflow: balance.Produced + balance.Granted - prevProducedGranted, meals: f.hourMeals}
	f.hourMeals = 0
	// Hourly ledger measurement: snapshot bytes and the projection loads
	// are exact current-state quantities; fibers/wakes come from the
	// scheduler snapshot.
	snapshot, err := component.EncodeSnapshot(f.registry, head.Reader, head.Authority, head.Version)
	if err != nil {
		return NaturalCheckpoint{}, err
	}
	f.ledger.set(NaturalCeilingSnapshotBytes, uint64(len(snapshot)))
	metricCount, metricBytes, err := naturalProjectionLoad(f.registry, head.Reader, head.Authority, head.Version)
	if err != nil {
		return NaturalCheckpoint{}, err
	}
	f.ledger.set(NaturalCeilingProjectedMetrics, metricCount)
	f.ledger.set(NaturalCeilingProjectionBytes, metricBytes)
	f.ledger.set(NaturalCeilingPortableFibers, uint64(len(snap.Fibers)))
	f.ledger.set(NaturalCeilingPortableWakes, uint64(len(snap.Wakes)))
	f.ledger.set(NaturalCeilingRunnerSteps, uint64(f.steps))
	f.ledger.set(NaturalCeilingAllocatorIDs, uint64(f.allocator.Last()))
	if ceiling, crossed := f.ledger.crossing(); crossed {
		return NaturalCheckpoint{}, &NaturalCeilingHitError{Hit: f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, head.Version)}
	}
	if ceiling, soon := f.ledger.projected(); soon {
		return NaturalCheckpoint{}, &NaturalCeilingHitError{Hit: f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, head.Version)}
	}
	rows := f.ledger.rows(h)
	for _, ceiling := range naturalHourlyProjectedCeilings {
		f.ledger.prev[ceiling] = f.ledger.observed[ceiling]
	}
	alive := 0
	for _, id := range f.roster {
		if naturalAlive(w.actors[id]) {
			alive++
		}
		if (f.pulseLifecycle[id] == scheduler.Alive) != naturalAlive(w.actors[id]) {
			return NaturalCheckpoint{}, ErrNaturalRunner
		}
	}
	metrics := NaturalMetrics{
		Alive: alive, DeathsAge: f.deathsAge, DeathsStarvation: f.deathsStarve,
		Births: balance.Births, BirthRequests: f.birthFiles, Divergences: f.divergences,
		Refusals: make(map[NaturalRefusalReason]int64, len(f.refusals)),
	}
	for reason, n := range f.refusals {
		metrics.Refusals[reason] = n
	}
	var prev NaturalBalance
	if len(f.checkpoints) > 0 {
		prev = f.checkpoints[len(f.checkpoints)-1].Balance
	}
	metrics.RegenCultivated = balance.Granted - prev.Granted
	metrics.RegenWild = balance.Produced - prev.Produced
	metrics.RegenUnrealized = balance.Unrealized - prev.Unrealized
	patches := append([]NaturalPatchState(nil), w.patches...)
	slots := make([][]NaturalSlotState, len(w.slots))
	for i := range w.slots {
		slots[i] = append([]NaturalSlotState(nil), w.slots[i]...)
	}
	return NaturalCheckpoint{Hour: h, Version: head.Version, Balance: balance, Patches: patches, Slots: slots,
		Founders: founders, Newborns: newborns, AllocatorLast: f.allocator.Last(), Metrics: metrics, Ledger: rows}, nil
}

// Run walks the frozen 168-hour horizon; a typed ceiling hit stops the run
// fail-loud and is returned as its typed error.
func (f *Natural) Run(ctx context.Context) error {
	for {
		processed, err := f.Step(ctx)
		if err != nil {
			return err
		}
		if !processed {
			break
		}
	}
	checks := f.Checkpoints()
	if len(checks) != NaturalHorizonHours+1 || checks[len(checks)-1].Hour != NaturalHorizonHours {
		return ErrNaturalRunner
	}
	return nil
}

func (f *Natural) SchedulerSnapshot() scheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sched.Snapshot()
}

func (f *Natural) Journal() []NaturalBatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneNaturalJournal(f.journal)
}

func cloneNaturalJournal(batches []NaturalBatch) []NaturalBatch {
	out := make([]NaturalBatch, len(batches))
	for i, b := range batches {
		out[i] = b
		out[i].Attempts = append([]NaturalAttempt(nil), b.Attempts...)
	}
	return out
}

func (f *Natural) Checkpoints() []NaturalCheckpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneNaturalCheckpoints(f.checkpoints)
}

func cloneNaturalCheckpoints(checks []NaturalCheckpoint) []NaturalCheckpoint {
	out := make([]NaturalCheckpoint, len(checks))
	for i := range checks {
		out[i] = checks[i]
		out[i].Patches = append([]NaturalPatchState(nil), checks[i].Patches...)
		out[i].Slots = make([][]NaturalSlotState, len(checks[i].Slots))
		for j := range checks[i].Slots {
			out[i].Slots[j] = append([]NaturalSlotState(nil), checks[i].Slots[j]...)
		}
		out[i].Founders = append([]NaturalActorState(nil), checks[i].Founders...)
		out[i].Newborns = append([]NaturalNewbornState(nil), checks[i].Newborns...)
		out[i].Ledger = append([]NaturalCeilingRow(nil), checks[i].Ledger...)
		out[i].Metrics.Refusals = make(map[NaturalRefusalReason]int64, len(checks[i].Metrics.Refusals))
		for reason, n := range checks[i].Metrics.Refusals {
			out[i].Metrics.Refusals[reason] = n
		}
	}
	return out
}

// CeilingHits exposes the typed ceiling evidence recorded so far.
func (f *Natural) CeilingHits() []NaturalCeilingHit {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]NaturalCeilingHit(nil), f.hits...)
}

// Handoff captures the portable kernel and scheduler bytes plus runner-only
// state under the coordinator boundary, with the ceiling pre-check and the
// typed classification of an export failure (fail-loud, never silent
// truncation).
func (f *Natural) Handoff() (NaturalHandoff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.halted != nil {
		return NaturalHandoff{}, f.halted
	}
	if ceiling, crossed := f.ledger.crossing(); crossed {
		h := int(f.sched.Time() / sim.SimTime(NaturalHour))
		return NaturalHandoff{}, f.haltWithHit(f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, f.k.SnapshotHead().Version))
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		for _, ceiling := range naturalCeilingOrder {
			if f.ledger.observed[ceiling] > f.ledger.thresholds[ceiling] {
				h := int(f.sched.Time() / sim.SimTime(NaturalHour))
				return NaturalHandoff{}, f.haltWithHit(f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, head.Version))
			}
		}
		return NaturalHandoff{}, err
	}
	portable, schedHead, err := f.sched.ExportPortable()
	if err != nil {
		return NaturalHandoff{}, err
	}
	if head != schedHead {
		return NaturalHandoff{}, ErrNaturalRunner
	}
	f.ledger.set(NaturalCeilingPortableBytes, uint64(len(portable)))
	if ceiling, crossed := f.ledger.crossing(); crossed {
		h := int(f.sched.Time() / sim.SimTime(NaturalHour))
		return NaturalHandoff{}, f.haltWithHit(f.ceilingHit(ceiling, f.ledger.thresholds[ceiling], f.ledger.observed[ceiling], h, head.Version))
	}
	out := NaturalHandoff{FormatVersion: NaturalFormatVersion, Patches: f.patches, R: f.r, Seed: f.seed, Births: f.births,
		Policy: f.ref, Steps: f.steps, History: history, SchedulerBytes: portable, Head: head,
		Newborns:          append([]sim.EntityID(nil), f.newborns...),
		BirthWakes:        make(map[sim.EntityID]int64, len(f.birthWakes)),
		Current:           make(map[sim.EntityID]strategy.NaturalAction, len(f.current)),
		DeniedGather:      make(map[sim.EntityID]int64, len(f.denied)),
		DeniedBuild:       make(map[sim.EntityID]int64, len(f.deniedBuild)),
		Meals:             make(map[sim.EntityID]NaturalMealIntent, len(f.meals)),
		AllocatorLast:     f.allocator.Last(),
		CeilingThresholds: make(map[NaturalCeiling]uint64, len(f.thresholds)),
		Ledger:            f.ledger.rows(int(f.sched.Time() / sim.SimTime(NaturalHour))),
		Hits:              append([]NaturalCeilingHit(nil), f.hits...),
		Journal:           cloneNaturalJournal(f.journal), Checkpoints: cloneNaturalCheckpoints(f.checkpoints)}
	for id, v := range f.birthWakes {
		out.BirthWakes[id] = v
	}
	for id, v := range f.current {
		out.Current[id] = v
	}
	for id, v := range f.denied {
		out.DeniedGather[id] = v
	}
	for id, v := range f.deniedBuild {
		out.DeniedBuild[id] = v
	}
	for id, v := range f.meals {
		out.Meals[id] = v
	}
	for ceiling, v := range f.thresholds {
		out.CeilingThresholds[ceiling] = v
	}
	return out, nil
}

// naturalPolicyRefusalReason maps a policy-side consent refusal reason into the
// contract's refusal vocabulary, so a policy refusal that the contract's
// weaker truth gate would have accepted is recorded as the typed refusal the
// policy actually made (the exact inverse of the accept-side tolerance).
func naturalPolicyRefusalReason(reason strategy.NaturalReplyReason) (NaturalRefusalReason, bool) {
	switch reason {
	case strategy.NaturalReplyReasonConsentOwn:
		return NaturalRefusalConsentOwn, true
	case strategy.NaturalReplyReasonConsentClaims:
		return NaturalRefusalConsentClaims, true
	case strategy.NaturalReplyReasonKin:
		return NaturalRefusalKin, true
	default:
		return 0, false
	}
}
