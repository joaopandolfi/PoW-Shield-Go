package pow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/models/domain"
	"pow-shield-go/services/pow"
	"pow-shield-go/web/controllers"
	powHandler "pow-shield-go/web/handler"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
	"time"

	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

const csrfCookieName = "csrf_token"
const csrfHeaderName = "X-CSRF-Token"
const tempSessionCacheKeyPrefix = "pow-temp:"

type tempChallengeSession struct {
	SessionID string `json:"session_id"`
	Prefix    string `json:"prefix"`
	CSRFToken string `json:"csrf_token"`
}

type controller struct {
	s                           *server.Server
	verifier                    pow.Verifier
	generator                   pow.Generator
	cache                       cache.Cache
	ipTollerance                int
	defaultIPTolleranceDuration time.Duration
}

func New(generator pow.Generator, verifier pow.Verifier) controllers.Controller {
	return &controller{
		s:                           nil,
		generator:                   generator,
		verifier:                    verifier,
		cache:                       cache.Get(),
		ipTollerance:                config.Get().Pow.IPTollerance,
		defaultIPTolleranceDuration: time.Duration(config.Get().Pow.IPTolleranceDurationSeconds) * time.Second,
	}
}

func (c *controller) sessionIDFromRequest(r *http.Request) string {
	id := r.Context().Value(middleware.UserID)
	strID, ok := id.(string)
	if !ok {
		return uuid.New().String()
	}
	return strID
}

func (c *controller) generateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (c *controller) maskSessionID(id string) string {
	return pow.CacheKey(id, "")
}

func (c *controller) tempSessionCacheKey(sessionID, prefix string) string {
	if prefix == "" {
		return tempSessionCacheKeyPrefix + sessionID
	}
	return tempSessionCacheKeyPrefix + sessionID + ":" + prefix
}

func (c *controller) storeTempChallenge(w http.ResponseWriter, r *http.Request, temp tempChallengeSession) error {
	payload, err := json.Marshal(temp)
	if err != nil {
		return err
	}

	maxAge := c.defaultIPTolleranceDuration / time.Second
	if maxAge <= 0 {
		maxAge = 1
	}

	if config.Get().Cache.Redis.Use {
		return c.cache.Put(c.tempSessionCacheKey(temp.SessionID, ""), string(payload), c.defaultIPTolleranceDuration)
	}

	return powHandler.SetTempSessionValue(w, r, temp.SessionID, string(payload), int(maxAge))
}

func (c *controller) loadTempChallenge(r *http.Request, sessionID string) (*tempChallengeSession, error) {
	if config.Get().Cache.Redis.Use {
		val, err := c.cache.Get(c.tempSessionCacheKey(sessionID, ""))
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, nil
		}

		strVal, ok := val.(string)
		if !ok {
			return nil, nil
		}

		var temp tempChallengeSession
		if err := json.Unmarshal([]byte(strVal), &temp); err != nil {
			return nil, err
		}
		return &temp, nil
	}

	raw, ok := powHandler.GetTempSessionValue(r, sessionID)
	if !ok {
		return nil, nil
	}

	var temp tempChallengeSession
	if err := json.Unmarshal([]byte(raw), &temp); err != nil {
		return nil, err
	}

	return &temp, nil
}

func (c *controller) burnTempChallenge(w http.ResponseWriter, r *http.Request, temp *tempChallengeSession) {
	if temp == nil {
		return
	}

	if config.Get().Cache.Redis.Use {
		_ = c.cache.Delete(c.tempSessionCacheKey(temp.SessionID, ""))
		return
	}

	powHandler.CleanTempSessions(w, r)
}

func (c *controller) cleanCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   config.Get().Session.Options.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (c *controller) getSession(r *http.Request, reusePunishment int) *domain.Session {
	var session *domain.Session
	if config.Get().Pow.UseSession {
		session = powHandler.GetSession(r)
	}

	if config.Get().Pow.UseHeader {
		wrappedSession := powHandler.PowHeader(r)
		if wrappedSession != "" {
			s := domain.Session{}
			err := s.Unrap(wrappedSession)
			if err == nil {
				session = &s
			}
		}
	}

	if session == nil {
		sessionID := c.sessionIDFromRequest(r)
		reqSessionID := c.maskSessionID(sessionID)
		reuseCount := 1
		sessionCount, _ := c.cache.Get(reqSessionID)
		if sessionCount != nil {
			if count, ok := sessionCount.(int); ok {
				if count < c.ipTollerance {
					sessionID = uuid.New().String()
					reqSessionID = c.maskSessionID(sessionID)
				}
				reuseCount = count + reusePunishment
			}
		}
		c.cache.Put(reqSessionID, reuseCount, c.defaultIPTolleranceDuration)
		session = domain.NewSession(sessionID)
	}

	return session
}

func (c *controller) challenge(w http.ResponseWriter, r *http.Request) {
	session := c.getSession(r, 1)
	log := logging.Get()

	challenge, err := c.generator.Problem(r.Context(), session)
	if err != nil {
		if log != nil {
			log.Error("Error generating PoW challenge", "error", err.Error())
		}
		powHandler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	csrfToken, err := c.generateCSRFToken()
	if err != nil {
		if log != nil {
			log.Error("Error generating CSRF token", "error", err.Error())
		}
		powHandler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	err = c.storeTempChallenge(w, r, tempChallengeSession{
		SessionID: session.ID,
		Prefix:    challenge.Prefix,
		CSRFToken: csrfToken,
	})
	if err != nil {
		if log != nil {
			log.Error("Error persisting temporary challenge", "error", err.Error())
		}
		powHandler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	if config.Get().Pow.UseSession {
		powHandler.SetSession(w, r, session)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.Get().Session.Options.Secure,
		SameSite: http.SameSiteStrictMode,
	})

	var payload problemResponsePayload
	payload.FromDomain(*challenge)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	powHandler.RespondJson(w, payload, http.StatusOK)
}

func (c *controller) verify(w http.ResponseWriter, r *http.Request) {
	log := logging.Get()

	session := c.getSession(r, 0)

	csrfCookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		if log != nil {
			log.Error("CSRF token cookie not found", "session_id", session.ID)
		}
		powHandler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	requestCsrf := r.Header.Get(csrfHeaderName)
	if requestCsrf == "" {
		requestCsrf = r.FormValue("csrf_token")
	}
	if requestCsrf == "" {
		requestCsrf = csrfCookie.Value
	}

	tempChallenge, err := c.loadTempChallenge(r, session.ID)
	if err != nil {
		if log != nil {
			log.Error("Error loading temporary challenge", "error", err.Error(), "session_id", session.ID)
		}
		powHandler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}
	if tempChallenge == nil {
		if log != nil {
			log.Warn("Temporary challenge session expired", "session_id", session.ID)
		}
		powHandler.RespondDefaultError(w, http.StatusForbidden)
		return
	}
	defer c.burnTempChallenge(w, r, tempChallenge)

	if tempChallenge.SessionID != session.ID {
		if log != nil {
			log.Warn("Session ID mismatch", "stored", tempChallenge.SessionID, "current", session.ID)
		}
		powHandler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	if tempChallenge.CSRFToken != requestCsrf || tempChallenge.CSRFToken != csrfCookie.Value {
		if log != nil {
			log.Warn("CSRF token mismatch", "session_id", session.ID)
		}
		powHandler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		if log != nil {
			log.Error("Error reading request body", "error", err.Error())
		}
		powHandler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}
	var payload verifyChallengePayload

	err = json.Unmarshal(b, &payload)
	if err != nil {
		if log != nil {
			log.Error("Error unmarshaling challenge payload", "error", err.Error())
		}
		powHandler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	err = validator.New().Struct(payload)
	if err != nil {
		if log != nil {
			log.Error("Validation failed for challenge payload", "error", err.Error(), "fields", payload)
		}
		powHandler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	data, err := hex.DecodeString(payload.Buffer)
	if err != nil {
		if log != nil {
			log.Error("Error decoding hex buffer", "error", err.Error(), "buffer", payload.Buffer)
		}
		powHandler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	if payload.Prefix != tempChallenge.Prefix {
		if log != nil {
			log.Warn("Prefix mismatch", "provided", payload.Prefix, "expected", tempChallenge.Prefix)
		}
		powHandler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	success, err := c.verifier.Verify(r.Context(), session, data, payload.Difficulty, payload.Prefix)
	if err != nil {
		if log != nil {
			log.Error("INVALID NONCE", "session_id", session.ID, "error", err.Error(), "difficulty", payload.Difficulty)
		}
		powHandler.RespondDefaultError(w, http.StatusNotAcceptable)
		return
	}

	session.RegisterNewChallenge(success, payload.Prefix, payload.Buffer)

	if config.Get().Pow.UseCookie {
		powHandler.SetCookie(w, session.ToCookie())
	}
	if config.Get().Pow.UseSession {
		powHandler.SetSession(w, r, session)
	}
	c.cleanCSRFCookie(w)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	powHandler.RespondJson(w, map[string]string{
		"token": session.PublicWrap(),
	}, http.StatusOK)
}
