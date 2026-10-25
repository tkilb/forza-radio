package player

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

var (
	ErrNoTracks = errors.New("no MP3 tracks found in directory")
)

// Deck manages playback of an MP3 playlist, decoding to 16-bit stereo PCM.
type Deck struct {
	mu           sync.Mutex
	name         string
	dir          string
	files        []string
	currentIndex int
	currentFile  *os.File
	decoder      *mp3.Decoder
	currentTitle string
	paused       bool
	volume       float64 // 0.0 to 1.0
	logger       *slog.Logger
	sampleRate   int
}

// NewDeck creates a new deck pointing to a directory of MP3 files.
func NewDeck(name, dir string, logger *slog.Logger) (*Deck, error) {
	if logger == nil {
		logger = slog.Default()
	}

	d := &Deck{
		name:       name,
		dir:        dir,
		volume:     0.0,
		paused:     true,
		logger:     logger,
		sampleRate: 44100,
	}

	if err := d.scanTracks(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Deck) scanTracks() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var files []string
	err := filepath.Walk(d.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".mp3" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning %s directory %s: %w", d.name, d.dir, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("%w: %s", ErrNoTracks, d.dir)
	}

	// Shuffle tracks
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	d.files = files
	d.currentIndex = 0
	d.logger.Info("Loaded playlist", "deck", d.name, "tracks", len(files))
	return nil
}

// CurrentTitle returns the currently playing song title.
func (d *Deck) CurrentTitle() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.currentTitle
}

// SetVolume sets the gain multiplier (0.0 to 1.0).
func (d *Deck) SetVolume(v float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v < 0.0 {
		v = 0.0
	} else if v > 1.0 {
		v = 1.0
	}
	d.volume = v
}

// Volume returns the current volume gain multiplier.
func (d *Deck) Volume() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.volume
}

// SetPaused controls playback pausing.
func (d *Deck) SetPaused(paused bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.paused = paused
}

// IsPaused returns whether the deck is paused.
func (d *Deck) IsPaused() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.paused
}

// openCurrentTrack opens and initializes the decoder for the track at currentIndex.
// Must be called with d.mu held.
func (d *Deck) openCurrentTrack() error {
	if d.currentFile != nil {
		_ = d.currentFile.Close()
		d.currentFile = nil
		d.decoder = nil
	}

	if len(d.files) == 0 {
		return ErrNoTracks
	}

	filePath := d.files[d.currentIndex]
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening track %s: %w", filePath, err)
	}

	dec, err := mp3.NewDecoder(file)
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("decoding track %s: %w", filePath, err)
	}

	d.currentFile = file
	d.decoder = dec
	d.currentTitle = filepath.Base(filePath)
	d.sampleRate = dec.SampleRate()
	d.logger.Info("Now playing", "deck", d.name, "title", d.currentTitle, "rate", d.sampleRate)
	return nil
}

// advanceTrack moves to the next track in the playlist.
// Must be called with d.mu held.
func (d *Deck) advanceTrack() error {
	if len(d.files) == 0 {
		return ErrNoTracks
	}
	d.currentIndex = (d.currentIndex + 1) % len(d.files)
	return d.openCurrentTrack()
}

// ReadPCM reads up to len(buf) bytes of decoded 16-bit stereo PCM.
// If paused or volume is 0, it fills buf with silence and does not consume stream.
func (d *Deck) ReadPCM(buf []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.paused || d.volume <= 0.0001 {
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	if d.decoder == nil {
		if err := d.openCurrentTrack(); err != nil {
			return 0, err
		}
	}

	totalRead := 0
	for totalRead < len(buf) {
		n, err := d.decoder.Read(buf[totalRead:])
		totalRead += n

		if errors.Is(err, io.EOF) || (n == 0 && err != nil) {
			// Current track ended, advance to next
			if advErr := d.advanceTrack(); advErr != nil {
				return totalRead, advErr
			}
			continue
		}

		if err != nil {
			d.logger.Warn("Error reading audio frame", "deck", d.name, "err", err)
			return totalRead, err
		}
	}

	return totalRead, nil
}

// Close closes any open file handles.
func (d *Deck) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.currentFile != nil {
		err := d.currentFile.Close()
		d.currentFile = nil
		d.decoder = nil
		return err
	}
	return nil
}
