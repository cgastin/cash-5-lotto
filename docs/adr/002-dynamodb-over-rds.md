# ADR 002: DynamoDB Over RDS for Primary Storage

## Status
Accepted

## Context
We need persistent storage for ~9,700 lottery draws, user accounts, subscriptions, and predictions. Options considered: RDS (PostgreSQL), DynamoDB, SQLite (local).

## Decision
Use DynamoDB (on-demand capacity) as the primary production store. Use local JSON files for CLI/development.

## Rationale
1. **Zero idle cost** — DynamoDB on-demand charges per request; ~9,700 rows at rest costs $0
2. **No always-on server** — eliminates the $15+/month minimum RDS cost for MVP
3. **Lambda-native** — no connection pooling issues, works perfectly with serverless
4. **Access patterns fit** — all primary patterns are simple key lookups (by draw date, by user ID)
5. **Managed** — backups, encryption, and replication are built in

## Consequences
- No SQL joins — data access patterns must be designed upfront
- Ad-hoc analytical queries require DynamoDB Scan or a separate analytics layer
- Complex reporting (e.g., backtesting history) must be done application-side in Go
- Migration to RDS possible later if query patterns exceed DynamoDB's capabilities
