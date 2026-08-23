package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mcp-ai-memory/internal/storage"
)

func main() {
	operation := flag.String("operation", "", "offline operation: inspect, backup, restore, or rebuild-fts")
	dbPath := flag.String("db", "", "absolute database path")
	backupPath := flag.String("backup", "", "absolute snapshot path for restore")
	flag.Parse()
	if *dbPath == "" || !filepath.IsAbs(*dbPath) {
		fmt.Fprintln(os.Stderr, "--db must be an absolute database path")
		os.Exit(2)
	}
	if *operation != "inspect" && *operation != "backup" && *operation != "restore" && *operation != "rebuild-fts" {
		fmt.Fprintln(os.Stderr, "--operation must be inspect, backup, restore, or rebuild-fts")
		os.Exit(2)
	}

	ctx := context.Background()
	if *operation == "inspect" {
		state, err := storage.InspectSchema(ctx, *dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "inspecting database: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, state)
		return
	}
	if *operation == "restore" {
		quarantine, err := storage.RestoreBackup(ctx, *dbPath, *backupPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "restoring database: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, quarantine)
		return
	}
	switch *operation {
	case "backup":
		backup, err := storage.BackupDatabase(ctx, *dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating backup: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, backup)
	case "rebuild-fts":
		if err := storage.RebuildDatabaseFTS(ctx, *dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "rebuilding FTS: %v\n", err)
			os.Exit(1)
		}
	}
}
