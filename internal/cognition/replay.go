package cognition

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrLedgerMiss is the hard replay error for a request hash that is not in the
// recorded ledger. Replay never falls back to a live call (spec 7.3, ADR 0006
// decision 2).
var ErrLedgerMiss = errors.New("replayed call not found in ledger")

// deadTransport is the sentinel wired during replay. It fails every request, so
// a replay path that ever reached for the network would fail loud rather than
// silently spend or diverge.
type deadTransport struct{}

func (deadTransport) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	return ModelResponse{}, ErrReplayNetworkForbidden
}

// Replay resolves escalated decisions from a recorded ledger by request hash.
// It holds the sentinel transport only so the runner can wire a dead live
// client; Resolve never calls it and Replay has no network dependency.
type Replay struct {
	transport ModelClient
	calls     map[[32]byte]CallResult
}

// OpenReplay verifies the ledger chain and indexes the used attempt of every
// recorded request hash. The used attempt is the first non-retryable attempt
// recorded for that hash.
func OpenReplay(path string) (*Replay, error) {
	records, err := ReadCallRecords(path)
	if err != nil {
		return nil, err
	}
	return newReplay(records, deadTransport{}), nil
}

func newReplay(records []CallRecord, sentinel ModelClient) *Replay {
	r := &Replay{transport: sentinel, calls: make(map[[32]byte]CallResult, len(records))}
	for _, rec := range records {
		hash, err := parseHashField(rec.RequestHash)
		if err != nil {
			continue
		}
		if _, seen := r.calls[hash]; seen {
			continue
		}
		if rec.Status.retryable(rec.HTTPStatus) {
			continue
		}
		r.calls[hash] = callResultFrom(rec)
	}
	return r
}

// Resolve returns the recorded used attempt for request bytes. A miss is the
// typed ErrLedgerMiss, never a live call.
func (r *Replay) Resolve(request []byte) (CallResult, error) {
	hash := sha256.Sum256(request)
	result, ok := r.calls[hash]
	if !ok {
		return CallResult{}, fmt.Errorf("%w: %s", ErrLedgerMiss, formatHash(hash))
	}
	return result, nil
}

// NetworkClient is the dead sentinel the runner wires as the live transport
// during replay. Calling it fails loud.
func (r *Replay) NetworkClient() ModelClient { return r.transport }

// Len is the number of distinct request hashes replay can resolve.
func (r *Replay) Len() int { return len(r.calls) }
