package netinfo

import (
	"strings"
	"testing"
)

func TestParseWifiLinkWiFi4(t *testing.T) {
	out := `Connected to 20:36:26:d5:31:60 (on wlan0)
	SSID: example-2g
	freq: 2412.0
	signal: -48 dBm
	rx bitrate: 86.7 MBit/s MCS 12 short GI
	tx bitrate: 130.0 MBit/s MCS 15
`
	var info Interface
	parseWifiLink(out, &info)
	if info.SSID != "example-2g" {
		t.Errorf("SSID = %q", info.SSID)
	}
	if info.Band != "2.4 GHz" {
		t.Errorf("Band = %q", info.Band)
	}
	if info.Signal != -48 {
		t.Errorf("Signal = %d", info.Signal)
	}
	if info.Speed != "130.0 MBit/s" {
		t.Errorf("Speed = %q", info.Speed)
	}
	if info.Gen != "WiFi 4" {
		t.Errorf("Gen = %q", info.Gen)
	}
}

func TestParseWifiLinkGenerations(t *testing.T) {
	cases := []struct {
		name string
		body string
		gen  string
	}{
		{"WiFi 4 (HT)", "tx bitrate: 130.0 MBit/s MCS 15\n", "WiFi 4"},
		{"WiFi 5 (VHT)", "tx bitrate: 866.7 MBit/s VHT-MCS 7 80MHz short GI VHT-NSS 2\n", "WiFi 5"},
		{"WiFi 6 (HE)", "tx bitrate: 1200.0 MBit/s HE-MCS 9 80MHz\n", "WiFi 6"},
		{"WiFi 7 (EHT)", "tx bitrate: 2400.0 MBit/s EHT-MCS 13\n", "WiFi 7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var info Interface
			parseWifiLink(tc.body, &info)
			if info.Gen != tc.gen {
				t.Errorf("Gen = %q, want %q", info.Gen, tc.gen)
			}
		})
	}
}

func TestParseWifiLinkBands(t *testing.T) {
	cases := []struct {
		freq string
		band string
	}{
		{"2412.0", "2.4 GHz"},
		{"2484", "2.4 GHz"},
		{"5180", "5 GHz"},
		{"5825", "5 GHz"},
		{"5955", "6 GHz"},
		{"7115", "6 GHz"},
	}
	for _, tc := range cases {
		t.Run(tc.freq, func(t *testing.T) {
			var info Interface
			parseWifiLink("freq: "+tc.freq+"\n", &info)
			if info.Band != tc.band {
				t.Errorf("freq=%s Band=%q, want %q", tc.freq, info.Band, tc.band)
			}
		})
	}
}

func TestParseWifiLinkNotConnected(t *testing.T) {
	var info Interface
	parseWifiLink("Not connected.\n", &info)
	if info.SSID != "" || info.Band != "" || info.Signal != 0 || info.Speed != "" || info.Gen != "" {
		t.Errorf("expected zero values, got %+v", info)
	}
}

func TestDNSServerRegex(t *testing.T) {
	sample := `Link 33 (wlan0)
    Current Scopes: DNS
         Protocols: ...
Current DNS Server: 192.168.1.1
       DNS Servers: 192.168.1.1 8.8.8.8
        DNS Domain: lan
`
	m := dnsServerRe.FindStringSubmatch(sample)
	if m == nil {
		t.Fatal("no match")
	}
	fields := strings.Fields(m[1])
	if len(fields) != 2 || fields[0] != "192.168.1.1" || fields[1] != "8.8.8.8" {
		t.Errorf("fields = %v", fields)
	}
}

func TestShouldSkip(t *testing.T) {
	cases := map[string]bool{
		"lo":              true,
		"docker0":         true,
		"docker_gwbridge": true,
		"br-abc123":       true,
		"vethfoo":         true,
		"wlan0":           false,
		"enp10s0":         false,
		"tailscale0":      false,
		"tun0":            false,
	}
	for name, want := range cases {
		if got := shouldSkip(name); got != want {
			t.Errorf("shouldSkip(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTypeRank(t *testing.T) {
	if typeRank(TypeWifi) >= typeRank(TypeWired) {
		t.Errorf("wifi should rank before wired")
	}
	if typeRank(TypeWired) >= typeRank(TypeTun) {
		t.Errorf("wired should rank before vpn")
	}
	if typeRank(TypeTun) >= typeRank(TypeOther) {
		t.Errorf("vpn should rank before other")
	}
}
