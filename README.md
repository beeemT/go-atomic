# go-atomic [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov] [![Go Report Card][reportcard-img]][reportcard]

A package enabling go business layers to define blocks of code accessing remote systems in a sql-transaction like way.

## Installation

```shell
$ go get -u github.com/beeemT/go-atomic
```

## Usage

1. Choose the appropriate executor for your datasource.
2. Open the data source as usual.
3. Create an executor with the data source.
4. Create a function which generates new resource/repository instances from the provided remote interface (eg [generic sql remote](generic/remotes.go)).
5. Create a new transacter with the executor and generation function.
6. Perform the data source interaction within the transacter's Transact block.

It is most useful to put interactions with services which do not allow the use of
transactions at the and of the transact block. This yields consistency between the
systems in more scenarios.

Since Transact allows automatic retries (depending on executor and the transacter options) it is a
good practice (if possible) to implement idempotency keys in remote systems if they do not allow
the use of transactions (using keys which are not generated within a Transact block).

Since these transact blocks (depending on interactions with remote systems) might result in longer
running transactions this can lead to contention on the data source. It is generally a good practice
to implement optional row locking on reading data which will later be updated in the Transact block.
One such example would be cockroachdb's / postgresql's 'SELECT ... FOR UPDATE'.

## Example
```go
// Choose whichever executor fits your use case
sqlDb, err := stdlibsql.Open("postgres", "postgresql://user:password@localhost:5432/dbname")
if err != nil {
	panic(err)
}

executor := sql.NewExecuter(sqlDb)

guard := generic.NewTransacter[generic.SQLRemote, Resources](
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
```

See the [documentation][doc] for a complete API specification.

For an example see the [example folder](example/transactor.go) of the relevant version.

## Development Status

Building tests.

---

Released under the [MIT License](LICENSE.txt).

[doc-img]: https://godoc.org/github.com/beeemT/go-atomic?status.svg
[doc]: https://godoc.org/github.com/beeemT/go-atomic
[ci-img]: https://github.com/beeemT/go-atomic/actions/workflows/go.yml/badge.svg
[ci]: https://github.com/beeemT/go-atomic/actions/workflows/go.yml
[cov-img]: https://codecov.io/gh/beeemT/go-atomic/branch/main/graph/badge.svg
[cov]: https://codecov.io/gh/beeemT/go-atomic
[reportcard-img]: https://goreportcard.com/badge/github.com/beeemT/go-atomic
[reportcard]: https://goreportcard.com/report/github.com/beeemT/go-atomic
