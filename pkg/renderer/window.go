// pkg/renderer/window.go
// Package renderer — window state.
// Tracks the renderer window position, size, and visibility state.
// The supervisor does not directly control the renderer window (the engine
// owns it), but it stores the last known state for observability.
package renderer

import "sync"

// WindowState holds the last known geometry of the renderer window.
type WindowState struct {
	mu      sync.RWMutex
	x, y    int
	w, h    int
	visible bool
}

// Update stores the latest window geometry reported by the renderer.
func (ws *WindowState) Update(x, y, w, h int, visible bool) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.x, ws.y, ws.w, ws.h = x, y, w, h
	ws.visible = visible
}

// Geometry returns the last known position and size.
func (ws *WindowState) Geometry() (x, y, w, h int) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.x, ws.y, ws.w, ws.h
}

// Visible returns whether the renderer window is currently shown.
func (ws *WindowState) Visible() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.visible
}
