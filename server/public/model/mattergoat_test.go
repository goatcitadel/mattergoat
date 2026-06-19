// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMGAgentProfilePreSaveAndValidate(t *testing.T) {
	p := &MGAgentProfile{OwnerType: MGOwnerTypeSystem, DisplayName: "Codex"}
	p.PreSave()

	assert.NotEmpty(t, p.Id)
	assert.Equal(t, MGTrustAdvisory, p.TrustLevel)
	assert.NotZero(t, p.CreateAt)
	assert.NotZero(t, p.UpdateAt)
	// JSON-ish fields get safe defaults so the store never writes NULL/invalid JSON.
	assert.Equal(t, "{}", p.ToolPolicy)
	assert.Equal(t, "[]", p.AllowedChannels)
	assert.Nil(t, p.IsValid())

	bad := &MGAgentProfile{OwnerType: "bogus", DisplayName: "x"}
	bad.PreSave()
	assert.NotNil(t, bad.IsValid())

	noName := &MGAgentProfile{OwnerType: MGOwnerTypeUser}
	noName.PreSave()
	assert.NotNil(t, noName.IsValid())
}

func TestMGSessionPreSaveDefaults(t *testing.T) {
	s := &MGSession{ChannelId: NewId(), CreatedBy: NewId()}
	s.PreSave()

	assert.NotEmpty(t, s.Id)
	assert.Equal(t, MGSessionStateCreated, s.State)
	assert.Equal(t, MGModeStrictTurns, s.Mode)
	assert.Equal(t, MatterGoatMemoryProposeOnly, s.MemoryBehavior)
	assert.Equal(t, "{}", s.ContextScope)
	assert.Nil(t, s.IsValid())

	bad := &MGSession{}
	bad.PreSave()
	assert.NotNil(t, bad.IsValid())
}

func TestMGParticipantLeastPrivilegeDefault(t *testing.T) {
	p := &MGSessionParticipant{SessionId: NewId(), AgentProfileId: NewId()}
	p.PreSave()

	// Default context grant must be least-privilege (current thread only).
	assert.Equal(t, `{"scope":"current_thread"}`, p.ContextGrant)
	assert.NotZero(t, p.JoinedAt)
	assert.Nil(t, p.IsValid())
}

func TestMatterGoatSettingsSetDefaultsAndValidate(t *testing.T) {
	s := &MatterGoatSettings{}
	s.SetDefaults()

	assert.False(t, *s.EnableAICollaboration)
	assert.Equal(t, MatterGoatProfileConservative, *s.DefaultProfile)
	assert.Equal(t, MatterGoatMemoryProposeOnly, *s.DefaultMemoryBehavior)
	assert.Equal(t, MatterGoatDefaultMaxAgentsPerSession, *s.MaxAgentsPerSession)
	assert.Nil(t, s.isValid())

	// Disabled feature skips validation entirely.
	bad := &MatterGoatSettings{EnableAICollaboration: NewPointer(false), DefaultMemoryBehavior: NewPointer("bogus")}
	assert.Nil(t, bad.isValid())

	// Enabled with a bad memory behavior must fail.
	enabledBad := &MatterGoatSettings{}
	enabledBad.SetDefaults()
	enabledBad.EnableAICollaboration = NewPointer(true)
	enabledBad.DefaultMemoryBehavior = NewPointer("bogus")
	assert.NotNil(t, enabledBad.isValid())
}
