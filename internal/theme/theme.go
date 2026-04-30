package theme

import (
	"context"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Palette holds the semantic colors used across wifiui. It is derived from
// the active Omarchy theme's colors.toml.
type Palette struct {
	Bg, Surface, SurfaceHi    color.NRGBA
	Border, BorderHi          color.NRGBA
	Text, TextMid, TextDim    color.NRGBA
	Accent, AccentDim         color.NRGBA
	Success, Warning, Danger  color.NRGBA
}

// Theme bundles the Material theme (for the shaper) with a hot-reloadable
// Palette pulled from Omarchy.
type Theme struct {
	Material *material.Theme
	pal      atomic.Pointer[Palette]
}

// New constructs a Theme by loading the active Omarchy palette. Falls back to
// Tokyo Night defaults if Omarchy isn't installed or the file is unreadable.
func New() *Theme {
	t := &Theme{Material: newMaterial()}
	pal := loadOmarchyPalette()
	t.pal.Store(&pal)
	t.applyMaterial(pal)
	return t
}

// Palette returns the current palette snapshot. Safe to call from the UI on
// every frame.
func (t *Theme) Palette() Palette {
	return *t.pal.Load()
}

// Watch polls the active Omarchy theme file and swaps the palette atomically
// when it changes, invoking onChange (typically window.Invalidate). The
// goroutine exits when ctx is cancelled.
func (t *Theme) Watch(ctx context.Context, onChange func()) {
	go func() {
		path, err := omarchyColorsPath()
		if err != nil {
			return
		}
		var lastMod time.Time
		var lastTarget string
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				continue
			}
			info, err := os.Stat(resolved)
			if err != nil {
				continue
			}
			mod := info.ModTime()
			if mod.Equal(lastMod) && resolved == lastTarget {
				continue
			}
			lastMod = mod
			lastTarget = resolved
			data, err := os.ReadFile(resolved)
			if err != nil {
				continue
			}
			pal := paletteFrom(parseFlatToml(data))
			t.pal.Store(&pal)
			t.applyMaterial(pal)
			if onChange != nil {
				onChange()
			}
		}
	}()
}

func (t *Theme) applyMaterial(p Palette) {
	t.Material.Palette.Bg = p.Bg
	t.Material.Palette.Fg = p.Text
	t.Material.Palette.ContrastBg = p.Accent
	t.Material.Palette.ContrastFg = p.Bg
}

// Spacing scale, 4 dp base.
const (
	S1  = unit.Dp(4)
	S2  = unit.Dp(8)
	S3  = unit.Dp(12)
	S4  = unit.Dp(16)
	S5  = unit.Dp(20)
	S6  = unit.Dp(24)
	S8  = unit.Dp(32)
	S12 = unit.Dp(48)
)

// Radii.
const (
	RadiusPill = unit.Dp(999)
	RadiusCard = unit.Dp(10)
	RadiusRow  = unit.Dp(8)
)

// Mono is the typeface name registered in the shaper.
const Mono = "Mono"

// Alpha returns c with the given alpha (0–255).
func Alpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func newMaterial() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(loadFonts()))
	return th
}

func loadFonts() []font.FontFace {
	families := []string{
		"CaskaydiaMono Nerd Font",
		"CaskaydiaCove Nerd Font",
		"JetBrainsMono Nerd Font",
	}
	weights := []struct {
		style string
		w     font.Weight
	}{
		{"Light", font.Light},
		{"Regular", font.Normal},
		{"SemiBold", font.SemiBold},
		{"Bold", font.Bold},
	}
	var coll []font.FontFace
	for _, fam := range families {
		for _, wt := range weights {
			path, ok := fcMatch(fam+":style="+wt.style, fam)
			if !ok {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			face, err := opentype.Parse(data)
			if err != nil {
				continue
			}
			coll = append(coll, font.FontFace{
				Font: font.Font{Typeface: Mono, Weight: wt.w},
				Face: face,
			})
		}
		if len(coll) >= 2 {
			break
		}
	}
	if len(coll) == 0 {
		return gofont.Collection()
	}
	return coll
}

func fcMatch(query, want string) (string, bool) {
	out, err := exec.Command("fc-match", "-f", "%{family[0]}|%{file}\n", query).Output()
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	if len(parts) != 2 {
		return "", false
	}
	got := strings.ToLower(strings.ReplaceAll(parts[0], " ", ""))
	first := strings.ToLower(strings.ReplaceAll(strings.SplitN(want, " ", 2)[0], " ", ""))
	if !strings.Contains(got, first) {
		return "", false
	}
	return parts[1], true
}

func omarchyColorsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/omarchy/current/theme/colors.toml"), nil
}

func loadOmarchyPalette() Palette {
	path, err := omarchyColorsPath()
	if err != nil {
		return defaultPalette()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultPalette()
	}
	return paletteFrom(parseFlatToml(data))
}

func parseFlatToml(data []byte) map[string]color.NRGBA {
	out := map[string]color.NRGBA{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"`)
		if c, ok := parseHex(val); ok {
			out[key] = c
		}
	}
	return out
}

func parseHex(s string) (color.NRGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, false
	}
	var n uint32
	for i := 1; i < 7; i++ {
		var v uint32
		switch d := s[i]; {
		case d >= '0' && d <= '9':
			v = uint32(d - '0')
		case d >= 'a' && d <= 'f':
			v = uint32(d-'a') + 10
		case d >= 'A' && d <= 'F':
			v = uint32(d-'A') + 10
		default:
			return color.NRGBA{}, false
		}
		n = (n << 4) | v
	}
	return color.NRGBA{R: byte(n >> 16), G: byte(n >> 8), B: byte(n), A: 0xff}, true
}

// paletteFrom merges the parsed theme map over the default seed and derives
// the full Palette. With an empty map the result equals defaultPalette().
func paletteFrom(m map[string]color.NRGBA) Palette {
	return paletteFromSeed(merged(defaultSeed(), m))
}

func paletteFromSeed(m map[string]color.NRGBA) Palette {
	bg := m["background"]
	fg := m["foreground"]
	cursor := m["cursor"]
	accent := m["accent"]
	return Palette{
		Bg:        bg,
		Surface:   mix(bg, fg, 0.05),
		SurfaceHi: mix(bg, fg, 0.10),
		Border:    mix(bg, fg, 0.10),
		BorderHi:  mix(bg, fg, 0.20),
		Text:      cursor,
		TextMid:   fg,
		TextDim:   m["color7"],
		Accent:    accent,
		AccentDim: mix(accent, bg, 0.55),
		Success:   m["color2"],
		Warning:   m["color3"],
		Danger:    m["color1"],
	}
}

func merged(base, override map[string]color.NRGBA) map[string]color.NRGBA {
	out := make(map[string]color.NRGBA, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: byte(float64(a.R)*(1-t) + float64(b.R)*t),
		G: byte(float64(a.G)*(1-t) + float64(b.G)*t),
		B: byte(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 0xff,
	}
}

// defaultSeed is the Tokyo Night-derived fallback colour set used when an
// Omarchy colors.toml is missing or doesn't specify a key.
func defaultSeed() map[string]color.NRGBA {
	return map[string]color.NRGBA{
		"background": rgb(0x1a1b26),
		"foreground": rgb(0xa9b1d6),
		"cursor":     rgb(0xc0caf5),
		"accent":     rgb(0x7aa2f7),
		"color1":     rgb(0xf7768e),
		"color2":     rgb(0x9ece6a),
		"color3":     rgb(0xe0af68),
		"color7":     rgb(0x787c99),
	}
}

func defaultPalette() Palette {
	return paletteFromSeed(defaultSeed())
}

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: byte(hex >> 16), G: byte(hex >> 8), B: byte(hex), A: 0xff}
}
