package middleware

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"pow-shield-go/web/handler"
)

type SessionData string

const (
	UserID SessionData = "userID"
)

const sessionKeyID = "s:"

// Identificator provides request identification
func Identificator(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasher := sha1.New()
		hasher.Write([]byte(handler.IP(r)))
		bs := fmt.Sprintf("%x", hasher.Sum(nil))
		id := fmt.Sprintf("%s%s", sessionKeyID, bs)
		ctx := context.WithValue(r.Context(), UserID, string(id))
		r = r.WithContext(ctx)

		h.ServeHTTP(w, r)
	})
}
