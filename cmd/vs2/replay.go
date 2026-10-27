package main

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"

	"agentworld/internal/cognition"
)

// The priority-arm replay lane (VS2-EC-5). Every budgeted arm that ran is
// re-run from its genesis with a transport that answers from the recorded
// ledger (never a model call) and a fresh arm-local selector, and the accepted
// history, its projection, and the selector's selection sequence are compared
// against the live run. A priority-bearing replay that drifted would fail the
// equality; a network-incapable sentinel proves the replay path cannot reach
// the gateway.

// runReplaySubset replays every recorded arm cell that carried a ledger: the
// budgeted (priority-bearing) arms and, since ticket 57, the R reference arm
// (nil Priority, full wake). R is the W2 reference deliverable, so its
// ledger-only replay is part of its determinism evidence. A0 has no seam and no
// ledger, so it has nothing to replay. An unreplayed cell is never silently
// dropped: a budget-expired cell records its typed skip.
func runReplaySubset(ctx context.Context, runs []*cellRun, root string, parallel int) map[string]string {
	if parallel < 1 {
		parallel = 1
	}
	var mu sync.Mutex
	out := map[string]string{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	for _, cr := range runs {
		if cr.spec.Arm == ArmA0 || !cr.haveAccepted {
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
// client and compares the accepted history and the selector sequence. A
// nil-Priority arm (R) has no selector, so its selector sequence is empty on
// both sides; its accepted history, projection and bytes are still compared.
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
	if rc.accepted.TipHash != cr.accepted.TipHash || rc.accepted.ProjectionHash != cr.accepted.ProjectionHash ||
		rc.accepted.HistoryBytes != cr.accepted.HistoryBytes {
		return key, "mismatch:accepted-history"
	}
	if !reflect.DeepEqual(rc.selectorSteps, cr.selectorSteps) {
		return key, "mismatch:selector-sequence"
	}
	return key, "identical"
}
