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
func Run() int {
	var (
		minSize    int64
		jsonMode   bool
		summary    bool
		excludeRaw string
		workers    int
	)

	flag.Int64Var(&minSize, "min-size", 0, "Minimum file size in bytes (e.g., 1048576 for 1MB)")
	flag.BoolVar(&jsonMode, "json", false, "Output results as JSON")
	flag.BoolVar(&summary, "summary", false, "Show only summary stats")
	flag.StringVar(&excludeRaw, "exclude", ".git", "Comma-separated directory names to skip")
	flag.IntVar(&workers, "workers", 0, "Number of hash workers (0 = NumCPU)")
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	excludes := strings.Split(excludeRaw, ",")
	for i := range excludes {
		excludes[i] = strings.TrimSpace(excludes[i])
	}

	// Stage 1: Walk and group by size
	sizeGroups, totalFiles, totalBytes, err := walker.WalkFromInfo(root, walker.Options{
		MinSize:  minSize,
		Excludes: excludes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Stage 2: Hash and find duplicates
	dupGroups := hasher.Deduplicate(sizeGroups, workers)

	// Sort by wasted space descending
	sort.Slice(dupGroups, func(i, j int) bool {
		return dupGroups[i].WastedBytes > dupGroups[j].WastedBytes
	})

	// Build result
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

	// Stage 3: Report
	var rep reporter.Reporter
	if jsonMode {
		rep = reporter.NewJSON()
	} else {
		rep = reporter.NewText()
	}

	if summary {
		fmt.Printf("Files: %d | Size: %s | Duplicates: %d | Wasted: %s\n",
			result.TotalFiles, formatBytesCmd(result.TotalBytes),
			result.DuplicateFiles, formatBytesCmd(result.WastedBytes))
		return 0
	}

	if err := rep.Report(result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func formatBytesCmd(b int64) string {
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
