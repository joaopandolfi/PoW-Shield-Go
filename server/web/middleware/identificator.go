package middleware

import (
	"context"
	"fmt"
	"net/http"
	"pow-shield-go/web/handler"
)

type SessionData string

const (
	UserID SessionData = "userID"
)

const sessionKeyID = "session:"

// Identificator provides request identification
func Identificator(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		id := fmt.Sprintf("%s%s", sessionKeyID, handler.IP(r))
		ctx := context.WithValue(r.Context(), UserID, id)
		r = r.WithContext(ctx)

		h.ServeHTTP(w, r)
	})
}
