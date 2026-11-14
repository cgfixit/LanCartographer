//go:build cgo

package gui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation
#import <CoreWLAN/CoreWLAN.h>
#include <net/if.h>

typedef struct {
	char name[IFNAMSIZ];
	long rssi, noise, channel, band;
	double tx;
} WiFiLink;

static WiFiLink sampleWiFi(void) {
	@autoreleasepool {
		WiFiLink link = {0};
		CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
		NSString *name = iface.interfaceName;
		if ([name getCString:link.name maxLength:sizeof(link.name) encoding:NSUTF8StringEncoding]) {
			CWChannel *channel = [iface wlanChannel];
			link.rssi = [iface rssiValue];
			link.noise = [iface noiseMeasurement];
			link.channel = channel.channelNumber;
			link.band = channel.channelBand;
			link.tx = [iface transmitRate];
		}
		return link;
	}
}
*/
import "C"

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const wifiDiagnosticsAvailable = true

func measureWiFi(ctx context.Context) (wifiSample, error) {
	link := C.sampleWiFi()
	sample := wifiSample{
		at: time.Now(), iface: C.GoString(&link.name[0]), signal: int(link.rssi),
		noise: int(link.noise), channel: int(link.channel), tx: float64(link.tx),
	}
	if !validDiagnosticInterface(sample.iface) || sample.signal >= 0 {
		return wifiSample{}, errors.New("No active Wi-Fi link. Join a Wi-Fi network and try again")
	}
	switch link.band {
	case C.kCWChannelBand2GHz:
		sample.band = "2.4 GHz"
	case C.kCWChannelBand5GHz:
		sample.band = "5 GHz"
	case C.kCWChannelBand6GHz:
		sample.band = "6 GHz"
	}
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	sample.gateway = wifiGateway(ctx, sample.iface)
	if !sample.gateway.IsValid() {
		sample.gatewayStatus = "No Wi-Fi default gateway found"
		return sample, nil
	}
	command, bindFlag := "/sbin/ping", "-b"
	if sample.gateway.Is6() {
		command, bindFlag = "/sbin/ping6", "-B"
	}
	output, err := runDiagnosticCommand(ctx, command, "-n", "-q", bindFlag, sample.iface,
		"-c", strconv.Itoa(gatewayProbeCount), sample.gateway.String())
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 2) {
		sample.gatewayStatus = "Latency unavailable"
		return sample, nil
	}
	replies, average, ok := parseGatewayPing(string(output))
	if !ok {
		sample.gatewayStatus = "Latency unavailable"
	} else if wifiGateway(ctx, sample.iface) != sample.gateway {
		sample.gatewayStatus = "Could not verify the gateway stayed the same"
		sample.gateway = netip.Addr{}
	} else {
		sample.replies, sample.averageMS = replies, average
	}
	return sample, nil
}

func wifiGateway(ctx context.Context, iface string) netip.Addr {
	for _, family := range []string{"-inet", "-inet6"} {
		output, err := runDiagnosticCommand(ctx, "/sbin/route", "-n", "get", family, "-ifscope", iface, "default")
		if err == nil {
			addr := parseWiFiGateway(string(output), iface)
			if addr.IsValid() && addr.Is4() == (family == "-inet") {
				return addr
			}
		}
	}
	return netip.Addr{}
}

func parseWiFiGateway(output, iface string) netip.Addr {
	var addr netip.Addr
	var routeInterface string
	for line := range strings.SplitSeq(output, "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), ":")
		switch key {
		case "gateway":
			addr, _ = netip.ParseAddr(strings.TrimSpace(value))
		case "interface":
			routeInterface = strings.TrimSpace(value)
		}
	}
	if routeInterface != iface || addr.Is4In6() || addr.IsLoopback() ||
		!(addr.IsGlobalUnicast() || addr.IsLinkLocalUnicast()) ||
		(addr.Zone() != "" && addr.Zone() != iface) {
		return netip.Addr{}
	}
	if addr.Is6() && addr.IsLinkLocalUnicast() {
		addr = addr.WithZone(iface)
	}
	return addr
}

var gatewayCounts = regexp.MustCompile(`(?m)^([0-9]+) packets transmitted, ([0-9]+) packets received,`)
var gatewayRTT = regexp.MustCompile(`(?m)^round-trip min/avg/max/std-?dev = [0-9.]+/([0-9.]+)/`)

func parseGatewayPing(output string) (replies int, average float64, ok bool) {
	counts := gatewayCounts.FindStringSubmatch(output)
	if counts == nil || counts[1] != strconv.Itoa(gatewayProbeCount) {
		return 0, 0, false
	}
	replies, err := strconv.Atoi(counts[2])
	if err != nil || replies > gatewayProbeCount {
		return 0, 0, false
	}
	if replies == 0 {
		return 0, 0, true
	}
	rtt := gatewayRTT.FindStringSubmatch(output)
	if rtt == nil {
		return 0, 0, false
	}
	average, err = strconv.ParseFloat(rtt[1], 64)
	return replies, average, err == nil && !math.IsInf(average, 0)
}
