# Agents.md — Vessel Database Library

**Project:** Vessel — multi-database SQL toolkit for Go services  
**Language:** Go 1.26+  
**Current version:** v0.2.0  
**Last updated:** 2026-07-08

Instructions for AI agents and automation working on this repository.
Human contributors should read [CONTRIBUTING.md](CONTRIBUTING.md); this file
condenses the facts an agent needs to work correctly without re-deriving them.

---

## What Vessel Is

A layered SQL toolkit that sits between raw `database/sql` and a full ORM:
fluent query builders, dialect-aware SQL generation, typed row scanning,
transactions with savepoints, OpenTelemetry tracing, retry helpers, and an
optional manager for read/write routing across multiple databases.

Layering (dependencies flow left to right, never back):

```text
condition DSL → query builder → SQL dialect → driver (MySQL/Postgres/SQLite/MSSQL)
```

Supported dialects: MySQL, PostgreSQL (pgx), SQLite (modernc, pure Go), MSSQL.
New features must work on all four or return an explicit "unsupported" error —
never emit silently broken SQL.

---

## Project Structure

```text
vessel/
├── db/v1/                  # Public driver API
│   ├── db.go               # DB/Tx interfaces, NewDB factory
│   ├── mysql.go, postgres.go, sqlite.go, mssql.go   # Configs + driver impls
│   ├── fluentDB.go         # Fluent builders (Select/Insert/Update/Delete)
│   ├── rows_scanning.go    # Typed scanning (ScanRowsTo, ScanAll, ScanOne)
│   ├── row_adapter.go      # Streaming row access; rows_pool.go = pooling
│   ├── logging.go          # SafeLogger, error classification, sanitizers
│   ├── logger.go           # db.Logger interface; logger_adapters.go
│   ├── transaction.go      # WithTransaction, panic-safe rollback
│   └── dberror/            # Sentinel errors + per-driver error mappers
├── internal/pkg/
│   ├── builder/            # SQL query builder (per-dialect files)
│   ├── sqldialect/         # Dialect quoting/placeholders/capabilities
│   ├── operator/, helpers/, otel/
├── pkg/
│   ├── query/condition/    # Condition DSL (Expr, And, Or, In, Between)
│   ├── query/options/      # QueryOptions (OrderBy, Limit, Returning…)
│   ├── query/definition/   # Driver-name constants
│   └── retry/              # Backoff strategies (fixed/linear/exponential…)
├── manager/v1/             # DBManager: routing, workers, health, batching
│   └── config/             # Manager YAML/JSON/TOML config + validation
├── tests/                  # Integration tests (build tag: integration)
├── examples/               # Runnable examples
└── docs/                   # Guides (see table at the bottom)
```

---

## Build, Test, Lint

**Unit tests are gated behind the `test` build tag.** A plain `go test ./...`
reports "no test files" — this is the single most common agent mistake here.

```bash
make test                        # unit tests (adds -tags=test)
go test -tags test -race ./...   # equivalent, explicit
make coverage                    # unit tests + coverage report
make lint                        # golangci-lint (40+ linters)
make fmt-check                   # gofmt/goimports verification

# Integration tests (build tag: integration; SQLite needs no containers)
make integration-test-sqlite
make integration-test-all        # requires Docker (MySQL/Postgres/MSSQL)
DB_TYPE=mysql go test -tags=integration ./tests -run TestIntegration
```

A pre-commit hook runs golangci-lint; commits with lint findings are rejected.
Fix findings rather than suppressing them — staticcheck suggestions
(De Morgan simplifications, `strconv` over `fmt.Sprintf`, etc.) are enforced.

---

## Conventions

**Commits** — `<type>: <Capitalized description>` with types
`feat fix refactor docs test chore perf ci`. Body explains *why*.
One logical change per commit; refactors separate from behavior changes.
Breaking changes use `refactor!:`/`feat!:` and a `BREAKING CHANGE:` footer.

**Changelog** — every user-visible change adds an entry under `## [Unreleased]`
in [CHANGELOG.md](CHANGELOG.md) (Keep a Changelog format) in the same commit.

**TDD** — for bug fixes, write the failing regression test first (RED), then
fix (GREEN). New code targets 80% coverage. Table-driven tests with `t.Run`
subtests; mocks are generated with gomock (`make mocks`).

**API stability** — exported API changes are breaking. Prefer additive
changes; when something must break, migrate all examples/tests in the same
commit and document the migration in the changelog.

---

## Current API Facts (v0.2.0)

Post-0.2.0 signatures agents get wrong when trained on older code:

- `db.NewFluentDB(db)` takes **one** argument (the old `(db, ctx)` form is
  gone; `ctx` goes to terminal calls like `Get(ctx)`).
- `DBManager.QueryWithRetry(ctx, cfg, fn)` — `fn` is
  `func(ctx context.Context, attempt int) ([]map[string]any, error)`.
  `MultiEntryQuery` and `QueryWithCustomRetry` **no longer exist**; pass a
  custom strategy via `QueryWithRetryConfig{Strategy: …}`.
- `SafeLogger.Debug(msg string)` — single message, no variadic strings.
- Manager config is validated fail-fast: every entry needs a unique `name`
  and a `type` of `readonly`/`readwrite`; violations error at load.
- Dialect capabilities come from the `sqldialect.CapabilityProvider`
  interface (`Capabilities()`); dialect identity is checked by **type**,
  never by inspecting `fmt.Sprintf("%T", …)` strings.
- `ORDER BY` directions are validated at the SQL-generation layer:
  only `ASC`/`DESC` (case-insensitive, empty = `ASC`).

---

## Task Recipes

### Bug fix

1. Reproduce with a failing test in the affected package (RED).
2. Fix; run `make test && make lint`.
3. Changelog entry + one commit. Update docs if behavior changed.

### New query feature (e.g., a new option)

1. Add the field to `pkg/query/options/`.
2. Add fluent methods in `db/v1/fluentDB.go` (return the builder for chaining).
3. Generate SQL in `internal/pkg/sqldialect/sql_dialect.go`
   (`supportedOptions`); handle per-dialect syntax (MSSQL OFFSET/FETCH…).
4. If a dialect can't support it, return an explicit error — check via
   `Capabilities()`, and extend the `Capabilities` struct if needed.
5. Test across all four dialects; update `docs/PORTABILITY_MATRIX.md`.

### New operator

1. Define in `internal/pkg/operator/`; map per dialect in
   `internal/pkg/sqldialect/`.
2. Expose in `pkg/query/condition/`; test; update
   `docs/OPERATORS_COMPATIBILITY.md`.

### New dialect (rare)

Follow the four existing pairs: config+driver in `db/v1/`, dialect in
`internal/pkg/sqldialect/` (must implement `CapabilityProvider`), builder
support in `internal/pkg/builder/`, error mapper in `db/v1/dberror/`,
integration tests in `tests/`.

---

## Pitfalls (learned the hard way — do not reintroduce)

- **Never `fmt.Errorf("…: %w", err)` outside an `if err != nil` branch.**
  Wrapping nil produces a *non-nil* error (`%!w(<nil>)`). This shipped as a
  real bug in the retry helpers.
- **Never use `reflect.Value.Convert` to turn integers into strings.**
  `int64(65) → "A"` (rune conversion), not `"65"`. Numeric→string scanning
  formats decimal text explicitly (`rows_scanning.go`).
- **Field-map precedence follows `encoding/json`:** shallower (outer) struct
  fields shadow embedded ones; same-depth collisions are dropped as ambiguous.
- **Error classification is typed-first:** check `dberror` sentinels with
  `errors.Is`/`errors.As` before any message-substring matching, and keep
  substring needles narrow (a bare `"invalid"` or `"near"` misclassifies).
- **Identifiers vs values:** values are always parameterized through dialect
  placeholders; identifiers go through `QuoteIdentifier`. Raw helpers
  (`ColumnRaw`, `HavingRaw`, raw queries) are caller-owned escape hatches —
  never route user input into them.
- **Compile regexes at package init**, not per call, in hot paths.
- **Don't detect types by their printed names** (`%T` + `strings.Contains`);
  use type switches or an interface method.

---

## Release Process

Releases are tag-driven via `.github/workflows/release.yml`:

1. Retitle `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` in CHANGELOG.md
   (the workflow fails without a matching section).
2. Merge to `main`, then tag: `git tag -a vX.Y.Z && git push origin vX.Y.Z`.
3. The workflow validates (lint, coverage, SQLite integration), packages a
   source archive, creates the GitHub Release from the changelog section,
   and warms the module proxy.

Breaking changes require at least a minor bump while on v0.x.

---

## Documentation Map

| File | Purpose |
| --- | --- |
| `README.md` | Overview, quick start |
| `CONTRIBUTING.md` | Human contributor guide, test/lint workflow |
| `CHANGELOG.md` | Version history (Keep a Changelog) |
| `docs/ARCHITECTURE.md` | Layering and design notes |
| `docs/FLUENTDB.md` | Fluent builder guide |
| `docs/DB_MANAGER.md` | Manager routing, workers, config |
| `docs/CONFIGURATION.md` / `docs/ENVIRONMENT_VARIABLES.md` | Config reference |
| `docs/ERROR_HANDLING.md` | Sentinel errors, classification |
| `docs/LOGGING.md` / `docs/OBSERVABILITY.md` | Logger adapters, OTel |
| `docs/OPERATORS_COMPATIBILITY.md` / `docs/PORTABILITY_MATRIX.md` | Dialect support tables |
| `docs/RESOURCE_POOLING.md` | RowsAdapter pooling (public feature) |
| `docs/SQL_NULL_TYPES.md` | Null handling in typed scanning |
| `docs/PLUGINS.md` | Custom driver registry |
| `docs/LINTING.md` | Markdown/doc linting tooling |

Update the relevant doc in the same commit as a behavior change — stale
examples in doc comments count as documentation too (`go vet` won't catch
them; reviewers must).
