package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panex-dev/panex/internal/release"
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

	firstPaths := writtenPaths(t, firstOut.String())
	secondPaths := writtenPaths(t, secondOut.String())
	if len(firstPaths) != 2 || len(secondPaths) != 2 {
		t.Fatalf("unexpected written file count: got %d and %d", len(firstPaths), len(secondPaths))
	}
	firstArchivePath := firstPaths[0]
	secondArchivePath := secondPaths[0]
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

	firstChecksumBytes, err := os.ReadFile(firstPaths[1])
	if err != nil {
		t.Fatalf("read first checksum manifest: %v", err)
	}
	secondChecksumBytes, err := os.ReadFile(secondPaths[1])
	if err != nil {
		t.Fatalf("read second checksum manifest: %v", err)
	}
	if !bytes.Equal(firstChecksumBytes, secondChecksumBytes) {
		t.Fatal("expected reproducible checksum manifest bytes across repeated runs")
	}
	wantLine := release.SHA256Hex(firstBytes) + "  " + filepath.Base(firstArchivePath)
	if !strings.Contains(string(firstChecksumBytes), wantLine) {
		t.Fatalf("checksum manifest missing archive digest line %q:\n%s", wantLine, string(firstChecksumBytes))
	}
}

func writtenPaths(t *testing.T, output string) []string {
	t.Helper()

	scanner := bufio.NewScanner(strings.NewReader(output))
	paths := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		paths = append(paths, strings.TrimSpace(strings.TrimPrefix(line, "wrote ")))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
	}
	return paths
}
