package main

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"

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

// runReplaySubset replays every A1 cell that ran and compares the accepted
// history. It is bounded by the caller's deadline: an unreplayed cell is
// recorded with its reason, never silently dropped.
func runReplaySubset(ctx context.Context, runs []*cellRun, root string, arms []string) map[string]string {
	out := map[string]string{}
	for _, cr := range runs {
		if cr.spec.Arm != ArmA1 || !cr.haveAccepted {
			continue
		}
		key := cr.spec.String()
		if ctx.Err() != nil {
			out[key] = "skipped:budget"
			continue
		}
		replay, err := cognition.OpenReplay(cr.ledgerPath)
		if err != nil {
			out[key] = "replay-open:" + err.Error()
			continue
		}
		// The replay sentinel must be incapable of a network call.
		if _, err := replay.NetworkClient().Complete(ctx, cognition.ModelRequest{
			Model: cognition.DefaultModelID, Endpoint: "sentinel", Request: []byte("{}"),
		}); err == nil {
			out[key] = "network-not-forbidden"
			continue
		}
		rc, err := runCell(ctx, cr.spec, cr.fp, filepath.Join(root, "replay"), replay)
		if err != nil {
			out[key] = "replay-run:" + err.Error()
			continue
		}
		if rc.accepted.TipHash == cr.accepted.TipHash && rc.accepted.ProjectionHash == cr.accepted.ProjectionHash &&
			rc.accepted.HistoryBytes == cr.accepted.HistoryBytes {
			out[key] = "identical"
		} else {
			out[key] = "mismatch"
		}
	}
	return out
}

func asCeiling(err error, target **cognition.CognitiveCeilingHitError) bool {
	return errors.As(err, target)
}

func hexString(b []byte) string { return hex.EncodeToString(b) }

func digestString(d [32]byte) string { return hex.EncodeToString(d[:]) }
