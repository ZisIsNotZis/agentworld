package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// The canonical reproduction policy wire is the complete frozen content —
// format, ref identity, and budget — in fixed big-endian order with exact
// framing. There are no optional fields, so any padding or trailing data
// fails closed. The v4 magic is disjoint from every other wire on disk
// (AWCP capacity, AWFP food-flow, AWSP social-food/scheduler, AWST portable
// strategy, AWCS component snapshots, AWKH kernel history), so no checkpoint
// section can be replayed as another.
var (
	ErrInvalidReproductionPolicyEncoding = errors.New("invalid reproduction policy encoding")
	ErrReproductionPolicyMismatch        = errors.New("reproduction policy content mismatch")
)

var reproductionPolicyMagic = [4]byte{'A', 'W', 'R', 'P'}

const reproductionPolicyMaxBytes = 4 + 4 + 1 + MaxRefIDBytes + 4 + 1 + 1

// EncodeReproductionPolicy encodes every semantic policy field once. The
// one-byte ID length and exact framing reject padding; the frozen
// representation is exactly 27 bytes.
func EncodeReproductionPolicy(p ReproductionPolicy) ([]byte, error) {
	if !validReproductionPolicy(p) {
		return nil, ErrInvalidReproductionPolicy
	}
	out := make([]byte, 0, reproductionPolicyMaxBytes)
	out = append(out, reproductionPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	return out, nil
}

// DecodeReproductionPolicy accepts only complete, bounded, canonical v4
// content. Decoding alone does not establish that a policy matches the
// current build; use VerifyReproductionPolicy with the executable's
// FrozenReproductionPolicy.
func DecodeReproductionPolicy(data []byte) (ReproductionPolicy, error) {
	const minBytes = 4 + 4 + 1 + 1 + 4 + 1 + 1
	if len(data) < minBytes || len(data) > reproductionPolicyMaxBytes || !bytes.Equal(data[:4], reproductionPolicyMagic[:]) {
		return ReproductionPolicy{}, ErrInvalidReproductionPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != minBytes-1+length {
		return ReproductionPolicy{}, ErrInvalidReproductionPolicyEncoding
	}
	refEnd := 9 + length
	p := ReproductionPolicy{
		FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: ReproductionRef{
			ID:      string(data[9:refEnd]),
			Version: binary.BigEndian.Uint32(data[refEnd : refEnd+4]),
		},
		Budget: ReproductionBudget{
			Candidates:  int(data[refEnd+4]),
			Evaluations: int(data[refEnd+5]),
		},
	}
	canonical, err := EncodeReproductionPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return ReproductionPolicy{}, ErrInvalidReproductionPolicyEncoding
	}
	return p, nil
}

// VerifyReproductionPolicy fails closed on both malformed bytes and same-ref
// semantic drift, including a budget that differs from the frozen 32/32. The
// expected policy must come from the current executable, never from the
// checkpoint being verified.
func VerifyReproductionPolicy(data []byte, expected ReproductionPolicy) error {
	if !validReproductionPolicy(expected) {
		return ErrInvalidReproductionPolicy
	}
	actual, err := DecodeReproductionPolicy(data)
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrReproductionPolicyMismatch
	}
	return nil
}

func ReproductionPolicyFingerprint(p ReproductionPolicy) ([sha256.Size]byte, error) {
	encoded, err := EncodeReproductionPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
