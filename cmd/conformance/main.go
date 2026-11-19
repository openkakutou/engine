// Command conformance runs characters through the engine's mirror match and
// reports the controller types and triggers the engine does not support.
//
// Usage: go run ./cmd/conformance [-ticks N] [-json] [-common DIR] <path>...
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/openkakutou/engine/v2/conformance"
)

func main() {
	ticks := flag.Int("ticks", conformance.DefaultTicks, "number of ticks to simulate per character")
	asJSON := flag.Bool("json", false, "write the report as JSON instead of text")
	common := flag.String("common", "", "extra folder searched for state files, such as the game's data/ folder holding common1.cns")
	flag.Parse()
	if flag.NArg() == 0 || *ticks <= 0 {
		fmt.Fprintln(os.Stderr, "usage: conformance [-ticks N] [-json] [-common DIR] <path>...")
		os.Exit(2)
	}

	opts := conformance.Options{Ticks: *ticks}
	if *common != "" {
		opts.SearchDirs = []string{*common}
	}
	report, err := conformance.Run(flag.Args(), opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *asJSON {
		err = report.WriteJSON(os.Stdout)
	} else {
		err = report.WriteText(os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
