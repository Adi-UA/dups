package hasher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Adi-UA/dups/internal/model"
)

func TestDeduplicateFindsExactDuplicates(t *testing.T) {
	dir := t.TempDir()

	// Two files with identical content
	write(t, filepath.Join(dir, "a.txt"), "duplicate content here")
	write(t, filepath.Join(dir, "b.txt"), "duplicate content here")

	// One file with different content but same size (22 bytes)
	write(t, filepath.Join(dir, "c.txt"), "different content!!!!")

	entries := []model.FileEntry{
		{Path: filepath.Join(dir, "a.txt"), Size: 22},
		{Path: filepath.Join(dir, "b.txt"), Size: 22},
		{Path: filepath.Join(dir, "c.txt"), Size: 22},
	}

	groups := Deduplicate([][]model.FileEntry{entries}, 2)

	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}

	if len(groups[0].Files) != 2 {
		t.Errorf("expected 2 files in group, got %d", len(groups[0].Files))
	}

	if groups[0].WastedBytes != 22 {
		t.Errorf("expected 22 wasted bytes, got %d", groups[0].WastedBytes)
	}
}

func TestDeduplicateNoDuplicates(t *testing.T) {
	dir := t.TempDir()

	write(t, filepath.Join(dir, "a.txt"), "aaaaaaaaaa")
	write(t, filepath.Join(dir, "b.txt"), "bbbbbbbbbb")

	entries := []model.FileEntry{
		{Path: filepath.Join(dir, "a.txt"), Size: 10},
		{Path: filepath.Join(dir, "b.txt"), Size: 10},
	}

	groups := Deduplicate([][]model.FileEntry{entries}, 2)

	if len(groups) != 0 {
		t.Errorf("expected 0 duplicate groups, got %d", len(groups))
	}
}

func TestDeduplicateMultipleGroups(t *testing.T) {
	dir := t.TempDir()

	// Group 1: same content
	write(t, filepath.Join(dir, "x1.txt"), "xxxxx")
	write(t, filepath.Join(dir, "x2.txt"), "xxxxx")

	// Group 2: same content (different from group 1 but same size)
	write(t, filepath.Join(dir, "y1.txt"), "yyyyy")
	write(t, filepath.Join(dir, "y2.txt"), "yyyyy")

	entries := []model.FileEntry{
		{Path: filepath.Join(dir, "x1.txt"), Size: 5},
		{Path: filepath.Join(dir, "x2.txt"), Size: 5},
		{Path: filepath.Join(dir, "y1.txt"), Size: 5},
		{Path: filepath.Join(dir, "y2.txt"), Size: 5},
	}

	groups := Deduplicate([][]model.FileEntry{entries}, 4)

	if len(groups) != 2 {
		t.Fatalf("expected 2 duplicate groups, got %d", len(groups))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
