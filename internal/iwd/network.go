package iwd

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/lewispb/wifiui/internal/wifi"
)

type Network struct {
	client    *Client
	path      dbus.ObjectPath
	SSID      string
	Type      string // psk, open, 8021x, wep
	Signal    int    // dBm (negative)
	Frequency uint32 // MHz of the strongest BSS, 0 if unknown
	Connected bool
	Known     bool
}

// Band returns a human-readable band label for the network's frequency.
func (n *Network) Band() string { return wifi.BandFromMHz(n.Frequency) }

// Connect requests a connection to the network. Unknown networks requiring a
// passphrase rely on an agent registered via Client.RegisterAgent.
func (n *Network) Connect(ctx context.Context) error {
	obj := n.client.conn.Object(busName, n.path)
	if err := obj.CallWithContext(ctx, networkIf+".Connect", 0).Store(); err != nil {
		return fmt.Errorf("Connect %s: %w", n.SSID, err)
	}
	return nil
}
