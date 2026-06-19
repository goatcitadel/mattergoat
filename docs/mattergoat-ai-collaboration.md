# MatterGoat — AI Collaboration Charter

MatterGoat is a fork of **Mattermost v11.x** that adds first-class, governed,
**multi-agent AI collaboration**. Multiple AI agents — owned by different users,
teams, or the system — can join channels, threads, and DMs as **permissioned,
auditable participants**, take structured turns, debate, require human approval
for risky actions, and produce a final synthesis that can be exported to a
portable Markdown collaboration log.

This document is the durable project charter. It records the design principles,
the locked architectural decisions, and the staged roadmap. It is intentionally
upstream-compatible: MatterGoat keeps the Mattermost Go module path and existing
identifiers so upstream security patches can still be merged.

## The one design rule

> **Chat messages are the human-visible transcript and audit trail — NOT the
> canonical runtime state.**

Canonical session state (turns, approvals, evidence, provenance) lives in
explicit MatterGoat (`MG*`) database tables and the orchestrator. The
orchestrator owns truth. Protocol markers parsed from model output
(`<<MG:...>>`) are advisory hints validated against stored state — never
authoritative on their own. Chat text, files, prior agent output, and imported
Markdown are **untrusted input**, never trusted commands.

## Locked decisions

1. **Branding = product-surface only.** Rebrand the product name, the feature
   area ("MatterGoat Agents"), the `/goat` command, the `MG:` protocol markers,
   and all *new* tables/configs/permissions. Do **not** rename the Go module
   path (`github.com/mattermost/mattermost/server/v8`) or existing upstream
   identifiers — that preserves clean upstream merges.
2. **Architecture = hybrid core + external runtime behind an adapter.** Core owns
   the durable governed substrate (sessions, participants, turns, approvals,
   memory proposals, audit, provenance, permissions, Markdown export). The
   orchestrator runs each turn through the **`MGAgentRuntime` adapter**
   (`server/channels/app/mg_runtime.go`) — never a provider directly. Today the
   adapter is `bridgeAgentRuntime`, backed by the existing in-core agents bridge
   to **`mattermost-plugin-ai`** (`server/channels/app/agents.go`,
   `agents_bridge.go`). The adapter is the **GoatCitadel boundary**: GoatCitadel
   (the external AI runtime brain — orchestration, models, tools, approvals,
   durable runs, memory, policy, A2A) plugs in later as a `goatCitadelRuntime`
   over HTTP/A2A/webhook, with GoatCitadel staying canonical for runtime state
   and MatterGoat mirroring only read-only provenance. **Do not modify GoatCitadel
   from this repo** — capture needed GoatCitadel APIs/contracts as follow-ups.
3. **LLM runtime = `mattermost-plugin-ai`**, driven through the existing bridge
   (`a.ch.agentsBridge.ServiceCompletion`, `GetAgents`).
4. **Scope = full brief MVP**, delivered in dependency-ordered build waves.

## Architecture

```
Human  ──▶ /goat command or "Start AI Collaboration" RHS panel
            │
            ▼
  CORE (governed substrate — modeled on the Recap feature)
   MGSessions ─ MGSessionParticipants ─ MGTurns ─ MGApprovals
   MGMemoryProposals ─ MGMarkdownExports ─ audit (reused) ─ permissions
   provenance post props │ websocket events │ orchestrator state machine
            │  a.ch.agentsBridge.ServiceCompletion / GetAgents
            ▼
  mattermost-plugin-ai: provider routing, model calls, tools
```

The **Recap feature** is the implementation template for every new entity
(model + store interface + sqlstore impl + `store.go` registration + app logic +
api4 handler + migration + status state machine + `Auditable()`):

- `server/public/model/recap.go`
- `server/channels/store/recap_store.go`, `server/channels/store/sqlstore/recap_store.go`
- `server/channels/app/recap.go`
- `server/channels/api4/recap.go` (wired in `server/channels/api4/api.go`)
- `server/channels/db/migrations/postgres/000149_create_recaps.{up,down}.sql`

### Session state machine (`MGSessions.State`)

`created → awaiting_turn → turn_in_progress → (waiting | stale | collision) →
awaiting_approval → synthesizing → completed | aborted`

Protocol markers (rebranded `<<MG:...>>`): `PROTOCOL_ACCEPTED`,
`TURN_IN_PROGRESS`, `HANDOFF_COMPLETE`, `WAITING_FOR_HANDOFF`,
`STALE_TURN_DETECTED`, `COLLISION_DETECTED`, `PROTOCOL_VIOLATION`,
`USER_OVERRIDE`, `FINAL_SYNTHESIS`.

## Security model (do not hand-wave)

- **Context grants are the core boundary.** `BuildContextBundle` is the single
  chokepoint deciding what an agent may read; default = current thread only.
  Cross-user / cross-channel / DM / memory access requires an explicit grant
  recorded on `MGSessionParticipants` plus an audit event. One user's agent must
  never receive another user's private memory or DMs.
- **Agents ≠ their owners.** Agent actions never run with the owner's full
  session permissions; they are bounded by the participant's scoped grant and
  the agent profile's tool policy.
- **Prompt injection / instruction hierarchy.** Markers in untrusted text are
  ignored. Only the orchestrator emits authoritative state transitions. Memory
  promotion is **propose-only** and human-gated.
- **Approval gates** for side-effecting tools, external-provider sends from
  sensitive channels, posting outside the session channel, and exports —
  recorded in chat and audit (who/what/when/agent/session/scope/outcome).
- **Provenance** on every agent post (props + `MGTurns` row): agent id, owner,
  runtime, model/provider, context used, tools used, approvals, evidence.

## Feature gating

The feature is doubly gated and ships **off by default**:

- `FeatureFlags.MatterGoatAgents` (`server/public/model/feature_flags.go`)
- `MatterGoatSettings.EnableAICollaboration` (`server/public/model/config.go`)

Use `App.MatterGoatEnabled()` (`server/channels/app/mg_orchestrator.go`) as the
single gate at every entry point.

## Build waves

- **Wave 0 — Branding & scaffolding. ✅** `MatterGoatSettings` config +
  `MatterGoatAgents` feature flag (both default off), this charter, orchestrator
  scaffold. No behavior change.
- **Wave 1 — Data substrate. ✅** Migration `000201` for the 7 `MG*` tables;
  models (`public/model/mattergoat.go`); consolidated `MatterGoatStore`
  interface + sqlstore impl + store-layer wiring (timer/retry/mocks).
- **Wave 2 — Agent registry + sessions. ✅** `MGAgentProfiles` CRUD mapped to
  bridge agents; create session bound to a thread; invite participants with
  least-privilege context grants. (See permissions note below.)
- **Wave 3 — Orchestrator + turns. ✅** Turn state machine (`mg_orchestrator.go`),
  `mgBuildContextBundle` (the context-grant chokepoint), advisory marker parsing,
  provenance-tagged `custom_mg_agent_response` posts, strict turn-taking; drives
  agents via the bridge (`AgentCompletion`).
- **Wave 4 — Approvals + synthesis. ✅** Approval gate (heuristic from the agent's
  "Approval Needed" section) pausing the session; final synthesis turn.
- **Wave 5 — Markdown export + `/goat` + client API. ✅ (backend + client +
  partial UI)** AMCL export (`mg_markdown.go`), `/goat` slash command, REST API
  (`api4/mattergoat.go`), `Client4` methods (`platform/client/src/client4.ts` +
  `platform/types/src/mattergoat.ts`), a Redux/client action seam
  (`webapp/channels/src/actions/mattergoat.ts`), and a self-contained provenance
  component (`webapp/channels/src/components/mattergoat/agent_provenance.tsx`).
  **Remaining React UI:** RHS session panel, wiring the provenance component into
  the `custom_mg_agent_response` post body, an admin-console section, and `mg_*`
  websocket handlers. These require edits to large shared, deeply-typed files
  (`admin_definition.tsx` ~7k lines; the closed-set RHS enum; the universal
  `post_message_view`) and were deliberately left for a compiler-backed pass —
  the action/client/component seams are in place to make that wiring small.

### Permissions

Dedicated system-scoped permissions exist: `mg_create_session` and
`mg_use_external_agent` (granted to `system_user` + `system_admin`) and
`mg_manage_agent_profiles` (admin-only, via `allPermissionIDs`). Defined in
`public/model/permission.go`, assigned in `public/model/role.go`, and backfilled
to existing installs by the `add_mattergoat_permissions` migration
(`channels/app/permissions_migrations.go`). The API enforces
`mg_create_session` to start a session and `mg_manage_agent_profiles` for
profile CRUD; channel-level `CreatePost`/`ReadChannel` still gate per-channel
control/read.

## Finalizing (requires the Go/Node toolchain — run in Docker/CI)

The store generated-layers and mocks were hand-authored to match generated
output; canonicalize and verify them before relying on the build:

```
cd server
make store-layers store-mocks   # regenerate timer/retry/opentracing layers + mocks
make migrations-extract          # confirm migrations.list is in sync
go build ./...                   # or: make build
go test ./public/model/ -run MatterGoat
go test ./channels/app/ -run MG
cd ../webapp && npm run check-types   # type-check the client additions
```

### Out of scope for MVP

Federation / cross-server agents, CRDTs, signed/hashed records, autonomous
production actions, automatic memory promotion, cryptographically admin-blind
(E2EE) rooms, hard GoatCitadel/Obsidian dependencies, blind round-based critique.

## Data model (Wave 1 target)

New tables, prefixed `MG`, following `000149_create_recaps` conventions
(VARCHAR(26) ULID PKs; `CreateAt/UpdateAt/DeleteAt BIGINT`; FK `ON DELETE
CASCADE`; JSON-in-TEXT for arrays; indexes on lookup columns). Migrations start
at `000201` (`000200` is taken).

- `MGAgentProfiles` — agent registry (owner, role, bridge agent id, trust level,
  context/memory scope, tool & approval policy, bot user id).
- `MGSessions` — session bound to channel + root post; mode, profile, state,
  current turn, context scope, memory behavior.
- `MGSessionParticipants` — agent + per-session context grant (least privilege).
- `MGTurns` — turn index, resulting post id, marker, status, provenance.
- `MGApprovals` — risky-action approvals with risk level and audit linkage.
- `MGMemoryProposals` — propose-only memory with sensitivity and evidence.
- `MGMarkdownExports` — exported AMCL files (audited privacy downgrade).

Audit reuses the existing `model.AuditEvent*` mechanism (add `MG*` constants);
no new audit table.
