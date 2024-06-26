package atomic

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// DefaultBackoffs are the default backoffs for transacters
var DefaultBackoffs = []time.Duration{
	100 * time.Millisecond,
	time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
}

// DefaultRetry is the default retry function for transacters.
// It retries errors that are have one of the following errors in their chain:
// - context.DeadlineExceeded
// - net.ErrClosed
// - os.ErrDeadlineExceeded
// It retries for a maximum of len(backoffs) times.
func DefaultRetry(backoffs []time.Duration, run func() error) error {
	var (
		i    int
		merr error
	)

	err := run()
	for i = 0; isRetryable(err) && i < len(backoffs); i++ {
		merr = multierr.Append(merr, errors.Wrapf(err, "try %d", i))
		time.Sleep(backoffs[i])
		err = run()
	}

	if err != nil {
		merr = multierr.Append(merr, errors.Wrapf(err, "try %d", i))

		return errors.Wrap(merr, "error not retryable or reached maximum number of retries")
	}

	return nil
}

func isRetryable(err error) bool {
	switch {
	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, net.ErrClosed),
		errors.Is(err, os.ErrDeadlineExceeded):
		return true
	}

	return false
}
