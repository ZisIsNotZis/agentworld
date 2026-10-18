package cognition_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// The natural-claim/v4 display-vocabulary contract (ticket 36). These tests pin
// the three properties the contract claims: the request is written in the
// decision vocabulary and carries no inert row, the older contracts' bytes are
// untouched, and the response ladder maps the display label back to the frozen
// internal ID before running the one admissibility ladder.

// goldenV1Request is the exact canonical request the v1 contract produced for
// sampleSeamRequest() before this ticket (captured from this revision's encoder
// and re-verified here): selecting v1 or v2 must reproduce it byte for byte.
const goldenV1Request = `{"schema_version":1,"role":"actor-decision","seam":"natural.claim","prompt_template":"natural-claim/v1","model":"github_copilot/gpt-6-luna","run":{"fixture":{"patches":2,"r":8,"founders":12,"mechanism":6,"births":true},"seed":0,"base_ref":{"id":"natural","version":7},"head_ref":{"id":"natural","version":7},"identity_digest":"4c256fd261e3d8746d021bb554e4fc46d3cb7ad36dc390453a1dd1f950d0bf76"},"actor":{"id":5,"hour":6},"observation":{"energy":6,"hunger":2,"bag_units":0,"capital":1,"wip":0,"invested_units":1,"last_build_hour":4,"granary_stock":3,"last_stored_meal_hour":2,"home_patch":1001,"home_slots":[{"slot":2001,"stock":1},{"slot":2002,"stock":0}],"last_denial":"none"},"strategy":{"parameters":{"bag.build_energy_floor":3,"bag.build_finish_energy_floor":3,"bag.build_start_energy_floor":4,"birth.filing_energy_floor":4,"claim.bank_energy_ceiling":3,"claim.gather_energy_floor":1,"claim.provision_energy_floor":4}},"actions":["gather","eat-stored","provision","wait"],"sampling":{"temperature":0.7,"top_p":1,"max_tokens":128,"seed":null}}`

// goldenSampling is the declared frozen sampling instance of the canonical
// request, spelled once for the v2 suffix comparison.
const goldenSampling = `{"temperature":0.7,"top_p":1,"max_tokens":128,"seed":null}`

// goldenV4Request is the exact canonical request the v4 contract produces for
// sampleSeamRequest(). It is pinned because the ticket-36 live run's ledger holds
// these bytes for every one of its 1 325 calls: a silent encoder change would
// make that run's recorded requests unreproducible while every key-set and
// ladder test still passed.
const goldenV4Request = `{"schema_version":4,"role":"actor-decision","seam":"natural.claim","prompt_template":"natural-claim/v4","model":"github_copilot/gpt-6-luna","run":{"fixture":{"patches":2,"r":8,"founders":12,"mechanism":6,"births":true},"seed":0,"base_ref":{"id":"natural","version":7},"head_ref":{"id":"natural","version":7}},"actor":{"id":5,"hour":6},"observation":{"energy":6,"bag_food_units":0,"capital":1,"build_point_in_progress":0,"build_done_this_hour":false,"granary_food_units":3,"granary_draw_used_this_hour":false,"food_units_waiting_to_pick_up":1,"last_refused_action":"none"},"actions":["take_slot_food_into_bag","eat_granary_food","take_granary_food_into_bag","wait_this_hour"],"sampling":{"temperature":0.7,"top_p":1,"max_tokens":128,"seed":null}}`

// v4Keys is the shared declaration of the v4 contract's wire key sets, the same
// rows VS2-EC-6 checks recorded requests against. This test consumes it rather
// than keeping a second copy, so a rename cannot leave the boundary check
// certifying a set no request carries.
var v4Keys = func() cognition.SeamRequestKeySet {
	keys, ok := cognition.DeclaredSeamRequestKeySet(cognition.ClaimContractV4)
	if !ok {
		panic("natural-claim/v4 has no declared key set")
	}
	return keys
}()

// TestActorSeamRequestV4UsesTheDecisionVocabulary is the deliverable-A pin: no
// cryptic name survives, no policy constant is sent, and every claim action is
// spelled as its display label.
func TestActorSeamRequestV4UsesTheDecisionVocabulary(t *testing.T) {
	body, err := cognition.EncodeActorSeamRequestFor(sampleSeamRequest(), cognition.ClaimContractV4)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatal(err)
	}
	wantTop := v4Keys.Request
	got := keySet(rawKeys(t, body))
	if len(got) != len(wantTop) {
		t.Fatalf("top-level key set = %v, want exactly %v", got, wantTop)
	}
	for _, k := range wantTop {
		if !got[k] {
			t.Fatalf("missing top-level key %q in %v", k, got)
		}
	}
	if _, ok := top["strategy"]; ok {
		t.Fatal("the v4 request must not carry the strategy parameter vector: those seven values are policy constants, not state")
	}
	obs := keySet(rawKeys(t, top["observation"]))
	if len(obs) != len(v4Keys.Observation) {
		t.Fatalf("observation key set = %v, want exactly %v", obs, v4Keys.Observation)
	}
	for _, k := range v4Keys.Observation {
		if !obs[k] {
			t.Fatalf("missing observation key %q in %v", k, obs)
		}
	}
	// The v1 spellings and every inert row are gone from the v4 wire. The list is
	// the declared audit: each entry was checked against the code that reads it.
	for _, dropped := range []string{
		"hunger", "wip", "invested_units", "last_build_hour", "granary_stock",
		"last_stored_meal_hour", "home_patch", "home_slots", "bag_units", "last_denial",
		"identity_digest", "parameters",
	} {
		if strings.Contains(string(body), `"`+dropped+`"`) {
			t.Fatalf("dropped name %q is still on the v4 wire: %s", dropped, body)
		}
	}
	// The run block stays as the per-call run identity; only its own digest (a
	// function of itself) is gone.
	run := keySet(rawKeys(t, top["run"]))
	for _, k := range []string{"fixture", "seed", "base_ref", "head_ref"} {
		if !run[k] {
			t.Fatalf("run key set = %v, missing %q", run, k)
		}
	}
	// The claim actions are the display labels, in the declared claim order.
	var wire struct {
		SchemaVersion  int      `json:"schema_version"`
		PromptTemplate string   `json:"prompt_template"`
		Actions        []string `json:"actions"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.SchemaVersion != cognition.ActorSeamSchemaVersionVocabulary || wire.PromptTemplate != "natural-claim/v4" {
		t.Fatalf("v4 contract identity: %+v", wire)
	}
	if string(body) != goldenV4Request {
		t.Fatalf("v4 request bytes changed:\n got %s\nwant %s", body, goldenV4Request)
	}
	want := []string{"take_slot_food_into_bag", "eat_granary_food", "take_granary_food_into_bag", "wait_this_hour"}
	if strings.Join(wire.Actions, ",") != strings.Join(want, ",") {
		t.Fatalf("actions = %v want %v", wire.Actions, want)
	}
	// Every internal id in the action set has a label and vice versa; an id the
	// world can emit but the table cannot name is an encoder error, not a request.
	req := sampleSeamRequest()
	req.Actions = []string{"gather", "unmapped-action"}
	if _, err := cognition.EncodeActorSeamRequestFor(req, cognition.ClaimContractV4); err == nil {
		t.Fatal("an unmet claim action id must not be encoded")
	}
}

// TestActorSeamRequestV4StatesTheGatedAffordances pins the derivation of the two
// gate booleans and the waiting-food count: they state the affordance, never the
// timestamp behind it.
func TestActorSeamRequestV4StatesTheGatedAffordances(t *testing.T) {
	req := sampleSeamRequest()
	req.Hour = 6
	req.Observation.GranaryStock = sim.IntegerValue(2)
	req.Observation.LastStoredMealHour = sim.IntegerValue(6) // drawn this hour
	req.Observation.LastBuildHour = sim.IntegerValue(5)      // built last hour
	req.Observation.Wip = sim.IntegerValue(1)
	body, err := cognition.EncodeActorSeamRequestFor(req, cognition.ClaimContractV4)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Observation struct {
			GranaryFoodUnits         int64  `json:"granary_food_units"`
			GranaryDrawUsedThisHour  bool   `json:"granary_draw_used_this_hour"`
			BuildDoneThisHour        bool   `json:"build_done_this_hour"`
			BuildPointInProgress     int64  `json:"build_point_in_progress"`
			FoodUnitsWaitingToPickUp int64  `json:"food_units_waiting_to_pick_up"`
			LastRefusedAction        string `json:"last_refused_action"`
		} `json:"observation"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if !wire.Observation.GranaryDrawUsedThisHour {
		t.Fatal("a granary draw at this hour must state the one-draw affordance as used")
	}
	if wire.Observation.BuildDoneThisHour {
		t.Fatal("a build at hour 5 must not read as built this hour at hour 6")
	}
	if wire.Observation.BuildPointInProgress != 1 || wire.Observation.GranaryFoodUnits != 2 {
		t.Fatalf("progress/granary = %+v", wire.Observation)
	}
	if wire.Observation.FoodUnitsWaitingToPickUp != 1 { // the fixture's one stocked home slot
		t.Fatalf("waiting food = %d want 1", wire.Observation.FoodUnitsWaitingToPickUp)
	}
	if wire.Observation.LastRefusedAction != protocol.RefusedActionNone {
		t.Fatalf("refused action = %q", wire.Observation.LastRefusedAction)
	}
}

// TestActorSeamRequestV1AndV2BytesAreUnchanged pins the frozen older contracts:
// selecting v1 or v2 must produce exactly the bytes they produced before this
// ticket, so replay and every recorded fingerprint stand.
func TestActorSeamRequestV1AndV2BytesAreUnchanged(t *testing.T) {
	v1, err := cognition.EncodeActorSeamRequest(sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	if string(v1) != goldenV1Request {
		t.Fatalf("v1 request bytes changed:\n got %s\nwant %s", v1, goldenV1Request)
	}
	v2, err := cognition.EncodeActorSeamRequestFor(sampleSeamRequest(), cognition.ClaimContractV2)
	if err != nil {
		t.Fatal(err)
	}
	// v2 is v1 plus the one declared field and its own identity: the envelope
	// changes and nothing else does.
	wantV2 := strings.Replace(goldenV1Request, `"schema_version":1`, `"schema_version":2`, 1)
	wantV2 = strings.Replace(wantV2, `"natural-claim/v1"`, `"natural-claim/v2"`, 1)
	wantV2 = strings.Replace(wantV2,
		`,"sampling":`,
		`,"admissible_actions":["gather","eat-stored","provision","wait"],"sampling":`, 1)
	if string(v2) != wantV2 {
		t.Fatalf("v2 request bytes changed:\n got %s\nwant %s", v2, wantV2)
	}
	if bytes.Contains(v1, []byte("admissible_actions")) {
		t.Fatal("v1 must not carry the v2 field")
	}
}

// TestActorSeamResponseV4MapsLabelThenRunsTheOneLadder pins the response shape:
// a declared label is mapped back to the internal ID before the single
// admissibility ladder runs, the reason is carried out of the decision path, and
// malformed is reserved for a violated schema.
func TestActorSeamResponseV4MapsLabelThenRunsTheOneLadder(t *testing.T) {
	req := sampleSeamRequest()
	req.Observation.GranaryStock = sim.IntegerValue(0) // no granary draw is possible this hour
	ladder := func(t *testing.T, content string) cognition.ActorSeamDecision {
		t.Helper()
		decision, err := cognition.ValidateActorSeamContentFor([]byte(content), req, cognition.ClaimContractV4.ResponseSchemaVersion())
		if err != nil {
			t.Fatalf("ladder error on %s: %v", content, err)
		}
		return decision
	}
	cases := []struct {
		name       string
		content    string
		wantSource string
		wantAction string
		wantReason cognition.FallbackReason
	}{
		{name: "declared label maps to the internal id", wantSource: cognition.SourceModel, wantAction: "gather",
			content: `{"schema_version":4,"action":"take_slot_food_into_bag","why":"There is food waiting in my slots and my bag is empty."}`},
		{name: "wait is always admissible", wantSource: cognition.SourceModel, wantAction: "wait",
			content: `{"schema_version":4,"action":"wait_this_hour","why":"x"}`},
		{name: "missing reason is not a schema violation", wantSource: cognition.SourceModel, wantAction: "wait",
			content: `{"schema_version":4,"action":"wait_this_hour"}`},
		{name: "granary label with an empty granary is inadmissible", wantSource: cognition.SourceFallback,
			wantReason: cognition.FallbackReasonInadmissible,
			content:    `{"schema_version":4,"action":"eat_granary_food","why":"x"}`},
		{name: "internal id is not a declared label", wantSource: cognition.SourceFallback,
			wantReason: cognition.FallbackReasonInadmissible,
			content:    `{"schema_version":4,"action":"gather","why":"x"}`},
		{name: "v1 fields are a schema violation", wantSource: cognition.SourceFallback,
			wantReason: cognition.FallbackReasonMalformed,
			content:    `{"schema_version":4,"action":"wait_this_hour","rationale":"x","confidence":0.5}`},
		{name: "wrong schema version is malformed", wantSource: cognition.SourceFallback,
			wantReason: cognition.FallbackReasonMalformed,
			content:    `{"schema_version":2,"action":"wait_this_hour","why":"x"}`},
		{name: "an over-long reason is malformed", wantSource: cognition.SourceFallback,
			wantReason: cognition.FallbackReasonMalformed,
			content:    `{"schema_version":4,"action":"wait_this_hour","why":"` + strings.Repeat("x", 513) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := ladder(t, tc.content)
			if decision.Source != tc.wantSource {
				t.Fatalf("source = %q want %q (decision %+v)", decision.Source, tc.wantSource, decision)
			}
			if decision.Action != tc.wantAction {
				t.Fatalf("action = %q want %q", decision.Action, tc.wantAction)
			}
			if tc.wantReason != "" && decision.Reason != string(tc.wantReason) {
				t.Fatalf("reason = %q want %q", decision.Reason, tc.wantReason)
			}
		})
	}
	// The reason is carried out of the decision path: it is recorded, and it
	// never changes what the ladder decides.
	decision := ladder(t, `{"schema_version":4,"action":"take_slot_food_into_bag","why":"Food is waiting."}`)
	if decision.Rationale != "Food is waiting." {
		t.Fatalf("the v4 reason was not carried: %+v", decision)
	}
}

// TestActorSeamV4AndV1AgreeOnAdmissibility pins the "one ladder" claim: for every
// observation shape and every action, the v4 label answer and the v1 id answer
// reach the same verdict, because the label is mapped back before the ladder runs.
func TestActorSeamV4AndV1AgreeOnAdmissibility(t *testing.T) {
	idFor := map[string]string{
		"take_slot_food_into_bag":    "gather",
		"eat_granary_food":           "eat-stored",
		"take_granary_food_into_bag": "provision",
		"wait_this_hour":             "wait",
	}
	observations := []func(*cognition.ActorSeamRequest){
		func(r *cognition.ActorSeamRequest) { r.Observation.GranaryStock = sim.IntegerValue(0) },
		func(r *cognition.ActorSeamRequest) {
			r.Observation.GranaryStock = sim.IntegerValue(3)
			r.Observation.LastStoredMealHour = sim.IntegerValue(-1)
		},
		func(r *cognition.ActorSeamRequest) {
			r.Observation.GranaryStock = sim.IntegerValue(3)
			r.Observation.LastStoredMealHour = sim.IntegerValue(int64(r.Hour))
		},
		func(r *cognition.ActorSeamRequest) { r.Observation.BagUnits = sim.IntegerValue(1) },
		func(r *cognition.ActorSeamRequest) { r.Observation.HomeSlots = nil },
	}
	for i, apply := range observations {
		req := sampleSeamRequest()
		apply(&req)
		for label, id := range idFor {
			v1Content := []byte(`{"schema_version":1,"action":"` + id + `","rationale":"x","confidence":1}`)
			v1, err := cognition.ValidateActorSeamContentFor(v1Content, req, cognition.ClaimContractV1.ResponseSchemaVersion())
			if err != nil {
				t.Fatal(err)
			}
			v4Content := []byte(`{"schema_version":4,"action":"` + label + `","why":"x"}`)
			v4, err := cognition.ValidateActorSeamContentFor(v4Content, req, cognition.ClaimContractV4.ResponseSchemaVersion())
			if err != nil {
				t.Fatal(err)
			}
			if v1.Source != v4.Source || v1.Action != v4.Action || v1.Reason != v4.Reason {
				t.Fatalf("observation %d action %q: v1 %+v vs v4 %+v", i, label, v1, v4)
			}
		}
	}
}

// captureRequestClient is a transport that records the request it was handed.
type captureRequestClient struct{ seen []cognition.ModelRequest }

func (c *captureRequestClient) Complete(_ context.Context, req cognition.ModelRequest) (cognition.ModelResponse, error) {
	c.seen = append(c.seen, req)
	return cognition.ModelResponse{
		Body:    []byte(`{"schema_version":4,"action":"wait_this_hour","why":"x"}`),
		Content: []byte(`{"schema_version":4,"action":"wait_this_hour","why":"x"}`),
		Usage:   cognition.Usage{PromptTokens: 10, CompletionTokens: 5}, Status: cognition.CallStatusOK,
	}, nil
}

// TestRecorderAppliesAndRecordsTheSystemPrompt pins the prompt's provenance: the
// applied system message reaches the transport, is recorded verbatim on the call
// record, and is absent (so the record's bytes are unchanged) when none is
// applied.
func TestRecorderAppliesAndRecordsTheSystemPrompt(t *testing.T) {
	for _, tc := range []struct{ name, prompt string }{
		{name: "applied prompt is sent and recorded", prompt: cognition.NaturalClaimSystemPromptV4(
			cognition.PersonaFor(4, protocol.ActorPortrait{BirthHour: 0, LocusM: 6, LocusY: 6, LocusW: 6}, 1))},
		{name: "no prompt keeps the fixed template", prompt: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ledger, err := cognition.CreateLedger(filepath.Join(dir, "calls.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			account, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{
				MaxCalls: 4, MaxPromptTokens: 1 << 40, MaxCompletionTokens: 1 << 40, MaxCostMicros: 1 << 40,
			})
			if err != nil {
				t.Fatal(err)
			}
			client := &captureRequestClient{}
			recorder, err := cognition.NewRecorder(cognition.RecorderConfig{
				Ledger: ledger, Client: client, Budget: account, Price: cognition.UnpricedPrice{},
				Sleep: func(context.Context, time.Duration) error { return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := recorder.Call(context.Background(), cognition.CallRequest{
				Role: cognition.RoleActorDecision, Hour: 1, Model: cognition.DefaultModelID,
				Endpoint: "gateway.local/v1/responses", Sampling: cognition.FrozenSampling(),
				Request: []byte(`{"schema_version":4}`), SystemPrompt: tc.prompt,
			}); err != nil {
				t.Fatal(err)
			}
			if len(client.seen) != 1 || client.seen[0].SystemPrompt != tc.prompt {
				t.Fatalf("transport saw %+v, want prompt %q", client.seen, tc.prompt)
			}
			data, err := os.ReadFile(filepath.Join(dir, "calls.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			hasField := bytes.Contains(data, []byte(`"system_prompt"`))
			if tc.prompt == "" && hasField {
				t.Fatalf("an unapplied prompt must leave the record unchanged: %s", data)
			}
			if tc.prompt != "" && !hasField {
				t.Fatalf("an applied prompt must be recorded verbatim: %s", data)
			}
		})
	}
}
