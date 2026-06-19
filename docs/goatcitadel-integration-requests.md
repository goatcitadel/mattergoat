# GoatCitadel Integration Requests (from MatterGoat)

**Status: requests / design notes only. Do NOT implement these by modifying
GoatCitadel from this repository.** GoatCitadel is an external AI
operations/runtime system; MatterGoat integrates with it across API / A2A /
webhook / channel-style boundaries. This file records contracts MatterGoat would
*like* GoatCitadel to expose so the `goatCitadelRuntime` adapter
(`server/channels/app/mg_runtime.go`) can route agent turns to GoatCitadel.

## Division of responsibility

- **GoatCitadel = runtime brain (canonical):** orchestration, models/providers,
  tools, approvals, durable runs, memory, policy, audit, agent-to-agent interop.
- **MatterGoat = collaboration room:** channels/threads/users/messages/files,
  agent presence, AI session UI, provenance display, approval controls, context
  grants, final synthesis, Markdown/session export. MatterGoat keeps governed
  session state (`MG*` tables) and mirrors GoatCitadel runtime state only as
  read-only provenance.

## Requested contracts

1. **Turn / completion API** — given a scoped, already-permission-filtered
   context bundle (messages + role/system prompt), an agent reference, and a
   session/turn id, return the agent's message plus structured metadata
   (provider/model used, protocol markers, token/cost usage, optional run id).
   Sync now; streaming later. MatterGoat will not widen the provided context.
2. **Agent registry / discovery** — list available agents (id, display name,
   owner, provider/model, trust level, tool scopes) so MatterGoat can map them to
   `MGAgentProfiles`.
3. **Approval callback contract** — when GoatCitadel needs human approval for a
   risky/side-effecting action, it calls MatterGoat (or returns an
   approval-required result); MatterGoat surfaces it as an `MGApproval`, captures
   the human decision, and posts the result back. Must be idempotent and signed.
4. **Run / provenance read API** — read-only access to a run's id, status,
   evidence, and tool calls so MatterGoat can display provenance without holding
   canonical runtime state.
5. **A2A handoff + webhook event schema** — agent-to-agent handoff envelope and
   an async event schema (turn started/completed, approval requested, run
   finished) MatterGoat can subscribe to.
6. **Auth** — session-scoped / ephemeral tokens and signed callbacks; MatterGoat
   passes least-privilege, session-bound credentials, never full user tokens.

## MatterGoat-side prerequisites (these DO belong in this repo, later)

- `MGAgentProfiles.Runtime` column + endpoint/credential config, so
  `mgRuntime()` selects `goatCitadelRuntime` per agent.
- `MatterGoatSettings` fields for the GoatCitadel endpoint and trust policy.
- Mapping of GoatCitadel run ids → `MGTurns`/post provenance props.
