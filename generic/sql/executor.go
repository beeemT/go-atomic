// Package sql implements [generic.Executer] for the stdlib sql db
package sql

import (
	"context"
	"database/sql"

	"github.com/beeemT/go-atomic/generic"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var (
	_ generic.SQLRemote                   = (*sql.Tx)(nil)
	_ generic.Executer[generic.SQLRemote] = Executer{}
)

type (
	// Executer implements the [generic.Executer] interface for a stlib sql db
	Executer struct {
		db     *sql.DB
		txOpts *sql.TxOptions
	}

	// ExecuterOption configures the [Executer] instance
	ExecuterOption func(*Executer)
)

// WithTxOptions allows setting the TxOptions to use when opening a new transaction
func WithTxOptions(opts *sql.TxOptions) ExecuterOption {
	return func(e *Executer) {
		e.txOpts = opts
	}
}

// NewExecuter creates a new Executer
func NewExecuter(db *sql.DB, opts ...ExecuterOption) Executer {
	executer := Executer{
		db:     db,
		txOpts: &sql.TxOptions{},
	}

	for _, opt := range opts {
		opt(&executer)
	}

	return executer
}

// Execute executes the provided function in a transaction
func (executer Executer) Execute(ctx context.Context, run func(generic.SQLRemote) error) error {
	tx, err := executer.db.BeginTx(ctx, executer.txOpts)
	if err != nil {
		return errors.Wrap(err, "opening sql tx")
	}

	err = run(tx)
	if err != nil {
		err = errors.Wrap(err, "executing run")
		innerErr := tx.Rollback()
		if innerErr != nil {
			return multierr.Append( //nolint:wrapcheck //individual errors are wrapped
				err,
				errors.Wrap(innerErr, "rolling back sql tx"),
			)
		}

		return err
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "committing sql tx")
	}

	return nil
}
