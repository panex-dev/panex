package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	p := Default()

	if p.Version != 1 {
		t.Errorf("version: got %d", p.Version)
	}
	if !p.Mutation.AllowFileCreation {
		t.Error("default should allow file creation")
	}
	if p.Mutation.AllowFileDeletion {
		t.Error("default should NOT allow file deletion")
	}
	if p.Permissions.AllowNewHostPermissions {
		t.Error("default should NOT allow new host permissions")
	}
	if !p.Repairs.AutoApplySafe {
		t.Error("default should auto-apply safe repairs")
	}
	if p.Publishing.AllowPublish {
		t.Error("default should NOT allow publish")
	}
}

func TestEvaluate_Allowed(t *testing.T) {
	p := Default()

	tests := []Action{
		{Kind: "file_create", Detail: "create panex.config.ts"},
		{Kind: "file_update", Detail: "update manifest.json"},
		{Kind: "dependency_install", Detail: "install react"},
		{Kind: "permission_add", Detail: "add tabs permission"},
	}

	for _, a := range tests {
		if d := p.Evaluate(a); d != nil {
			t.Errorf("expected %s to be allowed, got denial: %s", a.Kind, d.Reason)
		}
	}
}

func TestEvaluate_Denied(t *testing.T) {
	p := Default()

	tests := []struct {
		action Action
		rule   string
	}{
		{Action{Kind: "file_delete", Detail: "delete src/old.ts"}, "mutation.allow_file_deletion"},
		{Action{Kind: "host_permission_add", Detail: "add https://*/*"}, "permissions.allow_new_host_permissions"},
		{Action{Kind: "publish", Detail: "publish to chrome store"}, "publishing.allow_publish"},
	}

	for _, tt := range tests {
		d := p.Evaluate(tt.action)
		if d == nil {
			t.Errorf("expected denial for %s", tt.action.Kind)
			continue
		}
		if d.Rule != tt.rule {
			t.Errorf("rule: got %s, want %s", d.Rule, tt.rule)
		}
	}
}

func TestEvaluate_TargetAllowed(t *testing.T) {
	p := Default()
	p.Targets.Allowed = []string{"chrome", "firefox"}

	// Allowed target
	if d := p.Evaluate(Action{Kind: "target_add", Detail: "chrome"}); d != nil {
		t.Errorf("chrome should be allowed: %s", d.Reason)
	}

	// Disallowed target
	d := p.Evaluate(Action{Kind: "target_add", Detail: "safari"})
	if d == nil {
		t.Error("safari should be denied")
	}
}

func TestIsTargetAllowed(t *testing.T) {
	p := Default()
	p.Targets.Allowed = []string{"chrome"}

	if !p.IsTargetAllowed("chrome") {
		t.Error("chrome should be allowed")
	}
	if p.IsTargetAllowed("firefox") {
		t.Error("firefox should not be allowed")
	}
}

func TestIsTargetAllowed_EmptyList(t *testing.T) {
	p := Default()
	p.Targets.Allowed = nil

	if !p.IsTargetAllowed("anything") {
		t.Error("empty allowed list should permit all targets")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panex.policy.toml")

	content := `
version = 1

[mutation]
allow_file_creation = true
allow_file_deletion = true

[targets]
allowed = ["chrome"]

[permissions]
allow_new_host_permissions = true

[repairs]
auto_apply_safe_repairs = false
max_attempts = 5
`
	os.WriteFile(path, []byte(content), 0o644)

	p, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if !p.Mutation.AllowFileDeletion {
		t.Error("expected allow_file_deletion=true from file")
	}
	if len(p.Targets.Allowed) != 1 || p.Targets.Allowed[0] != "chrome" {
		t.Errorf("targets: got %v", p.Targets.Allowed)
	}
	if !p.Permissions.AllowNewHostPermissions {
		t.Error("expected allow_new_host_permissions=true from file")
	}
	if p.Repairs.AutoApplySafe {
		t.Error("expected auto_apply_safe_repairs=false from file")
	}
	if p.Repairs.MaxAttempts != 5 {
		t.Errorf("max_attempts: got %d, want 5", p.Repairs.MaxAttempts)
	}
	if p.Hash == "" {
		t.Error("expected policy hash")
	}
}
