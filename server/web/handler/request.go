package handler

import (
	"fmt"
	"net"
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

func IP(r *http.Request) string {
	ip := r.RemoteAddr
	parsedIP := net.ParseIP(r.RemoteAddr)
	if parsedIP != nil {
		ip = string(parsedIP)
	}
	splitedIP := strings.Split(ip, ":")
	ip = strings.Join(splitedIP[:len(splitedIP)-1], ":")
	return fmt.Sprintf("%s %s", ip, r.Header.Get("X-Real-Ip"))
}
