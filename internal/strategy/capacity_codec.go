package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// The canonical capacity policy wire is the complete frozen content — format,
// ref identity, and budget — in fixed big-endian order with exact framing.
// There are no optional fields, so any padding or trailing data fails closed.
var (
	ErrInvalidCapacityPolicyEncoding = errors.New("invalid capacity policy encoding")
	ErrCapacityPolicyMismatch        = errors.New("capacity policy content mismatch")
)

var capacityPolicyMagic = [4]byte{'A', 'W', 'C', 'P'}

const capacityPolicyMaxBytes = 4 + 4 + 1 + MaxRefIDBytes + 4 + 1 + 1

// EncodeCapacityPolicy encodes every semantic policy field once. The one-byte
// ID length and exact framing reject padding; the representation is at most 79
// bytes.
func EncodeCapacityPolicy(p CapacityPolicy) ([]byte, error) {
	if !validCapacityPolicy(p) {
		return nil, ErrInvalidCapacityPolicy
	}
	out := make([]byte, 0, capacityPolicyMaxBytes)
	out = append(out, capacityPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	return out, nil
}

// DecodeCapacityPolicy accepts only complete, bounded, canonical v3 content.
// Decoding alone does not establish that a policy matches the current build;
// use VerifyCapacityPolicy with the executable's FrozenCapacityPolicy.
func DecodeCapacityPolicy(data []byte) (CapacityPolicy, error) {
	const minBytes = 4 + 4 + 1 + 1 + 4 + 1 + 1
	if len(data) < minBytes || len(data) > capacityPolicyMaxBytes || !bytes.Equal(data[:4], capacityPolicyMagic[:]) {
		return CapacityPolicy{}, ErrInvalidCapacityPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != minBytes-1+length {
		return CapacityPolicy{}, ErrInvalidCapacityPolicyEncoding
	}
	refEnd := 9 + length
	p := CapacityPolicy{
		FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: CapacityRef{
			ID:      string(data[9:refEnd]),
			Version: binary.BigEndian.Uint32(data[refEnd : refEnd+4]),
		},
		Budget: CapacityBudget{
			Candidates:  int(data[refEnd+4]),
			Evaluations: int(data[refEnd+5]),
		},
	}
	canonical, err := EncodeCapacityPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return CapacityPolicy{}, ErrInvalidCapacityPolicyEncoding
	}
	return p, nil
}

// VerifyCapacityPolicy fails closed on both malformed bytes and same-ref
// semantic drift, including a budget that differs from the frozen 16/16. The
// expected policy must come from the current executable, never from the
// checkpoint being verified.
func VerifyCapacityPolicy(data []byte, expected CapacityPolicy) error {
	if !validCapacityPolicy(expected) {
		return ErrInvalidCapacityPolicy
	}
	actual, err := DecodeCapacityPolicy(data)
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrCapacityPolicyMismatch
	}
	return nil
}

func CapacityPolicyFingerprint(p CapacityPolicy) ([sha256.Size]byte, error) {
	encoded, err := EncodeCapacityPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
