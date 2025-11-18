package config

import (
	"flag"
	"os"
	"path/filepath"
)

type Options struct {
	BPFPath   string
	JSONLines bool
}

func Parse() Options {
	var opts Options

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	defaultObj := filepath.Join(cwd, "bpf", "main.bpf.o")
	flag.StringVar(&opts.BPFPath, "bpf", defaultObj, "absolute path to the compiled eBPF object file")
	flag.BoolVar(&opts.JSONLines, "json", false, "emit events as JSON lines")

	flag.Parse()
	return opts
}
