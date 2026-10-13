package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/services/contract"
	"pow-shield-go/services/utils"
	"pow-shield-go/web/handler"
	"strconv"
	"time"
)

const contractCachePrefix = "ct:"

var contractCache cache.Cache
var contractVerifier *contract.Verifier

func InitContract() {
	cfg := config.Get().Contract
	if !cfg.Active || contractVerifier != nil {
		return
	}
	contractCache = cache.Get()

	v, err := contract.Load(cfg.PrivateKeyFile, cfg.Passphrase, cfg.MaxAge)
	if err != nil {
		// fail fast: an empty keyring would block every request silently
		panic("loading contract keys: " + err.Error())
	}
	contractVerifier = v
}

// ContractPublicKey is the armored WAF public key handed to attested devices
func ContractPublicKey() string {
	return contractVerifier.PublicKey()
}

// AttestedKey is the cache key marking a client id issued by the attest endpoint
func AttestedKey(id string) string {
	return contractKey("a", id, "")
}

type contractChallenge struct {
	Prefix     string `json:"prefix"`
	Difficulty int    `json:"difficulty"`
}

func contractKey(kind, id, prefix string) string {
	return contractCachePrefix + kind + ":" + id + ":" + prefix
}

// Contract identifies the client by its encrypted contract header and gates it with a PoW challenge.
// The solved challenge travels inside the same header and is reused as token up to CONTRACT_MAX_USES,
// then (or on a wrong solution) the client gets a new challenge with increased difficulty.
func Contract(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get().Contract
		log := logging.Get()

		c, err := contractVerifier.Verify(r.Header.Get(cfg.Header), time.Now())
		if err != nil {
			if log != nil {
				log.Warn("Contract rejected", "ip", handler.IP(r), "reason", err.Error())
			}
			blockRequest(w)
			return
		}

		// binding: the header only authorizes the exact request (method, URI, body) it was made for
		if c.Binding != "" || cfg.RequireBinding {
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				blockRequest(w)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			if c.Binding != contract.RequestHash(r.Method, r.URL.RequestURI(), body) {
				if log != nil {
					log.Warn("Contract rejected", "ip", handler.IP(r), "client", c.ID, "reason", "request binding missing or mismatch")
				}
				blockRequest(w)
				return
			}
		}

		id, prefix, nonceHex := c.ID, c.Prefix, c.Nonce
		blocked, err := contractCache.Get(contractKey("b", id, ""))
		if cacheFailed(w, err) {
			return
		}
		if blocked != nil {
			if log != nil {
				log.Warn("Contract client blocked", "client", id)
			}
			blockRequest(w)
			return
		}

		// with attestation on, only ids issued to devices that passed App Check (Play Integrity / App Attest) get in
		if config.Get().Attest.Active {
			attested, err := contractCache.Get(AttestedKey(id))
			if cacheFailed(w, err) {
				return
			}
			if attested == nil {
				if log != nil {
					log.Warn("Contract rejected", "ip", handler.IP(r), "client", id, "reason", "client id not issued by attest")
				}
				blockRequest(w)
				return
			}
		}

		// requests of this id in the current window: feeds the rate limit and the progressive difficulty
		inWindow := 0
		if cfg.RateRequests > 0 || cfg.DifficultyStep > 0 {
			inWindow, err = contractCache.Incr(contractKey("l", id, ""), 0, 1, cfg.RateWindow)
			if cacheFailed(w, err) {
				return
			}
		}
		if cfg.RateRequests > 0 {
			count := inWindow
			if count > cfg.RateRequests {
				metrics.IncRateLimited()
				if log != nil {
					log.Warn("Contract rate limit exceeded", "client", id, "count", count, "limit", cfg.RateRequests)
				}
				w.Header().Set("Retry-After", strconv.Itoa(int(cfg.RateWindow.Seconds())))
				handler.RespondDefaultError(w, http.StatusTooManyRequests)
				return
			}
		}

		// replay: PGP encryption is randomized, so the same header twice means a captured request was resent.
		// Kept for 2*MaxAge, the whole window in which its timestamp is accepted.
		sum := sha256.Sum256([]byte(r.Header.Get(cfg.Header)))
		replayKey := contractKey("r", hex.EncodeToString(sum[:]), "")
		fresh, err := contractCache.PutIfAbsent(replayKey, 1, 2*cfg.MaxAge)
		if cacheFailed(w, err) {
			return
		}
		if !fresh {
			if log != nil {
				log.Warn("Contract replay detected", "client", id, "ip", handler.IP(r))
			}
			if prefix != "" {
				contractCache.Delete(contractKey("t", id, prefix)) // replayed token is compromised
			}
			issueContractChallenge(w, id, true, inWindow)
			return
		}

		if prefix == "" {
			issueContractChallenge(w, id, false, inWindow)
			return
		}

		tokenKey := contractKey("t", id, prefix)
		uses, existed, err := contractCache.IncrIfExists(tokenKey)
		if cacheFailed(w, err) {
			return
		}
		if existed {
			if uses > cfg.MaxUses {
				contractCache.Delete(tokenKey)
				// only the request that crossed the limit raises difficulty, a parallel burst is punished once
				issueContractChallenge(w, id, uses == cfg.MaxUses+1, inWindow)
				return
			}
			next(w, r)
			return
		}

		// one shot: GetDel burns the prefix, no retries and no double spend across instances
		raw, err := contractCache.GetDel(contractKey("c", id, prefix))
		if cacheFailed(w, err) {
			return
		}
		if raw == nil {
			issueContractChallenge(w, id, false, inWindow)
			return
		}

		nonce, err := hex.DecodeString(nonceHex)
		valid := false
		if err == nil {
			if hash, err := utils.Hash(nonce, prefix); err == nil {
				valid, _ = utils.CheckComplexity(hash, readRateValue(raw))
			}
		}
		if !valid {
			if log != nil {
				log.Warn("Contract challenge failed", "client", id)
			}
			issueContractChallenge(w, id, true, inWindow)
			return
		}

		if cacheFailed(w, contractCache.Put(tokenKey, 1, cfg.TokenTTL)) {
			return
		}
		next(w, r)
	}
}

// cacheFailed answers 500 when the cache errors: fail closed, a request never passes unchecked
func cacheFailed(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if log := logging.Get(); log != nil {
		log.Error("Contract cache error", "error", err.Error())
	}
	handler.RespondDefaultError(w, http.StatusInternalServerError)
	return true
}

// issueContractChallenge answers 401 with a new challenge. harder applies a punishment (kept for
// CONTRACT_PENALTY_SECONDS, can block); inWindow adds CONTRACT_DIFFICULTY_STEP per earlier request of
// the id in the current window (load based, capped at CONTRACT_MAX_DIFFICULTY, never blocks).
func issueContractChallenge(w http.ResponseWriter, id string, harder bool, inWindow int) {
	cfg := config.Get().Contract
	difficultyKey := contractKey("d", id, "")

	base := config.Get().Pow.DefaultPrefixSize
	difficulty := base
	if harder {
		// atomic: simultaneous punishments never lose an increment; the penalty window starts at the first raise
		var err error
		difficulty, err = contractCache.Incr(difficultyKey, base, config.Get().Pow.Punishment, cfg.PenaltyTTL)
		if cacheFailed(w, err) {
			return
		}
		if difficulty > cfg.MaxDifficulty {
			if cacheFailed(w, contractCache.Put(contractKey("b", id, ""), 1, cfg.PenaltyTTL)) {
				return
			}
			if log := logging.Get(); log != nil {
				log.Warn("Contract client reached max difficulty, blocking", "client", id)
			}
			blockRequest(w)
			return
		}
	} else {
		raw, err := contractCache.Get(difficultyKey)
		if cacheFailed(w, err) {
			return
		}
		if raw != nil {
			difficulty = readRateValue(raw)
		}
	}

	if inWindow > 1 && cfg.DifficultyStep > 0 {
		every := max(cfg.DifficultyEvery, 1)
		difficulty = min(difficulty+cfg.DifficultyStep*((inWindow-1)/every), cfg.MaxDifficulty)
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}
	prefix := hex.EncodeToString(buf)

	if cacheFailed(w, contractCache.Put(contractKey("c", id, prefix), difficulty, cfg.ChallengeTTL)) {
		return
	}

	handler.RespondJson(w, contractChallenge{Prefix: prefix, Difficulty: difficulty}, http.StatusUnauthorized)
}
