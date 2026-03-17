package usage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
}

func TestNewClientWithTimeout(t *testing.T) {
	timeout := 10 * time.Second
	c := NewClientWithTimeout(timeout)
	if c == nil {
		t.Fatal("NewClientWithTimeout() returned nil")
	}
	if c.httpClient.Timeout != timeout {
		t.Errorf("timeout = %v, want %v", c.httpClient.Timeout, timeout)
	}
}

func TestFetchUsage(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		statusCode  int
		wantErr     error
		wantStale   bool
		wantFiveHr  float64
		wantSevenDy float64
	}{
		{
			name: "successful response",
			response: `{
				"five_hour": {"utilization": 35.0, "resets_at": "2026-01-20T18:00:00Z"},
				"seven_day": {"utilization": 12.0, "resets_at": "2026-01-27T00:00:00Z"}
			}`,
			statusCode:  200,
			wantFiveHr:  35.0,
			wantSevenDy: 12.0,
		},
		{
			name: "full response with opus",
			response: `{
				"five_hour": {"utilization": 50.0, "resets_at": "2026-01-20T18:00:00Z"},
				"seven_day": {"utilization": 25.0, "resets_at": "2026-01-27T00:00:00Z"},
				"seven_day_opus": {"utilization": 0.0, "resets_at": null},
				"seven_day_oauth_apps": null,
				"iguana_necktie": null
			}`,
			statusCode:  200,
			wantFiveHr:  50.0,
			wantSevenDy: 25.0,
		},
		{
			name:       "401 unauthorized",
			response:   `{"error": "unauthorized"}`,
			statusCode: 401,
			wantErr:    ErrTokenExpired,
		},
		{
			name:       "500 server error",
			response:   `{"error": "internal server error"}`,
			statusCode: 500,
			wantErr:    ErrAPIError,
		},
		{
			name:       "400 bad request",
			response:   `{"error": "bad request"}`,
			statusCode: 400,
			wantErr:    ErrAPIError,
		},
		{
			name:       "invalid JSON response",
			response:   `{invalid json}`,
			statusCode: 200,
			wantErr:    nil, // Will fail with parse error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
					t.Errorf("Authorization header = %q, want Bearer prefix", auth)
				}
				if beta := r.Header.Get("anthropic-beta"); beta != "oauth-2025-04-20" {
					t.Errorf("anthropic-beta header = %q, want oauth-2025-04-20", beta)
				}
				if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "claude-code/cclv-") {
					t.Errorf("User-Agent header = %q, want claude-code/cclv- prefix", ua)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create client with mock server URL
			c := NewClient()

			// Create a custom transport that redirects to our test server
			c.httpClient = &http.Client{
				Timeout: apiTimeout,
				Transport: &mockTransport{
					handler: func(req *http.Request) (*http.Response, error) {
						// Modify request to use test server
						req.URL.Scheme = "http"
						req.URL.Host = server.URL[7:] // Remove "http://"
						return http.DefaultTransport.RoundTrip(req)
					},
				},
			}

			limits, stale, err := c.FetchUsage(context.Background(), "test-token")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("FetchUsage() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			// For invalid JSON test case
			if tt.name == "invalid JSON response" {
				if err == nil {
					t.Error("expected error for invalid JSON, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("FetchUsage() unexpected error = %v", err)
			}

			if stale != tt.wantStale {
				t.Errorf("stale = %v, want %v", stale, tt.wantStale)
			}

			if limits == nil {
				t.Fatal("limits is nil")
			}

			if limits.FiveHour != nil && limits.FiveHour.Utilization != tt.wantFiveHr {
				t.Errorf("FiveHour.Utilization = %v, want %v", limits.FiveHour.Utilization, tt.wantFiveHr)
			}

			if limits.SevenDay != nil && limits.SevenDay.Utilization != tt.wantSevenDy {
				t.Errorf("SevenDay.Utilization = %v, want %v", limits.SevenDay.Utilization, tt.wantSevenDy)
			}
		})
	}
}

// mockTransport allows overriding HTTP requests for testing
type mockTransport struct {
	handler func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.handler(req)
}

func TestFetchUsage_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// First call - should hit API
	_, stale1, err1 := c.FetchUsage(context.Background(), "test-token")
	if err1 != nil {
		t.Fatalf("first FetchUsage() error = %v", err1)
	}
	if stale1 {
		t.Error("first call stale = true, want false")
	}
	if callCount != 1 {
		t.Errorf("after first call, callCount = %d, want 1", callCount)
	}

	// Second call - should use cache
	_, stale2, err2 := c.FetchUsage(context.Background(), "test-token")
	if err2 != nil {
		t.Fatalf("second FetchUsage() error = %v", err2)
	}
	if stale2 {
		t.Error("second call stale = true, want false")
	}
	if callCount != 1 {
		t.Errorf("after second call, callCount = %d, want 1 (cache hit)", callCount)
	}
}

func TestFetchUsage_CacheMiss(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
	}))
	defer server.Close()

	c := NewClientWithTimeout(apiTimeout)
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// First call
	_, _, _ = c.FetchUsage(context.Background(), "test-token")

	// Invalidate cache
	c.InvalidateCache()

	// Second call - should hit API (cache invalidated)
	_, _, err := c.FetchUsage(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("FetchUsage() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (cache miss)", callCount)
	}
}

func TestFetchUsage_GracefulDegradation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call succeeds
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
		} else {
			// Subsequent calls fail
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error": "server error"}`))
		}
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// First call - succeeds and stores in lastGood
	limits1, stale1, err1 := c.FetchUsage(context.Background(), "test-token")
	if err1 != nil {
		t.Fatalf("first FetchUsage() error = %v", err1)
	}
	if stale1 {
		t.Error("first call stale = true, want false")
	}
	if limits1.FiveHour.Utilization != 35.0 {
		t.Errorf("first call utilization = %v, want 35.0", limits1.FiveHour.Utilization)
	}

	// Invalidate cache so second call hits API
	c.InvalidateCache()

	// Second call - fails but returns lastGood
	limits2, stale2, err2 := c.FetchUsage(context.Background(), "test-token")
	if err2 == nil {
		t.Error("second call should return error")
	}
	if !errors.Is(err2, ErrAPIError) {
		t.Errorf("second call error = %v, want ErrAPIError", err2)
	}
	if !stale2 {
		t.Error("second call stale = false, want true")
	}
	if limits2 == nil {
		t.Fatal("limits2 should not be nil (graceful degradation)")
	}
	if limits2.FiveHour.Utilization != 35.0 {
		t.Errorf("second call utilization = %v, want 35.0 (lastGood)", limits2.FiveHour.Utilization)
	}
}

func TestFetchUsage_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the timeout
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}}`))
	}))
	defer server.Close()

	c := NewClientWithTimeout(10 * time.Millisecond) // Very short timeout
	c.httpClient = &http.Client{
		Timeout: 10 * time.Millisecond,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, err := c.FetchUsage(context.Background(), "test-token")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if !errors.Is(err, ErrAPITimeout) {
		t.Errorf("expected ErrAPITimeout, got %v", err)
	}
}

func TestFetchUsage_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := c.FetchUsage(ctx, "test-token")
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

func TestFetchUsage_ContextDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(5 * time.Millisecond)

	_, _, err := c.FetchUsage(ctx, "test-token")
	if err == nil {
		t.Error("expected deadline exceeded error, got nil")
	}
	if !errors.Is(err, ErrAPITimeout) {
		t.Errorf("expected ErrAPITimeout, got %v", err)
	}
}

func TestFetchUsage_ConcurrentAccess(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		// Small delay to simulate real API
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// Launch multiple goroutines simultaneously
	var wg sync.WaitGroup
	errChan := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limits, _, err := c.FetchUsage(context.Background(), "test-token")
			if err != nil {
				errChan <- err
				return
			}
			if limits == nil {
				errChan <- errors.New("limits is nil")
				return
			}
			if limits.FiveHour == nil {
				errChan <- errors.New("FiveHour is nil")
				return
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent FetchUsage() error: %v", err)
	}
}

func TestInvalidateCache(t *testing.T) {
	c := NewClient()

	// Set some cache values
	c.cache = &UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35.0},
	}
	c.cacheTime = time.Now()

	// Verify cache is set
	if c.getCached() == nil {
		t.Error("cache should be set before invalidation")
	}

	// Invalidate
	c.InvalidateCache()

	// Verify cache is cleared
	if c.getCached() != nil {
		t.Error("cache should be nil after invalidation")
	}
}

func TestGetCached_Expiry(t *testing.T) {
	c := NewClient()

	// Set cache with old timestamp
	c.cache = &UsageLimits{
		FiveHour: &UsageWindow{Utilization: 35.0},
	}
	c.cacheTime = time.Now().Add(-2 * cacheTTL) // Expired

	// Should return nil for expired cache
	if c.getCached() != nil {
		t.Error("getCached() should return nil for expired cache")
	}

	// Set cache with recent timestamp
	c.cacheTime = time.Now()

	// Should return cache for valid entry
	if c.getCached() == nil {
		t.Error("getCached() should return cache for valid entry")
	}
}

func TestMakeRequest_Headers(t *testing.T) {
	receivedHeaders := make(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, values := range r.Header {
			receivedHeaders[key] = values
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 0}}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, _ = c.FetchUsage(context.Background(), "test-token-123")

	// Verify required headers
	expectedHeaders := map[string]string{
		"Authorization":  "Bearer test-token-123",
		"Anthropic-Beta": "oauth-2025-04-20",
		"User-Agent":     "claude-code/cclv-" + version.Version,
		"Accept":         "application/json",
		"Content-Type":   "application/json",
	}

	for key, want := range expectedHeaders {
		got := receivedHeaders.Get(key)
		if got != want {
			t.Errorf("Header %s = %q, want %q", key, got, want)
		}
	}
}

func TestFetchUsage_NoLastGoodOnFirstError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// First call fails - no lastGood available
	limits, stale, err := c.FetchUsage(context.Background(), "test-token")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if stale {
		t.Error("stale = true, want false (no lastGood)")
	}
	if limits != nil {
		t.Errorf("limits = %v, want nil (no lastGood)", limits)
	}
}

func TestFetchUsage_ResponseParsing(t *testing.T) {
	// Test parsing of various response formats
	tests := []struct {
		name     string
		response string
		check    func(t *testing.T, l *UsageLimits)
	}{
		{
			name:     "zero utilization",
			response: `{"five_hour": {"utilization": 0.0}, "seven_day": {"utilization": 0.0}}`,
			check: func(t *testing.T, l *UsageLimits) {
				if l.FiveHour.Utilization != 0.0 {
					t.Errorf("FiveHour.Utilization = %v, want 0.0", l.FiveHour.Utilization)
				}
			},
		},
		{
			name:     "100% utilization",
			response: `{"five_hour": {"utilization": 100.0}, "seven_day": {"utilization": 100.0}}`,
			check: func(t *testing.T, l *UsageLimits) {
				if l.FiveHour.Utilization != 100.0 {
					t.Errorf("FiveHour.Utilization = %v, want 100.0", l.FiveHour.Utilization)
				}
			},
		},
		{
			name:     "fractional utilization",
			response: `{"five_hour": {"utilization": 35.567}, "seven_day": {"utilization": 12.25}}`,
			check: func(t *testing.T, l *UsageLimits) {
				if l.FiveHour.Utilization != 35.567 {
					t.Errorf("FiveHour.Utilization = %v, want 35.567", l.FiveHour.Utilization)
				}
			},
		},
		{
			name: "with resets_at times",
			response: `{
				"five_hour": {"utilization": 35.0, "resets_at": "2026-01-20T18:00:00Z"},
				"seven_day": {"utilization": 12.0, "resets_at": "2026-01-27T00:00:00Z"}
			}`,
			check: func(t *testing.T, l *UsageLimits) {
				if l.FiveHour.ResetsAt == nil {
					t.Error("FiveHour.ResetsAt = nil, want non-nil")
				}
				if l.SevenDay.ResetsAt == nil {
					t.Error("SevenDay.ResetsAt = nil, want non-nil")
				}
			},
		},
		{
			name:     "null opus window",
			response: `{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}, "seven_day_opus": null}`,
			check: func(t *testing.T, l *UsageLimits) {
				if l.SevenDayOpus != nil {
					t.Errorf("SevenDayOpus = %v, want nil", l.SevenDayOpus)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			c := NewClient()
			c.httpClient = &http.Client{
				Timeout: apiTimeout,
				Transport: &mockTransport{
					handler: func(req *http.Request) (*http.Response, error) {
						req.URL.Scheme = "http"
						req.URL.Host = server.URL[7:]
						return http.DefaultTransport.RoundTrip(req)
					},
				},
			}

			limits, _, err := c.FetchUsage(context.Background(), "test-token")
			if err != nil {
				t.Fatalf("FetchUsage() error = %v", err)
			}

			tt.check(t, limits)
		})
	}
}

func TestFetchUsage_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		code    int
		wantErr error
	}{
		{400, ErrAPIError},
		{401, ErrTokenExpired},
		{403, ErrAPIError},
		{404, ErrAPIError},
		{429, ErrRateLimited},
		{500, ErrAPIError},
		{502, ErrAPIError},
		{503, ErrAPIError},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(`{"error": "test"}`))
			}))
			defer server.Close()

			c := NewClient()
			c.httpClient = &http.Client{
				Timeout: apiTimeout,
				Transport: &mockTransport{
					handler: func(req *http.Request) (*http.Response, error) {
						req.URL.Scheme = "http"
						req.URL.Host = server.URL[7:]
						return http.DefaultTransport.RoundTrip(req)
					},
				},
			}

			_, _, err := c.FetchUsage(context.Background(), "test-token")
			if err == nil {
				t.Errorf("FetchUsage() error = nil, wantErr %v", tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("FetchUsage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUsageLimitsJSONRoundTrip(t *testing.T) {
	// Test that UsageLimits can be marshaled and unmarshaled consistently
	original := &UsageLimits{
		FiveHour: &UsageWindow{
			Utilization: 35.5,
		},
		SevenDay: &UsageWindow{
			Utilization: 12.25,
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal
	var decoded UsageLimits
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify
	if decoded.FiveHour.Utilization != original.FiveHour.Utilization {
		t.Errorf("FiveHour.Utilization = %v, want %v", decoded.FiveHour.Utilization, original.FiveHour.Utilization)
	}
	if decoded.SevenDay.Utilization != original.SevenDay.Utilization {
		t.Errorf("SevenDay.Utilization = %v, want %v", decoded.SevenDay.Utilization, original.SevenDay.Utilization)
	}
}

// TestIntegration_GetOAuthTokenAndFetchUsage demonstrates the intended usage pattern:
// 1. Caller obtains token via GetOAuthToken()
// 2. Caller passes token to FetchUsage()
// This separation of concerns allows distinct error handling for credential vs API errors.
func TestIntegration_GetOAuthTokenAndFetchUsage(t *testing.T) {
	// This test demonstrates the integration pattern without hitting actual APIs.
	// The actual integration test that uses real credentials is intentionally skipped
	// in CI/automated tests since it requires real Claude Code credentials.

	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the token was passed correctly
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-from-credentials" {
			t.Errorf("Authorization = %q, want 'Bearer test-token-from-credentials'", auth)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
	}))
	defer server.Close()

	// Simulate the integration flow
	// Step 1: Get token (simulated - in real code this would be GetOAuthToken())
	token := "test-token-from-credentials"

	// Step 2: Create client and fetch usage
	client := NewClient()
	client.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// Step 3: Call FetchUsage with the token
	limits, stale, err := client.FetchUsage(context.Background(), token)
	if err != nil {
		t.Fatalf("FetchUsage() error = %v", err)
	}
	if stale {
		t.Error("stale = true, want false")
	}
	if limits == nil {
		t.Fatal("limits = nil")
	}
	if limits.FiveHour == nil || limits.FiveHour.Utilization != 35.0 {
		t.Errorf("FiveHour.Utilization = %v, want 35.0", limits.FiveHour.Utilization)
	}

	// Demonstrate error separation: credential errors vs API errors
	// In real usage:
	// token, credErr := GetOAuthToken()
	// if credErr != nil {
	//     // Handle credential error (ErrNoCredentials, ErrKeychainTimeout, etc.)
	// }
	// limits, stale, apiErr := client.FetchUsage(ctx, token)
	// if apiErr != nil {
	//     // Handle API error (ErrAPITimeout, ErrAPIError, ErrTokenExpired)
	//     // Note: if stale && limits != nil, can still use stale data
	// }
}

func TestFetchUsage_EmptyToken(t *testing.T) {
	// Empty token should still make the request (API will return 401)
	// Note: HTTP library sends "Bearer" not "Bearer " when token is empty
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// With empty token, header becomes "Bearer" (trimmed by HTTP lib)
		if auth != "Bearer" {
			t.Errorf("Authorization = %q, want 'Bearer'", auth)
		}
		// API returns 401 for empty/invalid token
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, err := c.FetchUsage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired for empty token, got %v", err)
	}
}

func TestFetchUsage_429WithoutRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, err := c.FetchUsage(context.Background(), "test-token")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}

	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatal("expected error to be *RateLimitError")
	}
	if rateLimitErr.RetryAfter != defaultRetryAfter {
		t.Errorf("RetryAfter = %v, want %v (default)", rateLimitErr.RetryAfter, defaultRetryAfter)
	}
}

func TestFetchUsage_429WithRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, err := c.FetchUsage(context.Background(), "test-token")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}

	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatal("expected error to be *RateLimitError")
	}
	if rateLimitErr.RetryAfter != 120*time.Second {
		t.Errorf("RetryAfter = %v, want 120s", rateLimitErr.RetryAfter)
	}
}

func TestFetchUsage_429WithInvalidRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "abc")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	_, _, err := c.FetchUsage(context.Background(), "test-token")
	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatal("expected error to be *RateLimitError")
	}
	if rateLimitErr.RetryAfter != defaultRetryAfter {
		t.Errorf("RetryAfter = %v, want %v (default for invalid header)", rateLimitErr.RetryAfter, defaultRetryAfter)
	}
}

func TestFetchUsage_429WithStaleData(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"five_hour": {"utilization": 35.0}, "seven_day": {"utilization": 12.0}}`))
		} else {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error": "rate limited"}`))
		}
	}))
	defer server.Close()

	c := NewClient()
	c.httpClient = &http.Client{
		Timeout: apiTimeout,
		Transport: &mockTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = server.URL[7:]
				return http.DefaultTransport.RoundTrip(req)
			},
		},
	}

	// First call succeeds
	limits1, _, err1 := c.FetchUsage(context.Background(), "test-token")
	if err1 != nil {
		t.Fatalf("first call error: %v", err1)
	}
	if limits1.FiveHour.Utilization != 35.0 {
		t.Errorf("first call utilization = %v, want 35.0", limits1.FiveHour.Utilization)
	}

	// Invalidate cache to force API call
	c.InvalidateCache()

	// Second call gets 429 but returns stale data
	limits2, stale, err2 := c.FetchUsage(context.Background(), "test-token")
	if !errors.Is(err2, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err2)
	}
	if !stale {
		t.Error("expected stale=true for 429 with lastGood")
	}
	if limits2 == nil {
		t.Fatal("expected stale data, got nil")
	}
	if limits2.FiveHour.Utilization != 35.0 {
		t.Errorf("stale utilization = %v, want 35.0", limits2.FiveHour.Utilization)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"", defaultRetryAfter},
		{"30", 30 * time.Second},
		{"0", defaultRetryAfter},
		{"-5", defaultRetryAfter},
		{"abc", defaultRetryAfter},
		{"120", 120 * time.Second},
	}

	for _, tt := range tests {
		t.Run("header="+tt.header, func(t *testing.T) {
			got := parseRetryAfter(tt.header)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{RetryAfter: 120 * time.Second}

	// Test errors.Is
	if !errors.Is(err, ErrRateLimited) {
		t.Error("errors.Is(RateLimitError, ErrRateLimited) should be true")
	}

	// Test Error() string
	errStr := err.Error()
	if !strings.Contains(errStr, "rate limited") {
		t.Errorf("Error() = %q, should contain 'rate limited'", errStr)
	}
	if !strings.Contains(errStr, "2m0s") {
		t.Errorf("Error() = %q, should contain retry duration", errStr)
	}

	// Test errors.As
	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatal("errors.As should succeed for *RateLimitError")
	}
	if rateLimitErr.RetryAfter != 120*time.Second {
		t.Errorf("RetryAfter = %v, want 120s", rateLimitErr.RetryAfter)
	}
}
