// Package policy implements the project-local policy engine.
// Policy constrains what agents and CI may do. Spec section 12.
package policy

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Policy is the resolved project policy.
type Policy struct {
	Version     int              `yaml:"version"     toml:"version"`
	Mutation    MutationPolicy   `yaml:"mutation"    toml:"mutation"`
	Targets     TargetsPolicy    `yaml:"targets"     toml:"targets"`
	Permissions PermPolicy       `yaml:"permissions" toml:"permissions"`
	Runtime     RuntimePolicy    `yaml:"runtime"     toml:"runtime"`
	Repairs     RepairsPolicy    `yaml:"repairs"     toml:"repairs"`
	Publishing  PublishingPolicy `yaml:"publishing"  toml:"publishing"`
	Hash        string           `yaml:"-"           toml:"-"`
}

type MutationPolicy struct {
	AllowFileCreation      bool `yaml:"allow_file_creation"     toml:"allow_file_creation"`
	AllowFileUpdate        bool `yaml:"allow_file_update"       toml:"allow_file_update"`
	AllowFileDeletion      bool `yaml:"allow_file_deletion"     toml:"allow_file_deletion"`
	AllowDependencyInstall bool `yaml:"allow_dependency_install" toml:"allow_dependency_install"`
	AllowLockfileChanges   bool `yaml:"allow_lockfile_changes"  toml:"allow_lockfile_changes"`
	AllowBundlerRewrite    bool `yaml:"allow_bundler_rewrite"   toml:"allow_bundler_rewrite"`
}

type TargetsPolicy struct {
	Allowed []string `yaml:"allowed" toml:"allowed"`
}

type PermPolicy struct {
	AllowNewPermissions     bool `yaml:"allow_new_permissions"          toml:"allow_new_permissions"`
	AllowNewHostPermissions bool `yaml:"allow_new_host_permissions"     toml:"allow_new_host_permissions"`
	RequirePermDiffReview   bool `yaml:"require_permission_diff_review" toml:"require_permission_diff_review"`
}

type RuntimePolicy struct {
	AllowLoopbackBridge  bool `yaml:"allow_loopback_bridge"  toml:"allow_loopback_bridge"`
	AllowNativeMessaging bool `yaml:"allow_native_messaging" toml:"allow_native_messaging"`
}

type RepairsPolicy struct {
	AutoApplySafe bool `yaml:"auto_apply_safe_repairs" toml:"auto_apply_safe_repairs"`
	MaxAttempts   int  `yaml:"max_attempts"            toml:"max_attempts"`
}

type PublishingPolicy struct {
	AllowPublish      bool `yaml:"allow_publish"       toml:"allow_publish"`
	RequireVerifyPass bool `yaml:"require_verify_pass"  toml:"require_verify_pass"`
}

// Default returns a conservative default policy.
func Default() *Policy {
	return &Policy{
		Version: 1,
		Mutation: MutationPolicy{
			AllowFileCreation:      true,
			AllowFileUpdate:        true,
			AllowFileDeletion:      false,
			AllowDependencyInstall: true,
			AllowLockfileChanges:   true,
			AllowBundlerRewrite:    false,
		},
		Targets: TargetsPolicy{
			Allowed: []string{"chrome", "firefox", "safari"},
		},
		Permissions: PermPolicy{
			AllowNewPermissions:     true,
			AllowNewHostPermissions: false,
			RequirePermDiffReview:   true,
		},
		Runtime: RuntimePolicy{
			AllowLoopbackBridge:  true,
			AllowNativeMessaging: false,
		},
		Repairs: RepairsPolicy{
			AutoApplySafe: true,
			MaxAttempts:   3,
		},
		Publishing: PublishingPolicy{
			AllowPublish:      false,
			RequireVerifyPass: true,
		},
	}
}

// LoadFromFile reads a policy from a TOML file.
// TOML is used instead of YAML to stay consistent with existing Panex config.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	p := Default()
	if err := toml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}

	h := sha256.Sum256(data)
	p.Hash = fmt.Sprintf("sha256:%x", h)

	return p, nil
}

// Action describes something a component wants to do.
type Action struct {
	Kind   string // "file_create", "file_update", "file_delete", "dependency_install", "permission_add", "host_permission_add", "publish", "repair"
	Detail string // human-readable detail
}

// Denial is a structured policy denial.
type Denial struct {
	Rule   string `json:"rule"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// Evaluate checks whether an action is permitted by the policy.
// Returns nil if allowed, or a Denial if blocked.
func (p *Policy) Evaluate(action Action) *Denial {
	switch action.Kind {
	case "file_create":
		if !p.Mutation.AllowFileCreation {
			return &Denial{
				Rule:   "mutation.allow_file_creation",
				Action: action.Detail,
				Reason: "file creation is not allowed by policy",
			}
		}
	case "file_update":
		if !p.Mutation.AllowFileUpdate {
			return &Denial{
				Rule:   "mutation.allow_file_update",
				Action: action.Detail,
				Reason: "file update is not allowed by policy",
			}
		}
	case "file_delete":
		if !p.Mutation.AllowFileDeletion {
			return &Denial{
				Rule:   "mutation.allow_file_deletion",
				Action: action.Detail,
				Reason: "file deletion is not allowed by policy",
			}
		}
	case "dependency_install":
		if !p.Mutation.AllowDependencyInstall {
			return &Denial{
				Rule:   "mutation.allow_dependency_install",
				Action: action.Detail,
				Reason: "dependency installation is not allowed by policy",
			}
		}
	case "permission_add":
		if !p.Permissions.AllowNewPermissions {
			return &Denial{
				Rule:   "permissions.allow_new_permissions",
				Action: action.Detail,
				Reason: "adding new permissions is not allowed by policy",
			}
		}
	case "host_permission_add":
		if !p.Permissions.AllowNewHostPermissions {
			return &Denial{
				Rule:   "permissions.allow_new_host_permissions",
				Action: action.Detail,
				Reason: "adding new host permissions is not allowed by policy",
			}
		}
	case "publish":
		if !p.Publishing.AllowPublish {
			return &Denial{
				Rule:   "publishing.allow_publish",
				Action: action.Detail,
				Reason: "publishing is not allowed by policy",
			}
		}
	case "target_add":
		if len(p.Targets.Allowed) > 0 {
			allowed := false
			for _, t := range p.Targets.Allowed {
				if t == action.Detail {
					allowed = true
					break
				}
			}
			if !allowed {
				return &Denial{
					Rule:   "targets.allowed",
					Action: action.Detail,
					Reason: fmt.Sprintf("target %q is not in the allowed list", action.Detail),
				}
			}
		}
	}

	return nil
}

// IsTargetAllowed checks if a target is permitted by policy.
func (p *Policy) IsTargetAllowed(tgt string) bool {
	if len(p.Targets.Allowed) == 0 {
		return true
	}
	for _, t := range p.Targets.Allowed {
		if t == tgt {
			return true
		}
	}
	return false
}
