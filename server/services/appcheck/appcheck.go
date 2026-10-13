// Package appcheck verifies Firebase App Check tokens: RS256 JWTs signed by Firebase after
// Play Integrity (Android) or App Attest/DeviceCheck (iOS) succeeded on the device.
package appcheck

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const DefaultJWKSURL = "https://firebaseappcheck.googleapis.com/v1/jwks"

const (
	jwksMaxAge       = 6 * time.Hour
	jwksMinRefetch   = time.Minute // unknown kids cannot force a fetch storm
	jwksFetchTimeout = 5 * time.Second
)

// Token is what the WAF uses from a verified App Check token
type Token struct {
	AppID     string
	ExpiresAt time.Time
}

type Verifier struct {
	jwksURL       string
	projectNumber string
	appIDs        []string // empty accepts any app of the project

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	client    *http.Client
}

func New(jwksURL, projectNumber string, appIDs []string) *Verifier {
	return &Verifier{
		jwksURL:       jwksURL,
		projectNumber: projectNumber,
		appIDs:        appIDs,
		client:        &http.Client{Timeout: jwksFetchTimeout},
	}
}

// ErrUnavailable means the token could not be checked (JWKS fetch failed), not that it is invalid
var ErrUnavailable = fmt.Errorf("app check keys unavailable")

func (v *Verifier) Verify(raw string, now time.Time) (*Token, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token")
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	// only RS256: never trust the token to pick its own algorithm
	if header.Alg != "RS256" || header.Typ != "JWT" {
		return nil, fmt.Errorf("unexpected alg/typ %q/%q", header.Alg, header.Typ)
	}

	key, err := v.key(header.Kid, now)
	if err != nil {
		return nil, err
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("signature encoding: %w", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}

	var claims struct {
		Iss string   `json:"iss"`
		Sub string   `json:"sub"`
		Aud []string `json:"aud"`
		Exp int64    `json:"exp"`
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return nil, fmt.Errorf("claims: %w", err)
	}
	if claims.Iss != "https://firebaseappcheck.googleapis.com/"+v.projectNumber {
		return nil, fmt.Errorf("issuer %q", claims.Iss)
	}
	if !slices.Contains(claims.Aud, "projects/"+v.projectNumber) {
		return nil, fmt.Errorf("audience %v", claims.Aud)
	}
	exp := time.Unix(claims.Exp, 0)
	if !now.Before(exp) {
		return nil, fmt.Errorf("expired at %s", exp)
	}
	if claims.Sub == "" || (len(v.appIDs) > 0 && !slices.Contains(v.appIDs, claims.Sub)) {
		return nil, fmt.Errorf("app %q not allowed", claims.Sub)
	}

	return &Token{AppID: claims.Sub, ExpiresAt: exp}, nil
}

func decodeSegment(seg string, out any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// key returns the signing key for kid, refetching the JWKS when stale or when kid is unknown
func (v *Verifier) key(kid string, now time.Time) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	k, ok := v.keys[kid]
	stale := now.Sub(v.fetchedAt) > jwksMaxAge
	if ok && !stale {
		return k, nil
	}
	if !stale && now.Sub(v.fetchedAt) < jwksMinRefetch {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}

	keys, err := v.fetch()
	if err != nil {
		if ok { // keep serving the cached key while Firebase is unreachable
			return k, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	v.keys, v.fetchedAt = keys, now

	if k, ok = keys[kid]; !ok {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	return k, nil
}

func (v *Verifier) fetch() (map[string]*rsa.PublicKey, error) {
	resp, err := v.client.Get(v.jwksURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", resp.StatusCode)
	}

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}

	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks without RSA keys")
	}
	return keys, nil
}
