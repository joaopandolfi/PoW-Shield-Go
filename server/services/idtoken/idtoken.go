// Package idtoken issues and verifies the client id token: an EdDSA (Ed25519) JWT signed by the WAF
// at the attest handshake. Its sub is the client uuid; validity is checked from the token alone.
package idtoken

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const issuer = "pow-shield"

// Claims carried by the id token
type Claims struct {
	Iss string `json:"iss"`
	Sub string `json:"sub"` // client uuid
	App string `json:"app"` // Firebase app id that passed App Check
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

type Signer struct {
	key ed25519.PrivateKey
	kid string
}

// Load reads a PEM (PKCS#8) Ed25519 private key
func Load(path string) (*Signer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM block", path)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an Ed25519 key", path)
	}
	return New(key), nil
}

func New(key ed25519.PrivateKey) *Signer {
	sum := sha256.Sum256(key.Public().(ed25519.PublicKey))
	return &Signer{key: key, kid: hex.EncodeToString(sum[:8])}
}

var b64 = base64.RawURLEncoding

func (s *Signer) Sign(c Claims) (string, error) {
	c.Iss = issuer
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": s.kid})
	claims, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signing := b64.EncodeToString(header) + "." + b64.EncodeToString(claims)
	return signing + "." + b64.EncodeToString(ed25519.Sign(s.key, []byte(signing))), nil
}

// Verify checks signature, issuer, expiry and that sub is a uuid
func (s *Signer) Verify(token string, now time.Time) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed id token")
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decode(parts[0], &header); err != nil {
		return nil, fmt.Errorf("id token header: %w", err)
	}
	if header.Alg != "EdDSA" || header.Kid != s.kid {
		return nil, fmt.Errorf("id token alg/kid %q/%q", header.Alg, header.Kid)
	}

	sig, err := b64.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(s.key.Public().(ed25519.PublicKey), []byte(parts[0]+"."+parts[1]), sig) {
		return nil, fmt.Errorf("id token signature")
	}

	var c Claims
	if err := decode(parts[1], &c); err != nil {
		return nil, fmt.Errorf("id token claims: %w", err)
	}
	if c.Iss != issuer {
		return nil, fmt.Errorf("id token issuer %q", c.Iss)
	}
	if now.Unix() >= c.Exp {
		return nil, fmt.Errorf("id token expired")
	}
	if _, err := uuid.Parse(c.Sub); err != nil {
		return nil, fmt.Errorf("id token sub is not a uuid")
	}
	return &c, nil
}

func decode(seg string, out any) error {
	raw, err := b64.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
