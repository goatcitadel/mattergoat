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
	"errors"

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
// them from the structured /v1/turns:complete response. Treating Message text as
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

// mgRuntime returns the runtime adapter for the current configuration. Today it
// is always the mattermost-plugin-ai bridge; once MGAgentProfiles carries a
// Runtime field (and provider/endpoint config exists) this becomes the per-agent
// selection point (e.g. goatCitadelRuntime for GoatCitadel-backed agents).
func (a *App) mgRuntime() MGAgentRuntime {
	return &bridgeAgentRuntime{app: a}
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

// goatCitadelRuntime is the documented extension point for routing turns to
// GoatCitadel (the external AI runtime brain) over HTTP/A2A/webhook. It is not
// wired into mgRuntime() yet — selection requires an MGAgentProfiles.Runtime
// field plus endpoint/credential config (see docs/mattergoat-ai-collaboration.md
// and the GoatCitadel integration follow-ups). Implementing this must NOT modify
// GoatCitadel from this repository.
type goatCitadelRuntime struct {
	app      *App
	endpoint string
}

func (r *goatCitadelRuntime) Name() string { return "goatcitadel" }

func (r *goatCitadelRuntime) Complete(_ request.CTX, _ MGRuntimeRequest) (MGRuntimeResult, error) {
	return MGRuntimeResult{}, errors.New("mattergoat: GoatCitadel runtime adapter not configured")
}

var _ MGAgentRuntime = (*bridgeAgentRuntime)(nil)
var _ MGAgentRuntime = (*goatCitadelRuntime)(nil)
