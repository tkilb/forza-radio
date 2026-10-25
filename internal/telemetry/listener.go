package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// Listener listens on a UDP port for Forza Horizon telemetry packets.
type Listener struct {
	addr   string
	logger *slog.Logger
}

// NewListener creates a new telemetry UDP listener.
func NewListener(addr string, logger *slog.Logger) *Listener {
	if logger == nil {
		logger = slog.Default()
	}
	return &Listener{
		addr:   addr,
		logger: logger,
	}
}

// Listen starts receiving telemetry packets and passes them to packetChan until context is canceled.
func (l *Listener) Listen(ctx context.Context, packetChan chan<- *Packet) error {
	pc, err := net.ListenPacket("udp", l.addr)
	if err != nil {
		return fmt.Errorf("failed to bind UDP listener on %s: %w", l.addr, err)
	}
	defer pc.Close()

	l.logger.Info("Listening for Forza telemetry", "addr", l.addr)

	buf := make([]byte, 1024)
	go func() {
		<-ctx.Done()
		pc.Close()
	}()

	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				l.logger.Debug("UDP read error", "err", err)
				continue
			}
		}

		pkt, err := ParsePacket(buf[:n])
		if err != nil {
			l.logger.Debug("Malformed telemetry packet", "err", err, "bytes", n)
			continue
		}

		select {
		case packetChan <- pkt:
		case <-ctx.Done():
			return nil
		default:
			// Non-blocking drop to avoid blocking UDP receive loop if consumer is slow
		}
	}
}
