package openpgp

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestWKDHashSpecVector(t *testing.T) {
	// draft-koch-openpgp-webkey-service: the hash of "Joe.Doe" (case-folded).
	if got := WKDHash("Joe.Doe"); got != "iy9q119eutrkn8s1mk4r39qejnbu3n5q" {
		t.Errorf("WKD hash = %s", got)
	}
}

func packet(tag byte, body []byte) []byte { // new-format header, one-octet length
	return append([]byte{0xC0 | tag, byte(len(body))}, body...)
}

func fixtureKey(t *testing.T) []byte {
	created := time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)
	body := []byte{4, byte(created.Unix() >> 24), byte(created.Unix() >> 16), byte(created.Unix() >> 8), byte(created.Unix()), 1, 0x0C, 0x00, 0xAB, 0xCD, 0x00, 0x11, 0x01, 0x01}
	var k []byte
	k = append(k, packet(6, body)...)
	k = append(k, packet(13, []byte("Jane Q. Public (work) <Jane@Example.org>"))...)
	k = append(k, packet(13, []byte("jane@personal.example"))...)
	k = append(k, packet(14, body)...)
	k = append(k, packet(13, []byte("ignored@sub.example"))...) // after a subkey: not the primary's
	return k
}

func TestParse(t *testing.T) {
	k, err := Parse(fixtureKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if k.Version != 4 || !k.Created.Equal(time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)) || k.Algorithm != "RSA 3072" {
		t.Errorf("%+v", k)
	}
	body := fixtureKey(t)[2 : 2+14]
	sum := sha1.Sum(append([]byte{0x99, 0, 14}, body...))
	if k.Fingerprint != strings.ToUpper(hex.EncodeToString(sum[:])) {
		t.Errorf("fingerprint %s", k.Fingerprint)
	}
	if len(k.UserIDs) != 3 {
		t.Errorf("user ids: %v", k.UserIDs)
	}
}

func TestDearmorAndParse(t *testing.T) {
	raw := fixtureKey(t)
	enc := base64.StdEncoding.EncodeToString(raw)
	armored := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: test\nComment: x\n\n" + enc[:40] + "\n" + enc[40:] + "\n=ABCD\n-----END PGP PUBLIC KEY BLOCK-----\n"
	bin, err := Dearmor([]byte(armored))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(bin); err != nil {
		t.Fatal(err)
	}
	if b, err := Dearmor(raw); err != nil || len(b) != len(raw) {
		t.Error("binary input must pass through")
	}
	if _, err := Dearmor([]byte("<html>nope</html>")); err == nil {
		t.Error("html is not a key")
	}
}

func TestParseUserID(t *testing.T) {
	id := ParseUserID("Jane Q. Public (work) <Jane@Example.org>")
	if id.Name != "Jane Q. Public" || id.Email != "jane@example.org" || id.Comment != "work" {
		t.Errorf("%+v", id)
	}
	if id := ParseUserID("just a name"); id.Name != "just a name" || id.Email != "" {
		t.Errorf("%+v", id)
	}
	if id := ParseUserID("bob@example.org"); id.Email != "bob@example.org" {
		t.Errorf("%+v", id)
	}
}

func TestOldFormatAndTruncation(t *testing.T) {
	body := []byte{4, 0, 0, 0, 1, 22, 0, 0}
	old := append([]byte{0x80 | 6<<2 | 0, byte(len(body))}, body...) // old format, tag 6, 1-byte length
	k, err := Parse(old)
	if err != nil || k.Algorithm != "EdDSA" {
		t.Errorf("old format: %v %+v", err, k)
	}
	if _, err := Parse(old[:3]); err == nil {
		t.Error("truncated key must fail")
	}
}
