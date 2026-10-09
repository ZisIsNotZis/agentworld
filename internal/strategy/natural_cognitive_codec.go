package strategy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// The canonical cognitive-generation wire. It is byte-pinned and
// fingerprinted with the same discipline as the frozen natural policy wire:
// fixed big-endian order, exact framing, and a re-encode equality check on
// decode, so padding, trailing data, or a non-canonical spelling fails closed.
// The v5 policy magic AWNP and this AWCG magic are disjoint.
var ErrInvalidNaturalCognitiveEncoding = errors.New("invalid natural cognitive generation encoding")

var naturalCognitiveMagic = [4]byte{'A', 'W', 'C', 'G'}

// cognitiveParameterCount is the fixed arity of the Appendix C contract,
// pinned by a test against the canonical order.
const cognitiveParameterCount = 7

// MaxNaturalCognitiveWireBytes bounds the canonical wire: magic, format, three
// length-prefixed refs, the parent fingerprint, the provision flag, the seven
// parameter bytes, and the two budget bytes.
const MaxNaturalCognitiveWireBytes = 4 + 4 + 3*(1+MaxRefIDBytes+4) + sha256.Size + 1 + cognitiveParameterCount + 1 + 1

// EncodeNaturalCognitiveGeneration returns the canonical wire of a generation.
func EncodeNaturalCognitiveGeneration(g NaturalCognitiveGeneration) ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	out := make([]byte, 0, MaxNaturalCognitiveWireBytes)
	out = append(out, naturalCognitiveMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, NaturalCognitiveFormatVersion)
	out = appendCognitiveRef(out, g.BaseRef)
	out = appendCognitiveRef(out, g.Ref)
	out = appendCognitiveRef(out, g.ParentRef)
	out = append(out, g.ParentFingerprint[:]...)
	if g.Provision {
		out = append(out, 1)
	} else {
		out = append(out, 0)
	}
	for _, id := range cognitiveParameterOrder {
		value, _ := g.Params.Value(id)
		out = append(out, byte(value))
	}
	out = append(out, byte(FrozenNaturalPolicy7().Budget.Candidates), byte(FrozenNaturalPolicy7().Budget.Evaluations))
	canonical, err := canonicalCognitiveWire(g)
	if err != nil || !bytes.Equal(out, canonical) {
		return nil, ErrInvalidNaturalCognitiveEncoding
	}
	return out, nil
}

// appendCognitiveRef writes one bounded ref (ID length + ID + version).
func appendCognitiveRef(out []byte, ref NaturalRef) []byte {
	out = append(out, byte(len(ref.ID)))
	out = append(out, ref.ID...)
	return binary.BigEndian.AppendUint32(out, ref.Version)
}

// canonicalCognitiveWire is the single spelling of the wire bytes, used both
// to emit and to validate canonicality.
func canonicalCognitiveWire(g NaturalCognitiveGeneration) ([]byte, error) {
	if len(g.BaseRef.ID) > MaxRefIDBytes || len(g.Ref.ID) > MaxRefIDBytes || len(g.ParentRef.ID) > MaxRefIDBytes {
		return nil, ErrInvalidNaturalCognitiveEncoding
	}
	out := make([]byte, 0, MaxNaturalCognitiveWireBytes)
	out = append(out, naturalCognitiveMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, NaturalCognitiveFormatVersion)
	out = appendCognitiveRef(out, g.BaseRef)
	out = appendCognitiveRef(out, g.Ref)
	out = appendCognitiveRef(out, g.ParentRef)
	out = append(out, g.ParentFingerprint[:]...)
	if g.Provision {
		out = append(out, 1)
	} else {
		out = append(out, 0)
	}
	for _, id := range cognitiveParameterOrder {
		value, _ := g.Params.Value(id)
		if value < 0 || value > 255 {
			return nil, ErrInvalidNaturalCognitiveEncoding
		}
		out = append(out, byte(value))
	}
	out = append(out, byte(FrozenNaturalPolicy7().Budget.Candidates), byte(FrozenNaturalPolicy7().Budget.Evaluations))
	return out, nil
}

// DecodeNaturalCognitiveGeneration accepts only a complete, canonical,
// self-consistent generation wire.
func DecodeNaturalCognitiveGeneration(data []byte) (NaturalCognitiveGeneration, error) {
	if len(data) < 4 || len(data) > MaxNaturalCognitiveWireBytes || !bytes.Equal(data[:4], naturalCognitiveMagic[:]) {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	r := cognitiveReader{data: data[4:]}
	format := r.u32()
	if format != NaturalCognitiveFormatVersion {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	g := NaturalCognitiveGeneration{}
	g.BaseRef = r.ref()
	g.Ref = r.ref()
	g.ParentRef = r.ref()
	copy(g.ParentFingerprint[:], r.take(sha256.Size))
	switch r.u8() {
	case 0:
		g.Provision = false
	case 1:
		g.Provision = true
	default:
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	for _, id := range cognitiveParameterOrder {
		value := int64(r.u8())
		var ok bool
		g.Params, ok = g.Params.WithValue(id, value)
		if !ok {
			return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
		}
	}
	candidates, evaluations := r.u8(), r.u8()
	if r.bad || r.remaining() != 0 {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	if int(candidates) != FrozenNaturalPolicy7().Budget.Candidates || int(evaluations) != FrozenNaturalPolicy7().Budget.Evaluations {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	if err := g.Validate(); err != nil {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	canonical, err := canonicalCognitiveWire(g)
	if err != nil || !bytes.Equal(data, canonical) {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalCognitiveEncoding
	}
	return g, nil
}

// NaturalCognitiveGenerationFingerprint is SHA-256 over the canonical wire: the
// content identity an outside party recomputes from the generation alone.
func NaturalCognitiveGenerationFingerprint(g NaturalCognitiveGeneration) ([32]byte, error) {
	wire, err := EncodeNaturalCognitiveGeneration(g)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(wire), nil
}

// cognitiveReader is a bounded big-endian reader over the cognitive wire.
type cognitiveReader struct {
	data []byte
	off  int
	bad  bool
}

func (r *cognitiveReader) take(n int) []byte {
	if r.bad || n < 0 || r.remaining() < n {
		r.bad = true
		return nil
	}
	out := r.data[r.off : r.off+n]
	r.off += n
	return out
}

func (r *cognitiveReader) u8() uint8 {
	b := r.take(1)
	if len(b) == 0 {
		return 0
	}
	return b[0]
}

func (r *cognitiveReader) u32() uint32 {
	b := r.take(4)
	if len(b) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}

func (r *cognitiveReader) ref() NaturalRef {
	length := int(r.u8())
	id := r.take(length)
	version := r.u32()
	if len(id) != length || length < 1 || length > MaxRefIDBytes {
		r.bad = true
		return NaturalRef{}
	}
	return NaturalRef{ID: string(id), Version: version}
}

func (r *cognitiveReader) remaining() int { return len(r.data) - r.off }
