package cognition

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// Role B — the in-run designer of behaviour logic (spec section 5). The
// controller owns the declared modification policy: the audit-hour trigger
// predicate over a trajectory evidence packet, exactly one proposal per audit
// hour, the deterministic validate → sandbox → apply lifecycle at a quiescent
// boundary, the measured window, and the mandatory rollback. It never touches
// the world directly: the world forking and rebinding are injected through the
// SandboxBranch and StrategyApplier interfaces, so this package never imports
// internal/world (EC-6).

var (
	ErrRoleB = errors.New("role B lifecycle")
	// ErrRoleBSandbox is the construction error for a controller without a
	// sandbox branch.
	ErrRoleBSandbox = errors.New("role B requires a sandbox branch")
)

// roleBValidatorIdentity is the declared validator identity recorded on every
// patch verdict (spec 5.5): the deterministic admissibility/sandbox validator.
const roleBValidatorIdentity = "deterministic-patch-validator/v1"

// The declared audit-hour set (spec section 5.1): a pulse boundary every 24
// hours over the 168-hour horizon.
var VS1AuditHours = []int{24, 48, 72, 96, 120, 144, 168}

// PatchProposer is the Role B half of the model-call protocol.
type PatchProposer interface {
	ProposeStrategyPatch(ctx context.Context, req protocol.PatchRequest) (protocol.PatchProposal, error)
}

// StrategyApplier is the world-side rebind seam. ApplyGeneration atomically
// rebinds the whole roster to the generation and returns the affected-actor
// digest; RollbackGeneration rebinds the roster to the parent generation, or
// clears the active generation when parent is nil (the frozen base). Both are
// quiescent-boundary operations.
type StrategyApplier interface {
	RosterActors(ctx context.Context) ([]sim.EntityID, error)
	ApplyGeneration(ctx context.Context, gen strategy.NaturalCognitiveGeneration, actors []sim.EntityID) ([32]byte, error)
	RollbackGeneration(ctx context.Context, parent *strategy.NaturalCognitiveGeneration, actors []sim.EntityID) error
}

// RoleBControllerConfig wires the lifecycle.
type RoleBControllerConfig struct {
	Ledger   *Ledger
	Proposer PatchProposer
	Sandbox  SandboxBranch
	Applier  StrategyApplier
	// Operator is the recorded proposer identity: the model id plus the call
	// id, or the reserved "harness".
	Operator string
	// MeasurementWindowHours is the declared measured window (spec section
	// 5.3 step 5): the applied generation is measured at the next audit hour.
	MeasurementWindowHours int
}

// pendingMeasurement is the applied generation awaiting its next-audit-hour
// measurement (and mandatory rollback re-check).
type pendingMeasurement struct {
	generation     strategy.NaturalCognitiveGeneration
	parent         strategy.NaturalRef
	affectedDigest [32]byte
	affectedActors []sim.EntityID
	applyHour      int
	indicator      int64
	proposer       string
}

// RoleBController is the stateful Role B lifecycle.
type RoleBController struct {
	ledger      *Ledger
	proposer    PatchProposer
	sandbox     SandboxBranch
	applier     StrategyApplier
	operator    string
	measurement int

	head        PatchParent
	started     bool
	lineage     []strategy.NaturalRef
	generations map[strategy.NaturalRef]strategy.NaturalCognitiveGeneration
	pending     *pendingMeasurement
}

// NewRoleBController validates and builds the controller.
func NewRoleBController(cfg RoleBControllerConfig) (*RoleBController, error) {
	if cfg.Ledger == nil || cfg.Proposer == nil || cfg.Applier == nil || cfg.Sandbox == nil {
		return nil, fmt.Errorf("%w: ledger, proposer, applier, and sandbox are required", ErrRoleB)
	}
	if cfg.Operator == "" {
		return nil, fmt.Errorf("%w: operator identity is required", ErrRoleB)
	}
	measurement := cfg.MeasurementWindowHours
	if measurement == 0 {
		measurement = 24
	}
	if measurement < 1 {
		return nil, fmt.Errorf("%w: measurement window %d", ErrRoleB, measurement)
	}
	return &RoleBController{
		ledger: cfg.Ledger, proposer: cfg.Proposer, sandbox: cfg.Sandbox, applier: cfg.Applier,
		operator: cfg.Operator, measurement: measurement,
		generations: make(map[strategy.NaturalRef]strategy.NaturalCognitiveGeneration),
	}, nil
}

// Start pins the head to a frozen cognitive base ref.
func (c *RoleBController) Start(base strategy.NaturalRef) error {
	parent, err := PatchParentFromBase(base)
	if err != nil {
		return err
	}
	c.head = parent
	c.started = true
	return nil
}

// Head returns the current parent (head) identity.
func (c *RoleBController) Head() PatchParent { return c.head }

// proposerIdentity is the recorded proposer identity (spec 5.5): the operator
// identity plus the used attempt's call id.
func (c *RoleBController) proposerIdentity(callRef uint64) string {
	return fmt.Sprintf("%s#%d", c.operator, callRef)
}

// headGeneration returns the active head's generation, or nil when the head is
// a frozen base. The sandbox parent branch runs under it.
func (c *RoleBController) headGeneration() *strategy.NaturalCognitiveGeneration {
	return c.generationOf(c.head.Ref)
}

// generationOf resolves a ref to its registered generation, or nil for a
// frozen base (a ref the controller never registered).
func (c *RoleBController) generationOf(ref strategy.NaturalRef) *strategy.NaturalCognitiveGeneration {
	if gen, ok := c.generations[ref]; ok {
		g := gen
		return &g
	}
	return nil
}

// TriggerPredicate is the declared in-run kick-in (spec section 5.1). It fires
// on any one of the three evidence thresholds; the returned name is recorded.
func TriggerPredicate(evidence protocol.TrajectoryEvidence) (string, bool) {
	switch {
	case evidence.BirthsTotal == 0:
		return "no-birth", true
	case evidence.BuildsCommitted == 0:
		return "no-build", true
	case evidence.MaxCapital < 2:
		return "capital-below-birth-gate", true
	}
	return "", false
}

// TrajectoryEvidenceDigest is SHA-256 over the canonical packet.
func TrajectoryEvidenceDigest(evidence protocol.TrajectoryEvidence) [32]byte {
	body, err := canonicalJSON(evidence)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(body)
}

// Audit runs the Role B lifecycle at one audit hour. It first resolves any
// pending measurement (recording patch-measure and, when the head no longer
// improves, the mandatory patch-rollback), then evaluates the trigger
// predicate and, if it fires, runs propose → validate → sandbox → apply. Every
// step is recorded; a rejection is recorded with its typed reason and leaves
// the world untouched.
func (c *RoleBController) Audit(ctx context.Context, hour int, evidence protocol.TrajectoryEvidence) ([]InterventionRecord, error) {
	if !c.started {
		return nil, fmt.Errorf("%w: controller not started", ErrRoleB)
	}
	var records []InterventionRecord

	if c.pending != nil {
		record, err := c.measureAndMaybeRollback(ctx, hour, evidence)
		if err != nil {
			return records, err
		}
		records = append(records, record...)
	}

	predicate, fired := TriggerPredicate(evidence)
	evidenceDigest := TrajectoryEvidenceDigest(evidence)
	if !fired {
		record, err := c.appendIntervention(InterventionRecord{
			Kind:   InterventionNoTriggerAudit,
			Cause:  InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "none", EvidenceDigest: formatHash(evidenceDigest)},
			Cost:   InterventionCost{},
			Result: InterventionResult{Status: "no-effect", Reason: "no-trigger-audit", IndicatorDeltas: []IndicatorDelta{}},
		})
		if err != nil {
			return records, err
		}
		return append(records, record), nil
	}

	actors, err := c.applier.RosterActors(ctx)
	if err != nil {
		return records, err
	}
	proposal, err := c.proposer.ProposeStrategyPatch(ctx, protocol.PatchRequest{
		Role: "strategy-patch", AuditHour: hour, Evidence: evidence,
		ParentRef: c.head.Ref, Parameters: patchParameterSpecs(),
	})
	if err != nil {
		return records, err
	}
	proposeRecord, err := c.appendIntervention(InterventionRecord{
		Kind:     InterventionPatchPropose,
		Cause:    InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: predicate, EvidenceDigest: formatHash(evidenceDigest)},
		CallRefs: callRefRefs(proposal.CallRef, proposal.CallAttempt),
		Cost:     InterventionCost{Calls: 1},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(c.head.Ref), ChildRef: NaturalRefRecordOf(proposal.ParentRef),
			ParentFingerprint: formatHash(c.head.Fingerprint), Deltas: proposal.Deltas, Intent: proposal.Intent,
			Scope: "role-b", Proposer: c.proposerIdentity(proposal.CallRef),
		},
		Result: InterventionResult{Status: "pending", Reason: "proposed", IndicatorDeltas: []IndicatorDelta{}},
	})
	if err != nil {
		return records, err
	}
	records = append(records, proposeRecord)

	child, validation, err := ValidatePatchProposal(proposal, c.head)
	validateRecord := InterventionRecord{
		Kind:  InterventionPatchValidate,
		Cause: InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: predicate, EvidenceDigest: formatHash(evidenceDigest)},
		Cost:  InterventionCost{},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(c.head.Ref), ChildRef: NaturalRefRecordOf(proposal.ParentRef),
			ParentFingerprint: formatHash(c.head.Fingerprint), Deltas: proposal.Deltas, Intent: proposal.Intent, Scope: "role-b",
			Proposer: c.proposerIdentity(proposal.CallRef),
		},
		Validation: &validation,
	}
	if validateRecord.Validation != nil {
		validateRecord.Validation.Validator = roleBValidatorIdentity
	}
	if err != nil {
		validateRecord.Result = InterventionResult{Status: "rejected", Reason: validation.Reason, IndicatorDeltas: []IndicatorDelta{}}
		record, appendErr := c.appendIntervention(validateRecord)
		if appendErr != nil {
			return records, appendErr
		}
		return append(records, record), nil
	}
	childRef := NaturalRefRecordOf(child.Ref)
	childFingerprint, err := strategy.NaturalCognitiveGenerationFingerprint(child)
	if err != nil {
		return records, err
	}
	childWire, err := strategy.EncodeNaturalCognitiveGeneration(child)
	if err != nil {
		return records, err
	}
	validateRecord.Patch.ChildRef = childRef
	validateRecord.Patch.ChildFingerprint = formatHash(childFingerprint)
	validateRecord.Patch.PatchDigest = formatHash(sha256.Sum256(childWire))

	verdict, err := RunSandbox(ctx, c.sandbox, c.headGeneration(), child, c.measurement)
	if err != nil {
		return records, err
	}
	if !verdict.Admitted {
		validateRecord.Result = InterventionResult{Status: "rejected", Reason: "sandbox:" + verdict.Reason, IndicatorDeltas: []IndicatorDelta{}}
		validateRecord.Validation.SandboxBranchDigest = formatHash(verdict.Patched.BranchDigest)
		validateRecord.Validation.WindowHours = verdict.WindowHours
		record, appendErr := c.appendIntervention(validateRecord)
		if appendErr != nil {
			return records, appendErr
		}
		return append(records, record), nil
	}
	validateRecord.Result = InterventionResult{Status: "validated", Reason: verdict.Reason, IndicatorDeltas: []IndicatorDelta{}}
	validateRecord.Validation.SandboxBranchDigest = formatHash(verdict.Patched.BranchDigest)
	validateRecord.Validation.WindowHours = verdict.WindowHours
	validateRecord.Patch.AffectedActorsDigest = formatHash(affectedDigest(actors))
	record, err := c.appendIntervention(validateRecord)
	if err != nil {
		return records, err
	}
	records = append(records, record)

	affected, err := c.applier.ApplyGeneration(ctx, child, actors)
	if err != nil {
		return records, err
	}
	applyRecord, err := c.appendIntervention(InterventionRecord{
		Kind:  InterventionPatchApply,
		Cause: InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: predicate, EvidenceDigest: formatHash(evidenceDigest)},
		Cost:  InterventionCost{},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(c.head.Ref), ChildRef: childRef,
			ParentFingerprint: formatHash(c.head.Fingerprint), ChildFingerprint: formatHash(childFingerprint),
			Deltas: proposal.Deltas, Intent: proposal.Intent, PatchDigest: formatHash(sha256.Sum256(childWire)),
			Scope: "role-b", Proposer: c.proposerIdentity(proposal.CallRef), AffectedActorsDigest: formatHash(affected),
		},
		Result: InterventionResult{Status: "applied", WindowHours: c.measurement, IndicatorDeltas: []IndicatorDelta{}},
	})
	if err != nil {
		return records, err
	}
	records = append(records, applyRecord)

	parent, err := PatchParentFromGeneration(child)
	if err != nil {
		return records, err
	}
	c.generations[child.Ref] = child
	c.lineage = append(c.lineage, child.Ref)
	c.head = parent
	c.pending = &pendingMeasurement{
		generation: child, parent: proposal.ParentRef, affectedDigest: affected, affectedActors: actors,
		applyHour: hour, indicator: TriggerIndicator(evidence), proposer: c.proposerIdentity(proposal.CallRef),
	}
	return records, nil
}

// AuditExtinct records an audit hour with no living roster (spec 14.1
// reduction 4). There is nothing to bind or measure, so it abandons a pending
// measurement with a typed `unavailable` result (never a fabricated value) and
// records the zero-cost no-effect audit. It never touches the world.
func (c *RoleBController) AuditExtinct(hour int) ([]InterventionRecord, error) {
	if !c.started {
		return nil, fmt.Errorf("%w: controller not started", ErrRoleB)
	}
	var records []InterventionRecord
	if c.pending != nil {
		pending := c.pending
		record, err := c.appendIntervention(InterventionRecord{
			Kind: InterventionPatchMeasure,
			Cause: InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "roster-extinct",
				EvidenceDigest: formatHash(TrajectoryEvidenceDigest(protocol.TrajectoryEvidence{}))},
			Cost: InterventionCost{},
			Patch: &PatchRecord{
				ParentRef: NaturalRefRecordOf(pending.parent), ChildRef: NaturalRefRecordOf(pending.generation.Ref),
				Scope: "role-b", Proposer: pending.proposer, AffectedActorsDigest: formatHash(pending.affectedDigest),
			},
			Result: InterventionResult{Status: "unavailable", Reason: "roster-extinct", IndicatorDeltas: []IndicatorDelta{}},
		})
		if err != nil {
			return records, err
		}
		records = append(records, record)
		c.pending = nil
	}
	noEffect, err := c.appendIntervention(InterventionRecord{
		Kind:   InterventionNoTriggerAudit,
		Cause:  InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "roster-extinct"},
		Cost:   InterventionCost{},
		Result: InterventionResult{Status: "no-effect", Reason: "roster-extinct", IndicatorDeltas: []IndicatorDelta{}},
	})
	if err != nil {
		return records, err
	}
	return append(records, noEffect), nil
}

// measureAndMaybeRollback resolves the pending measurement: it records the
// measured window, then re-runs the sandbox on the current head versus its
// parent; if the head no longer improves the trigger indicator, it rebinds the
// roster to the parent and records the rollback. A rollback never deletes the
// retained generation or its lineage.
func (c *RoleBController) measureAndMaybeRollback(ctx context.Context, hour int, evidence protocol.TrajectoryEvidence) ([]InterventionRecord, error) {
	pending := c.pending
	var records []InterventionRecord
	deltas := []IndicatorDelta{{Indicator: "trigger", Delta: TriggerIndicator(evidence) - pending.indicator}}
	measureRecord, err := c.appendIntervention(InterventionRecord{
		Kind: InterventionPatchMeasure,
		Cause: InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "measured",
			EvidenceDigest: formatHash(TrajectoryEvidenceDigest(evidence))},
		Cost: InterventionCost{},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(pending.parent), ChildRef: NaturalRefRecordOf(pending.generation.Ref),
			Scope: "role-b", Proposer: pending.proposer, AffectedActorsDigest: formatHash(pending.affectedDigest),
		},
		Result: InterventionResult{Status: "measured", WindowHours: hour - pending.applyHour, IndicatorDeltas: deltas},
	})
	if err != nil {
		return records, err
	}
	records = append(records, measureRecord)
	c.pending = nil

	verdict, err := RunSandbox(ctx, c.sandbox, c.generationOf(pending.parent), pending.generation, c.measurement)
	if err != nil {
		return records, err
	}
	if verdict.Admitted {
		return records, nil
	}
	var parentGen *strategy.NaturalCognitiveGeneration
	if g, ok := c.generations[pending.parent]; ok {
		parentGen = &g
	}
	if err := c.applier.RollbackGeneration(ctx, parentGen, pending.affectedActors); err != nil {
		return records, err
	}
	if pending.parent == c.head.Base {
		base, err := PatchParentFromBase(pending.parent)
		if err != nil {
			return records, err
		}
		c.head = base
	} else if parentGen, ok := c.generations[pending.parent]; ok {
		parent, err := PatchParentFromGeneration(parentGen)
		if err != nil {
			return records, err
		}
		c.head = parent
	}
	rollbackRecord, err := c.appendIntervention(InterventionRecord{
		Kind: InterventionPatchRollback,
		Cause: InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "rollback",
			EvidenceDigest: formatHash(TrajectoryEvidenceDigest(evidence))},
		Cost: InterventionCost{},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(pending.parent), ChildRef: NaturalRefRecordOf(pending.generation.Ref),
			Scope: "role-b", Proposer: pending.proposer, AffectedActorsDigest: formatHash(pending.affectedDigest),
		},
		Validation: &ValidationRecord{Verdict: "rollback", Reason: verdict.Reason, Validator: roleBValidatorIdentity, SandboxBranchDigest: formatHash(verdict.Patched.BranchDigest), WindowHours: verdict.WindowHours},
		Result:     InterventionResult{Status: "rolled-back", Reason: verdict.Reason, IndicatorDeltas: deltas},
	})
	if err != nil {
		return records, err
	}
	return append(records, rollbackRecord), nil
}

func (c *RoleBController) appendIntervention(record InterventionRecord) (InterventionRecord, error) {
	return c.ledger.AppendIntervention(record)
}

// TriggerIndicator is the declared value the sandbox/measurement compare: the
// binding problem's own reading.
func TriggerIndicator(evidence protocol.TrajectoryEvidence) int64 {
	switch {
	case evidence.BirthsTotal == 0:
		return evidence.MaxCapital
	case evidence.BuildsCommitted == 0:
		return evidence.BuildsCommitted
	default:
		return evidence.MaxCapital
	}
}

// affectedDigest is the sorted-actor digest recorded on an apply.
func affectedDigest(actors []sim.EntityID) [32]byte {
	sorted := append([]sim.EntityID(nil), actors...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	body, _ := canonicalJSON(sorted)
	return sha256.Sum256(body)
}

// patchParameterSpecs projects the strategy contract into the request schema.
func patchParameterSpecs() []protocol.ParameterSpec {
	contract := strategy.CognitiveParameterContract()
	out := make([]protocol.ParameterSpec, 0, len(contract))
	for _, row := range contract {
		out = append(out, protocol.ParameterSpec{
			ID: protocol.ParamID(row.ID), Meaning: row.Meaning, DecisionPoint: row.DecisionPoint,
			ParentValue: row.ParentValue, DomainMin: row.DomainMin, DomainMax: row.DomainMax,
		})
	}
	return out
}

// patchRefContent is the wire form of a natural ref inside a request or a
// proposal.
type patchRefContent struct {
	ID      string `json:"id"`
	Version uint32 `json:"version"`
}

// patchRequestWire is the canonical Role B request (spec section 6.2). It
// carries only the declared aggregate evidence packet, the parameter contract,
// and the head ref: no actor-private state.
type patchRequestWire struct {
	SchemaVersion  int                         `json:"schema_version"`
	Role           string                      `json:"role"`
	PromptTemplate string                      `json:"prompt_template"`
	Model          string                      `json:"model"`
	AuditHour      int                         `json:"audit_hour"`
	Run            runWire                     `json:"run"`
	Evidence       protocol.TrajectoryEvidence `json:"evidence"`
	ParentRef      patchRefContent             `json:"parent_ref"`
	Parameters     []protocol.ParameterSpec    `json:"parameters"`
	Sampling       Sampling                    `json:"sampling"`
}

// StrategyPatchPromptTemplate is the versioned Role B prompt template.
const StrategyPatchPromptTemplate = "natural-strategy-patch/v1"

// EncodeStrategyPatchRequest is the allow-list Role B encoder: every field is
// written by name, so no privileged state can reach the request without a
// schema-version edit.
func EncodeStrategyPatchRequest(req protocol.PatchRequest) ([]byte, error) {
	return encodeStrategyPatchRequest(req, DefaultModelID)
}

// encodeStrategyPatchRequest is the Role B encoder with an explicit application
// model, written into the canonical request's "model" field. An empty model is
// the declared default.
func encodeStrategyPatchRequest(req protocol.PatchRequest, model string) ([]byte, error) {
	if model == "" {
		model = DefaultModelID
	}
	if req.Role != RoleStrategyPatch {
		return nil, fmt.Errorf("%w: role %q", ErrInvalidModelRequest, req.Role)
	}
	if len(req.Parameters) == 0 {
		return nil, fmt.Errorf("%w: empty parameter contract", ErrInvalidModelRequest)
	}
	wire := patchRequestWire{
		SchemaVersion:  ActorSeamSchemaVersion,
		Role:           RoleStrategyPatch,
		PromptTemplate: StrategyPatchPromptTemplate,
		Model:          model,
		AuditHour:      req.AuditHour,
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
		Evidence:   req.Evidence,
		ParentRef:  patchRefContent{ID: req.ParentRef.ID, Version: req.ParentRef.Version},
		Parameters: append([]protocol.ParameterSpec(nil), req.Parameters...),
		Sampling:   FrozenSampling(),
	}
	return canonicalJSON(wire)
}

// patchProposalContent is the declared strategy-patch response schema (spec
// section 6.3).
type patchProposalContent struct {
	SchemaVersion *int                      `json:"schema_version"`
	Propose       bool                      `json:"propose"`
	ParentRef     patchRefContent           `json:"parent_ref"`
	Deltas        []protocol.ParameterDelta `json:"deltas"`
	Intent        string                    `json:"intent"`
}

// RoleBProposer is the concrete recorded Role B proposer: one recorded call
// per proposal, replayable from the ledger.
type RoleBProposer struct {
	recorder          *Recorder
	endpoint          string
	credentialPresent bool
	model             string
}

// NewRoleBProposer builds the recorded proposer.
func NewRoleBProposer(rec *Recorder, endpoint string, credentialPresent bool) (*RoleBProposer, error) {
	return NewRoleBProposerForModel(rec, endpoint, credentialPresent, DefaultModelID)
}

// NewRoleBProposerForModel is NewRoleBProposer under an explicit application
// model; the model is written into the canonical Role B request. An empty model
// is the declared default.
func NewRoleBProposerForModel(rec *Recorder, endpoint string, credentialPresent bool, model string) (*RoleBProposer, error) {
	if rec == nil || endpoint == "" {
		return nil, fmt.Errorf("%w: recorder and endpoint are required", ErrRoleB)
	}
	if model == "" {
		model = DefaultModelID
	}
	return &RoleBProposer{recorder: rec, endpoint: endpoint, credentialPresent: credentialPresent, model: model}, nil
}

// ProposeStrategyPatch performs exactly one recorded call and decodes the
// proposal. A malformed completion is a typed error; its record is persisted.
func (p *RoleBProposer) ProposeStrategyPatch(ctx context.Context, req protocol.PatchRequest) (protocol.PatchProposal, error) {
	canonical, err := encodeStrategyPatchRequest(req, p.model)
	if err != nil {
		return protocol.PatchProposal{}, err
	}
	call, err := p.recorder.Call(ctx, CallRequest{
		Role: RoleStrategyPatch, Hour: req.AuditHour, Model: p.model,
		Endpoint: p.endpoint, CredentialPresent: p.credentialPresent,
		Sampling: FrozenSampling(), Request: canonical,
	})
	if err != nil {
		return protocol.PatchProposal{}, err
	}
	proposal, err := DecodeStrategyPatchContent(call.Content, req, call.Seq)
	if err != nil {
		return protocol.PatchProposal{}, err
	}
	proposal.CallAttempt = call.Attempt
	return proposal, nil
}

// DecodeStrategyPatchContent decodes and validates the response schema. A
// malformed completion is a typed error and never a silently empty proposal.
func DecodeStrategyPatchContent(content []byte, req protocol.PatchRequest, callRef uint64) (protocol.PatchProposal, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	var body patchProposalContent
	if err := dec.Decode(&body); err != nil {
		return protocol.PatchProposal{}, fmt.Errorf("%w: %v", ErrRoleB, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return protocol.PatchProposal{}, fmt.Errorf("%w: trailing content", ErrRoleB)
	}
	if body.SchemaVersion == nil || *body.SchemaVersion != ActorSeamSchemaVersion || len(body.Intent) > 512 {
		return protocol.PatchProposal{}, fmt.Errorf("%w: malformed strategy-patch schema", ErrRoleB)
	}
	return protocol.PatchProposal{
		Propose:   body.Propose,
		ParentRef: strategy.NaturalRef{ID: body.ParentRef.ID, Version: body.ParentRef.Version},
		Deltas:    body.Deltas, Intent: body.Intent, CallRef: callRef,
	}, nil
}
