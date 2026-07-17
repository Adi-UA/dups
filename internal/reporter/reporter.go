// Package reporter formats and displays duplicate scan results.
package reporter

import "github.com/Adi-UA/dups/internal/model"

// Reporter outputs scan results in a specific format.
type Reporter interface {
	Report(result model.ScanResult) error
}
