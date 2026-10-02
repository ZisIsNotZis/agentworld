package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// The canonical natural policy wire is the complete frozen content — format,
// ref identity, and budget — in fixed big-endian order with exact framing.
// There are no optional fields, so any padding or trailing data fails closed.
// The v5 magic AWNP is disjoint from every other wire on disk (AWCP capacity,
// AWRP reproduction, AWFP food-flow, AWSP social-food, AWST portable
// strategy, AWCS component snapshots, AWKH kernel history, and every other
// AW** magic in the executable), so no checkpoint section can be replayed as
// another.
var (
	ErrInvalidNaturalPolicyEncoding = errors.New("invalid natural policy encoding")
	ErrNaturalPolicyMismatch        = errors.New("natural policy content mismatch")
)

var naturalPolicyMagic = [4]byte{'A', 'W', 'N', 'P'}

const naturalPolicyMaxBytes = 4 + 4 + 1 + MaxRefIDBytes + 4 + 1 + 1

// EncodeNaturalPolicy encodes every semantic policy field once. The one-byte
// ID length and exact framing reject padding; the frozen representation is
// exactly 22 bytes.
func EncodeNaturalPolicy(p NaturalPolicy) ([]byte, error) {
	if !validNaturalPolicy(p) {
		return nil, ErrInvalidNaturalPolicy
	}
	out := make([]byte, 0, naturalPolicyMaxBytes)
	out = append(out, naturalPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	return out, nil
}

// DecodeNaturalPolicy accepts only complete, bounded, canonical v5 content.
// Decoding alone does not establish that a policy matches the current build;
// use VerifyNaturalPolicy with the executable's FrozenNaturalPolicy.
func DecodeNaturalPolicy(data []byte) (NaturalPolicy, error) {
	const minBytes = 4 + 4 + 1 + 1 + 4 + 1 + 1
	if len(data) < minBytes || len(data) > naturalPolicyMaxBytes || !bytes.Equal(data[:4], naturalPolicyMagic[:]) {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != minBytes-1+length {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	refEnd := 9 + length
	p := NaturalPolicy{
		FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: NaturalRef{
			ID:      string(data[9:refEnd]),
			Version: binary.BigEndian.Uint32(data[refEnd : refEnd+4]),
		},
		Budget: NaturalBudget{
			Candidates:  int(data[refEnd+4]),
			Evaluations: int(data[refEnd+5]),
		},
	}
	canonical, err := EncodeNaturalPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	return p, nil
}

// VerifyNaturalPolicy fails closed on both malformed bytes and same-ref
// semantic drift, including a budget that differs from the frozen 32/32. The
// expected policy must come from the current executable, never from the
// checkpoint being verified.
func VerifyNaturalPolicy(data []byte, expected NaturalPolicy) error {
	if !validNaturalPolicy(expected) {
		return ErrInvalidNaturalPolicy
	}
	actual, err := DecodeNaturalPolicy(data)
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrNaturalPolicyMismatch
	}
	return nil
}

func NaturalPolicyFingerprint(p NaturalPolicy) ([sha256.Size]byte, error) {
	encoded, err := EncodeNaturalPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
