package iwd

import "testing"

func TestNetworkBand(t *testing.T) {
	cases := []struct {
		freq uint32
		want string
	}{
		{0, ""},
		{2412, "2.4 GHz"},
		{2484, "2.4 GHz"},
		{5180, "5 GHz"},
		{5825, "5 GHz"},
		{5955, "6 GHz"},
		{7115, "6 GHz"},
	}
	for _, tc := range cases {
		n := &Network{Frequency: tc.freq}
		if got := n.Band(); got != tc.want {
			t.Errorf("Band(freq=%d) = %q, want %q", tc.freq, got, tc.want)
		}
	}
}
