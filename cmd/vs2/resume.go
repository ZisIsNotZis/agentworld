package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/world"
)

// Resuming an interrupted live cell (ticket 57). The live R@max cell is ~1,440
// calls; when the execution environment kills the process the ledger keeps every
// call it already recorded, and the world state at any recorded hour is
// reproducible from that ledger with no model call at all (C12). The resume path
// therefore:
//
//  1. truncates the ledger back to its last COMPLETE hour (a killed hour may
//     have issued only part of its batch; those records are removed so the
//     ledger is a clean prefix chain),
//  2. replays the recorded hours hermetically through the ledger-resolving seam
//     — zero model calls, zero spend — to the exact state the live process was
//     in at that hour boundary,
//  3. continues LIVE from there to the declared horizon, appending to the same
//     hash-chained ledger, and
//  4. publishes the ordinary cell evidence and evaluation over the whole cell.
//
// The recorded cell is one continuous ledger and one continuous run; the
// resume boundary is named in the report (resume_from_hour) so no reader has to
// infer it. The unit of second-segment accounting is the ledger, not a fresh
// CognitiveAccount: the cell's token and cost totals are summed from every
// recorded call, so a resumed cell reports the whole cell's spend.

// ledgerRecordHour is the minimal shape needed to find the last complete hour:
// intervention records carry the simulated hour of their cause.
type ledgerRecordHour struct {
	Kind  string `json:"kind"`
	Cause *struct {
		Hour int `json:"hour"`
	} `json:"cause"`
}

// completeLedgerHours returns how many simulated hours (from 0) the ledger
// holds a complete batch for: one more than the highest hour of any recorded
// intervention (a Role A decision appends its call(s) and then its intervention,
// so the last record of an hour is an intervention).
func completeLedgerHours(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	last := -1
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec ledgerRecordHour
		if err := json.Unmarshal(line, &rec); err != nil {
			return 0, fmt.Errorf("resume: undecodable ledger line: %w", err)
		}
		if rec.Cause != nil && rec.Cause.Hour > last {
			last = rec.Cause.Hour
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return last + 1, nil
}

// truncateLedgerAtHour truncates the ledger file after the last record of hour
// (hours-1), removing a partially recorded hour. The remaining prefix must still
// verify as a hash chain (OpenLedger checks it afterwards).
func truncateLedgerAtHour(path string, hours int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cut := 0
	offset := 0
	for offset < len(data) {
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 {
			end = len(data) - offset
		}
		lineEnd := offset + end + 1
		if lineEnd > len(data) {
			lineEnd = len(data)
		}
		var rec ledgerRecordHour
		if err := json.Unmarshal(bytes.TrimSpace(data[offset:lineEnd]), &rec); err != nil {
			return fmt.Errorf("resume: undecodable ledger line: %w", err)
		}
		if rec.Cause != nil && rec.Cause.Hour < hours {
			cut = lineEnd
		}
		offset = lineEnd
	}
	if cut == len(data) {
		return nil
	}
	return os.Truncate(path, int64(cut))
}

// resumeCell completes an interrupted live cell from its run directory: it
// replays the recorded prefix hermetically, then continues live to the declared
// horizon. It returns the same cellRun the ordinary path returns, so the
// evaluation is built unchanged.
func resumeCell(ctx context.Context, spec cellSpec, fp fixture, dir string) (*cellRun, error) {
	if activeLive == nil {
		return nil, errors.New("resume: a live transport is required (pass -live and the spend token)")
	}
	if spec.Arm == ArmA0 {
		return nil, errors.New("resume: A0 makes no calls and has nothing to resume")
	}
	ledgerPath := dir + "/calls.jsonl"
	hours, err := completeLedgerHours(ledgerPath)
	if err != nil {
		return nil, err
	}
	if hours < 1 || hours >= fp.Hours {
		return nil, fmt.Errorf("resume: ledger covers %d complete hours of %d; nothing to resume", hours, fp.Hours)
	}
	if err := truncateLedgerAtHour(ledgerPath, hours); err != nil {
		return nil, err
	}
	// The genesis is rebuilt deterministically from the declared fixture: the
	// root/branch bundles are re-derived byte-identically, so the resumed runner
	// is the same world the aborted process built. They are written to a
	// sibling directory so the aborted run's own bundles stay untouched evidence.
	genesisDir := filepath.Join(dir, "resume-genesis")
	if err := os.MkdirAll(genesisDir, 0o700); err != nil {
		return nil, err
	}
	f, rootDigest, branchDigest, err := buildBranchRunner(ctx, fp, genesisDir)
	if err != nil {
		return nil, err
	}
	cr := &cellRun{spec: spec, fp: fp, dir: dir, rootDigest: rootDigest, branchDigest: branchDigest, ledgerPath: ledgerPath}
	if replayHours, err := completeLedgerHours(ledgerPath); err != nil || replayHours != hours {
		return nil, fmt.Errorf("resume: truncated ledger covers %d hours, want %d (%v)", replayHours, hours, err)
	}

	wake := declaredPolicy(spec.Arm, spec.Seed)
	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		return nil, err
	}
	replaySeam, err := cognition.NewReplaySeamForContractModel(replay, wake, declaredClaimContract, declaredModelID())
	if err != nil {
		return nil, err
	}
	if err := f.SetCognition(&world.NaturalCognition{Seam: replaySeam, WakePolicy: wake, ClaimConcurrency: declaredCallConcurrency}); err != nil {
		return nil, err
	}
	start := time.Now()
	stepTo(ctx, f, cr, hours-1)
	if cr.runErr != nil || cr.halted {
		return cr, cr.runErr
	}
	if served := replaySeam.ServedCalls(); served == 0 {
		return nil, errors.New("resume: the replay prefix served no call")
	}

	// The live segment appends to the same ledger and re-wires the seam over it.
	ledger, err := cognition.OpenLedger(ledgerPath)
	if err != nil {
		return nil, err
	}
	account, err := cognition.NewCognitiveAccount(budgetFor(spec.Arm))
	if err != nil {
		return nil, err
	}
	recorder, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: ledger, Client: activeLive.transport, Budget: account, Price: activeLive.price,
		Now:              time.Now,
		AttemptTimeout:   declaredAttemptTimeout,
		CallDeadline:     declaredCallDeadline,
		MaxResponseBytes: declaredResponseBound,
		Sleep:            func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		return nil, err
	}
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: recorder, Ledger: ledger, Endpoint: activeLive.endpoint,
		CredentialPresent: false, WakePolicy: wake, Votes: declaredVotes, Contract: declaredClaimContract,
		Model: declaredModelID(),
	})
	if err != nil {
		return nil, err
	}
	if err := f.SetCognition(&world.NaturalCognition{Seam: seam, WakePolicy: wake, ClaimConcurrency: declaredCallConcurrency}); err != nil {
		return nil, err
	}
	stack := &cognitionStack{ledger: ledger, account: account, recorder: recorder, endpoint: activeLive.endpoint, ledgerPath: ledgerPath, wake: wake}
	stepTo(ctx, f, cr, fp.Hours)
	cr.durationMS = time.Since(start).Milliseconds()
	gatherCellEvidence(f, stack, cr)

	// The cell's totals are the whole ledger's, so a resumed cell reports the
	// spend and calls of both live segments, never only the continuation.
	records, err := cognition.ReadCallRecords(ledgerPath)
	if err != nil && cr.runErr == nil {
		cr.runErr = fmt.Errorf("resume totals: %w", err)
	}
	var totals cognition.CognitiveTotals
	for _, rec := range records {
		if rec.Role != cognition.RoleActorDecision {
			continue
		}
		totals.Calls++
		totals.PromptTokens += rec.Usage.PromptTokens
		totals.CompletionTokens += rec.Usage.CompletionTokens
		totals.CostMicros += rec.CostMicros
	}
	cr.totals = totals
	cr.resumeFromHour = hours
	return cr, cr.runErr
}
