// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func TestGoatCitadelListAgents(t *testing.T) {
	t.Run("maps the GoatCitadel agents response", func(t *testing.T) {
		var gotAuth, gotPath, gotMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotMethod = r.Method
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"view":"active","items":[
				{"agentId":"a1","name":"Researcher","roleId":"research","title":"Research Agent"},
				{"agentId":"a2","name":"","roleId":"","title":"Reviewer"},
				{"agentId":"","name":"skip","roleId":"x","title":"skip"}
			]}`))
		}))
		defer server.Close()

		rt := &goatCitadelRuntime{endpoint: server.URL, token: "secret", client: server.Client()}
		agents, err := rt.ListAgents(request.TestContext(t))
		require.NoError(t, err)

		require.Len(t, agents, 2) // the entry with an empty agentId is dropped
		assert.Equal(t, "a1", agents[0].AgentID)
		assert.Equal(t, "Researcher", agents[0].Name)
		assert.Equal(t, "research", agents[0].RoleID)
		assert.Equal(t, "a2", agents[1].AgentID)

		assert.Equal(t, http.MethodGet, gotMethod)
		assert.Equal(t, "/api/v1/agents", gotPath)
		assert.Equal(t, "Bearer secret", gotAuth)
	})

	t.Run("surfaces non-200 errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
		}))
		defer server.Close()

		rt := &goatCitadelRuntime{endpoint: server.URL, token: "x", client: server.Client()}
		_, err := rt.ListAgents(request.TestContext(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("fails clearly when unconfigured", func(t *testing.T) {
		rt := &goatCitadelRuntime{}
		_, err := rt.ListAgents(request.TestContext(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}

func TestMGDiscoveredAgentToProfile(t *testing.T) {
	// New agent → create-shaped profile.
	p := mgDiscoveredAgentToProfile(MGDiscoveredAgent{AgentID: "a1", Name: "Researcher", RoleID: "research"}, nil)
	assert.Equal(t, model.MGOwnerTypeExternal, p.OwnerType)
	assert.Equal(t, goatCitadelDiscoveryOwnerID, p.OwnerId)
	assert.Equal(t, "Researcher", p.DisplayName)
	assert.Equal(t, "research", p.Role)
	assert.Equal(t, "a1", p.BridgeAgentId)
	assert.Equal(t, model.MGRuntimeGoatCitadel, p.Runtime)
	assert.Equal(t, model.MGTrustAdvisory, p.TrustLevel)
	assert.Empty(t, p.Id)

	// Missing name/role fall back to title, then agent id.
	p2 := mgDiscoveredAgentToProfile(MGDiscoveredAgent{AgentID: "a2", Title: "Reviewer"}, nil)
	assert.Equal(t, "Reviewer", p2.DisplayName)
	assert.Equal(t, "Reviewer", p2.Role)

	// Existing profile → upsert preserves identity (no duplicate insert).
	existing := &model.MGAgentProfile{Id: "prof1", BotUserId: "bot1", CreateAt: 123}
	p3 := mgDiscoveredAgentToProfile(MGDiscoveredAgent{AgentID: "a1", Name: "Researcher v2"}, existing)
	assert.Equal(t, "prof1", p3.Id)
	assert.Equal(t, "bot1", p3.BotUserId)
	assert.Equal(t, int64(123), p3.CreateAt)
	assert.Equal(t, "Researcher v2", p3.DisplayName)
}

func TestMGGoatCitadelBotUsername(t *testing.T) {
	u := mgGoatCitadelBotUsername("agent_01HXYZ")

	// Deterministic, stable across calls (same bot on re-sync).
	assert.Equal(t, u, mgGoatCitadelBotUsername("agent_01HXYZ"))
	// gc- + 16 hex chars = 19, a valid Mattermost username.
	assert.True(t, strings.HasPrefix(u, "gc-"))
	assert.Len(t, u, 19)
	assert.True(t, model.IsValidUsername(u))
	// Distinct agents get distinct usernames.
	assert.NotEqual(t, u, mgGoatCitadelBotUsername("agent_other"))
}
