//go:build !darwin && cgo

package gui

import (
	"context"
	"errors"
)

const wifiDiagnosticsAvailable = false

func measureWiFi(context.Context) (wifiSample, error) {
	return wifiSample{}, errors.New("Wi-Fi diagnostics are available on macOS")
}
