package player

import (
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"forza-radio/internal/telemetry"
)

// Mixer mixes two decks (Chill and Drive) into a single 16-bit stereo PCM stream using equal-power crossfading.
type Mixer struct {
	mu           sync.Mutex
	chillDeck    *Deck
	driveDeck    *Deck
	targetState  telemetry.State
	faderPos     float64 // 0.0 = 100% Chill, 1.0 = 100% Drive
	fadeDuration time.Duration
	sampleRate   int
	logger       *slog.Logger

	// Temporary buffers to avoid allocations in Read
	chillBuf []byte
	driveBuf []byte
}

// NewMixer creates a new audio mixer for Chill and Drive decks.
func NewMixer(chillDeck, driveDeck *Deck, fadeDuration time.Duration, sampleRate int, logger *slog.Logger) *Mixer {
	if logger == nil {
		logger = slog.Default()
	}
	if fadeDuration <= 0 {
		fadeDuration = 2 * time.Second
	}
	if sampleRate <= 0 {
		sampleRate = 44100
	}

	m := &Mixer{
		chillDeck:    chillDeck,
		driveDeck:    driveDeck,
		targetState:  telemetry.StateIdle,
		faderPos:     0.0, // Start with Chill active
		fadeDuration: fadeDuration,
		sampleRate:   sampleRate,
		logger:       logger,
	}

	chillDeck.SetVolume(1.0)
	chillDeck.SetPaused(false)
	driveDeck.SetVolume(0.0)
	driveDeck.SetPaused(true)

	return m
}

// SetTargetState switches target state between StateIdle and StateDrive.
func (m *Mixer) SetTargetState(state telemetry.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.targetState != state {
		m.targetState = state
		m.logger.Info("Target audio state changed", "state", state)
	}
}

// Read implements io.Reader to feed the audio device (Oto / PipeWire).
func (m *Mixer) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nBytes := len(p)
	if nBytes == 0 {
		return 0, nil
	}

	if len(m.chillBuf) < nBytes {
		m.chillBuf = make([]byte, nBytes)
		m.driveBuf = make([]byte, nBytes)
	}

	numFrames := nBytes / 4
	bufferDuration := time.Duration(float64(numFrames) / float64(m.sampleRate) * float64(time.Second))
	step := float64(bufferDuration) / float64(m.fadeDuration)

	// Target fader position: 0.0 for IDLE (Chill), 1.0 for DRIVE
	var targetPos float64
	if m.targetState == telemetry.StateDrive {
		targetPos = 1.0
	} else {
		targetPos = 0.0
	}

	m.faderPos = advanceGain(m.faderPos, targetPos, step)

	// Equal-power crossfade curve: cos for chill, sin for drive
	angle := m.faderPos * (math.Pi / 2.0)
	chillGain := math.Cos(angle)
	driveGain := math.Sin(angle)

	// Pause/unpause decks based on volume thresholds
	m.chillDeck.SetVolume(chillGain)
	m.chillDeck.SetPaused(chillGain <= 0.001)

	m.driveDeck.SetVolume(driveGain)
	m.driveDeck.SetPaused(driveGain <= 0.001)

	// Read decoded PCM from each deck
	_, errA := m.chillDeck.ReadPCM(m.chillBuf[:nBytes])
	if errA != nil && errA != io.EOF {
		m.logger.Debug("Chill deck read error", "err", errA)
	}
	_, errB := m.driveDeck.ReadPCM(m.driveBuf[:nBytes])
	if errB != nil && errB != io.EOF {
		m.logger.Debug("Drive deck read error", "err", errB)
	}

	// Mix 16-bit signed stereo samples with equal-power gains
	for i := 0; i < nBytes; i += 2 {
		sA := int16(binary.LittleEndian.Uint16(m.chillBuf[i : i+2]))
		sB := int16(binary.LittleEndian.Uint16(m.driveBuf[i : i+2]))

		mixed := float64(sA)*chillGain + float64(sB)*driveGain

		if mixed > 32767 {
			mixed = 32767
		} else if mixed < -32768 {
			mixed = -32768
		}

		binary.LittleEndian.PutUint16(p[i:i+2], uint16(int16(mixed)))
	}

	return nBytes, nil
}

func advanceGain(current, target, step float64) float64 {
	if current < target {
		current += step
		if current > target {
			current = target
		}
	} else if current > target {
		current -= step
		if current < target {
			current = target
		}
	}
	return current
}
