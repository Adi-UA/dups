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

// partialHashSize is how many bytes we read for the first-pass "partial hash."
// Files >1MB get this optimization: if their first 4KB differ, skip the full hash.
const partialHashSize = 4096

// Deduplicate takes size-matched file groups from the walker and returns
// confirmed duplicate groups (files with identical SHA-256 content hashes).
//
// numWorkers controls concurrency. 0 means runtime.NumCPU(), which auto-detects
// the machine's core count (e.g., 8 on an M1 Mac, 4 on the OCI ARM VM).
func Deduplicate(sizeGroups [][]model.FileEntry, numWorkers int) []model.DuplicateGroup {
	if numWorkers <= 0 {
		// Use 1/4 of available CPUs to avoid saturating the machine.
		// Minimum 1 worker even on single-core systems.
		numWorkers = max(1, runtime.NumCPU()/4)
	}

	// Separate small files (hash directly) from large files (two-pass).
	var smallGroups [][]model.FileEntry
	var largeGroups [][]model.FileEntry

	for _, group := range sizeGroups {
		if len(group) == 0 {
			continue
		}
		if group[0].Size > 1<<20 { // 1<<20 = 1 MB (bit shift, like Java)
			largeGroups = append(largeGroups, group)
		} else {
			smallGroups = append(smallGroups, group)
		}
	}

	// Small files: full hash directly (cheap to read entirely).
	results := hashFiles(smallGroups, numWorkers, false)

	// Large files: two-pass strategy to avoid reading gigabytes unnecessarily.
	if len(largeGroups) > 0 {
		// Pass 1: hash only first 4KB. If partial hashes differ, files differ.
		partialResults := hashFiles(largeGroups, numWorkers, true)
		// partialResults = groups where even the first 4KB matched. Now full-hash them.
		var candidates [][]model.FileEntry
		for _, g := range partialResults {
			candidates = append(candidates, g.Files)
		}
		// Pass 2: full SHA-256 on the remaining candidates.
		fullResults := hashFiles(candidates, numWorkers, false)
		results = append(results, fullResults...)
	}

	return results
}

// hashFiles is the core concurrent hashing engine. It spawns a worker pool,
// feeds files through a channel, and groups results by hash.
func hashFiles(groups [][]model.FileEntry, numWorkers int, partial bool) []model.DuplicateGroup {
	// hashResult pairs a file with its computed hash. This is a local struct
	// (Go allows defining types inside functions, like a private inner class).
	type hashResult struct {
		entry model.FileEntry
		hash  string
	}

	var allResults []hashResult
	var mu sync.Mutex    // protects allResults from concurrent appends
	var wg sync.WaitGroup // tracks how many workers are still running

	// make(chan T, bufferSize) creates a buffered channel.
	// A channel is a thread-safe FIFO queue built into the language.
	// Sending blocks when full, receiving blocks when empty.
	jobs := make(chan model.FileEntry, 256)

	// Spawn numWorkers goroutines. Each one loops pulling work from `jobs`.
	// `for range N` is Go 1.22+ syntax for "repeat N times."
	for range numWorkers {
		wg.Add(1) // increment "workers running" counter
		// `go func() { ... }()` launches a goroutine (lightweight thread, ~2KB stack).
		// The function runs concurrently. Execution continues immediately below.
		go func() {
			// `defer` schedules wg.Done() to run when this function exits,
			// regardless of how it exits (return, panic, etc.). Like try/finally.
			defer wg.Done()
			// `for entry := range jobs` blocks waiting for items on the channel.
			// When an item arrives, it's atomically dequeued and assigned to `entry`.
			// When the channel is closed AND empty, the loop exits.
			// This is the idiomatic Go worker pattern — no explicit dequeue call.
			for entry := range jobs {
				var h string
				var err error
				if partial {
					h, err = hashFilePartial(entry.Path)
				} else {
					h, err = hashFileFull(entry.Path)
				}
				if err != nil {
					continue // skip unreadable files, grab next job
				}
				// Mutex protects the shared slice. Only one goroutine appends at a time.
				mu.Lock()
				allResults = append(allResults, hashResult{entry: entry, hash: h})
				mu.Unlock()
			}
		}()
	}

	// Feed all files into the channel. Workers pick them up concurrently.
	// `jobs <- entry` enqueues. If the buffer (256) is full, this blocks
	// until a worker dequeues one (backpressure, prevents unbounded memory use).
	for _, group := range groups {
		for _, entry := range group {
			jobs <- entry
		}
	}
	// close() signals "no more items coming." Workers' `range` loops will exit
	// after draining any remaining items.
	close(jobs)
	// Block until all workers finish (counter reaches 0).
	wg.Wait()

	// Group results by hash. Same pattern as the walker's sizeMap.
	hashMap := make(map[string][]model.FileEntry)
	for _, r := range allResults {
		hashMap[r.hash] = append(hashMap[r.hash], r.entry)
	}

	// Keep only groups with 2+ files (confirmed duplicates).
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

// hashFileFull computes the full SHA-256 of a file, streaming in 64KB chunks.
// io.Copy handles the chunked reading internally — the file is never fully loaded into memory.
func hashFileFull(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	// defer f.Close() ensures the file handle is released even if we return early.
	defer f.Close()

	h := sha256.New()
	// io.Copy reads from f in 32KB chunks (default) and writes into the hasher.
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	// h.Sum(nil) finalizes the hash. hex.EncodeToString makes it a readable string.
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFilePartial computes SHA-256 of only the first 4KB of a file.
// io.CopyN reads exactly N bytes (or fewer at EOF).
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
