// Package cognition implements the recorded model-call boundary promised by
// ADR 0006: a hermetic model-client stack (a deterministic in-process fake, a
// spend-gated gateway stub that cannot reach the network, and a replay client
// that answers from the ledger), an append-only hash-chained recorded-call
// ledger, the intervention-record shape, a fail-loud cognitive budget, and the
// canonical codecs that pin their wire.
//
// The package is deliberately outside internal/world. The transport-free
// protocol surface lives in internal/cognition/protocol, so the world package
// consumes the Appendix A types without importing this package's model client,
// gateway stub, or ledger (EC-6); this package imports protocol and never
// imports the world. The observation boundary is enforced structurally (see
// TestCognitionSourcesImportNoNetworkOrWorld). A nil seam therefore leaves
// every existing world code path and every recorded run byte-identical.
package cognition

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"agentworld/internal/cognition/protocol"
)

// Declared protocol identity. The application model and the sampling
// parameters are part of run identity (spec section 6).
const (
	// DefaultModelID is the single declared application-model identity, the id
	// the gateway exposes and sends on the wire. It is the operator-decided
	// `github_copilot/gpt-6-luna` (underscore); the earlier `zai/glm-5.3-flash`
	// choice is superseded. The gateway's configured model comes from
	// $LITELLM_MODEL (DefaultGatewayModel); both spellings are this one value.
	DefaultModelID = "github_copilot/gpt-6-luna"

	RoleActorDecision = "actor-decision"
	RoleStrategyPatch = "strategy-patch"

	// MaxResponseBytes is the spec's 64 KiB bound on a recorded response body.
	MaxResponseBytes = 64 << 10

	// DefaultMaxAttempts, DefaultAttemptTimeout, and DefaultCallDeadline are
	// the declared harness liveness parameters of spec section 6.5. They are
	// experiment bounds, recorded in run identity, never world mechanisms.
	DefaultMaxAttempts    = 3
	DefaultAttemptTimeout = 30 * time.Second
	DefaultCallDeadline   = 90 * time.Second

	// defaultBackoff is the first retry backoff; it doubles per attempt.
	defaultBackoff = 100 * time.Millisecond
)

var (
	ErrModelClient            = errors.New("model client")
	ErrInvalidModelRequest    = errors.New("invalid model request")
	ErrModelUnavailable       = errors.New("model unavailable")
	ErrGatewayUnauthorized    = errors.New("model gateway spend is not authorized")
	ErrGatewayUnimplemented   = errors.New("model gateway transport is not implemented in this lane")
	ErrReplayNetworkForbidden = errors.New("replay is incapable of network access")
)

// ModelIdentityDigest is the declared run-identity digest of a model call:
// SHA-256 over the application model, the declared wire mode, and the sampling
// instance. It binds the checkpoint/manifest identity to the model and wire a
// run actually used, so two runs on different models or wires cannot share an
// identity. The empty wire mode is the declared chat default.
func ModelIdentityDigest(model string, wire WireMode, sampling Sampling) [32]byte {
	if wire == "" {
		wire = WireChatCompletions
	}
	body, err := canonicalJSON(modelIdentityWire{
		Model:       model,
		Wire:        string(wire),
		Temperature: sampling.Temperature,
		TopP:        sampling.TopP,
		MaxTokens:   sampling.MaxTokens,
		Seed:        sampling.Seed,
	})
	if err != nil {
		// The identity block is a fixed, always-encodable struct.
		panic(err)
	}
	return sha256.Sum256(body)
}

type modelIdentityWire struct {
	Model       string  `json:"model"`
	Wire        string  `json:"wire"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	MaxTokens   int     `json:"max_tokens"`
	Seed        *int64  `json:"seed"`
}

// ModelFeatureIdentityDigest is the declared model+feature run-identity digest:
// SHA-256 over the model identity block plus the declared candidate feature-set
// version. It is a NEW, separate digest; ModelIdentityDigest is unchanged, so
// VS1's recorded model/sampling identity digest stays byte-identical. It binds
// the candidate feature set a run's allocation policy used to the run identity,
// so a report names both the model+wires and the feature set.
func ModelFeatureIdentityDigest(model string, wire WireMode, sampling Sampling, featureSet string) [32]byte {
	if wire == "" {
		wire = WireChatCompletions
	}
	base := ModelIdentityDigest(model, wire, sampling)
	body, err := canonicalJSON(modelFeatureIdentityWire{
		Model:               model,
		Wire:                string(wire),
		Temperature:         sampling.Temperature,
		TopP:                sampling.TopP,
		MaxTokens:           sampling.MaxTokens,
		Seed:                sampling.Seed,
		FeatureSet:          featureSet,
		ModelIdentityDigest: formatHash(base),
	})
	if err != nil {
		// The identity block is a fixed, always-encodable struct.
		panic(err)
	}
	return sha256.Sum256(body)
}

type modelFeatureIdentityWire struct {
	Model               string  `json:"model"`
	Wire                string  `json:"wire"`
	Temperature         float64 `json:"temperature"`
	TopP                float64 `json:"top_p"`
	MaxTokens           int     `json:"max_tokens"`
	Seed                *int64  `json:"seed"`
	FeatureSet          string  `json:"feature_set"`
	ModelIdentityDigest string  `json:"model_identity_digest"`
}

// Sampling is the declared sampling state sent with a call. Seed is nil when
// the gateway does not support a seed (the VS1 instance sends none and records
// that fact).
type Sampling struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	MaxTokens   int     `json:"max_tokens"`
	Seed        *int64  `json:"seed"`
}

// FrozenSampling is the declared VS1 sampling instance (spec section 6.4).
func FrozenSampling() Sampling {
	return Sampling{Temperature: 0.7, TopP: 1.0, MaxTokens: 128, Seed: nil}
}

// Usage is the recorded token usage of one attempt.
type Usage struct {
	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
}

// CallStatus is the recorded terminal or retryable outcome of one attempt
// (Appendix B).
type CallStatus string

const (
	CallStatusOK             CallStatus = "ok"
	CallStatusMalformed      CallStatus = "malformed"
	CallStatusInadmissible   CallStatus = "inadmissible"
	CallStatusOversized      CallStatus = "oversized"
	CallStatusTransportError CallStatus = "transport-error"
	CallStatusHTTPError      CallStatus = "http-error"
	CallStatusTimeout        CallStatus = "timeout"
)

// retryable reports whether an attempt with this status may be retried:
// transport errors, timeouts, HTTP 429, and 5xx only. A schema-invalid
// completion is model behaviour and is never retried (spec section 6.5).
func (s CallStatus) retryable(httpStatus int) bool {
	switch s {
	case CallStatusTransportError, CallStatusTimeout:
		return true
	case CallStatusHTTPError:
		return httpStatus == 429 || (httpStatus >= 500 && httpStatus <= 599)
	default:
		return false
	}
}

// HTTPStatusError lets a transport report a non-2xx HTTP outcome as an error.
type HTTPStatusError struct{ Status int }

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("model gateway HTTP status %d", e.Status)
}

// ModelRequest is one physical attempt handed to a transport. Request carries
// the canonical request bytes whose SHA-256 is the recorded request hash.
type ModelRequest struct {
	Model    string
	Endpoint string
	Sampling Sampling
	Request  []byte
}

// ModelResponse is one transport outcome. Body is the full (bounded) response
// body; Content is the extracted completion content.
type ModelResponse struct {
	Body       []byte
	Content    []byte
	Usage      Usage
	Status     CallStatus
	HTTPStatus int
}

// ModelClient is the transport seam implemented by the fake, the gateway
// stub, and any future real adapter. It performs exactly one physical attempt:
// it never retries, never records, and never sees the ledger.
type ModelClient interface {
	Complete(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

// CallRequest is one recorded call: the canonical request plus the identity
// needed to classify and price it.
type CallRequest struct {
	Role              string
	Hour              int
	Model             string
	Endpoint          string
	CredentialPresent bool
	Sampling          Sampling
	Request           []byte
}

// CallResult is the used attempt's outcome, reconstructed identically by
// replay.
type CallResult struct {
	Seq                 uint64
	Attempt             int
	RequestHash         [32]byte
	Content             []byte
	ResponseBody        []byte
	Usage               Usage
	CostMicros          int64
	CostUnpriced        bool
	Status              CallStatus
	StartedAtUnixMicros int64
	LatencyMicros       int64
}

// RecorderConfig wires the client stack. Ledger, Client, Budget, and Price are
// required. Price is a pluggable Pricer so a model with no price entry takes
// the typed unpriced state instead of a fabricated zero. The zero values of the
// harness bounds take the declared defaults.
type RecorderConfig struct {
	Ledger         *Ledger
	Client         ModelClient
	Budget         *CognitiveAccount
	Price          Pricer
	MaxAttempts    int
	AttemptTimeout time.Duration
	CallDeadline   time.Duration
	Now            func() time.Time
	Sleep          func(ctx context.Context, d time.Duration) error
}

// Recorder is the recorded-call path: it checks the budget, performs one to
// MaxAttempts physical attempts, appends one hash-chained call record per
// attempt (flushed before the result is consumed), charges the budget, and
// returns the used attempt.
type Recorder struct {
	ledger         *Ledger
	client         ModelClient
	budget         *CognitiveAccount
	price          Pricer
	maxAttempts    int
	attemptTimeout time.Duration
	callDeadline   time.Duration
	now            func() time.Time
	sleep          func(ctx context.Context, d time.Duration) error
}

// NewRecorder validates and builds the recorded-call path.
func NewRecorder(cfg RecorderConfig) (*Recorder, error) {
	if cfg.Ledger == nil || cfg.Client == nil || cfg.Budget == nil {
		return nil, fmt.Errorf("%w: ledger, client, and budget are required", ErrModelClient)
	}
	if cfg.Price == nil {
		return nil, fmt.Errorf("%w: a pricer is required (use UnpricedPrice for a model with no price entry)", ErrModelClient)
	}
	if err := cfg.Price.Validate(); err != nil {
		return nil, err
	}
	r := &Recorder{
		ledger:         cfg.Ledger,
		client:         cfg.Client,
		budget:         cfg.Budget,
		price:          cfg.Price,
		maxAttempts:    cfg.MaxAttempts,
		attemptTimeout: cfg.AttemptTimeout,
		callDeadline:   cfg.CallDeadline,
		now:            cfg.Now,
		sleep:          cfg.Sleep,
	}
	if r.maxAttempts == 0 {
		r.maxAttempts = DefaultMaxAttempts
	}
	if r.attemptTimeout == 0 {
		r.attemptTimeout = DefaultAttemptTimeout
	}
	if r.callDeadline == 0 {
		r.callDeadline = DefaultCallDeadline
	}
	if r.maxAttempts < 1 {
		return nil, fmt.Errorf("%w: MaxAttempts %d", ErrInvalidModelRequest, r.maxAttempts)
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.sleep == nil {
		r.sleep = sleepContext
	}
	return r, nil
}

// Call performs the recorded call. It appends each attempt's record and flushes
// it before returning (or before charging the budget), so a used call can never
// be missing from the record. Exhausted retries are the typed, fail-loud
// ErrModelUnavailable halt; the persisted ledger is untouched by the caller.
func (r *Recorder) Call(ctx context.Context, req CallRequest) (CallResult, error) {
	if err := validateCallRequest(req); err != nil {
		return CallResult{}, err
	}
	requestHash := sha256.Sum256(req.Request)
	ctx, cancel := context.WithTimeout(ctx, r.callDeadline)
	defer cancel()
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		if err := r.budget.AllowCall(req.Hour); err != nil {
			return CallResult{}, err
		}
		started := r.now()
		resp, completeErr := r.complete(ctx, ModelRequest{
			Model: req.Model, Endpoint: req.Endpoint, Sampling: req.Sampling, Request: req.Request,
		})
		latency := r.now().Sub(started)
		status, httpStatus := classifyCall(resp, completeErr)
		digest, length := "", 0
		body, content := resp.Body, resp.Content
		truncatedPrefix := false
		if status == CallStatusOversized {
			sum := sha256.Sum256(resp.Body)
			digest, length = formatHash(sum), len(resp.Body)
			body, content = nil, nil
			// The transport buffers at most MaxResponseBytes+1 bytes, so this
			// digest and length describe that leading prefix, not the full
			// body; label the scope so an auditor knows what to recompute.
			truncatedPrefix = true
		}
		cost, err := r.price.Cost(resp.Usage)
		if err != nil {
			return CallResult{}, err
		}
		priceVersion := r.price.PriceVersion()
		// `unpriced` is the reserved price-table version; the typed Cost.Unpriced
		// state and the recorded version must agree, so the recorded unpriced
		// flag can be derived from the version without a dead field.
		if cost.Unpriced != (priceVersion == UnpricedPriceVersion) {
			return CallResult{}, fmt.Errorf(
				"%w: pricer is inconsistent: Unpriced=%t but PriceVersion=%q (the reserved unpriced version is %q)",
				ErrInvalidModelRequest, cost.Unpriced, priceVersion, UnpricedPriceVersion)
		}
		rec, err := r.ledger.AppendCall(CallRecord{
			Role:                    req.Role,
			RequestHash:             formatHash(requestHash),
			RequestBytes:            req.Request,
			ResponseBody:            body,
			Content:                 content,
			ModelID:                 r.recordedModel(req.Model),
			Endpoint:                req.Endpoint,
			CredentialPresent:       req.CredentialPresent,
			Sampling:                req.Sampling,
			Usage:                   resp.Usage,
			CostMicros:              cost.Micros,
			PriceVersion:            priceVersion,
			OutputMode:              r.recordedOutputMode(),
			Attempt:                 attempt,
			Status:                  status,
			HTTPStatus:              httpStatus,
			ResponseDigest:          digest,
			ResponseLength:          length,
			ResponseTruncatedPrefix: truncatedPrefix,
			StartedAtUnixMicros:     started.UnixMicro(),
			LatencyMicros:           latency.Microseconds(),
		})
		if err != nil {
			return CallResult{}, err
		}
		if err := r.budget.Charge(req.Hour, resp.Usage, cost.Micros); err != nil {
			return CallResult{}, err
		}
		if !status.retryable(httpStatus) {
			return callResultFrom(rec), nil
		}
		if attempt == r.maxAttempts {
			break
		}
		if err := r.sleep(ctx, backoffFor(attempt)); err != nil {
			return CallResult{}, err
		}
	}
	return CallResult{}, fmt.Errorf("%w: %d attempts exhausted", ErrModelUnavailable, r.maxAttempts)
}

func (r *Recorder) complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, r.attemptTimeout)
	defer cancel()
	return r.client.Complete(attemptCtx, req)
}

// recordedModel is the model id recorded for a call. A transport that addresses
// a declared wire model (the real gateway) is the authority on the model
// actually called; every hermetic transport reports none, so the request's
// declared model is recorded unchanged.
func (r *Recorder) recordedModel(declared string) string {
	if identity, ok := r.client.(TransportIdentity); ok {
		if model := identity.WireModel(); model != "" {
			return model
		}
	}
	return declared
}

// recordedOutputMode is the JSON output mode recorded for a call. A transport
// that requests a declared output mode (the real gateway) is the authority;
// every hermetic transport reports none, so its recorded bytes are unchanged.
func (r *Recorder) recordedOutputMode() string {
	if mode, ok := r.client.(TransportOutputMode); ok {
		return string(mode.OutputMode())
	}
	return ""
}

// classifyCall turns one transport outcome into a recorded status. An oversized
// body is recorded by digest and length and takes the fallback (spec 6.3).
func classifyCall(resp ModelResponse, err error) (CallStatus, int) {
	if err != nil {
		var hs *HTTPStatusError
		switch {
		case errors.As(err, &hs):
			return CallStatusHTTPError, hs.Status
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			return CallStatusTimeout, 0
		default:
			return CallStatusTransportError, 0
		}
	}
	if resp.Status == CallStatusHTTPError {
		return CallStatusHTTPError, resp.HTTPStatus
	}
	if len(resp.Body) > MaxResponseBytes {
		return CallStatusOversized, resp.HTTPStatus
	}
	if resp.Status == "" {
		return CallStatusOK, resp.HTTPStatus
	}
	return resp.Status, resp.HTTPStatus
}

func callResultFrom(rec CallRecord) CallResult {
	hash, _ := parseHashField(rec.RequestHash)
	return CallResult{
		Seq:                 rec.Seq,
		Attempt:             rec.Attempt,
		RequestHash:         hash,
		Content:             rec.Content,
		ResponseBody:        rec.ResponseBody,
		Usage:               rec.Usage,
		CostMicros:          rec.CostMicros,
		CostUnpriced:        rec.PriceVersion == UnpricedPriceVersion,
		Status:              rec.Status,
		StartedAtUnixMicros: rec.StartedAtUnixMicros,
		LatencyMicros:       rec.LatencyMicros,
	}
}

func validateCallRequest(req CallRequest) error {
	if req.Role == "" || req.Model == "" || req.Endpoint == "" || len(req.Request) == 0 {
		return fmt.Errorf("%w: role, model, endpoint, and request bytes are required", ErrInvalidModelRequest)
	}
	if req.Hour < 0 {
		return fmt.Errorf("%w: hour %d", ErrInvalidModelRequest, req.Hour)
	}
	return nil
}

func backoffFor(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 16 {
		attempt = 16
	}
	return time.Duration(1<<(attempt-1)) * defaultBackoff
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// FakeClient is the deterministic in-process client every other lane uses until
// the real gateway is authorized. It never touches the network, never reads a
// clock or a random source, and is a pure function of its request whenever
// Respond is. Respond is where a lane scripts a decision; nil means
// DefaultFakeResponse.
type FakeClient struct {
	Respond func(req ModelRequest) (ModelResponse, error)
	calls   atomic.Uint64
}

// Complete returns a deterministic in-process response.
func (c *FakeClient) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	c.calls.Add(1)
	if c.Respond != nil {
		return c.Respond(req)
	}
	return DefaultFakeResponse(), nil
}

// Calls is the number of physical attempts served, for hermetic tests.
func (c *FakeClient) Calls() uint64 { return c.calls.Load() }

// DefaultFakeResponse is the canned, well-formed actor-decision completion.
func DefaultFakeResponse() ModelResponse {
	body := []byte(`{"schema_version":1,"action":"gather","rationale":"fake","confidence":1}`)
	return ModelResponse{
		Body:    body,
		Content: body,
		Usage:   Usage{PromptTokens: 16, CompletionTokens: 8},
		Status:  CallStatusOK,
	}
}

// The gateway adapter (GatewayConfig, GatewayClient, NewGatewayClient, and
// Complete) lives in gateway.go, the package's single declared network
// boundary. client.go stays network-free so the fake, replay, ledger, and
// recorder paths cannot reach the network.

func splitEndpoint(raw string) (string, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("%w: empty gateway base URL", ErrInvalidModelRequest)
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	host, path := s, "/"
	if i := strings.IndexByte(s, '/'); i >= 0 {
		host, path = s[:i], s[i:]
	}
	if j := strings.IndexByte(path, '?'); j >= 0 {
		path = path[:j]
	}
	if host == "" {
		return "", "", fmt.Errorf("%w: gateway base URL has no host", ErrInvalidModelRequest)
	}
	return host, path, nil
}

// The Appendix A protocol types and the model-call interface are the
// transport-free surface of internal/cognition/protocol; these aliases keep the
// historical cognition spelling for the recorded seam, the codecs, and the
// tests.
type (
	ParamID            = protocol.ParamID
	FixtureIdentity    = protocol.FixtureIdentity
	RunIdentity        = protocol.RunIdentity
	ActorSeamRequest   = protocol.ActorSeamRequest
	ActorSeamDecision  = protocol.ActorSeamDecision
	TrajectoryEvidence = protocol.TrajectoryEvidence
	ActorEvidence      = protocol.ActorEvidence
	PatchEvidence      = protocol.PatchEvidence
	ParameterSpec      = protocol.ParameterSpec
	ParameterDelta     = protocol.ParameterDelta
	PatchRequest       = protocol.PatchRequest
	PatchProposal      = protocol.PatchProposal
	SeamDecider        = protocol.SeamDecider
)
