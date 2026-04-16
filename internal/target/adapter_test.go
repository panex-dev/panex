package target

import "testing"

func TestDefaultRegistry_ContainsChrome(t *testing.T) {
	reg := DefaultRegistry()

	adapter, ok := reg.Get("chrome")
	if !ok {
		t.Fatal("DefaultRegistry should contain a chrome adapter")
	}
	if adapter.Name() != "chrome" {
		t.Errorf("expected adapter name chrome, got %s", adapter.Name())
	}

	all := reg.All()
	if len(all) != 1 {
		t.Errorf("expected 1 adapter in DefaultRegistry, got %d", len(all))
	}
	if _, ok := all["chrome"]; !ok {
		t.Error("All() should contain chrome")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	// Empty registry returns false
	_, ok := reg.Get("chrome")
	if ok {
		t.Error("empty registry should not contain chrome")
	}

	all := reg.All()
	if len(all) != 0 {
		t.Errorf("empty registry All() should return empty map, got %d entries", len(all))
	}

	// Register and retrieve
	reg.Register(NewChrome())

	adapter, ok := reg.Get("chrome")
	if !ok {
		t.Fatal("expected chrome adapter after Register")
	}
	if adapter.Name() != "chrome" {
		t.Errorf("expected name chrome, got %s", adapter.Name())
	}

	all = reg.All()
	if len(all) != 1 {
		t.Errorf("expected 1 adapter, got %d", len(all))
	}

	// All() returns a copy — mutating it should not affect the registry
	delete(all, "chrome")
	if _, ok := reg.Get("chrome"); !ok {
		t.Error("deleting from All() result should not affect registry")
	}
}
