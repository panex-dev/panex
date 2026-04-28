// Package mcp implements the Panex MCP (Model Context Protocol) server.
// It runs over stdio using JSON-RPC 2.0, exposing Panex operations as
// tools and project state as resources. Spec section 35.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/panex-dev/panex/internal/capability"
	"github.com/panex-dev/panex/internal/cli"
	"github.com/panex-dev/panex/internal/core"
	"github.com/panex-dev/panex/internal/doctor"
	"github.com/panex-dev/panex/internal/fsmodel"
	"github.com/panex-dev/panex/internal/graph"
	"github.com/panex-dev/panex/internal/inspector"
	"github.com/panex-dev/panex/internal/ledger"
	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/manifest"
	"github.com/panex-dev/panex/internal/plan"
	"github.com/panex-dev/panex/internal/session"
	"github.com/panex-dev/panex/internal/target"
	"github.com/panex-dev/panex/internal/verify"
)

// JSONRPC types

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Server is the MCP server.
type Server struct {
	projectDir string
	reader     io.Reader
	writer     io.Writer
	registry   *target.Registry
}

// NewServer creates a new MCP server.
func NewServer(projectDir string) *Server {
	return &Server{
		projectDir: projectDir,
		reader:     os.Stdin,
		writer:     os.Stdout,
		registry:   target.DefaultRegistry(),
	}
}

// NewServerWithIO creates a server with custom I/O (for testing).
func NewServerWithIO(projectDir string, reader io.Reader, writer io.Writer) *Server {
	return &Server{
		projectDir: projectDir,
		reader:     reader,
		writer:     writer,
		registry:   target.DefaultRegistry(),
	}
}

// Run starts the server loop, reading requests from stdin and writing responses to stdout.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB buffer

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeResponse(Response{
				JSONRPC: "2.0",
				Error:   &Error{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := s.handleRequest(ctx, req)
		s.writeResponse(resp)
	}

	return scanner.Err()
}

// HandleSingleRequest processes one request and returns the response (for testing).
func (s *Server) HandleSingleRequest(ctx context.Context, req Request) Response {
	return s.handleRequest(ctx, req)
}

func (s *Server) handleRequest(ctx context.Context, req Request) Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req Request) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "panex",
				"version": "0.1.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req Request) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": s.toolDefinitions(),
		},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: -32602, Message: "invalid_params: " + err.Error()},
			}
		}
	}

	result, err := s.executeTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": fmt.Sprintf("error: %v", err)},
				},
				"isError": true,
			},
		}
	}

	text, _ := json.MarshalIndent(result, "", "  ")
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(text)},
			},
		},
	}
}

func (s *Server) handleResourcesList(req Request) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"resources": s.resourceDefinitions(),
		},
	}
}

func (s *Server) handleResourcesRead(req Request) Response {
	var params struct {
		URI string `json:"uri"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: -32602, Message: "invalid_params: " + err.Error()},
			}
		}
	}

	content, err := s.readResource(params.URI)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: err.Error()},
		}
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"contents": []map[string]any{
				{"uri": params.URI, "mimeType": "application/json", "text": content},
			},
		},
	}
}

// --- tool definitions ---

func (s *Server) toolDefinitions() []Tool {
	obj := map[string]any{"type": "object", "properties": map[string]any{}}
	return []Tool{
		{Name: "inspect_project", Description: "Scan project directory and report findings", InputSchema: obj},
		{Name: "initialize_project", Description: "Initialize Panex state for the project", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string", "description": "Project name"},
				"targets": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Target platforms"},
			},
		}},
		{Name: "plan_changes", Description: "Compute proposed changes for the project", InputSchema: obj},
		{Name: "apply_changes", Description: "Apply a computed plan", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"force": map[string]any{"type": "boolean", "description": "Skip drift check"},
			},
		}},
		{Name: "verify_project", Description: "Run verification checks", InputSchema: obj},
		{Name: "test_project", Description: "Run project tests and verification", InputSchema: obj},
		{Name: "doctor_project", Description: "Diagnose project health", InputSchema: obj},
		{Name: "repair_failure", Description: "Run auto-repairs", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repairs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repair IDs to run"},
			},
		}},
		{Name: "package_release", Description: "Package extension artifacts", InputSchema: obj},
		{Name: "read_report", Description: "Read the latest run report", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Specific run ID"},
			},
		}},
		{Name: "resume_run", Description: "Resume a paused or failed run", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string", "description": "Run ID to resume"},
			},
		}},
		{Name: "start_dev_session", Description: "Start a dev session with browser", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{"type": "string", "description": "Target platform"},
			},
		}},
	}
}

// --- resource definitions ---

func (s *Server) resourceDefinitions() []Resource {
	return []Resource{
		{URI: "panex://project/graph", Name: "Project Graph", MimeType: "application/json"},
		{URI: "panex://project/config-lock", Name: "Config Lock", MimeType: "application/json"},
		{URI: "panex://environment", Name: "Environment", MimeType: "application/json"},
		{URI: "panex://runs/latest", Name: "Latest Run", MimeType: "application/json"},
	}
}

// --- tool execution ---

func (s *Server) executeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "inspect_project":
		return s.toolInspect(ctx)
	case "initialize_project":
		return s.toolInit(ctx, args)
	case "verify_project":
		return s.toolVerify(ctx)
	case "doctor_project":
		return s.toolDoctor(ctx)
	case "plan_changes":
		return s.toolPlan(ctx)
	case "apply_changes":
		return s.toolApply(ctx, args)
	case "package_release":
		return s.toolPackage(ctx)
	case "test_project":
		return s.toolTest(ctx)
	case "read_report":
		return s.toolReadReport(args)
	case "repair_failure":
		return s.toolRepair(ctx, args)
	case "resume_run":
		return s.toolResume(args)
	case "start_dev_session":
		return s.toolStartDevSession(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) toolInspect(_ context.Context) (any, error) {
	ins := inspector.New(s.projectDir)
	report, err := ins.Inspect()
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Server) toolInit(_ context.Context, args map[string]any) (any, error) {
	root, err := fsmodel.NewRoot(s.projectDir)
	if err != nil {
		return nil, fmt.Errorf("init root: %w", err)
	}

	if root.IsInitialized() {
		return cli.Output{Status: "ok", Command: "init", Summary: "already initialized"}, nil
	}

	if err := root.Init(); err != nil {
		return nil, fmt.Errorf("init state: %w", err)
	}

	ins := inspector.New(s.projectDir)
	report, _ := ins.Inspect()

	builder := graph.NewBuilder(s.projectDir)
	g, err := builder.BuildFromInspection(report)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	if name, ok := args["name"].(string); ok && name != "" {
		g.Project.Name = name
		g.Project.ID = name
	}

	if targets, ok := args["targets"].([]any); ok {
		g.TargetsResolved = nil
		for _, t := range targets {
			if ts, ok := t.(string); ok {
				g.TargetsResolved = append(g.TargetsResolved, ts)
			}
		}
	}

	if err := graph.WriteToFile(g, root.ProjectGraphPath()); err != nil {
		return nil, fmt.Errorf("write graph: %w", err)
	}

	return cli.Output{
		Status:  "ok",
		Command: "init",
		Summary: fmt.Sprintf("initialized project %s", g.Project.Name),
		Data:    g,
	}, nil
}

func (s *Server) toolVerify(_ context.Context) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	adapters := s.registry.All()
	matrix := s.resolveCapabilities(g, adapters)

	result := verify.Verify(verify.Input{
		Graph:  g,
		Matrix: matrix,
	})
	return result, nil
}

func (s *Server) toolDoctor(_ context.Context) (any, error) {
	report := doctor.Run(doctor.Options{ProjectDir: s.projectDir})
	return report, nil
}

func (s *Server) toolPlan(_ context.Context) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	adapters := s.registry.All()
	matrix := s.resolveCapabilities(g, adapters)
	manifestResult := manifest.Compile(manifest.CompileInput{
		Graph: g, Matrix: matrix, Adapters: adapters,
	})

	p, err := plan.ComputePlan(plan.PlanInput{
		ProjectDir:     s.projectDir,
		Graph:          g,
		ManifestResult: manifestResult,
	})
	if err != nil {
		return nil, err
	}

	planPath := filepath.Join(s.projectDir, ".panex", "current.plan.json")
	_ = plan.WritePlan(p, planPath)

	return p, nil
}

func (s *Server) toolApply(ctx context.Context, args map[string]any) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	planPath := filepath.Join(s.projectDir, ".panex", "current.plan.json")
	p, err := plan.ReadPlan(planPath)
	if err != nil {
		return nil, fmt.Errorf("no plan found: %w", err)
	}

	force, _ := args["force"].(bool)

	orc := core.NewOrchestrator(s.projectDir, s.registry)
	result, err := orc.Apply(ctx, core.ApplyInput{
		Graph: g,
		Plan:  p,
		Force: force,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Server) toolPackage(_ context.Context) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	adapters := s.registry.All()
	var results []any

	for _, tgt := range g.TargetsResolved {
		adapter, ok := adapters[tgt]
		if !ok {
			continue
		}
		record, adapterResult := adapter.PackageArtifact(context.Background(), target.PackageOptions{
			SourceDir:    s.projectDir,
			OutputDir:    filepath.Join(s.projectDir, ".panex", "artifacts", tgt),
			ArtifactName: g.Project.Name,
			Version:      g.Project.Version,
		})
		if adapterResult.Outcome == target.Success {
			results = append(results, record)
		} else {
			results = append(results, adapterResult)
		}
	}

	return results, nil
}

func (s *Server) toolTest(_ context.Context) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	adapters := s.registry.All()
	matrix := s.resolveCapabilities(g, adapters)

	verifyResult := verify.Verify(verify.Input{
		Graph:  g,
		Matrix: matrix,
	})
	doctorReport := doctor.Run(doctor.Options{ProjectDir: s.projectDir})

	return map[string]any{
		"verify": verifyResult,
		"doctor": doctorReport,
	}, nil
}

func (s *Server) toolReadReport(args map[string]any) (any, error) {
	runsDir := filepath.Join(s.projectDir, ".panex", "runs")

	if runID, ok := args["run_id"].(string); ok && runID != "" {
		data, err := os.ReadFile(filepath.Join(runsDir, runID, "run.json"))
		if err != nil {
			return nil, fmt.Errorf("run not found: %w", err)
		}
		var run any
		_ = json.Unmarshal(data, &run)
		return run, nil
	}

	// Find latest run
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("no runs found")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no runs found")
	}

	latest := entries[len(entries)-1]
	data, err := os.ReadFile(filepath.Join(runsDir, latest.Name(), "run.json"))
	if err != nil {
		return nil, err
	}
	var run any
	_ = json.Unmarshal(data, &run)
	return run, nil
}

func (s *Server) toolRepair(_ context.Context, args map[string]any) (any, error) {
	report := doctor.Run(doctor.Options{
		ProjectDir: s.projectDir,
		Fix:        true,
	})
	return report, nil
}

func (s *Server) toolResume(args map[string]any) (any, error) {
	root, err := fsmodel.NewRoot(s.projectDir)
	if err != nil {
		return nil, err
	}

	runID, _ := args["run_id"].(string)
	if runID == "" {
		state, err := root.ReadState()
		if err != nil || state.LatestRunID == "" {
			return nil, fmt.Errorf("no run to resume")
		}
		runID = state.LatestRunID
	}

	run, err := ledger.ReadFromDir(root.RunDir(runID))
	if err != nil {
		return nil, fmt.Errorf("cannot read run: %w", err)
	}

	if !run.Resumable {
		return nil, fmt.Errorf("run %s is not resumable (status: %s)", runID, run.Status)
	}

	if err := run.Transition(ledger.StatusRunning); err != nil {
		return nil, err
	}
	_ = run.Transition(ledger.StatusSucceeded)
	_ = run.WriteToDir(root.RunDir(runID))

	return cli.Output{
		Status:  "ok",
		Command: "resume",
		RunID:   runID,
		Summary: fmt.Sprintf("resumed run %s", runID),
		Data:    run,
	}, nil
}

func (s *Server) toolStartDevSession(_ context.Context, args map[string]any) (any, error) {
	g, err := s.loadGraph()
	if err != nil {
		return nil, err
	}

	root, err := fsmodel.NewRoot(s.projectDir)
	if err != nil {
		return nil, err
	}

	targetName, _ := args["target"].(string)
	if targetName == "" {
		if len(g.TargetsResolved) > 0 {
			targetName = g.TargetsResolved[0]
		} else {
			targetName = "chrome"
		}
	}

	mgr := lock.NewManager(root.StateRoot())
	adapter, _ := s.registry.Get(targetName)

	var allowed []string
	for k := range g.Capabilities {
		allowed = append(allowed, k)
	}

	sess, err := session.New(session.Options{
		ProjectDir:          s.projectDir,
		Target:              targetName,
		AllowedCapabilities: allowed,
		LockManager:         mgr,
		Adapter:             adapter,
	})
	if err != nil {
		return nil, err
	}

	_ = sess.WriteToDir(root.SessionDir(sess.SessionID))

	return cli.Output{
		Status:  "ok",
		Command: "dev",
		Summary: fmt.Sprintf("session %s provisioned for %s", sess.SessionID, targetName),
		Data:    sess.Info(),
	}, nil
}

// --- resource reading ---

func (s *Server) readResource(uri string) (string, error) {
	root, err := fsmodel.NewRoot(s.projectDir)
	if err != nil {
		return "", fmt.Errorf("invalid project dir: %w", err)
	}

	switch uri {
	case "panex://project/graph":
		data, err := os.ReadFile(root.ProjectGraphPath())
		if err != nil {
			return "", fmt.Errorf("graph not found")
		}
		return string(data), nil
	case "panex://project/config-lock":
		data, err := os.ReadFile(root.ConfigLockPath())
		if err != nil {
			return "", fmt.Errorf("config lock not found")
		}
		return string(data), nil
	case "panex://environment":
		data, err := os.ReadFile(root.EnvironmentPath())
		if err != nil {
			return "", fmt.Errorf("environment not found")
		}
		return string(data), nil
	default:
		if strings.HasPrefix(uri, "panex://runs/") {
			runID := strings.TrimPrefix(uri, "panex://runs/")
			data, err := os.ReadFile(filepath.Join(s.projectDir, ".panex", "runs", runID, "run.json"))
			if err != nil {
				return "", fmt.Errorf("run not found: %s", runID)
			}
			return string(data), nil
		}
		return "", fmt.Errorf("unknown resource: %s", uri)
	}
}

// --- helpers ---

func (s *Server) loadGraph() (*graph.Graph, error) {
	return cli.LoadProjectGraph(s.projectDir)
}

func (s *Server) resolveCapabilities(g *graph.Graph, adapters map[string]target.Adapter) *capability.TargetMatrix {
	matrix, _ := capability.Compile(capability.CompilerInput{
		Capabilities: g.Capabilities,
		Targets:      g.TargetsResolved,
		Adapters:     adapters,
	})
	return matrix
}

func (s *Server) writeResponse(resp Response) {
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	_, _ = s.writer.Write(data)
}
