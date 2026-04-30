package wifi

import "testing"

func TestBandFromMHz(t *testing.T) {
	cases := []struct {
		mhz  uint32
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
		if got := BandFromMHz(tc.mhz); got != tc.want {
			t.Errorf("BandFromMHz(%d) = %q, want %q", tc.mhz, got, tc.want)
		}
	}
}
