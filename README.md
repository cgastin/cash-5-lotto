# Cash Five Lotto

A data-driven analysis tool for the Texas Lottery Cash Five game. The app ingests all historical drawings (1995–present), syncs new results daily, and generates 5 statistically ranked number combinations per draw day based on historical frequency, recency, gap analysis, and distribution patterns.

**What it is not:** A prediction tool. Lottery drawings are random. The candidates are ranked using statistical patterns from historical data — not gambling advice.

---

## Stack

- **Backend** — Go 1.26, plain HTTP server locally / AWS Lambda in production
- **Mobile** — Expo SDK 55 (React Native), iOS + Android + web
- **Storage** — Local JSON files for development, DynamoDB in production

---

## Prerequisites

### Go
Install Go 1.22 or later: https://go.dev/dl/

Verify:
```bash
go version
```

### Node.js
Install Node.js LTS (v20.19.4 or later): https://nodejs.org/en/download

Verify:
```bash
node --version
npm --version
```

### iOS Simulator (macOS only)
1. Install **Xcode** from the Mac App Store (free, ~15 GB)
2. Open Xcode → **Settings → Platforms** → download an iOS simulator runtime
3. Accept the license if prompted:
   ```bash
   sudo xcodebuild -license accept
   ```
4. Install Xcode Command Line Tools:
   ```bash
   xcode-select --install
   ```

### Android Emulator
1. Install **Android Studio**: https://developer.android.com/studio
2. Open Android Studio → **More Actions → Virtual Device Manager → Create Device**
3. Pick a device (e.g. Pixel 8), select an API 34+ system image, and click Finish
4. Add the Android SDK tools to your shell profile (`~/.zshrc` or `~/.bash_profile`):
   ```bash
   export ANDROID_HOME=$HOME/Library/Android/sdk
   export PATH=$PATH:$ANDROID_HOME/emulator
   export PATH=$PATH:$ANDROID_HOME/platform-tools
   ```
5. Reload your shell: `source ~/.zshrc`

### Expo Go (optional — physical device)
Install the **Expo Go** app on your iOS or Android device from the App Store / Google Play. Scan the QR code that appears when you run `make mobile-ios` or `make mobile-android`.

---

## Running Locally

### 1. Install Go dev tools (first time only)
```bash
make tools
```

### 2. Install mobile dependencies (first time only)
```bash
cd cash5-mobile && npm install && cd ..
```

### 3. Start the backend server

The server syncs the latest draw history from the Texas Lottery on startup, then serves the REST API at `http://localhost:8080`.

```bash
make server
```

Leave this running in its own terminal tab.

**Dev auth tokens** — pass one of these as a Bearer token in API requests:
| Token | Role |
|---|---|
| `dev-token` | Regular user (Pro plan in dev) |
| `admin-token` | Admin user (Pro plan + admin routes) |

### 4. Start the mobile app

Open a **second terminal tab** and run one of:

**iOS Simulator** (macOS only — requires Xcode):
```bash
make mobile-ios
```

**Android Emulator** (requires Android Studio + a running emulator):
```bash
# Start your emulator first from Android Studio → Virtual Device Manager
make mobile-android
```

**Web browser** (no simulator needed — fastest for development):
```bash
make mobile-web
# Opens at http://localhost:8081
```

> **New file not appearing?** Expo caches its bundle. If you add a new screen or component and it doesn't show up, restart Metro with the cache cleared:
> ```bash
> cd cash5-mobile && npx expo start --clear
> ```

---

## Project Structure

```
cash-5-lotto/
├── cmd/
│   ├── cli/          # CLI tool (sync, stats, predict, missing, backtest)
│   └── server/       # HTTP/Lambda server entry point
├── internal/
│   ├── api/          # HTTP handlers and router
│   ├── auth/         # JWT middleware, entitlement resolution
│   ├── ingestion/    # CSV download, parsing, sync strategy
│   ├── model/        # Scoring provider interface (statistical, LLM stubs)
│   ├── prediction/   # C(35,5) enumeration, scoring, top-5 selection
│   ├── stats/        # Frequency, gap, distribution, composite scorer
│   └── store/        # Repository interfaces + local JSON implementation
├── cash5-mobile/     # Expo React Native app
│   ├── app/(tabs)/   # Tab screens: Home, Picks, Stats, History
│   └── src/
│       ├── api/      # Typed API client
│       ├── components/
│       ├── constants/
│       └── hooks/
└── terraform/        # AWS infrastructure (Lambda, DynamoDB, API Gateway, etc.)
```

---

## CLI Commands

The CLI operates directly on the local data store without running the server.

```bash
make build       # compile CLI binary → bin/cash5

make sync        # download latest draws from Texas Lottery
make stats       # print frequency statistics
make predict     # generate today's top-5 candidate combinations
make missing     # list draw dates with no stored result
make backtest    # run walk-forward backtesting
```

---

## API

Base URL (local): `http://localhost:8080`

All endpoints except `GET /v1/health` require `Authorization: Bearer <token>`.

| Method | Path | Description |
|---|---|---|
| GET | `/v1/health` | Server + sync status |
| GET | `/v1/drawings/latest` | Most recent draw |
| GET | `/v1/drawings` | Draw history (`?from=&to=` dates) |
| GET | `/v1/predictions/latest` | Today's ranked candidates |
| POST | `/v1/predictions/clear` | Re-generate today's picks |
| GET | `/v1/predictions/performance` | Match history (`?days=30`) |
| GET | `/v1/stats/frequencies` | Number frequencies (`?window=all\|30\|60\|90\|180`) |

---

## Tests

```bash
make test         # run all tests
make test-race    # run with race detector
make lint         # go vet + staticcheck
```

Run a single package:
```bash
go test ./internal/prediction/... -v -count=1
```
