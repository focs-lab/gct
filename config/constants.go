package config

import "flag"

const DEBUG = true

const TESTING_TIME_OUT_SECONDS = 10

var OutputSuffix = "_instrumented"

func init() {
	flag.StringVar(&OutputSuffix, "suffix", "_instrumented", "suffix to add to the instrumented file name (e.g., '_instrumented' produces 'demo_instrumented.go')")
}