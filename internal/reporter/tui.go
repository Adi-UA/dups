package reporter

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/Adi-UA/dups/internal/model"
)

// TextReporter writes a human-readable colored summary to stdout.
type TextReporter struct {
	Writer io.Writer
}

// NewText creates a TextReporter that writes to stdout.
func NewText() *TextReporter {
	return &TextReporter{Writer: os.Stdout}
}

// Report outputs the scan result as a human-readable summary.
func (r *TextReporter) Report(result model.ScanResult) error {
	if len(result.Groups) == 0 {
		fmt.Fprintf(r.Writer, "Scanned %d files (%s). No duplicates found.\n",
			result.TotalFiles, formatBytes(result.TotalBytes))
		return nil
	}

	// Sort groups by wasted bytes descending
	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].WastedBytes > result.Groups[j].WastedBytes
	})

	fmt.Fprintf(r.Writer, "Scanned %d files (%s)\n",
		result.TotalFiles, formatBytes(result.TotalBytes))
	fmt.Fprintf(r.Writer, "Found %s of duplicates across %d files (%d groups)\n\n",
		formatBytes(result.WastedBytes), result.DuplicateFiles, len(result.Groups))

	for i, g := range result.Groups {
		fmt.Fprintf(r.Writer, "\033[1mGROUP %d\033[0m — %s wasted (%d copies)\n",
			i+1, formatBytes(g.WastedBytes), len(g.Files))
		for j, f := range g.Files {
			marker := "\033[32m✓\033[0m" // green check (keep)
			if j > 0 {
				marker = "\033[31m✗\033[0m" // red x (duplicate)
			}
			fmt.Fprintf(r.Writer, "  %s %s  %s  %s\n",
				marker, f.Path, formatBytes(f.Size), f.ModTime.Format("2006-01-02"))
		}
		fmt.Fprintln(r.Writer)
	}

	return nil
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
