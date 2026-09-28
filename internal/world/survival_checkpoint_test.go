package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type checkpointEvidence struct {
	History      [32]byte
	Journal      [32]byte
	Report       SurvivalReport
	RejectCounts [9]int
}

func survivalEvidence(s *Survival) (checkpointEvidence, error) {
	h, _, err := s.kernel.ExportHistory()
	if err != nil {
		return checkpointEvidence{}, err
	}
	j, err := s.ExportJournal()
	if err != nil {
		return checkpointEvidence{}, err
	}
	r, err := s.Report(0, 0)
	var counts [9]int
	for reason, n := range r.Rejected {
		counts[reason] = n
	}
	r.Rejected = nil // JSON map keys are text labels, not round-trippable enum keys.
	return checkpointEvidence{sha256.Sum256(h), sha256.Sum256(j), r, counts}, err
}

func finishSurvival(t *testing.T, s *Survival) checkpointEvidence {
	t.Helper()
	for {
		ok, err := s.Step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
	}
	e, err := survivalEvidence(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSurvivalCheckpointSubprocess(t *testing.T) {
	if os.Getenv("SURVIVAL_CHECKPOINT_HELPER") != "1" {
		return
	}
	var opts SurvivalOptions
	if err := json.Unmarshal([]byte(os.Getenv("SURVIVAL_CHECKPOINT_OPTIONS")), &opts); err != nil {
		t.Fatal(err)
	}
	s, _, err := RestoreSurvivalCheckpoint(os.Getenv("SURVIVAL_CHECKPOINT_PATH"), opts)
	if err != nil {
		t.Fatal(err)
	}
	for {
		ok, err := s.Step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
	}
	e, err := survivalEvidence(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(e); err != nil {
		t.Fatal(err)
	}
}

func subprocessEvidence(t *testing.T, path string, opts SurvivalOptions) checkpointEvidence {
	t.Helper()
	options, _ := json.Marshal(opts)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSurvivalCheckpointSubprocess$")
	cmd.Env = append(os.Environ(), "SURVIVAL_CHECKPOINT_HELPER=1", "SURVIVAL_CHECKPOINT_PATH="+path, "SURVIVAL_CHECKPOINT_OPTIONS="+string(options))
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess: %v: %s", err, data)
	}
	var e checkpointEvidence
	if err := json.Unmarshal(bytes.SplitN(data, []byte{'\n'}, 2)[0], &e); err != nil {
		t.Fatalf("subprocess output: %v: %s", err, data)
	}
	return e
}

func TestSurvivalCheckpointSubprocessContinuation(t *testing.T) {
	opts := SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 1, EatWeight: 1}
	s, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if ok, err := s.Step(context.Background()); !ok || err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if s.sched.Pending() == 0 {
		t.Fatal("no pending wakes at checkpoint")
	}
	path := filepath.Join(t.TempDir(), "mid-run.checkpoint")
	if _, err := s.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	baseline := finishSurvival(t, s)
	for _, workers := range []int{1, 4} {
		opts.Workers = workers
		actual := subprocessEvidence(t, path, opts)
		if !reflect.DeepEqual(actual, baseline) {
			t.Fatalf("worker count %d changed continuation: baseline=%+v actual=%+v", workers, baseline.Report, actual.Report)
		}
	}
	if !baseline.Report.ReplayOK || baseline.Report.Events == 0 || baseline.RejectCounts[SurvivalCollision] == 0 {
		t.Fatalf("missing trajectory evidence: %+v", baseline.Report)
	}
	t.Logf("history=%x journal=%x metrics=%+v", baseline.History, baseline.Journal, baseline.Report)
}

func TestSurvivalCheckpointBranchAndHistory(t *testing.T) {
	opts := SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 2, EatWeight: 1}
	s, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := s.Step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	prefix := s.Journal()
	prefixEvents := s.kernel.Events()
	path, fork := filepath.Join(t.TempDir(), "parent"), filepath.Join(t.TempDir(), "fork")
	parentDigest, err := s.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	branch, branchDigest, err := BranchSurvivalCheckpoint(path, fork, opts, 0)
	if err != nil || branchDigest == parentDigest {
		t.Fatalf("branch: %x %v", branchDigest, err)
	}
	forkSections, _, err := checkpoint.ReadFile(fork)
	if err != nil {
		t.Fatal(err)
	}
	forkManifest, err := decodeSurvivalManifest(forkSections[2].Data)
	if err != nil || forkManifest.lineage.parent != parentDigest || forkManifest.lineage.compatibility != survivalCompatibilityEatWeight || branch.lineage.parent != branchDigest ||
		!reflect.DeepEqual(branch.Journal(), prefix) || !reflect.DeepEqual(branch.kernel.Events(), prefixEvents) || branch.weight != 1 || branch.activeWeight != 0 {
		t.Fatal("branch lost verified common prefix or original config")
	}
	branched := subprocessEvidence(t, fork, opts)
	original := finishSurvival(t, s)
	if branched.History == original.History || branched.Journal == original.Journal || branched.Report.Final.Food == original.Report.Final.Food || branched.Report.AcceptedEat == original.Report.AcceptedEat {
		t.Fatalf("no post-fork divergence: original=%+v branch=%+v", original.Report, branched.Report)
	}
	if !reflect.DeepEqual(branch.Journal()[:len(prefix)], prefix) {
		t.Fatal("historical rejection/acceptance counts changed")
	}
	restored, _, err := RestoreSurvivalCheckpoint(fork, opts)
	if err != nil {
		t.Fatal(err)
	}
	restoredEvidence := finishSurvival(t, restored)
	if !reflect.DeepEqual(restoredEvidence, branched) {
		t.Fatal("fork continuation changed in subprocess")
	}
	continuedPath := filepath.Join(t.TempDir(), "fork-continued")
	if _, err := restored.SaveCheckpoint(continuedPath); err != nil {
		t.Fatal(err)
	}
	continuedSections, _, err := checkpoint.ReadFile(continuedPath)
	if err != nil {
		t.Fatal(err)
	}
	continuedManifest, err := decodeSurvivalManifest(continuedSections[2].Data)
	if err != nil || continuedManifest.lineage.parent != branchDigest || continuedManifest.lineage.parentHead != forkManifest.head {
		t.Fatal("continued fork lost immediate parent")
	}
	if after := subprocessEvidence(t, continuedPath, opts); !reflect.DeepEqual(after, branched) {
		t.Fatal("continued fork changed deterministic output")
	}
	// Re-encode valid bundles with forged parent state hashes. The fork has
	// the same accepted tip as its parent; continuation has an older parent.
	for _, parent := range []struct {
		name string
		path string
	}{{"same-tip", fork}, {"older-tip", continuedPath}} {
		for _, field := range []string{"snapshot", "projection"} {
			t.Run(parent.name+"-parent-"+field, func(t *testing.T) {
				sections, _, err := checkpoint.ReadFile(parent.path)
				if err != nil {
					t.Fatal(err)
				}
				m, err := decodeSurvivalManifest(sections[2].Data)
				if err != nil || m.lineage.parent == [32]byte{} || (parent.name == "same-tip") != (m.lineage.parentHead.TipID == m.head.TipID) {
					t.Fatal("invalid test parent", err)
				}
				if field == "snapshot" {
					m.lineage.parentHead.SnapshotHash[0]++
				} else {
					m.lineage.parentHead.ProjectionHash[0]++
				}
				sections[2].Data = encodeSurvivalManifest(m)
				bad := filepath.Join(t.TempDir(), "forged-parent")
				if _, err := checkpoint.WriteFile(bad, sections); err != nil {
					t.Fatal(err)
				}
				if got, digest, err := RestoreSurvivalCheckpoint(bad, opts); err == nil || got != nil || digest != [32]byte{} {
					t.Fatal("restored forged parent state", err)
				}
			})
		}
	}
	t.Logf("parent=%x fork=%x original events=%d rejected=%v food=%d; fork events=%d rejected=%v food=%d", parentDigest, branchDigest, original.Report.Events, original.RejectCounts, original.Report.Final.Food, branched.Report.Events, branched.RejectCounts, branched.Report.Final.Food)
}

func TestSurvivalCheckpointRejectsMissingZeroWeightOverride(t *testing.T) {
	opts := SurvivalOptions{Actors: 2, Seed: 7, Hours: 3, Workers: 1, EatWeight: 1}
	s, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "parent")
	fork := filepath.Join(t.TempDir(), "zero-weight-fork")
	if _, err := s.SaveCheckpoint(parent); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BranchSurvivalCheckpoint(parent, fork, opts, 0); err != nil {
		t.Fatal(err)
	}
	sections, _, err := checkpoint.ReadFile(fork)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeSurvivalManifest(sections[2].Data)
	if err != nil || manifest.active != 0 {
		t.Fatal("not a zero-weight fork", err)
	}
	bindings := make(map[sim.EntityID]*strategy.Bound, opts.Actors)
	for a := 1; a <= opts.Actors; a++ {
		b, err := s.policies.Bind(strategy.Binding{Ref: s.ref, Overrides: map[strategy.ParamID]float64{stockWeight: 0}})
		if err != nil {
			t.Fatal(err)
		}
		bindings[sim.EntityID(a)] = b
	}
	encoded, err := s.policies.ExportPortable(bindings)
	if err != nil {
		t.Fatal(err)
	}
	_, decoded, err := strategy.RestorePortable(encoded, s.policies)
	if err != nil {
		t.Fatal("strategy fixture must be independently valid", err)
	}
	for _, b := range decoded {
		if _, hasHunger := b.Binding().Overrides[hungerWeight]; hasHunger {
			t.Fatal("fixture unexpectedly has Eat override")
		}
	}
	sections[4].Data = encoded
	bad := filepath.Join(t.TempDir(), "wrong-binding")
	if _, err := checkpoint.WriteFile(bad, sections); err != nil {
		t.Fatal(err)
	}
	if got, digest, err := RestoreSurvivalCheckpoint(bad, opts); err == nil || got != nil || digest != [32]byte{} {
		t.Fatal("restored fork without explicit Eat override", err)
	}
}

func TestSurvivalCheckpointPendingActivityAndRejectedOnlyTime(t *testing.T) {
	opts := SurvivalOptions{Actors: 2, Seed: 7, Hours: 3, Workers: 1, EatWeight: 1}
	active, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := active.sched.Start(1, hour); err != nil {
		t.Fatal(err)
	}
	before := active.sched.Snapshot()
	activityPath := filepath.Join(t.TempDir(), "activity")
	if _, err := active.SaveCheckpoint(activityPath); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := RestoreSurvivalCheckpoint(activityPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	after := loaded.sched.Snapshot()
	if !reflect.DeepEqual(before.Fibers, after.Fibers) || !reflect.DeepEqual(before.Wakes, after.Wakes) || before.NextToken != after.NextToken {
		t.Fatal("lost pending activity or wake")
	}
	if expected, actual := finishSurvival(t, active), subprocessEvidence(t, activityPath, opts); !reflect.DeepEqual(expected, actual) {
		t.Fatal("pending activity changed continuation")
	}

	// A trusted test evaluator yields valid typed Wait evidence at hour one.
	// Such a timestamp has no accepted tip, yet closes the wake and is durable.
	rejected, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	rejected.sched, err = scheduler.New(rejected.kernel, opts.Workers, func(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
		a := SurvivalAttempt{Actor: ready.Fiber.Actor, Time: ready.At, Key: survivalKey(rejected.ref, ready.At, ready.Fiber.Actor), Ref: rejected.ref,
			Choice: strategy.Choice{Kind: strategy.Wait, Ref: rejected.ref, ObservedVersion: view.Version, Fallback: strategy.NoCandidate}, Status: Rejected, Reason: SurvivalWait}
		if err := rejected.collect(a, false); err != nil {
			return scheduler.Evaluation{}, err
		}
		return scheduler.Evaluation{Effects: []scheduler.Effect{{Kind: scheduler.EffectSchedule, Actor: a.Actor, Wake: scheduler.Wake{Actor: a.Actor, At: ready.At + sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for a := 1; a <= opts.Actors; a++ {
		id := sim.EntityID(a)
		if err := rejected.sched.Register(id); err != nil {
			t.Fatal(err)
		}
		if err := rejected.sched.Schedule(scheduler.Wake{Actor: id, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := rejected.Step(context.Background()); err != nil || !ok {
		t.Fatalf("rejected-only step: %v", err)
	}
	if len(rejected.kernel.Events()) != 0 || rejected.sched.Time() != sim.SimTime(hour) || rejected.sched.Pending() != opts.Actors {
		t.Fatal("rejected-only timestamp changed accepted state or lost wakes")
	}
	path := filepath.Join(t.TempDir(), "rejected-only")
	if _, err := rejected.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	resumed, _, err := RestoreSurvivalCheckpoint(path, opts)
	if err != nil || len(resumed.Journal()) != 1 || len(resumed.Journal()[0].Attempts) != opts.Actors || resumed.sched.Time() != sim.SimTime(hour) {
		t.Fatalf("lost rejected-only timestamp: %v", err)
	}
	if result := finishSurvival(t, resumed); result.RejectCounts[SurvivalWait] != opts.Actors || !result.Report.ReplayOK {
		t.Fatalf("lost historical rejection counts: %+v", result)
	}
}

func TestSurvivalCheckpointRejectsOutOfHorizonSchedulerState(t *testing.T) {
	opts := SurvivalOptions{Actors: 2, Seed: 7, Hours: 3, Workers: 1, EatWeight: 1}
	base, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	basePath := filepath.Join(t.TempDir(), "base")
	if _, err := base.SaveCheckpoint(basePath); err != nil {
		t.Fatal(err)
	}
	sections, _, err := checkpoint.ReadFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []struct {
		name  string
		stage func(*Survival) error
	}{
		{"wake", func(s *Survival) error {
			return s.sched.Schedule(scheduler.Wake{Actor: 1, At: sim.SimTime(opts.Hours+1) * sim.SimTime(hour), Cause: scheduler.WakeAudit})
		}},
		{"activity-deadline-with-in-range-interruption", func(s *Survival) error {
			token, err := s.sched.Start(1, sim.Duration(opts.Hours+1)*hour)
			if err != nil {
				return err
			}
			return s.sched.Interrupt(1, token, sim.SimTime(opts.Hours)*sim.SimTime(hour))
		}},
		{"activity-interruption", func(s *Survival) error {
			token, err := s.sched.Start(1, sim.Duration(opts.Hours+2)*hour)
			if err != nil {
				return err
			}
			return s.sched.Interrupt(1, token, sim.SimTime(opts.Hours+1)*sim.SimTime(hour))
		}},
	} {
		t.Run(variant.name, func(t *testing.T) {
			s, err := NewSurvival(opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := variant.stage(s); err != nil {
				t.Fatal(err)
			}
			portable, head, err := s.sched.ExportPortable()
			if err != nil {
				t.Fatal(err)
			}
			if variant.name == "activity-deadline-with-in-range-interruption" {
				snap := s.sched.Snapshot()
				if len(snap.Wakes) == 0 || snap.Fibers[0].Activity == nil || snap.Fibers[0].Activity.Deadline <= sim.SimTime(opts.Hours)*sim.SimTime(hour) {
					t.Fatal("fixture has no late activity deadline")
				}
				for _, wake := range snap.Wakes {
					if wake.At > sim.SimTime(opts.Hours)*sim.SimTime(hour) {
						t.Fatal("fixture masked by late wake")
					}
				}
			}
			m, err := decodeSurvivalManifest(sections[2].Data)
			if err != nil || m.head != head {
				t.Fatal("fixture changed kernel head", err)
			}
			copySections := append([]checkpoint.Section(nil), sections...)
			copySections[3].Data = portable
			bad := filepath.Join(t.TempDir(), "out-of-horizon")
			if _, err := checkpoint.WriteFile(bad, copySections); err != nil {
				t.Fatal(err)
			}
			if got, digest, err := RestoreSurvivalCheckpoint(bad, opts); err == nil || got != nil || digest != [32]byte{} {
				t.Fatal("restored out-of-horizon scheduler state", err)
			}
		})
	}
}

func TestSurvivalCheckpointAllowsExactHorizonWakeAndActivity(t *testing.T) {
	opts := SurvivalOptions{Actors: 2, Seed: 7, Hours: 3, Workers: 1, EatWeight: 1}
	for _, interrupt := range []bool{false, true} {
		name := "completion"
		if interrupt {
			name = "interruption"
		}
		t.Run(name, func(t *testing.T) {
			s, err := NewSurvival(opts)
			if err != nil {
				t.Fatal(err)
			}
			horizon := sim.SimTime(opts.Hours) * sim.SimTime(hour)
			if err := s.sched.Schedule(scheduler.Wake{Actor: 2, At: horizon, Cause: scheduler.WakeAudit}); err != nil {
				t.Fatal(err)
			}
			token, err := s.sched.Start(1, sim.Duration(horizon))
			if err != nil {
				t.Fatal(err)
			}
			if interrupt {
				if err := s.sched.Interrupt(1, token, horizon); err != nil {
					t.Fatal(err)
				}
			}
			before := s.sched.Snapshot()
			if before.Fibers[0].Activity == nil || before.Fibers[0].Activity.Deadline != horizon {
				t.Fatal("activity not at exact horizon")
			}
			path := filepath.Join(t.TempDir(), "exact-horizon")
			if _, err := s.SaveCheckpoint(path); err != nil {
				t.Fatal(err)
			}
			loaded, _, err := RestoreSurvivalCheckpoint(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			after := loaded.sched.Snapshot()
			if !reflect.DeepEqual(before.Wakes, after.Wakes) || !reflect.DeepEqual(before.Fibers, after.Fibers) {
				t.Fatal("exact-horizon wake or activity lost")
			}
		})
	}
}

func BenchmarkSurvivalCheckpointSaveRestore(b *testing.B) {
	opts := SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 4, EatWeight: 1}
	s, err := NewSurvival(opts)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := s.Step(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
	dir := b.TempDir()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, fmt.Sprintf("checkpoint-%d", i))
		if _, err := s.SaveCheckpoint(path); err != nil {
			b.Fatal(err)
		}
		if _, _, err := RestoreSurvivalCheckpoint(path, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSurvivalCheckpointRejectsMismatchesAndPartialPublish(t *testing.T) {
	opts := SurvivalOptions{Actors: 3, Seed: 9, Hours: 4, Workers: 1, EatWeight: 1}
	s, err := NewSurvival(opts)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "good")
	if _, err := s.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name   string
		change func([]checkpoint.Section) []checkpoint.Section
	}{
		{"manifest-head", func(ss []checkpoint.Section) []checkpoint.Section {
			m, _ := decodeSurvivalManifest(ss[2].Data)
			m.head.Version++
			ss[2].Data = encodeSurvivalManifest(m)
			return ss
		}},
		{"schema-rule", func(ss []checkpoint.Section) []checkpoint.Section {
			m, _ := decodeSurvivalManifest(ss[2].Data)
			m.head.RegistryFingerprint[0]++
			ss[2].Data = encodeSurvivalManifest(m)
			return ss
		}},
		{"projection", func(ss []checkpoint.Section) []checkpoint.Section {
			m, _ := decodeSurvivalManifest(ss[2].Data)
			m.head.ProjectionHash[0]++
			ss[2].Data = encodeSurvivalManifest(m)
			return ss
		}},
		{"format", func(ss []checkpoint.Section) []checkpoint.Section {
			ss[2].Data = append([]byte(nil), ss[2].Data...)
			ss[2].Data[7]++
			return ss
		}},
		{"parent-tip", func(ss []checkpoint.Section) []checkpoint.Section {
			m, _ := decodeSurvivalManifest(ss[2].Data)
			m.lineage.parent[0] = 1
			m.lineage.parentHead = m.head
			m.lineage.parentHead.TipHash[0]++
			ss[2].Data = encodeSurvivalManifest(m)
			return ss
		}},
		{"strategy-policy", func(ss []checkpoint.Section) []checkpoint.Section {
			alternative := strategy.NewRegistry()
			changed := survivalPolicy()
			changed.Defaults[restWeight] = 2
			if err := alternative.Register(changed); err != nil {
				t.Fatal(err)
			}
			bindings := make(map[sim.EntityID]*strategy.Bound)
			for a := 1; a <= opts.Actors; a++ {
				b, err := alternative.Bind(strategy.Binding{Ref: changed.Ref, Overrides: map[strategy.ParamID]float64{hungerWeight: opts.EatWeight}})
				if err != nil {
					t.Fatal(err)
				}
				bindings[sim.EntityID(a)] = b
			}
			encoded, err := alternative.ExportPortable(bindings)
			if err != nil {
				t.Fatal(err)
			}
			ss[4].Data = encoded
			return ss
		}},
		{"strategy-binding", func(ss []checkpoint.Section) []checkpoint.Section {
			bindings := make(map[sim.EntityID]*strategy.Bound, opts.Actors)
			for a := 1; a <= opts.Actors; a++ {
				b, err := s.policies.Bind(strategy.Binding{Ref: s.ref, Overrides: map[strategy.ParamID]float64{hungerWeight: 0}})
				if err != nil {
					t.Fatal(err)
				}
				bindings[sim.EntityID(a)] = b
			}
			encoded, err := s.policies.ExportPortable(bindings)
			if err != nil {
				t.Fatal(err)
			}
			ss[4].Data = encoded
			return ss
		}},
		{"journal", func(ss []checkpoint.Section) []checkpoint.Section {
			ss[0].Data = append([]byte(nil), ss[4].Data...)
			return ss
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			copySections := append([]checkpoint.Section(nil), sections...)
			bad := filepath.Join(t.TempDir(), "bad")
			if _, err := checkpoint.WriteFile(bad, mutation.change(copySections)); err != nil {
				t.Fatal(err)
			}
			if got, _, err := RestoreSurvivalCheckpoint(bad, opts); err == nil || got != nil {
				t.Fatal("returned partially restored world")
			}
		})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(t.TempDir(), "truncated")
	if err := os.WriteFile(truncated, data[:len(data)-1], 0600); err != nil {
		t.Fatal(err)
	}
	if got, _, err := RestoreSurvivalCheckpoint(truncated, opts); err == nil || got != nil {
		t.Fatal("accepted truncation")
	}
	for _, changed := range []func(*SurvivalOptions){
		func(o *SurvivalOptions) { o.Seed++ }, func(o *SurvivalOptions) { o.Hours++ }, func(o *SurvivalOptions) { o.EatWeight = 0 },
	} {
		o := opts
		changed(&o)
		if got, _, err := RestoreSurvivalCheckpoint(path, o); err == nil || got != nil {
			t.Fatal("accepted incompatible run")
		}
	}
	before, err := survivalEvidence(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCheckpoint(path); err == nil {
		t.Fatal("replaced published checkpoint")
	}
	after, err := survivalEvidence(s)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("changed live world on failed save", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(unchanged, data) {
		t.Fatal("failure changed published checkpoint", err)
	}
}
