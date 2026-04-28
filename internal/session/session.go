// Package session implements the dev session controller. A session
// coordinates browser launch, extension loading, file watching,
// rebuilding, and reload signaling. Spec sections 23-24.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/target"
)

// State represents the session lifecycle state.
type State string

const (
	Provisioned State = "provisioned"
	Launching   State = "launching"
	Attached    State = "attached"
	Active      State = "active"
	Detached    State = "detached"
	Terminated  State = "terminated"
	Failed      State = "failed"
)

// Session is a managed dev runtime session.
type Session struct {
	mu sync.Mutex

	SessionID           string   `json:"session_id"`
	ProjectDir          string   `json:"project_dir"`
	Target              string   `json:"target"`
	State               State    `json:"state"`
	BrowserPID          int      `json:"browser_pid,omitempty"`
	ProfileDir          string   `json:"profile_dir"`
	ExtensionDir        string   `json:"extension_dir"`
	DaemonPort          int      `json:"daemon_port"`
	Token               string   `json:"ephemeral_token"`
	AllowedCapabilities []string `json:"allowed_capabilities"`
	CreatedAt           string   `json:"created_at"`
	AttachedAt          string   `json:"attached_at,omitempty"`
	TerminatedAt        string   `json:"terminated_at,omitempty"`
	Error               string   `json:"error,omitempty"`

	browserCmd  *exec.Cmd
	cancelFunc  context.CancelFunc
	lockManager *lock.Manager
	sessionLock *lock.Lock
	adapter     target.Adapter
}

// Options configures a new session.
type Options struct {
	ProjectDir          string
	Target              string
	ExtensionDir        string // built extension directory
	DaemonPort          int
	ChromeBinary        string         // path to Chrome binary (auto-detect if empty)
	AllowedCapabilities []string       // capabilities granted to this session (C3)
	LockManager         *lock.Manager
	Adapter             target.Adapter // target adapter for environment inspection (H3)
}

// New creates a new session in provisioned state.
func New(opts Options) (*Session, error) {
	if opts.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir required")
	}
	if opts.Target == "" {
		opts.Target = "chrome"
	}

	sid, err := newSessionID()
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}

	profileDir := filepath.Join(opts.ProjectDir, ".panex", "cache", "browser-profiles", sid)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}

	s := &Session{
		SessionID:           sid,
		ProjectDir:          opts.ProjectDir,
		Target:              opts.Target,
		State:               Provisioned,
		ProfileDir:          profileDir,
		ExtensionDir:        opts.ExtensionDir,
		DaemonPort:          opts.DaemonPort,
		Token:               token,
		AllowedCapabilities: opts.AllowedCapabilities,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		lockManager:         opts.LockManager,
		adapter:             opts.Adapter,
	}

	return s, nil
}

// Launch starts the browser with the extension loaded.
func (s *Session) Launch(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != Provisioned {
		return fmt.Errorf("cannot launch from state %s", s.State)
	}

	// Acquire session lock
	if s.lockManager != nil {
		var err error
		s.sessionLock, err = s.lockManager.Acquire(lock.DevSession, "dev:"+s.SessionID, "cli")
		if err != nil {
			s.State = Failed
			s.Error = err.Error()
			return fmt.Errorf("session lock: %w", err)
		}
	}

	s.State = Launching

	if s.adapter == nil {
		s.State = Failed
		s.Error = "no target adapter provided"
		return fmt.Errorf("no target adapter")
	}

	info, res := s.adapter.InspectEnvironment(ctx)
	if res.Outcome != target.Success || !info.Launchable {
		s.State = Failed
		s.Error = res.Reason
		return fmt.Errorf("target environment: %s", res.Reason)
	}
	binary := info.BinaryPath

	args := s.buildArgs()

	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		s.State = Failed
		s.Error = err.Error()
		cancel()
		return fmt.Errorf("launch browser: %w", err)
	}

	s.browserCmd = cmd
	s.BrowserPID = cmd.Process.Pid
	s.State = Active
	s.AttachedAt = time.Now().UTC().Format(time.RFC3339Nano)

	return nil
}

// Terminate stops the browser and cleans up.
func (s *Session) Terminate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State == Terminated {
		return nil
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	if s.browserCmd != nil && s.browserCmd.Process != nil {
		_ = s.browserCmd.Process.Kill()
		_ = s.browserCmd.Wait()
	}

	if s.sessionLock != nil && s.lockManager != nil {
		_ = s.lockManager.Release(s.sessionLock)
		s.sessionLock = nil
	}

	s.State = Terminated
	s.TerminatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	return nil
}

// GetState returns the current session state.
func (s *Session) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

// Info returns a serializable snapshot of the session.
func (s *Session) Info() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"session_id":    s.SessionID,
		"target":        s.Target,
		"state":         s.State,
		"browser_pid":   s.BrowserPID,
		"profile_dir":   s.ProfileDir,
		"extension_dir": s.ExtensionDir,
		"daemon_port":   s.DaemonPort,
		"created_at":    s.CreatedAt,
		"attached_at":   s.AttachedAt,
	}
}

// WriteToDir persists session metadata.
func (s *Session) WriteToDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "session.json"), data, 0o644)
}

// HandshakePayload returns the expected handshake from the bridge.
type HandshakePayload struct {
	ProtocolVersion      int            `json:"protocol_version"`
	SessionID            string         `json:"session_id"`
	ProjectID            string         `json:"project_id"`
	EphemeralToken       string         `json:"ephemeral_token"`
	Surface              string         `json:"surface"`
	SurfaceContext       map[string]any `json:"surface_context"`
	BuildFingerprint     string         `json:"build_fingerprint"`
	DeclaredCapabilities []string       `json:"declared_capabilities"`
}

// HandshakeReply is the daemon's response.
type HandshakeReply struct {
	Status                  string         `json:"status"` // "accepted", "rejected_*"
	SessionStatus           string         `json:"session_status"`
	AcceptedProtocolVersion int            `json:"accepted_protocol_version"`
	AllowedCapabilities     []string       `json:"allowed_capabilities"`
	DeniedCapabilities      []string       `json:"denied_capabilities"`
	TraceSettings           map[string]any `json:"trace_settings"`
	LogSettings             map[string]any `json:"log_settings"`
	FingerprintMatch        bool           `json:"fingerprint_match"`
	Reason                  string         `json:"reason,omitempty"`
	SuggestedAction         string         `json:"suggested_action,omitempty"`
}

// ValidateHandshake checks an incoming handshake against this session.
func (s *Session) ValidateHandshake(payload HandshakePayload) HandshakeReply {
	s.mu.Lock()
	defer s.mu.Unlock()

	if payload.EphemeralToken != s.Token {
		return HandshakeReply{
			Status:          "rejected_token",
			Reason:          "ephemeral token invalid",
			SuggestedAction: "restart dev session",
		}
	}

	if payload.SessionID != s.SessionID {
		return HandshakeReply{
			Status:          "rejected_session",
			Reason:          "session ID unknown",
			SuggestedAction: "restart dev session",
		}
	}

	if payload.ProtocolVersion != 1 {
		return HandshakeReply{
			Status:          "rejected_version",
			Reason:          fmt.Sprintf("unsupported protocol version %d", payload.ProtocolVersion),
			SuggestedAction: "update bridge",
		}
	}

	// Filter capabilities (C3)
	granted := []string{}
	denied := []string{}
	allowlist := make(map[string]bool)
	for _, c := range s.AllowedCapabilities {
		allowlist[c] = true
	}

	for _, req := range payload.DeclaredCapabilities {
		if allowlist[req] {
			granted = append(granted, req)
		} else {
			denied = append(denied, req)
		}
	}

	return HandshakeReply{
		Status:                  "accepted",
		SessionStatus:           string(s.State),
		AcceptedProtocolVersion: 1,
		AllowedCapabilities:     granted,
		DeniedCapabilities:      denied,
		TraceSettings:           map[string]any{"enabled": false, "level": "standard"},
		LogSettings:             map[string]any{"level": "info", "forward_console": true},
		FingerprintMatch:        true,
	}
}

// --- helpers ---

func (s *Session) buildArgs() []string {
	args := []string{
		"--user-data-dir=" + s.ProfileDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if s.ExtensionDir != "" {
		args = append(args, "--load-extension="+s.ExtensionDir)
	}
	return args
}

func newSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return "ses_" + hex.EncodeToString(b), nil
}

func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "tok_" + hex.EncodeToString(b), nil
}
