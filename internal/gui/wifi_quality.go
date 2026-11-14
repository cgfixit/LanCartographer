//go:build cgo

package gui

import (
	"fmt"
	"net/netip"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const gatewayProbeCount = 5

type wifiSample struct {
	at            time.Time
	iface         string
	signal, noise int
	band          string
	channel       int
	tx            float64
	gateway       netip.Addr
	gatewayStatus string
	replies       int
	averageMS     float64
}

type wifiQualityView struct {
	content       fyne.CanvasObject
	before, after *widget.Button
	baseline      wifiSample
}

func newWiFiQualityView(u *ui, v *qualityView) *wifiQualityView {
	if !wifiDiagnosticsAvailable {
		return nil
	}
	w := &wifiQualityView{}
	beforeResult := wifiText("No baseline yet.")
	afterResult := wifiText("No comparison yet.")
	comparison := wifiText("")
	comparison.Hide()
	hint := wifiText("Capture before and after a change. Each sample sends up to five pings to the Wi-Fi gateway. Some gateways ignore pings.")
	w.before = widget.NewButton("Capture baseline", func() {
		startQualityMeasurement(u, v, "Wi-Fi sample", measureWiFi, func(sample wifiSample) {
			w.baseline = sample
			w.before.SetText("Replace baseline")
			beforeResult.SetText(sample.summary())
			afterResult.SetText("No comparison yet.")
			comparison.Hide()
		})
	})
	w.after = widget.NewButton("Capture after", func() {
		startQualityMeasurement(u, v, "Wi-Fi sample", measureWiFi, func(sample wifiSample) {
			afterResult.SetText(sample.summary())
			comparison.SetText(sample.compare(w.baseline))
			comparison.Show()
		})
	})
	w.content = container.NewVScroll(container.NewVBox(
		hint, container.NewGridWithColumns(2, w.before, w.after),
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			container.NewVBox(widget.NewLabelWithStyle("Baseline", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), beforeResult),
			container.NewVBox(widget.NewLabelWithStyle("After", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), afterResult),
		),
		comparison,
	))
	return w
}

func wifiText(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func (w *wifiQualityView) refresh(available bool) {
	w.before.Disable()
	w.after.Disable()
	if available {
		w.before.Enable()
		if !w.baseline.at.IsZero() {
			w.after.Enable()
		}
	}
}

func (s wifiSample) summary() string {
	text := fmt.Sprintf("%s · %s", s.at.Format("Jan 2, 15:04:05"), s.iface)
	if s.channel > 0 {
		text += "\n"
		if s.band != "" {
			text += s.band + " · "
		}
		text += fmt.Sprintf("Channel %d", s.channel)
	}
	text += fmt.Sprintf("\nSignal  %d dBm", s.signal)
	if s.noise < 0 {
		text += fmt.Sprintf("\nNoise  %d dBm\nSignal to noise  %d dB", s.noise, s.signal-s.noise)
	}
	if s.tx > 0 {
		text += fmt.Sprintf("\nTransmit link rate  %.0f Mbps", s.tx)
	}
	if s.gatewayStatus != "" {
		return text + "\n\nGateway  " + s.gatewayStatus
	}
	family := "IPv6"
	if s.gateway.Is4() {
		family = "IPv4"
	}
	text += fmt.Sprintf("\n\nGateway (%s)  %d/%d replies", family, s.replies, gatewayProbeCount)
	if s.replies > 0 {
		text += fmt.Sprintf("\nAverage round trip  %.1f ms", s.averageMS)
	}
	return text
}

func (s wifiSample) compare(before wifiSample) string {
	if s.iface != before.iface || (s.gateway.IsValid() && before.gateway.IsValid() && s.gateway != before.gateway) {
		return "The interface or gateway changed. Treat these as separate samples."
	}
	text := "Compare only if you stayed on the same network."
	if s.gateway.IsValid() && s.gateway == before.gateway {
		text += fmt.Sprintf("\nSignal change  %+d dB", s.signal-before.signal)
		if s.gatewayStatus == "" && before.gatewayStatus == "" && s.replies > 0 && before.replies > 0 {
			text += fmt.Sprintf("\nAverage round-trip change  %+.1f ms", s.averageMS-before.averageMS)
		}
	}
	return text
}
