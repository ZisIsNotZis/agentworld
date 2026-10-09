// Package protocol is the transport-free cognitive-layer protocol surface: the
// Appendix A request/response types, the declared run identity, the C7 wake
// policy, and the Appendix C parameter identities. It contains no model client,
// no gateway stub, and no ledger or transport code, so internal/world can
// depend on it without depending on the model client (spec section 6.6, EC-6).
//
// Only internal/sim and internal/strategy values appear here; the package
// imports neither net nor internal/world.
package protocol

import (
	"context"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// Declared schema and seam metadata (spec section 6.2). The schema version is
// part of the canonical request; the seam and prompt template are run identity.
const (
	// ActorSeamSchemaVersion is the declared canonical-request schema version.
	ActorSeamSchemaVersion = 1
	// SeamNaturalClaim is the one declared Role A seam.
	SeamNaturalClaim = "natural.claim"
	// NaturalClaimPromptTemplate is the versioned gateway prompt template.
	NaturalClaimPromptTemplate = "natural-claim/v1"
)

// Source values recorded on ActorSeamDecision.Source (spec section 4.3).
const (
	SourceModel    = "model"
	SourceFallback = "fallback"
)

// ParamID names one declared strategy parameter (Appendix C). The parameter
// contract and its domains are the patch lane's; this package carries the
// identity and the declared frozen vector.
type ParamID string

// The declared Appendix C parameter ids (the patchable strategy-side threshold
// vector).
const (
	ParamClaimProvisionEnergyFloor ParamID = "claim.provision_energy_floor"
	ParamClaimBankEnergyCeiling    ParamID = "claim.bank_energy_ceiling"
	ParamClaimGatherEnergyFloor    ParamID = "claim.gather_energy_floor"
	ParamBagBuildEnergyFloor       ParamID = "bag.build_energy_floor"
	ParamBagBuildStartEnergyFloor  ParamID = "bag.build_start_energy_floor"
	ParamBagBuildFinishEnergyFloor ParamID = "bag.build_finish_energy_floor"
	ParamBirthFilingEnergyFloor    ParamID = "birth.filing_energy_floor"
)

// FixtureIdentity is the declared experiment fixture (spec section 6.2).
type FixtureIdentity struct {
	Patches   int    `json:"patches"`
	R         int    `json:"r"`
	Founders  int    `json:"founders"`
	Mechanism uint32 `json:"mechanism"`
	Births    bool   `json:"births"`
}

// RunIdentity is the recorded run identity block. IdentityDigest binds it.
type RunIdentity struct {
	Fixture        FixtureIdentity     `json:"fixture"`
	Seed           uint64              `json:"seed"`
	BaseRef        strategy.NaturalRef `json:"-"`
	HeadRef        strategy.NaturalRef `json:"-"`
	IdentityDigest [32]byte            `json:"-"`
}

// ActorSeamRequest is the declared Role A request (Appendix A). It is built
// only from strategy values; Observation is the frozen actor-visible value and
// Strategy is the actor's own bound parameter vector.
type ActorSeamRequest struct {
	Seam        string
	Actor       sim.EntityID
	Hour        int
	Version     sim.WorldVersion
	Observation strategy.NaturalObservation
	Strategy    map[ParamID]int64
	Actions     []string
	Identity    RunIdentity
}

// ActorSeamDecision is the declared Role A outcome (Appendix A). Source is
// "model" or "fallback"; Reason is the typed fallback reason.
type ActorSeamDecision struct {
	Action     string
	Rationale  string
	Confidence float64
	Source     string
	CallRef    uint64
	Reason     string
}

// TrajectoryEvidence is the Role B aggregate packet (spec section 5.1). It
// carries only world-level aggregate/state rows: no actor-private state and no
// other actor's private state.
type TrajectoryEvidence struct {
	WindowStartHour int              `json:"window_start_hour"`
	WindowHours     int              `json:"window_hours"`
	AliveByHour     []int            `json:"alive_by_hour"`
	BirthsTotal     int64            `json:"births_total"`
	BirthsByCause   map[string]int64 `json:"births_by_cause"`
	DeathsTotal     int64            `json:"deaths_total"`
	DeathsByCause   map[string]int64 `json:"deaths_by_cause"`
	Actors          []ActorEvidence  `json:"actors"`
	Patches         []PatchEvidence  `json:"patches"`
	BuildsCommitted int64            `json:"builds_committed"`
	MealsCommitted  int64            `json:"meals_committed"`
	MaxAlive        int              `json:"max_alive"`
	MaxBirths       int64            `json:"max_births"`
	MaxCapital      int64            `json:"max_capital"`
}

// ActorEvidence is one actor's published capital and granary holding.
type ActorEvidence struct {
	Actor        sim.EntityID `json:"actor"`
	Capital      int64        `json:"capital"`
	GranaryStock int64        `json:"granary_stock"`
}

// PatchEvidence is one patch's published mouths and total capital.
type PatchEvidence struct {
	Patch   sim.EntityID `json:"patch"`
	Mouths  int64        `json:"mouths"`
	Capital int64        `json:"capital"`
}

// ParameterSpec declares one patchable parameter (Appendix C).
type ParameterSpec struct {
	ID            ParamID `json:"id"`
	Meaning       string  `json:"meaning"`
	DecisionPoint string  `json:"decision_point"`
	ParentValue   int64   `json:"parent_value"`
	DomainMin     int64   `json:"domain_min"`
	DomainMax     int64   `json:"domain_max"`
}

// ParameterDelta is one typed integer delta over the declared contract.
type ParameterDelta struct {
	Param ParamID `json:"param"`
	Value int64   `json:"value"`
}

// PatchRequest is the declared Role B request (Appendix A).
type PatchRequest struct {
	Role       string
	AuditHour  int
	Evidence   TrajectoryEvidence
	ParentRef  strategy.NaturalRef
	Parameters []ParameterSpec
	Identity   RunIdentity
}

// PatchProposal is the declared Role B outcome (Appendix A).
type PatchProposal struct {
	Propose   bool
	ParentRef strategy.NaturalRef
	Deltas    []ParameterDelta
	Intent    string
	CallRef   uint64
}

// SeamDecider is the model-call protocol interface (spec section 6.1,
// Appendix A). It is defined where it is consumed and implemented by the
// cognition client stack; only internal/sim and internal/strategy types appear
// in its requests.
type SeamDecider interface {
	DecideActorSeam(ctx context.Context, req ActorSeamRequest) (ActorSeamDecision, error)
	ProposeStrategyPatch(ctx context.Context, req PatchRequest) (PatchProposal, error)
}
