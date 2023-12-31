package example

import (
	"context"

	"github.com/beeemT/go-atomic/generic"
	"github.com/pkg/errors"
)

type (
	// FooRepo is a sample repo based on the sql remote for the sample entity Foo
	FooRepo struct {
		remote generic.SQLRemote
	}

	// BarRepo is a sample repo based on the sql remote for the sample entity Bar
	BarRepo struct {
		remote generic.SQLRemote
	}
)

// NewFooRepo is a sample factory function for the FooRepo
func NewFooRepo(remote generic.SQLRemote) FooRepo {
	return FooRepo{
		remote: remote,
	}
}

// NewBarRepo is a sample factory function for the BarRepo
func NewBarRepo(remote generic.SQLRemote) BarRepo {
	return BarRepo{
		remote: remote,
	}
}

// Create creates a foo entity in the remote
func (f FooRepo) Create(ctx context.Context, foo Foo) error {
	_, err := f.remote.ExecContext(ctx, "INSERT INTO foo (id) VALUES ($1)", foo.ID)
	return errors.Wrap(err, "inserting foo")
}

// Create creates a bar entity in the remote
func (b BarRepo) Create(ctx context.Context, bar Bar) error {
	_, err := b.remote.ExecContext(ctx, "INSERT INTO bar (id) VALUES ($1)", bar.ID)
	return errors.Wrap(err, "inserting bar")
}
