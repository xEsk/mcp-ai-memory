package storage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type backupManifest struct {
	Path      string
	Size      int64
	SHA256    string
	CreatedAt time.Time
}

// CreateBackup creates and verifies a unique same-directory SQLite snapshot.
func CreateBackup(ctx context.Context, db *sql.DB, sourcePath string) (string, error) {
	if !filepath.IsAbs(sourcePath) {
		return "", fmt.Errorf("database path must be absolute")
	}
	backupPath := filepath.Join(filepath.Dir(sourcePath), fmt.Sprintf("%s.backup-%s.db", filepath.Base(sourcePath), time.Now().UTC().Format("20060102T150405.000000000Z")))
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		return "", fmt.Errorf("backup path already exists: %s", backupPath)
	}
	complete := false
	manifestCreated := false
	defer func() {
		if complete {
			return
		}
		_ = os.Remove(backupPath)
		if manifestCreated {
			_ = os.Remove(backupPath + ".manifest")
		}
	}()
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
	manifestCreated = true
	if _, err := verifyBackupSnapshot(ctx, backupPath); err != nil {
		return "", err
	}
	complete = true
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
	sum, err := fileSHA256(backupPath)
	if err != nil {
		return fmt.Errorf("reading snapshot for checksum: %w", err)
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("stating snapshot: %w", err)
	}
	manifest := fmt.Sprintf("path=%s\nsize=%d\nsha256=%s\ncreated_at=%s\n", filepath.Base(backupPath), info.Size(), sum, time.Now().UTC().Format(time.RFC3339Nano))
	manifestPath := backupPath + ".manifest"
	file, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating snapshot manifest: %w", err)
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(manifestPath)
		}
	}()
	if _, err := io.WriteString(file, manifest); err != nil {
		return fmt.Errorf("writing snapshot manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("syncing snapshot manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing snapshot manifest: %w", err)
	}
	complete = true
	return nil
}

func verifyBackupSnapshot(ctx context.Context, backupPath string) (backupManifest, error) {
	manifest, err := readBackupManifest(backupPath + ".manifest")
	if err != nil {
		return backupManifest{}, err
	}
	if manifest.Path != filepath.Base(backupPath) {
		return backupManifest{}, fmt.Errorf("backup manifest path = %q, want %q", manifest.Path, filepath.Base(backupPath))
	}
	if err := verifySnapshotFile(ctx, backupPath); err != nil {
		return backupManifest{}, err
	}
	if err := verifySnapshotAgainstManifest(backupPath, manifest); err != nil {
		return backupManifest{}, err
	}
	return manifest, nil
}

func readBackupManifest(path string) (backupManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return backupManifest{}, fmt.Errorf("opening backup manifest: %w", err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if !found || key == "" || value == "" {
			return backupManifest{}, fmt.Errorf("invalid backup manifest line %q", scanner.Text())
		}
		switch key {
		case "path", "size", "sha256", "created_at":
		default:
			return backupManifest{}, fmt.Errorf("unknown backup manifest field %q", key)
		}
		if _, exists := values[key]; exists {
			return backupManifest{}, fmt.Errorf("duplicate backup manifest field %q", key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return backupManifest{}, fmt.Errorf("reading backup manifest: %w", err)
	}
	for _, key := range []string{"path", "size", "sha256", "created_at"} {
		if values[key] == "" {
			return backupManifest{}, fmt.Errorf("backup manifest field %q is required", key)
		}
	}
	size, err := strconv.ParseInt(values["size"], 10, 64)
	if err != nil || size < 0 {
		return backupManifest{}, fmt.Errorf("invalid backup manifest size %q", values["size"])
	}
	checksumBytes, err := hex.DecodeString(values["sha256"])
	if err != nil || len(checksumBytes) != sha256.Size {
		return backupManifest{}, fmt.Errorf("invalid backup manifest sha256 %q", values["sha256"])
	}
	createdAt, err := time.Parse(time.RFC3339Nano, values["created_at"])
	if err != nil {
		return backupManifest{}, fmt.Errorf("invalid backup manifest created_at %q", values["created_at"])
	}
	return backupManifest{Path: values["path"], Size: size, SHA256: values["sha256"], CreatedAt: createdAt}, nil
}

func verifySnapshotAgainstManifest(path string, manifest backupManifest) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stating snapshot for manifest verification: %w", err)
	}
	if info.Size() != manifest.Size {
		return fmt.Errorf("backup manifest checksum verification failed: size = %d, want %d", info.Size(), manifest.Size)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("reading snapshot for manifest verification: %w", err)
	}
	if !strings.EqualFold(sum, manifest.SHA256) {
		return fmt.Errorf("backup manifest checksum = %s, want %s", sum, manifest.SHA256)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
