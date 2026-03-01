package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSuccess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "panex.toml")
	writeConfig(t, configPath, `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Extension.SourceDir != "./extension-src" {
		t.Fatalf("unexpected source_dir: got %q", cfg.Extension.SourceDir)
	}
	if cfg.Extension.OutDir != "./dist" {
		t.Fatalf("unexpected out_dir: got %q", cfg.Extension.OutDir)
	}
	if cfg.Server.Port != 4317 {
		t.Fatalf("unexpected port: got %d", cfg.Server.Port)
	}
	if cfg.Server.AuthToken != "test-token" {
		t.Fatalf("unexpected auth_token: got %q", cfg.Server.AuthToken)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "panex.toml")
	writeConfig(t, configPath, `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"
extra = "oops"

[server]
port = 4317
auth_token = "test-token"
`)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for unknown keys, got nil")
	}
	if !strings.Contains(err.Error(), "unknown config keys") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "extension.extra") {
		t.Fatalf("unexpected unknown key set: %v", err)
	}
}

func TestLoadValidationFailures(t *testing.T) {
	testCases := []struct {
		name      string
		tomlData  string
		wantError string
	}{
		{
			name: "missing source_dir",
			tomlData: `
[extension]
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "extension.source_dir is required",
		},
		{
			name: "missing out_dir",
			tomlData: `
[extension]
source_dir = "./extension-src"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "extension.out_dir is required",
		},
		{
			name: "port too low",
			tomlData: `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"

[server]
port = 0
auth_token = "test-token"
`,
			wantError: "server.port must be between",
		},
		{
			name: "port too high",
			tomlData: `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"

[server]
port = 70000
auth_token = "test-token"
`,
			wantError: "server.port must be between",
		},
		{
			name: "missing auth token",
			tomlData: `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"

[server]
port = 4317
`,
			wantError: "server.auth_token is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "panex.toml")
			writeConfig(t, configPath, tc.tomlData)

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantError)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("expected file-not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalidPathArgument(t *testing.T) {
	_, err := Load("   ")
	if err == nil {
		t.Fatal("expected path validation error, got nil")
	}
	if !strings.Contains(err.Error(), "config path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeConfig(t *testing.T, path, value string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(strings.TrimSpace(value)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}
}
