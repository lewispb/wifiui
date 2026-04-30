package theme

import (
	"image/color"
	"testing"
)

func TestParseHex(t *testing.T) {
	tests := []struct {
		in   string
		want color.NRGBA
		ok   bool
	}{
		{"#1a1b26", color.NRGBA{0x1a, 0x1b, 0x26, 0xff}, true},
		{"#FFFFFF", color.NRGBA{0xff, 0xff, 0xff, 0xff}, true},
		{"#7AA2F7", color.NRGBA{0x7a, 0xa2, 0xf7, 0xff}, true},
		{"7aa2f7", color.NRGBA{}, false},
		{"#1234", color.NRGBA{}, false},
		{"#zzzzzz", color.NRGBA{}, false},
		{"", color.NRGBA{}, false},
	}
	for _, tc := range tests {
		got, ok := parseHex(tc.in)
		if ok != tc.ok {
			t.Errorf("parseHex(%q) ok=%v, want %v", tc.in, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Errorf("parseHex(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseFlatToml(t *testing.T) {
	input := `# comment
accent = "#7aa2f7"
foreground = "#a9b1d6"
[colors]
background = "#1a1b26"
color2 = "#9ece6a"
not_a_color = "blue"
`
	m := parseFlatToml([]byte(input))

	want := map[string]color.NRGBA{
		"accent":     {0x7a, 0xa2, 0xf7, 0xff},
		"foreground": {0xa9, 0xb1, 0xd6, 0xff},
		"background": {0x1a, 0x1b, 0x26, 0xff},
		"color2":     {0x9e, 0xce, 0x6a, 0xff},
	}
	for k, v := range want {
		if got := m[k]; got != v {
			t.Errorf("m[%q] = %v, want %v", k, got, v)
		}
	}
	if _, ok := m["not_a_color"]; ok {
		t.Errorf("non-hex value %q should not parse", "not_a_color")
	}
}

func TestPaletteFromTokyoNight(t *testing.T) {
	raw := `accent = "#7aa2f7"
foreground = "#a9b1d6"
background = "#1a1b26"
cursor = "#c0caf5"
color7 = "#787c99"
color2 = "#9ece6a"
color3 = "#e0af68"
color1 = "#f7768e"
`
	p := paletteFrom(parseFlatToml([]byte(raw)))

	checks := []struct {
		name string
		got  color.NRGBA
		want color.NRGBA
	}{
		{"Bg", p.Bg, color.NRGBA{0x1a, 0x1b, 0x26, 0xff}},
		{"Accent", p.Accent, color.NRGBA{0x7a, 0xa2, 0xf7, 0xff}},
		{"Text", p.Text, color.NRGBA{0xc0, 0xca, 0xf5, 0xff}},
		{"TextMid", p.TextMid, color.NRGBA{0xa9, 0xb1, 0xd6, 0xff}},
		{"TextDim", p.TextDim, color.NRGBA{0x78, 0x7c, 0x99, 0xff}},
		{"Success", p.Success, color.NRGBA{0x9e, 0xce, 0x6a, 0xff}},
		{"Warning", p.Warning, color.NRGBA{0xe0, 0xaf, 0x68, 0xff}},
		{"Danger", p.Danger, color.NRGBA{0xf7, 0x76, 0x8e, 0xff}},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestPaletteFromEmptyFallsBackToDefaults(t *testing.T) {
	p := paletteFrom(map[string]color.NRGBA{})
	def := defaultPalette()
	if p != def {
		t.Errorf("empty palette should equal default; got %+v", p)
	}
}

func TestMix(t *testing.T) {
	black := color.NRGBA{0x00, 0x00, 0x00, 0xff}
	white := color.NRGBA{0xff, 0xff, 0xff, 0xff}

	if got := mix(black, white, 0); got != black {
		t.Errorf("mix(t=0) = %v, want %v", got, black)
	}
	if got := mix(black, white, 1); got != white {
		t.Errorf("mix(t=1) = %v, want %v", got, white)
	}
	mid := mix(black, white, 0.5)
	if mid.R < 0x7e || mid.R > 0x80 {
		t.Errorf("mix(t=0.5).R = %x, want ~0x7f", mid.R)
	}
}
