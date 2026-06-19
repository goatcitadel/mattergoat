# AGENTS.md

Explicitly import subdirectory instruction files that must always be in context:
@server/AGENTS.md

## MatterGoat

This repository is **MatterGoat**, a fork of Mattermost that adds first-class,
governed, multi-agent AI collaboration. Before working on AI-agent / collaboration
features, read the project charter: `docs/mattergoat-ai-collaboration.md`.

Key rule: chat messages are the human-visible transcript and audit trail, **not**
the canonical runtime state — canonical state lives in the MatterGoat (`MG*`) store
tables and the orchestrator (`server/channels/app/mg_orchestrator.go`). The feature
is gated by `FeatureFlags.MatterGoatAgents` + `MatterGoatSettings.EnableAICollaboration`
(both default off); gate every entry point on `App.MatterGoatEnabled()`. Keep
branding changes to the product surface — do not rename the Go module path or
existing upstream identifiers, so upstream merges stay clean.

## Pull Requests

When creating a pull request, follow `.github/PULL_REQUEST_TEMPLATE.md` exactly:

- Remove all `<!-- -->` comments.
- Omit sections that are not applicable (Ticket Link, Screenshots) — do not write N/A, just remove the header.
- The `#### Release Note` header and its "```release-note" fenced code block **must always be present** (WITHOUT escaping the ``` characters). Write `NONE` if the change has no API, schema, UI, or breaking changes.

## Cursor Cloud Agents

This repository has a checked-in Cloud Agent environment under `.cursor/`. Docker is started by `.cursor/scripts/cloud-agent-start.sh`; if Docker is unavailable in Cloud, treat that as an environment failure rather than falling back to snapshot assumptions.

The environment declares `mattermost/enterprise` as a Cursor multi-repo dependency. Cursor clones the repositories as siblings, so `server/Makefile` can use its default `../../enterprise` path; the install hook does not clone or symlink enterprise.
