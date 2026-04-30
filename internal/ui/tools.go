package ui

import (
	"context"
	"errors"
	"image"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lewispb/wifiui/internal/netinfo"
	"github.com/lewispb/wifiui/internal/portal"
	"github.com/lewispb/wifiui/internal/theme"
)

// toolDef describes a single diagnostic command.
type toolDef struct {
	Label string
	Run   func(ctx context.Context, s *viewState) (string, error)
	click widget.Clickable
}

// toolsPanel owns all state for the diagnostics tab.
type toolsPanel struct {
	app *App

	defs   []toolDef
	scroll widget.List

	mu      sync.Mutex
	output  string
	running string
}

func newToolsPanel(a *App) *toolsPanel {
	t := &toolsPanel{app: a}
	t.scroll.Axis = layout.Vertical
	t.defs = builtinTools()
	return t
}

func builtinTools() []toolDef {
	return []toolDef{
		{
			Label: "ping gateway",
			Run: func(ctx context.Context, s *viewState) (string, error) {
				gw := primaryGateway(s.interfaces)
				if gw == "" {
					return "", errors.New("no gateway detected")
				}
				return runCmd(ctx, "ping", "-c", "3", "-W", "2", gw)
			},
		},
		{
			Label: "ping 1.1.1.1",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				return runCmd(ctx, "ping", "-c", "3", "-W", "2", "1.1.1.1")
			},
		},
		{
			Label: "dns lookup (google.com)",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				if _, err := exec.LookPath("dig"); err == nil {
					return runCmd(ctx, "dig", "+short", "+timeout=2", "google.com")
				}
				return runCmd(ctx, "host", "google.com")
			},
		},
		{
			Label: "captive portal",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				r := portal.Check(ctx)
				switch r.Status {
				case portal.StatusOK:
					return "no captive portal — internet reachable", nil
				case portal.StatusPortal:
					if r.RedirectURL != "" {
						return "captive portal at " + r.RedirectURL, nil
					}
					return "captive portal detected", nil
				case portal.StatusOffline:
					if r.Err != nil {
						return "no connectivity", r.Err
					}
					return "no connectivity", nil
				}
				return "unknown", nil
			},
		},
		{
			Label: "tracepath 1.1.1.1",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				return runCmd(ctx, "tracepath", "1.1.1.1")
			},
		},
		{
			Label: "ip routes",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				return runCmd(ctx, "ip", "route", "show")
			},
		},
		{
			Label: "resolvectl status",
			Run: func(ctx context.Context, _ *viewState) (string, error) {
				return runCmd(ctx, "resolvectl", "--no-pager", "status")
			},
		},
		{
			Label: "iw link",
			Run: func(ctx context.Context, s *viewState) (string, error) {
				if s.station == "" {
					return "", errors.New("no wifi station")
				}
				return runCmd(ctx, "iw", "dev", s.station, "link")
			},
		},
	}
}

func primaryGateway(ifaces []netinfo.Interface) string {
	for _, i := range ifaces {
		if i.Gateway != "" {
			return i.Gateway
		}
	}
	return ""
}

func runCmd(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

func (t *toolsPanel) run(td *toolDef, snap *viewState) {
	t.mu.Lock()
	t.running = td.Label
	t.output = ""
	t.mu.Unlock()
	t.app.w.Invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), toolDeadline)
		defer cancel()
		out, err := td.Run(ctx, snap)
		if out == "" && err != nil {
			out = err.Error()
		} else if err != nil {
			out += "\n[exit: " + err.Error() + "]"
		}
		t.mu.Lock()
		t.output = out
		t.running = ""
		t.mu.Unlock()
		t.app.w.Invalidate()
	}()
}

func (t *toolsPanel) view(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	for i := range t.defs {
		td := &t.defs[i]
		if td.click.Clicked(gtx) {
			t.run(td, s)
		}
	}

	t.mu.Lock()
	output := t.output
	running := t.running
	t.mu.Unlock()

	return material.List(t.app.th.Material, &t.scroll).Layout(gtx, 4+len(t.defs), func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			return sectionLabel(gtx, t.app.th.Material, pal, "DIAGNOSTICS")
		case len(t.defs) + 1:
			return layout.Spacer{Height: theme.S5}.Layout(gtx)
		case len(t.defs) + 2:
			return sectionLabel(gtx, t.app.th.Material, pal, "OUTPUT")
		case len(t.defs) + 3:
			return layout.Inset{Top: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return t.outputCard(gtx, pal, output, running)
			})
		default:
			td := &t.defs[i-1]
			return layout.Inset{Top: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return t.button(gtx, pal, td, running == td.Label)
			})
		}
	})
}

func (t *toolsPanel) button(gtx layout.Context, pal theme.Palette, td *toolDef, running bool) layout.Dimensions {
	col := pal.Text
	right := ""
	if running {
		col = pal.Accent
		right = "running…"
	}
	return clipRRectClickable(gtx, theme.RadiusRow, &td.click, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(theme.RadiusRow)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				paint.FillShape(gtx.Ops, pal.Surface, rr.Op(gtx.Ops))
				stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
				paint.ColorOp{Color: pal.Border}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				stk.Pop()
				return layout.Dimensions{Size: sz}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return label(t.app.th.Material, td.Label, unit.Sp(13), font.Normal, col).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if right == "" {
								return layout.Dimensions{}
							}
							return label(t.app.th.Material, right, unit.Sp(11), font.Normal, pal.Accent).Layout(gtx)
						}),
					)
				})
			},
		)
	})
}

func (t *toolsPanel) outputCard(gtx layout.Context, pal theme.Palette, output, running string) layout.Dimensions {
	display := output
	if display == "" {
		if running != "" {
			display = "$ " + running + " …"
		} else {
			display = "(no output yet — pick a diagnostic above)"
		}
	}
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			rd := gtx.Dp(theme.RadiusCard)
			rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
			paint.FillShape(gtx.Ops, pal.Surface, rr.Op(gtx.Ops))
			stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
			paint.ColorOp{Color: pal.Border}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			stk.Pop()
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col := pal.TextMid
				if output == "" {
					col = pal.TextDim
				}
				return label(t.app.th.Material, display, unit.Sp(12), font.Normal, col).Layout(gtx)
			})
		},
	)
}
