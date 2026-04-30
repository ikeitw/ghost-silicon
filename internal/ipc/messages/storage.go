// internal/ipc/messages/storage.go
// Package messages defines request/response types for the storage subsystem.
package messages

// StoragePathsResponse is returned by storage.getPaths.
// It tells the renderer where to write each storage category so that
// session isolation is enforced by directory separation.
type StoragePathsResponse struct {
	// Root is the session root directory (--user-data-dir equivalent).
	Root string `json:"root"`

	// Cache is the HTTP cache directory.
	Cache string `json:"cache"`

	// Cookies is the cookie storage directory.
	Cookies string `json:"cookies"`

	// LocalData is for localStorage / IndexedDB.
	LocalData string `json:"local_data"`

	// Downloads is the default download destination.
	Downloads string `json:"downloads"`

	// Logs is where renderer logs are written.
	Logs string `json:"logs"`
}

// StorageClearRequest is the request body for storage.clear.
type StorageClearRequest struct {
	// Types lists which storage categories to clear.
	// Valid values: "cache", "cookies", "local_data", "all".
	Types []string `json:"types"`
}

// StorageClearResponse is returned by storage.clear.
type StorageClearResponse struct {
	Cleared []string `json:"cleared"`
}

// Method names for the storage subsystem.
const (
	MethodStorageGetPaths = "storage.getPaths"
	MethodStorageClear    = "storage.clear"
)
