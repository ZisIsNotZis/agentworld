// Package checkpoint stores immutable, bounded checkpoint bundles. The store
// verifies framing and integrity, not the meaning of the opaque sections.
package checkpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const (
	FormatVersion    uint32 = 1
	MaxBundleBytes          = 160 << 20
	MaxManifestBytes        = 1 << 20

	KernelHistory   = "kernel-history"
	Scheduler       = "scheduler"
	Strategy        = "strategy"
	Journal         = "journal"
	ManifestLineage = "manifest-lineage"
)

var ErrBundle = errors.New("invalid or oversized checkpoint bundle")

// Section bytes are opaque to the store. Each required section occurs exactly
// once, at version 1, in the canonical order specified below. The caller must
// validate each section's own schema and its cross-section references.
type Section struct {
	Name    string
	Version uint32
	Data    []byte
}

var sectionNames = [...]string{Journal, KernelHistory, ManifestLineage, Scheduler, Strategy}
var sectionLimits = [...]int{16 << 20, 128 << 20, MaxManifestBytes, 8 << 20, 2 << 20}

var bundleMagic = [4]byte{'A', 'W', 'C', 'B'}

const bundleHeaderSize = 4 + 4 + 2
const sectionHeaderSize = 1 + 4 + 4 // name length, version, payload length

// Encode returns an owned, canonical bundle and its SHA-256 digest. The digest
// covers all bytes preceding its 32-byte trailer, including format and section
// metadata. It detects corruption, not replacement by an attacker; callers
// must retain or authenticate the digest independently for that guarantee.
func Encode(sections []Section) ([]byte, [32]byte, error) {
	if len(sections) != len(sectionNames) {
		return nil, [32]byte{}, ErrBundle
	}
	length := uint64(bundleHeaderSize + sha256.Size)
	for i, s := range sections {
		if s.Name != sectionNames[i] || s.Version != 1 || len(s.Data) == 0 || len(s.Data) > sectionLimits[i] {
			return nil, [32]byte{}, ErrBundle
		}
		length += uint64(sectionHeaderSize + len(s.Name) + len(s.Data))
		if length > MaxBundleBytes {
			return nil, [32]byte{}, ErrBundle
		}
	}
	out := make([]byte, 0, int(length))
	out = append(out, bundleMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, FormatVersion)
	out = binary.BigEndian.AppendUint16(out, uint16(len(sections)))
	for _, s := range sections {
		out = append(out, byte(len(s.Name)))
		out = append(out, s.Name...)
		out = binary.BigEndian.AppendUint32(out, s.Version)
		out = binary.BigEndian.AppendUint32(out, uint32(len(s.Data)))
		out = append(out, s.Data...)
	}
	digest := sha256.Sum256(out)
	out = append(out, digest[:]...)
	return out, digest, nil
}

// Decode checks the complete framing and digest before returning sections.
// The returned sections own the supplied data: do not mutate the input while
// using them. No section semantics are inferred from a valid envelope.
func Decode(data []byte) ([]Section, [32]byte, error) {
	fail := func() ([]Section, [32]byte, error) { return nil, [32]byte{}, ErrBundle }
	if len(data) < bundleHeaderSize+sha256.Size || len(data) > MaxBundleBytes {
		return fail()
	}
	body := data[:len(data)-sha256.Size]
	digest := sha256.Sum256(body)
	if !bytes.Equal(digest[:], data[len(body):]) || !bytes.Equal(body[:4], bundleMagic[:]) ||
		binary.BigEndian.Uint32(body[4:8]) != FormatVersion || binary.BigEndian.Uint16(body[8:10]) != uint16(len(sectionNames)) {
		return fail()
	}
	sections := make([]Section, 0, len(sectionNames))
	offset := bundleHeaderSize
	for i, name := range sectionNames {
		if len(body)-offset < 1 {
			return fail()
		}
		nameSize := int(body[offset])
		offset++
		if nameSize != len(name) || len(body)-offset < nameSize+8 || string(body[offset:offset+nameSize]) != name {
			return fail()
		}
		offset += nameSize
		version := binary.BigEndian.Uint32(body[offset:])
		size := binary.BigEndian.Uint32(body[offset+4:])
		offset += 8
		// Check count, per-section bound, and available bytes before any
		// section-sized allocation or slicing. The five-element slice above
		// is independent of the untrusted count.
		if version != 1 || size == 0 || size > uint32(sectionLimits[i]) || uint64(size) > uint64(len(body)-offset) {
			return fail()
		}
		sections = append(sections, Section{Name: name, Version: version, Data: body[offset : offset+int(size)]})
		offset += int(size)
	}
	if offset != len(body) {
		return fail()
	}
	return sections, digest, nil
}
