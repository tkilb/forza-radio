#!/usr/bin/env bash
set -euo pipefail

# Ensure ~/bin is in PATH for Steam container/runtime
export PATH="$HOME/bin:$PATH"

BIN="$(command -v forza-radio || echo "$HOME/bin/forza-radio")"

# Start daemon in background (reads ~/.config/forza-radio/config.yaml by default)
mkdir -p "$HOME/.local/state"
"$BIN" > "$HOME/.local/state/forza-radio.log" 2>&1 &
RADIO_PID=$!

cleanup() {
    if kill -0 "$RADIO_PID" 2>/dev/null; then
        kill "$RADIO_PID" 2>/dev/null || true
    fi
}

trap cleanup EXIT INT TERM

# Run game command if passed
if [ "$#" -gt 0 ]; then
    "$@"
else
    wait "$RADIO_PID"
fi
