# Project Memory MCP Tools

Use namespace tools exposed by the configured MCP server, typically `mcp__project_memory`.

## Canonical Project Key

Before calling memory tools in a repository, read `AGENTS.md`. If it contains:

```markdown
## Project Memory MCP

- Canonical project key: `some-key`
```

then always use `some-key` as the `project` argument.

If the section is missing, create a stable key from the repo folder or product slug, add the section to `AGENTS.md`, and run `project.upsert` once for that key.

## Projects

### `project.upsert`

Create or update a project.

```json
{
  "key": "project-slug",
  "name": "Human Project Name",
  "description": "Optional short description"
}
```

### `project.list`

List known projects. Use this when uncertain about the existing project key.

## Server Info

### `memory.server.info`

Return the currently loaded memory MCP server name and basic version.

```json
{}
```

Returns:

```json
{
  "name": "mcp-project-memory",
  "version": "0.1.0"
}
```

Use this when confirming whether a client has restarted onto the expected MCP server version. It intentionally returns only basic identity fields.

## Retrieval Workflow

Use this sequence when token budget or retrieval quality matters:

1. Call `memory.search.count` with the focused query and default mode.
2. If `count` is `0`, try `mode: "balanced"` or a broader query.
3. If `count` is too high, add `types`, `tags`, or a more specific query.
4. Call `memory.search` only when the count is useful.
5. Call `memory.get` for selected IDs when full content is needed.
6. Call `memory.search.debug` only when investigating unexpected results.

## Search Modes

- `precise`: default. Removes common stopwords and favors fewer, stronger matches.
- `balanced`: removes common stopwords but allows broader OR-style matching.
- `broad`: closest to the original broad FTS behavior; useful for exploratory recall but noisier.

## Memories

### `memory.search.count`

Count active memory matches inside one project without returning memories and without recording access.

```json
{
  "project": "project-slug",
  "query": "how do we call the API?",
  "mode": "precise",
  "types": ["architecture", "api", "convention"],
  "tags": ["frontend"]
}
```

Returns:

```json
{
  "project": "project-slug",
  "query": "how do we call the API?",
  "mode": "precise",
  "count": 3
}
```

### `memory.search`

Search active memories inside one project.

```json
{
  "project": "project-slug",
  "query": "how do we call the API?",
  "mode": "precise",
  "limit": 10,
  "types": ["architecture", "api", "convention"],
  "tags": ["frontend"],
  "include_content": false
}
```

Normal search responses are intentionally slim: ID, project, type, title, optional content, importance, source, created time, and updated time. They should not expose summaries, keywords, entities, aliases, snippets, scores, or match explanations.

Use `include_content: true` only when the full content is needed immediately. Otherwise fetch the chosen memory with `memory.get`.

### `memory.search.debug`

Inspect search ranking details for troubleshooting retrieval quality.

```json
{
  "project": "project-slug",
  "query": "how do routes work?",
  "mode": "balanced",
  "limit": 10,
  "types": ["architecture"],
  "tags": ["api"]
}
```

Returns slim memory data plus diagnostic fields such as `snippet`, `score`, and `match_explanation`. Do not use this as the default retrieval path.

### `memory.upsert`

Create or update a memory.

```json
{
  "project": "project-slug",
  "id": "optional-existing-id",
  "type": "architecture",
  "title": "API calls go through Server wrapper",
  "content": "Fact: API calls use the Server class. Where: assets/js/Server.ts. How to use: add new endpoints through the wrapper so auth headers and base URL stay consistent.",
  "tags": ["api", "frontend"],
  "importance": 4,
  "source": "codex investigation",
  "enrich": true
}
```

Use `enrich: true` by default for clean technical knowledge. The server preserves original content and stores generated retrieval metadata separately.

Use `enrich: false` when the memory may contain secrets, credentials, personal data, sensitive private context, raw logs, or anything that should not be sent to the configured AI enrichment provider. Prefer summarizing sensitive findings into a clean technical fact before storing them.

### `memory.get`

Fetch one memory by ID and project. Use when a search result needs full content or verification.

```json
{
  "project": "project-slug",
  "id": "memory-id"
}
```

### `memory.list`

List memories in a project.

```json
{
  "project": "project-slug",
  "type": "workflow",
  "tag": "deploy",
  "limit": 50,
  "offset": 0
}
```

### `memory.delete`

Soft-delete a memory. Use only when a memory is wrong, obsolete, or harmful.

```json
{
  "project": "project-slug",
  "id": "memory-id",
  "reason": "superseded by new workflow"
}
```

### `memory.suggest`

Analyze new information and return a structured suggestion without modifying memory.

```json
{
  "project": "project-slug",
  "input": "New technical finding or user decision",
  "context": "Optional task context"
}
```

Use this before writing when the information may duplicate, replace, split, or be too transient to store.

Do not call this as a mandatory preflight before every `memory.upsert`. Use it when there is uncertainty about duplicate, update, split, or ignore behavior.

`memory.suggest` searches with `precise` first. If no candidates are found, it retries once with `balanced` so existing memories are less likely to be missed when the new information uses different wording. The response includes `search_mode` with the mode that produced the returned candidates, or `balanced` when both searches found no candidates. Returned candidates then pass through a conservative local semantic quality gate; finding a candidate is not enough to recommend updating it.

Returns:

```json
{
  "project": "project-slug",
  "suggested_action": "update_existing",
  "reason": "a strong existing memory candidate was found",
  "search_mode": "precise",
  "recommended_candidate_id": "memory-id",
  "confidence": "high",
  "next_step": "inspect_recommended_candidate",
  "candidate_count": 1,
  "candidates": [
    {
      "rank": 1,
      "is_recommended": true,
      "candidate_quality": "strong",
      "memory": {
        "id": "memory-id",
        "project": "project-slug",
        "type": "bug",
        "title": "Existing related memory",
        "importance": 4,
        "source": "codex investigation",
        "created_at": "2026-07-01T08:00:00Z",
        "updated_at": "2026-07-01T08:00:00Z"
      },
      "match_reason": "strong title and metadata overlap"
    }
  ],
  "modifies_memory": false
}
```

Interpretation:

- `recommended_candidate_id` is the safest default candidate to inspect when `suggested_action` is `update_existing`; it is empty when candidates exist but none are strong enough.
- `candidate_count` is the number of returned candidates.
- `confidence` is coarse guidance: `high`, `medium`, `low`, or `none`.
- `next_step` is compact workflow guidance: `create_focused_memory`, `inspect_recommended_candidate`, `split_input`, or `ignore_input`.
- `candidates` are ranked by relevance to the suggestion input, but rank alone is not a recommendation.
- `is_recommended` is true only for the recommended update target.
- `candidate_quality` is structured match quality: `strong`, `possible`, or `weak`.
- `memory.importance` is global memory importance, not match quality.
- `match_reason` is short semantic guidance such as `strong title and metadata overlap`, `possible semantic overlap, review before updating`, or `matched only broad project terms`. It is not a debug score or raw search explanation.
- `modifies_memory: false` means `memory.suggest` did not write, update, or delete anything.

Suggested action guidance:

- `create`: write a new focused memory.
- `update_existing`: inspect `recommended_candidate_id` first and update an existing memory only if it is truly the same fact.
- `split`: create separate memories for unrelated facts.
- `ignore`: do not persist it.

## Memory Quality Workflow

Use memory as durable operating context for future agents, not as a chat transcript.

Before `memory.upsert`, ask:

- Is this specific to this project?
- Is it verified against code, config, tests, or an explicit user decision?
- Will it help a future agent avoid broad repo analysis, repeated debugging, or an unsafe change?
- Can it be written as a small fact with `Where`, `Why`, and `How to use`?

Prefer one focused memory per durable fact. Split unrelated facts instead of creating a long mixed memory.

Use this content pattern:

```text
Fact: <one durable project-specific fact>
Where: <files, packages, commands, routes, tables, config keys, or services>
Why: <decision, constraint, root cause, or tradeoff>
How to use: <what a future agent should do differently>
Gotchas: <optional edge cases or failure mode>
```

Avoid storing:

- Temporary progress updates.
- Raw logs or command output unless summarized into a reusable fact.
- Generic programming advice that applies to any project.
- Secrets, credentials, tokens, personal data, or sensitive private context.
- Facts that are obvious from opening the target file.

## Good Memory Examples

### Architecture

```text
Fact: REST routes are registered through RouteRegistrar, not directly in handlers.
Where: internal/http/routes.go and internal/http/registrar.go.
Why: keeps route ownership centralized and enables middleware ordering.
How to use: add new endpoints by extending the registrar for the bounded context.
```

### Bug

```text
Fact: Reveni partial validation can fail with 422 when the wrong account key is used.
Where: ReveniTrueArtistShopConnector and return-order processed consumer.
Root cause: TA shops require REVENI_TA_API_KEY account configuration.
How to use: verify account selection before debugging payload shape.
```

### Workflow

```text
Fact: Frontend assets are compiled with Gulp; do not edit hashed public assets manually.
Where: Application/Assets and public/assets.
How to use: edit source SCSS/JS and run the project build command.
```

### Decision

```text
Fact: MCP tool responses expose clean memory fields only; enrichment metadata stays internal.
Where: internal/mcpserver/responses.go.
Why: agents need the saved memory, while summaries, keywords, aliases, snippets, scores, and access metrics are retrieval machinery.
How to use: keep normal response DTOs focused on id, project, type, title, content, importance, source, created_at, and updated_at; use memory.search.debug for ranking diagnostics.
```

### Bad Examples

```text
I changed the migration and tests passed.
```

```text
Remember to write good code and use tests.
```

```text
Here is the full failing log: <large raw log>
```
