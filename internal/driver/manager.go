package driver

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Manager holds the active ClusterDriver and allows hot-switching between
// real Kubernetes contexts, SSH-tunneled clusters, uploaded kubeconfigs, and Demo Mode.
type Manager struct {
	mu           sync.RWMutex
	active       ClusterDriver
	activeID     string
	kubeconfig   string
	extraCtx     []ClusterContext
	drivers      map[string]ClusterDriver
	listContexts func(activeID string) []ClusterContext
}

// NewManager constructs a cluster connection manager.
func NewManager(initial ClusterDriver, initialID, kubeconfigPath string, contextLoader func(activeID string) []ClusterContext) *Manager {
	m := &Manager{
		active:       initial,
		activeID:     initialID,
		kubeconfig:   kubeconfigPath,
		drivers:      make(map[string]ClusterDriver),
		listContexts: contextLoader,
	}
	if initial != nil && initialID != "" {
		m.drivers[initialID] = initial
	}
	return m
}

// Register stores a named driver instance so users can switch to it seamlessly.
func (m *Manager) Register(ctxInfo ClusterContext, drv ClusterDriver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drivers[ctxInfo.ID] = drv
	m.upsertExtraCtxLocked(ctxInfo)
}

func (m *Manager) upsertExtraCtxLocked(ctxInfo ClusterContext) {
	for i, existing := range m.extraCtx {
		if existing.ID == ctxInfo.ID {
			m.extraCtx[i] = ctxInfo
			return
		}
	}
	m.extraCtx = append(m.extraCtx, ctxInfo)
}

// Active returns the currently selected ClusterDriver.
func (m *Manager) Active() ClusterDriver {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// Status returns ClusterStatus merged with all selectable cluster contexts.
func (m *Manager) Status(ctx context.Context) ClusterStatus {
	m.mu.RLock()
	drv := m.active
	activeID := m.activeID
	loader := m.listContexts
	extra := append([]ClusterContext(nil), m.extraCtx...)
	m.mu.RUnlock()

	st := drv.Info(ctx)

	var contexts []ClusterContext
	if loader != nil {
		contexts = append(contexts, loader(activeID)...)
	}
	for _, c := range extra {
		c.Active = c.ID == activeID
		found := false
		for i, existing := range contexts {
			if existing.ID == c.ID {
				contexts[i].Active = c.Active
				found = true
				break
			}
		}
		if !found {
			contexts = append(contexts, c)
		}
	}
	st.Contexts = contexts
	return st
}

// Switch activates a registered driver or dynamically initializes one via factory.
func (m *Manager) Switch(id string, factory func(id string) (ClusterDriver, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if drv, ok := m.drivers[id]; ok {
		m.active = drv
		m.activeID = id
		return nil
	}
	if factory == nil {
		return fmt.Errorf("cluster context %q not found", id)
	}
	drv, err := factory(id)
	if err != nil {
		return err
	}
	m.drivers[id] = drv
	m.active = drv
	m.activeID = id
	return nil
}

// ActivateCustom adds and immediately switches to an uploaded / SSH / token cluster driver.
func (m *Manager) ActivateCustom(ctxInfo ClusterContext, drv ClusterDriver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.drivers[ctxInfo.ID]; ok && old != drv {
		if c, ok := old.(io.Closer); ok {
			_ = c.Close()
		}
	}
	m.drivers[ctxInfo.ID] = drv
	m.upsertExtraCtxLocked(ctxInfo)
	m.active = drv
	m.activeID = ctxInfo.ID
}

// Disconnect closes and removes a dynamically registered cluster (not demo).
func (m *Manager) Disconnect(id string) error {
	if id == "" || id == "demo" {
		return fmt.Errorf("cannot remove built-in demo cluster")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	drv, ok := m.drivers[id]
	if ok {
		if c, isCloser := drv.(io.Closer); isCloser {
			_ = c.Close()
		}
		delete(m.drivers, id)
	}

	kept := make([]ClusterContext, 0, len(m.extraCtx))
	for _, c := range m.extraCtx {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	m.extraCtx = kept

	if m.activeID == id {
		if demoDrv, hasDemo := m.drivers["demo"]; hasDemo {
			m.active = demoDrv
			m.activeID = "demo"
		}
	}
	return nil
}

// CloseAll closes any drivers holding network tunnels or resources on shutdown.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, drv := range m.drivers {
		if c, ok := drv.(io.Closer); ok {
			_ = c.Close()
		}
	}
}
