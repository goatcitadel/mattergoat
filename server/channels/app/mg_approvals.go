// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package app — MatterGoat inbound runtime approvals (Phase 3).
//
// GoatCitadel (the runtime brain) calls MatterGoat when a turn needs human
// approval before a side-effecting action. MatterGoat records an MGApproval,
// pauses the session, surfaces it to humans, and the existing resolve flow
// returns the decision (which GoatCitadel polls).

package app

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// MGCreateRuntimeApproval records an approval requested by the GoatCitadel runtime
// for a turn and pauses the session until a human resolves it.
func (a *App) MGCreateRuntimeApproval(rctx request.CTX, req model.MGRuntimeApprovalRequest) (*model.MGApproval, *model.AppError) {
	if !a.MatterGoatEnabled() {
		return nil, mgErr("MGCreateRuntimeApproval", "app.mattergoat.disabled", http.StatusForbidden, nil)
	}
	if req.SessionID == "" || req.Action == "" {
		return nil, mgErr("MGCreateRuntimeApproval", "app.mattergoat.invalid_approval", http.StatusBadRequest, nil)
	}

	session, err := a.Srv().Store().MatterGoat().GetSession(req.SessionID)
	if err != nil {
		return nil, mgErr("MGCreateRuntimeApproval", "app.mattergoat.get_session.error", http.StatusNotFound, err)
	}

	// Best-effort: resolve agent_ref (a GoatCitadel agent id) to its mirrored
	// agent-profile id so the approval is attributed to the right agent.
	requestedBy := ""
	if req.AgentRef != "" {
		if profiles, pErr := a.MGGetAgentProfilesByOwner(model.MGOwnerTypeExternal, goatCitadelDiscoveryOwnerID); pErr == nil {
			for _, p := range profiles {
				if p.BridgeAgentId == req.AgentRef {
					requestedBy = p.Id
					break
				}
			}
		}
	}

	affected := "[]"
	if len(req.AffectedResources) > 0 {
		if b, mErr := json.Marshal(req.AffectedResources); mErr == nil {
			affected = string(b)
		}
	}

	approval := &model.MGApproval{
		SessionId:          session.Id,
		TurnId:             req.TurnID,
		RequestedByAgentId: requestedBy,
		Action:             req.Action,
		RiskLevel:          req.RiskLevel,
		Reason:             req.Reason,
		AffectedResources:  affected,
	}
	approval.PreSave()
	if appErr := approval.IsValid(); appErr != nil {
		return nil, appErr
	}

	saved, sErr := a.Srv().Store().MatterGoat().SaveApproval(approval)
	if sErr != nil {
		return nil, mgErr("MGCreateRuntimeApproval", "app.mattergoat.save_approval.error", http.StatusInternalServerError, sErr)
	}

	// Pause the session and surface the request to humans.
	session.State = model.MGSessionStateAwaitingApprove
	a.Srv().Store().MatterGoat().UpdateSession(session)
	a.mgPublishApprovalRequested(session, saved)

	return saved, nil
}

// MGGetApproval returns a single approval by id, so GoatCitadel can poll its
// decision (status pending | approved | rejected | expired).
func (a *App) MGGetApproval(id string) (*model.MGApproval, *model.AppError) {
	approval, err := a.Srv().Store().MatterGoat().GetApproval(id)
	if err != nil {
		return nil, mgErr("MGGetApproval", "app.mattergoat.get_approval.error", http.StatusNotFound, err)
	}
	return approval, nil
}
