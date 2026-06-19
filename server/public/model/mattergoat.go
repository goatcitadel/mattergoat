// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// MatterGoat multi-agent AI collaboration models.
//
// Canonical session state lives here (and in the MG* store tables) — NOT in
// chat text. JSON-ish policy/scope/evidence fields are stored as raw JSON
// strings so the store layer stays trivial; the app layer marshals/unmarshals.

// Agent owner types.
const (
	MGOwnerTypeUser     = "user"
	MGOwnerTypeTeam     = "team"
	MGOwnerTypeSystem   = "system"
	MGOwnerTypeExternal = "external"
)

// Agent trust levels. External/untrusted agents are advisory until promoted.
const (
	MGTrustTrusted   = "trusted"
	MGTrustAdvisory  = "advisory"
	MGTrustUntrusted = "untrusted"
)

// Session collaboration modes.
const (
	MGModeStrictTurns    = "strict_turns"
	MGModeLoose          = "loose"
	MGModeRoundCritique  = "round_critique"
	MGModeSynthesis      = "synthesis"
)

// Session states (the orchestrator state machine).
const (
	MGSessionStateCreated         = "created"
	MGSessionStateAwaitingTurn    = "awaiting_turn"
	MGSessionStateTurnInProgress  = "turn_in_progress"
	MGSessionStateWaiting         = "waiting"
	MGSessionStateStale           = "stale"
	MGSessionStateCollision       = "collision"
	MGSessionStateAwaitingApprove = "awaiting_approval"
	MGSessionStateSynthesizing    = "synthesizing"
	MGSessionStateCompleted       = "completed"
	MGSessionStateAborted         = "aborted"
)

// Turn statuses.
const (
	MGTurnStatusInProgress = "in_progress"
	MGTurnStatusComplete   = "complete"
	MGTurnStatusStale      = "stale"
	MGTurnStatusCollision  = "collision"
	MGTurnStatusViolation  = "violation"
)

// Approval statuses and risk levels.
const (
	MGApprovalStatusPending  = "pending"
	MGApprovalStatusApproved = "approved"
	MGApprovalStatusRejected = "rejected"
	MGApprovalStatusExpired  = "expired"

	MGRiskLow      = "low"
	MGRiskMedium   = "medium"
	MGRiskHigh     = "high"
	MGRiskCritical = "critical"
)

// Memory proposal statuses (promotion is propose-only by default).
const (
	MGMemoryStatusProposed = "proposed"
	MGMemoryStatusApproved = "approved"
	MGMemoryStatusRejected = "rejected"
)

// Protocol markers emitted/parsed in structured sessions. These are ADVISORY
// hints validated against stored state — never authoritative on their own.
const (
	MGMarkerProtocolAccepted = "PROTOCOL_ACCEPTED"
	MGMarkerTurnInProgress   = "TURN_IN_PROGRESS"
	MGMarkerHandoffComplete  = "HANDOFF_COMPLETE"
	MGMarkerWaitingForHandoff = "WAITING_FOR_HANDOFF"
	MGMarkerStaleTurn        = "STALE_TURN_DETECTED"
	MGMarkerCollision        = "COLLISION_DETECTED"
	MGMarkerProtocolViolation = "PROTOCOL_VIOLATION"
	MGMarkerUserOverride     = "USER_OVERRIDE"
	MGMarkerFinalSynthesis   = "FINAL_SYNTHESIS"
)

// Post type and props for governed agent messages.
const (
	PostTypeMGAgentResponse = "custom_mg_agent_response"

	PostPropsMGSessionID   = "mg_session_id"
	PostPropsMGAgentID     = "mg_agent_profile_id"
	PostPropsMGTurnID      = "mg_turn_id"
	PostPropsMGProvider    = "mg_provider"
	PostPropsMGModel       = "mg_model"
	PostPropsMGConfidence  = "mg_confidence"
	PostPropsMGMarker      = "mg_marker"
	PostPropsMGFinal       = "mg_final_synthesis"
)

// MGAgentProfile is a registered agent: a re-scoped wrapper around a bridge
// (mattermost-plugin-ai) agent, surfaced in Mattermost as a bot user.
type MGAgentProfile struct {
	Id                  string `json:"id"`
	OwnerType           string `json:"owner_type"`
	OwnerId             string `json:"owner_id"`
	DisplayName         string `json:"display_name"`
	Role                string `json:"role"`
	BridgeAgentId       string `json:"bridge_agent_id"`
	BotUserId           string `json:"bot_user_id"`
	TrustLevel          string `json:"trust_level"`
	DefaultContextScope string `json:"default_context_scope"`
	MemoryScope         string `json:"memory_scope"`
	ToolPolicy          string `json:"tool_policy"`       // raw JSON
	ApprovalPolicy      string `json:"approval_policy"`   // raw JSON
	AllowedChannels     string `json:"allowed_channels"`  // raw JSON array
	BlockedChannels     string `json:"blocked_channels"`  // raw JSON array
	CreateAt            int64  `json:"create_at"`
	UpdateAt            int64  `json:"update_at"`
	DeleteAt            int64  `json:"delete_at"`
}

func (p *MGAgentProfile) PreSave() {
	if p.Id == "" {
		p.Id = NewId()
	}
	if p.TrustLevel == "" {
		p.TrustLevel = MGTrustAdvisory
	}
	now := GetMillis()
	if p.CreateAt == 0 {
		p.CreateAt = now
	}
	p.UpdateAt = now
	p.ensureJSONDefaults()
}

func (p *MGAgentProfile) PreUpdate() {
	p.UpdateAt = GetMillis()
	p.ensureJSONDefaults()
}

func (p *MGAgentProfile) ensureJSONDefaults() {
	if p.ToolPolicy == "" {
		p.ToolPolicy = "{}"
	}
	if p.ApprovalPolicy == "" {
		p.ApprovalPolicy = "{}"
	}
	if p.AllowedChannels == "" {
		p.AllowedChannels = "[]"
	}
	if p.BlockedChannels == "" {
		p.BlockedChannels = "[]"
	}
}

func (p *MGAgentProfile) IsValid() *AppError {
	if !IsValidId(p.Id) {
		return mgInvalid("MGAgentProfile.IsValid", "id", p.Id)
	}
	switch p.OwnerType {
	case MGOwnerTypeUser, MGOwnerTypeTeam, MGOwnerTypeSystem, MGOwnerTypeExternal:
	default:
		return mgInvalid("MGAgentProfile.IsValid", "owner_type", p.Id)
	}
	if p.DisplayName == "" {
		return mgInvalid("MGAgentProfile.IsValid", "display_name", p.Id)
	}
	return nil
}

func (p *MGAgentProfile) Auditable() map[string]any {
	return map[string]any{
		"id":             p.Id,
		"owner_type":     p.OwnerType,
		"owner_id":       p.OwnerId,
		"display_name":   p.DisplayName,
		"role":           p.Role,
		"bridge_agent_id": p.BridgeAgentId,
		"bot_user_id":    p.BotUserId,
		"trust_level":    p.TrustLevel,
		"create_at":      p.CreateAt,
		"update_at":      p.UpdateAt,
		"delete_at":      p.DeleteAt,
	}
}

// MGSession is a collaboration session bound to a channel and (optionally) a
// root post / thread.
type MGSession struct {
	Id                 string `json:"id"`
	Title              string `json:"title"`
	ChannelId          string `json:"channel_id"`
	RootPostId         string `json:"root_post_id"`
	CreatedBy          string `json:"created_by"`
	Mode               string `json:"mode"`
	Profile            string `json:"profile"`
	State              string `json:"state"`
	CurrentTurnAgentId string `json:"current_turn_agent_id"`
	ContextScope       string `json:"context_scope"` // raw JSON
	MemoryBehavior     string `json:"memory_behavior"`
	CreateAt           int64  `json:"create_at"`
	UpdateAt           int64  `json:"update_at"`
	CompletedAt        int64  `json:"completed_at"`
	DeleteAt           int64  `json:"delete_at"`

	// Populated on read, not persisted on the session row.
	Participants []*MGSessionParticipant `json:"participants,omitempty" db:"-"`
}

func (s *MGSession) PreSave() {
	if s.Id == "" {
		s.Id = NewId()
	}
	if s.State == "" {
		s.State = MGSessionStateCreated
	}
	if s.Mode == "" {
		s.Mode = MGModeStrictTurns
	}
	if s.MemoryBehavior == "" {
		s.MemoryBehavior = MatterGoatMemoryProposeOnly
	}
	if s.ContextScope == "" {
		s.ContextScope = "{}"
	}
	now := GetMillis()
	if s.CreateAt == 0 {
		s.CreateAt = now
	}
	s.UpdateAt = now
}

func (s *MGSession) IsValid() *AppError {
	if !IsValidId(s.Id) {
		return mgInvalid("MGSession.IsValid", "id", s.Id)
	}
	if !IsValidId(s.ChannelId) {
		return mgInvalid("MGSession.IsValid", "channel_id", s.Id)
	}
	if !IsValidId(s.CreatedBy) {
		return mgInvalid("MGSession.IsValid", "created_by", s.Id)
	}
	return nil
}

func (s *MGSession) Auditable() map[string]any {
	return map[string]any{
		"id":          s.Id,
		"title":       s.Title,
		"channel_id":  s.ChannelId,
		"root_post_id": s.RootPostId,
		"created_by":  s.CreatedBy,
		"mode":        s.Mode,
		"profile":     s.Profile,
		"state":       s.State,
		"create_at":   s.CreateAt,
		"update_at":   s.UpdateAt,
		"completed_at": s.CompletedAt,
		"delete_at":   s.DeleteAt,
	}
}

// MGSessionParticipant binds an agent profile to a session with a per-session
// context grant — the least-privilege boundary enforced by the orchestrator.
type MGSessionParticipant struct {
	Id             string `json:"id"`
	SessionId      string `json:"session_id"`
	AgentProfileId string `json:"agent_profile_id"`
	Role           string `json:"role"`
	ContextGrant   string `json:"context_grant"` // raw JSON
	JoinedAt       int64  `json:"joined_at"`
}

func (p *MGSessionParticipant) PreSave() {
	if p.Id == "" {
		p.Id = NewId()
	}
	if p.ContextGrant == "" {
		// Default least privilege: current thread only.
		p.ContextGrant = `{"scope":"current_thread"}`
	}
	if p.JoinedAt == 0 {
		p.JoinedAt = GetMillis()
	}
}

func (p *MGSessionParticipant) IsValid() *AppError {
	if !IsValidId(p.Id) {
		return mgInvalid("MGSessionParticipant.IsValid", "id", p.Id)
	}
	if !IsValidId(p.SessionId) {
		return mgInvalid("MGSessionParticipant.IsValid", "session_id", p.Id)
	}
	if !IsValidId(p.AgentProfileId) {
		return mgInvalid("MGSessionParticipant.IsValid", "agent_profile_id", p.Id)
	}
	return nil
}

func (p *MGSessionParticipant) Auditable() map[string]any {
	return map[string]any{
		"id":               p.Id,
		"session_id":       p.SessionId,
		"agent_profile_id": p.AgentProfileId,
		"role":             p.Role,
		"joined_at":        p.JoinedAt,
	}
}

// MGTurn records a single agent turn and links to the governed post it produced.
type MGTurn struct {
	Id             string `json:"id"`
	SessionId      string `json:"session_id"`
	AgentProfileId string `json:"agent_profile_id"`
	TurnIndex      int    `json:"turn_index"`
	PostId         string `json:"post_id"`
	Marker         string `json:"marker"`
	Status         string `json:"status"`
	StartedAt      int64  `json:"started_at"`
	CompletedAt    int64  `json:"completed_at"`
}

func (t *MGTurn) PreSave() {
	if t.Id == "" {
		t.Id = NewId()
	}
	if t.Status == "" {
		t.Status = MGTurnStatusInProgress
	}
	if t.StartedAt == 0 {
		t.StartedAt = GetMillis()
	}
}

func (t *MGTurn) Auditable() map[string]any {
	return map[string]any{
		"id":               t.Id,
		"session_id":       t.SessionId,
		"agent_profile_id": t.AgentProfileId,
		"turn_index":       t.TurnIndex,
		"post_id":          t.PostId,
		"marker":           t.Marker,
		"status":           t.Status,
		"started_at":       t.StartedAt,
		"completed_at":     t.CompletedAt,
	}
}

// MGApproval records a pending/resolved approval for a risky agent action.
type MGApproval struct {
	Id                 string `json:"id"`
	SessionId          string `json:"session_id"`
	RequestedByAgentId string `json:"requested_by_agent_id"`
	Action             string `json:"action"`
	RiskLevel          string `json:"risk_level"`
	AffectedResources  string `json:"affected_resources"` // raw JSON
	Reason             string `json:"reason"`
	Status             string `json:"status"`
	ApproverUserId     string `json:"approver_user_id"`
	CreateAt           int64  `json:"create_at"`
	ResolvedAt         int64  `json:"resolved_at"`
}

func (a *MGApproval) PreSave() {
	if a.Id == "" {
		a.Id = NewId()
	}
	if a.Status == "" {
		a.Status = MGApprovalStatusPending
	}
	if a.RiskLevel == "" {
		a.RiskLevel = MGRiskMedium
	}
	if a.AffectedResources == "" {
		a.AffectedResources = "[]"
	}
	if a.CreateAt == 0 {
		a.CreateAt = GetMillis()
	}
}

func (a *MGApproval) IsValid() *AppError {
	if !IsValidId(a.Id) {
		return mgInvalid("MGApproval.IsValid", "id", a.Id)
	}
	if !IsValidId(a.SessionId) {
		return mgInvalid("MGApproval.IsValid", "session_id", a.Id)
	}
	if a.Action == "" {
		return mgInvalid("MGApproval.IsValid", "action", a.Id)
	}
	return nil
}

func (a *MGApproval) Auditable() map[string]any {
	return map[string]any{
		"id":                    a.Id,
		"session_id":            a.SessionId,
		"requested_by_agent_id": a.RequestedByAgentId,
		"action":                a.Action,
		"risk_level":            a.RiskLevel,
		"status":                a.Status,
		"approver_user_id":      a.ApproverUserId,
		"create_at":             a.CreateAt,
		"resolved_at":           a.ResolvedAt,
	}
}

// MGMemoryProposal is propose-only durable memory awaiting human promotion.
type MGMemoryProposal struct {
	Id             string `json:"id"`
	SessionId      string `json:"session_id"`
	AgentProfileId string `json:"agent_profile_id"`
	ProposedText   string `json:"proposed_text"`
	Scope          string `json:"scope"`
	Sensitivity    string `json:"sensitivity"`
	Evidence       string `json:"evidence"` // raw JSON
	Status         string `json:"status"`
	ApproverUserId string `json:"approver_user_id"`
	CreateAt       int64  `json:"create_at"`
	ResolvedAt     int64  `json:"resolved_at"`
	ExpiresAt      int64  `json:"expires_at"`
}

func (m *MGMemoryProposal) PreSave() {
	if m.Id == "" {
		m.Id = NewId()
	}
	if m.Status == "" {
		m.Status = MGMemoryStatusProposed
	}
	if m.Evidence == "" {
		m.Evidence = "[]"
	}
	if m.CreateAt == 0 {
		m.CreateAt = GetMillis()
	}
}

func (m *MGMemoryProposal) Auditable() map[string]any {
	return map[string]any{
		"id":               m.Id,
		"session_id":       m.SessionId,
		"agent_profile_id": m.AgentProfileId,
		"scope":            m.Scope,
		"sensitivity":      m.Sensitivity,
		"status":           m.Status,
		"approver_user_id": m.ApproverUserId,
		"create_at":        m.CreateAt,
		"resolved_at":      m.ResolvedAt,
		"expires_at":       m.ExpiresAt,
	}
}

// MGMarkdownExport records an AMCL-style export (an audited privacy downgrade).
type MGMarkdownExport struct {
	Id         string `json:"id"`
	SessionId  string `json:"session_id"`
	FileInfoId string `json:"file_info_id"`
	ExportedBy string `json:"exported_by"`
	CreateAt   int64  `json:"create_at"`
}

func (e *MGMarkdownExport) PreSave() {
	if e.Id == "" {
		e.Id = NewId()
	}
	if e.CreateAt == 0 {
		e.CreateAt = GetMillis()
	}
}

func (e *MGMarkdownExport) Auditable() map[string]any {
	return map[string]any{
		"id":           e.Id,
		"session_id":   e.SessionId,
		"file_info_id": e.FileInfoId,
		"exported_by":  e.ExportedBy,
		"create_at":    e.CreateAt,
	}
}

// --- request payloads ---

// MGStartSessionRequest is the payload to create a session.
type MGStartSessionRequest struct {
	Title          string   `json:"title"`
	ChannelId      string   `json:"channel_id"`
	RootPostId     string   `json:"root_post_id"`
	Mode           string   `json:"mode"`
	Profile        string   `json:"profile"`
	AgentProfileIds []string `json:"agent_profile_ids"`
	ContextScope   string   `json:"context_scope"`
}

// MGResolveRequest resolves an approval or memory proposal.
type MGResolveRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

func mgInvalid(where, field, id string) *AppError {
	return NewAppError(where, "model.mattergoat.is_valid.error", map[string]any{"Field": field, "Id": id}, "", 400)
}

// WebSocket events for live session UI updates.
const (
	WebsocketEventMGSessionUpdated    WebsocketEventType = "mg_session_updated"
	WebsocketEventMGTurnChanged       WebsocketEventType = "mg_turn_changed"
	WebsocketEventMGApprovalRequested WebsocketEventType = "mg_approval_requested"
	WebsocketEventMGSynthesisReady    WebsocketEventType = "mg_synthesis_ready"
)

// Audit events for security-relevant MatterGoat actions.
const (
	AuditEventMGStartSession     = "mg_start_session"
	AuditEventMGAbortSession     = "mg_abort_session"
	AuditEventMGAddParticipant   = "mg_add_participant"
	AuditEventMGSaveAgentProfile = "mg_save_agent_profile"
	AuditEventMGDeleteAgentProfile = "mg_delete_agent_profile"
	AuditEventMGRequestApproval  = "mg_request_approval"
	AuditEventMGResolveApproval  = "mg_resolve_approval"
	AuditEventMGResolveMemory    = "mg_resolve_memory_proposal"
	AuditEventMGExportMarkdown   = "mg_export_markdown"
	AuditEventMGSynthesize       = "mg_synthesize"
)
