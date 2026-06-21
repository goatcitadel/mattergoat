// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatterGoatStore is a CRUD smoke test that exercises the MatterGoat store
// SQL against a real database: every table is written and read back, the
// least-privilege participant default round-trips, soft vs hard deletes behave
// as the getters expect, and the MGTurns -> MGSessions foreign key is enforced.
func TestMatterGoatStore(t *testing.T) {
	StoreTest(t, func(t *testing.T, rctx request.CTX, ss store.Store) {
		mg := ss.MatterGoat()

		t.Run("AgentProfile save/get/list/update/soft-delete", func(t *testing.T) {
			ownerID := model.NewId()
			p := &model.MGAgentProfile{
				OwnerType:     model.MGOwnerTypeUser,
				OwnerId:       ownerID,
				DisplayName:   "Smoke Agent",
				Role:          "reviewer",
				BridgeAgentId: "bridge-1",
			}
			p.PreSave()

			saved, err := mg.SaveAgentProfile(p)
			require.NoError(t, err)
			require.Equal(t, p.Id, saved.Id)

			got, err := mg.GetAgentProfile(p.Id)
			require.NoError(t, err)
			assert.Equal(t, "Smoke Agent", got.DisplayName)
			assert.Equal(t, model.MGOwnerTypeUser, got.OwnerType)
			assert.Equal(t, model.MGTrustAdvisory, got.TrustLevel, "PreSave trust default round-trips")
			assert.Equal(t, "{}", got.ToolPolicy, "PreSave JSON default round-trips")
			assert.Equal(t, "[]", got.AllowedChannels)

			byOwner, err := mg.GetAgentProfilesByOwner(model.MGOwnerTypeUser, ownerID)
			require.NoError(t, err)
			require.Len(t, byOwner, 1)

			got.DisplayName = "Renamed Agent"
			got.PreUpdate()
			_, err = mg.UpdateAgentProfile(got)
			require.NoError(t, err)
			reGot, err := mg.GetAgentProfile(p.Id)
			require.NoError(t, err)
			assert.Equal(t, "Renamed Agent", reGot.DisplayName)

			require.NoError(t, mg.DeleteAgentProfile(p.Id))
			byOwnerAfter, err := mg.GetAgentProfilesByOwner(model.MGOwnerTypeUser, ownerID)
			require.NoError(t, err)
			assert.Len(t, byOwnerAfter, 0, "soft-deleted profile is filtered from owner listing")
		})

		t.Run("Session save/get/list/soft-delete", func(t *testing.T) {
			channelID := model.NewId()
			sess := &model.MGSession{
				Title:     "Smoke Session",
				ChannelId: channelID,
				CreatedBy: model.NewId(),
			}
			sess.PreSave()

			_, err := mg.SaveSession(sess)
			require.NoError(t, err)

			got, err := mg.GetSession(sess.Id)
			require.NoError(t, err)
			assert.Equal(t, model.MGSessionStateCreated, got.State, "PreSave state default round-trips")
			assert.Equal(t, model.MGModeStrictTurns, got.Mode, "PreSave mode default round-trips")
			assert.Equal(t, "{}", got.ContextScope)

			forChan, err := mg.GetSessionsForChannel(channelID)
			require.NoError(t, err)
			require.Len(t, forChan, 1)

			got.State = model.MGSessionStateCompleted
			got.CompletedAt = model.GetMillis()
			_, err = mg.UpdateSession(got)
			require.NoError(t, err)
			reGot, err := mg.GetSession(sess.Id)
			require.NoError(t, err)
			assert.Equal(t, model.MGSessionStateCompleted, reGot.State)

			require.NoError(t, mg.DeleteSession(sess.Id))
			forChanAfter, err := mg.GetSessionsForChannel(channelID)
			require.NoError(t, err)
			assert.Len(t, forChanAfter, 0, "soft-deleted session is filtered from channel listing")
		})

		t.Run("Participant least-privilege default + hard delete", func(t *testing.T) {
			sess := newSmokeSession(t, mg)
			agentProfileID := model.NewId()

			part := &model.MGSessionParticipant{
				SessionId:      sess.Id,
				AgentProfileId: agentProfileID,
				Role:           "agent",
			}
			part.PreSave()
			_, err := mg.SaveParticipant(part)
			require.NoError(t, err)

			parts, err := mg.GetParticipantsForSession(sess.Id)
			require.NoError(t, err)
			require.Len(t, parts, 1)
			assert.Equal(t, `{"scope":"current_thread"}`, parts[0].ContextGrant,
				"least-privilege context grant default persists")

			require.NoError(t, mg.DeleteParticipant(sess.Id, agentProfileID))
			partsAfter, err := mg.GetParticipantsForSession(sess.Id)
			require.NoError(t, err)
			assert.Len(t, partsAfter, 0, "participant is hard-deleted")
		})

		t.Run("Turn save/update/list ordered by TurnIndex", func(t *testing.T) {
			sess := newSmokeSession(t, mg)
			agentProfileID := model.NewId()

			// Save out of order to prove GetTurnsForSession orders by TurnIndex ASC.
			second := &model.MGTurn{SessionId: sess.Id, AgentProfileId: agentProfileID, TurnIndex: 1, PostId: model.NewId(), Marker: model.MGMarkerHandoffComplete}
			second.PreSave()
			_, err := mg.SaveTurn(second)
			require.NoError(t, err)

			first := &model.MGTurn{SessionId: sess.Id, AgentProfileId: agentProfileID, TurnIndex: 0, PostId: model.NewId(), Marker: model.MGMarkerTurnInProgress}
			first.PreSave()
			assert.Equal(t, model.MGTurnStatusInProgress, first.Status, "PreSave status default")
			_, err = mg.SaveTurn(first)
			require.NoError(t, err)

			first.Status = model.MGTurnStatusComplete
			first.CompletedAt = model.GetMillis()
			_, err = mg.UpdateTurn(first)
			require.NoError(t, err)

			turns, err := mg.GetTurnsForSession(sess.Id)
			require.NoError(t, err)
			require.Len(t, turns, 2)
			assert.Equal(t, 0, turns[0].TurnIndex)
			assert.Equal(t, 1, turns[1].TurnIndex)
			assert.Equal(t, model.MGTurnStatusComplete, turns[0].Status, "turn update persisted")
		})

		t.Run("Turn rejects unknown session (MGSessions FK enforced)", func(t *testing.T) {
			orphan := &model.MGTurn{SessionId: model.NewId(), AgentProfileId: model.NewId(), TurnIndex: 0}
			orphan.PreSave()
			_, err := mg.SaveTurn(orphan)
			require.Error(t, err, "FK to MGSessions must reject a turn with no parent session")
		})

		t.Run("Approval save/get/list with defaults", func(t *testing.T) {
			sess := newSmokeSession(t, mg)
			appr := &model.MGApproval{
				SessionId:          sess.Id,
				RequestedByAgentId: model.NewId(),
				Action:             "post_message",
			}
			appr.PreSave()
			_, err := mg.SaveApproval(appr)
			require.NoError(t, err)

			got, err := mg.GetApproval(appr.Id)
			require.NoError(t, err)
			assert.Equal(t, model.MGApprovalStatusPending, got.Status, "PreSave status default")
			assert.Equal(t, model.MGRiskMedium, got.RiskLevel, "PreSave risk default")
			assert.Equal(t, "[]", got.AffectedResources)

			got.Status = model.MGApprovalStatusApproved
			got.ApproverUserId = model.NewId()
			got.ResolvedAt = model.GetMillis()
			_, err = mg.UpdateApproval(got)
			require.NoError(t, err)

			list, err := mg.GetApprovalsForSession(sess.Id)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, model.MGApprovalStatusApproved, list[0].Status)
		})

		t.Run("MemoryProposal save/list with defaults", func(t *testing.T) {
			sess := newSmokeSession(t, mg)
			prop := &model.MGMemoryProposal{
				SessionId:      sess.Id,
				AgentProfileId: model.NewId(),
				ProposedText:   "Remember the deploy cadence is weekly.",
				Scope:          "team",
			}
			prop.PreSave()
			_, err := mg.SaveMemoryProposal(prop)
			require.NoError(t, err)

			list, err := mg.GetMemoryProposalsForSession(sess.Id)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, model.MGMemoryStatusProposed, list[0].Status, "propose-only default persists")
			assert.Equal(t, "[]", list[0].Evidence)
		})

		t.Run("MarkdownExport save/list", func(t *testing.T) {
			sess := newSmokeSession(t, mg)
			exp := &model.MGMarkdownExport{
				SessionId:  sess.Id,
				FileInfoId: model.NewId(),
				ExportedBy: model.NewId(),
			}
			exp.PreSave()
			_, err := mg.SaveMarkdownExport(exp)
			require.NoError(t, err)

			list, err := mg.GetMarkdownExportsForSession(sess.Id)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, exp.FileInfoId, list[0].FileInfoId)
		})
	})
}

// newSmokeSession persists a minimal valid MGSession and returns it, for tests
// that need a parent session to satisfy the MGSessions foreign key.
func newSmokeSession(t *testing.T, mg store.MatterGoatStore) *model.MGSession {
	t.Helper()
	sess := &model.MGSession{
		Title:     "Smoke Parent Session",
		ChannelId: model.NewId(),
		CreatedBy: model.NewId(),
	}
	sess.PreSave()
	_, err := mg.SaveSession(sess)
	require.NoError(t, err)
	return sess
}
