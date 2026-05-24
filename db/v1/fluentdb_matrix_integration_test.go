//go:build integration

package v1_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	v1 "tounilab.com/fabric/db/v1"
	cdt "tounilab.com/fabric/pkg/query/condition"
)

type fluentMatrixDB struct {
	name   string
	driver string
	cfg    v1.DBConfig
}

const fluentMatrixUsersTable = "fluent_matrix_users"

func TestFluentDBDialectMatrix(t *testing.T) {
	for _, testDB := range fluentMatrixDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			db := connectFluentMatrixDB(t, testDB)
			defer func() { _ = db.Close() }()
			setupFluentMatrixSchema(t, db, testDB.driver)

			ctx := context.Background()
			fluent := v1.NewFluentDB(db)

			_, err := fluent.Insert().
				Into(fluentMatrixUsersTable).
				Set("name", "Alice").
				Set("email", "alice.matrix@example.com").
				Set("age", 31).
				Set("status", "active").
				Exec(ctx)
			require.NoError(t, err)
			_, err = fluent.Insert().
				Into(fluentMatrixUsersTable).
				Set("name", "Bob").
				Set("email", "bob.matrix@example.com").
				Set("age", 42).
				Set("status", "inactive").
				Exec(ctx)
			require.NoError(t, err)

			countRows, err := fluent.Select(fluentMatrixUsersTable).
				Where(cdt.In("status", "active", "inactive")).
				CountRaw(ctx)
			require.NoError(t, err)
			require.True(t, countRows.Next())
			var count int64
			require.NoError(t, countRows.Scan(&count))
			require.NoError(t, countRows.Close())
			assert.Equal(t, int64(2), count)

			fetched, err := fluent.Insert().
				Into(fluentMatrixUsersTable).
				Set("name", "Carol").
				Set("email", "carol.matrix@example.com").
				Set("age", 28).
				Set("status", "active").
				InsertAndFetch(ctx, "email", "email", "status")
			require.NoError(t, err)
			assert.Equal(t, "active", fetched["status"])

			joinRows, err := fluent.Select(fluentMatrixUsersTable).
				ColumnAs(fluentMatrixUsersTable+".name", "user_name").
				ColumnRawAs("LOWER(fm2.email)", "joined_email").
				Join(cdt.Join{
					Type:       "LEFT",
					Table:      fluentMatrixUsersTable,
					Alias:      "fm2",
					Conditions: cdt.JoinCdts{{Left: "status", Right: "status"}},
					On:         cdt.IsNull("fm2.deleted_at"),
				}).
				Where(cdt.ILike(fluentMatrixUsersTable+".email", "%MATRIX@EXAMPLE.COM")).
				OrderByAsc(fluentMatrixUsersTable + ".id").
				Limit(1).
				Get(ctx)
			require.NoError(t, err)
			assert.Len(t, joinRows, 1)

			query, _, err := fluent.Select(fluentMatrixUsersTable, "status", "COUNT(*) AS total").
				Where(cdt.NewExpr().Column("age").Op(">").Value(20)).
				GroupBy("status").
				Having(cdt.NewExpr().Column("COUNT(*)").Op(">").Value(0)).
				OrderByAsc("status").
				Query()
			require.NoError(t, err)
			assert.NotEmpty(t, query)

			grouped, err := fluent.Select(fluentMatrixUsersTable, "status", "COUNT(*) AS total").
				Where(cdt.NewExpr().Column("age").Op(">").Value(20)).
				GroupBy("status").
				HavingRaw("COUNT(*) > 0").
				OrderByDesc("status").
				Get(ctx)
			require.NoError(t, err)
			assert.Len(t, grouped, 2)

			insertPreview, _, err := fluent.Insert().
				Into(fluentMatrixUsersTable).
				Set("name", "Preview").
				Set("email", "preview.matrix@example.com").
				Returning("id").
				Query()
			require.NoError(t, err)
			assertMutationPreview(t, testDB.driver, insertPreview)

			updateBuilder := fluent.Update(fluentMatrixUsersTable).
				Set("status", "active").
				Where(cdt.NewExpr().Column("status").Op("=").Value("inactive")).
				OrderByAsc("id").
				Limit(1)
			_, _, err = updateBuilder.Query()
			if testDB.driver == "mysql" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func fluentMatrixDatabases() []fluentMatrixDB {
	dbs := []fluentMatrixDB{
		{
			name:   "SQLite",
			driver: "sqlite",
			cfg: v1.SQLiteConfig{
				FilePath:     ":memory:",
				CacheMode:    "shared",
				Mode:         "memory",
				ForeignKeys:  true,
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
		{
			name:   "MySQL",
			driver: "mysql",
			cfg: v1.MysqlConfig{
				User:         fluentEnv("DB_MYSQL_USER", "root"),
				Password:     fluentEnv("DB_MYSQL_PASSWORD", "root_password"),
				Host:         fluentEnv("DB_MYSQL_HOST", "localhost"),
				Port:         fluentEnvUint16("DB_MYSQL_PORT", 3306),
				Database:     fluentEnv("DB_MYSQL_DATABASE", "test_db"),
				Charset:      "utf8mb4",
				ParseTime:    true,
				Loc:          "Local",
				Timeout:      10 * time.Second,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				MaxOpenConns: 10,
				MaxIdleConns: 2,
			},
		},
		{
			name:   "PostgreSQL",
			driver: "postgres",
			cfg: v1.PostgresConfig{
				User:           fluentEnv("DB_POSTGRES_USER", "test_user"),
				Password:       fluentEnv("DB_POSTGRES_PASSWORD", "test_password"),
				Host:           fluentEnv("DB_POSTGRES_HOST", "localhost"),
				Port:           fluentEnvUint16("DB_POSTGRES_PORT", 5432),
				Database:       fluentEnv("DB_POSTGRES_DATABASE", "test_db"),
				SSLMode:        "disable",
				ConnectTimeout: 10 * time.Second,
				PoolMaxConns:   10,
				PoolMinConns:   2,
			},
		},
		{
			name:   "MSSQL",
			driver: "sqlserver",
			cfg: v1.MSSQLConfig{
				User:            fluentEnv("DB_MSSQL_USER", "sa"),
				Password:        fluentEnv("DB_MSSQL_PASSWORD", "TestPassword123!"),
				Host:            fluentEnv("DB_MSSQL_HOST", "127.0.0.1"),
				Port:            fluentEnvUint16("DB_MSSQL_PORT", 1433),
				Database:        fluentEnv("DB_MSSQL_DATABASE", "test_db"),
				Encrypt:         "disable",
				TrustServerCert: true,
				MaxOpenConns:    10,
				MaxIdleConns:    2,
			},
		},
	}

	filter := strings.ToLower(os.Getenv("DB_TYPE"))
	if filter == "" {
		return dbs
	}
	filtered := make([]fluentMatrixDB, 0, 1)
	for _, db := range dbs {
		if filter == db.driver ||
			(filter == "postgresql" && db.driver == "postgres") ||
			(filter == "mssql" && db.driver == "sqlserver") {
			filtered = append(filtered, db)
		}
	}
	return filtered
}

func connectFluentMatrixDB(t *testing.T, testDB fluentMatrixDB) v1.DB {
	t.Helper()

	if testDB.driver == "sqlserver" {
		requireMSSQLDatabase(t, testDB.cfg.(v1.MSSQLConfig))
	}

	var database v1.DB
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		database, err = v1.NewDB(testDB.cfg, nil)
		if err == nil {
			if pingErr := database.Ping(context.Background()); pingErr == nil {
				return database
			} else {
				err = pingErr
			}
			_ = database.Close()
		}
		time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
	}

	msg := fmt.Sprintf("%s integration database unavailable: %v", testDB.name, err)
	if fluentStrict() || os.Getenv("DB_TYPE") != "" {
		t.Fatal(msg)
	}
	t.Skip(msg)
	return nil
}

func setupFluentMatrixSchema(t *testing.T, db v1.DB, driver string) {
	t.Helper()

	schema := map[string][]string{
		"sqlite": {
			"DROP TABLE IF EXISTS " + fluentMatrixUsersTable,
			`CREATE TABLE ` + fluentMatrixUsersTable + ` (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL,
				age INTEGER,
				status TEXT DEFAULT 'active',
				deleted_at DATETIME NULL
			)`,
		},
		"mysql": {
			"DROP TABLE IF EXISTS " + fluentMatrixUsersTable,
			`CREATE TABLE ` + fluentMatrixUsersTable + ` (
				id INT PRIMARY KEY AUTO_INCREMENT,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				age INT,
				status VARCHAR(50) DEFAULT 'active',
				deleted_at DATETIME NULL
			)`,
		},
		"postgres": {
			"DROP TABLE IF EXISTS " + fluentMatrixUsersTable + " CASCADE",
			`CREATE TABLE ` + fluentMatrixUsersTable + ` (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				age INT,
				status VARCHAR(50) DEFAULT 'active',
				deleted_at TIMESTAMP NULL
			)`,
		},
		"sqlserver": {
			"DROP TABLE IF EXISTS " + fluentMatrixUsersTable,
			`CREATE TABLE ` + fluentMatrixUsersTable + ` (
				id INT PRIMARY KEY IDENTITY(1,1),
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				age INT,
				status VARCHAR(50) DEFAULT 'active',
				deleted_at DATETIME2 NULL
			)`,
		},
	}

	for _, stmt := range schema[driver] {
		_, err := db.Exec(context.Background(), stmt)
		require.NoError(t, err, "schema statement failed: %s", stmt)
	}
}

func assertMutationPreview(t *testing.T, driver string, query string) {
	t.Helper()

	switch driver {
	case "postgres":
		assert.Contains(t, query, "RETURNING")
	case "sqlserver":
		assert.Contains(t, query, "OUTPUT")
	default:
		assert.NotContains(t, query, "RETURNING")
		assert.NotContains(t, query, "OUTPUT")
	}
}

func requireMSSQLDatabase(t *testing.T, cfg v1.MSSQLConfig) {
	t.Helper()

	masterCfg := cfg
	masterCfg.Database = "master"
	masterDB, err := openReadyFluentMSSQLMaster(masterCfg)
	if err != nil {
		if fluentStrict() || os.Getenv("DB_TYPE") != "" {
			require.NoError(t, err)
		}
		t.Skipf("MSSQL integration database unavailable: %v", err)
	}
	defer func() { _ = masterDB.Close() }()

	dbName := strings.ReplaceAll(cfg.Database, "'", "''")
	identifier := strings.ReplaceAll(cfg.Database, "]", "]]")
	_, err = masterDB.Exec(fmt.Sprintf("IF DB_ID(N'%s') IS NULL CREATE DATABASE [%s]", dbName, identifier))
	require.NoError(t, err)
}

func openReadyFluentMSSQLMaster(cfg v1.MSSQLConfig) (*sql.DB, error) {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error

	for attempt := 1; time.Now().Before(deadline); attempt++ {
		masterDB, err := sql.Open(cfg.Driver(), cfg.DSN())
		if err != nil {
			lastErr = err
		} else {
			pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = masterDB.PingContext(pingCtx)
			cancel()
			if err == nil {
				return masterDB, nil
			}
			lastErr = err
			_ = masterDB.Close()
		}

		time.Sleep(fluentMSSQLRetryDelay(attempt))
	}

	return nil, fmt.Errorf("ping MSSQL master within readiness window: %w", lastErr)
}

func fluentMSSQLRetryDelay(attempt int) time.Duration {
	delay := time.Duration(attempt) * 500 * time.Millisecond
	if delay > 5*time.Second {
		return 5 * time.Second
	}
	return delay
}

func fluentEnv(key string, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func fluentEnvUint16(key string, defaultVal uint16) uint16 {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.Atoi(val)
		if err == nil {
			return uint16(parsed)
		}
	}
	return defaultVal
}

func fluentStrict() bool {
	switch strings.ToLower(os.Getenv("FABRIC_INTEGRATION_STRICT")) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
