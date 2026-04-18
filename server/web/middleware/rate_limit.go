package middleware

import (
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	powHandler "pow-shield-go/web/handler"
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

		log := logging.Get()
		ip := powHandler.IP(r)
		key := rateLimitKey(ip)
		count := 0

		val, err := rateCache.Get(key)
		if err != nil {
			if log != nil {
				log.Error("Rate limit cache get error", "error", err.Error())
			}
			powHandler.RespondDefaultError(w, http.StatusInternalServerError)
			return
		}
		if val != nil {
			count = readRateValue(val)
		}

		count++
		window := time.Duration(cfg.WindowSeconds) * time.Second
		if err := rateCache.Put(key, count, window); err != nil {
			if log != nil {
				log.Error("Rate limit cache put error", "error", err.Error())
			}
			powHandler.RespondDefaultError(w, http.StatusInternalServerError)
			return
		}

		if count > cfg.Requests {
			metrics.IncRateLimited()
			if log != nil {
				log.Warn("Rate limit exceeded", "ip", ip, "count", count, "limit", cfg.Requests)
			}
			w.Header().Set("Retry-After", strconv.Itoa(cfg.WindowSeconds))
			powHandler.RespondDefaultError(w, http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
