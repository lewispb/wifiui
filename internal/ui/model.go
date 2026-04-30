package ui

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/lewispb/wifiui/internal/iwd"
	"github.com/lewispb/wifiui/internal/netinfo"
	"github.com/lewispb/wifiui/internal/portal"
)

// viewState is the immutable per-frame snapshot of everything the UI renders.
// Workers build a fresh viewState and Store it; readers Load and treat the
// value as read-only. Slices and maps must not be mutated after publication.
type viewState struct {
	station        string
	vendor         string
	state          string
	connected      string
	signal         int
	band           string
	scanning       bool
	networks       []*iwd.Network
	interfaces     []netinfo.Interface
	pinned         map[string]struct{}
	err            string
	lastTick       time.Time
	portalStatus   portal.Status
	portalRedirect string
}

// model is a single-writer-at-a-time atomic state cell. Concurrent mutators
// hold writeMu briefly while building the next state; the published pointer
// is treated as immutable so readers don't take any lock.
type model struct {
	cur     atomic.Pointer[viewState]
	writeMu sync.Mutex
}

func newModel(initial viewState) *model {
	m := &model{}
	m.cur.Store(&initial)
	return m
}

// snapshot returns the current state. The returned pointer must not be
// mutated by the caller — copy fields instead.
func (m *model) snapshot() *viewState { return m.cur.Load() }

// mutate copies the current state, applies fn to the copy, then publishes the
// result. fn runs under writeMu so concurrent mutations don't lose updates.
func (m *model) mutate(fn func(*viewState)) {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	cur := m.cur.Load()
	next := *cur
	fn(&next)
	m.cur.Store(&next)
}
