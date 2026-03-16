package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/panex-dev/panex/internal/release"
)

var execCommand = exec.Command

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("panex-release", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	version := fs.String("version", "", "Release version to embed into the panex binary")
	outDir := fs.String("out-dir", filepath.Join("dist", "release"), "Output directory for release archives")
	targetsRaw := fs.String("targets", "", "Comma-separated goos/goarch target list")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("invalid release flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if err := release.ValidateVersion(*version); err != nil {
		return err
	}

	targets, err := release.ParseTargets(*targetsRaw)
	if err != nil {
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	readmeBytes, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create release output dir: %w", err)
	}

	checksumEntries := map[string]string{}
	for _, target := range targets {
		archivePath := filepath.Join(*outDir, release.ArchiveFileName(*version, target))
		if err := packageTarget(repoRoot, archivePath, *version, target, readmeBytes); err != nil {
			return err
		}
		archiveBytes, err := os.ReadFile(archivePath)
		if err != nil {
			return fmt.Errorf("read archive %q for checksum: %w", archivePath, err)
		}
		checksumEntries[filepath.Base(archivePath)] = release.SHA256Hex(archiveBytes)
		if _, err := fmt.Fprintf(stdout, "wrote %s\n", archivePath); err != nil {
			return err
		}

		// Generate .deb package for eligible Linux targets.
		if debName := release.DebFileName(*version, target); debName != "" {
			debPath := filepath.Join(*outDir, debName)
			if err := packageDeb(repoRoot, debPath, *version, target); err != nil {
				return err
			}
			debBytes, err := os.ReadFile(debPath)
			if err != nil {
				return fmt.Errorf("read deb %q for checksum: %w", debPath, err)
			}
			checksumEntries[filepath.Base(debPath)] = release.SHA256Hex(debBytes)
			if _, err := fmt.Fprintf(stdout, "wrote %s\n", debPath); err != nil {
				return err
			}
		}
	}

	checksumPath := filepath.Join(*outDir, release.ChecksumFileName(*version))
	if err := writeChecksumManifest(checksumPath, checksumEntries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "wrote %s\n", checksumPath); err != nil {
		return err
	}

	return nil
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	current := wd
	for {
		if fileExists(filepath.Join(current, "go.mod")) && fileExists(filepath.Join(current, "README.md")) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", errors.New("repository root not found from current working directory")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func packageTarget(repoRoot, archivePath, version string, target release.Target, readmeBytes []byte) error {
	tempDir, err := os.MkdirTemp("", "panex-release-*")
	if err != nil {
		return fmt.Errorf("create temp release dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	binaryPath := filepath.Join(tempDir, target.BinaryFileName())
	if err := buildBinary(repoRoot, binaryPath, version, target); err != nil {
		return err
	}
	binaryBytes, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read built binary %q: %w", binaryPath, err)
	}

	files := release.ReleaseFiles(version, target, binaryBytes, readmeBytes)
	if err := writeArchive(archivePath, target, files); err != nil {
		return err
	}
	return nil
}

func buildBinary(repoRoot, outputPath, version string, target release.Target) error {
	ldflags := fmt.Sprintf("-buildid= -X main.version=%s", version)
	cmd := execCommand(
		"go",
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", ldflags,
		"-o", outputPath,
		"./cmd/panex/...",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("build panex for %s: %s", target.String(), message)
	}
	return nil
}

func writeArchive(path string, target release.Target, files []release.File) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create archive %q: %w", path, err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()

	if err := release.WriteArchive(file, target, files); err != nil {
		return fmt.Errorf("write archive %q: %w", path, err)
	}
	return nil
}

func packageDeb(repoRoot, debPath, version string, target release.Target) error {
	tempDir, err := os.MkdirTemp("", "panex-deb-*")
	if err != nil {
		return fmt.Errorf("create temp deb dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	binaryPath := filepath.Join(tempDir, target.BinaryFileName())
	if err := buildBinary(repoRoot, binaryPath, version, target); err != nil {
		return err
	}
	binaryBytes, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read built binary %q: %w", binaryPath, err)
	}

	return writeDebFile(debPath, version, target, binaryBytes)
}

func writeDebFile(path, version string, target release.Target, binaryData []byte) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create deb %q: %w", path, err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()

	if err := release.WriteDeb(file, version, target, binaryData); err != nil {
		return fmt.Errorf("write deb %q: %w", path, err)
	}
	return nil
}

func writeChecksumManifest(path string, entries map[string]string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create checksum manifest %q: %w", path, err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()

	if err := release.WriteChecksumManifest(file, entries); err != nil {
		return fmt.Errorf("write checksum manifest %q: %w", path, err)
	}
	return nil
}
