
.PHONY: build-front-tools
GO=env -u GOROOT go

.PHONY: build-front-tools
front-build-tools:
	@echo "installing nodejs libs"
	@cd client && npm install
	@echo "Sucess!"

.PHONY: build-front
build-front:
	@echo "generate bundle"
	@cd client && npm run build

.PHONY: build-front-beauty
build-front-beauty:
	@echo "generate bundle without uglify"
	@cd client && npm run build

.PHONY: build-backend
build-backend:
	@cd server && $(GO) get && $(GO) build

.PHONY: test-backend
test-backend:
	@cd server && $(GO) test ./...

.PHONY: run-backend
run-backend: 
	@cd server && ./pow-shield-go

.PHONY: check-vuln
check-vuln:
	@cd server && env -u GOROOT govulncheck ./...
