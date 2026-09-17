// Package manage provides the offline terminal interface for the memory store.
package manage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/project"
)

const pageSize = 12

var errQuit = errors.New("quit management session")

// Runner owns one line-oriented terminal management session.
type Runner struct {
	projects *project.Service
	memories *memory.Service
	in       *bufio.Reader
	out      io.Writer
	styled   bool
}

// New creates a terminal management runner.
func New(projects *project.Service, memories *memory.Service, input io.Reader, output io.Writer, styled bool) *Runner {
	return &Runner{projects: projects, memories: memories, in: bufio.NewReader(input), out: output, styled: styled}
}

// Run displays the project menu until the user quits or input closes.
func (r *Runner) Run(ctx context.Context) error {
	for {
		projects, err := r.projects.ListAll(ctx)
		if err != nil {
			return err
		}
		r.clear()
		r.header("MEMORY MCP", "Local memory manager")
		if len(projects) == 0 {
			r.muted("No projects yet.")
		} else {
			for index, item := range projects {
				stats, err := r.projects.Stats(ctx, item.Key)
				if err != nil {
					return err
				}
				state := r.green("active")
				if item.DeletedAt != nil {
					state = r.red("deleted")
				}
				fmt.Fprintf(r.out, "  %s  %-28s %4d memories   %s\n", r.accent(fmt.Sprintf("%2d", index+1)), safeInline(item.Key), stats.ActiveMemories, state)
			}
		}
		fmt.Fprintln(r.out)
		r.muted("[number] open   [c] clean junk   [q] quit")
		choice, err := r.prompt("Choose")
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch strings.ToLower(choice) {
		case "q":
			return nil
		case "c":
			if err := r.cleanup(ctx); err != nil {
				return err
			}
		default:
			index, err := strconv.Atoi(choice)
			if err == nil && index >= 1 && index <= len(projects) {
				if err := r.project(ctx, projects[index-1]); errors.Is(err, errQuit) {
					return nil
				} else if err != nil {
					return err
				}
			}
		}
	}
}

func (r *Runner) project(ctx context.Context, item project.Project) error {
	for {
		stats, err := r.projects.Stats(ctx, item.Key)
		if err != nil {
			return err
		}
		r.clear()
		state := r.green("ACTIVE")
		if item.DeletedAt != nil {
			state = r.red("DELETED")
		}
		r.header(safeInline(item.Name), safeInline(item.Key)+"  "+state)
		fmt.Fprintf(r.out, "  Active memories     %s\n", r.value(stats.ActiveMemories))
		fmt.Fprintf(r.out, "  Deleted memories    %s\n", r.value(stats.DeletedMemories))
		fmt.Fprintf(r.out, "  Total memories      %s\n", r.value(stats.TotalMemories))
		fmt.Fprintf(r.out, "  Retrieval exposure  %s\n", r.value(stats.Retrievals))
		fmt.Fprintf(r.out, "  Never retrieved     %s\n", r.value(stats.NeverRetrieved))
		if stats.LatestMemoryAt != nil {
			fmt.Fprintf(r.out, "  Latest update       %s\n", r.accent(stats.LatestMemoryAt.Local().Format("2006-01-02 15:04")))
		}
		if len(stats.MemoriesByType) > 0 {
			fmt.Fprintf(r.out, "  Types               %s\n", r.typeCounts(stats.MemoriesByType))
		}
		fmt.Fprintln(r.out)
		if item.DeletedAt != nil {
			r.muted("[r] restore   [b] back   [q] quit")
		} else {
			r.muted("[m] memories   [d] delete project   [b] back   [q] quit")
		}
		choice, err := r.prompt("Choose")
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch strings.ToLower(choice) {
		case "q":
			return errQuit
		case "b":
			return nil
		case "m":
			if item.DeletedAt == nil {
				if err := r.memoriesList(ctx, item.Key); err != nil {
					return err
				}
			}
		case "d":
			if item.DeletedAt == nil {
				confirmed, err := r.promptExact("Type " + safeInline(item.Key) + " to delete")
				if errors.Is(err, io.EOF) {
					return nil
				}
				if err != nil {
					return err
				}
				if confirmed == item.Key {
					if err := r.projects.SoftDelete(ctx, item.Key); err != nil {
						return err
					}
					return nil
				}
			}
		case "r":
			if item.DeletedAt != nil {
				if err := r.projects.Restore(ctx, item.Key); err != nil {
					return err
				}
				return nil
			}
		}
	}
}

func (r *Runner) memoriesList(ctx context.Context, projectKey string) error {
	offset := 0
	for {
		items, err := r.memories.ListForManagement(ctx, memory.ListInput{Project: projectKey, Limit: pageSize, Offset: offset})
		if err != nil {
			return err
		}
		r.clear()
		r.header("MEMORIES", safeInline(projectKey)+fmt.Sprintf("  page %d", offset/pageSize+1))
		if len(items) == 0 {
			r.muted("No memories on this page.")
		}
		for index, item := range items {
			fmt.Fprintf(r.out, "  %s  %s  %s\n", r.accent(fmt.Sprintf("%2d", index+1)), r.badge(safeInline(item.Type)), safeInline(item.Title))
			fmt.Fprintf(r.out, "      %s\n", r.mutedText(summary(item)))
		}
		fmt.Fprintln(r.out)
		r.muted("[number] detail   [n] next   [p] previous   [b] back   [q] quit")
		choice, err := r.prompt("Choose")
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch strings.ToLower(choice) {
		case "q":
			return errQuit
		case "b":
			return nil
		case "n":
			if len(items) == pageSize {
				offset += pageSize
			}
		case "p":
			if offset >= pageSize {
				offset -= pageSize
			}
		default:
			index, err := strconv.Atoi(choice)
			if err == nil && index >= 1 && index <= len(items) {
				item, err := r.memories.GetForManagement(ctx, projectKey, items[index-1].ID)
				if err != nil {
					return err
				}
				if err := r.memoryDetail(item); err != nil {
					return err
				}
			}
		}
	}
}

func (r *Runner) memoryDetail(item *memory.Memory) error {
	r.clear()
	r.header(safeInline(item.Title), safeInline(item.Type)+"  importance "+strconv.Itoa(item.Importance))
	if item.Source != "" {
		fmt.Fprintf(r.out, "  Source: %s\n\n", safeInline(item.Source))
	}
	for _, line := range wrap(safeTerminalText(item.Content), 84) {
		fmt.Fprintf(r.out, "  %s\n", line)
	}
	fmt.Fprintln(r.out)
	choice, err := r.prompt("Press Enter to go back, or q to quit")
	if errors.Is(err, io.EOF) {
		return nil
	}
	if strings.EqualFold(choice, "q") {
		return errQuit
	}
	return err
}

func (r *Runner) cleanup(ctx context.Context) error {
	preview, err := r.projects.PreviewCleanup(ctx)
	if err != nil {
		return err
	}
	r.clear()
	r.header("CLEAN JUNK", "Permanent database cleanup")
	fmt.Fprintf(r.out, "  Deleted projects          %s\n", r.value(preview.DeletedProjects))
	fmt.Fprintf(r.out, "  Memories in those projects %s\n", r.value(preview.DeletedProjectMemories))
	fmt.Fprintf(r.out, "  Deleted memories          %s\n", r.value(preview.DeletedMemories))
	fmt.Fprintf(r.out, "  Stale search rows         %s\n", r.value(preview.StaleFTS))
	fmt.Fprintf(r.out, "  Orphaned enrichments      %s\n\n", r.value(preview.OrphanedEnrichments))
	if preview == (project.CleanupStats{}) {
		r.greenLine("Everything is clean.")
		_, err := r.prompt("Press Enter to go back")
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	r.redLine("This cannot be undone from the manager.")
	confirmation, err := r.promptExact("Type CLEAN to continue")
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	if confirmation != "CLEAN" {
		return nil
	}
	cleaned, err := r.projects.Cleanup(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "\n  %s Removed %d projects and %d memories.\n", r.green("✓"), cleaned.DeletedProjects, cleaned.DeletedProjectMemories+cleaned.DeletedMemories)
	_, err = r.prompt("Press Enter to go back")
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (r *Runner) prompt(label string) (string, error) {
	value, err := r.promptExact(label)
	return strings.TrimSpace(value), err
}

func (r *Runner) promptExact(label string) (string, error) {
	fmt.Fprintf(r.out, "\n%s ", r.accent(label+" ›"))
	value, err := r.in.ReadString('\n')
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	return value, err
}

func (r *Runner) clear() {
	if r.styled {
		fmt.Fprint(r.out, "\033[2J\033[H")
	}
}

func (r *Runner) header(title, subtitle string) {
	fmt.Fprintf(r.out, "%s\n%s\n\n", r.accent("╭─ "+title+" ─────────────────────────────────────────╮"), r.mutedText("   "+subtitle))
}

func (r *Runner) typeCounts(values map[string]int) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", safeInline(key), values[key]))
	}
	return strings.Join(parts, " · ")
}

func (r *Runner) value(value int) string     { return r.accent(strconv.Itoa(value)) }
func (r *Runner) badge(value string) string  { return r.mutedText("[" + value + "]") }
func (r *Runner) muted(value string)         { fmt.Fprintln(r.out, r.mutedText("  "+value)) }
func (r *Runner) greenLine(value string)     { fmt.Fprintln(r.out, "  "+r.green(value)) }
func (r *Runner) redLine(value string)       { fmt.Fprintln(r.out, "  "+r.red(value)) }
func (r *Runner) accent(value string) string { return r.style("38;5;81", value) }
func (r *Runner) green(value string) string  { return r.style("38;5;78", value) }
func (r *Runner) red(value string) string    { return r.style("38;5;203", value) }
func (r *Runner) mutedText(value string) string {
	return r.style("38;5;244", value)
}
func (r *Runner) style(code, value string) string {
	if !r.styled {
		return value
	}
	return "\033[" + code + "m" + value + "\033[0m"
}

func summary(item memory.Memory) string {
	value := strings.TrimSpace(item.ShortSummary)
	if value == "" {
		value = strings.TrimSpace(item.Content)
	}
	value = safeInline(value)
	return truncate(value, 90)
}

func safeInline(value string) string {
	return strings.Join(strings.Fields(safeTerminalText(value)), " ")
}

func safeTerminalText(value string) string {
	return strings.Map(func(char rune) rune {
		if char == '\n' {
			return char
		}
		if char == '\t' {
			return ' '
		}
		if unicode.IsControl(char) {
			return -1
		}
		return char
	}, value)
}

func truncate(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func wrap(value string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) > width {
				lines = append(lines, line)
				line = word
			} else {
				line += " " + word
			}
		}
		lines = append(lines, line)
	}
	return lines
}
