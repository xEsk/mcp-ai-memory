package manage

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

func TestRunnerBrowsesSummaryThenMemoryDetail(t *testing.T) {
	projects, memories := managementServices(t)
	ctx := context.Background()
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "demo", Name: "Demo project"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{
		Project: "demo", Type: memory.TypeArchitecture, Title: "Search architecture",
		Content: "The authoritative full memory content is visible on the detail screen.",
	}); err != nil {
		t.Fatalf("create memory: %v", err)
	}
	input := strings.NewReader("1\nm\n1\n\nb\nb\nq\n")
	var output bytes.Buffer
	if err := New(projects, memories, input, &output, false).Run(ctx); err != nil {
		t.Fatalf("run manager: %v", err)
	}
	text := output.String()
	for _, expected := range []string{"MEMORY MCP", "Demo project", "Search architecture", "authoritative full memory content", "Retrieval exposure"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("manager output missing %q:\n%s", expected, text)
		}
	}
	stats, err := projects.Stats(ctx, "demo")
	if err != nil {
		t.Fatalf("read stats after browsing: %v", err)
	}
	if stats.Retrievals != 0 || stats.NeverRetrieved != 1 {
		t.Fatalf("browsing changed retrieval stats: %#v", stats)
	}
}

func TestRunnerCancelsProjectDeletionOnMismatchedConfirmation(t *testing.T) {
	projects, memories := managementServices(t)
	ctx := context.Background()
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "kept", Name: "Kept"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	input := strings.NewReader("1\nd\nnot-kept\nb\nq\n")
	if err := New(projects, memories, input, &bytes.Buffer{}, false).Run(ctx); err != nil {
		t.Fatalf("run manager: %v", err)
	}
	active, err := projects.List(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(active) != 1 || active[0].Key != "kept" {
		t.Fatalf("active projects = %#v, want kept project", active)
	}
}

func TestRunnerCleanupRequiresExactConfirmation(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "wrong case", input: "c\nclean\nq\n"},
		{name: "whitespace", input: "c\n CLEAN \nq\n"},
		{name: "eof", input: "c\nCLEAN"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projects, memories := managementServices(t)
			ctx := context.Background()
			if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "recoverable", Name: "Recoverable"}); err != nil {
				t.Fatalf("create project: %v", err)
			}
			if err := projects.SoftDelete(ctx, "recoverable"); err != nil {
				t.Fatalf("delete project: %v", err)
			}
			if err := New(projects, memories, strings.NewReader(test.input), &bytes.Buffer{}, false).Run(ctx); err != nil {
				t.Fatalf("run manager: %v", err)
			}
			all, err := projects.ListAll(ctx)
			if err != nil {
				t.Fatalf("list all projects: %v", err)
			}
			if len(all) != 1 || all[0].DeletedAt == nil {
				t.Fatalf("projects after %s cleanup confirmation = %#v", test.name, all)
			}
		})
	}
}

func TestRunnerDestructiveConfirmationRejectsWhitespaceAndEOF(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "whitespace", input: "1\nd\n recoverable \nb\nq\n"},
		{name: "eof", input: "1\nd\nrecoverable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projects, memories := managementServices(t)
			ctx := context.Background()
			if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "recoverable", Name: "Recoverable"}); err != nil {
				t.Fatalf("create project: %v", err)
			}
			if err := New(projects, memories, strings.NewReader(test.input), &bytes.Buffer{}, false).Run(ctx); err != nil {
				t.Fatalf("run manager: %v", err)
			}
			active, err := projects.List(ctx)
			if err != nil {
				t.Fatalf("list projects: %v", err)
			}
			if len(active) != 1 {
				t.Fatalf("project was deleted with %s confirmation", test.name)
			}
		})
	}
}

func TestRunnerSanitizesStoredTerminalControls(t *testing.T) {
	projects, memories := managementServices(t)
	ctx := context.Background()
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "safe", Name: "Unsafe\x1b]0;forged\a name"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{Project: "safe", Title: "Title\x1b[31m", Content: "Content\x1b]52;c;payload\a"}); err != nil {
		t.Fatalf("create memory: %v", err)
	}
	var output bytes.Buffer
	if err := New(projects, memories, strings.NewReader("1\nm\n1\n\nq\n"), &output, false).Run(ctx); err != nil {
		t.Fatalf("run manager: %v", err)
	}
	if strings.ContainsAny(output.String(), "\x1b\a") {
		t.Fatalf("manager emitted stored terminal controls: %q", output.String())
	}
}

func managementServices(t *testing.T) (*project.Service, *memory.Service) {
	t.Helper()
	db, err := storage.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := platform.NewClock()
	projects := project.NewService(project.NewRepository(db), clock)
	memories := memory.NewService(memory.NewRepository(db), projects, enrich.NewLocalHeuristicEnricher(), clock)
	return projects, memories
}
