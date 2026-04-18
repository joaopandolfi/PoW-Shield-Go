# Configuration Reference

## Environment Variables

All configuration is loaded from a `.env` file (or environment variables) using `godotenv`. The file should be placed at `server/.env` relative to the project root.

Copy `.env.example` to `server/.env` to start.

---

## Server Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `PORT` | `5656` | HTTP listen port |
| `SEC_BROWSER_XSS_FILTER` | `true` | Enable XSS filter header |
| `SEC_CONTENT_TYPE_NO_SNIFF` | `true` | Enable content-type sniffing prevention |
| `SEC_SSL_HOST` | `localhost:443` | SSL hostname for HSTS redirect |
| `SEC_SSL_REDIRECT` | `false` | Force HTTPS redirect |

---

## Session Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `SESSION_NAME` | `PoW-Session` | Session store name |
| `SESSION_PASS` | `12345670101112ABC` | CookieStore signing key |
| `SESSION_PATH` | `/` | Cookie path |
| ` SESSION_MAX_AGE` | `7200` | Session max age in seconds (2 hours) |
| `SESSION_HTTP_ONLY` | `true` | Mark cookies as HttpOnly |
| `SESSION_SECURE` | `true` | Mark cookies as Secure (HTTPS only) |

---

## Proof of Work Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `DEFAULT_PREFIX_SIZE` | `15` | Base difficulty level (leading zero bits) |
| `PUNISHMENT` | `1` | Difficulty increase per failed attempt |
| `NONCE_VALIDITY` | `150000` | Nonce timestamp validity window in milliseconds (2.5 min) |
| `USE_COOKIE` | `true` | Use cookie for session transport |
| `USE_SESSION` | `true` | Use server-side session store |
| `USE_HEADER` | `true` | Use `pow-token` header |
| `POW_ACTIVE` | `true` | Enable PoW middleware |
| `IP_TOLLERANCE_DURATION_SECONDS` | `120` | IP tolerance tracking duration (seconds) |
| `IP_TOLLERANCE` | `1` | Max failed session attempts before new allocation |

---

## Protected Server Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `PROTECTED_SERVER_HOST` | `http://localhost:3001` | Backend server URL to proxy |
| `PROTECTED_SERVER_HEADERS` | `[]` | JSON default headers to add to proxied requests |

**Example headers:**
```env
PROTECTED_SERVER_HEADERS=["X-Forwarded-For":"127.0.0.1","X-Real-IP":"192.168.1.1"]
```

---

## WAF (Web Application Firewall) Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `WAF_ACTIVE` | `true` | Enable WAF middleware |
| `WHITELIST_URL_RULES` | `[]` | JSON array of rule IDs to exclude from URL inspection |
| `WHITELIST_BODY_RULES` | `[]` | JSON array of rule IDs to exclude from body inspection |
| `WHITELIST_HEADER_RULES` | `[]` | JSON array of rule IDs to exclude from header inspection |
| `WAF_RULES_FILE` | `wafRules.json` | Path to WAF rules JSON file |

---

## Redis Cache Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `REDIS_USE` | `false` | Enable Redis backend (disabled = in-memory cache) |
| `REDIS_HOST` | `localhost:6379` | Redis server address |
| `REDIS_PASS` | (empty) | Redis password |
| `REDIS_DB` | `1` | Redis database number |

---

## TLS Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `USE_TLS` | `false` | Enable HTTPS with TLS |
| `TLS_CERT` | (empty) | Path to TLS certificate file |
| `TLS_KEY` | (empty) | Path to TLS private key file |

---

## Static Files Configuration

| Variable | Default | Description |
|----------|---------|-------|
| `SERVE_STATIC` | `true` | Enable static file serving |
| `SERVE_STATIC_PATH` | `/public` | URL path prefix for static files |
| `SERVE_STATIC_FOLDER` | `../client/public/` | Filesystem path to static files |

---

## Admin Panel Configuration

| Variable | Default | Description |
|----------|-----|--------|
| `ADMIN_ACTIVE` | `false` | Enable admin panel |
| `ADMIN_PATH` | `/admin` | URL path prefix for admin SPA |
| `ADMIN_PASSWORD` | `admin123` | Login password (only used when `ADMIN_KEY` is not set) |
| `ADMIN_KEY` | (empty) | Optional API key for header-based auth via `X-Admin-Key` header |

**Authentication methods (checked in order):**
1. `admin_session` cookie = `admin` (HttpOnly, SameSite=Strict)
2. `X-Admin-Key` request header = value of `ADMIN_KEY`

When `ADMIN_KEY` is set, cookie authentication is optional — header auth can be used as the sole method.

---

## Configuration Precedence

1. Environment variables (OS)
2. `.env` file (via `godotenv.Load`)

The `.env` file is loaded relative to the Go working directory, not the binary location. This means the server binary must be run from the `server/` directory, or the path adjustment must be configured.

**Note:** When running via `make run-backend`, the working directory is already `server/`, so `.env` should be at `server/.env`.

---

## Defaults Mismatch Warning

The `.env.example` file shows different defaults than the actual Go code for some variables. Notable differences:

| Variable | `.env.example` | Go Default | Impact |
|----------|-----------|---------|--------|
| `USE_COOKIE` | `false` | `true` | Example shows cookie disabled; code enables it |
| `WHITELIST_URL_RULES` | `[35,53]` | `[]` | Example whitelists rules 35 and 53 |
| `WHITELIST_BODY_RULES` | `[35,53]` | `[]` | Example whitelists rules 35 and 53 |
| `WHITELIST_HEADER_RULES` | `[33,35,53]` | `[]` | Example whitelists rules 33, 35, and 53 |

The whitelist rules in `.env.example` may be intentional deviations, but developers should be aware of the mismatch.

---

## Production Recommendations

1. **Change `SESSION_PASS`** to a strong random key
2. **Enable `REDIS_USE=true`** for distributed deployments
3. **Set `SESSION_SECURE=true`** and `SEC_SSL_REDIRECT=true` in production with TLS
4. **Set `SESSION_MAX_AGE`** to an appropriate expiration (default 2 hours)
5. **Configure `PROTECTED_SERVER_HOST`** to the actual backend URL
6. **Generate `TLS_CERT` and `TLS_KEY`** if using HTTPS
7. **Review WAF whitelist rules** to avoid blocking legitimate requests
8. **Monitor in-memory cache** -- the 150-entry limit should have eviction or Redis should be used
