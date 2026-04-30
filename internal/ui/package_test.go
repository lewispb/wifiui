package ui

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/lewispb/wifiui/internal/iwd"
	"github.com/lewispb/wifiui/internal/netinfo"
	"github.com/lewispb/wifiui/internal/theme"
)

func TestQualityOf(t *testing.T) {
	cases := []struct {
		dbm  int
		want quality
	}{
		{-30, qualityGood},
		{-50, qualityGood},
		{-60, qualityGood},
		{-65, qualityOK},
		{-75, qualityOK},
		{-76, qualityPoor},
		{-90, qualityPoor},
		{0, qualityPoor},
	}
	for _, tc := range cases {
		if got := qualityOf(tc.dbm); got != tc.want {
			t.Errorf("qualityOf(%d) = %v, want %v", tc.dbm, got, tc.want)
		}
	}
}

func TestSignalPct(t *testing.T) {
	cases := []struct {
		dbm  int
		want int
	}{
		{-30, 100},
		{-50, 100},
		{-55, 90},
		{-60, 80},
		{-70, 60},
		{-80, 40},
		{-100, 0},
		{-200, 0},
		{0, 0},
	}
	for _, tc := range cases {
		if got := signalPct(tc.dbm); got != tc.want {
			t.Errorf("signalPct(%d) = %d, want %d", tc.dbm, got, tc.want)
		}
	}
}

func TestQualityLabel(t *testing.T) {
	if qualityLabel(qualityGood) != "good" {
		t.Errorf("good")
	}
	if qualityLabel(qualityOK) != "ok" {
		t.Errorf("ok")
	}
	if qualityLabel(qualityPoor) != "poor" {
		t.Errorf("poor")
	}
}

func TestQualityColor(t *testing.T) {
	pal := theme.Palette{
		Success: color.NRGBA{1, 0, 0, 0xff},
		Warning: color.NRGBA{0, 1, 0, 0xff},
		Danger:  color.NRGBA{0, 0, 1, 0xff},
	}
	if got := qualityColor(qualityGood, pal); got != pal.Success {
		t.Errorf("good = %v", got)
	}
	if got := qualityColor(qualityOK, pal); got != pal.Warning {
		t.Errorf("ok = %v", got)
	}
	if got := qualityColor(qualityPoor, pal); got != pal.Danger {
		t.Errorf("poor = %v", got)
	}
}

func TestSecLabel(t *testing.T) {
	cases := map[string]string{
		"open":  "open",
		"psk":   "WPA",
		"8021x": "802.1X",
		"wep":   "WEP",
		"other": "other",
	}
	for k, want := range cases {
		if got := secLabel(k); got != want {
			t.Errorf("secLabel(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestSecIcon(t *testing.T) {
	if secIcon("open") != iconUnlock {
		t.Errorf("open should be unlock")
	}
	if secIcon("psk") != iconLock {
		t.Errorf("psk should be lock")
	}
	if secIcon("8021x") != iconLock {
		t.Errorf("8021x should be lock")
	}
}

func TestCleanVendor(t *testing.T) {
	cases := map[string]string{
		"NetGear, Inc.": "NetGear",
		"Foo, Bar":      "Foo",
		"Acme.":         "Acme",
		"Plain":         "Plain",
		"  Spaced  ":    "Spaced",
		"":              "",
	}
	for in, want := range cases {
		if got := cleanVendor(in); got != want {
			t.Errorf("cleanVendor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "0s"},
		{1 * time.Second, "1s"},
		{30 * time.Second, "30s"},
		{2 * time.Minute, "2m"},
		{59 * time.Minute, "59m"},
		{3 * time.Hour, "3h"},
		{25 * time.Hour, "25h"},
	}
	for _, tc := range cases {
		if got := humanDuration(tc.d); got != tc.want {
			t.Errorf("humanDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestSplitPinned(t *testing.T) {
	nets := []*iwd.Network{
		{SSID: "iphone", Signal: -45},
		{SSID: "cafe", Signal: -75},
		{SSID: "home", Signal: -50},
	}
	pin := map[string]struct{}{
		"iphone": {},
		"home":   {},
	}
	pinned, others := splitPinned(nets, pin)

	if len(pinned) != 2 {
		t.Fatalf("len(pinned) = %d", len(pinned))
	}
	if pinned[0].SSID != "iphone" {
		t.Errorf("pinned[0] = %q, want iphone", pinned[0].SSID)
	}
	if pinned[1].SSID != "home" {
		t.Errorf("pinned[1] = %q, want home", pinned[1].SSID)
	}
	if len(others) != 1 || others[0].SSID != "cafe" {
		t.Errorf("others = %v", others)
	}
}

func TestSectionCount(t *testing.T) {
	cases := []struct {
		name                   string
		status, pinned, others int
		errMsg                 string
		hasPortal              bool
		want                   int
	}{
		{"empty", 0, 0, 0, "", false, 1},
		{"connections + networks", 2, 0, 5, "", false, 1 + 2 + 1 + 1 + 5},
		{"pinned + networks", 0, 2, 3, "", false, 1 + 2 + 1 + 1 + 3},
		{"all sections + err + portal", 2, 1, 5, "boom", true, 1 + 1 + (1 + 2 + 1) + (1 + 1 + 1) + (1 + 5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sectionCount(tc.status, tc.pinned, tc.others, tc.errMsg, tc.hasPortal)
			if got != tc.want {
				t.Errorf("sectionCount(%+v) = %d, want %d", tc, got, tc.want)
			}
		})
	}
}

func TestDhcpStr(t *testing.T) {
	if dhcpStr(nil) != "" {
		t.Errorf("nil should be empty")
	}
	if dhcpStr(&netinfo.DHCPLease{}) != "" {
		t.Errorf("zero lease should be empty")
	}

	d := &netinfo.DHCPLease{Server: "192.168.1.1"}
	if got := dhcpStr(d); got != "192.168.1.1" {
		t.Errorf("server-only = %q", got)
	}

	d2 := &netinfo.DHCPLease{Server: "192.168.1.1", Expires: time.Now().Add(time.Hour)}
	got := dhcpStr(d2)
	if !strings.Contains(got, "192.168.1.1") || !strings.Contains(got, "left") {
		t.Errorf("got %q want contains server and 'left'", got)
	}
}

func TestFirstIPv6(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"nil", nil, ""},
		{"link-local only", []string{"fe80::1/64"}, ""},
		{"prefer global", []string{"fe80::1/64", "2001:db8::1/64"}, "2001:db8::1/64"},
		{"loopback ok", []string{"::1/128"}, "::1/128"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstIPv6(tc.in); got != tc.want {
				t.Errorf("firstIPv6(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrimaryGateway(t *testing.T) {
	if got := primaryGateway(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	ifaces := []netinfo.Interface{
		{Name: "wlan0"},
		{Name: "enp0", Gateway: "192.168.1.1"},
		{Name: "enp1", Gateway: "10.0.0.1"},
	}
	if got := primaryGateway(ifaces); got != "192.168.1.1" {
		t.Errorf("primaryGateway = %q, want first non-empty", got)
	}
}

func TestDbmLevel(t *testing.T) {
	cases := []struct {
		dbm  int
		want int
	}{
		{0, 0},
		{-30, 4},
		{-50, 4},
		{-55, 3},
		{-60, 3},
		{-70, 2},
		{-80, 1},
		{-90, 0},
	}
	for _, tc := range cases {
		if got := dbmLevel(tc.dbm); got != tc.want {
			t.Errorf("dbmLevel(%d) = %d, want %d", tc.dbm, got, tc.want)
		}
	}
}
