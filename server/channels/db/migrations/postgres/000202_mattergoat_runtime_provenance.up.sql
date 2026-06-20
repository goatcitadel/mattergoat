-- MatterGoat runtime routing + turn provenance + approval correlation.
-- Pairs with docs/goatcitadel-integration-requests.md (Phase 1 / Phase 3 prereqs).

-- Phase 1: per-agent runtime selector (bridge today, goatcitadel later).
ALTER TABLE MGAgentProfiles ADD COLUMN IF NOT EXISTS Runtime VARCHAR(32) NOT NULL DEFAULT 'bridge';

-- Provenance mirrored read-only from the runtime onto the turn.
ALTER TABLE MGTurns ADD COLUMN IF NOT EXISTS Provider VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE MGTurns ADD COLUMN IF NOT EXISTS Model VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE MGTurns ADD COLUMN IF NOT EXISTS RunId VARCHAR(128) NOT NULL DEFAULT '';

-- Phase 3: correlate an approval to its originating turn, and expire stale ones.
ALTER TABLE MGApprovals ADD COLUMN IF NOT EXISTS TurnId VARCHAR(26) NOT NULL DEFAULT '';
ALTER TABLE MGApprovals ADD COLUMN IF NOT EXISTS ExpiresAt BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_mg_approvals_turn_id ON MGApprovals(TurnId);
