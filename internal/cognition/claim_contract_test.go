package cognition_test

import (
	"encoding/json"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
)

// TestClaimContractV2StatesAdmissibleActions checks the second declared request
// contract: v2 states the observation-derived admissible subset of the legal
// action set, v1 does not (its bytes are unchanged), and the response schema
// version follows the contract.
func TestClaimContractV2StatesAdmissibleActions(t *testing.T) {
	req := sampleSeamRequest()
	// Empty granary: eat-stored and provision are inadmissible this hour, while
	// gather (a stocked home slot) and wait remain admissible.
	req.Observation.GranaryStock = sim.IntegerValue(0)
	v1, err := cognition.EncodeActorSeamRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := cognition.EncodeActorSeamRequestFor(req, cognition.ClaimContractV2)
	if err != nil {
		t.Fatal(err)
	}
	var wire1, wire2 struct {
		SchemaVersion     int      `json:"schema_version"`
		PromptTemplate    string   `json:"prompt_template"`
		Actions           []string `json:"actions"`
		AdmissibleActions []string `json:"admissible_actions"`
	}
	if err := json.Unmarshal(v1, &wire1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(v2, &wire2); err != nil {
		t.Fatal(err)
	}
	if wire1.SchemaVersion != 1 || wire1.PromptTemplate != "natural-claim/v1" || wire1.AdmissibleActions != nil {
		t.Fatalf("v1 contract changed: %+v", wire1)
	}
	if wire2.SchemaVersion != 2 || wire2.PromptTemplate != "natural-claim/v2" {
		t.Fatalf("v2 contract identity: %+v", wire2)
	}
	want := map[string]bool{"gather": true, "wait": true}
	if len(wire2.AdmissibleActions) != len(want) {
		t.Fatalf("admissible set: %v want %v", wire2.AdmissibleActions, want)
	}
	for _, action := range wire2.AdmissibleActions {
		if !want[action] {
			t.Fatalf("admissible set: %v want %v", wire2.AdmissibleActions, want)
		}
	}
	// The response must declare the contract's schema version.
	content := []byte(`{"schema_version":1,"action":"wait","rationale":"x","confidence":1}`)
	if _, err := cognition.ValidateActorSeamContentFor(content, req, cognition.ClaimContractV2.ResponseSchemaVersion()); err != nil {
		t.Fatal(err)
	}
	decision, err := cognition.ValidateActorSeamContentFor(content, req, cognition.ClaimContractV2.ResponseSchemaVersion())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Source != cognition.SourceFallback || decision.Reason != string(cognition.FallbackReasonMalformed) {
		t.Fatalf("schema-1 answer to a v2 request must be malformed, got %+v", decision)
	}
}
