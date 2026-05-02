package config

import (
	"flag"
	"time"
)

type Config struct {
	Port         string
	BackendURL   string
	APIKey       string
	DefaultModel string
	ForceModel   string
	Timeout      time.Duration
	LogFile      string
}

func Parse() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Port, "port", "8080", "server listen port")
	flag.StringVar(&cfg.BackendURL, "url", "", "backend chat completions endpoint (required)")
	flag.StringVar(&cfg.APIKey, "key", "", "API key for backend (required)")
	flag.StringVar(&cfg.DefaultModel, "default-model", "gpt-4o", "default model when not specified in request")
	flag.StringVar(&cfg.ForceModel, "model", "", "always use this model, overriding client request")
	flag.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "backend request timeout")
	flag.StringVar(&cfg.LogFile, "log", "", "log file path (default: stderr)")

	flag.Parse()

	return cfg
}
