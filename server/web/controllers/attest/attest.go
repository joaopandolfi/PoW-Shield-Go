// Package attest hands the WAF public key and a client id to devices that passed
// Firebase App Check (Play Integrity on Android, App Attest/DeviceCheck on iOS).
package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/services/appcheck"
	"pow-shield-go/services/idtoken"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
	"time"

	"github.com/google/uuid"
)

// tokenCachePrefix maps an App Check token to the id token it already got
const tokenCachePrefix = "ct:h:"

type response struct {
	Key   string `json:"key"`   // armored WAF public key
	ID    string `json:"id"`    // client uuid (the sub of token)
	Token string `json:"token"` // WAF signed id token, goes in the id field of the contract payload
	Exp   int64  `json:"exp"`   // unix seconds, attest again before it
}

type controller struct {
	verifier *appcheck.Verifier
	cache    cache.Cache
}

func New() controllers.Controller {
	cfg := config.Get().Attest
	return &controller{
		verifier: appcheck.New(cfg.JWKSURL, cfg.ProjectNumber, cfg.AppIDs),
		cache:    cache.Get(),
	}
}

func (c *controller) SetupRouter(s *server.Server) {
	h := c.handshake
	if config.Get().Rate.Active {
		h = middleware.RateLimit(h)
	}
	s.R.HandleFunc(config.Get().Attest.Path, h).Methods("POST")
}

func (c *controller) handshake(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get().Attest
	log := logging.Get()
	now := time.Now()

	raw := r.Header.Get(cfg.Header)
	token, err := c.verifier.Verify(raw, now)
	if errors.Is(err, appcheck.ErrUnavailable) {
		if log != nil {
			log.Error("Attest unavailable", "error", err.Error())
		}
		handler.RespondDefaultError(w, http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		if log != nil {
			log.Warn("Attest rejected", "ip", handler.IP(r), "reason", err.Error())
		}
		metrics.IncBlockedResponses()
		handler.RespondBlock(w)
		return
	}

	signed, err := middleware.IDSigner().Sign(idtoken.Claims{
		Sub: uuid.NewString(),
		App: token.AppID,
		Iat: now.Unix(),
		Exp: now.Add(cfg.IDTTL).Unix(),
	})
	if err != nil {
		c.cacheError(w, err)
		return
	}

	// one id per App Check token: a token lifted from a real device cannot mint ids in a loop.
	// The only state is at issuance; the id token itself is verified without lookups.
	sum := sha256.Sum256([]byte(raw))
	tokenKey := tokenCachePrefix + hex.EncodeToString(sum[:])
	// kept while both are valid, so a repeated handshake never returns an expired id token
	fresh, err := c.cache.PutIfAbsent(tokenKey, signed, min(token.ExpiresAt.Sub(now), cfg.IDTTL))
	if err != nil {
		c.cacheError(w, err)
		return
	}
	if !fresh {
		prev, err := c.cache.Get(tokenKey)
		if err != nil {
			c.cacheError(w, err)
			return
		}
		signed, _ = prev.(string)
	}

	claims, err := middleware.IDSigner().Verify(signed, now)
	if err != nil {
		c.cacheError(w, err)
		return
	}
	issued := response{
		Key:   middleware.ContractPublicKey(),
		ID:    claims.Sub,
		Token: signed,
		Exp:   claims.Exp,
	}
	w.Header().Set("Cache-Control", "no-store")
	handler.RespondJson(w, issued, http.StatusOK)
}

func (c *controller) cacheError(w http.ResponseWriter, err error) {
	if log := logging.Get(); log != nil {
		log.Error("Attest cache error", "error", err.Error())
	}
	handler.RespondDefaultError(w, http.StatusInternalServerError)
}
