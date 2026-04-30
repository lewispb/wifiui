// Package wifi holds tiny shared helpers (band classification, etc.) that are
// reused across the iwd D-Bus client and the netinfo OS-level snapshot.
package wifi

// BandFromMHz maps a centre frequency in MHz to a human-readable Wi-Fi band
// label. Returns "" for 0 (unknown).
func BandFromMHz(mhz uint32) string {
	switch {
	case mhz == 0:
		return ""
	case mhz < 3000:
		return "2.4 GHz"
	case mhz < 5925:
		return "5 GHz"
	default:
		return "6 GHz"
	}
}
