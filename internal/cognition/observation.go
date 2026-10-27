package cognition

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"agentworld/internal/cognition/protocol"
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
// The declared schema metadata and the Appendix C parameter ids are the
// transport-free constants of internal/cognition/protocol; these aliases keep
// the historical cognition spelling for the encoder and the tests.
const (
	ActorSeamSchemaVersion     = protocol.ActorSeamSchemaVersion
	SeamNaturalClaim           = protocol.SeamNaturalClaim
	NaturalClaimPromptTemplate = protocol.NaturalClaimPromptTemplate
	// ActorSeamSchemaVersionAdmissible and NaturalClaimPromptTemplateAdmissible
	// name the second declared request contract (see protocol).
	ActorSeamSchemaVersionAdmissible     = protocol.ActorSeamSchemaVersionAdmissible
	NaturalClaimPromptTemplateAdmissible = protocol.NaturalClaimPromptTemplateAdmissible
	// ActorSeamSchemaVersionVocabulary and NaturalClaimPromptTemplateVocabulary
	// name the fourth declared request contract (the display vocabulary, v4).
	ActorSeamSchemaVersionVocabulary     = protocol.ActorSeamSchemaVersionVocabulary
	NaturalClaimPromptTemplateVocabulary = protocol.NaturalClaimPromptTemplateVocabulary

	// Role A declared metadata (spec section 6.2); part of run identity.
	roleActorDecision = RoleActorDecision
)

// Declared Appendix C parameter ids (the patchable strategy-side threshold
// vector).
const (
	ParamClaimProvisionEnergyFloor = protocol.ParamClaimProvisionEnergyFloor
	ParamClaimBankEnergyCeiling    = protocol.ParamClaimBankEnergyCeiling
	ParamClaimGatherEnergyFloor    = protocol.ParamClaimGatherEnergyFloor
	ParamBagBuildEnergyFloor       = protocol.ParamBagBuildEnergyFloor
	ParamBagBuildStartEnergyFloor  = protocol.ParamBagBuildStartEnergyFloor
	ParamBagBuildFinishEnergyFloor = protocol.ParamBagBuildFinishEnergyFloor
	ParamBirthFilingEnergyFloor    = protocol.ParamBirthFilingEnergyFloor
)

// FrozenNaturalClaimParameters returns the declared Appendix C frozen
// parameter vector of the actor's bound cascade (the transport-free protocol
// function; the VS1 base policy is natural/7, whose ablation natural/7b shares
// the same thresholds).
func FrozenNaturalClaimParameters(ref strategy.NaturalRef) map[ParamID]int64 {
	return protocol.FrozenNaturalClaimParameters(ref)
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
	// AdmissibleActions is the observation-derived subset of Actions the frozen
	// gates admit this hour (the v2 contract). It is omitted for v1, so v1 request
	// bytes are unchanged.
	AdmissibleActions []string `json:"admissible_actions,omitempty"`
	Sampling          Sampling `json:"sampling"`
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
	return encodeActorSeamRequestFor(req, ClaimContractV1, DefaultModelID)
}

// ClaimContract names one declared Role A request contract.
type ClaimContract string

const (
	// ClaimContractV1 is the declared default contract: the legal action set only.
	ClaimContractV1 ClaimContract = ClaimContract("natural-claim/v1")
	// ClaimContractV2 additionally states the actions the observation admits.
	ClaimContractV2 ClaimContract = ClaimContract("natural-claim/v2")
	// ClaimContractV4 is the display-vocabulary contract: the same observation
	// rendered under names that state the fact the decision depends on, the claim
	// actions named by their display labels, no strategy parameter values, and a
	// response that carries a one-sentence reason. The internal action IDs stay the
	// wire-stable tokens behind the labels (protocol.ClaimActionVocabularyTable).
	ClaimContractV4 ClaimContract = ClaimContract("natural-claim/v4")
)

// ParseClaimContract validates a declared contract spelling; the empty string is
// the declared v1 default.
func ParseClaimContract(s string) (ClaimContract, error) {
	switch ClaimContract(strings.TrimSpace(s)) {
	case "", ClaimContractV1:
		return ClaimContractV1, nil
	case ClaimContractV2:
		return ClaimContractV2, nil
	case ClaimContractV4:
		return ClaimContractV4, nil
	default:
		return "", fmt.Errorf("%w: unknown claim contract %q", ErrInvalidModelRequest, s)
	}
}

// ResponseSchemaVersion is the schema version a response to this contract must
// declare.
func (c ClaimContract) ResponseSchemaVersion() int {
	switch c {
	case ClaimContractV4:
		return ActorSeamSchemaVersionVocabulary
	case ClaimContractV2:
		return ActorSeamSchemaVersionAdmissible
	default:
		return ActorSeamSchemaVersion
	}
}

// EncodeActorSeamRequestFor is the allow-list encoder for one declared contract.
// It returns the canonical request bytes whose SHA-256 is the recorded request
// hash. A request outside the declared seam or with an undecodable observation is
// a typed error, never a silently narrowed request.
func EncodeActorSeamRequestFor(req ActorSeamRequest, contract ClaimContract) ([]byte, error) {
	return encodeActorSeamRequestFor(req, contract, DefaultModelID)
}

// encodeActorSeamRequestFor is the declared-contract encoder with an explicit
// application model. The model is written into the canonical request's "model"
// field, so a run on a non-default model records that model in every request it
// sends (and in the request hash). An empty model is the declared default.
func encodeActorSeamRequestFor(req ActorSeamRequest, contract ClaimContract, model string) ([]byte, error) {
	if model == "" {
		model = DefaultModelID
	}
	if contract == ClaimContractV4 {
		return encodeActorSeamRequestV4(req, model)
	}
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
	schemaVersion := contract.ResponseSchemaVersion()
	promptTemplate := NaturalClaimPromptTemplate
	var admissible []string
	if contract == ClaimContractV2 {
		promptTemplate = NaturalClaimPromptTemplateAdmissible
		admissible, err = admissibleActions(req)
		if err != nil {
			return nil, err
		}
	}
	wire := seamRequestWire{
		SchemaVersion:  schemaVersion,
		Role:           roleActorDecision,
		Seam:           req.Seam,
		PromptTemplate: promptTemplate,
		Model:          model,
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
		Actor:             actorWire{ID: uint64(req.Actor), Hour: req.Hour},
		Observation:       obs,
		Strategy:          strategyWire{Parameters: encodeParameters(req.Strategy)},
		Actions:           append([]string(nil), req.Actions...),
		AdmissibleActions: admissible,
		Sampling:          FrozenSampling(),
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

// The natural-claim/v4 canonical request (ticket 36). It carries the same facts
// as the v1 wire under names that state what the actor can decide about, and it
// carries no row that no decision reads. Field order is the request order and
// the encoder writes every field by name, so an added privileged field is still
// impossible without editing this file and the schema version.
//
// What v4 changes against v1, each row checked against the code that reads it
// (the declared redundancy audit is in .scratch/36-harness-vocabulary/spec.md):
//
//   - hunger is gone. NaturalBasalFor decides death from energy alone
//     (starved := a.Body.Energy == 0), the frozen cascade validates hunger and
//     then discards it (`_, hungerOK := capacityInteger(...)`), and no claim
//     ladder reads it. It stays a world row and an allocator feature; it is not a
//     fact this decision can act on.
//   - invested_units is gone: a conservation ledger row that neither the frozen
//     cascade nor the feature set reads.
//   - home_patch is gone: an actor has exactly one home patch and no action can
//     address another, so its id decides nothing.
//   - identity_digest is gone: it is a hash of the run block the same request
//     already carries, so it is the one field derivable from the others.
//   - home_slots becomes a count of the units waiting to be picked up. The
//     per-slot ids are unusable (gather always takes the lowest-numbered stocked
//     slot and a model can never name one) and the array's sum is that count.
//   - last_stored_meal_hour becomes the affordance it gates (one granary draw per
//     hour) and last_build_hour the build fact it gates (one build per hour):
//     both are timestamps no decision reads, and both gates' outcomes are what
//     the actor can act on.
//   - the strategy parameter vector is gone: those seven values are policy
//     constants of the actor's bound cascade, not state. The recorded head ref
//     identifies them (frozen base vector plus the generation's deltas), so no
//     provenance is lost.
//
// The claim actions are written as their display labels; the internal IDs remain
// the wire-stable tokens on every other surface (protocol.ClaimActionVocabularyTable).
type seamRequestWireV4 struct {
	SchemaVersion  int               `json:"schema_version"`
	Role           string            `json:"role"`
	Seam           string            `json:"seam"`
	PromptTemplate string            `json:"prompt_template"`
	Model          string            `json:"model"`
	Run            runWireV4         `json:"run"`
	Actor          actorWire         `json:"actor"`
	Observation    observationWireV4 `json:"observation"`
	Actions        []string          `json:"actions"`
	Sampling       Sampling          `json:"sampling"`
}

// runWireV4 is the run identity block without its own digest: the digest is
// recomputable from these fields (RunIdentityDigest), so stating both would be
// two homes for one fact.
type runWireV4 struct {
	Fixture fixtureWire `json:"fixture"`
	Seed    uint64      `json:"seed"`
	BaseRef refWire     `json:"base_ref"`
	HeadRef refWire     `json:"head_ref"`
}

// observationWireV4 is the actor's own state under decision-facing names. Every
// field below is read by the frozen cascade, by a claim-admission gate, or by the
// action the actor is choosing; a row no decision reads is not here.
type observationWireV4 struct {
	Energy                   int64  `json:"energy"`
	BagFoodUnits             int64  `json:"bag_food_units"`
	Capital                  int64  `json:"capital"`
	BuildPointInProgress     int64  `json:"build_point_in_progress"`
	BuildDoneThisHour        bool   `json:"build_done_this_hour"`
	GranaryFoodUnits         int64  `json:"granary_food_units"`
	GranaryDrawUsedThisHour  bool   `json:"granary_draw_used_this_hour"`
	FoodUnitsWaitingToPickUp int64  `json:"food_units_waiting_to_pick_up"`
	LastRefusedAction        string `json:"last_refused_action"`
}

// encodeActorSeamRequestV4 is the allow-list encoder of the display-vocabulary
// contract. It reads the same observation as the v1 encoder and writes it under
// the v4 names; an action ID with no declared display label is a typed error
// rather than a request the model could not answer.
func encodeActorSeamRequestV4(req ActorSeamRequest, model string) ([]byte, error) {
	if model == "" {
		model = DefaultModelID
	}
	if req.Seam != SeamNaturalClaim {
		return nil, fmt.Errorf("%w: unknown seam %q", ErrInvalidModelRequest, req.Seam)
	}
	if len(req.Actions) == 0 {
		return nil, fmt.Errorf("%w: empty legal action set", ErrInvalidModelRequest)
	}
	obs, err := encodeObservationV4(req.Observation, req.Hour)
	if err != nil {
		return nil, err
	}
	actions := make([]string, 0, len(req.Actions))
	for _, id := range req.Actions {
		label, ok := protocol.ClaimActionLabel(id)
		if !ok {
			return nil, fmt.Errorf("%w: claim action %q has no declared display label", ErrInvalidModelRequest, id)
		}
		actions = append(actions, label)
	}
	wire := seamRequestWireV4{
		SchemaVersion:  ActorSeamSchemaVersionVocabulary,
		Role:           roleActorDecision,
		Seam:           req.Seam,
		PromptTemplate: NaturalClaimPromptTemplateVocabulary,
		Model:          model,
		Run: runWireV4{
			Fixture: fixtureWire{
				Patches: req.Identity.Fixture.Patches, R: req.Identity.Fixture.R,
				Founders: req.Identity.Fixture.Founders, Mechanism: req.Identity.Fixture.Mechanism,
				Births: req.Identity.Fixture.Births,
			},
			Seed:    req.Identity.Seed,
			BaseRef: refWire{ID: req.Identity.BaseRef.ID, Version: req.Identity.BaseRef.Version},
			HeadRef: refWire{ID: req.Identity.HeadRef.ID, Version: req.Identity.HeadRef.Version},
		},
		Actor:       actorWire{ID: uint64(req.Actor), Hour: req.Hour},
		Observation: obs,
		Actions:     actions,
		Sampling:    FrozenSampling(),
	}
	return canonicalJSON(wire)
}

// encodeObservationV4 renders the observation under the v4 names, deriving the
// two gate booleans from the timestamps they gate.
func encodeObservationV4(obs strategy.NaturalObservation, hour int) (observationWireV4, error) {
	energy, err := observationInteger(obs.Energy, "energy")
	if err != nil {
		return observationWireV4{}, err
	}
	bag, err := observationInteger(obs.BagUnits, "bag_food_units")
	if err != nil {
		return observationWireV4{}, err
	}
	capital, err := observationInteger(obs.Capital, "capital")
	if err != nil {
		return observationWireV4{}, err
	}
	wip, err := observationInteger(obs.Wip, "build_point_in_progress")
	if err != nil {
		return observationWireV4{}, err
	}
	lastBuild, err := observationInteger(obs.LastBuildHour, "last_build_hour")
	if err != nil {
		return observationWireV4{}, err
	}
	granary, err := observationInteger(obs.GranaryStock, "granary_food_units")
	if err != nil {
		return observationWireV4{}, err
	}
	lastMeal, err := observationInteger(obs.LastStoredMealHour, "last_stored_meal_hour")
	if err != nil {
		return observationWireV4{}, err
	}
	waiting := int64(0)
	for _, view := range obs.HomeSlots {
		stock, err := observationInteger(view.Stock, "home_slot stock")
		if err != nil {
			return observationWireV4{}, err
		}
		if stock >= 1 {
			waiting += stock
		}
	}
	return observationWireV4{
		Energy:                   energy,
		BagFoodUnits:             bag,
		Capital:                  capital,
		BuildPointInProgress:     wip,
		BuildDoneThisHour:        lastBuild >= int64(hour),
		GranaryFoodUnits:         granary,
		GranaryDrawUsedThisHour:  lastMeal >= int64(hour),
		FoodUnitsWaitingToPickUp: waiting,
		LastRefusedAction:        protocol.RefusedActionLabel(obs.LastDenial),
	}, nil
}
