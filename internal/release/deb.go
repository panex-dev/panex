package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"time"
)

const (
	debArMagic      = "!<arch>\n"
	debArEntryFmt   = "%-16s%-12d%-6d%-6d%-8s%-10d`\n"
	debBinaryMarker = "2.0\n"
)

// DebTarget filters the supported target set to those eligible for .deb packaging.
// Only linux/amd64 and linux/arm64 produce .deb files.
var debArchMap = map[string]string{
	"amd64": "amd64",
	"arm64": "arm64",
}

// DebArchitecture returns the Debian architecture name for a target, or empty
// string if the target is not eligible for .deb packaging.
func DebArchitecture(target Target) string {
	if target.GOOS != "linux" {
		return ""
	}
	return debArchMap[target.GOARCH]
}

// DebFileName returns the conventional .deb file name for a version and target.
func DebFileName(version string, target Target) string {
	debArch := DebArchitecture(target)
	if debArch == "" {
		return ""
	}
	// Strip leading "v" for Debian version convention.
	debVersion := version
	if len(debVersion) > 0 && debVersion[0] == 'v' {
		debVersion = debVersion[1:]
	}
	return fmt.Sprintf("panex_%s_%s.deb", debVersion, debArch)
}

// WriteDeb writes a minimal .deb package to w containing the panex binary.
//
// The .deb format is an ar(5) archive with three members:
//   - debian-binary: version marker "2.0\n"
//   - control.tar.gz: package metadata
//   - data.tar.gz: installed file tree
func WriteDeb(w io.Writer, version string, target Target, binaryData []byte) error {
	debArch := DebArchitecture(target)
	if debArch == "" {
		return fmt.Errorf("target %s is not eligible for .deb packaging", target.String())
	}

	debVersion := version
	if len(debVersion) > 0 && debVersion[0] == 'v' {
		debVersion = debVersion[1:]
	}

	controlData, err := buildControlTarGz(debVersion, debArch, len(binaryData))
	if err != nil {
		return fmt.Errorf("build control archive: %w", err)
	}

	dataTarGz, err := buildDataTarGz(binaryData)
	if err != nil {
		return fmt.Errorf("build data archive: %w", err)
	}

	// Write ar archive.
	if _, err := io.WriteString(w, debArMagic); err != nil {
		return err
	}
	if err := writeArEntry(w, "debian-binary", []byte(debBinaryMarker)); err != nil {
		return err
	}
	if err := writeArEntry(w, "control.tar.gz", controlData); err != nil {
		return err
	}
	if err := writeArEntry(w, "data.tar.gz", dataTarGz); err != nil {
		return err
	}

	return nil
}

func buildControlTarGz(debVersion, debArch string, installedSize int) ([]byte, error) {
	control := fmt.Sprintf(`Package: panex
Version: %s
Architecture: %s
Maintainer: Panex <hello@panex.dev>
Installed-Size: %d
Section: devel
Priority: optional
Homepage: https://github.com/panex-dev/panex
Description: Development runtime for Chrome extensions
 Panex is a development runtime for Chrome extensions that lets you
 save, inspect, and replay extension behavior across contexts.
`, debVersion, debArch, installedSize/1024)

	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gzWriter.ModTime = time.Unix(0, 0).UTC()
	gzWriter.OS = 255

	tw := tar.NewWriter(gzWriter)
	if err := writeTarEntry(tw, "./control", []byte(control), 0o644); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildDataTarGz(binaryData []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gzWriter.ModTime = time.Unix(0, 0).UTC()
	gzWriter.OS = 255

	tw := tar.NewWriter(gzWriter)

	// Create directory entries for /usr and /usr/local and /usr/local/bin.
	for _, dir := range []string{"./usr/", "./usr/local/", "./usr/local/bin/"} {
		if err := writeTarDir(tw, dir); err != nil {
			return nil, err
		}
	}

	if err := writeTarEntry(tw, "./usr/local/bin/panex", binaryData, 0o755); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:     name,
		Mode:     mode,
		Size:     int64(len(data)),
		ModTime:  time.Unix(0, 0).UTC(),
		Typeflag: tar.TypeReg,
		Format:   tar.FormatGNU,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarDir(tw *tar.Writer, name string) error {
	header := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
		ModTime:  time.Unix(0, 0).UTC(),
		Format:   tar.FormatGNU,
	}
	return tw.WriteHeader(header)
}

func writeArEntry(w io.Writer, name string, data []byte) error {
	header := fmt.Sprintf(debArEntryFmt, name, 0, 0, 0, "100644", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	// ar entries are padded to even byte boundaries.
	if len(data)%2 != 0 {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}
