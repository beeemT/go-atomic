package generic

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type (
	// SQLRemote is a subset of the shared methods of sql.DB and sql.Tx
	// To push context based development purposefully only the Context variants are used.
	SQLRemote interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
		PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
		QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	}

	// SQLXRemote is a subset of the shared methods of sqlx.DB and sqlx.Tx
	// To push context based development purposefully only the Context variants are used.
	// The exception to this rule is NamedQuery, as the sqlx project apparently refuses to
	// include a contextualized version on sqlx.Tx (see: https://github.com/jmoiron/sqlx/issues/447)
	SQLXRemote interface {
		BindNamed(query string, arg any) (string, []any, error)
		DriverName() string
		GetContext(ctx context.Context, dest any, query string, args ...any) error
		MustExecContext(ctx context.Context, query string, args ...any) sql.Result
		NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
		NamedQuery(query string, arg any) (*sqlx.Rows, error)
		NamedStmtContext(ctx context.Context, stmt *sqlx.NamedStmt) *sqlx.NamedStmt
		PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
		PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
		QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row
		QueryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error)
		Rebind(query string) string
		SelectContext(ctx context.Context, dest any, query string, args ...any) error
	}
)
