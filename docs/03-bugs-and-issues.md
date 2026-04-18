# Bugs and Issues Report

## Critical Bugs

### 1. Redis `Get()` -- Hardcoded Key Literal
- **File:** `server/internal/cache/redis.go:53`
- **Issue:** `c.client.Get(c.ctx, "key")` uses the literal string `"key"` instead of the `key` variable
- **Impact:** Redis cache completely non-functional -- all `Get()` calls return the same value regardless of the requested key, or always return `redis.Nil`
- **Fix:** `c.client.Get(c.ctx, key)`

```diff
- val, err := c.client.Get(c.ctx, "key").Result()
+ val, err := c.client.Get(c.ctx, key).Result()
```

### 2. WAF Middleware Consumes Body Without Restoring
- **File:** `server/web/middleware/waf.go:93-109`
- **Issue:** `io.ReadAll(r.Body)` reads the entire request body but the body is never restored with `io.NopCloser(bytes.NewReader(body))` before calling `next(w, r)`
- **Impact:** All proxied requests with a body receive an empty body -- the downstream proxy handler gets nothing. PUT/POST requests to the backend will fail silently
- **Fix:** Restore body after WAF inspection:

```diff
 detecteds = wafDetect(string(body), wafBody)
 if len(detecteds) > 0 {
     log.Println("[*][Middleware][Waf] BODY RULE TRIGGERED: ", detecteds, "on: ", url)
     blockRequest(w)
     return
 }
 
+r.Body = io.NopCloser(bytes.NewReader(body))
 next(w, r)
```

### 3. Proxy Double Body Read
- **File:** `server/web/controllers/proxy/proxy.go:28`
- **Issue:** The proxy handler reads `r.Body` via `io.ReadAll(r.Body)` at line 28. If the WAF middleware is enabled (default), the body has already been consumed and closed at `waf.go:93-99` (which also calls `r.Body.Close()`)
- **Impact:** Under WAF-active configuration (the default), the proxy always gets an empty body and returns `400 Bad Request` for any POST/PUT request. Even without WAF, the body is consumed once by WAF, making proxy body forwarding unreliable
- **Fix:** Use `io.NopCloser(bytes.NewReader(body))` in WAF to restore body (see bug #2), which resolves this issue

---

## High Severity Issues

### 4. In-Memory Cache Buffer Overflow -- No Eviction Strategy
- **File:** `server/internal/cache/memory.go:44-46`
- **Issue:** `MAX_BUFF_SIZE = 150` hard limit, no eviction strategy. When buffer reaches 150 entries, ALL subsequent `Put()` calls fail with "buffer overflow" error
- **Impact:** Under moderate load (150 concurrent sessions), new challenge generation fails and service returns 500 errors. No graceful degradation
- **Recommended Fix:** Implement TTL-based eviction (already partially present via `validAt`) or LRU eviction policy

```go
// Current -- rejects all inserts when full
if len(c.buff) > MAX_BUFF_SIZE {
    return fmt.Errorf("buffer overflow")
}

// Recommended -- remove this check, rely on GarbageCollector
```

### 5. Hardcoded CookieStore Secret -- Security Risk
- **File:** `server/config/config.go:123`
- **Issue:** CookieStore uses hardcoded secret `"12345670101112ABC"` in production default
- **Impact:** Anyone can forge valid sessions for any protected instance
- **Recommended Fix:** Generate a random secret on first run and store it, or enforce environment variable setting

---

## Medium Severity Issues

### 6. Proxy Transport Disables TLS Verification
- **File:** `server/internal/request/request.go:20`
- **Issue:** `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}` -- TLS certificate verification is completely bypassed
- **Impact:** The proxy is vulnerable to man-in-the-middle attacks on the connection to the protected backend. SSL pinning is completely bypassed
- **Recommended Fix:** Use default transport or configure proper CA certs:
```diff
- TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
+ // Use default transport with proper verification
```

### 7. IP() Helper Produces Malformed IP Strings
- **File:** `server/web/handler/request.go:32`
- **Issue:** `IP()` concatenates the stripped `RemoteAddr` with `X-Real-Ip` with a space separator: `fmt.Sprintf("%s %s", ip, r.Header.Get("X-Real-Ip"))`. If `X-Real-Ip` header is empty (no reverse proxy), the result is `"192.168.1.1 "` (trailing space)
- **Impact:** SHA1 hashes in the identificator middleware include trailing spaces for direct connections, producing inconsistent session IDs. When behind a reverse proxy, the hash combines two values into one string
- **Recommended Fix:** Handle empty `X-Real-Ip` separately, or use `X-Real-Ip` exclusively when present

### 8. Redis Test Won't Compile
- **File:** `server/internal/cache/redis_test.go`
- **Issue:** 
  - References `cfg.Cache.Redis.Port` which doesn't exist (struct has `Server` field, no `Port` field)
  - Calls `GetRedis()` without context but the signature is `GetRedis(ctx context.Context)`
  - Sets `Port = "333"` but struct only has `Server`, `Password`, `DB`, `Use`
- **Impact:** No Redis integration tests can run. Redis backend is untested

### 9. 10-Minute Proxy Timeout
- **File:** `server/internal/request/request.go:15`
- **Issue:** `defaultTimeout time.Duration = time.Minute * 10` -- extremely long timeout for a reverse proxy
- **Impact:** Slow connections or unresponsive backends hold connections for up to 10 minutes, exhausting server resources. Connection pool exhaustion under load

### 10. `respond.go:50` -- `compressB` Used Before Assignment in Error Path
- **File:** `server/internal/request/request.go:84`
- **Issue:** If `compressB.ReadFrom(r)` fails, function returns `b` (not `compressB`), but `compressB` was declared in the preceding outer scope
- **Impact:** Minor -- returns the zero value of `b` which is `nil`, but the error is correct

---

## Configuration Issues

### 11. Environment Variable Default Mismatch
- **File:** `.env.example` vs `config/config.go`
- **Issue:** `.env.example` sets `USE_COOKIE=false` as the example default, but `config/config.go:138` defaults `USE_COOKIE` to `true`. Same for `USE_SESSION` and `USE_HEADER`
- **Impact:** Developers following `.env.example` get different behavior than the actual defaults. If only `REDIS_USE=false` is in `.env` without setting PoW transports, the service won't initialize (the validation at line 168)

### 12. `SESSION_SECURE=true` Blocks Development
- **File:** `config/config.go:129`
- **Issue:** Session cookies are marked `Secure` by default (requires HTTPS). In local development without TLS, cookies are never sent by browsers
- **Impact:** Development requires setting `SESSION_SECURE=false`. No automatic fallback based on TLS configuration

---

## Low Severity / Cosmetic Issues

### 13. Function Name Typo: `gracefullShutdown`
- **Files:** `server/main.go:31`, `server/internal/cache/memory.go:130`, `server/internal/cache/redis.go:72`
- **Issue:** "Graceful" misspelled as "gracefull" (double L)
- **Status:** Cosmetic -- affects no functionality

### 14. Comment Typo: "ised" in injectable.go
- **File:** `server/internal/cache/injectable.go:13`
- **Issue:** "lateInitCache is ised to inject" should be "is used to inject"
- **Status:** Cosmetic

### 15. Function Name Typo: `NewGerator()`
- **File:** `server/services/pow/generator.go:23`
- **Issue:** "Generator" misspelled as "Gerator"
- **Status:** Affects public API naming

### 16. Function Name Typo: `NewVerifier()` -- Parameter named `compelexity`
- **File:** `server/services/pow/verifier.go:35`
- **Issue:** "Complexity" misspelled as "compelexity" in function signature
- **Status:** Cosmetic, affects API

### 17. `wafTypes.json` Unused
- **File:** `server/wafTypes.json`
- **Issue:** File defines integer-to-name mappings for WAF rule types but no Go code reads or references it
- **Status:** Dead code

### 18. `errors.go` Incomplete
- **File:** `server/web/errors.go`
- **Issue:** Only defines `ErrorCodeInternal = 10` and `ErrorMessageInternal = "internal error"`, neither of which are used anywhere in the codebase
- **Status:** Placeholder, not consumed

### 19. `permissions.go` Unused
- **File:** `server/models/permissions.go`
- **Issue:** Only contains `PermissionSystem = "system"`, never referenced
- **Status:** Placeholder
