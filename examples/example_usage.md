# Example Usage

Client config examples:

- `claude_desktop_config.json` for Claude Desktop.
- `opencode.json` for OpenCode.

1. Create a project with `project.upsert`.
2. Store a memory with `memory.upsert`.
3. Search for compact candidate cards with `memory.search`.
4. Select plausible IDs, then read their authoritative content with `memory.get`.

Example search input:

```json
{
  "project": "example-api",
  "query": "how are API routes wired?",
  "limit": 10
}
```

Search cards are selection guidance only. `description` summarizes the topic, while `match_excerpt` and `match_field` indicate why FTS may have matched. None replaces the stored memory content, and a description may omit the searched detail.

Read a selected memory before relying on it:

```json
{
  "project": "example-api",
  "id": "selected-memory-id"
}
```

Use `include_content: true` on `memory.search` only when immediate bulk reading is justified.
