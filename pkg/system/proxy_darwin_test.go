//go:build darwin

package system

import (
	"os/exec"
	"strings"
	"testing"
)

// primary_service_via_scutil resolves the primary network service independently of the code
// under test, so the assertions below cannot pass just because the implementation agrees with
// itself.
func primary_service_via_scutil(t *testing.T) string {
	t.Helper()
	for _, key := range []string{"State:/Network/Global/IPv4", "State:/Network/Global/IPv6"} {
		uuid := scutil_show_field(t, key, "PrimaryService")
		if uuid == "" {
			continue
		}
		if name := scutil_show_field(t, "Setup:/Network/Service/"+uuid, "UserDefinedName"); name != "" {
			return name
		}
	}
	return ""
}

func scutil_show_field(t *testing.T, key string, field string) string {
	t.Helper()
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader("show " + key + "\n")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == field {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// This is the regression test for the bug this file exists to prevent.
//
// merge_default_settings decides which network service the system proxy is written to. macOS
// stores proxy settings per service and honours only the primary one, so writing to any other
// service silently does nothing — networksetup reports no error either way.
//
// The previous implementation resolved the primary interface and looked it up in
// `networksetup -listallhardwareports`. VPN services are absent from that list, so a utun
// primary interface never matched and the lookup fell back to a hardcoded "Wi-Fi". Asserting
// against an independently resolved primary service makes that fallback fail the test instead
// of passing silently.
func TestMergeDefaultSettingsTargetsPrimaryService(t *testing.T) {
	want := primary_service_via_scutil(t)
	if want == "" {
		t.Skip("no primary network service; nothing to compare against")
	}
	got := merge_default_settings(ProxySettings{}).Device
	if got != want {
		t.Errorf("merge_default_settings targets network service %q, but the primary service is %q. "+
			"The proxy would be written to a service that is not in effect, and networksetup would not report an error.", got, want)
	}
}

// An explicitly configured service must win over detection, otherwise --dev silently does
// nothing.
func TestMergeDefaultSettingsKeepsConfiguredService(t *testing.T) {
	got := merge_default_settings(ProxySettings{Device: "Some Service"}).Device
	if got != "Some Service" {
		t.Errorf("Device = %q, want the configured value to be preserved", got)
	}
}

// The resolved name is passed straight to networksetup, which addresses services by name, so
// it has to exist in the service list.
func TestGetNetworkInterfacesReturnsUsableServiceName(t *testing.T) {
	port, err := get_network_interfaces()
	if err != nil {
		t.Skipf("cannot determine the primary network service (no active network?): %v", err)
	}
	if port == nil || port.Port == "" {
		t.Fatal("no service name resolved")
	}
	output, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		t.Skipf("networksetup failed: %v", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		// The first line is a note, and disabled services are prefixed with "*".
		if strings.TrimPrefix(strings.TrimSpace(line), "*") == port.Port {
			t.Logf("primary service %q (interface %q) is a valid networksetup service name", port.Port, port.Device)
			return
		}
	}
	t.Errorf("primary service %q is missing from networksetup -listallnetworkservices. Services:\n%s", port.Port, output)
}

// ProxyTargetDescription must not invent a service name when one was configured explicitly,
// and must not warn in that case either.
func TestProxyTargetDescriptionPrefersConfiguredService(t *testing.T) {
	service, warning := ProxyTargetDescription("Some Service")
	if service != "Some Service" {
		t.Errorf("service = %q, want the configured value", service)
	}
	if warning != "" {
		t.Errorf("warning = %q, want none when the service is configured", warning)
	}
}
