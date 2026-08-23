package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateBackup creates and verifies a unique same-directory SQLite snapshot.
func CreateBackup(ctx context.Context, db *sql.DB, sourcePath string) (string, error) {
	if !filepath.IsAbs(sourcePath) {
		return "", fmt.Errorf("database path must be absolute")
	}
	backupPath := filepath.Join(filepath.Dir(sourcePath), fmt.Sprintf("%s.backup-%s.db", filepath.Base(sourcePath), time.Now().UTC().Format("20060102T150405.000000000Z")))
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		return "", fmt.Errorf("backup path already exists: %s", backupPath)
	}
	quotedPath := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+quotedPath+"'"); err != nil {
		return "", fmt.Errorf("creating sqlite snapshot: %w", err)
	}
	if err := os.Chmod(backupPath, 0o600); err != nil {
		return "", fmt.Errorf("restricting snapshot permissions: %w", err)
	}
	if err := verifySnapshot(ctx, backupPath); err != nil {
		return "", err
	}
	if err := writeBackupManifest(backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

func verifySnapshot(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("opening snapshot read-only: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("checking snapshot integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("snapshot integrity check = %q", integrity)
	}
	return nil
}

func writeBackupManifest(backupPath string) error {
	contents, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("reading snapshot for checksum: %w", err)
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("stating snapshot: %w", err)
	}
	sum := sha256.Sum256(contents)
	manifest := fmt.Sprintf("path=%s\nsize=%d\nsha256=%s\ncreated_at=%s\n", filepath.Base(backupPath), info.Size(), hex.EncodeToString(sum[:]), time.Now().UTC().Format(time.RFC3339Nano))
	if err := os.WriteFile(backupPath+".manifest", []byte(manifest), 0o600); err != nil {
		return fmt.Errorf("writing snapshot manifest: %w", err)
	}
	return nil
}
