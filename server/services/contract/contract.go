package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"pow-shield-go/services/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/google/uuid"
)

// Payload versions: v1 "1:<uuid>:<ts>", v2 adds the request binding "2:<uuid>:<ts>:<request hash>".
// Both may carry a solved challenge at the end: ":<prefix>:<nonce hex>".
var versionFields = map[string]int{"1": 3, "2": 4}

// maxPayload bounds the decrypted payload, a compressed message cannot inflate past it
const maxPayload = 512

// Contract is the decrypted header payload
type Contract struct {
	ID      string
	Binding string // hex sha256 of the request (v2), empty on v1
	Prefix  string
	Nonce   string
}

// RequestHash is the v2 binding: hex(sha256(method "\n" request URI "\n" body))
func RequestHash(method, requestURI string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method + "\n" + requestURI + "\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// Verifier opens the contract header: base64 of an OpenPGP message (binary or armored) encrypted to the WAF key
type Verifier struct {
	keyring openpgp.EntityList
	maxAge  time.Duration
}

// Load reads the WAF armored private key, decrypting it with passphrase when protected
func Load(privateKeyFile, passphrase string, maxAge time.Duration) (*Verifier, error) {
	raw, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("key file %s: %w", privateKeyFile, err)
	}
	for _, e := range keyring {
		if e.PrivateKey == nil {
			return nil, fmt.Errorf("key file %s: no private key", privateKeyFile)
		}
		if e.PrivateKey.Encrypted {
			if err := e.DecryptPrivateKeys([]byte(passphrase)); err != nil {
				return nil, fmt.Errorf("decrypting private key: %w", err)
			}
		}
	}

	return &Verifier{keyring: keyring, maxAge: maxAge}, nil
}

// Verify decrypts the header and checks version, uuid and timestamp
func (v *Verifier) Verify(header string, now time.Time) (*Contract, error) {
	raw, err := utils.FromBase64(header)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}

	var msg io.Reader = bytes.NewReader(raw)
	if bytes.HasPrefix(raw, []byte("-----BEGIN")) {
		block, err := armor.Decode(msg)
		if err != nil {
			return nil, fmt.Errorf("armor: %w", err)
		}
		msg = block.Body
	}

	md, err := openpgp.ReadMessage(msg, v.keyring, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	if !md.IsEncrypted || md.IsSymmetricallyEncrypted {
		return nil, fmt.Errorf("message must be encrypted to the WAF key")
	}

	body, err := io.ReadAll(io.LimitReader(md.UnverifiedBody, maxPayload+1))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if len(body) > maxPayload {
		return nil, fmt.Errorf("payload too large")
	}

	parts := strings.Split(strings.TrimSpace(string(body)), ":")
	fields, ok := versionFields[parts[0]]
	if !ok {
		return nil, fmt.Errorf("unsupported version %q", parts[0])
	}
	if len(parts) != fields && len(parts) != fields+2 {
		return nil, fmt.Errorf("malformed payload")
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid uuid")
	}

	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp")
	}
	if age := now.Sub(time.Unix(ts, 0)); age > v.maxAge || age < -v.maxAge {
		return nil, fmt.Errorf("stale timestamp, age %s", age)
	}

	c := &Contract{ID: id.String()}
	if fields == 4 {
		if b, err := hex.DecodeString(parts[3]); err != nil || len(b) != sha256.Size {
			return nil, fmt.Errorf("invalid request hash")
		}
		c.Binding = strings.ToLower(parts[3])
	}
	if len(parts) == fields+2 {
		c.Prefix, c.Nonce = parts[fields], parts[fields+1]
	}
	return c, nil
}
