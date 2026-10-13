package appcheck_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"pow-shield-go/services/appcheck"
	"pow-shield-go/services/appcheck/appchecktest"
	"strings"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	f := appchecktest.NewFirebase(t)
	v := appcheck.New(f.Server.URL, f.Project, []string{f.App})
	now := time.Now()

	tok, err := v.Verify(f.Sign(t, nil), now)
	if err != nil || tok.AppID != f.App {
		t.Fatalf("valid token: %v %+v", err, tok)
	}

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	forged := f.Sign(t, nil)
	digest := sha256.Sum256([]byte(forged[:strings.LastIndex(forged, ".")]))
	badSig, _ := rsa.SignPKCS1v15(rand.Reader, other, crypto.SHA256, digest[:])
	forged = forged[:strings.LastIndex(forged, ".")+1] + base64.RawURLEncoding.EncodeToString(badSig)

	unsigned := f.Sign(t, func(h, _ map[string]any) { h["alg"] = "none" })
	cases := map[string]string{
		"garbage":       "a.b",
		"alg none":      unsigned[:strings.LastIndex(unsigned, ".")+1],
		"alg HS256":     f.Sign(t, func(h, _ map[string]any) { h["alg"] = "HS256" }),
		"wrong key":     forged,
		"unknown kid":   f.Sign(t, func(h, _ map[string]any) { h["kid"] = "nope" }),
		"expired":       f.Sign(t, func(_, c map[string]any) { c["exp"] = now.Add(-time.Second).Unix() }),
		"wrong issuer":  f.Sign(t, func(_, c map[string]any) { c["iss"] = "https://evil.example/123456789" }),
		"wrong project": f.Sign(t, func(_, c map[string]any) { c["aud"] = []string{"projects/999"} }),
		"other app":     f.Sign(t, func(_, c map[string]any) { c["sub"] = "1:123456789:ios:zzz" }),
	}
	for name, raw := range cases {
		if _, err := v.Verify(raw, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// unknown kids must not trigger a JWKS fetch per request
	if f.Fetches.Load() != 1 {
		t.Fatalf("jwks fetched %d times, want 1", f.Fetches.Load())
	}

	f.Server.Close()
	stale := appcheck.New(f.Server.URL, f.Project, nil)
	if _, err := stale.Verify(f.Sign(t, nil), now); err == nil || !strings.Contains(err.Error(), appcheck.ErrUnavailable.Error()) {
		t.Fatalf("jwks down: %v, want ErrUnavailable", err)
	}
}
