package metrics

import (
	"fmt"
	"pow-shield-go/internal/cache"
	"strings"
	"sync"
	"sync/atomic"
)

var totalRequests atomic.Uint64
var proxiedRequests atomic.Uint64
var blockedResponses atomic.Uint64
var powBlocked atomic.Uint64
var rateLimited atomic.Uint64

var wafMu sync.RWMutex
var wafCounts = map[string]*atomic.Uint64{}

func IncRequest() {
	totalRequests.Add(1)
}

func IncProxied() {
	proxiedRequests.Add(1)
}

func IncBlockedResponses() {
	blockedResponses.Add(1)
}

func IncPoWBlocked() {
	powBlocked.Add(1)
}

func IncRateLimited() {
	rateLimited.Add(1)
}

func IncWAFBlocked(scope, category string) {
	if category == "" {
		category = "unknown"
	}
	key := scope + ":" + category

	wafMu.Lock()
	counter, ok := wafCounts[key]
	if !ok {
		counter = &atomic.Uint64{}
		wafCounts[key] = counter
	}
	wafMu.Unlock()

	counter.Add(1)
}

func Prometheus() string {
	var b strings.Builder
	b.WriteString("# HELP pow_shield_requests_total Total HTTP requests seen by the proxy layer\n")
	b.WriteString("# TYPE pow_shield_requests_total counter\n")
	b.WriteString(fmt.Sprintf("pow_shield_requests_total %d\n", totalRequests.Load()))
	b.WriteString("# HELP pow_shield_proxied_requests_total Requests forwarded to the upstream server\n")
	b.WriteString("# TYPE pow_shield_proxied_requests_total counter\n")
	b.WriteString(fmt.Sprintf("pow_shield_proxied_requests_total %d\n", proxiedRequests.Load()))
	b.WriteString("# HELP pow_shield_blocked_responses_total Requests blocked before reaching the upstream\n")
	b.WriteString("# TYPE pow_shield_blocked_responses_total counter\n")
	b.WriteString(fmt.Sprintf("pow_shield_blocked_responses_total %d\n", blockedResponses.Load()))
	b.WriteString("# HELP pow_shield_pow_blocked_total Requests blocked by proof-of-work validation\n")
	b.WriteString("# TYPE pow_shield_pow_blocked_total counter\n")
	b.WriteString(fmt.Sprintf("pow_shield_pow_blocked_total %d\n", powBlocked.Load()))
	b.WriteString("# HELP pow_shield_rate_limited_total Requests blocked by rate limiting\n")
	b.WriteString("# TYPE pow_shield_rate_limited_total counter\n")
	b.WriteString(fmt.Sprintf("pow_shield_rate_limited_total %d\n", rateLimited.Load()))
	b.WriteString("# HELP pow_shield_cache_size Number of items in the configured cache backend when available\n")
	b.WriteString("# TYPE pow_shield_cache_size gauge\n")
	b.WriteString(fmt.Sprintf("pow_shield_cache_size %d\n", cache.Get().Size()))
	b.WriteString("# HELP pow_shield_waf_blocked_total Requests blocked by WAF rules\n")
	b.WriteString("# TYPE pow_shield_waf_blocked_total counter\n")

	wafMu.RLock()
	defer wafMu.RUnlock()
	for key, counter := range wafCounts {
		scope, category := key, "unknown"
		if parts := strings.SplitN(key, ":", 2); len(parts) == 2 {
			scope = parts[0]
			category = parts[1]
		}
		b.WriteString(fmt.Sprintf("pow_shield_waf_blocked_total{scope=%q,category=%q} %d\n", scope, category, counter.Load()))
	}

	return b.String()
}

func GetMetricsSnapshot() map[string]interface{} {
	wafMu.RLock()
	defer wafMu.RUnlock()

	wafCountsMap := map[string]interface{}{}
	for key, counter := range wafCounts {
		wafCountsMap[key] = counter.Load()
	}

	return map[string]interface{}{
		"total_requests":    totalRequests.Load(),
		"proxied_requests":  proxiedRequests.Load(),
		"blocked_responses": blockedResponses.Load(),
		"pow_blocked":       powBlocked.Load(),
		"rate_limited":      rateLimited.Load(),
		"cache_size":        cache.Get().Size(),
		"waf_blocked":       wafCountsMap,
	}
}

func ResetMetrics() {
	totalRequests.Store(0)
	proxiedRequests.Store(0)
	blockedResponses.Store(0)
	powBlocked.Store(0)
	rateLimited.Store(0)

	wafMu.Lock()
	defer wafMu.Unlock()
	for k := range wafCounts {
		delete(wafCounts, k)
	}
}
