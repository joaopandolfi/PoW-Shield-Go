//go:build e2e

// End-to-end contract test against the docker stack started by e2e/run.sh:
// two WAF instances (a, b) sharing redis, proxying to http-echo.
package e2e

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"pow-shield-go/services/appcheck/appchecktest"
	"pow-shield-go/services/contract"
	"pow-shield-go/services/utils"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
)

const header = "X-Request-Context"

type attestResponse struct {
	Key   string `json:"key"`
	ID    string `json:"id"`
	Token string `json:"token"`
	Exp   int64  `json:"exp"`
}

type challenge struct {
	Prefix     string `json:"prefix"`
	Difficulty int    `json:"difficulty"`
}

type client struct {
	t   *testing.T
	waf openpgp.EntityList
	id  string
}

// seal is what the front does: base64(gpg_encrypt(payload)) with the WAF public key
func (c *client) seal(payload string) string {
	var out bytes.Buffer
	w, err := openpgp.Encrypt(&out, c.waf, nil, nil, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	w.Write([]byte(payload))
	w.Close()
	return utils.ToBase64(out.Bytes())
}

func (c *client) contract(solution string) string {
	payload := fmt.Sprintf("1:%s:%d", c.id, time.Now().Unix())
	if solution != "" {
		payload += ":" + solution
	}
	return c.seal(payload)
}

// bound is a v2 payload: the header only authorizes this method, URI and body
func (c *client) bound(method, uri, body, solution string) string {
	payload := fmt.Sprintf("2:%s:%d:%s", c.id, time.Now().Unix(), contract.RequestHash(method, uri, []byte(body)))
	if solution != "" {
		payload += ":" + solution
	}
	return c.seal(payload)
}

func (c *client) send(url, value string) (int, string, challenge) {
	return c.sendReq("GET", url, "/api/anything", "", value)
}

func (c *client) sendReq(method, url, uri, reqBody, value string) (int, string, challenge) {
	req, _ := http.NewRequest(method, url+uri, strings.NewReader(reqBody))
	if value != "" {
		req.Header.Set(header, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ch challenge
	json.Unmarshal(body, &ch)
	return resp.StatusCode, string(body), ch
}

func solve(ch challenge) string {
	nonce := make([]byte, 16)
	for i := uint64(0); ; i++ {
		binary.BigEndian.PutUint64(nonce[8:], i)
		hash, _ := utils.Hash(nonce, ch.Prefix)
		if ok, _ := utils.CheckComplexity(hash, ch.Difficulty); ok {
			return ch.Prefix + ":" + hex.EncodeToString(nonce)
		}
	}
}

func TestContractE2E(t *testing.T) {
	urls := strings.Split(os.Getenv("E2E_URLS"), ",")
	if len(urls) != 2 {
		t.Fatal("E2E_URLS must list the two WAF instances")
	}
	a, b := urls[0], urls[1]

	step := func(name string, got, want int, body string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, want %d (body %q)", name, got, want, body)
		}
		t.Logf("ok  %-45s %d", name, got)
	}

	// attestation: the device gets the WAF key and its id from the handshake, after App Check
	firebase := appchecktest.NewFirebaseAt(t, ":18080")
	firebase.Project, firebase.App = "123456789", "1:123456789:android:abc"
	handshake := func(url, token string) (int, string, attestResponse) {
		req, _ := http.NewRequest("POST", url+"/v1/handshake", nil)
		if token != "" {
			req.Header.Set("X-Firebase-AppCheck", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var r attestResponse
		json.Unmarshal(body, &r)
		return resp.StatusCode, string(body), r
	}
	attested := func(url string) *client {
		t.Helper()
		code, body, r := handshake(url, firebase.Sign(t, func(_, c map[string]any) { c["jti"] = uuid.NewString() }))
		if code != 200 {
			t.Fatalf("handshake: status %d (body %q)", code, body)
		}
		waf, err := openpgp.ReadArmoredKeyRing(strings.NewReader(r.Key))
		if err != nil {
			t.Fatal(err)
		}
		return &client{t: t, waf: waf, id: r.Token} // the id field of the payload carries the id token
	}

	code, body, _ := handshake(a, "")
	step("handshake without App Check token", code, 406, body)
	code, body, _ = handshake(b, firebase.Sign(t, func(_, c map[string]any) { c["sub"] = "1:123456789:android:other" }))
	step("handshake with token of another app", code, 406, body)
	c := attested(a)
	t.Logf("ok  %-45s %d bytes", "handshake issued id token", len(c.id))

	forger := &client{t: t, waf: c.waf, id: uuid.NewString()}
	code, body, _ = forger.send(b, forger.contract(""))
	step("self-made uuid with the right key", code, 406, body)
	wantDifficulty := func(name string, ch challenge, want int) {
		t.Helper()
		if ch.Difficulty != want || ch.Prefix == "" {
			t.Fatalf("%s: challenge %+v, want difficulty %d", name, ch, want)
		}
	}

	// rejections
	code, body, _ = c.send(a, "")
	step("no header", code, 406, body)
	code, body, _ = c.send(a, utils.ToBase64([]byte("1:"+c.id+":1")))
	step("plaintext header", code, 406, body)
	code, body, _ = c.send(a, c.seal(fmt.Sprintf("1:%s:%d", c.id, time.Now().Add(-5*time.Minute).Unix())))
	step("stale timestamp", code, 406, body)
	code, body, _ = c.send(a, c.seal(fmt.Sprintf("9:%s:%d", c.id, time.Now().Unix())))
	step("unknown version", code, 406, body)

	// challenge, solution, proxied to the backend
	code, body, ch := c.send(a, c.contract(""))
	step("contract without challenge", code, 401, body)
	wantDifficulty("first challenge", ch, 12)

	solved := c.contract(solve(ch))
	code, body, _ = c.send(a, solved)
	step("solved challenge reaches backend", code, 200, body)
	if !strings.Contains(body, "protected backend") {
		t.Fatalf("backend body %q", body)
	}

	// replay across instances: b knows a already saw this header through redis
	code, body, ch = c.send(b, solved)
	step("replay on other instance", code, 401, body)
	wantDifficulty("after replay", ch, 14)

	// token shared between instances, limited to CONTRACT_MAX_USES=3
	token := solve(ch)
	code, body, _ = c.send(b, c.contract(token))
	step("solve on b (use 1)", code, 200, body)
	code, body, _ = c.send(a, c.contract(token))
	step("same token on a (use 2)", code, 200, body)
	code, body, _ = c.send(b, c.contract(token))
	step("same token on b (use 3)", code, 200, body)
	code, body, ch = c.send(a, c.contract(token))
	step("token exhausted", code, 401, body)
	wantDifficulty("after exhaustion", ch, 16)

	// wrong solutions climb to the block
	code, body, ch = c.send(a, c.contract(ch.Prefix+":00"))
	step("wrong solution", code, 401, body)
	wantDifficulty("after wrong solution", ch, 18)
	code, body, _ = c.send(b, c.contract(ch.Prefix+":00"))
	step("wrong solution past max difficulty", code, 406, body)
	code, body, _ = c.send(a, c.contract(""))
	step("blocked on the other instance", code, 406, body)

	// request binding (v2) through the real proxy
	bc := attested(b)
	uri, payload := "/api/orders?page=2", `{"item":42}`
	code, body, ch = bc.sendReq("POST", a, uri, payload, bc.bound("POST", uri, payload, ""))
	step("v2 bound, no challenge", code, 401, body)
	sol := solve(ch)
	code, body, _ = bc.sendReq("POST", b, uri, payload, bc.bound("POST", uri, payload, sol))
	step("v2 bound request reaches backend", code, 200, body)
	code, body, _ = bc.sendReq("POST", a, uri, `{"item":999}`, bc.bound("POST", uri, payload, sol))
	step("v2 body tampered", code, 406, body)
	code, body, _ = bc.sendReq("POST", a, "/api/admin", payload, bc.bound("POST", uri, payload, sol))
	step("v2 path tampered", code, 406, body)

	// rate limit per uuid (CONTRACT_RATE_LIMIT_REQUESTS=20), shared by both instances
	rl := attested(a)
	for i := 1; i <= 20; i++ {
		if code, body, _ = rl.send(urls[i%2], rl.contract("")); code != 401 {
			t.Fatalf("uuid rate limit request %d: status %d, want 401 (body %q)", i, code, body)
		}
	}
	code, body, _ = rl.send(a, rl.contract(""))
	step("uuid over rate limit (21st, across instances)", code, 429, body)

	// another client is unaffected
	other := attested(b)
	code, body, ch = other.send(b, other.contract(""))
	step("other client gets base difficulty", code, 401, body)
	wantDifficulty("other client", ch, 12)
}
