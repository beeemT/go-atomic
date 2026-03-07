package generic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	atomic "github.com/beeemT/go-atomic"
	"github.com/beeemT/go-atomic/generic"
)

// testRemote is a minimal remote type used in tests.
type testRemote struct {
	id int
}

// testResources holds a reference to the testRemote provided at resource-creation time.
type testResources struct {
	remote testRemote
}

// mockExecuter captures calls to Execute and delegates to fn.
type mockExecuter struct {
	fn func(ctx context.Context, run func(testRemote) error) error
}

func (m mockExecuter) Execute(ctx context.Context, run func(testRemote) error) error {
	return m.fn(ctx, run)
}

// passthroughExecuter returns an Executer that immediately calls run with a testRemote{id: id}.
func passthroughExecuter(id int) mockExecuter {
	return mockExecuter{fn: func(_ context.Context, run func(testRemote) error) error {
		return run(testRemote{id: id})
	}}
}

// noopCreateResources is a factory that always succeeds.
func noopCreateResources(
	_ context.Context,
	_ *generic.Transacter[testRemote, testResources],
	tx testRemote,
) (testResources, error) {
	return testResources{remote: tx}, nil
}

// TestTransacterNewSession verifies that Transact opens a new session via Execute when none exists.
func TestTransacterNewSession(t *testing.T) {
	executeCalled := false
	executer := mockExecuter{fn: func(_ context.Context, run func(testRemote) error) error {
		executeCalled = true
		return run(testRemote{id: 1})
	}}

	createCalled := false
	createResources := func(
		_ context.Context,
		_ *generic.Transacter[testRemote, testResources],
		tx testRemote,
	) (testResources, error) {
		createCalled = true
		return testResources{remote: tx}, nil
	}

	tr := generic.NewTransacter[testRemote, testResources](
		executer,
		createResources,
		generic.WithBackOffDelays[testRemote, testResources](),
	)

	runCalled := false
	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		runCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executeCalled {
		t.Error("Execute must be called for a new session")
	}
	if !createCalled {
		t.Error("createResources must be called")
	}
	if !runCalled {
		t.Error("run must be called")
	}
}

// TestTransacterExistingSession verifies that Transact reuses a session already in context.
func TestTransacterExistingSession(t *testing.T) {
	executer := mockExecuter{fn: func(_ context.Context, _ func(testRemote) error) error {
		t.Fatal("Execute must not be called when a session is already in context")
		return nil
	}}

	tr := generic.NewTransacter[testRemote, testResources](executer, noopCreateResources)

	existing := testRemote{id: 42}
	ctx := context.WithValue(
		context.Background(),
		atomic.SessionContextKey,
		&generic.Session[testRemote]{Tx: existing},
	)

	var seenRemote testRemote
	err := tr.Transact(ctx, func(_ context.Context, res testResources) error {
		seenRemote = res.remote
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenRemote != existing {
		t.Errorf("expected existing remote %v, got %v", existing, seenRemote)
	}
}

// TestTransacterWrongSessionType verifies that a non-Session value in context returns an error.
func TestTransacterWrongSessionType(t *testing.T) {
	tr := generic.NewTransacter[testRemote, testResources](passthroughExecuter(0), noopCreateResources)

	ctx := context.WithValue(context.Background(), atomic.SessionContextKey, "not a session")
	err := tr.Transact(ctx, func(_ context.Context, _ testResources) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for wrong session type in context, got nil")
	}
}

// TestTransacterCreateResourcesError verifies that a factory failure propagates as an error.
func TestTransacterCreateResourcesError(t *testing.T) {
	sentinel := errors.New("factory error")

	tr := generic.NewTransacter[testRemote, testResources](
		passthroughExecuter(0),
		func(_ context.Context, _ *generic.Transacter[testRemote, testResources], _ testRemote) (testResources, error) {
			return testResources{}, sentinel
		},
		generic.WithBackOffDelays[testRemote, testResources](),
	)

	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		t.Fatal("run must not be called when createResources fails")
		return nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected error chain to contain factory sentinel: %v", err)
	}
}

// TestTransacterRunError verifies that an error returned from run propagates.
func TestTransacterRunError(t *testing.T) {
	sentinel := errors.New("run error")

	tr := generic.NewTransacter[testRemote, testResources](
		passthroughExecuter(0),
		noopCreateResources,
		generic.WithBackOffDelays[testRemote, testResources](),
	)

	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		return sentinel
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected error chain to contain run sentinel: %v", err)
	}
}

// TestTransacterContextContainsSession verifies that a *Session is stored in the context
// passed to run.
func TestTransacterContextContainsSession(t *testing.T) {
	tr := generic.NewTransacter[testRemote, testResources](passthroughExecuter(7), noopCreateResources)

	var sessionVal any
	err := tr.Transact(context.Background(), func(ctx context.Context, _ testResources) error {
		sessionVal = ctx.Value(atomic.SessionContextKey)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, ok := sessionVal.(*generic.Session[testRemote])
	if !ok {
		t.Fatalf("expected *generic.Session[testRemote] in context, got %T", sessionVal)
	}
	if s.Tx.id != 7 {
		t.Errorf("expected session Tx.id == 7, got %d", s.Tx.id)
	}
}

// TestTransacterRetryEventuallySucceeds verifies that a retryable error is retried until success.
func TestTransacterRetryEventuallySucceeds(t *testing.T) {
	executeCalls := 0
	executer := mockExecuter{fn: func(_ context.Context, run func(testRemote) error) error {
		executeCalls++
		return run(testRemote{})
	}}

	tr := generic.NewTransacter[testRemote, testResources](
		executer,
		noopCreateResources,
		generic.WithBackOffDelays[testRemote, testResources](0, 0, 0),
	)

	runCalls := 0
	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		runCalls++
		if runCalls < 3 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runCalls != 3 {
		t.Errorf("expected 3 run calls, got %d", runCalls)
	}
	if executeCalls != 3 {
		t.Errorf("expected 3 Execute calls, got %d", executeCalls)
	}
}

// TestTransacterRetryExhausted verifies that after exhausting all retries an error is returned.
func TestTransacterRetryExhausted(t *testing.T) {
	tr := generic.NewTransacter[testRemote, testResources](
		passthroughExecuter(0),
		noopCreateResources,
		generic.WithBackOffDelays[testRemote, testResources](0, 0),
	)

	runCalls := 0
	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		runCalls++
		return context.DeadlineExceeded
	})

	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	// 1 initial attempt + 2 retries
	if runCalls != 3 {
		t.Errorf("expected 3 run calls (1 initial + 2 retries), got %d", runCalls)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded in error chain: %v", err)
	}
}

// TestTransacterExistingSessionNotRetried verifies that errors from an existing-session path
// are returned directly without retry.
func TestTransacterExistingSessionNotRetried(t *testing.T) {
	executer := mockExecuter{fn: func(_ context.Context, _ func(testRemote) error) error {
		t.Fatal("Execute must not be called on the existing-session path")
		return nil
	}}

	tr := generic.NewTransacter[testRemote, testResources](
		executer,
		noopCreateResources,
		generic.WithBackOffDelays[testRemote, testResources](0, 0, 0),
	)

	sentinel := errors.New("run error")
	ctx := context.WithValue(
		context.Background(),
		atomic.SessionContextKey,
		&generic.Session[testRemote]{Tx: testRemote{id: 1}},
	)

	runCalls := 0
	err := tr.Transact(ctx, func(_ context.Context, _ testResources) error {
		runCalls++
		return sentinel
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel in error chain: %v", err)
	}
	// no retry: existing-session path calls run exactly once
	if runCalls != 1 {
		t.Errorf("expected 1 run call (no retry on existing session), got %d", runCalls)
	}
}

// TestTransacterWithBackOffRetry verifies the custom retry function option is honoured.
func TestTransacterWithBackOffRetry(t *testing.T) {
	retryCalled := false
	customRetry := func(_ []time.Duration, run func() error) error {
		retryCalled = true
		return run()
	}

	tr := generic.NewTransacter[testRemote, testResources](
		passthroughExecuter(0),
		noopCreateResources,
		generic.WithBackOffRetry[testRemote, testResources](customRetry),
	)

	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retryCalled {
		t.Error("custom retry function must be called")
	}
}

// TestTransacterWithBackOffDelays verifies the custom delay slice option is honoured.
func TestTransacterWithBackOffDelays(t *testing.T) {
	calls := 0
	tr := generic.NewTransacter[testRemote, testResources](
		passthroughExecuter(0),
		noopCreateResources,
		generic.WithBackOffDelays[testRemote, testResources](0), // exactly 1 retry
	)

	err := tr.Transact(context.Background(), func(_ context.Context, _ testResources) error {
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls with 1 backoff entry, got %d", calls)
	}
}
