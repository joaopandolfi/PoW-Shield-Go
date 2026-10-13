package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/services/contract"
	"pow-shield-go/services/utils"
	"pow-shield-go/web/handler"
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
			issueContractChallenge(w, id, true)
			return
		}

		if prefix == "" {
			issueContractChallenge(w, id, false)
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
				issueContractChallenge(w, id, uses == cfg.MaxUses+1)
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
			issueContractChallenge(w, id, false)
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
			issueContractChallenge(w, id, true)
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

func issueContractChallenge(w http.ResponseWriter, id string, harder bool) {
	cfg := config.Get().Contract
	difficultyKey := contractKey("d", id, "")

	difficulty := config.Get().Pow.DefaultPrefixSize
	raw, err := contractCache.Get(difficultyKey)
	if cacheFailed(w, err) {
		return
	}
	if raw != nil {
		difficulty = readRateValue(raw)
	}

	if harder {
		difficulty += config.Get().Pow.Punishment
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
		if cacheFailed(w, contractCache.Put(difficultyKey, difficulty, cfg.PenaltyTTL)) {
			return
		}
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
