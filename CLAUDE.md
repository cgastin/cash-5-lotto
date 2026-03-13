# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Go backend
make build          # build CLI binary → bin/cash5
make test           # go test ./... -count=1
make test-verbose   # with -v
make test-race      # with -race
make lint           # go vet + staticcheck
make server         # sync CSV then run dev API server at :8080 (LOCAL_DEV=true)

# Run a single test package
go test ./internal/prediction/... -count=1 -v

# CLI (requires make build first)
make sync           # download + upsert draws
make predict        # generate top-5 candidates
make stats          # print frequency stats
make missing        # list missing draw dates
make backtest       # run walk-forward backtest

# Mobile app (run from repo root)
make mobile-ios     # Expo iOS simulator
make mobile-web     # Expo web at :8081 (no simulator required)

# Add dev tooling
make tools          # installs staticcheck + golangci-lint
```

**Dev auth tokens** (accepted by MockVerifier when `LOCAL_DEV=true`):
- `Authorization: Bearer dev-token` → regular user (free plan)
- `Authorization: Bearer admin-token` → admin user

**Mobile**: when adding new screens/files, always restart Metro with `--clear` (`npx expo start --clear`) or the new file won't be picked up.

## Architecture

### Two entry points, one shared router

- `cmd/cli/` — standalone CLI that directly calls `ingestion`, `stats`, and `prediction` packages
- `cmd/server/` — HTTP server (plain HTTP in `LOCAL_DEV=true`, AWS Lambda adapter when built with `lambda` tag). Both share `internal/api` handlers wired through `api.Dependencies`.

### Backend layer cake

```
cmd/server/main.go   → wires Dependencies (repos, auth, syncFunc)
internal/api/        → HTTP handlers, router (no business logic)
internal/prediction/ → enumerate C(35,5), score, diversity-select top-5
internal/stats/      → frequency windows, gap, distribution, composite scorer
internal/ingestion/  → CSV download/parse, HTML fallback scraper, sync strategy
internal/model/      → ModelProvider interface; StatisticalProvider (MVP); stubs for Claude/OpenAI/ML
internal/auth/       → JWT verification (Cognito prod, MockVerifier dev), entitlement resolution
internal/store/      → repository interfaces + LocalRepository (file-backed JSON, dev only)
```

### Repository pattern

`internal/store/interfaces.go` defines all repository interfaces. `internal/store/local.go` has file-backed JSON implementations (`LocalDrawRepository`, `LocalSyncStateRepository`, `LocalPredictionRepository`) stored under `.cash5-data/`. DynamoDB implementations are a future Phase 4+ task. The server stubs `UserRepository` and `SubscriptionRepository` in local dev.

### Prediction pipeline

1. `prediction.GenerateTop5(draws, weights)` — filters to pool-35 draws only, calls `stats.ComputeFeatures`, enumerates all 324,632 C(35,5) combos, scores with composite formula, diversity-selects top 5 (max 2 overlapping numbers between any two picks)
2. `model.StatisticalProvider.GenerateExplanation` — narrative from feature breakdown (no LLM yet)
3. `store.Prediction` — persisted with `Version` int (v1 on first view, v2+ after "Clear & Re-pick")

**Scoring weights** (configurable via `prediction.DefaultWeights()`): `0.25*allTimeFreq + 0.30*rolling30 + 0.15*overdue + 0.20*distribution + 0.05*sumAlignment - 0.05*consecutive`

### Pool size split

Cash Five changed from 1–39 to 1–35 on `2018-09-21` (`store.PoolSwitchDate`). All scoring and enumeration uses pool-35 draws only. `store.DrawPoolSize(date)` returns the correct pool for a given date.

### Auth & entitlements

`auth.Middleware` reads the JWT, resolves the subscription, and attaches `auth.Entitlements` to the request context. Handlers call `auth.EntitlementsFromContext(ctx)`. Plan limits: Free = 1 candidate, Plus/Pro = 5 (via `auth.MaxCandidates(plan)`). Admin routes require `auth.RequireAdmin` middleware wrapping.

### Mobile app (`cash5-mobile/`)

Expo SDK 55, expo-router file-based routing, TypeScript. All API calls go through `src/api/client.ts` (`api` object + typed response interfaces). State management via `src/hooks/useApi.ts` hook (returns `{ data, loading, error, reload }`). Auth token set globally via `setAuthToken()`.

Tab screens live in `app/(tabs)/`. Constants (colors, disclaimer text, API base URL) in `src/constants/index.ts`.

### Infrastructure

Terraform in `terraform/` with per-service modules (`lambda/`, `dynamodb/`, `api-gateway/`, `cognito/`, `eventbridge/`, `s3/`). Environments in `terraform/environments/{dev,prod}/`. CI uses OIDC for AWS auth (no long-lived keys). Dev auto-deploys on push to main; prod requires a version tag + manual approval gate via GitHub environment `prod`.
