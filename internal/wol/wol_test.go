package wol

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

var testMAC = net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

func TestMagicPacketLength(t *testing.T) {
	pkt, err := MagicPacket(testMAC)
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("packet length = %d, want 102", len(pkt))
	}
}

func TestMagicPacketPrefix(t *testing.T) {
	pkt, err := MagicPacket(testMAC)
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("pkt[%d] = %#x, want 0xff", i, pkt[i])
		}
	}
}

func TestMagicPacketBody(t *testing.T) {
	pkt, err := MagicPacket(testMAC)
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}
	for rep := 0; rep < 16; rep++ {
		off := 6 + rep*6
		if !bytes.Equal(pkt[off:off+6], testMAC) {
			t.Fatalf("repetition %d = %x, want %x", rep, pkt[off:off+6], testMAC)
		}
	}
}

func TestMagicPacketWrongLengthMAC(t *testing.T) {
	for _, bad := range []net.HardwareAddr{
		nil,
		{},
		{1, 2, 3, 4, 5},
		{1, 2, 3, 4, 5, 6, 7},
	} {
		if pkt, err := MagicPacket(bad); err == nil {
			t.Fatalf("MagicPacket(%x) = %d bytes, nil error; want error", bad, len(pkt))
		}
	}
}

func TestSendDeliversPacket(t *testing.T) {
	pkt, err := MagicPacket(testMAC)
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}

	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Send(ctx, pkt, pc.LocalAddr().String()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if !bytes.Equal(buf[:n], pkt) {
		t.Fatalf("received %d bytes %x, want %d bytes %x", n, buf[:n], len(pkt), pkt)
	}
}

func TestSendCancelledContext(t *testing.T) {
	pkt, err := MagicPacket(testMAC)
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Send(ctx, pkt, "127.0.0.1:9"); err == nil {
		t.Fatal("Send with cancelled context = nil error, want error")
	}
}
