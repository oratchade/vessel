// Package v1 applies versioned schema migrations with goose.
//
// It opens its own short-lived connection from a [db.DBConfig], the same
// configuration the rest of Vessel uses, runs SQL migrations read from an
// [io/fs.FS] (usually an embed.FS), and closes the connection with [Migrator.Close].
// No *sql.DB is exposed, and Vessel's connection pools are not touched.
//
// Migration files follow goose's SQL format: "-- +goose Up", optional
// "-- +goose StatementBegin"/"StatementEnd" around statements containing
// semicolons (function bodies), and "-- +goose NO TRANSACTION" for statements
// that refuse a transaction. The SQL itself is dialect-specific; keep one
// directory of migrations per dialect.
//
// See docs/MIGRATIONS.md.
package v1
