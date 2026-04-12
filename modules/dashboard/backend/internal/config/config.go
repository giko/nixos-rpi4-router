// Package config parses command-line flags into a Config struct
// used by the dashboard process.
package config

import (
	"flag"
	"fmt"
	"io"
)

// Config holds the runtime configuration for the dashboard binary.
type Config struct {
	Bind       string // address:port to bind the HTTP server to
	AdguardURL string // base URL for the AdGuard Home REST API
	LogLevel   string // slog level: debug | info | warn | error
	ConfigFile string // path to NixOS-generated dashboard-config.json (optional)
}

// FromFlags parses args into a Config. Pass os.Args[1:] or nil for defaults.
func FromFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	// Suppress flag package's default stderr output on parse failure —
	// errors are returned to the caller via fs.Parse() and wrapped below.
	fs.SetOutput(io.Discard)
	bind := fs.String("bind", "127.0.0.1:9090", "host:port to bind on")
	adguard := fs.String("adguard-url", "http://127.0.0.1:3000", "AdGuard Home base URL")
	level := fs.String("log-level", "info", "slog level: debug|info|warn|error")
	cfgFile := fs.String("config-file", "", "path to NixOS-generated dashboard-config.json (optional)")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	return &Config{
		Bind:       *bind,
		AdguardURL: *adguard,
		LogLevel:   *level,
		ConfigFile: *cfgFile,
	}, nil
}
