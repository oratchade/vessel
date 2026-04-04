# Test Environment Variables

This guide explains how to configure test credentials for the fabric
SQL abstraction library without exposing sensitive information in
source code.

## Overview

All test database credentials are configurable via environment variables
with sensible fallbacks for local development. This approach:

- ✅ Prevents accidental credential exposure in version control
- ✅ Supports both local and CI/CD environments
- ✅ Works seamlessly with Docker containers
- ✅ Enables different credentials for different environments

## Quick Start

### 1. Copy the Example Environment File

```bash
cd fabric
cp .env.example .env
```

### 2. (Optional) Customize Credentials

Edit `.env` if you have different local database configurations:

```bash
# Only edit if you've changed Docker container passwords
DB_MYSQL_PASSWORD=your_mysql_password
DB_POSTGRES_PASSWORD=your_postgres_password
DB_MSSQL_PASSWORD=your_mssql_password
```

### 3. Load Environment Variables (Optional)

For automatic loading (local development):

```bash
# Using direnv (if installed)
cp .env .envrc

# Or manually
export $(cat .env | xargs)

# Or for single test run
DB_MYSQL_PASSWORD=custom make test
```

### 4. Run Tests

```bash
# Unit tests (no credentials needed)
make test

# SQLite integration tests (no DB containers needed)
make integration-test-sqlite

# All integration tests (requires Docker)
docker-compose -f docker-compose.test.yml up -d
make integration-test-all
docker-compose -f docker-compose.test.yml down
```

---

## Environment Variables Reference

### MySQL Configuration

| Variable            | Purpose  | Default         | Example       |
| ------------------- | -------- | --------------- | ------------- |
| `DB_MYSQL_USER`     | User     | `root`          | `root`        |
| `DB_MYSQL_PASSWORD` | Password | `root_password` | (not exposed) |
| `DB_MYSQL_HOST`     | Host     | `localhost`     | `localhost`   |
| `DB_MYSQL_PORT`     | Port     | `3306`          | `3306`        |
| `DB_MYSQL_DATABASE` | Database | `test_db`       | `test_db`     |
| `DB_MYSQL_CHARSET`  | Charset  | `utf8mb4`       | `utf8mb4`     |

**Example `.env` entry:**

```bash
DB_MYSQL_USER=root
DB_MYSQL_PASSWORD=root_password
DB_MYSQL_HOST=localhost
DB_MYSQL_PORT=3306
DB_MYSQL_DATABASE=test_db
DB_MYSQL_CHARSET=utf8mb4
```

### PostgreSQL Configuration

| Variable               | Purpose  | Default         | Example       |
| ---------------------- | -------- | --------------- | ------------- |
| `DB_POSTGRES_USER`     | User     | `test_user`     | `postgres`    |
| `DB_POSTGRES_PASSWORD` | Password | `test_password` | (not exposed) |
| `DB_POSTGRES_HOST`     | Host     | `localhost`     | `localhost`   |
| `DB_POSTGRES_PORT`     | Port     | `5432`          | `5432`        |
| `DB_POSTGRES_DATABASE` | Database | `test_db`       | `test_db`     |
| `DB_POSTGRES_SSLMODE`  | SSL mode | `disable`       | `require`     |

**Example `.env` entry:**

```bash
DB_POSTGRES_USER=test_user
DB_POSTGRES_PASSWORD=test_password
DB_POSTGRES_HOST=localhost
DB_POSTGRES_PORT=5432
DB_POSTGRES_DATABASE=test_db
DB_POSTGRES_SSLMODE=disable
```

### Microsoft SQL Server Configuration

| Variable                     | Purpose    | Default            | Example     |
| ---------------------------- | ---------- | ------------------ | ----------- |
| `DB_MSSQL_USER`              | SA user    | `sa`               | `sa`        |
| `DB_MSSQL_PASSWORD`          | Password   | `TestPassword123!` | (secure)    |
| `DB_MSSQL_HOST`              | Host       | `localhost`        | `localhost` |
| `DB_MSSQL_PORT`              | Port       | `1433`             | `1433`      |
| `DB_MSSQL_DATABASE`          | Database   | `test_db`          | `test_db`   |
| `DB_MSSQL_ENCRYPT`           | Encryption | `disable`          | `true`      |
| `DB_MSSQL_TRUST_SERVER_CERT` | Trust cert | `true`             | `false`     |

**Example `.env` entry:**

```bash
DB_MSSQL_USER=sa
DB_MSSQL_PASSWORD=TestPassword123!
DB_MSSQL_HOST=localhost
DB_MSSQL_PORT=1433
DB_MSSQL_DATABASE=test_db
DB_MSSQL_ENCRYPT=disable
DB_MSSQL_TRUST_SERVER_CERT=true
```

### SQLite Configuration

| Variable         | Purpose            | Default    | Example   |
| ---------------- | ------------------ | ---------- | --------- |
| `DB_SQLITE_PATH` | Database file path | `:memory:` | `/tmp/db` |

**Note**: SQLite requires no authentication. In-memory mode (`:memory:`) is
recommended for test isolation.

---

## CI/CD Environment Setup

### GitHub Actions

```yaml
- name: Run Fabric Tests
  env:
    DB_MYSQL_PASSWORD: ${{ secrets.DB_MYSQL_PASSWORD }}
    DB_POSTGRES_PASSWORD: ${{ secrets.DB_POSTGRES_PASSWORD }}
    DB_MSSQL_PASSWORD: ${{ secrets.DB_MSSQL_PASSWORD }}
  run: make test
```

### GitLab CI

```yaml
test:integration:
  variables:
    DB_MYSQL_PASSWORD: $DB_MYSQL_PASSWORD
    DB_POSTGRES_PASSWORD: $DB_POSTGRES_PASSWORD
    DB_MSSQL_PASSWORD: $DB_MSSQL_PASSWORD
  script:
    - make integration-test-all
```

### Docker Compose

The `docker-compose.test.yml` automatically exports environment variables:

```yaml
services:
  mysql:
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_MYSQL_PASSWORD:-root_password}
  postgres:
    environment:
      POSTGRES_PASSWORD: ${DB_POSTGRES_PASSWORD:-test_password}
  mssql:
    environment:
      SA_PASSWORD: ${DB_MSSQL_PASSWORD:-TestPassword123!}
```

Set credentials before `docker-compose up`:

```bash
export DB_MYSQL_PASSWORD=custom_password
export DB_POSTGRES_PASSWORD=custom_password
export DB_MSSQL_PASSWORD=CustomPassword123!
docker-compose -f docker-compose.test.yml up -d
make integration-test-all
```

---

## Best Practices

### ✅ DO

- ✅ Use `.env` for **local development** only
- ✅ Add `.env` to `.gitignore` (already done)
- ✅ Use distinct, strong passwords for **production**
- ✅ Load credentials from **secrets management** in CI/CD
- ✅ Rotate credentials regularly
- ✅ Use environment-specific values (different per local/staging/production)

### ❌ DON'T

- ❌ Commit `.env` file to version control
- ❌ Use default credentials in production
- ❌ Store real production passwords in `.env.example`
- ❌ Log or print credential values
- ❌ Use the same credentials across environments

---

## Troubleshooting

### "Connection refused" Error

**Symptom**: Tests fail with "connection refused" or "authentication failed"

**Solution**:

1. Verify containers are running: `docker-compose -f docker-compose.test.yml ps`
2. Check `.env` credentials match Docker Compose setup
3. Ensure ports are not blocked: `nc -zv localhost 3306`

### "FOREIGN KEY constraint failed"

**Symptom**: SQLite tests fail with foreign key errors

**Solution**: Ensure `ForeignKeys: true` is set in SQLite config (already configured)

### "SSL Error" on PostgreSQL

**Symptom**: PostgreSQL tests fail with SSL errors

**Solution**:

- For local testing, set `DB_POSTGRES_SSLMODE=disable`
- For production, use `DB_POSTGRES_SSLMODE=require`

---

## See Also

- [.env.example](../.env.example) — Complete environment variable template
- [Makefile](../Makefile) — Test execution commands
- [docker-compose.test.yml](../docker-compose.test.yml) — Test container setup
