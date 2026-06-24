package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{
			name:       "no proxy header uses RemoteAddr without port",
			remoteAddr: "203.0.113.5:54321",
			want:       "203.0.113.5",
		},
		{
			name:       "single proxy hop returns the client IP",
			xff:        "203.0.113.9",
			remoteAddr: "10.0.0.1:5000",
			want:       "203.0.113.9",
		},
		{
			// A malicious client can prepend a fake entry; only the last entry,
			// appended by the trusted proxy, may be trusted.
			name:       "spoofed leading entry is ignored, trusted last entry wins",
			xff:        "1.2.3.4, 203.0.113.9",
			remoteAddr: "10.0.0.1:5000",
			want:       "203.0.113.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimiterEnforcesLimitPerIP(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	ip := "198.51.100.7"

	for i := 1; i <= 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("request %d should be allowed (limit 3)", i)
		}
	}
	if rl.allow(ip) {
		t.Fatal("4th request should be denied (over limit)")
	}

	// A different IP has its own independent budget.
	if !rl.allow("198.51.100.8") {
		t.Fatal("different IP should be allowed independently")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	rl := newRateLimiter(1, 20*time.Millisecond)
	ip := "198.51.100.9"

	if !rl.allow(ip) {
		t.Fatal("first request should be allowed")
	}
	if rl.allow(ip) {
		t.Fatal("second request within window should be denied")
	}

	time.Sleep(40 * time.Millisecond)
	if !rl.allow(ip) {
		t.Fatal("request after window reset should be allowed")
	}
}
