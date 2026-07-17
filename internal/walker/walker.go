// Package walker recursively traverses a directory tree and groups files by size.
// Files with unique sizes are discarded early since they cannot have duplicates.
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
	// map[fileSize] -> list of files with that size.
	// In Go, maps are reference types — no need to pass by pointer.
	sizeMap := make(map[int64][]model.FileEntry)
	totalFiles := 0
	var totalBytes int64

	// filepath.WalkDir is Go's stdlib recursive directory traversal.
	// The callback fires for every file/dir. Return filepath.SkipDir to skip a subtree.
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Return nil (not the error) to skip unreadable entries without aborting the walk.
			return nil
		}

		// Skip excluded directories.
		if d.IsDir() {
			name := d.Name()
			for _, exc := range opts.Excludes {
				// strings.EqualFold = case-insensitive comparison (like Java's equalsIgnoreCase)
				if strings.EqualFold(name, exc) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// d.Type().IsRegular() filters out symlinks, devices, pipes, etc.
		if !d.Type().IsRegular() {
			return nil
		}

		// d.Info() does the actual stat syscall to get size/modtime.
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

		// append() grows the slice if needed (like ArrayList.add in Java).
		// If the key doesn't exist yet, sizeMap[size] returns a nil slice,
		// and append on a nil slice works fine (creates a new one).
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

	// Filter: keep only groups with 2+ files (same size = maybe duplicates).
	// range over a map gives (key, value) pairs in random order.
	var groups [][]model.FileEntry
	for _, files := range sizeMap {
		if len(files) >= 2 {
			groups = append(groups, files)
		}
	}

	// Go supports multiple return values (no tuples or wrapper objects needed).
	return groups, totalFiles, totalBytes, nil
}

// WalkFromInfo validates that root is a directory, then calls Walk.
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
