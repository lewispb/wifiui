package iwd

import (
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	agentMgrPath = dbus.ObjectPath("/net/connman/iwd")
	agentMgrIf   = "net.connman.iwd.AgentManager"
	agentPath    = dbus.ObjectPath("/com/lewispb/wifiui/Agent")
	agentIf      = "net.connman.iwd.Agent"
)

// PassphraseFunc is called when iwd needs a passphrase for a network. The
// argument is the iwd network object path; resolve to an SSID via the
// containing Station/Network if needed for prompting.
type PassphraseFunc func(networkPath dbus.ObjectPath) (string, error)

// CancelFunc is called when iwd cancels an in-flight passphrase request, e.g.
// because the previously-supplied key was wrong. The reason is iwd's free-form
// short string (typically "out-of-range", "user-canceled", "timed-out", or
// the bare D-Bus error name).
type CancelFunc func(reason string)

// AgentHandlers groups the optional callbacks an agent needs.
type AgentHandlers struct {
	Passphrase PassphraseFunc
	Cancel     CancelFunc
}

type agent struct {
	mu       sync.Mutex
	handlers AgentHandlers
}

func (a *agent) Release() *dbus.Error { return nil }

func (a *agent) Cancel(reason string) *dbus.Error {
	a.mu.Lock()
	cb := a.handlers.Cancel
	a.mu.Unlock()
	if cb != nil {
		cb(reason)
	}
	return nil
}

func (a *agent) RequestPassphrase(network dbus.ObjectPath) (string, *dbus.Error) {
	a.mu.Lock()
	fn := a.handlers.Passphrase
	a.mu.Unlock()
	if fn == nil {
		return "", dbus.NewError("net.connman.iwd.Agent.Error.Canceled", []any{"no handler"})
	}
	pw, err := fn(network)
	if err != nil {
		return "", dbus.NewError("net.connman.iwd.Agent.Error.Canceled", []any{err.Error()})
	}
	return pw, nil
}

const agentIntrospectXML = `<node>
  <interface name="net.connman.iwd.Agent">
    <method name="Release"/>
    <method name="RequestPassphrase">
      <arg type="o" name="network" direction="in"/>
      <arg type="s" name="passphrase" direction="out"/>
    </method>
    <method name="Cancel">
      <arg type="s" name="reason" direction="in"/>
    </method>
  </interface>
</node>`

// RegisterAgent installs an iwd agent. The returned func unregisters the
// agent and tears down the export.
//
// Backwards-compat: if the supplied first arg is a PassphraseFunc the cancel
// callback defaults to no-op. Use RegisterAgentWith for both callbacks.
func (c *Client) RegisterAgent(ctx context.Context, fn PassphraseFunc) (func(), error) {
	return c.RegisterAgentWith(ctx, AgentHandlers{Passphrase: fn})
}

// RegisterAgentWith installs an iwd agent with both passphrase and cancel
// handlers. The returned func unregisters the agent and tears down the
// export.
func (c *Client) RegisterAgentWith(ctx context.Context, h AgentHandlers) (func(), error) {
	a := &agent{handlers: h}
	if err := c.conn.Export(a, agentPath, agentIf); err != nil {
		return nil, fmt.Errorf("export agent: %w", err)
	}
	if err := c.conn.Export(introspect.Introspectable(agentIntrospectXML), agentPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, fmt.Errorf("export introspect: %w", err)
	}
	mgr := c.conn.Object(busName, agentMgrPath)
	if err := mgr.CallWithContext(ctx, agentMgrIf+".RegisterAgent", 0, agentPath).Store(); err != nil {
		return nil, fmt.Errorf("RegisterAgent: %w", err)
	}
	unreg := func() {
		_ = mgr.Call(agentMgrIf+".UnregisterAgent", 0, agentPath).Err
		_ = c.conn.Export(nil, agentPath, agentIf)
		_ = c.conn.Export(nil, agentPath, "org.freedesktop.DBus.Introspectable")
	}
	return unreg, nil
}
