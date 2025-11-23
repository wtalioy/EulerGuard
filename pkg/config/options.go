package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// Default configuration values
	DefaultProcessTreeMaxSize = 10000
	DefaultLogBufferSize      = 64 * 1024  // 64KB
	DefaultRingBufferSize     = 256 * 1024 // 256KB
)

var (
	// Default duration (cannot be const)
	DefaultProcessTreeMaxAge = 30 * time.Minute
)

type Options struct {
	BPFPath   string
	RulesPath string
	LogFile   string
	JSONLines bool

	// Performance tuning
	ProcessTreeMaxAge  time.Duration
	ProcessTreeMaxSize int
	LogBufferSize      int
	RingBufferSize     int // eBPF ring buffer size in bytes
}

func Parse() Options {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Default values
	defaultObj := filepath.Join(cwd, "bpf", "main.bpf.o")
	defaultRules := filepath.Join(cwd, "rules.yaml")
	defaultLog := filepath.Join(cwd, "eulerguard.log")
	defaultConfig := filepath.Join(cwd, "eulerguard.yaml")

	// 1. Define all flags upfront to avoid "flag provided but not defined" errors
	var processTreeMaxAgeMin int
	configFile := flag.String("config", defaultConfig, "path to config file (optional)")
	bpfPath := flag.String("bpf", "", "absolute path to the compiled eBPF object file")
	rulesPath := flag.String("rules", "", "path to the rules YAML file")
	logFile := flag.String("log", "", "path to log file")
	jsonOutput := flag.Bool("json", false, "emit events as JSON lines")
	flag.IntVar(&processTreeMaxAgeMin, "proctree-max-age", 0, "process tree max age in minutes")
	processTreeMaxSize := flag.Int("proctree-max-size", 0, "max processes in tree")
	logBufferSize := flag.Int("log-buffer-size", 0, "log buffer size in bytes")
	ringBufferSize := flag.Int("ringbuf-size", 0, "eBPF ring buffer size in bytes")

	// Parse all flags once
	flag.Parse()

	// 2. Load config file (lowest priority)
	fileConfig, err := LoadConfigFile(*configFile)
	if err != nil {
		// Config file error is non-fatal, just log it
		fmt.Fprintf(os.Stderr, "Warning: failed to load config file: %v\n", err)
	}

	// 3. Apply defaults
	opts := Options{
		BPFPath:            defaultObj,
		RulesPath:          defaultRules,
		LogFile:            defaultLog,
		JSONLines:          false,
		ProcessTreeMaxAge:  DefaultProcessTreeMaxAge,
		ProcessTreeMaxSize: DefaultProcessTreeMaxSize,
		LogBufferSize:      DefaultLogBufferSize,
		RingBufferSize:     DefaultRingBufferSize,
	}

	// 4. Apply config file values (override defaults)
	applyFileConfig(&opts, fileConfig)

	// 5. Apply environment variables (override config file)
	applyEnvVars(&opts)

	// 6. Apply command-line flags (highest priority, override everything)
	applyFlagValues(&opts, bpfPath, rulesPath, logFile, jsonOutput,
		processTreeMaxAgeMin, processTreeMaxSize, logBufferSize, ringBufferSize)

	return opts
}

// applyFileConfig applies configuration from file
func applyFileConfig(opts *Options, cfg *FileConfig) {
	if cfg == nil {
		return
	}

	// Apply string fields if non-empty
	applyIfNotEmpty(&opts.BPFPath, cfg.BPFPath)
	applyIfNotEmpty(&opts.RulesPath, cfg.RulesPath)
	applyIfNotEmpty(&opts.LogFile, cfg.LogFile)

	// Apply boolean
	if cfg.JSONOutput {
		opts.JSONLines = true
	}

	// Apply performance settings if non-zero
	applyIfPositive(&opts.RingBufferSize, cfg.Performance.RingBufSize)
	applyIfPositive(&opts.ProcessTreeMaxSize, cfg.Performance.ProcessTreeMaxSize)
	applyIfPositive(&opts.LogBufferSize, cfg.Performance.LogBufferSize)

	// Apply duration if valid
	if cfg.Performance.ProcessTreeMaxAge != "" {
		if d, err := ParseDuration(cfg.Performance.ProcessTreeMaxAge); err == nil {
			opts.ProcessTreeMaxAge = d
		}
	}
}

// applyIfNotEmpty sets target to value if value is not empty
func applyIfNotEmpty(target *string, value string) {
	if value != "" {
		*target = value
	}
}

// applyIfPositive sets target to value if value is positive
func applyIfPositive(target *int, value int) {
	if value > 0 {
		*target = value
	}
}

// applyEnvVars applies environment variable overrides
func applyEnvVars(opts *Options) {
	// Apply string environment variables
	applyIfNotEmpty(&opts.BPFPath, os.Getenv("EULERGUARD_BPF_PATH"))
	applyIfNotEmpty(&opts.RulesPath, os.Getenv("EULERGUARD_RULES_PATH"))
	applyIfNotEmpty(&opts.LogFile, os.Getenv("EULERGUARD_LOG_FILE"))

	// Apply boolean environment variable
	if env := os.Getenv("EULERGUARD_JSON"); env == "true" || env == "1" {
		opts.JSONLines = true
	}
}

// applyFlagValues applies parsed command-line flag values
func applyFlagValues(opts *Options, bpfPath, rulesPath, logFile *string, jsonOutput *bool,
	processTreeMaxAgeMin int, processTreeMaxSize, logBufferSize, ringBufferSize *int) {

	// Apply string flags
	applyIfNotEmpty(&opts.BPFPath, *bpfPath)
	applyIfNotEmpty(&opts.RulesPath, *rulesPath)
	applyIfNotEmpty(&opts.LogFile, *logFile)

	// Apply boolean flag
	if *jsonOutput {
		opts.JSONLines = true
	}

	// Apply integer flags
	applyIfPositive(&opts.ProcessTreeMaxSize, *processTreeMaxSize)
	applyIfPositive(&opts.LogBufferSize, *logBufferSize)
	applyIfPositive(&opts.RingBufferSize, *ringBufferSize)

	// Apply duration flag
	if processTreeMaxAgeMin > 0 {
		opts.ProcessTreeMaxAge = time.Duration(processTreeMaxAgeMin) * time.Minute
	}
}
