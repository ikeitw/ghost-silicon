//go:build windows

// Package browser — developer tools toggle.
// DevToolsPanel is a thin handle around the WebView2-native DevTools window.
// The window itself is owned and managed by the WebView2 runtime.
package browser

import (
	"fmt"

	webview2 "github.com/jchv/go-webview2"
	"github.com/lxn/walk"
)

// DevToolsPanel manages the DevTools window for a single WebView2 view.
// No Walk widgets are created — DevTools is a separate OS window owned by
// the WebView2 runtime.
type DevToolsPanel struct {
	wv       webview2.WebView
	open     bool
	hotkey   *walk.Action // bound in menu.go; toggled here
	OnToggle func()       // if set, called instead of the default openDevToolsWindow
}

func NewDevToolsPanel(wv webview2.WebView) *DevToolsPanel {
	return &DevToolsPanel{wv: wv}
}

// Toggle opens DevTools via ICoreWebView2.OpenDevToolsWindow.
// WebView2 does not expose a close API, so a second call just re-focuses it.
// Must be called from the UI thread (Walk action Triggered() satisfies this).
func (d *DevToolsPanel) Toggle() {
	if d.OnToggle != nil {
		d.OnToggle()
		return
	}
	if d.wv == nil {
		return
	}
	openDevToolsWindow(extractChromium(d.wv))
	d.open = true
}

// IsOpen is advisory: the user can close DevTools via its own close button,
// so the flag may be stale.
func (d *DevToolsPanel) IsOpen() bool {
	return d.open
}

// InjectConsoleShortcut binds F12 to Toggle() via the menu.go action.
func (d *DevToolsPanel) InjectConsoleShortcut(action *walk.Action) {
	if action == nil {
		return
	}
	d.hotkey = action
	action.SetShortcut(walk.Shortcut{
		Key: walk.KeyF12,
	})
	action.Triggered().Attach(func() { d.Toggle() })
}

// InspectElement opens DevTools and attempts to highlight the element at (x, y).
func (d *DevToolsPanel) InspectElement(x, y int) {
	if d.wv == nil {
		return
	}
	openDevToolsWindow(extractChromium(d.wv))
	d.wv.Eval(fmt.Sprintf(
		`if(window.__proto__.inspect){inspect(document.elementFromPoint(%d,%d))}`,
		x, y,
	))
	d.open = true
}
