---
name: project-memory-mcp
description: Use the local project-memory MCP server to persist, retrieve, search, count, debug, suggest, and maintain AI-first technical memory for programming projects across Codex sessions. Trigger when starting work in a codebase, needing prior architecture/context/decisions/conventions/bugs/workflows, after discovering durable project knowledge, before changing unfamiliar code, when asked to remember something for future agents, or when using tools named project.upsert, project.list, memory.upsert, memory.search, memory.search.count, memory.search.debug, memory.get, memory.list, memory.delete, memory.suggest, or memory.server.info.
---

# Project Memory MCP

Use this skill to make project knowledge durable across sessions. The MCP is not a note app; use it as an AI-first memory layer for codebases: architecture, implementation decisions, APIs, conventions, bug history, workflows, domain concepts, and technical constraints.

## Core Rules

- Always provide a `project` key for memory operations. Never search, count, debug, suggest, read, or write without project scope.
- Prefer stable project keys: repository folder name, service name, or product slug. Use the same key consistently.
- Persist the canonical project key in the repository `AGENTS.md` so future agents use the same MCP project every time.
- Query memory before changing unfamiliar areas, especially architecture, APIs, workflows, and conventions.
- Prefer `memory.search.count` before full retrieval when token budget matters or when tuning a query.
- Use `memory.search` with default `precise` mode first; broaden explicitly with `balanced` or `broad` only when needed.
- Treat every `memory.search` result as a candidate card, not as memory evidence. `description`, `match_excerpt`, and `match_field` help select what to read; they are not authoritative content and may reflect generated retrieval metadata.
- Before asserting a remembered fact, making a technical decision, or changing code from a search result, call `memory.get` for the selected ID and use its full `content`. When cards are ambiguous, read multiple plausible candidates instead of trusting the first preview.
- Store only durable technical context that future coding agents can use. Do not store transient chat, secrets, credentials, personal data, or raw logs unless summarized into a useful technical fact.
- Use `enrich: true` by default only for clean technical knowledge. Use `enrich: false` when content may contain secrets, credentials, personal data, sensitive private context, or anything that should not be sent to the configured AI enrichment provider.
- Preserve exact technical names in memory content: class names, files, routes, tables, commands, config keys, services, and error names.
- Write memories for retrieval, not as human notes. Include the words, aliases, and natural-language phrases a future agent may search for.
- Keep memories focused. One memory should usually describe one decision, workflow, subsystem, bug pattern, convention, domain concept, or API area.
- Use `memory.suggest` before writing when unsure whether new information should create, update, split, replace, or be ignored.
- Use `memory.server.info` when you need to confirm the MCP server name and basic version currently loaded by the client.
- Use `memory.search.debug` only for troubleshooting retrieval quality; normal memory use should rely on slim `memory.search` responses and `memory.get` for full content.
- When Project Memory is called through `functions.exec`, never pass the complete `CallToolResult` to `text()` or serialize it equivalently. Select exactly one representation: emit error text from `content` when `isError` is true; otherwise prefer non-null `structuredContent` and fall back to text from `content`. Direct Codex MCP calls already perform this projection.

## Runtime and Maintenance Operations

- Multiple Codex tasks may run their own `stdio` Project Memory servers concurrently against the same current database. Do not treat another active runtime as an error or stop it merely to start a new task.
- Schema initialization, migrations, backup, restore, rebuild, and management operations require exclusive database access. If one reports that the database is active, stop every Project Memory runtime using that database, perform the operation, and then start the runtimes again.
- Canonical paths and symbolic links to the same database are supported and coordinate through the same lock identity.
- Do not use a hard link for `memory.db`. Project Memory rejects multiply linked database inodes because SQLite journal files are derived from the opened pathname and cannot be coordinated safely across hardlink aliases.

## Start-of-Task Workflow

1. Determine the project key. If `AGENTS.md` has a Project Memory MCP section, reuse that exact key. Otherwise derive a stable key from the repo folder or product slug.
2. If no canonical key exists, add it to the repository `AGENTS.md` before using memory:

   ```markdown
   ## Project Memory MCP

   - Canonical project key: `<project-key>`
   - Always use this value as `project` in `mcp__project_memory` tool calls.
   ```

3. Ensure the project exists with `project.upsert` when needed.
4. Count or search before acting:
   - Use `memory.search.count` for a cheap probe when the query may be too broad or when token budget matters.
   - Use `memory.search` with default `precise` mode for normal retrieval.
   - Use `mode: "balanced"` when precise returns too little for a focused query.
   - Use `mode: "broad"` for exploratory recall when noisier results are acceptable.
   - Inspect card order, title, description, source, update time, match excerpt, and match field only to select candidates. Missing query terms in a description do not prove that a result is irrelevant.
   - Call `memory.get` for each plausible selected ID before relying on the memory. Use `include_content: true` only when immediate bulk reading is justified.
5. Search semantically:
   - Architecture: `backend architecture`, `frontend organization`, `route registration`, `dependency injection`.
   - APIs: `how do we call the API`, `HTTP client`, `endpoint helper`, `auth headers`.
   - Conventions: `repository convention`, `naming`, `tests`, `error handling`.
   - Known issues: `bug`, `fallback`, `migration`, `production incident`.
6. Use results as context, but still verify against code when making changes.

## Updating Memory

Create or update memory after learning durable facts, for example:

- A confirmed architecture pattern.
- A project-specific convention.
- A recurring bug and its root cause.
- A non-obvious workflow or command.
- An API integration detail.
- A domain concept or acronym.
- A decision the user explicitly made.

Good memories are useful to a future coding agent. They should answer at least one of these questions:

- What should an agent do differently next time?
- Where is the relevant code, configuration, command, or workflow?
- Why was this decision made?
- What mistake, duplicate investigation, or unsafe change does this prevent?

Good memories are focused but self-contained. Avoid both extremes:

- Too small: a one-line fact with no `Where`, `Why`, aliases, or usage guidance is hard to retrieve and act on.
- Too large: a project-wide summary mixes unrelated concepts, becomes stale in pieces, and creates noisy search results.
- Preferred size: usually 100-400 words for one decision, workflow, subsystem, bug pattern, convention, domain concept, or API area.
- Split when one memory contains unrelated facts or would need different `type`, `tags`, `importance`, or update cadence.
- Keep a governing rule with the exceptions, recovery behavior, and gotchas needed to apply it correctly. Split only when the resulting memories have independent retrieval intent or update cadence and remain coherent on their own.
- Merge or update when a new fact is really the same decision, workflow, bug, or convention as an existing memory.

Maintain memories as current retrieval aids, not implementation history:

- Title memories with the current domain concept and distinguishing technical identifiers. Put phase numbers, dates, branches, and commit IDs in `source` or supporting context unless they are themselves the durable subject.
- After a clean cutover, superseding ADR, completed rollout, or renamed contract, search for memories that mention the replaced identifier, pending implementation, Stop Gate, or old status. Update or retire them in the same task when they would otherwise mislead future work.
- Do not retain completion ledgers, benchmark totals, request counts, token or cost totals, or one-off proof statistics unless future operation depends on the exact value. Link the durable report instead.
- Cite at least one currently resolvable canonical file, ADR, contract, schema, test, configuration path, or source symbol when one exists. A chat decision, branch name, or commit alone is insufficient after current authoritative documentation or code is available.

Prefer this content shape:

```text
Fact: <concise statement>
Where: <files/classes/routes/modules/tools/tables if known>
Why: <decision or constraint if known>
How to use: <what future agents should do>
Gotchas: <optional edge cases>
```

Make the content searchable:

- Include exact names: files, packages, symbols, commands, endpoints, config keys, database tables, tool names, and error strings.
- Include likely aliases and natural queries when useful, for example `route registration`, `HTTP routing`, and `where are routes added?`.
- State the consequence for future agents: what to do, what to avoid, or what to verify.
- Keep the `content` as the authoritative memory. The MCP may enrich it internally for search, but agents should not depend on generated summaries, keywords, aliases, scores, snippets, or match explanations as the source of truth.

Before writing, check whether the information is worth persisting:

- Write it when it is durable, project-specific, verified, and likely to help future work.
- Write it when it prevents duplicate investigation, preserves an explicit user decision, explains a non-obvious constraint, or documents a recurring workflow or bug.
- Skip it when it is temporary status, generic programming advice, raw command output, chat narration, speculative, unverified, or obvious from nearby code.
- Use `memory.suggest` when the finding may duplicate an existing memory, supersede old knowledge, need splitting, or be too weak to store.

Set `type` deliberately: `architecture`, `decision`, `api`, `implementation`, `bug`, `workflow`, `convention`, `domain`, or `other`.

Set `importance` from 1 to 5:

- 1: small local detail.
- 2: useful implementation context.
- 3: recurring workflow or subsystem knowledge.
- 4: important architecture, convention, or integration rule.
- 5: critical project rule, production incident, or explicit user decision.

## Search Guidance

Search semantically, not just literally. Ask how an agent would naturally ask later:

- `how do routes work?`
- `where is wallet recharge handled?`
- `how do we query Business Central?`
- `repository conventions`
- `known Reveni return order bug`

Search progressively instead of treating one query as final:

1. Start with one focused natural-language query in `precise` mode.
2. Use the returned cards only to choose plausible IDs; do not turn card text into a factual claim.
3. Call `memory.get` for selected IDs and reason from full `content`. Read several candidates when previews are ambiguous.
4. If results are weak, try synonyms, nearby concepts, exact technical names, or a different phrasing.
5. If the query is still focused, retry with `balanced`.
6. If exploring an unfamiliar area, use `broad` and accept that results may need more filtering.
7. Use `types` and `tags` to narrow large projects.
8. Use `memory.search.count` before fetching when a query may be too broad.
9. Use `memory.search.debug` only when retrieval quality itself is suspicious.

Use search modes deliberately:

- `precise`: default. Best for normal work because it reduces unrelated results.
- `balanced`: use when precise returns too few results but the query is still focused.
- `broad`: use for exploratory recall when you accept noisier results.

Retrieval is only as good as the memory wording. If a useful memory was hard to find, update it afterward with better aliases, exact names, or search phrases.

## Suggest Guidance

Use `memory.suggest` before `memory.upsert` when unsure what to do with new information. It does not mutate memory. Do not call it as a mandatory preflight before every write.

Skip `memory.suggest` and write directly when the new memory is clearly new, focused, durable, project-specific, and does not appear to overlap with remembered context. Use `memory.suggest` when the new information may duplicate or replace an existing memory, touches an area already documented in memory, mixes several facts, is short or generic, or would otherwise require judgment before writing.

`memory.suggest` searches for candidate memories with `precise` first. If that finds no candidates, it retries once with `balanced` to reduce false `create` suggestions when an existing memory uses different wording. The response includes `search_mode`; use it to understand whether candidates came from the strict search or the fallback.

Candidates are ranked by relevance to the suggestion input and then filtered by a conservative local semantic quality gate. Prefer `recommended_candidate_id` as the default update target only when `suggested_action` is `update_existing`; do not infer recommendation from rank or `importance`. `candidate_quality` is the structured match-quality signal (`strong`, `possible`, or `weak`). `importance` is the candidate memory's global project importance, not match quality. `confidence` is coarse guidance (`high`, `medium`, `low`, or `none`), not a guarantee.

If `memory.suggest` returns candidates with `suggested_action: "create"`, treat them as duplicate-awareness hints, not update targets. Only a candidate with `is_recommended: true` is safe as the default update candidate.

Use `next_step` as compact workflow guidance:

- `create_focused_memory`: write one focused memory if the fact is durable and verified.
- `inspect_recommended_candidate`: call `memory.get` for `recommended_candidate_id`, then update only if it is truly the same fact or decision.
- `split_input`: split unrelated facts into separate focused memories.
- `ignore_input`: do not persist weak, short, generic, or transient information.

Interpret suggestions conservatively:

- `create`: write a new focused memory if the fact is durable and verified.
- `update_existing`: inspect the recommended candidate first and update it only when it is truly the same fact or decision.
- `split`: create separate focused memories for unrelated facts.
- `ignore`: do not persist weak, short, generic, or transient information.

## Tool Reference

For exact schemas and examples, read `references/tools.md` when you need to call the MCP tools precisely or explain them to another agent.
