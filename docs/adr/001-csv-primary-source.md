# ADR 001: CSV as Primary Data Source

## Status
Accepted

## Context
Texas Lottery publishes Cash Five historical draws in multiple formats:
- A downloadable CSV file (`cashfive.csv`)
- An HTML page with year-tab navigation (`Winning_Numbers/index.html`)
- Individual draw detail pages

We need a reliable, maintainable strategy for ingesting ~9,700+ historical draws.

## Decision
Use the CSV as the primary and authoritative ingestion source. Use HTML scraping only as a fallback when the CSV download fails.

## Rationale
1. A single HTTP request downloads all ~9,700 rows — no pagination needed
2. Structured format eliminates HTML parsing fragility
3. Official state agency source — most authoritative
4. Trivially cacheable to S3 for audit trail and re-processing
5. CSV parsing is testable with static fixtures

## Consequences
- A change to the CSV URL or format will break primary ingestion — mitigated by S3 archiving and HTML fallback
- HTML fallback code must be maintained even if rarely used
- Must validate column order and header row presence before assuming format
