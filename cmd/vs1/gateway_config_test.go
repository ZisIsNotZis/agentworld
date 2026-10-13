package main

import (
	"strings"
	"testing"

	"agentworld/internal/cognition"
)

// TestHermeticEndpointNamesTheWireMode pins the cmd/vs1 endpoint/mode
// configuration: the default chat wire reproduces the existing recorded
// endpoint exactly, and the responses wire names itself.
func TestHermeticEndpointNamesTheWireMode(t *testing.T) {
	if got := hermeticEndpoint(cognition.WireChatCompletions); got != "hermetic.local/v1/chat/completions" {
		t.Fatalf("chat endpoint = %q, want the existing spelling", got)
	}
	if got := hermeticEndpoint(cognition.WireResponses); got != "hermetic.local/v1/responses" {
		t.Fatalf("responses endpoint = %q", got)
	}
	if declaredGatewayWire != cognition.WireChatCompletions {
		t.Fatalf("the declared default wire changed: %q", declaredGatewayWire)
	}
}

// TestGatewayProbeRefusesUnlessAuthorized pins that the only gateway-contacting
// command is disabled by default.
func TestGatewayProbeRefusesUnlessAuthorized(t *testing.T) {
	t.Setenv(gatewayProbeAuthorizedEnv, "")
	if code := runGatewayProbe(nil); code != 3 {
		t.Fatalf("unauthorised probe exit = %d, want 3", code)
	}
}

// TestGatewayProbeRequiresURL pins that an authorised probe with no URL is a
// usage error, before any request is made.
func TestGatewayProbeRequiresURL(t *testing.T) {
	if code := runGatewayProbe([]string{"-authorize", "-gateway-url="}); code != 2 {
		t.Fatalf("authorised probe without a URL exit = %d, want 2", code)
	}
}

// TestRenderGatewayContractCarriesNoCredential pins that the probe output names
// the discovered contract and never the credential.
func TestRenderGatewayContractCarriesNoCredential(t *testing.T) {
	contract := cognition.GatewayContract{
		Endpoint: "127.0.0.1:4000/",
		Models: []cognition.GatewayModelContract{{
			ID: "github_copilot/gpt-6-luna", Mode: "responses",
			SupportedEndpoints: []string{"/v1/responses"},
		}},
	}
	out := renderGatewayContract(contract, cognition.DefaultGatewayModel)
	if !strings.Contains(out, "github_copilot/gpt-6-luna") || !strings.Contains(out, "/v1/responses") {
		t.Fatalf("contract output missing the model contract:\n%s", out)
	}
	if strings.Contains(out, "Bearer") || strings.Contains(out, "Authorization") {
		t.Fatalf("contract output leaked credential material:\n%s", out)
	}
}
