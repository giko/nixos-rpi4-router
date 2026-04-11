package config

import (
	"testing"
)

func TestFromFlagsDefaults(t *testing.T) {
	cfg, err := FromFlags(nil)
	if err != nil {
		t.Fatalf("FromFlags(nil) error: %v", err)
	}
	if cfg.Bind != "127.0.0.1:9090" {
		t.Errorf("Bind = %q, want 127.0.0.1:9090", cfg.Bind)
	}
	if cfg.AdguardURL != "http://127.0.0.1:3000" {
		t.Errorf("AdguardURL = %q, want http://127.0.0.1:3000", cfg.AdguardURL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestFromFlagsOverrides(t *testing.T) {
	cfg, err := FromFlags([]string{
		"--bind", "192.168.1.1:9090",
		"--adguard-url", "http://adguard.local",
		"--log-level", "debug",
	})
	if err != nil {
		t.Fatalf("FromFlags error: %v", err)
	}
	if cfg.Bind != "192.168.1.1:9090" {
		t.Errorf("Bind = %q, want 192.168.1.1:9090", cfg.Bind)
	}
	if cfg.AdguardURL != "http://adguard.local" {
		t.Errorf("AdguardURL = %q, want http://adguard.local", cfg.AdguardURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}
