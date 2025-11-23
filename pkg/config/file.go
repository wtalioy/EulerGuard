package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// FileConfig represents the YAML config file structure
type FileConfig struct {
	BPFPath     string            `yaml:"bpf_path,omitempty"`
	RulesPath   string            `yaml:"rules_path,omitempty"`
	LogFile     string            `yaml:"log_file,omitempty"`
	JSONOutput  bool              `yaml:"json_output,omitempty"`
	Performance PerformanceConfig `yaml:"performance,omitempty"`
}

type PerformanceConfig struct {
	RingBufSize        int    `yaml:"ringbuf_size,omitempty"`
	ProcessTreeMaxAge  string `yaml:"proctree_max_age,omitempty"`
	ProcessTreeMaxSize int    `yaml:"proctree_max_size,omitempty"`
	LogBufferSize      int    `yaml:"log_buffer_size,omitempty"`
}

// LoadConfigFile loads configuration from a YAML file
func LoadConfigFile(path string) (*FileConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Config file is optional
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}

// ParseDuration parses duration strings like "30m", "1h", etc.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
