// Package walker recursively traverses a directory tree and groups files by size.
// Files with unique sizes are discarded (they cannot have duplicates).
package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Adi-UA/dups/internal/model"
)

// Options configures the directory walk.
type Options struct {
	MinSize  int64    // Minimum file size in bytes (0 = no minimum)
	Excludes []string // Directory names to skip (e.g., ".git", "node_modules")
}

// Walk traverses root and returns groups of files that share the same size.
// Only groups with 2+ files are returned (potential duplicates).
func Walk(root string, opts Options) ([][]model.FileEntry, int, int64, error) {
	sizeMap := make(map[int64][]model.FileEntry)
	totalFiles := 0
	var totalBytes int64

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		// Skip excluded directories
		if d.IsDir() {
			name := d.Name()
			for _, exc := range opts.Excludes {
				if strings.EqualFold(name, exc) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip non-regular files (symlinks, devices, etc.)
		if !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		size := info.Size()
		if size < opts.MinSize {
			return nil
		}

		totalFiles++
		totalBytes += size

		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		sizeMap[size] = append(sizeMap[size], model.FileEntry{
			Path:    absPath,
			Size:    size,
			ModTime: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, 0, 0, err
	}

	// Keep only groups with 2+ files (potential duplicates)
	var groups [][]model.FileEntry
	for _, files := range sizeMap {
		if len(files) >= 2 {
			groups = append(groups, files)
		}
	}

	return groups, totalFiles, totalBytes, nil
}

// WalkFromInfo is a convenience that wraps Walk and validates the root path.
func WalkFromInfo(root string, opts Options) ([][]model.FileEntry, int, int64, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, 0, 0, err
	}
	if !info.IsDir() {
		return nil, 0, 0, fs.ErrInvalid
	}
	return Walk(root, opts)
}
