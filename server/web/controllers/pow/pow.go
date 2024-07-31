package pow

import (
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

const sessionKeyID = "session:"

type controller struct {
	s                    *server.Server
	verifier             pow.Verifier
	generator            pow.Generator
	cache                cache.Cache
	ipTollerance         int
	defaultCacheDuration time.Duration
}

// New controller
func New(generator pow.Generator, verifier pow.Verifier) controllers.Controller {
	return &controller{
		s:            nil,
		generator:    generator,
		verifier:     verifier,
		cache:        cache.Get(),
		ipTollerance: config.Get().Pow.IPTollerance,
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

func (c *controller) getSession(r *http.Request) *domain.Session {
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
		reqSessionID := c.sessionIDFromRequest(r)
		sessionID := reqSessionID
		reuseCount := 1
		sessionCount, _ := c.cache.Get(reqSessionID)
		if sessionCount != nil {
			if count, ok := sessionCount.(int); ok {
				if count > c.ipTollerance {
					sessionID = uuid.New().String()
				}
				reuseCount = count + 1
			}
		}
		c.cache.Put(reqSessionID, reuseCount, c.defaultCacheDuration)
		session = domain.NewSession(sessionID)
	}

	return session
}

// challenge - get PoW challenge
func (c *controller) challenge(w http.ResponseWriter, r *http.Request) {
	session := c.getSession(r)

	challenge, err := c.generator.Problem(r.Context(), session)
	if err != nil {
		log.Println("[!][ERROR][challenge] Generating prefix", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	if config.Get().Pow.UseSession {
		handler.SetSession(w, r, session)
	}

	var payload problemResponsePayload

	payload.FromDomain(*challenge)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, payload, http.StatusOK)
}

// verify - verify PoW challenge
func (c *controller) verify(w http.ResponseWriter, r *http.Request) {
	session := c.getSession(r)

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

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	handler.RespondJson(w, map[string]string{
		"token": session.PublicWrap(),
	}, http.StatusOK)
}
