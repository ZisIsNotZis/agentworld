package main

import (
	"context"
	"testing"
)

// TestLoadCellReconstructsAndValidates pins the reuse path: a freshly run cell
// is reconstructed from its committed final bundle and ledger, reproduces the
// same world facts, and refuses a prior report that disagrees. The refusal is
// the mutation check for the reuse guard.
func TestLoadCellReconstructsAndValidates(t *testing.T) {
	root := t.TempDir()
	fp := fixture{Patches: 2, R: 8, Founders: 10, Seed: 0, Hours: 4, Workers: 1}
	spec := cellSpec{Arm: ArmA1, F: 10, Seed: 0}
	cr, err := runCell(context.Background(), spec, fp, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cr.haveAccepted {
		t.Fatal("run produced no accepted history")
	}
	prior := cellReportOf(cr)
	loaded, err := loadCell(prior, spec, fp, root)
	if err != nil {
		t.Fatalf("loadCell: %v", err)
	}
	if loaded.accepted.TipHash != cr.accepted.TipHash ||
		loaded.accepted.ProjectionHash != cr.accepted.ProjectionHash ||
		loaded.accepted.EventCount != cr.accepted.EventCount ||
		loaded.accepted.HistoryBytes != cr.accepted.HistoryBytes {
		t.Fatalf("loaded accepted evidence differs:\nloaded %+v\nran    %+v", loaded.accepted, cr.accepted)
	}
	if loaded.branchDigest != cr.branchDigest || loaded.rootDigest != cr.rootDigest {
		t.Fatalf("loaded digests differ: branch %s/%s root %s/%s",
			loaded.branchDigest, cr.branchDigest, loaded.rootDigest, cr.rootDigest)
	}
	if len(loaded.calls) != len(cr.calls) || loaded.appliedModel != cr.appliedModel {
		t.Fatalf("loaded call evidence differs: calls %d/%d applied %d/%d",
			len(loaded.calls), len(cr.calls), loaded.appliedModel, cr.appliedModel)
	}

	// Mutation: a prior report whose tip disagrees must be refused, never
	// silently reused.
	bad := prior
	bad.TipHash = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := loadCell(bad, spec, fp, root); err == nil {
		t.Fatal("loadCell accepted a prior report with a mismatched tip hash")
	}
}

// TestCellSpecTwinLabel pins the twin directory label (workers-1 matrix cells
// keep their historical names).
func TestCellSpecTwinLabel(t *testing.T) {
	if got := (cellSpec{Arm: ArmA0, F: 12, Seed: 0}).String(); got != "A0-f12-s0" {
		t.Fatalf("matrix label %q", got)
	}
	if got := (cellSpec{Arm: ArmA0, F: 12, Seed: 0, Workers: 4}).String(); got != "A0-f12-s0-w4" {
		t.Fatalf("twin label %q", got)
	}
}
