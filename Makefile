.PHONY: build test lint clean sync predict stats missing backtest \
        server server-stop mobile-web mobile-ios mobile-android

BINARY := cash5
BUILD_DIR := bin

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/cli

test:
	go test ./... -count=1

test-verbose:
	go test ./... -v -count=1

test-race:
	go test -race ./... -count=1

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest"

clean:
	rm -rf $(BUILD_DIR) .cash5-data

# CLI shortcuts (requires build first)
sync: build
	./$(BUILD_DIR)/$(BINARY) sync

stats: build
	./$(BUILD_DIR)/$(BINARY) stats

predict: build
	./$(BUILD_DIR)/$(BINARY) predict

missing: build
	./$(BUILD_DIR)/$(BINARY) missing

backtest: build
	./$(BUILD_DIR)/$(BINARY) backtest

# ── API Server (local dev) ────────────────────────────────────────────────────
# Serves at http://localhost:8080
# Uses dev-token / admin-token for auth (see cmd/server/main.go)
server: sync
	LOCAL_DEV=true STORE_DIR=.cash5-data go run ./cmd/server

server-stop:
	pkill -f "go run ./cmd/server" || true

# ── Mobile App ───────────────────────────────────────────────────────────────
# Requires: npm install inside cash5-mobile/ (done once)
#
# iOS:  Open a NEW terminal and run `make mobile-ios`
#       — Expo will prompt to update Expo Go on the simulator; press Enter/y.
#       — After Expo Go is updated it opens automatically.
#
# Web:  Runs the app in your browser at http://localhost:8081 (no simulator needed)
MOBILE_DIR := cash5-mobile

mobile-web:
	cd $(MOBILE_DIR) && EXPO_NO_VERSION_COMPATIBILITY_CHECK=1 npx expo start --web

mobile-ios:
	cd $(MOBILE_DIR) && npx expo start --ios

mobile-android:
	cd $(MOBILE_DIR) && npx expo start --android

# ── Install development tools
tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
