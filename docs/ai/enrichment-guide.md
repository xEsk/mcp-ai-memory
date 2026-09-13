# Enrichment Guide

## Purpose

This document explains how summaries, keywords, tags, entities, aliases, and search phrases are generated.

Read this before changing retrieval quality, heuristic enrichment, LLM enrichment, or embeddings.

## How This Area Works

`memory.Service.Upsert` validates input, resolves the project, and calls an `enrich.Enricher` when `enrich` is true. If the configured enricher fails, it falls back to `LocalHeuristicEnricher`.

The original memory content is preserved. Enrichment only adds metadata used for retrieval.

## Heuristic Enrichment

`LocalHeuristicEnricher` works without API keys.

It generates deterministic prospective metadata:

- `summary`: first normalized content segment up to 420 Unicode runes.
- `short_summary`: first normalized content segment up to 160 Unicode runes.
- `keywords`: top 24 Unicode lowercase tokens by frequency, after a small stopword filter, including a reviewed short technical vocabulary.
- `entities`: up to 24 tokens containing uppercase letters or technical separators like `.`, `/`, `:`, `_`, `-`.
- `tags`: user tags plus type and ordered whole-token/phrase inference rules.
- `search_phrases`: generated future-query phrases using project, title, keywords, and entities, deduplicated case-insensitively with a hard cap of 18.
- `aliases`: reviewed exact-token synonym groups for API and repository vocabulary.

The heuristic does not use arbitrary substring matches: for example, `build` does not imply `ui`, `contest` does not imply `test`, and `capitalization` does not imply `api`. Equal input produces equal ordered output.

Normal `memory.search` cards project stored `short_summary` as `description`. Response shaping normalizes and caps it at 160 Unicode runes, and falls back deterministically to original content when stored metadata is empty. This is selection guidance only, not authoritative memory content. Search never calls an enrichment provider to produce the description.

These corrections apply only when a memory is new or explicitly upserted. Existing enrichment is not automatically rewritten because stored rows do not record enough enricher provenance to distinguish local, provider-generated, and legacy metadata safely. Historical remediation requires a separately approved provenance and migration design; rebuilding FTS alone preserves the existing enrichment.

## OpenAI-Compatible Enrichment

`OpenAICompatibleEnricher` posts to `<baseURL>/chat/completions` and requests strict JSON using `Prompt()`.

The prompt asks the model to optimize for AI retrieval, preserve exact technical names, extract developer-relevant terms, generate likely future queries, and avoid inventing facts.

The response parser is intentionally tolerant for provider variance. It accepts `suggested_importance` as a number or numeric string, and converts string-like arrays that may contain objects into retrieval strings.

## Embeddings

Embedding interfaces exist in `internal/embedding`.

`NoopEmbedder` keeps embeddings disabled. `OllamaEmbedder` can call `/api/embeddings`, but vector search is not wired into the v1 search path yet.

## How To Extend Existing Code

When improving heuristic enrichment:

1. Keep deterministic local behavior.
2. Preserve exact technical tokens.
3. Add tests for retrieval fields.
4. Verify FTS still indexes any new metadata.

When changing LLM enrichment:

1. Keep provider-agnostic OpenAI-compatible behavior.
2. Keep strict JSON output.
3. Preserve fallback to heuristic enrichment.
4. Do not require API keys for default operation.

## Common Mistakes To Avoid

- Do not summarize so aggressively that search loses useful names.
- Do not drop original content.
- Do not allow provider failures to break default memory upsert.
- Do not make embeddings mandatory before vector storage/search exists.

## Validation

Run:

```bash
go test ./...
```

Update `internal/enrich/heuristic_test.go` when changing local extraction behavior.

For live provider validation, set the `MEMORY_LLM_*` variables and run:

```bash
go test -tags integration ./internal/enrich -run TestOpenAICompatibleEnricher_EnrichMemory -count=1 -v
```

## Related Documents

Also read:

- `storage-search-guide.md`
- `mcp-tools-guide.md`
