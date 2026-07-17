// dups finds duplicate files in a directory tree.
package main

import (
	"os"

	"github.com/Adi-UA/dups/cmd"
)

func main() {
	os.Exit(cmd.Run())
}
