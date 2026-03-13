# Architecture Overview

## System Summary

Texas Lottery Cash Five analysis app. Ingests historical draws (1995–present), syncs daily, generates 5 statistically ranked candidate combinations per draw day. Delivered via Go REST API on AWS + Expo React Native mobile app.

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Go CLI + Ingestion MVP | **In Progress** |
| 2 | Smart Incremental Sync | Planned |
| 3 | Statistical Engine | **In Progress** |
| 4 | User Accounts + Auth | Planned |
| 5 | Entitlement Model | Planned |
| 6 | LLM Explanations | Planned |
| 7–9 | ML Experiments | Future |
| 10 | REST API | Planned |
| 11 | CI/CD | Planned |
| 12 | Expo React Native | Future |
| 13 | Billing Integration | Future |
| 14–17 | Production + Stores | Future |

## Core Components

### Data Pipeline
- **Source:** `cashfive.csv` from Texas Lottery (primary), HTML fallback
- **Ingestion:** `internal/ingestion` — CSV parsing, validation, reconciliation
- **Store (CLI):** `internal/store` — local JSON for dev; DynamoDB for production
- **Schedule:** EventBridge Mon–Sat 11 PM CT, reconciliation Sunday 1 AM CT

### Statistical Engine
- **Stats:** `internal/stats` — frequency, rolling windows (30/60/90/180), gap analysis, distribution scoring
- **Prediction:** `internal/prediction` — full C(35,5)=324,632 enumeration, composite scoring, diversity-constrained top-5

### Scoring Formula
```
score = 0.25*freq + 0.30*recency + 0.15*overdue + 0.20*distribution + 0.05*sum_align - 0.05*consecutive
```
All weights configurable via SSM Parameter Store (production).

### Model Providers
- `statistical` — rule-based template explanations (MVP, zero cost, always available)
- `claude` — LLM narrative explanations (Phase 6, Pro tier only)
- `openai` — OpenAI fallback provider (Phase 6)
- `ml_rank` — classical ML candidate re-ranking (Phase 7 experiment)

## AWS Architecture (Production)

```
CloudFront + WAF
  └── API Gateway (HTTP API)
        └── Lambda: api-handler

EventBridge Scheduler
  ├── Lambda: ingestion-job (Mon-Sat 11 PM CT)
  └── Lambda: reconciliation-job (Sun 1 AM CT)

DynamoDB Tables:
  draws | users | subscriptions | predictions
  pretest_results | sync_state | stats_cache | audit_log

S3 Buckets:
  raw-snapshots | model-artifacts | exports

Cognito User Pools → JWT auth
Secrets Manager → Stripe, LLM API keys
SSM Parameter Store → scoring weights, config
```

Estimated MVP cost: **$10–25/month** at low traffic (no always-on servers).

## Key Design Decisions
- See [ADR 001](../adr/001-csv-primary-source.md) — CSV as primary source
- See [ADR 002](../adr/002-dynamodb-over-rds.md) — DynamoDB over RDS
- See [ADR 003](../adr/003-modular-monolith.md) — Modular monolith

## Honest Disclaimer
Cash Five draws are cryptographically random. C(35,5) = 324,632 combinations. The statistical engine ranks candidates by historical patterns only — it cannot predict future draws. All user-facing surfaces must display this prominently.
