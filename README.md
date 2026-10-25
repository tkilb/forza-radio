# Forza Radio

An adaptive, telemetry-driven background music daemon for **Forza Horizon 4 & 5** on Linux via PipeWire.

It listens to Forza's real-time UDP telemetry ("Data Out") and crossfades between two MP3 decks:
- **Chill Deck**: Ambient / low-tempo tracks while parked, cruising scenic vistas, or idling.
- **Drive Deck**: High-energy / racing tracks when accelerating, cornering, and driving fast.

When you transition between driving and idling, the inactive deck smoothly fades down and **pauses where you left off**, picking up seamlessly when you switch back.

---

## 1. Quick Start

### In-Game Setup (Forza Horizon 4 or 5)
1. Open Forza Horizon settings.
2. Navigate to **HUD and Gameplay**.
3. Scroll to the bottom to find **Data Out**:
   - **Data Out**: `ON`
   - **Data Out IP Address**: `127.0.0.1`
   - **Data Out IP Port**: `5300`

### Organize Your Music
Place your MP3 files into two folders (or use custom paths via flags):
```bash
~/Music/chill/   # Lo-fi, ambient, cruising tracks
~/Music/drive/   # High-energy, drum & bass, synthwave, metal, etc.
```

### Build & Install
```bash
# Install binaries and launcher to ~/bin
make install
```

### Steam Launch Option
Set the launch options in Steam for Forza Horizon:
```bash
forza-radio-launcher.sh %command%
```
*(Optionally override directory paths via `FORZA_CHILL_DIR` and `FORZA_DRIVE_DIR` environment variables if you don't use `~/Music/chill` and `~/Music/drive`)*.

---

## 2. Testing Without Forza (`forza-mock`)

You can test the crossfader, audio mixing, and state machine offline using the built-in telemetry simulator:

```bash
# Terminal 1: Run forza-radio
./forza-radio -chill-dir ~/Music/chill -drive-dir ~/Music/drive

# Terminal 2: Run mock telemetry generator
./forza-mock
```

`forza-mock` broadcasts a realistic 35-second driving cycle (idle $\rightarrow$ launch to 95 mph $\rightarrow$ brake to full stop $\rightarrow$ 4s cooldown $\rightarrow$ idle).

---

## 3. Configuration & CLI Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-chill-dir` | `~/Music/chill` | Directory containing chill/ambient MP3s |
| `-drive-dir` | `~/Music/drive` | Directory containing high-energy MP3s |
| `-addr` | `127.0.0.1:5300` | UDP address to bind for telemetry |
| `-drive-speed` | `10.0` | Speed in mph required to switch to Drive deck |
| `-drive-throttle` | `25.0` | Throttle percentage required to switch to Drive deck |
| `-idle-speed` | `5.0` | Speed in mph below which idle cooldown begins |
| `-cooldown` | `4.0` | Seconds vehicle must remain stopped before returning to Chill deck |
| `-fade` | `2.0` | Crossfade duration in seconds |
| `-debug` | `false` | Enable verbose telemetry heartbeat logs |

---

## 4. Architecture

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

- **`internal/telemetry`**: Decodes 324-byte (V2) or 311-byte (V1) little-endian Forza Horizon Data Out UDP packets. Tracks vehicle speed, throttle, and RPM with hysteresis to prevent audio flapping during cornering or gear shifts.
- **`internal/player`**: Dual-deck audio mixer using `github.com/hajimehoshi/go-mp3` for pure Go decoding and smooth volume ramping. Automatically pauses the inactive deck at 0% volume and resumes when switching back.
- **PipeWire Output**: Streams PCM frames via `github.com/ebitengine/oto/v3` directly into the user's default PipeWire sound sink.
