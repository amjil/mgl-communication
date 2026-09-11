package retry

import "time"

// DelayForAttempt returns the wait before the given attempt number (1-based).
// Spec: 1 immediate, 2 → 5s, 3 → 30s, 4 → 5m, 5 → 30m
func DelayForAttempt(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 0
	case 2:
		return 5 * time.Second
	case 3:
		return 30 * time.Second
	case 4:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func ShouldRetry(retryable bool, attempts, maxAttempts int) bool {
	return retryable && attempts < maxAttempts
}
