package sqlx

import (
	"context"
	"database/sql"

	"github.com/beeemT/go-atomic/generic"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
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
	tx, err := executer.db.BeginTxx(ctx, executer.txOpts)
	if err != nil {
		return errors.Wrap(err, "opening sqlx tx")
	}

	err = run(tx)
	if err != nil {
		err = errors.Wrap(err, "executing run")
		innerErr := tx.Rollback()
		if innerErr != nil {
			return multierr.Append(
				err,
				errors.Wrap(innerErr, "rolling back sqlx tx"),
			)
		}
		return err
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "committing sqlx tx")
	}

	return nil
}
