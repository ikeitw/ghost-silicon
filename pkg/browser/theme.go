// Theme defines the Ghost-Silicon browser visual language.
// Every colour, font, and spacing constant lives here — swap this file to
// switch colour schemes without touching any widget code.
//
// No build tag: Walk types used here compile on all platforms Walk supports.
package browser

import "github.com/lxn/walk"

// ── Pixel dimensions ─────────────────────────────────────────────────────────

const (
	ToolbarHeight = 44  // px — address-bar row
	TabBarHeight  = 38  // px — tab strip row
	TabMinWidth   = 80  // px — narrowest a pinned tab gets
	TabMaxWidth   = 220 // px — widest a normal tab renders
	TabCloseSize  = 16  // px — close-button hit area inside a tab
	TabPadX       = 12  // px — horizontal tab text padding
	ButtonSize    = 28  // px — square nav-button side
	AddressHeight = 30  // px — address bar inner height
	BadgeDiameter = 10  // px — profile indicator dot
	PaddingS      = 6   // px — small gap
	PaddingM      = 10  // px — medium gap
	PaddingL      = 16  // px — large gap
)

// ── Light-theme colour palette ────────────────────────────────────────────────
// The toolbar and tab strip are Walk Win32 widgets sitting above WebView2.
// Using light colours makes them clearly distinct from the white page content.

var (
	// Structural
	ColorBackground    = walk.RGB(248, 248, 250) // main window fill
	ColorSurface       = walk.RGB(237, 238, 242) // toolbar / tab-bar bg
	ColorSurfaceRaised = walk.RGB(255, 255, 255) // active tab / input field
	ColorBorder        = walk.RGB(210, 210, 218) // hairline borders

	// Interactive
	ColorAccent    = walk.RGB(66, 133, 244)  // Google-blue primary action
	ColorAccentDim = walk.RGB(30, 90, 190)   // pressed / secondary action
	ColorHover     = walk.RGB(220, 222, 228) // button hover fill
	ColorActive    = walk.RGB(200, 202, 210) // button pressed fill

	// Text
	ColorText      = walk.RGB(25, 25, 35)    // primary body text
	ColorTextMuted = walk.RGB(110, 110, 125) // secondary / placeholder
	ColorTextURL   = walk.RGB(20, 120, 40)   // URL in address bar

	// Semantic
	ColorDanger      = walk.RGB(200, 50, 50) // close-tab hover, error
	ColorSecureURL   = walk.RGB(30, 130, 60) // https: lock indicator
	ColorInsecureURL = walk.RGB(180, 80, 20) // http: indicator
	ColorProfileDot  = walk.RGB(50, 150, 90) // profile badge dot

	// Per-session accent
	ColorSessionAccent = walk.RGB(66, 133, 244)
)

// ── Fonts (lazy-initialised) ──────────────────────────────────────────────────

var (
	cachedFont        *walk.Font
	cachedFontBold    *walk.Font
	cachedFontAddress *walk.Font
	cachedFontSmall   *walk.Font
	cachedFontMono    *walk.Font
)

// Font returns the standard UI font (Segoe UI 9pt).
func Font() *walk.Font { return lazyFont(&cachedFont, "Segoe UI", 9, 0) }

// FontBold returns Segoe UI 9pt bold.
func FontBold() *walk.Font { return lazyFont(&cachedFontBold, "Segoe UI", 9, walk.FontBold) }

// FontAddress returns the address-bar font (Segoe UI 11pt).
func FontAddress() *walk.Font { return lazyFont(&cachedFontAddress, "Segoe UI", 11, 0) }

// FontSmall returns the small caption font (Segoe UI 8pt).
func FontSmall() *walk.Font { return lazyFont(&cachedFontSmall, "Segoe UI", 8, 0) }

// FontMono returns the monospace font used in DevTools (Cascadia Code 9pt,
// falls back to Consolas if Cascadia is not installed).
func FontMono() *walk.Font {
	f := lazyFont(&cachedFontMono, "Cascadia Code", 9, 0)
	if f == nil {
		f = lazyFont(&cachedFontMono, "Consolas", 9, 0)
	}
	return f
}

// DisposeThemeFonts releases all cached font resources.
// Call this when the application exits.
func DisposeThemeFonts() {
	for _, f := range []*(*walk.Font){
		&cachedFont, &cachedFontBold, &cachedFontAddress,
		&cachedFontSmall, &cachedFontMono,
	} {
		if *f != nil {
			(*f).Dispose()
			*f = nil
		}
	}
}

func lazyFont(ptr **walk.Font, face string, size int, style walk.FontStyle) *walk.Font {
	if *ptr != nil {
		return *ptr
	}
	f, err := walk.NewFont(face, size, style)
	if err != nil {
		return nil // caller falls back gracefully
	}
	*ptr = f
	return f
}
