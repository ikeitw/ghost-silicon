//go:build windows

// Package browser — tab strip (Walk fallback).
// TabStrip is a Walk CustomWidget that draws tabs using Walk's Canvas API.
// It is superseded by the HTML chrome overlay in webview.go; kept here as a
// fallback in case the HTML approach is ever dropped.
package browser

import (
	"fmt"
	"sync"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// Tab holds all state for a single browser tab.
type Tab struct {
	ID      string
	URL     string
	Title   string
	Loading bool
	History *NavigationHistory
}

// displayTitle falls back to a shortened URL if the page title is blank.
func (t *Tab) displayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	if t.URL == "" {
		return "New Tab"
	}
	if len(t.URL) > 40 {
		return t.URL[:37] + "…"
	}
	return t.URL
}

// ── TabStrip ──────────────────────────────────────────────────────────────────

// TabStrip is a horizontal tab bar drawn using a Walk CustomWidget.
// It fires OnSelect when the user clicks a tab, and OnClose when the user
// clicks the × button inside a tab.
type TabStrip struct {
	widget *walk.CustomWidget
	mu     sync.RWMutex
	tabs   []*Tab
	active int // index of the active tab (-1 if none)
	seqID  int

	// Callbacks — set by the Browser before the window opens.
	OnSelect func(tabID string)
	OnClose  func(tabID string)
	OnNew    func()
}

func NewTabStrip(parent walk.Container) (*TabStrip, error) {
	ts := &TabStrip{active: -1}

	cw, err := walk.NewCustomWidget(parent, 0, ts.paint)
	if err != nil {
		return nil, fmt.Errorf("tab strip widget: %w", err)
	}
	cw.SetMinMaxSize(
		walk.Size{Width: 0, Height: TabBarHeight},
		walk.Size{Width: 0, Height: TabBarHeight},
	)
	cw.MouseDown().Attach(ts.onMouseDown)
	ts.widget = cw

	ts.addTabLocked("", "New Tab")
	return ts, nil
}

// Handle returns the Win32 HWND of the tab strip widget.
// Used by Window to manually position the strip via SetWindowPos.
func (ts *TabStrip) Handle() win.HWND {
	return win.HWND(uintptr(ts.widget.Handle()))
}
func (ts *TabStrip) AddTab(url, title string) string {
	ts.mu.Lock()
	id := ts.addTabLocked(url, title)
	ts.mu.Unlock()
	ts.widget.Invalidate()
	return id
}

// CloseTab removes the tab with the given ID.
// If it was the active tab, the previous tab (or the next if no previous)
// becomes active.
func (ts *TabStrip) CloseTab(id string) {
	ts.mu.Lock()
	idx := ts.indexByID(id)
	if idx < 0 || len(ts.tabs) == 1 {
		ts.mu.Unlock()
		return
	}
	ts.tabs = append(ts.tabs[:idx], ts.tabs[idx+1:]...)
	if ts.active >= len(ts.tabs) {
		ts.active = len(ts.tabs) - 1
	}
	newActiveID := ""
	if ts.active >= 0 {
		newActiveID = ts.tabs[ts.active].ID
	}
	ts.mu.Unlock()
	ts.widget.Invalidate()
	if newActiveID != "" && ts.OnSelect != nil {
		ts.OnSelect(newActiveID)
	}
}

func (ts *TabStrip) SelectTab(id string) {
	ts.mu.Lock()
	idx := ts.indexByID(id)
	if idx >= 0 {
		ts.active = idx
	}
	ts.mu.Unlock()
	ts.widget.Invalidate()
}

// ActiveTab returns the currently active Tab, or nil if the strip is empty.
// Returns a copy — safe to read without holding the lock.
func (ts *TabStrip) ActiveTab() *Tab {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.active < 0 || ts.active >= len(ts.tabs) {
		return nil
	}
	cp := *ts.tabs[ts.active]
	return &cp
}

func (ts *TabStrip) UpdateTab(id, url, title string, loading bool) {
	ts.mu.Lock()
	if idx := ts.indexByID(id); idx >= 0 {
		ts.tabs[idx].URL = url
		ts.tabs[idx].Title = title
		ts.tabs[idx].Loading = loading
	}
	ts.mu.Unlock()
	ts.widget.Invalidate()
}

// Tabs returns a snapshot of all tabs (copies, safe to read without locking).
func (ts *TabStrip) Tabs() []*Tab {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]*Tab, len(ts.tabs))
	for i, t := range ts.tabs {
		cp := *t
		out[i] = &cp
	}
	return out
}

// ── painting ──────────────────────────────────────────────────────────────────

// DrawText format flags for tab labels (Walk wraps DT_ win32 constants as uint).
const (
	dtTabTitle  = walk.DrawTextFormat(win.DT_SINGLELINE | win.DT_VCENTER | win.DT_END_ELLIPSIS)
	dtTabCenter = walk.DrawTextFormat(win.DT_SINGLELINE | win.DT_VCENTER | win.DT_CENTER)
)

func (ts *TabStrip) paint(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
	ts.mu.RLock()
	tabs := make([]*Tab, len(ts.tabs))
	copy(tabs, ts.tabs)
	active := ts.active
	ts.mu.RUnlock()

	bgBrush, err := walk.NewSolidColorBrush(ColorSurface)
	if err != nil {
		return err
	}
	defer bgBrush.Dispose()
	canvas.FillRectangle(bgBrush, updateBounds)

	if len(tabs) == 0 {
		return nil
	}

	bounds := ts.widget.ClientBounds()
	totalW := bounds.Width - 34 // reserve room for the + button on the right
	tabW := totalW / len(tabs)
	if tabW > TabMaxWidth {
		tabW = TabMaxWidth
	}
	if tabW < TabMinWidth {
		tabW = TabMinWidth
	}

	font := Font() // may be nil before first font init; drawing is skipped below

	for i, t := range tabs {
		x := i * tabW
		y := 4
		w := tabW - 2
		h := bounds.Height - 4

		isActive := i == active

		// ── Tab background ────────────────────────────────────────────────
		fillColor := ColorSurface
		if isActive {
			fillColor = ColorSurfaceRaised
		}
		if tabBrush, bErr := walk.NewSolidColorBrush(fillColor); bErr == nil {
			canvas.FillRectangle(tabBrush, walk.Rectangle{X: x, Y: y, Width: w, Height: h})
			tabBrush.Dispose()
		}

		// Active tab: blue accent line along the top edge.
		if isActive {
			if accentBrush, aErr := walk.NewSolidColorBrush(ColorAccent); aErr == nil {
				canvas.FillRectangle(accentBrush, walk.Rectangle{X: x, Y: y, Width: w, Height: 3})
				accentBrush.Dispose()
			}
		}

		if font == nil {
			continue // font not ready; skip text this frame
		}

		// ── Tab title ─────────────────────────────────────────────────────
		title := t.displayTitle()
		if t.Loading {
			title = "... " + title
		}
		textColor := ColorText
		if !isActive {
			textColor = ColorTextMuted
		}
		textRect := walk.Rectangle{
			X:      x + TabPadX,
			Y:      y + 4,
			Width:  w - TabCloseSize - TabPadX*2,
			Height: h - 8,
		}
		_ = canvas.DrawText(title, font, textColor, textRect, dtTabTitle)

		// ── Close (×) button ──────────────────────────────────────────────
		// Show on active tab always, and on inactive tabs when there are
		// multiple tabs (so the user can close without switching first).
		if isActive || len(tabs) > 1 {
			closeX := x + w - TabCloseSize - 4
			closeY := y + (h-TabCloseSize)/2
			closeRect := walk.Rectangle{
				X: closeX, Y: closeY,
				Width: TabCloseSize, Height: TabCloseSize,
			}
			closeColor := ColorTextMuted
			if isActive {
				closeColor = ColorText
			}
			_ = canvas.DrawText("×", font, closeColor, closeRect, dtTabCenter)
		}
	}

	// ── "+" new-tab button ────────────────────────────────────────────────
	if font != nil {
		btnX := len(tabs) * tabW
		if btnX > totalW {
			btnX = totalW
		}
		newRect := walk.Rectangle{X: btnX + 4, Y: 4, Width: 24, Height: bounds.Height - 4}
		_ = canvas.DrawText("+", font, ColorTextMuted, newRect, dtTabCenter)
	}

	return nil
}

// ── mouse events ──────────────────────────────────────────────────────────────

func (ts *TabStrip) onMouseDown(x, y int, button walk.MouseButton) {
	if button != walk.LeftButton {
		return
	}
	ts.mu.RLock()
	tabs := make([]*Tab, len(ts.tabs))
	copy(tabs, ts.tabs)
	ts.mu.RUnlock()

	bounds := ts.widget.ClientBounds()
	totalW := bounds.Width - 34
	tabW := totalW / len(tabs)
	if tabW > TabMaxWidth {
		tabW = TabMaxWidth
	}
	if tabW < TabMinWidth {
		tabW = TabMinWidth
	}

	plusX := len(tabs) * tabW
	if x >= plusX && x <= plusX+32 {
		if ts.OnNew != nil {
			ts.OnNew()
		}
		return
	}

	for i, t := range tabs {
		tabLeft := i * tabW
		tabRight := tabLeft + tabW - 2
		if x < tabLeft || x > tabRight {
			continue
		}
		closeLeft := tabRight - TabCloseSize - 4
		if x >= closeLeft && (len(tabs) > 1 || i == ts.active) {
			if ts.OnClose != nil {
				ts.OnClose(t.ID)
			}
			return
		}
		ts.mu.Lock()
		ts.active = i
		ts.mu.Unlock()
		ts.widget.Invalidate()
		if ts.OnSelect != nil {
			ts.OnSelect(t.ID)
		}
		return
	}
}

// ── private helpers ───────────────────────────────────────────────────────────

func (ts *TabStrip) addTabLocked(url, title string) string {
	ts.seqID++
	id := fmt.Sprintf("tab-%d", ts.seqID)
	t := &Tab{
		ID:      id,
		URL:     url,
		Title:   title,
		History: NewNavigationHistory(),
	}
	ts.tabs = append(ts.tabs, t)
	ts.active = len(ts.tabs) - 1
	return id
}

func (ts *TabStrip) indexByID(id string) int {
	for i, t := range ts.tabs {
		if t.ID == id {
			return i
		}
	}
	return -1
}
