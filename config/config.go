package config

import (
	"flag"
	"os"
)

var Cfg *Config

type Config struct {
	SchedulerName string
	Path          string
	TraceLoc	  string
}

func ParseArgs() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Path, "path", "", "Path to the folder being instrumented")

	flag.Parse()

	if cfg.Path == "" {
		flag.Usage()
		os.Exit(1)
	}

	Cfg = cfg

	return cfg
}