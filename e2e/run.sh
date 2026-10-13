#!/usr/bin/env bash
# End-to-end test of the encrypted contract: builds and starts the docker stack
# (2 WAF instances + redis + http-echo backend), runs the Go e2e client, tears down.
# KEEP_UP=1 leaves the stack running.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
compose=(docker compose -p pow-shield-e2e -f docker-compose.yml -f e2e/docker-compose.e2e.yml)

cleanup() {
	if [ "${KEEP_UP:-0}" != 1 ]; then
		cd "$root" && "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

mkdir -p e2e/.keys
(cd server && go run ./e2e/keygen ../e2e/.keys)

"${compose[@]}" up -d --build

for port in 5656 5657; do
	for _ in $(seq 1 60); do
		curl -sf "http://localhost:$port/health" >/dev/null && break
		sleep 1
	done
	curl -sf "http://localhost:$port/health" >/dev/null || {
		"${compose[@]}" logs pow-shield pow-shield-2 | tail -50
		echo "instance on $port did not come up" >&2
		exit 1
	}
done

cd server
E2E_URLS="http://localhost:5656,http://localhost:5657" \
	go test -tags e2e -count=1 -v ./e2e/
