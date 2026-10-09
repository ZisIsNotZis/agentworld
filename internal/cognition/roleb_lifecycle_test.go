package cognition_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role B lifecycle tests (spec sections 5 and 10). The patch validator and
// the sandbox are pure; the lifecycle is driven through fakes so the world is
// never touched by a rejected patch.

func roleBTestLedger(t *testing.T) *cognition.Ledger {
	t.Helper()
	l, err := cognition.CreateLedger(filepath.Join(t.TempDir(), "roleb.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

type fakeProposer struct {
	prop  protocol.PatchProposal
	err   error
	calls int
}

func (f *fakeProposer) ProposeStrategyPatch(context.Context, protocol.PatchRequest) (protocol.PatchProposal, error) {
	f.calls++
	return f.prop, f.err
}

type fakeApplier struct {
	actors    []sim.EntityID
	applied   []strategy.NaturalRef
	rolled    int
	cleared   int
	applyFail error
}

func (f *fakeApplier) RosterActors(context.Context) ([]sim.EntityID, error) {
	return append([]sim.EntityID(nil), f.actors...), nil
}

func (f *fakeApplier) ApplyGeneration(_ context.Context, gen strategy.NaturalCognitiveGeneration, _ []sim.EntityID) ([32]byte, error) {
	if f.applyFail != nil {
		return [32]byte{}, f.applyFail
	}
	f.applied = append(f.applied, gen.Ref)
	return [32]byte{1}, nil
}

func (f *fakeApplier) RollbackGeneration(_ context.Context, parent *strategy.NaturalCognitiveGeneration, _ []sim.EntityID) error {
	if parent == nil {
		f.cleared++
	} else {
		f.rolled++
	}
	return nil
}

type fakeSandbox struct {
	patched int64
	parent  int64
	gating  bool
}

func (s *fakeSandbox) RunBranch(_ context.Context, gen *strategy.NaturalCognitiveGeneration) (cognition.SandboxOutcome, error) {
	if gen == nil {
		return cognition.SandboxOutcome{Indicator: s.parent}, nil
	}
	return cognition.SandboxOutcome{Indicator: s.patched, GatingRegressed: s.gating}, nil
}

func validProposal() protocol.PatchProposal {
	return protocol.PatchProposal{
		Propose:   true,
		ParentRef: strategy.FrozenNaturalPolicy7().Ref,
		Deltas:    []protocol.ParameterDelta{{Param: protocol.ParamClaimProvisionEnergyFloor, Value: 3}},
		Intent:    "lower the provision floor",
		CallRef:   1,
	}
}

func triggeringEvidence() protocol.TrajectoryEvidence {
	return protocol.TrajectoryEvidence{WindowStartHour: 0, WindowHours: 24, BirthsTotal: 0, BuildsCommitted: 2, MaxCapital: 1}
}

func nontriggeringEvidence() protocol.TrajectoryEvidence {
	return protocol.TrajectoryEvidence{WindowStartHour: 24, WindowHours: 24, BirthsTotal: 1, BuildsCommitted: 1, MaxCapital: 3}
}

func newRoleBTestController(t *testing.T, applier *fakeApplier, sandbox *fakeSandbox, proposer cognition.PatchProposer) (*cognition.RoleBController, *cognition.Ledger) {
	t.Helper()
	l := roleBTestLedger(t)
	c, err := cognition.NewRoleBController(cognition.RoleBControllerConfig{
		Ledger: l, Proposer: proposer, Sandbox: sandbox, Applier: applier, Operator: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(strategy.FrozenNaturalPolicy7().Ref); err != nil {
		t.Fatal(err)
	}
	return c, l
}

// TestPatchValidationRejects pins every typed rejection (spec section 10.1) and
// that a rejected proposal never yields a child.
func TestPatchValidationRejects(t *testing.T) {
	parent, err := cognition.PatchParentFromBase(strategy.FrozenNaturalPolicy7().Ref)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*protocol.PatchProposal)
		reason cognition.PatchRejection
	}{
		{"wrong-parent", func(p *protocol.PatchProposal) { p.ParentRef = strategy.FrozenNaturalPolicy9().Ref }, cognition.PatchRejectParentMismatch},
		{"unknown-parameter", func(p *protocol.PatchProposal) { p.Deltas[0].Param = "claim.magic" }, cognition.PatchRejectUnknownParam},
		{"forbidden-target", func(p *protocol.PatchProposal) { p.Deltas[0].Param = "world.point_cost" }, cognition.PatchRejectForbidden},
		{"out-of-domain", func(p *protocol.PatchProposal) { p.Deltas[0].Value = 99 }, cognition.PatchRejectOutOfDomain},
		{"empty-delta", func(p *protocol.PatchProposal) { p.Deltas = nil }, cognition.PatchRejectEmptyDelta},
		{"no-op-delta", func(p *protocol.PatchProposal) { p.Deltas[0].Value = 4 }, cognition.PatchRejectEmptyDelta},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prop := validProposal()
			tc.mutate(&prop)
			child, record, err := cognition.ValidatePatchProposal(prop, parent)
			if err == nil {
				t.Fatalf("accepted: %+v", child)
			}
			var pe *cognition.PatchError
			if !errors.As(err, &pe) || pe.Reason != tc.reason {
				t.Fatalf("reason %v, want %s (%v)", err, tc.reason, tc.reason)
			}
			if record.Verdict != "rejected" || record.Reason != string(tc.reason) {
				t.Fatalf("record %+v", record)
			}
			if (child != strategy.NaturalCognitiveGeneration{}) {
				t.Fatalf("rejected proposal returned a child: %+v", child)
			}
		})
	}
	// The admissible path returns exactly parent.Version+1 in the cognitive id
	// space.
	child, record, err := cognition.ValidatePatchProposal(validProposal(), parent)
	if err != nil {
		t.Fatal(err)
	}
	if record.Verdict != "admitted" || child.Ref.ID != strategy.NaturalCognitiveRefID ||
		child.Ref.Version != strategy.FrozenNaturalPolicy7().Ref.Version+1 {
		t.Fatalf("admitted child %+v record %+v", child, record)
	}
}

func TestSandboxAdmissionRule(t *testing.T) {
	candidate := cognitiveParentGenerationForTest(t)
	ctx := context.Background()

	admit, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 5, patched: 6}, candidate, 24)
	if err != nil || !admit.Admitted || admit.Reason != cognition.SandboxReasonAdmitted {
		t.Fatalf("admit: %+v err %v", admit, err)
	}
	noImprove, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 6, patched: 6}, candidate, 24)
	if err != nil || noImprove.Admitted || noImprove.Reason != cognition.SandboxReasonNoImprovement {
		t.Fatalf("no-improvement: %+v err %v", noImprove, err)
	}
	gating, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 5, patched: 9, gating: true}, candidate, 24)
	if err != nil || gating.Admitted || gating.Reason != cognition.SandboxReasonGating {
		t.Fatalf("gating: %+v err %v", gating, err)
	}
}

func cognitiveParentGenerationForTest(t *testing.T) strategy.NaturalCognitiveGeneration {
	t.Helper()
	gen, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, strategy.FrozenCognitiveParams())
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

// TestRoleBLifecycleApplyMeasureRollback drives propose -> validate -> sandbox
// -> apply -> measure -> mandatory rollback, recording every step.
func TestRoleBLifecycleApplyMeasureRollback(t *testing.T) {
	applier := &fakeApplier{actors: []sim.EntityID{1, 2}}
	sandbox := &fakeSandbox{parent: 5, patched: 6}
	c, l := newRoleBTestController(t, applier, sandbox, &fakeProposer{prop: validProposal()})

	records, err := c.Audit(context.Background(), 24, triggeringEvidence())
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionPatchPropose, cognition.InterventionPatchValidate, cognition.InterventionPatchApply)
	if len(applier.applied) != 1 {
		t.Fatalf("applied %v", applier.applied)
	}
	if head := c.Head().Ref; !strategy.IsCognitiveRef(head) {
		t.Fatalf("head did not advance: %+v", head)
	}

	// The measured window no longer improves: the mandatory rollback fires.
	sandbox.patched = 5
	records, err = c.Audit(context.Background(), 48, nontriggeringEvidence())
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionPatchMeasure, cognition.InterventionPatchRollback, cognition.InterventionNoTriggerAudit)
	if applier.cleared != 1 {
		t.Fatalf("rollback to base not performed: %+v", applier)
	}
	if head := c.Head().Ref; head != strategy.FrozenNaturalPolicy7().Ref {
		t.Fatalf("head after rollback %+v", head)
	}

	all, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	measured := 0
	rolledBack := 0
	for _, rec := range all {
		switch rec.Kind {
		case cognition.InterventionPatchMeasure:
			measured++
			if rec.Result.Status != "measured" {
				t.Fatalf("measure result %+v", rec.Result)
			}
		case cognition.InterventionPatchRollback:
			rolledBack++
			if rec.Result.Status != "rolled-back" {
				t.Fatalf("rollback result %+v", rec.Result)
			}
		}
	}
	if measured != 1 || rolledBack != 1 {
		t.Fatalf("measured %d rolledBack %d", measured, rolledBack)
	}
}

// TestRoleBRejectedPatchTouchesNothing pins that a broken patch is rejected
// before any world effect: the applier is never called and only propose +
// validate(rejected) are recorded.
func TestRoleBRejectedPatchTouchesNothing(t *testing.T) {
	applier := &fakeApplier{actors: []sim.EntityID{1, 2}}
	c, l := newRoleBTestController(t, applier, &fakeSandbox{parent: 5, patched: 6}, &fakeProposer{prop: validProposalWithUnknownParam()})

	headBefore := c.Head().Ref
	records, err := c.Audit(context.Background(), 24, triggeringEvidence())
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionPatchPropose, cognition.InterventionPatchValidate)
	if len(applier.applied) != 0 {
		t.Fatalf("a rejected patch reached the world: %v", applier.applied)
	}
	headAfter := c.Head().Ref
	if headAfter != headBefore {
		t.Fatalf("head changed on rejection: %v -> %v", headBefore, headAfter)
	}
	all, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	rejected := 0
	for _, rec := range all {
		if rec.Kind == cognition.InterventionPatchValidate {
			rejected++
			if rec.Result.Status != "rejected" {
				t.Fatalf("validate result %+v", rec.Result)
			}
		}
	}
	if rejected != 1 {
		t.Fatalf("validate records %d", rejected)
	}
}

// TestRoleBNoTriggerAudit pins the zero-cost no-trigger path.
func TestRoleBNoTriggerAudit(t *testing.T) {
	applier := &fakeApplier{actors: []sim.EntityID{1, 2}}
	c, _ := newRoleBTestController(t, applier, &fakeSandbox{parent: 5, patched: 6}, &fakeProposer{prop: validProposal()})
	records, err := c.Audit(context.Background(), 24, nontriggeringEvidence())
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionNoTriggerAudit)
	if len(applier.applied) != 0 {
		t.Fatal("no-trigger audit touched the world")
	}
}

func validProposalWithUnknownParam() protocol.PatchProposal {
	p := validProposal()
	p.Deltas = []protocol.ParameterDelta{{Param: "claim.unknown", Value: 3}}
	return p
}

func assertKinds(t *testing.T, records []cognition.InterventionRecord, want ...cognition.InterventionKind) {
	t.Helper()
	if len(records) != len(want) {
		t.Fatalf("record count %d, want %d (%+v)", len(records), len(want), records)
	}
	for i, kind := range want {
		if records[i].Kind != kind {
			t.Fatalf("record %d kind %q, want %q", i, records[i].Kind, kind)
		}
	}
}
