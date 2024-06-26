// Package gorm provides a way to integrate multiple different versions of gorm / potentially
// other databases go-atomic if they implement the GormlikeDB interface. This can be achieved
// by embedding the gorm.DB object in a struct which then implements the GormlikeDB interface.
// As remote either the [generic.GormRemote] can be used or a custom interface definition which is
// a subset of the methods offered by gorm.
package gorm

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type (
	// GormlikeDB is an interface that allows different versions of gorm or other similar db's to
	// be used as executer for [generic.Transacter].
	GormlikeDB[Remote any] interface {
		Begin(...*sql.TxOptions) GormlikeDB[Remote]
		Rollback() GormlikeDB[Remote]
		Commit() GormlikeDB[Remote]
		Remote() Remote
		Error() error
	}

	// Executer implements the [generic.Executer] interface for a sqlx db
	Executer[T GormlikeDB[Remote], Remote any] struct {
		db     T
		txOpts *sql.TxOptions
	}

	// ExecuterOption configures the [Executer] instance
	ExecuterOption[T GormlikeDB[Remote], Remote any] func(*Executer[T, Remote])
)

// WithTxOptions allows setting the TxOptions to use when opening a new transaction
func WithTxOptions[T GormlikeDB[Remote], Remote any](
	opts *sql.TxOptions,
) ExecuterOption[T, Remote] {
	return func(e *Executer[T, Remote]) {
		e.txOpts = opts
	}
}

// NewExecuter creates a new Executer
func NewExecuter[T GormlikeDB[Remote], Remote any](
	db T,
	opts ...ExecuterOption[T, Remote],
) Executer[T, Remote] {
	executer := Executer[T, Remote]{
		db:     db,
		txOpts: &sql.TxOptions{},
	}

	for _, opt := range opts {
		opt(&executer)
	}

	return executer
}

// Execute executes the provided function in a transaction
func (executer Executer[T, Remote]) Execute(_ context.Context, run func(Remote) error) error {
	db := executer.db.Begin(executer.txOpts)
	if db.Error() != nil {
		return errors.Wrap(db.Error(), "opening gorm tx")
	}

	err := run(db.Remote())
	if err != nil {
		db = db.Rollback()
		if db.Error() != nil {
			return multierr.Append( //nolint:wrapcheck //individual errors are wrapped
				err,
				errors.Wrap(db.Error(), "rolling back gorm tx"),
			)
		}

		return err
	}

	db = db.Commit()
	if db.Error() != nil {
		return errors.Wrap(err, "committing gorm tx")
	}

	return nil
}
