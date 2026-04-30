// Package ui implements the wifiui Gio interface. The package owns:
//
//   - App: lifecycle and top-level layout dispatch (this file).
//   - model / viewState: the immutable per-frame snapshot the renderer reads.
//   - passphrasePrompt: the iwd agent <-> UI bridge for connect prompts.
//   - toolsPanel: the diagnostics tab.
//   - networks-tab rendering helpers in networks.go.
package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os/exec"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lewispb/wifiui/internal/iwd"
	"github.com/lewispb/wifiui/internal/netinfo"
	"github.com/lewispb/wifiui/internal/pins"
	"github.com/lewispb/wifiui/internal/portal"
	"github.com/lewispb/wifiui/internal/theme"
)

// rowState holds per-row clickables; persistent across frames. Only mutated
// from the layout goroutine.
type rowState struct {
	connect    widget.Clickable
	pin        widget.Clickable
	disconnect widget.Clickable
}

// App is the Gio application shell.
type App struct {
	w     *app.Window
	th    *theme.Theme
	c     *iwd.Client
	st    *iwd.Station
	pins  *pins.Store
	model *model

	ctx    context.Context
	cancel context.CancelFunc

	// networks-tab widget state. Only touched from the layout goroutine.
	scanBtn    widget.Clickable
	scroll     widget.List
	rows       map[string]*rowState
	portalOpen widget.Clickable

	// tab strip
	view     view
	tabNet   widget.Clickable
	tabTools widget.Clickable

	// auto-scan tracking (layout goroutine only)
	tickCount int

	pw    *passphrasePrompt
	tools *toolsPanel
}

// Run wires iwd, pins, theme and starts the Gio loop.
func Run(w *app.Window) error {
	c, err := iwd.New()
	if err != nil {
		return err
	}
	defer c.Close()

	stations, err := c.Stations(context.Background())
	if err != nil {
		return err
	}
	if len(stations) == 0 {
		return errors.New("no wifi stations found — is iwd running and a wifi adapter present?")
	}
	st := stations[0]

	p, err := pins.New()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &App{
		w:      w,
		th:     theme.New(),
		c:      c,
		st:     st,
		pins:   p,
		ctx:    ctx,
		cancel: cancel,
		rows:   map[string]*rowState{},
		view:   viewNetworks,
		model: newModel(viewState{
			station: st.Name,
			pinned:  asSet(p.All()),
		}),
	}
	a.pw = newPassphrasePrompt(a)
	a.tools = newToolsPanel(a)
	a.scroll.Axis = layout.Vertical
	a.th.Watch(ctx, func() { a.w.Invalidate() })

	unreg, err := c.RegisterAgentWith(ctx, iwd.AgentHandlers{
		Passphrase: a.pw.handleRequest,
		Cancel:     a.pw.onAgentCancel,
	})
	if err != nil {
		return fmt.Errorf("register iwd agent: %w", err)
	}
	defer unreg()

	go a.refreshLoop(ctx)

	return a.run()
}

func asSet(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

// ---- background loops ----

func (a *App) refreshLoop(ctx context.Context) {
	a.refresh(ctx)
	tick := time.NewTicker(refreshInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		a.refresh(ctx)
		a.tickCount++
		if a.tickCount%int(rescanEvery/refreshInterval) == 0 {
			go a.backgroundScan(ctx)
		}
	}
}

func (a *App) backgroundScan(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, scanCallTimeout)
	defer cancel()
	if scanning, _ := a.st.Scanning(ctx); !scanning {
		_ = a.st.Scan(ctx)
	}
}

func (a *App) refresh(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, refreshCallTimeout)
	defer cancel()

	state, err := a.st.State(ctx)
	if err != nil {
		a.setError(err)
		return
	}
	scanning, _ := a.st.Scanning(ctx)
	nets, err := a.st.Networks(ctx)
	if err != nil {
		a.setError(err)
		return
	}
	vendor, _, _ := a.st.AdapterInfo(ctx)
	vendor = cleanVendor(vendor)

	var connected, band string
	var sig int
	for _, n := range nets {
		if n.Connected {
			connected = n.SSID
			sig = n.Signal
			band = n.Band()
			break
		}
	}

	ifaces, _ := netinfo.List(ctx)
	pinned := asSet(a.pins.All())
	now := time.Now()

	a.model.mutate(func(s *viewState) {
		s.state = state
		s.scanning = scanning
		s.networks = nets
		s.connected = connected
		s.signal = sig
		s.band = band
		s.vendor = vendor
		s.interfaces = ifaces
		s.pinned = pinned
		s.err = ""
		s.lastTick = now
	})
	a.w.Invalidate()
}

func cleanVendor(v string) string {
	if i := strings.IndexAny(v, ",."); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func (a *App) setError(err error) {
	now := time.Now()
	a.model.mutate(func(s *viewState) {
		s.err = err.Error()
		s.lastTick = now
	})
	a.w.Invalidate()
}

// ---- user-triggered actions ----

func (a *App) row(ssid string) *rowState {
	r, ok := a.rows[ssid]
	if !ok {
		r = &rowState{}
		a.rows[ssid] = r
	}
	return r
}

func (a *App) triggerScan() {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, scanCallTimeout)
		defer cancel()
		_ = a.st.Scan(ctx)
		_ = a.st.WaitScan(ctx, scanWaitDeadline)
		a.refresh(a.ctx)
	}()
}

func (a *App) triggerDisconnect() {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, disconnectTimeout)
		defer cancel()
		if err := a.st.Disconnect(ctx); err != nil {
			a.setError(err)
			return
		}
		a.refresh(a.ctx)
	}()
}

func (a *App) triggerConnect(ssid string) {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, connectTimeout)
		defer cancel()
		nets, err := a.st.Networks(ctx)
		if err != nil {
			a.setError(err)
			a.pw.clearPendingSave()
			return
		}
		for _, n := range nets {
			if n.SSID != ssid {
				continue
			}
			if err := n.Connect(ctx); err != nil {
				a.setError(err)
				a.pw.clearPendingSave()
				return
			}
			a.refresh(a.ctx)
			a.pw.persistOnSuccess()
			go a.checkPortal()
			return
		}
		a.setError(fmt.Errorf("network %q not visible", ssid))
		a.pw.clearPendingSave()
	}()
}

func (a *App) checkPortal() {
	a.model.mutate(func(s *viewState) {
		s.portalStatus = portal.StatusUnknown
		s.portalRedirect = ""
	})
	a.w.Invalidate()

	r := portal.Check(a.ctx)

	a.model.mutate(func(s *viewState) {
		s.portalStatus = r.Status
		s.portalRedirect = r.RedirectURL
	})
	a.w.Invalidate()
}

func (a *App) togglePin(ssid string) {
	if err := a.pins.Toggle(ssid); err != nil {
		a.setError(err)
		return
	}
	pinned := asSet(a.pins.All())
	a.model.mutate(func(s *viewState) { s.pinned = pinned })
	a.w.Invalidate()
}

// openInBrowser launches url in the system browser. Returns an error so the
// caller can surface it; logging to /dev/null hides real misconfigurations.
func openInBrowser(url string) error {
	if url == "" {
		return nil
	}
	for _, browser := range []string{"xdg-open", "chromium", "google-chrome-stable", "google-chrome"} {
		if _, err := exec.LookPath(browser); err == nil {
			return exec.Command(browser, url).Start()
		}
	}
	return errors.New("no browser found (install xdg-utils or a chromium-family browser)")
}

// ---- top-level layout ----

func (a *App) run() error {
	var ops op.Ops
	for {
		switch e := a.w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	if a.scanBtn.Clicked(gtx) {
		a.triggerScan()
	}
	if a.tabNet.Clicked(gtx) {
		a.view = viewNetworks
	}
	if a.tabTools.Clicked(gtx) {
		a.view = viewTools
	}
	if a.portalOpen.Clicked(gtx) {
		s := a.model.snapshot()
		go func(url string) {
			if err := openInBrowser(url); err != nil {
				a.setError(err)
			}
		}(s.portalRedirect)
	}
	a.pw.drain(gtx)

	pal := a.th.Palette()
	paint.Fill(gtx.Ops, pal.Bg)

	s := a.model.snapshot()

	return layout.Inset{Top: theme.S6, Bottom: theme.S5, Left: theme.S6, Right: theme.S6}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return centeredMaxWidth(gtx, unit.Dp(640), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.header(gtx, pal, s) }),
					layout.Rigid(layout.Spacer{Height: theme.S4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.tabBar(gtx, pal) }),
					layout.Rigid(layout.Spacer{Height: theme.S4}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if a.pw.hasActive() {
							return a.pw.view(gtx, pal)
						}
						switch a.view {
						case viewTools:
							return a.tools.view(gtx, pal, s)
						default:
							return a.body(gtx, pal, s)
						}
					}),
					layout.Rigid(layout.Spacer{Height: theme.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.footer(gtx, pal, s) }),
				)
			})
		})
}

// tabBar is a two-segment control switching between the networks and tools
// views.
func (a *App) tabBar(gtx layout.Context, pal theme.Palette) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.tab(gtx, pal, viewNetworks, iconWifi+"  networks", &a.tabNet)
		}),
		layout.Rigid(layout.Spacer{Width: theme.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.tab(gtx, pal, viewTools, iconWrench+"  tools", &a.tabTools)
		}),
	)
}

func (a *App) tab(gtx layout.Context, pal theme.Palette, id view, txt string, click *widget.Clickable) layout.Dimensions {
	active := a.view == id
	bg := color.NRGBA{}
	col := pal.TextDim
	border := pal.Border
	if active {
		bg = pal.Surface
		col = pal.Text
		border = pal.BorderHi
	}
	return clipRRectClickable(gtx, theme.RadiusRow, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(theme.RadiusRow)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				if bg.A != 0 {
					paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
				}
				stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
				paint.ColorOp{Color: border}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				stk.Pop()
				return layout.Dimensions{Size: sz}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return label(a.th.Material, txt, unit.Sp(12), font.SemiBold, col).Layout(gtx)
					})
			},
		)
	})
}

// centeredMaxWidth caps content width to maxW and centers it horizontally.
func centeredMaxWidth(gtx layout.Context, maxW unit.Dp, w layout.Widget) layout.Dimensions {
	target := gtx.Dp(maxW)
	if gtx.Constraints.Max.X <= target {
		return w(gtx)
	}
	side := (gtx.Constraints.Max.X - target) / 2
	sideDp := unit.Dp(float32(side) / gtx.Metric.PxPerDp)
	return layout.Inset{Left: sideDp, Right: sideDp}.Layout(gtx, w)
}
