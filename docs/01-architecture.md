# PoW-Shield-Go

Proof of work shield written in Golang to protect DDoS attacks.

## Description

PoW Shield is a DDoS protection solution for the OSI application layer, functioning as a reverse proxy that employs proof-of-work to secure communication between the backend service and the end user. This project offers an alternative to traditional anti-DDoS methods, such as Google's ReCaptcha, which are often cumbersome for users. With PoW Shield, accessing a protected web service is seamless: just navigate to the URL, and the browser handles the verification process automatically.

## Key Features

- **Proof of Work Mechanism**: Uses computational challenges (SHA256 leading zero bit puzzles) to verify legitimate users and deter attackers
- **User-Friendly**: Eliminates the need for users to solve complex captchas; verification happens automatically in the browser
- **Seamless Integration**: Easily integrates with existing backend services as a reverse proxy
- **Reverse Proxy**: Forwards legitimate requests to a protected backend server
- **WAF (Web Application Firewall)**: 120 built-in rules covering XSS, SQLi, RCE, LFI/RFI, command injection, and more
- **Multi-Instance Syncing**: Supports Redis for distributed cache across multiple instances
- **Dual Cache Backends**: In-memory (with LRU eviction) and Redis
- **Cookie/Session/Header Transport**: Flexible session token transport mechanisms
- **SSL/TLS Support**: Optional TLS termination
- **IP Tolerance System**: Tracks failed attempts per IP and applies progressive punishment (difficulty increase)
- **Encrypted Client Contract**: Clients send `gpg_encrypt(version:uuid:timestamp)` in a header; the WAF decrypts it, rejects stale or replayed requests and gates each client id with a PoW challenge whose difficulty grows on abuse (see [sprint1.md](sprint1.md))
- **Graceful Shutdown**: Proper cleanup on SIGINT/SIGTERM

## Architecture

```
                    Internet
                       |
                  [PoW Shield]
                       |
     +-----------+-----+-----+-----------+
     |           |           |           |
 [Contract]    [PoW]       [WAF]     [Static]
     |           |           |
     +-----> Rate Limit -> Proxy Layer -> [Backend]

 Contract: decrypts X-Request-Context (OpenPGP), replay check,
           per-client PoW challenge, state in memory or Redis
```

### Component Structure

```
server/
  main.go                        # Entry point, server lifecycle
  config/
    config.go                    # Config struct, env loading, validation
    utils.go                     # Generic Str[T] converters
  internal/
    cache/
      cache.go                   # Cache interface, initialization
      memory.go                  # In-memory TTL cache with GC (only expired entries are evicted)
      redis.go                   # Redis backend (SET NX, Lua INCR, GETDEL for atomic ops)
      safe.go                    # Generic type-safe wrapper
      injectable.go              # Late-init panic recovery
      entities.go                # Internal stored struct
      atomic_test.go             # Concurrency tests for PutIfAbsent/IncrIfExists/GetDel
      *(test).go                 # Unit/integration tests
    request/
      request.go                 # HTTP proxy request builder
  models/
    permissions.go               # Permission constants
    domain/
      challenge.go               # Challenge state machine
      session.go                 # Session token management
      cookie.go                  # Cookie struct
      waf.go                     # WAF rule domain model
  services/
    contract/
      contract.go                # WAF key loading, header decrypt, payload validation
    pow/
      generator.go               # PoW problem generator
      verifier.go                # PoW nonce verifier
      entities.go                # Shared constants
    utils/
      encoding.go                # Base64 helpers
      time.go                    # Millisecond timestamp ops
      hash.go                    # SHA256 + complexity checking
  web/
    server/server.go             # HTTP server wrapper
    errors.go                    # Error codes (minimal)
    router/router.go             # Route registration, middleware setup
    middleware/
      waf.go                     # WAF inspection middleware
      pow.go                     # PoW session validation middleware
      contract.go                # Encrypted contract + per-client challenge middleware
      contract_test.go           # Full contract flow on memory and Redis (miniredis)
      rate_limit.go              # Per-IP rate limit
      identificator.go           # Client IP hashing middleware
      commons.go                 # Shared helpers (cleanAll, blockRequest)
      middleware.go              # Init functions
    controllers/
      controller.go              # Controller interface
      pow/
        pow.go                   # Challenge/Verify endpoints
        routers.go               # /pow route setup
        entities.go              # Request/response payloads
      proxy/
        proxy.go                 # Reverse proxy handler
        routers.go               # / route setup
      health/
        health.go                # Health check endpoint
        routes.go                # /health route setup
      static/
        router.go                # Static file serving
    handler/
      request.go                 # Request extraction helpers
      response.go                # JSON response writer
      session.go                 # CookieStore session management
      cookie.go                  # powShield cookie management
  e2e/
    e2e_test.go                  # E2E client (build tag e2e), acts as the front
    keygen/main.go               # Throwaway WAF key pair for the e2e stack
e2e/
  run.sh                         # Builds the docker stack, runs e2e, tears down
  docker-compose.e2e.yml         # 2 WAF instances + Redis + http-echo
client/
  solver/
    solver.js                    # Client-sidenonce solver
    utils.js                     # SHA256/timestamp helpers
    bundle.js                    # Browserify entry point
  public/
    index.html                   # Welcome/verification page
  package.json                   # Build scripts (Browserify + UglifyJS)
wafRules.json                    # 120 WAF rules (regex patterns)
wafTypes.json                    # Rule type mapping (unused)
.env.example                     # Environment variable documentation
Makefile                         # Build targets
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.22.4 |
| Router | gorilla/mux |
| Cache | go-redis/redis v8 |
| Sessions | gorilla/sessions (CookieStore) |
| Security | unrolled/secure |
| Validation | go-playground/validator v9 |
| UUID | google/uuid |
| OpenPGP | ProtonMail/go-crypto |
| Tests | miniredis (in-process Redis), Docker compose e2e |
| Client JS | Browserify, create-hash, UglifyJS |
| WAF Engine | blackwhale (github.com/joaopandolfi/blackwhale) |

## Quick Start

```sh
# Backend
cp .env.example server/.env
make build-backend
make run-backend

# Frontend
make build-front-tools
make build-front

# Test
# Access http://localhost:5656/welcome
make test-backend
make test-e2e                    # docker: 2 WAF instances + Redis + backend, contract flow
```

## Documentation

| Doc | Content |
|-----|---------|
| [02-implementation-status.md](02-implementation-status.md) | Component status |
| [03-bugs-and-issues.md](03-bugs-and-issues.md) | Bugs found and fixed |
| [04-api-reference.md](04-api-reference.md) | Endpoints and middleware chain |
| [05-configuration-reference.md](05-configuration-reference.md) | Environment variables |
| [06-todo-and-recommendations.md](06-todo-and-recommendations.md) | Roadmap |
| [07-contract-payloads.md](07-contract-payloads.md) | Contract headers, payloads and responses |
| [sprint1.md](sprint1.md) | Encrypted contract: design, Redis, e2e, limitations |
| [sprint2.md](sprint2.md) | Per-uuid rate limit, atomic difficulty, request binding |
