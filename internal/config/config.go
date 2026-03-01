package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultPath           = "panex.toml"
	DefaultEventStorePath = ".panex/events.db"
	minPort               = 1
	maxPort               = 65535
)

type Config struct {
	Extension Extension `toml:"extension"`
	Server    Server    `toml:"server"`
}

type Extension struct {
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
			return Config{}, fmt.Errorf("config file not found: %s", path)
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

func (c Config) Validate() error {
	if strings.TrimSpace(c.Extension.SourceDir) == "" {
		return errors.New("extension.source_dir is required")
	}
	if strings.TrimSpace(c.Extension.OutDir) == "" {
		return errors.New("extension.out_dir is required")
	}
	if c.Server.Port < minPort || c.Server.Port > maxPort {
		return fmt.Errorf("server.port must be between %d and %d", minPort, maxPort)
	}
	if strings.TrimSpace(c.Server.AuthToken) == "" {
		return errors.New("server.auth_token is required")
	}

	return nil
}
