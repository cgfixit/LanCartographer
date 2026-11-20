//go:build cgo

package gui

import (
	"context"
	"errors"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/macdiag"
)

type routeUI struct {
	view *routeView
	job  *diagnosticJob
}

func (r *routeUI) stopAndWait() {
	if job := r.job; job != nil {
		job.cancel()
		<-job.done
	}
}

type routeView struct {
	win                fyne.Window
	destination        *widget.Entry
	inspect, dns, stop *widget.Button
	status             *widget.Label
	progress           *widget.ProgressBarInfinite
}

func (u *ui) routeMenu() []*fyne.MenuItem {
	return []*fyne.MenuItem{fyne.NewMenuItem("Route & DNS…", u.showRouteDNS)}
}

func (u *ui) showRouteDNS() {
	r := &u.route
	if r.view != nil {
		r.view.win.RequestFocus()
		return
	}
	v := &routeView{
		win: u.app.NewWindow("Route & DNS — LanCartographer"), destination: widget.NewEntry(),
		status: widget.NewLabel("Ready"), progress: widget.NewProgressBarInfinite(),
	}
	r.view = v
	v.destination.SetPlaceHolder("Numeric IPv4 or IPv6 address")
	v.status.Wrapping = fyne.TextWrapWord
	v.progress.Hide()
	v.stop = widget.NewButton("Stop", func() {
		if r.job != nil {
			r.job.cancel()
			v.status.SetText("Stopping…")
			v.stop.Disable()
		}
	})
	v.stop.Disable()
	routeOutput := newConfigurationLabel()
	dnsOutput := newConfigurationLabel()
	v.inspect = widget.NewButton("Inspect route", func() {
		destination := strings.TrimSpace(v.destination.Text)
		r.start(v, "route", routeOutput, func(ctx context.Context) (string, error) {
			return macdiag.ReadRoute(ctx, destination)
		})
	})
	v.inspect.Importance = widget.HighImportance
	v.destination.OnSubmitted = func(string) { v.inspect.OnTapped() }
	v.dns = widget.NewButton("Read resolver configuration", func() {
		r.start(v, "resolver configuration", dnsOutput, macdiag.ReadResolverConfiguration)
	})
	v.dns.Importance = widget.HighImportance
	routeHint := widget.NewLabel("Read this Mac’s routing table for one address. This sends no probes and provides no reachability measurement.")
	routeHint.Wrapping = fyne.TextWrapWord
	dnsHint := widget.NewLabel("Read configured resolver scopes. For observed system lookup answers, use Resolve in Check Service.")
	dnsHint.Wrapping = fyne.TextWrapWord
	dnsDetails := widget.NewAccordion(widget.NewAccordionItem("Configured resolver scopes", dnsOutput))
	tabs := container.NewAppTabs(
		container.NewTabItem("Route", container.NewBorder(container.NewVBox(routeHint,
			container.NewBorder(nil, nil, nil, v.inspect, v.destination)), nil, nil, nil, container.NewVScroll(routeOutput))),
		container.NewTabItem("Resolver configuration", container.NewBorder(container.NewVBox(dnsHint, v.dns),
			nil, nil, nil, container.NewVScroll(dnsDetails))),
	)
	if r.job != nil {
		v.setBusy(true)
		v.stop.Disable()
		v.status.SetText("Waiting for the previous helper to stop…")
	}
	v.win.SetContent(container.NewPadded(container.NewBorder(nil, container.NewVBox(v.status, v.progress,
		container.NewHBox(v.stop, widget.NewButton("Close", v.win.Close))), nil, nil, tabs)))
	v.win.Resize(fyne.NewSize(675, 495))
	v.win.SetOnClosed(func() {
		v.progress.Stop()
		r.view = nil
		if r.job != nil {
			r.job.cancel()
		}
	})
	v.win.Show()
}

func newConfigurationLabel() *widget.Label {
	label := widget.NewLabel("")
	label.TextStyle.Monospace = true
	label.Wrapping = fyne.TextWrapBreak
	return label
}

func (v *routeView) setBusy(busy bool) {
	if busy {
		v.inspect.Disable()
		v.dns.Disable()
		v.destination.Disable()
		v.stop.Enable()
		return
	}
	v.inspect.Enable()
	v.dns.Enable()
	v.destination.Enable()
	v.stop.Disable()
}

func (r *routeUI) start(v *routeView, name string, output *widget.Label, read func(context.Context) (string, error)) {
	if r.job != nil || r.view != v {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &diagnosticJob{cancel: cancel, done: make(chan struct{})}
	r.job = job
	v.setBusy(true)
	output.SetText("")
	v.progress.Show()
	v.progress.Start()
	v.status.SetText("Reading " + name + "…")
	go func() {
		result, err := read(ctx)
		close(job.done)
		fyne.Do(func() {
			r.job = nil
			defer cancel()
			if r.view != v {
				if r.view != nil {
					r.view.setBusy(false)
					r.view.status.SetText("Ready")
				}
				return
			}
			v.setBusy(false)
			v.progress.Stop()
			v.progress.Hide()
			switch {
			case ctx.Err() != nil:
				v.status.SetText("Cancelled")
			case errors.Is(err, context.DeadlineExceeded):
				v.status.SetText("Timed out reading " + name)
			case errors.Is(err, macdiag.ErrOutputLimit):
				v.status.SetText("The " + name + " output exceeded the display limit")
			case err != nil:
				v.status.SetText(err.Error())
			case result == "":
				v.status.SetText("No " + name + " returned")
			default:
				output.SetText(result)
				v.status.SetText("Read " + name + " at " + time.Now().Format(time.TimeOnly))
			}
		})
	}()
}
