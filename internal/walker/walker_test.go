package walker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkFindsGroupsBySize(t *testing.T) {
	dir := t.TempDir()

	// Create 2 files with same size (10 bytes)
	write(t, filepath.Join(dir, "a.txt"), "1234567890")
	write(t, filepath.Join(dir, "b.txt"), "abcdefghij")

	// Create 1 file with unique size (5 bytes)
	write(t, filepath.Join(dir, "c.txt"), "hello")

	groups, totalFiles, _, err := Walk(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if totalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", totalFiles)
	}

	// Only 1 group (the two 10-byte files), the 5-byte file is unique
	if len(groups) != 1 {
		t.Fatalf("expected 1 size group, got %d", len(groups))
	}

	if len(groups[0]) != 2 {
		t.Errorf("expected 2 files in group, got %d", len(groups[0]))
	}
}

func TestWalkRespectsMinSize(t *testing.T) {
	dir := t.TempDir()

	write(t, filepath.Join(dir, "small.txt"), "hi")
	write(t, filepath.Join(dir, "small2.txt"), "yo")
	write(t, filepath.Join(dir, "big.txt"), "1234567890")
	write(t, filepath.Join(dir, "big2.txt"), "abcdefghij")

	groups, _, _, err := Walk(dir, Options{MinSize: 5})
	if err != nil {
		t.Fatal(err)
	}

	// Only the big files (10 bytes) should form a group
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestWalkExcludesDirectories(t *testing.T) {
	dir := t.TempDir()
	excluded := filepath.Join(dir, "node_modules")
	os.MkdirAll(excluded, 0o755)

	write(t, filepath.Join(excluded, "a.txt"), "1234567890")
	write(t, filepath.Join(excluded, "b.txt"), "1234567890")
	write(t, filepath.Join(dir, "c.txt"), "hello")

	groups, totalFiles, _, err := Walk(dir, Options{Excludes: []string{"node_modules"}})
	if err != nil {
		t.Fatal(err)
	}

	if totalFiles != 1 {
		t.Errorf("expected 1 file (excluded dir skipped), got %d", totalFiles)
	}

	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
