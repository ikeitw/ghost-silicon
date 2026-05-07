module ghost-silicon

go 1.22.0

require (
	// Core dependencies (pre-existing)
	github.com/google/uuid v1.6.0

	// go-webview2: embeds the WebView2 runtime (ships with Edge on every
	// Windows 11 machine) as a Go-native widget.  The package exposes
	// webview2.WebView, NewWithOptions, Navigate, Init (polyfill injection),
	// Eval, Bind, and Dispatch — all used in pkg/browser/webview.go.
	github.com/jchv/go-webview2 v0.0.0-20221223143126-dc24628cff85

	// Browser GUI — added for pkg/browser
	//
	// Walk: Win32-native Go UI toolkit. Provides MainWindow, CustomWidget,
	// PushButton, LineEdit, ListBox, Menu, Action, Canvas, and all layout
	// types used throughout pkg/browser.
	// Requires CGO and the Walk manifest (see deployments/windows/).
	github.com/lxn/walk v0.0.0-20210112085537-c389da54e794
	golang.org/x/sys v0.21.0
	gopkg.in/yaml.v3 v3.0.1
)

// Walk's indirect dependency — required for Win32 constants and GDI types
// used in pkg/browser/tabs.go (win.DrawText, win.SetTextColor, etc.).
// golang.org/x/sys already satisfies this transitively but we pin it
// explicitly so the go toolchain does not downgrade it.
require github.com/lxn/win v0.0.0-20210218163916-a377121e959e

require (
	github.com/jchv/go-winloader v0.0.0-20200815041850-dec1ee9a7fd5 // indirect
	gopkg.in/Knetic/govaluate.v3 v3.0.0 // indirect
)
