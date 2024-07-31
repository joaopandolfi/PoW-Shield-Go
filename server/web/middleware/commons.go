package middleware

import (
	"net/http"
	"pow-shield-go/web/handler"
)

func cleanAll(w http.ResponseWriter, r *http.Request) {
	handler.CleanCookies(w)
	handler.CleanSessions(w, r)
}

func blockRequest(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotAcceptable)
	w.Write([]byte("blocked: x_x"))
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
