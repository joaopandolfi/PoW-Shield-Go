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
	"pow-shield-go/services/utils"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
)

const header = "X-Request-Context"

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

func (c *client) send(url, value string) (int, string, challenge) {
	req, _ := http.NewRequest("GET", url+"/api/anything", nil)
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

	raw, err := os.ReadFile(os.Getenv("E2E_PUBKEY"))
	if err != nil {
		t.Fatal(err)
	}
	waf, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, waf: waf, id: uuid.NewString()}

	step := func(name string, got, want int, body string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: status %d, want %d (body %q)", name, got, want, body)
		}
		t.Logf("ok  %-45s %d", name, got)
	}
	wantDifficulty := func(name string, ch challenge, want int) {
		t.Helper()
		if ch.Difficulty != want || ch.Prefix == "" {
			t.Fatalf("%s: challenge %+v, want difficulty %d", name, ch, want)
		}
	}

	// rejections
	code, body, _ := c.send(a, "")
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

	// another client is unaffected
	other := &client{t: t, waf: waf, id: uuid.NewString()}
	code, body, ch = other.send(b, other.contract(""))
	step("other client gets base difficulty", code, 401, body)
	wantDifficulty("other client", ch, 12)
}
