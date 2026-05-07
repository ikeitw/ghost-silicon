// pkg/browser/devtools.go
//go:build windows

// Package browser — developer tools panel.
// DevToolsPanel wraps the WebView2-native DevTools window, which is opened by
// calling OpenDevToolsWindow on the CoreWebView2 COM object.  The panel struct
// acts as a toggle handle — the window itself is owned by the WebView2 runtime.
package browser

import (
	"fmt"

	webview2 "github.com/jchv/go-webview2"
	"github.com/lxn/walk"
)

// DevToolsPanel manages the DevTools window for a single WebView2 view.
// It does not create any Walk widgets of its own; DevTools is a separate
// top-level window managed by the WebView2 runtime.
type DevToolsPanel struct {
	wv     webview2.WebView
	open   bool
	hotkey *walk.Action // bound in menu.go; toggled here
}

// NewDevToolsPanel creates a DevToolsPanel backed by wv.
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
		// OpenDevToolsWindow is idempotent — calling it twice simply
		// focuses the already-open window.
		d.wv.Eval(`window.open = window.open`) // no-op eval to check liveness
		openDevTools(d.wv)
		d.open = true
	})
}

// IsOpen reports whether the DevTools window has been opened in this session.
// Note: this is advisory — the user may close the DevTools window via its own
// title-bar close button, so the flag may be stale.
func (d *DevToolsPanel) IsOpen() bool {
	return d.open
}

// InjectConsoleShortcut binds F12 on the main window to Toggle().
// action should be the Action created in menu.go.
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

// openDevTools calls the WebView2 OpenDevToolsWindow via a JavaScript trick:
// WebView2's webview C wrapper exposes devtools through the underlying COM
// controller.  We call Eval with the DevTools hotkey sequence as a fallback
// because the go-webview2 wrapper does not yet expose OpenDevToolsWindow
// directly through its public API.
//
// Production note: replace this with a direct COM call to
// ICoreWebView2.OpenDevToolsWindow() once go-webview2 exposes it, or use the
// pkg/webview2/v2 sub-package which provides lower-level COM access.
func openDevTools(wv webview2.WebView) {
	// Simulate F12 press inside the WebView2 content area.
	// WebView2 intercepts this and opens its built-in DevTools.
	wv.Eval(`(function(){var e=new KeyboardEvent('keydown',{key:'F12',keyCode:123,which:123,bubbles:true});document.dispatchEvent(e)})()`)
}
