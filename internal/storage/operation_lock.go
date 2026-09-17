package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// ErrOperationLockConflict reports an incompatible active database operation.
var ErrOperationLockConflict = errors.New("database operation lock conflict")

// ErrMultipleHardLinks reports a database inode that has no single safe runtime pathname.
var ErrMultipleHardLinks = errors.New("database has multiple hard links")

// OperationLock coordinates normal MCP runtimes and exclusive database operations.
type OperationLock struct {
	file         *os.File
	identityFile *os.File
	path         string
	mode         int
	mu           sync.Mutex
	closed       bool
}

// AcquireRuntimeLock reserves a database path for a running MCP server.
func AcquireRuntimeLock(path string) (*OperationLock, error) {
	return acquireOperationLock(path, "runtime", syscall.LOCK_SH)
}

// AcquireMaintenanceLock reserves a database path for an offline maintenance operation.
func AcquireMaintenanceLock(path string) (*OperationLock, error) {
	return acquireOperationLock(path, "maintenance", syscall.LOCK_EX)
}

// AcquireSchemaLock reserves a database path for startup schema mutation.
func AcquireSchemaLock(path string) (*OperationLock, error) {
	return acquireOperationLock(path, "schema mutation", syscall.LOCK_EX)
}

func acquireOperationLock(path, operation string, mode int) (*OperationLock, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating operation lock directory: %w", err)
	}
	canonicalPath, err := canonicalDatabasePath(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path for %s lock: %w", operation, err)
	}
	lockPath := canonicalPath + ".operation.lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("acquiring %s operation lock: %w", operation, err)
	}
	if err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("%w: database is active or another maintenance operation is in progress", ErrOperationLockConflict)
		}
		return nil, fmt.Errorf("locking %s operation: %w", operation, err)
	}
	var identityFile *os.File
	if info, err := os.Stat(canonicalPath); err == nil {
		if operation == "runtime" && hasMultipleHardLinks(info) {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			_ = file.Close()
			return nil, fmt.Errorf("%w; MCP runtimes require one authoritative database path", ErrMultipleHardLinks)
		}
		identityFile, err = lockDatabaseIdentity(canonicalPath, mode)
		if err != nil {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			_ = file.Close()
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("stating database for %s identity lock: %w", operation, err)
	}
	return &OperationLock{file: file, identityFile: identityFile, path: lockPath, mode: mode}, nil
}

func hasMultipleHardLinks(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink > 1
}

func validateDatabaseHasSingleLink(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stating database link count: %w", err)
	}
	if hasMultipleHardLinks(info) {
		return fmt.Errorf("%w; database operations require one authoritative database path", ErrMultipleHardLinks)
	}
	return nil
}

// BindDatabaseIdentity locks a database created after the operation reservation was acquired.
func (l *OperationLock) BindDatabaseIdentity(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return fmt.Errorf("binding database identity after operation lock close")
	}
	if l.identityFile != nil {
		return nil
	}
	canonicalPath, err := canonicalDatabasePath(path)
	if err != nil {
		return fmt.Errorf("resolving created database identity: %w", err)
	}
	if canonicalPath+".operation.lock" != l.path {
		return fmt.Errorf("created database identity does not match operation lock")
	}
	l.identityFile, err = lockDatabaseIdentity(canonicalPath, l.mode)
	return err
}

func lockDatabaseIdentity(path string, mode int) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stating database identity: %w", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("reading database device and inode")
	}
	identityDir := filepath.Join("/tmp", fmt.Sprintf("mcp-ai-memory-%d-locks", os.Getuid()))
	if err := os.MkdirAll(identityDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating database identity lock directory: %w", err)
	}
	identityPath := filepath.Join(identityDir, fmt.Sprintf("%d-%d.operation.lock", uint64(stat.Dev), uint64(stat.Ino)))
	file, err := os.OpenFile(identityPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening database identity lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("%w: database is active or another maintenance operation is in progress", ErrOperationLockConflict)
		}
		return nil, fmt.Errorf("locking database identity: %w", err)
	}
	return file, nil
}

func canonicalDatabasePath(path string) (string, error) {
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(path)
		if readErr != nil {
			return "", readErr
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

// Close releases the operation reservation.
func (l *OperationLock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var identityErr error
	if l.identityFile != nil {
		if err := syscall.Flock(int(l.identityFile.Fd()), syscall.LOCK_UN); err != nil {
			identityErr = err
		}
		if err := l.identityFile.Close(); identityErr == nil {
			identityErr = err
		}
	}
	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if identityErr != nil {
		return identityErr
	}
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
