// pkg/bridge/navigator.go
// Package bridge — navigator provider.
// Serves all navigator.* string and array values from the active profile.
package bridge

import "ghost-silicon/pkg/identity"

// NavigatorProvider extracts browser/navigator identity values from a profile.
type NavigatorProvider struct{}

// UserAgent returns the navigator.userAgent string.
func (NavigatorProvider) UserAgent(p *identity.Profile) string { return p.Browser.UserAgent }

// AppVersion returns the navigator.appVersion string (everything after "Mozilla/").
func (NavigatorProvider) AppVersion(p *identity.Profile) string { return p.Browser.AppVersion }

// Vendor returns navigator.vendor (e.g. "Google Inc.").
func (NavigatorProvider) Vendor(p *identity.Profile) string { return p.Browser.Vendor }

// VendorSub returns navigator.vendorSub.
func (NavigatorProvider) VendorSub(p *identity.Profile) string { return p.Browser.VendorSub }

// Product returns navigator.product (always "Gecko" per spec).
func (NavigatorProvider) Product(p *identity.Profile) string { return p.Browser.Product }

// ProductSub returns navigator.productSub.
func (NavigatorProvider) ProductSub(p *identity.Profile) string { return p.Browser.ProductSub }

// Languages returns navigator.languages as a string slice.
func (NavigatorProvider) Languages(p *identity.Profile) []string { return p.Browser.Languages }

// Language returns navigator.language (the first entry in Languages).
func (NavigatorProvider) Language(p *identity.Profile) string { return p.Browser.PrimaryLanguage() }

// DoNotTrack returns navigator.doNotTrack ("1", "0", or "").
func (NavigatorProvider) DoNotTrack(p *identity.Profile) string { return p.Browser.DoNotTrack }

// CookieEnabled returns navigator.cookieEnabled.
func (NavigatorProvider) CookieEnabled(p *identity.Profile) bool { return p.Browser.CookieEnabled }
