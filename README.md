# go-atomic [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov] [![Go Report Card][reportcard-img]][reportcard]

A Go package enabling business layers to define blocks of code accessing remote systems in a transaction-like manner. This library implements the **Unit of Work** pattern, allowing multiple repository operations to be executed atomically with automatic rollback on failure.

## Features

- **Transaction Management**: Automatic commit/rollback for database operations
- **Nested Transaction Support**: Flattens nested `Transact` calls into a single transaction
- **Multiple Database Support**: Works with standard library `sql`, `sqlx`, `GORM`, and `CockroachDB`
- **Automatic Retries**: Built-in retry logic for transient failures
- **Context Propagation**: Full context support for cancellation and timeout handling
- **Generic Implementation**: Type-safe resource management with Go generics

## Installation

```shell
go get -u github.com/beeemT/go-atomic
```

## Quick Start

```go
package main

import (
    "context"
    "database/sql"
    stdlibsql "database/sql"
    
    "github.com/beeemT/go-atomic"
    "github.com/beeemT/go-atomic/generic"
    genericsql "github.com/beeemT/go-atomic/generic/sql"
    "github.com/pkg/errors"
)

// Resources holds all repositories managed by the transacter
type Resources struct {
    Users UserRepository
    Orders OrderRepository
}

func main() {
    // 1. Open your database connection
    db, err := stdlibsql.Open("postgres", "postgresql://user:password@localhost:5432/mydb")
    if err != nil {
        panic(err)
    }
    
    // 2. Create an executor for your database
    executor := genericsql.NewExecuter(db)
    
    // 3. Create a transacter with your executor and resource factory
    transacter := generic.NewTransacter[generic.SQLRemote, Resources](
        executor,
        createResources,
    )
    
    // 4. Use Transact for atomic operations
    err = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        if err := r.Users.Create(ctx, User{ID: 1, Name: "Alice"}); err != nil {
            return errors.Wrap(err, "creating user")
        }
        
        if err := r.Orders.Create(ctx, Order{ID: 100, UserID: 1}); err != nil {
            return errors.Wrap(err, "creating order")
        }
        
        return nil // Commits on success
    })
    
    if err != nil {
        panic(err) // Rolled back on error
    }
}

// createResources instantiates repositories with the transaction context
func createResources(
    ctx context.Context,
    t *generic.Transacter[generic.SQLRemote, Resources],
    tx generic.SQLRemote,
) (Resources, error) {
    return Resources{
        Users:  NewUserRepository(tx),
        Orders: NewOrderRepository(tx),
    }, nil
}
```

## Core Concepts

### The Transacter Pattern

The `Transacter` pattern ensures that all database operations within a `Transact` block are executed atomically:

- **On Success**: All changes are committed
- **On Error**: All changes are rolled back
- **Nested Calls**: Inner `Transact` calls reuse the existing transaction (flattened)

### Remote Interfaces

The library provides interfaces that work with both direct connections and transactions:

| Interface | Description | Matches |
|-----------|-------------|---------|
| `generic.SQLRemote` | Standard library `sql.DB` and `sql.Tx` | `ExecContext`, `QueryContext`, etc. |
| `generic.SQLXRemote` | `sqlx.DB` and `sqlx.Tx` | All sqlx context methods |
| `generic.GormRemote` | GORM `*gorm.DB` | Most GORM methods (excludes transaction methods) |

### Executors

Executors handle the transaction lifecycle:

| Executor | Package | Use Case |
|----------|---------|----------|
| `sql.Executer` | `generic/sql` | Standard library `*sql.DB` |
| `sqlx.Executer` | `generic/sqlx` | `jmoiron/sqlx` |
| `gorm.Executer` | `generic/gorm` | GORM v2 |
| `crdb.Executer` | `generic/crdb` | CockroachDB with automatic retries |

## Examples

### Standard Library SQL

```go
package main

import (
    "context"
    "database/sql"
    
    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/sql"
)

type Resources struct {
    Products ProductRepository
    Inventory InventoryRepository
}

func main() {
    db, _ := sql.Open("postgres", "postgresql://localhost/mydb")
    
    // With custom transaction options (e.g., Read-Only)
    executor := sql.NewExecuter(db, sql.WithTxOptions(&sql.TxOptions{
        ReadOnly: true,
    }))
    
    transacter := generic.NewTransacter[generic.SQLRemote, Resources](
        executor,
        func(ctx context.Context, t *generic.Transacter[generic.SQLRemote, Resources], tx generic.SQLRemote) (Resources, error) {
            return Resources{
                Products:  NewProductRepo(tx),
                Inventory: NewInventoryRepo(tx),
            }, nil
        },
    )
    
    // All operations share the same transaction
    _ = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        product, err := r.Products.GetByID(ctx, 1)
        if err != nil {
            return err
        }
        
        inventory, err := r.Inventory.GetByProductID(ctx, product.ID)
        if err != nil {
            return err
        }
        
        // Business logic here...
        return nil
    })
}
```

### Using with sqlx

```go
package main

import (
    "context"
    
    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/sqlx"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

type Resources struct {
    Users  UserRepo
    Audits AuditRepo
}

func main() {
    db, _ := sqlx.Connect("postgres", "postgresql://localhost/mydb")
    
    executor := sqlx.NewExecuter(db)
    
    transacter := generic.NewTransacter[generic.SQLXRemote, Resources](
        executor,
        func(ctx context.Context, t *generic.Transacter[generic.SQLXRemote, Resources], tx generic.SQLXRemote) (Resources, error) {
            return Resources{
                Users:  NewUserRepo(tx),
                Audits: NewAuditRepo(tx),
            }, nil
        },
    )
    
    _ = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        // Use sqlx features like NamedExecContext
        user := User{Name: "Bob", Email: "bob@example.com"}
        if err := r.Users.Create(ctx, user); err != nil {
            return err
        }
        
        // Audit is automatically rolled back if this fails
        return r.Audits.Log(ctx, "user_created", user.Email)
    })
}
```

### Using with GORM

```go
package main

import (
    "context"
    "database/sql"

    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/gorm"
    gormlib "gorm.io/gorm"
)

type Resources struct {
    Customers CustomerRepo
    Invoices  InvoiceRepo
}

// GormDB wraps gorm.DB to implement GormlikeDB interface
type GormDB struct {
    *gormlib.DB
}

func (g GormDB) Begin(opts ...*sql.TxOptions) GormDB {
    return GormDB{g.DB.Begin(opts...)}
}

func (g GormDB) Rollback() GormDB {
    return GormDB{g.DB.Rollback()}
}

func (g GormDB) Commit() GormDB {
    return GormDB{g.DB.Commit()}
}

func (g GormDB) Remote() generic.GormRemote {
    return g.DB
}

func (g GormDB) Error() error {
    return g.DB.Error
}

func main() {
    db, _ := gormlib.Open(/* ... */)
    
    gormDB := GormDB{db}
    executor := gorm.NewExecuter[GormDB, generic.GormRemote](gormDB)
    
    transacter := generic.NewTransacter[generic.GormRemote, Resources](
        executor,
        func(ctx context.Context, t *generic.Transacter[generic.GormRemote, Resources], tx generic.GormRemote) (Resources, error) {
            return Resources{
                Customers: NewCustomerRepo(tx),
                Invoices:  NewInvoiceRepo(tx),
            }, nil
        },
    )
    
    _ = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        customer := Customer{Name: "Acme Corp"}
        if err := r.Customers.Create(ctx, &customer); err != nil {
            return err
        }
        
        invoice := Invoice{CustomerID: customer.ID, Amount: 1000}
        return r.Invoices.Create(ctx, &invoice)
    })
}
```

### CockroachDB with Automatic Retries

CockroachDB may return retryable errors during contention. The CRDB executor handles this automatically:

```go
package main

import (
    "context"
    
    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/crdb"
    "github.com/jmoiron/sqlx"
)

type Resources struct {
    Accounts AccountRepo
    Transfers TransferRepo
}

func main() {
    db, _ := sqlx.Connect("cockroach", "postgresql://localhost:26257/bank")
    
    // CRDB executor automatically retries on retryable errors
    executor := crdb.NewExecuter(db)
    
    transacter := generic.NewTransacter[generic.SQLXRemote, Resources](
        executor,
        func(ctx context.Context, t *generic.Transacter[generic.SQLXRemote, Resources], tx generic.SQLXRemote) (Resources, error) {
            return Resources{
                Accounts:  NewAccountRepo(tx),
                Transfers: NewTransferRepo(tx),
            }, nil
        },
    )
    
    // Transfer money atomically with automatic CRDB retries
    _ = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        from, err := r.Accounts.GetForUpdate(ctx, 1)
        if err != nil {
            return err
        }
        
        to, err := r.Accounts.GetForUpdate(ctx, 2)
        if err != nil {
            return err
        }
        
        if from.Balance < 100 {
            return errors.New("insufficient funds")
        }
        
        if err := r.Accounts.Debit(ctx, 1, 100); err != nil {
            return err
        }
        
        if err := r.Accounts.Credit(ctx, 2, 100); err != nil {
            return err
        }
        
        return r.Transfers.Create(ctx, 1, 2, 100)
    })
}
```

### Nested Transactions

Nested `Transact` calls are flattened into a single transaction:

```go
func (s *Service) ProcessOrder(ctx context.Context, order Order) error {
    return s.transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
        // Create order
        if err := r.Orders.Create(ctx, order); err != nil {
            return err
        }
        
        // This nested call reuses the same transaction
        if err := s.reserveInventory(ctx, order); err != nil {
            return err
        }
        
        // If payment fails, both order and inventory reservation are rolled back
        return s.processPayment(ctx, order)
    })
}

func (s *Service) reserveInventory(ctx context.Context, order Order) error {
    // ctx already contains the transaction from ProcessOrder
    return s.transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
        for _, item := range order.Items {
            if err := r.Inventory.Reserve(ctx, item.ProductID, item.Quantity); err != nil {
                return err
            }
        }
        return nil
    })
}
```

### Custom Retry Logic

Configure custom retry behavior for your use case:

```go
package main

import (
    "context"
    "time"
    
    "github.com/beeemT/go-atomic"
    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/sql"
)

func main() {
    db, _ := sql.Open("postgres", "postgresql://localhost/mydb")
    executor := sql.NewExecuter(db)
    
    // Custom backoff delays (retries 5 times)
    customBackoffs := []time.Duration{
        50 * time.Millisecond,
        100 * time.Millisecond,
        200 * time.Millisecond,
        500 * time.Millisecond,
        time.Second,
    }
    
    transacter := generic.NewTransacter[generic.SQLRemote, Resources](
        executor,
        createResources,
        generic.WithBackOffDelays[generic.SQLRemote, Resources](customBackoffs...),
    )
    
    // Or provide a completely custom retry function
    transacter = generic.NewTransacter[generic.SQLRemote, Resources](
        executor,
        createResources,
        generic.WithBackOffRetry[generic.SQLRemote, Resources](func(backoffs []time.Duration, run func() error) error {
            // Custom retry logic with exponential backoff
            var lastErr error
            for i, backoff := range backoffs {
                if err := run(); err != nil {
                    lastErr = err
                    if !isRetryable(err) {
                        return err
                    }
                    time.Sleep(backoff)
                    continue
                }
                return nil
            }
            return lastErr
        }),
    )
}
```

### Transaction with Isolation Level

```go
package main

import (
    "context"
    "database/sql"
    
    "github.com/beeemT/go-atomic/generic"
    "github.com/beeemT/go-atomic/generic/sql"
)

func main() {
    db, _ := sql.Open("postgres", "postgresql://localhost/mydb")
    
    // Serializable isolation for strict consistency
    executor := sql.NewExecuter(db, sql.WithTxOptions(&sql.TxOptions{
        Isolation: sql.LevelSerializable,
        ReadOnly:  false,
    }))
    
    transacter := generic.NewTransacter[generic.SQLRemote, Resources](
        executor,
        createResources,
    )
    
    _ = transacter.Transact(context.Background(), func(ctx context.Context, r Resources) error {
        // Operations run with serializable isolation
        return nil
    })
}
```

## Best Practices

### 1. Always Use the Provided Context

```go
// GOOD: Pass the context provided to your callback
transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
    return r.Users.Create(ctx, user) // Uses transaction context
})

// BAD: Using the outer context bypasses transaction management
transacter.Transact(ctx, func(innerCtx context.Context, r Resources) error {
    return r.Users.Create(ctx, user) // Wrong context!
})
```

### 2. Use Row Locking for Contention Scenarios

For operations that read-then-update, use row locking to prevent race conditions:

```go
transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
    // Lock the row for update
    account, err := r.Accounts.GetForUpdate(ctx, accountID)
    if err != nil {
        return err
    }
    
    // Safe to update - no concurrent modifications
    account.Balance -= amount
    return r.Accounts.Update(ctx, account)
})
```

SQL for PostgreSQL/CockroachDB:
```sql
SELECT * FROM accounts WHERE id = $1 FOR UPDATE
```

### 3. Keep Transactions Short

Long-running transactions can cause contention and timeouts:

```go
// GOOD: Do minimal work in the transaction
transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
    return r.Orders.Create(ctx, order)
})

// BAD: Long computations inside transaction
transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
    result := expensiveComputation(data) // Do this outside
    return r.Results.Save(ctx, result)
})
```

### 4. Handle Idempotency for External Systems

When integrating with non-transactional systems, use idempotency keys:

```go
transacter.Transact(ctx, func(ctx context.Context, r Resources) error {
    // First, record the intent with an idempotency key
    payment, err := r.Payments.CreateIntent(ctx, orderID, idempotencyKey)
    if err != nil {
        return err
    }
    
    // Call external payment service with idempotency key
    // If this fails and retries, the payment service will recognize the key
    result, err := paymentClient.Charge(ctx, payment.Amount, idempotencyKey)
    if err != nil {
        return err
    }
    
    return r.Payments.Confirm(ctx, payment.ID, result.TransactionID)
})
```

### 5. Design Repositories Around Remote Interfaces

```go
// Repository accepts the generic interface, works with both DB and Tx
type UserRepository struct {
    db generic.SQLRemote
}

func NewUserRepository(db generic.SQLRemote) UserRepository {
    return UserRepository{db: db}
}

func (r UserRepository) Create(ctx context.Context, user User) error {
    _, err := r.db.ExecContext(ctx, "INSERT INTO users (...) VALUES (...)", /* ... */)
    return err
}
```

## API Reference

See the [GoDoc documentation][doc] for complete API details.

### Key Types

- `atomic.Transacter[Resources]` - Main interface for transaction management
- `generic.Transacter[Remote, Resources]` - Generic implementation
- `generic.Executer[Remote]` - Interface for transaction executors
- `generic.SQLRemote`, `generic.SQLXRemote`, `generic.GormRemote` - Remote interfaces

### Options

- `generic.WithBackOffRetry(fn)` - Custom retry function
- `generic.WithBackOffDelays(durations...)` - Custom backoff durations
- `sql.WithTxOptions(opts)`, `sqlx.WithTxOptions(opts)`, `gorm.WithTxOptions(opts)` - Transaction options

## Development Status

Actively maintained. See [issues](https://github.com/beeemT/go-atomic/issues) for planned features.

## License

Released under the [MIT License](LICENSE.txt).

[doc-img]: https://godoc.org/github.com/beeemT/go-atomic?status.svg
[doc]: https://godoc.org/github.com/beeemT/go-atomic
[ci-img]: https://github.com/beeemT/go-atomic/actions/workflows/go.yml/badge.svg
[ci]: https://github.com/beeemT/go-atomic/actions/workflows/go.yml
[cov-img]: https://codecov.io/gh/beeemT/go-atomic/branch/main/graph/badge.svg
[cov]: https://codecov.io/gh/beeemT/go-atomic
[reportcard-img]: https://goreportcard.com/badge/github.com/beeemT/go-atomic
[reportcard]: https://goreportcard.com/report/github.com/beeemT/go-atomic
