package v1

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"; Vessel's own PostgreSQL backend uses pgxpool
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/definition"
)

var (
	// ErrUnsupportedDriver is returned by [New] for a driver it cannot migrate,
	// including drivers registered through the plugin package.
	ErrUnsupportedDriver = errors.New("migrate: unsupported driver")
	// ErrNoMigrations is returned by [New] when the file system holds no migrations.
	ErrNoMigrations = errors.New("migrate: no migrations found")
	// ErrMySQLParseTime is returned by [New] for a MySQL configuration without
	// parseTime: goose reads the applied-at timestamps back as time.Time.
	ErrMySQLParseTime = errors.New("migrate: MySQL requires parseTime=true")
)

// State is whether a migration has been applied.
type State string

const (
	// StateApplied is a migration recorded in the version table.
	StateApplied State = "applied"
	// StatePending is a migration found in the file system but not applied yet.
	StatePending State = "pending"
)

// Result is the outcome of one applied migration.
type Result struct {
	Version  int64
	Source   string // file name, e.g. 00002_add_index.sql
	Duration time.Duration
	Empty    bool // the file held no statements; it is still recorded as applied
}

// Status describes one migration known to the file system.
type Status struct {
	Version   int64
	Source    string
	State     State
	AppliedAt time.Time // zero while pending
}

// Option configures a [Migrator].
type Option func(*options)

type options struct {
	versionTable string
	logger       db.Logger
}

// WithVersionTable sets the table that records applied migrations. The default
// is goose's "goose_db_version".
func WithVersionTable(name string) Option {
	return func(o *options) { o.versionTable = name }
}

// Migrator applies the migrations of one file system to one database.
type Migrator struct {
	provider *goose.Provider
}

// dialectFor maps a Vessel driver name to the database/sql driver that opens it
// and the goose dialect that migrates it.
func dialectFor(driver string) (sqlDriver string, dialect goose.Dialect, err error) {
	switch driver {
	case definition.DriverPostgres, definition.DriverPostgresAlias:
		return "pgx", goose.DialectPostgres, nil
	case definition.DriverMySQL:
		return "mysql", goose.DialectMySQL, nil
	case definition.DriverSQLite:
		return "sqlite", goose.DialectSQLite3, nil
	case definition.DriverMSSQL, definition.DriverMSSQLAlias:
		return "sqlserver", goose.DialectMSSQL, nil
	default:
		return "", "", fmt.Errorf("%w: %q", ErrUnsupportedDriver, driver)
	}
}

// lockerFor returns the goose option that serializes concurrent runs against
// one database, or nil where goose has no locker (SQLite, MSSQL): concurrent
// runs there are the caller's responsibility.
func lockerFor(dialect goose.Dialect) (goose.ProviderOption, error) {
	switch dialect {
	case goose.DialectPostgres:
		// An advisory lock, scoped to the database: migrating two databases at once
		// does not contend.
		locker, err := lock.NewPostgresSessionLocker()
		if err != nil {
			return nil, fmt.Errorf("migrate: postgres locker: %w", err)
		}
		return goose.WithSessionLocker(locker), nil
	case goose.DialectMySQL:
		locker, err := lock.NewMySQLTableLocker()
		if err != nil {
			return nil, fmt.Errorf("migrate: mysql locker: %w", err)
		}
		return goose.WithLocker(locker), nil
	default:
		return nil, nil
	}
}

// checkDSN refuses a connection string goose cannot work with, before any
// connection is opened.
func checkDSN(dialect goose.Dialect, dsn string) error {
	if dialect != goose.DialectMySQL {
		return nil
	}
	parsed, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("migrate: parse mysql DSN: %w", err)
	}
	if !parsed.ParseTime {
		return ErrMySQLParseTime
	}
	return nil
}

func providerOptions(dialect goose.Dialect, o options) ([]goose.ProviderOption, error) {
	// Only the supplied file system: by default goose also runs Go migrations registered
	// anywhere in the process, which may belong to another database.
	opts := []goose.ProviderOption{goose.WithDisableGlobalRegistry(true)}
	if o.versionTable != "" {
		opts = append(opts, goose.WithTableName(o.versionTable))
	}
	if o.logger != nil {
		// goose logs only when verbose.
		opts = append(opts, goose.WithVerbose(true), goose.WithSlog(slog.New(&loggerHandler{logger: o.logger})))
	}
	locker, err := lockerFor(dialect)
	if err != nil {
		return nil, err
	}
	if locker != nil {
		opts = append(opts, locker)
	}
	return opts, nil
}

func open(sqlDriver string, dialect goose.Dialect, dsn string) (*sql.DB, error) {
	conn, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("migrate: open %s: %w", sqlDriver, err)
	}
	if dialect == goose.DialectSQLite3 {
		// Every connection to an in-memory SQLite database is a separate, empty
		// database. Not applied elsewhere: the PostgreSQL session lock holds one
		// connection while the migrations run on another.
		conn.SetMaxOpenConns(1)
	}
	return conn, nil
}

// New prepares a Migrator for the database cfg describes and the migrations in
// fsys. It opens a connection pool of its own; call [Migrator.Close] to release it.
func New(cfg db.DBConfig, fsys fs.FS, opts ...Option) (*Migrator, error) {
	if cfg == nil {
		return nil, errors.New("migrate: nil config")
	}
	if fsys == nil {
		return nil, errors.New("migrate: nil file system")
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	sqlDriver, dialect, err := dialectFor(cfg.Driver())
	if err != nil {
		return nil, err
	}
	dsn := cfg.DSN()
	if err := checkDSN(dialect, dsn); err != nil {
		return nil, err
	}
	providerOpts, err := providerOptions(dialect, o)
	if err != nil {
		return nil, err
	}

	conn, err := open(sqlDriver, dialect, dsn)
	if err != nil {
		return nil, err
	}

	provider, err := goose.NewProvider(dialect, conn, fsys, providerOpts...)
	if err != nil {
		_ = conn.Close()
		if errors.Is(err, goose.ErrNoMigrations) {
			return nil, fmt.Errorf("%w: %w", ErrNoMigrations, err)
		}
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Migrator{provider: provider}, nil
}

// Up applies every pending migration in version order, and returns the ones it
// applied. On failure it returns those applied before the failure, and an error
// naming the failing file; the database stays at the last migration that
// succeeded.
func (m *Migrator) Up(ctx context.Context) ([]Result, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		var partial *goose.PartialError
		if errors.As(err, &partial) {
			failed := sourceName(partial.Failed.Source)
			return toResults(partial.Applied), fmt.Errorf("migrate: %s: %w", failed, partial.Err)
		}
		return toResults(results), fmt.Errorf("migrate: up: %w", err)
	}
	return toResults(results), nil
}

// Status lists every migration in the file system, applied or pending.
func (m *Migrator) Status(ctx context.Context) ([]Status, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate: status: %w", err)
	}
	out := make([]Status, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, Status{
			Version:   s.Source.Version,
			Source:    sourceName(s.Source),
			State:     State(s.State),
			AppliedAt: s.AppliedAt,
		})
	}
	return out, nil
}

// Version returns the highest applied version, or 0 when none is applied.
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	version, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate: version: %w", err)
	}
	return version, nil
}

// Close releases the Migrator's connection pool.
func (m *Migrator) Close() error {
	if err := m.provider.Close(); err != nil {
		return fmt.Errorf("migrate: close: %w", err)
	}
	return nil
}

func toResults(results []*goose.MigrationResult) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		out = append(out, Result{
			Version:  r.Source.Version,
			Source:   sourceName(r.Source),
			Duration: r.Duration,
			Empty:    r.Empty,
		})
	}
	return out
}

func sourceName(s *goose.Source) string {
	if s == nil {
		return ""
	}
	return path.Base(s.Path)
}
