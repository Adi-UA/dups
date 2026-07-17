// dups finds duplicate files in a directory tree.
//
// Entry point. Parses CLI flags and runs the scan pipeline.
// Go programs always start at main() in the "main" package.
package main

import (
	"os"

	"github.com/Adi-UA/dups/cmd"
)

func main() {
	// os.Exit terminates with a status code. cmd.Run() returns 0 (success) or 1 (error).
	// Unlike Java/Python, Go's main() doesn't return a value — you must call os.Exit explicitly.
	os.Exit(cmd.Run())
}
