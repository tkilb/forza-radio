package telemetry

import (
	"testing"
	"time"
)

func TestStateMachine(t *testing.T) {
	cfg := Config{
		DriveSpeedThreshold:    10.0,
		DriveThrottleThreshold: 25.0,
		IdleSpeedThreshold:     5.0,
		IdleCooldown:           3 * time.Second,
	}

	sm := NewStateMachine(cfg)
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		speed             float32
		throttle          uint8
		elapsed           time.Duration
		expectedState     State
		expectedChanged   bool
	}{
		{
			name:            "start at standby",
			speed:           0.0,
			throttle:        0,
			elapsed:         0,
			expectedState:   StateStandby,
			expectedChanged: false,
		},
		{
			name:            "remain in standby below drive threshold",
			speed:           float32(4.0 / SpeedConversionMpsToMph), // ~4 mph
			throttle:        20,                                     // ~7.8% < 25%
			elapsed:         500 * time.Millisecond,
			expectedState:   StateStandby,
			expectedChanged: false,
		},
		{
			name:            "cross drive speed threshold from standby",
			speed:           float32(12.0 / SpeedConversionMpsToMph), // ~12 mph
			throttle:        0,
			elapsed:         1 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: true,
		},
		{
			name:            "decelerate below idle threshold, starts cooldown",
			speed:           float32(2.0 / SpeedConversionMpsToMph), // ~2 mph
			throttle:        0,
			elapsed:         2 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: false, // In cooldown, still DRIVE
		},
		{
			name:            "tap throttle during cooldown cancels idle switch",
			speed:           float32(3.0 / SpeedConversionMpsToMph),
			throttle:        100, // 100/255 is ~39% > 25%
			elapsed:         3 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: false,
		},
		{
			name:            "stop completely again",
			speed:           0.0,
			throttle:        0,
			elapsed:         4 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: false, // New cooldown begins at t=4s
		},
		{
			name:            "still in cooldown at t=6s (2s < 3s)",
			speed:           0.0,
			throttle:        0,
			elapsed:         6 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: false,
		},
		{
			name:            "cooldown expired at t=7.1s (> 3s)",
			speed:           0.0,
			throttle:        0,
			elapsed:         7100 * time.Millisecond,
			expectedState:   StateIdle,
			expectedChanged: true,
		},
		{
			name:            "resume driving from idle",
			speed:           float32(15.0 / SpeedConversionMpsToMph),
			throttle:        0,
			elapsed:         8 * time.Second,
			expectedState:   StateDrive,
			expectedChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := &Packet{
				HasDashData: true,
				SpeedMps:    tt.speed,
				Throttle:    tt.throttle,
			}
			now := baseTime.Add(tt.elapsed)
			gotState, gotChanged := sm.Process(pkt, now)

			if gotState != tt.expectedState {
				t.Errorf("Process() gotState = %v, want %v", gotState, tt.expectedState)
			}
			if gotChanged != tt.expectedChanged {
				t.Errorf("Process() gotChanged = %v, want %v", gotChanged, tt.expectedChanged)
			}
		})
	}
}
