package main

import (
	"fmt"
	"os"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/instrumenter"
)

// usage:
// go run ./cmd/instrumentation/instr -path [dir_to_folder]
func main() {
	cfg := config.ParseArgs()

	replaceRoot := cfg.GCTRoot
	if replaceRoot == "" {
		replaceRoot = os.Getenv(config.ROOT_PROJ_LOC)
	}

	if err := instrumenter.Run(cfg.Path, instrumenter.Options{
		ReplaceRoot: replaceRoot,
		Version:     cfg.GCTVersion,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
