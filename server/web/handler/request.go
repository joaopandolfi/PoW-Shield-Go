package handler

import (
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
		ip = parsedIP.String()
	}
	splitedIP := strings.Split(ip, ":")
	if len(splitedIP) > 1 {
		ip = strings.Join(splitedIP[:len(splitedIP)-1], ":")
	}
	realIP := r.Header.Get("X-Real-Ip")
	if realIP != "" {
		return realIP
	}
	return ip
}
