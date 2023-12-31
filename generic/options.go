package generic

import "time"

func WithBackOffRetry[Remote any, Resources any](
	retry func(backoffs []time.Duration, run func() error) error,
) TransacterOption[Remote, Resources] {
	return func(transacter *Transacter[Remote, Resources]) {
		transacter.retry = retry
	}
}

func WithBackOffDelays[Remote any, Resources any](
	backoffs ...time.Duration,
) TransacterOption[Remote, Resources] {
	return func(transacter *Transacter[Remote, Resources]) {
		transacter.backoffs = backoffs
	}
}
