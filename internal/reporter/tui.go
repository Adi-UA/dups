package reporter

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Adi-UA/dups/internal/model"
)

// TextReporter writes a human-readable colored summary to stdout.
type TextReporter struct {
	Writer      io.Writer
	Reader      io.Reader
	DryRun      bool
	Interactive bool
}

// NewText creates a TextReporter that writes to stdout and reads from stdin.
func NewText(interactive bool, dryRun bool) *TextReporter {
	return &TextReporter{
		Writer:      os.Stdout,
		Reader:      os.Stdin,
		Interactive: interactive,
		DryRun:      dryRun,
	}
}

// Report outputs the scan result as a human-readable summary.
// If Interactive is true, prompts the user to delete duplicates.
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

	if r.Interactive {
		return r.promptDelete(result)
	}

	return nil
}

func (r *TextReporter) promptDelete(result model.ScanResult) error {
	// Collect all files marked for deletion (all copies except the first in each group)
	var toDelete []string
	var reclaimBytes int64

	for _, g := range result.Groups {
		for _, f := range g.Files[1:] {
			toDelete = append(toDelete, f.Path)
			reclaimBytes += f.Size
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	fmt.Fprintf(r.Writer, "Delete %d files (%s)? [y/N] ",
		len(toDelete), formatBytes(reclaimBytes))

	scanner := bufio.NewScanner(r.Reader)
	if !scanner.Scan() {
		return nil
	}

	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(r.Writer, "Aborted.")
		return nil
	}

	if r.DryRun {
		fmt.Fprintln(r.Writer, "\033[33m[dry-run]\033[0m Would delete:")
		for _, path := range toDelete {
			fmt.Fprintf(r.Writer, "  %s\n", path)
		}
		return nil
	}

	deleted := 0
	for _, path := range toDelete {
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(r.Writer, "  \033[31mfailed\033[0m %s: %v\n", path, err)
		} else {
			fmt.Fprintf(r.Writer, "  \033[32mdeleted\033[0m %s\n", path)
			deleted++
		}
	}

	fmt.Fprintf(r.Writer, "\nDeleted %d files, reclaimed %s.\n",
		deleted, formatBytes(reclaimBytes))
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
