// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoat multi-agent AI collaboration action seam.
//
// Thin wrappers over the Client4 MatterGoat endpoints. Kept as a dedicated
// module so UI (RHS panel, admin tools) and future Redux reducers have a stable
// place to hang side effects; today they simply proxy to the client.

import {Client4} from 'mattermost-redux/client';

import type {
    MGAgentProfile,
    MGSession,
    MGTurn,
    MGApproval,
    MGMemoryProposal,
    MGStartSessionRequest,
    MGResolveRequest,
} from '@mattermost/types/mattergoat';

export function getMatterGoatAgentProfiles(ownerType = '', ownerId = ''): Promise<MGAgentProfile[]> {
    return Client4.getMatterGoatAgentProfiles(ownerType, ownerId);
}

export function createMatterGoatAgentProfile(profile: Partial<MGAgentProfile>): Promise<MGAgentProfile> {
    return Client4.createMatterGoatAgentProfile(profile);
}

export function updateMatterGoatAgentProfile(profileId: string, profile: Partial<MGAgentProfile>): Promise<MGAgentProfile> {
    return Client4.updateMatterGoatAgentProfile(profileId, profile);
}

export function deleteMatterGoatAgentProfile(profileId: string) {
    return Client4.deleteMatterGoatAgentProfile(profileId);
}

export function startMatterGoatSession(request: MGStartSessionRequest): Promise<MGSession> {
    return Client4.startMatterGoatSession(request);
}

export function getMatterGoatSession(sessionId: string): Promise<MGSession> {
    return Client4.getMatterGoatSession(sessionId);
}

export function getMatterGoatChannelSessions(channelId: string): Promise<MGSession[]> {
    return Client4.getMatterGoatChannelSessions(channelId);
}

export function abortMatterGoatSession(sessionId: string) {
    return Client4.abortMatterGoatSession(sessionId);
}

export function advanceMatterGoatSession(sessionId: string): Promise<MGSession> {
    return Client4.advanceMatterGoatSession(sessionId);
}

export function runMatterGoatSession(sessionId: string): Promise<MGSession> {
    return Client4.runMatterGoatSession(sessionId);
}

export function synthesizeMatterGoatSession(sessionId: string): Promise<MGSession> {
    return Client4.synthesizeMatterGoatSession(sessionId);
}

export function addMatterGoatParticipant(sessionId: string, agentProfileId: string, contextGrant = ''): Promise<MGSession> {
    return Client4.addMatterGoatParticipant(sessionId, agentProfileId, contextGrant);
}

export function getMatterGoatTurns(sessionId: string): Promise<MGTurn[]> {
    return Client4.getMatterGoatTurns(sessionId);
}

export function getMatterGoatApprovals(sessionId: string): Promise<MGApproval[]> {
    return Client4.getMatterGoatApprovals(sessionId);
}

export function resolveMatterGoatApproval(approvalId: string, request: MGResolveRequest): Promise<MGApproval> {
    return Client4.resolveMatterGoatApproval(approvalId, request);
}

export function getMatterGoatMemoryProposals(sessionId: string): Promise<MGMemoryProposal[]> {
    return Client4.getMatterGoatMemoryProposals(sessionId);
}

export function resolveMatterGoatMemoryProposal(sessionId: string, proposalId: string, request: MGResolveRequest) {
    return Client4.resolveMatterGoatMemoryProposal(sessionId, proposalId, request);
}

export function exportMatterGoatSession(sessionId: string): Promise<{markdown: string}> {
    return Client4.exportMatterGoatSession(sessionId);
}
