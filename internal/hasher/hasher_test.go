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

func TestDeduplicateLargeFilesUsePartialHash(t *testing.T) {
	dir := t.TempDir()

	// Create two 2MB files with identical content
	bigContent := make([]byte, 2<<20)
	for i := range bigContent {
		bigContent[i] = byte(i % 256)
	}
	os.WriteFile(filepath.Join(dir, "big1.bin"), bigContent, 0o644)
	os.WriteFile(filepath.Join(dir, "big2.bin"), bigContent, 0o644)

	// Create a 2MB file with different content (same size, different first bytes)
	diffContent := make([]byte, 2<<20)
	for i := range diffContent {
		diffContent[i] = byte((i + 1) % 256)
	}
	os.WriteFile(filepath.Join(dir, "big3.bin"), diffContent, 0o644)

	entries := []model.FileEntry{
		{Path: filepath.Join(dir, "big1.bin"), Size: 2 << 20},
		{Path: filepath.Join(dir, "big2.bin"), Size: 2 << 20},
		{Path: filepath.Join(dir, "big3.bin"), Size: 2 << 20},
	}

	groups := Deduplicate([][]model.FileEntry{entries}, 2)

	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Errorf("expected 2 files in group, got %d", len(groups[0].Files))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
