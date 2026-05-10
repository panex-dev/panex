package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/panex-dev/panex/internal/cli"
	panexconfig "github.com/panex-dev/panex/internal/config"
)

type pathJSONEntry struct {
	ID        string `json:"id,omitempty"`
	SourceDir string `json:"source_dir"`
	OutDir    string `json:"out_dir"`
}

func writeJSONEnvelope(w io.Writer, out cli.Output) error {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func writeJSONCommandError(w io.Writer, code int, command string, message string, next []string, data any) error {
	if err := writeJSONEnvelope(w, cli.Output{
		Status:  "error",
		Command: command,
		Errors:  []string{message},
		Next:    next,
		Data:    data,
	}); err != nil {
		return err
	}
	return &cliError{code: code}
}

func configPathEntries(cfg panexconfig.Config) []pathJSONEntry {
	entries := make([]pathJSONEntry, 0, len(cfg.Extensions))
	for _, ext := range cfg.Extensions {
		absSource, err := filepath.Abs(ext.SourceDir)
		if err != nil {
			absSource = ext.SourceDir
		}
		absOut, err := filepath.Abs(ext.OutDir)
		if err != nil {
			absOut = ext.OutDir
		}
		entry := pathJSONEntry{
			SourceDir: absSource,
			OutDir:    absOut,
		}
		if len(cfg.Extensions) > 1 {
			entry.ID = ext.ID
		}
		entries = append(entries, entry)
	}
	return entries
}

func devStartupEnvelope(cfg panexconfig.Config, warnings []string) cli.Output {
	entries := configPathEntries(cfg)
	loadPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		loadPaths = append(loadPaths, entry.OutDir)
	}

	data := map[string]any{
		"ws_url":               fmt.Sprintf("ws://%s:%d/ws", cfg.Server.BindAddress, cfg.Server.Port),
		"extensions":           entries,
		"load_unpacked_paths":  loadPaths,
		"event_store_path":     cfg.Server.EventStorePath,
		"bind_address":         cfg.Server.BindAddress,
		"port":                 cfg.Server.Port,
		"multi_extension_mode": len(entries) > 1,
	}

	return cli.Output{
		Status:   "ok",
		Command:  "dev",
		Summary:  "dev runtime started",
		Data:     data,
		Warnings: warnings,
	}
}
