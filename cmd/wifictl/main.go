package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/lewispb/wifiui/internal/iwd"
	"golang.org/x/term"
)

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cmd, rest := args[0], args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := iwd.New()
	if err != nil {
		die("connect iwd: %v", err)
	}
	defer c.Close()

	stations, err := c.Stations(ctx)
	if err != nil {
		die("list stations: %v", err)
	}
	if len(stations) == 0 {
		die("no wifi stations found (is iwd running?)")
	}
	st := stations[0]

	switch cmd {
	case "stations":
		for _, s := range stations {
			fmt.Println(s.Name)
		}
	case "status":
		state, err := st.State(ctx)
		if err != nil {
			die("state: %v", err)
		}
		fmt.Printf("station: %s\nstate:   %s\n", st.Name, state)
	case "scan":
		fmt.Fprintln(os.Stderr, "scanning...")
		if err := st.Scan(ctx); err != nil {
			die("scan: %v", err)
		}
		_ = st.WaitScan(ctx, 8*time.Second)
		nets, err := st.Networks(ctx)
		if err != nil {
			die("list networks: %v", err)
		}
		printNetworks(nets)
	case "list":
		nets, err := st.Networks(ctx)
		if err != nil {
			die("list: %v", err)
		}
		printNetworks(nets)
	case "connect":
		if len(rest) == 0 {
			die("usage: wifictl connect <ssid>")
		}
		ssid := strings.Join(rest, " ")
		nets, err := st.Networks(ctx)
		if err != nil {
			die("list: %v", err)
		}
		var target *iwd.Network
		for _, n := range nets {
			if n.SSID == ssid {
				target = n
				break
			}
		}
		if target == nil {
			die("network %q not visible (try: wifictl scan)", ssid)
		}
		unreg, err := c.RegisterAgent(ctx, func(_ dbus.ObjectPath) (string, error) {
			return readPassphrase(ssid)
		})
		if err != nil {
			die("register agent: %v", err)
		}
		defer unreg()
		fmt.Fprintf(os.Stderr, "connecting to %q...\n", ssid)
		if err := target.Connect(ctx); err != nil {
			die("connect: %v", err)
		}
		fmt.Println("ok")
	case "disconnect":
		if err := st.Disconnect(ctx); err != nil {
			die("disconnect: %v", err)
		}
		fmt.Println("ok")
	default:
		usage()
		os.Exit(2)
	}
}

func printNetworks(nets []*iwd.Network) {
	for _, n := range nets {
		flags := []string{n.Type}
		if n.Known {
			flags = append(flags, "known")
		}
		if n.Connected {
			flags = append(flags, "connected")
		}
		band := n.Band()
		if band == "" {
			band = "?"
		}
		fmt.Printf("%4d dBm  %-7s  %-32s  %s\n", n.Signal, band, n.SSID, strings.Join(flags, ","))
	}
}

func readPassphrase(ssid string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("stdin is not a terminal — passphrase prompts require a tty")
	}
	fmt.Fprintf(os.Stderr, "passphrase for %q: ", ssid)
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "wifictl: "+format+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  wifictl stations            list wifi station interfaces
  wifictl status              current connection state
  wifictl scan                trigger scan and list networks
  wifictl list                list networks (no scan)
  wifictl connect <ssid>      connect to a known network
  wifictl disconnect          disconnect current network`)
}
