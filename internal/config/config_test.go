package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	yamlContent := `
music:
  chill_dir: /tmp/music/chill
  drive_dir: /tmp/music/drive
telemetry:
  addr: 127.0.0.1:5301
  drive_speed_mph: 15.5
  drive_throttle_pct: 30.0
  idle_speed_mph: 4.0
  idle_cooldown_sec: 5.5
audio:
  fade_sec: 3.0
  sample_rate: 48000
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Music.ChillDir != "/tmp/music/chill" {
		t.Errorf("ChillDir = %v, want /tmp/music/chill", cfg.Music.ChillDir)
	}
	if cfg.Telemetry.DriveSpeedMph != 15.5 {
		t.Errorf("DriveSpeedMph = %v, want 15.5", cfg.Telemetry.DriveSpeedMph)
	}
	if cfg.Telemetry.Addr != "127.0.0.1:5301" {
		t.Errorf("Addr = %v, want 127.0.0.1:5301", cfg.Telemetry.Addr)
	}
	if cfg.Audio.FadeSec != 3.0 {
		t.Errorf("FadeSec = %v, want 3.0", cfg.Audio.FadeSec)
	}
}
