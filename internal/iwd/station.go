package iwd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

type Station struct {
	client *Client
	path   dbus.ObjectPath
	Name   string // device name (e.g. wlan0)
}

// Scan triggers a fresh scan. The call returns once iwd has accepted the
// request; results land asynchronously on the next Networks() call after
// iwd's cache updates.
func (s *Station) Scan(ctx context.Context) error {
	obj := s.client.conn.Object(busName, s.path)
	if err := obj.CallWithContext(ctx, stationIf+".Scan", 0).Store(); err != nil {
		var dErr *dbus.Error
		if errors.As(err, &dErr) && dErr.Name == "net.connman.iwd.InProgress" {
			return nil
		}
		return fmt.Errorf("Scan: %w", err)
	}
	return nil
}

// State returns the station state: connected, disconnected, connecting,
// disconnecting, or roaming.
func (s *Station) State(ctx context.Context) (string, error) {
	v, err := s.prop(ctx, "State")
	if err != nil {
		return "", err
	}
	state, _ := v.Value().(string)
	return state, nil
}

// Scanning reports whether iwd is currently scanning.
func (s *Station) Scanning(ctx context.Context) (bool, error) {
	v, err := s.prop(ctx, "Scanning")
	if err != nil {
		return false, err
	}
	on, _ := v.Value().(bool)
	return on, nil
}

func (s *Station) prop(ctx context.Context, name string) (dbus.Variant, error) {
	obj := s.client.conn.Object(busName, s.path)
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, stationIf, name).Store(&v); err != nil {
		return v, fmt.Errorf("Get %s: %w", name, err)
	}
	return v, nil
}

// Networks lists currently-known networks visible to this station, ordered by
// signal strength (strongest first).
func (s *Station) Networks(ctx context.Context) ([]*Network, error) {
	obj := s.client.conn.Object(busName, s.path)
	type entry struct {
		Path   dbus.ObjectPath
		Signal int16
	}
	var raw []entry
	if err := obj.CallWithContext(ctx, stationIf+".GetOrderedNetworks", 0).Store(&raw); err != nil {
		return nil, fmt.Errorf("GetOrderedNetworks: %w", err)
	}
	managed, err := s.client.managed(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Network, 0, len(raw))
	for _, e := range raw {
		ifaces, ok := managed[e.Path]
		if !ok {
			continue
		}
		props := ifaces[networkIf]
		n := &Network{
			client: s.client,
			path:   e.Path,
			Signal: int(e.Signal) / 100,
		}
		if v, ok := props["Name"]; ok {
			n.SSID, _ = v.Value().(string)
		}
		if v, ok := props["Type"]; ok {
			n.Type, _ = v.Value().(string)
		}
		if v, ok := props["Connected"]; ok {
			n.Connected, _ = v.Value().(bool)
		}
		if v, ok := props["KnownNetwork"]; ok {
			if p, ok := v.Value().(dbus.ObjectPath); ok && p != "" && p != "/" {
				n.Known = true
			}
		}
		if v, ok := props["ExtendedServiceSet"]; ok {
			if bssPaths, ok := v.Value().([]dbus.ObjectPath); ok && len(bssPaths) > 0 {
				if freq, err := s.client.bssFrequency(ctx, bssPaths[0]); err == nil {
					n.Frequency = freq
				}
			}
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Station) Disconnect(ctx context.Context) error {
	obj := s.client.conn.Object(busName, s.path)
	if err := obj.CallWithContext(ctx, stationIf+".Disconnect", 0).Store(); err != nil {
		return fmt.Errorf("Disconnect: %w", err)
	}
	return nil
}

// AdapterInfo returns the vendor (and model, when iwd publishes one) of the
// underlying wireless adapter for this station.
func (s *Station) AdapterInfo(ctx context.Context) (vendor, model string, _ error) {
	dev := s.client.conn.Object(busName, s.path)
	var ap dbus.Variant
	if err := dev.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, deviceIf, "Adapter").Store(&ap); err != nil {
		return "", "", fmt.Errorf("Get Adapter: %w", err)
	}
	adapterPath, ok := ap.Value().(dbus.ObjectPath)
	if !ok {
		return "", "", fmt.Errorf("device has no Adapter property")
	}
	ad := s.client.conn.Object(busName, adapterPath)
	var vv dbus.Variant
	if err := ad.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, adapterIf, "Vendor").Store(&vv); err == nil {
		vendor, _ = vv.Value().(string)
	}
	var mv dbus.Variant
	if err := ad.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, adapterIf, "Model").Store(&mv); err == nil {
		model, _ = mv.Value().(string)
	}
	return vendor, model, nil
}

// WaitScan blocks until the station reports it is no longer scanning, the
// deadline elapses, or ctx is cancelled. Returns ctx.Err() on cancel.
func (s *Station) WaitScan(ctx context.Context, deadline time.Duration) error {
	cctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	for {
		on, err := s.Scanning(cctx)
		if err != nil || !on {
			return err
		}
		select {
		case <-cctx.Done():
			return cctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
