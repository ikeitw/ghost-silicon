// internal/platform/windows/appcontainer/capabilities.go
//go:build windows

// Package appcontainer — AppContainer capability constants.
// These map to the well-known Windows AppContainer capability SIDs.
package appcontainer

// Capability is a named AppContainer capability.
type Capability string

const (
	// CapInternetClient allows outbound internet connections.
	CapInternetClient Capability = "internetClient"

	// CapInternetClientServer allows inbound and outbound internet connections.
	CapInternetClientServer Capability = "internetClientServer"

	// CapPrivateNetworkClientServer allows access to home/work networks.
	CapPrivateNetworkClientServer Capability = "privateNetworkClientServer"

	// CapPicturesLibrary allows read/write access to the pictures library.
	CapPicturesLibrary Capability = "picturesLibrary"

	// CapVideosLibrary allows read/write access to the videos library.
	CapVideosLibrary Capability = "videosLibrary"

	// CapMusicLibrary allows read/write access to the music library.
	CapMusicLibrary Capability = "musicLibrary"

	// CapDocumentsLibrary allows read/write access to the documents library.
	CapDocumentsLibrary Capability = "documentsLibrary"

	// CapWebcam allows access to the webcam.
	CapWebcam Capability = "webcam"

	// CapMicrophone allows access to the microphone.
	CapMicrophone Capability = "microphone"
)

// MinimalBrowsingCapabilities returns the minimum capability set needed
// for a renderer to perform basic web browsing.
func MinimalBrowsingCapabilities() []Capability {
	return []Capability{
		CapInternetClient,
	}
}

// FullBrowsingCapabilities returns all capabilities a full-featured
// browser session might need.
func FullBrowsingCapabilities() []Capability {
	return []Capability{
		CapInternetClient,
		CapPrivateNetworkClientServer,
	}
}
