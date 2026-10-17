package cognition

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The real LiteLLM gateway binding (spec section 6.1, ADR 0006). This file is
// the package's single declared network boundary: it builds the wire request
// for a declared wire mode, performs exactly one HTTP attempt per Complete, and
// decodes the wire body into the same ModelResponse the hermetic transports
// produce. The Recorder owns retry, deadline, budget, ledger, and cost; this
// transport never retries and never records.
//
// The application model's gateway metadata (GET /v1/models, no spend) advertises
// mode=responses with supported_endpoints=['/v1/responses'] only and no
// input/output cost fields. The client therefore speaks both wires, records
// which one it used through the recorded endpoint host+path (no ledger field
// changes), and price computation defaults to the typed unpriced state.

// The environment variable names of the gateway configuration. Their values are
// read from the environment and never recorded; only the endpoint host+path and
// credential_present reach the ledger, and no secret material is ever recorded.
const (
	EnvGatewayBaseURL = "LITELLM_BASE_URL"
	EnvGatewayAPIKey  = "LITELLM_API_KEY"
	EnvGatewayModel   = "LITELLM_MODEL"
	EnvGatewayWire    = "AGENTWORLD_GATEWAY_WIRE"
)

// DefaultGatewayModel is the declared application model id as exposed by the
// gateway metadata. It is the single declared application-model identity
// (DefaultModelID); the gateway exposes the underscore spelling, which is the
// id actually sent on the wire. The gateway's configured model comes from the
// environment (GatewayConfig.Model); both spellings are this one value.
const DefaultGatewayModel = DefaultModelID

// WireMode names one declared gateway wire. The two declared wires differ in
// request path, request body, JSON output mode, and usage extraction.
type WireMode string

const (
	WireChatCompletions WireMode = "chat-completions"
	WireResponses       WireMode = "responses"
)

// Validate requires a declared wire mode.
func (w WireMode) Validate() error {
	switch w {
	case WireChatCompletions, WireResponses:
		return nil
	default:
		return fmt.Errorf("%w: unknown wire mode %q", ErrInvalidModelRequest, w)
	}
}

// ParseWireMode parses a declared wire-mode spelling.
func ParseWireMode(s string) (WireMode, error) {
	w := WireMode(strings.TrimSpace(strings.ToLower(s)))
	if err := w.Validate(); err != nil {
		return "", err
	}
	return w, nil
}

// OutputMode is the declared JSON output mode requested of the gateway (spec
// section 6.4). OutputJSON asks for the gateway's JSON-object mode; OutputPlain
// asks for a plain completion and leaves local extraction to the caller.
type OutputMode string

const (
	OutputJSON  OutputMode = "json_object"
	OutputPlain OutputMode = "text"
)

// Validate requires a declared output mode.
func (o OutputMode) Validate() error {
	switch o {
	case OutputJSON, OutputPlain:
		return nil
	default:
		return fmt.Errorf("%w: unknown output mode %q", ErrInvalidModelRequest, o)
	}
}

// GatewaySystemPrompt is the declared fixed system message of the versioned
// gateway template (spec section 6.2). It carries no world state, so it is
// reproducible from the recorded request alone.
const GatewaySystemPrompt = "You are the agentworld cognition gateway. Reply with exactly one JSON object matching the schema_version declared in the user message; no prose and no code fences."

// wireCodec is one declared gateway wire. It owns the request path, the request
// body construction (including the JSON output mode), and the response
// extraction for its protocol.
type wireCodec interface {
	Mode() WireMode
	Path() string
	Encode(req ModelRequest, model string, output OutputMode) ([]byte, error)
	Decode(body []byte) (wireCompletion, error)
}

// wireCompletion is a decoded 2xx wire body.
type wireCompletion struct {
	Content []byte
	Usage   Usage
}

func codecFor(mode WireMode) (wireCodec, error) {
	return codecForConfig(mode, "", 0)
}

// codecForConfig builds the codec of a wire with the applied reasoning effort
// and output-token bound a live run declared. An empty effort means the field is
// omitted (the model default); a zero bound means the wire's declared default
// bound. The two settings are wire facts, not world mechanisms: they change
// what the request bounds and records, never what the world computes.
func codecForConfig(mode WireMode, effort string, maxOutputTokens int) (wireCodec, error) {
	if maxOutputTokens < 0 {
		return nil, fmt.Errorf("%w: max_output_tokens %d", ErrInvalidModelRequest, maxOutputTokens)
	}
	switch mode {
	case WireChatCompletions:
		return chatCompletionsCodec{maxOutputTokens: maxOutputTokens}, nil
	case WireResponses:
		return responsesCodec{effort: effort, maxOutputTokens: maxOutputTokens}, nil
	default:
		return nil, fmt.Errorf("%w: unknown wire mode %q", ErrInvalidModelRequest, mode)
	}
}

// DeclaredReasoningEfforts are the reasoning-effort values the declared
// application model accepts on the responses wire, measured 2026-10-17:
// `minimal` and `auto` are rejected with HTTP 400, every value below is
// honoured. The ladder is the declared effort axis of the curve study.
var DeclaredReasoningEfforts = []string{"none", "low", "medium", "high", "xhigh", "max"}

// ParseReasoningEffort validates a declared reasoning-effort spelling. The empty
// string means "not sent" (the model's own default), which is a declared state
// and never a fabricated effort.
func ParseReasoningEffort(s string) (string, error) {
	e := strings.TrimSpace(strings.ToLower(s))
	if e == "" {
		return "", nil
	}
	for _, declared := range DeclaredReasoningEfforts {
		if declared == e {
			return e, nil
		}
	}
	return "", fmt.Errorf("%w: reasoning effort %q is not one of %v", ErrInvalidModelRequest, s, DeclaredReasoningEfforts)
}

// chatCompletionsCodec is the OpenAI-compatible chat wire (the existing
// behaviour): POST {base}/v1/chat/completions with a system + user message pair,
// `response_format: {"type":"json_object"}` in JSON mode, and usage under
// `prompt_tokens`/`completion_tokens`.
type chatCompletionsCodec struct {
	// maxOutputTokens is the applied `max_tokens` bound; zero sends the request's
	// declared Sampling.MaxTokens unchanged.
	maxOutputTokens int
}

func (chatCompletionsCodec) Mode() WireMode { return WireChatCompletions }
func (chatCompletionsCodec) Path() string   { return "/v1/chat/completions" }

func (c chatCompletionsCodec) appliedMaxTokens(sampling Sampling) int {
	if c.maxOutputTokens > 0 {
		return c.maxOutputTokens
	}
	return sampling.MaxTokens
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponseFormat struct {
	Type string `json:"type"`
}

type chatRequestBody struct {
	Model          string              `json:"model"`
	Messages       []chatMessage       `json:"messages"`
	Temperature    float64             `json:"temperature"`
	TopP           float64             `json:"top_p"`
	MaxTokens      int                 `json:"max_tokens"`
	Seed           *int64              `json:"seed,omitempty"`
	ResponseFormat *chatResponseFormat `json:"response_format,omitempty"`
}

func (c chatCompletionsCodec) Encode(req ModelRequest, model string, output OutputMode) ([]byte, error) {
	body := chatRequestBody{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: GatewaySystemPrompt},
			{Role: "user", Content: string(req.Request)},
		},
		Temperature: req.Sampling.Temperature,
		TopP:        req.Sampling.TopP,
		MaxTokens:   c.appliedMaxTokens(req.Sampling),
		Seed:        req.Sampling.Seed,
	}
	if output == OutputJSON {
		body.ResponseFormat = &chatResponseFormat{Type: string(OutputJSON)}
	}
	return json.Marshal(body)
}

type chatResponseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     uint64 `json:"prompt_tokens"`
		CompletionTokens uint64 `json:"completion_tokens"`
	} `json:"usage"`
}

func (chatCompletionsCodec) Decode(body []byte) (wireCompletion, error) {
	var resp chatResponseBody
	if err := json.Unmarshal(body, &resp); err != nil {
		return wireCompletion{}, fmt.Errorf("%w: chat completion body: %v", ErrModelClient, err)
	}
	if len(resp.Choices) == 0 {
		return wireCompletion{}, fmt.Errorf("%w: chat completion carried no choices", ErrModelClient)
	}
	return wireCompletion{
		Content: []byte(resp.Choices[0].Message.Content),
		Usage:   Usage{PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens},
	}, nil
}

// responsesCodec is the OpenAI Responses wire (the application model's only
// advertised endpoint): POST {base}/v1/responses with an `input` list,
// `text.format.type = "json_object"` in JSON mode, and usage under
// `input_tokens`/`output_tokens`.
type responsesCodec struct {
	// effort is the applied `reasoning.effort`; empty omits the field and leaves
	// the model its own default.
	effort string
	// maxOutputTokens is the applied `max_output_tokens`; zero sends the wire's
	// declared default bound (responsesOutputTokenBound).
	maxOutputTokens int
}

func (responsesCodec) Mode() WireMode { return WireResponses }
func (responsesCodec) Path() string   { return "/v1/responses" }

// appliedMaxOutputTokens is the outgoing output bound: the run's declared bound
// when one was applied, otherwise the wire's fixed bound.
func (c responsesCodec) appliedMaxOutputTokens() int {
	if c.maxOutputTokens > 0 {
		return c.maxOutputTokens
	}
	return responsesOutputTokenBound
}

type responsesTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesInputItem struct {
	Role    string              `json:"role"`
	Content []responsesTextPart `json:"content"`
}

type responsesTextFormat struct {
	Type string `json:"type"`
}

type responsesText struct {
	Format responsesTextFormat `json:"format"`
}

// responsesReasoning is the declared reasoning block of the responses wire. The
// effort is the only field sent: it is honoured by the declared application
// model (measured 2026-10-17), unlike the sampling parameters above.
type responsesReasoning struct {
	Effort string `json:"effort"`
}

type responsesRequestBody struct {
	Model string               `json:"model"`
	Input []responsesInputItem `json:"input"`
	// Temperature and TopP are never sent on the responses wire: the declared
	// application model (github_copilot/gpt-6-luna) rejects each as an
	// unsupported parameter (HTTP 400 invalid_request_body, ticket 34 smoke
	// gate). The declared Sampling instance stays recorded unchanged on every
	// call record; only the outgoing request construction omits the two fields.
	Temperature     *float64            `json:"temperature,omitempty"`
	TopP            *float64            `json:"top_p,omitempty"`
	MaxOutputTokens int                 `json:"max_output_tokens"`
	Reasoning       *responsesReasoning `json:"reasoning,omitempty"`
	Text            *responsesText      `json:"text,omitempty"`
}

// responsesOutputTokenBound is the fixed outgoing `max_output_tokens` of the
// responses wire. The declared application model (github_copilot/gpt-6-luna)
// is a reasoning model: on /v1/responses the bound covers reasoning + text
// together, and the frozen sampling MaxTokens (128) is consumed entirely by
// reasoning, returning status "incomplete" with no message text (ticket 34
// smoke gate, second wire constraint). Raw probes with the exact recorded
// request payload: 512 -> completed, 227 output tokens (207 reasoning + 20
// text), valid schema JSON; 1024 -> completed, 299; 128 + reasoning effort
// low -> still incomplete (the gateway ignores the reasoning parameter). 512
// is the approved applied bound (~2.3x probe headroom); the declared sampling
// instance stays frozen and recorded unchanged on every call record.
const responsesOutputTokenBound = 512

func (c responsesCodec) Encode(req ModelRequest, model string, output OutputMode) ([]byte, error) {
	body := responsesRequestBody{
		Model: model,
		Input: []responsesInputItem{
			{Role: "system", Content: []responsesTextPart{{Type: "input_text", Text: GatewaySystemPrompt}}},
			{Role: "user", Content: []responsesTextPart{{Type: "input_text", Text: string(req.Request)}}},
		},
		MaxOutputTokens: c.appliedMaxOutputTokens(),
	}
	if c.effort != "" {
		body.Reasoning = &responsesReasoning{Effort: c.effort}
	}
	if output == OutputJSON {
		body.Text = &responsesText{Format: responsesTextFormat{Type: string(OutputJSON)}}
	}
	return json.Marshal(body)
}

type responsesResponseBody struct {
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  uint64 `json:"input_tokens"`
		OutputTokens uint64 `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Decode extracts the assistant text of a /v1/responses body: the first
// output_text part of the first message output item. It is the Responses
// counterpart of the chat body's choices[0].message.content.
func (responsesCodec) Decode(body []byte) (wireCompletion, error) {
	var resp responsesResponseBody
	if err := json.Unmarshal(body, &resp); err != nil {
		return wireCompletion{}, fmt.Errorf("%w: responses body: %v", ErrModelClient, err)
	}
	if resp.Error != nil {
		return wireCompletion{}, fmt.Errorf("%w: responses error %q", ErrModelClient, resp.Error.Message)
	}
	var content string
	found := false
	for _, item := range resp.Output {
		if item.Type != "" && item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" || part.Type == "" {
				content, found = part.Text, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return wireCompletion{}, fmt.Errorf("%w: responses body carried no output_text", ErrModelClient)
	}
	return wireCompletion{
		Content: []byte(content),
		Usage:   Usage{PromptTokens: resp.Usage.InputTokens, CompletionTokens: resp.Usage.OutputTokens},
	}, nil
}

// GatewayConfig is the environment-derived configuration of the real
// OpenAI-compatible LiteLLM gateway. It is never recorded: only the endpoint
// host+path and whether a credential was present reach the ledger, and no
// secret material is ever recorded.
type GatewayConfig struct {
	BaseURL           string
	CredentialPresent bool
	// Authorized is the spend authorization. When false the adapter refuses a
	// call with ErrGatewayUnauthorized and never opens a connection.
	Authorized bool
	// WireMode selects the request/response wire; empty means chat-completions.
	WireMode WireMode
	// Model is the wire model id; empty means DefaultGatewayModel.
	Model string
	// APIKey is the bearer credential. It is never recorded.
	APIKey string
	// OutputMode selects the JSON output mode; empty means OutputJSON.
	OutputMode OutputMode
	// ReasoningEffort is the applied reasoning effort of the wire request; empty
	// omits the field and leaves the model its own default. It is a declared
	// wire setting of a run, validated against DeclaredReasoningEfforts.
	ReasoningEffort string
	// MaxOutputTokens is the applied output-token bound of the wire request; zero
	// sends the wire's own default bound. On the responses wire the bound covers
	// reasoning plus text together.
	MaxOutputTokens int
	// HTTPClient is the transport. A nil client keeps the adapter a
	// network-incapable stub (an authorized call fails with
	// ErrGatewayUnimplemented).
	HTTPClient *http.Client
}

// GatewayClient is the config-gated gateway adapter. Without an HTTPClient it
// is a stub incapable of a network call; with one it speaks exactly one
// declared wire, performs exactly one attempt per Complete, and returns the
// recorder's typed outcomes (HTTP status, transport error, timeout, malformed,
// oversized).
type GatewayClient struct {
	endpoint          string
	wireEndpoint      string
	credentialPresent bool
	authorized        bool
	codec             wireCodec
	model             string
	output            OutputMode
	effort            string
	maxOutputTokens   int
	baseURL           string
	apiKey            string
	httpClient        *http.Client
}

// NewGatewayClient records the endpoint host+path (no query, no credential) and
// the configured wire identity. It performs no I/O; only a non-nil HTTPClient
// gives it a transport.
func NewGatewayClient(cfg GatewayConfig) (*GatewayClient, error) {
	host, path, err := splitEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	mode := cfg.WireMode
	if mode == "" {
		mode = WireChatCompletions
	}
	effort, err := ParseReasoningEffort(cfg.ReasoningEffort)
	if err != nil {
		return nil, err
	}
	if effort != "" && mode != WireResponses {
		return nil, fmt.Errorf("%w: reasoning effort is only applied on the %s wire", ErrInvalidModelRequest, WireResponses)
	}
	codec, err := codecForConfig(mode, effort, cfg.MaxOutputTokens)
	if err != nil {
		return nil, err
	}
	model := cfg.Model
	if model == "" {
		model = DefaultGatewayModel
	}
	output := cfg.OutputMode
	if output == "" {
		output = OutputJSON
	}
	if err := output.Validate(); err != nil {
		return nil, err
	}
	g := &GatewayClient{
		endpoint:          host + path,
		wireEndpoint:      host + joinEndpoint(path, codec.Path()),
		credentialPresent: cfg.CredentialPresent,
		authorized:        cfg.Authorized,
		codec:             codec,
		model:             model,
		output:            output,
		effort:            effort,
		maxOutputTokens:   cfg.MaxOutputTokens,
		apiKey:            cfg.APIKey,
		httpClient:        cfg.HTTPClient,
	}
	if cfg.HTTPClient != nil {
		g.baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}
	return g, nil
}

// Endpoint is the recorded host+path of the gateway base (no query, no
// credential), unchanged from the config-gated stub.
func (g *GatewayClient) Endpoint() string { return g.endpoint }

// CredentialPresent reports whether a credential was configured, for the
// ledger's credential_present field.
func (g *GatewayClient) CredentialPresent() bool { return g.credentialPresent }

// WireEndpoint is the recorded host+path of the configured wire, so the
// recorded endpoint names the wire mode (/v1/chat/completions or /v1/responses).
func (g *GatewayClient) WireEndpoint() string { return g.wireEndpoint }

// WireMode is the configured wire mode.
func (g *GatewayClient) WireMode() WireMode { return g.codec.Mode() }

// WireModel is the configured wire model id; the Recorder records it as the
// model actually addressed.
func (g *GatewayClient) WireModel() string { return g.model }

// OutputMode is the configured JSON output mode. It satisfies
// TransportOutputMode, so the Recorder records the requested mode per call.
func (g *GatewayClient) OutputMode() OutputMode { return g.output }

// AppliedReasoningEffort and AppliedMaxOutputTokens are the reasoning effort and
// output-token bound the wire request actually carries. They satisfy
// TransportAppliedSampling, so every call record names the applied wire settings
// instead of only the frozen declared Sampling instance (the provenance lie the
// ticket-34 ledger carried: sampling.max_tokens 128 recorded, 512 sent). Empty
// effort and zero bound mean the request applied the declared state unchanged.
func (g *GatewayClient) AppliedReasoningEffort() string { return g.effort }

// AppliedMaxOutputTokens is the applied output-token bound of the configured
// wire.
func (g *GatewayClient) AppliedMaxOutputTokens() int {
	if g.maxOutputTokens > 0 {
		return g.maxOutputTokens
	}
	if g.codec.Mode() == WireResponses {
		return responsesOutputTokenBound
	}
	return 0
}

// Complete performs exactly one physical attempt. A refusal (unauthorized or no
// transport) is typed and opens no connection; an HTTP status is returned both
// as a CallStatusHTTPError ModelResponse and as a typed HTTPStatusError so the
// recorder's 429/5xx retry classification is exact. A 2xx body that does not
// decode is the recorded malformed outcome, never a retry.
func (g *GatewayClient) Complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	if !g.authorized {
		return ModelResponse{}, fmt.Errorf("%w: VS1 spend is not authorized", ErrGatewayUnauthorized)
	}
	if g.httpClient == nil {
		return ModelResponse{}, fmt.Errorf("%w: no gateway transport is configured", ErrGatewayUnimplemented)
	}
	if req.Endpoint != "" && req.Endpoint != g.wireEndpoint {
		return ModelResponse{}, fmt.Errorf("%w: request endpoint %q is not the configured wire endpoint %q",
			ErrInvalidModelRequest, req.Endpoint, g.wireEndpoint)
	}
	body, err := g.codec.Encode(req, g.model, g.output)
	if err != nil {
		return ModelResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+g.codec.Path(), bytes.NewReader(body))
	if err != nil {
		return ModelResponse{}, fmt.Errorf("%w: %v", ErrModelClient, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	}
	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return ModelResponse{}, ctx.Err()
		}
		return ModelResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return ModelResponse{}, fmt.Errorf("%w: %v", ErrModelClient, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ModelResponse{Body: raw, Status: CallStatusHTTPError, HTTPStatus: resp.StatusCode},
			&HTTPStatusError{Status: resp.StatusCode}
	}
	completion, decodeErr := g.codec.Decode(raw)
	if decodeErr != nil {
		return ModelResponse{Body: raw, Status: CallStatusMalformed, HTTPStatus: resp.StatusCode}, nil
	}
	return ModelResponse{
		Body: raw, Content: completion.Content, Usage: completion.Usage,
		Status: CallStatusOK, HTTPStatus: resp.StatusCode,
	}, nil
}

// GatewayModelContract is the discovered declared contract of one gateway model
// (GET /v1/models, metadata only). It is used by the disabled-by-default probe
// and never influences a recorded run by itself.
type GatewayModelContract struct {
	ID                      string
	Mode                    string
	SupportedEndpoints      []string
	MaxInputTokens          int64
	MaxOutputTokens         int64
	SupportsResponseSchema  bool
	SupportsReasoning       bool
	SupportsStreaming       bool
	SupportsFunctionCalling bool
	// Priced reports whether the metadata carried input/output cost fields. The
	// application model carries none, so it is unpriced.
	Priced bool
}

// WireModeForEndpoints returns the wire mode implied by the discovered
// supported endpoints. Chat is preferred when the gateway advertises it.
func WireModeForEndpoints(endpoints []string) (WireMode, error) {
	hasChat, hasResponses := false, false
	for _, endpoint := range endpoints {
		switch strings.TrimSpace(endpoint) {
		case "/v1/chat/completions":
			hasChat = true
		case "/v1/responses":
			hasResponses = true
		}
	}
	switch {
	case hasChat:
		return WireChatCompletions, nil
	case hasResponses:
		return WireResponses, nil
	default:
		return "", fmt.Errorf("%w: no declared wire endpoint in %v", ErrInvalidModelRequest, endpoints)
	}
}

// GatewayContract is the discovered declared contract of a gateway: its
// recorded endpoint host+path and every advertised model.
type GatewayContract struct {
	Endpoint string
	Models   []GatewayModelContract
}

// Model returns the advertised contract of an exact model id.
func (c GatewayContract) Model(id string) (GatewayModelContract, bool) {
	for _, model := range c.Models {
		if model.ID == id {
			return model, true
		}
	}
	return GatewayModelContract{}, false
}

type gatewayModelsBody struct {
	Data []struct {
		ID        string `json:"id"`
		ModelInfo struct {
			Mode                    string   `json:"mode"`
			SupportedEndpoints      []string `json:"supported_endpoints"`
			MaxInputTokens          int64    `json:"max_input_tokens"`
			MaxOutputTokens         int64    `json:"max_output_tokens"`
			SupportsResponseSchema  bool     `json:"supports_response_schema"`
			SupportsReasoning       bool     `json:"supports_reasoning"`
			SupportsStreaming       bool     `json:"supports_streaming"`
			SupportsFunctionCalling bool     `json:"supports_function_calling"`
			InputCostPerToken       *float64 `json:"input_cost_per_token"`
			OutputCostPerToken      *float64 `json:"output_cost_per_token"`
		} `json:"model_info"`
	} `json:"data"`
}

// DiscoverGatewayContract performs the declared metadata-only discovery probe:
// one authorized GET {base}/v1/models. It spends nothing and records nothing;
// the caller (cmd/vs1 gateway-probe) prints the result. It refuses, with a
// typed error and no network access, unless cfg.Authorized is set, so the
// probe cannot reach the gateway by any other path. The base URL and credential
// are not stored in the returned contract.
func DiscoverGatewayContract(ctx context.Context, cfg GatewayConfig) (GatewayContract, error) {
	if !cfg.Authorized {
		return GatewayContract{}, fmt.Errorf("%w: gateway discovery is not authorized", ErrGatewayUnauthorized)
	}
	host, path, err := splitEndpoint(cfg.BaseURL)
	if err != nil {
		return GatewayContract{}, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return GatewayContract{}, fmt.Errorf("%w: %v", ErrModelClient, err)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return GatewayContract{}, fmt.Errorf("%w: %v", ErrModelClient, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return GatewayContract{}, fmt.Errorf("%w: %v", ErrModelClient, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return GatewayContract{}, &HTTPStatusError{Status: resp.StatusCode}
	}
	var body gatewayModelsBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return GatewayContract{}, fmt.Errorf("%w: model list body: %v", ErrModelClient, err)
	}
	contract := GatewayContract{Endpoint: host + path}
	for _, model := range body.Data {
		info := model.ModelInfo
		contract.Models = append(contract.Models, GatewayModelContract{
			ID:                      model.ID,
			Mode:                    info.Mode,
			SupportedEndpoints:      append([]string(nil), info.SupportedEndpoints...),
			MaxInputTokens:          info.MaxInputTokens,
			MaxOutputTokens:         info.MaxOutputTokens,
			SupportsResponseSchema:  info.SupportsResponseSchema,
			SupportsReasoning:       info.SupportsReasoning,
			SupportsStreaming:       info.SupportsStreaming,
			SupportsFunctionCalling: info.SupportsFunctionCalling,
			Priced:                  info.InputCostPerToken != nil || info.OutputCostPerToken != nil,
		})
	}
	return contract, nil
}

// TransportIdentity is implemented by transports that address a declared wire
// model. The Recorder records the transport's model id when present (the model
// actually addressed), and otherwise the request's declared model. The hermetic
// fake and replay transports report none, so their recorded bytes are
// unchanged.
type TransportIdentity interface {
	WireModel() string
}

// TransportOutputMode is implemented by transports that request a declared JSON
// output mode. The Recorder records the mode when present, so an auditor can
// see whether a JSON-object or a plain completion was requested. The hermetic
// fake and replay transports report none, so their recorded bytes are
// unchanged.
type TransportOutputMode interface {
	OutputMode() OutputMode
}

// TransportAppliedSampling is implemented by transports that apply a reasoning
// effort and/or an output-token bound that supersedes the request's declared
// Sampling instance (the responses wire bounds reasoning and text with one
// field, ticket 34). The Recorder records the applied values per call, so the
// ledger names what the wire carried rather than only what was declared. The
// hermetic fake, the replay transport, and a wire that applies the declared
// sampling unchanged report none, so their recorded bytes are unchanged.
type TransportAppliedSampling interface {
	AppliedReasoningEffort() string
	AppliedMaxOutputTokens() int
}

// Pricer converts one attempt's usage to a cost and declares the price-table
// version recorded with every call. It is pluggable so a model with no price
// entry takes the typed unpriced state instead of a fabricated zero.
type Pricer interface {
	PriceVersion() string
	Cost(Usage) (Cost, error)
	Validate() error
}

// Cost is one attempt's cost: either a priced micros amount or the declared
// unpriced state (no entry in the price table). An unpriced cost is never a
// silent zero.
type Cost struct {
	Micros   int64
	Unpriced bool
}

// UnpricedPriceVersion is the declared price-version string recorded when no
// entry exists for the model.
const UnpricedPriceVersion = "unpriced"

// UnpricedPrice is the declared no-entry pricer. It returns the typed unpriced
// state and never invents a price. The application model carries no cost fields
// in its gateway metadata, so this is the default for the real arm.
type UnpricedPrice struct{}

func (UnpricedPrice) PriceVersion() string     { return UnpricedPriceVersion }
func (UnpricedPrice) Cost(Usage) (Cost, error) { return Cost{Unpriced: true}, nil }
func (UnpricedPrice) Validate() error          { return nil }

// PriceTable implements Pricer. Its Validate and per-token rates live in
// budget.go; PriceVersion and Cost are the pluggable surface.
func (p PriceTable) PriceVersion() string { return p.Version }

// Cost prices one attempt from the declared per-token rates.
func (p PriceTable) Cost(u Usage) (Cost, error) {
	micros, err := p.CostMicros(u)
	if err != nil {
		return Cost{}, err
	}
	return Cost{Micros: micros}, nil
}

// joinEndpoint appends a wire suffix to a base path without doubling the
// separator.
func joinEndpoint(base, suffix string) string {
	if base == "" {
		base = "/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + strings.TrimPrefix(suffix, "/")
}
