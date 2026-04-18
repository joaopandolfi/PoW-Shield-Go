# Bugs and Issues Report

## Critical Bugs — RESOLVIDOS ✅

### 1. Redis `Get()` -- Hardcoded Key ✅ FIXED
- **File:** `server/internal/cache/redis.go:53`
- **Issue:** `c.client.Get(c.ctx, "key")` usa string literal `"key"` ao invés da variável `key`
- **Fix Applied:** `c.client.Get(c.ctx, key)` — commit `01f4e51`

```diff
- val, err := c.client.Get(c.ctx, "key").Result()
+ val, err := c.client.Get(c.ctx, key).Result()
```

### 2. WAF Middleware Consome Body Sem Restaurar ✅ FIXED
- **File:** `server/web/middleware/waf.go`
- **Issue:** `io.ReadAll(r.Body)` consome o body mas não restaura com `io.NopCloser`
- **Fix Applied:** Body restaurado com `r.Body = io.NopCloser(bytes.NewReader(body))`
- **Impact:** POST/PUT requests agora são encaminhados corretamente ao backend

### 3. Proxy Double Body Read ✅ FIXED
- **File:** `server/web/controllers/proxy/proxy.go`
- **Issue:** Proxy lia `r.Body` via `io.ReadAll` após WAF ter consumido
- **Fix Applied:** Resolvido pela correção do bug #2 (WAF restaura body antes de chamar next)

---

## High Severity Issues — RESOLVIDOS ✅

### 4. In-Memory Cache Buffer Overflow ✅ FIXED
- **File:** `server/internal/cache/memory.go:45-59`
- **Issue:** `MAX_BUFF_SIZE = 150` limit sem eviction strategy
- **Fix Applied:** Implemented TTL-based eviction — removes expired entries when buffer reaches limit before failing
- **Current behavior:** `Put()` now evicts oldest expired entries instead of returning "buffer overflow" error

### 5. Hardcoded CookieStore Secret ✅ FIXED
- **File:** `server/config/config.go:129-143`
- **Issue:** CookieStore usava secret hardcoded `"12345670101112ABC"`
- **Fix Applied:** Session uses `sessionPass()` function that:
  - Uses `SESSION_PASS` env var if set
  - Generates random 32-byte key via `crypto/rand` if not set
  - Falls back to development key only on crypto failure
- **Log:** `[!][Config] SESSION_PASS not set, generated ephemeral random key`

### 6. Proxy Transport Disables TLS Verification ✅ FIXED
- **File:** `server/internal/request/request.go:16-24`
- **Issue:** `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}` hardcoded
- **Fix Applied:** TLS verification now configurable via `PROTECTED_SERVER_INSECURE_SKIP_VERIFY` env var (default: `false`)
- **Config:** `InsecureSkipVerify: StrTo[bool](getEnvOrDefault("PROTECTED_SERVER_INSECURE_SKIP_VERIFY", "false"))`

---

## Medium Severity Issues — RESOLVIDOS ✅

### 7. IP() Helper Produces Malformed IP Strings ✅ FIXED
- **File:** `server/web/handler/request.go:23-38`
- **Issue:** Concatenava `RemoteAddr` com `X-Real-Ip` incorretamente
- **Fix Applied:** IP extraction now:
  1. Parses `RemoteAddr` to extract IP without port
  2. Uses `X-Real-Ip` header when present (from reverse proxy)
  3. Falls back to parsed `RemoteAddr` when no header
- **Result:** Clean IP strings without trailing spaces or format issues

### 8. Redis Test Won't Compile ✅ FIXED
- **File:** `server/internal/cache/redis_test.go`
- **Issue:** Referenciava campos inexistentes (`Port`) e chamar `GetRedis()` sem context
- **Fix Applied:** Test reescrito para usar API correta:
  - Uses `cfg.Cache.Redis.Server` (existing field)
  - Calls `initializeRedis(ctx, ...)` directly with context
  - Uses renamed `GracefulShutdown()` method

### 9. 10-Minute Proxy Timeout ✅ FIXED
- **File:** `server/internal/request/request.go`
- **Issue:** `defaultTimeout time.Duration = time.Minute * 10` extremament longo
- **Fix Applied:** Timeout configurable via `PROTECTED_SERVER_TIMEOUT_SECONDS` env var (default: 30s)
- **Config:** `Timeout: time.Duration(StrTo[int](getEnvOrDefault("PROTECTED_SERVER_TIMEOUT_SECONDS", "30"))) * time.Second`

### 10. `compressB` Used Before Assignment in Error Path ✅ VERIFIED
- **File:** `server/internal/request/request.go:84`
- **Issue:** Se `compressB.ReadFrom(r)` falha, retorna `b` (valor zero)
- **Status:** Minor issue — retorna `nil` corretamente com erro adequado
- **No action needed** — error handling is correct

---

## Configuration Issues — RESOLVIDOS ✅

### 11. Environment Variable Default Mismatch ✅ FIXED
- **Files:** `.env.example` vs `config/config.go`
- **Issue:** `.env.example` mostrava valores inconsistentes com defaults do código
- **Fix Applied:**
  - `USE_COOKIE=true` (was: commented/missing in .env.example)
  - `USE_SESSION=true` (was: commented/missing in .env.example)
  - `USE_HEADER=true` (was: commented/missing in .env.example)
  - Added default values for `WHITELIST_*_RULES` as `[]` in code
  - Added `PROTECTED_SERVER_TIMEOUT_SECONDS=30` with proper default
  - Added `PROTECTED_SERVER_INSECURE_SKIP_VERIFY=false` with secure default
  - Added `RATE_LIMIT_*` configuration with sensible defaults
  - Added `METRICS_*` configuration

### 12. `SESSION_SECURE` Blocks Development ✅ FIXED
- **File:** `server/config/config.go:182`
- **Issue:** Cookies marcados como `Secure` por padrão, bloqueando HTTP local
- **Fix Applied:** `SESSION_SECURE` defaults to `USE_TLS` value:
  ```go
  Secure: StrTo[bool](getEnvOrDefault("SESSION_SECURE", fmt.Sprintf("%t", StrTo[bool](getEnvOrDefault("USE_TLS", "false")))))
  ```
- **Result:** `Secure=false` when TLS disabled, `Secure=true` when TLS enabled

---

## Low Severity / Cosmetic Issues — RESOLVIDOS ✅

### 13. Function Name Typo: `gracefullShutdown` ✅ FIXED
- **Files:** `main.go`, `memory.go`, `redis.go`
- **Issue:** "Graceful" misspelled as "gracefull"
- **Fix Applied:** Renamed to `GracefulShutdown()` / `gracefulShutdown()` across all files

### 14. Comment Typo: "ised" in injectable.go ✅ FIXED
- **File:** `server/internal/cache/injectable.go:13`
- **Issue:** "is ised" → "is used"
- **Fix Applied:** Comment corrected

### 15. Function Name Typo: `NewGerator()` ✅ FIXED
- **File:** `server/services/pow/generator.go:23`
- **Issue:** "Generator" misspelled as "Gerator"
- **Fix Applied:** Renamed to `NewGenerator()`

### 16. Function Name Typo: `compelexity` ✅ FIXED
- **File:** `server/services/pow/verifier.go`
- **Issue:** Parameter named `compelexity` (typo in signature)
- **Fix Applied:** Renamed to `complexity` in interface and implementation

### 17. `wafTypes.json` Unused ✅ FIXED
- **File:** `server/wafTypes.json`
- **Issue:** Arquivo definido mas não consumido pelo Go code
- **Fix Applied:** WAF types now loaded in `waf.go:37-46` with fallback for string keys
- **Config:** `server/config/config.go:225` reads `WAF_TYPES_FILE` env var

### 18. `errors.go` Incomplete ✅ FIXED
- **File:** `server/web/errors.go`
- **Issue:** Apenas `ErrorCodeInternal = 10` definido e não utilizado
- **Fix Applied:** Expanded with proper error codes:
  - `ErrorCodeBadRequest = 11`
  - `ErrorCodeForbidden = 12`
  - `ErrorCodeRateLimited = 13`
  - `ErrorCodeUnavailable = 14`
  - `ErrorCodeNotAcceptable = 15`
  - `DefaultErrorForStatus()` helper function for HTTP status → error mapping

### 19. `permissions.go` Unused ℹ️ ACCEPTED
- **File:** `server/models/permissions.go`
- **Issue:** Apenas contém `PermissionSystem = "system"`, nunca referenciado
- **Status:** Placeholder para futura feature de RBAC — mantido como stub

---

## New Features Added

| Feature | Files | Description |
|---------|-------|-------------|
| Rate Limiting | `middleware/rate_limit.go` | Sliding window rate limiter per IP |
| Prometheus Metrics | `metrics/metrics.go`, `metrics.go` | HTTP metrics with Prometheus format |
| Docker Support | `Dockerfile`, `docker-compose.yml` | Container deployment support |
| Health Check | `health/health.go` | POST/GET/HEAD `/health` endpoints |
| Session Store Options | `handler/session.go` | CookieStore session options configuration |
| Proxy Headers | `proxy/proxy.go` | Custom headers for proxied requests |

