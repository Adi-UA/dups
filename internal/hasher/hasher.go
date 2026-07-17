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

// Deduplicate takes size-matched file groups from the walker and returns
// confirmed duplicate groups (files with identical SHA-256 content hashes).
// numWorkers controls concurrency (0 = NumCPU).
func Deduplicate(sizeGroups [][]model.FileEntry, numWorkers int) []model.DuplicateGroup {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	type hashResult struct {
		entry model.FileEntry
		hash  string
	}

	var allResults []hashResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Channel of files to hash
	jobs := make(chan model.FileEntry, 256)

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				h, err := hashFile(entry.Path)
				if err != nil {
					continue // skip unreadable files
				}
				mu.Lock()
				allResults = append(allResults, hashResult{entry: entry, hash: h})
				mu.Unlock()
			}
		}()
	}

	// Feed all files from size-matched groups into the job channel
	for _, group := range sizeGroups {
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

// hashFile computes the full SHA-256 of a file, streaming in 64KB chunks.
func hashFile(path string) (string, error) {
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
