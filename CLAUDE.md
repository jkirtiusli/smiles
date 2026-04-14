# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
# Build both binaries
go build -o smiles ./cmd/cli
go build -o smiles-mcp ./cmd/mcp

# Run all tests
go test ./...

# Run single package or test
go test ./client
go test ./client -run TestSearchFlights
```

## Environment Variables

- `SMILES_API_KEY` (required) - Smiles API key
- `SMILES_BEARER_TOKEN` (optional) - Bearer token for additional auth

## CLI Usage

```bash
# One-way, multiple destinations, up to 31 days
smiles EZE MAD,BCN,FCO 2026-06-01 30

# Round-trip
smiles EZE PUJ 2026-05-15 2026-05-25 10
```

## Architecture

The project has two entry points sharing a common client:

- **cmd/cli/** - CLI tool that prints cheapest flights to stdout
- **cmd/mcp/** - MCP server exposing 3 tools (`search_flights`, `find_cheapest_flights`, `get_flight_taxes`) for LLMs
- **client/** - HTTP client that calls the Smiles API
- **server/** - MCP tool definitions and handlers, wraps client methods
- **model/** - API response structs with custom JSON unmarshalers (e.g., `FlexFloat64` handles the API returning numbers as either floats or strings)

## Key Design Decisions

**Akamai bypass**: The Smiles API is behind Akamai Bot Manager. The client uses `tls-client` (bogdanfinn) to impersonate Chrome 124's TLS and HTTP/2 fingerprint. The critical headers are `sec-ch-ua`, `sec-fetch-*`, and `sec-ch-ua-platform` - without these, requests return 406 even with correct TLS fingerprinting.

**Dual HTTP client**: `SmilesClient` has two fields: `tlsClient` (tls-client, used in production via `doChromeRequest`) and `httpClient` (standard `*http.Client`, used in tests via `doStandardRequest`). The `New()` constructor sets up tls-client; tests create `SmilesClient` directly with `httpClient` set to `httptest.Server.Client()`.

**Concurrency control**: `FindCheapestFlights` uses a semaphore (`chan struct{}` with capacity 3) to limit concurrent API requests. Individual date failures are skipped rather than aborting the entire search.

**API host**: `api-air-flightsearch-green.smiles.com.ar` (migrated from the old `api-air-flightsearch-prd.smiles.com.br`). The tax endpoint still uses the old `.com.br` host.

## Language

The codebase and CLI output are in Spanish (es-AR). Keep CLI messages, errors, and comments in Spanish.

## Documentation

When making changes, update CLAUDE.md and README.md if the change affects build commands, CLI usage, architecture, environment variables, or key design decisions.
