package cognition_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"agentworld/internal/cognition"
)

func TestReplayResolvesRecordedCallAndNeverInvokesModel(t *testing.T) {
	l, path := newTestLedger(t)
	request := `{"schema_version":1,"role":"actor-decision"}`
	res, err := newRecorder(t, l, &cognition.FakeClient{}, newAccount(t, generousBudget())).
		Call(context.Background(), sampleRequest(cognition.RoleActorDecision, request, 6))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := cognition.OpenReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Len() != 1 {
		t.Fatalf("replay indexed %d calls", replay.Len())
	}
	got, err := replay.Resolve([]byte(request))
	if err != nil {
		t.Fatal(err)
	}
	if got.Seq != res.Seq || got.Attempt != res.Attempt || got.Status != res.Status || string(got.Content) != string(res.Content) {
		t.Fatalf("replayed %+v, recorded %+v", got, res)
	}
	// The replay transport is dead: a stray live call fails loud rather than
	// spending or diverging.
	if _, err := replay.NetworkClient().Complete(context.Background(), cognition.ModelRequest{}); !errors.Is(err, cognition.ErrReplayNetworkForbidden) {
		t.Fatalf("replay transport is not dead: %v", err)
	}
}

func TestReplayMissIsTypedHardError(t *testing.T) {
	l, path := newTestLedger(t)
	if _, err := newRecorder(t, l, &cognition.FakeClient{}, newAccount(t, generousBudget())).
		Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":"recorded"}`, 6)); err != nil {
		t.Fatal(err)
	}
	replay, err := cognition.OpenReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replay.Resolve([]byte(`{"r":"never recorded"}`)); !errors.Is(err, cognition.ErrLedgerMiss) {
		t.Fatalf("miss must be ErrLedgerMiss, never a live call: %v", err)
	}
}

func TestReplayUsesTheUsedAttemptNotAFailedRetry(t *testing.T) {
	l, path := newTestLedger(t)
	attempts := 0
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		attempts++
		if attempts == 1 {
			return cognition.ModelResponse{}, errors.New("transport down")
		}
		return cognition.DefaultFakeResponse(), nil
	}}
	request := `{"r":"retry"}`
	if _, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).
		Call(context.Background(), sampleRequest(cognition.RoleActorDecision, request, 6)); err != nil {
		t.Fatal(err)
	}
	replay, err := cognition.OpenReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := replay.Resolve([]byte(request))
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempt != 2 || got.Status != cognition.CallStatusOK {
		t.Fatalf("replay returned a failed attempt: %+v", got)
	}
}

func TestReplayRefusesTamperedLedger(t *testing.T) {
	l, path := newTestLedger(t)
	if _, err := newRecorder(t, l, &cognition.FakeClient{}, newAccount(t, generousBudget())).
		Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":"t"}`, 6)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := append(append([]byte(nil), data[:len(data)/2]...), data[len(data)/2+1:]...)
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cognition.OpenReplay(path); err == nil {
		t.Fatal("replay accepted a tampered ledger")
	}
}
