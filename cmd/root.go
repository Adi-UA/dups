// Package cmd implements the CLI flag parsing and execution logic.
package cmd

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Adi-UA/dups/internal/hasher"
	"github.com/Adi-UA/dups/internal/model"
	"github.com/Adi-UA/dups/internal/reporter"
	"github.com/Adi-UA/dups/internal/walker"
)

// Run parses flags and executes the duplicate scan pipeline.
// Returns 0 on success, 1 on error. Called from main().
func Run() int {
	// Go's flag package is simpler than argparse/cobra. You declare flags, call Parse(),
	// then read the values. Flags must come before positional args (no mixing).
	var (
		minSize    int64
		jsonMode   bool
		summary    bool
		dryRun     bool
		excludeRaw string
		workers    int
		noDelete   bool
		batch      bool
	)

	flag.Int64Var(&minSize, "min-size", 0, "Minimum file size in bytes")
	flag.BoolVar(&jsonMode, "json", false, "Output results as JSON")
	flag.BoolVar(&summary, "summary", false, "Show only summary stats")
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")
	flag.BoolVar(&noDelete, "no-delete", false, "Report only, no delete prompt")
	flag.BoolVar(&batch, "batch", false, "Single delete prompt for all groups (non-interactive)")
	flag.StringVar(&excludeRaw, "exclude", ".git", "Comma-separated directory names to skip")
	flag.IntVar(&workers, "workers", 0, "Number of hash workers (0 = auto: NumCPU/4)")
	flag.Parse()

	// flag.Arg(0) is the first positional argument after all flags.
	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	// strings.Split + TrimSpace = splitting a CSV string into clean tokens.
	excludes := strings.Split(excludeRaw, ",")
	for i := range excludes {
		excludes[i] = strings.TrimSpace(excludes[i])
	}

	// Stage 1: Walk and group by size.
	sizeGroups, totalFiles, totalBytes, err := walker.WalkFromInfo(root, walker.Options{
		MinSize:  minSize,
		Excludes: excludes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Stage 2: Hash and find duplicates.
	// Workers default to 0 which means runtime.NumCPU() inside the hasher.
	dupGroups := hasher.Deduplicate(sizeGroups, workers)

	// Sort by wasted space descending (biggest waste first).
	sort.Slice(dupGroups, func(i, j int) bool {
		return dupGroups[i].WastedBytes > dupGroups[j].WastedBytes
	})

	// Build the result struct.
	var duplicateFiles int
	var wastedBytes int64
	for _, g := range dupGroups {
		duplicateFiles += len(g.Files)
		wastedBytes += g.WastedBytes
	}

	result := model.ScanResult{
		TotalFiles:     totalFiles,
		TotalBytes:     totalBytes,
		Groups:         dupGroups,
		DuplicateFiles: duplicateFiles,
		WastedBytes:    wastedBytes,
	}

	// Stage 3: Report.
	if summary {
		fmt.Printf("Files: %d | Size: %s | Duplicates: %d | Wasted: %s\n",
			result.TotalFiles, formatBytes(result.TotalBytes),
			result.DuplicateFiles, formatBytes(result.WastedBytes))
		return 0
	}

	if jsonMode {
		rep := reporter.NewJSON()
		if err := rep.Report(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	// Text mode: default is interactive (per-group y/n/q prompts).
	// --batch = single prompt for all groups at once.
	// --no-delete = report only, no prompts.
	// --dry-run = show what would be deleted (works with both modes).
	interactive := !noDelete && !batch
	batchDelete := batch && !noDelete
	rep := reporter.NewText(interactive, batchDelete, dryRun)
	if err := rep.Report(result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
