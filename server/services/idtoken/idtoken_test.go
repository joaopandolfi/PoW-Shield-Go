package idtoken

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignVerify(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	path := filepath.Join(t.TempDir(), "id.key.pem")
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	id := uuid.NewString()
	tok, err := s.Sign(Claims{Sub: id, App: "1:1:android:a", Iat: now.Unix(), Exp: now.Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Verify(tok, now)
	if err != nil || c.Sub != id || c.App != "1:1:android:a" {
		t.Fatalf("valid token: %v %+v", err, c)
	}
	if strings.Contains(tok, ":") || len(tok) > 400 {
		t.Fatalf("token must fit the contract payload: len %d", len(tok))
	}

	_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
	other := New(otherKey)
	foreign, _ := other.Sign(Claims{Sub: id, Exp: now.Add(time.Hour).Unix()})
	expired, _ := s.Sign(Claims{Sub: id, Exp: now.Add(-time.Second).Unix()})
	notUUID, _ := s.Sign(Claims{Sub: "admin", Exp: now.Add(time.Hour).Unix()})
	parts := strings.Split(tok, ".")
	tamperedClaims, _ := s.Sign(Claims{Sub: uuid.NewString(), Exp: now.Add(time.Hour).Unix()})
	swapped := parts[0] + "." + strings.Split(tamperedClaims, ".")[1] + "." + parts[2]

	for name, bad := range map[string]string{
		"garbage":        "x.y",
		"other key":      foreign,
		"expired":        expired,
		"sub not uuid":   notUUID,
		"swapped claims": swapped,
		"no signature":   parts[0] + "." + parts[1] + ".",
		"plain uuid":     id,
	} {
		if _, err := s.Verify(bad, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
