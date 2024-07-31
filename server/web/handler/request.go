package handler

import (
	"net/http"
	"strings"
)

const POW_HEADER = "pow-token"

func Headers(r *http.Request) map[string]string {
	result := map[string]string{}
	for v, k := range r.Header {
		result[v] = strings.Join(k, ",")
	}
	return result
}

func PowHeader(r *http.Request) string {
	return r.Header.Get(POW_HEADER)
}
