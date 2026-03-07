package atomic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// testCodedError implements errorCoder for testing.
type testCodedError struct {
	code int
}

func (e *testCodedError) Error() string { return fmt.Sprintf("sqlite error code %d", e.code) }
func (e *testCodedError) Code() int     { return e.code }

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"random error", fmt.Errorf("random"), false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"net closed", net.ErrClosed, true},
		{"os deadline exceeded", os.ErrDeadlineExceeded, true},
		{"wrapped deadline exceeded", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), true},
		{"SQLITE_BUSY", &testCodedError{code: 5}, true},
		{"SQLITE_LOCKED", &testCodedError{code: 6}, true},
		{"wrapped SQLITE_BUSY", fmt.Errorf("wrapped: %w", &testCodedError{code: 5}), true},
		{"wrapped SQLITE_LOCKED", fmt.Errorf("wrapped: %w", &testCodedError{code: 6}), true},
		{"other sqlite code", &testCodedError{code: 1}, false},
		{"SQLITE_READONLY", &testCodedError{code: 8}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDefaultRetry(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		calls := 0
		err := DefaultRetry([]time.Duration{0, 0}, func() error {
			calls++
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}
	})

	t.Run("non-retryable error returned immediately", func(t *testing.T) {
		sentinel := fmt.Errorf("non-retryable")
		calls := 0
		err := DefaultRetry([]time.Duration{0, 0}, func() error {
			calls++
			return sentinel
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 1 {
			t.Errorf("expected 1 call (no retries), got %d", calls)
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected sentinel in error chain, got: %v", err)
		}
	})

	t.Run("retryable error retries until success", func(t *testing.T) {
		calls := 0
		err := DefaultRetry([]time.Duration{0, 0, 0}, func() error {
			calls++
			if calls < 3 {
				return context.DeadlineExceeded
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("retryable error exhausts all retries", func(t *testing.T) {
		calls := 0
		err := DefaultRetry([]time.Duration{0, 0}, func() error {
			calls++
			return context.DeadlineExceeded
		})
		// 1 initial + 2 retries = 3 total
		if calls != 3 {
			t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", calls)
		}
		if err == nil {
			t.Fatal("expected error after exhausting retries, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded in error chain, got: %v", err)
		}
	})

	t.Run("empty backoffs runs once then returns error", func(t *testing.T) {
		calls := 0
		err := DefaultRetry([]time.Duration{}, func() error {
			calls++
			return context.DeadlineExceeded
		})
		if calls != 1 {
			t.Errorf("expected 1 call with empty backoffs, got %d", calls)
		}
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("retryable then non-retryable stops mid-flight", func(t *testing.T) {
		sentinel := fmt.Errorf("non-retryable")
		calls := 0
		err := DefaultRetry([]time.Duration{0, 0, 0}, func() error {
			calls++
			if calls == 1 {
				return context.DeadlineExceeded
			}
			return sentinel
		})
		// first call retryable, second call non-retryable: loop exits
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected sentinel in error chain, got: %v", err)
		}
	})
}
