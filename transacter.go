// Package atomic contains interfaces and shared code for the individual driver implementations
// to achieve transaction like behavior on access of remote systems in an implementation agnostic
// manner in business layers.
package atomic

import (
	"context"
)

type (
	// ContextKey is the type of the context keys used by the transacter.
	ContextKey string
)

const (
	// SessionContextKey is the key used to store active transacter sessions in context.
	SessionContextKey ContextKey = "session"
)

// Transacter interface consists of the Transact method.
type Transacter[Resources any] interface {
	// Transact executes run atomically, rolling back on error and committing on return.
	// Atomicity (depending on implementation in the remote systems accessed) can only be guaranteed
	// for statements made on the fields of the provided Resources type.
	// Transact opens a new session before calling run and inserts it into the context.
	// If a Transact session is already present in the provided context Transact will reuse that
	// session.
	// Depending on the implementation of Transact it is possible that this leads to nested
	// transactions.
	// In this case on error in the inner call to Transact only the statements made in the inner run
	// functions are rollbacked.
	// For this to work it is necessary to always pass the use the context provided to run as the
	// parent context in statements inside the run function.
	Transact(ctx context.Context, run func(context.Context, Resources) error) error
}
