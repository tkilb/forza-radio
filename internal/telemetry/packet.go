package telemetry

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	// PacketSizeSled is the V1 telemetry packet size (232 bytes).
	PacketSizeSled = 232
	// PacketSizeFH4Dash is the Forza Horizon 4/5 Dash packet size (324 bytes).
	PacketSizeFH4Dash = 324
	// PacketSizeFM7Dash is the Forza Motorsport 7 Dash packet size (311 bytes).
	PacketSizeFM7Dash = 311

	// SpeedConversionMpsToMph converts meters/second to miles per hour.
	SpeedConversionMpsToMph = 2.23693629
	// SpeedConversionMpsToKmh converts meters/second to kilometers per hour.
	SpeedConversionMpsToKmh = 3.6
)

var (
	// ErrPacketTooShort is returned when an incoming UDP buffer is smaller than the minimum packet size.
	ErrPacketTooShort = errors.New("telemetry packet smaller than minimum format (232 bytes)")
)

// Packet represents a parsed Forza Horizon Data Out telemetry frame.
type Packet struct {
	IsRaceOn         bool
	TimestampMS      uint32
	EngineMaxRPM     float32
	EngineIdleRPM    float32
	CurrentEngineRPM float32

	AccelerationX float32
	AccelerationY float32
	AccelerationZ float32

	VelocityX float32
	VelocityY float32
	VelocityZ float32

	// Dash format specific fields (populated if packet >= 311 bytes)
	HasDashData bool
	PositionX   float32
	PositionY   float32
	PositionZ   float32
	SpeedMps    float32 // Meters per second
	Throttle    uint8   // 0 - 255
	Brake       uint8   // 0 - 255
	Clutch      uint8   // 0 - 255
	Handbrake   uint8   // 0 - 255
	Gear        uint8   // 0 = reverse, 1-15 = forward
	Steer       int8    // -127 to 127
}

// SpeedMph returns vehicle speed in miles per hour.
func (p *Packet) SpeedMph() float64 {
	if p.HasDashData && p.SpeedMps > 0 {
		return float64(p.SpeedMps) * SpeedConversionMpsToMph
	}
	// Fallback to velocity vector magnitude (works for Sled and Dash)
	speedMps := math.Sqrt(float64(p.VelocityX*p.VelocityX + p.VelocityY*p.VelocityY + p.VelocityZ*p.VelocityZ))
	return speedMps * SpeedConversionMpsToMph
}

// SpeedKmh returns vehicle speed in kilometers per hour.
func (p *Packet) SpeedKmh() float64 {
	return p.SpeedMph() * 1.60934
}

// ThrottlePercent returns throttle input as a percentage (0.0 - 100.0).
func (p *Packet) ThrottlePercent() float64 {
	if !p.HasDashData {
		return 0.0
	}
	return (float64(p.Throttle) / 255.0) * 100.0
}

// ParsePacket decodes raw little-endian binary data from Forza's Data Out UDP stream.
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < PacketSizeSled {
		return nil, fmt.Errorf("%w: got %d bytes", ErrPacketTooShort, len(data))
	}

	reader := bytes.NewReader(data)
	var isRaceOnInt int32
	if err := binary.Read(reader, binary.LittleEndian, &isRaceOnInt); err != nil {
		return nil, fmt.Errorf("reading IsRaceOn: %w", err)
	}

	pkt := &Packet{
		IsRaceOn: isRaceOnInt != 0,
	}

	if err := binary.Read(reader, binary.LittleEndian, &pkt.TimestampMS); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.EngineMaxRPM); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.EngineIdleRPM); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.CurrentEngineRPM); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.AccelerationX); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.AccelerationY); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.AccelerationZ); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.VelocityX); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.VelocityY); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &pkt.VelocityZ); err != nil {
		return nil, err
	}

	// For Dash packets (FH4/FH5 = 324 bytes, FM7 = 311 bytes)
	if len(data) >= PacketSizeFH4Dash {
		pkt.HasDashData = true
		// Dash data in FH4/5 starts around offset 244
		dashReader := bytes.NewReader(data[244:])
		_ = binary.Read(dashReader, binary.LittleEndian, &pkt.PositionX)
		_ = binary.Read(dashReader, binary.LittleEndian, &pkt.PositionY)
		_ = binary.Read(dashReader, binary.LittleEndian, &pkt.PositionZ)
		_ = binary.Read(dashReader, binary.LittleEndian, &pkt.SpeedMps)

		// Throttle, Brake, Clutch, Handbrake, Gear, Steer are at offset 315..320
		pkt.Throttle = data[315]
		pkt.Brake = data[316]
		pkt.Clutch = data[317]
		pkt.Handbrake = data[318]
		pkt.Gear = data[319]
		pkt.Steer = int8(data[320])
	}

	return pkt, nil
}
