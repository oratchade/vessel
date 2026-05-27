# Vessel Portability Matrix

Vessel is portable-first for common SQL and explicit about dialect-specific
behavior. Values are parameterized; identifiers and raw SQL fragments remain
caller-owned inputs and should be trusted or allowlisted.

| Feature | MySQL | PostgreSQL | SQLite | MSSQL |
| --- | --- | --- | --- | --- |
| SELECT builder | Yes | Yes | Yes | Yes |
| WHERE conditions | Yes | Yes | Yes | Yes |
| Parameterized HAVING | Yes | Yes | Yes | Yes |
| Raw HAVING | Yes, trusted SQL | Yes, trusted SQL | Yes, trusted SQL | Yes, trusted SQL |
| Projection aliases | Yes | Yes | Yes | Yes |
| Raw projections | Yes, trusted SQL | Yes, trusted SQL | Yes, trusted SQL | Yes, trusted SQL |
| Joins | Yes | Yes | Yes | Yes |
| Joined UPDATE | `UPDATE ... JOIN` | `UPDATE ... FROM` | `UPDATE ... FROM` | `UPDATE ... FROM` |
| Joined DELETE | `DELETE t FROM ... JOIN` | `DELETE ... USING` | Unsupported | `DELETE ... FROM` |
| SELECT limit/offset | `LIMIT/OFFSET` | `LIMIT/OFFSET` | `LIMIT/OFFSET` | `OFFSET/FETCH`, requires `ORDER BY` |
| Mutation order/limit | UPDATE/DELETE without joins | Explicit unsupported errors | Explicit unsupported errors unless supported by runtime build | Explicit unsupported errors |
| INSERT returning preview | Ignored | `RETURNING` | Ignored | `OUTPUT inserted...` |
| UPDATE returning preview | Ignored | `RETURNING` | Ignored | `OUTPUT inserted...` |
| DELETE returning preview | Ignored | `RETURNING` | Ignored | `OUTPUT deleted...` |
| Returning execution | Unsupported, clear error | Unsupported, clear error | Unsupported, clear error | Unsupported, clear error |
| Upsert do update | `ON DUPLICATE KEY UPDATE` | `ON CONFLICT DO UPDATE` | `ON CONFLICT DO UPDATE` | Unsupported, clear error |
| Upsert do nothing | no-op duplicate-key update | `ON CONFLICT DO NOTHING` | `ON CONFLICT DO NOTHING` | Unsupported, clear error |
| Case-insensitive search | `LOWER(col) LIKE LOWER(?)` | `LOWER(col) LIKE LOWER(?)` | `LOWER(col) LIKE LOWER(?)` | `LOWER(col) LIKE LOWER(?)` |
| Array membership | Expand values with `IN` | Expand values with `IN`; use raw SQL for `ANY` | Expand values with `IN` | Expand values with `IN` |
| Latest per group/window SQL | Raw SQL | Raw SQL | Raw SQL | Raw SQL |
| Typed scanning | `ScanAll` / `ScanOne` | `ScanAll` / `ScanOne` | `ScanAll` / `ScanOne` | `ScanAll` / `ScanOne` |
| Transaction options | `Begin`/`WithTransaction` options | `Begin`/`WithTransaction` options | `Begin`/`WithTransaction` options | `Begin`/`WithTransaction` options |
| Savepoints | `SAVEPOINT` helpers | `SAVEPOINT` helpers | `SAVEPOINT` helpers | `SAVE TRANSACTION`; release unsupported |

## Portable Defaults

- Prefer condition helpers such as `Equal`, `In`, `IsNull`, `IsNotNull`, and
  `ILike` for dynamic values.
- Prefer `Column` and `ColumnAs` for identifiers.
- Use `ColumnRaw`, `ColumnRawAs`, `HavingRaw`, `QueryRaw`, and `Exec` only for
  trusted SQL fragments.
- Use app-generated IDs plus `InsertAndFetch` for portable create-and-fetch
  flows.
- Pass `TransactionOptions` to `Begin` or `WithTransaction` when isolation
  level or read-only behavior matters.

## Explicitly Not Portable

- PostgreSQL arrays and `ANY`.
- PostgreSQL/Timescale `DISTINCT ON`.
- Database-specific casts such as `ip_address::text`.
- Row-returning mutation execution via `RETURNING` or `OUTPUT`.
- MSSQL upsert via `MERGE`.
