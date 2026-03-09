package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

func TestParseTargetsDefaultsToSupportedMatrix(t *testing.T) {
	targets, err := ParseTargets("")
	if err != nil {
		t.Fatalf("ParseTargets() returned error: %v", err)
	}
	if len(targets) != 6 {
		t.Fatalf("unexpected target count: got %d, want 6", len(targets))
	}
	if targets[0].String() != "darwin/amd64" || targets[5].String() != "windows/arm64" {
		t.Fatalf("unexpected default targets: %v", targets)
	}
}

func TestParseTargetsRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name   string
		raw    string
		expect string
	}{
		{name: "missing slash", raw: "linux-amd64", expect: "expected goos/goarch"},
		{name: "unsupported target", raw: "freebsd/amd64", expect: "unsupported target"},
		{name: "duplicate target", raw: "linux/amd64,linux/amd64", expect: "duplicate target"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTargets(tc.raw)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.expect)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	testCases := []struct {
		name    string
		version string
		wantErr string
	}{
		{name: "valid", version: "v0.1.0"},
		{name: "missing", version: "", wantErr: "version is required"},
		{name: "dev", version: "dev", wantErr: `must not be "dev"`},
		{name: "missing prefix", version: "0.1.0", wantErr: `must start with "v"`},
		{name: "path separator", version: "v0.1.0/test", wantErr: "must not contain path separators"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateVersion(tc.version)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateVersion() returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestWriteArchiveTarGzIsDeterministic(t *testing.T) {
	files := ReleaseFiles("v1.2.3", Target{GOOS: "linux", GOARCH: "amd64"}, []byte("binary"), []byte("readme"))

	first := writeArchiveBytes(t, Target{GOOS: "linux", GOARCH: "amd64"}, files)
	second := writeArchiveBytes(t, Target{GOOS: "linux", GOARCH: "amd64"}, files)
	if !bytes.Equal(first, second) {
		t.Fatal("expected deterministic tar.gz output")
	}

	paths := readTarPaths(t, first)
	want := []string{
		"panex_v1.2.3_linux_amd64/README.md",
		"panex_v1.2.3_linux_amd64/panex",
	}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected tar paths: got %v, want %v", paths, want)
	}
}

func TestWriteArchiveZipIsDeterministic(t *testing.T) {
	files := ReleaseFiles("v1.2.3", Target{GOOS: "windows", GOARCH: "arm64"}, []byte("binary"), []byte("readme"))

	first := writeArchiveBytes(t, Target{GOOS: "windows", GOARCH: "arm64"}, files)
	second := writeArchiveBytes(t, Target{GOOS: "windows", GOARCH: "arm64"}, files)
	if !bytes.Equal(first, second) {
		t.Fatal("expected deterministic zip output")
	}

	paths := readZipPaths(t, first)
	want := []string{
		"panex_v1.2.3_windows_arm64/README.md",
		"panex_v1.2.3_windows_arm64/panex.exe",
	}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected zip paths: got %v, want %v", paths, want)
	}
}

func TestWriteChecksumManifestSortsEntries(t *testing.T) {
	entries := map[string]string{
		"panex_v1.2.3_windows_amd64.zip":  SHA256Hex([]byte("windows")),
		"panex_v1.2.3_linux_amd64.tar.gz": SHA256Hex([]byte("linux")),
	}

	var buffer bytes.Buffer
	if err := WriteChecksumManifest(&buffer, entries); err != nil {
		t.Fatalf("WriteChecksumManifest() returned error: %v", err)
	}

	want := strings.Join([]string{
		SHA256Hex([]byte("linux")) + "  panex_v1.2.3_linux_amd64.tar.gz",
		SHA256Hex([]byte("windows")) + "  panex_v1.2.3_windows_amd64.zip",
		"",
	}, "\n")
	if buffer.String() != want {
		t.Fatalf("unexpected checksum manifest:\n%s", buffer.String())
	}
}

func writeArchiveBytes(t *testing.T, target Target, files []File) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if err := WriteArchive(&buffer, target, files); err != nil {
		t.Fatalf("WriteArchive() returned error: %v", err)
	}
	return buffer.Bytes()
}

func readTarPaths(t *testing.T, data []byte) []string {
	t.Helper()

	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader() returned error: %v", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			t.Fatalf("gzipReader.Close() returned error: %v", closeErr)
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	paths := []string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return paths
		}
		if err != nil {
			t.Fatalf("tarReader.Next() returned error: %v", err)
		}
		paths = append(paths, header.Name)
	}
}

func readZipPaths(t *testing.T, data []byte) []string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() returned error: %v", err)
	}

	paths := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		paths = append(paths, file.Name)
	}
	return paths
}
