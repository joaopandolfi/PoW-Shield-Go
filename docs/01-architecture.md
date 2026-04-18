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
- **Graceful Shutdown**: Proper cleanup on SIGINT/SIGTERM

## Architecture

```
                    Internet
                       |
                  [PoW Shield]
                       |
          +------------+------------+
          |           |             |
        [WAF]       [PoW]        [Static]
          |           |             |
       Proxy Layer -> [Backend]
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
      memory.go                  # In-memory LRU cache with GC
      redis.go                   # Redis backend
      safe.go                    # Generic type-safe wrapper
      injectable.go              # Late-init panic recovery
      entities.go                # Internal stored struct
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
| Validation | go-playground/validator |
| Validation | go-playground/validator v9 |
| UUID | google/uuid |
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
```
