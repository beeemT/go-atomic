# go-atomic [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov] [![Go Report Card][reportcard-img]][reportcard]

A package enabling go business layers to define blocks of code accessing remote systems in a sql-transaction like way. Essentially this allows the use of the Unit of Work pattern with go repositories.

## What is go-atomic?

`go-atomic` provides a straightforward way to implement the [Unit of Work pattern](https://martinfowler.com/eaaCatalog/unitOfWork.html) in your Go applications. This is particularly useful when your business logic involves multiple steps that need to be performed atomically (i.e., either they all succeed, or they all fail, leaving the system in a consistent state).

For example, imagine transferring money between two bank accounts. This involves two separate operations: debiting one account and crediting another. If the credit operation fails after the debit has already succeeded, you'll have inconsistent data. `go-atomic` helps you group these operations into a single transactional block, ensuring that they are all committed to the database together or rolled back if any single operation fails.

### Core Concepts

`go-atomic` is built around a few core concepts:

*   **Transacter**: The central component of the library. It orchestrates the transaction, ensuring that all operations within a `Transact` block are executed within the same database transaction.
*   **Executor**: Manages the lifecycle of a transaction (begin, commit, rollback). `go-atomic` provides several executors for different database libraries, including standard `database/sql`, `sqlx`, and `gorm`.
*   **Repository**: Your data access layer. These are the components that query and modify your database. With `go-atomic`, you design your repositories to work with a transactional object provided by the `Transacter`.
*   **Resources**: A simple struct that groups all the repositories that need to participate in the transaction.
*   **Resource Factory**: A function that you provide. Its job is to create an instance of your `Resources` struct, initializing all repositories with the same transaction object. This ensures that all your repositories share the same transaction.

## Installation

```shell
$ go get -u github.com/beeemT/go-atomic
```

## Usage

Using `go-atomic` involves the following steps, which correspond to the numbered comments in the example above:

1.  **Define your entities**: These are the plain Go structs that map to your database tables.

2.  **Define your repositories**: A repository encapsulates the data access logic for a single entity. Each repository should be designed to work with a `generic.SQLRemote`, which allows it to be used in a transaction.

3.  **Define a `Resources` struct**: This struct acts as a container for all the repositories that will participate in a single transaction.

4.  **Create a resource factory function**: This function is responsible for creating an instance of your `Resources` struct. `go-atomic` will invoke this function and pass it the underlying transaction object, ensuring all your repositories share the same transaction.

5.  **Create an `Executor` and a `Transacter`**: The `Executor` is created from your database connection (e.g., `*sql.DB`). The `Transacter` is then created with the `Executor` and your resource factory function.

6.  **Execute your business logic within a `Transact` block**: The `Transacter.Transact()` method takes a function where you can safely use your repositories. `go-atomic` guarantees that all database operations within this block will be committed or rolled back atomically. If the function returns an error, the transaction is rolled back; otherwise, it's committed.

## Example

Here is a complete, runnable example that demonstrates how to use `go-atomic`.

```go
package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/beeemT/go-atomic/generic"
	gosql "github.com/beeemT/go-atomic/generic/sql"
	"github.com/pkg/errors"
	_ "github.com/mattn/go-sqlite3"
)

// 1. Define your entities
// These are the data structures that represent your database records.

type Foo struct {
	ID int
}

type Bar struct {
	ID int
}

// 2. Define your repositories
// Repositories are responsible for the persistence logic. They are designed to work
// with a generic.SQLRemote, which can be either a standard sql.DB or a sql.Tx.

type FooRepo struct {
	remote generic.SQLRemote
}

func NewFooRepo(remote generic.SQLRemote) *FooRepo {
	return &FooRepo{remote: remote}
}

func (r *FooRepo) Create(ctx context.Context, foo Foo) error {
	_, err := r.remote.ExecContext(ctx, "INSERT INTO foo (id) VALUES ($1)", foo.ID)
	return errors.Wrap(err, "inserting foo")
}

type BarRepo struct {
	remote generic.SQLRemote
}

func NewBarRepo(remote generic.SQLRemote) *BarRepo {
	return &BarRepo{remote: remote}
}

func (r *BarRepo) Create(ctx context.Context, bar Bar) error {
	_, err := r.remote.ExecContext(ctx, "INSERT INTO bar (id) VALUES ($1)", bar.ID)
	return errors.Wrap(err, "inserting bar")
}

// 3. Define a Resources struct
// This struct holds all the repositories that will participate in the transaction.

type Resources struct {
	Foos *FooRepo
	Bars *BarRepo
}

// 4. Create a resource factory function
// This function initializes your Resources struct. The Transacter will call this
// function and pass in the transaction object (generic.SQLRemote).

func resourceFactory(
	_ context.Context,
	_ *generic.Transacter[generic.SQLRemote, Resources],
	tx generic.SQLRemote,
) (Resources, error) {
	return Resources{
		Foos: NewFooRepo(tx),
		Bars: NewBarRepo(tx),
	}, nil
}

func main() {
	// For this example, we'll use an in-memory SQLite database.
	// In a real application, you would connect to your actual database (e.g., PostgreSQL).
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create the necessary tables for the example.
	if _, err := db.Exec(`CREATE TABLE foo (id INT)`); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`CREATE TABLE bar (id INT)`); err != nil {
		panic(err)
	}

	// 5. Create an Executor and a Transacter
	// The Executor wraps your database connection, and the Transacter orchestrates the transaction.
	executor := gosql.NewExecuter(db)
	transacter := generic.NewTransacter[generic.SQLRemote, Resources](executor, resourceFactory)

	// 6. Execute your business logic within a Transact block
	// The Transacter ensures that all operations inside this block are atomic.
	err = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
		fmt.Println("Creating foo...")
		if err := r.Foos.Create(ctx, Foo{ID: 1}); err != nil {
			return errors.Wrap(err, "failed to create foo")
		}

		fmt.Println("Creating bar...")
		if err := r.Bars.Create(ctx, Bar{ID: 1}); err != nil {
			return errors.Wrap(err, "failed to create bar")
		}

		// If this function returns an error, the transaction will be rolled back.
		// If it returns nil, the transaction will be committed.
		return nil
	})

	if err != nil {
		panic(err)
	}

	fmt.Println("Transaction committed successfully!")
}
```

See the [documentation][doc] for a complete API specification.

## Available Executors

`go-atomic` provides several executors for popular database libraries:

*   **[generic/sql](generic/sql/executor.go)**: For the standard `database/sql` package.
*   **[generic/sqlx](generic/sqlx/sqlx.go)**: For `sqlx`.
*   **[generic/gorm](generic/gorm/gorm.go)**: For `gorm`.
*   **[generic/crdb](generic/crdb/crdb.go)**: For CockroachDB-specific transaction logic.

You can choose the one that best fits your project's stack.

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
