//go:build cgo

package gui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/servicecheck"
)

// serviceView is the modeless Tools → Check Service window. Results stay
// in memory in this window; they are never saved, logged or exported.
type serviceView struct {
	win      fyne.Window
	host     *widget.Entry
	port     *widget.Entry
	protocol *widget.Select
	resolve  *widget.Button
	address  *widget.Select
	check    *widget.Button
	stop     *widget.Button
	copyBtn  *widget.Button
	status   *widget.Label
	progress *widget.ProgressBarInfinite
	stages   *fyne.Container

	// protocolChosen records an explicit protocol choice; until then a
	// port edit suggests one. suggesting marks programmatic selections.
	protocolChosen, suggesting bool

	resolved     []netip.Addr
	resolvedHost string
	resolveStage *servicecheck.Stage
	last         *servicecheck.Result
	job          *serviceJob
	afterJob     func() // test hook: runs after a job's UI callback
	confirming   bool   // a confirmation dialog is open; prefill waits
}

// serviceJob is one resolve or check. Results from a job that is no longer
// the view's current job (cancelled, superseded, window closed) are dropped.
type serviceJob struct {
	cancel context.CancelFunc
	done   chan struct{}
	label  string
}

// servicePrefill carries inventory values into the form. Prefilling never
// contacts anything.
type servicePrefill struct {
	host     string
	port     int
	protocol servicecheck.Protocol
}

const serviceAddressPlaceholder = "Resolve the name first"

// interfaceExists accepts an interface name (en0) or, as Windows uses for
// IPv6 zones, a numeric interface index (fe80::1%12).
func interfaceExists(name string) bool {
	if _, err := net.InterfaceByName(name); err == nil {
		return true
	}
	index, err := strconv.Atoi(name)
	if err != nil || index <= 0 {
		return false
	}
	_, err = net.InterfaceByIndex(index)
	return err == nil
}

// showServiceCheck opens (or focuses) the Check Service window and applies
// prefill when no job is running.
func (u *ui) showServiceCheck(prefill *servicePrefill) {
	if v := u.serviceView; v != nil {
		if prefill != nil && v.job == nil && !v.confirming {
			v.apply(prefill)
		}
		v.win.RequestFocus()
		return
	}
	v := &serviceView{
		win:      u.app.NewWindow("Check Service — LanCartographer"),
		host:     widget.NewEntry(),
		port:     widget.NewEntry(),
		status:   widget.NewLabel("Ready"),
		progress: widget.NewProgressBarInfinite(),
		stages:   container.NewVBox(),
	}
	u.serviceView = v
	v.status.Wrapping = fyne.TextWrapWord
	v.progress.Hide()
	v.host.SetPlaceHolder("nas.home.arpa or 192.168.1.20")
	v.port.SetPlaceHolder("443")
	labels := make([]string, len(servicecheck.Protocols))
	for i, p := range servicecheck.Protocols {
		labels[i] = string(p)
	}
	v.protocol = widget.NewSelect(labels, func(string) {
		if !v.suggesting {
			v.protocolChosen = true
		}
	})
	v.suggestProtocol(string(servicecheck.HTTPS))
	v.address = widget.NewSelect(nil, nil)
	v.address.PlaceHolder = serviceAddressPlaceholder
	v.resolve = widget.NewButton("Resolve", func() { u.resolveService(v) })
	v.check = widget.NewButtonWithIcon("Check…", theme.SearchIcon(), func() { u.confirmServiceCheck(v) })
	v.check.Importance = widget.HighImportance
	v.stop = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() { u.cancelServiceJob(v) })
	v.copyBtn = widget.NewButtonWithIcon("Copy results", theme.ContentCopyIcon(), func() {
		if text := v.copyText(); text != "" {
			v.win.Clipboard().SetContent(text)
			v.status.SetText("Copied the host, address and results to the clipboard.")
		}
	})
	v.host.OnChanged = func(string) { v.hostChanged() }
	v.host.OnSubmitted = func(string) {
		if v.resolve.Disabled() {
			u.confirmServiceCheck(v)
		} else {
			u.resolveService(v)
		}
	}
	v.port.OnChanged = func(text string) {
		if port, err := strconv.Atoi(strings.TrimSpace(text)); err == nil && !v.protocolChosen {
			v.suggestProtocol(string(servicecheck.SuggestProtocol(port)))
		}
	}
	v.port.OnSubmitted = func(string) { u.confirmServiceCheck(v) }

	intro := wifiText("Can this computer reach a service, and where does it fail? " +
		"Resolve looks a name up through this computer's resolver (DNS, or multicast DNS for .local names). " +
		"Check then connects only to the one address you choose.")
	form := widget.NewForm(
		widget.NewFormItem("Host", v.host),
		widget.NewFormItem("Port", container.NewGridWithColumns(2, v.port, v.protocol)),
		widget.NewFormItem("Address", container.NewBorder(nil, nil, nil, v.resolve, v.address)),
	)
	note := wifiText("Results describe this computer's path at the time shown. They stay in this window and are not saved.")
	note.TextStyle = fyne.TextStyle{Italic: true}
	body := container.NewVScroll(container.NewVBox(intro, form, v.check, widget.NewSeparator(), v.stages, note))
	footer := container.NewVBox(v.progress, v.status,
		container.NewGridWithColumns(3, v.stop, v.copyBtn, widget.NewButton("Close", v.win.Close)))
	v.win.SetContent(container.NewPadded(container.NewBorder(nil, footer, nil, nil, body)))
	v.win.Resize(fyne.NewSize(620, 560))
	v.win.SetOnClosed(func() {
		v.progress.Stop()
		if u.serviceView == v {
			u.serviceView = nil
		}
		u.cancelServiceJob(v)
	})
	if prefill != nil {
		v.apply(prefill)
	}
	v.hostChanged()
	v.win.Show()
	v.win.Canvas().Focus(v.host)
}

func (v *serviceView) suggestProtocol(p string) {
	v.suggesting = true
	v.protocol.SetSelected(p)
	v.suggesting = false
}

func (v *serviceView) apply(p *servicePrefill) {
	v.protocolChosen = false
	v.host.SetText(p.host)
	if p.port > 0 {
		v.port.SetText(strconv.Itoa(p.port))
		protocol := p.protocol
		if protocol == "" {
			protocol = servicecheck.SuggestProtocol(p.port)
		}
		v.suggestProtocol(string(protocol))
	} else {
		v.port.SetText("")
	}
	v.clearResults()
	v.status.SetText("Ready")
}

// hostChanged discards a resolution that no longer matches the host text
// and recomputes which controls apply.
func (v *serviceView) hostChanged() {
	text := strings.TrimSpace(v.host.Text)
	if text != v.resolvedHost {
		v.resolved, v.resolvedHost, v.resolveStage = nil, "", nil
		v.address.Options = nil
		v.address.ClearSelected()
	}
	if _, literal, err := servicecheck.ParseHost(text, interfaceExists); err == nil && literal.IsValid() {
		v.address.Options = []string{literal.String()}
		v.address.SetSelected(literal.String())
	}
	v.refreshControls()
}

func (v *serviceView) refreshControls() {
	running := v.job != nil
	_, literal, err := servicecheck.ParseHost(v.host.Text, interfaceExists)
	isName := err == nil && !literal.IsValid()
	for _, w := range []fyne.Disableable{v.host, v.port, v.protocol, v.address, v.resolve, v.check, v.copyBtn} {
		w.Disable()
	}
	if running {
		v.stop.Enable()
		return
	}
	v.stop.Disable()
	v.host.Enable()
	v.port.Enable()
	v.protocol.Enable()
	v.check.Enable()
	if isName {
		v.resolve.Enable()
		if len(v.resolved) > 1 {
			v.address.Enable()
		}
	}
	if v.last != nil {
		v.copyBtn.Enable()
	}
}

func (v *serviceView) clearResults() {
	v.last = nil
	v.stages.RemoveAll()
}

// startServiceJob runs work off the UI thread and hands its result to
// finish on the UI thread, unless the job was cancelled or replaced.
func startServiceJob[T any](u *ui, v *serviceView, label string, work func(context.Context) T, finish func(T)) {
	if v.job != nil || u.serviceView != v {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &serviceJob{cancel: cancel, done: make(chan struct{}), label: label}
	v.job = job
	v.status.SetText(label + "…")
	v.progress.Show()
	v.progress.Start()
	v.refreshControls()
	go func() {
		result := work(ctx)
		close(job.done)
		fyne.Do(func() {
			if v.afterJob != nil {
				defer v.afterJob()
			}
			// Only cancelServiceJob cancels ctx, after it has already reset
			// the view; checking ctx first orders this read after its writes.
			if ctx.Err() != nil || v.job != job {
				return
			}
			v.job = nil
			v.progress.Stop()
			v.progress.Hide()
			finish(result)
			cancel()
			v.refreshControls()
		})
	}()
}

// cancelServiceJob stops the current job. Its late result is dropped.
func (u *ui) cancelServiceJob(v *serviceView) {
	job := v.job
	if job == nil {
		return
	}
	v.job = nil
	v.progress.Stop()
	v.progress.Hide()
	v.status.SetText("Cancelled")
	if u.serviceView == v {
		v.refreshControls()
	}
	job.cancel()
}

func (u *ui) resolveService(v *serviceView) {
	host, literal, err := servicecheck.ParseHost(v.host.Text, interfaceExists)
	switch {
	case err != nil:
		v.status.SetText(err.Error())
		return
	case literal.IsValid():
		v.status.SetText("That is already an address; no lookup is needed.")
		return
	}
	v.clearResults()
	type answer struct {
		addrs []netip.Addr
		stage servicecheck.Stage
	}
	startServiceJob(u, v, "Looking up "+host, func(ctx context.Context) answer {
		addrs, stage := servicecheck.Resolve(ctx, nil, host, interfaceExists)
		return answer{addrs, stage}
	}, func(a answer) {
		v.resolveStage = &a.stage
		v.stages.Add(stageRow(a.stage))
		if len(a.addrs) == 0 {
			v.status.SetText("Lookup failed · " + time.Now().Format("15:04:05"))
			return
		}
		v.resolved, v.resolvedHost = a.addrs, strings.TrimSpace(v.host.Text)
		options := make([]string, len(a.addrs))
		for i, addr := range a.addrs {
			options[i] = addr.String()
		}
		v.address.Options = options
		v.address.SetSelected(options[0])
		v.status.SetText("Choose an address, then Check.")
	})
}

// serviceTarget validates the form. Only the selected address is dialed.
func (v *serviceView) serviceTarget() (servicecheck.Target, error) {
	host, literal, err := servicecheck.ParseHost(v.host.Text, interfaceExists)
	if err != nil {
		return servicecheck.Target{}, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(v.port.Text))
	if err != nil || port < 1 || port > 65535 {
		return servicecheck.Target{}, errors.New("Enter a port from 1 to 65535.")
	}
	addr := literal
	if !addr.IsValid() {
		if v.resolvedHost != strings.TrimSpace(v.host.Text) || v.address.Selected == "" {
			return servicecheck.Target{}, errors.New("Select Resolve to look up the name, then choose an address.")
		}
		addr, err = netip.ParseAddr(v.address.Selected)
		if err != nil || !containsAddr(v.resolved, addr) {
			return servicecheck.Target{}, errors.New("Choose one of the resolved addresses.")
		}
	}
	return servicecheck.Target{Host: host, Addr: addr, Port: uint16(port), Protocol: servicecheck.Protocol(v.protocol.Selected)}, nil
}

func containsAddr(list []netip.Addr, a netip.Addr) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}

// confirmServiceCheck names the exact endpoint and traffic before any
// contact. Declining sends nothing.
func (u *ui) confirmServiceCheck(v *serviceView) {
	if v.job != nil {
		return
	}
	target, err := v.serviceTarget()
	if err != nil {
		v.status.SetText(err.Error())
		return
	}
	resolveStage := v.resolveStage
	v.confirming = true
	dialog.NewConfirm("Check this service?", serviceConfirmText(target, u.local), func(ok bool) {
		v.confirming = false
		if ok && u.serviceView == v {
			u.runServiceCheck(v, target, resolveStage)
		}
	}, v.win).Show()
}

func (u *ui) runServiceCheck(v *serviceView, target servicecheck.Target, resolveStage *servicecheck.Stage) {
	v.clearResults()
	startServiceJob(u, v, "Checking "+target.Endpoint(), func(ctx context.Context) servicecheck.Result {
		r, err := servicecheck.Check(ctx, target, servicecheck.Options{})
		if err != nil {
			r = servicecheck.Result{Target: target, Started: time.Now(), Stages: []servicecheck.Stage{{
				Name: "Input", Outcome: servicecheck.Failed, Detail: err.Error()}}}
		}
		return r
	}, func(r servicecheck.Result) {
		if resolveStage != nil {
			r.Stages = append([]servicecheck.Stage{*resolveStage}, r.Stages...)
		}
		v.last = &r
		for _, s := range r.Stages {
			v.stages.Add(stageRow(s))
		}
		v.status.SetText(serviceVerdict(r) + " · " + r.Started.Format("Jan 2, 15:04:05"))
	})
}

// serviceConfirmText states the endpoint, what will be sent, any scope
// caution and the authorization notice verbatim.
func serviceConfirmText(t servicecheck.Target, local []netip.Prefix) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Connect to %s port %d", t.Addr, t.Port)
	if t.Host != t.Addr.String() {
		fmt.Fprintf(&b, " as %s", t.Host)
	}
	b.WriteString(".\n\nThis opens one TCP connection")
	switch t.Protocol {
	case servicecheck.TLS:
		b.WriteString(" and completes a verified TLS handshake")
	case servicecheck.HTTP:
		b.WriteString(" and sends one HTTP GET /")
	case servicecheck.HTTPS:
		b.WriteString(", completes a verified TLS handshake and sends one HTTP GET /")
	}
	b.WriteString(". Redirects are not followed and no other address is contacted.")
	if note := serviceScopeNote(t.Addr, local); note != "" {
		b.WriteString("\n\n" + note)
	}
	b.WriteString("\n\n" + authz.Notice)
	return b.String()
}

func serviceScopeNote(addr netip.Addr, local []netip.Prefix) string {
	single := netip.PrefixFrom(addr.WithZone(""), addr.BitLen())
	switch {
	case addr.IsLoopback():
		return "This address is this computer (loopback)."
	case addr.Is4() && !authz.NeedsConfirmation(single, local):
		return ""
	case addr.Is4() && authz.IsRFC1918(single):
		return "Caution: " + addr.String() + " is outside this computer's local subnets."
	case addr.Is4() && addr.IsLinkLocalUnicast():
		return "Caution: " + addr.String() + " is a link-local address outside this computer's local subnets."
	case addr.Is4() && netip.MustParsePrefix("100.64.0.0/10").Contains(addr):
		return "Caution: " + addr.String() + " is shared carrier-grade NAT space outside your local networks."
	case addr.Is4():
		return "Caution: " + addr.String() + " is a public address outside your local networks."
	case addr.IsPrivate() || addr.IsLinkLocalUnicast():
		return "Caution: this is a private IPv6 address, which the IPv4 subnet check cannot place. The inventory scan covers IPv4 only."
	}
	return "Caution: this is a public IPv6 address outside your local networks."
}

func serviceVerdict(r servicecheck.Result) string {
	switch r.Outcome() {
	case servicecheck.Passed:
		return "Service reachable"
	case servicecheck.Warning:
		return "Reachable with warnings"
	}
	for _, s := range r.Stages {
		if s.Outcome == servicecheck.Failed {
			return s.Name + " failed"
		}
	}
	return "Complete"
}

func (v *serviceView) copyText() string {
	if v.last == nil {
		return ""
	}
	return v.last.Text()
}

func stageRow(s servicecheck.Stage) fyne.CanvasObject {
	var icon fyne.Resource
	switch s.Outcome {
	case servicecheck.Passed:
		icon = theme.NewSuccessThemedResource(theme.ConfirmIcon())
	case servicecheck.Warning:
		icon = theme.NewWarningThemedResource(theme.WarningIcon())
	case servicecheck.Failed:
		icon = theme.NewErrorThemedResource(theme.CancelIcon())
	default:
		icon = theme.NewDisabledResource(theme.MediaSkipNextIcon())
	}
	name := widget.NewLabelWithStyle(s.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	lead := container.NewHBox(widget.NewIcon(icon), container.NewGridWrap(fyne.NewSize(72, name.MinSize().Height), name))
	detail := wifiText(s.Outcome.String() + " · " + s.Detail)
	var elapsed fyne.CanvasObject = widget.NewLabel("")
	if s.Outcome != servicecheck.Skipped {
		elapsed = widget.NewLabel(servicecheck.FormatElapsed(s.Elapsed))
	}
	return container.NewBorder(nil, nil, lead, elapsed, detail)
}

// servicePrefillFor picks the inventory host's address and the most likely
// service port: HTTPS, then HTTP (including ports the scan found serving a
// web page), then the first open port.
func servicePrefillFor(h model.Host) *servicePrefill {
	p := &servicePrefill{host: h.IP.String()}
	best := 0
	for _, port := range h.Ports {
		if port.State != model.PortOpen {
			continue
		}
		protocol := servicecheck.SuggestProtocol(port.Number)
		if protocol == servicecheck.TCP && port.Service == "Web" {
			protocol = servicecheck.HTTP
		}
		rank := 1
		switch protocol {
		case servicecheck.HTTPS:
			rank = 3
		case servicecheck.HTTP:
			rank = 2
		}
		if rank > best {
			best, p.port, p.protocol = rank, port.Number, protocol
		}
	}
	return p
}
