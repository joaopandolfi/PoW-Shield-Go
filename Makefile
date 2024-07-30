
.PHONY: front-build-tools
front-build-tools:
	@echo "installing nodejs libs"
	@cd client && npm install
	@echo "Sucess!"

.PHONY: front-build
front-build:
	@echo "generate bundle"
	@cd client && npm run build


.PHONY: front-build-beauty
front-build-beauty:
	@echo "generate bundle without uglify"
	@cd client && npm run build

.PHONY: build-backend
build-backend:
	@cd server && go get && go build

.PHONY: run-backend
run-backend: 
	@cd server && ./pow-shield-go