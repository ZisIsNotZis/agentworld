package cognition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// The ledger is one append-only file per lane: canonical JSON records, one per
// line, each carrying prev_hash and hash over its canonical bytes. It is
// flushed to disk before a response is consumed, so a used call can never be
// missing from the record (spec 7.1, ADR 0006 decision 1).
var (
	ErrLedger          = errors.New("cognition ledger")
	ErrLedgerTampered  = errors.New("cognition ledger tampered")
	ErrLedgerChain     = errors.New("cognition ledger hash chain broken")
	ErrLedgerTruncated = errors.New("cognition ledger truncated")
)

// LedgerSummary bounds a ledger file: its record count, head hash, and whole
// file digest. A restore compares this against the checkpoint's cognition
// block, which is what makes a truncated ledger detectable (a valid prefix
// chain cannot reveal missing tail records).
type LedgerSummary struct {
	Count  uint64
	Head   [32]byte
	Digest [32]byte
}

// Ledger is an append-only hash-chained record file. CreateLedger refuses to
// clobber an existing file; OpenLedger verifies the existing chain before
// appending.
type Ledger struct {
	mu    sync.Mutex
	path  string
	file  *os.File
	prev  [32]byte
	count uint64
}

// CreateLedger creates a new empty ledger at path and fails if it exists.
func CreateLedger(path string) (*Ledger, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrLedger)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedger, err)
	}
	return &Ledger{path: path, file: f}, nil
}

// OpenLedger opens an existing ledger, verifies its chain, and appends after
// its head.
func OpenLedger(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedger, err)
	}
	summary, err := VerifyLedgerData(data)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedger, err)
	}
	return &Ledger{path: path, file: f, prev: summary.Head, count: summary.Count}, nil
}

// AppendCall appends one call record, setting seq, prev_hash, and hash, and
// flushes it before returning.
func (l *Ledger) AppendCall(r CallRecord) (CallRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return CallRecord{}, fmt.Errorf("%w: closed", ErrLedger)
	}
	r.Seq = l.count
	r.PrevHash = formatHash(l.prev)
	chain, err := CallRecordChainBytes(r)
	if err != nil {
		return CallRecord{}, err
	}
	h := sha256.Sum256(chain)
	r.Hash = formatHash(h)
	full, err := EncodeCallRecord(r)
	if err != nil {
		return CallRecord{}, err
	}
	if err := l.writeLine(full); err != nil {
		return CallRecord{}, err
	}
	l.prev, l.count = h, l.count+1
	return r, nil
}

// AppendIntervention appends one intervention record, setting seq, prev_hash,
// and hash, and flushes it before returning.
func (l *Ledger) AppendIntervention(r InterventionRecord) (InterventionRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return InterventionRecord{}, fmt.Errorf("%w: closed", ErrLedger)
	}
	r.Seq = l.count
	r.PrevHash = formatHash(l.prev)
	chain, err := InterventionRecordChainBytes(r)
	if err != nil {
		return InterventionRecord{}, err
	}
	h := sha256.Sum256(chain)
	r.Hash = formatHash(h)
	full, err := EncodeInterventionRecord(r)
	if err != nil {
		return InterventionRecord{}, err
	}
	if err := l.writeLine(full); err != nil {
		return InterventionRecord{}, err
	}
	l.prev, l.count = h, l.count+1
	return r, nil
}

// writeLine writes one canonical record line and flushes it to disk.
func (l *Ledger) writeLine(line []byte) error {
	buf := make([]byte, 0, len(line)+1)
	buf = append(buf, line...)
	buf = append(buf, '\n')
	if _, err := l.file.Write(buf); err != nil {
		return fmt.Errorf("%w: write: %v", ErrLedger, err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("%w: flush: %v", ErrLedger, err)
	}
	return nil
}

// Path is the ledger file path.
func (l *Ledger) Path() string { return l.path }

// Count is the number of appended records.
func (l *Ledger) Count() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// Head is the current head hash.
func (l *Ledger) Head() [32]byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prev
}

// Flush syncs the file to disk.
func (l *Ledger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return fmt.Errorf("%w: closed", ErrLedger)
	}
	return l.file.Sync()
}

// Close closes the ledger file.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// Summary verifies the on-disk chain and returns count, head, and digest.
func (l *Ledger) Summary() (LedgerSummary, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("%w: %v", ErrLedger, err)
	}
	return VerifyLedgerData(data)
}

// VerifyLedgerData verifies a whole ledger: every line is a canonical,
// hash-consistent record, seqs are contiguous from zero, and each prev_hash
// equals its predecessor's hash. It returns the summary but does not itself
// detect a truncated tail; compare the summary against the bound checkpoint
// summary for that (see VerifyLedgerSummary).
func VerifyLedgerData(data []byte) (LedgerSummary, error) {
	summary := LedgerSummary{Digest: sha256.Sum256(data)}
	var prev [32]byte
	var count uint64
	rest := data
	for len(rest) > 0 {
		idx := bytes.IndexByte(rest, '\n')
		if idx < 0 {
			return summary, fmt.Errorf("%w: partial trailing line", ErrLedgerTampered)
		}
		line := rest[:idx]
		rest = rest[idx+1:]
		if len(line) == 0 {
			return summary, fmt.Errorf("%w: empty line", ErrLedgerTampered)
		}
		hash, err := verifyLine(line, prev, count)
		if err != nil {
			return summary, err
		}
		prev, count = hash, count+1
	}
	summary.Head, summary.Count = prev, count
	return summary, nil
}

// VerifyLedgerSummary verifies the file's chain and that its bound summary
// matches the expected one, so a truncated tail or a swapped file fails.
func VerifyLedgerSummary(path string, expected LedgerSummary) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLedger, err)
	}
	actual, err := VerifyLedgerData(data)
	if err != nil {
		return err
	}
	if actual.Count != expected.Count {
		return fmt.Errorf("%w: %d records, expected %d", ErrLedgerTruncated, actual.Count, expected.Count)
	}
	if actual.Head != expected.Head || actual.Digest != expected.Digest {
		return fmt.Errorf("%w: head or digest differs from the bound summary", ErrLedgerTampered)
	}
	return nil
}

func verifyLine(line []byte, prev [32]byte, expectedSeq uint64) ([32]byte, error) {
	pre, stored, ok := splitHashedLine(line)
	if !ok {
		return [32]byte{}, fmt.Errorf("%w: unhashed line", ErrLedgerTampered)
	}
	var envelope struct {
		Seq      uint64 `json:"seq"`
		PrevHash string `json:"prev_hash"`
	}
	if err := json.Unmarshal(pre, &envelope); err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrLedgerTampered, err)
	}
	if envelope.Seq != expectedSeq {
		return [32]byte{}, fmt.Errorf("%w: seq %d, expected %d", ErrLedgerChain, envelope.Seq, expectedSeq)
	}
	prevHash, err := parseHashField(envelope.PrevHash)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrLedgerTampered, err)
	}
	if prevHash != prev {
		return [32]byte{}, fmt.Errorf("%w: record %d prev_hash mismatch", ErrLedgerChain, expectedSeq)
	}
	if sha256.Sum256(pre) != stored {
		return [32]byte{}, fmt.Errorf("%w: record %d hash mismatch", ErrLedgerTampered, expectedSeq)
	}
	return stored, nil
}

// splitHashedLine recovers the pre-hash canonical bytes and the stored hash.
// The hash is always the last field, so the split is exact.
func splitHashedLine(line []byte) ([]byte, [32]byte, bool) {
	const prefix = `,"hash":"`
	if len(line) < len(prefix)+66 || line[len(line)-1] != '}' || line[len(line)-2] != '"' {
		return nil, [32]byte{}, false
	}
	idx := bytes.LastIndex(line, []byte(prefix))
	if idx < 0 {
		return nil, [32]byte{}, false
	}
	hexStart, hexEnd := idx+len(prefix), len(line)-2
	if hexEnd-hexStart != 64 {
		return nil, [32]byte{}, false
	}
	stored, err := parseHashField(string(line[hexStart:hexEnd]))
	if err != nil {
		return nil, [32]byte{}, false
	}
	pre := make([]byte, 0, idx+1)
	pre = append(pre, line[:idx]...)
	pre = append(pre, '}')
	return pre, stored, true
}

// recordKindOf returns the kind field of a ledger line.
func recordKindOf(line []byte) (string, error) {
	var envelope struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	return envelope.Kind, nil
}

// ReadCallRecords verifies a ledger file's chain and returns its call records
// in order. Intervention lines are skipped; the chain still covers them.
func ReadCallRecords(path string) ([]CallRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedger, err)
	}
	if _, err := VerifyLedgerData(data); err != nil {
		return nil, err
	}
	var out []CallRecord
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		kind, err := recordKindOf(line)
		if err != nil {
			return nil, err
		}
		if kind != callRecordKind {
			continue
		}
		rec, err := DecodeCallRecord(line)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}
