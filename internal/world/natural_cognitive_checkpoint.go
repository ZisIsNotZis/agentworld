package world

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"sort"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The checkpoint cognition block (spec section 7.2 and 5.4). It rides the
// natural manifest as a trailing, self-describing omitempty block: a
// non-cognitive manifest writes nothing (byte-identical), while a cognitive
// manifest carries the ledger binding, the wake-policy identity, the cognitive
// ceiling thresholds, the model/sampling identity digest, the base/head refs
// and fingerprints, the lineage chain, the registered generation wires, and the
// per-actor binding table. The block is inside the manifest digest, so it is
// tamper-evident.
const naturalCognitiveBlockMagic = "AWNC"

var ErrNaturalCognitiveBlock = errors.New("invalid natural checkpoint cognition block")

// naturalCognitiveBinding is one actor's binding to a generation ref.
type naturalCognitiveBinding struct {
	Actor sim.EntityID
	Ref   strategy.NaturalRef
}

// naturalCognitiveBlock is the decoded cognition block.
type naturalCognitiveBlock struct {
	WakePolicy      protocol.WakePolicy
	WakeFingerprint [32]byte
	CognitiveBudget cognitionBudgetWire
	ModelDigest     [32]byte
	LedgerDigest    [32]byte
	LedgerCount     uint64
	ApplyHour       int
	BaseRef         strategy.NaturalRef
	HeadRef         strategy.NaturalRef
	HeadFingerprint [32]byte
	Lineage         []strategy.NaturalRef
	Generations     []byte // concatenated generation wires (length-prefixed)
	Bindings        []naturalCognitiveBinding
}

// cognitionBudgetWire mirrors the declared cognitive ceiling (spec section
// 7.2/8.1) without importing the cognition package (EC-6).
type cognitionBudgetWire struct {
	Calls            uint64
	PromptTokens     uint64
	CompletionTokens uint64
	CostMicros       int64
}

// encodeNaturalCognitiveBlock emits the canonical block bytes (magic + length +
// payload).
func encodeNaturalCognitiveBlock(b naturalCognitiveBlock) ([]byte, error) {
	var payload []byte
	payload = appendNaturalRef(payload, b.BaseRef)
	payload = appendNaturalRef(payload, b.HeadRef)
	payload = append(payload, b.HeadFingerprint[:]...)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(b.WakePolicy.Name)))
	payload = append(payload, []byte(b.WakePolicy.Name)...)
	payload = binary.BigEndian.AppendUint32(payload, uint32(b.WakePolicy.PeriodHours))
	payload = binary.BigEndian.AppendUint32(payload, uint32(b.WakePolicy.PhaseHours))
	payload = append(payload, b.WakeFingerprint[:]...)
	payload = binary.BigEndian.AppendUint64(payload, b.CognitiveBudget.Calls)
	payload = binary.BigEndian.AppendUint64(payload, b.CognitiveBudget.PromptTokens)
	payload = binary.BigEndian.AppendUint64(payload, b.CognitiveBudget.CompletionTokens)
	payload = binary.BigEndian.AppendUint64(payload, uint64(b.CognitiveBudget.CostMicros))
	payload = append(payload, b.ModelDigest[:]...)
	payload = append(payload, b.LedgerDigest[:]...)
	payload = binary.BigEndian.AppendUint64(payload, b.LedgerCount)
	payload = binary.BigEndian.AppendUint32(payload, uint32(b.ApplyHour))
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(b.Lineage)))
	for _, ref := range b.Lineage {
		payload = appendNaturalRef(payload, ref)
	}
	genCount := 0
	for rest := b.Generations; len(rest) > 0; {
		length := int(binary.BigEndian.Uint32(rest))
		if length < 0 || length+4 > len(rest) {
			return nil, ErrNaturalCognitiveBlock
		}
		genCount++
		rest = rest[4+length:]
	}
	payload = binary.BigEndian.AppendUint16(payload, uint16(genCount))
	for len(b.Generations) > 0 {
		length := int(binary.BigEndian.Uint32(b.Generations))
		if length < 0 || length+4 > len(b.Generations) {
			return nil, ErrNaturalCognitiveBlock
		}
		payload = binary.BigEndian.AppendUint32(payload, uint32(length))
		payload = append(payload, b.Generations[4:4+length]...)
		b.Generations = b.Generations[4+length:]
	}
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(b.Bindings)))
	for _, binding := range b.Bindings {
		payload = binary.BigEndian.AppendUint64(payload, uint64(binding.Actor))
		payload = appendNaturalRef(payload, binding.Ref)
	}
	out := append([]byte(nil), naturalCognitiveBlockMagic...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(payload)))
	return append(out, payload...), nil
}

// decodeNaturalCognitiveBlock parses a block starting at data[0]; it returns
// the block and the total bytes consumed.
func decodeNaturalCognitiveBlock(data []byte) (naturalCognitiveBlock, int, error) {
	var b naturalCognitiveBlock
	if len(data) < 8 || string(data[:4]) != naturalCognitiveBlockMagic {
		return b, 0, ErrNaturalCognitiveBlock
	}
	payloadLen := int(binary.BigEndian.Uint32(data[4:8]))
	if payloadLen < 0 || 8+payloadLen > len(data) {
		return b, 0, ErrNaturalCognitiveBlock
	}
	r := journalReader{data: data[8 : 8+payloadLen]}
	b.BaseRef = readNaturalRef(&r)
	b.HeadRef = readNaturalRef(&r)
	copy(b.HeadFingerprint[:], r.take(32))
	nameLen := int(r.u16())
	name := r.take(nameLen)
	b.WakePolicy = protocol.WakePolicy{Name: string(name), PeriodHours: int(int32(r.u32())), PhaseHours: int(int32(r.u32()))}
	copy(b.WakeFingerprint[:], r.take(32))
	b.CognitiveBudget = cognitionBudgetWire{Calls: r.u64(), PromptTokens: r.u64(), CompletionTokens: r.u64(), CostMicros: int64(r.u64())}
	copy(b.ModelDigest[:], r.take(32))
	copy(b.LedgerDigest[:], r.take(32))
	b.LedgerCount = r.u64()
	b.ApplyHour = int(int32(r.u32()))
	lineageCount := int(r.u16())
	for i := 0; i < lineageCount; i++ {
		b.Lineage = append(b.Lineage, readNaturalRef(&r))
	}
	genCount := int(r.u16())
	for i := 0; i < genCount; i++ {
		length := int(r.u32())
		wire := r.take(length)
		if len(wire) != length {
			return naturalCognitiveBlock{}, 0, ErrNaturalCognitiveBlock
		}
		prefix := make([]byte, 4)
		binary.BigEndian.PutUint32(prefix, uint32(length))
		b.Generations = append(b.Generations, prefix...)
		b.Generations = append(b.Generations, wire...)
	}
	bindingCount := int(r.u32())
	for i := 0; i < bindingCount; i++ {
		b.Bindings = append(b.Bindings, naturalCognitiveBinding{Actor: sim.EntityID(r.u64()), Ref: readNaturalRef(&r)})
	}
	if r.bad || r.remaining() != 0 {
		return naturalCognitiveBlock{}, 0, ErrNaturalCognitiveBlock
	}
	return b, 8 + payloadLen, nil
}

// generations decodes the registered generation wires.
func (b naturalCognitiveBlock) decodeGenerations() ([]strategy.NaturalCognitiveGeneration, error) {
	var out []strategy.NaturalCognitiveGeneration
	rest := b.Generations
	for len(rest) > 0 {
		if len(rest) < 4 {
			return nil, ErrNaturalCognitiveBlock
		}
		length := int(binary.BigEndian.Uint32(rest))
		if length < 0 || 4+length > len(rest) {
			return nil, ErrNaturalCognitiveBlock
		}
		gen, err := strategy.DecodeNaturalCognitiveGeneration(rest[4 : 4+length])
		if err != nil {
			return nil, ErrNaturalCognitiveBlock
		}
		out = append(out, gen)
		rest = rest[4+length:]
	}
	return out, nil
}

func appendNaturalRef(out []byte, ref strategy.NaturalRef) []byte {
	out = append(out, byte(len(ref.ID)))
	out = append(out, ref.ID...)
	return binary.BigEndian.AppendUint32(out, ref.Version)
}

func readNaturalRef(r *journalReader) strategy.NaturalRef {
	length := int(r.u8())
	if r.bad || length < 1 || length > 255 {
		r.bad = true
		return strategy.NaturalRef{}
	}
	id := r.take(length)
	version := r.u32()
	if r.bad || len(id) != length {
		r.bad = true
		return strategy.NaturalRef{}
	}
	return strategy.NaturalRef{ID: string(id), Version: version}
}

// NaturalCognitiveCheckpoint is the caller-supplied binding the runner records
// on the next checkpoint (spec section 7.2). The zero value is "no cognition".
type NaturalCognitiveCheckpoint struct {
	WakePolicy      protocol.WakePolicy
	CognitiveBudget cognitionBudgetWire
	ModelDigest     [32]byte
	LedgerDigest    [32]byte
	LedgerCount     uint64
	// ApplyHour is the audit hour at/after which the active generation took
	// effect; journal replay re-activates it from that boundary.
	ApplyHour int
}

// SetCognitiveCheckpoint attaches the cognition binding the next checkpoint
// manifest records. A nil/zero binding leaves the manifest byte-unchanged.
func (f *Natural) SetCognitiveCheckpoint(binding *NaturalCognitiveCheckpoint) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if binding == nil {
		f.cognitiveCheckpoint = nil
		return
	}
	copy := *binding
	f.cognitiveCheckpoint = &copy
}

// cognitiveBlockFromState builds the manifest block from the runner's cognitive
// state and the attached binding. The block exists whenever the runner carries
// a cognitive seam or an active generation AND a binding is attached, so a
// seam-only lane (no generation yet) also binds its recorded-call ledger
// digest/count; a non-cognitive runner writes nothing.
func (f *Natural) cognitiveBlockFromState() (*naturalCognitiveBlock, error) {
	if f.cognitiveCheckpoint == nil || (f.cognition == nil && f.cognitiveStrategy == nil) {
		return nil, nil
	}
	binding := f.cognitiveCheckpoint
	wakeFP := binding.WakePolicy.Fingerprint()
	block := &naturalCognitiveBlock{
		WakePolicy: binding.WakePolicy, WakeFingerprint: wakeFP,
		CognitiveBudget: binding.CognitiveBudget, ModelDigest: binding.ModelDigest,
		LedgerDigest: binding.LedgerDigest, LedgerCount: binding.LedgerCount, ApplyHour: binding.ApplyHour,
		BaseRef: f.ref, HeadRef: f.ref,
	}
	if f.cognitiveStrategy == nil {
		// Seam-only lane: no active generation, so the head is the frozen base.
		// The head fingerprint is the frozen policy's own, so a seam-active
		// bundle still records the active behaviour logic identity.
		fingerprint, err := strategy.FrozenNaturalPolicyFingerprint(f.bound[1].Policy())
		if err != nil {
			return nil, err
		}
		block.HeadFingerprint = fingerprint
		return block, nil
	}
	gen := f.cognitiveStrategy.generation
	fingerprint, err := strategy.NaturalCognitiveGenerationFingerprint(gen)
	if err != nil {
		return nil, err
	}
	wire, err := strategy.EncodeNaturalCognitiveGeneration(gen)
	if err != nil {
		return nil, err
	}
	generations := make([]byte, 0, len(wire)+4)
	generations = binary.BigEndian.AppendUint32(generations, uint32(len(wire)))
	generations = append(generations, wire...)
	bindings := make([]naturalCognitiveBinding, 0, len(f.cognitiveStrategy.bound))
	for actor, bound := range f.cognitiveStrategy.bound {
		if bound {
			bindings = append(bindings, naturalCognitiveBinding{Actor: actor, Ref: gen.Ref})
		}
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Actor < bindings[j].Actor })
	block.BaseRef = gen.BaseRef
	block.HeadRef = gen.Ref
	block.HeadFingerprint = fingerprint
	block.Lineage = f.cognitiveStrategy.Lineage()
	block.Generations = generations
	block.Bindings = bindings
	return block, nil
}

// restoreCognitiveState re-activates a decoded cognition block on a freshly
// restored runner: the head generation must be one of the registered wires and
// every binding must resolve to it.
func restoreCognitiveState(f *Natural, block *naturalCognitiveBlock) error {
	replay, err := cognitiveReplayFromBlock(block)
	if err != nil {
		return err
	}
	if replay == nil {
		return nil
	}
	return f.activateCognitiveGenerationLocked(replay.generation, replay.actors, block.Lineage)
}

// cognitiveReplayFromBlock resolves the head generation and its bound actors
// from a decoded block, checking the head fingerprint and every binding. A
// seam-only block carries no generation (the head is the frozen base), so it
// yields no generation replay; its ledger binding is still verified by the
// restore.
func cognitiveReplayFromBlock(block *naturalCognitiveBlock) (*cognitiveJournalReplay, error) {
	if block == nil {
		return nil, nil
	}
	generations, err := block.decodeGenerations()
	if err != nil {
		return nil, err
	}
	if len(generations) == 0 {
		return nil, nil
	}
	var head *strategy.NaturalCognitiveGeneration
	for i := range generations {
		if generations[i].Ref == block.HeadRef {
			head = &generations[i]
		}
	}
	if head == nil {
		return nil, ErrNaturalCognitiveBlock
	}
	fingerprint, err := strategy.NaturalCognitiveGenerationFingerprint(*head)
	if err != nil || fingerprint != block.HeadFingerprint {
		return nil, ErrNaturalCognitiveBlock
	}
	actors := make([]sim.EntityID, 0, len(block.Bindings))
	for _, binding := range block.Bindings {
		if binding.Ref != block.HeadRef {
			return nil, ErrNaturalCognitiveBlock
		}
		actors = append(actors, binding.Actor)
	}
	return &cognitiveJournalReplay{generation: *head, actors: actors, applyHour: block.ApplyHour}, nil
}

// hasNaturalCognitiveBlock reports whether a trailing manifest region begins
// with the cognition-block magic.
func hasNaturalCognitiveBlock(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == naturalCognitiveBlockMagic
}

// cognitiveJournalReplay resolves the active generation replay context from the
// runner state, or nil when no generation is active.
func (f *Natural) cognitiveJournalReplay() *cognitiveJournalReplay {
	if f.cognitiveStrategy == nil || f.cognitiveCheckpoint == nil {
		return nil
	}
	return &cognitiveJournalReplay{
		generation: f.cognitiveStrategy.generation,
		actors:     f.cognitiveStrategy.BoundActors(),
		applyHour:  f.cognitiveCheckpoint.ApplyHour,
	}
}

// VerifyCognitiveLedger refuses a missing, short, or mismatched recorded-call
// ledger (spec section 7.2). The chain itself is the cognition layer's
// verification; the world binds the exact file digest and record count.
func VerifyCognitiveLedger(block *naturalCognitiveBlock, path string) error {
	if block == nil {
		return nil
	}
	summary, err := verifyCognitiveLedgerFile(path)
	if err != nil {
		return err
	}
	if summary.Count != block.LedgerCount || summary.Digest != block.LedgerDigest {
		return ErrNaturalCognitiveBlock
	}
	return nil
}

// cognitiveLedgerSummary is the whole-file binding the manifest records.
type cognitiveLedgerSummary struct {
	Count  uint64
	Digest [32]byte
}

// verifyCognitiveLedgerFile digests the ledger file and counts its records
// without importing the cognition layer (EC-6).
func verifyCognitiveLedgerFile(path string) (cognitiveLedgerSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cognitiveLedgerSummary{}, ErrNaturalCognitiveBlock
	}
	var count uint64
	if len(data) > 0 {
		for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
			if len(line) > 0 {
				count++
			}
		}
	}
	return cognitiveLedgerSummary{Count: count, Digest: sha256.Sum256(data)}, nil
}

// cognitiveDigest is a small helper for tests and callers that need the digest
// of an arbitrary byte slice.
func cognitiveDigest(data []byte) [32]byte { return sha256.Sum256(data) }

// NaturalCognitiveApplier adapts a *Natural to the Role B rebind seam. It is
// structurally compatible with cognition.StrategyApplier without importing
// internal/cognition (EC-6).
type NaturalCognitiveApplier struct {
	runner *Natural
}

// NewNaturalCognitiveApplier builds the adapter.
func NewNaturalCognitiveApplier(f *Natural) *NaturalCognitiveApplier {
	return &NaturalCognitiveApplier{runner: f}
}

// RosterActors returns the living roster in ascending ID order.
func (a *NaturalCognitiveApplier) RosterActors(_ context.Context) ([]sim.EntityID, error) {
	if a == nil || a.runner == nil {
		return nil, ErrNaturalRunner
	}
	a.runner.mu.Lock()
	defer a.runner.mu.Unlock()
	out := make([]sim.EntityID, 0, len(a.runner.roster))
	for _, id := range a.runner.roster {
		if a.runner.pulseLifecycleAlive(id) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// ApplyGeneration atomically rebinds the whole roster and returns the
// affected-actor digest.
func (a *NaturalCognitiveApplier) ApplyGeneration(_ context.Context, gen strategy.NaturalCognitiveGeneration, actors []sim.EntityID) ([32]byte, error) {
	if a == nil || a.runner == nil {
		return [32]byte{}, ErrNaturalRunner
	}
	a.runner.mu.Lock()
	defer a.runner.mu.Unlock()
	if err := a.runner.activateCognitiveGenerationLocked(gen, actors, nil); err != nil {
		return [32]byte{}, err
	}
	return cognitiveAffectedDigest(actors), nil
}

// RollbackGeneration rebinds the roster to the parent generation, or clears
// the active generation when parent is nil (the frozen base).
func (a *NaturalCognitiveApplier) RollbackGeneration(_ context.Context, parent *strategy.NaturalCognitiveGeneration, actors []sim.EntityID) error {
	if a == nil || a.runner == nil {
		return ErrNaturalRunner
	}
	a.runner.mu.Lock()
	defer a.runner.mu.Unlock()
	if parent == nil {
		if a.runner.pending != nil {
			return ErrNaturalRunner
		}
		a.runner.cognitiveStrategy = nil
		return nil
	}
	return a.runner.activateCognitiveGenerationLocked(*parent, actors, nil)
}

func cognitiveAffectedDigest(actors []sim.EntityID) [32]byte {
	sorted := append([]sim.EntityID(nil), actors...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	body := make([]byte, 0, len(sorted)*8)
	for _, id := range sorted {
		body = binary.BigEndian.AppendUint64(body, uint64(id))
	}
	return sha256.Sum256(body)
}
