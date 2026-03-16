package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

func TestDebArchitectureReturnsLinuxOnly(t *testing.T) {
	tests := []struct {
		target Target
		want   string
	}{
		{Target{GOOS: "linux", GOARCH: "amd64"}, "amd64"},
		{Target{GOOS: "linux", GOARCH: "arm64"}, "arm64"},
		{Target{GOOS: "darwin", GOARCH: "arm64"}, ""},
		{Target{GOOS: "windows", GOARCH: "amd64"}, ""},
	}
	for _, tt := range tests {
		got := DebArchitecture(tt.target)
		if got != tt.want {
			t.Errorf("DebArchitecture(%s) = %q, want %q", tt.target, got, tt.want)
		}
	}
}

func TestDebFileNameStripsVersionPrefix(t *testing.T) {
	name := DebFileName("v1.2.3", Target{GOOS: "linux", GOARCH: "amd64"})
	want := "panex_1.2.3_amd64.deb"
	if name != want {
		t.Errorf("DebFileName() = %q, want %q", name, want)
	}
}

func TestDebFileNameReturnsEmptyForNonLinux(t *testing.T) {
	name := DebFileName("v1.0.0", Target{GOOS: "darwin", GOARCH: "arm64"})
	if name != "" {
		t.Errorf("DebFileName() = %q, want empty", name)
	}
}

func TestWriteDebProducesValidArArchive(t *testing.T) {
	binary := []byte("fake-panex-binary")
	var buf bytes.Buffer
	err := WriteDeb(&buf, "v1.2.3", Target{GOOS: "linux", GOARCH: "amd64"}, binary)
	if err != nil {
		t.Fatalf("WriteDeb() error: %v", err)
	}

	data := buf.Bytes()

	// Verify ar magic.
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatal("missing ar magic header")
	}

	// Parse ar entries.
	entries := parseArEntries(t, data[8:])
	if len(entries) != 3 {
		t.Fatalf("expected 3 ar entries, got %d", len(entries))
	}

	// debian-binary must be "2.0\n".
	if entries[0].name != "debian-binary" {
		t.Errorf("first entry name = %q, want debian-binary", entries[0].name)
	}
	if string(entries[0].data) != "2.0\n" {
		t.Errorf("debian-binary content = %q, want %q", entries[0].data, "2.0\n")
	}

	// control.tar.gz must contain ./control with correct metadata.
	if entries[1].name != "control.tar.gz" {
		t.Errorf("second entry name = %q, want control.tar.gz", entries[1].name)
	}
	controlContent := extractTarGzFile(t, entries[1].data, "./control")
	for _, required := range []string{"Package: panex", "Version: 1.2.3", "Architecture: amd64"} {
		if !strings.Contains(controlContent, required) {
			t.Errorf("control missing %q:\n%s", required, controlContent)
		}
	}

	// data.tar.gz must contain the binary at ./usr/local/bin/panex.
	if entries[2].name != "data.tar.gz" {
		t.Errorf("third entry name = %q, want data.tar.gz", entries[2].name)
	}
	binaryContent := extractTarGzFile(t, entries[2].data, "./usr/local/bin/panex")
	if binaryContent != string(binary) {
		t.Errorf("binary content mismatch: got %q, want %q", binaryContent, binary)
	}
}

func TestWriteDebIsDeterministic(t *testing.T) {
	binary := []byte("panex-binary-content")
	target := Target{GOOS: "linux", GOARCH: "arm64"}

	var buf1, buf2 bytes.Buffer
	if err := WriteDeb(&buf1, "v2.0.0", target, binary); err != nil {
		t.Fatal(err)
	}
	if err := WriteDeb(&buf2, "v2.0.0", target, binary); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Fatal("expected deterministic .deb output")
	}
}

func TestWriteDebRejectsNonLinux(t *testing.T) {
	var buf bytes.Buffer
	err := WriteDeb(&buf, "v1.0.0", Target{GOOS: "darwin", GOARCH: "arm64"}, []byte("bin"))
	if err == nil {
		t.Fatal("expected error for non-linux target")
	}
	if !strings.Contains(err.Error(), "not eligible") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- helpers ----------------------------------------------------------------

type arEntry struct {
	name string
	data []byte
}

func parseArEntries(t *testing.T, data []byte) []arEntry {
	t.Helper()
	var entries []arEntry
	for len(data) > 0 {
		if len(data) < 60 {
			t.Fatalf("truncated ar header at offset %d", len(data))
		}
		name := strings.TrimRight(string(data[:16]), " ")
		// Remove trailing "/" from ar names if present.
		name = strings.TrimSuffix(name, "/")

		sizeStr := strings.TrimSpace(string(data[48:58]))
		var size int
		for _, c := range sizeStr {
			size = size*10 + int(c-'0')
		}

		// Skip the header (60 bytes) + magic "`\n" is part of header.
		data = data[60:]
		if len(data) < size {
			t.Fatalf("truncated ar entry %q: need %d bytes, have %d", name, size, len(data))
		}
		entries = append(entries, arEntry{name: name, data: data[:size]})
		data = data[size:]
		// Skip padding byte.
		if size%2 != 0 && len(data) > 0 {
			data = data[1:]
		}
	}
	return entries
}

func extractTarGzFile(t *testing.T, data []byte, targetPath string) string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader() error: %v", err)
	}
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			t.Fatalf("gz.Close() error: %v", closeErr)
		}
	}()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("file %q not found in tar.gz", targetPath)
		}
		if err != nil {
			t.Fatalf("tar.Next() error: %v", err)
		}
		if hdr.Name == targetPath {
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("reading %q: %v", targetPath, err)
			}
			return string(content)
		}
	}
}
