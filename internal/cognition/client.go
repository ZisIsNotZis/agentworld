// Package cognition implements the recorded model-call boundary promised by
// ADR 0006: a hermetic model-client stack (a deterministic in-process fake, a
// spend-gated gateway stub that cannot reach the network, and a replay client
// that answers from the ledger), an append-only hash-chained recorded-call
// ledger, the intervention-record shape, a fail-loud cognitive budget, and the
// canonical codecs that pin their wire.
//
// The package is deliberately outside internal/world. The world package never
// imports it and this package never imports the world: the actor-decision seam
// is an interface typed with internal/sim and internal/strategy values only,
// and the observation boundary is enforced structurally (see
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

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// Declared protocol identity. The gateway model and the sampling parameters
// are part of run identity (spec section 6).
const (
	DefaultModelID = "zai/glm-5.3-flash"

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
	Status              CallStatus
	StartedAtUnixMicros int64
	LatencyMicros       int64
}

// RecorderConfig wires the client stack. Ledger, Client, Budget, and Price are
// required. The zero values of the harness bounds take the declared defaults.
type RecorderConfig struct {
	Ledger         *Ledger
	Client         ModelClient
	Budget         *CognitiveAccount
	Price          PriceTable
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
	price          PriceTable
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
		if status == CallStatusOversized {
			sum := sha256.Sum256(resp.Body)
			digest, length = formatHash(sum), len(resp.Body)
			body, content = nil, nil
		}
		cost, err := r.price.CostMicros(resp.Usage)
		if err != nil {
			return CallResult{}, err
		}
		rec, err := r.ledger.AppendCall(CallRecord{
			Role:                req.Role,
			RequestHash:         formatHash(requestHash),
			RequestBytes:        req.Request,
			ResponseBody:        body,
			Content:             content,
			ModelID:             req.Model,
			Endpoint:            req.Endpoint,
			CredentialPresent:   req.CredentialPresent,
			Sampling:            req.Sampling,
			Usage:               resp.Usage,
			CostMicros:          cost,
			PriceVersion:        r.price.Version,
			Attempt:             attempt,
			Status:              status,
			HTTPStatus:          httpStatus,
			ResponseDigest:      digest,
			ResponseLength:      length,
			StartedAtUnixMicros: started.UnixMicro(),
			LatencyMicros:       latency.Microseconds(),
		})
		if err != nil {
			return CallResult{}, err
		}
		if err := r.budget.Charge(req.Hour, resp.Usage, cost); err != nil {
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

// GatewayConfig is the environment-derived configuration of the real
// OpenAI-compatible LiteLLM gateway. It is never recorded: only the endpoint
// host+path and whether a credential was present reach the ledger, and no
// secret material is ever recorded.
type GatewayConfig struct {
	BaseURL           string
	CredentialPresent bool
	// Authorized is the spend authorization. It is false in VS1 L1: no spend
	// has been authorized, so the adapter is a stub that cannot call a model.
	Authorized bool
}

// GatewayClient is the config-gated gateway adapter. In this lane it is a stub
// with no network dependency at all: an unauthorized client refuses with a
// typed error, and an authorized one refuses because the transport is not
// built yet. Either way it is incapable of making a network call.
type GatewayClient struct {
	endpoint          string
	credentialPresent bool
	authorized        bool
}

// NewGatewayClient records the endpoint host+path (no query, no credential)
// and returns the stub. It never performs I/O.
func NewGatewayClient(cfg GatewayConfig) (*GatewayClient, error) {
	host, path, err := splitEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	return &GatewayClient{endpoint: host + path, credentialPresent: cfg.CredentialPresent, authorized: cfg.Authorized}, nil
}

// Endpoint is the recorded host+path of the gateway.
func (g *GatewayClient) Endpoint() string { return g.endpoint }

// CredentialPresent reports whether a credential was configured, for the
// ledger's credential_present field.
func (g *GatewayClient) CredentialPresent() bool { return g.credentialPresent }

// Complete always fails: the stub is incapable of a network call.
func (g *GatewayClient) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	if !g.authorized {
		return ModelResponse{}, fmt.Errorf("%w: VS1 spend is not authorized", ErrGatewayUnauthorized)
	}
	return ModelResponse{}, fmt.Errorf("%w: the L1 lane ships only the config-gated stub", ErrGatewayUnimplemented)
}

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

// ParamID names one declared strategy parameter (Appendix C). The parameter
// contract and its domains are the patch lane's; this package only carries the
// identity.
type ParamID string

// FixtureIdentity is the declared experiment fixture (spec 6.2).
type FixtureIdentity struct {
	Patches   int    `json:"patches"`
	R         int    `json:"r"`
	Founders  int    `json:"founders"`
	Mechanism uint32 `json:"mechanism"`
	Births    bool   `json:"births"`
}

// RunIdentity is the recorded run identity block. IdentityDigest binds it.
type RunIdentity struct {
	Fixture        FixtureIdentity     `json:"fixture"`
	Seed           uint64              `json:"seed"`
	BaseRef        strategy.NaturalRef `json:"-"`
	HeadRef        strategy.NaturalRef `json:"-"`
	IdentityDigest [32]byte            `json:"-"`
}

// ActorSeamRequest is the declared Role A request (Appendix A). It is built
// only from strategy values; Observation is the frozen actor-visible value and
// Strategy is the actor's own bound parameter vector.
type ActorSeamRequest struct {
	Seam        string
	Actor       sim.EntityID
	Hour        int
	Version     sim.WorldVersion
	Observation strategy.NaturalObservation
	Strategy    map[ParamID]int64
	Actions     []string
	Identity    RunIdentity
}

// ActorSeamDecision is the declared Role A outcome (Appendix A). Source is
// "model" or "fallback"; Reason is the typed fallback reason.
type ActorSeamDecision struct {
	Action     string
	Rationale  string
	Confidence float64
	Source     string
	CallRef    uint64
	Reason     string
}

// TrajectoryEvidence is the Role B aggregate packet (spec 5.1). It carries only
// world-level aggregate/state rows: no actor-private state and no other
// actor's private state.
type TrajectoryEvidence struct {
	WindowStartHour int              `json:"window_start_hour"`
	WindowHours     int              `json:"window_hours"`
	AliveByHour     []int            `json:"alive_by_hour"`
	BirthsTotal     int64            `json:"births_total"`
	BirthsByCause   map[string]int64 `json:"births_by_cause"`
	DeathsTotal     int64            `json:"deaths_total"`
	DeathsByCause   map[string]int64 `json:"deaths_by_cause"`
	Actors          []ActorEvidence  `json:"actors"`
	Patches         []PatchEvidence  `json:"patches"`
	BuildsCommitted int64            `json:"builds_committed"`
	MealsCommitted  int64            `json:"meals_committed"`
	MaxAlive        int              `json:"max_alive"`
	MaxBirths       int64            `json:"max_births"`
	MaxCapital      int64            `json:"max_capital"`
}

// ActorEvidence is one actor's published capital and granary holding.
type ActorEvidence struct {
	Actor        sim.EntityID `json:"actor"`
	Capital      int64        `json:"capital"`
	GranaryStock int64        `json:"granary_stock"`
}

// PatchEvidence is one patch's published mouths and total capital.
type PatchEvidence struct {
	Patch   sim.EntityID `json:"patch"`
	Mouths  int64        `json:"mouths"`
	Capital int64        `json:"capital"`
}

// ParameterSpec declares one patchable parameter (Appendix C).
type ParameterSpec struct {
	ID            ParamID `json:"id"`
	Meaning       string  `json:"meaning"`
	DecisionPoint string  `json:"decision_point"`
	ParentValue   int64   `json:"parent_value"`
	DomainMin     int64   `json:"domain_min"`
	DomainMax     int64   `json:"domain_max"`
}

// ParameterDelta is one typed integer delta over the declared contract.
type ParameterDelta struct {
	Param ParamID `json:"param"`
	Value int64   `json:"value"`
}

// PatchRequest is the declared Role B request (Appendix A).
type PatchRequest struct {
	Role       string
	AuditHour  int
	Evidence   TrajectoryEvidence
	ParentRef  strategy.NaturalRef
	Parameters []ParameterSpec
	Identity   RunIdentity
}

// PatchProposal is the declared Role B outcome (Appendix A).
type PatchProposal struct {
	Propose   bool
	ParentRef strategy.NaturalRef
	Deltas    []ParameterDelta
	Intent    string
	CallRef   uint64
}

// SeamDecider is the model-call protocol interface (spec 6.1, Appendix A). It
// is defined where it is consumed and implemented by the cognition client
// stack; only internal/sim and internal/strategy types appear in its requests.
type SeamDecider interface {
	DecideActorSeam(ctx context.Context, req ActorSeamRequest) (ActorSeamDecision, error)
	ProposeStrategyPatch(ctx context.Context, req PatchRequest) (PatchProposal, error)
}
