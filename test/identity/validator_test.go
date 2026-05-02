// test/identity/validator_test.go
package identity_test

import (
	"testing"

	"ghost-silicon/pkg/identity"
)

func TestValidate_AllTemplatesPass(t *testing.T) {
	templates := []identity.TemplateName{
		identity.TemplateWindows11Desktop,
		identity.TemplateWindows11Laptop,
		identity.TemplateMobileLike,
		identity.TemplateHardened,
	}
	for _, tmpl := range templates {
		p := identity.FromTemplate(tmpl)
		if p == nil {
			t.Fatalf("FromTemplate(%q) returned nil", tmpl)
		}
		result := identity.Validate(p)
		if !result.Valid() {
			t.Errorf("template %q failed validation:\n%s", tmpl, result.Error())
		}
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.ID = ""
	p.Browser.UserAgent = "bad"
	p.Hardware.CPUCores = 99

	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected multiple validation errors")
	}
	if len(result.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestValidate_PortraitScreen(t *testing.T) {
	p := identity.MobileLikeTemplate()
	result := identity.Validate(p)
	if !result.Valid() {
		t.Fatalf("mobile-like template failed validation:\n%s", result.Error())
	}
}

func TestValidate_InvalidProxyScheme(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Network.ProxyURL = "ftp://proxy.example.com:21"
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for ftp:// proxy scheme")
	}
}

func TestValidate_ValidSocks5Proxy(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Network.ProxyURL = "socks5://127.0.0.1:1080"
	result := identity.Validate(p)
	if !result.Valid() {
		t.Fatalf("expected socks5 proxy to be valid:\n%s", result.Error())
	}
}
