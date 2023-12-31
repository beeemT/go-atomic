package example

import (
	"context"

	"github.com/beeemT/go-atomic/generic"
	"github.com/pkg/errors"
)

type (
	FooRepo struct {
		remote generic.SQLRemote
	}

	BarRepo struct {
		remote generic.SQLRemote
	}
)

func NewFooRepo(remote generic.SQLRemote) FooRepo {
	return FooRepo{
		remote: remote,
	}
}

func NewBarRepo(remote generic.SQLRemote) BarRepo {
	return BarRepo{
		remote: remote,
	}
}

func (f FooRepo) Create(ctx context.Context, foo Foo) error {
	_, err := f.remote.ExecContext(ctx, "INSERT INTO foo (id) VALUES ($1)", foo.ID)
	return errors.Wrap(err, "inserting foo")
}

func (b BarRepo) Create(ctx context.Context, bar Bar) error {
	_, err := b.remote.ExecContext(ctx, "INSERT INTO bar (id) VALUES ($1)", bar.ID)
	return errors.Wrap(err, "inserting bar")
}
