package intel

import (
	"net/netip"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"Example.COM", "example.com", true},
		{"https://www.Example.com/x?y=1", "www.example.com", true},
		{"  example.org.  ", "example.org", true},
		{"sub.a-b.co.uk:8080/path", "sub.a-b.co.uk", true},
		{"", "", false},
		{"localhost", "", false},
		{"1.2.3.4", "", false},
		{"[::1]", "", false},
		{"-bad.com", "", false},
		{"exa mple.com", "", false},
		{"a..com", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeDomain(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeDomain(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeIP(t *testing.T) {
	for _, in := range []string{"1.2.3.4", "[2001:db8::1]", " 8.8.8.8 ", "fe80::1%eth0"} {
		a, ok := NormalizeIP(in)
		if !ok || a.Zone() != "" {
			t.Errorf("NormalizeIP(%q) = %v,%v", in, a, ok)
		}
	}
	for _, in := range []string{"", "example.com", "300.1.1.1", "1.2.3"} {
		if _, ok := NormalizeIP(in); ok {
			t.Errorf("NormalizeIP(%q) should fail", in)
		}
	}
}

func TestReverseDNSName(t *testing.T) {
	if got := ReverseDNSName(netip.MustParseAddr("1.2.3.4")); got != "4.3.2.1" {
		t.Errorf("v4: %q", got)
	}
	if got := ReverseDNSName(netip.MustParseAddr("0.0.0.0")); got != "0.0.0.0" {
		t.Errorf("zero: %q", got)
	}
	got := ReverseDNSName(netip.MustParseAddr("2001:db8::1"))
	want := "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2"
	if got != want {
		t.Errorf("v6: %q want %q", got, want)
	}
}

func TestPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "1.1.1.1": true, "10.0.0.1": false, "192.168.1.1": false,
		"127.0.0.1": false, "172.16.0.1": false, "169.254.1.1": false, "::1": false,
		"fc00::1": false, "2606:4700::1111": true,
	} {
		if got := PublicIP(netip.MustParseAddr(ip)); got != want {
			t.Errorf("PublicIP(%s) = %v want %v", ip, got, want)
		}
	}
}

func TestMailLists(t *testing.T) {
	if !IsFreeMail("GMAIL.com") || IsFreeMail("example.com") {
		t.Error("IsFreeMail")
	}
	if !IsDisposable("Mailinator.com") || IsDisposable("gmail.com") {
		t.Error("IsDisposable")
	}
}
