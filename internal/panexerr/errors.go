// Package panexerr defines the canonical error taxonomy for Panex.
// All components must classify errors using these types. This is the
// error contract from spec section 29.
package panexerr

import "fmt"

// Category is a stable error classification.
type Category string

const (
	CatEnvironment  Category = "environment"
	CatConfig       Category = "config"
	CatInspection   Category = "inspection"
	CatGraph        Category = "graph"
	CatCapability   Category = "capability"
	CatManifest     Category = "manifest"
	CatPolicy       Category = "policy"
	CatDependency   Category = "dependency"
	CatBundler      Category = "bundler"
	CatRuntime      Category = "runtime"
	CatLifecycle    Category = "lifecycle"
	CatTrace        Category = "trace"
	CatVerify       Category = "verify"
	CatPackage      Category = "package"
	CatPublish      Category = "publish"
	CatInternal     Category = "internal"
)

// Error is a structured, machine-readable Panex error.
type Error struct {
	Code       string   `json:"code"`
	Category   Category `json:"category"`
	Message    string   `json:"message"`
	Component  string   `json:"component"`
	SafeRetry  bool     `json:"safe_to_retry"`
	Repairable bool     `json:"repairable"`
	PolicyFlag bool     `json:"policy_related"`
	Resumable  bool     `json:"resumable"`
	Recipes    []string `json:"suggested_recipes,omitempty"`
	Cause      error    `json:"-"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s/%s] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Category, e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new panex error.
func New(code string, cat Category, component, message string) *Error {
	return &Error{
		Code:      code,
		Category:  cat,
		Message:   message,
		Component: component,
	}
}

// Wrap creates a panex error wrapping an underlying cause.
func Wrap(code string, cat Category, component, message string, cause error) *Error {
	return &Error{
		Code:      code,
		Category:  cat,
		Message:   message,
		Component: component,
		Cause:     cause,
	}
}

// --- Convenience constructors for common error patterns ---

func ConfigInvalid(component, message string) *Error {
	return New("CONFIG_INVALID", CatConfig, component, message)
}

func ConfigMissing(component, message string) *Error {
	return New("CONFIG_MISSING", CatConfig, component, message)
}

func EnvironmentMissing(component, message string) *Error {
	e := New("ENVIRONMENT_MISSING", CatEnvironment, component, message)
	e.SafeRetry = true
	return e
}

func InspectionAmbiguity(component, message string) *Error {
	e := New("INSPECTION_AMBIGUITY", CatInspection, component, message)
	e.Repairable = true
	return e
}

func InspectionConflict(component, message string) *Error {
	return New("INSPECTION_CONFLICT", CatInspection, component, message)
}

func CapabilityBlocked(component, target, capability, message string) *Error {
	return New("CAPABILITY_BLOCKED_ON_TARGET", CatCapability, component,
		fmt.Sprintf("capability %q blocked on target %q: %s", capability, target, message))
}

func ManifestInvalid(component, message string) *Error {
	e := New("MANIFEST_INVALID", CatManifest, component, message)
	e.Repairable = true
	e.Recipes = []string{"regenerate_manifest"}
	return e
}

func PolicyDenied(component, rule, action string) *Error {
	e := New("POLICY_DENIED", CatPolicy, component,
		fmt.Sprintf("policy rule %q denied action %q", rule, action))
	e.PolicyFlag = true
	return e
}

func PlanDrift(component string) *Error {
	return New("PLAN_DRIFT_DETECTED", CatGraph, component,
		"project files changed after plan was computed")
}

func DependencyMissing(component, dep string) *Error {
	e := New("DEPENDENCY_MISSING", CatDependency, component,
		fmt.Sprintf("required dependency not found: %s", dep))
	e.Repairable = true
	e.Recipes = []string{"install_dependency"}
	return e
}

func InternalFault(component, message string, cause error) *Error {
	return Wrap("INTERNAL_FAULT", CatInternal, component, message, cause)
}

func PermissionExpansionOutsideCompiler(component, message string) *Error {
	return New("PERMISSION_EXPANSION_OUTSIDE_CAPABILITY_COMPILER", CatManifest, component, message)
}
