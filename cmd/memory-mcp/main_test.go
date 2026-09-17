package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"mcp-ai-memory/internal/app"
	"mcp-ai-memory/internal/storage"
)

func TestParseCommandPreservesDefaultServerAndSelectsManager(t *testing.T) {
	configuredPath := filepath.Join(t.TempDir(), "memory.db")
	overridePath := filepath.Join(t.TempDir(), "memory.db")
	mode, path, err := parseCommand(nil, configuredPath, &bytes.Buffer{})
	if err != nil || mode != "serve" || path != configuredPath {
		t.Fatalf("default command = (%q, %q, %v)", mode, path, err)
	}
	mode, path, err = parseCommand([]string{"manage", "--db", overridePath}, configuredPath, &bytes.Buffer{})
	if err != nil || mode != "manage" || path != overridePath {
		t.Fatalf("manage command = (%q, %q, %v)", mode, path, err)
	}
}

func TestRunRejectsUnknownCommandBeforeStorageAccess(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	dbPath := filepath.Join(parent, "memory.db")
	t.Setenv("MEMORY_DB_PATH", dbPath)
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatalf("storage parent stat = %v, want not exist", err)
	}
}

func TestNewManagementRequiresAbsolutePathAndExclusiveLock(t *testing.T) {
	if _, err := app.NewManagement(context.Background(), "relative.db", &bytes.Buffer{}, &bytes.Buffer{}, false); err == nil {
		t.Fatal("relative management path succeeded")
	}
	path := filepath.Join(t.TempDir(), "memory.db")
	runtimeLock, err := storage.AcquireRuntimeLock(path)
	if err != nil {
		t.Fatalf("acquire runtime lock: %v", err)
	}
	defer runtimeLock.Close()
	if _, err := app.NewManagement(context.Background(), path, &bytes.Buffer{}, &bytes.Buffer{}, false); err == nil {
		t.Fatal("management opened while runtime lock was active")
	}
}

func TestConcurrentMCPProcessesExposeToolsAndShareWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := storage.OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close initialized database: %v", err)
	}
	symlinkPath := filepath.Join(t.TempDir(), "symlink.db")
	hardlinkPath := filepath.Join(t.TempDir(), "hardlink.db")
	if err := os.Symlink(path, symlinkPath); err != nil {
		t.Fatalf("create database symlink: %v", err)
	}
	binary := buildMemoryMCPTestBinary(t)

	first := startMCPTestClient(t, binary, path, "first")
	second := startMCPTestClient(t, binary, symlinkPath, "second")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tools, err := second.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools from second process: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "project.upsert") || !slices.Contains(names, "memory.search") {
		t.Fatalf("second process tools = %v, want Project Memory tools", names)
	}
	for name, alias := range map[string]string{"canonical": path, "symlink": symlinkPath} {
		lock, err := storage.AcquireMaintenanceLock(alias)
		if err == nil {
			_ = lock.Close()
			t.Fatalf("maintenance lock through %s succeeded while MCP processes were active", name)
		}
	}
	if manager, err := app.NewManagement(context.Background(), path, &bytes.Buffer{}, &bytes.Buffer{}, false); err == nil {
		_ = manager.Close()
		t.Fatal("management opened while MCP processes were active")
	}

	upsert, err := first.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "project.upsert",
		Arguments: map[string]any{
			"key":  "shared-project",
			"name": "Shared Project",
		},
	}})
	if err != nil {
		t.Fatalf("upsert project through first process: %v", err)
	}
	if upsert.IsError {
		t.Fatalf("upsert project returned tool error: %#v", upsert.Content)
	}
	listed, err := second.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "project.list"}})
	if err != nil {
		t.Fatalf("list projects through second process: %v", err)
	}
	encoded, err := json.Marshal(listed.StructuredContent)
	if err != nil {
		t.Fatalf("encode project list: %v", err)
	}
	if !strings.Contains(string(encoded), "shared-project") {
		t.Fatalf("second process project list = %s, want shared project", encoded)
	}
	memoryUpsert, err := first.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "memory.upsert",
		Arguments: map[string]any{
			"project": "shared-project",
			"id":      "shared-memory",
			"type":    "implementation",
			"title":   "Shared memory",
			"content": "crossprocesssentinel is visible through the shared FTS index",
		},
	}})
	if err != nil {
		t.Fatalf("upsert memory through first process: %v", err)
	}
	if memoryUpsert.IsError {
		t.Fatalf("upsert memory returned tool error: %#v", memoryUpsert.Content)
	}
	gotMemory, err := second.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name:      "memory.get",
		Arguments: map[string]any{"project": "shared-project", "id": "shared-memory"},
	}})
	if err != nil {
		t.Fatalf("get memory through second process: %v", err)
	}
	gotMemoryJSON, err := json.Marshal(gotMemory.StructuredContent)
	if err != nil {
		t.Fatalf("encode memory get: %v", err)
	}
	if !strings.Contains(string(gotMemoryJSON), "crossprocesssentinel") {
		t.Fatalf("second process memory = %s, want authoritative content", gotMemoryJSON)
	}
	searched, err := second.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name:      "memory.search",
		Arguments: map[string]any{"project": "shared-project", "query": "crossprocesssentinel"},
	}})
	if err != nil {
		t.Fatalf("search memory through second process: %v", err)
	}
	searchedJSON, err := json.Marshal(searched.StructuredContent)
	if err != nil {
		t.Fatalf("encode memory search: %v", err)
	}
	if !strings.Contains(string(searchedJSON), "shared-memory") {
		t.Fatalf("second process search = %s, want shared memory", searchedJSON)
	}
	inspection, err := storage.OpenCurrentSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open database to inspect enrichment: %v", err)
	}
	var enrichmentCount int
	if err := inspection.QueryRow(`SELECT COUNT(*) FROM memory_enrichments WHERE memory_id = 'shared-memory'`).Scan(&enrichmentCount); err != nil {
		_ = inspection.Close()
		t.Fatalf("query enrichment row: %v", err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatalf("close enrichment inspection: %v", err)
	}
	if enrichmentCount != 1 {
		t.Fatalf("enrichment rows = %d, want 1", enrichmentCount)
	}
	if err := os.Link(path, hardlinkPath); err != nil {
		t.Fatalf("create database hardlink: %v", err)
	}
	hardlinkClient, err := client.NewStdioMCPClient(binary, append(os.Environ(),
		"MEMORY_DB_PATH="+hardlinkPath,
		"MEMORY_TRACE_MODE=off",
	))
	if err != nil {
		t.Fatalf("start hardlink MCP process: %v", err)
	}
	t.Cleanup(func() { _ = hardlinkClient.Close() })
	hardlinkCtx, hardlinkCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer hardlinkCancel()
	hardlinkRequest := mcp.InitializeRequest{}
	hardlinkRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	hardlinkRequest.Params.ClientInfo = mcp.Implementation{Name: "hardlink-test-client", Version: "1.0.0"}
	if _, err := hardlinkClient.Initialize(hardlinkCtx, hardlinkRequest); err == nil {
		t.Fatal("MCP process initialized through a hardlink alias")
	}
	if lock, err := storage.AcquireMaintenanceLock(hardlinkPath); err == nil {
		_ = lock.Close()
		t.Fatal("maintenance lock through hardlink succeeded while MCP processes were active")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first process: %v", err)
	}
	if err := second.Ping(ctx); err != nil {
		t.Fatalf("ping second process after first closed: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second process: %v", err)
	}
	if manager, err := app.NewManagement(context.Background(), hardlinkPath, &bytes.Buffer{}, &bytes.Buffer{}, false); err == nil {
		_ = manager.Close()
		t.Fatal("management opened a hardlinked database")
	} else if !errors.Is(err, storage.ErrMultipleHardLinks) {
		t.Fatalf("hardlink management error = %v, want ErrMultipleHardLinks", err)
	}
}

func TestMCPProcessRefusesSchemaInitializationWhileAnotherProcessHoldsRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	_ = startRuntimeLockHelper(t, path)
	binary := buildMemoryMCPTestBinary(t)
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "MEMORY_DB_PATH="+path, "MEMORY_TRACE_MODE=off")
	command.Stdin = bytes.NewReader(nil)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil {
		t.Fatal("MCP process initialized schema while another process held a runtime lock")
	}
	if !strings.Contains(stderr.String(), "stop all MCP runtimes") {
		t.Fatalf("startup stderr = %q, want actionable migration conflict", stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database stat error = %v, want not exist", err)
	}
}

func TestMCPProcessMigratesOnlyAfterOtherRuntimeProcessStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	createVersionOneTestDatabase(t, path)
	stopRuntime := startRuntimeLockHelper(t, path)
	binary := buildMemoryMCPTestBinary(t)

	blocked := exec.Command(binary)
	blocked.Env = append(os.Environ(), "MEMORY_DB_PATH="+path, "MEMORY_TRACE_MODE=off")
	blocked.Stdin = bytes.NewReader(nil)
	var blockedStderr bytes.Buffer
	blocked.Stderr = &blockedStderr
	if err := blocked.Run(); err == nil {
		t.Fatal("MCP process migrated schema while another process held a runtime lock")
	}
	if !strings.Contains(blockedStderr.String(), "stop all MCP runtimes") {
		t.Fatalf("blocked migration stderr = %q, want actionable conflict", blockedStderr.String())
	}
	if backups, err := filepath.Glob(path + ".backup-*.db"); err != nil {
		t.Fatalf("glob migration backups: %v", err)
	} else if len(backups) != 0 {
		t.Fatalf("blocked migration created backups: %v", backups)
	}
	readOnly, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path}).String()+"?mode=ro")
	if err != nil {
		t.Fatalf("open blocked v1 database: %v", err)
	}
	var version int
	if err := readOnly.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		_ = readOnly.Close()
		t.Fatalf("read blocked schema version: %v", err)
	}
	if err := readOnly.Close(); err != nil {
		t.Fatalf("close blocked v1 database: %v", err)
	}
	if version != 1 {
		t.Fatalf("blocked schema version = %d, want 1", version)
	}

	stopRuntime()
	migrating := exec.Command(binary)
	migrating.Env = append(os.Environ(), "MEMORY_DB_PATH="+path, "MEMORY_TRACE_MODE=off")
	migrating.Stdin = bytes.NewReader(nil)
	var migratingStderr bytes.Buffer
	migrating.Stderr = &migratingStderr
	if err := migrating.Run(); err != nil {
		t.Fatalf("migrate after runtime stopped: %v: %s", err, migratingStderr.String())
	}
	current, err := storage.OpenCurrentSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open migrated current store: %v", err)
	}
	if err := current.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		_ = current.Close()
		t.Fatalf("read migrated schema version: %v", err)
	}
	if err := current.Close(); err != nil {
		t.Fatalf("close migrated store: %v", err)
	}
	if version != 2 {
		t.Fatalf("migrated schema version = %d, want 2", version)
	}
	if backups, err := filepath.Glob(path + ".backup-*.db"); err != nil {
		t.Fatalf("glob successful migration backups: %v", err)
	} else if len(backups) != 1 {
		t.Fatalf("successful migration backups = %v, want one", backups)
	}
}

func TestConcurrentMCPProcessBootstrapLeavesCurrentStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	binary := buildMemoryMCPTestBinary(t)
	type process struct {
		command *exec.Cmd
		input   io.WriteCloser
		stderr  bytes.Buffer
	}
	processes := make([]*process, 2)
	for index := range processes {
		item := &process{}
		item.command = exec.Command(binary)
		item.command.Env = append(os.Environ(), "MEMORY_DB_PATH="+path, "MEMORY_TRACE_MODE=off")
		item.command.Stderr = &item.stderr
		input, err := item.command.StdinPipe()
		if err != nil {
			t.Fatalf("create bootstrap stdin %d: %v", index, err)
		}
		item.input = input
		if err := item.command.Start(); err != nil {
			t.Fatalf("start bootstrap process %d: %v", index, err)
		}
		processes[index] = item
	}
	for _, item := range processes {
		if err := item.input.Close(); err != nil {
			t.Fatalf("close bootstrap stdin: %v", err)
		}
	}
	successes := 0
	for index, item := range processes {
		err := item.command.Wait()
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(item.stderr.String(), "database operation lock conflict") {
			t.Fatalf("bootstrap process %d failed unexpectedly: %v: %s", index, err, item.stderr.String())
		}
	}
	if successes == 0 {
		t.Fatal("no MCP process initialized the empty store")
	}
	db, err := storage.OpenCurrentSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open store after process bootstrap: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close bootstrapped store: %v", err)
	}
}

func TestHelperProcessHoldsRuntimeLock(t *testing.T) {
	if os.Getenv("MEMORY_TEST_HOLD_RUNTIME_LOCK") != "1" {
		return
	}
	lock, err := storage.AcquireRuntimeLock(os.Getenv("MEMORY_TEST_DB_PATH"))
	if err != nil {
		t.Fatalf("acquire helper runtime lock: %v", err)
	}
	defer lock.Close()
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		t.Fatalf("signal helper readiness: %v", err)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func buildMemoryMCPTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "memory-mcp")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build memory-mcp: %v\n%s", err, output)
	}
	return binary
}

func startRuntimeLockHelper(t *testing.T, path string) func() {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestHelperProcessHoldsRuntimeLock$")
	command.Env = append(os.Environ(), "MEMORY_TEST_HOLD_RUNTIME_LOCK=1", "MEMORY_TEST_DB_PATH="+path)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create runtime helper stdin: %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create runtime helper stdout: %v", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start runtime helper: %v", err)
	}
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "ready" {
		_ = stdin.Close()
		_ = command.Wait()
		t.Fatalf("runtime helper readiness = %q, %v; stderr: %s", ready, err, stderr.String())
	}
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			_ = stdin.Close()
			if err := command.Wait(); err != nil {
				t.Errorf("runtime helper exit: %v; stderr: %s", err, stderr.String())
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func createVersionOneTestDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open v1 fixture: %v", err)
	}
	statements := []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE memories (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT, type TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, importance INTEGER NOT NULL DEFAULT 1, source TEXT NOT NULL DEFAULT '', access_count INTEGER NOT NULL DEFAULT 0, last_accessed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT)`,
		`CREATE TABLE memory_enrichments (memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE, summary TEXT NOT NULL DEFAULT '', short_summary TEXT NOT NULL DEFAULT '', keywords_json TEXT NOT NULL DEFAULT '[]', tags_json TEXT NOT NULL DEFAULT '[]', entities_json TEXT NOT NULL DEFAULT '[]', aliases_json TEXT NOT NULL DEFAULT '[]', search_phrases_json TEXT NOT NULL DEFAULT '[]', updated_at TEXT NOT NULL)`,
		`CREATE INDEX idx_memories_project_deleted ON memories(project_id, deleted_at)`,
		`CREATE INDEX idx_memories_project_type ON memories(project_id, type)`,
		`CREATE VIRTUAL TABLE memory_fts USING fts5(memory_id UNINDEXED, project_key UNINDEXED, type, title, summary, short_summary, keywords, tags, entities, aliases, search_phrases, content, tokenize='unicode61')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			t.Fatalf("create v1 fixture: %v", err)
		}
	}
	checksumInput := "mcp-ai-memory-migration-v1\x001\x00initial_schema\x00projects|memories|memory_enrichments|memory_fts|idx_memories_project_deleted|idx_memories_project_type"
	checksum := sha256.Sum256([]byte(checksumInput))
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (1, 'initial_schema', ?, 'now')`, hex.EncodeToString(checksum[:])); err != nil {
		_ = db.Close()
		t.Fatalf("seed v1 migration ledger: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close v1 fixture: %v", err)
	}
}

func startMCPTestClient(t *testing.T, binary, path, name string) *client.Client {
	t.Helper()
	instance, err := client.NewStdioMCPClient(binary, append(os.Environ(),
		"MEMORY_DB_PATH="+path,
		"MEMORY_TRACE_MODE=off",
	))
	if err != nil {
		t.Fatalf("start %s MCP process: %v", name, err)
	}
	t.Cleanup(func() { _ = instance.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{Name: name + "-test-client", Version: "1.0.0"}
	if _, err := instance.Initialize(ctx, request); err != nil {
		_ = instance.Close()
		t.Fatalf("initialize %s MCP process: %v", name, err)
	}
	return instance
}
