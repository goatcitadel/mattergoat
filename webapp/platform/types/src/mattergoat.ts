// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoat multi-agent AI collaboration types.

export type MGAgentProfile = {
    id: string;
    owner_type: string;
    owner_id: string;
    display_name: string;
    role: string;
    bridge_agent_id: string;
    bot_user_id: string;
    trust_level: string;
    default_context_scope: string;
    memory_scope: string;
    tool_policy: string;
    approval_policy: string;
    allowed_channels: string;
    blocked_channels: string;
    create_at: number;
    update_at: number;
    delete_at: number;
};

export type MGSessionParticipant = {
    id: string;
    session_id: string;
    agent_profile_id: string;
    role: string;
    context_grant: string;
    joined_at: number;
};

export type MGSession = {
    id: string;
    title: string;
    channel_id: string;
    root_post_id: string;
    created_by: string;
    mode: string;
    profile: string;
    state: string;
    current_turn_agent_id: string;
    context_scope: string;
    memory_behavior: string;
    create_at: number;
    update_at: number;
    completed_at: number;
    delete_at: number;
    participants?: MGSessionParticipant[];
};

export type MGTurn = {
    id: string;
    session_id: string;
    agent_profile_id: string;
    turn_index: number;
    post_id: string;
    marker: string;
    status: string;
    started_at: number;
    completed_at: number;
};

export type MGApproval = {
    id: string;
    session_id: string;
    requested_by_agent_id: string;
    action: string;
    risk_level: string;
    affected_resources: string;
    reason: string;
    status: string;
    approver_user_id: string;
    create_at: number;
    resolved_at: number;
};

export type MGMemoryProposal = {
    id: string;
    session_id: string;
    agent_profile_id: string;
    proposed_text: string;
    scope: string;
    sensitivity: string;
    evidence: string;
    status: string;
    approver_user_id: string;
    create_at: number;
    resolved_at: number;
    expires_at: number;
};

export type MGStartSessionRequest = {
    title?: string;
    channel_id: string;
    root_post_id?: string;
    mode?: string;
    profile?: string;
    agent_profile_ids?: string[];
    context_scope?: string;
};

export type MGResolveRequest = {
    approve: boolean;
    reason?: string;
};
