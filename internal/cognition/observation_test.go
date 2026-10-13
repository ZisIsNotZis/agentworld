package cognition_test

import (
	"encoding/json"
	"strings"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// sentinelObservation is a fully populated frozen observation. It holds only
// actor-observable values by construction; the encoder test proves the
// canonical request carries those values and nothing else.
func sentinelObservation() strategy.NaturalObservation {
	home, err := sim.EntityRefValue(1001)
	if err != nil {
		panic(err)
	}
	return strategy.NaturalObservation{
		Actor: 5, Ref: strategy.NaturalRef{ID: "natural", Version: 7}, Hour: 6, WorldVersion: 42,
		Energy: sim.IntegerValue(6), Hunger: sim.IntegerValue(2), BagUnits: sim.IntegerValue(0),
		BagSource: home, Capital: sim.IntegerValue(1), Wip: sim.IntegerValue(0),
		GranaryStock: sim.IntegerValue(3), InvestedUnits: sim.IntegerValue(1),
		LastBuildHour: sim.IntegerValue(4), LastStoredMealHour: sim.IntegerValue(2),
		LastDenial: strategy.NaturalNoDenial,
		HomeSlots: []strategy.NaturalSlotView{
			{ID: 2001, Stock: sim.IntegerValue(1)},
			{ID: 2002, Stock: sim.IntegerValue(0)},
		},
	}
}

func sampleSeamRequest() cognition.ActorSeamRequest {
	return cognition.ActorSeamRequest{
		Seam:        cognition.SeamNaturalClaim,
		Actor:       5,
		Hour:        6,
		Version:     42,
		Observation: sentinelObservation(),
		Strategy:    cognition.FrozenNaturalClaimParameters(strategy.FrozenNaturalPolicy7().Ref),
		Actions:     []string{"gather", "eat-stored", "provision", "wait"},
		Identity: cognition.RunIdentity{
			Fixture: cognition.FixtureIdentity{Patches: 2, R: 8, Founders: 12, Mechanism: 6, Births: true},
			Seed:    0, BaseRef: strategy.FrozenNaturalPolicy7().Ref, HeadRef: strategy.FrozenNaturalPolicy7().Ref,
		},
	}
}

func rawKeys(t *testing.T, body []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func keySet(keys []string) map[string]bool {
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// TestActorSeamRequestEncoderIsAllowListed is the encoder guard (spec 6.6 /
// EC-6): the canonical request carries exactly the declared field set, so a
// privileged field cannot be added without editing the encoder and the schema
// version. Mutating the wire struct's field set makes this fail.
func TestActorSeamRequestEncoderIsAllowListed(t *testing.T) {
	body, err := cognition.EncodeActorSeamRequest(sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	got := keySet(rawKeys(t, body))
	wantTop := []string{"schema_version", "role", "seam", "prompt_template", "model", "run",
		"actor", "observation", "strategy", "actions", "sampling"}
	if len(got) != len(wantTop) {
		t.Fatalf("top-level key set = %v, want exactly %v", got, wantTop)
	}
	for _, k := range wantTop {
		if !got[k] {
			t.Fatalf("missing top-level key %q in %v", k, got)
		}
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatal(err)
	}
	obs := keySet(rawKeys(t, top["observation"]))
	wantObs := []string{"energy", "hunger", "bag_units", "capital", "wip", "invested_units",
		"last_build_hour", "granary_stock", "last_stored_meal_hour", "home_patch", "home_slots", "last_denial"}
	if len(obs) != len(wantObs) {
		t.Fatalf("observation key set = %v, want exactly %v", obs, wantObs)
	}
	for _, k := range wantObs {
		if !obs[k] {
			t.Fatalf("missing observation key %q in %v", k, obs)
		}
	}

	// No privileged marker anywhere in the observation. The observation type
	// has no field for tenure, debt, allocator state, R, or another actor's
	// private rows, so the allow-list encoder cannot emit them.
	lower := strings.ToLower(string(top["observation"]))
	for _, marker := range []string{"tenure", "owner", "debt", "spoil", "allocator", "regeneration", "kin", "other", "parent", "genome"} {
		if strings.Contains(lower, marker) {
			t.Fatalf("privileged marker %q found in encoded observation: %s", marker, top["observation"])
		}
	}

	// The declared values are carried, including the derived home patch.
	var decoded struct {
		Observation struct {
			Energy     int64  `json:"energy"`
			HomePatch  uint64 `json:"home_patch"`
			LastDenial string `json:"last_denial"`
			HomeSlots  []struct {
				Slot  uint64 `json:"slot"`
				Stock int64  `json:"stock"`
			} `json:"home_slots"`
		} `json:"observation"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Observation.Energy != 6 || decoded.Observation.HomePatch != 1001 || decoded.Observation.LastDenial != "none" {
		t.Fatalf("observation values wrong: %+v", decoded.Observation)
	}
	if len(decoded.Observation.HomeSlots) != 2 || decoded.Observation.HomeSlots[0].Slot != 2001 {
		t.Fatalf("home slots wrong: %+v", decoded.Observation.HomeSlots)
	}
}

// TestActorSeamRequestEncoderIsDeterministic pins byte-determinism from the
// same request value (the request hash must be stable).
func TestActorSeamRequestEncoderIsDeterministic(t *testing.T) {
	first, err := cognition.EncodeActorSeamRequest(sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := cognition.EncodeActorSeamRequest(sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("encoder is not deterministic:\n%s\n%s", first, second)
	}
}

// TestActorSeamRequestEncoderRefusesUnknownSeam pins the seam gate.
func TestActorSeamRequestEncoderRefusesUnknownSeam(t *testing.T) {
	req := sampleSeamRequest()
	req.Seam = "something.else"
	if _, err := cognition.EncodeActorSeamRequest(req); err == nil {
		t.Fatal("unknown seam accepted")
	}
	req = sampleSeamRequest()
	req.Actions = nil
	if _, err := cognition.EncodeActorSeamRequest(req); err == nil {
		t.Fatal("empty action set accepted")
	}
}

// TestRunIdentityDigestIsStable pins the identity digest derivation.
func TestRunIdentityDigestIsStable(t *testing.T) {
	id := sampleSeamRequest().Identity
	first := cognition.RunIdentityDigest(id)
	second := cognition.RunIdentityDigest(id)
	if first != second {
		t.Fatal("identity digest is not stable")
	}
	other := id
	other.Seed = 1
	if cognition.RunIdentityDigest(other) == first {
		t.Fatal("identity digest ignores the seed")
	}
}

// TestActorSeamRequestEncoderUsesTheDeclaredApplicationModel pins the ticket-22
// repair: the canonical request's model field is the unified declared
// application-model identity, not the superseded zai/glm id.
func TestActorSeamRequestEncoderUsesTheDeclaredApplicationModel(t *testing.T) {
	body, err := cognition.EncodeActorSeamRequest(sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Model != cognition.DefaultModelID {
		t.Fatalf("canonical request model = %q, want the declared application model", decoded.Model)
	}
	if cognition.DefaultModelID != "github_copilot/gpt-6-luna" {
		t.Fatalf("declared application model = %q, want the gateway-exposed underscore id", cognition.DefaultModelID)
	}
}
