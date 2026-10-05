// Package wol builds and sends Wake-on-LAN magic packets over UDP.
package wol

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// PacketLen is the exact size of a magic packet: a 6-byte 0xFF prefix plus
// 16 repetitions of the 6-byte target MAC.
const PacketLen = 102

// MagicPacket builds the 102-byte magic packet for mac. It errors when mac
// is not exactly 6 bytes.
func MagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("wol: MAC must be 6 bytes, got %d", len(mac))
	}
	pkt := make([]byte, PacketLen)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for rep := 0; rep < 16; rep++ {
		copy(pkt[6+rep*6:], mac)
	}
	return pkt, nil
}

// Send writes pkt once to the UDP address broadcast (host:port, e.g.
// "255.255.255.255:9"). The dial and write are bounded by ctx; a missing
// deadline gets a short default so Send never blocks forever.
func Send(ctx context.Context, pkt []byte, broadcast string) error {
	if len(pkt) == 0 {
		return errors.New("wol: empty packet")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	addr, err := net.ResolveUDPAddr("udp", broadcast)
	if err != nil {
		return fmt.Errorf("wol: resolve %q: %w", broadcast, err)
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr.String())
	if err != nil {
		return fmt.Errorf("wol: dial %q: %w", broadcast, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("wol: set write deadline: %w", err)
		}
	}
	if _, err := conn.Write(pkt); err != nil {
		return fmt.Errorf("wol: write to %q: %w", broadcast, err)
	}
	return nil
}
