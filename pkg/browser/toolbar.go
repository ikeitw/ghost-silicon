// pkg/browser/toolbar.go
//go:build windows

// Package browser — browser toolbar.
// Toolbar is a Walk Composite containing the navigation buttons, the address
// bar, and the profile indicator badge.  It communicates with the rest of the
// browser via narrow callback hooks so it can be tested without a live WebView.
package browser

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"
)

// Toolbar groups the Walk controls that make up the browser chrome row.
type Toolbar struct {
	composite *walk.Composite

	backBtn     *walk.PushButton
	fwdBtn      *walk.PushButton
	reloadBtn   *walk.PushButton
	stopBtn     *walk.PushButton
	homeBtn     *walk.PushButton
	addrBar     *walk.LineEdit
	bookmarkBtn *walk.PushButton
	menuBtn     *walk.PushButton

	profileLabel *walk.Label // shows active profile name
	secureLabel  *walk.Label // shows 🔒 or ⚠ based on scheme

	// loading tracks whether a page is currently loading, to toggle
	// reload/stop button visibility.
	loading bool

	// Callbacks — wired by Browser after construction.
	OnNavigate   func(url string)
	OnBack       func()
	OnForward    func()
	OnReload     func()
	OnStop       func()
	OnHome       func()
	OnToggleMenu func()
	OnBookmark   func(url string)
}

// NewToolbar creates the toolbar composite as a child of parent.
func NewToolbar(parent walk.Container) (*Toolbar, error) {
	tb := &Toolbar{}

	comp, err := walk.NewComposite(parent)
	if err != nil {
		return nil, fmt.Errorf("toolbar composite: %w", err)
	}
	tb.composite = comp

	// Fixed height.
	comp.SetMinMaxSize(
		walk.Size{Width: 0, Height: ToolbarHeight},
		walk.Size{Width: 0, Height: ToolbarHeight},
	)

	// Dark background.
	bgBrush, err := walk.NewSolidColorBrush(ColorSurface)
	if err == nil {
		comp.SetBackground(bgBrush)
	}

	// Horizontal layout with fixed margins.
	layout := walk.NewHBoxLayout()
	layout.SetMargins(walk.Margins{
		HNear: PaddingM,
		VNear: (ToolbarHeight - ButtonSize) / 2,
		HFar:  PaddingM,
		VFar:  (ToolbarHeight - ButtonSize) / 2,
	})
	layout.SetSpacing(PaddingS)
	comp.SetLayout(layout)

	// ── Nav buttons ───────────────────────────────────────────────────────
	if tb.backBtn, err = makeNavButton(comp, "←"); err != nil {
		return nil, err
	}
	if tb.fwdBtn, err = makeNavButton(comp, "→"); err != nil {
		return nil, err
	}
	if tb.reloadBtn, err = makeNavButton(comp, "↻"); err != nil {
		return nil, err
	}
	if tb.stopBtn, err = makeNavButton(comp, "✕"); err != nil {
		return nil, err
	}
	if tb.homeBtn, err = makeNavButton(comp, "⌂"); err != nil {
		return nil, err
	}
	tb.stopBtn.SetVisible(false) // hidden until loading

	// ── Security indicator ────────────────────────────────────────────────
	tb.secureLabel, err = walk.NewLabel(comp)
	if err != nil {
		return nil, fmt.Errorf("secure label: %w", err)
	}
	tb.secureLabel.SetText("🔒")
	tb.secureLabel.SetMinMaxSize(
		walk.Size{Width: 20, Height: ButtonSize},
		walk.Size{Width: 20, Height: ButtonSize},
	)

	// ── Address bar ───────────────────────────────────────────────────────
	tb.addrBar, err = walk.NewLineEdit(comp)
	if err != nil {
		return nil, fmt.Errorf("address bar: %w", err)
	}
	tb.addrBar.SetCueBanner("Search or enter address")
	if f := FontAddress(); f != nil {
		tb.addrBar.SetFont(f)
	}
	// Stretch to fill available space.
	_ = layout.SetStretchFactor(tb.addrBar, 1)

	// Pressing Enter navigates.
	tb.addrBar.KeyPress().Attach(func(key walk.Key) {
		if key == walk.KeyReturn {
			tb.commitAddress()
		}
	})

	// ── Bookmark toggle ───────────────────────────────────────────────────
	if tb.bookmarkBtn, err = makeNavButton(comp, "☆"); err != nil {
		return nil, err
	}

	// ── Profile badge ─────────────────────────────────────────────────────
	tb.profileLabel, err = walk.NewLabel(comp)
	if err != nil {
		return nil, fmt.Errorf("profile label: %w", err)
	}
	tb.profileLabel.SetText("●")
	if f := FontSmall(); f != nil {
		tb.profileLabel.SetFont(f)
	}
	tb.profileLabel.SetMinMaxSize(
		walk.Size{Width: 24, Height: ButtonSize},
		walk.Size{Width: 24, Height: ButtonSize},
	)
	tb.profileLabel.SetToolTipText("Active profile")

	// ── Hamburger menu ────────────────────────────────────────────────────
	if tb.menuBtn, err = makeNavButton(comp, "⋮"); err != nil {
		return nil, err
	}

	// ── Wire button events ────────────────────────────────────────────────
	tb.backBtn.Clicked().Attach(func() {
		if tb.OnBack != nil {
			tb.OnBack()
		}
	})
	tb.fwdBtn.Clicked().Attach(func() {
		if tb.OnForward != nil {
			tb.OnForward()
		}
	})
	tb.reloadBtn.Clicked().Attach(func() {
		if tb.OnReload != nil {
			tb.OnReload()
		}
	})
	tb.stopBtn.Clicked().Attach(func() {
		if tb.OnStop != nil {
			tb.OnStop()
		}
	})
	tb.homeBtn.Clicked().Attach(func() {
		if tb.OnHome != nil {
			tb.OnHome()
		}
	})
	tb.bookmarkBtn.Clicked().Attach(func() {
		if tb.OnBookmark != nil {
			tb.OnBookmark(tb.addrBar.Text())
		}
	})
	tb.menuBtn.Clicked().Attach(func() {
		if tb.OnToggleMenu != nil {
			tb.OnToggleMenu()
		}
	})

	return tb, nil
}

// SetURL updates the address bar text.
func (tb *Toolbar) SetURL(url string) {
	_ = tb.addrBar.SetText(url)
	tb.updateSecureIndicator(url)
}

// SetLoading toggles between the reload and stop buttons and dims the
// address bar to indicate a page is in flight.
func (tb *Toolbar) SetLoading(loading bool) {
	tb.loading = loading
	tb.reloadBtn.SetVisible(!loading)
	tb.stopBtn.SetVisible(loading)
}

// SetNavState enables/disables the back and forward buttons.
func (tb *Toolbar) SetNavState(canBack, canForward bool) {
	tb.backBtn.SetEnabled(canBack)
	tb.fwdBtn.SetEnabled(canForward)
}

// SetProfileName updates the profile badge tooltip and label.
func (tb *Toolbar) SetProfileName(name string) {
	tb.profileLabel.SetToolTipText("Profile: " + name)
}

// SetBookmarked updates the bookmark star button to filled (★) or empty (☆).
func (tb *Toolbar) SetBookmarked(is bool) {
	if is {
		tb.bookmarkBtn.SetText("★")
	} else {
		tb.bookmarkBtn.SetText("☆")
	}
}

// ── private ───────────────────────────────────────────────────────────────────

// commitAddress reads the address bar and fires OnNavigate with a normalised URL.
func (tb *Toolbar) commitAddress() {
	raw := strings.TrimSpace(tb.addrBar.Text())
	if raw == "" {
		return
	}
	url := normaliseURL(raw)
	_ = tb.addrBar.SetText(url)
	if tb.OnNavigate != nil {
		tb.OnNavigate(url)
	}
}

func (tb *Toolbar) updateSecureIndicator(url string) {
	switch {
	case strings.HasPrefix(url, "https://"):
		tb.secureLabel.SetText("🔒")
		tb.secureLabel.SetToolTipText("Secure connection (HTTPS)")
	case strings.HasPrefix(url, "http://"):
		tb.secureLabel.SetText("⚠")
		tb.secureLabel.SetToolTipText("Not secure (HTTP)")
	default:
		tb.secureLabel.SetText("  ")
		tb.secureLabel.SetToolTipText("")
	}
}

// normaliseURL turns a bare hostname or query string into a usable URL.
// Full URLs pass through unchanged.
func normaliseURL(raw string) string {
	if strings.HasPrefix(raw, "ghost://") ||
		strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "file://") {
		return raw
	}
	// Looks like a hostname (contains a dot, no spaces).
	if !strings.Contains(raw, " ") && strings.Contains(raw, ".") {
		return "https://" + raw
	}
	// Treat as a search query.
	return "https://search.brave.com/search?q=" + strings.ReplaceAll(raw, " ", "+")
}

// makeNavButton creates a small square PushButton with centred text.
func makeNavButton(parent walk.Container, label string) (*walk.PushButton, error) {
	btn, err := walk.NewPushButton(parent)
	if err != nil {
		return nil, fmt.Errorf("nav button %q: %w", label, err)
	}
	btn.SetText(label)
	btn.SetMinMaxSize(
		walk.Size{Width: ButtonSize, Height: ButtonSize},
		walk.Size{Width: ButtonSize, Height: ButtonSize},
	)
	return btn, nil
}
