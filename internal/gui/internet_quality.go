//go:build cgo

package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const qualityTimeout = 45 * time.Second
const qualityOutputLimit = 128 << 10

var errQualityOutputLimit = errors.New("Apple returned more diagnostic output than the app can display")

type qualityResult struct {
	Download    *float64 `json:"dl_throughput"`
	Upload      *float64 `json:"ul_throughput"`
	Idle        *float64 `json:"base_rtt"`
	LegacyIdle  *float64 `json:"base_rt"`
	RPM         *float64 `json:"responsiveness"`
	Interface   string   `json:"interface_name"`
	ErrorCode   *int     `json:"error_code"`
	ErrorDomain string   `json:"error_domain"`
}

func parseQualityResult(output []byte) (qualityResult, error) {
	var r qualityResult
	if err := json.Unmarshal(output, &r); err != nil {
		return r, errors.New("Apple returned unreadable measurement data")
	}
	if r.ErrorCode != nil || r.ErrorDomain != "" {
		return r, errors.New("Apple could not complete the measurement. Check Internet access and try again")
	}
	if r.Idle == nil {
		r.Idle = r.LegacyIdle
	}
	if r.Download == nil || *r.Download <= 0 || r.Upload == nil || *r.Upload <= 0 ||
		r.Idle == nil || *r.Idle < 0 || r.RPM == nil || *r.RPM <= 0 {
		return r, errors.New("Apple returned an incomplete measurement")
	}
	return r, nil
}

type qualityOutput struct {
	data   bytes.Buffer
	cancel context.CancelFunc
}

func (b *qualityOutput) Write(p []byte) (int, error) {
	if len(p) > qualityOutputLimit-b.data.Len() {
		b.cancel()
		return 0, errQualityOutputLimit
	}
	return b.data.Write(p)
}

func runInternetQuality(ctx context.Context) (qualityResult, error) {
	if runtime.GOOS != "darwin" {
		return qualityResult{}, errors.New("Apple Network Quality is available on macOS")
	}
	ctx, cancel := context.WithTimeout(ctx, qualityTimeout)
	defer cancel()
	return runQualityCommand(ctx, "/usr/bin/networkQuality", "-c")
}

func runQualityCommand(parent context.Context, path string, args ...string) (qualityResult, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stdout := &qualityOutput{cancel: cancel}
	stderr := &qualityOutput{cancel: cancel}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 500 * time.Millisecond
	err := cmd.Run()
	switch {
	case parent.Err() != nil:
		return qualityResult{}, parent.Err()
	case ctx.Err() != nil:
		return qualityResult{}, errQualityOutputLimit
	case errors.Is(err, os.ErrNotExist):
		return qualityResult{}, errors.New("Apple Network Quality is unavailable on this Mac. You can try Cloudflare in your browser")
	case err != nil:
		return qualityResult{}, errors.New("The Apple test failed. Check Internet access and try again")
	default:
		return parseQualityResult(stdout.data.Bytes())
	}
}

type qualityView struct {
	win      fyne.Window
	run      *widget.Button
	stop     *widget.Button
	browser  *widget.Button
	status   *widget.Label
	result   *widget.Label
	progress *widget.ProgressBarInfinite
}

type qualityJob struct {
	view      *qualityView
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	scanAfter netip.Prefix
}

func (u *ui) showInternetQuality() {
	if u.qualityView != nil {
		u.qualityView.win.RequestFocus()
		return
	}
	v := &qualityView{
		win:      u.app.NewWindow("Internet Quality — LanCartographer"),
		status:   widget.NewLabel("Ready"),
		result:   widget.NewLabel(""),
		progress: widget.NewProgressBarInfinite(),
	}
	u.qualityView = v
	v.status.Wrapping = fyne.TextWrapWord
	v.result.Wrapping = fyne.TextWrapWord
	v.progress.Hide()
	v.run = widget.NewButton("Run Apple test…", func() {
		dialog.NewConfirm("Run Apple Network Quality?",
			"This sends test traffic to Apple and can use substantial data or slow down other traffic. Run on an unmetered connection.",
			func(ok bool) {
				if ok {
					u.startInternetQuality(v, runInternetQuality)
				}
			}, v.win).Show()
	})
	v.run.Importance = widget.HighImportance
	v.stop = widget.NewButton("Stop", u.cancelInternetQuality)
	v.browser = widget.NewButton("Open Cloudflare in browser…", func() {
		dialog.NewConfirm("Open Cloudflare Speed Test?",
			"Cloudflare receives your IP address and shares anonymized measurement data. A test can use up to 200 MB. Your browser controls the test.",
			func(ok bool) {
				if ok && u.qualityView == v && u.qualityJob == nil {
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := exec.CommandContext(ctx, "/usr/bin/open", "https://speed.cloudflare.com/").Run(); err != nil {
							fyne.Do(func() {
								if u.qualityView == v {
									dialog.ShowInformation("Browser unavailable", "Open speed.cloudflare.com in your browser to run the Cloudflare test.", v.win)
								}
							})
						}
					}()
				}
			}, v.win).Show()
	})
	intro := widget.NewLabel("Measure Internet speed, idle latency and responsiveness under load from this Mac. Results stay in this window.")
	intro.Wrapping = fyne.TextWrapWord
	v.win.SetContent(container.NewPadded(container.NewBorder(nil, widget.NewButton("Close", v.win.Close), nil, nil,
		container.NewVScroll(container.NewVBox(
			intro, container.NewHBox(v.run, v.stop), v.progress, v.status, v.result,
			widget.NewSeparator(), v.browser,
		)),
	)))
	v.win.Resize(fyne.NewSize(510, 390))
	v.win.SetOnClosed(func() {
		v.progress.Stop()
		u.qualityView = nil
		u.cancelInternetQuality()
	})
	u.refreshQualityControls()
	v.win.Show()
}

func (u *ui) refreshQualityControls() {
	v := u.qualityView
	if v == nil {
		return
	}
	v.run.SetText("Run Apple test…")
	v.run.Disable()
	v.stop.Disable()
	v.browser.Disable()
	if u.qualityJob == nil {
		v.browser.Enable()
		if !u.scanning {
			v.run.Enable()
		}
	} else if u.qualityJob.view == v && u.qualityJob.ctx.Err() == nil {
		v.stop.Enable()
	}
	if u.scanning {
		v.run.SetText("Available after scan")
	}
}

func (u *ui) startInternetQuality(v *qualityView, run func(context.Context) (qualityResult, error)) {
	if u.qualityView != v || u.qualityJob != nil || u.scanning {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &qualityJob{view: v, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	u.qualityJob = job
	v.result.SetText("")
	v.status.SetText("Measuring with Apple…")
	v.progress.Show()
	v.progress.Start()
	u.refreshQualityControls()
	go func() {
		started := time.Now()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-job.done:
				return
			case <-ticker.C:
				elapsed := time.Since(started).Round(time.Second)
				fyne.Do(func() {
					if u.qualityView == v && u.qualityJob == job && ctx.Err() == nil {
						v.status.SetText(fmt.Sprintf("Measuring with Apple… %s", elapsed))
					}
				})
			}
		}
	}()
	go func() {
		result, err := run(ctx)
		close(job.done)
		fyne.Do(func() { u.finishInternetQuality(job, result, err) })
	}()
}

func (u *ui) cancelInternetQuality() {
	if job := u.qualityJob; job != nil {
		job.cancel()
		if u.qualityView == job.view {
			job.view.status.SetText("Stopping Apple test…")
		}
		u.refreshQualityControls()
	}
}

func (u *ui) finishInternetQuality(job *qualityJob, r qualityResult, err error) {
	u.qualityJob = nil
	v := job.view
	if u.qualityView == v {
		v.progress.Stop()
		v.progress.Hide()
		switch {
		case job.ctx.Err() != nil || errors.Is(err, context.Canceled):
			v.status.SetText("Cancelled")
		case errors.Is(err, context.DeadlineExceeded):
			v.status.SetText("Timed out. You can retry or open Cloudflare in your browser.")
		case err != nil:
			v.status.SetText(err.Error())
		default:
			v.status.SetText("Complete · Apple Network Quality · " + time.Now().Format("Jan 2, 15:04:05"))
			iface := "Unavailable"
			if len(r.Interface) > 0 && len(r.Interface) <= 16 {
				valid := true
				for _, c := range r.Interface {
					valid = valid && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
				}
				if valid {
					iface = r.Interface
				}
			}
			v.result.SetText(fmt.Sprintf("Download  %.1f Mbps\nUpload  %.1f Mbps\nIdle latency  %.1f ms\nResponsiveness under load  %.0f RPM\nInterface  %s",
				*r.Download/1e6, *r.Upload/1e6, *r.Idle, *r.RPM, iface))
		}
	}
	job.cancel()
	if job.scanAfter.IsValid() {
		u.startScan(job.scanAfter)
	}
	u.refreshQualityControls()
}
