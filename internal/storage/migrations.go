package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ApplyMigrations creates the idempotent v1 schema for the local server.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			importance INTEGER NOT NULL DEFAULT 1,
			source TEXT NOT NULL DEFAULT '',
			access_count INTEGER NOT NULL DEFAULT 0,
			last_accessed_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS memory_enrichments (
			memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
			summary TEXT NOT NULL DEFAULT '',
			short_summary TEXT NOT NULL DEFAULT '',
			keywords_json TEXT NOT NULL DEFAULT '[]',
			tags_json TEXT NOT NULL DEFAULT '[]',
			entities_json TEXT NOT NULL DEFAULT '[]',
			aliases_json TEXT NOT NULL DEFAULT '[]',
			search_phrases_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_project_deleted ON memories(project_id, deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_project_type ON memories(project_id, type);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			memory_id UNINDEXED,
			project_key UNINDEXED,
			type,
			title,
			summary,
			short_summary,
			keywords,
			tags,
			entities,
			aliases,
			search_phrases,
			content,
			tokenize='unicode61'
		);`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("executing migration statement: %w", err)
		}
	}
	if err := ensureColumn(ctx, db, "memories", "access_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "memories", "last_accessed_at", "TEXT"); err != nil {
		return err
	}
	if err := backfillMemoryEnrichments(ctx, db); err != nil {
		return err
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	exists, err := hasColumn(ctx, db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if strings.ContainsAny(table+column, " ;'\"") {
		return fmt.Errorf("invalid table or column name")
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("adding column %s.%s: %w", table, column, err)
	}
	return nil
}

func hasColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, fmt.Errorf("checking table %s columns: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scanning table %s column info: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterating table %s columns: %w", table, err)
	}
	return false, nil
}

func backfillMemoryEnrichments(ctx context.Context, db *sql.DB) error {
	hasSummary, err := hasColumn(ctx, db, "memories", "summary")
	if err != nil {
		return err
	}
	if !hasSummary {
		return nil
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO memory_enrichments (
			memory_id, summary, short_summary, keywords_json, tags_json,
			entities_json, aliases_json, search_phrases_json, updated_at
		)
		SELECT id, summary, short_summary, keywords_json, tags_json,
			entities_json, aliases_json, search_phrases_json, updated_at
		FROM memories
		WHERE id NOT IN (SELECT memory_id FROM memory_enrichments)
	`)
	if err != nil {
		return fmt.Errorf("backfilling memory enrichments: %w", err)
	}
	return nil
}
