# Releases

All notable changes to db-connector are documented in [CHANGELOG.md](./CHANGELOG.md). This page provides release management information, version support, and roadmap.

For detailed technical changes, see the [CHANGELOG](./CHANGELOG.md).

## Latest Release

### v1.0.0 - Production Ready

**Release Date:** March 3, 2026  
**Status:** ✅ Supported

[Download v1.0.0](https://github.com/oratchade/db-connector/releases/tag/v1.0.0) | [View Detailed Changes](./CHANGELOG.md#100---2026-03-03)

First production-ready release with comprehensive documentation, 97+ test cases, and support for MySQL, PostgreSQL, SQLite, and MSSQL.

---

## Version Support Matrix

| Version | Release Date | Go Version | MySQL | PostgreSQL | SQLite | MSSQL | Status         | Support Ends |
| ------- | ------------ | ---------- | ----- | ---------- | ------ | ----- | -------------- | ------------ |
| 1.0.x   | Mar 3, 2026  | 1.26+      | 8.0+  | 15+        | ✅     | 2022+ | ✅ Supported   | Mar 2027     |
| 0.9.x   | Jan 15, 2026 | 1.25+      | 8.0+  | 15+        | ✅     | 2022+ | 🔄 Maintenance | Jun 2026     |

**Status Definitions:**

- ✅ **Supported** - Receives bug fixes and patches
- 🔄 **Maintenance** - Critical bug fixes only
- ⛔ **Retired** - No support, upgrade recommended

---

## Installation

### Latest Stable (v1.0.0)

```bash
go get github.com/oratchade/db-connector
```

### Specific Version

```bash
go get github.com/oratchade/db-connector@v1.0.0
go get github.com/oratchade/db-connector@v0.9.0
```

### Development Version

```bash
go get github.com/oratchade/db-connector@main
```

---

## Upgrade Guides

### v0.9 → v1.0 Migration

**Status:** Breaking changes - migration required

See [CHANGELOG.md#breaking-changes-from-v090](./CHANGELOG.md#breaking-changes-from-v090) for detailed migration instructions.

**Key Changes:**

- API moved from `db` to `v1/db` package
- Error types reorganized in `dberror` package
- `RowsAdapter` replaces generic `Row` interface
- `QueryOptions` structure changed

**Migration Time:** 1-2 hours for typical applications

**Support:** See [../CONTRIBUTING.md](../CONTRIBUTING.md) for help

---

## Roadmap

### v1.1.0 - Q2 2026 (May/June)

**Focus:** Extended type support and batch operations

#### Planned Features

- [ ] Extended type support (time.Time, UUID, custom JSON types)
- [ ] Batch insert/upsert operations
- [ ] Query result caching layer
- [ ] OpenTelemetry integration (optional)
- [ ] Performance benchmarks suite

#### Estimated Effort

- 6-8 weeks development
- Target: June 2026

---

### v1.2.0 - Q3 2026 (Aug/Sept)

**Focus:** Advanced features and optimization

#### Planned Features

- [ ] Connection retry policies
- [ ] Graceful shutdown helpers
- [ ] Query plan analysis helpers
- [ ] Migration integration hints
- [ ] Extended operator support

#### Estimated Effort

- 6-8 weeks development
- Target: September 2026

---

### v2.0.0 - 2027

**Focus:** Major improvements with breaking changes

#### Under Discussion

- Generic query builder (`Query[T]`)
- Context-aware builder chaining
- Advanced query optimization
- Potential breaking changes for major improvements

#### Timeline

- Planning: Q4 2026
- Development: Q1-Q2 2027
- Release: Summer 2027

---

## Release Process

### Schedule

- **Major versions (breaking)** - Approximately annually
- **Minor versions (features)** - Quarterly (every 3 months)
- **Patch versions (bug fixes)** - As needed
- **Pre-releases (alpha/beta)** - Before major releases

### Quality Gates

All releases must pass:

- ✅ 97+ test cases with 100% pass rate
- ✅ Zero linting issues (40+ linters)
- ✅ Code review completion
- ✅ Documentation updates
- ✅ Backward compatibility verification (minor/patch)
- ✅ Database compatibility testing (MySQL, PostgreSQL, SQLite, MSSQL)

### Process

1. Features accumulate on development branch
2. Release branch created with version bump
3. CHANGELOG.md updated
4. Git tag created: `vX.Y.Z`
5. GitHub release published
6. Package available on pkg.go.dev

---

## Security

### Reporting Vulnerabilities

**Do not create public GitHub issues for security vulnerabilities.**

Email security issues to: `security@example.com`

**Include:**

- Vulnerability description
- Affected versions
- Proof of concept (if available)
- Impact assessment
- Suggested fix (optional)

**Response:** Security patches released within 48 hours of verification.

### Security Policy

- Parameterized queries prevent SQL injection
- Error messages sanitized (no credential leakage)
- Context timeout propagation enforced
- No external dependencies for core library

---

## Download

### Official Releases

All releases available at [GitHub Releases](https://github.com/oratchade/db-connector/releases)

### Checksums

Verify downloads:

```bash
sha256sum -c db-connector-v1.0.0.sha256
```

---

## Deprecation Timeline

### v0.9.0 Features (Removed in v1.0.0)

- ❌ `Row` interface → Use `RowsAdapter`
- ❌ `OldQueryOptions` → Use `options.QueryOptions`
- ❌ Legacy error types → Use `dberror` sentinels

See [CHANGELOG.md](./CHANGELOG.md) for complete deprecation list and migration paths.

---

## Getting Help

- 📖 [README.md](../README.md) - Quick start and examples
- 🐛 [Issue Tracker](https://github.com/oratchade/db-connector/issues) - Bug reports
- 💬 [Discussions](https://github.com/oratchade/db-connector/discussions) - Questions
- 📝 [CHANGELOG.md](./CHANGELOG.md) - Detailed changes
- 🤝 [CONTRIBUTING.md](../CONTRIBUTING.md) - Developer guide

---

## Staying Updated

### Watch for Releases

[Watch the repository](https://github.com/oratchade/db-connector/subscription) on GitHub for notifications

### Subscribe to Releases

Enable "Releases only" notifications in GitHub

### Check Periodically

```bash
go list -u -m github.com/oratchade/db-connector
```

---

## License

db-connector is licensed under the MIT License. See [../LICENSE.md](../LICENSE.md) for details.

Changes are tracked in [CHANGELOG.md](./CHANGELOG.md).

---

**Last Updated:** March 3, 2026  
**Maintained by:** [@oratchade](https://github.com/oratchade)

For detailed technical changes across all versions, see [CHANGELOG.md](./CHANGELOG.md).
