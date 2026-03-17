package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrManifestNotFound   = errors.New("manifest.json not found")
)

const (
	DefaultPath           = "panex.toml"
	DefaultEventStorePath = ".panex/events.db"
	DefaultExtensionID    = "default"
	DefaultPort           = 4317
	DefaultOutDir         = ".panex/dist"
	DefaultAuthToken      = "dev-token"
	minPort               = 1024
	maxPort               = 65535
)

type Config struct {
	Extension  Extension   `toml:"extension"`
	Extensions []Extension `toml:"extensions"`
	Server     Server      `toml:"server"`
}

type Extension struct {
	ID        string `toml:"id"`
	SourceDir string `toml:"source_dir"`
	OutDir    string `toml:"out_dir"`
}

type Server struct {
	Port           int    `toml:"port"`
	AuthToken      string `toml:"auth_token"`
	EventStorePath string `toml:"event_store_path"`
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("%w: %s", ErrConfigFileNotFound, path)
		}
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parse TOML config %q: %w", path, err)
	}
	if err := validateUndecoded(meta); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.Server.EventStorePath) == "" {
		cfg.Server.EventStorePath = DefaultEventStorePath
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Infer returns a convention-based Config for a directory that contains a
// manifest.json but no panex.toml. The inferred config uses the directory
// itself as source_dir and .panex/dist as out_dir.
func Infer(dir string) (Config, error) {
	if strings.TrimSpace(dir) == "" {
		return Config{}, errors.New("directory is required")
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("%w: %s", ErrManifestNotFound, manifestPath)
		}
		return Config{}, fmt.Errorf("stat manifest.json: %w", err)
	}

	cfg := Config{
		Extensions: []Extension{{
			ID:        DefaultExtensionID,
			SourceDir: dir,
			OutDir:    filepath.Join(dir, DefaultOutDir),
		}},
		Server: Server{
			Port:           DefaultPort,
			AuthToken:      DefaultAuthToken,
			EventStorePath: DefaultEventStorePath,
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate inferred config: %w", err)
	}

	return cfg, nil
}

func validateUndecoded(meta toml.MetaData) error {
	unknown := meta.Undecoded()
	if len(unknown) == 0 {
		return nil
	}

	keys := make([]string, 0, len(unknown))
	for _, key := range unknown {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)

	return fmt.Errorf("unknown config keys: %s", strings.Join(keys, ", "))
}

func (c *Config) Validate() error {
	legacyMode := len(c.Extensions) == 0
	resolvedExtensions, err := resolveExtensions(c.Extension, c.Extensions)
	if err != nil {
		return err
	}
	if err := validateExtensions(resolvedExtensions, legacyMode); err != nil {
		return err
	}
	if c.Server.Port < minPort || c.Server.Port > maxPort {
		return fmt.Errorf("server.port must be between %d and %d", minPort, maxPort)
	}
	if strings.TrimSpace(c.Server.AuthToken) == "" {
		return errors.New("server.auth_token is required")
	}

	c.Extensions = resolvedExtensions
	c.Extension = resolvedExtensions[0]
	return nil
}

func resolveExtensions(legacy Extension, configured []Extension) ([]Extension, error) {
	hasLegacy := hasLegacyExtension(legacy)
	hasConfigured := len(configured) > 0
	if hasLegacy && hasConfigured {
		return nil, errors.New("use either [extension] or [[extensions]], not both")
	}

	if hasConfigured {
		resolved := make([]Extension, 0, len(configured))
		for index, extension := range configured {
			if strings.TrimSpace(extension.ID) == "" {
				return nil, fmt.Errorf("extensions[%d].id is required", index)
			}
			resolved = append(resolved, Extension{
				ID:        strings.TrimSpace(extension.ID),
				SourceDir: extension.SourceDir,
				OutDir:    extension.OutDir,
			})
		}
		return resolved, nil
	}

	return []Extension{{
		ID:        DefaultExtensionID,
		SourceDir: legacy.SourceDir,
		OutDir:    legacy.OutDir,
	}}, nil
}

func hasLegacyExtension(extension Extension) bool {
	return strings.TrimSpace(extension.SourceDir) != "" ||
		strings.TrimSpace(extension.OutDir) != "" ||
		strings.TrimSpace(extension.ID) != ""
}

func validateExtensions(extensions []Extension, legacyMode bool) error {
	if len(extensions) == 0 {
		return errors.New("at least one extension is required")
	}

	seenIDs := make(map[string]struct{}, len(extensions))
	resolvedPaths := make([]resolvedExtensionPaths, 0, len(extensions))
	for index, extension := range extensions {
		label := extensionLabel(index, extension.ID, legacyMode)
		if strings.TrimSpace(extension.SourceDir) == "" {
			return fmt.Errorf("%s.source_dir is required", label)
		}
		if strings.TrimSpace(extension.OutDir) == "" {
			return fmt.Errorf("%s.out_dir is required", label)
		}
		if _, ok := seenIDs[extension.ID]; ok {
			return fmt.Errorf("extension ids must be unique: %q", extension.ID)
		}
		seenIDs[extension.ID] = struct{}{}

		sourceOutOverlap, err := pathsOverlap(extension.SourceDir, extension.OutDir)
		if err != nil {
			return fmt.Errorf("resolve %s paths: %w", label, err)
		}
		if sourceOutOverlap {
			return fmt.Errorf("%s.source_dir and %s.out_dir must not overlap", label, label)
		}

		absSourceDir, err := filepath.Abs(extension.SourceDir)
		if err != nil {
			return fmt.Errorf("resolve %s.source_dir: %w", label, err)
		}
		absOutDir, err := filepath.Abs(extension.OutDir)
		if err != nil {
			return fmt.Errorf("resolve %s.out_dir: %w", label, err)
		}
		resolvedPaths = append(resolvedPaths, resolvedExtensionPaths{
			id:        extension.ID,
			sourceDir: absSourceDir,
			outDir:    absOutDir,
		})
	}

	for index := 0; index < len(resolvedPaths); index++ {
		for other := index + 1; other < len(resolvedPaths); other++ {
			if overlapsAnyPath(resolvedPaths[index], resolvedPaths[other]) {
				return fmt.Errorf(
					"extensions %q and %q must not share overlapping source_dir or out_dir paths",
					resolvedPaths[index].id,
					resolvedPaths[other].id,
				)
			}
		}
	}

	return nil
}

type resolvedExtensionPaths struct {
	id        string
	sourceDir string
	outDir    string
}

func overlapsAnyPath(first, second resolvedExtensionPaths) bool {
	return pathsOverlapResolved(first.sourceDir, second.sourceDir) ||
		pathsOverlapResolved(first.sourceDir, second.outDir) ||
		pathsOverlapResolved(first.outDir, second.sourceDir) ||
		pathsOverlapResolved(first.outDir, second.outDir)
}

func pathsOverlapResolved(first, second string) bool {
	return isSameOrNestedPath(first, second) || isSameOrNestedPath(second, first)
}

func extensionLabel(index int, id string, legacyMode bool) string {
	if legacyMode && index == 0 {
		return "extension"
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Sprintf("extensions[%d]", index)
	}

	return fmt.Sprintf("extensions[%q]", id)
}

// pathsOverlap reports whether sourceDir and outDir overlap in a way that
// would cause build loops or data corruption. Output nested inside source is
// allowed when the relative path passes through an infrastructure directory
// (e.g. .panex/dist) because source walkers and file watchers skip those
// directories.
func pathsOverlap(sourceDir, outDir string) (bool, error) {
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return false, err
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return false, err
	}

	if absSource == absOut {
		return true, nil
	}
	// Source nested inside output is always dangerous.
	if isSameOrNestedPath(absOut, absSource) {
		return true, nil
	}
	// Output nested inside source is safe only when the nesting passes
	// through an infrastructure directory (e.g. .panex/dist), because
	// source walkers and file watchers skip those directories.
	if isSameOrNestedPath(absSource, absOut) {
		return !isShieldedByInfrastructureDir(absSource, absOut), nil
	}
	return false, nil
}

func isSameOrNestedPath(parent, child string) bool {
	relPath, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if relPath == "." {
		return true
	}

	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator))
}

// isShieldedByInfrastructureDir reports whether the relative path from parent
// to child begins with a directory that source walkers and file watchers skip
// (node_modules or dot-prefixed directories like .panex, .git).
func isShieldedByInfrastructureDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	return len(parts) > 0 && isInfrastructureDir(parts[0])
}

// isInfrastructureDir reports whether a directory name represents build
// infrastructure that source walkers and file watchers skip. This matches
// the same predicate in package build and package daemon.
func isInfrastructureDir(name string) bool {
	return name == "node_modules" || strings.HasPrefix(name, ".")
}
