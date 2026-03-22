package config

import (
	"errors"
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

	if len(cfg.Extensions) != 1 {
		t.Fatalf("unexpected extensions count: got %d, want 1", len(cfg.Extensions))
	}
	if cfg.Extensions[0].ID != DefaultExtensionID {
		t.Fatalf("unexpected default extension id: got %q, want %q", cfg.Extensions[0].ID, DefaultExtensionID)
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
	if cfg.Server.EventStorePath != DefaultEventStorePath {
		t.Fatalf("unexpected default event_store_path: got %q, want %q", cfg.Server.EventStorePath, DefaultEventStorePath)
	}
	if cfg.Server.BindAddress != DefaultBindAddress {
		t.Fatalf("unexpected default bind_address: got %q, want %q", cfg.Server.BindAddress, DefaultBindAddress)
	}
}

func TestLoadMultipleExtensions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "panex.toml")
	writeConfig(t, configPath, `
[[extensions]]
id = "popup"
source_dir = "./extensions/popup"
out_dir = "./dist/popup"

[[extensions]]
id = "admin"
source_dir = "./extensions/admin"
out_dir = "./dist/admin"

[server]
port = 4317
auth_token = "test-token"
`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.Extensions) != 2 {
		t.Fatalf("unexpected extensions count: got %d, want 2", len(cfg.Extensions))
	}
	if cfg.Extensions[0].ID != "popup" || cfg.Extensions[1].ID != "admin" {
		t.Fatalf("unexpected extension ids: %+v", cfg.Extensions)
	}
	if cfg.Extension.ID != "popup" {
		t.Fatalf("expected legacy extension alias to point at the first extension, got %q", cfg.Extension.ID)
	}
}

func TestLoadExplicitEventStorePath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "panex.toml")
	writeConfig(t, configPath, `
[extension]
source_dir = "./extension-src"
out_dir = "./dist"

[server]
port = 4317
auth_token = "test-token"
event_store_path = "./.panex/custom-events.db"
`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Server.EventStorePath != "./.panex/custom-events.db" {
		t.Fatalf("unexpected event_store_path: got %q", cfg.Server.EventStorePath)
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
		{
			name: "source_dir equals out_dir",
			tomlData: `
[extension]
source_dir = "./same-dir"
out_dir = "./same-dir"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "extension.source_dir and extension.out_dir must not overlap",
		},
		{
			name: "out_dir nested within source_dir",
			tomlData: `
[extension]
source_dir = "./extension"
out_dir = "./extension/dist"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "extension.source_dir and extension.out_dir must not overlap",
		},
		{
			name: "source_dir nested within out_dir",
			tomlData: `
[extension]
source_dir = "./workspace/src"
out_dir = "./workspace"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "extension.source_dir and extension.out_dir must not overlap",
		},
		{
			name: "cannot mix legacy and multi-extension config",
			tomlData: `
[extension]
source_dir = "./legacy"
out_dir = "./legacy-dist"

[[extensions]]
id = "popup"
source_dir = "./popup"
out_dir = "./popup-dist"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: "use either [extension] or [[extensions]], not both",
		},
		{
			name: "multi-extension config requires ids",
			tomlData: `
[[extensions]]
source_dir = "./popup"
out_dir = "./popup-dist"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: `extensions[0].id is required`,
		},
		{
			name: "extension ids must be unique",
			tomlData: `
[[extensions]]
id = "popup"
source_dir = "./popup"
out_dir = "./popup-dist"

[[extensions]]
id = "popup"
source_dir = "./admin"
out_dir = "./admin-dist"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: `extension ids must be unique: "popup"`,
		},
		{
			name: "extensions must not overlap each other",
			tomlData: `
[[extensions]]
id = "popup"
source_dir = "./extensions/shared"
out_dir = "./dist/popup"

[[extensions]]
id = "admin"
source_dir = "./extensions/shared/admin"
out_dir = "./dist/admin"

[server]
port = 4317
auth_token = "test-token"
`,
			wantError: `extensions "popup" and "admin" must not share overlapping source_dir or out_dir paths`,
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
	if !errors.Is(err, ErrConfigFileNotFound) {
		t.Fatalf("expected ErrConfigFileNotFound, got %v", err)
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

func TestLoadAllowsInfrastructureShieldedOutput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "panex.toml")
	writeConfig(t, configPath, `
[extension]
source_dir = "."
out_dir = ".panex/dist"

[server]
port = 4317
auth_token = "test-token"
`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Extension.OutDir != ".panex/dist" {
		t.Fatalf("unexpected out_dir: got %q", cfg.Extension.OutDir)
	}
}

func TestInferSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"manifest_version": 3}`), 0o600); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}

	cfg, err := Infer(dir)
	if err != nil {
		t.Fatalf("Infer() returned error: %v", err)
	}

	if cfg.Extension.SourceDir != dir {
		t.Fatalf("unexpected source_dir: got %q, want %q", cfg.Extension.SourceDir, dir)
	}
	if cfg.Extension.OutDir != filepath.Join(dir, DefaultOutDir) {
		t.Fatalf("unexpected out_dir: got %q, want %q", cfg.Extension.OutDir, filepath.Join(dir, DefaultOutDir))
	}
	if cfg.Extension.ID != DefaultExtensionID {
		t.Fatalf("unexpected extension id: got %q, want %q", cfg.Extension.ID, DefaultExtensionID)
	}
	if cfg.Server.Port != DefaultPort {
		t.Fatalf("unexpected port: got %d, want %d", cfg.Server.Port, DefaultPort)
	}
	if cfg.Server.AuthToken != DefaultAuthToken {
		t.Fatalf("unexpected auth_token: got %q, want %q", cfg.Server.AuthToken, DefaultAuthToken)
	}
	if cfg.Server.EventStorePath != DefaultEventStorePath {
		t.Fatalf("unexpected event_store_path: got %q, want %q", cfg.Server.EventStorePath, DefaultEventStorePath)
	}
	if cfg.Server.BindAddress != DefaultBindAddress {
		t.Fatalf("unexpected bind_address: got %q, want %q", cfg.Server.BindAddress, DefaultBindAddress)
	}
	if len(cfg.Extensions) != 1 {
		t.Fatalf("unexpected extensions count: got %d, want 1", len(cfg.Extensions))
	}
}

func TestInferMissingManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := Infer(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrManifestNotFound) {
		t.Fatalf("expected ErrManifestNotFound, got %v", err)
	}
}

func TestInferEmptyDir(t *testing.T) {
	_, err := Infer("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "directory is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsShieldedByInfrastructureDir(t *testing.T) {
	base := t.TempDir()
	testCases := []struct {
		name   string
		parent string
		child  string
		want   bool
	}{
		{
			name:   "dot-prefixed output dir",
			parent: base,
			child:  filepath.Join(base, ".panex", "dist"),
			want:   true,
		},
		{
			name:   "node_modules output dir",
			parent: base,
			child:  filepath.Join(base, "node_modules", "cache"),
			want:   true,
		},
		{
			name:   "regular output dir",
			parent: base,
			child:  filepath.Join(base, "dist"),
			want:   false,
		},
		{
			name:   "nested non-infra output dir",
			parent: base,
			child:  filepath.Join(base, "build", "output"),
			want:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isShieldedByInfrastructureDir(tc.parent, tc.child)
			if got != tc.want {
				t.Fatalf("isShieldedByInfrastructureDir(%q, %q) = %v, want %v", tc.parent, tc.child, got, tc.want)
			}
		})
	}
}

func writeConfig(t *testing.T, path, value string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(strings.TrimSpace(value)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}
}
