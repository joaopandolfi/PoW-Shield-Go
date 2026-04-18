package middleware

import (
	"fmt"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/web/handler"
	"strconv"
	"time"
)

var rateCache cache.Cache

func InitRateLimiter() {
	if rateCache == nil {
		rateCache = cache.Get()
	}
}

func rateLimitKey(ip string) string {
	return "rate:" + ip
}

func readRateValue(raw interface{}) int {
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case string:
		count, err := strconv.Atoi(v)
		if err == nil {
			return count
		}
	}

	return 0
}

func RateLimit(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get().Rate
		if !cfg.Active {
			next(w, r)
			return
		}

		ip := handler.IP(r)
		key := rateLimitKey(ip)
		count := 0

		val, err := rateCache.Get(key)
		if err != nil {
			log.Println("[!][Middleware][RateLimit] cache get:", err.Error())
			handler.RespondDefaultError(w, http.StatusInternalServerError)
			return
		}
		if val != nil {
			count = readRateValue(val)
		}

		count++
		window := time.Duration(cfg.WindowSeconds) * time.Second
		if err := rateCache.Put(key, count, window); err != nil {
			log.Println("[!][Middleware][RateLimit] cache put:", err.Error())
			handler.RespondDefaultError(w, http.StatusInternalServerError)
			return
		}

		if count > cfg.Requests {
			metrics.IncRateLimited()
			log.Println("[+][Middleware][RateLimit] blocking", ip, fmt.Sprintf("%d/%d", count, cfg.Requests))
			w.Header().Set("Retry-After", strconv.Itoa(cfg.WindowSeconds))
			handler.RespondDefaultError(w, http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
