package netinfo

import (
	"testing"
	"time"
)

func TestParseDHCPLease(t *testing.T) {
	mtime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		data     string
		mtime    time.Time
		ok       bool
		server   string
		lifetime time.Duration
	}{
		{
			name:     "complete",
			data:     "SERVER_ADDRESS=192.168.1.1\nLIFETIME=3600\n",
			mtime:    mtime,
			ok:       true,
			server:   "192.168.1.1",
			lifetime: time.Hour,
		},
		{
			name:   "server only",
			data:   "SERVER_ADDRESS=10.0.0.1\n",
			mtime:  mtime,
			ok:     true,
			server: "10.0.0.1",
		},
		{
			name: "empty",
			data: "",
			ok:   false,
		},
		{
			name: "junk",
			data: "# comment\nNOT_A_KEY\n",
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lease, ok := parseDHCPLease([]byte(tc.data), tc.mtime)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if lease.Server != tc.server {
				t.Errorf("Server = %q, want %q", lease.Server, tc.server)
			}
			if tc.lifetime > 0 {
				want := tc.mtime.Add(tc.lifetime)
				if !lease.Expires.Equal(want) {
					t.Errorf("Expires = %v, want %v", lease.Expires, want)
				}
			}
		})
	}
}

func TestParseEthSpeed(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"-1":        "",
		"  -1\n":    "",
		"100":       "100 Mbps",
		"1000":      "1 Gbps",
		"2500":      "2500 Mbps",
		"10000":     "10 Gbps",
		"40000\n":   "40 Gbps",
		"weird":     "weird Mbps",
	}
	for in, want := range cases {
		if got := parseEthSpeed(in); got != want {
			t.Errorf("parseEthSpeed(%q) = %q, want %q", in, got, want)
		}
	}
}
