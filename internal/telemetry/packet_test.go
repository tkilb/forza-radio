package telemetry

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParsePacket(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		wantErr       bool
		expectedSpeed float64
		expectedRace  bool
	}{
		{
			name:    "packet too short",
			data:    make([]byte, 100),
			wantErr: true,
		},
		{
			name: "valid sled packet with velocity magnitude",
			data: func() []byte {
				buf := make([]byte, PacketSizeSled)
				// IsRaceOn = 1
				binary.LittleEndian.PutUint32(buf[0:4], 1)
				// VelocityX = 10.0 m/s at offset 32
				// Velocity offsets:
				// 0: IsRaceOn (4)
				// 4: Timestamp (4)
				// 8: MaxRPM (4)
				// 12: IdleRPM (4)
				// 16: CurrentRPM (4)
				// 20: AccelX (4)
				// 24: AccelY (4)
				// 28: AccelZ (4)
				// 32: VelX (4)
				// 36: VelY (4)
				// 40: VelZ (4)
				bits := math.Float32bits(10.0)
				binary.LittleEndian.PutUint32(buf[32:36], bits)
				return buf
			}(),
			wantErr:       false,
			expectedSpeed: 10.0 * SpeedConversionMpsToMph,
			expectedRace:  true,
		},
		{
			name: "valid fh4 dash packet with throttle and speed",
			data: func() []byte {
				buf := make([]byte, PacketSizeFH4Dash)
				binary.LittleEndian.PutUint32(buf[0:4], 1)
				// Speed at offset 256
				speedBits := math.Float32bits(25.0)
				binary.LittleEndian.PutUint32(buf[256:260], speedBits)
				// Throttle at offset 315
				buf[315] = 255 // 100%
				return buf
			}(),
			wantErr:       false,
			expectedSpeed: 25.0 * SpeedConversionMpsToMph,
			expectedRace:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := ParsePacket(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePacket() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if pkt.IsRaceOn != tt.expectedRace {
					t.Errorf("IsRaceOn = %v, want %v", pkt.IsRaceOn, tt.expectedRace)
				}
				if math.Abs(pkt.SpeedMph()-tt.expectedSpeed) > 0.01 {
					t.Errorf("SpeedMph() = %v, want %v", pkt.SpeedMph(), tt.expectedSpeed)
				}
			}
		})
	}
}
