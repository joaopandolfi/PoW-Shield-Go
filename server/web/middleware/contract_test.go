package middleware

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/services/contract"
	"pow-shield-go/services/utils"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func sealContract(t *testing.T, waf *openpgp.Entity, payload string) string {
	var out bytes.Buffer
	w, err := openpgp.Encrypt(&out, []*openpgp.Entity{waf}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(payload))
	w.Close()
	return utils.ToBase64(out.Bytes())
}

func writePrivateKey(t *testing.T, path string, e *openpgp.Entity) {
	var buf bytes.Buffer
	aw, _ := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	e.SerializePrivate(aw, nil)
	aw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func solve(prefix string, difficulty int) string {
	nonce := make([]byte, 16)
	for i := uint64(0); ; i++ {
		binary.BigEndian.PutUint64(nonce[8:], i)
		hash, _ := utils.Hash(nonce, prefix)
		if ok, _ := utils.CheckComplexity(hash, difficulty); ok {
			return prefix + ":" + hex.EncodeToString(nonce)
		}
	}
}

func TestContractFlowMemory(t *testing.T) {
	contractFlow(t, config.Config{})
}

// miniredis speaks the redis protocol in process, so this runs the real go-redis client path
func TestContractFlowRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := config.Config{}
	cfg.Cache.Redis.Use = true
	cfg.Cache.Redis.Server = mr.Addr()
	h := contractFlow(t, cfg)

	if err := cache.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if len(mr.Keys()) == 0 {
		t.Fatal("expected contract keys stored in redis")
	}
	if ttl := mr.TTL(mr.Keys()[0]); ttl <= 0 {
		t.Fatalf("redis key without ttl: %s", mr.Keys()[0])
	}

	mr.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, h.request(t))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("redis down: status %d, want 500 (fail closed)", w.Code)
	}
}

type contractHarness struct {
	http.HandlerFunc
	request func(t *testing.T) *http.Request
}

func contractFlow(t *testing.T, cfg config.Config) contractHarness {
	dir := t.TempDir()
	id := uuid.NewString()
	newEntity := func() *openpgp.Entity {
		e, err := openpgp.NewEntity("k", "", "k@test", nil)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	waf, other := newEntity(), newEntity()
	writePrivateKey(t, filepath.Join(dir, "waf.asc"), waf)

	cfg.Reply.BlockStatus = http.StatusNotAcceptable
	cfg.Pow.DefaultPrefixSize = 8
	cfg.Pow.Punishment = 2
	cfg.Contract.Active = true
	cfg.Contract.PrivateKeyFile = filepath.Join(dir, "waf.asc")
	cfg.Contract.Header = "X-Ctx"
	cfg.Contract.MaxAge = 30 * time.Second
	cfg.Contract.MaxUses = 2
	cfg.Contract.TokenTTL = time.Minute
	cfg.Contract.ChallengeTTL = time.Minute
	cfg.Contract.MaxDifficulty = 13
	cfg.Contract.PenaltyTTL = time.Minute
	config.Inject(cfg)
	cache.Initialize(context.Background(), time.Minute)
	contractVerifier = nil
	InitContract()

	h := Contract(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	do := func(header string) (int, contractChallenge) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Ctx", header)
		w := httptest.NewRecorder()
		h(w, r)
		if _, err := contractVerifier.Verify(header, time.Now()); err != nil {
			t.Log("verify:", err)
		}
		var c contractChallenge
		json.Unmarshal(w.Body.Bytes(), &c)
		return w.Code, c
	}
	now := func() string { return strconv.FormatInt(time.Now().Unix(), 10) }
	valid := func(solution string) string {
		payload := "1:" + id + ":" + now()
		if solution != "" {
			payload += ":" + solution
		}
		return sealContract(t, waf, payload)
	}
	expect := func(step string, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, want %d", step, got, want)
		}
	}

	code, _ := do("")
	expect("missing contract", code, 406)
	code, _ = do(utils.ToBase64([]byte("1:" + id + ":" + now())))
	expect("plaintext contract", code, 406)
	code, _ = do(sealContract(t, other, "1:"+id+":"+now()))
	expect("not encrypted to waf", code, 406)
	code, _ = do(sealContract(t, waf, "1:not-a-uuid:"+now()))
	expect("invalid uuid", code, 406)
	code, _ = do(sealContract(t, waf, fmt.Sprintf("1:%s:%d", id, time.Now().Add(-time.Minute).Unix())))
	expect("stale contract", code, 406)
	code, _ = do(sealContract(t, waf, "2:"+id+":"+now()))
	expect("unknown version", code, 406)

	code, ch := do(valid(""))
	expect("no challenge", code, 401)
	if ch.Difficulty != 8 {
		t.Fatalf("difficulty %d, want 8", ch.Difficulty)
	}

	token := solve(ch.Prefix, ch.Difficulty)
	solved := valid(token)
	code, _ = do(solved)
	expect("solved", code, 200)
	code, ch = do(solved)
	expect("replayed header", code, 401)
	if ch.Difficulty != 10 {
		t.Fatalf("difficulty after replay %d, want 10", ch.Difficulty)
	}
	code, _ = do(valid(token))
	expect("token burned by replay", code, 401)

	token = solve(ch.Prefix, ch.Difficulty)
	code, _ = do(valid(token))
	expect("solved again", code, 200)
	code, _ = do(valid(token))
	expect("token reuse", code, 200)
	code, ch = do(valid(token))
	expect("token exhausted", code, 401)
	if ch.Difficulty != 12 {
		t.Fatalf("difficulty after exhaustion %d, want 12", ch.Difficulty)
	}

	code, _ = do(valid(ch.Prefix + ":00"))
	expect("wrong solution goes past max difficulty", code, 406)
	code, _ = do(valid(""))
	expect("blocked client", code, 406)

	// request binding (v2): the header only authorizes the request it was hashed for
	doReq := func(method, uri, body, header string) (int, contractChallenge) {
		r := httptest.NewRequest(method, uri, strings.NewReader(body))
		r.Header.Set("X-Ctx", header)
		w := httptest.NewRecorder()
		h(w, r)
		var c contractChallenge
		json.Unmarshal(w.Body.Bytes(), &c)
		return w.Code, c
	}
	bound := uuid.NewString()
	v2 := func(method, uri, body, solution string) string {
		payload := "2:" + bound + ":" + now() + ":" + contract.RequestHash(method, uri, []byte(body))
		if solution != "" {
			payload += ":" + solution
		}
		return sealContract(t, waf, payload)
	}
	code, ch = doReq("POST", "/api/x?a=1", `{"v":1}`, v2("POST", "/api/x?a=1", `{"v":1}`, ""))
	expect("v2 bound, no challenge", code, 401)
	sol := solve(ch.Prefix, ch.Difficulty)
	code, _ = doReq("POST", "/api/x?a=1", `{"v":1}`, v2("POST", "/api/x?a=1", `{"v":1}`, sol))
	expect("v2 bound request", code, 200)
	code, _ = doReq("POST", "/api/x?a=1", `{"v":2}`, v2("POST", "/api/x?a=1", `{"v":1}`, sol))
	expect("v2 body tampered", code, 406)
	code, _ = doReq("POST", "/api/y?a=1", `{"v":1}`, v2("POST", "/api/x?a=1", `{"v":1}`, sol))
	expect("v2 path tampered", code, 406)
	code, _ = doReq("DELETE", "/api/x?a=1", `{"v":1}`, v2("POST", "/api/x?a=1", `{"v":1}`, sol))
	expect("v2 method tampered", code, 406)
	code, _ = do(sealContract(t, waf, "2:"+bound+":"+now()+":nothex"))
	expect("v2 invalid hash", code, 406)

	// rate limit per uuid and mandatory binding
	cfg.Contract.RateRequests = 2
	cfg.Contract.RateWindow = time.Minute
	cfg.Contract.RequireBinding = true
	config.Inject(cfg)
	code, _ = do(valid(""))
	expect("v1 refused when binding is required", code, 406)
	limited := uuid.NewString()
	for i := 1; i <= 3; i++ {
		payload := "2:" + limited + ":" + now() + ":" + contract.RequestHash("GET", "/", nil)
		code, _ = do(sealContract(t, waf, payload))
		want := 401
		if i == 3 {
			want = 429
		}
		expect(fmt.Sprintf("uuid rate limit request %d", i), code, want)
	}
	cfg.Contract.RateRequests = 0
	cfg.Contract.RequireBinding = false
	config.Inject(cfg)

	// progressive difficulty: +2 every 2 requests of the id in the window, capped at MaxDifficulty (13)
	cfg.Contract.DifficultyStep = 2
	cfg.Contract.DifficultyEvery = 2
	config.Inject(cfg)
	busy := uuid.NewString()
	for i, want := range []int{8, 8, 10, 10, 12, 12, 13, 13} {
		code, ch := do(sealContract(t, waf, "1:"+busy+":"+now()))
		expect(fmt.Sprintf("progressive request %d", i+1), code, 401)
		if ch.Difficulty != want {
			t.Fatalf("progressive request %d: difficulty %d, want %d", i+1, ch.Difficulty, want)
		}
	}
	cfg.Contract.DifficultyStep = 0
	config.Inject(cfg)

	return contractHarness{h, func(t *testing.T) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Ctx", sealContract(t, waf, "1:"+uuid.NewString()+":"+now()))
		return r
	}}
}
