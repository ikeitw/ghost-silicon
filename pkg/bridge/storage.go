// pkg/bridge/storage.go
// Package bridge — storage policy provider.
// Serves storage enablement flags and quota limits from the active profile.
package bridge

import "ghost-silicon/pkg/identity"

// StorageProvider extracts storage policy values from a profile.
type StorageProvider struct{}

// CookiesEnabled reports whether the session may persist cookies.
func (StorageProvider) CookiesEnabled(p *identity.Profile) bool {
	return p.Storage.EnableCookies
}

// LocalStorageEnabled reports whether localStorage is available.
func (StorageProvider) LocalStorageEnabled(p *identity.Profile) bool {
	return p.Storage.EnableLocalStorage
}

// SessionStorageEnabled reports whether sessionStorage is available.
func (StorageProvider) SessionStorageEnabled(p *identity.Profile) bool {
	return p.Storage.EnableSessionStorage
}

// IndexedDBEnabled reports whether IndexedDB is available.
func (StorageProvider) IndexedDBEnabled(p *identity.Profile) bool {
	return p.Storage.EnableIndexedDB
}

// CacheStorageEnabled reports whether the Cache Storage API is available.
func (StorageProvider) CacheStorageEnabled(p *identity.Profile) bool {
	return p.Storage.EnableCacheStorage
}

// ServiceWorkerEnabled reports whether Service Workers may be registered.
func (StorageProvider) ServiceWorkerEnabled(p *identity.Profile) bool {
	return p.Storage.EnableServiceWorker
}

// MaxCookieJarMB returns the cookie jar size cap in megabytes (0 = no limit).
func (StorageProvider) MaxCookieJarMB(p *identity.Profile) int {
	return p.Storage.MaxCookieJarMB
}

// MaxStorageMB returns the total web storage cap in megabytes (0 = no limit).
func (StorageProvider) MaxStorageMB(p *identity.Profile) int {
	return p.Storage.MaxStorageMB
}
