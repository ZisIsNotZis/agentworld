package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHistoryAcceptedKeysAcrossBatches(t *testing.T) {
	k, registry, _ := historyFixture(t, component.BuiltinStorage)
	f := fixtures()[0]
	_, auth, _ := k.Snapshot()
	plans := make([]Plan, 0, 2)
	for _, p := range []Proposal{proposal(f, "a-winner", 1, scalar(t, .1)), proposal(f, "b-loser", 1, scalar(t, .9))} {
		plan, err := k.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	// Alphabetic winner must publish only its own key; the collision loser
	// remains reusable at a later time.
	plans[0], plans[1] = plans[1], plans[0]
	accepted, err := k.CommitBatch(plans)
	if err != nil || len(accepted) != 1 || accepted[0].Key != "a-winner" {
		t.Fatalf("collision winner: %v %v", accepted, err)
	}
	// The loser was valid but never accepted, so its key remains reusable.
	_, auth, _ = k.Snapshot()
	reused := proposal(f, "a-winner", 2, scalar(t, .2))
	reused.Time = 20
	plan, err := k.Plan(reused, auth)
	if err != nil {
		t.Fatal(err)
	}
	before, beforeHead, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.CommitBatch([]Plan{plan}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("accepted winner reused: %v", err)
	}
	after, afterHead, err := k.ExportHistory()
	if err != nil || beforeHead != afterHead || !bytes.Equal(before, after) {
		t.Fatalf("rejected duplicate modified history: %v", err)
	}
	available := proposal(f, "b-loser", 2, scalar(t, .2))
	available.Time = 20
	plan, err = k.Plan(available, auth)
	if err != nil {
		t.Fatal(err)
	}
	if events, err := k.CommitBatch([]Plan{plan}); err != nil || len(events) != 1 || events[0].Key != "b-loser" {
		t.Fatalf("collision loser key unusable: %v %v", events, err)
	}
	wire, head, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	restored, restoredHead, err := RestoreHistory(registry, wire)
	if err != nil || restoredHead != head {
		t.Fatalf("restore: %v", err)
	}
	copyWire, copyHead, err := restored.ExportHistory()
	if err != nil || copyHead != head || !bytes.Equal(copyWire, wire) {
		t.Fatalf("restored output differs: %v", err)
	}
	_, auth, _ = restored.Snapshot()
	for _, key := range []string{"a-winner", "b-loser"} {
		p := proposal(f, key, 3, scalar(t, .3))
		p.Time = 21
		plan, err := restored.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := restored.CommitBatch([]Plan{plan}); !errors.Is(err, ErrDuplicateKey) {
			t.Fatalf("restored accepted key %q reusable: %v", key, err)
		}
	}
	fresh := proposal(f, "fresh", 3, scalar(t, .4))
	fresh.Time = 21
	plan, err = restored.Plan(fresh, auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.CommitBatch([]Plan{plan, Plan{}}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("invalid batch accepted: %v", err)
	}
	if _, err := restored.CommitBatch([]Plan{plan}); err != nil {
		t.Fatalf("rejected batch poisoned accepted-key index: %v", err)
	}
}

// The same test binary acts as a fresh process. Exchange only portable bytes
// over standard streams; no Go pointer, authority, registry object or file is
// shared between the original kernel and its continuation.
func TestHistorySubprocessRestoreAndContinue(t *testing.T) {
	const childEnv = "AGENTWORLD_KERNEL_HISTORY_CHILD"
	f := fixtures()[0]
	if os.Getenv(childEnv) == "1" {
		wire, err := io.ReadAll(io.LimitReader(os.Stdin, MaxHistoryBytes+1))
		if err != nil {
			t.Fatal(err)
		}
		dynamic := component.EnergyDescriptor()
		dynamic.StorageClass = component.DynamicStorage
		builder := component.NewRegistryBuilder()
		if err := builder.Register(dynamic); err != nil {
			t.Fatal(err)
		}
		registry, err := builder.Freeze()
		if err != nil {
			t.Fatal(err)
		}
		k, _, err := RestoreHistory(registry, wire)
		if err != nil {
			t.Fatal(err)
		}
		_, auth, _ := k.Snapshot()
		value, err := sim.ScalarValue(.6)
		if err != nil {
			t.Fatal(err)
		}
		p := proposal(f, "continued", 2, value)
		p.Time = 20
		plan, err := k.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := k.CommitBatch([]Plan{plan}); err != nil {
			t.Fatal(err)
		}
		continued, _, err := k.ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("PORTABLE_HISTORY:%s\n", base64.StdEncoding.EncodeToString(continued))
		return
	}
	original, registry, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ := original.Snapshot()
	plan, err := original.Plan(proposal(f, "initial", 1, scalar(t, .5)), auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := original.CommitBatch([]Plan{plan}); err != nil {
		t.Fatal(err)
	}
	wire, _, err := original.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHistorySubprocessRestoreAndContinue$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	cmd.Stdin = bytes.NewReader(wire)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess restore: %v: %s", err, output)
	}
	const marker = "PORTABLE_HISTORY:"
	line := ""
	for _, candidate := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(candidate, marker) {
			line = strings.TrimPrefix(candidate, marker)
		}
	}
	if line == "" {
		t.Fatalf("missing subprocess output: %s", output)
	}
	continued, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		t.Fatal(err)
	}
	_, auth, _ = original.Snapshot()
	value := scalar(t, .6)
	p := proposal(f, "continued", 2, value)
	p.Time = 20
	plan, err = original.Plan(p, auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := original.CommitBatch([]Plan{plan}); err != nil {
		t.Fatal(err)
	}
	expected, head, err := original.ExportHistory()
	if err != nil || !bytes.Equal(continued, expected) {
		t.Fatalf("cross-process continuation differs: %v", err)
	}
	_, fromChild, err := RestoreHistory(registry, continued)
	if err != nil || fromChild != head {
		t.Fatalf("subprocess result not restorable: %v", err)
	}
}

// Precompute a valid bounded history outside the timer. Each event uses a new
// timestamp, the worst case for replaying one accepted event per CommitBatch.
func benchmarkHistoryDistinctTimes(b *testing.B, n int) {
	builder := component.NewRegistryBuilder()
	if err := builder.Register(component.EnergyDescriptor()); err != nil {
		b.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		b.Fatal(err)
	}
	k, err := New(registry, 7, []component.ComponentSeed{{Entity: 1, Component: component.EnergyTypeID}})
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < n; i++ {
		_, auth, _ := k.Snapshot()
		v, err := sim.ScalarValue(float64(i%4) / 4)
		if err != nil {
			b.Fatal(err)
		}
		p := Proposal{Key: strconv.Itoa(i), Time: sim.SimTime(i + 1), Cause: Cause{World: true}, Rule: 1, RuleVersion: 1,
			Patches: []component.Patch{{Entity: 1, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: v}}}
		plan, err := k.Plan(p, auth)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := k.CommitBatch([]Plan{plan}); err != nil {
			b.Fatal(err)
		}
	}
	wire, head, err := k.ExportHistory()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		restored, got, err := RestoreHistory(registry, wire)
		if err != nil || got != head || restored.SnapshotHead().TipID != sim.EventID(n) {
			b.Fatalf("distinct-time restore %d: %v", n, err)
		}
	}
}

func BenchmarkHistoryRestoreDistinctTimes1024(b *testing.B) { benchmarkHistoryDistinctTimes(b, 1024) }
func BenchmarkHistoryRestoreDistinctTimes4096(b *testing.B) { benchmarkHistoryDistinctTimes(b, 4096) }
