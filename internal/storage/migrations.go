package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUnsupportedSchema = errors.New("unsupported database schema")

type migration struct {
	version int
	name    string
	payload string
	apply   func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		payload: "projects|memories|memory_enrichments|memory_fts|idx_memories_project_deleted|idx_memories_project_type",
		apply:   applyInitialSchema,
	},
	{
		version: 2,
		name:    "project_soft_delete",
		payload: "projects.deleted_at|idx_projects_deleted",
		apply:   applyProjectSoftDelete,
	},
}

// ApplyMigrations applies the current schema to a classified writable database.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	state, err := classifyDatabase(ctx, db)
	if err != nil {
		return err
	}
	return applyClassifiedMigrations(ctx, db, state)
}

func applyClassifiedMigrations(ctx context.Context, db *sql.DB, state schemaState) error {
	switch state {
	case schemaEmpty:
		if err := applyInitialMigration(ctx, db); err != nil {
			return err
		}
		return applyPendingMigrations(ctx, db, 1)
	case schemaLegacyUnversioned:
		if err := adoptSchema(ctx, db, 1); err != nil {
			return err
		}
		return applyPendingMigrations(ctx, db, 1)
	case schemaCurrentUnversioned:
		return adoptSchema(ctx, db, len(migrations))
	case schemaVersionedNeedsMigration:
		return applyPendingMigrations(ctx, db, 1)
	case schemaCurrentVersioned:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedSchema, state)
	}
}

func applyInitialMigration(ctx context.Context, db *sql.DB) error {
	migration := migrations[0]
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration %d transaction: %w", migration.version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("creating migration ledger: %w", err)
	}
	if err := migration.apply(ctx, tx); err != nil {
		return fmt.Errorf("applying migration %d: %w", migration.version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`, migration.version, migration.name, migrationChecksum(migration), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("recording migration %d: %w", migration.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %d: %w", migration.version, err)
	}
	return nil
}

func applyPendingMigrations(ctx context.Context, db *sql.DB, applied int) error {
	for _, migration := range migrations[applied:] {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("starting migration %d transaction: %w", migration.version, err)
		}
		if err := migration.apply(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("applying migration %d: %w", migration.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`, migration.version, migration.name, migrationChecksum(migration), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", migration.version, err)
		}
	}
	return nil
}

func adoptSchema(ctx context.Context, db *sql.DB, through int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting schema adoption transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("creating migration ledger: %w", err)
	}
	for _, migration := range migrations[:through] {
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`, migration.version, migration.name, migrationChecksum(migration), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("recording schema adoption: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing schema adoption: %w", err)
	}
	return nil
}

func applyProjectSoftDelete(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE projects ADD COLUMN deleted_at TEXT`,
		`CREATE INDEX idx_projects_deleted ON projects(deleted_at, key)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func applyInitialSchema(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`CREATE TABLE projects (id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE memories (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT, type TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, importance INTEGER NOT NULL DEFAULT 1, source TEXT NOT NULL DEFAULT '', access_count INTEGER NOT NULL DEFAULT 0, last_accessed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT)`,
		`CREATE TABLE memory_enrichments (memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE, summary TEXT NOT NULL DEFAULT '', short_summary TEXT NOT NULL DEFAULT '', keywords_json TEXT NOT NULL DEFAULT '[]', tags_json TEXT NOT NULL DEFAULT '[]', entities_json TEXT NOT NULL DEFAULT '[]', aliases_json TEXT NOT NULL DEFAULT '[]', search_phrases_json TEXT NOT NULL DEFAULT '[]', updated_at TEXT NOT NULL)`,
		`CREATE INDEX idx_memories_project_deleted ON memories(project_id, deleted_at)`,
		`CREATE INDEX idx_memories_project_type ON memories(project_id, type)`,
		`CREATE VIRTUAL TABLE memory_fts USING fts5(memory_id UNINDEXED, project_key UNINDEXED, type, title, summary, short_summary, keywords, tags, entities, aliases, search_phrases, content, tokenize='unicode61')`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func migrationChecksum(migration migration) string {
	value := "mcp-ai-memory-migration-v1\x00" + fmt.Sprint(migration.version) + "\x00" + migration.name + "\x00" + migration.payload
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type schemaState string

const (
	schemaEmpty                   schemaState = "empty"
	schemaLegacyUnversioned       schemaState = "recognized unversioned v1"
	schemaCurrentUnversioned      schemaState = "current unversioned v2"
	schemaVersionedNeedsMigration schemaState = "recognized versioned v1"
	schemaCurrentVersioned        schemaState = "current versioned v2"
	schemaUnknown                 schemaState = "unknown or partial"
	schemaFuture                  schemaState = "newer versioned schema"
)

func classifyDatabase(ctx context.Context, db *sql.DB) (schemaState, error) {
	tables, err := tableNames(ctx, db)
	if err != nil {
		return schemaUnknown, err
	}
	if len(tables) == 0 {
		return schemaEmpty, nil
	}
	_, versioned := tables["schema_migrations"]
	schemaVersion := 0
	for _, candidate := range []int{len(migrations), 1} {
		if !hasExpectedObjects(tables, versioned, candidate) {
			continue
		}
		valid, err := hasColumnSignatures(ctx, db, versioned, candidate)
		if err != nil {
			return schemaUnknown, err
		}
		if !valid {
			continue
		}
		valid, err = hasSchemaDefinitions(ctx, db, versioned, candidate)
		if err != nil {
			return schemaUnknown, err
		}
		if valid {
			schemaVersion = candidate
			break
		}
	}
	if schemaVersion == 0 {
		return schemaUnknown, nil
	}
	if !versioned {
		if schemaVersion == len(migrations) {
			return schemaCurrentUnversioned, nil
		}
		return schemaLegacyUnversioned, nil
	}
	ledgerVersion, state, err := classifyLedger(ctx, db)
	if err != nil || state == schemaUnknown || state == schemaFuture {
		return state, err
	}
	if ledgerVersion != schemaVersion {
		return schemaUnknown, nil
	}
	if ledgerVersion < len(migrations) {
		return schemaVersionedNeedsMigration, nil
	}
	return schemaCurrentVersioned, nil
}

func tableNames(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("listing schema objects: %w", err)
	}
	defer rows.Close()
	tables := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning schema object: %w", err)
		}
		tables[name] = struct{}{}
	}
	return tables, rows.Err()
}

func hasExpectedObjects(tables map[string]struct{}, versioned bool, schemaVersion int) bool {
	expected := []string{
		"projects", "memories", "memory_enrichments", "idx_memories_project_deleted", "idx_memories_project_type", "memory_fts",
		"memory_fts_config", "memory_fts_content", "memory_fts_data", "memory_fts_docsize", "memory_fts_idx",
	}
	if versioned {
		expected = append(expected, "schema_migrations")
	}
	if schemaVersion >= 2 {
		expected = append(expected, "idx_projects_deleted")
	}
	if len(tables) != len(expected) {
		return false
	}
	for _, name := range expected {
		if _, ok := tables[name]; !ok {
			return false
		}
	}
	return true
}

type schemaColumn struct {
	name, columnType, defaultValue string
	notNull, primaryKey            int
}

func hasColumnSignatures(ctx context.Context, db *sql.DB, versioned bool, schemaVersion int) (bool, error) {
	expected := map[string][]schemaColumn{
		"projects": {
			{"id", "TEXT", "", 0, 1}, {"key", "TEXT", "", 1, 0}, {"name", "TEXT", "", 1, 0},
			{"description", "TEXT", "''", 1, 0}, {"created_at", "TEXT", "", 1, 0}, {"updated_at", "TEXT", "", 1, 0},
		},
		"memories": {
			{"id", "TEXT", "", 0, 1}, {"project_id", "TEXT", "", 1, 0}, {"type", "TEXT", "", 1, 0}, {"title", "TEXT", "", 1, 0}, {"content", "TEXT", "", 1, 0},
			{"importance", "INTEGER", "1", 1, 0}, {"source", "TEXT", "''", 1, 0}, {"access_count", "INTEGER", "0", 1, 0}, {"last_accessed_at", "TEXT", "", 0, 0},
			{"created_at", "TEXT", "", 1, 0}, {"updated_at", "TEXT", "", 1, 0}, {"deleted_at", "TEXT", "", 0, 0},
		},
		"memory_enrichments": {
			{"memory_id", "TEXT", "", 0, 1}, {"summary", "TEXT", "''", 1, 0}, {"short_summary", "TEXT", "''", 1, 0},
			{"keywords_json", "TEXT", "'[]'", 1, 0}, {"tags_json", "TEXT", "'[]'", 1, 0}, {"entities_json", "TEXT", "'[]'", 1, 0},
			{"aliases_json", "TEXT", "'[]'", 1, 0}, {"search_phrases_json", "TEXT", "'[]'", 1, 0}, {"updated_at", "TEXT", "", 1, 0},
		},
		"memory_fts": {
			{"memory_id", "", "", 0, 0}, {"project_key", "", "", 0, 0}, {"type", "", "", 0, 0}, {"title", "", "", 0, 0},
			{"summary", "", "", 0, 0}, {"short_summary", "", "", 0, 0}, {"keywords", "", "", 0, 0}, {"tags", "", "", 0, 0},
			{"entities", "", "", 0, 0}, {"aliases", "", "", 0, 0}, {"search_phrases", "", "", 0, 0}, {"content", "", "", 0, 0},
		},
	}
	if schemaVersion >= 2 {
		expected["projects"] = append(expected["projects"], schemaColumn{"deleted_at", "TEXT", "", 0, 0})
	}
	if versioned {
		expected["schema_migrations"] = []schemaColumn{
			{"version", "INTEGER", "", 0, 1}, {"name", "TEXT", "", 1, 0}, {"checksum", "TEXT", "", 1, 0}, {"applied_at", "TEXT", "", 1, 0},
		}
	}
	for table, want := range expected {
		rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
		if err != nil {
			return false, fmt.Errorf("reading %s column signature: %w", table, err)
		}
		var got []schemaColumn
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
				_ = rows.Close()
				return false, fmt.Errorf("scanning %s column signature: %w", table, err)
			}
			got = append(got, schemaColumn{name: name, columnType: columnType, defaultValue: defaultValue.String, notNull: notNull, primaryKey: primaryKey})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return false, fmt.Errorf("iterating %s column signature: %w", table, err)
		}
		if err := rows.Close(); err != nil {
			return false, fmt.Errorf("closing %s column signature: %w", table, err)
		}
		if len(got) != len(want) {
			return false, nil
		}
		for index, column := range want {
			if got[index] != column {
				return false, nil
			}
		}
	}
	return true, nil
}

func hasSchemaDefinitions(ctx context.Context, db *sql.DB, versioned bool, schemaVersion int) (bool, error) {
	expected := map[string]string{
		"projects":                     `CREATE TABLE projects (id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		"memories":                     `CREATE TABLE memories (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT, type TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, importance INTEGER NOT NULL DEFAULT 1, source TEXT NOT NULL DEFAULT '', access_count INTEGER NOT NULL DEFAULT 0, last_accessed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT)`,
		"memory_enrichments":           `CREATE TABLE memory_enrichments (memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE, summary TEXT NOT NULL DEFAULT '', short_summary TEXT NOT NULL DEFAULT '', keywords_json TEXT NOT NULL DEFAULT '[]', tags_json TEXT NOT NULL DEFAULT '[]', entities_json TEXT NOT NULL DEFAULT '[]', aliases_json TEXT NOT NULL DEFAULT '[]', search_phrases_json TEXT NOT NULL DEFAULT '[]', updated_at TEXT NOT NULL)`,
		"idx_memories_project_deleted": `CREATE INDEX idx_memories_project_deleted ON memories(project_id, deleted_at)`,
		"idx_memories_project_type":    `CREATE INDEX idx_memories_project_type ON memories(project_id, type)`,
		"memory_fts":                   `CREATE VIRTUAL TABLE memory_fts USING fts5(memory_id UNINDEXED, project_key UNINDEXED, type, title, summary, short_summary, keywords, tags, entities, aliases, search_phrases, content, tokenize='unicode61')`,
	}
	if schemaVersion >= 2 {
		expected["projects"] = `CREATE TABLE projects (id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT)`
		expected["idx_projects_deleted"] = `CREATE INDEX idx_projects_deleted ON projects(deleted_at, key)`
	}
	if versioned {
		expected["schema_migrations"] = `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`
	}
	for name, want := range expected {
		var got string
		if err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&got); err != nil {
			return false, fmt.Errorf("reading %s schema definition: %w", name, err)
		}
		if canonicalSchemaSQL(got) != canonicalSchemaSQL(want) {
			return false, nil
		}
	}
	return true, nil
}

func canonicalSchemaSQL(value string) string {
	value = strings.ToLower(value)
	value = strings.Join(strings.Fields(value), "")
	return strings.ReplaceAll(value, "ifnotexists", "")
}

func classifyLedger(ctx context.Context, db *sql.DB) (int, schemaState, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return 0, schemaUnknown, fmt.Errorf("reading migration ledger: %w", err)
	}
	defer rows.Close()
	type entry struct {
		version        int
		name, checksum string
	}
	var entries []entry
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.version, &item.name, &item.checksum); err != nil {
			return 0, schemaUnknown, fmt.Errorf("scanning migration ledger: %w", err)
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		return 0, schemaUnknown, err
	}
	if len(entries) > len(migrations) {
		return len(entries), schemaFuture, nil
	}
	if len(entries) == 0 {
		return 0, schemaUnknown, nil
	}
	for index, entry := range entries {
		migration := migrations[index]
		if entry.version != migration.version || entry.name != migration.name || entry.checksum != migrationChecksum(migration) {
			return 0, schemaUnknown, nil
		}
	}
	return len(entries), schemaCurrentVersioned, nil
}
