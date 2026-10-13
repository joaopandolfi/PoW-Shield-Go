// keygen writes throwaway keys for the e2e stack: waf.key.asc / waf.pub.asc (OpenPGP contract key)
// and id.key.pem (Ed25519, signs the id tokens issued by the attest handshake)
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func write(path, kind string, serialize func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := armor.Encode(f, kind, nil)
	if err != nil {
		return err
	}
	if err := serialize(w); err != nil {
		return err
	}
	return w.Close()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: keygen <dir>")
		os.Exit(2)
	}
	dir := os.Args[1]

	// curve25519, same default as openpgp.js on the front
	e, err := openpgp.NewEntity("waf-e2e", "", "waf@e2e", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err == nil {
		err = write(filepath.Join(dir, "waf.key.asc"), openpgp.PrivateKeyType, func(w io.Writer) error { return e.SerializePrivate(w, nil) })
	}
	if err == nil {
		err = write(filepath.Join(dir, "waf.pub.asc"), openpgp.PublicKeyType, e.Serialize)
	}
	if err == nil {
		err = writeIDKey(filepath.Join(dir, "id.key.pem"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeIDKey(path string) error {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o644)
}
