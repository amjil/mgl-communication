package ratelimit_test

import (
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/ratelimit"
)

func TestLimiter(t *testing.T) {
	l := ratelimit.New(2, time.Minute)
	if !l.Allow("u1") || !l.Allow("u1") {
		t.Fatal("first two should pass")
	}
	if l.Allow("u1") {
		t.Fatal("third should fail")
	}
	if !l.Allow("u2") {
		t.Fatal("other key should pass")
	}
}
