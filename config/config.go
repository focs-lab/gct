package config

import (
	"flag"
	"os"
)

var Cfg *Config

type Config struct {
	SchedulerName string
	Path          string
	TraceLoc      string
	GCTRoot       string
	GCTVersion    string
}

func ParseArgs() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Path, "path", "", "Path to the folder being instrumented")
	flag.StringVar(&cfg.GCTRoot, "gct-root", "", "Local GCT checkout to use via go.mod replace")
	flag.StringVar(&cfg.GCTVersion, "gct-version", "", "GCT module version to require")

	flag.Parse()

	if cfg.Path == "" {
		flag.Usage()
		os.Exit(1)
	}

	Cfg = cfg

	return cfg
}
