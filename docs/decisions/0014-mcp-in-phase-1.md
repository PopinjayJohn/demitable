# ADR-0014: An MCP server over the vault ships in phase 1

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

The TTRPG tooling market has converged on exposing campaign data to LLM
clients over MCP. Grimoire is MCP-native; the Obsidian Local REST API plugin
ships an MCP server; several campaign managers now advertise MCP support as
a headline feature.

The argument for building it now rather than later: the phase-1 vault index
and pack interpreter already produce exactly the typed, structured view an MCP
client wants. Exposing it is thin. Deferring it means either exposing a raw
file view later (useless to a model) or rebuilding the typed layer.

The argument against: it widens the app's read surface, it drags an external
protocol into phase 1, and it has a privacy dimension that deserves thought
rather than momentum.

## Decision

**Ship an MCP server in phase 1, read-only, opt-in, token-gated.**

### What it exposes

The GM's **full** view of the campaign — not the player projection. MCP is a
GM-side tool by definition: it is the GM asking their own assistant questions
about their own campaign. Applying the player projection to it would make it
useless.

Resources:

| URI | Content |
|---|---|
| `demitable://note/<uid>` | A note: frontmatter, body, resolved links |
| `demitable://character/<uid>` | A character: parsed `data:`, **computed formula values**, rendered sheet |
| `demitable://pack/<id>` | A system pack: attributes, resources, workflows, difficulty model |
| `demitable://campaign` | Campaign index: note types, counts, the entity graph |

Tools (read-only in phase 1):

- `search_notes(query, type, tags, limit)` — literal and ranked results
- `get_character(name)` — resolved character with computed stats
- `list_by(type, filter)` — typed query over the index
- `get_links(uid, direction)` — backlinks and outgoing links
- `resolve_reference(text)` — resolve a wikilink or name to a note
- `evaluate_formula(expression, context)` — evaluate a pack formula, for
  questions the model cannot answer by reading values

### Transport and access

- Served on the same HTTP server, under `/mcp`, using the streamable HTTP
  transport.
- **Off by default.** Enabled by a setting or `--mcp` flag.
- When enabled, requires a **token** (printed at startup, or generated). The
  MCP endpoint is a much more powerful read surface than the UI, and it must
  not be reachable by anything on the network that did not deliberately opt in.
- The endpoint never binds publicly by default. If the server is bound to the
  LAN, so is MCP — hence the token, and hence it being off by default.

### Privacy, stated plainly

Connecting an LLM to this server means the campaign leaves the machine,
depending on where the model runs. For a campaign containing unrehearsed
player behaviour, unreleased secrets and a GM's notes, that is a genuine
consideration and the user must make it deliberately. The app should state this
plainly at the point of enabling the feature, not bury it in a doc. It is the
same privacy axis as phase 4's local transcription, and the project's stance
across both should be consistent: **local by default, and never silently
remote**.

## Consequences

**Good:**

- The cheapest possible differentiator, built on infrastructure phase 1
  already needs.
- The typed view — characters with computed formulas, the entity graph,
  pack definitions — is the thing a model actually finds useful, and it is the
  thing a naive file-search MCP cannot provide.
- It makes the project useful to people who already live in an LLM client.
  They do not have to learn a new UI to ask their campaign questions.

**Bad:**

- An external protocol in phase 1, and a dependency on its evolution.
- The typed layer becomes partly shaped by what MCP can express. Mitigation:
  the MCP server is a *view* over the internal model, not the model itself.
- Read-only in phase 1 means the obvious next question — "make me an NPC" —
  is unanswered, and users will ask for it immediately.

**Neutral:**

- The token requirement makes the MCP endpoint a first-class security
  surface. It needs the same care as the projection layer.
