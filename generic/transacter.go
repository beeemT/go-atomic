// Package generic implements a generic Transacter which is compatible with a wide range of
// executors for different data sources.
package generic

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/beeemT/go-atomic"
)

type (
	// Transacter implements the Transacter interface for sqlx compatible databases.
	// It flattens statements on nested uses of the Transact method into one sqlx transaction.
	Transacter[Remote any, Resources any] struct {
		executer Executer[Remote]

		createResources func(
			ctx context.Context,
			transacter *Transacter[Remote, Resources],
			tx Remote,
		) (Resources, error)

		retry func(backoffs []time.Duration, run func() error) error

		backoffs []time.Duration
	}

	// Session models all info passed from transacter through context to other nested
	// Transact calls.
	Session[Remote any] struct {
		Tx Remote
	}

	// Executer models the handler for the remote specific transaction logic.
	// A call to execute should:
	// - Open a new transaction
	// - Run the provided function
	// - On error roll back the transaction
	// - On success commit the transaction
	// The provided context is not passed down to the actual function that is executed,
	// changes or additions to the context in Execute are not propagated.
	Executer[Remote any] interface {
		Execute(context.Context, func(Remote) error) error
	}

	// TransacterOption is used to configure a Transacter.
	TransacterOption[Remote any, Resources any] func(*Transacter[Remote, Resources])
)

var _ atomic.Transacter[struct{}] = (*Transacter[struct{}, struct{}])(nil)

// NewTransacter creates a new Transacter.
// Remote is the type passed to the createResources function, ie a sql.DB or sql.Tx object
// (depending on the chosen executor). The executor must match the chosen remote type.
// It is useful to use an interface as remote like the [SQLRemote] or [SQLXRemote] interfaces,
// as it permits the use of eg repositories with both sql.DB and sql.Tx.
//
// By default sets:
//   - [atomic.DefaultRetry] as the retry function.
//   - [atomic.DefaultBackoffs] as the backoffs to use on retry.
//
// createResources is supposed to do any setup or new instantiation of members of Resources, ie
// create new repositories using the provided Remote.
// For an example view see [example.Example].
func NewTransacter[Remote any, Resources any](
	executer Executer[Remote],
	createResources func(
		ctx context.Context,
		transacter *Transacter[Remote, Resources],
		tx Remote,
	) (Resources, error),
	opts ...TransacterOption[Remote, Resources],
) Transacter[Remote, Resources] {
	transacter := Transacter[Remote, Resources]{
		executer:        executer,
		createResources: createResources,
		retry:           atomic.DefaultRetry,
		backoffs:        atomic.DefaultBackoffs,
	}

	for _, opt := range opts {
		opt(&transacter)
	}

	return transacter
}

// Transact will run run in a sqlx Session.
// If a session is present in ctx at [atomic.SessionContextKey] it will use the existing session,
// else it will create a new session and insert it into the context.
// It does not support nested transactions, rather all statements are flattened into a single
// transaction.
// Using the Session it executes the resource creation function which is provided on creation of
// the transacter to produce the resources for calling run.
// If an error is returned from run the outermost call of Transact will handle the error with the
// provided retry function.
func (transacter Transacter[Remote, Resources]) Transact(
	ctx context.Context,
	run func(context.Context, Resources) error,
) (err error) {
	session := ctx.Value(atomic.SessionContextKey)

	if session == nil {
		err = errors.Wrap(
			transacter.retry(
				transacter.backoffs,
				func() error {
					return transacter.executer.Execute( //nolint:wrapcheck //done outside closure
						ctx,
						transacter.inSession(ctx, run),
					)
				}),
			"new sqlx transaction",
		)
	} else {
		s, ok := session.(*Session[Remote])
		if !ok {
			return fmt.Errorf("cannot use %T as *Session", session)
		}

		err = errors.Wrap(
			transacter.inSession(ctx, run)(s.Tx),
			"using sqlx transaction from context",
		)
	}

	return errors.Wrap(err, "running transaction")
}

func (transacter *Transacter[Remote, Resources]) inSession(
	ctx context.Context,
	run func(context.Context, Resources) error,
) func(Remote) error {
	return func(tx Remote) error {
		ctx = context.WithValue(
			ctx,
			atomic.SessionContextKey,
			&Session[Remote]{
				Tx: tx,
			},
		)

		registry, err := transacter.createResources(ctx, transacter, tx)
		if err != nil {
			return fmt.Errorf("creating registry: %w", err)
		}

		err = run(ctx, registry)
		if err != nil {
			return fmt.Errorf("executing run: %w", err)
		}

		return nil
	}
}
