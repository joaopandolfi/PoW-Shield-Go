// Package appchecktest fakes the Firebase App Check issuer for tests.
package appchecktest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Firebase is a fake App Check issuer: a JWKS endpoint plus a signer, for tests in other packages too
type Firebase struct {
	Server  *httptest.Server
	Project string
	App     string
	key     *rsa.PrivateKey
	kid     string
	Fetches atomic.Int32
}

func NewFirebase(t *testing.T) *Firebase {
	return NewFirebaseAt(t, "")
}

// NewFirebaseAt serves the JWKS on addr (e.g. ":18080", reachable from docker); "" picks a free loopback port
func NewFirebaseAt(t *testing.T, addr string) *Firebase {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &Firebase{Project: "123456789", App: "1:123456789:android:abc", key: key, kid: "k1"}
	f.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.Fetches.Add(1)
		b64 := base64.RawURLEncoding.EncodeToString
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": f.kid,
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	if addr != "" {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		f.Server.Listener.Close()
		f.Server.Listener = l
	}
	f.Server.Start()
	t.Cleanup(f.Server.Close)
	return f
}

// Sign builds a token; mutate tweaks header and claims before signing
func (f *Firebase) Sign(t *testing.T, mutate func(header, claims map[string]any)) string {
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": f.kid}
	claims := map[string]any{
		"iss": "https://firebaseappcheck.googleapis.com/" + f.Project,
		"sub": f.App,
		"aud": []string{"projects/" + f.Project, "projects/my-project-id"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	if mutate != nil {
		mutate(header, claims)
	}
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signing := enc(header) + "." + enc(claims)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}
