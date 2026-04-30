package ui

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lewispb/wifiui/internal/iwd"
	"github.com/lewispb/wifiui/internal/netinfo"
	"github.com/lewispb/wifiui/internal/portal"
	"github.com/lewispb/wifiui/internal/theme"
)

// ---- header ----

func (a *App) header(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(a.th.Material, iconWifi, unit.Sp(18), font.Normal, pal.Accent).Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(a.th.Material, "wifiui", unit.Sp(22), font.Bold, pal.Text).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.scanButton(gtx, pal, s.scanning)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: theme.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.statusStrip(gtx, pal, s)
		}),
	)
}

func (a *App) scanButton(gtx layout.Context, pal theme.Palette, scanning bool) layout.Dimensions {
	col := pal.TextMid
	txt := "scan"
	if scanning {
		col = pal.Accent
		txt = "scanning"
	}
	return clipRRectClickable(gtx, unit.Dp(8), &a.scanBtn, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(unit.Dp(8))
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				paint.FillShape(gtx.Ops, pal.Surface, rr.Op(gtx.Ops))
				stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
				paint.ColorOp{Color: pal.Border}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				stk.Pop()
				return layout.Dimensions{Size: sz}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return label(a.th.Material, iconRefresh, unit.Sp(12), font.Normal, col).Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return label(a.th.Material, txt, unit.Sp(12), font.SemiBold, col).Layout(gtx)
							}),
						)
					})
			},
		)
	})
}

func (a *App) statusStrip(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	pipColor := pal.TextDim
	stateText := s.state
	switch s.state {
	case "connected":
		pipColor = pal.Success
	case "connecting", "roaming":
		pipColor = pal.Warning
	case "disconnected", "":
		pipColor = pal.Danger
	}
	if s.station == "" {
		stateText = "no adapter"
	}

	parts := []string{s.station}
	if s.vendor != "" {
		parts = append(parts, s.vendor)
	}
	parts = append(parts, stateText)
	if s.connected != "" {
		parts = append(parts, s.connected)
	}
	if s.signal != 0 {
		parts = append(parts, fmt.Sprintf("%d dBm", s.signal))
	}
	if s.band != "" {
		parts = append(parts, s.band)
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Right: unit.Dp(8)}.Layout(gtx, dot{Color: pipColor, Size: unit.Dp(8)}.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label(a.th.Material, strings.Join(parts, " · "), unit.Sp(12), font.Normal, pal.TextMid).Layout(gtx)
		}),
	)
}

// ---- body ----

func (a *App) body(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	pinned, others := splitPinned(s.networks, s.pinned)
	hasPortal := s.portalStatus == portal.StatusPortal

	count := sectionCount(len(s.interfaces), len(pinned), len(others), s.err, hasPortal)
	return material.List(a.th.Material, &a.scroll).Layout(gtx, count, func(gtx layout.Context, i int) layout.Dimensions {
		return a.bodyRow(gtx, pal, s, pinned, others, hasPortal, i)
	})
}

func (a *App) bodyRow(gtx layout.Context, pal theme.Palette, s *viewState, pinned, others []*iwd.Network, hasPortal bool, i int) layout.Dimensions {
	idx := i
	if s.err != "" {
		if idx == 0 {
			return a.errorBanner(gtx, pal, s.err)
		}
		idx--
	}
	if hasPortal {
		if idx == 0 {
			return a.portalBanner(gtx, pal, s)
		}
		idx--
	}
	if len(s.interfaces) > 0 {
		if idx == 0 {
			return sectionLabel(gtx, a.th.Material, pal, "CONNECTIONS")
		}
		idx--
		if idx < len(s.interfaces) {
			return layout.Inset{Top: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.interfaceTile(gtx, pal, s.interfaces[idx])
			})
		}
		idx -= len(s.interfaces)
		if idx == 0 {
			return layout.Spacer{Height: theme.S6}.Layout(gtx)
		}
		idx--
	}
	if len(pinned) > 0 {
		if idx == 0 {
			return sectionLabel(gtx, a.th.Material, pal, "PINNED")
		}
		idx--
		if idx < len(pinned) {
			return layout.Inset{Top: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.tile(gtx, pal, pinned[idx], true)
			})
		}
		idx -= len(pinned)
		if idx == 0 {
			return layout.Spacer{Height: theme.S6}.Layout(gtx)
		}
		idx--
	}
	if idx == 0 {
		return sectionLabel(gtx, a.th.Material, pal, "NETWORKS")
	}
	idx--
	if idx < len(others) {
		return layout.Inset{Top: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.tile(gtx, pal, others[idx], false)
		})
	}
	return layout.Dimensions{}
}

func sectionCount(status, pinned, others int, errMsg string, hasPortal bool) int {
	count := 0
	if errMsg != "" {
		count++
	}
	if hasPortal {
		count++
	}
	if status > 0 {
		count += 1 + status + 1
	}
	if pinned > 0 {
		count += 1 + pinned + 1
	}
	count += 1 + others
	return count
}

func (a *App) portalBanner(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	return layout.Inset{Bottom: theme.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				r := gtx.Dp(theme.RadiusCard)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), r)
				bg := pal.Warning
				bg.A = 0x22
				paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
				stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
				paint.ColorOp{Color: pal.Warning}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				stk.Pop()
				return layout.Dimensions{Size: sz}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return label(a.th.Material, iconExclamation, unit.Sp(14), font.Normal, pal.Warning).Layout(gtx)
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return label(a.th.Material, "captive portal detected", unit.Sp(13), font.SemiBold, pal.Text).Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									msg := "internet blocked until you sign in"
									if s.portalRedirect != "" {
										msg = s.portalRedirect
									}
									return label(a.th.Material, msg, unit.Sp(11), font.Normal, pal.TextDim).Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return primaryButton(gtx, a.th.Material, pal, &a.portalOpen, "open in browser")
						}),
					)
				})
			},
		)
	})
}

func splitPinned(nets []*iwd.Network, pin map[string]struct{}) (pinned, others []*iwd.Network) {
	for _, n := range nets {
		if _, ok := pin[n.SSID]; ok {
			pinned = append(pinned, n)
		} else {
			others = append(others, n)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool { return pinned[i].Signal > pinned[j].Signal })
	return
}

func sectionLabel(gtx layout.Context, th *material.Theme, pal theme.Palette, txt string) layout.Dimensions {
	return layout.Inset{Top: theme.S2, Bottom: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := label(th, txt, unit.Sp(11), font.SemiBold, pal.TextDim)
		l.Alignment = text.Start
		return l.Layout(gtx)
	})
}

func (a *App) errorBanner(gtx layout.Context, pal theme.Palette, msg string) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			r := gtx.Dp(theme.RadiusRow)
			rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), r)
			bg := pal.Danger
			bg.A = 0x22
			paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(a.th.Material, msg, unit.Sp(12), font.Normal, pal.Danger).Layout(gtx)
			})
		},
	)
}

// ---- tile ----

// tile is the unified "expanded" network row used everywhere. Click to
// connect; the right-edge pin glyph toggles the pinned state. When connected,
// a disconnect button appears next to the pin.
func (a *App) tile(gtx layout.Context, pal theme.Palette, n *iwd.Network, isPinned bool) layout.Dimensions {
	r := a.row(n.SSID)
	if r.connect.Clicked(gtx) && !n.Connected {
		a.triggerConnect(n.SSID)
	}
	if r.pin.Clicked(gtx) {
		a.togglePin(n.SSID)
	}
	if r.disconnect.Clicked(gtx) && n.Connected {
		a.triggerDisconnect()
	}
	borderColor := pal.Border
	if n.Connected {
		borderColor = pal.Accent
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return clipRRectClickable(gtx, theme.RadiusCard, &r.connect, func(gtx layout.Context) layout.Dimensions {
				return layout.Background{}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						sz := gtx.Constraints.Min
						rd := gtx.Dp(theme.RadiusCard)
						rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
						paint.FillShape(gtx.Ops, pal.Surface, rr.Op(gtx.Ops))
						stk := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
						paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						stk.Pop()
						return layout.Dimensions{Size: sz}
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.tileContent(gtx, pal, n)
						})
					},
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: theme.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !n.Connected {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: theme.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.disconnectButton(gtx, pal, &r.disconnect)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.pinToggle(gtx, pal, &r.pin, isPinned)
		}),
	)
}

func (a *App) disconnectButton(gtx layout.Context, pal theme.Palette, click *widget.Clickable) layout.Dimensions {
	return clipRRectClickable(gtx, theme.RadiusRow, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Dp(unit.Dp(44))
				return layout.Dimensions{Size: image.Pt(sz, sz)}
			}),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(theme.RadiusRow)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				bg := pal.Surface
				paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
				return layout.Dimensions{Size: sz}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return label(a.th.Material, iconTimes, unit.Sp(13), font.Normal, pal.Danger).Layout(gtx)
			}),
		)
	})
}

func (a *App) tileContent(gtx layout.Context, pal theme.Palette, n *iwd.Network) layout.Dimensions {
	q := qualityOf(n.Signal)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(10), Top: unit.Dp(2)}.Layout(gtx,
						dot{Color: pipColorFor(n, pal), Size: unit.Dp(8)}.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return label(a.th.Material, n.SSID, unit.Sp(17), font.SemiBold, pal.Text).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return signalBars{DBm: n.Signal, On: qualityColor(q, pal), Off: pal.Border}.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: theme.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.tileMeta(gtx, pal, n, q)
		}),
	)
}

func pipColorFor(n *iwd.Network, pal theme.Palette) color.NRGBA {
	switch {
	case n.Connected:
		return pal.Success
	case n.Known:
		return pal.Accent
	default:
		return pal.TextDim
	}
}

func (a *App) tileMeta(gtx layout.Context, pal theme.Palette, n *iwd.Network, q quality) layout.Dimensions {
	left := []string{}
	if n.Connected {
		left = append(left, iconCheck+" connected")
	} else if n.Known {
		left = append(left, "saved")
	}
	if b := n.Band(); b != "" {
		left = append(left, b)
	}
	left = append(left, secIcon(n.Type)+" "+secLabel(n.Type))
	leftStr := strings.Join(left, " · ") + " · "

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label(a.th.Material, leftStr, unit.Sp(12), font.Normal, pal.TextDim).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label(a.th.Material, fmt.Sprintf("%d%% ", signalPct(n.Signal)), unit.Sp(12), font.Normal, pal.TextDim).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label(a.th.Material, qualityLabel(q), unit.Sp(12), font.SemiBold, qualityColor(q, pal)).Layout(gtx)
		}),
	)
}

func secIcon(t string) string {
	if t == "open" {
		return iconUnlock
	}
	return iconLock
}

func secLabel(t string) string {
	switch t {
	case "open":
		return "open"
	case "psk":
		return "WPA"
	case "8021x":
		return "802.1X"
	case "wep":
		return "WEP"
	default:
		return t
	}
}

func (a *App) pinToggle(gtx layout.Context, pal theme.Palette, click *widget.Clickable, on bool) layout.Dimensions {
	bg := pal.Surface
	col := pal.TextDim
	if on {
		bg = pal.AccentDim
		col = pal.Accent
	}
	return clipRRectClickable(gtx, theme.RadiusRow, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Dp(unit.Dp(44))
				return layout.Dimensions{Size: image.Pt(sz, sz)}
			}),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				rd := gtx.Dp(theme.RadiusRow)
				rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), rd)
				paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
				return layout.Dimensions{Size: sz}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return label(a.th.Material, iconPin, unit.Sp(14), font.Normal, col).Layout(gtx)
			}),
		)
	})
}

// ---- connections panel ----

func (a *App) interfaceTile(gtx layout.Context, pal theme.Palette, iface netinfo.Interface) layout.Dimensions {
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
				return a.interfaceContent(gtx, pal, iface)
			})
		},
	)
}

func (a *App) interfaceContent(gtx layout.Context, pal theme.Palette, iface netinfo.Interface) layout.Dimensions {
	stateCol := pal.Success
	if iface.State != "up" {
		stateCol = pal.TextDim
	}

	sub := []string{strings.ToUpper(string(iface.Type))}
	if iface.SSID != "" {
		sub = append(sub, iface.SSID)
	}
	if iface.Speed != "" {
		sub = append(sub, iface.Speed)
	}
	if iface.Band != "" {
		sub = append(sub, iface.Band)
	}
	if iface.Gen != "" {
		sub = append(sub, iface.Gen)
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(a.th.Material, iface.Name, unit.Sp(15), font.SemiBold, pal.Text).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label(a.th.Material, strings.ToUpper(iface.State), unit.Sp(11), font.SemiBold, stateCol).Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label(a.th.Material, strings.Join(sub, " · "), unit.Sp(12), font.Normal, pal.TextDim).Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: theme.S3}.Layout),
		kvRow(a.th.Material, pal, "ipv4", strings.Join(iface.IPv4, ", ")),
		kvRow(a.th.Material, pal, "ipv6", firstIPv6(iface.IPv6)),
		kvRow(a.th.Material, pal, "gateway", iface.Gateway),
		kvRow(a.th.Material, pal, "dns", strings.Join(iface.DNS, ", ")),
		kvRow(a.th.Material, pal, "dhcp", dhcpStr(iface.DHCP)),
		kvRow(a.th.Material, pal, "mac", iface.MAC),
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func firstIPv6(addrs []string) string {
	for _, a := range addrs {
		if !strings.HasPrefix(a, "fe80:") {
			return a
		}
	}
	return ""
}

func dhcpStr(d *netinfo.DHCPLease) string {
	if d == nil || d.Server == "" {
		return ""
	}
	s := d.Server
	if !d.Expires.IsZero() {
		remaining := time.Until(d.Expires).Round(time.Minute)
		if remaining > 0 {
			s += " · " + humanDuration(remaining) + " left"
		}
	}
	return s
}

// ---- footer ----

func (a *App) footer(gtx layout.Context, pal theme.Palette, s *viewState) layout.Dimensions {
	var msg string
	switch {
	case s.scanning:
		msg = "scanning"
	case s.lastTick.IsZero():
		msg = "—"
	default:
		msg = fmt.Sprintf("updated %s ago", humanDuration(time.Since(s.lastTick)))
	}
	return label(a.th.Material, msg, unit.Sp(11), font.Normal, pal.TextDim).Layout(gtx)
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}
