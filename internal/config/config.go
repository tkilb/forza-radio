package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Music     MusicConfig     `yaml:"music"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Audio     AudioConfig     `yaml:"audio"`
}

type MusicConfig struct {
	ChillDir string `yaml:"chill_dir"`
	DriveDir string `yaml:"drive_dir"`
}

type TelemetryConfig struct {
	Addr             string  `yaml:"addr"`
	DriveSpeedMph    float64 `yaml:"drive_speed_mph"`
	DriveThrottlePct float64 `yaml:"drive_throttle_pct"`
	IdleSpeedMph     float64 `yaml:"idle_speed_mph"`
	IdleCooldownSec  float64 `yaml:"idle_cooldown_sec"`
}

type AudioConfig struct {
	FadeSec    float64 `yaml:"fade_sec"`
	SampleRate int     `yaml:"sample_rate"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Music: MusicConfig{
			ChillDir: filepath.Join(home, "Music", "chill"),
			DriveDir: filepath.Join(home, "Music", "drive"),
		},
		Telemetry: TelemetryConfig{
			Addr:             "127.0.0.1:5300",
			DriveSpeedMph:    10.0,
			DriveThrottlePct: 25.0,
			IdleSpeedMph:     5.0,
			IdleCooldownSec:  4.0,
		},
		Audio: AudioConfig{
			FadeSec:    2.0,
			SampleRate: 44100,
		},
	}
}

// ExpandHome replaces a leading ~/ with the user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// LoadConfig attempts to load a config from path. If path is empty, it checks ~/.config/forza-radio/config.yaml.
func LoadConfig(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultPath := filepath.Join(home, ".config", "forza-radio", "config.yaml")
			if _, err := os.Stat(defaultPath); err == nil {
				path = defaultPath
			}
		}
	}

	if path == "" {
		return &cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml config %s: %w", path, err)
	}

	cfg.Music.ChillDir = ExpandHome(cfg.Music.ChillDir)
	cfg.Music.DriveDir = ExpandHome(cfg.Music.DriveDir)

	return &cfg, nil
}
