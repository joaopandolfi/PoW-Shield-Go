package pow

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pow-shield-go/models/domain"
	"pow-shield-go/services/pow"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"

	"github.com/go-playground/validator"
)

// --- PoW ---

type controller struct {
	s         *server.Server
	verifier  pow.Verifier
	generator pow.Generator
}

// New controller
func New(generator pow.Generator, verifier pow.Verifier) controllers.Controller {
	return &controller{
		s:         nil,
		generator: generator,
		verifier:  verifier,
	}
}

// challenge - get PoW challenge
func (c *controller) challenge(w http.ResponseWriter, r *http.Request) {
	challenge, err := c.generator.Problem(r.Context())
	if err != nil {
		log.Println("[ERROR][challenge] Generating prefix", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	var payload problemResponsePayload

	payload.FromDomain(*challenge)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, payload, http.StatusOK)
}

// verify - verify PoW challenge
func (c *controller) verify(w http.ResponseWriter, r *http.Request) {

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("[ERROR][verify] unrap body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var payload verifyChallengePayload

	err = json.Unmarshal(b, &payload)
	if err != nil {
		log.Println("[ERROR][verify] unmarshal body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	err = validator.New().Struct(payload)
	if err != nil {
		log.Println("[ERROR][verify] validating body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	data, err := hex.DecodeString(payload.Buffer)
	if err != nil {
		log.Println("[ERROR][verify] decoding hex", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	success, err := c.verifier.Verify(r.Context(), data, payload.Difficulty, payload.Prefix)
	if err != nil {
		log.Println("[ERROR][verify] INVALID NONCE:", err.Error())
		handler.RespondDefaultError(w, http.StatusNotAcceptable)
		return
	}

	err = handler.SetSession(w, r, &domain.Session{
		Authorized: success,
		Prefix:     payload.Prefix,
		Buffer:     payload.Buffer,
		Difficulty: payload.Difficulty,
	})

	if err != nil {
		log.Println("[ERROR][verify] setting session", err.Error())
		handler.RespondDefaultError(w, http.StatusConflict)
		return
	}

	handler.RespondJson(w, true, http.StatusOK)
}
