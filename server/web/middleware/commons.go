package middleware

import "net/http"

func blockRequest(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotAcceptable)
	w.Write([]byte("O_O"))
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
