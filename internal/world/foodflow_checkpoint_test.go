package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func foodFlowSaveAt(t *testing.T, q int64, steps int) (*FoodFlow, string, [32]byte) {
	t.Helper()
	f, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: 7, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	for range steps {
		processed, err := f.Step(context.Background())
		if err != nil || !processed {
			t.Fatalf("step %d: %v", steps, err)
		}
	}
	path := filepath.Join(t.TempDir(), "pilot.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path, digest
}

func TestFoodFlowCheckpointActiveAndRejectedOnlyContinuation(t *testing.T) {
	for _, tc := range []struct {
		q     int64
		steps int
	}{{3, 0}, {8, 2}, {3, 2}, {3, 3}, {3, 4}, {0, 2}, {0, 24}} {
		t.Run(strconv.FormatInt(tc.q, 10)+"/"+strconv.Itoa(tc.steps), func(t *testing.T) {
			original, path, digest := foodFlowSaveAt(t, tc.q, tc.steps)
			before, err := original.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			got, restoredDigest, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: tc.q, Seed: 7, Workers: 1})
			if err != nil || restoredDigest != digest {
				t.Fatalf("restore: %x %x %v", restoredDigest, digest, err)
			}
			after, err := got.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("handoff did not round trip")
			}
			for range 4 {
				processed, err := original.Step(context.Background())
				if err != nil || !processed {
					t.Fatalf("original step: %v", err)
				}
				processed, err = got.Step(context.Background())
				if err != nil || !processed {
					t.Fatalf("restored step: %v", err)
				}
			}
			one, err := original.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			four, err := got.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(one, four) {
				t.Fatal("continuation differs in event history, scheduler, journal or projections")
			}
			if !reflect.DeepEqual(foodFlowEventBytes(t, original), foodFlowEventBytes(t, got)) {
				t.Fatal("accepted event bytes changed")
			}
			t.Logf("q%d boundary %d: steps=%d events=%d journal=%d", tc.q, tc.steps, one.Steps, len(got.k.Events()), len(one.Journal))
		})
	}
}

func TestFoodFlowCheckpointRejectsS6AndPolicyDrift(t *testing.T) {
	pilot, pilotPath, _ := foodFlowSaveAt(t, 3, 2)
	s6, err := NewSurvival(SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	s6Path := filepath.Join(t.TempDir(), "s6.bundle")
	if _, err := s6.SaveCheckpoint(s6Path); err != nil {
		t.Fatal(err)
	}
	if f, _, err := RestoreFoodFlowCheckpoint(s6Path, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("accepted S6 bundle: %v", err)
	}
	if f, _, err := RestoreSurvivalCheckpoint(pilotPath, SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1}); f != nil || !errors.Is(err, ErrSurvivalCheckpoint) {
		t.Fatalf("S6 accepted pilot: %v", err)
	}
	sections, _, err := checkpoint.ReadFile(pilotPath)
	if err != nil {
		t.Fatal(err)
	}
	modified := append([]checkpoint.Section(nil), sections...)
	modified[4].Data = bytes.Clone(modified[4].Data)
	strategyBytes := modified[4].Data
	first := 10 + int(binary.BigEndian.Uint16(strategyBytes[8:])) + 1 + FoodFlowActorCount*12
	if strategyBytes[first] != byte(strategy.FoodFlowGather) {
		t.Fatal("missing first active Gather")
	}
	strategyBytes[first+1+8+7]++ // another valid visible slot, not the chosen slot
	manifest, err := decodeFoodFlowManifest(modified[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	manifest.sections[3] = sha256.Sum256(strategyBytes)
	modified[2].Data = encodeFoodFlowManifest(manifest)
	changedChoice := filepath.Join(t.TempDir(), "changed-choice.bundle")
	if _, err := checkpoint.WriteFile(changedChoice, modified); err != nil {
		t.Fatal(err)
	}
	if f, _, err := RestoreFoodFlowCheckpoint(changedChoice, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("accepted forged open choice: %v", err)
	}
	original := pilot.bound[0]
	registry := strategy.NewFoodFlowRegistry()
	changed := original.Policy()
	changed.Budget.Candidates--
	if err := registry.Register(changed); err != nil {
		t.Fatal(err)
	}
	pilot.bound[0], err = registry.Bind(original.Binding())
	if err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(t.TempDir(), "altered-policy.bundle")
	if _, err := pilot.SaveCheckpoint(absent); !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("published altered policy: %v", err)
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
}

func TestFoodFlowCheckpointRejectsWrongIdentityAndDamage(t *testing.T) {
	_, path, digest := foodFlowSaveAt(t, 3, 3)
	for _, wrong := range []FoodFlowOptions{{Yield: 8, Seed: 7, Workers: 1}, {Yield: 3, Seed: 8, Workers: 1}, {Yield: 3, Seed: 7, Workers: 0}} {
		if f, _, err := RestoreFoodFlowCheckpoint(path, wrong); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
			t.Fatalf("accepted wrong config %+v: %v", wrong, err)
		}
	}
	sections, got, err := checkpoint.ReadFile(path)
	if err != nil || got != digest {
		t.Fatal(err)
	}
	for i := range sections {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[i].Data = bytes.Clone(bad[i].Data)
		bad[i].Data[len(bad[i].Data)/2] ^= 1
		corrupt := filepath.Join(t.TempDir(), "cross.bundle")
		if _, err := checkpoint.WriteFile(corrupt, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreFoodFlowCheckpoint(corrupt, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
			t.Fatalf("accepted rehashed section %d: %v", i, err)
		}
	}
	// Recompute both store and manifest integrity digests after editing each
	// version: incompatibility is semantic, not merely a damaged checksum.
	for _, offset := range []int{4, 8, 12, 16, 20, 24, 44, 52, 60, 68, 76, 84, 92, 100} {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[2].Data = bytes.Clone(bad[2].Data)
		if offset >= 44 {
			binary.BigEndian.PutUint64(bad[2].Data[offset:], 2)
		} else {
			binary.BigEndian.PutUint32(bad[2].Data[offset:], 2)
		}
		sum := sha256.Sum256(bad[2].Data[:len(bad[2].Data)-32])
		copy(bad[2].Data[len(bad[2].Data)-32:], sum[:])
		changed := filepath.Join(t.TempDir(), "wrong-version.bundle")
		if _, err := checkpoint.WriteFile(changed, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreFoodFlowCheckpoint(changed, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
			t.Fatalf("accepted version offset %d: %v", offset, err)
		}
	}
	policySections := append([]checkpoint.Section(nil), sections...)
	policySections[4].Data = bytes.Clone(policySections[4].Data)
	budgetOffset := 10 + 9 + int(policySections[4].Data[18]) + 4
	policySections[4].Data[budgetOffset]-- // valid same-ref policy, changed budget
	manifest, err := decodeFoodFlowManifest(policySections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	manifest.sections[3] = sha256.Sum256(policySections[4].Data)
	policySections[2].Data = encodeFoodFlowManifest(manifest)
	changedPolicy := filepath.Join(t.TempDir(), "changed-policy.bundle")
	if _, err := checkpoint.WriteFile(changedPolicy, policySections); err != nil {
		t.Fatal(err)
	}
	if f, _, err := RestoreFoodFlowCheckpoint(changedPolicy, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("accepted same-ref altered policy: %v", err)
	}
	_, otherPath, _ := foodFlowSaveAt(t, 3, 4)
	otherSections, _, err := checkpoint.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	bad := append([]checkpoint.Section(nil), sections...)
	bad[0] = otherSections[0]
	cross := filepath.Join(t.TempDir(), "cross-section.bundle")
	if _, err := checkpoint.WriteFile(cross, bad); err != nil {
		t.Fatal(err)
	}
	if f, _, err := RestoreFoodFlowCheckpoint(cross, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("accepted cross-section journal: %v", err)
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{wire[:len(wire)-1], wire[:len(wire)/2], append(bytes.Clone(wire), 0)} {
		bad := filepath.Join(t.TempDir(), "truncated.bundle")
		if err := os.WriteFile(bad, malformed, 0600); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreFoodFlowCheckpoint(bad, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
			t.Fatalf("accepted malformed bundle: %v", err)
		}
	}
	if _, _, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); err != nil {
		t.Fatal(err)
	}
	// A failed no-clobber publication never replaces an existing complete file.
	f, err := NewFoodFlow(FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.SaveCheckpoint(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("clobbered existing destination: %v", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wire, unchanged) {
		t.Fatal("existing bundle modified")
	}
	for _, e := range []string{"bad-dir/pilot.bundle", filepath.Join(t.TempDir(), ".checkpoint-forbidden")} {
		if _, err := f.SaveCheckpoint(e); err == nil {
			t.Fatalf("published unsafe path %s", e)
		}
	}
	// A failed semantic export does not publish even a temporary entry.
	f.current[1] = strategy.FoodFlowRest
	absent := filepath.Join(t.TempDir(), "absent.bundle")
	if _, err := f.SaveCheckpoint(absent); err == nil {
		t.Fatal("accepted inconsistent runner state")
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partially published: %v", err)
	}
}
