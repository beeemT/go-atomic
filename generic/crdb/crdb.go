package crdb

import (
	"context"
	"database/sql"

	"github.com/beeemT/go-atomic/generic"
	crdb "github.com/cockroachdb/cockroach-go/crdb/crdbsqlx"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var (
	_ generic.SQLXRemote                   = (*sqlx.Tx)(nil)
	_ generic.Executer[generic.SQLXRemote] = Executer{}
)

type (
	Executer struct {
		db     *sqlx.DB
		txOpts *sql.TxOptions
	}

	ExecuterOption func(*Executer)
)

func WithTxOptions(opts *sql.TxOptions) ExecuterOption {
	return func(e *Executer) {
		e.txOpts = opts
	}
}

func NewExecuter(db *sqlx.DB, opts ...ExecuterOption) Executer {
	executer := Executer{
		db:     db,
		txOpts: &sql.TxOptions{},
	}

	for _, opt := range opts {
		opt(&executer)
	}

	return executer
}

func (executer Executer) Execute(ctx context.Context, run func(generic.SQLXRemote) error) error {
	return errors.Wrap(
		crdb.ExecuteTx(
			ctx,
			executer.db,
			executer.txOpts,
			func(tx *sqlx.Tx) error {
				return run(tx)
			},
		),
		"creating / executing crdb sqlx tx",
	)
}
