# Integration Tests for fabric

This directory contains comprehensive integration tests for the fabric library, testing support for SQLite, MySQL, PostgreSQL, and MSSQL databases.

## Overview

The integration tests verify:

- Basic SELECT queries with various databases
- WHERE conditions and filtering
- JOINs (INNER and LEFT)
- Aggregation and GROUP BY
- DISTINCT queries
- LIMIT and OFFSET (pagination)
- Complex queries with multiple conditions
- Transaction handling
- Error handling
- Performance with large result sets

## Test Database Schema

All tests use a consistent schema across all databases:

### Users Table

```
id: INTEGER PRIMARY KEY
name: TEXT NOT NULL
email: TEXT UNIQUE NOT NULL
age: INTEGER
status: TEXT (default: 'active')
```

### Posts Table

```
id: INTEGER PRIMARY KEY
user_id: INTEGER FOREIGN KEY
title: TEXT NOT NULL
content: TEXT
published: BOOLEAN/INTEGER (default: 0/FALSE)
```

### Comments Table

```
id: INTEGER PRIMARY KEY
post_id: INTEGER FOREIGN KEY
user_id: INTEGER FOREIGN KEY
content: TEXT NOT NULL
```

## Running the Tests

### Quick Start with SQLite

SQLite tests can run without any external dependencies:

```bash
# Run all integration tests with SQLite
make integration-test

# Run with verbose output
chmod +x scripts/run-integration-tests.sh
./scripts/run-integration-tests.sh sqlite --verbose

# Run with coverage
./scripts/run-integration-tests.sh sqlite --coverage
```

### Testing with Docker Services

#### MySQL

```bash
# Run MySQL-specific integration tests
make integration-test-mysql

# Or using the script
./scripts/run-integration-tests.sh mysql --verbose
```

#### PostgreSQL

```bash
# Run PostgreSQL-specific integration tests
make integration-test-postgres

# Or using the script
./scripts/run-integration-tests.sh postgres --verbose
```

#### MSSQL

```bash
# Run MSSQL-specific integration tests
make integration-test-mssql

# Or using the script
./scripts/run-integration-tests.sh mssql --verbose
```

### Running All Database Tests

```bash
# Start all database services and run tests
docker-compose -f docker-compose.test.yml up -d

# Run tests against all databases
make integration-test-all

# Or use the script
./scripts/run-integration-tests.sh all

# Cleanup
docker-compose -f docker-compose.test.yml down
```

## Test Script Options

The test helper script supports several options:

```bash
./scripts/run-integration-tests.sh [test-type] [options]

Test Types:
  sqlite    - Run SQLite integration tests
  mysql     - Run MySQL integration tests
  postgres  - Run PostgreSQL integration tests
  mssql     - Run MSSQL integration tests
  all       - Run all database tests

Options:
  --verbose       - Enable verbose test output
  --coverage      - Generate coverage report
  --timeout SEC   - Set test timeout (default: 120s)
```

Examples:

```bash
# Verbose SQLite tests with 60-second timeout
./scripts/run-integration-tests.sh sqlite --verbose --timeout 60

# All tests with coverage reporting
./scripts/run-integration-tests.sh all --coverage

# PostgreSQL tests with verbose output
./scripts/run-integration-tests.sh postgres --verbose
```

## Environment Variables

The tests respect the following environment variables:

```bash
# Set the database type to test
export DB_TYPE=sqlite  # or mysql, postgres, sqlserver

# Database connection strings
export MYSQL_DSN="root:password@tcp(localhost:3306)/test_db?parseTime=true"
export POSTGRES_DSN="host=localhost user=postgres password=password dbname=test_db sslmode=disable"
export MSSQL_DSN="sqlserver://sa:YourPassword123@localhost:1433?database=TestDB"

# Test timeout
export TIMEOUT=300
```

## Docker Compose Configuration

The `docker-compose.test.yml` file provides:

- **MySQL 8.0** - Port 3306
  - Default user: `root`
  - Default password: `password`
  - Default database: `test_db`

- **PostgreSQL 15** - Port 5432
  - Default user: `postgres`
  - Default password: `password`
  - Default database: `test_db`

- **MSSQL 2022** - Port 1433
  - Default user: `sa` (System Administrator)
  - Default password: `YourPassword123`
  - Default database: `TestDB`

Each service includes:

- Automatic initialization with schema files
- Automatic seeding with test data
- Health checks for proper startup detection

## Test Data

All databases are seeded with identical test data:

### Users

- Alice Johnson (28, active)
- Bob Smith (34, active)
- Charlie Davis (45, inactive)
- Diana Wilson (29, active)
- Eve Martinez (31, active)

### Posts

- 5 posts distributed across users
- Mix of published and unpublished content

### Comments

- 7 comments across various posts
- Cross-referenced with users and posts

## CI/CD Integration

### GitHub Actions

The project includes a CI workflow (`.github/workflows/integration-tests.yml`) that:

1. Runs unit tests
2. Runs SQLite integration tests (always)
3. Optionally runs all database tests on demand or scheduled

See the workflow file for configuration details.

## Troubleshooting

### Port Already in Use

```bash
# Find and kill process on port 3306 (MySQL)
lsof -ti:3306 | xargs kill -9

# Or stop conflicting Docker containers
docker ps | grep test-
docker stop <container_id>
```

### Service Failed to Start

```bash
# Check Docker logs
docker logs test-mysql
docker logs test-postgres
docker logs test-mssql

# Verify Docker and docker-compose are running
docker ps
docker-compose version
```

### Test Timeout Issues

```bash
# Increase timeout
./scripts/run-integration-tests.sh postgres --timeout 300

# Or via environment variable
export TIMEOUT=300
go test -timeout 300s -v ./tests -run TestIntegration
```

### Database Connection Issues

Check connection strings match database configuration:

```bash
# Verify MySQL is running
mysql -h localhost -u root -ppassword -e "SELECT VERSION();"

# Verify PostgreSQL is running
psql -h localhost -U postgres -c "SELECT VERSION();"

# Verify MSSQL is running
/opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P YourPassword123 -Q "SELECT @@VERSION"
```

## Performance Considerations

- SQLite tests run entirely in memory (`:memory:`)
- Docker services start in ~10-15 seconds
- Most tests complete within 1-5 seconds per database
- Full test suite (all 4 databases) takes ~60-90 seconds

## Code Coverage

Generate coverage reports:

```bash
# SQLite coverage
./scripts/run-integration-tests.sh sqlite --coverage

# View coverage report
go tool cover -html=coverage.out

# Display terminal report
go tool cover -func=coverage.out
```

## Test Organization

### Integration Tests

- **File**: `tests/integration_test.go`
- **Pattern**: `TestIntegration_*`
- **Run**: `go test -run TestIntegration ./tests`

### Unit Tests

- **Location**: `internal/` and `pkg/` directories
- **Pattern**: `*_test.go`
- **Run**: `go test ./...`

## Adding New Integration Tests

When adding new tests:

1. **Follow the pattern** of existing tests:

   ```go
   func TestIntegration_YourFeature(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping integration tests in short mode")
       }

       db := testDatabases[0]  // Start with SQLite
       runYourFeatureTest(t, db)
   }
   ```

2. **Test with all databases**:

   ```go
   func runYourFeatureTest(t *testing.T, tdb TestDB) {
       t.Run(tdb.name, func(t *testing.T) {
           conn, err := sql.Open(tdb.driver, tdb.connString)
           // Test implementation
       })
   }
   ```

3. **Add seed data if needed** to the `setupSQLiteTestDB` function

4. **Test locally** before committing:
   ```bash
   go test -v ./tests -run TestIntegration_YourFeature
   ```

## Related Documentation

- [db-connector README](../README.md)
- [Builder Documentation](../internal/pkg/builder/)
- [Query Documentation](../pkg/query/)
- [SQL Dialect Documentation](../internal/pkg/sqldialect/)
