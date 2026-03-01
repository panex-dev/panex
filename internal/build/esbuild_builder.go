package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

type EsbuildBuilder struct {
	sourceDir string
	outDir    string
	seq       uint64
}

type Result struct {
	BuildID      string
	Success      bool
	DurationMS   int64
	ChangedFiles []string
	Errors       []string
}

func NewEsbuildBuilder(sourceDir, outDir string) (*EsbuildBuilder, error) {
	if strings.TrimSpace(sourceDir) == "" {
		return nil, errors.New("source directory is required")
	}
	if strings.TrimSpace(outDir) == "" {
		return nil, errors.New("output directory is required")
	}

	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("stat source directory %q: %w", sourceDir, err)
	}
	if !sourceInfo.IsDir() {
		return nil, fmt.Errorf("source directory must be a directory: %s", sourceDir)
	}

	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source directory %q: %w", sourceDir, err)
	}
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory %q: %w", outDir, err)
	}

	return &EsbuildBuilder{
		sourceDir: absSourceDir,
		outDir:    absOutDir,
	}, nil
}

func (b *EsbuildBuilder) Build(ctx context.Context, changedPaths []string) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	buildID := b.nextBuildID()
	startedAt := time.Now()

	if err := os.MkdirAll(b.outDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory %q: %w", b.outDir, err)
	}

	entryPoints, err := discoverEntryPoints(b.sourceDir)
	if err != nil {
		return Result{}, err
	}
	if len(entryPoints) == 0 {
		return Result{}, fmt.Errorf("no entry points found in %s", b.sourceDir)
	}

	result := api.Build(api.BuildOptions{
		AbsWorkingDir: b.sourceDir,
		EntryPoints:   entryPoints,
		Outdir:        b.outDir,
		Bundle:        true,
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		Write:         true,
		Sourcemap:     api.SourceMapLinked,
		LogLevel:      api.LogLevelSilent,
		EntryNames:    "[dir]/[name]",
	})

	durationMS := time.Since(startedAt).Milliseconds()
	normalizedChanges := normalizeChangedPaths(changedPaths)

	if len(result.Errors) > 0 {
		return Result{
			BuildID:      buildID,
			Success:      false,
			DurationMS:   durationMS,
			ChangedFiles: normalizedChanges,
			Errors:       collectMessages(result.Errors),
		}, nil
	}

	return Result{
		BuildID:      buildID,
		Success:      true,
		DurationMS:   durationMS,
		ChangedFiles: normalizedChanges,
	}, nil
}

func (b *EsbuildBuilder) nextBuildID() string {
	seq := atomic.AddUint64(&b.seq, 1)
	return fmt.Sprintf("build-%d-%d", time.Now().UTC().UnixMilli(), seq)
}

func discoverEntryPoints(sourceDir string) ([]string, error) {
	entries := make([]string, 0, 8)

	err := filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !isBundleEntry(path) {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover entry points in %q: %w", sourceDir, err)
	}

	sort.Strings(entries)
	return entries, nil
}

func isBundleEntry(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx":
		return true
	default:
		return false
	}
}

func collectMessages(messages []api.Message) []string {
	out := make([]string, 0, len(messages))
	for _, message := range messages {
		text := strings.TrimSpace(message.Text)
		if message.Location == nil {
			out = append(out, text)
			continue
		}

		out = append(out, fmt.Sprintf("%s:%d:%d: %s", message.Location.File, message.Location.Line, message.Location.Column, text))
	}
	sort.Strings(out)
	return out
}

func normalizeChangedPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if normalized == "" || normalized == "." {
			continue
		}
		set[normalized] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
