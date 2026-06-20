// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"
)

var (
	mgAgentProfileColumns = []string{
		"Id", "OwnerType", "OwnerId", "DisplayName", "Role", "BridgeAgentId",
		"BotUserId", "TrustLevel", "Runtime", "DefaultContextScope", "MemoryScope",
		"ToolPolicy", "ApprovalPolicy", "AllowedChannels", "BlockedChannels",
		"CreateAt", "UpdateAt", "DeleteAt",
	}

	mgSessionColumns = []string{
		"Id", "Title", "ChannelId", "RootPostId", "CreatedBy", "Mode", "Profile",
		"State", "CurrentTurnAgentId", "ContextScope", "MemoryBehavior",
		"CreateAt", "UpdateAt", "CompletedAt", "DeleteAt",
	}

	mgParticipantColumns = []string{
		"Id", "SessionId", "AgentProfileId", "Role", "ContextGrant", "JoinedAt",
	}

	mgTurnColumns = []string{
		"Id", "SessionId", "AgentProfileId", "TurnIndex", "PostId", "Marker",
		"Status", "Provider", "Model", "RunId", "StartedAt", "CompletedAt",
	}

	mgApprovalColumns = []string{
		"Id", "SessionId", "TurnId", "RequestedByAgentId", "Action", "RiskLevel",
		"AffectedResources", "Reason", "Status", "ApproverUserId", "CreateAt", "ResolvedAt", "ExpiresAt",
	}

	mgMemoryProposalColumns = []string{
		"Id", "SessionId", "AgentProfileId", "ProposedText", "Scope", "Sensitivity",
		"Evidence", "Status", "ApproverUserId", "CreateAt", "ResolvedAt", "ExpiresAt",
	}

	mgMarkdownExportColumns = []string{
		"Id", "SessionId", "FileInfoId", "ExportedBy", "CreateAt",
	}
)

type SqlMatterGoatStore struct {
	*SqlStore
}

func newSqlMatterGoatStore(sqlStore *SqlStore) store.MatterGoatStore {
	return &SqlMatterGoatStore{SqlStore: sqlStore}
}

// --- Agent profiles ---

func (s *SqlMatterGoatStore) agentProfileMap(p *model.MGAgentProfile) map[string]any {
	return map[string]any{
		"Id":                  p.Id,
		"OwnerType":           p.OwnerType,
		"OwnerId":             p.OwnerId,
		"DisplayName":         p.DisplayName,
		"Role":                p.Role,
		"BridgeAgentId":       p.BridgeAgentId,
		"BotUserId":           p.BotUserId,
		"TrustLevel":          p.TrustLevel,
		"Runtime":             p.Runtime,
		"DefaultContextScope": p.DefaultContextScope,
		"MemoryScope":         p.MemoryScope,
		"ToolPolicy":          p.ToolPolicy,
		"ApprovalPolicy":      p.ApprovalPolicy,
		"AllowedChannels":     p.AllowedChannels,
		"BlockedChannels":     p.BlockedChannels,
		"CreateAt":            p.CreateAt,
		"UpdateAt":            p.UpdateAt,
		"DeleteAt":            p.DeleteAt,
	}
}

func (s *SqlMatterGoatStore) SaveAgentProfile(p *model.MGAgentProfile) (*model.MGAgentProfile, error) {
	query := s.getQueryBuilder().Insert("MGAgentProfiles").SetMap(s.agentProfileMap(p))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGAgentProfile")
	}
	return p, nil
}

func (s *SqlMatterGoatStore) UpdateAgentProfile(p *model.MGAgentProfile) (*model.MGAgentProfile, error) {
	query := s.getQueryBuilder().Update("MGAgentProfiles").SetMap(s.agentProfileMap(p)).Where(sq.Eq{"Id": p.Id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrapf(err, "failed to update MGAgentProfile id=%s", p.Id)
	}
	return p, nil
}

func (s *SqlMatterGoatStore) GetAgentProfile(id string) (*model.MGAgentProfile, error) {
	var p model.MGAgentProfile
	query := s.getQueryBuilder().Select(mgAgentProfileColumns...).From("MGAgentProfiles").Where(sq.Eq{"Id": id})
	if err := s.GetReplica().GetBuilder(&p, query); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("MGAgentProfile", id)
		}
		return nil, errors.Wrapf(err, "failed to get MGAgentProfile id=%s", id)
	}
	return &p, nil
}

func (s *SqlMatterGoatStore) GetAgentProfilesByOwner(ownerType, ownerId string) ([]*model.MGAgentProfile, error) {
	var profiles []*model.MGAgentProfile
	q := s.getQueryBuilder().Select(mgAgentProfileColumns...).From("MGAgentProfiles").Where(sq.Eq{"DeleteAt": 0})
	if ownerType != "" {
		q = q.Where(sq.Eq{"OwnerType": ownerType})
	}
	if ownerId != "" {
		q = q.Where(sq.Eq{"OwnerId": ownerId})
	}
	q = q.OrderBy("DisplayName ASC")
	if err := s.GetReplica().SelectBuilder(&profiles, q); err != nil {
		return nil, errors.Wrap(err, "failed to get MGAgentProfiles by owner")
	}
	return profiles, nil
}

func (s *SqlMatterGoatStore) DeleteAgentProfile(id string) error {
	query := s.getQueryBuilder().Update("MGAgentProfiles").
		SetMap(map[string]any{"DeleteAt": model.GetMillis()}).Where(sq.Eq{"Id": id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return errors.Wrapf(err, "failed to delete MGAgentProfile id=%s", id)
	}
	return nil
}

// --- Sessions ---

func (s *SqlMatterGoatStore) sessionMap(m *model.MGSession) map[string]any {
	return map[string]any{
		"Id":                 m.Id,
		"Title":              m.Title,
		"ChannelId":          m.ChannelId,
		"RootPostId":         m.RootPostId,
		"CreatedBy":          m.CreatedBy,
		"Mode":               m.Mode,
		"Profile":            m.Profile,
		"State":              m.State,
		"CurrentTurnAgentId": m.CurrentTurnAgentId,
		"ContextScope":       m.ContextScope,
		"MemoryBehavior":     m.MemoryBehavior,
		"CreateAt":           m.CreateAt,
		"UpdateAt":           m.UpdateAt,
		"CompletedAt":        m.CompletedAt,
		"DeleteAt":           m.DeleteAt,
	}
}

func (s *SqlMatterGoatStore) SaveSession(m *model.MGSession) (*model.MGSession, error) {
	query := s.getQueryBuilder().Insert("MGSessions").SetMap(s.sessionMap(m))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGSession")
	}
	return m, nil
}

func (s *SqlMatterGoatStore) UpdateSession(m *model.MGSession) (*model.MGSession, error) {
	query := s.getQueryBuilder().Update("MGSessions").SetMap(s.sessionMap(m)).Where(sq.Eq{"Id": m.Id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrapf(err, "failed to update MGSession id=%s", m.Id)
	}
	return m, nil
}

func (s *SqlMatterGoatStore) GetSession(id string) (*model.MGSession, error) {
	var m model.MGSession
	query := s.getQueryBuilder().Select(mgSessionColumns...).From("MGSessions").Where(sq.Eq{"Id": id})
	if err := s.GetReplica().GetBuilder(&m, query); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("MGSession", id)
		}
		return nil, errors.Wrapf(err, "failed to get MGSession id=%s", id)
	}
	return &m, nil
}

func (s *SqlMatterGoatStore) GetSessionsForChannel(channelId string) ([]*model.MGSession, error) {
	var sessions []*model.MGSession
	query := s.getQueryBuilder().Select(mgSessionColumns...).From("MGSessions").
		Where(sq.Eq{"ChannelId": channelId, "DeleteAt": 0}).OrderBy("CreateAt DESC")
	if err := s.GetReplica().SelectBuilder(&sessions, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGSessions for channelId=%s", channelId)
	}
	return sessions, nil
}

func (s *SqlMatterGoatStore) DeleteSession(id string) error {
	query := s.getQueryBuilder().Update("MGSessions").
		SetMap(map[string]any{"DeleteAt": model.GetMillis()}).Where(sq.Eq{"Id": id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return errors.Wrapf(err, "failed to delete MGSession id=%s", id)
	}
	return nil
}

// --- Participants ---

func (s *SqlMatterGoatStore) SaveParticipant(p *model.MGSessionParticipant) (*model.MGSessionParticipant, error) {
	query := s.getQueryBuilder().Insert("MGSessionParticipants").SetMap(map[string]any{
		"Id":             p.Id,
		"SessionId":      p.SessionId,
		"AgentProfileId": p.AgentProfileId,
		"Role":           p.Role,
		"ContextGrant":   p.ContextGrant,
		"JoinedAt":       p.JoinedAt,
	})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGSessionParticipant")
	}
	return p, nil
}

func (s *SqlMatterGoatStore) GetParticipantsForSession(sessionId string) ([]*model.MGSessionParticipant, error) {
	var participants []*model.MGSessionParticipant
	query := s.getQueryBuilder().Select(mgParticipantColumns...).From("MGSessionParticipants").
		Where(sq.Eq{"SessionId": sessionId}).OrderBy("JoinedAt ASC")
	if err := s.GetReplica().SelectBuilder(&participants, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGSessionParticipants for sessionId=%s", sessionId)
	}
	return participants, nil
}

func (s *SqlMatterGoatStore) DeleteParticipant(sessionId, agentProfileId string) error {
	query := s.getQueryBuilder().Delete("MGSessionParticipants").
		Where(sq.Eq{"SessionId": sessionId, "AgentProfileId": agentProfileId})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return errors.Wrapf(err, "failed to delete MGSessionParticipant sessionId=%s agentProfileId=%s", sessionId, agentProfileId)
	}
	return nil
}

// --- Turns ---

func (s *SqlMatterGoatStore) SaveTurn(t *model.MGTurn) (*model.MGTurn, error) {
	query := s.getQueryBuilder().Insert("MGTurns").SetMap(s.turnMap(t))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGTurn")
	}
	return t, nil
}

func (s *SqlMatterGoatStore) turnMap(t *model.MGTurn) map[string]any {
	return map[string]any{
		"Id":             t.Id,
		"SessionId":      t.SessionId,
		"AgentProfileId": t.AgentProfileId,
		"TurnIndex":      t.TurnIndex,
		"PostId":         t.PostId,
		"Marker":         t.Marker,
		"Status":         t.Status,
		"Provider":       t.Provider,
		"Model":          t.Model,
		"RunId":          t.RunId,
		"StartedAt":      t.StartedAt,
		"CompletedAt":    t.CompletedAt,
	}
}

func (s *SqlMatterGoatStore) UpdateTurn(t *model.MGTurn) (*model.MGTurn, error) {
	query := s.getQueryBuilder().Update("MGTurns").SetMap(s.turnMap(t)).Where(sq.Eq{"Id": t.Id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrapf(err, "failed to update MGTurn id=%s", t.Id)
	}
	return t, nil
}

func (s *SqlMatterGoatStore) GetTurnsForSession(sessionId string) ([]*model.MGTurn, error) {
	var turns []*model.MGTurn
	query := s.getQueryBuilder().Select(mgTurnColumns...).From("MGTurns").
		Where(sq.Eq{"SessionId": sessionId}).OrderBy("TurnIndex ASC")
	if err := s.GetReplica().SelectBuilder(&turns, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGTurns for sessionId=%s", sessionId)
	}
	return turns, nil
}

// --- Approvals ---

func (s *SqlMatterGoatStore) approvalMap(a *model.MGApproval) map[string]any {
	return map[string]any{
		"Id":                 a.Id,
		"SessionId":          a.SessionId,
		"TurnId":             a.TurnId,
		"RequestedByAgentId": a.RequestedByAgentId,
		"Action":             a.Action,
		"RiskLevel":          a.RiskLevel,
		"AffectedResources":  a.AffectedResources,
		"Reason":             a.Reason,
		"Status":             a.Status,
		"ApproverUserId":     a.ApproverUserId,
		"CreateAt":           a.CreateAt,
		"ResolvedAt":         a.ResolvedAt,
		"ExpiresAt":          a.ExpiresAt,
	}
}

func (s *SqlMatterGoatStore) SaveApproval(a *model.MGApproval) (*model.MGApproval, error) {
	query := s.getQueryBuilder().Insert("MGApprovals").SetMap(s.approvalMap(a))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGApproval")
	}
	return a, nil
}

func (s *SqlMatterGoatStore) UpdateApproval(a *model.MGApproval) (*model.MGApproval, error) {
	query := s.getQueryBuilder().Update("MGApprovals").SetMap(s.approvalMap(a)).Where(sq.Eq{"Id": a.Id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrapf(err, "failed to update MGApproval id=%s", a.Id)
	}
	return a, nil
}

func (s *SqlMatterGoatStore) GetApproval(id string) (*model.MGApproval, error) {
	var a model.MGApproval
	query := s.getQueryBuilder().Select(mgApprovalColumns...).From("MGApprovals").Where(sq.Eq{"Id": id})
	if err := s.GetReplica().GetBuilder(&a, query); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("MGApproval", id)
		}
		return nil, errors.Wrapf(err, "failed to get MGApproval id=%s", id)
	}
	return &a, nil
}

func (s *SqlMatterGoatStore) GetApprovalsForSession(sessionId string) ([]*model.MGApproval, error) {
	var approvals []*model.MGApproval
	query := s.getQueryBuilder().Select(mgApprovalColumns...).From("MGApprovals").
		Where(sq.Eq{"SessionId": sessionId}).OrderBy("CreateAt ASC")
	if err := s.GetReplica().SelectBuilder(&approvals, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGApprovals for sessionId=%s", sessionId)
	}
	return approvals, nil
}

// --- Memory proposals ---

func (s *SqlMatterGoatStore) memoryProposalMap(m *model.MGMemoryProposal) map[string]any {
	return map[string]any{
		"Id":             m.Id,
		"SessionId":      m.SessionId,
		"AgentProfileId": m.AgentProfileId,
		"ProposedText":   m.ProposedText,
		"Scope":          m.Scope,
		"Sensitivity":    m.Sensitivity,
		"Evidence":       m.Evidence,
		"Status":         m.Status,
		"ApproverUserId": m.ApproverUserId,
		"CreateAt":       m.CreateAt,
		"ResolvedAt":     m.ResolvedAt,
		"ExpiresAt":      m.ExpiresAt,
	}
}

func (s *SqlMatterGoatStore) SaveMemoryProposal(m *model.MGMemoryProposal) (*model.MGMemoryProposal, error) {
	query := s.getQueryBuilder().Insert("MGMemoryProposals").SetMap(s.memoryProposalMap(m))
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGMemoryProposal")
	}
	return m, nil
}

func (s *SqlMatterGoatStore) UpdateMemoryProposal(m *model.MGMemoryProposal) (*model.MGMemoryProposal, error) {
	query := s.getQueryBuilder().Update("MGMemoryProposals").SetMap(s.memoryProposalMap(m)).Where(sq.Eq{"Id": m.Id})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrapf(err, "failed to update MGMemoryProposal id=%s", m.Id)
	}
	return m, nil
}

func (s *SqlMatterGoatStore) GetMemoryProposalsForSession(sessionId string) ([]*model.MGMemoryProposal, error) {
	var proposals []*model.MGMemoryProposal
	query := s.getQueryBuilder().Select(mgMemoryProposalColumns...).From("MGMemoryProposals").
		Where(sq.Eq{"SessionId": sessionId}).OrderBy("CreateAt ASC")
	if err := s.GetReplica().SelectBuilder(&proposals, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGMemoryProposals for sessionId=%s", sessionId)
	}
	return proposals, nil
}

// --- Markdown exports ---

func (s *SqlMatterGoatStore) SaveMarkdownExport(e *model.MGMarkdownExport) (*model.MGMarkdownExport, error) {
	query := s.getQueryBuilder().Insert("MGMarkdownExports").SetMap(map[string]any{
		"Id":         e.Id,
		"SessionId":  e.SessionId,
		"FileInfoId": e.FileInfoId,
		"ExportedBy": e.ExportedBy,
		"CreateAt":   e.CreateAt,
	})
	if _, err := s.GetMaster().ExecBuilder(query); err != nil {
		return nil, errors.Wrap(err, "failed to save MGMarkdownExport")
	}
	return e, nil
}

func (s *SqlMatterGoatStore) GetMarkdownExportsForSession(sessionId string) ([]*model.MGMarkdownExport, error) {
	var exports []*model.MGMarkdownExport
	query := s.getQueryBuilder().Select(mgMarkdownExportColumns...).From("MGMarkdownExports").
		Where(sq.Eq{"SessionId": sessionId}).OrderBy("CreateAt DESC")
	if err := s.GetReplica().SelectBuilder(&exports, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get MGMarkdownExports for sessionId=%s", sessionId)
	}
	return exports, nil
}
