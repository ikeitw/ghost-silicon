// internal/policy/javascript/blocklist.go
// Package javascript — API blocklist.
// Lists JavaScript APIs that are suppressed or intercepted by the renderer
// adapter's injection layer for privacy reasons.
package javascript

// BlockedAPIs is the set of JavaScript API names that are intercepted
// and either suppressed or replaced with profile-backed values.
var BlockedAPIs = []string{
	"navigator.getBattery",                    // battery status leaks charge level
	"navigator.connection",                    // network type can fingerprint device
	"navigator.deviceMemory",                  // replaced by profile value
	"navigator.hardwareConcurrency",           // replaced by profile value
	"screen.width",                            // replaced by profile value
	"screen.height",                           // replaced by profile value
	"screen.availWidth",                       // replaced by profile value
	"screen.availHeight",                      // replaced by profile value
	"screen.colorDepth",                       // replaced by profile value
	"screen.pixelDepth",                       // replaced by profile value
	"window.devicePixelRatio",                 // replaced by profile value
	"HTMLCanvasElement.toDataURL",             // noise-injected
	"HTMLCanvasElement.toBlob",                // noise-injected
	"CanvasRenderingContext2D.getImageData",   // noise-injected
	"WebGLRenderingContext.getParameter",      // noise-injected for vendor/renderer
	"RTCPeerConnection",                       // blocked unless webrtc_policy = "allow"
	"navigator.mediaDevices.enumerateDevices", // replaced by media policy list
}

// IsBlocked reports whether apiName is in the blocklist.
func IsBlocked(apiName string) bool {
	for _, b := range BlockedAPIs {
		if b == apiName {
			return true
		}
	}
	return false
}
