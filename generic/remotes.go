package generic

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

	// GormRemote is a subset of the methods on [gorm.io/gorm.DB], explicitly excluding Transaction
	// related methods to possibly avoid programming errors through manually using transactions
	// within [Transacter.Transact] closures.
	GormRemote interface {
		AddError(err error) error
		Assign(attrs ...any) *gorm.DB
		Association(column string) *gorm.Association
		Attrs(attrs ...any) *gorm.DB
		AutoMigrate(dst ...any) error
		Clauses(conds ...clause.Expression) *gorm.DB
		// Connection is not guaranteed to be transaction safe when used with go-atomic
		Connection(fc func(tx *gorm.DB) error) (err error)
		Count(count *int64) *gorm.DB
		Create(value any) *gorm.DB
		CreateInBatches(value any, batchSize int) *gorm.DB
		// Stataements run on DB are not guaranteed to be transaction safe when used with go-atomic
		DB() (*sql.DB, error)
		Debug() *gorm.DB
		Delete(value any, conds ...any) *gorm.DB
		Distinct(args ...any) *gorm.DB
		Exec(rawSQL string, values ...any) *gorm.DB
		Find(dest any, conds ...any) *gorm.DB
		FindInBatches(dest any, batchSize int, fc func(tx *gorm.DB, batch int) error) *gorm.DB
		First(dest any, conds ...any) *gorm.DB
		FirstOrCreate(dest any, conds ...any) *gorm.DB
		FirstOrInit(dest any, conds ...any) *gorm.DB
		Get(key string) (any, bool)
		Group(name string) *gorm.DB
		Having(query any, args ...any) *gorm.DB
		InnerJoins(query string, args ...any) *gorm.DB
		InstanceGet(key string) (any, bool)
		InstanceSet(key string, value any) *gorm.DB
		Joins(query string, args ...any) *gorm.DB
		Last(dest any, conds ...any) *gorm.DB
		Limit(limit int) *gorm.DB
		Migrator() gorm.Migrator
		Model(value any) *gorm.DB
		Not(query any, args ...any) *gorm.DB
		Offset(offset int) *gorm.DB
		Omit(columns ...string) *gorm.DB
		Or(query any, args ...any) *gorm.DB
		Order(value any) *gorm.DB
		Pluck(column string, dest any) *gorm.DB
		Preload(query string, args ...any) *gorm.DB
		Raw(rawSQL string, values ...any) *gorm.DB
		RollbackTo(name string) *gorm.DB
		Row() *sql.Row
		Rows() (*sql.Rows, error)
		Save(value any) *gorm.DB
		SavePoint(name string) *gorm.DB
		Scan(dest any) *gorm.DB
		ScanRows(rows *sql.Rows, dest any) error
		Scopes(funcs ...func(*gorm.DB) *gorm.DB) *gorm.DB
		Select(query any, args ...any) *gorm.DB
		Session(config *gorm.Session) *gorm.DB
		Set(key string, value any) *gorm.DB
		SetupJoinTable(model any, field string, joinTable any) error
		Table(name string, args ...any) *gorm.DB
		Take(dest any, conds ...any) *gorm.DB
		ToSQL(queryFn func(tx *gorm.DB) *gorm.DB) string
		Unscoped() *gorm.DB
		Update(column string, value any) *gorm.DB
		UpdateColumn(column string, value any) *gorm.DB
		UpdateColumns(values any) *gorm.DB
		Updates(values any) *gorm.DB
		Use(plugin gorm.Plugin) error
		Where(query any, args ...any) *gorm.DB
		WithContext(ctx context.Context) *gorm.DB
	}
)
