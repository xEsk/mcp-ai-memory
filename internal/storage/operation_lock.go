package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// OperationLock excludes normal MCP runtime and offline maintenance operations.
type OperationLock struct {
	file *os.File
	path string
}

// AcquireRuntimeLock reserves a database path for a running MCP server.
func AcquireRuntimeLock(path string) (*OperationLock, error) {
	return acquireOperationLock(path, "runtime")
}

// AcquireMaintenanceLock reserves a database path for an offline maintenance operation.
func AcquireMaintenanceLock(path string) (*OperationLock, error) {
	return acquireOperationLock(path, "maintenance")
}

func acquireOperationLock(path, operation string) (*OperationLock, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating operation lock directory: %w", err)
	}
	lockPath := path + ".operation.lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("acquiring %s operation lock: %w", operation, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("database is active or another maintenance operation is in progress")
		}
		return nil, fmt.Errorf("locking %s operation: %w", operation, err)
	}
	return &OperationLock{file: file, path: lockPath}, nil
}

// Close releases the exclusive operation reservation.
func (l *OperationLock) Close() error {
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
