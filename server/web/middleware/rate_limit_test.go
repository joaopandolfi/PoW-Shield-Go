package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// a parallel burst from one IP must let exactly Requests through
func TestRateLimitAtomic(t *testing.T) {
	cfg := config.Config{}
	cfg.Rate.Active = true
	cfg.Rate.Requests = 50
	cfg.Rate.WindowSeconds = 60
	config.Inject(cfg)
	cache.Initialize(context.Background(), time.Minute)
	rateCache = cache.Get()
	rateCache.Flush() // memory cache is a singleton, start clean on repeated runs

	h := RateLimit(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	var passed, limited atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Real-Ip", "10.0.0.1")
			w := httptest.NewRecorder()
			h(w, r)
			switch w.Code {
			case http.StatusOK:
				passed.Add(1)
			case http.StatusTooManyRequests:
				limited.Add(1)
			}
		}()
	}
	wg.Wait()
	if passed.Load() != 50 || limited.Load() != 30 {
		t.Fatalf("passed %d limited %d, want 50/30", passed.Load(), limited.Load())
	}
}
