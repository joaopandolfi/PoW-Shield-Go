# API Reference

All endpoints are served under the root path `/`. The server listens on port `5656` by default.

---

## Health Check

### `GET /health`

Returns the health status of the service.

**Response**

| HTTP Status | Content-Type | Body |
|------|--------|------|
| 200 | `application/json` | `true` |

**Example**

```bash
curl http://localhost:5656/health
# → true
```

---

## Static Files

### `GET /welcome`

Serves the welcome page with PoW verification required. Protected by PoW middleware.

**Response**

| HTTP Status | Content-Type | Body |
|------|--------|------|
| 200 | `text/html` | Welcome page HTML |

### `GET /public/...`

Serves static files from the configured static folder (default `../client/public/`).

**Response**

| HTTP Status | Content-Type | Body |
|------|--------|------|
| 200 | varies | Static asset |

---

## Proof of Work

### `GET /pow/`

Requests a new PoW challenge from the server. Returns a challenge with a hex prefix and difficulty level that the client solver must brute-force.

**Query Parameters**

None.

**Headers**

| Header | Required | Description |
|--------|----------|-------------|
| `pow-token` | No | Previously wrapped session token (for header transport mode) |

**Response**

| HTTP Status | Content-Type | Body |
|------|--------|------|
| 200 | `application/json` | Challenge payload |

**Response Body**

```json
{
  "prefix": "<hex_string>",
  "difficulty": <int>,
  "id": "<sha1_hash_prefixed_session_id>",
  "token": "<base64_encoded_json_payload>"
}
```

**Fields**

| Field | Type | Description |
|-------|------|-------------|
| `prefix` | string | Hex-encoded random prefix. Client must compute SHA256(prefix + nonce) |
| `difficulty` | int | Number of leading zero bits required in the hash |
| `id` | string | Session identifier (SHA1-hashed client IP) |
| `token` | string | Base64-encoded JSON of the full challenge payload |

**Example**

```bash
curl http://localhost:5656/pow/
# → {"prefix":"a1b2c3","difficulty":30,"id":"s:2a3b4c5d...","token":"eyJwcmV..."}
```

---

### `POST /pow/`

Submits a solved challenge for verification. Expects a nonce buffer pre-fixed with a millisecond timestamp.

**Request Body**

```json
{
  "buffer": "<hex_encoded_nonce>",
  "difficulty": <int>,
  "prefix": "<hex_string>"
}
```

**Fields**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `buffer` | string | Yes | Hex-encoded nonce (16 bytes: 8-byte BE timestamp + 8 random bytes) |
| `difficulty` | int | Yes | The complexity level to verify against |
| `prefix` | string | Yes | The hex prefix from the challenge |

**Validations**

- `buffer` must be valid hex (decoded)
- Nonce size must be between 8 and 32 bytes
- Nonce timestamp must be within `NONCE_VALIDITY` ms (default 150000ms = 2.5 min)
- SHA256(prefix + nonce) must have at least `difficulty` leading zero bits

**Response**

| HTTP Status | Content-Type | Body |
|------|--------|------|
| 200 | `application/json` | Verification token |
| 400 | `application/json` | Error response |
| 406 | `application/json` | Invalid nonce |

**Success Response**

```json
{
  "token": "<public_wrapped_session_string>"
}
```

The `token` field contains the base64-wrapped session data. This is set as a `powShield` cookie and/or session store.

**Example**

```bash
# Submit solved nonce
curl -X POST http://localhost:5656/pow/ \
  -H "Content-Type: application/json" \
  -d '{"buffer":"000000067890123456789abcdef0","difficulty":15,"prefix":"a1b2c3"}'
# → {"token":"eyJJZCI6InN..."}
```

---

## Reverse Proxy

### `ANY /*`

Forwards all requests to the protected backend server. Protected by WAF and PoW middleware chain.

**Middleware Chain**

1. **WAF** - Inspects URL, headers, and body against 120 regex rules
2. **PoW** - Validates session cookie, session store, or header token

**Upstream**

Requests are forwarded to `PROTECTED_SERVER_HOST` (default `http://localhost:3001`) preserving:
- Method (GET, POST, PUT, DELETE, etc.)
- Original headers
- Request body

**Response Headers**

Response headers from the backend are forwarded back to the client.

**Response Body**

Response body from the backend is forwarded back to the client.

---

## Middleware Details

### PoW Session Transport Modes

PoW supports three transport modes for session tokens. Multiple modes can be active simultaneously:

| Transport | Cookie/Header | Description |
|-----------|--------------|-------------|
| Cookie | `powShield` | Session stored in a `powShield` HTTP cookie |
| Session | Gorilla CookieStore | Session stored server-side in CookieStore |
| Header | `pow-token` | Session stored client-side in `pow-token` request header |

### Session State Machine

The challenge follows a state machine cached in the local/Redis cache:

| State | Format | Description |
|-------|--------|-------------|
| `to-solve:<difficulty>` | `to-solve:30` | New challenge, no previous failures |
| `error:<difficulty>` | `error:35` | Failed attempt, difficulty increased |
| `verified:<difficulty>:<nonce>` | `verified:30:0000...` | Successfully verified, nonce stored |

### IP Tolerance

Failed verification attempts are tracked per-IP with the key pattern `session:<session_id>`. The count increases by `PUNISHMENT` (default 1) on each new attempt. If the count is within `IP_TOLLERANCE` (default 1), a new session is allocated. Each failure also increases the PoW difficulty for new challenges.

### WAF Rule Whitelisting

Specific rules can be whitelisted (excluded) from URL, header, and body inspection via configuration:

| Whitelist | Config | Description |
|-----------|--------|-------------|
| URL | `WHITELIST_URL_RULES` | Rule IDs excluded from URL inspection |
| Header | `WHITELIST_HEADER_RULES` | Rule IDs excluded from header inspection |
| Body | `WHITELIST_BODY_RULES` | Rule IDs excluded from body inspection |

Example: `WHITELIST_URL_RULES=[35,53]` excludes rules with IDs 35 and 53 from URL matching.

---

## Admin Panel

The admin panel is a client-side SPA served at the configurable admin path (default `/admin`). It uses relative paths so it works with any configured admin path prefix.

**Configuration:**
- `ADMIN_ACTIVE=true` — enables the admin panel (default: `false`)
- `ADMIN_PATH` — path prefix (default: `/admin`)
- `ADMIN_PASSWORD` — login password (default: `admin123` if not set)
- `ADMIN_KEY` — optional API key for header-based auth via `X-Admin-Key`

**Authentication:**
Admin endpoints accept either a cookie (`admin_session=admim`) or the `X-Admin-Key` header when configured.

### `GET /admin/api/check`

Checks authentication status without requiring prior auth. Returns the authentication method if authenticated.

**Response**

| HTTP Status | Content-Type | Body |
|-----|-----|------|
| 200 | `application/json` | `{"ok": true, "auth": "cookie"|"header"}` |
| 401 | `application/json` | `{"ok": false}` |

**Example**

```bash
curl http://localhost:5656/admin/api/check
# → {"ok":false}

curl -H "X-Admin-Key: mysecret" http://localhost:5656/admin/api/check
# → {"ok":true,"auth":"header"}
```

### Authentication Endpoints

#### `POST /admin/api/login`

Authenticates and sets the `admin_session` cookie.

**Request Body**

```json
{"password": "<admin_password>"}
```

**Response**

| HTTP Status | Content-Type | Body |
|-----|-----|--|
| 200 | `application/json` | `{"ok": true}` |
| 400 | `application/json` | `{"error": "Invalid request"}` |
| 401 | `application/json` | `{"error": "Invalid credentials"}` |

#### `POST /admin/api/logout`

Clears the session cookie.

**Response**

| HTTP Status | Content-Type | Body |
|-----|-----|--|
| 200 | `application/json` | `{"ok": true}` |

### Protected API Endpoints

#### `GET /admin/api/stats`

Returns aggregated metrics, system uptime, and active feature flags. Protected — requires authentication.

**Response**

| HTTP Status | Content-Type | Body |
|-----|-----|--|
| 200 | `application/json` | Metrics payload |

**Response Body**

```json
{
  "metrics": {
    "total_requests": 1230,
    "proxied_requests": 1100,
    "blocked_responses": 50,
    "pow_blocked": 30,
    "rate_limited": 20,
    "waf_blocked": {"sqli": 10, "xss": 5}
  },
  "uptime": "2h30m15s",
  "config": {
    "port": 5656,
    "use_tls": false,
    "waf_active": true,
    "pow_active": true,
    "rate_active": true
  },
  "errors": 3
}
```

#### `POST /admin/api/reset`

Resets all collected metrics. Protected after reset.

**Response**

| HTTP Status | Content-Type | Body |
|-----|-----|--|
| 200 | `application/json` | `{"status": "ok", "message": "Metrics reset successful"}` |

### Static Files

#### `GET /admin/{filename}`

Serves SPA files from `client/public/admin/`. Protected — redirects unauthenticated requests to `/admin/login`.

| File | Purpose |
|------|---------|
| `login.html` | Login page with inline CSS/JS |
| `dashboard.html` | Dashboard with stats, config, and WAF blocks display |

#### `GET /admin/static/{path}`

Serves shared static assets from `client/public/`. No authentication required.

---

## Security Headers

When `SEC_SSL_REDIRECT=true`, the `unrolled/secure` middleware adds:

| Header | Value |
|--------|-------|
| `Strict-Transport-Security` | Enforces HTTPS |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
