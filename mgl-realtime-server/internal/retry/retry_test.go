package retry_test

import (
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/retry"
)

func TestDelayForAttempt(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 0},
		{2, 5 * time.Second},
		{3, 30 * time.Second},
		{4, 5 * time.Minute},
		{5, 30 * time.Minute},
		{6, 30 * time.Minute},
	}
	for _, tc := range cases {
		if got := retry.DelayForAttempt(tc.attempt); got != tc.want {
			t.Fatalf("attempt %d: got %v want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	if !retry.ShouldRetry(true, 1, 5) {
		t.Fatal("expected retry")
	}
	if retry.ShouldRetry(false, 1, 5) {
		t.Fatal("non-retryable should not retry")
	}
	if retry.ShouldRetry(true, 5, 5) {
		t.Fatal("max attempts reached")
	}
}
