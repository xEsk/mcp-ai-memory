package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var verifySnapshotFile = verifySnapshot

var renameMaintenanceFile = os.Rename

// InspectSchema classifies a database without opening it for writes.
func InspectSchema(ctx context.Context, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("database path must be absolute")
	}
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stating database path: %w", err)
	}
	state, err := classifyPath(ctx, path, err == nil)
	if err != nil {
		return "", err
	}
	return string(state), nil
}

// BackupDatabase creates a verified snapshot without applying runtime migrations.
func BackupDatabase(ctx context.Context, path string) (string, error) {
	if err := validateExistingDatabasePath(path); err != nil {
		return "", err
	}
	lock, err := AcquireMaintenanceLock(path)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	db, err := openMaintenanceSQLite(ctx, path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	return CreateBackup(ctx, db, path)
}

// RebuildDatabaseFTS rebuilds derived search state without applying runtime migrations.
func RebuildDatabaseFTS(ctx context.Context, path string) error {
	if err := validateExistingDatabasePath(path); err != nil {
		return err
	}
	lock, err := AcquireMaintenanceLock(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	db, err := openMaintenanceSQLite(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()
	return RebuildFTS(ctx, db)
}

func openMaintenanceSQLite(ctx context.Context, path string) (*sql.DB, error) {
	if err := validateExistingDatabasePath(path); err != nil {
		return nil, err
	}
	state, err := classifyPath(ctx, path, true)
	if err != nil {
		return nil, err
	}
	if state != schemaCurrentUnversioned && state != schemaCurrentVersioned {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchema, state)
	}
	db, err := sql.Open("sqlite", path+"?mode=rw&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database for maintenance: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging sqlite database for maintenance: %w", err)
	}
	return db, nil
}

func validateExistingDatabasePath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("database path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database does not exist: %s", path)
		}
		return fmt.Errorf("stating database path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("database path is not a regular file: %s", path)
	}
	if hasMultipleHardLinks(info) {
		return fmt.Errorf("%w; maintenance requires one authoritative database path", ErrMultipleHardLinks)
	}
	return nil
}

// RestoreBackup installs a verified snapshot while preserving the original as a quarantine file.
func RestoreBackup(ctx context.Context, path, backupPath string) (quarantinePath string, returnErr error) {
	if !filepath.IsAbs(path) || !filepath.IsAbs(backupPath) {
		return "", fmt.Errorf("database and backup paths must be absolute")
	}
	manifest, err := verifyBackupSnapshot(ctx, backupPath)
	if err != nil {
		return "", fmt.Errorf("verifying backup snapshot: %w", err)
	}
	operationLock, err := AcquireMaintenanceLock(path)
	if err != nil {
		return "", err
	}
	defer operationLock.Close()
	if err := validateDatabaseHasSingleLink(path); err != nil {
		return "", err
	}
	lock, err := acquireMigrationLock(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	temporaryPath := path + ".restore-tmp"
	if err := copySnapshot(backupPath, temporaryPath); err != nil {
		return "", err
	}
	quarantinePath = path + ".quarantine-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	type movedArtifact struct {
		original   string
		quarantine string
	}
	moved := make([]movedArtifact, 0, 3)
	installed := false
	defer func() {
		if returnErr == nil {
			return
		}
		var rollbackErrors []error
		if installed {
			for _, installedPath := range []string{path, path + "-wal", path + "-shm"} {
				if err := os.Remove(installedPath); err != nil && !os.IsNotExist(err) {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("removing failed restore artifact %s: %w", installedPath, err))
				}
			}
		}
		for index := len(moved) - 1; index >= 0; index-- {
			artifact := moved[index]
			if err := renameMaintenanceFile(artifact.quarantine, artifact.original); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restoring quarantined artifact %s: %w", artifact.quarantine, err))
			}
		}
		if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("removing restore temporary file %s: %w", temporaryPath, err))
		}
		if len(rollbackErrors) > 0 {
			returnErr = errors.Join(returnErr, errors.Join(rollbackErrors...))
		}
	}()
	if _, err := os.Stat(path); err == nil {
		if err := renameMaintenanceFile(path, quarantinePath); err != nil {
			return "", fmt.Errorf("quarantining existing database: %w", err)
		}
		moved = append(moved, movedArtifact{original: path, quarantine: quarantinePath})
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stating existing database before quarantine: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := path + suffix
		if _, err := os.Stat(sidecarPath); err == nil {
			quarantineSidecarPath := quarantinePath + suffix
			if err := renameMaintenanceFile(sidecarPath, quarantineSidecarPath); err != nil {
				return "", fmt.Errorf("quarantining database sidecar %s: %w", suffix, err)
			}
			moved = append(moved, movedArtifact{original: sidecarPath, quarantine: quarantineSidecarPath})
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stating database sidecar %s: %w", suffix, err)
		}
	}
	if err := renameMaintenanceFile(temporaryPath, path); err != nil {
		return "", fmt.Errorf("installing backup snapshot: %w", err)
	}
	installed = true
	if err := verifySnapshotFile(ctx, path); err != nil {
		return "", fmt.Errorf("validating restored database: %w", err)
	}
	if err := verifySnapshotAgainstManifest(path, manifest); err != nil {
		return "", fmt.Errorf("validating restored database manifest: %w", err)
	}
	return quarantinePath, nil
}

func copySnapshot(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening backup snapshot: %w", err)
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating restore temporary file: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return fmt.Errorf("copying backup snapshot: %w", err)
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return fmt.Errorf("syncing restore temporary file: %w", err)
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(destinationPath)
		return fmt.Errorf("closing restore temporary file: %w", err)
	}
	return nil
}
