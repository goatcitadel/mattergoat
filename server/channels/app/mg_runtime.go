// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package app — MatterGoat agent runtime adapter.
//
// MGAgentRuntime is the boundary between the MatterGoat orchestrator (which owns
// governed session/turn state) and whatever actually runs the model: today the
// in-core mattermost-plugin-ai bridge; later GoatCitadel (the external AI
// runtime brain) over HTTP/A2A/webhook. The orchestrator depends only on this
// interface, so GoatCitadel can become the runtime without MatterGoat taking a
// hard dependency on it. GoatCitadel stays canonical for runs/memory/policy;
// MatterGoat mirrors only read-only provenance back into posts and MGTurns.

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// MGRuntimeRequest is a runtime-agnostic completion request. AgentRef identifies
// the agent within the selected runtime (a bridge agent id today; a GoatCitadel
// agent reference later). Messages is the scoped context bundle the orchestrator
// already permission-filtered — runtimes must treat it as the full allowed
// context and never widen it. SessionID/TurnID are the MGSession/MGTurn ids (the
// turn id is the idempotency key for external runtimes); SessionUserID is the
// acting user's auth session, used by the in-core bridge.
type MGRuntimeRequest struct {
	SessionID     string
	TurnID        string
	SessionUserID string
	AgentRef      string
	Messages      []BridgeMessage
	UserID        string
	ChannelID     string
	Operation     string
}

// MGRuntimeResult is the structured outcome of one agent turn. The orchestrator
// makes control-flow decisions (final synthesis, approval gate) from these
// STRUCTURED fields — never by re-parsing Message. Each adapter populates
// Markers/NeedsApproval from a trusted source: the in-core bridge parses them
// from its own model output (transitional), while the GoatCitadel adapter fills
// them from the structured /api/v1/turns/complete response. Treating Message text as
// the marker channel would let untrusted prior-turn content spoof control signals.
type MGRuntimeResult struct {
	Message       string
	Markers       []string
	NeedsApproval bool
	Provider      string
	Model         string
	RunID         string
}

// MGAgentRuntime runs one agent turn. Implementations must not perform
// side-effecting tool actions without an approval recorded by the orchestrator.
type MGAgentRuntime interface {
	// Name identifies the runtime for provenance/audit.
	Name() string
	// Complete runs one agent turn and returns its structured result.
	Complete(rctx request.CTX, req MGRuntimeRequest) (MGRuntimeResult, error)
}

// mgRuntime returns the runtime adapter for the given agent profile. Profiles
// default to the in-core mattermost-plugin-ai bridge; a profile whose Runtime is
// GoatCitadel routes its turns to the external runtime over HTTP. A profile
// configured for GoatCitadel but with no endpoint set yields an adapter that
// fails the turn with a clear "not configured" error, rather than silently
// running the turn on a different runtime than the operator selected.
func (a *App) mgRuntime(profile *model.MGAgentProfile) MGAgentRuntime {
	if profile != nil && profile.Runtime == model.MGRuntimeGoatCitadel {
		if client, ok := a.goatCitadelClientFromConfig(); ok {
			return client
		}
		// Configured for GoatCitadel but no endpoint set: return an unconfigured
		// adapter that fails the turn with a clear error rather than silently
		// running on the bridge.
		return &goatCitadelRuntime{}
	}
	return &bridgeAgentRuntime{app: a}
}

// goatCitadelClientFromConfig builds a GoatCitadel HTTP client from
// MatterGoatSettings, or returns (nil, false) when no endpoint is configured.
// Shared by turn routing (mgRuntime) and agent discovery.
func (a *App) goatCitadelClientFromConfig() (*goatCitadelRuntime, bool) {
	cfg := a.Config().MatterGoatSettings
	endpoint := ""
	if cfg.GoatCitadelURL != nil {
		endpoint = *cfg.GoatCitadelURL
	}
	if endpoint == "" {
		return nil, false
	}
	token := ""
	if cfg.GoatCitadelToken != nil {
		token = *cfg.GoatCitadelToken
	}
	return &goatCitadelRuntime{
		endpoint: endpoint,
		token:    token,
		client:   &http.Client{Timeout: goatCitadelHTTPTimeout},
	}, true
}

// bridgeAgentRuntime adapts the existing in-core agents bridge
// (mattermost-plugin-ai) to MGAgentRuntime.
type bridgeAgentRuntime struct {
	app *App
}

func (r *bridgeAgentRuntime) Name() string { return "mattergoat_bridge" }

func (r *bridgeAgentRuntime) Complete(rctx request.CTX, req MGRuntimeRequest) (MGRuntimeResult, error) {
	op := req.Operation
	if op == "" {
		op = mgClientOperation
	}
	text, err := r.app.ch.agentsBridge.AgentCompletion(req.SessionUserID, req.AgentRef, BridgeCompletionRequest{
		Operation:       BridgeOperationCollaborate,
		ClientOperation: op,
		Messages:        req.Messages,
		UserID:          req.UserID,
		ChannelID:       req.ChannelID,
	})
	if err != nil {
		return MGRuntimeResult{}, err
	}
	// The in-core bridge returns only text — the model's own (trusted) output for
	// this turn. Parse protocol markers and the approval gate from it here so the
	// orchestrator can treat MGRuntimeResult as authoritative. An external runtime
	// (GoatCitadel) supplies these structurally instead of via text.
	return MGRuntimeResult{
		Message:       text,
		Markers:       mgParseMarkers(text),
		NeedsApproval: mgNeedsApproval(text),
	}, nil
}

const (
	// goatCitadelHTTPTimeout bounds a single turn call. Turns can be slow (tool
	// use, multiple model calls), so this is generous; streaming/webhooks (Phase 4)
	// are the answer for genuinely long-running turns.
	goatCitadelHTTPTimeout = 120 * time.Second
	// goatCitadelMaxResponseBytes caps how much of a response body is read.
	goatCitadelMaxResponseBytes = 8 << 20 // 8 MiB
)

// goatCitadelRuntime routes turns to GoatCitadel (the external AI runtime brain)
// via POST {endpoint}/api/v1/turns/complete. Selection happens in mgRuntime() when a
// profile's Runtime is GoatCitadel. This adapter only speaks the documented HTTP
// contract (docs/goatcitadel-integration-requests.md); it must NOT modify
// GoatCitadel from this repository.
type goatCitadelRuntime struct {
	endpoint string
	token    string
	client   *http.Client
}

func (r *goatCitadelRuntime) Name() string { return "goatcitadel" }

func (r *goatCitadelRuntime) Complete(rctx request.CTX, req MGRuntimeRequest) (MGRuntimeResult, error) {
	if r.endpoint == "" {
		return MGRuntimeResult{}, errors.New("mattergoat: GoatCitadel runtime adapter not configured")
	}

	payload, err := json.Marshal(newGoatCitadelTurnRequest(req))
	if err != nil {
		return MGRuntimeResult{}, errors.Wrap(err, "mattergoat: marshal GoatCitadel turn request")
	}

	url := strings.TrimRight(r.endpoint, "/") + "/api/v1/turns/complete"
	httpReq, err := http.NewRequestWithContext(rctx.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return MGRuntimeResult{}, errors.Wrap(err, "mattergoat: build GoatCitadel turn request")
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.token)
	httpReq.Header.Set("Content-Type", "application/json")
	// Idempotency-Key lets GoatCitadel dedupe a retried turn.
	httpReq.Header.Set("Idempotency-Key", req.TurnID)

	client := r.client
	if client == nil {
		client = &http.Client{Timeout: goatCitadelHTTPTimeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return MGRuntimeResult{}, errors.Wrap(err, "mattergoat: call GoatCitadel turns:complete")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, goatCitadelMaxResponseBytes))
	if err != nil {
		return MGRuntimeResult{}, errors.Wrap(err, "mattergoat: read GoatCitadel response")
	}
	if resp.StatusCode != http.StatusOK {
		return MGRuntimeResult{}, goatCitadelHTTPError(resp.StatusCode, body)
	}

	var parsed goatCitadelTurnResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return MGRuntimeResult{}, errors.Wrap(err, "mattergoat: decode GoatCitadel response")
	}
	return parsed.toResult(), nil
}

// --- GoatCitadel HTTP contract (docs/goatcitadel-integration-requests.md) ---

type goatCitadelTurnRequest struct {
	SessionID  string               `json:"session_id"`
	TurnID     string               `json:"turn_id"`
	AgentRef   string               `json:"agent_ref"`
	Operation  string               `json:"operation"`
	UserRef    string               `json:"user_ref"`
	ChannelRef string               `json:"channel_ref"`
	Messages   []goatCitadelMessage `json:"messages"`
}

type goatCitadelMessage struct {
	Role      string   `json:"role"`
	AuthorRef string   `json:"author_ref,omitempty"`
	Message   string   `json:"message"`
	FileIDs   []string `json:"file_ids"`
}

func newGoatCitadelTurnRequest(req MGRuntimeRequest) goatCitadelTurnRequest {
	op := req.Operation
	if op == "" {
		op = mgClientOperation
	}
	messages := make([]goatCitadelMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		fileIDs := m.FileIDs
		if fileIDs == nil {
			fileIDs = []string{}
		}
		messages = append(messages, goatCitadelMessage{
			Role:      m.Role,
			AuthorRef: m.AuthorRef,
			Message:   m.Message,
			FileIDs:   fileIDs,
		})
	}
	return goatCitadelTurnRequest{
		SessionID:  req.SessionID,
		TurnID:     req.TurnID,
		AgentRef:   req.AgentRef,
		Operation:  op,
		UserRef:    req.UserID,
		ChannelRef: req.ChannelID,
		Messages:   messages,
	}
}

type goatCitadelTurnResponse struct {
	Message       string   `json:"message"`
	Markers       []string `json:"markers"`
	NeedsApproval bool     `json:"needs_approval"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	RunID         string   `json:"run_id"`
}

func (resp goatCitadelTurnResponse) toResult() MGRuntimeResult {
	return MGRuntimeResult{
		Message:       resp.Message,
		Markers:       resp.Markers,
		NeedsApproval: resp.NeedsApproval,
		Provider:      resp.Provider,
		Model:         resp.Model,
		RunID:         resp.RunID,
	}
}

// goatCitadelHTTPError maps a non-200 response to an error, surfacing the JSON
// {"error":{"code","message"}} envelope when present.
func goatCitadelHTTPError(status int, body []byte) error {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return errors.Errorf("mattergoat: GoatCitadel turns:complete returned %d: %s", status, env.Error.Message)
	}
	return errors.Errorf("mattergoat: GoatCitadel turns:complete returned %d", status)
}

var _ MGAgentRuntime = (*bridgeAgentRuntime)(nil)
var _ MGAgentRuntime = (*goatCitadelRuntime)(nil)
