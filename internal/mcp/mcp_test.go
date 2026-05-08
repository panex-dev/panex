package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/fsmodel"
	"github.com/panex-dev/panex/internal/graph"
)

func TestInitialize(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocol: got %v", result["protocolVersion"])
	}

	info := result["serverInfo"].(map[string]any)
	if info["name"] != "panex" {
		t.Errorf("name: got %v", info["name"])
	}
}

func TestToolsList(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]Tool)
	if len(tools) < 10 {
		t.Errorf("expected at least 10 tools, got %d", len(tools))
	}

	// Check required tools exist
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	required := []string{"inspect_project", "initialize_project", "plan_changes",
		"apply_changes", "verify_project", "doctor_project", "package_release"}
	for _, r := range required {
		if !names[r] {
			t.Errorf("missing required tool: %s", r)
		}
	}
}

func TestResourcesList(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/list",
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	resources := result["resources"].([]Resource)
	if len(resources) == 0 {
		t.Error("expected resources")
	}
}

func TestToolInspect(t *testing.T) {
	dir := t.TempDir()
	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "inspect_project", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	if result["isError"] != nil {
		t.Errorf("tool returned error: %v", result)
	}
}

func TestToolInit(t *testing.T) {
	dir := t.TempDir()
	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{
		"name":      "initialize_project",
		"arguments": map[string]any{"name": "test-ext", "targets": []string{"chrome"}},
	})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	// Verify .panex dir was created
	if _, err := os.Stat(filepath.Join(dir, ".panex")); err != nil {
		t.Error("expected .panex directory")
	}
}

func TestToolDoctor(t *testing.T) {
	dir := t.TempDir()
	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "doctor_project", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}
}

func TestToolVerify(t *testing.T) {
	dir := t.TempDir()
	setupProject(t, dir)

	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "verify_project", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}
}

func TestToolPlan(t *testing.T) {
	dir := t.TempDir()
	setupProject(t, dir)

	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "plan_changes", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	// Verify plan was saved
	planPath := filepath.Join(dir, ".panex", "current.plan.json")
	if _, err := os.Stat(planPath); err != nil {
		t.Error("expected plan to be saved")
	}
}

func TestResourceRead(t *testing.T) {
	dir := t.TempDir()
	setupProject(t, dir)

	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"uri": "panex://project/graph"})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "resources/read",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}
}

func TestResourceReadUnknown(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	params, _ := json.Marshal(map[string]any{"uri": "panex://unknown"})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "resources/read",
		Params:  params,
	})

	if resp.Error == nil {
		t.Error("expected error for unknown resource")
	}
}

func TestMethodNotFound(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "nonexistent",
	})

	if resp.Error == nil {
		t.Error("expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("code: got %d", resp.Error.Code)
	}
}

func TestRunLoop(t *testing.T) {
	dir := t.TempDir()

	req1, _ := json.Marshal(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	req2, _ := json.Marshal(Request{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	input := string(req1) + "\n" + string(req2) + "\n"

	var output bytes.Buffer
	srv := NewServerWithIO(dir, bytes.NewReader([]byte(input)), &output)

	ctx := context.Background()
	if err := srv.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Should have two responses
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Errorf("expected 2 responses, got %d", len(lines))
	}
}

func TestUnknownTool(t *testing.T) {
	srv := NewServerWithIO(t.TempDir(), nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "nonexistent_tool", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params:  params,
	})

	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Error("expected isError for unknown tool")
	}
}

func TestToolRepair(t *testing.T) {
	dir := t.TempDir()
	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "repair_failure", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	if result["isError"] != nil {
		t.Errorf("tool returned error: %v", result)
	}
}

func TestToolResume_NoRun(t *testing.T) {
	dir := t.TempDir()
	root, err := fsmodel.NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Init(); err != nil {
		t.Fatal(err)
	}

	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{"name": "resume_run", "arguments": map[string]any{}})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  params,
	})

	// Should return an error since there's no run to resume
	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Error("expected error for no run to resume")
	}
}

func TestToolStartDevSession(t *testing.T) {
	dir := t.TempDir()
	setupProject(t, dir)

	srv := NewServerWithIO(dir, nil, nil)

	params, _ := json.Marshal(map[string]any{
		"name":      "start_dev_session",
		"arguments": map[string]any{"target": "chrome"},
	})
	resp := srv.HandleSingleRequest(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	if result["isError"] != nil {
		t.Errorf("tool returned error: %v", result)
	}

	// Verify session dir was created
	sessions, err := os.ReadDir(filepath.Join(dir, ".panex", "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) == 0 {
		t.Error("expected session directory to be created")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".panex", "sessions", sessions[0].Name(), "session.json"))
	if err != nil {
		t.Fatalf("read session.json: %v", err)
	}

	var stored struct {
		ExtensionID string `json:"extension_id"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal session.json: %v", err)
	}
	if stored.ExtensionID != "test-ext" {
		t.Fatalf("extension id: got %q, want %q", stored.ExtensionID, "test-ext")
	}
}

// --- helpers ---

func setupProject(t *testing.T, dir string) {
	t.Helper()
	root, err := fsmodel.NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Init(); err != nil {
		t.Fatal(err)
	}

	g := &graph.Graph{
		SchemaVersion:   1,
		Project:         graph.ProjectIdentity{Name: "test-ext", ID: "test-ext"},
		TargetsResolved: []string{"chrome"},
		Entries: map[string]graph.Entry{
			"background": {Path: "background.js", Type: "service-worker"},
		},
		Capabilities: map[string]any{"tabs": true},
	}
	if err := graph.WriteToFile(g, root.ProjectGraphPath()); err != nil {
		t.Fatal(err)
	}
}
