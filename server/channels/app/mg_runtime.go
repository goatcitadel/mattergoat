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
// context and never widen it.
type MGRuntimeRequest struct {
	SessionUserID string
	AgentRef      string
	Messages      []BridgeMessage
	UserID        string
	ChannelID     string
	Operation     string
}

// MGAgentRuntime runs one agent turn. Implementations must not perform
// side-effecting tool actions without an approval recorded by the orchestrator.
type MGAgentRuntime interface {
	// Name identifies the runtime for provenance/audit.
	Name() string
	// Complete returns the agent's message text for the given request.
	Complete(rctx request.CTX, req MGRuntimeRequest) (string, error)
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

func (r *bridgeAgentRuntime) Complete(rctx request.CTX, req MGRuntimeRequest) (string, error) {
	op := req.Operation
	if op == "" {
		op = mgClientOperation
	}
	return r.app.ch.agentsBridge.AgentCompletion(req.SessionUserID, req.AgentRef, BridgeCompletionRequest{
		Operation:       BridgeOperationCollaborate,
		ClientOperation: op,
		Messages:        req.Messages,
		UserID:          req.UserID,
		ChannelID:       req.ChannelID,
	})
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

func (r *goatCitadelRuntime) Complete(_ request.CTX, _ MGRuntimeRequest) (string, error) {
	return "", errors.New("mattergoat: GoatCitadel runtime adapter not configured")
}

var _ MGAgentRuntime = (*bridgeAgentRuntime)(nil)
var _ MGAgentRuntime = (*goatCitadelRuntime)(nil)
