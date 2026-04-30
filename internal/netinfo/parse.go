package netinfo

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lewispb/wifiui/internal/wifi"
)

var (
	reSSID   = regexp.MustCompile(`(?m)^\s*SSID:\s+(.+)$`)
	reFreq   = regexp.MustCompile(`(?m)^\s*freq:\s+([0-9.]+)`)
	reSignal = regexp.MustCompile(`(?m)^\s*signal:\s+(-?\d+)\s+dBm`)
	reTxRate = regexp.MustCompile(`(?m)^\s*tx bitrate:\s+([0-9.]+)\s+(\w+/s)\s*(\S+)?`)
)

// parseWifiLink fills the wifi-only fields of info from `iw dev <iface> link`
// output.
func parseWifiLink(out string, info *Interface) {
	if m := reSSID.FindStringSubmatch(out); m != nil {
		info.SSID = strings.TrimSpace(m[1])
	}
	if m := reFreq.FindStringSubmatch(out); m != nil {
		if mhz, err := strconv.ParseFloat(m[1], 64); err == nil {
			info.Band = wifi.BandFromMHz(uint32(mhz))
		}
	}
	if m := reSignal.FindStringSubmatch(out); m != nil {
		info.Signal, _ = strconv.Atoi(m[1])
	}
	if m := reTxRate.FindStringSubmatch(out); m != nil {
		info.Speed = m[1] + " " + m[2]
		if len(m) >= 4 {
			switch {
			case strings.HasPrefix(m[3], "EHT"):
				info.Gen = "WiFi 7"
			case strings.HasPrefix(m[3], "HE"):
				info.Gen = "WiFi 6"
			case strings.HasPrefix(m[3], "VHT"):
				info.Gen = "WiFi 5"
			case strings.HasPrefix(m[3], "MCS"):
				info.Gen = "WiFi 4"
			}
		}
	}
}

// parseDHCPLease parses the contents of a systemd-networkd lease file. mtime
// is the lease file's modification time; lease lifetime is added to it to
// produce an absolute expiry. ok is false if neither a server nor a lifetime
// was found.
func parseDHCPLease(data []byte, mtime time.Time) (DHCPLease, bool) {
	var lease DHCPLease
	var lifetime int64
	for _, line := range strings.Split(string(data), "\n") {
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		k, v := line[:eq], strings.TrimSpace(line[eq+1:])
		switch k {
		case "SERVER_ADDRESS":
			lease.Server = v
		case "LIFETIME":
			lifetime, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	if !mtime.IsZero() && lifetime > 0 {
		lease.Expires = mtime.Add(time.Duration(lifetime) * time.Second)
	}
	if lease.Server == "" && lease.Expires.IsZero() {
		return DHCPLease{}, false
	}
	return lease, true
}

// loadDHCPLease reads /run/systemd/netif/leases/<idx> and parses it. Returns
// (zero, false) when the file is missing or unreadable.
func loadDHCPLease(idx int) (DHCPLease, bool) {
	path := fmt.Sprintf("/run/systemd/netif/leases/%d", idx)
	data, err := os.ReadFile(path)
	if err != nil {
		return DHCPLease{}, false
	}
	var mtime time.Time
	if info, err := os.Stat(path); err == nil {
		mtime = info.ModTime()
	}
	return parseDHCPLease(data, mtime)
}

// parseEthSpeed turns the contents of /sys/class/net/<iface>/speed into a
// human-readable rate. Returns "" when the value is "-1" (link down) or empty.
func parseEthSpeed(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-1" {
		return ""
	}
	mb, err := strconv.Atoi(s)
	if err != nil {
		return s + " Mbps"
	}
	if mb >= 1000 && mb%1000 == 0 {
		return fmt.Sprintf("%d Gbps", mb/1000)
	}
	return fmt.Sprintf("%d Mbps", mb)
}
