package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mcp-ai-memory/internal/storage"
	"mcp-ai-memory/internal/trace"
)

func TestLoadConfigTraceDefaultsOffEvenWithMonitorURL(t *testing.T) {
	t.Setenv("MEMORY_TRACE_MODE", "")
	t.Setenv("MEMORY_TRACE_MONITOR_URL", "http://127.0.0.1:9999/events")

	cfg := LoadConfig()

	if cfg.TraceMode != TraceModeOff {
		t.Fatalf("trace mode = %q, want %q", cfg.TraceMode, TraceModeOff)
	}
}

func TestBuildTraceSinkMonitorURLAloneDoesNotEnableTracing(t *testing.T) {
	sink, err := buildTraceSink(Config{
		TraceMode:       TraceModeOff,
		TraceMonitorURL: "http://127.0.0.1:7379/events",
	})
	if err != nil {
		t.Fatalf("build trace sink: %v", err)
	}
	if _, ok := sink.(trace.NoopSink); !ok {
		t.Fatalf("sink = %T, want trace.NoopSink", sink)
	}
}

func TestNewRejectsUnknownTraceModeAtStartup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	application, err := New(context.Background(), Config{
		DBPath:          dbPath,
		TraceMode:       "verbose",
		TraceMonitorURL: "http://127.0.0.1:7379/events",
	})

	if err == nil {
		t.Fatal("application startup succeeded for unknown trace mode")
	}
	if !strings.Contains(err.Error(), "invalid trace mode") {
		t.Fatalf("startup error = %q, want invalid trace mode", err)
	}
	if application != nil {
		t.Fatalf("application = %#v, want nil", application)
	}
	if _, statErr := os.Stat(dbPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database stat error = %v, want not exist", statErr)
	}
}

func TestNewRejectsMissingDBPathBeforeCreatingFiles(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	application, err := New(context.Background(), Config{TraceMode: TraceModeOff})

	if err == nil {
		t.Fatal("application startup succeeded without a database path")
	}
	if !strings.Contains(err.Error(), "MEMORY_DB_PATH is required") {
		t.Fatalf("startup error = %q, want required database path error", err)
	}
	if application != nil {
		t.Fatalf("application = %#v, want nil", application)
	}
	if _, statErr := os.Stat(filepath.Join(workingDirectory, "data")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory stat error = %v, want not exist", statErr)
	}
}

func TestNewRejectsRelativeDBPathBeforeCreatingFiles(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	application, err := New(context.Background(), Config{
		DBPath:    filepath.Join("data", "memory.db"),
		TraceMode: TraceModeOff,
	})

	if err == nil {
		t.Fatal("application startup succeeded with a relative database path")
	}
	if !strings.Contains(err.Error(), "MEMORY_DB_PATH must be absolute") {
		t.Fatalf("startup error = %q, want absolute database path error", err)
	}
	if application != nil {
		t.Fatalf("application = %#v, want nil", application)
	}
	if _, statErr := os.Stat(filepath.Join(workingDirectory, "data")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory stat error = %v, want not exist", statErr)
	}
}

func TestNewUsesSameAbsoluteDBPathFromDifferentWorkingDirectories(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "store", "memory.db")
	firstDirectory := filepath.Join(root, "first")
	secondDirectory := filepath.Join(root, "second")
	for _, directory := range []string{firstDirectory, secondDirectory} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatalf("create working directory: %v", err)
		}
	}
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDirectory) })
	if err := os.Chdir(firstDirectory); err != nil {
		t.Fatalf("change first working directory: %v", err)
	}
	first, err := New(context.Background(), Config{DBPath: dbPath, TraceMode: TraceModeOff})
	if err != nil {
		t.Fatalf("open first application: %v", err)
	}
	if _, err := first.DB.Exec(`INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('continuity', 'continuity', 'Continuity', 'now', 'now')`); err != nil {
		t.Fatalf("create continuity project: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first application: %v", err)
	}
	if err := os.Chdir(secondDirectory); err != nil {
		t.Fatalf("change second working directory: %v", err)
	}
	second, err := New(context.Background(), Config{DBPath: dbPath, TraceMode: TraceModeOff})
	if err != nil {
		t.Fatalf("open second application: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	var count int
	if err := second.DB.QueryRow(`SELECT COUNT(*) FROM projects WHERE key = 'continuity'`).Scan(&count); err != nil {
		t.Fatalf("read continuity project: %v", err)
	}
	if count != 1 {
		t.Fatalf("continuity project count = %d, want 1", count)
	}
}

func TestNewPreventsOfflineMaintenanceWhileServerIsRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	application, err := New(context.Background(), Config{DBPath: path, TraceMode: TraceModeOff})
	if err != nil {
		t.Fatalf("start application: %v", err)
	}
	t.Cleanup(func() { _ = application.Close() })

	lock, err := storage.AcquireMaintenanceLock(path)
	if err == nil {
		_ = lock.Close()
		t.Fatal("maintenance lock succeeded while application was running")
	}
}
