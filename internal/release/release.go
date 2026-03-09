package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"time"
)

const (
	BinaryName          = "panex"
	fixedZipTimestamp   = 315532800 // 1980-01-01T00:00:00Z
	fixedTarTimestamp   = 0
	releaseNameTemplate = "%s_%s_%s"
)

var (
	defaultTargets = []Target{
		{GOOS: "darwin", GOARCH: "amd64"},
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
		{GOOS: "windows", GOARCH: "amd64"},
		{GOOS: "windows", GOARCH: "arm64"},
	}
	supportedTargets = map[string]struct{}{
		"darwin/amd64":  {},
		"darwin/arm64":  {},
		"linux/amd64":   {},
		"linux/arm64":   {},
		"windows/amd64": {},
		"windows/arm64": {},
	}
)

type Target struct {
	GOOS   string
	GOARCH string
}

type File struct {
	ArchivePath string
	Mode        fs.FileMode
	Data        []byte
}

func DefaultTargets() []Target {
	targets := make([]Target, len(defaultTargets))
	copy(targets, defaultTargets)
	return targets
}

func ValidateVersion(version string) error {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return errors.New("version is required")
	}
	if trimmed == "dev" {
		return errors.New(`version must not be "dev"`)
	}
	if !strings.HasPrefix(trimmed, "v") {
		return errors.New(`version must start with "v"`)
	}
	if strings.ContainsAny(trimmed, `/\ `+"\t\n\r") {
		return errors.New("version must not contain path separators or whitespace")
	}
	return nil
}

func ParseTargets(raw string) ([]Target, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultTargets(), nil
	}

	parts := strings.Split(trimmed, ",")
	targets := make([]Target, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		segments := strings.Split(value, "/")
		if len(segments) != 2 {
			return nil, fmt.Errorf("invalid target %q: expected goos/goarch", value)
		}

		target := Target{
			GOOS:   strings.TrimSpace(segments[0]),
			GOARCH: strings.TrimSpace(segments[1]),
		}
		key := target.String()
		if _, ok := supportedTargets[key]; !ok {
			return nil, fmt.Errorf("unsupported target %q", key)
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate target %q", key)
		}

		seen[key] = struct{}{}
		targets = append(targets, target)
	}

	return targets, nil
}

func (t Target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

func (t Target) BinaryFileName() string {
	if t.GOOS == "windows" {
		return BinaryName + ".exe"
	}
	return BinaryName
}

func ArchiveBaseName(version string, target Target) string {
	return fmt.Sprintf(releaseNameTemplate, BinaryName, version, target.GOOS+"_"+target.GOARCH)
}

func ArchiveFileName(version string, target Target) string {
	base := ArchiveBaseName(version, target)
	if target.GOOS == "windows" {
		return base + ".zip"
	}
	return base + ".tar.gz"
}

func ReleaseFiles(version string, target Target, binaryContents []byte, readmeContents []byte) []File {
	root := ArchiveBaseName(version, target)
	return []File{
		{
			ArchivePath: root + "/" + target.BinaryFileName(),
			Mode:        0o755,
			Data:        binaryContents,
		},
		{
			ArchivePath: root + "/README.md",
			Mode:        0o644,
			Data:        readmeContents,
		},
	}
}

func WriteArchive(w io.Writer, target Target, files []File) error {
	if target.GOOS == "windows" {
		return writeZipArchive(w, files)
	}
	return writeTarGzArchive(w, files)
}

func writeTarGzArchive(w io.Writer, files []File) error {
	gzipWriter, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Name = ""
	gzipWriter.Comment = ""
	gzipWriter.ModTime = time.Unix(fixedTarTimestamp, 0).UTC()
	gzipWriter.OS = 255

	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range sortedFiles(files) {
		header := &tar.Header{
			Name:     file.ArchivePath,
			Mode:     int64(file.Mode.Perm()),
			Size:     int64(len(file.Data)),
			ModTime:  time.Unix(fixedTarTimestamp, 0).UTC(),
			Typeflag: tar.TypeReg,
			Format:   tar.FormatPAX,
			Uid:      0,
			Gid:      0,
			Uname:    "",
			Gname:    "",
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return err
		}
		if _, err := tarWriter.Write(file.Data); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func writeZipArchive(w io.Writer, files []File) error {
	zipWriter := zip.NewWriter(w)
	for _, file := range sortedFiles(files) {
		header := &zip.FileHeader{
			Name:     file.ArchivePath,
			Method:   zip.Deflate,
			Modified: time.Unix(fixedZipTimestamp, 0).UTC(),
		}
		header.SetMode(file.Mode)

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			_ = zipWriter.Close()
			return err
		}
		if _, err := writer.Write(file.Data); err != nil {
			_ = zipWriter.Close()
			return err
		}
	}
	return zipWriter.Close()
}

func sortedFiles(files []File) []File {
	sorted := make([]File, len(files))
	copy(sorted, files)
	slices.SortFunc(sorted, func(left, right File) int {
		return strings.Compare(left.ArchivePath, right.ArchivePath)
	})
	return sorted
}
