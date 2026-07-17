package reporter

import (
	"encoding/json"
	"io"
	"os"

	"github.com/Adi-UA/dups/internal/model"
)

// JSONReporter writes scan results as JSON to stdout.
type JSONReporter struct {
	Writer io.Writer
}

// NewJSON creates a JSONReporter that writes to stdout.
func NewJSON() *JSONReporter {
	return &JSONReporter{Writer: os.Stdout}
}

type jsonOutput struct {
	TotalFiles     int         `json:"total_files"`
	TotalBytes     int64       `json:"total_bytes"`
	DuplicateFiles int         `json:"duplicate_files"`
	WastedBytes    int64       `json:"wasted_bytes"`
	Groups         []jsonGroup `json:"groups"`
}

type jsonGroup struct {
	Hash        string     `json:"hash"`
	Size        int64      `json:"size"`
	Copies      int        `json:"copies"`
	WastedBytes int64      `json:"wasted_bytes"`
	Files       []jsonFile `json:"files"`
}

type jsonFile struct {
	Path    string `json:"path"`
	ModTime string `json:"mod_time"`
}

// Report outputs the scan result as formatted JSON.
func (r *JSONReporter) Report(result model.ScanResult) error {
	out := jsonOutput{
		TotalFiles:     result.TotalFiles,
		TotalBytes:     result.TotalBytes,
		DuplicateFiles: result.DuplicateFiles,
		WastedBytes:    result.WastedBytes,
	}

	for _, g := range result.Groups {
		jg := jsonGroup{
			Hash:        g.Hash,
			Size:        g.Size,
			Copies:      len(g.Files),
			WastedBytes: g.WastedBytes,
		}
		for _, f := range g.Files {
			jg.Files = append(jg.Files, jsonFile{
				Path:    f.Path,
				ModTime: f.ModTime.Format("2006-01-02"),
			})
		}
		out.Groups = append(out.Groups, jg)
	}

	enc := json.NewEncoder(r.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
