# TODO 

## Completed Items ✅

- [x] CSRF protection
    - [x] Use filesystem token to burn session
    - [x] Use redis store temporary session
- [x] Use the prefix as a part of stored session key
- [x] Fix Redis Get() hardcoded key bug
- [x] Fix WAF body restoration issue
- [x] Fix typos (gracefullShutdown, NewGerator, compelexity, ised)
- [x] Redis integration tests compilation fixed
- [x] WAF types consumer implemented
- [x] Error codes expanded
- [x] Rate limiting middleware added
- [x] Prometheus metrics endpoints added
- [x] Docker and docker-compose deployment support
- [x] TLS configuration for proxy transport
- [x] Structured JSON logging (log/slog)
- [x] Admin dashboard (/admin)

## Remaining Items 🚧

- [ ] Integration tests (Redis, WAF, end-to-end)
- [ ] OpenAPI/Swagger spec for API documentation
- [ ] Add more granular WAF rule categories
