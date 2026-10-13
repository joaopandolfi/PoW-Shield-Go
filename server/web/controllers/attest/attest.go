// Package attest hands the WAF public key and a client id to devices that passed
// Firebase App Check (Play Integrity on Android, App Attest/DeviceCheck on iOS).
package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/services/appcheck"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// tokenCachePrefix maps an App Check token to the id it already got
const tokenCachePrefix = "ct:h:"

type response struct {
	Key string `json:"key"` // armored WAF public key
	ID  string `json:"id"`  // client uuid for the contract header
	Exp int64  `json:"exp"` // unix seconds, attest again before it
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

	// one id per App Check token: a token lifted from a real device cannot mint ids in a loop
	sum := sha256.Sum256([]byte(raw))
	tokenKey := tokenCachePrefix + hex.EncodeToString(sum[:])
	issued := response{ID: uuid.NewString(), Exp: now.Add(cfg.IDTTL).Unix()}

	fresh, err := c.cache.PutIfAbsent(tokenKey, issued.ID+"|"+strconv.FormatInt(issued.Exp, 10), token.ExpiresAt.Sub(now))
	if err != nil {
		c.cacheError(w, err)
		return
	}
	if fresh {
		if err := c.cache.Put(middleware.AttestedKey(issued.ID), token.AppID, cfg.IDTTL); err != nil {
			c.cacheError(w, err)
			return
		}
	} else {
		prev, err := c.cache.Get(tokenKey)
		if err != nil {
			c.cacheError(w, err)
			return
		}
		if issued, err = parseIssued(prev); err != nil {
			c.cacheError(w, err)
			return
		}
	}

	issued.Key = middleware.ContractPublicKey()
	w.Header().Set("Cache-Control", "no-store")
	handler.RespondJson(w, issued, http.StatusOK)
}

func parseIssued(raw interface{}) (response, error) {
	s, _ := raw.(string)
	id, exp, ok := strings.Cut(s, "|")
	n, err := strconv.ParseInt(exp, 10, 64)
	if !ok || err != nil {
		return response{}, fmt.Errorf("issued id entry %q", s)
	}
	return response{ID: id, Exp: n}, nil
}

func (c *controller) cacheError(w http.ResponseWriter, err error) {
	if log := logging.Get(); log != nil {
		log.Error("Attest cache error", "error", err.Error())
	}
	handler.RespondDefaultError(w, http.StatusInternalServerError)
}
