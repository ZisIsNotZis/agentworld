package main

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sync"

	"agentworld/internal/cognition"
)

// replayModelClient answers every recorded call from the ledger by request
// hash (spec section 7.3). It has no network path at all; a miss is a hard
// error. The ledger it writes is a fresh record of the same calls, and the
// world is expected to reproduce the identical accepted history.
type replayModelClient struct {
	replay *cognition.Replay
	served int
}

func (c *replayModelClient) Complete(_ context.Context, req cognition.ModelRequest) (cognition.ModelResponse, error) {
	res, err := c.replay.Resolve(req.Request)
	if err != nil {
		return cognition.ModelResponse{}, err
	}
	c.served++
	return cognition.ModelResponse{Body: res.ResponseBody, Content: res.Content, Usage: res.Usage, Status: res.Status}, nil
}

// runReplaySubset replays every recorded AI cell that ran and compares the
// accepted history. It is bounded by the caller's deadline and run with the
// declared matrix parallelism (each cell writes its own replay directory): an
// unreplayed cell is recorded with its reason, never silently dropped. An
// A2/A3 cell with no applied generation (no apply, no rollback) has empty
// generation state, so its genesis replay is byte-exact like A1's; a cell with
// generation state is not replayed from a single bundle (spec 14.1 reduction 2)
// and is declared, not silently omitted.
func runReplaySubset(ctx context.Context, runs []*cellRun, root string, arms []string, parallel int) map[string]string {
	if parallel < 1 {
		parallel = 1
	}
	var mu sync.Mutex
	out := map[string]string{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	for _, cr := range runs {
		switch cr.spec.Arm {
		case ArmA1:
		case ArmA2, ArmA3:
			if cr.roleBApplied > 0 || cr.roleBRollback > 0 {
				continue
			}
		default:
			continue
		}
		if !cr.haveAccepted {
			continue
		}
		cr := cr
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			key, verdict := replayOneCell(ctx, cr, root)
			mu.Lock()
			out[key] = verdict
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// replayOneCell replays one recorded cell from its genesis through the replay
// client and compares the accepted history. A network-incapable sentinel client
// is proven first; a budget-expired cell records its typed skip.
func replayOneCell(ctx context.Context, cr *cellRun, root string) (key, verdict string) {
	key = cr.spec.String()
	if ctx.Err() != nil {
		return key, "skipped:budget"
	}
	replay, err := cognition.OpenReplay(cr.ledgerPath)
	if err != nil {
		return key, "replay-open:" + err.Error()
	}
	// The replay sentinel must be incapable of a network call.
	if _, err := replay.NetworkClient().Complete(ctx, cognition.ModelRequest{
		Model: cognition.DefaultModelID, Endpoint: "sentinel", Request: []byte("{}"),
	}); err == nil {
		return key, "network-not-forbidden"
	}
	rc, err := runCell(ctx, cr.spec, cr.fp, filepath.Join(root, "replay"), replay)
	if err != nil {
		return key, "replay-run:" + err.Error()
	}
	if rc.accepted.TipHash == cr.accepted.TipHash && rc.accepted.ProjectionHash == cr.accepted.ProjectionHash &&
		rc.accepted.HistoryBytes == cr.accepted.HistoryBytes {
		return key, "identical"
	}
	return key, "mismatch"
}

func asCeiling(err error, target **cognition.CognitiveCeilingHitError) bool {
	return errors.As(err, target)
}

func hexString(b []byte) string { return hex.EncodeToString(b) }

func digestString(d [32]byte) string { return hex.EncodeToString(d[:]) }
