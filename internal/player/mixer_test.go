package player

import (
	"math"
	"testing"
)

func TestAdvanceGain(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		target   float64
		step     float64
		expected float64
	}{
		{
			name:     "ramp up partial",
			current:  0.0,
			target:   1.0,
			step:     0.25,
			expected: 0.25,
		},
		{
			name:     "ramp up clamp to target",
			current:  0.9,
			target:   1.0,
			step:     0.25,
			expected: 1.0,
		},
		{
			name:     "ramp down partial",
			current:  1.0,
			target:   0.0,
			step:     0.25,
			expected: 0.75,
		},
		{
			name:     "ramp down clamp to target",
			current:  0.1,
			target:   0.0,
			step:     0.25,
			expected: 0.0,
		},
		{
			name:     "already at target",
			current:  1.0,
			target:   1.0,
			step:     0.1,
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := advanceGain(tt.current, tt.target, tt.step)
			if math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("advanceGain() = %v, want %v", got, tt.expected)
			}
		})
	}
}
