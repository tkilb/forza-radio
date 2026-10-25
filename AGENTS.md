# AGENTS.md

Instructions, architecture notes, and conventions for AI assistants working on `forza-radio`.

---

## 1. Project Overview

`forza-radio` is an adaptive, telemetry-driven background music daemon for Forza Horizon 4 & 5 running on Linux via PipeWire. It ingests real-time UDP telemetry broadcasted by Forza ("Data Out") and crossfades between two independent audio decks:
- **Deck A (Chill)**: Ambient/low-tempo tracks for idling, parking, or scenic cruising.
- **Deck B (Drive)**: High-energy tracks for racing and fast driving.

When switching states, the inactive deck's volume ramps down and the deck is paused where it left off, resuming seamlessly when triggered again.

---

## 2. Architecture & Internal Packages

```
                      +-----------------------------+
                      |   Forza Horizon (Proton)   |
                      |    UDP Telemetry Broadcast  |
                      +--------------+--------------+
                                     | UDP (e.g. 127.0.0.1:5300)
                                     v
                      +-----------------------------+
                      |       Telemetry Ingestion   |
                      |  - Binary packet decoding   |
                      |  - State: IDLE vs. DRIVE    |
                      |  - Cooldown timer (4.0s)    |
                      +--------------+--------------+
                                     | State Event (Target Volumes)
                                     v
                      +-----------------------------+
                      |     Dual-Deck Audio Mixer   |
                      |  Deck A: Chill MP3 Playlist |
                      |  Deck B: Drive MP3 Playlist |
                      |  - MP3 Decoding (hajimehoshi/go-mp3)
                      |  - Smooth volume ramp/fader |
                      +--------------+--------------+
                                     | Mixed PCM Stream
                                     v
                      +-----------------------------+
                      |     PipeWire Output Sink    |
                      |   (via Oto / Pulse / ALSA)  |
                      +-----------------------------+
```

### Components
- **`internal/telemetry/packet.go`**: Parses the binary 324-byte (V2) or 311-byte (V1) Forza Horizon "Data Out" UDP packet.
- **`internal/telemetry/state.go`**: State machine managing `IDLE` vs `DRIVE` states with hysteresis:
  - Transition to `DRIVE`: Instant trigger when `Speed > driveSpeed` (default: 10 mph) OR `Throttle > driveThrottle` (default: 25%).
  - Transition to `IDLE`: Requires `Speed < idleSpeed` (default: 5 mph) AND `Throttle == 0` sustained for `cooldownDuration` (default: 4.0s).
- **`internal/player/deck.go`**: Manages directory playback, track shuffling/sequential queue, and pause/resume at PCM sample level.
- **`internal/player/mixer.go`**: In-memory PCM mixer combining Deck A and Deck B with smooth crossfading/volume ramping over a configurable duration (default: 2.0s).
- **`cmd/forza-radio/main.go`**: Daemon entry point wiring UDP listener, player engine, and Oto/PipeWire output sink.
- **`cmd/forza-mock/main.go`**: Standalone mock telemetry generator broadcasting a 35s simulated driving cycle for offline verification.

---

## 3. Conventions & Guidelines

- **Go Standards**: Idiomatic Go, explicit error handling, no panics in library code, table-driven unit tests.
- **Testing**: Maintain unit tests with `go test ./...`. When modifying telemetry parsing or state logic, ensure mock simulator and test suites remain aligned.
- **Human-in-the-Loop Rules**:
  - Do not run commands that mutate state or interact with external systems without explicit confirmation.
  - Never commit changes automatically; the user commits manually.
- **Tools**: Prefer scoped agent file tools (`read_file`, `edit_file`, `write_file`) over raw shell execution for file manipulations.
