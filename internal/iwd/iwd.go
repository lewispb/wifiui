package iwd

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	busName    = "net.connman.iwd"
	rootPath   = dbus.ObjectPath("/")
	stationIf  = "net.connman.iwd.Station"
	deviceIf   = "net.connman.iwd.Device"
	adapterIf  = "net.connman.iwd.Adapter"
	networkIf  = "net.connman.iwd.Network"
	bssIf      = "net.connman.iwd.BasicServiceSet"
	propsGetIf = "org.freedesktop.DBus.Properties"
)

type managedObjects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

type Client struct {
	conn *dbus.Conn
}

func New() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect system bus: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Stations(ctx context.Context) ([]*Station, error) {
	managed, err := c.managed(ctx)
	if err != nil {
		return nil, err
	}
	var out []*Station
	for path, ifaces := range managed {
		if _, ok := ifaces[stationIf]; !ok {
			continue
		}
		s := &Station{client: c, path: path}
		if dev, ok := ifaces[deviceIf]; ok {
			if v, ok := dev["Name"]; ok {
				s.Name, _ = v.Value().(string)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func (c *Client) bssFrequency(ctx context.Context, path dbus.ObjectPath) (uint32, error) {
	obj := c.conn.Object(busName, path)
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, bssIf, "Frequency").Store(&v); err != nil {
		return 0, err
	}
	freq, _ := v.Value().(uint32)
	return freq, nil
}

// NetworkSSID resolves an iwd network object path to its SSID. Used by the
// passphrase agent to label the prompt shown to the user.
func (c *Client) NetworkSSID(ctx context.Context, path dbus.ObjectPath) (string, error) {
	obj := c.conn.Object(busName, path)
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, networkIf, "Name").Store(&v); err != nil {
		return "", err
	}
	s, _ := v.Value().(string)
	return s, nil
}

func (c *Client) managed(ctx context.Context) (managedObjects, error) {
	obj := c.conn.Object(busName, rootPath)
	var m managedObjects
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&m); err != nil {
		return nil, fmt.Errorf("GetManagedObjects: %w", err)
	}
	return m, nil
}
