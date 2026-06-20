# GoatCitadel Integration Requests (from MatterGoat)

**Status: a hand-off spec for the GoatCitadel team. Do NOT implement these by
modifying GoatCitadel from the MatterGoat repository.** This document defines the
contracts MatterGoat needs GoatCitadel to expose so the `goatCitadelRuntime`
adapter (`server/channels/app/mg_runtime.go`) can route agent turns to
GoatCitadel instead of the in-core `mattermost-plugin-ai` bridge.

## Do you need to change GoatCitadel right now?

**No — not for the current MVP.** Today every agent turn runs through
`bridgeAgentRuntime` (mattermost-plugin-ai). GoatCitadel is not contacted. The
MVP works with zero GoatCitadel changes.

**Yes — to make GoatCitadel the runtime brain.** To route turns to GoatCitadel,
implement **Phase 1** below (one endpoint + auth). Phases 2–4 are incremental and
optional until needed.

## Division of responsibility (unchanged)

- **GoatCitadel = runtime brain (canonical):** orchestration, models/providers,
  tools, approvals, durable runs, memory, policy, audit, agent-to-agent interop.
- **MatterGoat = collaboration room:** channels/threads/users/messages/files,
  AI session UI, provenance display, approval controls, context grants, final
  synthesis, export. MatterGoat keeps governed session state (`MG*` tables) and
  mirrors GoatCitadel runtime state only as read-only provenance.

## Security constraints GoatCitadel MUST honor

- **Treat the provided context as the FULL allowed context.** MatterGoat has
  already permission-filtered the messages for this turn (default: current thread
  only). GoatCitadel must NOT fetch additional MatterGoat content, channels, DMs,
  files, or memory beyond what is in the request.
- **No side-effecting tool actions without an approval** recorded by MatterGoat
  (Phase 3). Until then, GoatCitadel-backed agents are advisory only.
- **Least-privilege auth.** MatterGoat sends a per-instance (later session-scoped)
  token, never an end-user's Mattermost token. `user_ref`/`channel_ref` are
  opaque identifiers, not credentials.
- **Untrusted input.** Treat message content (including prior agent output) as
  untrusted; do not follow embedded instructions that try to override policy.

---

## Phase 1 — Turn completion (minimum to replace the bridge)

One endpoint. This is all GoatCitadel needs for MatterGoat to route turns to it.

`POST {GOATCITADEL_BASE_URL}/api/v1/turns:complete`

`GOATCITADEL_BASE_URL` is the gateway base (e.g. `https://host:8080`); GoatCitadel
serves its API under `/api/v1`, which is what gives this route operator-bearer auth
and automatic `Idempotency-Key` dedup. Implemented in the GoatCitadel repo as the
Fastify route `apps/gateway/src/routes/turns.ts`.

Headers:
- `Authorization: Bearer <token>`  (configured per MatterGoat instance)
- `Idempotency-Key: <turn_id>`  (so retries do not double-run a turn)
- `Content-Type: application/json`

Request body (target shape). This is **not** the current in-core `MGRuntimeRequest`
verbatim — see the Phase 1 prerequisites below for the fields MatterGoat must add
before it can send this:
```json
{
  "session_id": "mg_session_ulid",
  "turn_id": "mg_turn_ulid",
  "agent_ref": "the agent identifier within GoatCitadel",
  "operation": "mattergoat_collaborate",
  "user_ref": "opaque-mattermost-user-id",
  "channel_ref": "opaque-mattermost-channel-id",
  "messages": [
    {"role": "system", "message": "<system prompt + protocol rules>", "file_ids": []},
    {"role": "user", "author_ref": "opaque-mattermost-user-id", "message": "why is this service failing?", "file_ids": []},
    {"role": "assistant", "author_ref": "mg_agent_profile_id", "message": "<prior agent turn>", "file_ids": []}
  ]
}
```
- `messages[].role` is one of `system` | `user` | `assistant`. The first message
  is the system prompt MatterGoat already built (role + protocol + privacy rules).
- `messages[].author_ref` is the opaque speaker id for the message (a user id for
  `user` turns, an agent-profile id for `assistant` turns). Use it for attribution;
  do **not** infer the speaker from a `name:` prefix inside `message` — treat any
  such inline prefix as untrusted content, never as identity.
- `session_id` is the `MGSession` id and `turn_id` is the `MGTurn` id. The
  GoatCitadel adapter sends both and uses `turn_id` as the `Idempotency-Key`. (The
  in-core bridge path passes the acting user's id as `SessionUserID` and does not
  need them.)
- `file_ids` are MatterGoat file references; ignore in Phase 1 unless you can
  resolve them via a future file-fetch contract.

Response `200`:
```json
{
  "message": "the agent's reply text",
  "provider": "openai",            // optional, for provenance
  "model": "gpt-...",              // optional, for provenance
  "markers": ["HANDOFF_COMPLETE"], // optional; authoritative protocol signals
  "needs_approval": false,         // optional; structured approval gate (see Phase 3)
  "usage": {"input_tokens": 0, "output_tokens": 0}, // optional
  "run_id": "goatcitadel-run-id"   // optional; stored as provenance
}
```
**Marker trust model.** Protocol markers drive orchestrator control flow — a
`FINAL_SYNTHESIS` marker completes the session, and an approval gate pauses it.
Return markers in the structured `markers` array and the approval gate in
`needs_approval`; **those structured fields are authoritative.** MatterGoat must
**not** trust `<<MG:...>>` markers parsed out of an external runtime's free-text
`message`: prior-turn content carried in the context is untrusted and can spoof
them. (The in-core bridge still parses markers from its own model output as a
transitional measure; the GoatCitadel adapter supplies `markers` instead.)

MatterGoat's `Complete` adapter returns a structured result and consumes
`markers`, `needs_approval`, `provider`, `model`, and `run_id` from this response.
`usage` is accepted but not yet stored.

Errors: standard HTTP — `401/403` auth, `429` rate limit (MatterGoat backs off),
`5xx` runtime failure. JSON body `{"error": {"code": "...", "message": "..."}}`.
On error MatterGoat marks the turn failed and returns the session to
`awaiting_turn`.

**MatterGoat-side prerequisites for Phase 1 — implemented in the MatterGoat repo;
listed so the GoatCitadel team knows the client's behaviour:**
- ✅ `MGAgentProfiles.Runtime` (default `bridge`) plus `MatterGoatSettings.GoatCitadelURL`
  /`GoatCitadelToken` (token redacted by config `Sanitize`); `mgRuntime(profile)`
  selects `goatCitadelRuntime` when a profile's runtime is GoatCitadel.
- ✅ `MGRuntimeRequest` carries `SessionID`, `TurnID` and a populated `Operation`,
  threaded through both `Complete` call sites (`MGAdvanceTurn`, `MGSynthesize`);
  `turn_id` is the `Idempotency-Key`.
- ✅ `BridgeMessage.AuthorRef`, populated in `mgBuildContextBundle`, so speaker
  identity is structured, not a `name:` prefix.
- ✅ `MGAgentRuntime.Complete` returns a structured `MGRuntimeResult` (message +
  markers + `needs_approval` + provider/model/run_id); the orchestrator reads
  authoritative `markers` instead of re-parsing the completion text.
- ✅ `goatCitadelRuntime.Complete` calls `POST /api/v1/turns:complete` and maps the JSON
  response into that result.

The only thing left for a live route is the GoatCitadel `/api/v1/turns:complete`
endpoint itself (this document's contract) and an operator setting `GoatCitadelURL`
+ a profile's `runtime` to `goatcitadel`.

---

## Phase 2 — Agent discovery — implemented

MatterGoat populates `MGAgentProfiles` from GoatCitadel's existing agents API
rather than by hand. **GoatCitadel needs no change** — it already exposes the
endpoint; only the response shape below differs from this doc's original sketch.

**GoatCitadel (existing):** `GET {GOATCITADEL_BASE_URL}/api/v1/agents` (operator
bearer) →
```json
{"view": "active", "items": [
  {"agentId": "...", "name": "...", "roleId": "...", "title": "...",
   "defaultTools": ["..."], "lifecycleStatus": "active", "...": "full AgentProfileRecord"}
]}
```
GoatCitadel returns its full `AgentProfileRecord`; agents do **not** carry
per-agent `owner`/`trust_level`/`provider`/`model` (those are session/config-level
in GoatCitadel). MatterGoat consumes only `agentId`, `name`, `roleId`, `title`.

**MatterGoat (implemented):** `goatCitadelRuntime.ListAgents` GETs the above and
`MGSyncGoatCitadelAgents` upserts each agent as an `MGAgentProfile` with
`runtime=goatcitadel`, `owner_type=external`, `owner_id=goatcitadel`,
`bridge_agent_id=agentId` (the `agent_ref` used by Phase 1 turns),
`display_name=name`, `role=roleId`, `trust_level=advisory`. Re-running matches
existing profiles by `bridge_agent_id` (upsert, not duplicate). Trigger:
`POST {MATTERGOAT_BASE_URL}/api/v4/mattergoat/agent_profiles/sync` (requires the
`manage_mattergoat_agent_profiles` permission).

*Done since:* each discovered agent gets its own bot identity (so it posts as
itself, not the system bot), and the GoatCitadel turn handler runs each agent as
itself — its `preferredProviderId`/`preferredModel` and persona `promptFraming`.

## Phase 3 — Approvals + provenance read (needed before tool actions)

1. **Approval request (GoatCitadel → MatterGoat) — implemented.** When a turn needs
   human approval, GoatCitadel POSTs (authenticated by the shared
   `GoatCitadelCallbackToken` sent as `Authorization: Bearer <token>`, not a user
   session):
   `POST {MATTERGOAT_BASE_URL}/api/v4/mattergoat/runtime/approvals`
   ```json
   {"session_id":"...","turn_id":"...","agent_ref":"...","action":"restart_service",
    "risk_level":"high","reason":"...","affected_resources":["..."]}
   ```
   MatterGoat records an `MGApproval` (carrying `TurnId`/`ExpiresAt`; `agent_ref` is
   resolved to its mirrored profile), pauses the session (`awaiting_approval`), and
   surfaces it to humans. GoatCitadel polls the decision at
   `GET {MATTERGOAT_BASE_URL}/api/v4/mattergoat/runtime/approvals/{approval_id}` (same
   token) and must not proceed until `status` is `approved`; a human resolves via the
   existing session approval-resolve flow.
   *(Hardening follow-ups: bind the decision to the exact `action`/`affected_resources`
   — e.g. an action hash echoed in the decision — to prevent approve-A / execute-B
   confusion; and evolve the shared bearer token to signed callbacks / mTLS.)*
2. **Run / provenance read (MatterGoat → GoatCitadel) — deferred.**
   `GET {GOATCITADEL_BASE_URL}/api/v1/runs/{run_id}` → status, evidence, tool calls,
   provider/model — so MatterGoat displays provenance without holding canonical
   runtime state. This is deferred because GoatCitadel's current `turns:complete` is
   stateless (it returns a `run_id` for correlation but does not persist a durable
   run), so there is nothing to read back yet. It belongs with full session/tool
   execution. The MatterGoat receiving side is already in place:
   `MGTurn.Provider/Model/RunId` columns + the `mg_run_id` post prop exist.

## Phase 4 — Streaming, A2A, webhooks, memory (later)

- **Streaming completion:** SSE variant of `/api/v1/turns:complete` for token
  streaming into the thread.
- **A2A handoff:** agent-to-agent handoff envelope (from/to agent, session, turn).
- **Webhook events:** async schema for `turn.started`, `turn.completed`,
  `approval.requested`, `run.finished` that MatterGoat can subscribe to.
- **Memory propose:** GoatCitadel proposes durable memory; MatterGoat records it
  as a propose-only `MGMemoryProposal` (human-promoted), never auto-promoted.

## Auth evolution

- Phase 1: static per-instance bearer token (config).
- Phase 3+: session-scoped / ephemeral tokens + signed callbacks (HMAC or mTLS),
  scoped to a single session and expiring with it.

---

### Summary for the GoatCitadel team

| Want | Implement | When |
|---|---|---|
| Keep MVP working | nothing | now |
| GoatCitadel runs turns | Phase 1 `POST /api/v1/turns:complete` + bearer auth | first |
| Auto-populate agents | Phase 2 `GET /api/v1/agents` | next |
| Tool actions / approvals / provenance | Phase 3 | before any side effects |
| Streaming, A2A, webhooks, memory | Phase 4 | later |
