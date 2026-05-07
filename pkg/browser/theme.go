// pkg/browser/theme.go
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

// ── Dark-theme colour palette ─────────────────────────────────────────────────

var (
	// Structural
	ColorBackground    = walk.RGB(18, 18, 21) // main window fill
	ColorSurface       = walk.RGB(27, 27, 32) // toolbar / tab-bar bg
	ColorSurfaceRaised = walk.RGB(38, 38, 46) // active tab / input field
	ColorBorder        = walk.RGB(52, 52, 62) // hairline borders

	// Interactive
	ColorAccent    = walk.RGB(99, 148, 237) // cornflower-blue primary action
	ColorAccentDim = walk.RGB(60, 92, 160)  // pressed / secondary action
	ColorHover     = walk.RGB(46, 46, 56)   // button hover fill
	ColorActive    = walk.RGB(60, 60, 72)   // button pressed fill

	// Text
	ColorText      = walk.RGB(228, 228, 234) // primary body text
	ColorTextMuted = walk.RGB(138, 138, 154) // secondary / placeholder
	ColorTextURL   = walk.RGB(128, 198, 128) // URL in address bar

	// Semantic
	ColorDanger      = walk.RGB(218, 68, 68)  // close-tab hover, error
	ColorSecureURL   = walk.RGB(96, 196, 128) // https: lock indicator
	ColorInsecureURL = walk.RGB(218, 118, 78) // http: indicator
	ColorProfileDot  = walk.RGB(78, 178, 118) // profile badge dot

	// Per-session glow (same hue as accent; caller may vary saturation)
	ColorSessionAccent = walk.RGB(99, 148, 237)
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
