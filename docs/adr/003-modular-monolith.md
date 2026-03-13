# ADR 003: Modular Monolith Over Microservices

## Status
Accepted

## Context
The system needs: daily ingestion jobs, a REST API, prediction generation, and optional ML/LLM features. Architecture options: microservices, monolith, modular monolith.

## Decision
Build a modular monolith — one Go repository with well-defined internal package boundaries — deployed as a small number of Lambda functions.

## Rationale
1. **Solo developer velocity** — microservices multiply operational overhead (separate deployments, service discovery, distributed tracing)
2. **Lambda boundaries are sufficient** — `api-handler`, `ingestion-job`, `prediction-job`, and `billing-webhook` cover all deployment separation needed
3. **Refactorable** — clean internal package interfaces make extraction to separate services possible later without rewriting
4. **Testable** — Go internal packages can be unit tested independently
5. **C(35,5) = 324,632** — the prediction workload fits comfortably in a single Lambda invocation

## Consequences
- All Lambda functions share one binary (fat binary trade-off — acceptable for this scale)
- If ML inference requires GPU, the `model` package would need extraction to ECS — interfaces are designed to allow this
- Long ingestion backfills (>15 min Lambda limit) would need Step Functions — mitigated by incremental sync design
