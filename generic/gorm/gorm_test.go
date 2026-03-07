package gorm_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	atomicgorm "github.com/beeemT/go-atomic/generic/gorm"
)

// testRemote is a minimal remote type used in tests.
type testRemote struct{}

// mockGormState is shared mutable state across mockGormDB instances produced by
// Begin/Commit/Rollback, allowing assertions on which operations were called.
type mockGormState struct {
	beginErr    error
	rollbackErr error
	commitErr   error

	beginCalled    bool
	rollbackCalled bool
	commitCalled   bool
}

// mockGormDB implements atomicgorm.GormlikeDB[testRemote].
type mockGormDB struct {
	state *mockGormState
	// err is the error reported by Error() on this specific instance, set by the
	// operation that produced it (Begin, Commit, Rollback).
	err error
}

func (m *mockGormDB) Begin(opts ...*sql.TxOptions) atomicgorm.GormlikeDB[testRemote] {
	m.state.beginCalled = true
	return &mockGormDB{state: m.state, err: m.state.beginErr}
}

func (m *mockGormDB) Rollback() atomicgorm.GormlikeDB[testRemote] {
	m.state.rollbackCalled = true
	return &mockGormDB{state: m.state, err: m.state.rollbackErr}
}

func (m *mockGormDB) Commit() atomicgorm.GormlikeDB[testRemote] {
	m.state.commitCalled = true
	return &mockGormDB{state: m.state, err: m.state.commitErr}
}

func (m *mockGormDB) Remote() testRemote { return testRemote{} }
func (m *mockGormDB) Error() error       { return m.err }

// newMockDB constructs a root mockGormDB with the given per-operation errors.
func newMockDB(beginErr, rollbackErr, commitErr error) *mockGormDB {
	return &mockGormDB{
		state: &mockGormState{
			beginErr:    beginErr,
			rollbackErr: rollbackErr,
			commitErr:   commitErr,
		},
	}
}

// TestGormExecuterSuccess verifies the happy path: Begin → run → Commit, no errors.
func TestGormExecuterSuccess(t *testing.T) {
	db := newMockDB(nil, nil, nil)
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](db)

	runCalled := false
	err := executer.Execute(context.Background(), func(_ testRemote) error {
		runCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !runCalled {
		t.Error("run must be called on success path")
	}
	if !db.state.beginCalled {
		t.Error("Begin must be called")
	}
	if !db.state.commitCalled {
		t.Error("Commit must be called on success")
	}
	if db.state.rollbackCalled {
		t.Error("Rollback must not be called on success")
	}
}

// TestGormExecuterBeginError verifies that a Begin failure is returned before run is called.
func TestGormExecuterBeginError(t *testing.T) {
	sentinel := errors.New("begin error")
	db := newMockDB(sentinel, nil, nil)
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](db)

	runCalled := false
	err := executer.Execute(context.Background(), func(_ testRemote) error {
		runCalled = true
		return nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected begin sentinel in error chain: %v", err)
	}
	if runCalled {
		t.Error("run must not be called when Begin fails")
	}
	if db.state.commitCalled || db.state.rollbackCalled {
		t.Error("Commit and Rollback must not be called when Begin fails")
	}
}

// TestGormExecuterRunError verifies that a run failure triggers Rollback.
func TestGormExecuterRunError(t *testing.T) {
	sentinel := errors.New("run error")
	db := newMockDB(nil, nil, nil)
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](db)

	err := executer.Execute(context.Background(), func(_ testRemote) error {
		return sentinel
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected run sentinel in error chain: %v", err)
	}
	if !db.state.rollbackCalled {
		t.Error("Rollback must be called when run fails")
	}
	if db.state.commitCalled {
		t.Error("Commit must not be called when run fails")
	}
}

// TestGormExecuterRunErrorRollbackError verifies that both run and rollback errors appear
// in the combined error.
func TestGormExecuterRunErrorRollbackError(t *testing.T) {
	runSentinel := errors.New("run error")
	rollbackSentinel := errors.New("rollback error")
	db := newMockDB(nil, rollbackSentinel, nil)
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](db)

	err := executer.Execute(context.Background(), func(_ testRemote) error {
		return runSentinel
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, runSentinel) {
		t.Errorf("expected run sentinel in error chain: %v", err)
	}
	if !errors.Is(err, rollbackSentinel) {
		t.Errorf("expected rollback sentinel in error chain: %v", err)
	}
}

// TestGormExecuterCommitError verifies that a Commit failure is surfaced as an error.
// This covers the bug where errors.Wrap(err, ...) was used instead of errors.Wrap(db.Error(), ...)
// causing the commit error to be silently discarded.
func TestGormExecuterCommitError(t *testing.T) {
	sentinel := errors.New("commit error")
	db := newMockDB(nil, nil, sentinel)
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](db)

	err := executer.Execute(context.Background(), func(_ testRemote) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error on commit failure, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected commit sentinel in error chain: %v", err)
	}
}

// TestGormExecuterWithTxOptions verifies that WithTxOptions is accepted without error.
func TestGormExecuterWithTxOptions(t *testing.T) {
	db := newMockDB(nil, nil, nil)
	txOpts := &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true}
	executer := atomicgorm.NewExecuter[*mockGormDB, testRemote](
		db,
		atomicgorm.WithTxOptions[*mockGormDB, testRemote](txOpts),
	)

	err := executer.Execute(context.Background(), func(_ testRemote) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
