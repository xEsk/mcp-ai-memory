package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// RebuildFTS recreates derived FTS documents from active authoritative rows.
func RebuildFTS(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting FTS rebuild transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_fts`); err != nil {
		return fmt.Errorf("clearing FTS index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memory_fts (
			memory_id, project_key, type, title, summary, short_summary,
			keywords, tags, entities, aliases, search_phrases, content
		)
		SELECT m.id, p.key, m.type, m.title,
			COALESCE(e.summary, ''), COALESCE(e.short_summary, ''),
			COALESCE(REPLACE(REPLACE(e.keywords_json, '["', ''), '"]', ''), ''),
			COALESCE(REPLACE(REPLACE(e.tags_json, '["', ''), '"]', ''), ''),
			COALESCE(REPLACE(REPLACE(e.entities_json, '["', ''), '"]', ''), ''),
			COALESCE(REPLACE(REPLACE(e.aliases_json, '["', ''), '"]', ''), ''),
			COALESCE(REPLACE(REPLACE(e.search_phrases_json, '["', ''), '"]', ''), ''),
			m.content
		FROM memories m
		JOIN projects p ON p.id = m.project_id
		LEFT JOIN memory_enrichments e ON e.memory_id = m.id
		WHERE m.deleted_at IS NULL
	`); err != nil {
		return fmt.Errorf("rebuilding FTS index: %w", err)
	}
	var activeCount, ftsCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE deleted_at IS NULL`).Scan(&activeCount); err != nil {
		return fmt.Errorf("counting active memories: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_fts`).Scan(&ftsCount); err != nil {
		return fmt.Errorf("counting rebuilt FTS rows: %w", err)
	}
	if activeCount != ftsCount {
		return fmt.Errorf("rebuilt FTS row count = %d, want %d active memories", ftsCount, activeCount)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing FTS rebuild: %w", err)
	}
	return nil
}
