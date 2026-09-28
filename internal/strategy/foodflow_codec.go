package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"agentworld/internal/sim"
)

// A food-flow policy contains only scalars: no caller can mutate a Bound by
// changing this value-copy. Binding() separately supplies the actor and ref.
func (b *FoodFlowBound) Policy() FoodFlowPolicy {
	if b == nil || b.policy == nil {
		return FoodFlowPolicy{}
	}
	return *b.policy
}

var (
	ErrInvalidFoodFlowPolicyEncoding = errors.New("invalid food-flow policy encoding")
	ErrFoodFlowPolicyMismatch        = errors.New("food-flow policy content mismatch")
)

var foodFlowPolicyMagic = [4]byte{'A', 'W', 'F', 'P'}

const foodFlowMaxPolicyBytes = 4 + 4 + 1 + MaxRefIDBytes + 4 + 1 + 1 + 8

// EncodeFoodFlowPolicy encodes every semantic policy field once, in fixed
// big-endian order. The one-byte ID length and exact framing reject padding
// and trailing data; the complete wire representation is at most 87 bytes.
func EncodeFoodFlowPolicy(p FoodFlowPolicy) ([]byte, error) {
	if !validFoodFlowPolicy(p) {
		return nil, ErrInvalidFoodFlowPolicy
	}
	out := make([]byte, 0, foodFlowMaxPolicyBytes)
	out = append(out, foodFlowPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	out = binary.BigEndian.AppendUint64(out, uint64(p.RestDuration))
	return out, nil
}

// DecodeFoodFlowPolicy accepts only complete, bounded, canonical v1 content.
// Decoding alone does not establish that a policy matches the current build;
// use VerifyFoodFlowPolicy with an independently constructed expected policy.
func DecodeFoodFlowPolicy(data []byte) (FoodFlowPolicy, error) {
	const minBytes = 4 + 4 + 1 + 1 + 4 + 1 + 1 + 8
	if len(data) < minBytes || len(data) > foodFlowMaxPolicyBytes || !bytes.Equal(data[:4], foodFlowPolicyMagic[:]) {
		return FoodFlowPolicy{}, ErrInvalidFoodFlowPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != minBytes-1+length {
		return FoodFlowPolicy{}, ErrInvalidFoodFlowPolicyEncoding
	}
	refEnd := 9 + length
	p := FoodFlowPolicy{
		FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: FoodFlowRef{
			ID:      string(data[9:refEnd]),
			Version: binary.BigEndian.Uint32(data[refEnd : refEnd+4]),
		},
		Budget: FoodFlowBudget{
			Candidates:  int(data[refEnd+4]),
			Evaluations: int(data[refEnd+5]),
		},
		RestDuration: sim.Duration(binary.BigEndian.Uint64(data[refEnd+6:])),
	}
	canonical, err := EncodeFoodFlowPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return FoodFlowPolicy{}, ErrInvalidFoodFlowPolicyEncoding
	}
	return p, nil
}

// VerifyFoodFlowPolicy fails closed on both malformed bytes and same-ref
// semantic drift. The expected policy must come from the current executable,
// never from the checkpoint being verified.
func VerifyFoodFlowPolicy(data []byte, expected FoodFlowPolicy) error {
	if !validFoodFlowPolicy(expected) {
		return ErrInvalidFoodFlowPolicy
	}
	actual, err := DecodeFoodFlowPolicy(data)
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrFoodFlowPolicyMismatch
	}
	return nil
}

func FoodFlowPolicyFingerprint(p FoodFlowPolicy) ([sha256.Size]byte, error) {
	encoded, err := EncodeFoodFlowPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
