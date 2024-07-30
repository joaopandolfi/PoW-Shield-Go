package utils

import "encoding/base64"

// ToBase64 convert byte to b64
func ToBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// FromBase64 decode from base64
func FromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
