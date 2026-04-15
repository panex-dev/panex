// Package target defines the target adapter interface and canonical
// outcome model. Every target adapter operation returns a structured
// Result with an explicit outcome. Spec sections 17.4 and 17.5.
package target

import "context"

// Outcome is the canonical result state for any adapter operation.
type Outcome string

const (
	Success            Outcome = "success"
	Degraded           Outcome = "degraded"
	Blocked            Outcome = "blocked"
	NotAvailable       Outcome = "not_available"
	PolicyDeniedResult Outcome = "policy_denied"
	EnvironmentMissing Outcome = "environment_missing"
)

// Result is the uniform envelope returned by every adapter method.
type Result struct {
	Adapter      string   `json:"adapter"`
	Operation    string   `json:"operation"`
	Outcome      Outcome  `json:"outcome"`
	Reason       string   `json:"reason,omitempty"`
	ReasonCode   string   `json:"reason_code,omitempty"`
	Details      any      `json:"details,omitempty"`
	Suggestions  []string `json:"suggestions,omitempty"`
	Repairable   bool     `json:"repairable"`
	EvidencePath string   `json:"evidence_path,omitempty"`
}

// CapabilityCatalog describes what a target supports.
type CapabilityCatalog struct {
	Target       string                       `json:"target"`
	Capabilities map[string]CapabilitySupport `json:"capabilities"`
}

// CapabilitySupport describes how a capability maps to this target.
type CapabilitySupport struct {
	State      string `json:"state"` // native, adapted, degraded, blocked
	Permission string `json:"permission,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// ManifestOutput is the output of manifest compilation for a target.
type ManifestOutput struct {
	Manifest        map[string]any `json:"manifest"`
	Permissions     []string       `json:"permissions"`
	HostPermissions []string       `json:"host_permissions"`
	Warnings        []string       `json:"warnings"`
}

// EnvironmentInfo describes the detected environment for a target.
type EnvironmentInfo struct {
	Available  bool   `json:"available"`
	BinaryPath string `json:"binary_path,omitempty"`
	Version    string `json:"version,omitempty"`
	Channel    string `json:"channel,omitempty"`
	Launchable bool   `json:"launchable"`
	Reason     string `json:"reason,omitempty"`
}

// Adapter is the interface every target must implement.
// All methods return a Result envelope. No method may throw an
// unstructured error as its primary failure path.
type Adapter interface {
	// Name returns the target identifier (e.g., "chrome", "firefox").
	Name() string

	// Catalog returns the capability support catalog for this target.
	Catalog() CapabilityCatalog

	// InspectEnvironment checks if this target can be used on the current system.
	InspectEnvironment(ctx context.Context) (EnvironmentInfo, Result)

	// ResolveCapabilities resolves requested capabilities against this target's catalog.
	ResolveCapabilities(capabilities map[string]any) (map[string]CapabilityResolution, Result)

	// CompileManifest generates a target-specific manifest from resolved capabilities.
	CompileManifest(opts ManifestCompileOptions) (ManifestOutput, Result)

	// PackageArtifact creates a distributable artifact for this target.
	PackageArtifact(ctx context.Context, opts PackageOptions) (ArtifactRecord, Result)
}

// CapabilityResolution is the resolved state of a capability for a target.
type CapabilityResolution struct {
	State       string   `json:"state"` // native, adapted, degraded, blocked, optional-fallback
	Reason      string   `json:"reason,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// ManifestCompileOptions are inputs for manifest compilation.
type ManifestCompileOptions struct {
	ProjectName     string                          `json:"project_name"`
	ProjectVersion  string                          `json:"project_version"`
	Entries         map[string]EntrySpec            `json:"entries"`
	Capabilities    map[string]CapabilityResolution `json:"capabilities"`
	Permissions     []string                        `json:"permissions"`
	HostPermissions []string                        `json:"host_permissions"`
}

// EntrySpec is a resolved entry for manifest compilation.
type EntrySpec struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// PackageOptions are inputs for artifact packaging.
type PackageOptions struct {
	SourceDir    string `json:"source_dir"`
	OutputDir    string `json:"output_dir"`
	ArtifactName string `json:"artifact_name"`
	Version      string `json:"version"`
	ManifestPath string `json:"manifest_path"`
}

// ArtifactRecord describes a packaged artifact.
type ArtifactRecord struct {
	Target              string `json:"target"`
	ArtifactType        string `json:"artifact_type"`
	FilePath            string `json:"file_path"`
	FileSize            int64  `json:"file_size"`
	SHA256              string `json:"sha256"`
	ManifestFingerprint string `json:"manifest_fingerprint"`
	BuildFingerprint    string `json:"build_fingerprint"`
	ProducedAt          string `json:"produced_at"`
}
