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
	Interactive bool // per-group prompts (y/n/q per group)
	BatchDelete bool // single prompt for all groups at once
}

// NewText creates a TextReporter that writes to stdout and reads from stdin.
func NewText(interactive bool, batchDelete bool, dryRun bool) *TextReporter {
	return &TextReporter{
		Writer:      os.Stdout,
		Reader:      os.Stdin,
		Interactive: interactive,
		BatchDelete: batchDelete,
		DryRun:      dryRun,
	}
}

// Report outputs the scan result as a human-readable summary.
func (r *TextReporter) Report(result model.ScanResult) error {
	if len(result.Groups) == 0 {
		fmt.Fprintf(r.Writer, "Scanned %d files (%s). No duplicates found.\n",
			result.TotalFiles, formatBytes(result.TotalBytes))
		return nil
	}

	// Sort groups by wasted bytes descending.
	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].WastedBytes > result.Groups[j].WastedBytes
	})

	fmt.Fprintf(r.Writer, "Scanned %d files (%s)\n",
		result.TotalFiles, formatBytes(result.TotalBytes))
	fmt.Fprintf(r.Writer, "Found %s of duplicates across %d files (%d groups)\n\n",
		formatBytes(result.WastedBytes), result.DuplicateFiles, len(result.Groups))

	if r.Interactive {
		return r.reportInteractive(result)
	}

	// Print all groups first.
	for i, g := range result.Groups {
		r.printGroup(i, g)
	}

	if r.BatchDelete || r.DryRun {
		return r.promptBatchDelete(result)
	}

	return nil
}

// reportInteractive shows each group one at a time and asks y/n/q per group.
func (r *TextReporter) reportInteractive(result model.ScanResult) error {
	scanner := bufio.NewScanner(r.Reader)
	var totalDeleted int
	var totalReclaimed int64

	for i, g := range result.Groups {
		r.printGroup(i, g)

		// Show duplicates that would be deleted (all except first).
		fmt.Fprintf(r.Writer, "  Delete %d duplicate(s) (%s)? [\033[32my\033[0m/\033[31mn\033[0m/\033[33mq\033[0m] ",
			len(g.Files)-1, formatBytes(g.WastedBytes))

		if !scanner.Scan() {
			break
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch answer {
		case "q", "quit":
			fmt.Fprintln(r.Writer, "Stopped.")
			return nil
		case "y", "yes":
			for _, f := range g.Files[1:] {
				if r.DryRun {
					fmt.Fprintf(r.Writer, "  \033[33m[dry-run]\033[0m would delete %s\n", f.Path)
				} else {
					if err := os.Remove(f.Path); err != nil {
						fmt.Fprintf(r.Writer, "  \033[31mfailed\033[0m %s: %v\n", f.Path, err)
					} else {
						fmt.Fprintf(r.Writer, "  \033[32mdeleted\033[0m %s\n", f.Path)
						totalDeleted++
						totalReclaimed += f.Size
					}
				}
			}
		default:
			fmt.Fprintln(r.Writer, "  Skipped.")
		}
		fmt.Fprintln(r.Writer)
	}

	if totalDeleted > 0 {
		fmt.Fprintf(r.Writer, "Done. Deleted %d files, reclaimed %s.\n",
			totalDeleted, formatBytes(totalReclaimed))
	}

	return nil
}

// promptBatchDelete asks once for all groups (non-interactive bulk mode).
func (r *TextReporter) promptBatchDelete(result model.ScanResult) error {
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

func (r *TextReporter) printGroup(i int, g model.DuplicateGroup) {
	fmt.Fprintf(r.Writer, "\033[1mGROUP %d\033[0m — %s wasted (%d copies)\n",
		i+1, formatBytes(g.WastedBytes), len(g.Files))
	for j, f := range g.Files {
		marker := "\033[32m✓\033[0m"
		if j > 0 {
			marker = "\033[31m✗\033[0m"
		}
		fmt.Fprintf(r.Writer, "  %s %s  %s  %s\n",
			marker, f.Path, formatBytes(f.Size), f.ModTime.Format("2006-01-02"))
	}
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
