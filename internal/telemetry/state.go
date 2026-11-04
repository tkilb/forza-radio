package telemetry

import (
	"time"
)

// State represents the vehicle driving mode.
type State string

const (
	StateStandby State = "STANDBY"
	StateIdle    State = "IDLE"
	StateDrive   State = "DRIVE"
)

// Config configures the telemetry thresholds and cooldown timings.
type Config struct {
	DriveSpeedThreshold    float64       // Speed in mph to trigger DRIVE
	DriveThrottleThreshold float64       // Throttle percent to trigger DRIVE
	IdleSpeedThreshold     float64       // Speed in mph below which cooldown starts
	IdleCooldown           time.Duration // Duration vehicle must stay idle before switching to IDLE
}

// DefaultConfig provides sensible defaults for Forza Horizon.
func DefaultConfig() Config {
	return Config{
		DriveSpeedThreshold:    10.0,
		DriveThrottleThreshold: 25.0,
		IdleSpeedThreshold:     5.0,
		IdleCooldown:           4 * time.Second,
	}
}

// StateMachine monitors telemetry packets and determines whether the vehicle is IDLE or in DRIVE.
type StateMachine struct {
	cfg           Config
	currentState  State
	idleStartTime time.Time
}

// NewStateMachine creates a new state machine initialized in StateStandby.
func NewStateMachine(cfg Config) *StateMachine {
	return &StateMachine{
		cfg:          cfg,
		currentState: StateStandby,
	}
}

// CurrentState returns the current state.
func (sm *StateMachine) CurrentState() State {
	return sm.currentState
}

// Process evaluates a telemetry packet and returns the state along with whether a transition occurred.
func (sm *StateMachine) Process(pkt *Packet, now time.Time) (State, bool) {
	speed := pkt.SpeedMph()
	throttle := pkt.ThrottlePercent()

	switch sm.currentState {
	case StateStandby:
		// Standby until first drive mode is triggered
		if speed >= sm.cfg.DriveSpeedThreshold || throttle >= sm.cfg.DriveThrottleThreshold {
			sm.currentState = StateDrive
			sm.idleStartTime = time.Time{}
			return sm.currentState, true
		}
		return sm.currentState, false

	case StateIdle:
		// Trigger drive immediately if speed or throttle passes the threshold
		if speed >= sm.cfg.DriveSpeedThreshold || throttle >= sm.cfg.DriveThrottleThreshold {
			sm.currentState = StateDrive
			sm.idleStartTime = time.Time{}
			return sm.currentState, true
		}
		return sm.currentState, false

	case StateDrive:
		// If vehicle is moving or throttle is applied, reset cooldown
		if speed >= sm.cfg.IdleSpeedThreshold || throttle >= sm.cfg.DriveThrottleThreshold {
			sm.idleStartTime = time.Time{}
			return sm.currentState, false
		}

		// Vehicle is stopped / below idle threshold
		if sm.idleStartTime.IsZero() {
			sm.idleStartTime = now
			return sm.currentState, false
		}

		// Check if cooldown window has elapsed
		if now.Sub(sm.idleStartTime) >= sm.cfg.IdleCooldown {
			sm.currentState = StateIdle
			sm.idleStartTime = time.Time{}
			return sm.currentState, true
		}

		return sm.currentState, false

	default:
		sm.currentState = StateStandby
		return sm.currentState, false
	}
}
