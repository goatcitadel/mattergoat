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

`POST {GOATCITADEL_BASE_URL}/v1/turns:complete`

Headers:
- `Authorization: Bearer <token>`  (configured per MatterGoat instance)
- `Idempotency-Key: <turn_id>`  (so retries do not double-run a turn)
- `Content-Type: application/json`

Request body (maps 1:1 to MatterGoat's `MGRuntimeRequest`):
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
    {"role": "user", "message": "alice: why is this service failing?", "file_ids": []},
    {"role": "assistant", "message": "<prior agent turn>", "file_ids": []}
  ]
}
```
- `messages[].role` is one of `system` | `user` | `assistant`. The first message
  is the system prompt MatterGoat already built (role + protocol + privacy rules).
- `file_ids` are MatterGoat file references; ignore in Phase 1 unless you can
  resolve them via a future file-fetch contract.

Response `200`:
```json
{
  "message": "the agent's reply text (may include <<MG:...>> markers)",
  "provider": "openai",            // optional, for provenance
  "model": "gpt-...",              // optional, for provenance
  "markers": ["HANDOFF_COMPLETE"], // optional; MatterGoat also parses from text
  "usage": {"input_tokens": 0, "output_tokens": 0}, // optional
  "run_id": "goatcitadel-run-id"   // optional; stored as provenance
}
```
MatterGoat's adapter today consumes only `message` (string). `provider`, `model`,
`run_id`, `usage` become post provenance when MatterGoat is extended to read them.

Errors: standard HTTP — `401/403` auth, `429` rate limit (MatterGoat backs off),
`5xx` runtime failure. JSON body `{"error": {"code": "...", "message": "..."}}`.
On error MatterGoat marks the turn failed and returns the session to
`awaiting_turn`.

**MatterGoat-side prerequisites for Phase 1 (these belong in the MatterGoat repo,
not GoatCitadel):** add `MGAgentProfiles.Runtime` + a `GoatCitadelURL`/token in
`MatterGoatSettings`, and implement `goatCitadelRuntime.Complete` to call the
endpoint above; have `mgRuntime()` select it when a profile's runtime is
GoatCitadel.

---

## Phase 2 — Agent discovery (optional)

So MatterGoat can populate `MGAgentProfiles` from GoatCitadel rather than by hand.

`GET {GOATCITADEL_BASE_URL}/v1/agents`  → 
```json
{"agents": [
  {"id": "...", "display_name": "...", "owner": "team|user|system|external",
   "provider": "...", "model": "...", "trust_level": "trusted|advisory|untrusted",
   "tool_scopes": ["..."]}
]}
```

## Phase 3 — Approvals + provenance read (needed before tool actions)

1. **Approval request (GoatCitadel → MatterGoat).** When a turn needs human
   approval, GoatCitadel calls MatterGoat (signed):
   `POST {MATTERGOAT_BASE_URL}/api/v4/mattergoat/runtime/approvals`
   ```json
   {"session_id":"...","turn_id":"...","agent_ref":"...","action":"restart_service",
    "risk_level":"high","reason":"...","affected_resources":["..."]}
   ```
   MatterGoat creates an `MGApproval`, surfaces it to humans, and returns the
   decision (poll `GET .../approvals/{id}` or a signed callback). GoatCitadel must
   not proceed with the action until approved. *(This MatterGoat endpoint does not
   exist yet — it is a MatterGoat-side follow-up that pairs with this contract.)*
2. **Run / provenance read (MatterGoat → GoatCitadel).**
   `GET {GOATCITADEL_BASE_URL}/v1/runs/{run_id}` → status, evidence, tool calls,
   provider/model — so MatterGoat displays provenance without holding canonical
   runtime state.

## Phase 4 — Streaming, A2A, webhooks, memory (later)

- **Streaming completion:** SSE variant of `/v1/turns:complete` for token
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
| GoatCitadel runs turns | Phase 1 `POST /v1/turns:complete` + bearer auth | first |
| Auto-populate agents | Phase 2 `GET /v1/agents` | next |
| Tool actions / approvals / provenance | Phase 3 | before any side effects |
| Streaming, A2A, webhooks, memory | Phase 4 | later |
