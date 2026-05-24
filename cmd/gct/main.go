package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/instrumentation/instrumenter"
)

const (
	defaultTrace     = "trace.log"
	defaultScheduler = "random_walk"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "instrument":
		err = runInstrument(os.Args[2:])
	case "test":
		err = runTest(os.Args[2:])
	case "replay":
		err = runReplay(os.Args[2:])
	case "clean":
		err = runClean(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "gct: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "gct:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`GCT: Controlled Concurrency Testing for Go

Usage:
  gct instrument <path>
  gct test [gct flags] [go test args...]
  gct replay <trace.log> [go test args...]
  gct clean [packages...]

Examples:
  gct instrument ./client/v3
  gct test ./...
  gct test -trace trace.log -scheduler random_walk ./...
  gct test -run TestTxnPanics
  gct replay trace.log
  gct replay trace.log -run TestTxnPanics
  gct clean ./...

Flags for gct test:
  -trace <file>        trace file to write; default trace.log
  -scheduler <name>    scheduler to use; default random_walk
`)
}

func runInstrument(args []string) error {
	if len(args) != 1 {
		return errors.New("instrument requires exactly one target path")
	}

	target := args[0]
	target = normalizeInstrumentPath(target)
	fmt.Printf("GCT: instrumenting %s\n", target)

	return instrumenter.Run(target, instrumenter.Options{
		ReplaceRoot: os.Getenv(config.ROOT_PROJ_LOC),
	})
}

func runTest(args []string) error {
	trace := defaultTrace
	scheduler := defaultScheduler

	goTestArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--":
			goTestArgs = append(goTestArgs, args[i+1:]...)
			i = len(args)
		case "-trace":
			if i+1 >= len(args) {
				return errors.New("-trace requires a file")
			}
			trace = args[i+1]
			i++
		case "-scheduler":
			if i+1 >= len(args) {
				return errors.New("-scheduler requires a name")
			}
			scheduler = args[i+1]
			i++
		default:
			if value, ok := strings.CutPrefix(args[i], "-trace="); ok {
				trace = value
				continue
			}
			if value, ok := strings.CutPrefix(args[i], "-scheduler="); ok {
				scheduler = value
				continue
			}
			goTestArgs = append(goTestArgs, args[i])
		}
	}

	if len(goTestArgs) == 0 {
		goTestArgs = []string{"./..."}
	}

	fmt.Printf("GCT: running go test with scheduler %q\n", scheduler)
	fmt.Printf("GCT: writing trace to %s\n", trace)

	return runGoTest(goTestArgs, trace, scheduler)
}

func runReplay(args []string) error {
	if len(args) < 1 {
		return errors.New("replay requires a trace file")
	}

	trace := args[0]
	goTestArgs := args[1:]
	if len(goTestArgs) == 0 {
		goTestArgs = []string{"./..."}
	}

	fmt.Printf("GCT: replaying trace %s\n", trace)

	return runGoTest(goTestArgs, trace, "replay")
}

func runClean(args []string) error {
	if len(args) == 0 {
		args = []string{"./..."}
	}

	fmt.Println("GCT: clean is not implemented yet.")
	fmt.Println("GCT: for now, restore instrumented files using git checkout -- <path>.")
	fmt.Println("GCT: requested target(s):", args)

	return nil
}

func runGoTest(goTestArgs []string, trace string, scheduler string) error {
	cmdArgs := append([]string{"test"}, goTestArgs...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		config.TRACE_LOC+"="+trace,
		config.SCHEDULER_NAME+"="+scheduler,
		config.RECORD_FLAG+"=true",
	)

	return cmd.Run()
}

func normalizeInstrumentPath(path string) string {
	if path == "..." {
		return "."
	}
	if strings.HasSuffix(path, string(filepath.Separator)+"...") {
		return strings.TrimSuffix(path, string(filepath.Separator)+"...")
	}
	if strings.HasSuffix(path, "/...") {
		return strings.TrimSuffix(path, "/...")
	}
	return path
}
