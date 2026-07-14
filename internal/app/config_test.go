package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
