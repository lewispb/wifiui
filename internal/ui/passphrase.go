package ui

import (
	"context"
	"errors"
	"image"
	"sync"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/godbus/dbus/v5"

	"github.com/lewispb/wifiui/internal/onepw"
	"github.com/lewispb/wifiui/internal/theme"
)

// pwRequest is sent by the agent goroutine to the UI to request a passphrase.
// The UI fulfills it by sending a pwReply.
type pwRequest struct {
	SSID  string
	Reply chan pwReply
}

type pwReply struct {
	Pass string
	Err  error
}

// pwSuggestion ties a 1Password lookup result to the request that triggered
// it, so a slow lookup can't bleed into a subsequent prompt.
type pwSuggestion struct {
	Req  *pwRequest
	Pass string
}

// passphrasePrompt is the UI's interactive passphrase dialog plus the channel
// machinery used to talk to the iwd agent goroutine. There is at most one
// active prompt at a time.
type passphrasePrompt struct {
	app *App // for window invalidation, used by handler

	queue   chan *pwRequest    // depth 1: hands a request from agent → UI
	suggest chan pwSuggestion  // depth 1: hands a 1Password value to the UI

	active   *pwRequest
	prefill  string // last 1Password value applied to the editor

	// last cancellation reason from iwd (e.g. wrong passphrase).
	cancelReason string

	// widgets
	input  widget.Editor
	submit widget.Clickable
	cancel widget.Clickable
	show   widget.Clickable
	showOn bool

	// pendingSave tracks a manually-entered passphrase to persist back to
	// 1Password if the connection succeeds.
	saveMu      sync.Mutex
	pendingSave pendingSave
}

// pendingSave tracks a manually-entered passphrase to persist back to
// 1Password if the connection succeeds.
type pendingSave struct {
	ssid  string
	pass  string
	valid bool
}

func newPassphrasePrompt(a *App) *passphrasePrompt {
	return &passphrasePrompt{
		app:     a,
		queue:   make(chan *pwRequest, 1),
		suggest: make(chan pwSuggestion, 1),
	}
}

// handleRequest is registered with iwd's AgentManager. It blocks until the UI
// submits or cancels the prompt.
func (p *passphrasePrompt) handleRequest(netPath dbus.ObjectPath) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), passphraseTimeout)
	defer cancel()

	ssid, _ := p.app.c.NetworkSSID(ctx, netPath)
	if ssid == "" {
		ssid = "wifi"
	}

	req := &pwRequest{SSID: ssid, Reply: make(chan pwReply, 1)}
	select {
	case p.queue <- req:
	default:
		return "", errors.New("another passphrase prompt is active")
	}
	p.app.w.Invalidate()

	// Try 1Password concurrently; pre-fill the editor if found.
	go func() {
		lctx, lcancel := context.WithTimeout(context.Background(), onepwLookupTimeout)
		defer lcancel()
		if pass, ok := onepw.Lookup(lctx, ssid); ok {
			select {
			case p.suggest <- pwSuggestion{Req: req, Pass: pass}:
				p.app.w.Invalidate()
			default:
			}
		}
	}()

	select {
	case r := <-req.Reply:
		if r.Err != nil {
			return "", r.Err
		}
		p.recordPendingSave(ssid, r.Pass)
		return r.Pass, nil
	case <-ctx.Done():
		return "", errors.New("passphrase prompt timed out")
	}
}

// onAgentCancel surfaces an iwd-initiated cancellation (e.g. wrong passphrase)
// to the active prompt. Called from the agent's Cancel D-Bus method.
func (p *passphrasePrompt) onAgentCancel(reason string) {
	if reason == "" {
		reason = "cancelled by iwd"
	}
	p.cancelReason = reason
	p.app.w.Invalidate()
}

// recordPendingSave records a manually-entered passphrase for save-on-connect.
// Skipped when the value matches the prefilled 1Password value.
func (p *passphrasePrompt) recordPendingSave(ssid, pass string) {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()
	p.pendingSave = pendingSave{
		ssid:  ssid,
		pass:  pass,
		valid: pass != "" && pass != p.prefill,
	}
}

func (p *passphrasePrompt) clearPendingSave() {
	p.saveMu.Lock()
	p.pendingSave = pendingSave{}
	p.saveMu.Unlock()
}

func (p *passphrasePrompt) takePendingSave() pendingSave {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()
	save := p.pendingSave
	p.pendingSave = pendingSave{}
	return save
}

// drain pulls any new request or suggestion into UI state. Called once per
// frame from layout.
func (p *passphrasePrompt) drain(gtx layout.Context) {
	select {
	case req := <-p.queue:
		p.active = req
		p.input.SetText("")
		p.prefill = ""
		p.cancelReason = ""
		gtx.Execute(key.FocusCmd{Tag: &p.input})
	default:
	}
	select {
	case s := <-p.suggest:
		// Only apply if the suggestion belongs to the active request and the
		// user hasn't typed anything yet.
		if p.active != nil && s.Req == p.active && p.input.Text() == "" {
			p.input.SetText(s.Pass)
			p.prefill = s.Pass
		}
	default:
	}
}

// active reports whether a prompt is currently visible.
func (p *passphrasePrompt) hasActive() bool { return p.active != nil }

// view replaces the body when a prompt is active.
func (p *passphrasePrompt) view(gtx layout.Context, pal theme.Palette) layout.Dimensions {
	req := p.active
	if req == nil {
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &p.input, Name: key.NameReturn},
			key.Filter{Focus: &p.input, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
			switch ke.Name {
			case key.NameReturn:
				p.submitNow()
			case key.NameEscape:
				p.cancelNow()
			}
		}
	}

	if p.submit.Clicked(gtx) {
		p.submitNow()
	}
	if p.cancel.Clicked(gtx) {
		p.cancelNow()
	}
	if p.show.Clicked(gtx) {
		p.showOn = !p.showOn
	}

	p.input.SingleLine = true
	if p.showOn {
		p.input.Mask = 0
	} else {
		p.input.Mask = '•'
	}

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(420)))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return label(p.app.th.Material, "CONNECT TO", unit.Sp(11), font.SemiBold, pal.TextDim).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: theme.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return label(p.app.th.Material, req.SSID, unit.Sp(20), font.Bold, pal.Text).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: theme.S5}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.field(gtx, pal)
			}),
			layout.Rigid(layout.Spacer{Height: theme.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.hintLine(gtx, pal)
			}),
			layout.Rigid(layout.Spacer{Height: theme.S6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.buttons(gtx, pal)
			}),
		)
	})
}

func (p *passphrasePrompt) hintLine(gtx layout.Context, pal theme.Palette) layout.Dimensions {
	switch {
	case p.cancelReason != "":
		return label(p.app.th.Material, p.cancelReason, unit.Sp(11), font.Normal, pal.Danger).Layout(gtx)
	case p.prefill != "" && p.input.Text() == p.prefill:
		return label(p.app.th.Material, "from 1Password", unit.Sp(11), font.Normal, pal.Accent).Layout(gtx)
	}
	return layout.Dimensions{}
}

func (p *passphrasePrompt) field(gtx layout.Context, pal theme.Palette) layout.Dimensions {
	eye := iconEye
	if p.showOn {
		eye = iconEyeSlash
	}
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			rd := gtx.Dp(theme.RadiusRow)
			rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
			paint.FillShape(gtx.Ops, pal.Surface, rr.Op(gtx.Ops))
			stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
			paint.ColorOp{Color: pal.BorderHi}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			stk.Pop()
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(14), Right: unit.Dp(8)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(p.app.th.Material, &p.input, "passphrase")
							ed.Font = font.Font{Typeface: theme.Mono, Weight: font.Normal}
							ed.TextSize = unit.Sp(15)
							ed.Color = pal.Text
							ed.HintColor = pal.TextDim
							return ed.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return clipRRectClickable(gtx, unit.Dp(6), &p.show, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return label(p.app.th.Material, eye, unit.Sp(14), font.Normal, pal.TextDim).Layout(gtx)
								})
							})
						}),
					)
				})
		},
	)
}

func (p *passphrasePrompt) buttons(gtx layout.Context, pal theme.Palette) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ghostButton(gtx, p.app.th.Material, pal, &p.cancel, "cancel")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return primaryButton(gtx, p.app.th.Material, pal, &p.submit, "connect")
		}),
	)
}

func (p *passphrasePrompt) submitNow() {
	if p.active == nil {
		return
	}
	p.active.Reply <- pwReply{Pass: p.input.Text()}
	p.active = nil
	p.app.w.Invalidate()
}

func (p *passphrasePrompt) cancelNow() {
	if p.active == nil {
		return
	}
	p.active.Reply <- pwReply{Err: errors.New("user cancelled")}
	p.active = nil
	p.clearPendingSave()
	p.app.w.Invalidate()
}

// persistOnSuccess writes the last submitted passphrase to 1Password if it was
// a manual entry (not the prefilled value). Best-effort, fire-and-forget.
func (p *passphrasePrompt) persistOnSuccess() {
	save := p.takePendingSave()
	if !save.valid {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), onepwSaveTimeout)
		defer cancel()
		_ = onepw.Save(ctx, save.ssid, save.pass)
	}()
}

// ghostButton renders a flat text button (used for secondary actions).
func ghostButton(gtx layout.Context, th *material.Theme, pal theme.Palette, click *widget.Clickable, txt string) layout.Dimensions {
	return material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := label(th, txt, unit.Sp(13), font.SemiBold, pal.TextDim)
			l.Alignment = text.Middle
			return l.Layout(gtx)
		})
	})
}

// primaryButton renders an accent-filled button (used for the primary action).
func primaryButton(gtx layout.Context, th *material.Theme, pal theme.Palette, click *widget.Clickable, txt string) layout.Dimensions {
	return clipRRectClickable(gtx, theme.RadiusRow, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(theme.RadiusRow)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				paint.FillShape(gtx.Ops, pal.Accent, rr.Op(gtx.Ops))
				return layout.Dimensions{Size: sz}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := label(th, txt, unit.Sp(13), font.SemiBold, pal.Bg)
					l.Alignment = text.Middle
					return l.Layout(gtx)
				})
			},
		)
	})
}

