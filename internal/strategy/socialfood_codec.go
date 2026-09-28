package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"agentworld/internal/sim"
)

var (
	ErrInvalidSocialFoodPolicyEncoding = errors.New("invalid social-food policy encoding")
	ErrSocialFoodPolicyMismatch        = errors.New("social-food policy content mismatch")
)

var socialFoodPolicyMagic = [4]byte{'A', 'W', 'S', 'P'}

const socialFoodPolicyMinBytes = 4 + 4 + 1 + 1 + 4 + 2 + 10 + SocialFoodMaxActors*9
const socialFoodPolicyMaxBytes = socialFoodPolicyMinBytes - 1 + MaxRefIDBytes

// EncodeSocialFoodPolicy includes the complete directed roster, all ten frozen
// decision parameters, budgets, format and ref. No checkpoint ref alone pins it.
func EncodeSocialFoodPolicy(p SocialFoodPolicy) ([]byte, error) {
	if !validSocialFoodPolicy(p) {
		return nil, ErrInvalidSocialFoodPolicy
	}
	out := make([]byte, 0, socialFoodPolicyMaxBytes)
	out = append(out, socialFoodPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	w := p.Weights
	out = append(out, byte(w.RequestEnergyCeiling), byte(w.ReportedUrgency), byte(w.ReserveFloor),
		byte(w.UrgencyBonus), byte(w.LostMealPenalty), byte(w.LowEnergyCeiling),
		byte(w.LowEnergyPenalty), byte(w.HighHungerFloor), byte(w.HighHungerPenalty), byte(w.AcceptThreshold))
	for _, tie := range p.Ties {
		out = binary.BigEndian.AppendUint64(out, uint64(tie.Target))
		out = append(out, byte(tie.Affinity))
	}
	return out, nil
}

// DecodeSocialFoodPolicy requires exact framing and frozen v2 parameters.
// It does not establish that content matches a particular executable ref or
// budget: VerifySocialFoodPolicy compares against an independent expectation.
func DecodeSocialFoodPolicy(data []byte) (SocialFoodPolicy, error) {
	if len(data) < socialFoodPolicyMinBytes || len(data) > socialFoodPolicyMaxBytes || !bytes.Equal(data[:4], socialFoodPolicyMagic[:]) {
		return SocialFoodPolicy{}, ErrInvalidSocialFoodPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != socialFoodPolicyMinBytes-1+length {
		return SocialFoodPolicy{}, ErrInvalidSocialFoodPolicyEncoding
	}
	end := 9 + length
	p := SocialFoodPolicy{FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: SocialFoodRef{ID: string(data[9:end]), Version: binary.BigEndian.Uint32(data[end : end+4])}}
	p.Budget = SocialFoodBudget{Candidates: int(data[end+4]), Evaluations: int(data[end+5])}
	w := data[end+6 : end+16]
	p.Weights = SocialFoodWeights{RequestEnergyCeiling: int64(w[0]), ReportedUrgency: int64(w[1]), ReserveFloor: int64(w[2]),
		UrgencyBonus: int64(w[3]), LostMealPenalty: int64(w[4]), LowEnergyCeiling: int64(w[5]),
		LowEnergyPenalty: int64(w[6]), HighHungerFloor: int64(w[7]), HighHungerPenalty: int64(w[8]), AcceptThreshold: int64(w[9])}
	pos := end + 16
	for i := range p.Ties {
		p.Ties[i] = SocialFoodTie{Target: sim.EntityID(binary.BigEndian.Uint64(data[pos : pos+8])), Affinity: int64(data[pos+8])}
		pos += 9
	}
	canonical, err := EncodeSocialFoodPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return SocialFoodPolicy{}, ErrInvalidSocialFoodPolicyEncoding
	}
	return p, nil
}

// expected must be constructed by the current executable, not decoded from
// the checkpoint. A well-formed same-ref changed budget also fails closed.
func VerifySocialFoodPolicy(data []byte, expected SocialFoodPolicy) error {
	if !validSocialFoodPolicy(expected) {
		return ErrInvalidSocialFoodPolicy
	}
	p, err := DecodeSocialFoodPolicy(data)
	if err != nil {
		return err
	}
	if p != expected {
		return ErrSocialFoodPolicyMismatch
	}
	return nil
}

func SocialFoodPolicyFingerprint(p SocialFoodPolicy) ([sha256.Size]byte, error) {
	data, err := EncodeSocialFoodPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(data), nil
}
