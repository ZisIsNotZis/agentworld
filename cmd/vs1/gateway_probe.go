package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"agentworld/internal/cognition"
)

// The gateway-probe subcommand is the only cmd/vs1 path that contacts the real
// gateway, and it is disabled unless explicitly authorised: it performs exactly
// one metadata request (GET /v1/models, no spend) and prints the discovered wire
// contract. The default evaluation path never reaches it and makes no HTTP
// request. A live lane enables it with one flag (or the declared env variable).
const gatewayProbeAuthorizedEnv = "AGENTWORLD_GATEWAY_PROBE_AUTHORIZED"

// runGatewayProbe returns a process exit code: 0 success, 1 probe failure,
// 2 usage error, 3 refused because it was not authorised.
func runGatewayProbe(args []string) int {
	fs := flag.NewFlagSet("gateway-probe", flag.ContinueOnError)
	baseURL := fs.String("gateway-url", os.Getenv(cognition.EnvGatewayBaseURL),
		"gateway base URL (default $"+cognition.EnvGatewayBaseURL+")")
	apiKey := fs.String("api-key", os.Getenv(cognition.EnvGatewayAPIKey),
		"gateway bearer credential (default $"+cognition.EnvGatewayAPIKey+"); never recorded")
	model := fs.String("model", cognition.DefaultGatewayModel, "application model id to inspect")
	authorize := fs.Bool("authorize", os.Getenv(gatewayProbeAuthorizedEnv) == "1",
		"authorize the single metadata GET (no spend)")
	timeout := fs.Duration("timeout", 10*time.Second, "probe request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*authorize {
		fmt.Fprintln(os.Stderr, "vs1 gateway-probe: disabled; pass -authorize or set "+gatewayProbeAuthorizedEnv+"=1")
		return 3
	}
	if strings.TrimSpace(*baseURL) == "" {
		fmt.Fprintf(os.Stderr, "vs1 gateway-probe: no gateway URL (set -gateway-url or $%s)\n", cognition.EnvGatewayBaseURL)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	contract, err := cognition.DiscoverGatewayContract(ctx, cognition.GatewayConfig{BaseURL: *baseURL, APIKey: *apiKey})
	if err != nil {
		fmt.Fprintln(os.Stderr, "vs1 gateway-probe:", err)
		return 1
	}
	fmt.Print(renderGatewayContract(contract, *model))
	return 0
}

// renderGatewayContract prints the discovered contract. It prints the endpoint
// host+path and the metadata only; the credential is never printed or recorded.
func renderGatewayContract(contract cognition.GatewayContract, applicationModel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "endpoint: %s (host+path only; no credential)\n", contract.Endpoint)
	names := make([]string, 0, len(contract.Models))
	for _, m := range contract.Models {
		names = append(names, m.ID)
	}
	sort.Strings(names)
	for _, name := range names {
		m, _ := contract.Model(name)
		fmt.Fprintf(&b, "model: %s\n", m.ID)
		fmt.Fprintf(&b, "  mode: %s\n", m.Mode)
		fmt.Fprintf(&b, "  supported_endpoints: %s\n", strings.Join(m.SupportedEndpoints, ","))
		fmt.Fprintf(&b, "  max_input_tokens: %d max_output_tokens: %d\n", m.MaxInputTokens, m.MaxOutputTokens)
		fmt.Fprintf(&b, "  response_schema=%t reasoning=%t streaming=%t function_calling=%t\n",
			m.SupportsResponseSchema, m.SupportsReasoning, m.SupportsStreaming, m.SupportsFunctionCalling)
		fmt.Fprintf(&b, "  priced: %t\n", m.Priced)
		if mode, err := cognition.WireModeForEndpoints(m.SupportedEndpoints); err == nil {
			fmt.Fprintf(&b, "  declared_wire: %s\n", mode)
		}
	}
	if m, ok := contract.Model(applicationModel); ok {
		if mode, err := cognition.WireModeForEndpoints(m.SupportedEndpoints); err == nil {
			fmt.Fprintf(&b, "application model %s -> wire %s\n", applicationModel, mode)
		}
	}
	return b.String()
}
