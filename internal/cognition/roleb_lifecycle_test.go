package cognition_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

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
	// parentGens records the parent generation passed on each parent-branch
	// call (nil for the frozen base), so a test can pin which branch the
	// sandbox compared against.
	parentGens []*strategy.NaturalCognitiveGeneration
}

func (s *fakeSandbox) RunBranch(_ context.Context, gen *strategy.NaturalCognitiveGeneration, parent *strategy.NaturalCognitiveGeneration) (cognition.SandboxOutcome, error) {
	if gen == nil {
		s.parentGens = append(s.parentGens, parent)
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

	admit, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 5, patched: 6}, nil, candidate, 24)
	if err != nil || !admit.Admitted || admit.Reason != cognition.SandboxReasonAdmitted {
		t.Fatalf("admit: %+v err %v", admit, err)
	}
	noImprove, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 6, patched: 6}, nil, candidate, 24)
	if err != nil || noImprove.Admitted || noImprove.Reason != cognition.SandboxReasonNoImprovement {
		t.Fatalf("no-improvement: %+v err %v", noImprove, err)
	}
	gating, err := cognition.RunSandbox(ctx, &fakeSandbox{parent: 5, patched: 9, gating: true}, nil, candidate, 24)
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

// echoingProposer returns a proposal whose parent ref echoes the request's
// current head, so a multi-audit lifecycle can chain children. Each call
// returns a distinct in-domain delta so every child differs from its parent.
type echoingProposer struct{ calls int }

func (p *echoingProposer) ProposeStrategyPatch(_ context.Context, req protocol.PatchRequest) (protocol.PatchProposal, error) {
	p.calls++
	deltas := []protocol.ParameterDelta{
		{Param: protocol.ParamClaimGatherEnergyFloor, Value: 2},
		{Param: protocol.ParamClaimBankEnergyCeiling, Value: 4},
	}
	return protocol.PatchProposal{
		Propose: true, ParentRef: req.ParentRef,
		Deltas: []protocol.ParameterDelta{deltas[(p.calls-1)%len(deltas)]},
		Intent: "echo the current head", CallRef: uint64(p.calls),
	}, nil
}

// TestRoleBSandboxParentIsPerCall pins spec 10.2: one audit hour resolves a
// pending measurement (its parent branch is the measured generation's parent)
// and then proposes a new candidate (its parent branch is the current head).
// After an apply that is not rolled back the two parents differ, so a fixed
// pre-audit parent would compare the new proposal against the wrong branch.
// Reverting the proposal call to a fixed parent fails this test.
func TestRoleBSandboxParentIsPerCall(t *testing.T) {
	applier := &fakeApplier{actors: []sim.EntityID{1, 2}}
	sandbox := &fakeSandbox{parent: 5, patched: 6}
	c, _ := newRoleBTestController(t, applier, sandbox, &echoingProposer{})

	if _, err := c.Audit(context.Background(), 24, triggeringEvidence()); err != nil {
		t.Fatal(err)
	}
	head := c.Head().Ref
	if !strategy.IsCognitiveRef(head) {
		t.Fatalf("head did not advance after the apply: %+v", head)
	}
	// The measurement at h48 admits, so the apply is not rolled back and the
	// head is still the first child when the new proposal runs.
	if _, err := c.Audit(context.Background(), 48, triggeringEvidence()); err != nil {
		t.Fatal(err)
	}
	if len(sandbox.parentGens) != 3 {
		t.Fatalf("parent branches run=%d, want 3 (%+v)", len(sandbox.parentGens), sandbox.parentGens)
	}
	measureParent := sandbox.parentGens[1]
	if measureParent != nil {
		t.Fatalf("measurement parent branch %+v, want the frozen base (nil)", measureParent)
	}
	proposalParent := sandbox.parentGens[2]
	if proposalParent == nil || proposalParent.Ref != head {
		t.Fatalf("proposal parent branch %+v, want the current head %+v", proposalParent, head)
	}
}

// TestRoleBExtinctAuditResolvesPending pins P2-13: an extinct-roster audit
// resolves a pending measurement with a typed `unavailable` result instead of
// abandoning it, then records the no-effect audit. Dropping the pending
// handling fails this test.
func TestRoleBExtinctAuditResolvesPending(t *testing.T) {
	applier := &fakeApplier{actors: []sim.EntityID{1, 2}}
	sandbox := &fakeSandbox{parent: 5, patched: 6}
	c, _ := newRoleBTestController(t, applier, sandbox, &echoingProposer{})
	if _, err := c.Audit(context.Background(), 24, triggeringEvidence()); err != nil {
		t.Fatal(err)
	}
	records, err := c.AuditExtinct(48)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionPatchMeasure, cognition.InterventionNoTriggerAudit)
	if records[0].Result.Status != "unavailable" || records[0].Result.Reason != "roster-extinct" {
		t.Fatalf("extinct pending measure %+v, want unavailable/roster-extinct", records[0].Result)
	}
	if records[1].Result.Status != "no-effect" {
		t.Fatalf("extinct no-effect %+v", records[1].Result)
	}
	// The second extinct audit has no pending measurement left to resolve.
	records, err = c.AuditExtinct(72)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, records, cognition.InterventionNoTriggerAudit)
}

func validProposalWithUnknownParam() protocol.PatchProposal {
	p := validProposal()
	p.Deltas = []protocol.ParameterDelta{{Param: "claim.unknown", Value: 3}}
	return p
}

// TestRoleBProposalReferencesWholeCallRange pins P2-7/P2-8: the recorded
// proposal carries the declared proposer identity and references every
// physical attempt of a retried call, so a retry cannot orphan a call record or
// understate cost. Using only the used attempt's seq fails this test.
func TestRoleBProposalReferencesWholeCallRange(t *testing.T) {
	l := roleBTestLedger(t)
	attempts := 0
	fake := &cognition.FakeClient{Respond: func(req cognition.ModelRequest) (cognition.ModelResponse, error) {
		attempts++
		if attempts < 3 {
			return cognition.ModelResponse{}, errors.New("transport down")
		}
		var wire struct {
			ParentRef struct {
				ID      string `json:"id"`
				Version uint32 `json:"version"`
			} `json:"parent_ref"`
		}
		if err := json.Unmarshal(req.Request, &wire); err != nil {
			return cognition.ModelResponse{}, err
		}
		body := fmt.Sprintf(`{"schema_version":1,"propose":true,"parent_ref":{"id":%q,"version":%d},"deltas":[{"param":"claim.gather_energy_floor","value":2}],"intent":"retry"}`,
			wire.ParentRef.ID, wire.ParentRef.Version)
		return cognition.ModelResponse{Body: []byte(body), Content: []byte(body),
			Usage: cognition.Usage{PromptTokens: 1, CompletionTokens: 1}, Status: cognition.CallStatusOK}, nil
	}}
	acct, err := cognition.NewCognitiveAccount(generousBudget())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: l, Client: fake, Budget: acct, Price: testPrice(),
		Now:   func() time.Time { return time.UnixMicro(1) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	proposer, err := cognition.NewRoleBProposer(rec, "gw.example/v1/chat/completions", false)
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := cognition.NewRoleBController(cognition.RoleBControllerConfig{
		Ledger: l, Proposer: proposer, Sandbox: &fakeSandbox{parent: 5, patched: 6}, Applier: &fakeApplier{actors: []sim.EntityID{1, 2}}, Operator: "model#test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Start(strategy.FrozenNaturalPolicy7().Ref); err != nil {
		t.Fatal(err)
	}
	if _, err := ctrl.Audit(context.Background(), 24, triggeringEvidence()); err != nil {
		t.Fatal(err)
	}
	records, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	var propose *cognition.InterventionRecord
	for i := range records {
		if records[i].Kind == cognition.InterventionPatchPropose {
			propose = &records[i]
		}
	}
	if propose == nil {
		t.Fatal("no patch-propose record")
	}
	if len(propose.CallRefs) != 3 {
		t.Fatalf("propose record call refs=%v, want all 3 physical attempts", propose.CallRefs)
	}
	if propose.Patch == nil || propose.Patch.Proposer != "model#test#2" {
		t.Fatalf("proposer identity %+v, want model#test#2", propose.Patch)
	}
	for _, rec := range records {
		if rec.Kind == cognition.InterventionPatchValidate && (rec.Validation == nil || rec.Validation.Validator == "") {
			t.Fatalf("validate record has no validator identity: %+v", rec)
		}
	}
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
