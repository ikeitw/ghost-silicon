// test/sandbox/isolation_test.go
package sandbox_test

import (
	"testing"

	"ghost-silicon/pkg/security"
)

func TestCheckIsolation_ReturnsReport(t *testing.T) {
	report := security.CheckIsolation()
	if report == nil {
		t.Fatal("CheckIsolation returned nil")
	}
	if report.Platform == "" {
		t.Error("report.Platform must not be empty")
	}
}

func TestIsolationReport_Describe(t *testing.T) {
	report := security.CheckIsolation()
	desc := report.Describe()
	if desc == "" {
		t.Error("Describe returned empty string")
	}
}

func TestRequireLoopback_Valid(t *testing.T) {
	cases := []string{
		"127.0.0.1:9223",
		"127.0.0.1:8080",
	}
	for _, addr := range cases {
		if err := security.RequireLoopback(addr); err != nil {
			t.Errorf("RequireLoopback(%q): unexpected error: %v", addr, err)
		}
	}
}

func TestRequireLoopback_Invalid(t *testing.T) {
	cases := []string{
		"0.0.0.0:9223",
		"192.168.1.1:9223",
		"8.8.8.8:53",
	}
	for _, addr := range cases {
		if err := security.RequireLoopback(addr); err == nil {
			t.Errorf("RequireLoopback(%q): expected error for non-loopback address", addr)
		}
	}
}

func TestRequireLoopback_MalformedAddr(t *testing.T) {
	if err := security.RequireLoopback("not-an-address"); err == nil {
		t.Error("expected error for malformed address")
	}
}

func TestSafeDefaults_NoViolations(t *testing.T) {
	settings := map[string]string{
		"tls_skip_verify": "false",
		"integrity_level": "medium",
		"api_addr":        "127.0.0.1:9223",
	}
	vs := security.AssertSafeDefaults(settings)
	if len(vs) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(vs), vs)
	}
}

func TestSafeDefaults_TLSSkipVerify(t *testing.T) {
	settings := map[string]string{
		"tls_skip_verify": "true",
	}
	vs := security.AssertSafeDefaults(settings)
	if len(vs) == 0 {
		t.Error("expected violation for tls_skip_verify=true")
	}
}

func TestSafeDefaults_HighIntegrity(t *testing.T) {
	settings := map[string]string{
		"integrity_level": "high",
	}
	vs := security.AssertSafeDefaults(settings)
	if len(vs) == 0 {
		t.Error("expected violation for integrity_level=high on renderer")
	}
}

func TestSafeDefaults_NonLoopbackAPI(t *testing.T) {
	settings := map[string]string{
		"api_addr": "0.0.0.0:9223",
	}
	vs := security.AssertSafeDefaults(settings)
	if len(vs) == 0 {
		t.Error("expected violation for non-loopback api_addr")
	}
}
