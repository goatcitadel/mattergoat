-- MatterGoat multi-agent AI collaboration: canonical session state.

CREATE TABLE IF NOT EXISTS MGAgentProfiles (
    Id VARCHAR(26) PRIMARY KEY,
    OwnerType VARCHAR(16) NOT NULL,
    OwnerId VARCHAR(26) NOT NULL DEFAULT '',
    DisplayName VARCHAR(128) NOT NULL,
    Role VARCHAR(64) NOT NULL DEFAULT '',
    BridgeAgentId VARCHAR(128) NOT NULL DEFAULT '',
    BotUserId VARCHAR(26) NOT NULL DEFAULT '',
    TrustLevel VARCHAR(16) NOT NULL DEFAULT 'advisory',
    DefaultContextScope VARCHAR(32) NOT NULL DEFAULT '',
    MemoryScope VARCHAR(32) NOT NULL DEFAULT '',
    ToolPolicy TEXT,
    ApprovalPolicy TEXT,
    AllowedChannels TEXT,
    BlockedChannels TEXT,
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL,
    DeleteAt BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mg_agent_profiles_owner ON MGAgentProfiles(OwnerType, OwnerId);
CREATE INDEX IF NOT EXISTS idx_mg_agent_profiles_delete_at ON MGAgentProfiles(DeleteAt);

CREATE TABLE IF NOT EXISTS MGSessions (
    Id VARCHAR(26) PRIMARY KEY,
    Title VARCHAR(255) NOT NULL DEFAULT '',
    ChannelId VARCHAR(26) NOT NULL,
    RootPostId VARCHAR(26) NOT NULL DEFAULT '',
    CreatedBy VARCHAR(26) NOT NULL,
    Mode VARCHAR(32) NOT NULL DEFAULT 'strict_turns',
    Profile VARCHAR(64) NOT NULL DEFAULT '',
    State VARCHAR(32) NOT NULL DEFAULT 'created',
    CurrentTurnAgentId VARCHAR(26) NOT NULL DEFAULT '',
    ContextScope TEXT,
    MemoryBehavior VARCHAR(32) NOT NULL DEFAULT 'propose_only',
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL,
    CompletedAt BIGINT NOT NULL DEFAULT 0,
    DeleteAt BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mg_sessions_channel_id ON MGSessions(ChannelId);
CREATE INDEX IF NOT EXISTS idx_mg_sessions_root_post_id ON MGSessions(RootPostId);
CREATE INDEX IF NOT EXISTS idx_mg_sessions_created_by ON MGSessions(CreatedBy);
CREATE INDEX IF NOT EXISTS idx_mg_sessions_delete_at ON MGSessions(DeleteAt);

CREATE TABLE IF NOT EXISTS MGSessionParticipants (
    Id VARCHAR(26) PRIMARY KEY,
    SessionId VARCHAR(26) NOT NULL,
    AgentProfileId VARCHAR(26) NOT NULL,
    Role VARCHAR(64) NOT NULL DEFAULT '',
    ContextGrant TEXT,
    JoinedAt BIGINT NOT NULL,
    FOREIGN KEY (SessionId) REFERENCES MGSessions(Id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mg_participants_session_id ON MGSessionParticipants(SessionId);
CREATE INDEX IF NOT EXISTS idx_mg_participants_agent ON MGSessionParticipants(AgentProfileId);

CREATE TABLE IF NOT EXISTS MGTurns (
    Id VARCHAR(26) PRIMARY KEY,
    SessionId VARCHAR(26) NOT NULL,
    AgentProfileId VARCHAR(26) NOT NULL DEFAULT '',
    TurnIndex INT NOT NULL DEFAULT 0,
    PostId VARCHAR(26) NOT NULL DEFAULT '',
    Marker VARCHAR(64) NOT NULL DEFAULT '',
    Status VARCHAR(16) NOT NULL DEFAULT 'in_progress',
    StartedAt BIGINT NOT NULL,
    CompletedAt BIGINT NOT NULL DEFAULT 0,
    FOREIGN KEY (SessionId) REFERENCES MGSessions(Id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mg_turns_session_id ON MGTurns(SessionId);
CREATE INDEX IF NOT EXISTS idx_mg_turns_session_index ON MGTurns(SessionId, TurnIndex);

CREATE TABLE IF NOT EXISTS MGApprovals (
    Id VARCHAR(26) PRIMARY KEY,
    SessionId VARCHAR(26) NOT NULL,
    RequestedByAgentId VARCHAR(26) NOT NULL DEFAULT '',
    Action VARCHAR(255) NOT NULL,
    RiskLevel VARCHAR(16) NOT NULL DEFAULT 'medium',
    AffectedResources TEXT,
    Reason TEXT,
    Status VARCHAR(16) NOT NULL DEFAULT 'pending',
    ApproverUserId VARCHAR(26) NOT NULL DEFAULT '',
    CreateAt BIGINT NOT NULL,
    ResolvedAt BIGINT NOT NULL DEFAULT 0,
    FOREIGN KEY (SessionId) REFERENCES MGSessions(Id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mg_approvals_session_id ON MGApprovals(SessionId);
CREATE INDEX IF NOT EXISTS idx_mg_approvals_status ON MGApprovals(Status);

CREATE TABLE IF NOT EXISTS MGMemoryProposals (
    Id VARCHAR(26) PRIMARY KEY,
    SessionId VARCHAR(26) NOT NULL,
    AgentProfileId VARCHAR(26) NOT NULL DEFAULT '',
    ProposedText TEXT NOT NULL,
    Scope VARCHAR(32) NOT NULL DEFAULT '',
    Sensitivity VARCHAR(16) NOT NULL DEFAULT '',
    Evidence TEXT,
    Status VARCHAR(16) NOT NULL DEFAULT 'proposed',
    ApproverUserId VARCHAR(26) NOT NULL DEFAULT '',
    CreateAt BIGINT NOT NULL,
    ResolvedAt BIGINT NOT NULL DEFAULT 0,
    ExpiresAt BIGINT NOT NULL DEFAULT 0,
    FOREIGN KEY (SessionId) REFERENCES MGSessions(Id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mg_memory_proposals_session_id ON MGMemoryProposals(SessionId);
CREATE INDEX IF NOT EXISTS idx_mg_memory_proposals_status ON MGMemoryProposals(Status);

CREATE TABLE IF NOT EXISTS MGMarkdownExports (
    Id VARCHAR(26) PRIMARY KEY,
    SessionId VARCHAR(26) NOT NULL,
    FileInfoId VARCHAR(26) NOT NULL DEFAULT '',
    ExportedBy VARCHAR(26) NOT NULL DEFAULT '',
    CreateAt BIGINT NOT NULL,
    FOREIGN KEY (SessionId) REFERENCES MGSessions(Id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mg_markdown_exports_session_id ON MGMarkdownExports(SessionId);
