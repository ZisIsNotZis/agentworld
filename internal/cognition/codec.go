package cognition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The recorded-call ledger and the intervention ledger share one canonical,
// hash-chained wire. Every record is a single canonical JSON object with the
// hash as its last field; the record hash is SHA-256 over the same object with
// the hash field absent. Field order is the declaration order below, so the
// bytes are deterministic across processes.
var (
	ErrInvalidRecord      = errors.New("invalid cognition ledger record")
	ErrRecordHashMismatch = errors.New("cognition ledger record hash mismatch")
	ErrNonCanonicalRecord = errors.New("non-canonical cognition ledger record")
)

const callRecordKind = "call"

// CallRecord is one physical model attempt (Appendix B). ResponseDigest and
// ResponseLength carry the oversized-body digest and length; HTTPStatus records
// the transport's code so retry classification is auditable.
type CallRecord struct {
	Seq               uint64   `json:"seq"`
	Kind              string   `json:"kind"`
	Role              string   `json:"role"`
	RequestHash       string   `json:"request_hash"`
	RequestBytes      []byte   `json:"request_bytes"`
	ResponseBody      []byte   `json:"response_body,omitempty"`
	Content           []byte   `json:"content,omitempty"`
	ModelID           string   `json:"model_id"`
	Endpoint          string   `json:"endpoint"`
	CredentialPresent bool     `json:"credential_present"`
	Sampling          Sampling `json:"sampling"`
	Usage             Usage    `json:"usage"`
	CostMicros        int64    `json:"cost_micros"`
	PriceVersion      string   `json:"price_version"`
	// OutputMode records the requested JSON output mode of an attempt when the
	// transport declares one (the real gateway: json_object or plain text).
	// Hermetic transports declare none, so this field stays empty and their
	// recorded bytes are unchanged.
	OutputMode     string     `json:"output_mode,omitempty"`
	Attempt        int        `json:"attempt"`
	Status         CallStatus `json:"status"`
	HTTPStatus     int        `json:"http_status,omitempty"`
	ResponseDigest string     `json:"response_digest,omitempty"`
	ResponseLength int        `json:"response_length,omitempty"`
	// AppliedReasoningEffort and AppliedMaxOutputTokens record the wire settings
	// an attempt actually carried when the transport applied settings other than
	// the declared Sampling instance above (the responses wire bounds reasoning
	// plus text with one token field and takes a reasoning effort). Empty and
	// zero mean the declared sampling instance was sent unchanged, so hermetic
	// records and wired records that apply the declared state keep their bytes.
	AppliedReasoningEffort string `json:"applied_reasoning_effort,omitempty"`
	AppliedMaxOutputTokens int    `json:"applied_max_output_tokens,omitempty"`
	// ResponseTruncatedPrefix marks ResponseDigest/ResponseLength as describing
	// only the leading prefix the transport buffered for an oversized body, not
	// the full body, so an auditor knows the digest is recomputable from the
	// first ResponseLength bytes of the response.
	ResponseTruncatedPrefix bool   `json:"response_truncated_prefix,omitempty"`
	StartedAtUnixMicros     int64  `json:"started_at_unix_micros"`
	LatencyMicros           int64  `json:"latency_micros"`
	PrevHash                string `json:"prev_hash"`
	Hash                    string `json:"hash,omitempty"`
}

// EncodeCallRecord returns the canonical full-record bytes including the hash.
func EncodeCallRecord(r CallRecord) ([]byte, error) {
	r.Kind = callRecordKind
	return canonicalJSON(r)
}

// CallRecordChainBytes returns the bytes over which the record hash is taken:
// the full canonical record with the hash field absent. An outside verifier
// uses it to recompute a recorded hash from the record alone.
func CallRecordChainBytes(r CallRecord) ([]byte, error) {
	r.Kind = callRecordKind
	r.Hash = ""
	return canonicalJSON(r)
}

// DecodeCallRecord accepts only a complete, canonical, hash-consistent record.
func DecodeCallRecord(line []byte) (CallRecord, error) {
	var r CallRecord
	if err := strictUnmarshal(line, &r); err != nil {
		return CallRecord{}, err
	}
	if r.Kind != callRecordKind {
		return CallRecord{}, fmt.Errorf("%w: kind %q", ErrInvalidRecord, r.Kind)
	}
	if r.Role == "" || r.ModelID == "" || r.Endpoint == "" || r.Attempt < 1 {
		return CallRecord{}, fmt.Errorf("%w: missing call identity", ErrInvalidRecord)
	}
	stored, err := parseHashField(r.Hash)
	if err != nil {
		return CallRecord{}, err
	}
	if _, err := parseHashField(r.RequestHash); err != nil {
		return CallRecord{}, err
	}
	if _, err := parseHashField(r.PrevHash); err != nil {
		return CallRecord{}, err
	}
	chain, err := CallRecordChainBytes(r)
	if err != nil {
		return CallRecord{}, err
	}
	if sha256.Sum256(chain) != stored {
		return CallRecord{}, ErrRecordHashMismatch
	}
	full, err := EncodeCallRecord(r)
	if err != nil {
		return CallRecord{}, err
	}
	if !bytes.Equal(full, line) {
		return CallRecord{}, ErrNonCanonicalRecord
	}
	return r, nil
}

// InterventionKind is the declared intervention category (Appendix B).
type InterventionKind string

const (
	InterventionIdentity       InterventionKind = "identity"
	InterventionRoleA          InterventionKind = "role-a"
	InterventionPatchPropose   InterventionKind = "patch-propose"
	InterventionPatchValidate  InterventionKind = "patch-validate"
	InterventionPatchApply     InterventionKind = "patch-apply"
	InterventionPatchMeasure   InterventionKind = "patch-measure"
	InterventionPatchRollback  InterventionKind = "patch-rollback"
	InterventionNoTriggerAudit InterventionKind = "no-trigger-audit"
	InterventionAllocation     InterventionKind = "allocation"
)

// interventionKinds is the closed set of admitted kinds.
var interventionKinds = map[InterventionKind]bool{
	InterventionIdentity: true, InterventionRoleA: true,
	InterventionPatchPropose: true, InterventionPatchValidate: true,
	InterventionPatchApply: true, InterventionPatchMeasure: true,
	InterventionPatchRollback: true, InterventionNoTriggerAudit: true,
	InterventionAllocation: true,
}

// InterventionCause records why an intervention happened (spec 9.1).
type InterventionCause struct {
	WakePolicy     string       `json:"wake_policy,omitempty"`
	Seam           string       `json:"seam,omitempty"`
	Actor          sim.EntityID `json:"actor,omitempty"`
	Hour           int          `json:"hour"`
	SimTimeMicros  int64        `json:"sim_time_micros,omitempty"`
	Trigger        string       `json:"trigger,omitempty"`
	EvidenceDigest string       `json:"evidence_digest,omitempty"`
}

// InterventionCost records the calls and engine time an intervention consumed.
type InterventionCost struct {
	Calls            uint64 `json:"calls"`
	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
	CostMicros       int64  `json:"cost_micros"`
	EngineMicros     int64  `json:"engine_micros"`
}

// RoleARecord is the Role A decision recorded on an intervention.
// RoleAVote is one outcome bucket of a k-sample Role A decision: the outcome
// name (model action or typed fallback reason) and how many samples backed it.
// It is recorded on the role-a intervention, so a near-tie is visible.
type RoleAVote struct {
	Outcome string `json:"outcome"`
	Count   int    `json:"count"`
}

// RoleARecord is the Role A decision block of an intervention record
type RoleARecord struct {
	Action         string  `json:"action"`
	Source         string  `json:"source"`
	Reason         string  `json:"reason,omitempty"`
	Confidence     float64 `json:"confidence"`
	FallbackReason string  `json:"fallback_reason,omitempty"`
	// FrozenAction is the frozen cascade's own claim action at the same seam
	// instant, recorded so the load-bearing check can verify the applied model
	// action diverged from it (spec EC-2). Empty on an unrecorded/frozen path.
	FrozenAction string `json:"frozen_action,omitempty"`
	// Votes is the vote distribution of a k-sample decision (k > 1). Empty on
	// the single-sample seam, so those records keep their bytes.
	Votes []RoleAVote `json:"votes,omitempty"`
}

// NaturalRefRecord is the canonical wire form of a strategy reference.
type NaturalRefRecord struct {
	ID      string `json:"id"`
	Version uint32 `json:"version"`
}

// NaturalRefRecordOf converts a strategy reference to its wire form.
func NaturalRefRecordOf(ref strategy.NaturalRef) NaturalRefRecord {
	return NaturalRefRecord{ID: ref.ID, Version: ref.Version}
}

// PatchRecord records a strategy-modification intervention (spec 9.1).
// Proposer is the declared proposer identity (model id + call id, or the
// reserved harness identity); it is empty on a Role A role marker.
type PatchRecord struct {
	ParentRef            NaturalRefRecord `json:"parent_ref"`
	ChildRef             NaturalRefRecord `json:"child_ref"`
	ParentFingerprint    string           `json:"parent_fingerprint"`
	ChildFingerprint     string           `json:"child_fingerprint"`
	Deltas               []ParameterDelta `json:"deltas"`
	Intent               string           `json:"intent"`
	PatchDigest          string           `json:"patch_digest"`
	Scope                string           `json:"scope"`
	Proposer             string           `json:"proposer,omitempty"`
	AffectedActorsDigest string           `json:"affected_actors_digest"`
}

// AllocationRecord is the additive allocation-intervention payload (spec
// Appendix B, C8): one record per escalated hour carrying the declared policy
// fingerprint, the candidate roster size, the per-candidate priority scores, the
// selected and protected-exploration sets, the budget state, the declared call
// ceiling N, the policy-PRNG position, and the cumulative realized calls. It is
// nil on every non-allocation record, so existing intervention bytes are
// unchanged.
type AllocationRecord struct {
	PolicyFingerprint string                    `json:"policy_fingerprint"`
	RosterSize        int                       `json:"roster_size"`
	Quota             int                       `json:"quota"`     // U/A: declared q(h); P: remaining budget (5.1)
	Shortfall         int                       `json:"shortfall"` // U/A: max(0, q(h) - roster size); P: 0 (5.1)
	Selected          []sim.EntityID            `json:"selected"`
	Exploration       []sim.EntityID            `json:"exploration"`
	Scores            []protocol.CandidateScore `json:"scores"`
	Budget            protocol.BudgetState      `json:"budget"`
	CallCeiling       uint64                    `json:"call_ceiling"`
	PRNGPos           uint64                    `json:"prng_pos"`
	RealizedCalls     uint64                    `json:"realized_calls"`
}

// ValidationRecord records the validator's verdict and sandbox evidence.
// Validator is the declared validator identity (spec 5.5).
type ValidationRecord struct {
	Verdict             string `json:"verdict"`
	Reason              string `json:"reason"`
	Validator           string `json:"validator,omitempty"`
	SandboxBranchDigest string `json:"sandbox_branch_digest,omitempty"`
	WindowHours         int    `json:"window_hours,omitempty"`
}

// IndicatorDelta is one measured indicator change.
type IndicatorDelta struct {
	Indicator string `json:"indicator"`
	Delta     int64  `json:"delta"`
}

// InterventionResult records the measured outcome, or a typed pending/
// unavailable status for a result the run resolves later.
type InterventionResult struct {
	Status          string           `json:"status"`
	WindowHours     int              `json:"window_hours,omitempty"`
	IndicatorDeltas []IndicatorDelta `json:"indicator_deltas"`
	Reason          string           `json:"reason,omitempty"`
}

// InterventionRecord is one hash-chained intervention (Appendix B). The Role A
// role marker lives in Patch for Role A interventions, and Patch is reserved
// for Role B ones (spec Appendix D.5).
type InterventionRecord struct {
	Seq        uint64            `json:"seq"`
	Kind       InterventionKind  `json:"kind"`
	Cause      InterventionCause `json:"cause"`
	CallRefs   []uint64          `json:"call_refs"`
	Cost       InterventionCost  `json:"cost"`
	RoleA      *RoleARecord      `json:"role_a,omitempty"`
	Patch      *PatchRecord      `json:"patch,omitempty"`
	Validation *ValidationRecord `json:"validation,omitempty"`
	// Allocation is the VS2 additive allocation payload (spec Appendix B, C8).
	// It is nil on every VS1 record, so existing intervention bytes are
	// unchanged (VS2-EC-1).
	Allocation *AllocationRecord  `json:"allocation,omitempty"`
	Result     InterventionResult `json:"result"`
	PrevHash   string             `json:"prev_hash"`
	Hash       string             `json:"hash,omitempty"`
}

// EncodeInterventionRecord returns the canonical full-record bytes.
func EncodeInterventionRecord(r InterventionRecord) ([]byte, error) {
	return canonicalJSON(r)
}

// InterventionRecordChainBytes returns the bytes over which an intervention
// record's hash is taken.
func InterventionRecordChainBytes(r InterventionRecord) ([]byte, error) {
	r.Hash = ""
	return canonicalJSON(r)
}

// DecodeInterventionRecord accepts only a complete, canonical, hash-consistent
// record with an admitted kind.
func DecodeInterventionRecord(line []byte) (InterventionRecord, error) {
	var r InterventionRecord
	if err := strictUnmarshal(line, &r); err != nil {
		return InterventionRecord{}, err
	}
	if !interventionKinds[r.Kind] {
		return InterventionRecord{}, fmt.Errorf("%w: kind %q", ErrInvalidRecord, r.Kind)
	}
	stored, err := parseHashField(r.Hash)
	if err != nil {
		return InterventionRecord{}, err
	}
	if _, err := parseHashField(r.PrevHash); err != nil {
		return InterventionRecord{}, err
	}
	chain, err := InterventionRecordChainBytes(r)
	if err != nil {
		return InterventionRecord{}, err
	}
	if sha256.Sum256(chain) != stored {
		return InterventionRecord{}, ErrRecordHashMismatch
	}
	full, err := EncodeInterventionRecord(r)
	if err != nil {
		return InterventionRecord{}, err
	}
	if !bytes.Equal(full, line) {
		return InterventionRecord{}, ErrNonCanonicalRecord
	}
	return r, nil
}

// canonicalJSON encodes v deterministically: struct field order is source
// declaration order, map keys are sorted, and HTML escaping is off.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func strictUnmarshal(line []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trailing content", ErrInvalidRecord)
	}
	return nil
}

func formatHash(h [32]byte) string { return hex.EncodeToString(h[:]) }

func parseHashField(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != sha256.Size {
		return out, fmt.Errorf("%w: bad hash %q", ErrInvalidRecord, s)
	}
	copy(out[:], b)
	return out, nil
}
