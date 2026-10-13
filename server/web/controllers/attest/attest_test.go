package attest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/services/appcheck/appchecktest"
	"pow-shield-go/services/utils"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func TestHandshake(t *testing.T) {
	firebase := appchecktest.NewFirebase(t)

	waf, err := openpgp.NewEntity("waf", "", "waf@test", nil)
	if err != nil {
		t.Fatal(err)
	}
	var priv bytes.Buffer
	aw, _ := armor.Encode(&priv, openpgp.PrivateKeyType, nil)
	waf.SerializePrivate(aw, nil)
	aw.Close()
	keyFile := filepath.Join(t.TempDir(), "waf.asc")
	os.WriteFile(keyFile, priv.Bytes(), 0o600)

	cfg := config.Config{}
	cfg.Reply.BlockStatus = http.StatusNotAcceptable
	cfg.Pow.DefaultPrefixSize = 8
	cfg.Pow.Punishment = 2
	cfg.Contract.Active = true
	cfg.Contract.PrivateKeyFile = keyFile
	cfg.Contract.Header = "X-Ctx"
	cfg.Contract.MaxAge = 30 * time.Second
	cfg.Contract.MaxUses = 10
	cfg.Contract.TokenTTL = time.Minute
	cfg.Contract.ChallengeTTL = time.Minute
	cfg.Contract.MaxDifficulty = 20
	cfg.Contract.PenaltyTTL = time.Minute
	cfg.Attest.Active = true
	cfg.Attest.Path = "/v1/handshake"
	cfg.Attest.Header = "X-Firebase-AppCheck"
	cfg.Attest.ProjectNumber = firebase.Project
	cfg.Attest.AppIDs = []string{firebase.App}
	cfg.Attest.JWKSURL = firebase.Server.URL
	cfg.Attest.IDTTL = time.Hour
	config.Inject(cfg)
	cache.Initialize(context.Background(), time.Minute)
	middleware.InitContract()

	r := mux.NewRouter()
	New().SetupRouter(&server.Server{R: r})
	handshake := func(token string) (int, response) {
		req := httptest.NewRequest("POST", "/v1/handshake", nil)
		if token != "" {
			req.Header.Set("X-Firebase-AppCheck", token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp response
		json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	expect := func(step string, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, want %d", step, got, want)
		}
	}

	code, _ := handshake("")
	expect("no app check token", code, 406)
	code, _ = handshake(firebase.Sign(t, func(_, c map[string]any) { c["sub"] = "1:999:android:other" }))
	expect("token from another app", code, 406)
	code, _ = handshake(firebase.Sign(t, func(_, c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }))
	expect("expired token", code, 406)

	token := firebase.Sign(t, nil)
	code, first := handshake(token)
	expect("attested device", code, 200)
	if _, err := uuid.Parse(first.ID); err != nil || !strings.Contains(first.Key, "BEGIN PGP PUBLIC KEY BLOCK") || first.Exp <= time.Now().Unix() {
		t.Fatalf("handshake response %+v", first)
	}
	code, again := handshake(token)
	expect("same token again", code, 200)
	if again.ID != first.ID || again.Exp != first.Exp {
		t.Fatalf("same App Check token must return the same id: %s vs %s", again.ID, first.ID)
	}
	code, second := handshake(firebase.Sign(t, func(_, c map[string]any) { c["iat"] = time.Now().Unix() + 1 }))
	expect("new token", code, 200)
	if second.ID == first.ID {
		t.Fatal("a new App Check token must get a new id")
	}

	// the contract only accepts ids issued here, encrypted with the key handed out
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(first.Key))
	if err != nil {
		t.Fatal(err)
	}
	seal := func(id string) string {
		var out bytes.Buffer
		w, _ := openpgp.Encrypt(&out, keyring, nil, nil, nil)
		fmt.Fprintf(w, "1:%s:%d", id, time.Now().Unix())
		w.Close()
		return utils.ToBase64(out.Bytes())
	}
	contract := middleware.Contract(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	send := func(id string) int {
		req := httptest.NewRequest("GET", "/api", nil)
		req.Header.Set("X-Ctx", seal(id))
		w := httptest.NewRecorder()
		contract(w, req)
		return w.Code
	}
	expect("attested id reaches the challenge", send(first.ID), 401)
	expect("self-made uuid", send(uuid.NewString()), 406)
}
