package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"forza-radio/internal/telemetry"
)

func main() {
	targetAddr := flag.String("addr", "127.0.0.1:5300", "Destination UDP address for mock telemetry")
	flag.Parse()

	conn, err := net.Dial("udp", *targetAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dialing UDP target: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Broadcasting mock Forza telemetry to %s (Ctrl+C to stop)...\n", *targetAddr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(16 * time.Millisecond) // ~60 Hz telemetry
	defer ticker.Stop()

	t := 0.0
	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopping mock telemetry broadcast.")
			return
		case <-ticker.C:
			t += 0.016

			// Simulate a driving cycle:
			// 0s-5s: Idle / parked (speed = 0)
			// 5s-20s: Accelerate up to 90 mph
			// 20s-25s: Brake to full stop
			// 25s-35s: Idle
			cycle := math.Mod(t, 35.0)

			var speedMps float32
			var throttle uint8

			if cycle < 5.0 {
				// Idle
				speedMps = 0.0
				throttle = 0
			} else if cycle < 20.0 {
				// Driving
				progress := (cycle - 5.0) / 15.0
				speedMph := progress * 95.0
				speedMps = float32(speedMph / telemetry.SpeedConversionMpsToMph)
				throttle = 255
			} else if cycle < 25.0 {
				// Braking to stop
				progress := 1.0 - ((cycle - 20.0) / 5.0)
				speedMph := progress * 95.0
				speedMps = float32(speedMph / telemetry.SpeedConversionMpsToMph)
				throttle = 0
			} else {
				// Idle again
				speedMps = 0.0
				throttle = 0
			}

			packet := makeFH4Packet(speedMps, throttle)
			if _, err := conn.Write(packet); err != nil {
				fmt.Fprintf(os.Stderr, "Error sending mock packet: %v\n", err)
			}
		}
	}
}

func makeFH4Packet(speedMps float32, throttle uint8) []byte {
	buf := make([]byte, telemetry.PacketSizeFH4Dash)
	// IsRaceOn = 1
	binary.LittleEndian.PutUint32(buf[0:4], 1)
	// Speed at offset 256
	binary.LittleEndian.PutUint32(buf[256:260], math.Float32bits(speedMps))
	// Throttle at offset 315
	buf[315] = throttle
	return buf
}
