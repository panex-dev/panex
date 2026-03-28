// Package inspector scans a project directory and produces a partial
// project graph with confidence-scored findings. The inspector never
// mutates the project. This is the contract from spec section 14.
package inspector

// Classification describes how a finding was obtained.
type Classification string

const (
	Declared    Classification = "declared"
	Detected    Classification = "detected"
	Inferred    Classification = "inferred"
	Conflicting Classification = "conflicting"
	Missing     Classification = "missing"
)

// Finding is a single inspector observation with provenance.
type Finding[T any] struct {
	Value          T              `json:"value"`
	Source         string         `json:"source"`
	Confidence     float64        `json:"confidence"`
	Classification Classification `json:"classification"`
}

// Report is the complete output of an inspection run.
type Report struct {
	Framework      *Finding[string]          `json:"framework"`
	Bundler        *Finding[string]          `json:"bundler"`
	Language       *Finding[string]          `json:"language"`
	PackageManager *Finding[string]          `json:"package_manager"`
	WorkspaceType  *Finding[string]          `json:"workspace_type"`
	Entrypoints    map[string]EntryCandidate `json:"entrypoints"`
	Targets        []Finding[string]         `json:"targets_detected"`
	Missing        []string                  `json:"missing_requirements"`
	Conflicts      []Conflict                `json:"conflicts"`
	Recommended    []string                  `json:"recommended_actions"`
}

// EntryCandidate is a detected extension entrypoint.
type EntryCandidate struct {
	Path           string         `json:"path"`
	Type           string         `json:"type"`
	Source         string         `json:"source"`
	Confidence     float64        `json:"confidence"`
	Classification Classification `json:"classification"`
}

// Conflict represents conflicting inspector findings.
type Conflict struct {
	Field    string   `json:"field"`
	Values   []string `json:"values"`
	Sources  []string `json:"sources"`
	Message  string   `json:"message"`
}
