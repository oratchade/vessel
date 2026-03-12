# Contributing to fabric

Thank you for your interest in contributing to fabric! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Code Style](#code-style)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Reporting Issues](#reporting-issues)
- [License](#license)

## Code of Conduct

This project adheres to the Contributor Covenant Code of Conduct. By participating, you are expected to uphold this code.

**Be respectful.** Be kind and constructive in all interactions with other contributors.

## Getting Started

### Prerequisites

- Go 1.26.0 or later
- Git
- Docker (for running database tests)
- Make
- One or more SQL databases for testing (MySQL, PostgreSQL, SQLite, MSSQL)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

   ```bash
   git clone https://github.com/YOUR_USERNAME/fabric.git
   cd fabric
   ```

3. Add upstream remote:

   ```bash
   git remote add upstream https://github.com/oratchade/fabric.git
   ```

## Development Setup

### Install Dependencies

```bash
# Download Go dependencies
go mod download

# Install development tools
make install-tools
```

### Verify Your Setup

```bash
# Run tests
make test

# Run linters
make lint

# Build the project
make build
```

If all commands succeed, your environment is ready!

## Making Changes

### Create a Feature Branch

```bash
# Update main branch
git fetch upstream
git checkout main
git reset --hard upstream/main

# Create feature branch
git checkout -b feature/your-feature-name
```

### Branch Naming Conventions

- `feature/` - New features (`feature/query-builder-improvements`)
- `fix/` - Bug fixes (`fix/connection-pool-leak`)
- `docs/` - Documentation updates (`docs/error-handling-guide`)
- `refactor/` - Code refactoring (`refactor/reduce-allocations`)
- `test/` - Adding/improving tests (`test/integration-tests`)
- `perf/` - Performance improvements (`perf/zero-copy-scanning`)

### Guidelines for Changes

1. **Single Responsibility** - Each PR should address one feature or fix
2. **Backward Compatibility** - Maintain API compatibility unless it's a major version
3. **Documentation** - Update README, docs, and code comments for your changes
4. **Tests** - Add tests for new features and bug fixes
5. **Comments** - Follow Go comment standards (`// Package...`, exported items documented)

## Testing

### Running Tests

```bash
# Run all unit tests
make test

# Run integration tests (requires Docker with databases)
go test -v -tags=integration -run TestIntegration ./tests

# Run unit tests for specific package
go test -v ./db/v1/...

# Run with coverage
make coverage
make cover-html  # Opens coverage report in browser
```

### Writing Tests

1. **Test Files** - Place tests in `*_test.go` files in the same package
2. **Test Functions** - Use `TestXxx(t *testing.T)` naming
3. **Mocks** - Use `go:generate` for mock generation:

   ```bash
   go generate ./...
   ```

4. **Table-Driven Tests** - Use table-driven approach for multiple scenarios:

   ```go
   tests := []struct {
       name    string
       input   string
       want    string
       wantErr bool
   }{
       {"case1", "input1", "output1", false},
       {"case2", "input2", "output2", true},
   }

   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           // test implementation
       })
   }
   ```

### Testing Against All Databases

For features that interact with databases:

```bash
# Create test databases
docker-compose up -d

# Run integration tests
make integration-test

# Cleanup
docker-compose down
```

Supported database versions in tests:

- MySQL 8.0+
- PostgreSQL 15+
- SQLite (latest)
- MSSQL 2022

## Code Style

### Go Standards

This project follows [Effective Go](https://golang.org/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Formatting

```bash
# Format all files
make fmt

# Check formatting without changes
make fmt-check

# Run gofumpt for stricter formatting
make fmt-strict
```

### Linting

```bash
# Run all linters (40+ enabled)
make lint

# Fix common issues automatically
make lint-fix

# Check specific linter
golangci-lint run ./... --enable=errname
```

### Comment Standards

1. **Package Comments** - Every package must have a comment:

   ```go
   // Package sqldialect provides SQL dialect abstractions.
   package sqldialect
   ```

2. **Exported Items** - All exported types, functions, and methods must have comments:

   ```go
   // User represents a user in the database.
   type User struct {
       ID   int    // User's unique identifier
       Name string // User's full name
   }

   // NewUser creates a new User instance.
   func NewUser(name string) *User {
       return &User{Name: name}
   }
   ```

3. **Comment Format** - Start with the name being described and use complete sentences

### Naming Conventions

- Use `camelCase` for variable/function names
- Use `PascalCase` for exported types
- Use `SCREAMING_SNAKE_CASE` for constants
- Avoid abbreviations unless widely understood (e.g., `ID`, `SQL`, `HTTP`)

### Error Handling

1. Use sentinel errors for specific conditions:

   ```go
   var ErrDuplicateKey = errors.New("duplicate key constraint violation")
   ```

2. Wrap errors with context:

   ```go
   return fmt.Errorf("failed to insert user: %w", err)
   ```

3. Check errors explicitly (don't ignore them)

## Commit Messages

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Examples

**Simple fix:**

```
fix(mysql): connection pool leak in Close()

Properly release all idle connections on database close.
```

**New feature:**

```
feat(builder): add HAVING clause support

Allow filtering aggregated results with HAVING conditions.
Closes #123
```

**Documentation:**

```
docs: add error handling guide

Comprehensive guide covering sentinel errors, dialect-specific
mapping, and recovery strategies.
```

### Guidelines

- **Type** - `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `perf`, `chore`
- **Scope** - Component affected: `mysql`, `postgres`, `builder`, `condition`, etc.
- **Subject** - Imperative mood, no period, 50 chars or less
- **Body** - Explain what and why, not how (72 chars per line)
- **Footer** - Reference issues: `Closes #123`, `Fixes #456`

## Pull Requests

### Before Submitting

1. **Update main branch:**

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests locally:**

   ```bash
   make test
   make lint
   ```

3. **Update documentation:**
   - Update README if behavior changed
   - Update relevant docs/ files
   - Add code comments per standards

4. **Commit with good messages:**

   ```bash
   git commit -m "feat(builder): add JOIN support"
   ```

### PR Description Template

When you open a pull request, GitHub will automatically display a template to help you fill in all required information. The template includes:

```markdown
## Description

Brief description of the changes and their purpose.

## Type of Change

- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Related Issues

Closes #(issue number)

## Testing

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Tested against all 4 databases (MySQL, PostgreSQL, SQLite, MSSQL)

## Checklist

- [ ] Code follows style guidelines
- [ ] Comments follow Go standards
- [ ] No new warnings generated
- [ ] Tests pass locally
- [ ] Documentation updated
```

**The full template is located at [`.github/pull_request_template.md`](./.github/pull_request_template.md) and will be automatically populated when you create a PR.**

Key points:

- ✅ Always fill in a meaningful **Description** - explain what changed and why
- ✅ Select appropriate **Type of Change** - helps reviewers categorize the PR
- ✅ Reference related **Issues** - use `Closes #123` to auto-close issues
- ✅ Document **Testing** - specify which databases and test types
- ✅ Complete the **Checklist** - ensures standards compliance

### Review Process

1. **Automated checks** - CI/CD pipeline runs tests and linters
2. **Code review** - Maintainers review your changes
3. **Feedback** - Address requested changes
4. **Approval** - Once approved, code is merged

### Tips for Faster Review

- Keep PRs focused and reasonably sized (<400 lines)
- Write clear commit messages
- Add comments for non-obvious logic
- Include test cases
- Reference related issues
- Be responsive to feedback

## Reporting Issues

### Bug Reports

Include:

- Go version (`go version`)
- Database system and version
- Minimal reproduction code
- Expected vs. actual behavior
- Error message/logs
- Steps to reproduce

**Example:**

```markdown
## Bug Report

**Go Version:** go 1.26.0
**Database:** PostgreSQL 15.2
**fabric Version:** v1.0.0

### Description

Inserting NULL values in query conditions causes panic.

### Steps to Reproduce

1. Create user with NULL email
2. Query with condition "email = NULL"
3. Panic occurs

### Expected Behavior

Query should return no results or proper error

### Error Message
```

panic: invalid argument to IsNil
goroutine 1 [running]: ...

````

### Minimal Code
```go
db.Get(ctx, "users", cols, "", "email IS NULL", nil)
````

````

### Feature Requests

Include:
- Clear description of the feature
- Motivation and use case
- Example usage
- Alternatives considered
- Implementation notes (optional)

**Example:**
```markdown
## Feature Request: Batch Insert

### Description
Support batch/bulk insert operations for better performance with large datasets.

### Use Case
Inserting 10,000+ records is slow with individual insert calls.

### Proposed API
```go
results, err := db.BatchInsert(ctx, "users", []map[string]any{
    {"name": "Alice"},
    {"name": "Bob"},
})
````

### Alternatives

- Use raw SQL for bulk insert
- Use transactions with multiple inserts

```

## Architecture Guidelines

### Package Structure

```

fabric/
├── db/v1/ # Public API (stable, versioned)
├── internal/pkg/ # Internal packages (no stability guarantee)
├── pkg/query/ # Query building DSL
└── tests/ # Integration tests

````

### Interfaces Over Implementations

- Define interfaces for abstraction
- Keep implementations internal when possible
- Use dependency injection for testability

### Error Handling Design

- Use sentinel errors for specific conditions
- Map dialect-specific errors to common sentinel errors
- Always include operation context in wrapped errors

### Testing Strategy

1. **Unit Tests** - Test individual functions and methods
2. **Mock Tests** - Use generated mocks for isolation
3. **Integration Tests** - Test against real databases
4. **Compatibility Tests** - Verify behavior across dialects

## Documentation

### Code Documentation

All public packages, types, and functions must have comments:

```bash
# Check documentation coverage
go doc ./db/v1
go doc github.com/oratchade/fabric/db/v1
````

### User Documentation

- **README.md** - Feature overview, quick start, examples
- **ERROR_HANDLING.md** - Error patterns and recovery strategies
- **OPERATORS_COMPATIBILITY.md** - Dialect-specific operator support
- **SQL_NULL_TYPES.md** - NULL type handling guide ([docs/SQL_NULL_TYPES.md](./docs/SQL_NULL_TYPES.md))
- **CODE_REVIEW.md** - Code quality assessment and metrics

## Performance Considerations

When contributing performance-critical code:

1. **Benchmark** - Add benchmarks for performance-sensitive operations:

   ```go
   func BenchmarkScanRowsTo(b *testing.B) {
       // benchmark setup
       for i := 0; i < b.N; i++ {
           // operation to benchmark
       }
   }
   ```

2. **Profile** - Use pprof to identify bottlenecks:

   ```bash
   go test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=. ./...
   go tool pprof cpu.prof
   ```

3. **Allocations** - Minimize allocations in hot paths:

   ```bash
   go test -benchmem ./...
   ```

4. **Document** - Explain performance implications in comments

## Release Process

Maintainers follow semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR** - Breaking API changes
- **MINOR** - New features, backward compatible
- **PATCH** - Bug fixes

Contributors: don't worry about versioning, maintainers handle releases.

## Resources

- [Go Effective Comments](https://go.dev/blog/peg)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)

## Getting Help

- **Questions** - Open a discussion or issue
- **Documentation** - Check README, ERROR_HANDLING.md, and code comments
- **Examples** - See examples/ directory for usage patterns
- **Issues** - Search existing issues before creating new ones

## Recognition

Contributors are recognized in:

- Release notes
- CHANGELOG
- GitHub contributors page

---

Thank you for contributing to fabric! Your efforts help make this library better for everyone.

**Happy contributing! 🎉**
