// internal/policy/javascript/allowlist.go
// Package javascript — API allowlist.
// Lists the JavaScript APIs that are explicitly permitted to run unmodified.
package javascript

// AllowedAPIs is the set of JavaScript API names that pass through
// the renderer without any interception or noise injection.
// Everything not in this set is either overridden or denied by the policy.
var AllowedAPIs = []string{
	"fetch",
	"XMLHttpRequest",
	"WebSocket",
	"console",
	"setTimeout",
	"setInterval",
	"requestAnimationFrame",
	"Promise",
	"Worker",
	"Blob",
	"URL",
	"URLSearchParams",
	"TextEncoder",
	"TextDecoder",
	"crypto.getRandomValues",
	"localStorage",   // subject to StoragePolicy.EnableLocalStorage
	"sessionStorage", // subject to StoragePolicy.EnableSessionStorage
	"indexedDB",      // subject to StoragePolicy.EnableIndexedDB
	"caches",         // subject to StoragePolicy.EnableCacheStorage
}

// IsAllowed reports whether apiName is in the allowlist.
func IsAllowed(apiName string) bool {
	for _, a := range AllowedAPIs {
		if a == apiName {
			return true
		}
	}
	return false
}
