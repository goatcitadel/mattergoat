// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package app — MatterGoat GoatCitadel agent discovery (Phase 2).
//
// Lets MatterGoat populate MGAgentProfiles from GoatCitadel's agent catalogue
// (GET /api/v1/agents) instead of operators creating GoatCitadel-backed profiles
// by hand. GoatCitadel stays canonical for the agents; MatterGoat mirrors them as
// runtime=goatcitadel profiles owned by the synthetic "goatcitadel" external owner.

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// goatCitadelDiscoveryOwnerID is the synthetic owner id under which
// GoatCitadel-discovered agent profiles are grouped.
const goatCitadelDiscoveryOwnerID = "goatcitadel"

// MGDiscoveredAgent is one agent exposed by GoatCitadel's GET /api/v1/agents.
type MGDiscoveredAgent struct {
	AgentID string
	Name    string
	RoleID  string
	Title   string
}

// goatCitadelAgentsResponse mirrors the subset of GoatCitadel's GET /api/v1/agents
// body that MatterGoat consumes. GoatCitadel returns the full AgentProfileRecord;
// only these fields are needed to mirror an agent as an MGAgentProfile.
type goatCitadelAgentsResponse struct {
	Items []struct {
		AgentID string `json:"agentId"`
		Name    string `json:"name"`
		RoleID  string `json:"roleId"`
		Title   string `json:"title"`
	} `json:"items"`
}

// ListAgents fetches the agents GoatCitadel exposes for discovery.
func (r *goatCitadelRuntime) ListAgents(rctx request.CTX) ([]MGDiscoveredAgent, error) {
	if r.endpoint == "" {
		return nil, errors.New("mattergoat: GoatCitadel runtime adapter not configured")
	}

	url := strings.TrimRight(r.endpoint, "/") + "/api/v1/agents"
	httpReq, err := http.NewRequestWithContext(rctx.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "mattergoat: build GoatCitadel agents request")
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.token)

	client := r.client
	if client == nil {
		client = &http.Client{Timeout: goatCitadelHTTPTimeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "mattergoat: call GoatCitadel agents")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, goatCitadelMaxResponseBytes))
	if err != nil {
		return nil, errors.Wrap(err, "mattergoat: read GoatCitadel agents response")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, goatCitadelHTTPError(resp.StatusCode, body)
	}

	var parsed goatCitadelAgentsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, errors.Wrap(err, "mattergoat: decode GoatCitadel agents response")
	}

	agents := make([]MGDiscoveredAgent, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		if it.AgentID == "" {
			continue
		}
		agents = append(agents, MGDiscoveredAgent{
			AgentID: it.AgentID,
			Name:    it.Name,
			RoleID:  it.RoleID,
			Title:   it.Title,
		})
	}
	return agents, nil
}

// mgDiscoveredAgentToProfile maps a discovered agent onto an MGAgentProfile.
// When an existing profile is supplied (matched by bridge agent id) its identity
// is preserved so the sync is an upsert, not a duplicate insert.
func mgDiscoveredAgentToProfile(agent MGDiscoveredAgent, existing *model.MGAgentProfile) *model.MGAgentProfile {
	displayName := agent.Name
	if displayName == "" {
		displayName = agent.Title
	}
	if displayName == "" {
		displayName = agent.AgentID
	}
	role := agent.RoleID
	if role == "" {
		role = agent.Title
	}

	profile := &model.MGAgentProfile{
		OwnerType:     model.MGOwnerTypeExternal,
		OwnerId:       goatCitadelDiscoveryOwnerID,
		DisplayName:   displayName,
		Role:          role,
		BridgeAgentId: agent.AgentID,
		Runtime:       model.MGRuntimeGoatCitadel,
		TrustLevel:    model.MGTrustAdvisory,
	}
	if existing != nil {
		profile.Id = existing.Id
		profile.BotUserId = existing.BotUserId
		profile.CreateAt = existing.CreateAt
	}
	return profile
}

// MGSyncGoatCitadelAgents discovers agents from GoatCitadel and upserts them as
// MGAgentProfiles (runtime=goatcitadel). Existing GoatCitadel-owned profiles are
// matched by bridge agent id and updated in place; new agents are created. This is
// the Phase 2 alternative to creating GoatCitadel-backed profiles by hand.
func (a *App) MGSyncGoatCitadelAgents(rctx request.CTX) ([]*model.MGAgentProfile, *model.AppError) {
	if !a.MatterGoatEnabled() {
		return nil, mgErr("MGSyncGoatCitadelAgents", "app.mattergoat.disabled", http.StatusForbidden, nil)
	}

	client, ok := a.goatCitadelClientFromConfig()
	if !ok {
		return nil, mgErr("MGSyncGoatCitadelAgents", "app.mattergoat.goatcitadel.not_configured", http.StatusBadRequest, nil)
	}

	discovered, err := client.ListAgents(rctx)
	if err != nil {
		return nil, mgErr("MGSyncGoatCitadelAgents", "app.mattergoat.goatcitadel.discovery.error", http.StatusBadGateway, err)
	}

	existing, appErr := a.MGGetAgentProfilesByOwner(model.MGOwnerTypeExternal, goatCitadelDiscoveryOwnerID)
	if appErr != nil {
		return nil, appErr
	}
	byBridgeID := make(map[string]*model.MGAgentProfile, len(existing))
	for _, p := range existing {
		if p.Runtime == model.MGRuntimeGoatCitadel && p.BridgeAgentId != "" {
			byBridgeID[p.BridgeAgentId] = p
		}
	}

	synced := make([]*model.MGAgentProfile, 0, len(discovered))
	for _, agent := range discovered {
		prior := byBridgeID[agent.AgentID]
		profile := mgDiscoveredAgentToProfile(agent, prior)
		saved, sErr := a.MGSaveAgentProfile(rctx, profile, prior != nil)
		if sErr != nil {
			return nil, sErr
		}
		synced = append(synced, saved)
	}
	return synced, nil
}
