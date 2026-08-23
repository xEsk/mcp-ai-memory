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
	lock, err := storage.AcquireMaintenanceLock(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquiring offline maintenance lock: %v\n", err)
		os.Exit(1)
	}
	defer lock.Close()
	db, err := storage.OpenSQLite(ctx, *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	switch *operation {
	case "backup":
		backup, err := storage.CreateBackup(ctx, db, *dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating backup: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, backup)
	case "rebuild-fts":
		if err := storage.RebuildFTS(ctx, db); err != nil {
			fmt.Fprintf(os.Stderr, "rebuilding FTS: %v\n", err)
			os.Exit(1)
		}
	}
}
