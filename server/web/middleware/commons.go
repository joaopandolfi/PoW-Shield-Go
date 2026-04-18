package middleware

import (
	"net/http"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/web/handler"
)

func cleanAll(w http.ResponseWriter, r *http.Request) {
	handler.CleanCookies(w)
	handler.CleanSessions(w, r)
	handler.CleanTempSessions(w, r)
}

func blockRequest(w http.ResponseWriter) {
	metrics.IncBlockedResponses()
	handler.RespondBlock(w)
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
