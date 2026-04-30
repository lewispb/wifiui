// Package netinfo gathers a snapshot of currently-active network interfaces:
// addresses, default gateway, DNS, DHCP lease, and (for wifi) live link info
// like rate and generation. It pulls from netlink, systemd-networkd lease
// files, resolvectl, and `iw dev <iface> link`.
package netinfo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type Type string

const (
	TypeWifi  Type = "wifi"
	TypeWired Type = "wired"
	TypeTun   Type = "vpn"
	TypeOther Type = "other"
)

type Interface struct {
	Name    string
	Type    Type
	State   string // up / down
	MAC     string
	Speed   string // e.g. "1 Gbps" (wired) or "130 MBit/s" (wifi tx rate)
	SSID    string // wifi only
	Band    string // wifi only — "2.4 GHz", "5 GHz", "6 GHz"
	Signal  int    // wifi only — dBm
	Gen     string // wifi only — "WiFi 4/5/6/7"
	IPv4    []string
	IPv6    []string
	Gateway string
	DNS     []string
	DHCP    *DHCPLease
}

type DHCPLease struct {
	Server  string
	Expires time.Time
}

// List returns the snapshot of active interfaces. Only interfaces that
// currently have at least one IP address are returned; loopback, docker
// bridges and veth pairs are skipped.
func List(ctx context.Context) ([]Interface, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("LinkList: %w", err)
	}
	var out []Interface
	for _, link := range links {
		attrs := link.Attrs()
		if shouldSkip(attrs.Name) {
			continue
		}
		addrs4, _ := netlink.AddrList(link, unix.AF_INET)
		addrs6, _ := netlink.AddrList(link, unix.AF_INET6)
		if len(addrs4) == 0 && len(addrs6) == 0 {
			continue
		}
		iface := Interface{
			Name:  attrs.Name,
			Type:  detectType(attrs.Name, link),
			State: stateOf(attrs.OperState),
			MAC:   attrs.HardwareAddr.String(),
			IPv4:  cidrsOf(addrs4, false),
			IPv6:  cidrsOf(addrs6, false),
		}
		iface.Gateway = defaultGateway(link)
		iface.DNS = dnsForLink(ctx, attrs.Name)
		if lease, ok := loadDHCPLease(attrs.Index); ok {
			iface.DHCP = &lease
		}
		switch iface.Type {
		case TypeWifi:
			fillWifiLink(ctx, attrs.Name, &iface)
		case TypeWired:
			iface.Speed = ethSpeed(attrs.Name)
		}
		out = append(out, iface)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return typeRank(out[i].Type) < typeRank(out[j].Type)
	})
	return out, nil
}

func typeRank(t Type) int {
	switch t {
	case TypeWifi:
		return 0
	case TypeWired:
		return 1
	case TypeTun:
		return 2
	default:
		return 3
	}
}

func shouldSkip(name string) bool {
	if name == "lo" {
		return true
	}
	switch {
	case strings.HasPrefix(name, "docker"),
		strings.HasPrefix(name, "br-"),
		strings.HasPrefix(name, "veth"):
		return true
	}
	return false
}

func detectType(name string, link netlink.Link) Type {
	typ := link.Type()
	if typ == "tun" || typ == "tap" || typ == "wireguard" ||
		strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "tailscale") || strings.HasPrefix(name, "wg") {
		return TypeTun
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", name, "phy80211")); err == nil {
		return TypeWifi
	}
	return TypeWired
}

func stateOf(s netlink.LinkOperState) string {
	if s == netlink.OperUp || s == netlink.OperUnknown {
		return "up"
	}
	return "down"
}

func cidrsOf(addrs []netlink.Addr, includeLinkLocal bool) []string {
	var out []string
	for _, a := range addrs {
		if !includeLinkLocal && a.IP.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, a.IPNet.String())
	}
	return out
}

func defaultGateway(link netlink.Link) string {
	routes, err := netlink.RouteList(link, unix.AF_INET)
	if err != nil {
		return ""
	}
	for _, r := range routes {
		if r.Dst == nil && r.Gw != nil {
			return r.Gw.String()
		}
	}
	return ""
}

var dnsServerRe = regexp.MustCompile(`(?m)DNS Servers:\s+([^\n]+)`)

func dnsForLink(ctx context.Context, iface string) []string {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "resolvectl", "--no-pager", "status", iface).Output()
	if err != nil {
		return nil
	}
	m := dnsServerRe.FindSubmatch(out)
	if m == nil {
		return nil
	}
	return strings.Fields(string(m[1]))
}

func ethSpeed(name string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "speed"))
	if err != nil {
		return ""
	}
	return parseEthSpeed(string(data))
}

func fillWifiLink(ctx context.Context, iface string, info *Interface) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "iw", "dev", iface, "link").Output()
	if err != nil {
		return
	}
	parseWifiLink(string(out), info)
}
