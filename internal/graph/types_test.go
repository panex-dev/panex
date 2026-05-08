package graph

import (
	"sync"
	"testing"
)

// TestComputeHash_StableAcrossSourceRoots is the H1 regression.
// Two graphs with identical content but different absolute SourceRoots
// must produce the same hash so a plan made on one machine does not
// always report drift on another.
func TestComputeHash_StableAcrossSourceRoots(t *testing.T) {
	mk := func(root string) *Graph {
		return &Graph{
			SchemaVersion: 1,
			Project:       ProjectIdentity{ID: "x", Name: "x"},
			SourceRoot:    root,
			Entries: map[string]Entry{
				"background": {Path: "bg.ts", Type: "service-worker"},
			},
			TargetsResolved: []string{"chrome"},
			Capabilities:    map[string]any{"tabs": true},
		}
	}

	a, err := mk("/home/alice/proj").ComputeHash()
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := mk("/Users/bob/work/proj").ComputeHash()
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if a != b {
		t.Errorf("hash should be SourceRoot-independent: %s != %s", a, b)
	}
}

// TestComputeHash_DoesNotMutateReceiver is the C5 regression.
// The previous implementation zeroed g.GraphHash and restored via defer,
// which is non-atomic and can leak intermediate state to concurrent readers.
func TestComputeHash_DoesNotMutateReceiver(t *testing.T) {
	g := &Graph{
		SchemaVersion: 1,
		Project:       ProjectIdentity{ID: "x", Name: "x"},
		GraphHash:     "sha256:preexisting",
	}
	original := g.GraphHash
	if _, err := g.ComputeHash(); err != nil {
		t.Fatalf("hash: %v", err)
	}
	if g.GraphHash != original {
		t.Errorf("ComputeHash mutated GraphHash: got %q want %q", g.GraphHash, original)
	}
}

// TestComputeHash_ConcurrentSafe verifies parallel ComputeHash calls all
// return the same value and produce no race detector trips. The previous
// implementation racing on g.GraphHash would fail with -race.
func TestComputeHash_ConcurrentSafe(t *testing.T) {
	g := &Graph{
		SchemaVersion:   1,
		Project:         ProjectIdentity{ID: "x", Name: "x"},
		Entries:         map[string]Entry{"bg": {Path: "bg.ts"}},
		TargetsResolved: []string{"chrome"},
		GraphHash:       "sha256:preexisting",
	}

	const workers = 32
	results := make([]string, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			h, err := g.ComputeHash()
			if err != nil {
				t.Errorf("worker %d: %v", idx, err)
				return
			}
			results[idx] = h
		}(i)
	}
	wg.Wait()

	for i := 1; i < workers; i++ {
		if results[i] != results[0] {
			t.Errorf("hash mismatch worker %d: %s != %s", i, results[i], results[0])
		}
	}
}

// TestComputeHash_ChangesWithContent guards the inverse: real content
// changes still flip the hash. Without this, omitting too many fields
// from hashView would silently weaken drift detection.
func TestComputeHash_ChangesWithContent(t *testing.T) {
	g := &Graph{
		SchemaVersion:   1,
		Project:         ProjectIdentity{ID: "x", Name: "x"},
		Entries:         map[string]Entry{"bg": {Path: "bg.ts"}},
		TargetsResolved: []string{"chrome"},
	}
	h1, _ := g.ComputeHash()
	g.Entries["popup"] = Entry{Path: "popup.html"}
	h2, _ := g.ComputeHash()
	if h1 == h2 {
		t.Errorf("hash should change when entries change")
	}
}

func TestRuntimeExtensionID(t *testing.T) {
	t.Run("prefers project id", func(t *testing.T) {
		g := &Graph{Project: ProjectIdentity{ID: "acme.popup", Name: "popup"}}
		if got := g.RuntimeExtensionID(); got != "acme.popup" {
			t.Fatalf("runtime extension id: got %q, want %q", got, "acme.popup")
		}
	})

	t.Run("falls back to project name", func(t *testing.T) {
		g := &Graph{Project: ProjectIdentity{Name: "popup"}}
		if got := g.RuntimeExtensionID(); got != "popup" {
			t.Fatalf("runtime extension id: got %q, want %q", got, "popup")
		}
	})

	t.Run("handles nil graph", func(t *testing.T) {
		var g *Graph
		if got := g.RuntimeExtensionID(); got != "" {
			t.Fatalf("runtime extension id: got %q, want empty", got)
		}
	})
}
