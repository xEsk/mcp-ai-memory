package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFindUnsafeTestDatabasePaths(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"safe_test.go": fmt.Sprintf(`package fixture
import "testing"
func TestSafe(t *testing.T) { _ = filepath.Join(t.TempDir(), %q + %q) }
`, "memory.", "db"),
		"unsafe_test.go": fmt.Sprintf(`package fixture
import "testing"
func TestUnsafe(t *testing.T) { _ = filepath.Join("data", %q + %q); _ = t.TempDir() }
`, "memory.", "db"),
		"wrong_receiver_test.go": fmt.Sprintf(`package fixture
func TestWrongReceiver(t *testing.T) { _ = filepath.Join("data", %q + %q); _ = x.TempDir() }
`, "memory.", "db"),
		"package_path_test.go": fmt.Sprintf(`package fixture
var path = filepath.Join("data", %q + %q)
`, "memory.", "db"),
		"late_chdir_test.go": fmt.Sprintf(`package fixture
func TestLateChdir(t *testing.T) { _ = filepath.Join("data", %q + %q); t.Chdir(t.TempDir()) }
`, "memory.", "db"),
		"late_assignment_test.go": fmt.Sprintf(`package fixture
func TestLateAssignment(t *testing.T) { root := "data"; _ = filepath.Join(root, %q + %q); root = t.TempDir() }
`, "memory.", "db"),
		"absolute_after_chdir_test.go": fmt.Sprintf(`package fixture
func TestAbsoluteAfterChdir(t *testing.T) { t.Chdir(t.TempDir()); _ = filepath.Join("/tmp", %q + %q) }
`, "memory.", "db"),
		"indirect_filename_test.go": fmt.Sprintf(`package fixture
func TestIndirectFilename(t *testing.T) { name := %q + %q; _ = filepath.Join("data", name) }
`, "memory.", "db"),
		"production_identifier_test.go": `package fixture
func TestProductionIdentifier(t *testing.T) { _, _ = storage.OpenSQLite(ctx, storage.DatabasePath) }
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	unsafePaths, err := findUnsafeTestDatabasePaths(root)
	if err != nil {
		t.Fatalf("find unsafe paths: %v", err)
	}
	if len(unsafePaths) != 8 {
		t.Fatalf("unsafe paths = %v, want eight", unsafePaths)
	}
}
