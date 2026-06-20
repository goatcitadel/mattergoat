// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

func (api *API) InitMatterGoat() {
	r := api.BaseRoutes.MatterGoat

	// Agent profiles (registry)
	r.Handle("/agent_profiles", api.APISessionRequired(getAgentProfiles)).Methods(http.MethodGet)
	r.Handle("/agent_profiles", api.APISessionRequired(createAgentProfile)).Methods(http.MethodPost)
	// Registered before the {profile_id} routes so the literal "sync" path wins.
	r.Handle("/agent_profiles/sync", api.APISessionRequired(syncGoatCitadelAgentProfiles)).Methods(http.MethodPost)
	r.Handle("/agent_profiles/{profile_id:[A-Za-z0-9]+}", api.APISessionRequired(getAgentProfile)).Methods(http.MethodGet)
	r.Handle("/agent_profiles/{profile_id:[A-Za-z0-9]+}", api.APISessionRequired(updateAgentProfile)).Methods(http.MethodPut)
	r.Handle("/agent_profiles/{profile_id:[A-Za-z0-9]+}", api.APISessionRequired(deleteAgentProfile)).Methods(http.MethodDelete)

	// Sessions
	r.Handle("/sessions", api.APISessionRequired(startSession)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}", api.APISessionRequired(getSession)).Methods(http.MethodGet)
	r.Handle("/channels/{channel_id:[A-Za-z0-9]+}/sessions", api.APISessionRequired(getChannelSessions)).Methods(http.MethodGet)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/abort", api.APISessionRequired(abortSession)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/advance", api.APISessionRequired(advanceSession)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/run", api.APISessionRequired(runSession)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/synthesize", api.APISessionRequired(synthesizeSession)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/participants", api.APISessionRequired(addSessionParticipant)).Methods(http.MethodPost)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/turns", api.APISessionRequired(getSessionTurns)).Methods(http.MethodGet)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/export", api.APISessionRequired(exportSession)).Methods(http.MethodPost)

	// Approvals
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/approvals", api.APISessionRequired(getSessionApprovals)).Methods(http.MethodGet)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/approvals", api.APISessionRequired(requestSessionApproval)).Methods(http.MethodPost)
	r.Handle("/approvals/{approval_id:[A-Za-z0-9]+}/resolve", api.APISessionRequired(resolveApproval)).Methods(http.MethodPost)

	// Memory proposals
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/memory_proposals", api.APISessionRequired(getSessionMemoryProposals)).Methods(http.MethodGet)
	r.Handle("/sessions/{session_id:[A-Za-z0-9]+}/memory_proposals/{proposal_id:[A-Za-z0-9]+}/resolve", api.APISessionRequired(resolveMemoryProposal)).Methods(http.MethodPost)
}

func requireMatterGoatEnabled(c *Context) {
	if !c.App.MatterGoatEnabled() {
		c.Err = model.NewAppError("requireMatterGoatEnabled", "api.mattergoat.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
}

func mgVar(r *http.Request, key string) string {
	return mux.Vars(r)[key]
}

// mgRequireSessionRead loads the session and verifies the caller can read its
// channel. Returns the session or sets c.Err.
func mgRequireSessionRead(c *Context, sessionID string) *model.MGSession {
	session, err := c.App.MGGetSession(sessionID)
	if err != nil {
		c.Err = err
		return nil
	}
	if ok, _ := c.App.HasPermissionToChannel(c.AppContext, c.AppContext.Session().UserId, session.ChannelId, model.PermissionReadChannel); !ok {
		c.Err = model.NewAppError("mgRequireSessionRead", "api.mattergoat.permission_denied", nil, "", http.StatusForbidden)
		return nil
	}
	return session
}

// mgRequireSessionControl additionally requires the caller be the session
// creator or have post permission in the channel.
func mgRequireSessionControl(c *Context, sessionID string) *model.MGSession {
	session := mgRequireSessionRead(c, sessionID)
	if session == nil {
		return nil
	}
	if session.CreatedBy == c.AppContext.Session().UserId {
		return session
	}
	if ok, _ := c.App.HasPermissionToChannel(c.AppContext, c.AppContext.Session().UserId, session.ChannelId, model.PermissionCreatePost); !ok {
		c.Err = model.NewAppError("mgRequireSessionControl", "api.mattergoat.permission_denied", nil, "", http.StatusForbidden)
		return nil
	}
	return session
}

func mgWriteJSON(c *Context, w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		c.Logger.Warn("Error encoding MatterGoat response", mlog.Err(err))
	}
}

// --- Agent profiles ---

func getAgentProfiles(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	ownerType := r.URL.Query().Get("owner_type")
	ownerID := r.URL.Query().Get("owner_id")
	profiles, err := c.App.MGGetAgentProfilesByOwner(ownerType, ownerID)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, profiles)
}

func createAgentProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionMGManageAgentProfiles) {
		c.SetPermissionError(model.PermissionMGManageAgentProfiles)
		return
	}

	var profile model.MGAgentProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMGSaveAgentProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventObjectType("mg_agent_profile")

	saved, err := c.App.MGSaveAgentProfile(c.AppContext, &profile, false)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddEventResultState(saved)
	w.WriteHeader(http.StatusCreated)
	mgWriteJSON(c, w, saved)
}

// syncGoatCitadelAgentProfiles discovers agents from the configured GoatCitadel
// runtime and upserts them as runtime=goatcitadel agent profiles.
func syncGoatCitadelAgentProfiles(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionMGManageAgentProfiles) {
		c.SetPermissionError(model.PermissionMGManageAgentProfiles)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMGSaveAgentProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventObjectType("mg_agent_profile")

	profiles, err := c.App.MGSyncGoatCitadelAgents(c.AppContext)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	mgWriteJSON(c, w, profiles)
}

func getAgentProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	profile, err := c.App.MGGetAgentProfile(mgVar(r, "profile_id"))
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, profile)
}

func updateAgentProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionMGManageAgentProfiles) {
		c.SetPermissionError(model.PermissionMGManageAgentProfiles)
		return
	}
	var profile model.MGAgentProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}
	profile.Id = mgVar(r, "profile_id")

	auditRec := c.MakeAuditRecord(model.AuditEventMGSaveAgentProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventObjectType("mg_agent_profile")

	saved, err := c.App.MGSaveAgentProfile(c.AppContext, &profile, true)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddEventResultState(saved)
	mgWriteJSON(c, w, saved)
}

func deleteAgentProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionMGManageAgentProfiles) {
		c.SetPermissionError(model.PermissionMGManageAgentProfiles)
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGDeleteAgentProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "profile_id", mgVar(r, "profile_id"))

	if err := c.App.MGDeleteAgentProfile(mgVar(r, "profile_id")); err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

// --- Sessions ---

func startSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionMGCreateSession) {
		c.SetPermissionError(model.PermissionMGCreateSession)
		return
	}

	var req model.MGStartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}
	if req.ChannelId == "" {
		c.SetInvalidParam("channel_id")
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMGStartSession, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	auditRec.AddEventObjectType("mg_session")
	model.AddEventParameterToAuditRec(auditRec, "channel_id", req.ChannelId)
	model.AddEventParameterToAuditRec(auditRec, "agent_profile_ids", req.AgentProfileIds)

	session, err := c.App.MGStartSession(c.AppContext, &req)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddEventResultState(session)
	w.WriteHeader(http.StatusCreated)
	mgWriteJSON(c, w, session)
}

func getSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionRead(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	mgWriteJSON(c, w, session)
}

func getChannelSessions(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	channelID := mgVar(r, "channel_id")
	if ok, _ := c.App.HasPermissionToChannel(c.AppContext, c.AppContext.Session().UserId, channelID, model.PermissionReadChannel); !ok {
		c.Err = model.NewAppError("getChannelSessions", "api.mattergoat.permission_denied", nil, "", http.StatusForbidden)
		return
	}
	sessions, err := c.App.MGGetSessionsForChannel(channelID)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, sessions)
}

func abortSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGAbortSession, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "session_id", session.Id)

	if err := c.App.MGAbortSession(c.AppContext, session.Id); err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

func advanceSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	if _, err := c.App.MGAdvanceTurn(c.AppContext, session.Id); err != nil {
		c.Err = err
		return
	}
	updated, err := c.App.MGGetSession(session.Id)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, updated)
}

func runSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	if err := c.App.MGRunSession(c.AppContext, session.Id); err != nil {
		c.Err = err
		return
	}
	updated, err := c.App.MGGetSession(session.Id)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, updated)
}

func synthesizeSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGSynthesize, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "session_id", session.Id)

	if err := c.App.MGSynthesize(c.AppContext, session.Id); err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	updated, _ := c.App.MGGetSession(session.Id)
	mgWriteJSON(c, w, updated)
}

func addSessionParticipant(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	var body struct {
		AgentProfileId string `json:"agent_profile_id"`
		ContextGrant   string `json:"context_grant"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}
	if body.AgentProfileId == "" {
		c.SetInvalidParam("agent_profile_id")
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMGAddParticipant, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "session_id", session.Id)
	model.AddEventParameterToAuditRec(auditRec, "agent_profile_id", body.AgentProfileId)

	if err := c.App.MGAddParticipant(c.AppContext, session.Id, body.AgentProfileId, body.ContextGrant); err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	updated, _ := c.App.MGGetSession(session.Id)
	mgWriteJSON(c, w, updated)
}

func getSessionTurns(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionRead(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	turns, err := c.App.Srv().Store().MatterGoat().GetTurnsForSession(session.Id)
	if err != nil {
		c.Err = model.NewAppError("getSessionTurns", "api.mattergoat.get_turns.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}
	mgWriteJSON(c, w, turns)
}

func exportSession(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGExportMarkdown, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	model.AddEventParameterToAuditRec(auditRec, "session_id", session.Id)

	md, err := c.App.MGExportMarkdown(c.AppContext, session.Id)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	mgWriteJSON(c, w, map[string]string{"markdown": md})
}

// --- Approvals ---

func getSessionApprovals(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionRead(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	approvals, err := c.App.MGGetApprovalsForSession(session.Id)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, approvals)
}

func requestSessionApproval(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	var body struct {
		Action    string `json:"action"`
		RiskLevel string `json:"risk_level"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}
	if body.Action == "" {
		c.SetInvalidParam("action")
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGRequestApproval, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "session_id", session.Id)

	approval, err := c.App.MGRequestApproval(c.AppContext, session.Id, body.Action, body.RiskLevel, body.Reason)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddEventResultState(approval)
	w.WriteHeader(http.StatusCreated)
	mgWriteJSON(c, w, approval)
}

func resolveApproval(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	var body model.MGResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}

	approvalID := mgVar(r, "approval_id")
	approval, err := c.App.Srv().Store().MatterGoat().GetApproval(approvalID)
	if err != nil {
		c.Err = model.NewAppError("resolveApproval", "api.mattergoat.get_approval.app_error", nil, "", http.StatusNotFound).Wrap(err)
		return
	}
	// Authorization is bound to the approval's session.
	if mgRequireSessionControl(c, approval.SessionId) == nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMGResolveApproval, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "approval_id", approvalID)
	model.AddEventParameterToAuditRec(auditRec, "approve", body.Approve)

	resolved, rErr := c.App.MGResolveApproval(c.AppContext, approvalID, body.Approve)
	if rErr != nil {
		c.Err = rErr
		return
	}
	auditRec.Success()
	auditRec.AddEventResultState(resolved)
	mgWriteJSON(c, w, resolved)
}

// --- Memory proposals ---

func getSessionMemoryProposals(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionRead(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	proposals, err := c.App.MGGetMemoryProposals(session.Id)
	if err != nil {
		c.Err = err
		return
	}
	mgWriteJSON(c, w, proposals)
}

func resolveMemoryProposal(c *Context, w http.ResponseWriter, r *http.Request) {
	requireMatterGoatEnabled(c)
	if c.Err != nil {
		return
	}
	session := mgRequireSessionControl(c, mgVar(r, "session_id"))
	if session == nil {
		return
	}
	var body model.MGResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventMGResolveMemory, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "proposal_id", mgVar(r, "proposal_id"))

	if err := c.App.MGResolveMemoryProposal(c.AppContext, session.Id, mgVar(r, "proposal_id"), body.Approve); err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}
