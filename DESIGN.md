# dups — Design Document

## Problem

Duplicate files accumulate on every machine: Downloads folder, photo libraries,
backup drives, project directories with vendored deps. Users don't know how
much space is wasted or which copies to keep. Existing tools (fdupes, rdfind)
are Linux-only, have no interactive mode, and produce ugly output.

dups scans a directory tree, finds duplicate files by content, shows the waste,
and lets you choose what to delete.

## Constraints

- Single binary, zero dependencies. Cross-platform (macOS, Linux, Windows).
- Fast: concurrent hashing with a worker pool sized to available CPUs.
- Smart: skip files by size first (files with unique sizes can't be duplicates),
  hash only when two or more files share the same size.
- Interactive TUI for reviewing and deleting, plus a non-interactive JSON mode
  for scripting.
- Installable via `go install` and Homebrew.

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   dups CLI                        │
│                                                  │
│  ┌──────────┐   ┌──────────┐   ┌─────────────┐ │
│  │  Walker   │──▶│  Hasher  │──▶│  Reporter   │ │
│  │ (fs tree) │   │ (worker  │   │ (TUI or     │ │
│  │           │   │  pool)   │   │  JSON)      │ │
│  └──────────┘   └──────────┘   └─────────────┘ │
└─────────────────────────────────────────────────┘
```

**Walker**: recursively traverses the target directory. Groups files by size.
Discards unique sizes (no duplicates possible). Sends size-matched groups to
the Hasher.

**Hasher**: worker pool (N = NumCPU). Reads files, computes SHA-256. Groups
files by hash. Discards unique hashes. Sends confirmed duplicate groups to
the Reporter.

**Reporter**: displays results. Two modes:
- `--json`: prints duplicate groups as JSON (for piping to other tools)
- Default: interactive TUI with color, sorted by wasted space (largest first),
  lets the user select which copies to delete.

## CLI Interface

```bash
# Scan current directory
dups .

# Scan a specific path, minimum file size 1MB
dups ~/Downloads --min-size 1mb

# Non-interactive JSON output
dups ~/Photos --json

# Dry run (show what would be deleted, don't actually delete)
dups . --dry-run

# Only show summary stats (no file list)
dups . --summary
```

## Output (interactive mode)

```
Scanning ~/Downloads... 4,218 files, 12.3 GB total

Found 847 MB of duplicates across 156 files (37 groups)

GROUP 1 — 234.5 MB wasted (3 copies, keep 1)
  ✓ ~/Downloads/video-2024.mp4          (234.5 MB, modified 2024-03-15)
  ✗ ~/Downloads/video-2024 (1).mp4      (234.5 MB, modified 2024-03-15)
  ✗ ~/Downloads/old/video-2024.mp4      (234.5 MB, modified 2024-01-02)

GROUP 2 — 89.2 MB wasted (2 copies, keep 1)
  ...

Delete 156 files (847 MB)? [y/N/select]
```

## Performance Strategy

1. **Size filter first**: if only one file has a given size, it can't have a
   duplicate. This eliminates 70-90% of files without reading any content.
2. **Partial hash**: for large files (>1 MB), hash only the first 4 KB. If
   partial hashes match, then hash the full file. Avoids reading 100 GB of
   video files that differ after the header.
3. **Worker pool**: N goroutines (default: NumCPU) read and hash in parallel.
   Channel-based pipeline: walker → hasher → reporter.
4. **Memory bounded**: stream file content through the hasher (64 KB buffer),
   never load entire files into memory.

## Data Flow

```
files on disk
    │
    ▼
Walker (filepath.WalkDir)
    │ groups by size
    ▼
Size Filter (discard unique sizes)
    │ size-matched groups
    ▼
Partial Hasher (first 4KB SHA-256)
    │ discard unique partial hashes
    ▼
Full Hasher (full SHA-256, worker pool)
    │ confirmed duplicate groups
    ▼
Reporter (TUI or JSON)
    │ user selects deletions
    ▼
Deleter (os.Remove)
```

## Milestones

**M1 (evening 1): Core pipeline**
- Walker + size filter + full hasher + JSON reporter
- No TUI, no partial hash, no deletion. Just finds and reports duplicates.
- Tests with temp directories containing known duplicates.

**M2 (evening 2): Performance + TUI**
- Partial hash optimization for large files
- Interactive TUI with color (lipgloss/bubbletea or simple ANSI)
- Delete confirmation + execution
- Progress bar during scan

**M3 (evening 3): Polish + release**
- `--min-size`, `--dry-run`, `--summary`, `--exclude` flags
- Cross-compile for macOS/Linux/Windows in CI
- Homebrew formula or goreleaser config
- README with demo GIF
