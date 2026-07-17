// Package model defines shared types used across the dups pipeline.
package model

import "time"

// FileEntry represents a single file on disk.
type FileEntry struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// DuplicateGroup holds a set of files that have identical content.
// WastedBytes = Size * (copies - 1), i.e., space you'd reclaim by keeping one.
type DuplicateGroup struct {
	Hash        string
	Size        int64
	Files       []FileEntry
	WastedBytes int64
}

// ScanResult is the output of a complete deduplication scan.
type ScanResult struct {
	TotalFiles     int
	TotalBytes     int64
	Groups         []DuplicateGroup
	DuplicateFiles int
	WastedBytes    int64
}
