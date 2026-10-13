package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"agentworld/internal/cognition"
)

// The live-model gate (VS2-EC-9, spec section 9). A live run against the real
// gateway is refused unless the operator both passes the explicit non-default
// -live flag AND sets the declared spend-authorisation environment variable to
// the exact declared token. Without both, the runner refuses to start and makes
// no network call. This lane does not authorise spend; the gate exists so that
// an accidental live run is impossible.

// EnvSpendAuthorization is the operator spend-authorisation environment
// variable. Its value is never recorded.
const EnvSpendAuthorization = "VS2_SPEND_AUTHORIZATION"

// DeclaredSpendToken is the exact token that authorises a live run.
const DeclaredSpendToken = "AUTHORIZED-VS2-SPEND"

// ErrLiveRefused is the typed refusal of a live run without authorisation.
var ErrLiveRefused = errors.New("vs2: live run refused")

// liveSpendRequirement is the refusal message and the EC-9 gate evidence: the
// real-model arm is blocked on spend authorisation.
const liveSpendRequirement = "the real-model arm is blocked on spend authorisation; live VS2 refuses to start without -live AND " +
	EnvSpendAuthorization + "=" + DeclaredSpendToken + "; no spend is authorised for this lane"

// liveConfig carries the real-gateway transport when a live run has been
// explicitly authorised.
type liveConfig struct {
	transport cognition.ModelClient
	price     cognition.Pricer
	endpoint  string
}

// activeLive is nil in hermetic mode (the default) and is set only by main when
// the live gate has passed.
var activeLive *liveConfig

// authorizeLive returns nil in hermetic mode; in live mode it refuses unless the
// declared spend-authorisation token is present.
func authorizeLive(live bool) error {
	if !live {
		return nil
	}
	if os.Getenv(EnvSpendAuthorization) != DeclaredSpendToken {
		return fmt.Errorf("%w: %s", ErrLiveRefused, liveSpendRequirement)
	}
	return nil
}

// newLiveConfig builds the real-gateway transport for an authorised live run:
// the declared application model on its only advertised wire (responses), with
// the base URL defaulting to the declared local gateway and the credential read
// from the environment only. Because the application model is unpriced
// (VS2-EC-8), the recorder takes the typed unpriced pricer, never a fabricated
// cost.
func newLiveConfig() (*liveConfig, error) {
	base := os.Getenv(cognition.EnvGatewayBaseURL)
	if base == "" {
		base = "http://127.0.0.1:4000"
	}
	key := os.Getenv(cognition.EnvGatewayAPIKey)
	client, err := cognition.NewGatewayClient(cognition.GatewayConfig{
		BaseURL: base, APIKey: key, CredentialPresent: key != "",
		Authorized: true, WireMode: cognition.WireResponses,
		Model: cognition.DefaultModelID, OutputMode: cognition.OutputJSON,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return &liveConfig{transport: client, price: cognition.UnpricedPrice{}, endpoint: client.WireEndpoint()}, nil
}
