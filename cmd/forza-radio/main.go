package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ebitengine/oto/v3"

	"forza-radio/internal/config"
	"forza-radio/internal/player"
	"forza-radio/internal/telemetry"
)

func main() {
	configPath := flag.String("config", "", "Path to YAML config file (default: ~/.config/forza-radio/config.yaml)")
	chillDir := flag.String("chill-dir", "", "Directory containing Chill/Idle MP3 files")
	driveDir := flag.String("drive-dir", "", "Directory containing Drive/Hype MP3 files")
	listenAddr := flag.String("addr", "", "UDP address to listen for Forza telemetry")
	driveSpeed := flag.Float64("drive-speed", 0.0, "Speed (mph) to transition to Drive deck")
	driveThrottle := flag.Float64("drive-throttle", 0.0, "Throttle (%) to transition to Drive deck")
	idleSpeed := flag.Float64("idle-speed", 0.0, "Speed (mph) below which idle cooldown begins")
	cooldownSec := flag.Float64("cooldown", 0.0, "Seconds vehicle must stay stopped before returning to Chill deck")
	fadeSec := flag.Float64("fade", 0.0, "Crossfade transition duration in seconds")
	debug := flag.Bool("debug", false, "Enable verbose debug logging")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "err", err)
		os.Exit(1)
	}

	// CLI flags override config file
	if *chillDir != "" {
		cfg.Music.ChillDir = config.ExpandHome(*chillDir)
	}
	if *driveDir != "" {
		cfg.Music.DriveDir = config.ExpandHome(*driveDir)
	}
	if *listenAddr != "" {
		cfg.Telemetry.Addr = *listenAddr
	}
	if *driveSpeed > 0 {
		cfg.Telemetry.DriveSpeedMph = *driveSpeed
	}
	if *driveThrottle > 0 {
		cfg.Telemetry.DriveThrottlePct = *driveThrottle
	}
	if *idleSpeed > 0 {
		cfg.Telemetry.IdleSpeedMph = *idleSpeed
	}
	if *cooldownSec > 0 {
		cfg.Telemetry.IdleCooldownSec = *cooldownSec
	}
	if *fadeSec > 0 {
		cfg.Audio.FadeSec = *fadeSec
	}

	logger.Info("Starting Forza Radio",
		"chill_dir", cfg.Music.ChillDir,
		"drive_dir", cfg.Music.DriveDir,
		"telemetry_addr", cfg.Telemetry.Addr,
		"fade_duration", fmt.Sprintf("%.1fs", cfg.Audio.FadeSec),
		"cooldown", fmt.Sprintf("%.1fs", cfg.Telemetry.IdleCooldownSec),
	)

	// Validate music directories
	if _, err := os.Stat(cfg.Music.ChillDir); os.IsNotExist(err) {
		logger.Error("Chill music directory does not exist", "path", cfg.Music.ChillDir)
		os.Exit(1)
	}
	if _, err := os.Stat(cfg.Music.DriveDir); os.IsNotExist(err) {
		logger.Error("Drive music directory does not exist", "path", cfg.Music.DriveDir)
		os.Exit(1)
	}

	// Initialize Playlists & Decks
	chillDeck, err := player.NewDeck("Chill", cfg.Music.ChillDir, logger)
	if err != nil {
		logger.Error("Failed to initialize Chill deck", "err", err)
		os.Exit(1)
	}
	defer chillDeck.Close()

	driveDeck, err := player.NewDeck("Drive", cfg.Music.DriveDir, logger)
	if err != nil {
		logger.Error("Failed to initialize Drive deck", "err", err)
		os.Exit(1)
	}
	defer driveDeck.Close()

	// Initialize Dual-Deck Mixer with Equal-Power Crossfade
	fadeDuration := time.Duration(cfg.Audio.FadeSec * float64(time.Second))
	sampleRate := cfg.Audio.SampleRate
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	mixer := player.NewMixer(chillDeck, driveDeck, fadeDuration, sampleRate, logger)

	// Initialize Audio Engine via Oto / PipeWire
	otoOpts := &oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}
	otoCtx, readyChan, err := oto.NewContext(otoOpts)
	if err != nil {
		logger.Error("Failed to initialize PipeWire/audio context", "err", err)
		os.Exit(1)
	}
	<-readyChan

	audioPlayer := otoCtx.NewPlayer(mixer)
	audioPlayer.Play()
	defer audioPlayer.Close()
	logger.Info("Audio output active (streaming to PipeWire)")

	// Setup Telemetry State Machine
	smConfig := telemetry.Config{
		DriveSpeedThreshold:    cfg.Telemetry.DriveSpeedMph,
		DriveThrottleThreshold: cfg.Telemetry.DriveThrottlePct,
		IdleSpeedThreshold:     cfg.Telemetry.IdleSpeedMph,
		IdleCooldown:           time.Duration(cfg.Telemetry.IdleCooldownSec * float64(time.Second)),
	}
	sm := telemetry.NewStateMachine(smConfig)

	// Listen for telemetry
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	packetChan := make(chan *telemetry.Packet, 64)
	listener := telemetry.NewListener(cfg.Telemetry.Addr, logger)

	go func() {
		if err := listener.Listen(ctx, packetChan); err != nil {
			logger.Error("Telemetry listener stopped with error", "err", err)
		}
	}()

	logger.Info("Ready for Forza Horizon telemetry. Waiting for packets...")

	// Telemetry event loop
	lastLogTime := time.Time{}
	for {
		select {
		case <-sigChan:
			logger.Info("Shutdown signal received, exiting gracefully...")
			cancel()
			return

		case pkt := <-packetChan:
			now := time.Now()
			newState, changed := sm.Process(pkt, now)

			if changed {
				mixer.SetTargetState(newState)
				logger.Info("Driving state changed",
					"state", newState,
					"speed_mph", fmt.Sprintf("%.1f", pkt.SpeedMph()),
					"throttle_pct", fmt.Sprintf("%.0f%%", pkt.ThrottlePercent()),
					"chill_track", chillDeck.CurrentTitle(),
					"drive_track", driveDeck.CurrentTitle(),
				)
			} else if *debug && now.Sub(lastLogTime) > 2*time.Second {
				lastLogTime = now
				logger.Debug("Telemetry heartbeat",
					"state", sm.CurrentState(),
					"speed_mph", fmt.Sprintf("%.1f", pkt.SpeedMph()),
					"rpm", pkt.CurrentEngineRPM,
				)
			}
		}
	}
}
