// Package openpgp reads just enough of an OpenPGP public key (RFC 4880 / 9580)
// to say whose it is: fingerprint, algorithm, creation time and the user IDs
// (names and e-mail addresses) it carries. It verifies nothing and never
// handles private keys.
package openpgp

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Key is the primary public key and its user IDs.
type Key struct {
	Fingerprint string // upper-case hex, v4 keys only
	Algorithm   string // "RSA 3072", "Ed25519", …
	Created     time.Time
	UserIDs     []string
	Version     int
}

// Identity is a parsed user ID.
type Identity struct{ Name, Email, Comment string }

// Dearmor decodes an ASCII-armored block; binary input is returned as is.
func Dearmor(b []byte) ([]byte, error) {
	if len(b) > 0 && b[0]&0x80 != 0 {
		return b, nil
	}
	s := string(b)
	i := strings.Index(s, "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	if i < 0 {
		return nil, errors.New("not an OpenPGP public key")
	}
	s = s[i+len("-----BEGIN PGP PUBLIC KEY BLOCK-----"):]
	if j := strings.Index(s, "-----END PGP"); j >= 0 {
		s = s[:j]
	}
	var body strings.Builder
	state := 0 // 0 start, 1 armor headers, 2 base64 body
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case state == 0 && line == "":
		case state == 0 && strings.Contains(line, ": "):
			state = 1
		case state == 1 && line == "":
			state = 2
		case state == 1:
		case strings.HasPrefix(line, "="): // CRC24
		case line != "":
			state = 2
			body.WriteString(line)
		}
	}
	return base64.StdEncoding.DecodeString(body.String())
}

var algoNames = map[byte]string{1: "RSA", 2: "RSA", 3: "RSA", 16: "ElGamal", 17: "DSA", 18: "ECDH", 19: "ECDSA", 22: "EdDSA", 25: "X25519", 27: "Ed25519", 28: "Ed448"}

// Parse reads the first primary key in data (binary packets) and the user IDs
// that follow it.
func Parse(data []byte) (*Key, error) {
	k := &Key{}
	r := data
	seenKey := false
	for len(r) > 0 {
		tag, body, rest, err := nextPacket(r)
		if err != nil {
			break
		}
		r = rest
		switch tag {
		case 6: // public key
			if seenKey {
				return finish(k)
			}
			seenKey = true
			if err := parseKey(k, body); err != nil {
				return nil, err
			}
		case 13: // user id
			if seenKey && len(k.UserIDs) < 20 {
				k.UserIDs = append(k.UserIDs, string(body))
			}
		case 14: // first subkey: user ids come before it for the primary key
		}
	}
	return finish(k)
}

func finish(k *Key) (*Key, error) {
	if k.Created.IsZero() {
		return nil, errors.New("no public key packet")
	}
	return k, nil
}

func parseKey(k *Key, body []byte) error {
	if len(body) < 6 {
		return errors.New("short key packet")
	}
	k.Version = int(body[0])
	switch body[0] {
	case 4:
		k.Created = time.Unix(int64(binary.BigEndian.Uint32(body[1:5])), 0).UTC()
		algo := body[5]
		k.Algorithm = algoNames[algo]
		if k.Algorithm == "" {
			k.Algorithm = fmt.Sprintf("algorithm %d", algo)
		}
		if algo <= 3 && len(body) >= 8 { // RSA: first MPI is the modulus, prefixed by its bit length
			k.Algorithm = fmt.Sprintf("RSA %d", binary.BigEndian.Uint16(body[6:8]))
		}
		h := sha1.New()
		h.Write([]byte{0x99, byte(len(body) >> 8), byte(len(body))})
		h.Write(body)
		k.Fingerprint = strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	case 5, 6:
		k.Created = time.Unix(int64(binary.BigEndian.Uint32(body[1:5])), 0).UTC()
		k.Algorithm = algoNames[body[5]]
		if k.Algorithm == "" {
			k.Algorithm = fmt.Sprintf("algorithm %d", body[5])
		}
	default:
		return fmt.Errorf("unsupported key version %d", body[0])
	}
	return nil
}

// nextPacket splits one packet off the front of b.
func nextPacket(b []byte) (tag byte, body, rest []byte, err error) {
	if len(b) < 2 || b[0]&0x80 == 0 {
		return 0, nil, nil, errors.New("bad packet header")
	}
	var n int
	hdr := 1
	if b[0]&0x40 != 0 { // new format
		tag = b[0] & 0x3f
		switch l := int(b[1]); {
		case l < 192:
			n, hdr = l, 2
		case l < 224:
			if len(b) < 3 {
				return 0, nil, nil, errors.New("short")
			}
			n, hdr = (l-192)<<8+int(b[2])+192, 3
		case l == 255:
			if len(b) < 6 {
				return 0, nil, nil, errors.New("short")
			}
			n, hdr = int(binary.BigEndian.Uint32(b[2:6])), 6
		default:
			return 0, nil, nil, errors.New("partial length not supported")
		}
	} else { // old format
		tag = (b[0] >> 2) & 0x0f
		switch b[0] & 3 {
		case 0:
			n, hdr = int(b[1]), 2
		case 1:
			if len(b) < 3 {
				return 0, nil, nil, errors.New("short")
			}
			n, hdr = int(binary.BigEndian.Uint16(b[1:3])), 3
		case 2:
			if len(b) < 5 {
				return 0, nil, nil, errors.New("short")
			}
			n, hdr = int(binary.BigEndian.Uint32(b[1:5])), 5
		default:
			n, hdr = len(b)-1, 1
		}
	}
	if n < 0 || hdr+n > len(b) {
		return 0, nil, nil, errors.New("truncated packet")
	}
	return tag, b[hdr : hdr+n], b[hdr+n:], nil
}

// ParseUserID splits "Name (comment) <email>" into parts.
func ParseUserID(uid string) Identity {
	uid = strings.TrimSpace(uid)
	if i := strings.Index(uid, "<"); i >= 0 {
		if j := strings.Index(uid[i:], ">"); j > 0 {
			return Identity{Name: strings.TrimSpace(stripComment(uid[:i])), Email: strings.ToLower(strings.TrimSpace(uid[i+1 : i+j])), Comment: comment(uid[:i])}
		}
	}
	if strings.Contains(uid, "@") && !strings.ContainsAny(uid, " \t") {
		return Identity{Email: strings.ToLower(uid)}
	}
	return Identity{Name: strings.TrimSpace(stripComment(uid)), Comment: comment(uid)}
}

func stripComment(s string) string {
	for {
		i := strings.Index(s, "(")
		j := strings.Index(s, ")")
		if i < 0 || j < i {
			return s
		}
		s = s[:i] + s[j+1:]
	}
}

func comment(s string) string {
	i, j := strings.Index(s, "("), strings.LastIndex(s, ")")
	if i >= 0 && j > i {
		return strings.TrimSpace(s[i+1 : j])
	}
	return ""
}

// ZBase32 encodes b with the z-base-32 alphabet used by Web Key Directory.
func ZBase32(b []byte) string {
	const alpha = "ybndrfg8ejkmcpqxot1uwisza345h769"
	var out bytes.Buffer
	var acc uint32
	bits := 0
	for _, c := range b {
		acc = acc<<8 | uint32(c)
		bits += 8
		for bits >= 5 {
			out.WriteByte(alpha[(acc>>(bits-5))&31])
			bits -= 5
		}
	}
	if bits > 0 {
		out.WriteByte(alpha[(acc<<(5-bits))&31])
	}
	return out.String()
}

// WKDHash returns the Web Key Directory hash of an e-mail local part.
func WKDHash(local string) string {
	h := sha1.Sum([]byte(strings.ToLower(local)))
	return ZBase32(h[:])
}
