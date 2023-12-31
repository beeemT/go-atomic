package example

import (
	"context"
	stdlibsql "database/sql"

	"github.com/beeemT/go-atomic"
	"github.com/beeemT/go-atomic/generic"
	"github.com/beeemT/go-atomic/generic/sql"
	"github.com/pkg/errors"
)

// Resources is a registry struct holding all the repos we want to be managed and created
// by the transacter.
type Resources struct {
	Foos FooRepo
	Bars BarRepo
}

// Example shows an example flow of how to setup and use the transacter
func Example() {
	// Choose whichever executor fits your use case
	sqlDB, err := stdlibsql.Open("postgres", "postgresql://user:password@localhost:5432/dbname")
	if err != nil {
		panic(err)
	}

	executor := sql.NewExecuter(sqlDB)

	var guard atomic.Transacter[Resources] //nolint:gosimple // this if for example purposes
	guard = generic.NewTransacter[generic.SQLRemote, Resources](
		executor,
		resourcesFactory,
	)

	err = guard.Transact(context.TODO(), func(ctx context.Context, resources Resources) error {
		err := resources.Foos.Create(ctx, Foo{
			ID: int(1),
		})
		if err != nil {
			return errors.Wrap(err, "creating foo")
		}

		err = resources.Bars.Create(ctx, Bar{
			ID: int(1),
		})
		if err != nil {
			return errors.Wrap(err, "creating bar")
		}

		// eg here we can do some more business logic, like payments or other things
		// Foo and Bar only get committed if the payment was successful

		return nil
	})

	if err != nil {
		panic(err)
	}
}

func resourcesFactory(
	_ context.Context,
	_ *generic.Transacter[generic.SQLRemote, Resources],
	tx generic.SQLRemote,
) (Resources, error) {
	// it is also possible to define business services which in turn need a transacter themselves,
	// then the factory function can use the passed transacter.

	return Resources{
		Foos: NewFooRepo(tx),
		Bars: NewBarRepo(tx),
	}, nil
}
