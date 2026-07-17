// Package hasher computes SHA-256 hashes of file groups using a concurrent
// worker pool and returns confirmed duplicate groups.
package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/Adi-UA/dups/internal/model"
)

const partialHashSize = 4096 // 4 KB for partial hash

// Deduplicate takes size-matched file groups from the walker and returns
// confirmed duplicate groups (files with identical SHA-256 content hashes).
// numWorkers controls concurrency (0 = NumCPU).
//
// For files larger than 1 MB, a two-pass strategy is used:
// 1. Hash only the first 4 KB (partial hash). Discard unique partial hashes.
// 2. Full hash only the files that share a partial hash.
// This avoids reading large files that differ early in their content.
func Deduplicate(sizeGroups [][]model.FileEntry, numWorkers int) []model.DuplicateGroup {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	// Separate small files (full hash directly) from large files (two-pass)
	var smallGroups [][]model.FileEntry
	var largeGroups [][]model.FileEntry

	for _, group := range sizeGroups {
		if len(group) == 0 {
			continue
		}
		if group[0].Size > 1<<20 { // > 1 MB
			largeGroups = append(largeGroups, group)
		} else {
			smallGroups = append(smallGroups, group)
		}
	}

	// Full hash small files directly
	results := hashFiles(smallGroups, numWorkers, false)

	// Two-pass for large files
	if len(largeGroups) > 0 {
		// Pass 1: partial hash
		partialResults := hashFiles(largeGroups, numWorkers, true)
		// partialResults are groups with matching partial hashes — now full hash them
		var candidates [][]model.FileEntry
		for _, g := range partialResults {
			candidates = append(candidates, g.Files)
		}
		// Pass 2: full hash on partial-hash matches
		fullResults := hashFiles(candidates, numWorkers, false)
		results = append(results, fullResults...)
	}

	return results
}

// hashFiles hashes files using a worker pool. If partial is true, only the
// first 4 KB is hashed; otherwise the full file is hashed.
func hashFiles(groups [][]model.FileEntry, numWorkers int, partial bool) []model.DuplicateGroup {
	type hashResult struct {
		entry model.FileEntry
		hash  string
	}

	var allResults []hashResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	jobs := make(chan model.FileEntry, 256)

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				var h string
				var err error
				if partial {
					h, err = hashFilePartial(entry.Path)
				} else {
					h, err = hashFileFull(entry.Path)
				}
				if err != nil {
					continue
				}
				mu.Lock()
				allResults = append(allResults, hashResult{entry: entry, hash: h})
				mu.Unlock()
			}
		}()
	}

	for _, group := range groups {
		for _, entry := range group {
			jobs <- entry
		}
	}
	close(jobs)
	wg.Wait()

	// Group by hash
	hashMap := make(map[string][]model.FileEntry)
	for _, r := range allResults {
		hashMap[r.hash] = append(hashMap[r.hash], r.entry)
	}

	// Build duplicate groups (2+ files with same hash)
	var duplicates []model.DuplicateGroup
	for hash, files := range hashMap {
		if len(files) >= 2 {
			size := files[0].Size
			duplicates = append(duplicates, model.DuplicateGroup{
				Hash:        hash,
				Size:        size,
				Files:       files,
				WastedBytes: size * int64(len(files)-1),
			})
		}
	}

	return duplicates
}

// hashFileFull computes the full SHA-256 of a file, streaming in 64 KB chunks.
func hashFileFull(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFilePartial computes SHA-256 of only the first 4 KB of a file.
func hashFilePartial(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.CopyN(h, f, partialHashSize); err != nil && err != io.EOF {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
