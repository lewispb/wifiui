package ui

import "time"

// Tunable constants for the UI lifecycle. Centralised here so the values are
// discoverable and easy to adjust during development.
const (
	refreshInterval     = 2 * time.Second
	rescanEvery         = 12 * time.Second
	scanWaitDeadline    = 8 * time.Second
	scanCallTimeout     = 10 * time.Second
	connectTimeout      = 90 * time.Second
	refreshCallTimeout  = 10 * time.Second
	disconnectTimeout   = 10 * time.Second
	onepwLookupTimeout  = 5 * time.Second
	onepwSaveTimeout    = 15 * time.Second
	passphraseTimeout   = 3 * time.Minute
	toolDeadline        = 30 * time.Second
	themePollInterval   = 2 * time.Second
)

// view identifies the active top-level pane.
type view int

const (
	viewNetworks view = iota
	viewTools
)
