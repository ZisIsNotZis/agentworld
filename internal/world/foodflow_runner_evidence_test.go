package world

import (
	"agentworld/internal/sim"
	"reflect"
	"testing"
)

func TestFoodFlowRunnerEventSourceAndSeedVariation(t *testing.T) {
	f := runFoodFlowFixture(t, 3, 0, 4)
	other := runFoodFlowFixture(t, 3, 1, 4)
	if reflect.DeepEqual(foodFlowEventBytes(t, f), foodFlowEventBytes(t, other)) {
		t.Fatal("changing allocation seed did not change accepted event stream")
	}
	for _, ev := range f.Kernel().Events() {
		if ev.RuleVersion != FoodFlowRuleVersion {
			t.Fatalf("rule/metric mismatch event %d", ev.ID)
		}
		switch ev.Rule {
		case FoodFlowProduceRule:
			if !ev.Cause.World {
				t.Fatalf("production actor cause %d", ev.ID)
			}
			patch := -1
			for _, delta := range ev.Deltas {
				index := -1
				if delta.Component == FoodFlowPatchTypeID {
					index = int(delta.Entity - 1001)
				} else if delta.Component == FoodFlowSlotTypeID {
					index = int(delta.Entity-2001) / FoodFlowSlotsPerPatch
				}
				if index < 0 || index >= FoodFlowPatchCount || (patch >= 0 && patch != index) {
					t.Fatalf("production mixed patches %d: %+v", ev.ID, delta)
				}
				patch = index
			}
		case FoodFlowBasalRule:
			if !ev.Cause.World || len(ev.Deltas) != 3 || ev.Deltas[0].Entity < 1 || ev.Deltas[0].Entity > 16 {
				t.Fatalf("basal source %d", ev.ID)
			}
			for _, d := range ev.Deltas {
				if d.Entity != ev.Deltas[0].Entity || d.Component != FoodFlowBodyTypeID {
					t.Fatalf("basal foreign delta %d: %+v", ev.ID, d)
				}
			}
		case FoodFlowGatherRule:
			if ev.Cause.World || len(ev.Deltas) != 5 {
				t.Fatalf("gather provenance %d", ev.ID)
			}
			patch, _ := FoodFlowActorPatchID(ev.Cause.Actor)
			for _, d := range ev.Deltas {
				switch d.Component {
				case FoodFlowSlotTypeID:
					if !foodFlowSlotBelongsToPatch(d.Entity, int(patch-1001)) {
						t.Fatalf("gather foreign slot %d", ev.ID)
					}
				case FoodFlowBagTypeID, FoodFlowBodyTypeID:
					if d.Entity != ev.Cause.Actor {
						t.Fatalf("gather foreign actor %d", ev.ID)
					}
				default:
					t.Fatalf("unexpected gather delta %d: %+v", ev.ID, d)
				}
				if d.Component == FoodFlowBagTypeID && d.Field == FoodFlowBagSourceField {
					source, err := d.After.EntityRef()
					if err != nil || source != patch {
						t.Fatalf("bag missing source %d", ev.ID)
					}
				}
			}
		case FoodFlowConsumeRule:
			patch, _ := FoodFlowActorPatchID(ev.Cause.Actor)
			for _, d := range ev.Deltas {
				if d.Entity != ev.Cause.Actor || (d.Component != FoodFlowBagTypeID && d.Component != FoodFlowBodyTypeID) {
					t.Fatalf("eat foreign state %d", ev.ID)
				}
				if d.Component == FoodFlowBagTypeID && d.Field == FoodFlowBagSourceField {
					source, err := d.Before.EntityRef()
					if err != nil || source != patch || d.After.State() != sim.Missing {
						t.Fatalf("eat source attribution %d", ev.ID)
					}
				}
			}
		default:
			t.Fatalf("unknown rule %d", ev.Rule)
		}
		for _, d := range ev.Deltas {
			if d.Field == FoodFlowBagSourceField && d.Component == FoodFlowBagTypeID {
				continue
			}
			n, err := d.After.Integer()
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, m := range ev.Metrics {
				if m.After.Provenance().Entity != d.Entity || m.After.Provenance().Component != d.Component || m.After.ProjectionID() != sim.ProjectionID(d.Field) {
					continue
				}
				found++
				projected, err := m.After.Value().Scalar()
				if err != nil || projected != float64(n) || m.After.ProjectionVersion() != FoodFlowProjectionVersion || m.After.Provenance().WorldVersion != ev.AfterVersion || m.After.Provenance().SchemaVersion != FoodFlowSchemaVersion {
					t.Fatalf("wrong numerical projection %d: %+v", ev.ID, m)
				}
			}
			if found != 1 {
				t.Fatalf("delta missing projection %d: %+v", ev.ID, d)
			}
		}
	}
	for _, h := range []int{1, 4, 12, 24, 72, 120, 168} {
		c := f.Checkpoints()[h]
		t.Logf("q3 h%d alive=%d patch=[%+v %+v] actor1=%+v actor8=%+v actor16=%+v slot2001=%+v slot2009=%+v", h, c.Alive, c.Patches[0], c.Patches[1], c.Actors[0], c.Actors[7], c.Actors[15], c.Slots[0][0], c.Slots[1][0])
	}
	// Check the h0 scarce per-patch production event commits three distinct
	// slot units, not one shared cache winner or a q-based gathering capacity.
	for _, p := range []sim.EntityID{1001, 1002} {
		var produced, gathered int
		for _, ev := range f.Kernel().Events() {
			if ev.Time >= sim.SimTime(FoodFlowHour) {
				break
			}
			if ev.Rule == FoodFlowProduceRule {
				for _, d := range ev.Deltas {
					if d.Component == FoodFlowSlotTypeID && foodFlowSlotBelongsToPatch(d.Entity, int(p-1001)) {
						produced++
					}
				}
			}
			if ev.Rule == FoodFlowGatherRule {
				owner, _ := FoodFlowActorPatchID(ev.Cause.Actor)
				if owner == p {
					gathered++
				}
			}
		}
		if produced != 3 || gathered != 3 {
			t.Fatalf("h0 patch%d produced=%d gathered=%d", p, produced, gathered)
		}
	}
}
