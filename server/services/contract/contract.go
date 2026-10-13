package contract

import (
	"bytes"
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

// Version is the only contract payload format accepted today
const Version = "1"

// maxPayload bounds the decrypted payload, a compressed message cannot inflate past it
const maxPayload = 512

// Contract is the decrypted header payload: "<version>:<uuid>:<unix_ts>[:<prefix>:<nonce hex>]"
type Contract struct {
	ID     string
	Prefix string
	Nonce  string
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
	if len(parts) != 3 && len(parts) != 5 {
		return nil, fmt.Errorf("malformed payload")
	}
	if parts[0] != Version {
		return nil, fmt.Errorf("unsupported version %q", parts[0])
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
	if len(parts) == 5 {
		c.Prefix, c.Nonce = parts[3], parts[4]
	}
	return c, nil
}
