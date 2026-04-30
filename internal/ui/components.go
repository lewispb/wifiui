package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lewispb/wifiui/internal/theme"
)

// signalBars draws a 4-bar signal indicator. Bars grow in height left to
// right; filled bars use `on`, the rest use `off`.
type signalBars struct {
	DBm int
	On  color.NRGBA
	Off color.NRGBA
}

func (s signalBars) Layout(gtx layout.Context) layout.Dimensions {
	const bars = 4
	barW := gtx.Dp(unit.Dp(3))
	gap := gtx.Dp(unit.Dp(2))
	maxH := gtx.Dp(unit.Dp(13))
	level := dbmLevel(s.DBm)
	width := bars*barW + (bars-1)*gap
	for i := 0; i < bars; i++ {
		bH := maxH * (i + 1) / bars
		x := i * (barW + gap)
		y := maxH - bH
		col := s.Off
		if i < level {
			col = s.On
		}
		rr := clip.UniformRRect(image.Rect(x, y, x+barW, maxH), 1)
		paint.FillShape(gtx.Ops, col, rr.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: image.Pt(width, maxH)}
}

func dbmLevel(dbm int) int {
	switch {
	case dbm == 0:
		return 0
	case dbm >= -50:
		return 4
	case dbm >= -60:
		return 3
	case dbm >= -70:
		return 2
	case dbm >= -80:
		return 1
	default:
		return 0
	}
}

// dot renders a small filled circle.
type dot struct {
	Color color.NRGBA
	Size  unit.Dp
}

func (d dot) Layout(gtx layout.Context) layout.Dimensions {
	if d.Size == 0 {
		d.Size = unit.Dp(8)
	}
	sz := gtx.Dp(d.Size)
	rect := image.Rect(0, 0, sz, sz)
	defer clip.Ellipse(rect).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: d.Color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(sz, sz)}
}

// pill is a small rounded label used for tags (band, security, gen).
type pill struct {
	Label string
	Fg    color.NRGBA
	Bg    color.NRGBA
	Theme *material.Theme
	Size  unit.Sp
}

func (p pill) Layout(gtx layout.Context) layout.Dimensions {
	if p.Size == 0 {
		p.Size = unit.Sp(11)
	}
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			r := sz.Y / 2
			rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), r)
			paint.FillShape(gtx.Ops, p.Bg, rr.Op(gtx.Ops))
			return layout.Dimensions{Size: sz}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(3), Bottom: unit.Dp(3),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.LabelStyle{
					Text:     p.Label,
					Color:    p.Fg,
					Font:     font.Font{Typeface: theme.Mono, Weight: font.SemiBold},
					TextSize: p.Size,
					Shaper:   p.Theme.Shaper,
					Alignment: text.Middle,
				}
				return l.Layout(gtx)
			})
		},
	)
}

// label is a thin wrapper around material.LabelStyle that defaults to our mono
// typeface and lets the caller pick weight/size/color.
func label(th *material.Theme, txt string, size unit.Sp, weight font.Weight, col color.NRGBA) material.LabelStyle {
	return material.LabelStyle{
		Text:     txt,
		Color:    col,
		Font:     font.Font{Typeface: theme.Mono, Weight: weight},
		TextSize: size,
		Shaper:   th.Shaper,
	}
}

// fillRoundedRect draws a filled rounded rectangle of the constraint min size.
func fillRoundedRect(gtx layout.Context, c color.NRGBA, r unit.Dp) layout.Dimensions {
	sz := gtx.Constraints.Min
	rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), gtx.Dp(r))
	paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
	return layout.Dimensions{Size: sz}
}

// strokeRoundedRect draws a 1-px stroked rounded rectangle of the constraint
// min size.
func strokeRoundedRect(gtx layout.Context, c color.NRGBA, r unit.Dp) layout.Dimensions {
	sz := gtx.Constraints.Min
	rr := clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), gtx.Dp(r))
	stack := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(unit.Dp(1)))}.Op().Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
	return layout.Dimensions{Size: sz}
}

// clipRRectClickable wraps a material.Clickable so its hover/press overlay is
// clipped to a rounded rectangle. Records the click region into a macro, then
// replays it inside an UniformRRect clip sized to the measured content.
func clipRRectClickable(gtx layout.Context, radius unit.Dp, click *widget.Clickable, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := material.Clickable(gtx, click, w)
	call := macro.Stop()

	rd := gtx.Dp(radius)
	rr := clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), rd)
	defer rr.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

// quality is a coarse three-step rating of a signal in dBm.
type quality int

const (
	qualityPoor quality = iota
	qualityOK
	qualityGood
)

func qualityOf(dbm int) quality {
	switch {
	case dbm == 0:
		return qualityPoor
	case dbm >= -60:
		return qualityGood
	case dbm >= -75:
		return qualityOK
	default:
		return qualityPoor
	}
}

// signalPct maps a dBm reading to an approximate 0..100 percentage,
// using the common linear approximation pct = 2 * (dBm + 100) clamped.
func signalPct(dbm int) int {
	if dbm == 0 {
		return 0
	}
	pct := 2 * (dbm + 100)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func qualityLabel(q quality) string {
	switch q {
	case qualityGood:
		return "good"
	case qualityOK:
		return "ok"
	default:
		return "poor"
	}
}

func qualityColor(q quality, pal theme.Palette) color.NRGBA {
	switch q {
	case qualityGood:
		return pal.Success
	case qualityOK:
		return pal.Warning
	default:
		return pal.Danger
	}
}

// kvRow renders a "label    value" line with a minimum-width key column so
// values align in monospace.
func kvRow(th *material.Theme, pal theme.Palette, k, v string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if v == "" {
			return layout.Dimensions{}
		}
		return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := label(th, k, unit.Sp(12), font.Normal, pal.TextDim).Layout(gtx)
					minW := gtx.Dp(unit.Dp(78))
					if d.Size.X < minW {
						d.Size.X = minW
					}
					return d
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return label(th, v, unit.Sp(12), font.Normal, pal.TextMid).Layout(gtx)
				}),
			)
		})
	})
}
