package middleware

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"net/http"
	"pow-shield-go/web/handler"
)

type SessionData string

const (
	UserID SessionData = "userID"
)

const sessionKeyID = "session:"

var hasher hash.Hash

func InitIdentificator() {
	hasher = sha1.New()
}

// Identificator provides request identification
func Identificator(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		id := fmt.Sprintf("%s%s", sessionKeyID, handler.IP(r))
		hasher.Write([]byte(id))
		bs := fmt.Sprintf("%x", hasher.Sum(nil))
		ctx := context.WithValue(r.Context(), UserID, string(bs))
		r = r.WithContext(ctx)

		h.ServeHTTP(w, r)
	})
}
