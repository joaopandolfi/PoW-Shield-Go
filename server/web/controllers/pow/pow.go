package pow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/models/domain"
	"pow-shield-go/services/pow"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
	"time"

	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// --- PoW ---

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

// New controller
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

	return handler.SetTempSessionValue(w, r, temp.SessionID, string(payload), int(maxAge))
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

	raw, ok := handler.GetTempSessionValue(r, sessionID)
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

	handler.CleanTempSessions(w, r)
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
		session = handler.GetSession(r)
	}

	if config.Get().Pow.UseHeader {
		wrappedSession := handler.PowHeader(r)
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

// challenge - get PoW challenge
func (c *controller) challenge(w http.ResponseWriter, r *http.Request) {
	session := c.getSession(r, 1)

	challenge, err := c.generator.Problem(r.Context(), session)
	if err != nil {
		log.Println("[!][ERROR][challenge] Generating prefix", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	csrfToken, err := c.generateCSRFToken()
	if err != nil {
		log.Println("[!][ERROR][challenge] Generating CSRF token", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	err = c.storeTempChallenge(w, r, tempChallengeSession{
		SessionID: session.ID,
		Prefix:    challenge.Prefix,
		CSRFToken: csrfToken,
	})
	if err != nil {
		log.Println("[!][ERROR][challenge] Persisting temporary challenge", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	if config.Get().Pow.UseSession {
		handler.SetSession(w, r, session)
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
	handler.RespondJson(w, payload, http.StatusOK)
}

// verify - verify PoW challenge
func (c *controller) verify(w http.ResponseWriter, r *http.Request) {
	session := c.getSession(r, 0)

	csrfCookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		log.Println("[!][ERROR][verify] CSRF token cookie not found")
		handler.RespondDefaultError(w, http.StatusForbidden)
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
		log.Println("[!][ERROR][verify] Loading temporary challenge", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}
	if tempChallenge == nil {
		log.Println("[!][ERROR][verify] Temporary challenge not found")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}
	defer c.burnTempChallenge(w, r, tempChallenge)

	if tempChallenge.SessionID != session.ID {
		log.Println("[!][ERROR][verify] Temporary challenge session mismatch")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	if tempChallenge.CSRFToken != requestCsrf || tempChallenge.CSRFToken != csrfCookie.Value {
		log.Println("[!][ERROR][verify] CSRF token mismatch")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("[!][ERROR][verify] unrap body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}
	var payload verifyChallengePayload

	err = json.Unmarshal(b, &payload)
	if err != nil {
		log.Println("[!][ERROR][verify] unmarshal body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	err = validator.New().Struct(payload)
	if err != nil {
		log.Println("[!][ERROR][verify] validating body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	data, err := hex.DecodeString(payload.Buffer)
	if err != nil {
		log.Println("[!][ERROR][verify] decoding hex", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	if payload.Prefix != tempChallenge.Prefix {
		log.Println("[!][ERROR][verify] Prefix mismatch with temporary challenge")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	success, err := c.verifier.Verify(r.Context(), session, data, payload.Difficulty, payload.Prefix)
	if err != nil {
		log.Println("[!][ERROR][verify] INVALID NONCE:", err.Error())
		handler.RespondDefaultError(w, http.StatusNotAcceptable)
		return
	}

	session.RegisterNewChallenge(success, payload.Prefix, payload.Buffer)

	if config.Get().Pow.UseCookie {
		handler.SetCookie(w, session.ToCookie())
	}
	if config.Get().Pow.UseSession {
		handler.SetSession(w, r, session)
	}
	c.cleanCSRFCookie(w)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	handler.RespondJson(w, map[string]string{
		"token": session.PublicWrap(),
	}, http.StatusOK)
}
