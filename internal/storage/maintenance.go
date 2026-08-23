package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var verifySnapshotFile = verifySnapshot

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

// RestoreBackup installs a verified snapshot while preserving the original as a quarantine file.
func RestoreBackup(ctx context.Context, path, backupPath string) (string, error) {
	if !filepath.IsAbs(path) || !filepath.IsAbs(backupPath) {
		return "", fmt.Errorf("database and backup paths must be absolute")
	}
	if err := verifySnapshotFile(ctx, backupPath); err != nil {
		return "", fmt.Errorf("verifying backup snapshot: %w", err)
	}
	operationLock, err := AcquireMaintenanceLock(path)
	if err != nil {
		return "", err
	}
	defer operationLock.Close()
	lock, err := acquireMigrationLock(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	temporaryPath := path + ".restore-tmp"
	if err := copySnapshot(backupPath, temporaryPath); err != nil {
		return "", err
	}
	quarantinePath := path + ".quarantine-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	sourceQuarantined := false
	installed := false
	defer func() {
		if installed || !sourceQuarantined {
			return
		}
		_ = os.Remove(path)
		for _, suffix := range []string{"-wal", "-shm"} {
			_ = os.Remove(path + suffix)
		}
		_ = os.Rename(quarantinePath, path)
		for _, suffix := range []string{"-wal", "-shm"} {
			_ = os.Rename(quarantinePath+suffix, path+suffix)
		}
		_ = os.Remove(temporaryPath)
	}()
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, quarantinePath); err != nil {
			return "", fmt.Errorf("quarantining existing database: %w", err)
		}
		sourceQuarantined = true
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := path + suffix
		if _, err := os.Stat(sidecarPath); err == nil {
			if err := os.Rename(sidecarPath, quarantinePath+suffix); err != nil {
				return "", fmt.Errorf("quarantining database sidecar %s: %w", suffix, err)
			}
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("installing backup snapshot: %w", err)
	}
	if err := verifySnapshotFile(ctx, path); err != nil {
		return "", fmt.Errorf("validating restored database: %w", err)
	}
	installed = true
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
		return fmt.Errorf("copying backup snapshot: %w", err)
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		return fmt.Errorf("syncing restore temporary file: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("closing restore temporary file: %w", err)
	}
	return nil
}
