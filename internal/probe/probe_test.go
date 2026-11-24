package probe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestSecurityHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Security-Policy", "default-src 'self'")
	h.Set("X-Frame-Options", "DENY")
	present, missing := SecurityHeaders(h, true)
	if len(present) != 2 || len(missing) != 4 {
		t.Errorf("https: present=%v missing=%v", present, missing)
	}
	_, missing = SecurityHeaders(h, false)
	for _, m := range missing {
		if m == "Strict-Transport-Security" {
			t.Error("HSTS is not expected over plain HTTP")
		}
	}
}

func TestCertInfo(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(255), Subject: pkix.Name{CommonName: "example.com"},
		Issuer: pkix.Name{CommonName: "ignored"}, DNSNames: []string{"b.example.com", "example.com"},
		IPAddresses: []net.IP{net.ParseIP("10.0.0.1")},
		NotBefore:   time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	got := certInfo(c, tls.VersionTLS13)
	if got.Subject != "example.com" || got.TLSVersion != "TLS 1.3" || got.Serial != "ff" {
		t.Errorf("%+v", got)
	}
	if !got.SelfSigned {
		t.Error("cert signed by itself with identical subject should be flagged")
	}
	if len(got.SANs) != 3 || got.SANs[0] != "10.0.0.1" {
		t.Errorf("SANs should include IPs and be sorted: %v", got.SANs)
	}
}
