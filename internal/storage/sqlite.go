package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// OpenSQLite opens the local database and applies the initial schema if missing.
func OpenSQLite(ctx context.Context, path string) (*sql.DB, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}
	lock, err := acquireMigrationLock(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Close() }()
	_, statErr := os.Stat(path)
	existedBeforeOpen := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("stating database path: %w", statErr)
	}

	state, err := classifyPath(ctx, path, existedBeforeOpen)
	if err != nil {
		return nil, err
	}
	if state != schemaEmpty && state != schemaCurrentUnversioned && state != schemaCurrentVersioned {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchema, state)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging sqlite database: %w", err)
	}
	if existedBeforeOpen && state != schemaCurrentVersioned {
		if _, err := CreateBackup(ctx, db, path); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("backing up existing database: %w", err)
		}
	}
	if err := applyClassifiedMigrations(ctx, db, state); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("applying migrations: %w", err)
	}
	return db, nil
}

func classifyPath(ctx context.Context, path string, exists bool) (schemaState, error) {
	if !exists {
		return schemaEmpty, nil
	}
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return schemaUnknown, fmt.Errorf("opening database read-only for classification: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return schemaUnknown, fmt.Errorf("pinging database read-only for classification: %w", err)
	}
	return classifyDatabase(ctx, db)
}

type migrationLock struct {
	file *os.File
	path string
}

func acquireMigrationLock(path string) (*migrationLock, error) {
	lockPath := path + ".migrate.lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("acquiring migration lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("database migration is already in progress")
		}
		return nil, fmt.Errorf("locking database migration: %w", err)
	}
	return &migrationLock{file: file, path: lockPath}, nil
}

func (l *migrationLock) Close() error {
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
