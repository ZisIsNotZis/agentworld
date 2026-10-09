package cognition

import (
	"crypto/sha256"
	"fmt"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role A observation-boundary encoder (spec section 6.2). The canonical
// request is produced by an explicit allow-list encoder: every field is
// written by name from the frozen actor-observable value and the declared
// protocol identity. There is no reflection over structs, no map iteration
// outside the canonical encoder, and no world reader in scope, so no
// privileged state can reach the request without editing this file and the
// schema version.
//
// The only strategy value read here is the frozen strategy.NaturalObservation
// and the actor's own bound parameter vector; the request carries nothing a
// second actor or the world holds privately (no tenure mode or owner, no
// debts, no spoilage counter, no allocator state, no R, no seed, no kin, no
// other actor's rows) because the observation type has no field for them.
const (
	// ActorSeamSchemaVersion is the declared canonical-request schema version.
	ActorSeamSchemaVersion = 1
	// SeamNaturalClaim is the one declared Role A seam.
	SeamNaturalClaim = "natural.claim"
	// NaturalClaimPromptTemplate is the versioned gateway prompt template.
	NaturalClaimPromptTemplate = "natural-claim/v1"

	// Role A declared metadata (spec section 6.2); part of run identity.
	roleActorDecision = RoleActorDecision
)

// Declared Appendix C parameter ids (the patchable strategy-side threshold
// vector). Only the identities live here; the patch contract and its
// validation belong to the patch lane.
const (
	ParamClaimProvisionEnergyFloor ParamID = "claim.provision_energy_floor"
	ParamClaimBankEnergyCeiling    ParamID = "claim.bank_energy_ceiling"
	ParamClaimGatherEnergyFloor    ParamID = "claim.gather_energy_floor"
	ParamBagBuildEnergyFloor       ParamID = "bag.build_energy_floor"
	ParamBagBuildStartEnergyFloor  ParamID = "bag.build_start_energy_floor"
	ParamBagBuildFinishEnergyFloor ParamID = "bag.build_finish_energy_floor"
	ParamBirthFilingEnergyFloor    ParamID = "birth.filing_energy_floor"
)

// The frozen Appendix C values for the VS1 base cascade natural/7 (v6) and
// its natural/7b ablation, which share the same thresholds. These are the
// declared frozen values of the bound cascade, not tunables: the patchable
// generation (a later lane) substitutes them at the declared decision points.
var frozenNatural7Parameters = map[ParamID]int64{
	// The natural/7 claim cascade (strategy natural7.go): provision iff
	// granary >= 1 and energy >= 4; the bank is food only at energy <= 3.
	ParamClaimProvisionEnergyFloor: 4,
	ParamClaimBankEnergyCeiling:    3,
	// A living claimer gathers whenever a stocked home slot is visible
	// (energy == 0 short-circuits before the claim), so the gather floor is 1.
	ParamClaimGatherEnergyFloor: 1,
	// The v6-verbatim bag decision (strategy natural6.go, 3/4/4): the shared
	// investment guard (and finish leg), the re-pioneer leg, and the
	// birth filing/accept floor.
	ParamBagBuildEnergyFloor:       3,
	ParamBagBuildStartEnergyFloor:  4,
	ParamBagBuildFinishEnergyFloor: 3,
	ParamBirthFilingEnergyFloor:    4,
}

// FrozenNaturalClaimParameters returns the declared Appendix C frozen
// parameter vector of the actor's bound cascade. The VS1 seam's declared
// base policy is natural/7 (spec section 11.3), whose ablation natural/7b
// shares the same thresholds; every other bound ref returns an empty vector,
// because the declared parameter contract is defined there and a seam bound
// to another cascade is outside the VS1 base.
func FrozenNaturalClaimParameters(ref strategy.NaturalRef) map[ParamID]int64 {
	if ref != strategy.FrozenNaturalPolicy7().Ref && ref != strategy.FrozenNaturalPolicy7b().Ref {
		return map[ParamID]int64{}
	}
	out := make(map[ParamID]int64, len(frozenNatural7Parameters))
	for id, value := range frozenNatural7Parameters {
		out[id] = value
	}
	return out
}

// RunIdentityDigest is the declared identity digest: SHA-256 over the
// canonical run identity block (fixture, seed, base ref, head ref). It binds
// the request to the run identity without a self-referential field.
func RunIdentityDigest(id RunIdentity) [32]byte {
	block := runWire{
		Fixture: fixtureWire{
			Patches: id.Fixture.Patches, R: id.Fixture.R, Founders: id.Fixture.Founders,
			Mechanism: id.Fixture.Mechanism, Births: id.Fixture.Births,
		},
		Seed:    id.Seed,
		BaseRef: refWire{ID: id.BaseRef.ID, Version: id.BaseRef.Version},
		HeadRef: refWire{ID: id.HeadRef.ID, Version: id.HeadRef.Version},
	}
	body, err := canonicalJSON(block)
	if err != nil {
		// The identity block is a fixed, always-encodable struct.
		panic(err)
	}
	return sha256.Sum256(body)
}

// The canonical request wire. Field order is the request order of spec
// section 6.2; the encoder writes each field by name so an added privileged
// field is impossible without a schema-version edit.
type seamRequestWire struct {
	SchemaVersion  int             `json:"schema_version"`
	Role           string          `json:"role"`
	Seam           string          `json:"seam"`
	PromptTemplate string          `json:"prompt_template"`
	Model          string          `json:"model"`
	Run            runWire         `json:"run"`
	Actor          actorWire       `json:"actor"`
	Observation    observationWire `json:"observation"`
	Strategy       strategyWire    `json:"strategy"`
	Actions        []string        `json:"actions"`
	Sampling       Sampling        `json:"sampling"`
}

type runWire struct {
	Fixture        fixtureWire `json:"fixture"`
	Seed           uint64      `json:"seed"`
	BaseRef        refWire     `json:"base_ref"`
	HeadRef        refWire     `json:"head_ref"`
	IdentityDigest string      `json:"identity_digest"`
}

type fixtureWire struct {
	Patches   int    `json:"patches"`
	R         int    `json:"r"`
	Founders  int    `json:"founders"`
	Mechanism uint32 `json:"mechanism"`
	Births    bool   `json:"births"`
}

type refWire struct {
	ID      string `json:"id"`
	Version uint32 `json:"version"`
}

type actorWire struct {
	ID   uint64 `json:"id"`
	Hour int    `json:"hour"`
}

type slotWire struct {
	Slot  uint64 `json:"slot"`
	Stock int64  `json:"stock"`
}

type observationWire struct {
	Energy             int64      `json:"energy"`
	Hunger             int64      `json:"hunger"`
	BagUnits           int64      `json:"bag_units"`
	Capital            int64      `json:"capital"`
	Wip                int64      `json:"wip"`
	InvestedUnits      int64      `json:"invested_units"`
	LastBuildHour      int64      `json:"last_build_hour"`
	GranaryStock       int64      `json:"granary_stock"`
	LastStoredMealHour int64      `json:"last_stored_meal_hour"`
	HomePatch          uint64     `json:"home_patch"`
	HomeSlots          []slotWire `json:"home_slots"`
	LastDenial         string     `json:"last_denial"`
}

type strategyWire struct {
	Parameters map[string]int64 `json:"parameters"`
}

// EncodeActorSeamRequest is the allow-list encoder. It returns the canonical
// request bytes whose SHA-256 is the recorded request hash. A request outside
// the declared seam or with an undecodable observation is a typed error, never
// a silently narrowed request.
func EncodeActorSeamRequest(req ActorSeamRequest) ([]byte, error) {
	if req.Seam != SeamNaturalClaim {
		return nil, fmt.Errorf("%w: unknown seam %q", ErrInvalidModelRequest, req.Seam)
	}
	if len(req.Actions) == 0 {
		return nil, fmt.Errorf("%w: empty legal action set", ErrInvalidModelRequest)
	}
	obs, err := encodeObservation(req.Observation)
	if err != nil {
		return nil, err
	}
	wire := seamRequestWire{
		SchemaVersion:  ActorSeamSchemaVersion,
		Role:           roleActorDecision,
		Seam:           req.Seam,
		PromptTemplate: NaturalClaimPromptTemplate,
		Model:          DefaultModelID,
		Run: runWire{
			Fixture: fixtureWire{
				Patches: req.Identity.Fixture.Patches, R: req.Identity.Fixture.R,
				Founders: req.Identity.Fixture.Founders, Mechanism: req.Identity.Fixture.Mechanism,
				Births: req.Identity.Fixture.Births,
			},
			Seed:           req.Identity.Seed,
			BaseRef:        refWire{ID: req.Identity.BaseRef.ID, Version: req.Identity.BaseRef.Version},
			HeadRef:        refWire{ID: req.Identity.HeadRef.ID, Version: req.Identity.HeadRef.Version},
			IdentityDigest: formatHash(RunIdentityDigest(req.Identity)),
		},
		Actor:       actorWire{ID: uint64(req.Actor), Hour: req.Hour},
		Observation: obs,
		Strategy:    strategyWire{Parameters: encodeParameters(req.Strategy)},
		Actions:     append([]string(nil), req.Actions...),
		Sampling:    FrozenSampling(),
	}
	return canonicalJSON(wire)
}

func encodeObservation(obs strategy.NaturalObservation) (observationWire, error) {
	energy, err := observationInteger(obs.Energy, "energy")
	if err != nil {
		return observationWire{}, err
	}
	hunger, err := observationInteger(obs.Hunger, "hunger")
	if err != nil {
		return observationWire{}, err
	}
	bag, err := observationInteger(obs.BagUnits, "bag_units")
	if err != nil {
		return observationWire{}, err
	}
	capital, err := observationInteger(obs.Capital, "capital")
	if err != nil {
		return observationWire{}, err
	}
	wip, err := observationInteger(obs.Wip, "wip")
	if err != nil {
		return observationWire{}, err
	}
	invested, err := observationInteger(obs.InvestedUnits, "invested_units")
	if err != nil {
		return observationWire{}, err
	}
	lastBuild, err := observationInteger(obs.LastBuildHour, "last_build_hour")
	if err != nil {
		return observationWire{}, err
	}
	granary, err := observationInteger(obs.GranaryStock, "granary_stock")
	if err != nil {
		return observationWire{}, err
	}
	lastMeal, err := observationInteger(obs.LastStoredMealHour, "last_stored_meal_hour")
	if err != nil {
		return observationWire{}, err
	}
	home, err := obs.BagSource.EntityRef()
	if err != nil || obs.BagSource.Kind() != sim.EntityRefKind {
		return observationWire{}, fmt.Errorf("%w: observation home patch is not an entity ref", ErrInvalidModelRequest)
	}
	slots := make([]slotWire, 0, len(obs.HomeSlots))
	for _, view := range obs.HomeSlots {
		stock, err := observationInteger(view.Stock, "home_slot stock")
		if err != nil {
			return observationWire{}, err
		}
		slots = append(slots, slotWire{Slot: uint64(view.ID), Stock: stock})
	}
	return observationWire{
		Energy: energy, Hunger: hunger, BagUnits: bag, Capital: capital, Wip: wip,
		InvestedUnits: invested, LastBuildHour: lastBuild, GranaryStock: granary,
		LastStoredMealHour: lastMeal, HomePatch: uint64(home), HomeSlots: slots,
		LastDenial: denialLabel(obs.LastDenial),
	}, nil
}

func observationInteger(v sim.Value, field string) (int64, error) {
	n, err := v.Integer()
	if err != nil {
		return 0, fmt.Errorf("%w: observation %s is not an integer", ErrInvalidModelRequest, field)
	}
	return n, nil
}

func encodeParameters(params map[ParamID]int64) map[string]int64 {
	out := make(map[string]int64, len(params))
	for id, value := range params {
		out[string(id)] = value
	}
	return out
}

// denialLabel is the declared wire spelling of the denial guard. It is derived
// from the actor's own observation only.
func denialLabel(denial strategy.NaturalDenial) string {
	switch denial {
	case strategy.NaturalGatherDenied:
		return "gather-denied"
	case strategy.NaturalBuildDenied:
		return "build-denied"
	default:
		return "none"
	}
}
