# Releases

This file describes how Vessel releases are packaged and published. For release
contents, see [CHANGELOG.md](CHANGELOG.md).

---

## Release Contents

### [0.1.1](CHANGELOG.md#011---2026-05-27)

**Release Date**: May 27, 2026

**Scope**:

- Complete project rename to Vessel.
- Public module path updated to `tounilab.com/vessel`.
- Documentation, examples, and release metadata refreshed for the Vessel name.
- Integration strict-mode flag renamed to `VESSEL_INTEGRATION_STRICT`.

### [0.1.0](CHANGELOG.md#010---2026-05-23)

**Release Date**: May 23, 2026

**Scope**:

- Core `db/v1.DB` and `db/v1.Tx` interfaces
- Fluent builders for select, insert, update, delete, and upsert
- MySQL, PostgreSQL, SQLite, and MSSQL support for core flows
- Dialect-specific SQL generation with explicit unsupported-feature errors
- OpenTelemetry tracing, retry helpers, transactions, pool stats, and manager
  routing
- Unit, race, and Docker-backed integration test coverage

## Dialect-Aware Database Support

- MySQL, PostgreSQL, SQLite, and MSSQL support for core CRUD/query flows
- Per-dialect SQL rendering and identifier quoting
- Clear errors when a dialect cannot support a requested feature

## Fluent Query Builder

- SQL generation with parameterized values
- SELECT, INSERT, UPDATE, DELETE, UPSERT, and query preview helpers
- JOINs, aliases, aggregates, GROUP BY, safe HAVING, and raw escape hatches
- `UpdateAll()` and `DeleteAll()` for intentional unfiltered mutations
- PostgreSQL `RETURNING` and MSSQL `OUTPUT` query preview support

## Pluggable Loggers

- **slog** (Go stdlib)
- **logrus** (sirupsen/logrus)
- **zap** (Uber go/zap)
- **apex** (github.com/apex/log)
- Context chaining via `With()` method

## Observability

- OpenTelemetry tracing integration
- Distributed tracing support
- Transaction context tracking

## Operational Features

- Transaction support with atomicity
- Health check API
- Plugin system for custom drivers
- Manager API for multi-database routing
- Configuration from YAML/TOML/JSON

## Test Coverage

- Unit tests for builders, dialects, conditions, transactions, scanning, retry,
  plugins, and manager behavior
- Integration tests across SQLite and Docker-backed MySQL/PostgreSQL/MSSQL
- Race targets for manager, DB, query, internal builder/dialect, and retry
  packages
- Resource pooling support for high-throughput row adapter paths

---

## Installation

```bash
# Get the latest version
go get tounilab.com/vessel

# Or pin to specific version
go get tounilab.com/vessel@v0.1.1
```

## Release Automation

Vessel releases are tag-driven. A release tag starts the `Release` workflow,
which validates the library, creates a source archive with a checksum, and
publishes the GitHub Release from the matching `CHANGELOG.md` section.

### Release Requirements

- The tag must use semantic version format, for example `v0.1.1` or
  `v0.2.0-rc.1`.
- `CHANGELOG.md` must contain a matching section, for example
  `## [0.1.1] - 2026-05-27`.
- Formatting, linting, module tidiness, unit coverage, and SQLite integration
  tests must pass.

### Release Command

```bash
git tag -a v0.1.1 -m "Release v0.1.1"
git push origin v0.1.1
```

The workflow can also be re-run manually from GitHub Actions with an existing
tag. Manual runs are useful for recreating a draft release or retrying release
publication after an external service issue.

### Published Artifacts

- GitHub Release named `Vessel vX.Y.Z`.
- Release notes generated from `CHANGELOG.md`.
- Source archive: `vessel-vX.Y.Z.tar.gz`.
- SHA-256 checksum: `vessel-vX.Y.Z.tar.gz.sha256`.

For Go consumers, the release artifact is the module tag:

```bash
go get tounilab.com/vessel@vX.Y.Z
```

---

## Quick Start

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "tounilab.com/vessel/db/v1"
    cdt "tounilab.com/vessel/pkg/query/condition"
)

func main() {
    // Create database config
    cfg := v1.PostgresConfig{
        Host:     "localhost",
        Port:     5432,
        User:     "postgres",
        Password: os.Getenv("DB_PASSWORD"),
        DBName:   "myapp",
    }

    // Create logger adapter
    logger := v1.NewSlogAdapter(slog.Default())

    // Connect to database
    db, err := v1.NewDB(cfg, logger)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Execute a query
    ctx := context.Background()
    rows, err := v1.NewFluentDB(db, ctx).
        Select("users", "id", "name", "email").
        Where(
            cdt.NewExpr().Column("age").Op(">").Value(18),
        ).
        OrderBy("name ASC").
        Limit(10).
        Execute()

    if err != nil {
        panic(err)
    }

    // Process results
    for _, row := range rows {
        println(row["name"].(string))
    }
}
```

## Support

### Documentation

- 📖 [README.md](README.md) - Feature overview and installation
- 📘 [CHANGELOG.md](CHANGELOG.md) - Detailed change history
- 📚 [docs/](docs/) - Comprehensive guides
- 💡 [examples/](examples/) - Code examples
  - Basic database operations (`basic/`)
  - Priority-based routing (`priority_selection/`)
  - Error handling patterns (`error_handling/`)
  - **Retry integration examples** (`retry/`) - 4 patterns with
    exponential, linear, and fixed backoff

### Getting Help

- 📝 Open an issue on GitHub
- 📧 Contact maintainers via GitHub discussions
- 💬 Check [CONTRIBUTING.md](CONTRIBUTING.md) for setup

---

## License

Vessel is licensed under the [MIT License](LICENSE.md).
