package cognition

import (
	"errors"
	"fmt"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
)

// Role B patch validation (spec section 10.1). A proposal is admissible only if
// it decodes against the versioned contract, its parent ref equals the current
// head exactly, every delta names a contract parameter and lies inside its
// domain, the delta set is non-empty, and the canonical wire fingerprints
// cleanly. Every failure is a typed rejection that touches nothing: this
// function only reads its inputs and returns a value; it has no world effect.
var ErrPatchRejected = errors.New("strategy patch rejected")

// PatchRejection is the typed reason a proposal was refused (spec section 10.1).
type PatchRejection string

const (
	PatchRejectMalformed      PatchRejection = "malformed"
	PatchRejectParentMismatch PatchRejection = "parent-ref-mismatch"
	PatchRejectVersion        PatchRejection = "version"
	PatchRejectUnknownParam   PatchRejection = "unknown-parameter"
	PatchRejectOutOfDomain    PatchRejection = "out-of-domain"
	PatchRejectEmptyDelta     PatchRejection = "empty-delta"
	PatchRejectForbidden      PatchRejection = "forbidden-target"
	PatchRejectWire           PatchRejection = "wire"
)

// PatchError is a typed rejection carrying its reason.
type PatchError struct {
	Reason PatchRejection
	Detail string
}

func (e *PatchError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", ErrPatchRejected, e.Reason, e.Detail)
}
func (e *PatchError) Is(target error) bool {
	return target == ErrPatchRejected
}

func reject(reason PatchRejection, detail string) error {
	return &PatchError{Reason: reason, Detail: detail}
}

// PatchParent is the current head of the cognitive lineage: its ref, content
// fingerprint, inherited base identity, and effective parameter vector. The
// first patch's parent is the frozen base (natural/7 or natural/7b); a later
// patch's parent is the previous generation.
type PatchParent struct {
	Ref         strategy.NaturalRef
	Fingerprint [32]byte
	Base        strategy.NaturalRef
	Provision   bool
	Params      strategy.NaturalCognitiveParams
}

// PatchParentFromBase builds the parent for the first patch over a frozen base.
func PatchParentFromBase(base strategy.NaturalRef) (PatchParent, error) {
	if !strategy.IsCognitiveBase(base) {
		return PatchParent{}, reject(PatchRejectForbidden, fmt.Sprintf("base ref %+v is not a cognitive base", base))
	}
	fingerprint, err := strategy.FrozenNaturalPolicyFingerprint(basePolicyFor(base))
	if err != nil {
		return PatchParent{}, err
	}
	return PatchParent{Ref: base, Fingerprint: fingerprint, Base: base, Provision: strategy.BaseProvision(base), Params: strategy.FrozenCognitiveParams()}, nil
}

// PatchParentFromGeneration builds the parent for a chained patch.
func PatchParentFromGeneration(gen strategy.NaturalCognitiveGeneration) (PatchParent, error) {
	if err := gen.Validate(); err != nil {
		return PatchParent{}, reject(PatchRejectMalformed, fmt.Sprintf("invalid parent generation: %v", err))
	}
	fingerprint, err := strategy.NaturalCognitiveGenerationFingerprint(gen)
	if err != nil {
		return PatchParent{}, err
	}
	return PatchParent{Ref: gen.Ref, Fingerprint: fingerprint, Base: gen.BaseRef, Provision: gen.Provision, Params: gen.Params}, nil
}

// basePolicyFor resolves the frozen policy for an admitted cognitive base.
func basePolicyFor(base strategy.NaturalRef) strategy.NaturalPolicy {
	if base == strategy.FrozenNaturalPolicy7b().Ref {
		return strategy.FrozenNaturalPolicy7b()
	}
	return strategy.FrozenNaturalPolicy7()
}

// ValidatePatchProposal is the deterministic admissibility check. It never
// mutates anything; a typed PatchError is a rejection and a nil error returns
// the declared child generation.
func ValidatePatchProposal(prop protocol.PatchProposal, parent PatchParent) (strategy.NaturalCognitiveGeneration, ValidationRecord, error) {
	if !prop.Propose {
		return strategy.NaturalCognitiveGeneration{}, ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectMalformed)}, reject(PatchRejectMalformed, "proposal not marked propose")
	}
	if prop.ParentRef != parent.Ref {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectParentMismatch)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectParentMismatch, fmt.Sprintf("proposal parent %+v, head %+v", prop.ParentRef, parent.Ref))
	}
	if len(prop.Deltas) == 0 {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectEmptyDelta)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectEmptyDelta, "no deltas")
	}
	childParams := parent.Params
	for _, delta := range prop.Deltas {
		min, max, ok := strategy.CognitiveParameterDomain(strategy.CognitiveParamID(delta.Param))
		if !ok {
			reason := forbiddenTargetReason(delta.Param)
			record := ValidationRecord{Verdict: "rejected", Reason: string(reason)}
			return strategy.NaturalCognitiveGeneration{}, record, reject(reason, string(delta.Param))
		}
		if delta.Value < min || delta.Value > max {
			record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectOutOfDomain)}
			return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectOutOfDomain, fmt.Sprintf("%s=%d outside [%d,%d]", delta.Param, delta.Value, min, max))
		}
		next, ok := childParams.WithValue(strategy.CognitiveParamID(delta.Param), delta.Value)
		if !ok {
			record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectUnknownParam)}
			return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectUnknownParam, string(delta.Param))
		}
		childParams = next
	}
	if childParams == parent.Params {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectEmptyDelta)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectEmptyDelta, "every delta equals the parent value")
	}
	child, err := strategy.NewNaturalCognitiveGeneration(parent.Ref, parent.Fingerprint, parent.Base, parent.Provision, childParams)
	if err != nil {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectVersion)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectVersion, err.Error())
	}
	wire, err := strategy.EncodeNaturalCognitiveGeneration(child)
	if err != nil {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectWire)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectWire, err.Error())
	}
	if _, err := strategy.DecodeNaturalCognitiveGeneration(wire); err != nil {
		record := ValidationRecord{Verdict: "rejected", Reason: string(PatchRejectWire)}
		return strategy.NaturalCognitiveGeneration{}, record, reject(PatchRejectWire, err.Error())
	}
	return child, ValidationRecord{Verdict: "admitted", Reason: ""}, nil
}

// forbiddenTargetReason classifies the non-contract projection fields a patch
// may never touch. It exists so a rejection names the forbidden target rather
// than a generic unknown parameter when the intent is a contract-adjacent
// field; the append-only contract makes any non-contract id a rejection.
func forbiddenTargetReason(param protocol.ParamID) PatchRejection {
	switch string(param) {
	case "mechanism.version", "observation.boundary", "policy.budget", "world.point_cost",
		"world.spoilage", "world.wear", "world.consent_margin":
		return PatchRejectForbidden
	default:
		return PatchRejectUnknownParam
	}
}
