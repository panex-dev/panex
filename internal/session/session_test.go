package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/lock"
	"github.com/panex-dev/panex/internal/target"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := New(Options{
		ProjectDir:   dir,
		Target:       "chrome",
		ExtensionDir: filepath.Join(dir, "dist"),
		DaemonPort:   9222,
		Adapter:      target.NewChrome(),
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	if s.State != Provisioned {
		t.Errorf("state: got %s", s.State)
	}
	if s.SessionID == "" {
		t.Error("expected session ID")
	}
	if s.Token == "" {
		t.Error("expected token")
	}
	if s.Target != "chrome" {
		t.Errorf("target: got %s", s.Target)
	}
}

func TestNew_DefaultTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := New(Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if s.Target != "chrome" {
		t.Errorf("default target: got %s", s.Target)
	}
}

func TestNew_RequiresProjectDir(t *testing.T) {
	_, err := New(Options{})
	if err == nil {
		t.Error("expected error for empty project dir")
	}
}

func TestSessionInfo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir, DaemonPort: 9222})
	info := s.Info()

	if info["session_id"] != s.SessionID {
		t.Error("info session_id mismatch")
	}
	if info["daemon_port"] != 9222 {
		t.Errorf("daemon_port: got %v", info["daemon_port"])
	}
}

func TestValidateHandshake_Accepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{
		ProjectDir:          dir,
		AllowedCapabilities: []string{"runtime_logs", "hot_reload_ack"},
		Adapter:             target.NewChrome(),
	})

	reply := s.ValidateHandshake(HandshakePayload{
		ProtocolVersion:      1,
		SessionID:            s.SessionID,
		EphemeralToken:       s.Token,
		Surface:              "background",
		DeclaredCapabilities: []string{"runtime_logs", "hot_reload_ack", "evil_hack"},
	})

	if reply.Status != "accepted" {
		t.Errorf("status: got %s, reason: %s", reply.Status, reply.Reason)
	}
	if reply.AcceptedProtocolVersion != 1 {
		t.Errorf("protocol version: got %d", reply.AcceptedProtocolVersion)
	}
	if len(reply.AllowedCapabilities) != 2 {
		t.Errorf("allowed capabilities: got %v", reply.AllowedCapabilities)
	}
	if len(reply.DeniedCapabilities) != 1 || reply.DeniedCapabilities[0] != "evil_hack" {
		t.Errorf("denied capabilities: got %v", reply.DeniedCapabilities)
	}
}

func TestValidateHandshake_RejectedToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})

	reply := s.ValidateHandshake(HandshakePayload{
		ProtocolVersion: 1,
		SessionID:       s.SessionID,
		EphemeralToken:  "wrong_token",
	})

	if reply.Status != "rejected_token" {
		t.Errorf("expected rejected_token, got %s", reply.Status)
	}
}

func TestValidateHandshake_RejectedSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})

	reply := s.ValidateHandshake(HandshakePayload{
		ProtocolVersion: 1,
		SessionID:       "ses_unknown",
		EphemeralToken:  s.Token,
	})

	if reply.Status != "rejected_session" {
		t.Errorf("expected rejected_session, got %s", reply.Status)
	}
}

func TestValidateHandshake_RejectedVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})

	reply := s.ValidateHandshake(HandshakePayload{
		ProtocolVersion: 99,
		SessionID:       s.SessionID,
		EphemeralToken:  s.Token,
	})

	if reply.Status != "rejected_version" {
		t.Errorf("expected rejected_version, got %s", reply.Status)
	}
}

func TestTerminate_Provisioned(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})
	if err := s.Terminate(); err != nil {
		t.Errorf("terminate: %v", err)
	}
	if s.GetState() != Terminated {
		t.Errorf("state: got %s", s.GetState())
	}
}

func TestTerminate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})
	_ = s.Terminate()
	if err := s.Terminate(); err != nil {
		t.Errorf("double terminate: %v", err)
	}
}

func TestWriteToDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".panex", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, _ := New(Options{ProjectDir: dir})
	sessDir := filepath.Join(dir, ".panex", "sessions", s.SessionID)

	if err := s.WriteToDir(sessDir); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(sessDir, "session.json")); err != nil {
		t.Error("expected session.json")
	}
}

func TestSessionWithLock(t *testing.T) {
	dir := t.TempDir()
	panexDir := filepath.Join(dir, ".panex")
	if err := os.MkdirAll(filepath.Join(panexDir, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(panexDir, "locks"), 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := lock.NewManager(panexDir)

	s, _ := New(Options{
		ProjectDir:  dir,
		LockManager: mgr,
		Adapter:     target.NewChrome(),
	})

	// Session is provisioned, not launched — no lock acquired yet
	held, _ := mgr.IsHeld(lock.DevSession)
	if held {
		t.Error("lock should not be held before launch")
	}

	// Terminate releases session
	_ = s.Terminate()
}
