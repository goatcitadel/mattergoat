// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoat multi-agent AI collaboration action seam.
//
// Thin wrappers over the Client4 MatterGoat endpoints, plus thunks that load
// session data into the views.mattergoat Redux slice for the RHS panel.

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

import {ActionTypes} from 'utils/constants';

import type {ActionFuncAsync} from 'types/store';

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

// --- Redux thunks (load data into views.mattergoat) ---

export function fetchMatterGoatChannelSessions(channelId: string): ActionFuncAsync<MGSession[]> {
    return async (dispatch) => {
        const sessions = await Client4.getMatterGoatChannelSessions(channelId);
        dispatch({type: ActionTypes.RECEIVED_MATTERGOAT_CHANNEL_SESSIONS, channelId, sessions});
        return {data: sessions};
    };
}

export function fetchMatterGoatSessionDetail(sessionId: string): ActionFuncAsync<MGSession> {
    return async (dispatch) => {
        const [session, turns, approvals] = await Promise.all([
            Client4.getMatterGoatSession(sessionId),
            Client4.getMatterGoatTurns(sessionId),
            Client4.getMatterGoatApprovals(sessionId),
        ]);
        dispatch({type: ActionTypes.RECEIVED_MATTERGOAT_SESSION_DETAIL, session, turns, approvals});
        return {data: session};
    };
}

// Mark a session as the RHS focus and load its detail.
export function selectMatterGoatSession(sessionId: string): ActionFuncAsync<boolean> {
    return async (dispatch) => {
        dispatch({type: ActionTypes.SELECTED_MATTERGOAT_SESSION, sessionId});
        if (sessionId) {
            await dispatch(fetchMatterGoatSessionDetail(sessionId));
        }
        return {data: true};
    };
}
