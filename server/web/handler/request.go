package handler

import (
	"net/http"
	"strings"
)

func Headers(r *http.Request) map[string]string {
	result := map[string]string{}
	for v, k := range r.Header {
		result[v] = strings.Join(k, ",")
	}
	return result
}
