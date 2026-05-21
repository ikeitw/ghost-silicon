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
	wv     webview2.WebView
	open   bool
	hotkey *walk.Action // bound in menu.go; toggled here
}

func NewDevToolsPanel(wv webview2.WebView) *DevToolsPanel {
	return &DevToolsPanel{wv: wv}
}

// Toggle opens the DevTools window if it is closed, or focuses it if already
// open.  WebView2 does not expose a way to programmatically close DevTools, so
// a second call is treated as a focus request.
func (d *DevToolsPanel) Toggle() {
	if d.wv == nil {
		return
	}
	d.wv.Dispatch(func() {
		d.wv.Eval(`window.open = window.open`) // no-op eval to check liveness
		openDevTools(d.wv)
		d.open = true
	})
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

// InspectElement is a convenience that opens DevTools and attempts to
// highlight the element at the given page coordinates.
func (d *DevToolsPanel) InspectElement(x, y int) {
	if d.wv == nil {
		return
	}
	d.wv.Dispatch(func() {
		openDevTools(d.wv)
		js := fmt.Sprintf(
			`if(window.__proto__.inspect){inspect(document.elementFromPoint(%d,%d))}`,
			x, y,
		)
		d.wv.Eval(js)
		d.open = true
	})
}

// openDevTools triggers DevTools by dispatching a synthetic F12 keydown event.
// go-webview2 doesn't expose ICoreWebView2.OpenDevToolsWindow(); the synthetic
// key event is the workaround. Replace with a direct COM call if the wrapper
// ever exposes OpenDevToolsWindow.
func openDevTools(wv webview2.WebView) {
	wv.Eval(`(function(){var e=new KeyboardEvent('keydown',{key:'F12',keyCode:123,which:123,bubbles:true});document.dispatchEvent(e)})()`)
}
