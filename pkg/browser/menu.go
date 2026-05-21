//go:build windows

// Package browser — application menu and keyboard shortcuts.
// All Walk Actions are created here. Callers bind Triggered() after the fact.
package browser

import (
	"fmt"

	"github.com/lxn/walk"
)

// Actions groups every Walk Action that the browser exposes.
// Fields are populated by BuildMenu; callers bind Triggered() handlers.
type Actions struct {
	// Navigation
	Back    *walk.Action
	Forward *walk.Action
	Reload  *walk.Action
	Stop    *walk.Action
	Home    *walk.Action

	// Tabs
	NewTab   *walk.Action
	CloseTab *walk.Action

	// View
	ZoomIn     *walk.Action
	ZoomOut    *walk.Action
	ZoomReset  *walk.Action
	Fullscreen *walk.Action
	DevTools   *walk.Action

	// Profile
	SwitchProfile *walk.Action

	// Window
	Downloads *walk.Action
	Bookmarks *walk.Action
	Settings  *walk.Action

	// Application
	About *walk.Action
	Quit  *walk.Action
}

// BuildMenu attaches a full menu bar to mw and returns the populated Actions.
// The caller is responsible for binding Triggered() on each action.
func BuildMenu(mw *walk.MainWindow) (*Actions, error) {
	a := &Actions{}

	// Pre-create all actions.
	if err := a.createAll(); err != nil {
		return nil, err
	}

	// ── File menu ─────────────────────────────────────────────────────────
	fileMenu, err := walk.NewMenu()
	if err != nil {
		return nil, fmt.Errorf("new file menu: %w", err)
	}
	fileMenuAction, err := mw.Menu().Actions().AddMenu(fileMenu)
	if err != nil {
		return nil, fmt.Errorf("add file menu: %w", err)
	}
	fileMenuAction.SetText("&File")
	fileMenu.Actions().Add(a.NewTab)
	fileMenu.Actions().Add(a.CloseTab)
	fileMenu.Actions().Add(walk.NewSeparatorAction())
	fileMenu.Actions().Add(a.Downloads)
	fileMenu.Actions().Add(a.Bookmarks)
	fileMenu.Actions().Add(walk.NewSeparatorAction())
	fileMenu.Actions().Add(a.Quit)

	// ── View menu ─────────────────────────────────────────────────────────
	viewMenu, err := walk.NewMenu()
	if err != nil {
		return nil, fmt.Errorf("new view menu: %w", err)
	}
	viewMenuAction, err := mw.Menu().Actions().AddMenu(viewMenu)
	if err != nil {
		return nil, fmt.Errorf("add view menu: %w", err)
	}
	viewMenuAction.SetText("&View")
	viewMenu.Actions().Add(a.Reload)
	viewMenu.Actions().Add(a.Stop)
	viewMenu.Actions().Add(walk.NewSeparatorAction())
	viewMenu.Actions().Add(a.ZoomIn)
	viewMenu.Actions().Add(a.ZoomOut)
	viewMenu.Actions().Add(a.ZoomReset)
	viewMenu.Actions().Add(walk.NewSeparatorAction())
	viewMenu.Actions().Add(a.Fullscreen)
	viewMenu.Actions().Add(a.DevTools)

	// ── Profile menu ──────────────────────────────────────────────────────
	profileMenu, err := walk.NewMenu()
	if err != nil {
		return nil, fmt.Errorf("new profile menu: %w", err)
	}
	profileMenuAction, err := mw.Menu().Actions().AddMenu(profileMenu)
	if err != nil {
		return nil, fmt.Errorf("add profile menu: %w", err)
	}
	profileMenuAction.SetText("&Profile")
	profileMenu.Actions().Add(a.SwitchProfile)
	profileMenu.Actions().Add(walk.NewSeparatorAction())
	profileMenu.Actions().Add(a.Settings)

	// ── Help menu ─────────────────────────────────────────────────────────
	helpMenu, err := walk.NewMenu()
	if err != nil {
		return nil, fmt.Errorf("new help menu: %w", err)
	}
	helpMenuAction, err := mw.Menu().Actions().AddMenu(helpMenu)
	if err != nil {
		return nil, fmt.Errorf("add help menu: %w", err)
	}
	helpMenuAction.SetText("&Help")
	helpMenu.Actions().Add(a.About)

	return a, nil
}

// BuildContextMenu returns a right-click context menu for the WebView area.
// The caller owns the menu and must call Dispose() on it when done.
func BuildContextMenu(includeDevTools bool) (*walk.Menu, error) {
	menu, err := walk.NewMenu()
	if err != nil {
		return nil, err
	}

	back := walk.NewAction()
	back.SetText("Back")
	back.SetShortcut(walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyLeft})
	menu.Actions().Add(back)

	forward := walk.NewAction()
	forward.SetText("Forward")
	forward.SetShortcut(walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyRight})
	menu.Actions().Add(forward)

	reload := walk.NewAction()
	reload.SetText("Reload")
	reload.SetShortcut(walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyR})
	menu.Actions().Add(reload)

	menu.Actions().Add(walk.NewSeparatorAction())

	saveAs := walk.NewAction()
	saveAs.SetText("Save Page As…")
	menu.Actions().Add(saveAs)

	viewSource := walk.NewAction()
	viewSource.SetText("View Page Source")
	viewSource.SetShortcut(walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyU})
	menu.Actions().Add(viewSource)

	if includeDevTools {
		menu.Actions().Add(walk.NewSeparatorAction())
		inspect := walk.NewAction()
		inspect.SetText("Inspect Element")
		menu.Actions().Add(inspect)
	}

	return menu, nil
}

// ── private ───────────────────────────────────────────────────────────────────

func (a *Actions) createAll() error {
	var err error

	// Navigation
	if a.Back, err = actionWithShortcut("&Back",
		walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyLeft}); err != nil {
		return err
	}
	if a.Forward, err = actionWithShortcut("&Forward",
		walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyRight}); err != nil {
		return err
	}
	if a.Reload, err = actionWithShortcut("&Reload",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyR}); err != nil {
		return err
	}
	if a.Stop, err = actionWithShortcut("&Stop",
		walk.Shortcut{Key: walk.KeyEscape}); err != nil {
		return err
	}
	if a.Home, err = actionWithShortcut("&Home",
		walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyHome}); err != nil {
		return err
	}

	// Tabs
	if a.NewTab, err = actionWithShortcut("&New Tab",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyT}); err != nil {
		return err
	}
	if a.CloseTab, err = actionWithShortcut("&Close Tab",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyW}); err != nil {
		return err
	}

	// View
	if a.ZoomIn, err = actionWithShortcut("Zoom &In",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyAdd}); err != nil {
		return err
	}
	if a.ZoomOut, err = actionWithShortcut("Zoom &Out",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeySubtract}); err != nil {
		return err
	}
	if a.ZoomReset, err = actionWithShortcut("Reset &Zoom",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.Key0}); err != nil {
		return err
	}
	if a.Fullscreen, err = actionWithShortcut("&Full Screen",
		walk.Shortcut{Key: walk.KeyF11}); err != nil {
		return err
	}
	if a.DevTools, err = actionWithShortcut("&Developer Tools",
		walk.Shortcut{Key: walk.KeyF12}); err != nil {
		return err
	}

	// Profile / settings
	a.SwitchProfile = walk.NewAction()
	a.SwitchProfile.SetText("&Switch Profile…")
	a.Settings = walk.NewAction()
	a.Settings.SetText("&Settings")

	// Window panels
	if a.Downloads, err = actionWithShortcut("&Downloads",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyJ}); err != nil {
		return err
	}
	a.Bookmarks = walk.NewAction()
	a.Bookmarks.SetText("&Bookmarks")

	// Application
	a.About = walk.NewAction()
	a.About.SetText("&About Ghost-Silicon")
	if a.Quit, err = actionWithShortcut("&Quit",
		walk.Shortcut{Modifiers: walk.ModControl, Key: walk.KeyQ}); err != nil {
		return err
	}

	return nil
}

func actionWithShortcut(text string, sc walk.Shortcut) (*walk.Action, error) {
	a := walk.NewAction()
	a.SetText(text)
	if err := a.SetShortcut(sc); err != nil {
		// Non-fatal: shortcut may conflict on some machines.
		_ = err
	}
	return a, nil
}
