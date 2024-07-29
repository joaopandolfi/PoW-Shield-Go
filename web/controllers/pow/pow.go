package pow

import (
	"context"
	"log"
	"net/http"
	"pow-shield-go/services/pow"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
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
	w.Header().Set("Access-Control-Allow-Origin", "*")

	prefix, err := c.generator.Problem(context.Background())
	if err != nil {
		log.Println("[ERROR] Generating prefix", err.Error())
		handler.RespondDefaultError(w, http.StatusInternalServerError)
		return
	}

	handler.RespondJson(w, problemResponsePayload{
		Prefix: prefix,
	}, http.StatusOK)
}

// verify - verify PoW challenge
func (c *controller) verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, true, http.StatusOK)
}
