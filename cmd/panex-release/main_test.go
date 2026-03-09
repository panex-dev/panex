package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunRejectsMissingVersion(t *testing.T) {
	var out bytes.Buffer
	err := run(nil, &out)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPackagesDeterministicArchiveForCurrentTarget(t *testing.T) {
	target := runtime.GOOS + "/" + runtime.GOARCH

	firstDir := t.TempDir()
	var firstOut bytes.Buffer
	err := run([]string{"--version", "v0.0.1-test", "--targets", target, "--out-dir", firstDir}, &firstOut)
	if err != nil {
		t.Fatalf("first run() returned error: %v", err)
	}

	secondDir := t.TempDir()
	var secondOut bytes.Buffer
	err = run([]string{"--version", "v0.0.1-test", "--targets", target, "--out-dir", secondDir}, &secondOut)
	if err != nil {
		t.Fatalf("second run() returned error: %v", err)
	}

	firstArchivePath := strings.TrimSpace(strings.TrimPrefix(firstOut.String(), "wrote "))
	secondArchivePath := strings.TrimSpace(strings.TrimPrefix(secondOut.String(), "wrote "))
	firstBytes, err := os.ReadFile(firstArchivePath)
	if err != nil {
		t.Fatalf("read first archive: %v", err)
	}
	secondBytes, err := os.ReadFile(secondArchivePath)
	if err != nil {
		t.Fatalf("read second archive: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("expected reproducible archive bytes across repeated runs")
	}
	if filepath.Ext(firstArchivePath) == "" {
		t.Fatalf("expected archive extension in %q", firstArchivePath)
	}
}
