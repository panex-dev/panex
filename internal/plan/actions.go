// Package plan — polymorphic action types.
//
// Every plannable mutation implements Action. The previous design held a
// flat {Type, Path, Description} record and dispatched on Type in apply,
// which silently corrupted multi-target manifests (one Path field could
// not address per-target outputs) and left rollback unimplemented.
//
// The interface gives each action ownership of its own data and undo
// logic, and lets Apply roll back completed steps in reverse on failure.
package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/panex-dev/panex/internal/manifest"
)

// Action is the interface every plannable mutation implements.
type Action interface {
	Kind() string               // discriminator used for serialization and step naming
	Describe() string           // human-readable description
	Risk() string               // "safe" | "low" | "medium" | "high"
	Reversible() bool           // whether Rollback is implemented
	Execute(ExecContext) error  // perform the mutation
	Rollback(ExecContext) error // undo (no-op if !Reversible)
}

// ExecContext carries dependencies an action needs at execution time.
type ExecContext struct {
	ProjectDir string
}

// ActionList is a serializable slice of polymorphic actions.
//
// We define a named slice type so JSON marshal/unmarshal can dispatch
// through the registry — interfaces alone cannot round-trip without help.
type ActionList []Action

// actionWire is the on-disk envelope for a single Action.
type actionWire struct {
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Risk        string          `json:"risk"`
	Reversible  bool            `json:"reversible"`
	Spec        json.RawMessage `json:"spec,omitempty"`
}

// MarshalJSON emits each action as {type, description, risk, reversible, spec}.
func (al ActionList) MarshalJSON() ([]byte, error) {
	wires := make([]actionWire, len(al))
	for i, a := range al {
		spec, err := json.Marshal(a)
		if err != nil {
			return nil, fmt.Errorf("marshal action spec %q: %w", a.Kind(), err)
		}
		wires[i] = actionWire{
			Type:        a.Kind(),
			Description: a.Describe(),
			Risk:        a.Risk(),
			Reversible:  a.Reversible(),
			Spec:        spec,
		}
	}
	return json.Marshal(wires)
}

// UnmarshalJSON dispatches each wire entry through the action registry.
func (al *ActionList) UnmarshalJSON(data []byte) error {
	var wires []actionWire
	if err := json.Unmarshal(data, &wires); err != nil {
		return err
	}
	out := make([]Action, len(wires))
	for i, w := range wires {
		factory, ok := actionRegistry[w.Type]
		if !ok {
			return fmt.Errorf("unknown action type %q", w.Type)
		}
		body := factory()
		if len(w.Spec) > 0 {
			if err := json.Unmarshal(w.Spec, body); err != nil {
				return fmt.Errorf("unmarshal action spec %q: %w", w.Type, err)
			}
		}
		out[i] = body
	}
	*al = out
	return nil
}

// actionRegistry maps a Kind() to a factory that returns an empty instance
// for unmarshaling. Every Action implementation must register here.
var actionRegistry = map[string]func() Action{
	"generate_manifest": func() Action { return &GenerateManifestAction{} },
}

// --- GenerateManifestAction ---

// GenerateManifestAction writes a single target's manifest.json to disk.
//
// One action per target. The previous flat-Action design had a single Path
// per Action and looped over all targets inside applyGenerateManifest,
// writing every target's manifest to that one Path — last write wins.
// Holding Target/Path/Manifest per-action eliminates that ambiguity.
type GenerateManifestAction struct {
	Target   string         `json:"target"`
	Path     string         `json:"path"`
	Manifest map[string]any `json:"manifest"`
}

func (g *GenerateManifestAction) Kind() string     { return "generate_manifest" }
func (g *GenerateManifestAction) Risk() string     { return "safe" }
func (g *GenerateManifestAction) Reversible() bool { return true }

func (g *GenerateManifestAction) Describe() string {
	return fmt.Sprintf("generate manifest.json for %s", g.Target)
}

func (g *GenerateManifestAction) Execute(_ ExecContext) error {
	if g.Path == "" {
		return errors.New("generate_manifest: empty path")
	}
	if g.Manifest == nil {
		return errors.New("generate_manifest: nil manifest")
	}
	if err := os.MkdirAll(filepath.Dir(g.Path), 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", filepath.Dir(g.Path), err)
	}
	return manifest.WriteManifest(g.Manifest, g.Path)
}

func (g *GenerateManifestAction) Rollback(_ ExecContext) error {
	if g.Path == "" {
		return nil
	}
	if err := os.Remove(g.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", g.Path, err)
	}
	return nil
}
