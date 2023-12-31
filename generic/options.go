package generic

import "time"

// WithBackOffRetry sets the retry function which manages automatic retries on errors.
func WithBackOffRetry[Remote any, Resources any](
	retry func(backoffs []time.Duration, run func() error) error,
) TransacterOption[Remote, Resources] {
	return func(transacter *Transacter[Remote, Resources]) {
		transacter.retry = retry
	}
}

// WithBackOffDelays sets the backoffs to use on retry.
// The maximum amount of retries is determined by the length of backoffs.
func WithBackOffDelays[Remote any, Resources any](
	backoffs ...time.Duration,
) TransacterOption[Remote, Resources] {
	return func(transacter *Transacter[Remote, Resources]) {
		transacter.backoffs = backoffs
	}
}
