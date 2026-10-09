package world

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/strategy"
)

// The Role A seam integration tests (spec section 4). The nil-seam pins are
// captured from the pre-change revision: a nil seam must leave the frozen path
// byte-identical. The escalation tests drive the world through L1's
// deterministic FakeClient.

func naturalCognitionTestLedger(t *testing.T) *cognition.Ledger {
	t.Helper()
	l, err := cognition.CreateLedger(filepath.Join(t.TempDir(), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

// naturalCognitionTestSeam builds a recorded seam whose FakeClient returns the
// given completion content.
func naturalCognitionTestSeam(t *testing.T, l *cognition.Ledger, body string) *cognition.RoleASeam {
	t.Helper()
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		return cognition.ModelResponse{
			Body: []byte(body), Content: []byte(body),
			Usage: cognition.Usage{PromptTokens: 4, CompletionTokens: 2}, Status: cognition.CallStatusOK,
		}, nil
	}}
	acct, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{
		MaxCalls: 1_000_000, MaxPromptTokens: 1_000_000_000,
		MaxCompletionTokens: 1_000_000_000, MaxCostMicros: 1_000_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: l, Client: fake, Budget: acct,
		Price: cognition.PriceTable{Version: "test/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2},
		Now:   func() time.Time { return time.UnixMicro(1) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Ledger: l, Endpoint: "gw.example/v1/chat/completions",
		CredentialPresent: true, WakePolicy: cognition.VS1WakePolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return seam
}

func naturalCognitionTestOptions() NaturalOptions {
	return NaturalOptions{
		Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
	}
}

// TestNaturalNilSeamDigestIdentity is the hard nil-seam guard: the same fixture
// cells must reproduce the pre-change digests exactly. The pinned values were
// captured at revision f8aa81b in a detached worktree with the identical
// world-API call sequence. A change to any frozen code path fails this.
func TestNaturalNilSeamDigestIdentity(t *testing.T) {
	cases := []struct {
		hour int
		want string
	}{
		{24, "948630df4b9f6b9cf4303f64957676475c5d2e35d361e43c19ab3954ed47e563"},
		{168, "6ba160f2d19af1e389863d7c517a23a72d4f3987c582236879b97e6e974eba06"},
	}
	for _, tc := range cases {
		f, err := NewNatural(naturalCognitionTestOptions())
		if err != nil {
			t.Fatal(err)
		}
		naturalRunTo(t, f, tc.hour)
		path := filepath.Join(t.TempDir(), "nil-seam.bundle")
		digest, err := f.SaveResumeCheckpoint(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(digest[:]); got != tc.want {
			t.Fatalf("hour %d: nil-seam digest %s, want pre-change %s", tc.hour, got, tc.want)
		}
	}
}

// TestNaturalCognitionRequiresSeamAndPolicy pins the construction guard: a
// half-wired cognition block is refused before any world exists.
func TestNaturalCognitionRequiresSeamAndPolicy(t *testing.T) {
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{WakePolicy: cognition.VS1WakePolicy()}
	if _, err := NewNatural(opts); err == nil {
		t.Fatal("nil seam accepted")
	}
	opts = naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: stubClaimSeam{}, WakePolicy: cognition.WakePolicy{}}
	if _, err := NewNatural(opts); err == nil {
		t.Fatal("invalid wake policy accepted")
	}
}

type stubClaimSeam struct{}

func (stubClaimSeam) DecideActorSeam(context.Context, cognition.ActorSeamRequest) (cognition.ActorSeamDecision, error) {
	return cognition.ActorSeamDecision{Source: cognition.SourceFallback}, nil
}

// TestNaturalSeamInadmissibleOutputZeroMutation is the zero-mutation guard: a
// seam whose every model output is inadmissible leaves the accepted history
// byte-identical to the frozen run while recording the calls. Deleting the
// level-1 admissibility check makes the model action leak into the world and
// this fails.
func TestNaturalSeamInadmissibleOutputZeroMutation(t *testing.T) {
	frozen, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, frozen, 24)

	l := naturalCognitionTestLedger(t)
	seam := naturalCognitionTestSeam(t, l, `{"schema_version":1,"action":"teleport"}`)
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: cognition.VS1WakePolicy()}
	seamed, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, seamed, 24)

	frozenJournal, err := frozen.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	seamedJournal, err := seamed.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if string(frozenJournal) != string(seamedJournal) {
		t.Fatal("an inadmissible model output mutated the world")
	}
	if !reflect.DeepEqual(frozen.Checkpoints(), seamed.Checkpoints()) {
		t.Fatal("an inadmissible model output changed the checkpoints")
	}
	interventions, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(interventions) == 0 {
		t.Fatal("no escalated decision was recorded")
	}
	for _, rec := range interventions {
		if rec.RoleA == nil || rec.RoleA.Source != cognition.SourceFallback ||
			rec.Result.Status != cognition.RoleAResultRejected || rec.Result.Reason != string(cognition.FallbackReasonInadmissible) {
			t.Fatalf("expected every decision to be a typed fallback: %+v", rec)
		}
		if rec.Cause.Hour%6 != 0 {
			t.Fatalf("decision outside the selected hours: hour %d", rec.Cause.Hour)
		}
	}
}

// TestNaturalSeamEscalatesSelectedHoursAndDiverges proves the model action is
// load-bearing: a seam that always waits removes the gathers the frozen
// cascade would have made, diverging from the frozen run, and every escalation
// is at a selected hour.
func TestNaturalSeamEscalatesSelectedHoursAndDiverges(t *testing.T) {
	frozen, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, frozen, 24)

	l := naturalCognitionTestLedger(t)
	seam := naturalCognitionTestSeam(t, l, `{"schema_version":1,"action":"wait"}`)
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: cognition.VS1WakePolicy()}
	seamed, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, seamed, 24)

	interventions, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	applied := 0
	for _, rec := range interventions {
		if rec.Cause.Hour%6 != 0 {
			t.Fatalf("escalation at a non-selected hour %d", rec.Cause.Hour)
		}
		if rec.RoleA != nil && rec.RoleA.Source == cognition.SourceModel {
			applied++
		}
	}
	if applied == 0 {
		t.Fatal("no model decision was recorded")
	}
	if l.Count() == 0 {
		t.Fatal("the seam recorded no call")
	}
	// The load-bearing decision changes the run: the checkpoint sequences
	// differ from the frozen run. (ExportJournal is a replay-reconciliation
	// guard that a cognitive run cannot satisfy until the replay ledger is
	// wired by the persistence lane, so the divergence is compared directly.)
	if reflect.DeepEqual(frozen.Checkpoints(), seamed.Checkpoints()) {
		t.Fatal("a load-bearing wait seam did not diverge from the frozen run")
	}
}
